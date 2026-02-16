package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Config 主配置结构
type Config struct {
	App                AppConfig                `json:"app"`
	Database           DatabaseConfig           `json:"database"`
	Redis              RedisConfig              `json:"redis"`
	JWT                JWTConfig                `json:"jwt"`
	OAuth              OAuthConfig              `json:"oauth"`
	SMTP               SMTPConfig               `json:"smtp"`
	Security           SecurityConfig           `json:"security"`
	RateLimit          RateLimitConfig          `json:"rate_limit"`
	Log                LogConfig                `json:"log"`
	Order              OrderConfig              `json:"order"`
	MagicLink          MagicLinkConfig          `json:"magic_link"`
	Form               FormConfig               `json:"form"`
	Upload             UploadConfig             `json:"upload"`
	Ticket             TicketConfig             `json:"ticket"`
	Serial             SerialConfig             `json:"serial"`
	Customization      CustomizationConfig      `json:"customization"`
	EmailNotifications EmailNotificationsConfig `json:"email_notifications"`
	Analytics          AnalyticsConfig          `json:"analytics"`
}

// AppConfig 应用配置
type AppConfig struct {
	Name         string `json:"name"`
	Env          string `json:"env"`
	Port         int    `json:"port"`
	URL          string `json:"url"`
	Debug        bool   `json:"debug"`
	DefaultTheme string `json:"default_theme"` // light, dark, system
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Driver          string `json:"driver"`
	Host            string `json:"host"`
	Port            int    `json:"port"`
	Name            string `json:"name"`
	User            string `json:"user"`
	Password        string `json:"password"`
	SSLMode         string `json:"ssl_mode"`
	MaxIdleConns    int    `json:"max_idle_conns"`
	MaxOpenConns    int    `json:"max_open_conns"`
	ConnMaxLifetime int    `json:"conn_max_lifetime"`
}

// RedisConfig Redis配置
type RedisConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Password string `json:"password"`
	DB       int    `json:"db"`
	PoolSize int    `json:"pool_size"`
}

// JWTConfig JWT配置
type JWTConfig struct {
	Secret             string `json:"secret"`
	ExpireHours        int    `json:"expire_hours"`
	RefreshExpireHours int    `json:"refresh_expire_hours"`
}

// OAuthProviderConfig OAuth提供商配置
type OAuthProviderConfig struct {
	Enabled      bool   `json:"enabled"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RedirectURL  string `json:"redirect_url"`
	APIBaseURL   string `json:"api_base_url"`
}

// OAuthConfig OAuth配置
type OAuthConfig struct {
	Google OAuthProviderConfig `json:"google"`
	Github OAuthProviderConfig `json:"github"`
}

// SMTPConfig SMTP邮件配置
type SMTPConfig struct {
	Enabled   bool   `json:"enabled"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	User      string `json:"user"`
	Password  string `json:"password"`
	FromEmail string `json:"from_email"`
	FromName  string `json:"from_name"`
}

// CORSConfig CORS配置
type CORSConfig struct {
	AllowedOrigins []string `json:"allowed_origins"`
	AllowedMethods []string `json:"allowed_methods"`
	AllowedHeaders []string `json:"allowed_headers"`
	MaxAge         int      `json:"max_age"`
}

// LoginConfig 登录配置
type LoginConfig struct {
	AllowPasswordLogin       bool `json:"allow_password_login"`
	AllowRegistration        bool `json:"allow_registration"`
	RequireEmailVerification bool `json:"require_email_verification"`
}

// PasswordPolicyConfig Password策略配置
type PasswordPolicyConfig struct {
	MinLength        int  `json:"min_length"`
	RequireUppercase bool `json:"require_uppercase"`
	RequireLowercase bool `json:"require_lowercase"`
	RequireNumber    bool `json:"require_number"`
	RequireSpecial   bool `json:"require_special"`
}

