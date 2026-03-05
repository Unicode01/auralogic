package models

import "time"

type MarketingTaskChannel string

const (
	MarketingTaskChannelEmail MarketingTaskChannel = "email"
	MarketingTaskChannelSMS   MarketingTaskChannel = "sms"
)

type MarketingTaskStatus string

const (
	MarketingTaskStatusPending MarketingTaskStatus = "pending"
	MarketingTaskStatusQueued  MarketingTaskStatus = "queued"
	MarketingTaskStatusSent    MarketingTaskStatus = "sent"
	MarketingTaskStatusFailed  MarketingTaskStatus = "failed"
	MarketingTaskStatusSkipped MarketingTaskStatus = "skipped"
)

type MarketingBatchTask struct {
	ID      uint            `gorm:"primaryKey" json:"id"`
	BatchID uint            `gorm:"not null;index:idx_marketing_batch_status" json:"batch_id"`
	Batch   *MarketingBatch `gorm:"foreignKey:BatchID" json:"batch,omitempty"`

	UserID uint  `gorm:"not null;index" json:"user_id"`
	User   *User `gorm:"foreignKey:UserID" json:"user,omitempty"`

	Channel      MarketingTaskChannel `gorm:"type:varchar(20);not null;index:idx_marketing_batch_status" json:"channel"`
	Status       MarketingTaskStatus  `gorm:"type:varchar(20);not null;default:'pending';index:idx_marketing_batch_status" json:"status"`
	ErrorMessage string               `gorm:"type:text" json:"error_message,omitempty"`
	ProcessedAt  *time.Time           `json:"processed_at,omitempty"`
	CreatedAt    time.Time            `gorm:"index" json:"created_at"`
	UpdatedAt    time.Time            `json:"updated_at"`
}

func (MarketingBatchTask) TableName() string {
	return "marketing_batch_tasks"
}
