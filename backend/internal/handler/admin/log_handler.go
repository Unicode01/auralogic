package admin

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"auralogic/internal/models"
	"auralogic/internal/pkg/response"
	"gorm.io/gorm"
)

type LogHandler struct {
	db *gorm.DB
}

func NewLogHandler(db *gorm.DB) *LogHandler {
	return &LogHandler{db: db}
}

// ListOperationLogs get操作日志列表
func (h *LogHandler) ListOperationLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	action := c.Query("action")
	resourceType := c.Query("resource_type")
	userID := c.Query("user_id")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	if limit > 100 {
		limit = 100
	}

	var logs []models.OperationLog
	var total int64

	query := h.db.Model(&models.OperationLog{}).Preload("User")

	// 过滤条件
	if action != "" {
		query = query.Where("action = ?", action)
	}
	if resourceType != "" {
		query = query.Where("resource_type = ?", resourceType)
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
			// 包含当天结束时间
			t = t.Add(24*time.Hour - time.Second)
			query = query.Where("created_at <= ?", t)
		}
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
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	status := c.Query("status")
	eventType := c.Query("event_type")
	toEmail := c.Query("to_email")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	if limit > 100 {
		limit = 100
	}

	var logs []models.EmailLog
	var total int64

	query := h.db.Model(&models.EmailLog{}).Preload("User").Preload("Order")

	// 过滤条件
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if eventType != "" {
		query = query.Where("event_type = ?", eventType)
	}
	if toEmail != "" {
		query = query.Where("to_email LIKE ?", "%"+toEmail+"%")
	}
	if startDate != "" {
		if t, err := time.Parse("2006-01-02", startDate); err == nil {
			query = query.Where("created_at >= ?", t)
		}
	}
	if endDate != "" {
		if t, err := time.Parse("2006-01-02", endDate); err == nil {
			t = t.Add(24*time.Hour - time.Second)
			query = query.Where("created_at <= ?", t)
		}
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
		} `json:"email_log_count"`
		TopActions []struct {
			Action string `json:"action"`
			Count  int64  `json:"count"`
		} `json:"top_actions"`
	}

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
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

	// Update状态为待发送
	result := h.db.Model(&models.EmailLog{}).
		Where("id IN ? AND status = ?", req.EmailIDs, models.EmailLogStatusFailed).
		Updates(map[string]interface{}{
			"status":     models.EmailLogStatusPending,
			"sent_at":    nil,
			"updated_at": time.Now(),
		})

	if result.Error != nil {
		response.InternalError(c, "Retry failed")
		return
	}

	response.Success(c, gin.H{
		"message":  "已将邮件重新加入发送队列",
		"affected": result.RowsAffected,
	})
}

