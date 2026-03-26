package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"auralogic/internal/config"
	"auralogic/internal/models"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
)

func TestCreatePluginManualJSWorkerNormalizesRelativeEntryPath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := openPluginUploadTestDB(t)
	handler := NewPluginHandler(db, service.NewPluginManagerService(db, &config.Config{
		Plugin: config.PluginPlatformConfig{
			Enabled:         true,
			AllowedRuntimes: []string{service.PluginRuntimeGRPC, service.PluginRuntimeJSWorker},
			DefaultRuntime:  service.PluginRuntimeJSWorker,
		},
	}), filepath.Join(t.TempDir(), "uploads", "plugins"))

	pluginDir := filepath.Join(t.TempDir(), "manual-js-plugin")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("mkdir plugin dir failed: %v", err)
	}
	scriptPath := filepath.Join(pluginDir, "index.js")
	if err := os.WriteFile(scriptPath, []byte("module.exports = {};"), 0644); err != nil {
		t.Fatalf("write script failed: %v", err)
	}

	payload, err := json.Marshal(map[string]interface{}{
		"name":         "manual-js-plugin",
		"display_name": "Manual JS Plugin",
		"type":         "custom",
		"runtime":      service.PluginRuntimeJSWorker,
		"package_path": pluginDir,
		"address":      scriptPath,
		"enabled":      false,
	})
	if err != nil {
		t.Fatalf("marshal payload failed: %v", err)
	}

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/admin/plugins", bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")

	handler.CreatePlugin(ctx)

	if rec.Code != http.StatusCreated && rec.Code != http.StatusBadGateway {
		t.Fatalf("expected status 201 or 502, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var plugin models.Plugin
	if err := db.Where("name = ?", "manual-js-plugin").First(&plugin).Error; err != nil {
		t.Fatalf("query plugin failed: %v", err)
	}
	if plugin.PackagePath != filepath.ToSlash(filepath.Clean(pluginDir)) {
		t.Fatalf("expected package_path %s, got %s", filepath.ToSlash(filepath.Clean(pluginDir)), plugin.PackagePath)
	}
	if plugin.Address != "index.js" {
		t.Fatalf("expected relative entry index.js, got %s", plugin.Address)
	}

	var version models.PluginVersion
	if err := db.Where("plugin_id = ?", plugin.ID).First(&version).Error; err != nil {
		t.Fatalf("query plugin version failed: %v", err)
	}
	if version.Address != "index.js" {
		t.Fatalf("expected version relative entry index.js, got %s", version.Address)
	}
}

func TestCreatePluginManualJSWorkerRejectsEntryOutsidePackageRoot(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := openPluginUploadTestDB(t)
	handler := NewPluginHandler(db, service.NewPluginManagerService(db, &config.Config{
		Plugin: config.PluginPlatformConfig{
			Enabled:         true,
			AllowedRuntimes: []string{service.PluginRuntimeGRPC, service.PluginRuntimeJSWorker},
			DefaultRuntime:  service.PluginRuntimeJSWorker,
		},
	}), filepath.Join(t.TempDir(), "uploads", "plugins"))

	pluginDir := filepath.Join(t.TempDir(), "manual-js-plugin")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("mkdir plugin dir failed: %v", err)
	}
	otherDir := filepath.Join(t.TempDir(), "outside")
	if err := os.MkdirAll(otherDir, 0755); err != nil {
		t.Fatalf("mkdir outside dir failed: %v", err)
	}
	outsideScript := filepath.Join(otherDir, "evil.js")
	if err := os.WriteFile(outsideScript, []byte("module.exports = {};"), 0644); err != nil {
		t.Fatalf("write outside script failed: %v", err)
	}

	payload, err := json.Marshal(map[string]interface{}{
		"name":         "manual-js-plugin-outside",
		"display_name": "Manual JS Plugin Outside",
		"type":         "custom",
		"runtime":      service.PluginRuntimeJSWorker,
		"package_path": pluginDir,
		"address":      outsideScript,
		"enabled":      false,
	})
	if err != nil {
		t.Fatalf("marshal payload failed: %v", err)
	}

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/admin/plugins", bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")

	handler.CreatePlugin(ctx)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var pluginCount int64
	if err := db.Model(&models.Plugin{}).Where("name = ?", "manual-js-plugin-outside").Count(&pluginCount).Error; err != nil {
		t.Fatalf("count plugins failed: %v", err)
	}
	if pluginCount != 0 {
		t.Fatalf("expected rejected plugin not to be created, count=%d", pluginCount)
	}
}
