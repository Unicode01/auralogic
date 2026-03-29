package admin

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"auralogic/internal/models"
	"auralogic/internal/pkg/utils"
	"auralogic/internal/pluginipc"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const maxPluginWebhookBodyBytes = int64(1024 * 1024)

func (h *PluginHandler) HandlePluginWebhook(c *gin.Context) {
	pluginName := strings.TrimSpace(c.Param("name"))
	webhookKey := strings.TrimSpace(c.Param("hook"))
	if pluginName == "" || webhookKey == "" {
		h.respondPluginError(c, http.StatusBadRequest, "plugin name and webhook key are required")
		return
	}
	if h.pluginManager == nil {
		h.respondPluginError(c, http.StatusInternalServerError, "Plugin manager is unavailable")
		return
	}

	var plugin models.Plugin
	if err := h.db.Where("name = ?", pluginName).First(&plugin).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			h.respondPluginError(c, http.StatusNotFound, "Plugin not found")
			return
		}
		h.respondPluginError(c, http.StatusInternalServerError, "Failed to query plugin")
		return
	}
	if !plugin.Enabled {
		h.respondPluginError(c, http.StatusServiceUnavailable, "Plugin webhook is unavailable")
		return
	}

	webhook, exists, err := findStoredPluginWebhook(plugin.Manifest, webhookKey)
	if err != nil {
		h.respondPluginErrorErr(c, http.StatusInternalServerError, err)
		return
	}
	if !exists {
		h.respondPluginError(c, http.StatusNotFound, "Plugin webhook not found")
		return
	}
	if !pluginWebhookAllowsMethod(webhook, c.Request.Method) {
		h.respondPluginError(c, http.StatusMethodNotAllowed, "Plugin webhook method is not allowed")
		return
	}

	rawBody, err := readPluginWebhookBody(c.Request.Body, maxPluginWebhookBodyBytes)
	if err != nil {
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
		return
	}

	secrets, err := h.pluginManager.LoadPluginSecretSnapshot(plugin.ID)
	if err != nil {
		h.respondPluginErrorErr(c, http.StatusInternalServerError, err)
		return
	}
	if authErr := authenticatePluginWebhookRequest(c, webhook, secrets, rawBody); authErr != nil {
		h.respondPluginErrorErr(c, http.StatusUnauthorized, authErr)
		return
	}

	bodyText := ""
	if utf8.Valid(rawBody) {
		bodyText = string(rawBody)
	}
	headers := normalizePluginWebhookHeaders(c.Request.Header)
	queryParams := normalizePluginWebhookQueryParams(c.Request.URL.Query())
	action := strings.TrimSpace(webhook.Action)
	if action == "" {
		action = "webhook." + webhook.Key
	}

	params := map[string]string{
		"webhook_key":          webhook.Key,
		"webhook_method":       strings.ToUpper(strings.TrimSpace(c.Request.Method)),
		"webhook_path":         strings.TrimSpace(c.Request.URL.Path),
		"webhook_content_type": strings.TrimSpace(c.ContentType()),
	}
	execCtx := &service.ExecutionContext{
		SessionID: strings.TrimSpace(c.GetHeader("X-Session-ID")),
		Metadata: map[string]string{
			"request_path":               strings.TrimSpace(c.Request.URL.Path),
			"plugin_webhook_key":         webhook.Key,
			"plugin_webhook_method":      strings.ToUpper(strings.TrimSpace(c.Request.Method)),
			"plugin_webhook_auth_mode":   webhook.AuthMode,
			"plugin_webhook_plugin_name": plugin.Name,
		},
		RequestContext: c.Request.Context(),
		Webhook: &pluginipc.WebhookRequest{
			Key:         webhook.Key,
			Method:      strings.ToUpper(strings.TrimSpace(c.Request.Method)),
			Path:        strings.TrimSpace(c.Request.URL.Path),
			QueryString: strings.TrimSpace(c.Request.URL.RawQuery),
			QueryParams: queryParams,
			Headers:     headers,
			BodyText:    bodyText,
			BodyBase64:  base64.StdEncoding.EncodeToString(rawBody),
			ContentType: strings.TrimSpace(c.ContentType()),
			RemoteAddr:  strings.TrimSpace(utils.GetRealIP(c)),
		},
	}

	result, execErr := h.pluginManager.ExecutePlugin(plugin.ID, action, params, execCtx)
	if execErr != nil {
		h.respondPluginErrorErr(c, http.StatusInternalServerError, execErr)
		return
	}
	if result == nil {
		h.respondPluginError(c, http.StatusInternalServerError, "Plugin webhook returned empty result")
		return
	}
	if !result.Success {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":  false,
			"error":    strings.TrimSpace(result.Error),
			"data":     result.Data,
			"metadata": result.Metadata,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"data":     result.Data,
		"metadata": result.Metadata,
	})
}

func findStoredPluginWebhook(rawManifest string, key string) (pluginPackageWebhookManifest, bool, error) {
	items, err := parseStoredPluginWebhooks(rawManifest)
	if err != nil {
		return pluginPackageWebhookManifest{}, false, err
	}
	normalizedKey := strings.TrimSpace(key)
	for _, item := range items {
		if item.Key == normalizedKey {
			return item, true, nil
		}
	}
	return pluginPackageWebhookManifest{}, false, nil
}

func parseStoredPluginWebhooks(rawManifest string) ([]pluginPackageWebhookManifest, error) {
	manifest := parseJSONObjectString(rawManifest)
	if len(manifest) == 0 {
		return nil, nil
	}
	if manifest["webhooks"] == nil {
		return nil, nil
	}

	body, err := json.Marshal(manifest["webhooks"])
	if err != nil {
		return nil, err
	}
	var items []pluginPackageWebhookManifest
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, err
	}

	normalized := make([]pluginPackageWebhookManifest, 0, len(items))
	for _, item := range items {
		resolved, resolveErr := normalizeStoredPluginWebhook(item)
		if resolveErr != nil {
			return nil, resolveErr
		}
		normalized = append(normalized, resolved)
	}
	return normalized, nil
}

