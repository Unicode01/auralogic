package service

import (
	"testing"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/models"
)

func TestPluginCapabilityPolicyRuntimeAccessRequiresExplicitGrant(t *testing.T) {
	legacyPlugin := &models.Plugin{
		Name:         "legacy-runtime-plugin",
		Capabilities: `{"allow_network": true, "allow_file_system": true}`,
	}
	legacyPolicy := resolvePluginCapabilityPolicy(legacyPlugin)
	if legacyPolicy.AllowsRuntimeNetwork() {
		t.Fatalf("expected runtime network to require explicit grant for legacy plugin")
	}
	if legacyPolicy.AllowsRuntimeFileSystem() {
		t.Fatalf("expected runtime file system to require explicit grant for legacy plugin")
	}

	explicitGrantPlugin := &models.Plugin{
		Name: "granted-runtime-plugin",
		Capabilities: `{
			"allow_network": true,
			"allow_file_system": true,
			"granted_permissions": ["runtime.network", "runtime.file_system"]
		}`,
	}
	explicitGrantPolicy := resolvePluginCapabilityPolicy(explicitGrantPlugin)
	if !explicitGrantPolicy.AllowsRuntimeNetwork() {
		t.Fatalf("expected explicit runtime.network grant to enable network access")
	}
	if !explicitGrantPolicy.AllowsRuntimeFileSystem() {
		t.Fatalf("expected explicit runtime.file_system grant to enable file system access")
	}
}

func TestPluginCapabilityPolicyParsesExecuteActionStorageProfiles(t *testing.T) {
	plugin := &models.Plugin{
		Name: "action-storage-plugin",
		Capabilities: `{
			"requested_permissions": ["api.execute"],
			"granted_permissions": ["api.execute"],
			"execute_action_storage": {
				" template.echo ": "none",
				"template.page.get": "read",
				"template.page.save": "write",
				"invalid.mode": "bogus"
			}
		}`,
	}

	policy := resolvePluginCapabilityPolicy(plugin)
	if got := policy.ResolveExecuteActionStorageMode("template.echo"); got != pluginStorageAccessNone {
		t.Fatalf("expected template.echo storage mode none, got %q", got)
	}
	if got := policy.ResolveExecuteActionStorageMode("template.page.get"); got != pluginStorageAccessRead {
		t.Fatalf("expected template.page.get storage mode read, got %q", got)
	}
	if got := policy.ResolveExecuteActionStorageMode("template.page.save"); got != pluginStorageAccessWrite {
		t.Fatalf("expected template.page.save storage mode write, got %q", got)
	}
	if got := policy.ResolveExecuteActionStorageMode("invalid.mode"); got != pluginStorageAccessUnknown {
		t.Fatalf("expected invalid mode to normalize to unknown, got %q", got)
	}

	effective := ResolveEffectivePluginCapabilityPolicy(plugin)
	if got := effective.ExecuteActionStorage["template.page.save"]; got != pluginStorageAccessWrite {
		t.Fatalf("expected effective policy to expose write profile, got %+v", effective.ExecuteActionStorage)
	}
}

func TestDiagnosePluginHookParticipationReportsFrontendScopeDeny(t *testing.T) {
	plugin := &models.Plugin{
		Name: "frontend-scope-plugin",
		Capabilities: `{
			"hooks": ["frontend.bootstrap"],
			"requested_permissions": ["hook.execute", "frontend.extension"],
			"granted_permissions": ["hook.execute", "frontend.extension"],
			"frontend_min_scope": "authenticated",
			"frontend_allowed_areas": ["user"]
		}`,
	}

	diag := DiagnosePluginHookParticipation(
		plugin,
		"frontend.bootstrap",
		map[string]interface{}{
			"area": "user",
			"path": "/plugin-pages/sample",
			"slot": "bootstrap",
		},
		&ExecutionContext{
			Metadata: map[string]string{
				PluginScopeMetadataAuthenticated: "false",
				PluginScopeMetadataSuperAdmin:    "false",
			},
		},
	)

	if diag.Participates {
		t.Fatalf("expected frontend bootstrap to be denied for guest scope")
	}
	if diag.ReasonCode != "frontend_scope_requires_authenticated" {
		t.Fatalf("expected frontend scope deny reason, got %+v", diag)
	}
}

