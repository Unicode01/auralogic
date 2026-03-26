package user

import (
	"encoding/json"
	"log"
	"strconv"

	"auralogic/internal/middleware"
	"auralogic/internal/models"
	"auralogic/internal/pkg/response"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
)

type ProductHandler struct {
	productService          *service.ProductService
	orderService            *service.OrderService
	bindingService          *service.BindingService
	virtualInventoryService *service.VirtualInventoryService
	pluginManager           *service.PluginManagerService
}

func NewProductHandler(
	productService *service.ProductService,
	orderService *service.OrderService,
	bindingService *service.BindingService,
	virtualInventoryService *service.VirtualInventoryService,
	pluginManager *service.PluginManagerService,
) *ProductHandler {
	return &ProductHandler{
		productService:          productService,
		orderService:            orderService,
		bindingService:          bindingService,
		virtualInventoryService: virtualInventoryService,
		pluginManager:           pluginManager,
	}
}

func productHookOptionalBoolValue(value *bool) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

func buildUserProductHookPayload(product *models.Product) map[string]interface{} {
	if product == nil {
		return map[string]interface{}{}
	}

	return map[string]interface{}{
		"product_id":           product.ID,
		"sku":                  product.SKU,
		"name":                 product.Name,
		"product_code":         product.ProductCode,
		"product_type":         product.ProductType,
		"category":             product.Category,
		"tags":                 product.Tags,
		"price_minor":          product.Price,
		"original_price_minor": product.OriginalPrice,
		"stock":                product.Stock,
		"status":               product.Status,
		"sort_order":           product.SortOrder,
		"is_featured":          product.IsFeatured,
		"is_recommended":       product.IsRecommended,
		"view_count":           product.ViewCount,
		"sale_count":           product.SaleCount,
		"inventory_mode":       product.InventoryMode,
		"auto_delivery":        product.AutoDelivery,
		"max_purchase_limit":   product.MaxPurchaseLimit,
		"created_at":           product.CreatedAt,
		"updated_at":           product.UpdatedAt,
	}
}

func buildUserProductHookSummaries(products []models.Product) []map[string]interface{} {
	if len(products) == 0 {
		return []map[string]interface{}{}
	}

	items := make([]map[string]interface{}, 0, len(products))
	for i := range products {
		items = append(items, buildUserProductHookPayload(&products[i]))
	}
	return items
}

func executeProductListReadOnlyBeforeHook(
	pluginManager *service.PluginManagerService,
	c *gin.Context,
	userID *uint,
	page int,
	limit int,
	category string,
	search string,
	isFeatured *bool,
	isRecommended *bool,
	listKind string,
) {
	if pluginManager == nil {
		return
	}

	payload := map[string]interface{}{
		"user_id":        userID,
		"authenticated":  userID != nil,
		"page":           page,
		"limit":          limit,
		"status":         string(models.ProductStatusActive),
		"category":       category,
		"search":         search,
		"is_featured":    productHookOptionalBoolValue(isFeatured),
		"is_recommended": productHookOptionalBoolValue(isRecommended),
		"list_kind":      listKind,
		"source":         "user_api",
	}
	_, hookErr := pluginManager.ExecuteHook(service.HookExecutionRequest{
		Hook:    "product.list.query.before",
		Payload: payload,
	}, buildOptionalUserHookExecutionContext(c, userID, map[string]string{
		"hook_resource": "product",
		"hook_source":   "user_api",
		"hook_action":   "list_query",
		"list_kind":     listKind,
	}))
	if hookErr != nil {
		uid := uint(0)
		if userID != nil {
			uid = *userID
		}
		log.Printf("product.list.query.before hook execution failed: user=%d list_kind=%s err=%v", uid, listKind, hookErr)
	}
}

