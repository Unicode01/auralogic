package admin

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"auralogic/internal/config"
	"auralogic/internal/database"
	"auralogic/internal/middleware"
	"auralogic/internal/models"
	"auralogic/internal/pkg/logger"
	"auralogic/internal/pkg/response"
	"auralogic/internal/pkg/validator"
	"auralogic/internal/service"
)

type OrderHandler struct {
	orderService            *service.OrderService
	serialService           *service.SerialService
	virtualInventoryService *service.VirtualInventoryService
	cfg                     *config.Config
}

func NewOrderHandler(orderService *service.OrderService, serialService *service.SerialService, virtualInventoryService *service.VirtualInventoryService, cfg *config.Config) *OrderHandler {
	return &OrderHandler{
		orderService:            orderService,
		serialService:           serialService,
		virtualInventoryService: virtualInventoryService,
		cfg:                     cfg,
	}
}

// hasPrivacyPermission Check if admin has permission to view privacy info
// Note: Even super admin needs explicit order.view_privacy permission to view privacy orders
// Only shipping managers and similar roles who need to view shipping info should have this permission
func (h *OrderHandler) hasPrivacyPermission(c *gin.Context) bool {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		return false
	}

	db := database.GetDB()

	// Query admin permission
	var perm models.AdminPermission
	if err := db.Where("user_id = ?", userID).First(&perm).Error; err != nil {
		// If no permission record exists, return false (including super admin)
		return false
	}

	// Check for order.view_privacy permission
	hasPerm := perm.HasPermission("order.view_privacy")
	return hasPerm
}

// ListOrders Order List
func (h *OrderHandler) ListOrders(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	status := c.Query("status")
	search := c.Query("search")
	country := c.Query("country")
	productSearch := c.Query("product_search") // 新增：按ProductSKU/名称搜索
	promoCode := strings.ToUpper(strings.TrimSpace(c.Query("promo_code")))
	promoCodeIDStr := c.Query("promo_code_id")
	userIDStr := c.Query("user_id")

	if limit > 100 {
		limit = 100
	}

	// 解析 user_id 参数
	var userID *uint
	if userIDStr != "" {
		if uid, err := strconv.ParseUint(userIDStr, 10, 32); err == nil {
			uidUint := uint(uid)
			userID = &uidUint
		}
	}

	var promoCodeID *uint
	if promoCodeIDStr != "" {
		if pid, err := strconv.ParseUint(promoCodeIDStr, 10, 32); err == nil {
			pidUint := uint(pid)
			promoCodeID = &pidUint
		}
	}

	orders, total, err := h.orderService.ListOrders(page, limit, status, search, country, productSearch, promoCodeID, promoCode, userID)
	if err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	// Check if admin has privacy view permission, mask if not
	hasPrivacyPerm := h.hasPrivacyPermission(c)
	for i := range orders {
		h.orderService.MaskOrderIfNeeded(&orders[i], hasPrivacyPerm)
	}

	response.Paginated(c, orders, page, limit, total)
}

// GetOrderCountries get所有有Order的国家列表
func (h *OrderHandler) GetOrderCountries(c *gin.Context) {
	countries, err := h.orderService.GetOrderCountries()
	if err != nil {
		response.InternalError(c, "Failed to get country list")
		return
	}

	response.Success(c, gin.H{
		"countries": countries,
	})
}

