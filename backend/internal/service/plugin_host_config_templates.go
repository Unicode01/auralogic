package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"auralogic/internal/config"
)

const (
	pluginHostInvoiceTemplateTargetKey      = "invoice"
	pluginHostAuthBrandingTemplateTargetKey = "auth_branding"
)

func executePluginHostInvoiceTemplateGet(runtime *PluginHostRuntime, params map[string]interface{}) (map[string]interface{}, error) {
	cfg, filePath, updatedAt, err := loadPluginHostTemplateConfigState(runtime)
	if err != nil {
		return nil, err
	}
	return buildPluginHostInvoiceTemplateResponse(cfg, filePath, updatedAt), nil
}

func executePluginHostInvoiceTemplateSave(runtime *PluginHostRuntime, params map[string]interface{}) (map[string]interface{}, error) {
	content := parsePluginHostOptionalString(params, "content", "html_content", "htmlContent", "custom_template", "customTemplate")
	if strings.TrimSpace(content) == "" {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "content/html_content/htmlContent/custom_template/customTemplate is required"}
	}
	if _, err := template.New("invoice-template-validate").Parse(content); err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: fmt.Sprintf("invalid invoice template syntax: %v", err)}
	}

	cfg, filePath, updatedAt, originalBytes, doc, err := loadPluginHostConfigDocument(runtime)
	if err != nil {
		return nil, err
	}
	if conflictErr := pluginHostValidateOptimisticTextWrite(
		[]byte(cfg.Order.Invoice.CustomTemplate),
		filePath,
		params,
		"invoice template",
		updatedAt,
	); conflictErr != nil {
		return nil, conflictErr
	}

	orderMap := ensurePluginHostJSONObject(doc, "order")
	invoiceMap := ensurePluginHostJSONObject(orderMap, "invoice")
	invoiceMap["template_type"] = "custom"
	invoiceMap["custom_template"] = content

	if err := commitPluginHostConfigDocument(runtime, filePath, originalBytes, doc); err != nil {
		return nil, err
	}

	refreshedCfg, refreshedPath, refreshedUpdatedAt, err := loadPluginHostTemplateConfigState(runtime)
	if err != nil {
		return nil, err
	}
	result := buildPluginHostInvoiceTemplateResponse(refreshedCfg, refreshedPath, refreshedUpdatedAt)
	result["saved"] = true
	return result, nil
}

func executePluginHostInvoiceTemplateReset(runtime *PluginHostRuntime, params map[string]interface{}) (map[string]interface{}, error) {
	cfg, filePath, updatedAt, originalBytes, doc, err := loadPluginHostConfigDocument(runtime)
	if err != nil {
		return nil, err
	}
	if conflictErr := pluginHostValidateOptimisticTextWrite(
		[]byte(cfg.Order.Invoice.CustomTemplate),
		filePath,
		params,
		"invoice template",
		updatedAt,
	); conflictErr != nil {
		return nil, conflictErr
	}

	orderMap := ensurePluginHostJSONObject(doc, "order")
	invoiceMap := ensurePluginHostJSONObject(orderMap, "invoice")
	invoiceMap["template_type"] = "builtin"
	invoiceMap["custom_template"] = ""

	if err := commitPluginHostConfigDocument(runtime, filePath, originalBytes, doc); err != nil {
		return nil, err
	}

	refreshedCfg, refreshedPath, refreshedUpdatedAt, err := loadPluginHostTemplateConfigState(runtime)
	if err != nil {
		return nil, err
	}
	result := buildPluginHostInvoiceTemplateResponse(refreshedCfg, refreshedPath, refreshedUpdatedAt)
	result["saved"] = true
	result["reset"] = true
	return result, nil
}

func executePluginHostAuthBrandingGet(runtime *PluginHostRuntime, params map[string]interface{}) (map[string]interface{}, error) {
	cfg, filePath, updatedAt, err := loadPluginHostTemplateConfigState(runtime)
	if err != nil {
		return nil, err
	}
	return buildPluginHostAuthBrandingResponse(cfg, filePath, updatedAt), nil
}

