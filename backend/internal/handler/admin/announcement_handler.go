package admin

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"auralogic/internal/models"
	"auralogic/internal/pkg/logger"
	"auralogic/internal/pkg/response"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AnnouncementHandler struct {
	db            *gorm.DB
	emailService  *service.EmailService
	smsService    *service.SMSService
	pluginManager *service.PluginManagerService
}

func NewAnnouncementHandler(db *gorm.DB, emailService *service.EmailService, smsService *service.SMSService, pluginManager *service.PluginManagerService) *AnnouncementHandler {
	return &AnnouncementHandler{
		db:            db,
		emailService:  emailService,
		smsService:    smsService,
		pluginManager: pluginManager,
	}
}

func normalizeAnnouncementCategory(category string) string {
	c := strings.TrimSpace(strings.ToLower(category))
	if c == "" {
		return "general"
	}
	if c != "general" && c != "marketing" {
		return ""
	}
	return c
}

func buildAnnouncementHookPayload(announcement *models.Announcement) map[string]interface{} {
	if announcement == nil {
		return map[string]interface{}{}
	}

	return map[string]interface{}{
		"announcement_id":   announcement.ID,
		"title":             announcement.Title,
		"content":           announcement.Content,
		"category":          announcement.Category,
		"send_email":        announcement.SendEmail,
		"send_sms":          announcement.SendSMS,
		"is_mandatory":      announcement.IsMandatory,
		"require_full_read": announcement.RequireFullRead,
		"created_at":        announcement.CreatedAt,
		"updated_at":        announcement.UpdatedAt,
	}
}

func (h *AnnouncementHandler) dispatchAnnouncement(announcement *models.Announcement) {
	if announcement == nil {
		return
	}
	if !announcement.SendEmail && !announcement.SendSMS {
		return
	}

	var users []models.User
	if err := h.db.Where("is_active = ?", true).Find(&users).Error; err != nil {
		log.Printf("dispatchAnnouncement query users failed: %v", err)
		return
	}

	for i := range users {
		user := &users[i]
		if announcement.SendEmail && h.emailService != nil {
			if err := h.emailService.SendMarketingAnnouncementEmail(user, announcement.Title, announcement.Content); err != nil {
				log.Printf("dispatchAnnouncement email failed, user=%d: %v", user.ID, err)
			}
		}
		if announcement.SendSMS && h.smsService != nil {
			if err := h.smsService.SendMarketingSMS(user, announcement.Content); err != nil {
				log.Printf("dispatchAnnouncement sms failed, user=%d: %v", user.ID, err)
			}
		}
	}
}

func (h *AnnouncementHandler) buildAnnouncementListQuery(c *gin.Context) *gorm.DB {
	search := strings.TrimSpace(c.Query("search"))
	mandatory := strings.TrimSpace(c.Query("is_mandatory"))
	category := normalizeAnnouncementCategory(c.Query("category"))

	query := h.db.Model(&models.Announcement{})
	if search != "" {
		query = query.Where("title LIKE ?", "%"+search+"%")
	}
	if mandatory == "true" {
		query = query.Where("is_mandatory = ?", true)
	} else if mandatory == "false" {
		query = query.Where("is_mandatory = ?", false)
	}
	if category != "" {
		query = query.Where("category = ?", category)
	}

	return query
}

// ListAnnouncements 公告列表
func (h *AnnouncementHandler) ListAnnouncements(c *gin.Context) {
	page, limit := response.GetPagination(c)
	query := h.buildAnnouncementListQuery(c)

	var total int64
	query.Count(&total)

	var announcements []models.Announcement
	if err := query.Order("id DESC").
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&announcements).Error; err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	response.Paginated(c, announcements, page, limit, total)
}

