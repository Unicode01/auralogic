package service

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/jsworker"
	"auralogic/internal/models"
	"gorm.io/gorm"
)

const (
	pluginHostMarketInstallMaxDownloadBytes       = int64(128 * 1024 * 1024)
	pluginHostMarketInstallMaxPackageFiles        = 1024
	pluginHostMarketInstallMaxSingleFileBytes     = int64(16 * 1024 * 1024)
	pluginHostMarketInstallMaxCompressionRatio    = 200.0
	pluginHostMarketInstallDownloadTimeout        = 60 * time.Second
	pluginHostMarketInstallManifestReadLimitBytes = int64(1024 * 1024)
)

type pluginHostMarketInstallDownloadInfo struct {
	URL          string
	PackageName  string
	ExpectedSHA  string
	ExpectedSize int64
}

type pluginHostMarketInstallArtifact struct {
	PackagePath string
	PackageName string
	Checksum    string
}

type pluginHostMarketPackagePermission struct {
	Key      string                `json:"key"`
	Required bool                  `json:"required"`
	Reason   ManifestLocalizedText `json:"reason"`
}

type pluginHostMarketWorkspaceCommand struct {
	Name        string                `json:"name"`
	Title       ManifestLocalizedText `json:"title"`
	Description ManifestLocalizedText `json:"description"`
	Entry       string                `json:"entry"`
	Interactive bool                  `json:"interactive"`
	Permissions []string              `json:"permissions"`
}

type pluginHostMarketWorkspaceManifest struct {
	Enabled  *bool                              `json:"enabled"`
	Title    ManifestLocalizedText              `json:"title"`
	Commands []pluginHostMarketWorkspaceCommand `json:"commands"`
}

type pluginHostMarketPackageManifest struct {
	Name                   string                              `json:"name"`
	DisplayName            ManifestLocalizedText               `json:"display_name"`
	Description            ManifestLocalizedText               `json:"description"`
	Type                   string                              `json:"type"`
	Runtime                string                              `json:"runtime"`
	Address                string                              `json:"address"`
	Entry                  string                              `json:"entry"`
	Version                string                              `json:"version"`
	Changelog              ManifestLocalizedText               `json:"changelog"`
	ManifestVersion        string                              `json:"manifest_version"`
	ProtocolVersion        string                              `json:"protocol_version"`
	MinHostProtocolVersion string                              `json:"min_host_protocol_version"`
	MaxHostProtocolVersion string                              `json:"max_host_protocol_version"`
	Activate               *bool                               `json:"activate"`
	AutoStart              *bool                               `json:"auto_start"`
	Config                 map[string]interface{}              `json:"config"`
	RuntimeParams          map[string]interface{}              `json:"runtime_params"`
	Workspace              *pluginHostMarketWorkspaceManifest  `json:"workspace"`
	Capabilities           map[string]interface{}              `json:"capabilities"`
	RequestedPermissions   []string                            `json:"requested_permissions"`
	RequiredPermissions    []string                            `json:"required_permissions"`
	Permissions            []pluginHostMarketPackagePermission `json:"permissions"`
}

func executePluginHostMarketInstallExecute(
	runtime *PluginHostRuntime,
	claims *PluginHostAccessClaims,
	params map[string]interface{},
) (map[string]interface{}, error) {
	db := runtime.database()
	plugin, err := pluginHostLoadPluginForScopedConfig(db, claims)
	if err != nil {
		return nil, err
	}
	source, kind, name, version, err := resolvePluginMarketCoordinates(plugin, params)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(version) == "" {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "version is required"}
	}
	coordinates := newPluginHostMarketCoordinates(source.SourceID, kind, name, version)

	client := newPluginMarketSourceClient()
	release, err := client.FetchRelease(context.Background(), source, kind, name, version)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadGateway, Message: err.Error()}
	}

	compatibility, warnings := buildPluginMarketCompatibilityPreview(release)
	if compatible, _ := compatibility["compatible"].(bool); !compatible {
		return nil, &PluginHostActionError{Status: http.StatusConflict, Message: "market release is not compatible with current host bridge"}
	}

	switch kind {
	case "plugin_package":
		result, execErr := executePluginHostMarketInstallPluginPackage(runtime, claims, source, name, version, release, params)
		if execErr != nil {
			return nil, execErr
		}
		if len(warnings) > 0 {
			result["warnings"] = clonePluginMarketStringSlice(warnings)
		}
		result["compatibility"] = compatibility
		result["source"] = source.Summary()
		result["coordinates"] = map[string]interface{}{
			"source_id": source.SourceID,
			"kind":      kind,
			"name":      name,
			"version":   version,
		}
		result["release"] = release
		logPluginHostMarketOperation(db, claims, plugin, "plugin_market_install", coordinates, params, result)
		return result, nil
	case "payment_package":
		result, execErr := ExecutePaymentMethodMarketPackageWithSource(runtime, source, name, version, params)
		if execErr != nil {
			return nil, execErr
		}
		if len(warnings) > 0 {
			result["warnings"] = clonePluginMarketStringSlice(warnings)
		}
		result["compatibility"] = compatibility
		result["source"] = source.Summary()
		result["coordinates"] = map[string]interface{}{
			"source_id": source.SourceID,
			"kind":      kind,
			"name":      name,
			"version":   version,
		}
		result["release"] = release
		logPluginHostMarketOperation(db, claims, plugin, "plugin_market_install", coordinates, params, result)
		return result, nil
	case "email_template", "landing_page_template", "invoice_template", "auth_branding_template", "page_rule_pack":
		var importedBy *uint
		if claims != nil && claims.OperatorUserID > 0 {
			importedBy = &claims.OperatorUserID
		}
		result, execErr := ExecuteTemplateMarketReleaseWithSource(runtime, source, kind, name, version, params, importedBy)
		if execErr != nil {
			return nil, execErr
		}
		if len(warnings) > 0 {
			result["warnings"] = clonePluginMarketStringSlice(warnings)
		}
		result["compatibility"] = compatibility
		result["source"] = source.Summary()
		result["coordinates"] = map[string]interface{}{
			"source_id": source.SourceID,
			"kind":      kind,
			"name":      name,
			"version":   version,
		}
		result["release"] = release
		logPluginHostMarketOperation(db, claims, plugin, "plugin_market_install", coordinates, params, result)
		return result, nil
	default:
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: fmt.Sprintf("market install kind %q is not supported yet", kind)}
	}
}

