package admin

import (
	"log"
	"strconv"
	"strings"

	"auralogic/internal/config"
	"auralogic/internal/middleware"
	"auralogic/internal/models"
	"auralogic/internal/pkg/logger"
	"auralogic/internal/pkg/password"
	"auralogic/internal/pkg/response"
	"auralogic/internal/repository"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type UserHandler struct {
	userRepo      *repository.UserRepository
	orderRepo     *repository.OrderRepository
	db            *gorm.DB
	cfg           *config.Config
	pluginManager *service.PluginManagerService
}

func NewUserHandler(userRepo *repository.UserRepository, db *gorm.DB, cfg *config.Config, pluginManager *service.PluginManagerService) *UserHandler {
	return &UserHandler{
		userRepo:      userRepo,
		orderRepo:     repository.NewOrderRepository(db),
		db:            db,
		cfg:           cfg,
		pluginManager: pluginManager,
	}
}

var userConsumptionStatuses = []models.OrderStatus{
	models.OrderStatusDraft,
	models.OrderStatusNeedResubmit,
	models.OrderStatusPending,
	models.OrderStatusShipped,
	models.OrderStatusCompleted,
}

func (h *UserHandler) enrichUserConsumptionStats(user *models.User) {
	if user == nil || user.ID == 0 || h.orderRepo == nil {
		return
	}

	orderCount, totalSpentMinor, err := h.orderRepo.GetUserConsumptionSummary(user.ID, userConsumptionStatuses)
	if err != nil {
		return
	}

	originalCount := user.TotalOrderCount
	originalSpent := user.TotalSpentMinor
	user.TotalOrderCount = orderCount
	user.TotalSpentMinor = totalSpentMinor

	if originalCount != orderCount || originalSpent != totalSpentMinor {
		_ = h.userRepo.UpdateConsumptionStats(user.ID, totalSpentMinor, orderCount)
	}
}

// userToResponse converts a User model to a safe response map with explicit fields
func userToResponse(user *models.User) gin.H {
	resp := gin.H{
		"id":                user.ID,
		"uuid":              user.UUID,
		"email":             user.Email,
		"name":              user.Name,
		"avatar":            user.Avatar,
		"role":              user.Role,
		"is_active":         user.IsActive,
		"email_verified":    user.EmailVerified,
		"locale":            user.Locale,
		"last_login_ip":     user.LastLoginIP,
		"register_ip":       user.RegisterIP,
		"country":           user.Country,
		"last_login_at":     user.LastLoginAt,
		"total_spent_minor": user.TotalSpentMinor,
		"total_order_count": user.TotalOrderCount,
		"created_at":        user.CreatedAt,
		"updated_at":        user.UpdatedAt,
	}
	if user.Phone != nil {
		resp["phone"] = user.Phone
	}
	return resp
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
	adminID, _ := middleware.GetUserID(c)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	initialIsActive := true

	if h.pluginManager != nil {
		hookExecCtx := buildAdminHookExecutionContext(c, &adminID, map[string]string{
			"resource_type": "user",
		})
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook: "user.admin.create.before",
			Payload: map[string]interface{}{
				"source":           "admin_api",
				"email":            req.Email,
				"password_present": strings.TrimSpace(req.Password) != "",
				"name":             req.Name,
				"role":             req.Role,
				"is_active":        initialIsActive,
			},
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("user.admin.create.before hook execution failed: email=%s err=%v", req.Email, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "User creation rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if value, exists := hookResult.Payload["email"]; exists {
					email, convErr := adminHookValueToString(value)
					if convErr != nil {
						log.Printf("user.admin.create.before email decode failed: %v", convErr)
					} else {
						req.Email = strings.ToLower(strings.TrimSpace(email))
					}
				}
				if value, exists := hookResult.Payload["name"]; exists {
					name, convErr := adminHookValueToString(value)
					if convErr != nil {
						log.Printf("user.admin.create.before name decode failed: %v", convErr)
					} else {
						req.Name = strings.TrimSpace(name)
					}
				}
				if value, exists := hookResult.Payload["role"]; exists {
					role, convErr := adminHookValueToString(value)
					if convErr != nil {
						log.Printf("user.admin.create.before role decode failed: %v", convErr)
					} else {
						req.Role = strings.TrimSpace(role)
					}
				}
				if value, exists := hookResult.Payload["is_active"]; exists {
					updated, ok, convErr := orderValueToOptionalBool(value)
					if convErr != nil {
						log.Printf("user.admin.create.before is_active decode failed: %v", convErr)
					} else if ok {
						initialIsActive = updated
					}
				}
			}
		}
	}

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
		if respondAdminPasswordPolicyBizError(c, err) {
			return
		}
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
		UUID:                 uuid.New().String(),
		Email:                req.Email,
		PasswordHash:         hashedPassword,
		Name:                 req.Name,
		Role:                 req.Role,
		IsActive:             initialIsActive,
		EmailVerified:        true,
		EmailNotifyMarketing: true,
		SMSNotifyMarketing:   true,
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

	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"source":         "admin_api",
			"user_id":        user.ID,
			"email":          user.Email,
			"name":           user.Name,
			"role":           user.Role,
			"is_active":      user.IsActive,
			"email_verified": user.EmailVerified,
			"created_by":     adminID,
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, email string) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "user.admin.create.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("user.admin.create.after hook execution failed: email=%s err=%v", email, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, &adminID, map[string]string{
			"resource_type": "user",
			"resource_id":   strconv.FormatUint(uint64(user.ID), 10),
		})), afterPayload, user.Email)
	}

	response.Success(c, userToResponse(user))
}

