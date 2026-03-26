package service

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"
)

const TemplatePackageImportMaxBytes = int64(8 * 1024 * 1024)

const (
	templatePackageMaxEntries       = 256
	templatePackageManifestMaxBytes = int64(512 * 1024)
	templatePackageContentMaxBytes  = int64(2 * 1024 * 1024)
	templatePackageFallbackSourceID = "admin_upload"
	templatePackageManifestFileName = "manifest.json"
)

type TemplatePackageImportOptions struct {
	ExpectedKind string
	TargetKey    string
	ImportedBy   *uint
	SourceID     string
}

type templatePackageArchive struct {
	ManifestPath string
	ContentPath  string
	Manifest     map[string]interface{}
	Content      string
	EntryCount   int
}

// ImportTemplatePackage imports a host-managed template package zip directly into
// native host-managed template storage and records a rollback snapshot.
func ImportTemplatePackage(
	db *gorm.DB,
	emailService *EmailService,
	packageFilename string,
	packageBody []byte,
	options TemplatePackageImportOptions,
) (map[string]interface{}, error) {
	if db == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "template database is unavailable"}
	}
	if len(packageBody) == 0 {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "template package is empty"}
	}
	if int64(len(packageBody)) > TemplatePackageImportMaxBytes {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: fmt.Sprintf("template package exceeds %d bytes", TemplatePackageImportMaxBytes)}
	}

	archive, err := readTemplatePackageArchive(packageBody)
	if err != nil {
		return nil, err
	}

	trimmedExpectedKind := normalizePluginMarketArtifactKind(options.ExpectedKind)
	manifestKind := normalizePluginMarketArtifactKind(pluginHostMarketFirstNonEmpty(
		pluginMarketStringFromAny(archive.Manifest["kind"]),
		inferTemplatePackageKind(archive.Manifest),
	))
	if trimmedExpectedKind != "" && manifestKind != "" && trimmedExpectedKind != manifestKind {
		return nil, &PluginHostActionError{
			Status:  http.StatusBadRequest,
			Message: fmt.Sprintf("template package kind %q does not match expected kind %q", manifestKind, trimmedExpectedKind),
		}
	}

	selectedKind := pluginHostMarketFirstNonEmpty(trimmedExpectedKind, manifestKind)
	if selectedKind != "email_template" &&
		selectedKind != "landing_page_template" &&
		selectedKind != "invoice_template" &&
		selectedKind != "auth_branding_template" &&
		selectedKind != "page_rule_pack" {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "template package kind is invalid"}
	}
	runtime := NewPluginHostRuntime(db, resolvePluginHostRuntimeConfig(nil), nil)

	archiveBaseName := strings.TrimSuffix(filepath.Base(strings.TrimSpace(packageFilename)), filepath.Ext(strings.TrimSpace(packageFilename)))
	if archiveBaseName == "" {
		archiveBaseName = "template-package"
	}
	name := strings.TrimSpace(pluginHostMarketFirstNonEmpty(
		pluginMarketStringFromAny(archive.Manifest["name"]),
		archiveBaseName,
	))
	version := pluginHostMarketNormalizeVersion(pluginHostMarketFirstNonEmpty(
		pluginMarketStringFromAny(archive.Manifest["version"]),
		time.Now().UTC().Format("20060102T150405Z"),
	))

	release := buildTemplatePackagePseudoRelease(selectedKind, name, archive)
	params := map[string]interface{}{}
	if trimmedTargetKey := strings.TrimSpace(options.TargetKey); trimmedTargetKey != "" {
		params["target_key"] = trimmedTargetKey
	}

	targetKey, err := resolveTemplateMarketTargetKey(selectedKind, params, release, name)
	if err != nil {
		return nil, err
	}
	content, err := resolveTemplateMarketContent(selectedKind, release)
	if err != nil {
		return nil, err
	}

	saved, err := saveTemplateMarketTarget(runtime, selectedKind, targetKey, content)
	if err != nil {
		return nil, err
	}

	hotReloaded := false
	hotReloadWarning := ""
	if selectedKind == "email_template" && emailService != nil {
		if reloadErr := emailService.ReloadTemplates(); reloadErr != nil {
			hotReloadWarning = reloadErr.Error()
		} else {
			hotReloaded = true
		}
	}

	coordinates := newPluginHostMarketCoordinates(
		pluginHostMarketFirstNonEmpty(strings.TrimSpace(options.SourceID), templatePackageFallbackSourceID),
		selectedKind,
		name,
		version,
	)
	record, err := createTemplateVersionSnapshot(db, selectedKind, targetKey, content, coordinates, options.ImportedBy)
	if err != nil {
		return nil, err
	}
	currentState, err := loadTemplateMarketCurrentState(runtime, selectedKind, targetKey)
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"status": "imported",
		"message": func() string {
			if hotReloadWarning != "" {
				return "Template package imported; runtime reload will be applied on next email render"
			}
			return "Template package imported successfully"
		}(),
		"kind":    selectedKind,
		"name":    name,
		"version": version,
		"package": map[string]interface{}{
			"filename": packageFilename,
			"bytes":    len(packageBody),
			"digest":   pluginHostDigestBytes(packageBody),
		},
		"manifest":      archive.Manifest,
		"saved":         saved,
		"history_entry": buildTemplateVersionSummary(record),
		"resolved": map[string]interface{}{
			"target_key":      targetKey,
			"content_bytes":   len(content),
			"content_digest":  pluginHostDigestString(content),
			"manifest_path":   archive.ManifestPath,
			"content_path":    archive.ContentPath,
			"archive_entries": archive.EntryCount,
		},
		"target_state":   buildTemplateMarketTargetState(runtime, selectedKind, targetKey, version, currentState),
		"hot_reloaded":   hotReloaded,
		"reload_warning": hotReloadWarning,
	}
	return result, nil
}

