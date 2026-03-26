package service

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"auralogic/internal/models"
	"gorm.io/gorm"
)

type marketPaymentPackageWebhookManifest struct {
	Key         string                `json:"key"`
	Description ManifestLocalizedText `json:"description"`
	Action      string                `json:"action"`
	Method      string                `json:"method"`
	AuthMode    string                `json:"auth_mode"`
	SecretKey   string                `json:"secret_key"`
}

type marketPaymentPackageManifest struct {
	Name                   string                                `json:"name"`
	DisplayName            ManifestLocalizedText                 `json:"display_name"`
	Description            ManifestLocalizedText                 `json:"description"`
	Icon                   string                                `json:"icon"`
	Runtime                string                                `json:"runtime"`
	Address                string                                `json:"address"`
	Entry                  string                                `json:"entry"`
	Version                string                                `json:"version"`
	PollInterval           *int                                  `json:"poll_interval"`
	ManifestVersion        string                                `json:"manifest_version"`
	ProtocolVersion        string                                `json:"protocol_version"`
	MinHostProtocolVersion string                                `json:"min_host_protocol_version"`
	MaxHostProtocolVersion string                                `json:"max_host_protocol_version"`
	Config                 map[string]interface{}                `json:"config"`
	ConfigSchema           map[string]interface{}                `json:"config_schema"`
	Webhooks               []marketPaymentPackageWebhookManifest `json:"webhooks"`
}

type marketPaymentPackageResolved struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	Version      string `json:"version"`
	Entry        string `json:"entry"`
	Icon         string `json:"icon"`
	Config       string `json:"config"`
	PollInterval int    `json:"poll_interval"`
	Script       string `json:"-"`
	ScriptBytes  int    `json:"script_bytes"`
	Checksum     string `json:"checksum"`
}

func PreviewPaymentMethodMarketPackageWithSource(
	runtime *PluginHostRuntime,
	source PluginMarketSource,
	name string,
	version string,
	targetMethodID uint,
) (map[string]interface{}, error) {
	normalizedSource, err := normalizeAdminPluginMarketSource(source)
	if err != nil {
		return nil, err
	}
	if !normalizedSource.AllowsKind("payment_package") {
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
	release, err := client.FetchRelease(context.Background(), normalizedSource, "payment_package", trimmedName, trimmedVersion)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadGateway, Message: err.Error()}
	}

	return previewPaymentMethodMarketPackageRelease(
		runtime,
		normalizedSource,
		trimmedName,
		trimmedVersion,
		release,
		targetMethodID,
	)
}

