package models

import (
	"time"
)

// ProductInventoryBinding Product-Inventory绑定表（多对多关系）
// 注意：此表不使用软Delete，因为绑定关系应该是明确的Create和Delete
type ProductInventoryBinding struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	ProductID      uint      `gorm:"not null;index:idx_product;uniqueIndex:idx_product_attrs" json:"product_id"`
	InventoryID    uint      `gorm:"not null;index:idx_inventory" json:"inventory_id"`
	Attributes     JSON      `gorm:"type:json" json:"attributes"`                                           // 规格组合，例如：{"Color":"Blue","Size":"L"}
	AttributesHash string    `gorm:"type:varchar(64);uniqueIndex:idx_product_attrs" json:"attributes_hash"` // 规格组合哈希，唯一性约束：同一Product的同一规格组合只能绑定一次
	IsRandom       bool      `gorm:"default:false" json:"is_random"`                                        // 是否参与盲盒随机分配
	Priority       int       `gorm:"default:1" json:"priority"`                                             // 权重（用于盲盒随机分配，值越大概率越高）
	Notes          string    `gorm:"type:text" json:"notes,omitempty"`                                      // 备注（可选，用于Admin添加说明）
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`

	// 关联
	Product   *Product   `gorm:"foreignKey:ProductID" json:"product,omitempty"`
	Inventory *Inventory `gorm:"foreignKey:InventoryID" json:"inventory,omitempty"`
}

// TableName 指定表名
func (ProductInventoryBinding) TableName() string {
	return "product_inventory_bindings"
}

// BindingWithInventoryInfo 绑定关系及Inventory详情（用于前端展示）
type BindingWithInventoryInfo struct {
	ID          uint                `json:"id"`
	ProductID   uint                `json:"product_id"`
	InventoryID uint                `json:"inventory_id"`
	IsRandom    bool                `json:"is_random"`
	Priority    int                 `json:"priority"`
	Notes       string              `json:"notes,omitempty"`
	Inventory   *InventoryWithStock `json:"inventory"`
	CreatedAt   time.Time           `json:"created_at"`
}

// InventoryWithStock Inventory及Inventory状态Info
type InventoryWithStock struct {
	ID                uint   `json:"id"`
	Name              string `json:"name"`
	SKU               string `json:"sku"`
	Attributes        JSON   `json:"attributes"` // 使用JSON类型以支持Scan
	Stock             int    `json:"stock"`
	AvailableQuantity int    `json:"available_quantity"`
	SoldQuantity      int    `json:"sold_quantity"`
	ReservedQuantity  int    `json:"reserved_quantity"`
	RemainingStock    int    `json:"remaining_stock"` // 计算得出
	IsActive          bool   `json:"is_active"`
}
