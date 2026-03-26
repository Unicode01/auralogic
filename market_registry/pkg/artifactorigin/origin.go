package artifactorigin

import (
	"context"
	"fmt"
	"path"
	"strings"

	"auralogic/market_registry/pkg/storage"
)

const (
	ProviderLocal         = "local"
	ProviderHTTP          = "http"
	ProviderGitHubRelease = "github_release"
	ProviderGitHubArchive = "github_archive"
	ProviderWebDAV        = "webdav"
	ProviderS3            = "s3"

	ModeMirror   = "mirror"
	ModeProxy    = "proxy"
	ModeRedirect = "redirect"

	CacheStatusReady = "ready"

	SyncStrategyManual   = "manual"
	SyncStrategyInterval = "interval"
	SyncStrategyWebhook  = "webhook"
	SyncStrategyLazy     = "lazy"
)

type Document struct {
	Version          int
	Provider         string
	Mode             string
	StorageProfileID string
	Locator          map[string]any
	Integrity        Integrity
	Sync             SyncState
	Cache            CacheState
}

type Integrity struct {
	SHA256 string
	Size   int64
}

type SyncState struct {
	Strategy        string
	IntervalSeconds int64
	LastSyncedAt    string
}

type CacheState struct {
	ArtifactPath string
	Status       string
}

type ResolveResult struct {
	Provider    string
	Mode        string
	StoragePath string
	RemoteURL   string
	Headers     map[string]string
	ContentType string
	Integrity   Integrity
}

type Resolver interface {
	Provider() string
	Resolve(ctx context.Context, store storage.Storage, doc Document) (ResolveResult, error)
}

type Registry struct {
	resolvers map[string]Resolver
}

type LocalResolver struct{}

type HTTPResolver struct{}

func NewRegistry(resolvers ...Resolver) *Registry {
	items := make(map[string]Resolver, len(resolvers))
	for _, resolver := range resolvers {
		if resolver == nil {
			continue
		}
		provider := strings.ToLower(strings.TrimSpace(resolver.Provider()))
		if provider == "" {
			continue
		}
		items[provider] = resolver
	}
	return &Registry{resolvers: items}
}

func NewDefaultRegistry() *Registry {
	return NewRegistry(LocalResolver{}, HTTPResolver{})
}

func DefaultLocalMirrorOrigin(artifactPath string, sha256 string, size int64) Document {
	return DefaultMirrorOrigin(ProviderLocal, "", artifactPath, sha256, size)
}

func DefaultMirrorOrigin(provider string, storageProfileID string, artifactPath string, sha256 string, size int64) Document {
	return NormalizeDocument(Document{
		Version:          1,
		Provider:         firstNonEmpty(provider, ProviderLocal),
		Mode:             ModeMirror,
		StorageProfileID: strings.TrimSpace(storageProfileID),
		Locator: map[string]any{
			"path": artifactPath,
		},
		Integrity: Integrity{
			SHA256: strings.ToLower(strings.TrimSpace(sha256)),
			Size:   size,
		},
		Sync: SyncState{
			Strategy: SyncStrategyManual,
		},
		Cache: CacheState{
			ArtifactPath: artifactPath,
			Status:       CacheStatusReady,
		},
	})
}

func NormalizeDocument(doc Document) Document {
	if doc.Version <= 0 {
		doc.Version = 1
	}
	doc.Provider = strings.ToLower(strings.TrimSpace(doc.Provider))
	doc.Mode = strings.ToLower(strings.TrimSpace(doc.Mode))
	doc.StorageProfileID = strings.TrimSpace(doc.StorageProfileID)
	if doc.Mode == "" {
		doc.Mode = ModeMirror
	}
	if doc.Locator == nil {
		doc.Locator = map[string]any{}
	} else {
		doc.Locator = cloneDynamicMap(doc.Locator)
	}
	doc.Integrity.SHA256 = strings.ToLower(strings.TrimSpace(doc.Integrity.SHA256))
	doc.Sync.Strategy = strings.ToLower(strings.TrimSpace(doc.Sync.Strategy))
	if doc.Sync.Strategy == "" {
		doc.Sync.Strategy = SyncStrategyManual
	}
	doc.Cache.ArtifactPath = strings.TrimSpace(strings.ReplaceAll(doc.Cache.ArtifactPath, "\\", "/"))
	doc.Cache.Status = strings.ToLower(strings.TrimSpace(doc.Cache.Status))
	if doc.Provider == "" {
		switch {
		case doc.Cache.ArtifactPath != "", stringMapValue(doc.Locator, "path") != "":
			doc.Provider = ProviderLocal
		case stringMapValue(doc.Locator, "url") != "":
			doc.Provider = ProviderHTTP
		default:
			doc.Provider = ProviderLocal
		}
	}
	return doc
}