// ExportAnnouncements 导出公告
func (h *AnnouncementHandler) ExportAnnouncements(c *gin.Context) {
	query := h.buildAnnouncementListQuery(c)

	var announcements []models.Announcement
	if err := query.Order("id DESC").Limit(adminCSVExportMaxRows + 1).Find(&announcements).Error; err != nil {
		response.InternalError(c, "Query failed")
		return
	}
	if len(announcements) > adminCSVExportMaxRows {
		response.BadRequest(c, fmt.Sprintf("Too many records to export (max %d). Please narrow the filters.", adminCSVExportMaxRows))
		return
	}

	announcementIDs := make([]uint, 0, len(announcements))
	for _, item := range announcements {
		announcementIDs = append(announcementIDs, item.ID)
	}

	readCountByAnnouncementID := make(map[uint]int64, len(announcementIDs))
	if len(announcementIDs) > 0 {
		var rows []struct {
			AnnouncementID uint  `gorm:"column:announcement_id"`
			Cnt            int64 `gorm:"column:cnt"`
		}
		if err := h.db.Model(&models.AnnouncementRead{}).
			Select("announcement_id, COUNT(*) as cnt").
			Where("announcement_id IN ?", announcementIDs).
			Group("announcement_id").
			Scan(&rows).Error; err != nil {
			response.InternalError(c, "Query failed")
			return
		}
		for _, row := range rows {
			readCountByAnnouncementID[row.AnnouncementID] = row.Cnt
		}
	}

	rows := make([][]string, 0, len(announcements))
	for _, item := range announcements {
		rows = append(rows, []string{
			strconv.FormatUint(uint64(item.ID), 10),
			item.Title,
			item.Content,
			item.Category,
			strconv.FormatBool(item.SendEmail),
			strconv.FormatBool(item.SendSMS),
			strconv.FormatBool(item.IsMandatory),
			strconv.FormatBool(item.RequireFullRead),
			strconv.FormatInt(readCountByAnnouncementID[item.ID], 10),
			csvTimeValue(item.CreatedAt),
			csvTimeValue(item.UpdatedAt),
		})
	}

	logger.LogOperation(h.db, c, "export", "announcement", nil, map[string]interface{}{
		"count":        len(rows),
		"search":       strings.TrimSpace(c.Query("search")),
		"is_mandatory": strings.TrimSpace(c.Query("is_mandatory")),
		"category":     strings.TrimSpace(c.Query("category")),
		"format":       "xlsx",
	})

	writeXLSXAttachment(c, buildAdminXLSXFileName("announcements"), "Announcements", []string{
		"ID",
		"Title",
		"Content",
		"Category",
		"Send Email",
		"Send SMS",
		"Is Mandatory",
		"Require Full Read",
		"Read Count",
		"Created At",
		"Updated At",
	}, rows)
}

