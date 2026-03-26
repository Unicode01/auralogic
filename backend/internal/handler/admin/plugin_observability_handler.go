package admin

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"auralogic/internal/models"
	"auralogic/internal/pluginobs"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	pluginObservabilityExecutionWindowDefaultHours = 24
	pluginObservabilityExecutionWindowMaxHours     = 168
	pluginObservabilityRecentFailuresLimit         = 10
	pluginObservabilityFailureGroupsLimit          = 10
	pluginObservabilityHookGroupsLimit             = 10
	pluginObservabilityErrorSignaturesLimit        = 10
	pluginObservabilityBreakerStateClosed          = "closed"
	pluginObservabilityBreakerStateOpen            = "open"
	pluginObservabilityBreakerStateHalfOpen        = "half_open"
	pluginObservabilityBreakerStateUnknown         = "unknown"
	pluginObservabilityDefaultBucketTimeLayout     = "2006-01-02T15:04:05Z"
)

type pluginObservabilityQueryOptions struct {
	PluginID    uint
	WindowHours int
}

type pluginObservabilitySnapshot struct {
	pluginobs.Snapshot
	BreakerOverview pluginObservabilityBreakerOverview      `json:"breaker_overview"`
	ExecutionWindow pluginObservabilityExecutionWindowStats `json:"execution_window"`
}

type pluginObservabilityBreakerOverview struct {
	WindowHours         int                             `json:"window_hours"`
	TotalPlugins        int                             `json:"total_plugins"`
	EnabledPlugins      int                             `json:"enabled_plugins"`
	OpenCount           int                             `json:"open_count"`
	HalfOpenCount       int                             `json:"half_open_count"`
	ClosedCount         int                             `json:"closed_count"`
	CooldownActiveCount int                             `json:"cooldown_active_count"`
	ProbeInFlightCount  int                             `json:"probe_in_flight_count"`
	Rows                []pluginObservabilityBreakerRow `json:"rows"`
}

type pluginObservabilityBreakerRow struct {
	PluginID               uint       `json:"plugin_id"`
	PluginName             string     `json:"plugin_name"`
	Runtime                string     `json:"runtime"`
	Enabled                bool       `json:"enabled"`
	LifecycleStatus        string     `json:"lifecycle_status,omitempty"`
	HealthStatus           string     `json:"health_status,omitempty"`
	BreakerState           string     `json:"breaker_state"`
	FailureCount           int        `json:"failure_count"`
	FailureThreshold       int        `json:"failure_threshold"`
	CooldownActive         bool       `json:"cooldown_active"`
	CooldownUntil          *time.Time `json:"cooldown_until,omitempty"`
	CooldownReason         string     `json:"cooldown_reason,omitempty"`
	ProbeInFlight          bool       `json:"probe_in_flight"`
	ProbeStartedAt         *time.Time `json:"probe_started_at,omitempty"`
	WindowTotalExecutions  int        `json:"window_total_executions"`
	WindowFailedExecutions int        `json:"window_failed_executions"`
}

type pluginObservabilityExecutionWindowStats struct {
	WindowHours            int                                          `json:"window_hours"`
	TotalExecutions        int                                          `json:"total_executions"`
	FailedExecutions       int                                          `json:"failed_executions"`
	HookFailedExecutions   int                                          `json:"hook_failed_executions"`
	ActionFailedExecutions int                                          `json:"action_failed_executions"`
	LastFailureAt          *time.Time                                   `json:"last_failure_at,omitempty"`
	LastSuccessAt          *time.Time                                   `json:"last_success_at,omitempty"`
	ByHour                 []pluginObservabilityExecutionHourBucket     `json:"by_hour"`
	FailureGroups          []pluginObservabilityExecutionFailureGroup   `json:"failure_groups"`
	HookGroups             []pluginObservabilityExecutionHookGroup      `json:"hook_groups"`
	ErrorSignatures        []pluginObservabilityExecutionErrorSignature `json:"error_signatures"`
	RecentFailures         []pluginObservabilityExecutionFailureSample  `json:"recent_failures"`
}

