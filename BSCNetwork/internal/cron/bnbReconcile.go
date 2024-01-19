package cron

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"bsc_network/internal/config"
	"bsc_network/internal/config/log"
	"bsc_network/internal/entity"
	"bsc_network/internal/geth"
	"bsc_network/internal/tools"

	hdw "bsc_network/internal/hdwallet"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

type (
	BNBReconcile interface {
		Run(ctx context.Context)
	}

	bnbReconcile struct {
		geth     geth.Geth
		hdwallet *hdw.Wallet
		cfg      config.Config
		ph       config.PhraseConfig
		db       *sqlx.DB
		logger   log.Logger

		bnbReconcileThreshold  decimal.Decimal
		usdtReconcileThreshold decimal.Decimal
		pendingTx              map[string]string

		listNotReconcileTxStmt *sqlx.Stmt
		updateBNBBlockTxStmt   *sqlx.Stmt
	}
)

const (
	listNotReconcileBNBTxQuery string = `SELECT * FROM block_transaction WHERE type = 'DEPOSIT' AND is_reconcile = 0 AND token = 'USDT-BNB'`
	updateBNBBlockTxQuery      string = `UPDATE block_transaction SET is_reconcile = 1 WHERE recipient = ?`

	bnbReconcilePendingTxInterval time.Duration = 5 * time.Second
	bnbReconcileInterval          time.Duration = 1 * time.Minute
)

func NewBNBReconcile(
	ctx context.Context,
	geth geth.Geth,
	hdwallet *hdw.Wallet,
	cfg config.Config,
	ph config.PhraseConfig,
	db *sqlx.DB,
	logger log.Logger,
) (BNBReconcile, error) {
	bnbReconcileThreshold, err := decimal.NewFromString(cfg.Blockchain.BNBReconcileThreshold)
	if err != nil {
		return &bnbReconcile{}, fmt.Errorf("[NewReconcile] internal error: %w", err)
	}

	usdtReconcileThreshold, err := decimal.NewFromString(cfg.Blockchain.USDTReconcileThreshold)
	if err != nil {
		return &bnbReconcile{}, fmt.Errorf("[NewReconcile] internal error: %w", err)
	}

	listNotReconcileTxStmt, err := db.PreparexContext(ctx, listNotReconcileBNBTxQuery)
	if err != nil {
		return &bnbReconcile{}, fmt.Errorf("[NewReconcile] internal error: %w", err)
	}

	updateBNBBlockTxStmt, err := db.PreparexContext(ctx, updateBNBBlockTxQuery)
	if err != nil {
		return &bnbReconcile{}, fmt.Errorf("[NewReconcile] internal error: %w", err)
	}

	return &bnbReconcile{
		geth,
		hdwallet,
		cfg,
		ph,
		db,
		logger,
		bnbReconcileThreshold,
		usdtReconcileThreshold,
		make(map[string]string),
		listNotReconcileTxStmt,
		updateBNBBlockTxStmt,
	}, nil
}

func (e *bnbReconcile) Run(ctx context.Context) {
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				e.logger.Error("bnb reconcile panic", zap.Stack("stack"))
			}
		}()

		for {
			e.logger.Info("Checking bnb block transactions...")

			var blockTxs []entity.CSBlockTransaction
			if err := e.listNotReconcileTxStmt.SelectContext(ctx, &blockTxs); err != nil {
				e.logger.Errorf("[Run] listNotReconcileTxStmt internal error: %w", err)
				time.Sleep(bnbReconcileInterval)
				continue
			}

			fmt.Printf("\n\n block need to reconciled is %d \n\n", len(blockTxs))

			if len(blockTxs) == 0 {
				e.logger.Info("no transaction need to reconcile")
				time.Sleep(bnbReconcileInterval)
				continue
			}

			derivationPath := tools.GetDerivationPath(tools.HDWalletDefaultIndex)
			rootAccount, err := e.hdwallet.Derive(hdw.MustParseDerivationPath(derivationPath), false)
			if err != nil {
				e.logger.Errorf("[Run] rootAccount internal error: %w", err)
				time.Sleep(bnbReconcileInterval)
				continue
			}

			for _, btx := range blockTxs {
				derivationPath = tools.GetDerivationPath(btx.WalletIndex)
				userAccount, herr := e.hdwallet.Derive(hdw.MustParseDerivationPath(derivationPath), false)
				if herr != nil {
					e.logger.Errorf("[Run] userAccount internal error: %w", err)
				}

				fmt.Printf("\n\n btx is %v \n\n\n", btx)

				if err = e.processReconcile(ctx, btx.Token, btx.Amount, userAccount, rootAccount, btx.ID); err != nil {
					e.logger.Errorf("[Run] processReconcile internal error: %w", err)
				}

			}

			e.logger.Infof("Block transaction reconciled, total: %d", len(blockTxs))
			time.Sleep(bnbReconcileInterval)
		}
	}()
}

