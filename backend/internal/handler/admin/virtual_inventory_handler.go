package admin

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"auralogic/internal/middleware"
	"auralogic/internal/models"
	"auralogic/internal/pkg/response"
	"auralogic/internal/service"
)

type VirtualInventoryHandler struct {
	service *service.VirtualInventoryService
}

func NewVirtualInventoryHandler(service *service.VirtualInventoryService) *VirtualInventoryHandler {
	return &VirtualInventoryHandler{
		service: service,
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

	inventories, total, err := h.service.ListVirtualInventories(pageInt, limitInt, search)
	if err != nil {
		response.InternalError(c, "Failed to get virtual inventories")
		return
	}

	response.Success(c, gin.H{
		"list":  inventories,
		"total": total,
		"page":  pageInt,
		"limit": limitInt,
	})
}

// CreateVirtualInventory 创建虚拟库存
func (h *VirtualInventoryHandler) CreateVirtualInventory(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		SKU         string `json:"sku"`
		Description string `json:"description"`
		IsActive    bool   `json:"is_active"`
		Notes       string `json:"notes"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request data")
		return
	}

	inventory := &models.VirtualInventory{
		Name:        req.Name,
		SKU:         req.SKU,
		Description: req.Description,
		IsActive:    req.IsActive,
		Notes:       req.Notes,
	}

	if err := h.service.CreateVirtualInventory(inventory); err != nil {
		response.InternalError(c, "Failed to create virtual inventory")
		return
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
		Name        string `json:"name"`
		SKU         string `json:"sku"`
		Description string `json:"description"`
		IsActive    *bool  `json:"is_active"`
		Notes       string `json:"notes"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request data")
		return
	}

	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.SKU != "" {
		updates["sku"] = req.SKU
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if req.Notes != "" {
		updates["notes"] = req.Notes
	}

	if err := h.service.UpdateVirtualInventory(id, updates); err != nil {
		response.InternalError(c, "Failed to update virtual inventory")
		return
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

	if err := h.service.DeleteVirtualInventory(id); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, gin.H{"message": "Virtual inventory deleted successfully"})
}

// ImportStock 导入库存项
func (h *VirtualInventoryHandler) ImportStock(c *gin.Context) {
	id, err := middleware.GetUintParam(c, "id")
	if err != nil {
		response.BadRequest(c, "Invalid inventory ID")
		return
	}

	// 检查库存是否存在
	_, err = h.service.GetVirtualInventory(id)
	if err != nil {
		response.NotFound(c, "Virtual inventory not found")
		return
	}

	importType := c.PostForm("import_type") // file, text
	importedBy := c.GetString("user_email")

	var count int
	var importErr error

	switch importType {
	case "file":
		// 文件上传
		file, err := c.FormFile("file")
		if err != nil {
			response.BadRequest(c, "File upload failed")
			return
		}

		count, importErr = h.handleFileImport(id, file, importedBy)

	case "text":
		// 文本内容
		content := c.PostForm("content")
		if content == "" {
			response.BadRequest(c, "Content cannot be empty")
			return
		}

		count, importErr = h.service.ImportFromText(id, content, importedBy)

	default:
		response.BadRequest(c, "Invalid import type")
		return
	}

	if importErr != nil {
		response.InternalError(c, fmt.Sprintf("Import failed: %v", importErr))
		return
	}

	response.Success(c, gin.H{
		"message": fmt.Sprintf("Successfully imported %d items", count),
		"count":   count,
	})
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
		return 0, fmt.Errorf("unsupported file type: %s", ext)
	}
}

// CreateStockManually 手动创建单个库存项
func (h *VirtualInventoryHandler) CreateStockManually(c *gin.Context) {
	id, err := middleware.GetUintParam(c, "id")
	if err != nil {
		response.BadRequest(c, "Invalid inventory ID")
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

	importedBy := c.GetString("user_email")

	stock, err := h.service.CreateStockManually(id, req.Content, req.Remark, importedBy)
	if err != nil {
		response.InternalError(c, "Failed to create stock item")
		return
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

	stocks, total, err := h.service.ListStocks(id, status, pageInt, limitInt)
	if err != nil {
		response.InternalError(c, "Failed to get stock list")
		return
	}

	response.Success(c, gin.H{
		"list":  stocks,
		"total": total,
		"page":  pageInt,
		"limit": limitInt,
	})
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

	if err := h.service.DeleteStock(stockID); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, gin.H{"message": "Stock item deleted successfully"})
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

	count, err := h.service.DeleteBatch(req.BatchNo)
	if err != nil {
		response.InternalError(c, err.Error())
		return
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

	binding, err := h.service.CreateBinding(productID, req.VirtualInventoryID, req.IsRandom, req.Priority, req.Notes)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
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

	if err := h.service.UpdateBinding(bindingID, req.IsRandom, req.Priority, req.Notes); err != nil {
		response.InternalError(c, "Failed to update binding")
		return
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

	if err := h.service.DeleteBinding(bindingID); err != nil {
		response.InternalError(c, "Failed to delete binding")
		return
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
		Bindings []struct {
			Attributes         map[string]string `json:"attributes"`
			VirtualInventoryID *uint             `json:"virtual_inventory_id"`
			IsRandom           bool              `json:"is_random"`
			Priority           int               `json:"priority"`
		} `json:"bindings"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request data")
		return
	}

	if err := h.service.SaveVariantBindings(productID, req.Bindings); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, gin.H{"message": "Bindings saved successfully"})
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

	stocks, total, err := h.service.ListStocksForProduct(productID, status, pageInt, limitInt)
	if err != nil {
		response.InternalError(c, "Failed to get stock list")
		return
	}

	response.Success(c, gin.H{
		"list":  stocks,
		"total": total,
		"page":  pageInt,
		"limit": limitInt,
	})
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
		response.BadRequest(c, "No virtual inventory bound to this product. Please bind a virtual inventory first.")
		return
	}

	importType := c.PostForm("import_type") // file, text
	importedBy := c.GetString("user_email")

	var count int
	var importErr error

	switch importType {
	case "file":
		// 文件上传
		file, err := c.FormFile("file")
		if err != nil {
			response.BadRequest(c, "File upload failed")
			return
		}

		count, importErr = h.handleFileImport(binding.VirtualInventoryID, file, importedBy)

	case "text":
		// 文本内容
		content := c.PostForm("content")
		if content == "" {
			response.BadRequest(c, "Content cannot be empty")
			return
		}

		count, importErr = h.service.ImportFromText(binding.VirtualInventoryID, content, importedBy)

	default:
		response.BadRequest(c, "Invalid import type")
		return
	}

	if importErr != nil {
		response.InternalError(c, fmt.Sprintf("Import failed: %v", importErr))
		return
	}

	response.Success(c, gin.H{
		"message": fmt.Sprintf("Successfully imported %d items", count),
		"count":   count,
	})
}

// DeleteStockByID 删除单个库存项（通用API）
func (h *VirtualInventoryHandler) DeleteStockByID(c *gin.Context) {
	stockID, err := middleware.GetUintParam(c, "id")
	if err != nil {
		response.BadRequest(c, "Invalid stock ID")
		return
	}

	if err := h.service.DeleteStock(stockID); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, gin.H{"message": "Stock item deleted successfully"})
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

	if err := h.service.ManualReserveStock(stockID, req.Remark); err != nil {
		response.BadRequest(c, err.Error())
		return
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

	if err := h.service.ManualReleaseStock(stockID); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, gin.H{"message": "Stock item released successfully"})
}