type pluginObservabilityExecutionHourBucket struct {
	HourStart        time.Time `json:"hour_start"`
	TotalExecutions  int       `json:"total_executions"`
	FailedExecutions int       `json:"failed_executions"`
}

type pluginObservabilityExecutionFailureGroup struct {
	PluginID      uint       `json:"plugin_id"`
	PluginName    string     `json:"plugin_name"`
	Action        string     `json:"action"`
	Hook          string     `json:"hook,omitempty"`
	FailureCount  int        `json:"failure_count"`
	LastFailureAt *time.Time `json:"last_failure_at,omitempty"`
}

type pluginObservabilityExecutionFailureSample struct {
	ID         uint      `json:"id"`
	PluginID   uint      `json:"plugin_id"`
	PluginName string    `json:"plugin_name"`
	Action     string    `json:"action"`
	Hook       string    `json:"hook,omitempty"`
	Error      string    `json:"error,omitempty"`
	Duration   int       `json:"duration"`
	CreatedAt  time.Time `json:"created_at,omitempty"`
}

type pluginObservabilityExecutionHookGroup struct {
	PluginID      uint       `json:"plugin_id"`
	PluginName    string     `json:"plugin_name"`
	Hook          string     `json:"hook"`
	FailureCount  int        `json:"failure_count"`
	LastFailureAt *time.Time `json:"last_failure_at,omitempty"`
	LastError     string     `json:"last_error,omitempty"`
}

type pluginObservabilityExecutionErrorSignature struct {
	PluginID      uint       `json:"plugin_id"`
	PluginName    string     `json:"plugin_name"`
	Signature     string     `json:"signature"`
	FailureCount  int        `json:"failure_count"`
	LastFailureAt *time.Time `json:"last_failure_at,omitempty"`
	SampleError   string     `json:"sample_error,omitempty"`
}

type pluginObservabilityExecutionWindowBuildResult struct {
	Snapshot  pluginObservabilityExecutionWindowStats
	perPlugin map[uint]pluginObservabilityPluginWindowCounters
}

type pluginObservabilityPluginWindowCounters struct {
	TotalExecutions  int
	FailedExecutions int
}

// GetPluginObservability 获取插件观测指标快照（管理员接口）
func (h *PluginHandler) GetPluginObservability(c *gin.Context) {
	options, ok := parsePluginObservabilityQueryOptions(c)
	if !ok {
		return
	}

	baseSnapshot := pluginobs.SnapshotNow()
	now := baseSnapshot.GeneratedAt.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
		baseSnapshot.GeneratedAt = now
	}

	plugins := h.listPluginObservabilityPlugins(options.PluginID)
	if options.PluginID > 0 && len(plugins) == 0 {
		h.respondPluginError(c, http.StatusNotFound, "Plugin not found")
		return
	}

	executionWindow := h.buildPluginObservabilityExecutionWindow(plugins, now, options)
	resp := pluginObservabilitySnapshot{
		Snapshot:        baseSnapshot,
		BreakerOverview: h.buildPluginObservabilityBreakerOverview(plugins, executionWindow, options),
		ExecutionWindow: executionWindow.Snapshot,
	}
	c.JSON(http.StatusOK, resp)
}

func parsePluginObservabilityQueryOptions(c *gin.Context) (pluginObservabilityQueryOptions, bool) {
	options := pluginObservabilityQueryOptions{
		WindowHours: pluginObservabilityExecutionWindowDefaultHours,
	}
	if c == nil {
		return options, true
	}

	rawPluginID := strings.TrimSpace(c.Query("plugin_id"))
	if rawPluginID != "" {
		parsed, err := strconv.ParseUint(rawPluginID, 10, 64)
		if err != nil || parsed == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid plugin_id"})
			return pluginObservabilityQueryOptions{}, false
		}
		options.PluginID = uint(parsed)
	}

	rawHours := strings.TrimSpace(c.Query("hours"))
	if rawHours == "" {
		return options, true
	}

	parsedHours, err := strconv.Atoi(rawHours)
	if err != nil || (parsedHours != pluginObservabilityExecutionWindowDefaultHours && parsedHours != pluginObservabilityExecutionWindowMaxHours) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid hours"})
		return pluginObservabilityQueryOptions{}, false
	}
	options.WindowHours = parsedHours
	return options, true
}

