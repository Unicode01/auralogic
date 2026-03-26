package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path"
	"sort"
	"strings"
	"time"

	"auralogic/market_registry/pkg/artifactorigin"
	"auralogic/market_registry/pkg/registrysettings"
	"auralogic/market_registry/pkg/signing"
	"auralogic/market_registry/pkg/storage"
)

var ErrNotFound = errors.New("catalog: not found")

type RuntimeStoreConfig struct {
	SourceID   string
	SourceName string
	KeyID      string
	Settings   *registrysettings.Service
	Now        func() time.Time
}

type RuntimeStore struct {
	storage    storage.Storage
	signing    *signing.Service
	settings   *registrysettings.Service
	origins    *artifactorigin.Registry
	sourceID   string
	sourceName string
	keyID      string
	now        func() time.Time
}

type artifactRef struct {
	Kind string
	Name string
}

func NewRuntimeStore(store storage.Storage, sign *signing.Service, cfg RuntimeStoreConfig) *RuntimeStore {
	if store == nil {
		panic("catalog: storage is required")
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &RuntimeStore{
		storage:    store,
		signing:    sign,
		settings:   cfg.Settings,
		origins:    artifactorigin.NewDefaultRegistry(),
		sourceID:   firstNonEmpty(cfg.SourceID, "official"),
		sourceName: firstNonEmpty(cfg.SourceName, "AuraLogic Official Source"),
		keyID:      strings.TrimSpace(cfg.KeyID),
		now:        now,
	}
}

func (s *RuntimeStore) BuildSourceDocument(ctx context.Context, baseURL string) (map[string]any, error) {
	source := map[string]any{
		"api_version":  "v1",
		"source_id":    s.sourceID,
		"name":         s.sourceName,
		"base_url":     strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		"generated_at": s.now().UTC(),
		"capabilities": map[string]any{
			"artifact_kinds":   []string{},
			"governance_modes": []string{"host_managed"},
			"supports_delta":   false,
		},
		"compatibility": map[string]any{
			"min_host_version":        "1.0.0",
			"min_host_bridge_version": hostBridgeVersion,
		},
	}

	if existing, err := s.readJSON(ctx, "index/source.json"); err == nil {
		mergeSourceDocument(source, existing)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	refs, err := s.listArtifactRefs(ctx)
	if err != nil {
		return nil, err
	}
	if capabilities, ok := source["capabilities"].(map[string]any); ok {
		capabilities["artifact_kinds"] = collectArtifactKinds(refs)
	}

	if capabilities, ok := source["capabilities"].(map[string]any); ok {
		capabilities["supports_signature"] = false
		if s.signing != nil && s.keyID != "" {
			publicKey, exportErr := s.signing.ExportPublicKey(s.keyID)
			if exportErr == nil {
				source["signing"] = map[string]any{
					"algorithm":  "ed25519",
					"key_id":     s.keyID,
					"public_key": publicKey,
				}
				capabilities["supports_signature"] = true
			}
		}
	}

	return source, nil
}

func (s *RuntimeStore) ListCatalog(ctx context.Context, query Query, baseURL string) ([]map[string]any, int, error) {
	items, err := s.loadCatalogItems(ctx, baseURL)
	if err != nil {
		return nil, 0, err
	}
	items = filterCatalogItems(items, query)
	total := len(items)
	offset := maxInt(query.Offset, 0)
	limit := query.Limit
	if limit <= 0 {
		limit = 20
	}
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return items[offset:end], total, nil
}

func (s *RuntimeStore) BuildCatalogSnapshot(ctx context.Context, baseURL string) (map[string]any, error) {
	items, err := s.buildCatalogItemsFromIndexes(ctx, baseURL)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"generated_at": s.now().UTC().Format(time.RFC3339),
		"items":        items,
		"total":        len(items),
	}, nil
}

func (s *RuntimeStore) BuildArtifactDocument(ctx context.Context, kind string, name string) (map[string]any, error) {
	index, err := s.loadArtifactIndex(ctx, kind, name)
	if err != nil {
		return nil, err
	}

	latestVersion := latestVersionFromIndex(index)
	governance := map[string]any{}
	publisher := map[string]any{}
	description := firstNonEmptyString(index["description"])
	if latestVersion != "" {
		release, releaseErr := s.BuildReleaseDocument(ctx, kind, name, latestVersion, "")
		if releaseErr != nil && !errors.Is(releaseErr, ErrNotFound) {
			return nil, releaseErr
		}
		if releaseErr == nil {
			governance = cloneMap(mapValue(release["governance"]))
			publisher = cloneMap(mapValue(release["publisher"]))
			description = firstNonEmptyString(description, release["description"])
		}
	}

	versions := normalizeArtifactVersionList(index["versions"])
	documentVersions := make([]map[string]any, 0, len(versions))
	for _, version := range versions {
		documentVersions = append(documentVersions, map[string]any{
			"version":      firstNonEmptyString(version["version"]),
			"channel":      firstNonEmpty(ArtifactVersionPrimaryChannel(version), "stable"),
			"channels":     ArtifactVersionChannels(version),
			"published_at": firstNonEmptyString(version["published_at"]),
		})
	}

	document := map[string]any{
		"kind":           firstNonEmptyString(index["kind"], kind),
		"name":           firstNonEmptyString(index["name"], name),
		"title":          firstNonEmptyString(index["title"], name),
		"summary":        firstNonEmptyString(index["summary"]),
		"description":    description,
		"latest_version": latestVersion,
		"channels":       collectChannelsFromIndex(versions),
		"governance":     governance,
		"versions":       documentVersions,
		"releases":       documentVersions,
		"labels":         stringSliceValue(index["labels"]),
	}
	if len(publisher) > 0 {
		document["publisher"] = publisher
	}
	return document, nil
}

func (s *RuntimeStore) BuildReleaseDocument(ctx context.Context, kind string, name string, version string, baseURL string) (map[string]any, error) {
	manifestPath := path.Join("artifacts", kind, name, version, "manifest.json")
	document, err := s.readJSON(ctx, manifestPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	normalized := normalizeReleaseDocument(kind, name, version, baseURL, document)
	if index, indexErr := s.loadArtifactIndex(ctx, kind, name); indexErr == nil {
		if entry := FindArtifactVersionEntry(normalizeArtifactVersionList(index["versions"]), version); entry != nil {
			channels := ArtifactVersionChannels(entry)
			if len(channels) > 0 {
				normalized["channels"] = channels
				normalized["channel"] = channels[0]
			}
		}
	}
	origin, exists, originErr := s.loadOriginDocument(ctx, kind, name, version)
	if originErr != nil {
		return nil, originErr
	}
	applyReleaseDownloadTransport(normalized, origin, exists)
	return normalized, nil
}

func (s *RuntimeStore) ReadReleaseArtifact(ctx context.Context, kind string, name string, version string) ([]byte, string, error) {
	contentType := "application/zip"
	manifest, err := s.BuildReleaseDocument(ctx, kind, name, version, "")
	if err == nil {
		contentType = firstNonEmpty(firstNonEmptyString(mapValue(manifest["download"])["content_type"]), contentType)
	}
	resolved, err := s.resolveReleaseArtifact(ctx, kind, name, version, contentType)
	if err != nil {
		return nil, "", err
	}
	switch resolved.Mode {
	case artifactorigin.ModeMirror, "":
		payload, readErr := resolved.Storage.Read(ctx, resolved.StoragePath)
		if readErr != nil {
			if errors.Is(readErr, fs.ErrNotExist) {
				return nil, "", ErrNotFound
			}
			return nil, "", fmt.Errorf("read artifact payload: %w", readErr)
		}
		return payload, resolved.ContentType, nil
	case artifactorigin.ModeProxy:
		return s.readRemoteReleaseArtifact(ctx, resolved.ResolveResult)
	case artifactorigin.ModeRedirect:
		return nil, "", fmt.Errorf("release artifact requires redirect resolution")
	default:
		return nil, "", fmt.Errorf("unsupported artifact origin mode %q", resolved.Mode)
	}
}

func (s *RuntimeStore) loadArtifactIndex(ctx context.Context, kind string, name string) (map[string]any, error) {
	indexPath := path.Join("index", "artifacts", kind, name, "index.json")
	index, err := s.readJSON(ctx, indexPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return NormalizeArtifactIndex(kind, name, index), nil
}

func (s *RuntimeStore) listArtifactRefs(ctx context.Context) ([]artifactRef, error) {
	paths, err := s.storage.List(ctx, "index/artifacts")
	if err != nil {
		return nil, fmt.Errorf("list artifact indexes: %w", err)
	}
	seen := map[string]struct{}{}
	refs := make([]artifactRef, 0, len(paths))
	for _, itemPath := range paths {
		kind, name, ok := ParseArtifactIndexPath(itemPath)
		if !ok {
			continue
		}
		key := storeKey(kind, name)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		refs = append(refs, artifactRef{Kind: kind, Name: name})
	}
	sort.SliceStable(refs, func(i int, j int) bool {
		if refs[i].Kind == refs[j].Kind {
			return refs[i].Name < refs[j].Name
		}
		return refs[i].Kind < refs[j].Kind
	})
	return refs, nil
}

func (s *RuntimeStore) loadCatalogItems(ctx context.Context, baseURL string) ([]map[string]any, error) {
	snapshot, err := s.readJSON(ctx, "index/catalog.json")
	if err == nil {
		if items := normalizeCatalogSnapshotItems(snapshot["items"]); len(items) > 0 {
			return items, nil
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	return s.buildCatalogItemsFromIndexes(ctx, baseURL)
}

func (s *RuntimeStore) buildCatalogItemsFromIndexes(ctx context.Context, baseURL string) ([]map[string]any, error) {
	refs, err := s.listArtifactRefs(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]map[string]any, 0, len(refs))
	for _, ref := range refs {
		index, err := s.loadArtifactIndex(ctx, ref.Kind, ref.Name)
		if err != nil {
			return nil, err
		}

		version := latestVersionFromIndex(index)
		if version == "" {
			continue
		}

		release, err := s.BuildReleaseDocument(ctx, ref.Kind, ref.Name, version, baseURL)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				continue
			}
			return nil, err
		}

		item := map[string]any{
			"kind":           ref.Kind,
			"name":           ref.Name,
			"title":          firstNonEmptyString(index["title"], release["title"], ref.Name),
			"summary":        firstNonEmptyString(index["summary"], release["summary"]),
			"description":    firstNonEmptyString(index["description"], release["description"]),
			"latest_version": version,
			"channel":        firstNonEmptyString(release["channel"], "stable"),
			"governance":     cloneMap(mapValue(release["governance"])),
			"compatibility":  cloneMap(mapValue(release["compatibility"])),
			"permissions":    cloneMap(mapValue(release["permissions"])),
			"download": map[string]any{
				"size":   mapValue(release["download"])["size"],
				"sha256": mapValue(release["download"])["sha256"],
			},
			"labels":       stringSliceValue(firstNonEmptyValue(index["labels"], release["labels"])),
			"published_at": firstNonEmptyString(release["published_at"]),
		}
		if latestEntry := FindArtifactVersionEntry(normalizeArtifactVersionList(index["versions"]), version); latestEntry != nil {
			item["channel"] = firstNonEmpty(ArtifactVersionPrimaryChannel(latestEntry), firstNonEmptyString(item["channel"]))
			item["channels"] = ArtifactVersionChannels(latestEntry)
		}

		if publisher := cloneMap(mapValue(release["publisher"])); len(publisher) > 0 {
			item["publisher"] = publisher
		}
		if iconURL := firstNonEmptyString(release["icon_url"], mapValue(release["ui"])["icon_url"]); iconURL != "" {
			item["icon_url"] = iconURL
		}

		items = append(items, item)
	}
	return items, nil
}

func (s *RuntimeStore) readJSON(ctx context.Context, itemPath string) (map[string]any, error) {
	payload, err := s.storage.Read(ctx, itemPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", itemPath, err)
	}
	var out map[string]any
	if err := json.Unmarshal(payload, &out); err != nil {
		return nil, fmt.Errorf("decode %s: %w", itemPath, err)
	}
	return out, nil
}

func (s *RuntimeStore) resolveArtifactPayloadPath(ctx context.Context, kind string, name string, version string) (string, error) {
	preferred := path.Join("artifacts", kind, name, version, fmt.Sprintf("%s-%s.zip", name, version))
	if exists, err := s.storage.Exists(ctx, preferred); err == nil && exists {
		return preferred, nil
	}

	files, err := s.storage.List(ctx, path.Join("artifacts", kind, name, version))
	if err != nil {
		return "", fmt.Errorf("list artifact payloads: %w", err)
	}
	for _, file := range files {
		if strings.HasSuffix(strings.ToLower(strings.TrimSpace(file)), ".zip") {
			return file, nil
		}
	}
	return "", ErrNotFound
}

func (s *RuntimeStore) ResolveReleaseDownloadRedirect(ctx context.Context, kind string, name string, version string) (string, error) {
	origin, exists, err := s.loadOriginDocument(ctx, kind, name, version)
	if err != nil || !exists {
		return "", err
	}
	targetStore, err := s.resolveOriginStorage(ctx, origin)
	if err != nil {
		return "", err
	}
	resolved, err := s.origins.Resolve(ctx, targetStore, origin)
	if err != nil {
		return "", err
	}
	if strings.EqualFold(resolved.Mode, artifactorigin.ModeRedirect) {
		return strings.TrimSpace(resolved.RemoteURL), nil
	}
	return "", nil
}

func (s *RuntimeStore) loadOriginDocument(ctx context.Context, kind string, name string, version string) (artifactorigin.Document, bool, error) {
	itemPath := artifactorigin.DocumentPath(kind, name, version)
	payload, err := s.storage.Read(ctx, itemPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return artifactorigin.Document{}, false, nil
		}
		return artifactorigin.Document{}, false, fmt.Errorf("read %s: %w", itemPath, err)
	}
	var document map[string]any
	if err := json.Unmarshal(payload, &document); err != nil {
		return artifactorigin.Document{}, false, fmt.Errorf("decode %s: %w", itemPath, err)
	}
	return artifactorigin.FromMap(document), true, nil
}

func (s *RuntimeStore) resolveReleaseArtifact(
	ctx context.Context,
	kind string,
	name string,
	version string,
	contentType string,
) (resolvedReleaseArtifact, error) {
	origin, exists, err := s.loadOriginDocument(ctx, kind, name, version)
	if err != nil {
		return resolvedReleaseArtifact{}, err
	}
	if exists {
		targetStore, storageErr := s.resolveOriginStorage(ctx, origin)
		if storageErr != nil {
			return resolvedReleaseArtifact{}, storageErr
		}
		resolved, resolveErr := s.origins.Resolve(ctx, targetStore, origin)
		if resolveErr != nil {
			return resolvedReleaseArtifact{}, resolveErr
		}
		if resolved.ContentType == "" {
			resolved.ContentType = contentType
		}
		return resolvedReleaseArtifact{
			ResolveResult: resolved,
			Storage:       targetStore,
		}, nil
	}
	artifactPath, err := s.resolveArtifactPayloadPath(ctx, kind, name, version)
	if err != nil {
		return resolvedReleaseArtifact{}, err
	}
	return resolvedReleaseArtifact{
		ResolveResult: artifactorigin.ResolveResult{
			Provider:    artifactorigin.ProviderLocal,
			Mode:        artifactorigin.ModeMirror,
			StoragePath: artifactPath,
			ContentType: contentType,
			Headers:     map[string]string{},
		},
		Storage: s.storage,
	}, nil
}

type resolvedReleaseArtifact struct {
	artifactorigin.ResolveResult
	Storage storage.Storage
}

func (s *RuntimeStore) resolveOriginStorage(ctx context.Context, origin artifactorigin.Document) (storage.Storage, error) {
	profileID := strings.TrimSpace(origin.StorageProfileID)
	if profileID == "" || strings.EqualFold(profileID, registrysettings.CanonicalArtifactStorageProfileID) {
		return s.storage, nil
	}
	if s.settings == nil {
		return nil, fmt.Errorf("artifact storage profile %q is not configured", profileID)
	}
	resolved, err := s.settings.ResolveArtifactStorage(ctx, profileID)
	if err != nil {
		return nil, fmt.Errorf("resolve artifact storage profile %q: %w", profileID, err)
	}
	return resolved.Store, nil
}

func (s *RuntimeStore) readRemoteReleaseArtifact(
	ctx context.Context,
	resolved artifactorigin.ResolveResult,
) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, resolved.RemoteURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("build remote artifact request: %w", err)
	}
	for key, value := range resolved.Headers {
		if strings.TrimSpace(key) != "" && strings.TrimSpace(value) != "" {
			req.Header.Set(key, value)
		}
	}
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("download remote artifact: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("download remote artifact failed with status %d", resp.StatusCode)
	}

	const maxRemoteArtifactBytes = int64(256 * 1024 * 1024)
	payload, err := io.ReadAll(io.LimitReader(resp.Body, maxRemoteArtifactBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("read remote artifact: %w", err)
	}
	if int64(len(payload)) > maxRemoteArtifactBytes {
		return nil, "", fmt.Errorf("remote artifact exceeds size limit")
	}
	if resolved.Integrity.Size > 0 && int64(len(payload)) != resolved.Integrity.Size {
		return nil, "", fmt.Errorf("remote artifact size mismatch")
	}
	if resolved.Integrity.SHA256 != "" {
		sum := sha256.Sum256(payload)
		checksum := hex.EncodeToString(sum[:])
		if !strings.EqualFold(checksum, resolved.Integrity.SHA256) {
			return nil, "", fmt.Errorf("remote artifact checksum mismatch")
		}
	}
	contentType := firstNonEmpty(
		strings.TrimSpace(resp.Header.Get("Content-Type")),
		resolved.ContentType,
		"application/octet-stream",
	)
	return payload, contentType, nil
}

