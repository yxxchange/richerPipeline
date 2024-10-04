package models

type Model struct {
	Id        int64 `gorm:"primary_key;auto_increment" json:"id"`
	CreatedAt int64 `gorm:"created_at" json:"created_at"`
	UpdatedAt int64 `gorm:"updated_at" json:"updated_at"`
	IsDel     int   `gorm:"is_del" json:"is_del"`
}
