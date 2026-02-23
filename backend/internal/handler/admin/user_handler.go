package admin

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"auralogic/internal/config"
	"auralogic/internal/middleware"
	"auralogic/internal/models"
	"auralogic/internal/pkg/logger"
	"auralogic/internal/pkg/password"
	"auralogic/internal/pkg/response"
	"auralogic/internal/repository"
	"gorm.io/gorm"
)

type UserHandler struct {
	userRepo *repository.UserRepository
	db       *gorm.DB
	cfg      *config.Config
}

func NewUserHandler(userRepo *repository.UserRepository, db *gorm.DB, cfg *config.Config) *UserHandler {
	return &UserHandler{
		userRepo: userRepo,
		db:       db,
		cfg:      cfg,
	}
}

// CreateUser CreateUser
func (h *UserHandler) CreateUser(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required,min=8"`
		Name     string `json:"name"`
		Role     string `json:"role"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	// Check if email already exists
	if _, err := h.userRepo.FindByEmail(req.Email); err == nil {
		response.Conflict(c, "Email already in use")
		return
	} else if err != nil && err != gorm.ErrRecordNotFound {
		response.InternalError(c, "Query failed")
		return
	}

	// Default role is user
	if req.Role == "" {
		req.Role = "user"
	}

	// 验证角色
	if req.Role != "user" && req.Role != "admin" && req.Role != "super_admin" {
		response.BadRequest(c, "Invalid role")
		return
	}

	// Only super admin can create admin accounts
	currentRole, _ := middleware.GetUserRole(c)
	if (req.Role == "admin" || req.Role == "super_admin") && currentRole != "super_admin" {
		response.Forbidden(c, "Only super admin can create admin accounts")
		return
	}

	// Encrypt password
	policy := h.cfg.Security.PasswordPolicy
	if err := password.ValidatePasswordPolicy(req.Password, policy.MinLength, policy.RequireUppercase,
		policy.RequireLowercase, policy.RequireNumber, policy.RequireSpecial); err != nil {
		// Password policy errors are safe to show.
		response.BadRequest(c, err.Error())
		return
	}

	hashedPassword, err := password.HashPassword(req.Password)
	if err != nil {
		response.InternalError(c, "Password encryption failed")
		return
	}

	// CreateUser
	user := &models.User{
		UUID:          uuid.New().String(),
		Email:         req.Email,
		PasswordHash:  hashedPassword,
		Name:          req.Name,
		Role:          req.Role,
		IsActive:      true,
		EmailVerified: true,
	}

	if err := h.userRepo.Create(user); err != nil {
		response.InternalError(c, "CreateFailed")
		return
	}

	// 记录操作日志
	logger.LogUserOperation(h.db, c, "create", user.ID, map[string]interface{}{
		"email": user.Email,
		"name":  user.Name,
		"role":  user.Role,
	})

	response.Success(c, user)
}

// ListUsers - Get user list
func (h *UserHandler) ListUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	search := c.Query("search")

	if limit > 100 {
		limit = 100
	}

	users, total, err := h.userRepo.List(page, limit, search)
	if err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	// 为管理员用户附加权限信息
	type UserWithPermissions struct {
		models.User
		Permissions []string `json:"permissions"`
	}

	result := make([]UserWithPermissions, 0, len(users))
	for _, user := range users {
		uwp := UserWithPermissions{User: user}

		// 如果是管理员，获取权限
		if user.IsAdmin() {
			var perm models.AdminPermission
			if err := h.db.Where("user_id = ?", user.ID).First(&perm).Error; err == nil {
				uwp.Permissions = perm.Permissions
			} else {
				uwp.Permissions = []string{}
			}
		}

		result = append(result, uwp)
	}

	response.Paginated(c, result, page, limit, total)
}

// GetUser - Get user details
func (h *UserHandler) GetUser(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid user ID format")
		return
	}

	user, err := h.userRepo.FindByID(uint(userID))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.NotFound(c, "User not found")
			return
		}
		response.InternalError(c, "Query failed")
		return
	}

	response.Success(c, user)
}

// UpdateUser UpdateUserInfo
func (h *UserHandler) UpdateUser(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid user ID format")
		return
	}

	var req struct {
		Name     string  `json:"name"`
		Role     string  `json:"role"`
		IsActive *bool   `json:"is_active"`
		Password *string `json:"password" binding:"omitempty,min=8"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	user, err := h.userRepo.FindByID(uint(userID))
	if err != nil {
		response.NotFound(c, "User not found")
		return
	}

	// Only super admin can modify roles
	currentRole, _ := middleware.GetUserRole(c)
	if req.Role != "" && currentRole != "super_admin" {
		response.Forbidden(c, "Only Admin can modify user role")
		return
	}

	passwordChanged := false
	if req.Password != nil {
		newPwd := strings.TrimSpace(*req.Password)
		if newPwd != "" {
			// Prevent privilege escalation: only super_admin can change admin passwords here.
			if user.IsAdmin() && currentRole != "super_admin" {
				response.Forbidden(c, "Only super admin can change admin password")
				return
			}

			policy := h.cfg.Security.PasswordPolicy
			if err := password.ValidatePasswordPolicy(newPwd, policy.MinLength, policy.RequireUppercase,
				policy.RequireLowercase, policy.RequireNumber, policy.RequireSpecial); err != nil {
				response.BadRequest(c, err.Error())
				return
			}

			hashedPassword, err := password.HashPassword(newPwd)
			if err != nil {
				response.InternalError(c, "Password encryption failed")
				return
			}

			user.PasswordHash = hashedPassword
			passwordChanged = true
		}
	}

	if req.Name != "" {
		user.Name = req.Name
	}
	if req.Role != "" {
		user.Role = req.Role
	}
	if req.IsActive != nil {
		user.IsActive = *req.IsActive
	}

	if err := h.userRepo.Update(user); err != nil {
		response.InternalError(c, "UpdateFailed")
		return
	}

	// 角色变更时清除权限缓存
	if req.Role != "" {
		middleware.InvalidatePermissionCache(user.ID)
	}

	// 记录操作日志
	details := map[string]interface{}{
		"name":      req.Name,
		"role":      req.Role,
		"is_active": req.IsActive,
	}
	if passwordChanged {
		// Never log plaintext password.
		details["password_changed"] = true
	}
	logger.LogUserOperation(h.db, c, "update", user.ID, details)

	response.Success(c, user)
}

