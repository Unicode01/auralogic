package models

import (
	"time"
)

// ProductSerial 产品序列号
type ProductSerial struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	SerialNumber   string    `gorm:"uniqueIndex;size:100;not null" json:"serial_number"` // 完整序列号：产品码+序号+防伪码
	ProductID      uint      `gorm:"index;not null" json:"product_id"`                    // 商品ID
	Product        *Product  `gorm:"foreignKey:ProductID" json:"product,omitempty"`
	OrderID        uint      `gorm:"index;not null" json:"order_id"`                      // 订单ID
	Order          *Order    `gorm:"foreignKey:OrderID" json:"order,omitempty"`
	ProductCode    string    `gorm:"size:20;not null" json:"product_code"`                // 产品码
	SequenceNumber int       `gorm:"not null" json:"sequence_number"`                     // 出厂序号 (001, 002...)
	AntiCounterfeitCode string `gorm:"size:10;not null" json:"anti_counterfeit_code"`   // 防伪码 (4位随机)
	ViewCount      int       `gorm:"default:0" json:"view_count"`                         // 查看次数
	FirstViewedAt  *time.Time `json:"first_viewed_at,omitempty"`                          // 首次查看时间
	LastViewedAt   *time.Time `json:"last_viewed_at,omitempty"`                           // 最后查看时间
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (ProductSerial) TableName() string {
	return "product_serials"
}
