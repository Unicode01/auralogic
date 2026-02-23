package models

import (
	"time"

	"gorm.io/gorm"
)

// OrderStatus Order状态
type OrderStatus string

const (
	OrderStatusPendingPayment OrderStatus = "pending_payment" // 待付款
	OrderStatusDraft          OrderStatus = "draft"           // 草稿（等待User填写发货Info）
	OrderStatusPending        OrderStatus = "pending"         // 待发货（已填写发货Info）
	OrderStatusNeedResubmit   OrderStatus = "need_resubmit"   // need重填Info
	OrderStatusShipped        OrderStatus = "shipped"         // 已发货
	OrderStatusCompleted      OrderStatus = "completed"       // 已完成
	OrderStatusCancelled      OrderStatus = "cancelled"       // 已取消
	OrderStatusRefunded       OrderStatus = "refunded"        // 已退款
)

// OrderItem OrderProduct项
type OrderItem struct {
	SKU         string                 `json:"sku"`
	Name        string                 `json:"name"`
	Quantity    int                    `json:"quantity"`
	ImageURL    string                 `json:"image_url,omitempty"`
	Attributes  map[string]interface{} `json:"attributes,omitempty"`
	ProductType ProductType            `json:"product_type,omitempty"` // physical(实物), virtual(虚拟)
}

// Order Order模型
type Order struct {
	ID      uint   `gorm:"primaryKey" json:"id"`
	OrderNo string `gorm:"type:varchar(50);uniqueIndex;not null" json:"order_no"`
	UserID  *uint  `gorm:"index" json:"user_id,omitempty"`
	User    *User  `gorm:"foreignKey:UserID" json:"user,omitempty"`

	// OrderInfo
	Items []OrderItem `gorm:"type:text;serializer:json;not null" json:"items"`

	// 实际分配的属性（盲盒模式）
	ActualAttributes JSON `gorm:"type:json" json:"actual_attributes,omitempty"` // 盲盒Product实际分配的属性

	// Inventory绑定关系（内部使用，不对外暴露）
	// Key: Order项索引(0,1,2...), Value: InventoryID
	InventoryBindings map[int]uint `gorm:"type:text;serializer:json" json:"-"` // json:"-" 表示不序列化给前端

	// 状态
	Status OrderStatus `gorm:"type:varchar(30);not null;default:'draft';index" json:"status"`

	// 收货Info
	ReceiverName     string `gorm:"type:varchar(100)" json:"receiver_name,omitempty"`
	PhoneCode        string `gorm:"type:varchar(10);default:'+86'" json:"phone_code,omitempty"` // 手机区号
	ReceiverPhone    string `gorm:"type:varchar(50)" json:"receiver_phone,omitempty"`
	ReceiverEmail    string `gorm:"type:varchar(255)" json:"receiver_email,omitempty"`
	ReceiverCountry  string `gorm:"type:varchar(100);default:'CN'" json:"receiver_country,omitempty"` // 收货国家代码
	ReceiverProvince string `gorm:"type:varchar(50)" json:"receiver_province,omitempty"`
	ReceiverCity     string `gorm:"type:varchar(50)" json:"receiver_city,omitempty"`
	ReceiverDistrict string `gorm:"type:varchar(50)" json:"receiver_district,omitempty"`
	ReceiverAddress  string `gorm:"type:text" json:"receiver_address,omitempty"`
	ReceiverPostcode string `gorm:"type:varchar(20)" json:"receiver_postcode,omitempty"`

	// 隐私保护
	PrivacyProtected bool `gorm:"default:false" json:"privacy_protected"`

	// 物流Info
	TrackingNo   string     `gorm:"type:varchar(100);index" json:"tracking_no,omitempty"`
	ShippedAt    *time.Time `json:"shipped_at,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	CompletedBy  *uint      `json:"completed_by,omitempty"`
	UserFeedback string     `gorm:"type:text" json:"user_feedback,omitempty"`

	// 表单访问Token
	FormToken       *string    `gorm:"type:varchar(255);uniqueIndex" json:"form_token,omitempty"`
	FormSubmittedAt *time.Time `json:"form_submitted_at,omitempty"`
	FormExpiresAt   *time.Time `json:"form_expires_at,omitempty"`

	// 邮件通知
	UserEmail                 string `gorm:"type:varchar(255)" json:"user_email,omitempty"`
	EmailNotificationsEnabled bool   `gorm:"default:true" json:"email_notifications_enabled"`

	// 优惠码
	PromoCodeID  *uint   `gorm:"index" json:"promo_code_id,omitempty"`
	PromoCodeStr string  `gorm:"type:varchar(50)" json:"promo_code,omitempty"`
	DiscountAmount float64 `gorm:"type:decimal(10,2);default:0" json:"discount_amount"`

	// 金额
	TotalAmount float64 `gorm:"type:decimal(10,2);default:0" json:"total_amount"`
	Currency    string  `gorm:"type:varchar(10);default:'CNY'" json:"currency"`

	// 备注
	Remark      string `gorm:"type:text" json:"remark,omitempty"`
	AdminRemark string `gorm:"type:text" json:"admin_remark,omitempty"`

	// 来源
	Source           string `gorm:"type:varchar(50);default:'api'" json:"source"`
	SourcePlatform   string `gorm:"type:varchar(100)" json:"source_platform,omitempty"`
	ExternalUserID   string `gorm:"type:varchar(100);index" json:"external_user_id,omitempty"`
	ExternalUserName string `gorm:"type:varchar(100)" json:"external_user_name,omitempty"` // 第三方平台的User名
	ExternalOrderID  string `gorm:"type:varchar(100)" json:"external_order_id,omitempty"`

	// 分配Info
	AssignedTo *uint      `json:"assigned_to,omitempty"`
	AssignedAt *time.Time `json:"assigned_at,omitempty"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName 指定表名
func (Order) TableName() string {
	return "orders"
}

// MaskSensitiveInfo 打码敏感Info
func (o *Order) MaskSensitiveInfo() {
	if !o.PrivacyProtected {
		return
	}

	o.ReceiverName = "***"
	if len(o.ReceiverPhone) > 7 {
		o.ReceiverPhone = o.ReceiverPhone[:3] + "****" + o.ReceiverPhone[len(o.ReceiverPhone)-4:]
	}
	// Email不再打码，保持原样显示
	// 保留省市区，详细Address打码
	o.ReceiverAddress = "***"
}
