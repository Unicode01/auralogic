package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"auralogic/internal/config"
	"auralogic/internal/models"
	"gorm.io/gorm"
)

func executePluginHostMarketSourceList(
	db *gorm.DB,
	claims *PluginHostAccessClaims,
) (map[string]interface{}, error) {
	plugin, err := pluginHostLoadPluginForScopedConfig(db, claims)
	if err != nil {
		return nil, err
	}
	sources, err := loadPluginMarketSourcesFromPlugin(plugin)
	if err != nil {
		return nil, err
	}

	items := make([]map[string]interface{}, 0, len(sources))
	for _, source := range sources {
		items = append(items, source.Summary())
	}
	return map[string]interface{}{
		"items": items,
	}, nil
}

func executePluginHostMarketSourceGet(
	db *gorm.DB,
	claims *PluginHostAccessClaims,
	params map[string]interface{},
) (map[string]interface{}, error) {
	plugin, err := pluginHostLoadPluginForScopedConfig(db, claims)
	if err != nil {
		return nil, err
	}
	source, err := resolvePluginMarketSourceForParams(plugin, params)
	if err != nil {
		return nil, err
	}
	return source.Summary(), nil
}

func executePluginHostMarketCatalogList(
	db *gorm.DB,
	claims *PluginHostAccessClaims,
	params map[string]interface{},
) (map[string]interface{}, error) {
	plugin, err := pluginHostLoadPluginForScopedConfig(db, claims)
	if err != nil {
		return nil, err
	}
	source, err := resolvePluginMarketSourceForParams(plugin, params)
	if err != nil {
		return nil, err
	}

	client := newPluginMarketSourceClient()
	payload, err := client.FetchCatalog(context.Background(), source, params)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadGateway, Message: err.Error()}
	}
	filterPluginMarketCatalogItemsBySource(payload, source)
	payload["source"] = source.Summary()
	return payload, nil
}

func executePluginHostMarketArtifactGet(
	db *gorm.DB,
	claims *PluginHostAccessClaims,
	params map[string]interface{},
) (map[string]interface{}, error) {
	return executePluginHostMarketRemoteDetailAction(db, claims, params, false)
}

func executePluginHostMarketReleaseGet(
	db *gorm.DB,
	claims *PluginHostAccessClaims,
	params map[string]interface{},
) (map[string]interface{}, error) {
	return executePluginHostMarketRemoteDetailAction(db, claims, params, true)
}

func executePluginHostMarketInstallPreview(
	db *gorm.DB,
	claims *PluginHostAccessClaims,
	params map[string]interface{},
) (map[string]interface{}, error) {
	plugin, err := pluginHostLoadPluginForScopedConfig(db, claims)
	if err != nil {
		return nil, err
	}
	source, kind, name, version, err := resolvePluginMarketCoordinates(plugin, params)
	if err != nil {
		return nil, err
	}

	client := newPluginMarketSourceClient()
	release, err := client.FetchRelease(context.Background(), source, kind, name, version)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadGateway, Message: err.Error()}
	}

	compatibility, compatibilityWarnings := buildPluginMarketCompatibilityPreview(release)
	response := map[string]interface{}{
		"source": source.Summary(),
		"coordinates": map[string]interface{}{
			"source_id": source.SourceID,
			"kind":      kind,
			"name":      name,
			"version":   version,
		},
		"release":       release,
		"governance":    clonePluginMarketMap(release["governance"]),
		"download":      clonePluginMarketMap(release["download"]),
		"compatibility": compatibility,
		"warnings":      compatibilityWarnings,
		"target_state": map[string]interface{}{
			"installed":           false,
			"current_version":     "",
			"update_available":    false,
			"installed_target":    "",
			"installed_target_id": nil,
		},
	}

	switch kind {
	case "plugin_package":
		targetState, permissionPreview, warnings, previewErr := buildPluginMarketPluginPreview(db, name, version, release)
		if previewErr != nil {
			return nil, previewErr
		}
		response["target_state"] = targetState
		response["permissions"] = permissionPreview
		if len(warnings) > 0 {
			response["warnings"] = append(clonePluginMarketStringSlice(compatibilityWarnings), warnings...)
		}
	case "payment_package":
		preview, previewErr := previewPaymentMethodMarketPackageRelease(
			NewPluginHostRuntime(db, nil, nil),
			source,
			name,
			version,
			release,
			0,
		)
		if previewErr != nil {
			return nil, previewErr
		}
		return preview, nil
	case "email_template", "landing_page_template", "invoice_template", "auth_branding_template", "page_rule_pack":
		preview, previewErr := previewTemplateMarketRelease(
			NewPluginHostRuntime(db, config.GetConfig(), nil),
			source,
			kind,
			name,
			version,
			release,
			params,
		)
		if previewErr != nil {
			return nil, previewErr
		}
		return preview, nil
	default:
		response["target_state"] = map[string]interface{}{
			"installed":           false,
			"current_version":     "",
			"update_available":    false,
			"installed_target":    kind,
			"installed_target_id": nil,
		}
	}

	return response, nil
}

