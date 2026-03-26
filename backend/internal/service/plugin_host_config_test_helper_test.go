package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"auralogic/internal/config"
)

func preparePluginHostConfigFile(t *testing.T, document map[string]interface{}) (*config.Config, string, func()) {
	t.Helper()

	rootDir := t.TempDir()
	configDir := filepath.Join(rootDir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir failed: %v", err)
	}

	configPath := filepath.Join(configDir, "config.json")
	writePluginHostConfigDocument(t, configPath, document)

	runtimeConfig := &config.Config{}
	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config file failed: %v", err)
	}
	if err := json.Unmarshal(raw, runtimeConfig); err != nil {
		t.Fatalf("unmarshal config file failed: %v", err)
	}

	previousConfigPath, hadConfigPath := os.LookupEnv("CONFIG_PATH")
	if err := os.Setenv("CONFIG_PATH", configPath); err != nil {
		t.Fatalf("set CONFIG_PATH failed: %v", err)
	}

	return runtimeConfig, configPath, func() {
		if hadConfigPath {
			_ = os.Setenv("CONFIG_PATH", previousConfigPath)
			return
		}
		_ = os.Unsetenv("CONFIG_PATH")
	}
}

func writePluginHostConfigDocument(t *testing.T, configPath string, document map[string]interface{}) {
	t.Helper()

	body, err := json.MarshalIndent(document, "", "    ")
	if err != nil {
		t.Fatalf("marshal config document failed: %v", err)
	}
	if err := os.WriteFile(configPath, body, 0o644); err != nil {
		t.Fatalf("write config document failed: %v", err)
	}
}

func readPluginHostConfigDocument(t *testing.T, configPath string) map[string]interface{} {
	t.Helper()

	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config document failed: %v", err)
	}

	var document map[string]interface{}
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatalf("unmarshal config document failed: %v", err)
	}
	return document
}
