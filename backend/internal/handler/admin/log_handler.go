package admin

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"auralogic/internal/models"
	"auralogic/internal/pkg/logger"
	"auralogic/internal/pkg/response"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type LogHandler struct {
	db            *gorm.DB
	pluginManager *service.PluginManagerService
}

func NewLogHandler(db *gorm.DB, pluginManager *service.PluginManagerService) *LogHandler {
	return &LogHandler{
		db:            db,
		pluginManager: pluginManager,
	}
}

func buildEmailLogRetryHookPayload(emailLog *models.EmailLog) map[string]interface{} {
	if emailLog == nil {
		return map[string]interface{}{}
	}

	return map[string]interface{}{
		"email_id":      emailLog.ID,
		"to":            emailLog.ToEmail,
		"subject":       emailLog.Subject,
		"event_type":    emailLog.EventType,
		"order_id":      emailLog.OrderID,
		"user_id":       emailLog.UserID,
		"batch_id":      emailLog.BatchID,
		"status":        emailLog.Status,
		"error_message": emailLog.ErrorMessage,
		"retry_count":   emailLog.RetryCount,
		"expire_at":     emailLog.ExpireAt,
		"sent_at":       emailLog.SentAt,
		"created_at":    emailLog.CreatedAt,
		"updated_at":    emailLog.UpdatedAt,
	}
}

func buildEmailLogRetryHookPayloadList(emailLogs []models.EmailLog) []map[string]interface{} {
	if len(emailLogs) == 0 {
		return []map[string]interface{}{}
	}

	items := make([]map[string]interface{}, 0, len(emailLogs))
	for i := range emailLogs {
		items = append(items, buildEmailLogRetryHookPayload(&emailLogs[i]))
	}
	return items
}

func decodeRetryEmailIDs(value interface{}) ([]uint, error) {
	if value == nil {
		return nil, nil
	}

	body, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	var ids []uint
	if err := json.Unmarshal(body, &ids); err != nil {
		return nil, err
	}
	return ids, nil
}

func escapeLikePattern(value string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`%`, `\%`,
		`_`, `\_`,
	)
	return replacer.Replace(value)
}

func buildJSONContainsStringPairPattern(key, value string) string {
	keyJSON, _ := json.Marshal(key)
	valueJSON, _ := json.Marshal(value)
	return "%" + escapeLikePattern(string(keyJSON)+":"+string(valueJSON)) + "%"
}

func (h *LogHandler) buildOperationLogQuery(c *gin.Context) (*gorm.DB, error) {
	action := c.Query("action")
	resourceType := c.Query("resource_type")
	resourceID := strings.TrimSpace(c.Query("resource_id"))
	orderNo := strings.TrimSpace(c.Query("order_no"))
	userID := strings.TrimSpace(c.Query("user_id"))
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	query := h.db.Model(&models.OperationLog{}).Preload("User")

	if action != "" {
		query = query.Where("action = ?", action)
	}
	if resourceType != "" {
		query = query.Where("resource_type = ?", resourceType)
	}
	if resourceID != "" {
		query = query.Where("resource_id = ?", resourceID)
	}
	if orderNo != "" {
		orderNoPattern := buildJSONContainsStringPairPattern("order_no", orderNo)
		var matchedOrderIDs []uint
		if err := h.db.Model(&models.Order{}).Where("order_no = ?", orderNo).Pluck("id", &matchedOrderIDs).Error; err != nil {
			return nil, err
		}
		if len(matchedOrderIDs) > 0 {
			query = query.Where(
				"((resource_type = ? AND resource_id IN ?) OR details LIKE ? ESCAPE '\\')",
				"order",
				matchedOrderIDs,
				orderNoPattern,
			)
		} else {
			query = query.Where("details LIKE ? ESCAPE '\\'", orderNoPattern)
		}
	}
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	if startDate != "" {
		if t, err := time.Parse("2006-01-02", startDate); err == nil {
			query = query.Where("created_at >= ?", t)
		}
	}
	if endDate != "" {
		if t, err := time.Parse("2006-01-02", endDate); err == nil {
			query = query.Where("created_at <= ?", t.Add(24*time.Hour-time.Second))
		}
	}

	return query, nil
}