func executePluginHostMarketRemoteDetailAction(
	db *gorm.DB,
	claims *PluginHostAccessClaims,
	params map[string]interface{},
	release bool,
) (map[string]interface{}, error) {
	plugin, err := pluginHostLoadPluginForScopedConfig(db, claims)
	if err != nil {
		return nil, err
	}
	source, kind, name, version, err := resolvePluginMarketCoordinates(plugin, params)
	if err != nil {
		return nil, err
	}

	client := newPluginMarketSourceClient()
	var payload map[string]interface{}
	if release {
		if strings.TrimSpace(version) == "" {
			return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "version is required"}
		}
		payload, err = client.FetchRelease(context.Background(), source, kind, name, version)
	} else {
		payload, err = client.FetchArtifact(context.Background(), source, kind, name)
	}
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadGateway, Message: err.Error()}
	}
	payload["source"] = source.Summary()
	return payload, nil
}

func pluginHostLoadPluginForScopedConfig(db *gorm.DB, claims *PluginHostAccessClaims) (*models.Plugin, error) {
	if db == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "plugin host database is unavailable"}
	}
	if claims == nil || claims.PluginID == 0 {
		return nil, &PluginHostActionError{Status: http.StatusUnauthorized, Message: "plugin host plugin context is unavailable"}
	}

	var plugin models.Plugin
	if err := db.First(&plugin, claims.PluginID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &PluginHostActionError{Status: http.StatusNotFound, Message: "plugin not found"}
		}
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query plugin failed"}
	}
	return &plugin, nil
}

func loadPluginMarketSourcesFromPlugin(plugin *models.Plugin) ([]PluginMarketSource, error) {
	if plugin == nil {
		return nil, &PluginHostActionError{Status: http.StatusUnauthorized, Message: "plugin context is required"}
	}

	configRaw := strings.TrimSpace(plugin.Config)
	if configRaw == "" {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "plugin market source configuration is empty"}
	}

	var root map[string]interface{}
	if err := json.Unmarshal([]byte(configRaw), &root); err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "plugin market source configuration is invalid"}
	}

	candidates := collectPluginMarketSourceCandidates(root)
	if len(candidates) == 0 {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "plugin market source configuration is empty"}
	}

	seen := make(map[string]struct{}, len(candidates))
	sources := make([]PluginMarketSource, 0, len(candidates))
	for _, candidate := range candidates {
		source, ok := buildPluginMarketSourceFromMap(candidate)
		if !ok || !source.Enabled {
			continue
		}
		dedupKey := strings.ToLower(strings.TrimSpace(source.SourceID)) + "|" + strings.ToLower(strings.TrimSpace(source.BaseURL))
		if _, exists := seen[dedupKey]; exists {
			continue
		}
		seen[dedupKey] = struct{}{}
		sources = append(sources, source)
	}
	if len(sources) == 0 {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "plugin market source configuration is empty"}
	}

	sort.SliceStable(sources, func(i int, j int) bool {
		left := strings.ToLower(strings.TrimSpace(sources[i].SourceID))
		right := strings.ToLower(strings.TrimSpace(sources[j].SourceID))
		if left == right {
			return strings.ToLower(strings.TrimSpace(sources[i].BaseURL)) < strings.ToLower(strings.TrimSpace(sources[j].BaseURL))
		}
		return left < right
	})
	return sources, nil
}

