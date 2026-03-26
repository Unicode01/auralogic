package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image/color"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/models"
	"auralogic/internal/pkg/money"

	"github.com/dop251/goja"
	"github.com/google/uuid"
	qrcode "github.com/skip2/go-qrcode"
	"gorm.io/gorm"
)

// Block SSRF to internal networks from payment scripts by default.
// If a deployment needs access to internal services, it should be implemented
// explicitly with allowlists rather than enabling broad network access here.
func newPaymentHTTPClient() *http.Client {
	transport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}

			// IP literal.
			if ip, err := netip.ParseAddr(host); err == nil {
				if isBlockedIP(ip) {
					return nil, fmt.Errorf("blocked address")
				}
				return (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, network, net.JoinHostPort(host, port))
			}

			// Hostname: resolve and block private/loopback/link-local etc.
			if isBlockedHostname(host) {
				return nil, fmt.Errorf("blocked host")
			}
			ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
			if err != nil {
				return nil, err
			}
			var lastErr error
			for _, ip := range ips {
				if isBlockedIP(ip) {
					continue
				}
				conn, err := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
				if err == nil {
					return conn, nil
				}
				lastErr = err
			}
			if lastErr == nil {
				lastErr = fmt.Errorf("blocked address")
			}
			return nil, lastErr
		},
	}

	return &http.Client{
		Timeout:   30 * time.Second,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("stopped after too many redirects")
			}
			if req.URL == nil {
				return fmt.Errorf("invalid redirect url")
			}
			if err := validateExternalURL(req.URL); err != nil {
				return err
			}
			return nil
		},
	}
}

func validateExternalURL(u *url.URL) error {
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("unsupported scheme")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("invalid host")
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		if isBlockedIP(ip) {
			return fmt.Errorf("blocked address")
		}
		return nil
	}
	if isBlockedHostname(host) {
		return fmt.Errorf("blocked host")
	}
	return nil
}

func isBlockedHostname(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "localhost" || strings.HasSuffix(h, ".localhost") {
		return true
	}
	// Common internal TLDs.
	if strings.HasSuffix(h, ".local") || strings.HasSuffix(h, ".internal") {
		return true
	}
	return false
}

func isBlockedIP(ip netip.Addr) bool {
	// Block anything that isn't routable on the public internet.
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified()
}

// JSRuntimeService JS运行时服务
type JSRuntimeService struct {
	db              *gorm.DB
	moneyMinorUnits bool
	publicBaseURL   string
}

// NewJSRuntimeService 创建JS运行时服务
func NewJSRuntimeService(db *gorm.DB, cfg *config.Config) *JSRuntimeService {
	svc := &JSRuntimeService{
		db:            db,
		publicBaseURL: normalizeJSRuntimePublicBaseURL(cfg),
	}
	svc.moneyMinorUnits = svc.detectMoneyMinorUnits()
	svc.ensurePaymentStorageMigrated()
	return svc
}

func normalizeJSRuntimePublicBaseURL(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	return strings.TrimRight(strings.TrimSpace(cfg.App.URL), "/")
}

func (s *JSRuntimeService) detectMoneyMinorUnits() bool {
	if s == nil || s.db == nil {
		return true
	}
	if !s.db.Migrator().HasTable("system_migrations") {
		return false
	}

	var count int64
	if err := s.db.Table("system_migrations").
		Where("name = ?", "money_minor_units_v1").
		Count(&count).Error; err != nil {
		return true
	}
	return count > 0
}

// JSContext JS执行上下文
type JSContext struct {
	PaymentMethodID uint
	OrderID         uint
	Order           *models.Order
	DB              *gorm.DB
	TestStorage     map[string]string
	Webhook         *PaymentWebhookRequest
}

type PaymentWebhookRequest struct {
	Key         string
	Method      string
	Path        string
	QueryString string
	QueryParams map[string]string
	Headers     map[string]string
	BodyText    string
	BodyBase64  string
	ContentType string
	RemoteAddr  string
}

func (s *JSRuntimeService) newJSContext(pmID uint, order *models.Order) *JSContext {
	ctx := &JSContext{
		PaymentMethodID: pmID,
		OrderID:         0,
		Order:           order,
		DB:              s.db,
	}
	if order != nil {
		ctx.OrderID = order.ID
	}
	// Payment method ID=0 is used by script test path (not persisted).
	// Keep storage in-memory to avoid polluting DB with PM=0 rows.
	if pmID == 0 {
		ctx.TestStorage = make(map[string]string)
	}
	return ctx
}

