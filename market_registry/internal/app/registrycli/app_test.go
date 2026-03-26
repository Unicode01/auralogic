package registrycli

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"auralogic/market_registry/internal/signing"
)

func TestConfigCommandPrintsResolvedRuntime(t *testing.T) {
	t.Setenv("MARKET_REGISTRY_DATA_DIR", "registry-data")
	t.Setenv("MARKET_REGISTRY_CHANNEL", "beta")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := App{Stdout: &stdout, Stderr: &stderr}

	code := app.Run([]string{"market-registry-cli", "config"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "Implementation module: auralogic/market_registry") {
		t.Fatalf("expected config output to contain implementation module, got %q", output)
	}
	if !strings.Contains(output, "Shell module: auralogic/market_registry") {
		t.Fatalf("expected config output to contain shell module, got %q", output)
	}
	if !strings.Contains(output, "Command binding:") {
		t.Fatalf("expected config output to contain command binding, got %q", output)
	}
	if !strings.Contains(output, "data_dir: registry-data") {
		t.Fatalf("expected resolved data_dir in output, got %q", output)
	}
	if !strings.Contains(output, "MARKET_REGISTRY_CHANNEL (legacy: CHANNEL)") {
		t.Fatalf("expected alias listing in output, got %q", output)
	}
	if !strings.Contains(output, "./cmd/market-registry-cli") {
		t.Fatalf("expected filesystem binding in output, got %q", output)
	}
}

func TestConfigCommandSupportsJSON(t *testing.T) {
	t.Setenv("MARKET_REGISTRY_BASE_URL", "https://registry.example.com")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := App{Stdout: &stdout, Stderr: &stderr}

	code := app.Run([]string{"market-registry-cli", "config", "--json"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("expected JSON config payload, got error: %v", err)
	}
	modules := payload["modules"].(map[string]any)
	if got := modules["shell"]; got != "auralogic/market_registry" {
		t.Fatalf("expected shell module in JSON payload, got %#v", got)
	}

	runtimeData := payload["runtime"].(map[string]any)
	if got := runtimeData["base_url"]; got != "https://registry.example.com" {
		t.Fatalf("expected base_url in JSON payload, got %#v", got)
	}
}

func TestConfigCommandSupportsAPIScopeWithoutLeakingPassword(t *testing.T) {
	t.Setenv("MARKET_REGISTRY_ADMIN_USERNAME", "registry-admin")
	t.Setenv("MARKET_REGISTRY_ADMIN_PASSWORD", "super-secret")
	t.Setenv("MARKET_REGISTRY_AUTH_TOKEN_TTL", "30m")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := App{Stdout: &stdout, Stderr: &stderr}

	code := app.Run([]string{"market-registry-cli", "config", "--api", "--json"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("expected JSON config payload, got error: %v", err)
	}
	if got := payload["scope"]; got != "api" {
		t.Fatalf("expected scope api, got %#v", got)
	}

	runtimeData := payload["runtime"].(map[string]any)
	if got := runtimeData["admin_username"]; got != "registry-admin" {
		t.Fatalf("expected admin_username in JSON payload, got %#v", got)
	}
	if got := runtimeData["admin_password_set"]; got != true {
		t.Fatalf("expected admin_password_set true, got %#v", got)
	}
	if _, exists := runtimeData["admin_password"]; exists {
		t.Fatalf("did not expect admin_password plaintext in payload: %#v", runtimeData)
	}
	if got := runtimeData["token_ttl"]; got != "30m0s" {
		t.Fatalf("expected token_ttl 30m0s, got %#v", got)
	}
}

func TestConfigCommandSupportsSharedScope(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := App{Stdout: &stdout, Stderr: &stderr}

	code := app.Run([]string{"market-registry-cli", "config", "--shared"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "Scope: shared") {
		t.Fatalf("expected shared scope in output, got %q", output)
	}
	if !strings.Contains(output, "Filesystem bindings:") {
		t.Fatalf("expected shared filesystem bindings in output, got %q", output)
	}
	if !strings.Contains(output, "./cmd/market-registry-api") {
		t.Fatalf("expected API filesystem path in shared output, got %q", output)
	}
}

func TestAuditCommandSupportsJSON(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := App{Stdout: &stdout, Stderr: &stderr}

	code := app.Run([]string{"market-registry-cli", "audit", "--json"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("expected JSON audit payload, got error: %v", err)
	}
	if got := payload["implementation_module"]; got != "auralogic/market_registry" {
		t.Fatalf("expected implementation module, got %#v", got)
	}
	if got := payload["shell_dir_present"]; got != true {
		t.Fatalf("expected shell_dir_present true, got %#v", got)
	}
	moduleReadiness := payload["module_readiness"].(map[string]any)
	if got := moduleReadiness["ready_for_module_rename"]; got != true {
		t.Fatalf("expected module rename readiness true, got %#v", got)
	}
	if got := moduleReadiness["entrypoints_using_parent_internal"]; got != float64(0) {
		t.Fatalf("expected 0 entrypoints using parent internal packages, got %#v", got)
	}
	if got := moduleReadiness["bootstrap_files_using_parent_internal"]; got != float64(0) {
		t.Fatalf("expected 0 bootstrap files using parent internal packages, got %#v", got)
	}
	if got := moduleReadiness["runtime_facade_files_using_parent_internal"]; got != float64(0) {
		t.Fatalf("expected 0 runtime facade files using parent internal packages, got %#v", got)
	}
	if got := moduleReadiness["service_layer_files_using_parent_internal"]; got != float64(0) {
		t.Fatalf("expected 0 service layer files using parent internal packages, got %#v", got)
	}
	pendingRefs := payload["pending_legacy_refs"].([]any)
	for _, item := range pendingRefs {
		entry := item.(map[string]any)
		if got := entry["count"]; got != float64(0) {
			t.Fatalf("expected zero pending legacy refs, got %#v", pendingRefs)
		}
	}
	canonicalRefs := payload["canonical_command_refs"].([]any)
	if len(canonicalRefs) == 0 {
		t.Fatalf("expected canonical command refs in audit payload, got %#v", payload)
	}
}

func TestAuditCommandPrintsSummary(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := App{Stdout: &stdout, Stderr: &stderr}

	code := app.Run([]string{"market-registry-cli", "audit"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "Legacy command references:") {
		t.Fatalf("expected audit summary in output, got %q", output)
	}
	if !strings.Contains(output, "Pending legacy references:") {
		t.Fatalf("expected pending legacy section in output, got %q", output)
	}
	if !strings.Contains(output, "Ready for true module rename: true") {
		t.Fatalf("expected module readiness summary in output, got %q", output)
	}
	if strings.Contains(output, "Service layer files using parent internal packages:") {
		t.Fatalf("did not expect remaining service layer dependency summary in output, got %q", output)
	}
	if !strings.Contains(output, "./cmd/market-registry-api") {
		t.Fatalf("expected shell filesystem token in output, got %q", output)
	}
}

func TestAuditCommandStrictPassesWhenNoPendingRefsRemain(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := App{Stdout: &stdout, Stderr: &stderr}

	code := app.Run([]string{"market-registry-cli", "audit", "--strict"})
	if code != 0 {
		t.Fatalf("expected strict audit to pass, got %d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
}

func TestSyncGitHubReleaseCommand(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	keyDir := filepath.Join(root, "keys")
	signSvc := signing.NewService(keyDir)
	if _, err := signSvc.GenerateKeyPair("official-test"); err != nil {
		t.Fatalf("GenerateKeyPair returned error: %v", err)
	}

	t.Setenv("MARKET_REGISTRY_DATA_DIR", dataDir)
	t.Setenv("MARKET_REGISTRY_KEY_DIR", keyDir)
	t.Setenv("MARKET_REGISTRY_KEY_ID", "official-test")
	t.Setenv("MARKET_REGISTRY_BASE_URL", "https://registry.example.com")

	zipPayload := buildCLISyncZip(t, map[string]string{
		"manifest.json": `{
  "name": "cli-sync-demo",
  "display_name": "CLI Sync Demo",
  "version": "1.0.0",
  "description": "cli sync demo",
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
		case "/repos/auralogic/plugins/releases/tags/v1.0.0":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
  "id": 10,
  "tag_name": "v1.0.0",
  "name": "CLI Sync Demo",
  "body": "CLI synced release",
  "author": { "login": "auralogic" },
  "assets": [
    {
      "id": 20,
      "name": "cli-sync-demo-1.0.0.zip",
      "url": "` + serverURL + `/assets/20",
      "browser_download_url": "` + serverURL + `/downloads/cli-sync-demo-1.0.0.zip",
      "size": ` + intToStringCLI(len(zipPayload)) + `,
      "content_type": "application/zip"
    }
  ]
}`))
		case "/downloads/cli-sync-demo-1.0.0.zip":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(zipPayload)
		case "/assets/20":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(zipPayload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := App{Stdout: &stdout, Stderr: &stderr}
	code := app.Run([]string{
		"market-registry-cli",
		"sync",
		"github-release",
		"--kind", "plugin_package",
		"--name", "cli-sync-demo",
		"--version", "1.0.0",
		"--owner", "auralogic",
		"--repo", "plugins",
		"--tag", "v1.0.0",
		"--asset", "cli-sync-demo-1.0.0.zip",
		"--api-base-url", server.URL,
	})
	if code != 0 {
		t.Fatalf("expected sync command to pass, got %d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "Successfully synced GitHub release auralogic/plugins@v1.0.0 asset cli-sync-demo-1.0.0.zip") {
		t.Fatalf("expected sync success output, got %q", output)
	}
	if !strings.Contains(output, "Registry snapshots rebuilt:") {
		t.Fatalf("expected registry rebuild output, got %q", output)
	}
}

func TestSyncGitHubReleaseCommandAutofillsFromLocalManifest(t *testing.T) {
	root := t.TempDir()
	projectDir := filepath.Join(root, "project")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "manifest.json"), []byte(`{
  "name": "cli-sync-demo",
  "display_name": "Local Manifest Title",
  "version": "1.0.0",
  "description": "local manifest description",
  "runtime": "js_worker",
  "entry": "index.js"
}`), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	t.Chdir(projectDir)

	dataDir := filepath.Join(root, "data")
	keyDir := filepath.Join(root, "keys")
	signSvc := signing.NewService(keyDir)
	if _, err := signSvc.GenerateKeyPair("official-test"); err != nil {
		t.Fatalf("GenerateKeyPair returned error: %v", err)
	}

	t.Setenv("MARKET_REGISTRY_DATA_DIR", dataDir)
	t.Setenv("MARKET_REGISTRY_KEY_DIR", keyDir)
	t.Setenv("MARKET_REGISTRY_KEY_ID", "official-test")
	t.Setenv("MARKET_REGISTRY_BASE_URL", "https://registry.example.com")

	zipPayload := buildCLISyncZip(t, map[string]string{
		"manifest.json": `{
  "name": "cli-sync-demo",
  "display_name": "Remote Package Title",
  "version": "1.0.0",
  "description": "remote package description",
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
		case "/repos/auralogic/plugins/releases/tags/v1.0.0":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
  "id": 10,
  "tag_name": "v1.0.0",
  "name": "GitHub Release Title",
  "body": "CLI synced release",
  "author": { "login": "auralogic" },
  "assets": [
    {
      "id": 20,
      "name": "cli-sync-demo-1.0.0.zip",
      "url": "` + serverURL + `/assets/20",
      "browser_download_url": "` + serverURL + `/downloads/cli-sync-demo-1.0.0.zip",
      "size": ` + intToStringCLI(len(zipPayload)) + `,
      "content_type": "application/zip"
    }
  ]
}`))
		case "/downloads/cli-sync-demo-1.0.0.zip":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(zipPayload)
		case "/assets/20":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(zipPayload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := App{Stdout: &stdout, Stderr: &stderr}
	code := app.Run([]string{
		"market-registry-cli",
		"sync",
		"github-release",
		"--owner", "auralogic",
		"--repo", "plugins",
		"--tag", "v1.0.0",
		"--asset", "cli-sync-demo-1.0.0.zip",
		"--api-base-url", server.URL,
	})
	if code != 0 {
		t.Fatalf("expected sync command to pass, got %d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "Published as plugin_package:cli-sync-demo:1.0.0") {
		t.Fatalf("expected sync command to autofill coordinates, got %q", output)
	}

	manifestBody, err := os.ReadFile(filepath.Join(dataDir, "artifacts", "plugin_package", "cli-sync-demo", "1.0.0", "manifest.json"))
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(manifestBody, &manifest); err != nil {
		t.Fatalf("Unmarshal manifest returned error: %v", err)
	}
	if got := manifest["title"]; got != "Local Manifest Title" {
		t.Fatalf("expected local manifest title to win, got %#v", got)
	}
}

func TestPullCommandDownloadsLatestRelease(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	zipPayload := buildCLISyncZip(t, map[string]string{
		"manifest.json": `{
  "name": "pull-demo",
  "display_name": "Pull Demo",
  "version": "1.2.3",
  "description": "pull demo",
  "runtime": "js_worker",
  "entry": "index.js"
}`,
		"index.js": `module.exports = {};`,
	})
	sum := sha256.Sum256(zipPayload)
	sha256Hex := hex.EncodeToString(sum[:])

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/artifacts/plugin_package/pull-demo":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
  "success": true,
  "data": {
    "kind": "plugin_package",
    "name": "pull-demo",
    "latest_version": "1.2.3"
  }
}`))
		case "/v1/artifacts/plugin_package/pull-demo/releases/1.2.3":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
  "success": true,
  "data": {
    "kind": "plugin_package",
    "name": "pull-demo",
    "version": "1.2.3",
    "download": {
      "url": "/v1/artifacts/plugin_package/pull-demo/releases/1.2.3/download",
      "size": ` + intToStringCLI(len(zipPayload)) + `,
      "content_type": "application/zip",
      "sha256": "` + sha256Hex + `"
    }
  }
}`))
		case "/v1/artifacts/plugin_package/pull-demo/releases/1.2.3/download":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(zipPayload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	t.Setenv("MARKET_REGISTRY_BASE_URL", server.URL)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := App{Stdout: &stdout, Stderr: &stderr}
	code := app.Run([]string{
		"market-registry-cli",
		"pull",
		"--kind", "plugin_package",
		"--name", "pull-demo",
	})
	if code != 0 {
		t.Fatalf("expected pull command to pass, got %d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "Resolved stable version: 1.2.3") {
		t.Fatalf("expected latest version resolution in output, got %q", output)
	}
	if !strings.Contains(output, "Pulled plugin_package:pull-demo:1.2.3") {
		t.Fatalf("expected pull success output, got %q", output)
	}
	savedPath := filepath.Join(root, "pull-demo-1.2.3.zip")
	savedPayload, err := os.ReadFile(savedPath)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if !bytes.Equal(savedPayload, zipPayload) {
		t.Fatal("expected pulled artifact payload to match source payload")
	}
}

func TestPullCommandSkipsExistingMatchingArtifact(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	zipPayload := buildCLISyncZip(t, map[string]string{
		"manifest.json": `{
  "name": "pull-demo",
  "display_name": "Pull Demo",
  "version": "1.2.3",
  "description": "pull demo",
  "runtime": "js_worker",
  "entry": "index.js"
}`,
		"index.js": `module.exports = {};`,
	})
	sum := sha256.Sum256(zipPayload)
	sha256Hex := hex.EncodeToString(sum[:])
	outputPath := filepath.Join(root, "pull-demo-1.2.3.zip")
	if err := os.WriteFile(outputPath, zipPayload, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	downloadHits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/artifacts/plugin_package/pull-demo/releases/1.2.3":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
  "success": true,
  "data": {
    "kind": "plugin_package",
    "name": "pull-demo",
    "version": "1.2.3",
    "download": {
      "url": "/v1/artifacts/plugin_package/pull-demo/releases/1.2.3/download",
      "size": ` + intToStringCLI(len(zipPayload)) + `,
      "content_type": "application/zip",
      "sha256": "` + sha256Hex + `"
    }
  }
}`))
		case "/v1/artifacts/plugin_package/pull-demo/releases/1.2.3/download":
			downloadHits++
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(zipPayload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	t.Setenv("MARKET_REGISTRY_BASE_URL", server.URL)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := App{Stdout: &stdout, Stderr: &stderr}
	code := app.Run([]string{
		"market-registry-cli",
		"pull",
		"--kind", "plugin_package",
		"--name", "pull-demo",
		"--version", "1.2.3",
	})
	if code != 0 {
		t.Fatalf("expected pull command to pass, got %d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if downloadHits != 0 {
		t.Fatalf("expected existing matching artifact to skip download, got %d download hits", downloadHits)
	}
	if !strings.Contains(stdout.String(), "Artifact already up to date:") {
		t.Fatalf("expected up-to-date output, got %q", stdout.String())
	}
}

func TestPullCommandResolvesVersionByChannel(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	zipPayload := buildCLISyncZip(t, map[string]string{
		"manifest.json": `{
  "name": "pull-demo",
  "display_name": "Pull Demo",
  "version": "2.0.0-beta.1",
  "description": "pull demo beta",
  "runtime": "js_worker",
  "entry": "index.js"
}`,
		"index.js": `module.exports = {};`,
	})
	sum := sha256.Sum256(zipPayload)
	sha256Hex := hex.EncodeToString(sum[:])

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/artifacts/plugin_package/pull-demo":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
  "success": true,
  "data": {
    "kind": "plugin_package",
    "name": "pull-demo",
    "latest_version": "1.0.0",
    "versions": [
      { "version": "2.0.0-beta.1", "channel": "beta" },
      { "version": "1.0.0", "channel": "stable" }
    ]
  }
}`))
		case "/v1/artifacts/plugin_package/pull-demo/releases/2.0.0-beta.1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
  "success": true,
  "data": {
    "kind": "plugin_package",
    "name": "pull-demo",
    "version": "2.0.0-beta.1",
    "download": {
      "url": "/v1/artifacts/plugin_package/pull-demo/releases/2.0.0-beta.1/download",
      "size": ` + intToStringCLI(len(zipPayload)) + `,
      "content_type": "application/zip",
      "sha256": "` + sha256Hex + `"
    }
  }
}`))
		case "/v1/artifacts/plugin_package/pull-demo/releases/2.0.0-beta.1/download":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(zipPayload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	t.Setenv("MARKET_REGISTRY_BASE_URL", server.URL)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := App{Stdout: &stdout, Stderr: &stderr}
	code := app.Run([]string{
		"market-registry-cli",
		"pull",
		"--kind", "plugin_package",
		"--name", "pull-demo",
		"--channel", "beta",
	})
	if code != 0 {
		t.Fatalf("expected pull command to pass, got %d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "Resolved beta version: 2.0.0-beta.1") {
		t.Fatalf("expected beta version resolution output, got %q", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(root, "pull-demo-2.0.0-beta.1.zip")); err != nil {
		t.Fatalf("expected beta artifact to be downloaded, got %v", err)
	}
}

func buildCLISyncZip(t *testing.T, files map[string]string) []byte {
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

func intToStringCLI(value int) string {
	return strings.TrimSpace(strconv.Itoa(value))
}
