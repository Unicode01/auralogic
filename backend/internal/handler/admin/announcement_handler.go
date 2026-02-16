package admin

import (
	"strconv"

	"github.com/gin-gonic/gin"
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

// ListAnnouncements 公告列表
func (h *AnnouncementHandler) ListAnnouncements(c *gin.Context) {
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

	response.Paginated(c, announcements, page, limit, total)
}

// CreateAnnouncement 创建公告
func (h *AnnouncementHandler) CreateAnnouncement(c *gin.Context) {
	var req struct {
		Title           string `json:"title" binding:"required"`
		Content         string `json:"content"`
		IsMandatory     bool   `json:"is_mandatory"`
		RequireFullRead bool   `json:"require_full_read"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	announcement := models.Announcement{
		Title:           req.Title,
		Content:         req.Content,
		IsMandatory:     req.IsMandatory,
		RequireFullRead: req.RequireFullRead,
	}
	if err := h.db.Create(&announcement).Error; err != nil {
		response.InternalError(c, "CreateFailed")
		return
	}
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
		IsMandatory     *bool  `json:"is_mandatory"`
		RequireFullRead *bool  `json:"require_full_read"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	if req.Title != "" {
		announcement.Title = req.Title
	}
	if req.Content != "" {
		announcement.Content = req.Content
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
	response.Success(c, announcement)
}

// DeleteAnnouncement 删除公告
func (h *AnnouncementHandler) DeleteAnnouncement(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}

	if err := h.db.Delete(&models.Announcement{}, uint(id)).Error; err != nil {
		response.InternalError(c, "DeleteFailed")
		return
	}

	// 同时清理已读记录
	h.db.Where("announcement_id = ?", uint(id)).Delete(&models.AnnouncementRead{})

	response.Success(c, gin.H{"message": "Announcement deleted"})
}
