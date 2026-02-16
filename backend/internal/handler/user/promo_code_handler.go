package user

import (
	"github.com/gin-gonic/gin"
	"auralogic/internal/pkg/response"
	"auralogic/internal/service"
)

type PromoCodeHandler struct {
	promoCodeService *service.PromoCodeService
}

func NewPromoCodeHandler(promoCodeService *service.PromoCodeService) *PromoCodeHandler {
	return &PromoCodeHandler{promoCodeService: promoCodeService}
}

// ValidatePromoCodeRequest 验证优惠码请求
type ValidatePromoCodeRequest struct {
	Code       string  `json:"code" binding:"required"`
	ProductIDs []uint  `json:"product_ids"`
	Amount     float64 `json:"amount"`
}

// ValidatePromoCode 验证优惠码
func (h *PromoCodeHandler) ValidatePromoCode(c *gin.Context) {
	var req ValidatePromoCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	promoCode, discount, err := h.promoCodeService.ValidateCode(req.Code, req.ProductIDs, req.Amount)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, gin.H{
		"promo_code":       promoCode.Code,
		"promo_code_id":    promoCode.ID,
		"name":             promoCode.Name,
		"discount_type":    promoCode.DiscountType,
		"discount_value":   promoCode.DiscountValue,
		"max_discount":     promoCode.MaxDiscount,
		"min_order_amount": promoCode.MinOrderAmount,
		"discount":         discount,
	})
}
