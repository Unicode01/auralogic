package admin

import (
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

// ListTickets 获取工单列表
func (h *TicketHandler) ListTickets(c *gin.Context) {
	page, limit := response.GetPagination(c)
	status := c.Query("status")
	excludeStatus := c.Query("exclude_status")
	search := c.Query("search")
	assignedTo := c.Query("assigned_to")

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
	adminID := middleware.MustGetUserID(c)
	if adminID == 0 {
		return
	}

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

	if h.pluginManager != nil {
		hookPayload := map[string]interface{}{
			"ticket_id": ticket.ID,
			"ticket_no": ticket.TicketNo,
			"admin_id":  adminID,
			"status":    ticket.Status,
			"source":    "admin_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, aid uint, tid uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "ticket.message.read.admin.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("ticket.message.read.admin.after hook execution failed: admin=%d ticket=%d err=%v", aid, tid, hookErr)
			}
		}(h.buildTicketHookExecutionContext(c, adminID, ticket.ID), hookPayload, adminID, ticket.ID)
	}

	response.Success(c, ticket)
}

// GetTicketMessages 获取工单消息
func (h *TicketHandler) GetTicketMessages(c *gin.Context) {
	adminID := middleware.MustGetUserID(c)
	if adminID == 0 {
		return
	}

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

	var ticket models.Ticket
	_ = h.db.Select("id", "ticket_no", "status").First(&ticket, ticketID).Error

	if h.pluginManager != nil {
		hookPayload := map[string]interface{}{
			"ticket_id":       ticketID,
			"ticket_no":       ticket.TicketNo,
			"admin_id":        adminID,
			"status":          ticket.Status,
			"message_count":   len(messages),
			"unread_to_admin": 0,
			"source":          "admin_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, aid uint, tid uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "ticket.message.read.admin.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("ticket.message.read.admin.after hook execution failed: admin=%d ticket=%d err=%v", aid, tid, hookErr)
			}
		}(h.buildTicketHookExecutionContext(c, adminID, uint(ticketID)), hookPayload, adminID, uint(ticketID))
	}

	response.Success(c, messages)
}

func (h *TicketHandler) buildTicketHookExecutionContext(c *gin.Context, adminID uint, ticketID uint) *service.ExecutionContext {
	if c == nil {
		return nil
	}

	metadata := map[string]string{
		"request_path":    c.Request.URL.Path,
		"route":           c.FullPath(),
		"method":          c.Request.Method,
		"client_ip":       c.ClientIP(),
		"user_agent":      c.GetHeader("User-Agent"),
		"accept_language": c.GetHeader("Accept-Language"),
		"ticket_id":       strconv.FormatUint(uint64(ticketID), 10),
		"operator_type":   "admin",
	}
	sessionID := strings.TrimSpace(c.GetHeader("X-Session-ID"))
	return &service.ExecutionContext{
		UserID:         &adminID,
		OrderID:        nil,
		SessionID:      sessionID,
		Metadata:       metadata,
		RequestContext: c.Request.Context(),
	}
}

func applyAdminTicketMessageHookPayload(req *AdminSendMessageRequest, payload map[string]interface{}) error {
	if req == nil || payload == nil {
		return nil
	}
	if raw, exists := payload["content"]; exists {
		content, err := valueToOptionalString(raw)
		if err != nil {
			return fmt.Errorf("decode content: %w", err)
		}
		req.Content = content
	}
	if raw, exists := payload["content_type"]; exists {
		contentType, err := valueToOptionalString(raw)
		if err != nil {
			return fmt.Errorf("decode content_type: %w", err)
		}
		req.ContentType = contentType
	}
	return nil
}

func applyAdminTicketUpdateHookPayload(req *UpdateTicketRequest, payload map[string]interface{}) error {
	if req == nil || payload == nil {
		return nil
	}
	if raw, exists := payload["status"]; exists {
		status, err := valueToOptionalString(raw)
		if err != nil {
			return fmt.Errorf("decode status: %w", err)
		}
		req.Status = status
	}
	if raw, exists := payload["priority"]; exists {
		priority, err := valueToOptionalString(raw)
		if err != nil {
			return fmt.Errorf("decode priority: %w", err)
		}
		req.Priority = priority
	}
	if raw, exists := payload["assigned_to"]; exists {
		assignedTo, err := valueToOptionalUint(raw)
		if err != nil {
			return fmt.Errorf("decode assigned_to: %w", err)
		}
		req.AssignedTo = assignedTo
	}
	return nil
}

