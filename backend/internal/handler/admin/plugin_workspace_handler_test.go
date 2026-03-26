package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/models"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
)

func TestSanitizeUserProvidedExecutionMetadataStripsPluginScopeKeys(t *testing.T) {
	metadata := sanitizeUserProvidedExecutionMetadata(map[string]string{
		service.PluginScopeMetadataAuthenticated: "false",
		service.PluginScopeMetadataSuperAdmin:    "true",
		service.PluginScopeMetadataPermissions:   "order.view",
		"custom":                                 "kept",
	})

	if _, exists := metadata[service.PluginScopeMetadataAuthenticated]; exists {
		t.Fatalf("expected authenticated scope metadata to be stripped")
	}
	if _, exists := metadata[service.PluginScopeMetadataSuperAdmin]; exists {
		t.Fatalf("expected super admin scope metadata to be stripped")
	}
	if _, exists := metadata[service.PluginScopeMetadataPermissions]; exists {
		t.Fatalf("expected permissions scope metadata to be stripped")
	}
	if metadata["custom"] != "kept" {
		t.Fatalf("expected custom metadata to remain, got %#v", metadata["custom"])
	}
}

func TestApplyPluginWorkspaceRuntimeTaskID(t *testing.T) {
	execCtx := applyPluginWorkspaceRuntimeTaskID(&service.ExecutionContext{}, " pex_runtime_1 ")
	if execCtx == nil {
		t.Fatalf("expected execution context")
	}
	if got := execCtx.Metadata[service.PluginExecutionMetadataID]; got != "pex_runtime_1" {
		t.Fatalf("expected runtime task id to be applied, got %q", got)
	}

	unchanged := applyPluginWorkspaceRuntimeTaskID(execCtx, "   ")
	if got := unchanged.Metadata[service.PluginExecutionMetadataID]; got != "pex_runtime_1" {
		t.Fatalf("expected blank task id to preserve existing execution id, got %q", got)
	}
}

func TestResolvePluginWorkspaceSignalTaskID(t *testing.T) {
	if got := resolvePluginWorkspaceSignalTaskID(" task-1 ", service.PluginWorkspaceSnapshot{}); got != "task-1" {
		t.Fatalf("expected explicit task id to win, got %q", got)
	}
	if got := resolvePluginWorkspaceSignalTaskID("", service.PluginWorkspaceSnapshot{ActiveTaskID: "task-2"}); got != "task-2" {
		t.Fatalf("expected active task id fallback, got %q", got)
	}
}

func TestSanitizePluginWorkspaceSnapshotForAdminKeepsFrontendFieldsOnly(t *testing.T) {
	sanitized := sanitizePluginWorkspaceSnapshotForAdmin(service.PluginWorkspaceSnapshot{
		PluginID:                  7,
		PluginName:                "debugger",
		Runtime:                   "js_worker",
		Enabled:                   true,
		OwnerAdminID:              42,
		OwnerLastActiveAt:         time.Now().UTC(),
		ViewerCount:               2,
		ControlGranted:            true,
		ControlIdleTimeoutSeconds: 300,
		Status:                    "running",
		ActiveTaskID:              "pex_1",
		ActiveCommand:             "debugger/report",
		ActiveCommandID:           "cmd_1",
		Interactive:               true,
		WaitingInput:              false,
		Prompt:                    ">",
		StartedAt:                 time.Now().UTC(),
		CompletedAt:               nil,
		CompletionReason:          "completed",
		LastError:                 "boom",
		BufferCapacity:            128,
		EntryCount:                1,
		LastSeq:                   2,
		UpdatedAt:                 time.Now().UTC(),
		HasMore:                   true,
		RecentControlEvents:       []service.PluginWorkspaceControlEvent{{Seq: 1, Type: "control_assigned"}},
		Entries:                   []service.PluginWorkspaceBufferEntry{{Seq: 2, Message: "hello"}},
	})

	if _, exists := sanitized["plugin_id"]; exists {
		t.Fatalf("expected plugin_id to be stripped, got %#v", sanitized)
	}
	if _, exists := sanitized["plugin_name"]; exists {
		t.Fatalf("expected plugin_name to be stripped, got %#v", sanitized)
	}
	if _, exists := sanitized["active_command_id"]; exists {
		t.Fatalf("expected active_command_id to be stripped, got %#v", sanitized)
	}
	if _, exists := sanitized["interactive"]; exists {
		t.Fatalf("expected interactive to be stripped, got %#v", sanitized)
	}
	if got := sanitized["control_granted"]; got != true {
		t.Fatalf("expected control_granted to remain, got %#v", got)
	}
	if got := sanitized["active_task_id"]; got != "pex_1" {
		t.Fatalf("expected active_task_id to remain, got %#v", got)
	}
	if entries, ok := sanitized["entries"].([]service.PluginWorkspaceBufferEntry); !ok || len(entries) != 1 || entries[0].Message != "hello" {
		t.Fatalf("expected entries to remain, got %#v", sanitized["entries"])
	}
}

