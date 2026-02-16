package middleware

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"auralogic/internal/database"
	"auralogic/internal/models"
	"auralogic/internal/pkg/jwt"
	"auralogic/internal/pkg/response"
)

// AuthMiddleware 双认证中间件：优先JWT，回退API Key
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 优先尝试 JWT Bearer Token
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && parts[0] == "Bearer" {
				tokenString := parts[1]
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
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && parts[0] == "Bearer" {
				tokenString := parts[1]
				claims, err := jwt.ParseToken(tokenString)
				if err == nil {
					c.Set("user_id", claims.UserID)
					c.Set("user_email", claims.Email)
					c.Set("user_role", claims.Role)
				}
			}
		}
		c.Next()
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

// MustGetUserID 从上下文getUserID，does not exist则返回0并中止请求
func MustGetUserID(c *gin.Context) uint {
	userID, exists := GetUserID(c)
	if !exists {
		response.Unauthorized(c, "Authentication required")
		c.Abort()
		return 0
	}
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

