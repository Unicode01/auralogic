package models

import "time"

// SerialGenerationTask 持久化的序列号生成任务，用于后台恢复与重试。
type SerialGenerationTask struct {
	ID          uint                   `gorm:"primaryKey" json:"id"`
	OrderID     uint                   `gorm:"not null;uniqueIndex" json:"order_id"`
	Status      SerialGenerationStatus `gorm:"type:varchar(20);not null;index" json:"status"`
	RetryCount  int                    `gorm:"not null;default:0" json:"retry_count"`
	LastError   string                 `gorm:"type:text" json:"last_error,omitempty"`
	NextRunAt   time.Time              `gorm:"index" json:"next_run_at"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

func (SerialGenerationTask) TableName() string {
	return "serial_generation_tasks"
}
