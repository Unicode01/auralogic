package service

import (
	"testing"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestUpdatePluginLifecycleSyncsActiveVersion(t *testing.T) {
	dsn := "file:plugin-manager-lifecycle-" + time.Now().UTC().Format("20060102150405.000000000") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&models.Plugin{}, &models.PluginVersion{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	plugin := models.Plugin{
		Name:            "lifecycle-sync-plugin",
		Type:            "custom",
		Runtime:         PluginRuntimeJSWorker,
		Address:         "index.js",
		Enabled:         true,
		LifecycleStatus: models.PluginLifecyclePaused,
	}
	if err := db.Create(&plugin).Error; err != nil {
		t.Fatalf("create plugin failed: %v", err)
	}

	activeVersion := models.PluginVersion{
		PluginID:        plugin.ID,
		Version:         "1.0.0",
		PackageName:     "lifecycle-sync-plugin",
		LifecycleStatus: models.PluginLifecyclePaused,
		IsActive:        true,
	}
	if err := db.Create(&activeVersion).Error; err != nil {
		t.Fatalf("create active version failed: %v", err)
	}

	inactiveVersion := models.PluginVersion{
		PluginID:        plugin.ID,
		Version:         "0.9.0",
		PackageName:     "lifecycle-sync-plugin",
		LifecycleStatus: models.PluginLifecycleUploaded,
		IsActive:        false,
	}
	if err := db.Create(&inactiveVersion).Error; err != nil {
		t.Fatalf("create inactive version failed: %v", err)
	}

	svc := NewPluginManagerService(db, &config.Config{
		Plugin: config.PluginPlatformConfig{
			Enabled:         true,
			AllowedRuntimes: []string{PluginRuntimeGRPC, PluginRuntimeJSWorker},
			DefaultRuntime:  PluginRuntimeJSWorker,
		},
	})

	if err := svc.updatePluginLifecycle(plugin.ID, models.PluginLifecycleRunning, map[string]interface{}{
		"last_error": "",
	}); err != nil {
		t.Fatalf("update lifecycle failed: %v", err)
	}

	var refreshedPlugin models.Plugin
	if err := db.First(&refreshedPlugin, plugin.ID).Error; err != nil {
		t.Fatalf("reload plugin failed: %v", err)
	}
	if refreshedPlugin.LifecycleStatus != models.PluginLifecycleRunning {
		t.Fatalf("expected plugin lifecycle running, got %s", refreshedPlugin.LifecycleStatus)
	}

	var refreshedActive models.PluginVersion
	if err := db.First(&refreshedActive, activeVersion.ID).Error; err != nil {
		t.Fatalf("reload active version failed: %v", err)
	}
	if refreshedActive.LifecycleStatus != models.PluginLifecycleRunning {
		t.Fatalf("expected active version lifecycle running, got %s", refreshedActive.LifecycleStatus)
	}

	var refreshedInactive models.PluginVersion
	if err := db.First(&refreshedInactive, inactiveVersion.ID).Error; err != nil {
		t.Fatalf("reload inactive version failed: %v", err)
	}
	if refreshedInactive.LifecycleStatus != models.PluginLifecycleUploaded {
		t.Fatalf("expected inactive version lifecycle uploaded, got %s", refreshedInactive.LifecycleStatus)
	}
}

func TestUpdatePluginLifecycleKeepsPluginStateWhenVersionTableUnavailable(t *testing.T) {
	dsn := "file:plugin-manager-lifecycle-missing-version-" + time.Now().UTC().Format("20060102150405.000000000") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&models.Plugin{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	plugin := models.Plugin{
		Name:            "lifecycle-missing-version-plugin",
		Type:            "custom",
		Runtime:         PluginRuntimeJSWorker,
		Address:         "index.js",
		Enabled:         true,
		LifecycleStatus: models.PluginLifecycleInstalled,
	}
	if err := db.Create(&plugin).Error; err != nil {
		t.Fatalf("create plugin failed: %v", err)
	}

	svc := NewPluginManagerService(db, &config.Config{
		Plugin: config.PluginPlatformConfig{
			Enabled:         true,
			AllowedRuntimes: []string{PluginRuntimeGRPC, PluginRuntimeJSWorker},
			DefaultRuntime:  PluginRuntimeJSWorker,
		},
	})

	if err := svc.updatePluginLifecycle(plugin.ID, models.PluginLifecycleRunning, map[string]interface{}{
		"last_error": "",
	}); err != nil {
		t.Fatalf("update lifecycle failed: %v", err)
	}

	var refreshed models.Plugin
	if err := db.First(&refreshed, plugin.ID).Error; err != nil {
		t.Fatalf("reload plugin failed: %v", err)
	}
	if refreshed.LifecycleStatus != models.PluginLifecycleRunning {
		t.Fatalf("expected plugin lifecycle running, got %s", refreshed.LifecycleStatus)
	}
}

func TestPluginManagerServiceStartStopIsIdempotent(t *testing.T) {
	svc := NewPluginManagerService(nil, &config.Config{
		Plugin: config.PluginPlatformConfig{
			Enabled: false,
		},
	})

	svc.Start()
	firstStopChan := svc.stopChan
	firstAuditQueue := svc.getPluginAuditLogQueue()
	if firstStopChan == nil {
		t.Fatalf("expected stop channel to be initialized on first start")
	}
	if firstAuditQueue == nil {
		t.Fatalf("expected audit queue to be initialized on first start")
	}

	svc.Start()
	if svc.stopChan != firstStopChan {
		t.Fatalf("expected repeated start to reuse existing lifecycle state")
	}
	if svc.getPluginAuditLogQueue() != firstAuditQueue {
		t.Fatalf("expected repeated start to avoid spawning a second audit queue")
	}

	svc.Stop()
	if svc.stopChan != nil {
		t.Fatalf("expected stop channel to be cleared after stop")
	}
	if svc.getPluginAuditLogQueue() != nil {
		t.Fatalf("expected audit queue to be cleared after stop")
	}

	svc.Stop()

	svc.Start()
	if svc.stopChan == nil {
		t.Fatalf("expected stop channel to be reinitialized after restart")
	}
	if svc.stopChan == firstStopChan {
		t.Fatalf("expected restart to create a fresh stop channel")
	}
	if svc.getPluginAuditLogQueue() == nil {
		t.Fatalf("expected audit queue to be reinitialized after restart")
	}

	svc.Stop()
}
