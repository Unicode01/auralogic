package api

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"auralogic/internal/config"
	"auralogic/internal/database"
	"auralogic/internal/models"
	"auralogic/internal/pkg/logger"
	"auralogic/internal/pkg/response"
	"auralogic/internal/pkg/validator"
	"auralogic/internal/service"
)

type OrderHandler struct {
	orderService *service.OrderService
	cfg          *config.Config
}

func NewOrderHandler(orderService *service.OrderService, cfg *config.Config) *OrderHandler {
	return &OrderHandler{
		orderService: orderService,
		cfg:          cfg,
	}
}

// CreateDraftRequest CreateOrder草稿请求
type CreateDraftRequest struct {
	ExternalUserID  string             `json:"external_user_id" binding:"required"`
	UserEmail       string             `json:"user_email"`
	UserName        string             `json:"user_name"` // 第三方平台的User名，用于表单默认值
	Items           []models.OrderItem `json:"items" binding:"required"`
	ExternalOrderID string             `json:"external_order_id"`
	Platform        string             `json:"platform"`
	Remark          string             `json:"remark"`
}

// CreateDraft CreateOrder草稿
func (h *OrderHandler) CreateDraft(c *gin.Context) {
	var req CreateDraftRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	// 清理和验证输入
	req.ExternalUserID = validator.SanitizeInput(req.ExternalUserID)
	if !validator.ValidateLength(req.ExternalUserID, 1, 100) {
		response.BadRequest(c, "External user ID length must be between 1-100 characters")
		return
	}

	req.UserEmail = validator.SanitizeInput(req.UserEmail)
	if !validator.ValidateLength(req.UserEmail, 0, 255) {
		response.BadRequest(c, "Email length cannot exceed 255 characters")
		return
	}

	req.UserName = validator.SanitizeInput(req.UserName)
	if !validator.ValidateLength(req.UserName, 0, 100) {
		response.BadRequest(c, "Username length cannot exceed 100 characters")
		return
	}

	req.ExternalOrderID = validator.SanitizeInput(req.ExternalOrderID)
	if !validator.ValidateLength(req.ExternalOrderID, 0, 100) {
		response.BadRequest(c, "External order ID length cannot exceed 100 characters")
		return
	}

	req.Platform = validator.SanitizeInput(req.Platform)
	if !validator.ValidateLength(req.Platform, 0, 100) {
		response.BadRequest(c, "Platform name length cannot exceed 100 characters")
		return
	}

	req.Remark = validator.SanitizeText(req.Remark)
	if !validator.ValidateLength(req.Remark, 0, 1000) {
		response.BadRequest(c, "Order remark length cannot exceed 1000 characters")
		return
	}

	// CreateOrder草稿
	order, err := h.orderService.CreateDraft(
		req.Items,
		req.ExternalUserID,
		req.ExternalOrderID,
		req.Platform,
		req.UserEmail,
		req.UserName,
		req.Remark,
	)
	if err != nil {
		// Validation / expected failures should be 400, not 500.
		if errors.Is(err, service.ErrProductNotAvailable) ||
			strings.Contains(err.Error(), "does not exist") ||
			strings.Contains(err.Error(), "cannot be empty") ||
			strings.Contains(err.Error(), "must be greater than 0") {
			response.BadRequest(c, err.Error())
			return
		}
		// 记录Failed日志
		db := database.GetDB()
		logger.LogOperation(db, c, "create_draft_failed", "order", nil, map[string]interface{}{
			"external_user_id":  req.ExternalUserID,
			"external_order_id": req.ExternalOrderID,
			"platform":          req.Platform,
			"error":             err.Error(),
		})
		response.InternalError(c, "Failed to create order")
		return
	}

	// 记录Success日志
	db := database.GetDB()
	logger.LogOrderOperation(db, c, "create_draft", order.ID, map[string]interface{}{
		"order_no":          order.OrderNo,
		"external_user_id":  req.ExternalUserID,
		"external_order_id": req.ExternalOrderID,
		"platform":          req.Platform,
		"items_count":       len(req.Items),
	})

	// 构建表单URL
	var formURL string
	if order.FormToken != nil {
		formURL = h.cfg.App.URL + "/form/shipping?token=" + *order.FormToken
	}

	response.Success(c, gin.H{
		"order_id":   order.ID,
		"order_no":   order.OrderNo,
		"form_url":   formURL,
		"form_token": order.FormToken,
		"status":     order.Status,
		"expires_at": order.FormExpiresAt,
		"created_at": order.CreatedAt,
	})
}

