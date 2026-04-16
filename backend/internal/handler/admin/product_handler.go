package admin

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	"auralogic/internal/database"
	"auralogic/internal/middleware"
	"auralogic/internal/models"
	"auralogic/internal/pkg/logger"
	"auralogic/internal/pkg/response"
	"auralogic/internal/pkg/utils"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
)

type ProductHandler struct {
	productService          *service.ProductService
	virtualInventoryService *service.VirtualInventoryService
	pluginManager           *service.PluginManagerService
}

type productListFilters struct {
	Status     string
	Category   string
	Search     string
	IsFeatured *bool
}

func NewProductHandler(productService *service.ProductService, virtualInventoryService *service.VirtualInventoryService, pluginManager *service.PluginManagerService) *ProductHandler {
	return &ProductHandler{
		productService:          productService,
		virtualInventoryService: virtualInventoryService,
		pluginManager:           pluginManager,
	}
}

func (h *ProductHandler) buildProductHookExecutionContext(c *gin.Context, adminID uint, productID uint) *service.ExecutionContext {
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
	}
	if productID > 0 {
		metadata["product_id"] = strconv.FormatUint(uint64(productID), 10)
	}
	sessionID := strings.TrimSpace(c.GetHeader("X-Session-ID"))
	return &service.ExecutionContext{
		UserID:         &adminID,
		SessionID:      sessionID,
		Metadata:       metadata,
		RequestContext: c.Request.Context(),
	}
}

func respondProductServiceError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	if respondAdminBizError(c, err) {
		return true
	}
	if errors.Is(err, service.ErrProductNotFound) {
		response.NotFound(c, err.Error())
		return true
	}
	return false
}

func productHookValueToOptionalString(value interface{}) (string, error) {
	if value == nil {
		return "", nil
	}
	str, ok := value.(string)
	if !ok {
		return "", errors.New("value must be string")
	}
	return str, nil
}

func productHookValueToProductStatus(value interface{}) (models.ProductStatus, error) {
	raw, err := productHookValueToOptionalString(value)
	if err != nil {
		return "", err
	}
	status := models.ProductStatus(strings.ToLower(strings.TrimSpace(raw)))
	switch status {
	case models.ProductStatusDraft, models.ProductStatusActive, models.ProductStatusInactive, models.ProductStatusOutOfStock:
		return status, nil
	default:
		return "", errors.New("invalid product status")
	}
}

func productHookValueToInventoryMode(value interface{}) (string, error) {
	raw, err := productHookValueToOptionalString(value)
	if err != nil {
		return "", err
	}
	mode := strings.ToLower(strings.TrimSpace(raw))
	switch mode {
	case string(models.InventoryModeFixed), string(models.InventoryModeRandom):
		return mode, nil
	default:
		return "", errors.New("invalid inventory mode")
	}
}

func productHookValueToProductType(value interface{}) (models.ProductType, error) {
	raw, err := productHookValueToOptionalString(value)
	if err != nil {
		return "", err
	}
	productType := models.ProductType(strings.ToLower(strings.TrimSpace(raw)))
	switch productType {
	case models.ProductTypePhysical, models.ProductTypeVirtual:
		return productType, nil
	default:
		return "", errors.New("invalid product type")
	}
}

func productHookValueToInt64(value interface{}) (int64, error) {
	switch typed := value.(type) {
	case int:
		return int64(typed), nil
	case int32:
		return int64(typed), nil
	case int64:
		return typed, nil
	case uint:
		return int64(typed), nil
	case uint32:
		return int64(typed), nil
	case uint64:
		if typed > uint64(^uint64(0)>>1) {
			return 0, errors.New("value out of range")
		}
		return int64(typed), nil
	case float64:
		out := int64(typed)
		if float64(out) != typed {
			return 0, errors.New("value must be integer")
		}
		return out, nil
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return 0, nil
		}
		out, err := strconv.ParseInt(trimmed, 10, 64)
		if err != nil {
			return 0, errors.New("invalid int64 string")
		}
		return out, nil
	default:
		return 0, errors.New("value must be int64")
	}
}

func productHookValueToInt(value interface{}) (int, error) {
	out, err := productHookValueToInt64(value)
	if err != nil {
		return 0, err
	}
	converted := int(out)
	if int64(converted) != out {
		return 0, errors.New("value out of range")
	}
	return converted, nil
}

func productHookValueToBool(value interface{}) (bool, error) {
	switch typed := value.(type) {
	case bool:
		return typed, nil
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return false, nil
		}
		out, err := strconv.ParseBool(trimmed)
		if err != nil {
			return false, errors.New("invalid bool string")
		}
		return out, nil
	case int:
		if typed == 0 {
			return false, nil
		}
		if typed == 1 {
			return true, nil
		}
	case int64:
		if typed == 0 {
			return false, nil
		}
		if typed == 1 {
			return true, nil
		}
	case float64:
		if typed == 0 {
			return false, nil
		}
		if typed == 1 {
			return true, nil
		}
	}
	return false, errors.New("value must be bool")
}

func productHookDecodeJSONValue(value interface{}, target interface{}) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, target)
}

