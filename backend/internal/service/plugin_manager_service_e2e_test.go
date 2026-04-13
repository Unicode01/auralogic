package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/jsworker"
	"auralogic/internal/models"
	"auralogic/internal/pb"
	"google.golang.org/grpc"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPluginManagerServiceGRPCEndToEnd(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	serverAddr, cleanupGRPC := startTestPluginGRPCServer(t)
	defer cleanupGRPC()

	plugin := models.Plugin{
		Name:            "grpc-e2e-plugin",
		DisplayName:     "gRPC E2E Plugin",
		Type:            "custom",
		Runtime:         PluginRuntimeGRPC,
		Address:         serverAddr,
		Version:         "1.0.0",
		Capabilities:    mustMarshalCapabilities(t, PluginPermissionExecuteAPI, PluginPermissionHookExecute, PluginPermissionHookPayloadPatch),
		Enabled:         true,
		Status:          "unknown",
		LifecycleStatus: models.PluginLifecycleInstalled,
	}
	if err := db.Create(&plugin).Error; err != nil {
		t.Fatalf("create plugin failed: %v", err)
	}

	svc := NewPluginManagerService(db, &config.Config{
		Plugin: config.PluginPlatformConfig{
			Enabled:         true,
			AllowedRuntimes: []string{PluginRuntimeGRPC, PluginRuntimeJSWorker},
			DefaultRuntime:  PluginRuntimeGRPC,
			GRPC: config.PluginGRPCTransportConfig{
				Mode: "insecure_local",
			},
			Sandbox: config.PluginSandboxConfig{
				ExecTimeoutMs:  30000,
				MaxMemoryMB:    128,
				MaxConcurrency: 4,
			},
		},
	})
	svc.Start()
	defer svc.Stop()

	health, err := svc.TestPlugin(plugin.ID)
	if err != nil {
		t.Fatalf("grpc TestPlugin failed: %v", err)
	}
	if health == nil || !health.Healthy {
		t.Fatalf("expected grpc plugin healthy, got %+v", health)
	}
	if health.Version != "test-grpc/1.0.0" {
		t.Fatalf("expected grpc health version test-grpc/1.0.0, got %q", health.Version)
	}

	userID := uint(7)
	result, err := svc.ExecutePlugin(plugin.ID, "ping", map[string]string{"alpha": "1"}, &ExecutionContext{
		UserID:    &userID,
		SessionID: "grpc-session",
		Metadata:  map[string]string{"source": "grpc-e2e"},
	})
	if err != nil {
		t.Fatalf("grpc ExecutePlugin failed: %v", err)
	}
	if result == nil || !result.Success {
		t.Fatalf("expected grpc execute success, got %+v", result)
	}
	if got := stringifyAny(result.Data["action"]); got != "ping" {
		t.Fatalf("expected action=ping, got %q", got)
	}
	params, ok := result.Data["params"].(map[string]interface{})
	if !ok || stringifyAny(params["alpha"]) != "1" {
		t.Fatalf("expected params.alpha=1, got %#v", result.Data["params"])
	}
	if got := stringifyAny(result.Data["session_id"]); got != "grpc-session" {
		t.Fatalf("expected session_id=grpc-session, got %q", got)
	}

	hookResult, err := svc.ExecuteHook(HookExecutionRequest{
		Hook:    "test.custom.event",
		Payload: map[string]interface{}{"seed": "origin"},
	}, nil)
	if err != nil {
		t.Fatalf("grpc ExecuteHook failed: %v", err)
	}
	if hookResult == nil {
		t.Fatalf("expected hook result")
	}
	if len(hookResult.PluginResults) != 1 || !hookResult.PluginResults[0].Success {
		t.Fatalf("expected single successful grpc hook result, got %+v", hookResult.PluginResults)
	}
	if got := stringifyAny(hookResult.Payload["grpc_hook"]); got != "test.custom.event" {
		t.Fatalf("expected hook payload patched by grpc plugin, got %+v", hookResult.Payload)
	}

	waitForPluginExecutionRecords(t, db, plugin.ID, 2)
}

