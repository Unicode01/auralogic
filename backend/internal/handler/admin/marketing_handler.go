package admin

import (
	"errors"
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
}

func NewMarketingHandler(db *gorm.DB, marketingService *service.MarketingService) *MarketingHandler {
	return &MarketingHandler{
		db:               db,
		marketingService: marketingService,
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

	query := h.db.Model(&models.User{}).Where("role = ?", "user")
	if search != "" {
		like := "%" + search + "%"
		query = query.Where("email LIKE ? OR name LIKE ?", like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	var users []models.User
	if err := query.
		Select("id", "name", "email", "phone", "is_active", "email_notify_marketing", "sms_notify_marketing", "created_at").
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
	if err := query.Select("id").Find(&users).Error; err != nil {
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
		TargetedUsers:      len(users),
		OperatorID:         operatorID,
		OperatorName:       operatorName,
	}

	tasks := make([]models.MarketingBatchTask, 0, len(users)*2)
	for i := range users {
		userID := users[i].ID
		if req.SendEmail {
			tasks = append(tasks, models.MarketingBatchTask{
				UserID:  userID,
				Channel: models.MarketingTaskChannelEmail,
				Status:  models.MarketingTaskStatusPending,
			})
		}
		if req.SendSMS {
			tasks = append(tasks, models.MarketingBatchTask{
				UserID:  userID,
				Channel: models.MarketingTaskChannelSMS,
				Status:  models.MarketingTaskStatusPending,
			})
		}
	}
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
