package admin

import (
	"log"
	"strconv"
	"strings"

	"auralogic/internal/config"
	"auralogic/internal/middleware"
	"auralogic/internal/models"
	"auralogic/internal/pkg/response"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// PaymentMethodHandler 付款方式管理处理器
type PaymentMethodHandler struct {
	db            *gorm.DB
	service       *service.PaymentMethodService
	pluginManager *service.PluginManagerService
}

// NewPaymentMethodHandler 创建付款方式处理器
func NewPaymentMethodHandler(db *gorm.DB, cfg *config.Config, pluginManager *service.PluginManagerService) *PaymentMethodHandler {
	return &PaymentMethodHandler{
		db:            db,
		service:       service.NewPaymentMethodService(db, cfg),
		pluginManager: pluginManager,
	}
}

// List 获取所有付款方式
func (h *PaymentMethodHandler) List(c *gin.Context) {
	enabledOnly := c.Query("enabled_only") == "true"
	methods, err := h.service.List(enabledOnly)
	if err != nil {
		response.InternalError(c, "Failed to get payment method list")
		return
	}
	response.Success(c, gin.H{"items": methods})
}

// Get 获取单个付款方式
func (h *PaymentMethodHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}

	pm, err := h.service.Get(uint(id))
	if err != nil {
		response.NotFound(c, "Payment method not found")
		return
	}
	response.Success(c, pm)
}

// CreateRequest 创建付款方式请求
type CreatePaymentMethodRequest struct {
	Name         string `json:"name" binding:"required"`
	Description  string `json:"description"`
	Type         string `json:"type" binding:"required,oneof=builtin custom"`
	Icon         string `json:"icon"`
	Script       string `json:"script"`
	Config       string `json:"config"`
	PollInterval int    `json:"poll_interval"`
}

// Create 创建付款方式
func (h *PaymentMethodHandler) Create(c *gin.Context) {
	var req CreatePaymentMethodRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}
	adminID, _ := middleware.GetUserID(c)
	desiredEnabled := true

	if h.pluginManager != nil {
		hookExecCtx := buildAdminHookExecutionContext(c, &adminID, map[string]string{
			"resource_type": "payment_method",
		})
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook: "payment.method.create.before",
			Payload: map[string]interface{}{
				"source":        "admin_api",
				"name":          req.Name,
				"description":   req.Description,
				"icon":          req.Icon,
				"script":        req.Script,
				"config":        req.Config,
				"poll_interval": req.PollInterval,
				"enabled":       desiredEnabled,
			},
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("payment.method.create.before hook execution failed: name=%s err=%v", req.Name, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Payment method creation rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if value, exists := hookResult.Payload["name"]; exists {
					req.Name = parseStringFromAny(value)
				}
				if value, exists := hookResult.Payload["description"]; exists {
					req.Description = parseStringFromAny(value)
				}
				if value, exists := hookResult.Payload["icon"]; exists {
					req.Icon = parseStringFromAny(value)
				}
				if value, exists := hookResult.Payload["script"]; exists {
					req.Script = parseStringFromAny(value)
				}
				if value, exists := hookResult.Payload["config"]; exists {
					req.Config = parseStringFromAny(value)
				}
				if value, exists := hookResult.Payload["poll_interval"]; exists {
					req.PollInterval = parseIntFromAny(value, req.PollInterval)
				}
				if value, exists := hookResult.Payload["enabled"]; exists {
					desiredEnabled = parseBoolFromAny(value, desiredEnabled)
				}
			}
		}
	}

	if models.PaymentMethodType(req.Type) != models.PaymentMethodTypeCustom {
		response.BadRequest(c, "Built-in payment methods must be initialized through package governance")
		return
	}

	method, err := h.service.CreateLegacyPaymentMethod(service.LegacyPaymentMethodUpsertInput{
		Name:         &req.Name,
		Description:  &req.Description,
		Icon:         &req.Icon,
		Script:       &req.Script,
		Config:       &req.Config,
		PollInterval: &req.PollInterval,
	})
	if err != nil {
		h.respondPaymentMethodMarketError(c, err)
		return
	}
	if !desiredEnabled && method != nil && method.ID > 0 && method.Enabled {
		if updateErr := h.service.Update(method.ID, map[string]interface{}{"enabled": false}); updateErr != nil {
			h.respondPaymentMethodMarketError(c, updateErr)
			return
		}
		method.Enabled = false
	}
	if h.pluginManager != nil && method != nil {
		afterPayload := map[string]interface{}{
			"source":            "admin_api",
			"payment_method_id": method.ID,
			"name":              method.Name,
			"description":       method.Description,
			"icon":              method.Icon,
			"poll_interval":     method.PollInterval,
			"enabled":           method.Enabled,
			"created_by":        adminID,
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, methodID uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "payment.method.create.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("payment.method.create.after hook execution failed: payment_method_id=%d err=%v", methodID, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, &adminID, map[string]string{
			"resource_type": "payment_method",
			"resource_id":   strconv.FormatUint(uint64(method.ID), 10),
		})), afterPayload, method.ID)
	}

	response.Success(c, method)
}