func collectPluginMarketSourceCandidates(root map[string]interface{}) []map[string]interface{} {
	candidates := make([]map[string]interface{}, 0, 4)
	var appendCandidate func(value interface{})
	appendCandidate = func(value interface{}) {
		switch typed := value.(type) {
		case map[string]interface{}:
			if hasPluginMarketSourceFields(typed) {
				candidates = append(candidates, typed)
			}
			appendCandidate(typed["sources"])
			appendCandidate(typed["market_sources"])
			appendCandidate(typed["market_source"])
			appendCandidate(typed["source"])
		case []interface{}:
			for _, item := range typed {
				if mapped, ok := item.(map[string]interface{}); ok {
					appendCandidate(mapped)
				}
			}
		}
	}

	appendCandidate(root["market"])
	appendCandidate(root["market_source"])
	appendCandidate(root["source"])
	appendCandidate(root["sources"])
	appendCandidate(root["market_sources"])

	if len(candidates) > 0 {
		return candidates
	}
	if hasPluginMarketSourceFields(root) {
		return []map[string]interface{}{root}
	}
	return []map[string]interface{}{}
}

func hasPluginMarketSourceFields(values map[string]interface{}) bool {
	if values == nil {
		return false
	}
	keys := []string{
		"source_base_url",
		"base_url",
		"baseUrl",
		"sourceBaseURL",
	}
	for _, key := range keys {
		if strings.TrimSpace(pluginMarketStringFromAny(values[key])) != "" {
			return true
		}
	}
	return false
}

func buildPluginMarketSourceFromMap(values map[string]interface{}) (PluginMarketSource, bool) {
	if values == nil {
		return PluginMarketSource{}, false
	}

	baseURL := firstPluginMarketString(values,
		"source_base_url",
		"base_url",
		"baseUrl",
		"sourceBaseURL",
	)
	if strings.TrimSpace(baseURL) == "" {
		return PluginMarketSource{}, false
	}

	sourceID := firstPluginMarketString(values, "source_id", "sourceId")
	if sourceID == "" {
		if parsed, err := url.Parse(baseURL); err == nil {
			sourceID = strings.ToLower(strings.TrimSpace(parsed.Hostname()))
		}
	}
	if sourceID == "" {
		sourceID = "default"
	}

	name := firstPluginMarketString(values, "name", "display_name", "displayName")
	if name == "" {
		name = sourceID
	}
	defaultChannel := firstPluginMarketString(values, "default_channel", "defaultChannel")
	if defaultChannel == "" {
		defaultChannel = "stable"
	}
	enabled := true
	if parsed, ok := pluginMarketBoolFromAny(values["enabled"]); ok {
		enabled = parsed
	}

	source := PluginMarketSource{
		SourceID:       strings.ToLower(strings.TrimSpace(sourceID)),
		Name:           name,
		BaseURL:        strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		PublicKey:      firstPluginMarketString(values, "source_public_key", "public_key", "publicKey", "sourcePublicKey"),
		DefaultChannel: strings.ToLower(strings.TrimSpace(defaultChannel)),
		AllowedKinds: normalizePluginMarketArtifactKinds(
			firstPluginMarketStringSlice(values,
				"allowed_kinds",
				"allowedKinds",
			),
		),
		Enabled: enabled,
	}
	if source.SourceID == "" || source.BaseURL == "" {
		return PluginMarketSource{}, false
	}
	return source, true
}

func firstPluginMarketString(values map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		value := strings.TrimSpace(pluginMarketStringFromAny(values[key]))
		if value != "" && value != "<nil>" {
			return value
		}
	}
	return ""
}

func firstPluginMarketStringSlice(values map[string]interface{}, keys ...string) []string {
	for _, key := range keys {
		raw, exists := values[key]
		if !exists || raw == nil {
			continue
		}
		switch typed := raw.(type) {
		case []string:
			return typed
		case []interface{}:
			out := make([]string, 0, len(typed))
			for _, item := range typed {
				value := strings.TrimSpace(pluginMarketStringFromAny(item))
				if value == "" || value == "<nil>" {
					continue
				}
				out = append(out, value)
			}
			return out
		case string:
			value := strings.TrimSpace(typed)
			if value == "" {
				return nil
			}
			return strings.FieldsFunc(value, func(r rune) bool {
				return r == ',' || r == ';'
			})
		}
	}
	return nil
}

