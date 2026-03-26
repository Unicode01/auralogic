package service

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"auralogic/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	pluginStorageLocks   = make(map[uint]*sync.RWMutex)
	pluginStorageLocksMu sync.Mutex
)

const (
	defaultPluginStorageMaxKeys       = 512
	defaultPluginStorageMaxTotalBytes = int64(4 * 1024 * 1024)
	defaultPluginStorageMaxValueBytes = int64(64 * 1024)
	defaultPluginStorageMaxKeyBytes   = 191

	pluginStorageAccessUnknown = "unknown"
	pluginStorageAccessNone    = "none"
	pluginStorageAccessRead    = "read"
	pluginStorageAccessWrite   = "write"
	pluginStorageAccessMetaKey = "storage_access_mode"
)

type pluginStorageQuota struct {
	MaxKeys       int
	MaxTotalBytes int64
	MaxValueBytes int64
	MaxKeyBytes   int
}

type pluginStorageDelta struct {
	removedKeys []string
	upserts     []models.PluginStorageEntry
}

func getPluginStorageLock(pluginID uint) *sync.RWMutex {
	pluginStorageLocksMu.Lock()
	defer pluginStorageLocksMu.Unlock()

	if lock, ok := pluginStorageLocks[pluginID]; ok {
		return lock
	}
	lock := &sync.RWMutex{}
	pluginStorageLocks[pluginID] = lock
	return lock
}

func releasePluginStorageLock(pluginID uint) {
	pluginStorageLocksMu.Lock()
	defer pluginStorageLocksMu.Unlock()
	delete(pluginStorageLocks, pluginID)
}

func normalizePluginStorageAccessMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case pluginStorageAccessNone:
		return pluginStorageAccessNone
	case pluginStorageAccessRead:
		return pluginStorageAccessRead
	case pluginStorageAccessWrite:
		return pluginStorageAccessWrite
	default:
		return pluginStorageAccessUnknown
	}
}

func pluginStorageAccessModeRank(mode string) int {
	switch normalizePluginStorageAccessMode(mode) {
	case pluginStorageAccessNone:
		return 0
	case pluginStorageAccessRead:
		return 1
	case pluginStorageAccessWrite:
		return 2
	default:
		return 3
	}
}

func acquirePluginStorageExecutionLock(pluginID uint, mode string) func() {
	if pluginID == 0 {
		return func() {}
	}
	lock := getPluginStorageLock(pluginID)
	switch normalizePluginStorageAccessMode(mode) {
	case pluginStorageAccessRead:
		lock.RLock()
		return lock.RUnlock
	case pluginStorageAccessNone:
		return func() {}
	default:
		lock.Lock()
		return lock.Unlock
	}
}

func resolvePluginStorageAccessModeFromMetadata(metadata map[string]string) string {
	if len(metadata) == 0 {
		return pluginStorageAccessUnknown
	}
	return normalizePluginStorageAccessMode(metadata[pluginStorageAccessMetaKey])
}

func validatePluginStorageAccessMode(
	declaredMode string,
	actualMode string,
	storageChanged bool,
) error {
	declaredMode = normalizePluginStorageAccessMode(declaredMode)
	actualMode = normalizePluginStorageAccessMode(actualMode)

	if declaredMode == pluginStorageAccessUnknown || declaredMode == pluginStorageAccessWrite {
		return nil
	}
	if storageChanged {
		return fmt.Errorf(
			"plugin action declared storage access %s but attempted to write Plugin.storage",
			declaredMode,
		)
	}
	if actualMode == pluginStorageAccessUnknown {
		return nil
	}
	if pluginStorageAccessModeRank(actualMode) > pluginStorageAccessModeRank(declaredMode) {
		return fmt.Errorf(
			"plugin action declared storage access %s but used %s access",
			declaredMode,
			actualMode,
		)
	}
	return nil
}