func productHookValueToStringSlice(value interface{}) ([]string, error) {
	if value == nil {
		return nil, nil
	}
	var out []string
	if err := productHookDecodeJSONValue(value, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func productHookValueToImages(value interface{}) ([]models.ProductImage, error) {
	if value == nil {
		return nil, nil
	}
	var out []models.ProductImage
	if err := productHookDecodeJSONValue(value, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func productHookValueToAttributes(value interface{}) ([]models.ProductAttribute, error) {
	if value == nil {
		return nil, nil
	}
	var out []models.ProductAttribute
	if err := productHookDecodeJSONValue(value, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func applyUpdateProductHookPayload(req *UpdateProductRequest, payload map[string]interface{}) error {
	if req == nil || payload == nil {
		return nil
	}

	if raw, exists := payload["sku"]; exists {
		value, err := productHookValueToOptionalString(raw)
		if err != nil {
			return fmt.Errorf("decode sku: %w", err)
		}
		req.SKU = value
	}
	if raw, exists := payload["name"]; exists {
		value, err := productHookValueToOptionalString(raw)
		if err != nil {
			return fmt.Errorf("decode name: %w", err)
		}
		req.Name = value
	}
	if raw, exists := payload["product_code"]; exists {
		value, err := productHookValueToOptionalString(raw)
		if err != nil {
			return fmt.Errorf("decode product_code: %w", err)
		}
		req.ProductCode = value
	}
	if raw, exists := payload["product_type"]; exists {
		value, err := productHookValueToProductType(raw)
		if err != nil {
			return fmt.Errorf("decode product_type: %w", err)
		}
		req.ProductType = value
	}
	if raw, exists := payload["description"]; exists {
		value, err := productHookValueToOptionalString(raw)
		if err != nil {
			return fmt.Errorf("decode description: %w", err)
		}
		req.Description = value
	}
	if raw, exists := payload["short_description"]; exists {
		value, err := productHookValueToOptionalString(raw)
		if err != nil {
			return fmt.Errorf("decode short_description: %w", err)
		}
		req.ShortDescription = value
	}
	if raw, exists := payload["category"]; exists {
		value, err := productHookValueToOptionalString(raw)
		if err != nil {
			return fmt.Errorf("decode category: %w", err)
		}
		req.Category = value
	}
	if raw, exists := payload["tags"]; exists {
		value, err := productHookValueToStringSlice(raw)
		if err != nil {
			return fmt.Errorf("decode tags: %w", err)
		}
		req.Tags = value
	}
	if raw, exists := payload["price_minor"]; exists {
		value, err := productHookValueToInt64(raw)
		if err != nil {
			return fmt.Errorf("decode price_minor: %w", err)
		}
		req.PriceMinor = value
	}
	if raw, exists := payload["original_price_minor"]; exists {
		value, err := productHookValueToInt64(raw)
		if err != nil {
			return fmt.Errorf("decode original_price_minor: %w", err)
		}
		req.OriginalPriceMinor = value
	}
	if raw, exists := payload["stock"]; exists {
		value, err := productHookValueToInt(raw)
		if err != nil {
			return fmt.Errorf("decode stock: %w", err)
		}
		req.Stock = value
	}
	if raw, exists := payload["max_purchase_limit"]; exists {
		value, err := productHookValueToInt(raw)
		if err != nil {
			return fmt.Errorf("decode max_purchase_limit: %w", err)
		}
		req.MaxPurchaseLimit = value
	}
	if raw, exists := payload["images"]; exists {
		value, err := productHookValueToImages(raw)
		if err != nil {
			return fmt.Errorf("decode images: %w", err)
		}
		req.Images = value
	}
	if raw, exists := payload["attributes"]; exists {
		value, err := productHookValueToAttributes(raw)
		if err != nil {
			return fmt.Errorf("decode attributes: %w", err)
		}
		req.Attributes = value
	}
	if raw, exists := payload["status"]; exists {
		value, err := productHookValueToProductStatus(raw)
		if err != nil {
			return fmt.Errorf("decode status: %w", err)
		}
		req.Status = value
	}
	if raw, exists := payload["sort_order"]; exists {
		value, err := productHookValueToInt(raw)
		if err != nil {
			return fmt.Errorf("decode sort_order: %w", err)
		}
		req.SortOrder = value
	}
	if raw, exists := payload["is_featured"]; exists {
		value, err := productHookValueToBool(raw)
		if err != nil {
			return fmt.Errorf("decode is_featured: %w", err)
		}
		req.IsFeatured = value
	}
	if raw, exists := payload["is_recommended"]; exists {
		value, err := productHookValueToBool(raw)
		if err != nil {
			return fmt.Errorf("decode is_recommended: %w", err)
		}
		req.IsRecommended = value
	}
	if raw, exists := payload["remark"]; exists {
		value, err := productHookValueToOptionalString(raw)
		if err != nil {
			return fmt.Errorf("decode remark: %w", err)
		}
		req.Remark = value
	}
	if raw, exists := payload["auto_delivery"]; exists {
		value, err := productHookValueToBool(raw)
		if err != nil {
			return fmt.Errorf("decode auto_delivery: %w", err)
		}
		req.AutoDelivery = value
	}

	return nil
}

func applyCreateProductHookPayload(req *CreateProductRequest, payload map[string]interface{}) error {
	if req == nil || payload == nil {
		return nil
	}

	patch := UpdateProductRequest{
		SKU:                req.SKU,
		Name:               req.Name,
		ProductCode:        req.ProductCode,
		ProductType:        req.ProductType,
		Description:        req.Description,
		ShortDescription:   req.ShortDescription,
		Category:           req.Category,
		Tags:               req.Tags,
		PriceMinor:         req.PriceMinor,
		OriginalPriceMinor: req.OriginalPriceMinor,
		Stock:              req.Stock,
		MaxPurchaseLimit:   req.MaxPurchaseLimit,
		Images:             req.Images,
		Attributes:         req.Attributes,
		Status:             req.Status,
		SortOrder:          req.SortOrder,
		IsFeatured:         req.IsFeatured,
		IsRecommended:      req.IsRecommended,
		Remark:             req.Remark,
		AutoDelivery:       req.AutoDelivery,
	}
	if err := applyUpdateProductHookPayload(&patch, payload); err != nil {
		return err
	}

	req.SKU = patch.SKU
	req.Name = patch.Name
	req.ProductCode = patch.ProductCode
	req.ProductType = patch.ProductType
	req.Description = patch.Description
	req.ShortDescription = patch.ShortDescription
	req.Category = patch.Category
	req.Tags = patch.Tags
	req.PriceMinor = patch.PriceMinor
	req.OriginalPriceMinor = patch.OriginalPriceMinor
	req.Stock = patch.Stock
	req.MaxPurchaseLimit = patch.MaxPurchaseLimit
	req.Images = patch.Images
	req.Attributes = patch.Attributes
	req.Status = patch.Status
	req.SortOrder = patch.SortOrder
	req.IsFeatured = patch.IsFeatured
	req.IsRecommended = patch.IsRecommended
	req.Remark = patch.Remark
	req.AutoDelivery = patch.AutoDelivery
	return nil
}

func applyDeleteProductHookPayload(options *service.DeleteProductOptions, payload map[string]interface{}) error {
	if options == nil || payload == nil {
		return nil
	}
	if raw, exists := payload["delete_images"]; exists {
		value, err := productHookValueToBool(raw)
		if err != nil {
			return fmt.Errorf("decode delete_images: %w", err)
		}
		options.DeleteImages = value
	}
	return nil
}

func parseProductListFilters(c *gin.Context) productListFilters {
	status := strings.TrimSpace(c.Query("status"))
	category := strings.TrimSpace(c.Query("category"))
	search := strings.TrimSpace(c.Query("search"))
	isFeaturedStr := strings.TrimSpace(c.Query("is_featured"))

	var isFeatured *bool
	if isFeaturedStr != "" {
		val := strings.EqualFold(isFeaturedStr, "true") || isFeaturedStr == "1"
		isFeatured = &val
	}

	return productListFilters{
		Status:     status,
		Category:   category,
		Search:     search,
		IsFeatured: isFeatured,
	}
}

// CreateProductRequest CreateProduct请求
type CreateProductRequest struct {
	SKU                string                    `json:"sku" binding:"required"`
	Name               string                    `json:"name" binding:"required"`
	ProductCode        string                    `json:"product_code"` // 产品码（用于生成防伪序列号）
	ProductType        models.ProductType        `json:"product_type"` // 商品类型：physical(实物) 或 virtual(虚拟)
	Description        string                    `json:"description"`
	ShortDescription   string                    `json:"short_description"`
	Category           string                    `json:"category"`
	Tags               []string                  `json:"tags"`
	PriceMinor         int64                     `json:"price_minor" binding:"gte=0"`
	OriginalPriceMinor int64                     `json:"original_price_minor"`
	Stock              int                       `json:"stock" binding:"gte=0"`
	MaxPurchaseLimit   int                       `json:"max_purchase_limit" binding:"gte=0"` // 购买限制
	Images             []models.ProductImage     `json:"images"`
	Attributes         []models.ProductAttribute `json:"attributes"`
	Status             models.ProductStatus      `json:"status"`
	SortOrder          int                       `json:"sort_order"`
	IsFeatured         bool                      `json:"is_featured"`
	IsRecommended      bool                      `json:"is_recommended"`
	Remark             string                    `json:"remark"`
	AutoDelivery       bool                      `json:"auto_delivery"` // 虚拟商品自动发货
}

// CreateProduct CreateProduct
func (h *ProductHandler) CreateProduct(c *gin.Context) {
	adminID, adminIDOK := middleware.RequireUserID(c)
	if !adminIDOK {
		return
	}
	if adminID == 0 {
		return
	}

	var req CreateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}
	hookExecCtx := h.buildProductHookExecutionContext(c, adminID, 0)
	if h.pluginManager != nil {
		hookPayload := map[string]interface{}{
			"admin_id":             adminID,
			"sku":                  req.SKU,
			"name":                 req.Name,
			"product_code":         req.ProductCode,
			"product_type":         req.ProductType,
			"description":          req.Description,
			"short_description":    req.ShortDescription,
			"category":             req.Category,
			"tags":                 req.Tags,
			"price_minor":          req.PriceMinor,
			"original_price_minor": req.OriginalPriceMinor,
			"stock":                req.Stock,
			"max_purchase_limit":   req.MaxPurchaseLimit,
			"status":               req.Status,
			"sort_order":           req.SortOrder,
			"is_featured":          req.IsFeatured,
			"is_recommended":       req.IsRecommended,
			"remark":               req.Remark,
			"auto_delivery":        req.AutoDelivery,
			"source":               "admin_api",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "product.create.before",
			Payload: hookPayload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("product.create.before hook execution failed: admin=%d sku=%s err=%v", adminID, req.SKU, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Product creation rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				originalReq := req
				if applyErr := applyCreateProductHookPayload(&req, hookResult.Payload); applyErr != nil {
					log.Printf("product.create.before payload apply failed, fallback to original request: admin=%d sku=%s err=%v", adminID, req.SKU, applyErr)
					req = originalReq
				}
			}
		}
	}

	product := &models.Product{
		SKU:              req.SKU,
		Name:             req.Name,
		ProductCode:      req.ProductCode,
		ProductType:      req.ProductType,
		Description:      req.Description,
		ShortDescription: req.ShortDescription,
		Category:         req.Category,
		Tags:             req.Tags,
		Price:            req.PriceMinor,
		OriginalPrice:    req.OriginalPriceMinor,
		Stock:            req.Stock,
		MaxPurchaseLimit: req.MaxPurchaseLimit,
		Images:           req.Images,
		Attributes:       req.Attributes,
		Status:           req.Status,
		SortOrder:        req.SortOrder,
		IsFeatured:       req.IsFeatured,
		IsRecommended:    req.IsRecommended,
		Remark:           req.Remark,
		AutoDelivery:     req.AutoDelivery,
	}

	if err := h.productService.CreateProduct(product); err != nil {
		if respondProductServiceError(c, err) {
			return
		}
		response.BadRequest(c, err.Error())
		return
	}

	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"admin_id":   adminID,
			"product_id": product.ID,
			"sku":        product.SKU,
			"name":       product.Name,
			"status":     product.Status,
			"stock":      product.Stock,
			"source":     "admin_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, aid uint, sku string) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "product.create.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("product.create.after hook execution failed: admin=%d sku=%s err=%v", aid, sku, hookErr)
			}
		}(h.buildProductHookExecutionContext(c, adminID, product.ID), afterPayload, adminID, product.SKU)
	}

	// 记录操作日志
	db := database.GetDB()
	logger.LogOperation(db, c, "create", "product", &product.ID, map[string]interface{}{
		"sku":  product.SKU,
		"name": product.Name,
	})

	response.Success(c, product)
}

// UpdateProductRequest UpdateProduct请求
type UpdateProductRequest struct {
	SKU                string                    `json:"sku"`
	Name               string                    `json:"name"`
	ProductCode        string                    `json:"product_code"` // 产品码（用于生成防伪序列号）
	ProductType        models.ProductType        `json:"product_type"` // 商品类型：physical(实物) 或 virtual(虚拟)
	Description        string                    `json:"description"`
	ShortDescription   string                    `json:"short_description"`
	Category           string                    `json:"category"`
	Tags               []string                  `json:"tags"`
	PriceMinor         int64                     `json:"price_minor"`
	OriginalPriceMinor int64                     `json:"original_price_minor"`
	Stock              int                       `json:"stock"`
	MaxPurchaseLimit   int                       `json:"max_purchase_limit"`
	Images             []models.ProductImage     `json:"images"`
	Attributes         []models.ProductAttribute `json:"attributes"`
	Status             models.ProductStatus      `json:"status"`
	SortOrder          int                       `json:"sort_order"`
	IsFeatured         bool                      `json:"is_featured"`
	IsRecommended      bool                      `json:"is_recommended"`
	Remark             string                    `json:"remark"`
	AutoDelivery       bool                      `json:"auto_delivery"` // 虚拟商品自动发货
}

// UpdateProduct UpdateProduct
func (h *ProductHandler) UpdateProduct(c *gin.Context) {
	adminID, adminIDOK := middleware.RequireUserID(c)
	if !adminIDOK {
		return
	}
	if adminID == 0 {
		return
	}

	productID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid product ID format")
		return
	}

	var req UpdateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}
	currentProduct, err := h.productService.GetProductByID(uint(productID), false)
	if err != nil {
		if respondProductServiceError(c, err) {
			return
		}
		response.InternalServerError(c, "Failed to load product", err)
		return
	}
	hookExecCtx := h.buildProductHookExecutionContext(c, adminID, currentProduct.ID)
	if h.pluginManager != nil {
		hookPayload := map[string]interface{}{
			"admin_id":             adminID,
			"product_id":           currentProduct.ID,
			"sku_before":           currentProduct.SKU,
			"name_before":          currentProduct.Name,
			"status_before":        currentProduct.Status,
			"stock_before":         currentProduct.Stock,
			"sku":                  req.SKU,
			"name":                 req.Name,
			"product_code":         req.ProductCode,
			"product_type":         req.ProductType,
			"description":          req.Description,
			"short_description":    req.ShortDescription,
			"category":             req.Category,
			"tags":                 req.Tags,
			"price_minor":          req.PriceMinor,
			"original_price_minor": req.OriginalPriceMinor,
			"stock":                req.Stock,
			"max_purchase_limit":   req.MaxPurchaseLimit,
			"status":               req.Status,
			"sort_order":           req.SortOrder,
			"is_featured":          req.IsFeatured,
			"is_recommended":       req.IsRecommended,
			"remark":               req.Remark,
			"auto_delivery":        req.AutoDelivery,
			"source":               "admin_api",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "product.update.before",
			Payload: hookPayload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("product.update.before hook execution failed: admin=%d product=%d err=%v", adminID, currentProduct.ID, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Product update rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				originalReq := req
				if applyErr := applyUpdateProductHookPayload(&req, hookResult.Payload); applyErr != nil {
					log.Printf("product.update.before payload apply failed, fallback to original request: admin=%d product=%d err=%v", adminID, currentProduct.ID, applyErr)
					req = originalReq
				}
			}
		}
	}

	updates := &models.Product{
		SKU:              req.SKU,
		Name:             req.Name,
		ProductCode:      req.ProductCode,
		ProductType:      req.ProductType,
		Description:      req.Description,
		ShortDescription: req.ShortDescription,
		Category:         req.Category,
		Tags:             req.Tags,
		Price:            req.PriceMinor,
		OriginalPrice:    req.OriginalPriceMinor,
		Stock:            req.Stock,
		MaxPurchaseLimit: req.MaxPurchaseLimit,
		Images:           req.Images,
		Attributes:       req.Attributes,
		Status:           req.Status,
		SortOrder:        req.SortOrder,
		IsFeatured:       req.IsFeatured,
		IsRecommended:    req.IsRecommended,
		Remark:           req.Remark,
		AutoDelivery:     req.AutoDelivery,
	}

	if err := h.productService.UpdateProduct(uint(productID), updates); err != nil {
		if respondProductServiceError(c, err) {
			return
		}
		response.BadRequest(c, err.Error())
		return
	}

	product, _ := h.productService.GetProductByID(uint(productID), false)
	if h.pluginManager != nil && product != nil {
		afterPayload := map[string]interface{}{
			"admin_id":      adminID,
			"product_id":    product.ID,
			"sku_before":    currentProduct.SKU,
			"sku_after":     product.SKU,
			"name_before":   currentProduct.Name,
			"name_after":    product.Name,
			"status_before": currentProduct.Status,
			"status_after":  product.Status,
			"stock_before":  currentProduct.Stock,
			"stock_after":   product.Stock,
			"source":        "admin_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, aid uint, pid uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "product.update.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("product.update.after hook execution failed: admin=%d product=%d err=%v", aid, pid, hookErr)
			}
		}(hookExecCtx, afterPayload, adminID, product.ID)
	}

	// 记录操作日志
	db := database.GetDB()
	logger.LogOperation(db, c, "update", "product", &product.ID, map[string]interface{}{
		"sku":  product.SKU,
		"name": product.Name,
	})

	response.Success(c, product)
}