func collectArtifactKinds(refs []artifactRef) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		kind := strings.TrimSpace(ref.Kind)
		if kind == "" {
			continue
		}
		if _, exists := seen[kind]; exists {
			continue
		}
		seen[kind] = struct{}{}
		out = append(out, kind)
	}
	sort.Strings(out)
	return out
}

func collectChannelsFromIndex(versions []map[string]any) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(versions))
	for _, version := range versions {
		for _, channel := range ArtifactVersionChannels(version) {
			if _, exists := seen[channel]; exists {
				continue
			}
			seen[channel] = struct{}{}
			out = append(out, channel)
		}
	}
	return out
}

func normalizeReleaseDocument(kind string, name string, version string, baseURL string, document map[string]any) map[string]any {
	normalized := cloneDynamicMap(document)
	normalized["artifact_id"] = firstNonEmptyString(normalized["artifact_id"], fmt.Sprintf("%s:%s", kind, name))
	normalized["kind"] = firstNonEmptyString(normalized["kind"], kind)
	normalized["name"] = firstNonEmptyString(normalized["name"], name)
	normalized["version"] = firstNonEmptyString(normalized["version"], version)
	normalized["channel"] = firstNonEmptyString(normalized["channel"], "stable")
	normalized["governance"] = cloneMap(mapValue(normalized["governance"]))
	normalized["compatibility"] = cloneMap(mapValue(normalized["compatibility"]))
	normalized["permissions"] = cloneMap(mapValue(normalized["permissions"]))
	normalized["install"] = cloneMap(mapValue(normalized["install"]))
	normalized["targets"] = cloneMap(mapValue(normalized["targets"]))
	normalized["template"] = cloneMap(mapValue(normalized["template"]))
	normalized["docs"] = cloneMap(mapValue(normalized["docs"]))
	normalized["ui"] = cloneMap(mapValue(normalized["ui"]))
	normalized["publisher"] = cloneMap(mapValue(normalized["publisher"]))
	normalized["signature"] = cloneMap(mapValue(normalized["signature"]))
	normalized["labels"] = stringSliceValue(normalized["labels"])

	download := cloneMap(mapValue(normalized["download"]))
	download["url"] = releaseDownloadURL(strings.TrimSpace(baseURL), kind, name, version)
	download["content_type"] = firstNonEmptyString(download["content_type"], "application/zip")
	download["filename"] = firstNonEmptyString(
		download["filename"],
		releaseDownloadFilename(name, version, firstNonEmptyString(download["content_type"], "application/zip")),
	)
	if len(mapValue(download["transport"])) == 0 {
		download["transport"] = artifactorigin.DefaultLocalMirrorTransport()
	}
	normalized["download"] = download

	if iconURL := firstNonEmptyString(normalized["icon_url"], mapValue(normalized["ui"])["icon_url"]); iconURL != "" {
		normalized["icon_url"] = iconURL
	}
	return normalized
}

