package models

import "time"

type MarketingBatchStatus string

const (
	MarketingBatchStatusQueued    MarketingBatchStatus = "queued"
	MarketingBatchStatusRunning   MarketingBatchStatus = "running"
	MarketingBatchStatusCompleted MarketingBatchStatus = "completed"
	MarketingBatchStatusFailed    MarketingBatchStatus = "failed"
)

// MarketingBatch tracks one marketing send operation.
type MarketingBatch struct {
	ID                 uint                 `gorm:"primaryKey" json:"id"`
	BatchNo            string               `gorm:"type:varchar(64);uniqueIndex;not null" json:"batch_no"`
	Title              string               `gorm:"type:varchar(500);not null" json:"title"`
	Content            string               `gorm:"type:text;not null" json:"-"`
	SendEmail          bool                 `json:"send_email"`
	SendSMS            bool                 `json:"send_sms"`
	TargetAll          bool                 `json:"target_all"`
	Status             MarketingBatchStatus `gorm:"type:varchar(20);not null;default:'queued';index" json:"status"`
	TotalTasks         int                  `gorm:"default:0" json:"total_tasks"`
	ProcessedTasks     int                  `gorm:"default:0" json:"processed_tasks"`
	RequestedUserCount int                  `json:"requested_user_count"`
	TargetedUsers      int                  `json:"targeted_users"`
	EmailSent          int                  `json:"email_sent"`
	EmailFailed        int                  `json:"email_failed"`
	EmailSkipped       int                  `json:"email_skipped"`
	SmsSent            int                  `json:"sms_sent"`
	SmsFailed          int                  `json:"sms_failed"`
	SmsSkipped         int                  `json:"sms_skipped"`
	FailedReason       string               `gorm:"type:text" json:"failed_reason,omitempty"`

	OperatorID   *uint  `gorm:"index" json:"operator_id,omitempty"`
	Operator     *User  `gorm:"foreignKey:OperatorID" json:"operator,omitempty"`
	OperatorName string `gorm:"type:varchar(100)" json:"operator_name,omitempty"`

	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CreatedAt   time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (MarketingBatch) TableName() string {
	return "marketing_batches"
}