// CaptchaConfig 验证码配置
type CaptchaConfig struct {
	Provider              string `json:"provider"`                 // none, cloudflare, google, builtin
	SiteKey               string `json:"site_key"`                 // Cloudflare/Google 的站点密钥
	SecretKey             string `json:"secret_key"`               // Cloudflare/Google 的服务端密钥
	EnableForLogin        bool   `json:"enable_for_login"`         // 登录时是否需要验证码
	EnableForRegister     bool   `json:"enable_for_register"`      // 注册时是否需要验证码
	EnableForSerialVerify bool   `json:"enable_for_serial_verify"` // 序列号验证时是否需要验证码
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	CORS           CORSConfig           `json:"cors"`
	Login          LoginConfig          `json:"login"`
	PasswordPolicy PasswordPolicyConfig `json:"password_policy"`
	Captcha        CaptchaConfig        `json:"captcha"`
	IPHeader       string               `json:"ip_header"`       // 获取真实IP的header名称，如 "CF-Connecting-IP", "X-Real-IP", "X-Forwarded-For"
	TrustedProxies []string             `json:"trusted_proxies"` // Trusted reverse proxies CIDRs/IPs. Only trusted peers can supply IPHeader.
}

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	Enabled      bool `json:"enabled"`
	API          int  `json:"api"`
	UserLogin    int  `json:"user_login"`
	UserRequest  int  `json:"user_request"`
	AdminRequest int  `json:"admin_request"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level    string `json:"level"`
	Format   string `json:"format"`
	Output   string `json:"output"`
	FilePath string `json:"file_path"`
}

// OrderConfig Order配置
type OrderConfig struct {
	NoPrefix             string             `json:"no_prefix"`
	AutoCancelHours      int                `json:"auto_cancel_hours"`
	Currency             string             `json:"currency"` // 货币单位: CNY, USD, EUR, JPY, etc.
	StockDisplay         StockDisplayConfig `json:"stock_display"`
	VirtualDeliveryOrder string             `json:"virtual_delivery_order"` // 虚拟库存发货顺序: random(随机), newest(先发新库存), oldest(先发老库存)
}

// StockDisplayConfig 库存显示配置
type StockDisplayConfig struct {
	Mode               string `json:"mode"`                 // exact: 显示具体数量, level: 显示量级, hidden: 不显示
	LowStockThreshold  int    `json:"low_stock_threshold"`  // 低库存阈值
	HighStockThreshold int    `json:"high_stock_threshold"` // 高库存阈值
}

// MagicLinkConfig 快速登录配置
type MagicLinkConfig struct {
	ExpireMinutes int `json:"expire_minutes"`
	MaxUses       int `json:"max_uses"`
}

// FormConfig 表单配置
type FormConfig struct {
	ExpireHours int `json:"expire_hours"`
}

// UploadConfig 文件上传配置
type UploadConfig struct {
	Dir          string   `json:"dir"`           // 上传目录
	MaxSize      int64    `json:"max_size"`      // 最大文件大小（字节）
	AllowedTypes []string `json:"allowed_types"` // 允许的文件类型
}

// TicketAttachmentConfig 工单附件配置
type TicketAttachmentConfig struct {
	EnableImage       bool     `json:"enable_image"`        // 是否允许上传图片
	EnableVoice       bool     `json:"enable_voice"`        // 是否允许上传语音
	MaxImageSize      int64    `json:"max_image_size"`      // 最大图片大小（字节）
	MaxVoiceSize      int64    `json:"max_voice_size"`      // 最大语音大小（字节）
	MaxVoiceDuration  int      `json:"max_voice_duration"`  // 最大语音时长（秒）
	AllowedImageTypes []string `json:"allowed_image_types"` // 允许的图片类型
	RetentionDays     int      `json:"retention_days"`      // 附件保存天数，0表示永久保存
}

// TicketConfig 工单配置
type TicketConfig struct {
	Enabled          bool                    `json:"enabled"`              // 是否启用工单系统
	Categories       []string                `json:"categories"`           // 工单分类列表
	Template         string                  `json:"template"`             // 工单提交模板/格式说明
	MaxContentLength int                     `json:"max_content_length"`   // 工单内容最大字符数，0表示不限制
	AutoCloseHours   int                     `json:"auto_close_hours"`     // 超时无回复自动关闭（小时），0表示不自动关闭
	Attachment       *TicketAttachmentConfig `json:"attachment,omitempty"` // 附件配置
}

// SerialConfig 序列号查询配置
type SerialConfig struct {
	Enabled bool `json:"enabled"` // 是否启用序列号查询功能
}

// PageRule 页面定向规则
type PageRule struct {
	Name      string `json:"name"`       // 规则名称
	Pattern   string `json:"pattern"`    // 匹配模式（正则或固定路径）
	MatchType string `json:"match_type"` // exact | regex
	CSS       string `json:"css"`        // 注入的CSS
	JS        string `json:"js"`         // 注入的JS
	Enabled   bool   `json:"enabled"`    // 是否启用
}

// EmailNotificationsConfig 邮件通知配置
type EmailNotificationsConfig struct {
	UserRegister     bool `json:"user_register"`      // 用户注册欢迎邮件
	OrderCreated     bool `json:"order_created"`      // 订单创建/表单提交
	OrderPaid        bool `json:"order_paid"`         // 付款确认
	OrderShipped     bool `json:"order_shipped"`      // 订单发货
	OrderCompleted   bool `json:"order_completed"`    // 订单完成
	OrderCancelled   bool `json:"order_cancelled"`    // 订单取消
	OrderResubmit    bool `json:"order_resubmit"`     // 需要重填信息
	TicketCreated    bool `json:"ticket_created"`     // 新工单（通知管理员）
	TicketAdminReply bool `json:"ticket_admin_reply"` // 客服回复（通知用户）
	TicketUserReply  bool `json:"ticket_user_reply"`  // 用户回复（通知管理员）
	TicketResolved   bool `json:"ticket_resolved"`    // 工单已解决
}

// CustomizationConfig 个性化配置
type CustomizationConfig struct {
	PrimaryColor string     `json:"primary_color"` // 主题主色调 (HSL格式, 如 "217.2 91% 60%")
	LogoURL      string     `json:"logo_url"`      // 自定义Logo URL
	FaviconURL   string     `json:"favicon_url"`   // 自定义Favicon URL
	PageRules    []PageRule `json:"page_rules"`    // 页面定向规则
}

// AnalyticsConfig 数据分析配置
type AnalyticsConfig struct {
	Enabled bool `json:"enabled"` // 是否启用数据分析功能
}

// AdminConfig Admin配置
type AdminConfig struct {
	SuperAdmin struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
	} `json:"super_admin"`
}

var (
	instance *Config
	once     sync.Once
	mu       sync.RWMutex // 用于热更新配置的读写锁
)

// LoadConfig 加载配置文件
func LoadConfig(configPath string) (*Config, error) {
	var err error
	once.Do(func() {
		// 如果未指定配置文件路径，使用默认路径
		if configPath == "" {
			configPath = "config/config.json"
		}

		// 读取配置文件
		data, readErr := os.ReadFile(configPath)
		if readErr != nil {
			err = fmt.Errorf("failed to read config file: %w", readErr)
			return
		}

		// 解析JSON
		var cfg Config
		if parseErr := json.Unmarshal(data, &cfg); parseErr != nil {
			err = fmt.Errorf("failed to parse config file: %w", parseErr)
			return
		}

		// 验证配置
		if validateErr := cfg.Validate(); validateErr != nil {
			err = fmt.Errorf("invalid config: %w", validateErr)
			return
		}

		instance = &cfg
	})

	if err != nil {
		return nil, err
	}

	return instance, nil
}

// GetConfig get配置实例
func GetConfig() *Config {
	mu.RLock()
	defer mu.RUnlock()
	return instance
}

// ReloadConfig 重新加载配置文件（热更新）
func ReloadConfig() error {
	configPath := GetConfigPath()

	// 读取配置文件
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// 解析JSON
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	// 验证配置
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// 更新内存中的配置实例
	mu.Lock()
	defer mu.Unlock()

	// 直接更新实例的各个字段（保持指针不变）
	instance.App = cfg.App
	instance.SMTP = cfg.SMTP
	instance.Security = cfg.Security
	instance.RateLimit = cfg.RateLimit
	instance.Log = cfg.Log
	instance.Order = cfg.Order
	instance.MagicLink = cfg.MagicLink
	instance.Form = cfg.Form
	instance.Upload = cfg.Upload
	instance.OAuth = cfg.OAuth
	instance.Ticket = cfg.Ticket
	instance.Serial = cfg.Serial
	instance.Customization = cfg.Customization
	instance.EmailNotifications = cfg.EmailNotifications
	instance.Analytics = cfg.Analytics
	// 注意：Database、Redis、JWT 通常需要重启才能生效，这里不更新

	return nil
}

// Validate 验证配置
func (c *Config) Validate() error {
	// 验证应用配置
	if c.App.Name == "" {
		return fmt.Errorf("app.name is required")
	}
	if c.App.Port == 0 {
		c.App.Port = 8080
	}

	// 验证数据库配置
	if c.Database.Driver == "" {
		return fmt.Errorf("database.driver is required")
	}
	// SQLite 不need host
	if c.Database.Driver != "sqlite" && c.Database.Host == "" {
		return fmt.Errorf("database.host is required")
	}
	if c.Database.Name == "" {
		return fmt.Errorf("database.name is required (for sqlite, this is the database file path)")
	}

	// 验证JWT配置
	if c.JWT.Secret == "" {
		return fmt.Errorf("jwt.secret is required")
	}
	if len(c.JWT.Secret) < 32 {
		return fmt.Errorf("jwt.secret must be at least 32 characters")
	}

	// 设置默认值
	if c.JWT.ExpireHours == 0 {
		c.JWT.ExpireHours = 24
	}
	if c.JWT.RefreshExpireHours == 0 {
		c.JWT.RefreshExpireHours = 168
	}
	if c.Order.NoPrefix == "" {
		c.Order.NoPrefix = "ORD"
	}
	if c.Order.StockDisplay.Mode == "" {
		c.Order.StockDisplay.Mode = "exact"
	}
	if c.Order.StockDisplay.LowStockThreshold == 0 {
		c.Order.StockDisplay.LowStockThreshold = 10
	}
	if c.Order.StockDisplay.HighStockThreshold == 0 {
		c.Order.StockDisplay.HighStockThreshold = 50
	}
	if c.Order.VirtualDeliveryOrder == "" {
		c.Order.VirtualDeliveryOrder = "random"
	}
	if c.MagicLink.ExpireMinutes == 0 {
		c.MagicLink.ExpireMinutes = 15
	}
	if c.Form.ExpireHours == 0 {
		c.Form.ExpireHours = 24
	}
	if c.Upload.Dir == "" {
		c.Upload.Dir = "uploads"
	}
	if c.Upload.MaxSize == 0 {
		c.Upload.MaxSize = 5 * 1024 * 1024 // 默认5MB
	}
	if len(c.Upload.AllowedTypes) == 0 {
		c.Upload.AllowedTypes = []string{".jpg", ".jpeg", ".png", ".gif", ".webp"}
	}

	// 工单附件默认配置
	if c.Ticket.Attachment == nil {
		c.Ticket.Attachment = &TicketAttachmentConfig{
			EnableImage:       true,
			EnableVoice:       true,
			MaxImageSize:      5 * 1024 * 1024,  // 5MB
			MaxVoiceSize:      10 * 1024 * 1024, // 10MB
			MaxVoiceDuration:  60,               // 60秒
			AllowedImageTypes: []string{".jpg", ".jpeg", ".png", ".gif", ".webp"},
		}
	}

	// 验证码默认配置
	if c.Security.Captcha.Provider == "" {
		c.Security.Captcha.Provider = "none"
	}

	// 邮件通知默认配置（首次加载时全部开启）
	// 注意：此处不设置默认值，零值false表示未配置时不发送
	// 管理员需要在设置页面手动开启想要的通知

	// 数据分析默认开启
	// 注意：零值false表示未配置，这里不设置默认值
	// 如果配置文件中没有analytics字段，则默认为false（关闭）
	// 管理员需要在设置页面手动开启

	return nil
}

// LoadAdminConfig 加载Admin配置
func LoadAdminConfig(configPath string) (*AdminConfig, error) {
	if configPath == "" {
		configPath = "config/admin.json"
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read admin config file: %w", err)
	}

	var cfg AdminConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse admin config file: %w", err)
	}

	return &cfg, nil
}

// GetDSN get数据库连接字符串
func (c *DatabaseConfig) GetDSN() string {
	switch c.Driver {
	case "postgres":
		return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode)
	case "mysql":
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			c.User, c.Password, c.Host, c.Port, c.Name)
	case "sqlite":
		// SQLite使用Name字段作为数据库文件路径
		if c.Name == "" {
			return "auralogic.db?_busy_timeout=5000&_journal_mode=WAL"
		}
		// If the path already has query params, don't override (InitDatabase also applies PRAGMAs).
		if strings.Contains(c.Name, "?") {
			return c.Name
		}
		return c.Name + "?_busy_timeout=5000&_journal_mode=WAL"
	default:
		return ""
	}
}

// GetRedisAddr getRedisAddress
func (c *RedisConfig) GetRedisAddr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// GetConfigPath get配置文件路径
func GetConfigPath() string {
	// 优先使用环境变量
	if path := os.Getenv("CONFIG_PATH"); path != "" {
		return path
	}

	// 查找配置文件
	possiblePaths := []string{
		"config/config.json",
		"../config/config.json",
		"../../config/config.json",
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			absPath, _ := filepath.Abs(path)
			return absPath
		}
	}

	return "config/config.json"
}
