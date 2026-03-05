package admin

import (
	"log"
	"strconv"
	"strings"

	"auralogic/internal/models"
	"auralogic/internal/pkg/response"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AnnouncementHandler struct {
	db           *gorm.DB
	emailService *service.EmailService
	smsService   *service.SMSService
}

func NewAnnouncementHandler(db *gorm.DB, emailService *service.EmailService, smsService *service.SMSService) *AnnouncementHandler {
	return &AnnouncementHandler{
		db:           db,
		emailService: emailService,
		smsService:   smsService,
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

// ListAnnouncements 公告列表
func (h *AnnouncementHandler) ListAnnouncements(c *gin.Context) {
	page, limit := response.GetPagination(c)
	search := c.Query("search")
	mandatory := c.Query("is_mandatory") // "true" / "false" / ""
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

	if req.Title != "" {
		announcement.Title = req.Title
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