// ListUsers - Get user list
func (h *UserHandler) ListUsers(c *gin.Context) {
	page, limit := response.GetPagination(c)
	search := strings.TrimSpace(c.Query("search"))
	role := strings.TrimSpace(c.Query("role"))
	locale := strings.TrimSpace(c.Query("locale"))
	country := strings.TrimSpace(c.Query("country"))

	isActive, ok := parseOptionalBoolQuery(c.Query("is_active"))
	if !ok {
		response.BadRequest(c, "Invalid is_active parameter")
		return
	}
	emailVerified, ok := parseOptionalBoolQuery(c.Query("email_verified"))
	if !ok {
		response.BadRequest(c, "Invalid email_verified parameter")
		return
	}
	emailNotifyMarketing, ok := parseOptionalBoolQuery(c.Query("email_notify_marketing"))
	if !ok {
		response.BadRequest(c, "Invalid email_notify_marketing parameter")
		return
	}
	smsNotifyMarketing, ok := parseOptionalBoolQuery(c.Query("sms_notify_marketing"))
	if !ok {
		response.BadRequest(c, "Invalid sms_notify_marketing parameter")
		return
	}
	hasPhone, ok := parseOptionalBoolQuery(c.Query("has_phone"))
	if !ok {
		response.BadRequest(c, "Invalid has_phone parameter")
		return
	}

	users, total, err := h.userRepo.List(page, limit, repository.UserListFilters{
		Search:               search,
		Role:                 role,
		IsActive:             isActive,
		EmailVerified:        emailVerified,
		EmailNotifyMarketing: emailNotifyMarketing,
		SMSNotifyMarketing:   smsNotifyMarketing,
		HasPhone:             hasPhone,
		Locale:               locale,
		Country:              country,
	})
	if err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	// 为管理员用户附加权限信息
	result := make([]gin.H, 0, len(users))
	for _, user := range users {
		h.enrichUserConsumptionStats(&user)
		item := userToResponse(&user)

		// 如果是管理员，获取权限
		if user.IsAdmin() {
			var perm models.AdminPermission
			if err := h.db.Where("user_id = ?", user.ID).First(&perm).Error; err == nil {
				item["permissions"] = perm.Permissions
			} else {
				item["permissions"] = []string{}
			}
		}

		result = append(result, item)
	}

	response.Paginated(c, result, page, limit, total)
}

