package models

import "time"

const (
	PluginDeploymentOperationHotReload = "hot_reload"
	PluginDeploymentOperationHotUpdate = "hot_update"
	PluginDeploymentOperationInstall   = "install"
	PluginDeploymentOperationRollback  = "rollback"
	PluginDeploymentOperationStart     = "start"
	PluginDeploymentOperationPause     = "pause"
	PluginDeploymentOperationRestart   = "restart"
	PluginDeploymentOperationRetire    = "retire"

	PluginDeploymentStatusPending    = "pending"
	PluginDeploymentStatusRunning    = "running"
	PluginDeploymentStatusSucceeded  = "succeeded"
	PluginDeploymentStatusFailed     = "failed"
	PluginDeploymentStatusRolledBack = "rolled_back"
)

type PluginDeployment struct {
	ID                    uint           `gorm:"primaryKey" json:"id"`
	PluginID              uint           `gorm:"not null;index;index:idx_plugin_deployments_plugin_created_at,priority:1" json:"plugin_id"`
	Plugin                *Plugin        `gorm:"foreignKey:PluginID" json:"plugin,omitempty"`
	Operation             string         `gorm:"size:40;not null;index" json:"operation"`
	Trigger               string         `gorm:"size:80;index" json:"trigger"`
	Status                string         `gorm:"size:30;not null;index" json:"status"`
	TargetVersionID       *uint          `gorm:"index" json:"target_version_id,omitempty"`
	TargetVersion         *PluginVersion `gorm:"foreignKey:TargetVersionID;constraint:OnDelete:SET NULL;" json:"target_version,omitempty"`
	MarketSourceID        string         `gorm:"size:100;index:idx_plugin_deployments_market_coords,priority:1" json:"market_source_id,omitempty"`
	MarketArtifactKind    string         `gorm:"size:40;index:idx_plugin_deployments_market_coords,priority:2" json:"market_artifact_kind,omitempty"`
	MarketArtifactName    string         `gorm:"size:191;index:idx_plugin_deployments_market_coords,priority:3" json:"market_artifact_name,omitempty"`
	MarketArtifactVersion string         `gorm:"size:50;index:idx_plugin_deployments_market_coords,priority:4" json:"market_artifact_version,omitempty"`
	RequestedGeneration   uint           `gorm:"default:1" json:"requested_generation"`
	AppliedGeneration     uint           `gorm:"default:0" json:"applied_generation"`
	RuntimeSpecHash       string         `gorm:"size:64" json:"runtime_spec_hash"`
	AutoStart             bool           `gorm:"default:false" json:"auto_start"`
	RequestedBy           *uint          `gorm:"index" json:"requested_by,omitempty"`
	Detail                string         `gorm:"type:text" json:"detail"`
	Error                 string         `gorm:"type:text" json:"error"`
	StartedAt             *time.Time     `json:"started_at,omitempty"`
	FinishedAt            *time.Time     `json:"finished_at,omitempty"`
	CreatedAt             time.Time      `gorm:"index:idx_plugin_deployments_plugin_created_at,priority:2" json:"created_at"`
	UpdatedAt             time.Time      `json:"updated_at"`
}

func (PluginDeployment) TableName() string {
	return "plugin_deployments"
}
