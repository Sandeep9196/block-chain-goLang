package cron

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"reflect"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	"bsc_network/internal/config"
	"bsc_network/internal/config/log"
	"bsc_network/internal/entity"
	"bsc_network/internal/geth"
	"bsc_network/internal/randx"
	"bsc_network/internal/tools"

	hdw "bsc_network/internal/hdwallet"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/spf13/viper"
)

type (
	BNBBlockCrawler interface {
		Run(ctx context.Context)
	}

	bnbblockCrawler struct {
		env string

		hdwallet *hdw.Wallet
		geth     geth.Geth
		cfg      config.Config
		ph       config.PhraseConfig
		rds      redis.Client
		db       *sqlx.DB
		logger   log.Logger
		viper    *viper.Viper
		mu       sync.Mutex

		bnbReconcileThreshold  decimal.Decimal
		usdtReconcileThreshold decimal.Decimal
		rootWallet             string
		wallets                map[string]interface{}
		blockInternal          *big.Int

		countBnbWalletStmt   *sqlx.Stmt
		listAddressIndexStmt *sqlx.Stmt
		getTxDataStmt        *sqlx.NamedStmt
		createBlockTxStmt    *sqlx.NamedStmt
	}

	bnbtxType   string
	bnbredisKey string
)

const (
	bnbdeposit        bnbtxType = "DEPOSIT"
	bnbwithdraw       bnbtxType = "WITHDRAW"
	bnbreconciliation bnbtxType = "RECONCILE"
	bnbgasTx          bnbtxType = "GAS_FEE"

	// Cron setting.
	bnbpath                               = "./BEP20.abi"
	bnbBlockCrawlerFileName string        = "block-bnb"
	bnbBlockCrawlerFileEXT  string        = "yml"
	bnbBlockCrawlerInterval time.Duration = 10 * time.Second

	// Query.
	bnbGetRootWalletQuery    string = "SELECT COUNT(id) FROM wallet WHERE " + "`index`" + "= 0 AND network = 'BNB'"
	countBnbWalletQuery      string = "SELECT COUNT(id) FROM wallet WHERE network = 'BNB'"
	createRootBNBWalletQuery string = `INSERT INTO wallet(address, ` + "`index`" + `, network) 
  values(:address, :index, :network)`
	bnbListAddressIndexQuery string = `SELECT address, ` + "`index`" + ` FROM wallet ORDER BY ` + "`index`" + ` ASC`
	bnbCreateBlockTxQuery    string = `INSERT INTO block_transaction (
  type, block_num, txn, gas_cost, wallet_index, sender, recipient, amount, token, is_reconcile, tran_no
  ) VALUES (:type, :block_num, :txn, :gas_cost, :wallet_index, :sender, :recipient, :amount, :token, :is_reconcile, :tran_no)`
	bnbGetCryptoWithdrawDetailQuery string = `SELECT * FROM member_request_details 
  WHERE txn = ?`
	bnbMemberVipDetailQuery string = `SELECT id, user_id, card_number, member_name FROM member 
  WHERE user_id = ?`

	bnbGetTxDataQuery string = `SELECT count(*) FROM block_transaction WHERE block_num= ? AND sender= ? AND recipient= ? AND type= ? AND token= ?`

	// Redis key.
	bnbPrefix    bnbredisKey = "bnb-crypto:"
	bnbWalletKey bnbredisKey = "bnb_wallet"

	// BNB network setting.
	bnb20TransferMethodID string = "a9059cbb"
	bnb20MainnetChainID   int64  = 56
	bnb20TestNetChainID   int64  = 97
	bnbDecimals           int    = 18
	bnbusdtDecimals       int    = 18
	bnbTruncateDecimal    int32  = 6
)

