package admin

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"auralogic/market_registry/internal/analytics"
	"auralogic/market_registry/internal/auth"
	"auralogic/market_registry/internal/publish"
	"auralogic/market_registry/internal/signing"
	"auralogic/market_registry/internal/storage"
)

func TestListArtifactsRequiresStructuredAuthError(t *testing.T) {
	handler, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/artifacts", nil)
	recorder := httptest.NewRecorder()
	handler.ListArtifacts(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("expected utf-8 json content type, got %q", got)
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal error payload returned error: %v", err)
	}
	errorPayload := payload["error"].(map[string]any)
	if got := errorPayload["code"]; got != "missing_authorization" {
		t.Fatalf("expected missing_authorization code, got %#v", got)
	}
	if got := errorPayload["message"]; got != "missing authorization" {
		t.Fatalf("expected missing authorization message, got %#v", got)
	}
}

func TestPublishAndListArtifacts(t *testing.T) {
	handler, token := newTestHandler(t)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("kind", "plugin_package"); err != nil {
		t.Fatalf("WriteField kind returned error: %v", err)
	}
	if err := writer.WriteField("channel", "stable"); err != nil {
		t.Fatalf("WriteField channel returned error: %v", err)
	}
	metadata, err := json.Marshal(publish.Metadata{
		Title:       "Admin Published Plugin",
		Summary:     "Published through admin handler test.",
		Description: "Used to validate admin index routes.",
		Publisher: publish.Publisher{
			ID:   "auralogic",
			Name: "AuraLogic",
		},
		Labels: []string{"official", "test"},
	})
	if err != nil {
		t.Fatalf("Marshal metadata returned error: %v", err)
	}
	if err := writer.WriteField("metadata", string(metadata)); err != nil {
		t.Fatalf("WriteField metadata returned error: %v", err)
	}
	fileWriter, err := writer.CreateFormFile("artifact", "demo-plugin.zip")
	if err != nil {
		t.Fatalf("CreateFormFile returned error: %v", err)
	}
	if _, err := fileWriter.Write(buildAdminTestZip(t, map[string]string{
		"manifest.json": `{
  "name": "admin-demo-plugin",
  "display_name": "Admin Demo Plugin",
  "version": "1.0.0",
  "description": "admin published plugin",
  "runtime": "js_worker",
  "entry": "index.js",
  "capabilities": {
    "requested_permissions": ["host.user.read"]
  }
}`,
		"index.js": `module.exports = {};`,
	})); err != nil {
		t.Fatalf("Write artifact payload returned error: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close multipart writer returned error: %v", err)
	}

	publishReq := httptest.NewRequest(http.MethodPost, "/admin/publish", body)
	publishReq.Header.Set("Authorization", "Bearer "+token)
	publishReq.Header.Set("Content-Type", writer.FormDataContentType())
	publishRecorder := httptest.NewRecorder()
	handler.Publish(publishRecorder, publishReq)
	if publishRecorder.Code != http.StatusOK {
		t.Fatalf("expected publish 200, got %d with body %s", publishRecorder.Code, publishRecorder.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/admin/artifacts", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listRecorder := httptest.NewRecorder()
	handler.ListArtifacts(listRecorder, listReq)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("expected list 200, got %d with body %s", listRecorder.Code, listRecorder.Body.String())
	}

	var listPayload map[string]any
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &listPayload); err != nil {
		t.Fatalf("Unmarshal list payload returned error: %v", err)
	}
	data := listPayload["data"].(map[string]any)
	pluginKind := data["plugin_package"].(map[string]any)
	artifact := pluginKind["admin-demo-plugin"].(map[string]any)
	if got := artifact["latest_version"]; got != "1.0.0" {
		t.Fatalf("expected latest_version 1.0.0, got %#v", got)
	}

	versionReq := httptest.NewRequest(http.MethodGet, "/admin/artifacts/plugin_package/admin-demo-plugin", nil)
	versionReq.Header.Set("Authorization", "Bearer "+token)
	versionRecorder := httptest.NewRecorder()
	handler.GetArtifactVersions(versionRecorder, versionReq)
	if versionRecorder.Code != http.StatusOK {
		t.Fatalf("expected versions 200, got %d with body %s", versionRecorder.Code, versionRecorder.Body.String())
	}

	var versionPayload map[string]any
	if err := json.Unmarshal(versionRecorder.Body.Bytes(), &versionPayload); err != nil {
		t.Fatalf("Unmarshal version payload returned error: %v", err)
	}
	versionData := versionPayload["data"].(map[string]any)
	releases := versionData["releases"].([]any)
	if len(releases) != 1 {
		t.Fatalf("expected 1 release entry, got %#v", releases)
	}

	if _, err := handler.storage.Read(context.Background(), "index/catalog.json"); err != nil {
		t.Fatalf("expected admin publish to rebuild catalog snapshot, got error: %v", err)
	}
}

