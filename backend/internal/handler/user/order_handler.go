package user

import (
	"bytes"
	crand "crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"auralogic/internal/config"
	"auralogic/internal/middleware"
	"auralogic/internal/models"
	"auralogic/internal/pkg/bizerr"
	"auralogic/internal/pkg/cache"
	"auralogic/internal/pkg/response"
	"auralogic/internal/pkg/validator"
	"auralogic/internal/service"
)

type OrderHandler struct {
	orderService            *service.OrderService
	bindingService          *service.BindingService
	virtualInventoryService *service.VirtualInventoryService
	cfg                     *config.Config
}

func NewOrderHandler(orderService *service.OrderService, bindingService *service.BindingService, virtualInventoryService *service.VirtualInventoryService, cfg *config.Config) *OrderHandler {
	return &OrderHandler{
		orderService:            orderService,
		bindingService:          bindingService,
		virtualInventoryService: virtualInventoryService,
		cfg:                     cfg,
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
		var bizErr *bizerr.Error
		if errors.As(err, &bizErr) {
			response.BizError(c, bizErr.Message, bizErr.Key, bizErr.Params)
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
		response.HandleError(c, "Failed to complete order", err)
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

// ============================================================
// 账单/Invoice 生成
// ============================================================

// currencySymbol 返回货币符号
func currencySymbol(code string) string {
	symbols := map[string]string{
		"CNY": "¥", "USD": "$", "EUR": "€", "GBP": "£", "JPY": "¥",
		"KRW": "₩", "HKD": "HK$", "TWD": "NT$", "SGD": "S$", "AUD": "A$",
		"CAD": "CA$", "RUB": "₽", "INR": "₹", "THB": "฿", "MYR": "RM",
		"BRL": "R$", "TRY": "₺", "PLN": "zł", "SEK": "kr", "NOK": "kr",
		"DKK": "kr", "CHF": "CHF", "PHP": "₱", "IDR": "Rp", "VND": "₫",
	}
	if s, ok := symbols[strings.ToUpper(code)]; ok {
		return s
	}
	return code + " "
}

// formatAmount 格式化金额
func formatAmount(amount float64, currency string) string {
	sym := currencySymbol(currency)
	// JPY/KRW 等无小数货币
	switch strings.ToUpper(currency) {
	case "JPY", "KRW", "VND", "IDR":
		return fmt.Sprintf("%s%.0f", sym, amount)
	default:
		return fmt.Sprintf("%s%.2f", sym, amount)
	}
}

// invoiceItem 账单行项目
type invoiceItem struct {
	Name     string
	SKU      string
	Quantity int
}

// invoiceData 账单模板数据
type invoiceData struct {
	// 公司信息
	CompanyName    string
	CompanyAddress string
	CompanyPhone   string
	CompanyEmail   string
	CompanyLogo    string
	TaxID          string
	FooterText     string
	// 订单信息
	InvoiceNo     string
	OrderNo       string
	OrderDate     string
	CompletedDate string
	// 客户信息
	CustomerName    string
	CustomerEmail   string
	CustomerPhone   string
	CustomerAddress string
	// 商品
	Items []invoiceItem
	// 金额
	Subtotal       string
	DiscountAmount string
	HasDiscount    bool
	TotalAmount    string
	Currency       string
	// 系统
	AppName        string
	PrintBtnText   string
	CloseBtnText   string
}

// DownloadInvoice 生成并返回订单账单 HTML
func (h *OrderHandler) DownloadInvoice(c *gin.Context) {
	userID := middleware.MustGetUserID(c)
	orderNo := c.Param("order_no")

	if orderNo == "" {
		response.BadRequest(c, "Order number cannot be empty")
		return
	}

	// 检查是否启用账单功能
	invoiceCfg := h.cfg.Order.Invoice
	if !invoiceCfg.Enabled {
		response.BadRequest(c, "Invoice generation is not enabled")
		return
	}

	order, err := h.orderService.GetOrderByNo(orderNo)
	if err != nil {
		response.NotFound(c, "Order not found")
		return
	}

	// 检查订单归属
	if order.UserID == nil || *order.UserID != userID {
		response.Forbidden(c, "No permission to access this order")
		return
	}

	// 只允许已完成的订单生成账单
	if order.Status != models.OrderStatusCompleted {
		response.BadRequest(c, "Invoice is only available for completed orders")
		return
	}

	// 构建模板数据
	data := h.buildInvoiceData(order, &invoiceCfg)

	// 选择模板
	var tmplStr string
	if invoiceCfg.TemplateType == "custom" && invoiceCfg.CustomTemplate != "" {
		tmplStr = invoiceCfg.CustomTemplate
	} else {
		tmplStr = builtinInvoiceTemplate
	}

	tmpl, err := template.New("invoice").Parse(tmplStr)
	if err != nil {
		response.InternalError(c, "Failed to parse invoice template")
		return
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		response.InternalError(c, "Failed to render invoice")
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(200, buf.String())
}

func (h *OrderHandler) buildInvoiceData(order *models.Order, invoiceCfg *config.InvoiceConfig) invoiceData {
	currency := h.cfg.Order.Currency
	if currency == "" {
		currency = "CNY"
	}

	// 构建商品列表
	var items []invoiceItem
	for _, item := range order.Items {
		items = append(items, invoiceItem{
			Name:     item.Name,
			SKU:      item.SKU,
			Quantity: item.Quantity,
		})
	}

	discount := order.DiscountAmount
	total := order.TotalAmount
	subtotal := total + discount

	// 格式化日期
	orderDate := order.CreatedAt.Format("2006-01-02")
	completedDate := ""
	if order.CompletedAt != nil {
		completedDate = order.CompletedAt.Format("2006-01-02")
	}

	// 客户信息：虚拟产品订单无收货人，用账户邮箱兜底
	customerName := order.ReceiverName
	customerEmail := order.UserEmail
	customerPhone := order.ReceiverPhone
	if customerName == "" {
		customerName = customerEmail
	}
	var addrParts []string
	if order.ReceiverAddress != "" {
		addrParts = append(addrParts, order.ReceiverAddress)
	}
	if order.ReceiverDistrict != "" {
		addrParts = append(addrParts, order.ReceiverDistrict)
	}
	if order.ReceiverCity != "" {
		addrParts = append(addrParts, order.ReceiverCity)
	}
	if order.ReceiverProvince != "" {
		addrParts = append(addrParts, order.ReceiverProvince)
	}
	if order.ReceiverPostcode != "" {
		addrParts = append(addrParts, order.ReceiverPostcode)
	}
	if order.ReceiverCountry != "" {
		addrParts = append(addrParts, order.ReceiverCountry)
	}
	customerAddr := strings.Join(addrParts, ", ")

	// 按钮文本 - 根据 Accept-Language 简单判断
	printBtn := "Print / Save as PDF"
	closeBtn := "Close"

	return invoiceData{
		CompanyName:     invoiceCfg.CompanyName,
		CompanyAddress:  invoiceCfg.CompanyAddress,
		CompanyPhone:    invoiceCfg.CompanyPhone,
		CompanyEmail:    invoiceCfg.CompanyEmail,
		CompanyLogo:     invoiceCfg.CompanyLogo,
		TaxID:           invoiceCfg.TaxID,
		FooterText:      invoiceCfg.FooterText,
		InvoiceNo:       "INV-" + order.OrderNo,
		OrderNo:         order.OrderNo,
		OrderDate:       orderDate,
		CompletedDate:   completedDate,
		CustomerName:    customerName,
		CustomerEmail:   customerEmail,
		CustomerPhone:   customerPhone,
		CustomerAddress: customerAddr,
		Items:           items,
		Subtotal:        formatAmount(subtotal, currency),
		DiscountAmount:  formatAmount(discount, currency),
		HasDiscount:     discount > 0,
		TotalAmount:     formatAmount(total, currency),
		Currency:        currency,
		AppName:         h.cfg.App.Name,
		PrintBtnText:    printBtn,
		CloseBtnText:    closeBtn,
	}
}

// GetInvoiceToken 生成一次性账单下载令牌（60秒有效，单次使用）
func (h *OrderHandler) GetInvoiceToken(c *gin.Context) {
	userID := middleware.MustGetUserID(c)
	orderNo := c.Param("order_no")

	if orderNo == "" {
		response.BadRequest(c, "Order number cannot be empty")
		return
	}

	if !h.cfg.Order.Invoice.Enabled {
		response.BadRequest(c, "Invoice generation is not enabled")
		return
	}

	order, err := h.orderService.GetOrderByNo(orderNo)
	if err != nil {
		response.NotFound(c, "Order not found")
		return
	}

	if order.UserID == nil || *order.UserID != userID {
		response.Forbidden(c, "No permission to access this order")
		return
	}

	if order.Status != models.OrderStatusCompleted {
		response.BadRequest(c, "Invoice is only available for completed orders")
		return
	}

	// 检查是否已有未消费的令牌（防止重复生成）
	pendingKey := fmt.Sprintf("invoice_pending:%d:%s", userID, orderNo)
	if existingToken, err := cache.Get(pendingKey); err == nil && existingToken != "" {
		// 验证对应的下载令牌是否还在（未被消费）
		if _, err := cache.Get("invoice_dl:" + existingToken); err == nil {
			response.Success(c, gin.H{
				"token": existingToken,
			})
			return
		}
	}

	// 生成随机令牌
	b := make([]byte, 32)
	crand.Read(b)
	token := fmt.Sprintf("%x", b)

	// 存入 Redis，60秒有效，值为 userID:orderNo
	dlKey := "invoice_dl:" + token
	value := fmt.Sprintf("%d:%s", userID, orderNo)
	if err := cache.Set(dlKey, value, 60*time.Second); err != nil {
		response.InternalError(c, "Failed to generate download token")
		return
	}
	// 记录用户+订单 -> token 的映射，用于去重
	_ = cache.Set(pendingKey, token, 60*time.Second)

	response.Success(c, gin.H{
		"token": token,
	})
}

// ViewInvoiceByToken 通过一次性令牌查看账单（无需JWT认证）
func (h *OrderHandler) ViewInvoiceByToken(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		c.String(401, "Missing download token")
		return
	}

	// 从 Redis 获取并删除令牌（单次使用）
	key := "invoice_dl:" + token
	value, err := cache.Get(key)
	if err != nil {
		c.String(401, "Invalid or expired download token")
		return
	}
	_ = cache.Del(key)

	// 解析 userID:orderNo，并清理 pending key
	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 {
		c.String(500, "Invalid token data")
		return
	}
	userID, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		c.String(500, "Invalid token data")
		return
	}
	orderNo := parts[1]
	_ = cache.Del(fmt.Sprintf("invoice_pending:%s:%s", parts[0], orderNo))

	invoiceCfg := h.cfg.Order.Invoice
	if !invoiceCfg.Enabled {
		c.String(400, "Invoice generation is not enabled")
		return
	}

	order, err := h.orderService.GetOrderByNo(orderNo)
	if err != nil {
		c.String(404, "Order not found")
		return
	}

	if order.UserID == nil || *order.UserID != uint(userID) {
		c.String(403, "No permission")
		return
	}

	if order.Status != models.OrderStatusCompleted {
		c.String(400, "Invoice is only available for completed orders")
		return
	}

	data := h.buildInvoiceData(order, &invoiceCfg)

	var tmplStr string
	if invoiceCfg.TemplateType == "custom" && invoiceCfg.CustomTemplate != "" {
		tmplStr = invoiceCfg.CustomTemplate
	} else {
		tmplStr = builtinInvoiceTemplate
	}

	tmpl, err := template.New("invoice").Parse(tmplStr)
	if err != nil {
		c.String(500, "Failed to parse invoice template")
		return
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		c.String(500, "Failed to render invoice")
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(200, buf.String())
}

// builtinInvoiceTemplate 内置账单 HTML 模板
const builtinInvoiceTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Invoice {{.InvoiceNo}}</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Helvetica Neue',Arial,sans-serif;color:#1a1a1a;background:#f5f5f5;line-height:1.6}
.invoice-wrapper{max-width:800px;margin:20px auto;background:#fff;box-shadow:0 1px 10px rgba(0,0,0,.08);border-radius:8px;overflow:hidden}
.invoice-header{display:flex;justify-content:space-between;align-items:flex-start;padding:40px 40px 30px;border-bottom:2px solid #f0f0f0}
.company-info h1{font-size:22px;font-weight:700;margin-bottom:4px}
.company-info p{font-size:13px;color:#666;margin:1px 0}
.company-logo{max-height:60px;max-width:180px;object-fit:contain}
.invoice-title{text-align:right}
.invoice-title h2{font-size:28px;font-weight:300;color:#333;letter-spacing:2px;text-transform:uppercase}
.invoice-title p{font-size:13px;color:#666;margin:2px 0}
.invoice-body{padding:30px 40px}
.info-row{display:flex;justify-content:space-between;margin-bottom:30px;gap:40px}
.info-block h3{font-size:11px;text-transform:uppercase;letter-spacing:1px;color:#999;margin-bottom:8px;font-weight:600}
.info-block p{font-size:14px;color:#333;margin:2px 0}
table{width:100%;border-collapse:collapse;margin-bottom:30px}
thead th{background:#fafafa;padding:12px 16px;text-align:left;font-size:12px;text-transform:uppercase;letter-spacing:.5px;color:#666;font-weight:600;border-bottom:2px solid #eee}
tbody td{padding:12px 16px;font-size:14px;border-bottom:1px solid #f0f0f0}
.item-name{font-weight:500}
.item-sku{font-size:12px;color:#999;margin-top:2px}
.totals{margin-left:auto;width:280px}
.totals .row{display:flex;justify-content:space-between;padding:8px 0;font-size:14px}
.totals .row.discount{color:#e74c3c}
.totals .row.total{border-top:2px solid #333;padding-top:12px;margin-top:4px;font-size:18px;font-weight:700}
.invoice-footer{padding:20px 40px 30px;border-top:1px solid #f0f0f0;text-align:center}
.invoice-footer p{font-size:12px;color:#999}
.no-print{text-align:center;padding:20px;background:#f5f5f5}
.no-print button{padding:10px 28px;margin:0 8px;border:none;border-radius:6px;font-size:14px;cursor:pointer;transition:all .2s}
.btn-print{background:#1a1a1a;color:#fff}
.btn-print:hover{background:#333}
.btn-close{background:#e5e5e5;color:#333}
.btn-close:hover{background:#d5d5d5}
@media print{
  body{background:#fff}
  .invoice-wrapper{box-shadow:none;margin:0;border-radius:0}
  .no-print{display:none!important}
  .invoice-header{padding:20px 30px 15px}
  .invoice-body{padding:15px 30px}
}
@media(max-width:600px){
  .invoice-header,.invoice-body,.invoice-footer{padding-left:20px;padding-right:20px}
  .info-row{flex-direction:column;gap:20px}
  .totals{width:100%}
}
</style>
</head>
<body>
<div class="invoice-wrapper">
  <div class="invoice-header">
    <div class="company-info">
      {{if .CompanyLogo}}<img src="{{.CompanyLogo}}" alt="Logo" class="company-logo"><br>{{end}}
      <h1>{{if .CompanyName}}{{.CompanyName}}{{else}}{{.AppName}}{{end}}</h1>
      {{if .CompanyAddress}}<p>{{.CompanyAddress}}</p>{{end}}
      {{if .CompanyPhone}}<p>{{.CompanyPhone}}</p>{{end}}
      {{if .CompanyEmail}}<p>{{.CompanyEmail}}</p>{{end}}
      {{if .TaxID}}<p>Tax ID: {{.TaxID}}</p>{{end}}
    </div>
    <div class="invoice-title">
      <h2>Invoice</h2>
      <p><strong>{{.InvoiceNo}}</strong></p>
      <p>Date: {{.OrderDate}}</p>
      {{if .CompletedDate}}<p>Completed: {{.CompletedDate}}</p>{{end}}
    </div>
  </div>

  <div class="invoice-body">
    <div class="info-row">
      <div class="info-block">
        <h3>Bill To</h3>
        {{if .CustomerName}}<p><strong>{{.CustomerName}}</strong></p>{{end}}
        {{if .CustomerEmail}}<p>{{.CustomerEmail}}</p>{{end}}
        {{if .CustomerPhone}}<p>{{.CustomerPhone}}</p>{{end}}
        {{if .CustomerAddress}}<p>{{.CustomerAddress}}</p>{{end}}
      </div>
      <div class="info-block" style="text-align:right">
        <h3>Order Info</h3>
        <p>Order: {{.OrderNo}}</p>
        <p>Currency: {{.Currency}}</p>
      </div>
    </div>

    <table>
      <thead>
        <tr><th>Item</th><th>SKU</th><th style="text-align:center">Qty</th></tr>
      </thead>
      <tbody>
        {{range .Items}}
        <tr>
          <td><div class="item-name">{{.Name}}</div></td>
          <td><span class="item-sku">{{.SKU}}</span></td>
          <td style="text-align:center">{{.Quantity}}</td>
        </tr>
        {{end}}
      </tbody>
    </table>

    <div class="totals">
      <div class="row"><span>Subtotal</span><span>{{.Subtotal}}</span></div>
      {{if .HasDiscount}}<div class="row discount"><span>Discount</span><span>-{{.DiscountAmount}}</span></div>{{end}}
      <div class="row total"><span>Total</span><span>{{.TotalAmount}}</span></div>
    </div>
  </div>

  {{if .FooterText}}
  <div class="invoice-footer">
    <p>{{.FooterText}}</p>
  </div>
  {{end}}
</div>

<div class="no-print">
  <button class="btn-print" onclick="window.print()">{{.PrintBtnText}}</button>
  <button class="btn-close" onclick="window.close()">{{.CloseBtnText}}</button>
</div>
</body>
</html>`