func (s *PluginManagerService) loadPluginStorageSnapshot(pluginID uint) (map[string]string, error) {
	out := make(map[string]string)
	if s == nil || s.db == nil || pluginID == 0 {
		return out, nil
	}

	if cached, ok := s.getCachedPluginStorageSnapshot(pluginID); ok {
		return cached, nil
	}

	loaded, err := s.queryPluginStorageSnapshot(pluginID)
	if err != nil {
		return nil, err
	}
	s.setCachedPluginStorageSnapshot(pluginID, loaded)
	return cloneStringMap(loaded), nil
}

func (s *PluginManagerService) queryPluginStorageSnapshot(pluginID uint) (map[string]string, error) {
	out := make(map[string]string)
	if s == nil || s.db == nil || pluginID == 0 {
		return out, nil
	}

	var rows []models.PluginStorageEntry
	if err := s.db.Where("plugin_id = ?", pluginID).
		Order(clause.OrderByColumn{Column: clause.Column{Name: "key"}}).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	for _, row := range rows {
		out[row.Key] = row.Value
	}
	return out, nil
}

func (s *PluginManagerService) replacePluginStorageSnapshot(pluginID uint, data map[string]string) error {
	if s == nil || s.db == nil || pluginID == 0 {
		return nil
	}
	normalizedData, err := s.normalizeAndValidatePluginStorageSnapshot(data)
	if err != nil {
		return err
	}

	currentSnapshot, ok := s.getCachedPluginStorageSnapshot(pluginID)
	if !ok {
		currentSnapshot, err = s.queryPluginStorageSnapshot(pluginID)
		if err != nil {
			return err
		}
	}

	delta := buildPluginStorageDelta(pluginID, currentSnapshot, normalizedData, time.Now().UTC())
	if len(delta.removedKeys) == 0 && len(delta.upserts) == 0 {
		s.setCachedPluginStorageSnapshot(pluginID, normalizedData)
		return nil
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if len(delta.removedKeys) > 0 {
			if err := tx.Where("plugin_id = ? AND key IN ?", pluginID, delta.removedKeys).
				Delete(&models.PluginStorageEntry{}).Error; err != nil {
				return err
			}
		}
		if len(delta.upserts) == 0 {
			return nil
		}
		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "plugin_id"},
				{Name: "key"},
			},
			DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
		}).Create(&delta.upserts).Error
	}); err != nil {
		return err
	}

	s.setCachedPluginStorageSnapshot(pluginID, normalizedData)
	return nil
}

func (s *PluginManagerService) clearPluginStorage(pluginID uint) error {
	if s == nil || s.db == nil || pluginID == 0 {
		return nil
	}
	if err := s.db.Where("plugin_id = ?", pluginID).Delete(&models.PluginStorageEntry{}).Error; err != nil {
		return err
	}
	s.setCachedPluginStorageSnapshot(pluginID, map[string]string{})
	return nil
}

func (s *PluginManagerService) getCachedPluginStorageSnapshot(pluginID uint) (map[string]string, bool) {
	if s == nil || pluginID == 0 {
		return nil, false
	}

	s.storageMu.RLock()
	defer s.storageMu.RUnlock()

	if s.storageSnapshots == nil {
		return nil, false
	}
	snapshot, exists := s.storageSnapshots[pluginID]
	if !exists {
		return nil, false
	}
	return cloneStringMap(snapshot), true
}

func (s *PluginManagerService) setCachedPluginStorageSnapshot(pluginID uint, snapshot map[string]string) {
	if s == nil || pluginID == 0 {
		return
	}

	s.storageMu.Lock()
	defer s.storageMu.Unlock()

	if s.storageSnapshots == nil {
		s.storageSnapshots = make(map[uint]map[string]string)
	}
	s.storageSnapshots[pluginID] = cloneStringMap(snapshot)
}

func (s *PluginManagerService) invalidatePluginStorageSnapshot(pluginID uint) {
	if s == nil || pluginID == 0 {
		return
	}

	s.storageMu.Lock()
	defer s.storageMu.Unlock()

	if s.storageSnapshots == nil {
		return
	}
	delete(s.storageSnapshots, pluginID)
}

