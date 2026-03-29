package user

import (
	"encoding/json"
	"fmt"
	"log"
	"mime/multipart"
	"net/textproto"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/middleware"
	"auralogic/internal/models"
	"auralogic/internal/pkg/response"
	"auralogic/internal/pkg/ticketbiz"
	"auralogic/internal/pkg/utils"
	"auralogic/internal/pkg/validator"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TicketHandler struct {
	db            *gorm.DB
	emailService  *service.EmailService
	pluginManager *service.PluginManagerService
}

func NewTicketHandler(db *gorm.DB, emailService *service.EmailService, pluginManager *service.PluginManagerService) *TicketHandler {
	return &TicketHandler{db: db, emailService: emailService, pluginManager: pluginManager}
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

type ticketAutoReplyPayload struct {
	Enabled        *bool                  `json:"enabled"`
	Content        string                 `json:"content"`
	ContentType    string                 `json:"content_type"`
	SenderName     string                 `json:"sender_name"`
	Metadata       map[string]interface{} `json:"metadata"`
	MarkProcessing bool                   `json:"mark_processing"`
}

// CreateTicket 创建工单
func (h *TicketHandler) CreateTicket(c *gin.Context) {
	var req CreateTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	userID := middleware.MustGetUserID(c)

	if h.pluginManager != nil {
		originalReq := req
		beforePayload := map[string]interface{}{
			"user_id":   userID,
			"subject":   req.Subject,
			"content":   req.Content,
			"category":  req.Category,
			"priority":  req.Priority,
			"source":    "user_api",
			"hook_time": time.Now().Format(time.RFC3339),
		}
		if req.OrderID != nil && *req.OrderID > 0 {
			beforePayload["order_id"] = *req.OrderID
		}

		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "ticket.create.before",
			Payload: beforePayload,
		}, h.buildTicketHookExecutionContext(c, userID, 0))
		if hookErr != nil {
			log.Printf("ticket.create.before hook execution failed: user=%d err=%v", userID, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Ticket creation rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if applyErr := applyCreateTicketHookPayload(&req, hookResult.Payload); applyErr != nil {
					log.Printf("ticket.create.before payload apply failed, fallback to original request: user=%d err=%v", userID, applyErr)
					req = originalReq
				}
			}
		}
	}

	priority := models.TicketPriorityNormal
	if req.Priority != "" {
		parsedPriority, ok := ticketbiz.ParsePriority(req.Priority)
		if !ok {
			respondUserBizError(c, ticketbiz.PriorityInvalid())
			return
		}
		priority = parsedPriority
		req.Priority = string(parsedPriority)
	}

	// 清理内容，防止XSS
	sanitizedSubject := validator.SanitizeInput(req.Subject)
	sanitizedContent := validator.SanitizeMarkdown(req.Content)

	// 检查内容长度限制
	cfg := config.GetConfig()
	if cfg.Ticket.MaxContentLength > 0 && len([]rune(sanitizedContent)) > cfg.Ticket.MaxContentLength {
		respondUserBizError(c, ticketbiz.ContentTooLong(cfg.Ticket.MaxContentLength))
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

	if h.pluginManager != nil {
		hookPayload := map[string]interface{}{
			"ticket_id":   ticket.ID,
			"ticket_no":   ticket.TicketNo,
			"user_id":     userID,
			"subject":     ticket.Subject,
			"content":     ticket.Content,
			"category":    ticket.Category,
			"priority":    string(ticket.Priority),
			"status":      string(ticket.Status),
			"created_at":  ticket.CreatedAt.Format(time.RFC3339),
			"user_name":   user.Name,
			"user_email":  user.Email,
			"user_locale": user.Locale,
		}
		if req.OrderID != nil && *req.OrderID > 0 {
			hookPayload["order_id"] = *req.OrderID
		}

		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "ticket.create.after",
			Payload: hookPayload,
		}, h.buildTicketHookExecutionContext(c, userID, ticket.ID))

		if hookErr != nil {
			log.Printf("ticket.create.after hook execution failed: user=%d ticket=%d err=%v", userID, ticket.ID, hookErr)
		} else if hookResult != nil && hookResult.Payload != nil {
			if applyErr := h.applyTicketAutoReplyPayload(ticket, hookResult.Payload); applyErr != nil {
				log.Printf("ticket.create.after payload apply failed: user=%d ticket=%d err=%v", userID, ticket.ID, applyErr)
			}
		}
	}

	if err := h.db.First(ticket, ticket.ID).Error; err != nil {
		response.InternalError(c, "Failed to load ticket")
		return
	}

	response.Success(c, ticket)

	// 发送工单创建通知邮件（通知管理员）
	if h.emailService != nil {
		go h.emailService.SendTicketCreatedEmail(ticket, user.Email)
	}
}

