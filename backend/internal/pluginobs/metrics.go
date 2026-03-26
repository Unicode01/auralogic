package pluginobs

import (
	"sort"
	"strings"
	"sync"
	"time"
)

type executionCounters struct {
	Total           uint64
	Success         uint64
	Failed          uint64
	Timeout         uint64
	DurationTotalMs uint64
	MaxDurationMs   int64
}

type pluginExecutionCounters struct {
	PluginID   uint
	PluginName string
	Runtime    string
	Counters   executionCounters
}

type publicEndpointCounters struct {
	Requests    uint64
	RateLimited uint64
	CacheHits   uint64
	CacheMisses uint64
}

type frontendResolverCounters struct {
	CacheHits        uint64
	CacheMisses      uint64
	SingleflightWait uint64
	CatalogHits      uint64
	DBFallbacks      uint64
}

type frontendCounters struct {
	SlotRequests      uint64
	BatchRequests     uint64
	BootstrapRequests uint64
	BatchItems        uint64
	BatchUniqueItems  uint64
	BatchDedupedItems uint64
	HTMLMode          frontendResolverCounters
	ExecuteAPI        frontendResolverCounters
	PreparedHook      frontendResolverCounters
}

type metricsCollector struct {
	mu              sync.RWMutex
	execution       executionCounters
	executionByRun  map[string]*executionCounters
	executionByAct  map[string]*executionCounters
	executionByPlug map[uint]*pluginExecutionCounters
	hookLimiterHits map[string]uint64
	publicEndpoints map[string]*publicEndpointCounters
	frontend        frontendCounters
}

type ExecutionCountersSnapshot struct {
	Total         uint64  `json:"total"`
	Success       uint64  `json:"success"`
	Failed        uint64  `json:"failed"`
	ErrorRate     float64 `json:"error_rate"`
	Timeout       uint64  `json:"timeout"`
	TimeoutRate   float64 `json:"timeout_rate"`
	AvgDurationMs float64 `json:"avg_duration_ms"`
	MaxDurationMs int64   `json:"max_duration_ms"`
}

type PluginExecutionSnapshot struct {
	PluginID   uint   `json:"plugin_id"`
	PluginName string `json:"plugin_name"`
	Runtime    string `json:"runtime"`
	ExecutionCountersSnapshot
}

type HookLimiterSnapshot struct {
	TotalHits uint64            `json:"total_hits"`
	ByHook    map[string]uint64 `json:"by_hook"`
}

type PublicEndpointSnapshot struct {
	Requests         uint64  `json:"requests"`
	RateLimited      uint64  `json:"rate_limited"`
	RateLimitHitRate float64 `json:"rate_limit_hit_rate"`
	CacheHits        uint64  `json:"cache_hits"`
	CacheMisses      uint64  `json:"cache_misses"`
	CacheHitRate     float64 `json:"cache_hit_rate"`
}

type FrontendResolverSnapshot struct {
	CacheHits        uint64  `json:"cache_hits"`
	CacheMisses      uint64  `json:"cache_misses"`
	CacheHitRate     float64 `json:"cache_hit_rate"`
	SingleflightWait uint64  `json:"singleflight_waits"`
	CatalogHits      uint64  `json:"catalog_hits"`
	DBFallbacks      uint64  `json:"db_fallbacks"`
}

type FrontendSnapshot struct {
	SlotRequests      uint64                   `json:"slot_requests"`
	BatchRequests     uint64                   `json:"batch_requests"`
	BootstrapRequests uint64                   `json:"bootstrap_requests"`
	BatchItems        uint64                   `json:"batch_items"`
	BatchUniqueItems  uint64                   `json:"batch_unique_items"`
	BatchDedupedItems uint64                   `json:"batch_deduped_items"`
	HTMLMode          FrontendResolverSnapshot `json:"html_mode"`
	ExecuteAPI        FrontendResolverSnapshot `json:"execute_api"`
	PreparedHook      FrontendResolverSnapshot `json:"prepared_hook"`
}

type Snapshot struct {
	GeneratedAt  time.Time                         `json:"generated_at"`
	Execution    ExecutionSnapshot                 `json:"execution"`
	HookLimiter  HookLimiterSnapshot               `json:"hook_limiter"`
	PublicAccess map[string]PublicEndpointSnapshot `json:"public_access"`
	Frontend     FrontendSnapshot                  `json:"frontend"`
}

type ExecutionSnapshot struct {
	Overall   ExecutionCountersSnapshot            `json:"overall"`
	ByRuntime map[string]ExecutionCountersSnapshot `json:"by_runtime"`
	ByAction  map[string]ExecutionCountersSnapshot `json:"by_action"`
	ByPlugin  []PluginExecutionSnapshot            `json:"by_plugin"`
}

