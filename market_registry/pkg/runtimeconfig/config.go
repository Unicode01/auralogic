package runtimeconfig

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

const ModulePath = "auralogic/market_registry"

type Shared struct {
	DataDir                  string
	KeyDir                   string
	BaseURL                  string
	KeyID                    string
	SourceID                 string
	SourceName               string
	StorageType              string
	StorageS3Endpoint        string
	StorageS3Region          string
	StorageS3Bucket          string
	StorageS3Prefix          string
	StorageS3AccessKeyID     string
	StorageS3SecretAccessKey string
	StorageS3SessionToken    string
	StorageS3UsePathStyle    bool
	StorageWebDAVEndpoint    string
	StorageWebDAVUsername    string
	StorageWebDAVPassword    string
	StorageWebDAVSkipVerify  bool
	StorageFTPAddress        string
	StorageFTPUsername       string
	StorageFTPPassword       string
	StorageFTPRootDir        string
	StorageFTPSecurity       string
	StorageFTPSkipVerify     bool
}

type API struct {
	Shared            Shared
	Addr              string
	AdminUsername     string
	AdminPassword     string
	AdminPasswordHash string
	TokenTTL          time.Duration
}

type CLI struct {
	Shared  Shared
	Channel string
}

type EnvBinding struct {
	Canonical   string   `json:"canonical"`
	Legacy      []string `json:"legacy,omitempty"`
	Description string   `json:"description,omitempty"`
}

type CommandBinding struct {
	Canonical []string `json:"canonical"`
	Legacy    []string `json:"legacy,omitempty"`
}

type FilesystemBinding struct {
	Canonical []string `json:"canonical"`
	Legacy    []string `json:"legacy,omitempty"`
}

var sharedBindings = []EnvBinding{
	{Canonical: "MARKET_REGISTRY_DATA_DIR", Legacy: []string{"DATA_DIR"}, Description: "registry data root"},
	{Canonical: "MARKET_REGISTRY_KEY_DIR", Legacy: []string{"KEY_DIR"}, Description: "signing key directory"},
	{Canonical: "MARKET_REGISTRY_BASE_URL", Legacy: []string{"SOURCE_API_BASE_URL", "BASE_URL"}, Description: "public base URL"},
	{Canonical: "MARKET_REGISTRY_KEY_ID", Legacy: []string{"KEY_ID"}, Description: "active signing key id"},
	{Canonical: "MARKET_REGISTRY_ID", Legacy: []string{"SOURCE_ID"}, Description: "registry source id"},
	{Canonical: "MARKET_REGISTRY_NAME", Legacy: []string{"SOURCE_NAME"}, Description: "registry display name"},
	{Canonical: "MARKET_REGISTRY_STORAGE_TYPE", Description: "canonical storage backend type"},
	{Canonical: "MARKET_REGISTRY_STORAGE_S3_ENDPOINT", Description: "s3-compatible endpoint"},
	{Canonical: "MARKET_REGISTRY_STORAGE_S3_REGION", Description: "s3-compatible region"},
	{Canonical: "MARKET_REGISTRY_STORAGE_S3_BUCKET", Description: "s3-compatible bucket"},
	{Canonical: "MARKET_REGISTRY_STORAGE_S3_PREFIX", Description: "s3-compatible object prefix"},
	{Canonical: "MARKET_REGISTRY_STORAGE_S3_ACCESS_KEY_ID", Description: "s3-compatible access key id"},
	{Canonical: "MARKET_REGISTRY_STORAGE_S3_SECRET_ACCESS_KEY", Description: "s3-compatible secret access key"},
	{Canonical: "MARKET_REGISTRY_STORAGE_S3_SESSION_TOKEN", Description: "s3-compatible session token"},
	{Canonical: "MARKET_REGISTRY_STORAGE_S3_USE_PATH_STYLE", Description: "s3-compatible path style toggle"},
	{Canonical: "MARKET_REGISTRY_STORAGE_WEBDAV_ENDPOINT", Description: "webdav endpoint"},
	{Canonical: "MARKET_REGISTRY_STORAGE_WEBDAV_USERNAME", Description: "webdav username"},
	{Canonical: "MARKET_REGISTRY_STORAGE_WEBDAV_PASSWORD", Description: "webdav password"},
	{Canonical: "MARKET_REGISTRY_STORAGE_WEBDAV_SKIP_VERIFY", Description: "webdav tls verification toggle"},
	{Canonical: "MARKET_REGISTRY_STORAGE_FTP_ADDRESS", Description: "ftp address"},
	{Canonical: "MARKET_REGISTRY_STORAGE_FTP_USERNAME", Description: "ftp username"},
	{Canonical: "MARKET_REGISTRY_STORAGE_FTP_PASSWORD", Description: "ftp password"},
	{Canonical: "MARKET_REGISTRY_STORAGE_FTP_ROOT_DIR", Description: "ftp root directory"},
	{Canonical: "MARKET_REGISTRY_STORAGE_FTP_SECURITY", Description: "ftp security mode"},
	{Canonical: "MARKET_REGISTRY_STORAGE_FTP_SKIP_VERIFY", Description: "ftp tls verification toggle"},
}