func TestDiagnosePluginHookParticipationKeepsFrontendExtensionFlag(t *testing.T) {
	plugin := &models.Plugin{
		Name: "frontend-flag-plugin",
		Capabilities: `{
			"hooks": ["frontend.bootstrap"],
			"requested_permissions": ["hook.execute"],
			"granted_permissions": ["hook.execute"],
			"allow_frontend_extensions": false,
			"frontend_allowed_areas": ["admin"]
		}`,
	}

	diag := DiagnosePluginHookParticipation(
		plugin,
		"frontend.bootstrap",
		map[string]interface{}{
			"area": "admin",
			"path": "/admin/plugin-pages/sample",
			"slot": "bootstrap",
		},
		&ExecutionContext{
			Metadata: map[string]string{
				PluginScopeMetadataAuthenticated: "true",
				PluginScopeMetadataSuperAdmin:    "true",
			},
		},
	)

	if !diag.Participates {
		t.Fatalf("expected hook to remain eligible even when frontend extension output is disabled")
	}
	if diag.AllowFrontendExtensions {
		t.Fatalf("expected frontend extension flag to remain disabled in diagnosis")
	}
}

func TestBuildPreparedHookPluginsCacheKeyStableAcrossPermissionMapOrder(t *testing.T) {
	scopeA := hookRequestAccessScope{
		Known:         true,
		Authenticated: true,
		SuperAdmin:    false,
		Permissions: map[string]struct{}{
			"orders.read":  {},
			"plugins.view": {},
		},
	}
	scopeB := hookRequestAccessScope{
		Known:         true,
		Authenticated: true,
		SuperAdmin:    false,
		Permissions: map[string]struct{}{
			"plugins.view": {},
			"orders.read":  {},
		},
	}

	keyA := buildPreparedHookPluginsCacheKey("frontend.slot.render", map[string]interface{}{
		"slot": "admin.orders.row_actions",
		"path": "/admin/orders",
	}, scopeA)
	keyB := buildPreparedHookPluginsCacheKey("frontend.slot.render", map[string]interface{}{
		"slot": "admin.orders.row_actions",
		"path": "/admin/orders/1001",
	}, scopeB)
	if keyA == "" {
		t.Fatalf("expected frontend hook cache key")
	}
	if keyA != keyB {
		t.Fatalf("expected cache key stable across permission order and unrelated path changes, got %q vs %q", keyA, keyB)
	}

	keyC := buildPreparedHookPluginsCacheKey("frontend.slot.render", map[string]interface{}{
		"slot": "admin.orders.actions",
	}, scopeA)
	if keyA == keyC {
		t.Fatalf("expected different slot to produce different cache key")
	}

	if key := buildPreparedHookPluginsCacheKey("order.create.before", nil, scopeA); key != "" {
		t.Fatalf("expected non-frontend hook to skip prepared cache, got %q", key)
	}
}

func TestExecutionRequestCacheCachesFrontendPreparedHookPlugins(t *testing.T) {
	cache := NewExecutionRequestCache()
	calls := 0
	cacheKey := "hook=frontend.slot.render|slot=user.orders.top|area=user|scope=known=true|auth=true|super=false|permissions=orders.read"

	first := cache.resolveFrontendPreparedHookPlugins(cacheKey, func() []preparedHookPlugin {
		calls++
		return []preparedHookPlugin{
			{
				Plugin: models.Plugin{
					ID:   1,
					Name: "original-plugin",
				},
				Runtime: PluginRuntimeGRPC,
			},
		}
	})
	if len(first) != 1 {
		t.Fatalf("expected cached prepared hook plugin, got %+v", first)
	}
	first[0].Plugin.Name = "mutated-plugin"

	second := cache.resolveFrontendPreparedHookPlugins(cacheKey, func() []preparedHookPlugin {
		calls++
		return nil
	})
	if calls != 1 {
		t.Fatalf("expected prepared hook resolver to run once, got %d", calls)
	}
	if len(second) != 1 || second[0].Plugin.Name != "original-plugin" {
		t.Fatalf("expected cached prepared hook plugin to remain immutable across callers, got %+v", second)
	}
}

