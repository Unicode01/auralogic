package admin

import (
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"auralogic/internal/middleware"
	"auralogic/internal/models"
	"auralogic/internal/pkg/bizerr"
	"auralogic/internal/pkg/response"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type VirtualInventoryHandler struct {
	service       *service.VirtualInventoryService
	db            *gorm.DB
	pluginManager *service.PluginManagerService
}

func NewVirtualInventoryHandler(service *service.VirtualInventoryService, db *gorm.DB, pluginManager *service.PluginManagerService) *VirtualInventoryHandler {
	return &VirtualInventoryHandler{
		service:       service,
		db:            db,
		pluginManager: pluginManager,
	}
}

func (h *VirtualInventoryHandler) loadVirtualBinding(bindingID uint) (*models.ProductVirtualInventoryBinding, error) {
	if h.db == nil {
		return nil, gorm.ErrInvalidDB
	}

	var binding models.ProductVirtualInventoryBinding
	if err := h.db.Preload("VirtualInventory").First(&binding, bindingID).Error; err != nil {
		return nil, err
	}
	return &binding, nil
}

func (h *VirtualInventoryHandler) loadVirtualStock(stockID uint) (*models.VirtualProductStock, error) {
	if h.db == nil {
		return nil, gorm.ErrInvalidDB
	}

	var stock models.VirtualProductStock
	if err := h.db.First(&stock, stockID).Error; err != nil {
		return nil, err
	}
	return &stock, nil
}

func buildVirtualInventoryHookPayload(inventory *models.VirtualInventory) map[string]interface{} {
	if inventory == nil {
		return map[string]interface{}{}
	}

	return map[string]interface{}{
		"virtual_inventory_id": inventory.ID,
		"name":                 inventory.Name,
		"sku":                  inventory.SKU,
		"type":                 inventory.Type,
		"script":               inventory.Script,
		"script_config":        inventory.ScriptConfig,
		"description":          inventory.Description,
		"total_limit":          inventory.TotalLimit,
		"is_active":            inventory.IsActive,
		"notes":                inventory.Notes,
		"created_at":           inventory.CreatedAt,
		"updated_at":           inventory.UpdatedAt,
	}
}

func buildVirtualInventoryBindingHookPayload(binding *models.ProductVirtualInventoryBinding) map[string]interface{} {
	if binding == nil {
		return map[string]interface{}{}
	}

	payload := map[string]interface{}{
		"binding_id":           binding.ID,
		"product_id":           binding.ProductID,
		"virtual_inventory_id": binding.VirtualInventoryID,
		"attributes":           binding.Attributes,
		"attributes_hash":      binding.AttributesHash,
		"is_random":            binding.IsRandom,
		"priority":             binding.Priority,
		"notes":                binding.Notes,
		"created_at":           binding.CreatedAt,
		"updated_at":           binding.UpdatedAt,
	}
	if binding.VirtualInventory != nil {
		payload["virtual_inventory_name"] = binding.VirtualInventory.Name
		payload["virtual_inventory_type"] = binding.VirtualInventory.Type
		payload["virtual_inventory_active"] = binding.VirtualInventory.IsActive
	}
	return payload
}

func buildVirtualInventoryBindingHookPayloadList(bindings []models.ProductVirtualInventoryBinding) []map[string]interface{} {
	payloads := make([]map[string]interface{}, 0, len(bindings))
	for idx := range bindings {
		payloads = append(payloads, buildVirtualInventoryBindingHookPayload(&bindings[idx]))
	}
	return payloads
}

func buildVirtualStockHookPayload(stock *models.VirtualProductStock) map[string]interface{} {
	if stock == nil {
		return map[string]interface{}{}
	}

	return map[string]interface{}{
		"stock_id":             stock.ID,
		"virtual_inventory_id": stock.VirtualInventoryID,
		"content":              stock.Content,
		"remark":               stock.Remark,
		"status":               stock.Status,
		"order_id":             stock.OrderID,
		"order_no":             stock.OrderNo,
		"delivered_at":         stock.DeliveredAt,
		"delivered_by":         stock.DeliveredBy,
		"batch_no":             stock.BatchNo,
		"imported_by":          stock.ImportedBy,
		"created_at":           stock.CreatedAt,
		"updated_at":           stock.UpdatedAt,
	}
}

// ListVirtualInventories 获取虚拟库存列表
func (h *VirtualInventoryHandler) ListVirtualInventories(c *gin.Context) {
	page := c.DefaultQuery("page", "1")
	limit := c.DefaultQuery("limit", "20")
	search := c.Query("search")

	pageInt, limitInt := 1, 20
	fmt.Sscanf(page, "%d", &pageInt)
	fmt.Sscanf(limit, "%d", &limitInt)

	if pageInt < 1 {
		pageInt = 1
	}
	if limitInt < 1 || limitInt > 100 {
		limitInt = 20
	}

	inventories, total, err := h.service.ListVirtualInventories(pageInt, limitInt, search)
	if err != nil {
		response.InternalError(c, "Failed to get virtual inventories")
		return
	}

	response.Paginated(c, inventories, pageInt, limitInt, total)
}

// CreateVirtualInventory 创建虚拟库存
func (h *VirtualInventoryHandler) CreateVirtualInventory(c *gin.Context) {
	var req struct {
		Name         string `json:"name" binding:"required"`
		SKU          string `json:"sku"`
		Type         string `json:"type"`
		Script       string `json:"script"`
		ScriptConfig string `json:"script_config"`
		Description  string `json:"description"`
		TotalLimit   int64  `json:"total_limit"`
		IsActive     bool   `json:"is_active"`
		Notes        string `json:"notes"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request data")
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
			log.Printf("virtual_inventory.create.before payload build failed: admin=%d err=%v", adminIDValue, payloadErr)
		} else {
			hookPayload["admin_id"] = adminIDValue
			hookPayload["source"] = "admin_api"
			hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "virtual_inventory.create.before",
				Payload: hookPayload,
			}, buildAdminHookExecutionContext(c, adminID, map[string]string{
				"hook_resource": "virtual_inventory",
				"hook_source":   "admin_api",
			}))
			if hookErr != nil {
				log.Printf("virtual_inventory.create.before hook execution failed: admin=%d err=%v", adminIDValue, hookErr)
			} else if hookResult != nil {
				if hookResult.Blocked {
					reason := strings.TrimSpace(hookResult.BlockReason)
					if reason == "" {
						reason = "Virtual inventory creation rejected by plugin"
					}
					response.BadRequest(c, reason)
					return
				}
				if hookResult.Payload != nil {
					if mergeErr := mergeAdminHookStructPatch(&req, hookResult.Payload); mergeErr != nil {
						log.Printf("virtual_inventory.create.before payload apply failed, fallback to original request: admin=%d err=%v", adminIDValue, mergeErr)
						req = originalReq
					}
				}
			}
		}
	}

	invType := models.VirtualInventoryTypeStatic
	if req.Type == "script" {
		invType = models.VirtualInventoryTypeScript
		if strings.TrimSpace(req.Script) == "" {
			response.BadRequest(c, "Script content is required for script type inventory")
			return
		}
	}

	inventory := &models.VirtualInventory{
		Name:         req.Name,
		SKU:          req.SKU,
		Type:         invType,
		Script:       req.Script,
		ScriptConfig: req.ScriptConfig,
		Description:  req.Description,
		TotalLimit:   req.TotalLimit,
		IsActive:     req.IsActive,
		Notes:        req.Notes,
	}

	if err := h.service.CreateVirtualInventory(inventory); err != nil {
		if respondAdminBizError(c, err) {
			return
		}
		response.InternalError(c, "Failed to create virtual inventory")
		return
	}

	if h.pluginManager != nil {
		afterPayload := buildVirtualInventoryHookPayload(inventory)
		afterPayload["admin_id"] = adminIDValue
		afterPayload["source"] = "admin_api"
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, inventoryID uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "virtual_inventory.create.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("virtual_inventory.create.after hook execution failed: admin=%d inventory=%d err=%v", adminIDValue, inventoryID, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource":        "virtual_inventory",
			"hook_source":          "admin_api",
			"virtual_inventory_id": strconv.FormatUint(uint64(inventory.ID), 10),
		})), afterPayload, inventory.ID)
	}

	response.Success(c, gin.H{
		"message":   "Virtual inventory created successfully",
		"inventory": inventory,
	})
}

// GetVirtualInventory 获取虚拟库存详情
func (h *VirtualInventoryHandler) GetVirtualInventory(c *gin.Context) {
	id, err := middleware.GetUintParam(c, "id")
	if err != nil {
		response.BadRequest(c, "Invalid inventory ID")
		return
	}

	inventory, err := h.service.GetVirtualInventoryWithStats(id)
	if err != nil {
		response.NotFound(c, "Virtual inventory not found")
		return
	}

	response.Success(c, inventory)
}

// UpdateVirtualInventory 更新虚拟库存
func (h *VirtualInventoryHandler) UpdateVirtualInventory(c *gin.Context) {
	id, err := middleware.GetUintParam(c, "id")
	if err != nil {
		response.BadRequest(c, "Invalid inventory ID")
		return
	}

	var req struct {
		Name         string  `json:"name"`
		SKU          string  `json:"sku"`
		Type         string  `json:"type"`
		Script       *string `json:"script"`
		ScriptConfig *string `json:"script_config"`
		Description  string  `json:"description"`
		TotalLimit   *int64  `json:"total_limit"`
		IsActive     *bool   `json:"is_active"`
		Notes        string  `json:"notes"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request data")
		return
	}
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}
	beforeInventory, _ := h.service.GetVirtualInventory(id)
	if h.pluginManager != nil {
		originalReq := req
		hookPayload, payloadErr := adminHookStructToPayload(req)
		if payloadErr != nil {
			log.Printf("virtual_inventory.update.before payload build failed: admin=%d inventory=%d err=%v", adminIDValue, id, payloadErr)
		} else {
			hookPayload["virtual_inventory_id"] = id
			if beforeInventory != nil {
				hookPayload["current"] = buildVirtualInventoryHookPayload(beforeInventory)
			}
			hookPayload["admin_id"] = adminIDValue
			hookPayload["source"] = "admin_api"
			hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "virtual_inventory.update.before",
				Payload: hookPayload,
			}, buildAdminHookExecutionContext(c, adminID, map[string]string{
				"hook_resource":        "virtual_inventory",
				"hook_source":          "admin_api",
				"virtual_inventory_id": strconv.FormatUint(uint64(id), 10),
			}))
			if hookErr != nil {
				log.Printf("virtual_inventory.update.before hook execution failed: admin=%d inventory=%d err=%v", adminIDValue, id, hookErr)
			} else if hookResult != nil {
				if hookResult.Blocked {
					reason := strings.TrimSpace(hookResult.BlockReason)
					if reason == "" {
						reason = "Virtual inventory update rejected by plugin"
					}
					response.BadRequest(c, reason)
					return
				}
				if hookResult.Payload != nil {
					if mergeErr := mergeAdminHookStructPatch(&req, hookResult.Payload); mergeErr != nil {
						log.Printf("virtual_inventory.update.before payload apply failed, fallback to original request: admin=%d inventory=%d err=%v", adminIDValue, id, mergeErr)
						req = originalReq
					}
				}
			}
		}
	}

	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.SKU != "" {
		updates["sku"] = req.SKU
	}
	if req.Type != "" {
		if req.Type != "static" && req.Type != "script" {
			response.BadRequest(c, "Invalid inventory type, must be 'static' or 'script'")
			return
		}
		updates["type"] = req.Type
	}
	if req.Script != nil {
		updates["script"] = *req.Script
	}
	if req.ScriptConfig != nil {
		updates["script_config"] = *req.ScriptConfig
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if req.TotalLimit != nil {
		updates["total_limit"] = *req.TotalLimit
	}
	if req.Notes != "" {
		updates["notes"] = req.Notes
	}

	if err := h.service.UpdateVirtualInventory(id, updates); err != nil {
		if respondAdminBizError(c, err) {
			return
		}
		response.InternalError(c, "Failed to update virtual inventory")
		return
	}

	if h.pluginManager != nil {
		updatedInventory, loadErr := h.service.GetVirtualInventory(id)
		if loadErr == nil && updatedInventory != nil {
			afterPayload := buildVirtualInventoryHookPayload(updatedInventory)
			if beforeInventory != nil {
				afterPayload["before_name"] = beforeInventory.Name
				afterPayload["before_sku"] = beforeInventory.SKU
				afterPayload["before_type"] = beforeInventory.Type
				afterPayload["before_script"] = beforeInventory.Script
				afterPayload["before_script_config"] = beforeInventory.ScriptConfig
				afterPayload["before_description"] = beforeInventory.Description
				afterPayload["before_total_limit"] = beforeInventory.TotalLimit
				afterPayload["before_is_active"] = beforeInventory.IsActive
				afterPayload["before_notes"] = beforeInventory.Notes
			}
			afterPayload["admin_id"] = adminIDValue
			afterPayload["source"] = "admin_api"
			go func(execCtx *service.ExecutionContext, payload map[string]interface{}) {
				_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
					Hook:    "virtual_inventory.update.after",
					Payload: payload,
				}, execCtx)
				if hookErr != nil {
					log.Printf("virtual_inventory.update.after hook execution failed: admin=%d inventory=%d err=%v", adminIDValue, id, hookErr)
				}
			}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
				"hook_resource":        "virtual_inventory",
				"hook_source":          "admin_api",
				"virtual_inventory_id": strconv.FormatUint(uint64(id), 10),
			})), afterPayload)
		}
	}

	response.Success(c, gin.H{"message": "Virtual inventory updated successfully"})
}