func executePluginHostAuthBrandingSave(runtime *PluginHostRuntime, params map[string]interface{}) (map[string]interface{}, error) {
	content := parsePluginHostOptionalString(params, "content", "html_content", "htmlContent", "custom_html", "customHtml")
	if strings.TrimSpace(content) == "" {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "content/html_content/htmlContent/custom_html/customHtml is required"}
	}
	if _, err := template.New("auth-branding-validate").Parse(content); err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: fmt.Sprintf("invalid auth branding template syntax: %v", err)}
	}

	cfg, filePath, updatedAt, originalBytes, doc, err := loadPluginHostConfigDocument(runtime)
	if err != nil {
		return nil, err
	}
	if conflictErr := pluginHostValidateOptimisticTextWrite(
		[]byte(cfg.Customization.AuthBranding.CustomHTML),
		filePath,
		params,
		"auth branding template",
		updatedAt,
	); conflictErr != nil {
		return nil, conflictErr
	}

	customizationMap := ensurePluginHostJSONObject(doc, "customization")
	authBrandingMap := ensurePluginHostJSONObject(customizationMap, "auth_branding")
	authBrandingMap["mode"] = "custom"
	authBrandingMap["custom_html"] = content

	if err := commitPluginHostConfigDocument(runtime, filePath, originalBytes, doc); err != nil {
		return nil, err
	}

	refreshedCfg, refreshedPath, refreshedUpdatedAt, err := loadPluginHostTemplateConfigState(runtime)
	if err != nil {
		return nil, err
	}
	result := buildPluginHostAuthBrandingResponse(refreshedCfg, refreshedPath, refreshedUpdatedAt)
	result["saved"] = true
	return result, nil
}

func executePluginHostAuthBrandingReset(runtime *PluginHostRuntime, params map[string]interface{}) (map[string]interface{}, error) {
	cfg, filePath, updatedAt, originalBytes, doc, err := loadPluginHostConfigDocument(runtime)
	if err != nil {
		return nil, err
	}
	if conflictErr := pluginHostValidateOptimisticTextWrite(
		[]byte(cfg.Customization.AuthBranding.CustomHTML),
		filePath,
		params,
		"auth branding template",
		updatedAt,
	); conflictErr != nil {
		return nil, conflictErr
	}

	customizationMap := ensurePluginHostJSONObject(doc, "customization")
	authBrandingMap := ensurePluginHostJSONObject(customizationMap, "auth_branding")
	authBrandingMap["mode"] = "default"
	authBrandingMap["custom_html"] = ""

	if err := commitPluginHostConfigDocument(runtime, filePath, originalBytes, doc); err != nil {
		return nil, err
	}

	refreshedCfg, refreshedPath, refreshedUpdatedAt, err := loadPluginHostTemplateConfigState(runtime)
	if err != nil {
		return nil, err
	}
	result := buildPluginHostAuthBrandingResponse(refreshedCfg, refreshedPath, refreshedUpdatedAt)
	result["saved"] = true
	result["reset"] = true
	return result, nil
}

func loadPluginHostTemplateConfigState(runtime *PluginHostRuntime) (*config.Config, string, time.Time, error) {
	cfg := resolvePluginHostRuntimeConfig(runtime)
	if cfg == nil {
		return nil, "", time.Time{}, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "config runtime is unavailable"}
	}

	configPath := strings.TrimSpace(config.GetConfigPath())
	if configPath == "" {
		return nil, "", time.Time{}, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "config file path is unavailable"}
	}
	absPath, err := filepath.Abs(configPath)
	if err != nil {
		return nil, "", time.Time{}, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "resolve config file path failed"}
	}

	info, statErr := os.Stat(absPath)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return nil, "", time.Time{}, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "config file is unavailable"}
		}
		return nil, "", time.Time{}, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "stat config file failed"}
	}
	return cfg, absPath, info.ModTime().UTC(), nil
}

func loadPluginHostConfigDocument(runtime *PluginHostRuntime) (*config.Config, string, time.Time, []byte, map[string]interface{}, error) {
	cfg, filePath, updatedAt, err := loadPluginHostTemplateConfigState(runtime)
	if err != nil {
		return nil, "", time.Time{}, nil, nil, err
	}

	originalBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, "", time.Time{}, nil, nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "read config file failed"}
	}

	doc := map[string]interface{}{}
	if len(originalBytes) > 0 {
		if err := json.Unmarshal(originalBytes, &doc); err != nil {
			return nil, "", time.Time{}, nil, nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "parse config file failed"}
		}
	}
	return cfg, filePath, updatedAt, originalBytes, doc, nil
}