func (h *PluginHandler) listPluginObservabilityPlugins(pluginID uint) []models.Plugin {
	if h == nil || h.db == nil {
		return []models.Plugin{}
	}

	var plugins []models.Plugin
	query := h.db.Order("id ASC")
	if pluginID > 0 {
		query = query.Where("id = ?", pluginID)
	}
	if err := query.Find(&plugins).Error; err != nil {
		return []models.Plugin{}
	}
	return plugins
}

func (h *PluginHandler) buildPluginObservabilityBreakerOverview(
	plugins []models.Plugin,
	executionWindow pluginObservabilityExecutionWindowBuildResult,
	options pluginObservabilityQueryOptions,
) pluginObservabilityBreakerOverview {
	overview := pluginObservabilityBreakerOverview{
		WindowHours: options.WindowHours,
		Rows:        []pluginObservabilityBreakerRow{},
	}
	if len(plugins) == 0 {
		return overview
	}

	rows := make([]pluginObservabilityBreakerRow, 0, len(plugins))
	for _, plugin := range plugins {
		if options.PluginID > 0 && plugin.ID != options.PluginID {
			continue
		}
		overview.TotalPlugins++
		if plugin.Enabled {
			overview.EnabledPlugins++
		}

		runtime := buildPluginRuntimeInspectionUnavailable(&plugin)
		if h != nil && h.pluginManager != nil {
			runtime = h.pluginManager.InspectPluginRuntime(&plugin)
		}

		windowCounters := executionWindow.perPlugin[plugin.ID]
		row := pluginObservabilityBreakerRow{
			PluginID:               plugin.ID,
			PluginName:             pluginObservabilityPluginName(plugin),
			Runtime:                firstNonEmpty(strings.TrimSpace(runtime.ResolvedRuntime), strings.TrimSpace(runtime.ConfiguredRuntime), strings.TrimSpace(plugin.Runtime)),
			Enabled:                plugin.Enabled,
			LifecycleStatus:        strings.TrimSpace(plugin.LifecycleStatus),
			HealthStatus:           strings.TrimSpace(plugin.Status),
			BreakerState:           strings.TrimSpace(runtime.BreakerState),
			FailureCount:           runtime.FailureCount,
			FailureThreshold:       runtime.FailureThreshold,
			CooldownActive:         runtime.CooldownActive,
			CooldownReason:         strings.TrimSpace(runtime.CooldownReason),
			ProbeInFlight:          runtime.ProbeInFlight,
			WindowTotalExecutions:  windowCounters.TotalExecutions,
			WindowFailedExecutions: windowCounters.FailedExecutions,
		}
		if row.FailureCount < plugin.FailCount {
			row.FailureCount = plugin.FailCount
		}
		if row.BreakerState == "" {
			if row.FailureCount > 0 {
				row.BreakerState = pluginObservabilityBreakerStateUnknown
			} else {
				row.BreakerState = pluginObservabilityBreakerStateClosed
			}
		}
		if runtime.CooldownUntil != nil && !runtime.CooldownUntil.IsZero() {
			cooldownUntil := runtime.CooldownUntil.UTC()
			row.CooldownUntil = &cooldownUntil
		}
		if runtime.ProbeStartedAt != nil && !runtime.ProbeStartedAt.IsZero() {
			probeStartedAt := runtime.ProbeStartedAt.UTC()
			row.ProbeStartedAt = &probeStartedAt
		}

		switch row.BreakerState {
		case pluginObservabilityBreakerStateOpen:
			overview.OpenCount++
		case pluginObservabilityBreakerStateHalfOpen:
			overview.HalfOpenCount++
		case pluginObservabilityBreakerStateClosed:
			overview.ClosedCount++
		}
		if row.CooldownActive {
			overview.CooldownActiveCount++
		}
		if row.ProbeInFlight {
			overview.ProbeInFlightCount++
		}
		rows = append(rows, row)
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Enabled != rows[j].Enabled {
			return rows[i].Enabled
		}
		leftState := pluginObservabilityBreakerStateRank(rows[i].BreakerState)
		rightState := pluginObservabilityBreakerStateRank(rows[j].BreakerState)
		if leftState != rightState {
			return leftState < rightState
		}
		if rows[i].WindowFailedExecutions != rows[j].WindowFailedExecutions {
			return rows[i].WindowFailedExecutions > rows[j].WindowFailedExecutions
		}
		if rows[i].FailureCount != rows[j].FailureCount {
			return rows[i].FailureCount > rows[j].FailureCount
		}
		leftName := strings.ToLower(strings.TrimSpace(rows[i].PluginName))
		rightName := strings.ToLower(strings.TrimSpace(rows[j].PluginName))
		if leftName != rightName {
			return leftName < rightName
		}
		return rows[i].PluginID < rows[j].PluginID
	})

	overview.Rows = rows
	return overview
}