// DeleteVirtualInventory 删除虚拟库存
func (h *VirtualInventoryHandler) DeleteVirtualInventory(c *gin.Context) {
	id, err := middleware.GetUintParam(c, "id")
	if err != nil {
		response.BadRequest(c, "Invalid inventory ID")
		return
	}
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}
	beforeInventory, _ := h.service.GetVirtualInventory(id)
	if h.pluginManager != nil && beforeInventory != nil {
		hookPayload := buildVirtualInventoryHookPayload(beforeInventory)
		hookPayload["admin_id"] = adminIDValue
		hookPayload["source"] = "admin_api"
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "virtual_inventory.delete.before",
			Payload: hookPayload,
		}, buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource":        "virtual_inventory",
			"hook_source":          "admin_api",
			"virtual_inventory_id": strconv.FormatUint(uint64(id), 10),
		}))
		if hookErr != nil {
			log.Printf("virtual_inventory.delete.before hook execution failed: admin=%d inventory=%d err=%v", adminIDValue, id, hookErr)
		} else if hookResult != nil && hookResult.Blocked {
			reason := strings.TrimSpace(hookResult.BlockReason)
			if reason == "" {
				reason = "Virtual inventory deletion rejected by plugin"
			}
			response.BadRequest(c, reason)
			return
		}
	}

	if err := h.service.DeleteVirtualInventory(id); err != nil {
		if respondAdminBizError(c, err) {
			return
		}
		response.InternalError(c, "Failed to delete virtual inventory")
		return
	}

	if h.pluginManager != nil && beforeInventory != nil {
		afterPayload := buildVirtualInventoryHookPayload(beforeInventory)
		afterPayload["admin_id"] = adminIDValue
		afterPayload["source"] = "admin_api"
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "virtual_inventory.delete.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("virtual_inventory.delete.after hook execution failed: admin=%d inventory=%d err=%v", adminIDValue, id, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource":        "virtual_inventory",
			"hook_source":          "admin_api",
			"virtual_inventory_id": strconv.FormatUint(uint64(id), 10),
		})), afterPayload)
	}

	response.Success(c, gin.H{"message": "Virtual inventory deleted successfully"})
}