// DeleteUser DeleteUser
func (h *UserHandler) DeleteUser(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid user ID format")
		return
	}

	currentUserID := middleware.MustGetUserID(c)

	// Cannot delete yourself
	if uint(userID) == currentUserID {
		response.BadRequest(c, "Cannot delete yourself")
		return
	}

	user, err := h.userRepo.FindByID(uint(userID))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.NotFound(c, "User not found")
			return
		}
		response.InternalError(c, "Query failed")
		return
	}

	// Check if user is admin (admins should be deleted via AdminDelete interface)
	if user.IsAdmin() {
		response.BadRequest(c, "Please use admin delete interface for admin accounts")
		return
	}

	// Soft delete user
	if err := h.userRepo.Delete(uint(userID)); err != nil {
		response.InternalError(c, "DeleteFailed")
		return
	}

	// 记录操作日志
	logger.LogUserOperation(h.db, c, "delete", uint(userID), map[string]interface{}{
		"email": user.Email,
		"name":  user.Name,
	})

	response.Success(c, gin.H{
		"message": "User has been deleted",
	})
}

// GetUserOrders getUserOrder List
func (h *UserHandler) GetUserOrders(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid user ID format")
		return
	}

	// Call OrderRepository to get user orders
	// 简化处理，返回提示
	response.Success(c, gin.H{
		"user_id": userID,
		"message": "Please use Order management interface to query user order",
	})
}