func TestExecutionRequestCacheDeduplicatesConcurrentFrontendPreparedHookPlugins(t *testing.T) {
	cache := NewExecutionRequestCache()
	cacheKey := "hook=frontend.slot.render|slot=user.orders.top|area=user|scope=known=true|auth=true|super=false|permissions=orders.read"
	releaseResolver := make(chan struct{})
	resolverStarted := make(chan struct{})
	resolverCalls := make(chan struct{}, 2)
	done := make(chan []preparedHookPlugin, 2)

	resolver := func() []preparedHookPlugin {
		resolverCalls <- struct{}{}
		select {
		case <-resolverStarted:
		default:
			close(resolverStarted)
		}
		<-releaseResolver
		return []preparedHookPlugin{
			{
				Plugin: models.Plugin{
					ID:   1,
					Name: "singleflight-plugin",
				},
				Runtime: PluginRuntimeGRPC,
			},
		}
	}

	go func() {
		done <- cache.resolveFrontendPreparedHookPlugins(cacheKey, resolver)
	}()
	<-resolverStarted
	go func() {
		done <- cache.resolveFrontendPreparedHookPlugins(cacheKey, resolver)
	}()

	close(releaseResolver)
	for i := 0; i < 2; i++ {
		select {
		case prepared := <-done:
			if len(prepared) != 1 || prepared[0].Plugin.Name != "singleflight-plugin" {
				t.Fatalf("expected prepared plugin from shared inflight resolver, got %+v", prepared)
			}
		case <-time.After(time.Second):
			t.Fatalf("expected concurrent prepared hook plugin resolutions to complete")
		}
	}
	if len(resolverCalls) != 1 {
		t.Fatalf("expected concurrent prepared hook plugin resolution to invoke resolver once, got %d calls", len(resolverCalls))
	}
}

func TestPrepareHookPluginsRequestCacheKeepsBreakerDynamic(t *testing.T) {
	svc := NewPluginManagerService(nil, &config.Config{
		Plugin: config.PluginPlatformConfig{
			Enabled:         true,
			AllowedRuntimes: []string{PluginRuntimeGRPC, PluginRuntimeJSWorker},
			DefaultRuntime:  PluginRuntimeGRPC,
		},
	})

	plugin := models.Plugin{
		ID:      1,
		Name:    "dynamic-breaker-plugin",
		Runtime: PluginRuntimeGRPC,
		Capabilities: mustMarshalCapabilityMap(t, map[string]interface{}{
			"hooks": []string{"frontend.slot.render"},
			"requested_permissions": []string{
				PluginPermissionHookExecute,
				PluginPermissionFrontendExtension,
			},
			"granted_permissions": []string{
				PluginPermissionHookExecute,
				PluginPermissionFrontendExtension,
			},
			"allowed_frontend_slots": []string{"user.orders.top"},
		}),
	}
	entry := pluginExecutionCatalogEntry{
		Plugin:           plugin,
		Runtime:          PluginRuntimeGRPC,
		CapabilityPolicy: resolvePluginCapabilityPolicy(&plugin),
	}
	execCtx := &ExecutionContext{
		RequestCache: NewExecutionRequestCache(),
		Metadata: map[string]string{
			PluginScopeMetadataAuthenticated: "true",
		},
	}
	payload := map[string]interface{}{
		"slot": "user.orders.top",
		"path": "/orders",
	}
	scope := resolveHookRequestAccessScope(execCtx)

	first := svc.prepareHookPlugins([]pluginExecutionCatalogEntry{entry}, "frontend.slot.render", payload, scope, execCtx)
	if len(first) != 1 {
		t.Fatalf("expected plugin to be prepared before breaker opens, got %+v", first)
	}

	svc.breakerMu.Lock()
	svc.executionBreakers[plugin.ID] = &pluginExecutionBreakerRuntime{
		ConsecutiveFailures: normalizePluginFailureThreshold(svc.getHookExecutionPolicy()),
		OpenUntil:           time.Now().UTC().Add(time.Minute),
	}
	svc.breakerMu.Unlock()

	second := svc.prepareHookPlugins([]pluginExecutionCatalogEntry{entry}, "frontend.slot.render", payload, scope, execCtx)
	if len(second) != 0 {
		t.Fatalf("expected open breaker to remove cached plugin from final prepared list, got %+v", second)
	}
}