func executePluginHostMarketInstallPluginPackage(
	runtime *PluginHostRuntime,
	claims *PluginHostAccessClaims,
	source PluginMarketSource,
	name string,
	version string,
	release map[string]interface{},
	params map[string]interface{},
) (map[string]interface{}, error) {
	db := runtime.database()
	if db == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "plugin host database is unavailable"}
	}

	cfg := pluginHostMarketRuntimeConfig(runtime)
	if cfg == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "plugin config is unavailable"}
	}

	governance := clonePluginMarketMap(release["governance"])
	if mode := strings.ToLower(strings.TrimSpace(pluginMarketStringFromAny(governance["mode"]))); mode != "" && mode != "host_managed" {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "market plugin package must use host_managed governance"}
	}

	artifact, err := pluginHostMarketDownloadReleaseArtifact(cfg, source, "plugin_package", name, version, release)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadGateway, Message: err.Error()}
	}

	manifest, manifestRaw, err := pluginHostMarketReadPluginManifestFromPackage(artifact.PackagePath)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}

	manifestName := pluginHostMarketFirstNonEmpty(manifest.Name, name)
	if manifestName == "" {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "plugin manifest name is required"}
	}
	if !strings.EqualFold(manifestName, name) {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "plugin manifest name does not match market coordinates"}
	}

	selectedVersion := pluginHostMarketNormalizeVersion(pluginHostMarketFirstNonEmpty(manifest.Version, version))
	if strings.TrimSpace(version) != "" && selectedVersion != strings.TrimSpace(version) {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "plugin manifest version does not match market coordinates"}
	}
	coordinates := newPluginHostMarketCoordinates(source.SourceID, "plugin_package", manifestName, selectedVersion)

	var plugin models.Plugin
	pluginExists := false
	if err := db.Where("name = ?", manifestName).First(&plugin).Error; err == nil {
		pluginExists = true
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query local plugin failed"}
	}

	selectedType := strings.ToLower(strings.TrimSpace(pluginHostMarketFirstNonEmpty(manifest.Type, plugin.Type, "custom")))
	resolvedRuntime, err := pluginHostMarketResolveRuntime(runtime.PluginManager, cfg, pluginHostMarketFirstNonEmpty(manifest.Runtime, plugin.Runtime))
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	if err := pluginHostMarketValidateProfile(runtime.PluginManager, cfg, resolvedRuntime, selectedType); err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	if err := ValidatePluginManifestCompatibility(manifestRaw, resolvedRuntime); err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}

	selectedAddress := pluginHostMarketFirstNonEmpty(
		manifest.Address,
		manifest.Entry,
		pluginHostMarketReleaseInstallEntry(release),
		plugin.Address,
	)
	if resolvedRuntime == PluginRuntimeJSWorker {
		resolvedAddress, normalizedPackagePath, _, resolveErr := pluginHostMarketPrepareJSWorkerPackage(artifact.PackagePath, selectedAddress)
		if resolveErr != nil {
			return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: resolveErr.Error()}
		}
		artifact.PackagePath = normalizedPackagePath
		selectedAddress = resolvedAddress
	}
	if resolvedRuntime == PluginRuntimeGRPC && strings.TrimSpace(selectedAddress) == "" {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "service address is required for grpc runtime"}
	}
	if resolvedRuntime == PluginRuntimeJSWorker && strings.TrimSpace(selectedAddress) == "" {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "entry script path is required for js_worker runtime"}
	}

	configJSON, err := pluginHostMarketMarshalJSONObject(manifest.Config, plugin.Config)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "invalid plugin config json"}
	}
	runtimeParamsJSON, err := pluginHostMarketMarshalJSONObject(manifest.RuntimeParams, plugin.RuntimeParams)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "invalid plugin runtime_params json"}
	}
	capabilitiesJSON, err := pluginHostMarketMarshalJSONObject(manifest.Capabilities, plugin.Capabilities)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "invalid plugin capabilities json"}
	}

	permissionRequests, defaultGranted := pluginHostMarketBuildPermissionRequests(manifest, release)
	grantedPermissions, grantedSpecified := pluginHostMarketInstallStringSliceOption(params, "granted_permissions", "grantedPermissions")
	if grantedSpecified && grantedPermissions == nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "granted_permissions must be a string array"}
	}
	if !grantedSpecified {
		grantedPermissions = defaultGranted
	}
	grantedPermissions, err = ValidateGrantedPluginPermissions(permissionRequests, grantedPermissions)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	capabilitiesJSON, err = pluginHostMarketApplyGrantedPermissions(capabilitiesJSON, permissionRequests, grantedPermissions)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "invalid plugin capabilities after applying permission grants"}
	}

	displayName := pluginHostMarketFirstNonEmpty(manifest.DisplayName.String(), plugin.DisplayName, manifestName)
	description := pluginHostMarketFirstNonEmpty(manifest.Description.String(), plugin.Description)
	releaseInstall := clonePluginMarketMap(release["install"])
	activateDefault := pluginHostMarketReleaseInstallBool(releaseInstall, "auto_activate_default", manifest.Activate, false)
	autoStartDefault := pluginHostMarketReleaseInstallBool(releaseInstall, "auto_start_default", manifest.AutoStart, false)
	activate, err := pluginHostMarketInstallBoolOption(params, activateDefault, "activate")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	autoStart, err := pluginHostMarketInstallBoolOption(params, autoStartDefault, "auto_start", "autoStart")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	if autoStart {
		activate = true
	}

	detail := pluginHostMarketInstallStringOption(params, "note", "detail")
	if detail == "" {
		detail = fmt.Sprintf("market install %s/%s@%s", source.SourceID, manifestName, selectedVersion)
	}
	changelog := pluginHostMarketFirstNonEmpty(
		manifest.Changelog.String(),
		ResolveManifestLocalizedTextValue(release["release_notes"]),
	)
	operatorUserID := pluginHostMarketOptionalUint(claims.OperatorUserID)

	createdPlugin := false
	if !pluginExists {
		plugin = models.Plugin{
			Name:            manifestName,
			DisplayName:     displayName,
			Description:     description,
			Type:            selectedType,
			Runtime:         resolvedRuntime,
			Address:         selectedAddress,
			Version:         selectedVersion,
			Config:          configJSON,
			RuntimeParams:   runtimeParamsJSON,
			Capabilities:    capabilitiesJSON,
			Manifest:        manifestRaw,
			PackagePath:     artifact.PackagePath,
			PackageChecksum: artifact.Checksum,
			Enabled:         false,
			Status:          "unknown",
			LifecycleStatus: models.PluginLifecycleUploaded,
		}
		if strings.TrimSpace(selectedAddress) != "" {
			plugin.LifecycleStatus = models.PluginLifecycleInstalled
		}
		plugin.RuntimeSpecHash = ComputePluginRuntimeSpecHash(&plugin)
		plugin.DesiredGeneration = 1
		plugin.AppliedGeneration = 1
		if err := db.Select("*").Create(&plugin).Error; err != nil {
			return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "create local plugin failed"}
		}
		createdPlugin = true
	}

	versionRecord := &models.PluginVersion{
		PluginID:              plugin.ID,
		Version:               selectedVersion,
		MarketSourceID:        coordinates.SourceID,
		MarketArtifactKind:    coordinates.Kind,
		MarketArtifactName:    coordinates.Name,
		MarketArtifactVersion: coordinates.Version,
		PackageName:           artifact.PackageName,
		PackagePath:           artifact.PackagePath,
		PackageChecksum:       artifact.Checksum,
		Manifest:              manifestRaw,
		Type:                  selectedType,
		Runtime:               resolvedRuntime,
		Address:               selectedAddress,
		ConfigSnapshot:        configJSON,
		RuntimeParams:         runtimeParamsJSON,
		CapabilitiesSnapshot:  capabilitiesJSON,
		Changelog:             changelog,
		LifecycleStatus:       models.PluginLifecycleUploaded,
		UploadedBy:            operatorUserID,
	}
	if err := db.Create(versionRecord).Error; err != nil {
		if createdPlugin {
			_ = db.Delete(&plugin).Error
		}
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "create plugin version failed"}
	}

	response := map[string]interface{}{
		"status":             "uploaded",
		"activate_requested": activate,
		"auto_start":         autoStart,
		"coordinates":        coordinates.Map(),
		"plugin":             pluginHostMarketBuildPluginSummary(&plugin),
		"version":            pluginHostMarketBuildVersionSummary(versionRecord),
		"permissions": map[string]interface{}{
			"requested":       pluginHostMarketPermissionKeys(permissionRequests),
			"default_granted": defaultGranted,
			"granted":         grantedPermissions,
		},
	}
	if !activate {
		runtimeSpecHash := ResolvePluginRuntimeSpecHash(&plugin)
		if runtimeSpecHash == "" {
			runtimeSpecHash = ComputePluginRuntimeSpecHash(&plugin)
		}
		deployment, deploymentErr := pluginHostMarketCreateDeployment(
			db,
			plugin.ID,
			models.PluginDeploymentOperationInstall,
			"market_install",
			&versionRecord.ID,
			plugin.DesiredGeneration,
			runtimeSpecHash,
			false,
			operatorUserID,
			detail,
			coordinates,
		)
		if deploymentErr != nil {
			return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "create market install task failed"}
		}
		if err := pluginHostMarketMarkDeploymentRunning(db, deployment, detail); err != nil {
			return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "start market install task failed"}
		}
		if err := pluginHostMarketMarkDeploymentSucceeded(db, deployment, plugin.AppliedGeneration, detail); err != nil {
			return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "complete market install task failed"}
		}
		response["deployment"] = pluginHostMarketBuildDeploymentSummary(deployment)
		response["task_id"] = pluginHostMarketTaskIDForDeploymentID(deployment.ID)
		return response, nil
	}

	activatedPlugin, activatedVersion, deployment, activateErr := pluginHostMarketActivatePluginVersion(
		runtime,
		plugin.ID,
		versionRecord.ID,
		autoStart,
		operatorUserID,
		detail,
		models.PluginDeploymentOperationInstall,
		"market_install",
		coordinates,
	)
	if activateErr != nil {
		refreshedPlugin := plugin
		_ = db.First(&refreshedPlugin, plugin.ID).Error
		refreshedVersion := *versionRecord
		_ = db.First(&refreshedVersion, versionRecord.ID).Error
		response["status"] = "activate_failed"
		response["activate_failed"] = true
		response["error"] = activateErr.Error()
		response["plugin"] = pluginHostMarketBuildPluginSummary(&refreshedPlugin)
		response["version"] = pluginHostMarketBuildVersionSummary(&refreshedVersion)
		if deployment != nil {
			response["deployment"] = pluginHostMarketBuildDeploymentSummary(deployment)
			response["task_id"] = pluginHostMarketTaskIDForDeploymentID(deployment.ID)
		}
		return response, nil
	}

	response["status"] = "activated"
	response["plugin"] = pluginHostMarketBuildPluginSummary(activatedPlugin)
	response["version"] = pluginHostMarketBuildVersionSummary(activatedVersion)
	if deployment != nil {
		response["deployment"] = pluginHostMarketBuildDeploymentSummary(deployment)
		response["task_id"] = pluginHostMarketTaskIDForDeploymentID(deployment.ID)
	}
	return response, nil
}

