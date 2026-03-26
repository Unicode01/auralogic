package admin

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/middleware"
	"auralogic/internal/models"
	"auralogic/internal/pkg/cache"
	"auralogic/internal/pkg/logger"
	"auralogic/internal/pkg/pluginutil"
	"auralogic/internal/pkg/response"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
	"gopkg.in/gomail.v2"
	"gorm.io/gorm"
)

type SettingsHandler struct {
	db            *gorm.DB
	cfg           *config.Config
	smsService    *service.SMSService
	emailService  *service.EmailService
	pluginManager *service.PluginManagerService
}

func NewSettingsHandler(
	db *gorm.DB,
	cfg *config.Config,
	smsService *service.SMSService,
	emailService *service.EmailService,
	pluginManager *service.PluginManagerService,
) *SettingsHandler {
	return &SettingsHandler{
		db:            db,
		cfg:           cfg,
		smsService:    smsService,
		emailService:  emailService,
		pluginManager: pluginManager,
	}
}

type pluginManagedWorkerRuntimeSnapshot struct {
	ArtifactDir            string
	JSFSMaxFiles           int
	JSFSMaxTotalBytes      int64
	JSFSMaxReadBytes       int64
	JSStorageMaxKeys       int
	JSStorageMaxTotalBytes int64
	JSStorageMaxValueBytes int64
	Sandbox                struct {
		ExecTimeoutMs      int
		MaxMemoryMB        int
		MaxConcurrency     int
		JSWorkerSocketPath string
		JSWorkerAutoStart  bool
		JSWorkerBinary     string
		JSWorkerArgs       []string
		JSAllowNetwork     bool
		JSAllowFileSystem  bool
	}
}

func snapshotPluginManagedWorkerRuntime(cfg config.PluginPlatformConfig) pluginManagedWorkerRuntimeSnapshot {
	snapshot := pluginManagedWorkerRuntimeSnapshot{
		ArtifactDir:            strings.TrimSpace(cfg.ArtifactDir),
		JSFSMaxFiles:           cfg.JSFSMaxFiles,
		JSFSMaxTotalBytes:      cfg.JSFSMaxTotalBytes,
		JSFSMaxReadBytes:       cfg.JSFSMaxReadBytes,
		JSStorageMaxKeys:       cfg.JSStorageMaxKeys,
		JSStorageMaxTotalBytes: cfg.JSStorageMaxTotalBytes,
		JSStorageMaxValueBytes: cfg.JSStorageMaxValueBytes,
	}
	snapshot.Sandbox.ExecTimeoutMs = cfg.Sandbox.ExecTimeoutMs
	snapshot.Sandbox.MaxMemoryMB = cfg.Sandbox.MaxMemoryMB
	snapshot.Sandbox.MaxConcurrency = cfg.Sandbox.MaxConcurrency
	snapshot.Sandbox.JSWorkerSocketPath = strings.TrimSpace(cfg.Sandbox.JSWorkerSocketPath)
	snapshot.Sandbox.JSWorkerAutoStart = cfg.Sandbox.JSWorkerAutoStart
	snapshot.Sandbox.JSWorkerBinary = strings.TrimSpace(cfg.Sandbox.JSWorkerBinary)
	snapshot.Sandbox.JSWorkerArgs = append([]string(nil), cfg.Sandbox.JSWorkerArgs...)
	snapshot.Sandbox.JSAllowNetwork = cfg.Sandbox.JSAllowNetwork
	snapshot.Sandbox.JSAllowFileSystem = cfg.Sandbox.JSAllowFileSystem
	return snapshot
}

func shouldAutoRestartManagedJSWorker(before, after config.PluginPlatformConfig) bool {
	return !reflect.DeepEqual(
		snapshotPluginManagedWorkerRuntime(before),
		snapshotPluginManagedWorkerRuntime(after),
	)
}

func shouldReloadGRPCPlugins(before, after config.PluginPlatformConfig) bool {
	return !reflect.DeepEqual(before.GRPC, after.GRPC)
}

func (h *SettingsHandler) reloadEnabledPluginsByRuntime(runtime string) (int, error) {
	if h == nil || h.pluginManager == nil || h.db == nil {
		return 0, nil
	}

	var plugins []models.Plugin
	if err := h.db.Where("enabled = ?", true).Order("id ASC").Find(&plugins).Error; err != nil {
		return 0, err
	}

	targetRuntime := strings.ToLower(strings.TrimSpace(runtime))
	reloaded := 0
	var reloadErrors []string
	for i := range plugins {
		resolvedRuntime, err := h.pluginManager.ResolveRuntime(plugins[i].Runtime)
		if err != nil || resolvedRuntime != targetRuntime {
			continue
		}
		if err := h.pluginManager.ReloadPlugin(plugins[i].ID); err != nil {
			reloadErrors = append(reloadErrors, fmt.Sprintf("%s(%d): %v", plugins[i].Name, plugins[i].ID, err))
			continue
		}
		reloaded++
	}

	if len(reloadErrors) > 0 {
		return reloaded, errors.New(strings.Join(reloadErrors, "; "))
	}
	return reloaded, nil
}

func (h *SettingsHandler) renderAuthBranding() config.AuthBrandingConfig {
	ab := h.cfg.Customization.AuthBranding
	if ab.Mode != "custom" || ab.CustomHTML == "" {
		return ab
	}
	tmpl, err := template.New("ab").Parse(ab.CustomHTML)
	if err != nil {
		return ab
	}
	appURL := h.cfg.App.URL
	if appURL == "" {
		appURL = fmt.Sprintf("http://localhost:%d", h.cfg.App.Port)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, map[string]interface{}{
		"AppName":      h.cfg.App.Name,
		"AppURL":       appURL,
		"LogoURL":      h.cfg.Customization.LogoURL,
		"PrimaryColor": h.cfg.Customization.PrimaryColor,
		"Year":         time.Now().Year(),
	}); err != nil {
		return ab
	}
	ab.CustomHTML = buf.String()
	return ab
}

// GetPublicConfig 获取公开配置（无需登录）
func (h *SettingsHandler) GetPublicConfig(c *gin.Context) {
	defaultTheme := h.cfg.App.DefaultTheme
	if defaultTheme == "" {
		defaultTheme = "system"
	}
	publicConfig := gin.H{
		"currency":                   h.cfg.Order.Currency,
		"max_order_items":            h.cfg.Order.MaxOrderItems,
		"max_item_quantity":          h.cfg.Order.MaxItemQuantity,
		"app_name":                   h.cfg.App.Name,
		"default_theme":              defaultTheme,
		"allow_registration":         h.cfg.Security.Login.AllowRegistration,
		"allow_password_login":       h.cfg.Security.Login.AllowPasswordLogin,
		"allow_guest_product_browse": h.cfg.Security.Login.AllowGuestProductBrowse,
		"allow_email_login":          h.cfg.Security.Login.AllowEmailLogin,
		"allow_password_reset":       h.cfg.Security.Login.AllowPasswordReset,
		"sms_enabled":                h.cfg.SMS.Enabled,
		"allow_phone_login":          h.cfg.Security.Login.AllowPhoneLogin,
		"allow_phone_register":       h.cfg.Security.Login.AllowPhoneRegister,
		"allow_phone_password_reset": h.cfg.Security.Login.AllowPhonePasswordReset,
		"stock_display": gin.H{
			"mode":                 h.cfg.Order.StockDisplay.Mode,
			"low_stock_threshold":  h.cfg.Order.StockDisplay.LowStockThreshold,
			"high_stock_threshold": h.cfg.Order.StockDisplay.HighStockThreshold,
		},
		"customization": gin.H{
			"primary_color": h.cfg.Customization.PrimaryColor,
			"logo_url":      h.cfg.Customization.LogoURL,
			"favicon_url":   h.cfg.Customization.FaviconURL,
			"auth_branding": h.renderAuthBranding(),
		},
		"ticket": gin.H{
			"enabled":            h.cfg.Ticket.Enabled,
			"categories":         h.cfg.Ticket.Categories,
			"attachment":         h.cfg.Ticket.Attachment,
			"max_content_length": h.cfg.Ticket.MaxContentLength,
			"auto_close_hours":   h.cfg.Ticket.AutoCloseHours,
		},
		"serial": gin.H{
			"enabled": h.cfg.Serial.Enabled,
		},
		"auto_cancel_hours":         h.cfg.Order.AutoCancelHours,
		"invoice_enabled":           h.cfg.Order.Invoice.Enabled,
		"show_virtual_stock_remark": h.cfg.Order.ShowVirtualStockRemark,
		"smtp_enabled":              h.cfg.SMTP.Enabled,
		"captcha": gin.H{
			"provider":                 h.cfg.Security.Captcha.Provider,
			"site_key":                 h.cfg.Security.Captcha.SiteKey,
			"enable_for_login":         h.cfg.Security.Captcha.EnableForLogin,
			"enable_for_register":      h.cfg.Security.Captcha.EnableForRegister,
			"enable_for_serial_verify": h.cfg.Security.Captcha.EnableForSerialVerify,
			"enable_for_bind":          h.cfg.Security.Captcha.EnableForBind,
		},
		"plugin": gin.H{
			"enabled": h.cfg.Plugin.Enabled,
			"frontend": gin.H{
				"slot_animations_enabled": h.cfg.Plugin.Frontend.SlotAnimationsEnabledValue(),
			},
		},
	}
	response.Success(c, publicConfig)
}