// GetOrder - Get order details
func (h *OrderHandler) GetOrder(c *gin.Context) {
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid order ID format")
		return
	}

	order, err := h.orderService.GetOrderByID(uint(orderID))
	if err != nil {
		response.NotFound(c, "Order not found")
		return
	}

	// Check if admin has privacy view permission, mask if not
	hasPrivacyPerm := h.hasPrivacyPermission(c)
	h.orderService.MaskOrderIfNeeded(order, hasPrivacyPerm)

	// 获取该订单的序列号
	var serials interface{}
	if h.serialService != nil {
		serialList, err := h.serialService.GetSerialsByOrderID(uint(orderID))
		if err == nil && len(serialList) > 0 {
			serials = serialList
		}
	}

	// 获取该订单的虚拟产品库存（只有已付款后才返回）
	var virtualStocks interface{}
	if h.virtualInventoryService != nil && order.Status != models.OrderStatusPendingPayment && order.Status != models.OrderStatusDraft && order.Status != models.OrderStatusNeedResubmit {
		stockList, err := h.virtualInventoryService.GetStockByOrderID(uint(orderID))
		if err == nil && len(stockList) > 0 {
			virtualStocks = stockList
		}
	}

	// 获取订单付款信息
	var paymentInfo interface{}
	db := database.GetDB()
	var opm models.OrderPaymentMethod
	if err := db.Where("order_id = ?", orderID).First(&opm).Error; err == nil {
		var pm models.PaymentMethod
		if err := db.First(&pm, opm.PaymentMethodID).Error; err == nil {
			paymentInfo = gin.H{
				"payment_method": gin.H{
					"id":   pm.ID,
					"name": pm.Name,
					"icon": pm.Icon,
					"type": pm.Type,
				},
				"selected_at":   opm.CreatedAt,
				"payment_data":  opm.PaymentData,
			}
		}
	}

	// 返回订单信息和序列号
	response.Success(c, gin.H{
		"order":          order,
		"serials":        serials,
		"virtual_stocks": virtualStocks,
		"payment_info":   paymentInfo,
	})
}

// AssignTrackingRequest 分配物流单号请求
type AssignTrackingRequest struct {
	TrackingNo string `json:"tracking_no" binding:"required"`
}

// AssignTracking 分配物流单号
func (h *OrderHandler) AssignTracking(c *gin.Context) {
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid order ID format")
		return
	}

	var req AssignTrackingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	// 清理和验证物流单号（最大100个字符）
	req.TrackingNo = validator.SanitizeInput(req.TrackingNo)
	if !validator.ValidateLength(req.TrackingNo, 1, 100) {
		response.BadRequest(c, "Tracking number length must be between 1-100 characters")
		return
	}

	if err := h.orderService.AssignTracking(uint(orderID), req.TrackingNo); err != nil {
		response.InternalError(c, "Failed to assign tracking number")
		return
	}

	order, _ := h.orderService.GetOrderByID(uint(orderID))

	// 记录操作日志
	db := database.GetDB()
	logger.LogOrderOperation(db, c, "assign_tracking", order.ID, map[string]interface{}{
		"order_no":    order.OrderNo,
		"tracking_no": req.TrackingNo,
		"status":      order.Status,
	})

	response.Success(c, gin.H{
		"order_no":    order.OrderNo,
		"tracking_no": order.TrackingNo,
		"status":      order.Status,
		"shipped_at":  order.ShippedAt,
	})
}

// CompleteOrderRequest - Complete order request
type CompleteOrderRequest struct {
	AdminRemark string `json:"admin_remark"`
}

// CompleteOrder Admin标记Order完成
func (h *OrderHandler) CompleteOrder(c *gin.Context) {
	adminID := middleware.MustGetUserID(c)
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid order ID format")
		return
	}

	var req CompleteOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 允许不传body
	}

	// 清理Admin备注（最大1000个字符）
	req.AdminRemark = validator.SanitizeText(req.AdminRemark)
	if !validator.ValidateLength(req.AdminRemark, 0, 1000) {
		response.BadRequest(c, "Admin remark length cannot exceed 1000 characters")
		return
	}

	if err := h.orderService.CompleteOrder(uint(orderID), adminID, "", req.AdminRemark); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	order, _ := h.orderService.GetOrderByID(uint(orderID))

	// 记录操作日志
	db := database.GetDB()
	logger.LogOrderOperation(db, c, "complete", order.ID, map[string]interface{}{
		"order_no":     order.OrderNo,
		"admin_remark": req.AdminRemark,
		"completed_by": adminID,
	})

	response.Success(c, gin.H{
		"order_no":     order.OrderNo,
		"status":       order.Status,
		"completed_at": order.CompletedAt,
	})
}

// CancelOrderRequest 取消Order请求
type CancelOrderRequest struct {
	Reason string `json:"reason"`
}

