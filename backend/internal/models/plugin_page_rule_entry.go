package models

import "time"

// PluginPageRuleEntry stores plugin-owned page inject rules.
type PluginPageRuleEntry struct {
	ID        uint    `gorm:"primaryKey" json:"id"`
	PluginID  uint    `gorm:"not null;index;uniqueIndex:uidx_plugin_page_rule_entries_plugin_key,priority:1" json:"plugin_id"`
	Plugin    *Plugin `gorm:"foreignKey:PluginID;constraint:OnDelete:CASCADE" json:"plugin,omitempty"`
	Key       string  `gorm:"type:varchar(191);not null;uniqueIndex:uidx_plugin_page_rule_entries_plugin_key,priority:2" json:"key"`
	Name      string  `gorm:"size:200" json:"name"`
	Pattern   string  `gorm:"type:text;not null" json:"pattern"`
	MatchType string  `gorm:"size:16;not null" json:"match_type"`
	CSS       string  `gorm:"type:text" json:"css"`
	JS        string  `gorm:"type:text" json:"js"`
	Enabled   bool    `gorm:"default:true;index" json:"enabled"`
	Priority  int     `gorm:"default:100;index" json:"priority"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName specifies the DB table for plugin page rules.
func (PluginPageRuleEntry) TableName() string {
	return "plugin_page_rule_entries"
}
