package models

import "time"

// VirtualInventoryStorageEntry stores persistent key/value data for virtual inventory scripts.
type VirtualInventoryStorageEntry struct {
	ID                 uint      `gorm:"primaryKey" json:"id"`
	VirtualInventoryID uint      `gorm:"not null;index;uniqueIndex:uidx_virtual_inventory_storage_entries_inv_key,priority:1" json:"virtual_inventory_id"`
	Key                string    `gorm:"type:varchar(191);not null;uniqueIndex:uidx_virtual_inventory_storage_entries_inv_key,priority:2" json:"key"`
	Value              string    `gorm:"type:text;not null" json:"value"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// TableName specifies the DB table for virtual inventory script storage.
func (VirtualInventoryStorageEntry) TableName() string {
	return "virtual_inventory_storage_entries"
}
