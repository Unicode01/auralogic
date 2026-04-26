package admin

import (
	"encoding/json"
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/database"
	"auralogic/internal/middleware"
	"auralogic/internal/models"
	"auralogic/internal/pkg/bizerr"
	"auralogic/internal/pkg/logger"
	"auralogic/internal/pkg/orderbiz"
	"auralogic/internal/pkg/response"
	"auralogic/internal/pkg/utils"
	"auralogic/internal/pkg/validator"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type OrderHandler struct {
	orderService            *service.OrderService
	serialService           *service.SerialService
	virtualInventoryService *service.VirtualInventoryService
	jsRuntimeService        *service.JSRuntimeService
	pluginManager           *service.PluginManagerService
	cfg                     *config.Config
}

func NewOrderHandler(orderService *service.OrderService, serialService *service.SerialService, virtualInventoryService *service.VirtualInventoryService, jsRuntimeService *service.JSRuntimeService, pluginManager *service.PluginManagerService, cfg *config.Config) *OrderHandler {
	return &OrderHandler{
		orderService:            orderService,
		serialService:           serialService,
		virtualInventoryService: virtualInventoryService,
		jsRuntimeService:        jsRuntimeService,
		pluginManager:           pluginManager,
		cfg:                     cfg,
	}
}

func respondAdminOrderServiceError(c *gin.Context, err error, fallback string) bool {
	if err == nil {
		return false
	}

	var bizErr *bizerr.Error
	if errors.As(err, &bizErr) {
		response.BizError(c, bizErr.Message, bizErr.Key, bizErr.Params)
		return true
	}

	response.InternalServerError(c, fallback, err)
	return true
}

func respondAdminOrderValidationError(c *gin.Context, err error) {
	if err == nil {
		return
	}
	if respondAdminBizError(c, err) {
		return
	}
	response.BadRequest(c, err.Error())
}

func parseAdminOrderID(c *gin.Context) (uint, bool) {
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		respondAdminOrderValidationError(c, orderbiz.InvalidOrderID())
		return 0, false
	}
	return uint(orderID), true
}

func (h *OrderHandler) buildShippingFormURL(formToken *string) string {
	if formToken == nil {
		return ""
	}
	baseURL := strings.TrimRight(strings.TrimSpace(h.cfg.App.URL), "/")
	if baseURL == "" {
		return ""
	}
	return baseURL + "/form/shipping?token=" + *formToken
}

func (h *OrderHandler) buildOrderHookExecutionContext(c *gin.Context, adminID uint, orderID uint) *service.ExecutionContext {
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
		"operator_type":   "admin",
		"order_id":        strconv.FormatUint(uint64(orderID), 10),
	}
	sessionID := strings.TrimSpace(c.GetHeader("X-Session-ID"))
	return &service.ExecutionContext{
		UserID:         &adminID,
		OrderID:        &orderID,
		SessionID:      sessionID,
		Metadata:       metadata,
		RequestContext: c.Request.Context(),
	}
}

func orderValueToOptionalString(value interface{}) (string, error) {
	if value == nil {
		return "", nil
	}
	str, ok := value.(string)
	if !ok {
		return "", errors.New("value must be string")
	}
	return str, nil
}

func orderValueToOptionalBool(value interface{}) (bool, bool, error) {
	if value == nil {
		return false, false, nil
	}
	switch typed := value.(type) {
	case bool:
		return typed, true, nil
	case string:
		normalized := strings.ToLower(strings.TrimSpace(typed))
		switch normalized {
		case "1", "true", "yes", "y":
			return true, true, nil
		case "0", "false", "no", "n":
			return false, true, nil
		}
		return false, false, errors.New("invalid bool string")
	default:
		return false, false, errors.New("value must be bool")
	}
}

func orderValueToOptionalInt64(value interface{}) (int64, error) {
	if value == nil {
		return 0, nil
	}
	switch typed := value.(type) {
	case int:
		return int64(typed), nil
	case int64:
		return typed, nil
	case uint:
		return int64(typed), nil
	case uint64:
		if typed > uint64(^uint64(0)>>1) {
			return 0, errors.New("value out of range")
		}
		return int64(typed), nil
	case float64:
		if typed != float64(int64(typed)) {
			return 0, errors.New("value must be integer")
		}
		return int64(typed), nil
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return 0, nil
		}
		parsed, err := strconv.ParseInt(trimmed, 10, 64)
		if err != nil {
			return 0, errors.New("invalid int string")
		}
		return parsed, nil
	default:
		return 0, errors.New("value must be int64")
	}
}

func applyAdminCompleteOrderHookPayload(req *CompleteOrderRequest, payload map[string]interface{}) error {
	if req == nil || payload == nil {
		return nil
	}
	if raw, exists := payload["remark"]; exists {
		remark, err := orderValueToOptionalString(raw)
		if err != nil {
			return err
		}
		req.AdminRemark = remark
		return nil
	}
	if raw, exists := payload["admin_remark"]; exists {
		remark, err := orderValueToOptionalString(raw)
		if err != nil {
			return err
		}
		req.AdminRemark = remark
	}
	return nil
}

func applyAdminCancelOrderHookPayload(req *CancelOrderRequest, payload map[string]interface{}) error {
	if req == nil || payload == nil {
		return nil
	}
	if raw, exists := payload["reason"]; exists {
		reason, err := orderValueToOptionalString(raw)
		if err != nil {
			return err
		}
		req.Reason = reason
	}
	return nil
}

func applyAdminRefundOrderHookPayload(req *RefundOrderRequest, payload map[string]interface{}) error {
	if req == nil || payload == nil {
		return nil
	}
	if raw, exists := payload["reason"]; exists {
		reason, err := orderValueToOptionalString(raw)
		if err != nil {
			return err
		}
		req.Reason = reason
	}
	return nil
}

func applyAdminConfirmRefundHookPayload(req *ConfirmRefundRequest, payload map[string]interface{}) error {
	if req == nil || payload == nil {
		return nil
	}
	if raw, exists := payload["remark"]; exists {
		remark, err := orderValueToOptionalString(raw)
		if err != nil {
			return err
		}
		req.Remark = remark
	}
	if raw, exists := payload["transaction_id"]; exists {
		transactionID, err := orderValueToOptionalString(raw)
		if err != nil {
			return err
		}
		req.TransactionID = transactionID
	}
	return nil
}

func applyAdminUpdateShippingHookPayload(req *UpdateShippingInfoRequest, payload map[string]interface{}) error {
	if req == nil || payload == nil {
		return nil
	}
	if raw, exists := payload["receiver_name"]; exists {
		value, err := orderValueToOptionalString(raw)
		if err != nil {
			return err
		}
		req.ReceiverName = value
	}
	if raw, exists := payload["phone_code"]; exists {
		value, err := orderValueToOptionalString(raw)
		if err != nil {
			return err
		}
		req.PhoneCode = value
	}
	if raw, exists := payload["receiver_phone"]; exists {
		value, err := orderValueToOptionalString(raw)
		if err != nil {
			return err
		}
		req.ReceiverPhone = value
	}
	if raw, exists := payload["receiver_email"]; exists {
		value, err := orderValueToOptionalString(raw)
		if err != nil {
			return err
		}
		req.ReceiverEmail = value
	}
	if raw, exists := payload["receiver_country"]; exists {
		value, err := orderValueToOptionalString(raw)
		if err != nil {
			return err
		}
		req.ReceiverCountry = value
	}
	if raw, exists := payload["receiver_province"]; exists {
		value, err := orderValueToOptionalString(raw)
		if err != nil {
			return err
		}
		req.ReceiverProvince = value
	}
	if raw, exists := payload["receiver_city"]; exists {
		value, err := orderValueToOptionalString(raw)
		if err != nil {
			return err
		}
		req.ReceiverCity = value
	}
	if raw, exists := payload["receiver_district"]; exists {
		value, err := orderValueToOptionalString(raw)
		if err != nil {
			return err
		}
		req.ReceiverDistrict = value
	}
	if raw, exists := payload["receiver_address"]; exists {
		value, err := orderValueToOptionalString(raw)
		if err != nil {
			return err
		}
		req.ReceiverAddress = value
	}
	if raw, exists := payload["receiver_postcode"]; exists {
		value, err := orderValueToOptionalString(raw)
		if err != nil {
			return err
		}
		req.ReceiverPostcode = value
	}
	return nil
}

func applyAdminUpdateOrderPriceHookPayload(req *UpdateOrderPriceRequest, payload map[string]interface{}) error {
	if req == nil || payload == nil {
		return nil
	}
	if raw, exists := payload["total_amount_minor"]; exists {
		value, err := orderValueToOptionalInt64(raw)
		if err != nil {
			return err
		}
		req.TotalAmountMinor = &value
	}
	return nil
}

