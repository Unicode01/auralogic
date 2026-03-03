package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"auralogic/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const legacyPaymentStorageDir = "storage/payments"

var (
	paymentStorageLocks       = make(map[uint]*sync.RWMutex)
	paymentStorageLocksMu     sync.Mutex
	paymentStorageMigrateOnce sync.Once
)

func getPaymentStorageLock(paymentMethodID uint) *sync.RWMutex {
	paymentStorageLocksMu.Lock()
	defer paymentStorageLocksMu.Unlock()

	if lock, ok := paymentStorageLocks[paymentMethodID]; ok {
		return lock
	}
	lock := &sync.RWMutex{}
	paymentStorageLocks[paymentMethodID] = lock
	return lock
}

func releasePaymentStorageLock(paymentMethodID uint) {
	paymentStorageLocksMu.Lock()
	defer paymentStorageLocksMu.Unlock()
	delete(paymentStorageLocks, paymentMethodID)
}

func (s *JSRuntimeService) ensurePaymentStorageMigrated() {
	if s == nil || s.db == nil {
		return
	}
	// Run migration only after table migration is complete.
	if !s.db.Migrator().HasTable(&models.PaymentMethodStorageEntry{}) {
		return
	}

	paymentStorageMigrateOnce.Do(func() {
		if err := s.migrateLegacyPaymentStorageFiles(); err != nil {
			log.Printf("Warning: failed to migrate legacy payment storage files: %v", err)
		}
	})
}

func (s *JSRuntimeService) migrateLegacyPaymentStorageFiles() error {
	if s == nil || s.db == nil {
		return nil
	}

	entries, err := os.ReadDir(legacyPaymentStorageDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		paymentMethodID, ok := parseLegacyPaymentStorageFilename(entry.Name())
		if !ok {
			continue
		}

		path := filepath.Join(legacyPaymentStorageDir, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}

		storage := make(map[string]string)
		if len(strings.TrimSpace(string(raw))) > 0 {
			if err := json.Unmarshal(raw, &storage); err != nil {
				failedPath := path + ".migration_failed"
				if renameErr := os.Rename(path, failedPath); renameErr != nil {
					log.Printf("Warning: failed to rename invalid legacy storage file %s: %v", path, renameErr)
				} else {
					log.Printf("Warning: invalid legacy payment storage file moved to %s: %v", failedPath, err)
				}
				continue
			}
		}

		if err := s.storageInsertMissing(paymentMethodID, storage); err != nil {
			return fmt.Errorf("migrate %s: %w", path, err)
		}

		migratedPath := path + ".migrated"
		if err := os.Rename(path, migratedPath); err != nil {
			return fmt.Errorf("rename %s to %s: %w", path, migratedPath, err)
		}
		log.Printf("Migrated legacy payment storage %s -> %s", path, migratedPath)
	}

	return nil
}

func parseLegacyPaymentStorageFilename(name string) (uint, bool) {
	if !strings.HasSuffix(name, ".json") {
		return 0, false
	}
	idPart := strings.TrimSuffix(name, ".json")
	if idPart == "" {
		return 0, false
	}

	id64, err := strconv.ParseUint(idPart, 10, 64)
	if err != nil || id64 == 0 {
		return 0, false
	}
	return uint(id64), true
}

func (s *JSRuntimeService) storageGetValue(paymentMethodID uint, key string) (string, bool, error) {
	var entry models.PaymentMethodStorageEntry
	err := s.db.Where(map[string]interface{}{
		"payment_method_id": paymentMethodID,
		"key":               key,
	}).Take(&entry).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return entry.Value, true, nil
}

func (s *JSRuntimeService) storageSetValue(paymentMethodID uint, key, value string) error {
	now := time.Now().UTC()
	entry := models.PaymentMethodStorageEntry{
		PaymentMethodID: paymentMethodID,
		Key:             key,
		Value:           value,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	return s.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "payment_method_id"},
			{Name: "key"},
		},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"value":      value,
			"updated_at": now,
		}),
	}).Create(&entry).Error
}

func (s *JSRuntimeService) storageDeleteKey(paymentMethodID uint, key string) error {
	return s.db.Where(map[string]interface{}{
		"payment_method_id": paymentMethodID,
		"key":               key,
	}).Delete(&models.PaymentMethodStorageEntry{}).Error
}

func (s *JSRuntimeService) storageListKeys(paymentMethodID uint) ([]string, error) {
	keys := make([]string, 0)
	err := s.db.Model(&models.PaymentMethodStorageEntry{}).
		Where("payment_method_id = ?", paymentMethodID).
		Order(clause.OrderByColumn{Column: clause.Column{Name: "key"}}).
		Pluck("key", &keys).Error
	if err != nil {
		return nil, err
	}
	return keys, nil
}

func (s *JSRuntimeService) storageClearAll(paymentMethodID uint) error {
	return s.db.Where("payment_method_id = ?", paymentMethodID).
		Delete(&models.PaymentMethodStorageEntry{}).Error
}

func (s *JSRuntimeService) storageInsertMissing(paymentMethodID uint, data map[string]string) error {
	if len(data) == 0 {
		return nil
	}

	now := time.Now().UTC()
	rows := make([]models.PaymentMethodStorageEntry, 0, len(data))
	for k, v := range data {
		rows = append(rows, models.PaymentMethodStorageEntry{
			PaymentMethodID: paymentMethodID,
			Key:             k,
			Value:           v,
			CreatedAt:       now,
			UpdatedAt:       now,
		})
	}

	return s.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "payment_method_id"},
			{Name: "key"},
		},
		DoNothing: true,
	}).Create(&rows).Error
}