func (h *LogHandler) buildEmailLogQuery(c *gin.Context) *gorm.DB {
	status := c.Query("status")
	eventType := c.Query("event_type")
	toEmail := c.Query("to_email")
	batchID := c.Query("batch_id")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	query := h.db.Model(&models.EmailLog{}).Preload("User").Preload("Order").Preload("Batch")
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if eventType != "" {
		query = query.Where("event_type = ?", eventType)
	}
	if toEmail != "" {
		query = query.Where("to_email LIKE ?", "%"+toEmail+"%")
	}
	if batchID != "" {
		query = query.Where("batch_id = ?", batchID)
	}
	if startDate != "" {
		if t, err := time.Parse("2006-01-02", startDate); err == nil {
			query = query.Where("created_at >= ?", t)
		}
	}
	if endDate != "" {
		if t, err := time.Parse("2006-01-02", endDate); err == nil {
			query = query.Where("created_at <= ?", t.Add(24*time.Hour-time.Second))
		}
	}
	return query
}

func (h *LogHandler) buildSMSLogQuery(c *gin.Context) *gorm.DB {
	status := c.Query("status")
	eventType := c.Query("event_type")
	phone := c.Query("phone")
	batchID := c.Query("batch_id")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	query := h.db.Model(&models.SmsLog{}).Preload("User").Preload("Batch")
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if eventType != "" {
		query = query.Where("event_type = ?", eventType)
	}
	if phone != "" {
		query = query.Where("phone LIKE ?", "%"+phone+"%")
	}
	if batchID != "" {
		query = query.Where("batch_id = ?", batchID)
	}
	if startDate != "" {
		if t, err := time.Parse("2006-01-02", startDate); err == nil {
			query = query.Where("created_at >= ?", t)
		}
	}
	if endDate != "" {
		if t, err := time.Parse("2006-01-02", endDate); err == nil {
			query = query.Where("created_at <= ?", t.Add(24*time.Hour-time.Second))
		}
	}
	return query
}

// ListOperationLogs get操作日志列表
func (h *LogHandler) ListOperationLogs(c *gin.Context) {
	page, limit := response.GetPagination(c)

	var logs []models.OperationLog
	var total int64

	query, err := h.buildOperationLogQuery(c)
	if err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	// get总数
	if err := query.Count(&total).Error; err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	// 分页Query，按Create时间倒序
	offset := (page - 1) * limit
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&logs).Error; err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	response.Paginated(c, logs, page, limit, total)
}

// ListEmailLogs get邮件日志列表
func (h *LogHandler) ListEmailLogs(c *gin.Context) {
	page, limit := response.GetPagination(c)

	var logs []models.EmailLog
	var total int64

	query := h.buildEmailLogQuery(c)

	// get总数
	if err := query.Count(&total).Error; err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	// 分页Query，按Create时间倒序
	offset := (page - 1) * limit
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&logs).Error; err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	response.Paginated(c, logs, page, limit, total)
}

// ListSmsLogs get短信日志列表
func (h *LogHandler) ListSmsLogs(c *gin.Context) {
	page, limit := response.GetPagination(c)

	var logs []models.SmsLog
	var total int64

	query := h.buildSMSLogQuery(c)

	if err := query.Count(&total).Error; err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	offset := (page - 1) * limit
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&logs).Error; err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	// 返回脱敏内容
	type smsLogView struct {
		models.SmsLog
		MaskedContent string `json:"content"`
	}
	items := make([]smsLogView, len(logs))
	for i, l := range logs {
		items[i] = smsLogView{SmsLog: l, MaskedContent: models.MaskContent(l.Content)}
	}

	response.Paginated(c, items, page, limit, total)
}