// GetSettings get系统设置
func (h *SettingsHandler) GetSettings(c *gin.Context) {
	defaultTheme := h.cfg.App.DefaultTheme
	if defaultTheme == "" {
		defaultTheme = "system"
	}
	// 返回可编辑的配置项（敏感Info需脱敏）
	settings := gin.H{
		"app": gin.H{
			"name":          h.cfg.App.Name,
			"url":           h.cfg.App.URL,
			"debug":         h.cfg.App.Debug,
			"default_theme": defaultTheme,
		},
		"smtp": gin.H{
			"enabled":    h.cfg.SMTP.Enabled,
			"host":       h.cfg.SMTP.Host,
			"port":       h.cfg.SMTP.Port,
			"user":       h.cfg.SMTP.User,
			"from_email": h.cfg.SMTP.FromEmail,
			"from_name":  h.cfg.SMTP.FromName,
		},
		"sms": gin.H{
			"enabled":              h.cfg.SMS.Enabled,
			"provider":             h.cfg.SMS.Provider,
			"aliyun_access_key_id": h.cfg.SMS.AliyunAccessKeyID,
			"aliyun_sign_name":     h.cfg.SMS.AliyunSignName,
			"aliyun_template_code": h.cfg.SMS.AliyunTemplateCode,
			"templates": gin.H{
				"login":          h.cfg.SMS.Templates.Login,
				"register":       h.cfg.SMS.Templates.Register,
				"reset_password": h.cfg.SMS.Templates.ResetPassword,
				"bind_phone":     h.cfg.SMS.Templates.BindPhone,
			},
			"dypns_code_length":    h.cfg.SMS.DYPNSCodeLength,
			"twilio_account_sid":   h.cfg.SMS.TwilioAccountSID,
			"twilio_from_number":   h.cfg.SMS.TwilioFromNumber,
			"custom_url":           h.cfg.SMS.CustomURL,
			"custom_method":        h.cfg.SMS.CustomMethod,
			"custom_headers":       h.cfg.SMS.CustomHeaders,
			"custom_body_template": h.cfg.SMS.CustomBodyTemplate,
		},
		"security": gin.H{
			"password_policy": h.cfg.Security.PasswordPolicy,
			"login":           h.cfg.Security.Login,
			"cors":            h.cfg.Security.CORS,
			"captcha":         h.cfg.Security.Captcha,
			"ip_header":       h.cfg.Security.IPHeader,
			"trusted_proxies": h.cfg.Security.TrustedProxies,
		},
		"rate_limit":       h.cfg.RateLimit,
		"email_rate_limit": h.cfg.EmailRateLimit,
		"sms_rate_limit":   h.cfg.SMSRateLimit,
		"order": gin.H{
			"no_prefix":                 h.cfg.Order.NoPrefix,
			"auto_cancel_hours":         h.cfg.Order.AutoCancelHours,
			"currency":                  h.cfg.Order.Currency,
			"max_order_items":           h.cfg.Order.MaxOrderItems,
			"max_item_quantity":         h.cfg.Order.MaxItemQuantity,
			"virtual_delivery_order":    h.cfg.Order.VirtualDeliveryOrder,
			"show_virtual_stock_remark": h.cfg.Order.ShowVirtualStockRemark,
			"high_concurrency_protection": gin.H{
				"enabled":         h.cfg.Order.HighConcurrencyProtection.Enabled,
				"mode":            h.cfg.Order.HighConcurrencyProtection.Mode,
				"max_inflight":    h.cfg.Order.HighConcurrencyProtection.MaxInFlight,
				"wait_timeout_ms": h.cfg.Order.HighConcurrencyProtection.WaitTimeoutMs,
				"redis_lease_ms":  h.cfg.Order.HighConcurrencyProtection.RedisLeaseMs,
			},
			"stock_display": gin.H{
				"mode":                 h.cfg.Order.StockDisplay.Mode,
				"low_stock_threshold":  h.cfg.Order.StockDisplay.LowStockThreshold,
				"high_stock_threshold": h.cfg.Order.StockDisplay.HighStockThreshold,
			},
			"invoice": gin.H{
				"enabled":         h.cfg.Order.Invoice.Enabled,
				"template_type":   h.cfg.Order.Invoice.TemplateType,
				"custom_template": h.cfg.Order.Invoice.CustomTemplate,
				"company_name":    h.cfg.Order.Invoice.CompanyName,
				"company_address": h.cfg.Order.Invoice.CompanyAddress,
				"company_phone":   h.cfg.Order.Invoice.CompanyPhone,
				"company_email":   h.cfg.Order.Invoice.CompanyEmail,
				"company_logo":    h.cfg.Order.Invoice.CompanyLogo,
				"tax_id":          h.cfg.Order.Invoice.TaxID,
				"footer_text":     h.cfg.Order.Invoice.FooterText,
			},
		},
		"magic_link": gin.H{
			"expire_minutes": h.cfg.MagicLink.ExpireMinutes,
			"max_uses":       h.cfg.MagicLink.MaxUses,
		},
		"form": gin.H{
			"expire_hours": h.cfg.Form.ExpireHours,
		},
		"upload": gin.H{
			"dir":           h.cfg.Upload.Dir,
			"max_size":      h.cfg.Upload.MaxSize,
			"allowed_types": h.cfg.Upload.AllowedTypes,
		},
		"oauth": gin.H{
			"google": gin.H{
				"enabled":      h.cfg.OAuth.Google.Enabled,
				"client_id":    h.cfg.OAuth.Google.ClientID,
				"redirect_url": h.cfg.OAuth.Google.RedirectURL,
				// client_secret 不返回
			},
			"github": gin.H{
				"enabled":      h.cfg.OAuth.Github.Enabled,
				"client_id":    h.cfg.OAuth.Github.ClientID,
				"redirect_url": h.cfg.OAuth.Github.RedirectURL,
				// client_secret 不返回
			},
		},
		"log": gin.H{
			"level":     h.cfg.Log.Level,
			"format":    h.cfg.Log.Format,
			"output":    h.cfg.Log.Output,
			"file_path": h.cfg.Log.FilePath,
		},
		"redis": gin.H{
			"host": h.cfg.Redis.Host,
			"port": h.cfg.Redis.Port,
			"db":   h.cfg.Redis.DB,
			// password 不返回
		},
		"ticket": gin.H{
			"enabled":            h.cfg.Ticket.Enabled,
			"categories":         h.cfg.Ticket.Categories,
			"template":           h.cfg.Ticket.Template,
			"max_content_length": h.cfg.Ticket.MaxContentLength,
			"auto_close_hours":   h.cfg.Ticket.AutoCloseHours,
			"attachment":         h.cfg.Ticket.Attachment,
		},
		"serial": gin.H{
			"enabled": h.cfg.Serial.Enabled,
		},
		"customization": gin.H{
			"primary_color": h.cfg.Customization.PrimaryColor,
			"logo_url":      h.cfg.Customization.LogoURL,
			"favicon_url":   h.cfg.Customization.FaviconURL,
			"page_rules":    h.cfg.Customization.PageRules,
			"auth_branding": h.cfg.Customization.AuthBranding,
		},
		"email_notifications": h.cfg.EmailNotifications,
		"plugin": gin.H{
			"enabled":                    h.cfg.Plugin.Enabled,
			"allowed_runtimes":           h.cfg.Plugin.AllowedRuntimes,
			"allowed_types":              h.cfg.Plugin.AllowedTypes,
			"default_runtime":            h.cfg.Plugin.DefaultRuntime,
			"artifact_dir":               h.cfg.Plugin.ArtifactDir,
			"js_fs_max_files":            h.cfg.Plugin.JSFSMaxFiles,
			"js_fs_max_total_bytes":      h.cfg.Plugin.JSFSMaxTotalBytes,
			"js_fs_max_read_bytes":       h.cfg.Plugin.JSFSMaxReadBytes,
			"js_storage_max_keys":        h.cfg.Plugin.JSStorageMaxKeys,
			"js_storage_max_total_bytes": h.cfg.Plugin.JSStorageMaxTotalBytes,
			"js_storage_max_value_bytes": h.cfg.Plugin.JSStorageMaxValueBytes,
			"frontend": gin.H{
				"force_sanitize_html":     h.cfg.Plugin.Frontend.ForceSanitizeHTML,
				"slot_animations_enabled": h.cfg.Plugin.Frontend.SlotAnimationsEnabledValue(),
			},
			"grpc": gin.H{
				"mode":                 h.cfg.Plugin.GRPC.Mode,
				"ca_file":              h.cfg.Plugin.GRPC.CAFile,
				"cert_file":            h.cfg.Plugin.GRPC.CertFile,
				"key_file":             h.cfg.Plugin.GRPC.KeyFile,
				"server_name":          h.cfg.Plugin.GRPC.ServerName,
				"insecure_skip_verify": h.cfg.Plugin.GRPC.InsecureSkipVerify,
			},
			"sandbox": gin.H{
				"level":                 h.cfg.Plugin.Sandbox.Level,
				"exec_timeout_ms":       h.cfg.Plugin.Sandbox.ExecTimeoutMs,
				"max_memory_mb":         h.cfg.Plugin.Sandbox.MaxMemoryMB,
				"max_concurrency":       h.cfg.Plugin.Sandbox.MaxConcurrency,
				"js_worker_socket_path": h.cfg.Plugin.Sandbox.JSWorkerSocketPath,
				"js_worker_auto_start":  h.cfg.Plugin.Sandbox.JSWorkerAutoStart,
				"js_worker_binary":      h.cfg.Plugin.Sandbox.JSWorkerBinary,
				"js_worker_args":        h.cfg.Plugin.Sandbox.JSWorkerArgs,
				"js_allow_network":      h.cfg.Plugin.Sandbox.JSAllowNetwork,
				"js_allow_file_system":  h.cfg.Plugin.Sandbox.JSAllowFileSystem,
			},
			"execution": gin.H{
				"hook_max_inflight":            h.cfg.Plugin.Execution.HookMaxInFlight,
				"hook_max_retries":             h.cfg.Plugin.Execution.HookMaxRetries,
				"hook_retry_backoff_ms":        h.cfg.Plugin.Execution.HookRetryBackoffMs,
				"hook_before_timeout_ms":       h.cfg.Plugin.Execution.HookBeforeTimeoutMs,
				"hook_after_timeout_ms":        h.cfg.Plugin.Execution.HookAfterTimeoutMs,
				"failure_threshold":            h.cfg.Plugin.Execution.FailureThreshold,
				"failure_cooldown_ms":          h.cfg.Plugin.Execution.FailureCooldownMs,
				"execution_log_retention_days": h.cfg.Plugin.Execution.ExecutionLogRetentionDays,
			},
		},
		"analytics": gin.H{
			"enabled": h.cfg.Analytics.Enabled,
		},
	}

	response.Success(c, settings)
}

