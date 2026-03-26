package middleware

import (
	"encoding/json"
	"fmt"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/database"
	"auralogic/internal/models"
	"auralogic/internal/pkg/cache"
	"auralogic/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

const permCacheTTL = 1 * time.Minute

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

// InvalidateAllPermissionCache 清除所有用户权限缓存
func InvalidateAllPermissionCache() (int64, error) {
	return cache.DeleteByPatterns("perm:user:*")
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

	// 写入缓存
	if data, err := json.Marshal(entry); err == nil {
		cache.Set(key, string(data), permCacheTTL)
	}

	return &entry, nil
}

// RequirePermission Permission检查中间件
func RequirePermission(permission string) gin.HandlerFunc {
	return RequireAllPermissions(permission)
}

// RequireAnyPermission 任一权限检查中间件
func RequireAnyPermission(permissions ...string) gin.HandlerFunc {
	return requirePermissions(permissionMatchAny, permissions...)
}

// RequireAllPermissions 全部权限检查中间件
func RequireAllPermissions(permissions ...string) gin.HandlerFunc {
	return requirePermissions(permissionMatchAll, permissions...)
}

type permissionMatchMode int

const (
	permissionMatchAny permissionMatchMode = iota
	permissionMatchAll
)

func requirePermissions(mode permissionMatchMode, permissions ...string) gin.HandlerFunc {
	requiredPermissions := uniqueNormalizedPermissions(permissions)
	return func(c *gin.Context) {
		if len(requiredPermissions) == 0 {
			response.Forbidden(c, "No permission")
			c.Abort()
			return
		}

		// API Key 认证：检查 scopes
		if IsAPIKeyAuth(c) {
			if apiKeyHasPermissions(c, requiredPermissions, mode) {
				c.Next()
				return
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

		if jwtHasPermissions(entry, requiredPermissions, mode) {
			c.Next()
			return
		}

		response.Forbidden(c, "No permission")
		c.Abort()
	}
}

func apiKeyHasPermissions(c *gin.Context, requiredPermissions []string, mode permissionMatchMode) bool {
	if c == nil {
		return false
	}

	scopes, exists := c.Get("api_scopes")
	if !exists {
		return false
	}

	scopeList, ok := scopes.([]string)
	if !ok {
		return false
	}

	return permissionSetMatches(buildPermissionSet(scopeList), requiredPermissions, mode)
}

func jwtHasPermissions(entry *permCacheEntry, requiredPermissions []string, mode permissionMatchMode) bool {
	if entry == nil {
		return false
	}

	if entry.Role == "super_admin" {
		switch mode {
		case permissionMatchAny:
			for _, permission := range requiredPermissions {
				if !IsSpecialPermission(permission) {
					return true
				}
			}
		case permissionMatchAll:
			requiredPermissions = filterSpecialPermissions(requiredPermissions)
			if len(requiredPermissions) == 0 {
				return true
			}
		}
	}

	return permissionSetMatches(buildPermissionSet(entry.Permissions), requiredPermissions, mode)
}

func buildPermissionSet(permissions []string) map[string]struct{} {
	set := make(map[string]struct{}, len(permissions))
	for _, permission := range permissions {
		normalized := permission
		if normalized == "" {
			continue
		}
		set[normalized] = struct{}{}
	}
	return set
}

func permissionSetMatches(permissionSet map[string]struct{}, requiredPermissions []string, mode permissionMatchMode) bool {
	switch mode {
	case permissionMatchAny:
		for _, permission := range requiredPermissions {
			if _, exists := permissionSet[permission]; exists {
				return true
			}
		}
		return false
	case permissionMatchAll:
		for _, permission := range requiredPermissions {
			if _, exists := permissionSet[permission]; !exists {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func filterSpecialPermissions(permissions []string) []string {
	filtered := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		if IsSpecialPermission(permission) {
			filtered = append(filtered, permission)
		}
	}
	return filtered
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

// RequireAdmin 要求AdminPermission（API Key认证需至少拥有一个scope）
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if IsAPIKeyAuth(c) {
			scopes, exists := c.Get("api_scopes")
			if exists {
				if scopeList, ok := scopes.([]string); ok && len(scopeList) > 0 {
					c.Next()
					return
				}
			}
			response.Forbidden(c, "No permission")
			c.Abort()
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

// RequireSuperAdmin 要求超级管理员权限（API Key 不允许访问超级管理员端点）
func RequireSuperAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		// API Key 不允许访问超级管理员端点，必须使用 JWT 登录
		if IsAPIKeyAuth(c) {
			response.Forbidden(c, "API keys cannot access super admin endpoints")
			c.Abort()
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
			response.BadRequest(c, "Ticket system is disabled")
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
			response.BadRequest(c, "Serial number query feature is disabled")
			c.Abort()
			return
		}
		c.Next()
	}
}