func BNBBNBNewBlockCrawler(
	ctx context.Context,
	env string,
	geth geth.Geth,
	hdwallet *hdw.Wallet,
	cfg config.Config,
	ph config.PhraseConfig,
	db *sqlx.DB,
	rds redis.Client,
	logger log.Logger,
) (BNBBlockCrawler, error) {
	b := &bnbblockCrawler{
		env:      env,
		hdwallet: hdwallet,
		geth:     geth,
		cfg:      cfg,
		rds:      rds,
		db:       db,
		logger:   logger,
		wallets:  make(map[string]interface{}),
	}

	path := "/usr/www/app/"
	if env == "local" {
		path = "./"
	}

	v := viper.New()
	v.SetConfigName(bnbBlockCrawlerFileName)
	v.SetConfigType(bnbBlockCrawlerFileEXT)
	v.AddConfigPath(path)

	b.viper = v

	b.ph = ph

	file := fmt.Sprintf("%s.%s", bnbBlockCrawlerFileName, bnbBlockCrawlerFileEXT)

	if verr := b.viper.ReadInConfig(); errors.As(verr, &viper.ConfigFileNotFoundError{}) {
		b.logger.Info("block-bnb.yml not found, creating new block-bnb.yml...")

		if _, err := os.Create(path + file); err != nil {
			return &bnbblockCrawler{}, fmt.Errorf("[BNBNewBlockCrawler] internal error: %w", err)
		}

		blockNum, err := b.geth.GetLatestBlock(ctx)
		if err != nil {
			return &bnbblockCrawler{}, fmt.Errorf("[BNBNewBlockCrawler] internal error: %w", err)
		}

		if err = b.updateBlockNum(blockNum.Uint64()); err != nil {
			return &bnbblockCrawler{}, fmt.Errorf("[BNBNewBlockCrawler] internal error: %w", err)
		}
	}

	b.blockInternal = new(big.Int).SetInt64(cfg.Blockchain.BlockInterval)

	var err error
	b.bnbReconcileThreshold, err = decimal.NewFromString(b.cfg.Blockchain.BNBReconcileThreshold)
	if err != nil {
		return &bnbblockCrawler{}, err
	}

	b.usdtReconcileThreshold, err = decimal.NewFromString(b.cfg.Blockchain.USDTReconcileThreshold)
	if err != nil {
		return &bnbblockCrawler{}, err
	}

	derivationPath := tools.GetDerivationPath(tools.HDWalletDefaultIndex)
	rootAccount, err := b.hdwallet.Derive(hdw.MustParseDerivationPath(derivationPath), false)
	if err != nil {
		return &bnbblockCrawler{}, fmt.Errorf("[BNBNewBlockCrawler] internal error: %w", err)
	}

	var count int
	if err = b.db.GetContext(ctx, &count, bnbGetRootWalletQuery); err != nil {
		return &bnbblockCrawler{}, fmt.Errorf("[BNBNewBlockCrawler] internal error: %w", err)
	}

	if count == 0 {
		wallet := entity.CSWallet{
			Address: rootAccount.Address.String(),
			Index:   tools.HDWalletDefaultIndex,
			Network: "BNB",
		}
		if _, err = b.db.NamedExecContext(ctx, createRootBNBWalletQuery, &wallet); err != nil {
			return &bnbblockCrawler{}, fmt.Errorf("[BNBNewBlockCrawler] internal error: %w", err)
		}
	}

	b.rootWallet = rootAccount.Address.String()

	if err = b.setupStatement(ctx); err != nil {
		return &bnbblockCrawler{}, fmt.Errorf("[BNBNewBlockCrawler] internal error: %w", err)
	}

	return b, nil
}

