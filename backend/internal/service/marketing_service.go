package service

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
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
	db            *gorm.DB
	emailService  *EmailService
	smsService    *SMSService
	pluginManager *PluginManagerService
	workerMu      sync.Mutex
	workerWG      sync.WaitGroup
	workerStop    chan struct{}
	workerRunning bool
}

func NewMarketingService(db *gorm.DB, emailService *EmailService, smsService *SMSService) *MarketingService {
	return &MarketingService{
		db:           db,
		emailService: emailService,
		smsService:   smsService,
	}
}

func (s *MarketingService) SetPluginManager(pluginManager *PluginManagerService) {
	s.pluginManager = pluginManager
}

func (s *MarketingService) Start() {
	if cache.RedisClient == nil {
		log.Println("Marketing queue worker not started: redis client is not initialized")
		return
	}

	s.workerMu.Lock()
	defer s.workerMu.Unlock()

	if s.workerRunning {
		return
	}

	stopChan := make(chan struct{})
	s.workerStop = stopChan
	s.workerRunning = true
	s.workerWG.Add(1)

	go func() {
		defer s.workerWG.Done()
		s.processQueueLoop(stopChan)
	}()
}

func (s *MarketingService) Stop() {
	s.workerMu.Lock()
	defer s.workerMu.Unlock()

	if !s.workerRunning {
		return
	}

	close(s.workerStop)
	s.workerStop = nil
	s.workerRunning = false
	s.workerWG.Wait()
}

func buildMarketingBatchServiceHookPayload(batch *models.MarketingBatch) map[string]interface{} {
	if batch == nil {
		return map[string]interface{}{}
	}

	return map[string]interface{}{
		"batch_id":             batch.ID,
		"batch_no":             batch.BatchNo,
		"title":                batch.Title,
		"content":              batch.Content,
		"send_email":           batch.SendEmail,
		"send_sms":             batch.SendSMS,
		"target_all":           batch.TargetAll,
		"audience_mode":        batch.AudienceMode,
		"audience_query":       batch.AudienceQuery,
		"status":               batch.Status,
		"total_tasks":          batch.TotalTasks,
		"processed_tasks":      batch.ProcessedTasks,
		"requested_user_count": batch.RequestedUserCount,
		"targeted_users":       batch.TargetedUsers,
		"email_sent":           batch.EmailSent,
		"email_failed":         batch.EmailFailed,
		"email_skipped":        batch.EmailSkipped,
		"sms_sent":             batch.SmsSent,
		"sms_failed":           batch.SmsFailed,
		"sms_skipped":          batch.SmsSkipped,
		"failed_reason":        batch.FailedReason,
		"operator_id":          batch.OperatorID,
		"operator_name":        batch.OperatorName,
		"started_at":           batch.StartedAt,
		"completed_at":         batch.CompletedAt,
		"created_at":           batch.CreatedAt,
		"updated_at":           batch.UpdatedAt,
	}
}

func buildMarketingTaskServiceHookPayload(batch *models.MarketingBatch, user *models.User, task *models.MarketingBatchTask) map[string]interface{} {
	payload := map[string]interface{}{}
	if batch != nil {
		for key, value := range buildMarketingBatchServiceHookPayload(batch) {
			payload[key] = value
		}
	}
	if task != nil {
		payload["task_id"] = task.ID
		payload["channel"] = task.Channel
		payload["task_status"] = task.Status
		payload["task_error_message"] = task.ErrorMessage
		payload["task_processed_at"] = task.ProcessedAt
		payload["task_created_at"] = task.CreatedAt
		payload["task_updated_at"] = task.UpdatedAt
		payload["user_id"] = task.UserID
	}
	if user != nil {
		payload["user_email"] = user.Email
		payload["user_name"] = user.Name
		payload["user_locale"] = user.Locale
		payload["user_email_verified"] = user.EmailVerified
		payload["user_email_notify_marketing"] = user.EmailNotifyMarketing
		payload["user_sms_notify_marketing"] = user.SMSNotifyMarketing
		if user.Phone != nil {
			payload["user_phone"] = *user.Phone
		}
	}
	return payload
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
	s.processQueueLoop(nil)
}

