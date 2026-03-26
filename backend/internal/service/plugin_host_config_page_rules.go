package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"auralogic/internal/config"
)

const pluginHostPageRulePackTargetKey = "page_rules"

func executePluginHostPageRulePackGet(runtime *PluginHostRuntime, params map[string]interface{}) (map[string]interface{}, error) {
	cfg, filePath, updatedAt, err := loadPluginHostTemplateConfigState(runtime)
	if err != nil {
		return nil, err
	}
	return buildPluginHostPageRulePackResponse(pluginHostPageRulesFromConfig(cfg), filePath, updatedAt)
}

func executePluginHostPageRulePackSave(runtime *PluginHostRuntime, params map[string]interface{}) (map[string]interface{}, error) {
	rules, err := parsePluginHostPageRulePackInput(params)
	if err != nil {
		return nil, err
	}

	cfg, filePath, updatedAt, originalBytes, doc, err := loadPluginHostConfigDocument(runtime)
	if err != nil {
		return nil, err
	}
	currentContent, err := marshalPluginHostPageRules(pluginHostPageRulesFromConfig(cfg))
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "marshal current page rules failed"}
	}
	if conflictErr := pluginHostValidateOptimisticTextWrite(
		currentContent,
		filePath,
		params,
		"page rule pack",
		updatedAt,
	); conflictErr != nil {
		return nil, conflictErr
	}

	customizationMap := ensurePluginHostJSONObject(doc, "customization")
	customizationMap["page_rules"] = clonePluginHostPageRules(rules)

	if err := commitPluginHostConfigDocument(runtime, filePath, originalBytes, doc); err != nil {
		return nil, err
	}

	refreshedCfg, refreshedPath, refreshedUpdatedAt, err := loadPluginHostTemplateConfigState(runtime)
	if err != nil {
		return nil, err
	}
	result, err := buildPluginHostPageRulePackResponse(pluginHostPageRulesFromConfig(refreshedCfg), refreshedPath, refreshedUpdatedAt)
	if err != nil {
		return nil, err
	}
	result["saved"] = true
	return result, nil
}

func executePluginHostPageRulePackReset(runtime *PluginHostRuntime, params map[string]interface{}) (map[string]interface{}, error) {
	cfg, filePath, updatedAt, originalBytes, doc, err := loadPluginHostConfigDocument(runtime)
	if err != nil {
		return nil, err
	}
	currentContent, err := marshalPluginHostPageRules(pluginHostPageRulesFromConfig(cfg))
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "marshal current page rules failed"}
	}
	if conflictErr := pluginHostValidateOptimisticTextWrite(
		currentContent,
		filePath,
		params,
		"page rule pack",
		updatedAt,
	); conflictErr != nil {
		return nil, conflictErr
	}

	customizationMap := ensurePluginHostJSONObject(doc, "customization")
	customizationMap["page_rules"] = []config.PageRule{}

	if err := commitPluginHostConfigDocument(runtime, filePath, originalBytes, doc); err != nil {
		return nil, err
	}

	refreshedCfg, refreshedPath, refreshedUpdatedAt, err := loadPluginHostTemplateConfigState(runtime)
	if err != nil {
		return nil, err
	}
	result, err := buildPluginHostPageRulePackResponse(pluginHostPageRulesFromConfig(refreshedCfg), refreshedPath, refreshedUpdatedAt)
	if err != nil {
		return nil, err
	}
	result["saved"] = true
	result["reset"] = true
	return result, nil
}

func buildPluginHostPageRulePackResponse(rules []config.PageRule, filePath string, updatedAt time.Time) (map[string]interface{}, error) {
	canonicalRules := clonePluginHostPageRules(rules)
	contentBytes, err := marshalPluginHostPageRules(canonicalRules)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "marshal page rules failed"}
	}
	return map[string]interface{}{
		"target_key":    pluginHostPageRulePackTargetKey,
		"file_path":     filePath,
		"rules":         canonicalRules,
		"page_rules":    canonicalRules,
		"content":       string(contentBytes),
		"json_content":  string(contentBytes),
		"digest":        pluginHostDigestBytes(contentBytes),
		"updated_at":    pluginHostOptionalTime(updatedAt),
		"size":          len(contentBytes),
		"content_bytes": len(contentBytes),
		"count":         len(canonicalRules),
		"exists":        len(canonicalRules) > 0,
	}, nil
}