// CancelOrder 取消Order
func (h *OrderHandler) CancelOrder(c *gin.Context) {
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid order ID format")
		return
	}

	var req CancelOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 允许不传body
	}

	// 清理取消原因（最大500个字符）
	req.Reason = validator.SanitizeText(req.Reason)
	if !validator.ValidateLength(req.Reason, 0, 500) {
		response.BadRequest(c, "Cancellation reason length cannot exceed 500 characters")
		return
	}

	if err := h.orderService.CancelOrder(uint(orderID), req.Reason); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	order, _ := h.orderService.GetOrderByID(uint(orderID))

	// 记录操作日志
	db := database.GetDB()
	logger.LogOrderOperation(db, c, "cancel", order.ID, map[string]interface{}{
		"order_no": order.OrderNo,
		"reason":   req.Reason,
	})

	response.Success(c, gin.H{
		"order_no": order.OrderNo,
		"status":   order.Status,
		"message":  "Order Cancelled",
	})
}

// MarkAsPaid 标记订单为已付款
func (h *OrderHandler) MarkAsPaid(c *gin.Context) {
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid order ID format")
		return
	}

	if err := h.orderService.MarkAsPaid(uint(orderID)); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	order, _ := h.orderService.GetOrderByID(uint(orderID))

	// 记录操作日志
	db := database.GetDB()
	logger.LogOrderOperation(db, c, "mark_paid", order.ID, map[string]interface{}{
		"order_no": order.OrderNo,
	})

	response.Success(c, gin.H{
		"order_no": order.OrderNo,
		"status":   order.Status,
		"message":  "订单已标记为已付款",
	})
}

// DeliverVirtualStock 手动发货虚拟商品库存
func (h *OrderHandler) DeliverVirtualStock(c *gin.Context) {
	adminID := middleware.MustGetUserID(c)
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid order ID format")
		return
	}

	if err := h.orderService.DeliverVirtualStock(uint(orderID), adminID); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	order, _ := h.orderService.GetOrderByID(uint(orderID))

	// 记录操作日志
	db := database.GetDB()
	logger.LogOrderOperation(db, c, "deliver_virtual_stock", order.ID, map[string]interface{}{
		"order_no":     order.OrderNo,
		"delivered_by": adminID,
		"status":       order.Status,
	})

	response.Success(c, gin.H{
		"order_no": order.OrderNo,
		"status":   order.Status,
		"message":  "虚拟商品已发货",
	})
}

// DeleteOrder DeleteOrder
func (h *OrderHandler) DeleteOrder(c *gin.Context) {
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid order ID format")
		return
	}

	order, _ := h.orderService.GetOrderByID(uint(orderID))

	if err := h.orderService.DeleteOrder(uint(orderID)); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// 记录操作日志
	db := database.GetDB()
	logger.LogOrderOperation(db, c, "delete", uint(orderID), map[string]interface{}{
		"order_no": order.OrderNo,
	})

	response.Success(c, gin.H{
		"message": "Order deleted",
	})
}

// UpdateShippingInfoRequest Update收货Info请求
type UpdateShippingInfoRequest struct {
	ReceiverName     string `json:"receiver_name"`
	PhoneCode        string `json:"phone_code"`
	ReceiverPhone    string `json:"receiver_phone"`
	ReceiverEmail    string `json:"receiver_email"`
	ReceiverCountry  string `json:"receiver_country"`
	ReceiverProvince string `json:"receiver_province"`
	ReceiverCity     string `json:"receiver_city"`
	ReceiverDistrict string `json:"receiver_district"`
	ReceiverAddress  string `json:"receiver_address"`
	ReceiverPostcode string `json:"receiver_postcode"`
}

