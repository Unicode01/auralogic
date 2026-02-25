package user

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"auralogic/internal/middleware"
	"auralogic/internal/models"
	"auralogic/internal/pkg/response"
	"auralogic/internal/pkg/validator"
	"auralogic/internal/service"
)

type OrderHandler struct {
	orderService            *service.OrderService
	bindingService          *service.BindingService
	virtualInventoryService *service.VirtualInventoryService
}

func NewOrderHandler(orderService *service.OrderService, bindingService *service.BindingService, virtualInventoryService *service.VirtualInventoryService) *OrderHandler {
	return &OrderHandler{
		orderService:            orderService,
		bindingService:          bindingService,
		virtualInventoryService: virtualInventoryService,
	}
}

// CreateOrderRequest - Create order request
type CreateOrderRequest struct {
	Items     []models.OrderItem `json:"items" binding:"required"`
	Remark    string             `json:"remark"`
	PromoCode string             `json:"promo_code"`
}

// CreateOrder CreateOrder
func (h *OrderHandler) CreateOrder(c *gin.Context) {
	userID := middleware.MustGetUserID(c)

	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	// Validate product items
	if len(req.Items) == 0 {
		response.BadRequest(c, "Order items cannot be empty")
		return
	}

	// 清理备注（最大500个字符）
	req.Remark = validator.SanitizeText(req.Remark)
	if !validator.ValidateLength(req.Remark, 0, 500) {
		response.BadRequest(c, "Order remark length cannot exceed 500 characters")
		return
	}

	// 清理优惠码
	req.PromoCode = validator.SanitizeInput(req.PromoCode)
	if !validator.ValidateLength(req.PromoCode, 0, 50) {
		response.BadRequest(c, "Promo code length cannot exceed 50 characters")
		return
	}

	// Create order draft (internal user)
	order, err := h.orderService.CreateUserOrder(userID, req.Items, req.Remark, req.PromoCode)
	if err != nil {
		if errors.Is(err, service.ErrProductNotAvailable) {
			response.BadRequest(c, err.Error())
			return
		}
		response.InternalError(c, "Failed to create order")
		return
	}

	response.Success(c, gin.H{
		"order_id":   order.ID,
		"order_no":   order.OrderNo,
		"status":     order.Status,
		"created_at": order.CreatedAt,
	})
}