func inferTemplatePackageKind(manifest map[string]interface{}) string {
	kind := strings.ToLower(strings.TrimSpace(pluginMarketStringFromAny(manifest["kind"])))
	if kind != "" {
		return kind
	}
	contentFile := pluginHostMarketFirstNonEmpty(
		pluginMarketStringFromAny(manifest["content_file"]),
		pluginMarketStringFromAny(manifest["template_file"]),
		pluginMarketStringFromAny(manifest["rules_file"]),
	)
	if contentFile == "" {
		templatePayload := clonePluginMarketMap(manifest["template"])
		pageRulesPayload := clonePluginMarketMap(manifest["page_rules"])
		if strings.TrimSpace(pluginMarketStringFromAny(templatePayload["content"])) == "" &&
			strings.TrimSpace(pluginMarketStringFromAny(pageRulesPayload["content"])) == "" &&
			templatePayload["rules"] == nil &&
			pageRulesPayload["rules"] == nil {
			return ""
		}
	}
	if strings.TrimSpace(pluginMarketStringFromAny(manifest["event"])) != "" {
		return "email_template"
	}
	resolvedKey := strings.ToLower(strings.TrimSpace(pluginHostMarketFirstNonEmpty(
		pluginMarketStringFromAny(manifest["key"]),
		pluginMarketStringFromAny(manifest["target_key"]),
		pluginMarketStringFromAny(clonePluginMarketMap(manifest["targets"])["key"]),
		pluginMarketStringFromAny(clonePluginMarketMap(manifest["template"])["key"]),
	)))
	switch resolvedKey {
	case pluginHostInvoiceTemplateTargetKey:
		return "invoice_template"
	case pluginHostAuthBrandingTemplateTargetKey:
		return "auth_branding_template"
	case pluginHostPageRulePackTargetKey:
		return "page_rule_pack"
	}
	if strings.TrimSpace(pluginMarketStringFromAny(manifest["slug"])) != "" ||
		strings.TrimSpace(pluginMarketStringFromAny(manifest["page_key"])) != "" ||
		strings.TrimSpace(pluginMarketStringFromAny(manifest["engine"])) != "" {
		return "landing_page_template"
	}
	return ""
}

