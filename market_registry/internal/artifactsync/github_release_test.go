package artifactsync

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"auralogic/market_registry/internal/catalog"
	"auralogic/market_registry/internal/publish"
	"auralogic/market_registry/internal/signing"
	"auralogic/market_registry/internal/storage"
	pubsync "auralogic/market_registry/pkg/artifactsync"
)

func TestGitHubReleaseSyncerSyncsIntoCanonicalStorage(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	store, err := storage.NewLocalStorage(filepath.Join(root, "data"), "https://registry.example.com")
	if err != nil {
		t.Fatalf("NewLocalStorage returned error: %v", err)
	}
	signSvc := signing.NewService(filepath.Join(root, "keys"))
	if _, err := signSvc.GenerateKeyPair("official-test"); err != nil {
		t.Fatalf("GenerateKeyPair returned error: %v", err)
	}
	pubSvc := publish.NewServiceWithOptions(store, signSvc, "official-test", publish.Options{
		SourceID:   "official",
		SourceName: "Registry Test Source",
		BaseURL:    "https://registry.example.com",
	})

	zipPayload := buildGitHubSyncZip(t, map[string]string{
		"manifest.json": `{
  "name": "github-demo",
  "display_name": "GitHub Demo",
  "version": "1.2.3",
  "description": "synced from github release",
  "runtime": "js_worker",
  "entry": "index.js",
  "manifest_version": "1.0.0",
  "protocol_version": "1.1.0",
  "min_host_protocol_version": "1.0.0",
  "min_host_bridge_version": "1.0.0"
}`,
		"index.js": `module.exports = {};`,
	})

	serverURL := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/auralogic/plugins/releases/tags/v1.2.3":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
  "id": 1001,
  "tag_name": "v1.2.3",
  "name": "GitHub Demo Release",
  "body": "GitHub release notes",
  "html_url": "https://github.example.com/auralogic/plugins/releases/tag/v1.2.3",
  "published_at": "2026-03-15T12:00:00Z",
  "author": { "login": "auralogic" },
  "assets": [
    {
      "id": 2002,
      "name": "github-demo-1.2.3.zip",
      "url": "` + serverURL + `/assets/2002",
      "browser_download_url": "` + serverURL + `/downloads/github-demo-1.2.3.zip",
      "size": ` + intToString(len(zipPayload)) + `,
      "content_type": "application/zip"
    }
  ]
}`))
		case "/downloads/github-demo-1.2.3.zip":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(zipPayload)
		case "/assets/2002":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(zipPayload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	syncer := NewGitHubReleaseSyncer(pubSvc)
	result, err := syncer.Sync(ctx, GitHubReleaseRequest{
		Name:       "github-demo",
		Version:    "1.2.3",
		Channel:    "stable",
		Owner:      "auralogic",
		Repo:       "plugins",
		Tag:        "v1.2.3",
		AssetName:  "github-demo-1.2.3.zip",
		APIBaseURL: server.URL,
		Metadata: publish.Metadata{
			Title:   "GitHub Demo",
			Summary: "Synced release",
		},
	})
	if err != nil {
		t.Fatalf("Sync returned error: %v", err)
	}
	if result.ReleaseID != 1001 || result.AssetID != 2002 {
		t.Fatalf("unexpected sync result ids: %+v", result)
	}
	if result.SHA256 == "" {
		t.Fatalf("expected sync result sha256, got %+v", result)
	}

	manifestBody, err := store.Read(ctx, "artifacts/plugin_package/github-demo/1.2.3/manifest.json")
	if err != nil {
		t.Fatalf("Read manifest returned error: %v", err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(manifestBody, &manifest); err != nil {
		t.Fatalf("Unmarshal manifest returned error: %v", err)
	}
	download := manifest["download"].(map[string]any)
	transport := download["transport"].(map[string]any)
	if got := transport["provider"]; got != "github_release" {
		t.Fatalf("expected github_release provider, got %#v", got)
	}
	if got := manifest["release_notes"]; got != "GitHub release notes" {
		t.Fatalf("expected github release notes fallback, got %#v", got)
	}

	originBody, err := store.Read(ctx, "artifacts/plugin_package/github-demo/1.2.3/origin.json")
	if err != nil {
		t.Fatalf("Read origin returned error: %v", err)
	}
	var origin map[string]any
	if err := json.Unmarshal(originBody, &origin); err != nil {
		t.Fatalf("Unmarshal origin returned error: %v", err)
	}
	if got := origin["provider"]; got != "github_release" {
		t.Fatalf("expected github_release origin provider, got %#v", got)
	}
	locator := origin["locator"].(map[string]any)
	if got := locator["owner"]; got != "auralogic" {
		t.Fatalf("expected owner auralogic, got %#v", got)
	}

	runtimeStore := catalog.NewRuntimeStore(store, signSvc, catalog.RuntimeStoreConfig{
		SourceID:   "official",
		SourceName: "Registry Test Source",
		KeyID:      "official-test",
		Now: func() time.Time {
			return time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
		},
	})
	releaseDoc, err := runtimeStore.BuildReleaseDocument(ctx, "plugin_package", "github-demo", "1.2.3", "https://registry.example.com")
	if err != nil {
		t.Fatalf("BuildReleaseDocument returned error: %v", err)
	}
	docTransport := releaseDoc["download"].(map[string]any)["transport"].(map[string]any)
	if got := docTransport["provider"]; got != "github_release" {
		t.Fatalf("expected github_release document transport, got %#v", got)
	}
	downloaded, contentType, err := runtimeStore.ReadReleaseArtifact(ctx, "plugin_package", "github-demo", "1.2.3")
	if err != nil {
		t.Fatalf("ReadReleaseArtifact returned error: %v", err)
	}
	if contentType != "application/zip" {
		t.Fatalf("expected application/zip content type, got %q", contentType)
	}
	if !bytes.Equal(downloaded, zipPayload) {
		t.Fatal("expected downloaded payload to match synced canonical artifact")
	}
}

func TestGitHubReleaseSyncerInspectDetectsDrift(t *testing.T) {
	ctx := context.Background()

	zipPayload := buildGitHubSyncZip(t, map[string]string{
		"manifest.json": `{
  "name": "github-demo",
  "display_name": "GitHub Demo",
  "version": "1.2.3",
  "description": "synced from github release",
  "runtime": "js_worker",
  "entry": "index.js"
}`,
		"index.js": `module.exports = { version: "remote" };`,
	})

	serverURL := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/auralogic/plugins/releases/tags/v1.2.3":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
  "id": 1001,
  "tag_name": "v1.2.3",
  "name": "GitHub Demo Release",
  "body": "GitHub release notes",
  "html_url": "https://github.example.com/auralogic/plugins/releases/tag/v1.2.3",
  "published_at": "2026-03-15T12:00:00Z",
  "author": { "login": "auralogic" },
  "assets": [
    {
      "id": 2002,
      "name": "github-demo-1.2.3.zip",
      "url": "` + serverURL + `/assets/2002",
      "browser_download_url": "` + serverURL + `/downloads/github-demo-1.2.3.zip",
      "size": ` + intToString(len(zipPayload)) + `,
      "content_type": "application/zip",
      "updated_at": "2026-03-16T08:00:00Z"
    }
  ]
}`))
		case "/downloads/github-demo-1.2.3.zip", "/assets/2002":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(zipPayload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	syncer := NewGitHubReleaseSyncer(nil)
	result, err := syncer.Inspect(ctx, GitHubReleaseInspectionRequest{
		Kind:           "plugin_package",
		Name:           "github-demo",
		Version:        "1.2.3",
		Owner:          "auralogic",
		Repo:           "plugins",
		Tag:            "v1.2.3",
		AssetName:      "github-demo-1.2.3.zip",
		APIBaseURL:     server.URL,
		ExpectedSHA256: "deadbeef",
		ExpectedSize:   1,
	})
	if err != nil {
		t.Fatalf("Inspect returned error: %v", err)
	}
	if !result.Changed {
		t.Fatalf("expected inspection drift, got %+v", result)
	}
	if len(result.ChangedFields) == 0 {
		t.Fatalf("expected changed fields, got %+v", result)
	}
	if result.AssetUpdatedAt != "2026-03-16T08:00:00Z" {
		t.Fatalf("expected asset updated at to be preserved, got %+v", result)
	}
}

