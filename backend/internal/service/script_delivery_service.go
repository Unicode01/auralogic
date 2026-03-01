package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"auralogic/internal/models"

	"github.com/dop251/goja"
	"gorm.io/gorm"
)

// ScriptDeliveryResult JS脚本发货结果
type ScriptDeliveryResult struct {
	Success bool                 `json:"success"`
	Items   []ScriptDeliveryItem `json:"items"`
	Message string               `json:"message,omitempty"`
}

// ScriptDeliveryItem 脚本发货的单个卡密项
type ScriptDeliveryItem struct {
	Content string `json:"content"`
	Remark  string `json:"remark,omitempty"`
}

// ScriptDeliveryService JS脚本发货服务
type ScriptDeliveryService struct {
	db *gorm.DB
}

// NewScriptDeliveryService 创建脚本发货服务
func NewScriptDeliveryService(db *gorm.DB) *ScriptDeliveryService {
	return &ScriptDeliveryService{db: db}
}

// ScriptDeliveryContext 脚本执行上下文
type ScriptDeliveryContext struct {
	VirtualInventoryID uint
	OrderID            uint
	OrderNo            string
	Quantity           int
}

// ExecuteDeliveryScript 执行发货脚本
// 调用脚本中的 onDeliver(order, config) 函数，返回发货结果
func (s *ScriptDeliveryService) ExecuteDeliveryScript(
	inventory *models.VirtualInventory,
	order *models.Order,
	quantity int,
) (*ScriptDeliveryResult, error) {
	if inventory.Script == "" {
		return nil, fmt.Errorf("inventory %d has no script", inventory.ID)
	}

	vm := goja.New()

	// 设置超时（10秒，脚本发货可能涉及外部API调用）
	timer := time.AfterFunc(10*time.Second, func() {
		vm.Interrupt("execution timeout")
	})
	defer timer.Stop()

	ctx := &ScriptDeliveryContext{
		VirtualInventoryID: inventory.ID,
		OrderID:            order.ID,
		OrderNo:            order.OrderNo,
		Quantity:           quantity,
	}

	// 注册API
	s.registerAPIs(vm, ctx, inventory, order)

	// 执行脚本
	_, err := vm.RunString(inventory.Script)
	if err != nil {
		return nil, fmt.Errorf("script execution error: %w", err)
	}

	// 调用 onDeliver 函数
	fn, ok := goja.AssertFunction(vm.Get("onDeliver"))
	if !ok {
		return nil, fmt.Errorf("onDeliver function not found in script")
	}

	// 准备参数
	orderData := s.orderToJS(order, quantity)
	configData := s.parseScriptConfig(inventory.ScriptConfig)

	result, err := fn(goja.Undefined(), vm.ToValue(orderData), vm.ToValue(configData))
	if err != nil {
		return nil, fmt.Errorf("onDeliver execution error: %w", err)
	}

	return s.parseDeliveryResult(result, quantity)
}