func buildTemplatePackagePseudoRelease(
	kind string,
	name string,
	archive *templatePackageArchive,
) map[string]interface{} {
	manifest := archive.Manifest
	targets := clonePluginMarketMap(manifest["targets"])
	templatePayload := clonePluginMarketMap(manifest["template"])
	install := clonePluginMarketMap(manifest["install"])
	pageRulesPayload := clonePluginMarketMap(manifest["page_rules"])

	contentFile := pluginHostMarketFirstNonEmpty(
		pluginMarketStringFromAny(manifest["content_file"]),
		pluginMarketStringFromAny(manifest["template_file"]),
		pluginMarketStringFromAny(manifest["rules_file"]),
		filepath.Base(strings.TrimSpace(archive.ContentPath)),
	)
	templatePayload["content"] = archive.Content
	install["content"] = archive.Content
	install["inline_content"] = true

	switch kind {
	case "email_template":
		if templatePayload["key"] == nil {
			templatePayload["key"] = pluginHostMarketFirstNonEmpty(
				pluginMarketStringFromAny(manifest["key"]),
				pluginMarketStringFromAny(manifest["name"]),
				name,
			)
		}
		if templatePayload["filename"] == nil {
			templatePayload["filename"] = pluginHostMarketFirstNonEmpty(contentFile, name+".html")
		}
		if templatePayload["subject"] == nil {
			templatePayload["subject"] = pluginMarketStringFromAny(manifest["subject"])
		}
		if targets["event"] == nil {
			targets["event"] = pluginMarketStringFromAny(manifest["event"])
		}
		if targets["key"] == nil {
			targets["key"] = pluginHostMarketFirstNonEmpty(
				pluginMarketStringFromAny(manifest["key"]),
				pluginMarketStringFromAny(manifest["name"]),
				name,
			)
		}
	case "landing_page_template":
		if templatePayload["page_key"] == nil {
			templatePayload["page_key"] = pluginHostMarketFirstNonEmpty(
				pluginMarketStringFromAny(manifest["page_key"]),
				pluginMarketStringFromAny(manifest["name"]),
				name,
			)
		}
		if templatePayload["slug"] == nil {
			templatePayload["slug"] = pluginHostMarketFirstNonEmpty(
				pluginMarketStringFromAny(manifest["slug"]),
				pluginMarketStringFromAny(manifest["page_key"]),
				pluginMarketStringFromAny(manifest["name"]),
				name,
			)
		}
		if templatePayload["filename"] == nil {
			templatePayload["filename"] = pluginHostMarketFirstNonEmpty(contentFile, name+".html")
		}
		if targets["page_key"] == nil {
			targets["page_key"] = pluginHostMarketFirstNonEmpty(
				pluginMarketStringFromAny(manifest["page_key"]),
				pluginMarketStringFromAny(manifest["name"]),
				name,
			)
		}
		if targets["slug"] == nil {
			targets["slug"] = pluginHostMarketFirstNonEmpty(
				pluginMarketStringFromAny(manifest["slug"]),
				pluginMarketStringFromAny(manifest["page_key"]),
				pluginMarketStringFromAny(manifest["name"]),
				name,
			)
		}
	case "invoice_template":
		if templatePayload["key"] == nil {
			templatePayload["key"] = pluginHostInvoiceTemplateTargetKey
		}
		if templatePayload["filename"] == nil {
			templatePayload["filename"] = pluginHostMarketFirstNonEmpty(contentFile, name+".html")
		}
		if targets["key"] == nil {
			targets["key"] = pluginHostInvoiceTemplateTargetKey
		}
	case "auth_branding_template":
		if templatePayload["key"] == nil {
			templatePayload["key"] = pluginHostAuthBrandingTemplateTargetKey
		}
		if templatePayload["filename"] == nil {
			templatePayload["filename"] = pluginHostMarketFirstNonEmpty(contentFile, name+".html")
		}
		if targets["key"] == nil {
			targets["key"] = pluginHostAuthBrandingTemplateTargetKey
		}
	case "page_rule_pack":
		if templatePayload["key"] == nil {
			templatePayload["key"] = pluginHostPageRulePackTargetKey
		}
		if templatePayload["filename"] == nil {
			templatePayload["filename"] = pluginHostMarketFirstNonEmpty(contentFile, name+".json")
		}
		if targets["key"] == nil {
			targets["key"] = pluginHostPageRulePackTargetKey
		}
		pageRulesPayload["key"] = pluginHostPageRulePackTargetKey
		pageRulesPayload["content"] = archive.Content
		if parsedRules, ok := tryParseTemplatePackagePageRules(archive.Content); ok {
			pageRulesPayload["rules"] = parsedRules
		}
	}

	release := map[string]interface{}{
		"targets":  targets,
		"template": templatePayload,
		"install":  install,
	}
	if kind == "page_rule_pack" {
		release["page_rules"] = pageRulesPayload
	}
	return release
}

