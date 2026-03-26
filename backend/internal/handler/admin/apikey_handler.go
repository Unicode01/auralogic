package admin

import (
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	"auralogic/internal/middleware"
	"auralogic/internal/models"
	"auralogic/internal/pkg/logger"
	"auralogic/internal/pkg/response"
	"auralogic/internal/pkg/utils"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type APIKeyHandler struct {
	db            *gorm.DB
	pluginManager *service.PluginManagerService
}

func NewAPIKeyHandler(db *gorm.DB, pluginManager *service.PluginManagerService) *APIKeyHandler {
	return &APIKeyHandler{
		db:            db,
		pluginManager: pluginManager,
	}
}

func buildAPIKeyHookPayload(key *models.APIKey) map[string]interface{} {
	if key == nil {
		return map[string]interface{}{}
	}

	return map[string]interface{}{
		"api_key_id":   key.ID,
		"key_name":     key.KeyName,
		"api_key":      key.APIKey,
		"platform":     key.Platform,
		"scopes":       key.Scopes,
		"rate_limit":   key.RateLimit,
		"is_active":    key.IsActive,
		"last_used_at": key.LastUsedAt,
		"expires_at":   key.ExpiresAt,
		"created_by":   key.CreatedBy,
		"created_at":   key.CreatedAt,
		"updated_at":   key.UpdatedAt,
	}
}

// ListAPIKeys getAPI密钥列表
func (h *APIKeyHandler) ListAPIKeys(c *gin.Context) {
	page, limit := response.GetPagination(c)

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
	if h.pluginManager != nil {
		originalReq := req
		hookPayload, payloadErr := adminHookStructToPayload(req)
		if payloadErr != nil {
			log.Printf("apikey.create.before payload build failed: admin=%d err=%v", currentUserID, payloadErr)
		} else {
			hookPayload["admin_id"] = currentUserID
			hookPayload["source"] = "admin_api"
			hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "apikey.create.before",
				Payload: hookPayload,
			}, buildAdminHookExecutionContext(c, &currentUserID, map[string]string{
				"hook_resource": "api_key",
				"hook_source":   "admin_api",
				"hook_action":   "create",
			}))
			if hookErr != nil {
				log.Printf("apikey.create.before hook execution failed: admin=%d err=%v", currentUserID, hookErr)
			} else if hookResult != nil {
				if hookResult.Blocked {
					reason := strings.TrimSpace(hookResult.BlockReason)
					if reason == "" {
						reason = "API key creation rejected by plugin"
					}
					response.BadRequest(c, reason)
					return
				}
				if hookResult.Payload != nil {
					if mergeErr := mergeAdminHookStructPatch(&req, hookResult.Payload); mergeErr != nil {
						log.Printf("apikey.create.before payload apply failed, fallback to original request: admin=%d err=%v", currentUserID, mergeErr)
						req = originalReq
					}
				}
			}
		}
	}

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
		response.InternalError(c, "Failed to generate API secret")
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
		"api_secret": apiSecret,
		"platform":   key.Platform,
		"scopes":     key.Scopes,
		"rate_limit": key.RateLimit,
		"created_at": key.CreatedAt,
		"message":    "⚠️ API Secret is only shown once, please keep it safe!",
	})

	if h.pluginManager != nil {
		afterPayload := buildAPIKeyHookPayload(key)
		afterPayload["admin_id"] = currentUserID
		afterPayload["source"] = "admin_api"
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, keyID uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "apikey.create.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("apikey.create.after hook execution failed: admin=%d api_key=%d err=%v", currentUserID, keyID, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, &currentUserID, map[string]string{
			"hook_resource": "api_key",
			"hook_source":   "admin_api",
			"hook_action":   "create",
			"api_key_id":    strconv.FormatUint(uint64(key.ID), 10),
		})), afterPayload, key.ID)
	}
}

