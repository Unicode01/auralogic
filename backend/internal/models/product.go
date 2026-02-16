package models

import (
	"time"

	"gorm.io/gorm"
)

// ProductStatus Product状态
type ProductStatus string

const (
	ProductStatusDraft      ProductStatus = "draft"        // 草稿
	ProductStatusActive     ProductStatus = "active"       // 上架
	ProductStatusInactive   ProductStatus = "inactive"     // 下架
	ProductStatusOutOfStock ProductStatus = "out_of_stock" // 缺货
)

// PurchaseType 购买类型
type PurchaseType string

const (
	PurchaseTypeMoney PurchaseType = "money" // 人民币购买
)

// ProductType 商品类型
type ProductType string

const (
	ProductTypePhysical ProductType = "physical" // 实物商品
	ProductTypeVirtual  ProductType = "virtual"  // 虚拟商品
)

// InventoryMode Inventory模式
type InventoryMode string

const (
	InventoryModeFixed  InventoryMode = "fixed"  // 固定模式：User选择属性
	InventoryModeRandom InventoryMode = "random" // 盲盒模式：系统随机分配
)

// ProductImage Product图片
type ProductImage struct {
	URL       string `json:"url"`
	Alt       string `json:"alt,omitempty"`
	IsPrimary bool   `json:"is_primary"`
}

// AttributeMode 属性模式
type AttributeMode string

const (
	AttributeModeUserSelect AttributeMode = "user_select" // User自选
	AttributeModeBlindBox   AttributeMode = "blind_box"   // 盲盒随机
)

// ProductAttribute Product属性（如颜色、尺寸等）
type ProductAttribute struct {
	Name   string        `json:"name"`
	Values []string      `json:"values"`
	Mode   AttributeMode `json:"mode,omitempty"` // 属性模式：User自选或盲盒随机
}

// Product Product模型
type Product struct {
	ID uint `gorm:"primaryKey" json:"id"`
	// SKU uniqueness is enforced for "active" (non-deleted) rows via DB migrations:
	// - sqlite/postgres: partial unique index WHERE deleted_at IS NULL
	// This allows reusing the same SKU after soft-delete.
	SKU         string `gorm:"type:varchar(100);index:idx_products_sku_lookup;not null" json:"sku"`
	Name        string `gorm:"type:varchar(255);not null" json:"name"`
	ProductCode string `gorm:"type:varchar(20);index" json:"product_code,omitempty"` // 产品码，用于生成防伪序列号

	// 商品类型
	ProductType ProductType `gorm:"type:varchar(20);not null;default:'physical';index" json:"product_type"` // physical(实物), virtual(虚拟)

	// ProductInfo
	Description      string `gorm:"type:text" json:"description,omitempty"`
	ShortDescription string `gorm:"type:varchar(500)" json:"short_description,omitempty"`

	// 分类和标签
	Category string   `gorm:"type:varchar(100);index" json:"category,omitempty"`
	Tags     []string `gorm:"type:text;serializer:json" json:"tags,omitempty"`

	// 价格和Inventory
	Price         float64 `gorm:"type:decimal(10,2);not null" json:"price"`
	OriginalPrice float64 `gorm:"type:decimal(10,2)" json:"original_price,omitempty"`
	Stock         int     `gorm:"not null;default:0" json:"stock"` // Inventory汇总字段，实际Inventory由Inventory表管理

	// 购买限制
	MaxPurchaseLimit int `gorm:"default:0" json:"max_purchase_limit,omitempty"` // 每个账户最大购买数量，0表示不限制

	// 图片
	Images []ProductImage `gorm:"type:text;serializer:json" json:"images,omitempty"`

	// 属性（如颜色、尺寸等）
	Attributes []ProductAttribute `gorm:"type:text;serializer:json" json:"attributes,omitempty"`

	// 状态
	Status ProductStatus `gorm:"type:varchar(30);not null;default:'draft';index" json:"status"`

	// 排序和推荐
	SortOrder     int  `gorm:"default:0;index" json:"sort_order"`
	IsFeatured    bool `gorm:"default:false;index" json:"is_featured"` // 是否精选
	IsRecommended bool `gorm:"default:false" json:"is_recommended"`    // 是否推荐

	// 统计数据
	ViewCount int `gorm:"default:0" json:"view_count"` // 浏览次数
	SaleCount int `gorm:"default:0" json:"sale_count"` // 销售数量

	// 备注
	Remark string `gorm:"type:text" json:"remark,omitempty"`

	// Inventory模式
	InventoryMode string `gorm:"type:varchar(20);default:'fixed'" json:"inventory_mode"` // fixed(固定), random(盲盒/随机)

	// 虚拟商品自动发货
	AutoDelivery bool `gorm:"default:false" json:"auto_delivery"` // 虚拟商品是否自动发货

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// 关联
	InventoryBindings []ProductInventoryBinding `gorm:"foreignKey:ProductID" json:"inventory_bindings,omitempty"`
}

// TableName 指定表名
func (Product) TableName() string {
	return "products"
}

// GetPrimaryImage get主图
func (p *Product) GetPrimaryImage() string {
	for _, img := range p.Images {
		if img.IsPrimary {
			return img.URL
		}
	}
	if len(p.Images) > 0 {
		return p.Images[0].URL
	}
	return ""
}

// IsAvailable 判断Product是否可购买
func (p *Product) IsAvailable() bool {
	return p.Status == ProductStatusActive && p.Stock > 0
}