func applyCreateTicketHookPayload(req *CreateTicketRequest, payload map[string]interface{}) error {
	if req == nil || payload == nil {
		return nil
	}

	if raw, exists := payload["subject"]; exists {
		subject, err := ticketValueToOptionalString(raw)
		if err != nil {
			return fmt.Errorf("decode subject: %w", err)
		}
		req.Subject = subject
	}
	if raw, exists := payload["content"]; exists {
		content, err := ticketValueToOptionalString(raw)
		if err != nil {
			return fmt.Errorf("decode content: %w", err)
		}
		req.Content = content
	}
	if raw, exists := payload["category"]; exists {
		category, err := ticketValueToOptionalString(raw)
		if err != nil {
			return fmt.Errorf("decode category: %w", err)
		}
		req.Category = category
	}
	if raw, exists := payload["priority"]; exists {
		priority, err := ticketValueToOptionalString(raw)
		if err != nil {
			return fmt.Errorf("decode priority: %w", err)
		}
		req.Priority = priority
	}
	if raw, exists := payload["order_id"]; exists {
		orderID, err := ticketValueToOptionalUint(raw)
		if err != nil {
			return fmt.Errorf("decode order_id: %w", err)
		}
		req.OrderID = orderID
	}

	return nil
}

func (h *TicketHandler) buildTicketHookExecutionContext(c *gin.Context, userID uint, ticketID uint) *service.ExecutionContext {
	if c == nil {
		return nil
	}

	metadata := map[string]string{
		"request_path":    c.Request.URL.Path,
		"route":           c.FullPath(),
		"method":          c.Request.Method,
		"client_ip":       utils.GetRealIP(c),
		"user_agent":      c.GetHeader("User-Agent"),
		"accept_language": c.GetHeader("Accept-Language"),
		"ticket_id":       strconv.FormatUint(uint64(ticketID), 10),
	}
	sessionID := strings.TrimSpace(c.GetHeader("X-Session-ID"))
	return &service.ExecutionContext{
		UserID:         &userID,
		OrderID:        nil,
		SessionID:      sessionID,
		Metadata:       metadata,
		RequestContext: c.Request.Context(),
	}
}

func ticketValueToOptionalString(value interface{}) (string, error) {
	if value == nil {
		return "", nil
	}
	str, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("value must be string")
	}
	return str, nil
}

func ticketValueToOptionalUint(value interface{}) (*uint, error) {
	if value == nil {
		return nil, nil
	}

	switch typed := value.(type) {
	case uint:
		if typed == 0 {
			return nil, nil
		}
		out := typed
		return &out, nil
	case uint64:
		if typed == 0 {
			return nil, nil
		}
		out := uint(typed)
		return &out, nil
	case int:
		if typed <= 0 {
			return nil, nil
		}
		out := uint(typed)
		return &out, nil
	case int64:
		if typed <= 0 {
			return nil, nil
		}
		out := uint(typed)
		return &out, nil
	case float64:
		if typed <= 0 {
			return nil, nil
		}
		out := uint(typed)
		if float64(out) != typed {
			return nil, fmt.Errorf("value must be an integer")
		}
		return &out, nil
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" || trimmed == "0" {
			return nil, nil
		}
		parsed, err := strconv.ParseUint(trimmed, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid uint string")
		}
		out := uint(parsed)
		return &out, nil
	default:
		return nil, fmt.Errorf("value must be uint")
	}
}

func applyTicketAttachmentUploadHookPayload(file *multipart.FileHeader, payload map[string]interface{}) error {
	if file == nil || payload == nil {
		return nil
	}
	if raw, exists := payload["filename"]; exists {
		filename, err := ticketValueToOptionalString(raw)
		if err != nil {
			return fmt.Errorf("decode filename: %w", err)
		}
		normalized := strings.TrimSpace(filepath.Base(filename))
		if normalized == "" {
			return fmt.Errorf("filename cannot be empty")
		}
		file.Filename = normalized
	}
	if raw, exists := payload["content_type"]; exists {
		contentType, err := ticketValueToOptionalString(raw)
		if err != nil {
			return fmt.Errorf("decode content_type: %w", err)
		}
		if file.Header == nil {
			file.Header = make(textproto.MIMEHeader)
		}
		normalized := strings.TrimSpace(contentType)
		if normalized == "" {
			file.Header.Del("Content-Type")
		} else {
			file.Header.Set("Content-Type", normalized)
		}
	}
	return nil
}

