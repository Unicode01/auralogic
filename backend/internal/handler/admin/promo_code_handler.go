package admin

import (
	"log"
	"strconv"
	"strings"
	"time"

	"auralogic/internal/models"
	"auralogic/internal/pkg/response"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
)

type PromoCodeHandler struct {
	promoCodeService *service.PromoCodeService
	pluginManager    *service.PluginManagerService
}

func NewPromoCodeHandler(promoCodeService *service.PromoCodeService, pluginManager *service.PluginManagerService) *PromoCodeHandler {
	return &PromoCodeHandler{
		promoCodeService: promoCodeService,
		pluginManager:    pluginManager,
	}
}

// CreatePromoCodeRequest 创建优惠码请求
type CreatePromoCodeRequest struct {
	Code                string              `json:"code" binding:"required"`
	Name                string              `json:"name" binding:"required"`
	Description         string              `json:"description"`
	DiscountType        models.DiscountType `json:"discount_type" binding:"required"`
	DiscountValueMinor  int64               `json:"discount_value_minor" binding:"required,gt=0"`
	MaxDiscountMinor    int64               `json:"max_discount_minor"`
	MinOrderAmountMinor int64               `json:"min_order_amount_minor"`
	TotalQuantity       int                 `json:"total_quantity"`
	ProductIDs          []uint              `json:"product_ids"`
	ProductScope        string              `json:"product_scope"`
	Status              string              `json:"status"`
	ExpiresAt           *string             `json:"expires_at"`
}

// UpdatePromoCodeRequest 更新优惠码请求（不需要code字段）
type UpdatePromoCodeRequest struct {
	Name                string              `json:"name" binding:"required"`
	Description         string              `json:"description"`
	DiscountType        models.DiscountType `json:"discount_type" binding:"required"`
	DiscountValueMinor  int64               `json:"discount_value_minor" binding:"required,gt=0"`
	MaxDiscountMinor    int64               `json:"max_discount_minor"`
	MinOrderAmountMinor int64               `json:"min_order_amount_minor"`
	TotalQuantity       int                 `json:"total_quantity"`
	ProductIDs          []uint              `json:"product_ids"`
	ProductScope        string              `json:"product_scope"`
	Status              string              `json:"status"`
	ExpiresAt           *string             `json:"expires_at"`
}

func parsePromoCodeExpiryInput(value *string) (*time.Time, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}

	t, err := time.Parse(time.RFC3339, *value)
	if err != nil {
		t, err = time.Parse("2006-01-02T15:04:05", *value)
		if err != nil {
			t, err = time.Parse("2006-01-02", *value)
			if err != nil {
				return nil, err
			}
			t = t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
		}
	}
	return &t, nil
}

func buildPromoCodeAdminHookPayload(promoCode *models.PromoCode) map[string]interface{} {
	if promoCode == nil {
		return map[string]interface{}{}
	}

	return map[string]interface{}{
		"promo_code_id":          promoCode.ID,
		"code":                   promoCode.Code,
		"name":                   promoCode.Name,
		"description":            promoCode.Description,
		"discount_type":          promoCode.DiscountType,
		"discount_value_minor":   promoCode.DiscountValue,
		"max_discount_minor":     promoCode.MaxDiscount,
		"min_order_amount_minor": promoCode.MinOrderAmount,
		"total_quantity":         promoCode.TotalQuantity,
		"used_quantity":          promoCode.UsedQuantity,
		"reserved_quantity":      promoCode.ReservedQuantity,
		"product_ids":            promoCode.ProductIDs,
		"product_scope":          promoCode.ProductScope,
		"status":                 promoCode.Status,
		"expires_at":             promoCode.ExpiresAt,
		"created_at":             promoCode.CreatedAt,
		"updated_at":             promoCode.UpdatedAt,
	}
}