// UpdateSettingsRequest Update设置请求
type UpdateSettingsRequest struct {
	App struct {
		Name         string `json:"name"`
		URL          string `json:"url"`
		Debug        bool   `json:"debug"`
		DefaultTheme string `json:"default_theme"`
	} `json:"app,omitempty"`

	SMTP struct {
		Enabled   bool   `json:"enabled"`
		Host      string `json:"host"`
		Port      int    `json:"port"`
		User      string `json:"user"`
		Password  string `json:"password,omitempty"` // 可选，不修改则保持原值
		FromEmail string `json:"from_email"`
		FromName  string `json:"from_name"`
	} `json:"smtp,omitempty"`

	SMS struct {
		Submitted             bool              `json:"_submitted"`
		Enabled               bool              `json:"enabled"`
		Provider              string            `json:"provider"`
		AliyunAccessKeyID     string            `json:"aliyun_access_key_id"`
		AliyunAccessSecret    string            `json:"aliyun_access_secret,omitempty"`
		AliyunSignName        string            `json:"aliyun_sign_name"`
		AliyunTemplateCode    string            `json:"aliyun_template_code"`
		TemplateLogin         string            `json:"template_login"`
		TemplateRegister      string            `json:"template_register"`
		TemplateResetPassword string            `json:"template_reset_password"`
		TemplateBindPhone     string            `json:"template_bind_phone"`
		TwilioAccountSID      string            `json:"twilio_account_sid"`
		TwilioAuthToken       string            `json:"twilio_auth_token,omitempty"`
		TwilioFromNumber      string            `json:"twilio_from_number"`
		DYPNSCodeLength       int               `json:"dypns_code_length"`
		CustomURL             string            `json:"custom_url"`
		CustomMethod          string            `json:"custom_method"`
		CustomHeaders         map[string]string `json:"custom_headers"`
		CustomBodyTemplate    string            `json:"custom_body_template"`
	} `json:"sms,omitempty"`

	Security struct {
		PasswordPolicy          config.PasswordPolicyConfig `json:"password_policy,omitempty"`
		Login                   config.LoginConfig          `json:"login,omitempty"`
		LoginSubmitted          bool                        `json:"login_submitted,omitempty"`
		CORS                    config.CORSConfig           `json:"cors,omitempty"`
		Captcha                 *config.CaptchaConfig       `json:"captcha,omitempty"`
		IPHeader                string                      `json:"ip_header,omitempty"`
		IPHeaderSubmitted       bool                        `json:"ip_header_submitted,omitempty"`
		TrustedProxies          []string                    `json:"trusted_proxies,omitempty"`
		TrustedProxiesSubmitted bool                        `json:"trusted_proxies_submitted,omitempty"`
	} `json:"security,omitempty"`

	RateLimit config.RateLimitConfig `json:"rate_limit,omitempty"`

	EmailRateLimit *config.MessageRateLimit `json:"email_rate_limit,omitempty"`
	SMSRateLimit   *config.MessageRateLimit `json:"sms_rate_limit,omitempty"`

	Order struct {
		NoPrefix                       string                                      `json:"no_prefix"`
		AutoCancelHours                int                                         `json:"auto_cancel_hours"`
		MaxPendingPaymentOrdersPerUser int                                         `json:"max_pending_payment_orders_per_user"`
		MaxPaymentPollingTasksPerUser  int                                         `json:"max_payment_polling_tasks_per_user"`
		MaxPaymentPollingTasksGlobal   int                                         `json:"max_payment_polling_tasks_global"`
		Currency                       string                                      `json:"currency"`
		MaxOrderItems                  int                                         `json:"max_order_items"`
		MaxItemQuantity                int                                         `json:"max_item_quantity"`
		VirtualDeliveryOrder           string                                      `json:"virtual_delivery_order"`
		ShowVirtualStockRemark         *bool                                       `json:"show_virtual_stock_remark"`
		StockDisplay                   config.StockDisplayConfig                   `json:"stock_display"`
		Invoice                        config.InvoiceConfig                        `json:"invoice"`
		HighConcurrencyProtection      config.OrderHighConcurrencyProtectionConfig `json:"high_concurrency_protection"`
	} `json:"order,omitempty"`

	MagicLink struct {
		ExpireMinutes int `json:"expire_minutes"`
		MaxUses       int `json:"max_uses"`
	} `json:"magic_link,omitempty"`

	Form struct {
		ExpireHours int `json:"expire_hours"`
	} `json:"form,omitempty"`

	Upload struct {
		Dir          string   `json:"dir"`
		MaxSize      int64    `json:"max_size"`
		AllowedTypes []string `json:"allowed_types"`
	} `json:"upload,omitempty"`

	OAuth struct {
		Google config.OAuthProviderConfig `json:"google,omitempty"`
		Github config.OAuthProviderConfig `json:"github,omitempty"`
	} `json:"oauth,omitempty"`

	Log struct {
		Level    string `json:"level"`
		Format   string `json:"format"`
		Output   string `json:"output"`
		FilePath string `json:"file_path"`
	} `json:"log,omitempty"`

	Ticket struct {
		Enabled          bool                           `json:"enabled"`
		Categories       []string                       `json:"categories"`
		Template         string                         `json:"template"`
		MaxContentLength int                            `json:"max_content_length"`
		AutoCloseHours   int                            `json:"auto_close_hours"`
		Attachment       *config.TicketAttachmentConfig `json:"attachment,omitempty"`
	} `json:"ticket,omitempty"`

	Serial struct {
		Submitted bool `json:"_submitted"`
		Enabled   bool `json:"enabled"`
	} `json:"serial,omitempty"`

	Customization struct {
		Submitted    bool                       `json:"_submitted"`
		PrimaryColor string                     `json:"primary_color"`
		LogoURL      string                     `json:"logo_url"`
		FaviconURL   string                     `json:"favicon_url"`
		PageRules    []config.PageRule          `json:"page_rules"`
		AuthBranding *config.AuthBrandingConfig `json:"auth_branding,omitempty"`
	} `json:"customization,omitempty"`

	EmailNotifications *config.EmailNotificationsConfig `json:"email_notifications,omitempty"`

	Analytics struct {
		Submitted bool `json:"_submitted"`
		Enabled   bool `json:"enabled"`
	} `json:"analytics,omitempty"`

	Plugin struct {
		Submitted              bool     `json:"_submitted"`
		Enabled                bool     `json:"enabled"`
		AllowedRuntimes        []string `json:"allowed_runtimes"`
		AllowedTypes           []string `json:"allowed_types"`
		DefaultRuntime         string   `json:"default_runtime"`
		ArtifactDir            string   `json:"artifact_dir"`
		JSFSMaxFiles           int      `json:"js_fs_max_files"`
		JSFSMaxTotalBytes      int64    `json:"js_fs_max_total_bytes"`
		JSFSMaxReadBytes       int64    `json:"js_fs_max_read_bytes"`
		JSStorageMaxKeys       int      `json:"js_storage_max_keys"`
		JSStorageMaxTotalBytes int64    `json:"js_storage_max_total_bytes"`
		JSStorageMaxValueBytes int64    `json:"js_storage_max_value_bytes"`
		Frontend               struct {
			ForceSanitizeHTML     *bool `json:"force_sanitize_html"`
			SlotAnimationsEnabled *bool `json:"slot_animations_enabled"`
		} `json:"frontend"`
		GRPC struct {
			Mode               string `json:"mode"`
			CAFile             string `json:"ca_file"`
			CertFile           string `json:"cert_file"`
			KeyFile            string `json:"key_file"`
			ServerName         string `json:"server_name"`
			InsecureSkipVerify bool   `json:"insecure_skip_verify"`
		} `json:"grpc"`
		Sandbox struct {
			Level              string   `json:"level"`
			ExecTimeoutMs      int      `json:"exec_timeout_ms"`
			MaxMemoryMB        int      `json:"max_memory_mb"`
			MaxConcurrency     int      `json:"max_concurrency"`
			JSWorkerSocketPath string   `json:"js_worker_socket_path"`
			JSWorkerAutoStart  bool     `json:"js_worker_auto_start"`
			JSWorkerBinary     string   `json:"js_worker_binary"`
			JSWorkerArgs       []string `json:"js_worker_args"`
			JSAllowNetwork     bool     `json:"js_allow_network"`
			JSAllowFileSystem  bool     `json:"js_allow_file_system"`
		} `json:"sandbox"`
		Execution struct {
			HookMaxInFlight           int `json:"hook_max_inflight"`
			HookMaxRetries            int `json:"hook_max_retries"`
			HookRetryBackoffMs        int `json:"hook_retry_backoff_ms"`
			HookBeforeTimeoutMs       int `json:"hook_before_timeout_ms"`
			HookAfterTimeoutMs        int `json:"hook_after_timeout_ms"`
			FailureThreshold          int `json:"failure_threshold"`
			FailureCooldownMs         int `json:"failure_cooldown_ms"`
			ExecutionLogRetentionDays int `json:"execution_log_retention_days"`
		} `json:"execution"`
	} `json:"plugin,omitempty"`

	Maintenance struct {
		Submitted               bool `json:"_submitted"`
		ClearPaymentCardCache   bool `json:"clear_payment_card_cache"`
		ClearJSProgramCache     bool `json:"clear_js_program_cache"`
		ClearPermissionCache    bool `json:"clear_permission_cache"`
		ClearRuntimeRedisCache  bool `json:"clear_runtime_redis_cache"`
		DeleteUnreferencedFiles bool `json:"delete_unreferenced_files"`
		RestartJSWorker         bool `json:"restart_js_worker"`
	} `json:"maintenance,omitempty"`
}