// registerAPIs 注册脚本API
func (s *ScriptDeliveryService) registerAPIs(
	vm *goja.Runtime,
	ctx *ScriptDeliveryContext,
	inventory *models.VirtualInventory,
	order *models.Order,
) {
	auralogic := vm.NewObject()
	vm.Set("AuraLogic", auralogic)

	// 订单API（只读）
	orderObj := vm.NewObject()
	auralogic.Set("order", orderObj)
	orderObj.Set("get", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(s.orderToJS(order, ctx.Quantity))
	})
	orderObj.Set("getItems", func(call goja.FunctionCall) goja.Value {
		var items []models.OrderItem
		s.db.Where("order_id = ?", order.ID).Find(&items)
		var result []map[string]interface{}
		for _, item := range items {
			result = append(result, map[string]interface{}{
				"sku":          item.SKU,
				"name":         item.Name,
				"quantity":     item.Quantity,
				"product_type": item.ProductType,
			})
		}
		if result == nil {
			// Order items are embedded JSON, use order.Items directly
			for _, item := range order.Items {
				result = append(result, map[string]interface{}{
					"sku":          item.SKU,
					"name":         item.Name,
					"quantity":     item.Quantity,
					"product_type": item.ProductType,
				})
			}
		}
		return vm.ToValue(result)
	})
	orderObj.Set("getUser", func(call goja.FunctionCall) goja.Value {
		if order.UserID == nil {
			return goja.Undefined()
		}
		var user models.User
		if err := s.db.First(&user, *order.UserID).Error; err != nil {
			return goja.Undefined()
		}
		return vm.ToValue(map[string]interface{}{
			"id":    user.ID,
			"name":  user.Name,
			"email": user.Email,
		})
	})

	// 工具API
	utils := vm.NewObject()
	auralogic.Set("utils", utils)
	utils.Set("generateId", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(fmt.Sprintf("%d", time.Now().UnixNano()))
	})
	utils.Set("jsonEncode", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue("")
		}
		data := call.Arguments[0].Export()
		b, err := json.Marshal(data)
		if err != nil {
			return vm.ToValue("")
		}
		return vm.ToValue(string(b))
	})
	utils.Set("jsonDecode", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Undefined()
		}
		var result interface{}
		if err := json.Unmarshal([]byte(call.Arguments[0].String()), &result); err != nil {
			return goja.Undefined()
		}
		return vm.ToValue(result)
	})
	utils.Set("formatDate", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(time.Now().Format(time.RFC3339))
	})

	// HTTP API（使用 SSRF-safe 客户端）
	httpObj := vm.NewObject()
	auralogic.Set("http", httpObj)
	httpObj.Set("get", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(map[string]interface{}{"error": "URL is required", "status": 0})
		}
		return s.doHTTPRequest(vm, "GET", call.Arguments[0].String(), nil, s.extractHeaders(call, 1))
	})
	httpObj.Set("post", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(map[string]interface{}{"error": "URL is required", "status": 0})
		}
		var body interface{}
		if len(call.Arguments) > 1 {
			body = call.Arguments[1].Export()
		}
		return s.doHTTPRequest(vm, "POST", call.Arguments[0].String(), body, s.extractHeaders(call, 2))
	})
	httpObj.Set("request", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return vm.ToValue(map[string]interface{}{"error": "method and URL are required", "status": 0})
		}
		var body interface{}
		if len(call.Arguments) > 2 {
			body = call.Arguments[2].Export()
		}
		return s.doHTTPRequest(vm, call.Arguments[0].String(), call.Arguments[1].String(), body, s.extractHeaders(call, 3))
	})

	// 配置API
	config := vm.NewObject()
	auralogic.Set("config", config)
	configData := s.parseScriptConfig(inventory.ScriptConfig)
	config.Set("get", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(configData)
		}
		key := call.Arguments[0].String()
		if val, ok := configData[key]; ok {
			return vm.ToValue(val)
		}
		if len(call.Arguments) > 1 {
			return call.Arguments[1]
		}
		return goja.Undefined()
	})

	// 系统API
	system := vm.NewObject()
	auralogic.Set("system", system)
	system.Set("getTimestamp", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(time.Now().Unix())
	})
	system.Set("log", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			log.Printf("[ScriptDelivery] inventory=%d order=%s: %s",
				ctx.VirtualInventoryID, ctx.OrderNo, call.Arguments[0].String())
		}
		return goja.Undefined()
	})
}

func (s *ScriptDeliveryService) extractHeaders(call goja.FunctionCall, idx int) map[string]string {
	headers := make(map[string]string)
	if len(call.Arguments) > idx {
		if headersObj := call.Arguments[idx].Export(); headersObj != nil {
			if h, ok := headersObj.(map[string]interface{}); ok {
				for k, v := range h {
					if str, ok := v.(string); ok {
						headers[k] = str
					}
				}
			}
		}
	}
	return headers
}