func resolvePluginHostMarketInstallOptions(params map[string]interface{}) map[string]interface{} {
	if len(params) == 0 {
		return map[string]interface{}{}
	}
	if raw, exists := params["options"]; exists {
		if mapped, ok := raw.(map[string]interface{}); ok && mapped != nil {
			return mapped
		}
	}
	return map[string]interface{}{}
}

func pluginHostMarketInstallOptionValue(params map[string]interface{}, keys ...string) (interface{}, bool) {
	options := resolvePluginHostMarketInstallOptions(params)
	for _, key := range keys {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			continue
		}
		if value, exists := options[trimmed]; exists {
			return value, true
		}
	}
	for _, key := range keys {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			continue
		}
		if value, exists := params[trimmed]; exists {
			return value, true
		}
	}
	return nil, false
}

func pluginHostMarketInstallBoolOption(params map[string]interface{}, defaultValue bool, keys ...string) (bool, error) {
	raw, exists := pluginHostMarketInstallOptionValue(params, keys...)
	if !exists {
		return defaultValue, nil
	}
	parsed, err := parsePluginHostOptionalBool(map[string]interface{}{"value": raw}, "value")
	if err != nil {
		return false, err
	}
	if parsed == nil {
		return defaultValue, nil
	}
	return *parsed, nil
}

func pluginHostMarketInstallStringOption(params map[string]interface{}, keys ...string) string {
	raw, exists := pluginHostMarketInstallOptionValue(params, keys...)
	if !exists {
		return ""
	}
	return strings.TrimSpace(pluginMarketStringFromAny(raw))
}

func pluginHostMarketInstallStringSliceOption(params map[string]interface{}, keys ...string) ([]string, bool) {
	raw, exists := pluginHostMarketInstallOptionValue(params, keys...)
	if !exists || raw == nil {
		return nil, false
	}
	switch typed := raw.(type) {
	case []string:
		return NormalizePluginPermissionList(typed), true
	case []interface{}:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			value := strings.TrimSpace(pluginMarketStringFromAny(item))
			if value == "" || value == "<nil>" {
				continue
			}
			out = append(out, value)
		}
		return NormalizePluginPermissionList(out), true
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return []string{}, true
		}
		parts := strings.FieldsFunc(trimmed, func(r rune) bool {
			return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t'
		})
		return NormalizePluginPermissionList(parts), true
	default:
		return nil, true
	}
}

func pluginHostMarketRuntimeConfig(runtime *PluginHostRuntime) *config.Config {
	if runtime != nil && runtime.Config != nil {
		return runtime.Config
	}
	return config.GetConfig()
}

func pluginHostMarketResolveArtifactRoot(cfg *config.Config) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("plugin config is unavailable")
	}
	artifactRoot := filepath.Clean(filepath.FromSlash(strings.TrimSpace(cfg.Plugin.ArtifactDir)))
	if artifactRoot == "" || artifactRoot == "." {
		artifactRoot = filepath.Join("data", "plugins")
	}
	if !filepath.IsAbs(artifactRoot) {
		absRoot, err := filepath.Abs(artifactRoot)
		if err != nil {
			return "", fmt.Errorf("resolve plugin artifact dir failed: %w", err)
		}
		artifactRoot = filepath.Clean(absRoot)
	}
	if err := os.MkdirAll(artifactRoot, 0o755); err != nil {
		return "", fmt.Errorf("prepare plugin artifact dir failed: %w", err)
	}
	return artifactRoot, nil
}

func pluginHostMarketResolveRuntime(
	pluginManager *PluginManagerService,
	cfg *config.Config,
	runtime string,
) (string, error) {
	if pluginManager != nil {
		return pluginManager.ResolveRuntime(runtime)
	}
	if cfg == nil {
		return "", fmt.Errorf("plugin config is unavailable")
	}
	resolved := strings.ToLower(strings.TrimSpace(runtime))
	if resolved == "" {
		resolved = strings.ToLower(strings.TrimSpace(cfg.Plugin.DefaultRuntime))
	}
	switch resolved {
	case PluginRuntimeGRPC, PluginRuntimeJSWorker:
	default:
		return "", fmt.Errorf("unsupported plugin runtime %q", resolved)
	}
	if !containsString(cfg.Plugin.AllowedRuntimes, resolved) {
		return "", fmt.Errorf("plugin runtime %q is not allowed by system settings", resolved)
	}
	return resolved, nil
}

func pluginHostMarketValidateProfile(
	pluginManager *PluginManagerService,
	cfg *config.Config,
	runtime string,
	pluginType string,
) error {
	if pluginManager != nil {
		return pluginManager.ValidatePluginProfile(runtime, pluginType)
	}
	resolvedRuntime, err := pluginHostMarketResolveRuntime(pluginManager, cfg, runtime)
	if err != nil {
		return err
	}
	normalizedType := strings.ToLower(strings.TrimSpace(pluginType))
	if normalizedType == "" {
		return fmt.Errorf("plugin type is required")
	}
	if cfg == nil || len(cfg.Plugin.AllowedTypes) == 0 {
		return nil
	}
	if !containsString(cfg.Plugin.AllowedTypes, normalizedType) {
		return fmt.Errorf("plugin type %q is not allowed by system settings", normalizedType)
	}
	if resolvedRuntime == "" {
		return fmt.Errorf("plugin runtime is required")
	}
	return nil
}

func pluginHostMarketDownloadReleaseArtifact(
	cfg *config.Config,
	source PluginMarketSource,
	kind string,
	name string,
	version string,
	release map[string]interface{},
) (*pluginHostMarketInstallArtifact, error) {
	artifactRoot, err := pluginHostMarketResolveArtifactRoot(cfg)
	if err != nil {
		return nil, err
	}

	downloadInfo, err := pluginHostMarketResolveDownloadInfo(source, kind, name, version, release)
	if err != nil {
		return nil, err
	}
	if downloadInfo.ExpectedSize > pluginHostMarketInstallMaxDownloadBytes {
		return nil, fmt.Errorf("market artifact exceeds download size limit")
	}

	packageDir := filepath.Join(
		artifactRoot,
		"market",
		sanitizeJSWorkerPathSegment(source.SourceID),
		sanitizeJSWorkerPathSegment(kind),
		sanitizeJSWorkerPathSegment(name),
		sanitizeJSWorkerPathSegment(version),
		sanitizeJSWorkerPathSegment(downloadInfo.ExpectedSHA[:minInt(len(downloadInfo.ExpectedSHA), 16)]),
	)
	if err := os.MkdirAll(packageDir, 0o755); err != nil {
		return nil, fmt.Errorf("prepare market artifact directory failed: %w", err)
	}

	packagePath := filepath.Join(packageDir, downloadInfo.PackageName)
	if checksum, exists := pluginHostMarketExistingFileSHA256(packagePath); exists && strings.EqualFold(checksum, downloadInfo.ExpectedSHA) {
		return &pluginHostMarketInstallArtifact{
			PackagePath: filepath.ToSlash(packagePath),
			PackageName: downloadInfo.PackageName,
			Checksum:    strings.ToLower(checksum),
		}, nil
	}

	tmpFile, err := os.CreateTemp(packageDir, "*.download")
	if err != nil {
		return nil, fmt.Errorf("create market artifact temp file failed: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpClosed := false
	defer func() {
		if !tmpClosed {
			_ = tmpFile.Close()
		}
		_ = os.Remove(tmpPath)
	}()

	reqCtx, cancel := context.WithTimeout(context.Background(), pluginHostMarketInstallDownloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, downloadInfo.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("build market artifact download request failed: %w", err)
	}
	req.Header.Set("Accept", "application/octet-stream, application/zip")

	allowedOrigin, err := parsePluginMarketHTTPURL(source.BaseURL, "market source base_url")
	if err != nil {
		return nil, err
	}
	client := newPluginMarketSameOriginHTTPClient(pluginHostMarketInstallDownloadTimeout, allowedOrigin)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download market artifact failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("download market artifact failed with status %d", resp.StatusCode)
	}

	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(tmpFile, hasher), io.LimitReader(resp.Body, pluginHostMarketInstallMaxDownloadBytes+1))
	if err != nil {
		return nil, fmt.Errorf("write market artifact failed: %w", err)
	}
	if written > pluginHostMarketInstallMaxDownloadBytes {
		return nil, fmt.Errorf("market artifact exceeds download size limit")
	}
	if closeErr := tmpFile.Close(); closeErr != nil {
		tmpClosed = true
		return nil, fmt.Errorf("close market artifact failed: %w", closeErr)
	}
	tmpClosed = true

	checksum := strings.ToLower(hex.EncodeToString(hasher.Sum(nil)))
	if !strings.EqualFold(checksum, downloadInfo.ExpectedSHA) {
		return nil, fmt.Errorf("market artifact checksum mismatch")
	}

	if err := os.Remove(packagePath); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("replace existing market artifact failed: %w", err)
	}
	if err := os.Rename(tmpPath, packagePath); err != nil {
		return nil, fmt.Errorf("finalize market artifact failed: %w", err)
	}

	return &pluginHostMarketInstallArtifact{
		PackagePath: filepath.ToSlash(packagePath),
		PackageName: downloadInfo.PackageName,
		Checksum:    checksum,
	}, nil
}