func applyAdminMarkPaidHookPayload(options *service.MarkAsPaidOptions, payload map[string]interface{}) error {
	if options == nil || payload == nil {
		return nil
	}
	if raw, exists := payload["admin_remark"]; exists {
		remark, err := orderValueToOptionalString(raw)
		if err != nil {
			return err
		}
		options.AdminRemark = remark
	}
	if raw, exists := payload["skip_auto_delivery"]; exists {
		value, ok, err := orderValueToOptionalBool(raw)
		if err != nil {
			return err
		}
		if ok {
			options.SkipAutoDelivery = value
		}
	}
	return nil
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
	page, limit := response.GetPagination(c)
	status := c.Query("status")
	search := c.Query("search")
	country := c.Query("country")
	productSearch := c.Query("product_search") // 新增：按ProductSKU/名称搜索
	promoCode := strings.ToUpper(strings.TrimSpace(c.Query("promo_code")))
	promoCodeIDStr := c.Query("promo_code_id")
	userIDStr := c.Query("user_id")

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
	orderID, ok := parseAdminOrderID(c)
	if !ok {
		return
	}

	order, err := h.orderService.GetOrderByID(orderID)
	if err != nil {
		response.NotFound(c, "Order not found")
		return
	}

	// Check if admin has privacy view permission, mask if not
	hasPrivacyPerm := h.hasPrivacyPermission(c)
	h.orderService.MaskOrderIfNeeded(order, hasPrivacyPerm)

	// 获取该订单的序列号
	var serials interface{}
	warnings := make([]string, 0, 3)
	if h.serialService != nil {
		serialList, err := h.serialService.GetSerialsByOrderID(orderID)
		if err != nil {
			log.Printf("admin.get_order failed to load serials: order_id=%d err=%v", orderID, err)
			warnings = append(warnings, "Failed to load order serials")
		} else if len(serialList) > 0 {
			serials = serialList
		}
	}

	// 获取该订单的虚拟产品库存（只有已付款后才返回）
	var virtualStocks interface{}
	hasPendingVirtualStock := false
	if h.virtualInventoryService != nil && order.Status != models.OrderStatusPendingPayment && order.Status != models.OrderStatusDraft && order.Status != models.OrderStatusNeedResubmit {
		stockList, err := h.virtualInventoryService.GetStockByOrderNo(order.OrderNo)
		if err != nil {
			log.Printf("admin.get_order failed to load virtual stocks: order_no=%s err=%v", order.OrderNo, err)
			warnings = append(warnings, "Failed to load order virtual stock")
		} else if len(stockList) > 0 {
			if !h.cfg.Order.EnableVirtualStockInlineIframe {
				for i := range stockList {
					stockList[i].Presentation = ""
				}
			}
			virtualStocks = stockList
		}
		if pending, err := h.virtualInventoryService.HasPendingVirtualStock(order.OrderNo); err != nil {
			log.Printf("admin.get_order failed to check pending virtual stock: order_no=%s err=%v", order.OrderNo, err)
			warnings = append(warnings, "Failed to check pending virtual stock")
		} else {
			hasPendingVirtualStock = pending
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
				"selected_at":                   opm.CreatedAt,
				"updated_at":                    opm.UpdatedAt,
				"payment_data":                  opm.PaymentData,
				"payment_card_cached":           strings.TrimSpace(opm.PaymentCardCache) != "",
				"payment_card_cache_expires_at": opm.CacheExpiresAt,
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("admin.get_order failed to load payment method: order_id=%d payment_method_id=%d err=%v", orderID, opm.PaymentMethodID, err)
			warnings = append(warnings, "Failed to load order payment method")
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Printf("admin.get_order failed to load payment method mapping: order_id=%d err=%v", orderID, err)
		warnings = append(warnings, "Failed to load order payment information")
	}

	// 返回订单信息和序列号
	payload := gin.H{
		"order":                     order,
		"serials":                   serials,
		"virtual_stocks":            virtualStocks,
		"has_pending_virtual_stock": hasPendingVirtualStock,
		"payment_info":              paymentInfo,
		"form_url":                  h.buildShippingFormURL(order.FormToken),
	}
	if len(warnings) > 0 {
		payload["warnings"] = warnings
	}
	response.Success(c, payload)
}

// AssignTrackingRequest 分配物流单号请求
type AssignTrackingRequest struct {
	TrackingNo string `json:"tracking_no" binding:"required"`
}

// AssignTracking 分配物流单号
func (h *OrderHandler) AssignTracking(c *gin.Context) {
	orderID, ok := parseAdminOrderID(c)
	if !ok {
		return
	}

	var req AssignTrackingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondAdminOrderValidationError(c, orderbiz.InvalidRequestParameters())
		return
	}

	// 清理和验证物流单号（最大100个字符）
	req.TrackingNo = validator.SanitizeInput(req.TrackingNo)
	if !validator.ValidateLength(req.TrackingNo, 1, 100) {
		respondAdminOrderValidationError(c, orderbiz.TrackingNumberLengthInvalid(1, 100))
		return
	}

	if err := h.orderService.AssignTracking(orderID, req.TrackingNo); err != nil {
		respondAdminOrderServiceError(c, err, "Failed to assign tracking number")
		return
	}

	order, _ := h.orderService.GetOrderByID(orderID)

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
	AdminRemark string `json:"remark"`
}

// CompleteOrder Admin标记Order完成
func (h *OrderHandler) CompleteOrder(c *gin.Context) {
	adminID, adminIDOK := middleware.RequireUserID(c)
	if !adminIDOK {
		return
	}
	orderID, ok := parseAdminOrderID(c)
	if !ok {
		return
	}

	var req CompleteOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 允许不传body
	}

	// 清理Admin备注（最大1000个字符）
	req.AdminRemark = validator.SanitizeText(req.AdminRemark)
	if !validator.ValidateLength(req.AdminRemark, 0, 1000) {
		respondAdminOrderValidationError(c, orderbiz.AdminRemarkTooLong(1000))
		return
	}

	order, err := h.orderService.GetOrderByID(orderID)
	if err != nil {
		response.NotFound(c, "Order not found")
		return
	}
	beforeStatus := order.Status
	hookExecCtx := h.buildOrderHookExecutionContext(c, adminID, uint(orderID))
	if h.pluginManager != nil {
		originalReq := req
		hookPayload := map[string]interface{}{
			"order_id":      order.ID,
			"order_no":      order.OrderNo,
			"admin_id":      adminID,
			"status_before": order.Status,
			"admin_remark":  req.AdminRemark,
			"source":        "admin_api",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "order.admin.complete.before",
			Payload: hookPayload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("order.admin.complete.before hook execution failed: admin=%d order=%s err=%v", adminID, order.OrderNo, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Order completion rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if applyErr := applyAdminCompleteOrderHookPayload(&req, hookResult.Payload); applyErr != nil {
					log.Printf("order.admin.complete.before payload apply failed, fallback to original request: admin=%d order=%s err=%v", adminID, order.OrderNo, applyErr)
					req = originalReq
				}
			}
		}
	}

	if err := h.orderService.CompleteOrder(orderID, adminID, "", req.AdminRemark); err != nil {
		respondAdminOrderServiceError(c, err, "Failed to complete order")
		return
	}

	order, _ = h.orderService.GetOrderByID(orderID)
	if h.pluginManager != nil && order != nil {
		afterPayload := map[string]interface{}{
			"order_id":      order.ID,
			"order_no":      order.OrderNo,
			"admin_id":      adminID,
			"status_before": beforeStatus,
			"status_after":  order.Status,
			"admin_remark":  req.AdminRemark,
			"completed_at":  order.CompletedAt,
			"source":        "admin_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, aid uint, orderNo string) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "order.admin.complete.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("order.admin.complete.after hook execution failed: admin=%d order=%s err=%v", aid, orderNo, hookErr)
			}
		}(hookExecCtx, afterPayload, adminID, order.OrderNo)
	}

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
	adminID, adminIDOK := middleware.RequireUserID(c)
	if !adminIDOK {
		return
	}
	orderID, ok := parseAdminOrderID(c)
	if !ok {
		return
	}

	var req CancelOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 允许不传body
	}

	// 清理取消原因（最大500个字符）
	req.Reason = validator.SanitizeText(req.Reason)
	if !validator.ValidateLength(req.Reason, 0, 500) {
		respondAdminOrderValidationError(c, orderbiz.CancellationReasonTooLong(500))
		return
	}

	order, err := h.orderService.GetOrderByID(orderID)
	if err != nil {
		response.NotFound(c, "Order not found")
		return
	}
	beforeStatus := order.Status
	hookExecCtx := h.buildOrderHookExecutionContext(c, adminID, uint(orderID))
	if h.pluginManager != nil {
		originalReq := req
		hookPayload := map[string]interface{}{
			"order_id":      order.ID,
			"order_no":      order.OrderNo,
			"admin_id":      adminID,
			"status_before": order.Status,
			"reason":        req.Reason,
			"source":        "admin_api",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "order.admin.cancel.before",
			Payload: hookPayload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("order.admin.cancel.before hook execution failed: admin=%d order=%s err=%v", adminID, order.OrderNo, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Order cancellation rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if applyErr := applyAdminCancelOrderHookPayload(&req, hookResult.Payload); applyErr != nil {
					log.Printf("order.admin.cancel.before payload apply failed, fallback to original request: admin=%d order=%s err=%v", adminID, order.OrderNo, applyErr)
					req = originalReq
				}
			}
		}
	}

	if err := h.orderService.CancelOrder(orderID, req.Reason); err != nil {
		respondAdminOrderServiceError(c, err, "Failed to cancel order")
		return
	}

	order, _ = h.orderService.GetOrderByID(orderID)
	if h.pluginManager != nil && order != nil {
		afterPayload := map[string]interface{}{
			"order_id":      order.ID,
			"order_no":      order.OrderNo,
			"admin_id":      adminID,
			"status_before": beforeStatus,
			"status_after":  order.Status,
			"reason":        req.Reason,
			"source":        "admin_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, aid uint, orderNo string) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "order.admin.cancel.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("order.admin.cancel.after hook execution failed: admin=%d order=%s err=%v", aid, orderNo, hookErr)
			}
		}(hookExecCtx, afterPayload, adminID, order.OrderNo)
	}

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