// DeleteAPIKey DeleteAPI密钥
func (h *APIKeyHandler) DeleteAPIKey(c *gin.Context) {
	keyID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid API key ID format")
		return
	}

	var key models.APIKey
	if err := h.db.First(&key, keyID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "API key does not exist")
			return
		}
		response.InternalError(c, "Query failed")
		return
	}

	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}
	if h.pluginManager != nil {
		hookPayload := buildAPIKeyHookPayload(&key)
		hookPayload["admin_id"] = adminIDValue
		hookPayload["source"] = "admin_api"
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "apikey.delete.before",
			Payload: hookPayload,
		}, buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "api_key",
			"hook_source":   "admin_api",
			"hook_action":   "delete",
			"api_key_id":    strconv.FormatUint(keyID, 10),
		}))
		if hookErr != nil {
			log.Printf("apikey.delete.before hook execution failed: admin=%d api_key=%d err=%v", adminIDValue, uint(keyID), hookErr)
		} else if hookResult != nil && hookResult.Blocked {
			reason := strings.TrimSpace(hookResult.BlockReason)
			if reason == "" {
				reason = "API key deletion rejected by plugin"
			}
			response.BadRequest(c, reason)
			return
		}
	}

	if err := h.db.Delete(&models.APIKey{}, keyID).Error; err != nil {
		response.InternalError(c, "DeleteFailed")
		return
	}

	logger.LogAPIKeyOperation(h.db, c, "delete", uint(keyID), map[string]interface{}{
		"key_name": key.KeyName,
		"platform": key.Platform,
	})

	response.Success(c, gin.H{
		"message": "DeleteSuccess",
	})

	if h.pluginManager != nil {
		afterPayload := buildAPIKeyHookPayload(&key)
		afterPayload["admin_id"] = adminIDValue
		afterPayload["source"] = "admin_api"
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, deletedID uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "apikey.delete.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("apikey.delete.after hook execution failed: admin=%d api_key=%d err=%v", adminIDValue, deletedID, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "api_key",
			"hook_source":   "admin_api",
			"hook_action":   "delete",
			"api_key_id":    strconv.FormatUint(keyID, 10),
		})), afterPayload, key.ID)
	}
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

	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}
	if h.pluginManager != nil {
		originalReq := req
		hookPayload, payloadErr := adminHookStructToPayload(req)
		if payloadErr != nil {
			log.Printf("apikey.update.before payload build failed: admin=%d api_key=%d err=%v", adminIDValue, uint(keyID), payloadErr)
		} else {
			hookPayload["api_key_id"] = uint(keyID)
			hookPayload["current"] = buildAPIKeyHookPayload(&key)
			hookPayload["admin_id"] = adminIDValue
			hookPayload["source"] = "admin_api"
			hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "apikey.update.before",
				Payload: hookPayload,
			}, buildAdminHookExecutionContext(c, adminID, map[string]string{
				"hook_resource": "api_key",
				"hook_source":   "admin_api",
				"hook_action":   "update",
				"api_key_id":    strconv.FormatUint(keyID, 10),
			}))
			if hookErr != nil {
				log.Printf("apikey.update.before hook execution failed: admin=%d api_key=%d err=%v", adminIDValue, uint(keyID), hookErr)
			} else if hookResult != nil {
				if hookResult.Blocked {
					reason := strings.TrimSpace(hookResult.BlockReason)
					if reason == "" {
						reason = "API key update rejected by plugin"
					}
					response.BadRequest(c, reason)
					return
				}
				if hookResult.Payload != nil {
					if mergeErr := mergeAdminHookStructPatch(&req, hookResult.Payload); mergeErr != nil {
						log.Printf("apikey.update.before payload apply failed, fallback to original request: admin=%d api_key=%d err=%v", adminIDValue, uint(keyID), mergeErr)
						req = originalReq
					}
				}
			}
		}
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

	logger.LogAPIKeyOperation(h.db, c, "update", key.ID, map[string]interface{}{
		"key_name":   req.KeyName,
		"is_active":  req.IsActive,
		"rate_limit": req.RateLimit,
	})

	response.Success(c, key)

	if h.pluginManager != nil {
		afterPayload := buildAPIKeyHookPayload(&key)
		afterPayload["admin_id"] = adminIDValue
		afterPayload["source"] = "admin_api"
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, updatedID uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "apikey.update.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("apikey.update.after hook execution failed: admin=%d api_key=%d err=%v", adminIDValue, updatedID, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "api_key",
			"hook_source":   "admin_api",
			"hook_action":   "update",
			"api_key_id":    strconv.FormatUint(keyID, 10),
		})), afterPayload, key.ID)
	}
}
