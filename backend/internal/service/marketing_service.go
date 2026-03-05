package service

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"auralogic/internal/models"
	"auralogic/internal/pkg/cache"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

const (
	marketingQueueKey     = "marketing:queue"
	marketingLockKeyFmt   = "marketing:batch:lock:%d"
	marketingLockDuration = 2 * time.Hour
)

type MarketingService struct {
	db           *gorm.DB
	emailService *EmailService
	smsService   *SMSService
}

func NewMarketingService(db *gorm.DB, emailService *EmailService, smsService *SMSService) *MarketingService {
	return &MarketingService{
		db:           db,
		emailService: emailService,
		smsService:   smsService,
	}
}

func (s *MarketingService) EnqueueBatch(batchID uint) error {
	if batchID == 0 {
		return fmt.Errorf("invalid batch id")
	}
	if cache.RedisClient == nil {
		return fmt.Errorf("redis client is not initialized")
	}
	return cache.RedisClient.RPush(cache.RedisClient.Context(), marketingQueueKey, batchID).Err()
}

func (s *MarketingService) ProcessQueue() {
	if cache.RedisClient == nil {
		log.Println("Marketing queue worker not started: redis client is not initialized")
		return
	}

	ctx := cache.RedisClient.Context()
	for {
		result, err := cache.RedisClient.BLPop(ctx, 5*time.Second, marketingQueueKey).Result()
		if err != nil {
			if err == redis.Nil {
				continue
			}
			log.Printf("marketing queue BLPop failed: %v", err)
			continue
		}
		if len(result) < 2 {
			continue
		}

		batchID64, err := strconv.ParseUint(strings.TrimSpace(result[1]), 10, 64)
		if err != nil {
			log.Printf("marketing queue batch id invalid: %q, err=%v", result[1], err)
			continue
		}

		batchID := uint(batchID64)
		if err := s.processBatch(batchID); err != nil {
			log.Printf("process marketing batch failed, batch=%d: %v", batchID, err)
			s.failBatch(batchID, err.Error())
		}
	}
}

func (s *MarketingService) processBatch(batchID uint) error {
	lockKey := fmt.Sprintf(marketingLockKeyFmt, batchID)
	locked, err := cache.SetNX(lockKey, "1", marketingLockDuration)
	if err != nil {
		return fmt.Errorf("acquire batch lock failed: %w", err)
	}
	if !locked {
		return nil
	}
	defer func() {
		if err := cache.Del(lockKey); err != nil {
			log.Printf("release marketing batch lock failed, batch=%d: %v", batchID, err)
		}
	}()

	var batch models.MarketingBatch
	if err := s.db.First(&batch, batchID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if batch.Status == models.MarketingBatchStatusCompleted {
		return nil
	}

	now := models.NowFunc()
	if err := s.db.Model(&models.MarketingBatch{}).
		Where("id = ?", batchID).
		Updates(map[string]interface{}{
			"status":        models.MarketingBatchStatusRunning,
			"started_at":    now,
			"failed_reason": "",
		}).Error; err != nil {
		return err
	}

	if err := s.db.First(&batch, batchID).Error; err != nil {
		return err
	}

	for {
		var tasks []models.MarketingBatchTask
		if err := s.db.Where("batch_id = ? AND status = ?", batchID, models.MarketingTaskStatusPending).
			Order("id ASC").
			Limit(200).
			Find(&tasks).Error; err != nil {
			return err
		}
		if len(tasks) == 0 {
			break
		}

		for i := range tasks {
			if err := s.processTask(&batch, &tasks[i]); err != nil {
				log.Printf("process marketing task failed, batch=%d task=%d: %v", batchID, tasks[i].ID, err)
			}
		}

		if err := s.refreshBatchStats(batchID); err != nil {
			log.Printf("refresh marketing batch stats failed, batch=%d: %v", batchID, err)
		}
	}

	if err := s.refreshBatchStats(batchID); err != nil {
		return err
	}

	unresolved, err := s.countUnresolvedTasks(batchID)
	if err != nil {
		return err
	}
	if unresolved > 0 {
		// Some tasks are still waiting for downstream processing (for example email queue).
		return nil
	}

	completedAt := models.NowFunc()
	return s.db.Model(&models.MarketingBatch{}).
		Where("id = ? AND status <> ?", batchID, models.MarketingBatchStatusFailed).
		Updates(map[string]interface{}{
			"status":       models.MarketingBatchStatusCompleted,
			"completed_at": completedAt,
		}).Error
}

func (s *MarketingService) processTask(batch *models.MarketingBatch, task *models.MarketingBatchTask) error {
	var user models.User
	if err := s.db.Select("id", "name", "email", "phone", "locale", "email_notify_marketing", "sms_notify_marketing").
		First(&user, task.UserID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return s.updateTaskResult(task.ID, models.MarketingTaskStatusFailed, "user not found")
		}
		return s.updateTaskResult(task.ID, models.MarketingTaskStatusFailed, "query user failed: "+err.Error())
	}

	switch task.Channel {
	case models.MarketingTaskChannelEmail:
		return s.processEmailTask(batch, &user, task.ID)
	case models.MarketingTaskChannelSMS:
		return s.processSMSTask(batch, &user, task.ID)
	default:
		return s.updateTaskResult(task.ID, models.MarketingTaskStatusFailed, "unsupported channel")
	}
}