func pluginHostMarketResolveDownloadInfo(
	source PluginMarketSource,
	kind string,
	name string,
	version string,
	release map[string]interface{},
) (pluginHostMarketInstallDownloadInfo, error) {
	download := clonePluginMarketMap(release["download"])
	expectedSHA := strings.ToLower(strings.TrimSpace(pluginMarketStringFromAny(download["sha256"])))
	if expectedSHA == "" {
		return pluginHostMarketInstallDownloadInfo{}, fmt.Errorf("market release download sha256 is required")
	}

	baseURL, err := parsePluginMarketHTTPURL(source.BaseURL, "market source base_url")
	if err != nil {
		return pluginHostMarketInstallDownloadInfo{}, err
	}

	downloadURL := strings.TrimSpace(pluginMarketStringFromAny(download["url"]))
	if downloadURL == "" {
		parsedBaseURL := *baseURL
		parsedBaseURL.Path = strings.TrimRight(parsedBaseURL.Path, "/")
		parsedBaseURL.Path = path.Clean(parsedBaseURL.Path + "/" + strings.TrimLeft(pluginHostMarketReleaseDownloadPath(kind, name, version), "/"))
		downloadURL = parsedBaseURL.String()
	} else {
		parsedDownloadURL, err := resolvePluginMarketHTTPURL(baseURL, downloadURL, "market release download url")
		if err != nil {
			return pluginHostMarketInstallDownloadInfo{}, err
		}
		downloadURL = parsedDownloadURL.String()
	}

	expectedSize, _, _ := parsePluginHostOptionalInt64(download, "size")
	install := clonePluginMarketMap(release["install"])
	packageName := pluginHostMarketResolveDownloadPackageName(download, install, kind, name, version, downloadURL)

	return pluginHostMarketInstallDownloadInfo{
		URL:          downloadURL,
		PackageName:  packageName,
		ExpectedSHA:  expectedSHA,
		ExpectedSize: expectedSize,
	}, nil
}

func pluginHostMarketReleaseDownloadPath(kind string, name string, version string) string {
	return fmt.Sprintf(
		"/v1/artifacts/%s/%s/releases/%s/download",
		url.PathEscape(strings.TrimSpace(kind)),
		url.PathEscape(strings.TrimSpace(name)),
		url.PathEscape(strings.TrimSpace(version)),
	)
}

func pluginHostMarketResolveDownloadPackageName(
	download map[string]interface{},
	install map[string]interface{},
	kind string,
	name string,
	version string,
	downloadURL string,
) string {
	packageName := pluginHostMarketEnsureDownloadFileExtension(
		pluginHostMarketSanitizeFileName(strings.TrimSpace(pluginMarketStringFromAny(download["filename"]))),
		download,
		install,
		kind,
	)
	if packageName != "" {
		return packageName
	}

	parsedURL, err := url.Parse(strings.TrimSpace(downloadURL))
	if err == nil {
		candidate := pluginHostMarketSanitizeFileName(path.Base(parsedURL.Path))
		if pluginHostMarketDownloadURLFileNameUsable(candidate) {
			candidate = pluginHostMarketEnsureDownloadFileExtension(candidate, download, install, kind)
			if candidate != "" {
				return candidate
			}
		}
	}

	fallbackBase := pluginHostMarketFirstNonEmpty(name, kind, "artifact")
	if trimmedVersion := strings.TrimSpace(version); trimmedVersion != "" {
		fallbackBase += "-" + trimmedVersion
	}
	fallbackBase = pluginHostMarketSanitizeFileName(fallbackBase)
	if fallbackBase == "" {
		fallbackBase = "artifact"
	}
	extension := pluginHostMarketResolveDownloadFileExtension(download, install, kind)
	if extension != "" && !strings.HasSuffix(strings.ToLower(fallbackBase), strings.ToLower(extension)) {
		fallbackBase += extension
	}
	return pluginHostMarketSanitizeFileName(fallbackBase)
}

func pluginHostMarketDownloadURLFileNameUsable(name string) bool {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	if path.Ext(lower) == "" {
		switch lower {
		case "download", "artifact", "release", "file":
			return false
		}
	}
	return true
}

func pluginHostMarketEnsureDownloadFileExtension(
	name string,
	download map[string]interface{},
	install map[string]interface{},
	kind string,
) string {
	trimmed := pluginHostMarketSanitizeFileName(name)
	if trimmed == "" {
		return ""
	}
	if path.Ext(trimmed) != "" {
		return trimmed
	}
	extension := pluginHostMarketResolveDownloadFileExtension(download, install, kind)
	if extension == "" {
		return trimmed
	}
	return pluginHostMarketSanitizeFileName(trimmed + extension)
}

func pluginHostMarketResolveDownloadFileExtension(
	download map[string]interface{},
	install map[string]interface{},
	kind string,
) string {
	contentType := strings.ToLower(strings.TrimSpace(pluginMarketStringFromAny(download["content_type"])))
	if idx := strings.Index(contentType, ";"); idx >= 0 {
		contentType = strings.TrimSpace(contentType[:idx])
	}
	switch contentType {
	case "application/zip", "application/x-zip-compressed":
		return ".zip"
	case "application/gzip", "application/x-gzip":
		return ".gz"
	case "application/json", "text/json":
		return ".json"
	case "application/javascript", "text/javascript":
		return ".js"
	case "text/plain":
		return ".txt"
	}

	switch strings.ToLower(strings.TrimSpace(pluginMarketStringFromAny(install["package_format"]))) {
	case "zip":
		return ".zip"
	case "gzip", "gz", "tgz", "tar.gz":
		return ".gz"
	case "json":
		return ".json"
	case "js", "javascript":
		return ".js"
	}

	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "plugin_package", "payment_package", "email_template", "landing_page_template", "invoice_template", "auth_branding_template", "page_rule_pack":
		return ".zip"
	default:
		return ""
	}
}

func pluginHostMarketExistingFileSHA256(path string) (string, bool) {
	file, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil || info.IsDir() {
		return "", false
	}

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", false
	}
	return hex.EncodeToString(hasher.Sum(nil)), true
}