func FromMap(payload map[string]any) Document {
	if payload == nil {
		return NormalizeDocument(Document{})
	}
	integrity := mapMapValue(payload["integrity"])
	syncValue := mapMapValue(payload["sync"])
	cache := mapMapValue(payload["cache"])
	return NormalizeDocument(Document{
		Version:          intValue(payload["version"]),
		Provider:         stringValue(payload["provider"]),
		Mode:             stringValue(payload["mode"]),
		StorageProfileID: stringValue(payload["storage_profile_id"]),
		Locator:          mapMapValue(payload["locator"]),
		Integrity: Integrity{
			SHA256: stringValue(integrity["sha256"]),
			Size:   int64Value(integrity["size"]),
		},
		Sync: SyncState{
			Strategy:        stringValue(syncValue["strategy"]),
			IntervalSeconds: int64Value(syncValue["interval_seconds"]),
			LastSyncedAt:    stringValue(syncValue["last_synced_at"]),
		},
		Cache: CacheState{
			ArtifactPath: stringValue(cache["artifact_path"]),
			Status:       stringValue(cache["status"]),
		},
	})
}

func (d Document) ToMap() map[string]any {
	doc := NormalizeDocument(d)
	out := map[string]any{
		"version":  doc.Version,
		"provider": doc.Provider,
		"mode":     doc.Mode,
	}
	if doc.StorageProfileID != "" {
		out["storage_profile_id"] = doc.StorageProfileID
	}
	if len(doc.Locator) > 0 {
		out["locator"] = cloneDynamicMap(doc.Locator)
	}
	if doc.Integrity.SHA256 != "" || doc.Integrity.Size > 0 {
		out["integrity"] = map[string]any{
			"sha256": doc.Integrity.SHA256,
			"size":   doc.Integrity.Size,
		}
	}
	if doc.Sync.Strategy != "" || doc.Sync.IntervalSeconds > 0 || doc.Sync.LastSyncedAt != "" {
		out["sync"] = map[string]any{
			"strategy":         doc.Sync.Strategy,
			"interval_seconds": doc.Sync.IntervalSeconds,
			"last_synced_at":   doc.Sync.LastSyncedAt,
		}
	}
	if doc.Cache.ArtifactPath != "" || doc.Cache.Status != "" {
		out["cache"] = map[string]any{
			"artifact_path": doc.Cache.ArtifactPath,
			"status":        doc.Cache.Status,
		}
	}
	return out
}

func TransportMap(doc Document) map[string]any {
	normalized := NormalizeDocument(doc)
	out := map[string]any{
		"provider": normalized.Provider,
		"mode":     normalized.Mode,
	}
	if normalized.StorageProfileID != "" {
		out["storage_profile_id"] = normalized.StorageProfileID
	}
	return out
}

func DefaultLocalMirrorTransport() map[string]any {
	return TransportMap(Document{Provider: ProviderLocal, Mode: ModeMirror})
}

func DocumentPath(kind string, name string, version string) string {
	return path.Join(
		"artifacts",
		strings.TrimSpace(kind),
		strings.TrimSpace(name),
		strings.TrimSpace(version),
		"origin.json",
	)
}

func (d Document) MirrorArtifactPath() string {
	doc := NormalizeDocument(d)
	if doc.Cache.ArtifactPath != "" {
		return doc.Cache.ArtifactPath
	}
	return stringMapValue(doc.Locator, "path")
}

func (d Document) ExternalURL() string {
	doc := NormalizeDocument(d)
	return stringMapValue(doc.Locator, "url")
}

func (d Document) HeaderMap() map[string]string {
	doc := NormalizeDocument(d)
	headersValue := doc.Locator["headers"]
	switch typed := headersValue.(type) {
	case map[string]string:
		return cloneStringMap(typed)
	case map[string]any:
		out := make(map[string]string, len(typed))
		for key, value := range typed {
			headerValue := stringValue(value)
			if headerKey := strings.TrimSpace(key); headerKey != "" && headerValue != "" {
				out[headerKey] = headerValue
			}
		}
		return out
	default:
		return map[string]string{}
	}
}

