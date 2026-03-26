package sourceapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"auralogic/market_registry/pkg/analytics"
	"auralogic/market_registry/pkg/catalog"
	"auralogic/market_registry/pkg/publish"
)

type Store interface {
	BuildSourceDocument(ctx context.Context, baseURL string) (map[string]any, error)
	ListCatalog(ctx context.Context, query catalog.Query, baseURL string) ([]map[string]any, int, error)
	BuildArtifactDocument(ctx context.Context, kind string, name string) (map[string]any, error)
	BuildReleaseDocument(ctx context.Context, kind string, name string, version string, baseURL string) (map[string]any, error)
	ReadReleaseArtifact(ctx context.Context, kind string, name string, version string) ([]byte, string, error)
}

type ReleaseDownloadRedirectResolver interface {
	ResolveReleaseDownloadRedirect(ctx context.Context, kind string, name string, version string) (string, error)
}

type RegistryStatusProvider interface {
	RegistryStatus(ctx context.Context) (publish.RegistryStatus, error)
}

type Config struct {
	BaseURL   string
	Store     Store
	Analytics *analytics.Service
	Registry  RegistryStatusProvider
	AdminUI   string
}

func NewHandler(cfg Config) http.Handler {
	store := cfg.Store
	if store == nil {
		panic("sourceapi: store is required")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			writeError(r, w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if r.URL.Path != "/" {
			writeError(r, w, http.StatusNotFound, "route_not_found", "route not found")
			return
		}
		entries := []directoryIndexEntry{
			{
				Name:        "healthz",
				Href:        "/healthz",
				Type:        "file",
				Description: "Health check endpoint",
			},
			{
				Name:        "v1/",
				Href:        "/v1/",
				Type:        "directory",
				Description: "Browse public registry APIs and artifacts",
			},
		}
		writeDirectoryIndex(w, http.StatusOK, directoryIndexPage{
			Title:       "Index of /",
			Path:        "/",
			Description: "Browse the public market registry APIs and artifact tree. JSON and download endpoints remain unchanged.",
			Entries:     entries,
		})
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(r, w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		statusCode, payload := buildHealthzPayload(r.Context(), cfg.Registry)
		writeJSON(r, w, statusCode, "no-store", payload)
	})
	mux.HandleFunc("/v1/source.json", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(r, w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		document, err := store.BuildSourceDocument(r.Context(), resolveBaseURL(cfg.BaseURL, r))
		if err != nil {
			writeError(r, w, http.StatusInternalServerError, "source_document_failed", "source document unavailable")
			return
		}
		recordAnalytics(r.Context(), cfg.Analytics, analytics.Event{Type: analytics.EventSourceView})
		writeJSON(r, w, http.StatusOK, "public, max-age=300", map[string]any{
			"success": true,
			"data":    document,
		})
	})
	mux.HandleFunc("/v1/catalog", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(r, w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		query := catalog.Query{
			Kind:    strings.TrimSpace(r.URL.Query().Get("kind")),
			Channel: strings.TrimSpace(r.URL.Query().Get("channel")),
			Search:  firstNonEmpty(r.URL.Query().Get("q"), r.URL.Query().Get("search")),
			Runtime: strings.TrimSpace(r.URL.Query().Get("runtime")),
			Offset:  parseIntDefault(r.URL.Query().Get("offset"), 0),
			Limit:   parseIntDefault(firstNonEmpty(r.URL.Query().Get("limit"), r.URL.Query().Get("page_size")), 20),
		}
		items, total, err := store.ListCatalog(r.Context(), query, resolveBaseURL(cfg.BaseURL, r))
		if err != nil {
			writeError(r, w, http.StatusInternalServerError, "catalog_unavailable", "catalog unavailable")
			return
		}
		recordAnalytics(r.Context(), cfg.Analytics, analytics.Event{Type: analytics.EventCatalogView})
		writeJSON(r, w, http.StatusOK, "public, max-age=60", map[string]any{
			"success": true,
			"data": map[string]any{
				"items": items,
				"pagination": map[string]any{
					"offset":   query.Offset,
					"limit":    query.Limit,
					"total":    total,
					"has_more": query.Offset+query.Limit < total,
				},
			},
		})
	})
	mux.HandleFunc("/v1/diagnostics/registry", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(r, w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if cfg.Registry == nil {
			writeError(r, w, http.StatusNotFound, "registry_diagnostics_unavailable", "registry diagnostics unavailable")
			return
		}
		status, err := cfg.Registry.RegistryStatus(r.Context())
		if err != nil {
			writeError(r, w, http.StatusServiceUnavailable, "registry_status_unavailable", "registry status unavailable")
			return
		}
		writeJSON(r, w, http.StatusOK, "no-store", map[string]any{"success": true, "data": status})
	})
	mux.HandleFunc("/v1/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			writeError(r, w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if r.URL.Path != "/v1/" {
			writeError(r, w, http.StatusNotFound, "route_not_found", "route not found")
			return
		}
		writeDirectoryIndex(w, http.StatusOK, directoryIndexPage{
			Title:       "Index of /v1/",
			Path:        "/v1/",
			Description: "Public API index for source documents, diagnostics and browseable artifact listings.",
			ParentHref:  "/",
			Entries: []directoryIndexEntry{
				{
					Name:        "source.json",
					Href:        "/v1/source.json",
					Type:        "file",
					Description: "Source metadata document",
				},
				{
					Name:        "catalog",
					Href:        "/v1/catalog",
					Type:        "file",
					Description: "Flat catalog API with query parameters",
				},
				{
					Name:        "diagnostics/",
					Href:        "/v1/diagnostics/",
					Type:        "directory",
					Description: "Registry diagnostics endpoints",
				},
				{
					Name:        "artifacts/",
					Href:        "/v1/artifacts/",
					Type:        "directory",
					Description: "Browse artifact kinds, entries and versions",
				},
			},
		})
	})
	mux.HandleFunc("/v1/diagnostics/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			writeError(r, w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if r.URL.Path != "/v1/diagnostics/" {
			writeError(r, w, http.StatusNotFound, "route_not_found", "route not found")
			return
		}
		writeDirectoryIndex(w, http.StatusOK, directoryIndexPage{
			Title:       "Index of /v1/diagnostics/",
			Path:        "/v1/diagnostics/",
			Description: "Operational diagnostics for the market registry.",
			ParentHref:  "/v1/",
			Entries: []directoryIndexEntry{
				{
					Name:        "registry",
					Href:        "/v1/diagnostics/registry",
					Type:        "file",
					Description: "Registry snapshot health and repair diagnostics",
				},
			},
		})
	})
	mux.HandleFunc("/v1/artifacts/", func(w http.ResponseWriter, r *http.Request) {
		serveArtifactDocument := func(kind string, name string) {
			document, err := store.BuildArtifactDocument(r.Context(), kind, name)
			if err != nil {
				writeCatalogError(r, w, err, "artifact_not_found", "artifact not found")
				return
			}
			recordAnalytics(r.Context(), cfg.Analytics, analytics.Event{
				Type:         analytics.EventArtifactView,
				ArtifactKind: kind,
				ArtifactName: name,
			})
			writeJSON(r, w, http.StatusOK, "public, max-age=60", map[string]any{"success": true, "data": document})
		}
		serveReleaseDocument := func(kind string, name string, version string) {
			document, err := store.BuildReleaseDocument(r.Context(), kind, name, version, resolveBaseURL(cfg.BaseURL, r))
			if err != nil {
				writeCatalogError(r, w, err, "release_not_found", "release not found")
				return
			}
			recordAnalytics(r.Context(), cfg.Analytics, analytics.Event{
				Type:         analytics.EventReleaseView,
				ArtifactKind: kind,
				ArtifactName: name,
			})
			writeJSON(r, w, http.StatusOK, "public, max-age=60", map[string]any{"success": true, "data": document})
		}
		serveReleaseDownload := func(kind string, name string, version string) {
			document, err := store.BuildReleaseDocument(r.Context(), kind, name, version, resolveBaseURL(cfg.BaseURL, r))
			if err != nil {
				writeCatalogError(r, w, err, "release_not_found", "release not found")
				return
			}
			payload, contentType, err := store.ReadReleaseArtifact(r.Context(), kind, name, version)
			if err != nil {
				writeCatalogError(r, w, err, "release_not_found", "release not found")
				return
			}
			download := document["download"].(map[string]any)
			etag := quoteETag(stringValue(download["sha256"]))
			if matchesETag(r.Header.Get("If-None-Match"), etag) {
				w.Header().Set("ETag", etag)
				w.Header().Set("Cache-Control", "public, max-age=3600, immutable")
				w.WriteHeader(http.StatusNotModified)
				return
			}
			if redirectResolver, ok := store.(ReleaseDownloadRedirectResolver); ok {
				redirectURL, redirectErr := redirectResolver.ResolveReleaseDownloadRedirect(r.Context(), kind, name, version)
				if redirectErr != nil {
					writeCatalogError(r, w, redirectErr, "release_not_found", "release not found")
					return
				}
				if strings.TrimSpace(redirectURL) != "" {
					w.Header().Set("ETag", etag)
					w.Header().Set("Cache-Control", "no-store")
					w.Header().Set("X-Artifact-Sha256", stringValue(download["sha256"]))
					w.Header().Set("X-Artifact-Version", version)
					w.Header().Set("X-Artifact-Kind", kind)
					recordAnalytics(r.Context(), cfg.Analytics, analytics.Event{
						Type:         analytics.EventDownload,
						ArtifactKind: kind,
						ArtifactName: name,
					})
					http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
					return
				}
			}
			w.Header().Set("Content-Disposition", buildAttachmentDisposition(kind, name, version, contentType))
			w.Header().Set("Content-Type", contentType)
			w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
			w.Header().Set("ETag", etag)
			w.Header().Set("Cache-Control", "public, max-age=3600, immutable")
			w.Header().Set("X-Artifact-Sha256", stringValue(download["sha256"]))
			w.Header().Set("X-Artifact-Version", version)
			w.Header().Set("X-Artifact-Kind", kind)
			recordAnalytics(r.Context(), cfg.Analytics, analytics.Event{
				Type:         analytics.EventDownload,
				ArtifactKind: kind,
				ArtifactName: name,
			})
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(payload)
		}

		relative := strings.TrimPrefix(r.URL.Path, "/v1/artifacts/")
		segments := splitPath(relative)
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			writeError(r, w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		hasTrailingSlash := strings.HasSuffix(r.URL.Path, "/")
		if len(segments) == 0 {
			items, err := listAllCatalogEntries(r.Context(), store, resolveBaseURL(cfg.BaseURL, r), catalog.Query{})
			if err != nil {
				writeError(r, w, http.StatusInternalServerError, "catalog_unavailable", "catalog unavailable")
				return
			}
			kinds := map[string]int{}
			for _, item := range items {
				kind := strings.TrimSpace(stringValue(item["kind"]))
				if kind == "" {
					continue
				}
				kinds[kind]++
			}
			entries := make([]directoryIndexEntry, 0, len(kinds))
			for kind, count := range kinds {
				entries = append(entries, directoryIndexEntry{
					Name:        kind + "/",
					Href:        fmt.Sprintf("/v1/artifacts/%s/", kind),
					Type:        "directory",
					Description: fmt.Sprintf("%d artifact(s)", count),
				})
			}
			sort.SliceStable(entries, func(i, j int) bool {
				return entries[i].Name < entries[j].Name
			})
			writeDirectoryIndex(w, http.StatusOK, directoryIndexPage{
				Title:       "Index of /v1/artifacts/",
				Path:        "/v1/artifacts/",
				Description: "Browse all published artifact kinds.",
				ParentHref:  "/v1/",
				Entries:     entries,
			})
			return
		}
		if len(segments) == 1 && hasTrailingSlash {
			items, err := listAllCatalogEntries(r.Context(), store, resolveBaseURL(cfg.BaseURL, r), catalog.Query{Kind: segments[0]})
			if err != nil {
				writeError(r, w, http.StatusInternalServerError, "catalog_unavailable", "catalog unavailable")
				return
			}
			if len(items) == 0 {
				writeError(r, w, http.StatusNotFound, "artifact_kind_not_found", "artifact kind not found")
				return
			}
			entries := make([]directoryIndexEntry, 0, len(items))
			for _, item := range items {
				name := strings.TrimSpace(stringValue(item["name"]))
				if name == "" {
					continue
				}
				title := strings.TrimSpace(stringValue(item["title"]))
				version := strings.TrimSpace(stringValue(item["latest_version"]))
				description := title
				if version != "" {
					if description != "" {
						description += " - "
					}
					description += "latest " + version
				}
				entries = append(entries, directoryIndexEntry{
					Name:        name + "/",
					Href:        fmt.Sprintf("/v1/artifacts/%s/%s/", segments[0], name),
					Type:        "directory",
					Description: description,
				})
			}
			sort.SliceStable(entries, func(i, j int) bool {
				return entries[i].Name < entries[j].Name
			})
			writeDirectoryIndex(w, http.StatusOK, directoryIndexPage{
				Title:       fmt.Sprintf("Index of /v1/artifacts/%s/", segments[0]),
				Path:        fmt.Sprintf("/v1/artifacts/%s/", segments[0]),
				Description: "Browse artifacts inside this kind.",
				ParentHref:  "/v1/artifacts/",
				Entries:     entries,
			})
			return
		}
		if len(segments) == 2 && hasTrailingSlash {
			document, err := store.BuildArtifactDocument(r.Context(), segments[0], segments[1])
			if err != nil {
				writeCatalogError(r, w, err, "artifact_not_found", "artifact not found")
				return
			}
			writeDirectoryIndex(w, http.StatusOK, directoryIndexPage{
				Title:       fmt.Sprintf("Index of /v1/artifacts/%s/%s/", segments[0], segments[1]),
				Path:        fmt.Sprintf("/v1/artifacts/%s/%s/", segments[0], segments[1]),
				Description: fmt.Sprintf("%s | latest version %s", stringValue(document["title"]), stringValue(document["latest_version"])),
				ParentHref:  fmt.Sprintf("/v1/artifacts/%s/", segments[0]),
				Entries: []directoryIndexEntry{
					{
						Name:        "artifact.json",
						Href:        fmt.Sprintf("/v1/artifacts/%s/%s/artifact.json", segments[0], segments[1]),
						Type:        "file",
						Description: "Artifact metadata document",
					},
					{
						Name:        "releases/",
						Href:        fmt.Sprintf("/v1/artifacts/%s/%s/releases/", segments[0], segments[1]),
						Type:        "directory",
						Description: "Browse published versions",
					},
				},
			})
			return
		}
		if len(segments) == 2 {
			serveArtifactDocument(segments[0], segments[1])
			return
		}
		if len(segments) == 3 && segments[2] == "artifact.json" {
			serveArtifactDocument(segments[0], segments[1])
			return
		}
		if len(segments) == 3 && segments[2] == "releases" {
			document, err := store.BuildArtifactDocument(r.Context(), segments[0], segments[1])
			if err != nil {
				writeCatalogError(r, w, err, "artifact_not_found", "artifact not found")
				return
			}
			rawVersions := normalizeDocumentVersions(document["versions"])
			entries := make([]directoryIndexEntry, 0, len(rawVersions))
			for _, item := range rawVersions {
				version := item
				versionName := strings.TrimSpace(stringValue(version["version"]))
				if versionName == "" {
					continue
				}
				channel := formatVersionChannels(version)
				publishedAt := strings.TrimSpace(stringValue(version["published_at"]))
				description := channel
				if publishedAt != "" {
					if description != "" {
						description += " | "
					}
					description += publishedAt
				}
				entries = append(entries, directoryIndexEntry{
					Name:        versionName + "/",
					Href:        fmt.Sprintf("/v1/artifacts/%s/%s/releases/%s/", segments[0], segments[1], versionName),
					Type:        "directory",
					Description: description,
				})
			}
			writeDirectoryIndex(w, http.StatusOK, directoryIndexPage{
				Title:       fmt.Sprintf("Index of /v1/artifacts/%s/%s/releases/", segments[0], segments[1]),
				Path:        fmt.Sprintf("/v1/artifacts/%s/%s/releases/", segments[0], segments[1]),
				Description: "Browse release versions for this artifact.",
				ParentHref:  fmt.Sprintf("/v1/artifacts/%s/%s/", segments[0], segments[1]),
				Entries:     entries,
			})
			return
		}
		if len(segments) == 4 && segments[2] == "releases" && hasTrailingSlash {
			document, err := store.BuildReleaseDocument(r.Context(), segments[0], segments[1], segments[3], resolveBaseURL(cfg.BaseURL, r))
			if err != nil {
				writeCatalogError(r, w, err, "release_not_found", "release not found")
				return
			}
			description := formatVersionChannels(document)
			if publishedAt := stringValue(document["published_at"]); publishedAt != "" {
				if description != "" {
					description += " | "
				}
				description += publishedAt
			}
			download, _ := document["download"].(map[string]any)
			downloadDescription := "Release archive download"
			downloadDetails := make([]string, 0, 2)
			if size := formatBytes(download["size"]); size != "" {
				downloadDetails = append(downloadDetails, size)
			}
			if digest := formatDigest(stringValue(download["sha256"])); digest != "" {
				downloadDetails = append(downloadDetails, "sha256 "+digest)
			}
			if len(downloadDetails) > 0 {
				downloadDescription += " | " + strings.Join(downloadDetails, " | ")
			}
			writeDirectoryIndex(w, http.StatusOK, directoryIndexPage{
				Title:       fmt.Sprintf("Index of /v1/artifacts/%s/%s/releases/%s/", segments[0], segments[1], segments[3]),
				Path:        fmt.Sprintf("/v1/artifacts/%s/%s/releases/%s/", segments[0], segments[1], segments[3]),
				Description: description,
				ParentHref:  fmt.Sprintf("/v1/artifacts/%s/%s/releases/", segments[0], segments[1]),
				Entries: []directoryIndexEntry{
					{
						Name:        "release.json",
						Href:        fmt.Sprintf("/v1/artifacts/%s/%s/releases/%s/release.json", segments[0], segments[1], segments[3]),
						Type:        "file",
						Description: "Release metadata document",
					},
					{
						Name:        "download",
						Href:        fmt.Sprintf("/v1/artifacts/%s/%s/releases/%s/download", segments[0], segments[1], segments[3]),
						Type:        "file",
						Description: downloadDescription,
					},
				},
			})
			return
		}
		if len(segments) == 4 && segments[2] == "releases" {
			serveReleaseDocument(segments[0], segments[1], segments[3])
			return
		}
		if len(segments) == 5 && segments[2] == "releases" && segments[4] == "release.json" {
			serveReleaseDocument(segments[0], segments[1], segments[3])
			return
		}
		if len(segments) == 5 && segments[2] == "releases" && segments[4] == "download" {
			serveReleaseDownload(segments[0], segments[1], segments[3])
			return
		}
		writeError(r, w, http.StatusNotFound, "route_not_found", "route not found")
	})
	return mux
}

