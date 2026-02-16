package service

import (
	"fmt"
	"strings"

	"auralogic/internal/models"
	"auralogic/internal/repository"
)

type PromoCodeService struct {
	repo        *repository.PromoCodeRepository
	productRepo *repository.ProductRepository
}

func NewPromoCodeService(repo *repository.PromoCodeRepository, productRepo *repository.ProductRepository) *PromoCodeService {
	return &PromoCodeService{
		repo:        repo,
		productRepo: productRepo,
	}
}

// Create 创建优惠码
func (s *PromoCodeService) Create(promoCode *models.PromoCode) error {
	promoCode.Code = strings.ToUpper(strings.TrimSpace(promoCode.Code))
	if promoCode.Code == "" {
		return fmt.Errorf("promo code cannot be empty")
	}

	// 检查是否已存在
	existing, err := s.repo.FindByCode(promoCode.Code)
	if err == nil && existing.ID > 0 {
		return fmt.Errorf("promo code already exists")
	}

	return s.repo.Create(promoCode)
}

// Update 更新优惠码
func (s *PromoCodeService) Update(id uint, updates *models.PromoCode) error {
	existing, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}

	existing.Name = updates.Name
	existing.Description = updates.Description
	existing.DiscountType = updates.DiscountType
	existing.DiscountValue = updates.DiscountValue
	existing.MaxDiscount = updates.MaxDiscount
	existing.MinOrderAmount = updates.MinOrderAmount
	existing.TotalQuantity = updates.TotalQuantity
	existing.ProductIDs = updates.ProductIDs
	existing.Status = updates.Status
	existing.ExpiresAt = updates.ExpiresAt

	return s.repo.Update(existing)
}

// GetByID 获取优惠码
func (s *PromoCodeService) GetByID(id uint) (*models.PromoCode, error) {
	return s.repo.FindByID(id)
}

// List 分页列表
func (s *PromoCodeService) List(page, limit int, status string, search string) ([]models.PromoCode, int64, error) {
	return s.repo.List(page, limit, status, search)
}

// Delete 删除优惠码
func (s *PromoCodeService) Delete(id uint) error {
	promoCode, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}

	// 有预留中的不允许删除
	if promoCode.ReservedQuantity > 0 {
		return fmt.Errorf("promo code has reserved usage, cannot delete")
	}

	return s.repo.Delete(id)
}

// ValidateCode 验证优惠码是否可用于指定商品
func (s *PromoCodeService) ValidateCode(code string, productIDs []uint, orderAmount float64) (*models.PromoCode, float64, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	promoCode, err := s.repo.FindByCode(code)
	if err != nil {
		return nil, 0, fmt.Errorf("promo code not found")
	}

	if !promoCode.IsAvailable() {
		return nil, 0, fmt.Errorf("promo code is not available")
	}

	// 检查是否适用于指定商品
	if len(promoCode.ProductIDs) > 0 && len(productIDs) > 0 {
		applicable := false
		for _, pid := range productIDs {
			if promoCode.IsApplicableToProduct(pid) {
				applicable = true
				break
			}
		}
		if !applicable {
			return nil, 0, fmt.Errorf("promo code is not applicable to the selected products")
		}
	}

	// 检查最低订单金额
	if promoCode.MinOrderAmount > 0 && orderAmount < promoCode.MinOrderAmount {
		return nil, 0, fmt.Errorf("order amount does not meet the minimum requirement")
	}

	discount := promoCode.CalculateDiscount(orderAmount)
	return promoCode, discount, nil
}

// Reserve 预留优惠码
func (s *PromoCodeService) Reserve(promoCodeID uint, orderNo string) error {
	return s.repo.Reserve(promoCodeID, orderNo)
}

// ReleaseReserve 释放优惠码预留
func (s *PromoCodeService) ReleaseReserve(promoCodeID uint, orderNo string) error {
	return s.repo.ReleaseReserve(promoCodeID, orderNo)
}

// Deduct 扣减优惠码
func (s *PromoCodeService) Deduct(promoCodeID uint, orderNo string) error {
	return s.repo.Deduct(promoCodeID, orderNo)
}
