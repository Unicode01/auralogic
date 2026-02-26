package admin

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"auralogic/internal/config"
	"auralogic/internal/middleware"
	"auralogic/internal/models"
	"auralogic/internal/pkg/response"
	"auralogic/internal/pkg/validator"
	"auralogic/internal/service"
	"gorm.io/gorm"
)

type TicketHandler struct {
	db           *gorm.DB
	emailService *service.EmailService
}

func NewTicketHandler(db *gorm.DB, emailService *service.EmailService) *TicketHandler {
	return &TicketHandler{db: db, emailService: emailService}
}

// ListTickets 获取工单列表
func (h *TicketHandler) ListTickets(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	status := c.Query("status")
	excludeStatus := c.Query("exclude_status")
	search := c.Query("search")
	assignedTo := c.Query("assigned_to")

	if limit > 100 {
		limit = 100
	}

	var tickets []models.Ticket
	var total int64

	query := h.db.Model(&models.Ticket{}).Preload("User")

	if status != "" {
		query = query.Where("status = ?", status)
	} else if excludeStatus != "" {
		query = query.Where("status != ?", excludeStatus)
	}
	if search != "" {
		query = query.Where("ticket_no LIKE ? OR subject LIKE ?", "%"+search+"%", "%"+search+"%")
	}
	if assignedTo == "me" {
		adminID := middleware.MustGetUserID(c)
		query = query.Where("assigned_to = ?", adminID)
	} else if assignedTo == "unassigned" {
		query = query.Where("assigned_to IS NULL")
	}

	query.Count(&total)

	offset := (page - 1) * limit
	// 默认排序：状态优先级(open>processing>resolved>closed) → 未读优先 → 最近活跃优先
	orderClause := `
		CASE status
			WHEN 'open' THEN 0
			WHEN 'processing' THEN 1
			WHEN 'resolved' THEN 2
			WHEN 'closed' THEN 3
			ELSE 4
		END ASC,
		CASE WHEN unread_count_admin > 0 THEN 0 ELSE 1 END ASC,
		last_message_at DESC`
	if err := query.Order(orderClause).Offset(offset).Limit(limit).Find(&tickets).Error; err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	response.Paginated(c, tickets, page, limit, total)
}

// GetTicket 获取工单详情
func (h *TicketHandler) GetTicket(c *gin.Context) {
	ticketID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}

	var ticket models.Ticket
	if err := h.db.Preload("User").Preload("AssignedUser").First(&ticket, ticketID).Error; err != nil {
		response.NotFound(c, "Ticket not found")
		return
	}

	// 标记管理员已读
	h.db.Model(&ticket).Update("unread_count_admin", 0)
	h.db.Model(&models.TicketMessage{}).Where("ticket_id = ? AND sender_type = ?", ticketID, "user").Update("is_read_by_admin", true)

	response.Success(c, ticket)
}

// GetTicketMessages 获取工单消息
func (h *TicketHandler) GetTicketMessages(c *gin.Context) {
	ticketID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}

	// 标记用户发送的消息为管理员已读
	h.db.Model(&models.TicketMessage{}).Where("ticket_id = ? AND sender_type = ?", ticketID, "user").Update("is_read_by_admin", true)
	h.db.Model(&models.Ticket{}).Where("id = ?", ticketID).Update("unread_count_admin", 0)

	var messages []models.TicketMessage
	if err := h.db.Where("ticket_id = ?", ticketID).Order("created_at ASC").Find(&messages).Error; err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	response.Success(c, messages)
}

// SendMessageRequest 发送消息请求
type AdminSendMessageRequest struct {
	Content     string `json:"content" binding:"required"`
	ContentType string `json:"content_type"`
}

