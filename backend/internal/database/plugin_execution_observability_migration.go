package database

import (
	"encoding/json"
	"regexp"
	"strings"

	"auralogic/internal/models"
)

const (
	pluginExecutionObservabilityBackfillBatchSize = 500
	pluginExecutionMetadataHookKey                = "plugin_execution_hook"
	pluginExecutionErrorSignatureMaxLength        = 96
)

var pluginExecutionObservabilityDigitsPattern = regexp.MustCompile(`\d+`)

func migratePluginExecutionObservabilityFields() error {
	if DB == nil {
		return nil
	}

	var lastID uint
	for {
		executions := make([]models.PluginExecution, 0, pluginExecutionObservabilityBackfillBatchSize)
		if err := DB.Model(&models.PluginExecution{}).
			Select("id, action, hook, params, metadata, success, error, error_signature").
			Where("id > ?", lastID).
			Where("((action = ? AND COALESCE(hook, '') = '') OR (success = ? AND COALESCE(error_signature, '') = ''))", "hook.execute", false).
			Order("id ASC").
			Limit(pluginExecutionObservabilityBackfillBatchSize).
			Find(&executions).Error; err != nil {
			return err
		}
		if len(executions) == 0 {
			return nil
		}

		for _, execution := range executions {
			if execution.ID > lastID {
				lastID = execution.ID
			}

			updates := map[string]interface{}{}
			if strings.TrimSpace(execution.Hook) == "" {
				if hook := derivePluginExecutionHook(execution.Action, execution.Params, execution.Metadata); hook != "" {
					updates["hook"] = hook
				}
			}
			if !execution.Success && strings.TrimSpace(execution.ErrorSignature) == "" {
				updates["error_signature"] = normalizePluginExecutionErrorSignature(
					normalizePluginExecutionErrorText(execution.Error),
				)
			}
			if len(updates) == 0 {
				continue
			}
			if err := DB.Model(&models.PluginExecution{}).
				Where("id = ?", execution.ID).
				Updates(updates).Error; err != nil {
				return err
			}
		}
	}
}

func derivePluginExecutionHook(action string, paramsJSON string, metadata models.JSONMap) string {
	if !strings.EqualFold(strings.TrimSpace(action), "hook.execute") {
		return ""
	}
	if hook := strings.ToLower(strings.TrimSpace(metadata[pluginExecutionMetadataHookKey])); hook != "" {
		return hook
	}
	if strings.TrimSpace(paramsJSON) == "" {
		return ""
	}
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return ""
	}
	rawHook, _ := params["hook"].(string)
	return strings.ToLower(strings.TrimSpace(rawHook))
}

func normalizePluginExecutionErrorText(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "unknown error"
	}
	lines := strings.Split(trimmed, "\n")
	normalized := strings.Join(strings.Fields(strings.TrimSpace(lines[0])), " ")
	if normalized == "" {
		return "unknown error"
	}
	return normalized
}

func normalizePluginExecutionErrorSignature(raw string) string {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" {
		return "unknown error"
	}
	normalized = pluginExecutionObservabilityDigitsPattern.ReplaceAllString(normalized, "#")
	normalized = strings.Join(strings.Fields(normalized), " ")
	if normalized == "" {
		return "unknown error"
	}
	if len(normalized) > pluginExecutionErrorSignatureMaxLength {
		return normalized[:pluginExecutionErrorSignatureMaxLength-1] + "…"
	}
	return normalized
}
