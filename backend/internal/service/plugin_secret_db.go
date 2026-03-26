package service

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"auralogic/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	defaultPluginSecretMaxValueBytes = int64(64 * 1024)
)

type PluginSecretMeta struct {
	Key        string    `json:"key"`
	Configured bool      `json:"configured"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (s *PluginManagerService) LoadPluginSecretSnapshot(pluginID uint) (map[string]string, error) {
	return s.loadPluginSecretSnapshot(pluginID)
}

func (s *PluginManagerService) loadPluginSecretSnapshot(pluginID uint) (map[string]string, error) {
	out := make(map[string]string)
	if s == nil || s.db == nil || pluginID == 0 {
		return out, nil
	}

	if cached, ok := s.getCachedPluginSecretSnapshot(pluginID); ok {
		return cached, nil
	}

	loaded, err := s.queryPluginSecretSnapshot(pluginID)
	if err != nil {
		return nil, err
	}
	s.setCachedPluginSecretSnapshot(pluginID, loaded)
	return cloneStringMap(loaded), nil
}

func (s *PluginManagerService) queryPluginSecretSnapshot(pluginID uint) (map[string]string, error) {
	out := make(map[string]string)
	if s == nil || s.db == nil || pluginID == 0 {
		return out, nil
	}

	var rows []models.PluginSecretEntry
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

func (s *PluginManagerService) ListPluginSecretMeta(pluginID uint) ([]PluginSecretMeta, error) {
	if s == nil || s.db == nil || pluginID == 0 {
		return []PluginSecretMeta{}, nil
	}

	var rows []models.PluginSecretEntry
	if err := s.db.Select("key", "updated_at").
		Where("plugin_id = ?", pluginID).
		Order(clause.OrderByColumn{Column: clause.Column{Name: "key"}}).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	items := make([]PluginSecretMeta, 0, len(rows))
	for _, row := range rows {
		items = append(items, PluginSecretMeta{
			Key:        row.Key,
			Configured: true,
			UpdatedAt:  row.UpdatedAt,
		})
	}
	return items, nil
}

func (s *PluginManagerService) ApplyPluginSecretPatch(pluginID uint, upserts map[string]string, deleteKeys []string) error {
	if s == nil || s.db == nil || pluginID == 0 {
		return nil
	}

	normalizedUpserts, normalizedDeleteKeys, err := normalizePluginSecretPatch(upserts, deleteKeys)
	if err != nil {
		return err
	}
	if len(normalizedUpserts) == 0 && len(normalizedDeleteKeys) == 0 {
		return nil
	}

	currentSnapshot, ok := s.getCachedPluginSecretSnapshot(pluginID)
	if !ok {
		currentSnapshot, err = s.queryPluginSecretSnapshot(pluginID)
		if err != nil {
			return err
		}
	}

	now := time.Now().UTC()
	rows := make([]models.PluginSecretEntry, 0, len(normalizedUpserts))
	for key, value := range normalizedUpserts {
		rows = append(rows, models.PluginSecretEntry{
			PluginID:  pluginID,
			Key:       key,
			Value:     value,
			CreatedAt: now,
			UpdatedAt: now,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Key < rows[j].Key
	})

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if len(normalizedDeleteKeys) > 0 {
			if err := tx.Where("plugin_id = ? AND key IN ?", pluginID, normalizedDeleteKeys).
				Delete(&models.PluginSecretEntry{}).Error; err != nil {
				return err
			}
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "plugin_id"},
				{Name: "key"},
			},
			DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
		}).Create(&rows).Error
	}); err != nil {
		return err
	}

	nextSnapshot := cloneStringMap(currentSnapshot)
	if nextSnapshot == nil {
		nextSnapshot = map[string]string{}
	}
	for _, key := range normalizedDeleteKeys {
		delete(nextSnapshot, key)
	}
	for key, value := range normalizedUpserts {
		nextSnapshot[key] = value
	}
	s.setCachedPluginSecretSnapshot(pluginID, nextSnapshot)
	return nil
}

func normalizePluginSecretPatch(upserts map[string]string, deleteKeys []string) (map[string]string, []string, error) {
	normalizedUpserts := make(map[string]string)
	for key, value := range upserts {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "" {
			continue
		}
		if len(normalizedKey) > defaultPluginStorageMaxKeyBytes {
			return nil, nil, fmt.Errorf("plugin secret key %q exceeds %d bytes", normalizedKey, defaultPluginStorageMaxKeyBytes)
		}
		if int64(len(value)) > defaultPluginSecretMaxValueBytes {
			return nil, nil, fmt.Errorf("plugin secret %q exceeds %d bytes", normalizedKey, defaultPluginSecretMaxValueBytes)
		}
		normalizedUpserts[normalizedKey] = value
	}

	seenDelete := make(map[string]struct{})
	normalizedDeleteKeys := make([]string, 0, len(deleteKeys))
	for _, key := range deleteKeys {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "" {
			continue
		}
		if len(normalizedKey) > defaultPluginStorageMaxKeyBytes {
			return nil, nil, fmt.Errorf("plugin secret key %q exceeds %d bytes", normalizedKey, defaultPluginStorageMaxKeyBytes)
		}
		delete(normalizedUpserts, normalizedKey)
		if _, exists := seenDelete[normalizedKey]; exists {
			continue
		}
		seenDelete[normalizedKey] = struct{}{}
		normalizedDeleteKeys = append(normalizedDeleteKeys, normalizedKey)
	}
	sort.Strings(normalizedDeleteKeys)
	return normalizedUpserts, normalizedDeleteKeys, nil
}

func (s *PluginManagerService) getCachedPluginSecretSnapshot(pluginID uint) (map[string]string, bool) {
	if s == nil || pluginID == 0 {
		return nil, false
	}

	s.secretMu.RLock()
	defer s.secretMu.RUnlock()

	if s.secretSnapshots == nil {
		return nil, false
	}
	snapshot, exists := s.secretSnapshots[pluginID]
	if !exists {
		return nil, false
	}
	return cloneStringMap(snapshot), true
}

func (s *PluginManagerService) setCachedPluginSecretSnapshot(pluginID uint, snapshot map[string]string) {
	if s == nil || pluginID == 0 {
		return
	}

	s.secretMu.Lock()
	defer s.secretMu.Unlock()

	if s.secretSnapshots == nil {
		s.secretSnapshots = make(map[uint]map[string]string)
	}
	s.secretSnapshots[pluginID] = cloneStringMap(snapshot)
}

func (s *PluginManagerService) invalidatePluginSecretSnapshot(pluginID uint) {
	if s == nil || pluginID == 0 {
		return
	}

	s.secretMu.Lock()
	defer s.secretMu.Unlock()

	if s.secretSnapshots == nil {
		return
	}
	delete(s.secretSnapshots, pluginID)
}
