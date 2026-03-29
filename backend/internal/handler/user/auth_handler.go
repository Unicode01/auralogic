package user

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/database"
	"auralogic/internal/middleware"
	"auralogic/internal/models"
	"auralogic/internal/pkg/authbiz"
	"auralogic/internal/pkg/cache"
	"auralogic/internal/pkg/logger"
	"auralogic/internal/pkg/response"
	"auralogic/internal/pkg/utils"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AuthHandler struct {
	authService    *service.AuthService
	emailService   *service.EmailService
	smsService     *service.SMSService
	captchaService *service.CaptchaService
	pluginManager  *service.PluginManagerService
}

func NewAuthHandler(authService *service.AuthService, emailService *service.EmailService, smsService *service.SMSService, pluginManager *service.PluginManagerService) *AuthHandler {
	return &AuthHandler{
		authService:    authService,
		emailService:   emailService,
		smsService:     smsService,
		captchaService: service.NewCaptchaService(),
		pluginManager:  pluginManager,
	}
}

func (h *AuthHandler) buildAuthHookExecutionContext(c *gin.Context, userID *uint) *service.ExecutionContext {
	if c == nil {
		return nil
	}

	metadata := map[string]string{
		"request_path":    c.Request.URL.Path,
		"route":           c.FullPath(),
		"method":          c.Request.Method,
		"client_ip":       utils.GetRealIP(c),
		"user_agent":      c.GetHeader("User-Agent"),
		"accept_language": c.GetHeader("Accept-Language"),
		"operator_type":   "user",
		"auth_scene":      "auth_api",
	}
	sessionID := strings.TrimSpace(c.GetHeader("X-Session-ID"))
	return &service.ExecutionContext{
		UserID:         userID,
		SessionID:      sessionID,
		Metadata:       metadata,
		RequestContext: c.Request.Context(),
	}
}

func cloneAuthExecutionContext(execCtx *service.ExecutionContext) *service.ExecutionContext {
	if execCtx == nil {
		return nil
	}

	cloned := &service.ExecutionContext{
		SessionID:      execCtx.SessionID,
		RequestContext: execCtx.RequestContext,
	}
	if execCtx.UserID != nil {
		userID := *execCtx.UserID
		cloned.UserID = &userID
	}
	if execCtx.OrderID != nil {
		orderID := *execCtx.OrderID
		cloned.OrderID = &orderID
	}
	if len(execCtx.Metadata) > 0 {
		cloned.Metadata = make(map[string]string, len(execCtx.Metadata))
		for key, value := range execCtx.Metadata {
			cloned.Metadata[key] = value
		}
	}
	return cloned
}

func loadEffectiveAdminPermissions(db *gorm.DB, userID uint, role string) []string {
	if role != "admin" && role != "super_admin" {
		return []string{}
	}

	permissions := middleware.EffectiveAdminPermissions(role, nil)
	if db == nil {
		return permissions
	}

	var perm models.AdminPermission
	if err := db.Where("user_id = ?", userID).First(&perm).Error; err == nil {
		return middleware.EffectiveAdminPermissions(role, perm.Permissions)
	} else if err != gorm.ErrRecordNotFound {
		return []string{}
	}

	return permissions
}

// LoginRequest 登录请求
type LoginRequest struct {
	Email        string `json:"email" binding:"required,email"`
	Password     string `json:"password" binding:"required"`
	CaptchaToken string `json:"captcha_token"`
}

// Login User登录
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	// 验证码校验
	if h.captchaService.NeedCaptcha("login") {
		if req.CaptchaToken == "" {
			respondAuthBizError(c, authbiz.CaptchaRequired(), nil)
			return
		}
		if err := h.captchaService.VerifyCaptcha(req.CaptchaToken, utils.GetRealIP(c)); err != nil {
			respondAuthBizError(c, authbiz.CaptchaFailed(), nil)
			return
		}
	}
	hookExecCtx := h.buildAuthHookExecutionContext(c, nil)
	if h.pluginManager != nil {
		hookPayload := map[string]interface{}{
			"auth_method":      "password",
			"email":            req.Email,
			"password_present": strings.TrimSpace(req.Password) != "",
			"source":           "user_api",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "auth.login.before",
			Payload: hookPayload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("auth.login.before hook execution failed: email=%s err=%v", req.Email, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Login rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if rawEmail, exists := hookResult.Payload["email"]; exists {
					email, convErr := authHookValueToOptionalString(rawEmail)
					if convErr != nil {
						log.Printf("auth.login.before payload email decode failed, keep original request: email=%s err=%v", req.Email, convErr)
					} else {
						req.Email = strings.ToLower(strings.TrimSpace(email))
					}
				}
			}
		}
	}

	token, user, err := h.authService.Login(req.Email, req.Password)
	if err != nil {
		db := database.GetDB()
		logger.LogLoginAttempt(db, c, req.Email, false, nil)
		if respondAuthBizError(c, err, gin.H{
			"email":           req.Email,
			"allowed_methods": []string{"magic_link", "oauth"},
		}) {
			return
		}
		response.InternalServerError(c, "Login failed", err)
		return
	}

	// 记录登录IP
	user.LastLoginIP = utils.GetRealIP(c)
	h.authService.UpdateLoginIP(user)

	// 记录成功的登录
	db := database.GetDB()
	logger.LogLoginAttempt(db, c, req.Email, true, &user.ID)

	// 构建响应数据
	result := gin.H{
		"id":                user.ID,
		"user_id":           user.ID,
		"uuid":              user.UUID,
		"email":             user.Email,
		"name":              user.Name,
		"role":              user.Role,
		"avatar":            user.Avatar,
		"locale":            user.Locale,
		"total_spent_minor": user.TotalSpentMinor,
		"total_order_count": user.TotalOrderCount,
	}

	// 如果是Admin，getPermission列表
	if user.IsAdmin() {
		result["permissions"] = loadEffectiveAdminPermissions(database.GetDB(), user.ID, user.Role)
	}
	if h.pluginManager != nil {
		uid := user.ID
		afterExecCtx := h.buildAuthHookExecutionContext(c, &uid)
		afterPayload := map[string]interface{}{
			"auth_method": "password",
			"user_id":     user.ID,
			"email":       user.Email,
			"role":        user.Role,
			"source":      "user_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, email string) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "auth.login.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("auth.login.after hook execution failed: email=%s err=%v", email, hookErr)
			}
		}(cloneAuthExecutionContext(afterExecCtx), afterPayload, user.Email)
	}

	response.Success(c, gin.H{
		"token":      token,
		"token_type": "Bearer",
		"user":       result,
	})
}

// RegisterRequest 注册请求
type RegisterRequest struct {
	Email        string `json:"email" binding:"required,email,max=255"`
	Password     string `json:"password" binding:"required,min=8"`
	Name         string `json:"name" binding:"required,min=2,max=100"`
	CaptchaToken string `json:"captcha_token"`
}