func TestInspectPluginRegistrationTracksRecordedOutcome(t *testing.T) {
	service := NewPluginManagerService(nil, nil)
	plugin := &models.Plugin{
		ID:      42,
		Name:    "tracked-plugin",
		Runtime: "grpc",
	}
	startedAt := time.Date(2026, time.March, 11, 8, 0, 0, 0, time.UTC)

	service.recordPluginRegistrationOutcome(plugin, "startup_load", PluginRuntimeGRPC, startedAt, nil)

	inspection := service.InspectPluginRegistration(plugin)
	if inspection.State != "success" {
		t.Fatalf("expected success registration state, got %+v", inspection)
	}
	if inspection.Trigger != "startup_load" {
		t.Fatalf("expected startup_load trigger, got %+v", inspection)
	}
	if inspection.Runtime != PluginRuntimeGRPC {
		t.Fatalf("expected grpc runtime, got %+v", inspection)
	}
	if inspection.AttemptedAt.IsZero() || inspection.CompletedAt.IsZero() {
		t.Fatalf("expected registration timestamps to be populated, got %+v", inspection)
	}
}

func TestInspectPluginRegistrationDefaultsToNeverAttempted(t *testing.T) {
	service := NewPluginManagerService(nil, nil)
	plugin := &models.Plugin{
		ID:      7,
		Name:    "idle-plugin",
		Runtime: "js_worker",
	}

	inspection := service.InspectPluginRegistration(plugin)
	if inspection.State != "never_attempted" {
		t.Fatalf("expected never_attempted state, got %+v", inspection)
	}
	if inspection.Runtime != "js_worker" {
		t.Fatalf("expected configured runtime fallback, got %+v", inspection)
	}
}

func TestInspectPluginManifestCompatibilityMetadataRejectsNewerProtocolVersion(t *testing.T) {
	inspection := InspectPluginManifestCompatibilityMetadata(
		PluginRuntimeJSWorker,
		PluginHostManifestVersion,
		"1.1.0",
		"",
		"",
		true,
	)

	if inspection.Compatible {
		t.Fatalf("expected newer plugin protocol version to be incompatible, got %+v", inspection)
	}
	if inspection.ReasonCode != "protocol_version_unsupported" {
		t.Fatalf("expected protocol_version_unsupported, got %+v", inspection)
	}
}

func TestInspectPluginManifestCompatibilityMetadataFlagsLegacyDefaults(t *testing.T) {
	inspection := InspectPluginManifestCompatibilityMetadata(
		PluginRuntimeGRPC,
		"",
		"",
		"",
		"",
		true,
	)

	if !inspection.Compatible {
		t.Fatalf("expected legacy defaults to remain compatible, got %+v", inspection)
	}
	if !inspection.LegacyDefaultsApplied {
		t.Fatalf("expected legacy defaults flag, got %+v", inspection)
	}
}

