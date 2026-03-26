package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type Storage interface {
	Read(ctx context.Context, path string) ([]byte, error)
	Write(ctx context.Context, path string, data []byte) error
	Exists(ctx context.Context, path string) (bool, error)
	List(ctx context.Context, prefix string) ([]string, error)
	Delete(ctx context.Context, path string) error
	PublicURL(path string) string
}

type UsageSummary struct {
	Backend     string `json:"backend"`
	DisplayName string `json:"displayName"`
	Location    string `json:"location,omitempty"`
	FileCount   int    `json:"fileCount"`
	TotalBytes  int64  `json:"totalBytes"`
}

type UsageReporter interface {
	Usage(ctx context.Context) (UsageSummary, error)
}

type Config struct {
	Type              string
	BaseDir           string
	BaseURL           string
	S3Endpoint        string
	S3Region          string
	S3Bucket          string
	S3Prefix          string
	S3AccessKeyID     string
	S3SecretAccessKey string
	S3SessionToken    string
	S3UsePathStyle    bool
	WebDAVEndpoint    string
	WebDAVUsername    string
	WebDAVPassword    string
	WebDAVSkipVerify  bool
	FTPAddress        string
	FTPUsername       string
	FTPPassword       string
	FTPRootDir        string
	FTPSecurity       string
	FTPSkipVerify     bool
}

type LocalStorage struct {
	baseDir string
	baseURL string
}

func NewLocalStorage(baseDir, baseURL string) (*LocalStorage, error) {
	cleanBaseDir := filepath.Clean(baseDir)
	if err := os.MkdirAll(cleanBaseDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create base dir: %w", err)
	}
	return &LocalStorage{
		baseDir: cleanBaseDir,
		baseURL: strings.TrimRight(baseURL, "/"),
	}, nil
}

func New(cfg Config) (Storage, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Type)) {
	case "", "local":
		return NewLocalStorage(cfg.BaseDir, cfg.BaseURL)
	case "s3":
		return NewS3Storage(cfg)
	case "webdav":
		return NewWebDAVStorage(cfg)
	case "ftp":
		return NewFTPStorage(cfg)
	default:
		return nil, fmt.Errorf("unsupported storage type %q", strings.TrimSpace(cfg.Type))
	}
}

func (s *LocalStorage) Read(ctx context.Context, path string) ([]byte, error) {
	fullPath, err := s.resolvePath(path, false)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(fullPath)
}

func (s *LocalStorage) Write(ctx context.Context, path string, data []byte) error {
	fullPath, err := s.resolvePath(path, false)
	if err != nil {
		return err
	}
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tempFile, err := os.CreateTemp(dir, "."+filepath.Base(fullPath)+".tmp-*")
	if err != nil {
		return err
	}
	tempPath := tempFile.Name()
	success := false
	defer func() {
		if !success {
			_ = os.Remove(tempPath)
		}
	}()

	if _, err := tempFile.Write(data); err != nil {
		_ = tempFile.Close()
		return err
	}
	if err := tempFile.Sync(); err != nil {
		_ = tempFile.Close()
		return err
	}
	if err := tempFile.Close(); err != nil {
		return err
	}
	if err := replaceFile(tempPath, fullPath); err != nil {
		return err
	}
	if err := syncDir(dir); err != nil {
		return err
	}
	success = true
	return nil
}

func (s *LocalStorage) Exists(ctx context.Context, path string) (bool, error) {
	fullPath, err := s.resolvePath(path, false)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(fullPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (s *LocalStorage) List(ctx context.Context, prefix string) ([]string, error) {
	fullPrefix, err := s.resolvePath(prefix, true)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(fullPrefix)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	var files []string
	if !info.IsDir() {
		rel, relErr := filepath.Rel(s.baseDir, fullPrefix)
		if relErr != nil {
			return nil, relErr
		}
		return []string{filepath.ToSlash(rel)}, nil
	}

	err = filepath.Walk(fullPrefix, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			rel, relErr := filepath.Rel(s.baseDir, path)
			if relErr != nil {
				return relErr
			}
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

func (s *LocalStorage) Delete(ctx context.Context, path string) error {
	fullPath, err := s.resolvePath(path, false)
	if err != nil {
		return err
	}
	return os.Remove(fullPath)
}

func (s *LocalStorage) PublicURL(path string) string {
	return s.baseURL + "/" + strings.TrimLeft(path, "/")
}

func (s *LocalStorage) Usage(ctx context.Context) (UsageSummary, error) {
	summary := UsageSummary{
		Backend:     "local",
		DisplayName: "Local Disk",
		Location:    s.baseDir,
	}
	err := filepath.Walk(s.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if info == nil || info.IsDir() {
			return nil
		}
		summary.FileCount++
		summary.TotalBytes += info.Size()
		return nil
	})
	if err != nil {
		return UsageSummary{}, err
	}
	return summary, nil
}

func (s *LocalStorage) resolvePath(path string, allowEmpty bool) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		if allowEmpty {
			return s.baseDir, nil
		}
		return "", fmt.Errorf("path is required")
	}

	normalized := filepath.Clean(filepath.FromSlash(trimmed))
	if filepath.IsAbs(normalized) {
		return "", fmt.Errorf("absolute path is not allowed")
	}
	if normalized == "." {
		if allowEmpty {
			return s.baseDir, nil
		}
		return "", fmt.Errorf("path is required")
	}

	fullPath := filepath.Clean(filepath.Join(s.baseDir, normalized))
	if !isWithinRoot(s.baseDir, fullPath) {
		return "", fmt.Errorf("path escapes storage root")
	}
	return fullPath, nil
}

func isWithinRoot(root string, target string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(target))
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func replaceFile(source string, target string) error {
	if err := os.Rename(source, target); err == nil {
		return nil
	}

	if _, err := os.Stat(target); err != nil {
		return err
	}
	if err := os.Remove(target); err != nil {
		return err
	}
	return os.Rename(source, target)
}

func syncDir(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