// Register 用户注册
func (h *AuthHandler) Register(c *gin.Context) {
	// 检查是否允许注册
	cfg := config.GetConfig()
	if !cfg.Security.Login.AllowRegistration {
		respondAuthBizError(c, authbiz.RegistrationDisabled(), nil)
		return
	}

	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Name = strings.TrimSpace(req.Name)

	// 验证码校验
	if h.captchaService.NeedCaptcha("register") {
		if req.CaptchaToken == "" {
			respondAuthBizError(c, authbiz.CaptchaRequired(), nil)
			return
		}
		if err := h.captchaService.VerifyCaptcha(req.CaptchaToken, utils.GetRealIP(c)); err != nil {
			respondAuthBizError(c, authbiz.CaptchaFailed(), nil)
			return
		}
	}
	hookExecCtx := h.buildAuthHookExecutionContext(c, nil)
	if h.pluginManager != nil {
		hookPayload := map[string]interface{}{
			"auth_method":      "email",
			"email":            req.Email,
			"name":             req.Name,
			"password_present": strings.TrimSpace(req.Password) != "",
			"source":           "user_api",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "auth.register.before",
			Payload: hookPayload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("auth.register.before hook execution failed: email=%s err=%v", req.Email, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Registration rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if rawEmail, exists := hookResult.Payload["email"]; exists {
					email, convErr := authHookValueToOptionalString(rawEmail)
					if convErr != nil {
						log.Printf("auth.register.before payload email decode failed, keep original request: email=%s err=%v", req.Email, convErr)
					} else {
						req.Email = strings.ToLower(strings.TrimSpace(email))
					}
				}
				if rawName, exists := hookResult.Payload["name"]; exists {
					name, convErr := authHookValueToOptionalString(rawName)
					if convErr != nil {
						log.Printf("auth.register.before payload name decode failed, keep original request: email=%s err=%v", req.Email, convErr)
					} else {
						req.Name = strings.TrimSpace(name)
					}
				}
			}
		}
	}

	user, err := h.authService.Register(req.Email, "", req.Name, req.Password)
	if err != nil {
		switch {
		case respondAuthBizError(c, err, nil):
		case errors.Is(err, service.ErrRegisterInternal):
			response.InternalError(c, "Registration failed")
		default:
			respondAuthValidationOrInternalError(c, err, "Registration failed")
		}
		return
	}

	// 记录注册IP
	user.RegisterIP = utils.GetRealIP(c)
	db := database.GetDB()
	if err := db.Save(user).Error; err != nil {
		response.InternalError(c, "Registration failed")
		return
	}

	// 记录注册日志
	logger.LogOperation(db, c, "register", "user", &user.ID, map[string]interface{}{
		"email":       user.Email,
		"name":        user.Name,
		"register_ip": user.RegisterIP,
	})
	emitRegisterAfter := func(requireVerification bool) {
		if h.pluginManager == nil {
			return
		}
		uid := user.ID
		afterPayload := map[string]interface{}{
			"auth_method":          "email",
			"user_id":              user.ID,
			"email":                user.Email,
			"name":                 user.Name,
			"require_verification": requireVerification,
			"email_verified":       user.EmailVerified,
			"source":               "user_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, email string) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "auth.register.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("auth.register.after hook execution failed: email=%s err=%v", email, hookErr)
			}
		}(cloneAuthExecutionContext(h.buildAuthHookExecutionContext(c, &uid)), afterPayload, user.Email)
	}

	// 如果需要邮箱验证
	if cfg.Security.Login.RequireEmailVerification && h.emailService != nil {
		// 生成验证 token
		token, err := generateVerificationToken()
		if err != nil {
			response.InternalError(c, "Failed to generate verification token")
			return
		}

		// 保存 token 到数据库
		verifyToken := &models.EmailVerificationToken{
			Token:     token,
			UserID:    user.ID,
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}
		if err := db.Create(verifyToken).Error; err != nil {
			response.InternalError(c, "Failed to create verification token")
			return
		}

		// 发送验证邮件
		go h.emailService.SendVerificationEmail(user.Email, user.Name, token, user.Locale)
		emitRegisterAfter(true)

		response.Success(c, gin.H{
			"require_verification": true,
			"message":              "Registration successful. Please check your email to verify your account.",
			"email":                user.Email,
		})
		return
	}

	// 不需要邮箱验证，直接标记已验证并登录
	user.EmailVerified = true
	if err := db.Save(user).Error; err != nil {
		response.InternalError(c, "Registration failed")
		return
	}

	// 生成JWT Token
	jwtToken, err := h.authService.GenerateToken(user)
	if err != nil {
		response.InternalError(c, "Failed to generate token")
		return
	}

	// 记录登录IP
	user.LastLoginIP = utils.GetRealIP(c)
	h.authService.UpdateLoginIP(user)

	// 发送注册欢迎邮件
	if h.emailService != nil {
		go h.emailService.SendRegistrationWelcomeEmail(user.Email, user.Name, user.Locale)
	}
	emitRegisterAfter(false)

	response.Success(c, gin.H{
		"token":      jwtToken,
		"token_type": "Bearer",
		"user": gin.H{
			"id":                user.ID,
			"user_id":           user.ID,
			"uuid":              user.UUID,
			"email":             user.Email,
			"name":              user.Name,
			"role":              user.Role,
			"avatar":            user.Avatar,
			"locale":            user.Locale,
			"total_spent_minor": user.TotalSpentMinor,
			"total_order_count": user.TotalOrderCount,
		},
	})
}

// maskPhone masks a phone number, e.g. "13300003333" -> "13*******33"
func maskPhone(phone string) string {
	n := len(phone)
	if n <= 4 {
		return phone
	}
	return phone[:2] + strings.Repeat("*", n-4) + phone[n-2:]
}

// phoneRegexp validates phone numbers: digits only, 5-20 chars.
var phoneRegexp = regexp.MustCompile(`^\d{5,20}$`)

// validatePhone checks that a phone number contains only digits and is a reasonable length.
func validatePhone(phone string) bool {
	return phoneRegexp.MatchString(phone)
}

func authHookValueToOptionalString(value interface{}) (string, error) {
	if value == nil {
		return "", nil
	}
	str, ok := value.(string)
	if !ok {
		return "", errors.New("value must be string")
	}
	return str, nil
}

// GetMe getcurrentUserInfo
func (h *AuthHandler) GetMe(c *gin.Context) {
	userID := middleware.MustGetUserID(c)

	user, err := h.authService.GetUserByID(userID)
	if err != nil {
		response.NotFound(c, "User not found")
		return
	}

	// 构建响应数据
	result := gin.H{
		"id":                     user.ID,
		"user_id":                user.ID,
		"uuid":                   user.UUID,
		"email":                  user.Email,
		"name":                   user.Name,
		"role":                   user.Role,
		"avatar":                 user.Avatar,
		"is_active":              user.IsActive,
		"locale":                 user.Locale,
		"country":                user.Country,
		"email_notify_order":     user.EmailNotifyOrder,
		"email_notify_ticket":    user.EmailNotifyTicket,
		"email_notify_marketing": user.EmailNotifyMarketing,
		"sms_notify_marketing":   user.SMSNotifyMarketing,
		"total_spent_minor":      user.TotalSpentMinor,
		"total_order_count":      user.TotalOrderCount,
		"created_at":             user.CreatedAt,
	}
	if user.Phone != nil && *user.Phone != "" {
		result["phone"] = maskPhone(*user.Phone)
	}

	// 如果是Admin，getPermission列表
	if user.IsAdmin() {
		result["permissions"] = loadEffectiveAdminPermissions(database.GetDB(), userID, user.Role)
	} else {
		result["permissions"] = []string{}
	}

	response.Success(c, result)
}

// Logout 用户登出（客户端清除token即可，服务端预留扩展）
func (h *AuthHandler) Logout(c *gin.Context) {
	response.Success(c, gin.H{
		"message": "Logged out successfully",
	})
}

// ChangePasswordRequest 修改Password请求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

// ChangePassword 修改Password
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID := middleware.MustGetUserID(c)

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}
	hookExecCtx := h.buildAuthHookExecutionContext(c, &userID)
	if h.pluginManager != nil {
		hookPayload := map[string]interface{}{
			"user_id":              userID,
			"old_password_present": strings.TrimSpace(req.OldPassword) != "",
			"new_password_present": strings.TrimSpace(req.NewPassword) != "",
			"source":               "user_api",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "auth.password.change.before",
			Payload: hookPayload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("auth.password.change.before hook execution failed: user=%d err=%v", userID, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Password change rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
		}
	}

	if err := h.authService.ChangePassword(userID, req.OldPassword, req.NewPassword); err != nil {
		if respondAuthBizError(c, err, nil) {
			return
		}
		respondAuthValidationOrInternalError(c, err, "Password change failed")
		return
	}
	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"user_id": userID,
			"source":  "user_api",
			"success": true,
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "auth.password.change.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("auth.password.change.after hook execution failed: user=%d err=%v", userID, hookErr)
			}
		}(cloneAuthExecutionContext(hookExecCtx), afterPayload)
	}

	response.Success(c, gin.H{
		"message": "Password changed successfully",
	})
}