func (h *TicketHandler) applyTicketAutoReplyPayload(ticket *models.Ticket, payload map[string]interface{}) error {
	if ticket == nil || payload == nil {
		return nil
	}

	raw, exists := payload["auto_reply"]
	if !exists {
		raw, exists = payload["ticket_auto_reply"]
	}
	if !exists || raw == nil {
		return nil
	}

	body, err := json.Marshal(raw)
	if err != nil {
		return fmt.Errorf("encode auto_reply failed: %w", err)
	}

	var reply ticketAutoReplyPayload
	if err := json.Unmarshal(body, &reply); err != nil {
		return fmt.Errorf("decode auto_reply failed: %w", err)
	}

	if reply.Enabled != nil && !*reply.Enabled {
		return nil
	}

	content := strings.TrimSpace(reply.Content)
	if content == "" {
		return nil
	}

	contentType := strings.TrimSpace(reply.ContentType)
	if contentType == "" {
		contentType = "text"
	}

	senderName := strings.TrimSpace(reply.SenderName)
	if senderName == "" {
		senderName = "System"
	}

	var metadata models.JSON
	if len(reply.Metadata) > 0 {
		metaBody, metaErr := json.Marshal(reply.Metadata)
		if metaErr != nil {
			return fmt.Errorf("encode auto_reply metadata failed: %w", metaErr)
		}
		metadata = models.JSON(metaBody)
	}

	now := time.Now()
	preview := truncateString(content, 200)

	return h.db.Transaction(func(tx *gorm.DB) error {
		message := &models.TicketMessage{
			TicketID:      ticket.ID,
			SenderType:    "admin",
			SenderID:      0,
			SenderName:    senderName,
			Content:       content,
			ContentType:   contentType,
			Metadata:      metadata,
			IsReadByUser:  false,
			IsReadByAdmin: true,
		}
		if err := tx.Create(message).Error; err != nil {
			return err
		}

		updates := map[string]interface{}{
			"unread_count_user":    gorm.Expr("unread_count_user + 1"),
			"last_message_at":      now,
			"last_message_preview": preview,
			"last_message_by":      "admin",
		}
		if reply.MarkProcessing {
			updates["status"] = models.TicketStatusProcessing
			updates["closed_at"] = nil
		}

		return tx.Model(&models.Ticket{}).Where("id = ?", ticket.ID).Updates(updates).Error
	})
}

// ListTickets 获取用户工单列表
func (h *TicketHandler) ListTickets(c *gin.Context) {
	userID := middleware.MustGetUserID(c)
	page, limit := response.GetPagination(c)
	status := c.Query("status")
	search := c.Query("search")

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

	if h.pluginManager != nil {
		hookPayload := map[string]interface{}{
			"ticket_id": ticket.ID,
			"ticket_no": ticket.TicketNo,
			"user_id":   userID,
			"status":    ticket.Status,
			"source":    "user_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, uid uint, tid uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "ticket.message.read.user.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("ticket.message.read.user.after hook execution failed: user=%d ticket=%d err=%v", uid, tid, hookErr)
			}
		}(h.buildTicketHookExecutionContext(c, userID, ticket.ID), hookPayload, userID, ticket.ID)
	}

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

	if h.pluginManager != nil {
		hookPayload := map[string]interface{}{
			"ticket_id":      ticket.ID,
			"ticket_no":      ticket.TicketNo,
			"user_id":        userID,
			"status":         ticket.Status,
			"message_count":  len(messages),
			"unread_to_user": 0,
			"source":         "user_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, uid uint, tid uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "ticket.message.read.user.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("ticket.message.read.user.after hook execution failed: user=%d ticket=%d err=%v", uid, tid, hookErr)
			}
		}(h.buildTicketHookExecutionContext(c, userID, ticket.ID), hookPayload, userID, ticket.ID)
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
		respondUserBizError(c, ticketbiz.ClosedCannotSend())
		return
	}

	hookExecCtx := h.buildTicketHookExecutionContext(c, userID, ticket.ID)
	if h.pluginManager != nil {
		originalReq := req
		hookPayload := map[string]interface{}{
			"ticket_id":    ticket.ID,
			"ticket_no":    ticket.TicketNo,
			"user_id":      userID,
			"status":       ticket.Status,
			"content":      req.Content,
			"content_type": req.ContentType,
			"source":       "user_api",
		}

		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "ticket.message.user.before",
			Payload: hookPayload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("ticket.message.user.before hook execution failed: user=%d ticket=%d err=%v", userID, ticket.ID, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Ticket message rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if applyErr := applyUserTicketMessageHookPayload(&req, hookResult.Payload); applyErr != nil {
					log.Printf("ticket.message.user.before payload apply failed, fallback to original request: user=%d ticket=%d err=%v", userID, ticket.ID, applyErr)
					req = originalReq
				}
			}
		}
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
		respondUserBizError(c, ticketbiz.ContentTooLong(cfg.Ticket.MaxContentLength))
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

	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"ticket_id":    ticket.ID,
			"ticket_no":    ticket.TicketNo,
			"message_id":   message.ID,
			"user_id":      userID,
			"content":      sanitizedContent,
			"content_type": contentType,
			"status":       models.TicketStatusOpen,
			"source":       "user_api",
			"created_at":   message.CreatedAt.Format(time.RFC3339),
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, uid uint, tid uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "ticket.message.user.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("ticket.message.user.after hook execution failed: user=%d ticket=%d err=%v", uid, tid, hookErr)
			}
		}(hookExecCtx, afterPayload, userID, ticket.ID)
	}

	response.Success(c, message)

	// 发送用户回复通知邮件（通知管理员）
	if h.emailService != nil {
		go h.emailService.SendTicketUserReplyEmail(&ticket, user.Name, truncateString(sanitizedContent, 200))
	}
}

