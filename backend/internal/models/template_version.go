package models

import "time"

type TemplateVersion struct {
	ID                    uint       `gorm:"primaryKey" json:"id"`
	ResourceKind          string     `gorm:"size:40;not null;index:idx_template_versions_target_active,priority:1;index:idx_template_versions_market_coords,priority:1" json:"resource_kind"`
	TargetKey             string     `gorm:"size:191;not null;index:idx_template_versions_target_active,priority:2;index:idx_template_versions_market_coords,priority:2" json:"target_key"`
	ContentSnapshot       string     `gorm:"type:text" json:"content_snapshot"`
	ContentDigest         string     `gorm:"size:64" json:"content_digest"`
	MarketSourceID        string     `gorm:"size:100;index:idx_template_versions_market_coords,priority:3" json:"market_source_id,omitempty"`
	MarketArtifactKind    string     `gorm:"size:40;index:idx_template_versions_market_coords,priority:4" json:"market_artifact_kind,omitempty"`
	MarketArtifactName    string     `gorm:"size:191;index:idx_template_versions_market_coords,priority:5" json:"market_artifact_name,omitempty"`
	MarketArtifactVersion string     `gorm:"size:50;index:idx_template_versions_market_coords,priority:6" json:"market_artifact_version,omitempty"`
	ImportedBy            *uint      `gorm:"index" json:"imported_by,omitempty"`
	IsActive              bool       `gorm:"default:false;index:idx_template_versions_target_active,priority:3" json:"is_active"`
	ActivatedAt           *time.Time `json:"activated_at,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

func (TemplateVersion) TableName() string {
	return "template_versions"
}
