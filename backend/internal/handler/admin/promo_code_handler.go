package admin

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"auralogic/internal/models"
	"auralogic/internal/pkg/response"
	"auralogic/internal/service"
)

type PromoCodeHandler struct {
	promoCodeService *service.PromoCodeService
}

func NewPromoCodeHandler(promoCodeService *service.PromoCodeService) *PromoCodeHandler {
	return &PromoCodeHandler{promoCodeService: promoCodeService}
}

// CreatePromoCodeRequest 创建优惠码请求
type CreatePromoCodeRequest struct {
	Code           string              `json:"code" binding:"required"`
	Name           string              `json:"name" binding:"required"`
	Description    string              `json:"description"`
	DiscountType   models.DiscountType `json:"discount_type" binding:"required"`
	DiscountValue  float64             `json:"discount_value" binding:"required,gt=0"`
	MaxDiscount    float64             `json:"max_discount"`
	MinOrderAmount float64             `json:"min_order_amount"`
	TotalQuantity  int                 `json:"total_quantity"`
	ProductIDs     []uint              `json:"product_ids"`
	ProductScope   string              `json:"product_scope"`
	Status         string              `json:"status"`
	ExpiresAt      *string             `json:"expires_at"`
}

// UpdatePromoCodeRequest 更新优惠码请求（不需要code字段）
type UpdatePromoCodeRequest struct {
	Name           string              `json:"name" binding:"required"`
	Description    string              `json:"description"`
	DiscountType   models.DiscountType `json:"discount_type" binding:"required"`
	DiscountValue  float64             `json:"discount_value" binding:"required,gt=0"`
	MaxDiscount    float64             `json:"max_discount"`
	MinOrderAmount float64             `json:"min_order_amount"`
	TotalQuantity  int                 `json:"total_quantity"`
	ProductIDs     []uint              `json:"product_ids"`
	ProductScope   string              `json:"product_scope"`
	Status         string              `json:"status"`
	ExpiresAt      *string             `json:"expires_at"`
}

// CreatePromoCode 创建优惠码
func (h *PromoCodeHandler) CreatePromoCode(c *gin.Context) {
	var req CreatePromoCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	promoCode := &models.PromoCode{
		Code:           req.Code,
		Name:           req.Name,
		Description:    req.Description,
		DiscountType:   req.DiscountType,
		DiscountValue:  req.DiscountValue,
		MaxDiscount:    req.MaxDiscount,
		MinOrderAmount: req.MinOrderAmount,
		TotalQuantity:  req.TotalQuantity,
		ProductIDs:     req.ProductIDs,
		ProductScope:   req.ProductScope,
		Status:         models.PromoCodeStatusActive,
	}

	if req.Status != "" {
		promoCode.Status = models.PromoCodeStatus(req.Status)
	}

	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			// 尝试其他格式
			t, err = time.Parse("2006-01-02T15:04:05", *req.ExpiresAt)
			if err != nil {
				t, err = time.Parse("2006-01-02", *req.ExpiresAt)
				if err != nil {
					response.BadRequest(c, "Invalid expiry date format")
					return
				}
				// 设置为当天结束
				t = t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
			}
		}
		promoCode.ExpiresAt = &t
	}

	if err := h.promoCodeService.Create(promoCode); err != nil {
		response.BadRequest(c, err.Error())
		return
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

	var req UpdatePromoCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	updates := &models.PromoCode{
		Name:           req.Name,
		Description:    req.Description,
		DiscountType:   req.DiscountType,
		DiscountValue:  req.DiscountValue,
		MaxDiscount:    req.MaxDiscount,
		MinOrderAmount: req.MinOrderAmount,
		TotalQuantity:  req.TotalQuantity,
		ProductIDs:     req.ProductIDs,
		ProductScope:   req.ProductScope,
		Status:         models.PromoCodeStatusActive,
	}

	if req.Status != "" {
		updates.Status = models.PromoCodeStatus(req.Status)
	}

	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			t, err = time.Parse("2006-01-02T15:04:05", *req.ExpiresAt)
			if err != nil {
				t, err = time.Parse("2006-01-02", *req.ExpiresAt)
				if err != nil {
					response.BadRequest(c, "Invalid expiry date format")
					return
				}
				t = t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
			}
		}
		updates.ExpiresAt = &t
	}

	if err := h.promoCodeService.Update(uint(id), updates); err != nil {
		response.BadRequest(c, err.Error())
		return
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

	if err := h.promoCodeService.Delete(uint(id)); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, gin.H{"message": "deleted"})
}

// ListPromoCodes 列表
func (h *PromoCodeHandler) ListPromoCodes(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	status := c.Query("status")
	search := c.Query("search")

	if limit > 100 {
		limit = 100
	}

	promoCodes, total, err := h.promoCodeService.List(page, limit, status, search)
	if err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	response.Paginated(c, promoCodes, page, limit, total)
}