func pluginHostMarketReadPluginManifestFromPackage(packagePath string) (*pluginHostMarketPackageManifest, string, error) {
	reader, err := zip.OpenReader(packagePath)
	if err != nil {
		return nil, "", fmt.Errorf("read market plugin package failed: %w", err)
	}
	defer reader.Close()

	manifestNames := map[string]struct{}{
		"manifest.json":        {},
		"plugin.json":          {},
		"plugin-manifest.json": {},
	}
	for _, file := range reader.File {
		base := strings.ToLower(filepath.Base(file.Name))
		if _, exists := manifestNames[base]; !exists {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, "", fmt.Errorf("open market plugin manifest failed: %w", err)
		}
		raw, err := io.ReadAll(io.LimitReader(rc, pluginHostMarketInstallManifestReadLimitBytes))
		_ = rc.Close()
		if err != nil {
			return nil, "", fmt.Errorf("read market plugin manifest failed: %w", err)
		}

		var manifest pluginHostMarketPackageManifest
		if err := json.Unmarshal(raw, &manifest); err != nil {
			return nil, "", fmt.Errorf("market plugin manifest is invalid json: %w", err)
		}
		return &manifest, string(raw), nil
	}
	return nil, "", fmt.Errorf("market plugin package manifest not found")
}

func pluginHostMarketNormalizeJSONObject(raw string, fallback map[string]interface{}) (string, error) {
	value := fallback
	if value == nil {
		value = map[string]interface{}{}
	}
	body, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return string(body), nil
	}

	var decoded interface{}
	if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
		return "", err
	}
	if decoded == nil {
		decoded = map[string]interface{}{}
	}
	if _, ok := decoded.(map[string]interface{}); !ok {
		return "", fmt.Errorf("json must be object")
	}
	normalized, err := json.Marshal(decoded)
	if err != nil {
		return "", err
	}
	return string(normalized), nil
}

func pluginHostMarketMarshalJSONObject(value map[string]interface{}, fallback string) (string, error) {
	if value == nil {
		return pluginHostMarketNormalizeJSONObject(fallback, nil)
	}
	body, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return pluginHostMarketNormalizeJSONObject(string(body), nil)
}

func pluginHostMarketBuildPermissionRequests(
	manifest *pluginHostMarketPackageManifest,
	release map[string]interface{},
) ([]PluginPermissionRequest, []string) {
	if manifest == nil {
		return []PluginPermissionRequest{}, []string{}
	}

	reasonByKey := make(map[string]string)
	requested := NormalizePluginPermissionList(manifest.RequestedPermissions)
	required := NormalizePluginPermissionList(manifest.RequiredPermissions)
	for _, item := range manifest.Permissions {
		key := NormalizePluginPermissionKey(item.Key)
		if key == "" {
			continue
		}
		requested = append(requested, key)
		if item.Required {
			required = append(required, key)
		}
		if reason := item.Reason.String(); reason != "" {
			reasonByKey[key] = reason
		}
	}

	releasePermissions := clonePluginMarketMap(release["permissions"])
	if len(requested) == 0 {
		requested = NormalizePluginPermissionList(pluginMarketInterfaceSliceToStrings(releasePermissions["requested"]))
	}
	requests := BuildPluginPermissionRequests(requested, required, reasonByKey)
	defaultGranted := NormalizePluginPermissionList(pluginMarketInterfaceSliceToStrings(releasePermissions["default_granted"]))
	if len(defaultGranted) == 0 {
		defaultGranted = DefaultGrantedPluginPermissions(requests)
	}
	return requests, defaultGranted
}

func pluginHostMarketApplyGrantedPermissions(
	capabilitiesJSON string,
	requests []PluginPermissionRequest,
	granted []string,
) (string, error) {
	capabilities := map[string]interface{}{}
	trimmed := strings.TrimSpace(capabilitiesJSON)
	if trimmed != "" {
		if err := json.Unmarshal([]byte(trimmed), &capabilities); err != nil {
			return "", err
		}
	}
	requested := make([]string, 0, len(requests))
	for _, request := range requests {
		requested = append(requested, request.Key)
	}
	capabilities["requested_permissions"] = NormalizePluginPermissionList(requested)
	capabilities["granted_permissions"] = NormalizePluginPermissionList(granted)
	body, err := json.Marshal(capabilities)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func pluginHostMarketNormalizeVersion(version string) string {
	trimmed := strings.TrimSpace(version)
	if trimmed != "" {
		return trimmed
	}
	return "0.0.0"
}

func pluginHostMarketFirstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func pluginHostMarketSanitizeFileName(name string) string {
	base := filepath.Base(strings.TrimSpace(name))
	if base == "" || base == "." || base == ".." {
		return ""
	}
	var builder strings.Builder
	for _, ch := range base {
		switch {
		case ch >= 'a' && ch <= 'z':
			builder.WriteRune(ch)
		case ch >= 'A' && ch <= 'Z':
			builder.WriteRune(ch)
		case ch >= '0' && ch <= '9':
			builder.WriteRune(ch)
		case ch == '.', ch == '-', ch == '_':
			builder.WriteRune(ch)
		default:
			builder.WriteByte('_')
		}
	}
	return strings.TrimSpace(builder.String())
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}

func pluginHostMarketOptionalUint(value uint) *uint {
	if value == 0 {
		return nil
	}
	copied := value
	return &copied
}

func cloneOptionalUint(value *uint) *uint {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func pluginHostMarketPermissionKeys(requests []PluginPermissionRequest) []string {
	out := make([]string, 0, len(requests))
	for _, request := range requests {
		out = append(out, request.Key)
	}
	return NormalizePluginPermissionList(out)
}

func pluginHostMarketReleaseInstallEntry(release map[string]interface{}) string {
	install := clonePluginMarketMap(release["install"])
	return pluginHostMarketFirstNonEmpty(
		strings.TrimSpace(pluginMarketStringFromAny(install["entry"])),
		strings.TrimSpace(pluginMarketStringFromAny(install["address"])),
	)
}

func pluginHostMarketReleaseInstallBool(
	install map[string]interface{},
	key string,
	manifestValue *bool,
	defaultValue bool,
) bool {
	if install != nil {
		if parsed, ok := pluginMarketBoolFromAny(install[key]); ok {
			return parsed
		}
	}
	if manifestValue != nil {
		return *manifestValue
	}
	return defaultValue
}

func pluginHostMarketBuildPluginSummary(plugin *models.Plugin) map[string]interface{} {
	if plugin == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":                 plugin.ID,
		"name":               plugin.Name,
		"display_name":       plugin.DisplayName,
		"description":        plugin.Description,
		"type":               plugin.Type,
		"runtime":            plugin.Runtime,
		"address":            plugin.Address,
		"version":            plugin.Version,
		"package_path":       plugin.PackagePath,
		"package_checksum":   plugin.PackageChecksum,
		"enabled":            plugin.Enabled,
		"status":             plugin.Status,
		"lifecycle_status":   plugin.LifecycleStatus,
		"desired_generation": plugin.DesiredGeneration,
		"applied_generation": plugin.AppliedGeneration,
		"installed_at":       plugin.InstalledAt,
		"started_at":         plugin.StartedAt,
		"updated_at":         plugin.UpdatedAt,
	}
}

func pluginHostMarketBuildVersionSummary(version *models.PluginVersion) map[string]interface{} {
	if version == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":                      version.ID,
		"plugin_id":               version.PluginID,
		"version":                 version.Version,
		"market_source_id":        version.MarketSourceID,
		"market_artifact_kind":    version.MarketArtifactKind,
		"market_artifact_name":    version.MarketArtifactName,
		"market_artifact_version": version.MarketArtifactVersion,
		"package_name":            version.PackageName,
		"package_path":            version.PackagePath,
		"package_checksum":        version.PackageChecksum,
		"runtime":                 version.Runtime,
		"address":                 version.Address,
		"is_active":               version.IsActive,
		"lifecycle_status":        version.LifecycleStatus,
		"activated_at":            version.ActivatedAt,
		"created_at":              version.CreatedAt,
		"updated_at":              version.UpdatedAt,
	}
}

func pluginHostMarketBuildDeploymentSummary(record *models.PluginDeployment) map[string]interface{} {
	if record == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":                      record.ID,
		"task_id":                 pluginHostMarketTaskIDForDeploymentID(record.ID),
		"plugin_id":               record.PluginID,
		"operation":               record.Operation,
		"trigger":                 record.Trigger,
		"status":                  record.Status,
		"target_version_id":       record.TargetVersionID,
		"market_source_id":        record.MarketSourceID,
		"market_artifact_kind":    record.MarketArtifactKind,
		"market_artifact_name":    record.MarketArtifactName,
		"market_artifact_version": record.MarketArtifactVersion,
		"requested_generation":    record.RequestedGeneration,
		"applied_generation":      record.AppliedGeneration,
		"runtime_spec_hash":       record.RuntimeSpecHash,
		"auto_start":              record.AutoStart,
		"detail":                  record.Detail,
		"error":                   record.Error,
		"started_at":              record.StartedAt,
		"finished_at":             record.FinishedAt,
		"created_at":              record.CreatedAt,
		"updated_at":              record.UpdatedAt,
	}
}

