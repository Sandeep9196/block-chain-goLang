package entity

type TsConstant struct {
	Value    string `db:"value" json:"value"`
	Label    string `db:"label" json:"label"`
	Category string `db:"category" json:"category"`
	Tag      string `db:"tag" json:"tag"`
}
