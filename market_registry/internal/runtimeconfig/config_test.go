package runtimeconfig

import (
	"path/filepath"
	"testing"
	"time"
)

func TestLoadSharedPrefersMarketRegistryAliases(t *testing.T) {
	clearEnvSet(t,
		"MARKET_REGISTRY_DATA_DIR",
		"DATA_DIR",
		"MARKET_REGISTRY_KEY_DIR",
		"KEY_DIR",
		"MARKET_REGISTRY_BASE_URL",
		"SOURCE_API_BASE_URL",
		"BASE_URL",
		"MARKET_REGISTRY_KEY_ID",
		"KEY_ID",
		"MARKET_REGISTRY_ID",
		"SOURCE_ID",
		"MARKET_REGISTRY_NAME",
		"SOURCE_NAME",
		"MARKET_REGISTRY_STORAGE_TYPE",
		"MARKET_REGISTRY_STORAGE_S3_ENDPOINT",
		"MARKET_REGISTRY_STORAGE_S3_REGION",
		"MARKET_REGISTRY_STORAGE_S3_BUCKET",
		"MARKET_REGISTRY_STORAGE_S3_PREFIX",
		"MARKET_REGISTRY_STORAGE_S3_ACCESS_KEY_ID",
		"MARKET_REGISTRY_STORAGE_S3_SECRET_ACCESS_KEY",
		"MARKET_REGISTRY_STORAGE_S3_SESSION_TOKEN",
		"MARKET_REGISTRY_STORAGE_S3_USE_PATH_STYLE",
	)

	t.Setenv("DATA_DIR", "legacy-data")
	t.Setenv("MARKET_REGISTRY_DATA_DIR", "registry-data")
	t.Setenv("SOURCE_API_BASE_URL", "http://legacy.example.com")
	t.Setenv("MARKET_REGISTRY_BASE_URL", "https://registry.example.com")
	t.Setenv("KEY_ID", "legacy-key")
	t.Setenv("MARKET_REGISTRY_KEY_ID", "registry-key")
	t.Setenv("SOURCE_ID", "legacy-source")
	t.Setenv("MARKET_REGISTRY_ID", "registry-source")
	t.Setenv("SOURCE_NAME", "Legacy Source")
	t.Setenv("MARKET_REGISTRY_NAME", "Registry Source")
	t.Setenv("MARKET_REGISTRY_STORAGE_TYPE", "s3")
	t.Setenv("MARKET_REGISTRY_STORAGE_S3_ENDPOINT", "https://r2.example.com")
	t.Setenv("MARKET_REGISTRY_STORAGE_S3_REGION", "auto")
	t.Setenv("MARKET_REGISTRY_STORAGE_S3_BUCKET", "market-artifacts")
	t.Setenv("MARKET_REGISTRY_STORAGE_S3_PREFIX", "registry/prod")
	t.Setenv("MARKET_REGISTRY_STORAGE_S3_ACCESS_KEY_ID", "ak")
	t.Setenv("MARKET_REGISTRY_STORAGE_S3_SECRET_ACCESS_KEY", "sk")
	t.Setenv("MARKET_REGISTRY_STORAGE_S3_SESSION_TOKEN", "token")
	t.Setenv("MARKET_REGISTRY_STORAGE_S3_USE_PATH_STYLE", "true")

	cfg := LoadShared()
	if cfg.DataDir != "registry-data" {
		t.Fatalf("expected DataDir registry-data, got %q", cfg.DataDir)
	}
	if cfg.KeyDir != filepath.Join("registry-data", "keys") {
		t.Fatalf("expected KeyDir under registry-data, got %q", cfg.KeyDir)
	}
	if cfg.BaseURL != "https://registry.example.com" {
		t.Fatalf("expected MARKET_REGISTRY_BASE_URL to win, got %q", cfg.BaseURL)
	}
	if cfg.KeyID != "registry-key" {
		t.Fatalf("expected MARKET_REGISTRY_KEY_ID to win, got %q", cfg.KeyID)
	}
	if cfg.SourceID != "registry-source" {
		t.Fatalf("expected MARKET_REGISTRY_ID to win, got %q", cfg.SourceID)
	}
	if cfg.SourceName != "Registry Source" {
		t.Fatalf("expected MARKET_REGISTRY_NAME to win, got %q", cfg.SourceName)
	}
	if cfg.StorageType != "s3" {
		t.Fatalf("expected storage type s3, got %q", cfg.StorageType)
	}
	if cfg.StorageS3Endpoint != "https://r2.example.com" {
		t.Fatalf("expected s3 endpoint, got %q", cfg.StorageS3Endpoint)
	}
	if cfg.StorageS3Region != "auto" {
		t.Fatalf("expected s3 region auto, got %q", cfg.StorageS3Region)
	}
	if cfg.StorageS3Bucket != "market-artifacts" {
		t.Fatalf("expected s3 bucket market-artifacts, got %q", cfg.StorageS3Bucket)
	}
	if cfg.StorageS3Prefix != "registry/prod" {
		t.Fatalf("expected s3 prefix registry/prod, got %q", cfg.StorageS3Prefix)
	}
	if cfg.StorageS3AccessKeyID != "ak" {
		t.Fatalf("expected s3 access key id ak, got %q", cfg.StorageS3AccessKeyID)
	}
	if cfg.StorageS3SecretAccessKey != "sk" {
		t.Fatalf("expected s3 secret access key sk, got %q", cfg.StorageS3SecretAccessKey)
	}
	if cfg.StorageS3SessionToken != "token" {
		t.Fatalf("expected s3 session token token, got %q", cfg.StorageS3SessionToken)
	}
	if !cfg.StorageS3UsePathStyle {
		t.Fatal("expected s3 path style toggle to be enabled")
	}
}