// DeleteProduct DeleteProduct
func (h *ProductHandler) DeleteProduct(c *gin.Context) {
	adminID, adminIDOK := middleware.RequireUserID(c)
	if !adminIDOK {
		return
	}
	if adminID == 0 {
		return
	}

	productID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid product ID format")
		return
	}

	product, _ := h.productService.GetProductByID(uint(productID), false)
	if product == nil {
		response.NotFound(c, service.ErrProductNotFound.Error())
		return
	}
	hookExecCtx := h.buildProductHookExecutionContext(c, adminID, product.ID)
	deleteOptions := service.DeleteProductOptions{DeleteImages: true}
	if h.pluginManager != nil {
		originalOptions := deleteOptions
		hookPayload := map[string]interface{}{
			"admin_id":      adminID,
			"product_id":    product.ID,
			"sku":           product.SKU,
			"name":          product.Name,
			"status":        product.Status,
			"stock":         product.Stock,
			"delete_images": deleteOptions.DeleteImages,
			"source":        "admin_api",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "product.delete.before",
			Payload: hookPayload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("product.delete.before hook execution failed: admin=%d product=%d err=%v", adminID, product.ID, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Product deletion rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if applyErr := applyDeleteProductHookPayload(&deleteOptions, hookResult.Payload); applyErr != nil {
					log.Printf("product.delete.before payload apply failed, fallback to original options: admin=%d product=%d err=%v", adminID, product.ID, applyErr)
					deleteOptions = originalOptions
				}
			}
		}
	}

	if err := h.productService.DeleteProductWithOptions(uint(productID), deleteOptions); err != nil {
		if respondProductServiceError(c, err) {
			return
		}
		response.BadRequest(c, err.Error())
		return
	}

	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"admin_id":      adminID,
			"product_id":    product.ID,
			"sku":           product.SKU,
			"name":          product.Name,
			"status":        product.Status,
			"stock":         product.Stock,
			"delete_images": deleteOptions.DeleteImages,
			"source":        "admin_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, aid uint, pid uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "product.delete.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("product.delete.after hook execution failed: admin=%d product=%d err=%v", aid, pid, hookErr)
			}
		}(hookExecCtx, afterPayload, adminID, product.ID)
	}

	// 记录操作日志
	db := database.GetDB()
	logger.LogOperation(db, c, "delete", "product", &product.ID, map[string]interface{}{
		"sku":  product.SKU,
		"name": product.Name,
	})

	response.Success(c, gin.H{"message": "Product deleted"})
}