func TestValidatePluginProtocolCompatibilityRejectsInvalidManifestJSON(t *testing.T) {
	plugin := &models.Plugin{
		Name:     "invalid-manifest-plugin",
		Runtime:  PluginRuntimeGRPC,
		Manifest: "{invalid",
	}

	if err := ValidatePluginProtocolCompatibility(plugin); err == nil {
		t.Fatalf("expected invalid manifest json to be rejected")
	}
}

func TestResolvePluginExecutionBreakerStateStaysClosedBelowThreshold(t *testing.T) {
	now := time.Date(2026, time.March, 11, 10, 0, 0, 0, time.UTC)
	failureAt := now.Add(-5 * time.Second)
	policy := config.PluginExecutionPolicyConfig{
		FailureThreshold:  3,
		FailureCooldownMs: 30000,
	}

	state := resolvePluginExecutionBreakerState(&models.Plugin{
		Name:            "below-threshold-plugin",
		Status:          "unhealthy",
		LifecycleStatus: models.PluginLifecycleDegraded,
		FailCount:       1,
		UpdatedAt:       failureAt,
	}, buildPluginExecutionBreakerRuntimeFromPlugin(&models.Plugin{
		Name:            "below-threshold-plugin",
		Status:          "unhealthy",
		LifecycleStatus: models.PluginLifecycleDegraded,
		FailCount:       1,
		UpdatedAt:       failureAt,
	}, policy), policy, now)

	if state.State != pluginBreakerStateClosed {
		t.Fatalf("expected closed breaker below threshold, got %+v", state)
	}
}

func TestResolvePluginExecutionBreakerStateOpensDuringCooldown(t *testing.T) {
	now := time.Date(2026, time.March, 11, 10, 0, 0, 0, time.UTC)
	failureAt := now.Add(-5 * time.Second)
	policy := config.PluginExecutionPolicyConfig{
		FailureThreshold:  3,
		FailureCooldownMs: 30000,
	}
	plugin := &models.Plugin{
		Name:            "open-breaker-plugin",
		Status:          "unhealthy",
		LifecycleStatus: models.PluginLifecycleDegraded,
		FailCount:       3,
		UpdatedAt:       failureAt,
	}

	state := resolvePluginExecutionBreakerState(plugin, buildPluginExecutionBreakerRuntimeFromPlugin(plugin, policy), policy, now)

	if state.State != pluginBreakerStateOpen {
		t.Fatalf("expected open breaker during cooldown, got %+v", state)
	}
	if !state.CooldownActive || state.CooldownUntil == nil || !state.CooldownUntil.Equal(failureAt.Add(30*time.Second)) {
		t.Fatalf("expected cooldown metadata while breaker open, got %+v", state)
	}
}

func TestResolvePluginExecutionBreakerStateBecomesHalfOpenAfterCooldown(t *testing.T) {
	now := time.Date(2026, time.March, 11, 10, 0, 40, 0, time.UTC)
	failureAt := now.Add(-40 * time.Second)
	policy := config.PluginExecutionPolicyConfig{
		FailureThreshold:  3,
		FailureCooldownMs: 30000,
	}
	plugin := &models.Plugin{
		Name:            "half-open-plugin",
		Status:          "unhealthy",
		LifecycleStatus: models.PluginLifecycleDegraded,
		FailCount:       3,
		UpdatedAt:       failureAt,
	}

	state := resolvePluginExecutionBreakerState(plugin, buildPluginExecutionBreakerRuntimeFromPlugin(plugin, policy), policy, now)

	if state.State != pluginBreakerStateHalfOpen {
		t.Fatalf("expected half-open breaker after cooldown, got %+v", state)
	}
	if state.CooldownActive {
		t.Fatalf("expected cooldown to be inactive in half-open state, got %+v", state)
	}
}