// UpdateRequest 更新付款方式请求
type UpdatePaymentMethodRequest struct {
	Name         *string `json:"name"`
	Description  *string `json:"description"`
	Icon         *string `json:"icon"`
	Script       *string `json:"script"`
	Config       *string `json:"config"`
	Enabled      *bool   `json:"enabled"`
	PollInterval *int    `json:"poll_interval"`
}

// Update 更新付款方式
func (h *PaymentMethodHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}

	var req UpdatePaymentMethodRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}
	adminID, _ := middleware.GetUserID(c)

	if h.pluginManager != nil {
		hookExecCtx := buildAdminHookExecutionContext(c, &adminID, map[string]string{
			"resource_type": "payment_method",
			"resource_id":   strconv.FormatUint(id, 10),
		})
		payload := map[string]interface{}{
			"source": "admin_api",
		}
		if req.Name != nil {
			payload["name"] = *req.Name
		}
		if req.Description != nil {
			payload["description"] = *req.Description
		}
		if req.Icon != nil {
			payload["icon"] = *req.Icon
		}
		if req.Script != nil {
			payload["script"] = *req.Script
		}
		if req.Config != nil {
			payload["config"] = *req.Config
		}
		if req.Enabled != nil {
			payload["enabled"] = *req.Enabled
		}
		if req.PollInterval != nil {
			payload["poll_interval"] = *req.PollInterval
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "payment.method.update.before",
			Payload: payload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("payment.method.update.before hook execution failed: payment_method_id=%d err=%v", id, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Payment method update rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if value, exists := hookResult.Payload["name"]; exists {
					text := parseStringFromAny(value)
					req.Name = &text
				}
				if value, exists := hookResult.Payload["description"]; exists {
					text := parseStringFromAny(value)
					req.Description = &text
				}
				if value, exists := hookResult.Payload["icon"]; exists {
					text := parseStringFromAny(value)
					req.Icon = &text
				}
				if value, exists := hookResult.Payload["script"]; exists {
					text := parseStringFromAny(value)
					req.Script = &text
				}
				if value, exists := hookResult.Payload["config"]; exists {
					text := parseStringFromAny(value)
					req.Config = &text
				}
				if value, exists := hookResult.Payload["enabled"]; exists {
					enabled := parseBoolFromAny(value, req.Enabled != nil && *req.Enabled)
					req.Enabled = &enabled
				}
				if value, exists := hookResult.Payload["poll_interval"]; exists {
					pollInterval := parseIntFromAny(value, 0)
					req.PollInterval = &pollInterval
				}
			}
		}
	}

	var updatedMethod *models.PaymentMethod
	if req.Name != nil ||
		req.Description != nil ||
		req.Icon != nil ||
		req.Script != nil ||
		req.Config != nil ||
		req.PollInterval != nil {
		method, err := h.service.UpdateLegacyPaymentMethod(uint(id), service.LegacyPaymentMethodUpsertInput{
			Name:         req.Name,
			Description:  req.Description,
			Icon:         req.Icon,
			Script:       req.Script,
			Config:       req.Config,
			PollInterval: req.PollInterval,
			Enabled:      req.Enabled,
		})
		if err != nil {
			h.respondPaymentMethodMarketError(c, err)
			return
		}
		updatedMethod = method
	} else {
		updates := make(map[string]interface{})
		if req.Enabled != nil {
			updates["enabled"] = *req.Enabled
		}

		if err := h.service.Update(uint(id), updates); err != nil {
			response.InternalError(c, "Failed to update payment method")
			return
		}

		pm, _ := h.service.Get(uint(id))
		updatedMethod = pm
	}
	if h.pluginManager != nil && updatedMethod != nil {
		afterPayload := map[string]interface{}{
			"source":            "admin_api",
			"payment_method_id": updatedMethod.ID,
			"name":              updatedMethod.Name,
			"description":       updatedMethod.Description,
			"icon":              updatedMethod.Icon,
			"poll_interval":     updatedMethod.PollInterval,
			"enabled":           updatedMethod.Enabled,
			"updated_by":        adminID,
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, methodID uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "payment.method.update.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("payment.method.update.after hook execution failed: payment_method_id=%d err=%v", methodID, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, &adminID, map[string]string{
			"resource_type": "payment_method",
			"resource_id":   strconv.FormatUint(uint64(updatedMethod.ID), 10),
		})), afterPayload, updatedMethod.ID)
	}

	response.Success(c, updatedMethod)
}

