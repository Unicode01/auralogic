package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

const pluginHostMarketBridgeVersion = "1.0.0"

var pluginHostMarketArtifactKinds = []string{
	"plugin_package",
	"payment_package",
	"email_template",
	"landing_page_template",
	"invoice_template",
	"auth_branding_template",
	"page_rule_pack",
}

type PluginMarketSource struct {
	SourceID       string
	Name           string
	BaseURL        string
	PublicKey      string
	DefaultChannel string
	AllowedKinds   []string
	Enabled        bool
}

func (s PluginMarketSource) SupportsSignature() bool {
	return strings.TrimSpace(s.PublicKey) != ""
}

func (s PluginMarketSource) AllowsKind(kind string) bool {
	normalizedKind := normalizePluginMarketArtifactKind(kind)
	if normalizedKind == "" {
		return false
	}
	for _, allowed := range normalizePluginMarketArtifactKinds(s.AllowedKinds) {
		if allowed == normalizedKind {
			return true
		}
	}
	return false
}

func (s PluginMarketSource) Summary() map[string]interface{} {
	return map[string]interface{}{
		"source_id":          s.SourceID,
		"name":               s.Name,
		"base_url":           s.BaseURL,
		"default_channel":    s.DefaultChannel,
		"allowed_kinds":      append([]string(nil), s.AllowedKinds...),
		"supports_signature": s.SupportsSignature(),
		"enabled":            s.Enabled,
	}
}

type pluginMarketSourceClient struct {
	timeout time.Duration
}

func newPluginMarketSourceClient() *pluginMarketSourceClient {
	return &pluginMarketSourceClient{
		timeout: 12 * time.Second,
	}
}

func (c *pluginMarketSourceClient) FetchCatalog(ctx context.Context, source PluginMarketSource, params map[string]interface{}) (map[string]interface{}, error) {
	query := url.Values{}
	appendPluginMarketOptionalStringQuery(query, "kind", params, "kind")
	appendPluginMarketOptionalStringQuery(query, "channel", params, "channel")
	appendPluginMarketOptionalStringQuery(query, "q", params, "q", "search")
	appendPluginMarketOptionalIntQuery(query, "offset", params, "offset")
	appendPluginMarketOptionalIntQuery(query, "limit", params, "limit", "page_size", "pageSize")
	appendPluginMarketOptionalStringQuery(query, "host_version", params, "host_version", "hostVersion")
	appendPluginMarketOptionalStringQuery(query, "host_protocol_version", params, "host_protocol_version", "hostProtocolVersion")
	appendPluginMarketOptionalStringQuery(query, "host_bridge_version", params, "host_bridge_version", "hostBridgeVersion")
	appendPluginMarketOptionalStringQuery(query, "runtime", params, "runtime")
	return c.fetchJSON(ctx, source, "/v1/catalog", query)
}

func (c *pluginMarketSourceClient) FetchArtifact(ctx context.Context, source PluginMarketSource, kind string, name string) (map[string]interface{}, error) {
	return c.fetchJSON(ctx, source, pluginMarketArtifactPath(kind, name), nil)
}

func (c *pluginMarketSourceClient) FetchRelease(ctx context.Context, source PluginMarketSource, kind string, name string, version string) (map[string]interface{}, error) {
	return c.fetchJSON(ctx, source, pluginMarketReleasePath(kind, name, version), nil)
}

func (c *pluginMarketSourceClient) fetchJSON(ctx context.Context, source PluginMarketSource, relativePath string, query url.Values) (map[string]interface{}, error) {
	parsedBaseURL, err := parsePluginMarketHTTPURL(source.BaseURL, "market source base_url")
	if err != nil {
		return nil, err
	}
	parsedBaseURL.Path = strings.TrimRight(parsedBaseURL.Path, "/")
	parsedBaseURL.Path = path.Clean(parsedBaseURL.Path + "/" + strings.TrimLeft(relativePath, "/"))
	if query != nil && len(query) > 0 {
		parsedBaseURL.RawQuery = query.Encode()
	}

	reqCtx := ctx
	if reqCtx == nil {
		reqCtx = context.Background()
	}
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, parsedBaseURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build market source request failed: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	client := newPluginMarketSameOriginHTTPClient(c.timeout, parsedBaseURL)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request market source failed: %w", err)
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read market source response failed: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("market source request failed with status %d", resp.StatusCode)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return nil, fmt.Errorf("decode market source response failed: %w", err)
	}
	return unwrapPluginMarketEnvelope(decoded)
}

