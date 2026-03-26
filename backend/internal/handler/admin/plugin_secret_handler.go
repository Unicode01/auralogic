package admin

import (
	"errors"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"auralogic/internal/models"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type updatePluginSecretsRequest struct {
	Upserts    map[string]string `json:"upserts"`
	DeleteKeys []string          `json:"delete_keys"`
}

func buildPluginSecretUpdateHookPayload(plugin *models.Plugin, current []service.PluginSecretMeta, upserts map[string]string, deleteKeys []string) map[string]interface{} {
	payload := map[string]interface{}{
		"upsert_keys":  sortedStringMapKeys(upserts),
		"upsert_count": len(upserts),
		"delete_keys":  normalizePluginSecretDeleteKeys(deleteKeys),
		"delete_count": len(normalizePluginSecretDeleteKeys(deleteKeys)),
		"current":      current,
	}
	if plugin != nil {
		payload["plugin_id"] = plugin.ID
		payload["plugin_name"] = plugin.Name
		payload["plugin_display_name"] = plugin.DisplayName
		payload["plugin_type"] = plugin.Type
		payload["plugin_runtime"] = plugin.Runtime
		payload["plugin_enabled"] = plugin.Enabled
	}
	return payload
}

func (h *PluginHandler) GetPluginSecrets(c *gin.Context) {
	id, ok := h.parsePluginID(c)
	if !ok {
		return
	}
	if h.pluginManager == nil {
		h.respondPluginError(c, http.StatusInternalServerError, "Plugin manager is unavailable")
		return
	}

	var plugin models.Plugin
	if err := h.db.First(&plugin, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			h.respondPluginError(c, http.StatusNotFound, "Plugin not found")
			return
		}
		h.respondPluginError(c, http.StatusInternalServerError, "Failed to query plugin")
		return
	}

	items, err := h.pluginManager.ListPluginSecretMeta(id)
	if err != nil {
		h.respondPluginErrorErr(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *PluginHandler) UpdatePluginSecrets(c *gin.Context) {
	id, ok := h.parsePluginID(c)
	if !ok {
		return
	}
	if h.pluginManager == nil {
		h.respondPluginError(c, http.StatusInternalServerError, "Plugin manager is unavailable")
		return
	}

	var plugin models.Plugin
	if err := h.db.First(&plugin, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			h.respondPluginError(c, http.StatusNotFound, "Plugin not found")
			return
		}
		h.respondPluginError(c, http.StatusInternalServerError, "Failed to query plugin")
		return
	}

	var req updatePluginSecretsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
		return
	}

	if err := validatePluginSecretPatchAgainstManifest(plugin.Manifest, req.Upserts, req.DeleteKeys); err != nil {
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
		return
	}
	currentItems, err := h.pluginManager.ListPluginSecretMeta(id)
	if err != nil {
		h.respondPluginErrorErr(c, http.StatusInternalServerError, err)
		return
	}
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}
	if h.pluginManager != nil {
		hookPayload := buildPluginSecretUpdateHookPayload(&plugin, currentItems, req.Upserts, req.DeleteKeys)
		hookPayload["admin_id"] = adminIDValue
		hookPayload["source"] = "admin_api"
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "plugin.secret.update.before",
			Payload: hookPayload,
		}, buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "plugin_secret",
			"hook_source":   "admin_api",
			"hook_action":   "update",
			"plugin_id":     strconv.FormatUint(uint64(id), 10),
		}))
		if hookErr != nil {
			log.Printf("plugin.secret.update.before hook execution failed: admin=%d plugin=%d err=%v", adminIDValue, id, hookErr)
		} else if hookResult != nil && hookResult.Blocked {
			reason := strings.TrimSpace(hookResult.BlockReason)
			if reason == "" {
				reason = "Plugin secret update rejected by plugin"
			}
			h.respondPluginError(c, http.StatusBadRequest, reason)
			return
		}
	}
	if err := h.pluginManager.ApplyPluginSecretPatch(id, req.Upserts, req.DeleteKeys); err != nil {
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
		return
	}

	items, err := h.pluginManager.ListPluginSecretMeta(id)
	if err != nil {
		h.respondPluginErrorErr(c, http.StatusInternalServerError, err)
		return
	}

	h.logPluginOperation(c, "plugin_secret_update", &plugin, &plugin.ID, map[string]interface{}{
		"upsert_keys": sortedStringMapKeys(req.Upserts),
		"delete_keys": normalizePluginSecretDeleteKeys(req.DeleteKeys),
	})
	c.JSON(http.StatusOK, items)
	if h.pluginManager != nil {
		afterPayload := buildPluginSecretUpdateHookPayload(&plugin, items, req.Upserts, req.DeleteKeys)
		afterPayload["admin_id"] = adminIDValue
		afterPayload["source"] = "admin_api"
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, pluginID uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "plugin.secret.update.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("plugin.secret.update.after hook execution failed: admin=%d plugin=%d err=%v", adminIDValue, pluginID, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "plugin_secret",
			"hook_source":   "admin_api",
			"hook_action":   "update",
			"plugin_id":     strconv.FormatUint(uint64(id), 10),
		})), afterPayload, id)
	}
}

func validatePluginSecretPatchAgainstManifest(rawManifest string, upserts map[string]string, deleteKeys []string) error {
	allowedKeys := extractPluginSecretSchemaKeys(rawManifest)
	if len(allowedKeys) == 0 {
		return nil
	}

	for key := range upserts {
		if err := validatePluginSecretKeyAllowed(key, allowedKeys); err != nil {
			return err
		}
	}
	for _, key := range deleteKeys {
		if err := validatePluginSecretKeyAllowed(key, allowedKeys); err != nil {
			return err
		}
	}
	return nil
}

func extractPluginSecretSchemaKeys(rawManifest string) map[string]struct{} {
	manifest := parseJSONObjectString(rawManifest)
	if len(manifest) == 0 {
		return nil
	}
	schema := asStringAnyMap(manifest["secret_schema"])
	if len(schema) == 0 {
		return nil
	}
	fields, _ := schema["fields"].([]interface{})
	if len(fields) == 0 {
		return nil
	}

	keys := make(map[string]struct{}, len(fields))
	for _, item := range fields {
		field, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		key := strings.TrimSpace(parseStringFromAny(field["key"]))
		if key == "" {
			continue
		}
		keys[key] = struct{}{}
	}
	return keys
}

func validatePluginSecretKeyAllowed(key string, allowedKeys map[string]struct{}) error {
	normalizedKey := strings.TrimSpace(key)
	if normalizedKey == "" {
		return nil
	}
	if _, exists := allowedKeys[normalizedKey]; exists {
		return nil
	}
	return newPluginBizError(http.StatusBadRequest, "plugin secret key is not declared by secret_schema", map[string]interface{}{
		"key": normalizedKey,
	})
}

func normalizePluginSecretDeleteKeys(keys []string) []string {
	if len(keys) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(keys))
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "" {
			continue
		}
		if _, exists := seen[normalizedKey]; exists {
			continue
		}
		seen[normalizedKey] = struct{}{}
		out = append(out, normalizedKey)
	}
	sort.Strings(out)
	return out
}