func TestLoadSharedFallsBackToLegacyAliases(t *testing.T) {
	clearEnvSet(t,
		"MARKET_REGISTRY_DATA_DIR",
		"DATA_DIR",
		"MARKET_REGISTRY_KEY_DIR",
		"KEY_DIR",
		"MARKET_REGISTRY_BASE_URL",
		"SOURCE_API_BASE_URL",
		"BASE_URL",
	)

	t.Setenv("DATA_DIR", "legacy-data")
	t.Setenv("KEY_DIR", "legacy-keys")
	t.Setenv("SOURCE_API_BASE_URL", "http://legacy-source-api.example.com")

	cfg := LoadShared()
	if cfg.DataDir != "legacy-data" {
		t.Fatalf("expected legacy DATA_DIR, got %q", cfg.DataDir)
	}
	if cfg.KeyDir != "legacy-keys" {
		t.Fatalf("expected legacy KEY_DIR, got %q", cfg.KeyDir)
	}
	if cfg.BaseURL != "http://legacy-source-api.example.com" {
		t.Fatalf("expected legacy SOURCE_API_BASE_URL, got %q", cfg.BaseURL)
	}
}

func TestLoadCLIAndAPIConfigsUseAliasedValues(t *testing.T) {
	clearEnvSet(t,
		"MARKET_REGISTRY_CHANNEL",
		"CHANNEL",
		"MARKET_REGISTRY_ADDR",
		"SOURCE_API_ADDR",
		"MARKET_REGISTRY_ADMIN_USERNAME",
		"SOURCE_ADMIN_USERNAME",
		"MARKET_REGISTRY_ADMIN_PASSWORD",
		"SOURCE_ADMIN_PASSWORD",
		"MARKET_REGISTRY_ADMIN_PASSWORD_HASH",
		"SOURCE_ADMIN_PASSWORD_HASH",
		"MARKET_REGISTRY_AUTH_TOKEN_TTL",
		"SOURCE_AUTH_TOKEN_TTL",
	)

	t.Setenv("MARKET_REGISTRY_CHANNEL", "beta")
	t.Setenv("MARKET_REGISTRY_ADDR", ":19090")
	t.Setenv("MARKET_REGISTRY_ADMIN_USERNAME", "registry-admin")
	t.Setenv("MARKET_REGISTRY_ADMIN_PASSWORD", " registry-pass ")
	t.Setenv("MARKET_REGISTRY_ADMIN_PASSWORD_HASH", "registry-hash")
	t.Setenv("MARKET_REGISTRY_AUTH_TOKEN_TTL", "45m")

	cliCfg := LoadCLI()
	if cliCfg.Channel != "beta" {
		t.Fatalf("expected channel beta, got %q", cliCfg.Channel)
	}

	apiCfg := LoadAPI()
	if apiCfg.Addr != ":19090" {
		t.Fatalf("expected addr :19090, got %q", apiCfg.Addr)
	}
	if apiCfg.AdminUsername != "registry-admin" {
		t.Fatalf("expected registry-admin username, got %q", apiCfg.AdminUsername)
	}
	if apiCfg.AdminPassword != " registry-pass " {
		t.Fatalf("expected raw admin password to be preserved, got %q", apiCfg.AdminPassword)
	}
	if apiCfg.AdminPasswordHash != "registry-hash" {
		t.Fatalf("expected registry-hash password hash, got %q", apiCfg.AdminPasswordHash)
	}
	if apiCfg.TokenTTL != 45*time.Minute {
		t.Fatalf("expected token ttl 45m, got %s", apiCfg.TokenTTL)
	}
}

func TestBindingsExposeCanonicalAndLegacyMappings(t *testing.T) {
	cliBindings := CLIBindings()
	if len(cliBindings) == 0 {
		t.Fatal("expected CLI bindings to be populated")
	}
	if cliBindings[0].Canonical != "MARKET_REGISTRY_DATA_DIR" {
		t.Fatalf("unexpected first CLI binding: %#v", cliBindings[0])
	}

	apiBindings := APIBindings()
	found := false
	for _, binding := range apiBindings {
		if binding.Canonical != "MARKET_REGISTRY_AUTH_TOKEN_TTL" {
			continue
		}
		found = true
		if len(binding.Legacy) != 1 || binding.Legacy[0] != "SOURCE_AUTH_TOKEN_TTL" {
			t.Fatalf("unexpected auth ttl legacy mapping: %#v", binding)
		}
	}
	if !found {
		t.Fatal("expected MARKET_REGISTRY_AUTH_TOKEN_TTL binding to exist")
	}

	commandBinding := CLICommandBinding()
	if len(commandBinding.Canonical) != 1 || commandBinding.Canonical[0] != "market-registry-cli" {
		t.Fatalf("unexpected CLI command binding: %#v", commandBinding)
	}
	if len(commandBinding.Legacy) != 0 {
		t.Fatalf("expected no legacy CLI command binding, got %#v", commandBinding)
	}

	filesystemBinding := APIFilesystemBinding()
	if len(filesystemBinding.Canonical) != 1 || filesystemBinding.Canonical[0] != "./cmd/market-registry-api" {
		t.Fatalf("unexpected API filesystem canonical binding: %#v", filesystemBinding)
	}
	if len(filesystemBinding.Legacy) != 0 {
		t.Fatalf("expected no API filesystem legacy binding, got %#v", filesystemBinding)
	}
}

func clearEnvSet(t *testing.T, keys ...string) {
	t.Helper()
	for _, key := range keys {
		t.Setenv(key, "")
	}
}