func TestSanitizePluginWorkspaceRuntimeStateForAdminKeepsFrontendFieldsOnly(t *testing.T) {
	sanitized := sanitizePluginWorkspaceRuntimeStateForAdmin(service.PluginWorkspaceRuntimeState{
		Available:       true,
		PluginID:        7,
		Generation:      3,
		InstanceID:      "jsrt_1",
		CompletionPaths: []string{"Plugin.order.list"},
		CallablePaths:   []string{"host.order.list"},
		RefCount:        9,
		Disposed:        true,
	})

	state, ok := sanitized.(gin.H)
	if !ok {
		t.Fatalf("expected sanitized runtime state, got %#v", sanitized)
	}
	if _, exists := state["callable_paths"]; exists {
		t.Fatalf("expected callable paths to be stripped, got %#v", state)
	}
	if _, exists := state["plugin_id"]; exists {
		t.Fatalf("expected plugin_id to be stripped, got %#v", state)
	}
	if _, exists := state["generation"]; exists {
		t.Fatalf("expected generation to be stripped, got %#v", state)
	}
	if _, exists := state["ref_count"]; exists {
		t.Fatalf("expected ref_count to be stripped, got %#v", state)
	}
	if completionPaths, ok := state["completion_paths"].([]string); !ok || len(completionPaths) != 1 || completionPaths[0] != "Plugin.order.list" {
		t.Fatalf("expected completion paths to remain, got %#v", state["completion_paths"])
	}
}

func TestExtractPluginWorkspaceRuntimeStateForAdminSanitizesNestedPayload(t *testing.T) {
	sanitized := extractPluginWorkspaceRuntimeStateForAdmin(gin.H{
		"runtime_state": gin.H{
			"available":        true,
			"completion_paths": []string{"Plugin.order.list"},
			"callable_paths":   []string{"host.order.list"},
		},
	})

	state, ok := sanitized.(map[string]interface{})
	if !ok {
		t.Fatalf("expected sanitized runtime state map, got %#v", sanitized)
	}
	if _, exists := state["callable_paths"]; exists {
		t.Fatalf("expected callable_paths to be stripped, got %#v", state)
	}
	if completionPaths, ok := state["completion_paths"].([]string); !ok || len(completionPaths) != 1 || completionPaths[0] != "Plugin.order.list" {
		t.Fatalf("expected completion paths to remain, got %#v", state["completion_paths"])
	}
}

func TestSanitizePluginWorkspaceRuntimeResponseDataForAdminRemovesEmbeddedRuntimeState(t *testing.T) {
	sanitized := sanitizePluginWorkspaceRuntimeResponseDataForAdmin(gin.H{
		"summary":       "2",
		"type":          "number",
		"runtime_state": gin.H{"available": true},
	})

	data, ok := sanitized.(map[string]interface{})
	if !ok {
		t.Fatalf("expected sanitized runtime response data, got %#v", sanitized)
	}
	if _, exists := data["runtime_state"]; exists {
		t.Fatalf("expected embedded runtime_state to be stripped, got %#v", data)
	}
	if data["summary"] != "2" || data["type"] != "number" {
		t.Fatalf("expected non-runtime_state fields to remain, got %#v", data)
	}
}