// Delete 删除付款方式
func (h *PaymentMethodHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}
	adminID, _ := middleware.GetUserID(c)
	existing, getErr := h.service.Get(uint(id))
	if getErr != nil {
		response.NotFound(c, "Payment method not found")
		return
	}
	if h.pluginManager != nil {
		hookExecCtx := buildAdminHookExecutionContext(c, &adminID, map[string]string{
			"resource_type": "payment_method",
			"resource_id":   strconv.FormatUint(id, 10),
		})
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook: "payment.method.delete.before",
			Payload: map[string]interface{}{
				"source":            "admin_api",
				"payment_method_id": existing.ID,
				"name":              existing.Name,
				"enabled":           existing.Enabled,
			},
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("payment.method.delete.before hook execution failed: payment_method_id=%d err=%v", id, hookErr)
		} else if hookResult != nil && hookResult.Blocked {
			reason := strings.TrimSpace(hookResult.BlockReason)
			if reason == "" {
				reason = "Payment method deletion rejected by plugin"
			}
			response.BadRequest(c, reason)
			return
		}
	}

	if err := h.service.Delete(uint(id)); err != nil {
		response.InternalError(c, "Failed to delete payment method")
		return
	}
	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"source":            "admin_api",
			"payment_method_id": existing.ID,
			"name":              existing.Name,
			"enabled":           existing.Enabled,
			"deleted_by":        adminID,
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, methodID uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "payment.method.delete.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("payment.method.delete.after hook execution failed: payment_method_id=%d err=%v", methodID, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, &adminID, map[string]string{
			"resource_type": "payment_method",
			"resource_id":   strconv.FormatUint(id, 10),
		})), afterPayload, existing.ID)
	}

	response.Success(c, nil)
}