func TestRefreshPluginExecutionCatalogBuildsSortedHookIndex(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)

	plugins := []models.Plugin{
		{
			Name:            "wildcard-hook-plugin",
			Type:            "custom",
			Runtime:         PluginRuntimeGRPC,
			Address:         "127.0.0.1:9101",
			Enabled:         true,
			LifecycleStatus: models.PluginLifecycleInstalled,
			Capabilities:    mustMarshalCapabilities(t, PluginPermissionExecuteAPI, PluginPermissionHookExecute),
		},
		{
			Name:            "specific-hook-plugin",
			Type:            "custom",
			Runtime:         PluginRuntimeGRPC,
			Address:         "127.0.0.1:9102",
			Enabled:         true,
			LifecycleStatus: models.PluginLifecycleInstalled,
			Capabilities: mustMarshalCapabilityMap(t, map[string]interface{}{
				"hooks":                 []string{"order.create.before"},
				"requested_permissions": []string{PluginPermissionExecuteAPI, PluginPermissionHookExecute},
				"granted_permissions":   []string{PluginPermissionExecuteAPI, PluginPermissionHookExecute},
			}),
		},
		{
			Name:            "js-worker-plugin",
			Type:            "custom",
			Runtime:         PluginRuntimeJSWorker,
			Address:         "plugins/index.js",
			PackagePath:     "packages/js-worker-plugin.zip",
			Enabled:         true,
			LifecycleStatus: models.PluginLifecycleInstalled,
			Capabilities:    mustMarshalCapabilities(t, PluginPermissionExecuteAPI),
		},
		{
			Name:            "invalid-manifest-plugin",
			Type:            "custom",
			Runtime:         PluginRuntimeGRPC,
			Address:         "127.0.0.1:9103",
			Enabled:         true,
			LifecycleStatus: models.PluginLifecycleInstalled,
			Manifest:        "{invalid",
			Capabilities:    mustMarshalCapabilities(t, PluginPermissionExecuteAPI, PluginPermissionHookExecute),
		},
		{
			Name:            "disabled-plugin",
			Type:            "custom",
			Runtime:         PluginRuntimeGRPC,
			Address:         "127.0.0.1:9104",
			Enabled:         false,
			LifecycleStatus: models.PluginLifecyclePaused,
			Capabilities:    mustMarshalCapabilities(t, PluginPermissionExecuteAPI, PluginPermissionHookExecute),
		},
	}
	for i := range plugins {
		if err := db.Create(&plugins[i]).Error; err != nil {
			t.Fatalf("create plugin %s failed: %v", plugins[i].Name, err)
		}
	}
	if err := db.Model(&models.Plugin{}).
		Where("id = ?", plugins[4].ID).
		Updates(map[string]interface{}{
			"enabled":          false,
			"lifecycle_status": models.PluginLifecyclePaused,
		}).Error; err != nil {
		t.Fatalf("disable plugin failed: %v", err)
	}

	svc := NewPluginManagerService(db, &config.Config{
		Plugin: config.PluginPlatformConfig{
			Enabled:         true,
			AllowedRuntimes: []string{PluginRuntimeGRPC, PluginRuntimeJSWorker},
			DefaultRuntime:  PluginRuntimeGRPC,
		},
	})

	if err := svc.RefreshPluginExecutionCatalog(); err != nil {
		t.Fatalf("RefreshPluginExecutionCatalog failed: %v", err)
	}

	wildcardEntry, exists := svc.getPluginExecutionCatalogEntry(plugins[0].ID)
	if !exists {
		t.Fatalf("expected wildcard plugin to be cached")
	}
	if wildcardEntry.Runtime != PluginRuntimeGRPC {
		t.Fatalf("expected wildcard runtime grpc, got %+v", wildcardEntry)
	}
	if wildcardEntry.ValidationError != "" {
		t.Fatalf("expected wildcard plugin validation to pass, got %+v", wildcardEntry)
	}

	invalidEntry, exists := svc.getPluginExecutionCatalogEntry(plugins[3].ID)
	if !exists {
		t.Fatalf("expected invalid manifest plugin to remain cached")
	}
	if invalidEntry.ValidationError == "" {
		t.Fatalf("expected invalid manifest validation error, got %+v", invalidEntry)
	}

	if _, exists := svc.getPluginExecutionCatalogEntry(plugins[4].ID); exists {
		t.Fatalf("expected disabled plugin to be excluded from execution catalog")
	}

	hookEntries := svc.listHookExecutionCatalogEntries("order.create.before")
	if len(hookEntries) != 2 {
		t.Fatalf("expected 2 hook candidates, got %+v", hookEntries)
	}
	if hookEntries[0].Plugin.ID != plugins[0].ID || hookEntries[1].Plugin.ID != plugins[1].ID {
		t.Fatalf(
			"expected hook candidates in id order [%d %d], got [%d %d]",
			plugins[0].ID,
			plugins[1].ID,
			hookEntries[0].Plugin.ID,
			hookEntries[1].Plugin.ID,
		)
	}

	jsPlugins := svc.listJSWorkerCatalogPlugins()
	if len(jsPlugins) != 1 || jsPlugins[0].ID != plugins[2].ID {
		t.Fatalf("expected single js worker catalog plugin, got %+v", jsPlugins)
	}
}

