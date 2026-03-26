package registrycli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"auralogic/market_registry/pkg/catalog"
	"auralogic/market_registry/pkg/runtimeconfig"
)

type pullSourceEnvelope struct {
	Success bool           `json:"success"`
	Data    map[string]any `json:"data"`
	Error   struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Message string `json:"message"`
}

type pullReleaseMetadata struct {
	Version     string
	DownloadURL string
	ContentType string
	SHA256      string
	Size        int64
}

func (app App) handlePull(args []string) int {
	kind := strings.TrimSpace(getFlag(args, "--kind"))
	name := strings.TrimSpace(getFlag(args, "--name"))
	if kind == "" || name == "" {
		fmt.Fprintln(app.Stderr, "Error: --kind and --name required")
		return 1
	}

	cfg := runtimeconfig.LoadCLI()
	sourceURL := strings.TrimSpace(getFlag(args, "--source-url"))
	if sourceURL == "" {
		sourceURL = strings.TrimSpace(cfg.Shared.BaseURL)
	}
	if sourceURL == "" {
		fmt.Fprintln(app.Stderr, "Error: --source-url required")
		return 1
	}

	version := strings.TrimSpace(getFlag(args, "--version"))
	channel := strings.TrimSpace(getFlag(args, "--channel"))
	if channel == "" {
		channel = strings.TrimSpace(cfg.Channel)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	client := &http.Client{Timeout: 90 * time.Second}
	if version == "" {
		resolvedVersion, err := fetchLatestArtifactVersion(ctx, client, sourceURL, kind, name, channel)
		if err != nil {
			fmt.Fprintf(app.Stderr, "Error resolving latest version: %v\n", err)
			return 1
		}
		version = resolvedVersion
		if channel != "" {
			fmt.Fprintf(app.Stdout, "Resolved %s version: %s\n", channel, version)
		} else {
			fmt.Fprintf(app.Stdout, "Resolved latest version: %s\n", version)
		}
	}

	release, err := fetchReleaseMetadata(ctx, client, sourceURL, kind, name, version)
	if err != nil {
		fmt.Fprintf(app.Stderr, "Error loading release metadata: %v\n", err)
		return 1
	}

	outputPath, err := resolvePullOutputPath(getFlag(args, "--output"), name, version, release.ContentType)
	if err != nil {
		fmt.Fprintf(app.Stderr, "Error resolving output path: %v\n", err)
		return 1
	}

	force := hasFlag(args, "--force")
	upToDate, err := ensurePullDestination(outputPath, release, force)
	if err != nil {
		fmt.Fprintf(app.Stderr, "Error preparing output path: %v\n", err)
		return 1
	}
	if upToDate {
		fmt.Fprintf(app.Stdout, "Artifact already up to date: %s\n", outputPath)
		fmt.Fprintf(app.Stdout, "SHA256: %s\n", release.SHA256)
		return 0
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		fmt.Fprintf(app.Stderr, "Error creating output directory: %v\n", err)
		return 1
	}

	size, sha, err := downloadReleaseArtifact(ctx, client, sourceURL, release, outputPath)
	if err != nil {
		fmt.Fprintf(app.Stderr, "Error pulling artifact: %v\n", err)
		return 1
	}

	fmt.Fprintf(app.Stdout, "Pulled %s:%s:%s\n", kind, name, version)
	fmt.Fprintf(app.Stdout, "Saved to: %s\n", outputPath)
	fmt.Fprintf(app.Stdout, "Size: %s\n", formatPullSize(size))
	fmt.Fprintf(app.Stdout, "SHA256: %s\n", sha)
	return 0
}

func fetchLatestArtifactVersion(ctx context.Context, client *http.Client, sourceURL string, kind string, name string, channel string) (string, error) {
	document, err := fetchSourceJSON(ctx, client, resolveSourceURL(sourceURL, fmt.Sprintf("/v1/artifacts/%s/%s", url.PathEscape(kind), url.PathEscape(name))))
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(channel) != "" {
		versions := catalog.NormalizeArtifactVersionEntries(document["versions"])
		foundVersions := len(versions) > 0
		for _, item := range versions {
			entry := item
			entryVersion := strings.TrimSpace(stringValueFromMap(entry, "version"))
			if entryVersion == "" {
				continue
			}
			if catalog.ArtifactVersionMatchesChannel(entry, channel) {
				return entryVersion, nil
			}
		}
		if foundVersions {
			return "", fmt.Errorf("channel %q has no published versions", channel)
		}
	}
	version := strings.TrimSpace(stringValueFromMap(document, "latest_version"))
	if version == "" {
		return "", fmt.Errorf("latest_version is missing")
	}
	return version, nil
}

func fetchReleaseMetadata(ctx context.Context, client *http.Client, sourceURL string, kind string, name string, version string) (pullReleaseMetadata, error) {
	document, err := fetchSourceJSON(ctx, client, resolveSourceURL(sourceURL, fmt.Sprintf("/v1/artifacts/%s/%s/releases/%s", url.PathEscape(kind), url.PathEscape(name), url.PathEscape(version))))
	if err != nil {
		return pullReleaseMetadata{}, err
	}
	download, _ := document["download"].(map[string]any)
	downloadURL := strings.TrimSpace(stringValueFromMap(download, "url"))
	if downloadURL == "" {
		downloadURL = resolveSourceURL(sourceURL, fmt.Sprintf("/v1/artifacts/%s/%s/releases/%s/download", url.PathEscape(kind), url.PathEscape(name), url.PathEscape(version)))
	} else {
		downloadURL = resolveSourceURL(sourceURL, downloadURL)
	}
	return pullReleaseMetadata{
		Version:     firstNonEmpty(stringValueFromMap(document, "version"), version),
		DownloadURL: downloadURL,
		ContentType: firstNonEmpty(stringValueFromMap(download, "content_type"), "application/zip"),
		SHA256:      strings.ToLower(strings.TrimSpace(stringValueFromMap(download, "sha256"))),
		Size:        int64ValueFromAny(download["size"]),
	}, nil
}

func fetchSourceJSON(ctx context.Context, client *http.Client, endpoint string) (map[string]any, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")

	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		payload, _ := io.ReadAll(io.LimitReader(response.Body, 32*1024))
		return nil, fmt.Errorf("request failed with status %d: %s", response.StatusCode, strings.TrimSpace(string(payload)))
	}

	var envelope pullSourceEnvelope
	if err := json.NewDecoder(io.LimitReader(response.Body, 2*1024*1024)).Decode(&envelope); err != nil {
		return nil, err
	}
	if !envelope.Success && strings.TrimSpace(envelope.Error.Message) != "" {
		return nil, errors.New(strings.TrimSpace(envelope.Error.Message))
	}
	if envelope.Data == nil {
		return nil, fmt.Errorf("response did not include data")
	}
	return envelope.Data, nil
}

