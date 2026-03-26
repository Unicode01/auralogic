package service

import (
	"testing"

	"auralogic/internal/models"
)

func TestComputePluginRuntimeSpecHashIgnoresJSONFormatting(t *testing.T) {
	pluginA := &models.Plugin{
		Name:          "spec-demo",
		Type:          "custom",
		Runtime:       PluginRuntimeJSWorker,
		Address:       "index.js",
		Version:       "1.0.0",
		Config:        "{\n  \"enabled\": true,\n  \"count\": 1\n}",
		RuntimeParams: "{ \"mode\": \"fast\" }",
		Capabilities:  "{ \"hooks\": [\"*\"] }",
		Manifest:      "{ \"name\": \"spec-demo\" }",
		PackagePath:   "/tmp/spec-demo",
	}
	pluginB := &models.Plugin{
		Name:          "spec-demo",
		Type:          "custom",
		Runtime:       PluginRuntimeJSWorker,
		Address:       "index.js",
		Version:       "1.0.0",
		Config:        "{\"count\":1,\"enabled\":true}",
		RuntimeParams: "{\"mode\":\"fast\"}",
		Capabilities:  "{\"hooks\":[\"*\"]}",
		Manifest:      "{\"name\":\"spec-demo\"}",
		PackagePath:   "/tmp/spec-demo",
	}

	hashA := ComputePluginRuntimeSpecHash(pluginA)
	hashB := ComputePluginRuntimeSpecHash(pluginB)
	if hashA == "" || hashB == "" {
		t.Fatalf("expected non-empty runtime spec hashes")
	}
	if hashA != hashB {
		t.Fatalf("expected runtime spec hash to ignore JSON formatting differences")
	}
}

func TestResolveNextPluginGenerationUsesLargestKnownGeneration(t *testing.T) {
	plugin := &models.Plugin{
		DesiredGeneration: 4,
		AppliedGeneration: 7,
	}

	next := ResolveNextPluginGeneration(plugin)
	if next != 8 {
		t.Fatalf("expected next generation to be 8, got %d", next)
	}
}
