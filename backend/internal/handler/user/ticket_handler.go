package user

import (
	"encoding/json"
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

// generateTicketNo 生成工单号
func (h *TicketHandler) generateTicketNo() string {
	return fmt.Sprintf("TK%s%04d", time.Now().Format("20060102150405"), time.Now().UnixNano()%10000)
}

// CreateTicketRequest 创建工单请求
type CreateTicketRequest struct {
	Subject  string `json:"subject" binding:"required,max=255"`
	Content  string `json:"content" binding:"required"`
	Category string `json:"category"`
	Priority string `json:"priority"`
	OrderID  *uint  `json:"order_id"` // 可选绑定订单
}

// CreateTicket 创建工单
func (h *TicketHandler) CreateTicket(c *gin.Context) {
	var req CreateTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	userID := middleware.MustGetUserID(c)

	priority := models.TicketPriorityNormal
	if req.Priority != "" {
		priority = models.TicketPriority(req.Priority)
	}

	// 清理内容，防止XSS
	sanitizedSubject := validator.SanitizeInput(req.Subject)
	sanitizedContent := validator.SanitizeMarkdown(req.Content)

	// 检查内容长度限制
	cfg := config.GetConfig()
	if cfg.Ticket.MaxContentLength > 0 && len([]rune(sanitizedContent)) > cfg.Ticket.MaxContentLength {
		response.BadRequest(c, fmt.Sprintf("Content cannot exceed %d characters", cfg.Ticket.MaxContentLength))
		return
	}

	now := time.Now()
	ticket := &models.Ticket{
		TicketNo:           h.generateTicketNo(),
		UserID:             userID,
		Subject:            sanitizedSubject,
		Content:            sanitizedContent,
		Category:           req.Category,
		Priority:           priority,
		Status:             models.TicketStatusOpen,
		LastMessageAt:      &now,
		LastMessagePreview: truncateString(sanitizedContent, 200),
		LastMessageBy:      "user",
		UnreadCountAdmin:   1,
	}

	if err := h.db.Create(ticket).Error; err != nil {
		response.InternalError(c, "Failed to create ticket")
		return
	}

	// 创建初始消息
	var user models.User
	h.db.First(&user, userID)

	message := &models.TicketMessage{
		TicketID:      ticket.ID,
		SenderType:    "user",
		SenderID:      userID,
		SenderName:    user.Name,
		Content:       sanitizedContent,
		ContentType:   "text",
		IsReadByUser:  true,
		IsReadByAdmin: false,
	}
	h.db.Create(message)

	// 如果绑定了订单，自动分享订单给客服
	if req.OrderID != nil && *req.OrderID > 0 {
		// 验证订单属于当前用户
		var order models.Order
		if err := h.db.First(&order, *req.OrderID).Error; err == nil {
			// 安全检查：确保order.UserID不为nil且属于当前用户
			if order.UserID != nil && *order.UserID == userID {
				// 创建订单访问权限
				access := &models.TicketOrderAccess{
					TicketID:       ticket.ID,
					OrderID:        *req.OrderID,
					GrantedBy:      userID,
					CanView:        true,
					CanEdit:        false,
					CanViewPrivacy: false,
				}
				h.db.Create(access)

				// 创建订单分享消息
				orderMsg := &models.TicketMessage{
					TicketID:      ticket.ID,
					SenderType:    "user",
					SenderID:      userID,
					SenderName:    user.Name,
					Content:       fmt.Sprintf("Shared order %s", order.OrderNo),
					ContentType:   "order",
					IsReadByUser:  true,
					IsReadByAdmin: false,
				}
				// 添加订单信息到 metadata
				metadataBytes, _ := json.Marshal(map[string]interface{}{
					"order_id": order.ID,
					"order_no": order.OrderNo,
				})
				orderMsg.Metadata = models.JSON(metadataBytes)
				h.db.Create(orderMsg)

				// 更新工单最后消息预览
				h.db.Model(ticket).Updates(map[string]interface{}{
					"last_message_preview": fmt.Sprintf("Shared order %s", order.OrderNo),
				})
			}
		}
	}

	response.Success(c, ticket)

	// 发送工单创建通知邮件（通知管理员）
	if h.emailService != nil {
		go h.emailService.SendTicketCreatedEmail(ticket, user.Email)
	}
}

// ListTickets 获取用户工单列表
func (h *TicketHandler) ListTickets(c *gin.Context) {
	userID := middleware.MustGetUserID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	status := c.Query("status")
	search := c.Query("search")

	if limit > 100 {
		limit = 100
	}

	var tickets []models.Ticket
	var total int64

	query := h.db.Model(&models.Ticket{}).Where("user_id = ?", userID)

	if status != "" {
		query = query.Where("status = ?", status)
	}

	if search != "" {
		query = query.Where("subject ILIKE ? OR ticket_no ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	query.Count(&total)

	offset := (page - 1) * limit
	if err := query.Order("last_message_at DESC").Offset(offset).Limit(limit).Find(&tickets).Error; err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	response.Paginated(c, tickets, page, limit, total)
}

// GetTicket 获取工单详情
func (h *TicketHandler) GetTicket(c *gin.Context) {
	userID := middleware.MustGetUserID(c)
	ticketID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}

	var ticket models.Ticket
	if err := h.db.First(&ticket, ticketID).Error; err != nil {
		response.NotFound(c, "Ticket not found")
		return
	}

	if ticket.UserID != userID {
		response.Forbidden(c, "No permission to access this ticket")
		return
	}

	// 标记用户已读
	h.db.Model(&ticket).Update("unread_count_user", 0)
	h.db.Model(&models.TicketMessage{}).Where("ticket_id = ? AND sender_type = ?", ticketID, "admin").Update("is_read_by_user", true)

	response.Success(c, ticket)
}

// GetTicketMessages 获取工单消息列表
func (h *TicketHandler) GetTicketMessages(c *gin.Context) {
	userID := middleware.MustGetUserID(c)
	ticketID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}

	var ticket models.Ticket
	if err := h.db.First(&ticket, ticketID).Error; err != nil {
		response.NotFound(c, "Ticket not found")
		return
	}

	if ticket.UserID != userID {
		response.Forbidden(c, "No permission to access this ticket")
		return
	}

	// 标记管理员发送的消息为用户已读
	h.db.Model(&models.TicketMessage{}).Where("ticket_id = ? AND sender_type = ?", ticketID, "admin").Update("is_read_by_user", true)
	h.db.Model(&ticket).Update("unread_count_user", 0)

	var messages []models.TicketMessage
	if err := h.db.Where("ticket_id = ?", ticketID).Order("created_at ASC").Find(&messages).Error; err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	response.Success(c, messages)
}

// SendMessageRequest 发送消息请求
type SendMessageRequest struct {
	Content     string `json:"content" binding:"required"`
	ContentType string `json:"content_type"`
}

// SendMessage 发送消息
func (h *TicketHandler) SendMessage(c *gin.Context) {
	userID := middleware.MustGetUserID(c)
	ticketID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}

	var req SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	var ticket models.Ticket
	if err := h.db.First(&ticket, ticketID).Error; err != nil {
		response.NotFound(c, "Ticket not found")
		return
	}

	if ticket.UserID != userID {
		response.Forbidden(c, "No permission to access this ticket")
		return
	}

	if ticket.Status == models.TicketStatusClosed {
		response.BadRequest(c, "Ticket is closed, cannot send messages")
		return
	}

	var user models.User
	h.db.First(&user, userID)

	contentType := "text"
	if req.ContentType != "" {
		contentType = req.ContentType
	}

	// 清理消息内容，防止XSS
	sanitizedContent := validator.SanitizeMarkdown(req.Content)

	// 检查内容长度限制
	cfg := config.GetConfig()
	if cfg.Ticket.MaxContentLength > 0 && len([]rune(sanitizedContent)) > cfg.Ticket.MaxContentLength {
		response.BadRequest(c, fmt.Sprintf("Message cannot exceed %d characters", cfg.Ticket.MaxContentLength))
		return
	}

	message := &models.TicketMessage{
		TicketID:      uint(ticketID),
		SenderType:    "user",
		SenderID:      userID,
		SenderName:    user.Name,
		Content:       sanitizedContent,
		ContentType:   contentType,
		IsReadByUser:  true,
		IsReadByAdmin: false,
	}

	if err := h.db.Create(message).Error; err != nil {
		response.InternalError(c, "Failed to send")
		return
	}

	// 更新工单信息
	now := time.Now()
	h.db.Model(&ticket).Updates(map[string]interface{}{
		"last_message_at":      now,
		"last_message_preview": truncateString(sanitizedContent, 200),
		"last_message_by":      "user",
		"unread_count_admin":   gorm.Expr("unread_count_admin + 1"),
		"status":               models.TicketStatusOpen, // 用户回复后重新打开工单
	})

	response.Success(c, message)

	// 发送用户回复通知邮件（通知管理员）
	if h.emailService != nil {
		go h.emailService.SendTicketUserReplyEmail(&ticket, user.Name, truncateString(sanitizedContent, 200))
	}
}

