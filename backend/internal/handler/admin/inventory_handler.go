package admin

import (
	"log"
	"strconv"
	"strings"

	"auralogic/internal/database"
	"auralogic/internal/pkg/logger"
	"auralogic/internal/pkg/response"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type InventoryHandler struct {
	inventoryService *service.InventoryService
	db               *gorm.DB
	pluginManager    *service.PluginManagerService
}

func NewInventoryHandler(inventoryService *service.InventoryService, db *gorm.DB, pluginManager *service.PluginManagerService) *InventoryHandler {
	return &InventoryHandler{
		inventoryService: inventoryService,
		db:               db,
		pluginManager:    pluginManager,
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
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}
	if h.pluginManager != nil {
		originalReq := req
		hookPayload, payloadErr := adminHookStructToPayload(req)
		if payloadErr != nil {
			log.Printf("inventory.create.before payload build failed: admin=%d err=%v", adminIDValue, payloadErr)
		} else {
			hookPayload["admin_id"] = adminIDValue
			hookPayload["source"] = "admin_api"
			hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "inventory.create.before",
				Payload: hookPayload,
			}, buildAdminHookExecutionContext(c, adminID, map[string]string{
				"hook_resource": "inventory",
				"hook_source":   "admin_api",
			}))
			if hookErr != nil {
				log.Printf("inventory.create.before hook execution failed: admin=%d err=%v", adminIDValue, hookErr)
			} else if hookResult != nil {
				if hookResult.Blocked {
					reason := strings.TrimSpace(hookResult.BlockReason)
					if reason == "" {
						reason = "Inventory creation rejected by plugin"
					}
					response.BadRequest(c, reason)
					return
				}
				if hookResult.Payload != nil {
					if mergeErr := mergeAdminHookStructPatch(&req, hookResult.Payload); mergeErr != nil {
						log.Printf("inventory.create.before payload apply failed, fallback to original request: admin=%d err=%v", adminIDValue, mergeErr)
						req = originalReq
					}
				}
			}
		}
	}

	// 验证：Purchasable quantity cannot exceed inventory quantity
	if req.AvailableQuantity > req.Stock {
		response.BizError(c, "Available quantity cannot exceed total stock", "inventory.availableExceedsStock", nil)
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
		if respondAdminBizError(c, err) {
			return
		}
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
	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"inventory_id":       inventory.ID,
			"name":               inventory.Name,
			"sku":                inventory.SKU,
			"attributes":         inventory.Attributes,
			"stock":              inventory.Stock,
			"available_quantity": inventory.AvailableQuantity,
			"sold_quantity":      inventory.SoldQuantity,
			"reserved_quantity":  inventory.ReservedQuantity,
			"safety_stock":       inventory.SafetyStock,
			"alert_email":        inventory.AlertEmail,
			"notes":              inventory.Notes,
			"is_active":          inventory.IsActive,
			"admin_id":           adminIDValue,
			"source":             "admin_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, inventoryID uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "inventory.create.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("inventory.create.after hook execution failed: admin=%d inventory=%d err=%v", adminIDValue, inventoryID, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "inventory",
			"hook_source":   "admin_api",
		})), afterPayload, inventory.ID)
	}

	response.Success(c, inventory)
}

