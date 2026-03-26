package sourceapi

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"auralogic/market_registry/internal/analytics"
	"auralogic/market_registry/internal/catalog"
	"auralogic/market_registry/internal/publish"
	"auralogic/market_registry/internal/signing"
	"auralogic/market_registry/internal/storage"
)

func TestSourceAPIExposesPublishedArtifacts(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	store, err := storage.NewLocalStorage(filepath.Join(root, "data"), "http://localhost:18080")
	if err != nil {
		t.Fatalf("NewLocalStorage returned error: %v", err)
	}
	signSvc := signing.NewService(filepath.Join(root, "keys"))
	if _, err := signSvc.GenerateKeyPair("official-test"); err != nil {
		t.Fatalf("GenerateKeyPair returned error: %v", err)
	}

	pubSvc := publish.NewService(store, signSvc, "official-test")
	if err := pubSvc.Publish(ctx, publish.Request{
		Kind:    "plugin_package",
		Channel: "stable",
		ArtifactZip: buildTestZip(t, map[string]string{
			"manifest.json": `{
  "name": "demo-plugin",
  "display_name": "Demo Plugin",
  "version": "1.0.0",
  "description": "demo plugin package",
  "runtime": "js_worker",
  "entry": "index.js",
  "manifest_version": "1.0.0",
  "protocol_version": "1.1.0",
  "min_host_protocol_version": "1.0.0",
  "min_host_bridge_version": "1.0.0",
  "icon_url": "https://example.com/plugin.png",
  "capabilities": {
    "requested_permissions": ["host.order.read"],
    "granted_permissions": ["host.order.read"]
  }
}`,
			"index.js": `module.exports = {};`,
		}),
		Metadata: publish.Metadata{
			Title:        "Demo Plugin",
			Summary:      "Published test plugin",
			Description:  "Plugin release used by source-api tests.",
			ReleaseNotes: "Initial runtime-backed release.",
			Publisher: publish.Publisher{
				ID:   "auralogic",
				Name: "AuraLogic",
			},
			Labels: []string{"official", "debugger"},
		},
	}); err != nil {
		t.Fatalf("Publish plugin returned error: %v", err)
	}

	if err := pubSvc.Publish(ctx, publish.Request{
		Channel: "stable",
		ArtifactZip: buildTestZip(t, map[string]string{
			"manifest.json": `{
  "kind": "email_template",
  "name": "order_paid",
  "title": "Order Paid Email",
  "version": "1.0.0",
  "event": "order_paid",
  "engine": "go_template",
  "content_file": "template.html",
  "subject": "Your order has been paid"
}`,
			"template.html": `<html><body>paid {{.OrderNo}}</body></html>`,
		}),
		Metadata: publish.Metadata{
			Title:       "Order Paid Email",
			Summary:     "Transactional order-paid email.",
			Description: "Email template served from storage-backed source-api.",
		},
	}); err != nil {
		t.Fatalf("Publish email template returned error: %v", err)
	}

	catalogStore := catalog.NewRuntimeStore(store, signSvc, catalog.RuntimeStoreConfig{
		SourceID:   "official",
		SourceName: "Test Source",
		KeyID:      "official-test",
		Now: func() time.Time {
			return time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
		},
	})
	analyticsSvc := analytics.NewService(store, analytics.Config{
		Now: func() time.Time {
			return time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
		},
	})

	server := httptest.NewServer(NewHandler(Config{
		Store:     catalogStore,
		Analytics: analyticsSvc,
		Registry:  pubSvc,
		AdminUI:   "/admin/ui/",
	}))
	defer server.Close()

	assertJSON := func(path string, headers map[string]string) map[string]any {
		t.Helper()
		req, err := http.NewRequest(http.MethodGet, server.URL+path, nil)
		if err != nil {
			t.Fatalf("NewRequest returned error: %v", err)
		}
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET %s failed: %v", path, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 for %s, got %d", path, resp.StatusCode)
		}
		var payload map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			t.Fatalf("decode %s failed: %v", path, err)
		}
		return payload
	}

	assertHTML := func(path string, contains ...string) {
		t.Helper()
		req, err := http.NewRequest(http.MethodGet, server.URL+path, nil)
		if err != nil {
			t.Fatalf("NewRequest returned error: %v", err)
		}
		req.Header.Set("Accept", "text/html")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET %s failed: %v", path, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 for %s, got %d", path, resp.StatusCode)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("read %s failed: %v", path, err)
		}
		text := string(body)
		for _, fragment := range contains {
			if !strings.Contains(text, fragment) {
				t.Fatalf("expected %s to contain %q, got %q", path, fragment, text)
			}
		}
	}

	assertHTML(
		"/",
		"Index of /",
		"/v1/",
	)
	rootResp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("GET / failed: %v", err)
	}
	defer rootResp.Body.Close()
	rootBody, err := io.ReadAll(rootResp.Body)
	if err != nil {
		t.Fatalf("read / failed: %v", err)
	}
	if strings.Contains(string(rootBody), "/admin/ui/") {
		t.Fatalf("expected root directory index not to expose admin ui path, got %q", string(rootBody))
	}
	assertHTML(
		"/v1/",
		"Index of /v1/",
		"/v1/source.json",
		"/v1/artifacts/",
	)
	assertHTML(
		"/v1/artifacts/",
		"plugin_package/",
		"email_template/",
	)
	assertHTML(
		"/v1/artifacts/plugin_package/",
		"demo-plugin/",
	)
	assertHTML(
		"/v1/artifacts/plugin_package/demo-plugin/",
		"artifact.json",
		"releases/",
	)
	assertHTML(
		"/v1/artifacts/plugin_package/demo-plugin/releases/",
		"1.0.0/",
	)
	assertHTML(
		"/v1/artifacts/plugin_package/demo-plugin/releases/1.0.0/",
		"release.json",
		"download",
		"sha256 ",
	)

	sourcePayload := assertJSON("/v1/source.json", nil)
	sourceData := sourcePayload["data"].(map[string]any)
	if got := sourceData["source_id"]; got != "official" {
		t.Fatalf("expected source_id official, got %#v", got)
	}
	signingData := sourceData["signing"].(map[string]any)
	if got := signingData["key_id"]; got != "official-test" {
		t.Fatalf("expected signing key_id official-test, got %#v", got)
	}

	catalogPayload := assertJSON("/v1/catalog?kind=plugin_package", nil)
	catalogItems := catalogPayload["data"].(map[string]any)["items"].([]any)
	if len(catalogItems) != 1 {
		t.Fatalf("expected single plugin package catalog item, got %#v", catalogItems)
	}
	catalogItem := catalogItems[0].(map[string]any)
	if got := catalogItem["icon_url"]; got != "https://example.com/plugin.png" {
		t.Fatalf("expected plugin icon_url, got %#v", got)
	}
	permissions := catalogItem["permissions"].(map[string]any)
	requested := permissions["requested"].([]any)
	if len(requested) != 1 || requested[0] != "host.order.read" {
		t.Fatalf("expected requested permissions to include host.order.read, got %#v", requested)
	}

	releasePayload := assertJSON("/v1/artifacts/email_template/order_paid/releases/1.0.0", nil)
	releaseData := releasePayload["data"].(map[string]any)
	templateData := releaseData["template"].(map[string]any)
	if content := templateData["content"]; content == "" {
		t.Fatalf("expected inline email template content, got %#v", templateData)
	}
	downloadData := releaseData["download"].(map[string]any)
	if downloadURL := downloadData["url"].(string); !strings.Contains(downloadURL, "/v1/artifacts/email_template/order_paid/releases/1.0.0/download") {
		t.Fatalf("expected release download url to be rewritten by runtime source-api, got %q", downloadURL)
	}
	if got := downloadData["filename"]; got != "order_paid-1.0.0.zip" {
		t.Fatalf("expected release download filename order_paid-1.0.0.zip, got %#v", got)
	}
	transport := downloadData["transport"].(map[string]any)
	if got := transport["provider"]; got != "local" {
		t.Fatalf("expected local download transport provider, got %#v", got)
	}
	if got := transport["mode"]; got != "mirror" {
		t.Fatalf("expected mirror download transport mode, got %#v", got)
	}

	artifactAliasPayload := assertJSON("/v1/artifacts/plugin_package/demo-plugin/artifact.json", nil)
	artifactAliasData := artifactAliasPayload["data"].(map[string]any)
	if got := artifactAliasData["name"]; got != "demo-plugin" {
		t.Fatalf("expected artifact alias to return demo-plugin, got %#v", got)
	}

	releaseAliasPayload := assertJSON("/v1/artifacts/plugin_package/demo-plugin/releases/1.0.0/release.json", nil)
	releaseAliasData := releaseAliasPayload["data"].(map[string]any)
	if got := releaseAliasData["version"]; got != "1.0.0" {
		t.Fatalf("expected release alias to return version 1.0.0, got %#v", got)
	}

	resp, err := http.Get(server.URL + "/v1/artifacts/plugin_package/demo-plugin/releases/1.0.0/download")
	if err != nil {
		t.Fatalf("download request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 download status, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Disposition"); !strings.Contains(got, "attachment") || !strings.Contains(got, "demo-plugin-1.0.0.zip") {
		t.Fatalf("expected Content-Disposition attachment header, got %q", got)
	}
	etag := resp.Header.Get("ETag")
	if etag == "" {
		t.Fatal("expected download response to include ETag")
	}
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read download body failed: %v", err)
	}
	if len(payload) == 0 {
		t.Fatal("expected non-empty download payload")
	}

	req, err := http.NewRequest(http.MethodGet, server.URL+"/v1/artifacts/plugin_package/demo-plugin/releases/1.0.0/download", nil)
	if err != nil {
		t.Fatalf("NewRequest for conditional download returned error: %v", err)
	}
	req.Header.Set("If-None-Match", etag)
	notModifiedResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("conditional download request failed: %v", err)
	}
	defer notModifiedResp.Body.Close()
	if notModifiedResp.StatusCode != http.StatusNotModified {
		t.Fatalf("expected 304 for conditional download, got %d", notModifiedResp.StatusCode)
	}

	overview, err := analyticsSvc.BuildOverview(ctx)
	if err != nil {
		t.Fatalf("BuildOverview returned error: %v", err)
	}
	if overview.TotalDownloads != 1 {
		t.Fatalf("expected TotalDownloads 1, got %d", overview.TotalDownloads)
	}
	if overview.TodayVisits != 6 {
		t.Fatalf("expected TodayVisits to include public API requests, got %d", overview.TodayVisits)
	}
}