// RefundOrderRequest 退款请求
type RefundOrderRequest struct {
	Reason string `json:"reason"`
}

type ConfirmRefundRequest struct {
	TransactionID string `json:"transaction_id"`
	Remark        string `json:"remark"`
}

// RefundOrder 退款Order
func (h *OrderHandler) RefundOrder(c *gin.Context) {
	adminID, adminIDOK := middleware.RequireUserID(c)
	if !adminIDOK {
		return
	}
	orderID, ok := parseAdminOrderID(c)
	if !ok {
		return
	}

	var req RefundOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 允许不传body
	}

	req.Reason = validator.SanitizeText(req.Reason)
	if !validator.ValidateLength(req.Reason, 0, 500) {
		respondAdminOrderValidationError(c, orderbiz.RefundReasonTooLong(500))
		return
	}

	order, err := h.orderService.GetOrderByID(orderID)
	if err != nil {
		response.NotFound(c, "Order not found")
		return
	}
	beforeStatus := order.Status
	hookExecCtx := h.buildOrderHookExecutionContext(c, adminID, uint(orderID))
	if h.pluginManager != nil {
		originalReq := req
		hookPayload := map[string]interface{}{
			"order_id":      order.ID,
			"order_no":      order.OrderNo,
			"admin_id":      adminID,
			"status_before": order.Status,
			"reason":        req.Reason,
			"source":        "admin_api",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "order.admin.refund.before",
			Payload: hookPayload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("order.admin.refund.before hook execution failed: admin=%d order=%s err=%v", adminID, order.OrderNo, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Order refund rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if applyErr := applyAdminRefundOrderHookPayload(&req, hookResult.Payload); applyErr != nil {
					log.Printf("order.admin.refund.before payload apply failed, fallback to original request: admin=%d order=%s err=%v", adminID, order.OrderNo, applyErr)
					req = originalReq
				}
			}
		}
	}

	// 只允许已付款后的订单退款（包括草稿状态，草稿表示已付款但用户尚未填写收货信息）
	allowedStatuses := map[models.OrderStatus]bool{
		models.OrderStatusDraft:        true,
		models.OrderStatusPending:      true,
		models.OrderStatusNeedResubmit: true,
		models.OrderStatusShipped:      true,
		models.OrderStatusCompleted:    true,
	}
	if !allowedStatuses[order.Status] {
		respondAdminOrderValidationError(c, orderbiz.RefundStatusInvalid(order.Status))
		return
	}

	// 获取订单的付款方式
	db := database.GetDB()
	var opm models.OrderPaymentMethod
	if err := db.Where("order_id = ?", orderID).First(&opm).Error; err != nil {
		respondAdminOrderValidationError(c, orderbiz.OrderPaymentMethodNotFound())
		return
	}

	var pm models.PaymentMethod
	if err := db.First(&pm, opm.PaymentMethodID).Error; err != nil {
		respondAdminOrderValidationError(c, orderbiz.PaymentMethodNotFound())
		return
	}

	// 调用JS退款API
	refundResult, err := h.jsRuntimeService.ExecuteRefund(&pm, order)
	if err != nil {
		response.InternalError(c, "Refund execution failed")
		return
	}

	if !refundResult.Success {
		msg := "Refund failed"
		if refundResult.Message != "" {
			msg = refundResult.Message
		}
		response.BadRequest(c, msg)
		return
	}

	// 未发货的订单退款时释放预留库存（物理库存 + 虚拟库存 + 优惠码）
	if order.Status == models.OrderStatusDraft || order.Status == models.OrderStatusPending || order.Status == models.OrderStatusNeedResubmit {
		h.orderService.ReleaseOrderReserves(order)
	}

	nextStatus := models.OrderStatusRefunded
	if refundResult.Pending {
		nextStatus = models.OrderStatusRefundPending
	}

	// 更新订单状态
	updates := map[string]interface{}{
		"status": nextStatus,
	}
	if req.Reason != "" {
		remark := order.AdminRemark
		if remark != "" {
			remark += "\n"
		}
		remark += "[Refund] " + req.Reason
		updates["admin_remark"] = remark
	}
	if err := db.Model(order).Updates(updates).Error; err != nil {
		response.InternalError(c, "Failed to update order status")
		return
	}
	if order.UserID != nil {
		if err := h.orderService.SyncUserConsumptionStats(*order.UserID); err != nil {
			logger.LogOrderOperation(db, c, "sync_user_consumption_stats_failed", order.ID, map[string]interface{}{
				"order_no": order.OrderNo,
				"user_id":  *order.UserID,
				"error":    err.Error(),
			})
		}
	}

	// 记录操作日志
	logger.LogOrderOperation(db, c, "refund", order.ID, map[string]interface{}{
		"order_no":       order.OrderNo,
		"reason":         req.Reason,
		"status_after":   nextStatus,
		"refund_pending": refundResult.Pending,
		"refund_message": refundResult.Message,
		"transaction_id": refundResult.TransactionID,
	})

	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"order_id":       order.ID,
			"order_no":       order.OrderNo,
			"admin_id":       adminID,
			"status_before":  beforeStatus,
			"status_after":   nextStatus,
			"reason":         req.Reason,
			"transaction_id": refundResult.TransactionID,
			"refund_pending": refundResult.Pending,
			"message":        refundResult.Message,
			"source":         "admin_api",
			"completed_at":   time.Now().Format(time.RFC3339),
		}
		service.EmitOrderStatusChangedAfterHookAsync(h.pluginManager, hookExecCtx, order, beforeStatus, nextStatus, map[string]interface{}{
			"source":          "admin_api",
			"trigger_action":  "order.admin.refund",
			"admin_id":        adminID,
			"reason":          req.Reason,
			"transaction_id":  refundResult.TransactionID,
			"refund_pending":  refundResult.Pending,
			"payment_message": refundResult.Message,
		})
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, aid uint, orderNo string) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "order.admin.refund.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("order.admin.refund.after hook execution failed: admin=%d order=%s err=%v", aid, orderNo, hookErr)
			}
		}(hookExecCtx, afterPayload, adminID, order.OrderNo)
	}

	response.Success(c, gin.H{
		"order_no":       order.OrderNo,
		"status":         nextStatus,
		"message":        refundResult.Message,
		"transaction_id": refundResult.TransactionID,
		"pending":        refundResult.Pending,
		"data":           refundResult.Data,
	})
}

