package config

import (
	"strings"
	"testing"
)

func TestValidateRejectsNonLocalJSWorkerSocketPath(t *testing.T) {
	cfg := newValidTestConfig()
	cfg.Plugin.Sandbox.JSWorkerSocketPath = "tcp://0.0.0.0:17345"

	err := cfg.Validate()
	if err == nil {
		t.Fatalf("expected validation error for non-local js worker socket path")
	}
	if !strings.Contains(err.Error(), "plugin.sandbox.js_worker_socket_path") {
		t.Fatalf("expected js worker socket path error, got %v", err)
	}
}

func TestValidateAcceptsLoopbackJSWorkerSocketPath(t *testing.T) {
	cfg := newValidTestConfig()
	cfg.Plugin.Sandbox.JSWorkerSocketPath = "tcp://127.0.0.1:17345"

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected loopback js worker socket path to be valid, got %v", err)
	}
}

func TestValidateDefaultsPluginSlotAnimationsEnabled(t *testing.T) {
	cfg := newValidTestConfig()

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected config to be valid, got %v", err)
	}
	if !cfg.Plugin.Frontend.SlotAnimationsEnabledValue() {
		t.Fatalf("expected plugin slot animations to default to enabled")
	}
}

func TestValidatePreservesExplicitDisabledPluginSlotAnimations(t *testing.T) {
	cfg := newValidTestConfig()
	disabled := false
	cfg.Plugin.Frontend.SlotAnimationsEnabled = &disabled

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected config to be valid, got %v", err)
	}
	if cfg.Plugin.Frontend.SlotAnimationsEnabledValue() {
		t.Fatalf("expected explicit disabled plugin slot animations to be preserved")
	}
}

func TestValidateRejectsWildcardCORSOrigins(t *testing.T) {
	cfg := newValidTestConfig()
	cfg.Security.CORS.AllowedOrigins = []string{"https://*.example.com"}

	err := cfg.Validate()
	if err == nil {
		t.Fatalf("expected validation error for wildcard cors origin")
	}
	if !strings.Contains(err.Error(), "allowed_origins") {
		t.Fatalf("expected cors allowed_origins error, got %v", err)
	}
}

func TestValidateNormalizesCORSOrigins(t *testing.T) {
	cfg := newValidTestConfig()
	cfg.Security.CORS.AllowedOrigins = []string{
		" https://Example.com/ ",
		"https://example.com",
		"http://LOCALHOST:3000/",
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected config to be valid, got %v", err)
	}
	if len(cfg.Security.CORS.AllowedOrigins) != 2 {
		t.Fatalf("expected 2 normalized origins, got %v", cfg.Security.CORS.AllowedOrigins)
	}
	if cfg.Security.CORS.AllowedOrigins[0] != "https://example.com" {
		t.Fatalf("unexpected first normalized origin %q", cfg.Security.CORS.AllowedOrigins[0])
	}
	if cfg.Security.CORS.AllowedOrigins[1] != "http://localhost:3000" {
		t.Fatalf("unexpected second normalized origin %q", cfg.Security.CORS.AllowedOrigins[1])
	}
}

func TestValidateDefaultsVirtualScriptTimeoutMax(t *testing.T) {
	cfg := newValidTestConfig()

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected config to be valid, got %v", err)
	}
	if cfg.Order.VirtualScriptTimeoutMaxMs != 10000 {
		t.Fatalf("expected virtual script timeout max default 10000, got %d", cfg.Order.VirtualScriptTimeoutMaxMs)
	}
}

func TestValidateNormalizesVirtualScriptTimeoutMaxFloor(t *testing.T) {
	cfg := newValidTestConfig()
	cfg.Order.VirtualScriptTimeoutMaxMs = 1

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected config to be valid, got %v", err)
	}
	if cfg.Order.VirtualScriptTimeoutMaxMs != 100 {
		t.Fatalf("expected virtual script timeout max to normalize to 100, got %d", cfg.Order.VirtualScriptTimeoutMaxMs)
	}
}

func newValidTestConfig() Config {
	return Config{
		App: AppConfig{
			Name: "AuraLogic",
			Port: 8080,
		},
		Database: DatabaseConfig{
			Driver: "sqlite",
			Name:   "test.db",
		},
		JWT: JWTConfig{
			Secret: strings.Repeat("x", 32),
		},
	}
}
