package models

import "time"

// PluginSecretEntry stores sensitive key/value data for plugins outside config/runtime_params.
type PluginSecretEntry struct {
	ID       uint    `gorm:"primaryKey" json:"id"`
	PluginID uint    `gorm:"not null;index;uniqueIndex:uidx_plugin_secret_entries_plugin_key,priority:1" json:"plugin_id"`
	Plugin   *Plugin `gorm:"foreignKey:PluginID;constraint:OnDelete:CASCADE" json:"plugin,omitempty"`
	Key      string  `gorm:"type:varchar(191);not null;uniqueIndex:uidx_plugin_secret_entries_plugin_key,priority:2" json:"key"`
	Value    string  `gorm:"type:text;not null" json:"-"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName specifies the DB table for plugin secrets.
func (PluginSecretEntry) TableName() string {
	return "plugin_secret_entries"
}