func (s *MarketingService) processQueueLoop(stopChan <-chan struct{}) {
	defer recoverBackgroundServicePanic("marketing.processQueueLoop")
	if cache.RedisClient == nil {
		log.Println("Marketing queue worker not started: redis client is not initialized")
		return
	}

	ctx := cache.RedisClient.Context()
	for {
		select {
		case <-stopChan:
			return
		default:
		}

		result, err := cache.RedisClient.BLPop(ctx, 5*time.Second, marketingQueueKey).Result()
		if err != nil {
			select {
			case <-stopChan:
				return
			default:
			}
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
	if s.pluginManager != nil {
		afterPayload := buildMarketingBatchServiceHookPayload(&batch)
		afterPayload["source"] = "marketing_worker"
		go func(execCtx *ExecutionContext, payload map[string]interface{}, currentBatchID uint) {
			_, hookErr := s.pluginManager.ExecuteHook(HookExecutionRequest{
				Hook:    "marketing.batch.start.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("marketing.batch.start.after hook execution failed: batch=%d err=%v", currentBatchID, hookErr)
			}
		}(cloneServiceHookExecutionContext(buildServiceHookExecutionContext(batch.OperatorID, nil, map[string]string{
			"hook_source": "marketing_worker",
			"hook_phase":  "start",
			"batch_id":    strconv.FormatUint(uint64(batch.ID), 10),
		})), afterPayload, batch.ID)
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

func (s *MarketingService) executeMarketingTaskDispatchBeforeHook(
	batch *models.MarketingBatch,
	user *models.User,
	task *models.MarketingBatchTask,
	title *string,
	content *string,
	emailSubject *string,
	emailHTML *string,
	smsText *string,
) error {
	if s.pluginManager == nil || batch == nil || task == nil {
		return nil
	}

	payload := buildMarketingTaskServiceHookPayload(batch, user, task)
	payload["title"] = derefString(title)
	payload["content"] = derefString(content)
	payload["email_subject"] = derefString(emailSubject)
	payload["email_html"] = derefString(emailHTML)
	payload["sms_text"] = derefString(smsText)
	payload["source"] = "marketing_worker"

	execCtx := buildServiceHookExecutionContext(&task.UserID, nil, map[string]string{
		"hook_source": "marketing_worker",
		"batch_id":    strconv.FormatUint(uint64(batch.ID), 10),
		"task_id":     strconv.FormatUint(uint64(task.ID), 10),
		"channel":     string(task.Channel),
	})
	hookResult, hookErr := s.pluginManager.ExecuteHook(HookExecutionRequest{
		Hook:    "marketing.task.dispatch.before",
		Payload: payload,
	}, execCtx)
	if hookErr != nil {
		log.Printf("marketing.task.dispatch.before hook execution failed: batch=%d task=%d err=%v", batch.ID, task.ID, hookErr)
		return nil
	}
	if hookResult == nil {
		return nil
	}
	if hookResult.Blocked {
		reason := strings.TrimSpace(hookResult.BlockReason)
		if reason == "" {
			reason = "Marketing task dispatch rejected by plugin"
		}
		return newHookBlockedError(reason)
	}
	if hookResult.Payload == nil {
		return nil
	}

	if title != nil {
		if value, exists := hookResult.Payload["title"]; exists {
			updated, convErr := serviceHookValueToString(value)
			if convErr != nil {
				log.Printf("marketing.task.dispatch.before title patch ignored: batch=%d task=%d err=%v", batch.ID, task.ID, convErr)
			} else {
				*title = strings.TrimSpace(updated)
			}
		}
	}
	if content != nil {
		if value, exists := hookResult.Payload["content"]; exists {
			updated, convErr := serviceHookValueToString(value)
			if convErr != nil {
				log.Printf("marketing.task.dispatch.before content patch ignored: batch=%d task=%d err=%v", batch.ID, task.ID, convErr)
			} else {
				*content = strings.TrimSpace(updated)
			}
		}
	}
	if emailSubject != nil {
		if value, exists := hookResult.Payload["email_subject"]; exists {
			updated, convErr := serviceHookValueToString(value)
			if convErr != nil {
				log.Printf("marketing.task.dispatch.before email_subject patch ignored: batch=%d task=%d err=%v", batch.ID, task.ID, convErr)
			} else {
				*emailSubject = strings.TrimSpace(updated)
			}
		}
	}
	if emailHTML != nil {
		if value, exists := hookResult.Payload["email_html"]; exists {
			updated, convErr := serviceHookValueToString(value)
			if convErr != nil {
				log.Printf("marketing.task.dispatch.before email_html patch ignored: batch=%d task=%d err=%v", batch.ID, task.ID, convErr)
			} else {
				*emailHTML = updated
			}
		}
	}
	if smsText != nil {
		if value, exists := hookResult.Payload["sms_text"]; exists {
			updated, convErr := serviceHookValueToString(value)
			if convErr != nil {
				log.Printf("marketing.task.dispatch.before sms_text patch ignored: batch=%d task=%d err=%v", batch.ID, task.ID, convErr)
			} else {
				*smsText = strings.TrimSpace(updated)
			}
		}
	}

	return nil
}

func (s *MarketingService) emitMarketingTaskDispatchAfterHook(
	batch *models.MarketingBatch,
	user *models.User,
	task *models.MarketingBatchTask,
	title string,
	content string,
	emailSubject string,
	emailHTML string,
	smsText string,
	status models.MarketingTaskStatus,
	errMessage string,
) {
	if s.pluginManager == nil || batch == nil || task == nil {
		return
	}

	payload := buildMarketingTaskServiceHookPayload(batch, user, task)
	payload["title"] = title
	payload["content"] = content
	payload["email_subject"] = emailSubject
	payload["email_html"] = emailHTML
	payload["sms_text"] = smsText
	payload["status"] = status
	payload["error_message"] = strings.TrimSpace(errMessage)
	payload["source"] = "marketing_worker"

	go func(execCtx *ExecutionContext, hookPayload map[string]interface{}, batchID uint, taskID uint) {
		_, hookErr := s.pluginManager.ExecuteHook(HookExecutionRequest{
			Hook:    "marketing.task.dispatch.after",
			Payload: hookPayload,
		}, execCtx)
		if hookErr != nil {
			log.Printf("marketing.task.dispatch.after hook execution failed: batch=%d task=%d err=%v", batchID, taskID, hookErr)
		}
	}(cloneServiceHookExecutionContext(buildServiceHookExecutionContext(&task.UserID, nil, map[string]string{
		"hook_source": "marketing_worker",
		"batch_id":    strconv.FormatUint(uint64(batch.ID), 10),
		"task_id":     strconv.FormatUint(uint64(task.ID), 10),
		"channel":     string(task.Channel),
	})), payload, batch.ID, task.ID)
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (s *MarketingService) processTask(batch *models.MarketingBatch, task *models.MarketingBatchTask) error {
	var user models.User
	if err := s.db.Select("id", "name", "email", "phone", "locale", "email_verified", "email_notify_marketing", "sms_notify_marketing").
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
	if user.Email == "" || !user.EmailVerified || !user.EmailNotifyMarketing {
		return s.updateTaskResult(taskID, models.MarketingTaskStatusSkipped, "")
	}
	if s.emailService == nil || !s.emailService.IsEnabled() {
		return s.updateTaskResult(taskID, models.MarketingTaskStatusFailed, "email service is not enabled")
	}

	task := &models.MarketingBatchTask{
		ID:      taskID,
		BatchID: batch.ID,
		UserID:  user.ID,
		Channel: models.MarketingTaskChannelEmail,
	}
	title := batch.Title
	content := batch.Content
	rendered := RenderMarketingContent(title, content, user)
	emailSubject := rendered.EmailSubject
	emailHTML := rendered.EmailHTML
	if strings.TrimSpace(emailHTML) == "" {
		return s.updateTaskResult(taskID, models.MarketingTaskStatusSkipped, "")
	}
	if hookErr := s.executeMarketingTaskDispatchBeforeHook(batch, user, task, &title, &content, &emailSubject, &emailHTML, nil); hookErr != nil {
		status := models.MarketingTaskStatusFailed
		errMessage := hookErr.Error()
		if updateErr := s.updateTaskResult(taskID, status, errMessage); updateErr != nil {
			return updateErr
		}
		s.emitMarketingTaskDispatchAfterHook(batch, user, task, title, content, emailSubject, emailHTML, "", status, errMessage)
		return nil
	}
	if strings.TrimSpace(emailHTML) == "" {
		status := models.MarketingTaskStatusSkipped
		if updateErr := s.updateTaskResult(taskID, status, ""); updateErr != nil {
			return updateErr
		}
		s.emitMarketingTaskDispatchAfterHook(batch, user, task, title, content, emailSubject, emailHTML, "", status, "")
		return nil
	}

	batchID := batch.ID
	if err := s.emailService.queueEmail(user.Email, emailSubject, emailHTML, "marketing.announcement", nil, &user.ID, &batchID); err != nil {
		status := models.MarketingTaskStatusFailed
		errMessage := err.Error()
		if updateErr := s.updateTaskResult(taskID, status, errMessage); updateErr != nil {
			return updateErr
		}
		s.emitMarketingTaskDispatchAfterHook(batch, user, task, title, content, emailSubject, emailHTML, "", status, errMessage)
		return nil
	}
	status := models.MarketingTaskStatusQueued
	if err := s.updateTaskResult(taskID, status, ""); err != nil {
		return err
	}
	s.emitMarketingTaskDispatchAfterHook(batch, user, task, title, content, emailSubject, emailHTML, "", status, "")
	return nil
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

	task := &models.MarketingBatchTask{
		ID:      taskID,
		BatchID: batch.ID,
		UserID:  user.ID,
		Channel: models.MarketingTaskChannelSMS,
	}
	title := batch.Title
	content := batch.Content
	rendered := RenderMarketingContent(title, content, user)
	smsText := rendered.SMSText
	if strings.TrimSpace(smsText) == "" {
		return s.updateTaskResult(taskID, models.MarketingTaskStatusSkipped, "")
	}
	if hookErr := s.executeMarketingTaskDispatchBeforeHook(batch, user, task, &title, &content, nil, nil, &smsText); hookErr != nil {
		status := models.MarketingTaskStatusFailed
		errMessage := hookErr.Error()
		if updateErr := s.updateTaskResult(taskID, status, errMessage); updateErr != nil {
			return updateErr
		}
		s.emitMarketingTaskDispatchAfterHook(batch, user, task, title, content, "", "", smsText, status, errMessage)
		return nil
	}
	if strings.TrimSpace(smsText) == "" {
		status := models.MarketingTaskStatusSkipped
		if updateErr := s.updateTaskResult(taskID, status, ""); updateErr != nil {
			return updateErr
		}
		s.emitMarketingTaskDispatchAfterHook(batch, user, task, title, content, "", "", smsText, status, "")
		return nil
	}

	batchID := batch.ID
	if err := s.smsService.sendMarketingDirect(phone, "", smsText, &user.ID, &batchID); err != nil {
		status := models.MarketingTaskStatusFailed
		errMessage := err.Error()
		if updateErr := s.updateTaskResult(taskID, status, errMessage); updateErr != nil {
			return updateErr
		}
		s.emitMarketingTaskDispatchAfterHook(batch, user, task, title, content, "", "", smsText, status, errMessage)
		return nil
	}
	status := models.MarketingTaskStatusSent
	if err := s.updateTaskResult(taskID, status, ""); err != nil {
		return err
	}
	s.emitMarketingTaskDispatchAfterHook(batch, user, task, title, content, "", "", smsText, status, "")
	return nil
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
