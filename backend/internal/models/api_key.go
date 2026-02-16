package models

import (
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// APIKey API密钥模型
type APIKey struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	KeyName       string         `gorm:"type:varchar(100);not null" json:"key_name"`
	APIKey        string         `gorm:"type:varchar(255);uniqueIndex;not null" json:"api_key"`
	APISecretHash string         `gorm:"type:varchar(255);not null" json:"-"` // bcrypt哈希存储
	Platform      string         `gorm:"type:varchar(100)" json:"platform,omitempty"`

	// Permission范围
	Scopes []string `gorm:"type:text;serializer:json" json:"scopes,omitempty"`

	// 限流
	RateLimit int `gorm:"default:1000" json:"rate_limit"`

	IsActive   bool       `gorm:"default:true;index" json:"is_active"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`

	CreatedBy uint           `json:"created_by"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName 指定表名
func (APIKey) TableName() string {
	return "api_keys"
}

// SetSecret 设置API Secret (会自动哈希存储)
func (ak *APIKey) SetSecret(secret string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	ak.APISecretHash = string(hash)
	return nil
}

// VerifySecret 验证API Secret
func (ak *APIKey) VerifySecret(secret string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(ak.APISecretHash), []byte(secret))
	return err == nil
}

// IsExpired 检查是否已过期
func (ak *APIKey) IsExpired() bool {
	if ak.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*ak.ExpiresAt)
}

// HasScope 检查是否拥有指定Permission范围
func (ak *APIKey) HasScope(scope string) bool {
	for _, s := range ak.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

