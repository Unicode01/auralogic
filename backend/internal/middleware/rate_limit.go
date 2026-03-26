package middleware

import (
	"fmt"
	"strconv"
	"time"

	"auralogic/internal/pkg/cache"
	"auralogic/internal/pkg/response"
	"auralogic/internal/pkg/utils"
	"github.com/gin-gonic/gin"
)

// RateLimitMiddleware 限流中间件
func RateLimitMiddleware(limit int, window time.Duration) gin.HandlerFunc {
	return RateLimitMiddlewareWithObserver(limit, window, nil)
}

type RateLimitObserver func(c *gin.Context, blocked bool, currentCount int64, limit int, window time.Duration)
type RateLimitResolver func(c *gin.Context) (limit int, enabled bool)

// RateLimitMiddlewareWithObserver 带观察回调的限流中间件
func RateLimitMiddlewareWithObserver(limit int, window time.Duration, observer RateLimitObserver) gin.HandlerFunc {
	return DynamicRateLimitMiddlewareWithObserver(func(*gin.Context) (int, bool) {
		return limit, limit > 0
	}, window, observer)
}

// DynamicRateLimitMiddleware 按请求动态解析限流配置。
func DynamicRateLimitMiddleware(resolve RateLimitResolver, window time.Duration) gin.HandlerFunc {
	return DynamicRateLimitMiddlewareWithObserver(resolve, window, nil)
}

// DynamicRateLimitMiddlewareWithObserver 按请求动态解析限流配置，并支持观测回调。
func DynamicRateLimitMiddlewareWithObserver(resolve RateLimitResolver, window time.Duration, observer RateLimitObserver) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit := 0
		enabled := false
		if resolve != nil {
			limit, enabled = resolve(c)
		}
		if !enabled || limit <= 0 {
			if observer != nil {
				observer(c, false, 0, limit, window)
			}
			c.Next()
			return
		}

		// get客户端标识（IP或UserID）
		key := getClientKey(c)
		if key == "" {
			if observer != nil {
				observer(c, false, 0, limit, window)
			}
			c.Next()
			return
		}

		// 限流键
		rateLimitKey := fmt.Sprintf("rate:%s:%d", key, time.Now().Unix()/int64(window.Seconds()))

		// 增加计数
		count, err := cache.Incr(rateLimitKey)
		if err != nil {
			// 限流Failed不影响业务
			if observer != nil {
				observer(c, false, 0, limit, window)
			}
			c.Next()
			return
		}

		// 设置过期时间
		if count == 1 {
			cache.Expire(rateLimitKey, window)
		}

		// 检查是否超过限制
		if count > int64(limit) {
			c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
			c.Header("X-RateLimit-Remaining", "0")
			c.Header("Retry-After", strconv.Itoa(int(window.Seconds())))
			if observer != nil {
				observer(c, true, count, limit, window)
			}
			response.Error(c, 429, response.CodeTooManyRequests, "Too many requests, please try again later")
			c.Abort()
			return
		}

		// 设置限流头
		c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
		c.Header("X-RateLimit-Remaining", strconv.FormatInt(int64(limit)-count, 10))
		if observer != nil {
			observer(c, false, count, limit, window)
		}

		c.Next()
	}
}

// getClientKey get客户端标识
func getClientKey(c *gin.Context) string {
	// 优先使用UserID
	if userID, exists := GetUserID(c); exists {
		return fmt.Sprintf("user:%d", userID)
	}

	// 使用API Key
	if apiKey, exists := c.Get("api_key"); exists {
		return fmt.Sprintf("api:%s", apiKey)
	}

	// 使用IP
	return fmt.Sprintf("ip:%s", utils.GetRealIP(c))
}