func TestPluginManagerServiceGRPCExecuteStreamEndToEnd(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	serverAddr, cleanupGRPC := startTestPluginGRPCServer(t)
	defer cleanupGRPC()

	plugin := models.Plugin{
		Name:            "grpc-stream-plugin",
		DisplayName:     "gRPC Stream Plugin",
		Type:            "custom",
		Runtime:         PluginRuntimeGRPC,
		Address:         serverAddr,
		Version:         "1.0.0",
		Capabilities:    mustMarshalCapabilities(t, PluginPermissionExecuteAPI),
		Enabled:         true,
		Status:          "unknown",
		LifecycleStatus: models.PluginLifecycleInstalled,
	}
	if err := db.Create(&plugin).Error; err != nil {
		t.Fatalf("create plugin failed: %v", err)
	}

	svc := NewPluginManagerService(db, &config.Config{
		Plugin: config.PluginPlatformConfig{
			Enabled:         true,
			AllowedRuntimes: []string{PluginRuntimeGRPC},
			DefaultRuntime:  PluginRuntimeGRPC,
			GRPC: config.PluginGRPCTransportConfig{
				Mode: "insecure_local",
			},
			Sandbox: config.PluginSandboxConfig{
				ExecTimeoutMs:  30000,
				MaxMemoryMB:    128,
				MaxConcurrency: 4,
			},
		},
	})
	svc.Start()
	defer svc.Stop()

	userID := uint(9)
	chunks := make([]*ExecutionStreamChunk, 0)
	execCtx := &ExecutionContext{
		UserID:    &userID,
		SessionID: "grpc-stream-session",
	}
	taskID := EnsurePluginExecutionMetadata(execCtx, true)
	result, err := svc.ExecutePluginStream(plugin.ID, "stream.echo", map[string]string{"alpha": "1"}, execCtx, func(chunk *ExecutionStreamChunk) error {
		chunks = append(chunks, cloneExecutionStreamChunk(chunk))
		return nil
	})
	if err != nil {
		t.Fatalf("grpc ExecutePluginStream failed: %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("expected two stream chunks, got %+v", chunks)
	}
	if got := strings.TrimSpace(chunks[0].TaskID); got != taskID {
		t.Fatalf("expected first grpc chunk task_id=%s, got %q", taskID, got)
	}
	if chunks[0].IsFinal {
		t.Fatalf("expected first chunk to be non-final, got %+v", chunks[0])
	}
	if !chunks[1].IsFinal || !chunks[1].Success {
		t.Fatalf("expected second chunk to be final success, got %+v", chunks[1])
	}
	if result == nil || !result.Success {
		t.Fatalf("expected final stream result success, got %+v", result)
	}
	if got := strings.TrimSpace(result.TaskID); got != taskID {
		t.Fatalf("expected grpc result task_id=%s, got %q", taskID, got)
	}
	if got := stringifyAny(result.Data["action"]); got != "stream.echo" {
		t.Fatalf("expected final action=stream.echo, got %q", got)
	}
	if got := stringifyAny(result.Data["session_id"]); got != "grpc-stream-session" {
		t.Fatalf("expected session_id=grpc-stream-session, got %q", got)
	}
}

func TestPluginManagerServiceGRPCExecuteStreamCancellation(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	serverAddr, cleanupGRPC := startTestPluginGRPCServer(t)
	defer cleanupGRPC()

	plugin := models.Plugin{
		Name:            "grpc-stream-cancel-plugin",
		DisplayName:     "gRPC Stream Cancel Plugin",
		Type:            "custom",
		Runtime:         PluginRuntimeGRPC,
		Address:         serverAddr,
		Version:         "1.0.0",
		Capabilities:    mustMarshalCapabilities(t, PluginPermissionExecuteAPI),
		Enabled:         true,
		Status:          "unknown",
		LifecycleStatus: models.PluginLifecycleInstalled,
	}
	if err := db.Create(&plugin).Error; err != nil {
		t.Fatalf("create plugin failed: %v", err)
	}

	svc := NewPluginManagerService(db, &config.Config{
		Plugin: config.PluginPlatformConfig{
			Enabled:         true,
			AllowedRuntimes: []string{PluginRuntimeGRPC},
			DefaultRuntime:  PluginRuntimeGRPC,
			GRPC: config.PluginGRPCTransportConfig{
				Mode: "insecure_local",
			},
			Sandbox: config.PluginSandboxConfig{
				ExecTimeoutMs:  30000,
				MaxMemoryMB:    128,
				MaxConcurrency: 4,
			},
		},
	})
	svc.Start()
	defer svc.Stop()

	reqCtx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(120*time.Millisecond, cancel)

	startedAt := time.Now()
	execCtx := &ExecutionContext{
		SessionID:      "grpc-stream-cancel-session",
		RequestContext: reqCtx,
	}
	taskID := EnsurePluginExecutionMetadata(execCtx, true)
	_, err := svc.ExecutePluginStream(plugin.ID, "stream.wait_cancel", map[string]string{"alpha": "1"}, execCtx, nil)
	if !isExecutionCanceledError(err) {
		t.Fatalf("expected context canceled, got %v", err)
	}
	if elapsed := time.Since(startedAt); elapsed > 2*time.Second {
		t.Fatalf("expected grpc stream cancellation to return quickly, got %s", elapsed)
	}
	task, exists := svc.GetPluginExecutionTask(plugin.ID, taskID)
	if !exists || task == nil {
		t.Fatalf("expected grpc canceled task snapshot for %s", taskID)
	}
	if task.Status != PluginExecutionStatusCanceled {
		t.Fatalf("expected grpc task status=canceled, got %+v", task)
	}
}

func TestPluginManagerServiceJSWorkerEndToEnd(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	artifactRoot := t.TempDir()
	scriptPath := writeJSE2ETestPlugin(t, artifactRoot)
	workerAddr := startTestJSWorker(t, artifactRoot, "-allow-fs=true")

	plugin := models.Plugin{
		Name:            "js-e2e-plugin",
		DisplayName:     "JS E2E Plugin",
		Type:            "custom",
		Runtime:         PluginRuntimeJSWorker,
		Address:         filepath.ToSlash(scriptPath),
		PackagePath:     filepath.ToSlash(filepath.Join(artifactRoot, "plugin-js-e2e.zip")),
		Version:         "1.0.0",
		Capabilities:    mustMarshalCapabilities(t, PluginPermissionExecuteAPI, PluginPermissionHookExecute, PluginPermissionHookPayloadPatch, PluginPermissionRuntimeFileSystem),
		Enabled:         true,
		Status:          "unknown",
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
			ArtifactDir:     artifactRoot,
			Sandbox: config.PluginSandboxConfig{
				ExecTimeoutMs:      30000,
				MaxMemoryMB:        128,
				MaxConcurrency:     4,
				JSWorkerSocketPath: "tcp://" + workerAddr,
				JSWorkerAutoStart:  false,
				JSAllowFileSystem:  true,
			},
		},
	})
	svc.Start()
	defer svc.Stop()

	health, err := svc.TestPlugin(plugin.ID)
	if err != nil {
		t.Fatalf("js TestPlugin failed: %v", err)
	}
	if health == nil || !health.Healthy {
		t.Fatalf("expected js plugin healthy, got %+v", health)
	}
	if health.Version != "test-js/1.0.0" {
		t.Fatalf("expected js health version test-js/1.0.0, got %q", health.Version)
	}

	setResult, err := svc.ExecutePlugin(plugin.ID, "kv.set", map[string]string{"key": "alpha", "value": "beta"}, nil)
	if err != nil {
		t.Fatalf("js kv.set failed: %v", err)
	}
	if setResult == nil || !setResult.Success {
		t.Fatalf("expected js kv.set success, got %+v", setResult)
	}

	getResult, err := svc.ExecutePlugin(plugin.ID, "kv.get", map[string]string{"key": "alpha"}, nil)
	if err != nil {
		t.Fatalf("js kv.get failed: %v", err)
	}
	if getResult == nil || !getResult.Success {
		t.Fatalf("expected js kv.get success, got %+v", getResult)
	}
	if got := stringifyAny(getResult.Data["value"]); got != "beta" {
		t.Fatalf("expected stored value beta, got %q", got)
	}

	fsResult, err := svc.ExecutePlugin(plugin.ID, "fs.probe", map[string]string{}, nil)
	if err != nil {
		t.Fatalf("js fs.probe failed: %v", err)
	}
	if fsResult == nil || !fsResult.Success {
		t.Fatalf("expected js fs.probe success, got %+v", fsResult)
	}
	if got := stringifyAny(fsResult.Data["fs_enabled"]); got != "true" {
		t.Fatalf("expected fs_enabled=true, got %q", got)
	}
	if got := stringifyAny(fsResult.Data["content"]); got != "ok" {
		t.Fatalf("expected fs content ok, got %q", got)
	}
	if got := stringifyAny(fsResult.Data["usage_file_count"]); got != "1" {
		t.Fatalf("expected usage_file_count=1, got %q", got)
	}
	if got := stringifyAny(fsResult.Data["recalculated_file_count"]); got != "1" {
		t.Fatalf("expected recalculated_file_count=1, got %q", got)
	}
	if got := stringifyAny(fsResult.Data["usage_max_files"]); got != "2048" {
		t.Fatalf("expected usage_max_files=2048, got %q", got)
	}

	var storageEntry models.PluginStorageEntry
	if err := db.Where("plugin_id = ? AND key = ?", plugin.ID, "alpha").First(&storageEntry).Error; err != nil {
		t.Fatalf("query plugin storage failed: %v", err)
	}
	if storageEntry.Value != "beta" {
		t.Fatalf("expected persisted storage beta, got %q", storageEntry.Value)
	}

	hookResult, err := svc.ExecuteHook(HookExecutionRequest{
		Hook:    "test.custom.event",
		Payload: map[string]interface{}{"seed": "origin"},
	}, nil)
	if err != nil {
		t.Fatalf("js ExecuteHook failed: %v", err)
	}
	if hookResult == nil {
		t.Fatalf("expected hook result")
	}
	if len(hookResult.PluginResults) != 1 || !hookResult.PluginResults[0].Success {
		t.Fatalf("expected single successful js hook result, got %+v", hookResult.PluginResults)
	}
	if got := stringifyAny(hookResult.Payload["js_hook"]); got != "test.custom.event" {
		t.Fatalf("expected hook payload patched by js plugin, got %+v", hookResult.Payload)
	}

	waitForPluginExecutionRecords(t, db, plugin.ID, 3)
}