// UpdateInventory UpdateInventory配置
func (h *InventoryHandler) UpdateInventory(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	inventoryID := uint(id)

	var req UpdateInventoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}

	// 验证：Purchasable quantity cannot exceed inventory quantity
	if req.AvailableQuantity > req.Stock {
		response.BizError(c, "Available quantity cannot exceed total stock", "inventory.availableExceedsStock", nil)
		return
	}

	// 获取更新前的库存信息（用于日志）
	beforeInventory, _ := h.inventoryService.GetInventory(inventoryID)
	if h.pluginManager != nil {
		originalReq := req
		hookPayload, payloadErr := adminHookStructToPayload(req)
		if payloadErr != nil {
			log.Printf("inventory.update.before payload build failed: admin=%d inventory=%d err=%v", adminIDValue, inventoryID, payloadErr)
		} else {
			hookPayload["inventory_id"] = inventoryID
			if beforeInventory != nil {
				hookPayload["current"] = map[string]interface{}{
					"stock":              beforeInventory.Stock,
					"available_quantity": beforeInventory.AvailableQuantity,
					"safety_stock":       beforeInventory.SafetyStock,
					"is_active":          beforeInventory.IsActive,
					"alert_email":        beforeInventory.AlertEmail,
					"notes":              beforeInventory.Notes,
				}
			}
			hookPayload["admin_id"] = adminIDValue
			hookPayload["source"] = "admin_api"
			hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "inventory.update.before",
				Payload: hookPayload,
			}, buildAdminHookExecutionContext(c, adminID, map[string]string{
				"hook_resource": "inventory",
				"hook_source":   "admin_api",
				"inventory_id":  strconv.FormatUint(id, 10),
			}))
			if hookErr != nil {
				log.Printf("inventory.update.before hook execution failed: admin=%d inventory=%d err=%v", adminIDValue, inventoryID, hookErr)
			} else if hookResult != nil {
				if hookResult.Blocked {
					reason := strings.TrimSpace(hookResult.BlockReason)
					if reason == "" {
						reason = "Inventory update rejected by plugin"
					}
					response.BadRequest(c, reason)
					return
				}
				if hookResult.Payload != nil {
					if mergeErr := mergeAdminHookStructPatch(&req, hookResult.Payload); mergeErr != nil {
						log.Printf("inventory.update.before payload apply failed, fallback to original request: admin=%d inventory=%d err=%v", adminIDValue, inventoryID, mergeErr)
						req = originalReq
					}
				}
			}
		}
	}

	err := h.inventoryService.UpdateInventory(
		inventoryID,
		req.Stock,
		req.AvailableQuantity,
		req.SafetyStock,
		req.IsActive,
	)

	if err != nil {
		if respondAdminBizError(c, err) {
			return
		}
		response.BadRequest(c, err.Error())
		return
	}

	// getUpdate后的InventoryInfo
	inventory, _ := h.inventoryService.GetInventory(inventoryID)

	// Update其他字段
	if req.AlertEmail != "" {
		inventory.AlertEmail = req.AlertEmail
	}
	if req.Notes != "" {
		inventory.Notes = req.Notes
	}

	// 记录操作日志
	db := database.GetDB()
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
	if h.pluginManager != nil && inventory != nil {
		afterPayload := map[string]interface{}{
			"inventory_id":              inventory.ID,
			"name":                      inventory.Name,
			"sku":                       inventory.SKU,
			"stock":                     inventory.Stock,
			"available_quantity":        inventory.AvailableQuantity,
			"sold_quantity":             inventory.SoldQuantity,
			"reserved_quantity":         inventory.ReservedQuantity,
			"safety_stock":              inventory.SafetyStock,
			"alert_email":               inventory.AlertEmail,
			"notes":                     inventory.Notes,
			"is_active":                 inventory.IsActive,
			"before_stock":              beforeInventory.Stock,
			"before_available_quantity": beforeInventory.AvailableQuantity,
			"before_safety_stock":       beforeInventory.SafetyStock,
			"before_is_active":          beforeInventory.IsActive,
			"before_alert_email":        beforeInventory.AlertEmail,
			"before_notes":              beforeInventory.Notes,
			"admin_id":                  adminIDValue,
			"source":                    "admin_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "inventory.update.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("inventory.update.after hook execution failed: admin=%d inventory=%d err=%v", adminIDValue, inventoryID, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "inventory",
			"hook_source":   "admin_api",
			"inventory_id":  strconv.FormatUint(id, 10),
		})), afterPayload)
	}

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
	page, limit := response.GetPagination(c)

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
	inventoryID := uint(id)
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}

	// 获取删除前的库存信息（用于日志）
	inventory, _ := h.inventoryService.GetInventory(inventoryID)
	if h.pluginManager != nil && inventory != nil {
		hookPayload := map[string]interface{}{
			"inventory_id":       inventory.ID,
			"name":               inventory.Name,
			"sku":                inventory.SKU,
			"stock":              inventory.Stock,
			"available_quantity": inventory.AvailableQuantity,
			"sold_quantity":      inventory.SoldQuantity,
			"reserved_quantity":  inventory.ReservedQuantity,
			"safety_stock":       inventory.SafetyStock,
			"is_active":          inventory.IsActive,
			"admin_id":           adminIDValue,
			"source":             "admin_api",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "inventory.delete.before",
			Payload: hookPayload,
		}, buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "inventory",
			"hook_source":   "admin_api",
			"inventory_id":  strconv.FormatUint(id, 10),
		}))
		if hookErr != nil {
			log.Printf("inventory.delete.before hook execution failed: admin=%d inventory=%d err=%v", adminIDValue, inventoryID, hookErr)
		} else if hookResult != nil && hookResult.Blocked {
			reason := strings.TrimSpace(hookResult.BlockReason)
			if reason == "" {
				reason = "Inventory deletion rejected by plugin"
			}
			response.BadRequest(c, reason)
			return
		}
	}

	err := h.inventoryService.DeleteInventory(inventoryID)
	if err != nil {
		if respondAdminBizError(c, err) {
			return
		}
		response.BadRequest(c, err.Error())
		return
	}

	// 记录操作日志
	db := database.GetDB()
	logger.LogOperation(db, c, "delete", "inventory", &inventoryID, map[string]interface{}{
		"inventory_id":   id,
		"inventory_name": inventory.Name,
		"stock":          inventory.Stock,
	})
	if h.pluginManager != nil && inventory != nil {
		afterPayload := map[string]interface{}{
			"inventory_id":       inventory.ID,
			"name":               inventory.Name,
			"sku":                inventory.SKU,
			"stock":              inventory.Stock,
			"available_quantity": inventory.AvailableQuantity,
			"sold_quantity":      inventory.SoldQuantity,
			"reserved_quantity":  inventory.ReservedQuantity,
			"safety_stock":       inventory.SafetyStock,
			"is_active":          inventory.IsActive,
			"admin_id":           adminIDValue,
			"source":             "admin_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "inventory.delete.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("inventory.delete.after hook execution failed: admin=%d inventory=%d err=%v", adminIDValue, inventoryID, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "inventory",
			"hook_source":   "admin_api",
			"inventory_id":  strconv.FormatUint(id, 10),
		})), afterPayload)
	}

	response.Success(c, gin.H{"message": "DeleteSuccess"})
}