func pluginHostMarketActivatePluginVersion(
	runtime *PluginHostRuntime,
	pluginID uint,
	versionID uint,
	autoStart bool,
	requestedBy *uint,
	detail string,
	operation string,
	trigger string,
	coordinates pluginHostMarketCoordinates,
) (*models.Plugin, *models.PluginVersion, *models.PluginDeployment, error) {
	db := runtime.database()
	if db == nil {
		return nil, nil, nil, fmt.Errorf("plugin host database is unavailable")
	}

	var plugin models.Plugin
	if err := db.First(&plugin, pluginID).Error; err != nil {
		return nil, nil, nil, err
	}
	originalPlugin := plugin

	var version models.PluginVersion
	if err := db.Where("id = ? AND plugin_id = ?", versionID, pluginID).First(&version).Error; err != nil {
		return nil, nil, nil, err
	}
	if err := ValidatePluginManifestCompatibility(version.Manifest, pluginHostMarketFirstNonEmpty(version.Runtime, plugin.Runtime)); err != nil {
		return nil, nil, nil, fmt.Errorf("plugin manifest compatibility check failed: %w", err)
	}

	var previousActive models.PluginVersion
	hadPreviousActive := false
	if err := db.Where("plugin_id = ? AND is_active = ?", pluginID, true).First(&previousActive).Error; err == nil {
		hadPreviousActive = true
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, nil, err
	}

	if err := pluginHostMarketMigrateJSWorkerWritableFilesForActivation(pluginHostMarketRuntimeConfig(runtime), &originalPlugin, &version); err != nil {
		return nil, nil, nil, fmt.Errorf("migrate js_worker writable files failed: %w", err)
	}

	requestedGeneration := ResolveNextPluginGeneration(&originalPlugin)
	targetPlugin := pluginHostMarketBuildActivatedPluginSnapshot(originalPlugin, version, requestedGeneration, "")
	targetRuntimeSpecHash := ComputePluginRuntimeSpecHash(&targetPlugin)
	if targetRuntimeSpecHash == "" {
		targetRuntimeSpecHash = ResolvePluginRuntimeSpecHash(&originalPlugin)
	}
	targetPlugin.RuntimeSpecHash = targetRuntimeSpecHash
	shouldStart := autoStart || originalPlugin.Enabled

	deployment, err := pluginHostMarketCreateDeployment(
		db,
		pluginID,
		operation,
		trigger,
		&versionID,
		requestedGeneration,
		targetRuntimeSpecHash,
		shouldStart,
		requestedBy,
		detail,
		coordinates,
	)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := pluginHostMarketMarkDeploymentRunning(db, deployment, detail); err != nil {
		return nil, nil, deployment, err
	}

	now := time.Now().UTC()
	update := map[string]interface{}{
		"version":            pluginHostMarketNormalizeVersion(version.Version),
		"package_path":       version.PackagePath,
		"package_checksum":   version.PackageChecksum,
		"runtime_spec_hash":  targetRuntimeSpecHash,
		"desired_generation": requestedGeneration,
		"applied_generation": requestedGeneration,
		"lifecycle_status":   models.PluginLifecycleInstalled,
		"installed_at":       now,
		"last_error":         "",
		"retired_at":         nil,
	}
	if strings.TrimSpace(version.Type) != "" {
		update["type"] = version.Type
	}
	if strings.TrimSpace(version.Runtime) != "" {
		update["runtime"] = version.Runtime
	}
	if strings.TrimSpace(version.Address) != "" {
		update["address"] = version.Address
	}
	if strings.TrimSpace(version.ConfigSnapshot) != "" {
		update["config"] = version.ConfigSnapshot
	}
	if strings.TrimSpace(version.RuntimeParams) != "" {
		update["runtime_params"] = version.RuntimeParams
	}
	if strings.TrimSpace(version.CapabilitiesSnapshot) != "" {
		update["capabilities"] = version.CapabilitiesSnapshot
	}
	if strings.TrimSpace(version.Manifest) != "" {
		update["manifest"] = version.Manifest
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.Plugin{}).Where("id = ?", pluginID).Updates(update).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.PluginVersion{}).Where("plugin_id = ?", pluginID).Updates(map[string]interface{}{
			"is_active":        false,
			"lifecycle_status": models.PluginLifecycleUploaded,
		}).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.PluginVersion{}).Where("id = ?", versionID).Updates(map[string]interface{}{
			"is_active":        true,
			"activated_at":     now,
			"lifecycle_status": models.PluginLifecycleInstalled,
		}).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		_ = pluginHostMarketMarkDeploymentFailed(db, deployment, err)
		return nil, nil, deployment, err
	}

	pluginHostMarketInvalidateJSWorkerProgramCaches(&originalPlugin, &targetPlugin)
	if shouldStart {
		if runtime == nil || runtime.PluginManager == nil {
			err := fmt.Errorf("plugin manager is unavailable")
			rollbackErr := pluginHostMarketRollbackActivatedVersion(db, pluginID, originalPlugin, previousActive, hadPreviousActive)
			if rollbackErr != nil {
				combined := fmt.Errorf("start plugin failed: %v; rollback failed: %v", err, rollbackErr)
				_ = pluginHostMarketMarkDeploymentFailed(db, deployment, combined)
				return nil, nil, deployment, combined
			}
			_ = pluginHostMarketMarkDeploymentFailed(db, deployment, err)
			return nil, nil, deployment, err
		}
		if err := runtime.PluginManager.StartPlugin(pluginID); err != nil {
			rollbackErr := pluginHostMarketRollbackActivatedVersion(db, pluginID, originalPlugin, previousActive, hadPreviousActive)
			if rollbackErr != nil {
				combined := fmt.Errorf("start plugin failed: %v; rollback failed: %v", err, rollbackErr)
				_ = pluginHostMarketMarkDeploymentFailed(db, deployment, combined)
				return nil, nil, deployment, combined
			}
			_ = pluginHostMarketMarkDeploymentFailed(db, deployment, err)
			return nil, nil, deployment, fmt.Errorf("start plugin failed: %w", err)
		}
	}

	if err := db.First(&plugin, pluginID).Error; err != nil {
		_ = pluginHostMarketMarkDeploymentFailed(db, deployment, err)
		return nil, nil, deployment, err
	}
	if err := db.First(&version, versionID).Error; err != nil {
		_ = pluginHostMarketMarkDeploymentFailed(db, deployment, err)
		return nil, nil, deployment, err
	}

	successDetail := strings.TrimSpace(detail)
	if successDetail == "" {
		successDetail = "market install activated"
	}
	_ = pluginHostMarketMarkDeploymentSucceeded(db, deployment, requestedGeneration, successDetail)
	return &plugin, &version, deployment, nil
}

func pluginHostMarketCreateDeployment(
	db *gorm.DB,
	pluginID uint,
	operation string,
	trigger string,
	targetVersionID *uint,
	requestedGeneration uint,
	runtimeSpecHash string,
	autoStart bool,
	requestedBy *uint,
	detail string,
	coordinates pluginHostMarketCoordinates,
) (*models.PluginDeployment, error) {
	if db == nil {
		return nil, nil
	}
	record := &models.PluginDeployment{
		PluginID:              pluginID,
		Operation:             strings.TrimSpace(operation),
		Trigger:               strings.TrimSpace(trigger),
		Status:                models.PluginDeploymentStatusPending,
		TargetVersionID:       cloneOptionalUint(targetVersionID),
		MarketSourceID:        coordinates.SourceID,
		MarketArtifactKind:    coordinates.Kind,
		MarketArtifactName:    coordinates.Name,
		MarketArtifactVersion: coordinates.Version,
		RequestedGeneration:   requestedGeneration,
		AppliedGeneration:     0,
		RuntimeSpecHash:       strings.TrimSpace(runtimeSpecHash),
		AutoStart:             autoStart,
		RequestedBy:           cloneOptionalUint(requestedBy),
		Detail:                strings.TrimSpace(detail),
	}
	if err := db.Create(record).Error; err != nil {
		return nil, err
	}
	return record, nil
}