// UpdateSettings Update系统设置
func (h *SettingsHandler) UpdateSettings(c *gin.Context) {
	var req UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}
	adminID := middleware.MustGetUserID(c)
	if h.pluginManager != nil {
		originalReq := req
		hookPayload, payloadErr := adminHookStructToPayload(req)
		if payloadErr != nil {
			log.Printf("settings.update.before payload build failed: admin=%d err=%v", adminID, payloadErr)
		} else {
			hookPayload["admin_id"] = adminID
			hookPayload["source"] = "admin_api"
			hookPayload["config_path"] = config.GetConfigPath()
			hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "settings.update.before",
				Payload: hookPayload,
			}, buildAdminHookExecutionContext(c, &adminID, map[string]string{
				"hook_resource": "settings",
				"hook_source":   "admin_api",
			}))
			if hookErr != nil {
				log.Printf("settings.update.before hook execution failed: admin=%d err=%v", adminID, hookErr)
			} else if hookResult != nil {
				if hookResult.Blocked {
					reason := strings.TrimSpace(hookResult.BlockReason)
					if reason == "" {
						reason = "Settings update rejected by plugin"
					}
					response.BadRequest(c, reason)
					return
				}
				if hookResult.Payload != nil {
					if mergeErr := mergeAdminHookStructPatch(&req, hookResult.Payload); mergeErr != nil {
						log.Printf("settings.update.before payload apply failed, fallback to original request: admin=%d err=%v", adminID, mergeErr)
						req = originalReq
					}
				}
			}
		}
	}
	var paymentCardCacheClearedRows int64
	var jsProgramCacheCleared int
	var permissionCacheCleared int64
	var runtimeRedisCacheCleared int64
	var unreferencedFileCleanup unreferencedFileCleanupStats
	var jsWorkerRestarted bool
	var grpcPluginsReloaded int
	previousPluginEnabled := false
	var previousPluginConfig config.PluginPlatformConfig
	if h.cfg != nil {
		previousPluginEnabled = h.cfg.Plugin.Enabled
		previousPluginConfig = h.cfg.Plugin
	}
	pluginRuntimeAction := ""

	// 读取current配置文件
	configPath := config.GetConfigPath()
	currentConfig, err := readConfigFile(configPath)
	if err != nil {
		response.InternalError(c, "Failed to read config file")
		return
	}

	// Update配置
	if req.App.Name != "" {
		currentConfig["app"].(map[string]interface{})["name"] = req.App.Name
	}
	if req.App.URL != "" {
		currentConfig["app"].(map[string]interface{})["url"] = req.App.URL
	}
	currentConfig["app"].(map[string]interface{})["debug"] = req.App.Debug
	if req.App.DefaultTheme != "" {
		currentConfig["app"].(map[string]interface{})["default_theme"] = req.App.DefaultTheme
	}

	// UpdateSMTP配置
	if req.SMTP.Host != "" {
		smtpConfig := currentConfig["smtp"].(map[string]interface{})
		smtpConfig["enabled"] = req.SMTP.Enabled
		smtpConfig["host"] = req.SMTP.Host
		smtpConfig["port"] = req.SMTP.Port
		smtpConfig["user"] = req.SMTP.User
		smtpConfig["from_email"] = req.SMTP.FromEmail
		smtpConfig["from_name"] = req.SMTP.FromName
		// 只有提供了Password才Update
		if req.SMTP.Password != "" {
			smtpConfig["password"] = req.SMTP.Password
		}
	}

	// Update SMS配置
	if req.SMS.Submitted {
		smsConfig, ok := currentConfig["sms"].(map[string]interface{})
		if !ok {
			smsConfig = map[string]interface{}{}
			currentConfig["sms"] = smsConfig
		}
		smsConfig["enabled"] = req.SMS.Enabled
		smsConfig["provider"] = req.SMS.Provider
		smsConfig["aliyun_access_key_id"] = req.SMS.AliyunAccessKeyID
		smsConfig["aliyun_sign_name"] = req.SMS.AliyunSignName
		smsConfig["aliyun_template_code"] = req.SMS.AliyunTemplateCode
		smsConfig["twilio_account_sid"] = req.SMS.TwilioAccountSID
		smsConfig["twilio_from_number"] = req.SMS.TwilioFromNumber
		smsConfig["custom_url"] = req.SMS.CustomURL
		smsConfig["custom_method"] = req.SMS.CustomMethod
		smsConfig["custom_headers"] = req.SMS.CustomHeaders
		smsConfig["custom_body_template"] = req.SMS.CustomBodyTemplate
		smsConfig["templates"] = map[string]interface{}{
			"login":          req.SMS.TemplateLogin,
			"register":       req.SMS.TemplateRegister,
			"reset_password": req.SMS.TemplateResetPassword,
			"bind_phone":     req.SMS.TemplateBindPhone,
		}
		smsConfig["dypns_code_length"] = req.SMS.DYPNSCodeLength
		if req.SMS.AliyunAccessSecret != "" {
			smsConfig["aliyun_access_secret"] = req.SMS.AliyunAccessSecret
		}
		if req.SMS.TwilioAuthToken != "" {
			smsConfig["twilio_auth_token"] = req.SMS.TwilioAuthToken
		}
	}

	// Update安全配置
	if req.Security.PasswordPolicy.MinLength > 0 {
		securityConfig := currentConfig["security"].(map[string]interface{})
		securityConfig["password_policy"] = map[string]interface{}{
			"min_length":        req.Security.PasswordPolicy.MinLength,
			"require_uppercase": req.Security.PasswordPolicy.RequireUppercase,
			"require_lowercase": req.Security.PasswordPolicy.RequireLowercase,
			"require_number":    req.Security.PasswordPolicy.RequireNumber,
			"require_special":   req.Security.PasswordPolicy.RequireSpecial,
		}
	}

	// Update登录配置
	if req.Security.LoginSubmitted {
		// 验证：邮件相关选项需要SMTP已启用
		smtpEnabled := h.cfg.SMTP.Enabled
		if req.SMTP.Host != "" {
			smtpEnabled = req.SMTP.Enabled
		}
		if !smtpEnabled {
			if req.Security.Login.AllowEmailLogin {
				response.BadRequest(c, "Allow email login requires SMTP to be enabled")
				return
			}
			if req.Security.Login.AllowPasswordReset {
				response.BadRequest(c, "Allow password reset requires SMTP to be enabled")
				return
			}
			if req.Security.Login.RequireEmailVerification {
				response.BadRequest(c, "Require email verification requires SMTP to be enabled")
				return
			}
		}

		// 验证：手机相关选项需要SMS已启用
		smsEnabled := h.cfg.SMS.Enabled
		if req.SMS.Submitted {
			smsEnabled = req.SMS.Enabled
		}
		if !smsEnabled {
			if req.Security.Login.AllowPhoneLogin {
				response.BadRequest(c, "Allow phone login requires SMS to be enabled")
				return
			}
			if req.Security.Login.AllowPhoneRegister {
				response.BadRequest(c, "Allow phone register requires SMS to be enabled")
				return
			}
			if req.Security.Login.AllowPhonePasswordReset {
				response.BadRequest(c, "Allow phone password reset requires SMS to be enabled")
				return
			}
		}

		securityConfig := currentConfig["security"].(map[string]interface{})
		securityConfig["login"] = map[string]interface{}{
			"allow_password_login":       req.Security.Login.AllowPasswordLogin,
			"allow_registration":         req.Security.Login.AllowRegistration,
			"allow_guest_product_browse": req.Security.Login.AllowGuestProductBrowse,
			"require_email_verification": req.Security.Login.RequireEmailVerification,
			"allow_email_login":          req.Security.Login.AllowEmailLogin,
			"allow_password_reset":       req.Security.Login.AllowPasswordReset,
			"allow_phone_login":          req.Security.Login.AllowPhoneLogin,
			"allow_phone_register":       req.Security.Login.AllowPhoneRegister,
			"allow_phone_password_reset": req.Security.Login.AllowPhonePasswordReset,
		}
	}

	// Update限流配置
	if req.RateLimit.API > 0 || req.RateLimit.OrderCreate > 0 || req.RateLimit.PaymentInfo > 0 || req.RateLimit.PaymentSelect > 0 {
		currentConfig["rate_limit"] = map[string]interface{}{
			"enabled":        req.RateLimit.Enabled,
			"api":            req.RateLimit.API,
			"user_login":     req.RateLimit.UserLogin,
			"user_request":   req.RateLimit.UserRequest,
			"admin_request":  req.RateLimit.AdminRequest,
			"order_create":   req.RateLimit.OrderCreate,
			"payment_info":   req.RateLimit.PaymentInfo,
			"payment_select": req.RateLimit.PaymentSelect,
		}
	}

	// Update邮件发送限流
	if req.EmailRateLimit != nil {
		currentConfig["email_rate_limit"] = map[string]interface{}{
			"hourly":        req.EmailRateLimit.Hourly,
			"daily":         req.EmailRateLimit.Daily,
			"exceed_action": req.EmailRateLimit.ExceedAction,
		}
	}

	// Update短信发送限流
	if req.SMSRateLimit != nil {
		currentConfig["sms_rate_limit"] = map[string]interface{}{
			"hourly":        req.SMSRateLimit.Hourly,
			"daily":         req.SMSRateLimit.Daily,
			"exceed_action": req.SMSRateLimit.ExceedAction,
		}
	}

	// UpdateOrder配置
	if req.Order.NoPrefix != "" {
		showVirtualStockRemark := false
		if req.Order.ShowVirtualStockRemark != nil {
			showVirtualStockRemark = *req.Order.ShowVirtualStockRemark
		}
		currentConfig["order"] = map[string]interface{}{
			"no_prefix":                           req.Order.NoPrefix,
			"auto_cancel_hours":                   req.Order.AutoCancelHours,
			"max_pending_payment_orders_per_user": req.Order.MaxPendingPaymentOrdersPerUser,
			"max_payment_polling_tasks_per_user":  req.Order.MaxPaymentPollingTasksPerUser,
			"max_payment_polling_tasks_global":    req.Order.MaxPaymentPollingTasksGlobal,
			"currency":                            req.Order.Currency,
			"max_order_items":                     req.Order.MaxOrderItems,
			"max_item_quantity":                   req.Order.MaxItemQuantity,
			"virtual_delivery_order":              req.Order.VirtualDeliveryOrder,
			"show_virtual_stock_remark":           showVirtualStockRemark,
			"high_concurrency_protection": map[string]interface{}{
				"enabled":         req.Order.HighConcurrencyProtection.Enabled,
				"mode":            req.Order.HighConcurrencyProtection.Mode,
				"max_inflight":    req.Order.HighConcurrencyProtection.MaxInFlight,
				"wait_timeout_ms": req.Order.HighConcurrencyProtection.WaitTimeoutMs,
				"redis_lease_ms":  req.Order.HighConcurrencyProtection.RedisLeaseMs,
			},
			"stock_display": map[string]interface{}{
				"mode":                 req.Order.StockDisplay.Mode,
				"low_stock_threshold":  req.Order.StockDisplay.LowStockThreshold,
				"high_stock_threshold": req.Order.StockDisplay.HighStockThreshold,
			},
			"invoice": map[string]interface{}{
				"enabled":         req.Order.Invoice.Enabled,
				"template_type":   req.Order.Invoice.TemplateType,
				"custom_template": req.Order.Invoice.CustomTemplate,
				"company_name":    req.Order.Invoice.CompanyName,
				"company_address": req.Order.Invoice.CompanyAddress,
				"company_phone":   req.Order.Invoice.CompanyPhone,
				"company_email":   req.Order.Invoice.CompanyEmail,
				"company_logo":    req.Order.Invoice.CompanyLogo,
				"tax_id":          req.Order.Invoice.TaxID,
				"footer_text":     req.Order.Invoice.FooterText,
			},
		}
	}

	// Update魔法链接配置
	if req.MagicLink.ExpireMinutes > 0 {
		currentConfig["magic_link"] = map[string]interface{}{
			"expire_minutes": req.MagicLink.ExpireMinutes,
			"max_uses":       req.MagicLink.MaxUses,
		}
	}

	// Update表单配置
	if req.Form.ExpireHours > 0 {
		currentConfig["form"] = map[string]interface{}{
			"expire_hours": req.Form.ExpireHours,
		}
	}

	// Update上传配置
	if req.Upload.Dir != "" {
		currentConfig["upload"] = map[string]interface{}{
			"dir":           req.Upload.Dir,
			"max_size":      req.Upload.MaxSize,
			"allowed_types": req.Upload.AllowedTypes,
		}
	}

	// UpdateOAuth配置
	if req.OAuth.Google.ClientID != "" || req.OAuth.Github.ClientID != "" {
		oauthConfig := currentConfig["oauth"].(map[string]interface{})

		if req.OAuth.Google.ClientID != "" {
			googleConfig := oauthConfig["google"].(map[string]interface{})
			googleConfig["enabled"] = req.OAuth.Google.Enabled
			googleConfig["client_id"] = req.OAuth.Google.ClientID
			googleConfig["redirect_url"] = req.OAuth.Google.RedirectURL
			if req.OAuth.Google.ClientSecret != "" {
				googleConfig["client_secret"] = req.OAuth.Google.ClientSecret
			}
		}

		if req.OAuth.Github.ClientID != "" {
			githubConfig := oauthConfig["github"].(map[string]interface{})
			githubConfig["enabled"] = req.OAuth.Github.Enabled
			githubConfig["client_id"] = req.OAuth.Github.ClientID
			githubConfig["redirect_url"] = req.OAuth.Github.RedirectURL
			if req.OAuth.Github.ClientSecret != "" {
				githubConfig["client_secret"] = req.OAuth.Github.ClientSecret
			}
		}
	}

	// Update日志配置
	if req.Log.Level != "" {
		currentConfig["log"] = map[string]interface{}{
			"level":     req.Log.Level,
			"format":    req.Log.Format,
			"output":    req.Log.Output,
			"file_path": req.Log.FilePath,
		}
	}

	// UpdateCORS配置
	if len(req.Security.CORS.AllowedOrigins) > 0 {
		securityConfig := currentConfig["security"].(map[string]interface{})
		corsConfig := securityConfig["cors"].(map[string]interface{})
		corsConfig["allowed_origins"] = req.Security.CORS.AllowedOrigins
		corsConfig["max_age"] = req.Security.CORS.MaxAge
	}

	// Update验证码配置
	if req.Security.Captcha != nil {
		securityConfig := currentConfig["security"].(map[string]interface{})
		securityConfig["captcha"] = map[string]interface{}{
			"provider":                 req.Security.Captcha.Provider,
			"site_key":                 req.Security.Captcha.SiteKey,
			"secret_key":               req.Security.Captcha.SecretKey,
			"enable_for_login":         req.Security.Captcha.EnableForLogin,
			"enable_for_register":      req.Security.Captcha.EnableForRegister,
			"enable_for_serial_verify": req.Security.Captcha.EnableForSerialVerify,
			"enable_for_bind":          req.Security.Captcha.EnableForBind,
		}
	}

	// Update IP Header config (allow clearing)
	if req.Security.IPHeaderSubmitted {
		securityConfig := currentConfig["security"].(map[string]interface{})
		securityConfig["ip_header"] = req.Security.IPHeader
	}

	// Update Trusted Proxies (allow clearing)
	if req.Security.TrustedProxiesSubmitted {
		securityConfig := currentConfig["security"].(map[string]interface{})
		if req.Security.TrustedProxies == nil {
			securityConfig["trusted_proxies"] = []string{}
		} else {
			securityConfig["trusted_proxies"] = req.Security.TrustedProxies
		}
	}

	// Update工单配置
	if req.Ticket.Categories != nil || req.Ticket.Template != "" || req.Ticket.Attachment != nil {
		ticketConfig, ok := currentConfig["ticket"].(map[string]interface{})
		if !ok {
			ticketConfig = make(map[string]interface{})
			currentConfig["ticket"] = ticketConfig
		}
		// 只有提交了 categories 才更新 enabled（即工单基本设置表单）
		if req.Ticket.Categories != nil {
			ticketConfig["enabled"] = req.Ticket.Enabled
			ticketConfig["categories"] = req.Ticket.Categories
			ticketConfig["max_content_length"] = req.Ticket.MaxContentLength
			ticketConfig["auto_close_hours"] = req.Ticket.AutoCloseHours
		}
		if req.Ticket.Template != "" {
			ticketConfig["template"] = req.Ticket.Template
		}
		if req.Ticket.Attachment != nil {
			ticketConfig["attachment"] = map[string]interface{}{
				"enable_image":        req.Ticket.Attachment.EnableImage,
				"enable_voice":        req.Ticket.Attachment.EnableVoice,
				"max_image_size":      req.Ticket.Attachment.MaxImageSize,
				"max_voice_size":      req.Ticket.Attachment.MaxVoiceSize,
				"max_voice_duration":  req.Ticket.Attachment.MaxVoiceDuration,
				"allowed_image_types": req.Ticket.Attachment.AllowedImageTypes,
				"retention_days":      req.Ticket.Attachment.RetentionDays,
			}
		}
	}

	// Update序列号查询配置
	if req.Serial.Submitted {
		serialConfig, ok := currentConfig["serial"].(map[string]interface{})
		if !ok {
			serialConfig = make(map[string]interface{})
			currentConfig["serial"] = serialConfig
		}
		serialConfig["enabled"] = req.Serial.Enabled
	}

	// Update个性化配置
	if req.Customization.Submitted {
		custMap := map[string]interface{}{
			"primary_color": req.Customization.PrimaryColor,
			"logo_url":      req.Customization.LogoURL,
			"favicon_url":   req.Customization.FaviconURL,
			"page_rules":    req.Customization.PageRules,
		}
		if req.Customization.AuthBranding != nil {
			custMap["auth_branding"] = req.Customization.AuthBranding
		} else if existing, ok := currentConfig["customization"].(map[string]interface{}); ok {
			if ab, exists := existing["auth_branding"]; exists {
				custMap["auth_branding"] = ab
			}
		}
		currentConfig["customization"] = custMap
	}

	// Update邮件通知配置
	if req.EmailNotifications != nil {
		currentConfig["email_notifications"] = map[string]interface{}{
			"user_register":      req.EmailNotifications.UserRegister,
			"order_created":      req.EmailNotifications.OrderCreated,
			"order_paid":         req.EmailNotifications.OrderPaid,
			"order_shipped":      req.EmailNotifications.OrderShipped,
			"order_completed":    req.EmailNotifications.OrderCompleted,
			"order_cancelled":    req.EmailNotifications.OrderCancelled,
			"order_resubmit":     req.EmailNotifications.OrderResubmit,
			"ticket_created":     req.EmailNotifications.TicketCreated,
			"ticket_admin_reply": req.EmailNotifications.TicketAdminReply,
			"ticket_user_reply":  req.EmailNotifications.TicketUserReply,
			"ticket_resolved":    req.EmailNotifications.TicketResolved,
		}
	}

	// Update数据分析配置
	if req.Analytics.Submitted {
		analyticsConfig, ok := currentConfig["analytics"].(map[string]interface{})
		if !ok {
			analyticsConfig = make(map[string]interface{})
			currentConfig["analytics"] = analyticsConfig
		}
		analyticsConfig["enabled"] = req.Analytics.Enabled
	}

	// Update插件平台配置
	if req.Plugin.Submitted {
		pluginConfig, ok := currentConfig["plugin"].(map[string]interface{})
		if !ok {
			pluginConfig = make(map[string]interface{})
			currentConfig["plugin"] = pluginConfig
		}

		allowedRuntimes := normalizeLowerStringList(req.Plugin.AllowedRuntimes)
		if len(allowedRuntimes) == 0 {
			allowedRuntimes = []string{"grpc"}
		}
		allowedTypes := normalizeLowerStringList(req.Plugin.AllowedTypes)

		defaultRuntime := strings.ToLower(strings.TrimSpace(req.Plugin.DefaultRuntime))
		if defaultRuntime == "" {
			defaultRuntime = allowedRuntimes[0]
		}
		artifactDir := strings.TrimSpace(req.Plugin.ArtifactDir)
		if artifactDir == "" && h.cfg != nil {
			artifactDir = strings.TrimSpace(h.cfg.Plugin.ArtifactDir)
		}
		if artifactDir == "" {
			artifactDir = filepath.Join("data", "plugins")
		}
		uploadDir := "uploads"
		if h.cfg != nil {
			if configuredUploadDir := strings.TrimSpace(h.cfg.Upload.Dir); configuredUploadDir != "" {
				uploadDir = configuredUploadDir
			}
		}
		if isPathEqualOrWithin(uploadDir, artifactDir) {
			response.BadRequest(c, "plugin artifact_dir must not be inside upload.dir")
			return
		}
		fsMaxFiles := req.Plugin.JSFSMaxFiles
		if fsMaxFiles <= 0 && h.cfg != nil {
			fsMaxFiles = h.cfg.Plugin.JSFSMaxFiles
		}
		if fsMaxFiles <= 0 {
			fsMaxFiles = 2048
		}
		fsMaxTotalBytes := req.Plugin.JSFSMaxTotalBytes
		if fsMaxTotalBytes <= 0 && h.cfg != nil {
			fsMaxTotalBytes = h.cfg.Plugin.JSFSMaxTotalBytes
		}
		if fsMaxTotalBytes <= 0 {
			fsMaxTotalBytes = 128 * 1024 * 1024
		}
		fsMaxReadBytes := req.Plugin.JSFSMaxReadBytes
		if fsMaxReadBytes <= 0 && h.cfg != nil {
			fsMaxReadBytes = h.cfg.Plugin.JSFSMaxReadBytes
		}
		if fsMaxReadBytes <= 0 {
			fsMaxReadBytes = 4 * 1024 * 1024
		}
		if fsMaxReadBytes > fsMaxTotalBytes {
			fsMaxReadBytes = fsMaxTotalBytes
		}
		storageMaxKeys := req.Plugin.JSStorageMaxKeys
		if storageMaxKeys <= 0 && h.cfg != nil {
			storageMaxKeys = h.cfg.Plugin.JSStorageMaxKeys
		}
		if storageMaxKeys <= 0 {
			storageMaxKeys = 512
		}
		storageMaxTotalBytes := req.Plugin.JSStorageMaxTotalBytes
		if storageMaxTotalBytes <= 0 && h.cfg != nil {
			storageMaxTotalBytes = h.cfg.Plugin.JSStorageMaxTotalBytes
		}
		if storageMaxTotalBytes <= 0 {
			storageMaxTotalBytes = 4 * 1024 * 1024
		}
		storageMaxValueBytes := req.Plugin.JSStorageMaxValueBytes
		if storageMaxValueBytes <= 0 && h.cfg != nil {
			storageMaxValueBytes = h.cfg.Plugin.JSStorageMaxValueBytes
		}
		if storageMaxValueBytes <= 0 {
			storageMaxValueBytes = 64 * 1024
		}
		if storageMaxValueBytes > storageMaxTotalBytes {
			storageMaxValueBytes = storageMaxTotalBytes
		}
		frontendForceSanitize := false
		if h.cfg != nil {
			frontendForceSanitize = h.cfg.Plugin.Frontend.ForceSanitizeHTML
		}
		if req.Plugin.Frontend.ForceSanitizeHTML != nil {
			frontendForceSanitize = *req.Plugin.Frontend.ForceSanitizeHTML
		}
		frontendSlotAnimationsEnabled := true
		if h.cfg != nil {
			frontendSlotAnimationsEnabled = h.cfg.Plugin.Frontend.SlotAnimationsEnabledValue()
		}
		if req.Plugin.Frontend.SlotAnimationsEnabled != nil {
			frontendSlotAnimationsEnabled = *req.Plugin.Frontend.SlotAnimationsEnabled
		}
		grpcPayloadProvided := strings.TrimSpace(req.Plugin.GRPC.Mode) != "" ||
			strings.TrimSpace(req.Plugin.GRPC.CAFile) != "" ||
			strings.TrimSpace(req.Plugin.GRPC.CertFile) != "" ||
			strings.TrimSpace(req.Plugin.GRPC.KeyFile) != "" ||
			strings.TrimSpace(req.Plugin.GRPC.ServerName) != "" ||
			req.Plugin.GRPC.InsecureSkipVerify
		grpcMode := strings.ToLower(strings.TrimSpace(req.Plugin.GRPC.Mode))
		if grpcMode == "" && h.cfg != nil {
			grpcMode = strings.ToLower(strings.TrimSpace(h.cfg.Plugin.GRPC.Mode))
		}
		if grpcMode == "" {
			grpcMode = "insecure_local"
		}
		switch grpcMode {
		case "insecure", "insecure_local", "tls":
		default:
			response.BadRequest(c, "plugin.grpc.mode must be one of insecure/insecure_local/tls")
			return
		}
		grpcCAFile := strings.TrimSpace(req.Plugin.GRPC.CAFile)
		if grpcCAFile == "" && h.cfg != nil {
			grpcCAFile = strings.TrimSpace(h.cfg.Plugin.GRPC.CAFile)
		}
		grpcCertFile := strings.TrimSpace(req.Plugin.GRPC.CertFile)
		if grpcCertFile == "" && h.cfg != nil {
			grpcCertFile = strings.TrimSpace(h.cfg.Plugin.GRPC.CertFile)
		}
		grpcKeyFile := strings.TrimSpace(req.Plugin.GRPC.KeyFile)
		if grpcKeyFile == "" && h.cfg != nil {
			grpcKeyFile = strings.TrimSpace(h.cfg.Plugin.GRPC.KeyFile)
		}
		grpcServerName := strings.TrimSpace(req.Plugin.GRPC.ServerName)
		if grpcServerName == "" && h.cfg != nil {
			grpcServerName = strings.TrimSpace(h.cfg.Plugin.GRPC.ServerName)
		}
		grpcInsecureSkipVerify := req.Plugin.GRPC.InsecureSkipVerify
		if !grpcPayloadProvided && h.cfg != nil {
			grpcInsecureSkipVerify = h.cfg.Plugin.GRPC.InsecureSkipVerify
		}
		if (grpcCertFile == "") != (grpcKeyFile == "") {
			response.BadRequest(c, "plugin.grpc.cert_file and plugin.grpc.key_file must be provided together")
			return
		}
		jsWorkerSocketPath := strings.TrimSpace(req.Plugin.Sandbox.JSWorkerSocketPath)
		if jsWorkerSocketPath != "" {
			if _, _, err := pluginutil.ResolveJSWorkerSocketEndpoint(jsWorkerSocketPath); err != nil {
				response.BadRequest(c, "plugin.sandbox.js_worker_socket_path must be a unix socket path or loopback tcp://host:port")
				return
			}
		}

		pluginConfig["enabled"] = req.Plugin.Enabled
		pluginConfig["allowed_runtimes"] = allowedRuntimes
		pluginConfig["allowed_types"] = allowedTypes
		pluginConfig["default_runtime"] = defaultRuntime
		pluginConfig["artifact_dir"] = artifactDir
		pluginConfig["js_fs_max_files"] = fsMaxFiles
		pluginConfig["js_fs_max_total_bytes"] = fsMaxTotalBytes
		pluginConfig["js_fs_max_read_bytes"] = fsMaxReadBytes
		pluginConfig["js_storage_max_keys"] = storageMaxKeys
		pluginConfig["js_storage_max_total_bytes"] = storageMaxTotalBytes
		pluginConfig["js_storage_max_value_bytes"] = storageMaxValueBytes
		pluginConfig["frontend"] = map[string]interface{}{
			"force_sanitize_html":     frontendForceSanitize,
			"slot_animations_enabled": frontendSlotAnimationsEnabled,
		}
		pluginConfig["grpc"] = map[string]interface{}{
			"mode":                 grpcMode,
			"ca_file":              grpcCAFile,
			"cert_file":            grpcCertFile,
			"key_file":             grpcKeyFile,
			"server_name":          grpcServerName,
			"insecure_skip_verify": grpcInsecureSkipVerify,
		}
		pluginConfig["sandbox"] = map[string]interface{}{
			"level":                 strings.ToLower(strings.TrimSpace(req.Plugin.Sandbox.Level)),
			"exec_timeout_ms":       req.Plugin.Sandbox.ExecTimeoutMs,
			"max_memory_mb":         req.Plugin.Sandbox.MaxMemoryMB,
			"max_concurrency":       req.Plugin.Sandbox.MaxConcurrency,
			"js_worker_socket_path": jsWorkerSocketPath,
			"js_worker_auto_start":  req.Plugin.Sandbox.JSWorkerAutoStart,
			"js_worker_binary":      strings.TrimSpace(req.Plugin.Sandbox.JSWorkerBinary),
			"js_worker_args":        normalizeTrimmedStringList(req.Plugin.Sandbox.JSWorkerArgs),
			"js_allow_network":      req.Plugin.Sandbox.JSAllowNetwork,
			"js_allow_file_system":  req.Plugin.Sandbox.JSAllowFileSystem,
		}
		pluginConfig["execution"] = map[string]interface{}{
			"hook_max_inflight":            req.Plugin.Execution.HookMaxInFlight,
			"hook_max_retries":             req.Plugin.Execution.HookMaxRetries,
			"hook_retry_backoff_ms":        req.Plugin.Execution.HookRetryBackoffMs,
			"hook_before_timeout_ms":       req.Plugin.Execution.HookBeforeTimeoutMs,
			"hook_after_timeout_ms":        req.Plugin.Execution.HookAfterTimeoutMs,
			"failure_threshold":            req.Plugin.Execution.FailureThreshold,
			"failure_cooldown_ms":          req.Plugin.Execution.FailureCooldownMs,
			"execution_log_retention_days": req.Plugin.Execution.ExecutionLogRetentionDays,
		}
	}

	// Maintenance actions: run one-time cache cleanup operations.
	if req.Maintenance.Submitted {
		if req.Maintenance.ClearPaymentCardCache {
			result := h.db.Model(&models.OrderPaymentMethod{}).
				Where("payment_card_cache <> '' OR cache_expires_at IS NOT NULL").
				Updates(map[string]interface{}{
					"payment_card_cache": "",
					"cache_expires_at":   nil,
				})
			if result.Error != nil {
				response.InternalError(c, "Failed to clear payment card cache")
				return
			}
			paymentCardCacheClearedRows = result.RowsAffected
			logger.LogOperation(h.db, c, "maintenance", "payment_card_cache", nil, map[string]interface{}{
				"cleared_rows": paymentCardCacheClearedRows,
			})
		}

		if req.Maintenance.ClearJSProgramCache {
			jsProgramCacheCleared = service.ClearJSProgramCache()
			logger.LogOperation(h.db, c, "maintenance", "js_program_cache", nil, map[string]interface{}{
				"cleared_entries": jsProgramCacheCleared,
			})
		}

		if req.Maintenance.ClearPermissionCache {
			cleared, err := middleware.InvalidateAllPermissionCache()
			if err != nil {
				response.InternalError(c, "Failed to clear permission cache")
				return
			}
			permissionCacheCleared = cleared
			logger.LogOperation(h.db, c, "maintenance", "permission_cache", nil, map[string]interface{}{
				"cleared_keys": permissionCacheCleared,
			})
		}

		if req.Maintenance.ClearRuntimeRedisCache {
			cleared, err := cache.DeleteByPatterns(
				"rate:*",
				"captcha:*",
				"email_login_code:*",
				"password_reset:*",
				"phone_login_code:*",
				"phone_reset_code:*",
				"phone_register_code:*",
				"bind_email_code:*",
				"bind_phone_code:*",
				"email_login_cooldown:*",
				"password_reset_cooldown:*",
				"phone_login_cooldown:*",
				"phone_reset_cooldown:*",
				"bind_email_cooldown:*",
				"bind_phone_cooldown:*",
				"phone_register_cooldown:*",
				"invoice_dl:*",
				"invoice_pending:*",
				"ticket_notify:*",
			)
			if err != nil {
				response.InternalError(c, "Failed to clear runtime redis cache")
				return
			}
			runtimeRedisCacheCleared = cleared
			logger.LogOperation(h.db, c, "maintenance", "runtime_redis_cache", nil, map[string]interface{}{
				"cleared_keys": runtimeRedisCacheCleared,
			})
		}

		if req.Maintenance.DeleteUnreferencedFiles {
			cleanupStats, err := h.cleanupUnreferencedFiles()
			if err != nil {
				response.InternalError(c, fmt.Sprintf("Failed to delete unreferenced files: %v", err))
				return
			}
			unreferencedFileCleanup = cleanupStats
			logger.LogOperation(h.db, c, "maintenance", "unreferenced_files", nil, map[string]interface{}{
				"deleted_files": cleanupStats.DeletedFiles,
				"deleted_dirs":  cleanupStats.DeletedDirs,
				"deleted_bytes": cleanupStats.DeletedBytes,
			})
		}

		if req.Maintenance.RestartJSWorker {
			if h.pluginManager == nil {
				response.InternalError(c, "Plugin manager is not initialized")
				return
			}
			if err := h.pluginManager.RestartJSWorker(); err != nil {
				response.InternalError(c, fmt.Sprintf("Failed to restart JS worker: %v", err))
				return
			}
			jsWorkerRestarted = true
			logger.LogOperation(h.db, c, "maintenance", "js_worker_restart", nil, map[string]interface{}{
				"restarted": true,
			})
		}
	}

	// 保存到文件
	if err := writeConfigFile(configPath, currentConfig); err != nil {
		response.InternalError(c, "Failed to save config file")
		return
	}

	hotReloaded := false
	reloadErrorMessage := ""
	// 热更新内存中的配置
	if err := config.ReloadConfig(); err != nil {
		reloadErrorMessage = err.Error()
		// 记录错误但不阻止响应，配置文件已保存成功
		// 某些配置（如数据库、Redis、JWT）仍需重启才能生效
		logger.LogOperation(h.db, c, "update", "system_config", nil, map[string]interface{}{
			"config_sections": []string{"app", "smtp", "security", "rate_limit", "order"},
			"reload_error":    err.Error(),
		})
	} else {
		hotReloaded = true
		postReloadErrors := make([]string, 0, 4)

		if err := config.ReloadLogger(); err != nil {
			postReloadErrors = append(postReloadErrors, fmt.Sprintf("reload logger failed: %v", err))
		}
		if h.emailService != nil {
			h.emailService.RefreshConfig()
		}

		if req.Plugin.Submitted && h.pluginManager != nil && h.cfg != nil && previousPluginEnabled != h.cfg.Plugin.Enabled {
			if h.cfg.Plugin.Enabled {
				h.pluginManager.Stop()
				h.pluginManager.Start()
				pluginRuntimeAction = "started"
			} else {
				h.pluginManager.Stop()
				pluginRuntimeAction = "stopped"
			}
		}

		if req.Plugin.Submitted && h.pluginManager != nil && h.cfg != nil && previousPluginEnabled == h.cfg.Plugin.Enabled && h.cfg.Plugin.Enabled {
			if !jsWorkerRestarted && shouldAutoRestartManagedJSWorker(previousPluginConfig, h.cfg.Plugin) {
				if err := h.pluginManager.RestartJSWorker(); err != nil {
					postReloadErrors = append(postReloadErrors, fmt.Sprintf("restart js worker failed: %v", err))
				} else {
					jsWorkerRestarted = true
				}
			}
			if shouldReloadGRPCPlugins(previousPluginConfig, h.cfg.Plugin) {
				reloaded, err := h.reloadEnabledPluginsByRuntime(service.PluginRuntimeGRPC)
				grpcPluginsReloaded = reloaded
				if err != nil {
					postReloadErrors = append(postReloadErrors, fmt.Sprintf("reload grpc plugins failed: %v", err))
				}
			}
		}

		if len(postReloadErrors) > 0 {
			reloadErrorMessage = strings.Join(postReloadErrors, "; ")
		}

		// 记录操作日志
		logPayload := map[string]interface{}{
			"config_sections": []string{"app", "smtp", "security", "rate_limit", "order"},
			"hot_reload":      true,
		}
		if reloadErrorMessage != "" {
			logPayload["reload_error"] = reloadErrorMessage
		}
		if grpcPluginsReloaded > 0 {
			logPayload["grpc_plugins_reloaded"] = grpcPluginsReloaded
		}
		if jsWorkerRestarted {
			logPayload["js_worker_restarted"] = true
		}
		logger.LogOperation(h.db, c, "update", "system_config", nil, logPayload)
	}

	resp := gin.H{
		"message": "Settings saved and applied. Some configurations (Database, Redis, JWT) require service restart to take effect",
	}
	if pluginRuntimeAction != "" {
		resp["plugin_runtime_action"] = pluginRuntimeAction
	}
	if grpcPluginsReloaded > 0 {
		resp["grpc_plugins_reloaded"] = grpcPluginsReloaded
	}
	if jsWorkerRestarted {
		resp["js_worker_restarted"] = true
	}
	if req.Maintenance.Submitted {
		if req.Maintenance.ClearPaymentCardCache {
			resp["payment_card_cache_cleared"] = paymentCardCacheClearedRows
		}
		if req.Maintenance.ClearJSProgramCache {
			resp["js_program_cache_cleared"] = jsProgramCacheCleared
		}
		if req.Maintenance.ClearPermissionCache {
			resp["permission_cache_cleared"] = permissionCacheCleared
		}
		if req.Maintenance.ClearRuntimeRedisCache {
			resp["runtime_redis_cache_cleared"] = runtimeRedisCacheCleared
		}
		if req.Maintenance.DeleteUnreferencedFiles {
			resp["unreferenced_files_deleted"] = unreferencedFileCleanup.DeletedFiles
			resp["unreferenced_dirs_deleted"] = unreferencedFileCleanup.DeletedDirs
			resp["unreferenced_bytes_deleted"] = unreferencedFileCleanup.DeletedBytes
		}
	}
	if h.pluginManager != nil && (h.cfg == nil || h.cfg.Plugin.Enabled) {
		afterPayload, payloadErr := adminHookStructToPayload(req)
		if payloadErr != nil {
			log.Printf("settings.update.after payload build failed: admin=%d err=%v", adminID, payloadErr)
		} else {
			afterPayload["admin_id"] = adminID
			afterPayload["source"] = "admin_api"
			afterPayload["hot_reloaded"] = hotReloaded
			afterPayload["reload_error"] = reloadErrorMessage
			afterPayload["maintenance"] = map[string]interface{}{
				"payment_card_cache_cleared":  paymentCardCacheClearedRows,
				"js_program_cache_cleared":    jsProgramCacheCleared,
				"permission_cache_cleared":    permissionCacheCleared,
				"runtime_redis_cache_cleared": runtimeRedisCacheCleared,
				"unreferenced_files_deleted":  unreferencedFileCleanup.DeletedFiles,
				"unreferenced_dirs_deleted":   unreferencedFileCleanup.DeletedDirs,
				"unreferenced_bytes_deleted":  unreferencedFileCleanup.DeletedBytes,
				"js_worker_restarted":         jsWorkerRestarted,
			}
			afterPayload["plugin_runtime_action"] = pluginRuntimeAction
			go func(execCtx *service.ExecutionContext, payload map[string]interface{}) {
				_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
					Hook:    "settings.update.after",
					Payload: payload,
				}, execCtx)
				if hookErr != nil {
					log.Printf("settings.update.after hook execution failed: admin=%d err=%v", adminID, hookErr)
				}
			}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, &adminID, map[string]string{
				"hook_resource": "settings",
				"hook_source":   "admin_api",
			})), afterPayload)
		}
	}
	response.Success(c, resp)
}

