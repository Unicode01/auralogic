package models

import (
	"encoding/json"
	"time"

	"auralogic/internal/pkg/money"
	"gorm.io/gorm"
)

type PromoCodeStatus string

const (
	PromoCodeStatusActive   PromoCodeStatus = "active"
	PromoCodeStatusInactive PromoCodeStatus = "inactive"
)

type DiscountType string

const (
	DiscountTypePercentage DiscountType = "percentage"
	DiscountTypeFixed      DiscountType = "fixed"
)

// PromoCode monetary fields are stored in integer minor units.
// For percentage type, DiscountValue is basis points (100% = 10000).
type PromoCode struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	Code        string `gorm:"type:varchar(50);uniqueIndex;not null" json:"code"`
	Name        string `gorm:"type:varchar(255);not null" json:"name"`
	Description string `gorm:"type:text" json:"description,omitempty"`

	DiscountType   DiscountType `gorm:"type:varchar(20);not null" json:"discount_type"`
	DiscountValue  int64        `gorm:"type:bigint;not null;default:0" json:"-"`
	MaxDiscount    int64        `gorm:"type:bigint;default:0" json:"-"`
	MinOrderAmount int64        `gorm:"type:bigint;default:0" json:"-"`

	TotalQuantity    int `gorm:"not null;default:0" json:"total_quantity"`
	UsedQuantity     int `gorm:"not null;default:0" json:"used_quantity"`
	ReservedQuantity int `gorm:"not null;default:0" json:"reserved_quantity"`

	ProductIDs   []uint `gorm:"type:text;serializer:json" json:"product_ids,omitempty"`
	ProductScope string `gorm:"type:varchar(20);default:'all'" json:"product_scope"`

	Status    PromoCodeStatus `gorm:"type:varchar(20);not null;default:'active';index" json:"status"`
	ExpiresAt *time.Time      `json:"expires_at,omitempty"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (PromoCode) TableName() string {
	return "promo_codes"
}

func (p PromoCode) MarshalJSON() ([]byte, error) {
	type Alias PromoCode
	return json.Marshal(&struct {
		Alias
		DiscountValueMinor  int64 `json:"discount_value_minor"`
		MaxDiscountMinor    int64 `json:"max_discount_minor"`
		MinOrderAmountMinor int64 `json:"min_order_amount_minor"`
	}{
		Alias:               Alias(p),
		DiscountValueMinor:  p.DiscountValue,
		MaxDiscountMinor:    p.MaxDiscount,
		MinOrderAmountMinor: p.MinOrderAmount,
	})
}

func (p *PromoCode) IsExpired() bool {
	if p.ExpiresAt == nil {
		return false
	}
	return NowFunc().After(*p.ExpiresAt)
}

func (p *PromoCode) GetAvailableQuantity() int {
	if p.TotalQuantity == 0 {
		return -1
	}
	available := p.TotalQuantity - p.UsedQuantity - p.ReservedQuantity
	if available < 0 {
		return 0
	}
	return available
}

func (p *PromoCode) IsAvailable() bool {
	if p.Status != PromoCodeStatusActive {
		return false
	}
	if p.IsExpired() {
		return false
	}
	if p.TotalQuantity > 0 && p.GetAvailableQuantity() <= 0 {
		return false
	}
	return true
}

func (p *PromoCode) IsApplicableToProduct(productID uint) bool {
	if len(p.ProductIDs) == 0 || p.ProductScope == "" || p.ProductScope == "all" {
		return true
	}
	if p.ProductScope == "exclude" {
		for _, id := range p.ProductIDs {
			if id == productID {
				return false
			}
		}
		return true
	}
	for _, id := range p.ProductIDs {
		if id == productID {
			return true
		}
	}
	return false
}

func (p *PromoCode) CalculateDiscount(orderAmount int64) int64 {
	if orderAmount < p.MinOrderAmount {
		return 0
	}

	var discount int64
	switch p.DiscountType {
	case DiscountTypePercentage:
		discount = money.ApplyPercentage(orderAmount, p.DiscountValue)
		if p.MaxDiscount > 0 && discount > p.MaxDiscount {
			discount = p.MaxDiscount
		}
	case DiscountTypeFixed:
		discount = p.DiscountValue
	}

	if discount > orderAmount {
		discount = orderAmount
	}
	return discount
}
