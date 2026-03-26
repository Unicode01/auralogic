package artifactsync

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
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"auralogic/market_registry/pkg/artifactmanifest"
	"auralogic/market_registry/pkg/artifactorigin"
	"auralogic/market_registry/pkg/publish"
)

const (
	defaultGitHubAPIBaseURL     = "https://api.github.com"
	defaultGitHubSyncUserAgent  = "AuraLogic-Market-Registry/1.0"
	gitHubReleaseSyncMaxZipSize = int64(256 * 1024 * 1024)
)

type GitHubReleaseRequest struct {
	Kind                     string
	Name                     string
	Version                  string
	Channel                  string
	ArtifactStorageProfileID string
	Metadata                 publish.Metadata
	Owner                    string
	Repo                     string
	Tag                      string
	AssetName                string
	APIBaseURL               string
	Token                    string
}

type GitHubReleaseResult struct {
	Kind                     string `json:"kind"`
	Name                     string `json:"name"`
	Version                  string `json:"version"`
	Channel                  string `json:"channel"`
	ArtifactStorageProfileID string `json:"artifact_storage_profile_id,omitempty"`
	Owner                    string `json:"owner"`
	Repo                     string `json:"repo"`
	Tag                      string `json:"tag"`
	AssetName                string `json:"asset_name"`
	ReleaseID                int64  `json:"release_id"`
	AssetID                  int64  `json:"asset_id"`
	AssetSize                int64  `json:"asset_size"`
	SHA256                   string `json:"sha256"`
	APIBaseURL               string `json:"api_base_url"`
	PublishedAt              string `json:"published_at,omitempty"`
	BrowserURL               string `json:"browser_url,omitempty"`
	AssetAPIURL              string `json:"asset_api_url,omitempty"`
	AssetDownloadURL         string `json:"asset_download_url,omitempty"`
}

type GitHubReleaseInspectionRequest struct {
	Kind           string
	Name           string
	Version        string
	Owner          string
	Repo           string
	Tag            string
	AssetName      string
	APIBaseURL     string
	Token          string
	ExpectedSHA256 string
	ExpectedSize   int64
}

type GitHubReleaseInspectionResult struct {
	Kind                   string   `json:"kind"`
	Name                   string   `json:"name"`
	Version                string   `json:"version"`
	Owner                  string   `json:"owner"`
	Repo                   string   `json:"repo"`
	Tag                    string   `json:"tag"`
	AssetName              string   `json:"asset_name"`
	ReleaseID              int64    `json:"release_id"`
	AssetID                int64    `json:"asset_id"`
	AssetSize              int64    `json:"asset_size"`
	SHA256                 string   `json:"sha256"`
	APIBaseURL             string   `json:"api_base_url"`
	PublishedAt            string   `json:"published_at,omitempty"`
	BrowserURL             string   `json:"browser_url,omitempty"`
	AssetAPIURL            string   `json:"asset_api_url,omitempty"`
	AssetDownloadURL       string   `json:"asset_download_url,omitempty"`
	AssetUpdatedAt         string   `json:"asset_updated_at,omitempty"`
	ExpectedKind           string   `json:"expected_kind,omitempty"`
	ExpectedName           string   `json:"expected_name,omitempty"`
	ExpectedVersion        string   `json:"expected_version,omitempty"`
	ExpectedSHA256         string   `json:"expected_sha256,omitempty"`
	ExpectedSize           int64    `json:"expected_size,omitempty"`
	MatchesExpectedSHA256  bool     `json:"matches_expected_sha256"`
	MatchesExpectedSize    bool     `json:"matches_expected_size"`
	MatchesExpectedKind    bool     `json:"matches_expected_kind"`
	MatchesExpectedName    bool     `json:"matches_expected_name"`
	MatchesExpectedVersion bool     `json:"matches_expected_version"`
	Changed                bool     `json:"changed"`
	ChangedFields          []string `json:"changed_fields,omitempty"`
}

type GitHubReleasePreviewRequest struct {
	Owner      string
	Repo       string
	Tag        string
	AssetName  string
	APIBaseURL string
	Token      string
}

