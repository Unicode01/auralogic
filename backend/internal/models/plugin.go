package models

import "time"

const (
	PluginLifecycleDraft     = "draft"
	PluginLifecycleUploaded  = "uploaded"
	PluginLifecycleInstalled = "installed"
	PluginLifecycleRunning   = "running"
	PluginLifecyclePaused    = "paused"
	PluginLifecycleDegraded  = "degraded"
	PluginLifecycleRetired   = "retired"
)

// Plugin 插件配置
type Plugin struct {
	ID                uint       `gorm:"primaryKey" json:"id"`
	Name              string     `gorm:"size:100;not null;uniqueIndex" json:"name"`
	DisplayName       string     `gorm:"size:200" json:"display_name"`
	Description       string     `gorm:"type:text" json:"description"`
	Type              string     `gorm:"size:50;not null" json:"type"` // ai_chat, data_analysis, custom
	Runtime           string     `gorm:"size:30;default:'grpc';index" json:"runtime"`
	Address           string     `gorm:"size:500;not null" json:"address"` // gRPC 地址，如 localhost:50051
	Version           string     `gorm:"size:50;default:'0.0.0'" json:"version"`
	Config            string     `gorm:"type:text" json:"config"` // JSON 配置
	RuntimeParams     string     `gorm:"type:text" json:"runtime_params"`
	Capabilities      string     `gorm:"type:text" json:"capabilities"` // JSON 能力/安全策略
	Manifest          string     `gorm:"type:text" json:"manifest"`     // 原始 manifest JSON
	PackagePath       string     `gorm:"size:1000" json:"package_path"`
	PackageChecksum   string     `gorm:"size:128" json:"package_checksum"`
	RuntimeSpecHash   string     `gorm:"size:64;default:''" json:"runtime_spec_hash"`
	DesiredGeneration uint       `gorm:"default:1" json:"desired_generation"`
	AppliedGeneration uint       `gorm:"default:1" json:"applied_generation"`
	Enabled           bool       `gorm:"default:true" json:"enabled"`
	Status            string     `gorm:"size:20;default:'unknown'" json:"status"`               // healthy, unhealthy, unknown
	LifecycleStatus   string     `gorm:"size:30;default:'draft';index" json:"lifecycle_status"` // draft, uploaded, installed, running, paused, degraded, retired
	LastError         string     `gorm:"type:text" json:"last_error"`
	LastHealthy       *time.Time `json:"last_healthy"`
	InstalledAt       *time.Time `json:"installed_at"`
	StartedAt         *time.Time `json:"started_at"`
	StoppedAt         *time.Time `json:"stopped_at"`
	RetiredAt         *time.Time `json:"retired_at"`
	FailCount         int        `gorm:"default:0" json:"fail_count"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// PluginExecution 插件执行记录
type PluginExecution struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	PluginID       uint      `gorm:"not null;index;index:idx_plugin_executions_plugin_created_at,priority:1;index:idx_plugin_executions_plugin_success_created_at,priority:1;index:idx_plugin_executions_plugin_hook_created_at,priority:1;index:idx_plugin_executions_plugin_error_signature_created_at,priority:1" json:"plugin_id"`
	Plugin         *Plugin   `gorm:"foreignKey:PluginID" json:"plugin,omitempty"`
	UserID         *uint     `gorm:"index" json:"user_id"`
	OrderID        *uint     `gorm:"index" json:"order_id"`
	Action         string    `gorm:"size:100;not null" json:"action"`
	Hook           string    `gorm:"size:191;index:idx_plugin_executions_plugin_hook_created_at,priority:3" json:"hook,omitempty"`
	Params         string    `gorm:"type:text" json:"params"` // JSON
	Metadata       JSONMap   `gorm:"type:text" json:"metadata,omitempty"`
	Success        bool      `gorm:"index:idx_plugin_executions_plugin_success_created_at,priority:2" json:"success"`
	ErrorSignature string    `gorm:"size:191;index:idx_plugin_executions_plugin_error_signature_created_at,priority:3" json:"error_signature,omitempty"`
	Result         string    `gorm:"type:text" json:"result"` // JSON
	Error          string    `gorm:"type:text" json:"error"`
	Duration       int       `json:"duration"` // 毫秒
	CreatedAt      time.Time `gorm:"index:idx_plugin_executions_created_at;index:idx_plugin_executions_plugin_created_at,priority:2;index:idx_plugin_executions_plugin_success_created_at,priority:3;index:idx_plugin_executions_plugin_hook_created_at,priority:2;index:idx_plugin_executions_plugin_error_signature_created_at,priority:2" json:"created_at"`
}

// PluginVersion 插件包版本记录
type PluginVersion struct {
	ID                    uint       `gorm:"primaryKey" json:"id"`
	PluginID              uint       `gorm:"not null;index" json:"plugin_id"`
	Plugin                *Plugin    `gorm:"foreignKey:PluginID" json:"plugin,omitempty"`
	Version               string     `gorm:"size:50;not null" json:"version"`
	MarketSourceID        string     `gorm:"size:100;index:idx_plugin_versions_market_coords,priority:1" json:"market_source_id,omitempty"`
	MarketArtifactKind    string     `gorm:"size:40;index:idx_plugin_versions_market_coords,priority:2" json:"market_artifact_kind,omitempty"`
	MarketArtifactName    string     `gorm:"size:191;index:idx_plugin_versions_market_coords,priority:3" json:"market_artifact_name,omitempty"`
	MarketArtifactVersion string     `gorm:"size:50;index:idx_plugin_versions_market_coords,priority:4" json:"market_artifact_version,omitempty"`
	PackageName           string     `gorm:"size:255" json:"package_name"`
	PackagePath           string     `gorm:"size:1000" json:"package_path"`
	PackageChecksum       string     `gorm:"size:128" json:"package_checksum"`
	Manifest              string     `gorm:"type:text" json:"manifest"` // JSON
	Type                  string     `gorm:"size:50" json:"type"`
	Runtime               string     `gorm:"size:30" json:"runtime"`
	Address               string     `gorm:"size:500" json:"address"`
	ConfigSnapshot        string     `gorm:"type:text" json:"config_snapshot"` // JSON
	RuntimeParams         string     `gorm:"type:text" json:"runtime_params"`
	CapabilitiesSnapshot  string     `gorm:"type:text" json:"capabilities_snapshot"` // JSON
	Changelog             string     `gorm:"type:text" json:"changelog"`
	LifecycleStatus       string     `gorm:"size:30;default:'uploaded'" json:"lifecycle_status"`
	IsActive              bool       `gorm:"default:false;index" json:"is_active"`
	UploadedBy            *uint      `gorm:"index" json:"uploaded_by"`
	ActivatedAt           *time.Time `json:"activated_at"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

func (Plugin) TableName() string {
	return "plugins"
}

func (PluginExecution) TableName() string {
	return "plugin_executions"
}

func (PluginVersion) TableName() string {
	return "plugin_versions"
}