// UpdatePreferencesRequest 更新用户偏好请求
type UpdatePreferencesRequest struct {
	Locale               string `json:"locale"`
	Country              string `json:"country"`
	EmailNotifyOrder     *bool  `json:"email_notify_order"`
	EmailNotifyTicket    *bool  `json:"email_notify_ticket"`
	EmailNotifyMarketing *bool  `json:"email_notify_marketing"`
	SMSNotifyMarketing   *bool  `json:"sms_notify_marketing"`
}

// UpdatePreferences 更新用户偏好设置
func (h *AuthHandler) UpdatePreferences(c *gin.Context) {
	userID := middleware.MustGetUserID(c)

	var req UpdatePreferencesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}
	hookExecCtx := h.buildAuthHookExecutionContext(c, &userID)
	if h.pluginManager != nil {
		originalReq := req
		hookPayload, payloadErr := userHookStructToPayload(req)
		if payloadErr != nil {
			log.Printf("auth.preferences.update.before payload build failed: user=%d err=%v", userID, payloadErr)
		} else {
			hookPayload["user_id"] = userID
			hookPayload["source"] = "user_api"
			hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "auth.preferences.update.before",
				Payload: hookPayload,
			}, hookExecCtx)
			if hookErr != nil {
				log.Printf("auth.preferences.update.before hook execution failed: user=%d err=%v", userID, hookErr)
			} else if hookResult != nil {
				if hookResult.Blocked {
					reason := strings.TrimSpace(hookResult.BlockReason)
					if reason == "" {
						reason = "Preference update rejected by plugin"
					}
					response.BadRequest(c, reason)
					return
				}
				if hookResult.Payload != nil {
					if mergeErr := mergeUserHookStructPatch(&req, hookResult.Payload); mergeErr != nil {
						log.Printf("auth.preferences.update.before payload apply failed, fallback to original request: user=%d err=%v", userID, mergeErr)
						req = originalReq
					}
				}
			}
		}
	}

	if err := h.authService.UpdatePreferences(
		userID,
		req.Locale,
		req.Country,
		req.EmailNotifyOrder,
		req.EmailNotifyTicket,
		req.EmailNotifyMarketing,
		req.SMSNotifyMarketing,
	); err != nil {
		if respondAuthBizError(c, err, nil) {
			return
		}
		response.InternalServerError(c, "Failed to update preferences", err)
		return
	}
	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"user_id":                userID,
			"locale":                 req.Locale,
			"country":                req.Country,
			"email_notify_order":     req.EmailNotifyOrder,
			"email_notify_ticket":    req.EmailNotifyTicket,
			"email_notify_marketing": req.EmailNotifyMarketing,
			"sms_notify_marketing":   req.SMSNotifyMarketing,
			"source":                 "user_api",
		}
		if user, lookupErr := h.authService.GetUserByID(userID); lookupErr == nil && user != nil {
			afterPayload["locale"] = user.Locale
			afterPayload["country"] = user.Country
			afterPayload["email_notify_order"] = user.EmailNotifyOrder
			afterPayload["email_notify_ticket"] = user.EmailNotifyTicket
			afterPayload["email_notify_marketing"] = user.EmailNotifyMarketing
			afterPayload["sms_notify_marketing"] = user.SMSNotifyMarketing
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "auth.preferences.update.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("auth.preferences.update.after hook execution failed: user=%d err=%v", userID, hookErr)
			}
		}(cloneAuthExecutionContext(hookExecCtx), afterPayload)
	}

	response.Success(c, gin.H{
		"message": "Preferences updated successfully",
	})
}

// GetCaptcha 获取内置验证码
func (h *AuthHandler) GetCaptcha(c *gin.Context) {
	// Basic abuse protection for builtin captcha generation (even when global rate-limit is off).
	// Best-effort: if Redis is unavailable, we fail open to avoid blocking login entirely.
	if cache.RedisClient != nil {
		ip := utils.GetRealIP(c)
		window := int64(60)
		bucket := time.Now().Unix() / window
		key := fmt.Sprintf("captcha:gen:%s:%d", ip, bucket)
		count, err := cache.Incr(key)
		if err == nil {
			if count == 1 {
				_ = cache.Expire(key, time.Duration(window)*time.Second)
			}
			if count > 120 {
				response.Error(c, 429, response.CodeTooManyRequests, "Too many requests, please try again later")
				return
			}
		}
	}

	captchaID, svg, err := h.captchaService.GenerateBuiltinCaptcha()
	if err != nil {
		response.InternalError(c, "Failed to generate captcha")
		return
	}

	response.Success(c, gin.H{
		"captcha_id": captchaID,
		"image":      svg,
	})
}

// generateVerificationToken 生成邮箱验证 token
func generateVerificationToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// VerifyEmail 验证邮箱
func (h *AuthHandler) VerifyEmail(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		response.BadRequest(c, "Verification token is required")
		return
	}

	db := database.GetDB()

	var user models.User
	if err := db.Transaction(func(tx *gorm.DB) error {
		var verifyToken models.EmailVerificationToken
		if err := tx.Where("token = ?", token).First(&verifyToken).Error; err != nil {
			return err
		}

		if !verifyToken.IsValid() {
			return errors.New("TOKEN_INVALID")
		}

		now := time.Now()
		// Conditional update to ensure single-use even under concurrency.
		res := tx.Model(&models.EmailVerificationToken{}).
			Where("id = ? AND used = ? AND expires_at > ?", verifyToken.ID, false, now).
			Updates(map[string]interface{}{"used": true, "used_at": now})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return errors.New("TOKEN_INVALID")
		}

		if err := tx.First(&user, verifyToken.UserID).Error; err != nil {
			return err
		}
		user.EmailVerified = true
		if err := tx.Save(&user).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		if err.Error() == "TOKEN_INVALID" || errors.Is(err, gorm.ErrRecordNotFound) {
			response.BadRequest(c, "Invalid or expired verification token")
			return
		}
		response.InternalError(c, "Email verification failed")
		return
	}

	// 记录日志
	logger.LogOperation(db, c, "verify_email", "user", &user.ID, map[string]interface{}{
		"email": user.Email,
	})

	// 生成 JWT Token 让用户直接登录
	jwtToken, err := h.authService.GenerateToken(&user)
	if err != nil {
		// 验证成功但 token 生成失败，仍然返回成功
		response.Success(c, gin.H{
			"verified": true,
			"message":  "Email verified successfully. Please login.",
		})
		return
	}

	response.Success(c, gin.H{
		"verified":   true,
		"message":    "Email verified successfully",
		"token":      jwtToken,
		"token_type": "Bearer",
		"user": gin.H{
			"user_id":           user.ID,
			"uuid":              user.UUID,
			"email":             user.Email,
			"name":              user.Name,
			"role":              user.Role,
			"avatar":            user.Avatar,
			"locale":            user.Locale,
			"total_spent_minor": user.TotalSpentMinor,
			"total_order_count": user.TotalOrderCount,
		},
	})
}

// ResendVerification 重新发送验证邮件
func (h *AuthHandler) ResendVerification(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	db := database.GetDB()

	// 查找用户
	var user models.User
	if err := db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		// 不暴露用户是否存在
		response.Success(c, gin.H{
			"message": "If the email exists, a verification email has been sent.",
		})
		return
	}

	// 已验证的用户不需要重发
	if user.EmailVerified {
		response.Success(c, gin.H{
			"message": "If the email exists, a verification email has been sent.",
		})
		return
	}

	// 使旧 token 失效
	db.Model(&models.EmailVerificationToken{}).
		Where("user_id = ? AND used = ?", user.ID, false).
		Update("used", true)

	// 生成新 token
	token, err := generateVerificationToken()
	if err != nil {
		response.InternalError(c, "Failed to generate verification token")
		return
	}

	verifyToken := &models.EmailVerificationToken{
		Token:     token,
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	if err := db.Create(verifyToken).Error; err != nil {
		response.InternalError(c, "Failed to create verification token")
		return
	}

	// 发送验证邮件
	if h.emailService != nil {
		go h.emailService.SendVerificationEmail(user.Email, user.Name, token, user.Locale)
	}

	response.Success(c, gin.H{
		"message": "If the email exists, a verification email has been sent.",
	})
}

