package models

import (
	"time"

	"gorm.io/gorm"
)

// Announcement 公告
type Announcement struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	Title           string         `gorm:"type:varchar(255);not null" json:"title"`
	Content         string         `gorm:"type:text" json:"content"`
	IsMandatory     bool           `gorm:"default:false" json:"is_mandatory"`
	RequireFullRead bool           `gorm:"default:false" json:"require_full_read"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Announcement) TableName() string {
	return "announcements"
}

// AnnouncementRead 公告已读记录
type AnnouncementRead struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	AnnouncementID uint      `gorm:"uniqueIndex:idx_announcement_user;not null" json:"announcement_id"`
	UserID         uint      `gorm:"uniqueIndex:idx_announcement_user;not null" json:"user_id"`
	ReadAt         time.Time `json:"read_at"`
}

func (AnnouncementRead) TableName() string {
	return "announcement_reads"
}
