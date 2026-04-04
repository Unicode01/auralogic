package user

import (
	"log"
	"strconv"
	"time"

	"auralogic/internal/middleware"
	"auralogic/internal/models"
	"auralogic/internal/pkg/response"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AnnouncementHandler struct {
	db            *gorm.DB
	pluginManager *service.PluginManagerService
}

func NewAnnouncementHandler(db *gorm.DB, pluginManager *service.PluginManagerService) *AnnouncementHandler {
	return &AnnouncementHandler{
		db:            db,
		pluginManager: pluginManager,
	}
}

func buildUserAnnouncementHookPayload(announcement *models.Announcement) map[string]interface{} {
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

// ListAnnouncements 公告列表（带已读状态）
func (h *AnnouncementHandler) ListAnnouncements(c *gin.Context) {
	userID, userIDOK := middleware.RequireUserID(c)
	if !userIDOK {
		return
	}
	page, limit := response.GetPagination(c)

	query := h.db.Model(&models.Announcement{})

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

	// 查询已读记录
	var readRecords []models.AnnouncementRead
	announcementIDs := make([]uint, len(announcements))
	for i, a := range announcements {
		announcementIDs[i] = a.ID
	}
	if len(announcementIDs) > 0 {
		h.db.Where("user_id = ? AND announcement_id IN ?", userID, announcementIDs).Find(&readRecords)
	}

	readMap := make(map[uint]bool)
	for _, r := range readRecords {
		readMap[r.AnnouncementID] = true
	}

	// 构建带已读状态的响应
	type AnnouncementWithRead struct {
		models.Announcement
		IsRead bool `json:"is_read"`
	}

	result := make([]AnnouncementWithRead, len(announcements))
	for i, a := range announcements {
		result[i] = AnnouncementWithRead{
			Announcement: a,
			IsRead:       readMap[a.ID],
		}
	}

	response.Paginated(c, result, page, limit, total)
}

// GetAnnouncement 公告详情（带已读状态）
func (h *AnnouncementHandler) GetAnnouncement(c *gin.Context) {
	userID, userIDOK := middleware.RequireUserID(c)
	if !userIDOK {
		return
	}
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

	// 检查已读状态
	var readRecord models.AnnouncementRead
	isRead := h.db.Where("announcement_id = ? AND user_id = ?", announcement.ID, userID).First(&readRecord).Error == nil

	type AnnouncementWithRead struct {
		models.Announcement
		IsRead bool `json:"is_read"`
	}

	response.Success(c, AnnouncementWithRead{
		Announcement: announcement,
		IsRead:       isRead,
	})
	if h.pluginManager != nil {
		payload := buildUserAnnouncementHookPayload(&announcement)
		payload["user_id"] = userID
		payload["is_read"] = isRead
		payload["source"] = "user_api"
		go func(execCtx *service.ExecutionContext, hookPayload map[string]interface{}, announcementID uint, uid uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "announcement.view.after",
				Payload: hookPayload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("announcement.view.after hook execution failed: user=%d announcement=%d err=%v", uid, announcementID, hookErr)
			}
		}(cloneUserHookExecutionContext(buildUserHookExecutionContext(c, userID, map[string]string{
			"hook_resource":   "announcement",
			"hook_source":     "user_api",
			"hook_action":     "view",
			"announcement_id": strconv.FormatUint(id, 10),
		})), payload, announcement.ID, userID)
	}
}

// GetUnreadMandatory 获取未读的强制公告
func (h *AnnouncementHandler) GetUnreadMandatory(c *gin.Context) {
	userID, userIDOK := middleware.RequireUserID(c)
	if !userIDOK {
		return
	}

	var announcements []models.Announcement
	if err := h.db.Where("is_mandatory = ?", true).
		Where("id NOT IN (?)",
			h.db.Model(&models.AnnouncementRead{}).
				Select("announcement_id").
				Where("user_id = ?", userID),
		).
		Order("id ASC").
		Find(&announcements).Error; err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	response.Success(c, announcements)
}

// MarkAsRead 标记公告为已读
func (h *AnnouncementHandler) MarkAsRead(c *gin.Context) {
	userID, userIDOK := middleware.RequireUserID(c)
	if !userIDOK {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}

	// 确认公告存在
	var announcement models.Announcement
	if err := h.db.First(&announcement, uint(id)).Error; err != nil {
		response.NotFound(c, "Announcement not found")
		return
	}
	var existingRead models.AnnouncementRead
	alreadyRead := h.db.Where("announcement_id = ? AND user_id = ?", uint(id), userID).First(&existingRead).Error == nil

	// 创建已读记录（忽略重复）
	readRecord := models.AnnouncementRead{
		AnnouncementID: uint(id),
		UserID:         userID,
		ReadAt:         time.Now(),
	}
	// 使用 FirstOrCreate 避免重复插入
	h.db.Where("announcement_id = ? AND user_id = ?", uint(id), userID).
		FirstOrCreate(&readRecord)
	if alreadyRead {
		readRecord = existingRead
	}
	if h.pluginManager != nil {
		payload := buildUserAnnouncementHookPayload(&announcement)
		payload["user_id"] = userID
		payload["already_read"] = alreadyRead
		payload["read_at"] = readRecord.ReadAt
		payload["source"] = "user_api"
		go func(execCtx *service.ExecutionContext, hookPayload map[string]interface{}, announcementID uint, uid uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "announcement.read.after",
				Payload: hookPayload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("announcement.read.after hook execution failed: user=%d announcement=%d err=%v", uid, announcementID, hookErr)
			}
		}(cloneUserHookExecutionContext(buildUserHookExecutionContext(c, userID, map[string]string{
			"hook_resource":   "announcement",
			"hook_source":     "user_api",
			"hook_action":     "read",
			"announcement_id": strconv.FormatUint(id, 10),
		})), payload, announcement.ID, userID)
	}

	response.Success(c, gin.H{"message": "Marked as read"})
}
