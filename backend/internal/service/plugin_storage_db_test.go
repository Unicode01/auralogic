package service

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"auralogic/internal/models"
	"gorm.io/gorm"
)

func TestPluginStorageSnapshotCacheSupportsInvalidate(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	plugin := createPluginStorageTestPlugin(t, db, "storage-cache-plugin")
	row := models.PluginStorageEntry{
		PluginID: plugin.ID,
		Key:      "alpha",
		Value:    "beta",
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("create plugin storage row failed: %v", err)
	}

	svc := NewPluginManagerService(db, nil)

	firstSnapshot, err := svc.loadPluginStorageSnapshot(plugin.ID)
	if err != nil {
		t.Fatalf("loadPluginStorageSnapshot first failed: %v", err)
	}
	if got := firstSnapshot["alpha"]; got != "beta" {
		t.Fatalf("expected cached snapshot value beta, got %q", got)
	}

	if err := db.Model(&models.PluginStorageEntry{}).
		Where("plugin_id = ? AND key = ?", plugin.ID, "alpha").
		Update("value", "gamma").Error; err != nil {
		t.Fatalf("update plugin storage row failed: %v", err)
	}

	cachedSnapshot, err := svc.loadPluginStorageSnapshot(plugin.ID)
	if err != nil {
		t.Fatalf("loadPluginStorageSnapshot cached failed: %v", err)
	}
	if got := cachedSnapshot["alpha"]; got != "beta" {
		t.Fatalf("expected in-memory cache to keep beta before invalidation, got %q", got)
	}

	svc.invalidatePluginStorageSnapshot(plugin.ID)

	reloadedSnapshot, err := svc.loadPluginStorageSnapshot(plugin.ID)
	if err != nil {
		t.Fatalf("loadPluginStorageSnapshot after invalidate failed: %v", err)
	}
	if got := reloadedSnapshot["alpha"]; got != "gamma" {
		t.Fatalf("expected invalidated snapshot to reload gamma, got %q", got)
	}
}

func TestReplacePluginStorageSnapshotPersistsDelta(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	plugin := createPluginStorageTestPlugin(t, db, "storage-delta-plugin")

	seedTime := time.Now().UTC().Add(-time.Hour)
	rows := []models.PluginStorageEntry{
		{
			PluginID:  plugin.ID,
			Key:       "change",
			Value:     "old",
			CreatedAt: seedTime,
			UpdatedAt: seedTime,
		},
		{
			PluginID:  plugin.ID,
			Key:       "keep",
			Value:     "same",
			CreatedAt: seedTime,
			UpdatedAt: seedTime,
		},
		{
			PluginID:  plugin.ID,
			Key:       "remove",
			Value:     "gone",
			CreatedAt: seedTime,
			UpdatedAt: seedTime,
		},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed plugin storage rows failed: %v", err)
	}

	svc := NewPluginManagerService(db, nil)
	if _, err := svc.loadPluginStorageSnapshot(plugin.ID); err != nil {
		t.Fatalf("warm storage snapshot cache failed: %v", err)
	}

	if err := svc.replacePluginStorageSnapshot(plugin.ID, map[string]string{
		"add":    "fresh",
		"change": "new",
		"keep":   "same",
	}); err != nil {
		t.Fatalf("replacePluginStorageSnapshot failed: %v", err)
	}

	var keepRow models.PluginStorageEntry
	if err := db.Where("plugin_id = ? AND key = ?", plugin.ID, "keep").First(&keepRow).Error; err != nil {
		t.Fatalf("query keep row failed: %v", err)
	}
	if keepRow.ID != rows[1].ID {
		t.Fatalf("expected keep row id %d to remain unchanged, got %d", rows[1].ID, keepRow.ID)
	}

	var changedRow models.PluginStorageEntry
	if err := db.Where("plugin_id = ? AND key = ?", plugin.ID, "change").First(&changedRow).Error; err != nil {
		t.Fatalf("query change row failed: %v", err)
	}
	if changedRow.ID != rows[0].ID {
		t.Fatalf("expected changed row id %d to be updated in place, got %d", rows[0].ID, changedRow.ID)
	}
	if changedRow.Value != "new" {
		t.Fatalf("expected changed row value new, got %q", changedRow.Value)
	}
	if !changedRow.UpdatedAt.After(rows[0].UpdatedAt) {
		t.Fatalf("expected changed row UpdatedAt to advance, got old=%s new=%s", rows[0].UpdatedAt, changedRow.UpdatedAt)
	}

	var addedRow models.PluginStorageEntry
	if err := db.Where("plugin_id = ? AND key = ?", plugin.ID, "add").First(&addedRow).Error; err != nil {
		t.Fatalf("query add row failed: %v", err)
	}
	if addedRow.Value != "fresh" {
		t.Fatalf("expected add row value fresh, got %q", addedRow.Value)
	}

	var removedCount int64
	if err := db.Model(&models.PluginStorageEntry{}).
		Where("plugin_id = ? AND key = ?", plugin.ID, "remove").
		Count(&removedCount).Error; err != nil {
		t.Fatalf("count remove row failed: %v", err)
	}
	if removedCount != 0 {
		t.Fatalf("expected remove row to be deleted, count=%d", removedCount)
	}

	cachedSnapshot, err := svc.loadPluginStorageSnapshot(plugin.ID)
	if err != nil {
		t.Fatalf("load cached storage snapshot after replace failed: %v", err)
	}
	if got := cachedSnapshot["add"]; got != "fresh" {
		t.Fatalf("expected cached add value fresh, got %q", got)
	}
	if got := cachedSnapshot["change"]; got != "new" {
		t.Fatalf("expected cached change value new, got %q", got)
	}
	if _, exists := cachedSnapshot["remove"]; exists {
		t.Fatalf("expected cached snapshot to drop removed key")
	}
}