// ConfirmRefund 手动确认退款完成
func (h *OrderHandler) ConfirmRefund(c *gin.Context) {
	adminID, adminIDOK := middleware.RequireUserID(c)
	if !adminIDOK {
		return
	}
	orderID, ok := parseAdminOrderID(c)
	if !ok {
		return
	}

	var req ConfirmRefundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 允许不传 body
	}

	req.TransactionID = validator.SanitizeInput(req.TransactionID)
	req.Remark = validator.SanitizeText(req.Remark)
	if !validator.ValidateLength(req.TransactionID, 0, 255) {
		respondAdminOrderValidationError(c, orderbiz.RefundTransactionIDTooLong(255))
		return
	}
	if !validator.ValidateLength(req.Remark, 0, 500) {
		respondAdminOrderValidationError(c, orderbiz.AdminRemarkTooLong(500))
		return
	}

	order, err := h.orderService.GetOrderByID(orderID)
	if err != nil {
		response.NotFound(c, "Order not found")
		return
	}
	if order.Status != models.OrderStatusRefundPending {
		respondAdminOrderValidationError(c, orderbiz.RefundFinalizeStatusInvalid(order.Status))
		return
	}

	beforeStatus := order.Status
	hookExecCtx := h.buildOrderHookExecutionContext(c, adminID, uint(orderID))
	if h.pluginManager != nil {
		originalReq := req
		hookPayload := map[string]interface{}{
			"order_id":       order.ID,
			"order_no":       order.OrderNo,
			"admin_id":       adminID,
			"status_before":  order.Status,
			"remark":         req.Remark,
			"transaction_id": req.TransactionID,
			"source":         "admin_api",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "order.admin.refund_finalize.before",
			Payload: hookPayload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("order.admin.refund_finalize.before hook execution failed: admin=%d order=%s err=%v", adminID, order.OrderNo, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Order refund finalization rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if applyErr := applyAdminConfirmRefundHookPayload(&req, hookResult.Payload); applyErr != nil {
					log.Printf("order.admin.refund_finalize.before payload apply failed, fallback to original request: admin=%d order=%s err=%v", adminID, order.OrderNo, applyErr)
					req = originalReq
				}
			}
		}
	}
	req.TransactionID = validator.SanitizeInput(req.TransactionID)
	req.Remark = validator.SanitizeText(req.Remark)
	if !validator.ValidateLength(req.TransactionID, 0, 255) {
		respondAdminOrderValidationError(c, orderbiz.RefundTransactionIDTooLong(255))
		return
	}
	if !validator.ValidateLength(req.Remark, 0, 500) {
		respondAdminOrderValidationError(c, orderbiz.AdminRemarkTooLong(500))
		return
	}

	db := database.GetDB()
	now := time.Now().UTC()
	remarkLines := make([]string, 0, 2)
	if req.TransactionID != "" {
		remarkLines = append(remarkLines, "[Refund Confirmed] transaction_id="+req.TransactionID)
	} else {
		remarkLines = append(remarkLines, "[Refund Confirmed]")
	}
	if req.Remark != "" {
		remarkLines = append(remarkLines, req.Remark)
	}
	nextAdminRemark := order.AdminRemark
	if len(remarkLines) > 0 {
		if nextAdminRemark != "" {
			nextAdminRemark += "\n"
		}
		nextAdminRemark += strings.Join(remarkLines, "\n")
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		updates := map[string]interface{}{
			"status":       models.OrderStatusRefunded,
			"admin_remark": nextAdminRemark,
		}
		if err := tx.Model(order).Updates(updates).Error; err != nil {
			return err
		}

		var opm models.OrderPaymentMethod
		if err := tx.Where("order_id = ?", order.ID).First(&opm).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}

		paymentData := map[string]interface{}{}
		if strings.TrimSpace(opm.PaymentData) != "" {
			if err := json.Unmarshal([]byte(opm.PaymentData), &paymentData); err != nil {
				paymentData["_raw_payment_data"] = opm.PaymentData
			}
		}
		paymentData["refund_confirmed_at"] = now.Format(time.RFC3339)
		paymentData["refund_confirmed_by"] = adminID
		if req.TransactionID != "" {
			paymentData["refund_transaction_id"] = req.TransactionID
		}
		if req.Remark != "" {
			paymentData["refund_confirm_remark"] = req.Remark
		}
		encoded, err := json.Marshal(paymentData)
		if err != nil {
			return err
		}
		return tx.Model(&models.OrderPaymentMethod{}).
			Where("order_id = ?", order.ID).
			Update("payment_data", string(encoded)).Error
	}); err != nil {
		response.InternalError(c, "Failed to confirm refund")
		return
	}

	if order.UserID != nil {
		if err := h.orderService.SyncUserConsumptionStats(*order.UserID); err != nil {
			logger.LogOrderOperation(db, c, "sync_user_consumption_stats_failed", order.ID, map[string]interface{}{
				"order_no": order.OrderNo,
				"user_id":  *order.UserID,
				"error":    err.Error(),
			})
		}
	}

	logger.LogOrderOperation(db, c, "confirm_refund", order.ID, map[string]interface{}{
		"order_no":       order.OrderNo,
		"remark":         req.Remark,
		"transaction_id": req.TransactionID,
		"status_before":  beforeStatus,
		"status_after":   models.OrderStatusRefunded,
	})

	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"order_id":       order.ID,
			"order_no":       order.OrderNo,
			"admin_id":       adminID,
			"status_before":  beforeStatus,
			"status_after":   models.OrderStatusRefunded,
			"remark":         req.Remark,
			"transaction_id": req.TransactionID,
			"source":         "admin_api",
			"completed_at":   now.Format(time.RFC3339),
		}
		service.EmitOrderStatusChangedAfterHookAsync(h.pluginManager, hookExecCtx, order, beforeStatus, models.OrderStatusRefunded, map[string]interface{}{
			"source":         "admin_api",
			"trigger_action": "order.admin.refund_finalize",
			"admin_id":       adminID,
			"remark":         req.Remark,
			"transaction_id": req.TransactionID,
		})
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, aid uint, orderNo string) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "order.admin.refund_finalize.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("order.admin.refund_finalize.after hook execution failed: admin=%d order=%s err=%v", aid, orderNo, hookErr)
			}
		}(hookExecCtx, afterPayload, adminID, order.OrderNo)
	}

	response.Success(c, gin.H{
		"order_no":       order.OrderNo,
		"status":         models.OrderStatusRefunded,
		"transaction_id": req.TransactionID,
		"remark":         req.Remark,
		"message":        "Refund confirmed",
	})
}

// MarkAsPaid 标记订单为已付款
func (h *OrderHandler) MarkAsPaid(c *gin.Context) {
	adminID, adminIDOK := middleware.RequireUserID(c)
	if !adminIDOK {
		return
	}
	orderID, ok := parseAdminOrderID(c)
	if !ok {
		return
	}

	order, err := h.orderService.GetOrderByID(orderID)
	if err != nil {
		response.NotFound(c, "Order not found")
		return
	}
	beforeStatus := order.Status
	options := service.MarkAsPaidOptions{}
	hookExecCtx := h.buildOrderHookExecutionContext(c, adminID, uint(orderID))
	if h.pluginManager != nil {
		originalOptions := options
		hookPayload := map[string]interface{}{
			"order_id":           order.ID,
			"order_no":           order.OrderNo,
			"admin_id":           adminID,
			"status_before":      order.Status,
			"admin_remark":       options.AdminRemark,
			"skip_auto_delivery": options.SkipAutoDelivery,
			"source":             "admin_api",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "order.admin.mark_paid.before",
			Payload: hookPayload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("order.admin.mark_paid.before hook execution failed: admin=%d order=%s err=%v", adminID, order.OrderNo, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Order mark-as-paid rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if applyErr := applyAdminMarkPaidHookPayload(&options, hookResult.Payload); applyErr != nil {
					log.Printf("order.admin.mark_paid.before payload apply failed, fallback to original options: admin=%d order=%s err=%v", adminID, order.OrderNo, applyErr)
					options = originalOptions
				}
			}
		}
	}
	options.AdminRemark = validator.SanitizeText(options.AdminRemark)
	if !validator.ValidateLength(options.AdminRemark, 0, 1000) {
		respondAdminOrderValidationError(c, orderbiz.AdminRemarkTooLong(1000))
		return
	}

	if err := h.orderService.MarkAsPaidWithOptions(orderID, options); err != nil {
		respondAdminOrderServiceError(c, err, "Failed to mark order as paid")
		return
	}

	order, _ = h.orderService.GetOrderByID(orderID)
	if h.pluginManager != nil && order != nil {
		afterPayload := map[string]interface{}{
			"order_id":           order.ID,
			"order_no":           order.OrderNo,
			"admin_id":           adminID,
			"status_before":      beforeStatus,
			"status_after":       order.Status,
			"admin_remark":       options.AdminRemark,
			"skip_auto_delivery": options.SkipAutoDelivery,
			"source":             "admin_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, aid uint, orderNo string) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "order.admin.mark_paid.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("order.admin.mark_paid.after hook execution failed: admin=%d order=%s err=%v", aid, orderNo, hookErr)
			}
		}(hookExecCtx, afterPayload, adminID, order.OrderNo)
	}

	// 记录操作日志
	db := database.GetDB()
	logger.LogOrderOperation(db, c, "mark_paid", order.ID, map[string]interface{}{
		"order_no":           order.OrderNo,
		"admin_remark":       options.AdminRemark,
		"skip_auto_delivery": options.SkipAutoDelivery,
	})

	response.Success(c, gin.H{
		"order_no": order.OrderNo,
		"status":   order.Status,
		"message":  "Order marked as paid",
	})
}