// TestSMTP 测试SMTP配置
func (h *SettingsHandler) TestSMTP(c *gin.Context) {
	var req struct {
		Host     string `json:"host" binding:"required"`
		Port     int    `json:"port" binding:"required"`
		User     string `json:"user" binding:"required"`
		Password string `json:"password" binding:"required"`
		ToEmail  string `json:"to_email" binding:"required,email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	// 创建 SMTP 拨号器
	dialer := gomail.NewDialer(req.Host, req.Port, req.User, req.Password)
	dialer.TLSConfig = &tls.Config{
		ServerName:         req.Host,
		InsecureSkipVerify: false,
	}

	// 构建测试邮件
	m := gomail.NewMessage()
	m.SetHeader("From", req.User)
	m.SetHeader("To", req.ToEmail)
	m.SetHeader("Subject", "AuraLogic SMTP Test Email")

	appName := h.cfg.App.Name
	if appName == "" {
		appName = "AuraLogic"
	}

	body := fmt.Sprintf(`
		<html>
		<body style="font-family: Arial, sans-serif; padding: 20px;">
			<h2>SMTP Configuration Test Successful</h2>
			<p>This is a test email from <strong>%s</strong>.</p>
			<p>This is a test email from <strong>%s</strong>.</p>
			<hr>
			<p style="color: #666; font-size: 12px;">
				If you received this email, your SMTP configuration is correct.
			</p>
		</body>
		</html>
	`, appName, appName)

	m.SetBody("text/html", body)

	// 发送邮件
	if err := dialer.DialAndSend(m); err != nil {
		response.InternalError(c, fmt.Sprintf("Failed to send test email: %v", err))
		return
	}

	response.Success(c, gin.H{
		"message": "Test email sent, please check your inbox",
	})
}

// TestSMS 测试SMS配置
func (h *SettingsHandler) TestSMS(c *gin.Context) {
	var req struct {
		Phone string `json:"phone" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	if h.smsService == nil {
		response.InternalError(c, "SMS service is not initialized")
		return
	}

	if err := h.smsService.TestSMS(req.Phone); err != nil {
		response.InternalError(c, fmt.Sprintf("Failed to send test SMS: %v", err))
		return
	}

	response.Success(c, gin.H{
		"message": "Test SMS sent, please check your phone",
	})
}

// GetPageInject 根据页面路径返回匹配的注入脚本和样式
// 前端通过 path 查询参数传递当前页面路径（穿透CDN），同时回退检查 Referer
func (h *SettingsHandler) GetPageInject(c *gin.Context) {
	pagePath := c.Query("path")
	if pagePath == "" {
		// 回退：从 Referer 中提取路径
		referer := c.GetHeader("Referer")
		if referer != "" {
			// 简单解析：找到第三个 / 之后的路径部分
			slashCount := 0
			for i, ch := range referer {
				if ch == '/' {
					slashCount++
					if slashCount == 3 {
						pagePath = referer[i:]
						break
					}
				}
			}
		}
	}
	if pagePath == "" {
		pagePath = "/"
	}

	type matchedPageRule struct {
		Name      string `json:"name,omitempty"`
		Pattern   string `json:"pattern,omitempty"`
		MatchType string `json:"match_type,omitempty"`
		CSS       string `json:"css,omitempty"`
		JS        string `json:"js,omitempty"`
	}

	var css, js string
	matchedRules := make([]matchedPageRule, 0)
	for _, rule := range h.cfg.Customization.PageRules {
		if !rule.Enabled {
			continue
		}

		matched := false
		if rule.MatchType == "regex" {
			re, err := regexp.Compile(rule.Pattern)
			if err != nil {
				continue
			}
			matched = re.MatchString(pagePath)
		} else {
			matched = pagePath == rule.Pattern
		}

		if matched {
			if rule.CSS != "" {
				css += rule.CSS + "\n"
			}
			if rule.JS != "" {
				js += rule.JS + "\n"
			}
			matchedRules = append(matchedRules, matchedPageRule{
				Name:      rule.Name,
				Pattern:   rule.Pattern,
				MatchType: rule.MatchType,
				CSS:       rule.CSS,
				JS:        rule.JS,
			})
		}
	}

	c.Header("Cache-Control", "public, max-age=300")
	response.Success(c, gin.H{
		"path":          pagePath,
		"css":           css,
		"js":            js,
		"rules":         matchedRules,
		"matched_count": len(matchedRules),
	})
}

// ListEmailTemplates 获取所有邮件模板列表
func (h *SettingsHandler) ListEmailTemplates(c *gin.Context) {
	templateDir, err := filepath.Abs("templates/email")
	if err != nil {
		response.InternalError(c, "Failed to resolve template path")
		return
	}

	entries, err := os.ReadDir(templateDir)
	if err != nil {
		response.Success(c, []interface{}{})
		return
	}

	type TemplateInfo struct {
		Name     string `json:"name"`
		Event    string `json:"event"`
		Locale   string `json:"locale"`
		Filename string `json:"filename"`
	}

	var templates []TemplateInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".html") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".html")
		event := name
		locale := ""

		// 解析 {event}_{locale}.html 格式
		if idx := strings.LastIndex(name, "_"); idx > 0 {
			possibleLocale := name[idx+1:]
			if possibleLocale == "zh" || possibleLocale == "en" {
				event = name[:idx]
				locale = possibleLocale
			}
		}

		templates = append(templates, TemplateInfo{
			Name:     name,
			Event:    event,
			Locale:   locale,
			Filename: entry.Name(),
		})
	}

	response.Success(c, templates)
}

