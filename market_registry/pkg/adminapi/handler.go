package adminapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"auralogic/market_registry/pkg/analytics"
	"auralogic/market_registry/pkg/artifactorigin"
	"auralogic/market_registry/pkg/artifactsync"
	"auralogic/market_registry/pkg/auth"
	"auralogic/market_registry/pkg/catalog"
	"auralogic/market_registry/pkg/publish"
	"auralogic/market_registry/pkg/registrysettings"
	"auralogic/market_registry/pkg/signing"
	"auralogic/market_registry/pkg/storage"
)

var maxArtifactUploadSize int64 = 100 << 20

const maxArtifactUploadOverhead int64 = 1 << 20
const maxArtifactMultipartMemory int64 = 8 << 20

type Handler struct {
	auth     *auth.Service
	publish  *publish.Service
	storage  storage.Storage
	signing  *signing.Service
	settings *registrysettings.Service
}

type Options struct {
	Settings *registrysettings.Service
}

type gitHubReleaseAdminRequest struct {
	Kind                     string           `json:"kind"`
	Name                     string           `json:"name"`
	Version                  string           `json:"version"`
	Channel                  string           `json:"channel"`
	ArtifactStorageProfileID string           `json:"artifact_storage_profile_id"`
	Owner                    string           `json:"owner"`
	Repo                     string           `json:"repo"`
	Tag                      string           `json:"tag"`
	Asset                    string           `json:"asset"`
	AssetName                string           `json:"asset_name"`
	APIBaseURL               string           `json:"api_base_url"`
	Token                    string           `json:"token"`
	Metadata                 publish.Metadata `json:"metadata"`
}

func NewHandler(authSvc *auth.Service, pubSvc *publish.Service, store storage.Storage, sign *signing.Service) *Handler {
	return NewHandlerWithOptions(authSvc, pubSvc, store, sign, Options{})
}

func NewHandlerWithOptions(authSvc *auth.Service, pubSvc *publish.Service, store storage.Storage, sign *signing.Service, opts Options) *Handler {
	return &Handler{
		auth:     authSvc,
		publish:  pubSvc,
		storage:  store,
		signing:  sign,
		settings: opts.Settings,
	}
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid request")
		return
	}

	token, err := h.auth.Login(req.Username, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid credentials")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"token": token,
			"user": map[string]string{
				"username": req.Username,
				"role":     "admin",
			},
		},
	})
}

func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if !h.authenticate(w, r) {
		return
	}
	if h.settings == nil {
		writeError(w, http.StatusServiceUnavailable, "settings_unavailable", "settings unavailable")
		return
	}

	settings, err := h.settings.Get(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "settings_unavailable", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    settings,
	})
}

func (h *Handler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if !h.authenticate(w, r) {
		return
	}
	if h.settings == nil {
		writeError(w, http.StatusServiceUnavailable, "settings_unavailable", "settings unavailable")
		return
	}

	var req registrysettings.Document
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid request")
		return
	}

	settings, err := h.settings.Update(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "settings_update_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "settings updated successfully",
		"data":    settings,
	})
}