func TestPluginManagerServiceJSWorkerRejectsMisdeclaredStorageWrite(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	artifactRoot := t.TempDir()
	scriptPath := writeJSE2ETestPlugin(t, artifactRoot)
	workerAddr := startTestJSWorker(t, artifactRoot)

	plugin := models.Plugin{
		Name:        "js-storage-profile-plugin",
		DisplayName: "JS Storage Profile Plugin",
		Type:        "custom",
		Runtime:     PluginRuntimeJSWorker,
		Address:     filepath.ToSlash(scriptPath),
		PackagePath: filepath.ToSlash(filepath.Join(artifactRoot, "plugin-js-storage-profile.zip")),
		Version:     "1.0.0",
		Capabilities: mustMarshalCapabilityMap(t, map[string]interface{}{
			"requested_permissions": []string{PluginPermissionExecuteAPI},
			"granted_permissions":   []string{PluginPermissionExecuteAPI},
			"allow_execute_api":     true,
			"execute_action_storage": map[string]string{
				"kv.get": pluginStorageAccessRead,
				"kv.set": pluginStorageAccessRead,
			},
		}),
		Enabled:         true,
		Status:          "unknown",
		LifecycleStatus: models.PluginLifecycleInstalled,
	}
	if err := db.Create(&plugin).Error; err != nil {
		t.Fatalf("create plugin failed: %v", err)
	}

	svc := NewPluginManagerService(db, &config.Config{
		Plugin: config.PluginPlatformConfig{
			Enabled:         true,
			AllowedRuntimes: []string{PluginRuntimeJSWorker},
			DefaultRuntime:  PluginRuntimeJSWorker,
			ArtifactDir:     artifactRoot,
			Sandbox: config.PluginSandboxConfig{
				ExecTimeoutMs:      30000,
				MaxMemoryMB:        128,
				MaxConcurrency:     4,
				JSWorkerSocketPath: "tcp://" + workerAddr,
				JSWorkerAutoStart:  false,
			},
		},
	})
	svc.Start()
	defer svc.Stop()

	if _, err := svc.ExecutePlugin(plugin.ID, "kv.get", map[string]string{"key": "alpha"}, nil); err != nil {
		t.Fatalf("expected declared read action to execute successfully, got %v", err)
	}

	_, err := svc.ExecutePlugin(plugin.ID, "kv.set", map[string]string{"key": "alpha", "value": "beta"}, nil)
	if err == nil {
		t.Fatalf("expected misdeclared storage write to be rejected")
	}
	if !strings.Contains(err.Error(), "declared storage access read") {
		t.Fatalf("expected storage profile validation error, got %v", err)
	}
}

func TestPluginManagerServiceJSWorkerExecuteStreamEndToEnd(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	artifactRoot := t.TempDir()
	scriptPath := writeJSStreamE2ETestPlugin(t, artifactRoot)
	workerAddr := startTestJSWorker(t, artifactRoot)

	plugin := models.Plugin{
		Name:            "js-real-stream-plugin",
		DisplayName:     "JS Real Stream Plugin",
		Type:            "custom",
		Runtime:         PluginRuntimeJSWorker,
		Address:         filepath.ToSlash(scriptPath),
		PackagePath:     filepath.ToSlash(filepath.Join(artifactRoot, "plugin-js-real-stream.zip")),
		Version:         "1.0.0",
		Capabilities:    mustMarshalCapabilities(t, PluginPermissionExecuteAPI),
		Enabled:         true,
		Status:          "unknown",
		LifecycleStatus: models.PluginLifecycleInstalled,
	}
	if err := db.Create(&plugin).Error; err != nil {
		t.Fatalf("create plugin failed: %v", err)
	}

	svc := NewPluginManagerService(db, &config.Config{
		Plugin: config.PluginPlatformConfig{
			Enabled:         true,
			AllowedRuntimes: []string{PluginRuntimeJSWorker},
			DefaultRuntime:  PluginRuntimeJSWorker,
			ArtifactDir:     artifactRoot,
			Sandbox: config.PluginSandboxConfig{
				ExecTimeoutMs:      30000,
				MaxMemoryMB:        128,
				MaxConcurrency:     4,
				JSWorkerSocketPath: "tcp://" + workerAddr,
				JSWorkerAutoStart:  false,
			},
		},
	})
	svc.Start()
	defer svc.Stop()

	chunks := make([]*ExecutionStreamChunk, 0)
	execCtx := &ExecutionContext{
		SessionID: "js-stream-session",
	}
	taskID := EnsurePluginExecutionMetadata(execCtx, true)
	result, err := svc.ExecutePluginStream(plugin.ID, "stream.echo", map[string]string{"alpha": "1"}, execCtx, func(chunk *ExecutionStreamChunk) error {
		chunks = append(chunks, cloneExecutionStreamChunk(chunk))
		return nil
	})
	if err != nil {
		t.Fatalf("js ExecutePluginStream failed: %v", err)
	}
	if len(chunks) != 3 {
		t.Fatalf("expected three js stream chunks, got %+v", chunks)
	}
	if got := strings.TrimSpace(chunks[0].TaskID); got != taskID {
		t.Fatalf("expected first js chunk task_id=%s, got %q", taskID, got)
	}
	if chunks[0].IsFinal || chunks[1].IsFinal {
		t.Fatalf("expected first two chunks to be non-final, got %+v", chunks)
	}
	if !chunks[2].IsFinal || !chunks[2].Success {
		t.Fatalf("expected final js chunk to be successful and final, got %+v", chunks[2])
	}
	if got := stringifyAny(chunks[0].Data["status"]); got != "preparing" {
		t.Fatalf("expected first chunk status=preparing, got %q", got)
	}
	if got := stringifyAny(chunks[1].Data["progress"]); got != "70" {
		t.Fatalf("expected second chunk progress=70, got %q", got)
	}
	if result == nil || !result.Success {
		t.Fatalf("expected js stream result success, got %+v", result)
	}
	if got := strings.TrimSpace(result.TaskID); got != taskID {
		t.Fatalf("expected js result task_id=%s, got %q", taskID, got)
	}
	if got := stringifyAny(result.Data["action"]); got != "stream.echo" {
		t.Fatalf("expected final action=stream.echo, got %q", got)
	}
	if got := stringifyAny(result.Data["session_id"]); got != "js-stream-session" {
		t.Fatalf("expected final session_id=js-stream-session, got %q", got)
	}
	if got := strings.TrimSpace(result.Metadata["stream"]); got != "true" {
		t.Fatalf("expected final metadata.stream=true, got %+v", result.Metadata)
	}
}