// CreatePromoCode 创建优惠码
func (h *PromoCodeHandler) CreatePromoCode(c *gin.Context) {
	var req CreatePromoCodeRequest
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
			log.Printf("promo.admin.create.before payload build failed: admin=%d err=%v", adminIDValue, payloadErr)
		} else {
			hookPayload["admin_id"] = adminIDValue
			hookPayload["source"] = "admin_api"
			hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "promo.admin.create.before",
				Payload: hookPayload,
			}, buildAdminHookExecutionContext(c, adminID, map[string]string{
				"hook_resource": "promo_code",
				"hook_source":   "admin_api",
			}))
			if hookErr != nil {
				log.Printf("promo.admin.create.before hook execution failed: admin=%d err=%v", adminIDValue, hookErr)
			} else if hookResult != nil {
				if hookResult.Blocked {
					reason := strings.TrimSpace(hookResult.BlockReason)
					if reason == "" {
						reason = "Promo code creation rejected by plugin"
					}
					response.BadRequest(c, reason)
					return
				}
				if hookResult.Payload != nil {
					if mergeErr := mergeAdminHookStructPatch(&req, hookResult.Payload); mergeErr != nil {
						log.Printf("promo.admin.create.before payload apply failed, fallback to original request: admin=%d err=%v", adminIDValue, mergeErr)
						req = originalReq
					}
				}
			}
		}
	}

	promoCode := &models.PromoCode{
		Code:           req.Code,
		Name:           req.Name,
		Description:    req.Description,
		DiscountType:   req.DiscountType,
		DiscountValue:  req.DiscountValueMinor,
		MaxDiscount:    req.MaxDiscountMinor,
		MinOrderAmount: req.MinOrderAmountMinor,
		TotalQuantity:  req.TotalQuantity,
		ProductIDs:     req.ProductIDs,
		ProductScope:   req.ProductScope,
		Status:         models.PromoCodeStatusActive,
	}

	if req.Status != "" {
		promoCode.Status = models.PromoCodeStatus(req.Status)
	}

	if expiresAt, err := parsePromoCodeExpiryInput(req.ExpiresAt); err != nil {
		response.BadRequest(c, "Invalid expiry date format")
		return
	} else {
		promoCode.ExpiresAt = expiresAt
	}

	if err := h.promoCodeService.Create(promoCode); err != nil {
		if respondAdminBizError(c, err) {
			return
		}
		response.BadRequest(c, err.Error())
		return
	}

	if h.pluginManager != nil {
		afterPayload := buildPromoCodeAdminHookPayload(promoCode)
		afterPayload["admin_id"] = adminIDValue
		afterPayload["source"] = "admin_api"
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, promoCodeID uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "promo.admin.create.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("promo.admin.create.after hook execution failed: admin=%d promo=%d err=%v", adminIDValue, promoCodeID, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "promo_code",
			"hook_source":   "admin_api",
			"promo_code_id": strconv.FormatUint(uint64(promoCode.ID), 10),
		})), afterPayload, promoCode.ID)
	}

	response.Success(c, promoCode)
}

// GetPromoCode 获取优惠码详情
func (h *PromoCodeHandler) GetPromoCode(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}

	promoCode, err := h.promoCodeService.GetByID(uint(id))
	if err != nil {
		response.NotFound(c, "Promo code not found")
		return
	}

	response.Success(c, promoCode)
}

