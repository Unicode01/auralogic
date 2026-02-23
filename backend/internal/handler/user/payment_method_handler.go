package user

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"auralogic/internal/models"
	"auralogic/internal/pkg/response"
	"auralogic/internal/service"
	"gorm.io/gorm"
)

// PaymentMethodHandler 用户付款方式处理器
type PaymentMethodHandler struct {
	service        *service.PaymentMethodService
	db             *gorm.DB
	pollingService *service.PaymentPollingService
}

// NewPaymentMethodHandler 创建用户付款方式处理器
func NewPaymentMethodHandler(db *gorm.DB, pollingService *service.PaymentPollingService) *PaymentMethodHandler {
	return &PaymentMethodHandler{
		service:        service.NewPaymentMethodService(db),
		db:             db,
		pollingService: pollingService,
	}
}

// List 获取可用的付款方式列表
func (h *PaymentMethodHandler) List(c *gin.Context) {
	methods, err := h.service.GetEnabledMethods()
	if err != nil {
		response.InternalError(c, "Failed to get payment methods")
		return
	}

	// 返回简化的付款方式信息（不包含脚本和配置详情）
	var items []gin.H
	for _, pm := range methods {
		items = append(items, gin.H{
			"id":          pm.ID,
			"name":        pm.Name,
			"description": pm.Description,
			"icon":        pm.Icon,
			"type":        pm.Type,
		})
	}

	response.Success(c, gin.H{"items": items})
}

// GetPaymentCard 获取订单的付款卡片
func (h *PaymentMethodHandler) GetPaymentCard(c *gin.Context) {
	orderNo := c.Param("order_no")
	paymentMethodID, err := strconv.ParseUint(c.Query("payment_method_id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid payment method ID")
		return
	}

	// 获取当前用户
	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "User not logged in")
		return
	}

	// 获取订单
	var order models.Order
	if err := h.db.Where("order_no = ? AND user_id = ?", orderNo, userID).First(&order).Error; err != nil {
		response.NotFound(c, "Order not found")
		return
	}

	// 验证订单状态
	if order.Status != models.OrderStatusPendingPayment {
		response.BadRequest(c, "Order status does not support payment")
		return
	}

	// 生成付款卡片
	result, err := h.service.GeneratePaymentCard(uint(paymentMethodID), &order)
	if err != nil {
		response.InternalError(c, "Failed to generate payment info")
		return
	}

	response.Success(c, result)
}

// SelectPaymentMethod 选择付款方式
func (h *PaymentMethodHandler) SelectPaymentMethod(c *gin.Context) {
	orderNo := c.Param("order_no")

	var req struct {
		PaymentMethodID uint `json:"payment_method_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	// 获取当前用户
	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "User not logged in")
		return
	}

	// 获取订单
	var order models.Order
	if err := h.db.Where("order_no = ? AND user_id = ?", orderNo, userID).First(&order).Error; err != nil {
		response.NotFound(c, "Order not found")
		return
	}

	// 选择付款方式
	if err := h.service.SelectPaymentMethod(order.ID, req.PaymentMethodID); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// 将订单加入付款状态轮询队列
	if h.pollingService != nil {
		h.pollingService.AddToQueue(order.ID, req.PaymentMethodID)
	}

	// 生成付款卡片并缓存
	result, err := h.service.GeneratePaymentCard(req.PaymentMethodID, &order)
	if err != nil {
		response.InternalError(c, "Failed to generate payment info")
		return
	}

	// 缓存付款卡片到数据库
	if err := h.service.CachePaymentCard(order.ID, result); err != nil {
		// 缓存失败不影响主流程，记录日志即可
		// log.Printf("Failed to cache payment card: %v", err)
	}

	response.Success(c, result)
}

// GetOrderPaymentInfo 获取订单当前的付款信息
func (h *PaymentMethodHandler) GetOrderPaymentInfo(c *gin.Context) {
	orderNo := c.Param("order_no")

	// 获取当前用户
	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "User not logged in")
		return
	}

	// 获取订单
	var order models.Order
	if err := h.db.Where("order_no = ? AND user_id = ?", orderNo, userID).First(&order).Error; err != nil {
		response.NotFound(c, "Order not found")
		return
	}

	// 获取订单选择的付款方式
	pm, opm, err := h.service.GetOrderPaymentMethod(order.ID)
	if err != nil {
		response.InternalError(c, "Failed to get payment info")
		return
	}

	if pm == nil {
		// 未选择付款方式，返回可用的付���方式列表
		methods, _ := h.service.GetEnabledMethods()
		var items []gin.H
		for _, m := range methods {
			items = append(items, gin.H{
				"id":          m.ID,
				"name":        m.Name,
				"description": m.Description,
				"icon":        m.Icon,
			})
		}
		response.Success(c, gin.H{
			"selected":          false,
			"available_methods": items,
		})
		return
	}

	// 已选择付款方式，优先使用缓存的付款卡片
	result, err := h.service.GetCachedPaymentCard(order.ID)
	if err != nil || result == nil {
		// 缓存不存在或失败，重新生成并缓存
		result, err = h.service.GeneratePaymentCard(pm.ID, &order)
		if err != nil {
			response.InternalError(c, "Failed to generate payment info")
			return
		}
		_ = h.service.CachePaymentCard(order.ID, result)
	}

	response.Success(c, gin.H{
		"selected":        true,
		"payment_method":  gin.H{"id": pm.ID, "name": pm.Name, "icon": pm.Icon},
		"payment_card":    result,
		"order_payment":   opm,
	})
}