// AdjustStock 调整库存（增加或减少）
func (h *InventoryHandler) AdjustStock(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	inventoryID := uint(id)

	var req AdjustStockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}

	// 获取操作人
	userEmail, _ := c.Get("user_email")
	operator := "unknown"
	if email, ok := userEmail.(string); ok {
		operator = email
	}

	// 获取调整前的库存信息（用于日志）
	beforeInventory, _ := h.inventoryService.GetInventory(inventoryID)
	if h.pluginManager != nil && beforeInventory != nil {
		originalReq := req
		hookPayload, payloadErr := adminHookStructToPayload(req)
		if payloadErr != nil {
			log.Printf("inventory.adjust.before payload build failed: admin=%d inventory=%d err=%v", adminIDValue, inventoryID, payloadErr)
		} else {
			hookPayload["inventory_id"] = inventoryID
			hookPayload["name"] = beforeInventory.Name
			hookPayload["operator"] = operator
			hookPayload["admin_id"] = adminIDValue
			hookPayload["source"] = "admin_api"
			hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "inventory.adjust.before",
				Payload: hookPayload,
			}, buildAdminHookExecutionContext(c, adminID, map[string]string{
				"hook_resource": "inventory",
				"hook_source":   "admin_api",
				"inventory_id":  strconv.FormatUint(id, 10),
			}))
			if hookErr != nil {
				log.Printf("inventory.adjust.before hook execution failed: admin=%d inventory=%d err=%v", adminIDValue, inventoryID, hookErr)
			} else if hookResult != nil {
				if hookResult.Blocked {
					reason := strings.TrimSpace(hookResult.BlockReason)
					if reason == "" {
						reason = "Inventory adjustment rejected by plugin"
					}
					response.BadRequest(c, reason)
					return
				}
				if hookResult.Payload != nil {
					if mergeErr := mergeAdminHookStructPatch(&req, hookResult.Payload); mergeErr != nil {
						log.Printf("inventory.adjust.before payload apply failed, fallback to original request: admin=%d inventory=%d err=%v", adminIDValue, inventoryID, mergeErr)
						req = originalReq
					}
				}
			}
		}
	}

	err := h.inventoryService.AdjustStockByDelta(
		inventoryID,
		req.StockDelta,
		req.AvailableQuantityDelta,
		operator,
		req.Reason,
	)

	if err != nil {
		if respondAdminBizError(c, err) {
			return
		}
		response.BadRequest(c, err.Error())
		return
	}

	// 获取调整后的库存信息
	inventory, _ := h.inventoryService.GetInventory(inventoryID)

	// 记录操作日志
	db := database.GetDB()
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
	if h.pluginManager != nil && inventory != nil && beforeInventory != nil {
		afterPayload := map[string]interface{}{
			"inventory_id":              inventory.ID,
			"name":                      inventory.Name,
			"stock_delta":               req.StockDelta,
			"available_quantity_delta":  req.AvailableQuantityDelta,
			"reason":                    req.Reason,
			"notes":                     req.Notes,
			"operator":                  operator,
			"before_stock":              beforeInventory.Stock,
			"after_stock":               inventory.Stock,
			"before_available_quantity": beforeInventory.AvailableQuantity,
			"after_available_quantity":  inventory.AvailableQuantity,
			"before_sold_quantity":      beforeInventory.SoldQuantity,
			"after_sold_quantity":       inventory.SoldQuantity,
			"before_reserved_quantity":  beforeInventory.ReservedQuantity,
			"after_reserved_quantity":   inventory.ReservedQuantity,
			"admin_id":                  adminIDValue,
			"source":                    "admin_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "inventory.adjust.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("inventory.adjust.after hook execution failed: admin=%d inventory=%d err=%v", adminIDValue, inventoryID, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "inventory",
			"hook_source":   "admin_api",
			"inventory_id":  strconv.FormatUint(id, 10),
		})), afterPayload)
	}

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