func pluginMarketBoolFromAny(value interface{}) (bool, bool) {
	switch typed := value.(type) {
	case bool:
		return typed, true
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "1", "true", "yes", "on":
			return true, true
		case "0", "false", "no", "off":
			return false, true
		}
	}
	return false, false
}

func resolvePluginMarketSourceForParams(plugin *models.Plugin, params map[string]interface{}) (PluginMarketSource, error) {
	sources, err := loadPluginMarketSourcesFromPlugin(plugin)
	if err != nil {
		return PluginMarketSource{}, err
	}

	requestedID := strings.ToLower(strings.TrimSpace(parsePluginHostOptionalString(params, "source_id", "sourceId")))
	if requestedID == "" {
		if len(sources) == 1 {
			return sources[0], nil
		}
		return PluginMarketSource{}, &PluginHostActionError{Status: http.StatusBadRequest, Message: "source_id/sourceId is required"}
	}

	for _, source := range sources {
		if strings.EqualFold(source.SourceID, requestedID) {
			return source, nil
		}
	}
	return PluginMarketSource{}, &PluginHostActionError{Status: http.StatusNotFound, Message: "market source not found"}
}

func resolvePluginMarketCoordinates(plugin *models.Plugin, params map[string]interface{}) (PluginMarketSource, string, string, string, error) {
	source, err := resolvePluginMarketSourceForParams(plugin, params)
	if err != nil {
		return PluginMarketSource{}, "", "", "", err
	}
	kind := normalizePluginMarketArtifactKind(parsePluginHostOptionalString(params, "kind"))
	if kind == "" {
		return PluginMarketSource{}, "", "", "", &PluginHostActionError{Status: http.StatusBadRequest, Message: "kind is required"}
	}
	if !source.AllowsKind(kind) {
		return PluginMarketSource{}, "", "", "", &PluginHostActionError{Status: http.StatusForbidden, Message: "market artifact kind is not allowed for this source"}
	}
	name := strings.TrimSpace(parsePluginHostOptionalString(params, "name"))
	if name == "" {
		return PluginMarketSource{}, "", "", "", &PluginHostActionError{Status: http.StatusBadRequest, Message: "name is required"}
	}
	version := strings.TrimSpace(parsePluginHostOptionalString(params, "version"))
	return source, kind, name, version, nil
}

func filterPluginMarketCatalogItemsBySource(payload map[string]interface{}, source PluginMarketSource) {
	if payload == nil {
		return
	}
	itemsRaw, ok := payload["items"].([]interface{})
	if !ok {
		return
	}
	filtered := make([]interface{}, 0, len(itemsRaw))
	for _, item := range itemsRaw {
		mapped, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		kind := normalizePluginMarketArtifactKind(pluginMarketStringFromAny(mapped["kind"]))
		if kind == "" || !source.AllowsKind(kind) {
			continue
		}
		filtered = append(filtered, mapped)
	}
	if len(filtered) == len(itemsRaw) {
		return
	}
	payload["items"] = filtered
	if pagination, ok := payload["pagination"].(map[string]interface{}); ok {
		pagination["total"] = len(filtered)
		pagination["has_more"] = false
	}
}

func buildPluginMarketCompatibilityPreview(release map[string]interface{}) (map[string]interface{}, []string) {
	compatibility := clonePluginMarketMap(release["compatibility"])
	warnings := make([]string, 0, 2)
	compatible := true

	minBridgeVersion := pluginMarketStringFromAny(compatibility["min_host_bridge_version"])
	if minBridgeVersion != "" && comparePluginMarketSemver(pluginHostMarketBridgeVersion, minBridgeVersion) < 0 {
		compatible = false
		warnings = append(warnings, fmt.Sprintf("requires host bridge version %s or later", minBridgeVersion))
	}
	compatibility["compatible"] = compatible
	compatibility["host_bridge_version"] = pluginHostMarketBridgeVersion
	compatibility["warnings"] = clonePluginMarketStringSlice(warnings)
	return compatibility, warnings
}