// SendMessage 管理员发送消息
func (h *TicketHandler) SendMessage(c *gin.Context) {
	adminID := middleware.MustGetUserID(c)
	ticketID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}

	var req AdminSendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	var ticket models.Ticket
	if err := h.db.First(&ticket, ticketID).Error; err != nil {
		response.NotFound(c, "Ticket not found")
		return
	}

	var admin models.User
	h.db.First(&admin, adminID)

	contentType := "text"
	if req.ContentType != "" {
		contentType = req.ContentType
	}

	// 清理消息内容，防止XSS
	sanitizedContent := validator.SanitizeMarkdown(req.Content)

	// 检查内容长度限制
	cfg := config.GetConfig()
	if cfg.Ticket.MaxContentLength > 0 && len([]rune(sanitizedContent)) > cfg.Ticket.MaxContentLength {
		response.BadRequest(c, fmt.Sprintf("Message length cannot exceed %d characters", cfg.Ticket.MaxContentLength))
		return
	}

	message := &models.TicketMessage{
		TicketID:      uint(ticketID),
		SenderType:    "admin",
		SenderID:      adminID,
		SenderName:    admin.Name,
		Content:       sanitizedContent,
		ContentType:   contentType,
		IsReadByUser:  false,
		IsReadByAdmin: true,
	}

	if err := h.db.Create(message).Error; err != nil {
		response.InternalError(c, "Send failed")
		return
	}

	// 更新工单信息
	now := time.Now()
	updates := map[string]interface{}{
		"last_message_at":      now,
		"last_message_preview": truncateString(sanitizedContent, 200),
		"last_message_by":      "admin",
		"unread_count_user":    gorm.Expr("unread_count_user + 1"),
	}

	// 如果工单未分配，自动分配给当前管理员
	if ticket.AssignedTo == nil {
		updates["assigned_to"] = adminID
	}

	// 如果是待处理状态，改为处理中
	if ticket.Status == models.TicketStatusOpen {
		updates["status"] = models.TicketStatusProcessing
	}

	h.db.Model(&ticket).Updates(updates)

	response.Success(c, message)

	// 发送管理员回复通知邮件（通知用户）
	if h.emailService != nil {
		go h.emailService.SendTicketAdminReplyEmail(&ticket, admin.Name, truncateString(sanitizedContent, 200))
	}
}

// UpdateTicketRequest 更新工单请求
type UpdateTicketRequest struct {
	Status     string `json:"status"`
	Priority   string `json:"priority"`
	AssignedTo *uint  `json:"assigned_to"`
}

// UpdateTicket 更新工单
func (h *TicketHandler) UpdateTicket(c *gin.Context) {
	ticketID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}

	var req UpdateTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	var ticket models.Ticket
	if err := h.db.First(&ticket, ticketID).Error; err != nil {
		response.NotFound(c, "Ticket not found")
		return
	}

	updates := make(map[string]interface{})

	if req.Status != "" {
		updates["status"] = req.Status
		if req.Status == "closed" || req.Status == "resolved" {
			now := time.Now()
			updates["closed_at"] = now
		}
	}

	if req.Priority != "" {
		updates["priority"] = req.Priority
	}

	if req.AssignedTo != nil {
		updates["assigned_to"] = req.AssignedTo
	}

	if len(updates) > 0 {
		if err := h.db.Model(&ticket).Updates(updates).Error; err != nil {
			response.InternalError(c, "Update failed")
			return
		}
	}

	// 重新加载工单
	h.db.Preload("User").Preload("AssignedUser").First(&ticket, ticketID)

	response.Success(c, ticket)

	// 如果工单被标记为已解决，发送通知邮件给用户
	if req.Status == "resolved" && h.emailService != nil {
		go h.emailService.SendTicketResolvedEmail(&ticket)
	}
}

// GetSharedOrders 获取工单中分享的订单
func (h *TicketHandler) GetSharedOrders(c *gin.Context) {
	ticketID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}

	var accesses []models.TicketOrderAccess
	if err := h.db.Preload("Order").Where("ticket_id = ?", ticketID).Find(&accesses).Error; err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	response.Success(c, accesses)
}

// GetSharedOrder 获取分享的订单详情
func (h *TicketHandler) GetSharedOrder(c *gin.Context) {
	ticketID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}

	orderID, err := strconv.ParseUint(c.Param("orderId"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid order ID")
		return
	}

	// 检查授权
	var access models.TicketOrderAccess
	if err := h.db.Where("ticket_id = ? AND order_id = ?", ticketID, orderID).First(&access).Error; err != nil {
		response.Forbidden(c, "No permission to access this order")
		return
	}

	if access.IsExpired() {
		response.Forbidden(c, "Authorization expired")
		return
	}

	var order models.Order
	if err := h.db.First(&order, orderID).Error; err != nil {
		response.NotFound(c, "Order not found")
		return
	}

	// 用户主动分享订单到工单即视为同意客服查看所有信息，不再隐藏隐私信息
	// 客服可以查看完整的订单详情，包括地址等

	response.Success(c, gin.H{
		"order":  order,
		"access": access,
	})
}

