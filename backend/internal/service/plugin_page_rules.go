package service

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"auralogic/internal/config"
	"auralogic/internal/models"
	"gorm.io/gorm"
)

const defaultPluginPageRulePriority = 100

type PageInjectMatchedRule struct {
	Source     string `json:"source,omitempty"`
	Name       string `json:"name,omitempty"`
	Pattern    string `json:"pattern,omitempty"`
	MatchType  string `json:"match_type,omitempty"`
	CSS        string `json:"css,omitempty"`
	JS         string `json:"js,omitempty"`
	PluginID   uint   `json:"plugin_id,omitempty"`
	PluginName string `json:"plugin_name,omitempty"`
	Namespace  string `json:"namespace,omitempty"`
	Key        string `json:"key,omitempty"`
	Priority   int    `json:"priority,omitempty"`
}

type PageInjectPayload struct {
	Path         string                  `json:"path"`
	CSS          string                  `json:"css"`
	JS           string                  `json:"js"`
	Rules        []PageInjectMatchedRule `json:"rules"`
	MatchedCount int                     `json:"matched_count"`
}

type pluginPageRuleUpsertInput struct {
	Key       string
	Name      string
	Pattern   string
	MatchType string
	CSS       string
	JS        string
	Enabled   bool
	Priority  int
}

func ResolvePageInjectPayload(db *gorm.DB, cfg *config.Config, pagePath string) (PageInjectPayload, error) {
	normalizedPath := strings.TrimSpace(pagePath)
	if normalizedPath == "" {
		normalizedPath = "/"
	}

	matchedRules := collectMatchedHostPageInjectRules(cfg, normalizedPath)
	pluginRules, err := collectMatchedPluginPageInjectRules(db, normalizedPath)
	if len(pluginRules) > 0 {
		matchedRules = append(matchedRules, pluginRules...)
	}

	return buildPageInjectPayload(normalizedPath, matchedRules), err
}

func collectMatchedHostPageInjectRules(cfg *config.Config, pagePath string) []PageInjectMatchedRule {
	if cfg == nil {
		return []PageInjectMatchedRule{}
	}

	matched := make([]PageInjectMatchedRule, 0)
	for _, rule := range cfg.Customization.PageRules {
		if !rule.Enabled || !pageInjectRuleMatches(pagePath, rule.Pattern, rule.MatchType) {
			continue
		}
		matched = append(matched, PageInjectMatchedRule{
			Source:    "host",
			Name:      strings.TrimSpace(rule.Name),
			Pattern:   strings.TrimSpace(rule.Pattern),
			MatchType: normalizePageInjectMatchType(rule.MatchType),
			CSS:       rule.CSS,
			JS:        rule.JS,
		})
	}
	return matched
}

func collectMatchedPluginPageInjectRules(db *gorm.DB, pagePath string) ([]PageInjectMatchedRule, error) {
	if db == nil {
		return []PageInjectMatchedRule{}, nil
	}

	var entries []models.PluginPageRuleEntry
	if err := db.Preload("Plugin").
		Where("enabled = ?", true).
		Order("priority ASC").
		Order("id ASC").
		Find(&entries).Error; err != nil {
		return []PageInjectMatchedRule{}, err
	}

	matched := make([]PageInjectMatchedRule, 0)
	for _, entry := range entries {
		if entry.Plugin == nil || !shouldInjectPluginPageRules(entry.Plugin) {
			continue
		}
		if !pageInjectRuleMatches(pagePath, entry.Pattern, entry.MatchType) {
			continue
		}
		matched = append(matched, PageInjectMatchedRule{
			Source:     "plugin",
			Name:       strings.TrimSpace(entry.Name),
			Pattern:    strings.TrimSpace(entry.Pattern),
			MatchType:  normalizePageInjectMatchType(entry.MatchType),
			CSS:        entry.CSS,
			JS:         entry.JS,
			PluginID:   entry.PluginID,
			PluginName: strings.TrimSpace(entry.Plugin.Name),
			Namespace:  pluginPageRuleNamespace(entry.Plugin),
			Key:        strings.TrimSpace(entry.Key),
			Priority:   entry.Priority,
		})
	}

	sort.SliceStable(matched, func(i, j int) bool {
		if matched[i].Priority != matched[j].Priority {
			return matched[i].Priority < matched[j].Priority
		}
		if matched[i].Namespace != matched[j].Namespace {
			return matched[i].Namespace < matched[j].Namespace
		}
		if matched[i].Key != matched[j].Key {
			return matched[i].Key < matched[j].Key
		}
		return matched[i].PluginID < matched[j].PluginID
	})

	return matched, nil
}