func (h *Handler) HandleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.GetSettings(w, r)
	case http.MethodPut:
		h.UpdateSettings(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (h *Handler) Publish(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if !h.authenticate(w, r) {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxArtifactUploadBodySize())
	if err := r.ParseMultipartForm(maxArtifactMultipartMemory); err != nil {
		if isRequestBodyTooLarge(err) {
			writeError(w, http.StatusRequestEntityTooLarge, "artifact_too_large", "artifact file exceeds upload size limit")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_form", "invalid form")
		return
	}

	file, header, err := r.FormFile("artifact")
	if err != nil {
		writeError(w, http.StatusBadRequest, "artifact_file_required", "artifact file required")
		return
	}
	defer file.Close()
	if header != nil && header.Size > maxArtifactUploadSize {
		writeError(w, http.StatusRequestEntityTooLarge, "artifact_too_large", "artifact file exceeds upload size limit")
		return
	}

	zipData, err := io.ReadAll(io.LimitReader(file, maxArtifactUploadSize+1))
	if err != nil {
		if isRequestBodyTooLarge(err) {
			writeError(w, http.StatusRequestEntityTooLarge, "artifact_too_large", "artifact file exceeds upload size limit")
			return
		}
		writeError(w, http.StatusInternalServerError, "read_file_failed", "read file failed")
		return
	}
	if int64(len(zipData)) > maxArtifactUploadSize {
		writeError(w, http.StatusRequestEntityTooLarge, "artifact_too_large", "artifact file exceeds upload size limit")
		return
	}

	metadataStr := strings.TrimSpace(r.FormValue("metadata"))
	var metadata publish.Metadata
	if metadataStr != "" {
		if err := json.Unmarshal([]byte(metadataStr), &metadata); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_metadata", "invalid metadata")
			return
		}
	}

	req := publish.Request{
		Kind:                     strings.TrimSpace(r.FormValue("kind")),
		Name:                     strings.TrimSpace(r.FormValue("name")),
		Version:                  strings.TrimSpace(r.FormValue("version")),
		Channel:                  strings.TrimSpace(r.FormValue("channel")),
		ArtifactStorageProfileID: strings.TrimSpace(r.FormValue("artifact_storage_profile_id")),
		ArtifactZip:              zipData,
		Metadata:                 metadata,
	}

	if err := h.publish.Publish(r.Context(), req); err != nil {
		if publish.IsRequestError(err) {
			writeError(w, http.StatusBadRequest, "invalid_publish_request", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "publish_failed", err.Error())
		return
	}

	response := map[string]any{
		"success": true,
		"message": "published successfully",
	}
	h.attachRegistryRefresh(r.Context(), response, "artifact published, but registry snapshots refresh failed")
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) SyncGitHubRelease(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if !h.authenticate(w, r) {
		return
	}

	req, ok := decodeGitHubReleaseAdminRequest(w, r)
	if !ok {
		return
	}

	result, err := artifactsync.NewGitHubReleaseSyncer(h.publish).Sync(r.Context(), artifactsync.GitHubReleaseRequest{
		Kind:                     strings.TrimSpace(req.Kind),
		Name:                     strings.TrimSpace(req.Name),
		Version:                  strings.TrimSpace(req.Version),
		Channel:                  firstNonEmpty(strings.TrimSpace(req.Channel), "stable"),
		ArtifactStorageProfileID: strings.TrimSpace(req.ArtifactStorageProfileID),
		Owner:                    strings.TrimSpace(req.Owner),
		Repo:                     strings.TrimSpace(req.Repo),
		Tag:                      strings.TrimSpace(req.Tag),
		AssetName:                firstNonEmpty(strings.TrimSpace(req.AssetName), strings.TrimSpace(req.Asset)),
		APIBaseURL:               strings.TrimSpace(req.APIBaseURL),
		Token:                    strings.TrimSpace(req.Token),
		Metadata:                 req.Metadata,
	})
	if err != nil {
		switch {
		case publish.IsRequestError(err), artifactsync.IsRequestError(err):
			writeError(w, http.StatusBadRequest, "invalid_sync_request", err.Error())
		default:
			writeError(w, http.StatusBadGateway, "sync_failed", err.Error())
		}
		return
	}

	response := map[string]any{
		"success": true,
		"message": "github release synced successfully",
		"data": map[string]any{
			"result": result,
		},
	}
	h.attachRegistryRefresh(r.Context(), response, "github release synced, but registry snapshots refresh failed")
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) InspectGitHubRelease(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if !h.authenticate(w, r) {
		return
	}

	req, ok := decodeGitHubReleaseAdminRequest(w, r)
	if !ok {
		return
	}

	result, err := artifactsync.NewGitHubReleaseSyncer(h.publish).Inspect(r.Context(), artifactsync.GitHubReleaseInspectionRequest{
		Kind:       strings.TrimSpace(req.Kind),
		Name:       strings.TrimSpace(req.Name),
		Version:    strings.TrimSpace(req.Version),
		Owner:      strings.TrimSpace(req.Owner),
		Repo:       strings.TrimSpace(req.Repo),
		Tag:        strings.TrimSpace(req.Tag),
		AssetName:  firstNonEmpty(strings.TrimSpace(req.AssetName), strings.TrimSpace(req.Asset)),
		APIBaseURL: strings.TrimSpace(req.APIBaseURL),
		Token:      strings.TrimSpace(req.Token),
	})
	if err != nil {
		switch {
		case publish.IsRequestError(err), artifactsync.IsRequestError(err):
			writeError(w, http.StatusBadRequest, "invalid_sync_request", err.Error())
		default:
			writeError(w, http.StatusBadGateway, "sync_inspection_failed", err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "github release inspected successfully",
		"data": map[string]any{
			"result": result,
		},
	})
}

func (h *Handler) PreviewGitHubRelease(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if !h.authenticate(w, r) {
		return
	}

	req, ok := decodeGitHubReleaseAdminRequest(w, r)
	if !ok {
		return
	}

	result, err := artifactsync.NewGitHubReleaseSyncer(h.publish).Preview(r.Context(), artifactsync.GitHubReleasePreviewRequest{
		Owner:      strings.TrimSpace(req.Owner),
		Repo:       strings.TrimSpace(req.Repo),
		Tag:        strings.TrimSpace(req.Tag),
		AssetName:  firstNonEmpty(strings.TrimSpace(req.AssetName), strings.TrimSpace(req.Asset)),
		APIBaseURL: strings.TrimSpace(req.APIBaseURL),
		Token:      strings.TrimSpace(req.Token),
	})
	if err != nil {
		switch {
		case publish.IsRequestError(err), artifactsync.IsRequestError(err):
			writeError(w, http.StatusBadRequest, "invalid_sync_request", err.Error())
		default:
			writeError(w, http.StatusBadGateway, "sync_preview_failed", err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "github release preview loaded successfully",
		"data": map[string]any{
			"result": result,
		},
	})
}

func decodeGitHubReleaseAdminRequest(w http.ResponseWriter, r *http.Request) (gitHubReleaseAdminRequest, bool) {
	var req gitHubReleaseAdminRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid request")
		return gitHubReleaseAdminRequest{}, false
	}
	return req, true
}

func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if !h.authenticate(w, r) {
		return
	}

	stats, err := analytics.NewService(h.storage, analytics.Config{}).BuildOverview(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "stats_unavailable", "stats unavailable")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    stats,
	})
}

