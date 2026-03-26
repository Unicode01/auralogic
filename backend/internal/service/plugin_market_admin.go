package service

import (
	"context"
	"net/http"
	"strings"
)

func normalizeAdminPluginMarketSource(source PluginMarketSource) (PluginMarketSource, error) {
	candidate := map[string]interface{}{
		"source_id":       strings.TrimSpace(source.SourceID),
		"name":            strings.TrimSpace(source.Name),
		"base_url":        strings.TrimSpace(source.BaseURL),
		"public_key":      strings.TrimSpace(source.PublicKey),
		"default_channel": strings.TrimSpace(source.DefaultChannel),
		"allowed_kinds":   clonePluginMarketStringSlice(source.AllowedKinds),
		"enabled":         source.Enabled,
	}
	normalized, ok := buildPluginMarketSourceFromMap(candidate)
	if !ok {
		return PluginMarketSource{}, &PluginHostActionError{Status: http.StatusBadRequest, Message: "market source configuration is invalid"}
	}
	if len(normalized.AllowedKinds) == 0 {
		normalized.AllowedKinds = clonePluginMarketStringSlice(pluginHostMarketArtifactKinds)
	}
	return normalized, nil
}

func buildAdminPluginMarketPermissionPreview(
	release map[string]interface{},
	permissionPreview map[string]interface{},
) ([]PluginPermissionRequest, []string) {
	requested := NormalizePluginPermissionList(pluginMarketInterfaceSliceToStrings(permissionPreview["requested"]))
	defaultGranted := NormalizePluginPermissionList(pluginMarketInterfaceSliceToStrings(permissionPreview["default_granted"]))
	required := NormalizePluginPermissionList(
		pluginMarketInterfaceSliceToStrings(clonePluginMarketMap(release["permissions"])["required"]),
	)
	return BuildPluginPermissionRequests(requested, required, nil), defaultGranted
}

func buildAdminPluginMarketManifestPreview(
	source PluginMarketSource,
	kind string,
	name string,
	version string,
	release map[string]interface{},
) map[string]interface{} {
	compatibility := clonePluginMarketMap(release["compatibility"])
	install := clonePluginMarketMap(release["install"])
	displayName := release["title"]
	if ResolveManifestLocalizedTextValue(displayName) == "" {
		displayName = strings.TrimSpace(name)
	}
	description := release["description"]
	if ResolveManifestLocalizedTextValue(description) == "" {
		description = release["summary"]
	}
	if ResolveManifestLocalizedTextValue(description) == "" {
		description = ""
	}
	changelog := release["release_notes"]
	if ResolveManifestLocalizedTextValue(changelog) == "" {
		changelog = ""
	}
	manifest := map[string]interface{}{
		"name":         strings.TrimSpace(name),
		"display_name": displayName,
		"description":  description,
		"version": pluginHostMarketNormalizeVersion(
			pluginHostMarketFirstNonEmpty(version, pluginMarketStringFromAny(release["version"])),
		),
		"runtime": pluginHostMarketFirstNonEmpty(
			pluginMarketStringFromAny(compatibility["runtime"]),
			PluginRuntimeJSWorker,
		),
		"type":      "custom",
		"entry":     pluginMarketStringFromAny(install["entry"]),
		"changelog": changelog,
		"market": map[string]interface{}{
			"source":       source.Summary(),
			"kind":         kind,
			"name":         strings.TrimSpace(name),
			"version":      pluginHostMarketNormalizeVersion(version),
			"published_at": pluginMarketStringFromAny(release["published_at"]),
		},
	}
	return manifest
}

