package adminui

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestHandlerServesIndexForRootAndSPARoutes(t *testing.T) {
	t.Parallel()

	handler := NewHandlerWithFS("/admin/ui", fstest.MapFS{
		"index.html":          {Data: []byte("<html>registry admin</html>")},
		"static/js/main.js":   {Data: []byte("console.log('ok');")},
		"static/css/main.css": {Data: []byte("body{}")},
	})

	testCases := []struct {
		name string
		path string
	}{
		{name: "root", path: "/admin/ui/"},
		{name: "spa route", path: "/admin/ui/dashboard"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.Header.Set("Accept", "text/html")

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)

			if recorder.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d", recorder.Code)
			}
			if !strings.Contains(recorder.Body.String(), "registry admin") {
				t.Fatalf("expected index.html response, got %q", recorder.Body.String())
			}
		})
	}
}

func TestHandlerServesStaticAssetsAndRejectsMissingFiles(t *testing.T) {
	t.Parallel()

	handler := NewHandlerWithFS("/admin/ui", fstest.MapFS{
		"index.html":        {Data: []byte("<html>registry admin</html>")},
		"static/js/main.js": {Data: []byte("console.log('ok');")},
	})

	assetReq := httptest.NewRequest(http.MethodGet, "/admin/ui/static/js/main.js", nil)
	assetRecorder := httptest.NewRecorder()
	handler.ServeHTTP(assetRecorder, assetReq)

	if assetRecorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", assetRecorder.Code)
	}
	if !strings.Contains(assetRecorder.Body.String(), "console.log") {
		t.Fatalf("expected js bundle response, got %q", assetRecorder.Body.String())
	}

	missingReq := httptest.NewRequest(http.MethodGet, "/admin/ui/static/js/missing.js", nil)
	missingRecorder := httptest.NewRecorder()
	handler.ServeHTTP(missingRecorder, missingReq)

	if missingRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", missingRecorder.Code)
	}
}

func TestNormalizeAssetPathStaysInsideBundleRoot(t *testing.T) {
	t.Parallel()

	if got := normalizeAssetPath("/../static/js/main.js"); got != "static/js/main.js" {
		t.Fatalf("unexpected normalized asset path: %q", got)
	}
}

func TestEmbeddedDistFSContainsIndexHTML(t *testing.T) {
	t.Parallel()

	if _, err := fs.Stat(embeddedDistFS, "index.html"); err != nil {
		t.Fatalf("expected embedded dist to contain index.html: %v", err)
	}
}
