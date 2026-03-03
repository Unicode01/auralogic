package service

import (
	"errors"
	"sync"
	"time"

	"auralogic/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	virtualInventoryStorageLocks   = make(map[uint]*sync.RWMutex)
	virtualInventoryStorageLocksMu sync.Mutex
)

func getVirtualInventoryStorageLock(virtualInventoryID uint) *sync.RWMutex {
	virtualInventoryStorageLocksMu.Lock()
	defer virtualInventoryStorageLocksMu.Unlock()

	if lock, ok := virtualInventoryStorageLocks[virtualInventoryID]; ok {
		return lock
	}
	lock := &sync.RWMutex{}
	virtualInventoryStorageLocks[virtualInventoryID] = lock
	return lock
}

func releaseVirtualInventoryStorageLock(virtualInventoryID uint) {
	virtualInventoryStorageLocksMu.Lock()
	defer virtualInventoryStorageLocksMu.Unlock()
	delete(virtualInventoryStorageLocks, virtualInventoryID)
}

func (s *ScriptDeliveryService) storageGetValue(virtualInventoryID uint, key string) (string, bool, error) {
	var entry models.VirtualInventoryStorageEntry
	err := s.db.Where(map[string]interface{}{
		"virtual_inventory_id": virtualInventoryID,
		"key":                  key,
	}).Take(&entry).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return entry.Value, true, nil
}

func (s *ScriptDeliveryService) storageSetValue(virtualInventoryID uint, key, value string) error {
	now := time.Now().UTC()
	entry := models.VirtualInventoryStorageEntry{
		VirtualInventoryID: virtualInventoryID,
		Key:                key,
		Value:              value,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	return s.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "virtual_inventory_id"},
			{Name: "key"},
		},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"value":      value,
			"updated_at": now,
		}),
	}).Create(&entry).Error
}

func (s *ScriptDeliveryService) storageDeleteKey(virtualInventoryID uint, key string) error {
	return s.db.Where(map[string]interface{}{
		"virtual_inventory_id": virtualInventoryID,
		"key":                  key,
	}).Delete(&models.VirtualInventoryStorageEntry{}).Error
}

func (s *ScriptDeliveryService) storageListKeys(virtualInventoryID uint) ([]string, error) {
	keys := make([]string, 0)
	err := s.db.Model(&models.VirtualInventoryStorageEntry{}).
		Where("virtual_inventory_id = ?", virtualInventoryID).
		Order(clause.OrderByColumn{Column: clause.Column{Name: "key"}}).
		Pluck("key", &keys).Error
	if err != nil {
		return nil, err
	}
	return keys, nil
}

func (s *ScriptDeliveryService) storageClearAll(virtualInventoryID uint) error {
	return s.db.Where("virtual_inventory_id = ?", virtualInventoryID).
		Delete(&models.VirtualInventoryStorageEntry{}).Error
}
