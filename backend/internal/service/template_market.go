package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"auralogic/internal/models"
	"gorm.io/gorm"
)

type templateMarketResolved struct {
	TargetKey     string `json:"target_key"`
	ContentBytes  int    `json:"content_bytes"`
	ContentDigest string `json:"content_digest"`
}

type templateMarketCurrentState struct {
	Exists    bool
	Digest    string
	Content   string
	UpdatedAt interface{}
}

func PreviewTemplateMarketReleaseWithSource(
	runtime *PluginHostRuntime,
	source PluginMarketSource,
	kind string,
	name string,
	version string,
	params map[string]interface{},
) (map[string]interface{}, error) {
	normalizedSource, err := normalizeAdminPluginMarketSource(source)
	if err != nil {
		return nil, err
	}
	normalizedKind := normalizePluginMarketArtifactKind(kind)
	if normalizedKind != "email_template" && normalizedKind != "landing_page_template" && normalizedKind != "invoice_template" && normalizedKind != "auth_branding_template" && normalizedKind != "page_rule_pack" {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "market template kind is invalid"}
	}
	if !normalizedSource.AllowsKind(normalizedKind) {
		return nil, &PluginHostActionError{Status: http.StatusForbidden, Message: "market artifact kind is not allowed for this source"}
	}

	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "name is required"}
	}
	trimmedVersion := strings.TrimSpace(version)
	if trimmedVersion == "" {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "version is required"}
	}

	client := newPluginMarketSourceClient()
	release, err := client.FetchRelease(context.Background(), normalizedSource, normalizedKind, trimmedName, trimmedVersion)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadGateway, Message: err.Error()}
	}
	return previewTemplateMarketRelease(runtime, normalizedSource, normalizedKind, trimmedName, trimmedVersion, release, params)
}

func ExecuteTemplateMarketReleaseWithSource(
	runtime *PluginHostRuntime,
	source PluginMarketSource,
	kind string,
	name string,
	version string,
	params map[string]interface{},
	importedBy *uint,
) (map[string]interface{}, error) {
	normalizedSource, err := normalizeAdminPluginMarketSource(source)
	if err != nil {
		return nil, err
	}
	normalizedKind := normalizePluginMarketArtifactKind(kind)
	if normalizedKind != "email_template" && normalizedKind != "landing_page_template" && normalizedKind != "invoice_template" && normalizedKind != "auth_branding_template" && normalizedKind != "page_rule_pack" {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "market template kind is invalid"}
	}
	if !normalizedSource.AllowsKind(normalizedKind) {
		return nil, &PluginHostActionError{Status: http.StatusForbidden, Message: "market artifact kind is not allowed for this source"}
	}

	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "name is required"}
	}
	trimmedVersion := strings.TrimSpace(version)
	if trimmedVersion == "" {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "version is required"}
	}

	client := newPluginMarketSourceClient()
	release, err := client.FetchRelease(context.Background(), normalizedSource, normalizedKind, trimmedName, trimmedVersion)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadGateway, Message: err.Error()}
	}

	compatibility, warnings := buildPluginMarketCompatibilityPreview(release)
	if compatible, _ := compatibility["compatible"].(bool); !compatible {
		return nil, &PluginHostActionError{Status: http.StatusConflict, Message: "market release is not compatible with current host bridge"}
	}

	db := runtime.database()
	if db == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "template database is unavailable"}
	}

	targetKey, err := resolveTemplateMarketTargetKey(normalizedKind, params, release, trimmedName)
	if err != nil {
		return nil, err
	}
	content, err := resolveTemplateMarketContent(normalizedKind, release)
	if err != nil {
		return nil, err
	}

	saved, err := saveTemplateMarketTarget(runtime, normalizedKind, targetKey, content)
	if err != nil {
		return nil, err
	}

	coordinates := newPluginHostMarketCoordinates(normalizedSource.SourceID, normalizedKind, trimmedName, trimmedVersion)
	record, err := createTemplateVersionSnapshot(db, normalizedKind, targetKey, content, coordinates, importedBy)
	if err != nil {
		return nil, err
	}
	current, err := loadTemplateMarketCurrentState(runtime, normalizedKind, targetKey)
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"status":        "imported",
		"saved":         saved,
		"history_entry": buildTemplateVersionSummary(record),
		"resolved": templateMarketResolved{
			TargetKey:     targetKey,
			ContentBytes:  len(content),
			ContentDigest: pluginHostDigestString(content),
		},
		"source":        normalizedSource.Summary(),
		"coordinates":   coordinates.Map(),
		"release":       release,
		"governance":    clonePluginMarketMap(release["governance"]),
		"download":      clonePluginMarketMap(release["download"]),
		"compatibility": compatibility,
		"target_state":  buildTemplateMarketTargetState(runtime, normalizedKind, targetKey, trimmedVersion, current),
	}
	if len(warnings) > 0 {
		result["warnings"] = clonePluginMarketStringSlice(warnings)
	}
	return result, nil
}