var cliBindings = []EnvBinding{
	{Canonical: "MARKET_REGISTRY_CHANNEL", Legacy: []string{"CHANNEL"}, Description: "default publish channel"},
}

var apiBindings = []EnvBinding{
	{Canonical: "MARKET_REGISTRY_ADDR", Legacy: []string{"SOURCE_API_ADDR"}, Description: "listen address"},
	{Canonical: "MARKET_REGISTRY_ADMIN_USERNAME", Legacy: []string{"SOURCE_ADMIN_USERNAME"}, Description: "admin username"},
	{Canonical: "MARKET_REGISTRY_ADMIN_PASSWORD", Legacy: []string{"SOURCE_ADMIN_PASSWORD"}, Description: "admin password"},
	{Canonical: "MARKET_REGISTRY_ADMIN_PASSWORD_HASH", Legacy: []string{"SOURCE_ADMIN_PASSWORD_HASH"}, Description: "admin password hash"},
	{Canonical: "MARKET_REGISTRY_AUTH_TOKEN_TTL", Legacy: []string{"SOURCE_AUTH_TOKEN_TTL"}, Description: "admin token ttl"},
}

func LoadShared() Shared {
	dataDir := resolveEnv("data", "MARKET_REGISTRY_DATA_DIR", "DATA_DIR")
	keyDir := resolveEnv("", "MARKET_REGISTRY_KEY_DIR", "KEY_DIR")
	if keyDir == "" {
		keyDir = filepath.Join(dataDir, "keys")
	}
	return Shared{
		DataDir:                  dataDir,
		KeyDir:                   keyDir,
		BaseURL:                  resolveEnv("http://localhost:18080", "MARKET_REGISTRY_BASE_URL", "SOURCE_API_BASE_URL", "BASE_URL"),
		KeyID:                    resolveEnv("official-2026-01", "MARKET_REGISTRY_KEY_ID", "KEY_ID"),
		SourceID:                 resolveEnv("official", "MARKET_REGISTRY_ID", "SOURCE_ID"),
		SourceName:               resolveEnv("AuraLogic Official Source", "MARKET_REGISTRY_NAME", "SOURCE_NAME"),
		StorageType:              resolveEnv("local", "MARKET_REGISTRY_STORAGE_TYPE"),
		StorageS3Endpoint:        resolveEnv("", "MARKET_REGISTRY_STORAGE_S3_ENDPOINT"),
		StorageS3Region:          resolveEnv("", "MARKET_REGISTRY_STORAGE_S3_REGION"),
		StorageS3Bucket:          resolveEnv("", "MARKET_REGISTRY_STORAGE_S3_BUCKET"),
		StorageS3Prefix:          resolveEnv("", "MARKET_REGISTRY_STORAGE_S3_PREFIX"),
		StorageS3AccessKeyID:     resolveEnv("", "MARKET_REGISTRY_STORAGE_S3_ACCESS_KEY_ID"),
		StorageS3SecretAccessKey: resolveEnvRaw("", "MARKET_REGISTRY_STORAGE_S3_SECRET_ACCESS_KEY"),
		StorageS3SessionToken:    resolveEnvRaw("", "MARKET_REGISTRY_STORAGE_S3_SESSION_TOKEN"),
		StorageS3UsePathStyle:    resolveBool(false, "MARKET_REGISTRY_STORAGE_S3_USE_PATH_STYLE"),
		StorageWebDAVEndpoint:    resolveEnv("", "MARKET_REGISTRY_STORAGE_WEBDAV_ENDPOINT"),
		StorageWebDAVUsername:    resolveEnv("", "MARKET_REGISTRY_STORAGE_WEBDAV_USERNAME"),
		StorageWebDAVPassword:    resolveEnvRaw("", "MARKET_REGISTRY_STORAGE_WEBDAV_PASSWORD"),
		StorageWebDAVSkipVerify:  resolveBool(false, "MARKET_REGISTRY_STORAGE_WEBDAV_SKIP_VERIFY"),
		StorageFTPAddress:        resolveEnv("", "MARKET_REGISTRY_STORAGE_FTP_ADDRESS"),
		StorageFTPUsername:       resolveEnv("", "MARKET_REGISTRY_STORAGE_FTP_USERNAME"),
		StorageFTPPassword:       resolveEnvRaw("", "MARKET_REGISTRY_STORAGE_FTP_PASSWORD"),
		StorageFTPRootDir:        resolveEnv("", "MARKET_REGISTRY_STORAGE_FTP_ROOT_DIR"),
		StorageFTPSecurity:       resolveEnv("plain", "MARKET_REGISTRY_STORAGE_FTP_SECURITY"),
		StorageFTPSkipVerify:     resolveBool(false, "MARKET_REGISTRY_STORAGE_FTP_SKIP_VERIFY"),
	}
}

