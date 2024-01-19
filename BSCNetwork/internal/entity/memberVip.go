package entity

import "database/sql"

type MemberVip struct {
	ID         int64          `db:"id" json:"id"`
	UserId     int64          `db:"user_id" json:"user_id"`
	CardNumber string         `db:"card_number" json:"card_number"`
	MemberName sql.NullString `db:"member_name" json:"member_name"`
}