func pluginHostMarketMarkDeploymentRunning(db *gorm.DB, record *models.PluginDeployment, detail string) error {
	if db == nil || record == nil || record.ID == 0 {
		return nil
	}
	now := time.Now().UTC()
	update := map[string]interface{}{
		"status":     models.PluginDeploymentStatusRunning,
		"started_at": now,
	}
	if trimmed := strings.TrimSpace(detail); trimmed != "" {
		update["detail"] = trimmed
		record.Detail = trimmed
	}
	record.Status = models.PluginDeploymentStatusRunning
	record.StartedAt = &now
	return db.Model(&models.PluginDeployment{}).Where("id = ?", record.ID).Updates(update).Error
}

func pluginHostMarketMarkDeploymentSucceeded(
	db *gorm.DB,
	record *models.PluginDeployment,
	appliedGeneration uint,
	detail string,
) error {
	if db == nil || record == nil || record.ID == 0 {
		return nil
	}
	now := time.Now().UTC()
	update := map[string]interface{}{
		"status":             models.PluginDeploymentStatusSucceeded,
		"applied_generation": appliedGeneration,
		"finished_at":        now,
		"error":              "",
	}
	if trimmed := strings.TrimSpace(detail); trimmed != "" {
		update["detail"] = trimmed
		record.Detail = trimmed
	}
	record.Status = models.PluginDeploymentStatusSucceeded
	record.AppliedGeneration = appliedGeneration
	record.FinishedAt = &now
	record.Error = ""
	return db.Model(&models.PluginDeployment{}).Where("id = ?", record.ID).Updates(update).Error
}

func pluginHostMarketMarkDeploymentFailed(db *gorm.DB, record *models.PluginDeployment, err error) error {
	if db == nil || record == nil || record.ID == 0 {
		return nil
	}
	now := time.Now().UTC()
	errText := ""
	if err != nil {
		errText = strings.TrimSpace(err.Error())
	}
	record.Status = models.PluginDeploymentStatusFailed
	record.Error = errText
	record.FinishedAt = &now
	return db.Model(&models.PluginDeployment{}).Where("id = ?", record.ID).Updates(map[string]interface{}{
		"status":      models.PluginDeploymentStatusFailed,
		"error":       errText,
		"finished_at": now,
	}).Error
}

func pluginHostMarketRollbackActivatedVersion(
	db *gorm.DB,
	pluginID uint,
	originalPlugin models.Plugin,
	previousActive models.PluginVersion,
	hadPreviousActive bool,
) error {
	if db == nil {
		return nil
	}
	restorePlugin := map[string]interface{}{
		"version":            pluginHostMarketNormalizeVersion(originalPlugin.Version),
		"package_path":       originalPlugin.PackagePath,
		"package_checksum":   originalPlugin.PackageChecksum,
		"lifecycle_status":   originalPlugin.LifecycleStatus,
		"installed_at":       originalPlugin.InstalledAt,
		"last_error":         originalPlugin.LastError,
		"retired_at":         originalPlugin.RetiredAt,
		"type":               originalPlugin.Type,
		"runtime":            originalPlugin.Runtime,
		"address":            originalPlugin.Address,
		"config":             originalPlugin.Config,
		"runtime_params":     originalPlugin.RuntimeParams,
		"capabilities":       originalPlugin.Capabilities,
		"manifest":           originalPlugin.Manifest,
		"runtime_spec_hash":  originalPlugin.RuntimeSpecHash,
		"desired_generation": originalPlugin.DesiredGeneration,
		"applied_generation": originalPlugin.AppliedGeneration,
		"enabled":            originalPlugin.Enabled,
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.Plugin{}).Where("id = ?", pluginID).Updates(restorePlugin).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.PluginVersion{}).Where("plugin_id = ?", pluginID).Update("is_active", false).Error; err != nil {
			return err
		}
		if !hadPreviousActive {
			return nil
		}
		restoreVersion := map[string]interface{}{
			"is_active":        true,
			"lifecycle_status": previousActive.LifecycleStatus,
			"activated_at":     previousActive.ActivatedAt,
		}
		return tx.Model(&models.PluginVersion{}).Where("id = ?", previousActive.ID).Updates(restoreVersion).Error
	})
}

func pluginHostMarketBuildActivatedPluginSnapshot(
	base models.Plugin,
	version models.PluginVersion,
	requestedGeneration uint,
	runtimeSpecHash string,
) models.Plugin {
	snapshot := base
	snapshot.Version = pluginHostMarketNormalizeVersion(version.Version)
	snapshot.PackagePath = version.PackagePath
	snapshot.PackageChecksum = version.PackageChecksum
	snapshot.RuntimeSpecHash = strings.TrimSpace(runtimeSpecHash)
	if requestedGeneration > 0 {
		snapshot.DesiredGeneration = requestedGeneration
		snapshot.AppliedGeneration = requestedGeneration
	}
	if strings.TrimSpace(version.Type) != "" {
		snapshot.Type = version.Type
	}
	if strings.TrimSpace(version.Runtime) != "" {
		snapshot.Runtime = version.Runtime
	}
	if strings.TrimSpace(version.Address) != "" {
		snapshot.Address = version.Address
	}
	if strings.TrimSpace(version.ConfigSnapshot) != "" {
		snapshot.Config = version.ConfigSnapshot
	}
	if strings.TrimSpace(version.RuntimeParams) != "" {
		snapshot.RuntimeParams = version.RuntimeParams
	}
	if strings.TrimSpace(version.CapabilitiesSnapshot) != "" {
		snapshot.Capabilities = version.CapabilitiesSnapshot
	}
	if strings.TrimSpace(version.Manifest) != "" {
		snapshot.Manifest = version.Manifest
	}
	return snapshot
}

func pluginHostMarketInvalidateJSWorkerProgramCaches(plugins ...*models.Plugin) {
	for _, plugin := range plugins {
		if plugin == nil {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(plugin.Runtime), PluginRuntimeJSWorker) {
			continue
		}
		if scriptPath, err := ResolveJSWorkerScriptPath(plugin.Address, plugin.PackagePath); err == nil {
			jsworker.InvalidateWorkerProgramCachePath(scriptPath)
		}
		if root, err := ResolveJSWorkerPackageRoot(plugin.PackagePath); err == nil {
			jsworker.InvalidateWorkerProgramCachePathPrefix(root)
		}
	}
}

func pluginHostMarketPrepareJSWorkerPackage(packagePath string, requestedAddress string) (string, string, string, error) {
	normalizedPackagePath, err := NormalizeJSWorkerPackagePath(packagePath)
	if err != nil {
		return "", "", "", err
	}

	extractRoot, err := ResolveJSWorkerPackageRoot(normalizedPackagePath)
	if err != nil {
		return "", "", "", err
	}
	if info, statErr := os.Stat(normalizedPackagePath); statErr == nil && !info.IsDir() {
		if filepath.Clean(extractRoot) == filepath.Clean(normalizedPackagePath) {
			extractRoot = jsWorkerDerivedExtractRoot(normalizedPackagePath)
		}
	}
	extractRootPublic := filepath.ToSlash(extractRoot)

	if err := os.RemoveAll(extractRoot); err != nil && !os.IsNotExist(err) {
		return "", normalizedPackagePath, extractRootPublic, fmt.Errorf("reset js_worker extract root failed: %w", err)
	}
	if err := pluginHostMarketUnzipPackageSafe(normalizedPackagePath, extractRoot); err != nil {
		return "", normalizedPackagePath, extractRootPublic, err
	}

	entry := strings.TrimSpace(requestedAddress)
	if entry == "" {
		entry, _ = pluginHostMarketFindDefaultJSWorkerEntryFile(extractRoot)
	}
	if entry == "" {
		firstJS, err := pluginHostMarketFindFirstJSFile(extractRoot)
		if err != nil {
			return "", normalizedPackagePath, extractRootPublic, fmt.Errorf("detect js_worker entry failed: %w", err)
		}
		entry = firstJS
	}

	normalizedAddress, err := NormalizeJSWorkerRelativeEntryPath(normalizedPackagePath, entry)
	if err != nil {
		return "", normalizedPackagePath, extractRootPublic, err
	}
	return normalizedAddress, normalizedPackagePath, extractRootPublic, nil
}

