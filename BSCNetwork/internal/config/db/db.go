package db

import (
	"bsc_network/internal/config"
	"context"
	"time"

	// postgres driver required by database/sql.
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

const (
	maxOpenConns   int           = 25
	maxIdleConns   int           = 25
	maxLifetime    time.Duration = 5 * time.Minute
	contextTimeout time.Duration = 5 * time.Second
)

func Connect(ctx context.Context, cfg config.Config) (*sqlx.DB, error) {
	db, err := sqlx.Open("mysql", cfg.Dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxLifetime(maxLifetime)

	ctx, cancel := context.WithTimeout(ctx, contextTimeout)
	defer cancel()

	err = db.PingContext(ctx)
	if err != nil {
		return nil, err
	}

	return db, nil
}
