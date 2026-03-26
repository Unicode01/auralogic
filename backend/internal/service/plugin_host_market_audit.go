package service

import (
	"fmt"
	"strings"

	"auralogic/internal/models"
	"auralogic/internal/pkg/logger"
	"gorm.io/gorm"
)

func logPluginHostMarketOperation(
	db *gorm.DB,
	claims *PluginHostAccessClaims,
	plugin *models.Plugin,
	action string,
	coordinates pluginHostMarketCoordinates,
	params map[string]interface{},
	result map[string]interface{},
) {
	if db == nil || claims == nil || claims.PluginID == 0 {
		return
	}

	resourceType, resourceID := pluginHostMarketAuditResource(result, coordinates.Kind)
	details := map[string]interface{}{
		"origin":      "plugin_host_bridge",
		"plugin_id":   claims.PluginID,
		"plugin_name": pluginHostMarketAuditPluginName(plugin),
		"source_id":   coordinates.SourceID,
		"kind":        coordinates.Kind,
		"name":        coordinates.Name,
		"version":     pluginHostMarketNormalizeVersion(coordinates.Version),
	}

	if status := strings.TrimSpace(pluginMarketStringFromAny(result["status"])); status != "" {
		details["status"] = status
	}
	if taskID := strings.TrimSpace(pluginMarketStringFromAny(result["task_id"])); taskID != "" {
		details["task_id"] = taskID
	}
	if targetKey := pluginHostMarketAuditTargetKey(result, params, coordinates.Kind); targetKey != "" {
		details["target_key"] = targetKey
	}

	if value, ok := result["activate_requested"].(bool); ok {
		details["activate_requested"] = value
	}
	if value, ok := result["auto_start"].(bool); ok {
		details["auto_start"] = value
	}
	if note := strings.TrimSpace(parsePluginHostOptionalString(params, "note")); note != "" {
		details["note"] = note
	}
	if message := strings.TrimSpace(pluginMarketStringFromAny(result["error"])); message != "" {
		details["error"] = message
	}

	var userID *uint
	if claims.OperatorUserID > 0 {
		userID = &claims.OperatorUserID
	}
	logger.LogOperationWithActor(
		db,
		userID,
		fmt.Sprintf("plugin:%s", pluginHostMarketAuditPluginName(plugin)),
		action,
		resourceType,
		resourceID,
		details,
		"",
		"",
	)
}

func pluginHostMarketAuditPluginName(plugin *models.Plugin) string {
	if plugin == nil {
		return "unknown"
	}
	if name := strings.TrimSpace(plugin.Name); name != "" {
		return name
	}
	return fmt.Sprintf("id-%d", plugin.ID)
}

func pluginHostMarketAuditTargetKey(result map[string]interface{}, params map[string]interface{}, kind string) string {
	if template := clonePluginMarketMap(result["template"]); len(template) > 0 {
		if targetKey := strings.TrimSpace(pluginMarketStringFromAny(template["target_key"])); targetKey != "" {
			return targetKey
		}
	}
	if resolved := clonePluginMarketMap(result["resolved"]); len(resolved) > 0 {
		if targetKey := strings.TrimSpace(pluginMarketStringFromAny(resolved["target_key"])); targetKey != "" {
			return targetKey
		}
	}
	if targetState := clonePluginMarketMap(result["target_state"]); len(targetState) > 0 {
		if targetKey := strings.TrimSpace(pluginMarketStringFromAny(targetState["installed_target_key"])); targetKey != "" {
			return targetKey
		}
	}
	return parsePluginHostMarketTemplateTargetKey(kind, params)
}

func pluginHostMarketAuditResource(
	result map[string]interface{},
	kind string,
) (string, *uint) {
	switch kind {
	case "email_template":
		return "email_template", nil
	case "landing_page_template":
		if saved := clonePluginMarketMap(result["saved"]); len(saved) > 0 {
			if id, ok := pluginHostMarketAuditUint(saved["id"]); ok {
				return "landing_page", &id
			}
		}
		return "landing_page", nil
	case "invoice_template":
		return "invoice_template", nil
	case "auth_branding_template":
		return "auth_branding", nil
	case "page_rule_pack":
		return "page_rule_pack", nil
	case "payment_package":
		if method := clonePluginMarketMap(result["payment_method"]); len(method) > 0 {
			if id, ok := pluginHostMarketAuditUint(method["id"]); ok {
				return "payment_method", &id
			}
		}
		if item := clonePluginMarketMap(result["item"]); len(item) > 0 {
			if id, ok := pluginHostMarketAuditUint(item["id"]); ok {
				return "payment_method", &id
			}
		}
		return "payment_method", nil
	case "plugin_package":
		if item := clonePluginMarketMap(result["item"]); len(item) > 0 {
			if id, ok := pluginHostMarketAuditUint(item["id"]); ok {
				return "plugin", &id
			}
		}
		return "plugin", nil
	default:
		return kind, nil
	}
}

func pluginHostMarketAuditUint(value interface{}) (uint, bool) {
	switch typed := value.(type) {
	case uint:
		if typed > 0 {
			return typed, true
		}
	case uint64:
		if typed > 0 {
			return uint(typed), true
		}
	case int:
		if typed > 0 {
			return uint(typed), true
		}
	case int64:
		if typed > 0 {
			return uint(typed), true
		}
	case float64:
		if typed > 0 {
			return uint(typed), true
		}
	}
	return 0, false
}