func emitProductListReadOnlyAfterHook(
	pluginManager *service.PluginManagerService,
	c *gin.Context,
	userID *uint,
	page int,
	limit int,
	category string,
	search string,
	isFeatured *bool,
	isRecommended *bool,
	listKind string,
	products []models.Product,
	total int64,
) {
	if pluginManager == nil {
		return
	}

	payload := map[string]interface{}{
		"user_id":        userID,
		"authenticated":  userID != nil,
		"page":           page,
		"limit":          limit,
		"status":         string(models.ProductStatusActive),
		"category":       category,
		"search":         search,
		"is_featured":    productHookOptionalBoolValue(isFeatured),
		"is_recommended": productHookOptionalBoolValue(isRecommended),
		"list_kind":      listKind,
		"result_count":   len(products),
		"total":          total,
		"products":       buildUserProductHookSummaries(products),
		"source":         "user_api",
	}
	go func(execCtx *service.ExecutionContext, hookPayload map[string]interface{}, count int) {
		_, hookErr := pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "product.list.query.after",
			Payload: hookPayload,
		}, execCtx)
		if hookErr != nil {
			uid := uint(0)
			if userID != nil {
				uid = *userID
			}
			log.Printf("product.list.query.after hook execution failed: user=%d list_kind=%s count=%d err=%v", uid, listKind, count, hookErr)
		}
	}(cloneUserHookExecutionContext(buildOptionalUserHookExecutionContext(c, userID, map[string]string{
		"hook_resource": "product",
		"hook_source":   "user_api",
		"hook_action":   "list_query",
		"list_kind":     listKind,
	})), payload, len(products))
}

// ListProducts Product列表（User端，仅显示上架Product）
func (h *ProductHandler) ListProducts(c *gin.Context) {
	page, limit := response.GetPagination(c)
	category := c.Query("category")
	search := c.Query("search")
	isFeaturedStr := c.Query("is_featured")
	userID, hasUser := middleware.GetUserID(c)
	var optionalUserID *uint
	if hasUser {
		optionalUserID = &userID
	}

	var isFeatured *bool
	if isFeaturedStr != "" {
		val := isFeaturedStr == "true"
		isFeatured = &val
	}
	executeProductListReadOnlyBeforeHook(h.pluginManager, c, optionalUserID, page, limit, category, search, isFeatured, nil, "catalog")

	// User端只显示上架Product
	products, total, err := h.productService.ListProducts(page, limit, string(models.ProductStatusActive), category, search, isFeatured, nil, true)
	if err != nil {
		response.InternalError(c, "Query failed")
		return
	}
	emitProductListReadOnlyAfterHook(h.pluginManager, c, optionalUserID, page, limit, category, search, isFeatured, nil, "catalog", products, total)

	response.Paginated(c, products, page, limit, total)
}

// GetProduct getProduct详情（User端）
func (h *ProductHandler) GetProduct(c *gin.Context) {
	productID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid product ID format")
		return
	}
	userID, hasUser := middleware.GetUserID(c)
	var optionalUserID *uint
	if hasUser {
		optionalUserID = &userID
	}

	product, err := h.productService.GetProductByID(uint(productID), true) // 增加浏览次数
	if err != nil {
		response.NotFound(c, "Product not found")
		return
	}

	// 只返回上架的Product
	if product.Status != models.ProductStatusActive {
		response.NotFound(c, "Product not found")
		return
	}
	if h.pluginManager != nil {
		payload := buildUserProductHookPayload(product)
		payload["user_id"] = optionalUserID
		payload["authenticated"] = optionalUserID != nil
		payload["view_source"] = "detail"
		payload["source"] = "user_api"
		go func(execCtx *service.ExecutionContext, hookPayload map[string]interface{}, pid uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "product.view.after",
				Payload: hookPayload,
			}, execCtx)
			if hookErr != nil {
				uid := uint(0)
				if optionalUserID != nil {
					uid = *optionalUserID
				}
				log.Printf("product.view.after hook execution failed: user=%d product=%d err=%v", uid, pid, hookErr)
			}
		}(cloneUserHookExecutionContext(buildOptionalUserHookExecutionContext(c, optionalUserID, map[string]string{
			"hook_resource": "product",
			"hook_source":   "user_api",
			"hook_action":   "view",
			"product_id":    strconv.FormatUint(productID, 10),
		})), payload, product.ID)
	}

	response.Success(c, product)
}