func listAllCatalogEntries(ctx context.Context, store Store, baseURL string, query catalog.Query) ([]map[string]any, error) {
	const pageSize = 200

	offset := 0
	items := make([]map[string]any, 0)
	for {
		nextQuery := query
		nextQuery.Offset = offset
		nextQuery.Limit = pageSize

		page, total, err := store.ListCatalog(ctx, nextQuery, baseURL)
		if err != nil {
			return nil, err
		}
		items = append(items, page...)
		offset += len(page)
		if len(page) == 0 || offset >= total {
			break
		}
	}
	return items, nil
}

func normalizeDocumentVersions(value any) []map[string]any {
	switch typed := value.(type) {
	case []map[string]any:
		return typed
	case []any:
		out := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			version, _ := item.(map[string]any)
			if version != nil {
				out = append(out, version)
			}
		}
		return out
	default:
		return nil
	}
}

func formatVersionChannels(value map[string]any) string {
	channels := make([]string, 0, 4)
	if value != nil {
		if rawChannels, ok := value["channels"].([]string); ok {
			channels = append(channels, rawChannels...)
		} else if rawChannels, ok := value["channels"].([]any); ok {
			for _, item := range rawChannels {
				text := strings.TrimSpace(stringValue(item))
				if text != "" {
					channels = append(channels, text)
				}
			}
		}
		if channel := strings.TrimSpace(stringValue(value["channel"])); channel != "" {
			exists := false
			for _, item := range channels {
				if strings.EqualFold(strings.TrimSpace(item), channel) {
					exists = true
					break
				}
			}
			if !exists {
				channels = append([]string{channel}, channels...)
			}
		}
	}
	return strings.Join(channels, ", ")
}