func (b *bnbblockCrawler) Run(ctx context.Context) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("\n\nbnb block crawler panic\n\n%v\n\n", r)
				b.logger.Error("bnb block crawler panic", zap.Stack("stack"))
			}

			b.countBnbWalletStmt.Close()
			b.listAddressIndexStmt.Close()
			b.countBnbWalletStmt.Close()
		}()

		for {
			// check if lenth of wallet of redis are the same with db, we will use the user wallets list of redis
			if err := b.cacheCheck(ctx); err != nil {
				b.logger.Error("[Run] internal error: %w", err)
				time.Sleep(bnbBlockCrawlerInterval)
				continue
			}

			// get the last block number of Ethereum blockchain
			latestBlockNum, gerr := b.geth.GetLatestBlock(ctx)
			if gerr != nil {
				b.logger.Error("[Run] internal error: %w", gerr)
				time.Sleep(bnbBlockCrawlerInterval)
				continue
			}

			blockNumInt64 := b.viper.GetUint64("block")               // get local block number
			currentBlockNum := new(big.Int).SetUint64(blockNumInt64)  // convert local number to big int
			nextBlockNum := new(big.Int).SetUint64(blockNumInt64 + 1) // next block number

			// compare the lastest block number of chain with local block number plush blockInternal
			if latestBlockNum.Cmp(new(big.Int).Add(currentBlockNum, b.blockInternal)) > 0 {
				b.logger.Infof("fetching block %d", nextBlockNum.Uint64())

				// get all transaction by block number
				transactions, err := b.geth.GetTransactionByBlockNum(ctx, nextBlockNum)
				if err != nil {
					b.logger.Error("[Run] internal error: %w", err)
					time.Sleep(bnbBlockCrawlerInterval)
					continue
				}
				// total of process of root or chlid transaction
				total, err := b.processTransactions(ctx, transactions)
				if err != nil {
					b.logger.Error("[Run] internal error: %w", err)
				}

				b.logger.Infof("block crawling completed, processed transactions: %d", total)

				if err = b.updateBlockNum(nextBlockNum.Uint64()); err != nil {
					b.logger.Error("[Run] internal error: %w", err)
				}
			} else {
				time.Sleep(bnbBlockCrawlerInterval)
			}
		}
	}()
}

func (b *bnbblockCrawler) setupStatement(ctx context.Context) error {
	countBnbWalletStmt, err := b.db.PreparexContext(ctx, countBnbWalletQuery)
	if err != nil {
		return fmt.Errorf("[BNBNewBlockCrawler] internal error: %w", err)
	}

	listAddressIndexStmt, err := b.db.PreparexContext(ctx, bnbListAddressIndexQuery)
	if err != nil {
		return fmt.Errorf("[BNBNewBlockCrawler] internal error: %w", err)
	}

	// fmt.Printf("\n\n ctx \n %s \n\n", ctx)

	createBlockTxStmt, err := b.db.PrepareNamedContext(ctx, bnbCreateBlockTxQuery)

	fmt.Printf("\n\nCreate block tx statement \n\n%v \n\n", createBlockTxStmt)

	if err != nil {
		return fmt.Errorf("[BNBNewBlockCrawler] internal error: %w", err)
	}

	getTxDataStmt, err := b.db.PrepareNamedContext(ctx, bnbGetTxDataQuery)
	if err != nil {
		return fmt.Errorf("[BNBNewBlockCrawler] internal error: %w", err)
	}

	b.countBnbWalletStmt = countBnbWalletStmt
	b.listAddressIndexStmt = listAddressIndexStmt
	b.createBlockTxStmt = createBlockTxStmt
	b.getTxDataStmt = getTxDataStmt

	return nil
}