func resolvePullOutputPath(rawOutput string, name string, version string, contentType string) (string, error) {
	defaultName := buildPulledFilename(name, version, contentType)
	trimmed := strings.TrimSpace(rawOutput)
	if trimmed == "" {
		return filepath.Abs(defaultName)
	}
	if info, err := os.Stat(trimmed); err == nil && info.IsDir() {
		return filepath.Abs(filepath.Join(trimmed, defaultName))
	}
	if strings.HasSuffix(trimmed, "/") || strings.HasSuffix(trimmed, "\\") {
		return filepath.Abs(filepath.Join(trimmed, defaultName))
	}
	return filepath.Abs(trimmed)
}

func ensurePullDestination(outputPath string, release pullReleaseMetadata, force bool) (bool, error) {
	info, err := os.Stat(outputPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if info.IsDir() {
		return false, fmt.Errorf("output path is a directory")
	}
	if force {
		return false, nil
	}
	if release.SHA256 != "" {
		sum, size, err := fileSHA256(outputPath)
		if err == nil && strings.EqualFold(sum, release.SHA256) && (release.Size <= 0 || release.Size == size) {
			return true, nil
		}
	}
	return false, fmt.Errorf("output file already exists; use --force to overwrite")
}

func downloadReleaseArtifact(ctx context.Context, client *http.Client, sourceURL string, release pullReleaseMetadata, outputPath string) (int64, string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, resolveSourceURL(sourceURL, release.DownloadURL), nil)
	if err != nil {
		return 0, "", err
	}
	request.Header.Set("Accept", firstNonEmpty(release.ContentType, "application/octet-stream"))

	response, err := client.Do(request)
	if err != nil {
		return 0, "", err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		payload, _ := io.ReadAll(io.LimitReader(response.Body, 32*1024))
		return 0, "", fmt.Errorf("download failed with status %d: %s", response.StatusCode, strings.TrimSpace(string(payload)))
	}

	tempFile, err := os.CreateTemp(filepath.Dir(outputPath), ".market-registry-pull-*")
	if err != nil {
		return 0, "", err
	}
	tempPath := tempFile.Name()
	defer func() {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
	}()

	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(tempFile, hash), response.Body)
	if err != nil {
		return 0, "", err
	}
	sha := hex.EncodeToString(hash.Sum(nil))
	if release.Size > 0 && written != release.Size {
		return 0, "", fmt.Errorf("downloaded size mismatch: expected %d bytes, got %d", release.Size, written)
	}
	if release.SHA256 != "" && !strings.EqualFold(sha, release.SHA256) {
		return 0, "", fmt.Errorf("downloaded sha256 mismatch: expected %s, got %s", release.SHA256, sha)
	}
	if err := tempFile.Close(); err != nil {
		return 0, "", err
	}
	if err := os.Remove(outputPath); err != nil && !os.IsNotExist(err) {
		return 0, "", err
	}
	if err := os.Rename(tempPath, outputPath); err != nil {
		return 0, "", err
	}
	return written, sha, nil
}

func resolveSourceURL(baseURL string, ref string) string {
	base, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return strings.TrimSpace(ref)
	}
	target, err := url.Parse(strings.TrimSpace(ref))
	if err != nil {
		return strings.TrimSpace(ref)
	}
	return base.ResolveReference(target).String()
}

func buildPulledFilename(name string, version string, contentType string) string {
	filename := strings.TrimSpace(name)
	if filename == "" {
		filename = "artifact"
	}
	if strings.TrimSpace(version) != "" {
		filename += "-" + strings.TrimSpace(version)
	}
	filename += fileExtensionForPullContentType(contentType)
	return filename
}

func fileExtensionForPullContentType(contentType string) string {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
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

func stringValueFromMap(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	text, _ := values[key].(string)
	return strings.TrimSpace(text)
}

func int64ValueFromAny(value any) int64 {
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int32:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	case json.Number:
		out, err := typed.Int64()
		if err == nil {
			return out
		}
	case string:
		out, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err == nil {
			return out
		}
	}
	return 0
}

func fileSHA256(path string) (string, int64, error) {
	handle, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer handle.Close()

	hash := sha256.New()
	written, err := io.Copy(hash, handle)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(hash.Sum(nil)), written, nil
}

func formatPullSize(value int64) string {
	if value < 1024 {
		return fmt.Sprintf("%d B", value)
	}
	if value < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(value)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(value)/(1024*1024))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
