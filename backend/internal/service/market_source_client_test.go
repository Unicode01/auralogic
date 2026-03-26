package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPluginMarketSourceClientRejectsCrossOriginRedirect(t *testing.T) {
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer redirectTarget.Close()

	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/artifacts/plugin_package/demo-market/releases/1.2.0" {
			t.Fatalf("unexpected request path %q", r.URL.Path)
		}
		http.Redirect(w, r, redirectTarget.URL+r.URL.RequestURI(), http.StatusFound)
	}))
	defer sourceServer.Close()

	client := newPluginMarketSourceClient()
	_, err := client.FetchRelease(context.Background(), PluginMarketSource{
		SourceID: "official",
		BaseURL:  sourceServer.URL,
	}, "plugin_package", "demo-market", "1.2.0")
	if err == nil {
		t.Fatalf("expected cross-origin redirect to be rejected")
	}
	if !strings.Contains(err.Error(), "configured market source origin") {
		t.Fatalf("expected same-origin redirect error, got %v", err)
	}
}
