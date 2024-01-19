package entity

import (
	"database/sql"
	"time"
)

type ASCurrency struct {
	ID        int64        `db:"id" json:"id"`
	Code      string       `db:"code" json:"code"`
	IsActive  bool         `db:"is_active" json:"is_active"`
	CreatedAt time.Time    `db:"created_at" json:"created_at"`
	UpdatedAt sql.NullTime `db:"updated_at" json:"updated_at"`
}