// SendLoginCodeRequest 发送登录验证码请求
type SendLoginCodeRequest struct {
	Email        string `json:"email" binding:"required,email"`
	CaptchaToken string `json:"captcha_token"`
}

// SendLoginCode 发送邮箱登录验证码
func (h *AuthHandler) SendLoginCode(c *gin.Context) {
	cfg := config.GetConfig()
	if !cfg.SMTP.Enabled {
		respondAuthBizError(c, authbiz.EmailLoginUnavailable(), nil)
		return
	}
	if !cfg.Security.Login.AllowEmailLogin {
		respondAuthBizError(c, authbiz.EmailLoginDisabled(), nil)
		return
	}

	var req SendLoginCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	// 先检查冷却，避免浪费验证码
	ip := utils.GetRealIP(c)
	ipKey := fmt.Sprintf("email_login_cooldown:ip:%s", ip)
	emailKey := fmt.Sprintf("email_login_cooldown:email:%s", req.Email)
	if n, _ := cache.Exists(ipKey); n > 0 {
		response.Error(c, 429, response.CodeCooldown, "Please wait 60 seconds before requesting again")
		return
	}
	if n, _ := cache.Exists(emailKey); n > 0 {
		response.Error(c, 429, response.CodeCooldown, "Please wait 60 seconds before requesting again")
		return
	}

	// 验证码校验
	if h.captchaService.NeedCaptcha("login") {
		if req.CaptchaToken == "" {
			respondAuthBizError(c, authbiz.CaptchaRequired(), nil)
			return
		}
		if err := h.captchaService.VerifyCaptcha(req.CaptchaToken, utils.GetRealIP(c)); err != nil {
			respondAuthBizError(c, authbiz.CaptchaFailed(), nil)
			return
		}
	}

	// 设置冷却
	cache.Set(ipKey, "1", 60*time.Second)
	cache.Set(emailKey, "1", 60*time.Second)

	code, err := h.authService.SendLoginCode(req.Email)
	if err != nil {
		// 不暴露用户是否存在
		response.Success(c, gin.H{"message": "If the email is registered, a verification code has been sent"})
		return
	}

	// 查找用户locale用于邮件语言
	db := database.GetDB()
	var user models.User
	locale := "en"
	if err := db.Select("locale").Where("email = ?", req.Email).First(&user).Error; err == nil && user.Locale != "" {
		locale = user.Locale
	}

	if h.emailService != nil {
		go h.emailService.SendLoginCodeEmail(req.Email, code, locale)
	}

	response.Success(c, gin.H{"message": "If the email is registered, a verification code has been sent"})
}

// ForgotPasswordRequest 忘记密码请求
type ForgotPasswordRequest struct {
	Email        string `json:"email" binding:"required,email"`
	CaptchaToken string `json:"captcha_token"`
}

// ForgotPassword 发送密码重置邮件
func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	cfg := config.GetConfig()
	if !cfg.SMTP.Enabled {
		respondAuthBizError(c, authbiz.EmailLoginUnavailable(), nil)
		return
	}
	if !cfg.Security.Login.AllowPasswordReset {
		respondAuthBizError(c, authbiz.PasswordResetDisabled(), nil)
		return
	}

	var req ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	hookExecCtx := h.buildAuthHookExecutionContext(c, nil)
	if h.pluginManager != nil {
		originalReq := req
		hookPayload := map[string]interface{}{
			"auth_method": "email",
			"email":       req.Email,
			"source":      "user_api",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "auth.password.reset.before",
			Payload: hookPayload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("auth.password.reset.before hook execution failed: email=%s err=%v", req.Email, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Password reset request rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if rawEmail, exists := hookResult.Payload["email"]; exists {
					email, convErr := authHookValueToOptionalString(rawEmail)
					if convErr != nil {
						log.Printf("auth.password.reset.before payload email decode failed, fallback to original request: email=%s err=%v", req.Email, convErr)
						req = originalReq
					} else {
						req.Email = strings.ToLower(strings.TrimSpace(email))
					}
				}
			}
		}
	}

	// 冷却检查
	ip := utils.GetRealIP(c)
	ipKey := fmt.Sprintf("password_reset_cooldown:ip:%s", ip)
	emailKey := fmt.Sprintf("password_reset_cooldown:email:%s", req.Email)
	if n, _ := cache.Exists(ipKey); n > 0 {
		response.Error(c, 429, response.CodeCooldown, "Please wait 60 seconds before requesting again")
		return
	}
	if n, _ := cache.Exists(emailKey); n > 0 {
		response.Error(c, 429, response.CodeCooldown, "Please wait 60 seconds before requesting again")
		return
	}

	// 验证码校验
	if h.captchaService.NeedCaptcha("login") {
		if req.CaptchaToken == "" {
			respondAuthBizError(c, authbiz.CaptchaRequired(), nil)
			return
		}
		if err := h.captchaService.VerifyCaptcha(req.CaptchaToken, ip); err != nil {
			respondAuthBizError(c, authbiz.CaptchaFailed(), nil)
			return
		}
	}

	// 设置冷却
	cache.Set(ipKey, "1", 60*time.Second)
	cache.Set(emailKey, "1", 60*time.Second)

	// 生成token并发送邮件（不暴露用户是否存在）
	token, err := h.authService.GeneratePasswordResetToken(req.Email)
	if err == nil && h.emailService != nil {
		// 查找用户locale
		db := database.GetDB()
		var user models.User
		locale := "en"
		if err := db.Select("locale").Where("email = ?", req.Email).First(&user).Error; err == nil && user.Locale != "" {
			locale = user.Locale
		}
		go h.emailService.SendPasswordResetEmail(req.Email, token, locale)
	}

	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"auth_method": "email",
			"email":       req.Email,
			"accepted":    true,
			"source":      "user_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, email string) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "auth.password.reset.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("auth.password.reset.after hook execution failed: email=%s err=%v", email, hookErr)
			}
		}(cloneAuthExecutionContext(hookExecCtx), afterPayload, req.Email)
	}

	response.Success(c, gin.H{"message": "If the email is registered, a password reset link has been sent"})
}

// ResetPasswordRequest 重置密码请求
type ResetPasswordRequest struct {
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

// ResetPassword 使用token重置密码
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}
	hookExecCtx := h.buildAuthHookExecutionContext(c, nil)
	if h.pluginManager != nil {
		hookPayload := map[string]interface{}{
			"auth_method":          "email",
			"token_present":        strings.TrimSpace(req.Token) != "",
			"new_password_present": strings.TrimSpace(req.NewPassword) != "",
			"source":               "user_api",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "auth.password.reset.before",
			Payload: hookPayload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("auth.password.reset.before hook execution failed: err=%v", hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Password reset rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
		}
	}

	if err := h.authService.ResetPassword(req.Token, req.NewPassword); err != nil {
		if respondAuthBizError(c, err, nil) {
			return
		}
		respondAuthValidationOrInternalError(c, err, "Failed to reset password")
		return
	}

	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"auth_method": "email",
			"success":     true,
			"source":      "user_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "auth.password.reset.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("auth.password.reset.after hook execution failed: err=%v", hookErr)
			}
		}(cloneAuthExecutionContext(hookExecCtx), afterPayload)
	}

	response.Success(c, gin.H{"message": "Password reset successfully"})
}