// GetProductAvailableStock getProduct的可用Inventory总数
func (h *ProductHandler) GetProductAvailableStock(c *gin.Context) {
	productID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid product ID format")
		return
	}

	// getProductInfo
	product, err := h.productService.GetProductByID(uint(productID), false)
	if err != nil {
		response.NotFound(c, "Product not found")
		return
	}

	// 只处理上架的Product
	if product.Status != models.ProductStatusActive {
		response.NotFound(c, "Product not found")
		return
	}

	// 解析查询参数中的属性
	attributesParam := c.Query("attributes")
	var attributes map[string]string
	if attributesParam != "" {
		if err := json.Unmarshal([]byte(attributesParam), &attributes); err != nil {
			// 忽略解析错误，使用总库存
			attributes = nil
		}
	}

	// 剔除盲盒属性，防止用户通过库存变化量反推盲盒结果
	if len(attributes) > 0 {
		for _, attr := range product.Attributes {
			if attr.Mode == models.AttributeModeBlindBox {
				delete(attributes, attr.Name)
			}
		}
	}

	var totalStock int
	isUnlimited := false
	// 根据商品类型获取不同的库存
	if product.ProductType == models.ProductTypeVirtual {
		// 虚拟商品：从虚拟库存绑定中获取可用库存
		var stock int64
		var unlimited bool
		if len(attributes) > 0 {
			// 根据规格属性查询
			stock, err = h.virtualInventoryService.GetAvailableCountForProductByAttributes(uint(productID), attributes)
			if err == nil {
				unlimited, _ = h.virtualInventoryService.HasUnlimitedScriptInventoryForProductByAttributes(uint(productID), attributes)
			}
		} else {
			// 无属性，查询总库存
			stock, err = h.virtualInventoryService.GetAvailableCountForProduct(uint(productID))
			if err == nil {
				unlimited, _ = h.virtualInventoryService.HasUnlimitedScriptInventoryForProduct(uint(productID))
			}
		}
		if err != nil {
			response.InternalError(c, "QueryInventoryFailed")
			return
		}
		totalStock = int(stock)
		isUnlimited = unlimited
	} else {
		// 实体商品：从实体库存绑定中获取可用库存
		var stock int
		if len(attributes) > 0 {
			// 根据规格属性查询
			stock, err = h.bindingService.GetAvailableStockByAttributes(uint(productID), attributes)
		} else {
			// 无属性，查询总库存
			stock, err = h.bindingService.GetTotalAvailableStock(uint(productID))
		}
		if err != nil {
			response.InternalError(c, "QueryInventoryFailed")
			return
		}
		totalStock = stock
	}

	response.Success(c, gin.H{
		"product_id":      productID,
		"available_stock": totalStock,
		"is_unlimited":    isUnlimited,
	})
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

// GetFeaturedProducts get精选Product
func (h *ProductHandler) GetFeaturedProducts(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit > 50 {
		limit = 50
	}
	userID, hasUser := middleware.GetUserID(c)
	var optionalUserID *uint
	if hasUser {
		optionalUserID = &userID
	}

	isFeatured := true
	executeProductListReadOnlyBeforeHook(h.pluginManager, c, optionalUserID, 1, limit, "", "", &isFeatured, nil, "featured")
	products, total, err := h.productService.ListProducts(1, limit, string(models.ProductStatusActive), "", "", &isFeatured, nil, true)
	if err != nil {
		response.InternalError(c, "Query failed")
		return
	}
	emitProductListReadOnlyAfterHook(h.pluginManager, c, optionalUserID, 1, limit, "", "", &isFeatured, nil, "featured", products, total)

	response.Success(c, gin.H{"products": products})
}

// GetRecommendedProducts get推荐Product
func (h *ProductHandler) GetRecommendedProducts(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit > 50 {
		limit = 50
	}
	userID, hasUser := middleware.GetUserID(c)
	var optionalUserID *uint
	if hasUser {
		optionalUserID = &userID
	}

	isRecommended := true
	executeProductListReadOnlyBeforeHook(h.pluginManager, c, optionalUserID, 1, limit, "", "", nil, &isRecommended, "recommended")
	products, total, err := h.productService.ListProducts(1, limit, string(models.ProductStatusActive), "", "", nil, &isRecommended, true)
	if err != nil {
		response.InternalError(c, "Query failed")
		return
	}
	emitProductListReadOnlyAfterHook(h.pluginManager, c, optionalUserID, 1, limit, "", "", nil, &isRecommended, "recommended", products, total)

	response.Success(c, gin.H{"products": products})
}