func ExecutePaymentMethodMarketPackageWithSource(
	runtime *PluginHostRuntime,
	source PluginMarketSource,
	name string,
	version string,
	params map[string]interface{},
) (map[string]interface{}, error) {
	normalizedSource, err := normalizeAdminPluginMarketSource(source)
	if err != nil {
		return nil, err
	}
	if !normalizedSource.AllowsKind("payment_package") {
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
	release, err := client.FetchRelease(context.Background(), normalizedSource, "payment_package", trimmedName, trimmedVersion)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadGateway, Message: err.Error()}
	}

	compatibility, warnings := buildPluginMarketCompatibilityPreview(release)
	if compatible, _ := compatibility["compatible"].(bool); !compatible {
		return nil, &PluginHostActionError{Status: http.StatusConflict, Message: "market release is not compatible with current host bridge"}
	}
	coordinates := newPluginHostMarketCoordinates(normalizedSource.SourceID, "payment_package", trimmedName, trimmedVersion)

	cfg := pluginHostMarketRuntimeConfig(runtime)
	if cfg == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "plugin config is unavailable"}
	}

	artifact, err := pluginHostMarketDownloadReleaseArtifact(cfg, normalizedSource, "payment_package", trimmedName, trimmedVersion, release)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadGateway, Message: err.Error()}
	}

	result, err := importPaymentMethodPackageArchiveWithOptions(
		runtime,
		artifact.PackagePath,
		artifact.PackageName,
		artifact.Checksum,
		params,
		paymentMethodPackageImportOptions{
			Coordinates:   coordinates,
			Source:        normalizedSource.Summary(),
			Release:       release,
			Governance:    clonePluginMarketMap(release["governance"]),
			Download:      clonePluginMarketMap(release["download"]),
			Compatibility: compatibility,
			Warnings:      warnings,
		},
	)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func previewPaymentMethodMarketPackageRelease(
	runtime *PluginHostRuntime,
	source PluginMarketSource,
	name string,
	version string,
	release map[string]interface{},
	targetMethodID uint,
) (map[string]interface{}, error) {
	compatibility, warnings := buildPluginMarketCompatibilityPreview(release)
	db := runtime.database()
	cfg := pluginHostMarketRuntimeConfig(runtime)
	if cfg == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "plugin config is unavailable"}
	}

	artifact, err := pluginHostMarketDownloadReleaseArtifact(cfg, source, "payment_package", name, version, release)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadGateway, Message: err.Error()}
	}

	manifest, _, err := readMarketPaymentPackageManifestFromPackage(artifact.PackagePath)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}

	targetMethod, err := loadMarketPaymentMethodTarget(db, targetMethodID)
	if err != nil {
		return nil, err
	}

	resolved, err := resolveMarketPaymentPackage(artifact.PackagePath, artifact.Checksum, manifest, "", targetMethod)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}

	result := map[string]interface{}{
		"source":        source.Summary(),
		"coordinates":   newPluginHostMarketCoordinates(source.SourceID, "payment_package", name, version).Map(),
		"release":       release,
		"governance":    clonePluginMarketMap(release["governance"]),
		"download":      clonePluginMarketMap(release["download"]),
		"compatibility": compatibility,
		"target_state":  buildMarketPaymentMethodTargetState(db, targetMethod, resolved.Version, resolved.Name),
		"manifest":      manifest,
		"resolved":      resolved,
	}
	if len(warnings) > 0 {
		result["warnings"] = clonePluginMarketStringSlice(warnings)
	}
	return result, nil
}

func readMarketPaymentPackageManifestFromPackage(packagePath string) (*marketPaymentPackageManifest, string, error) {
	reader, err := zip.OpenReader(packagePath)
	if err != nil {
		return nil, "", fmt.Errorf("read market payment package failed: %w", err)
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
			return nil, "", fmt.Errorf("open market payment manifest failed: %w", err)
		}
		raw, err := io.ReadAll(io.LimitReader(rc, pluginHostMarketInstallManifestReadLimitBytes))
		_ = rc.Close()
		if err != nil {
			return nil, "", fmt.Errorf("read market payment manifest failed: %w", err)
		}

		var manifest marketPaymentPackageManifest
		if err := json.Unmarshal(raw, &manifest); err != nil {
			return nil, "", fmt.Errorf("market payment manifest is invalid json: %w", err)
		}
		if err := validateMarketPaymentPackageManifest(&manifest); err != nil {
			return nil, "", err
		}
		return &manifest, string(raw), nil
	}
	return nil, "", fmt.Errorf("market payment package manifest not found")
}

func validateMarketPaymentPackageManifest(manifest *marketPaymentPackageManifest) error {
	if manifest == nil {
		return fmt.Errorf("payment package manifest is required")
	}
	if err := validateMarketPaymentPackageSchema(manifest.ConfigSchema, "config_schema"); err != nil {
		return err
	}
	if err := validateMarketPaymentPackageWebhooks(manifest.Webhooks); err != nil {
		return err
	}
	inspection := InspectPluginManifestCompatibilityMetadata(
		strings.TrimSpace(manifest.Runtime),
		manifest.ManifestVersion,
		manifest.ProtocolVersion,
		manifest.MinHostProtocolVersion,
		manifest.MaxHostProtocolVersion,
		true,
	)
	if !inspection.Compatible {
		return fmt.Errorf("payment package manifest compatibility is invalid: %s", inspection.Reason)
	}
	return nil
}