// LoginWithCodeRequest 验证码登录请求
type LoginWithCodeRequest struct {
	Email string `json:"email" binding:"required,email"`
	Code  string `json:"code" binding:"required,len=6"`
}

// LoginWithCode 使用邮箱验证码登录
func (h *AuthHandler) LoginWithCode(c *gin.Context) {
	var req LoginWithCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	hookExecCtx := h.buildAuthHookExecutionContext(c, nil)
	if h.pluginManager != nil {
		originalReq := req
		hookPayload := map[string]interface{}{
			"auth_method":  "email_code",
			"email":        req.Email,
			"code_present": strings.TrimSpace(req.Code) != "",
			"source":       "user_api",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "auth.login.before",
			Payload: hookPayload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("auth.login.before hook execution failed: email=%s method=email_code err=%v", req.Email, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Login rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if rawEmail, exists := hookResult.Payload["email"]; exists {
					email, convErr := authHookValueToOptionalString(rawEmail)
					if convErr != nil {
						log.Printf("auth.login.before payload email decode failed, fallback to original request: email=%s err=%v", req.Email, convErr)
						req = originalReq
					} else {
						req.Email = strings.ToLower(strings.TrimSpace(email))
					}
				}
			}
		}
	}

	token, user, err := h.authService.LoginWithCode(req.Email, req.Code)
	if err != nil {
		db := database.GetDB()
		logger.LogLoginAttempt(db, c, req.Email, false, nil)
		if respondAuthBizError(c, err, nil) {
			return
		}
		response.InternalServerError(c, "Login failed", err)
		return
	}

	user.LastLoginIP = utils.GetRealIP(c)
	h.authService.UpdateLoginIP(user)

	db := database.GetDB()
	logger.LogLoginAttempt(db, c, req.Email, true, &user.ID)

	result := gin.H{
		"id":                user.ID,
		"user_id":           user.ID,
		"uuid":              user.UUID,
		"email":             user.Email,
		"name":              user.Name,
		"role":              user.Role,
		"avatar":            user.Avatar,
		"locale":            user.Locale,
		"total_spent_minor": user.TotalSpentMinor,
		"total_order_count": user.TotalOrderCount,
	}

	if user.IsAdmin() {
		result["permissions"] = loadEffectiveAdminPermissions(db, user.ID, user.Role)
	}

	if h.pluginManager != nil {
		uid := user.ID
		afterPayload := map[string]interface{}{
			"auth_method": "email_code",
			"user_id":     user.ID,
			"email":       user.Email,
			"role":        user.Role,
			"source":      "user_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, email string) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "auth.login.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("auth.login.after hook execution failed: email=%s method=email_code err=%v", email, hookErr)
			}
		}(cloneAuthExecutionContext(h.buildAuthHookExecutionContext(c, &uid)), afterPayload, user.Email)
	}

	response.Success(c, gin.H{
		"token":      token,
		"token_type": "Bearer",
		"user":       result,
	})
}

