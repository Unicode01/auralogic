package models

import (
	"time"
)

// OperationLog 操作日志
type OperationLog struct {
	ID           uint                   `gorm:"primaryKey" json:"id"`
	UserID       *uint                  `gorm:"index" json:"user_id,omitempty"`
	User         *User                  `gorm:"foreignKey:UserID" json:"user,omitempty"`
	OperatorName string                 `gorm:"type:varchar(100)" json:"operator_name,omitempty"` // API 平台名称或其他操作者名称
	Action       string                 `gorm:"type:varchar(50);not null" json:"action"`
	ResourceType string                 `gorm:"type:varchar(50);index" json:"resource_type,omitempty"`
	ResourceID   *uint                  `gorm:"index" json:"resource_id,omitempty"`
	Details      map[string]interface{} `gorm:"type:text;serializer:json" json:"details,omitempty"`
	IPAddress    string                 `gorm:"type:varchar(50)" json:"ip_address,omitempty"`
	UserAgent    string                 `gorm:"type:text" json:"user_agent,omitempty"`
	CreatedAt    time.Time              `gorm:"index" json:"created_at"`
}

// TableName 指定表名
func (OperationLog) TableName() string {
	return "operation_logs"
}

// EmailLogStatus 邮件日志状态
type EmailLogStatus string

const (
	EmailLogStatusPending EmailLogStatus = "pending" // 待发送
	EmailLogStatusSent    EmailLogStatus = "sent"    // 已发送
	EmailLogStatusFailed  EmailLogStatus = "failed"  // 发送Failed
	EmailLogStatusExpired EmailLogStatus = "expired" // 已过期
)

// EmailLog 邮件日志
type EmailLog struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	ToEmail      string         `gorm:"type:varchar(255);not null;index" json:"to_email"`
	Subject      string         `gorm:"type:varchar(500);not null" json:"subject"`
	Content      string         `gorm:"type:text;not null" json:"-"`
	EventType    string         `gorm:"type:varchar(50);index" json:"event_type,omitempty"`
	OrderID      *uint          `gorm:"index" json:"order_id,omitempty"`
	Order        *Order         `gorm:"foreignKey:OrderID" json:"order,omitempty"`
	UserID       *uint          `gorm:"index" json:"user_id,omitempty"`
	User         *User          `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Status       EmailLogStatus `gorm:"type:varchar(20);default:'pending';index" json:"status"`
	ErrorMessage string         `gorm:"type:text" json:"error_message,omitempty"`
	RetryCount   int            `gorm:"default:0" json:"retry_count"`
	ExpireAt     *time.Time     `gorm:"index" json:"expire_at,omitempty"`
	SentAt       *time.Time     `json:"sent_at,omitempty"`
	CreatedAt    time.Time      `gorm:"index" json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

// TableName 指定表名
func (EmailLog) TableName() string {
	return "email_logs"
}

// SmsLogStatus 短信日志状态
type SmsLogStatus string

const (
	SmsLogStatusPending SmsLogStatus = "pending"
	SmsLogStatusSent    SmsLogStatus = "sent"
	SmsLogStatusFailed  SmsLogStatus = "failed"
	SmsLogStatusExpired SmsLogStatus = "expired"
)

// SmsLog 短信日志
type SmsLog struct {
	ID           uint         `gorm:"primaryKey" json:"id"`
	Phone        string       `gorm:"type:varchar(50);not null;index" json:"phone"`
	Content      string       `gorm:"type:text;not null" json:"-"`
	EventType    string       `gorm:"type:varchar(50);index" json:"event_type,omitempty"`
	UserID       *uint        `gorm:"index" json:"user_id,omitempty"`
	User         *User        `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Provider     string       `gorm:"type:varchar(50)" json:"provider"`
	Status       SmsLogStatus `gorm:"type:varchar(20);default:'pending';index" json:"status"`
	ErrorMessage string       `gorm:"type:text" json:"error_message,omitempty"`
	ExpireAt     *time.Time   `gorm:"index" json:"expire_at,omitempty"`
	SentAt       *time.Time   `json:"sent_at,omitempty"`
	CreatedAt    time.Time    `gorm:"index" json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

func (SmsLog) TableName() string {
	return "sms_logs"
}

// MaskContent 将内容脱敏，保留前2字符和最后1字符，中间用*替代
func MaskContent(s string) string {
	runes := []rune(s)
	n := len(runes)
	if n <= 3 {
		return "***"
	}
	masked := make([]rune, n)
	copy(masked, runes)
	for i := 2; i < n-1; i++ {
		masked[i] = '*'
	}
	return string(masked)
}