// CreateAnnouncement 创建公告
func (h *AnnouncementHandler) CreateAnnouncement(c *gin.Context) {
	var req struct {
		Title           string `json:"title" binding:"required"`
		Content         string `json:"content"`
		Category        string `json:"category"`
		SendEmail       bool   `json:"send_email"`
		SendSMS         bool   `json:"send_sms"`
		IsMandatory     bool   `json:"is_mandatory"`
		RequireFullRead bool   `json:"require_full_read"`
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
			log.Printf("announcement.create.before payload build failed: admin=%d err=%v", adminIDValue, payloadErr)
		} else {
			hookPayload["admin_id"] = adminIDValue
			hookPayload["source"] = "admin_api"
			hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "announcement.create.before",
				Payload: hookPayload,
			}, buildAdminHookExecutionContext(c, adminID, map[string]string{
				"hook_resource": "announcement",
				"hook_source":   "admin_api",
			}))
			if hookErr != nil {
				log.Printf("announcement.create.before hook execution failed: admin=%d err=%v", adminIDValue, hookErr)
			} else if hookResult != nil {
				if hookResult.Blocked {
					reason := strings.TrimSpace(hookResult.BlockReason)
					if reason == "" {
						reason = "Announcement creation rejected by plugin"
					}
					response.BadRequest(c, reason)
					return
				}
				if hookResult.Payload != nil {
					if mergeErr := mergeAdminHookStructPatch(&req, hookResult.Payload); mergeErr != nil {
						log.Printf("announcement.create.before payload apply failed, fallback to original request: admin=%d err=%v", adminIDValue, mergeErr)
						req = originalReq
					}
				}
			}
		}
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		response.BadRequest(c, "Title is required")
		return
	}

	category := normalizeAnnouncementCategory(req.Category)
	if category == "" {
		response.BadRequest(c, "Invalid category")
		return
	}

	announcement := models.Announcement{
		Title:           req.Title,
		Content:         req.Content,
		Category:        category,
		SendEmail:       req.SendEmail,
		SendSMS:         req.SendSMS,
		IsMandatory:     req.IsMandatory,
		RequireFullRead: req.RequireFullRead,
	}
	if err := h.db.Create(&announcement).Error; err != nil {
		response.InternalError(c, "CreateFailed")
		return
	}

	if h.pluginManager != nil {
		afterPayload := buildAnnouncementHookPayload(&announcement)
		afterPayload["admin_id"] = adminIDValue
		afterPayload["source"] = "admin_api"
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, announcementID uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "announcement.create.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("announcement.create.after hook execution failed: admin=%d announcement=%d err=%v", adminIDValue, announcementID, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource":   "announcement",
			"hook_source":     "admin_api",
			"announcement_id": strconv.FormatUint(uint64(announcement.ID), 10),
		})), afterPayload, announcement.ID)
	}

	// Async dispatch: do not block admin request on bulk sending.
	go h.dispatchAnnouncement(&announcement)

	response.Success(c, announcement)
}

// GetAnnouncement 获取公告详情
func (h *AnnouncementHandler) GetAnnouncement(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}

	var announcement models.Announcement
	if err := h.db.First(&announcement, uint(id)).Error; err != nil {
		response.NotFound(c, "Announcement not found")
		return
	}
	response.Success(c, announcement)
}

// UpdateAnnouncement 更新公告
func (h *AnnouncementHandler) UpdateAnnouncement(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}

	var announcement models.Announcement
	if err := h.db.First(&announcement, uint(id)).Error; err != nil {
		response.NotFound(c, "Announcement not found")
		return
	}

	var req struct {
		Title           string `json:"title"`
		Content         string `json:"content"`
		Category        string `json:"category"`
		SendEmail       *bool  `json:"send_email"`
		SendSMS         *bool  `json:"send_sms"`
		IsMandatory     *bool  `json:"is_mandatory"`
		RequireFullRead *bool  `json:"require_full_read"`
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
			log.Printf("announcement.update.before payload build failed: admin=%d announcement=%d err=%v", adminIDValue, uint(id), payloadErr)
		} else {
			hookPayload["announcement_id"] = uint(id)
			hookPayload["current"] = buildAnnouncementHookPayload(&announcement)
			hookPayload["admin_id"] = adminIDValue
			hookPayload["source"] = "admin_api"
			hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "announcement.update.before",
				Payload: hookPayload,
			}, buildAdminHookExecutionContext(c, adminID, map[string]string{
				"hook_resource":   "announcement",
				"hook_source":     "admin_api",
				"announcement_id": strconv.FormatUint(id, 10),
			}))
			if hookErr != nil {
				log.Printf("announcement.update.before hook execution failed: admin=%d announcement=%d err=%v", adminIDValue, uint(id), hookErr)
			} else if hookResult != nil {
				if hookResult.Blocked {
					reason := strings.TrimSpace(hookResult.BlockReason)
					if reason == "" {
						reason = "Announcement update rejected by plugin"
					}
					response.BadRequest(c, reason)
					return
				}
				if hookResult.Payload != nil {
					if mergeErr := mergeAdminHookStructPatch(&req, hookResult.Payload); mergeErr != nil {
						log.Printf("announcement.update.before payload apply failed, fallback to original request: admin=%d announcement=%d err=%v", adminIDValue, uint(id), mergeErr)
						req = originalReq
					}
				}
			}
		}
	}
	beforeAnnouncement := announcement

	if req.Title != "" {
		announcement.Title = strings.TrimSpace(req.Title)
	}
	if req.Content != "" {
		announcement.Content = req.Content
	}
	if strings.TrimSpace(req.Category) != "" {
		category := normalizeAnnouncementCategory(req.Category)
		if category == "" {
			response.BadRequest(c, "Invalid category")
			return
		}
		announcement.Category = category
	}
	if req.SendEmail != nil {
		announcement.SendEmail = *req.SendEmail
	}
	if req.SendSMS != nil {
		announcement.SendSMS = *req.SendSMS
	}
	if req.IsMandatory != nil {
		announcement.IsMandatory = *req.IsMandatory
	}
	if req.RequireFullRead != nil {
		announcement.RequireFullRead = *req.RequireFullRead
	}

	if err := h.db.Save(&announcement).Error; err != nil {
		response.InternalError(c, "UpdateFailed")
		return
	}
	if h.pluginManager != nil {
		afterPayload := buildAnnouncementHookPayload(&announcement)
		afterPayload["before_title"] = beforeAnnouncement.Title
		afterPayload["before_content"] = beforeAnnouncement.Content
		afterPayload["before_category"] = beforeAnnouncement.Category
		afterPayload["before_send_email"] = beforeAnnouncement.SendEmail
		afterPayload["before_send_sms"] = beforeAnnouncement.SendSMS
		afterPayload["before_is_mandatory"] = beforeAnnouncement.IsMandatory
		afterPayload["before_require_full_read"] = beforeAnnouncement.RequireFullRead
		afterPayload["admin_id"] = adminIDValue
		afterPayload["source"] = "admin_api"
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "announcement.update.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("announcement.update.after hook execution failed: admin=%d announcement=%d err=%v", adminIDValue, uint(id), hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource":   "announcement",
			"hook_source":     "admin_api",
			"announcement_id": strconv.FormatUint(id, 10),
		})), afterPayload)
	}
	response.Success(c, announcement)
}