// SendPhoneLoginCode 发送手机登录验证码
func (h *AuthHandler) SendPhoneLoginCode(c *gin.Context) {
	cfg := config.GetConfig()
	if !cfg.SMS.Enabled {
		respondAuthBizError(c, authbiz.SMSServiceUnavailable(), nil)
		return
	}
	if !cfg.Security.Login.AllowPhoneLogin {
		respondAuthBizError(c, authbiz.PhoneLoginDisabled(), nil)
		return
	}
	var req struct {
		Phone        string `json:"phone" binding:"required"`
		PhoneCode    string `json:"phone_code"`
		CaptchaToken string `json:"captcha_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}
	req.Phone = strings.TrimSpace(req.Phone)
	req.PhoneCode = strings.TrimSpace(req.PhoneCode)
	if !validatePhone(req.Phone) {
		respondAuthBizError(c, authbiz.InvalidPhoneFormat(), nil)
		return
	}

	ip := utils.GetRealIP(c)
	ipKey := fmt.Sprintf("phone_login_cooldown:ip:%s", ip)
	phoneKey := fmt.Sprintf("phone_login_cooldown:phone:%s", req.Phone)
	if n, _ := cache.Exists(ipKey); n > 0 {
		response.Error(c, 429, response.CodeCooldown, "Please wait 60 seconds before requesting again")
		return
	}
	if n, _ := cache.Exists(phoneKey); n > 0 {
		response.Error(c, 429, response.CodeCooldown, "Please wait 60 seconds before requesting again")
		return
	}
	if h.captchaService.NeedCaptcha("login") {
		if req.CaptchaToken == "" {
			respondAuthBizError(c, authbiz.CaptchaRequired(), nil)
			return
		}
		if err := h.captchaService.VerifyCaptcha(req.CaptchaToken, ip); err != nil {
			respondAuthBizError(c, authbiz.CaptchaFailed(), nil)
			return
		}
	}
	cache.Set(ipKey, "1", 60*time.Second)
	cache.Set(phoneKey, "1", 60*time.Second)

	code, err := h.authService.SendPhoneLoginCode(req.Phone)
	if err != nil {
		response.Success(c, gin.H{"message": "If the phone is registered, a verification code has been sent"})
		return
	}
	if h.smsService != nil {
		go h.smsService.SendVerificationCode(req.Phone, req.PhoneCode, code, "login")
	}
	response.Success(c, gin.H{"message": "If the phone is registered, a verification code has been sent"})
}

// LoginWithPhoneCode 使用手机验证码登录
func (h *AuthHandler) LoginWithPhoneCode(c *gin.Context) {
	var req struct {
		Phone     string `json:"phone" binding:"required"`
		PhoneCode string `json:"phone_code"`
		Code      string `json:"code" binding:"required,len=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}
	phone := strings.TrimSpace(req.Phone)
	if !validatePhone(phone) {
		respondAuthBizError(c, authbiz.InvalidPhoneFormat(), nil)
		return
	}
	hookExecCtx := h.buildAuthHookExecutionContext(c, nil)
	if h.pluginManager != nil {
		hookPayload := map[string]interface{}{
			"auth_method":  "phone_code",
			"phone":        phone,
			"code_present": strings.TrimSpace(req.Code) != "",
			"source":       "user_api",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "auth.login.before",
			Payload: hookPayload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("auth.login.before hook execution failed: phone=%s method=phone_code err=%v", phone, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Login rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
		}
	}
	token, user, err := h.authService.LoginWithPhoneCode(phone, req.Code)
	if err != nil {
		db := database.GetDB()
		logger.LogLoginAttempt(db, c, phone, false, nil)
		if respondAuthBizError(c, err, nil) {
			return
		}
		response.InternalServerError(c, "Login failed", err)
		return
	}
	user.LastLoginIP = utils.GetRealIP(c)
	h.authService.UpdateLoginIP(user)

	db := database.GetDB()
	logger.LogLoginAttempt(db, c, phone, true, &user.ID)

	if h.pluginManager != nil {
		uid := user.ID
		afterPayload := map[string]interface{}{
			"auth_method": "phone_code",
			"user_id":     user.ID,
			"phone":       phone,
			"role":        user.Role,
			"source":      "user_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, p string) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "auth.login.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("auth.login.after hook execution failed: phone=%s method=phone_code err=%v", p, hookErr)
			}
		}(cloneAuthExecutionContext(h.buildAuthHookExecutionContext(c, &uid)), afterPayload, phone)
	}

	response.Success(c, gin.H{
		"token": token, "token_type": "Bearer",
		"user": gin.H{
			"id":                user.ID,
			"user_id":           user.ID,
			"uuid":              user.UUID,
			"email":             user.Email,
			"name":              user.Name,
			"role":              user.Role,
			"avatar":            user.Avatar,
			"locale":            user.Locale,
			"total_spent_minor": user.TotalSpentMinor,
			"total_order_count": user.TotalOrderCount,
		},
	})
}

// PhoneRegister 手机号注册
func (h *AuthHandler) PhoneRegister(c *gin.Context) {
	cfg := config.GetConfig()
	if !cfg.Security.Login.AllowRegistration {
		respondAuthBizError(c, authbiz.RegistrationDisabled(), nil)
		return
	}
	if !cfg.SMS.Enabled || !cfg.Security.Login.AllowPhoneRegister {
		respondAuthBizError(c, authbiz.PhoneRegistrationDisabled(), nil)
		return
	}
	var req struct {
		Phone        string `json:"phone" binding:"required"`
		PhoneCode    string `json:"phone_code"`
		Name         string `json:"name" binding:"required,min=2,max=100"`
		Password     string `json:"password" binding:"required,min=8"`
		Code         string `json:"code" binding:"required,len=6"`
		CaptchaToken string `json:"captcha_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}
	req.Phone = strings.TrimSpace(req.Phone)
	req.Name = strings.TrimSpace(req.Name)
	if !validatePhone(req.Phone) {
		respondAuthBizError(c, authbiz.InvalidPhoneFormat(), nil)
		return
	}
	hookExecCtx := h.buildAuthHookExecutionContext(c, nil)
	if h.pluginManager != nil {
		originalReq := req
		hookPayload := map[string]interface{}{
			"auth_method":      "phone",
			"phone":            req.Phone,
			"name":             req.Name,
			"password_present": strings.TrimSpace(req.Password) != "",
			"source":           "user_api",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "auth.register.before",
			Payload: hookPayload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("auth.register.before hook execution failed: phone=%s err=%v", req.Phone, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Registration rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if rawPhone, exists := hookResult.Payload["phone"]; exists {
					phone, convErr := authHookValueToOptionalString(rawPhone)
					if convErr != nil {
						log.Printf("auth.register.before payload phone decode failed, fallback to original request: phone=%s err=%v", req.Phone, convErr)
						req = originalReq
					} else {
						req.Phone = strings.TrimSpace(phone)
					}
				}
				if rawName, exists := hookResult.Payload["name"]; exists {
					name, convErr := authHookValueToOptionalString(rawName)
					if convErr != nil {
						log.Printf("auth.register.before payload name decode failed, fallback to original request: phone=%s err=%v", req.Phone, convErr)
						req = originalReq
					} else {
						req.Name = strings.TrimSpace(name)
					}
				}
			}
		}
	}
	if !validatePhone(req.Phone) {
		respondAuthBizError(c, authbiz.InvalidPhoneFormat(), nil)
		return
	}

	// Verify SMS code
	key := "phone_register_code:" + req.Phone
	storedCode, err := cache.Get(key)
	if err != nil || storedCode != req.Code {
		respondAuthBizError(c, authbiz.CodeExpired(), nil)
		return
	}
	_ = cache.Del(key)

	user, err := h.authService.Register("", req.Phone, req.Name, req.Password)
	if err != nil {
		switch {
		case respondAuthBizError(c, err, nil):
		case errors.Is(err, service.ErrRegisterInternal):
			response.InternalError(c, "Registration failed")
		default:
			respondAuthValidationOrInternalError(c, err, "Registration failed")
		}
		return
	}

	user.EmailVerified = true
	user.RegisterIP = utils.GetRealIP(c)
	db := database.GetDB()
	db.Save(user)

	// 记录注册日志
	logger.LogOperation(db, c, "register", "user", &user.ID, map[string]interface{}{
		"phone":       req.Phone,
		"name":        user.Name,
		"register_ip": user.RegisterIP,
	})

	jwtToken, err := h.authService.GenerateToken(user)
	if err != nil {
		response.InternalError(c, "Failed to generate token")
		return
	}

	if h.pluginManager != nil {
		uid := user.ID
		afterPayload := map[string]interface{}{
			"auth_method":          "phone",
			"user_id":              user.ID,
			"phone":                req.Phone,
			"name":                 user.Name,
			"require_verification": false,
			"email_verified":       user.EmailVerified,
			"source":               "user_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, phone string) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "auth.register.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("auth.register.after hook execution failed: phone=%s err=%v", phone, hookErr)
			}
		}(cloneAuthExecutionContext(h.buildAuthHookExecutionContext(c, &uid)), afterPayload, req.Phone)
	}

	response.Success(c, gin.H{
		"token": jwtToken, "token_type": "Bearer",
		"user": gin.H{
			"id":                user.ID,
			"user_id":           user.ID,
			"uuid":              user.UUID,
			"email":             user.Email,
			"name":              user.Name,
			"role":              user.Role,
			"avatar":            user.Avatar,
			"locale":            user.Locale,
			"total_spent_minor": user.TotalSpentMinor,
			"total_order_count": user.TotalOrderCount,
		},
	})
}

// PhoneForgotPassword 手机号找回密码
func (h *AuthHandler) PhoneForgotPassword(c *gin.Context) {
	cfg := config.GetConfig()
	if !cfg.SMS.Enabled || !cfg.Security.Login.AllowPhonePasswordReset {
		respondAuthBizError(c, authbiz.PhonePasswordResetDisabled(), nil)
		return
	}
	var req struct {
		Phone        string `json:"phone" binding:"required"`
		PhoneCode    string `json:"phone_code"`
		CaptchaToken string `json:"captcha_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}
	req.Phone = strings.TrimSpace(req.Phone)
	req.PhoneCode = strings.TrimSpace(req.PhoneCode)
	if !validatePhone(req.Phone) {
		respondAuthBizError(c, authbiz.InvalidPhoneFormat(), nil)
		return
	}
	hookExecCtx := h.buildAuthHookExecutionContext(c, nil)
	if h.pluginManager != nil {
		originalReq := req
		hookPayload := map[string]interface{}{
			"auth_method": "phone",
			"phone":       req.Phone,
			"source":      "user_api",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "auth.password.reset.before",
			Payload: hookPayload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("auth.password.reset.before hook execution failed: phone=%s err=%v", req.Phone, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Password reset request rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if rawPhone, exists := hookResult.Payload["phone"]; exists {
					phone, convErr := authHookValueToOptionalString(rawPhone)
					if convErr != nil {
						log.Printf("auth.password.reset.before payload phone decode failed, fallback to original request: phone=%s err=%v", req.Phone, convErr)
						req = originalReq
					} else {
						req.Phone = strings.TrimSpace(phone)
					}
				}
			}
		}
	}
	if !validatePhone(req.Phone) {
		respondAuthBizError(c, authbiz.InvalidPhoneFormat(), nil)
		return
	}
	ip := utils.GetRealIP(c)
	ipKey := fmt.Sprintf("phone_reset_cooldown:ip:%s", ip)
	phoneKey := fmt.Sprintf("phone_reset_cooldown:phone:%s", req.Phone)
	if n, _ := cache.Exists(ipKey); n > 0 {
		response.Error(c, 429, response.CodeCooldown, "Please wait 60 seconds before requesting again")
		return
	}
	if n, _ := cache.Exists(phoneKey); n > 0 {
		response.Error(c, 429, response.CodeCooldown, "Please wait 60 seconds before requesting again")
		return
	}
	if h.captchaService.NeedCaptcha("login") {
		if req.CaptchaToken == "" {
			respondAuthBizError(c, authbiz.CaptchaRequired(), nil)
			return
		}
		if err := h.captchaService.VerifyCaptcha(req.CaptchaToken, ip); err != nil {
			respondAuthBizError(c, authbiz.CaptchaFailed(), nil)
			return
		}
	}
	cache.Set(ipKey, "1", 60*time.Second)
	cache.Set(phoneKey, "1", 60*time.Second)
	code, err := h.authService.GeneratePhoneResetCode(req.Phone)
	if err == nil && h.smsService != nil {
		go h.smsService.SendVerificationCode(req.Phone, req.PhoneCode, code, "reset_password")
	}
	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"auth_method": "phone",
			"phone":       req.Phone,
			"accepted":    true,
			"source":      "user_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, phone string) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "auth.password.reset.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("auth.password.reset.after hook execution failed: phone=%s err=%v", phone, hookErr)
			}
		}(cloneAuthExecutionContext(hookExecCtx), afterPayload, req.Phone)
	}
	response.Success(c, gin.H{"message": "If the phone is registered, a verification code has been sent"})
}