// TestDeliveryScript 测试发货脚本
func (h *VirtualInventoryHandler) TestDeliveryScript(c *gin.Context) {
	var req struct {
		Script   string                 `json:"script" binding:"required"`
		Config   map[string]interface{} `json:"config"`
		Quantity int                    `json:"quantity"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request data")
		return
	}

	if req.Quantity <= 0 {
		req.Quantity = 1
	}

	result, err := h.service.TestDeliveryScript(req.Script, req.Config, req.Quantity)
	if err != nil {
		if respondAdminBizError(c, err) {
			return
		}
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, result)
}

// ImportStock 导入库存项
func (h *VirtualInventoryHandler) ImportStock(c *gin.Context) {
	id, err := middleware.GetUintParam(c, "id")
	if err != nil {
		response.BadRequest(c, "Invalid inventory ID")
		return
	}

	h.handleImportStockRequest(c, id, nil)
}

// handleFileImport 处理文件导入
func (h *VirtualInventoryHandler) handleFileImport(virtualInventoryID uint, file *multipart.FileHeader, importedBy string) (int, error) {
	// 检查文件类型
	ext := strings.ToLower(filepath.Ext(file.Filename))

	// 打开上传的文件
	src, err := file.Open()
	if err != nil {
		return 0, fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer src.Close()

	switch ext {
	case ".xlsx", ".xls":
		// 保存临时文件用于Excel处理
		// 确保tmp目录存在
		if err := os.MkdirAll("./tmp", 0755); err != nil {
			return 0, fmt.Errorf("failed to create tmp dir: %w", err)
		}

		// Use a server-generated temp filename to prevent path traversal via file.Filename.
		dst, err := os.CreateTemp("./tmp", "virtual-inv-*"+ext)
		if err != nil {
			return 0, fmt.Errorf("failed to create temp file: %w", err)
		}
		tempPath := dst.Name()
		defer dst.Close()
		defer os.Remove(tempPath) // 处理完后删除临时文件

		// 复制文件内容
		if _, err := io.Copy(dst, src); err != nil {
			return 0, fmt.Errorf("failed to save temp file: %w", err)
		}

		return h.service.ImportFromExcel(virtualInventoryID, tempPath, importedBy)

	case ".txt":
		// txt文件读取内容后调用ImportFromText
		content, err := io.ReadAll(src)
		if err != nil {
			return 0, fmt.Errorf("failed to read txt file: %w", err)
		}
		return h.service.ImportFromText(virtualInventoryID, string(content), importedBy)

	case ".csv":
		// CSV导入
		return h.service.ImportFromCSV(virtualInventoryID, src, importedBy)

	default:
		return 0, bizerr.New("virtual_inventory.unsupportedFileType", "Unsupported file type").
			WithParams(map[string]interface{}{"ext": ext})
	}
}

func (h *VirtualInventoryHandler) handleImportStockRequest(c *gin.Context, virtualInventoryID uint, productID *uint) {
	inv, err := h.service.GetVirtualInventory(virtualInventoryID)
	if err != nil {
		response.NotFound(c, "Virtual inventory not found")
		return
	}
	if inv.Type == models.VirtualInventoryTypeScript {
		response.BizError(
			c,
			"Script type inventory does not support manual stock import",
			"virtual_inventory.manualImportUnsupported",
			nil,
		)
		return
	}

	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}
	importType := strings.TrimSpace(c.PostForm("import_type"))
	content := c.PostForm("content")
	if h.pluginManager != nil {
		hookPayload := map[string]interface{}{
			"virtual_inventory_id": virtualInventoryID,
			"name":                 inv.Name,
			"type":                 inv.Type,
			"import_type":          importType,
			"content":              content,
			"admin_id":             adminIDValue,
			"source":               "admin_api",
		}
		if productID != nil {
			hookPayload["product_id"] = *productID
		}
		if importType == "file" {
			if file, fileErr := c.FormFile("file"); fileErr == nil && file != nil {
				hookPayload["file_name"] = file.Filename
				hookPayload["file_size"] = file.Size
			}
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "virtual_inventory.stock.import.before",
			Payload: hookPayload,
		}, buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource":        "virtual_inventory_stock",
			"hook_source":          "admin_api",
			"virtual_inventory_id": strconv.FormatUint(uint64(virtualInventoryID), 10),
		}))
		if hookErr != nil {
			log.Printf("virtual_inventory.stock.import.before hook execution failed: admin=%d inventory=%d err=%v", adminIDValue, virtualInventoryID, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Virtual inventory stock import rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if importTypeValue, exists := hookResult.Payload["import_type"]; exists {
					patchedImportType, valueErr := adminHookValueToString(importTypeValue)
					if valueErr != nil {
						log.Printf("virtual_inventory.stock.import.before import_type patch ignored: admin=%d inventory=%d err=%v", adminIDValue, virtualInventoryID, valueErr)
					} else {
						importType = strings.TrimSpace(patchedImportType)
					}
				}
				if contentValue, exists := hookResult.Payload["content"]; exists {
					patchedContent, valueErr := adminHookValueToString(contentValue)
					if valueErr != nil {
						log.Printf("virtual_inventory.stock.import.before content patch ignored: admin=%d inventory=%d err=%v", adminIDValue, virtualInventoryID, valueErr)
					} else {
						content = patchedContent
					}
				}
			}
		}
	}

	importedBy := c.GetString("user_email")
	var count int
	var importErr error

	switch importType {
	case "file":
		file, fileErr := c.FormFile("file")
		if fileErr != nil {
			response.BadRequest(c, "File upload failed")
			return
		}
		count, importErr = h.handleFileImport(virtualInventoryID, file, importedBy)
	case "text":
		if content == "" {
			response.BizError(c, "Content cannot be empty", "virtual_inventory.contentRequired", nil)
			return
		}
		count, importErr = h.service.ImportFromText(virtualInventoryID, content, importedBy)
	default:
		response.BizError(c, "Invalid import type", "virtual_inventory.importTypeInvalid", nil)
		return
	}

	if importErr != nil {
		if respondAdminBizError(c, importErr) {
			return
		}
		response.InternalError(c, fmt.Sprintf("Import failed: %v", importErr))
		return
	}

	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"virtual_inventory_id": virtualInventoryID,
			"name":                 inv.Name,
			"type":                 inv.Type,
			"import_type":          importType,
			"count":                count,
			"admin_id":             adminIDValue,
			"source":               "admin_api",
		}
		if importType == "text" {
			afterPayload["content_length"] = len(content)
		}
		if productID != nil {
			afterPayload["product_id"] = *productID
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "virtual_inventory.stock.import.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("virtual_inventory.stock.import.after hook execution failed: admin=%d inventory=%d err=%v", adminIDValue, virtualInventoryID, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource":        "virtual_inventory_stock",
			"hook_source":          "admin_api",
			"virtual_inventory_id": strconv.FormatUint(uint64(virtualInventoryID), 10),
		})), afterPayload)
	}

	response.Success(c, gin.H{
		"message": fmt.Sprintf("Successfully imported %d items", count),
		"count":   count,
	})
}

func (h *VirtualInventoryHandler) handleDeleteStockRequest(c *gin.Context, stockID uint) {
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}
	beforeStock, _ := h.loadVirtualStock(stockID)
	if h.pluginManager != nil && beforeStock != nil {
		hookPayload := buildVirtualStockHookPayload(beforeStock)
		hookPayload["admin_id"] = adminIDValue
		hookPayload["source"] = "admin_api"
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "virtual_inventory.stock.delete.before",
			Payload: hookPayload,
		}, buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource":        "virtual_inventory_stock",
			"hook_source":          "admin_api",
			"virtual_inventory_id": strconv.FormatUint(uint64(beforeStock.VirtualInventoryID), 10),
			"stock_id":             strconv.FormatUint(uint64(stockID), 10),
		}))
		if hookErr != nil {
			log.Printf("virtual_inventory.stock.delete.before hook execution failed: admin=%d stock=%d err=%v", adminIDValue, stockID, hookErr)
		} else if hookResult != nil && hookResult.Blocked {
			reason := strings.TrimSpace(hookResult.BlockReason)
			if reason == "" {
				reason = "Virtual inventory stock deletion rejected by plugin"
			}
			response.BadRequest(c, reason)
			return
		}
	}

	if err := h.service.DeleteStock(stockID); err != nil {
		if respondAdminBizError(c, err) {
			return
		}
		response.InternalError(c, "Operation failed")
		return
	}

	if h.pluginManager != nil && beforeStock != nil {
		afterPayload := buildVirtualStockHookPayload(beforeStock)
		afterPayload["admin_id"] = adminIDValue
		afterPayload["source"] = "admin_api"
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "virtual_inventory.stock.delete.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("virtual_inventory.stock.delete.after hook execution failed: admin=%d stock=%d err=%v", adminIDValue, stockID, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource":        "virtual_inventory_stock",
			"hook_source":          "admin_api",
			"virtual_inventory_id": strconv.FormatUint(uint64(beforeStock.VirtualInventoryID), 10),
			"stock_id":             strconv.FormatUint(uint64(stockID), 10),
		})), afterPayload)
	}

	response.Success(c, gin.H{"message": "Stock item deleted successfully"})
}

// CreateStockManually 手动创建单个库存项
func (h *VirtualInventoryHandler) CreateStockManually(c *gin.Context) {
	id, err := middleware.GetUintParam(c, "id")
	if err != nil {
		response.BadRequest(c, "Invalid inventory ID")
		return
	}

	// 检查库存类型
	inv, err := h.service.GetVirtualInventory(id)
	if err != nil {
		response.NotFound(c, "Virtual inventory not found")
		return
	}
	if inv.Type == models.VirtualInventoryTypeScript {
		response.BizError(
			c,
			"Script type inventory does not support manual stock creation",
			"virtual_inventory.manualCreateUnsupported",
			nil,
		)
		return
	}

	var req struct {
		Content string `json:"content" binding:"required"`
		Remark  string `json:"remark"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request data")
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
			log.Printf("virtual_inventory.stock.create.before payload build failed: admin=%d inventory=%d err=%v", adminIDValue, id, payloadErr)
		} else {
			hookPayload["virtual_inventory_id"] = id
			hookPayload["inventory_name"] = inv.Name
			hookPayload["inventory_type"] = inv.Type
			hookPayload["admin_id"] = adminIDValue
			hookPayload["source"] = "admin_api"
			hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "virtual_inventory.stock.create.before",
				Payload: hookPayload,
			}, buildAdminHookExecutionContext(c, adminID, map[string]string{
				"hook_resource":        "virtual_inventory_stock",
				"hook_source":          "admin_api",
				"virtual_inventory_id": strconv.FormatUint(uint64(id), 10),
			}))
			if hookErr != nil {
				log.Printf("virtual_inventory.stock.create.before hook execution failed: admin=%d inventory=%d err=%v", adminIDValue, id, hookErr)
			} else if hookResult != nil {
				if hookResult.Blocked {
					reason := strings.TrimSpace(hookResult.BlockReason)
					if reason == "" {
						reason = "Virtual inventory stock creation rejected by plugin"
					}
					response.BadRequest(c, reason)
					return
				}
				if hookResult.Payload != nil {
					if mergeErr := mergeAdminHookStructPatch(&req, hookResult.Payload); mergeErr != nil {
						log.Printf("virtual_inventory.stock.create.before payload apply failed, fallback to original request: admin=%d inventory=%d err=%v", adminIDValue, id, mergeErr)
						req = originalReq
					}
				}
			}
		}
	}

	importedBy := c.GetString("user_email")

	stock, err := h.service.CreateStockManually(id, req.Content, req.Remark, importedBy)
	if err != nil {
		if respondAdminBizError(c, err) {
			return
		}
		response.InternalError(c, "Failed to create stock item")
		return
	}

	if h.pluginManager != nil {
		afterPayload := buildVirtualStockHookPayload(stock)
		afterPayload["inventory_name"] = inv.Name
		afterPayload["inventory_type"] = inv.Type
		afterPayload["admin_id"] = adminIDValue
		afterPayload["source"] = "admin_api"
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, stockID uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "virtual_inventory.stock.create.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("virtual_inventory.stock.create.after hook execution failed: admin=%d inventory=%d stock=%d err=%v", adminIDValue, id, stockID, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource":        "virtual_inventory_stock",
			"hook_source":          "admin_api",
			"virtual_inventory_id": strconv.FormatUint(uint64(id), 10),
			"stock_id":             strconv.FormatUint(uint64(stock.ID), 10),
		})), afterPayload, stock.ID)
	}

	response.Success(c, gin.H{
		"message": "Stock item created successfully",
		"stock":   stock,
	})
}