func pluginHostMarketUnzipPackageSafe(zipPath string, destDir string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open package zip failed: %w", err)
	}
	defer reader.Close()

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("create extract dir failed: %w", err)
	}
	rootClean := filepath.Clean(destDir)

	var fileCount int
	var declaredTotalBytes uint64
	var extractedTotalBytes int64
	for _, file := range reader.File {
		fileCount++
		if fileCount > pluginHostMarketInstallMaxPackageFiles {
			return fmt.Errorf("zip contains too many entries")
		}

		targetPath := filepath.Clean(filepath.Join(destDir, filepath.FromSlash(file.Name)))
		if !pluginHostMarketIsPathWithinRoot(rootClean, targetPath) {
			return fmt.Errorf("invalid zip entry path: %s", file.Name)
		}
		if file.FileInfo().Mode()&os.ModeSymlink != 0 || file.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("zip entry contains symlink: %s", file.Name)
		}

		if !file.FileInfo().IsDir() {
			if file.UncompressedSize64 > uint64(pluginHostMarketInstallMaxSingleFileBytes) {
				return fmt.Errorf("zip entry %s exceeds single-file limit", file.Name)
			}
			declaredTotalBytes += file.UncompressedSize64
			if declaredTotalBytes > uint64(pluginHostMarketInstallMaxDownloadBytes) {
				return fmt.Errorf("zip declared uncompressed size exceeds limit")
			}
			if file.CompressedSize64 > 0 {
				ratio := float64(file.UncompressedSize64) / float64(file.CompressedSize64)
				if ratio > pluginHostMarketInstallMaxCompressionRatio {
					return fmt.Errorf("zip entry %s compression ratio too high", file.Name)
				}
			} else if file.UncompressedSize64 > 0 {
				return fmt.Errorf("zip entry %s has invalid compressed size", file.Name)
			}
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return fmt.Errorf("create folder failed for %s: %w", file.Name, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return fmt.Errorf("create parent folder failed for %s: %w", file.Name, err)
		}
		src, err := file.Open()
		if err != nil {
			return fmt.Errorf("open zip entry %s failed: %w", file.Name, err)
		}
		dst, err := os.Create(targetPath)
		if err != nil {
			_ = src.Close()
			return fmt.Errorf("create target file %s failed: %w", targetPath, err)
		}
		written, copyErr := io.Copy(dst, io.LimitReader(src, pluginHostMarketInstallMaxSingleFileBytes+1))
		closeErr := dst.Close()
		_ = src.Close()
		if closeErr != nil {
			_ = os.Remove(targetPath)
			return fmt.Errorf("close target file %s failed: %w", targetPath, closeErr)
		}
		if copyErr != nil {
			_ = os.Remove(targetPath)
			return fmt.Errorf("extract zip entry %s failed: %w", file.Name, copyErr)
		}
		if written > pluginHostMarketInstallMaxSingleFileBytes {
			_ = os.Remove(targetPath)
			return fmt.Errorf("zip entry %s exceeds single-file limit", file.Name)
		}
		extractedTotalBytes += written
		if extractedTotalBytes > pluginHostMarketInstallMaxDownloadBytes {
			_ = os.Remove(targetPath)
			return fmt.Errorf("zip extracted size exceeds total limit")
		}
	}
	return nil
}

func pluginHostMarketFindDefaultJSWorkerEntryFile(root string) (string, string) {
	for _, ext := range []string{".js", ".mjs", ".cjs", ".jsx", ".ts", ".tsx"} {
		relative := "index" + ext
		candidate := filepath.Join(root, relative)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, relative
		}
	}
	return "", ""
}

func pluginHostMarketFindFirstJSFile(root string) (string, error) {
	var first string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if pluginHostMarketIsSupportedJSWorkerEntryFile(path) {
			first = path
			return io.EOF
		}
		return nil
	})
	if errors.Is(err, io.EOF) && first != "" {
		return first, nil
	}
	if err != nil {
		return "", err
	}
	return "", fmt.Errorf("no supported js_worker entry found in package")
}

func pluginHostMarketIsSupportedJSWorkerEntryFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".js", ".mjs", ".cjs", ".jsx", ".ts", ".tsx":
		return true
	default:
		return false
	}
}

func pluginHostMarketMigrateJSWorkerWritableFilesForActivation(cfg *config.Config, current *models.Plugin, next *models.PluginVersion) error {
	if current == nil || next == nil || current.ID == 0 {
		return nil
	}
	runtimeName := strings.ToLower(strings.TrimSpace(pluginHostMarketFirstNonEmpty(next.Runtime, current.Runtime)))
	if runtimeName != PluginRuntimeJSWorker {
		return nil
	}

	sourceScript, err := ResolveJSWorkerScriptPath(current.Address, current.PackagePath)
	if err != nil || strings.TrimSpace(sourceScript) == "" {
		return nil
	}
	sourceRoot := pluginHostMarketDetectModuleRoot(sourceScript)
	if sourceRoot == "" || !pluginHostMarketIsExistingDir(sourceRoot) {
		return nil
	}

	artifactRoot, err := pluginHostMarketResolveArtifactRoot(cfg)
	if err != nil {
		return err
	}
	targetRoot := filepath.Clean(filepath.Join(artifactRoot, "data", fmt.Sprintf("plugin_%d", current.ID)))
	if err := os.MkdirAll(targetRoot, 0o755); err != nil {
		return fmt.Errorf("prepare plugin data root failed: %w", err)
	}
	return pluginHostMarketCopyMigratableFiles(sourceRoot, targetRoot)
}

func pluginHostMarketCopyMigratableFiles(sourceRoot string, targetRoot string) error {
	sourceRoot = filepath.Clean(sourceRoot)
	targetRoot = filepath.Clean(targetRoot)
	return filepath.WalkDir(sourceRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		cleanPath := filepath.Clean(path)
		if cleanPath == sourceRoot {
			return nil
		}
		if !pluginHostMarketIsPathWithinRoot(sourceRoot, cleanPath) {
			return fmt.Errorf("source path escapes module root: %s", cleanPath)
		}

		rel, err := filepath.Rel(sourceRoot, cleanPath)
		if err != nil {
			return err
		}
		rel = filepath.Clean(rel)
		if rel == "." || rel == "" {
			return nil
		}

		targetPath := filepath.Clean(filepath.Join(targetRoot, rel))
		if !pluginHostMarketIsPathWithinRoot(targetRoot, targetPath) {
			return fmt.Errorf("target path escapes data root: %s", targetPath)
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if d.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}
		if !pluginHostMarketShouldMigrateFile(rel) {
			return nil
		}
		if _, err := os.Stat(targetPath); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		_, err = pluginHostMarketCopyFileWithMode(cleanPath, targetPath, info.Mode().Perm())
		return err
	})
}

func pluginHostMarketShouldMigrateFile(relPath string) bool {
	slashRel := filepath.ToSlash(filepath.Clean(relPath))
	loweredRel := strings.ToLower(slashRel)
	base := strings.ToLower(filepath.Base(loweredRel))
	switch base {
	case "manifest.json", "plugin.json", "plugin-manifest.json":
		return false
	}
	switch strings.ToLower(filepath.Ext(base)) {
	case ".js", ".mjs", ".cjs", ".jsx", ".ts", ".tsx", ".map":
		return false
	}
	return true
}

func pluginHostMarketCopyFileWithMode(sourcePath string, targetPath string, mode os.FileMode) (int64, error) {
	src, err := os.Open(sourcePath)
	if err != nil {
		return 0, err
	}
	defer src.Close()

	if mode == 0 {
		mode = 0o644
	}
	dst, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return 0, err
	}
	defer dst.Close()

	return io.Copy(dst, src)
}

func pluginHostMarketDetectModuleRoot(entryScriptPath string) string {
	trimmed := strings.TrimSpace(entryScriptPath)
	if trimmed == "" {
		return ""
	}
	current := filepath.Clean(filepath.Dir(filepath.FromSlash(trimmed)))
	for i := 0; i < 16; i++ {
		if pluginHostMarketHasManifest(current) {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return filepath.Clean(filepath.Dir(filepath.FromSlash(trimmed)))
}

func pluginHostMarketHasManifest(dir string) bool {
	for _, name := range []string{"manifest.json", "plugin.json", "plugin-manifest.json"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

func pluginHostMarketIsExistingDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func pluginHostMarketIsPathWithinRoot(root string, target string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(target))
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}