// ListOrders - Get my order list
func (h *OrderHandler) ListOrders(c *gin.Context) {
	userID := middleware.MustGetUserID(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	status := c.Query("status")

	if page < 1 {
		page = 1
	}
	if limit > 100 {
		limit = 100
	}

	orders, total, err := h.orderService.ListUserOrders(userID, page, limit, status)
	if err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	// 获取所有订单的shared_to_support状态
	orderIDs := make([]uint, len(orders))
	for i, order := range orders {
		orderIDs[i] = order.ID
	}
	sharedMap, _ := h.orderService.GetSharedOrderIDs(orderIDs)

	// 构建带有shared_to_support标记的订单列表
	type OrderWithShared struct {
		models.Order
		SharedToSupport bool `json:"shared_to_support"`
	}
	result := make([]OrderWithShared, len(orders))
	for i, order := range orders {
		// 未付款订单隐藏盲盒分配结果
		if order.Status == models.OrderStatusPendingPayment ||
			order.Status == models.OrderStatusDraft ||
			order.Status == models.OrderStatusNeedResubmit ||
			order.Status == models.OrderStatusCancelled {
			order.ActualAttributes = ""
		}
		result[i] = OrderWithShared{
			Order:           order,
			SharedToSupport: sharedMap[order.ID],
		}
	}

	response.Paginated(c, result, page, limit, total)
}

// GetOrder - Get order details
func (h *OrderHandler) GetOrder(c *gin.Context) {
	userID := middleware.MustGetUserID(c)
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

	// Check if order belongs to current user
	if order.UserID == nil || *order.UserID != userID {
		response.Forbidden(c, "No permission to access this order")
		return
	}

	// 检查订单是否被分享到客服工单
	sharedToSupport, _ := h.orderService.IsOrderSharedToSupport(order.ID)

	// 处理盲盒属性：已付款订单将盲盒结果合并回items，未付款订单隐藏盲盒结果
	isPaid := order.Status != models.OrderStatusPendingPayment &&
		order.Status != models.OrderStatusDraft &&
		order.Status != models.OrderStatusNeedResubmit &&
		order.Status != models.OrderStatusCancelled

	responseItems := order.Items
	if isPaid && len(order.ActualAttributes) > 0 {
		// 已付款：将 ActualAttributes 中的盲盒属性合并回 items
		var actualMap map[string]map[string]interface{}
		if err := json.Unmarshal([]byte(order.ActualAttributes), &actualMap); err == nil {
			// 复制 items 以免修改原始数据
			responseItems = make([]models.OrderItem, len(order.Items))
			copy(responseItems, order.Items)
			for idxStr, bbVals := range actualMap {
				var idx int
				if _, err := fmt.Sscanf(idxStr, "%d", &idx); err == nil && idx >= 0 && idx < len(responseItems) {
					if responseItems[idx].Attributes == nil {
						responseItems[idx].Attributes = make(map[string]interface{})
					}
					for k, v := range bbVals {
						responseItems[idx].Attributes[k] = v
					}
				}
			}
		}
	}
	// 未付款订单的 items 中不含盲盒属性（在 CreateUserOrder 中已剥离）

	response.Success(c, gin.H{
		"id":                          order.ID,
		"order_no":                    order.OrderNo,
		"user_id":                     order.UserID,
		"items":                       responseItems,
		"status":                      order.Status,
		"receiver_name":               order.ReceiverName,
		"phone_code":                  order.PhoneCode,
		"receiver_phone":              order.ReceiverPhone,
		"receiver_email":              order.ReceiverEmail,
		"receiver_country":            order.ReceiverCountry,
		"receiver_province":           order.ReceiverProvince,
		"receiver_city":               order.ReceiverCity,
		"receiver_district":           order.ReceiverDistrict,
		"receiver_address":            order.ReceiverAddress,
		"receiver_postcode":           order.ReceiverPostcode,
		"privacy_protected":           order.PrivacyProtected,
		"tracking_no":                 order.TrackingNo,
		"shipped_at":                  order.ShippedAt,
		"completed_at":                order.CompletedAt,
		"form_submitted_at":           order.FormSubmittedAt,
		"user_email":                  order.UserEmail,
		"email_notifications_enabled": order.EmailNotificationsEnabled,
		"total_amount":                order.TotalAmount,
		"currency":                    order.Currency,
		"remark":                      order.Remark,
		"created_at":                  order.CreatedAt,
		"updated_at":                  order.UpdatedAt,
		"shared_to_support":           sharedToSupport,
	})
}

// CompleteOrderRequest - Complete order request
type CompleteOrderRequest struct {
	Feedback string `json:"feedback"`
}

// CompleteOrder - User confirms order completion
func (h *OrderHandler) CompleteOrder(c *gin.Context) {
	userID := middleware.MustGetUserID(c)
	orderNo := c.Param("order_no")

	if orderNo == "" {
		response.BadRequest(c, "Order number cannot be empty")
		return
	}

	var req CompleteOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	// Find order
	order, err := h.orderService.GetOrderByNo(orderNo)
	if err != nil {
		response.NotFound(c, "Order not found")
		return
	}

	// Check if order belongs to current user
	if order.UserID == nil || *order.UserID != userID {
		response.Forbidden(c, "No permission to operate this order")
		return
	}

	// Complete order
	if err := h.orderService.CompleteOrder(order.ID, userID, req.Feedback, ""); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Re-query order
	order, _ = h.orderService.GetOrderByID(order.ID)

	response.Success(c, gin.H{
		"order_no":     order.OrderNo,
		"status":       order.Status,
		"completed_at": order.CompletedAt,
	})
}

// GetOrRefreshFormToken - Get or refresh form token
func (h *OrderHandler) GetOrRefreshFormToken(c *gin.Context) {
	userID := middleware.MustGetUserID(c)
	orderNo := c.Param("order_no")

	if orderNo == "" {
		response.BadRequest(c, "Order number cannot be empty")
		return
	}

	// Find order
	order, err := h.orderService.GetOrderByNo(orderNo)
	if err != nil {
		response.NotFound(c, "Order not found")
		return
	}

	// Check if order belongs to current user
	if order.UserID == nil || *order.UserID != userID {
		response.Forbidden(c, "No permission to access this order")
		return
	}

	// 检查Order状态是否need填写表单
	if order.Status != models.OrderStatusDraft && order.Status != models.OrderStatusNeedResubmit {
		response.BadRequest(c, "Order status does not require form submission")
		return
	}

	// get或刷新表单Token
	formToken, expiresAt, err := h.orderService.GetOrRefreshFormToken(order)
	if err != nil {
		response.InternalError(c, "Failed to get form token")
		return
	}

	response.Success(c, gin.H{
		"form_token": formToken,
		"expires_at": expiresAt,
		"order_no":   order.OrderNo,
		"order_id":   order.ID,
	})
}

// GetVirtualProducts - Get virtual product content for an order
func (h *OrderHandler) GetVirtualProducts(c *gin.Context) {
	userID := middleware.MustGetUserID(c)
	orderNo := c.Param("order_no")

	if orderNo == "" {
		response.BadRequest(c, "Order number cannot be empty")
		return
	}

	// Find order
	order, err := h.orderService.GetOrderByNo(orderNo)
	if err != nil {
		response.NotFound(c, "Order not found")
		return
	}

	// Check if order belongs to current user
	if order.UserID == nil || *order.UserID != userID {
		response.Forbidden(c, "No permission to access this order")
		return
	}

	// Check if order status allows viewing virtual products
	// Only allow viewing after payment (pending, shipped, completed)
	// pending_payment, draft, need_resubmit are not allowed
	if order.Status == models.OrderStatusPendingPayment || order.Status == models.OrderStatusDraft || order.Status == models.OrderStatusNeedResubmit {
		response.BadRequest(c, "Virtual products are not available yet")
		return
	}

	// Get virtual product stocks
	if h.virtualInventoryService == nil {
		response.Success(c, gin.H{"stocks": []interface{}{}})
		return
	}

	stocks, err := h.virtualInventoryService.GetStockByOrderNo(orderNo)
	if err != nil {
		response.InternalError(c, "Failed to get virtual products")
		return
	}

	response.Success(c, gin.H{
		"stocks": stocks,
	})
}
