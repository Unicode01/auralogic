package user

import (
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"auralogic/internal/config"
	"auralogic/internal/middleware"
	"auralogic/internal/models"
	"auralogic/internal/pkg/response"
	"auralogic/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// PaymentMethodHandler 用户付款方式处理器
type PaymentMethodHandler struct {
	service        *service.PaymentMethodService
	db             *gorm.DB
	pollingService *service.PaymentPollingService
	pluginManager  *service.PluginManagerService
}

// NewPaymentMethodHandler 创建用户付款方式处理器
func NewPaymentMethodHandler(db *gorm.DB, pollingService *service.PaymentPollingService, pluginManager *service.PluginManagerService, cfg *config.Config) *PaymentMethodHandler {
	return &PaymentMethodHandler{
		service:        service.NewPaymentMethodService(db, cfg),
		db:             db,
		pollingService: pollingService,
		pluginManager:  pluginManager,
	}
}

const maxPaymentWebhookBodyBytes = int64(1024 * 1024)

func (h *PaymentMethodHandler) buildPaymentHookExecutionContext(c *gin.Context, userID uint, orderID uint) *service.ExecutionContext {
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
		"operator_type":   "user",
		"auth_method":     "session",
	}
	sessionID := strings.TrimSpace(c.GetHeader("X-Session-ID"))
	return &service.ExecutionContext{
		UserID:         &userID,
		OrderID:        &orderID,
		SessionID:      sessionID,
		Metadata:       metadata,
		RequestContext: c.Request.Context(),
	}
}

func paymentHookValueToUint(value interface{}) (uint, error) {
	if value == nil {
		return 0, nil
	}

	switch typed := value.(type) {
	case uint:
		return typed, nil
	case uint64:
		return uint(typed), nil
	case int:
		if typed < 0 {
			return 0, strconv.ErrSyntax
		}
		return uint(typed), nil
	case int64:
		if typed < 0 {
			return 0, strconv.ErrSyntax
		}
		return uint(typed), nil
	case float64:
		if typed < 0 || typed != float64(uint(typed)) {
			return 0, strconv.ErrSyntax
		}
		return uint(typed), nil
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return 0, nil
		}
		parsed, err := strconv.ParseUint(trimmed, 10, 32)
		if err != nil {
			return 0, err
		}
		return uint(parsed), nil
	default:
		return 0, strconv.ErrSyntax
	}
}

func readPaymentWebhookBody(body io.ReadCloser, limit int64) ([]byte, error) {
	if body == nil {
		return []byte{}, nil
	}
	defer body.Close()
	limited := io.LimitReader(body, limit+1)
	payload, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(payload)) > limit {
		return nil, fmt.Errorf("payment webhook body exceeds %d bytes", limit)
	}
	return payload, nil
}

func normalizePaymentWebhookHeaders(header http.Header) map[string]string {
	if len(header) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(header))
	for key, values := range header {
		normalizedKey := strings.ToLower(strings.TrimSpace(key))
		if normalizedKey == "" {
			continue
		}
		out[normalizedKey] = strings.TrimSpace(strings.Join(values, ","))
	}
	return out
}

func normalizePaymentWebhookQueryParams(values map[string][]string) map[string]string {
	if len(values) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(values))
	for key, items := range values {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "" {
			continue
		}
		if len(items) == 0 {
			out[normalizedKey] = ""
			continue
		}
		out[normalizedKey] = strings.TrimSpace(items[0])
	}
	return out
}

func (h *PaymentMethodHandler) resolveWebhookOrderID(result *service.PaymentWebhookResult) (uint, error) {
	if result == nil || (!result.Paid && !result.QueuePolling) {
		return 0, nil
	}
	if result.OrderID > 0 {
		return result.OrderID, nil
	}
	if strings.TrimSpace(result.OrderNo) == "" {
		return 0, fmt.Errorf("payment webhook result must provide order_id or order_no")
	}

	var order models.Order
	if err := h.db.Select("id").Where("order_no = ?", strings.TrimSpace(result.OrderNo)).First(&order).Error; err != nil {
		return 0, err
	}
	return order.ID, nil
}