func validateMarketPaymentPackageSchema(schema map[string]interface{}, schemaName string) error {
	if len(schema) == 0 {
		return nil
	}
	fieldsRaw, exists := schema["fields"]
	if !exists {
		return fmt.Errorf("%s.fields is required", schemaName)
	}
	fields, ok := fieldsRaw.([]interface{})
	if !ok || len(fields) == 0 {
		return fmt.Errorf("%s.fields must be a non-empty array", schemaName)
	}

	seen := make(map[string]struct{}, len(fields))
	for idx, item := range fields {
		fieldPath := fmt.Sprintf("%s.fields[%d]", schemaName, idx)
		field, ok := item.(map[string]interface{})
		if !ok {
			return fmt.Errorf("%s must be an object", fieldPath)
		}
		key := strings.TrimSpace(pluginMarketStringFromAny(field["key"]))
		if key == "" {
			return fmt.Errorf("%s.key is required", fieldPath)
		}
		if _, exists := seen[key]; exists {
			return fmt.Errorf("%s.key duplicates %q", fieldPath, key)
		}
		seen[key] = struct{}{}

		fieldType, err := normalizeMarketPaymentSchemaFieldType(field["type"])
		if err != nil {
			return fmt.Errorf("%s.type %v is unsupported", fieldPath, field["type"])
		}
		if err := validateMarketPaymentSchemaDefaultValue(field["default"], fieldType, fieldPath); err != nil {
			return err
		}

		options, hasOptions, err := validateMarketPaymentSchemaFieldOptions(field["options"], fieldPath)
		if err != nil {
			return err
		}
		if fieldType == "select" {
			if !hasOptions || len(options) == 0 {
				return fmt.Errorf("%s.options must be a non-empty array for select fields", fieldPath)
			}
			if defaultValue, exists := field["default"]; exists {
				fingerprint := marketPaymentSchemaValueFingerprint(defaultValue)
				matched := false
				for _, optionFingerprint := range options {
					if optionFingerprint == fingerprint {
						matched = true
						break
					}
				}
				if !matched {
					return fmt.Errorf("%s.default must match one of the select options", fieldPath)
				}
			}
		}
	}
	return nil
}

func normalizeMarketPaymentSchemaFieldType(raw interface{}) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", raw)))
	if trimmed == "" || trimmed == "<nil>" {
		return "string", nil
	}
	switch trimmed {
	case "string", "textarea", "number", "boolean", "select", "json", "secret":
		return trimmed, nil
	default:
		return "", fmt.Errorf("unsupported field type %q", trimmed)
	}
}

func validateMarketPaymentSchemaDefaultValue(value interface{}, fieldType string, fieldPath string) error {
	if value == nil {
		return nil
	}
	switch fieldType {
	case "string", "textarea", "secret":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("%s.default must be a string", fieldPath)
		}
	case "number":
		if !marketPaymentSchemaIsNumber(value) {
			return fmt.Errorf("%s.default must be a number", fieldPath)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%s.default must be a boolean", fieldPath)
		}
	}
	return nil
}

func validateMarketPaymentSchemaFieldOptions(raw interface{}, fieldPath string) ([]string, bool, error) {
	if raw == nil {
		return nil, false, nil
	}
	items, ok := raw.([]interface{})
	if !ok {
		return nil, true, fmt.Errorf("%s.options must be an array", fieldPath)
	}

	fingerprints := make([]string, 0, len(items))
	for idx, item := range items {
		optionPath := fmt.Sprintf("%s.options[%d]", fieldPath, idx)
		switch typed := item.(type) {
		case nil:
			continue
		case map[string]interface{}:
			value, exists := typed["value"]
			if !exists {
				value = typed["key"]
			}
			if strings.TrimSpace(pluginMarketStringFromAny(value)) == "" && !marketPaymentSchemaIsNumber(value) && value != false {
				return nil, true, fmt.Errorf("%s.value is required", optionPath)
			}
			fingerprints = append(fingerprints, marketPaymentSchemaValueFingerprint(value))
		case string, float64, bool:
			fingerprints = append(fingerprints, marketPaymentSchemaValueFingerprint(typed))
		default:
			return nil, true, fmt.Errorf("%s must be a scalar or object", optionPath)
		}
	}
	return fingerprints, true, nil
}

