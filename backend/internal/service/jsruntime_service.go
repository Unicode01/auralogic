package service

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"
	"github.com/google/uuid"
	"auralogic/internal/models"
	"gorm.io/gorm"
)

// paymentStorageDir 付款方式存储目录
const paymentStorageDir = "storage/payments"

// Block SSRF to internal networks from payment scripts by default.
// If a deployment needs access to internal services, it should be implemented
// explicitly with allowlists rather than enabling broad network access here.
func newPaymentHTTPClient() *http.Client {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
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

// paymentStorageMutex 文件操作锁（按付款方式ID隔离）
var (
	paymentStorageLocks = make(map[uint]*sync.RWMutex)
	paymentLocksLock    sync.Mutex
)

// getPaymentStorageLock 获取付款方式的存储锁
func getPaymentStorageLock(paymentMethodID uint) *sync.RWMutex {
	paymentLocksLock.Lock()
	defer paymentLocksLock.Unlock()

	if lock, ok := paymentStorageLocks[paymentMethodID]; ok {
		return lock
	}
	lock := &sync.RWMutex{}
	paymentStorageLocks[paymentMethodID] = lock
	return lock
}

// getStorageFilePath 获取付款方式的存储文件路径
func getStorageFilePath(paymentMethodID uint) string {
	return filepath.Join(paymentStorageDir, fmt.Sprintf("%d.json", paymentMethodID))
}

// readPaymentStorageUnsafe 读取付款方式存储（调用者必须持有锁）
func readPaymentStorageUnsafe(paymentMethodID uint) (map[string]string, error) {
	filePath := getStorageFilePath(paymentMethodID)
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]string), nil
		}
		return nil, err
	}

	var storage map[string]string
	if err := json.Unmarshal(data, &storage); err != nil {
		return make(map[string]string), nil
	}
	return storage, nil
}

// writePaymentStorageUnsafe 写入付款方式存储（调用者必须持有写锁）
func writePaymentStorageUnsafe(paymentMethodID uint, storage map[string]string) error {
	// 确保目录存在
	if err := os.MkdirAll(paymentStorageDir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(storage, "", "  ")
	if err != nil {
		return err
	}

	filePath := getStorageFilePath(paymentMethodID)

	// 先写入临时文件，再原子重命名，防止写入中断导致数据损坏
	tempPath := filePath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return err
	}
	return os.Rename(tempPath, filePath)
}

// JSRuntimeService JS运行时服务
type JSRuntimeService struct {
	db *gorm.DB
}

// NewJSRuntimeService 创建JS运行时服务
func NewJSRuntimeService(db *gorm.DB) *JSRuntimeService {
	return &JSRuntimeService{db: db}
}

