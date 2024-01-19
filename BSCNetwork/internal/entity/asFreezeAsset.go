package entity

import (
	"database/sql"
	"time"

	"github.com/shopspring/decimal"
)

type ASFreezeAsset struct {
	ID          int64           `db:"id" json:"id"`
	MemberID    int64           `db:"as_member_vip_id" json:"member_id"`
	CurrencyID  int64           `db:"currency_id" json:"currency_id"`
	Amount      decimal.Decimal `db:"amount" json:"amount"`
	Description string          `db:"description" json:"description"`
	CreatedAt   time.Time       `db:"created_at" json:"created_at"`
	CreatedBy   sql.NullInt64   `db:"created_by" json:"created_by"`
	UnfrozeAt   sql.NullTime    `db:"unfroze_at" json:"unfroze_at"`
	UnfrozeBy   sql.NullInt64   `db:"unfroze_by" json:"unfroze_by"`
	TranNo      string          `db:"tran_no" json:"tran_no"`
}
