package user

import (
	"encoding/json"
	"log"
	"strconv"
	"strings"

	"auralogic/internal/middleware"
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

// ValidatePromoCodeRequest 验证优惠码请求
type ValidatePromoCodeRequest struct {
	Code        string `json:"code" binding:"required"`
	ProductIDs  []uint `json:"product_ids"`
	AmountMinor int64  `json:"amount_minor"`
}

func (h *PromoCodeHandler) buildPromoHookExecutionContext(c *gin.Context, userID uint) *service.ExecutionContext {
	if c == nil {
		return nil
	}
	metadata := map[string]string{
		"request_path":    c.Request.URL.Path,
		"route":           c.FullPath(),
		"method":          c.Request.Method,
		"client_ip":       c.ClientIP(),
		"user_agent":      c.GetHeader("User-Agent"),
		"accept_language": c.GetHeader("Accept-Language"),
		"operator_type":   "user",
	}
	sessionID := strings.TrimSpace(c.GetHeader("X-Session-ID"))
	return &service.ExecutionContext{
		UserID:         &userID,
		SessionID:      sessionID,
		Metadata:       metadata,
		RequestContext: c.Request.Context(),
	}
}

func promoHookDecodeProductIDs(value interface{}) ([]uint, error) {
	if value == nil {
		return nil, nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var productIDs []uint
	if err := json.Unmarshal(raw, &productIDs); err != nil {
		return nil, err
	}
	return productIDs, nil
}

func promoHookValueToInt64(value interface{}) (int64, error) {
	if value == nil {
		return 0, nil
	}
	switch typed := value.(type) {
	case int:
		return int64(typed), nil
	case int64:
		return typed, nil
	case uint:
		return int64(typed), nil
	case uint64:
		if typed > uint64(^uint64(0)>>1) {
			return 0, strconv.ErrRange
		}
		return int64(typed), nil
	case float64:
		if typed != float64(int64(typed)) {
			return 0, strconv.ErrSyntax
		}
		return int64(typed), nil
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return 0, nil
		}
		return strconv.ParseInt(trimmed, 10, 64)
	default:
		return 0, strconv.ErrSyntax
	}
}

// ValidatePromoCode 验证优惠码
func (h *PromoCodeHandler) ValidatePromoCode(c *gin.Context) {
	userID := middleware.MustGetUserID(c)
	if userID == 0 {
		return
	}

	var req ValidatePromoCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}
	hookExecCtx := h.buildPromoHookExecutionContext(c, userID)
	if h.pluginManager != nil {
		originalReq := req
		hookPayload := map[string]interface{}{
			"user_id":      userID,
			"code":         req.Code,
			"product_ids":  req.ProductIDs,
			"amount_minor": req.AmountMinor,
			"source":       "user_api",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "promo.validate.before",
			Payload: hookPayload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("promo.validate.before hook execution failed: user=%d code=%s err=%v", userID, req.Code, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Promo validation rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if rawCode, exists := hookResult.Payload["code"]; exists {
					code, ok := rawCode.(string)
					if !ok {
						log.Printf("promo.validate.before payload apply failed, fallback to original request: user=%d code=%s", userID, req.Code)
						req = originalReq
					} else {
						req.Code = strings.TrimSpace(code)
					}
				}
				if rawProductIDs, exists := hookResult.Payload["product_ids"]; exists {
					productIDs, decodeErr := promoHookDecodeProductIDs(rawProductIDs)
					if decodeErr != nil {
						log.Printf("promo.validate.before payload product_ids decode failed, fallback to original request: user=%d code=%s err=%v", userID, req.Code, decodeErr)
						req = originalReq
					} else {
						req.ProductIDs = productIDs
					}
				}
				if rawAmount, exists := hookResult.Payload["amount_minor"]; exists {
					amountMinor, convErr := promoHookValueToInt64(rawAmount)
					if convErr != nil {
						log.Printf("promo.validate.before payload amount_minor decode failed, fallback to original request: user=%d code=%s err=%v", userID, req.Code, convErr)
						req = originalReq
					} else {
						req.AmountMinor = amountMinor
					}
				}
			}
		}
	}

	promoCode, discount, err := h.promoCodeService.ValidateCode(req.Code, req.ProductIDs, req.AmountMinor)
	if err != nil {
		response.HandleError(c, "Invalid promo code", err)
		return
	}

	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"user_id":                userID,
			"code":                   req.Code,
			"product_ids":            req.ProductIDs,
			"amount_minor":           req.AmountMinor,
			"promo_code":             promoCode.Code,
			"promo_code_id":          promoCode.ID,
			"discount_type":          promoCode.DiscountType,
			"discount_value_minor":   promoCode.DiscountValue,
			"max_discount_minor":     promoCode.MaxDiscount,
			"min_order_amount_minor": promoCode.MinOrderAmount,
			"discount_minor":         discount,
			"source":                 "user_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, uid uint, code string) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "promo.validate.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("promo.validate.after hook execution failed: user=%d code=%s err=%v", uid, code, hookErr)
			}
		}(hookExecCtx, afterPayload, userID, req.Code)
	}

	response.Success(c, gin.H{
		"promo_code":             promoCode.Code,
		"promo_code_id":          promoCode.ID,
		"name":                   promoCode.Name,
		"discount_type":          promoCode.DiscountType,
		"discount_value_minor":   promoCode.DiscountValue,
		"max_discount_minor":     promoCode.MaxDiscount,
		"min_order_amount_minor": promoCode.MinOrderAmount,
		"discount_minor":         discount,
	})
}