func (h *PluginHandler) buildPluginObservabilityExecutionWindow(
	plugins []models.Plugin,
	now time.Time,
	options pluginObservabilityQueryOptions,
) pluginObservabilityExecutionWindowBuildResult {
	result := pluginObservabilityExecutionWindowBuildResult{
		Snapshot: pluginObservabilityExecutionWindowStats{
			WindowHours:     options.WindowHours,
			ByHour:          []pluginObservabilityExecutionHourBucket{},
			FailureGroups:   []pluginObservabilityExecutionFailureGroup{},
			HookGroups:      []pluginObservabilityExecutionHookGroup{},
			ErrorSignatures: []pluginObservabilityExecutionErrorSignature{},
			RecentFailures:  []pluginObservabilityExecutionFailureSample{},
		},
		perPlugin: map[uint]pluginObservabilityPluginWindowCounters{},
	}
	if h == nil || h.db == nil {
		return result
	}

	windowDuration := time.Duration(options.WindowHours) * time.Hour
	if windowDuration <= 0 {
		windowDuration = time.Duration(pluginObservabilityExecutionWindowDefaultHours) * time.Hour
	}
	windowStart := now.UTC().Add(-windowDuration)
	pluginNames := make(map[uint]string, len(plugins))
	pluginIDs := make([]uint, 0, len(plugins))
	for _, plugin := range plugins {
		if options.PluginID > 0 && plugin.ID != options.PluginID {
			continue
		}
		pluginNames[plugin.ID] = pluginObservabilityPluginName(plugin)
		pluginIDs = append(pluginIDs, plugin.ID)
	}
	if options.PluginID > 0 && len(pluginIDs) == 0 {
		return result
	}

	buildBaseQuery := func() *gorm.DB {
		query := h.db.Model(&models.PluginExecution{}).Where("created_at >= ?", windowStart)
		if len(pluginIDs) > 0 {
			query = query.Where("plugin_id IN ?", pluginIDs)
		}
		return query
	}

	var totalExecutions int64
	if err := buildBaseQuery().
		Count(&totalExecutions).Error; err != nil {
		return result
	}
	result.Snapshot.TotalExecutions = int(totalExecutions)

	type pluginTotalRow struct {
		PluginID        uint  `gorm:"column:plugin_id"`
		TotalExecutions int64 `gorm:"column:total_executions"`
	}
	var pluginTotals []pluginTotalRow
	if result.Snapshot.TotalExecutions > 0 {
		if err := buildBaseQuery().
			Select("plugin_id, COUNT(*) AS total_executions").
			Group("plugin_id").
			Scan(&pluginTotals).Error; err == nil {
			for _, row := range pluginTotals {
				counters := result.perPlugin[row.PluginID]
				counters.TotalExecutions = int(row.TotalExecutions)
				result.perPlugin[row.PluginID] = counters
			}
		}
	}

	var lastSucceeded models.PluginExecution
	if err := buildBaseQuery().
		Select("id, created_at").
		Where("success = ?", true).
		Order("created_at DESC").
		First(&lastSucceeded).Error; err == nil && !lastSucceeded.CreatedAt.IsZero() {
		lastSuccessAt := lastSucceeded.CreatedAt.UTC()
		result.Snapshot.LastSuccessAt = &lastSuccessAt
	}

	var lastFailed models.PluginExecution
	if err := buildBaseQuery().
		Select("id, created_at").
		Where("success = ?", false).
		Order("created_at DESC").
		First(&lastFailed).Error; err == nil && !lastFailed.CreatedAt.IsZero() {
		lastFailureAt := lastFailed.CreatedAt.UTC()
		result.Snapshot.LastFailureAt = &lastFailureAt
	}

	type failedCounterRow struct {
		PluginID               uint  `gorm:"column:plugin_id"`
		FailedExecutions       int64 `gorm:"column:failed_executions"`
		HookFailedExecutions   int64 `gorm:"column:hook_failed_executions"`
		ActionFailedExecutions int64 `gorm:"column:action_failed_executions"`
	}
	var failedCounters []failedCounterRow
	if err := buildBaseQuery().
		Where("success = ?", false).
		Select(`
			plugin_id,
			COUNT(*) AS failed_executions,
			COALESCE(SUM(CASE WHEN action = 'hook.execute' THEN 1 ELSE 0 END), 0) AS hook_failed_executions,
			COALESCE(SUM(CASE WHEN action <> 'hook.execute' THEN 1 ELSE 0 END), 0) AS action_failed_executions
		`).
		Group("plugin_id").
		Scan(&failedCounters).Error; err != nil {
		return result
	}
	for _, row := range failedCounters {
		result.Snapshot.FailedExecutions += int(row.FailedExecutions)
		result.Snapshot.HookFailedExecutions += int(row.HookFailedExecutions)
		result.Snapshot.ActionFailedExecutions += int(row.ActionFailedExecutions)
		counters := result.perPlugin[row.PluginID]
		counters.FailedExecutions = int(row.FailedExecutions)
		result.perPlugin[row.PluginID] = counters
	}

	var recentFailures []models.PluginExecution
	if err := buildBaseQuery().
		Where("success = ?", false).
		Select("id, plugin_id, action, hook, params, metadata, error, duration, created_at").
		Order("created_at DESC").
		Limit(pluginObservabilityRecentFailuresLimit).
		Find(&recentFailures).Error; err != nil {
		return result
	}
	for _, execution := range recentFailures {
		action := strings.TrimSpace(execution.Action)
		result.Snapshot.RecentFailures = append(result.Snapshot.RecentFailures, pluginObservabilityExecutionFailureSample{
			ID:         execution.ID,
			PluginID:   execution.PluginID,
			PluginName: pluginNames[execution.PluginID],
			Action:     action,
			Hook:       resolvePluginExecutionObservedHook(action, execution.Hook, execution.Params, execution.Metadata),
			Error:      service.NormalizePluginExecutionErrorText(execution.Error),
			Duration:   execution.Duration,
			CreatedAt:  execution.CreatedAt.UTC(),
		})
	}

	type failureGroupRow struct {
		PluginID         uint   `gorm:"column:plugin_id"`
		Action           string `gorm:"column:action"`
		Hook             string `gorm:"column:hook"`
		FailureCount     int64  `gorm:"column:failure_count"`
		LastFailureAtRaw string `gorm:"column:last_failure_at"`
	}
	var failureGroups []failureGroupRow
	if err := buildBaseQuery().
		Where("success = ?", false).
		Select("plugin_id, action, hook, COUNT(*) AS failure_count, MAX(created_at) AS last_failure_at").
		Group("plugin_id, action, hook").
		Order("failure_count DESC").
		Order("last_failure_at DESC").
		Order("plugin_id ASC").
		Order("hook ASC").
		Order("action ASC").
		Limit(pluginObservabilityFailureGroupsLimit).
		Scan(&failureGroups).Error; err != nil {
		return result
	}
	result.Snapshot.FailureGroups = make([]pluginObservabilityExecutionFailureGroup, 0, len(failureGroups))
	for _, group := range failureGroups {
		lastFailureAt, _ := parsePluginAggregateTimestamp(group.LastFailureAtRaw)
		result.Snapshot.FailureGroups = append(result.Snapshot.FailureGroups, pluginObservabilityExecutionFailureGroup{
			PluginID:      group.PluginID,
			PluginName:    pluginNames[group.PluginID],
			Action:        strings.TrimSpace(group.Action),
			Hook:          strings.TrimSpace(group.Hook),
			FailureCount:  int(group.FailureCount),
			LastFailureAt: optionalPluginAggregateTimestamp(lastFailureAt),
		})
	}

	type hookGroupRow struct {
		PluginID         uint   `gorm:"column:plugin_id"`
		Hook             string `gorm:"column:hook"`
		FailureCount     int64  `gorm:"column:failure_count"`
		LastFailureAtRaw string `gorm:"column:last_failure_at"`
		LastError        string `gorm:"column:last_error"`
	}
	var hookGroups []hookGroupRow
	if err := buildBaseQuery().
		Where("success = ? AND hook <> ?", false, "").
		Select("plugin_id, hook, COUNT(*) AS failure_count, MAX(created_at) AS last_failure_at, MAX(error) AS last_error").
		Group("plugin_id, hook").
		Order("failure_count DESC").
		Order("last_failure_at DESC").
		Order("plugin_id ASC").
		Order("hook ASC").
		Limit(pluginObservabilityHookGroupsLimit).
		Scan(&hookGroups).Error; err != nil {
		return result
	}
	result.Snapshot.HookGroups = make([]pluginObservabilityExecutionHookGroup, 0, len(hookGroups))
	for _, group := range hookGroups {
		lastFailureAt, _ := parsePluginAggregateTimestamp(group.LastFailureAtRaw)
		result.Snapshot.HookGroups = append(result.Snapshot.HookGroups, pluginObservabilityExecutionHookGroup{
			PluginID:      group.PluginID,
			PluginName:    pluginNames[group.PluginID],
			Hook:          strings.TrimSpace(group.Hook),
			FailureCount:  int(group.FailureCount),
			LastFailureAt: optionalPluginAggregateTimestamp(lastFailureAt),
			LastError:     service.NormalizePluginExecutionErrorText(group.LastError),
		})
	}

	type errorSignatureRow struct {
		PluginID         uint   `gorm:"column:plugin_id"`
		Signature        string `gorm:"column:error_signature"`
		FailureCount     int64  `gorm:"column:failure_count"`
		LastFailureAtRaw string `gorm:"column:last_failure_at"`
		SampleError      string `gorm:"column:sample_error"`
	}
	var errorSignatures []errorSignatureRow
	if err := buildBaseQuery().
		Where("success = ? AND error_signature <> ?", false, "").
		Select("plugin_id, error_signature, COUNT(*) AS failure_count, MAX(created_at) AS last_failure_at, MAX(error) AS sample_error").
		Group("plugin_id, error_signature").
		Order("failure_count DESC").
		Order("last_failure_at DESC").
		Order("plugin_id ASC").
		Order("error_signature ASC").
		Limit(pluginObservabilityErrorSignaturesLimit).
		Scan(&errorSignatures).Error; err != nil {
		return result
	}
	result.Snapshot.ErrorSignatures = make([]pluginObservabilityExecutionErrorSignature, 0, len(errorSignatures))
	for _, group := range errorSignatures {
		lastFailureAt, _ := parsePluginAggregateTimestamp(group.LastFailureAtRaw)
		result.Snapshot.ErrorSignatures = append(result.Snapshot.ErrorSignatures, pluginObservabilityExecutionErrorSignature{
			PluginID:      group.PluginID,
			PluginName:    pluginNames[group.PluginID],
			Signature:     strings.TrimSpace(group.Signature),
			FailureCount:  int(group.FailureCount),
			LastFailureAt: optionalPluginAggregateTimestamp(lastFailureAt),
			SampleError:   service.NormalizePluginExecutionErrorText(group.SampleError),
		})
	}

	result.Snapshot.ByHour = h.buildPluginObservabilityExecutionHourBuckets(now, options, pluginIDs)
	return result
}