func TestGetStatsUsesStoredAnalytics(t *testing.T) {
	handler, token := newTestHandler(t)
	statsSvc := analytics.NewService(handler.storage, analytics.Config{})

	if err := handler.storage.Write(context.Background(), "index/catalog.json", []byte(`{
  "items": [
    {"kind":"plugin_package","name":"demo-plugin"}
  ]
}`)); err != nil {
		t.Fatalf("Write catalog snapshot returned error: %v", err)
	}

	if err := statsSvc.RecordEvent(context.Background(), analytics.Event{Type: analytics.EventCatalogView}); err != nil {
		t.Fatalf("RecordEvent catalog view returned error: %v", err)
	}
	if err := statsSvc.RecordEvent(context.Background(), analytics.Event{
		Type:         analytics.EventDownload,
		ArtifactKind: "plugin_package",
		ArtifactName: "demo-plugin",
	}); err != nil {
		t.Fatalf("RecordEvent download returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/stats/overview", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	handler.GetStats(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected stats 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal stats payload returned error: %v", err)
	}
	data := payload["data"].(map[string]any)
	if got := data["totalArtifacts"]; got != float64(1) {
		t.Fatalf("expected totalArtifacts 1, got %#v", got)
	}
	if got := data["totalDownloads"]; got != float64(1) {
		t.Fatalf("expected totalDownloads 1, got %#v", got)
	}
	if got := data["todayVisits"]; got != float64(2) {
		t.Fatalf("expected todayVisits 2, got %#v", got)
	}
	storageData, ok := data["storage"].(map[string]any)
	if !ok {
		t.Fatalf("expected storage overview object, got %#v", data["storage"])
	}
	if got := storageData["available"]; got != true {
		t.Fatalf("expected storage overview available, got %#v", got)
	}
	if got := storageData["backend"]; got != "local" {
		t.Fatalf("expected local storage backend, got %#v", got)
	}
}

func TestRegistryStatusAndReindexEndpoints(t *testing.T) {
	handler, token := newTestHandler(t)

	statusReq := httptest.NewRequest(http.MethodGet, "/admin/registry/status", nil)
	statusReq.Header.Set("Authorization", "Bearer "+token)
	statusRecorder := httptest.NewRecorder()
	handler.GetRegistryStatus(statusRecorder, statusReq)
	if statusRecorder.Code != http.StatusOK {
		t.Fatalf("expected registry status 200, got %d with body %s", statusRecorder.Code, statusRecorder.Body.String())
	}

	var statusPayload map[string]any
	if err := json.Unmarshal(statusRecorder.Body.Bytes(), &statusPayload); err != nil {
		t.Fatalf("Unmarshal registry status payload returned error: %v", err)
	}
	statusData := statusPayload["data"].(map[string]any)
	if got := statusData["healthy"]; got != false {
		t.Fatalf("expected registry to be unhealthy before reindex, got %#v", got)
	}

	reindexReq := httptest.NewRequest(http.MethodPost, "/admin/registry/reindex", nil)
	reindexReq.Header.Set("Authorization", "Bearer "+token)
	reindexRecorder := httptest.NewRecorder()
	handler.ReindexRegistry(reindexRecorder, reindexReq)
	if reindexRecorder.Code != http.StatusOK {
		t.Fatalf("expected registry reindex 200, got %d with body %s", reindexRecorder.Code, reindexRecorder.Body.String())
	}

	var reindexPayload map[string]any
	if err := json.Unmarshal(reindexRecorder.Body.Bytes(), &reindexPayload); err != nil {
		t.Fatalf("Unmarshal reindex payload returned error: %v", err)
	}
	data := reindexPayload["data"].(map[string]any)
	registryStatus := data["status"].(map[string]any)
	if got := registryStatus["healthy"]; got != true {
		t.Fatalf("expected registry to be healthy after reindex, got %#v", got)
	}
}

func TestSyncGitHubReleaseEndpoint(t *testing.T) {
	handler, token := newTestHandler(t)

	zipPayload := buildAdminTestZip(t, map[string]string{
		"manifest.json": `{
  "name": "admin-sync-demo",
  "display_name": "Admin Sync Demo",
  "version": "1.2.3",
  "description": "admin synced plugin",
  "runtime": "js_worker",
  "entry": "index.js"
}`,
		"index.js": `module.exports = {};`,
	})

	serverURL := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/auralogic/plugins/releases/tags/v1.2.3":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
  "id": 3001,
  "tag_name": "v1.2.3",
  "name": "Admin Sync Demo Release",
  "body": "Admin sync release notes",
  "html_url": "https://github.example.com/auralogic/plugins/releases/tag/v1.2.3",
  "published_at": "2026-03-15T12:00:00Z",
  "author": { "login": "auralogic" },
  "assets": [
    {
      "id": 4002,
      "name": "admin-sync-demo-1.2.3.zip",
      "url": "` + serverURL + `/assets/4002",
      "browser_download_url": "` + serverURL + `/downloads/admin-sync-demo-1.2.3.zip",
      "size": ` + strconv.Itoa(len(zipPayload)) + `,
      "content_type": "application/zip"
    }
  ]
}`))
		case "/downloads/admin-sync-demo-1.2.3.zip", "/assets/4002":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(zipPayload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	body, err := json.Marshal(map[string]any{
		"owner":        "auralogic",
		"repo":         "plugins",
		"tag":          "v1.2.3",
		"asset":        "admin-sync-demo-1.2.3.zip",
		"api_base_url": server.URL,
		"metadata": map[string]any{
			"title":   "Synced From Admin",
			"summary": "Admin-triggered sync",
		},
	})
	if err != nil {
		t.Fatalf("Marshal request returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/sync/github-release", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.SyncGitHubRelease(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected sync 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal sync response returned error: %v", err)
	}
	data := payload["data"].(map[string]any)
	result := data["result"].(map[string]any)
	if got := result["kind"]; got != "plugin_package" {
		t.Fatalf("expected inferred plugin_package kind, got %#v", got)
	}
	if got := result["name"]; got != "admin-sync-demo" {
		t.Fatalf("expected synced artifact name, got %#v", got)
	}
	status := data["status"].(map[string]any)
	if got := status["healthy"]; got != true {
		t.Fatalf("expected healthy registry status after sync, got %#v", got)
	}

	manifestBody, err := handler.storage.Read(context.Background(), "artifacts/plugin_package/admin-sync-demo/1.2.3/manifest.json")
	if err != nil {
		t.Fatalf("expected synced manifest to be written, got error: %v", err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(manifestBody, &manifest); err != nil {
		t.Fatalf("Unmarshal synced manifest returned error: %v", err)
	}
	if got := manifest["title"]; got != "Synced From Admin" {
		t.Fatalf("expected metadata title override, got %#v", got)
	}

	versionsReq := httptest.NewRequest(http.MethodGet, "/admin/artifacts/plugin_package/admin-sync-demo", nil)
	versionsReq.Header.Set("Authorization", "Bearer "+token)
	versionsRecorder := httptest.NewRecorder()
	handler.GetArtifactVersions(versionsRecorder, versionsReq)
	if versionsRecorder.Code != http.StatusOK {
		t.Fatalf("expected versions 200 after sync, got %d with body %s", versionsRecorder.Code, versionsRecorder.Body.String())
	}

	var versionsPayload map[string]any
	if err := json.Unmarshal(versionsRecorder.Body.Bytes(), &versionsPayload); err != nil {
		t.Fatalf("Unmarshal versions payload returned error: %v", err)
	}
	versionData := versionsPayload["data"].(map[string]any)
	latestTransport := versionData["latest_transport"].(map[string]any)
	if got := latestTransport["provider"]; got != "github_release" {
		t.Fatalf("expected latest transport provider github_release, got %#v", got)
	}
	releases := versionData["releases"].([]any)
	if len(releases) != 1 {
		t.Fatalf("expected one release row, got %#v", releases)
	}
	firstRelease := releases[0].(map[string]any)
	if got := firstRelease["download_url"]; got == "" {
		t.Fatalf("expected release download_url, got %#v", firstRelease)
	}
	if _, exists := firstRelease["origin"]; exists {
		t.Fatalf("expected version summary to omit origin payload, got %#v", firstRelease["origin"])
	}

	releaseReq := httptest.NewRequest(http.MethodGet, "/admin/artifacts/plugin_package/admin-sync-demo/1.2.3", nil)
	releaseReq.Header.Set("Authorization", "Bearer "+token)
	releaseRecorder := httptest.NewRecorder()
	handler.GetArtifactRelease(releaseRecorder, releaseReq)
	if releaseRecorder.Code != http.StatusOK {
		t.Fatalf("expected release detail 200, got %d with body %s", releaseRecorder.Code, releaseRecorder.Body.String())
	}

	var releasePayload map[string]any
	if err := json.Unmarshal(releaseRecorder.Body.Bytes(), &releasePayload); err != nil {
		t.Fatalf("Unmarshal release detail payload returned error: %v", err)
	}
	releaseData := releasePayload["data"].(map[string]any)
	origin := releaseData["origin"].(map[string]any)
	if got := origin["provider"]; got != "github_release" {
		t.Fatalf("expected release origin provider github_release, got %#v", got)
	}
}

func TestSyncGitHubReleaseEndpointRejectsInvalidAPIBaseURL(t *testing.T) {
	handler, token := newTestHandler(t)

	body, err := json.Marshal(map[string]any{
		"owner":        "auralogic",
		"repo":         "plugins",
		"tag":          "v1.2.3",
		"asset":        "admin-sync-demo-1.2.3.zip",
		"api_base_url": "ftp://github.example.com/api/v3",
	})
	if err != nil {
		t.Fatalf("Marshal request returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/sync/github-release", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.SyncGitHubRelease(recorder, req)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected sync 400, got %d with body %s", recorder.Code, recorder.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal sync response returned error: %v", err)
	}
	errorPayload := payload["error"].(map[string]any)
	if got := errorPayload["code"]; got != "invalid_sync_request" {
		t.Fatalf("expected invalid_sync_request code, got %#v", got)
	}
	if got := errorPayload["message"]; got != "github api base url must use http or https" {
		t.Fatalf("unexpected sync error message: %#v", got)
	}
}

func TestPreviewGitHubReleaseEndpoint(t *testing.T) {
	handler, token := newTestHandler(t)

	serverURL := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/auralogic/plugins/releases/tags/v1.2.3":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
  "id": 4201,
  "tag_name": "v1.2.3",
  "name": "Preview Sync Demo Release",
  "body": "Preview release notes",
  "html_url": "https://github.example.com/auralogic/plugins/releases/tag/v1.2.3",
  "published_at": "2026-03-15T12:00:00Z",
  "author": { "login": "auralogic" },
  "assets": [
    {
      "id": 4202,
      "name": "preview-sync-demo-1.2.3.zip",
      "url": "` + serverURL + `/assets/4202",
      "browser_download_url": "` + serverURL + `/downloads/preview-sync-demo-1.2.3.zip",
      "size": 123,
      "content_type": "application/zip",
      "updated_at": "2026-03-16T08:00:00Z"
    },
    {
      "id": 4203,
      "name": "preview-sync-demo-1.2.3.sha256",
      "url": "` + serverURL + `/assets/4203",
      "browser_download_url": "` + serverURL + `/downloads/preview-sync-demo-1.2.3.sha256",
      "size": 64,
      "content_type": "text/plain",
      "updated_at": "2026-03-16T08:01:00Z"
    }
  ]
}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	body, err := json.Marshal(map[string]any{
		"owner":        "auralogic",
		"repo":         "plugins",
		"tag":          "v1.2.3",
		"asset":        "preview-sync-demo-1.2.3.zip",
		"api_base_url": server.URL,
	})
	if err != nil {
		t.Fatalf("Marshal request returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/sync/github-release/release", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.PreviewGitHubRelease(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected preview 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal preview response returned error: %v", err)
	}
	result := payload["data"].(map[string]any)["result"].(map[string]any)
	if got := result["selected_asset"]; got != "preview-sync-demo-1.2.3.zip" {
		t.Fatalf("expected selected asset preview-sync-demo-1.2.3.zip, got %#v", got)
	}
	if got := result["asset_count"]; got != float64(2) {
		t.Fatalf("expected asset_count 2, got %#v", got)
	}
	assets := result["assets"].([]any)
	if len(assets) != 2 {
		t.Fatalf("expected two preview assets, got %#v", assets)
	}
}

func TestInspectGitHubReleaseEndpoint(t *testing.T) {
	handler, token := newTestHandler(t)

	zipPayload := buildAdminTestZip(t, map[string]string{
		"manifest.json": `{
  "name": "inspect-sync-demo",
  "display_name": "Inspect Sync Demo",
  "version": "1.2.3",
  "description": "inspect sync payload",
  "runtime": "js_worker",
  "entry": "index.js"
}`,
		"index.js": `module.exports = {};`,
	})

	serverURL := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/auralogic/plugins/releases/tags/v1.2.3":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
  "id": 4101,
  "tag_name": "v1.2.3",
  "name": "Inspect Sync Demo Release",
  "body": "Inspect release notes",
  "html_url": "https://github.example.com/auralogic/plugins/releases/tag/v1.2.3",
  "published_at": "2026-03-15T12:00:00Z",
  "author": { "login": "auralogic" },
  "assets": [
    {
      "id": 4102,
      "name": "inspect-sync-demo-1.2.3.zip",
      "url": "` + serverURL + `/assets/4102",
      "browser_download_url": "` + serverURL + `/downloads/inspect-sync-demo-1.2.3.zip",
      "size": ` + strconv.Itoa(len(zipPayload)) + `,
      "content_type": "application/zip",
      "updated_at": "2026-03-16T08:00:00Z"
    }
  ]
}`))
		case "/downloads/inspect-sync-demo-1.2.3.zip", "/assets/4102":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(zipPayload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	body, err := json.Marshal(map[string]any{
		"kind":         "plugin_package",
		"name":         "inspect-sync-demo",
		"version":      "1.2.3",
		"owner":        "auralogic",
		"repo":         "plugins",
		"tag":          "v1.2.3",
		"asset":        "inspect-sync-demo-1.2.3.zip",
		"api_base_url": server.URL,
	})
	if err != nil {
		t.Fatalf("Marshal request returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/sync/github-release/inspect", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.InspectGitHubRelease(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected inspect 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal inspect response returned error: %v", err)
	}
	result := payload["data"].(map[string]any)["result"].(map[string]any)
	if got := result["kind"]; got != "plugin_package" {
		t.Fatalf("expected inferred plugin_package kind, got %#v", got)
	}
	if got := result["name"]; got != "inspect-sync-demo" {
		t.Fatalf("expected synced artifact name, got %#v", got)
	}
	if got := result["version"]; got != "1.2.3" {
		t.Fatalf("expected synced artifact version, got %#v", got)
	}
	if got := result["changed"]; got != false {
		t.Fatalf("expected inspect changed=false, got %#v", got)
	}
	if got := result["asset_size"]; got != float64(len(zipPayload)) {
		t.Fatalf("expected asset_size %d, got %#v", len(zipPayload), got)
	}
}

func TestResyncArtifactVersionEndpoint(t *testing.T) {
	handler, token := newTestHandler(t)

	zipPayloadV1 := buildAdminTestZip(t, map[string]string{
		"manifest.json": `{
  "name": "resync-demo",
  "display_name": "Resync Demo",
  "version": "2.0.0",
  "description": "original payload",
  "runtime": "js_worker",
  "entry": "index.js"
}`,
		"index.js": `module.exports = { version: "v1" };`,
	})
	zipPayloadV2 := buildAdminTestZip(t, map[string]string{
		"manifest.json": `{
  "name": "resync-demo",
  "display_name": "Resync Demo",
  "version": "2.0.0",
  "description": "updated payload",
  "runtime": "js_worker",
  "entry": "index.js"
}`,
		"index.js": `module.exports = { version: "v2" };`,
	})

	currentPayload := zipPayloadV1
	serverURL := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/auralogic/plugins/releases/tags/v2.0.0":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
  "id": 501,
  "tag_name": "v2.0.0",
  "name": "Resync Demo Release",
  "body": "Resync demo notes",
  "author": { "login": "auralogic" },
  "assets": [
    {
      "id": 601,
      "name": "resync-demo-2.0.0.zip",
      "url": "` + serverURL + `/assets/601",
      "browser_download_url": "` + serverURL + `/downloads/resync-demo-2.0.0.zip",
      "size": ` + strconv.Itoa(len(currentPayload)) + `,
      "content_type": "application/zip"
    }
  ]
}`))
		case "/downloads/resync-demo-2.0.0.zip", "/assets/601":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(currentPayload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	syncBody, err := json.Marshal(map[string]any{
		"owner":        "auralogic",
		"repo":         "plugins",
		"tag":          "v2.0.0",
		"asset":        "resync-demo-2.0.0.zip",
		"api_base_url": server.URL,
	})
	if err != nil {
		t.Fatalf("Marshal sync request returned error: %v", err)
	}
	syncReq := httptest.NewRequest(http.MethodPost, "/admin/sync/github-release", bytes.NewReader(syncBody))
	syncReq.Header.Set("Authorization", "Bearer "+token)
	syncReq.Header.Set("Content-Type", "application/json")
	syncRecorder := httptest.NewRecorder()
	handler.SyncGitHubRelease(syncRecorder, syncReq)
	if syncRecorder.Code != http.StatusOK {
		t.Fatalf("expected initial sync 200, got %d with body %s", syncRecorder.Code, syncRecorder.Body.String())
	}

	beforeBody, err := handler.storage.Read(context.Background(), "artifacts/plugin_package/resync-demo/2.0.0/plugin_package-resync-demo-2.0.0.zip")
	if err == nil && len(beforeBody) > 0 {
		t.Fatalf("unexpected path read succeeded; expected canonical path naming from publish service")
	}
	artifactPath := "artifacts/plugin_package/resync-demo/2.0.0/resync-demo-2.0.0.zip"
	beforeBody, err = handler.storage.Read(context.Background(), artifactPath)
	if err != nil {
		t.Fatalf("Read before resync returned error: %v", err)
	}

	currentPayload = zipPayloadV2
	resyncReq := httptest.NewRequest(http.MethodPost, "/admin/artifacts/plugin_package/resync-demo/2.0.0/resync", strings.NewReader(`{}`))
	resyncReq.Header.Set("Authorization", "Bearer "+token)
	resyncReq.Header.Set("Content-Type", "application/json")
	resyncRecorder := httptest.NewRecorder()
	handler.ResyncArtifactVersion(resyncRecorder, resyncReq)
	if resyncRecorder.Code != http.StatusOK {
		t.Fatalf("expected resync 200, got %d with body %s", resyncRecorder.Code, resyncRecorder.Body.String())
	}

	afterBody, err := handler.storage.Read(context.Background(), artifactPath)
	if err != nil {
		t.Fatalf("Read after resync returned error: %v", err)
	}
	if bytes.Equal(beforeBody, afterBody) {
		t.Fatal("expected canonical artifact payload to change after resync")
	}
}

func TestCheckArtifactOriginEndpoint(t *testing.T) {
	handler, token := newTestHandler(t)

	zipPayloadV1 := buildAdminTestZip(t, map[string]string{
		"manifest.json": `{
  "name": "check-demo",
  "display_name": "Check Demo",
  "version": "3.0.0",
  "description": "original payload",
  "runtime": "js_worker",
  "entry": "index.js"
}`,
		"index.js": `module.exports = { version: "v1" };`,
	})
	zipPayloadV2 := buildAdminTestZip(t, map[string]string{
		"manifest.json": `{
  "name": "check-demo",
  "display_name": "Check Demo",
  "version": "3.0.0",
  "description": "updated payload",
  "runtime": "js_worker",
  "entry": "index.js"
}`,
		"index.js": `module.exports = { version: "v2" };`,
	})

	currentPayload := zipPayloadV1
	serverURL := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/auralogic/plugins/releases/tags/v3.0.0":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
  "id": 701,
  "tag_name": "v3.0.0",
  "name": "Check Demo Release",
  "body": "Check demo notes",
  "author": { "login": "auralogic" },
  "assets": [
    {
      "id": 801,
      "name": "check-demo-3.0.0.zip",
      "url": "` + serverURL + `/assets/801",
      "browser_download_url": "` + serverURL + `/downloads/check-demo-3.0.0.zip",
      "size": ` + strconv.Itoa(len(currentPayload)) + `,
      "content_type": "application/zip",
      "updated_at": "2026-03-16T12:00:00Z"
    }
  ]
}`))
		case "/downloads/check-demo-3.0.0.zip", "/assets/801":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(currentPayload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	syncBody, err := json.Marshal(map[string]any{
		"owner":        "auralogic",
		"repo":         "plugins",
		"tag":          "v3.0.0",
		"asset":        "check-demo-3.0.0.zip",
		"api_base_url": server.URL,
	})
	if err != nil {
		t.Fatalf("Marshal sync request returned error: %v", err)
	}
	syncReq := httptest.NewRequest(http.MethodPost, "/admin/sync/github-release", bytes.NewReader(syncBody))
	syncReq.Header.Set("Authorization", "Bearer "+token)
	syncReq.Header.Set("Content-Type", "application/json")
	syncRecorder := httptest.NewRecorder()
	handler.SyncGitHubRelease(syncRecorder, syncReq)
	if syncRecorder.Code != http.StatusOK {
		t.Fatalf("expected initial sync 200, got %d with body %s", syncRecorder.Code, syncRecorder.Body.String())
	}

	currentPayload = zipPayloadV2
	checkReq := httptest.NewRequest(http.MethodPost, "/admin/artifacts/plugin_package/check-demo/3.0.0/check-origin", strings.NewReader(`{}`))
	checkReq.Header.Set("Authorization", "Bearer "+token)
	checkReq.Header.Set("Content-Type", "application/json")
	checkRecorder := httptest.NewRecorder()
	handler.CheckArtifactOrigin(checkRecorder, checkReq)
	if checkRecorder.Code != http.StatusOK {
		t.Fatalf("expected check-origin 200, got %d with body %s", checkRecorder.Code, checkRecorder.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(checkRecorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal check-origin payload returned error: %v", err)
	}
	result := payload["data"].(map[string]any)["result"].(map[string]any)
	if got := result["changed"]; got != true {
		t.Fatalf("expected changed=true, got %#v", got)
	}
	changedFields := result["changed_fields"].([]any)
	if len(changedFields) == 0 {
		t.Fatalf("expected changed fields, got %#v", result)
	}
}

func TestCheckArtifactOriginsEndpoint(t *testing.T) {
	handler, token := newTestHandler(t)

	versionPayloads := map[string][]byte{
		"v4.0.0": buildAdminTestZip(t, map[string]string{
			"manifest.json": `{
  "name": "batch-check-demo",
  "display_name": "Batch Check Demo",
  "version": "4.0.0",
  "description": "stable payload",
  "runtime": "js_worker",
  "entry": "index.js"
}`,
			"index.js": `module.exports = { version: "v4.0.0" };`,
		}),
		"v4.1.0": buildAdminTestZip(t, map[string]string{
			"manifest.json": `{
  "name": "batch-check-demo",
  "display_name": "Batch Check Demo",
  "version": "4.1.0",
  "description": "old payload",
  "runtime": "js_worker",
  "entry": "index.js"
}`,
			"index.js": `module.exports = { version: "old" };`,
		}),
	}
	updatedPayload := buildAdminTestZip(t, map[string]string{
		"manifest.json": `{
  "name": "batch-check-demo",
  "display_name": "Batch Check Demo",
  "version": "4.1.0",
  "description": "new payload",
  "runtime": "js_worker",
  "entry": "index.js"
}`,
		"index.js": `module.exports = { version: "new" };`,
	})

	serverURL := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/auralogic/plugins/releases/tags/v4.0.0":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
  "id": 901,
  "tag_name": "v4.0.0",
  "name": "Batch Check Demo 4.0.0",
  "body": "Batch check demo 4.0.0",
  "author": { "login": "auralogic" },
  "assets": [
    {
      "id": 1001,
      "name": "batch-check-demo-4.0.0.zip",
      "url": "` + serverURL + `/assets/1001",
      "browser_download_url": "` + serverURL + `/downloads/batch-check-demo-4.0.0.zip",
      "size": ` + strconv.Itoa(len(versionPayloads["v4.0.0"])) + `,
      "content_type": "application/zip",
      "updated_at": "2026-03-16T10:00:00Z"
    }
  ]
}`))
		case "/repos/auralogic/plugins/releases/tags/v4.1.0":
			_, _ = w.Write([]byte(`{
  "id": 902,
  "tag_name": "v4.1.0",
  "name": "Batch Check Demo 4.1.0",
  "body": "Batch check demo 4.1.0",
  "author": { "login": "auralogic" },
  "assets": [
    {
      "id": 1002,
      "name": "batch-check-demo-4.1.0.zip",
      "url": "` + serverURL + `/assets/1002",
      "browser_download_url": "` + serverURL + `/downloads/batch-check-demo-4.1.0.zip",
      "size": ` + strconv.Itoa(len(versionPayloads["v4.1.0"])) + `,
      "content_type": "application/zip",
      "updated_at": "2026-03-16T11:00:00Z"
    }
  ]
}`))
		case "/downloads/batch-check-demo-4.0.0.zip", "/assets/1001":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(versionPayloads["v4.0.0"])
		case "/downloads/batch-check-demo-4.1.0.zip", "/assets/1002":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(versionPayloads["v4.1.0"])
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	for _, version := range []string{"v4.0.0", "v4.1.0"} {
		syncBody, err := json.Marshal(map[string]any{
			"owner":        "auralogic",
			"repo":         "plugins",
			"tag":          version,
			"asset":        "batch-check-demo-" + strings.TrimPrefix(version, "v") + ".zip",
			"api_base_url": server.URL,
		})
		if err != nil {
			t.Fatalf("Marshal sync request returned error: %v", err)
		}
		syncReq := httptest.NewRequest(http.MethodPost, "/admin/sync/github-release", bytes.NewReader(syncBody))
		syncReq.Header.Set("Authorization", "Bearer "+token)
		syncReq.Header.Set("Content-Type", "application/json")
		syncRecorder := httptest.NewRecorder()
		handler.SyncGitHubRelease(syncRecorder, syncReq)
		if syncRecorder.Code != http.StatusOK {
			t.Fatalf("expected sync 200 for %s, got %d with body %s", version, syncRecorder.Code, syncRecorder.Body.String())
		}
	}

	versionPayloads["v4.1.0"] = updatedPayload
	checkReq := httptest.NewRequest(http.MethodPost, "/admin/artifacts/plugin_package/batch-check-demo/check-origin", strings.NewReader(`{}`))
	checkReq.Header.Set("Authorization", "Bearer "+token)
	checkReq.Header.Set("Content-Type", "application/json")
	checkRecorder := httptest.NewRecorder()
	handler.CheckArtifactOrigins(checkRecorder, checkReq)
	if checkRecorder.Code != http.StatusOK {
		t.Fatalf("expected batch check 200, got %d with body %s", checkRecorder.Code, checkRecorder.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(checkRecorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal batch check payload returned error: %v", err)
	}
	data := payload["data"].(map[string]any)
	if got := data["checked_versions"]; got != float64(2) {
		t.Fatalf("expected checked_versions=2, got %#v", got)
	}
	if got := data["changed_versions"]; got != float64(1) {
		t.Fatalf("expected changed_versions=1, got %#v", got)
	}
	items := data["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("expected two batch check items, got %#v", items)
	}
}

func TestDeleteArtifactReleaseEndpoint(t *testing.T) {
	handler, token := newTestHandler(t)

	publishVersion := func(version string) {
		t.Helper()
		zipPayload := buildAdminTestZip(t, map[string]string{
			"manifest.json": `{
  "name": "delete-handler-demo",
  "display_name": "Delete Handler Demo",
  "version": "` + version + `",
  "description": "admin delete test",
  "runtime": "js_worker",
  "entry": "index.js"
}`,
			"index.js": `module.exports = {};`,
		})
		if err := handler.publish.Publish(context.Background(), publish.Request{
			Kind:        "plugin_package",
			Name:        "delete-handler-demo",
			Version:     version,
			Channel:     "stable",
			ArtifactZip: zipPayload,
			Metadata: publish.Metadata{
				Title:   "Delete Handler Demo " + version,
				Summary: "Delete handler demo " + version,
			},
		}); err != nil {
			t.Fatalf("Publish(%s) returned error: %v", version, err)
		}
	}

	publishVersion("1.0.0")
	publishVersion("1.1.0")

	req := httptest.NewRequest(http.MethodDelete, "/admin/artifacts/plugin_package/delete-handler-demo/1.1.0", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	handler.HandleArtifactRoute(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected delete release 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal delete release payload returned error: %v", err)
	}
	result := payload["data"].(map[string]any)["result"].(map[string]any)
	if got := result["artifact_deleted"]; got != false {
		t.Fatalf("expected artifact_deleted=false, got %#v", got)
	}
	if got := result["latest_version"]; got != "1.0.0" {
		t.Fatalf("expected latest_version=1.0.0, got %#v", got)
	}

	versionsReq := httptest.NewRequest(http.MethodGet, "/admin/artifacts/plugin_package/delete-handler-demo", nil)
	versionsReq.Header.Set("Authorization", "Bearer "+token)
	versionsRecorder := httptest.NewRecorder()
	handler.GetArtifactVersions(versionsRecorder, versionsReq)
	if versionsRecorder.Code != http.StatusOK {
		t.Fatalf("expected versions 200 after delete, got %d with body %s", versionsRecorder.Code, versionsRecorder.Body.String())
	}
	var versionsPayload map[string]any
	if err := json.Unmarshal(versionsRecorder.Body.Bytes(), &versionsPayload); err != nil {
		t.Fatalf("Unmarshal versions payload returned error: %v", err)
	}
	versionData := versionsPayload["data"].(map[string]any)
	if got := versionData["latest_version"]; got != "1.0.0" {
		t.Fatalf("expected latest_version=1.0.0 after delete, got %#v", got)
	}
	releases := versionData["releases"].([]any)
	if len(releases) != 1 {
		t.Fatalf("expected one remaining release, got %#v", releases)
	}

	if _, err := handler.storage.Read(context.Background(), "artifacts/plugin_package/delete-handler-demo/1.1.0/manifest.json"); !os.IsNotExist(err) {
		t.Fatalf("expected deleted manifest removed, got err=%v", err)
	}
}

func TestDeleteArtifactEndpoint(t *testing.T) {
	handler, token := newTestHandler(t)

	zipPayload := buildAdminTestZip(t, map[string]string{
		"manifest.json": `{
  "name": "delete-artifact-demo",
  "display_name": "Delete Artifact Demo",
  "version": "3.0.0",
  "description": "admin delete artifact test",
  "runtime": "js_worker",
  "entry": "index.js"
}`,
		"index.js": `module.exports = {};`,
	})
	if err := handler.publish.Publish(context.Background(), publish.Request{
		Kind:        "plugin_package",
		Name:        "delete-artifact-demo",
		Version:     "3.0.0",
		Channel:     "stable",
		ArtifactZip: zipPayload,
		Metadata: publish.Metadata{
			Title:   "Delete Artifact Demo",
			Summary: "Delete artifact demo",
		},
	}); err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/admin/artifacts/plugin_package/delete-artifact-demo", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	handler.HandleArtifactRoute(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected delete artifact 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/admin/artifacts", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listRecorder := httptest.NewRecorder()
	handler.ListArtifacts(listRecorder, listReq)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("expected list 200, got %d with body %s", listRecorder.Code, listRecorder.Body.String())
	}
	var listPayload map[string]any
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &listPayload); err != nil {
		t.Fatalf("Unmarshal list payload returned error: %v", err)
	}
	data := listPayload["data"].(map[string]any)
	if pluginKind, ok := data["plugin_package"].(map[string]any); ok {
		if _, exists := pluginKind["delete-artifact-demo"]; exists {
			t.Fatalf("expected artifact to be absent from list after delete, got %#v", pluginKind["delete-artifact-demo"])
		}
	}

	if _, err := handler.storage.Read(context.Background(), "index/artifacts/plugin_package/delete-artifact-demo/index.json"); !os.IsNotExist(err) {
		t.Fatalf("expected artifact index removed, got err=%v", err)
	}
}

func TestAdminEndpointsRejectWrongMethod(t *testing.T) {
	handler, token := newTestHandler(t)

	cases := []struct {
		name string
		req  *http.Request
		run  func(http.ResponseWriter, *http.Request)
	}{
		{
			name: "login",
			req:  httptest.NewRequest(http.MethodGet, "/admin/auth/login", nil),
			run:  handler.Login,
		},
		{
			name: "publish",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/admin/publish", nil)
				req.Header.Set("Authorization", "Bearer "+token)
				return req
			}(),
			run: handler.Publish,
		},
		{
			name: "stats",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/admin/stats/overview", nil)
				req.Header.Set("Authorization", "Bearer "+token)
				return req
			}(),
			run: handler.GetStats,
		},
		{
			name: "artifacts",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/admin/artifacts", nil)
				req.Header.Set("Authorization", "Bearer "+token)
				return req
			}(),
			run: handler.ListArtifacts,
		},
		{
			name: "sync github release",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/admin/sync/github-release", nil)
				req.Header.Set("Authorization", "Bearer "+token)
				return req
			}(),
			run: handler.SyncGitHubRelease,
		},
		{
			name: "preview github release",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/admin/sync/github-release/release", nil)
				req.Header.Set("Authorization", "Bearer "+token)
				return req
			}(),
			run: handler.PreviewGitHubRelease,
		},
		{
			name: "inspect github release",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/admin/sync/github-release/inspect", nil)
				req.Header.Set("Authorization", "Bearer "+token)
				return req
			}(),
			run: handler.InspectGitHubRelease,
		},
		{
			name: "artifact versions",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/admin/artifacts/plugin_package/demo", nil)
				req.Header.Set("Authorization", "Bearer "+token)
				return req
			}(),
			run: handler.GetArtifactVersions,
		},
		{
			name: "artifact release",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/admin/artifacts/plugin_package/demo/1.0.0", nil)
				req.Header.Set("Authorization", "Bearer "+token)
				return req
			}(),
			run: handler.GetArtifactRelease,
		},
		{
			name: "artifact resync",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/admin/artifacts/plugin_package/demo/1.0.0/resync", nil)
				req.Header.Set("Authorization", "Bearer "+token)
				return req
			}(),
			run: handler.ResyncArtifactVersion,
		},
		{
			name: "artifact origin check",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/admin/artifacts/plugin_package/demo/1.0.0/check-origin", nil)
				req.Header.Set("Authorization", "Bearer "+token)
				return req
			}(),
			run: handler.CheckArtifactOrigin,
		},
		{
			name: "artifact origins batch check",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/admin/artifacts/plugin_package/demo/check-origin", nil)
				req.Header.Set("Authorization", "Bearer "+token)
				return req
			}(),
			run: handler.CheckArtifactOrigins,
		},
	}

	for _, tc := range cases {
		recorder := httptest.NewRecorder()
		tc.run(recorder, tc.req)
		if recorder.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s: expected 405, got %d with body %s", tc.name, recorder.Code, recorder.Body.String())
		}
	}
}

func newTestHandler(t *testing.T) (*Handler, string) {
	t.Helper()
	root := t.TempDir()

	store, err := storage.NewLocalStorage(filepath.Join(root, "data"), "http://localhost:18080")
	if err != nil {
		t.Fatalf("NewLocalStorage returned error: %v", err)
	}
	signSvc := signing.NewService(filepath.Join(root, "keys"))
	if _, err := signSvc.GenerateKeyPair("official-test"); err != nil {
		t.Fatalf("GenerateKeyPair returned error: %v", err)
	}
	authSvc := auth.NewServiceWithConfig(auth.Config{
		AdminUsername: "admin",
		AdminPassword: "password",
	})
	pubSvc := publish.NewService(store, signSvc, "official-test")

	token, err := authSvc.Login("admin", "password")
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}
	return NewHandler(authSvc, pubSvc, store, signSvc), token
}

func buildAdminTestZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	keys := make([]string, 0, len(files))
	for name := range files {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	for _, name := range keys {
		entry, err := writer.Create(strings.TrimSpace(name))
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