var defaultCollector = newMetricsCollector()

func newMetricsCollector() *metricsCollector {
	return &metricsCollector{
		executionByRun:  map[string]*executionCounters{},
		executionByAct:  map[string]*executionCounters{},
		executionByPlug: map[uint]*pluginExecutionCounters{},
		hookLimiterHits: map[string]uint64{},
		publicEndpoints: map[string]*publicEndpointCounters{},
	}
}

func RecordExecution(pluginID uint, pluginName string, runtime string, action string, durationMs int64, success bool, timedOut bool) {
	defaultCollector.recordExecution(pluginID, pluginName, runtime, action, durationMs, success, timedOut)
}

func RecordHookLimiterHit(hook string) {
	defaultCollector.recordHookLimiterHit(hook)
}

func RecordPublicCache(endpoint string, hit bool) {
	defaultCollector.recordPublicCache(endpoint, hit)
}

func RecordPublicRequest(endpoint string) {
	defaultCollector.recordPublicRequest(endpoint)
}

func RecordPublicRateLimit(endpoint string, blocked bool) {
	defaultCollector.recordPublicRateLimit(endpoint, blocked)
}

func RecordFrontendSlotRequest() {
	defaultCollector.recordFrontendSlotRequest()
}

func RecordFrontendBootstrapRequest() {
	defaultCollector.recordFrontendBootstrapRequest()
}

func RecordFrontendBatchRequest(itemCount int, uniqueCount int) {
	defaultCollector.recordFrontendBatchRequest(itemCount, uniqueCount)
}

func RecordFrontendResolverCacheHit(kind string, count int) {
	defaultCollector.recordFrontendResolverEvent(kind, "cache_hit", count)
}

func RecordFrontendResolverCacheMiss(kind string, count int) {
	defaultCollector.recordFrontendResolverEvent(kind, "cache_miss", count)
}

func RecordFrontendResolverSingleflightWait(kind string, count int) {
	defaultCollector.recordFrontendResolverEvent(kind, "singleflight_wait", count)
}

func RecordFrontendResolverCatalogHit(kind string, count int) {
	defaultCollector.recordFrontendResolverEvent(kind, "catalog_hit", count)
}

func RecordFrontendResolverDBFallback(kind string, count int) {
	defaultCollector.recordFrontendResolverEvent(kind, "db_fallback", count)
}

func SnapshotNow() Snapshot {
	return defaultCollector.snapshot(time.Now().UTC())
}

func ResetForTest() {
	defaultCollector = newMetricsCollector()
}

func (c *metricsCollector) recordExecution(pluginID uint, pluginName string, runtime string, action string, durationMs int64, success bool, timedOut bool) {
	if c == nil {
		return
	}
	if durationMs < 0 {
		durationMs = 0
	}
	resolvedRuntime := normalizeLabel(runtime, "unknown")
	resolvedAction := normalizeLabel(action, "unknown")

	c.mu.Lock()
	defer c.mu.Unlock()

	applyExecutionSample(&c.execution, durationMs, success, timedOut)
	runtimeCounters := ensureExecutionCounterMap(c.executionByRun, resolvedRuntime)
	applyExecutionSample(runtimeCounters, durationMs, success, timedOut)
	actionCounters := ensureExecutionCounterMap(c.executionByAct, resolvedAction)
	applyExecutionSample(actionCounters, durationMs, success, timedOut)

	if pluginID > 0 {
		pluginCounters, exists := c.executionByPlug[pluginID]
		if !exists {
			pluginCounters = &pluginExecutionCounters{
				PluginID: pluginID,
				Runtime:  resolvedRuntime,
			}
			c.executionByPlug[pluginID] = pluginCounters
		}
		if strings.TrimSpace(pluginName) != "" {
			pluginCounters.PluginName = strings.TrimSpace(pluginName)
		}
		pluginCounters.Runtime = resolvedRuntime
		applyExecutionSample(&pluginCounters.Counters, durationMs, success, timedOut)
	}
}

func (c *metricsCollector) recordHookLimiterHit(hook string) {
	if c == nil {
		return
	}
	resolvedHook := normalizeLabel(hook, "unknown")

	c.mu.Lock()
	c.hookLimiterHits[resolvedHook]++
	c.mu.Unlock()
}

func (c *metricsCollector) recordPublicCache(endpoint string, hit bool) {
	if c == nil {
		return
	}
	resolvedEndpoint := normalizeLabel(endpoint, "unknown")

	c.mu.Lock()
	metrics := ensurePublicEndpointCounter(c.publicEndpoints, resolvedEndpoint)
	if hit {
		metrics.CacheHits++
	} else {
		metrics.CacheMisses++
	}
	c.mu.Unlock()
}