func normalizeStoredPluginWebhook(item pluginPackageWebhookManifest) (pluginPackageWebhookManifest, error) {
	item.Key = strings.TrimSpace(item.Key)
	if item.Key == "" {
		return item, fmt.Errorf("plugin webhook key is required")
	}
	method, err := normalizePluginWebhookMethod(item.Method)
	if err != nil {
		return item, err
	}
	authMode, err := normalizePluginWebhookAuthMode(item.AuthMode)
	if err != nil {
		return item, err
	}
	item.Method = method
	item.AuthMode = authMode
	item.Action = strings.TrimSpace(item.Action)
	if item.Action == "" {
		item.Action = "webhook." + item.Key
	}
	item.SecretKey = strings.TrimSpace(item.SecretKey)
	item.Header = strings.TrimSpace(item.Header)
	item.QueryParam = strings.TrimSpace(item.QueryParam)
	item.SignatureHeader = strings.TrimSpace(item.SignatureHeader)
	if item.AuthMode == "query" && item.QueryParam == "" {
		item.QueryParam = "token"
	}
	if item.AuthMode == "header" && item.Header == "" {
		item.Header = "X-Plugin-Webhook-Token"
	}
	if item.AuthMode == "hmac_sha256" && item.SignatureHeader == "" {
		item.SignatureHeader = "X-Plugin-Webhook-Signature"
	}
	return item, nil
}

func pluginWebhookAllowsMethod(webhook pluginPackageWebhookManifest, method string) bool {
	normalizedMethod, err := normalizePluginWebhookMethod(webhook.Method)
	if err != nil {
		return false
	}
	if normalizedMethod == "*" {
		return true
	}
	return strings.EqualFold(normalizedMethod, strings.TrimSpace(method))
}

func readPluginWebhookBody(body io.ReadCloser, limit int64) ([]byte, error) {
	if body == nil {
		return []byte{}, nil
	}
	defer body.Close()
	limited := io.LimitReader(body, limit+1)
	payload, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(payload)) > limit {
		return nil, fmt.Errorf("plugin webhook body exceeds %d bytes", limit)
	}
	return payload, nil
}

func authenticatePluginWebhookRequest(c *gin.Context, webhook pluginPackageWebhookManifest, secrets map[string]string, rawBody []byte) error {
	mode, err := normalizePluginWebhookAuthMode(webhook.AuthMode)
	if err != nil {
		return err
	}
	if mode == "none" {
		return nil
	}

	secret := secrets[strings.TrimSpace(webhook.SecretKey)]
	if secret == "" {
		return fmt.Errorf("plugin webhook secret %q is not configured", webhook.SecretKey)
	}

	switch mode {
	case "query":
		return verifyPluginWebhookQueryToken(c, webhook.QueryParam, secret)
	case "header":
		return verifyPluginWebhookHeaderToken(c, webhook.Header, secret)
	case "hmac_sha256":
		return verifyPluginWebhookHMAC(c, webhook.SignatureHeader, secret, rawBody)
	default:
		return fmt.Errorf("unsupported plugin webhook auth_mode %q", mode)
	}
}

func verifyPluginWebhookQueryToken(c *gin.Context, queryParam string, secret string) error {
	paramName := strings.TrimSpace(queryParam)
	if paramName == "" {
		paramName = "token"
	}
	return comparePluginWebhookSecret(secret, c.Query(paramName))
}

func verifyPluginWebhookHeaderToken(c *gin.Context, headerName string, secret string) error {
	resolvedHeader := strings.TrimSpace(headerName)
	if resolvedHeader == "" {
		resolvedHeader = "X-Plugin-Webhook-Token"
	}
	return comparePluginWebhookSecret(secret, c.GetHeader(resolvedHeader))
}

func verifyPluginWebhookHMAC(c *gin.Context, headerName string, secret string, rawBody []byte) error {
	resolvedHeader := strings.TrimSpace(headerName)
	if resolvedHeader == "" {
		resolvedHeader = "X-Plugin-Webhook-Signature"
	}
	provided := strings.ToLower(strings.TrimSpace(c.GetHeader(resolvedHeader)))
	if strings.HasPrefix(provided, "sha256=") {
		provided = strings.TrimPrefix(provided, "sha256=")
	}
	if provided == "" {
		return fmt.Errorf("plugin webhook signature is missing")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(rawBody)
	expected := hex.EncodeToString(mac.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(expected), []byte(provided)) != 1 {
		return fmt.Errorf("plugin webhook signature is invalid")
	}
	return nil
}

func comparePluginWebhookSecret(expected string, provided string) error {
	if subtle.ConstantTimeCompare([]byte(expected), []byte(strings.TrimSpace(provided))) != 1 {
		return fmt.Errorf("plugin webhook secret is invalid")
	}
	return nil
}

func normalizePluginWebhookHeaders(header http.Header) map[string]string {
	if len(header) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(header))
	for key, values := range header {
		normalizedKey := strings.ToLower(strings.TrimSpace(key))
		if normalizedKey == "" {
			continue
		}
		out[normalizedKey] = strings.TrimSpace(strings.Join(values, ","))
	}
	return out
}

func normalizePluginWebhookQueryParams(values map[string][]string) map[string]string {
	if len(values) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(values))
	for key, items := range values {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "" {
			continue
		}
		if len(items) == 0 {
			out[normalizedKey] = ""
			continue
		}
		out[normalizedKey] = strings.TrimSpace(items[0])
	}
	return out
}