func buildAttachmentDisposition(kind string, name string, version string, contentType string) string {
	filename := strings.TrimSpace(name)
	if filename == "" {
		filename = "artifact"
	}
	if strings.TrimSpace(version) != "" {
		filename += "-" + strings.TrimSpace(version)
	}
	filename += fileExtensionForContentType(contentType)
	return mime.FormatMediaType("attachment", map[string]string{"filename": filename})
}

func fileExtensionForContentType(contentType string) string {
	value := strings.ToLower(strings.TrimSpace(contentType))
	switch value {
	case "application/zip":
		return ".zip"
	case "application/gzip":
		return ".gz"
	case "application/json":
		return ".json"
	case "text/plain":
		return ".txt"
	default:
		return ".bin"
	}
}

func buildHealthzPayload(ctx context.Context, registry RegistryStatusProvider) (int, map[string]any) {
	checks := map[string]any{
		"api": map[string]any{
			"status": "ok",
		},
	}
	payload := map[string]any{
		"success": true,
		"data": map[string]any{
			"status":  "ok",
			"message": "source api is healthy",
			"checks":  checks,
		},
	}
	if registry == nil {
		return http.StatusOK, payload
	}

	status, err := registry.RegistryStatus(ctx)
	if err != nil {
		return http.StatusServiceUnavailable, map[string]any{
			"success": false,
			"message": "registry status unavailable",
			"error": map[string]any{
				"code":      "registry_status_unavailable",
				"message":   "registry status unavailable",
				"retryable": true,
				"details":   map[string]any{},
			},
			"data": map[string]any{
				"status":  "error",
				"message": "source api is running but registry diagnostics are unavailable",
				"checks": map[string]any{
					"api": checks["api"],
					"registry": map[string]any{
						"status":  "error",
						"message": "registry status unavailable",
					},
				},
			},
		}
	}

	registrySummary := map[string]any{
		"status":        status.Status,
		"healthy":       status.Healthy,
		"message":       status.Message,
		"artifactCount": status.ArtifactCount,
		"issues":        status.Issues,
		"checkedAt":     status.CheckedAt,
	}
	checks["registry"] = registrySummary

	if !status.Healthy {
		payload["data"] = map[string]any{
			"status":  "degraded",
			"message": "source api is healthy, registry snapshots need repair",
			"checks":  checks,
		}
	}

	return http.StatusOK, payload
}