type GitHubReleasePreviewAsset struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	ContentType        string `json:"content_type,omitempty"`
	UpdatedAt          string `json:"updated_at,omitempty"`
	AssetAPIURL        string `json:"asset_api_url,omitempty"`
	BrowserDownloadURL string `json:"browser_download_url,omitempty"`
	Selected           bool   `json:"selected"`
}

type GitHubReleasePreviewResult struct {
	Owner         string                      `json:"owner"`
	Repo          string                      `json:"repo"`
	Tag           string                      `json:"tag"`
	ReleaseID     int64                       `json:"release_id"`
	ReleaseName   string                      `json:"release_name,omitempty"`
	ReleaseBody   string                      `json:"release_body,omitempty"`
	BrowserURL    string                      `json:"browser_url,omitempty"`
	PublishedAt   string                      `json:"published_at,omitempty"`
	APIBaseURL    string                      `json:"api_base_url"`
	AssetCount    int                         `json:"asset_count"`
	SelectedAsset string                      `json:"selected_asset,omitempty"`
	Assets        []GitHubReleasePreviewAsset `json:"assets"`
}

type GitHubReleaseSyncer struct {
	publish *publish.Service
	client  *http.Client
	now     func() time.Time
}

type RequestError struct {
	Message string
}

func (e *RequestError) Error() string {
	return strings.TrimSpace(e.Message)
}

func IsRequestError(err error) bool {
	var target *RequestError
	return errors.As(err, &target)
}

type gitHubRelease struct {
	ID          int64         `json:"id"`
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	Body        string        `json:"body"`
	HTMLURL     string        `json:"html_url"`
	PublishedAt string        `json:"published_at"`
	Author      gitHubActor   `json:"author"`
	Assets      []gitHubAsset `json:"assets"`
}

type gitHubActor struct {
	Login string `json:"login"`
}

