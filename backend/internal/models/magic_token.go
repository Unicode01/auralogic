package models

import (
	"time"

	"gorm.io/gorm"
)

// MagicToken 快速登录Token
type MagicToken struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Token     string         `gorm:"type:varchar(255);uniqueIndex;not null" json:"token"`
	UserID    uint           `gorm:"not null;index" json:"user_id"`
	User      *User          `gorm:"foreignKey:UserID" json:"user,omitempty"`
	ExpiresAt time.Time      `gorm:"not null;index" json:"expires_at"`
	Used      bool           `gorm:"default:false" json:"used"`
	UsedAt    *time.Time     `json:"used_at,omitempty"`
	IPAddress string         `gorm:"type:varchar(50)" json:"ip_address,omitempty"`
	UserAgent string         `gorm:"type:text" json:"user_agent,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName 指定表名
func (MagicToken) TableName() string {
	return "magic_tokens"
}

// IsValid 检查Token是否有效
func (mt *MagicToken) IsValid() bool {
	return !mt.Used && time.Now().Before(mt.ExpiresAt)
}

