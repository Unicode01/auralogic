package config

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"auralogic/internal/pkg/pluginutil"
)

// Config 主配置结构
type Config struct {
	App                AppConfig                `json:"app"`
	Database           DatabaseConfig           `json:"database"`
	Redis              RedisConfig              `json:"redis"`
	JWT                JWTConfig                `json:"jwt"`
	OAuth              OAuthConfig              `json:"oauth"`
	SMTP               SMTPConfig               `json:"smtp"`
	SMS                SMSConfig                `json:"sms"`
	Security           SecurityConfig           `json:"security"`
	RateLimit          RateLimitConfig          `json:"rate_limit"`
	EmailRateLimit     MessageRateLimit         `json:"email_rate_limit"`
	SMSRateLimit       MessageRateLimit         `json:"sms_rate_limit"`
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
	Plugin             PluginPlatformConfig     `json:"plugin"`
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

// SMSConfig 短信配置
type SMSConfig struct {
	Enabled            bool              `json:"enabled"`
	Provider           string            `json:"provider"` // aliyun, aliyun_dypns, twilio, custom
	AliyunAccessKeyID  string            `json:"aliyun_access_key_id"`
	AliyunAccessSecret string            `json:"aliyun_access_secret"`
	AliyunSignName     string            `json:"aliyun_sign_name"`
	AliyunTemplateCode string            `json:"aliyun_template_code"`
	Templates          SMSTemplates      `json:"templates"`
	DYPNSCodeLength    int               `json:"dypns_code_length"`
	TwilioAccountSID   string            `json:"twilio_account_sid"`
	TwilioAuthToken    string            `json:"twilio_auth_token"`
	TwilioFromNumber   string            `json:"twilio_from_number"`
	CustomURL          string            `json:"custom_url"`
	CustomMethod       string            `json:"custom_method"`
	CustomHeaders      map[string]string `json:"custom_headers"`
	CustomBodyTemplate string            `json:"custom_body_template"`
}

// SMSTemplates 各操作的短信模板配置
type SMSTemplates struct {
	Login         string `json:"login"`
	Register      string `json:"register"`
	ResetPassword string `json:"reset_password"`
	BindPhone     string `json:"bind_phone"`
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
	AllowGuestProductBrowse  bool `json:"allow_guest_product_browse"`
	RequireEmailVerification bool `json:"require_email_verification"`
	AllowEmailLogin          bool `json:"allow_email_login"`
	AllowPasswordReset       bool `json:"allow_password_reset"`
	AllowPhoneLogin          bool `json:"allow_phone_login"`
	AllowPhoneRegister       bool `json:"allow_phone_register"`
	AllowPhonePasswordReset  bool `json:"allow_phone_password_reset"`
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
	EnableForBind         bool   `json:"enable_for_bind"`          // 绑定邮箱/手机时是否需要验证码
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

// MessageRateLimit 邮件/短信发送频率限制
type MessageRateLimit struct {
	Hourly       int    `json:"hourly"`        // max per recipient per hour, 0=unlimited
	Daily        int    `json:"daily"`         // max per recipient per day, 0=unlimited
	ExceedAction string `json:"exceed_action"` // "cancel" or "delay"
}

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	Enabled       bool `json:"enabled"`
	API           int  `json:"api"`
	UserLogin     int  `json:"user_login"`
	UserRequest   int  `json:"user_request"`
	AdminRequest  int  `json:"admin_request"`
	OrderCreate   int  `json:"order_create"`
	PaymentInfo   int  `json:"payment_info"`
	PaymentSelect int  `json:"payment_select"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level    string `json:"level"`
	Format   string `json:"format"`
	Output   string `json:"output"`
	FilePath string `json:"file_path"`
}

// InitLogger 根据 LogConfig 初始化日志系统。
// 使用 log/slog 作为结构化日志后端，同时桥接标准 log 包的输出。
// 返回当前活动日志文件句柄（如果 output=file），调用方可在进程退出时调用 CloseLogger。
func InitLogger(cfg *LogConfig) (*os.File, error) {
	loggerMu.Lock()
	defer loggerMu.Unlock()
	return initLoggerLocked(cfg, true)
}

func initLoggerLocked(cfg *LogConfig, closeExisting bool) (*os.File, error) {
	var previousLogFile *os.File
	if closeExisting {
		previousLogFile = currentLogFile
	}

	// 解析日志级别
	var level slog.Level
	switch strings.ToLower(cfg.Level) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	// 确定输出目标
	var writer io.Writer
	var logFile *os.File
	switch strings.ToLower(cfg.Output) {
	case "file":
		if cfg.FilePath == "" {
			cfg.FilePath = "auralogic.log"
		}
		f, err := os.OpenFile(cfg.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file %s: %w", cfg.FilePath, err)
		}
		writer = f
		logFile = f
	default:
		writer = os.Stdout
	}

	// 根据格式创建 handler
	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if strings.ToLower(cfg.Format) == "json" {
		handler = slog.NewJSONHandler(writer, opts)
	} else {
		handler = slog.NewTextHandler(writer, opts)
	}

	// 设置为默认 logger（同时桥接标准 log 包）
	slog.SetDefault(slog.New(handler))
	log.SetOutput(writer)
	currentLogFile = logFile
	if closeExisting && previousLogFile != nil && previousLogFile != logFile {
		_ = previousLogFile.Close()
	}

	return logFile, nil
}

// ReloadLogger 根据当前配置重建日志输出目标和日志级别。
func ReloadLogger() error {
	cfg := GetConfig()
	if cfg == nil {
		return fmt.Errorf("config is not loaded")
	}

	loggerMu.Lock()
	defer loggerMu.Unlock()
	_, err := initLoggerLocked(&cfg.Log, true)
	return err
}

// CloseLogger 关闭当前活动日志文件句柄。
func CloseLogger() {
	loggerMu.Lock()
	defer loggerMu.Unlock()
	if currentLogFile != nil {
		_ = currentLogFile.Close()
		currentLogFile = nil
	}
}

// OrderConfig Order配置
type OrderHighConcurrencyProtectionConfig struct {
	Enabled       bool   `json:"enabled"`
	Mode          string `json:"mode"`            // auto, memory, redis
	MaxInFlight   int    `json:"max_inflight"`    // 每条热点链路允许的最大并发写入数
	WaitTimeoutMs int    `json:"wait_timeout_ms"` // 等待获取并发槽位的超时时间
	RedisLeaseMs  int    `json:"redis_lease_ms"`  // Redis 分布式槽位租约时长
}

type OrderConfig struct {
	NoPrefix                       string                               `json:"no_prefix"`
	AutoCancelHours                int                                  `json:"auto_cancel_hours"`
	MaxPendingPaymentOrdersPerUser int                                  `json:"max_pending_payment_orders_per_user"`
	MaxPaymentPollingTasksPerUser  int                                  `json:"max_payment_polling_tasks_per_user"`
	MaxPaymentPollingTasksGlobal   int                                  `json:"max_payment_polling_tasks_global"`
	Currency                       string                               `json:"currency"`                  // 货币单位: CNY, USD, EUR, JPY, etc.
	MaxOrderItems                  int                                  `json:"max_order_items"`           // 单个订单最大商品项数，0表示使用默认值100
	MaxItemQuantity                int                                  `json:"max_item_quantity"`         // 单个商品项最大数量，0表示使用默认值9999
	ShowVirtualStockRemark         bool                                 `json:"show_virtual_stock_remark"` // 是否在用户侧显示虚拟产品备注
	StockDisplay                   StockDisplayConfig                   `json:"stock_display"`
	VirtualDeliveryOrder           string                               `json:"virtual_delivery_order"`        // 虚拟库存发货顺序: random(随机), newest(先发新库存), oldest(先发老库存)
	VirtualScriptTimeoutMaxMs      int                                  `json:"virtual_script_timeout_max_ms"` // 虚拟脚本发货允许的最大执行时长
	Invoice                        InvoiceConfig                        `json:"invoice"`
	HighConcurrencyProtection      OrderHighConcurrencyProtectionConfig `json:"high_concurrency_protection"`
}

// InvoiceConfig 账单/发票配置
type InvoiceConfig struct {
	Enabled        bool   `json:"enabled"`
	TemplateType   string `json:"template_type"`   // "builtin" or "custom"
	CustomTemplate string `json:"custom_template"` // 自定义 HTML 模板
	CompanyName    string `json:"company_name"`
	CompanyAddress string `json:"company_address"`
	CompanyPhone   string `json:"company_phone"`
	CompanyEmail   string `json:"company_email"`
	CompanyLogo    string `json:"company_logo"` // Logo URL
	TaxID          string `json:"tax_id"`
	FooterText     string `json:"footer_text"`
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

// AuthBrandingConfig 认证页品牌面板配置
type AuthBrandingConfig struct {
	Mode       string `json:"mode"`        // "default" | "custom"
	Title      string `json:"title"`       // 默认模式: 标题(zh)
	TitleEn    string `json:"title_en"`    // 默认模式: 标题(en)
	Subtitle   string `json:"subtitle"`    // 默认模式: 副标题(zh)
	SubtitleEn string `json:"subtitle_en"` // 默认模式: 副标题(en)
	CustomHTML string `json:"custom_html"` // 自定义模式: 完整HTML
}

// CustomizationConfig 个性化配置
type CustomizationConfig struct {
	PrimaryColor string             `json:"primary_color"` // 主题主色调 (HSL格式, 如 "217.2 91% 60%")
	LogoURL      string             `json:"logo_url"`      // 自定义Logo URL
	FaviconURL   string             `json:"favicon_url"`   // 自定义Favicon URL
	PageRules    []PageRule         `json:"page_rules"`    // 页面定向规则
	AuthBranding AuthBrandingConfig `json:"auth_branding"` // 认证页品牌面板
}

// AnalyticsConfig 数据分析配置
type AnalyticsConfig struct {
	Enabled bool `json:"enabled"` // 是否启用数据分析功能
}

// PluginSandboxConfig 插件沙箱配置
type PluginSandboxConfig struct {
	Level              string   `json:"level"`                 // strict | balanced | permissive
	ExecTimeoutMs      int      `json:"exec_timeout_ms"`       // 单次执行超时
	MaxMemoryMB        int      `json:"max_memory_mb"`         // 软限制，仅用于策略与监控提示
	MaxConcurrency     int      `json:"max_concurrency"`       // JS Worker 并发执行上限
	JSWorkerSocketPath string   `json:"js_worker_socket_path"` // Unix socket 路径
	JSWorkerAutoStart  bool     `json:"js_worker_auto_start"`  // 是否由主进程托管启动 JS Worker
	JSWorkerBinary     string   `json:"js_worker_binary"`      // Worker 可执行文件路径
	JSWorkerArgs       []string `json:"js_worker_args"`        // 额外参数
	JSAllowNetwork     bool     `json:"js_allow_network"`      // JS 插件是否允许网络访问（策略位）
	JSAllowFileSystem  bool     `json:"js_allow_file_system"`  // JS 插件是否允许文件系统访问（策略位）
}

// PluginExecutionPolicyConfig 插件执行策略（稳定性/灵活性）
type PluginExecutionPolicyConfig struct {
	HookMaxInFlight           int `json:"hook_max_inflight"`            // Hook 并发闸门
	HookMaxRetries            int `json:"hook_max_retries"`             // Hook 执行失败重试次数
	HookRetryBackoffMs        int `json:"hook_retry_backoff_ms"`        // Hook 重试退避间隔
	HookBeforeTimeoutMs       int `json:"hook_before_timeout_ms"`       // before Hook 超时预算
	HookAfterTimeoutMs        int `json:"hook_after_timeout_ms"`        // after/event Hook 超时预算
	FailureThreshold          int `json:"failure_threshold"`            // 连续失败达到阈值后触发断路器
	FailureCooldownMs         int `json:"failure_cooldown_ms"`          // 故障后冷却窗口，避免坏插件持续进入热路径
	ExecutionLogRetentionDays int `json:"execution_log_retention_days"` // 执行日志保留天数，0=默认值，<0=禁用清理
}

// PluginFrontendPolicyConfig 插件前端渲染策略
type PluginFrontendPolicyConfig struct {
	ForceSanitizeHTML     bool  `json:"force_sanitize_html"`               // 全局强制 sanitize（trusted kill switch）
	SlotAnimationsEnabled *bool `json:"slot_animations_enabled,omitempty"` // 插槽动画全局默认开关，nil=默认启用
}

func (c PluginFrontendPolicyConfig) SlotAnimationsEnabledValue() bool {
	if c.SlotAnimationsEnabled == nil {
		return true
	}
	return *c.SlotAnimationsEnabled
}

// PluginGRPCTransportConfig gRPC 传输安全配置
type PluginGRPCTransportConfig struct {
	Mode               string `json:"mode"`                 // insecure | insecure_local | tls
	CAFile             string `json:"ca_file"`              // CA 证书路径（TLS）
	CertFile           string `json:"cert_file"`            // 客户端证书路径（mTLS，可选）
	KeyFile            string `json:"key_file"`             // 客户端私钥路径（mTLS，可选）
	ServerName         string `json:"server_name"`          // TLS ServerName（可选）
	InsecureSkipVerify bool   `json:"insecure_skip_verify"` // 是否跳过服务端证书校验（仅调试）
}

// PluginPlatformConfig 插件平台配置
type PluginPlatformConfig struct {
	Enabled                bool                        `json:"enabled"`                    // 是否启用插件平台
	AllowedRuntimes        []string                    `json:"allowed_runtimes"`           // 允许 runtime: grpc, js_worker
	AllowedTypes           []string                    `json:"allowed_types"`              // 允许的业务插件类型（空=不限制）
	DefaultRuntime         string                      `json:"default_runtime"`            // 默认 runtime
	ArtifactDir            string                      `json:"artifact_dir"`               // 插件包/解压产物目录（不应暴露到公网静态目录）
	JSFSMaxFiles           int                         `json:"js_fs_max_files"`            // JS 插件可见目录最大文件数（配额）
	JSFSMaxTotalBytes      int64                       `json:"js_fs_max_total_bytes"`      // JS 插件可见目录最大总大小（字节）
	JSFSMaxReadBytes       int64                       `json:"js_fs_max_read_bytes"`       // JS 单次读取文件大小上限（字节）
	JSStorageMaxKeys       int                         `json:"js_storage_max_keys"`        // JS 插件 KV 最大键数量（按插件）
	JSStorageMaxTotalBytes int64                       `json:"js_storage_max_total_bytes"` // JS 插件 KV 最大总容量（字节，key+value）
	JSStorageMaxValueBytes int64                       `json:"js_storage_max_value_bytes"` // JS 插件 KV 单值最大容量（字节）
	Frontend               PluginFrontendPolicyConfig  `json:"frontend"`
	GRPC                   PluginGRPCTransportConfig   `json:"grpc"`
	Sandbox                PluginSandboxConfig         `json:"sandbox"`
	Execution              PluginExecutionPolicyConfig `json:"execution"`
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
	instance       *Config
	once           sync.Once
	mu             sync.RWMutex // 用于热更新配置的读写锁
	loggerMu       sync.Mutex
	currentLogFile *os.File
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
	instance.SMS = cfg.SMS
	instance.Security = cfg.Security
	instance.RateLimit = cfg.RateLimit
	instance.EmailRateLimit = cfg.EmailRateLimit
	instance.SMSRateLimit = cfg.SMSRateLimit
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
	instance.Plugin = cfg.Plugin
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
	normalizedCORSOrigins, err := NormalizeCORSAllowedOrigins(c.Security.CORS.AllowedOrigins)
	if err != nil {
		return err
	}
	c.Security.CORS.AllowedOrigins = normalizedCORSOrigins
	if c.Security.CORS.MaxAge < 0 {
		return fmt.Errorf("security.cors.max_age must be greater than or equal to 0")
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
	if c.Order.VirtualScriptTimeoutMaxMs <= 0 {
		c.Order.VirtualScriptTimeoutMaxMs = 10000
	}
	if c.Order.VirtualScriptTimeoutMaxMs < 100 {
		c.Order.VirtualScriptTimeoutMaxMs = 100
	}
	if c.Order.MaxOrderItems == 0 {
		c.Order.MaxOrderItems = 100
	}
	if c.Order.MaxItemQuantity == 0 {
		c.Order.MaxItemQuantity = 9999
	}
	if c.Order.MaxPendingPaymentOrdersPerUser == 0 {
		c.Order.MaxPendingPaymentOrdersPerUser = 10
	}
	if c.Order.MaxPaymentPollingTasksPerUser == 0 {
		c.Order.MaxPaymentPollingTasksPerUser = 20
	}
	if c.Order.MaxPaymentPollingTasksGlobal == 0 {
		c.Order.MaxPaymentPollingTasksGlobal = 2000
	}
	if c.Order.HighConcurrencyProtection.Mode == "" {
		c.Order.HighConcurrencyProtection.Mode = "auto"
	}
	switch c.Order.HighConcurrencyProtection.Mode {
	case "auto", "memory", "redis":
	default:
		return fmt.Errorf("order.high_concurrency_protection.mode must be one of auto/memory/redis")
	}
	if c.Order.HighConcurrencyProtection.MaxInFlight <= 0 {
		c.Order.HighConcurrencyProtection.MaxInFlight = 8
	}
	if c.Order.HighConcurrencyProtection.WaitTimeoutMs <= 0 {
		c.Order.HighConcurrencyProtection.WaitTimeoutMs = 5000
	}
	if c.Order.HighConcurrencyProtection.RedisLeaseMs <= 0 {
		c.Order.HighConcurrencyProtection.RedisLeaseMs = 30000
	}
	if c.RateLimit.OrderCreate == 0 {
		c.RateLimit.OrderCreate = 30
	}
	if c.RateLimit.PaymentInfo == 0 {
		c.RateLimit.PaymentInfo = 120
	}
	if c.RateLimit.PaymentSelect == 0 {
		c.RateLimit.PaymentSelect = 60
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

	// 插件平台默认配置
	pluginCfgUnset := !c.Plugin.Enabled &&
		c.Plugin.DefaultRuntime == "" &&
		len(c.Plugin.AllowedRuntimes) == 0 &&
		len(c.Plugin.AllowedTypes) == 0 &&
		c.Plugin.ArtifactDir == "" &&
		c.Plugin.JSFSMaxFiles == 0 &&
		c.Plugin.JSFSMaxTotalBytes == 0 &&
		c.Plugin.JSFSMaxReadBytes == 0 &&
		c.Plugin.JSStorageMaxKeys == 0 &&
		c.Plugin.JSStorageMaxTotalBytes == 0 &&
		c.Plugin.JSStorageMaxValueBytes == 0 &&
		!c.Plugin.Frontend.ForceSanitizeHTML &&
		c.Plugin.GRPC.Mode == "" &&
		c.Plugin.GRPC.CAFile == "" &&
		c.Plugin.GRPC.CertFile == "" &&
		c.Plugin.GRPC.KeyFile == "" &&
		c.Plugin.GRPC.ServerName == "" &&
		!c.Plugin.GRPC.InsecureSkipVerify &&
		c.Plugin.Sandbox.Level == "" &&
		c.Plugin.Sandbox.ExecTimeoutMs == 0 &&
		c.Plugin.Sandbox.MaxMemoryMB == 0 &&
		c.Plugin.Sandbox.MaxConcurrency == 0 &&
		c.Plugin.Sandbox.JSWorkerSocketPath == "" &&
		c.Plugin.Sandbox.JSWorkerBinary == "" &&
		len(c.Plugin.Sandbox.JSWorkerArgs) == 0 &&
		!c.Plugin.Sandbox.JSWorkerAutoStart &&
		!c.Plugin.Sandbox.JSAllowNetwork &&
		!c.Plugin.Sandbox.JSAllowFileSystem &&
		c.Plugin.Execution.HookMaxInFlight == 0 &&
		c.Plugin.Execution.HookMaxRetries == 0 &&
		c.Plugin.Execution.HookRetryBackoffMs == 0 &&
		c.Plugin.Execution.HookBeforeTimeoutMs == 0 &&
		c.Plugin.Execution.HookAfterTimeoutMs == 0 &&
		c.Plugin.Execution.FailureThreshold == 0 &&
		c.Plugin.Execution.FailureCooldownMs == 0 &&
		c.Plugin.Execution.ExecutionLogRetentionDays == 0
	if pluginCfgUnset {
		c.Plugin.Enabled = true
	}

	c.Plugin.AllowedRuntimes = normalizeLowerStringList(c.Plugin.AllowedRuntimes)
	if len(c.Plugin.AllowedRuntimes) == 0 {
		c.Plugin.AllowedRuntimes = []string{"grpc"}
	}
	for _, runtime := range c.Plugin.AllowedRuntimes {
		switch runtime {
		case "grpc", "js_worker":
		default:
			return fmt.Errorf("plugin.allowed_runtimes contains unsupported runtime %q", runtime)
		}
	}

	c.Plugin.AllowedTypes = normalizeLowerStringList(c.Plugin.AllowedTypes)
	c.Plugin.DefaultRuntime = strings.ToLower(strings.TrimSpace(c.Plugin.DefaultRuntime))
	if c.Plugin.DefaultRuntime == "" {
		c.Plugin.DefaultRuntime = c.Plugin.AllowedRuntimes[0]
	}
	c.Plugin.ArtifactDir = filepath.Clean(filepath.FromSlash(strings.TrimSpace(c.Plugin.ArtifactDir)))
	if c.Plugin.ArtifactDir == "" || c.Plugin.ArtifactDir == "." {
		c.Plugin.ArtifactDir = filepath.Join("data", "plugins")
	}
	uploadDir := filepath.Clean(filepath.FromSlash(strings.TrimSpace(c.Upload.Dir)))
	if uploadDir == "" || uploadDir == "." {
		uploadDir = "uploads"
	}
	if isPathEqualOrWithin(uploadDir, c.Plugin.ArtifactDir) {
		return fmt.Errorf("plugin.artifact_dir must not be inside upload.dir")
	}
	if c.Plugin.JSFSMaxFiles <= 0 {
		c.Plugin.JSFSMaxFiles = 2048
	}
	if c.Plugin.JSFSMaxTotalBytes <= 0 {
		c.Plugin.JSFSMaxTotalBytes = 128 * 1024 * 1024
	}
	if c.Plugin.JSFSMaxReadBytes <= 0 {
		c.Plugin.JSFSMaxReadBytes = 4 * 1024 * 1024
	}
	if c.Plugin.JSFSMaxReadBytes > c.Plugin.JSFSMaxTotalBytes {
		c.Plugin.JSFSMaxReadBytes = c.Plugin.JSFSMaxTotalBytes
	}
	if c.Plugin.JSStorageMaxKeys <= 0 {
		c.Plugin.JSStorageMaxKeys = 512
	}
	if c.Plugin.JSStorageMaxTotalBytes <= 0 {
		c.Plugin.JSStorageMaxTotalBytes = 4 * 1024 * 1024
	}
	if c.Plugin.JSStorageMaxValueBytes <= 0 {
		c.Plugin.JSStorageMaxValueBytes = 64 * 1024
	}
	if c.Plugin.JSStorageMaxValueBytes > c.Plugin.JSStorageMaxTotalBytes {
		c.Plugin.JSStorageMaxValueBytes = c.Plugin.JSStorageMaxTotalBytes
	}
	if c.Plugin.Frontend.SlotAnimationsEnabled == nil {
		c.Plugin.Frontend.SlotAnimationsEnabled = boolPtr(true)
	}
	c.Plugin.GRPC.Mode = strings.ToLower(strings.TrimSpace(c.Plugin.GRPC.Mode))
	if c.Plugin.GRPC.Mode == "" {
		c.Plugin.GRPC.Mode = "insecure_local"
	}
	switch c.Plugin.GRPC.Mode {
	case "insecure", "insecure_local", "tls":
	default:
		return fmt.Errorf("plugin.grpc.mode must be one of insecure/insecure_local/tls")
	}
	c.Plugin.GRPC.CAFile = filepath.Clean(filepath.FromSlash(strings.TrimSpace(c.Plugin.GRPC.CAFile)))
	if c.Plugin.GRPC.CAFile == "." {
		c.Plugin.GRPC.CAFile = ""
	}
	c.Plugin.GRPC.CertFile = filepath.Clean(filepath.FromSlash(strings.TrimSpace(c.Plugin.GRPC.CertFile)))
	if c.Plugin.GRPC.CertFile == "." {
		c.Plugin.GRPC.CertFile = ""
	}
	c.Plugin.GRPC.KeyFile = filepath.Clean(filepath.FromSlash(strings.TrimSpace(c.Plugin.GRPC.KeyFile)))
	if c.Plugin.GRPC.KeyFile == "." {
		c.Plugin.GRPC.KeyFile = ""
	}
	c.Plugin.GRPC.ServerName = strings.TrimSpace(c.Plugin.GRPC.ServerName)
	if (c.Plugin.GRPC.CertFile == "") != (c.Plugin.GRPC.KeyFile == "") {
		return fmt.Errorf("plugin.grpc.cert_file and plugin.grpc.key_file must be provided together")
	}
	if !containsString(c.Plugin.AllowedRuntimes, c.Plugin.DefaultRuntime) {
		return fmt.Errorf("plugin.default_runtime %q is not in plugin.allowed_runtimes", c.Plugin.DefaultRuntime)
	}

	c.Plugin.Sandbox.Level = strings.ToLower(strings.TrimSpace(c.Plugin.Sandbox.Level))
	if c.Plugin.Sandbox.Level == "" {
		c.Plugin.Sandbox.Level = "balanced"
	}
	switch c.Plugin.Sandbox.Level {
	case "strict", "balanced", "permissive":
	default:
		return fmt.Errorf("plugin.sandbox.level must be one of strict/balanced/permissive")
	}
	if c.Plugin.Sandbox.ExecTimeoutMs <= 0 {
		c.Plugin.Sandbox.ExecTimeoutMs = 30000
	}
	if c.Plugin.Sandbox.MaxMemoryMB <= 0 {
		c.Plugin.Sandbox.MaxMemoryMB = 128
	}
	if c.Plugin.Sandbox.MaxConcurrency <= 0 {
		c.Plugin.Sandbox.MaxConcurrency = 4
	}
	c.Plugin.Sandbox.JSWorkerSocketPath = strings.TrimSpace(c.Plugin.Sandbox.JSWorkerSocketPath)
	if c.Plugin.Sandbox.JSWorkerSocketPath == "" {
		c.Plugin.Sandbox.JSWorkerSocketPath = defaultPluginJSWorkerSocketPath()
	}
	if _, _, err := pluginutil.ResolveJSWorkerSocketEndpoint(c.Plugin.Sandbox.JSWorkerSocketPath); err != nil {
		return fmt.Errorf("plugin.sandbox.js_worker_socket_path is invalid: %w", err)
	}
	c.Plugin.Sandbox.JSWorkerBinary = strings.TrimSpace(c.Plugin.Sandbox.JSWorkerBinary)
	c.Plugin.Sandbox.JSWorkerArgs = normalizeTrimmedStringList(c.Plugin.Sandbox.JSWorkerArgs)

	// 插件执行策略默认值
	if c.Plugin.Execution.HookMaxInFlight <= 0 {
		c.Plugin.Execution.HookMaxInFlight = 64
	}
	if c.Plugin.Execution.HookMaxRetries < 0 {
		c.Plugin.Execution.HookMaxRetries = 0
	}
	if c.Plugin.Execution.HookRetryBackoffMs <= 0 {
		c.Plugin.Execution.HookRetryBackoffMs = 100
	}
	if c.Plugin.Execution.HookBeforeTimeoutMs <= 0 {
		c.Plugin.Execution.HookBeforeTimeoutMs = c.Plugin.Sandbox.ExecTimeoutMs
	}
	if c.Plugin.Execution.HookAfterTimeoutMs <= 0 {
		c.Plugin.Execution.HookAfterTimeoutMs = c.Plugin.Sandbox.ExecTimeoutMs
	}
	if c.Plugin.Execution.FailureThreshold <= 0 {
		c.Plugin.Execution.FailureThreshold = 3
	}
	if c.Plugin.Execution.FailureCooldownMs < 0 {
		c.Plugin.Execution.FailureCooldownMs = 0
	}
	if c.Plugin.Execution.FailureCooldownMs == 0 {
		c.Plugin.Execution.FailureCooldownMs = 30000
	}
	if c.Plugin.Execution.ExecutionLogRetentionDays == 0 {
		c.Plugin.Execution.ExecutionLogRetentionDays = 90
	}

	return nil
}

func NormalizeCORSAllowedOrigins(origins []string) ([]string, error) {
	if len(origins) == 0 {
		return nil, nil
	}

	normalized := make([]string, 0, len(origins))
	seen := make(map[string]struct{}, len(origins))
	for _, rawOrigin := range origins {
		origin := strings.TrimSpace(rawOrigin)
		if origin == "" {
			continue
		}
		if origin == "*" || strings.Contains(origin, "*") {
			return nil, fmt.Errorf("security.cors.allowed_origins must not contain wildcard origins")
		}

		parsed, err := url.Parse(origin)
		if err != nil {
			return nil, fmt.Errorf("security.cors.allowed_origins contains invalid origin %q: %w", origin, err)
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return nil, fmt.Errorf("security.cors.allowed_origins contains invalid scheme for %q", origin)
		}
		if parsed.Host == "" {
			return nil, fmt.Errorf("security.cors.allowed_origins contains origin without host %q", origin)
		}
		if parsed.User != nil {
			return nil, fmt.Errorf("security.cors.allowed_origins must not include credentials for %q", origin)
		}
		if parsed.RawQuery != "" || parsed.Fragment != "" {
			return nil, fmt.Errorf("security.cors.allowed_origins must not include query or fragment for %q", origin)
		}
		if parsed.Path != "" && parsed.Path != "/" {
			return nil, fmt.Errorf("security.cors.allowed_origins must not include path for %q", origin)
		}

		canonicalOrigin := strings.ToLower(parsed.Scheme) + "://" + strings.ToLower(parsed.Host)
		if _, exists := seen[canonicalOrigin]; exists {
			continue
		}
		seen[canonicalOrigin] = struct{}{}
		normalized = append(normalized, canonicalOrigin)
	}

	return normalized, nil
}

func normalizeLowerStringList(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.ToLower(strings.TrimSpace(value))
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func defaultPluginJSWorkerSocketPath() string {
	if runtime.GOOS == "windows" {
		return "tcp://127.0.0.1:17345"
	}
	return "/tmp/auralogic-jsworker.sock"
}

func normalizeTrimmedStringList(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func boolPtr(value bool) *bool {
	return &value
}

func isPathEqualOrWithin(basePath string, targetPath string) bool {
	baseTrimmed := strings.TrimSpace(basePath)
	targetTrimmed := strings.TrimSpace(targetPath)
	if baseTrimmed == "" || targetTrimmed == "" {
		return false
	}

	baseAbs, err := filepath.Abs(filepath.Clean(filepath.FromSlash(baseTrimmed)))
	if err != nil {
		return false
	}
	targetAbs, err := filepath.Abs(filepath.Clean(filepath.FromSlash(targetTrimmed)))
	if err != nil {
		return false
	}

	rel, err := filepath.Rel(baseAbs, targetAbs)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	if rel == ".." {
		return false
	}
	parentPrefix := ".." + string(os.PathSeparator)
	return !strings.HasPrefix(rel, parentPrefix)
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