func TestSourceAPIDedupesSameVersionAcrossChannels(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	store, err := storage.NewLocalStorage(filepath.Join(root, "data"), "http://localhost:18080")
	if err != nil {
		t.Fatalf("NewLocalStorage returned error: %v", err)
	}
	signSvc := signing.NewService(filepath.Join(root, "keys"))
	if _, err := signSvc.GenerateKeyPair("official-test"); err != nil {
		t.Fatalf("GenerateKeyPair returned error: %v", err)
	}

	pubSvc := publish.NewService(store, signSvc, "official-test")
	artifactZip := buildTestZip(t, map[string]string{
		"manifest.json": `{
  "name": "channel-demo",
  "display_name": "Channel Demo",
  "version": "1.0.0",
  "description": "channel demo package",
  "runtime": "js_worker",
  "entry": "index.js"
}`,
		"index.js": `module.exports = {};`,
	})
	for _, channel := range []string{"alpha", "stable", "beta"} {
		if err := pubSvc.Publish(ctx, publish.Request{
			Kind:        "plugin_package",
			Name:        "channel-demo",
			Version:     "1.0.0",
			Channel:     channel,
			ArtifactZip: artifactZip,
			Metadata: publish.Metadata{
				Title:   "Channel Demo",
				Summary: "Channel Demo Summary",
			},
		}); err != nil {
			t.Fatalf("Publish(%s) returned error: %v", channel, err)
		}
	}

	catalogStore := catalog.NewRuntimeStore(store, signSvc, catalog.RuntimeStoreConfig{
		SourceID:   "official",
		SourceName: "Test Source",
		KeyID:      "official-test",
	})

	server := httptest.NewServer(NewHandler(Config{
		Store:    catalogStore,
		Registry: pubSvc,
	}))
	defer server.Close()

	releasesResp, err := http.Get(server.URL + "/v1/artifacts/plugin_package/channel-demo/releases/")
	if err != nil {
		t.Fatalf("GET releases index returned error: %v", err)
	}
	defer releasesResp.Body.Close()
	releasesBody, err := io.ReadAll(releasesResp.Body)
	if err != nil {
		t.Fatalf("read releases index body failed: %v", err)
	}
	bodyText := string(releasesBody)
	if count := strings.Count(bodyText, `href="/v1/artifacts/plugin_package/channel-demo/releases/1.0.0/"`); count != 1 {
		t.Fatalf("expected one version directory entry, got %d in %q", count, bodyText)
	}
	for _, channel := range []string{"alpha", "stable", "beta"} {
		if !strings.Contains(bodyText, channel) {
			t.Fatalf("expected merged channel description to contain %q, got %q", channel, bodyText)
		}
	}

	releasePayload := assertSourceJSON(t, server.URL, "/v1/artifacts/plugin_package/channel-demo/releases/1.0.0")
	releaseData := releasePayload["data"].(map[string]any)
	channels := releaseData["channels"].([]any)
	if len(channels) != 3 {
		t.Fatalf("expected three channels in release payload, got %#v", channels)
	}

	for _, channel := range []string{"alpha", "stable", "beta"} {
		catalogPayload := assertSourceJSON(t, server.URL, "/v1/catalog?kind=plugin_package&channel="+channel)
		catalogData := catalogPayload["data"].(map[string]any)
		items := catalogData["items"].([]any)
		if len(items) != 1 {
			t.Fatalf("expected one catalog item for channel %s, got %#v", channel, catalogData["items"])
		}
		item := items[0].(map[string]any)
		itemChannels := item["channels"].([]any)
		if len(itemChannels) != 3 {
			t.Fatalf("expected merged item channels for channel %s, got %#v", channel, item["channels"])
		}
	}
}