func (e *bnbReconcile) reconcileBNBTransaction(
	ctx context.Context,
	userAccount, rootAccount accounts.Account,
	bnbReconcileThreshold decimal.Decimal,
	btxId int64,
) error {
	gasPrice, err := e.geth.SuggestGasPrice(ctx)
	if err != nil {
		return fmt.Errorf("[reconcileBNBTransaction] internal error: %w", err)
	}

	gasLimit := new(big.Float).SetUint64(e.geth.BNBGasLimit())
	gasPriceBigFloat := new(big.Float).SetInt(gasPrice)

	gasCost := tools.CalculateGasCost(gasLimit, gasPriceBigFloat)
	gasCostDecimal, _ := decimal.NewFromString(gasCost.String())
	bnbGasCost := tools.ToDecimal(gasCostDecimal.String(), e.geth.BNBDecimals())

	availableBalance, err := e.geth.GetBNB(ctx, userAccount.Address.Hex())
	if err != nil {
		return fmt.Errorf("[reconcileBNBTransaction] internal error: %w", err)
	}

	amount, err := decimal.NewFromString(availableBalance.String())
	if err != nil {
		return fmt.Errorf("[reconcileBNBTransaction] internal error: %w", err)
	}

	if amount.IsZero() {
		e.logger.Info("[reconcileBNBTransaction] user balance is empty, skipping reconcile step")
		return nil
	}

	actualAmount := amount.Sub(bnbGasCost)

	// leave some eth for future transaction
	actualAmount = actualAmount.Sub(bnbReconcileThreshold)

	if e.bnbReconcileThreshold.GreaterThan(actualAmount) {
		return nil
	}

	transferAmountInWei := tools.ToWei(actualAmount, e.geth.BNBDecimals())

	if err = e.checkPendingTx(ctx, userAccount.Address.Hex()); err != nil {
		return fmt.Errorf("[reconcileUSDTTransaction] internal error: %w", err)
	}

	userPK, err := e.hdwallet.PrivateKeyBytes(userAccount)
	if err != nil {
		return fmt.Errorf("[reconcileBNBTransaction] internal error: %w", err)
	}

	txn, err := e.geth.TransferBNB(ctx, userPK, rootAccount.Address.Hex(), transferAmountInWei, gasPrice)
	if err != nil {
		return fmt.Errorf("[reconcileBNBTransaction] internal error: %w", err)
	}

	e.pendingTx[userAccount.Address.Hex()] = txn

	return nil
}