func parsePluginHostPageRulePackInput(params map[string]interface{}) ([]config.PageRule, error) {
	for _, key := range []string{"rules", "page_rules", "pageRules"} {
		if raw, exists := params[key]; exists && raw != nil {
			return normalizePluginHostPageRulesValue(raw)
		}
	}

	content := parsePluginHostOptionalString(params, "content", "json_content", "jsonContent", "rules_json", "rulesJson")
	if strings.TrimSpace(content) == "" {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "content/json_content/jsonContent/rules or page_rules is required"}
	}
	return parsePluginHostPageRulePackContent([]byte(content))
}

func normalizePluginHostPageRulesValue(value interface{}) ([]config.PageRule, error) {
	switch typed := value.(type) {
	case string:
		return parsePluginHostPageRulePackContent([]byte(typed))
	case []config.PageRule:
		return normalizePluginHostPageRules(typed)
	case []interface{}:
		body, err := json.Marshal(typed)
		if err != nil {
			return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "page rule pack rules payload is invalid"}
		}
		return parsePluginHostPageRulePackContent(body)
	default:
		body, err := json.Marshal(typed)
		if err != nil {
			return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "page rule pack rules payload is invalid"}
		}
		return parsePluginHostPageRulePackContent(body)
	}
}

func parsePluginHostPageRulePackContent(raw []byte) ([]config.PageRule, error) {
	trimmed := bytes.TrimSpace(bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF}))
	if len(trimmed) == 0 {
		return []config.PageRule{}, nil
	}

	var rules []config.PageRule
	if err := json.Unmarshal(trimmed, &rules); err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: fmt.Sprintf("page rule pack content is invalid: %v", err)}
	}
	return normalizePluginHostPageRules(rules)
}

func normalizePluginHostPageRules(rules []config.PageRule) ([]config.PageRule, error) {
	if len(rules) == 0 {
		return []config.PageRule{}, nil
	}

	normalized := make([]config.PageRule, 0, len(rules))
	for idx, rule := range rules {
		matchType := strings.ToLower(strings.TrimSpace(rule.MatchType))
		if matchType == "" {
			matchType = "exact"
		}
		if matchType != "exact" && matchType != "regex" {
			return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: fmt.Sprintf("page rule %d match_type must be exact or regex", idx)}
		}
		pattern := strings.TrimSpace(rule.Pattern)
		if pattern == "" {
			return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: fmt.Sprintf("page rule %d pattern is required", idx)}
		}
		normalized = append(normalized, config.PageRule{
			Name:      strings.TrimSpace(rule.Name),
			Pattern:   pattern,
			MatchType: matchType,
			CSS:       rule.CSS,
			JS:        rule.JS,
			Enabled:   rule.Enabled,
		})
	}
	return normalized, nil
}

func marshalPluginHostPageRules(rules []config.PageRule) ([]byte, error) {
	normalized, err := normalizePluginHostPageRules(rules)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(normalized)
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return []byte("[]"), nil
	}
	return body, nil
}

func pluginHostPageRulesFromConfig(cfg *config.Config) []config.PageRule {
	if cfg == nil {
		return []config.PageRule{}
	}
	rules, err := normalizePluginHostPageRules(cfg.Customization.PageRules)
	if err != nil {
		return []config.PageRule{}
	}
	return rules
}

func clonePluginHostPageRules(rules []config.PageRule) []config.PageRule {
	if len(rules) == 0 {
		return []config.PageRule{}
	}
	cloned := make([]config.PageRule, 0, len(rules))
	cloned = append(cloned, rules...)
	return cloned
}