// PhoneResetPassword 使用手机验证码重置密码
func (h *AuthHandler) PhoneResetPassword(c *gin.Context) {
	var req struct {
		Phone       string `json:"phone" binding:"required"`
		PhoneCode   string `json:"phone_code"`
		Code        string `json:"code" binding:"required,len=6"`
		NewPassword string `json:"new_password" binding:"required,min=8"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}
	phone := strings.TrimSpace(req.Phone)
	if !validatePhone(phone) {
		respondAuthBizError(c, authbiz.InvalidPhoneFormat(), nil)
		return
	}
	hookExecCtx := h.buildAuthHookExecutionContext(c, nil)
	if h.pluginManager != nil {
		hookPayload := map[string]interface{}{
			"auth_method":          "phone",
			"phone":                phone,
			"code_present":         strings.TrimSpace(req.Code) != "",
			"new_password_present": strings.TrimSpace(req.NewPassword) != "",
			"source":               "user_api",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "auth.password.reset.before",
			Payload: hookPayload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("auth.password.reset.before hook execution failed: phone=%s err=%v", phone, hookErr)
		} else if hookResult != nil && hookResult.Blocked {
			reason := strings.TrimSpace(hookResult.BlockReason)
			if reason == "" {
				reason = "Password reset rejected by plugin"
			}
			response.BadRequest(c, reason)
			return
		}
	}
	if err := h.authService.ResetPasswordByPhone(phone, req.Code, req.NewPassword); err != nil {
		if respondAuthBizError(c, err, nil) {
			return
		}
		respondAuthValidationOrInternalError(c, err, "Failed to reset password")
		return
	}
	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"auth_method": "phone",
			"phone":       phone,
			"success":     true,
			"source":      "user_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, p string) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "auth.password.reset.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("auth.password.reset.after hook execution failed: phone=%s err=%v", p, hookErr)
			}
		}(cloneAuthExecutionContext(hookExecCtx), afterPayload, phone)
	}
	response.Success(c, gin.H{"message": "Password reset successfully"})
}

// SendBindEmailCode 发送绑定邮箱验证码
func (h *AuthHandler) SendBindEmailCode(c *gin.Context) {
	userID := middleware.MustGetUserID(c)
	var req struct {
		Email        string `json:"email" binding:"required,email"`
		CaptchaToken string `json:"captcha_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	hookExecCtx := h.buildAuthHookExecutionContext(c, &userID)
	if h.pluginManager != nil {
		originalReq := req
		hookPayload := map[string]interface{}{
			"user_id": userID,
			"email":   req.Email,
			"source":  "user_api",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "auth.bind_email.request.before",
			Payload: hookPayload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("auth.bind_email.request.before hook execution failed: user=%d email=%s err=%v", userID, req.Email, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Bind email request rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if value, exists := hookResult.Payload["email"]; exists {
					text, convErr := authHookValueToOptionalString(value)
					if convErr != nil {
						log.Printf("auth.bind_email.request.before payload email decode failed, fallback to original request: user=%d email=%s err=%v", userID, req.Email, convErr)
						req = originalReq
					} else {
						req.Email = strings.ToLower(strings.TrimSpace(text))
					}
				}
			}
		}
	}

	if h.captchaService.NeedCaptcha("bind") {
		if req.CaptchaToken == "" {
			respondAuthBizError(c, authbiz.CaptchaRequired(), nil)
			return
		}
		if err := h.captchaService.VerifyCaptcha(req.CaptchaToken, utils.GetRealIP(c)); err != nil {
			respondAuthBizError(c, authbiz.CaptchaFailed(), nil)
			return
		}
	}

	ip := utils.GetRealIP(c)
	ipKey := fmt.Sprintf("bind_email_cooldown:ip:%s", ip)
	emailKey := fmt.Sprintf("bind_email_cooldown:email:%s", req.Email)
	if n, _ := cache.Exists(ipKey); n > 0 {
		response.Error(c, 429, response.CodeCooldown, "Please wait 60 seconds before requesting again")
		return
	}
	if n, _ := cache.Exists(emailKey); n > 0 {
		response.Error(c, 429, response.CodeCooldown, "Please wait 60 seconds before requesting again")
		return
	}
	cache.Set(ipKey, "1", 60*time.Second)
	cache.Set(emailKey, "1", 60*time.Second)

	code, err := h.authService.SendBindEmailCode(userID, req.Email)
	if err != nil {
		if respondAuthBizError(c, err, nil) {
			return
		}
		response.InternalServerError(c, "Failed to send verification code", err)
		return
	}

	if h.emailService != nil {
		user, _ := h.authService.GetUserByID(userID)
		locale := "en"
		if user != nil && user.Locale != "" {
			locale = user.Locale
		}
		go h.emailService.SendLoginCodeEmail(req.Email, code, locale)
	}
	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"user_id":  userID,
			"email":    req.Email,
			"accepted": true,
			"source":   "user_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, email string) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "auth.bind_email.request.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("auth.bind_email.request.after hook execution failed: user=%d email=%s err=%v", userID, email, hookErr)
			}
		}(cloneAuthExecutionContext(hookExecCtx), afterPayload, req.Email)
	}
	response.Success(c, gin.H{"message": "Verification code sent"})
}