func previewTemplateMarketRelease(
	runtime *PluginHostRuntime,
	source PluginMarketSource,
	kind string,
	name string,
	version string,
	release map[string]interface{},
	params map[string]interface{},
) (map[string]interface{}, error) {
	targetKey, err := resolveTemplateMarketTargetKey(kind, params, release, name)
	if err != nil {
		return nil, err
	}
	content, err := resolveTemplateMarketContent(kind, release)
	if err != nil {
		return nil, err
	}
	current, err := loadTemplateMarketCurrentState(runtime, kind, targetKey)
	if err != nil {
		return nil, err
	}

	compatibility, warnings := buildPluginMarketCompatibilityPreview(release)
	result := map[string]interface{}{
		"source":        source.Summary(),
		"coordinates":   newPluginHostMarketCoordinates(source.SourceID, kind, name, version).Map(),
		"release":       release,
		"governance":    clonePluginMarketMap(release["governance"]),
		"download":      clonePluginMarketMap(release["download"]),
		"compatibility": compatibility,
		"resolved": templateMarketResolved{
			TargetKey:     targetKey,
			ContentBytes:  len(content),
			ContentDigest: pluginHostDigestString(content),
		},
		"target_state": buildTemplateMarketTargetState(runtime, kind, targetKey, version, current),
	}
	if len(warnings) > 0 {
		result["warnings"] = clonePluginMarketStringSlice(warnings)
	}
	return result, nil
}

func createTemplateVersionSnapshot(
	db *gorm.DB,
	kind string,
	targetKey string,
	content string,
	coordinates pluginHostMarketCoordinates,
	importedBy *uint,
) (*models.TemplateVersion, error) {
	if db == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "template database is unavailable"}
	}
	now := time.Now().UTC()
	record := &models.TemplateVersion{
		ResourceKind:          kind,
		TargetKey:             targetKey,
		ContentSnapshot:       content,
		ContentDigest:         pluginHostDigestString(content),
		MarketSourceID:        coordinates.SourceID,
		MarketArtifactKind:    coordinates.Kind,
		MarketArtifactName:    coordinates.Name,
		MarketArtifactVersion: pluginHostMarketNormalizeVersion(coordinates.Version),
		ImportedBy:            importedBy,
		IsActive:              true,
		ActivatedAt:           &now,
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.TemplateVersion{}).
			Where("resource_kind = ? AND target_key = ?", kind, targetKey).
			Update("is_active", false).Error; err != nil {
			return err
		}
		return tx.Create(record).Error
	}); err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "create template version failed"}
	}
	return record, nil
}

func activateTemplateVersion(
	db *gorm.DB,
	targetVersion *models.TemplateVersion,
) (map[string]interface{}, *models.TemplateVersion, error) {
	if db == nil {
		return nil, nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "template database is unavailable"}
	}
	if targetVersion == nil || targetVersion.ID == 0 {
		return nil, nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "template version is required"}
	}

	saved, err := saveTemplateMarketTarget(
		NewPluginHostRuntime(db, resolvePluginHostRuntimeConfig(nil), nil),
		targetVersion.ResourceKind,
		targetVersion.TargetKey,
		targetVersion.ContentSnapshot,
	)
	if err != nil {
		return nil, nil, err
	}

	now := time.Now().UTC()
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.TemplateVersion{}).
			Where("resource_kind = ? AND target_key = ?", targetVersion.ResourceKind, targetVersion.TargetKey).
			Update("is_active", false).Error; err != nil {
			return err
		}
		return tx.Model(&models.TemplateVersion{}).Where("id = ?", targetVersion.ID).Updates(map[string]interface{}{
			"is_active":    true,
			"activated_at": now,
		}).Error
	}); err != nil {
		return nil, nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "activate template version failed"}
	}

	var refreshed models.TemplateVersion
	if err := db.First(&refreshed, targetVersion.ID).Error; err != nil {
		return saved, nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "load template version failed"}
	}
	return saved, &refreshed, nil
}