func PreviewPluginMarketInstallWithSource(
	runtime *PluginHostRuntime,
	source PluginMarketSource,
	kind string,
	name string,
	version string,
) (map[string]interface{}, error) {
	normalizedSource, err := normalizeAdminPluginMarketSource(source)
	if err != nil {
		return nil, err
	}

	kind = normalizePluginMarketArtifactKind(kind)
	if kind == "" {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "kind is required"}
	}
	if !normalizedSource.AllowsKind(kind) {
		return nil, &PluginHostActionError{Status: http.StatusForbidden, Message: "market artifact kind is not allowed for this source"}
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "name is required"}
	}
	version = strings.TrimSpace(version)
	if version == "" {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "version is required"}
	}

	client := newPluginMarketSourceClient()
	release, err := client.FetchRelease(context.Background(), normalizedSource, kind, name, version)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadGateway, Message: err.Error()}
	}

	compatibility, compatibilityWarnings := buildPluginMarketCompatibilityPreview(release)
	response := map[string]interface{}{
		"source": normalizedSource.Summary(),
		"coordinates": map[string]interface{}{
			"source_id": normalizedSource.SourceID,
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
			"installed_target":    kind,
			"installed_target_id": nil,
		},
	}

	if kind == "plugin_package" {
		targetState, permissionPreview, warnings, previewErr := buildPluginMarketPluginPreview(
			runtime.database(),
			name,
			version,
			release,
		)
		if previewErr != nil {
			return nil, previewErr
		}
		requestedPermissions, defaultGranted := buildAdminPluginMarketPermissionPreview(release, permissionPreview)
		response["target_state"] = targetState
		response["permissions"] = permissionPreview
		response["requested_permissions"] = requestedPermissions
		response["default_granted_permissions"] = defaultGranted
		response["manifest"] = buildAdminPluginMarketManifestPreview(normalizedSource, kind, name, version, release)
		if len(warnings) > 0 {
			response["warnings"] = append(clonePluginMarketStringSlice(compatibilityWarnings), warnings...)
		}
		return response, nil
	}
	if kind == "payment_package" {
		return previewPaymentMethodMarketPackageRelease(runtime, normalizedSource, name, version, release, 0)
	}
	if kind == "email_template" || kind == "landing_page_template" || kind == "invoice_template" || kind == "auth_branding_template" || kind == "page_rule_pack" {
		return previewTemplateMarketRelease(runtime, normalizedSource, kind, name, version, release, nil)
	}

	return response, nil
}

func ExecutePluginMarketInstallWithSource(
	runtime *PluginHostRuntime,
	source PluginMarketSource,
	kind string,
	name string,
	version string,
	params map[string]interface{},
	operatorUserID *uint,
) (map[string]interface{}, error) {
	normalizedSource, err := normalizeAdminPluginMarketSource(source)
	if err != nil {
		return nil, err
	}

	kind = normalizePluginMarketArtifactKind(kind)
	if kind == "" {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "kind is required"}
	}
	if !normalizedSource.AllowsKind(kind) {
		return nil, &PluginHostActionError{Status: http.StatusForbidden, Message: "market artifact kind is not allowed for this source"}
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "name is required"}
	}
	version = strings.TrimSpace(version)
	if version == "" {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "version is required"}
	}

	client := newPluginMarketSourceClient()
	release, err := client.FetchRelease(context.Background(), normalizedSource, kind, name, version)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadGateway, Message: err.Error()}
	}

	compatibility, warnings := buildPluginMarketCompatibilityPreview(release)
	if compatible, _ := compatibility["compatible"].(bool); !compatible {
		return nil, &PluginHostActionError{Status: http.StatusConflict, Message: "market release is not compatible with current host bridge"}
	}

	switch kind {
	case "plugin_package":
		claims := &PluginHostAccessClaims{}
		if operatorUserID != nil {
			claims.OperatorUserID = *operatorUserID
		}
		result, execErr := executePluginHostMarketInstallPluginPackage(
			runtime,
			claims,
			normalizedSource,
			name,
			version,
			release,
			params,
		)
		if execErr != nil {
			return nil, execErr
		}
		if len(warnings) > 0 {
			result["warnings"] = clonePluginMarketStringSlice(warnings)
		}
		result["compatibility"] = compatibility
		result["source"] = normalizedSource.Summary()
		result["coordinates"] = map[string]interface{}{
			"source_id": normalizedSource.SourceID,
			"kind":      kind,
			"name":      name,
			"version":   version,
		}
		result["release"] = release
		return result, nil
	case "payment_package":
		result, execErr := ExecutePaymentMethodMarketPackageWithSource(
			runtime,
			normalizedSource,
			name,
			version,
			params,
		)
		if execErr != nil {
			return nil, execErr
		}
		if len(warnings) > 0 {
			result["warnings"] = clonePluginMarketStringSlice(warnings)
		}
		result["compatibility"] = compatibility
		result["source"] = normalizedSource.Summary()
		result["coordinates"] = map[string]interface{}{
			"source_id": normalizedSource.SourceID,
			"kind":      kind,
			"name":      name,
			"version":   version,
		}
		result["release"] = release
		return result, nil
	case "email_template", "landing_page_template", "invoice_template", "auth_branding_template", "page_rule_pack":
		result, execErr := ExecuteTemplateMarketReleaseWithSource(
			runtime,
			normalizedSource,
			kind,
			name,
			version,
			params,
			operatorUserID,
		)
		if execErr != nil {
			return nil, execErr
		}
		if len(warnings) > 0 {
			result["warnings"] = clonePluginMarketStringSlice(warnings)
		}
		result["compatibility"] = compatibility
		result["source"] = normalizedSource.Summary()
		result["coordinates"] = map[string]interface{}{
			"source_id": normalizedSource.SourceID,
			"kind":      kind,
			"name":      name,
			"version":   version,
		}
		result["release"] = release
		return result, nil
	default:
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "market install kind is not supported yet"}
	}
}
