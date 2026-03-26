package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	"auralogic/internal/models"
)

type pluginRuntimeSpec struct {
	Name            string `json:"name"`
	Type            string `json:"type"`
	Runtime         string `json:"runtime"`
	Address         string `json:"address"`
	Version         string `json:"version"`
	Config          string `json:"config"`
	RuntimeParams   string `json:"runtime_params"`
	Capabilities    string `json:"capabilities"`
	Manifest        string `json:"manifest"`
	PackagePath     string `json:"package_path"`
	PackageChecksum string `json:"package_checksum"`
}

func ComputePluginRuntimeSpecHash(plugin *models.Plugin) string {
	if plugin == nil {
		return ""
	}

	payload, err := json.Marshal(pluginRuntimeSpec{
		Name:            strings.TrimSpace(plugin.Name),
		Type:            strings.TrimSpace(plugin.Type),
		Runtime:         strings.TrimSpace(plugin.Runtime),
		Address:         strings.TrimSpace(plugin.Address),
		Version:         strings.TrimSpace(plugin.Version),
		Config:          normalizePluginRuntimeSpecJSON(plugin.Config, "{}"),
		RuntimeParams:   normalizePluginRuntimeSpecJSON(plugin.RuntimeParams, "{}"),
		Capabilities:    normalizePluginRuntimeSpecJSON(plugin.Capabilities, "{}"),
		Manifest:        normalizePluginRuntimeSpecJSON(plugin.Manifest, "{}"),
		PackagePath:     strings.TrimSpace(plugin.PackagePath),
		PackageChecksum: strings.TrimSpace(plugin.PackageChecksum),
	})
	if err != nil {
		return ""
	}

	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func ResolvePluginRuntimeSpecHash(plugin *models.Plugin) string {
	if plugin == nil {
		return ""
	}
	if normalized := strings.TrimSpace(plugin.RuntimeSpecHash); normalized != "" {
		return normalized
	}
	return ComputePluginRuntimeSpecHash(plugin)
}

func ResolveNextPluginGeneration(plugin *models.Plugin) uint {
	if plugin == nil {
		return 1
	}
	next := plugin.DesiredGeneration
	if plugin.AppliedGeneration > next {
		next = plugin.AppliedGeneration
	}
	if next < 1 {
		next = 1
	}
	return next + 1
}

func normalizePluginRuntimeSpecJSON(value string, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}

	var parsed interface{}
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return trimmed
	}
	normalized, err := json.Marshal(parsed)
	if err != nil {
		return trimmed
	}
	return string(normalized)
}
