package service

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"auralogic/internal/database"
	"auralogic/internal/models"
	"gorm.io/gorm"
)

func executePluginHostEmailTemplateList(params map[string]interface{}) (map[string]interface{}, error) {
	templateDir, err := pluginHostEmailTemplateDir()
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: err.Error()}
	}

	entries, err := os.ReadDir(templateDir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]interface{}{"items": []map[string]interface{}{}}, nil
		}
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "read email template directory failed"}
	}

	eventFilter := strings.ToLower(strings.TrimSpace(parsePluginHostOptionalString(params, "event")))
	items := make([]map[string]interface{}, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".html") {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			continue
		}
		key, event, locale := pluginHostParseEmailTemplateFilename(entry.Name())
		if eventFilter != "" && !strings.EqualFold(event, eventFilter) {
			continue
		}
		fullPath := filepath.Join(templateDir, entry.Name())
		content, readErr := os.ReadFile(fullPath)
		if readErr != nil {
			continue
		}
		items = append(items, map[string]interface{}{
			"key":        key,
			"event":      event,
			"locale":     locale,
			"filename":   entry.Name(),
			"digest":     pluginHostDigestBytes(content),
			"updated_at": info.ModTime().UTC(),
			"size":       len(content),
		})
	}

	sortPluginHostTemplateItems(items, "filename")
	return map[string]interface{}{"items": items}, nil
}

func executePluginHostEmailTemplateGet(params map[string]interface{}) (map[string]interface{}, error) {
	filename, err := resolvePluginHostEmailTemplateFilename(params)
	if err != nil {
		return nil, err
	}
	templateDir, err := pluginHostEmailTemplateDir()
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: err.Error()}
	}

	fullPath := filepath.Join(templateDir, filename)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &PluginHostActionError{Status: http.StatusNotFound, Message: "email template not found"}
		}
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "read email template failed"}
	}
	info, statErr := os.Stat(fullPath)
	if statErr != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "stat email template failed"}
	}

	key, event, locale := pluginHostParseEmailTemplateFilename(filename)
	return map[string]interface{}{
		"key":        key,
		"event":      event,
		"locale":     locale,
		"filename":   filename,
		"content":    string(content),
		"digest":     pluginHostDigestBytes(content),
		"updated_at": info.ModTime().UTC(),
		"size":       len(content),
	}, nil
}

func executePluginHostEmailTemplateSave(params map[string]interface{}) (map[string]interface{}, error) {
	filename, err := resolvePluginHostEmailTemplateFilename(params)
	if err != nil {
		return nil, err
	}
	content := parsePluginHostOptionalString(params, "content", "html_content", "htmlContent")
	if strings.TrimSpace(content) == "" {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "content/html_content/htmlContent is required"}
	}
	if _, err := template.New("email-template-validate").Parse(content); err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: fmt.Sprintf("invalid email template syntax: %v", err)}
	}

	templateDir, err := pluginHostEmailTemplateDir()
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: err.Error()}
	}
	fullPath := filepath.Join(templateDir, filename)
	currentContent, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &PluginHostActionError{Status: http.StatusNotFound, Message: "email template not found"}
		}
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "read email template failed"}
	}
	if conflictErr := pluginHostValidateOptimisticTextWrite(currentContent, fullPath, params, "email template"); conflictErr != nil {
		return nil, conflictErr
	}

	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "save email template failed"}
	}
	info, statErr := os.Stat(fullPath)
	if statErr != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "stat email template failed"}
	}
	key, event, locale := pluginHostParseEmailTemplateFilename(filename)
	return map[string]interface{}{
		"key":        key,
		"event":      event,
		"locale":     locale,
		"filename":   filename,
		"content":    content,
		"digest":     pluginHostDigestString(content),
		"updated_at": info.ModTime().UTC(),
		"saved":      true,
	}, nil
}

func executePluginHostLandingPageGet(db *gorm.DB, params map[string]interface{}) (map[string]interface{}, error) {
	if db == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "landing page database is unavailable"}
	}

	slug := strings.TrimSpace(parsePluginHostOptionalString(params, "slug", "page_key", "pageKey"))
	if slug == "" {
		slug = "home"
	}

	var page models.LandingPage
	if err := db.Where("slug = ?", slug).First(&page).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return map[string]interface{}{
				"page_key":     slug,
				"slug":         slug,
				"html_content": "",
				"digest":       pluginHostDigestString(""),
				"updated_at":   nil,
				"exists":       false,
			}, nil
		}
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query landing page failed"}
	}
	return buildPluginHostLandingPageResponse(&page), nil
}

func executePluginHostLandingPageSave(db *gorm.DB, params map[string]interface{}) (map[string]interface{}, error) {
	if db == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "landing page database is unavailable"}
	}

	slug := strings.TrimSpace(parsePluginHostOptionalString(params, "slug", "page_key", "pageKey"))
	if slug == "" {
		slug = "home"
	}
	htmlContent := parsePluginHostOptionalString(params, "html_content", "htmlContent", "content")
	if strings.TrimSpace(htmlContent) == "" {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "html_content/htmlContent/content is required"}
	}
	if _, err := template.New("landing-page-validate").Parse(htmlContent); err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: fmt.Sprintf("invalid landing page template syntax: %v", err)}
	}

	var page models.LandingPage
	err := db.Where("slug = ?", slug).First(&page).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query landing page failed"}
	}
	if err == nil {
		if conflictErr := pluginHostValidateOptimisticTextWrite([]byte(page.HTMLContent), "", params, "landing page", page.UpdatedAt); conflictErr != nil {
			return nil, conflictErr
		}
		page.HTMLContent = htmlContent
		if saveErr := db.Save(&page).Error; saveErr != nil {
			return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "save landing page failed"}
		}
		result := buildPluginHostLandingPageResponse(&page)
		result["saved"] = true
		return result, nil
	}

	page = models.LandingPage{
		Slug:        slug,
		HTMLContent: htmlContent,
		IsActive:    true,
	}
	if createErr := db.Create(&page).Error; createErr != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "create landing page failed"}
	}
	result := buildPluginHostLandingPageResponse(&page)
	result["saved"] = true
	return result, nil
}

