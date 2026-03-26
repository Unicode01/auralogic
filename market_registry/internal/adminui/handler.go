package adminui

import (
	"bytes"
	"io/fs"
	"net/http"
	pathpkg "path"
	"strings"
	"time"
)

type handler struct {
	basePath   string
	files      fs.FS
	fileServer http.Handler
}

func NewHandler(basePath string) http.Handler {
	return NewHandlerWithFS(basePath, embeddedDistFS)
}

func NewHandlerWithFS(basePath string, files fs.FS) http.Handler {
	normalizedBasePath := normalizeBasePath(basePath)
	return &handler{
		basePath:   normalizedBasePath,
		files:      files,
		fileServer: http.FileServer(http.FS(files)),
	}
}

func RedirectHandler(targetPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, targetPath, http.StatusTemporaryRedirect)
	}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, h.basePath) {
		http.NotFound(w, r)
		return
	}

	requestPath := strings.TrimPrefix(r.URL.Path, h.basePath)
	cleanedPath := normalizeAssetPath(requestPath)
	if cleanedPath == "" {
		h.serveIndex(w, r)
		return
	}

	if fileExists(h.files, cleanedPath) {
		h.serveAsset(w, r, cleanedPath)
		return
	}

	if pathpkg.Ext(cleanedPath) != "" {
		http.NotFound(w, r)
		return
	}

	if acceptsHTML(r) {
		h.serveIndex(w, r)
		return
	}

	http.NotFound(w, r)
}

func (h *handler) serveIndex(w http.ResponseWriter, r *http.Request) {
	data, err := fs.ReadFile(h.files, "index.html")
	if err != nil {
		http.Error(w, "embedded admin ui index not found", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(data))
}

func (h *handler) serveAsset(w http.ResponseWriter, r *http.Request, assetPath string) {
	request := r.Clone(r.Context())
	request.URL.Path = "/" + strings.TrimPrefix(assetPath, "/")
	h.fileServer.ServeHTTP(w, request)
}

func normalizeBasePath(basePath string) string {
	trimmed := strings.TrimSpace(basePath)
	if trimmed == "" || trimmed == "/" {
		return ""
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	return strings.TrimRight(trimmed, "/")
}

func normalizeAssetPath(assetPath string) string {
	cleaned := pathpkg.Clean("/" + assetPath)
	trimmed := strings.TrimPrefix(cleaned, "/")
	if trimmed == "." {
		return ""
	}
	return trimmed
}

func fileExists(filesystem fs.FS, assetPath string) bool {
	info, err := fs.Stat(filesystem, assetPath)
	return err == nil && !info.IsDir()
}

func acceptsHTML(r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	accept := strings.ToLower(strings.TrimSpace(r.Header.Get("Accept")))
	return accept == "" || strings.Contains(accept, "text/html")
}