func writePaymentWebhookResponse(c *gin.Context, result *service.PaymentWebhookResult) {
	if result == nil {
		c.String(http.StatusOK, "ok")
		return
	}
	status := result.AckStatus
	if status <= 0 {
		status = http.StatusOK
	}
	for key, value := range result.AckHeaders {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "" {
			continue
		}
		c.Header(normalizedKey, value)
	}
	body := result.AckBody
	if body == "" {
		body = "ok"
	}
	if strings.TrimSpace(c.Writer.Header().Get("Content-Type")) == "" {
		c.Header("Content-Type", "text/plain; charset=utf-8")
	}
	c.String(status, body)
}

// HandleWebhook 处理 PaymentJS 公开回调
func (h *PaymentMethodHandler) HandleWebhook(c *gin.Context) {
	paymentMethodID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid payment method ID")
		return
	}
	hookKey := strings.TrimSpace(c.Param("hook"))
	if hookKey == "" {
		response.BadRequest(c, "Webhook key is required")
		return
	}
	pm, err := h.service.Get(uint(paymentMethodID))
	if err != nil {
		response.NotFound(c, "Payment method not found")
		return
	}
	if !pm.Enabled {
		response.BadRequest(c, "Payment method is disabled")
		return
	}

	rawBody, err := readPaymentWebhookBody(c.Request.Body, maxPaymentWebhookBodyBytes)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	queryParams := normalizePaymentWebhookQueryParams(c.Request.URL.Query())
	headers := normalizePaymentWebhookHeaders(c.Request.Header)
	declaredWebhooks, err := service.ParseDeclaredWebhookManifests(pm.Manifest)
	if err != nil {
		response.InternalServerError(c, "Payment webhook manifest is invalid", err)
		return
	}
	if len(declaredWebhooks) > 0 {
		declaredWebhook, exists := service.FindDeclaredWebhookManifest(declaredWebhooks, hookKey)
		if !exists {
			response.NotFound(c, "Payment webhook not found")
			return
		}
		if !service.DeclaredWebhookAllowsMethod(declaredWebhook, c.Request.Method) {
			response.Error(c, http.StatusMethodNotAllowed, response.CodeParamError, "Webhook method is not allowed")
			return
		}
		if declaredWebhook.AuthMode != "none" {
			secrets, secretErr := service.BuildWebhookSecretsFromConfig(pm.Config)
			if secretErr != nil {
				response.InternalServerError(c, "Payment webhook configuration is invalid", secretErr)
				return
			}
			if authErr := service.AuthenticateDeclaredWebhookRequest(declaredWebhook, queryParams, headers, rawBody, secrets); authErr != nil {
				log.Printf("payment webhook authentication failed: payment_method=%d hook=%s err=%v", pm.ID, hookKey, authErr)
				response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "Webhook authentication failed")
				return
			}
		}
	}
	bodyText := ""
	if utf8.Valid(rawBody) {
		bodyText = string(rawBody)
	}

	result, err := h.service.ExecuteWebhook(uint(paymentMethodID), &service.PaymentWebhookRequest{
		Key:         hookKey,
		Method:      strings.ToUpper(strings.TrimSpace(c.Request.Method)),
		Path:        strings.TrimSpace(c.Request.URL.Path),
		QueryString: strings.TrimSpace(c.Request.URL.RawQuery),
		QueryParams: queryParams,
		Headers:     headers,
		BodyText:    bodyText,
		BodyBase64:  base64.StdEncoding.EncodeToString(rawBody),
		ContentType: strings.TrimSpace(c.ContentType()),
		RemoteAddr:  strings.TrimSpace(c.ClientIP()),
	})
	if err != nil {
		response.HandleError(c, "Payment webhook execution failed", err)
		return
	}

	orderID, err := h.resolveWebhookOrderID(result)
	if err != nil {
		response.HandleError(c, "Failed to resolve webhook order", err)
		return
	}

	if result != nil && result.Paid {
		if h.pollingService == nil {
			response.InternalError(c, "Payment webhook confirmation is unavailable")
			return
		}
		_, err = h.pollingService.ConfirmPaymentResult(orderID, uint(paymentMethodID), &service.PaymentCheckResult{
			Paid:          true,
			TransactionID: result.TransactionID,
			Message:       result.Message,
			Data:          result.Data,
		}, "payment_webhook")
		if err != nil {
			response.HandleError(c, "Failed to confirm payment webhook", err)
			return
		}
	}

	if result != nil && result.QueuePolling {
		if h.pollingService == nil {
			response.InternalError(c, "Payment polling service is unavailable")
			return
		}
		if err := h.pollingService.AddToQueue(orderID, uint(paymentMethodID)); err != nil {
			response.HandleError(c, "Failed to queue payment polling task", err)
			return
		}
	}

	writePaymentWebhookResponse(c, result)
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
	userID := middleware.MustGetUserID(c)
	if userID == 0 {
		return
	}

	var req struct {
		PaymentMethodID uint `json:"payment_method_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	// 获取订单
	var order models.Order
	if err := h.db.Where("order_no = ? AND user_id = ?", orderNo, userID).First(&order).Error; err != nil {
		response.NotFound(c, "Order not found")
		return
	}
	hookExecCtx := h.buildPaymentHookExecutionContext(c, userID, order.ID)
	if h.pluginManager != nil {
		originalReq := req
		hookPayload := map[string]interface{}{
			"order_id":          order.ID,
			"order_no":          order.OrderNo,
			"user_id":           userID,
			"status_before":     order.Status,
			"payment_method_id": req.PaymentMethodID,
			"source":            "user_api",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "payment.method.select.before",
			Payload: hookPayload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("payment.method.select.before hook execution failed: user=%d order=%s err=%v", userID, order.OrderNo, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Payment method selection rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if rawMethodID, exists := hookResult.Payload["payment_method_id"]; exists {
					methodID, convErr := paymentHookValueToUint(rawMethodID)
					if convErr != nil || methodID == 0 {
						log.Printf("payment.method.select.before payload apply failed, fallback to original request: user=%d order=%s err=%v", userID, order.OrderNo, convErr)
						req = originalReq
					} else {
						req.PaymentMethodID = methodID
					}
				}
			}
		}
	}

	// 选择付款方式
	if err := h.service.SelectPaymentMethod(order.ID, req.PaymentMethodID); err != nil {
		response.HandleError(c, "Failed to select payment method", err)
		return
	}

	// 将订单加入付款状态轮询队列
	if h.pollingService != nil {
		if err := h.pollingService.AddToQueue(order.ID, req.PaymentMethodID); err != nil {
			response.HandleError(c, "Failed to queue payment polling task", err)
			return
		}
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

	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"order_id":          order.ID,
			"order_no":          order.OrderNo,
			"user_id":           userID,
			"status_before":     order.Status,
			"payment_method_id": req.PaymentMethodID,
			"queued_polling":    h.pollingService != nil,
			"source":            "user_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, uid uint, orderNumber string) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "payment.method.select.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("payment.method.select.after hook execution failed: user=%d order=%s err=%v", uid, orderNumber, hookErr)
			}
		}(hookExecCtx, afterPayload, userID, order.OrderNo)
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
		// 未选择付款方式，返回可用的付款方式列表
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
		"selected":       true,
		"payment_method": gin.H{"id": pm.ID, "name": pm.Name, "icon": pm.Icon},
		"payment_card":   result,
		"order_payment":  opm,
	})
}