// DeliverVirtualStock 手动发货虚拟商品库存
func (h *OrderHandler) DeliverVirtualStock(c *gin.Context) {
	adminID, adminIDOK := middleware.RequireUserID(c)
	if !adminIDOK {
		return
	}
	orderID, ok := parseAdminOrderID(c)
	if !ok {
		return
	}

	var req struct {
		MarkOnlyShipped  bool `json:"mark_only_shipped"`
		MarkOnlyComplete bool `json:"mark_only_complete"` // backward compatibility
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		// 允许不传 body，默认执行完整发货流程
	}

	markOnlyShipped := req.MarkOnlyShipped || req.MarkOnlyComplete
	order, err := h.orderService.GetOrderByID(orderID)
	if err != nil {
		response.NotFound(c, "Order not found")
		return
	}
	beforeStatus := order.Status
	hookExecCtx := h.buildOrderHookExecutionContext(c, adminID, uint(orderID))
	if h.pluginManager != nil {
		hookPayload := map[string]interface{}{
			"order_id":          order.ID,
			"order_no":          order.OrderNo,
			"admin_id":          adminID,
			"status_before":     order.Status,
			"mark_only_shipped": markOnlyShipped,
			"source":            "admin_api",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "order.admin.deliver_virtual.before",
			Payload: hookPayload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("order.admin.deliver_virtual.before hook execution failed: admin=%d order=%s err=%v", adminID, order.OrderNo, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Virtual delivery rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if raw, exists := hookResult.Payload["mark_only_shipped"]; exists {
					if parsed, present, parseErr := orderValueToOptionalBool(raw); parseErr == nil && present {
						markOnlyShipped = parsed
					}
				}
			}
		}
	}

	if err := h.orderService.DeliverVirtualStock(orderID, adminID, markOnlyShipped); err != nil {
		respondAdminOrderServiceError(c, err, "Failed to deliver virtual stock")
		return
	}

	order, _ = h.orderService.GetOrderByID(orderID)
	if h.pluginManager != nil && order != nil {
		afterPayload := map[string]interface{}{
			"order_id":          order.ID,
			"order_no":          order.OrderNo,
			"admin_id":          adminID,
			"status_before":     beforeStatus,
			"status_after":      order.Status,
			"mark_only_shipped": markOnlyShipped,
			"source":            "admin_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, aid uint, orderNo string) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "order.admin.deliver_virtual.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("order.admin.deliver_virtual.after hook execution failed: admin=%d order=%s err=%v", aid, orderNo, hookErr)
			}
		}(hookExecCtx, afterPayload, adminID, order.OrderNo)
	}

	// 记录操作日志
	db := database.GetDB()
	logger.LogOrderOperation(db, c, "deliver_virtual_stock", order.ID, map[string]interface{}{
		"order_no":          order.OrderNo,
		"delivered_by":      adminID,
		"mark_only_shipped": markOnlyShipped,
		"status":            order.Status,
	})

	message := "Virtual goods shipped"
	if markOnlyShipped {
		message = "Virtual script goods marked as shipped (script skipped)"
	}

	response.Success(c, gin.H{
		"order_no": order.OrderNo,
		"status":   order.Status,
		"message":  message,
	})
}