func LoadAPI() API {
	return API{
		Shared:            LoadShared(),
		Addr:              resolveEnv(":18080", "MARKET_REGISTRY_ADDR", "SOURCE_API_ADDR"),
		AdminUsername:     resolveEnv("", "MARKET_REGISTRY_ADMIN_USERNAME", "SOURCE_ADMIN_USERNAME"),
		AdminPassword:     resolveEnvRaw("", "MARKET_REGISTRY_ADMIN_PASSWORD", "SOURCE_ADMIN_PASSWORD"),
		AdminPasswordHash: resolveEnv("", "MARKET_REGISTRY_ADMIN_PASSWORD_HASH", "SOURCE_ADMIN_PASSWORD_HASH"),
		TokenTTL:          resolveDuration(12*time.Hour, "MARKET_REGISTRY_AUTH_TOKEN_TTL", "SOURCE_AUTH_TOKEN_TTL"),
	}
}

func LoadCLI() CLI {
	return CLI{
		Shared:  LoadShared(),
		Channel: resolveEnv("stable", "MARKET_REGISTRY_CHANNEL", "CHANNEL"),
	}
}

func SharedBindings() []EnvBinding {
	return cloneBindings(sharedBindings)
}

func CLIBindings() []EnvBinding {
	return append(cloneBindings(sharedBindings), cloneBindings(cliBindings)...)
}

func APIBindings() []EnvBinding {
	return append(cloneBindings(sharedBindings), cloneBindings(apiBindings)...)
}

func CLICommandBinding() CommandBinding {
	return CommandBinding{
		Canonical: []string{"market-registry-cli"},
	}
}

func APICommandBinding() CommandBinding {
	return CommandBinding{
		Canonical: []string{"market-registry-api"},
	}
}

func CLIFilesystemBinding() FilesystemBinding {
	return FilesystemBinding{
		Canonical: []string{
			"./cmd/market-registry-cli",
		},
	}
}

func APIFilesystemBinding() FilesystemBinding {
	return FilesystemBinding{
		Canonical: []string{
			"./cmd/market-registry-api",
		},
	}
}

func resolveEnv(defaultValue string, keys ...string) string {
	for _, key := range keys {
		if value, ok := lookupEnvTrimmed(key); ok {
			return value
		}
	}
	return defaultValue
}

func resolveEnvRaw(defaultValue string, keys ...string) string {
	for _, key := range keys {
		if value, ok := os.LookupEnv(key); ok {
			return value
		}
	}
	return defaultValue
}

func resolveDuration(fallback time.Duration, keys ...string) time.Duration {
	for _, key := range keys {
		value, ok := lookupEnvTrimmed(key)
		if !ok {
			continue
		}
		parsed, err := time.ParseDuration(value)
		if err != nil || parsed <= 0 {
			continue
		}
		return parsed
	}
	return fallback
}

func resolveBool(fallback bool, keys ...string) bool {
	for _, key := range keys {
		value, ok := lookupEnvTrimmed(key)
		if !ok {
			continue
		}
		switch strings.ToLower(value) {
		case "1", "true", "yes", "y", "on":
			return true
		case "0", "false", "no", "n", "off":
			return false
		}
	}
	return fallback
}

func lookupEnvTrimmed(key string) (string, bool) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return "", false
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	return value, true
}

func cloneBindings(values []EnvBinding) []EnvBinding {
	if len(values) == 0 {
		return []EnvBinding{}
	}
	out := make([]EnvBinding, 0, len(values))
	for _, value := range values {
		item := value
		item.Legacy = append([]string(nil), value.Legacy...)
		out = append(out, item)
	}
	return out
}
