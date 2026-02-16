package models

import (
	"time"

	"gorm.io/gorm"
)

// CartItem 购物车项
type CartItem struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// 关联用户
	UserID uint `gorm:"not null;index" json:"user_id"`

	// 关联商品
	ProductID uint     `gorm:"not null;index" json:"product_id"`
	Product   *Product `gorm:"foreignKey:ProductID" json:"product,omitempty"`

	// 商品信息快照
	SKU        string  `gorm:"size:100" json:"sku"`
	Name       string  `gorm:"size:255" json:"name"`
	Price      float64 `json:"price"`
	ImageURL   string  `gorm:"size:500" json:"image_url"`
	ProductType ProductType `gorm:"size:20" json:"product_type"`

	// 购买数量
	Quantity int `gorm:"not null;default:1" json:"quantity"`

	// 选中的属性 (JSON格式)
	// 不同属性的商品应该分开存储
	Attributes     JSONMap `gorm:"type:text" json:"attributes"`
	AttributesHash string  `gorm:"type:varchar(32);index" json:"-"` // 属性哈希，用于快速匹配
}

// BeforeCreate 创建前计算属性哈希
func (c *CartItem) BeforeCreate(tx *gorm.DB) error {
	c.AttributesHash = GenerateAttributesHash(map[string]string(c.Attributes))
	return nil
}

// BeforeUpdate 更新前重新计算属性哈希
func (c *CartItem) BeforeUpdate(tx *gorm.DB) error {
	c.AttributesHash = GenerateAttributesHash(map[string]string(c.Attributes))
	return nil
}

// TableName 表名
func (CartItem) TableName() string {
	return "cart_items"
}

// CartItemWithStock 带库存信息的购物车项
type CartItemWithStock struct {
	CartItem
	AvailableStock int  `json:"available_stock"`
	IsAvailable    bool `json:"is_available"`
}
