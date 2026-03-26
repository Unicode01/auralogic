package admin

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	"auralogic/internal/models"
	"auralogic/internal/pkg/bizerr"
	"auralogic/internal/pkg/response"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type BindingHandler struct {
	bindingService *service.BindingService
	db             *gorm.DB
	pluginManager  *service.PluginManagerService
}

func NewBindingHandler(bindingService *service.BindingService, db *gorm.DB, pluginManager *service.PluginManagerService) *BindingHandler {
	return &BindingHandler{
		bindingService: bindingService,
		db:             db,
		pluginManager:  pluginManager,
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

func (h *BindingHandler) loadBinding(bindingID uint) (*models.ProductInventoryBinding, error) {
	if h.db == nil {
		return nil, gorm.ErrInvalidDB
	}

	var binding models.ProductInventoryBinding
	if err := h.db.Preload("Inventory").First(&binding, bindingID).Error; err != nil {
		return nil, err
	}
	return &binding, nil
}

func buildInventoryBindingHookPayload(binding *models.ProductInventoryBinding) map[string]interface{} {
	if binding == nil {
		return map[string]interface{}{}
	}

	payload := map[string]interface{}{
		"binding_id":      binding.ID,
		"product_id":      binding.ProductID,
		"inventory_id":    binding.InventoryID,
		"attributes":      binding.Attributes,
		"attributes_hash": binding.AttributesHash,
		"is_random":       binding.IsRandom,
		"priority":        binding.Priority,
		"notes":           binding.Notes,
		"created_at":      binding.CreatedAt,
		"updated_at":      binding.UpdatedAt,
	}
	if binding.Inventory != nil {
		payload["inventory_name"] = binding.Inventory.Name
		payload["inventory_sku"] = binding.Inventory.SKU
		payload["inventory_active"] = binding.Inventory.IsActive
	}
	return payload
}

func buildInventoryBindingHookPayloadList(bindings []models.ProductInventoryBinding) []map[string]interface{} {
	payloads := make([]map[string]interface{}, 0, len(bindings))
	for idx := range bindings {
		payloads = append(payloads, buildInventoryBindingHookPayload(&bindings[idx]))
	}
	return payloads
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
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}
	if h.pluginManager != nil {
		originalReq := req
		hookPayload, payloadErr := adminHookStructToPayload(req)
		if payloadErr != nil {
			log.Printf("inventory.binding.create.before payload build failed: admin=%d product=%d err=%v", adminIDValue, uint(productID), payloadErr)
		} else {
			hookPayload["product_id"] = uint(productID)
			hookPayload["admin_id"] = adminIDValue
			hookPayload["source"] = "admin_api"
			hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "inventory.binding.create.before",
				Payload: hookPayload,
			}, buildAdminHookExecutionContext(c, adminID, map[string]string{
				"hook_resource": "inventory_binding",
				"hook_source":   "admin_api",
				"product_id":    strconv.FormatUint(productID, 10),
			}))
			if hookErr != nil {
				log.Printf("inventory.binding.create.before hook execution failed: admin=%d product=%d err=%v", adminIDValue, uint(productID), hookErr)
			} else if hookResult != nil {
				if hookResult.Blocked {
					reason := strings.TrimSpace(hookResult.BlockReason)
					if reason == "" {
						reason = "Inventory binding creation rejected by plugin"
					}
					response.BadRequest(c, reason)
					return
				}
				if hookResult.Payload != nil {
					if mergeErr := mergeAdminHookStructPatch(&req, hookResult.Payload); mergeErr != nil {
						log.Printf("inventory.binding.create.before payload apply failed, fallback to original request: admin=%d product=%d err=%v", adminIDValue, uint(productID), mergeErr)
						req = originalReq
					}
				}
				if req.Priority == 0 {
					req.Priority = 1
				}
			}
		}
	}

	binding, err := h.bindingService.CreateBinding(
		uint(productID),
		req.InventoryID,
		req.IsRandom,
		req.Priority,
		req.Notes,
	)

	if err != nil {
		if respondAdminBizError(c, err) {
			return
		}
		response.BadRequest(c, err.Error())
		return
	}

	if h.pluginManager != nil && binding != nil {
		afterPayload := buildInventoryBindingHookPayload(binding)
		afterPayload["admin_id"] = adminIDValue
		afterPayload["source"] = "admin_api"
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, productIDValue uint, bindingID uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "inventory.binding.create.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("inventory.binding.create.after hook execution failed: admin=%d product=%d binding=%d err=%v", adminIDValue, productIDValue, bindingID, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "inventory_binding",
			"hook_source":   "admin_api",
			"product_id":    strconv.FormatUint(productID, 10),
			"binding_id":    strconv.FormatUint(uint64(binding.ID), 10),
		})), afterPayload, uint(productID), binding.ID)
	}

	response.Success(c, binding)
}

