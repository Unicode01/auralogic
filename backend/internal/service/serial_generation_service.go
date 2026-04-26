package service

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"auralogic/internal/models"
	"auralogic/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	defaultSerialGenerationPollInterval = 2 * time.Second
	defaultSerialGenerationMaxRetries   = 6
)

// SerialGenerationService 持久化处理订单序列号后台生成与重试。
type SerialGenerationService struct {
	db            *gorm.DB
	serialService *SerialService
	orderRepo     *repository.OrderRepository

	lifecycleMu sync.Mutex
	running     bool
	stopChan    chan struct{}
	doneChan    chan struct{}
	wakeupChan  chan struct{}

	pollInterval time.Duration
	maxRetries   int
}

func NewSerialGenerationService(db *gorm.DB, serialService *SerialService) *SerialGenerationService {
	return &SerialGenerationService{
		db:            db,
		serialService: serialService,
		orderRepo:     repository.NewOrderRepository(db),
		wakeupChan:    make(chan struct{}, 1),
		pollInterval:  defaultSerialGenerationPollInterval,
		maxRetries:    defaultSerialGenerationMaxRetries,
	}
}

func (s *SerialGenerationService) Start() {
	if s == nil || s.db == nil || s.serialService == nil {
		return
	}

	s.lifecycleMu.Lock()
	if s.running {
		s.lifecycleMu.Unlock()
		return
	}
	stopChan := make(chan struct{})
	doneChan := make(chan struct{})
	s.stopChan = stopChan
	s.doneChan = doneChan
	s.running = true
	s.lifecycleMu.Unlock()

	if err := s.recoverTasks(); err != nil {
		log.Printf("serial generation recover tasks failed: %v", err)
	}
	go func() {
		defer close(doneChan)
		runBackgroundServiceWithStopChan("serial_generation.runLoop", stopChan, s.runLoop)
	}()
	s.NotifyOrderQueued(0)
}

func (s *SerialGenerationService) Stop() {
	if s == nil {
		return
	}

	s.lifecycleMu.Lock()
	if !s.running {
		s.lifecycleMu.Unlock()
		return
	}
	stopChan := s.stopChan
	doneChan := s.doneChan
	s.stopChan = nil
	s.doneChan = nil
	s.running = false
	close(stopChan)
	s.lifecycleMu.Unlock()

	<-doneChan
}

func (s *SerialGenerationService) NotifyOrderQueued(orderID uint) {
	if s == nil {
		return
	}
	select {
	case s.wakeupChan <- struct{}{}:
	default:
	}
}

func (s *SerialGenerationService) EnqueueOrderTx(tx *gorm.DB, orderID uint) error {
	if s == nil || tx == nil {
		return nil
	}
	if orderID == 0 {
		return fmt.Errorf("order id is required")
	}

	now := models.NowFunc().UTC()
	record := models.SerialGenerationTask{
		OrderID:    orderID,
		Status:     models.SerialGenerationStatusQueued,
		RetryCount: 0,
		NextRunAt:  now,
	}

	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "order_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"status":       models.SerialGenerationStatusQueued,
			"retry_count":  0,
			"last_error":   "",
			"next_run_at":  now,
			"started_at":   nil,
			"completed_at": nil,
			"updated_at":   now,
		}),
	}).Create(&record).Error
}

func (s *SerialGenerationService) CancelOrder(orderID uint) error {
	if s == nil || s.db == nil || orderID == 0 {
		return nil
	}

	now := models.NowFunc().UTC()
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.SerialGenerationTask{}).
			Where("order_id = ? AND status <> ?", orderID, models.SerialGenerationStatusCompleted).
			Updates(map[string]interface{}{
				"status":       models.SerialGenerationStatusCancelled,
				"last_error":   "",
				"next_run_at":  now,
				"started_at":   nil,
				"completed_at": now,
			}).Error; err != nil {
			return err
		}
		return tx.Model(&models.Order{}).
			Where("id = ?", orderID).
			Updates(map[string]interface{}{
				"serial_generation_status": models.SerialGenerationStatusCancelled,
				"serial_generation_error":  "",
				"serial_generated_at":      nil,
			}).Error
	})
}