func (h *PluginHandler) buildPluginObservabilityExecutionHourBuckets(
	now time.Time,
	options pluginObservabilityQueryOptions,
	pluginIDs []uint,
) []pluginObservabilityExecutionHourBucket {
	if h == nil || h.db == nil {
		return []pluginObservabilityExecutionHourBucket{}
	}

	bucketCount := options.WindowHours
	if bucketCount <= 0 {
		bucketCount = pluginObservabilityExecutionWindowDefaultHours
	}
	chartStart := now.UTC().Truncate(time.Hour).Add(-time.Duration(bucketCount-1) * time.Hour)
	type hourRow struct {
		HourStart        string `gorm:"column:hour_start"`
		TotalExecutions  int64  `gorm:"column:total_executions"`
		FailedExecutions int64  `gorm:"column:failed_executions"`
	}
	var rows []hourRow
	selectExpr := fmt.Sprintf(
		"%s AS hour_start, COUNT(*) AS total_executions, COALESCE(SUM(CASE WHEN success THEN 0 ELSE 1 END), 0) AS failed_executions",
		pluginObservabilityHourBucketExpression(h.db),
	)
	query := h.db.Model(&models.PluginExecution{}).
		Where("created_at >= ?", chartStart)
	if len(pluginIDs) > 0 {
		query = query.Where("plugin_id IN ?", pluginIDs)
	}
	if err := query.
		Select(selectExpr).
		Group("hour_start").
		Order("hour_start ASC").
		Scan(&rows).Error; err != nil || len(rows) == 0 {
		return []pluginObservabilityExecutionHourBucket{}
	}

	type hourCounters struct {
		TotalExecutions  int
		FailedExecutions int
	}
	bucketMap := map[string]hourCounters{}
	for _, row := range rows {
		hourStart, ok := parsePluginObservabilityHourStart(row.HourStart)
		if !ok {
			continue
		}
		key := hourStart.UTC().Format(time.RFC3339)
		bucketMap[key] = hourCounters{
			TotalExecutions:  int(row.TotalExecutions),
			FailedExecutions: int(row.FailedExecutions),
		}
	}
	if len(bucketMap) == 0 {
		return []pluginObservabilityExecutionHourBucket{}
	}

	out := make([]pluginObservabilityExecutionHourBucket, 0, bucketCount)
	for idx := 0; idx < bucketCount; idx++ {
		hourStart := chartStart.Add(time.Duration(idx) * time.Hour).UTC()
		counters := bucketMap[hourStart.Format(time.RFC3339)]
		out = append(out, pluginObservabilityExecutionHourBucket{
			HourStart:        hourStart,
			TotalExecutions:  counters.TotalExecutions,
			FailedExecutions: counters.FailedExecutions,
		})
	}
	return out
}

