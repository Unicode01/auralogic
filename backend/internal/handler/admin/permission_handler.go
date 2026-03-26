package admin

import (
	"encoding/json"
	"log"
	"strconv"
	"strings"

	"auralogic/internal/middleware"
	"auralogic/internal/models"
	"auralogic/internal/pkg/response"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type PermissionHandler struct {
	db            *gorm.DB
	pluginManager *service.PluginManagerService
}

func NewPermissionHandler(db *gorm.DB, pluginManager *service.PluginManagerService) *PermissionHandler {
	return &PermissionHandler{db: db, pluginManager: pluginManager}
}

// GetUserPermissions getUserPermission
func (h *PermissionHandler) GetUserPermissions(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid user ID format")
		return
	}

	var perm models.AdminPermission
	if err := h.db.Where("user_id = ?", userID).First(&perm).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Success(c, gin.H{
				"user_id":     userID,
				"permissions": []string{},
			})
			return
		}
		response.InternalError(c, "Query failed")
		return
	}

	response.Success(c, perm)
}

// UpdateUserPermissions UpdateUserPermission
func (h *PermissionHandler) UpdateUserPermissions(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid user ID format")
		return
	}

	var req struct {
		Permissions []string `json:"permissions" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	currentUserID, _ := middleware.GetUserID(c)
	if h.pluginManager != nil {
		hookExecCtx := buildAdminHookExecutionContext(c, &currentUserID, map[string]string{
			"resource_type": "admin_permission",
			"resource_id":   strconv.FormatUint(userID, 10),
		})
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook: "user.permissions.update.before",
			Payload: map[string]interface{}{
				"source":      "admin_api",
				"user_id":     uint(userID),
				"permissions": append([]string(nil), req.Permissions...),
			},
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("user.permissions.update.before hook execution failed: user_id=%d err=%v", userID, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Permission update rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if value, exists := hookResult.Payload["permissions"]; exists {
					decoded, marshalErr := json.Marshal(value)
					if marshalErr != nil {
						log.Printf("user.permissions.update.before permissions encode failed: %v", marshalErr)
					} else {
						var patched []string
						if unmarshalErr := json.Unmarshal(decoded, &patched); unmarshalErr != nil {
							log.Printf("user.permissions.update.before permissions decode failed: %v", unmarshalErr)
						} else {
							req.Permissions = patched
						}
					}
				}
			}
		}
	}

	// 检查User是否存在
	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		response.NotFound(c, "User not found")
		return
	}

	// 查找或CreatePermission记录
	var perm models.AdminPermission
	err = h.db.Where("user_id = ?", userID).First(&perm).Error
	if err == gorm.ErrRecordNotFound {
		// Create新Permission记录
		perm = models.AdminPermission{
			UserID:      uint(userID),
			Permissions: req.Permissions,
			CreatedBy:   &currentUserID,
		}
		if err := h.db.Create(&perm).Error; err != nil {
			response.InternalError(c, "CreatePermissionFailed")
			return
		}
	} else if err != nil {
		response.InternalError(c, "Query failed")
		return
	} else {
		// UpdatePermission
		perm.Permissions = req.Permissions
		if err := h.db.Save(&perm).Error; err != nil {
			response.InternalError(c, "UpdatePermissionFailed")
			return
		}
	}

	// 清除权限缓存
	middleware.InvalidatePermissionCache(uint(userID))

	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"source":      "admin_api",
			"user_id":     perm.UserID,
			"permissions": append([]string(nil), perm.Permissions...),
			"updated_by":  currentUserID,
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, targetID uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "user.permissions.update.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("user.permissions.update.after hook execution failed: user_id=%d err=%v", targetID, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, &currentUserID, map[string]string{
			"resource_type": "admin_permission",
			"resource_id":   strconv.FormatUint(userID, 10),
		})), afterPayload, uint(userID))
	}

	response.Success(c, perm)
}

// ListAllPermissions get所有可用Permission
func (h *PermissionHandler) ListAllPermissions(c *gin.Context) {
	response.Success(c, middleware.RegisteredAdminPermissionsMap())
}