func unwrapPluginMarketEnvelope(decoded map[string]interface{}) (map[string]interface{}, error) {
	if len(decoded) == 0 {
		return map[string]interface{}{}, nil
	}

	if successRaw, exists := decoded["success"]; exists {
		if success, ok := successRaw.(bool); ok && !success {
			return nil, errors.New(pluginMarketEnvelopeErrorMessage(decoded))
		}
		if data, ok := decoded["data"].(map[string]interface{}); ok {
			return data, nil
		}
	}

	if codeRaw, exists := decoded["code"]; exists {
		code := int64(0)
		switch typed := codeRaw.(type) {
		case float64:
			code = int64(typed)
		case int64:
			code = typed
		case int:
			code = int64(typed)
		}
		if code != 0 {
			return nil, errors.New(pluginMarketEnvelopeErrorMessage(decoded))
		}
		if data, ok := decoded["data"].(map[string]interface{}); ok {
			return data, nil
		}
	}

	return decoded, nil
}

func pluginMarketEnvelopeErrorMessage(decoded map[string]interface{}) string {
	if errValue, ok := decoded["error"].(map[string]interface{}); ok {
		if message := strings.TrimSpace(pluginMarketStringFromAny(errValue["message"])); message != "" {
			return message
		}
	}
	if message := strings.TrimSpace(pluginMarketStringFromAny(decoded["message"])); message != "" {
		return message
	}
	return "market source request failed"
}

func pluginMarketArtifactPath(kind string, name string) string {
	return fmt.Sprintf(
		"/v1/artifacts/%s/%s",
		url.PathEscape(strings.TrimSpace(kind)),
		url.PathEscape(strings.TrimSpace(name)),
	)
}

func pluginMarketReleasePath(kind string, name string, version string) string {
	return fmt.Sprintf(
		"/v1/artifacts/%s/%s/releases/%s",
		url.PathEscape(strings.TrimSpace(kind)),
		url.PathEscape(strings.TrimSpace(name)),
		url.PathEscape(strings.TrimSpace(version)),
	)
}

func normalizePluginMarketArtifactKind(kind string) string {
	normalized := strings.ToLower(strings.TrimSpace(kind))
	for _, candidate := range pluginHostMarketArtifactKinds {
		if normalized == candidate {
			return normalized
		}
	}
	return ""
}

func normalizePluginMarketArtifactKinds(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		normalized := normalizePluginMarketArtifactKind(value)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	if len(out) > 0 {
		return out
	}
	return append([]string(nil), pluginHostMarketArtifactKinds...)
}

func pluginMarketStringFromAny(value interface{}) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", typed))
	}
}

func appendPluginMarketOptionalStringQuery(values url.Values, queryKey string, params map[string]interface{}, keys ...string) {
	if values == nil || params == nil {
		return
	}
	value := parsePluginHostOptionalString(params, keys...)
	if strings.TrimSpace(value) == "" {
		return
	}
	values.Set(queryKey, value)
}

func appendPluginMarketOptionalIntQuery(values url.Values, queryKey string, params map[string]interface{}, keys ...string) {
	if values == nil || params == nil {
		return
	}
	parsed, ok, err := parsePluginHostOptionalInt64(params, keys...)
	if err != nil || !ok {
		return
	}
	values.Set(queryKey, strconv.FormatInt(parsed, 10))
}

func parsePluginMarketHTTPURL(raw string, field string) (*url.URL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("%s is required", strings.TrimSpace(field))
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("%s is invalid: %w", strings.TrimSpace(field), err)
	}
	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("%s must use http or https", strings.TrimSpace(field))
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return nil, fmt.Errorf("%s host is required", strings.TrimSpace(field))
	}
	return parsed, nil
}

func resolvePluginMarketHTTPURL(base *url.URL, raw string, field string) (*url.URL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("%s is required", strings.TrimSpace(field))
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("%s is invalid: %w", strings.TrimSpace(field), err)
	}
	if !parsed.IsAbs() {
		if base == nil {
			return nil, fmt.Errorf("%s must be absolute", strings.TrimSpace(field))
		}
		parsed = base.ResolveReference(parsed)
	}
	parsed, err = parsePluginMarketHTTPURL(parsed.String(), field)
	if err != nil {
		return nil, err
	}
	if base != nil {
		if err := ensurePluginMarketSameOrigin(base, parsed, field); err != nil {
			return nil, err
		}
	}
	return parsed, nil
}

func ensurePluginMarketSameOrigin(base *url.URL, candidate *url.URL, field string) error {
	if base == nil || candidate == nil {
		return nil
	}
	if !strings.EqualFold(strings.TrimSpace(base.Scheme), strings.TrimSpace(candidate.Scheme)) ||
		!strings.EqualFold(strings.TrimSpace(base.Host), strings.TrimSpace(candidate.Host)) {
		return fmt.Errorf("%s must stay on the configured market source origin", strings.TrimSpace(field))
	}
	return nil
}

func newPluginMarketSameOriginHTTPClient(timeout time.Duration, origin *url.URL) *http.Client {
	if timeout <= 0 {
		timeout = 12 * time.Second
	}
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) == 0 {
				return nil
			}
			if err := ensurePluginMarketSameOrigin(origin, req.URL, "market request redirect"); err != nil {
				return err
			}
			return nil
		},
	}
}
