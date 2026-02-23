package admin

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"auralogic/internal/models"
	"auralogic/internal/pkg/response"
	"auralogic/internal/service"
	"gorm.io/gorm"
)

// PaymentMethodHandler 付款方式管理处理器
type PaymentMethodHandler struct {
	service *service.PaymentMethodService
}

// NewPaymentMethodHandler 创建付款方式处理器
func NewPaymentMethodHandler(db *gorm.DB) *PaymentMethodHandler {
	return &PaymentMethodHandler{
		service: service.NewPaymentMethodService(db),
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
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Type        string `json:"type" binding:"required,oneof=builtin custom"`
	Icon        string `json:"icon"`
	Script      string `json:"script"`
	Config      string `json:"config"`
}

// Create 创建付款方式
func (h *PaymentMethodHandler) Create(c *gin.Context) {
	var req CreatePaymentMethodRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	pm := &models.PaymentMethod{
		Name:        req.Name,
		Description: req.Description,
		Type:        models.PaymentMethodType(req.Type),
		Icon:        req.Icon,
		Script:      req.Script,
		Config:      req.Config,
		Enabled:     true,
	}

	if err := h.service.Create(pm); err != nil {
		response.InternalError(c, "Failed to create payment method")
		return
	}

	response.Success(c, pm)
}

// UpdateRequest 更新付款方式请求
type UpdatePaymentMethodRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Icon        *string `json:"icon"`
	Script      *string `json:"script"`
	Config      *string `json:"config"`
	Enabled     *bool   `json:"enabled"`
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

	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Icon != nil {
		updates["icon"] = *req.Icon
	}
	if req.Script != nil {
		updates["script"] = *req.Script
	}
	if req.Config != nil {
		updates["config"] = *req.Config
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}

	if err := h.service.Update(uint(id), updates); err != nil {
		response.InternalError(c, "Failed to update payment method")
		return
	}

	pm, _ := h.service.Get(uint(id))
	response.Success(c, pm)
}

// Delete 删除付款方式
func (h *PaymentMethodHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}

	if err := h.service.Delete(uint(id)); err != nil {
		response.InternalError(c, "Failed to delete payment method")
		return
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

	if err := h.service.ToggleEnabled(uint(id)); err != nil {
		response.InternalError(c, "Failed to toggle status")
		return
	}

	pm, _ := h.service.Get(uint(id))
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

	if err := h.service.Reorder(req.IDs); err != nil {
		response.InternalError(c, "Reorder failed")
		return
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

	result, err := h.service.TestScript(req.Script, req.Config)
	if err != nil {
		response.BadRequest(c, "Script execution failed")
		return
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