// GetProduct - Get product details (simplified version, bindings don't include inventory details)
func (h *ProductHandler) GetProduct(c *gin.Context) {
	productID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid product ID format")
		return
	}

	product, err := h.productService.GetProductByID(uint(productID), false)
	if err != nil {
		if respondProductServiceError(c, err) {
			return
		}
		response.InternalServerError(c, "Failed to load product", err)
		return
	}

	// 构建响应：ProductInfo + 简化的绑定关系
	productResponse := map[string]interface{}{
		"id":                   product.ID,
		"sku":                  product.SKU,
		"name":                 product.Name,
		"product_code":         product.ProductCode,
		"product_type":         product.ProductType,
		"description":          product.Description,
		"short_description":    product.ShortDescription,
		"category":             product.Category,
		"tags":                 product.Tags,
		"price_minor":          product.Price,
		"original_price_minor": product.OriginalPrice,
		"stock":                product.Stock,
		"max_purchase_limit":   product.MaxPurchaseLimit,
		"images":               product.Images,
		"attributes":           product.Attributes,
		"status":               product.Status,
		"sort_order":           product.SortOrder,
		"is_featured":          product.IsFeatured,
		"is_recommended":       product.IsRecommended,
		"remark":               product.Remark,
		"auto_delivery":        product.AutoDelivery,
		"inventory_mode":       product.InventoryMode,
		"view_count":           product.ViewCount,
		"sale_count":           product.SaleCount,
		"created_at":           product.CreatedAt,
		"updated_at":           product.UpdatedAt,
	}

	// 简化的绑定关系：只返回必要的映射Info，不包含完整的Inventory对象
	if len(product.InventoryBindings) > 0 {
		bindings := make([]map[string]interface{}, 0, len(product.InventoryBindings))
		for _, binding := range product.InventoryBindings {
			bindingData := map[string]interface{}{
				"id":              binding.ID,
				"inventory_id":    binding.InventoryID,
				"attributes":      binding.Attributes, // 直接返回规格组合JSON
				"attributes_hash": binding.AttributesHash,
				"is_random":       binding.IsRandom,
				"priority":        binding.Priority,
				"notes":           binding.Notes, // 真正的备注
				"created_at":      binding.CreatedAt,
				"updated_at":      binding.UpdatedAt,
			}
			bindings = append(bindings, bindingData)
		}
		productResponse["inventory_bindings"] = bindings
	} else {
		productResponse["inventory_bindings"] = []interface{}{}
	}

	// 获取虚拟库存绑定（对于虚拟商品）
	if product.ProductType == models.ProductTypeVirtual {
		virtualBindings, err := h.virtualInventoryService.GetProductBindings(uint(productID))
		if err == nil && len(virtualBindings) > 0 {
			vBindings := make([]map[string]interface{}, 0, len(virtualBindings))
			for _, binding := range virtualBindings {
				vBindingData := map[string]interface{}{
					"id":                   binding.ID,
					"virtual_inventory_id": binding.VirtualInventoryID,
					"attributes":           binding.Attributes,
					"attributes_hash":      binding.AttributesHash,
					"is_random":            binding.IsRandom,
					"priority":             binding.Priority,
					"notes":                binding.Notes,
					"created_at":           binding.CreatedAt,
				}
				vBindings = append(vBindings, vBindingData)
			}
			productResponse["virtual_inventory_bindings"] = vBindings
		} else {
			productResponse["virtual_inventory_bindings"] = []interface{}{}
		}
	} else {
		productResponse["virtual_inventory_bindings"] = []interface{}{}
	}

	response.Success(c, productResponse)
}