// GetStockList 获取库存项列表
func (h *VirtualInventoryHandler) GetStockList(c *gin.Context) {
	id, err := middleware.GetUintParam(c, "id")
	if err != nil {
		response.BadRequest(c, "Invalid inventory ID")
		return
	}

	page := c.DefaultQuery("page", "1")
	limit := c.DefaultQuery("limit", "20")
	status := c.Query("status")

	pageInt, limitInt := 1, 20
	fmt.Sscanf(page, "%d", &pageInt)
	fmt.Sscanf(limit, "%d", &limitInt)

	if pageInt < 1 {
		pageInt = 1
	}
	if limitInt < 1 || limitInt > 100 {
		limitInt = 20
	}

	stocks, total, err := h.service.ListStocks(id, status, pageInt, limitInt)
	if err != nil {
		response.InternalError(c, "Failed to get stock list")
		return
	}

	response.Paginated(c, stocks, pageInt, limitInt, total)
}

// GetStockStats 获取库存统计
func (h *VirtualInventoryHandler) GetStockStats(c *gin.Context) {
	id, err := middleware.GetUintParam(c, "id")
	if err != nil {
		response.BadRequest(c, "Invalid inventory ID")
		return
	}

	stats, err := h.service.GetStockStats(id)
	if err != nil {
		response.InternalError(c, "Failed to get stock stats")
		return
	}

	response.Success(c, stats)
}