func TestBuildPluginWorkspaceExecutionContextSuppliesOperatorScopeToHostActions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := openPluginUploadTestDB(t)
	if err := db.AutoMigrate(&models.User{}, &models.AdminPermission{}, &models.Order{}, &models.Announcement{}); err != nil {
		t.Fatalf("auto migrate workspace handler models failed: %v", err)
	}

	adminID := uint(42)
	if err := db.Create(&models.AdminPermission{
		UserID:      adminID,
		Permissions: []string{"announcement.view", "order.view"},
	}).Error; err != nil {
		t.Fatalf("create admin permissions failed: %v", err)
	}

	handler := NewPluginHandler(db, nil, "")
	request := httptest.NewRequest(http.MethodPost, "/api/admin/plugins/7/workspace/runtime/eval", nil)
	request.Header.Set("User-Agent", "workspace-test")
	request.Header.Set("Accept-Language", "zh-CN")
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = request
	ctx.Set("user_id", adminID)
	ctx.Set("user_role", "admin")

	execCtx := handler.buildPluginWorkspaceExecutionContext(ctx, &service.ExecutionContext{
		Metadata: map[string]string{
			service.PluginScopeMetadataAuthenticated: "false",
			service.PluginScopeMetadataPermissions:   "user.view",
			"custom":                                 "value",
		},
	})
	if execCtx == nil {
		t.Fatalf("expected workspace execution context")
	}
	if execCtx.UserID == nil || *execCtx.UserID != adminID {
		t.Fatalf("expected workspace execution context to default user id to admin %d, got %#v", adminID, execCtx.UserID)
	}
	if execCtx.OperatorUserID == nil || *execCtx.OperatorUserID != adminID {
		t.Fatalf("expected workspace execution context to carry operator user id %d, got %#v", adminID, execCtx.OperatorUserID)
	}
	if got := execCtx.Metadata[service.PluginScopeMetadataAuthenticated]; got != "true" {
		t.Fatalf("expected authenticated scope metadata, got %q", got)
	}
	if got := execCtx.Metadata[service.PluginScopeMetadataSuperAdmin]; got != "false" {
		t.Fatalf("expected non-super-admin scope metadata, got %q", got)
	}
	if got := execCtx.Metadata[service.PluginScopeMetadataPermissions]; got != "announcement.view,order.view" {
		t.Fatalf("unexpected scope permissions metadata: %q", got)
	}
	if execCtx.Metadata["custom"] != "value" {
		t.Fatalf("expected custom metadata to survive workspace context build, got %#v", execCtx.Metadata["custom"])
	}

	claims := service.BuildPluginHostAccessClaims(nil, execCtx, time.Minute)
	claims.GrantedPermissions = []string{
		service.PluginPermissionHostOrderList,
		service.PluginPermissionHostAnnouncementList,
	}

	if _, err := service.ExecutePluginHostAction(db, &claims, "host.order.list", map[string]interface{}{}); err != nil {
		t.Fatalf("expected host.order.list to succeed with workspace scope, got %v", err)
	}
	if _, err := service.ExecutePluginHostAction(db, &claims, "host.announcement.list", map[string]interface{}{}); err != nil {
		t.Fatalf("expected host.announcement.list to succeed with workspace scope, got %v", err)
	}
}

func TestResolveJSWorkerWorkspacePluginRejectsDisabledPlugin(t *testing.T) {
	db := openPluginUploadTestDB(t)
	manager := service.NewPluginManagerService(db, &config.Config{
		Plugin: config.PluginPlatformConfig{
			Enabled:         true,
			AllowedRuntimes: []string{service.PluginRuntimeGRPC, service.PluginRuntimeJSWorker},
			DefaultRuntime:  service.PluginRuntimeJSWorker,
		},
	})
	handler := NewPluginHandler(db, manager, "")

	plugin := models.Plugin{
		Name:    "disabled-admin-workspace-plugin",
		Type:    "tool",
		Runtime: service.PluginRuntimeJSWorker,
		Address: "index.js",
		Enabled: false,
	}
	if err := db.Create(&plugin).Error; err != nil {
		t.Fatalf("create plugin failed: %v", err)
	}
	if err := db.Model(&plugin).Update("enabled", false).Error; err != nil {
		t.Fatalf("disable plugin failed: %v", err)
	}

	if _, err := handler.resolveJSWorkerWorkspacePlugin(plugin.ID); err == nil {
		t.Fatalf("expected disabled plugin to be rejected")
	} else if !strings.Contains(strings.ToLower(err.Error()), "disabled") {
		t.Fatalf("expected disabled error, got %v", err)
	}
}

func TestBuildPluginWorkspaceExecutionContextPreservesSubjectUserAndSetsOperator(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adminID := uint(42)
	subjectUserID := uint(7)
	request := httptest.NewRequest(http.MethodPost, "/api/admin/plugins/7/workspace/execute", nil)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = request
	ctx.Set("user_id", adminID)
	ctx.Set("user_role", "admin")

	handler := NewPluginHandler(nil, nil, "")
	execCtx := handler.buildPluginWorkspaceExecutionContext(ctx, &service.ExecutionContext{
		UserID: &subjectUserID,
	})
	if execCtx == nil {
		t.Fatalf("expected execution context")
	}
	if execCtx.UserID == nil || *execCtx.UserID != subjectUserID {
		t.Fatalf("expected subject user id %d to remain, got %#v", subjectUserID, execCtx.UserID)
	}
	if execCtx.OperatorUserID == nil || *execCtx.OperatorUserID != adminID {
		t.Fatalf("expected operator user id %d, got %#v", adminID, execCtx.OperatorUserID)
	}

	claims := service.BuildPluginHostAccessClaims(nil, execCtx, time.Minute)
	if claims.OperatorUserID != adminID {
		t.Fatalf("expected operator claims user id %d, got %d", adminID, claims.OperatorUserID)
	}
}