func applyUserTicketMessageHookPayload(req *SendMessageRequest, payload map[string]interface{}) error {
	if req == nil || payload == nil {
		return nil
	}
	if raw, exists := payload["content"]; exists {
		content, err := ticketValueToOptionalString(raw)
		if err != nil {
			return fmt.Errorf("decode content: %w", err)
		}
		req.Content = content
	}
	if raw, exists := payload["content_type"]; exists {
		contentType, err := ticketValueToOptionalString(raw)
		if err != nil {
			return fmt.Errorf("decode content_type: %w", err)
		}
		req.ContentType = contentType
	}
	return nil
}

// UpdateTicketStatusRequest 更新工单状态请求
type UpdateTicketStatusRequest struct {
	Status string `json:"status" binding:"required"`
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
	beforeStatus := ticket.Status
	hookExecCtx := h.buildTicketHookExecutionContext(c, userID, ticket.ID)

	if h.pluginManager != nil {
		originalReq := req
		hookPayload := map[string]interface{}{
			"ticket_id":     ticket.ID,
			"ticket_no":     ticket.TicketNo,
			"user_id":       userID,
			"status_before": ticket.Status,
			"status_after":  req.Status,
			"source":        "user_api",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "ticket.status.user.before",
			Payload: hookPayload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("ticket.status.user.before hook execution failed: user=%d ticket=%d err=%v", userID, ticket.ID, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Ticket status update rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if applyErr := applyUserTicketStatusHookPayload(&req, hookResult.Payload); applyErr != nil {
					log.Printf("ticket.status.user.before payload apply failed, fallback to original request: user=%d ticket=%d err=%v", userID, ticket.ID, applyErr)
					req = originalReq
				}
			}
		}
	}
	status, ok := ticketbiz.ParseStatus(req.Status)
	if !ok || status == models.TicketStatusProcessing {
		respondUserBizError(c, ticketbiz.StatusInvalid())
		return
	}
	req.Status = string(status)

	updates := map[string]interface{}{
		"status": status,
	}

	if status == models.TicketStatusClosed {
		now := time.Now()
		updates["closed_at"] = now
	}

	if err := h.db.Model(&ticket).Updates(updates).Error; err != nil {
		response.InternalError(c, "Failed to update")
		return
	}

	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"ticket_id":     ticket.ID,
			"ticket_no":     ticket.TicketNo,
			"user_id":       userID,
			"status_before": beforeStatus,
			"status_after":  req.Status,
			"source":        "user_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, uid uint, tid uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "ticket.status.user.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("ticket.status.user.after hook execution failed: user=%d ticket=%d err=%v", uid, tid, hookErr)
			}
		}(hookExecCtx, afterPayload, userID, ticket.ID)
	}

	response.Success(c, gin.H{"message": "Status updated successfully"})
}

