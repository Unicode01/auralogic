package admin

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"auralogic/internal/config"
	"auralogic/internal/pkg/logger"
	"auralogic/internal/pkg/response"
	"gopkg.in/gomail.v2"
	"gorm.io/gorm"
)

type SettingsHandler struct {
	db  *gorm.DB
	cfg *config.Config
}

func NewSettingsHandler(db *gorm.DB, cfg *config.Config) *SettingsHandler {
	return &SettingsHandler{
		db:  db,
		cfg: cfg,
	}
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
		"currency":           h.cfg.Order.Currency,
		"app_name":           h.cfg.App.Name,
		"default_theme":      defaultTheme,
		"allow_registration": h.cfg.Security.Login.AllowRegistration,
		"stock_display": gin.H{
			"mode":                 h.cfg.Order.StockDisplay.Mode,
			"low_stock_threshold":  h.cfg.Order.StockDisplay.LowStockThreshold,
			"high_stock_threshold": h.cfg.Order.StockDisplay.HighStockThreshold,
		},
		"customization": gin.H{
			"primary_color":  h.cfg.Customization.PrimaryColor,
			"logo_url":       h.cfg.Customization.LogoURL,
			"favicon_url":    h.cfg.Customization.FaviconURL,
			"auth_branding":  h.renderAuthBranding(),
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
		"smtp_enabled": h.cfg.SMTP.Enabled,
		"captcha": gin.H{
			"provider":                 h.cfg.Security.Captcha.Provider,
			"site_key":                 h.cfg.Security.Captcha.SiteKey,
			"enable_for_login":         h.cfg.Security.Captcha.EnableForLogin,
			"enable_for_register":      h.cfg.Security.Captcha.EnableForRegister,
			"enable_for_serial_verify": h.cfg.Security.Captcha.EnableForSerialVerify,
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
			// Password不返回
		},
		"security": gin.H{
			"password_policy": h.cfg.Security.PasswordPolicy,
			"login":           h.cfg.Security.Login,
			"cors":            h.cfg.Security.CORS,
			"captcha":         h.cfg.Security.Captcha,
			"ip_header":       h.cfg.Security.IPHeader,
			"trusted_proxies": h.cfg.Security.TrustedProxies,
		},
		"rate_limit": h.cfg.RateLimit,
		"order": gin.H{
			"no_prefix":              h.cfg.Order.NoPrefix,
			"auto_cancel_hours":      h.cfg.Order.AutoCancelHours,
			"currency":               h.cfg.Order.Currency,
			"virtual_delivery_order": h.cfg.Order.VirtualDeliveryOrder,
			"stock_display": gin.H{
				"mode":                 h.cfg.Order.StockDisplay.Mode,
				"low_stock_threshold":  h.cfg.Order.StockDisplay.LowStockThreshold,
				"high_stock_threshold": h.cfg.Order.StockDisplay.HighStockThreshold,
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
			"primary_color":  h.cfg.Customization.PrimaryColor,
			"logo_url":       h.cfg.Customization.LogoURL,
			"favicon_url":    h.cfg.Customization.FaviconURL,
			"page_rules":     h.cfg.Customization.PageRules,
			"auth_branding":  h.cfg.Customization.AuthBranding,
		},
		"email_notifications": h.cfg.EmailNotifications,
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

	Order struct {
		NoPrefix             string                    `json:"no_prefix"`
		AutoCancelHours      int                       `json:"auto_cancel_hours"`
		Currency             string                    `json:"currency"`
		VirtualDeliveryOrder string                    `json:"virtual_delivery_order"`
		StockDisplay         config.StockDisplayConfig `json:"stock_display"`
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
		Submitted    bool                      `json:"_submitted"`
		PrimaryColor string                    `json:"primary_color"`
		LogoURL      string                    `json:"logo_url"`
		FaviconURL   string                    `json:"favicon_url"`
		PageRules    []config.PageRule          `json:"page_rules"`
		AuthBranding *config.AuthBrandingConfig `json:"auth_branding,omitempty"`
	} `json:"customization,omitempty"`

	EmailNotifications *config.EmailNotificationsConfig `json:"email_notifications,omitempty"`

	Analytics struct {
		Submitted bool `json:"_submitted"`
		Enabled   bool `json:"enabled"`
	} `json:"analytics,omitempty"`
}

// UpdateSettings Update系统设置
func (h *SettingsHandler) UpdateSettings(c *gin.Context) {
	var req UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

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
		securityConfig := currentConfig["security"].(map[string]interface{})
		securityConfig["login"] = map[string]interface{}{
			"allow_password_login":       req.Security.Login.AllowPasswordLogin,
			"allow_registration":         req.Security.Login.AllowRegistration,
			"require_email_verification": req.Security.Login.RequireEmailVerification,
		}
	}

	// Update限流配置
	if req.RateLimit.API > 0 {
		currentConfig["rate_limit"] = map[string]interface{}{
			"enabled":       req.RateLimit.Enabled,
			"api":           req.RateLimit.API,
			"user_login":    req.RateLimit.UserLogin,
			"user_request":  req.RateLimit.UserRequest,
			"admin_request": req.RateLimit.AdminRequest,
		}
	}

	// UpdateOrder配置
	if req.Order.NoPrefix != "" {
		currentConfig["order"] = map[string]interface{}{
			"no_prefix":              req.Order.NoPrefix,
			"auto_cancel_hours":      req.Order.AutoCancelHours,
			"currency":               req.Order.Currency,
			"virtual_delivery_order": req.Order.VirtualDeliveryOrder,
			"stock_display": map[string]interface{}{
				"mode":                 req.Order.StockDisplay.Mode,
				"low_stock_threshold":  req.Order.StockDisplay.LowStockThreshold,
				"high_stock_threshold": req.Order.StockDisplay.HighStockThreshold,
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

	// 保存到文件
	if err := writeConfigFile(configPath, currentConfig); err != nil {
		response.InternalError(c, "Failed to save config file")
		return
	}

	// 热更新内存中的配置
	if err := config.ReloadConfig(); err != nil {
		// 记录错误但不阻止响应，配置文件已保存成功
		// 某些配置（如数据库、Redis、JWT）仍需重启才能生效
		logger.LogOperation(h.db, c, "update", "system_config", nil, map[string]interface{}{
			"config_sections": []string{"app", "smtp", "security", "rate_limit", "order"},
			"reload_error":    err.Error(),
		})
	} else {
		// 记录操作日志
		logger.LogOperation(h.db, c, "update", "system_config", nil, map[string]interface{}{
			"config_sections": []string{"app", "smtp", "security", "rate_limit", "order"},
			"hot_reload":      true,
		})
	}

	response.Success(c, gin.H{
		"message": "设置已保存并应用，部分配置（数据库、Redis、JWT）需重启服务后生效",
	})
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
	m.SetHeader("Subject", "AuraLogic SMTP 测试邮件 / SMTP Test Email")

	appName := h.cfg.App.Name
	if appName == "" {
		appName = "AuraLogic"
	}

	body := fmt.Sprintf(`
		<html>
		<body style="font-family: Arial, sans-serif; padding: 20px;">
			<h2>SMTP 配置测试成功 / SMTP Configuration Test Successful</h2>
			<p>这是一封来自 <strong>%s</strong> 的测试邮件。</p>
			<p>This is a test email from <strong>%s</strong>.</p>
			<hr>
			<p style="color: #666; font-size: 12px;">
				如果您收到此邮件，说明 SMTP 配置正确。<br>
				If you received this email, your SMTP configuration is correct.
			</p>
		</body>
		</html>
	`, appName, appName)

	m.SetBody("text/html", body)

	// 发送邮件
	if err := dialer.DialAndSend(m); err != nil {
		response.InternalError(c, fmt.Sprintf("发送测试邮件失败: %v", err))
		return
	}

	response.Success(c, gin.H{
		"message": "测试邮件已发送，请检查收件箱",
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

	var css, js string
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
		}
	}

	c.Header("Cache-Control", "public, max-age=300")
	response.Success(c, gin.H{
		"css": css,
		"js":  js,
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

	// 记录操作日志
	logger.LogOperation(h.db, c, "update", "email_template", nil, map[string]interface{}{
		"filename": filename,
	})

	response.Success(c, gin.H{
		"message": "Template saved successfully",
	})
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