func (d Document) ResolvedContentType(defaultValue string) string {
	doc := NormalizeDocument(d)
	if contentType := stringMapValue(doc.Locator, "content_type"); contentType != "" {
		return contentType
	}
	return strings.TrimSpace(defaultValue)
}

func (r *Registry) Resolve(ctx context.Context, store storage.Storage, doc Document) (ResolveResult, error) {
	normalized := NormalizeDocument(doc)
	if normalized.Mode == ModeMirror {
		if storagePath := normalized.MirrorArtifactPath(); storagePath != "" {
			return ResolveResult{
				Provider:    firstNonEmpty(normalized.Provider, ProviderLocal),
				Mode:        normalized.Mode,
				StoragePath: storagePath,
				ContentType: normalized.ResolvedContentType(""),
				Integrity:   normalized.Integrity,
				Headers:     map[string]string{},
			}, nil
		}
	}
	if r == nil || len(r.resolvers) == 0 {
		r = NewDefaultRegistry()
	}
	resolver, ok := r.resolvers[normalized.Provider]
	if !ok {
		return ResolveResult{}, fmt.Errorf("artifact origin provider %q is not supported", normalized.Provider)
	}
	result, err := resolver.Resolve(ctx, store, normalized)
	if err != nil {
		return ResolveResult{}, err
	}
	result.Provider = firstNonEmpty(result.Provider, normalized.Provider)
	result.Mode = firstNonEmpty(result.Mode, normalized.Mode)
	if result.ContentType == "" {
		result.ContentType = normalized.ResolvedContentType("")
	}
	if result.Integrity.SHA256 == "" {
		result.Integrity = normalized.Integrity
	}
	if result.Headers == nil {
		result.Headers = map[string]string{}
	}
	return result, nil
}

func (LocalResolver) Provider() string {
	return ProviderLocal
}

func (LocalResolver) Resolve(_ context.Context, _ storage.Storage, doc Document) (ResolveResult, error) {
	normalized := NormalizeDocument(doc)
	storagePath := normalized.MirrorArtifactPath()
	if storagePath == "" {
		return ResolveResult{}, fmt.Errorf("local artifact origin path is required")
	}
	return ResolveResult{
		Provider:    normalized.Provider,
		Mode:        normalized.Mode,
		StoragePath: storagePath,
		ContentType: normalized.ResolvedContentType("application/zip"),
		Integrity:   normalized.Integrity,
		Headers:     map[string]string{},
	}, nil
}

func (HTTPResolver) Provider() string {
	return ProviderHTTP
}

func (HTTPResolver) Resolve(_ context.Context, _ storage.Storage, doc Document) (ResolveResult, error) {
	normalized := NormalizeDocument(doc)
	remoteURL := normalized.ExternalURL()
	if remoteURL == "" {
		return ResolveResult{}, fmt.Errorf("http artifact origin url is required")
	}
	result := ResolveResult{
		Provider:    normalized.Provider,
		Mode:        normalized.Mode,
		RemoteURL:   remoteURL,
		ContentType: normalized.ResolvedContentType("application/zip"),
		Integrity:   normalized.Integrity,
		Headers:     normalized.HeaderMap(),
	}
	if normalized.Mode == ModeMirror {
		storagePath := normalized.MirrorArtifactPath()
		if storagePath == "" {
			return ResolveResult{}, fmt.Errorf("http mirror artifact origin cache path is required")
		}
		result.StoragePath = storagePath
	}
	return result, nil
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

func cloneStringMap(value map[string]string) map[string]string {
	if len(value) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(value))
	for key, item := range value {
		out[key] = item
	}
	return out
}

func mapMapValue(value any) map[string]any {
	mapped, _ := value.(map[string]any)
	if len(mapped) == 0 {
		return map[string]any{}
	}
	return cloneDynamicMap(mapped)
}

func stringMapValue(value map[string]any, key string) string {
	if len(value) == 0 {
		return ""
	}
	return stringValue(value[key])
}

func stringValue(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func intValue(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int8:
		return int(typed)
	case int16:
		return int(typed)
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float32:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
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
