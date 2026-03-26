package admin

import (
	"testing"
	"time"

	"auralogic/internal/models"
	"auralogic/internal/service"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBuildPluginPublicCacheDiagnosticsFiltersByPlugin(t *testing.T) {
	handler := &PluginHandler{
		publicExtensionsCache: newPluginPublicEndpointCache(time.Minute, 16),
		publicBootstrapCache:  newPluginPublicEndpointCache(time.Minute, 16),
	}
	plugin := &models.Plugin{
		ID:   7,
		Name: "diagnostic-plugin",
	}

	handler.publicExtensionsCache.Set("extensions|demo", frontendExtensionsResponseData{
		Path: "/plugin-pages/demo",
		Slot: "user.plugin_page.top",
		Extensions: []service.FrontendExtension{
			{PluginID: 7, PluginName: "diagnostic-plugin"},
			{PluginID: 8, PluginName: "other-plugin"},
		},
	})
	handler.publicExtensionsCache.Set("extensions|other", frontendExtensionsResponseData{
		Path: "/plugin-pages/other",
		Slot: "user.plugin_page.bottom",
		Extensions: []service.FrontendExtension{
			{PluginID: 8, PluginName: "other-plugin"},
		},
	})
	handler.publicBootstrapCache.Set("bootstrap|demo", frontendBootstrapResponseData{
		Area: "user",
		Path: "/plugin-pages/demo",
		Menus: []frontendBootstrapMenuItem{
			{PluginID: 7, PluginName: "diagnostic-plugin"},
		},
		Routes: []frontendBootstrapRouteItem{
			{PluginID: 7, PluginName: "diagnostic-plugin"},
			{PluginID: 9, PluginName: "third-plugin"},
		},
	})

	diag := handler.buildPluginPublicCacheDiagnostics(plugin)

	if diag.Extensions.TotalEntries != 2 {
		t.Fatalf("expected 2 total extension cache entries, got %+v", diag.Extensions)
	}
	if diag.Extensions.MatchingEntries != 1 {
		t.Fatalf("expected 1 matching extension cache entry, got %+v", diag.Extensions)
	}
	if len(diag.Extensions.Entries) != 1 || diag.Extensions.Entries[0].ExtensionCount != 1 {
		t.Fatalf("expected extension cache entry details for plugin, got %+v", diag.Extensions.Entries)
	}
	if diag.Bootstrap.TotalEntries != 1 {
		t.Fatalf("expected 1 total bootstrap cache entry, got %+v", diag.Bootstrap)
	}
	if diag.Bootstrap.MatchingEntries != 1 {
		t.Fatalf("expected 1 matching bootstrap cache entry, got %+v", diag.Bootstrap)
	}
	if len(diag.Bootstrap.Entries) != 1 {
		t.Fatalf("expected 1 bootstrap cache entry detail, got %+v", diag.Bootstrap.Entries)
	}
	if diag.Bootstrap.Entries[0].MenuCount != 1 || diag.Bootstrap.Entries[0].RouteCount != 1 {
		t.Fatalf("expected bootstrap cache counts to include only plugin entries, got %+v", diag.Bootstrap.Entries[0])
	}
}

func TestBuildPluginDiagnosticIssuesIncludesProtocolCompatibilityFailure(t *testing.T) {
	plugin := models.Plugin{
		ID:       9,
		Name:     "protocol-mismatch-plugin",
		Runtime:  service.PluginRuntimeJSWorker,
		Manifest: `{"manifest_version":"1.0.0","protocol_version":"1.1.0"}`,
	}

	compatibility := service.InspectPluginProtocolCompatibility(&plugin)
	issues := buildPluginDiagnosticIssues(
		plugin,
		service.PluginRuntimeInspection{
			Valid:           true,
			ResolvedRuntime: service.PluginRuntimeJSWorker,
			ConnectionState: "stateless",
		},
		compatibility,
		service.PluginRegistrationInspection{},
		service.ResolveEffectivePluginCapabilityPolicy(&plugin),
		nil,
		pluginStorageDiagnostics{},
		nil,
	)

	found := false
	for _, issue := range issues {
		if issue.Code != "plugin_protocol_incompatible" {
			continue
		}
		found = true
		if issue.Severity != "error" {
			t.Fatalf("expected protocol incompatibility severity error, got %+v", issue)
		}
	}
	if !found {
		t.Fatalf("expected protocol compatibility issue, got %+v", issues)
	}
}

func TestBuildPluginDiagnosticIssuesIncludesCooldownWarning(t *testing.T) {
	plugin := models.Plugin{
		ID:              11,
		Name:            "cooldown-plugin",
		Runtime:         service.PluginRuntimeGRPC,
		Enabled:         true,
		Status:          "unhealthy",
		LifecycleStatus: models.PluginLifecycleDegraded,
	}
	cooldownUntil := time.Date(2026, time.March, 11, 11, 0, 0, 0, time.UTC)

	issues := buildPluginDiagnosticIssues(
		plugin,
		service.PluginRuntimeInspection{
			Valid:           true,
			ResolvedRuntime: service.PluginRuntimeGRPC,
			ConnectionState: "connected",
			CooldownActive:  true,
			CooldownUntil:   &cooldownUntil,
			CooldownReason:  "plugin is cooling down after recent unhealthy/degraded transition",
		},
		service.InspectPluginProtocolCompatibility(&plugin),
		service.PluginRegistrationInspection{},
		service.ResolveEffectivePluginCapabilityPolicy(&plugin),
		nil,
		pluginStorageDiagnostics{},
		nil,
	)

	found := false
	for _, issue := range issues {
		if issue.Code != "plugin_cooldown_active" {
			continue
		}
		found = true
		if issue.Severity != "warn" {
			t.Fatalf("expected cooldown severity warn, got %+v", issue)
		}
		if issue.Detail == "" {
			t.Fatalf("expected cooldown detail, got %+v", issue)
		}
	}
	if !found {
		t.Fatalf("expected cooldown issue, got %+v", issues)
	}
}

func TestBuildPluginExecutionObservabilityDiagnosticsAggregatesFailures(t *testing.T) {
	db := openPluginDiagnosticTestDB(t)
	handler := &PluginHandler{db: db}
	plugin := &models.Plugin{
		Name:    "observability-plugin",
		Type:    "custom",
		Runtime: service.PluginRuntimeGRPC,
		Address: "127.0.0.1:50051",
		Enabled: true,
	}
	if err := db.Create(plugin).Error; err != nil {
		t.Fatalf("create plugin failed: %v", err)
	}

	now := time.Now().UTC()
	executions := []models.PluginExecution{
		{
			PluginID: plugin.ID,
			Action:   "hook.execute",
			Hook:     "order.create.after",
			Params:   `{"hook":"order.create.after"}`,
			Success:  false,
			Error:    "hook failure 1",
			ErrorSignature: service.NormalizePluginExecutionErrorSignature(
				service.NormalizePluginExecutionErrorText("hook failure 1"),
			),
			Duration:  40,
			CreatedAt: now.Add(-10 * time.Minute),
		},
		{
			PluginID: plugin.ID,
			Action:   "hook.execute",
			Hook:     "order.create.after",
			Params:   `{"hook":"order.create.after"}`,
			Success:  false,
			Error:    "hook failure 2",
			ErrorSignature: service.NormalizePluginExecutionErrorSignature(
				service.NormalizePluginExecutionErrorText("hook failure 2"),
			),
			Duration:  50,
			CreatedAt: now.Add(-8 * time.Minute),
		},
		{
			PluginID: plugin.ID,
			Action:   "ping",
			Params:   `{"alpha":"1"}`,
			Success:  false,
			Error:    "manual failure",
			ErrorSignature: service.NormalizePluginExecutionErrorSignature(
				service.NormalizePluginExecutionErrorText("manual failure"),
			),
			Duration:  20,
			CreatedAt: now.Add(-5 * time.Minute),
		},
		{
			PluginID:  plugin.ID,
			Action:    "ping",
			Params:    `{"alpha":"1"}`,
			Success:   true,
			Result:    `{"ok":true}`,
			Duration:  10,
			CreatedAt: now.Add(-2 * time.Minute),
		},
	}
	if err := db.Create(&executions).Error; err != nil {
		t.Fatalf("create executions failed: %v", err)
	}

	diag := handler.buildPluginExecutionObservabilityDiagnostics(plugin)
	if diag.TotalExecutions != 4 {
		t.Fatalf("expected total executions=4, got %+v", diag)
	}
	if diag.FailedExecutions != 3 || diag.HookFailedExecutions != 2 || diag.ActionFailedExecutions != 1 {
		t.Fatalf("expected failure counters to be aggregated, got %+v", diag)
	}
	if diag.LastFailureAt == nil || diag.LastSuccessAt == nil {
		t.Fatalf("expected last failure/success timestamps, got %+v", diag)
	}
	if len(diag.RecentFailures) != 3 {
		t.Fatalf("expected three recent failures, got %+v", diag.RecentFailures)
	}
	if diag.RecentFailures[0].Action != "ping" {
		t.Fatalf("expected most recent failure first, got %+v", diag.RecentFailures)
	}
	if len(diag.FailureGroups) != 2 {
		t.Fatalf("expected two failure groups, got %+v", diag.FailureGroups)
	}
	if diag.FailureGroups[0].Action != "hook.execute" || diag.FailureGroups[0].Hook != "order.create.after" || diag.FailureGroups[0].FailureCount != 2 {
		t.Fatalf("expected hook failure hotspot first, got %+v", diag.FailureGroups)
	}
}

func TestBuildPluginStorageDiagnosticsUsesRecentTaskMetadata(t *testing.T) {
	policy := service.EffectivePluginCapabilityPolicy{
		ExecuteActionStorage: map[string]string{
			"template.page.get":  "read",
			"template.page.save": "write",
			"template.echo":      "none",
		},
	}
	completedAt := time.Date(2026, time.March, 12, 10, 0, 0, 0, time.UTC)
	updatedAt := completedAt.Add(-30 * time.Second)
	overview := service.PluginExecutionTaskOverview{
		Recent: []service.PluginExecutionTaskSnapshot{
			{
				ID:          "pex_recent_1",
				Action:      "template.page.save",
				Status:      service.PluginExecutionStatusCompleted,
				UpdatedAt:   updatedAt,
				CompletedAt: &completedAt,
				Metadata: map[string]string{
					"storage_access_mode": "write",
				},
			},
			{
				ID:        "pex_recent_0",
				Action:    "template.echo",
				Status:    service.PluginExecutionStatusCompleted,
				UpdatedAt: updatedAt.Add(-time.Minute),
				Metadata: map[string]string{
					"storage_access_mode": "none",
				},
			},
		},
	}

	diag := buildPluginStorageDiagnostics(policy, nil, overview)
	if diag.ProfileCount != 3 {
		t.Fatalf("expected profile_count=3, got %+v", diag)
	}
	if len(diag.DeclaredProfiles) != 3 {
		t.Fatalf("expected three declared profiles, got %+v", diag.DeclaredProfiles)
	}
	if diag.LastObserved == nil {
		t.Fatalf("expected last observed storage access, got %+v", diag)
	}
	if diag.LastObserved.Action != "template.page.save" {
		t.Fatalf("expected latest action template.page.save, got %+v", diag.LastObserved)
	}
	if diag.LastObserved.DeclaredAccessMode != "write" || diag.LastObserved.ObservedAccessMode != "write" {
		t.Fatalf("expected declared/observed write, got %+v", diag.LastObserved)
	}
}

func TestBuildPluginStorageDiagnosticsUsesPersistedExecutionMetadata(t *testing.T) {
	policy := service.EffectivePluginCapabilityPolicy{
		ExecuteActionStorage: map[string]string{
			"template.page.get":  "read",
			"template.page.save": "write",
		},
	}
	completedAt := time.Date(2026, time.March, 12, 10, 30, 0, 0, time.UTC)
	executions := []models.PluginExecution{
		{
			ID:        42,
			Action:    "template.page.save",
			Success:   true,
			CreatedAt: completedAt,
			Metadata: models.JSONMap{
				service.PluginExecutionMetadataID:     "pex_persisted_1",
				service.PluginExecutionMetadataStatus: service.PluginExecutionStatusCompleted,
				service.PluginExecutionMetadataStream: "false",
				pluginDiagnosticStorageAccessMetaKey:  "write",
			},
		},
	}
	overview := service.PluginExecutionTaskOverview{
		Recent: []service.PluginExecutionTaskSnapshot{
			{
				ID:        "pex_recent_1",
				Action:    "template.page.get",
				Status:    service.PluginExecutionStatusCompleted,
				UpdatedAt: completedAt.Add(-time.Minute),
				Metadata: map[string]string{
					pluginDiagnosticStorageAccessMetaKey: "read",
				},
			},
		},
	}

	diag := buildPluginStorageDiagnostics(policy, executions, overview)
	if diag.LastObserved == nil {
		t.Fatalf("expected last observed storage access, got %+v", diag)
	}
	if diag.LastObserved.Source != pluginDiagnosticStorageObservationSourcePersisted {
		t.Fatalf("expected persisted observation source, got %+v", diag.LastObserved)
	}
	if diag.LastObserved.TaskID != "pex_persisted_1" {
		t.Fatalf("expected persisted task id, got %+v", diag.LastObserved)
	}
	if diag.LastObserved.Action != "template.page.save" {
		t.Fatalf("expected persisted action template.page.save, got %+v", diag.LastObserved)
	}
	if diag.LastObserved.DeclaredAccessMode != "write" || diag.LastObserved.ObservedAccessMode != "write" {
		t.Fatalf("expected declared/observed write, got %+v", diag.LastObserved)
	}
}

func TestBuildPluginDiagnosticIssuesIncludesMissingExecuteActionStorageWarning(t *testing.T) {
	plugin := models.Plugin{
		ID:       13,
		Name:     "missing-storage-profile-plugin",
		Runtime:  service.PluginRuntimeJSWorker,
		Manifest: `{"manifest_version":"1.0.0","protocol_version":"1.0.0"}`,
	}

	issues := buildPluginDiagnosticIssues(
		plugin,
		service.PluginRuntimeInspection{
			Valid:           true,
			ResolvedRuntime: service.PluginRuntimeJSWorker,
			ConnectionState: "stateless",
		},
		service.InspectPluginProtocolCompatibility(&plugin),
		service.PluginRegistrationInspection{},
		service.ResolveEffectivePluginCapabilityPolicy(&plugin),
		nil,
		pluginStorageDiagnostics{
			LastObserved: &pluginStorageObservationDiagnostic{
				Source:             "execution_tasks.recent",
				Action:             "template.page.get",
				DeclaredAccessMode: "unknown",
				ObservedAccessMode: "read",
			},
		},
		nil,
	)

	found := false
	for _, issue := range issues {
		if issue.Code != "execute_action_storage_missing_profile" {
			continue
		}
		found = true
		if issue.Severity != "warn" {
			t.Fatalf("expected warn severity, got %+v", issue)
		}
	}
	if !found {
		t.Fatalf("expected execute_action_storage_missing_profile issue, got %+v", issues)
	}
}

func openPluginDiagnosticTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:plugin-diagnostic-" + time.Now().UTC().Format("20060102150405.000000000") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&models.Plugin{}, &models.PluginExecution{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	return db
}
