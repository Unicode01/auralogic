package service

import (
	"testing"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/models"
)

func TestRecordExecutionPersistsMergedMetadata(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)

	plugin := models.Plugin{
		Name:            "record-metadata-plugin",
		DisplayName:     "Record Metadata Plugin",
		Type:            "custom",
		Runtime:         PluginRuntimeJSWorker,
		Address:         "test.js",
		Version:         "1.0.0",
		Enabled:         true,
		Status:          "healthy",
		LifecycleStatus: models.PluginLifecycleInstalled,
	}
	if err := db.Create(&plugin).Error; err != nil {
		t.Fatalf("create plugin failed: %v", err)
	}

	svc := &PluginManagerService{db: db}
	userID := uint(7)
	execCtx := &ExecutionContext{
		UserID: &userID,
		Metadata: map[string]string{
			PluginExecutionMetadataID:     "pex_record_1",
			PluginExecutionMetadataStatus: PluginExecutionStatusRunning,
			"request_path":                "/admin/plugins",
			"api_token":                   "secret-token",
		},
	}
	result := &ExecutionResult{
		Success: true,
		Metadata: map[string]string{
			PluginExecutionMetadataStatus: "completed",
			"storage_access_mode":         "write",
		},
	}

	svc.recordExecution(plugin.ID, "template.page.save", map[string]string{"slug": "home"}, execCtx, result, nil, 33)

	var execution models.PluginExecution
	if err := db.Where("plugin_id = ?", plugin.ID).First(&execution).Error; err != nil {
		t.Fatalf("query execution failed: %v", err)
	}
	if execution.Metadata == nil {
		t.Fatalf("expected persisted metadata, got nil")
	}
	if got := execution.Metadata[PluginExecutionMetadataID]; got != "pex_record_1" {
		t.Fatalf("expected task id to persist, got %+v", execution.Metadata)
	}
	if got := execution.Metadata[PluginExecutionMetadataStatus]; got != PluginExecutionStatusCompleted {
		t.Fatalf("expected completed status to override running status, got %+v", execution.Metadata)
	}
	if got := execution.Metadata["storage_access_mode"]; got != "write" {
		t.Fatalf("expected storage_access_mode=write, got %+v", execution.Metadata)
	}
	if got := execution.Metadata["request_path"]; got != "/admin/plugins" {
		t.Fatalf("expected request_path to persist, got %+v", execution.Metadata)
	}
	if got := execution.Metadata["api_token"]; got != "[REDACTED]" {
		t.Fatalf("expected sensitive metadata to be redacted, got %+v", execution.Metadata)
	}
	if got := execution.Hook; got != "" {
		t.Fatalf("expected non-hook action to persist empty hook, got %+v", execution)
	}
	if got := execution.ErrorSignature; got != "" {
		t.Fatalf("expected successful execution to persist empty error signature, got %+v", execution)
	}
}

func TestRecordExecutionPersistsHookAndErrorSignature(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)

	plugin := models.Plugin{
		Name:            "record-failure-plugin",
		DisplayName:     "Record Failure Plugin",
		Type:            "custom",
		Runtime:         PluginRuntimeJSWorker,
		Address:         "test.js",
		Version:         "1.0.0",
		Enabled:         true,
		Status:          "healthy",
		LifecycleStatus: models.PluginLifecycleInstalled,
	}
	if err := db.Create(&plugin).Error; err != nil {
		t.Fatalf("create plugin failed: %v", err)
	}

	svc := &PluginManagerService{db: db}
	execCtx := &ExecutionContext{
		Metadata: map[string]string{
			PluginExecutionMetadataHook: "order.create.after",
		},
	}
	result := &ExecutionResult{
		Success: false,
		Error:   "hook failed 42",
		Metadata: map[string]string{
			PluginExecutionMetadataStatus: PluginExecutionStatusFailed,
		},
	}

	svc.recordExecution(
		plugin.ID,
		"hook.execute",
		map[string]string{"hook": "order.create.after"},
		execCtx,
		result,
		nil,
		20,
	)

	var execution models.PluginExecution
	if err := db.Where("plugin_id = ?", plugin.ID).First(&execution).Error; err != nil {
		t.Fatalf("query execution failed: %v", err)
	}
	if execution.Hook != "order.create.after" {
		t.Fatalf("expected persisted hook order.create.after, got %+v", execution)
	}
	if execution.ErrorSignature != "hook failed #" {
		t.Fatalf("expected normalized error signature, got %+v", execution)
	}
}

func TestPluginExecutionTaskCompleteMergesResultMetadata(t *testing.T) {
	startedAt := time.Date(2026, time.March, 12, 9, 0, 0, 0, time.UTC)
	task := &pluginExecutionTask{
		id:         "pex_test_1",
		pluginID:   7,
		pluginName: "metadata-plugin",
		runtime:    PluginRuntimeJSWorker,
		action:     "template.page.save",
		status:     PluginExecutionStatusRunning,
		startedAt:  startedAt,
		updatedAt:  startedAt,
		metadata: map[string]string{
			"seed": "value",
		},
	}

	snapshot := task.complete(&ExecutionResult{
		Success: true,
		Metadata: map[string]string{
			"storage_access_mode": "write",
		},
	}, nil)

	if got := snapshot.Metadata["storage_access_mode"]; got != "write" {
		t.Fatalf("expected storage_access_mode=write, got %+v", snapshot.Metadata)
	}
	if got := snapshot.Metadata["seed"]; got != "value" {
		t.Fatalf("expected existing metadata to survive merge, got %+v", snapshot.Metadata)
	}
	if got := snapshot.Metadata[PluginExecutionMetadataStatus]; got != PluginExecutionStatusCompleted {
		t.Fatalf("expected completed status metadata, got %+v", snapshot.Metadata)
	}
}

func TestRecordExecutionQueueFlushesOnStop(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)

	plugin := models.Plugin{
		Name:            "record-queue-plugin",
		DisplayName:     "Record Queue Plugin",
		Type:            "custom",
		Runtime:         PluginRuntimeJSWorker,
		Address:         "test.js",
		Version:         "1.0.0",
		Enabled:         true,
		Status:          "healthy",
		LifecycleStatus: models.PluginLifecycleInstalled,
	}
	if err := db.Create(&plugin).Error; err != nil {
		t.Fatalf("create plugin failed: %v", err)
	}

	svc := NewPluginManagerService(db, &config.Config{
		Plugin: config.PluginPlatformConfig{
			Enabled: false,
		},
	})
	svc.Start()

	svc.recordExecution(plugin.ID, "hook.execute", map[string]string{"hook": "order.create.after"}, &ExecutionContext{
		Metadata: map[string]string{
			PluginExecutionMetadataHook: "order.create.after",
		},
	}, &ExecutionResult{
		Success: true,
	}, nil, 12)

	svc.Stop()

	var count int64
	if err := db.Model(&models.PluginExecution{}).Where("plugin_id = ?", plugin.ID).Count(&count).Error; err != nil {
		t.Fatalf("count executions failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected flushed execution record, got %d", count)
	}
}