// doHTTPRequest 执行HTTP请求（SSRF安全）
func (s *ScriptDeliveryService) doHTTPRequest(vm *goja.Runtime, method, urlStr string, body interface{}, headers map[string]string) goja.Value {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return vm.ToValue(map[string]interface{}{"error": fmt.Sprintf("Invalid URL: %v", err), "status": 0})
	}
	if err := validateExternalURL(parsedURL); err != nil {
		return vm.ToValue(map[string]interface{}{"error": "URL is not allowed", "status": 0})
	}

	client := newPaymentHTTPClient()

	// 准备请求体
	var reqBody io.Reader
	contentType := ""
	if body != nil {
		switch v := body.(type) {
		case string:
			reqBody = strings.NewReader(v)
			if strings.HasPrefix(strings.TrimSpace(v), "{") || strings.HasPrefix(strings.TrimSpace(v), "[") {
				contentType = "application/json"
			} else {
				contentType = "text/plain"
			}
		default:
			jsonBytes, err := json.Marshal(v)
			if err != nil {
				return vm.ToValue(map[string]interface{}{"error": fmt.Sprintf("Failed to encode body: %v", err), "status": 0})
			}
			reqBody = bytes.NewReader(jsonBytes)
			contentType = "application/json"
		}
	}

	req, err := http.NewRequest(method, parsedURL.String(), reqBody)
	if err != nil {
		return vm.ToValue(map[string]interface{}{"error": fmt.Sprintf("Failed to create request: %v", err), "status": 0})
	}

	req.Header.Set("User-Agent", "AuraLogic-ScriptDelivery/1.0")
	if contentType != "" && headers["Content-Type"] == "" {
		req.Header.Set("Content-Type", contentType)
	}
	for k, v := range headers {
		if strings.EqualFold(k, "Host") || strings.EqualFold(k, "Proxy-Authorization") {
			continue
		}
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return vm.ToValue(map[string]interface{}{"error": fmt.Sprintf("Request failed: %v", err), "status": 0})
	}
	defer resp.Body.Close()

	limitedReader := io.LimitReader(resp.Body, 10*1024*1024)
	respBody, err := io.ReadAll(limitedReader)
	if err != nil {
		return vm.ToValue(map[string]interface{}{"error": fmt.Sprintf("Failed to read response: %v", err), "status": resp.StatusCode})
	}

	result := map[string]interface{}{
		"status": resp.StatusCode,
		"body":   string(respBody),
	}

	var jsonData interface{}
	if strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
		json.Unmarshal(respBody, &jsonData)
	}
	if jsonData != nil {
		result["data"] = jsonData
	}

	return vm.ToValue(result)
}

// orderToJS 将订单转换为JS对象
func (s *ScriptDeliveryService) orderToJS(order *models.Order, quantity int) map[string]interface{} {
	return map[string]interface{}{
		"id":           order.ID,
		"order_no":     order.OrderNo,
		"status":       order.Status,
		"total_amount": order.TotalAmount,
		"currency":     order.Currency,
		"quantity":     quantity,
		"created_at":   order.CreatedAt.Format(time.RFC3339),
	}
}

// parseScriptConfig 解析JSON配置
func (s *ScriptDeliveryService) parseScriptConfig(configJSON string) map[string]interface{} {
	config := make(map[string]interface{})
	if configJSON != "" {
		if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
			log.Printf("[ScriptDelivery] Warning: failed to parse script_config JSON: %v", err)
		}
	}
	return config
}

// parseDeliveryResult 解析发货结果
func (s *ScriptDeliveryService) parseDeliveryResult(result goja.Value, expectedQty int) (*ScriptDeliveryResult, error) {
	if result == nil || goja.IsUndefined(result) || goja.IsNull(result) {
		return nil, fmt.Errorf("script returned empty result")
	}

	exported := result.Export()
	obj, ok := exported.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("script must return an object with {success, items}")
	}

	deliveryResult := &ScriptDeliveryResult{}

	if success, ok := obj["success"].(bool); ok {
		deliveryResult.Success = success
	}
	if msg, ok := obj["message"].(string); ok {
		deliveryResult.Message = msg
	}

	if !deliveryResult.Success {
		msg := deliveryResult.Message
		if msg == "" {
			msg = "script delivery failed"
		}
		return deliveryResult, fmt.Errorf("script delivery failed: %s", msg)
	}

	// 解析 items
	if items, ok := obj["items"]; ok {
		if arr, ok := items.([]interface{}); ok {
			for _, item := range arr {
				if itemMap, ok := item.(map[string]interface{}); ok {
					di := ScriptDeliveryItem{}
					if content, ok := itemMap["content"].(string); ok {
						di.Content = content
					}
					if remark, ok := itemMap["remark"].(string); ok {
						di.Remark = remark
					}
					if di.Content != "" {
						deliveryResult.Items = append(deliveryResult.Items, di)
					}
				}
			}
		}
	}

	if len(deliveryResult.Items) == 0 {
		return deliveryResult, fmt.Errorf("script returned no delivery items")
	}

	if len(deliveryResult.Items) != expectedQty {
		log.Printf("[ScriptDelivery] Warning: expected %d items, got %d", expectedQty, len(deliveryResult.Items))
	}

	return deliveryResult, nil
}