func marketPaymentSchemaIsNumber(value interface{}) bool {
	switch value.(type) {
	case float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return true
	default:
		return false
	}
}

func marketPaymentSchemaValueFingerprint(value interface{}) string {
	body, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(body)
}

func validateMarketPaymentPackageWebhooks(items []marketPaymentPackageWebhookManifest) error {
	if len(items) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(items))
	for idx := range items {
		fieldPath := fmt.Sprintf("webhooks[%d]", idx)
		key := strings.TrimSpace(items[idx].Key)
		if key == "" {
			return fmt.Errorf("%s.key is required", fieldPath)
		}
		if _, exists := seen[key]; exists {
			return fmt.Errorf("%s.key duplicates %q", fieldPath, key)
		}
		seen[key] = struct{}{}

		if _, err := normalizeMarketPaymentWebhookMethod(items[idx].Method); err != nil {
			return fmt.Errorf("%s.method %s", fieldPath, err.Error())
		}
		authMode, err := normalizeMarketPaymentWebhookAuthMode(items[idx].AuthMode)
		if err != nil {
			return fmt.Errorf("%s.auth_mode %s", fieldPath, err.Error())
		}
		if authMode != "none" && strings.TrimSpace(items[idx].SecretKey) == "" {
			return fmt.Errorf("%s.secret_key is required when auth_mode is %q", fieldPath, authMode)
		}
	}
	return nil
}

func normalizeMarketPaymentWebhookMethod(raw string) (string, error) {
	method := strings.ToUpper(strings.TrimSpace(raw))
	if method == "" {
		return http.MethodPost, nil
	}
	if method == "*" || method == "ANY" {
		return "*", nil
	}
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return method, nil
	default:
		return "", fmt.Errorf("must be one of GET/POST/PUT/PATCH/DELETE/*")
	}
}

func normalizeMarketPaymentWebhookAuthMode(raw string) (string, error) {
	mode := strings.ToLower(strings.TrimSpace(raw))
	if mode == "" {
		return "none", nil
	}
	switch mode {
	case "none", "query", "header", "hmac_sha256":
		return mode, nil
	default:
		return "", fmt.Errorf("must be one of none/query/header/hmac_sha256")
	}
}

func parseMarketPaymentMethodID(params map[string]interface{}) (uint, error) {
	id, ok, err := parsePluginHostOptionalUint(params, "payment_method_id", "paymentMethodID", "target_id", "targetId")
	if err != nil {
		return 0, fmt.Errorf("payment_method_id is invalid")
	}
	if !ok {
		return 0, nil
	}
	return id, nil
}

func loadMarketPaymentMethodTarget(db *gorm.DB, paymentMethodID uint) (*models.PaymentMethod, error) {
	if paymentMethodID == 0 {
		return nil, nil
	}
	if db == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "payment method database is unavailable"}
	}

	var method models.PaymentMethod
	if err := db.First(&method, paymentMethodID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &PluginHostActionError{Status: http.StatusNotFound, Message: "payment method not found"}
		}
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "load payment method failed"}
	}
	return &method, nil
}

