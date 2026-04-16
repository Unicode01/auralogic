package service

import (
	"log"
	"time"

	"auralogic/internal/models"
)

const (
	defaultPluginExecutionLogRetentionDays   = 90
	defaultPluginExecutionRetentionBatchSize = 500
)

func (s *PluginManagerService) executionRetentionLoop(stopChan <-chan struct{}) {
	if s == nil || s.db == nil {
		return
	}

	s.runPluginExecutionRetentionCleanup("startup")

	interval := s.executionRetentionTick
	if interval <= 0 {
		interval = 6 * time.Hour
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-stopChan:
			return
		case <-ticker.C:
			s.runPluginExecutionRetentionCleanup("scheduled")
		}
	}
}

func (s *PluginManagerService) runPluginExecutionRetentionCleanup(trigger string) {
	deleted, err := s.CleanupExpiredPluginExecutionsNow()
	if err != nil {
		log.Printf("Plugin execution retention cleanup failed (%s): %v", trigger, err)
		return
	}
	if deleted > 0 {
		log.Printf("Plugin execution retention cleanup removed %d rows (%s)", deleted, trigger)
	}
}

func (s *PluginManagerService) CleanupExpiredPluginExecutionsNow() (int64, error) {
	return s.cleanupExpiredPluginExecutionsAt(time.Now().UTC())
}

func (s *PluginManagerService) cleanupExpiredPluginExecutionsAt(now time.Time) (int64, error) {
	cutoff, enabled := s.resolvePluginExecutionRetentionCutoff(now)
	if !enabled {
		return 0, nil
	}
	return s.cleanupExpiredPluginExecutionsBefore(cutoff, defaultPluginExecutionRetentionBatchSize)
}

func (s *PluginManagerService) resolvePluginExecutionRetentionCutoff(now time.Time) (time.Time, bool) {
	if s == nil {
		return time.Time{}, false
	}

	retentionDays := s.getPluginPlatformConfig().Execution.ExecutionLogRetentionDays
	if retentionDays == 0 {
		retentionDays = defaultPluginExecutionLogRetentionDays
	}
	if retentionDays < 0 {
		return time.Time{}, false
	}

	return now.UTC().Add(-time.Duration(retentionDays) * 24 * time.Hour), true
}

func (s *PluginManagerService) cleanupExpiredPluginExecutionsBefore(
	cutoff time.Time,
	batchSize int,
) (int64, error) {
	if s == nil || s.db == nil {
		return 0, nil
	}
	if batchSize <= 0 {
		batchSize = defaultPluginExecutionRetentionBatchSize
	}

	var totalDeleted int64
	for {
		if s.stopChan != nil {
			select {
			case <-s.stopChan:
				return totalDeleted, nil
			default:
			}
		}

		ids := make([]uint, 0, batchSize)
		if err := s.db.Model(&models.PluginExecution{}).
			Where("created_at < ?", cutoff.UTC()).
			Order("created_at ASC").
			Limit(batchSize).
			Pluck("id", &ids).Error; err != nil {
			return totalDeleted, err
		}
		if len(ids) == 0 {
			return totalDeleted, nil
		}

		result := s.db.Where("id IN ?", ids).Delete(&models.PluginExecution{})
		if result.Error != nil {
			return totalDeleted, result.Error
		}

		totalDeleted += result.RowsAffected
		if len(ids) < batchSize || result.RowsAffected == 0 {
			return totalDeleted, nil
		}
	}
}