// ToggleEnabled 切换启用状态
func (h *PaymentMethodHandler) ToggleEnabled(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}
	adminID, _ := middleware.GetUserID(c)
	existing, getErr := h.service.Get(uint(id))
	if getErr != nil {
		response.NotFound(c, "Payment method not found")
		return
	}
	targetEnabled := !existing.Enabled
	if h.pluginManager != nil {
		hookExecCtx := buildAdminHookExecutionContext(c, &adminID, map[string]string{
			"resource_type": "payment_method",
			"resource_id":   strconv.FormatUint(id, 10),
		})
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook: "payment.method.enable.before",
			Payload: map[string]interface{}{
				"source":            "admin_api",
				"payment_method_id": existing.ID,
				"name":              existing.Name,
				"enabled":           targetEnabled,
				"current_enabled":   existing.Enabled,
			},
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("payment.method.enable.before hook execution failed: payment_method_id=%d err=%v", id, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Payment method toggle rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if value, exists := hookResult.Payload["enabled"]; exists {
					targetEnabled = parseBoolFromAny(value, targetEnabled)
				}
			}
		}
	}

	if err := h.service.Update(uint(id), map[string]interface{}{"enabled": targetEnabled}); err != nil {
		response.InternalError(c, "Failed to toggle status")
		return
	}

	pm, _ := h.service.Get(uint(id))
	if h.pluginManager != nil && pm != nil {
		afterPayload := map[string]interface{}{
			"source":            "admin_api",
			"payment_method_id": pm.ID,
			"name":              pm.Name,
			"enabled":           pm.Enabled,
			"updated_by":        adminID,
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, methodID uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "payment.method.enable.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("payment.method.enable.after hook execution failed: payment_method_id=%d err=%v", methodID, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, &adminID, map[string]string{
			"resource_type": "payment_method",
			"resource_id":   strconv.FormatUint(uint64(pm.ID), 10),
		})), afterPayload, pm.ID)
	}
	response.Success(c, pm)
}

// ReorderRequest 重排序请求
type ReorderPaymentMethodRequest struct {
	IDs []uint `json:"ids" binding:"required"`
}

// Reorder 重新排序
func (h *PaymentMethodHandler) Reorder(c *gin.Context) {
	var req ReorderPaymentMethodRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}
	adminID, _ := middleware.GetUserID(c)

	if err := h.service.Reorder(req.IDs); err != nil {
		response.InternalError(c, "Reorder failed")
		return
	}
	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"source":     "admin_api",
			"ids":        append([]uint(nil), req.IDs...),
			"updated_by": adminID,
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "payment.method.reorder.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("payment.method.reorder.after hook execution failed: err=%v", hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, &adminID, map[string]string{
			"resource_type": "payment_method",
		})), afterPayload)
	}

	response.Success(c, nil)
}

// TestScriptRequest 测试脚本请求
type TestScriptRequest struct {
	Script string                 `json:"script" binding:"required"`
	Config map[string]interface{} `json:"config"`
}

// TestScript 测试JS脚本
func (h *PaymentMethodHandler) TestScript(c *gin.Context) {
	var req TestScriptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}
	adminID, _ := middleware.GetUserID(c)
	if h.pluginManager != nil {
		hookExecCtx := buildAdminHookExecutionContext(c, &adminID, map[string]string{
			"resource_type": "payment_method",
		})
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook: "payment.method.test.before",
			Payload: map[string]interface{}{
				"source": "admin_api",
				"script": req.Script,
				"config": req.Config,
			},
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("payment.method.test.before hook execution failed: err=%v", hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Payment method test rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if value, exists := hookResult.Payload["script"]; exists {
					req.Script = parseStringFromAny(value)
				}
				if value, exists := hookResult.Payload["config"]; exists {
					req.Config = asStringAnyMap(value)
				}
			}
		}
	}

	result, err := h.service.TestScript(req.Script, req.Config)
	if err != nil {
		response.BadRequest(c, "Script execution failed")
		return
	}
	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"source":     "admin_api",
			"updated_by": adminID,
			"result":     result,
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "payment.method.test.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("payment.method.test.after hook execution failed: err=%v", hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, &adminID, map[string]string{
			"resource_type": "payment_method",
		})), afterPayload)
	}

	response.Success(c, result)
}

// InitBuiltinMethods 初始化内置付款方式
func (h *PaymentMethodHandler) InitBuiltinMethods(c *gin.Context) {
	if err := h.service.InitBuiltinPaymentMethods(); err != nil {
		response.InternalError(c, "Initialization failed")
		return
	}
	response.Success(c, gin.H{"message": "Built-in payment methods initialized"})
}