// ListProducts Product列表
func (h *ProductHandler) ListProducts(c *gin.Context) {
	page, limit := response.GetPagination(c)
	filters := parseProductListFilters(c)

	products, total, err := h.productService.ListProducts(
		page,
		limit,
		filters.Status,
		filters.Category,
		filters.Search,
		filters.IsFeatured,
		nil,
		false,
	)
	if err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	response.Paginated(c, products, page, limit, total)
}

// ExportProducts 导出商品列表
func (h *ProductHandler) ExportProducts(c *gin.Context) {
	filters := parseProductListFilters(c)

	products, total, err := h.productService.ListProducts(
		1,
		adminCSVExportMaxRows+1,
		filters.Status,
		filters.Category,
		filters.Search,
		filters.IsFeatured,
		nil,
		false,
	)
	if err != nil {
		response.InternalError(c, "Query failed")
		return
	}
	if total > int64(adminCSVExportMaxRows) {
		response.BadRequest(c, fmt.Sprintf("Too many records to export (max %d). Please narrow the filters.", adminCSVExportMaxRows))
		return
	}

	db := database.GetDB()
	productIDs := make([]uint, 0, len(products))
	for _, product := range products {
		productIDs = append(productIDs, product.ID)
	}

	physicalBindingsByProductID := make(map[uint][]models.ProductInventoryBinding, len(productIDs))
	virtualBindingsByProductID := make(map[uint][]models.ProductVirtualInventoryBinding, len(productIDs))

	if db != nil && len(productIDs) > 0 {
		var physicalBindings []models.ProductInventoryBinding
		if err := db.Where("product_id IN ?", productIDs).
			Preload("Inventory").
			Order("created_at ASC").
			Find(&physicalBindings).Error; err != nil {
			response.InternalError(c, "Query failed")
			return
		}
		for _, binding := range physicalBindings {
			physicalBindingsByProductID[binding.ProductID] = append(physicalBindingsByProductID[binding.ProductID], binding)
		}

		var virtualBindings []models.ProductVirtualInventoryBinding
		if err := db.Where("product_id IN ?", productIDs).
			Preload("VirtualInventory").
			Order("created_at ASC").
			Find(&virtualBindings).Error; err != nil {
			response.InternalError(c, "Query failed")
			return
		}
		for _, binding := range virtualBindings {
			virtualBindingsByProductID[binding.ProductID] = append(virtualBindingsByProductID[binding.ProductID], binding)
		}
	}

	rows := make([][]string, 0, len(products))
	for _, product := range products {
		rows = append(rows, []string{
			strconv.FormatUint(uint64(product.ID), 10),
			product.SKU,
			product.Name,
			product.ProductCode,
			string(product.ProductType),
			product.Category,
			csvJSONValue(product.Tags),
			strconv.FormatInt(product.Price, 10),
			strconv.FormatInt(product.OriginalPrice, 10),
			strconv.Itoa(product.Stock),
			strconv.Itoa(product.MaxPurchaseLimit),
			string(product.Status),
			strconv.Itoa(product.SortOrder),
			strconv.FormatBool(product.IsFeatured),
			strconv.FormatBool(product.IsRecommended),
			product.InventoryMode,
			strconv.FormatBool(product.AutoDelivery),
			strconv.Itoa(product.ViewCount),
			strconv.Itoa(product.SaleCount),
			product.ShortDescription,
			product.Description,
			product.Remark,
			csvJSONValue(product.Images),
			csvJSONValue(product.Attributes),
			csvJSONValue(buildProductExportPhysicalBindings(physicalBindingsByProductID[product.ID])),
			csvJSONValue(buildProductExportVirtualBindings(virtualBindingsByProductID[product.ID])),
			csvTimeValue(product.CreatedAt),
			csvTimeValue(product.UpdatedAt),
		})
	}

	logPayload := map[string]interface{}{
		"count":    len(rows),
		"status":   filters.Status,
		"category": filters.Category,
		"search":   filters.Search,
		"format":   "xlsx",
	}
	if filters.IsFeatured != nil {
		logPayload["is_featured"] = *filters.IsFeatured
	}
	logger.LogOperation(db, c, "export", "product", nil, logPayload)

	writeXLSXAttachment(c, buildAdminXLSXFileName("products"), "Products", buildProductCSVHeaders(), rows)
}