func TestPluginManagerServiceJSWorkerExecuteStreamCancellation(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	artifactRoot := t.TempDir()
	scriptPath := writeJSStreamE2ETestPlugin(t, artifactRoot)
	workerAddr := startTestJSWorker(t, artifactRoot)

	plugin := models.Plugin{
		Name:            "js-stream-cancel-plugin",
		DisplayName:     "JS Stream Cancel Plugin",
		Type:            "custom",
		Runtime:         PluginRuntimeJSWorker,
		Address:         filepath.ToSlash(scriptPath),
		PackagePath:     filepath.ToSlash(filepath.Join(artifactRoot, "plugin-js-stream-cancel.zip")),
		Version:         "1.0.0",
		Capabilities:    mustMarshalCapabilities(t, PluginPermissionExecuteAPI),
		Enabled:         true,
		Status:          "unknown",
		LifecycleStatus: models.PluginLifecycleInstalled,
	}
	if err := db.Create(&plugin).Error; err != nil {
		t.Fatalf("create plugin failed: %v", err)
	}

	svc := NewPluginManagerService(db, &config.Config{
		Plugin: config.PluginPlatformConfig{
			Enabled:         true,
			AllowedRuntimes: []string{PluginRuntimeJSWorker},
			DefaultRuntime:  PluginRuntimeJSWorker,
			ArtifactDir:     artifactRoot,
			Sandbox: config.PluginSandboxConfig{
				ExecTimeoutMs:      30000,
				MaxMemoryMB:        128,
				MaxConcurrency:     4,
				JSWorkerSocketPath: "tcp://" + workerAddr,
				JSWorkerAutoStart:  false,
			},
		},
	})
	svc.Start()
	defer svc.Stop()

	reqCtx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(120*time.Millisecond, cancel)

	startedAt := time.Now()
	execCtx := &ExecutionContext{
		SessionID:      "js-stream-cancel-session",
		RequestContext: reqCtx,
	}
	taskID := EnsurePluginExecutionMetadata(execCtx, true)
	_, err := svc.ExecutePluginStream(plugin.ID, "stream.wait_cancel", map[string]string{"alpha": "1"}, execCtx, nil)
	if !isExecutionCanceledError(err) {
		t.Fatalf("expected context canceled, got %v", err)
	}
	if elapsed := time.Since(startedAt); elapsed > 2*time.Second {
		t.Fatalf("expected js worker stream cancellation to return quickly, got %s", elapsed)
	}
	task, exists := svc.GetPluginExecutionTask(plugin.ID, taskID)
	if !exists || task == nil {
		t.Fatalf("expected js canceled task snapshot for %s", taskID)
	}
	if task.Status != PluginExecutionStatusCanceled {
		t.Fatalf("expected js task status=canceled, got %+v", task)
	}
}

func TestPluginManagerServiceJSWorkerExecuteStreamFallsBackToFinalChunk(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	artifactRoot := t.TempDir()
	scriptPath := writeJSE2ETestPlugin(t, artifactRoot)
	workerAddr := startTestJSWorker(t, artifactRoot)

	plugin := models.Plugin{
		Name:            "js-stream-plugin",
		DisplayName:     "JS Stream Plugin",
		Type:            "custom",
		Runtime:         PluginRuntimeJSWorker,
		Address:         filepath.ToSlash(scriptPath),
		PackagePath:     filepath.ToSlash(filepath.Join(artifactRoot, "plugin-js-stream.zip")),
		Version:         "1.0.0",
		Capabilities:    mustMarshalCapabilities(t, PluginPermissionExecuteAPI),
		Enabled:         true,
		Status:          "unknown",
		LifecycleStatus: models.PluginLifecycleInstalled,
	}
	if err := db.Create(&plugin).Error; err != nil {
		t.Fatalf("create plugin failed: %v", err)
	}

	svc := NewPluginManagerService(db, &config.Config{
		Plugin: config.PluginPlatformConfig{
			Enabled:         true,
			AllowedRuntimes: []string{PluginRuntimeJSWorker},
			DefaultRuntime:  PluginRuntimeJSWorker,
			ArtifactDir:     artifactRoot,
			Sandbox: config.PluginSandboxConfig{
				ExecTimeoutMs:      30000,
				MaxMemoryMB:        128,
				MaxConcurrency:     4,
				JSWorkerSocketPath: "tcp://" + workerAddr,
				JSWorkerAutoStart:  false,
			},
		},
	})
	svc.Start()
	defer svc.Stop()

	chunks := make([]*ExecutionStreamChunk, 0)
	result, err := svc.ExecutePluginStream(plugin.ID, "kv.set", map[string]string{"key": "alpha", "value": "beta"}, nil, func(chunk *ExecutionStreamChunk) error {
		chunks = append(chunks, cloneExecutionStreamChunk(chunk))
		return nil
	})
	if err != nil {
		t.Fatalf("js ExecutePluginStream failed: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected single fallback chunk, got %+v", chunks)
	}
	if !chunks[0].IsFinal || !chunks[0].Success {
		t.Fatalf("expected fallback chunk to be final success, got %+v", chunks[0])
	}
	if result == nil || !result.Success {
		t.Fatalf("expected js stream result success, got %+v", result)
	}
	if got := stringifyAny(result.Data["saved"]); got != "true" {
		t.Fatalf("expected saved=true, got %q", got)
	}
}