func (b *bnbblockCrawler) processTransactions(ctx context.Context, transactions types.Transactions) (int, error) {
	var total int

	for _, tx := range transactions {
		// skip if recipient address is not found
		// fmt.Printf("\n\nTransactions tx %v\n", tx.To().Hex())
		// fmt.Printf("\n\nb.cfg.Blockchain.BNBUSDTContract tx %v\n\n\n\n", b.cfg.Blockchain.BNBUSDTContract)

		if tx.To() == nil {
			continue
		}

		// get message of transaction
		msg, err := b.getMessage(tx)
		if err != nil {
			return 0, fmt.Errorf("[processTransactions] internal error: %w", err)
		}

		// fmt.Printf("\n\nmsg from is %v", msg.From().Hex())
		// fmt.Printf("\n\nmsg to is %v", msg.To().Hex())
		// fmt.Printf("\n\nlen(tx.Data()) %v", len(tx.Data()))
		// fmt.Printf("\n\nroot Wallet is %v", b.rootWallet)

		t := b.getTransactionType(ctx, msg, tx)

		// fmt.Printf("\n\nt is %v", t)

		if t == "" {
			continue
		}

		//	Check if receipt status is complete
		receipt, gerr := b.geth.GetTransactionReceipt(ctx, tx.Hash())

		// fmt.Printf("\n\nreceipt %v\n\n\n\n", receipt)

		if gerr != nil {
			return 0, fmt.Errorf("[processTransactions] GetTransactionReceipt internal error: %w", gerr)
		}

		if receipt.Status == 0 || tx.Value() == nil {
			continue
		}
		// need to response err to prevent blocktransaction being created
		args, err := b.processData(ctx, t, tx, receipt, msg)
		if err != nil {
			return 0, fmt.Errorf("[processTransactions] processData internal error: %w", err)
		}
		// ?block_num = ? AND sender = ? AND recipient = ? AND type = ? AND token = ?

		var countTx int
		if err = b.db.GetContext(ctx, &countTx, bnbGetTxDataQuery, args.BlockNum, args.Sender, args.Recipient, args.Type, args.Token); err != nil {
			return 0, fmt.Errorf("[BNBNewBlockCrawler] internal error: %w", err)
		}

		fmt.Printf("\n\n Count is %d \n\n", countTx)
		// fmt.Printf("\n\n args.Token is %s \n\n", args.Token)

		if countTx == 0 {
			insertData := true
			rootWallet := b.rootWallet
			if args.Token == "BNB" && args.Sender == rootWallet {
				args.Type = string(bnbgasTx)
			}
			if args.Token == "BNB" && args.Sender != rootWallet {
				insertData = false
			}

			fmt.Printf("\n\n insertData is %v \n\n", insertData)

			if insertData {
				if err = b.createData(ctx, args); err != nil {
					return 0, fmt.Errorf("[processTransactions] createData internal error: %w", err)
				}
			}

		}
		total++
	}

	return total, nil
}

// set up all of user wallet
func (b *bnbblockCrawler) cacheCheck(ctx context.Context) error {
	redisLen, err := b.rds.HLen(ctx, string(bnbPrefix+bnbWalletKey)).Result()
	if err != nil {
		return fmt.Errorf("[cacheCheck] internal error: %w", err)
	}

	var dbLen int64
	if err = b.countBnbWalletStmt.GetContext(ctx, &dbLen); err != nil {
		return fmt.Errorf("[cacheCheck] internal error: %w", err)
	}

	if redisLen != dbLen {
		// if we don't remove the old one the cache only work when increase record, but not work with decrease record
		err := b.rds.Del(ctx, string(bnbPrefix+bnbWalletKey)).Err()
		if err != nil {
			fmt.Errorf("[cacheCheck] internal error: %w", err)
		}
		b.wallets = make(map[string]interface{})

		var wallets []entity.CSWallet
		if err = b.listAddressIndexStmt.SelectContext(ctx, &wallets); err != nil {
			return fmt.Errorf("[cacheCheck] internal error: %w", err)
		}

		for _, wallet := range wallets {
			b.wallets[wallet.Address] = wallet.Index
		}

		if _, err = b.rds.HSet(ctx, string(bnbPrefix+bnbWalletKey), b.wallets).Result(); err != nil {
			return fmt.Errorf("[cacheCheck] internal error: %w", err)
		}
	}

	return nil
}

func (b *bnbblockCrawler) updateBlockNum(num uint64) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.viper.Set("block", num)
	if err := b.viper.WriteConfig(); err != nil {
		return fmt.Errorf("[updateBlockNum] internal error: %w", err)
	}

	return nil
}

