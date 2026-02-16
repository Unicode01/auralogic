package middleware

import (
	"github.com/gin-gonic/gin"
)

// SecurityHeaders 添加安全响应头
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 防止点击劫持
		c.Header("X-Frame-Options", "DENY")

		// 启用XSS过滤器
		c.Header("X-XSS-Protection", "1; mode=block")

		// 防止MIME类型嗅探
		c.Header("X-Content-Type-Options", "nosniff")

		// 强制HTTPS
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

		// 限制Referer信息泄露
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// 内容安全策略 - 防止XSS和数据注入
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self' data:; connect-src 'self' https:; frame-ancestors 'none'")

		// 权限策略
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")

		c.Next()
	}
}

// NoCache 禁用缓存（用于敏感接口）
func NoCache() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		c.Next()
	}
}