// GetTicketStats 获取工单统计
func (h *TicketHandler) GetTicketStats(c *gin.Context) {
	var stats struct {
		Total      int64 `json:"total"`
		Open       int64 `json:"open"`
		Processing int64 `json:"processing"`
		Resolved   int64 `json:"resolved"`
		Closed     int64 `json:"closed"`
		Unread     int64 `json:"unread"`
	}

	h.db.Model(&models.Ticket{}).Count(&stats.Total)
	h.db.Model(&models.Ticket{}).Where("status = ?", "open").Count(&stats.Open)
	h.db.Model(&models.Ticket{}).Where("status = ?", "processing").Count(&stats.Processing)
	h.db.Model(&models.Ticket{}).Where("status = ?", "resolved").Count(&stats.Resolved)
	h.db.Model(&models.Ticket{}).Where("status = ?", "closed").Count(&stats.Closed)
	h.db.Model(&models.Ticket{}).Where("unread_count_admin > 0").Count(&stats.Unread)

	response.Success(c, stats)
}

// UploadFile 管理员上传工单附件
func (h *TicketHandler) UploadFile(c *gin.Context) {
	ticketID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}

	// 验证工单存在
	var ticket models.Ticket
	if err := h.db.First(&ticket, ticketID).Error; err != nil {
		response.NotFound(c, "Ticket not found")
		return
	}

	cfg := config.GetConfig()
	attachment := cfg.Ticket.Attachment

	file, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "Please select a file")
		return
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))

	// 判断文件类型并验证
	isAudio := strings.HasPrefix(file.Header.Get("Content-Type"), "audio/")
	if isAudio {
		if attachment != nil && !attachment.EnableVoice {
			response.BadRequest(c, "Voice upload not allowed")
			return
		}
		allowedAudioTypes := []string{".mp3", ".wav", ".m4a", ".ogg", ".aac", ".webm"}
		audioAllowed := false
		for _, t := range allowedAudioTypes {
			if ext == strings.ToLower(t) {
				audioAllowed = true
				break
			}
		}
		if !audioAllowed {
			response.BadRequest(c, "Invalid audio format")
			return
		}
		maxSize := int64(10 * 1024 * 1024)
		if attachment != nil && attachment.MaxVoiceSize > 0 {
			maxSize = attachment.MaxVoiceSize
		}
		if file.Size > maxSize {
			response.BadRequest(c, fmt.Sprintf("Voice file size cannot exceed %dMB", maxSize/1024/1024))
			return
		}
	} else {
		if attachment != nil && !attachment.EnableImage {
			response.BadRequest(c, "Image upload not allowed")
			return
		}
		maxSize := int64(5 * 1024 * 1024)
		if attachment != nil && attachment.MaxImageSize > 0 {
			maxSize = attachment.MaxImageSize
		}
		if file.Size > maxSize {
			response.BadRequest(c, fmt.Sprintf("Image size cannot exceed %dMB", maxSize/1024/1024))
			return
		}

		// 验证图片类型
		allowedTypes := []string{".jpg", ".jpeg", ".png", ".gif", ".webp"}
		if attachment != nil && len(attachment.AllowedImageTypes) > 0 {
			allowedTypes = attachment.AllowedImageTypes
		}
		allowed := false
		for _, t := range allowedTypes {
			if ext == strings.ToLower(t) {
				allowed = true
				break
			}
		}
		if !allowed {
			response.BadRequest(c, "Unsupported image format")
			return
		}
	}

	// 保存文件
	filename := fmt.Sprintf("%s%s", uuid.New().String(), ext)
	dateDir := time.Now().Format("2006/01/02")
	targetDir := filepath.Join(cfg.Upload.Dir, "tickets", dateDir)

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		response.InternalError(c, "Failed to create directory")
		return
	}

	targetPath := filepath.Join(targetDir, filename)
	if err := c.SaveUploadedFile(file, targetPath); err != nil {
		response.InternalError(c, "Failed to save file")
		return
	}

	fileURL := fmt.Sprintf("%s/uploads/tickets/%s/%s", cfg.App.URL, dateDir, filename)

	response.Success(c, gin.H{
		"url":      fileURL,
		"filename": filename,
		"size":     file.Size,
	})
}

func truncateString(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func maskString(s string) string {
	if len(s) <= 2 {
		return "**"
	}
	return s[:1] + strings.Repeat("*", len(s)-2) + s[len(s)-1:]
}