// UpdateTicketStatusRequest 更新工单状态请求
type UpdateTicketStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=open resolved closed"`
}

// UpdateTicketStatus 更新工单状态（用户只能关闭或重新打开）
func (h *TicketHandler) UpdateTicketStatus(c *gin.Context) {
	userID := middleware.MustGetUserID(c)
	ticketID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}

	var req UpdateTicketStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	var ticket models.Ticket
	if err := h.db.First(&ticket, ticketID).Error; err != nil {
		response.NotFound(c, "Ticket not found")
		return
	}

	if ticket.UserID != userID {
		response.Forbidden(c, "No permission to operate this ticket")
		return
	}

	updates := map[string]interface{}{
		"status": req.Status,
	}

	if req.Status == "closed" {
		now := time.Now()
		updates["closed_at"] = now
	}

	if err := h.db.Model(&ticket).Updates(updates).Error; err != nil {
		response.InternalError(c, "Failed to update")
		return
	}

	response.Success(c, gin.H{"message": "Status updated successfully"})
}

// ShareOrderRequest 分享订单请求
type ShareOrderRequest struct {
	OrderID uint `json:"order_id" binding:"required"`
}

// ShareOrder 分享订单给客服
func (h *TicketHandler) ShareOrder(c *gin.Context) {
	userID := middleware.MustGetUserID(c)
	ticketID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}

	var req ShareOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	var ticket models.Ticket
	if err := h.db.First(&ticket, ticketID).Error; err != nil {
		response.NotFound(c, "Ticket not found")
		return
	}

	if ticket.UserID != userID {
		response.Forbidden(c, "No permission to operate this ticket")
		return
	}

	// 验证订单属于用户
	var order models.Order
	if err := h.db.First(&order, req.OrderID).Error; err != nil {
		response.NotFound(c, "Order not found")
		return
	}

	if order.UserID == nil || *order.UserID != userID {
		response.Forbidden(c, "No permission to share this order")
		return
	}

	// 检查是否已授权
	var existingAccess models.TicketOrderAccess
	if err := h.db.Where("ticket_id = ? AND order_id = ?", ticketID, req.OrderID).First(&existingAccess).Error; err == nil {
		// 已经分享过，无需更新
	} else {
		// 创建新授权
		access := &models.TicketOrderAccess{
			TicketID:       uint(ticketID),
			OrderID:        req.OrderID,
			GrantedBy:      userID,
			CanView:        true,
			CanEdit:        false,
			CanViewPrivacy: false,
		}
		h.db.Create(access)
	}

	// 发送系统消息
	var user models.User
	h.db.First(&user, userID)

	metadataBytes, _ := json.Marshal(map[string]interface{}{
		"order_id": req.OrderID,
		"order_no": order.OrderNo,
	})

	message := &models.TicketMessage{
		TicketID:      uint(ticketID),
		SenderType:    "user",
		SenderID:      userID,
		SenderName:    user.Name,
		Content:       fmt.Sprintf("Shared order %s", order.OrderNo),
		ContentType:   "order",
		Metadata:      models.JSON(metadataBytes),
		IsReadByUser:  true,
		IsReadByAdmin: false,
	}
	h.db.Create(message)

	// 更新工单
	now := time.Now()
	h.db.Model(&ticket).Updates(map[string]interface{}{
		"last_message_at":      now,
		"last_message_preview": fmt.Sprintf("Shared order %s", order.OrderNo),
		"last_message_by":      "user",
		"unread_count_admin":   gorm.Expr("unread_count_admin + 1"),
	})

	response.Success(c, gin.H{"message": "Order shared successfully"})
}