func readTemplatePackageArchive(packageBody []byte) (*templatePackageArchive, error) {
	reader, err := zip.NewReader(bytes.NewReader(packageBody), int64(len(packageBody)))
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: fmt.Sprintf("open template package failed: %v", err)}
	}
	if len(reader.File) > templatePackageMaxEntries {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "template package contains too many entries"}
	}

	files := make(map[string]*zip.File, len(reader.File))
	manifestCandidates := make([]string, 0, 2)
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		name, normalizeErr := normalizeTemplatePackageEntryName(file.Name)
		if normalizeErr != nil {
			return nil, normalizeErr
		}
		files[name] = file
		if path.Base(name) == templatePackageManifestFileName {
			manifestCandidates = append(manifestCandidates, name)
		}
	}

	if len(manifestCandidates) == 0 {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "template package manifest.json not found"}
	}
	if len(manifestCandidates) > 1 {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "template package contains multiple manifest.json files"}
	}

	manifestPath := manifestCandidates[0]
	manifestBytes, err := readTemplatePackageFile(files[manifestPath], templatePackageManifestMaxBytes)
	if err != nil {
		return nil, err
	}

	var manifest map[string]interface{}
	if err := json.Unmarshal(bytes.TrimPrefix(manifestBytes, []byte{0xEF, 0xBB, 0xBF}), &manifest); err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: fmt.Sprintf("parse template package manifest failed: %v", err)}
	}

	templatePayload := clonePluginMarketMap(manifest["template"])
	pageRulesPayload := clonePluginMarketMap(manifest["page_rules"])
	content := strings.TrimSpace(pluginMarketStringFromAny(templatePayload["content"]))
	if content == "" {
		content = strings.TrimSpace(pluginMarketStringFromAny(pageRulesPayload["content"]))
	}
	if content == "" {
		if body, ok := tryMarshalTemplatePackageInlineRules(pageRulesPayload["rules"]); ok {
			content = body
		} else if body, ok := tryMarshalTemplatePackageInlineRules(templatePayload["rules"]); ok {
			content = body
		}
	}
	contentPath := ""
	if content == "" {
		declaredContentPath := pluginHostMarketFirstNonEmpty(
			pluginMarketStringFromAny(manifest["content_file"]),
			pluginMarketStringFromAny(manifest["template_file"]),
			pluginMarketStringFromAny(manifest["rules_file"]),
		)
		if declaredContentPath == "" {
			return nil, &PluginHostActionError{
				Status:  http.StatusBadRequest,
				Message: "template package content_file/template_file/rules_file is required",
			}
		}

		manifestDir := path.Dir(manifestPath)
		if manifestDir == "." {
			manifestDir = ""
		}
		resolvedContentPath, err := normalizeTemplatePackageRelativeEntryName(manifestDir, declaredContentPath)
		if err != nil {
			return nil, err
		}

		contentFile, exists := files[resolvedContentPath]
		if !exists {
			return nil, &PluginHostActionError{
				Status:  http.StatusBadRequest,
				Message: fmt.Sprintf("template package content file %s not found", declaredContentPath),
			}
		}
		contentBytes, readErr := readTemplatePackageFile(contentFile, templatePackageContentMaxBytes)
		if readErr != nil {
			return nil, readErr
		}
		content = string(contentBytes)
		contentPath = resolvedContentPath
	}

	return &templatePackageArchive{
		ManifestPath: manifestPath,
		ContentPath:  contentPath,
		Manifest:     manifest,
		Content:      content,
		EntryCount:   len(files),
	}, nil
}