// UpdateStatusRequest Update状态请求
type UpdateStatusRequest struct {
	Status models.ProductStatus `json:"status" binding:"required"`
}

// UpdateProductStatus UpdateProduct状态
func (h *ProductHandler) UpdateProductStatus(c *gin.Context) {
	adminID, adminIDOK := middleware.RequireUserID(c)
	if !adminIDOK {
		return
	}
	if adminID == 0 {
		return
	}

	productID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid product ID format")
		return
	}

	var req UpdateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	product, err := h.productService.GetProductByID(uint(productID), false)
	if err != nil {
		if respondProductServiceError(c, err) {
			return
		}
		response.InternalServerError(c, "Failed to load product", err)
		return
	}
	beforeStatus := product.Status
	hookExecCtx := h.buildProductHookExecutionContext(c, adminID, product.ID)
	if h.pluginManager != nil {
		originalReq := req
		hookPayload := map[string]interface{}{
			"admin_id":      adminID,
			"product_id":    product.ID,
			"sku":           product.SKU,
			"status_before": beforeStatus,
			"status_after":  req.Status,
			"source":        "admin_api",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "product.status.update.before",
			Payload: hookPayload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("product.status.update.before hook execution failed: admin=%d product=%d err=%v", adminID, product.ID, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Product status update rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if rawStatus, exists := hookResult.Payload["status"]; exists {
					status, convErr := productHookValueToProductStatus(rawStatus)
					if convErr != nil {
						log.Printf("product.status.update.before payload status decode failed, fallback to original request: admin=%d product=%d err=%v", adminID, product.ID, convErr)
						req = originalReq
					} else {
						req.Status = status
					}
				}
			}
		}
	}

	if err := h.productService.UpdateProductStatus(uint(productID), req.Status); err != nil {
		if respondProductServiceError(c, err) {
			return
		}
		response.BadRequest(c, err.Error())
		return
	}

	product, _ = h.productService.GetProductByID(uint(productID), false)
	if h.pluginManager != nil && product != nil {
		afterPayload := map[string]interface{}{
			"admin_id":      adminID,
			"product_id":    product.ID,
			"sku":           product.SKU,
			"status_before": beforeStatus,
			"status_after":  product.Status,
			"source":        "admin_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, aid uint, pid uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "product.status.update.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("product.status.update.after hook execution failed: admin=%d product=%d err=%v", aid, pid, hookErr)
			}
		}(hookExecCtx, afterPayload, adminID, product.ID)
	}

	response.Success(c, product)
}