// DeleteStock 删除库存项
func (h *VirtualInventoryHandler) DeleteStock(c *gin.Context) {
	stockID, err := middleware.GetUintParam(c, "stock_id")
	if err != nil {
		response.BadRequest(c, "Invalid stock ID")
		return
	}

	h.handleDeleteStockRequest(c, stockID)
}

// DeleteBatch 删除批次
func (h *VirtualInventoryHandler) DeleteBatch(c *gin.Context) {
	var req struct {
		BatchNo string `json:"batch_no" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request")
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
			log.Printf("virtual_inventory.batch.delete.before payload build failed: admin=%d err=%v", adminIDValue, payloadErr)
		} else {
			hookPayload["admin_id"] = adminIDValue
			hookPayload["source"] = "admin_api"
			hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "virtual_inventory.batch.delete.before",
				Payload: hookPayload,
			}, buildAdminHookExecutionContext(c, adminID, map[string]string{
				"hook_resource": "virtual_inventory_batch",
				"hook_source":   "admin_api",
			}))
			if hookErr != nil {
				log.Printf("virtual_inventory.batch.delete.before hook execution failed: admin=%d err=%v", adminIDValue, hookErr)
			} else if hookResult != nil {
				if hookResult.Blocked {
					reason := strings.TrimSpace(hookResult.BlockReason)
					if reason == "" {
						reason = "Virtual inventory batch deletion rejected by plugin"
					}
					response.BadRequest(c, reason)
					return
				}
				if hookResult.Payload != nil {
					if mergeErr := mergeAdminHookStructPatch(&req, hookResult.Payload); mergeErr != nil {
						log.Printf("virtual_inventory.batch.delete.before payload apply failed, fallback to original request: admin=%d err=%v", adminIDValue, mergeErr)
						req = originalReq
					}
				}
			}
		}
	}

	var batchStocks []models.VirtualProductStock
	if h.db != nil {
		_ = h.db.Where("batch_no = ?", req.BatchNo).Find(&batchStocks).Error
	}
	inventoryIDs := make([]uint, 0)
	seenInventoryIDs := make(map[uint]struct{})
	for _, stock := range batchStocks {
		if _, exists := seenInventoryIDs[stock.VirtualInventoryID]; exists {
			continue
		}
		seenInventoryIDs[stock.VirtualInventoryID] = struct{}{}
		inventoryIDs = append(inventoryIDs, stock.VirtualInventoryID)
	}

	count, err := h.service.DeleteBatch(req.BatchNo)
	if err != nil {
		if respondAdminBizError(c, err) {
			return
		}
		response.InternalError(c, "Operation failed")
		return
	}

	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"batch_no":      req.BatchNo,
			"deleted_count": count,
			"matched_count": len(batchStocks),
			"inventory_ids": inventoryIDs,
			"admin_id":      adminIDValue,
			"source":        "admin_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "virtual_inventory.batch.delete.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("virtual_inventory.batch.delete.after hook execution failed: admin=%d batch=%s err=%v", adminIDValue, req.BatchNo, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "virtual_inventory_batch",
			"hook_source":   "admin_api",
		})), afterPayload)
	}

	response.Success(c, gin.H{
		"message": fmt.Sprintf("Successfully deleted %d items", count),
		"count":   count,
	})
}

// ========== 商品绑定相关 ==========

// GetProductBindings 获取商品的虚拟库存绑定
func (h *VirtualInventoryHandler) GetProductBindings(c *gin.Context) {
	productID, err := middleware.GetUintParam(c, "id")
	if err != nil {
		response.BadRequest(c, "Invalid product ID")
		return
	}

	bindings, err := h.service.GetProductBindings(productID)
	if err != nil {
		response.InternalError(c, "Failed to get bindings")
		return
	}

	response.Success(c, gin.H{"bindings": bindings})
}

// CreateBinding 创建商品-虚拟库存绑定
func (h *VirtualInventoryHandler) CreateBinding(c *gin.Context) {
	productID, err := middleware.GetUintParam(c, "id")
	if err != nil {
		response.BadRequest(c, "Invalid product ID")
		return
	}

	var req struct {
		VirtualInventoryID uint   `json:"virtual_inventory_id" binding:"required"`
		IsRandom           bool   `json:"is_random"`
		Priority           int    `json:"priority"`
		Notes              string `json:"notes"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request data")
		return
	}

	if req.Priority <= 0 {
		req.Priority = 1
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
			log.Printf("virtual_inventory.binding.create.before payload build failed: admin=%d product=%d err=%v", adminIDValue, productID, payloadErr)
		} else {
			hookPayload["product_id"] = productID
			hookPayload["admin_id"] = adminIDValue
			hookPayload["source"] = "admin_api"
			hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "virtual_inventory.binding.create.before",
				Payload: hookPayload,
			}, buildAdminHookExecutionContext(c, adminID, map[string]string{
				"hook_resource": "virtual_inventory_binding",
				"hook_source":   "admin_api",
				"product_id":    strconv.FormatUint(uint64(productID), 10),
			}))
			if hookErr != nil {
				log.Printf("virtual_inventory.binding.create.before hook execution failed: admin=%d product=%d err=%v", adminIDValue, productID, hookErr)
			} else if hookResult != nil {
				if hookResult.Blocked {
					reason := strings.TrimSpace(hookResult.BlockReason)
					if reason == "" {
						reason = "Virtual inventory binding creation rejected by plugin"
					}
					response.BadRequest(c, reason)
					return
				}
				if hookResult.Payload != nil {
					if mergeErr := mergeAdminHookStructPatch(&req, hookResult.Payload); mergeErr != nil {
						log.Printf("virtual_inventory.binding.create.before payload apply failed, fallback to original request: admin=%d product=%d err=%v", adminIDValue, productID, mergeErr)
						req = originalReq
					}
				}
				if req.Priority <= 0 {
					req.Priority = 1
				}
			}
		}
	}

	binding, err := h.service.CreateBinding(productID, req.VirtualInventoryID, req.IsRandom, req.Priority, req.Notes)
	if err != nil {
		if respondAdminBizError(c, err) {
			return
		}
		response.BadRequest(c, err.Error())
		return
	}

	if h.pluginManager != nil && binding != nil {
		afterPayload := buildVirtualInventoryBindingHookPayload(binding)
		afterPayload["admin_id"] = adminIDValue
		afterPayload["source"] = "admin_api"
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, bindingID uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "virtual_inventory.binding.create.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("virtual_inventory.binding.create.after hook execution failed: admin=%d product=%d binding=%d err=%v", adminIDValue, productID, bindingID, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "virtual_inventory_binding",
			"hook_source":   "admin_api",
			"product_id":    strconv.FormatUint(uint64(productID), 10),
			"binding_id":    strconv.FormatUint(uint64(binding.ID), 10),
		})), afterPayload, binding.ID)
	}

	response.Success(c, gin.H{
		"message": "Binding created successfully",
		"binding": binding,
	})
}