type gitHubAsset struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	URL                string `json:"url"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
	ContentType        string `json:"content_type"`
	UpdatedAt          string `json:"updated_at"`
}

type gitHubReleaseLookupRequest struct {
	Kind       string
	Name       string
	Version    string
	Owner      string
	Repo       string
	Tag        string
	AssetName  string
	APIBaseURL string
	Token      string
}

type fetchedGitHubReleaseArtifact struct {
	release     gitHubRelease
	asset       gitHubAsset
	apiBaseURL  string
	downloadURL string
	payload     []byte
	sha256      string
	kind        string
	name        string
	version     string
}

func NewGitHubReleaseSyncer(pub *publish.Service) *GitHubReleaseSyncer {
	return &GitHubReleaseSyncer{
		publish: pub,
		client: &http.Client{
			Timeout: 90 * time.Second,
		},
		now: time.Now,
	}
}

func (s *GitHubReleaseSyncer) Sync(ctx context.Context, req GitHubReleaseRequest) (GitHubReleaseResult, error) {
	if s == nil || s.publish == nil {
		return GitHubReleaseResult{}, fmt.Errorf("github release syncer is unavailable")
	}

	fetched, err := s.fetchReleaseArtifact(ctx, gitHubReleaseLookupRequest{
		Kind:       req.Kind,
		Name:       req.Name,
		Version:    req.Version,
		Owner:      req.Owner,
		Repo:       req.Repo,
		Tag:        req.Tag,
		AssetName:  req.AssetName,
		APIBaseURL: req.APIBaseURL,
		Token:      req.Token,
	})
	if err != nil {
		return GitHubReleaseResult{}, err
	}

	metadata := mergeGitHubReleaseMetadata(req.Metadata, fetched.release)
	request := publish.Request{
		Kind:                     fetched.kind,
		Name:                     fetched.name,
		Version:                  fetched.version,
		Channel:                  strings.TrimSpace(req.Channel),
		ArtifactStorageProfileID: strings.TrimSpace(req.ArtifactStorageProfileID),
		ArtifactZip:              fetched.payload,
		Metadata:                 metadata,
	}
	origin := artifactorigin.NormalizeDocument(artifactorigin.Document{
		Version:  1,
		Provider: artifactorigin.ProviderGitHubRelease,
		Mode:     artifactorigin.ModeMirror,
		Locator: map[string]any{
			"owner":                strings.TrimSpace(req.Owner),
			"repo":                 strings.TrimSpace(req.Repo),
			"tag":                  strings.TrimSpace(req.Tag),
			"asset_name":           strings.TrimSpace(fetched.asset.Name),
			"api_base_url":         fetched.apiBaseURL,
			"release_id":           fetched.release.ID,
			"asset_id":             fetched.asset.ID,
			"asset_api_url":        strings.TrimSpace(fetched.asset.URL),
			"browser_download_url": strings.TrimSpace(fetched.asset.BrowserDownloadURL),
			"content_type":         firstNonEmpty(strings.TrimSpace(fetched.asset.ContentType), "application/zip"),
		},
		Integrity: artifactorigin.Integrity{
			SHA256: fetched.sha256,
			Size:   int64(len(fetched.payload)),
		},
		Sync: artifactorigin.SyncState{
			Strategy:     artifactorigin.SyncStrategyManual,
			LastSyncedAt: s.now().UTC().Format(time.RFC3339),
		},
	})
	if err := s.publish.PublishWithOrigin(ctx, request, origin); err != nil {
		return GitHubReleaseResult{}, err
	}

	return GitHubReleaseResult{
		Kind:                     fetched.kind,
		Name:                     fetched.name,
		Version:                  fetched.version,
		Channel:                  strings.TrimSpace(req.Channel),
		ArtifactStorageProfileID: strings.TrimSpace(req.ArtifactStorageProfileID),
		Owner:                    strings.TrimSpace(req.Owner),
		Repo:                     strings.TrimSpace(req.Repo),
		Tag:                      strings.TrimSpace(req.Tag),
		AssetName:                strings.TrimSpace(fetched.asset.Name),
		ReleaseID:                fetched.release.ID,
		AssetID:                  fetched.asset.ID,
		AssetSize:                int64(len(fetched.payload)),
		SHA256:                   fetched.sha256,
		APIBaseURL:               fetched.apiBaseURL,
		PublishedAt:              strings.TrimSpace(fetched.release.PublishedAt),
		BrowserURL:               strings.TrimSpace(fetched.release.HTMLURL),
		AssetAPIURL:              strings.TrimSpace(fetched.asset.URL),
		AssetDownloadURL:         strings.TrimSpace(fetched.downloadURL),
	}, nil
}

func (s *GitHubReleaseSyncer) Inspect(ctx context.Context, req GitHubReleaseInspectionRequest) (GitHubReleaseInspectionResult, error) {
	if s == nil {
		return GitHubReleaseInspectionResult{}, fmt.Errorf("github release syncer is unavailable")
	}

	fetched, err := s.fetchReleaseArtifact(ctx, gitHubReleaseLookupRequest{
		Kind:       req.Kind,
		Name:       req.Name,
		Version:    req.Version,
		Owner:      req.Owner,
		Repo:       req.Repo,
		Tag:        req.Tag,
		AssetName:  req.AssetName,
		APIBaseURL: req.APIBaseURL,
		Token:      req.Token,
	})
	if err != nil {
		return GitHubReleaseInspectionResult{}, err
	}

	result := GitHubReleaseInspectionResult{
		Kind:             fetched.kind,
		Name:             fetched.name,
		Version:          fetched.version,
		Owner:            strings.TrimSpace(req.Owner),
		Repo:             strings.TrimSpace(req.Repo),
		Tag:              strings.TrimSpace(req.Tag),
		AssetName:        strings.TrimSpace(fetched.asset.Name),
		ReleaseID:        fetched.release.ID,
		AssetID:          fetched.asset.ID,
		AssetSize:        int64(len(fetched.payload)),
		SHA256:           fetched.sha256,
		APIBaseURL:       fetched.apiBaseURL,
		PublishedAt:      strings.TrimSpace(fetched.release.PublishedAt),
		BrowserURL:       strings.TrimSpace(fetched.release.HTMLURL),
		AssetAPIURL:      strings.TrimSpace(fetched.asset.URL),
		AssetDownloadURL: strings.TrimSpace(fetched.downloadURL),
		AssetUpdatedAt:   strings.TrimSpace(fetched.asset.UpdatedAt),
		ExpectedKind:     strings.TrimSpace(req.Kind),
		ExpectedName:     strings.TrimSpace(req.Name),
		ExpectedVersion:  strings.TrimSpace(req.Version),
		ExpectedSHA256:   strings.ToLower(strings.TrimSpace(req.ExpectedSHA256)),
		ExpectedSize:     req.ExpectedSize,
		ChangedFields:    []string{},
	}
	result.MatchesExpectedKind = result.ExpectedKind == "" || strings.EqualFold(result.ExpectedKind, result.Kind)
	result.MatchesExpectedName = result.ExpectedName == "" || strings.EqualFold(result.ExpectedName, result.Name)
	result.MatchesExpectedVersion = result.ExpectedVersion == "" || strings.EqualFold(result.ExpectedVersion, result.Version)
	result.MatchesExpectedSHA256 = result.ExpectedSHA256 == "" || strings.EqualFold(result.ExpectedSHA256, result.SHA256)
	result.MatchesExpectedSize = result.ExpectedSize <= 0 || result.ExpectedSize == result.AssetSize
	if !result.MatchesExpectedKind {
		result.ChangedFields = append(result.ChangedFields, "kind")
	}
	if !result.MatchesExpectedName {
		result.ChangedFields = append(result.ChangedFields, "name")
	}
	if !result.MatchesExpectedVersion {
		result.ChangedFields = append(result.ChangedFields, "version")
	}
	if !result.MatchesExpectedSHA256 {
		result.ChangedFields = append(result.ChangedFields, "sha256")
	}
	if !result.MatchesExpectedSize {
		result.ChangedFields = append(result.ChangedFields, "size")
	}
	result.Changed = len(result.ChangedFields) > 0
	return result, nil
}

func (s *GitHubReleaseSyncer) Preview(ctx context.Context, req GitHubReleasePreviewRequest) (GitHubReleasePreviewResult, error) {
	if s == nil {
		return GitHubReleasePreviewResult{}, fmt.Errorf("github release syncer is unavailable")
	}
	if err := validateGitHubReleasePreviewRequest(req); err != nil {
		return GitHubReleasePreviewResult{}, err
	}

	apiBaseURL := normalizeGitHubAPIBaseURL(req.APIBaseURL)
	release, err := s.fetchRelease(ctx, apiBaseURL, GitHubReleaseRequest{
		Owner:      req.Owner,
		Repo:       req.Repo,
		Tag:        req.Tag,
		APIBaseURL: apiBaseURL,
		Token:      req.Token,
	})
	if err != nil {
		return GitHubReleasePreviewResult{}, err
	}

	selectedAsset := strings.TrimSpace(req.AssetName)
	assets := make([]GitHubReleasePreviewAsset, 0, len(release.Assets))
	zipCandidates := make([]int, 0, len(release.Assets))
	for _, asset := range release.Assets {
		isZip := strings.EqualFold(strings.TrimSpace(asset.ContentType), "application/zip") || strings.HasSuffix(strings.ToLower(strings.TrimSpace(asset.Name)), ".zip")
		assets = append(assets, GitHubReleasePreviewAsset{
			ID:                 asset.ID,
			Name:               strings.TrimSpace(asset.Name),
			Size:               asset.Size,
			ContentType:        strings.TrimSpace(asset.ContentType),
			UpdatedAt:          strings.TrimSpace(asset.UpdatedAt),
			AssetAPIURL:        strings.TrimSpace(asset.URL),
			BrowserDownloadURL: strings.TrimSpace(asset.BrowserDownloadURL),
			Selected:           selectedAsset != "" && strings.EqualFold(strings.TrimSpace(asset.Name), selectedAsset),
		})
		if isZip {
			zipCandidates = append(zipCandidates, len(assets)-1)
		}
	}
	sort.SliceStable(assets, func(i, j int) bool {
		iIsZip := strings.EqualFold(strings.TrimSpace(assets[i].ContentType), "application/zip") || strings.HasSuffix(strings.ToLower(strings.TrimSpace(assets[i].Name)), ".zip")
		jIsZip := strings.EqualFold(strings.TrimSpace(assets[j].ContentType), "application/zip") || strings.HasSuffix(strings.ToLower(strings.TrimSpace(assets[j].Name)), ".zip")
		if iIsZip != jIsZip {
			return iIsZip
		}
		return assets[i].Name < assets[j].Name
	})

	if selectedAsset == "" && len(assets) == 1 {
		selectedAsset = assets[0].Name
		assets[0].Selected = true
	}
	if selectedAsset == "" && len(zipCandidates) == 1 {
		for index := range assets {
			if strings.EqualFold(strings.TrimSpace(assets[index].ContentType), "application/zip") || strings.HasSuffix(strings.ToLower(strings.TrimSpace(assets[index].Name)), ".zip") {
				selectedAsset = assets[index].Name
				assets[index].Selected = true
				break
			}
		}
	}

	return GitHubReleasePreviewResult{
		Owner:         strings.TrimSpace(req.Owner),
		Repo:          strings.TrimSpace(req.Repo),
		Tag:           strings.TrimSpace(req.Tag),
		ReleaseID:     release.ID,
		ReleaseName:   strings.TrimSpace(release.Name),
		ReleaseBody:   strings.TrimSpace(release.Body),
		BrowserURL:    strings.TrimSpace(release.HTMLURL),
		PublishedAt:   strings.TrimSpace(release.PublishedAt),
		APIBaseURL:    apiBaseURL,
		AssetCount:    len(assets),
		SelectedAsset: selectedAsset,
		Assets:        assets,
	}, nil
}

func (s *GitHubReleaseSyncer) fetchReleaseArtifact(ctx context.Context, req gitHubReleaseLookupRequest) (fetchedGitHubReleaseArtifact, error) {
	if err := validateGitHubReleaseRequest(GitHubReleaseRequest{
		Owner:      req.Owner,
		Repo:       req.Repo,
		Tag:        req.Tag,
		AssetName:  req.AssetName,
		APIBaseURL: req.APIBaseURL,
		Token:      req.Token,
	}); err != nil {
		return fetchedGitHubReleaseArtifact{}, err
	}

	apiBaseURL := normalizeGitHubAPIBaseURL(req.APIBaseURL)
	release, asset, err := s.fetchReleaseAndAsset(ctx, apiBaseURL, GitHubReleaseRequest{
		Owner:      req.Owner,
		Repo:       req.Repo,
		Tag:        req.Tag,
		AssetName:  req.AssetName,
		APIBaseURL: apiBaseURL,
		Token:      req.Token,
	})
	if err != nil {
		return fetchedGitHubReleaseArtifact{}, err
	}

	payload, downloadURL, err := s.downloadAssetPayload(ctx, req.Token, asset)
	if err != nil {
		return fetchedGitHubReleaseArtifact{}, err
	}
	if len(payload) == 0 {
		return fetchedGitHubReleaseArtifact{}, newRequestError("github release asset is empty")
	}

	sum := sha256.Sum256(payload)
	sha256Hex := hex.EncodeToString(sum[:])
	kind, name, version, err := resolveArtifactCoordinates(GitHubReleaseRequest{
		Kind:    req.Kind,
		Name:    req.Name,
		Version: req.Version,
	}, payload)
	if err != nil {
		return fetchedGitHubReleaseArtifact{}, err
	}

	return fetchedGitHubReleaseArtifact{
		release:     release,
		asset:       asset,
		apiBaseURL:  apiBaseURL,
		downloadURL: downloadURL,
		payload:     payload,
		sha256:      sha256Hex,
		kind:        kind,
		name:        name,
		version:     version,
	}, nil
}

func (s *GitHubReleaseSyncer) fetchRelease(
	ctx context.Context,
	apiBaseURL string,
	req GitHubReleaseRequest,
) (gitHubRelease, error) {
	releaseURL := fmt.Sprintf(
		"%s/repos/%s/%s/releases/tags/%s",
		strings.TrimRight(apiBaseURL, "/"),
		url.PathEscape(strings.TrimSpace(req.Owner)),
		url.PathEscape(strings.TrimSpace(req.Repo)),
		url.PathEscape(strings.TrimSpace(req.Tag)),
	)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseURL, nil)
	if err != nil {
		return gitHubRelease{}, fmt.Errorf("build github release request failed: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", defaultGitHubSyncUserAgent)
	if token := strings.TrimSpace(req.Token); token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}

	response, err := s.client.Do(request)
	if err != nil {
		return gitHubRelease{}, fmt.Errorf("fetch github release failed: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		payload, _ := io.ReadAll(io.LimitReader(response.Body, 32*1024))
		return gitHubRelease{}, newRequestError(fmt.Sprintf(
			"fetch github release failed with status %d: %s",
			response.StatusCode,
			strings.TrimSpace(string(payload)),
		))
	}

	var release gitHubRelease
	if err := json.NewDecoder(io.LimitReader(response.Body, 2*1024*1024)).Decode(&release); err != nil {
		return gitHubRelease{}, fmt.Errorf("decode github release failed: %w", err)
	}
	return release, nil
}

func (s *GitHubReleaseSyncer) fetchReleaseAndAsset(
	ctx context.Context,
	apiBaseURL string,
	req GitHubReleaseRequest,
) (gitHubRelease, gitHubAsset, error) {
	release, err := s.fetchRelease(ctx, apiBaseURL, req)
	if err != nil {
		return gitHubRelease{}, gitHubAsset{}, err
	}
	for _, asset := range release.Assets {
		if strings.TrimSpace(asset.Name) == strings.TrimSpace(req.AssetName) {
			return release, asset, nil
		}
	}
	return gitHubRelease{}, gitHubAsset{}, newRequestError(fmt.Sprintf("github release asset %q was not found", strings.TrimSpace(req.AssetName)))
}

func (s *GitHubReleaseSyncer) downloadAssetPayload(
	ctx context.Context,
	token string,
	asset gitHubAsset,
) ([]byte, string, error) {
	downloadURL := strings.TrimSpace(asset.BrowserDownloadURL)
	useAssetAPI := strings.TrimSpace(token) != "" || downloadURL == ""
	if useAssetAPI {
		downloadURL = strings.TrimSpace(asset.URL)
	}
	if downloadURL == "" {
		return nil, "", newRequestError("github release asset download url is empty")
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("build github asset request failed: %w", err)
	}
	request.Header.Set("User-Agent", defaultGitHubSyncUserAgent)
	if useAssetAPI {
		request.Header.Set("Accept", "application/octet-stream")
	} else {
		request.Header.Set("Accept", "application/zip, application/octet-stream")
	}
	if trimmedToken := strings.TrimSpace(token); trimmedToken != "" {
		request.Header.Set("Authorization", "Bearer "+trimmedToken)
	}

	response, err := s.client.Do(request)
	if err != nil {
		return nil, "", fmt.Errorf("download github release asset failed: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		payload, _ := io.ReadAll(io.LimitReader(response.Body, 32*1024))
		return nil, "", newRequestError(fmt.Sprintf(
			"download github release asset failed with status %d: %s",
			response.StatusCode,
			strings.TrimSpace(string(payload)),
		))
	}

	payload, err := io.ReadAll(io.LimitReader(response.Body, gitHubReleaseSyncMaxZipSize+1))
	if err != nil {
		return nil, "", fmt.Errorf("read github release asset failed: %w", err)
	}
	if int64(len(payload)) > gitHubReleaseSyncMaxZipSize {
		return nil, "", newRequestError("github release asset exceeds size limit")
	}
	if asset.Size > 0 && int64(len(payload)) != asset.Size {
		return nil, "", newRequestError("github release asset size mismatch")
	}
	return payload, downloadURL, nil
}

func validateGitHubReleaseRequest(req GitHubReleaseRequest) error {
	if strings.TrimSpace(req.Owner) == "" {
		return newRequestError("github owner is required")
	}
	if strings.TrimSpace(req.Repo) == "" {
		return newRequestError("github repo is required")
	}
	if strings.TrimSpace(req.Tag) == "" {
		return newRequestError("github release tag is required")
	}
	if strings.TrimSpace(req.AssetName) == "" {
		return newRequestError("github release asset name is required")
	}
	if err := validateGitHubAPIBaseURL(req.APIBaseURL); err != nil {
		return err
	}
	return nil
}

func validateGitHubReleasePreviewRequest(req GitHubReleasePreviewRequest) error {
	if strings.TrimSpace(req.Owner) == "" {
		return newRequestError("github owner is required")
	}
	if strings.TrimSpace(req.Repo) == "" {
		return newRequestError("github repo is required")
	}
	if strings.TrimSpace(req.Tag) == "" {
		return newRequestError("github release tag is required")
	}
	if err := validateGitHubAPIBaseURL(req.APIBaseURL); err != nil {
		return err
	}
	return nil
}

func validateGitHubAPIBaseURL(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return newRequestError("github api base url is invalid")
	}
	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	if scheme != "http" && scheme != "https" {
		return newRequestError("github api base url must use http or https")
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return newRequestError("github api base url host is required")
	}
	return nil
}

func resolveArtifactCoordinates(req GitHubReleaseRequest, payload []byte) (string, string, string, error) {
	manifest, err := readManifest(payload)
	if err != nil {
		return "", "", "", newRequestError(fmt.Sprintf("parse synced artifact manifest failed: %v", err))
	}
	resolvedKind := artifactmanifest.FirstNonEmpty(strings.TrimSpace(req.Kind), artifactmanifest.InferKind(manifest))
	resolvedName := artifactmanifest.FirstNonEmpty(strings.TrimSpace(req.Name), artifactmanifest.String(manifest, "name"))
	resolvedVersion := artifactmanifest.FirstNonEmpty(strings.TrimSpace(req.Version), artifactmanifest.String(manifest, "version"))
	if resolvedKind == "" {
		return "", "", "", newRequestError("artifact kind is required")
	}
	if resolvedName == "" {
		return "", "", "", newRequestError("artifact name is required")
	}
	if resolvedVersion == "" {
		return "", "", "", newRequestError("artifact version is required")
	}
	return resolvedKind, resolvedName, resolvedVersion, nil
}

func readManifest(payload []byte) (map[string]any, error) {
	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		return nil, err
	}
	for _, file := range reader.File {
		if strings.TrimSpace(file.Name) != "manifest.json" {
			continue
		}
		handle, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer handle.Close()
		raw, err := io.ReadAll(handle)
		if err != nil {
			return nil, err
		}
		var manifest map[string]any
		if err := json.Unmarshal(raw, &manifest); err != nil {
			return nil, err
		}
		return manifest, nil
	}
	return nil, fmt.Errorf("manifest.json not found in synced asset")
}

func normalizeGitHubAPIBaseURL(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		trimmed = defaultGitHubAPIBaseURL
	}
	return strings.TrimRight(trimmed, "/")
}

func mergeGitHubReleaseMetadata(metadata publish.Metadata, release gitHubRelease) publish.Metadata {
	if strings.TrimSpace(metadata.ReleaseNotes) == "" {
		metadata.ReleaseNotes = strings.TrimSpace(release.Body)
	}
	if strings.TrimSpace(metadata.Publisher.ID) == "" {
		metadata.Publisher.ID = strings.TrimSpace(release.Author.Login)
	}
	if strings.TrimSpace(metadata.Publisher.Name) == "" {
		metadata.Publisher.Name = strings.TrimSpace(release.Author.Login)
	}
	if strings.TrimSpace(metadata.Title) == "" {
		metadata.Title = strings.TrimSpace(release.Name)
	}
	return metadata
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func newRequestError(message string) error {
	return &RequestError{Message: strings.TrimSpace(message)}
}
