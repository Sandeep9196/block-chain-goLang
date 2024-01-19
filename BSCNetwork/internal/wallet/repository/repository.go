package repository

import (
	"context"

	"bsc_network/internal/config/log"
	"bsc_network/internal/entity"

	"github.com/jmoiron/sqlx"
)

type (
	// Repository encapsulates the logic to access wallet from the data source.
	Repository interface {
		GetBNBWallet(ctx context.Context, index int64) (string, error)
		CreateWallet(ctx context.Context, wallet entity.CSWallet) (int64, error)
	}
	// repository persists wallet in database.
	repository struct {
		db     *sqlx.DB
		logger log.Logger
	}
)

const (
	getBNBWalletQuery string = `SELECT address FROM wallet WHERE ` + "`index`" + ` = ? AND network = 'BNB'`
	createWalletQuery string = `INSERT INTO wallet(address, ` + "`index`" +
		`, network) VALUES(:address, :index, :network)`
)

func New(db *sqlx.DB, logger log.Logger) Repository {
	return repository{db, logger}
}

func (r repository) GetBNBWallet(ctx context.Context, index int64) (string, error) {
	getBNBWalletStmt, err := r.db.PreparexContext(ctx, getBNBWalletQuery)
	if err != nil {
		return "", err
	}
	defer getBNBWalletStmt.Close()

	var address string
	if err = getBNBWalletStmt.GetContext(ctx, &address, index); err != nil {
		return "", err
	}

	return address, nil
}

func (r repository) CreateWallet(ctx context.Context, wallet entity.CSWallet) (int64, error) {
	createWalletStmt, err := r.db.PrepareNamedContext(ctx, createWalletQuery)
	if err != nil {
		return 0, err
	}
	defer createWalletStmt.Close()

	result, err := createWalletStmt.ExecContext(ctx, &wallet)
	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}
