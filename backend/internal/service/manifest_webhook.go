package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

type DeclaredWebhookManifest struct {
	Key             string `json:"key"`
	Description     string `json:"description"`
	Action          string `json:"action"`
	Method          string `json:"method"`
	AuthMode        string `json:"auth_mode"`
	SecretKey       string `json:"secret_key"`
	Header          string `json:"header"`
	QueryParam      string `json:"query_param"`
	SignatureHeader string `json:"signature_header"`
}

func ParseDeclaredWebhookManifests(rawManifest string) ([]DeclaredWebhookManifest, error) {
	manifest, err := parseDeclaredWebhookManifestObject(rawManifest)
	if err != nil || len(manifest) == 0 {
		return nil, err
	}
	rawItems, exists := manifest["webhooks"]
	if !exists || rawItems == nil {
		return nil, nil
	}

	body, err := json.Marshal(rawItems)
	if err != nil {
		return nil, err
	}
	var items []DeclaredWebhookManifest
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, err
	}

	normalized := make([]DeclaredWebhookManifest, 0, len(items))
	for _, item := range items {
		resolved, err := NormalizeDeclaredWebhookManifest(item)
		if err != nil {
			return nil, err
		}
		normalized = append(normalized, resolved)
	}
	return normalized, nil
}

func FindDeclaredWebhookManifest(items []DeclaredWebhookManifest, key string) (DeclaredWebhookManifest, bool) {
	normalizedKey := strings.TrimSpace(key)
	if normalizedKey == "" {
		return DeclaredWebhookManifest{}, false
	}
	for _, item := range items {
		if item.Key == normalizedKey {
			return item, true
		}
	}
	return DeclaredWebhookManifest{}, false
}

func NormalizeDeclaredWebhookManifest(item DeclaredWebhookManifest) (DeclaredWebhookManifest, error) {
	item.Key = strings.TrimSpace(item.Key)
	if item.Key == "" {
		return item, fmt.Errorf("webhook key is required")
	}

	method, err := NormalizeDeclaredWebhookMethod(item.Method)
	if err != nil {
		return item, err
	}
	authMode, err := NormalizeDeclaredWebhookAuthMode(item.AuthMode)
	if err != nil {
		return item, err
	}

	item.Method = method
	item.AuthMode = authMode
	item.Description = strings.TrimSpace(item.Description)
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

func NormalizeDeclaredWebhookMethod(raw string) (string, error) {
	method := strings.ToUpper(strings.TrimSpace(raw))
	if method == "" {
		return "POST", nil
	}
	if method == "*" || method == "ANY" {
		return "*", nil
	}
	switch method {
	case "GET", "POST", "PUT", "PATCH", "DELETE":
		return method, nil
	default:
		return "", fmt.Errorf("webhook method must be one of GET/POST/PUT/PATCH/DELETE/*")
	}
}

func NormalizeDeclaredWebhookAuthMode(raw string) (string, error) {
	mode := strings.ToLower(strings.TrimSpace(raw))
	if mode == "" {
		return "none", nil
	}
	switch mode {
	case "none", "query", "header", "hmac_sha256":
		return mode, nil
	default:
		return "", fmt.Errorf("webhook auth_mode must be one of none/query/header/hmac_sha256")
	}
}

func DeclaredWebhookAllowsMethod(webhook DeclaredWebhookManifest, method string) bool {
	normalizedMethod, err := NormalizeDeclaredWebhookMethod(webhook.Method)
	if err != nil {
		return false
	}
	if normalizedMethod == "*" {
		return true
	}
	return strings.EqualFold(normalizedMethod, strings.TrimSpace(method))
}

func BuildWebhookSecretsFromConfig(rawConfig string) (map[string]string, error) {
	trimmed := strings.TrimSpace(rawConfig)
	if trimmed == "" {
		return map[string]string{}, nil
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(decoded))
	for key, value := range decoded {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "" || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			out[normalizedKey] = strings.TrimSpace(typed)
		default:
			out[normalizedKey] = strings.TrimSpace(fmt.Sprintf("%v", typed))
		}
	}
	return out, nil
}

func AuthenticateDeclaredWebhookRequest(
	webhook DeclaredWebhookManifest,
	queryParams map[string]string,
	headers map[string]string,
	rawBody []byte,
	secrets map[string]string,
) error {
	mode, err := NormalizeDeclaredWebhookAuthMode(webhook.AuthMode)
	if err != nil {
		return err
	}
	if mode == "none" {
		return nil
	}

	secretKey := strings.TrimSpace(webhook.SecretKey)
	secret := strings.TrimSpace(secrets[secretKey])
	if secret == "" {
		return fmt.Errorf("webhook secret %q is not configured", secretKey)
	}

	switch mode {
	case "query":
		paramName := strings.TrimSpace(webhook.QueryParam)
		if paramName == "" {
			paramName = "token"
		}
		return compareDeclaredWebhookSecret(secret, queryParams[paramName])
	case "header":
		headerName := strings.ToLower(strings.TrimSpace(webhook.Header))
		if headerName == "" {
			headerName = strings.ToLower("X-Plugin-Webhook-Token")
		}
		return compareDeclaredWebhookSecret(secret, headers[headerName])
	case "hmac_sha256":
		headerName := strings.ToLower(strings.TrimSpace(webhook.SignatureHeader))
		if headerName == "" {
			headerName = strings.ToLower("X-Plugin-Webhook-Signature")
		}
		provided := strings.ToLower(strings.TrimSpace(headers[headerName]))
		if strings.HasPrefix(provided, "sha256=") {
			provided = strings.TrimPrefix(provided, "sha256=")
		}
		if provided == "" {
			return fmt.Errorf("webhook signature is missing")
		}
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write(rawBody)
		expected := hex.EncodeToString(mac.Sum(nil))
		if subtle.ConstantTimeCompare([]byte(expected), []byte(provided)) != 1 {
			return fmt.Errorf("webhook signature is invalid")
		}
		return nil
	default:
		return fmt.Errorf("unsupported webhook auth_mode %q", mode)
	}
}

func compareDeclaredWebhookSecret(expected string, provided string) error {
	if subtle.ConstantTimeCompare([]byte(expected), []byte(strings.TrimSpace(provided))) != 1 {
		return fmt.Errorf("webhook secret is invalid")
	}
	return nil
}

func parseDeclaredWebhookManifestObject(rawManifest string) (map[string]interface{}, error) {
	trimmed := strings.TrimSpace(rawManifest)
	if trimmed == "" {
		return nil, nil
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return nil, err
	}
	return parsed, nil
}