func commitPluginHostConfigDocument(runtime *PluginHostRuntime, filePath string, originalBytes []byte, doc map[string]interface{}) error {
	newBytes, err := json.MarshalIndent(doc, "", "    ")
	if err != nil {
		return &PluginHostActionError{Status: http.StatusInternalServerError, Message: "marshal config file failed"}
	}

	var parsed config.Config
	if err := json.Unmarshal(newBytes, &parsed); err != nil {
		return &PluginHostActionError{Status: http.StatusInternalServerError, Message: "parse updated config file failed"}
	}

	if err := os.WriteFile(filePath, newBytes, 0o644); err != nil {
		return &PluginHostActionError{Status: http.StatusInternalServerError, Message: "write config file failed"}
	}

	globalCfg := config.GetConfig()
	shouldReloadGlobal := globalCfg != nil && (runtime == nil || runtime.Config == nil || runtime.Config == globalCfg)
	if shouldReloadGlobal {
		if err := config.ReloadConfig(); err != nil {
			_ = os.WriteFile(filePath, originalBytes, 0o644)
			_ = config.ReloadConfig()
			return &PluginHostActionError{Status: http.StatusInternalServerError, Message: fmt.Sprintf("reload config failed: %v", err)}
		}
	}

	if runtime != nil && runtime.Config != nil && (!shouldReloadGlobal || runtime.Config != globalCfg) {
		*runtime.Config = parsed
	}
	return nil
}

func buildPluginHostInvoiceTemplateResponse(cfg *config.Config, filePath string, updatedAt time.Time) map[string]interface{} {
	content := ""
	templateType := "builtin"
	enabled := false
	if cfg != nil {
		content = cfg.Order.Invoice.CustomTemplate
		templateType = strings.TrimSpace(cfg.Order.Invoice.TemplateType)
		enabled = cfg.Order.Invoice.Enabled
	}
	if templateType == "" {
		templateType = "builtin"
	}
	return map[string]interface{}{
		"target_key":      pluginHostInvoiceTemplateTargetKey,
		"file_path":       filePath,
		"template_type":   templateType,
		"enabled":         enabled,
		"content":         content,
		"html_content":    content,
		"custom_template": content,
		"digest":          pluginHostDigestString(content),
		"updated_at":      pluginHostOptionalTime(updatedAt),
		"size":            len(content),
		"exists":          strings.TrimSpace(content) != "",
	}
}

func buildPluginHostAuthBrandingResponse(cfg *config.Config, filePath string, updatedAt time.Time) map[string]interface{} {
	mode := "default"
	title := ""
	titleEn := ""
	subtitle := ""
	subtitleEn := ""
	content := ""
	if cfg != nil {
		mode = strings.TrimSpace(cfg.Customization.AuthBranding.Mode)
		title = cfg.Customization.AuthBranding.Title
		titleEn = cfg.Customization.AuthBranding.TitleEn
		subtitle = cfg.Customization.AuthBranding.Subtitle
		subtitleEn = cfg.Customization.AuthBranding.SubtitleEn
		content = cfg.Customization.AuthBranding.CustomHTML
	}
	if mode == "" {
		mode = "default"
	}
	return map[string]interface{}{
		"target_key":   pluginHostAuthBrandingTemplateTargetKey,
		"file_path":    filePath,
		"mode":         mode,
		"title":        title,
		"title_en":     titleEn,
		"subtitle":     subtitle,
		"subtitle_en":  subtitleEn,
		"content":      content,
		"html_content": content,
		"custom_html":  content,
		"digest":       pluginHostDigestString(content),
		"updated_at":   pluginHostOptionalTime(updatedAt),
		"size":         len(content),
		"exists":       strings.TrimSpace(content) != "",
	}
}

func resolvePluginHostRuntimeConfig(runtime *PluginHostRuntime) *config.Config {
	if runtime != nil && runtime.Config != nil {
		return runtime.Config
	}
	if snapshot := loadPluginHostConfigSnapshot(); snapshot != nil {
		return snapshot
	}
	return config.GetConfig()
}

func loadPluginHostConfigSnapshot() *config.Config {
	configPath := strings.TrimSpace(config.GetConfigPath())
	if configPath == "" {
		return nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return &config.Config{}
	}

	var snapshot config.Config
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil
	}
	return &snapshot
}

func ensurePluginHostJSONObject(root map[string]interface{}, key string) map[string]interface{} {
	if root == nil {
		return map[string]interface{}{}
	}
	if existing, ok := root[key].(map[string]interface{}); ok && existing != nil {
		return existing
	}
	next := map[string]interface{}{}
	root[key] = next
	return next
}

func pluginHostOptionalTime(value time.Time) interface{} {
	if value.IsZero() {
		return nil
	}
	return value.UTC()
}
