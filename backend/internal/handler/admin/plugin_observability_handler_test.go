package admin

import (
	"testing"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/models"
	"auralogic/internal/service"
)

func TestBuildPluginObservabilityExecutionWindowAggregatesGlobalStats(t *testing.T) {
	db := openPluginDiagnosticTestDB(t)
	handler := &PluginHandler{db: db}

	plugins := []models.Plugin{
		{
			Name:        "alpha-plugin",
			DisplayName: "Alpha Plugin",
			Type:        "custom",
			Runtime:     service.PluginRuntimeGRPC,
			Address:     "127.0.0.1:50051",
			Enabled:     true,
		},
		{
			Name:        "beta-plugin",
			DisplayName: "Beta Plugin",
			Type:        "custom",
			Runtime:     service.PluginRuntimeGRPC,
			Address:     "127.0.0.1:50052",
			Enabled:     true,
		},
	}
	if err := db.Create(&plugins).Error; err != nil {
		t.Fatalf("create plugins failed: %v", err)
	}

	now := time.Date(2026, time.March, 11, 12, 34, 0, 0, time.UTC)
	executions := []models.PluginExecution{
		{
			PluginID: plugins[0].ID,
			Action:   "hook.execute",
			Hook:     "order.create.after",
			Params:   `{"hook":"order.create.after"}`,
			Success:  false,
			Error:    "alpha hook failed 1",
			ErrorSignature: service.NormalizePluginExecutionErrorSignature(
				service.NormalizePluginExecutionErrorText("alpha hook failed 1"),
			),
			Duration:  40,
			CreatedAt: now.Add(-10 * time.Minute),
		},
		{
			PluginID: plugins[0].ID,
			Action:   "hook.execute",
			Hook:     "order.create.after",
			Params:   `{"hook":"order.create.after"}`,
			Success:  false,
			Error:    "alpha hook failed 2",
			ErrorSignature: service.NormalizePluginExecutionErrorSignature(
				service.NormalizePluginExecutionErrorText("alpha hook failed 2"),
			),
			Duration:  45,
			CreatedAt: now.Add(-8 * time.Minute),
		},
		{
			PluginID:  plugins[0].ID,
			Action:    "ping",
			Success:   true,
			Result:    `{"ok":true}`,
			Duration:  12,
			CreatedAt: now.Add(-20 * time.Minute),
		},
		{
			PluginID: plugins[1].ID,
			Action:   "sync",
			Success:  false,
			Error:    "beta manual failure",
			ErrorSignature: service.NormalizePluginExecutionErrorSignature(
				service.NormalizePluginExecutionErrorText("beta manual failure"),
			),
			Duration:  33,
			CreatedAt: now.Add(-5 * time.Minute),
		},
		{
			PluginID: plugins[1].ID,
			Action:   "hook.execute",
			Hook:     "ticket.reply.before",
			Params:   `{"hook":"ticket.reply.before"}`,
			Success:  false,
			Error:    "beta hook failure",
			ErrorSignature: service.NormalizePluginExecutionErrorSignature(
				service.NormalizePluginExecutionErrorText("beta hook failure"),
			),
			Duration:  28,
			CreatedAt: now.Add(-70 * time.Minute),
		},
		{
			PluginID:  plugins[1].ID,
			Action:    "sync",
			Success:   true,
			Result:    `{"ok":true}`,
			Duration:  15,
			CreatedAt: now.Add(-2 * time.Minute),
		},
		{
			PluginID:  plugins[1].ID,
			Action:    "sync",
			Success:   true,
			Result:    `{"ignored":true}`,
			Duration:  15,
			CreatedAt: now.Add(-26 * time.Hour),
		},
	}
	if err := db.Create(&executions).Error; err != nil {
		t.Fatalf("create executions failed: %v", err)
	}

	result := handler.buildPluginObservabilityExecutionWindow(plugins, now, pluginObservabilityQueryOptions{
		WindowHours: pluginObservabilityExecutionWindowDefaultHours,
	})
	snapshot := result.Snapshot

	if snapshot.TotalExecutions != 6 {
		t.Fatalf("expected total executions=6, got %+v", snapshot)
	}
	if snapshot.FailedExecutions != 4 || snapshot.HookFailedExecutions != 3 || snapshot.ActionFailedExecutions != 1 {
		t.Fatalf("expected failed execution counters to be aggregated, got %+v", snapshot)
	}
	if snapshot.LastFailureAt == nil || !snapshot.LastFailureAt.Equal(now.Add(-5*time.Minute)) {
		t.Fatalf("expected last failure timestamp to point to latest failed execution, got %+v", snapshot.LastFailureAt)
	}
	if snapshot.LastSuccessAt == nil || !snapshot.LastSuccessAt.Equal(now.Add(-2*time.Minute)) {
		t.Fatalf("expected last success timestamp to point to latest succeeded execution, got %+v", snapshot.LastSuccessAt)
	}
	if len(snapshot.RecentFailures) != 4 {
		t.Fatalf("expected four recent failures, got %+v", snapshot.RecentFailures)
	}
	if snapshot.RecentFailures[0].PluginID != plugins[1].ID || snapshot.RecentFailures[0].Action != "sync" {
		t.Fatalf("expected most recent failure first, got %+v", snapshot.RecentFailures[0])
	}
	if len(snapshot.FailureGroups) != 3 {
		t.Fatalf("expected three failure groups, got %+v", snapshot.FailureGroups)
	}
	if snapshot.FailureGroups[0].PluginID != plugins[0].ID || snapshot.FailureGroups[0].Hook != "order.create.after" || snapshot.FailureGroups[0].FailureCount != 2 {
		t.Fatalf("expected alpha hook hotspot to be grouped first, got %+v", snapshot.FailureGroups)
	}
	if len(snapshot.HookGroups) != 2 {
		t.Fatalf("expected two hook groups, got %+v", snapshot.HookGroups)
	}
	if snapshot.HookGroups[0].PluginID != plugins[0].ID || snapshot.HookGroups[0].Hook != "order.create.after" || snapshot.HookGroups[0].FailureCount != 2 {
		t.Fatalf("expected alpha hook drill-down group first, got %+v", snapshot.HookGroups)
	}
	if len(snapshot.ErrorSignatures) != 3 {
		t.Fatalf("expected three error signatures, got %+v", snapshot.ErrorSignatures)
	}
	if snapshot.ErrorSignatures[0].PluginID != plugins[0].ID || snapshot.ErrorSignatures[0].Signature != "alpha hook failed #" || snapshot.ErrorSignatures[0].FailureCount != 2 {
		t.Fatalf("expected normalized alpha error signature to be grouped first, got %+v", snapshot.ErrorSignatures)
	}

	alphaCounters := result.perPlugin[plugins[0].ID]
	if alphaCounters.TotalExecutions != 3 || alphaCounters.FailedExecutions != 2 {
		t.Fatalf("expected alpha per-plugin counters to be aggregated, got %+v", alphaCounters)
	}
	betaCounters := result.perPlugin[plugins[1].ID]
	if betaCounters.TotalExecutions != 3 || betaCounters.FailedExecutions != 2 {
		t.Fatalf("expected beta per-plugin counters to be aggregated, got %+v", betaCounters)
	}

	if len(snapshot.ByHour) != pluginObservabilityExecutionWindowDefaultHours {
		t.Fatalf("expected %d hourly buckets, got %d", pluginObservabilityExecutionWindowDefaultHours, len(snapshot.ByHour))
	}
	lastBucket := snapshot.ByHour[len(snapshot.ByHour)-1]
	if !lastBucket.HourStart.Equal(time.Date(2026, time.March, 11, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected latest bucket to match current hour, got %+v", lastBucket)
	}
	if lastBucket.TotalExecutions != 5 || lastBucket.FailedExecutions != 3 {
		t.Fatalf("expected latest bucket to include current-hour traffic, got %+v", lastBucket)
	}
	previousBucket := snapshot.ByHour[len(snapshot.ByHour)-2]
	if !previousBucket.HourStart.Equal(time.Date(2026, time.March, 11, 11, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected previous bucket to match previous hour, got %+v", previousBucket)
	}
	if previousBucket.TotalExecutions != 1 || previousBucket.FailedExecutions != 1 {
		t.Fatalf("expected previous bucket to include prior-hour failed execution, got %+v", previousBucket)
	}
}

func TestBuildPluginObservabilityBreakerOverviewUsesRuntimeStateAndWindowCounters(t *testing.T) {
	now := time.Now().UTC()
	handler := &PluginHandler{
		pluginManager: service.NewPluginManagerService(nil, &config.Config{}),
	}
	plugins := []models.Plugin{
		{
			ID:              1,
			Name:            "open-plugin",
			Runtime:         service.PluginRuntimeGRPC,
			Enabled:         true,
			Status:          "unhealthy",
			LifecycleStatus: models.PluginLifecycleDegraded,
			FailCount:       4,
			UpdatedAt:       now.Add(-10 * time.Second),
		},
		{
			ID:              2,
			Name:            "half-open-plugin",
			Runtime:         service.PluginRuntimeGRPC,
			Enabled:         true,
			Status:          "unhealthy",
			LifecycleStatus: models.PluginLifecycleDegraded,
			FailCount:       4,
			UpdatedAt:       now.Add(-2 * time.Minute),
		},
		{
			ID:              3,
			Name:            "closed-plugin",
			Runtime:         service.PluginRuntimeGRPC,
			Enabled:         true,
			Status:          "healthy",
			LifecycleStatus: models.PluginLifecycleRunning,
			FailCount:       1,
			UpdatedAt:       now.Add(-5 * time.Minute),
		},
		{
			ID:              4,
			Name:            "disabled-plugin",
			Runtime:         service.PluginRuntimeGRPC,
			Enabled:         false,
			Status:          "healthy",
			LifecycleStatus: models.PluginLifecyclePaused,
			FailCount:       0,
			UpdatedAt:       now.Add(-5 * time.Minute),
		},
	}
	window := pluginObservabilityExecutionWindowBuildResult{
		Snapshot: pluginObservabilityExecutionWindowStats{
			WindowHours: pluginObservabilityExecutionWindowDefaultHours,
		},
		perPlugin: map[uint]pluginObservabilityPluginWindowCounters{
			1: {TotalExecutions: 5, FailedExecutions: 4},
			2: {TotalExecutions: 3, FailedExecutions: 3},
			3: {TotalExecutions: 9, FailedExecutions: 1},
		},
	}

	overview := handler.buildPluginObservabilityBreakerOverview(plugins, window, pluginObservabilityQueryOptions{
		WindowHours: pluginObservabilityExecutionWindowDefaultHours,
	})
	if overview.TotalPlugins != 4 || overview.EnabledPlugins != 3 {
		t.Fatalf("expected plugin totals to be counted, got %+v", overview)
	}
	if overview.OpenCount != 1 || overview.HalfOpenCount != 1 || overview.ClosedCount != 2 {
		t.Fatalf("expected breaker states to be classified, got %+v", overview)
	}
	if len(overview.Rows) != 4 {
		t.Fatalf("expected four breaker rows, got %+v", overview.Rows)
	}
	if overview.Rows[0].PluginID != 1 || overview.Rows[0].BreakerState != pluginObservabilityBreakerStateOpen {
		t.Fatalf("expected open plugin to sort first, got %+v", overview.Rows)
	}
	if overview.Rows[0].WindowFailedExecutions != 4 || !overview.Rows[0].CooldownActive || overview.Rows[0].CooldownUntil == nil {
		t.Fatalf("expected open plugin row to include cooldown and window counters, got %+v", overview.Rows[0])
	}
	if overview.Rows[1].PluginID != 2 || overview.Rows[1].BreakerState != pluginObservabilityBreakerStateHalfOpen {
		t.Fatalf("expected half-open plugin to sort after open plugin, got %+v", overview.Rows)
	}
	if overview.Rows[2].PluginID != 3 || overview.Rows[2].BreakerState != pluginObservabilityBreakerStateClosed {
		t.Fatalf("expected closed enabled plugin to sort before disabled plugin, got %+v", overview.Rows)
	}
	if overview.Rows[3].PluginID != 4 || overview.Rows[3].Enabled {
		t.Fatalf("expected disabled plugin to sort last, got %+v", overview.Rows)
	}
	if overview.Rows[2].FailureThreshold != 3 {
		t.Fatalf("expected default failure threshold to be surfaced, got %+v", overview.Rows[2])
	}
}

func TestBuildPluginObservabilityExecutionWindowSupportsPluginAndWindowFilter(t *testing.T) {
	db := openPluginDiagnosticTestDB(t)
	handler := &PluginHandler{db: db}

	plugins := []models.Plugin{
		{
			Name:        "alpha-plugin",
			DisplayName: "Alpha Plugin",
			Type:        "custom",
			Runtime:     service.PluginRuntimeGRPC,
			Address:     "127.0.0.1:50051",
			Enabled:     true,
		},
		{
			Name:        "beta-plugin",
			DisplayName: "Beta Plugin",
			Type:        "custom",
			Runtime:     service.PluginRuntimeGRPC,
			Address:     "127.0.0.1:50052",
			Enabled:     true,
		},
	}
	if err := db.Create(&plugins).Error; err != nil {
		t.Fatalf("create plugins failed: %v", err)
	}

	now := time.Date(2026, time.March, 11, 12, 34, 0, 0, time.UTC)
	executions := []models.PluginExecution{
		{
			PluginID: plugins[0].ID,
			Action:   "hook.execute",
			Hook:     "order.create.after",
			Params:   `{"hook":"order.create.after"}`,
			Success:  false,
			Error:    "alpha hook failed",
			ErrorSignature: service.NormalizePluginExecutionErrorSignature(
				service.NormalizePluginExecutionErrorText("alpha hook failed"),
			),
			Duration:  40,
			CreatedAt: now.Add(-25 * time.Hour),
		},
		{
			PluginID:  plugins[0].ID,
			Action:    "ping",
			Success:   true,
			Result:    `{"ok":true}`,
			Duration:  12,
			CreatedAt: now.Add(-2 * time.Hour),
		},
		{
			PluginID: plugins[1].ID,
			Action:   "sync",
			Success:  false,
			Error:    "beta recent failure",
			ErrorSignature: service.NormalizePluginExecutionErrorSignature(
				service.NormalizePluginExecutionErrorText("beta recent failure"),
			),
			Duration:  33,
			CreatedAt: now.Add(-30 * time.Minute),
		},
	}
	if err := db.Create(&executions).Error; err != nil {
		t.Fatalf("create executions failed: %v", err)
	}

	result := handler.buildPluginObservabilityExecutionWindow(plugins, now, pluginObservabilityQueryOptions{
		PluginID:    plugins[0].ID,
		WindowHours: pluginObservabilityExecutionWindowMaxHours,
	})
	snapshot := result.Snapshot

	if snapshot.WindowHours != pluginObservabilityExecutionWindowMaxHours {
		t.Fatalf("expected 7d window hours to be echoed, got %+v", snapshot)
	}
	if snapshot.TotalExecutions != 2 || snapshot.FailedExecutions != 1 || snapshot.HookFailedExecutions != 1 {
		t.Fatalf("expected selected plugin executions inside 7d window only, got %+v", snapshot)
	}
	if snapshot.ActionFailedExecutions != 0 {
		t.Fatalf("expected no non-hook failures for selected plugin, got %+v", snapshot)
	}
	if len(snapshot.RecentFailures) != 1 || snapshot.RecentFailures[0].PluginID != plugins[0].ID {
		t.Fatalf("expected only selected plugin recent failures, got %+v", snapshot.RecentFailures)
	}
	if len(snapshot.HookGroups) != 1 || snapshot.HookGroups[0].Hook != "order.create.after" {
		t.Fatalf("expected selected plugin hook group only, got %+v", snapshot.HookGroups)
	}
	if len(snapshot.ErrorSignatures) != 1 || snapshot.ErrorSignatures[0].Signature != "alpha hook failed" {
		t.Fatalf("expected selected plugin error signature only, got %+v", snapshot.ErrorSignatures)
	}
	if len(snapshot.ByHour) != pluginObservabilityExecutionWindowMaxHours {
		t.Fatalf("expected 7d bucket count=%d, got %d", pluginObservabilityExecutionWindowMaxHours, len(snapshot.ByHour))
	}
	if _, exists := result.perPlugin[plugins[1].ID]; exists {
		t.Fatalf("expected other plugin counters to be excluded, got %+v", result.perPlugin)
	}
}
