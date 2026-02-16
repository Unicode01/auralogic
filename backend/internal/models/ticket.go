package models

import (
	"time"

	"gorm.io/gorm"
)

// TicketStatus 工单状态
type TicketStatus string

const (
	TicketStatusOpen       TicketStatus = "open"        // 待处理
	TicketStatusProcessing TicketStatus = "processing"  // 处理中
	TicketStatusResolved   TicketStatus = "resolved"    // 已解决
	TicketStatusClosed     TicketStatus = "closed"      // 已关闭
)

// TicketPriority 工单优先级
type TicketPriority string

const (
	TicketPriorityLow    TicketPriority = "low"    // 低
	TicketPriorityNormal TicketPriority = "normal" // 普通
	TicketPriorityHigh   TicketPriority = "high"   // 高
	TicketPriorityUrgent TicketPriority = "urgent" // 紧急
)

// Ticket 工单模型
type Ticket struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	TicketNo  string         `gorm:"type:varchar(50);uniqueIndex;not null" json:"ticket_no"`
	UserID    uint           `gorm:"index;not null" json:"user_id"`
	User      *User          `gorm:"foreignKey:UserID" json:"user,omitempty"`

	// 工单信息
	Subject     string         `gorm:"type:varchar(255);not null" json:"subject"`
	Content     string         `gorm:"type:text;not null" json:"content"`
	Category    string         `gorm:"type:varchar(50)" json:"category,omitempty"`
	Priority    TicketPriority `gorm:"type:varchar(20);default:'normal'" json:"priority"`
	Status      TicketStatus   `gorm:"type:varchar(20);default:'open';index" json:"status"`

	// 处理人
	AssignedTo   *uint  `gorm:"index" json:"assigned_to,omitempty"`
	AssignedUser *User  `gorm:"foreignKey:AssignedTo" json:"assigned_user,omitempty"`

	// 最后消息信息（用于列表显示）
	LastMessageAt      *time.Time `gorm:"index" json:"last_message_at,omitempty"`
	LastMessagePreview string     `gorm:"type:varchar(200)" json:"last_message_preview,omitempty"`
	LastMessageBy      string     `gorm:"type:varchar(20)" json:"last_message_by,omitempty"` // user/admin

	// 未读消息数
	UnreadCountUser  int `gorm:"default:0" json:"unread_count_user"`
	UnreadCountAdmin int `gorm:"default:0" json:"unread_count_admin"`

	// 时间戳
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	ClosedAt  *time.Time     `json:"closed_at,omitempty"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Ticket) TableName() string {
	return "tickets"
}

// TicketMessage 工单消息
type TicketMessage struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	TicketID uint   `gorm:"index;not null" json:"ticket_id"`
	Ticket   *Ticket `gorm:"foreignKey:TicketID" json:"-"`

	// 发送者信息
	SenderType string `gorm:"type:varchar(20);not null" json:"sender_type"` // user/admin
	SenderID   uint   `gorm:"index;not null" json:"sender_id"`
	SenderName string `gorm:"type:varchar(100)" json:"sender_name"`

	// 消息内容
	Content     string `gorm:"type:text;not null" json:"content"`
	ContentType string `gorm:"type:varchar(20);default:'text'" json:"content_type"` // text/image/order

	// 附加数据（如订单授权信息）
	Metadata JSON `gorm:"type:text" json:"metadata,omitempty"`

	// 已读状态
	IsReadByUser  bool `gorm:"default:false" json:"is_read_by_user"`
	IsReadByAdmin bool `gorm:"default:false" json:"is_read_by_admin"`

	CreatedAt time.Time `json:"created_at"`
}

func (TicketMessage) TableName() string {
	return "ticket_messages"
}

// TicketOrderAccess 工单订单授权
// 用户可以通过工单授权管理员查看/编辑特定订单
type TicketOrderAccess struct {
	ID       uint    `gorm:"primaryKey" json:"id"`
	TicketID uint    `gorm:"index;not null" json:"ticket_id"`
	Ticket   *Ticket `gorm:"foreignKey:TicketID" json:"-"`
	OrderID  uint    `gorm:"index;not null" json:"order_id"`
	Order    *Order  `gorm:"foreignKey:OrderID" json:"order,omitempty"`

	// 授权者
	GrantedBy uint  `gorm:"not null" json:"granted_by"`
	Granter   *User `gorm:"foreignKey:GrantedBy" json:"-"`

	// 授权权限
	CanView         bool `gorm:"default:true" json:"can_view"`
	CanEdit         bool `gorm:"default:false" json:"can_edit"`
	CanViewPrivacy  bool `gorm:"default:false" json:"can_view_privacy"` // 是否可查看隐私保护订单的详细信息

	// 有效期（可选）
	ExpiresAt *time.Time `json:"expires_at,omitempty"`

	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (TicketOrderAccess) TableName() string {
	return "ticket_order_access"
}

// IsExpired 检查授权是否过期
func (a *TicketOrderAccess) IsExpired() bool {
	if a.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*a.ExpiresAt)
}