// UpdateBinding 更新绑定
func (h *VirtualInventoryHandler) UpdateBinding(c *gin.Context) {
	bindingID, err := middleware.GetUintParam(c, "bindingId")
	if err != nil {
		response.BadRequest(c, "Invalid binding ID")
		return
	}

	var req struct {
		IsRandom bool   `json:"is_random"`
		Priority int    `json:"priority"`
		Notes    string `json:"notes"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request data")
		return
	}
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}
	beforeBinding, _ := h.loadVirtualBinding(bindingID)
	if h.pluginManager != nil {
		originalReq := req
		hookPayload, payloadErr := adminHookStructToPayload(req)
		if payloadErr != nil {
			log.Printf("virtual_inventory.binding.update.before payload build failed: admin=%d binding=%d err=%v", adminIDValue, bindingID, payloadErr)
		} else {
			hookPayload["binding_id"] = bindingID
			if beforeBinding != nil {
				hookPayload["product_id"] = beforeBinding.ProductID
				hookPayload["virtual_inventory_id"] = beforeBinding.VirtualInventoryID
				hookPayload["current"] = buildVirtualInventoryBindingHookPayload(beforeBinding)
			}
			hookPayload["admin_id"] = adminIDValue
			hookPayload["source"] = "admin_api"
			hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "virtual_inventory.binding.update.before",
				Payload: hookPayload,
			}, buildAdminHookExecutionContext(c, adminID, map[string]string{
				"hook_resource": "virtual_inventory_binding",
				"hook_source":   "admin_api",
				"binding_id":    strconv.FormatUint(uint64(bindingID), 10),
			}))
			if hookErr != nil {
				log.Printf("virtual_inventory.binding.update.before hook execution failed: admin=%d binding=%d err=%v", adminIDValue, bindingID, hookErr)
			} else if hookResult != nil {
				if hookResult.Blocked {
					reason := strings.TrimSpace(hookResult.BlockReason)
					if reason == "" {
						reason = "Virtual inventory binding update rejected by plugin"
					}
					response.BadRequest(c, reason)
					return
				}
				if hookResult.Payload != nil {
					if mergeErr := mergeAdminHookStructPatch(&req, hookResult.Payload); mergeErr != nil {
						log.Printf("virtual_inventory.binding.update.before payload apply failed, fallback to original request: admin=%d binding=%d err=%v", adminIDValue, bindingID, mergeErr)
						req = originalReq
					}
				}
			}
		}
	}

	if err := h.service.UpdateBinding(bindingID, req.IsRandom, req.Priority, req.Notes); err != nil {
		if respondAdminBizError(c, err) {
			return
		}
		response.InternalError(c, "Failed to update binding")
		return
	}

	if h.pluginManager != nil {
		updatedBinding := beforeBinding
		if latestBinding, loadErr := h.loadVirtualBinding(bindingID); loadErr == nil {
			updatedBinding = latestBinding
		}
		if updatedBinding != nil {
			afterPayload := buildVirtualInventoryBindingHookPayload(updatedBinding)
			if beforeBinding != nil {
				afterPayload["before_is_random"] = beforeBinding.IsRandom
				afterPayload["before_priority"] = beforeBinding.Priority
				afterPayload["before_notes"] = beforeBinding.Notes
			}
			afterPayload["admin_id"] = adminIDValue
			afterPayload["source"] = "admin_api"
			go func(execCtx *service.ExecutionContext, payload map[string]interface{}) {
				_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
					Hook:    "virtual_inventory.binding.update.after",
					Payload: payload,
				}, execCtx)
				if hookErr != nil {
					log.Printf("virtual_inventory.binding.update.after hook execution failed: admin=%d binding=%d err=%v", adminIDValue, bindingID, hookErr)
				}
			}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
				"hook_resource": "virtual_inventory_binding",
				"hook_source":   "admin_api",
				"binding_id":    strconv.FormatUint(uint64(bindingID), 10),
			})), afterPayload)
		}
	}

	response.Success(c, gin.H{"message": "Binding updated successfully"})
}

// DeleteBinding 删除绑定
func (h *VirtualInventoryHandler) DeleteBinding(c *gin.Context) {
	bindingID, err := middleware.GetUintParam(c, "bindingId")
	if err != nil {
		response.BadRequest(c, "Invalid binding ID")
		return
	}
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}
	beforeBinding, _ := h.loadVirtualBinding(bindingID)
	if h.pluginManager != nil && beforeBinding != nil {
		hookPayload := buildVirtualInventoryBindingHookPayload(beforeBinding)
		hookPayload["admin_id"] = adminIDValue
		hookPayload["source"] = "admin_api"
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "virtual_inventory.binding.delete.before",
			Payload: hookPayload,
		}, buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "virtual_inventory_binding",
			"hook_source":   "admin_api",
			"binding_id":    strconv.FormatUint(uint64(bindingID), 10),
		}))
		if hookErr != nil {
			log.Printf("virtual_inventory.binding.delete.before hook execution failed: admin=%d binding=%d err=%v", adminIDValue, bindingID, hookErr)
		} else if hookResult != nil && hookResult.Blocked {
			reason := strings.TrimSpace(hookResult.BlockReason)
			if reason == "" {
				reason = "Virtual inventory binding deletion rejected by plugin"
			}
			response.BadRequest(c, reason)
			return
		}
	}

	if err := h.service.DeleteBinding(bindingID); err != nil {
		if respondAdminBizError(c, err) {
			return
		}
		response.InternalError(c, "Failed to delete binding")
		return
	}

	if h.pluginManager != nil && beforeBinding != nil {
		afterPayload := buildVirtualInventoryBindingHookPayload(beforeBinding)
		afterPayload["admin_id"] = adminIDValue
		afterPayload["source"] = "admin_api"
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "virtual_inventory.binding.delete.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("virtual_inventory.binding.delete.after hook execution failed: admin=%d binding=%d err=%v", adminIDValue, bindingID, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "virtual_inventory_binding",
			"hook_source":   "admin_api",
			"binding_id":    strconv.FormatUint(uint64(bindingID), 10),
		})), afterPayload)
	}

	response.Success(c, gin.H{"message": "Binding deleted successfully"})
}

// SaveVariantBindings 批量保存商品的规格-虚拟库存绑定（类似实体库存）
func (h *VirtualInventoryHandler) SaveVariantBindings(c *gin.Context) {
	productID, err := middleware.GetUintParam(c, "id")
	if err != nil {
		response.BadRequest(c, "Invalid product ID")
		return
	}

	var req struct {
		Bindings []service.VirtualVariantBindingInput `json:"bindings"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request data")
		return
	}
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}
	var currentBindings []models.ProductVirtualInventoryBinding
	if h.db != nil {
		_ = h.db.Preload("VirtualInventory").Where("product_id = ?", productID).Find(&currentBindings).Error
	}
	if h.pluginManager != nil {
		originalReq := req
		hookPayload, payloadErr := adminHookStructToPayload(req)
		if payloadErr != nil {
			log.Printf("virtual_inventory.binding.save.before payload build failed: admin=%d product=%d err=%v", adminIDValue, productID, payloadErr)
		} else {
			hookPayload["product_id"] = productID
			hookPayload["current_bindings"] = buildVirtualInventoryBindingHookPayloadList(currentBindings)
			hookPayload["admin_id"] = adminIDValue
			hookPayload["source"] = "admin_api"
			hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "virtual_inventory.binding.save.before",
				Payload: hookPayload,
			}, buildAdminHookExecutionContext(c, adminID, map[string]string{
				"hook_resource": "virtual_inventory_binding",
				"hook_source":   "admin_api",
				"product_id":    strconv.FormatUint(uint64(productID), 10),
			}))
			if hookErr != nil {
				log.Printf("virtual_inventory.binding.save.before hook execution failed: admin=%d product=%d err=%v", adminIDValue, productID, hookErr)
			} else if hookResult != nil {
				if hookResult.Blocked {
					reason := strings.TrimSpace(hookResult.BlockReason)
					if reason == "" {
						reason = "Virtual inventory binding save rejected by plugin"
					}
					response.BadRequest(c, reason)
					return
				}
				if hookResult.Payload != nil {
					if mergeErr := mergeAdminHookStructPatch(&req, hookResult.Payload); mergeErr != nil {
						log.Printf("virtual_inventory.binding.save.before payload apply failed, fallback to original request: admin=%d product=%d err=%v", adminIDValue, productID, mergeErr)
						req = originalReq
					}
				}
			}
		}
	}

	result, err := h.service.SaveVariantBindings(productID, req.Bindings)
	if err != nil {
		if respondAdminBizError(c, err) {
			return
		}
		response.InternalError(c, "Operation failed")
		return
	}

	batchErrors := make([]VirtualBindingBatchError, 0, len(result.Errors))
	for _, batchErr := range result.Errors {
		batchErrors = append(batchErrors, buildVirtualBindingBatchError(batchErr))
	}

	message := fmt.Sprintf("Deleted %d old bindings, successfully created %d/%d virtual bindings", result.DeletedCount, len(result.Created), len(req.Bindings))
	if len(req.Bindings) == 0 {
		message = fmt.Sprintf("Deleted %d old bindings", result.DeletedCount)
	}

	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"product_id":      productID,
			"deleted_count":   result.DeletedCount,
			"created_count":   len(result.Created),
			"created":         buildVirtualInventoryBindingHookPayloadList(result.Created),
			"errors":          batchErrors,
			"requested_count": len(req.Bindings),
			"admin_id":        adminIDValue,
			"source":          "admin_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "virtual_inventory.binding.save.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("virtual_inventory.binding.save.after hook execution failed: admin=%d product=%d err=%v", adminIDValue, productID, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "virtual_inventory_binding",
			"hook_source":   "admin_api",
			"product_id":    strconv.FormatUint(uint64(productID), 10),
		})), afterPayload)
	}

	response.Success(c, gin.H{
		"deleted_count": result.DeletedCount,
		"created_count": len(result.Created),
		"created":       result.Created,
		"errors":        batchErrors,
		"message":       message,
	})
}