func (h *Handler) GetRegistryStatus(w http.ResponseWriter, r *http.Request) {
	if !h.authenticate(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	status, err := h.publish.RegistryStatus(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "registry_status_unavailable", "registry status unavailable")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    status,
	})
}

func (h *Handler) ReindexRegistry(w http.ResponseWriter, r *http.Request) {
	if !h.authenticate(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	result, err := h.publish.RebuildRegistry(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "registry_reindex_failed", "registry reindex failed")
		return
	}
	status, err := h.publish.RegistryStatus(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "registry_status_unavailable", "registry status unavailable")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "registry rebuilt successfully",
		"data": map[string]any{
			"result": result,
			"status": status,
		},
	})
}

func (h *Handler) ListArtifacts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if !h.authenticate(w, r) {
		return
	}

	paths, err := h.storage.List(r.Context(), "index/artifacts")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_artifacts_failed", "list artifacts failed")
		return
	}

	artifactCatalog := map[string]map[string]any{}
	for _, itemPath := range paths {
		if !strings.HasSuffix(strings.ToLower(strings.TrimSpace(itemPath)), "/index.json") {
			continue
		}
		kind, name, ok := catalog.ParseArtifactIndexPath(itemPath)
		if !ok {
			continue
		}
		index, loadErr := h.readArtifactIndex(r.Context(), kind, name)
		if loadErr != nil {
			continue
		}
		if _, exists := artifactCatalog[kind]; !exists {
			artifactCatalog[kind] = map[string]any{}
		}
		artifactCatalog[kind][name] = h.summarizeArtifactIndex(r.Context(), kind, name, index)
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true, "data": artifactCatalog})
}

func (h *Handler) GetArtifactVersions(w http.ResponseWriter, r *http.Request) {
	kind, name, ok := h.authenticateAndParseArtifactIdentity(w, r)
	if !ok {
		return
	}
	index, err := h.readArtifactIndex(r.Context(), kind, name)
	if err != nil {
		writeError(w, http.StatusNotFound, "artifact_not_found", "artifact not found")
		return
	}
	index = h.summarizeArtifactIndex(r.Context(), kind, name, index)

	writeJSON(w, http.StatusOK, map[string]any{"success": true, "data": index})
}