func (c *metricsCollector) recordPublicRequest(endpoint string) {
	if c == nil {
		return
	}
	resolvedEndpoint := normalizeLabel(endpoint, "unknown")

	c.mu.Lock()
	metrics := ensurePublicEndpointCounter(c.publicEndpoints, resolvedEndpoint)
	metrics.Requests++
	c.mu.Unlock()
}

func (c *metricsCollector) recordPublicRateLimit(endpoint string, blocked bool) {
	if c == nil || !blocked {
		return
	}
	resolvedEndpoint := normalizeLabel(endpoint, "unknown")

	c.mu.Lock()
	metrics := ensurePublicEndpointCounter(c.publicEndpoints, resolvedEndpoint)
	metrics.RateLimited++
	c.mu.Unlock()
}

func (c *metricsCollector) recordFrontendSlotRequest() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.frontend.SlotRequests++
	c.mu.Unlock()
}

func (c *metricsCollector) recordFrontendBootstrapRequest() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.frontend.BootstrapRequests++
	c.mu.Unlock()
}

func (c *metricsCollector) recordFrontendBatchRequest(itemCount int, uniqueCount int) {
	if c == nil {
		return
	}
	if itemCount < 0 {
		itemCount = 0
	}
	if uniqueCount < 0 {
		uniqueCount = 0
	}
	if uniqueCount > itemCount {
		uniqueCount = itemCount
	}
	c.mu.Lock()
	c.frontend.BatchRequests++
	c.frontend.BatchItems += uint64(itemCount)
	c.frontend.BatchUniqueItems += uint64(uniqueCount)
	c.frontend.BatchDedupedItems += uint64(itemCount - uniqueCount)
	c.mu.Unlock()
}

func (c *metricsCollector) recordFrontendResolverEvent(kind string, event string, count int) {
	if c == nil || count <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	counters := resolveFrontendResolverCounters(&c.frontend, kind)
	switch normalizeLabel(event, "") {
	case "cache_hit":
		counters.CacheHits += uint64(count)
	case "cache_miss":
		counters.CacheMisses += uint64(count)
	case "singleflight_wait":
		counters.SingleflightWait += uint64(count)
	case "catalog_hit":
		counters.CatalogHits += uint64(count)
	case "db_fallback":
		counters.DBFallbacks += uint64(count)
	}
}

