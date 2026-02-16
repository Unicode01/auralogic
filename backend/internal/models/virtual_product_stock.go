package models

import (
	"time"

	"gorm.io/gorm"
)

// VirtualProductStockStatus 虚拟产品库存状态
type VirtualProductStockStatus string

const (
	VirtualStockStatusAvailable VirtualProductStockStatus = "available" // 可用
	VirtualStockStatusSold      VirtualProductStockStatus = "sold"      // 已售出
	VirtualStockStatusReserved  VirtualProductStockStatus = "reserved"  // 已预留
	VirtualStockStatusInvalid   VirtualProductStockStatus = "invalid"   // 已失效
)

// VirtualProductStock 虚拟产品库存表（存储卡密、激活码等）
type VirtualProductStock struct {
	ID                 uint              `gorm:"primaryKey" json:"id"`
	VirtualInventoryID uint              `gorm:"not null;index:idx_virtual_inventory_status" json:"virtual_inventory_id"`
	VirtualInventory   *VirtualInventory `gorm:"foreignKey:VirtualInventoryID" json:"virtual_inventory,omitempty"`

	// 虚拟商品内容
	Content string `gorm:"type:text;not null" json:"content"`         // 卡密/激活码内容
	Remark  string `gorm:"type:varchar(500)" json:"remark,omitempty"` // 备注信息

	// 状态
	Status VirtualProductStockStatus `gorm:"type:varchar(20);not null;default:'available';index:idx_virtual_inventory_status" json:"status"`

	// 订单关联
	OrderID *uint  `gorm:"index" json:"order_id,omitempty"`
	OrderNo string `gorm:"type:varchar(50);index" json:"order_no,omitempty"`

	// 发货信息
	DeliveredAt *time.Time `json:"delivered_at,omitempty"`
	DeliveredBy *uint      `json:"delivered_by,omitempty"`

	// 导入批次
	BatchNo    string `gorm:"type:varchar(100);index" json:"batch_no,omitempty"` // 批次号，用于追踪导入批次
	ImportedBy string `gorm:"type:varchar(100)" json:"imported_by,omitempty"`    // 导入人

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName 指定表名
func (VirtualProductStock) TableName() string {
	return "virtual_product_stocks"
}

// IsAvailable 是否可用
func (v *VirtualProductStock) IsAvailable() bool {
	return v.Status == VirtualStockStatusAvailable
}

// MarkAsSold 标记为已售出
func (v *VirtualProductStock) MarkAsSold(orderID uint, orderNo string) {
	v.Status = VirtualStockStatusSold
	v.OrderID = &orderID
	v.OrderNo = orderNo
	now := NowFunc()
	v.DeliveredAt = &now
}

// MarkAsReserved 标记为已预留
func (v *VirtualProductStock) MarkAsReserved(orderNo string) {
	v.Status = VirtualStockStatusReserved
	v.OrderNo = orderNo
}

// Release 释放预留（取消订单时）
func (v *VirtualProductStock) Release() {
	v.Status = VirtualStockStatusAvailable
	v.OrderNo = ""
}