func (h *LogHandler) ExportOperationLogs(c *gin.Context) {
	query, err := h.buildOperationLogQuery(c)
	if err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	var logs []models.OperationLog
	if err := query.Order("created_at DESC").Limit(adminCSVExportMaxRows + 1).Find(&logs).Error; err != nil {
		response.InternalError(c, "Query failed")
		return
	}
	if len(logs) > adminCSVExportMaxRows {
		response.BadRequest(c, fmt.Sprintf("Too many records to export (max %d). Please narrow the filters.", adminCSVExportMaxRows))
		return
	}

	rows := make([][]string, 0, len(logs))
	for _, item := range logs {
		userID := ""
		if item.UserID != nil {
			userID = strconv.FormatUint(uint64(*item.UserID), 10)
		}
		userEmail := ""
		if item.User != nil {
			userEmail = item.User.Email
		}
		resourceID := ""
		if item.ResourceID != nil {
			resourceID = strconv.FormatUint(uint64(*item.ResourceID), 10)
		}
		rows = append(rows, []string{
			strconv.FormatUint(uint64(item.ID), 10),
			userID,
			userEmail,
			item.OperatorName,
			item.Action,
			item.ResourceType,
			resourceID,
			csvJSONValue(item.Details),
			item.IPAddress,
			item.UserAgent,
			csvTimeValue(item.CreatedAt),
		})
	}

	logger.LogOperation(h.db, c, "export", "operation_log", nil, map[string]interface{}{
		"count":         len(rows),
		"action":        strings.TrimSpace(c.Query("action")),
		"resource_type": strings.TrimSpace(c.Query("resource_type")),
		"resource_id":   strings.TrimSpace(c.Query("resource_id")),
		"order_no":      strings.TrimSpace(c.Query("order_no")),
		"user_id":       strings.TrimSpace(c.Query("user_id")),
		"start_date":    strings.TrimSpace(c.Query("start_date")),
		"end_date":      strings.TrimSpace(c.Query("end_date")),
		"format":        "xlsx",
	})

	writeXLSXAttachment(c, buildAdminXLSXFileName("operation_logs"), "Operation Logs", []string{
		"ID",
		"User ID",
		"User Email",
		"Operator",
		"Action",
		"Resource Type",
		"Resource ID",
		"Details",
		"IP Address",
		"User Agent",
		"Created At",
	}, rows)
}

