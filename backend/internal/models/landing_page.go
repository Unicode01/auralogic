package models

import (
	"time"

	"gorm.io/gorm"
)

// LandingPage 落地页
type LandingPage struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Slug        string         `gorm:"type:varchar(100);uniqueIndex;not null" json:"slug"`
	HTMLContent string         `gorm:"type:text" json:"html_content"`
	IsActive    bool           `gorm:"default:true" json:"is_active"`
	UpdatedBy   uint           `gorm:"default:0" json:"updated_by"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// PageView 页面访问记录（仅追加）
type PageView struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Page      string    `gorm:"type:varchar(255);index;not null" json:"page"`
	IP        string    `gorm:"type:varchar(45)" json:"ip"`
	UserAgent string    `gorm:"type:text" json:"user_agent"`
	Referer   string    `gorm:"type:text" json:"referer"`
	CreatedAt time.Time `json:"created_at"`
}