func (s *MarketingService) processEmailTask(batch *models.MarketingBatch, user *models.User, taskID uint) error {
	if user.Email == "" || !user.EmailNotifyMarketing {
		return s.updateTaskResult(taskID, models.MarketingTaskStatusSkipped, "")
	}
	if s.emailService == nil {
		return s.updateTaskResult(taskID, models.MarketingTaskStatusFailed, "email service unavailable")
	}

	rendered := RenderMarketingContent(batch.Title, batch.Content, user)
	if strings.TrimSpace(rendered.EmailHTML) == "" {
		return s.updateTaskResult(taskID, models.MarketingTaskStatusSkipped, "")
	}

	batchID := batch.ID
	if err := s.emailService.SendMarketingAnnouncementEmailWithBatch(user, rendered.EmailSubject, rendered.EmailHTML, &batchID); err != nil {
		return s.updateTaskResult(taskID, models.MarketingTaskStatusFailed, err.Error())
	}
	return s.updateTaskResult(taskID, models.MarketingTaskStatusQueued, "")
}

func (s *MarketingService) processSMSTask(batch *models.MarketingBatch, user *models.User, taskID uint) error {
	phone := ""
	if user.Phone != nil {
		phone = strings.TrimSpace(*user.Phone)
	}
	if phone == "" || !user.SMSNotifyMarketing {
		return s.updateTaskResult(taskID, models.MarketingTaskStatusSkipped, "")
	}
	if s.smsService == nil {
		return s.updateTaskResult(taskID, models.MarketingTaskStatusFailed, "sms service unavailable")
	}

	rendered := RenderMarketingContent(batch.Title, batch.Content, user)
	if strings.TrimSpace(rendered.SMSText) == "" {
		return s.updateTaskResult(taskID, models.MarketingTaskStatusSkipped, "")
	}

	batchID := batch.ID
	if err := s.smsService.SendMarketingSMSWithBatch(user, rendered.SMSText, &batchID); err != nil {
		return s.updateTaskResult(taskID, models.MarketingTaskStatusFailed, err.Error())
	}
	return s.updateTaskResult(taskID, models.MarketingTaskStatusSent, "")
}

func (s *MarketingService) updateTaskResult(taskID uint, status models.MarketingTaskStatus, errMessage string) error {
	var processedAt interface{} = nil
	if status == models.MarketingTaskStatusSent || status == models.MarketingTaskStatusFailed || status == models.MarketingTaskStatusSkipped {
		now := models.NowFunc()
		processedAt = now
	}

	updates := map[string]interface{}{
		"status":        status,
		"processed_at":  processedAt,
		"error_message": "",
	}
	if status == models.MarketingTaskStatusFailed {
		updates["error_message"] = trimError(errMessage)
	}

	return s.db.Model(&models.MarketingBatchTask{}).Where("id = ?", taskID).Updates(updates).Error
}