// GetInventoryProducts 获取虚拟库存绑定的商品
func (h *VirtualInventoryHandler) GetInventoryProducts(c *gin.Context) {
	id, err := middleware.GetUintParam(c, "id")
	if err != nil {
		response.BadRequest(c, "Invalid inventory ID")
		return
	}

	bindings, err := h.service.GetInventoryProducts(id)
	if err != nil {
		response.InternalError(c, "Failed to get products")
		return
	}

	response.Success(c, gin.H{"products": bindings})
}

// ========== 基于产品ID的虚拟库存管理（兼容前端API） ==========

// GetStockListForProduct 获取商品的虚拟库存列表
func (h *VirtualInventoryHandler) GetStockListForProduct(c *gin.Context) {
	productID, err := middleware.GetUintParam(c, "id")
	if err != nil {
		response.BadRequest(c, "Invalid product ID")
		return
	}

	page := c.DefaultQuery("page", "1")
	limit := c.DefaultQuery("limit", "20")
	status := c.Query("status")

	pageInt, limitInt := 1, 20
	fmt.Sscanf(page, "%d", &pageInt)
	fmt.Sscanf(limit, "%d", &limitInt)

	if pageInt < 1 {
		pageInt = 1
	}
	if limitInt < 1 || limitInt > 100 {
		limitInt = 20
	}

	stocks, total, err := h.service.ListStocksForProduct(productID, status, pageInt, limitInt)
	if err != nil {
		response.InternalError(c, "Failed to get stock list")
		return
	}

	response.Paginated(c, stocks, pageInt, limitInt, total)
}

// GetStockStatsForProduct 获取商品的虚拟库存统计
func (h *VirtualInventoryHandler) GetStockStatsForProduct(c *gin.Context) {
	productID, err := middleware.GetUintParam(c, "id")
	if err != nil {
		response.BadRequest(c, "Invalid product ID")
		return
	}

	stats, err := h.service.GetStockStatsForProduct(productID)
	if err != nil {
		response.InternalError(c, "Failed to get stock stats")
		return
	}

	response.Success(c, stats)
}

// ImportStockForProduct 为商品导入虚拟库存
func (h *VirtualInventoryHandler) ImportStockForProduct(c *gin.Context) {
	productID, err := middleware.GetUintParam(c, "id")
	if err != nil {
		response.BadRequest(c, "Invalid product ID")
		return
	}

	// 检查商品是否有绑定的虚拟库存
	binding, err := h.service.GetFirstBindingForProduct(productID)
	if err != nil {
		if respondAdminBizError(c, err) {
			return
		}
		response.InternalError(c, "Failed to load product virtual inventory binding")
		return
	}

	// 检查绑定的库存是否为脚本类型
	inv, err := h.service.GetVirtualInventory(binding.VirtualInventoryID)
	if err != nil {
		response.InternalError(c, "Failed to get virtual inventory")
		return
	}
	if inv.Type == models.VirtualInventoryTypeScript {
		response.BizError(
			c,
			"Script type inventory does not support manual stock import",
			"virtual_inventory.manualImportUnsupported",
			nil,
		)
		return
	}

	h.handleImportStockRequest(c, binding.VirtualInventoryID, &productID)
}