func resolveMarketPaymentPackage(
	packagePath string,
	checksum string,
	manifest *marketPaymentPackageManifest,
	requestedEntry string,
	existing *models.PaymentMethod,
) (*marketPaymentPackageResolved, error) {
	entry := pluginHostMarketFirstNonEmpty(strings.TrimSpace(requestedEntry), strings.TrimSpace(manifest.Address), strings.TrimSpace(manifest.Entry))
	resolvedEntry, script, scriptBytes, err := readMarketPaymentPackageEntryScript(packagePath, entry)
	if err != nil {
		return nil, err
	}

	configJSON, err := normalizeMarketPaymentPackageConfigJSON("", manifest, marketPaymentMethodTargetConfig(existing))
	if err != nil {
		return nil, err
	}
	pollInterval, err := normalizeMarketPaymentPackagePollInterval("", manifest, marketPaymentMethodTargetPollInterval(existing))
	if err != nil {
		return nil, err
	}

	name := pluginHostMarketFirstNonEmpty(
		manifest.DisplayName.String(),
		strings.TrimSpace(manifest.Name),
		marketPaymentMethodTargetName(existing),
	)
	if name == "" {
		return nil, fmt.Errorf("manifest.name is required")
	}

	return &marketPaymentPackageResolved{
		Name:         name,
		Description:  pluginHostMarketFirstNonEmpty(manifest.Description.String(), marketPaymentMethodTargetDescription(existing)),
		Version:      pluginHostMarketNormalizeVersion(pluginHostMarketFirstNonEmpty(strings.TrimSpace(manifest.Version), marketPaymentMethodTargetVersion(existing))),
		Entry:        resolvedEntry,
		Icon:         pluginHostMarketFirstNonEmpty(strings.TrimSpace(manifest.Icon), marketPaymentMethodTargetIcon(existing), "CreditCard"),
		Config:       configJSON,
		PollInterval: pollInterval,
		Script:       script,
		ScriptBytes:  scriptBytes,
		Checksum:     checksum,
	}, nil
}

func normalizeMarketPaymentPackageConfigJSON(raw string, manifest *marketPaymentPackageManifest, fallback string) (string, error) {
	merged := marketPaymentPackageSchemaDefaultValues(manifest)
	marketPaymentPackageMergeObjectValues(merged, marketPaymentPackageManifestConfigValues(manifest))
	if strings.TrimSpace(raw) != "" {
		explicitValues, err := parseMarketPaymentPackageJSONObjectString(raw)
		if err != nil {
			return "", err
		}
		marketPaymentPackageMergeObjectValues(merged, explicitValues)
	} else {
		fallbackValues, err := parseMarketPaymentPackageJSONObjectString(fallback)
		if err == nil {
			marketPaymentPackageMergeObjectValues(merged, fallbackValues)
		}
	}
	body, err := json.Marshal(merged)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func normalizeMarketPaymentPackagePollInterval(raw string, manifest *marketPaymentPackageManifest, fallback int) (int, error) {
	if strings.TrimSpace(raw) != "" {
		value, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil {
			return 0, err
		}
		if value <= 0 {
			return 30, nil
		}
		return value, nil
	}
	if manifest != nil && manifest.PollInterval != nil && *manifest.PollInterval > 0 {
		return *manifest.PollInterval, nil
	}
	if fallback > 0 {
		return fallback, nil
	}
	return 30, nil
}

func parseMarketPaymentPackageJSONObjectString(raw string) (map[string]interface{}, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return nil, err
	}
	if parsed == nil {
		parsed = map[string]interface{}{}
	}
	return parsed, nil
}

func marketPaymentPackageManifestConfigValues(manifest *marketPaymentPackageManifest) map[string]interface{} {
	if manifest == nil || manifest.Config == nil {
		return nil
	}
	return manifest.Config
}

func marketPaymentPackageSchemaDefaultValues(manifest *marketPaymentPackageManifest) map[string]interface{} {
	if manifest == nil || manifest.ConfigSchema == nil {
		return map[string]interface{}{}
	}
	fieldsRaw, ok := manifest.ConfigSchema["fields"].([]interface{})
	if !ok || len(fieldsRaw) == 0 {
		return map[string]interface{}{}
	}
	defaults := make(map[string]interface{}, len(fieldsRaw))
	for _, item := range fieldsRaw {
		field, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		key := strings.TrimSpace(pluginMarketStringFromAny(field["key"]))
		if key == "" {
			continue
		}
		if defaultValue, exists := field["default"]; exists {
			defaults[key] = defaultValue
		}
	}
	return defaults
}