// DeleteAnnouncement 删除公告
func (h *AnnouncementHandler) DeleteAnnouncement(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}
	var announcement models.Announcement
	if err := h.db.First(&announcement, uint(id)).Error; err != nil {
		response.NotFound(c, "Announcement not found")
		return
	}
	if h.pluginManager != nil {
		hookPayload := buildAnnouncementHookPayload(&announcement)
		hookPayload["admin_id"] = adminIDValue
		hookPayload["source"] = "admin_api"
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "announcement.delete.before",
			Payload: hookPayload,
		}, buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource":   "announcement",
			"hook_source":     "admin_api",
			"announcement_id": strconv.FormatUint(id, 10),
		}))
		if hookErr != nil {
			log.Printf("announcement.delete.before hook execution failed: admin=%d announcement=%d err=%v", adminIDValue, uint(id), hookErr)
		} else if hookResult != nil && hookResult.Blocked {
			reason := strings.TrimSpace(hookResult.BlockReason)
			if reason == "" {
				reason = "Announcement deletion rejected by plugin"
			}
			response.BadRequest(c, reason)
			return
		}
	}

	if err := h.db.Delete(&models.Announcement{}, uint(id)).Error; err != nil {
		response.InternalError(c, "DeleteFailed")
		return
	}

	// 同时清理已读记录
	h.db.Where("announcement_id = ?", uint(id)).Delete(&models.AnnouncementRead{})

	if h.pluginManager != nil {
		afterPayload := buildAnnouncementHookPayload(&announcement)
		afterPayload["admin_id"] = adminIDValue
		afterPayload["source"] = "admin_api"
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "announcement.delete.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("announcement.delete.after hook execution failed: admin=%d announcement=%d err=%v", adminIDValue, uint(id), hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource":   "announcement",
			"hook_source":     "admin_api",
			"announcement_id": strconv.FormatUint(id, 10),
		})), afterPayload)
	}

	response.Success(c, gin.H{"message": "Announcement deleted"})
}
