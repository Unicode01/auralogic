package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeJSWorkerRelativeEntryPathWithDirectoryRoot(t *testing.T) {
	pluginDir := filepath.Join(t.TempDir(), "manual-plugin")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("mkdir plugin dir failed: %v", err)
	}
	scriptPath := filepath.Join(pluginDir, "src", "index.js")
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0755); err != nil {
		t.Fatalf("mkdir script dir failed: %v", err)
	}
	if err := os.WriteFile(scriptPath, []byte("module.exports = {};"), 0644); err != nil {
		t.Fatalf("write script failed: %v", err)
	}

	relPath, err := NormalizeJSWorkerRelativeEntryPath(pluginDir, scriptPath)
	if err != nil {
		t.Fatalf("normalize relative entry failed: %v", err)
	}
	if relPath != "src/index.js" {
		t.Fatalf("expected src/index.js, got %s", relPath)
	}

	resolvedPath, err := ResolveJSWorkerScriptPath(relPath, pluginDir)
	if err != nil {
		t.Fatalf("resolve script path failed: %v", err)
	}
	if filepath.Clean(resolvedPath) != filepath.Clean(scriptPath) {
		t.Fatalf("expected resolved path %s, got %s", scriptPath, resolvedPath)
	}
}

func TestNormalizeJSWorkerRelativeEntryPathRejectsTraversal(t *testing.T) {
	pluginDir := filepath.Join(t.TempDir(), "manual-plugin")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("mkdir plugin dir failed: %v", err)
	}
	outsideScript := filepath.Join(t.TempDir(), "outside.js")
	if err := os.WriteFile(outsideScript, []byte("module.exports = {};"), 0644); err != nil {
		t.Fatalf("write outside script failed: %v", err)
	}

	if _, err := NormalizeJSWorkerRelativeEntryPath(pluginDir, outsideScript); err == nil {
		t.Fatalf("expected traversal path to be rejected")
	}
	if _, err := ResolveJSWorkerScriptPath("../outside.js", pluginDir); err == nil {
		t.Fatalf("expected relative traversal path to be rejected at runtime")
	}
}

func TestResolveJSWorkerScriptPathWithDerivedExtractRoot(t *testing.T) {
	artifactRoot := t.TempDir()
	packagePath := filepath.Join(artifactRoot, "123_plugin.zip")
	if err := os.WriteFile(packagePath, []byte("zip"), 0644); err != nil {
		t.Fatalf("write package file failed: %v", err)
	}

	extractRoot, err := ResolveJSWorkerPackageRoot(packagePath)
	if err != nil {
		t.Fatalf("resolve package root failed: %v", err)
	}
	if err := os.MkdirAll(extractRoot, 0755); err != nil {
		t.Fatalf("mkdir extract root failed: %v", err)
	}
	scriptPath := filepath.Join(extractRoot, "index.js")
	if err := os.WriteFile(scriptPath, []byte("module.exports = {};"), 0644); err != nil {
		t.Fatalf("write extracted script failed: %v", err)
	}

	relPath, err := NormalizeJSWorkerRelativeEntryPath(packagePath, scriptPath)
	if err != nil {
		t.Fatalf("normalize extracted entry failed: %v", err)
	}
	if relPath != "index.js" {
		t.Fatalf("expected index.js, got %s", relPath)
	}

	resolvedPath, err := ResolveJSWorkerScriptPath(relPath, packagePath)
	if err != nil {
		t.Fatalf("resolve extracted script path failed: %v", err)
	}
	if filepath.Clean(resolvedPath) != filepath.Clean(scriptPath) {
		t.Fatalf("expected resolved path %s, got %s", scriptPath, resolvedPath)
	}
}

func TestResolveJSWorkerScriptPathAllowsLegacyAbsoluteArtifactPath(t *testing.T) {
	artifactRoot := t.TempDir()
	packagePath := filepath.Join(artifactRoot, "plugin.zip")
	if err := os.WriteFile(packagePath, []byte("zip"), 0644); err != nil {
		t.Fatalf("write package file failed: %v", err)
	}

	legacyRoot := filepath.Join(artifactRoot, "jsworker", "legacy-plugin", "1.0.0", "test")
	if err := os.MkdirAll(legacyRoot, 0755); err != nil {
		t.Fatalf("mkdir legacy root failed: %v", err)
	}
	scriptPath := filepath.Join(legacyRoot, "index.js")
	if err := os.WriteFile(scriptPath, []byte("module.exports = {};"), 0644); err != nil {
		t.Fatalf("write legacy script failed: %v", err)
	}

	resolvedPath, err := ResolveJSWorkerScriptPath(scriptPath, packagePath)
	if err != nil {
		t.Fatalf("resolve legacy script path failed: %v", err)
	}
	if filepath.Clean(resolvedPath) != filepath.Clean(scriptPath) {
		t.Fatalf("expected resolved path %s, got %s", scriptPath, resolvedPath)
	}
}