func buildTemplateVersionSummary(version *models.TemplateVersion) map[string]interface{} {
	if version == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":                      version.ID,
		"resource_kind":           version.ResourceKind,
		"target_key":              version.TargetKey,
		"content_digest":          version.ContentDigest,
		"content_bytes":           len(version.ContentSnapshot),
		"market_source_id":        version.MarketSourceID,
		"market_artifact_kind":    version.MarketArtifactKind,
		"market_artifact_name":    version.MarketArtifactName,
		"market_artifact_version": version.MarketArtifactVersion,
		"is_active":               version.IsActive,
		"activated_at":            version.ActivatedAt,
		"created_at":              version.CreatedAt,
		"updated_at":              version.UpdatedAt,
	}
}

func resolveTemplateMarketTargetKey(
	kind string,
	params map[string]interface{},
	release map[string]interface{},
	fallbackName string,
) (string, error) {
	targets := clonePluginMarketMap(release["targets"])
	templatePayload := clonePluginMarketMap(release["template"])
	switch kind {
	case "email_template":
		targetKey := strings.TrimSpace(pluginHostMarketFirstNonEmpty(
			parsePluginHostOptionalString(params, "target_key", "targetKey", "email_key", "emailKey"),
			pluginMarketStringFromAny(templatePayload["key"]),
			pluginMarketStringFromAny(targets["key"]),
			pluginMarketStringFromAny(targets["event"]),
			pluginMarketStringFromAny(templatePayload["filename"]),
			fallbackName,
		))
		targetKey = strings.TrimSuffix(strings.TrimSpace(targetKey), ".html")
		if targetKey == "" {
			return "", &PluginHostActionError{Status: http.StatusBadRequest, Message: "template target key could not be resolved"}
		}
		return targetKey, nil
	case "landing_page_template":
		targetKey := strings.TrimSpace(pluginHostMarketFirstNonEmpty(
			parsePluginHostOptionalString(params, "target_key", "targetKey", "landing_slug", "landingSlug", "slug", "page_key", "pageKey"),
			pluginMarketStringFromAny(templatePayload["slug"]),
			pluginMarketStringFromAny(templatePayload["page_key"]),
			pluginMarketStringFromAny(targets["slug"]),
			pluginMarketStringFromAny(targets["page_key"]),
			"home",
		))
		if targetKey == "" {
			targetKey = "home"
		}
		return targetKey, nil
	case "invoice_template":
		targetKey := strings.TrimSpace(pluginHostMarketFirstNonEmpty(
			parsePluginHostOptionalString(params, "target_key", "targetKey", "key"),
			pluginMarketStringFromAny(templatePayload["key"]),
			pluginMarketStringFromAny(targets["key"]),
			pluginHostInvoiceTemplateTargetKey,
		))
		if targetKey == "" || !strings.EqualFold(targetKey, pluginHostInvoiceTemplateTargetKey) {
			return "", &PluginHostActionError{Status: http.StatusBadRequest, Message: "invoice template target key is invalid"}
		}
		return pluginHostInvoiceTemplateTargetKey, nil
	case "auth_branding_template":
		targetKey := strings.TrimSpace(pluginHostMarketFirstNonEmpty(
			parsePluginHostOptionalString(params, "target_key", "targetKey", "key"),
			pluginMarketStringFromAny(templatePayload["key"]),
			pluginMarketStringFromAny(targets["key"]),
			pluginHostAuthBrandingTemplateTargetKey,
		))
		if targetKey == "" || !strings.EqualFold(targetKey, pluginHostAuthBrandingTemplateTargetKey) {
			return "", &PluginHostActionError{Status: http.StatusBadRequest, Message: "auth branding template target key is invalid"}
		}
		return pluginHostAuthBrandingTemplateTargetKey, nil
	case "page_rule_pack":
		targetKey := strings.TrimSpace(pluginHostMarketFirstNonEmpty(
			parsePluginHostOptionalString(params, "target_key", "targetKey", "key"),
			pluginMarketStringFromAny(clonePluginMarketMap(release["page_rules"])["key"]),
			pluginMarketStringFromAny(templatePayload["key"]),
			pluginMarketStringFromAny(targets["key"]),
			pluginHostPageRulePackTargetKey,
		))
		if targetKey == "" || !strings.EqualFold(targetKey, pluginHostPageRulePackTargetKey) {
			return "", &PluginHostActionError{Status: http.StatusBadRequest, Message: "page rule pack target key is invalid"}
		}
		return pluginHostPageRulePackTargetKey, nil
	default:
		return "", &PluginHostActionError{Status: http.StatusBadRequest, Message: "market template kind is invalid"}
	}
}

