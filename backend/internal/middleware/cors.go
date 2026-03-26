package middleware

import (
	"time"

	"auralogic/internal/config"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// CORS 跨域中间件
func CORS(cfg *config.CORSConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		corsConfig := cors.Config{
			AllowOrigins:     append([]string(nil), cfg.AllowedOrigins...),
			AllowMethods:     append([]string(nil), cfg.AllowedMethods...),
			AllowHeaders:     append([]string(nil), cfg.AllowedHeaders...),
			ExposeHeaders:    []string{"Content-Length"},
			AllowCredentials: true,
			MaxAge:           time.Duration(cfg.MaxAge) * time.Second,
		}
		cors.New(corsConfig)(c)
	}
}
