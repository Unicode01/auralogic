package admin

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"auralogic/internal/database"
	"auralogic/internal/pkg/logger"
	"auralogic/internal/pkg/response"
	"auralogic/internal/service"
)

type InventoryHandler struct {
	inventoryService *service.InventoryService
}

func NewInventoryHandler(inventoryService *service.InventoryService) *InventoryHandler {
	return &InventoryHandler{
		inventoryService: inventoryService,
	}
}

// CreateInventoryRequest CreateInventory请求（独立Create，不need关联Product）
type CreateInventoryRequest struct {
	Name              string            `json:"name" binding:"required"` // Inventory配置名称
	SKU               string            `json:"sku"`                     // SKU（可选）
	Attributes        map[string]string `json:"attributes"`              // 属性组合，如：{"颜色":"红色","尺寸":"L"}
	Stock             int               `json:"stock" binding:"required,min=0"`
	AvailableQuantity int               `json:"available_quantity" binding:"required,min=0"`
	SafetyStock       int               `json:"safety_stock" binding:"min=0"`
	AlertEmail        string            `json:"alert_email,omitempty"`
	Notes             string            `json:"notes,omitempty"`
}

// UpdateInventoryRequest UpdateInventory请求
type UpdateInventoryRequest struct {
	Stock             int    `json:"stock" binding:"required,min=0"`
	AvailableQuantity int    `json:"available_quantity" binding:"required,min=0"`
	SafetyStock       int    `json:"safety_stock" binding:"min=0"`
	IsActive          bool   `json:"is_active"`
	AlertEmail        string `json:"alert_email,omitempty"`
	Notes             string `json:"notes,omitempty"`
}

// AdjustStockRequest 调整库存请求
type AdjustStockRequest struct {
	StockDelta             int    `json:"stock_delta" binding:"required"`              // 库存增量（正数增加，负数减少）
	AvailableQuantityDelta int    `json:"available_quantity_delta" binding:"required"` // 可售数量增量
	Reason                 string `json:"reason" binding:"required"`
	Notes                  string `json:"notes,omitempty"`
}

// CreateInventory CreateInventory配置（独立Create，之后通过绑定API关联到Product）
func (h *InventoryHandler) CreateInventory(c *gin.Context) {
	var req CreateInventoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	// 验证：Purchasable quantity cannot exceed inventory quantity
	if req.AvailableQuantity > req.Stock {
		response.BadRequest(c, "Purchasable quantity cannot exceed inventory quantity")
		return
	}

	// 如果没有提供属性，使用空对象
	if req.Attributes == nil {
		req.Attributes = make(map[string]string)
	}

	inventory, err := h.inventoryService.CreateInventory(
		req.Name,
		req.SKU,
		req.Attributes,
		req.Stock,
		req.AvailableQuantity,
		req.SafetyStock,
	)

	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Update其他字段
	if req.AlertEmail != "" {
		inventory.AlertEmail = req.AlertEmail
	}
	if req.Notes != "" {
		inventory.Notes = req.Notes
	}

	// 记录操作日志
	db := database.GetDB()
	logger.LogOperation(db, c, "create", "inventory", &inventory.ID, map[string]interface{}{
		"name":               req.Name,
		"sku":                req.SKU,
		"stock":              req.Stock,
		"available_quantity": req.AvailableQuantity,
		"safety_stock":       req.SafetyStock,
	})

	response.Success(c, inventory)
}

// UpdateInventory UpdateInventory配置
func (h *InventoryHandler) UpdateInventory(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var req UpdateInventoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	// 验证：Purchasable quantity cannot exceed inventory quantity
	if req.AvailableQuantity > req.Stock {
		response.BadRequest(c, "Purchasable quantity cannot exceed inventory quantity")
		return
	}

	// 获取更新前的库存信息（用于日志）
	beforeInventory, _ := h.inventoryService.GetInventory(uint(id))

	err := h.inventoryService.UpdateInventory(
		uint(id),
		req.Stock,
		req.AvailableQuantity,
		req.SafetyStock,
		req.IsActive,
	)

	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// getUpdate后的InventoryInfo
	inventory, _ := h.inventoryService.GetInventory(uint(id))

	// Update其他字段
	if req.AlertEmail != "" {
		inventory.AlertEmail = req.AlertEmail
	}
	if req.Notes != "" {
		inventory.Notes = req.Notes
	}

	// 记录操作日志
	db := database.GetDB()
	inventoryID := uint(id)
	logger.LogOperation(db, c, "update", "inventory", &inventoryID, map[string]interface{}{
		"inventory_id":              id,
		"inventory_name":            inventory.Name,
		"before_stock":              beforeInventory.Stock,
		"after_stock":               inventory.Stock,
		"before_available_quantity": beforeInventory.AvailableQuantity,
		"after_available_quantity":  inventory.AvailableQuantity,
		"before_is_active":          beforeInventory.IsActive,
		"after_is_active":           inventory.IsActive,
	})

	response.Success(c, inventory)
}