func resolveTemplateMarketContent(kind string, release map[string]interface{}) (string, error) {
	if kind == "page_rule_pack" {
		return resolveTemplateMarketPageRulePackContent(release)
	}

	templatePayload := clonePluginMarketMap(release["template"])
	install := clonePluginMarketMap(release["install"])
	content := strings.TrimSpace(pluginHostMarketFirstNonEmpty(
		pluginMarketStringFromAny(templatePayload["content"]),
		pluginMarketStringFromAny(templatePayload["html"]),
		pluginMarketStringFromAny(install["content"]),
		pluginMarketStringFromAny(release["content"]),
	))
	if content == "" {
		switch kind {
		case "email_template":
			return "", &PluginHostActionError{Status: http.StatusBadRequest, Message: "email template release did not expose inline content"}
		case "landing_page_template":
			return "", &PluginHostActionError{Status: http.StatusBadRequest, Message: "landing page release did not expose inline content"}
		case "invoice_template":
			return "", &PluginHostActionError{Status: http.StatusBadRequest, Message: "invoice template release did not expose inline content"}
		case "auth_branding_template":
			return "", &PluginHostActionError{Status: http.StatusBadRequest, Message: "auth branding template release did not expose inline content"}
		default:
			return "", &PluginHostActionError{Status: http.StatusBadRequest, Message: "market template kind is invalid"}
		}
	}
	return content, nil
}

func resolveTemplateMarketPageRulePackContent(release map[string]interface{}) (string, error) {
	pageRulesPayload := clonePluginMarketMap(release["page_rules"])
	templatePayload := clonePluginMarketMap(release["template"])
	install := clonePluginMarketMap(release["install"])

	for _, candidate := range []interface{}{
		pageRulesPayload["rules"],
		templatePayload["rules"],
		pageRulesPayload["content"],
		templatePayload["content"],
		install["content"],
		release["content"],
	} {
		content, ok, err := normalizeTemplateMarketPageRulePackContent(candidate)
		if err != nil {
			return "", err
		}
		if ok {
			return content, nil
		}
	}

	return "", &PluginHostActionError{Status: http.StatusBadRequest, Message: "page rule pack release did not expose valid inline content"}
}

func normalizeTemplateMarketPageRulePackContent(value interface{}) (string, bool, error) {
	if value == nil {
		return "", false, nil
	}

	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) == "" {
			return "", false, nil
		}
		rules, err := parsePluginHostPageRulePackContent([]byte(typed))
		if err != nil {
			return "", false, err
		}
		body, err := marshalPluginHostPageRules(rules)
		if err != nil {
			return "", false, &PluginHostActionError{Status: http.StatusBadRequest, Message: "page rule pack content is invalid"}
		}
		return string(body), true, nil
	default:
		body, err := json.Marshal(typed)
		if err != nil {
			return "", false, &PluginHostActionError{Status: http.StatusBadRequest, Message: "page rule pack content is invalid"}
		}
		rules, err := parsePluginHostPageRulePackContent(body)
		if err != nil {
			return "", false, err
		}
		canonical, err := marshalPluginHostPageRules(rules)
		if err != nil {
			return "", false, &PluginHostActionError{Status: http.StatusBadRequest, Message: "page rule pack content is invalid"}
		}
		return string(canonical), true, nil
	}
}