func TestGitHubReleaseSyncerRejectsInvalidAPIBaseURL(t *testing.T) {
	syncer := NewGitHubReleaseSyncer(nil)
	_, err := syncer.Inspect(context.Background(), GitHubReleaseInspectionRequest{
		Owner:      "auralogic",
		Repo:       "plugins",
		Tag:        "v1.2.3",
		AssetName:  "demo.zip",
		APIBaseURL: "ftp://github.example.com/api/v3",
	})
	if err == nil {
		t.Fatal("expected invalid api base url to be rejected")
	}
	if !pubsync.IsRequestError(err) {
		t.Fatalf("expected request error, got %T", err)
	}
	if !strings.Contains(err.Error(), "github api base url must use http or https") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestGitHubReleaseSyncerPreviewListsAssets(t *testing.T) {
	serverURL := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/auralogic/plugins/releases/tags/v1.2.3":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
  "id": 1001,
  "tag_name": "v1.2.3",
  "name": "GitHub Demo Release",
  "body": "GitHub release notes",
  "html_url": "https://github.example.com/auralogic/plugins/releases/tag/v1.2.3",
  "published_at": "2026-03-15T12:00:00Z",
  "author": { "login": "auralogic" },
  "assets": [
    {
      "id": 2003,
      "name": "b-demo-1.2.3.zip",
      "url": "` + serverURL + `/assets/2003",
      "browser_download_url": "` + serverURL + `/downloads/b-demo-1.2.3.zip",
      "size": 22,
      "content_type": "application/zip",
      "updated_at": "2026-03-16T08:00:00Z"
    },
    {
      "id": 2002,
      "name": "a-demo-1.2.3.zip",
      "url": "` + serverURL + `/assets/2002",
      "browser_download_url": "` + serverURL + `/downloads/a-demo-1.2.3.zip",
      "size": 11,
      "content_type": "application/zip",
      "updated_at": "2026-03-15T08:00:00Z"
    }
  ]
}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	syncer := NewGitHubReleaseSyncer(nil)
	result, err := syncer.Preview(context.Background(), GitHubReleasePreviewRequest{
		Owner:      "auralogic",
		Repo:       "plugins",
		Tag:        "v1.2.3",
		AssetName:  "b-demo-1.2.3.zip",
		APIBaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("Preview returned error: %v", err)
	}
	if result.ReleaseID != 1001 {
		t.Fatalf("expected release id 1001, got %+v", result)
	}
	if result.AssetCount != 2 {
		t.Fatalf("expected asset count 2, got %+v", result)
	}
	if result.SelectedAsset != "b-demo-1.2.3.zip" {
		t.Fatalf("expected selected asset b-demo-1.2.3.zip, got %+v", result)
	}
	if len(result.Assets) != 2 {
		t.Fatalf("expected two assets, got %+v", result)
	}
	if result.Assets[0].Name != "a-demo-1.2.3.zip" || result.Assets[1].Name != "b-demo-1.2.3.zip" {
		t.Fatalf("expected assets to be sorted by name, got %+v", result.Assets)
	}
	if !result.Assets[1].Selected {
		t.Fatalf("expected selected asset to be marked, got %+v", result.Assets)
	}
}

func TestGitHubReleaseSyncerPreviewAutoSelectsSingleZipAsset(t *testing.T) {
	serverURL := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/auralogic/plugins/releases/tags/v1.2.3":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
  "id": 1001,
  "tag_name": "v1.2.3",
  "name": "GitHub Demo Release",
  "body": "GitHub release notes",
  "html_url": "https://github.example.com/auralogic/plugins/releases/tag/v1.2.3",
  "published_at": "2026-03-15T12:00:00Z",
  "author": { "login": "auralogic" },
  "assets": [
    {
      "id": 2002,
      "name": "demo-1.2.3.zip",
      "url": "` + serverURL + `/assets/2002",
      "browser_download_url": "` + serverURL + `/downloads/demo-1.2.3.zip",
      "size": 11,
      "content_type": "application/zip",
      "updated_at": "2026-03-15T08:00:00Z"
    },
    {
      "id": 2003,
      "name": "demo-1.2.3.sha256",
      "url": "` + serverURL + `/assets/2003",
      "browser_download_url": "` + serverURL + `/downloads/demo-1.2.3.sha256",
      "size": 64,
      "content_type": "text/plain",
      "updated_at": "2026-03-16T08:00:00Z"
    }
  ]
}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	syncer := NewGitHubReleaseSyncer(nil)
	result, err := syncer.Preview(context.Background(), GitHubReleasePreviewRequest{
		Owner:      "auralogic",
		Repo:       "plugins",
		Tag:        "v1.2.3",
		APIBaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("Preview returned error: %v", err)
	}
	if result.SelectedAsset != "demo-1.2.3.zip" {
		t.Fatalf("expected auto-selected zip asset, got %+v", result)
	}
	if len(result.Assets) == 0 || result.Assets[0].Name != "demo-1.2.3.zip" || !result.Assets[0].Selected {
		t.Fatalf("expected zip asset to be sorted first and selected, got %+v", result.Assets)
	}
}

func buildGitHubSyncZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, name := range sortedKeys(files) {
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

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func intToString(value int) string {
	return strings.TrimSpace(strconv.Itoa(value))
}
