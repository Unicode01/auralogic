package models

import "time"

// UserPurchaseStat stores per-user purchase quantities per SKU for fast limit checks.
type UserPurchaseStat struct {
	UserID   uint   `gorm:"primaryKey;autoIncrement:false" json:"user_id"`
	SKU      string `gorm:"primaryKey;type:varchar(255)" json:"sku"`
	Quantity int64  `gorm:"type:bigint;default:0" json:"quantity"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (UserPurchaseStat) TableName() string {
	return "user_purchase_stats"
}
