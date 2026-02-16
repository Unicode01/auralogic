package models

import (
	"time"

	"gorm.io/gorm"
)

// PromoCodeStatus 优惠码状态
type PromoCodeStatus string

const (
	PromoCodeStatusActive   PromoCodeStatus = "active"   // 启用
	PromoCodeStatusInactive PromoCodeStatus = "inactive" // 停用
)

// DiscountType 折扣类型
type DiscountType string

const (
	DiscountTypePercentage DiscountType = "percentage" // 百分比折扣
	DiscountTypeFixed      DiscountType = "fixed"      // 固定金额折扣
)

// PromoCode 优惠码模型
type PromoCode struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	Code        string `gorm:"type:varchar(50);uniqueIndex;not null" json:"code"`
	Name        string `gorm:"type:varchar(255);not null" json:"name"`
	Description string `gorm:"type:text" json:"description,omitempty"`

	// 折扣配置
	DiscountType  DiscountType `gorm:"type:varchar(20);not null" json:"discount_type"`            // percentage, fixed
	DiscountValue float64      `gorm:"type:decimal(10,2);not null" json:"discount_value"`          // 折扣值：百分比(0-100)或固定金额
	MaxDiscount   float64      `gorm:"type:decimal(10,2);default:0" json:"max_discount,omitempty"` // 最大折扣金额（百分比类型时有效），0表示不限制
	MinOrderAmount float64     `gorm:"type:decimal(10,2);default:0" json:"min_order_amount,omitempty"` // 最低订单金额，0表示不限制

	// 数量管理（类似库存的Reserve/Deduct/Release模式）
	TotalQuantity    int `gorm:"not null;default:0" json:"total_quantity"`    // 总数量，0表示不限制
	UsedQuantity     int `gorm:"not null;default:0" json:"used_quantity"`     // 已使用数量
	ReservedQuantity int `gorm:"not null;default:0" json:"reserved_quantity"` // 预留数量（下单未完成）

	// 适用商品，空数组表示适用所有商品
	ProductIDs   []uint `gorm:"type:text;serializer:json" json:"product_ids,omitempty"`
	ProductScope string `gorm:"type:varchar(20);default:'all'" json:"product_scope"` // all, specific, exclude

	// 状态和有效期
	Status    PromoCodeStatus `gorm:"type:varchar(20);not null;default:'active';index" json:"status"`
	ExpiresAt *time.Time      `json:"expires_at,omitempty"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName 指定表名
func (PromoCode) TableName() string {
	return "promo_codes"
}

// IsExpired 检查是否已过期
func (p *PromoCode) IsExpired() bool {
	if p.ExpiresAt == nil {
		return false
	}
	return NowFunc().After(*p.ExpiresAt)
}

// GetAvailableQuantity 获取可用数量
func (p *PromoCode) GetAvailableQuantity() int {
	if p.TotalQuantity == 0 {
		return -1 // 无限制
	}
	available := p.TotalQuantity - p.UsedQuantity - p.ReservedQuantity
	if available < 0 {
		return 0
	}
	return available
}

// IsAvailable 检查优惠码是否可用
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

// IsApplicableToProduct 检查是否适用于指定商品
func (p *PromoCode) IsApplicableToProduct(productID uint) bool {
	if len(p.ProductIDs) == 0 || p.ProductScope == "" || p.ProductScope == "all" {
		return true // 空列表或all表示适用所有商品
	}
	if p.ProductScope == "exclude" {
		// 排除模式：不在列表中的商品适用
		for _, id := range p.ProductIDs {
			if id == productID {
				return false
			}
		}
		return true
	}
	// 指定模式(specific)：仅列表中的商品适用
	for _, id := range p.ProductIDs {
		if id == productID {
			return true
		}
	}
	return false
}

// CalculateDiscount 计算折扣金额
func (p *PromoCode) CalculateDiscount(orderAmount float64) float64 {
	if orderAmount < p.MinOrderAmount {
		return 0
	}

	var discount float64
	switch p.DiscountType {
	case DiscountTypePercentage:
		discount = orderAmount * p.DiscountValue / 100
		if p.MaxDiscount > 0 && discount > p.MaxDiscount {
			discount = p.MaxDiscount
		}
	case DiscountTypeFixed:
		discount = p.DiscountValue
	}

	// 折扣不能超过订单金额
	if discount > orderAmount {
		discount = orderAmount
	}

	return discount
}