// GetEmailTemplate 获取单个邮件模板内容
func (h *SettingsHandler) GetEmailTemplate(c *gin.Context) {
	filename := c.Param("filename")

	// 安全检查：只允许 .html 文件，不允许路径遍历
	if !strings.HasSuffix(filename, ".html") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") || strings.Contains(filename, "..") {
		response.BadRequest(c, "Invalid template filename")
		return
	}

	templateDir, err := filepath.Abs("templates/email")
	if err != nil {
		response.InternalError(c, "Failed to resolve template path")
		return
	}

	content, err := os.ReadFile(filepath.Join(templateDir, filename))
	if err != nil {
		response.NotFound(c, "Template not found")
		return
	}

	response.Success(c, gin.H{
		"filename": filename,
		"content":  string(content),
	})
}

// UpdateEmailTemplate 更新邮件模板内容
func (h *SettingsHandler) UpdateEmailTemplate(c *gin.Context) {
	filename := c.Param("filename")

	// 安全检查
	if !strings.HasSuffix(filename, ".html") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") || strings.Contains(filename, "..") {
		response.BadRequest(c, "Invalid template filename")
		return
	}

	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}
	adminID := middleware.MustGetUserID(c)
	if h.pluginManager != nil {
		hookPayload := map[string]interface{}{
			"filename": filename,
			"content":  req.Content,
			"admin_id": adminID,
			"source":   "admin_api",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "email_template.update.before",
			Payload: hookPayload,
		}, buildAdminHookExecutionContext(c, &adminID, map[string]string{
			"hook_resource": "email_template",
			"hook_source":   "admin_api",
			"template_name": filename,
		}))
		if hookErr != nil {
			log.Printf("email_template.update.before hook execution failed: admin=%d filename=%s err=%v", adminID, filename, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Email template update rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if value, exists := hookResult.Payload["content"]; exists {
					req.Content = parseStringFromAny(value)
				}
			}
		}
	}

	templateDir, err := filepath.Abs("templates/email")
	if err != nil {
		response.InternalError(c, "Failed to resolve template path")
		return
	}

	tmplPath := filepath.Join(templateDir, filename)

	// 确认文件存在
	if _, err := os.Stat(tmplPath); os.IsNotExist(err) {
		response.NotFound(c, "Template not found")
		return
	}

	// 写入文件
	if err := os.WriteFile(tmplPath, []byte(req.Content), 0644); err != nil {
		response.InternalError(c, "Failed to save template")
		return
	}

	hotReloaded, reloadWarning := h.reloadEmailTemplates()

	// 记录操作日志
	logger.LogOperation(h.db, c, "update", "email_template", nil, map[string]interface{}{
		"filename":       filename,
		"hot_reloaded":   hotReloaded,
		"reload_warning": reloadWarning,
	})
	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"filename":       filename,
			"content":        req.Content,
			"content_length": len(req.Content),
			"hot_reloaded":   hotReloaded,
			"reload_warning": reloadWarning,
			"admin_id":       adminID,
			"source":         "admin_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "email_template.update.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("email_template.update.after hook execution failed: admin=%d filename=%s err=%v", adminID, filename, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, &adminID, map[string]string{
			"hook_resource": "email_template",
			"hook_source":   "admin_api",
			"template_name": filename,
		})), afterPayload)
	}

	response.Success(c, gin.H{
		"message": func() string {
			if reloadWarning != "" {
				return "Template saved; runtime reload will be applied on next email render"
			}
			return "Template saved successfully"
		}(),
		"hot_reloaded":   hotReloaded,
		"reload_warning": reloadWarning,
	})
}