func (s *SerialGenerationService) DeleteOrderTask(orderID uint) error {
	if s == nil || s.db == nil || orderID == 0 {
		return nil
	}
	return s.db.Where("order_id = ?", orderID).Delete(&models.SerialGenerationTask{}).Error
}

func (s *SerialGenerationService) recoverTasks() error {
	now := models.NowFunc().UTC()
	return s.db.Model(&models.SerialGenerationTask{}).
		Where("status = ?", models.SerialGenerationStatusProcessing).
		Updates(map[string]interface{}{
			"status":      models.SerialGenerationStatusQueued,
			"next_run_at": now,
			"started_at":  nil,
			"last_error":  "",
		}).Error
}

func (s *SerialGenerationService) runLoop(stopChan <-chan struct{}) {
	for {
		select {
		case <-stopChan:
			return
		default:
		}

		if err := s.processReadyTasks(stopChan); err != nil {
			log.Printf("serial generation worker failed: %v", err)
		}

		timer := time.NewTimer(s.pollInterval)
		select {
		case <-stopChan:
			timer.Stop()
			return
		case <-s.wakeupChan:
			timer.Stop()
		case <-timer.C:
		}
	}
}

func (s *SerialGenerationService) processReadyTasks(stopChan <-chan struct{}) error {
	for {
		select {
		case <-stopChan:
			return nil
		default:
		}

		task, err := s.claimNextTask(models.NowFunc().UTC())
		if err != nil {
			return err
		}
		if task == nil {
			return nil
		}

		if err := s.processTask(task); err != nil {
			log.Printf("serial generation task failed: order_id=%d err=%v", task.OrderID, err)
		}
	}
}

func (s *SerialGenerationService) claimNextTask(now time.Time) (*models.SerialGenerationTask, error) {
	for attempt := 0; attempt < 8; attempt++ {
		var claimed *models.SerialGenerationTask
		err := s.db.Transaction(func(tx *gorm.DB) error {
			var task models.SerialGenerationTask
			if err := tx.Where("status = ? AND next_run_at <= ?", models.SerialGenerationStatusQueued, now).
				Order("next_run_at ASC, id ASC").
				First(&task).Error; err != nil {
				return err
			}

			result := tx.Model(&models.SerialGenerationTask{}).
				Where("id = ? AND status = ?", task.ID, models.SerialGenerationStatusQueued).
				Updates(map[string]interface{}{
					"status":     models.SerialGenerationStatusProcessing,
					"started_at": now,
					"last_error": "",
				})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return nil
			}

			if err := tx.Model(&models.Order{}).
				Where("id = ?", task.OrderID).
				Updates(map[string]interface{}{
					"serial_generation_status": models.SerialGenerationStatusProcessing,
					"serial_generation_error":  "",
				}).Error; err != nil {
				return err
			}

			task.Status = models.SerialGenerationStatusProcessing
			task.StartedAt = &now
			claimed = &task
			return nil
		})
		switch {
		case err == nil && claimed != nil:
			return claimed, nil
		case err == nil:
			continue
		case errors.Is(err, gorm.ErrRecordNotFound):
			return nil, nil
		default:
			return nil, err
		}
	}

	return nil, nil
}