// DeleteStockByID 删除单个库存项（通用API）
func (h *VirtualInventoryHandler) DeleteStockByID(c *gin.Context) {
	stockID, err := middleware.GetUintParam(c, "id")
	if err != nil {
		response.BadRequest(c, "Invalid stock ID")
		return
	}

	h.handleDeleteStockRequest(c, stockID)
}

// ReserveStock 手动预留库存项
func (h *VirtualInventoryHandler) ReserveStock(c *gin.Context) {
	stockID, err := middleware.GetUintParam(c, "stock_id")
	if err != nil {
		response.BadRequest(c, "Invalid stock ID")
		return
	}

	var req struct {
		Remark string `json:"remark"`
	}
	c.ShouldBindJSON(&req)
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}
	beforeStock, _ := h.loadVirtualStock(stockID)
	if h.pluginManager != nil {
		originalReq := req
		hookPayload, payloadErr := adminHookStructToPayload(req)
		if payloadErr != nil {
			log.Printf("virtual_inventory.stock.reserve.manual.before payload build failed: admin=%d stock=%d err=%v", adminIDValue, stockID, payloadErr)
		} else {
			hookPayload["stock_id"] = stockID
			if beforeStock != nil {
				hookPayload["virtual_inventory_id"] = beforeStock.VirtualInventoryID
				hookPayload["current"] = buildVirtualStockHookPayload(beforeStock)
			}
			hookPayload["admin_id"] = adminIDValue
			hookPayload["source"] = "admin_api"
			hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "virtual_inventory.stock.reserve.manual.before",
				Payload: hookPayload,
			}, buildAdminHookExecutionContext(c, adminID, map[string]string{
				"hook_resource": "virtual_inventory_stock",
				"hook_source":   "admin_api",
				"stock_id":      strconv.FormatUint(uint64(stockID), 10),
			}))
			if hookErr != nil {
				log.Printf("virtual_inventory.stock.reserve.manual.before hook execution failed: admin=%d stock=%d err=%v", adminIDValue, stockID, hookErr)
			} else if hookResult != nil {
				if hookResult.Blocked {
					reason := strings.TrimSpace(hookResult.BlockReason)
					if reason == "" {
						reason = "Virtual inventory stock reserve rejected by plugin"
					}
					response.BadRequest(c, reason)
					return
				}
				if hookResult.Payload != nil {
					if mergeErr := mergeAdminHookStructPatch(&req, hookResult.Payload); mergeErr != nil {
						log.Printf("virtual_inventory.stock.reserve.manual.before payload apply failed, fallback to original request: admin=%d stock=%d err=%v", adminIDValue, stockID, mergeErr)
						req = originalReq
					}
				}
			}
		}
	}

	if err := h.service.ManualReserveStock(stockID, req.Remark); err != nil {
		if respondAdminBizError(c, err) {
			return
		}
		response.BadRequest(c, err.Error())
		return
	}

	if h.pluginManager != nil {
		afterStock := beforeStock
		if latestStock, loadErr := h.loadVirtualStock(stockID); loadErr == nil {
			afterStock = latestStock
		}
		if afterStock != nil {
			afterPayload := buildVirtualStockHookPayload(afterStock)
			afterPayload["remark"] = req.Remark
			if beforeStock != nil {
				afterPayload["before_status"] = beforeStock.Status
				afterPayload["before_order_no"] = beforeStock.OrderNo
			}
			afterPayload["admin_id"] = adminIDValue
			afterPayload["source"] = "admin_api"
			go func(execCtx *service.ExecutionContext, payload map[string]interface{}) {
				_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
					Hook:    "virtual_inventory.stock.reserve.manual.after",
					Payload: payload,
				}, execCtx)
				if hookErr != nil {
					log.Printf("virtual_inventory.stock.reserve.manual.after hook execution failed: admin=%d stock=%d err=%v", adminIDValue, stockID, hookErr)
				}
			}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
				"hook_resource": "virtual_inventory_stock",
				"hook_source":   "admin_api",
				"stock_id":      strconv.FormatUint(uint64(stockID), 10),
			})), afterPayload)
		}
	}

	response.Success(c, gin.H{"message": "Stock item reserved successfully"})
}

// ReleaseStockItem 手动释放库存项
func (h *VirtualInventoryHandler) ReleaseStockItem(c *gin.Context) {
	stockID, err := middleware.GetUintParam(c, "stock_id")
	if err != nil {
		response.BadRequest(c, "Invalid stock ID")
		return
	}
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}
	beforeStock, _ := h.loadVirtualStock(stockID)
	if h.pluginManager != nil && beforeStock != nil {
		hookPayload := buildVirtualStockHookPayload(beforeStock)
		hookPayload["admin_id"] = adminIDValue
		hookPayload["source"] = "admin_api"
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "virtual_inventory.stock.release.manual.before",
			Payload: hookPayload,
		}, buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "virtual_inventory_stock",
			"hook_source":   "admin_api",
			"stock_id":      strconv.FormatUint(uint64(stockID), 10),
		}))
		if hookErr != nil {
			log.Printf("virtual_inventory.stock.release.manual.before hook execution failed: admin=%d stock=%d err=%v", adminIDValue, stockID, hookErr)
		} else if hookResult != nil && hookResult.Blocked {
			reason := strings.TrimSpace(hookResult.BlockReason)
			if reason == "" {
				reason = "Virtual inventory stock release rejected by plugin"
			}
			response.BadRequest(c, reason)
			return
		}
	}

	if err := h.service.ManualReleaseStock(stockID); err != nil {
		if respondAdminBizError(c, err) {
			return
		}
		response.BadRequest(c, err.Error())
		return
	}

	if h.pluginManager != nil {
		afterStock := beforeStock
		if latestStock, loadErr := h.loadVirtualStock(stockID); loadErr == nil {
			afterStock = latestStock
		}
		if afterStock != nil {
			afterPayload := buildVirtualStockHookPayload(afterStock)
			if beforeStock != nil {
				afterPayload["before_status"] = beforeStock.Status
				afterPayload["before_order_no"] = beforeStock.OrderNo
			}
			afterPayload["admin_id"] = adminIDValue
			afterPayload["source"] = "admin_api"
			go func(execCtx *service.ExecutionContext, payload map[string]interface{}) {
				_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
					Hook:    "virtual_inventory.stock.release.manual.after",
					Payload: payload,
				}, execCtx)
				if hookErr != nil {
					log.Printf("virtual_inventory.stock.release.manual.after hook execution failed: admin=%d stock=%d err=%v", adminIDValue, stockID, hookErr)
				}
			}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
				"hook_resource": "virtual_inventory_stock",
				"hook_source":   "admin_api",
				"stock_id":      strconv.FormatUint(uint64(stockID), 10),
			})), afterPayload)
		}
	}

	response.Success(c, gin.H{"message": "Stock item released successfully"})
}