// ListUserCountries returns distinct country codes from users.
func (h *UserHandler) ListUserCountries(c *gin.Context) {
	role := strings.TrimSpace(c.Query("role"))

	countries, err := h.userRepo.ListCountries(role)
	if err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	response.Success(c, gin.H{
		"countries": countries,
	})
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
	h.enrichUserConsumptionStats(user)

	response.Success(c, userToResponse(user))
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
	adminID, _ := middleware.GetUserID(c)

	user, err := h.userRepo.FindByID(uint(userID))
	if err != nil {
		response.NotFound(c, "User not found")
		return
	}

	if h.pluginManager != nil {
		beforePayload := map[string]interface{}{
			"source":       "admin_api",
			"user_id":      user.ID,
			"email":        user.Email,
			"name":         req.Name,
			"role":         req.Role,
			"password_set": req.Password != nil && strings.TrimSpace(*req.Password) != "",
		}
		if req.IsActive != nil {
			beforePayload["is_active"] = *req.IsActive
		}

		hookExecCtx := buildAdminHookExecutionContext(c, &adminID, map[string]string{
			"resource_type": "user",
			"resource_id":   strconv.FormatUint(userID, 10),
		})
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "user.admin.update.before",
			Payload: beforePayload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("user.admin.update.before hook execution failed: user_id=%d err=%v", user.ID, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "User update rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if value, exists := hookResult.Payload["name"]; exists {
					name, convErr := adminHookValueToString(value)
					if convErr != nil {
						log.Printf("user.admin.update.before name decode failed: %v", convErr)
					} else {
						req.Name = strings.TrimSpace(name)
					}
				}
				if value, exists := hookResult.Payload["role"]; exists {
					role, convErr := adminHookValueToString(value)
					if convErr != nil {
						log.Printf("user.admin.update.before role decode failed: %v", convErr)
					} else {
						req.Role = strings.TrimSpace(role)
					}
				}
				if value, exists := hookResult.Payload["is_active"]; exists {
					updated, ok, convErr := orderValueToOptionalBool(value)
					if convErr != nil {
						log.Printf("user.admin.update.before is_active decode failed: %v", convErr)
					} else if ok {
						req.IsActive = &updated
					}
				}
			}
		}
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
				if respondAdminPasswordPolicyBizError(c, err) {
					return
				}
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

	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"source":           "admin_api",
			"user_id":          user.ID,
			"email":            user.Email,
			"name":             user.Name,
			"role":             user.Role,
			"is_active":        user.IsActive,
			"password_changed": passwordChanged,
			"updated_by":       adminID,
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, targetID uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "user.admin.update.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("user.admin.update.after hook execution failed: user_id=%d err=%v", targetID, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, &adminID, map[string]string{
			"resource_type": "user",
			"resource_id":   strconv.FormatUint(uint64(user.ID), 10),
		})), afterPayload, user.ID)
	}

	response.Success(c, userToResponse(user))
}

// DeleteUser DeleteUser
func (h *UserHandler) DeleteUser(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid user ID format")
		return
	}

	currentUserID, _ := middleware.GetUserID(c)

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

	if h.pluginManager != nil {
		hookExecCtx := buildAdminHookExecutionContext(c, &currentUserID, map[string]string{
			"resource_type": "user",
			"resource_id":   strconv.FormatUint(userID, 10),
		})
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook: "user.admin.delete.before",
			Payload: map[string]interface{}{
				"source":    "admin_api",
				"user_id":   user.ID,
				"email":     user.Email,
				"name":      user.Name,
				"role":      user.Role,
				"is_active": user.IsActive,
			},
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("user.admin.delete.before hook execution failed: user_id=%d err=%v", user.ID, hookErr)
		} else if hookResult != nil && hookResult.Blocked {
			reason := strings.TrimSpace(hookResult.BlockReason)
			if reason == "" {
				reason = "User deletion rejected by plugin"
			}
			response.BadRequest(c, reason)
			return
		}
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

	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"source":     "admin_api",
			"user_id":    user.ID,
			"email":      user.Email,
			"name":       user.Name,
			"role":       user.Role,
			"is_active":  user.IsActive,
			"deleted_by": currentUserID,
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, targetID uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "user.admin.delete.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("user.admin.delete.after hook execution failed: user_id=%d err=%v", targetID, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, &currentUserID, map[string]string{
			"resource_type": "user",
			"resource_id":   strconv.FormatUint(userID, 10),
		})), afterPayload, user.ID)
	}

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
