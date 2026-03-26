package service

import (
	"auralogic/internal/config"
	"gorm.io/gorm"
)

type PluginHostRuntime struct {
	DB            *gorm.DB
	Config        *config.Config
	PluginManager *PluginManagerService
}

func NewPluginHostRuntime(db *gorm.DB, cfg *config.Config, pluginManager *PluginManagerService) *PluginHostRuntime {
	return &PluginHostRuntime{
		DB:            db,
		Config:        cfg,
		PluginManager: pluginManager,
	}
}

func (r *PluginHostRuntime) database() *gorm.DB {
	if r == nil {
		return nil
	}
	return r.DB
}