// UpdateShippingInfo UpdateOrder收货Info（need order.edit Permission）
func (h *OrderHandler) UpdateShippingInfo(c *gin.Context) {
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid order ID format")
		return
	}

	var req UpdateShippingInfoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	// ============= 输入验证和清理 =============
	if req.ReceiverName != "" {
		req.ReceiverName = validator.SanitizeInput(req.ReceiverName)
		if !validator.ValidateLength(req.ReceiverName, 1, 100) {
			response.BadRequest(c, "Receiver name length must be between 1-100 characters")
			return
		}
	}

	if req.PhoneCode != "" {
		req.PhoneCode = validator.SanitizeInput(req.PhoneCode)
		if !validator.ValidatePhoneCode(req.PhoneCode) {
			response.BadRequest(c, "Invalid phone code format")
			return
		}
	}

	if req.ReceiverPhone != "" {
		req.ReceiverPhone = validator.SanitizeInput(req.ReceiverPhone)
		if !validator.ValidateLength(req.ReceiverPhone, 1, 50) || !validator.ValidatePhone(req.ReceiverPhone) {
			response.BadRequest(c, "Invalid phone number format or length")
			return
		}
	}

	if req.ReceiverEmail != "" {
		req.ReceiverEmail = validator.SanitizeInput(req.ReceiverEmail)
		if !validator.ValidateLength(req.ReceiverEmail, 0, 255) {
			response.BadRequest(c, "Email length cannot exceed 255 characters")
			return
		}
	}

	if req.ReceiverCountry != "" {
		req.ReceiverCountry = strings.ToUpper(validator.SanitizeInput(req.ReceiverCountry))
		if !validator.ValidateCountryCode(req.ReceiverCountry) {
			response.BadRequest(c, "Invalid country code format")
			return
		}
	}

	if req.ReceiverProvince != "" {
		req.ReceiverProvince = validator.SanitizeInput(req.ReceiverProvince)
		if !validator.ValidateLength(req.ReceiverProvince, 0, 50) {
			response.BadRequest(c, "Province length cannot exceed 50 characters")
			return
		}
	}

	if req.ReceiverCity != "" {
		req.ReceiverCity = validator.SanitizeInput(req.ReceiverCity)
		if !validator.ValidateLength(req.ReceiverCity, 0, 50) {
			response.BadRequest(c, "City length cannot exceed 50 characters")
			return
		}
	}

	if req.ReceiverDistrict != "" {
		req.ReceiverDistrict = validator.SanitizeInput(req.ReceiverDistrict)
		if !validator.ValidateLength(req.ReceiverDistrict, 0, 50) {
			response.BadRequest(c, "District length cannot exceed 50 characters")
			return
		}
	}

	if req.ReceiverAddress != "" {
		req.ReceiverAddress = validator.SanitizeText(req.ReceiverAddress)
		if !validator.ValidateLength(req.ReceiverAddress, 1, 500) {
			response.BadRequest(c, "Detailed address length must be between 1-500 characters")
			return
		}
	}

	if req.ReceiverPostcode != "" {
		req.ReceiverPostcode = validator.SanitizeInput(req.ReceiverPostcode)
		if !validator.ValidateLength(req.ReceiverPostcode, 0, 20) || !validator.ValidatePostcode(req.ReceiverPostcode) {
			response.BadRequest(c, "Invalid postal code format or length")
			return
		}
	}

	// QueryOrder
	order, err := h.orderService.GetOrderByID(uint(orderID))
	if err != nil {
		response.NotFound(c, "Order not found")
		return
	}

	// 只允许Update待发货和need重填状态的Order
	if order.Status != models.OrderStatusPending && order.Status != models.OrderStatusNeedResubmit {
		response.BadRequest(c, "Order status does not allow shipping information modification")
		return
	}

	// Update收货Info
	if req.ReceiverName != "" {
		order.ReceiverName = req.ReceiverName
	}
	if req.PhoneCode != "" {
		order.PhoneCode = req.PhoneCode
	}
	if req.ReceiverPhone != "" {
		order.ReceiverPhone = req.ReceiverPhone
	}
	if req.ReceiverEmail != "" {
		order.ReceiverEmail = req.ReceiverEmail
	}
	if req.ReceiverCountry != "" {
		order.ReceiverCountry = req.ReceiverCountry
	}
	if req.ReceiverProvince != "" {
		order.ReceiverProvince = req.ReceiverProvince
	}
	if req.ReceiverCity != "" {
		order.ReceiverCity = req.ReceiverCity
	}
	if req.ReceiverDistrict != "" {
		order.ReceiverDistrict = req.ReceiverDistrict
	}
	if req.ReceiverAddress != "" {
		order.ReceiverAddress = req.ReceiverAddress
	}
	if req.ReceiverPostcode != "" {
		order.ReceiverPostcode = req.ReceiverPostcode
	}

	if err := h.orderService.UpdateOrder(order); err != nil {
		response.InternalError(c, "UpdateOrderFailed")
		return
	}

	response.Success(c, gin.H{
		"order_no": order.OrderNo,
		"message":  "Shipping information updated",
	})
}

