package entity

import (
	"database/sql"
	"time"

	"github.com/shopspring/decimal"
)

type CSBlockTransaction struct {
	ID            int64           `db:"id" json:"id"`
	Type          string          `db:"type" json:"type"`
	BlockNum      int64           `db:"block_num" json:"block_num"`
	Txn           string          `db:"txn" json:"txn"`
	GasCost       decimal.Decimal `db:"gas_cost" json:"gas_cost"`
	WalletIndex   int64           `db:"wallet_index" json:"wallet_index"`
	Sender        string          `db:"sender" json:"sender"`
	Recipient     string          `db:"recipient" json:"recipient"`
	Amount        decimal.Decimal `db:"amount" json:"amount"`
	Token         string          `db:"token" json:"token"`
	IsReconcile   bool            `db:"is_reconcile" json:"is_reconcile"`
	IsProcess     bool            `db:"is_process" json:"is_process"`
	IsGasTransfer bool            `db:"is_gas_transfer" json:"is_gas_transfer"`
	TranNo        string          `db:"tran_no" json:"tran_no"`
	CreatedAt     time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt     sql.NullTime    `db:"updated_at" json:"updated_at"`
}