func (s *MarketingService) refreshBatchStats(batchID uint) error {
	var total int64
	if err := s.db.Model(&models.MarketingBatchTask{}).Where("batch_id = ?", batchID).Count(&total).Error; err != nil {
		return err
	}

	var processed int64
	if err := s.db.Model(&models.MarketingBatchTask{}).
		Where("batch_id = ? AND status IN ?", batchID, []models.MarketingTaskStatus{
			models.MarketingTaskStatusSent,
			models.MarketingTaskStatusFailed,
			models.MarketingTaskStatusSkipped,
		}).
		Count(&processed).Error; err != nil {
		return err
	}

	type aggRow struct {
		Channel models.MarketingTaskChannel `gorm:"column:channel"`
		Status  models.MarketingTaskStatus  `gorm:"column:status"`
		Count   int64                       `gorm:"column:count"`
	}
	var rows []aggRow
	if err := s.db.Model(&models.MarketingBatchTask{}).
		Select("channel, status, COUNT(*) as count").
		Where("batch_id = ?", batchID).
		Group("channel, status").
		Scan(&rows).Error; err != nil {
		return err
	}

	emailSent := int64(0)
	emailFailed := int64(0)
	emailSkipped := int64(0)
	smsSent := int64(0)
	smsFailed := int64(0)
	smsSkipped := int64(0)

	for _, row := range rows {
		switch row.Channel {
		case models.MarketingTaskChannelEmail:
			switch row.Status {
			case models.MarketingTaskStatusSent:
				emailSent = row.Count
			case models.MarketingTaskStatusFailed:
				emailFailed = row.Count
			case models.MarketingTaskStatusSkipped:
				emailSkipped = row.Count
			}
		case models.MarketingTaskChannelSMS:
			switch row.Status {
			case models.MarketingTaskStatusSent:
				smsSent = row.Count
			case models.MarketingTaskStatusFailed:
				smsFailed = row.Count
			case models.MarketingTaskStatusSkipped:
				smsSkipped = row.Count
			}
		}
	}

	return s.db.Model(&models.MarketingBatch{}).
		Where("id = ?", batchID).
		Updates(map[string]interface{}{
			"total_tasks":     int(total),
			"processed_tasks": int(processed),
			"email_sent":      int(emailSent),
			"email_failed":    int(emailFailed),
			"email_skipped":   int(emailSkipped),
			"sms_sent":        int(smsSent),
			"sms_failed":      int(smsFailed),
			"sms_skipped":     int(smsSkipped),
		}).Error
}

func (s *MarketingService) countUnresolvedTasks(batchID uint) (int64, error) {
	var unresolved int64
	err := s.db.Model(&models.MarketingBatchTask{}).
		Where("batch_id = ? AND status IN ?", batchID, []models.MarketingTaskStatus{
			models.MarketingTaskStatusPending,
			models.MarketingTaskStatusQueued,
		}).
		Count(&unresolved).Error
	return unresolved, err
}

func (s *MarketingService) failBatch(batchID uint, errMessage string) {
	if batchID == 0 {
		return
	}
	completedAt := models.NowFunc()
	if err := s.db.Model(&models.MarketingBatch{}).
		Where("id = ?", batchID).
		Updates(map[string]interface{}{
			"status":        models.MarketingBatchStatusFailed,
			"failed_reason": trimError(errMessage),
			"completed_at":  completedAt,
		}).Error; err != nil {
		log.Printf("update failed marketing batch status failed, batch=%d: %v", batchID, err)
	}
}

func trimError(msg string) string {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return ""
	}
	const maxLen = 1000
	if len(msg) <= maxLen {
		return msg
	}
	return msg[:maxLen]
}