// BatchCreateBindingsRequest 批量Create绑定请求
type BatchCreateBindingsRequest struct {
	Bindings []CreateBindingRequest `json:"bindings" binding:"required"`
}

type BindingBatchError struct {
	Index       int                    `json:"index"`
	InventoryID uint                   `json:"inventory_id,omitempty"`
	ErrorKey    string                 `json:"error_key,omitempty"`
	Message     string                 `json:"message"`
	Params      map[string]interface{} `json:"params,omitempty"`
}

func buildBindingBatchError(index int, req CreateBindingRequest, err error) BindingBatchError {
	batchError := BindingBatchError{
		Index:       index,
		InventoryID: req.InventoryID,
		Message:     err.Error(),
	}

	var bizErr *bizerr.Error
	if errors.As(err, &bizErr) {
		batchError.ErrorKey = bizErr.Key
		batchError.Message = bizErr.Message
		batchError.Params = bizErr.Params
	}

	return batchError
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
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}
	if h.pluginManager != nil {
		originalReq := req
		hookPayload, payloadErr := adminHookStructToPayload(req)
		if payloadErr != nil {
			log.Printf("inventory.binding.batch_create.before payload build failed: admin=%d product=%d err=%v", adminIDValue, uint(productID), payloadErr)
		} else {
			currentBindings, listErr := h.bindingService.GetProductBindings(uint(productID))
			if listErr == nil {
				hookPayload["current_bindings"] = buildInventoryBindingHookPayloadList(currentBindings)
			}
			hookPayload["product_id"] = uint(productID)
			hookPayload["admin_id"] = adminIDValue
			hookPayload["source"] = "admin_api"
			hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "inventory.binding.batch_create.before",
				Payload: hookPayload,
			}, buildAdminHookExecutionContext(c, adminID, map[string]string{
				"hook_resource": "inventory_binding",
				"hook_source":   "admin_api",
				"product_id":    strconv.FormatUint(productID, 10),
			}))
			if hookErr != nil {
				log.Printf("inventory.binding.batch_create.before hook execution failed: admin=%d product=%d err=%v", adminIDValue, uint(productID), hookErr)
			} else if hookResult != nil {
				if hookResult.Blocked {
					reason := strings.TrimSpace(hookResult.BlockReason)
					if reason == "" {
						reason = "Inventory binding batch creation rejected by plugin"
					}
					response.BadRequest(c, reason)
					return
				}
				if hookResult.Payload != nil {
					if mergeErr := mergeAdminHookStructPatch(&req, hookResult.Payload); mergeErr != nil {
						log.Printf("inventory.binding.batch_create.before payload apply failed, fallback to original request: admin=%d product=%d err=%v", adminIDValue, uint(productID), mergeErr)
						req = originalReq
					}
				}
			}
		}
	}

	// 批量Create绑定
	var createdBindings []interface{}
	createdPayloads := make([]map[string]interface{}, 0, len(req.Bindings))
	var batchErrors []BindingBatchError

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
			batchErrors = append(batchErrors, buildBindingBatchError(i+1, bindingReq, err))
		} else {
			createdBindings = append(createdBindings, binding)
			createdPayloads = append(createdPayloads, buildInventoryBindingHookPayload(binding))
		}
	}

	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"product_id":      uint(productID),
			"requested_count": len(req.Bindings),
			"created_count":   len(createdPayloads),
			"created":         createdPayloads,
			"errors":          batchErrors,
			"admin_id":        adminIDValue,
			"source":          "admin_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "inventory.binding.batch_create.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("inventory.binding.batch_create.after hook execution failed: admin=%d product=%d err=%v", adminIDValue, uint(productID), hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "inventory_binding",
			"hook_source":   "admin_api",
			"product_id":    strconv.FormatUint(productID, 10),
		})), afterPayload)
	}

	// 如果有Error，返回部分Success的结果
	if len(batchErrors) > 0 {
		response.Success(c, gin.H{
			"created":       createdBindings,
			"created_count": len(createdBindings),
			"errors":        batchErrors,
			"message":       fmt.Sprintf("Successfully created %d/%d bindings", len(createdBindings), len(req.Bindings)),
		})
	} else {
		response.Success(c, gin.H{
			"created":       createdBindings,
			"created_count": len(createdBindings),
			"errors":        []BindingBatchError{},
			"message":       fmt.Sprintf("Successfully created %d bindings", len(createdBindings)),
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
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}
	beforeBinding, _ := h.loadBinding(uint(bindingID))
	if h.pluginManager != nil {
		originalReq := req
		hookPayload, payloadErr := adminHookStructToPayload(req)
		if payloadErr != nil {
			log.Printf("inventory.binding.update.before payload build failed: admin=%d binding=%d err=%v", adminIDValue, uint(bindingID), payloadErr)
		} else {
			hookPayload["binding_id"] = uint(bindingID)
			if beforeBinding != nil {
				hookPayload["product_id"] = beforeBinding.ProductID
				hookPayload["inventory_id"] = beforeBinding.InventoryID
				hookPayload["current"] = buildInventoryBindingHookPayload(beforeBinding)
			}
			hookPayload["admin_id"] = adminIDValue
			hookPayload["source"] = "admin_api"
			hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "inventory.binding.update.before",
				Payload: hookPayload,
			}, buildAdminHookExecutionContext(c, adminID, map[string]string{
				"hook_resource": "inventory_binding",
				"hook_source":   "admin_api",
				"binding_id":    strconv.FormatUint(bindingID, 10),
			}))
			if hookErr != nil {
				log.Printf("inventory.binding.update.before hook execution failed: admin=%d binding=%d err=%v", adminIDValue, uint(bindingID), hookErr)
			} else if hookResult != nil {
				if hookResult.Blocked {
					reason := strings.TrimSpace(hookResult.BlockReason)
					if reason == "" {
						reason = "Inventory binding update rejected by plugin"
					}
					response.BadRequest(c, reason)
					return
				}
				if hookResult.Payload != nil {
					if mergeErr := mergeAdminHookStructPatch(&req, hookResult.Payload); mergeErr != nil {
						log.Printf("inventory.binding.update.before payload apply failed, fallback to original request: admin=%d binding=%d err=%v", adminIDValue, uint(bindingID), mergeErr)
						req = originalReq
					}
				}
			}
		}
	}

	if err := h.bindingService.UpdateBinding(
		uint(bindingID),
		req.IsRandom,
		req.Priority,
		req.Notes,
	); err != nil {
		if respondAdminBizError(c, err) {
			return
		}
		response.BadRequest(c, err.Error())
		return
	}

	if h.pluginManager != nil {
		updatedBinding := beforeBinding
		if latestBinding, loadErr := h.loadBinding(uint(bindingID)); loadErr == nil {
			updatedBinding = latestBinding
		}
		if updatedBinding != nil {
			afterPayload := buildInventoryBindingHookPayload(updatedBinding)
			if beforeBinding != nil {
				afterPayload["before_is_random"] = beforeBinding.IsRandom
				afterPayload["before_priority"] = beforeBinding.Priority
				afterPayload["before_notes"] = beforeBinding.Notes
			}
			afterPayload["admin_id"] = adminIDValue
			afterPayload["source"] = "admin_api"
			go func(execCtx *service.ExecutionContext, payload map[string]interface{}) {
				_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
					Hook:    "inventory.binding.update.after",
					Payload: payload,
				}, execCtx)
				if hookErr != nil {
					log.Printf("inventory.binding.update.after hook execution failed: admin=%d binding=%d err=%v", adminIDValue, uint(bindingID), hookErr)
				}
			}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
				"hook_resource": "inventory_binding",
				"hook_source":   "admin_api",
				"binding_id":    strconv.FormatUint(bindingID, 10),
			})), afterPayload)
		}
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
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}
	beforeBinding, _ := h.loadBinding(uint(bindingID))
	if h.pluginManager != nil && beforeBinding != nil {
		hookPayload := buildInventoryBindingHookPayload(beforeBinding)
		hookPayload["admin_id"] = adminIDValue
		hookPayload["source"] = "admin_api"
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "inventory.binding.delete.before",
			Payload: hookPayload,
		}, buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "inventory_binding",
			"hook_source":   "admin_api",
			"binding_id":    strconv.FormatUint(bindingID, 10),
		}))
		if hookErr != nil {
			log.Printf("inventory.binding.delete.before hook execution failed: admin=%d binding=%d err=%v", adminIDValue, uint(bindingID), hookErr)
		} else if hookResult != nil && hookResult.Blocked {
			reason := strings.TrimSpace(hookResult.BlockReason)
			if reason == "" {
				reason = "Inventory binding deletion rejected by plugin"
			}
			response.BadRequest(c, reason)
			return
		}
	}

	if err := h.bindingService.DeleteBinding(uint(bindingID)); err != nil {
		if respondAdminBizError(c, err) {
			return
		}
		response.BadRequest(c, err.Error())
		return
	}

	if h.pluginManager != nil && beforeBinding != nil {
		afterPayload := buildInventoryBindingHookPayload(beforeBinding)
		afterPayload["admin_id"] = adminIDValue
		afterPayload["source"] = "admin_api"
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "inventory.binding.delete.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("inventory.binding.delete.after hook execution failed: admin=%d binding=%d err=%v", adminIDValue, uint(bindingID), hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "inventory_binding",
			"hook_source":   "admin_api",
			"binding_id":    strconv.FormatUint(bindingID, 10),
		})), afterPayload)
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
		if respondAdminBizError(c, err) {
			return
		}
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
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}
	currentBindings, _ := h.bindingService.GetProductBindings(uint(productID))
	if h.pluginManager != nil {
		originalReq := req
		hookPayload, payloadErr := adminHookStructToPayload(req)
		if payloadErr != nil {
			log.Printf("inventory.binding.replace.before payload build failed: admin=%d product=%d err=%v", adminIDValue, uint(productID), payloadErr)
		} else {
			hookPayload["product_id"] = uint(productID)
			hookPayload["current_bindings"] = buildInventoryBindingHookPayloadList(currentBindings)
			hookPayload["admin_id"] = adminIDValue
			hookPayload["source"] = "admin_api"
			hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "inventory.binding.replace.before",
				Payload: hookPayload,
			}, buildAdminHookExecutionContext(c, adminID, map[string]string{
				"hook_resource": "inventory_binding",
				"hook_source":   "admin_api",
				"product_id":    strconv.FormatUint(productID, 10),
			}))
			if hookErr != nil {
				log.Printf("inventory.binding.replace.before hook execution failed: admin=%d product=%d err=%v", adminIDValue, uint(productID), hookErr)
			} else if hookResult != nil {
				if hookResult.Blocked {
					reason := strings.TrimSpace(hookResult.BlockReason)
					if reason == "" {
						reason = "Inventory binding replacement rejected by plugin"
					}
					response.BadRequest(c, reason)
					return
				}
				if hookResult.Payload != nil {
					if mergeErr := mergeAdminHookStructPatch(&req, hookResult.Payload); mergeErr != nil {
						log.Printf("inventory.binding.replace.before payload apply failed, fallback to original request: admin=%d product=%d err=%v", adminIDValue, uint(productID), mergeErr)
						req = originalReq
					}
				}
			}
		}
	}

	// 1. Delete所有旧绑定
	deletedCount, err := h.bindingService.DeleteAllProductBindings(uint(productID))
	if err != nil {
		if respondAdminBizError(c, err) {
			return
		}
		response.BadRequest(c, "Failed to delete old bindings")
		return
	}

	// 2. 批量Create新绑定（如果bindings为空，只Delete不Create）
	var createdBindings []interface{}
	createdPayloads := make([]map[string]interface{}, 0, len(req.Bindings))
	var batchErrors []BindingBatchError

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
				batchErrors = append(batchErrors, buildBindingBatchError(i+1, bindingReq, err))
			} else {
				createdBindings = append(createdBindings, binding)
				createdPayloads = append(createdPayloads, buildInventoryBindingHookPayload(binding))
			}
		}
	}

	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"product_id":      uint(productID),
			"deleted_count":   deletedCount,
			"requested_count": len(req.Bindings),
			"created_count":   len(createdPayloads),
			"created":         createdPayloads,
			"errors":          batchErrors,
			"admin_id":        adminIDValue,
			"source":          "admin_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "inventory.binding.replace.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("inventory.binding.replace.after hook execution failed: admin=%d product=%d err=%v", adminIDValue, uint(productID), hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "inventory_binding",
			"hook_source":   "admin_api",
			"product_id":    strconv.FormatUint(productID, 10),
		})), afterPayload)
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
		"errors":        batchErrors,
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
