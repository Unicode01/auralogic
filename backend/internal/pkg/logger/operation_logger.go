package logger

import (
	"github.com/gin-gonic/gin"
	"auralogic/internal/models"
	"auralogic/internal/pkg/utils"
	"gorm.io/gorm"
)

// LogOperation 记录操作日志
func LogOperation(db *gorm.DB, c *gin.Context, action, resourceType string, resourceID *uint, details map[string]interface{}) {
	// getcurrentUserID（可能为空，比如登录操作）
	var userID *uint
	if id, exists := c.Get("user_id"); exists {
		if uid, ok := id.(uint); ok {
			userID = &uid
		}
	}

	// get操作者名称（API平台名称或其他）
	var operatorName string
	if platform, exists := c.Get("api_platform"); exists {
		if platformStr, ok := platform.(string); ok && platformStr != "" {
			operatorName = platformStr
		}
	}

	log := &models.OperationLog{
		UserID:       userID,
		OperatorName: operatorName,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Details:      details,
		IPAddress:    utils.GetRealIP(c),
		UserAgent:    c.Request.UserAgent(),
	}

	// 同步记录日志，确保立即可见
	db.Create(log)
}

// LogSystemOperation 记录系统操作日志（无需gin.Context，用于后台服务）
func LogSystemOperation(db *gorm.DB, action, resourceType string, resourceID *uint, details map[string]interface{}) {
	log := &models.OperationLog{
		OperatorName: "system",
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Details:      details,
	}
	db.Create(log)
}

// LogPaymentOperation 记录付款相关操作日志
func LogPaymentOperation(db *gorm.DB, action string, orderID uint, details map[string]interface{}) {
	LogSystemOperation(db, action, "payment", &orderID, details)
}

// LogUserOperation 记录User相关操作
func LogUserOperation(db *gorm.DB, c *gin.Context, action string, targetUserID uint, details map[string]interface{}) {
	LogOperation(db, c, action, "user", &targetUserID, details)
}

// LogAdminOperation 记录Admin相关操作
func LogAdminOperation(db *gorm.DB, c *gin.Context, action string, targetAdminID uint, details map[string]interface{}) {
	LogOperation(db, c, action, "admin", &targetAdminID, details)
}

// LogAPIKeyOperation 记录API密钥相关操作
func LogAPIKeyOperation(db *gorm.DB, c *gin.Context, action string, keyID uint, details map[string]interface{}) {
	LogOperation(db, c, action, "api_key", &keyID, details)
}

// LogOrderOperation 记录Order相关操作
func LogOrderOperation(db *gorm.DB, c *gin.Context, action string, orderID uint, details map[string]interface{}) {
	LogOperation(db, c, action, "order", &orderID, details)
}

// LogLoginAttempt 记录登录尝试
func LogLoginAttempt(db *gorm.DB, c *gin.Context, email string, success bool, userID *uint) {
	details := map[string]interface{}{
		"email":   email,
		"success": success,
	}

	log := &models.OperationLog{
		UserID:       userID,
		Action:       "login",
		ResourceType: "auth",
		Details:      details,
		IPAddress:    utils.GetRealIP(c),
		UserAgent:    c.Request.UserAgent(),
	}

	// 同步记录登录日志
	db.Create(log)
}