func buildPageInjectPayload(pagePath string, rules []PageInjectMatchedRule) PageInjectPayload {
	payload := PageInjectPayload{
		Path:         pagePath,
		CSS:          "",
		JS:           "",
		Rules:        append([]PageInjectMatchedRule(nil), rules...),
		MatchedCount: len(rules),
	}
	var cssBuilder strings.Builder
	var jsBuilder strings.Builder
	for _, rule := range rules {
		if rule.CSS != "" {
			cssBuilder.WriteString(rule.CSS)
			cssBuilder.WriteByte('\n')
		}
		if rule.JS != "" {
			jsBuilder.WriteString(rule.JS)
			jsBuilder.WriteByte('\n')
		}
	}
	payload.CSS = cssBuilder.String()
	payload.JS = jsBuilder.String()
	return payload
}

func pageInjectRuleMatches(pagePath string, pattern string, matchType string) bool {
	normalizedPattern := strings.TrimSpace(pattern)
	if normalizedPattern == "" {
		return false
	}
	if normalizePageInjectMatchType(matchType) == "regex" {
		re, err := regexp.Compile(normalizedPattern)
		if err != nil {
			return false
		}
		return re.MatchString(pagePath)
	}
	return pagePath == normalizedPattern
}

func normalizePageInjectMatchType(matchType string) string {
	switch strings.ToLower(strings.TrimSpace(matchType)) {
	case "regex":
		return "regex"
	default:
		return "exact"
	}
}

func shouldInjectPluginPageRules(plugin *models.Plugin) bool {
	if plugin == nil || !plugin.Enabled {
		return false
	}

	switch strings.TrimSpace(plugin.LifecycleStatus) {
	case models.PluginLifecycleInstalled, models.PluginLifecycleRunning, models.PluginLifecycleDegraded:
	default:
		return false
	}

	policy := ResolveEffectivePluginCapabilityPolicy(plugin)
	return IsPluginPermissionGranted(
		policy.RequestedPermissions,
		policy.GrantedPermissions,
		PluginPermissionHostPluginPageRuleWrite,
	)
}

func executePluginHostPluginPageRuleList(
	runtime *PluginHostRuntime,
	claims *PluginHostAccessClaims,
	params map[string]interface{},
) (map[string]interface{}, error) {
	plugin, err := loadPluginForPluginPageRuleAction(runtime, claims)
	if err != nil {
		return nil, err
	}

	rules, err := listPluginPageRules(runtime.database(), plugin.ID)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "list plugin page rules failed"}
	}

	items := make([]map[string]interface{}, 0, len(rules))
	for _, rule := range rules {
		items = append(items, buildPluginPageRuleEntryResponse(rule, plugin))
	}

	return map[string]interface{}{
		"plugin_id":   plugin.ID,
		"plugin_name": strings.TrimSpace(plugin.Name),
		"namespace":   pluginPageRuleNamespace(plugin),
		"rules":       items,
		"count":       len(items),
		"exists":      len(items) > 0,
	}, nil
}

func executePluginHostPluginPageRuleGet(
	runtime *PluginHostRuntime,
	claims *PluginHostAccessClaims,
	params map[string]interface{},
) (map[string]interface{}, error) {
	plugin, err := loadPluginForPluginPageRuleAction(runtime, claims)
	if err != nil {
		return nil, err
	}

	key := strings.TrimSpace(parsePluginHostOptionalString(params, "key"))
	if key == "" {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "key is required"}
	}

	rule, err := getPluginPageRule(runtime.database(), plugin.ID, key)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &PluginHostActionError{Status: http.StatusNotFound, Message: "plugin page rule not found"}
		}
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query plugin page rule failed"}
	}

	return map[string]interface{}{
		"plugin_id":   plugin.ID,
		"plugin_name": strings.TrimSpace(plugin.Name),
		"namespace":   pluginPageRuleNamespace(plugin),
		"rule":        buildPluginPageRuleEntryResponse(rule, plugin),
	}, nil
}