func marketPaymentPackageMergeObjectValues(target map[string]interface{}, source map[string]interface{}) {
	if len(source) == 0 {
		return
	}
	for key, value := range source {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		target[trimmedKey] = value
	}
}

func validateMarketPaymentPackageConfigAgainstSchema(config map[string]interface{}, schema map[string]interface{}) error {
	if len(schema) == 0 {
		return nil
	}
	fieldsRaw, ok := schema["fields"].([]interface{})
	if !ok || len(fieldsRaw) == 0 {
		return nil
	}
	for _, item := range fieldsRaw {
		field, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		key := strings.TrimSpace(pluginMarketStringFromAny(field["key"]))
		if key == "" {
			continue
		}
		value, exists := config[key]
		required, _ := field["required"].(bool)
		if !exists || value == nil {
			if required {
				return fmt.Errorf("config field %q is required", key)
			}
			continue
		}
		if required {
			if text, ok := value.(string); ok && strings.TrimSpace(text) == "" {
				return fmt.Errorf("config field %q is required", key)
			}
		}
		fieldType, err := normalizeMarketPaymentSchemaFieldType(field["type"])
		if err != nil {
			return err
		}
		if !marketPaymentPackageConfigValueMatchesType(value, fieldType) {
			return fmt.Errorf("config field %q type is invalid", key)
		}
		if fieldType == "select" {
			allowedFingerprints, hasOptions, err := validateMarketPaymentSchemaFieldOptions(field["options"], "config_schema")
			if err != nil {
				return err
			}
			if hasOptions && len(allowedFingerprints) > 0 {
				currentFingerprint := marketPaymentSchemaValueFingerprint(value)
				matched := false
				for _, fingerprint := range allowedFingerprints {
					if fingerprint == currentFingerprint {
						matched = true
						break
					}
				}
				if !matched {
					return fmt.Errorf("config field %q must match one of the declared select options", key)
				}
			}
		}
	}
	return nil
}

func marketPaymentPackageConfigValueMatchesType(value interface{}, fieldType string) bool {
	switch fieldType {
	case "string", "textarea", "secret", "select":
		_, ok := value.(string)
		return ok
	case "number":
		return marketPaymentSchemaIsNumber(value)
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "json":
		return true
	default:
		return true
	}
}

func readMarketPaymentPackageEntryScript(packagePath string, requestedEntry string) (string, string, int, error) {
	extractDir, err := os.MkdirTemp("", "auralogic-market-payment-package-*")
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to create temp extract dir: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(extractDir)
	}()

	if err := pluginHostMarketUnzipPackageSafe(packagePath, extractDir); err != nil {
		return "", "", 0, err
	}
	entryPath, entryPublic, err := resolveMarketPaymentPackageEntryPath(extractDir, requestedEntry)
	if err != nil {
		return "", "", 0, err
	}
	body, err := os.ReadFile(entryPath)
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to read entry script: %w", err)
	}
	if !utf8.Valid(body) {
		return "", "", 0, fmt.Errorf("entry script must be UTF-8 encoded")
	}
	return entryPublic, string(body), len(body), nil
}

func resolveMarketPaymentPackageEntryPath(extractDir string, requestedEntry string) (string, string, error) {
	if strings.TrimSpace(requestedEntry) != "" {
		normalized := normalizeMarketPaymentPackageEntry(requestedEntry)
		targetPath := filepath.Clean(filepath.Join(extractDir, normalized))
		if !pluginHostMarketIsPathWithinRoot(filepath.Clean(extractDir), targetPath) {
			return "", "", fmt.Errorf("entry script path is invalid")
		}
		if info, err := os.Stat(targetPath); err != nil || info.IsDir() {
			return "", "", fmt.Errorf("entry script %q not found in package", filepath.ToSlash(normalized))
		}
		return targetPath, filepath.ToSlash(normalized), nil
	}

	if defaultEntry, defaultPublic := pluginHostMarketFindDefaultJSWorkerEntryFile(extractDir); defaultEntry != "" {
		return defaultEntry, defaultPublic, nil
	}

	firstJS, err := pluginHostMarketFindFirstJSFile(extractDir)
	if err != nil {
		return "", "", fmt.Errorf("failed to detect entry script: %w", err)
	}
	rel, err := filepath.Rel(extractDir, firstJS)
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve entry script: %w", err)
	}
	return firstJS, filepath.ToSlash(rel), nil
}