func (h *Handler) GetArtifactRelease(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if !h.authenticate(w, r) {
		return
	}

	parts := artifactPathParts(r.URL.Path)
	if len(parts) != 3 {
		writeError(w, http.StatusBadRequest, "invalid_path", "invalid path")
		return
	}
	release, err := h.readArtifactReleaseDetail(r.Context(), parts[0], parts[1], parts[2])
	if err != nil {
		writeError(w, http.StatusNotFound, "artifact_release_not_found", "artifact release not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "data": release})
}

func (h *Handler) HandleArtifactRoute(w http.ResponseWriter, r *http.Request) {
	parts := artifactPathParts(r.URL.Path)
	switch {
	case len(parts) == 2 && r.Method == http.MethodGet:
		h.GetArtifactVersions(w, r)
	case len(parts) == 2 && r.Method == http.MethodDelete:
		h.DeleteArtifact(w, r)
	case len(parts) == 3 && !strings.EqualFold(parts[2], "check-origin") && r.Method == http.MethodGet:
		h.GetArtifactRelease(w, r)
	case len(parts) == 3 && !strings.EqualFold(parts[2], "check-origin") && r.Method == http.MethodDelete:
		h.DeleteArtifactRelease(w, r)
	case len(parts) == 3 && strings.EqualFold(parts[2], "check-origin") && r.Method == http.MethodPost:
		h.CheckArtifactOrigins(w, r)
	case len(parts) == 4 && strings.EqualFold(parts[3], "check-origin") && r.Method == http.MethodPost:
		h.CheckArtifactOrigin(w, r)
	case len(parts) == 4 && strings.EqualFold(parts[3], "resync") && r.Method == http.MethodPost:
		h.ResyncArtifactVersion(w, r)
	case len(parts) == 2 ||
		(len(parts) == 3 && !strings.EqualFold(parts[2], "check-origin")) ||
		(len(parts) == 3 && strings.EqualFold(parts[2], "check-origin")) ||
		(len(parts) == 4 && (strings.EqualFold(parts[3], "resync") || strings.EqualFold(parts[3], "check-origin"))):
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	default:
		writeError(w, http.StatusBadRequest, "invalid_path", "invalid path")
	}
}

func (h *Handler) DeleteArtifact(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if !h.authenticate(w, r) {
		return
	}

	parts := artifactPathParts(r.URL.Path)
	if len(parts) != 2 {
		writeError(w, http.StatusBadRequest, "invalid_path", "invalid path")
		return
	}

	result, err := h.publish.DeleteArtifact(r.Context(), parts[0], parts[1])
	if err != nil {
		switch {
		case publish.IsRequestError(err) && isRequestNotFound(err):
			writeError(w, http.StatusNotFound, "artifact_not_found", err.Error())
		case publish.IsRequestError(err):
			writeError(w, http.StatusBadRequest, "artifact_delete_failed", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "artifact_delete_failed", err.Error())
		}
		return
	}

	response := map[string]any{
		"success": true,
		"message": "artifact deleted successfully",
		"data": map[string]any{
			"result": result,
		},
	}
	h.attachRegistryRefresh(r.Context(), response, "artifact deleted, but registry snapshots refresh failed")
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) DeleteArtifactRelease(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if !h.authenticate(w, r) {
		return
	}

	parts := artifactPathParts(r.URL.Path)
	if len(parts) != 3 || strings.EqualFold(parts[2], "check-origin") {
		writeError(w, http.StatusBadRequest, "invalid_path", "invalid path")
		return
	}

	result, err := h.publish.DeleteRelease(r.Context(), parts[0], parts[1], parts[2])
	if err != nil {
		switch {
		case publish.IsRequestError(err) && isRequestNotFound(err):
			writeError(w, http.StatusNotFound, "artifact_release_not_found", err.Error())
		case publish.IsRequestError(err):
			writeError(w, http.StatusBadRequest, "artifact_release_delete_failed", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "artifact_release_delete_failed", err.Error())
		}
		return
	}

	response := map[string]any{
		"success": true,
		"message": "artifact release deleted successfully",
		"data": map[string]any{
			"result": result,
		},
	}
	h.attachRegistryRefresh(r.Context(), response, "artifact release deleted, but registry snapshots refresh failed")
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) CheckArtifactOrigins(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if !h.authenticate(w, r) {
		return
	}

	parts := artifactPathParts(r.URL.Path)
	if len(parts) != 3 || !strings.EqualFold(parts[2], "check-origin") {
		writeError(w, http.StatusBadRequest, "invalid_path", "invalid path")
		return
	}
	kind := parts[0]
	name := parts[1]

	req, ok := decodeOptionalTokenRequest(w, r)
	if !ok {
		return
	}

	index, err := h.readArtifactIndex(r.Context(), kind, name)
	if err != nil {
		writeError(w, http.StatusNotFound, "artifact_not_found", "artifact not found")
		return
	}

	entries := parseVersionEntries(index["releases"])
	if len(entries) == 0 {
		entries = parseVersionEntries(index["versions"])
	}

	syncer := artifactsync.NewGitHubReleaseSyncer(h.publish)
	items := make([]map[string]any, 0, len(entries))
	checkedVersions := 0
	changedVersions := 0
	failedVersions := 0
	skippedVersions := 0
	gitHubVersions := 0

	for _, entry := range entries {
		version := stringValue(entry["version"])
		if version == "" {
			continue
		}

		origin, release, originDoc, loadErr := h.readGitHubOriginArtifactContext(r.Context(), kind, name, version)
		if loadErr != nil {
			if strings.Contains(loadErr.Error(), "unsupported_origin_provider") {
				skippedVersions++
				continue
			}
			failedVersions++
			items = append(items, map[string]any{
				"version": version,
				"ok":      false,
				"error":   loadErr.Error(),
			})
			continue
		}

		gitHubVersions++
		integrity := mapValue(origin["integrity"])
		result, inspectErr := syncer.Inspect(r.Context(), artifactsync.GitHubReleaseInspectionRequest{
			Kind:           firstNonEmpty(kind, stringValue(release["kind"])),
			Name:           firstNonEmpty(name, stringValue(release["name"])),
			Version:        firstNonEmpty(version, stringValue(release["version"])),
			Owner:          stringValue(originDoc.Locator["owner"]),
			Repo:           stringValue(originDoc.Locator["repo"]),
			Tag:            stringValue(originDoc.Locator["tag"]),
			AssetName:      stringValue(originDoc.Locator["asset_name"]),
			APIBaseURL:     stringValue(originDoc.Locator["api_base_url"]),
			Token:          strings.TrimSpace(req.Token),
			ExpectedSHA256: stringValue(integrity["sha256"]),
			ExpectedSize:   int64Value(integrity["size"]),
		})
		if inspectErr != nil {
			failedVersions++
			items = append(items, map[string]any{
				"version": version,
				"ok":      false,
				"error":   inspectErr.Error(),
			})
			continue
		}

		checkedVersions++
		if result.Changed {
			changedVersions++
		}
		items = append(items, map[string]any{
			"version": version,
			"ok":      true,
			"changed": result.Changed,
			"result":  result,
		})
	}

	if gitHubVersions == 0 {
		writeError(w, http.StatusBadRequest, "unsupported_origin_provider", "this artifact does not contain github_release versions")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "artifact origins checked successfully",
		"data": map[string]any{
			"checked_versions": checkedVersions,
			"changed_versions": changedVersions,
			"failed_versions":  failedVersions,
			"skipped_versions": skippedVersions,
			"github_versions":  gitHubVersions,
			"items":            items,
		},
	})
}

func (h *Handler) ResyncArtifactVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if !h.authenticate(w, r) {
		return
	}

	parts := artifactPathParts(r.URL.Path)
	if len(parts) != 4 || !strings.EqualFold(parts[3], "resync") {
		writeError(w, http.StatusBadRequest, "invalid_path", "invalid path")
		return
	}
	kind := parts[0]
	name := parts[1]
	version := parts[2]

	req, ok := decodeOptionalTokenRequest(w, r)
	if !ok {
		return
	}

	_, release, originDoc, ok := h.loadGitHubOriginArtifactContext(w, r.Context(), kind, name, version)
	if !ok {
		return
	}

	locator := cloneMap(originDoc.Locator)
	result, err := artifactsync.NewGitHubReleaseSyncer(h.publish).Sync(r.Context(), artifactsync.GitHubReleaseRequest{
		Kind:                     firstNonEmpty(kind, stringValue(release["kind"])),
		Name:                     firstNonEmpty(name, stringValue(release["name"])),
		Version:                  firstNonEmpty(version, stringValue(release["version"])),
		Channel:                  firstNonEmpty(stringValue(release["channel"]), "stable"),
		ArtifactStorageProfileID: strings.TrimSpace(originDoc.StorageProfileID),
		Owner:                    stringValue(locator["owner"]),
		Repo:                     stringValue(locator["repo"]),
		Tag:                      stringValue(locator["tag"]),
		AssetName:                stringValue(locator["asset_name"]),
		APIBaseURL:               stringValue(locator["api_base_url"]),
		Token:                    strings.TrimSpace(req.Token),
		Metadata:                 metadataFromReleaseManifest(release),
	})
	if err != nil {
		switch {
		case publish.IsRequestError(err), artifactsync.IsRequestError(err):
			writeError(w, http.StatusBadRequest, "artifact_resync_failed", err.Error())
		default:
			writeError(w, http.StatusBadGateway, "artifact_resync_failed", err.Error())
		}
		return
	}

	response := map[string]any{
		"success": true,
		"message": "artifact version resynced successfully",
		"data": map[string]any{
			"result": result,
		},
	}
	h.attachRegistryRefresh(r.Context(), response, "artifact version resynced, but registry snapshots refresh failed")
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) CheckArtifactOrigin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if !h.authenticate(w, r) {
		return
	}

	parts := artifactPathParts(r.URL.Path)
	if len(parts) != 4 || !strings.EqualFold(parts[3], "check-origin") {
		writeError(w, http.StatusBadRequest, "invalid_path", "invalid path")
		return
	}
	kind := parts[0]
	name := parts[1]
	version := parts[2]

	req, ok := decodeOptionalTokenRequest(w, r)
	if !ok {
		return
	}

	origin, release, originDoc, ok := h.loadGitHubOriginArtifactContext(w, r.Context(), kind, name, version)
	if !ok {
		return
	}

	integrity := mapValue(origin["integrity"])
	result, err := artifactsync.NewGitHubReleaseSyncer(h.publish).Inspect(r.Context(), artifactsync.GitHubReleaseInspectionRequest{
		Kind:           firstNonEmpty(kind, stringValue(release["kind"])),
		Name:           firstNonEmpty(name, stringValue(release["name"])),
		Version:        firstNonEmpty(version, stringValue(release["version"])),
		Owner:          stringValue(originDoc.Locator["owner"]),
		Repo:           stringValue(originDoc.Locator["repo"]),
		Tag:            stringValue(originDoc.Locator["tag"]),
		AssetName:      stringValue(originDoc.Locator["asset_name"]),
		APIBaseURL:     stringValue(originDoc.Locator["api_base_url"]),
		Token:          strings.TrimSpace(req.Token),
		ExpectedSHA256: stringValue(integrity["sha256"]),
		ExpectedSize:   int64Value(integrity["size"]),
	})
	if err != nil {
		switch {
		case publish.IsRequestError(err), artifactsync.IsRequestError(err):
			writeError(w, http.StatusBadRequest, "artifact_origin_check_failed", err.Error())
		default:
			writeError(w, http.StatusBadGateway, "artifact_origin_check_failed", err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "artifact origin checked successfully",
		"data": map[string]any{
			"result": result,
		},
	})
}

