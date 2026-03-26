package service

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"auralogic/internal/config"
)

func TestPluginHostMarketResolveDownloadInfoRejectsCrossOriginURL(t *testing.T) {
	source := PluginMarketSource{
		SourceID: "official",
		BaseURL:  "https://market.example.com",
	}
	release := map[string]interface{}{
		"download": map[string]interface{}{
			"url":    "https://evil.example.com/download/demo-market-1.2.0.zip",
			"sha256": "abc123",
		},
	}

	_, err := pluginHostMarketResolveDownloadInfo(source, "plugin_package", "demo-market", "1.2.0", release)
	if err == nil {
		t.Fatalf("expected cross-origin download url to be rejected")
	}
	if !strings.Contains(err.Error(), "configured market source origin") {
		t.Fatalf("expected same-origin validation error, got %v", err)
	}
}

func TestPluginHostMarketDownloadReleaseArtifactRejectsCrossOriginRedirect(t *testing.T) {
	packageBytes := buildPluginMarketTestZip(t, map[string]string{
		"manifest.json": `{"name":"demo-market","runtime":"js_worker","entry":"index.js","version":"1.2.0"}`,
		"index.js":      `module.exports = {};`,
	})

	redirectTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write(packageBytes)
	}))
	defer redirectTarget.Close()

	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/download/demo-market-1.2.0.zip" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, redirectTarget.URL+r.URL.RequestURI(), http.StatusFound)
	}))
	defer sourceServer.Close()

	_, err := pluginHostMarketDownloadReleaseArtifact(
		&config.Config{
			Plugin: config.PluginPlatformConfig{
				ArtifactDir: t.TempDir(),
			},
		},
		PluginMarketSource{
			SourceID: "official",
			BaseURL:  sourceServer.URL,
		},
		"plugin_package",
		"demo-market",
		"1.2.0",
		map[string]interface{}{
			"download": map[string]interface{}{
				"url":    sourceServer.URL + "/download/demo-market-1.2.0.zip",
				"size":   len(packageBytes),
				"sha256": computeSHA256Hex(packageBytes),
			},
		},
	)
	if err == nil {
		t.Fatalf("expected cross-origin download redirect to be rejected")
	}
	if !strings.Contains(err.Error(), "configured market source origin") {
		t.Fatalf("expected same-origin redirect error, got %v", err)
	}
}