func TestPluginManagerServiceFrontendBootstrapBypassesFrontendSlotWhitelist(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	artifactRoot := t.TempDir()
	scriptPath := writeJSFrontendBootstrapTestPlugin(t, artifactRoot)
	workerAddr := startTestJSWorker(t, artifactRoot)

	plugin := models.Plugin{
		Name:        "js-bootstrap-route-plugin",
		DisplayName: "JS Bootstrap Route Plugin",
		Type:        "custom",
		Runtime:     PluginRuntimeJSWorker,
		Address:     filepath.ToSlash(scriptPath),
		PackagePath: filepath.ToSlash(filepath.Join(artifactRoot, "plugin-js-bootstrap-route.zip")),
		Version:     "1.0.0",
		Capabilities: mustMarshalCapabilityMap(t, map[string]interface{}{
			"hooks": []string{"frontend.bootstrap"},
			"requested_permissions": []string{
				PluginPermissionHookExecute,
				PluginPermissionFrontendExtension,
			},
			"granted_permissions": []string{
				PluginPermissionHookExecute,
				PluginPermissionFrontendExtension,
			},
			"frontend_allowed_areas": []string{"admin"},
			"allowed_frontend_slots": []string{
				"admin.plugin_page.top",
				"admin.plugin_page.bottom",
			},
		}),
		Enabled:         true,
		Status:          "unknown",
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
			ArtifactDir:     artifactRoot,
			Sandbox: config.PluginSandboxConfig{
				ExecTimeoutMs:      30000,
				MaxMemoryMB:        128,
				MaxConcurrency:     4,
				JSWorkerSocketPath: "tcp://" + workerAddr,
				JSWorkerAutoStart:  false,
			},
		},
	})
	svc.Start()
	defer svc.Stop()

	hookResult, err := svc.ExecuteHook(HookExecutionRequest{
		Hook: "frontend.bootstrap",
		Payload: map[string]interface{}{
			"area": "admin",
			"path": "/admin/plugin-pages/bootstrap-route-test",
			"slot": "bootstrap",
		},
	}, &ExecutionContext{
		Metadata: map[string]string{
			PluginScopeMetadataAuthenticated: "true",
			PluginScopeMetadataSuperAdmin:    "true",
		},
	})
	if err != nil {
		t.Fatalf("frontend.bootstrap ExecuteHook failed: %v", err)
	}
	if hookResult == nil {
		t.Fatalf("expected hook result")
	}
	if len(hookResult.PluginResults) != 1 || !hookResult.PluginResults[0].Success {
		t.Fatalf("expected single successful bootstrap hook result, got %+v", hookResult.PluginResults)
	}
	if len(hookResult.FrontendExtensions) != 2 {
		t.Fatalf("expected bootstrap frontend extensions to bypass slot whitelist, got %+v", hookResult.FrontendExtensions)
	}
}

func TestPluginManagerServiceExecuteHookParallelizesReadOnlyPlugins(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	serverAddr, cleanupGRPC := startTestPluginGRPCServer(t)
	defer cleanupGRPC()

	pluginA := models.Plugin{
		Name:            "grpc-hook-parallel-a",
		DisplayName:     "gRPC Hook Parallel A",
		Type:            "custom",
		Runtime:         PluginRuntimeGRPC,
		Address:         serverAddr,
		Version:         "1.0.0",
		RuntimeParams:   `{"delay_ms":"350","plugin_label":"A","priority":"20"}`,
		Capabilities:    mustMarshalCapabilityMap(t, map[string]interface{}{"hooks": []string{"test.parallel.event"}, "requested_permissions": []string{PluginPermissionHookExecute, PluginPermissionFrontendExtension}, "granted_permissions": []string{PluginPermissionHookExecute, PluginPermissionFrontendExtension}, "allowed_frontend_slots": []string{"user.orders.top"}}),
		Enabled:         true,
		Status:          "unknown",
		LifecycleStatus: models.PluginLifecycleInstalled,
	}
	pluginB := models.Plugin{
		Name:            "grpc-hook-parallel-b",
		DisplayName:     "gRPC Hook Parallel B",
		Type:            "custom",
		Runtime:         PluginRuntimeGRPC,
		Address:         serverAddr,
		Version:         "1.0.0",
		RuntimeParams:   `{"delay_ms":"350","plugin_label":"B","priority":"10"}`,
		Capabilities:    mustMarshalCapabilityMap(t, map[string]interface{}{"hooks": []string{"test.parallel.event"}, "requested_permissions": []string{PluginPermissionHookExecute, PluginPermissionFrontendExtension}, "granted_permissions": []string{PluginPermissionHookExecute, PluginPermissionFrontendExtension}, "allowed_frontend_slots": []string{"user.orders.top"}}),
		Enabled:         true,
		Status:          "unknown",
		LifecycleStatus: models.PluginLifecycleInstalled,
	}
	if err := db.Create(&pluginA).Error; err != nil {
		t.Fatalf("create plugin A failed: %v", err)
	}
	if err := db.Create(&pluginB).Error; err != nil {
		t.Fatalf("create plugin B failed: %v", err)
	}

	svc := NewPluginManagerService(db, &config.Config{
		Plugin: config.PluginPlatformConfig{
			Enabled:         true,
			AllowedRuntimes: []string{PluginRuntimeGRPC},
			DefaultRuntime:  PluginRuntimeGRPC,
			Execution: config.PluginExecutionPolicyConfig{
				HookMaxInFlight:     2,
				HookMaxRetries:      0,
				HookRetryBackoffMs:  10,
				HookBeforeTimeoutMs: 2000,
				HookAfterTimeoutMs:  2000,
			},
		},
	})
	svc.Start()
	defer svc.Stop()

	start := time.Now()
	hookResult, err := svc.ExecuteHook(HookExecutionRequest{
		Hook:    "test.parallel.event",
		Payload: map[string]interface{}{"seed": "origin"},
	}, nil)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("parallel ExecuteHook failed: %v", err)
	}
	if hookResult == nil {
		t.Fatalf("expected hook result")
	}
	if len(hookResult.PluginResults) != 2 {
		t.Fatalf("expected two plugin hook results, got %+v", hookResult.PluginResults)
	}
	if !hookResult.PluginResults[0].Success || !hookResult.PluginResults[1].Success {
		t.Fatalf("expected successful parallel hook results, got %+v", hookResult.PluginResults)
	}
	if hookResult.PluginResults[0].PluginID != pluginA.ID || hookResult.PluginResults[1].PluginID != pluginB.ID {
		t.Fatalf("expected deterministic plugin result order by plugin id, got %+v", hookResult.PluginResults)
	}
	if len(hookResult.FrontendExtensions) != 2 {
		t.Fatalf("expected two frontend extensions, got %+v", hookResult.FrontendExtensions)
	}
	if hookResult.FrontendExtensions[0].Title != "B" || hookResult.FrontendExtensions[1].Title != "A" {
		t.Fatalf("expected frontend extensions sorted by priority, got %+v", hookResult.FrontendExtensions)
	}
	if elapsed >= 650*time.Millisecond {
		t.Fatalf("expected hook execution to complete in parallel (<650ms), got %s", elapsed)
	}
}