func (h *Handler) authenticate(w http.ResponseWriter, r *http.Request) bool {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		writeError(w, http.StatusUnauthorized, "missing_authorization", "missing authorization")
		return false
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if _, err := h.auth.ValidateToken(token); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_token", "invalid token")
		return false
	}

	return true
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, code string, message string) {
	writeJSON(w, status, map[string]any{
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

func maxArtifactUploadBodySize() int64 {
	return maxArtifactUploadSize + maxArtifactUploadOverhead
}

func isRequestBodyTooLarge(err error) bool {
	var maxBytesErr *http.MaxBytesError
	return errors.As(err, &maxBytesErr) || strings.Contains(strings.ToLower(strings.TrimSpace(err.Error())), "request body too large")
}

func (h *Handler) attachRegistryRefresh(ctx context.Context, response map[string]any, warningMessage string) {
	if _, err := h.publish.RebuildRegistry(ctx); err != nil {
		response["warning"] = strings.TrimSpace(warningMessage)
		return
	}
	status, err := h.publish.RegistryStatus(ctx)
	if err != nil {
		response["warning"] = strings.TrimSpace(warningMessage)
		return
	}
	data := mapValue(response["data"])
	data["status"] = status
	response["data"] = data
}

type optionalTokenRequest struct {
	Token string `json:"token"`
}

func decodeOptionalTokenRequest(w http.ResponseWriter, r *http.Request) (optionalTokenRequest, bool) {
	var req optionalTokenRequest
	if r.Body == nil {
		return req, true
	}
	defer r.Body.Close()
	if len(strings.TrimSpace(r.Header.Get("Content-Type"))) == 0 {
		return req, true
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid request")
		return optionalTokenRequest{}, false
	}
	return req, true
}

func (h *Handler) authenticateAndParseArtifactIdentity(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return "", "", false
	}
	if !h.authenticate(w, r) {
		return "", "", false
	}
	parts := artifactPathParts(r.URL.Path)
	if len(parts) != 2 {
		writeError(w, http.StatusBadRequest, "invalid_path", "invalid path")
		return "", "", false
	}
	return parts[0], parts[1], true
}

func artifactPathParts(path string) []string {
	rawParts := strings.Split(strings.TrimPrefix(path, "/admin/artifacts/"), "/")
	parts := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

func (h *Handler) loadGitHubOriginArtifactContext(
	w http.ResponseWriter,
	ctx context.Context,
	kind string,
	name string,
	version string,
) (map[string]any, map[string]any, artifactorigin.Document, bool) {
	release, err := h.readReleaseManifest(ctx, kind, name, version)
	if err != nil {
		writeError(w, http.StatusNotFound, "artifact_not_found", "artifact release not found")
		return nil, nil, artifactorigin.Document{}, false
	}
	origin, err := h.readOriginDocument(ctx, kind, name, version)
	if err != nil {
		writeError(w, http.StatusNotFound, "artifact_origin_not_found", "artifact origin not found")
		return nil, nil, artifactorigin.Document{}, false
	}
	originDoc := artifactorigin.FromMap(origin)
	if !strings.EqualFold(strings.TrimSpace(originDoc.Provider), artifactorigin.ProviderGitHubRelease) {
		writeError(w, http.StatusBadRequest, "unsupported_origin_provider", "only github_release artifacts support this action right now")
		return nil, nil, artifactorigin.Document{}, false
	}
	return origin, release, originDoc, true
}

func (h *Handler) readGitHubOriginArtifactContext(
	ctx context.Context,
	kind string,
	name string,
	version string,
) (map[string]any, map[string]any, artifactorigin.Document, error) {
	release, err := h.readReleaseManifest(ctx, kind, name, version)
	if err != nil {
		return nil, nil, artifactorigin.Document{}, fmt.Errorf("artifact release not found")
	}
	origin, err := h.readOriginDocument(ctx, kind, name, version)
	if err != nil {
		return nil, nil, artifactorigin.Document{}, fmt.Errorf("artifact origin not found")
	}
	originDoc := artifactorigin.FromMap(origin)
	if !strings.EqualFold(strings.TrimSpace(originDoc.Provider), artifactorigin.ProviderGitHubRelease) {
		return nil, nil, artifactorigin.Document{}, fmt.Errorf("unsupported_origin_provider")
	}
	return origin, release, originDoc, nil
}

func (h *Handler) readArtifactIndex(ctx context.Context, kind string, name string) (map[string]any, error) {
	indexPath := strings.Join([]string{"index", "artifacts", kind, name, "index.json"}, "/")
	data, err := h.storage.Read(ctx, indexPath)
	if err != nil {
		return nil, err
	}

	var index map[string]any
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, err
	}
	return catalog.NormalizeArtifactIndex(kind, name, index), nil
}

func (h *Handler) enrichArtifactIndex(ctx context.Context, kind string, name string, index map[string]any) map[string]any {
	index = catalog.NormalizeArtifactIndex(kind, name, index)
	index["versions"] = h.enrichVersionEntries(ctx, kind, name, parseVersionEntries(index["versions"]))
	index["releases"] = h.enrichVersionEntries(ctx, kind, name, parseVersionEntries(index["releases"]))
	h.enrichLatestArtifactSummary(ctx, kind, name, index)
	return index
}

func (h *Handler) summarizeArtifactIndex(ctx context.Context, kind string, name string, index map[string]any) map[string]any {
	index = catalog.NormalizeArtifactIndex(kind, name, index)
	index["versions"] = h.summarizeVersionEntries(ctx, kind, name, parseVersionEntries(index["versions"]))
	index["releases"] = h.summarizeVersionEntries(ctx, kind, name, parseVersionEntries(index["releases"]))
	h.summarizeLatestArtifactSummary(ctx, kind, name, index)
	delete(index, "latest_origin")
	return index
}

func (h *Handler) summarizeVersionEntries(ctx context.Context, kind string, name string, entries []map[string]any) []map[string]any {
	if len(entries) == 0 {
		return []map[string]any{}
	}

	out := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		cloned := cloneMap(entry)
		delete(cloned, "origin")

		version := stringValue(cloned["version"])
		if version == "" {
			out = append(out, cloned)
			continue
		}

		if release, err := h.readReleaseManifest(ctx, kind, name, version); err == nil {
			download := mapValue(release["download"])
			transport := mapValue(download["transport"])
			if stringValue(cloned["channel"]) == "" {
				cloned["channel"] = stringValue(release["channel"])
			}
			if stringValue(cloned["published_at"]) == "" {
				cloned["published_at"] = stringValue(release["published_at"])
			}
			if stringValue(cloned["sha256"]) == "" {
				cloned["sha256"] = stringValue(download["sha256"])
			}
			if _, exists := cloned["size"]; !exists {
				cloned["size"] = download["size"]
			}
			if stringValue(cloned["content_type"]) == "" {
				cloned["content_type"] = stringValue(download["content_type"])
			}
			if len(transport) > 0 {
				cloned["transport"] = cloneMap(transport)
			}
			if downloadURL := stringValue(download["url"]); downloadURL != "" {
				cloned["download_url"] = downloadURL
			}
			if title := stringValue(release["title"]); title != "" {
				cloned["title"] = title
			}
			if summary := stringValue(release["summary"]); summary != "" {
				cloned["summary"] = summary
			}
		}

		if origin, err := h.readOriginDocument(ctx, kind, name, version); err == nil && len(origin) > 0 {
			originDoc := artifactorigin.FromMap(origin)
			if len(mapValue(cloned["transport"])) == 0 {
				cloned["transport"] = artifactorigin.TransportMap(originDoc)
			}
			if strings.TrimSpace(originDoc.StorageProfileID) != "" {
				cloned["storage_profile_id"] = originDoc.StorageProfileID
			}
			if sync := mapValue(origin["sync"]); len(sync) > 0 {
				cloned["sync"] = cloneMap(sync)
			}
		}

		out = append(out, cloned)
	}

	return out
}

func (h *Handler) summarizeLatestArtifactSummary(ctx context.Context, kind string, name string, index map[string]any) {
	version := stringValue(index["latest_version"])
	if version == "" {
		return
	}

	if release, err := h.readReleaseManifest(ctx, kind, name, version); err == nil {
		download := mapValue(release["download"])
		transport := mapValue(download["transport"])
		if len(transport) > 0 {
			index["latest_transport"] = cloneMap(transport)
		}
		if downloadURL := stringValue(download["url"]); downloadURL != "" {
			index["latest_download_url"] = downloadURL
		}
	}

	if origin, err := h.readOriginDocument(ctx, kind, name, version); err == nil && len(origin) > 0 {
		originDoc := artifactorigin.FromMap(origin)
		if len(mapValue(index["latest_transport"])) == 0 {
			index["latest_transport"] = artifactorigin.TransportMap(originDoc)
		}
		if strings.TrimSpace(originDoc.StorageProfileID) != "" {
			index["latest_storage_profile_id"] = originDoc.StorageProfileID
		}
		if sync := mapValue(origin["sync"]); len(sync) > 0 {
			index["latest_sync"] = cloneMap(sync)
		}
	}
}

func (h *Handler) enrichVersionEntries(ctx context.Context, kind string, name string, entries []map[string]any) []map[string]any {
	if len(entries) == 0 {
		return []map[string]any{}
	}
	out := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		cloned := cloneMap(entry)
		version := stringValue(cloned["version"])
		if version == "" {
			out = append(out, cloned)
			continue
		}
		release, err := h.readReleaseManifest(ctx, kind, name, version)
		if err == nil {
			download := mapValue(release["download"])
			transport := mapValue(download["transport"])
			if stringValue(cloned["channel"]) == "" {
				cloned["channel"] = stringValue(release["channel"])
			}
			if stringValue(cloned["published_at"]) == "" {
				cloned["published_at"] = stringValue(release["published_at"])
			}
			if stringValue(cloned["sha256"]) == "" {
				cloned["sha256"] = stringValue(download["sha256"])
			}
			if _, exists := cloned["size"]; !exists {
				cloned["size"] = download["size"]
			}
			if stringValue(cloned["content_type"]) == "" {
				cloned["content_type"] = stringValue(download["content_type"])
			}
			if len(transport) > 0 {
				cloned["transport"] = cloneMap(transport)
			}
			if downloadURL := stringValue(download["url"]); downloadURL != "" {
				cloned["download_url"] = downloadURL
			}
			if title := stringValue(release["title"]); title != "" {
				cloned["title"] = title
			}
			if summary := stringValue(release["summary"]); summary != "" {
				cloned["summary"] = summary
			}
		}
		if origin, err := h.readOriginDocument(ctx, kind, name, version); err == nil && len(origin) > 0 {
			cloned["origin"] = origin
			if originDoc := artifactorigin.FromMap(origin); strings.TrimSpace(originDoc.StorageProfileID) != "" {
				cloned["storage_profile_id"] = originDoc.StorageProfileID
			}
		}
		out = append(out, cloned)
	}
	return out
}

