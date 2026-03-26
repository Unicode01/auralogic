package admin

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	pluginPublicEndpointCacheTTL        = 5 * time.Second
	pluginPublicEndpointCacheMaxEntries = 512
)

type pluginPublicEndpointCache struct {
	ttl        time.Duration
	maxEntries int
	mu         sync.RWMutex
	entries    map[string]pluginPublicEndpointCacheEntry
}

type pluginPublicEndpointCacheEntry struct {
	value     interface{}
	expiresAt time.Time
	createdAt time.Time
}

type pluginPublicEndpointCacheSnapshotEntry struct {
	Key       string
	Value     interface{}
	CreatedAt time.Time
	ExpiresAt time.Time
}

func newPluginPublicEndpointCache(ttl time.Duration, maxEntries int) *pluginPublicEndpointCache {
	if ttl <= 0 {
		ttl = pluginPublicEndpointCacheTTL
	}
	if maxEntries <= 0 {
		maxEntries = pluginPublicEndpointCacheMaxEntries
	}
	return &pluginPublicEndpointCache{
		ttl:        ttl,
		maxEntries: maxEntries,
		entries:    make(map[string]pluginPublicEndpointCacheEntry, maxEntries),
	}
}

func (c *pluginPublicEndpointCache) Get(key string) (interface{}, bool) {
	if c == nil {
		return nil, false
	}
	normalizedKey := strings.TrimSpace(key)
	if normalizedKey == "" {
		return nil, false
	}

	now := time.Now()
	c.mu.RLock()
	entry, exists := c.entries[normalizedKey]
	c.mu.RUnlock()
	if !exists {
		return nil, false
	}
	if now.After(entry.expiresAt) {
		c.mu.Lock()
		if current, ok := c.entries[normalizedKey]; ok && now.After(current.expiresAt) {
			delete(c.entries, normalizedKey)
		}
		c.mu.Unlock()
		return nil, false
	}
	return entry.value, true
}

func (c *pluginPublicEndpointCache) Set(key string, value interface{}) {
	if c == nil {
		return
	}
	normalizedKey := strings.TrimSpace(key)
	if normalizedKey == "" {
		return
	}

	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()

	c.sweepExpiredLocked(now)
	if _, exists := c.entries[normalizedKey]; !exists && len(c.entries) >= c.maxEntries {
		c.evictOneLocked()
	}
	c.entries[normalizedKey] = pluginPublicEndpointCacheEntry{
		value:     value,
		expiresAt: now.Add(c.ttl),
		createdAt: now,
	}
}

func (c *pluginPublicEndpointCache) Clear() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]pluginPublicEndpointCacheEntry, c.maxEntries)
}

func (c *pluginPublicEndpointCache) Snapshot() []pluginPublicEndpointCacheSnapshotEntry {
	if c == nil {
		return []pluginPublicEndpointCacheSnapshotEntry{}
	}

	now := time.Now()
	c.mu.Lock()
	c.sweepExpiredLocked(now)
	snapshots := make([]pluginPublicEndpointCacheSnapshotEntry, 0, len(c.entries))
	for key, entry := range c.entries {
		snapshots = append(snapshots, pluginPublicEndpointCacheSnapshotEntry{
			Key:       key,
			Value:     entry.value,
			CreatedAt: entry.createdAt,
			ExpiresAt: entry.expiresAt,
		})
	}
	c.mu.Unlock()

	sort.Slice(snapshots, func(i, j int) bool {
		if snapshots[i].Key == snapshots[j].Key {
			return snapshots[i].CreatedAt.Before(snapshots[j].CreatedAt)
		}
		return snapshots[i].Key < snapshots[j].Key
	})
	return snapshots
}

func (c *pluginPublicEndpointCache) sweepExpiredLocked(now time.Time) {
	for key, entry := range c.entries {
		if now.After(entry.expiresAt) {
			delete(c.entries, key)
		}
	}
}

func (c *pluginPublicEndpointCache) evictOneLocked() {
	var (
		selectedKey string
		selected    pluginPublicEndpointCacheEntry
		found       bool
	)
	for key, entry := range c.entries {
		if !found || entry.expiresAt.Before(selected.expiresAt) || (entry.expiresAt.Equal(selected.expiresAt) && entry.createdAt.Before(selected.createdAt)) {
			selectedKey = key
			selected = entry
			found = true
		}
	}
	if found {
		delete(c.entries, selectedKey)
	}
}

func buildPublicExtensionsCacheKey(slot, path string, queryParams map[string]string, hostContext map[string]interface{}, scope pluginAccessScope, varyKey string) string {
	return fmt.Sprintf(
		"extensions|slot=%s|path=%s|query=%s|host_context=%s|scope=%s|vary=%s",
		strings.TrimSpace(slot),
		strings.TrimSpace(path),
		marshalFrontendParamMapJSON(queryParams),
		marshalFrontendHostContextJSON(hostContext),
		buildPluginAccessScopeCacheKey(scope),
		normalizePluginPublicCacheVaryKey(varyKey),
	)
}

func buildPublicBootstrapCacheKey(area, path string, queryParams map[string]string, scope pluginAccessScope, varyKey string) string {
	return fmt.Sprintf(
		"bootstrap|area=%s|path=%s|query=%s|scope=%s|vary=%s",
		strings.TrimSpace(area),
		strings.TrimSpace(path),
		marshalFrontendParamMapJSON(queryParams),
		buildPluginAccessScopeCacheKey(scope),
		normalizePluginPublicCacheVaryKey(varyKey),
	)
}

func buildFrontendBootstrapInternalCacheKey(area, path string, queryParams map[string]string, scope pluginAccessScope, varyKey string) string {
	return fmt.Sprintf(
		"frontend-bootstrap|area=%s|path=%s|query=%s|scope=%s|vary=%s",
		strings.TrimSpace(area),
		strings.TrimSpace(path),
		marshalFrontendParamMapJSON(queryParams),
		buildPluginAccessScopeCacheKey(scope),
		normalizePluginPublicCacheVaryKey(varyKey),
	)
}

func buildPublicRequestCacheVaryKey(subjectKey, sessionID, acceptLanguage, clientIP, userAgent string) string {
	return fmt.Sprintf(
		"subject=%s|session=%s|lang=%s|ip=%s|ua=%s",
		normalizePluginPublicCacheVaryValue(subjectKey),
		normalizePluginPublicCacheVaryValue(sessionID),
		normalizePluginPublicCacheVaryValue(acceptLanguage),
		normalizePluginPublicCacheVaryValue(clientIP),
		normalizePluginPublicCacheVaryValue(userAgent),
	)
}

func normalizePluginPublicCacheVaryKey(varyKey string) string {
	trimmed := strings.TrimSpace(varyKey)
	if trimmed == "" {
		return "default"
	}
	return trimmed
}

func normalizePluginPublicCacheVaryValue(value string) string {
	return fmt.Sprintf("%q", strings.TrimSpace(value))
}

func buildPluginAccessScopeCacheKey(scope pluginAccessScope) string {
	if !scope.authenticated {
		return "guest"
	}
	if scope.superAdmin {
		return "auth:super_admin"
	}
	if len(scope.permissions) == 0 {
		return "auth:user"
	}
	keys := make([]string, 0, len(scope.permissions))
	for key := range scope.permissions {
		normalized := strings.ToLower(strings.TrimSpace(key))
		if normalized == "" {
			continue
		}
		keys = append(keys, normalized)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return "auth:user"
	}
	return "auth:user|perm=" + strings.Join(keys, ",")
}