func (h *SettingsHandler) ImportTemplatePackage(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		fileHeader, err = c.FormFile("package")
	}
	if err != nil || fileHeader == nil {
		response.BadRequest(c, "Template package file is required")
		return
	}

	adminID := middleware.MustGetUserID(c)
	file, err := fileHeader.Open()
	if err != nil {
		response.InternalError(c, "Failed to open template package")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, service.TemplatePackageImportMaxBytes+1))
	if err != nil {
		response.InternalError(c, "Failed to read template package")
		return
	}
	if int64(len(data)) > service.TemplatePackageImportMaxBytes {
		response.BadRequest(c, fmt.Sprintf("Template package exceeds %d bytes", service.TemplatePackageImportMaxBytes))
		return
	}
	expectedKind := strings.TrimSpace(c.PostForm("expected_kind"))
	targetKey := strings.TrimSpace(c.PostForm("target_key"))
	if h.pluginManager != nil {
		hookPayload := map[string]interface{}{
			"filename":           fileHeader.Filename,
			"package_size_bytes": len(data),
			"expected_kind":      expectedKind,
			"target_key":         targetKey,
			"admin_id":           adminID,
			"source":             "admin_api",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "template.package.import.before",
			Payload: hookPayload,
		}, buildAdminHookExecutionContext(c, &adminID, map[string]string{
			"hook_resource": "template_package",
			"hook_source":   "admin_api",
		}))
		if hookErr != nil {
			log.Printf("template.package.import.before hook execution failed: admin=%d filename=%s err=%v", adminID, fileHeader.Filename, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Template package import rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if value, exists := hookResult.Payload["expected_kind"]; exists {
					expectedKind = parseStringFromAny(value)
				}
				if value, exists := hookResult.Payload["target_key"]; exists {
					targetKey = parseStringFromAny(value)
				}
			}
		}
	}

	var importedBy *uint
	if userID, exists := middleware.GetUserID(c); exists {
		importedBy = &userID
	}

	result, importErr := service.ImportTemplatePackage(
		h.db,
		h.emailService,
		fileHeader.Filename,
		data,
		service.TemplatePackageImportOptions{
			ExpectedKind: expectedKind,
			TargetKey:    targetKey,
			ImportedBy:   importedBy,
		},
	)
	if importErr != nil {
		h.writeTemplatePackageImportError(c, importErr)
		return
	}

	logger.LogOperation(h.db, c, "import", "template_package", nil, map[string]interface{}{
		"filename":       fileHeader.Filename,
		"kind":           pluginMarketStringValue(result["kind"]),
		"name":           pluginMarketStringValue(result["name"]),
		"version":        pluginMarketStringValue(result["version"]),
		"target_key":     nestedTemplateImportResultString(result, "resolved", "target_key"),
		"hot_reloaded":   result["hot_reloaded"],
		"reload_warning": pluginMarketStringValue(result["reload_warning"]),
	})
	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"filename":        fileHeader.Filename,
			"expected_kind":   expectedKind,
			"target_key":      targetKey,
			"kind":            pluginMarketStringValue(result["kind"]),
			"name":            pluginMarketStringValue(result["name"]),
			"version":         pluginMarketStringValue(result["version"]),
			"resolved_target": nestedTemplateImportResultString(result, "resolved", "target_key"),
			"hot_reloaded":    result["hot_reloaded"],
			"reload_warning":  pluginMarketStringValue(result["reload_warning"]),
			"package_size":    len(data),
			"admin_id":        adminID,
			"source":          "admin_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "template.package.import.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("template.package.import.after hook execution failed: admin=%d filename=%s err=%v", adminID, fileHeader.Filename, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, &adminID, map[string]string{
			"hook_resource": "template_package",
			"hook_source":   "admin_api",
		})), afterPayload)
	}

	response.Success(c, result)
}