func (b *bnbblockCrawler) processData(
	ctx context.Context,
	t bnbtxType,
	tx *types.Transaction,
	receipt *types.Receipt,
	msg types.Message,
) (entity.CSBlockTransaction, error) {
	var token, recipient string
	var amount decimal.Decimal

	if len(tx.Data()) > 0 {
		inputData := hex.EncodeToString(tx.Data())

		params, err := b.decodeData(inputData)
		if err != nil || len(params) == 0 {
			return entity.CSBlockTransaction{}, fmt.Errorf("[processData] processData decodeData function internal error: %w", err)
		}

		recipient, amount = b.processBnb20Data(params)
		token = tools.USDTBNBToken
	} else {
		recipient = tx.To().Hex()
		amount = b.processNativeData(tx.Value()) //TODO
		token = tools.BNBCoin
	}
	sender := msg.From().Hex()
	if recipient == "" && reflect.DeepEqual(amount, decimal.Decimal{}) {
		return entity.CSBlockTransaction{},
			fmt.Errorf("[processData] internal error: %s", "not a valid transaction")
	}

	weiGasCost := tools.CalculateGasCost(new(big.Float).SetUint64(receipt.GasUsed), new(big.Float).SetInt(tx.GasPrice()))
	weiGasCostInDecimal, _ := decimal.NewFromString(weiGasCost.String())
	etherGasCost := tools.ToDecimal(weiGasCostInDecimal.String(), bnbDecimals)
	reconcile := false

	if recipient == b.rootWallet {
		t = bnbreconciliation
		reconcile = true
	}

	if token == tools.BNBCoin && b.bnbReconcileThreshold.GreaterThan(amount) {
		reconcile = true
	}

	if token == tools.USDTBNBToken && b.usdtReconcileThreshold.GreaterThan(amount) {
		reconcile = true
	}
	var detail entity.ASMemberVIPCryptoWithdrawDetail
	tranNo := ""
	if t == bnbwithdraw {
		reconcile = true
		getCryptoWithdrawDetailStmt, err := b.db.PreparexContext(ctx, bnbGetCryptoWithdrawDetailQuery)
		if err = getCryptoWithdrawDetailStmt.GetContext(ctx, &detail, tx.Hash().Hex()); err != nil {
			return entity.CSBlockTransaction{}, fmt.Errorf("[processData] bnbwithdraw internal error: %w", err)
		}
		tranNo = detail.TranNo
	} else {
		tranNo = randx.GenUniqueId()
	}

	idx, err := b.rds.HGet(ctx, string(bnbPrefix+bnbWalletKey), recipient).Int64()
	if errors.Is(err, redis.Nil) {
		if t == bnbreconciliation {
			return entity.CSBlockTransaction{}, fmt.Errorf("[processData] internal error: idx if %s", "index not found")
		}
	} else if err != nil {
		return entity.CSBlockTransaction{}, fmt.Errorf("[processData] internal error: idx else %w", err)
	}

	blockNumInt64 := b.viper.GetUint64("block")
	currentBlockNum := new(big.Int).SetUint64(blockNumInt64)

	// fmt.Printf("\n\nrecipient %s\n\n", recipient)

	if recipient == b.ph.BnbReconcileAddress {
		t = bnbreconciliation
		reconcile = true
	}

	wallet := entity.CSBlockTransaction{
		Type:        string(t),
		BlockNum:    currentBlockNum.Int64() + 1,
		Txn:         tx.Hash().Hex(),
		GasCost:     etherGasCost,
		WalletIndex: idx,
		Sender:      sender,
		Recipient:   recipient,
		Amount:      amount,
		Token:       token,
		IsReconcile: reconcile,
		TranNo:      tranNo,
	}

	return wallet, nil
}

// TO return WITHDRAW, DEPOSIT as string,
// return value only when it is USDT smart contract,
// return WITHDRAW when its recipient == root
func (b *bnbblockCrawler) getTransactionType(ctx context.Context, msg types.Message, tx *types.Transaction) bnbtxType {
	var t bnbtxType

	//check whether sender or reciptient is in db
	var recipient string
	if len(tx.Data()) > 0 {
		inputData := hex.EncodeToString(tx.Data())

		params, err := b.decodeData(inputData)
		// len(params) == 0 cause panic to below process
		if err != nil {
			return t
		}

		fmt.Printf("\n\n\n\n params %v", inputData)

		recipient, _ = b.processBnb20Data(params)
		//ignore self transfer
		if msg.From().Hex() == recipient {
			return t
		}
	} else {
		recipient = tx.To().Hex()
	}
	_, recipientErr := b.rds.HGet(ctx, string(bnbPrefix+bnbWalletKey), recipient).Int64()
	_, senderErr := b.rds.HGet(ctx, string(bnbPrefix+bnbWalletKey), msg.From().Hex()).Int64()

	if recipientErr != nil && senderErr != nil {
		return t
	}
	//end check whether sender or reciptient is in db

	_, err := b.rds.HGet(ctx, string(bnbPrefix+bnbWalletKey), tx.To().Hex()).Int64()
	// wallet is transaction is USDT smart contract(tx.To().Hex() is not recipient)
	if err == nil || tx.To().Hex() == b.cfg.Blockchain.BNBUSDTContract {
		t = bnbdeposit
	}

	if msg.From().Hex() == b.rootWallet {
		t = bnbwithdraw
	}

	return t
}