// GetSharedOrders 获取工单中分享的订单
func (h *TicketHandler) GetSharedOrders(c *gin.Context) {
	userID := middleware.MustGetUserID(c)
	ticketID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}

	var ticket models.Ticket
	if err := h.db.First(&ticket, ticketID).Error; err != nil {
		response.NotFound(c, "Ticket not found")
		return
	}

	if ticket.UserID != userID {
		response.Forbidden(c, "No permission to access this ticket")
		return
	}

	var accesses []models.TicketOrderAccess
	if err := h.db.Preload("Order").Where("ticket_id = ?", ticketID).Find(&accesses).Error; err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	response.Success(c, accesses)
}

// RevokeOrderAccess 撤销订单授权
func (h *TicketHandler) RevokeOrderAccess(c *gin.Context) {
	userID := middleware.MustGetUserID(c)
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

	var ticket models.Ticket
	if err := h.db.First(&ticket, ticketID).Error; err != nil {
		response.NotFound(c, "Ticket not found")
		return
	}

	if ticket.UserID != userID {
		response.Forbidden(c, "No permission to operate this ticket")
		return
	}

	if err := h.db.Where("ticket_id = ? AND order_id = ?", ticketID, orderID).Delete(&models.TicketOrderAccess{}).Error; err != nil {
		response.InternalError(c, "Failed to revoke")
		return
	}

	response.Success(c, gin.H{"message": "Access revoked"})
}

// UploadFile 用户上传工单附件
func (h *TicketHandler) UploadFile(c *gin.Context) {
	userID := middleware.MustGetUserID(c)
	ticketID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid ticket ID")
		return
	}

	// 验证工单属于当前用户
	var ticket models.Ticket
	if err := h.db.First(&ticket, ticketID).Error; err != nil {
		response.NotFound(c, "Ticket not found")
		return
	}
	if ticket.UserID != userID {
		response.Forbidden(c, "No permission to operate this ticket")
		return
	}

	if ticket.Status == models.TicketStatusClosed {
		response.BadRequest(c, "Ticket is closed, cannot upload files")
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
