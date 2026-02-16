package user

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"auralogic/internal/middleware"
	"auralogic/internal/models"
	"auralogic/internal/pkg/response"
	"gorm.io/gorm"
)

type AnnouncementHandler struct {
	db *gorm.DB
}

func NewAnnouncementHandler(db *gorm.DB) *AnnouncementHandler {
	return &AnnouncementHandler{db: db}
}

// ListAnnouncements 公告列表（带已读状态）
func (h *AnnouncementHandler) ListAnnouncements(c *gin.Context) {
	userID := middleware.MustGetUserID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	if limit > 100 {
		limit = 100
	}

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
	userID := middleware.MustGetUserID(c)
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
}

// GetUnreadMandatory 获取未读的强制公告
func (h *AnnouncementHandler) GetUnreadMandatory(c *gin.Context) {
	userID := middleware.MustGetUserID(c)

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
	userID := middleware.MustGetUserID(c)
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

	// 创建已读记录（忽略重复）
	readRecord := models.AnnouncementRead{
		AnnouncementID: uint(id),
		UserID:         userID,
		ReadAt:         time.Now(),
	}
	// 使用 FirstOrCreate 避免重复插入
	h.db.Where("announcement_id = ? AND user_id = ?", uint(id), userID).
		FirstOrCreate(&readRecord)

	response.Success(c, gin.H{"message": "Marked as read"})
}