// DeleteOrder DeleteOrder
func (h *OrderHandler) DeleteOrder(c *gin.Context) {
	adminID, adminIDOK := middleware.RequireUserID(c)
	if !adminIDOK {
		return
	}
	if adminID == 0 {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	orderID, ok := parseAdminOrderID(c)
	if !ok {
		return
	}

	order, err := h.orderService.GetOrderByID(orderID)
	if err != nil || order == nil {
		response.NotFound(c, "Order not found")
		return
	}
	beforeStatus := order.Status
	hookExecCtx := h.buildOrderHookExecutionContext(c, adminID, uint(orderID))
	if h.pluginManager != nil {
		hookPayload := map[string]interface{}{
			"order_id":      order.ID,
			"order_no":      order.OrderNo,
			"admin_id":      adminID,
			"status_before": order.Status,
			"source":        "admin_api",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "order.admin.delete.before",
			Payload: hookPayload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("order.admin.delete.before hook execution failed: admin=%d order=%s err=%v", adminID, order.OrderNo, hookErr)
		} else if hookResult != nil && hookResult.Blocked {
			reason := strings.TrimSpace(hookResult.BlockReason)
			if reason == "" {
				reason = "Order deletion rejected by plugin"
			}
			response.BadRequest(c, reason)
			return
		}
	}

	if err := h.orderService.DeleteOrder(orderID); err != nil {
		respondAdminOrderServiceError(c, err, "Failed to delete order")
		return
	}

	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"order_id":      order.ID,
			"order_no":      order.OrderNo,
			"admin_id":      adminID,
			"status_before": beforeStatus,
			"status_after":  "deleted",
			"source":        "admin_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, aid uint, orderNo string) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "order.admin.delete.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("order.admin.delete.after hook execution failed: admin=%d order=%s err=%v", aid, orderNo, hookErr)
			}
		}(hookExecCtx, afterPayload, adminID, order.OrderNo)
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
	adminID, adminIDOK := middleware.RequireUserID(c)
	if !adminIDOK {
		return
	}
	if adminID == 0 {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	orderID, ok := parseAdminOrderID(c)
	if !ok {
		return
	}

	var req UpdateShippingInfoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondAdminOrderValidationError(c, orderbiz.InvalidRequestParameters())
		return
	}

	// QueryOrder
	order, err := h.orderService.GetOrderByID(orderID)
	if err != nil {
		response.NotFound(c, "Order not found")
		return
	}
	beforeStatus := order.Status
	beforeShipping := map[string]interface{}{
		"receiver_name":     order.ReceiverName,
		"phone_code":        order.PhoneCode,
		"receiver_phone":    order.ReceiverPhone,
		"receiver_email":    order.ReceiverEmail,
		"receiver_country":  order.ReceiverCountry,
		"receiver_province": order.ReceiverProvince,
		"receiver_city":     order.ReceiverCity,
		"receiver_district": order.ReceiverDistrict,
		"receiver_address":  order.ReceiverAddress,
		"receiver_postcode": order.ReceiverPostcode,
	}
	hookExecCtx := h.buildOrderHookExecutionContext(c, adminID, uint(orderID))
	if h.pluginManager != nil {
		originalReq := req
		hookPayload := map[string]interface{}{
			"order_id":          order.ID,
			"order_no":          order.OrderNo,
			"admin_id":          adminID,
			"status_before":     order.Status,
			"receiver_name":     req.ReceiverName,
			"phone_code":        req.PhoneCode,
			"receiver_phone":    req.ReceiverPhone,
			"receiver_email":    req.ReceiverEmail,
			"receiver_country":  req.ReceiverCountry,
			"receiver_province": req.ReceiverProvince,
			"receiver_city":     req.ReceiverCity,
			"receiver_district": req.ReceiverDistrict,
			"receiver_address":  req.ReceiverAddress,
			"receiver_postcode": req.ReceiverPostcode,
			"shipping_before":   beforeShipping,
			"source":            "admin_api",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "order.admin.update_shipping.before",
			Payload: hookPayload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("order.admin.update_shipping.before hook execution failed: admin=%d order=%s err=%v", adminID, order.OrderNo, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Shipping info update rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if applyErr := applyAdminUpdateShippingHookPayload(&req, hookResult.Payload); applyErr != nil {
					log.Printf("order.admin.update_shipping.before payload apply failed, fallback to original request: admin=%d order=%s err=%v", adminID, order.OrderNo, applyErr)
					req = originalReq
				}
			}
		}
	}

	// ============= 输入验证和清理 =============
	if req.ReceiverName != "" {
		req.ReceiverName = validator.SanitizeInput(req.ReceiverName)
		if !validator.ValidateLength(req.ReceiverName, 1, 100) {
			respondAdminOrderValidationError(c, orderbiz.ReceiverNameLengthInvalid(1, 100))
			return
		}
	}

	if req.PhoneCode != "" {
		req.PhoneCode = validator.SanitizeInput(req.PhoneCode)
		if !validator.ValidatePhoneCode(req.PhoneCode) {
			respondAdminOrderValidationError(c, orderbiz.PhoneCodeInvalid())
			return
		}
	}

	if req.ReceiverPhone != "" {
		req.ReceiverPhone = validator.SanitizeInput(req.ReceiverPhone)
		if !validator.ValidateLength(req.ReceiverPhone, 1, 50) || !validator.ValidatePhone(req.ReceiverPhone) {
			respondAdminOrderValidationError(c, orderbiz.ReceiverPhoneInvalid())
			return
		}
	}

	if req.ReceiverEmail != "" {
		req.ReceiverEmail = validator.SanitizeInput(req.ReceiverEmail)
		if !validator.ValidateLength(req.ReceiverEmail, 0, 255) {
			respondAdminOrderValidationError(c, orderbiz.EmailTooLong(255))
			return
		}
	}

	if req.ReceiverCountry != "" {
		req.ReceiverCountry = strings.ToUpper(validator.SanitizeInput(req.ReceiverCountry))
		if !validator.ValidateCountryCode(req.ReceiverCountry) {
			respondAdminOrderValidationError(c, orderbiz.CountryCodeInvalid())
			return
		}
	}

	if req.ReceiverProvince != "" {
		req.ReceiverProvince = validator.SanitizeInput(req.ReceiverProvince)
		if !validator.ValidateLength(req.ReceiverProvince, 0, 50) {
			respondAdminOrderValidationError(c, orderbiz.ProvinceTooLong(50))
			return
		}
	}

	if req.ReceiverCity != "" {
		req.ReceiverCity = validator.SanitizeInput(req.ReceiverCity)
		if !validator.ValidateLength(req.ReceiverCity, 0, 50) {
			respondAdminOrderValidationError(c, orderbiz.CityTooLong(50))
			return
		}
	}

	if req.ReceiverDistrict != "" {
		req.ReceiverDistrict = validator.SanitizeInput(req.ReceiverDistrict)
		if !validator.ValidateLength(req.ReceiverDistrict, 0, 50) {
			respondAdminOrderValidationError(c, orderbiz.DistrictTooLong(50))
			return
		}
	}

	if req.ReceiverAddress != "" {
		req.ReceiverAddress = validator.SanitizeText(req.ReceiverAddress)
		if !validator.ValidateLength(req.ReceiverAddress, 1, 500) {
			respondAdminOrderValidationError(c, orderbiz.AddressLengthInvalid(1, 500))
			return
		}
	}

	if req.ReceiverPostcode != "" {
		req.ReceiverPostcode = validator.SanitizeInput(req.ReceiverPostcode)
		if !validator.ValidateLength(req.ReceiverPostcode, 0, 20) || !validator.ValidatePostcode(req.ReceiverPostcode) {
			respondAdminOrderValidationError(c, orderbiz.PostcodeInvalid())
			return
		}
	}

	// 只允许Update待发货和need重填状态的Order
	if order.Status != models.OrderStatusPending && order.Status != models.OrderStatusNeedResubmit {
		respondAdminOrderValidationError(c, orderbiz.ShippingInfoStatusInvalid(order.Status))
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
		response.InternalError(c, "Failed to update shipping information")
		return
	}

	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"order_id":          order.ID,
			"order_no":          order.OrderNo,
			"admin_id":          adminID,
			"status_before":     beforeStatus,
			"status_after":      order.Status,
			"receiver_name":     order.ReceiverName,
			"phone_code":        order.PhoneCode,
			"receiver_phone":    order.ReceiverPhone,
			"receiver_email":    order.ReceiverEmail,
			"receiver_country":  order.ReceiverCountry,
			"receiver_province": order.ReceiverProvince,
			"receiver_city":     order.ReceiverCity,
			"receiver_district": order.ReceiverDistrict,
			"receiver_address":  order.ReceiverAddress,
			"receiver_postcode": order.ReceiverPostcode,
			"shipping_before":   beforeShipping,
			"source":            "admin_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, aid uint, orderNo string) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "order.admin.update_shipping.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("order.admin.update_shipping.after hook execution failed: admin=%d order=%s err=%v", aid, orderNo, hookErr)
			}
		}(hookExecCtx, afterPayload, adminID, order.OrderNo)
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
	orderID, ok := parseAdminOrderID(c)
	if !ok {
		return
	}

	var req RequestResubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondAdminOrderValidationError(c, orderbiz.InvalidRequestParameters())
		return
	}

	// 清理重填原因（最大500个字符）
	req.Reason = validator.SanitizeText(req.Reason)
	if !validator.ValidateLength(req.Reason, 1, 500) {
		respondAdminOrderValidationError(c, orderbiz.ResubmitReasonLengthInvalid(1, 500))
		return
	}

	// QueryOrder
	order, err := h.orderService.GetOrderByID(orderID)
	if err != nil {
		response.NotFound(c, "Order not found")
		return
	}

	// 只允许待发货状态的Order要求重填
	if order.Status != models.OrderStatusPending {
		respondAdminOrderValidationError(c, orderbiz.ResubmitStatusInvalid(order.Status))
		return
	}

	// 调用service方法要求重填
	newToken, err := h.orderService.RequestResubmit(order.ID, req.Reason)
	if err != nil {
		response.InternalError(c, "Failed to request resubmission")
		return
	}
	updatedOrder, err := h.orderService.GetOrderByID(order.ID)
	if err != nil {
		response.InternalError(c, "Failed to load updated order")
		return
	}

	// 记录操作日志
	db := database.GetDB()
	logger.LogOrderOperation(db, c, "request_resubmit", order.ID, map[string]interface{}{
		"order_no": order.OrderNo,
		"reason":   req.Reason,
	})

	response.Success(c, gin.H{
		"order_no":        updatedOrder.OrderNo,
		"status":          models.OrderStatusNeedResubmit,
		"new_form_token":  newToken,
		"new_form_url":    h.buildShippingFormURL(updatedOrder.FormToken),
		"form_expires_at": updatedOrder.FormExpiresAt,
		"reason":          req.Reason,
		"message":         "User has been asked to resubmit shipping info",
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
			"message":         "No shipped orders to complete",
		})
		return
	}

	// 批量完成订单
	successCount := 0
	failedCount := 0
	var failedOrders []string

	for _, order := range orders {
		completeReq := CompleteOrderRequest{AdminRemark: "Batch complete"}
		hookExecCtx := h.buildOrderHookExecutionContext(c, userID, order.ID)
		beforeStatus := order.Status

		if h.pluginManager != nil {
			originalReq := completeReq
			hookPayload := map[string]interface{}{
				"order_id":      order.ID,
				"order_no":      order.OrderNo,
				"admin_id":      userID,
				"status_before": order.Status,
				"admin_remark":  completeReq.AdminRemark,
				"source":        "admin_batch_api",
				"batch_action":  "complete_all_shipped",
			}
			hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "order.admin.complete.before",
				Payload: hookPayload,
			}, hookExecCtx)
			if hookErr != nil {
				log.Printf("order.admin.complete.before hook execution failed in complete_all_shipped: admin=%d order=%s err=%v", userID, order.OrderNo, hookErr)
			} else if hookResult != nil {
				if hookResult.Blocked {
					failedCount++
					failedOrders = append(failedOrders, order.OrderNo)
					continue
				}
				if hookResult.Payload != nil {
					if applyErr := applyAdminCompleteOrderHookPayload(&completeReq, hookResult.Payload); applyErr != nil {
						log.Printf("order.admin.complete.before payload apply failed in complete_all_shipped, fallback to original request: admin=%d order=%s err=%v", userID, order.OrderNo, applyErr)
						completeReq = originalReq
					}
				}
			}
		}

		err := h.orderService.CompleteOrder(order.ID, userID, "", completeReq.AdminRemark)
		if err != nil {
			failedCount++
			failedOrders = append(failedOrders, order.OrderNo)
			continue
		}
		successCount++

		if h.pluginManager != nil {
			updatedOrder, getErr := h.orderService.GetOrderByID(order.ID)
			if getErr == nil && updatedOrder != nil {
				afterPayload := map[string]interface{}{
					"order_id":      updatedOrder.ID,
					"order_no":      updatedOrder.OrderNo,
					"admin_id":      userID,
					"status_before": beforeStatus,
					"status_after":  updatedOrder.Status,
					"admin_remark":  completeReq.AdminRemark,
					"completed_at":  updatedOrder.CompletedAt,
					"source":        "admin_batch_api",
					"batch_action":  "complete_all_shipped",
				}
				go func(execCtx *service.ExecutionContext, payload map[string]interface{}, aid uint, orderNo string) {
					_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
						Hook:    "order.admin.complete.after",
						Payload: payload,
					}, execCtx)
					if hookErr != nil {
						log.Printf("order.admin.complete.after hook execution failed in complete_all_shipped: admin=%d order=%s err=%v", aid, orderNo, hookErr)
					}
				}(hookExecCtx, afterPayload, userID, updatedOrder.OrderNo)
			}
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
		result["message"] = "Some orders failed to complete"
	} else {
		result["message"] = "All shipped orders completed"
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
	adminID, adminIDOK := middleware.RequireUserID(c)
	if !adminIDOK {
		return
	}

	var req BatchUpdateOrdersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondAdminOrderValidationError(c, orderbiz.InvalidRequestParameters())
		return
	}

	if len(req.OrderIDs) > 100 {
		respondAdminOrderValidationError(c, orderbiz.BatchLimitExceeded(100))
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
		order, getErr := h.orderService.GetOrderByID(orderID)
		if getErr != nil || order == nil {
			failedCount++
			failedOrders = append(failedOrders, strconv.FormatUint(uint64(orderID), 10))
			continue
		}
		hookExecCtx := h.buildOrderHookExecutionContext(c, adminID, orderID)
		beforeStatus := order.Status

		var err error
		switch req.Action {
		case "complete":
			completeReq := CompleteOrderRequest{AdminRemark: "Batch complete"}
			if h.pluginManager != nil {
				originalReq := completeReq
				hookPayload := map[string]interface{}{
					"order_id":      order.ID,
					"order_no":      order.OrderNo,
					"admin_id":      adminID,
					"status_before": order.Status,
					"admin_remark":  completeReq.AdminRemark,
					"source":        "admin_batch_api",
					"batch_action":  "batch_update_orders",
				}
				hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
					Hook:    "order.admin.complete.before",
					Payload: hookPayload,
				}, hookExecCtx)
				if hookErr != nil {
					log.Printf("order.admin.complete.before hook execution failed in batch_update_orders: admin=%d order=%s err=%v", adminID, order.OrderNo, hookErr)
				} else if hookResult != nil {
					if hookResult.Blocked {
						reason := strings.TrimSpace(hookResult.BlockReason)
						if reason == "" {
							reason = "order completion rejected by plugin"
						}
						err = errors.New(reason)
						break
					}
					if hookResult.Payload != nil {
						if applyErr := applyAdminCompleteOrderHookPayload(&completeReq, hookResult.Payload); applyErr != nil {
							log.Printf("order.admin.complete.before payload apply failed in batch_update_orders, fallback to original request: admin=%d order=%s err=%v", adminID, order.OrderNo, applyErr)
							completeReq = originalReq
						}
					}
				}
			}
			if err == nil {
				err = h.orderService.CompleteOrder(orderID, adminID, "", completeReq.AdminRemark)
			}
			if err == nil && h.pluginManager != nil {
				updatedOrder, getErr := h.orderService.GetOrderByID(orderID)
				if getErr == nil && updatedOrder != nil {
					afterPayload := map[string]interface{}{
						"order_id":      updatedOrder.ID,
						"order_no":      updatedOrder.OrderNo,
						"admin_id":      adminID,
						"status_before": beforeStatus,
						"status_after":  updatedOrder.Status,
						"admin_remark":  completeReq.AdminRemark,
						"completed_at":  updatedOrder.CompletedAt,
						"source":        "admin_batch_api",
						"batch_action":  "batch_update_orders",
					}
					go func(execCtx *service.ExecutionContext, payload map[string]interface{}, aid uint, orderNo string) {
						_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
							Hook:    "order.admin.complete.after",
							Payload: payload,
						}, execCtx)
						if hookErr != nil {
							log.Printf("order.admin.complete.after hook execution failed in batch_update_orders: admin=%d order=%s err=%v", aid, orderNo, hookErr)
						}
					}(hookExecCtx, afterPayload, adminID, updatedOrder.OrderNo)
				}
			}
		case "cancel":
			cancelReq := CancelOrderRequest{Reason: "Batch cancel"}
			if h.pluginManager != nil {
				originalReq := cancelReq
				hookPayload := map[string]interface{}{
					"order_id":      order.ID,
					"order_no":      order.OrderNo,
					"admin_id":      adminID,
					"status_before": order.Status,
					"reason":        cancelReq.Reason,
					"source":        "admin_batch_api",
					"batch_action":  "batch_update_orders",
				}
				hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
					Hook:    "order.admin.cancel.before",
					Payload: hookPayload,
				}, hookExecCtx)
				if hookErr != nil {
					log.Printf("order.admin.cancel.before hook execution failed in batch_update_orders: admin=%d order=%s err=%v", adminID, order.OrderNo, hookErr)
				} else if hookResult != nil {
					if hookResult.Blocked {
						reason := strings.TrimSpace(hookResult.BlockReason)
						if reason == "" {
							reason = "order cancellation rejected by plugin"
						}
						err = errors.New(reason)
						break
					}
					if hookResult.Payload != nil {
						if applyErr := applyAdminCancelOrderHookPayload(&cancelReq, hookResult.Payload); applyErr != nil {
							log.Printf("order.admin.cancel.before payload apply failed in batch_update_orders, fallback to original request: admin=%d order=%s err=%v", adminID, order.OrderNo, applyErr)
							cancelReq = originalReq
						}
					}
				}
			}
			if err == nil {
				err = h.orderService.CancelOrder(orderID, cancelReq.Reason)
			}
			if err == nil && h.pluginManager != nil {
				updatedOrder, getErr := h.orderService.GetOrderByID(orderID)
				if getErr == nil && updatedOrder != nil {
					afterPayload := map[string]interface{}{
						"order_id":      updatedOrder.ID,
						"order_no":      updatedOrder.OrderNo,
						"admin_id":      adminID,
						"status_before": beforeStatus,
						"status_after":  updatedOrder.Status,
						"reason":        cancelReq.Reason,
						"source":        "admin_batch_api",
						"batch_action":  "batch_update_orders",
					}
					go func(execCtx *service.ExecutionContext, payload map[string]interface{}, aid uint, orderNo string) {
						_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
							Hook:    "order.admin.cancel.after",
							Payload: payload,
						}, execCtx)
						if hookErr != nil {
							log.Printf("order.admin.cancel.after hook execution failed in batch_update_orders: admin=%d order=%s err=%v", aid, orderNo, hookErr)
						}
					}(hookExecCtx, afterPayload, adminID, updatedOrder.OrderNo)
				}
			}
		case "delete":
			if h.pluginManager != nil {
				hookPayload := map[string]interface{}{
					"order_id":      order.ID,
					"order_no":      order.OrderNo,
					"admin_id":      adminID,
					"status_before": order.Status,
					"source":        "admin_batch_api",
					"batch_action":  "batch_update_orders",
				}
				hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
					Hook:    "order.admin.delete.before",
					Payload: hookPayload,
				}, hookExecCtx)
				if hookErr != nil {
					log.Printf("order.admin.delete.before hook execution failed in batch_update_orders: admin=%d order=%s err=%v", adminID, order.OrderNo, hookErr)
				} else if hookResult != nil && hookResult.Blocked {
					reason := strings.TrimSpace(hookResult.BlockReason)
					if reason == "" {
						reason = "order deletion rejected by plugin"
					}
					err = errors.New(reason)
					break
				}
			}
			if err == nil {
				err = h.orderService.DeleteOrder(orderID)
			}
			if err == nil && h.pluginManager != nil {
				afterPayload := map[string]interface{}{
					"order_id":      order.ID,
					"order_no":      order.OrderNo,
					"admin_id":      adminID,
					"status_before": beforeStatus,
					"status_after":  "deleted",
					"source":        "admin_batch_api",
					"batch_action":  "batch_update_orders",
				}
				go func(execCtx *service.ExecutionContext, payload map[string]interface{}, aid uint, orderNo string) {
					_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
						Hook:    "order.admin.delete.after",
						Payload: payload,
					}, execCtx)
					if hookErr != nil {
						log.Printf("order.admin.delete.after hook execution failed in batch_update_orders: admin=%d order=%s err=%v", aid, orderNo, hookErr)
					}
				}(hookExecCtx, afterPayload, adminID, order.OrderNo)
			}
		}

		if err != nil {
			failedCount++
			failedOrders = append(failedOrders, order.OrderNo)
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
		result["message"] = "Some orders failed"
	} else {
		result["message"] = "Batch operation completed"
	}

	response.Success(c, result)
}

