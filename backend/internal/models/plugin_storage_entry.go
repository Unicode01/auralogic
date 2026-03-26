package models

import "time"

// PluginStorageEntry stores persistent key/value data for js_worker plugins.
type PluginStorageEntry struct {
	ID       uint    `gorm:"primaryKey" json:"id"`
	PluginID uint    `gorm:"not null;index;uniqueIndex:uidx_plugin_storage_entries_plugin_key,priority:1" json:"plugin_id"`
	Plugin   *Plugin `gorm:"foreignKey:PluginID;constraint:OnDelete:CASCADE" json:"plugin,omitempty"`
	Key      string  `gorm:"type:varchar(191);not null;uniqueIndex:uidx_plugin_storage_entries_plugin_key,priority:2" json:"key"`
	Value    string  `gorm:"type:text;not null" json:"value"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName specifies the DB table for plugin storage.
func (PluginStorageEntry) TableName() string {
	return "plugin_storage_entries"
}