func TestAcquirePluginStorageExecutionLockAllowsConcurrentReaders(t *testing.T) {
	pluginID := uint(9001)
	defer releasePluginStorageLock(pluginID)

	start := make(chan struct{})
	var current int32
	var maxConcurrent int32
	var wg sync.WaitGroup

	runReader := func() {
		defer wg.Done()
		<-start
		release := acquirePluginStorageExecutionLock(pluginID, pluginStorageAccessRead)
		defer release()

		active := atomic.AddInt32(&current, 1)
		for {
			previous := atomic.LoadInt32(&maxConcurrent)
			if active <= previous || atomic.CompareAndSwapInt32(&maxConcurrent, previous, active) {
				break
			}
		}
		time.Sleep(80 * time.Millisecond)
		atomic.AddInt32(&current, -1)
	}

	wg.Add(2)
	go runReader()
	go runReader()
	close(start)
	wg.Wait()

	if atomic.LoadInt32(&maxConcurrent) < 2 {
		t.Fatalf("expected read-mode storage lock to allow concurrent readers, max=%d", atomic.LoadInt32(&maxConcurrent))
	}
}

func TestValidatePluginStorageAccessModeRejectsUndeclaredWrite(t *testing.T) {
	err := validatePluginStorageAccessMode(pluginStorageAccessRead, pluginStorageAccessWrite, true)
	if err == nil {
		t.Fatalf("expected undeclared write to be rejected")
	}
}

func createPluginStorageTestPlugin(t *testing.T, db *gorm.DB, name string) models.Plugin {
	t.Helper()
	plugin := models.Plugin{
		Name:            name,
		DisplayName:     name,
		Type:            "custom",
		Runtime:         PluginRuntimeJSWorker,
		Address:         "/tmp/test-plugin.js",
		Version:         "1.0.0",
		Enabled:         true,
		Status:          "healthy",
		LifecycleStatus: models.PluginLifecycleInstalled,
	}
	if err := db.Create(&plugin).Error; err != nil {
		t.Fatalf("create plugin failed: %v", err)
	}
	return plugin
}
