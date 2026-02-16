package admin

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"auralogic/internal/database"
	"auralogic/internal/models"
	"auralogic/internal/pkg/logger"
	"auralogic/internal/pkg/response"
	"auralogic/internal/service"
)

type ProductHandler struct {
	productService          *service.ProductService
	virtualInventoryService *service.VirtualInventoryService
}

func NewProductHandler(productService *service.ProductService, virtualInventoryService *service.VirtualInventoryService) *ProductHandler {
	return &ProductHandler{
		productService:          productService,
		virtualInventoryService: virtualInventoryService,
	}
}

// CreateProductRequest CreateProduct请求
type CreateProductRequest struct {
	SKU              string                    `json:"sku" binding:"required"`
	Name             string                    `json:"name" binding:"required"`
	ProductCode      string                    `json:"product_code"` // 产品码（用于生成防伪序列号）
	ProductType      models.ProductType        `json:"product_type"` // 商品类型：physical(实物) 或 virtual(虚拟)
	Description      string                    `json:"description"`
	ShortDescription string                    `json:"short_description"`
	Category         string                    `json:"category"`
	Tags             []string                  `json:"tags"`
	Price            float64                   `json:"price" binding:"gte=0"`
	OriginalPrice    float64                   `json:"original_price"`
	Stock            int                       `json:"stock" binding:"gte=0"`
	MaxPurchaseLimit int                       `json:"max_purchase_limit" binding:"gte=0"` // 购买限制
	Images           []models.ProductImage     `json:"images"`
	Attributes       []models.ProductAttribute `json:"attributes"`
	Status           models.ProductStatus      `json:"status"`
	SortOrder        int                       `json:"sort_order"`
	IsFeatured       bool                      `json:"is_featured"`
	IsRecommended    bool                      `json:"is_recommended"`
	Remark           string                    `json:"remark"`
	AutoDelivery     bool                      `json:"auto_delivery"` // 虚拟商品自动发货
}

// CreateProduct CreateProduct
func (h *ProductHandler) CreateProduct(c *gin.Context) {
	var req CreateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
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
		Price:            req.Price,
		OriginalPrice:    req.OriginalPrice,
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
		response.BadRequest(c, err.Error())
		return
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
	SKU              string                    `json:"sku"`
	Name             string                    `json:"name"`
	ProductCode      string                    `json:"product_code"` // 产品码（用于生成防伪序列号）
	ProductType      models.ProductType        `json:"product_type"` // 商品类型：physical(实物) 或 virtual(虚拟)
	Description      string                    `json:"description"`
	ShortDescription string                    `json:"short_description"`
	Category         string                    `json:"category"`
	Tags             []string                  `json:"tags"`
	Price            float64                   `json:"price"`
	OriginalPrice    float64                   `json:"original_price"`
	Stock            int                       `json:"stock"`
	MaxPurchaseLimit int                       `json:"max_purchase_limit"`
	Images           []models.ProductImage     `json:"images"`
	Attributes       []models.ProductAttribute `json:"attributes"`
	Status           models.ProductStatus      `json:"status"`
	SortOrder        int                       `json:"sort_order"`
	IsFeatured       bool                      `json:"is_featured"`
	IsRecommended    bool                      `json:"is_recommended"`
	Remark           string                    `json:"remark"`
	AutoDelivery     bool                      `json:"auto_delivery"` // 虚拟商品自动发货
}

// UpdateProduct UpdateProduct
func (h *ProductHandler) UpdateProduct(c *gin.Context) {
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

	updates := &models.Product{
		SKU:              req.SKU,
		Name:             req.Name,
		ProductCode:      req.ProductCode,
		ProductType:      req.ProductType,
		Description:      req.Description,
		ShortDescription: req.ShortDescription,
		Category:         req.Category,
		Tags:             req.Tags,
		Price:            req.Price,
		OriginalPrice:    req.OriginalPrice,
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
		response.BadRequest(c, err.Error())
		return
	}

	product, _ := h.productService.GetProductByID(uint(productID), false)

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
	productID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid product ID format")
		return
	}

	product, _ := h.productService.GetProductByID(uint(productID), false)

	if err := h.productService.DeleteProduct(uint(productID)); err != nil {
		response.BadRequest(c, err.Error())
		return
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
		response.NotFound(c, "Product not found")
		return
	}

	// 构建响应：ProductInfo + 简化的绑定关系
	productResponse := map[string]interface{}{
		"id":                 product.ID,
		"sku":                product.SKU,
		"name":               product.Name,
		"product_code":       product.ProductCode,
		"product_type":       product.ProductType,
		"description":        product.Description,
		"short_description":  product.ShortDescription,
		"category":           product.Category,
		"tags":               product.Tags,
		"price":              product.Price,
		"original_price":     product.OriginalPrice,
		"stock":              product.Stock,
		"max_purchase_limit": product.MaxPurchaseLimit,
		"images":             product.Images,
		"attributes":         product.Attributes,
		"status":             product.Status,
		"sort_order":         product.SortOrder,
		"is_featured":        product.IsFeatured,
		"is_recommended":     product.IsRecommended,
		"remark":             product.Remark,
		"auto_delivery":      product.AutoDelivery,
		"inventory_mode":     product.InventoryMode,
		"view_count":         product.ViewCount,
		"sale_count":         product.SaleCount,
		"created_at":         product.CreatedAt,
		"updated_at":         product.UpdatedAt,
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
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	status := c.Query("status")
	category := c.Query("category")
	search := c.Query("search")
	isFeaturedStr := c.Query("is_featured")

	if limit > 100 {
		limit = 100
	}

	var isFeatured *bool
	if isFeaturedStr != "" {
		val := isFeaturedStr == "true"
		isFeatured = &val
	}

	products, total, err := h.productService.ListProducts(page, limit, status, category, search, isFeatured, nil, false)
	if err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	response.Paginated(c, products, page, limit, total)
}

// UpdateStatusRequest Update状态请求
type UpdateStatusRequest struct {
	Status models.ProductStatus `json:"status" binding:"required"`
}

// UpdateProductStatus UpdateProduct状态
func (h *ProductHandler) UpdateProductStatus(c *gin.Context) {
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

	if err := h.productService.UpdateProductStatus(uint(productID), req.Status); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	product, _ := h.productService.GetProductByID(uint(productID), false)
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
	productID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid product ID format")
		return
	}

	var req UpdateInventoryModeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters: "+err.Error())
		return
	}

	product, err := h.productService.GetProductByID(uint(productID), false)
	if err != nil {
		response.NotFound(c, "Product not found")
		return
	}

	product.InventoryMode = req.InventoryMode
	if err := h.productService.UpdateProduct(uint(productID), product); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, gin.H{"message": "Inventory mode updated", "inventory_mode": req.InventoryMode})
}