func (h *LogHandler) ExportEmailLogs(c *gin.Context) {
	query := h.buildEmailLogQuery(c)

	var logs []models.EmailLog
	if err := query.Order("created_at DESC").Limit(adminCSVExportMaxRows + 1).Find(&logs).Error; err != nil {
		response.InternalError(c, "Query failed")
		return
	}
	if len(logs) > adminCSVExportMaxRows {
		response.BadRequest(c, fmt.Sprintf("Too many records to export (max %d). Please narrow the filters.", adminCSVExportMaxRows))
		return
	}

	rows := make([][]string, 0, len(logs))
	for _, item := range logs {
		userID := ""
		userEmail := ""
		if item.UserID != nil {
			userID = strconv.FormatUint(uint64(*item.UserID), 10)
		}
		if item.User != nil {
			userEmail = item.User.Email
		}
		orderID := ""
		orderNo := ""
		if item.OrderID != nil {
			orderID = strconv.FormatUint(uint64(*item.OrderID), 10)
		}
		if item.Order != nil {
			orderNo = item.Order.OrderNo
		}
		batchID := ""
		batchNo := ""
		if item.BatchID != nil {
			batchID = strconv.FormatUint(uint64(*item.BatchID), 10)
		}
		if item.Batch != nil {
			batchNo = item.Batch.BatchNo
		}
		rows = append(rows, []string{
			strconv.FormatUint(uint64(item.ID), 10),
			item.ToEmail,
			item.Subject,
			item.EventType,
			userID,
			userEmail,
			orderID,
			orderNo,
			batchID,
			batchNo,
			string(item.Status),
			item.ErrorMessage,
			strconv.Itoa(item.RetryCount),
			csvTimePtrValue(item.ExpireAt),
			csvTimePtrValue(item.SentAt),
			csvTimeValue(item.CreatedAt),
			csvTimeValue(item.UpdatedAt),
		})
	}

	logger.LogOperation(h.db, c, "export", "email_log", nil, map[string]interface{}{
		"count":      len(rows),
		"status":     strings.TrimSpace(c.Query("status")),
		"event_type": strings.TrimSpace(c.Query("event_type")),
		"to_email":   strings.TrimSpace(c.Query("to_email")),
		"batch_id":   strings.TrimSpace(c.Query("batch_id")),
		"start_date": strings.TrimSpace(c.Query("start_date")),
		"end_date":   strings.TrimSpace(c.Query("end_date")),
		"format":     "xlsx",
	})

	writeXLSXAttachment(c, buildAdminXLSXFileName("email_logs"), "Email Logs", []string{
		"ID",
		"To Email",
		"Subject",
		"Event Type",
		"User ID",
		"User Email",
		"Order ID",
		"Order No",
		"Batch ID",
		"Batch No",
		"Status",
		"Error Message",
		"Retry Count",
		"Expire At",
		"Sent At",
		"Created At",
		"Updated At",
	}, rows)
}

func (h *LogHandler) ExportSmsLogs(c *gin.Context) {
	query := h.buildSMSLogQuery(c)

	var logs []models.SmsLog
	if err := query.Order("created_at DESC").Limit(adminCSVExportMaxRows + 1).Find(&logs).Error; err != nil {
		response.InternalError(c, "Query failed")
		return
	}
	if len(logs) > adminCSVExportMaxRows {
		response.BadRequest(c, fmt.Sprintf("Too many records to export (max %d). Please narrow the filters.", adminCSVExportMaxRows))
		return
	}

	rows := make([][]string, 0, len(logs))
	for _, item := range logs {
		userID := ""
		userEmail := ""
		if item.UserID != nil {
			userID = strconv.FormatUint(uint64(*item.UserID), 10)
		}
		if item.User != nil {
			userEmail = item.User.Email
		}
		batchID := ""
		batchNo := ""
		if item.BatchID != nil {
			batchID = strconv.FormatUint(uint64(*item.BatchID), 10)
		}
		if item.Batch != nil {
			batchNo = item.Batch.BatchNo
		}
		rows = append(rows, []string{
			strconv.FormatUint(uint64(item.ID), 10),
			item.Phone,
			models.MaskContent(item.Content),
			item.EventType,
			userID,
			userEmail,
			batchID,
			batchNo,
			item.Provider,
			string(item.Status),
			item.ErrorMessage,
			csvTimePtrValue(item.ExpireAt),
			csvTimePtrValue(item.SentAt),
			csvTimeValue(item.CreatedAt),
			csvTimeValue(item.UpdatedAt),
		})
	}

	logger.LogOperation(h.db, c, "export", "sms_log", nil, map[string]interface{}{
		"count":      len(rows),
		"status":     strings.TrimSpace(c.Query("status")),
		"event_type": strings.TrimSpace(c.Query("event_type")),
		"phone":      strings.TrimSpace(c.Query("phone")),
		"batch_id":   strings.TrimSpace(c.Query("batch_id")),
		"start_date": strings.TrimSpace(c.Query("start_date")),
		"end_date":   strings.TrimSpace(c.Query("end_date")),
		"format":     "xlsx",
	})

	writeXLSXAttachment(c, buildAdminXLSXFileName("sms_logs"), "SMS Logs", []string{
		"ID",
		"Phone",
		"Content",
		"Event Type",
		"User ID",
		"User Email",
		"Batch ID",
		"Batch No",
		"Provider",
		"Status",
		"Error Message",
		"Expire At",
		"Sent At",
		"Created At",
		"Updated At",
	}, rows)
}

