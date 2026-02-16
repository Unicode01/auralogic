package middleware

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"auralogic/internal/config"
	"auralogic/internal/database"
	"auralogic/internal/models"
	"auralogic/internal/pkg/cache"
	"auralogic/internal/pkg/response"
)

const permCacheTTL = 5 * time.Minute

// permCacheEntry 权限缓存条目
type permCacheEntry struct {
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
}

// SpecialPermissions 需要单独授权的特殊权限（即使超级管理员也需要显式授予）
// 这些权限涉及敏感数据访问，必须经过明确授权
var SpecialPermissions = map[string]bool{
	"order.view_privacy": true, // 查看订单隐私保护信息
}

// IsSpecialPermission 检查是否为特殊权限
func IsSpecialPermission(permission string) bool {
	return SpecialPermissions[permission]
}

func permCacheKey(userID uint) string {
	return fmt.Sprintf("perm:user:%d", userID)
}

// InvalidatePermissionCache 清除用户权限缓存（权限变更时调用）
func InvalidatePermissionCache(userID uint) {
	cache.Del(permCacheKey(userID))
}

// getPermCached 从缓存获取用户角色和权限，缓存未命中则查DB并写入缓存
func getPermCached(userID uint) (*permCacheEntry, error) {
	key := permCacheKey(userID)

	// 尝试从缓存获取
	cached, err := cache.Get(key)
	if err == nil && cached != "" {
		var entry permCacheEntry
		if json.Unmarshal([]byte(cached), &entry) == nil {
			return &entry, nil
		}
	}

	// 缓存未命中，查数据库
	db := database.GetDB()
	var user models.User
	if err := db.Select("id", "role").First(&user, userID).Error; err != nil {
		return nil, fmt.Errorf("user not found")
	}

	entry := permCacheEntry{Role: user.Role}

	var perm models.AdminPermission
	if err := db.Where("user_id = ?", userID).First(&perm).Error; err == nil {
		entry.Permissions = perm.Permissions
	}

	// 写入缓���
	if data, err := json.Marshal(entry); err == nil {
		cache.Set(key, string(data), permCacheTTL)
	}

	return &entry, nil
}

// RequirePermission Permission检查中间件
func RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// API Key 认证：检查 scopes
		if IsAPIKeyAuth(c) {
			scopes, exists := c.Get("api_scopes")
			if exists {
				if scopeList, ok := scopes.([]string); ok {
					for _, s := range scopeList {
						if s == permission {
							c.Next()
							return
						}
					}
				}
			}
			response.Forbidden(c, "No permission")
			c.Abort()
			return
		}

		// JWT 认证：原有逻辑
		userID, exists := GetUserID(c)
		if !exists {
			response.Forbidden(c, "Login required")
			c.Abort()
			return
		}

		entry, err := getPermCached(userID)
		if err != nil {
			response.Forbidden(c, "User not found")
			c.Abort()
			return
		}

		// 超级Admin拥有所有Permission（除非是需要特殊授权的Permission）
		if entry.Role == "super_admin" {
			if !IsSpecialPermission(permission) {
				c.Next()
				return
			}
		}

		// 检查Permission
		for _, p := range entry.Permissions {
			if p == permission {
				c.Next()
				return
			}
		}

		response.Forbidden(c, "No permission")
		c.Abort()
	}
}

// RequireRole 角色检查中间件
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := GetUserRole(c)
		if !exists {
			response.Forbidden(c, "Login required")
			c.Abort()
			return
		}

		for _, r := range roles {
			if r == role {
				c.Next()
				return
			}
		}

		response.Forbidden(c, "No permission to access")
		c.Abort()
	}
}

// RequireAdmin 要求AdminPermission（API Key认证跳过角色检查，由scopes控制）
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if IsAPIKeyAuth(c) {
			c.Next()
			return
		}
		role, exists := GetUserRole(c)
		if !exists {
			response.Forbidden(c, "Login required")
			c.Abort()
			return
		}
		if role == "admin" || role == "super_admin" {
			c.Next()
			return
		}
		response.Forbidden(c, "No permission to access")
		c.Abort()
	}
}

// RequireSuperAdmin 要求超级AdminPermission（API Key认证跳过角色检查，由scopes控制）
func RequireSuperAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if IsAPIKeyAuth(c) {
			c.Next()
			return
		}
		role, exists := GetUserRole(c)
		if !exists {
			response.Forbidden(c, "Login required")
			c.Abort()
			return
		}
		if role == "super_admin" {
			c.Next()
			return
		}
		response.Forbidden(c, "No permission to access")
		c.Abort()
	}
}

// RequireTicketEnabled 工单系统开关中间件
func RequireTicketEnabled() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := config.GetConfig()
		if !cfg.Ticket.Enabled {
			response.BadRequest(c, "工单系统已关闭")
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireSerialEnabled 序列号查询开关中间件
func RequireSerialEnabled() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := config.GetConfig()
		if !cfg.Serial.Enabled {
			response.BadRequest(c, "序列号查询功能已关闭")
			c.Abort()
			return
		}
		c.Next()
	}
}
