package models

import "time"

// PaymentMethodStorageEntry stores persistent key/value data for payment scripts.
type PaymentMethodStorageEntry struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	PaymentMethodID uint      `gorm:"not null;index;uniqueIndex:uidx_payment_method_storage_entries_pm_key,priority:1" json:"payment_method_id"`
	Key             string    `gorm:"type:varchar(191);not null;uniqueIndex:uidx_payment_method_storage_entries_pm_key,priority:2" json:"key"`
	Value           string    `gorm:"type:text;not null" json:"value"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// TableName specifies the DB table for payment script storage.
func (PaymentMethodStorageEntry) TableName() string {
	return "payment_method_storage_entries"
}