// PaymentCardResult 付款卡片结果
type PaymentCardResult struct {
	HTML        string                 `json:"html"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Data        map[string]interface{} `json:"data"`
	CacheTTL    int                    `json:"cache_ttl"` // 缓存有效期（秒），0表示不缓存，-1表示永久缓存
}

// ExecutePaymentCard 执行付款方式脚本生成付款卡片
func (s *JSRuntimeService) ExecutePaymentCard(pm *models.PaymentMethod, order *models.Order) (*PaymentCardResult, error) {
	if pm.Script == "" {
		return s.generateDefaultCard(pm, order)
	}

	vm := goja.New()
	ctx := s.newJSContext(pm.ID, order)

	// 设置超时
	timer := time.AfterFunc(5*time.Second, func() {
		vm.Interrupt("execution timeout")
	})
	defer timer.Stop()

	// 注册系统API
	s.registerAPIs(vm, ctx, pm)

	// 执行脚本
	program, err := getOrCompileJSProgram("payment_method", pm.Script)
	if err != nil {
		return nil, fmt.Errorf("script compile error: %w", err)
	}
	_, err = vm.RunProgram(program)
	if err != nil {
		return nil, fmt.Errorf("script execution error: %w", err)
	}

	// 调用 onGeneratePaymentCard
	fn, ok := goja.AssertFunction(vm.Get("onGeneratePaymentCard"))
	if !ok {
		return nil, fmt.Errorf("onGeneratePaymentCard function not found")
	}

	// 准备订单数据
	orderData := s.orderToJS(order)
	configData := s.parseConfig(pm.Config)

	result, err := fn(goja.Undefined(), vm.ToValue(orderData), vm.ToValue(configData))
	if err != nil {
		return nil, fmt.Errorf("onGeneratePaymentCard error: %w", err)
	}

	// 解析结果
	return s.parseCardResult(result)
}

// generateDefaultCard 生成默认付款卡片（当没有脚本时使用）
func (s *JSRuntimeService) generateDefaultCard(pm *models.PaymentMethod, order *models.Order) (*PaymentCardResult, error) {
	html := fmt.Sprintf(`
		<div class="text-sm text-muted-foreground">
			<p>%s</p>
			<p class="mt-2">Amount: <span class="font-bold text-primary">%s %s</span></p>
			<p class="mt-1">Order No: <code class="bg-muted px-1 rounded">%s</code></p>
		</div>
	`, pm.Description, order.Currency, money.MinorToString(order.TotalAmount), order.OrderNo)

	return &PaymentCardResult{
		HTML:        html,
		Title:       pm.Name,
		Description: pm.Description,
	}, nil
}

// registerAPIs 注册系统API到JS虚拟机
func (s *JSRuntimeService) registerAPIs(vm *goja.Runtime, ctx *JSContext, pm *models.PaymentMethod) {
	// 创建 AuraLogic 命名空间
	auralogic := vm.NewObject()
	vm.Set("AuraLogic", auralogic)

	// 存储API（按付款方式隔离）
	storage := vm.NewObject()
	auralogic.Set("storage", storage)
	storage.Set("get", s.createStorageGet(vm, ctx))
	storage.Set("set", s.createStorageSet(vm, ctx))
	storage.Set("delete", s.createStorageDelete(vm, ctx))
	storage.Set("list", s.createStorageList(vm, ctx))
	storage.Set("clear", s.createStorageClear(vm, ctx))

	// 订单API
	order := vm.NewObject()
	auralogic.Set("order", order)
	order.Set("get", s.createOrderGet(vm, ctx))
	order.Set("updatePaymentData", s.createOrderUpdatePaymentData(vm, ctx))
	order.Set("getItems", s.createOrderGetItems(vm, ctx))
	order.Set("getUser", s.createOrderGetUser(vm, ctx))

	// 工具API
	utils := vm.NewObject()
	auralogic.Set("utils", utils)
	utils.Set("formatPrice", s.createFormatPrice(vm, ctx))
	utils.Set("formatDate", s.createFormatDate(vm))
	utils.Set("generateId", s.createGenerateId(vm))
	utils.Set("md5", s.createMD5(vm))
	utils.Set("hmacSHA256", s.createHMACSHA256(vm))
	utils.Set("base64Encode", s.createBase64Encode(vm))
	utils.Set("base64Decode", s.createBase64Decode(vm))
	utils.Set("jsonEncode", s.createJSONEncode(vm))
	utils.Set("jsonDecode", s.createJSONDecode(vm))
	utils.Set("qrcode", s.createQRCode(vm))

	// Webhook API
	webhook := vm.NewObject()
	auralogic.Set("webhook", webhook)
	s.registerWebhookAPI(vm, webhook, ctx)

	// HTTP API
	httpObj := vm.NewObject()
	auralogic.Set("http", httpObj)
	httpObj.Set("get", s.createHTTPGet(vm, ctx, pm.Name))
	httpObj.Set("post", s.createHTTPPost(vm, ctx, pm.Name))
	httpObj.Set("request", s.createHTTPRequest(vm, ctx, pm.Name))

	// 配置API
	config := vm.NewObject()
	auralogic.Set("config", config)
	config.Set("get", s.createConfigGet(vm, pm))

	// 系统信息
	system := vm.NewObject()
	auralogic.Set("system", system)
	system.Set("getTimestamp", s.createGetTimestamp(vm))
	system.Set("getPaymentMethodInfo", s.createGetPaymentMethodInfo(vm, pm))
	system.Set("getWebhookUrl", s.createGetWebhookURL(vm, pm))
}

func (s *JSRuntimeService) registerWebhookAPI(vm *goja.Runtime, webhook *goja.Object, ctx *JSContext) {
	req := &PaymentWebhookRequest{
		QueryParams: map[string]string{},
		Headers:     map[string]string{},
	}
	if ctx != nil && ctx.Webhook != nil {
		req = ctx.Webhook
		if req.QueryParams == nil {
			req.QueryParams = map[string]string{}
		}
		if req.Headers == nil {
			req.Headers = map[string]string{}
		}
	}

	_ = webhook.Set("enabled", ctx != nil && ctx.Webhook != nil)
	_ = webhook.Set("key", req.Key)
	_ = webhook.Set("method", req.Method)
	_ = webhook.Set("path", req.Path)
	_ = webhook.Set("query_string", req.QueryString)
	_ = webhook.Set("queryString", req.QueryString)
	_ = webhook.Set("query_params", req.QueryParams)
	_ = webhook.Set("queryParams", req.QueryParams)
	_ = webhook.Set("headers", req.Headers)
	_ = webhook.Set("content_type", req.ContentType)
	_ = webhook.Set("contentType", req.ContentType)
	_ = webhook.Set("remote_addr", req.RemoteAddr)
	_ = webhook.Set("remoteAddr", req.RemoteAddr)
	_ = webhook.Set("body_text", req.BodyText)
	_ = webhook.Set("bodyText", req.BodyText)
	_ = webhook.Set("body_base64", req.BodyBase64)
	_ = webhook.Set("bodyBase64", req.BodyBase64)
	_ = webhook.Set("header", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Undefined()
		}
		name := strings.ToLower(strings.TrimSpace(call.Arguments[0].String()))
		if name == "" {
			return goja.Undefined()
		}
		value, ok := req.Headers[name]
		if !ok {
			return goja.Undefined()
		}
		return vm.ToValue(value)
	})
	_ = webhook.Set("query", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Undefined()
		}
		name := strings.TrimSpace(call.Arguments[0].String())
		if name == "" {
			return goja.Undefined()
		}
		value, ok := req.QueryParams[name]
		if !ok {
			return goja.Undefined()
		}
		return vm.ToValue(value)
	})
	_ = webhook.Set("text", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(req.BodyText)
	})
	_ = webhook.Set("json", func(call goja.FunctionCall) goja.Value {
		if strings.TrimSpace(req.BodyText) == "" {
			return goja.Undefined()
		}
		var decoded interface{}
		if err := json.Unmarshal([]byte(req.BodyText), &decoded); err != nil {
			panic(vm.ToValue(fmt.Sprintf("invalid webhook json: %v", err)))
		}
		return vm.ToValue(decoded)
	})
}

// Storage APIs - 按付款方式隔离的持久化 KV 存储
func (s *JSRuntimeService) createStorageGet(vm *goja.Runtime, ctx *JSContext) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Undefined()
		}
		key := call.Arguments[0].String()
		if ctx != nil && ctx.PaymentMethodID == 0 {
			if ctx.TestStorage == nil {
				return goja.Undefined()
			}
			if val, ok := ctx.TestStorage[key]; ok {
				return vm.ToValue(val)
			}
			return goja.Undefined()
		}

		lock := getPaymentStorageLock(ctx.PaymentMethodID)
		lock.RLock()
		defer lock.RUnlock()

		value, exists, err := s.storageGetValue(ctx.PaymentMethodID, key)
		if err != nil {
			return goja.Undefined()
		}
		if exists {
			return vm.ToValue(value)
		}
		return goja.Undefined()
	}
}

func (s *JSRuntimeService) createStorageSet(vm *goja.Runtime, ctx *JSContext) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return vm.ToValue(false)
		}
		key := call.Arguments[0].String()
		value := call.Arguments[1].String()
		if ctx != nil && ctx.PaymentMethodID == 0 {
			if ctx.TestStorage == nil {
				ctx.TestStorage = make(map[string]string)
			}
			ctx.TestStorage[key] = value
			return vm.ToValue(true)
		}

		lock := getPaymentStorageLock(ctx.PaymentMethodID)
		lock.Lock()
		defer lock.Unlock()

		if err := s.storageSetValue(ctx.PaymentMethodID, key, value); err != nil {
			return vm.ToValue(false)
		}
		return vm.ToValue(true)
	}
}

func (s *JSRuntimeService) createStorageDelete(vm *goja.Runtime, ctx *JSContext) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(false)
		}
		key := call.Arguments[0].String()
		if ctx != nil && ctx.PaymentMethodID == 0 {
			if ctx.TestStorage == nil {
				return vm.ToValue(true)
			}
			delete(ctx.TestStorage, key)
			return vm.ToValue(true)
		}

		lock := getPaymentStorageLock(ctx.PaymentMethodID)
		lock.Lock()
		defer lock.Unlock()

		if err := s.storageDeleteKey(ctx.PaymentMethodID, key); err != nil {
			return vm.ToValue(false)
		}
		return vm.ToValue(true)
	}
}

// createStorageList 列出当前付款方式的所有存储键
func (s *JSRuntimeService) createStorageList(vm *goja.Runtime, ctx *JSContext) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if ctx != nil && ctx.PaymentMethodID == 0 {
			keys := make([]string, 0, len(ctx.TestStorage))
			for k := range ctx.TestStorage {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			return vm.ToValue(keys)
		}

		lock := getPaymentStorageLock(ctx.PaymentMethodID)
		lock.RLock()
		defer lock.RUnlock()

		keys, err := s.storageListKeys(ctx.PaymentMethodID)
		if err != nil {
			return vm.ToValue([]string{})
		}
		return vm.ToValue(keys)
	}
}

// createStorageClear 清除当前付款方式的所有存储
func (s *JSRuntimeService) createStorageClear(vm *goja.Runtime, ctx *JSContext) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if ctx != nil && ctx.PaymentMethodID == 0 {
			ctx.TestStorage = make(map[string]string)
			return vm.ToValue(true)
		}

		lock := getPaymentStorageLock(ctx.PaymentMethodID)
		lock.Lock()
		defer lock.Unlock()

		if err := s.storageClearAll(ctx.PaymentMethodID); err != nil {
			return vm.ToValue(false)
		}
		return vm.ToValue(true)
	}
}

// Order APIs
func (s *JSRuntimeService) createOrderGet(vm *goja.Runtime, ctx *JSContext) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if ctx.Order == nil {
			return goja.Undefined()
		}
		return vm.ToValue(s.orderToJS(ctx.Order))
	}
}

func (s *JSRuntimeService) createOrderUpdatePaymentData(vm *goja.Runtime, ctx *JSContext) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 || ctx.OrderID == 0 {
			return vm.ToValue(false)
		}

		data := call.Arguments[0].Export()
		jsonData, err := json.Marshal(data)
		if err != nil {
			return vm.ToValue(false)
		}

		result := s.db.Model(&models.OrderPaymentMethod{}).
			Where("order_id = ?", ctx.OrderID).
			Update("payment_data", string(jsonData))

		return vm.ToValue(result.Error == nil)
	}
}

// Utils APIs
func (s *JSRuntimeService) createFormatPrice(vm *goja.Runtime, ctx *JSContext) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue("")
		}
		amountArg := call.Arguments[0].Export()
		orderTotalMinor, orderDiscountMinor, hasOrderAmount := s.contextOrderAmountsMinor(ctx)
		currency := "CNY"
		if len(call.Arguments) > 1 {
			currency = call.Arguments[1].String()
		}
		var amountMinor int64
		switch v := amountArg.(type) {
		case int64:
			amountMinor = resolveScriptIntegerAmount(v, hasOrderAmount, orderTotalMinor, orderDiscountMinor)
		case int32:
			amountMinor = resolveScriptIntegerAmount(int64(v), hasOrderAmount, orderTotalMinor, orderDiscountMinor)
		case int:
			amountMinor = resolveScriptIntegerAmount(int64(v), hasOrderAmount, orderTotalMinor, orderDiscountMinor)
		case uint64:
			amountMinor = resolveScriptIntegerAmount(int64(v), hasOrderAmount, orderTotalMinor, orderDiscountMinor)
		case uint32:
			amountMinor = resolveScriptIntegerAmount(int64(v), hasOrderAmount, orderTotalMinor, orderDiscountMinor)
		case uint:
			amountMinor = resolveScriptIntegerAmount(int64(v), hasOrderAmount, orderTotalMinor, orderDiscountMinor)
		case float64:
			// Backward compatibility: legacy scripts usually pass major-unit amounts
			// through order.total_amount. New scripts should pass int minor units.
			if matchedMinor, matched := resolveScriptFloatAmount(v, hasOrderAmount, orderTotalMinor, orderDiscountMinor); matched {
				amountMinor = matchedMinor
			} else {
				amountMinor = int64(math.Round(v * float64(money.CurrencyScale)))
			}
		case float32:
			floatVal := float64(v)
			if matchedMinor, matched := resolveScriptFloatAmount(floatVal, hasOrderAmount, orderTotalMinor, orderDiscountMinor); matched {
				amountMinor = matchedMinor
			} else {
				amountMinor = int64(math.Round(floatVal * float64(money.CurrencyScale)))
			}
		case string:
			amountMinor = parseScriptAmountToMinor(v, hasOrderAmount, orderTotalMinor, orderDiscountMinor)
		default:
			amountMinor = resolveScriptIntegerAmount(call.Arguments[0].ToInteger(), hasOrderAmount, orderTotalMinor, orderDiscountMinor)
		}

		symbols := map[string]string{
			"CNY": "¥", "USD": "$", "EUR": "€", "JPY": "¥", "GBP": "£",
		}
		symbol := symbols[currency]
		if symbol == "" {
			symbol = currency + " "
		}
		return vm.ToValue(symbol + money.MinorToString(amountMinor))
	}
}

func (s *JSRuntimeService) contextOrderAmountsMinor(ctx *JSContext) (int64, int64, bool) {
	if ctx == nil || ctx.Order == nil {
		return 0, 0, false
	}

	totalMinor := ctx.Order.TotalAmount
	discountMinor := ctx.Order.DiscountAmount
	if !s.moneyMinorUnits {
		totalMinor = ctx.Order.TotalAmount * money.CurrencyScale
		discountMinor = ctx.Order.DiscountAmount * money.CurrencyScale
	}
	return totalMinor, discountMinor, true
}

func resolveScriptFloatAmount(raw float64, hasOrderAmount bool, orderTotalMinor, orderDiscountMinor int64) (int64, bool) {
	if !hasOrderAmount {
		return 0, false
	}
	if matchedMinor, matched := matchFloatToOrderAmount(raw, orderTotalMinor); matched {
		return matchedMinor, true
	}
	if matchedMinor, matched := matchFloatToOrderAmount(raw, orderDiscountMinor); matched {
		return matchedMinor, true
	}
	return 0, false
}

func matchFloatToOrderAmount(raw float64, orderMinor int64) (int64, bool) {
	const epsilon = 1e-9

	// Explicit minor-unit match (e.g. formatPrice(order.total_amount_minor)).
	if math.Abs(raw-float64(orderMinor)) < epsilon {
		return orderMinor, true
	}

	// Legacy major-unit match (e.g. formatPrice(order.total_amount)).
	orderMajor := float64(orderMinor) / float64(money.CurrencyScale)
	if math.Abs(raw-orderMajor) < epsilon {
		return int64(math.Round(raw * float64(money.CurrencyScale))), true
	}
	return 0, false
}

func resolveScriptIntegerAmount(raw int64, hasOrderAmount bool, orderTotalMinor, orderDiscountMinor int64) int64 {
	if !hasOrderAmount {
		return raw
	}

	// Prefer explicit minor-unit fields when matched.
	if raw == orderTotalMinor || raw == orderDiscountMinor {
		return raw
	}

	// Backward compatibility for legacy major-unit fields (whole numbers).
	if isIntegerMajorAlias(raw, orderTotalMinor) || isIntegerMajorAlias(raw, orderDiscountMinor) {
		return raw * money.CurrencyScale
	}

	// Default to minor units for int inputs on the new API contract.
	return raw
}

func isIntegerMajorAlias(raw, orderMinor int64) bool {
	if orderMinor%money.CurrencyScale != 0 {
		return false
	}
	return raw == orderMinor/money.CurrencyScale
}

func parseScriptAmountToMinor(raw string, hasOrderAmount bool, orderTotalMinor, orderDiscountMinor int64) int64 {
	v := strings.TrimSpace(raw)
	if v == "" {
		return 0
	}

	if strings.Contains(v, ".") {
		major, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0
		}
		return int64(math.Round(major * float64(money.CurrencyScale)))
	}

	minor, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0
	}
	return resolveScriptIntegerAmount(minor, hasOrderAmount, orderTotalMinor, orderDiscountMinor)
}

// Config API
func (s *JSRuntimeService) createConfigGet(vm *goja.Runtime, pm *models.PaymentMethod) func(call goja.FunctionCall) goja.Value {
	config := s.parseConfig(pm.Config)
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(config)
		}
		key := call.Arguments[0].String()
		if val, ok := config[key]; ok {
			return vm.ToValue(val)
		}
		if len(call.Arguments) > 1 {
			return call.Arguments[1]
		}
		return goja.Undefined()
	}
}

// HTTP APIs - 支持外部网络请求
func (s *JSRuntimeService) createHTTPGet(vm *goja.Runtime, ctx *JSContext, pmName string) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(map[string]interface{}{
				"error":  "URL is required",
				"status": 0,
			})
		}

		url := call.Arguments[0].String()

		// 验证URL格式
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			return vm.ToValue(map[string]interface{}{
				"error":  "URL must start with http:// or https://",
				"status": 0,
			})
		}

		// 解析可选的headers参数
		headers := make(map[string]string)
		if len(call.Arguments) > 1 {
			if headersObj := call.Arguments[1].Export(); headersObj != nil {
				if h, ok := headersObj.(map[string]interface{}); ok {
					for k, v := range h {
						if str, ok := v.(string); ok {
							headers[k] = str
						}
					}
				}
			}
		}

		return s.doHTTPRequest(vm, "GET", url, nil, headers, pmName)
	}
}

func (s *JSRuntimeService) createHTTPPost(vm *goja.Runtime, ctx *JSContext, pmName string) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(map[string]interface{}{
				"error":  "URL is required",
				"status": 0,
			})
		}

		url := call.Arguments[0].String()

		// 验证URL格式
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			return vm.ToValue(map[string]interface{}{
				"error":  "URL must start with http:// or https://",
				"status": 0,
			})
		}

		// 解析body参数
		var body interface{}
		if len(call.Arguments) > 1 {
			body = call.Arguments[1].Export()
		}

		// 解析可选的headers参数
		headers := make(map[string]string)
		if len(call.Arguments) > 2 {
			if headersObj := call.Arguments[2].Export(); headersObj != nil {
				if h, ok := headersObj.(map[string]interface{}); ok {
					for k, v := range h {
						if str, ok := v.(string); ok {
							headers[k] = str
						}
					}
				}
			}
		}

		return s.doHTTPRequest(vm, "POST", url, body, headers, pmName)
	}
}

// createHTTPRequest 创建通用HTTP请求方法
func (s *JSRuntimeService) createHTTPRequest(vm *goja.Runtime, ctx *JSContext, pmName string) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(map[string]interface{}{
				"error":  "Options object is required",
				"status": 0,
			})
		}

		opts := call.Arguments[0].Export()
		optsMap, ok := opts.(map[string]interface{})
		if !ok {
			return vm.ToValue(map[string]interface{}{
				"error":  "Options must be an object",
				"status": 0,
			})
		}

		// 解析请求参数
		url, _ := optsMap["url"].(string)
		method, _ := optsMap["method"].(string)
		if method == "" {
			method = "GET"
		}
		method = strings.ToUpper(method)

		// 验证URL格式
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			return vm.ToValue(map[string]interface{}{
				"error":  "URL must start with http:// or https://",
				"status": 0,
			})
		}

		// 解析headers
		headers := make(map[string]string)
		if h, ok := optsMap["headers"].(map[string]interface{}); ok {
			for k, v := range h {
				if str, ok := v.(string); ok {
					headers[k] = str
				}
			}
		}

		// 解析body
		body := optsMap["body"]

		return s.doHTTPRequest(vm, method, url, body, headers, pmName)
	}
}

// doHTTPRequest 执行HTTP请求
func (s *JSRuntimeService) doHTTPRequest(vm *goja.Runtime, method, urlStr string, body interface{}, headers map[string]string, pmName string) goja.Value {
	start := time.Now()

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return vm.ToValue(map[string]interface{}{
			"error":  fmt.Sprintf("Invalid URL: %v", err),
			"status": 0,
		})
	}
	if err := validateExternalURL(parsedURL); err != nil {
		return vm.ToValue(map[string]interface{}{
			"error":  "URL is not allowed",
			"status": 0,
		})
	}

	client := getPaymentHTTPClient()

	// 准备请求体
	var reqBody io.Reader
	contentType := ""
	if body != nil {
		switch v := body.(type) {
		case string:
			reqBody = strings.NewReader(v)
			// 尝试检测是否为JSON
			if strings.HasPrefix(strings.TrimSpace(v), "{") || strings.HasPrefix(strings.TrimSpace(v), "[") {
				contentType = "application/json"
			} else {
				contentType = "text/plain"
			}
		case map[string]interface{}, []interface{}:
			jsonBytes, err := json.Marshal(v)
			if err != nil {
				return vm.ToValue(map[string]interface{}{
					"error":  fmt.Sprintf("Failed to encode body: %v", err),
					"status": 0,
				})
			}
			reqBody = bytes.NewReader(jsonBytes)
			contentType = "application/json"
		default:
			jsonBytes, err := json.Marshal(v)
			if err != nil {
				return vm.ToValue(map[string]interface{}{
					"error":  fmt.Sprintf("Failed to encode body: %v", err),
					"status": 0,
				})
			}
			reqBody = bytes.NewReader(jsonBytes)
			contentType = "application/json"
		}
	}

	// 创建请求
	req, err := http.NewRequest(method, parsedURL.String(), reqBody)
	if err != nil {
		return vm.ToValue(map[string]interface{}{
			"error":  fmt.Sprintf("Failed to create request: %v", err),
			"status": 0,
		})
	}

	// 设置默认User-Agent
	req.Header.Set("User-Agent", "AuraLogic-PaymentScript/1.0")

	// 设置Content-Type
	if contentType != "" && headers["Content-Type"] == "" {
		req.Header.Set("Content-Type", contentType)
	}

	// 设置自定义headers
	for k, v := range headers {
		// Prevent scripts from trying to smuggle proxy hops / override host.
		if strings.EqualFold(k, "Host") || strings.EqualFold(k, "Proxy-Authorization") {
			continue
		}
		req.Header.Set(k, v)
	}

	// 执行请求
	resp, err := client.Do(req)
	if err != nil {
		return vm.ToValue(map[string]interface{}{
			"error":  fmt.Sprintf("Request failed: %v", err),
			"status": 0,
		})
	}
	defer resp.Body.Close()

	// 限制响应体大小 (最大 10MB)
	limitedReader := io.LimitReader(resp.Body, 10*1024*1024)
	respBody, err := io.ReadAll(limitedReader)
	if err != nil {
		return vm.ToValue(map[string]interface{}{
			"error":  fmt.Sprintf("Failed to read response: %v", err),
			"status": resp.StatusCode,
		})
	}

	// 解析响应headers
	respHeaders := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			respHeaders[k] = v[0]
		}
	}

	// 尝试解析JSON响应
	var jsonData interface{}
	respContentType := resp.Header.Get("Content-Type")
	if strings.Contains(respContentType, "application/json") {
		json.Unmarshal(respBody, &jsonData)
	}

	result := map[string]interface{}{
		"status":     resp.StatusCode,
		"statusText": resp.Status,
		"headers":    respHeaders,
		"body":       string(respBody),
	}

	// 如果是JSON响应，添加解析后的数据
	if jsonData != nil {
		result["data"] = jsonData
	}

	// 输出请求日志
	latency := time.Since(start)
	log.Printf("[%s] [%s] %s - %d - %v", pmName, method, parsedURL.String(), resp.StatusCode, latency)

	return vm.ToValue(result)
}

// Helper functions
func (s *JSRuntimeService) orderToJS(order *models.Order) map[string]interface{} {
	totalAmountMinor := order.TotalAmount
	discountAmountMinor := order.DiscountAmount
	if !s.moneyMinorUnits {
		totalAmountMinor = order.TotalAmount * money.CurrencyScale
		discountAmountMinor = order.DiscountAmount * money.CurrencyScale
	}

	return map[string]interface{}{
		"id":                    order.ID,
		"order_no":              order.OrderNo,
		"status":                order.Status,
		"total_amount_minor":    totalAmountMinor,
		"discount_amount_minor": discountAmountMinor,
		// Legacy aliases for old built-in/custom scripts that still read major-unit fields.
		"total_amount":    float64(totalAmountMinor) / float64(money.CurrencyScale),
		"discount_amount": float64(discountAmountMinor) / float64(money.CurrencyScale),
		"currency":        order.Currency,
		"created_at":      order.CreatedAt.Format(time.RFC3339),
	}
}

func (s *JSRuntimeService) parseConfig(configJSON string) map[string]interface{} {
	config := make(map[string]interface{})
	if configJSON != "" {
		json.Unmarshal([]byte(configJSON), &config)
	}
	return config
}

func (s *JSRuntimeService) parseCardResult(result goja.Value) (*PaymentCardResult, error) {
	if result == nil || goja.IsUndefined(result) || goja.IsNull(result) {
		return nil, fmt.Errorf("empty result from onGeneratePaymentCard")
	}

	exported := result.Export()
	switch v := exported.(type) {
	case string:
		return &PaymentCardResult{HTML: v}, nil
	case map[string]interface{}:
		cardResult := &PaymentCardResult{}
		if html, ok := v["html"].(string); ok {
			cardResult.HTML = html
		}
		if title, ok := v["title"].(string); ok {
			cardResult.Title = title
		}
		if desc, ok := v["description"].(string); ok {
			cardResult.Description = desc
		}
		if data, ok := v["data"].(map[string]interface{}); ok {
			cardResult.Data = data
		}
		// 解析 cache_ttl 字段
		if cacheTTL, ok := v["cache_ttl"]; ok {
			switch ttl := cacheTTL.(type) {
			case int64:
				cardResult.CacheTTL = int(ttl)
			case float64:
				cardResult.CacheTTL = int(ttl)
			case int:
				cardResult.CacheTTL = ttl
			}
		}
		return cardResult, nil
	default:
		return &PaymentCardResult{HTML: fmt.Sprintf("%v", exported)}, nil
	}
}

func getConfigString(config map[string]interface{}, key, defaultVal string) string {
	if val, ok := config[key].(string); ok {
		return val
	}
	return defaultVal
}

// CheckPaymentStatus 检查付款状态
func (s *JSRuntimeService) CheckPaymentStatus(pm *models.PaymentMethod, order *models.Order) (*PaymentCheckResult, error) {
	// 没有脚本时返回需要人工确认
	if pm.Script == "" {
		return &PaymentCheckResult{Paid: false, Message: "Payment method requires manual confirmation"}, nil
	}

	vm := goja.New()
	ctx := s.newJSContext(pm.ID, order)

	// 设置超时
	timer := time.AfterFunc(10*time.Second, func() {
		vm.Interrupt("execution timeout")
	})
	defer timer.Stop()

	// 注册系统API
	s.registerAPIs(vm, ctx, pm)

	// 执行脚本
	program, err := getOrCompileJSProgram("payment_method", pm.Script)
	if err != nil {
		return nil, fmt.Errorf("script compile error: %w", err)
	}
	_, err = vm.RunProgram(program)
	if err != nil {
		return nil, fmt.Errorf("script execution error: %w", err)
	}

	// 调用 onCheckPaymentStatus
	fn, ok := goja.AssertFunction(vm.Get("onCheckPaymentStatus"))
	if !ok {
		// 如果没有定义此方法，返回未付款
		return &PaymentCheckResult{Paid: false, Message: "Payment method has not implemented status check"}, nil
	}

	// 准备订单数据
	orderData := s.orderToJS(order)
	configData := s.parseConfig(pm.Config)

	result, err := fn(goja.Undefined(), vm.ToValue(orderData), vm.ToValue(configData))
	if err != nil {
		return nil, fmt.Errorf("onCheckPaymentStatus error: %w", err)
	}

	// 解析结果
	return s.parseCheckResult(result)
}

func (s *JSRuntimeService) ExecuteWebhook(pm *models.PaymentMethod, req *PaymentWebhookRequest) (*PaymentWebhookResult, error) {
	if pm == nil {
		return nil, fmt.Errorf("payment method is required")
	}
	if pm.Script == "" {
		return nil, fmt.Errorf("payment method has no script configured")
	}

	vm := goja.New()
	ctx := s.newJSContext(pm.ID, nil)
	ctx.Webhook = clonePaymentWebhookRequest(req)

	timer := time.AfterFunc(10*time.Second, func() {
		vm.Interrupt("execution timeout")
	})
	defer timer.Stop()

	s.registerAPIs(vm, ctx, pm)

	program, err := getOrCompileJSProgram("payment_method", pm.Script)
	if err != nil {
		return nil, fmt.Errorf("script compile error: %w", err)
	}
	_, err = vm.RunProgram(program)
	if err != nil {
		return nil, fmt.Errorf("script execution error: %w", err)
	}

	fn, ok := goja.AssertFunction(vm.Get("onWebhook"))
	if !ok {
		return nil, fmt.Errorf("onWebhook function not found")
	}

	configData := s.parseConfig(pm.Config)
	hookKey := ""
	if ctx.Webhook != nil {
		hookKey = ctx.Webhook.Key
	}
	result, err := fn(goja.Undefined(), vm.ToValue(hookKey), vm.ToValue(configData))
	if err != nil {
		return nil, fmt.Errorf("onWebhook error: %w", err)
	}
	return s.parseWebhookResult(result)
}

// parseCheckResult 解析付款检查结果
func (s *JSRuntimeService) parseCheckResult(result goja.Value) (*PaymentCheckResult, error) {
	if result == nil || goja.IsUndefined(result) || goja.IsNull(result) {
		return &PaymentCheckResult{Paid: false}, nil
	}

	exported := result.Export()
	switch v := exported.(type) {
	case bool:
		return &PaymentCheckResult{Paid: v}, nil
	case map[string]interface{}:
		checkResult := &PaymentCheckResult{}
		if paid, ok := v["paid"].(bool); ok {
			checkResult.Paid = paid
		}
		if txID, ok := v["transaction_id"].(string); ok {
			checkResult.TransactionID = txID
		}
		if msg, ok := v["message"].(string); ok {
			checkResult.Message = msg
		}
		if data, ok := v["data"].(map[string]interface{}); ok {
			checkResult.Data = data
		}
		return checkResult, nil
	default:
		return &PaymentCheckResult{Paid: false}, nil
	}
}

type PaymentWebhookResult struct {
	AckStatus    int                    `json:"ack_status"`
	AckHeaders   map[string]string      `json:"ack_headers,omitempty"`
	AckBody      string                 `json:"ack_body,omitempty"`
	Paid         bool                   `json:"paid"`
	OrderID      uint                   `json:"order_id,omitempty"`
	OrderNo      string                 `json:"order_no,omitempty"`
	TransactionID string                `json:"transaction_id,omitempty"`
	Message      string                 `json:"message,omitempty"`
	Data         map[string]interface{} `json:"data,omitempty"`
	QueuePolling bool                   `json:"queue_polling,omitempty"`
}

func (s *JSRuntimeService) parseWebhookResult(result goja.Value) (*PaymentWebhookResult, error) {
	if result == nil || goja.IsUndefined(result) || goja.IsNull(result) {
		return &PaymentWebhookResult{
			AckStatus: 200,
			AckBody:   "ok",
		}, nil
	}

	exported := result.Export()
	switch v := exported.(type) {
	case bool:
		return &PaymentWebhookResult{
			AckStatus: 200,
			AckBody:   "ok",
			Paid:      v,
		}, nil
	case string:
		return &PaymentWebhookResult{
			AckStatus: 200,
			AckBody:   v,
		}, nil
	case map[string]interface{}:
		out := &PaymentWebhookResult{
			AckStatus: 200,
			AckHeaders: map[string]string{},
		}
		if value, ok := parsePaymentWebhookResultInt(v["ack_status"]); ok && value > 0 {
			out.AckStatus = value
		} else if value, ok := parsePaymentWebhookResultInt(v["status"]); ok && value > 0 {
			out.AckStatus = value
		}
		if body, contentType, err := stringifyPaymentWebhookBody(v["ack_body"]); err == nil && body != "" {
			out.AckBody = body
			if contentType != "" {
				out.AckHeaders["Content-Type"] = contentType
			}
		} else if body, contentType, err := stringifyPaymentWebhookBody(v["body"]); err == nil && body != "" {
			out.AckBody = body
			if contentType != "" {
				out.AckHeaders["Content-Type"] = contentType
			}
		}
		if headers := parsePaymentWebhookHeaders(v["ack_headers"]); len(headers) > 0 {
			for key, value := range headers {
				out.AckHeaders[key] = value
			}
		}
		if headers := parsePaymentWebhookHeaders(v["headers"]); len(headers) > 0 {
			for key, value := range headers {
				out.AckHeaders[key] = value
			}
		}
		if paid, ok := v["paid"].(bool); ok {
			out.Paid = paid
		}
		if queuePolling, ok := v["queue_polling"].(bool); ok {
			out.QueuePolling = queuePolling
		} else if queuePolling, ok := v["queuePolling"].(bool); ok {
			out.QueuePolling = queuePolling
		}
		if orderID, ok := parsePaymentWebhookResultUint(v["order_id"]); ok {
			out.OrderID = orderID
		} else if orderID, ok := parsePaymentWebhookResultUint(v["orderId"]); ok {
			out.OrderID = orderID
		}
		if orderNo, ok := v["order_no"].(string); ok {
			out.OrderNo = strings.TrimSpace(orderNo)
		} else if orderNo, ok := v["orderNo"].(string); ok {
			out.OrderNo = strings.TrimSpace(orderNo)
		}
		if txID, ok := v["transaction_id"].(string); ok {
			out.TransactionID = strings.TrimSpace(txID)
		} else if txID, ok := v["transactionId"].(string); ok {
			out.TransactionID = strings.TrimSpace(txID)
		}
		if message, ok := v["message"].(string); ok {
			out.Message = strings.TrimSpace(message)
		}
		if data, ok := v["data"].(map[string]interface{}); ok {
			out.Data = data
		}
		if out.AckBody == "" {
			out.AckBody = "ok"
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported onWebhook result type %T", exported)
	}
}

func clonePaymentWebhookRequest(req *PaymentWebhookRequest) *PaymentWebhookRequest {
	if req == nil {
		return nil
	}
	cloned := &PaymentWebhookRequest{
		Key:         req.Key,
		Method:      req.Method,
		Path:        req.Path,
		QueryString: req.QueryString,
		BodyText:    req.BodyText,
		BodyBase64:  req.BodyBase64,
		ContentType: req.ContentType,
		RemoteAddr:  req.RemoteAddr,
		QueryParams: map[string]string{},
		Headers:     map[string]string{},
	}
	for key, value := range req.QueryParams {
		cloned.QueryParams[key] = value
	}
	for key, value := range req.Headers {
		cloned.Headers[key] = value
	}
	return cloned
}

func parsePaymentWebhookResultInt(value interface{}) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return 0, false
		}
		parsed, err := strconv.Atoi(trimmed)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func parsePaymentWebhookResultUint(value interface{}) (uint, bool) {
	switch typed := value.(type) {
	case uint:
		return typed, true
	case uint64:
		return uint(typed), true
	case int:
		if typed < 0 {
			return 0, false
		}
		return uint(typed), true
	case int64:
		if typed < 0 {
			return 0, false
		}
		return uint(typed), true
	case float64:
		if typed < 0 {
			return 0, false
		}
		return uint(typed), true
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return 0, false
		}
		parsed, err := strconv.ParseUint(trimmed, 10, 64)
		if err != nil {
			return 0, false
		}
		return uint(parsed), true
	default:
		return 0, false
	}
}

func parsePaymentWebhookHeaders(value interface{}) map[string]string {
	if value == nil {
		return nil
	}
	typed, ok := value.(map[string]interface{})
	if !ok {
		return nil
	}
	out := make(map[string]string, len(typed))
	for key, raw := range typed {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "" {
			continue
		}
		out[normalizedKey] = fmt.Sprintf("%v", raw)
	}
	return out
}

func stringifyPaymentWebhookBody(value interface{}) (string, string, error) {
	if value == nil {
		return "", "", nil
	}
	switch typed := value.(type) {
	case string:
		return typed, "text/plain; charset=utf-8", nil
	case []byte:
		return string(typed), "application/octet-stream", nil
	default:
		body, err := json.Marshal(typed)
		if err != nil {
			return "", "", err
		}
		return string(body), "application/json; charset=utf-8", nil
	}
}

// PaymentCheckResult 付款检查结果
type PaymentCheckResult struct {
	Paid          bool                   `json:"paid"`
	TransactionID string                 `json:"transaction_id,omitempty"`
	Message       string                 `json:"message,omitempty"`
	Data          map[string]interface{} `json:"data,omitempty"`
}

// RefundResult 退款结果
type RefundResult struct {
	Success       bool                   `json:"success"`
	TransactionID string                 `json:"transaction_id,omitempty"`
	Message       string                 `json:"message,omitempty"`
	Data          map[string]interface{} `json:"data,omitempty"`
}

// ExecuteRefund 执行退款
func (s *JSRuntimeService) ExecuteRefund(pm *models.PaymentMethod, order *models.Order) (*RefundResult, error) {
	if pm.Script == "" {
		return &RefundResult{Success: false, Message: "Payment method has no script configured"}, nil
	}

	vm := goja.New()
	ctx := s.newJSContext(pm.ID, order)

	timer := time.AfterFunc(10*time.Second, func() {
		vm.Interrupt("execution timeout")
	})
	defer timer.Stop()

	s.registerAPIs(vm, ctx, pm)

	program, err := getOrCompileJSProgram("payment_method", pm.Script)
	if err != nil {
		return nil, fmt.Errorf("script compile error: %w", err)
	}
	_, err = vm.RunProgram(program)
	if err != nil {
		return nil, fmt.Errorf("script execution error: %w", err)
	}

	fn, ok := goja.AssertFunction(vm.Get("onRefund"))
	if !ok {
		return &RefundResult{Success: false, Message: "Payment method has not implemented refund"}, nil
	}

	orderData := s.orderToJS(order)
	configData := s.parseConfig(pm.Config)

	result, err := fn(goja.Undefined(), vm.ToValue(orderData), vm.ToValue(configData))
	if err != nil {
		return nil, fmt.Errorf("onRefund error: %w", err)
	}

	return s.parseRefundResult(result)
}

// parseRefundResult 解析退款结果
func (s *JSRuntimeService) parseRefundResult(result goja.Value) (*RefundResult, error) {
	if result == nil || goja.IsUndefined(result) || goja.IsNull(result) {
		return &RefundResult{Success: false}, nil
	}

	exported := result.Export()
	switch v := exported.(type) {
	case bool:
		return &RefundResult{Success: v}, nil
	case map[string]interface{}:
		refundResult := &RefundResult{}
		if success, ok := v["success"].(bool); ok {
			refundResult.Success = success
		}
		if txID, ok := v["transaction_id"].(string); ok {
			refundResult.TransactionID = txID
		}
		if msg, ok := v["message"].(string); ok {
			refundResult.Message = msg
		}
		if data, ok := v["data"].(map[string]interface{}); ok {
			refundResult.Data = data
		}
		return refundResult, nil
	default:
		return &RefundResult{Success: false}, nil
	}
}

// 新增 API 函数

// createOrderGetItems 获取订单商品列表
func (s *JSRuntimeService) createOrderGetItems(vm *goja.Runtime, ctx *JSContext) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if ctx.Order == nil {
			return vm.ToValue([]interface{}{})
		}

		var result []map[string]interface{}
		for i, item := range ctx.Order.Items {
			result = append(result, map[string]interface{}{
				"index":        i,
				"name":         item.Name,
				"sku":          item.SKU,
				"quantity":     item.Quantity,
				"image_url":    item.ImageURL,
				"product_type": item.ProductType,
				"attributes":   item.Attributes,
			})
		}
		return vm.ToValue(result)
	}
}

// createOrderGetUser 获取订单用户信息
func (s *JSRuntimeService) createOrderGetUser(vm *goja.Runtime, ctx *JSContext) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if ctx.Order == nil || ctx.Order.UserID == nil {
			return goja.Undefined()
		}

		var user models.User
		if err := s.db.First(&user, *ctx.Order.UserID).Error; err != nil {
			return goja.Undefined()
		}

		return vm.ToValue(map[string]interface{}{
			"id":    user.ID,
			"email": user.Email,
			"name":  user.Name,
		})
	}
}

// createFormatDate 格式化日期
func (s *JSRuntimeService) createFormatDate(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue("")
		}

		var t time.Time
		arg := call.Arguments[0].Export()

		switch v := arg.(type) {
		case string:
			parsed, err := time.Parse(time.RFC3339, v)
			if err != nil {
				return vm.ToValue(v)
			}
			t = parsed
		case time.Time:
			t = v
		case int64:
			t = time.Unix(v, 0)
		default:
			return vm.ToValue("")
		}

		format := "2006-01-02 15:04:05"
		if len(call.Arguments) > 1 {
			format = call.Arguments[1].String()
		}

		return vm.ToValue(t.Format(format))
	}
}

// createGenerateId 生成唯一ID
func (s *JSRuntimeService) createGenerateId(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(uuid.New().String())
	}
}

// createMD5 计算MD5
func (s *JSRuntimeService) createMD5(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue("")
		}
		data := call.Arguments[0].String()
		hash := md5.Sum([]byte(data))
		return vm.ToValue(hex.EncodeToString(hash[:]))
	}
}

// createHMACSHA256 计算 HMAC-SHA256 十六进制摘要
func (s *JSRuntimeService) createHMACSHA256(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return vm.ToValue("")
		}
		payload := call.Arguments[0].String()
		secret := call.Arguments[1].String()
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write([]byte(payload))
		return vm.ToValue(hex.EncodeToString(mac.Sum(nil)))
	}
}

// createBase64Encode Base64编码
func (s *JSRuntimeService) createBase64Encode(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue("")
		}
		data := call.Arguments[0].String()
		return vm.ToValue(base64.StdEncoding.EncodeToString([]byte(data)))
	}
}

// createBase64Decode Base64解码
func (s *JSRuntimeService) createBase64Decode(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue("")
		}
		data := call.Arguments[0].String()
		decoded, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			return vm.ToValue("")
		}
		return vm.ToValue(string(decoded))
	}
}

// createJSONEncode JSON编码
func (s *JSRuntimeService) createJSONEncode(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue("")
		}
		data := call.Arguments[0].Export()
		jsonBytes, err := json.Marshal(data)
		if err != nil {
			return vm.ToValue("")
		}
		return vm.ToValue(string(jsonBytes))
	}
}

// createJSONDecode JSON解码
func (s *JSRuntimeService) createJSONDecode(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Undefined()
		}
		data := call.Arguments[0].String()
		var result interface{}
		if err := json.Unmarshal([]byte(data), &result); err != nil {
			return goja.Undefined()
		}
		return vm.ToValue(result)
	}
}

// createQRCode 生成QR码（返回data URI）
// 支持两种调用方式：
//   - qrcode(text, size)          — 向后兼容
//   - qrcode(text, { size, level, fg, bg, disableBorder })
func (s *JSRuntimeService) createQRCode(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue("")
		}
		text := call.Arguments[0].String()
		if text == "" {
			return vm.ToValue("")
		}

		size := 256
		level := qrcode.Medium
		var fgColor color.Color = color.Black
		var bgColor color.Color = color.White
		disableBorder := false

		if len(call.Arguments) >= 2 {
			arg := call.Arguments[1].Export()
			switch v := arg.(type) {
			case int64:
				if v > 0 && v <= 1024 {
					size = int(v)
				}
			case float64:
				if v > 0 && v <= 1024 {
					size = int(v)
				}
			case map[string]interface{}:
				if s, ok := toInt(v["size"]); ok && s > 0 && s <= 1024 {
					size = s
				}
				if l, ok := v["level"].(string); ok {
					switch strings.ToUpper(l) {
					case "L", "LOW":
						level = qrcode.Low
					case "M", "MEDIUM":
						level = qrcode.Medium
					case "Q", "HIGH":
						level = qrcode.High
					case "H", "HIGHEST":
						level = qrcode.Highest
					}
				}
				if fg, ok := v["fg"].(string); ok {
					if c, err := parseHexColor(fg); err == nil {
						fgColor = c
					}
				}
				if bg, ok := v["bg"].(string); ok {
					if c, err := parseHexColor(bg); err == nil {
						bgColor = c
					}
				}
				if db, ok := v["disableBorder"].(bool); ok {
					disableBorder = db
				}
			}
		}

		qrc, err := qrcode.New(text, level)
		if err != nil {
			return vm.ToValue("")
		}
		qrc.ForegroundColor = fgColor
		qrc.BackgroundColor = bgColor
		qrc.DisableBorder = disableBorder

		png, err := qrc.PNG(size)
		if err != nil {
			return vm.ToValue("")
		}
		dataURI := "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
		return vm.ToValue(dataURI)
	}
}

// toInt 尝试将 interface{} 转为 int
func toInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	case int:
		return n, true
	}
	return 0, false
}

// parseHexColor 解析十六进制颜色字符串（支持 #RGB, #RGBA, #RRGGBB, #RRGGBBAA）
func parseHexColor(s string) (color.Color, error) {
	s = strings.TrimPrefix(s, "#")
	var r, g, b, a uint8
	a = 255
	switch len(s) {
	case 3: // RGB
		_, err := fmt.Sscanf(s, "%1x%1x%1x", &r, &g, &b)
		if err != nil {
			return nil, err
		}
		r *= 17
		g *= 17
		b *= 17
	case 4: // RGBA
		_, err := fmt.Sscanf(s, "%1x%1x%1x%1x", &r, &g, &b, &a)
		if err != nil {
			return nil, err
		}
		r *= 17
		g *= 17
		b *= 17
		a *= 17
	case 6: // RRGGBB
		_, err := fmt.Sscanf(s, "%02x%02x%02x", &r, &g, &b)
		if err != nil {
			return nil, err
		}
	case 8: // RRGGBBAA
		_, err := fmt.Sscanf(s, "%02x%02x%02x%02x", &r, &g, &b, &a)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("invalid hex color: %s", s)
	}
	return color.RGBA{R: r, G: g, B: b, A: a}, nil
}

// createGetTimestamp 获取当前时间戳
func (s *JSRuntimeService) createGetTimestamp(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(time.Now().Unix())
	}
}

// createGetPaymentMethodInfo 获取付款方式信息
func (s *JSRuntimeService) createGetPaymentMethodInfo(vm *goja.Runtime, pm *models.PaymentMethod) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(map[string]interface{}{
			"id":                pm.ID,
			"name":              pm.Name,
			"description":       pm.Description,
			"type":              pm.Type,
			"icon":              pm.Icon,
			"webhook_base_path": buildPaymentWebhookPath(pm.ID, ""),
			"webhook_base_url":  s.buildPaymentWebhookURL(pm.ID, ""),
		})
	}
}

func (s *JSRuntimeService) createGetWebhookURL(vm *goja.Runtime, pm *models.PaymentMethod) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		hook := ""
		if len(call.Arguments) > 0 {
			hook = call.Arguments[0].String()
		}
		return vm.ToValue(s.buildPaymentWebhookURL(pm.ID, hook))
	}
}

func (s *JSRuntimeService) buildPaymentWebhookURL(paymentMethodID uint, hook string) string {
	path := buildPaymentWebhookPath(paymentMethodID, hook)
	if s == nil || s.publicBaseURL == "" {
		return path
	}
	return s.publicBaseURL + path
}

func buildPaymentWebhookPath(paymentMethodID uint, hook string) string {
	path := fmt.Sprintf("/api/payment-methods/%d/webhooks", paymentMethodID)
	normalizedHook := strings.Trim(strings.TrimSpace(hook), "/")
	if normalizedHook != "" {
		path += "/" + normalizedHook
	}
	return path
}
