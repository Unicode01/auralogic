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
