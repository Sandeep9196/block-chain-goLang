package entity

import (
	"database/sql"

	"github.com/shopspring/decimal"
)

type (
	TxType string
	Change string

	ASAsset struct {
		ID           int64               `db:"id" json:"id"`
		UserID       int64               `db:"user_id" json:"user_id"`
		CurrencyID   int64               `db:"currency_id" json:"currency_id"`
		ExchangeRate decimal.NullDecimal `db:"exchange_rate" json:"exchange_rate"`
		TxAmount     decimal.Decimal     `db:"tx_amount" json:"tx_amount"`
		BeforeAmount decimal.Decimal     `db:"before_amount" json:"before_amount"`
		AfterAmount  decimal.Decimal     `db:"after_amount" json:"after_amount"`
		TxType       TxType              `db:"tx_type" json:"tx_type"`
		Change       Change              `db:"change" json:"change"`
		Tag          sql.NullString      `db:"tag" json:"tag"`
		Description  sql.NullString      `db:"description" json:"description"`
		TranNo       string              `db:"tran_no" json:"tran_no"`
		Remark       sql.NullString      `db:"remark" json:"remark"`
		CreatedAt    string              `db:"created_at" json:"created_at"`
		CreatedBy    sql.NullInt32       `db:"created_by" json:"created_by"`
	}
)

const (
	Transfer       TxType = "TRANSFER"
	CryptoDeposit  TxType = "CRYPTO_DEPOSIT"
	CryptoWithdraw TxType = "CRYPTO_WITHDRAW"

	Increase Change = "INCREASE"
	Decrease Change = "DECREASE"
)
