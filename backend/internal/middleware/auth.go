package middleware

import (
	"context"
	"strconv"
	"strings"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/database"
	"auralogic/internal/models"
	"auralogic/internal/pkg/jwt"
	"auralogic/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

const websocketBearerTokenProtocolPrefix = "auralogic.auth.bearer."

// AuthMiddleware 双认证中间件：优先JWT，回退API Key
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 优先尝试 JWT Bearer Token
		if tokenString := extractBearerToken(c); tokenString != "" {
			claims, err := jwt.ParseToken(tokenString)
			if err != nil {
				response.Error(c, 401, response.CodeTokenInvalid, "Invalid authentication token")
				c.Abort()
				return
			}

			db := database.GetDB()
			var user models.User
			if err := db.Select("id", "email", "role", "is_active").First(&user, claims.UserID).Error; err != nil {
				response.Unauthorized(c, "Invalid authentication token")
				c.Abort()
				return
			}
			if !user.IsActive {
				response.Unauthorized(c, "User account has been disabled")
				c.Abort()
				return
			}

			c.Set("auth_type", "jwt")
			c.Set("user_id", user.ID)
			c.Set("user_email", user.Email)
			c.Set("user_role", user.Role)
			c.Next()
			return
		}

		// 回退到 API Key 认证
		apiKey := c.GetHeader("X-API-Key")
		apiSecret := c.GetHeader("X-API-Secret")
		if apiKey != "" && apiSecret != "" {
			var key models.APIKey
			db := database.GetDB()
			if err := db.Where("api_key = ? AND is_active = ?", apiKey, true).First(&key).Error; err != nil {
				response.Error(c, 401, response.CodeAPIKeyInvalid, "Invalid API key")
				c.Abort()
				return
			}

			if !key.VerifySecret(apiSecret) {
				response.Error(c, 401, response.CodeAPIKeyInvalid, "API key verification failed")
				c.Abort()
				return
			}

			if key.IsExpired() {
				response.Error(c, 401, response.CodeAPIKeyInvalid, "API key has expired")
				c.Abort()
				return
			}

			c.Set("auth_type", "api_key")
			c.Set("user_id", key.CreatedBy)
			c.Set("api_key_id", key.ID)
			c.Set("api_key", apiKey)
			c.Set("api_scopes", key.Scopes)
			c.Set("api_platform", key.Platform)

			// 异步更新最后使用时间
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				now := models.NowFunc()
				db.WithContext(ctx).Model(&key).Update("last_used_at", now)
			}()

			c.Next()
			return
		}

		response.Unauthorized(c, "Missing authentication token")
		c.Abort()
	}
}

// IsAPIKeyAuth 检查当前请求是否为 API Key 认证
func IsAPIKeyAuth(c *gin.Context) bool {
	authType, exists := c.Get("auth_type")
	return exists && authType == "api_key"
}

// OptionalAuthMiddleware 可选的认证中间件
func OptionalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if tokenString := extractBearerToken(c); tokenString != "" {
			claims, err := jwt.ParseToken(tokenString)
			if err == nil {
				db := database.GetDB()
				if db != nil {
					var user models.User
					if db.Select("id", "email", "role", "is_active").First(&user, claims.UserID).Error == nil && user.IsActive {
						c.Set("user_id", user.ID)
						c.Set("user_email", user.Email)
						c.Set("user_role", user.Role)
					}
				}
			}
		}
		c.Next()
	}
}

func extractBearerToken(c *gin.Context) string {
	if c == nil {
		return ""
	}
	authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(strings.TrimSpace(parts[0]), "Bearer") {
			return strings.TrimSpace(parts[1])
		}
	}
	if !isWebSocketUpgradeRequest(c) {
		return ""
	}
	for _, protocol := range strings.Split(c.GetHeader("Sec-WebSocket-Protocol"), ",") {
		trimmed := strings.TrimSpace(protocol)
		if !strings.HasPrefix(trimmed, websocketBearerTokenProtocolPrefix) {
			continue
		}
		if token := strings.TrimSpace(strings.TrimPrefix(trimmed, websocketBearerTokenProtocolPrefix)); token != "" {
			return token
		}
	}
	return ""
}

func isWebSocketUpgradeRequest(c *gin.Context) bool {
	if c == nil || c.Request == nil {
		return false
	}
	connection := strings.ToLower(strings.TrimSpace(c.GetHeader("Connection")))
	upgrade := strings.ToLower(strings.TrimSpace(c.GetHeader("Upgrade")))
	return strings.Contains(connection, "upgrade") && upgrade == "websocket"
}

// ProductBrowseAuthMiddleware 动态控制商品浏览是否需要登录。
// 当 allow_guest_product_browse=true 时放行游客，否则走 AuthMiddleware。
func ProductBrowseAuthMiddleware(cfg *config.Config) gin.HandlerFunc {
	auth := AuthMiddleware()
	return func(c *gin.Context) {
		if cfg != nil && cfg.Security.Login.AllowGuestProductBrowse {
			c.Next()
			return
		}
		auth(c)
	}
}

// GetUserID 从上下文getUserID
func GetUserID(c *gin.Context) (uint, bool) {
	userID, exists := c.Get("user_id")
	if !exists {
		return 0, false
	}
	id, ok := userID.(uint)
	return id, ok
}

// GetUserRole 从上下文getUser角色
func GetUserRole(c *gin.Context) (string, bool) {
	role, exists := c.Get("user_role")
	if !exists {
		return "", false
	}
	r, ok := role.(string)
	return r, ok
}

// RequireUserID 从上下文获取用户 ID；不存在则返回未授权并中止请求
func RequireUserID(c *gin.Context) (uint, bool) {
	userID, exists := GetUserID(c)
	if !exists {
		response.Unauthorized(c, "Authentication required")
		c.Abort()
		return 0, false
	}
	return userID, true
}

// MustGetUserID 从上下文获取用户 ID；不存在则返回 0 并中止请求
func MustGetUserID(c *gin.Context) uint {
	userID, _ := RequireUserID(c)
	return userID
}

// GetUintParam 从URL参数获取uint类型的值
func GetUintParam(c *gin.Context, key string) (uint, error) {
	param := c.Param(key)
	id, err := strconv.ParseUint(param, 10, 32)
	if err != nil {
		return 0, err
	}
	return uint(id), nil
}
