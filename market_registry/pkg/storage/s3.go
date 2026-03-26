package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type S3Storage struct {
	client  *minio.Client
	bucket  string
	prefix  string
	baseURL string
}

func NewS3Storage(cfg Config) (*S3Storage, error) {
	bucket := strings.TrimSpace(cfg.S3Bucket)
	if bucket == "" {
		return nil, fmt.Errorf("s3 bucket is required")
	}
	endpoint, secure, err := parseS3Endpoint(cfg.S3Endpoint)
	if err != nil {
		return nil, err
	}
	creds, err := buildS3Credentials(cfg)
	if err != nil {
		return nil, err
	}

	bucketLookup := minio.BucketLookupAuto
	if cfg.S3UsePathStyle {
		bucketLookup = minio.BucketLookupPath
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:        creds,
		Secure:       secure,
		Region:       strings.TrimSpace(cfg.S3Region),
		BucketLookup: bucketLookup,
	})
	if err != nil {
		return nil, fmt.Errorf("build s3 client: %w", err)
	}

	return &S3Storage{
		client:  client,
		bucket:  bucket,
		prefix:  normalizeS3Prefix(cfg.S3Prefix),
		baseURL: strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/"),
	}, nil
}

func (s *S3Storage) Read(ctx context.Context, itemPath string) ([]byte, error) {
	key, err := s.resolveKey(itemPath, false)
	if err != nil {
		return nil, err
	}
	if _, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{}); err != nil {
		return nil, translateS3Error(err)
	}
	object, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, translateS3Error(err)
	}
	defer object.Close()

	payload, err := io.ReadAll(object)
	if err != nil {
		return nil, translateS3Error(err)
	}
	return payload, nil
}

func (s *S3Storage) Write(ctx context.Context, itemPath string, data []byte) error {
	key, err := s.resolveKey(itemPath, false)
	if err != nil {
		return err
	}
	_, err = s.client.PutObject(ctx, s.bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType: "application/octet-stream",
	})
	if err != nil {
		return translateS3Error(err)
	}
	return nil
}

func (s *S3Storage) Exists(ctx context.Context, itemPath string) (bool, error) {
	key, err := s.resolveKey(itemPath, false)
	if err != nil {
		return false, err
	}
	if _, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{}); err != nil {
		if isS3NotFound(err) {
			return false, nil
		}
		return false, translateS3Error(err)
	}
	return true, nil
}

func (s *S3Storage) List(ctx context.Context, prefix string) ([]string, error) {
	normalized, err := normalizeStoragePath(prefix, true)
	if err != nil {
		return nil, err
	}

	if normalized != "" {
		key, keyErr := s.resolveKey(normalized, false)
		if keyErr != nil {
			return nil, keyErr
		}
		if _, statErr := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{}); statErr == nil {
			return []string{normalized}, nil
		} else if !isS3NotFound(statErr) {
			return nil, translateS3Error(statErr)
		}
	}

	listPrefix := s.listPrefix(normalized)
	out := make([]string, 0, 8)
	for item := range s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{
		Prefix:    listPrefix,
		Recursive: true,
		UseV1:     false,
	}) {
		if item.Err != nil {
			return nil, translateS3Error(item.Err)
		}
		relative, ok := s.relativePath(item.Key)
		if !ok || strings.HasSuffix(relative, "/") {
			continue
		}
		out = append(out, relative)
	}
	sort.Strings(out)
	return out, nil
}

func (s *S3Storage) Delete(ctx context.Context, itemPath string) error {
	key, err := s.resolveKey(itemPath, false)
	if err != nil {
		return err
	}
	err = s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
	if err != nil {
		return translateS3Error(err)
	}
	return nil
}

func (s *S3Storage) PublicURL(itemPath string) string {
	return strings.TrimRight(s.baseURL, "/") + "/" + strings.TrimLeft(strings.TrimSpace(itemPath), "/")
}

func (s *S3Storage) Usage(ctx context.Context) (UsageSummary, error) {
	location := "s3://" + s.bucket
	if s.prefix != "" {
		location += "/" + s.prefix
	}
	summary := UsageSummary{
		Backend:     "s3",
		DisplayName: "S3-Compatible Storage",
		Location:    location,
	}
	for item := range s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{
		Prefix:    s.listPrefix(""),
		Recursive: true,
		UseV1:     false,
	}) {
		if item.Err != nil {
			return UsageSummary{}, translateS3Error(item.Err)
		}
		if strings.HasSuffix(strings.TrimSpace(item.Key), "/") {
			continue
		}
		summary.FileCount++
		summary.TotalBytes += item.Size
	}
	return summary, nil
}