func (h *Handler) readArtifactReleaseDetail(ctx context.Context, kind string, name string, version string) (map[string]any, error) {
	release, err := h.readReleaseManifest(ctx, kind, name, version)
	if err != nil {
		return nil, err
	}

	detail := cloneMap(release)
	detail["kind"] = firstNonEmpty(stringValue(detail["kind"]), kind)
	detail["name"] = firstNonEmpty(stringValue(detail["name"]), name)
	detail["version"] = firstNonEmpty(stringValue(detail["version"]), version)
	detail["channel"] = firstNonEmpty(stringValue(detail["channel"]), "stable")
	if index, indexErr := h.readArtifactIndex(ctx, kind, name); indexErr == nil {
		if entry := catalog.FindArtifactVersionEntry(parseVersionEntries(index["versions"]), version); entry != nil {
			if channels := catalog.ArtifactVersionChannels(entry); len(channels) > 0 {
				detail["channels"] = channels
				detail["channel"] = channels[0]
			}
		}
	}

	download := cloneMap(mapValue(detail["download"]))
	if downloadURL := stringValue(download["url"]); downloadURL != "" {
		detail["download_url"] = downloadURL
	}

	if origin, originErr := h.readOriginDocument(ctx, kind, name, version); originErr == nil && len(origin) > 0 {
		originDoc := artifactorigin.FromMap(origin)
		if len(mapValue(download["transport"])) == 0 {
			download["transport"] = artifactorigin.TransportMap(originDoc)
		}
		detail["origin"] = origin
		if strings.TrimSpace(originDoc.StorageProfileID) != "" {
			detail["storage_profile_id"] = originDoc.StorageProfileID
		}
	} else if len(mapValue(download["transport"])) == 0 {
		download["transport"] = artifactorigin.DefaultLocalMirrorTransport()
	}

	detail["download"] = download
	detail["transport"] = cloneMap(mapValue(download["transport"]))
	detail["size"] = download["size"]
	detail["sha256"] = stringValue(download["sha256"])
	detail["content_type"] = stringValue(download["content_type"])

	return detail, nil
}