func TestSourceAPIHealthzIncludesRegistrySummary(t *testing.T) {
	server := httptest.NewServer(NewHandler(Config{
		Store: stubSourceStore{},
		Registry: stubRegistryStatusProvider{
			status: publish.RegistryStatus{
				Healthy:       false,
				Status:        "stale",
				Message:       "registry snapshots need repair",
				CheckedAt:     "2026-03-15T10:00:00Z",
				ArtifactCount: 2,
				Issues:        []string{"catalog missing artifacts: plugin_package:demo-plugin"},
			},
		},
	}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for /healthz, got %d", resp.StatusCode)
	}

	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode /healthz failed: %v", err)
	}

	data := payload["data"].(map[string]any)
	if got := data["status"]; got != "degraded" {
		t.Fatalf("expected degraded health status, got %#v", got)
	}
	checks := data["checks"].(map[string]any)
	registry := checks["registry"].(map[string]any)
	if got := registry["status"]; got != "stale" {
		t.Fatalf("expected registry status stale, got %#v", got)
	}
}

func TestSourceAPIRegistryDiagnosticsEndpoint(t *testing.T) {
	server := httptest.NewServer(NewHandler(Config{
		Store: stubSourceStore{},
		Registry: stubRegistryStatusProvider{
			status: publish.RegistryStatus{
				Healthy:       true,
				Status:        "healthy",
				Message:       "registry snapshots are healthy",
				CheckedAt:     "2026-03-15T10:00:00Z",
				ArtifactCount: 1,
			},
		},
	}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/v1/diagnostics/registry")
	if err != nil {
		t.Fatalf("GET /v1/diagnostics/registry failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for diagnostics endpoint, got %d", resp.StatusCode)
	}

	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode diagnostics response failed: %v", err)
	}

	data := payload["data"].(map[string]any)
	if got := data["status"]; got != "healthy" {
		t.Fatalf("expected healthy diagnostics status, got %#v", got)
	}
	if got := data["artifactCount"]; got != float64(1) {
		t.Fatalf("expected artifactCount 1, got %#v", got)
	}
}

func TestSourceAPIRegistryDiagnosticsEndpointReturnsUnavailable(t *testing.T) {
	server := httptest.NewServer(NewHandler(Config{
		Store:    stubSourceStore{},
		Registry: stubRegistryStatusProvider{err: context.DeadlineExceeded},
	}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/v1/diagnostics/registry")
	if err != nil {
		t.Fatalf("GET /v1/diagnostics/registry failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for unavailable diagnostics endpoint, got %d", resp.StatusCode)
	}
}

func TestSourceAPIRejectsWrongMethod(t *testing.T) {
	server := httptest.NewServer(NewHandler(Config{
		Store: stubSourceStore{},
	}))
	defer server.Close()

	paths := []string{
		"/healthz",
		"/v1/source.json",
		"/v1/catalog",
	}
	for _, path := range paths {
		req, err := http.NewRequest(http.MethodPost, server.URL+path, nil)
		if err != nil {
			t.Fatalf("NewRequest(%s) returned error: %v", path, err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST %s failed: %v", path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Fatalf("%s: expected 405, got %d", path, resp.StatusCode)
		}
	}
}

func TestSourceAPISupportsRedirectDownloads(t *testing.T) {
	server := httptest.NewServer(NewHandler(Config{
		Store: redirectingSourceStore{},
	}))
	defer server.Close()

	client := http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Get(server.URL + "/v1/artifacts/plugin_package/demo/releases/1.0.0/download")
	if err != nil {
		t.Fatalf("GET redirect download failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusTemporaryRedirect {
		t.Fatalf("expected 307 redirect status, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Location"); got != "https://downloads.example.com/demo-1.0.0.zip" {
		t.Fatalf("expected redirect location, got %q", got)
	}
}

func buildTestZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	keys := make([]string, 0, len(files))
	for name := range files {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	for _, name := range keys {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("Create zip entry %s returned error: %v", name, err)
		}
		if _, err := entry.Write([]byte(files[name])); err != nil {
			t.Fatalf("Write zip entry %s returned error: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close zip writer returned error: %v", err)
	}
	return buffer.Bytes()
}

func assertSourceJSON(t *testing.T, serverURL string, targetPath string) map[string]any {
	t.Helper()
	resp, err := http.Get(serverURL + targetPath)
	if err != nil {
		t.Fatalf("GET %s failed: %v", targetPath, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for %s, got %d", targetPath, resp.StatusCode)
	}
	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode %s failed: %v", targetPath, err)
	}
	return payload
}

type stubSourceStore struct{}

func (stubSourceStore) BuildSourceDocument(context.Context, string) (map[string]any, error) {
	return map[string]any{}, nil
}

func (stubSourceStore) ListCatalog(context.Context, catalog.Query, string) ([]map[string]any, int, error) {
	return []map[string]any{}, 0, nil
}

func (stubSourceStore) BuildArtifactDocument(context.Context, string, string) (map[string]any, error) {
	return map[string]any{}, nil
}

func (stubSourceStore) BuildReleaseDocument(context.Context, string, string, string, string) (map[string]any, error) {
	return map[string]any{}, nil
}

func (stubSourceStore) ReadReleaseArtifact(context.Context, string, string, string) ([]byte, string, error) {
	return nil, "application/zip", nil
}

type stubRegistryStatusProvider struct {
	status publish.RegistryStatus
	err    error
}

func (s stubRegistryStatusProvider) RegistryStatus(context.Context) (publish.RegistryStatus, error) {
	return s.status, s.err
}

type redirectingSourceStore struct {
	stubSourceStore
}

func (redirectingSourceStore) BuildReleaseDocument(context.Context, string, string, string, string) (map[string]any, error) {
	return map[string]any{
		"download": map[string]any{
			"sha256": "demo-sha",
		},
	}, nil
}

func (redirectingSourceStore) ResolveReleaseDownloadRedirect(context.Context, string, string, string) (string, error) {
	return "https://downloads.example.com/demo-1.0.0.zip", nil
}
