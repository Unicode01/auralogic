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

type AdminHandler struct {
	userRepo *repository.UserRepository
	db       *gorm.DB
	cfg      *config.Config
}

func NewAdminHandler(userRepo *repository.UserRepository, db *gorm.DB, cfg *config.Config) *AdminHandler {
	return &AdminHandler{
		userRepo: userRepo,
		db:       db,
		cfg:      cfg,
	}
}

// ListAdmins Admin列表
func (h *AdminHandler) ListAdmins(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	search := c.Query("search")

	if limit > 100 {
		limit = 100
	}

	var admins []models.User
	var total int64

	query := h.db.Model(&models.User{}).Where("role IN ?", []string{"admin", "super_admin"})

	if search != "" {
		query = query.Where("email LIKE ? OR name LIKE ?", "%"+search+"%", "%"+search+"%")
	}

	// get总数
	if err := query.Count(&total).Error; err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	// 分页Query
	offset := (page - 1) * limit
	if err := query.Offset(offset).Limit(limit).Find(&admins).Error; err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	// get每个Admin的Permission
	type AdminWithPermissions struct {
		models.User
		Permissions []string `json:"permissions"`
	}

	result := make([]AdminWithPermissions, 0, len(admins))
	for _, admin := range admins {
		awp := AdminWithPermissions{User: admin}

		var perm models.AdminPermission
		if err := h.db.Where("user_id = ?", admin.ID).First(&perm).Error; err == nil {
			awp.Permissions = perm.Permissions
		} else {
			awp.Permissions = []string{}
		}

		result = append(result, awp)
	}

	response.Paginated(c, result, page, limit, total)
}

// GetAdmin getAdmin详情
func (h *AdminHandler) GetAdmin(c *gin.Context) {
	adminID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid admin ID format")
		return
	}

	admin, err := h.userRepo.FindByID(uint(adminID))
	if err != nil {
		response.NotFound(c, "Admindoes not exist")
		return
	}

	if !admin.IsAdmin() {
		response.NotFound(c, "This user is not an admin")
		return
	}

	// getPermission
	var perm models.AdminPermission
	permissions := []string{}
	if err := h.db.Where("user_id = ?", adminID).First(&perm).Error; err == nil {
		permissions = perm.Permissions
	}

	response.Success(c, gin.H{
		"user_id":     admin.ID,
		"uuid":        admin.UUID,
		"email":       admin.Email,
		"name":        admin.Name,
		"role":        admin.Role,
		"is_active":   admin.IsActive,
		"permissions": permissions,
		"created_at":  admin.CreatedAt,
	})
}

// CreateAdminRequest CreateAdmin请求
type CreateAdminRequest struct {
	Email       string   `json:"email" binding:"required,email"`
	Password    string   `json:"password" binding:"required,min=8"`
	Name        string   `json:"name" binding:"required"`
	Role        string   `json:"role" binding:"omitempty,oneof=admin super_admin"`
	Permissions []string `json:"permissions"`
}

// CreateAdmin CreateAdmin
func (h *AdminHandler) CreateAdmin(c *gin.Context) {
	var req CreateAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Name = strings.TrimSpace(req.Name)

	currentUserID := middleware.MustGetUserID(c)

	// Check if email already exists
	if _, err := h.userRepo.FindByEmail(req.Email); err == nil {
		response.BadRequest(c, "Email already in use")
		return
	} else if err != nil && err != gorm.ErrRecordNotFound {
		response.InternalError(c, "Query failed")
		return
	}

	// 哈希Password
	policy := h.cfg.Security.PasswordPolicy
	if err := password.ValidatePasswordPolicy(req.Password, policy.MinLength, policy.RequireUppercase,
		policy.RequireLowercase, policy.RequireNumber, policy.RequireSpecial); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	hashedPassword, err := password.HashPassword(req.Password)
	if err != nil {
		response.InternalError(c, "Password encryption failed")
		return
	}

	// 如果没有指定角色，默认为admin
	role := req.Role
	if role == "" {
		role = "admin"
	}

	// CreateAdmin
	admin := &models.User{
		UUID:          uuid.New().String(),
		Email:         req.Email,
		PasswordHash:  hashedPassword,
		Name:          req.Name,
		Role:          role,
		IsActive:      true,
		EmailVerified: true,
	}

	if err := h.userRepo.Create(admin); err != nil {
		response.InternalError(c, "CreateAdminFailed")
		return
	}

	// 如果提供了Permission，CreatePermission记录
	if len(req.Permissions) > 0 {
		perm := &models.AdminPermission{
			UserID:      admin.ID,
			Permissions: req.Permissions,
			CreatedBy:   &currentUserID,
		}
		if err := h.db.Create(perm).Error; err != nil {
			// PermissionCreateFailed不影响AdminCreate
			response.Success(c, gin.H{
				"user_id":     admin.ID,
				"uuid":        admin.UUID,
				"email":       admin.Email,
				"name":        admin.Name,
				"role":        admin.Role,
				"permissions": []string{},
				"created_at":  admin.CreatedAt,
				"message":     "Admin created successfully, but permission creation failed. Please assign permissions manually",
			})
			return
		}
	}

	// 记录操作日志
	logger.LogAdminOperation(h.db, c, "create", admin.ID, map[string]interface{}{
		"email":       admin.Email,
		"name":        admin.Name,
		"role":        admin.Role,
		"permissions": req.Permissions,
	})

	response.Success(c, gin.H{
		"user_id":     admin.ID,
		"uuid":        admin.UUID,
		"email":       admin.Email,
		"name":        admin.Name,
		"role":        admin.Role,
		"permissions": req.Permissions,
		"created_at":  admin.CreatedAt,
	})
}

