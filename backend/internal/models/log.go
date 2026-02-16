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
)

// EmailLog 邮件日志
type EmailLog struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	ToEmail      string         `gorm:"type:varchar(255);not null;index" json:"to_email"`
	Subject      string         `gorm:"type:varchar(500);not null" json:"subject"`
	Content      string         `gorm:"type:text;not null" json:"content"`
	EventType    string         `gorm:"type:varchar(50);index" json:"event_type,omitempty"`
	OrderID      *uint          `gorm:"index" json:"order_id,omitempty"`
	Order        *Order         `gorm:"foreignKey:OrderID" json:"order,omitempty"`
	UserID       *uint          `gorm:"index" json:"user_id,omitempty"`
	User         *User          `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Status       EmailLogStatus `gorm:"type:varchar(20);default:'pending';index" json:"status"`
	ErrorMessage string         `gorm:"type:text" json:"error_message,omitempty"`
	RetryCount   int            `gorm:"default:0" json:"retry_count"`
	SentAt       *time.Time     `json:"sent_at,omitempty"`
	CreatedAt    time.Time      `gorm:"index" json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

// TableName 指定表名
func (EmailLog) TableName() string {
	return "email_logs"
}
