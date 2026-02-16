package admin

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"auralogic/internal/middleware"
	"auralogic/internal/models"
	"auralogic/internal/pkg/logger"
	"auralogic/internal/pkg/response"
	"auralogic/internal/pkg/utils"
	"gorm.io/gorm"
)

type APIKeyHandler struct {
	db *gorm.DB
}

func NewAPIKeyHandler(db *gorm.DB) *APIKeyHandler {
	return &APIKeyHandler{db: db}
}

// ListAPIKeys getAPI密钥列表
func (h *APIKeyHandler) ListAPIKeys(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	if limit > 100 {
		limit = 100
	}

	var keys []models.APIKey
	var total int64

	query := h.db.Model(&models.APIKey{})

	// get总数
	if err := query.Count(&total).Error; err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	// 分页Query
	offset := (page - 1) * limit
	if err := query.Offset(offset).Limit(limit).Find(&keys).Error; err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	response.Paginated(c, keys, page, limit, total)
}

// CreateAPIKey CreateAPI密钥
func (h *APIKeyHandler) CreateAPIKey(c *gin.Context) {
	var req struct {
		KeyName   string    `json:"key_name" binding:"required"`
		Platform  string    `json:"platform"`
		Scopes    []string  `json:"scopes"`
		RateLimit int       `json:"rate_limit"`
		ExpiresAt time.Time `json:"expires_at"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	currentUserID := middleware.MustGetUserID(c)

	// generateAPI密钥
	apiKey, err := utils.GenerateAPIKey("ak_live")
	if err != nil {
		response.InternalError(c, "generateAPI KeyFailed")
		return
	}

	apiSecret, err := utils.GenerateAPIKey("sk_live")
	if err != nil {
		response.InternalError(c, "generateAPI SecretFailed")
		return
	}

	// 设置默认限流
	if req.RateLimit == 0 {
		req.RateLimit = 1000
	}

	key := &models.APIKey{
		KeyName:   req.KeyName,
		APIKey:    apiKey,
		Platform:  req.Platform,
		Scopes:    req.Scopes,
		RateLimit: req.RateLimit,
		IsActive:  true,
		CreatedBy: currentUserID,
	}

	// 使用bcrypt哈希存储Secret
	if err := key.SetSecret(apiSecret); err != nil {
		response.InternalError(c, "生成API Secret失败")
		return
	}

	if !req.ExpiresAt.IsZero() {
		key.ExpiresAt = &req.ExpiresAt
	}

	if err := h.db.Create(key).Error; err != nil {
		response.InternalError(c, "CreateFailed")
		return
	}

	// 记录操作日志
	logger.LogAPIKeyOperation(h.db, c, "create", key.ID, map[string]interface{}{
		"key_name": key.KeyName,
		"platform": key.Platform,
		"scopes":   key.Scopes,
	})

	response.Success(c, gin.H{
		"id":         key.ID,
		"key_name":   key.KeyName,
		"api_key":    key.APIKey,
		"api_secret": apiSecret, // 返回原始Secret（仅此一次，之后无法恢复）
		"platform":   key.Platform,
		"scopes":     key.Scopes,
		"rate_limit": key.RateLimit,
		"created_at": key.CreatedAt,
		"message":    "⚠️ API Secret仅显示一次，请妥善保管！",
	})
}

// DeleteAPIKey DeleteAPI密钥
func (h *APIKeyHandler) DeleteAPIKey(c *gin.Context) {
	keyID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid API key ID format")
		return
	}

	// 先get密钥Info用于日志
	var key models.APIKey
	h.db.First(&key, keyID)

	if err := h.db.Delete(&models.APIKey{}, keyID).Error; err != nil {
		response.InternalError(c, "DeleteFailed")
		return
	}

	// 记录操作日志
	logger.LogAPIKeyOperation(h.db, c, "delete", uint(keyID), map[string]interface{}{
		"key_name": key.KeyName,
		"platform": key.Platform,
	})

	response.Success(c, gin.H{
		"message": "DeleteSuccess",
	})
}

// UpdateAPIKey UpdateAPI密钥状态
func (h *APIKeyHandler) UpdateAPIKey(c *gin.Context) {
	keyID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid API key ID format")
		return
	}

	var req struct {
		IsActive  *bool  `json:"is_active"`
		RateLimit *int   `json:"rate_limit"`
		KeyName   string `json:"key_name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	var key models.APIKey
	if err := h.db.First(&key, keyID).Error; err != nil {
		response.NotFound(c, "API key does not exist")
		return
	}

	if req.IsActive != nil {
		key.IsActive = *req.IsActive
	}
	if req.RateLimit != nil {
		key.RateLimit = *req.RateLimit
	}
	if req.KeyName != "" {
		key.KeyName = req.KeyName
	}

	if err := h.db.Save(&key).Error; err != nil {
		response.InternalError(c, "UpdateFailed")
		return
	}

	// 记录操作日志
	logger.LogAPIKeyOperation(h.db, c, "update", key.ID, map[string]interface{}{
		"key_name":   req.KeyName,
		"is_active":  req.IsActive,
		"rate_limit": req.RateLimit,
	})

	response.Success(c, key)
}