func (b *bnbblockCrawler) getMessage(tx *types.Transaction) (types.Message, error) {
	var chainID *big.Int
	if b.env == "prod" {
		chainID = big.NewInt(bnb20MainnetChainID)
	} else {
		chainID = big.NewInt(bnb20TestNetChainID)
	}

	msg, terr := tx.AsMessage(types.LatestSignerForChainID(chainID), tx.GasPrice())
	if terr != nil {
		return types.Message{}, fmt.Errorf("[processTransactions] getMessage function internal error: %w", terr)
	}

	return msg, nil
}

func (b *bnbblockCrawler) processBnb20Data(params []interface{}) (string, decimal.Decimal) {
	var recipient string
	var amount decimal.Decimal
	// bep20 transaction
	// ignore address that transfer to the contract
	fmt.Printf("\n\n\n\n BNBUSDTContract %v\n\n\n", b.cfg.Blockchain.BNBUSDTContract)
	fmt.Printf("\n\n\n\n params[0].(common.Address).String() %v\n\n\n", params[0].(common.Address).String())
	if params[0].(common.Address).String() == b.cfg.Blockchain.BNBUSDTContract {
		return "", decimal.Decimal{}
	}

	amount = tools.ToDecimal(params[1].(*big.Int), bnbusdtDecimals)
	amount = amount.Truncate(bnbTruncateDecimal)
	recipient = params[0].(common.Address).String()

	return recipient, amount
}

func (b *bnbblockCrawler) processNativeData(value *big.Int) decimal.Decimal {
	// eth native transaction
	amount := tools.ToDecimal(value, bnbDecimals)
	amount = amount.Truncate(bnbTruncateDecimal)

	return amount
}

func (b *bnbblockCrawler) decodeData(inputData string) ([]interface{}, error) {
	var tokenAbi abi.ABI
	if reflect.DeepEqual(tokenAbi, abi.ABI{}) {
		reader, err := os.Open(bnbpath)
		if err != nil {
			return make([]interface{}, 0), err
		}
		defer reader.Close()

		tokenAbi, err = abi.JSON(reader)
		if err != nil {
			return make([]interface{}, 0), err
		}
	}

	decodedData, err := hex.DecodeString(inputData)

	if err != nil || len(decodedData) < 4 {
		return make([]interface{}, 0), err
	}
	var params []interface{}
	if method, ok := tokenAbi.Methods["transfer"]; ok {
		params, err = method.Inputs.Unpack(decodedData[4:])
		if err != nil {
			return make([]interface{}, 0), err
		}
	}

	return params, nil
}

func (b *bnbblockCrawler) createData(ctx context.Context, blockTx entity.CSBlockTransaction) error {
	fmt.Printf("\n\nblockTx.Amount is %v\n\n", blockTx.Amount)
	if blockTx.Amount.GreaterThan(decimal.NewFromInt(0)) {
		if _, err := b.createBlockTxStmt.ExecContext(ctx, blockTx); err != nil {
			return fmt.Errorf("[createData] internal error: %w", err)
		}
	}

	return nil
}

func (b *bnbblockCrawler) MarshalBinary() ([]byte, error) {
	return json.Marshal(b.wallets)
}

func (b *bnbblockCrawler) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, &b.wallets)
}

type PushNotificationRequest struct {
	UserId     int64
	CardNumber string
	Content    []Notification
}

type SendNotifyPersonal struct {
	CardNumber string         `json:"cardNumber"`
	Content    []Notification `json:"notification"`
	UserId     int64          `json:"userId"`
	Token      string         `json:"token"`
}
type Notification struct {
	Description  string   `json:"description"`
	ImageGallery []string `json:"imageGallery"`
	Language     string   `json:"language"`
	Title        string   `json:"title"`
}