// RequestResubmitRequest 要求重填Info请求
type RequestResubmitRequest struct {
	Reason string `json:"reason" binding:"required"`
}

// RequestResubmit 要求User重填收货Info
func (h *OrderHandler) RequestResubmit(c *gin.Context) {
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid order ID format")
		return
	}

	var req RequestResubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	// 清理重填原因（最大500个字符）
	req.Reason = validator.SanitizeText(req.Reason)
	if !validator.ValidateLength(req.Reason, 1, 500) {
		response.BadRequest(c, "Resubmit reason length must be between 1-500 characters")
		return
	}

	// QueryOrder
	order, err := h.orderService.GetOrderByID(uint(orderID))
	if err != nil {
		response.NotFound(c, "Order not found")
		return
	}

	// 只允许待发货状态的Order要求重填
	if order.Status != models.OrderStatusPending {
		response.BadRequest(c, "Only orders in pending status can request resubmission")
		return
	}

	// 调用service方法要求重填
	newToken, err := h.orderService.RequestResubmit(order.ID, req.Reason)
	if err != nil {
		response.InternalError(c, "Operation failed")
		return
	}

	response.Success(c, gin.H{
		"order_no":       order.OrderNo,
		"status":         models.OrderStatusNeedResubmit,
		"new_form_token": newToken,
		"reason":         req.Reason,
		"message":        "已要求User重新填写收货Info",
	})
}

// CompleteAllShippedOrders 批量完成所有已发货订单
func (h *OrderHandler) CompleteAllShippedOrders(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	db := database.GetDB()

	// 查询所有已发货订单
	var orders []models.Order
	if err := db.Where("status = ?", models.OrderStatusShipped).Find(&orders).Error; err != nil {
		response.InternalError(c, "Failed to query shipped orders")
		return
	}

	if len(orders) == 0 {
		response.Success(c, gin.H{
			"completed_count": 0,
			"message":         "没有需要完成的已发货订单",
		})
		return
	}

	// 批量完成订单
	successCount := 0
	failedCount := 0
	var failedOrders []string

	for _, order := range orders {
		err := h.orderService.CompleteOrder(order.ID, userID, "", "批量完成操作")
		if err != nil {
			failedCount++
			failedOrders = append(failedOrders, order.OrderNo)
		} else {
			successCount++
		}
	}

	// 记录操作日志
	logger.LogOperation(db, c, "batch_complete_orders", "order", nil, map[string]interface{}{
		"success_count": successCount,
		"failed_count":  failedCount,
		"failed_orders": failedOrders,
		"total":         len(orders),
	})

	result := gin.H{
		"completed_count": successCount,
		"total_count":     len(orders),
	}

	if failedCount > 0 {
		result["failed_count"] = failedCount
		result["failed_orders"] = failedOrders
		result["message"] = "部分订单完成失败"
	} else {
		result["message"] = "所有已发货订单已完成"
	}

	response.Success(c, result)
}

// BatchUpdateOrdersRequest 批量操作订单请求
type BatchUpdateOrdersRequest struct {
	OrderIDs []uint `json:"order_ids" binding:"required,min=1"`
	Action   string `json:"action" binding:"required,oneof=complete cancel delete"`
}