func executePluginHostPluginPageRuleUpsert(
	runtime *PluginHostRuntime,
	claims *PluginHostAccessClaims,
	params map[string]interface{},
) (map[string]interface{}, error) {
	plugin, err := loadPluginForPluginPageRuleAction(runtime, claims)
	if err != nil {
		return nil, err
	}

	input, err := parsePluginPageRuleUpsertInput(params)
	if err != nil {
		return nil, err
	}

	db := runtime.database()
	if db == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "database is unavailable"}
	}

	entry := models.PluginPageRuleEntry{
		PluginID:  plugin.ID,
		Key:       input.Key,
		Name:      input.Name,
		Pattern:   input.Pattern,
		MatchType: input.MatchType,
		CSS:       input.CSS,
		JS:        input.JS,
		Enabled:   input.Enabled,
		Priority:  input.Priority,
	}

	var existing models.PluginPageRuleEntry
	err = db.Where("plugin_id = ? AND key = ?", plugin.ID, input.Key).First(&existing).Error
	created := false
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		created = true
		if err := db.Create(&entry).Error; err != nil {
			return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "create plugin page rule failed"}
		}
	case err != nil:
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query plugin page rule failed"}
	default:
		existing.Name = entry.Name
		existing.Pattern = entry.Pattern
		existing.MatchType = entry.MatchType
		existing.CSS = entry.CSS
		existing.JS = entry.JS
		existing.Enabled = entry.Enabled
		existing.Priority = entry.Priority
		if err := db.Save(&existing).Error; err != nil {
			return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "update plugin page rule failed"}
		}
		entry = existing
	}

	if created {
		if err := db.First(&entry, entry.ID).Error; err != nil {
			return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "reload plugin page rule failed"}
		}
	}

	return map[string]interface{}{
		"plugin_id":   plugin.ID,
		"plugin_name": strings.TrimSpace(plugin.Name),
		"namespace":   pluginPageRuleNamespace(plugin),
		"saved":       true,
		"created":     created,
		"rule":        buildPluginPageRuleEntryResponse(entry, plugin),
	}, nil
}

func executePluginHostPluginPageRuleDelete(
	runtime *PluginHostRuntime,
	claims *PluginHostAccessClaims,
	params map[string]interface{},
) (map[string]interface{}, error) {
	plugin, err := loadPluginForPluginPageRuleAction(runtime, claims)
	if err != nil {
		return nil, err
	}

	key := strings.TrimSpace(parsePluginHostOptionalString(params, "key"))
	if key == "" {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "key is required"}
	}

	db := runtime.database()
	if db == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "database is unavailable"}
	}

	result := db.Where("plugin_id = ? AND key = ?", plugin.ID, key).Delete(&models.PluginPageRuleEntry{})
	if result.Error != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "delete plugin page rule failed"}
	}

	return map[string]interface{}{
		"plugin_id":     plugin.ID,
		"plugin_name":   strings.TrimSpace(plugin.Name),
		"namespace":     pluginPageRuleNamespace(plugin),
		"key":           key,
		"deleted":       result.RowsAffected > 0,
		"deleted_count": result.RowsAffected,
	}, nil
}

func executePluginHostPluginPageRuleReset(
	runtime *PluginHostRuntime,
	claims *PluginHostAccessClaims,
	params map[string]interface{},
) (map[string]interface{}, error) {
	plugin, err := loadPluginForPluginPageRuleAction(runtime, claims)
	if err != nil {
		return nil, err
	}

	db := runtime.database()
	if db == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "database is unavailable"}
	}

	result := db.Where("plugin_id = ?", plugin.ID).Delete(&models.PluginPageRuleEntry{})
	if result.Error != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "reset plugin page rules failed"}
	}

	return map[string]interface{}{
		"plugin_id":     plugin.ID,
		"plugin_name":   strings.TrimSpace(plugin.Name),
		"namespace":     pluginPageRuleNamespace(plugin),
		"saved":         true,
		"reset":         true,
		"deleted_count": result.RowsAffected,
	}, nil
}

func loadPluginForPluginPageRuleAction(runtime *PluginHostRuntime, claims *PluginHostAccessClaims) (*models.Plugin, error) {
	if runtime == nil || runtime.database() == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "database is unavailable"}
	}
	if claims == nil || claims.PluginID == 0 {
		return nil, &PluginHostActionError{Status: http.StatusForbidden, Message: "plugin identity is required"}
	}

	var plugin models.Plugin
	if err := runtime.database().First(&plugin, claims.PluginID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &PluginHostActionError{Status: http.StatusNotFound, Message: "plugin not found"}
		}
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query plugin failed"}
	}
	return &plugin, nil
}

func listPluginPageRules(db *gorm.DB, pluginID uint) ([]models.PluginPageRuleEntry, error) {
	if db == nil {
		return []models.PluginPageRuleEntry{}, nil
	}

	var rules []models.PluginPageRuleEntry
	if err := db.Where("plugin_id = ?", pluginID).
		Order("priority ASC").
		Order("key ASC").
		Order("id ASC").
		Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}

func getPluginPageRule(db *gorm.DB, pluginID uint, key string) (models.PluginPageRuleEntry, error) {
	var rule models.PluginPageRuleEntry
	if db == nil {
		return rule, gorm.ErrRecordNotFound
	}
	err := db.Where("plugin_id = ? AND key = ?", pluginID, key).First(&rule).Error
	return rule, err
}