// GetLogStatistics get日志统计Info
func (h *LogHandler) GetLogStatistics(c *gin.Context) {
	var stats struct {
		OperationLogCount struct {
			Today int64 `json:"today"`
			Week  int64 `json:"week"`
			Month int64 `json:"month"`
			Total int64 `json:"total"`
		} `json:"operation_log_count"`
		EmailLogCount struct {
			Today   int64 `json:"today"`
			Week    int64 `json:"week"`
			Month   int64 `json:"month"`
			Total   int64 `json:"total"`
			Pending int64 `json:"pending"`
			Failed  int64 `json:"failed"`
			Expired int64 `json:"expired"`
		} `json:"email_log_count"`
		SmsLogCount struct {
			Today   int64 `json:"today"`
			Week    int64 `json:"week"`
			Month   int64 `json:"month"`
			Total   int64 `json:"total"`
			Pending int64 `json:"pending"`
			Failed  int64 `json:"failed"`
			Expired int64 `json:"expired"`
		} `json:"sms_log_count"`
		TopActions []struct {
			Action string `json:"action"`
			Count  int64  `json:"count"`
		} `json:"top_actions"`
	}

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).UTC()
	weekStart := todayStart.AddDate(0, 0, -7)
	monthStart := todayStart.AddDate(0, -1, 0)

	// 操作日志统计
	h.db.Model(&models.OperationLog{}).Count(&stats.OperationLogCount.Total)
	h.db.Model(&models.OperationLog{}).Where("created_at >= ?", todayStart).Count(&stats.OperationLogCount.Today)
	h.db.Model(&models.OperationLog{}).Where("created_at >= ?", weekStart).Count(&stats.OperationLogCount.Week)
	h.db.Model(&models.OperationLog{}).Where("created_at >= ?", monthStart).Count(&stats.OperationLogCount.Month)

	// 邮件日志统计
	h.db.Model(&models.EmailLog{}).Count(&stats.EmailLogCount.Total)
	h.db.Model(&models.EmailLog{}).Where("created_at >= ?", todayStart).Count(&stats.EmailLogCount.Today)
	h.db.Model(&models.EmailLog{}).Where("created_at >= ?", weekStart).Count(&stats.EmailLogCount.Week)
	h.db.Model(&models.EmailLog{}).Where("created_at >= ?", monthStart).Count(&stats.EmailLogCount.Month)
	h.db.Model(&models.EmailLog{}).Where("status = ?", models.EmailLogStatusPending).Count(&stats.EmailLogCount.Pending)
	h.db.Model(&models.EmailLog{}).Where("status = ?", models.EmailLogStatusFailed).Count(&stats.EmailLogCount.Failed)
	h.db.Model(&models.EmailLog{}).Where("status = ?", models.EmailLogStatusExpired).Count(&stats.EmailLogCount.Expired)

	// 短信日志统计
	h.db.Model(&models.SmsLog{}).Count(&stats.SmsLogCount.Total)
	h.db.Model(&models.SmsLog{}).Where("created_at >= ?", todayStart).Count(&stats.SmsLogCount.Today)
	h.db.Model(&models.SmsLog{}).Where("created_at >= ?", weekStart).Count(&stats.SmsLogCount.Week)
	h.db.Model(&models.SmsLog{}).Where("created_at >= ?", monthStart).Count(&stats.SmsLogCount.Month)
	h.db.Model(&models.SmsLog{}).Where("status = ?", models.SmsLogStatusPending).Count(&stats.SmsLogCount.Pending)
	h.db.Model(&models.SmsLog{}).Where("status = ?", models.SmsLogStatusFailed).Count(&stats.SmsLogCount.Failed)
	h.db.Model(&models.SmsLog{}).Where("status = ?", models.SmsLogStatusExpired).Count(&stats.SmsLogCount.Expired)

	// 热门操作统计
	h.db.Model(&models.OperationLog{}).
		Select("action, COUNT(*) as count").
		Group("action").
		Order("count DESC").
		Limit(10).
		Scan(&stats.TopActions)

	response.Success(c, stats)
}

