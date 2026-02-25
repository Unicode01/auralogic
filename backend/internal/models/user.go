package models

import (
	"time"

	"gorm.io/gorm"
)

// User User模型
type User struct {
	ID   uint   `gorm:"primaryKey" json:"id"`
	UUID string `gorm:"type:varchar(36);uniqueIndex;not null" json:"uuid"`
	// Uniqueness is enforced via "active-only" (deleted_at IS NULL) unique indexes in database.AutoMigrate().
	Email         string         `gorm:"type:varchar(255);index" json:"email"`
	Phone         *string        `gorm:"type:varchar(50);index" json:"phone,omitempty"`
	PasswordHash  string         `gorm:"type:varchar(255)" json:"-"`
	Name          string         `gorm:"type:varchar(100)" json:"name"`
	Avatar        string         `gorm:"type:varchar(500)" json:"avatar,omitempty"`
	Role          string         `gorm:"type:varchar(20);default:'user'" json:"role"` // user/admin/super_admin
	IsActive      bool           `gorm:"default:true" json:"is_active"`
	EmailVerified bool           `gorm:"default:false" json:"email_verified"`                 // 邮箱是否已验证
	Locale        string         `gorm:"type:varchar(10)" json:"locale,omitempty"`            // 用户语言偏好: zh, en
	LastLoginIP   string         `gorm:"type:varchar(50)" json:"-"`     // 最后登录IP
	RegisterIP    string         `gorm:"type:varchar(50)" json:"-"`       // 注册IP
	Country       string         `gorm:"type:varchar(100)" json:"country,omitempty"`          // 用户所在国家
	LastLoginAt   *time.Time     `json:"last_login_at,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName 指定表名
func (User) TableName() string {
	return "users"
}

// IsAdmin 是否是Admin
func (u *User) IsAdmin() bool {
	return u.Role == "admin" || u.Role == "super_admin"
}

// IsSuperAdmin 是否是超级Admin
func (u *User) IsSuperAdmin() bool {
	return u.Role == "super_admin"
}

// AdminPermission AdminPermission模型
type AdminPermission struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	UserID      uint           `gorm:"uniqueIndex;not null" json:"user_id"`
	User        *User          `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Permissions []string       `gorm:"type:text;serializer:json" json:"permissions"`
	CreatedBy   *uint          `json:"created_by,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName 指定表名
func (AdminPermission) TableName() string {
	return "admin_permissions"
}

// MagicToken 快速登录Token 的表名在 magic_token.go

// EmailVerificationToken 邮箱验证Token
type EmailVerificationToken struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Token     string         `gorm:"type:varchar(255);uniqueIndex;not null" json:"token"`
	UserID    uint           `gorm:"not null;index" json:"user_id"`
	User      *User          `gorm:"foreignKey:UserID" json:"user,omitempty"`
	ExpiresAt time.Time      `gorm:"not null;index" json:"expires_at"`
	Used      bool           `gorm:"default:false" json:"used"`
	UsedAt    *time.Time     `json:"used_at,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (EmailVerificationToken) TableName() string {
	return "email_verification_tokens"
}

func (t *EmailVerificationToken) IsValid() bool {
	return !t.Used && time.Now().Before(t.ExpiresAt)
}

// HasPermission 检查是否拥有指定Permission
func (ap *AdminPermission) HasPermission(permission string) bool {
	for _, p := range ap.Permissions {
		if p == permission {
			return true
		}
	}
	return false
}
