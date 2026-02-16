package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"auralogic/internal/pkg/utils"
)

// Logger 日志中间件
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		c.Next()

		duration := time.Since(start)
		statusCode := c.Writer.Status()

		// 如果有Error，记录ErrorInfo
		if len(c.Errors) > 0 {
			log.Printf("[%s] %s %s - %d - %v - ERROR: %v",
				method,
				path,
				utils.GetRealIP(c),
				statusCode,
				duration,
				c.Errors.String(),
			)
		} else {
			log.Printf("[%s] %s %s - %d - %v",
				method,
				path,
				utils.GetRealIP(c),
				statusCode,
				duration,
			)
		}
	}
}

