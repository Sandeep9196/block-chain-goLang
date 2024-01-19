package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"bsc_network/internal/config/log"
	"bsc_network/internal/entity"
	errs "bsc_network/internal/errors"
	"bsc_network/internal/geth"
	"bsc_network/internal/tools"
	"bsc_network/internal/wallet/repository"

	hdw "bsc_network/internal/hdwallet"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

type (
	// Service encapsulates usecase logic for wallet.
	Service interface {
		GetBNBWallet(ctx context.Context, req GetWalletRequest) (string, error)
		ListWallet(ctx context.Context, req GetWalletRequest) ([]string, error)
		TransferCrypto(ctx context.Context, req TransferCryptoRequest) error
	}

	service struct {
		hdWallet *hdw.Wallet
		geth     geth.Geth
		db       *sqlx.DB
		repo     repository.Repository
		logger   log.Logger
		timeout  time.Duration
	}

	GetWalletRequest struct {
		MemberID int64 `param:"member_id" validate:"required"`
	}

	TransferCryptoRequest struct {
		RequestID int64 `json:"request_id" validate:"required"`
	}

	CoinType   string
	ChangeType string
)

const (
	getCryptoWithdrawDetailQuery string = `SELECT * FROM member_request_details 
  WHERE as_member_vip_request_id = ?`
	updateCryptoWithdrawDetailQuery string = `UPDATE member_request_details 
  SET txn = ? WHERE as_member_vip_request_id = ?`
)

// NewService creates a new wallet service.
func New(
	hdWallet *hdw.Wallet,
	geth geth.Geth,
	db *sqlx.DB,
	repo repository.Repository,
	logger log.Logger,
	timeout time.Duration,
) Service {
	return service{hdWallet, geth, db, repo, logger, timeout}
}

func (s service) GetBNBWallet(ctx context.Context, req GetWalletRequest) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	address, err := s.repo.GetBNBWallet(ctx, req.MemberID)
	if errors.Is(err, sql.ErrNoRows) {
		var account accounts.Account
		derivationPath := tools.GetDerivationPath(req.MemberID)

		account, err = s.hdWallet.Derive(hdw.MustParseDerivationPath(derivationPath), false)
		if err != nil {
			return "", fmt.Errorf("[GetBNBWallet] internal error: %w", err)
		}

		wallet := entity.CSWallet{
			Address: account.Address.Hex(),
			Index:   req.MemberID,
			Network: "BNB",
		}
		if _, err = s.repo.CreateWallet(ctx, wallet); err != nil {
			return "", fmt.Errorf("[GetBNBWallet] internal error: %w", err)
		}

		return account.Address.Hex(), nil
	} else if err != nil {
		return "", fmt.Errorf("[GetBNBWallet] internal error: %w", err)
	}

	return address, nil
}

func (s service) ListWallet(ctx context.Context, req GetWalletRequest) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	bnbWallet, err := s.GetBNBWallet(ctx, req)
	if err != nil {
		return []string{}, fmt.Errorf("[ListWallet] internal error: %w", err)
	}

	return []string{bnbWallet}, nil
}

func (s service) TransferCrypto(ctx context.Context, req TransferCryptoRequest) error {
	getCryptoWithdrawDetailStmt, err := s.db.PreparexContext(ctx, getCryptoWithdrawDetailQuery)
	if err != nil {
		return fmt.Errorf("[TransferCrypto] internal error: %w", err)
	}

	var detail entity.ASMemberVIPCryptoWithdrawDetail
	if err = getCryptoWithdrawDetailStmt.GetContext(ctx, &detail, req.RequestID); err != nil {
		return fmt.Errorf("[TransferCrypto] internal error: %w", err)
	}

	if detail.Txn.String != "" {
		return errs.ErrRequestProcessed
	}

	go func() {
		gCtx := context.Background()
		txn := ""
		if detail.Token == "USDT-BNB" {
			bnbderivationPath := tools.GetDerivationPath(tools.HDWalletDefaultIndex)
			bnbrootAccount, err := s.hdWallet.Derive(hdw.MustParseDerivationPath(bnbderivationPath), false)
			if err != nil {
				s.logger.Error("Fail to derive bnb account", zap.Error(err))
				return
			}

			bnbPk, err := s.hdWallet.PrivateKeyBytes(bnbrootAccount)
			if err != nil {
				s.logger.Error("Fail to derive private key", zap.Error(err))
				return
			}
			nbnAmount := tools.ToWei(detail.Amount, s.geth.BNBDecimals())
			// Create a context with a timeout, cancel function, or deadline.
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel()

			bnbGasPrice, err := s.geth.SuggestGasPrice(ctx)
			if err != nil {
				s.logger.Error("Fail to SuggestGasPrice", zap.Error(err))
				return
			}

			txn, err = s.geth.TransferUSDT(gCtx, bnbPk, detail.Recipient, nbnAmount, bnbGasPrice, tools.USDTBNBToken)
			if err != nil {
				s.logger.Error("Fail to transfer BNB USDT", zap.Error(err))
				return
			}
		}
		// end section
		updateCryptoWithdrawDetailStmt, err := s.db.PreparexContext(gCtx, updateCryptoWithdrawDetailQuery)
		if err != nil {
			s.logger.Errorf("[TransferCrypto] internal error: %w", err)
		}
		if _, err = updateCryptoWithdrawDetailStmt.ExecContext(gCtx, txn, req.RequestID); err != nil {
			s.logger.Errorf("[TransferCrypto] internal error: %w", err)
		}
	}()

	return nil
}