// JSContext JS执行上下文
type JSContext struct {
	PaymentMethodID uint
	OrderID         uint
	Order           *models.Order
	DB              *gorm.DB
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
	ctx := &JSContext{
		PaymentMethodID: pm.ID,
		OrderID:         order.ID,
		Order:           order,
		DB:              s.db,
	}

	// 设置超时
	timer := time.AfterFunc(5*time.Second, func() {
		vm.Interrupt("execution timeout")
	})
	defer timer.Stop()

	// 注册系统API
	s.registerAPIs(vm, ctx, pm)

	// 执行脚本
	_, err := vm.RunString(pm.Script)
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
			<p class="mt-2">Amount: <span class="font-bold text-primary">%s %.2f</span></p>
			<p class="mt-1">Order No: <code class="bg-muted px-1 rounded">%s</code></p>
		</div>
	`, pm.Description, order.Currency, order.TotalAmount, order.OrderNo)

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
	utils.Set("formatPrice", s.createFormatPrice(vm))
	utils.Set("formatDate", s.createFormatDate(vm))
	utils.Set("generateId", s.createGenerateId(vm))
	utils.Set("md5", s.createMD5(vm))
	utils.Set("base64Encode", s.createBase64Encode(vm))
	utils.Set("base64Decode", s.createBase64Decode(vm))
	utils.Set("jsonEncode", s.createJSONEncode(vm))
	utils.Set("jsonDecode", s.createJSONDecode(vm))

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
}

// Storage APIs - 本地文件存储（按付款方式隔离）
func (s *JSRuntimeService) createStorageGet(vm *goja.Runtime, ctx *JSContext) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Undefined()
		}
		key := call.Arguments[0].String()

		lock := getPaymentStorageLock(ctx.PaymentMethodID)
		lock.RLock()
		defer lock.RUnlock()

		storage, err := readPaymentStorageUnsafe(ctx.PaymentMethodID)
		if err != nil {
			return goja.Undefined()
		}

		if value, exists := storage[key]; exists {
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

		lock := getPaymentStorageLock(ctx.PaymentMethodID)
		lock.Lock()
		defer lock.Unlock()

		storage, _ := readPaymentStorageUnsafe(ctx.PaymentMethodID)
		storage[key] = value

		if err := writePaymentStorageUnsafe(ctx.PaymentMethodID, storage); err != nil {
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

		lock := getPaymentStorageLock(ctx.PaymentMethodID)
		lock.Lock()
		defer lock.Unlock()

		storage, _ := readPaymentStorageUnsafe(ctx.PaymentMethodID)
		delete(storage, key)

		if err := writePaymentStorageUnsafe(ctx.PaymentMethodID, storage); err != nil {
			return vm.ToValue(false)
		}
		return vm.ToValue(true)
	}
}

// createStorageList 列出当前付款方式的所有存储键
func (s *JSRuntimeService) createStorageList(vm *goja.Runtime, ctx *JSContext) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		lock := getPaymentStorageLock(ctx.PaymentMethodID)
		lock.RLock()
		defer lock.RUnlock()

		storage, err := readPaymentStorageUnsafe(ctx.PaymentMethodID)
		if err != nil {
			return vm.ToValue([]string{})
		}

		keys := make([]string, 0, len(storage))
		for key := range storage {
			keys = append(keys, key)
		}
		return vm.ToValue(keys)
	}
}

// createStorageClear 清除当前付款方式的所有存储
func (s *JSRuntimeService) createStorageClear(vm *goja.Runtime, ctx *JSContext) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		lock := getPaymentStorageLock(ctx.PaymentMethodID)
		lock.Lock()
		defer lock.Unlock()

		filePath := getStorageFilePath(ctx.PaymentMethodID)
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
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
func (s *JSRuntimeService) createFormatPrice(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue("")
		}
		amount := call.Arguments[0].ToFloat()
		currency := "CNY"
		if len(call.Arguments) > 1 {
			currency = call.Arguments[1].String()
		}

		symbols := map[string]string{
			"CNY": "¥", "USD": "$", "EUR": "€", "JPY": "¥", "GBP": "£",
		}
		symbol := symbols[currency]
		if symbol == "" {
			symbol = currency + " "
		}
		return vm.ToValue(fmt.Sprintf("%s%.2f", symbol, amount))
	}
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

		// ���析请求参数
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

		client := newPaymentHTTPClient()

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
	return map[string]interface{}{
		"id":           order.ID,
		"order_no":     order.OrderNo,
		"status":       order.Status,
		"total_amount": order.TotalAmount,
		"currency":     order.Currency,
		"created_at":   order.CreatedAt.Format(time.RFC3339),
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
	ctx := &JSContext{
		PaymentMethodID: pm.ID,
		OrderID:         order.ID,
		Order:           order,
		DB:              s.db,
	}

	// 设置超时
	timer := time.AfterFunc(10*time.Second, func() {
		vm.Interrupt("execution timeout")
	})
	defer timer.Stop()

	// 注册系统API
	s.registerAPIs(vm, ctx, pm)

	// 执行脚本
	_, err := vm.RunString(pm.Script)
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
	ctx := &JSContext{
		PaymentMethodID: pm.ID,
		OrderID:         order.ID,
		Order:           order,
		DB:              s.db,
	}

	time.AfterFunc(10*time.Second, func() {
		vm.Interrupt("execution timeout")
	})

	s.registerAPIs(vm, ctx, pm)

	_, err := vm.RunString(pm.Script)
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
		if ctx.Order == nil {
			return goja.Undefined()
		}

		var user models.User
		if err := s.db.First(&user, ctx.Order.UserID).Error; err != nil {
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
			"id":          pm.ID,
			"name":        pm.Name,
			"description": pm.Description,
			"type":        pm.Type,
			"icon":        pm.Icon,
		})
	}
}