func (s *S3Storage) resolveKey(itemPath string, allowEmpty bool) (string, error) {
	normalized, err := normalizeStoragePath(itemPath, allowEmpty)
	if err != nil {
		return "", err
	}
	switch {
	case normalized == "" && s.prefix == "":
		return "", nil
	case normalized == "":
		return s.prefix, nil
	case s.prefix == "":
		return normalized, nil
	default:
		return s.prefix + "/" + normalized, nil
	}
}

func (s *S3Storage) listPrefix(normalized string) string {
	switch {
	case normalized == "" && s.prefix == "":
		return ""
	case normalized == "":
		return strings.TrimRight(s.prefix, "/") + "/"
	case s.prefix == "":
		return normalized + "/"
	default:
		return s.prefix + "/" + normalized + "/"
	}
}

func (s *S3Storage) relativePath(key string) (string, bool) {
	normalizedKey := strings.Trim(strings.TrimSpace(strings.ReplaceAll(key, "\\", "/")), "/")
	if normalizedKey == "" {
		return "", false
	}
	if s.prefix == "" {
		return normalizedKey, true
	}
	prefix := strings.TrimRight(s.prefix, "/") + "/"
	if !strings.HasPrefix(normalizedKey, prefix) {
		return "", false
	}
	relative := strings.TrimPrefix(normalizedKey, prefix)
	if relative == "" {
		return "", false
	}
	return relative, true
}

func parseS3Endpoint(value string) (string, bool, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", false, fmt.Errorf("s3 endpoint is required")
	}
	if strings.Contains(trimmed, "://") {
		trimmed = strings.TrimSpace(trimmed)
		parts := strings.SplitN(trimmed, "://", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
			return "", false, fmt.Errorf("s3 endpoint is invalid")
		}
		secure := strings.EqualFold(strings.TrimSpace(parts[0]), "https")
		host := strings.TrimRight(strings.TrimSpace(parts[1]), "/")
		if host == "" {
			return "", false, fmt.Errorf("s3 endpoint is invalid")
		}
		return host, secure, nil
	}
	return strings.TrimRight(trimmed, "/"), true, nil
}

func buildS3Credentials(cfg Config) (*credentials.Credentials, error) {
	accessKeyID := strings.TrimSpace(cfg.S3AccessKeyID)
	secretAccessKey := strings.TrimSpace(cfg.S3SecretAccessKey)
	sessionToken := strings.TrimSpace(cfg.S3SessionToken)
	if accessKeyID == "" || secretAccessKey == "" {
		return nil, fmt.Errorf("s3 access key id and secret access key are required")
	}
	return credentials.NewStaticV4(accessKeyID, secretAccessKey, sessionToken), nil
}

func normalizeS3Prefix(value string) string {
	trimmed := strings.Trim(strings.TrimSpace(strings.ReplaceAll(value, "\\", "/")), "/")
	if trimmed == "" || trimmed == "." {
		return ""
	}
	return path.Clean(trimmed)
}

func normalizeStoragePath(value string, allowEmpty bool) (string, error) {
	trimmed := strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if trimmed == "" {
		if allowEmpty {
			return "", nil
		}
		return "", fmt.Errorf("path is required")
	}
	if strings.HasPrefix(trimmed, "/") {
		return "", fmt.Errorf("absolute path is not allowed")
	}
	normalized := path.Clean(trimmed)
	if normalized == "." {
		if allowEmpty {
			return "", nil
		}
		return "", fmt.Errorf("path is required")
	}
	if normalized == ".." || strings.HasPrefix(normalized, "../") {
		return "", fmt.Errorf("path escapes storage root")
	}
	return normalized, nil
}

func isS3NotFound(err error) bool {
	response := minio.ToErrorResponse(err)
	switch strings.ToLower(strings.TrimSpace(response.Code)) {
	case "nosuchkey", "nosuchbucket", "notfound":
		return true
	}
	return false
}

func translateS3Error(err error) error {
	if isS3NotFound(err) {
		return fmt.Errorf("object not found: %w", fs.ErrNotExist)
	}
	return err
}