func (c *metricsCollector) snapshot(now time.Time) Snapshot {
	if c == nil {
		return Snapshot{
			GeneratedAt:  now,
			Execution:    ExecutionSnapshot{ByRuntime: map[string]ExecutionCountersSnapshot{}, ByAction: map[string]ExecutionCountersSnapshot{}, ByPlugin: []PluginExecutionSnapshot{}},
			HookLimiter:  HookLimiterSnapshot{ByHook: map[string]uint64{}},
			PublicAccess: map[string]PublicEndpointSnapshot{},
			Frontend:     FrontendSnapshot{},
		}
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	execSnapshot := ExecutionSnapshot{
		Overall:   toExecutionCountersSnapshot(c.execution),
		ByRuntime: map[string]ExecutionCountersSnapshot{},
		ByAction:  map[string]ExecutionCountersSnapshot{},
		ByPlugin:  make([]PluginExecutionSnapshot, 0, len(c.executionByPlug)),
	}
	for key, counters := range c.executionByRun {
		if counters == nil {
			continue
		}
		execSnapshot.ByRuntime[key] = toExecutionCountersSnapshot(*counters)
	}
	for key, counters := range c.executionByAct {
		if counters == nil {
			continue
		}
		execSnapshot.ByAction[key] = toExecutionCountersSnapshot(*counters)
	}

	pluginKeys := make([]uint, 0, len(c.executionByPlug))
	for pluginID := range c.executionByPlug {
		pluginKeys = append(pluginKeys, pluginID)
	}
	sort.Slice(pluginKeys, func(i, j int) bool {
		return pluginKeys[i] < pluginKeys[j]
	})
	for _, pluginID := range pluginKeys {
		counters := c.executionByPlug[pluginID]
		if counters == nil {
			continue
		}
		execSnapshot.ByPlugin = append(execSnapshot.ByPlugin, PluginExecutionSnapshot{
			PluginID:                  counters.PluginID,
			PluginName:                counters.PluginName,
			Runtime:                   counters.Runtime,
			ExecutionCountersSnapshot: toExecutionCountersSnapshot(counters.Counters),
		})
	}

	hookSnapshot := HookLimiterSnapshot{
		ByHook: map[string]uint64{},
	}
	for hook, count := range c.hookLimiterHits {
		hookSnapshot.ByHook[hook] = count
		hookSnapshot.TotalHits += count
	}

	publicSnapshot := map[string]PublicEndpointSnapshot{}
	for endpoint, counters := range c.publicEndpoints {
		if counters == nil {
			continue
		}
		lookupTotal := counters.CacheHits + counters.CacheMisses
		publicSnapshot[endpoint] = PublicEndpointSnapshot{
			Requests:         counters.Requests,
			RateLimited:      counters.RateLimited,
			RateLimitHitRate: ratio(counters.RateLimited, counters.Requests),
			CacheHits:        counters.CacheHits,
			CacheMisses:      counters.CacheMisses,
			CacheHitRate:     ratio(counters.CacheHits, lookupTotal),
		}
	}

	return Snapshot{
		GeneratedAt:  now,
		Execution:    execSnapshot,
		HookLimiter:  hookSnapshot,
		PublicAccess: publicSnapshot,
		Frontend: FrontendSnapshot{
			SlotRequests:      c.frontend.SlotRequests,
			BatchRequests:     c.frontend.BatchRequests,
			BootstrapRequests: c.frontend.BootstrapRequests,
			BatchItems:        c.frontend.BatchItems,
			BatchUniqueItems:  c.frontend.BatchUniqueItems,
			BatchDedupedItems: c.frontend.BatchDedupedItems,
			HTMLMode:          toFrontendResolverSnapshot(c.frontend.HTMLMode),
			ExecuteAPI:        toFrontendResolverSnapshot(c.frontend.ExecuteAPI),
			PreparedHook:      toFrontendResolverSnapshot(c.frontend.PreparedHook),
		},
	}
}

func applyExecutionSample(counters *executionCounters, durationMs int64, success bool, timedOut bool) {
	if counters == nil {
		return
	}
	counters.Total++
	if success {
		counters.Success++
	} else {
		counters.Failed++
	}
	if timedOut {
		counters.Timeout++
	}
	counters.DurationTotalMs += uint64(durationMs)
	if durationMs > counters.MaxDurationMs {
		counters.MaxDurationMs = durationMs
	}
}

func ensureExecutionCounterMap(source map[string]*executionCounters, key string) *executionCounters {
	counters, exists := source[key]
	if exists && counters != nil {
		return counters
	}
	counters = &executionCounters{}
	source[key] = counters
	return counters
}

func ensurePublicEndpointCounter(source map[string]*publicEndpointCounters, key string) *publicEndpointCounters {
	counters, exists := source[key]
	if exists && counters != nil {
		return counters
	}
	counters = &publicEndpointCounters{}
	source[key] = counters
	return counters
}

func resolveFrontendResolverCounters(frontend *frontendCounters, kind string) *frontendResolverCounters {
	if frontend == nil {
		return &frontendResolverCounters{}
	}
	switch normalizeLabel(kind, "") {
	case "html_mode":
		return &frontend.HTMLMode
	case "execute_api":
		return &frontend.ExecuteAPI
	case "prepared_hook":
		return &frontend.PreparedHook
	default:
		return &frontend.PreparedHook
	}
}

func toExecutionCountersSnapshot(counters executionCounters) ExecutionCountersSnapshot {
	return ExecutionCountersSnapshot{
		Total:         counters.Total,
		Success:       counters.Success,
		Failed:        counters.Failed,
		ErrorRate:     ratio(counters.Failed, counters.Total),
		Timeout:       counters.Timeout,
		TimeoutRate:   ratio(counters.Timeout, counters.Total),
		AvgDurationMs: averageDuration(counters.DurationTotalMs, counters.Total),
		MaxDurationMs: counters.MaxDurationMs,
	}
}

func toFrontendResolverSnapshot(counters frontendResolverCounters) FrontendResolverSnapshot {
	lookupTotal := counters.CacheHits + counters.CacheMisses
	return FrontendResolverSnapshot{
		CacheHits:        counters.CacheHits,
		CacheMisses:      counters.CacheMisses,
		CacheHitRate:     ratio(counters.CacheHits, lookupTotal),
		SingleflightWait: counters.SingleflightWait,
		CatalogHits:      counters.CatalogHits,
		DBFallbacks:      counters.DBFallbacks,
	}
}

func averageDuration(total uint64, count uint64) float64 {
	if count == 0 {
		return 0
	}
	return round(float64(total) / float64(count))
}

func ratio(part uint64, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return round(float64(part) / float64(total))
}

func round(value float64) float64 {
	// Keep 4 decimal places for stable JSON outputs and tests.
	return float64(int(value*10000+0.5)) / 10000
}

func normalizeLabel(raw string, fallback string) string {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" {
		return fallback
	}
	return normalized
}