func buildPluginStorageDelta(
	pluginID uint,
	current map[string]string,
	next map[string]string,
	now time.Time,
) pluginStorageDelta {
	delta := pluginStorageDelta{
		removedKeys: make([]string, 0),
		upserts:     make([]models.PluginStorageEntry, 0),
	}

	for key := range current {
		if _, exists := next[key]; exists {
			continue
		}
		delta.removedKeys = append(delta.removedKeys, key)
	}
	for key, value := range next {
		if currentValue, exists := current[key]; exists && currentValue == value {
			continue
		}
		delta.upserts = append(delta.upserts, models.PluginStorageEntry{
			PluginID:  pluginID,
			Key:       key,
			Value:     value,
			CreatedAt: now,
			UpdatedAt: now,
		})
	}

	sort.Strings(delta.removedKeys)
	sort.Slice(delta.upserts, func(i, j int) bool {
		return delta.upserts[i].Key < delta.upserts[j].Key
	})
	return delta
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func (s *PluginManagerService) pluginStorageQuota() pluginStorageQuota {
	quota := pluginStorageQuota{
		MaxKeys:       defaultPluginStorageMaxKeys,
		MaxTotalBytes: defaultPluginStorageMaxTotalBytes,
		MaxValueBytes: defaultPluginStorageMaxValueBytes,
		MaxKeyBytes:   defaultPluginStorageMaxKeyBytes,
	}
	if s == nil || s.cfg == nil {
		return quota
	}
	if s.cfg.Plugin.JSStorageMaxKeys > 0 {
		quota.MaxKeys = s.cfg.Plugin.JSStorageMaxKeys
	}
	if s.cfg.Plugin.JSStorageMaxTotalBytes > 0 {
		quota.MaxTotalBytes = s.cfg.Plugin.JSStorageMaxTotalBytes
	}
	if s.cfg.Plugin.JSStorageMaxValueBytes > 0 {
		quota.MaxValueBytes = s.cfg.Plugin.JSStorageMaxValueBytes
	}
	if quota.MaxValueBytes > quota.MaxTotalBytes {
		quota.MaxValueBytes = quota.MaxTotalBytes
	}
	return quota
}

func (s *PluginManagerService) normalizeAndValidatePluginStorageSnapshot(data map[string]string) (map[string]string, error) {
	quota := s.pluginStorageQuota()
	if len(data) == 0 {
		return map[string]string{}, nil
	}

	normalized := make(map[string]string, len(data))
	for rawKey, value := range data {
		key := strings.TrimSpace(rawKey)
		if key == "" {
			continue
		}
		if _, exists := normalized[key]; exists {
			return nil, fmt.Errorf("plugin storage contains duplicated normalized key %q", key)
		}
		if quota.MaxKeyBytes > 0 && len(key) > quota.MaxKeyBytes {
			return nil, fmt.Errorf("plugin storage key %q exceeds max key bytes %d", key, quota.MaxKeyBytes)
		}
		if quota.MaxValueBytes > 0 && int64(len(value)) > quota.MaxValueBytes {
			return nil, fmt.Errorf("plugin storage key %q exceeds max value bytes %d", key, quota.MaxValueBytes)
		}
		normalized[key] = value
	}

	if quota.MaxKeys > 0 && len(normalized) > quota.MaxKeys {
		return nil, fmt.Errorf("plugin storage exceeds max keys: %d > %d", len(normalized), quota.MaxKeys)
	}

	var totalBytes int64
	for key, value := range normalized {
		totalBytes += int64(len(key) + len(value))
	}
	if quota.MaxTotalBytes > 0 && totalBytes > quota.MaxTotalBytes {
		return nil, fmt.Errorf("plugin storage exceeds max total bytes: %d > %d", totalBytes, quota.MaxTotalBytes)
	}

	return normalized, nil
}