// BatchUpdateOrders 批量操作订单（完成/取消/删除）
func (h *OrderHandler) BatchUpdateOrders(c *gin.Context) {
	adminID := middleware.MustGetUserID(c)

	var req BatchUpdateOrdersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	if len(req.OrderIDs) > 100 {
		response.BadRequest(c, "Cannot process more than 100 orders at once")
		return
	}

	// delete action 需要额外检查 order.delete 权限
	if req.Action == "delete" {
		db := database.GetDB()
		userID, _ := middleware.GetUserID(c)
		var perm models.AdminPermission
		if err := db.Where("user_id = ?", userID).First(&perm).Error; err != nil || !perm.HasPermission("order.delete") {
			response.Forbidden(c, "No permission to delete orders")
			return
		}
	}

	successCount := 0
	failedCount := 0
	var failedOrders []string

	for _, orderID := range req.OrderIDs {
		var err error
		switch req.Action {
		case "complete":
			err = h.orderService.CompleteOrder(orderID, adminID, "", "批量完成操作")
		case "cancel":
			err = h.orderService.CancelOrder(orderID, "批量取消操作")
		case "delete":
			err = h.orderService.DeleteOrder(orderID)
		}

		if err != nil {
			failedCount++
			order, _ := h.orderService.GetOrderByID(orderID)
			if order != nil {
				failedOrders = append(failedOrders, order.OrderNo)
			} else {
				failedOrders = append(failedOrders, strconv.FormatUint(uint64(orderID), 10))
			}
		} else {
			successCount++
		}
	}

	// 记录操作日志
	db := database.GetDB()
	logger.LogOperation(db, c, "batch_"+req.Action+"_orders", "order", nil, map[string]interface{}{
		"action":        req.Action,
		"order_ids":     req.OrderIDs,
		"success_count": successCount,
		"failed_count":  failedCount,
		"failed_orders": failedOrders,
	})

	result := gin.H{
		"success_count": successCount,
		"failed_count":  failedCount,
		"total_count":   len(req.OrderIDs),
	}

	if failedCount > 0 {
		result["failed_orders"] = failedOrders
		result["message"] = "部分订单操作失败"
	} else {
		result["message"] = "批量操作完成"
	}

	response.Success(c, result)
}

// UpdateOrderPriceRequest 修改订单价格请求
type UpdateOrderPriceRequest struct {
	TotalAmount float64 `json:"total_amount" binding:"required,min=0"`
}

// UpdateOrderPrice 修改未付款订单价格
func (h *OrderHandler) UpdateOrderPrice(c *gin.Context) {
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid order ID format")
		return
	}

	var req UpdateOrderPriceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters: "+err.Error())
		return
	}

	// 获取订单
	order, err := h.orderService.GetOrderByID(uint(orderID))
	if err != nil {
		response.NotFound(c, "Order not found")
		return
	}

	// 只允许修改待付款状态的订单价格
	if order.Status != models.OrderStatusPendingPayment {
		response.BadRequest(c, "Only pending payment orders can have price modified")
		return
	}

	// 保存修改前的价格
	oldAmount := order.TotalAmount

	// 更新订单价格
	order.TotalAmount = req.TotalAmount

	db := database.GetDB()
	if err := db.Save(order).Error; err != nil {
		response.InternalError(c, "Failed to update order price")
		return
	}

	// 记录操作日志
	logger.LogOrderOperation(db, c, "update_price", order.ID, map[string]interface{}{
		"order_no":         order.OrderNo,
		"old_total_amount": oldAmount,
		"new_total_amount": req.TotalAmount,
	})

	response.Success(c, gin.H{
		"order_no":     order.OrderNo,
		"total_amount": order.TotalAmount,
		"message":      "订单价格已更新",
	})
}

// CreateDraftRequest 创建订单草稿请求
type CreateDraftRequest struct {
	ExternalUserID  string             `json:"external_user_id" binding:"required"`
	UserEmail       string             `json:"user_email"`
	UserName        string             `json:"user_name"`
	Items           []models.OrderItem `json:"items" binding:"required"`
	ExternalOrderID string             `json:"external_order_id"`
	Platform        string             `json:"platform"`
	Remark          string             `json:"remark"`
}