// GetOrder QueryOrder
func (h *OrderHandler) GetOrder(c *gin.Context) {
	orderNo := c.Param("order_no")
	if orderNo == "" {
		response.BadRequest(c, "Order number cannot be empty")
		return
	}

	order, err := h.orderService.GetOrderByNo(orderNo)
	if err != nil {
		response.NotFound(c, "Order not found")
		return
	}

	response.Success(c, order)
}

// AssignTrackingRequest 分配物流单号请求
type AssignTrackingRequest struct {
	TrackingNo string `json:"tracking_no" binding:"required"`
}

// AssignTracking 分配物流单号
func (h *OrderHandler) AssignTracking(c *gin.Context) {
	orderNo := c.Param("order_no")
	if orderNo == "" {
		response.BadRequest(c, "Order number cannot be empty")
		return
	}

	var req AssignTrackingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	// 清理和验证物流单号
	req.TrackingNo = validator.SanitizeInput(req.TrackingNo)
	if !validator.ValidateLength(req.TrackingNo, 1, 100) {
		response.BadRequest(c, "Tracking number length must be between 1-100 characters")
		return
	}

	// Find order
	order, err := h.orderService.GetOrderByNo(orderNo)
	if err != nil {
		response.NotFound(c, "Order not found")
		return
	}

	// 分配物流单号
	if err := h.orderService.AssignTracking(order.ID, req.TrackingNo); err != nil {
		response.InternalError(c, "Failed to assign tracking number")
		return
	}

	// Re-query order
	order, _ = h.orderService.GetOrderByID(order.ID)

	response.Success(c, gin.H{
		"order_no":    order.OrderNo,
		"tracking_no": order.TrackingNo,
		"status":      order.Status,
		"shipped_at":  order.ShippedAt,
	})
}

// RequestResubmitRequest 要求重填Info请求
type RequestResubmitRequest struct {
	Reason string `json:"reason" binding:"required"`
}

// RequestResubmit 要求重填Info
func (h *OrderHandler) RequestResubmit(c *gin.Context) {
	orderNo := c.Param("order_no")
	if orderNo == "" {
		response.BadRequest(c, "Order number cannot be empty")
		return
	}

	var req RequestResubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	// 清理重填原因
	req.Reason = validator.SanitizeText(req.Reason)
	if !validator.ValidateLength(req.Reason, 1, 500) {
		response.BadRequest(c, "Resubmit reason length must be between 1-500 characters")
		return
	}

	// Find order
	order, err := h.orderService.GetOrderByNo(orderNo)
	if err != nil {
		response.NotFound(c, "Order not found")
		return
	}

	// 要求重填
	newToken, err := h.orderService.RequestResubmit(order.ID, req.Reason)
	if err != nil {
		response.InternalError(c, "Operation failed")
		return
	}

	// 构建表单URL
	formURL := h.cfg.App.URL + "/form/shipping?token=" + newToken

	response.Success(c, gin.H{
		"order_no":       order.OrderNo,
		"status":         models.OrderStatusNeedResubmit,
		"form_url":       formURL,
		"new_form_token": newToken,
		"reason":         req.Reason,
	})
}

// ListOrders QueryOrder List
func (h *OrderHandler) ListOrders(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	status := c.Query("status")
	search := c.Query("search")
	country := c.Query("country")
	productSearch := c.Query("product_search") // ProductSKU/名称搜索
	promoCode := strings.ToUpper(strings.TrimSpace(c.Query("promo_code")))
	promoCodeIDStr := c.Query("promo_code_id")

	if limit > 100 {
		limit = 100
	}

	var promoCodeID *uint
	if promoCodeIDStr != "" {
		if pid, err := strconv.ParseUint(promoCodeIDStr, 10, 32); err == nil {
			pidUint := uint(pid)
			promoCodeID = &pidUint
		}
	}

	orders, total, err := h.orderService.ListOrders(page, limit, status, search, country, productSearch, promoCodeID, promoCode, nil)
	if err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	response.Paginated(c, orders, page, limit, total)
}