func buildPluginPageRuleEntryResponse(entry models.PluginPageRuleEntry, plugin *models.Plugin) map[string]interface{} {
	return map[string]interface{}{
		"id":          entry.ID,
		"plugin_id":   entry.PluginID,
		"plugin_name": strings.TrimSpace(pluginNameFromRule(entry, plugin)),
		"namespace":   pluginNamespaceFromRule(entry, plugin),
		"key":         strings.TrimSpace(entry.Key),
		"name":        strings.TrimSpace(entry.Name),
		"pattern":     strings.TrimSpace(entry.Pattern),
		"match_type":  normalizePageInjectMatchType(entry.MatchType),
		"css":         entry.CSS,
		"js":          entry.JS,
		"enabled":     entry.Enabled,
		"priority":    entry.Priority,
		"created_at":  entry.CreatedAt,
		"updated_at":  entry.UpdatedAt,
	}
}

func pluginNameFromRule(entry models.PluginPageRuleEntry, plugin *models.Plugin) string {
	if plugin != nil && strings.TrimSpace(plugin.Name) != "" {
		return strings.TrimSpace(plugin.Name)
	}
	if entry.Plugin != nil && strings.TrimSpace(entry.Plugin.Name) != "" {
		return strings.TrimSpace(entry.Plugin.Name)
	}
	return fmt.Sprintf("plugin-%d", entry.PluginID)
}

func pluginPageRuleNamespace(plugin *models.Plugin) string {
	if plugin == nil {
		return ""
	}
	name := strings.TrimSpace(plugin.Name)
	if name == "" {
		return fmt.Sprintf("plugin-%d", plugin.ID)
	}
	return name
}

func pluginNamespaceFromRule(entry models.PluginPageRuleEntry, plugin *models.Plugin) string {
	if namespace := pluginPageRuleNamespace(plugin); namespace != "" {
		return namespace
	}
	if entry.Plugin != nil {
		return pluginPageRuleNamespace(entry.Plugin)
	}
	return fmt.Sprintf("plugin-%d", entry.PluginID)
}

func parsePluginPageRuleUpsertInput(params map[string]interface{}) (pluginPageRuleUpsertInput, error) {
	key := strings.TrimSpace(parsePluginHostOptionalString(params, "key"))
	if key == "" {
		return pluginPageRuleUpsertInput{}, &PluginHostActionError{Status: http.StatusBadRequest, Message: "key is required"}
	}
	if len(key) > 191 {
		return pluginPageRuleUpsertInput{}, &PluginHostActionError{Status: http.StatusBadRequest, Message: "key exceeds max length 191"}
	}

	pattern := strings.TrimSpace(parsePluginHostOptionalString(params, "pattern"))
	if pattern == "" {
		return pluginPageRuleUpsertInput{}, &PluginHostActionError{Status: http.StatusBadRequest, Message: "pattern is required"}
	}

	priority := defaultPluginPageRulePriority
	if parsedPriority, ok, err := parsePluginHostOptionalInt64(params, "priority"); err != nil {
		return pluginPageRuleUpsertInput{}, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	} else if ok {
		if parsedPriority < -2147483648 || parsedPriority > 2147483647 {
			return pluginPageRuleUpsertInput{}, &PluginHostActionError{Status: http.StatusBadRequest, Message: "priority is out of range"}
		}
		priority = int(parsedPriority)
	}

	enabled := true
	if parsedEnabled, err := parsePluginHostOptionalBool(params, "enabled"); err != nil {
		return pluginPageRuleUpsertInput{}, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	} else if parsedEnabled != nil {
		enabled = *parsedEnabled
	}

	name := strings.TrimSpace(parsePluginHostOptionalString(params, "name"))
	matchType := strings.TrimSpace(parsePluginHostOptionalString(params, "match_type", "matchType"))
	css := parsePluginHostOptionalString(params, "css")
	js := parsePluginHostOptionalString(params, "js")

	normalizedRules, err := normalizePluginHostPageRules([]config.PageRule{{
		Name:      name,
		Pattern:   pattern,
		MatchType: matchType,
		CSS:       css,
		JS:        js,
		Enabled:   enabled,
	}})
	if err != nil {
		return pluginPageRuleUpsertInput{}, err
	}
	rule := normalizedRules[0]

	return pluginPageRuleUpsertInput{
		Key:       key,
		Name:      rule.Name,
		Pattern:   rule.Pattern,
		MatchType: rule.MatchType,
		CSS:       rule.CSS,
		JS:        rule.JS,
		Enabled:   rule.Enabled,
		Priority:  priority,
	}, nil
}