func TestPluginManagerServiceExecutionBreakerTripsAndAllowsSingleHalfOpenProbe(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	serverAddr, cleanupGRPC := startTestPluginGRPCServer(t)
	defer cleanupGRPC()

	plugin := models.Plugin{
		Name:            "grpc-breaker-plugin",
		DisplayName:     "gRPC Breaker Plugin",
		Type:            "custom",
		Runtime:         PluginRuntimeGRPC,
		Address:         serverAddr,
		Version:         "1.0.0",
		Capabilities:    mustMarshalCapabilities(t, PluginPermissionExecuteAPI, PluginPermissionHookExecute, PluginPermissionHookPayloadPatch),
		Enabled:         true,
		Status:          "unknown",
		LifecycleStatus: models.PluginLifecycleInstalled,
	}
	if err := db.Create(&plugin).Error; err != nil {
		t.Fatalf("create plugin failed: %v", err)
	}

	svc := NewPluginManagerService(db, &config.Config{
		Plugin: config.PluginPlatformConfig{
			Enabled:         true,
			AllowedRuntimes: []string{PluginRuntimeGRPC},
			DefaultRuntime:  PluginRuntimeGRPC,
			GRPC: config.PluginGRPCTransportConfig{
				Mode: "insecure_local",
			},
			Sandbox: config.PluginSandboxConfig{
				ExecTimeoutMs:  30000,
				MaxMemoryMB:    128,
				MaxConcurrency: 4,
			},
			Execution: config.PluginExecutionPolicyConfig{
				HookMaxInFlight:     1,
				HookMaxRetries:      0,
				HookRetryBackoffMs:  10,
				HookBeforeTimeoutMs: 2000,
				HookAfterTimeoutMs:  2000,
				FailureThreshold:    2,
				FailureCooldownMs:   200,
			},
		},
	})
	svc.Start()
	defer svc.Stop()

	for attempt := 0; attempt < 2; attempt++ {
		if _, err := svc.ExecutePlugin(plugin.ID, "always.error", nil, nil); err == nil {
			t.Fatalf("expected forced plugin failure on attempt %d", attempt+1)
		}
	}
	refreshedAfterFailures, err := svc.getPluginByID(plugin.ID)
	if err != nil {
		t.Fatalf("load plugin after failures failed: %v", err)
	}
	breakerAfterFailures := svc.InspectPluginRuntime(refreshedAfterFailures)
	if breakerAfterFailures.BreakerState != pluginBreakerStateOpen {
		t.Fatalf("expected breaker=open after repeated failures, got %+v", breakerAfterFailures)
	}

	openHookResult, err := svc.ExecuteHook(HookExecutionRequest{
		Hook:    "test.custom.event",
		Payload: map[string]interface{}{"seed": "origin"},
	}, nil)
	if err != nil {
		t.Fatalf("ExecuteHook during open breaker failed: %v", err)
	}
	if openHookResult == nil {
		t.Fatalf("expected open breaker hook result")
	}
	if len(openHookResult.PluginResults) != 0 {
		t.Fatalf("expected open breaker to skip plugin entirely, got %+v", openHookResult.PluginResults)
	}
	if got := stringifyAny(openHookResult.Payload["seed"]); got != "origin" {
		t.Fatalf("expected payload to remain unchanged while breaker open, got %+v", openHookResult.Payload)
	}

	time.Sleep(260 * time.Millisecond)

	firstProbeDone := make(chan error, 1)
	go func() {
		_, probeErr := svc.ExecutePlugin(plugin.ID, "probe.delay", map[string]string{"delay_ms": "250"}, nil)
		firstProbeDone <- probeErr
	}()
	time.Sleep(60 * time.Millisecond)

	_, err = svc.ExecutePlugin(plugin.ID, "probe.delay", map[string]string{"delay_ms": "10"}, nil)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "half-open") {
		t.Fatalf("expected second request to be blocked by half-open probe, got %v", err)
	}

	if probeErr := <-firstProbeDone; probeErr != nil {
		t.Fatalf("expected first half-open probe to succeed, got %v", probeErr)
	}

	postProbeResult, err := svc.ExecutePlugin(plugin.ID, "ping", map[string]string{"alpha": "1"}, nil)
	if err != nil {
		t.Fatalf("expected plugin execution after successful probe, got %v", err)
	}
	if postProbeResult == nil || !postProbeResult.Success {
		t.Fatalf("expected successful execution after probe recovery, got %+v", postProbeResult)
	}

	var refreshed models.Plugin
	if err := db.First(&refreshed, plugin.ID).Error; err != nil {
		t.Fatalf("query refreshed plugin failed: %v", err)
	}
	if refreshed.FailCount != 0 || refreshed.Status != "healthy" || refreshed.LifecycleStatus != models.PluginLifecycleRunning {
		t.Fatalf("expected breaker recovery to reset plugin health, got %+v", refreshed)
	}
}