func applyReleaseDownloadTransport(document map[string]any, origin artifactorigin.Document, exists bool) {
	if document == nil {
		return
	}
	download := cloneMap(mapValue(document["download"]))
	if len(mapValue(download["transport"])) == 0 {
		if exists {
			download["transport"] = artifactorigin.TransportMap(origin)
		} else {
			download["transport"] = artifactorigin.DefaultLocalMirrorTransport()
		}
	}
	document["download"] = download
}

func mergeSourceDocument(target map[string]any, incoming map[string]any) {
	if incoming == nil {
		return
	}
	if sourceID := firstNonEmptyString(incoming["source_id"]); sourceID != "" {
		target["source_id"] = sourceID
	}
	if name := firstNonEmptyString(incoming["name"]); name != "" {
		target["name"] = name
	}
	if capabilities := mapValue(incoming["capabilities"]); len(capabilities) > 0 {
		target["capabilities"] = mergeMap(target["capabilities"], capabilities)
	}
	if compatibility := mapValue(incoming["compatibility"]); len(compatibility) > 0 {
		target["compatibility"] = mergeMap(target["compatibility"], compatibility)
	}
}

func releaseDownloadURL(baseURL string, kind string, name string, version string) string {
	relative := fmt.Sprintf("/v1/artifacts/%s/%s/releases/%s/download", kind, name, version)
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		return relative
	}
	return base + relative
}