func applyAdminTicketAssignHookPayload(req *UpdateTicketRequest, payload map[string]interface{}) error {
	if req == nil || payload == nil {
		return nil
	}
	if raw, exists := payload["assigned_to"]; exists {
		assignedTo, err := valueToOptionalUint(raw)
		if err != nil {
			return fmt.Errorf("decode assigned_to: %w", err)
		}
		req.AssignedTo = assignedTo
	}
	return nil
}

func valueToOptionalString(value interface{}) (string, error) {
	if value == nil {
		return "", nil
	}
	str, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("value must be string")
	}
	return str, nil
}

func valueToOptionalUint(value interface{}) (*uint, error) {
	if value == nil {
		return nil, nil
	}
	switch typed := value.(type) {
	case uint:
		out := typed
		return &out, nil
	case uint64:
		out := uint(typed)
		return &out, nil
	case int:
		if typed < 0 {
			return nil, fmt.Errorf("value must be non-negative")
		}
		out := uint(typed)
		return &out, nil
	case int64:
		if typed < 0 {
			return nil, fmt.Errorf("value must be non-negative")
		}
		out := uint(typed)
		return &out, nil
	case float64:
		if typed < 0 {
			return nil, fmt.Errorf("value must be non-negative")
		}
		out := uint(typed)
		if float64(out) != typed {
			return nil, fmt.Errorf("value must be integer")
		}
		return &out, nil
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
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

func ticketUintPtrEqual(left *uint, right *uint) bool {
	if left == nil && right == nil {
		return true
	}
	if left == nil || right == nil {
		return false
	}
	return *left == *right
}

func applyTicketAttachmentUploadHookPayload(file *multipart.FileHeader, payload map[string]interface{}) error {
	if file == nil || payload == nil {
		return nil
	}
	if raw, exists := payload["filename"]; exists {
		filename, err := valueToOptionalString(raw)
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
		contentType, err := valueToOptionalString(raw)
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
	if ticket.Status == models.TicketStatusClosed {
		respondAdminBizError(c, ticketbiz.ClosedCannotSend())
		return
	}
	hookExecCtx := h.buildTicketHookExecutionContext(c, adminID, ticket.ID)
	if h.pluginManager != nil {
		originalReq := req
		hookPayload := map[string]interface{}{
			"ticket_id":    ticket.ID,
			"ticket_no":    ticket.TicketNo,
			"admin_id":     adminID,
			"status":       ticket.Status,
			"content":      req.Content,
			"content_type": req.ContentType,
			"source":       "admin_api",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "ticket.message.admin.before",
			Payload: hookPayload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("ticket.message.admin.before hook execution failed: admin=%d ticket=%d err=%v", adminID, ticket.ID, hookErr)
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
				if applyErr := applyAdminTicketMessageHookPayload(&req, hookResult.Payload); applyErr != nil {
					log.Printf("ticket.message.admin.before payload apply failed, fallback to original request: admin=%d ticket=%d err=%v", adminID, ticket.ID, applyErr)
					req = originalReq
				}
			}
		}
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
		respondAdminBizError(c, ticketbiz.ContentTooLong(cfg.Ticket.MaxContentLength))
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

	if h.pluginManager != nil {
		afterStatus := ticket.Status
		if statusRaw, exists := updates["status"]; exists {
			if status, ok := statusRaw.(models.TicketStatus); ok {
				afterStatus = status
			}
		}
		afterPayload := map[string]interface{}{
			"ticket_id":    ticket.ID,
			"ticket_no":    ticket.TicketNo,
			"message_id":   message.ID,
			"admin_id":     adminID,
			"content":      sanitizedContent,
			"content_type": contentType,
			"status":       afterStatus,
			"source":       "admin_api",
			"created_at":   message.CreatedAt.Format(time.RFC3339),
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, aid uint, tid uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "ticket.message.admin.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("ticket.message.admin.after hook execution failed: admin=%d ticket=%d err=%v", aid, tid, hookErr)
			}
		}(hookExecCtx, afterPayload, adminID, ticket.ID)
	}

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
	adminID := middleware.MustGetUserID(c)
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
	beforeStatus := ticket.Status
	beforePriority := ticket.Priority
	beforeAssignedTo := ticket.AssignedTo
	hookExecCtx := h.buildTicketHookExecutionContext(c, adminID, ticket.ID)

	if h.pluginManager != nil {
		originalReq := req
		hookPayload := map[string]interface{}{
			"ticket_id":          ticket.ID,
			"ticket_no":          ticket.TicketNo,
			"admin_id":           adminID,
			"status_before":      ticket.Status,
			"status_after":       req.Status,
			"priority_before":    ticket.Priority,
			"priority_after":     req.Priority,
			"assigned_to_before": ticket.AssignedTo,
			"assigned_to_after":  req.AssignedTo,
			"source":             "admin_api",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "ticket.update.admin.before",
			Payload: hookPayload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("ticket.update.admin.before hook execution failed: admin=%d ticket=%d err=%v", adminID, ticket.ID, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Ticket update rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if applyErr := applyAdminTicketUpdateHookPayload(&req, hookResult.Payload); applyErr != nil {
					log.Printf("ticket.update.admin.before payload apply failed, fallback to original request: admin=%d ticket=%d err=%v", adminID, ticket.ID, applyErr)
					req = originalReq
				}
			}
		}
	}
	assignHookTriggered := req.AssignedTo != nil
	if assignHookTriggered && h.pluginManager != nil {
		originalReq := req
		assignBeforePayload := map[string]interface{}{
			"ticket_id":          ticket.ID,
			"ticket_no":          ticket.TicketNo,
			"admin_id":           adminID,
			"status_before":      ticket.Status,
			"assigned_to_before": beforeAssignedTo,
			"assigned_to_after":  req.AssignedTo,
			"source":             "admin_api",
		}
		assignHookResult, assignHookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "ticket.assign.before",
			Payload: assignBeforePayload,
		}, hookExecCtx)
		if assignHookErr != nil {
			log.Printf("ticket.assign.before hook execution failed: admin=%d ticket=%d err=%v", adminID, ticket.ID, assignHookErr)
		} else if assignHookResult != nil {
			if assignHookResult.Blocked {
				reason := strings.TrimSpace(assignHookResult.BlockReason)
				if reason == "" {
					reason = "Ticket assignment rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if assignHookResult.Payload != nil {
				if applyErr := applyAdminTicketAssignHookPayload(&req, assignHookResult.Payload); applyErr != nil {
					log.Printf("ticket.assign.before payload apply failed, fallback to original request: admin=%d ticket=%d err=%v", adminID, ticket.ID, applyErr)
					req = originalReq
				}
			}
		}
	}

	updates := make(map[string]interface{})

	if req.Status != "" {
		status, ok := ticketbiz.ParseStatus(req.Status)
		if !ok {
			respondAdminBizError(c, ticketbiz.StatusInvalid())
			return
		}
		req.Status = string(status)
		updates["status"] = status
		if status == models.TicketStatusClosed || status == models.TicketStatusResolved {
			now := time.Now()
			updates["closed_at"] = now
		} else {
			updates["closed_at"] = nil
		}
	}

	if req.Priority != "" {
		priority, ok := ticketbiz.ParsePriority(req.Priority)
		if !ok {
			respondAdminBizError(c, ticketbiz.PriorityInvalid())
			return
		}
		req.Priority = string(priority)
		updates["priority"] = priority
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

	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"ticket_id":          ticket.ID,
			"ticket_no":          ticket.TicketNo,
			"admin_id":           adminID,
			"status_before":      beforeStatus,
			"status_after":       ticket.Status,
			"priority_before":    beforePriority,
			"priority_after":     ticket.Priority,
			"assigned_to_before": beforeAssignedTo,
			"assigned_to_after":  ticket.AssignedTo,
			"source":             "admin_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, aid uint, tid uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "ticket.update.admin.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("ticket.update.admin.after hook execution failed: admin=%d ticket=%d err=%v", aid, tid, hookErr)
			}
		}(hookExecCtx, afterPayload, adminID, ticket.ID)
	}

	if assignHookTriggered && h.pluginManager != nil && !ticketUintPtrEqual(beforeAssignedTo, ticket.AssignedTo) {
		assignAfterPayload := map[string]interface{}{
			"ticket_id":          ticket.ID,
			"ticket_no":          ticket.TicketNo,
			"admin_id":           adminID,
			"status_before":      beforeStatus,
			"status_after":       ticket.Status,
			"assigned_to_before": beforeAssignedTo,
			"assigned_to_after":  ticket.AssignedTo,
			"source":             "admin_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, aid uint, tid uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "ticket.assign.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("ticket.assign.after hook execution failed: admin=%d ticket=%d err=%v", aid, tid, hookErr)
			}
		}(hookExecCtx, assignAfterPayload, adminID, ticket.ID)
	}

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
	adminID := middleware.MustGetUserID(c)
	if adminID == 0 {
		return
	}

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
	if ticket.Status == models.TicketStatusClosed {
		respondAdminBizError(c, ticketbiz.ClosedCannotUpload())
		return
	}

	cfg := config.GetConfig()
	attachment := cfg.Ticket.Attachment

	file, err := c.FormFile("file")
	if err != nil {
		respondAdminBizError(c, ticketbiz.FileRequired())
		return
	}
	originalFilename := file.Filename
	originalContentType := file.Header.Get("Content-Type")
	hookExecCtx := h.buildTicketHookExecutionContext(c, adminID, ticket.ID)
	if h.pluginManager != nil {
		hookPayload := map[string]interface{}{
			"ticket_id":    ticket.ID,
			"ticket_no":    ticket.TicketNo,
			"admin_id":     adminID,
			"status":       ticket.Status,
			"filename":     file.Filename,
			"size":         file.Size,
			"content_type": file.Header.Get("Content-Type"),
			"source":       "admin_api",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "ticket.attachment.upload.before",
			Payload: hookPayload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("ticket.attachment.upload.before hook execution failed: admin=%d ticket=%d err=%v", adminID, ticket.ID, hookErr)
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
					log.Printf("ticket.attachment.upload.before payload apply failed, fallback to original metadata: admin=%d ticket=%d err=%v", adminID, ticket.ID, applyErr)
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
			respondAdminBizError(c, ticketbiz.VoiceUploadDisabled())
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
			respondAdminBizError(c, ticketbiz.AudioFormatInvalid())
			return
		}
		maxSize := int64(10 * 1024 * 1024)
		if attachment != nil && attachment.MaxVoiceSize > 0 {
			maxSize = attachment.MaxVoiceSize
		}
		if file.Size > maxSize {
			respondAdminBizError(c, ticketbiz.VoiceFileTooLarge(ticketbiz.BytesToMegabytes(maxSize)))
			return
		}
	} else {
		if attachment != nil && !attachment.EnableImage {
			respondAdminBizError(c, ticketbiz.ImageUploadDisabled())
			return
		}
		maxSize := int64(5 * 1024 * 1024)
		if attachment != nil && attachment.MaxImageSize > 0 {
			maxSize = attachment.MaxImageSize
		}
		if file.Size > maxSize {
			respondAdminBizError(c, ticketbiz.ImageFileTooLarge(ticketbiz.BytesToMegabytes(maxSize)))
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
			respondAdminBizError(c, ticketbiz.ImageFormatUnsupported())
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
			"admin_id":      adminID,
			"status":        ticket.Status,
			"filename":      filename,
			"original_name": file.Filename,
			"size":          file.Size,
			"url":           fileURL,
			"source":        "admin_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, aid uint, tid uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "ticket.attachment.upload.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("ticket.attachment.upload.after hook execution failed: admin=%d ticket=%d err=%v", aid, tid, hookErr)
			}
		}(hookExecCtx, afterPayload, adminID, ticket.ID)
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

func maskString(s string) string {
	if len(s) <= 2 {
		return "**"
	}
	return s[:1] + strings.Repeat("*", len(s)-2) + s[len(s)-1:]
}
