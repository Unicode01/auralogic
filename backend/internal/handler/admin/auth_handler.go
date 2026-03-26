package admin

import (
	"strings"

	"auralogic/internal/database"
	"auralogic/internal/middleware"
	"auralogic/internal/models"
	"auralogic/internal/pkg/logger"
	"auralogic/internal/pkg/response"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AuthHandler struct {
	authService *service.AuthService
}

func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

// LoginRequest Admin登录请求
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// Login Admin登录
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	token, user, err := h.authService.Login(req.Email, req.Password)
	if err != nil {
		// 记录Failed的登录尝试
		db := database.GetDB()
		logger.LogLoginAttempt(db, c, req.Email, false, nil)
		response.Unauthorized(c, err.Error())
		return
	}

	// 检查是否是Admin
	if !user.IsAdmin() {
		// 记录非Admin尝试登录管理后台
		db := database.GetDB()
		logger.LogLoginAttempt(db, c, req.Email, false, &user.ID)
		response.Forbidden(c, "No permission to access admin panel")
		return
	}

	// getAdminPermission列表
	permissions := []string{}
	db := database.GetDB()
	if user.IsAdmin() {
		permissions = middleware.EffectiveAdminPermissions(user.Role, nil)
		var perm models.AdminPermission
		if err := db.Where("user_id = ?", user.ID).First(&perm).Error; err == nil {
			permissions = middleware.EffectiveAdminPermissions(user.Role, perm.Permissions)
		} else if err != gorm.ErrRecordNotFound {
			permissions = []string{}
		}
	}

	// 记录Success的登录
	logger.LogLoginAttempt(db, c, req.Email, true, &user.ID)

	response.Success(c, gin.H{
		"token":      token,
		"token_type": "Bearer",
		"user": gin.H{
			"user_id":     user.ID,
			"email":       user.Email,
			"name":        user.Name,
			"role":        user.Role,
			"permissions": permissions,
		},
	})
}
