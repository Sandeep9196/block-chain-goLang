package entity

import "time"

type CSWallet struct {
	ID        int64     `db:"id" json:"id"`
	Address   string    `db:"address" json:"address"`
	Index     int64     `db:"index" json:"index"`
	Network   string    `db:"network" json:"network"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}
