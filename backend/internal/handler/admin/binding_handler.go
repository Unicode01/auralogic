package admin

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"auralogic/internal/pkg/response"
	"auralogic/internal/service"
)

type BindingHandler struct {
	bindingService *service.BindingService
}

func NewBindingHandler(bindingService *service.BindingService) *BindingHandler {
	return &BindingHandler{
		bindingService: bindingService,
	}
}

// CreateBindingRequest Create绑定请求
type CreateBindingRequest struct {
	InventoryID uint   `json:"inventory_id" binding:"required"`
	IsRandom    bool   `json:"is_random"`
	Priority    int    `json:"priority" binding:"min=0"`
	Notes       string `json:"notes"`
}

// UpdateBindingRequest Update绑定请求
type UpdateBindingRequest struct {
	IsRandom bool   `json:"is_random"`
	Priority int    `json:"priority" binding:"min=0"`
	Notes    string `json:"notes"`
}

// CreateBinding CreateProduct-Inventory绑定
func (h *BindingHandler) CreateBinding(c *gin.Context) {
	productID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid product ID format")
		return
	}

	var req CreateBindingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	// 默认权重为1
	if req.Priority == 0 {
		req.Priority = 1
	}

	binding, err := h.bindingService.CreateBinding(
		uint(productID),
		req.InventoryID,
		req.IsRandom,
		req.Priority,
		req.Notes,
	)

	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, binding)
}

// BatchCreateBindingsRequest 批量Create绑定请求
type BatchCreateBindingsRequest struct {
	Bindings []CreateBindingRequest `json:"bindings" binding:"required"`
}

// BatchCreateBindings 批量CreateProduct-Inventory绑定
func (h *BindingHandler) BatchCreateBindings(c *gin.Context) {
	productID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid product ID format")
		return
	}

	var req BatchCreateBindingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	// 批量Create绑定
	var createdBindings []interface{}
	var errors []string

	for i, bindingReq := range req.Bindings {
		// 默认权重为1
		if bindingReq.Priority == 0 {
			bindingReq.Priority = 1
		}

		binding, err := h.bindingService.CreateBinding(
			uint(productID),
			bindingReq.InventoryID,
			bindingReq.IsRandom,
			bindingReq.Priority,
			bindingReq.Notes,
		)

		if err != nil {
			errors = append(errors, fmt.Sprintf("Binding %d failed: %s", i+1, err.Error()))
		} else {
			createdBindings = append(createdBindings, binding)
		}
	}

	// 如果有Error，返回部分Success的结果
	if len(errors) > 0 {
		response.Success(c, gin.H{
			"created": createdBindings,
			"errors":  errors,
			"message": fmt.Sprintf("Successfully created %d/%d bindings", len(createdBindings), len(req.Bindings)),
		})
	} else {
		response.Success(c, gin.H{
			"created": createdBindings,
			"message": fmt.Sprintf("Successfully created %d bindings", len(createdBindings)),
		})
	}
}

// GetProductBindings getProduct的所有Inventory绑定
func (h *BindingHandler) GetProductBindings(c *gin.Context) {
	productID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid product ID format")
		return
	}

	bindings, err := h.bindingService.GetProductBindings(uint(productID))
	if err != nil {
		c.Error(err)
		response.InternalError(c, "Query failed")
		return
	}

	response.Success(c, bindings)
}

// UpdateBinding Update绑定关系
func (h *BindingHandler) UpdateBinding(c *gin.Context) {
	bindingID, err := strconv.ParseUint(c.Param("bindingId"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid binding ID format")
		return
	}

	var req UpdateBindingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	if err := h.bindingService.UpdateBinding(
		uint(bindingID),
		req.IsRandom,
		req.Priority,
		req.Notes,
	); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, gin.H{"message": "UpdateSuccess"})
}

// DeleteBinding Delete绑定关系
func (h *BindingHandler) DeleteBinding(c *gin.Context) {
	bindingID, err := strconv.ParseUint(c.Param("bindingId"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid binding ID format")
		return
	}

	if err := h.bindingService.DeleteBinding(uint(bindingID)); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, gin.H{"message": "DeleteSuccess"})
}

// DeleteAllProductBindings DeleteProduct的所有绑定关系（批量Delete）
func (h *BindingHandler) DeleteAllProductBindings(c *gin.Context) {
	productID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid product ID format")
		return
	}

	count, err := h.bindingService.DeleteAllProductBindings(uint(productID))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, gin.H{
		"message": "Batch delete successful",
		"count":   count,
	})
}

// ReplaceProductBindings 替换Product的所有绑定关系（先Delete所有，再批量Create）
func (h *BindingHandler) ReplaceProductBindings(c *gin.Context) {
	productID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid product ID format")
		return
	}

	var req BatchCreateBindingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	// 1. Delete所有旧绑定
	deletedCount, err := h.bindingService.DeleteAllProductBindings(uint(productID))
	if err != nil {
		response.BadRequest(c, "Failed to delete old bindings")
		return
	}

	// 2. 批量Create新绑定（如果bindings为空，只Delete不Create）
	var createdBindings []interface{}
	var errors []string

	if len(req.Bindings) > 0 {
		for i, bindingReq := range req.Bindings {
			// 默认权重为1
			if bindingReq.Priority == 0 {
				bindingReq.Priority = 1
			}

			binding, err := h.bindingService.CreateBinding(
				uint(productID),
				bindingReq.InventoryID,
				bindingReq.IsRandom,
				bindingReq.Priority,
				bindingReq.Notes,
			)

			if err != nil {
				errors = append(errors, fmt.Sprintf("Binding %d failed: %s", i+1, err.Error()))
			} else {
				createdBindings = append(createdBindings, binding)
			}
		}
	}

	// 返回结果
	var message string
	if len(req.Bindings) == 0 {
		message = fmt.Sprintf("Deleted %d old bindings", deletedCount)
	} else {
		message = fmt.Sprintf("Deleted %d old bindings, successfully created %d/%d new bindings", deletedCount, len(createdBindings), len(req.Bindings))
	}

	response.Success(c, gin.H{
		"deleted_count": deletedCount,
		"created_count": len(createdBindings),
		"created":       createdBindings,
		"errors":        errors,
		"message":       message,
	})
}

// GetInventoryProducts getInventory绑定的所有Product
func (h *BindingHandler) GetInventoryProducts(c *gin.Context) {
	inventoryID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid inventory ID format")
		return
	}

	bindings, err := h.bindingService.GetInventoryBindings(uint(inventoryID))
	if err != nil {
		c.Error(err)
		response.InternalError(c, "Query failed")
		return
	}

	response.Success(c, bindings)
}