// UpdatePromoCode 更新优惠码
func (h *PromoCodeHandler) UpdatePromoCode(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}
	currentPromoCode, err := h.promoCodeService.GetByID(uint(id))
	if err != nil {
		response.NotFound(c, "Promo code not found")
		return
	}

	var req UpdatePromoCodeRequest
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
			log.Printf("promo.admin.update.before payload build failed: admin=%d promo=%d err=%v", adminIDValue, uint(id), payloadErr)
		} else {
			hookPayload["promo_code_id"] = uint(id)
			hookPayload["current"] = buildPromoCodeAdminHookPayload(currentPromoCode)
			hookPayload["admin_id"] = adminIDValue
			hookPayload["source"] = "admin_api"
			hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "promo.admin.update.before",
				Payload: hookPayload,
			}, buildAdminHookExecutionContext(c, adminID, map[string]string{
				"hook_resource": "promo_code",
				"hook_source":   "admin_api",
				"promo_code_id": strconv.FormatUint(id, 10),
			}))
			if hookErr != nil {
				log.Printf("promo.admin.update.before hook execution failed: admin=%d promo=%d err=%v", adminIDValue, uint(id), hookErr)
			} else if hookResult != nil {
				if hookResult.Blocked {
					reason := strings.TrimSpace(hookResult.BlockReason)
					if reason == "" {
						reason = "Promo code update rejected by plugin"
					}
					response.BadRequest(c, reason)
					return
				}
				if hookResult.Payload != nil {
					if mergeErr := mergeAdminHookStructPatch(&req, hookResult.Payload); mergeErr != nil {
						log.Printf("promo.admin.update.before payload apply failed, fallback to original request: admin=%d promo=%d err=%v", adminIDValue, uint(id), mergeErr)
						req = originalReq
					}
				}
			}
		}
	}

	updates := &models.PromoCode{
		Name:           req.Name,
		Description:    req.Description,
		DiscountType:   req.DiscountType,
		DiscountValue:  req.DiscountValueMinor,
		MaxDiscount:    req.MaxDiscountMinor,
		MinOrderAmount: req.MinOrderAmountMinor,
		TotalQuantity:  req.TotalQuantity,
		ProductIDs:     req.ProductIDs,
		ProductScope:   req.ProductScope,
		Status:         models.PromoCodeStatusActive,
	}

	if req.Status != "" {
		updates.Status = models.PromoCodeStatus(req.Status)
	}

	if expiresAt, err := parsePromoCodeExpiryInput(req.ExpiresAt); err != nil {
		response.BadRequest(c, "Invalid expiry date format")
		return
	} else {
		updates.ExpiresAt = expiresAt
	}

	if err := h.promoCodeService.Update(uint(id), updates); err != nil {
		if respondAdminBizError(c, err) {
			return
		}
		response.BadRequest(c, err.Error())
		return
	}

	if h.pluginManager != nil {
		updatedPromoCode, reloadErr := h.promoCodeService.GetByID(uint(id))
		if reloadErr != nil {
			log.Printf("promo.admin.update.after reload failed: admin=%d promo=%d err=%v", adminIDValue, uint(id), reloadErr)
		} else {
			afterPayload := buildPromoCodeAdminHookPayload(updatedPromoCode)
			afterPayload["admin_id"] = adminIDValue
			afterPayload["source"] = "admin_api"
			go func(execCtx *service.ExecutionContext, payload map[string]interface{}, promoCodeID uint) {
				_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
					Hook:    "promo.admin.update.after",
					Payload: payload,
				}, execCtx)
				if hookErr != nil {
					log.Printf("promo.admin.update.after hook execution failed: admin=%d promo=%d err=%v", adminIDValue, promoCodeID, hookErr)
				}
			}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
				"hook_resource": "promo_code",
				"hook_source":   "admin_api",
				"promo_code_id": strconv.FormatUint(id, 10),
			})), afterPayload, updatedPromoCode.ID)
		}
	}

	response.Success(c, gin.H{"message": "updated"})
}

// DeletePromoCode 删除优惠码
func (h *PromoCodeHandler) DeletePromoCode(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}
	promoCode, err := h.promoCodeService.GetByID(uint(id))
	if err != nil {
		response.NotFound(c, "Promo code not found")
		return
	}
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}
	if h.pluginManager != nil {
		hookPayload := buildPromoCodeAdminHookPayload(promoCode)
		hookPayload["admin_id"] = adminIDValue
		hookPayload["source"] = "admin_api"
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "promo.admin.delete.before",
			Payload: hookPayload,
		}, buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "promo_code",
			"hook_source":   "admin_api",
			"promo_code_id": strconv.FormatUint(id, 10),
		}))
		if hookErr != nil {
			log.Printf("promo.admin.delete.before hook execution failed: admin=%d promo=%d err=%v", adminIDValue, uint(id), hookErr)
		} else if hookResult != nil && hookResult.Blocked {
			reason := strings.TrimSpace(hookResult.BlockReason)
			if reason == "" {
				reason = "Promo code deletion rejected by plugin"
			}
			response.BadRequest(c, reason)
			return
		}
	}

	if err := h.promoCodeService.Delete(uint(id)); err != nil {
		if respondAdminBizError(c, err) {
			return
		}
		response.BadRequest(c, err.Error())
		return
	}

	if h.pluginManager != nil {
		afterPayload := buildPromoCodeAdminHookPayload(promoCode)
		afterPayload["admin_id"] = adminIDValue
		afterPayload["source"] = "admin_api"
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, promoCodeID uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "promo.admin.delete.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("promo.admin.delete.after hook execution failed: admin=%d promo=%d err=%v", adminIDValue, promoCodeID, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "promo_code",
			"hook_source":   "admin_api",
			"promo_code_id": strconv.FormatUint(id, 10),
		})), afterPayload, promoCode.ID)
	}

	response.Success(c, gin.H{"message": "deleted"})
}

// ListPromoCodes 列表
func (h *PromoCodeHandler) ListPromoCodes(c *gin.Context) {
	page, limit := response.GetPagination(c)
	status := c.Query("status")
	search := c.Query("search")

	promoCodes, total, err := h.promoCodeService.List(page, limit, status, search)
	if err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	response.Paginated(c, promoCodes, page, limit, total)
}