func (e *bnbReconcile) reconcileUSDTTransaction(ctx context.Context, userAccount, rootAccount accounts.Account, btxId int64) error {
	reconcileAddress := e.ph.BnbReconcileAddress
	fmt.Printf("\n\nreconcileAddress  is %s \n\n", reconcileAddress)
	gasPrice, gerr := e.geth.SuggestGasPrice(ctx)
	if gerr != nil {
		return fmt.Errorf("[reconcileUSDTTransaction] gasPrice internal error: %w", gerr)
	}
	gasLimit := new(big.Float).SetUint64(e.geth.Bnb20GasLimit())
	gasPriceBigFloat := new(big.Float).SetInt(gasPrice)

	bigFloatGasCost := tools.CalculateGasCost(gasLimit, gasPriceBigFloat)
	decimalGasCost, _ := decimal.NewFromString(bigFloatGasCost.String())
	requiredGasCostInBnb := tools.ToDecimal(decimalGasCost.String(), e.geth.BNBDecimals())

	//	Check if root wallet still got pending transaction
	if err := e.checkPendingTx(ctx, rootAccount.Address.Hex()); err != nil {
		return fmt.Errorf("[reconcileUSDTTransaction] checkPendingTx internal error: %w", err)
	}

	userBalance, gerr := e.geth.GetBNB(ctx, userAccount.Address.Hex())
	if gerr != nil {
		return fmt.Errorf("[reconcileUSDTTransaction] userBalance internal error: %w", gerr)
	}

	decimalUserBalance, _ := decimal.NewFromString(userBalance.String())

	availableBalance, err := e.geth.GetUSDT(userAccount.Address.Hex())
	if err != nil {
		return fmt.Errorf("[reconcileUSDTTransaction] availableBalance user internal error: %w", err)
	}

	decimalavailableBalance, _ := decimal.NewFromString(availableBalance.String())

	fmt.Printf("\n\ndecimalavailableBalance is %v ", decimalavailableBalance)

	if requiredGasCostInBnb.GreaterThan(decimalUserBalance) && decimalavailableBalance.GreaterThan(e.usdtReconcileThreshold) {
		e.logger.Infof(
			"User wallet dont have enough eth to begin reconcile process, balance: %s, min balance: %s",
			decimalUserBalance.String(),
			requiredGasCostInBnb.String(),
		)

		// Transfer required gas fee to member wallet
		rootPK, err := e.hdwallet.PrivateKeyBytes(rootAccount)
		if err != nil {
			return fmt.Errorf("[reconcileUSDTTransaction] e.hdwallet.PrivateKeyBytes internal error: %w", err)
		}

		weiRequiredGasCost := tools.ToWei(requiredGasCostInBnb, e.geth.BNBDecimals())
		gasTxn, err := e.geth.TransferBNB(ctx, rootPK, userAccount.Address.Hex(), weiRequiredGasCost, gasPrice)
		if err != nil {
			return fmt.Errorf("[reconcileUSDTTransaction] e.geth.TransferBNB internal error: %w", err)
		}

		e.pendingTx[rootAccount.Address.Hex()] = gasTxn
	}

	//	Check if root wallet still got pending transaction
	if err := e.checkPendingTx(ctx, rootAccount.Address.Hex()); err != nil {
		return fmt.Errorf("[reconcileUSDTTransaction] e.checkPendingTx root internal error: %w", err)
	}

	//	Check if user wallet still got pending transaction
	if err := e.checkPendingTx(ctx, userAccount.Address.Hex()); err != nil {
		return fmt.Errorf("[reconcileUSDTTransaction] e.checkPendingTx user internal error: %w", err)
	}

	userPK, err := e.hdwallet.PrivateKeyBytes(userAccount)
	if err != nil {
		return fmt.Errorf("[reconcileUSDTTransaction] userPK internal error: %w", err)
	}

	amount := tools.ToWei(availableBalance.String(), e.geth.BNBUSDTDecimals())

	fmt.Printf("\n\n amount is %s \n\n", amount)
	if decimalavailableBalance.GreaterThanOrEqual(e.usdtReconcileThreshold) && userAccount.Address.Hex() != rootAccount.Address.Hex() {

		txn, err := e.geth.TransferUSDT(ctx, userPK, reconcileAddress, amount, gasPrice, tools.USDTBNBToken)
		if err != nil {
			return fmt.Errorf("[reconcileUSDTTransaction] TransferUSDT internal error: %w", err)
		}

		fmt.Printf("\n\n TransferUSDT txn is %s \n\n", txn)
		if txn != "" {
			e.pendingTx[userAccount.Address.Hex()] = txn
		}
		if _, err = e.updateBNBBlockTxStmt.ExecContext(ctx, userAccount.Address.Hex()); err != nil {
			e.logger.Errorf("[Run] updateBNBBlockTxStmt internal error: %w", err)
		}
	}

	return nil
}

func (e *bnbReconcile) checkPendingTx(ctx context.Context, address string) error {
	if txn, ok := e.pendingTx[address]; ok {
		isPending := true
		err := error(nil)

		for isPending {
			_, isPending, err = e.geth.GetTransaction(ctx, common.HexToHash(txn))
			if err != nil {
				if err.Error() == e.geth.TransactionNotFoundMsg() {
					isPending = true
				} else {
					return fmt.Errorf("[checkPendingTx] internal error: %w", err)
				}
			}
			time.Sleep(bnbReconcilePendingTxInterval)
		}

		delete(e.pendingTx, address)
	}

	return nil
}

func (e *bnbReconcile) getReconcileType(token string, amount decimal.Decimal) string {
	var reconcileType string

	switch token {
	case tools.BNBCoin:
		if amount.GreaterThanOrEqual(e.bnbReconcileThreshold) {
			reconcileType = "BNB"
		}
	case tools.USDTBNBToken:
		if amount.GreaterThanOrEqual(e.usdtReconcileThreshold) {
			reconcileType = "USDT-BNB"
		}
	default:
		return reconcileType
	}

	return reconcileType
}

func (e *bnbReconcile) processReconcile(
	ctx context.Context,
	token string,
	amount decimal.Decimal,
	userAccount, rootAccount accounts.Account,
	btxId int64,
) error {
	var err error
	fmt.Printf("process started %d\n", btxId)
	fmt.Printf("token %s", token)
	switch e.getReconcileType(token, amount) {

	case "BNB":
		if rerr := e.reconcileBNBTransaction(ctx, userAccount, rootAccount, e.bnbReconcileThreshold, btxId); rerr != nil {
			err = fmt.Errorf("[reconcileTransaction] internal error: %w", rerr)
		}
	case "USDT-BNB":
		if rerr := e.reconcileUSDTTransaction(ctx, userAccount, rootAccount, btxId); rerr != nil {
			err = fmt.Errorf("[reconcileTransaction] internal error: %w", rerr)
		}
	}

	return err
}