// CreateDraft 创建订单草稿
func (h *OrderHandler) CreateDraft(c *gin.Context) {
	var req CreateDraftRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

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
		if errors.Is(err, service.ErrProductNotAvailable) ||
			strings.Contains(err.Error(), "does not exist") ||
			strings.Contains(err.Error(), "cannot be empty") ||
			strings.Contains(err.Error(), "must be greater than 0") {
			response.BadRequest(c, err.Error())
			return
		}
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

	db := database.GetDB()
	logger.LogOrderOperation(db, c, "create_draft", order.ID, map[string]interface{}{
		"order_no":          order.OrderNo,
		"external_user_id":  req.ExternalUserID,
		"external_order_id": req.ExternalOrderID,
		"platform":          req.Platform,
		"items_count":       len(req.Items),
	})

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

// CreateOrderForUserRequest 管理员为用户创建订单请求
type CreateOrderForUserRequest struct {
	UserID           *uint                    `json:"user_id"`
	Items            []service.AdminOrderItem `json:"items" binding:"required"`
	ReceiverName     string                   `json:"receiver_name"`
	PhoneCode        string                   `json:"phone_code"`
	ReceiverPhone    string                   `json:"receiver_phone"`
	ReceiverEmail    string                   `json:"receiver_email"`
	ReceiverCountry  string                   `json:"receiver_country"`
	ReceiverProvince string                   `json:"receiver_province"`
	ReceiverCity     string                   `json:"receiver_city"`
	ReceiverDistrict string                   `json:"receiver_district"`
	ReceiverAddress  string                   `json:"receiver_address"`
	ReceiverPostcode string                   `json:"receiver_postcode"`
	Remark           string                   `json:"remark"`
	AdminRemark      string                   `json:"admin_remark"`
	Status           string                   `json:"status"`
	TotalAmount      *float64                 `json:"total_amount"`
	UserEmail        string                   `json:"user_email"`
}

// CreateOrderForUser 管理员为用户创建订单
func (h *OrderHandler) CreateOrderForUser(c *gin.Context) {
	var req CreateOrderForUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	if len(req.Items) == 0 {
		response.BadRequest(c, "Order items cannot be empty")
		return
	}

	// 清理输入
	req.ReceiverName = validator.SanitizeInput(req.ReceiverName)
	req.ReceiverPhone = validator.SanitizeInput(req.ReceiverPhone)
	req.ReceiverEmail = validator.SanitizeInput(req.ReceiverEmail)
	req.ReceiverAddress = validator.SanitizeInput(req.ReceiverAddress)
	req.ReceiverCity = validator.SanitizeInput(req.ReceiverCity)
	req.ReceiverProvince = validator.SanitizeInput(req.ReceiverProvince)
	req.ReceiverCountry = validator.SanitizeInput(req.ReceiverCountry)
	req.ReceiverPostcode = validator.SanitizeInput(req.ReceiverPostcode)
	req.Remark = validator.SanitizeText(req.Remark)
	req.AdminRemark = validator.SanitizeText(req.AdminRemark)
	req.UserEmail = validator.SanitizeInput(req.UserEmail)

	order, err := h.orderService.CreateAdminOrder(service.AdminOrderRequest{
		UserID:           req.UserID,
		Items:            req.Items,
		ReceiverName:     req.ReceiverName,
		PhoneCode:        req.PhoneCode,
		ReceiverPhone:    req.ReceiverPhone,
		ReceiverEmail:    req.ReceiverEmail,
		ReceiverCountry:  req.ReceiverCountry,
		ReceiverProvince: req.ReceiverProvince,
		ReceiverCity:     req.ReceiverCity,
		ReceiverDistrict: req.ReceiverDistrict,
		ReceiverAddress:  req.ReceiverAddress,
		ReceiverPostcode: req.ReceiverPostcode,
		Remark:           req.Remark,
		AdminRemark:      req.AdminRemark,
		Status:           req.Status,
		TotalAmount:      req.TotalAmount,
		UserEmail:        req.UserEmail,
	})
	if err != nil {
		if strings.Contains(err.Error(), "not found") ||
			strings.Contains(err.Error(), "cannot be empty") ||
			strings.Contains(err.Error(), "must be greater than 0") ||
			strings.Contains(err.Error(), "must select") ||
			strings.Contains(err.Error(), "Failed to reserve") ||
			strings.Contains(err.Error(), "Failed to allocate") {
			response.BadRequest(c, err.Error())
			return
		}
		response.InternalError(c, "Failed to create order")
		return
	}

	db := database.GetDB()
	logger.LogOrderOperation(db, c, "admin_create_order", order.ID, map[string]interface{}{
		"order_no": order.OrderNo,
		"user_id":  req.UserID,
		"status":   order.Status,
	})

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
		"created_at": order.CreatedAt,
	})
}