// RetryEmailRequest 重试邮件发送请求
type RetryEmailRequest struct {
	EmailIDs []uint `json:"email_ids" binding:"required"`
}

// RetryFailedEmails Retry failed的邮件
func (h *LogHandler) RetryFailedEmails(c *gin.Context) {
	var req RetryEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}
	var beforeLogs []models.EmailLog
	if len(req.EmailIDs) > 0 {
		if err := h.db.Where("id IN ?", req.EmailIDs).Find(&beforeLogs).Error; err != nil {
			response.InternalError(c, "Query failed")
			return
		}
	}
	if h.pluginManager != nil {
		hookPayload := map[string]interface{}{
			"email_ids":   req.EmailIDs,
			"email_count": len(req.EmailIDs),
			"emails":      buildEmailLogRetryHookPayloadList(beforeLogs),
			"admin_id":    adminIDValue,
			"source":      "admin_api",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "log.email.retry.before",
			Payload: hookPayload,
		}, buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "email_log",
			"hook_source":   "admin_api",
			"hook_action":   "retry",
		}))
		if hookErr != nil {
			log.Printf("log.email.retry.before hook execution failed: admin=%d count=%d err=%v", adminIDValue, len(req.EmailIDs), hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Email retry rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if rawIDs, exists := hookResult.Payload["email_ids"]; exists {
					updatedIDs, decodeErr := decodeRetryEmailIDs(rawIDs)
					if decodeErr != nil {
						log.Printf("log.email.retry.before email_ids patch ignored: admin=%d err=%v", adminIDValue, decodeErr)
					} else {
						req.EmailIDs = updatedIDs
					}
				}
			}
		}
	}
	if len(req.EmailIDs) == 0 {
		response.BadRequest(c, "No email IDs provided")
		return
	}

	// Update状态为待发送，重置过期时间
	newExpire := time.Now().Add(30 * time.Minute)
	result := h.db.Model(&models.EmailLog{}).
		Where("id IN ? AND status IN ?", req.EmailIDs, []models.EmailLogStatus{models.EmailLogStatusFailed, models.EmailLogStatusExpired}).
		Updates(map[string]interface{}{
			"status":     models.EmailLogStatusPending,
			"sent_at":    nil,
			"expire_at":  newExpire,
			"updated_at": time.Now(),
		})

	if result.Error != nil {
		response.InternalError(c, "Retry failed")
		return
	}

	if h.pluginManager != nil {
		var afterLogs []models.EmailLog
		if err := h.db.Where("id IN ?", req.EmailIDs).Find(&afterLogs).Error; err != nil {
			log.Printf("log.email.retry.after reload failed: admin=%d count=%d err=%v", adminIDValue, len(req.EmailIDs), err)
		}
		afterPayload := map[string]interface{}{
			"email_ids":     req.EmailIDs,
			"email_count":   len(req.EmailIDs),
			"affected":      result.RowsAffected,
			"new_expire_at": newExpire,
			"emails":        buildEmailLogRetryHookPayloadList(afterLogs),
			"admin_id":      adminIDValue,
			"source":        "admin_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, affected int64) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "log.email.retry.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("log.email.retry.after hook execution failed: admin=%d affected=%d err=%v", adminIDValue, affected, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "email_log",
			"hook_source":   "admin_api",
			"hook_action":   "retry",
		})), afterPayload, result.RowsAffected)
	}

	response.Success(c, gin.H{
		"message":  "Email re-added to send queue",
		"affected": result.RowsAffected,
	})
}
