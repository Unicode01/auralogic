package admin

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"auralogic/internal/middleware"
	"auralogic/internal/models"
	"auralogic/internal/pkg/response"
	"gorm.io/gorm"
)

type PermissionHandler struct {
	db *gorm.DB
}

func NewPermissionHandler(db *gorm.DB) *PermissionHandler {
	return &PermissionHandler{db: db}
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

	currentUserID := middleware.MustGetUserID(c)

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

	response.Success(c, perm)
}

// ListAllPermissions get所有可用Permission
func (h *PermissionHandler) ListAllPermissions(c *gin.Context) {
	permissions := map[string][]string{
		"OrderPermission": {
			"order.view",
			"order.view_privacy",
			"order.edit",
			"order.delete",
			"order.status_update",
			"order.assign_tracking",
			"order.request_resubmit",
		},
		"ProductPermission": {
			"product.view",
			"product.edit",
			"product.delete",
		},
		"SerialPermission": {
			"serial.view",
			"serial.manage",
		},
		"UserPermission": {
			"user.view",
			"user.edit",
			"user.permission",
		},
		"AdminPermission": {
			"admin.create",
			"admin.edit",
			"admin.delete",
			"admin.permission",
		},
		"SystemPermission": {
			"system.config",
			"system.logs",
			"api.manage",
		},
		"KnowledgePermission": {
			"knowledge.view",
			"knowledge.edit",
		},
		"AnnouncementPermission": {
			"announcement.view",
			"announcement.edit",
		},
		"TicketPermission": {
			"ticket.view",
			"ticket.reply",
			"ticket.status_update",
		},
	}

	response.Success(c, permissions)
}