func applyUserTicketStatusHookPayload(req *UpdateTicketStatusRequest, payload map[string]interface{}) error {
	if req == nil || payload == nil {
		return nil
	}
	if raw, exists := payload["status"]; exists {
		status, err := ticketValueToOptionalString(raw)
		if err != nil {
			return fmt.Errorf("decode status: %w", err)
		}
		req.Status = status
	}
	return nil
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

	if h.pluginManager != nil {
		hookPayload := map[string]interface{}{
			"ticket_id":  ticket.ID,
			"ticket_no":  ticket.TicketNo,
			"user_id":    userID,
			"order_id":   order.ID,
			"order_no":   order.OrderNo,
			"message_id": message.ID,
			"source":     "user_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, uid uint, tid uint, oid uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "ticket.order.share.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("ticket.order.share.after hook execution failed: user=%d ticket=%d order=%d err=%v", uid, tid, oid, hookErr)
			}
		}(h.buildTicketHookExecutionContext(c, userID, ticket.ID), hookPayload, userID, ticket.ID, order.ID)
	}

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
		respondUserBizError(c, ticketbiz.ClosedCannotUpload())
		return
	}

	cfg := config.GetConfig()
	attachment := cfg.Ticket.Attachment

	file, err := c.FormFile("file")
	if err != nil {
		respondUserBizError(c, ticketbiz.FileRequired())
		return
	}
	originalFilename := file.Filename
	originalContentType := file.Header.Get("Content-Type")
	hookExecCtx := h.buildTicketHookExecutionContext(c, userID, ticket.ID)
	if h.pluginManager != nil {
		hookPayload := map[string]interface{}{
			"ticket_id":    ticket.ID,
			"ticket_no":    ticket.TicketNo,
			"user_id":      userID,
			"status":       ticket.Status,
			"filename":     file.Filename,
			"size":         file.Size,
			"content_type": file.Header.Get("Content-Type"),
			"source":       "user_api",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "ticket.attachment.upload.before",
			Payload: hookPayload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("ticket.attachment.upload.before hook execution failed: user=%d ticket=%d err=%v", userID, ticket.ID, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Attachment upload rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if applyErr := applyTicketAttachmentUploadHookPayload(file, hookResult.Payload); applyErr != nil {
					log.Printf("ticket.attachment.upload.before payload apply failed, fallback to original metadata: user=%d ticket=%d err=%v", userID, ticket.ID, applyErr)
					file.Filename = originalFilename
					if file.Header == nil {
						file.Header = make(textproto.MIMEHeader)
					}
					if strings.TrimSpace(originalContentType) == "" {
						file.Header.Del("Content-Type")
					} else {
						file.Header.Set("Content-Type", originalContentType)
					}
				}
			}
		}
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))

	// 判断文件类型并验证
	isAudio := strings.HasPrefix(file.Header.Get("Content-Type"), "audio/")
	if isAudio {
		if attachment != nil && !attachment.EnableVoice {
			respondUserBizError(c, ticketbiz.VoiceUploadDisabled())
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
			respondUserBizError(c, ticketbiz.AudioFormatInvalid())
			return
		}
		maxSize := int64(10 * 1024 * 1024)
		if attachment != nil && attachment.MaxVoiceSize > 0 {
			maxSize = attachment.MaxVoiceSize
		}
		if file.Size > maxSize {
			respondUserBizError(c, ticketbiz.VoiceFileTooLarge(ticketbiz.BytesToMegabytes(maxSize)))
			return
		}
	} else {
		if attachment != nil && !attachment.EnableImage {
			respondUserBizError(c, ticketbiz.ImageUploadDisabled())
			return
		}
		maxSize := int64(5 * 1024 * 1024)
		if attachment != nil && attachment.MaxImageSize > 0 {
			maxSize = attachment.MaxImageSize
		}
		if file.Size > maxSize {
			respondUserBizError(c, ticketbiz.ImageFileTooLarge(ticketbiz.BytesToMegabytes(maxSize)))
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
			respondUserBizError(c, ticketbiz.ImageFormatUnsupported())
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

	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"ticket_id":     ticket.ID,
			"ticket_no":     ticket.TicketNo,
			"user_id":       userID,
			"status":        ticket.Status,
			"filename":      filename,
			"original_name": file.Filename,
			"size":          file.Size,
			"url":           fileURL,
			"source":        "user_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, uid uint, tid uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "ticket.attachment.upload.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("ticket.attachment.upload.after hook execution failed: user=%d ticket=%d err=%v", uid, tid, hookErr)
			}
		}(hookExecCtx, afterPayload, userID, ticket.ID)
	}

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