func mapValue(value any) map[string]any {
	mapped, ok := value.(map[string]any)
	if !ok || mapped == nil {
		return map[string]any{}
	}
	return mapped
}

func mergeMap(base any, overlay map[string]any) map[string]any {
	out := cloneMap(mapValue(base))
	for key, value := range overlay {
		out[key] = value
	}
	return out
}

func cloneDynamicMap(value map[string]any) map[string]any {
	if len(value) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(value))
	for key, item := range value {
		out[key] = item
	}
	return out
}

func stringSliceValue(value any) []string {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			text, _ := item.(string)
			if trimmed := strings.TrimSpace(text); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		return out
	default:
		return []string{}
	}
}

func firstNonEmptyValue(values ...any) any {
	for _, value := range values {
		switch typed := value.(type) {
		case nil:
			continue
		case string:
			if strings.TrimSpace(typed) == "" {
				continue
			}
			return typed
		case []string:
			if len(typed) == 0 {
				continue
			}
			return typed
		case []any:
			if len(typed) == 0 {
				continue
			}
			return typed
		default:
			return value
		}
	}
	return nil
}

func filterCatalogItems(items []map[string]any, query Query) []map[string]any {
	if len(items) == 0 {
		return []map[string]any{}
	}
	filtered := make([]map[string]any, 0, len(items))
	searchTerm := strings.TrimSpace(query.Search)
	for _, item := range items {
		if query.Kind != "" && !strings.EqualFold(firstNonEmptyString(item["kind"]), query.Kind) {
			continue
		}
		if query.Channel != "" && !ArtifactVersionMatchesChannel(item, query.Channel) {
			continue
		}
		if query.Runtime != "" && !strings.EqualFold(firstNonEmptyString(mapValue(item["compatibility"])["runtime"]), query.Runtime) {
			continue
		}
		searchable := []string{
			firstNonEmptyString(item["kind"]),
			firstNonEmptyString(item["name"]),
			firstNonEmptyString(item["title"]),
			firstNonEmptyString(item["summary"]),
			firstNonEmptyString(item["description"]),
		}
		searchable = append(searchable, ArtifactVersionChannels(item)...)
		if searchTerm != "" && !matchesCatalogSearch(searchable, searchTerm) {
			continue
		}
		filtered = append(filtered, cloneDynamicMap(item))
	}
	return filtered
}

func normalizeCatalogSnapshotItems(value any) []map[string]any {
	switch typed := value.(type) {
	case []map[string]any:
		out := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, normalizeCatalogSnapshotItem(cloneDynamicMap(item)))
		}
		return out
	case []any:
		out := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			mapped, ok := item.(map[string]any)
			if !ok || mapped == nil {
				continue
			}
			out = append(out, normalizeCatalogSnapshotItem(cloneDynamicMap(mapped)))
		}
		return out
	default:
		return []map[string]any{}
	}
}

func normalizeCatalogSnapshotItem(item map[string]any) map[string]any {
	if len(item) == 0 {
		return map[string]any{}
	}
	channels := ArtifactVersionChannels(item)
	if len(channels) > 0 {
		item["channels"] = append([]string(nil), channels...)
		item["channel"] = channels[0]
	}
	return item
}