func pluginObservabilityHourBucketExpression(db *gorm.DB) string {
	if db == nil || db.Dialector == nil {
		return "DATE_FORMAT(created_at, '%Y-%m-%dT%H:00:00Z')"
	}
	dialectName := strings.ToLower(strings.TrimSpace(db.Dialector.Name()))
	switch dialectName {
	case "sqlite":
		return "strftime('%Y-%m-%dT%H:00:00Z', created_at)"
	case "postgres", "postgresql":
		return "to_char(date_trunc('hour', created_at AT TIME ZONE 'UTC'), 'YYYY-MM-DD\"T\"HH24:00:00\"Z\"')"
	default:
		return "DATE_FORMAT(created_at, '%Y-%m-%dT%H:00:00Z')"
	}
}

func parsePluginObservabilityHourStart(raw string) (time.Time, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Time{}, false
	}

	layouts := []string{
		time.RFC3339,
		pluginObservabilityDefaultBucketTimeLayout,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02 15:04:05-07:00",
	}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, trimmed)
		if err == nil {
			return parsed.UTC(), true
		}
	}
	return time.Time{}, false
}

func pluginObservabilityBreakerStateRank(state string) int {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case pluginObservabilityBreakerStateOpen:
		return 0
	case pluginObservabilityBreakerStateHalfOpen:
		return 1
	case pluginObservabilityBreakerStateClosed:
		return 2
	case pluginObservabilityBreakerStateUnknown:
		return 3
	default:
		return 4
	}
}

func pluginObservabilityPluginName(plugin models.Plugin) string {
	return firstNonEmpty(strings.TrimSpace(plugin.DisplayName), strings.TrimSpace(plugin.Name))
}
