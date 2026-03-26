package admin

import (
	"errors"
	"log"
	"strconv"
	"strings"

	"auralogic/internal/models"
	"auralogic/internal/pkg/logger"
	"auralogic/internal/pkg/response"
	"auralogic/internal/pkg/utils"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type MarketingHandler struct {
	db               *gorm.DB
	marketingService *service.MarketingService
	pluginManager    *service.PluginManagerService
}

func NewMarketingHandler(db *gorm.DB, marketingService *service.MarketingService, pluginManager *service.PluginManagerService) *MarketingHandler {
	return &MarketingHandler{
		db:               db,
		marketingService: marketingService,
		pluginManager:    pluginManager,
	}
}

func buildMarketingBatchHookPayload(batch *models.MarketingBatch) map[string]interface{} {
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

func uniqueUserIDs(ids []uint) []uint {
	if len(ids) == 0 {
		return ids
	}
	seen := make(map[uint]struct{}, len(ids))
	result := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func contextUserID(c *gin.Context) *uint {
	v, exists := c.Get("user_id")
	if !exists {
		return nil
	}

	switch id := v.(type) {
	case uint:
		uid := id
		return &uid
	case *uint:
		if id == nil {
			return nil
		}
		uid := *id
		return &uid
	default:
		return nil
	}
}

func (h *MarketingHandler) resolveOperator(c *gin.Context) (*uint, string) {
	operatorID := contextUserID(c)
	operatorName := strings.TrimSpace(c.GetString("api_platform"))

	if operatorID != nil {
		var operator models.User
		if err := h.db.Select("id", "name", "email").First(&operator, *operatorID).Error; err == nil {
			if strings.TrimSpace(operator.Name) != "" {
				operatorName = strings.TrimSpace(operator.Name)
			} else if strings.TrimSpace(operator.Email) != "" {
				operatorName = strings.TrimSpace(operator.Email)
			}
		}
	}

	if operatorName == "" {
		operatorName = "system"
	}

	return operatorID, operatorName
}

func (h *MarketingHandler) ListRecipients(c *gin.Context) {
	page, limit := response.GetPagination(c)
	search := strings.TrimSpace(c.Query("search"))
	locale := strings.TrimSpace(c.Query("locale"))
	country := strings.TrimSpace(c.Query("country"))

	isActive, ok := parseOptionalBoolQuery(c.Query("is_active"))
	if !ok {
		response.BadRequest(c, "Invalid is_active parameter")
		return
	}
	emailVerified, ok := parseOptionalBoolQuery(c.Query("email_verified"))
	if !ok {
		response.BadRequest(c, "Invalid email_verified parameter")
		return
	}
	emailNotifyMarketing, ok := parseOptionalBoolQuery(c.Query("email_notify_marketing"))
	if !ok {
		response.BadRequest(c, "Invalid email_notify_marketing parameter")
		return
	}
	smsNotifyMarketing, ok := parseOptionalBoolQuery(c.Query("sms_notify_marketing"))
	if !ok {
		response.BadRequest(c, "Invalid sms_notify_marketing parameter")
		return
	}
	hasPhone, ok := parseOptionalBoolQuery(c.Query("has_phone"))
	if !ok {
		response.BadRequest(c, "Invalid has_phone parameter")
		return
	}

	query := h.db.Model(&models.User{}).Where("role = ?", "user")
	if search != "" {
		like := "%" + search + "%"
		query = query.Where("email LIKE ? OR name LIKE ? OR phone LIKE ?", like, like, like)
	}
	if isActive != nil {
		query = query.Where("is_active = ?", *isActive)
	}
	if emailVerified != nil {
		query = query.Where("email_verified = ?", *emailVerified)
	}
	if emailNotifyMarketing != nil {
		query = query.Where("email_notify_marketing = ?", *emailNotifyMarketing)
	}
	if smsNotifyMarketing != nil {
		query = query.Where("sms_notify_marketing = ?", *smsNotifyMarketing)
	}
	if hasPhone != nil {
		if *hasPhone {
			query = query.Where("phone IS NOT NULL AND phone <> ''")
		} else {
			query = query.Where("phone IS NULL OR phone = ''")
		}
	}
	if locale != "" {
		query = query.Where("LOWER(locale) = LOWER(?)", locale)
	}
	if country != "" {
		query = query.Where("LOWER(country) = LOWER(?)", country)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	var users []models.User
	if err := query.
		Select("id", "name", "email", "phone", "is_active", "email_verified", "locale", "country", "email_notify_marketing", "sms_notify_marketing", "created_at").
		Order("id DESC").
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&users).Error; err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	items := make([]gin.H, 0, len(users))
	for i := range users {
		user := users[i]
		item := gin.H{
			"id":                     user.ID,
			"name":                   user.Name,
			"email":                  user.Email,
			"is_active":              user.IsActive,
			"email_verified":         user.EmailVerified,
			"locale":                 user.Locale,
			"country":                user.Country,
			"email_notify_marketing": user.EmailNotifyMarketing,
			"sms_notify_marketing":   user.SMSNotifyMarketing,
			"created_at":             user.CreatedAt,
		}
		if user.Phone != nil {
			item["phone"] = *user.Phone
		}
		items = append(items, item)
	}

	response.Paginated(c, items, page, limit, total)
}

func (h *MarketingHandler) ListRecipientCountries(c *gin.Context) {
	countries := make([]string, 0)
	if err := h.db.Model(&models.User{}).
		Where("role = ?", "user").
		Where("country IS NOT NULL").
		Where("TRIM(country) <> ''").
		Select("DISTINCT UPPER(TRIM(country)) AS country").
		Order("country ASC").
		Pluck("country", &countries).Error; err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	response.Success(c, gin.H{
		"countries": countries,
	})
}

func (h *MarketingHandler) ListBatches(c *gin.Context) {
	page, limit := response.GetPagination(c)
	batchNo := strings.TrimSpace(c.Query("batch_no"))
	operator := strings.TrimSpace(c.Query("operator"))
	status := strings.TrimSpace(c.Query("status"))

	query := h.db.Model(&models.MarketingBatch{})
	if batchNo != "" {
		query = query.Where("batch_no LIKE ?", "%"+batchNo+"%")
	}
	if operator != "" {
		query = query.Where("operator_name LIKE ?", "%"+operator+"%")
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	var batches []models.MarketingBatch
	if err := query.Order("id DESC").
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&batches).Error; err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	response.Paginated(c, batches, page, limit, total)
}

func (h *MarketingHandler) GetBatch(c *gin.Context) {
	batchID, err := parseUintParam(c.Param("id"))
	if err != nil || batchID == 0 {
		response.BadRequest(c, "Invalid batch id")
		return
	}

	var batch models.MarketingBatch
	if err := h.db.First(&batch, batchID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "Batch not found")
			return
		}
		response.InternalError(c, "Query failed")
		return
	}

	response.Success(c, gin.H{
		"id":                   batch.ID,
		"batch_id":             batch.ID,
		"batch_no":             batch.BatchNo,
		"title":                batch.Title,
		"status":               batch.Status,
		"failed_reason":        batch.FailedReason,
		"send_email":           batch.SendEmail,
		"send_sms":             batch.SendSMS,
		"target_all":           batch.TargetAll,
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
		"operator_id":          batch.OperatorID,
		"operator_name":        batch.OperatorName,
		"started_at":           batch.StartedAt,
		"completed_at":         batch.CompletedAt,
		"created_at":           batch.CreatedAt,
		"updated_at":           batch.UpdatedAt,
	})
}

func (h *MarketingHandler) PreviewMarketing(c *gin.Context) {
	var req struct {
		Title   string `json:"title"`
		Content string `json:"content"`
		UserID  *uint  `json:"user_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}
	if h.pluginManager != nil {
		originalReq := req
		hookPayload, payloadErr := adminHookStructToPayload(req)
		if payloadErr != nil {
			log.Printf("marketing.preview.before payload build failed: admin=%d err=%v", adminIDValue, payloadErr)
		} else {
			hookPayload["admin_id"] = adminIDValue
			hookPayload["source"] = "admin_api"
			hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "marketing.preview.before",
				Payload: hookPayload,
			}, buildAdminHookExecutionContext(c, adminID, map[string]string{
				"hook_resource": "marketing",
				"hook_source":   "admin_api",
				"hook_action":   "preview",
			}))
			if hookErr != nil {
				log.Printf("marketing.preview.before hook execution failed: admin=%d err=%v", adminIDValue, hookErr)
			} else if hookResult != nil {
				if hookResult.Blocked {
					reason := strings.TrimSpace(hookResult.BlockReason)
					if reason == "" {
						reason = "Marketing preview rejected by plugin"
					}
					response.BadRequest(c, reason)
					return
				}
				if hookResult.Payload != nil {
					if mergeErr := mergeAdminHookStructPatch(&req, hookResult.Payload); mergeErr != nil {
						log.Printf("marketing.preview.before payload apply failed, fallback to original request: admin=%d err=%v", adminIDValue, mergeErr)
						req = originalReq
					}
				}
			}
		}
	}

	req.Title = strings.TrimSpace(req.Title)
	req.Content = strings.TrimSpace(req.Content)
	if req.Title == "" && req.Content == "" {
		response.BadRequest(c, "Title or content is required")
		return
	}

	var user *models.User
	if req.UserID != nil && *req.UserID > 0 {
		var target models.User
		if err := h.db.Select("id", "name", "email", "phone", "locale").
			Where("id = ? AND role = ?", *req.UserID, "user").
			First(&target).Error; err == nil {
			user = &target
		}
	}

	rendered := service.RenderMarketingContent(req.Title, req.Content, user)
	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"title":                        rendered.Title,
			"email_subject":                rendered.EmailSubject,
			"content_html":                 rendered.ContentHTML,
			"email_html":                   rendered.EmailHTML,
			"sms_text":                     rendered.SMSText,
			"resolved_variables":           rendered.Variables,
			"supported_placeholders":       rendered.Placeholders,
			"supported_template_variables": rendered.TemplateVars,
			"admin_id":                     adminIDValue,
			"source":                       "admin_api",
		}
		if req.UserID != nil {
			afterPayload["user_id"] = *req.UserID
		}
		if user != nil {
			afterPayload["resolved_user_id"] = user.ID
			afterPayload["resolved_user_email"] = user.Email
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "marketing.preview.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("marketing.preview.after hook execution failed: admin=%d err=%v", adminIDValue, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "marketing",
			"hook_source":   "admin_api",
			"hook_action":   "preview",
		})), afterPayload)
	}
	response.Success(c, gin.H{
		"title":                        rendered.Title,
		"email_subject":                rendered.EmailSubject,
		"content_html":                 rendered.ContentHTML,
		"email_html":                   rendered.EmailHTML,
		"sms_text":                     rendered.SMSText,
		"resolved_variables":           rendered.Variables,
		"supported_placeholders":       rendered.Placeholders,
		"supported_template_variables": rendered.TemplateVars,
	})
}

func (h *MarketingHandler) ListBatchTasks(c *gin.Context) {
	batchID, err := parseUintParam(c.Param("id"))
	if err != nil || batchID == 0 {
		response.BadRequest(c, "Invalid batch id")
		return
	}

	page, limit := response.GetPagination(c)
	status := strings.TrimSpace(c.Query("status"))
	channel := strings.TrimSpace(c.Query("channel"))
	search := strings.TrimSpace(c.Query("search"))

	query := h.db.Model(&models.MarketingBatchTask{}).Where("batch_id = ?", batchID).Preload("User")
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if channel != "" {
		query = query.Where("channel = ?", channel)
	}
	if search != "" {
		like := "%" + search + "%"
		query = query.Joins("LEFT JOIN users ON users.id = marketing_batch_tasks.user_id").
			Where("users.email LIKE ? OR users.name LIKE ?", like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	var tasks []models.MarketingBatchTask
	if err := query.Order("marketing_batch_tasks.id ASC").
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&tasks).Error; err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	items := make([]gin.H, 0, len(tasks))
	for i := range tasks {
		task := tasks[i]
		item := gin.H{
			"id":            task.ID,
			"batch_id":      task.BatchID,
			"user_id":       task.UserID,
			"channel":       task.Channel,
			"status":        task.Status,
			"error_message": task.ErrorMessage,
			"processed_at":  task.ProcessedAt,
			"created_at":    task.CreatedAt,
		}
		if task.User != nil {
			user := gin.H{
				"id":    task.User.ID,
				"name":  task.User.Name,
				"email": task.User.Email,
			}
			if task.User.Phone != nil {
				user["phone"] = *task.User.Phone
			}
			item["user"] = user
		}
		items = append(items, item)
	}

	response.Paginated(c, items, page, limit, total)
}

func (h *MarketingHandler) SendMarketing(c *gin.Context) {
	var req struct {
		Title     string `json:"title" binding:"required"`
		Content   string `json:"content" binding:"required"`
		SendEmail bool   `json:"send_email"`
		SendSMS   bool   `json:"send_sms"`
		TargetAll bool   `json:"target_all"`
		UserIDs   []uint `json:"user_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}
	if h.pluginManager != nil {
		originalReq := req
		hookPayload, payloadErr := adminHookStructToPayload(req)
		if payloadErr != nil {
			log.Printf("marketing.send.before payload build failed: admin=%d err=%v", adminIDValue, payloadErr)
		} else {
			hookPayload["admin_id"] = adminIDValue
			hookPayload["source"] = "admin_api"
			hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "marketing.send.before",
				Payload: hookPayload,
			}, buildAdminHookExecutionContext(c, adminID, map[string]string{
				"hook_resource": "marketing",
				"hook_source":   "admin_api",
				"hook_action":   "send",
			}))
			if hookErr != nil {
				log.Printf("marketing.send.before hook execution failed: admin=%d err=%v", adminIDValue, hookErr)
			} else if hookResult != nil {
				if hookResult.Blocked {
					reason := strings.TrimSpace(hookResult.BlockReason)
					if reason == "" {
						reason = "Marketing send rejected by plugin"
					}
					response.BadRequest(c, reason)
					return
				}
				if hookResult.Payload != nil {
					if mergeErr := mergeAdminHookStructPatch(&req, hookResult.Payload); mergeErr != nil {
						log.Printf("marketing.send.before payload apply failed, fallback to original request: admin=%d err=%v", adminIDValue, mergeErr)
						req = originalReq
					}
				}
			}
		}
	}

	req.Title = strings.TrimSpace(req.Title)
	req.Content = strings.TrimSpace(req.Content)
	req.UserIDs = uniqueUserIDs(req.UserIDs)

	if req.Title == "" {
		response.BadRequest(c, "Title is required")
		return
	}
	if req.Content == "" {
		response.BadRequest(c, "Content is required")
		return
	}
	if !req.SendEmail && !req.SendSMS {
		response.BadRequest(c, "At least one channel must be selected")
		return
	}
	if !req.TargetAll && len(req.UserIDs) == 0 {
		response.BadRequest(c, "User IDs are required when target_all is false")
		return
	}

	query := h.db.Model(&models.User{}).Where("is_active = ? AND role = ?", true, "user")
	if !req.TargetAll {
		query = query.Where("id IN ?", req.UserIDs)
	}

	var users []models.User
	if err := query.Select("id", "email_verified").Find(&users).Error; err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	operatorID, operatorName := h.resolveOperator(c)
	batch := models.MarketingBatch{
		BatchNo:            utils.GenerateOrderNo("MKT"),
		Title:              req.Title,
		Content:            req.Content,
		SendEmail:          req.SendEmail,
		SendSMS:            req.SendSMS,
		TargetAll:          req.TargetAll,
		Status:             models.MarketingBatchStatusQueued,
		RequestedUserCount: len(req.UserIDs),
		TargetedUsers:      0,
		OperatorID:         operatorID,
		OperatorName:       operatorName,
	}

	tasks := make([]models.MarketingBatchTask, 0, len(users)*2)
	targetedUserIDs := make(map[uint]struct{}, len(users))
	for i := range users {
		user := users[i]
		userID := user.ID
		if req.SendEmail && user.EmailVerified {
			tasks = append(tasks, models.MarketingBatchTask{
				UserID:  userID,
				Channel: models.MarketingTaskChannelEmail,
				Status:  models.MarketingTaskStatusPending,
			})
			targetedUserIDs[userID] = struct{}{}
		}
		if req.SendSMS {
			tasks = append(tasks, models.MarketingBatchTask{
				UserID:  userID,
				Channel: models.MarketingTaskChannelSMS,
				Status:  models.MarketingTaskStatusPending,
			})
			targetedUserIDs[userID] = struct{}{}
		}
	}
	batch.TargetedUsers = len(targetedUserIDs)
	batch.TotalTasks = len(tasks)
	if batch.TotalTasks == 0 {
		batch.Status = models.MarketingBatchStatusCompleted
		completedAt := models.NowFunc()
		batch.CompletedAt = &completedAt
	}

	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&batch).Error; err != nil {
			return err
		}
		if len(tasks) == 0 {
			return nil
		}

		const chunkSize = 500
		for start := 0; start < len(tasks); start += chunkSize {
			end := start + chunkSize
			if end > len(tasks) {
				end = len(tasks)
			}

			chunk := tasks[start:end]
			for i := range chunk {
				chunk[i].BatchID = batch.ID
			}
			if err := tx.Create(&chunk).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		response.InternalError(c, "Create marketing batch failed")
		return
	}

	if batch.TotalTasks > 0 {
		if h.marketingService == nil {
			h.db.Model(&models.MarketingBatch{}).Where("id = ?", batch.ID).Updates(map[string]interface{}{
				"status":        models.MarketingBatchStatusFailed,
				"failed_reason": "marketing queue service unavailable",
				"completed_at":  models.NowFunc(),
			})
			response.InternalError(c, "Marketing queue service unavailable")
			return
		}
		if err := h.marketingService.EnqueueBatch(batch.ID); err != nil {
			h.db.Model(&models.MarketingBatch{}).Where("id = ?", batch.ID).Updates(map[string]interface{}{
				"status":        models.MarketingBatchStatusFailed,
				"failed_reason": err.Error(),
				"completed_at":  models.NowFunc(),
			})
			response.InternalError(c, "Failed to enqueue marketing batch")
			return
		}
		if h.pluginManager != nil {
			afterPayload := buildMarketingBatchHookPayload(&batch)
			afterPayload["admin_id"] = adminIDValue
			afterPayload["source"] = "admin_api"
			afterPayload["queue_enqueued"] = true
			go func(execCtx *service.ExecutionContext, payload map[string]interface{}) {
				_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
					Hook:    "marketing.batch.enqueue.after",
					Payload: payload,
				}, execCtx)
				if hookErr != nil {
					log.Printf("marketing.batch.enqueue.after hook execution failed: admin=%d batch=%d err=%v", adminIDValue, batch.ID, hookErr)
				}
			}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
				"hook_resource": "marketing",
				"hook_source":   "admin_api",
				"hook_action":   "enqueue",
				"batch_id":      strconv.FormatUint(uint64(batch.ID), 10),
			})), afterPayload)
		}
	}

	logger.LogOperation(h.db, c, "queue_marketing", "marketing_batch", &batch.ID, map[string]interface{}{
		"batch_no":             batch.BatchNo,
		"target_all":           req.TargetAll,
		"requested_user_count": len(req.UserIDs),
		"targeted_users":       len(users),
		"total_tasks":          batch.TotalTasks,
		"send_email":           req.SendEmail,
		"send_sms":             req.SendSMS,
		"operator_name":        operatorName,
	})
	if h.pluginManager != nil {
		afterPayload := buildMarketingBatchHookPayload(&batch)
		afterPayload["requested_user_ids"] = req.UserIDs
		afterPayload["resolved_user_count"] = len(users)
		afterPayload["admin_id"] = adminIDValue
		afterPayload["source"] = "admin_api"
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "marketing.send.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("marketing.send.after hook execution failed: admin=%d batch=%d err=%v", adminIDValue, batch.ID, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "marketing",
			"hook_source":   "admin_api",
			"hook_action":   "send",
			"batch_id":      strconv.FormatUint(uint64(batch.ID), 10),
		})), afterPayload)
	}

	response.Success(c, gin.H{
		"id":                   batch.ID,
		"batch_id":             batch.ID,
		"batch_no":             batch.BatchNo,
		"title":                batch.Title,
		"status":               batch.Status,
		"failed_reason":        batch.FailedReason,
		"send_email":           batch.SendEmail,
		"send_sms":             batch.SendSMS,
		"target_all":           batch.TargetAll,
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
		"operator_id":          batch.OperatorID,
		"operator_name":        batch.OperatorName,
		"started_at":           batch.StartedAt,
		"completed_at":         batch.CompletedAt,
		"created_at":           batch.CreatedAt,
		"updated_at":           batch.UpdatedAt,
	})
}

func parseUintParam(raw string) (uint, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, errors.New("empty id")
	}
	var parsed uint64
	for i := 0; i < len(raw); i++ {
		ch := raw[i]
		if ch < '0' || ch > '9' {
			return 0, errors.New("invalid id")
		}
		parsed = parsed*10 + uint64(ch-'0')
	}
	if parsed == 0 {
		return 0, errors.New("invalid id")
	}
	return uint(parsed), nil
}
