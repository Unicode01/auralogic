package publish

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"auralogic/market_registry/pkg/artifactmanifest"
	"auralogic/market_registry/pkg/artifactorigin"
	"auralogic/market_registry/pkg/registrysettings"
	"auralogic/market_registry/pkg/signing"
	"auralogic/market_registry/pkg/storage"
)

type Request struct {
	Kind                     string
	Name                     string
	Version                  string
	Channel                  string
	ArtifactStorageProfileID string
	ArtifactZip              []byte
	Metadata                 Metadata
}

type Metadata struct {
	Title         string         `json:"title"`
	Summary       string         `json:"summary"`
	Description   string         `json:"description"`
	ReleaseNotes  string         `json:"release_notes"`
	Publisher     Publisher      `json:"publisher"`
	Labels        []string       `json:"labels"`
	Compatibility map[string]any `json:"compatibility,omitempty"`
	Permissions   map[string]any `json:"permissions,omitempty"`
}

type Publisher struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Options struct {
	SourceID   string
	SourceName string
	BaseURL    string
	Settings   *registrysettings.Service
}

type RequestError struct {
	Message string
}

func (e *RequestError) Error() string {
	return e.Message
}

func IsRequestError(err error) bool {
	var target *RequestError
	return errors.As(err, &target)
}

type Service struct {
	storage    storage.Storage
	signing    *signing.Service
	settings   *registrysettings.Service
	keyID      string
	sourceID   string
	sourceName string
	baseURL    string
}

type DeleteResult struct {
	Kind              string   `json:"kind"`
	Name              string   `json:"name"`
	Version           string   `json:"version,omitempty"`
	DeletedFiles      int      `json:"deleted_files"`
	DeletedVersions   []string `json:"deleted_versions,omitempty"`
	RemainingVersions int      `json:"remaining_versions"`
	LatestVersion     string   `json:"latest_version,omitempty"`
	ArtifactDeleted   bool     `json:"artifact_deleted"`
}

var supportedArtifactKinds = map[string]struct{}{
	"plugin_package":         {},
	"payment_package":        {},
	"email_template":         {},
	"landing_page_template":  {},
	"invoice_template":       {},
	"auth_branding_template": {},
	"page_rule_pack":         {},
}

var artifactIdentifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
var artifactVersionPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]{0,127}$`)

func NewService(store storage.Storage, sign *signing.Service, keyID string) *Service {
	return NewServiceWithOptions(store, sign, keyID, Options{})
}

func NewServiceWithOptions(store storage.Storage, sign *signing.Service, keyID string, opts Options) *Service {
	return &Service{
		storage:    store,
		signing:    sign,
		settings:   opts.Settings,
		keyID:      keyID,
		sourceID:   firstNonEmpty(opts.SourceID, "official"),
		sourceName: firstNonEmpty(opts.SourceName, "AuraLogic Official Source"),
		baseURL:    strings.TrimRight(strings.TrimSpace(opts.BaseURL), "/"),
	}
}

func (s *Service) Publish(ctx context.Context, req Request) error {
	return s.publishWithOrigin(ctx, req, artifactorigin.Document{})
}

func (s *Service) PublishWithOrigin(ctx context.Context, req Request, origin artifactorigin.Document) error {
	return s.publishWithOrigin(ctx, req, origin)
}

func (s *Service) ReleaseExists(ctx context.Context, kind string, name string, version string) (bool, error) {
	normalizedKind := strings.TrimSpace(kind)
	normalizedName := strings.TrimSpace(name)
	normalizedVersion := strings.TrimSpace(version)
	if err := validateArtifactKind(normalizedKind); err != nil {
		return false, err
	}
	if err := validateArtifactIdentifier("name", normalizedName, artifactIdentifierPattern); err != nil {
		return false, err
	}
	if err := validateArtifactIdentifier("version", normalizedVersion, artifactVersionPattern); err != nil {
		return false, err
	}
	return s.storage.Exists(ctx, fmt.Sprintf("artifacts/%s/%s/%s/manifest.json", normalizedKind, normalizedName, normalizedVersion))
}

func (s *Service) DeleteArtifact(ctx context.Context, kind string, name string) (DeleteResult, error) {
	normalizedKind := strings.TrimSpace(kind)
	normalizedName := strings.TrimSpace(name)
	if err := validateArtifactKind(normalizedKind); err != nil {
		return DeleteResult{}, err
	}
	if err := validateArtifactIdentifier("name", normalizedName, artifactIdentifierPattern); err != nil {
		return DeleteResult{}, err
	}

	artifactPrefix := path.Join("artifacts", normalizedKind, normalizedName)
	files, err := s.storage.List(ctx, artifactPrefix)
	if err != nil {
		return DeleteResult{}, fmt.Errorf("list artifact files: %w", err)
	}
	if len(files) == 0 {
		return DeleteResult{}, newRequestError("artifact not found")
	}

	artifactPayloads, err := s.collectExternalArtifactPayloads(ctx, normalizedKind, normalizedName, collectVersionsFromArtifactFiles(files, normalizedKind, normalizedName))
	if err != nil {
		return DeleteResult{}, err
	}
	deletedFiles, err := s.deleteStorageFiles(ctx, files)
	if err != nil {
		return DeleteResult{}, err
	}
	if _, err := s.deleteStoragePrefix(ctx, path.Join("index", "artifacts", normalizedKind, normalizedName)); err != nil {
		return DeleteResult{}, err
	}
	if err := s.deleteExternalArtifactPayloads(ctx, artifactPayloads); err != nil {
		return DeleteResult{}, err
	}

	return DeleteResult{
		Kind:              normalizedKind,
		Name:              normalizedName,
		DeletedFiles:      deletedFiles,
		DeletedVersions:   collectVersionsFromArtifactFiles(files, normalizedKind, normalizedName),
		RemainingVersions: 0,
		ArtifactDeleted:   true,
	}, nil
}

func (s *Service) DeleteRelease(ctx context.Context, kind string, name string, version string) (DeleteResult, error) {
	normalizedKind := strings.TrimSpace(kind)
	normalizedName := strings.TrimSpace(name)
	normalizedVersion := strings.TrimSpace(version)
	if err := validateArtifactKind(normalizedKind); err != nil {
		return DeleteResult{}, err
	}
	if err := validateArtifactIdentifier("name", normalizedName, artifactIdentifierPattern); err != nil {
		return DeleteResult{}, err
	}
	if err := validateArtifactIdentifier("version", normalizedVersion, artifactVersionPattern); err != nil {
		return DeleteResult{}, err
	}

	releasePrefix := path.Join("artifacts", normalizedKind, normalizedName, normalizedVersion)
	files, err := s.storage.List(ctx, releasePrefix)
	if err != nil {
		return DeleteResult{}, fmt.Errorf("list release files: %w", err)
	}
	if len(files) == 0 {
		return DeleteResult{}, newRequestError("artifact release not found")
	}

	artifactPayload, err := s.collectExternalArtifactPayload(ctx, normalizedKind, normalizedName, normalizedVersion)
	if err != nil {
		return DeleteResult{}, err
	}
	deletedFiles, err := s.deleteStorageFiles(ctx, files)
	if err != nil {
		return DeleteResult{}, err
	}
	if err := s.deleteExternalArtifactPayloads(ctx, artifactPayload); err != nil {
		return DeleteResult{}, err
	}

	rebuildResult, err := s.rebuildArtifactIndexFromStorage(ctx, normalizedKind, normalizedName)
	if err != nil {
		return DeleteResult{}, err
	}

	return DeleteResult{
		Kind:              normalizedKind,
		Name:              normalizedName,
		Version:           normalizedVersion,
		DeletedFiles:      deletedFiles,
		DeletedVersions:   []string{normalizedVersion},
		RemainingVersions: rebuildResult.RemainingVersions,
		LatestVersion:     rebuildResult.LatestVersion,
		ArtifactDeleted:   rebuildResult.ArtifactDeleted,
	}, nil
}

func (s *Service) publishWithOrigin(ctx context.Context, req Request, origin artifactorigin.Document) error {
	manifest, err := s.parseManifest(req.ArtifactZip)
	if err != nil {
		return newRequestError(fmt.Sprintf("parse manifest: %v", err))
	}
	if err := validateManifestIdentity(req, manifest); err != nil {
		return err
	}
	req = normalizeRequest(req, manifest)
	if err := validateRequest(req); err != nil {
		return err
	}

	hash := sha256.Sum256(req.ArtifactZip)
	sha256Hex := hex.EncodeToString(hash[:])

	signature, err := s.signArtifactPayload(req, sha256Hex)
	if err != nil {
		return err
	}

	artifactPath := fmt.Sprintf("artifacts/%s/%s/%s/%s-%s.zip", req.Kind, req.Name, req.Version, req.Name, req.Version)
	artifactStorage, err := s.resolveArtifactStorage(ctx, req.ArtifactStorageProfileID)
	if err != nil {
		return err
	}
	origin = s.normalizePublishOrigin(origin, artifactStorage.Profile, artifactPath, sha256Hex, int64(len(req.ArtifactZip)))
	if err := artifactStorage.Store.Write(ctx, artifactPath, req.ArtifactZip); err != nil {
		return fmt.Errorf("upload artifact: %w", err)
	}

	releaseManifest, err := s.buildReleaseManifest(req, manifest, req.ArtifactZip, sha256Hex, signature, origin)
	if err != nil {
		return err
	}
	manifestData, _ := json.MarshalIndent(releaseManifest, "", "  ")

	manifestPath := fmt.Sprintf("artifacts/%s/%s/%s/manifest.json", req.Kind, req.Name, req.Version)
	if err := s.storage.Write(ctx, manifestPath, manifestData); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	originData, _ := json.MarshalIndent(origin.ToMap(), "", "  ")
	if err := s.storage.Write(ctx, artifactorigin.DocumentPath(req.Kind, req.Name, req.Version), originData); err != nil {
		return fmt.Errorf("write origin: %w", err)
	}

	if err := s.updateArtifactIndex(ctx, req, sha256Hex, len(req.ArtifactZip)); err != nil {
		return fmt.Errorf("update index: %w", err)
	}
	return nil
}

type artifactIndexRebuildResult struct {
	RemainingVersions int
	LatestVersion     string
	ArtifactDeleted   bool
}

type storedReleaseIndexEntry struct {
	Version     string
	Channel     string
	Channels    []string
	PublishedAt string
	SHA256      string
	Size        any
	ContentType string
	Title       string
	Summary     string
	Description string
	Publisher   map[string]any
	Labels      []string
}

func (s *Service) rebuildArtifactIndexFromStorage(ctx context.Context, kind string, name string) (artifactIndexRebuildResult, error) {
	entries, err := s.listStoredReleaseIndexEntries(ctx, kind, name)
	if err != nil {
		return artifactIndexRebuildResult{}, err
	}

	indexPrefix := path.Join("index", "artifacts", kind, name)
	indexPath := path.Join(indexPrefix, "index.json")
	if len(entries) == 0 {
		if _, err := s.deleteStoragePrefix(ctx, indexPrefix); err != nil {
			return artifactIndexRebuildResult{}, err
		}
		return artifactIndexRebuildResult{
			RemainingVersions: 0,
			ArtifactDeleted:   true,
		}, nil
	}

	sort.SliceStable(entries, func(i int, j int) bool {
		leftTime, leftOK := parsePublishedAt(entries[i].PublishedAt)
		rightTime, rightOK := parsePublishedAt(entries[j].PublishedAt)
		switch {
		case leftOK && rightOK && !leftTime.Equal(rightTime):
			return leftTime.After(rightTime)
		case leftOK != rightOK:
			return leftOK
		case entries[i].Version != entries[j].Version:
			return entries[i].Version > entries[j].Version
		default:
			return entries[i].Channel < entries[j].Channel
		}
	})

	versionRows := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		channels := dedupeStringValues(append([]string{entry.Channel}, entry.Channels...)...)
		primaryChannel := firstNonEmpty(entry.Channel, firstNonEmpty(channels...))
		versionRows = append(versionRows, map[string]any{
			"version":      entry.Version,
			"channel":      firstNonEmpty(primaryChannel, "stable"),
			"channels":     channels,
			"published_at": entry.PublishedAt,
			"sha256":       entry.SHA256,
			"size":         entry.Size,
			"content_type": firstNonEmpty(entry.ContentType, "application/zip"),
		})
	}

	index := map[string]any{
		"kind":           kind,
		"name":           name,
		"latest_version": entries[0].Version,
		"versions":       versionRows,
		"releases":       versionRows,
	}
	for _, entry := range entries {
		if title := strings.TrimSpace(entry.Title); title != "" {
			index["title"] = title
			break
		}
	}
	for _, entry := range entries {
		if summary := strings.TrimSpace(entry.Summary); summary != "" {
			index["summary"] = summary
			break
		}
	}
	for _, entry := range entries {
		if description := strings.TrimSpace(entry.Description); description != "" {
			index["description"] = description
			break
		}
	}
	for _, entry := range entries {
		if len(entry.Publisher) > 0 {
			index["publisher"] = cloneMap(entry.Publisher)
			break
		}
	}
	for _, entry := range entries {
		if len(entry.Labels) > 0 {
			index["labels"] = append([]string(nil), entry.Labels...)
			break
		}
	}

	indexData, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return artifactIndexRebuildResult{}, fmt.Errorf("marshal artifact index: %w", err)
	}
	if err := s.storage.Write(ctx, indexPath, indexData); err != nil {
		return artifactIndexRebuildResult{}, fmt.Errorf("write artifact index: %w", err)
	}

	return artifactIndexRebuildResult{
		RemainingVersions: len(entries),
		LatestVersion:     entries[0].Version,
		ArtifactDeleted:   false,
	}, nil
}

func (s *Service) listStoredReleaseIndexEntries(ctx context.Context, kind string, name string) ([]storedReleaseIndexEntry, error) {
	existingChannelsByVersion := map[string][]string{}
	indexPath := path.Join("index", "artifacts", kind, name, "index.json")
	if payload, err := s.storage.Read(ctx, indexPath); err == nil {
		var index map[string]any
		if err := json.Unmarshal(payload, &index); err == nil {
			for _, entry := range parseVersionEntries(index["versions"]) {
				version := stringValue(entry["version"])
				if version == "" {
					continue
				}
				channels := stringSliceValue(entry["channels"])
				if channel := stringValue(entry["channel"]); channel != "" {
					channels = append([]string{channel}, channels...)
				}
				existingChannelsByVersion[strings.ToLower(version)] = dedupeStringValues(channels...)
			}
		}
	}

	files, err := s.storage.List(ctx, path.Join("artifacts", kind, name))
	if err != nil {
		return nil, fmt.Errorf("list artifact releases: %w", err)
	}

	entries := make([]storedReleaseIndexEntry, 0)
	for _, itemPath := range files {
		version, ok := parseStoredReleaseManifestPath(itemPath, kind, name)
		if !ok {
			continue
		}

		payload, err := s.storage.Read(ctx, itemPath)
		if err != nil {
			return nil, fmt.Errorf("read release manifest: %w", err)
		}

		var release map[string]any
		if err := json.Unmarshal(payload, &release); err != nil {
			return nil, fmt.Errorf("decode release manifest: %w", err)
		}
		download := mapValue(release["download"])
		channels := existingChannelsByVersion[strings.ToLower(version)]
		if channel := firstNonEmpty(stringValue(release["channel"]), "stable"); channel != "" {
			channels = dedupeStringValues(append([]string{channel}, channels...)...)
		}
		entries = append(entries, storedReleaseIndexEntry{
			Version:     firstNonEmpty(stringValue(release["version"]), version),
			Channel:     firstNonEmpty(stringValue(release["channel"]), firstNonEmpty(channels...), "stable"),
			Channels:    channels,
			PublishedAt: stringValue(release["published_at"]),
			SHA256:      stringValue(download["sha256"]),
			Size:        download["size"],
			ContentType: firstNonEmpty(stringValue(download["content_type"]), "application/zip"),
			Title:       stringValue(release["title"]),
			Summary:     stringValue(release["summary"]),
			Description: stringValue(release["description"]),
			Publisher:   cloneMap(mapValue(release["publisher"])),
			Labels:      stringSliceValue(release["labels"]),
		})
	}
	return entries, nil
}

func parseStoredReleaseManifestPath(itemPath string, kind string, name string) (string, bool) {
	normalized := path.Clean(strings.TrimSpace(strings.ReplaceAll(itemPath, "\\", "/")))
	parts := strings.Split(normalized, "/")
	if len(parts) != 5 {
		return "", false
	}
	if parts[0] != "artifacts" || parts[1] != kind || parts[2] != name || parts[4] != "manifest.json" {
		return "", false
	}
	version := strings.TrimSpace(parts[3])
	if version == "" {
		return "", false
	}
	return version, true
}

func parsePublishedAt(value string) (time.Time, bool) {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}

func collectVersionsFromArtifactFiles(files []string, kind string, name string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, itemPath := range files {
		version, ok := parseStoredArtifactReleasePath(itemPath, kind, name)
		if !ok {
			continue
		}
		if _, exists := seen[version]; exists {
			continue
		}
		seen[version] = struct{}{}
		out = append(out, version)
	}
	sort.Strings(out)
	return out
}

func parseStoredArtifactReleasePath(itemPath string, kind string, name string) (string, bool) {
	normalized := path.Clean(strings.TrimSpace(strings.ReplaceAll(itemPath, "\\", "/")))
	parts := strings.Split(normalized, "/")
	if len(parts) < 4 {
		return "", false
	}
	if parts[0] != "artifacts" || parts[1] != kind || parts[2] != name {
		return "", false
	}
	version := strings.TrimSpace(parts[3])
	if version == "" {
		return "", false
	}
	return version, true
}

func (s *Service) deleteStoragePrefix(ctx context.Context, prefix string) (int, error) {
	files, err := s.storage.List(ctx, prefix)
	if err != nil {
		return 0, fmt.Errorf("list storage prefix: %w", err)
	}
	return s.deleteStorageFiles(ctx, files)
}

func (s *Service) deleteStorageFiles(ctx context.Context, files []string) (int, error) {
	deleted := 0
	for _, itemPath := range files {
		if strings.TrimSpace(itemPath) == "" {
			continue
		}
		if err := s.storage.Delete(ctx, itemPath); err != nil {
			exists, existsErr := s.storage.Exists(ctx, itemPath)
			if existsErr == nil && !exists {
				continue
			}
			if existsErr != nil {
				return deleted, fmt.Errorf("delete storage item %s: %w", itemPath, existsErr)
			}
			return deleted, fmt.Errorf("delete storage item %s: %w", itemPath, err)
		}
		deleted++
	}
	return deleted, nil
}

type externalArtifactPayload struct {
	store storage.Storage
	path  string
}

func (s *Service) collectExternalArtifactPayloads(
	ctx context.Context,
	kind string,
	name string,
	versions []string,
) ([]externalArtifactPayload, error) {
	if len(versions) == 0 {
		return nil, nil
	}
	payloads := make([]externalArtifactPayload, 0, len(versions))
	for _, version := range versions {
		payload, err := s.collectExternalArtifactPayload(ctx, kind, name, version)
		if err != nil {
			return nil, err
		}
		payloads = append(payloads, payload...)
	}
	return payloads, nil
}

func (s *Service) collectExternalArtifactPayload(
	ctx context.Context,
	kind string,
	name string,
	version string,
) ([]externalArtifactPayload, error) {
	if s.settings == nil {
		return nil, nil
	}

	originPath := artifactorigin.DocumentPath(kind, name, version)
	data, err := s.storage.Read(ctx, originPath)
	if err != nil {
		if !isNotExistStorageError(err) {
			return nil, fmt.Errorf("read origin for %s/%s/%s: %w", kind, name, version, err)
		}
		return nil, nil
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("decode origin for %s/%s/%s: %w", kind, name, version, err)
	}
	origin := artifactorigin.FromMap(payload)
	profileID := strings.TrimSpace(origin.StorageProfileID)
	if profileID == "" || strings.EqualFold(profileID, registrysettings.CanonicalArtifactStorageProfileID) {
		return nil, nil
	}
	if s.settings == nil {
		return nil, fmt.Errorf("artifact storage profile %q is not configured", profileID)
	}
	artifactPath := strings.TrimSpace(origin.MirrorArtifactPath())
	if artifactPath == "" {
		return nil, nil
	}

	resolved, err := s.settings.ResolveArtifactStorage(ctx, profileID)
	if err != nil {
		return nil, fmt.Errorf("resolve artifact storage profile %q for %s/%s/%s: %w", profileID, kind, name, version, err)
	}
	return []externalArtifactPayload{{
		store: resolved.Store,
		path:  artifactPath,
	}}, nil
}

func (s *Service) deleteExternalArtifactPayloads(ctx context.Context, payloads []externalArtifactPayload) error {
	for _, payload := range payloads {
		if payload.store == nil || strings.TrimSpace(payload.path) == "" {
			continue
		}
		if err := payload.store.Delete(ctx, payload.path); err != nil {
			exists, existsErr := payload.store.Exists(ctx, payload.path)
			if existsErr == nil && !exists {
				continue
			}
			if existsErr != nil {
				return fmt.Errorf("delete external artifact payload %s: %w", payload.path, existsErr)
			}
			return fmt.Errorf("delete external artifact payload %s: %w", payload.path, err)
		}
	}
	return nil
}

func (s *Service) resolveArtifactStorage(ctx context.Context, requestedProfileID string) (registrysettings.ResolvedArtifactStorage, error) {
	requestedProfileID = strings.TrimSpace(requestedProfileID)
	if s.settings == nil {
		if requestedProfileID != "" && !strings.EqualFold(requestedProfileID, registrysettings.CanonicalArtifactStorageProfileID) {
			return registrysettings.ResolvedArtifactStorage{}, newRequestError(fmt.Sprintf("artifact storage profile %q is not configured", requestedProfileID))
		}
		return registrysettings.ResolvedArtifactStorage{
			Profile: registrysettings.ArtifactStorageProfile{
				ID:   registrysettings.CanonicalArtifactStorageProfileID,
				Name: "Canonical Storage",
			},
			Store: s.storage,
		}, nil
	}

	resolved, err := s.settings.ResolveArtifactStorage(ctx, requestedProfileID)
	if err != nil {
		return registrysettings.ResolvedArtifactStorage{}, newRequestError(err.Error())
	}
	return resolved, nil
}

func (s *Service) normalizePublishOrigin(
	origin artifactorigin.Document,
	profile registrysettings.ArtifactStorageProfile,
	artifactPath string,
	sha256Hex string,
	size int64,
) artifactorigin.Document {
	if strings.TrimSpace(origin.Provider) == "" {
		return artifactorigin.DefaultMirrorOrigin(
			firstNonEmpty(profile.Type, artifactorigin.ProviderLocal),
			firstNonEmpty(profile.ID, registrysettings.CanonicalArtifactStorageProfileID),
			artifactPath,
			sha256Hex,
			size,
		)
	}

	normalized := artifactorigin.NormalizeDocument(origin)
	if strings.TrimSpace(normalized.StorageProfileID) == "" {
		normalized.StorageProfileID = firstNonEmpty(profile.ID, registrysettings.CanonicalArtifactStorageProfileID)
	}
	if normalized.Mode == "" {
		normalized.Mode = artifactorigin.ModeMirror
	}
	if normalized.Locator == nil {
		normalized.Locator = map[string]any{}
	}
	if normalized.Mode == artifactorigin.ModeMirror && strings.TrimSpace(normalized.Cache.ArtifactPath) == "" {
		normalized.Cache.ArtifactPath = artifactPath
	}
	if normalized.Mode == artifactorigin.ModeMirror && strings.TrimSpace(normalized.Cache.Status) == "" {
		normalized.Cache.Status = artifactorigin.CacheStatusReady
	}
	if strings.TrimSpace(normalized.Integrity.SHA256) == "" {
		normalized.Integrity.SHA256 = strings.ToLower(strings.TrimSpace(sha256Hex))
	}
	if normalized.Integrity.Size <= 0 {
		normalized.Integrity.Size = size
	}
	if strings.TrimSpace(normalized.Sync.LastSyncedAt) == "" && !strings.EqualFold(normalized.Provider, artifactorigin.ProviderLocal) {
		normalized.Sync.LastSyncedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return artifactorigin.NormalizeDocument(normalized)
}

func (s *Service) signArtifactPayload(req Request, sha256Hex string) ([]byte, error) {
	if s.signing == nil {
		return nil, errors.New("publish: signing service is required")
	}
	if strings.TrimSpace(s.keyID) == "" {
		return nil, errors.New("publish: signing key id is required")
	}
	payload := fmt.Sprintf("%s:%s:%s:%s", req.Kind, req.Name, req.Version, sha256Hex)
	signature, err := s.signing.Sign(s.keyID, []byte(payload))
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}
	return signature, nil
}

func (s *Service) parseManifest(zipData []byte) (map[string]any, error) {
	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, err
	}

	for _, file := range reader.File {
		if file.Name != "manifest.json" {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()

		data, err := io.ReadAll(rc)
		if err != nil {
			return nil, err
		}

		var manifest map[string]any
		if err := json.Unmarshal(data, &manifest); err != nil {
			return nil, err
		}
		return manifest, nil
	}
	return nil, fmt.Errorf("manifest.json not found in zip")
}

func (s *Service) buildReleaseManifest(
	req Request,
	manifest map[string]any,
	artifactZip []byte,
	sha256Hex string,
	signature []byte,
	origin artifactorigin.Document,
) (map[string]any, error) {
	description := req.Metadata.Description
	if description == "" {
		description = stringValue(manifest["description"])
	}
	compatibility := deriveCompatibility(req, manifest)
	permissions := derivePermissions(req, manifest)
	labels := deriveLabels(req, manifest)
	governance := deriveGovernance(req.Kind, manifest)
	install := deriveInstall(req.Kind, manifest)
	targets := deriveTargets(req.Kind, req.Name, manifest)
	template, err := deriveTemplate(req.Kind, req.Name, manifest, artifactZip)
	if err != nil {
		return nil, err
	}
	pageRules := derivePageRules(req.Kind, manifest, artifactZip)
	docs := deriveDocs(manifest)
	ui := deriveUI(manifest)

	document := map[string]any{
		"artifact_id":   fmt.Sprintf("%s:%s", req.Kind, req.Name),
		"kind":          req.Kind,
		"name":          req.Name,
		"version":       req.Version,
		"channel":       req.Channel,
		"title":         req.Metadata.Title,
		"summary":       req.Metadata.Summary,
		"description":   description,
		"release_notes": req.Metadata.ReleaseNotes,
		"published_at":  time.Now().UTC().Format(time.RFC3339),
		"publisher":     req.Metadata.Publisher,
		"labels":        labels,
		"download": map[string]any{
			"url":          s.releaseDownloadURL(req.Kind, req.Name, req.Version),
			"filename":     releaseDownloadFilename(req.Name, req.Version, "application/zip"),
			"size":         len(req.ArtifactZip),
			"content_type": "application/zip",
			"sha256":       sha256Hex,
			"transport":    artifactorigin.TransportMap(origin),
		},
		"signature": map[string]any{
			"algorithm": "ed25519",
			"key_id":    s.keyID,
			"sig":       hex.EncodeToString(signature),
		},
		"governance":    governance,
		"compatibility": compatibility,
		"permissions":   permissions,
		"install":       install,
		"targets":       targets,
		"template":      template,
		"docs":          docs,
		"ui":            ui,
	}
	if len(pageRules) > 0 {
		document["page_rules"] = pageRules
	}
	if iconURL := stringValue(ui["icon_url"]); iconURL != "" {
		document["icon_url"] = iconURL
	}
	return document, nil
}

func (s *Service) releaseDownloadURL(kind string, name string, version string) string {
	relative := fmt.Sprintf("/v1/artifacts/%s/%s/releases/%s/download", strings.TrimSpace(kind), strings.TrimSpace(name), strings.TrimSpace(version))
	base := strings.TrimRight(strings.TrimSpace(s.registryBaseURL()), "/")
	if base == "" {
		return relative
	}
	return base + relative
}

func releaseDownloadFilename(name string, version string, contentType string) string {
	filename := strings.TrimSpace(name)
	if filename == "" {
		filename = "artifact"
	}
	if trimmedVersion := strings.TrimSpace(version); trimmedVersion != "" {
		filename += "-" + trimmedVersion
	}
	return filename + releaseDownloadFileExtension(contentType)
}

func releaseDownloadFileExtension(contentType string) string {
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

func (s *Service) updateArtifactIndex(ctx context.Context, req Request, sha256Hex string, artifactSize int) error {
	indexPath := fmt.Sprintf("index/artifacts/%s/%s/index.json", req.Kind, req.Name)

	var index map[string]any
	data, err := s.storage.Read(ctx, indexPath)
	if err == nil {
		if err := json.Unmarshal(data, &index); err != nil {
			return fmt.Errorf("decode existing index: %w", err)
		}
	} else {
		index = map[string]any{
			"kind":        req.Kind,
			"name":        req.Name,
			"title":       req.Metadata.Title,
			"summary":     req.Metadata.Summary,
			"description": req.Metadata.Description,
			"publisher": map[string]any{
				"id":   req.Metadata.Publisher.ID,
				"name": req.Metadata.Publisher.Name,
			},
			"labels":   req.Metadata.Labels,
			"versions": []map[string]any{},
			"releases": []map[string]any{},
		}
	}
	if index == nil {
		index = map[string]any{}
	}

	versions := parseVersionEntries(index["versions"])
	releases := parseVersionEntries(index["releases"])
	newVersion := map[string]any{
		"version":      req.Version,
		"channel":      req.Channel,
		"published_at": time.Now().UTC().Format(time.RFC3339),
		"sha256":       strings.TrimSpace(sha256Hex),
		"size":         artifactSize,
		"content_type": "application/zip",
	}
	versions = prependVersionEntry(versions, newVersion)
	releases = prependVersionEntry(releases, newVersion)
	index["versions"] = versions
	index["releases"] = releases
	index["latest_version"] = req.Version
	if req.Metadata.Title != "" {
		index["title"] = req.Metadata.Title
	}
	if req.Metadata.Summary != "" {
		index["summary"] = req.Metadata.Summary
	}
	if req.Metadata.Description != "" {
		index["description"] = req.Metadata.Description
	}
	if req.Metadata.Publisher.ID != "" || req.Metadata.Publisher.Name != "" {
		index["publisher"] = map[string]any{
			"id":   req.Metadata.Publisher.ID,
			"name": req.Metadata.Publisher.Name,
		}
	}
	if len(req.Metadata.Labels) > 0 {
		index["labels"] = append([]string(nil), req.Metadata.Labels...)
	}

	indexData, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal index: %w", err)
	}
	return s.storage.Write(ctx, indexPath, indexData)
}

func normalizeRequest(req Request, manifest map[string]any) Request {
	req.Kind = strings.ToLower(artifactmanifest.FirstNonEmpty(req.Kind, artifactmanifest.InferKind(manifest)))
	req.Name = artifactmanifest.FirstNonEmpty(req.Name, artifactmanifest.String(manifest, "name"))
	req.Version = artifactmanifest.FirstNonEmpty(req.Version, artifactmanifest.String(manifest, "version"))
	req.Channel = firstNonEmpty(req.Channel, "stable")

	req.Metadata.Title = artifactmanifest.FirstNonEmpty(
		req.Metadata.Title,
		artifactmanifest.Title(manifest),
		req.Name,
	)
	req.Metadata.Summary = artifactmanifest.FirstNonEmpty(req.Metadata.Summary, artifactmanifest.Summary(manifest))
	req.Metadata.Description = artifactmanifest.FirstNonEmpty(
		req.Metadata.Description,
		artifactmanifest.Description(manifest),
		req.Metadata.Summary,
	)
	return req
}

func validateManifestIdentity(req Request, manifest map[string]any) error {
	if err := validateExplicitFieldMatchesManifest("kind", req.Kind, artifactmanifest.InferKind(manifest), true); err != nil {
		return err
	}
	if err := validateExplicitFieldMatchesManifest("name", req.Name, artifactmanifest.String(manifest, "name"), false); err != nil {
		return err
	}
	if err := validateExplicitFieldMatchesManifest("version", req.Version, artifactmanifest.String(manifest, "version"), false); err != nil {
		return err
	}
	return nil
}

func validateExplicitFieldMatchesManifest(field string, requestValue string, manifestValue string, caseInsensitive bool) error {
	requestValue = strings.TrimSpace(requestValue)
	manifestValue = strings.TrimSpace(manifestValue)
	if requestValue == "" || manifestValue == "" {
		return nil
	}
	if caseInsensitive {
		if strings.EqualFold(requestValue, manifestValue) {
			return nil
		}
	} else if requestValue == manifestValue {
		return nil
	}
	return newRequestError(fmt.Sprintf("%s %q does not match manifest %s %q", field, requestValue, field, manifestValue))
}

func validateRequest(req Request) error {
	if strings.TrimSpace(req.Kind) == "" {
		return newRequestError("kind is required")
	}
	if err := validateArtifactKind(req.Kind); err != nil {
		return err
	}
	if err := validateArtifactIdentifier("name", req.Name, artifactIdentifierPattern); err != nil {
		return err
	}
	if err := validateArtifactIdentifier("version", req.Version, artifactVersionPattern); err != nil {
		return err
	}
	if err := validateArtifactIdentifier("channel", req.Channel, artifactIdentifierPattern); err != nil {
		return err
	}
	if len(req.ArtifactZip) == 0 {
		return newRequestError("artifact zip is required")
	}
	return nil
}

func validateArtifactKind(kind string) error {
	trimmed := strings.TrimSpace(kind)
	if trimmed == "" {
		return newRequestError("kind is required")
	}
	if _, ok := supportedArtifactKinds[trimmed]; !ok {
		return newRequestError(fmt.Sprintf("unsupported artifact kind %q", trimmed))
	}
	return nil
}

func validateArtifactIdentifier(field string, value string, pattern *regexp.Regexp) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return newRequestError(fmt.Sprintf("%s is required", field))
	}
	if strings.Contains(trimmed, "/") || strings.Contains(trimmed, "\\") || strings.Contains(trimmed, "..") {
		return newRequestError(fmt.Sprintf("%s contains forbidden path characters", field))
	}
	if !pattern.MatchString(trimmed) {
		return newRequestError(fmt.Sprintf("%s must be a safe identifier", field))
	}
	return nil
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
			cloned := make(map[string]any, len(mapped))
			for key, current := range mapped {
				cloned[key] = current
			}
			out = append(out, cloned)
		}
		return out
	default:
		return []map[string]any{}
	}
}

func prependVersionEntry(entries []map[string]any, next map[string]any) []map[string]any {
	version := stringValue(next["version"])
	filtered := make([]map[string]any, 0, len(entries)+1)
	mergedNext := cloneMap(next)
	mergedChannels := stringSliceValue(mergedNext["channels"])
	if channel := stringValue(mergedNext["channel"]); channel != "" {
		mergedChannels = append([]string{channel}, mergedChannels...)
	}
	mergedChannels = dedupeStringValues(mergedChannels...)
	if len(mergedChannels) > 0 {
		mergedNext["channels"] = mergedChannels
		mergedNext["channel"] = mergedChannels[0]
	}
	filtered = append(filtered, mergedNext)
	for _, entry := range entries {
		if stringValue(entry["version"]) == version {
			existingChannels := stringSliceValue(entry["channels"])
			if channel := stringValue(entry["channel"]); channel != "" {
				existingChannels = append([]string{channel}, existingChannels...)
			}
			existingChannels = dedupeStringValues(append(mergedChannels, existingChannels...)...)
			if len(existingChannels) > 0 {
				filtered[0]["channels"] = existingChannels
				filtered[0]["channel"] = existingChannels[0]
			}
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

func dedupeStringValues(values ...string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}
	return out
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

func newRequestError(message string) error {
	return &RequestError{Message: strings.TrimSpace(message)}
}

func setStringIfMissing(target map[string]any, key string, values ...any) {
	if stringValue(target[key]) != "" {
		return
	}
	for _, value := range values {
		if text := stringValue(value); text != "" {
			target[key] = text
			return
		}
	}
}

func setBoolIfMissing(target map[string]any, key string, fallback bool) {
	if _, exists := target[key]; exists {
		return
	}
	target[key] = fallback
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

func isNotExistStorageError(err error) bool {
	return errors.Is(err, fs.ErrNotExist)
}