func (s *SerialGenerationService) processTask(task *models.SerialGenerationTask) error {
	if s == nil || task == nil {
		return nil
	}

	var (
		createdSerials []models.ProductSerial
		orderUserID    *uint
	)
	err := s.db.Transaction(func(tx *gorm.DB) error {
		order, err := s.orderRepo.FindByIDForUpdate(tx, task.OrderID)
		if err != nil {
			return err
		}
		if order.UserID != nil {
			userID := *order.UserID
			orderUserID = &userID
		}

		now := models.NowFunc().UTC()
		if shouldSkipSerialGeneration(order) {
			if err := tx.Model(&models.SerialGenerationTask{}).
				Where("id = ?", task.ID).
				Updates(map[string]interface{}{
					"status":       models.SerialGenerationStatusCancelled,
					"last_error":   "",
					"completed_at": now,
				}).Error; err != nil {
				return err
			}
			return tx.Model(&models.Order{}).
				Where("id = ?", order.ID).
				Updates(map[string]interface{}{
					"serial_generation_status": models.SerialGenerationStatusCancelled,
					"serial_generation_error":  "",
					"serial_generated_at":      nil,
				}).Error
		}

		created, hasEligible, createErr := s.serialService.createMissingOrderSerialsTx(tx, order)
		if createErr != nil {
			return createErr
		}
		createdSerials = created

		nextStatus := models.SerialGenerationStatusCompleted
		orderUpdates := map[string]interface{}{
			"serial_generation_status": nextStatus,
			"serial_generation_error":  "",
		}
		if hasEligible {
			orderUpdates["serial_generated_at"] = now
		} else {
			nextStatus = models.SerialGenerationStatusNotRequired
			orderUpdates["serial_generation_status"] = nextStatus
			orderUpdates["serial_generated_at"] = nil
		}
		if err := tx.Model(&models.Order{}).Where("id = ?", order.ID).Updates(orderUpdates).Error; err != nil {
			return err
		}

		return tx.Model(&models.SerialGenerationTask{}).
			Where("id = ?", task.ID).
			Updates(map[string]interface{}{
				"status":       models.SerialGenerationStatusCompleted,
				"last_error":   "",
				"completed_at": now,
			}).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return s.DeleteOrderTask(task.OrderID)
	}
	if err != nil {
		return s.handleTaskFailure(task, err)
	}

	if len(createdSerials) > 0 {
		s.serialService.emitSerialCreateAfterHook(createdSerials, "serial_generation_worker", orderUserID, task.OrderID)
	}
	return nil
}

func (s *SerialGenerationService) handleTaskFailure(task *models.SerialGenerationTask, taskErr error) error {
	if s == nil || task == nil || taskErr == nil {
		return taskErr
	}

	retryCount := task.RetryCount + 1
	now := models.NowFunc().UTC()
	taskStatus := models.SerialGenerationStatusQueued
	if retryCount >= s.maxRetries {
		taskStatus = models.SerialGenerationStatusFailed
	}
	nextRunAt := now.Add(s.retryBackoff(retryCount))
	if taskStatus == models.SerialGenerationStatusFailed {
		nextRunAt = now
	}

	updateErr := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.SerialGenerationTask{}).
			Where("id = ?", task.ID).
			Updates(map[string]interface{}{
				"status":      taskStatus,
				"retry_count": retryCount,
				"last_error":  truncateSerialGenerationError(taskErr.Error()),
				"next_run_at": nextRunAt,
				"started_at":  nil,
			}).Error; err != nil {
			return err
		}
		return tx.Model(&models.Order{}).
			Where("id = ?", task.OrderID).
			Updates(map[string]interface{}{
				"serial_generation_status": models.SerialGenerationStatusFailed,
				"serial_generation_error":  truncateSerialGenerationError(taskErr.Error()),
			}).Error
	})
	if updateErr != nil {
		return fmt.Errorf("mark serial generation task failed: %w", updateErr)
	}

	if taskStatus == models.SerialGenerationStatusQueued {
		s.NotifyOrderQueued(task.OrderID)
	}
	return taskErr
}

func (s *SerialGenerationService) retryBackoff(retryCount int) time.Duration {
	switch {
	case retryCount <= 1:
		return 5 * time.Second
	case retryCount == 2:
		return 15 * time.Second
	case retryCount == 3:
		return 30 * time.Second
	default:
		return time.Minute
	}
}

func truncateSerialGenerationError(message string) string {
	trimmed := message
	if len(trimmed) <= 500 {
		return trimmed
	}
	return trimmed[:500]
}

func shouldSkipSerialGeneration(order *models.Order) bool {
	if order == nil {
		return true
	}

	switch order.Status {
	case models.OrderStatusPending, models.OrderStatusShipped, models.OrderStatusCompleted:
		return false
	default:
		return true
	}
}