func TestPluginManagerServiceGRPCHotReloadDrainsOldRuntimeSlot(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	serverAddrA, cleanupGRPCA := startNamedTestPluginGRPCServer(t, "A")
	defer cleanupGRPCA()
	serverAddrB, cleanupGRPCB := startNamedTestPluginGRPCServer(t, "B")
	defer cleanupGRPCB()

	plugin := models.Plugin{
		Name:              "grpc-hot-reload-plugin",
		DisplayName:       "gRPC Hot Reload Plugin",
		Type:              "custom",
		Runtime:           PluginRuntimeGRPC,
		Address:           serverAddrA,
		Version:           "1.0.0",
		Capabilities:      mustMarshalCapabilities(t, PluginPermissionExecuteAPI),
		Enabled:           true,
		Status:            "unknown",
		LifecycleStatus:   models.PluginLifecycleInstalled,
		DesiredGeneration: 1,
		AppliedGeneration: 1,
	}
	if err := db.Create(&plugin).Error; err != nil {
		t.Fatalf("create plugin failed: %v", err)
	}

	svc := NewPluginManagerService(db, &config.Config{
		Plugin: config.PluginPlatformConfig{
			Enabled:         true,
			AllowedRuntimes: []string{PluginRuntimeGRPC},
			DefaultRuntime:  PluginRuntimeGRPC,
			GRPC: config.PluginGRPCTransportConfig{
				Mode: "insecure_local",
			},
			Sandbox: config.PluginSandboxConfig{
				ExecTimeoutMs:  30000,
				MaxMemoryMB:    128,
				MaxConcurrency: 4,
			},
		},
	})
	svc.Start()
	defer svc.Stop()

	longRunningDone := make(chan *ExecutionResult, 1)
	longRunningErr := make(chan error, 1)
	go func() {
		result, err := svc.ExecutePlugin(plugin.ID, "ping", map[string]string{"delay_ms": "250"}, nil)
		if err != nil {
			longRunningErr <- err
			return
		}
		longRunningDone <- result
	}()
	time.Sleep(60 * time.Millisecond)

	if err := db.Model(&models.Plugin{}).Where("id = ?", plugin.ID).Updates(map[string]interface{}{
		"address":            serverAddrB,
		"desired_generation": 2,
		"applied_generation": 2,
	}).Error; err != nil {
		t.Fatalf("update plugin address failed: %v", err)
	}
	if err := svc.ReloadPlugin(plugin.ID); err != nil {
		t.Fatalf("ReloadPlugin failed: %v", err)
	}

	var reloaded models.Plugin
	if err := db.First(&reloaded, plugin.ID).Error; err != nil {
		t.Fatalf("load reloaded plugin failed: %v", err)
	}
	runtimeAfterReload := svc.InspectPluginRuntime(&reloaded)
	if runtimeAfterReload.ActiveGeneration != 2 {
		t.Fatalf("expected active generation=2 after hot reload, got %+v", runtimeAfterReload)
	}
	if runtimeAfterReload.DrainingSlotCount != 1 {
		t.Fatalf("expected one draining runtime slot after cutover, got %+v", runtimeAfterReload)
	}

	newResult, err := svc.ExecutePlugin(plugin.ID, "ping", nil, nil)
	if err != nil {
		t.Fatalf("execute on new runtime slot failed: %v", err)
	}
	if got := stringifyAny(newResult.Data["server_label"]); got != "B" {
		t.Fatalf("expected new execution to use server B, got %q", got)
	}

	select {
	case err := <-longRunningErr:
		t.Fatalf("expected old in-flight request to complete successfully, got %v", err)
	case result := <-longRunningDone:
		if got := stringifyAny(result.Data["server_label"]); got != "A" {
			t.Fatalf("expected old in-flight request to stay on server A, got %q", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for old in-flight request to finish")
	}

	waitDeadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(waitDeadline) {
		runtimeState := svc.InspectPluginRuntime(&reloaded)
		if runtimeState.DrainingSlotCount == 0 && runtimeState.DrainingInFlight == 0 {
			return
		}
		time.Sleep(30 * time.Millisecond)
	}
	t.Fatalf("expected draining runtime slot to be released after in-flight request completion")
}

type testPluginGRPCServer struct {
	pb.UnimplementedPluginServiceServer
	label string
}

func (s *testPluginGRPCServer) HealthCheck(context.Context, *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	label := strings.TrimSpace(s.label)
	if label == "" {
		label = "1.0.0"
	}
	return &pb.HealthCheckResponse{
		Healthy: true,
		Version: "test-grpc/" + label,
		Metadata: map[string]string{
			"runtime": "grpc",
		},
	}, nil
}

func (s *testPluginGRPCServer) Execute(_ context.Context, req *pb.ExecuteRequest) (*pb.ExecuteResponse, error) {
	payload := map[string]interface{}{}
	if delayMs, err := strconv.Atoi(strings.TrimSpace(req.GetParams()["delay_ms"])); err == nil && delayMs > 0 {
		time.Sleep(time.Duration(delayMs) * time.Millisecond)
	}
	switch req.GetAction() {
	case "always.error":
		return nil, fmt.Errorf("forced plugin failure")
	case "hook.execute":
		if req.GetParams()["hook"] == "test.parallel.event" {
			priority, _ := strconv.Atoi(strings.TrimSpace(req.GetParams()["priority"]))
			label := strings.TrimSpace(req.GetParams()["plugin_label"])
			if label == "" {
				label = "parallel"
			}
			payload["frontend_extensions"] = []map[string]interface{}{
				{
					"type":     "text",
					"slot":     "user.orders.top",
					"title":    label,
					"content":  "parallel",
					"priority": priority,
				},
			}
			break
		}
		payload["payload"] = map[string]interface{}{
			"grpc_hook": req.GetParams()["hook"],
		}
	default:
		payload["action"] = req.GetAction()
		payload["params"] = req.GetParams()
		payload["server_label"] = strings.TrimSpace(s.label)
		if req.GetContext() != nil {
			payload["session_id"] = req.GetContext().GetSessionId()
			payload["user_id"] = req.GetContext().GetUserId()
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &pb.ExecuteResponse{
		Success: true,
		Data:    string(body),
		Metadata: map[string]string{
			"runtime": "grpc",
		},
	}, nil
}

func (s *testPluginGRPCServer) ExecuteStream(req *pb.ExecuteRequest, stream pb.PluginService_ExecuteStreamServer) error {
	if req.GetAction() == "stream.wait_cancel" {
		<-stream.Context().Done()
		return stream.Context().Err()
	}
	if req.GetAction() == "stream.echo" {
		if err := stream.Send(&pb.ExecuteResponse{
			Success: false,
			Data:    `{"status":"running","progress":50}`,
			IsFinal: false,
			Metadata: map[string]string{
				"runtime": "grpc",
			},
		}); err != nil {
			return err
		}

		payload := map[string]interface{}{
			"action": req.GetAction(),
			"params": req.GetParams(),
			"status": "completed",
		}
		if req.GetContext() != nil {
			payload["session_id"] = req.GetContext().GetSessionId()
			payload["user_id"] = req.GetContext().GetUserId()
		}
		body, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		return stream.Send(&pb.ExecuteResponse{
			Success: true,
			Data:    string(body),
			IsFinal: true,
			Metadata: map[string]string{
				"runtime": "grpc",
				"stream":  "true",
			},
		})
	}

	if err := stream.Send(&pb.ExecuteResponse{
		Success: false,
		Data:    `{"status":"running","progress":50}`,
		IsFinal: false,
		Metadata: map[string]string{
			"runtime": "grpc",
		},
	}); err != nil {
		return err
	}
	return stream.Send(&pb.ExecuteResponse{
		Success: true,
		Data:    `{"status":"completed","progress":100}`,
		IsFinal: true,
		Metadata: map[string]string{
			"runtime": "grpc",
			"stream":  "true",
		},
	})
}

func openPluginManagerE2ETestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:plugin-manager-e2e-" + time.Now().UTC().Format("20060102150405.000000000") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Plugin{},
		&models.PluginVersion{},
		&models.PluginExecution{},
		&models.PluginStorageEntry{},
		&models.PluginSecretEntry{},
		&models.PluginPageRuleEntry{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	return db
}

func waitForPluginExecutionRecords(t *testing.T, db *gorm.DB, pluginID uint, minCount int) []models.PluginExecution {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for {
		var executions []models.PluginExecution
		if err := db.Where("plugin_id = ?", pluginID).Find(&executions).Error; err != nil {
			t.Fatalf("query plugin executions failed: %v", err)
		}
		if len(executions) >= minCount {
			return executions
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected plugin %d execution records >= %d, got %d", pluginID, minCount, len(executions))
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func startTestPluginGRPCServer(t *testing.T) (string, func()) {
	return startNamedTestPluginGRPCServer(t, "")
}

func startNamedTestPluginGRPCServer(t *testing.T, label string) (string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen grpc test server failed: %v", err)
	}
	server := grpc.NewServer()
	pb.RegisterPluginServiceServer(server, &testPluginGRPCServer{label: label})
	go func() {
		_ = server.Serve(listener)
	}()
	return listener.Addr().String(), func() {
		server.Stop()
		_ = listener.Close()
	}
}

func startTestJSWorker(t *testing.T, artifactRoot string, extraArgs ...string) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve js worker address failed: %v", err)
	}
	addr := listener.Addr().String()
	_ = listener.Close()

	go func() {
		args := []string{
			"-network", "tcp",
			"-socket", addr,
			"-artifact-root", artifactRoot,
			"-timeout-ms", "30000",
			"-max-concurrency", "4",
			"-max-memory-mb", "128",
		}
		args = append(args, extraArgs...)
		_ = jsworker.Run(args)
	}()
	waitForTCPReady(t, addr, 5*time.Second)
	return addr
}

func waitForTCPReady(t *testing.T, addr string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("tcp endpoint did not become ready: %s", addr)
}

func writeJSE2ETestPlugin(t *testing.T, artifactRoot string) string {
	t.Helper()
	pluginDir := filepath.Join(artifactRoot, "jsworker", "js-e2e-plugin", "1.0.0", "test")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("mkdir js plugin dir failed: %v", err)
	}
	scriptPath := filepath.Join(pluginDir, "index.js")
	script := strings.TrimSpace(`
module.exports.execute = function execute(action, params) {
  var storage = globalThis.Plugin && globalThis.Plugin.storage;
  var fs = globalThis.Plugin && globalThis.Plugin.fs;
  if (action === "kv.set") {
    var saved = storage && typeof storage.set === "function" ? storage.set(params.key, params.value) : false;
    return { success: saved, data: { saved: saved } };
  }
  if (action === "kv.get") {
    var value = storage && typeof storage.get === "function" ? storage.get(params.key) : "";
    return { success: true, data: { value: value } };
  }
  if (action === "hook.execute") {
    return { success: true, data: { payload: { js_hook: params.hook } } };
  }
  if (action === "fs.probe") {
    if (!fs || !fs.enabled) {
      return { success: false, error: "fs disabled", data: { fs_enabled: !!(fs && fs.enabled) } };
    }
    fs.writeText("notes/probe.txt", "ok");
    var usage = fs.usage();
    var recalculated = fs.recalculateUsage();
    return {
      success: true,
      data: {
        fs_enabled: !!fs.enabled,
        exists_after_write: fs.exists("notes/probe.txt"),
        content: fs.readText("notes/probe.txt"),
        usage_file_count: usage && usage.file_count,
        usage_max_files: usage && usage.max_files,
        recalculated_file_count: recalculated && recalculated.file_count
      }
    };
  }
  return { success: true, data: { action: action } };
};

module.exports.health = function health() {
  return {
    healthy: true,
    version: "test-js/1.0.0",
    metadata: {
      runtime: "goja"
    }
  };
};
`)
	if err := os.WriteFile(scriptPath, []byte(script), 0644); err != nil {
		t.Fatalf("write js test plugin failed: %v", err)
	}
	return scriptPath
}

func writeJSStreamE2ETestPlugin(t *testing.T, artifactRoot string) string {
	t.Helper()
	pluginDir := filepath.Join(artifactRoot, "jsworker", "js-stream-e2e-plugin", "1.0.0", "test")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("mkdir js stream plugin dir failed: %v", err)
	}
	scriptPath := filepath.Join(pluginDir, "index.js")
	script := strings.TrimSpace(`
module.exports.execute = function execute(action, params, context) {
  return {
    success: true,
    data: {
      action: action,
      params: params || {},
      session_id: context && context.session_id || "",
      status: "completed"
    }
  };
};

module.exports.executeStream = function executeStream(action, params, context, config, sandbox, stream) {
  if (action === "stream.wait_cancel") {
    while (true) {}
  }
  if (action !== "stream.echo") {
    return module.exports.execute(action, params, context, config, sandbox);
  }
  if (stream && typeof stream.progress === "function") {
    stream.progress("preparing", 25, { phase: "prepare" });
  }
  if (stream && typeof stream.write === "function") {
    stream.write({
      status: "running",
      progress: 70,
      action: action
    }, {
      phase: "mid"
    });
  }
  return {
    success: true,
    data: {
      action: action,
      params: params || {},
      session_id: context && context.session_id || "",
      status: "completed"
    }
  };
};

module.exports.health = function health() {
  return {
    healthy: true,
    version: "test-js-stream/1.0.0",
    metadata: {
      runtime: "goja"
    }
  };
};
`)
	if err := os.WriteFile(scriptPath, []byte(script), 0644); err != nil {
		t.Fatalf("write js stream test plugin failed: %v", err)
	}
	return scriptPath
}

func writeJSFrontendBootstrapTestPlugin(t *testing.T, artifactRoot string) string {
	t.Helper()
	pluginDir := filepath.Join(artifactRoot, "jsworker", "js-bootstrap-route-plugin", "1.0.0", "test")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("mkdir js bootstrap plugin dir failed: %v", err)
	}
	scriptPath := filepath.Join(pluginDir, "index.js")
	script := strings.TrimSpace(`
module.exports.execute = function execute(action, params) {
  if (action === "hook.execute" && params && params.hook === "frontend.bootstrap") {
    return {
      success: true,
      data: {
        frontend_extensions: [
          {
            type: "menu_item",
            data: {
              area: "admin",
              path: "/admin/plugin-pages/bootstrap-route-test",
              title: "Bootstrap Route Test"
            }
          },
          {
            type: "route_page",
            data: {
              area: "admin",
              path: "/admin/plugin-pages/bootstrap-route-test",
              title: "Bootstrap Route Test"
            }
          }
        ]
      }
    };
  }
  return { success: true, data: {} };
};

module.exports.health = function health() {
  return {
    healthy: true,
    version: "test-js/1.0.0",
    metadata: {
      runtime: "goja"
    }
  };
};
`)
	if err := os.WriteFile(scriptPath, []byte(script), 0644); err != nil {
		t.Fatalf("write js bootstrap test plugin failed: %v", err)
	}
	return scriptPath
}

func mustMarshalCapabilities(t *testing.T, permissions ...string) string {
	t.Helper()
	body, err := json.Marshal(map[string]interface{}{
		"requested_permissions": permissions,
		"granted_permissions":   permissions,
	})
	if err != nil {
		t.Fatalf("marshal capabilities failed: %v", err)
	}
	return string(body)
}

func mustMarshalCapabilityMap(t *testing.T, payload map[string]interface{}) string {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal capability map failed: %v", err)
	}
	return string(body)
}

func stringifyAny(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	case float64:
		if typed == float64(int64(typed)) {
			return fmt.Sprintf("%d", int64(typed))
		}
	}
	return fmt.Sprintf("%v", value)
}

func isExecutionCanceledError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "context canceled")
}