func (h *Handler) readReleaseManifest(ctx context.Context, kind string, name string, version string) (map[string]any, error) {
	manifestPath := strings.Join([]string{"artifacts", kind, name, version, "manifest.json"}, "/")
	data, err := h.storage.Read(ctx, manifestPath)
	if err != nil {
		return nil, err
	}

	var manifest map[string]any
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	return manifest, nil
}

func (h *Handler) readOriginDocument(ctx context.Context, kind string, name string, version string) (map[string]any, error) {
	data, err := h.storage.Read(ctx, artifactorigin.DocumentPath(kind, name, version))
	if err != nil {
		return nil, err
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	return artifactorigin.FromMap(payload).ToMap(), nil
}

func (h *Handler) enrichLatestArtifactSummary(ctx context.Context, kind string, name string, index map[string]any) {
	version := stringValue(index["latest_version"])
	if version == "" {
		return
	}

	release, err := h.readReleaseManifest(ctx, kind, name, version)
	if err == nil {
		download := mapValue(release["download"])
		transport := mapValue(download["transport"])
		if len(transport) > 0 {
			index["latest_transport"] = transport
		}
		if downloadURL := stringValue(download["url"]); downloadURL != "" {
			index["latest_download_url"] = downloadURL
		}
	}

	origin, err := h.readOriginDocument(ctx, kind, name, version)
	if err == nil && len(origin) > 0 {
		index["latest_origin"] = origin
	}
}

func metadataFromReleaseManifest(release map[string]any) publish.Metadata {
	publisher := mapValue(release["publisher"])
	return publish.Metadata{
		Title:        stringValue(release["title"]),
		Summary:      stringValue(release["summary"]),
		Description:  stringValue(release["description"]),
		ReleaseNotes: stringValue(release["release_notes"]),
		Publisher: publish.Publisher{
			ID:   stringValue(publisher["id"]),
			Name: stringValue(publisher["name"]),
		},
		Labels:        stringSliceValue(release["labels"]),
		Compatibility: cloneMap(mapValue(release["compatibility"])),
		Permissions:   cloneMap(mapValue(release["permissions"])),
	}
}

func isRequestNotFound(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, "not found")
}

func parseVersionEntries(value any) []map[string]any {
	switch typed := value.(type) {
	case []map[string]any:
		return append([]map[string]any(nil), typed...)
	case []any:
		out := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			mapped, ok := item.(map[string]any)
			if !ok || mapped == nil {
				continue
			}
			out = append(out, cloneMap(mapped))
		}
		return out
	default:
		return []map[string]any{}
	}
}

func cloneMap(value map[string]any) map[string]any {
	if len(value) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(value))
	for key, item := range value {
		out[key] = item
	}
	return out
}

func mapValue(value any) map[string]any {
	mapped, _ := value.(map[string]any)
	if mapped == nil {
		return map[string]any{}
	}
	return mapped
}

func stringValue(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func stringSliceValue(value any) []string {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			text := stringValue(item)
			if text == "" {
				continue
			}
			out = append(out, text)
		}
		return out
	default:
		return []string{}
	}
}

func int64Value(value any) int64 {
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int8:
		return int64(typed)
	case int16:
		return int64(typed)
	case int32:
		return int64(typed)
	case int64:
		return typed
	case float32:
		return int64(typed)
	case float64:
		return int64(typed)
	default:
		return 0
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
