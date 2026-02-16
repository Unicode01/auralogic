package repository

import (
	"fmt"

	"auralogic/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PromoCodeRepository struct {
	db *gorm.DB
}

func NewPromoCodeRepository(db *gorm.DB) *PromoCodeRepository {
	return &PromoCodeRepository{db: db}
}

// Create 创建优惠码
func (r *PromoCodeRepository) Create(promoCode *models.PromoCode) error {
	return r.db.Create(promoCode).Error
}

// Update 更新优惠码
func (r *PromoCodeRepository) Update(promoCode *models.PromoCode) error {
	return r.db.Save(promoCode).Error
}

// FindByID 根据ID查找
func (r *PromoCodeRepository) FindByID(id uint) (*models.PromoCode, error) {
	var promoCode models.PromoCode
	err := r.db.First(&promoCode, id).Error
	return &promoCode, err
}

// FindByCode 根据优惠码查找
func (r *PromoCodeRepository) FindByCode(code string) (*models.PromoCode, error) {
	var promoCode models.PromoCode
	err := r.db.Where("code = ?", code).First(&promoCode).Error
	return &promoCode, err
}

// List 分页列表
func (r *PromoCodeRepository) List(page, limit int, status string, search string) ([]models.PromoCode, int64, error) {
	var promoCodes []models.PromoCode
	var total int64

	query := r.db.Model(&models.PromoCode{})

	if status != "" {
		query = query.Where("status = ?", status)
	}
	if search != "" {
		query = query.Where("code LIKE ? OR name LIKE ?", "%"+search+"%", "%"+search+"%")
	}

	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err = query.Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&promoCodes).Error

	return promoCodes, total, err
}

// Delete 删除优惠码（软删除）
func (r *PromoCodeRepository) Delete(id uint) error {
	return r.db.Delete(&models.PromoCode{}, id).Error
}

// Reserve 预留优惠码（下单时）
func (r *PromoCodeRepository) Reserve(promoCodeID uint, orderNo string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var promoCode models.PromoCode
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&promoCode, promoCodeID).Error; err != nil {
			return err
		}

		if !promoCode.IsAvailable() {
			return fmt.Errorf("promo code is not available")
		}

		if promoCode.TotalQuantity > 0 {
			promoCode.ReservedQuantity++
		}

		return tx.Save(&promoCode).Error
	})
}

// ReleaseReserve 释放预留优惠码（取消订单）
func (r *PromoCodeRepository) ReleaseReserve(promoCodeID uint, orderNo string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var promoCode models.PromoCode
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&promoCode, promoCodeID).Error; err != nil {
			return err
		}

		if promoCode.TotalQuantity > 0 && promoCode.ReservedQuantity > 0 {
			promoCode.ReservedQuantity--
		}

		return tx.Save(&promoCode).Error
	})
}

// Deduct 扣减优惠码（订单完成）
func (r *PromoCodeRepository) Deduct(promoCodeID uint, orderNo string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var promoCode models.PromoCode
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&promoCode, promoCodeID).Error; err != nil {
			return err
		}

		promoCode.UsedQuantity++
		if promoCode.TotalQuantity > 0 && promoCode.ReservedQuantity > 0 {
			promoCode.ReservedQuantity--
		}

		return tx.Save(&promoCode).Error
	})
}
