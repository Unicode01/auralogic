package user

import (
	"encoding/json"
	"strconv"

	"github.com/gin-gonic/gin"
	"auralogic/internal/models"
	"auralogic/internal/pkg/response"
	"auralogic/internal/service"
)

type ProductHandler struct {
	productService          *service.ProductService
	orderService            *service.OrderService
	bindingService          *service.BindingService
	virtualInventoryService *service.VirtualInventoryService
}

func NewProductHandler(
	productService *service.ProductService,
	orderService *service.OrderService,
	bindingService *service.BindingService,
	virtualInventoryService *service.VirtualInventoryService,
) *ProductHandler {
	return &ProductHandler{
		productService:          productService,
		orderService:            orderService,
		bindingService:          bindingService,
		virtualInventoryService: virtualInventoryService,
	}
}

// ListProducts Product列表（User端，仅显示上架Product）
func (h *ProductHandler) ListProducts(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
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

	// User端只显示上架Product
	products, total, err := h.productService.ListProducts(page, limit, string(models.ProductStatusActive), category, search, isFeatured, nil, true)
	if err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	response.Paginated(c, products, page, limit, total)
}

// GetProduct getProduct详情（User端）
func (h *ProductHandler) GetProduct(c *gin.Context) {
	productID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid product ID format")
		return
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
	// 根据商品类型获取不同的库存
	if product.ProductType == models.ProductTypeVirtual {
		// 虚拟商品：从虚拟库存绑定中获取可用库存
		var stock int64
		if len(attributes) > 0 {
			// 根据规格属性查询
			stock, err = h.virtualInventoryService.GetAvailableCountForProductByAttributes(uint(productID), attributes)
		} else {
			// 无属性，查询总库存
			stock, err = h.virtualInventoryService.GetAvailableCountForProduct(uint(productID))
		}
		if err != nil {
			response.InternalError(c, "QueryInventoryFailed")
			return
		}
		totalStock = int(stock)
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

	isFeatured := true
	products, _, err := h.productService.ListProducts(1, limit, string(models.ProductStatusActive), "", "", &isFeatured, nil, true)
	if err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	response.Success(c, gin.H{"products": products})
}

// GetRecommendedProducts get推荐Product
func (h *ProductHandler) GetRecommendedProducts(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit > 50 {
		limit = 50
	}

	isRecommended := true
	products, _, err := h.productService.ListProducts(1, limit, string(models.ProductStatusActive), "", "", nil, &isRecommended, true)
	if err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	response.Success(c, gin.H{"products": products})
}