func tryMarshalTemplatePackageInlineRules(value interface{}) (string, bool) {
	if value == nil {
		return "", false
	}
	body, err := json.Marshal(value)
	if err != nil {
		return "", false
	}
	rules, err := parsePluginHostPageRulePackContent(body)
	if err != nil {
		return "", false
	}
	canonical, err := marshalPluginHostPageRules(rules)
	if err != nil {
		return "", false
	}
	return string(canonical), true
}

func tryParseTemplatePackagePageRules(content string) (interface{}, bool) {
	rules, err := parsePluginHostPageRulePackContent([]byte(content))
	if err != nil {
		return nil, false
	}
	return clonePluginHostPageRules(rules), true
}

func normalizeTemplatePackageEntryName(name string) (string, error) {
	normalized := strings.TrimSpace(strings.ReplaceAll(name, "\\", "/"))
	if normalized == "" {
		return "", &PluginHostActionError{Status: http.StatusBadRequest, Message: "template package contains an empty file name"}
	}
	cleaned := path.Clean("/" + normalized)
	if cleaned == "/" {
		return "", &PluginHostActionError{Status: http.StatusBadRequest, Message: "template package contains an invalid file path"}
	}
	return strings.TrimPrefix(cleaned, "/"), nil
}

func normalizeTemplatePackageRelativeEntryName(baseDir string, name string) (string, error) {
	normalized := strings.TrimSpace(strings.ReplaceAll(name, "\\", "/"))
	if normalized == "" {
		return "", &PluginHostActionError{Status: http.StatusBadRequest, Message: "template package content path is empty"}
	}
	baseDir = strings.Trim(strings.TrimSpace(strings.ReplaceAll(baseDir, "\\", "/")), "/")
	joined := normalized
	if baseDir != "" {
		joined = path.Join(baseDir, normalized)
	}
	return normalizeTemplatePackageEntryName(joined)
}

func readTemplatePackageFile(file *zip.File, maxBytes int64) ([]byte, error) {
	if file == nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "template package file is missing"}
	}
	if file.UncompressedSize64 > uint64(maxBytes) {
		return nil, &PluginHostActionError{
			Status:  http.StatusBadRequest,
			Message: fmt.Sprintf("template package file %s exceeds size limit", path.Base(file.Name)),
		}
	}

	reader, err := file.Open()
	if err != nil {
		return nil, &PluginHostActionError{
			Status:  http.StatusBadRequest,
			Message: fmt.Sprintf("open template package file %s failed: %v", path.Base(file.Name), err),
		}
	}
	defer reader.Close()

	data, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return nil, &PluginHostActionError{
			Status:  http.StatusBadRequest,
			Message: fmt.Sprintf("read template package file %s failed: %v", path.Base(file.Name), err),
		}
	}
	if int64(len(data)) > maxBytes {
		return nil, &PluginHostActionError{
			Status:  http.StatusBadRequest,
			Message: fmt.Sprintf("template package file %s exceeds size limit", path.Base(file.Name)),
		}
	}
	return data, nil
}