func (h *SettingsHandler) reloadEmailTemplates() (bool, string) {
	if h == nil || h.emailService == nil {
		return false, "email service is not initialized"
	}
	if err := h.emailService.ReloadTemplates(); err != nil {
		return false, err.Error()
	}
	return true, ""
}

func (h *SettingsHandler) writeTemplatePackageImportError(c *gin.Context, err error) {
	var hostErr *service.PluginHostActionError
	if errors.As(err, &hostErr) {
		switch hostErr.Status {
		case http.StatusBadRequest:
			response.BadRequest(c, hostErr.Message)
			return
		case http.StatusNotFound:
			response.NotFound(c, hostErr.Message)
			return
		case http.StatusConflict:
			response.Conflict(c, hostErr.Message)
			return
		case http.StatusServiceUnavailable:
			response.Error(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, hostErr.Message)
			return
		default:
			response.Error(c, hostErr.Status, response.CodeInternalError, hostErr.Message)
			return
		}
	}
	response.InternalError(c, "Failed to import template package")
}

func pluginMarketStringValue(value interface{}) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprintf("%v", value))
}

func nestedTemplateImportResultString(result map[string]interface{}, parentKey string, childKey string) string {
	parent, ok := result[parentKey].(map[string]interface{})
	if !ok || parent == nil {
		return ""
	}
	return pluginMarketStringValue(parent[childKey])
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

// readConfigFile 读取配置文件
func readConfigFile(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return config, nil
}

// writeConfigFile 写入配置文件
func writeConfigFile(path string, config map[string]interface{}) error {
	data, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