func buildPluginMarketPluginPreview(
	db *gorm.DB,
	name string,
	version string,
	release map[string]interface{},
) (map[string]interface{}, map[string]interface{}, []string, error) {
	targetState := map[string]interface{}{
		"installed":           false,
		"current_version":     "",
		"update_available":    false,
		"installed_target":    "plugin",
		"installed_target_id": nil,
	}
	warnings := make([]string, 0, 1)

	var localPlugin models.Plugin
	if db != nil {
		if err := db.Where("name = ?", name).First(&localPlugin).Error; err == nil {
			targetState["installed"] = true
			targetState["current_version"] = strings.TrimSpace(localPlugin.Version)
			targetState["installed_target_id"] = localPlugin.ID
			targetState["update_available"] = strings.TrimSpace(localPlugin.Version) != "" && strings.TrimSpace(localPlugin.Version) != strings.TrimSpace(version)
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query local plugin version failed"}
		}
	}

	permissionsRaw := clonePluginMarketMap(release["permissions"])
	requested := NormalizePluginPermissionList(pluginMarketInterfaceSliceToStrings(permissionsRaw["requested"]))
	defaultGranted := NormalizePluginPermissionList(pluginMarketInterfaceSliceToStrings(permissionsRaw["default_granted"]))
	currentRequested := []string{}
	if localPlugin.ID > 0 {
		currentRequested = ResolveEffectivePluginCapabilityPolicy(&localPlugin).RequestedPermissions
	}
	newPermissions := pluginMarketStringSliceDiff(requested, currentRequested)
	requiresReconfirm := len(requested) > 0 && (localPlugin.ID == 0 || len(newPermissions) > 0 || strings.TrimSpace(localPlugin.Version) != strings.TrimSpace(version))
	if len(requested) == 0 {
		warnings = append(warnings, "plugin release did not declare requested permissions")
	}

	permissionPreview := map[string]interface{}{
		"requested":          requested,
		"default_granted":    defaultGranted,
		"current_requested":  currentRequested,
		"new_permissions":    newPermissions,
		"requires_reconfirm": requiresReconfirm,
	}
	return targetState, permissionPreview, warnings, nil
}

func clonePluginMarketMap(value interface{}) map[string]interface{} {
	mapped, ok := value.(map[string]interface{})
	if !ok || mapped == nil {
		return map[string]interface{}{}
	}
	cloned := make(map[string]interface{}, len(mapped))
	for key, item := range mapped {
		cloned[key] = item
	}
	return cloned
}

func clonePluginMarketStringSlice(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	cloned := make([]string, 0, len(values))
	cloned = append(cloned, values...)
	return cloned
}

func pluginMarketInterfaceSliceToStrings(value interface{}) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []interface{}:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			text := strings.TrimSpace(pluginMarketStringFromAny(item))
			if text == "" || text == "<nil>" {
				continue
			}
			out = append(out, text)
		}
		return out
	default:
		return []string{}
	}
}

func pluginMarketStringSliceDiff(values []string, baseline []string) []string {
	baseSet := make(map[string]struct{}, len(baseline))
	for _, item := range NormalizePluginPermissionList(baseline) {
		baseSet[item] = struct{}{}
	}
	diff := make([]string, 0, len(values))
	for _, item := range NormalizePluginPermissionList(values) {
		if _, exists := baseSet[item]; exists {
			continue
		}
		diff = append(diff, item)
	}
	return diff
}

func comparePluginMarketSemver(left string, right string) int {
	leftParts := parsePluginMarketSemver(left)
	rightParts := parsePluginMarketSemver(right)
	maxLen := len(leftParts)
	if len(rightParts) > maxLen {
		maxLen = len(rightParts)
	}
	for idx := 0; idx < maxLen; idx++ {
		leftValue := 0
		if idx < len(leftParts) {
			leftValue = leftParts[idx]
		}
		rightValue := 0
		if idx < len(rightParts) {
			rightValue = rightParts[idx]
		}
		if leftValue < rightValue {
			return -1
		}
		if leftValue > rightValue {
			return 1
		}
	}
	return 0
}

func parsePluginMarketSemver(raw string) []int {
	normalized := strings.TrimSpace(raw)
	if normalized == "" {
		return []int{0}
	}
	if idx := strings.IndexByte(normalized, '-'); idx >= 0 {
		normalized = normalized[:idx]
	}
	parts := strings.Split(normalized, ".")
	out := make([]int, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			out = append(out, 0)
			continue
		}
		value := 0
		for _, ch := range part {
			if ch < '0' || ch > '9' {
				break
			}
			value = value*10 + int(ch-'0')
		}
		out = append(out, value)
	}
	return out
}