// UpdateStockRequest UpdateInventory请求
type UpdateStockRequest struct {
	Stock int `json:"stock" binding:"gte=0"`
}

// UpdateStock UpdateInventory
func (h *ProductHandler) UpdateStock(c *gin.Context) {
	productID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid product ID format")
		return
	}

	var req UpdateStockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	if err := h.productService.UpdateStock(uint(productID), req.Stock); err != nil {
		if respondProductServiceError(c, err) {
			return
		}
		response.BadRequest(c, err.Error())
		return
	}

	product, _ := h.productService.GetProductByID(uint(productID), false)
	response.Success(c, product)
}

// ToggleFeatured 切换精选状态
func (h *ProductHandler) ToggleFeatured(c *gin.Context) {
	productID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid product ID format")
		return
	}

	if err := h.productService.ToggleFeatured(uint(productID)); err != nil {
		if respondProductServiceError(c, err) {
			return
		}
		response.BadRequest(c, err.Error())
		return
	}

	product, _ := h.productService.GetProductByID(uint(productID), false)
	response.Success(c, product)
}

// GetCategories get所有分类
func (h *ProductHandler) GetCategories(c *gin.Context) {
	categories, err := h.productService.GetCategories()
	if err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	response.Success(c, gin.H{"categories": categories})
}