// BindEmail 绑定邮箱
func (h *AuthHandler) BindEmail(c *gin.Context) {
	userID := middleware.MustGetUserID(c)
	var req struct {
		Email string `json:"email" binding:"required,email"`
		Code  string `json:"code" binding:"required,len=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	hookExecCtx := h.buildAuthHookExecutionContext(c, &userID)
	if h.pluginManager != nil {
		originalReq := req
		hookPayload := map[string]interface{}{
			"user_id":      userID,
			"email":        req.Email,
			"code_present": strings.TrimSpace(req.Code) != "",
			"source":       "user_api",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "auth.bind_email.confirm.before",
			Payload: hookPayload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("auth.bind_email.confirm.before hook execution failed: user=%d email=%s err=%v", userID, req.Email, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Bind email confirmation rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if value, exists := hookResult.Payload["email"]; exists {
					text, convErr := authHookValueToOptionalString(value)
					if convErr != nil {
						log.Printf("auth.bind_email.confirm.before payload email decode failed, fallback to original request: user=%d email=%s err=%v", userID, req.Email, convErr)
						req = originalReq
					} else {
						req.Email = strings.ToLower(strings.TrimSpace(text))
					}
				}
			}
		}
	}
	if err := h.authService.BindEmail(userID, req.Email, req.Code); err != nil {
		if respondAuthBizError(c, err, nil) {
			return
		}
		response.InternalServerError(c, "Failed to bind email", err)
		return
	}
	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"user_id": userID,
			"email":   req.Email,
			"source":  "user_api",
			"success": true,
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, email string) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "auth.bind_email.confirm.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("auth.bind_email.confirm.after hook execution failed: user=%d email=%s err=%v", userID, email, hookErr)
			}
		}(cloneAuthExecutionContext(hookExecCtx), afterPayload, req.Email)
	}
	response.Success(c, gin.H{"message": "Email bound successfully"})
}

// SendBindPhoneCode 发送绑定手机验证码
func (h *AuthHandler) SendBindPhoneCode(c *gin.Context) {
	userID := middleware.MustGetUserID(c)
	var req struct {
		Phone        string `json:"phone" binding:"required"`
		PhoneCode    string `json:"phone_code"`
		CaptchaToken string `json:"captcha_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}
	req.Phone = strings.TrimSpace(req.Phone)
	req.PhoneCode = strings.TrimSpace(req.PhoneCode)
	hookExecCtx := h.buildAuthHookExecutionContext(c, &userID)
	if h.pluginManager != nil {
		originalReq := req
		hookPayload := map[string]interface{}{
			"user_id":    userID,
			"phone":      req.Phone,
			"phone_code": req.PhoneCode,
			"source":     "user_api",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "auth.bind_phone.request.before",
			Payload: hookPayload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("auth.bind_phone.request.before hook execution failed: user=%d phone=%s err=%v", userID, req.Phone, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Bind phone request rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if value, exists := hookResult.Payload["phone"]; exists {
					text, convErr := authHookValueToOptionalString(value)
					if convErr != nil {
						log.Printf("auth.bind_phone.request.before payload phone decode failed, fallback to original request: user=%d phone=%s err=%v", userID, req.Phone, convErr)
						req = originalReq
					} else {
						req.Phone = strings.TrimSpace(text)
					}
				}
				if value, exists := hookResult.Payload["phone_code"]; exists {
					text, convErr := authHookValueToOptionalString(value)
					if convErr != nil {
						log.Printf("auth.bind_phone.request.before payload phone_code decode failed, fallback to original request: user=%d phone=%s err=%v", userID, req.Phone, convErr)
						req = originalReq
					} else {
						req.PhoneCode = strings.TrimSpace(text)
					}
				}
			}
		}
	}
	if !validatePhone(req.Phone) {
		respondAuthBizError(c, authbiz.InvalidPhoneFormat(), nil)
		return
	}

	if h.captchaService.NeedCaptcha("bind") {
		if req.CaptchaToken == "" {
			respondAuthBizError(c, authbiz.CaptchaRequired(), nil)
			return
		}
		if err := h.captchaService.VerifyCaptcha(req.CaptchaToken, utils.GetRealIP(c)); err != nil {
			respondAuthBizError(c, authbiz.CaptchaFailed(), nil)
			return
		}
	}

	ip := utils.GetRealIP(c)
	ipKey := fmt.Sprintf("bind_phone_cooldown:ip:%s", ip)
	phoneKey := fmt.Sprintf("bind_phone_cooldown:phone:%s", req.Phone)
	if n, _ := cache.Exists(ipKey); n > 0 {
		response.Error(c, 429, response.CodeCooldown, "Please wait 60 seconds before requesting again")
		return
	}
	if n, _ := cache.Exists(phoneKey); n > 0 {
		response.Error(c, 429, response.CodeCooldown, "Please wait 60 seconds before requesting again")
		return
	}
	cache.Set(ipKey, "1", 60*time.Second)
	cache.Set(phoneKey, "1", 60*time.Second)

	code, err := h.authService.SendBindPhoneCode(userID, req.Phone)
	if err != nil {
		if respondAuthBizError(c, err, nil) {
			return
		}
		response.InternalServerError(c, "Failed to send verification code", err)
		return
	}
	if h.smsService != nil {
		go h.smsService.SendVerificationCode(req.Phone, req.PhoneCode, code, "bind_phone")
	}
	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"user_id":    userID,
			"phone":      req.Phone,
			"phone_code": req.PhoneCode,
			"accepted":   true,
			"source":     "user_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, phone string) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "auth.bind_phone.request.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("auth.bind_phone.request.after hook execution failed: user=%d phone=%s err=%v", userID, phone, hookErr)
			}
		}(cloneAuthExecutionContext(hookExecCtx), afterPayload, req.Phone)
	}
	response.Success(c, gin.H{"message": "Verification code sent"})
}

// BindPhone 绑定手机号
func (h *AuthHandler) BindPhone(c *gin.Context) {
	userID := middleware.MustGetUserID(c)
	var req struct {
		Phone string `json:"phone" binding:"required"`
		Code  string `json:"code" binding:"required,len=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}
	req.Phone = strings.TrimSpace(req.Phone)
	hookExecCtx := h.buildAuthHookExecutionContext(c, &userID)
	if h.pluginManager != nil {
		originalReq := req
		hookPayload := map[string]interface{}{
			"user_id":      userID,
			"phone":        req.Phone,
			"code_present": strings.TrimSpace(req.Code) != "",
			"source":       "user_api",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "auth.bind_phone.confirm.before",
			Payload: hookPayload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("auth.bind_phone.confirm.before hook execution failed: user=%d phone=%s err=%v", userID, req.Phone, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Bind phone confirmation rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if value, exists := hookResult.Payload["phone"]; exists {
					text, convErr := authHookValueToOptionalString(value)
					if convErr != nil {
						log.Printf("auth.bind_phone.confirm.before payload phone decode failed, fallback to original request: user=%d phone=%s err=%v", userID, req.Phone, convErr)
						req = originalReq
					} else {
						req.Phone = strings.TrimSpace(text)
					}
				}
			}
		}
	}
	phone := strings.TrimSpace(req.Phone)
	if !validatePhone(phone) {
		respondAuthBizError(c, authbiz.InvalidPhoneFormat(), nil)
		return
	}
	if err := h.authService.BindPhone(userID, phone, req.Code); err != nil {
		if respondAuthBizError(c, err, nil) {
			return
		}
		response.InternalServerError(c, "Failed to bind phone", err)
		return
	}
	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"user_id": userID,
			"phone":   phone,
			"source":  "user_api",
			"success": true,
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, boundPhone string) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "auth.bind_phone.confirm.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("auth.bind_phone.confirm.after hook execution failed: user=%d phone=%s err=%v", userID, boundPhone, hookErr)
			}
		}(cloneAuthExecutionContext(hookExecCtx), afterPayload, phone)
	}
	response.Success(c, gin.H{"message": "Phone bound successfully"})
}

// SendPhoneRegisterCode 发送手机注册验证码
func (h *AuthHandler) SendPhoneRegisterCode(c *gin.Context) {
	cfg := config.GetConfig()
	if !cfg.SMS.Enabled || !cfg.Security.Login.AllowPhoneRegister {
		respondAuthBizError(c, authbiz.PhoneRegistrationDisabled(), nil)
		return
	}
	var req struct {
		Phone        string `json:"phone" binding:"required"`
		PhoneCode    string `json:"phone_code"`
		CaptchaToken string `json:"captcha_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}
	req.Phone = strings.TrimSpace(req.Phone)
	req.PhoneCode = strings.TrimSpace(req.PhoneCode)
	if !validatePhone(req.Phone) {
		respondAuthBizError(c, authbiz.InvalidPhoneFormat(), nil)
		return
	}

	ip := utils.GetRealIP(c)
	ipKey := fmt.Sprintf("phone_register_cooldown:ip:%s", ip)
	phoneKey := fmt.Sprintf("phone_register_cooldown:phone:%s", req.Phone)
	if n, _ := cache.Exists(ipKey); n > 0 {
		response.Error(c, 429, response.CodeCooldown, "Please wait 60 seconds before requesting again")
		return
	}
	if n, _ := cache.Exists(phoneKey); n > 0 {
		response.Error(c, 429, response.CodeCooldown, "Please wait 60 seconds before requesting again")
		return
	}
	if h.captchaService.NeedCaptcha("register") {
		if req.CaptchaToken == "" {
			respondAuthBizError(c, authbiz.CaptchaRequired(), nil)
			return
		}
		if err := h.captchaService.VerifyCaptcha(req.CaptchaToken, ip); err != nil {
			respondAuthBizError(c, authbiz.CaptchaFailed(), nil)
			return
		}
	}
	cache.Set(ipKey, "1", 60*time.Second)
	cache.Set(phoneKey, "1", 60*time.Second)

	code, err := h.authService.SendPhoneRegisterCode(req.Phone)
	if err != nil {
		response.Success(c, gin.H{"message": "Verification code sent"})
		return
	}
	if h.smsService != nil {
		go h.smsService.SendVerificationCode(req.Phone, req.PhoneCode, code, "register")
	}
	response.Success(c, gin.H{"message": "Verification code sent"})
}
