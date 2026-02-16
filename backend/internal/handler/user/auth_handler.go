package user

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"auralogic/internal/config"
	"auralogic/internal/database"
	"auralogic/internal/middleware"
	"auralogic/internal/models"
	"auralogic/internal/pkg/cache"
	"auralogic/internal/pkg/logger"
	"auralogic/internal/pkg/response"
	"auralogic/internal/pkg/utils"
	"auralogic/internal/service"
	"gorm.io/gorm"
)

type AuthHandler struct {
	authService    *service.AuthService
	emailService   *service.EmailService
	captchaService *service.CaptchaService
}

func NewAuthHandler(authService *service.AuthService, emailService *service.EmailService) *AuthHandler {
	return &AuthHandler{
		authService:    authService,
		emailService:   emailService,
		captchaService: service.NewCaptchaService(),
	}
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
			response.Error(c, 400, response.CodeParamMissing, "Captcha is required")
			return
		}
		if err := h.captchaService.VerifyCaptcha(req.CaptchaToken, utils.GetRealIP(c)); err != nil {
			response.Error(c, 400, response.CodeParamError, "Captcha verification failed")
			return
		}
	}

	token, user, err := h.authService.Login(req.Email, req.Password)
	if err != nil {
		db := database.GetDB()
		logger.LogLoginAttempt(db, c, req.Email, false, nil)
		if err.Error() == "EMAIL_NOT_VERIFIED" {
			response.ErrorWithData(c, 403, response.CodeEmailNotVerified, "Please verify your email before logging in", gin.H{
				"email": req.Email,
			})
			return
		}
		if err.Error() == "Password login is disabled, please use quick login or OAuth login" {
			response.ErrorWithData(c, 403, response.CodePasswordDisabled, err.Error(), gin.H{
				"allowed_methods": []string{"magic_link", "oauth"},
			})
			return
		}
		response.Unauthorized(c, err.Error())
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
		"user_id": user.ID,
		"uuid":    user.UUID,
		"email":   user.Email,
		"name":    user.Name,
		"role":    user.Role,
		"avatar":  user.Avatar,
		"locale":  user.Locale,
	}

	// 如果是Admin，getPermission列表
	if user.IsAdmin() {
		db := database.GetDB()
		var perm models.AdminPermission
		if err := db.Where("user_id = ?", user.ID).First(&perm).Error; err == nil {
			result["permissions"] = perm.Permissions
		} else if err == gorm.ErrRecordNotFound {
			// 超级Admin默认拥有所有Permission（除了特殊Permission）
			if user.IsSuperAdmin() {
				result["permissions"] = getAllPermissions()
			} else {
				result["permissions"] = []string{}
			}
		} else {
			result["permissions"] = []string{}
		}
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
		response.ErrorWithData(c, 403, response.CodeForbidden, "Registration is disabled", nil)
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
			response.Error(c, 400, response.CodeParamMissing, "Captcha is required")
			return
		}
		if err := h.captchaService.VerifyCaptcha(req.CaptchaToken, utils.GetRealIP(c)); err != nil {
			response.Error(c, 400, response.CodeParamError, "Captcha verification failed")
			return
		}
	}

	user, err := h.authService.Register(req.Email, "", req.Name, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrEmailAlreadyInUse), errors.Is(err, service.ErrPhoneAlreadyInUse):
			response.Error(c, 409, response.CodeConflict, err.Error())
		case errors.Is(err, service.ErrRegisterInternal):
			response.InternalError(c, "Registration failed")
		default:
			// Password policy / validation errors are safe to show.
			response.BadRequest(c, err.Error())
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

	response.Success(c, gin.H{
		"token":      jwtToken,
		"token_type": "Bearer",
		"user": gin.H{
			"user_id": user.ID,
			"uuid":    user.UUID,
			"email":   user.Email,
			"name":    user.Name,
			"role":    user.Role,
			"avatar":  user.Avatar,
			"locale":  user.Locale,
		},
	})
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
		"user_id":       user.ID,
		"uuid":          user.UUID,
		"email":         user.Email,
		"name":          user.Name,
		"role":          user.Role,
		"avatar":        user.Avatar,
		"is_active":     user.IsActive,
		"locale":        user.Locale,
		"last_login_ip": user.LastLoginIP,
		"country":       user.Country,
		"created_at":    user.CreatedAt,
	}

	// 如果是Admin，getPermission列表
	if user.IsAdmin() {
		db := database.GetDB()
		var perm models.AdminPermission
		if err := db.Where("user_id = ?", userID).First(&perm).Error; err == nil {
			result["permissions"] = perm.Permissions
		} else if err == gorm.ErrRecordNotFound {
			// 超级Admin默认拥有所有Permission
			if user.IsSuperAdmin() {
				result["permissions"] = getAllPermissions()
			} else {
				result["permissions"] = []string{}
			}
		} else {
			result["permissions"] = []string{}
		}
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

	if err := h.authService.ChangePassword(userID, req.OldPassword, req.NewPassword); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, gin.H{
		"message": "Password changed successfully",
	})
}

// UpdatePreferencesRequest 更新用户偏好请求
type UpdatePreferencesRequest struct {
	Locale  string `json:"locale"`
	Country string `json:"country"`
}

// UpdatePreferences 更新用户偏好设置
func (h *AuthHandler) UpdatePreferences(c *gin.Context) {
	userID := middleware.MustGetUserID(c)

	var req UpdatePreferencesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	if err := h.authService.UpdatePreferences(userID, req.Locale, req.Country); err != nil {
		response.BadRequest(c, err.Error())
		return
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

// getAllPermissions get所有Permission列表
// 超级Admin默认拥有所有Permission（除了特殊Permission order.view_privacy）
func getAllPermissions() []string {
	return []string{
		"order.view",
		// "order.view_privacy", // 特殊Permission，need单独授予
		"order.edit",
		"order.delete",
		"order.status_update",
		"order.assign_tracking",
		"order.request_resubmit",
		"user.view",
		"user.edit",
		"user.permission",
		"admin.create",
		"admin.edit",
		"admin.delete",
		"admin.permission",
		"system.config",
		"system.logs",
		"api.manage",
	}
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
			"user_id": user.ID,
			"uuid":    user.UUID,
			"email":   user.Email,
			"name":    user.Name,
			"role":    user.Role,
			"avatar":  user.Avatar,
			"locale":  user.Locale,
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
