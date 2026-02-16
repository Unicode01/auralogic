package models

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// Inventory Inventory管理表
// 支持独立的Inventory配置，可被多个Product绑定（共享Inventory）
type Inventory struct {
	ID                 uint           `gorm:"primaryKey" json:"id"`
	Name               string         `gorm:"type:varchar(255);not null" json:"name"`                // Inventory配置名称
	SKU                string         `gorm:"type:varchar(100);index" json:"sku"`                    // InventorySKU（可选）
	AttributesHash     string         `gorm:"type:varchar(64);index" json:"-"`                       // 属性组合的哈希值（用于快速Query）
	Attributes         JSON           `gorm:"type:json" json:"attributes"`                           // 属性组合，如：{"颜色":"红色","尺寸":"L"}
	Stock              int            `gorm:"not null;default:0" json:"stock"`                       // Inventory数量
	AvailableQuantity  int            `gorm:"not null;default:0" json:"available_quantity"`          // 可购买数量（可以小于Inventory）
	SoldQuantity       int            `gorm:"not null;default:0" json:"sold_quantity"`               // 已售数量
	ReservedQuantity   int            `gorm:"not null;default:0" json:"reserved_quantity"`           // 预留数量（下单未支付）
	SafetyStock        int            `gorm:"default:0" json:"safety_stock"`                         // 安全Inventory（低于此值告警）
	AlertEmail         string         `gorm:"type:varchar(255)" json:"alert_email,omitempty"`        // Inventory告警Email
	IsActive           bool           `gorm:"default:true" json:"is_active"`                         // 是否启用
	Notes              string         `gorm:"type:text" json:"notes,omitempty"`                      // 备注
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`
	
	// 关联
	ProductBindings    []ProductInventoryBinding `gorm:"foreignKey:InventoryID" json:"product_bindings,omitempty"`
}

// TableName 指定表名
func (Inventory) TableName() string {
	return "inventories"
}

// GetRemainingStock get剩余Inventory（Inventory - 已售 - 预留）
func (i *Inventory) GetRemainingStock() int {
	return i.Stock - i.SoldQuantity - i.ReservedQuantity
}

// GetAvailableStock get可用Inventory（可购买数 - 已售 - 预留）
func (i *Inventory) GetAvailableStock() int {
	available := i.AvailableQuantity - i.SoldQuantity - i.ReservedQuantity
	if available < 0 {
		return 0
	}
	return available
}

// CanPurchase 检查是否可以购买指定数量
func (i *Inventory) CanPurchase(quantity int) (bool, string) {
	if !i.IsActive {
		return false, "This specification is unavailable"
	}
	
	// 检查可购买数
	availableStock := i.GetAvailableStock()
	if quantity > availableStock {
		if availableStock <= 0 {
			return false, "该规格已售罄"
		}
		return false, "Inventoryinsufficient, current可购买数量: " + string(rune(availableStock))
	}
	
	// 检查实际Inventory
	remainingStock := i.GetRemainingStock()
	if quantity > remainingStock {
		return false, "Insufficient stock"
	}
	
	return true, ""
}

// IsLowStock 是否低Inventory
func (i *Inventory) IsLowStock() bool {
	return i.GetRemainingStock() <= i.SafetyStock
}

// AttributesMap 将属性JSON转换为map
func (i *Inventory) AttributesMap() map[string]string {
	var attrs map[string]string
	if err := json.Unmarshal([]byte(i.Attributes), &attrs); err != nil {
		return make(map[string]string)
	}
	return attrs
}

// SetAttributes 设置属性并计算哈希
func (i *Inventory) SetAttributes(attrs map[string]string) error {
	// 转换为JSON
	jsonData, err := json.Marshal(attrs)
	if err != nil {
		return err
	}
	i.Attributes = JSON(jsonData)
	
	// 计算哈希值（用于快速Query）
	i.AttributesHash = GenerateAttributesHash(attrs)
	
	return nil
}

// InventoryLog Inventory变动日志
type InventoryLog struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	InventoryID   uint           `gorm:"not null;index" json:"inventory_id"`
	ProductID     uint           `gorm:"not null;index" json:"product_id"`
	Type          string         `gorm:"type:varchar(20);not null" json:"type"` // in(入库), out(出库), reserve(预留), release(释放), adjust(调整)
	Quantity      int            `gorm:"not null" json:"quantity"`              // 变动数量（正数或负数）
	BeforeStock   int            `gorm:"not null" json:"before_stock"`          // 变动前Inventory
	AfterStock    int            `gorm:"not null" json:"after_stock"`           // 变动后Inventory
	OrderNo       string         `gorm:"type:varchar(50);index" json:"order_no,omitempty"` // 关联Order号
	Operator      string         `gorm:"type:varchar(100)" json:"operator"`     // 操作人
	Reason        string         `gorm:"type:varchar(255)" json:"reason"`       // 变动原因
	Notes         string         `gorm:"type:text" json:"notes,omitempty"`      // 备注
	CreatedAt     time.Time      `json:"created_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName 指定表名
func (InventoryLog) TableName() string {
	return "inventory_logs"
}

// Inventory变动类型常量
const (
	InventoryLogTypeIn      = "in"      // 入库
	InventoryLogTypeOut     = "out"     // 出库
	InventoryLogTypeReserve = "reserve" // 预留
	InventoryLogTypeRelease = "release" // 释放预留
	InventoryLogTypeAdjust  = "adjust"  // 调整
)