// UpdateOrderPriceRequest 修改订单价格请求
type UpdateOrderPriceRequest struct {
	TotalAmountMinor *int64 `json:"total_amount_minor" binding:"required,min=0"`
}

// UpdateOrderPrice 修改未付款订单价格
func (h *OrderHandler) UpdateOrderPrice(c *gin.Context) {
	adminID, adminIDOK := middleware.RequireUserID(c)
	if !adminIDOK {
		return
	}
	if adminID == 0 {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	orderID, ok := parseAdminOrderID(c)
	if !ok {
		return
	}

	var req UpdateOrderPriceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondAdminOrderValidationError(c, orderbiz.InvalidRequestParameters())
		return
	}
	// 获取订单
	order, err := h.orderService.GetOrderByID(orderID)
	if err != nil {
		response.NotFound(c, "Order not found")
		return
	}

	// 只允许修改待付款状态的订单价格
	if order.Status != models.OrderStatusPendingPayment {
		respondAdminOrderValidationError(c, orderbiz.UpdatePriceStatusInvalid(order.Status))
		return
	}
	hookExecCtx := h.buildOrderHookExecutionContext(c, adminID, uint(orderID))
	if h.pluginManager != nil {
		originalReq := req
		hookPayload := map[string]interface{}{
			"order_id":               order.ID,
			"order_no":               order.OrderNo,
			"admin_id":               adminID,
			"status_before":          order.Status,
			"old_total_amount_minor": order.TotalAmount,
			"new_total_amount_minor": *req.TotalAmountMinor,
			"source":                 "admin_api",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "order.admin.update_price.before",
			Payload: hookPayload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("order.admin.update_price.before hook execution failed: admin=%d order=%s err=%v", adminID, order.OrderNo, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Order price update rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if applyErr := applyAdminUpdateOrderPriceHookPayload(&req, hookResult.Payload); applyErr != nil {
					log.Printf("order.admin.update_price.before payload apply failed, fallback to original request: admin=%d order=%s err=%v", adminID, order.OrderNo, applyErr)
					req = originalReq
				}
			}
		}
	}
	if req.TotalAmountMinor == nil {
		respondAdminOrderValidationError(c, orderbiz.InvalidRequestParameters())
		return
	}
	if *req.TotalAmountMinor < 0 {
		respondAdminOrderValidationError(c, orderbiz.TotalAmountNegative())
		return
	}

	// 保存修改前的价格
	oldAmount := order.TotalAmount

	// 更新订单价格
	order.TotalAmount = *req.TotalAmountMinor

	db := database.GetDB()
	paymentArtifactsReset := false
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(order).Error; err != nil {
			return err
		}

		var opm models.OrderPaymentMethod
		if err := tx.Where("order_id = ?", order.ID).First(&opm).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}

		cacheResult := tx.Model(&models.OrderPaymentMethod{}).
			Where("order_id = ?", order.ID).
			Updates(map[string]interface{}{
				"payment_data":       "",
				"payment_card_cache": "",
				"cache_expires_at":   nil,
			})
		if cacheResult.Error != nil {
			return cacheResult.Error
		}

		storageKeys := []string{
			"order_" + strconv.FormatUint(uint64(order.ID), 10) + "_amount",
			"order_" + strconv.FormatUint(uint64(order.ID), 10) + "_time",
			"order_" + strconv.FormatUint(uint64(order.ID), 10) + "_address",
		}
		deleteResult := tx.
			Where("payment_method_id = ? AND key IN ?", opm.PaymentMethodID, storageKeys).
			Delete(&models.PaymentMethodStorageEntry{})
		if deleteResult.Error != nil {
			return deleteResult.Error
		}

		paymentArtifactsReset = cacheResult.RowsAffected > 0 || deleteResult.RowsAffected > 0
		return nil
	}); err != nil {
		response.InternalError(c, "Failed to update order price")
		return
	}

	// 记录操作日志
	logger.LogOrderOperation(db, c, "update_price", order.ID, map[string]interface{}{
		"order_no":                order.OrderNo,
		"old_total_amount_minor":  oldAmount,
		"new_total_amount_minor":  *req.TotalAmountMinor,
		"payment_artifacts_reset": paymentArtifactsReset,
	})

	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"order_id":               order.ID,
			"order_no":               order.OrderNo,
			"admin_id":               adminID,
			"status_before":          models.OrderStatusPendingPayment,
			"status_after":           order.Status,
			"old_total_amount_minor": oldAmount,
			"new_total_amount_minor": order.TotalAmount,
			"source":                 "admin_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, aid uint, orderNo string) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "order.admin.update_price.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("order.admin.update_price.after hook execution failed: admin=%d order=%s err=%v", aid, orderNo, hookErr)
			}
		}(hookExecCtx, afterPayload, adminID, order.OrderNo)
	}

	response.Success(c, gin.H{
		"order_no":                order.OrderNo,
		"total_amount_minor":      order.TotalAmount,
		"payment_artifacts_reset": paymentArtifactsReset,
		"message":                 "Order price updated",
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
		respondAdminOrderValidationError(c, orderbiz.InvalidRequestParameters())
		return
	}

	req.ExternalUserID = validator.SanitizeInput(req.ExternalUserID)
	if !validator.ValidateLength(req.ExternalUserID, 1, 100) {
		respondAdminOrderValidationError(c, orderbiz.ExternalUserIDLengthInvalid(1, 100))
		return
	}

	req.UserEmail = validator.SanitizeInput(req.UserEmail)
	if !validator.ValidateLength(req.UserEmail, 0, 255) {
		respondAdminOrderValidationError(c, orderbiz.EmailTooLong(255))
		return
	}

	req.UserName = validator.SanitizeInput(req.UserName)
	if !validator.ValidateLength(req.UserName, 0, 100) {
		respondAdminOrderValidationError(c, orderbiz.UsernameTooLong(100))
		return
	}

	req.ExternalOrderID = validator.SanitizeInput(req.ExternalOrderID)
	if !validator.ValidateLength(req.ExternalOrderID, 0, 100) {
		respondAdminOrderValidationError(c, orderbiz.ExternalOrderIDTooLong(100))
		return
	}

	req.Platform = validator.SanitizeInput(req.Platform)
	if !validator.ValidateLength(req.Platform, 0, 100) {
		respondAdminOrderValidationError(c, orderbiz.PlatformNameTooLong(100))
		return
	}

	req.Remark = validator.SanitizeText(req.Remark)
	if !validator.ValidateLength(req.Remark, 0, 1000) {
		respondAdminOrderValidationError(c, orderbiz.OrderRemarkTooLong(1000))
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
		var bizErr *bizerr.Error
		if errors.As(err, &bizErr) {
			response.BizError(c, bizErr.Message, bizErr.Key, bizErr.Params)
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

	response.Success(c, gin.H{
		"order_id":   order.ID,
		"order_no":   order.OrderNo,
		"form_url":   h.buildShippingFormURL(order.FormToken),
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
	TotalAmountMinor *int64                   `json:"total_amount_minor"`
	UserEmail        string                   `json:"user_email"`
}

// CreateOrderForUser 管理员为用户创建订单
func (h *OrderHandler) CreateOrderForUser(c *gin.Context) {
	var req CreateOrderForUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondAdminOrderValidationError(c, orderbiz.InvalidRequestParameters())
		return
	}

	if len(req.Items) == 0 {
		respondAdminOrderValidationError(c, orderbiz.ItemsEmpty())
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
	if req.TotalAmountMinor != nil && *req.TotalAmountMinor < 0 {
		respondAdminOrderValidationError(c, orderbiz.TotalAmountNegative())
		return
	}

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
		TotalAmount:      req.TotalAmountMinor,
		UserEmail:        req.UserEmail,
	})
	if err != nil {
		var bizErr *bizerr.Error
		if errors.As(err, &bizErr) {
			response.BizError(c, bizErr.Message, bizErr.Key, bizErr.Params)
			return
		}
		response.InternalError(c, "Failed to create order")
		return
	}

	db := database.GetDB()
	logDetails := map[string]interface{}{
		"order_no":           order.OrderNo,
		"user_id":            req.UserID,
		"status":             order.Status,
		"total_amount_minor": order.TotalAmount,
	}
	if req.TotalAmountMinor != nil {
		logDetails["amount_override"] = true
		logDetails["override_amount_minor"] = *req.TotalAmountMinor
	}
	logger.LogOrderOperation(db, c, "admin_create_order", order.ID, logDetails)

	response.Success(c, gin.H{
		"order_id":        order.ID,
		"order_no":        order.OrderNo,
		"form_url":        h.buildShippingFormURL(order.FormToken),
		"form_token":      order.FormToken,
		"form_expires_at": order.FormExpiresAt,
		"status":          order.Status,
		"created_at":      order.CreatedAt,
	})
}