func normalizeMarketPaymentPackageEntry(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimLeft(trimmed, "/\\")
	trimmed = strings.ReplaceAll(trimmed, "\\", "/")
	return filepath.Clean(filepath.FromSlash(trimmed))
}

func buildMarketPaymentMethodTargetState(
	db *gorm.DB,
	explicitTarget *models.PaymentMethod,
	packageVersion string,
	resolvedName string,
) map[string]interface{} {
	state := map[string]interface{}{
		"installed":           false,
		"current_version":     "",
		"update_available":    false,
		"installed_target":    "payment_method",
		"installed_target_id": nil,
	}

	target := explicitTarget
	if target == nil && db != nil {
		trimmedName := strings.TrimSpace(resolvedName)
		if trimmedName != "" {
			var byName models.PaymentMethod
			if err := db.Where("name = ?", trimmedName).First(&byName).Error; err == nil {
				target = &byName
			}
		}
	}
	if target == nil || target.ID == 0 {
		return state
	}

	state["installed"] = true
	state["current_version"] = strings.TrimSpace(target.Version)
	state["installed_target_id"] = target.ID
	state["update_available"] = strings.TrimSpace(target.Version) != "" && strings.TrimSpace(target.Version) != strings.TrimSpace(packageVersion)
	return state
}

func ensureMarketPaymentMethodNameAvailable(db *gorm.DB, name string, excludeID uint) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return &PluginHostActionError{Status: http.StatusBadRequest, Message: "payment method name is required"}
	}

	var existing models.PaymentMethod
	query := db.Select("id", "name").Where("name = ?", trimmed)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	if err := query.First(&existing).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return &PluginHostActionError{Status: http.StatusInternalServerError, Message: "validate payment method name failed"}
	}
	return &PluginHostActionError{Status: http.StatusConflict, Message: "payment method name conflicts with existing record"}
}

func createMarketPaymentMethod(db *gorm.DB, method *models.PaymentMethod) error {
	if method == nil {
		return fmt.Errorf("payment method is required")
	}
	var maxSort int
	if err := db.Model(&models.PaymentMethod{}).Select("COALESCE(MAX(sort_order), 0)").Scan(&maxSort).Error; err != nil {
		return err
	}
	method.SortOrder = maxSort + 1
	return db.Create(method).Error
}

func marketPaymentMethodTargetID(method *models.PaymentMethod) uint {
	if method == nil {
		return 0
	}
	return method.ID
}

func marketPaymentMethodTargetName(method *models.PaymentMethod) string {
	if method == nil {
		return ""
	}
	return strings.TrimSpace(method.Name)
}

func marketPaymentMethodTargetDescription(method *models.PaymentMethod) string {
	if method == nil {
		return ""
	}
	return strings.TrimSpace(method.Description)
}

func marketPaymentMethodTargetVersion(method *models.PaymentMethod) string {
	if method == nil {
		return ""
	}
	return strings.TrimSpace(method.Version)
}

func marketPaymentMethodTargetIcon(method *models.PaymentMethod) string {
	if method == nil {
		return ""
	}
	return strings.TrimSpace(method.Icon)
}

func marketPaymentMethodTargetConfig(method *models.PaymentMethod) string {
	if method == nil {
		return ""
	}
	return strings.TrimSpace(method.Config)
}

func marketPaymentMethodTargetPollInterval(method *models.PaymentMethod) int {
	if method == nil {
		return 0
	}
	return method.PollInterval
}
