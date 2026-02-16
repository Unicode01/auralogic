package models

import (
	"time"

	"gorm.io/gorm"
)

// VirtualInventory 虚拟库存表（存储卡密/激活码等虚拟商品的库存池）
// 类似于实体库存 Inventory，可以独立创建，然后绑定到商品
type VirtualInventory struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Name        string         `gorm:"type:varchar(255);not null" json:"name"`          // 库存名称
	SKU         string         `gorm:"type:varchar(100);index" json:"sku"`              // SKU（可选）
	Description string         `gorm:"type:text" json:"description,omitempty"`          // 描述
	IsActive    bool           `gorm:"default:true" json:"is_active"`                   // 是否启用
	Notes       string         `gorm:"type:text" json:"notes,omitempty"`                // 备注
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	// 关联
	Stocks          []VirtualProductStock          `gorm:"foreignKey:VirtualInventoryID" json:"stocks,omitempty"`
	ProductBindings []ProductVirtualInventoryBinding `gorm:"foreignKey:VirtualInventoryID" json:"product_bindings,omitempty"`
}

// TableName 指定表名
func (VirtualInventory) TableName() string {
	return "virtual_inventories"
}

// ProductVirtualInventoryBinding 商品-虚拟库存绑定表（多对多关系）
// 与实体库存 ProductInventoryBinding 采用相同的绑定制设计
type ProductVirtualInventoryBinding struct {
	ID                 uint              `gorm:"primaryKey" json:"id"`
	ProductID          uint              `gorm:"not null;index:idx_pvib_product;uniqueIndex:idx_pvib_product_attrs" json:"product_id"`
	VirtualInventoryID uint              `gorm:"not null;index:idx_pvib_virtual_inventory" json:"virtual_inventory_id"`
	Attributes         JSONMap           `gorm:"type:text" json:"attributes"`                                              // 规格属性组合（JSON格式）
	AttributesHash     string            `gorm:"type:varchar(64);uniqueIndex:idx_pvib_product_attrs" json:"attributes_hash"` // 规格组合哈希，唯一性约束：同一商品的同一规格组合只能绑定一次
	IsRandom           bool              `gorm:"default:false" json:"is_random"`                                           // 是否参与盲盒随机分配
	Priority           int               `gorm:"default:1" json:"priority"`                                                // 权重
	Notes              string            `gorm:"type:text" json:"notes,omitempty"`                                         // 备注
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`

	// 关联
	Product          *Product          `gorm:"foreignKey:ProductID" json:"product,omitempty"`
	VirtualInventory *VirtualInventory `gorm:"foreignKey:VirtualInventoryID" json:"virtual_inventory,omitempty"`
}

// TableName 指定表名
func (ProductVirtualInventoryBinding) TableName() string {
	return "product_virtual_inventory_bindings"
}

// VirtualInventoryWithStats 虚拟库存及统计信息（用于前端展示）
type VirtualInventoryWithStats struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	SKU         string `json:"sku"`
	Description string `json:"description"`
	IsActive    bool   `json:"is_active"`
	Notes       string `json:"notes"`
	Total       int64  `json:"total"`
	Available   int64  `json:"available"`
	Reserved    int64  `json:"reserved"`
	Sold        int64  `json:"sold"`
	CreatedAt   time.Time `json:"created_at"`
}

// BindingWithVirtualInventoryInfo 绑定关系及虚拟库存详情（用于前端展示）
type BindingWithVirtualInventoryInfo struct {
	ID                 uint                       `json:"id"`
	ProductID          uint                       `json:"product_id"`
	VirtualInventoryID uint                       `json:"virtual_inventory_id"`
	Attributes         JSONMap                    `json:"attributes"`
	AttributesHash     string                     `json:"attributes_hash"`
	IsRandom           bool                       `json:"is_random"`
	Priority           int                        `json:"priority"`
	Notes              string                     `json:"notes,omitempty"`
	VirtualInventory   *VirtualInventoryWithStats `json:"virtual_inventory"`
	CreatedAt          time.Time                  `json:"created_at"`
}