func TestPluginExecutionIndexesCreatedByAutoMigrate(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)

	indexes := []string{
		"idx_plugin_executions_created_at",
		"idx_plugin_executions_plugin_created_at",
		"idx_plugin_executions_plugin_success_created_at",
	}
	for _, indexName := range indexes {
		if !db.Migrator().HasIndex(&models.PluginExecution{}, indexName) {
			t.Fatalf("expected plugin execution index %s to exist", indexName)
		}
	}
}

func TestCleanupExpiredPluginExecutionsBeforeRemovesOnlyExpiredRows(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)

	plugin := models.Plugin{
		Name:            "retention-plugin",
		Type:            "custom",
		Runtime:         PluginRuntimeGRPC,
		Address:         "127.0.0.1:9105",
		Enabled:         true,
		LifecycleStatus: models.PluginLifecycleInstalled,
	}
	if err := db.Create(&plugin).Error; err != nil {
		t.Fatalf("create plugin failed: %v", err)
	}

	now := time.Date(2026, time.March, 11, 12, 0, 0, 0, time.UTC)
	executions := []models.PluginExecution{
		{
			PluginID:  plugin.ID,
			Action:    "hook.execute",
			Success:   false,
			Error:     "expired-1",
			CreatedAt: now.Add(-95 * 24 * time.Hour),
		},
		{
			PluginID:  plugin.ID,
			Action:    "hook.execute",
			Success:   true,
			Result:    `{"ok":true}`,
			CreatedAt: now.Add(-91 * 24 * time.Hour),
		},
		{
			PluginID:  plugin.ID,
			Action:    "hook.execute",
			Success:   true,
			Result:    `{"ok":"recent"}`,
			CreatedAt: now.Add(-7 * 24 * time.Hour),
		},
	}
	for i := range executions {
		if err := db.Create(&executions[i]).Error; err != nil {
			t.Fatalf("create execution %d failed: %v", i, err)
		}
	}

	svc := NewPluginManagerService(db, &config.Config{
		Plugin: config.PluginPlatformConfig{
			Enabled: true,
			Execution: config.PluginExecutionPolicyConfig{
				ExecutionLogRetentionDays: 90,
			},
		},
	})

	deleted, err := svc.cleanupExpiredPluginExecutionsBefore(now.Add(-90*24*time.Hour), 1)
	if err != nil {
		t.Fatalf("cleanupExpiredPluginExecutionsBefore failed: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("expected 2 expired executions to be deleted, got %d", deleted)
	}

	var remaining []models.PluginExecution
	if err := db.Order("created_at ASC").Find(&remaining).Error; err != nil {
		t.Fatalf("query remaining executions failed: %v", err)
	}
	if len(remaining) != 1 {
		t.Fatalf("expected 1 recent execution to remain, got %d", len(remaining))
	}
	if remaining[0].CreatedAt.UTC() != executions[2].CreatedAt.UTC() {
		t.Fatalf("expected recent execution to remain, got %+v", remaining[0])
	}
}