func executePluginHostLandingPageReset(db *gorm.DB, params map[string]interface{}) (map[string]interface{}, error) {
	defaultHTML := database.GetDefaultLandingPageHTML()
	if strings.TrimSpace(defaultHTML) == "" {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "default landing page template is unavailable"}
	}

	resetParams := map[string]interface{}{}
	for key, value := range params {
		resetParams[key] = value
	}
	resetParams["html_content"] = defaultHTML
	result, err := executePluginHostLandingPageSave(db, resetParams)
	if err != nil {
		return nil, err
	}
	result["reset"] = true
	return result, nil
}

func pluginHostEmailTemplateDir() (string, error) {
	return filepath.Abs(filepath.Join("templates", "email"))
}

func resolvePluginHostEmailTemplateFilename(params map[string]interface{}) (string, error) {
	filename := strings.TrimSpace(parsePluginHostOptionalString(params, "filename", "key", "name"))
	if filename == "" {
		return "", &PluginHostActionError{Status: http.StatusBadRequest, Message: "filename/key/name is required"}
	}
	if !strings.HasSuffix(strings.ToLower(filename), ".html") {
		filename += ".html"
	}
	if strings.Contains(filename, "/") || strings.Contains(filename, "\\") || strings.Contains(filename, "..") {
		return "", &PluginHostActionError{Status: http.StatusBadRequest, Message: "invalid email template filename"}
	}
	return filename, nil
}

func pluginHostParseEmailTemplateFilename(filename string) (string, string, string) {
	name := strings.TrimSuffix(strings.TrimSpace(filename), filepath.Ext(filename))
	event := name
	locale := ""
	if idx := strings.LastIndex(name, "_"); idx > 0 {
		possibleLocale := strings.ToLower(strings.TrimSpace(name[idx+1:]))
		switch possibleLocale {
		case "zh", "en":
			event = name[:idx]
			locale = possibleLocale
		}
	}
	return name, event, locale
}

func pluginHostValidateOptimisticTextWrite(
	currentContent []byte,
	fullPath string,
	params map[string]interface{},
	resourceLabel string,
	updatedAtCandidates ...time.Time,
) error {
	expectedDigest := strings.TrimSpace(parsePluginHostOptionalString(params, "expected_digest", "expectedDigest"))
	if expectedDigest != "" {
		currentDigest := pluginHostDigestBytes(currentContent)
		if !strings.EqualFold(expectedDigest, currentDigest) {
			return &PluginHostActionError{Status: http.StatusConflict, Message: fmt.Sprintf("%s has changed; digest mismatch", resourceLabel)}
		}
	}

	expectedUpdatedAt := strings.TrimSpace(parsePluginHostOptionalString(params, "expected_updated_at", "expectedUpdatedAt"))
	if expectedUpdatedAt == "" {
		return nil
	}
	parsedExpectedAt, err := time.Parse(time.RFC3339, expectedUpdatedAt)
	if err != nil {
		return &PluginHostActionError{Status: http.StatusBadRequest, Message: "expected_updated_at/expectedUpdatedAt must be RFC3339 timestamp"}
	}

	var currentUpdatedAt time.Time
	switch {
	case len(updatedAtCandidates) > 0 && !updatedAtCandidates[0].IsZero():
		currentUpdatedAt = updatedAtCandidates[0].UTC()
	case strings.TrimSpace(fullPath) != "":
		info, statErr := os.Stat(fullPath)
		if statErr == nil {
			currentUpdatedAt = info.ModTime().UTC()
		}
	}
	if !currentUpdatedAt.IsZero() && currentUpdatedAt.After(parsedExpectedAt.UTC()) {
		return &PluginHostActionError{Status: http.StatusConflict, Message: fmt.Sprintf("%s has changed; updated_at mismatch", resourceLabel)}
	}
	return nil
}

func pluginHostDigestBytes(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func pluginHostDigestString(content string) string {
	return pluginHostDigestBytes([]byte(content))
}

func buildPluginHostLandingPageResponse(page *models.LandingPage) map[string]interface{} {
	if page == nil {
		return map[string]interface{}{
			"page_key":     "home",
			"slug":         "home",
			"html_content": "",
			"digest":       pluginHostDigestString(""),
			"updated_at":   nil,
			"exists":       false,
		}
	}
	return map[string]interface{}{
		"id":           page.ID,
		"page_key":     page.Slug,
		"slug":         page.Slug,
		"html_content": page.HTMLContent,
		"is_active":    page.IsActive,
		"updated_by":   page.UpdatedBy,
		"digest":       pluginHostDigestString(page.HTMLContent),
		"updated_at":   page.UpdatedAt,
		"created_at":   page.CreatedAt,
		"exists":       true,
	}
}

func sortPluginHostTemplateItems(items []map[string]interface{}, key string) {
	if len(items) <= 1 {
		return
	}
	sort.SliceStable(items, func(i int, j int) bool {
		return strings.ToLower(pluginMarketStringFromAny(items[i][key])) < strings.ToLower(pluginMarketStringFromAny(items[j][key]))
	})
}