// UpdateAdminRequest UpdateAdmin请求
type UpdateAdminRequest struct {
	Name        string   `json:"name"`
	Role        string   `json:"role" binding:"omitempty,oneof=user admin super_admin"`
	IsActive    *bool    `json:"is_active"`
	Password    *string  `json:"password" binding:"omitempty,min=8"`
	Permissions []string `json:"permissions"`
}

// UpdateAdmin UpdateAdminInfo
func (h *AdminHandler) UpdateAdmin(c *gin.Context) {
	adminID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid admin ID format")
		return
	}

	var req UpdateAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	admin, err := h.userRepo.FindByID(uint(adminID))
	if err != nil {
		response.NotFound(c, "Admindoes not exist")
		return
	}

	if !admin.IsAdmin() {
		response.BadRequest(c, "This user is not an admin")
		return
	}

	// 不能修改自己的角色和状态
	currentUserID := middleware.MustGetUserID(c)
	if admin.ID == currentUserID {
		if req.Role != "" || req.IsActive != nil {
			response.BadRequest(c, "Cannot modify your own role or status")
			return
		}
	}

	// UpdateInfo
	passwordChanged := false
	if req.Password != nil {
		newPwd := strings.TrimSpace(*req.Password)
		if newPwd != "" {
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

			admin.PasswordHash = hashedPassword
			passwordChanged = true
		}
	}

	if req.Name != "" {
		admin.Name = req.Name
	}
	if req.Role != "" {
		admin.Role = req.Role
	}
	if req.IsActive != nil {
		admin.IsActive = *req.IsActive
	}

	if err := h.userRepo.Update(admin); err != nil {
		response.InternalError(c, "UpdateFailed")
		return
	}

	// 清除权限缓存（角色或权限变更时立即生效）
	middleware.InvalidatePermissionCache(uint(adminID))

	// 如果降级为普通用户，清除管理员权限
	if req.Role == "user" {
		h.db.Where("user_id = ?", adminID).Delete(&models.AdminPermission{})
	} else if req.Permissions != nil {
		// 更新权限
		// 查找现有权限记录
		var existingPerm models.AdminPermission
		if err := h.db.Where("user_id = ?", adminID).First(&existingPerm).Error; err != nil {
			// 不存在则创建
			if len(req.Permissions) > 0 {
				perm := &models.AdminPermission{
					UserID:      uint(adminID),
					Permissions: req.Permissions,
					CreatedBy:   &currentUserID,
				}
				h.db.Create(perm)
			}
		} else {
			// 存在则更新
			existingPerm.Permissions = req.Permissions
			h.db.Save(&existingPerm)
		}
	}

	// 获取更新后的权限
	var perm models.AdminPermission
	permissions := []string{}
	if err := h.db.Where("user_id = ?", adminID).First(&perm).Error; err == nil {
		permissions = perm.Permissions
	}

	// 记录操作日志
	details := map[string]interface{}{
		"name":        req.Name,
		"role":        req.Role,
		"is_active":   req.IsActive,
		"permissions": req.Permissions,
	}
	if passwordChanged {
		details["password_changed"] = true
	}
	logger.LogAdminOperation(h.db, c, "update", admin.ID, details)

	response.Success(c, gin.H{
		"id":          admin.ID,
		"uuid":        admin.UUID,
		"email":       admin.Email,
		"name":        admin.Name,
		"role":        admin.Role,
		"is_active":   admin.IsActive,
		"permissions": permissions,
		"updated_at":  admin.UpdatedAt,
	})
}

// DeleteAdmin DeleteAdmin
func (h *AdminHandler) DeleteAdmin(c *gin.Context) {
	adminID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid admin ID format")
		return
	}

	currentUserID := middleware.MustGetUserID(c)

	// Cannot delete yourself
	if uint(adminID) == currentUserID {
		response.BadRequest(c, "Cannot delete yourself")
		return
	}

	admin, err := h.userRepo.FindByID(uint(adminID))
	if err != nil {
		response.NotFound(c, "Admindoes not exist")
		return
	}

	if !admin.IsAdmin() {
		response.BadRequest(c, "This user is not an admin")
		return
	}

	// 软Delete
	if err := h.db.Delete(admin).Error; err != nil {
		response.InternalError(c, "DeleteFailed")
		return
	}

	// 同时DeletePermission记录
	h.db.Where("user_id = ?", adminID).Delete(&models.AdminPermission{})

	// 清除权限缓存
	middleware.InvalidatePermissionCache(uint(adminID))

	// 记录操作日志
	logger.LogAdminOperation(h.db, c, "delete", uint(adminID), map[string]interface{}{
		"email": admin.Email,
		"name":  admin.Name,
		"role":  admin.Role,
	})

	response.Success(c, gin.H{
		"message": "Admin deleted",
	})
}
