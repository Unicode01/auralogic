package storage

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/studio-b12/gowebdav"
)

type WebDAVStorage struct {
	client   *gowebdav.Client
	endpoint string
	baseURL  string
}

func NewWebDAVStorage(cfg Config) (*WebDAVStorage, error) {
	endpoint := strings.TrimRight(strings.TrimSpace(cfg.WebDAVEndpoint), "/")
	if endpoint == "" {
		return nil, fmt.Errorf("webdav endpoint is required")
	}

	client := gowebdav.NewClient(endpoint, strings.TrimSpace(cfg.WebDAVUsername), cfg.WebDAVPassword)
	client.SetTimeout(60 * time.Second)
	if cfg.WebDAVSkipVerify {
		client.SetTransport(&http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		})
	}

	return &WebDAVStorage{
		client:   client,
		endpoint: endpoint,
		baseURL:  strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/"),
	}, nil
}

func (s *WebDAVStorage) Read(_ context.Context, itemPath string) ([]byte, error) {
	resolved, err := s.resolvePath(itemPath, false)
	if err != nil {
		return nil, err
	}
	payload, err := s.client.Read(resolved)
	if err != nil {
		return nil, translateWebDAVError(err)
	}
	return payload, nil
}

func (s *WebDAVStorage) Write(_ context.Context, itemPath string, data []byte) error {
	resolved, err := s.resolvePath(itemPath, false)
	if err != nil {
		return err
	}
	parent := path.Dir(resolved)
	if parent != "." && parent != "/" {
		if err := s.client.MkdirAll(parent, 0o755); err != nil {
			return translateWebDAVError(err)
		}
	}
	if err := s.client.Write(resolved, data, 0o644); err != nil {
		return translateWebDAVError(err)
	}
	return nil
}

func (s *WebDAVStorage) Exists(_ context.Context, itemPath string) (bool, error) {
	resolved, err := s.resolvePath(itemPath, false)
	if err != nil {
		return false, err
	}
	_, err = s.client.Stat(resolved)
	if err == nil {
		return true, nil
	}
	if gowebdav.IsErrNotFound(err) {
		return false, nil
	}
	return false, translateWebDAVError(err)
}

func (s *WebDAVStorage) List(ctx context.Context, prefix string) ([]string, error) {
	normalized, err := normalizeStoragePath(prefix, true)
	if err != nil {
		return nil, err
	}
	resolved, err := s.resolvePath(prefix, true)
	if err != nil {
		return nil, err
	}

	if normalized != "" {
		info, statErr := s.client.Stat(resolved)
		if statErr == nil && !info.IsDir() {
			return []string{normalized}, nil
		}
		if statErr != nil && !gowebdav.IsErrNotFound(statErr) {
			return nil, translateWebDAVError(statErr)
		}
		if statErr != nil && gowebdav.IsErrNotFound(statErr) {
			return []string{}, nil
		}
	}

	files := make([]string, 0, 8)
	if err := s.walk(resolved, normalized, &files); err != nil {
		if errorsIsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func (s *WebDAVStorage) Delete(_ context.Context, itemPath string) error {
	resolved, err := s.resolvePath(itemPath, false)
	if err != nil {
		return err
	}
	if err := s.client.Remove(resolved); err != nil {
		return translateWebDAVError(err)
	}
	return nil
}

func (s *WebDAVStorage) PublicURL(itemPath string) string {
	base := s.baseURL
	if base == "" {
		base = s.endpoint
	}
	base = strings.TrimRight(base, "/")
	trimmed := strings.TrimLeft(strings.TrimSpace(itemPath), "/")
	if trimmed == "" {
		return base
	}
	return base + "/" + trimmed
}

func (s *WebDAVStorage) Usage(ctx context.Context) (UsageSummary, error) {
	files, err := s.List(ctx, "")
	if err != nil {
		return UsageSummary{}, err
	}
	summary := UsageSummary{
		Backend:     "webdav",
		DisplayName: "WebDAV Storage",
		Location:    s.endpoint,
		FileCount:   len(files),
	}
	for _, itemPath := range files {
		resolved, resolveErr := s.resolvePath(itemPath, false)
		if resolveErr != nil {
			return UsageSummary{}, resolveErr
		}
		info, statErr := s.client.Stat(resolved)
		if statErr != nil {
			return UsageSummary{}, translateWebDAVError(statErr)
		}
		summary.TotalBytes += info.Size()
	}
	return summary, nil
}

func (s *WebDAVStorage) walk(resolved string, relative string, files *[]string) error {
	entries, err := s.client.ReadDir(resolved)
	if err != nil {
		return translateWebDAVError(err)
	}
	for _, entry := range entries {
		name := strings.TrimSpace(entry.Name())
		if name == "" {
			continue
		}
		nextRelative := name
		if relative != "" {
			nextRelative = path.Join(relative, name)
		}
		nextResolved := path.Join(resolved, name)
		if entry.IsDir() {
			if err := s.walk(nextResolved, nextRelative, files); err != nil {
				return err
			}
			continue
		}
		*files = append(*files, nextRelative)
	}
	return nil
}

func (s *WebDAVStorage) resolvePath(value string, allowEmpty bool) (string, error) {
	normalized, err := normalizeStoragePath(value, allowEmpty)
	if err != nil {
		return "", err
	}
	if normalized == "" {
		return "/", nil
	}
	return "/" + normalized, nil
}

func translateWebDAVError(err error) error {
	if err == nil {
		return nil
	}
	if gowebdav.IsErrNotFound(err) {
		return fmt.Errorf("webdav object not found: %w", fs.ErrNotExist)
	}
	return err
}

func errorsIsNotExist(err error) bool {
	return errors.Is(err, fs.ErrNotExist)
}