// UpdateInventoryModeRequest UpdateInventory模式请求
type UpdateInventoryModeRequest struct {
	InventoryMode string `json:"inventory_mode" binding:"required,oneof=fixed random"`
}

// UpdateInventoryMode UpdateProductInventory模式
func (h *ProductHandler) UpdateInventoryMode(c *gin.Context) {
	adminID, adminIDOK := middleware.RequireUserID(c)
	if !adminIDOK {
		return
	}
	if adminID == 0 {
		return
	}

	productID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid product ID format")
		return
	}

	var req UpdateInventoryModeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	product, err := h.productService.GetProductByID(uint(productID), false)
	if err != nil {
		if respondProductServiceError(c, err) {
			return
		}
		response.InternalServerError(c, "Failed to load product", err)
		return
	}
	beforeMode := product.InventoryMode
	hookExecCtx := h.buildProductHookExecutionContext(c, adminID, product.ID)
	if h.pluginManager != nil {
		originalReq := req
		hookPayload := map[string]interface{}{
			"admin_id":              adminID,
			"product_id":            product.ID,
			"sku":                   product.SKU,
			"inventory_mode_before": beforeMode,
			"inventory_mode_after":  req.InventoryMode,
			"source":                "admin_api",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "product.inventory_mode.update.before",
			Payload: hookPayload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("product.inventory_mode.update.before hook execution failed: admin=%d product=%d err=%v", adminID, product.ID, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Product inventory mode update rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if rawMode, exists := hookResult.Payload["inventory_mode"]; exists {
					mode, convErr := productHookValueToInventoryMode(rawMode)
					if convErr != nil {
						log.Printf("product.inventory_mode.update.before payload inventory_mode decode failed, fallback to original request: admin=%d product=%d err=%v", adminID, product.ID, convErr)
						req = originalReq
					} else {
						req.InventoryMode = mode
					}
				}
			}
		}
	}

	if err := h.productService.UpdateInventoryMode(uint(productID), req.InventoryMode); err != nil {
		if respondProductServiceError(c, err) {
			return
		}
		response.BadRequest(c, err.Error())
		return
	}

	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"admin_id":              adminID,
			"product_id":            product.ID,
			"sku":                   product.SKU,
			"inventory_mode_before": beforeMode,
			"inventory_mode_after":  req.InventoryMode,
			"source":                "admin_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, aid uint, pid uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "product.inventory_mode.update.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("product.inventory_mode.update.after hook execution failed: admin=%d product=%d err=%v", aid, pid, hookErr)
			}
		}(hookExecCtx, afterPayload, adminID, product.ID)
	}

	response.Success(c, gin.H{"message": "Inventory mode updated", "inventory_mode": req.InventoryMode})
}