func loadTemplateMarketCurrentState(runtime *PluginHostRuntime, kind string, targetKey string) (*templateMarketCurrentState, error) {
	db := runtime.database()
	switch kind {
	case "email_template":
		payload, err := executePluginHostEmailTemplateGet(map[string]interface{}{
			"key": targetKey,
		})
		if err != nil {
			var hostErr *PluginHostActionError
			if errors.As(err, &hostErr) && hostErr.Status == http.StatusNotFound {
				return &templateMarketCurrentState{Exists: false}, nil
			}
			return nil, err
		}
		return &templateMarketCurrentState{
			Exists:    true,
			Digest:    strings.TrimSpace(pluginMarketStringFromAny(payload["digest"])),
			Content:   pluginMarketStringFromAny(payload["content"]),
			UpdatedAt: payload["updated_at"],
		}, nil
	case "landing_page_template":
		payload, err := executePluginHostLandingPageGet(db, map[string]interface{}{
			"slug": targetKey,
		})
		if err != nil {
			return nil, err
		}
		return &templateMarketCurrentState{
			Exists:    payload["exists"] == true,
			Digest:    strings.TrimSpace(pluginMarketStringFromAny(payload["digest"])),
			Content:   pluginMarketStringFromAny(payload["html_content"]),
			UpdatedAt: payload["updated_at"],
		}, nil
	case "invoice_template":
		payload, err := executePluginHostInvoiceTemplateGet(runtime, map[string]interface{}{
			"target_key": targetKey,
		})
		if err != nil {
			return nil, err
		}
		return &templateMarketCurrentState{
			Exists:    payload["exists"] == true,
			Digest:    strings.TrimSpace(pluginMarketStringFromAny(payload["digest"])),
			Content:   pluginMarketStringFromAny(payload["custom_template"]),
			UpdatedAt: payload["updated_at"],
		}, nil
	case "auth_branding_template":
		payload, err := executePluginHostAuthBrandingGet(runtime, map[string]interface{}{
			"target_key": targetKey,
		})
		if err != nil {
			return nil, err
		}
		return &templateMarketCurrentState{
			Exists:    payload["exists"] == true,
			Digest:    strings.TrimSpace(pluginMarketStringFromAny(payload["digest"])),
			Content:   pluginMarketStringFromAny(payload["custom_html"]),
			UpdatedAt: payload["updated_at"],
		}, nil
	case "page_rule_pack":
		payload, err := executePluginHostPageRulePackGet(runtime, map[string]interface{}{
			"target_key": targetKey,
		})
		if err != nil {
			return nil, err
		}
		return &templateMarketCurrentState{
			Exists:    payload["exists"] == true,
			Digest:    strings.TrimSpace(pluginMarketStringFromAny(payload["digest"])),
			Content:   pluginMarketStringFromAny(payload["content"]),
			UpdatedAt: payload["updated_at"],
		}, nil
	default:
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "market template kind is invalid"}
	}
}

func saveTemplateMarketTarget(
	runtime *PluginHostRuntime,
	kind string,
	targetKey string,
	content string,
) (map[string]interface{}, error) {
	db := runtime.database()
	switch kind {
	case "email_template":
		return executePluginHostEmailTemplateSave(map[string]interface{}{
			"key":     targetKey,
			"content": content,
		})
	case "landing_page_template":
		return executePluginHostLandingPageSave(db, map[string]interface{}{
			"slug":         targetKey,
			"html_content": content,
		})
	case "invoice_template":
		return executePluginHostInvoiceTemplateSave(runtime, map[string]interface{}{
			"target_key": targetKey,
			"content":    content,
		})
	case "auth_branding_template":
		return executePluginHostAuthBrandingSave(runtime, map[string]interface{}{
			"target_key": targetKey,
			"content":    content,
		})
	case "page_rule_pack":
		return executePluginHostPageRulePackSave(runtime, map[string]interface{}{
			"target_key": targetKey,
			"content":    content,
		})
	default:
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "market template kind is invalid"}
	}
}

func buildTemplateMarketTargetState(
	runtime *PluginHostRuntime,
	kind string,
	targetKey string,
	requestedVersion string,
	current *templateMarketCurrentState,
) map[string]interface{} {
	db := runtime.database()
	state := map[string]interface{}{
		"installed":             false,
		"current_version":       "",
		"update_available":      false,
		"installed_target":      kind,
		"installed_target_key":  targetKey,
		"installed_target_id":   nil,
		"target_exists":         current != nil && current.Exists,
		"current_digest":        "",
		"current_content_bytes": 0,
	}
	if current != nil {
		state["current_digest"] = current.Digest
		state["current_content_bytes"] = len(current.Content)
	}
	if db == nil {
		return state
	}

	var active models.TemplateVersion
	if err := db.Where("resource_kind = ? AND target_key = ? AND is_active = ?", kind, targetKey, true).First(&active).Error; err != nil {
		return state
	}
	state["installed"] = true
	state["current_version"] = active.MarketArtifactVersion
	state["installed_target_id"] = active.ID
	state["update_available"] = strings.TrimSpace(active.MarketArtifactVersion) != "" &&
		strings.TrimSpace(active.MarketArtifactVersion) != pluginHostMarketNormalizeVersion(requestedVersion)
	return state
}