// GetInventory getInventory详情
func (h *InventoryHandler) GetInventory(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	inventory, err := h.inventoryService.GetInventory(uint(id))
	if err != nil {
		response.NotFound(c, "Inventory record does not exist")
		return
	}

	response.Success(c, inventory)
}

// ListInventories Inventory列表
func (h *InventoryHandler) ListInventories(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	filters := make(map[string]interface{})

	// 状态过滤
	if isActiveStr := c.Query("is_active"); isActiveStr != "" {
		filters["is_active"] = isActiveStr == "true"
	}

	// 低Inventory过滤
	if lowStockStr := c.Query("low_stock"); lowStockStr == "true" {
		filters["low_stock"] = true
	}

	inventories, total, err := h.inventoryService.ListInventories(page, limit, filters)
	if err != nil {
		// 记录详细ErrorInfo
		c.Error(err)
		response.InternalError(c, "Query failed")
		return
	}

	response.Paginated(c, inventories, page, limit, total)
}

// GetProductInventories getProduct的所有Inventory配置
func (h *InventoryHandler) GetProductInventories(c *gin.Context) {
	productID, _ := strconv.ParseUint(c.Param("product_id"), 10, 32)

	inventories, err := h.inventoryService.GetProductInventories(uint(productID))
	if err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	response.Success(c, inventories)
}

// DeleteInventory DeleteInventory配置
func (h *InventoryHandler) DeleteInventory(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	// 获取删除前的库存信息（用于日志）
	inventory, _ := h.inventoryService.GetInventory(uint(id))

	err := h.inventoryService.DeleteInventory(uint(id))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// 记录操作日志
	db := database.GetDB()
	inventoryID := uint(id)
	logger.LogOperation(db, c, "delete", "inventory", &inventoryID, map[string]interface{}{
		"inventory_id":   id,
		"inventory_name": inventory.Name,
		"stock":          inventory.Stock,
	})

	response.Success(c, gin.H{"message": "DeleteSuccess"})
}

// AdjustStock 调整库存（增加或减少）
func (h *InventoryHandler) AdjustStock(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var req AdjustStockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	// 获取操作人
	userEmail, _ := c.Get("user_email")
	operator := "unknown"
	if email, ok := userEmail.(string); ok {
		operator = email
	}

	// 获取调整前的库存信息（用于日志）
	beforeInventory, _ := h.inventoryService.GetInventory(uint(id))

	err := h.inventoryService.AdjustStockByDelta(
		uint(id),
		req.StockDelta,
		req.AvailableQuantityDelta,
		operator,
		req.Reason,
	)

	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// 获取调整后的库存信息
	inventory, _ := h.inventoryService.GetInventory(uint(id))

	// 记录操作日志
	db := database.GetDB()
	inventoryID := uint(id)
	logger.LogOperation(db, c, "adjust_stock", "inventory", &inventoryID, map[string]interface{}{
		"inventory_id":              id,
		"inventory_name":            inventory.Name,
		"stock_delta":               req.StockDelta,
		"available_quantity_delta":  req.AvailableQuantityDelta,
		"before_stock":              beforeInventory.Stock,
		"after_stock":               inventory.Stock,
		"before_available_quantity": beforeInventory.AvailableQuantity,
		"after_available_quantity":  inventory.AvailableQuantity,
		"reason":                    req.Reason,
		"notes":                     req.Notes,
		"operator":                  operator,
	})

	response.Success(c, inventory)
}

// GetLowStockList get低Inventory列表
func (h *InventoryHandler) GetLowStockList(c *gin.Context) {
	inventories, err := h.inventoryService.GetLowStockList()
	if err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	response.Success(c, inventories)
}
