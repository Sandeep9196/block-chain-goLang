package entity

import (
	"database/sql"

	"github.com/shopspring/decimal"
)

type ASMemberVIPCryptoWithdrawDetail struct {
	ID        int64           `db:"id" json:"id"`
	RequestID int64           `db:"as_member_vip_request_id" json:"as_member_vip_request_id"`
	Amount    decimal.Decimal `db:"amount" json:"amount"`
	Recipient string          `db:"recipient" json:"recipient"`
	Fee       decimal.Decimal `db:"fee" json:"fee"`
	Txn       sql.NullString  `db:"txn" json:"txn"`
	Token     string          `db:"token" json:"token"`
	TranNo    string          `db:"tran_no" json:"tran_no"`
	Sender    string          `db:"sender" json:"sender"`
}