func resolveBaseURL(configured string, r *http.Request) string {
	if trimmed := strings.TrimRight(strings.TrimSpace(configured), "/"); trimmed != "" {
		return trimmed
	}
	scheme := "http"
	if r != nil && r.TLS != nil {
		scheme = "https"
	}
	host := "127.0.0.1"
	if r != nil && strings.TrimSpace(r.Host) != "" {
		host = strings.TrimSpace(r.Host)
	}
	return scheme + "://" + host
}

func writeJSON(r *http.Request, w http.ResponseWriter, status int, cacheControl string, payload map[string]any) {
	body, err := json.Marshal(payload)
	if err != nil {
		writeError(r, w, http.StatusInternalServerError, "json_encode_failed", "response encode failed")
		return
	}
	etag := quoteETag(hashBytes(body))
	if matchesETag(headerValue(r, "If-None-Match"), etag) {
		w.Header().Set("ETag", etag)
		if cacheControl != "" {
			w.Header().Set("Cache-Control", cacheControl)
		}
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if cacheControl != "" {
		w.Header().Set("Cache-Control", cacheControl)
	}
	w.Header().Set("ETag", etag)
	w.WriteHeader(status)
	_, _ = w.Write(append(body, '\n'))
}

func writeError(r *http.Request, w http.ResponseWriter, status int, code string, message string) {
	writeJSON(r, w, status, "no-store", map[string]any{
		"success": false,
		"message": message,
		"error": map[string]any{
			"code":      code,
			"message":   message,
			"retryable": status >= http.StatusInternalServerError,
			"details":   map[string]any{},
		},
	})
}

func writeCatalogError(r *http.Request, w http.ResponseWriter, err error, notFoundCode string, notFoundMessage string) {
	if errors.Is(err, catalog.ErrNotFound) {
		writeError(r, w, http.StatusNotFound, notFoundCode, notFoundMessage)
		return
	}
	writeError(r, w, http.StatusInternalServerError, "catalog_read_failed", "catalog read failed")
}

func splitPath(value string) []string {
	parts := strings.Split(strings.Trim(value, "/"), "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func parseIntDefault(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed < 0 {
		return fallback
	}
	return parsed
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func stringValue(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func headerValue(r *http.Request, key string) string {
	if r == nil {
		return ""
	}
	return r.Header.Get(key)
}

func hashBytes(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func quoteETag(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return `"` + strings.Trim(strings.TrimSpace(value), `"`) + `"`
}

func matchesETag(header string, current string) bool {
	header = strings.TrimSpace(header)
	current = strings.TrimSpace(current)
	if header == "" || current == "" {
		return false
	}
	for _, candidate := range strings.Split(header, ",") {
		value := strings.TrimSpace(candidate)
		if value == "*" || value == current {
			return true
		}
	}
	return false
}

func recordAnalytics(ctx context.Context, service *analytics.Service, event analytics.Event) {
	if service == nil {
		return
	}
	_ = service.RecordEvent(ctx, event)
}
