package models

import "time"

type PaymentMethodVersion struct {
	ID                    uint              `gorm:"primaryKey" json:"id"`
	PaymentMethodID       uint              `gorm:"not null;index;index:idx_payment_method_versions_method_created_at,priority:1" json:"payment_method_id"`
	PaymentMethod         *PaymentMethod    `gorm:"foreignKey:PaymentMethodID;constraint:OnDelete:CASCADE;" json:"payment_method,omitempty"`
	Version               string            `gorm:"size:50;not null" json:"version"`
	Type                  PaymentMethodType `gorm:"size:20;not null;default:custom" json:"type"`
	NameSnapshot          string            `gorm:"size:100;not null" json:"name_snapshot"`
	DescriptionSnapshot   string            `gorm:"size:500" json:"description_snapshot"`
	Icon                  string            `gorm:"size:100" json:"icon"`
	ScriptSnapshot        string            `gorm:"type:text" json:"script_snapshot"`
	ConfigSnapshot        string            `gorm:"type:text" json:"config_snapshot"`
	Manifest              string            `gorm:"type:text" json:"manifest"`
	PackageName           string            `gorm:"size:255" json:"package_name"`
	PackageEntry          string            `gorm:"size:255" json:"package_entry"`
	PackageChecksum       string            `gorm:"size:128" json:"package_checksum"`
	PollInterval          int               `gorm:"default:30" json:"poll_interval"`
	Enabled               bool              `gorm:"default:true" json:"enabled"`
	MarketSourceID        string            `gorm:"size:100;index:idx_payment_method_versions_market_coords,priority:1" json:"market_source_id,omitempty"`
	MarketArtifactKind    string            `gorm:"size:40;index:idx_payment_method_versions_market_coords,priority:2" json:"market_artifact_kind,omitempty"`
	MarketArtifactName    string            `gorm:"size:191;index:idx_payment_method_versions_market_coords,priority:3" json:"market_artifact_name,omitempty"`
	MarketArtifactVersion string            `gorm:"size:50;index:idx_payment_method_versions_market_coords,priority:4" json:"market_artifact_version,omitempty"`
	ImportedBy            *uint             `gorm:"index" json:"imported_by,omitempty"`
	IsActive              bool              `gorm:"default:false;index" json:"is_active"`
	ActivatedAt           *time.Time        `json:"activated_at,omitempty"`
	CreatedAt             time.Time         `gorm:"index:idx_payment_method_versions_method_created_at,priority:2" json:"created_at"`
	UpdatedAt             time.Time         `json:"updated_at"`
}

func (PaymentMethodVersion) TableName() string {
	return "payment_method_versions"
}
