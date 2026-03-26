package dbutil

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

// LockForUpdate acquires a row-level lock for a model inside an existing transaction.
// SQLite does not support FOR UPDATE, so we use a no-op UPDATE on updated_at to force
// a write lock early and serialize conflicting writers in tests and development.
func LockForUpdate(tx *gorm.DB, model interface{}, where interface{}, args ...interface{}) error {
	if tx == nil {
		return fmt.Errorf("transaction is required")
	}

	if tx.Dialector.Name() == "sqlite" {
		lockTx := tx.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
		var lastErr error
		for attempt := 0; attempt < 200; attempt++ {
			result := lockTx.Model(model).
				Where(where, args...).
				UpdateColumn("updated_at", gorm.Expr("updated_at"))
			if result.Error == nil {
				if result.RowsAffected == 0 {
					return gorm.ErrRecordNotFound
				}
				return nil
			}
			lastErr = result.Error
			if !isSQLiteBusyError(result.Error) {
				return result.Error
			}
			time.Sleep(25 * time.Millisecond)
		}
		return lastErr
	}

	var marker struct {
		ID uint
	}
	return tx.Model(model).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id").
		Where(where, args...).
		Take(&marker).Error
}

func isSQLiteBusyError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "database is locked") ||
		strings.Contains(msg, "database table is locked") ||
		strings.Contains(msg, "database is busy")
}
