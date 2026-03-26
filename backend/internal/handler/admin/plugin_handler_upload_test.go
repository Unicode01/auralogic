package admin

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"auralogic/internal/config"
	"auralogic/internal/models"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestUploadPluginPackageJSWorkerSuccessStagesResolvedEntry(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := openPluginUploadTestDB(t)
	pluginManager := service.NewPluginManagerService(db, &config.Config{
		Plugin: config.PluginPlatformConfig{
			Enabled:         true,
			AllowedRuntimes: []string{service.PluginRuntimeGRPC, service.PluginRuntimeJSWorker},
			DefaultRuntime:  service.PluginRuntimeJSWorker,
		},
	})
	uploadDir := filepath.Join(t.TempDir(), "uploads", "plugins")
	handler := NewPluginHandler(db, pluginManager, uploadDir)

	manifest := map[string]interface{}{
		"name":         "jsworker-upload-success-plugin",
		"display_name": "jsworker-upload-success-plugin",
		"type":         "custom",
		"runtime":      "js_worker",
		"version":      "1.0.0",
		"capabilities": map[string]interface{}{
			"requested_permissions": []string{
				service.PluginPermissionHookExecute,
				service.PluginPermissionFrontendExtension,
			},
			"granted_permissions": []string{
				service.PluginPermissionHookExecute,
				service.PluginPermissionFrontendExtension,
			},
			"execute_action_storage": map[string]interface{}{
				"template.page.get":  "read",
				"template.page.save": "write",
			},
		},
		"config_schema": map[string]interface{}{
			"fields": []map[string]interface{}{
				{
					"key":     "greeting",
					"type":    "string",
					"default": "hello",
				},
			},
		},
		"activate": false,
	}
	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest failed: %v", err)
	}

	zipPath := filepath.Join(t.TempDir(), "plugin-jsworker-success.zip")
	if err := writeZipFile(zipPath, map[string]string{
		"manifest.json": string(manifestRaw),
		"index.js":      "module.exports.health = () => ({ healthy: true, version: 'test-js/1.0.0' }); module.exports.execute = () => ({ success: true, data: { ok: true } });",
	}); err != nil {
		t.Fatalf("write plugin zip failed: %v", err)
	}

	request := newPluginUploadRequest(t, zipPath, map[string]string{
		"activate": "false",
	})
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = request

	handler.UploadPluginPackage(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var plugin models.Plugin
	if err := db.Where("name = ?", "jsworker-upload-success-plugin").First(&plugin).Error; err != nil {
		t.Fatalf("query plugin failed: %v", err)
	}
	if plugin.Runtime != service.PluginRuntimeJSWorker {
		t.Fatalf("expected runtime %s, got %s", service.PluginRuntimeJSWorker, plugin.Runtime)
	}
	if plugin.Address == "" {
		t.Fatalf("expected resolved js entry address")
	}
	if filepath.IsAbs(filepath.Clean(filepath.FromSlash(plugin.Address))) {
		t.Fatalf("expected stored js entry path to be relative, got %s", plugin.Address)
	}
	if plugin.Address != "index.js" {
		t.Fatalf("expected stored entry path index.js, got %s", plugin.Address)
	}
	resolvedAddress, err := service.ResolveJSWorkerScriptPath(plugin.Address, plugin.PackagePath)
	if err != nil {
		t.Fatalf("resolve stored js entry failed: %v", err)
	}
	if !strings.HasSuffix(filepath.ToSlash(resolvedAddress), "/index.js") {
		t.Fatalf("expected resolved entry to end with index.js, got %s", plugin.Address)
	}
	if _, err := os.Stat(resolvedAddress); err != nil {
		t.Fatalf("expected extracted js entry to exist, stat err=%v", err)
	}
	if plugin.Enabled {
		t.Fatalf("expected staged plugin to remain disabled before activation")
	}
	if plugin.LifecycleStatus != models.PluginLifecycleInstalled {
		t.Fatalf("expected lifecycle installed, got %s", plugin.LifecycleStatus)
	}
	if strings.TrimSpace(plugin.Manifest) != string(manifestRaw) {
		t.Fatalf("expected plugin manifest to be persisted")
	}
	var pluginCapabilities map[string]interface{}
	if err := json.Unmarshal([]byte(plugin.Capabilities), &pluginCapabilities); err != nil {
		t.Fatalf("decode plugin capabilities failed: %v", err)
	}
	grantedPermissions, ok := pluginCapabilities["granted_permissions"].([]interface{})
	if !ok {
		t.Fatalf("expected plugin capabilities.granted_permissions array, got %#v", pluginCapabilities["granted_permissions"])
	}
	grantedSet := make(map[string]struct{}, len(grantedPermissions))
	for _, item := range grantedPermissions {
		grantedSet[strings.TrimSpace(fmt.Sprint(item))] = struct{}{}
	}
	if _, ok := grantedSet[service.PluginPermissionHookExecute]; !ok {
		t.Fatalf("expected granted_permissions to include %s, got %#v", service.PluginPermissionHookExecute, grantedPermissions)
	}
	if _, ok := grantedSet[service.PluginPermissionFrontendExtension]; !ok {
		t.Fatalf("expected granted_permissions to include %s, got %#v", service.PluginPermissionFrontendExtension, grantedPermissions)
	}
	actionStorageRaw, ok := pluginCapabilities["execute_action_storage"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected plugin capabilities.execute_action_storage object, got %#v", pluginCapabilities["execute_action_storage"])
	}
	if got := strings.TrimSpace(fmt.Sprint(actionStorageRaw["template.page.get"])); got != "read" {
		t.Fatalf("expected execute_action_storage template.page.get=read, got %#v", actionStorageRaw)
	}
	if got := strings.TrimSpace(fmt.Sprint(actionStorageRaw["template.page.save"])); got != "write" {
		t.Fatalf("expected execute_action_storage template.page.save=write, got %#v", actionStorageRaw)
	}

	var version models.PluginVersion
	if err := db.Where("plugin_id = ?", plugin.ID).First(&version).Error; err != nil {
		t.Fatalf("query plugin version failed: %v", err)
	}
	if version.Runtime != service.PluginRuntimeJSWorker {
		t.Fatalf("expected version runtime %s, got %s", service.PluginRuntimeJSWorker, version.Runtime)
	}
	if version.Address != plugin.Address {
		t.Fatalf("expected version address %s, got %s", plugin.Address, version.Address)
	}
	if version.IsActive {
		t.Fatalf("expected uploaded version to remain inactive")
	}
	if strings.TrimSpace(version.Manifest) != string(manifestRaw) {
		t.Fatalf("expected version manifest to be persisted")
	}
}

func TestUploadPluginPackageJSWorkerAutoDetectsIndexMJS(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := openPluginUploadTestDB(t)
	pluginManager := service.NewPluginManagerService(db, &config.Config{
		Plugin: config.PluginPlatformConfig{
			Enabled:         true,
			AllowedRuntimes: []string{service.PluginRuntimeGRPC, service.PluginRuntimeJSWorker},
			DefaultRuntime:  service.PluginRuntimeJSWorker,
		},
	})
	uploadDir := filepath.Join(t.TempDir(), "uploads", "plugins")
	handler := NewPluginHandler(db, pluginManager, uploadDir)

	manifestRaw := []byte(`{
		"name": "jsworker-upload-index-mjs-plugin",
		"display_name": "jsworker-upload-index-mjs-plugin",
		"type": "custom",
		"runtime": "js_worker",
		"version": "1.0.0",
		"activate": false
	}`)

	zipPath := filepath.Join(t.TempDir(), "plugin-jsworker-index-mjs.zip")
	if err := writeZipFile(zipPath, map[string]string{
		"manifest.json":        string(manifestRaw),
		"helpers/bootstrap.js": "module.exports = {};",
		"index.mjs":            "export function execute() { return { success: true }; }",
	}); err != nil {
		t.Fatalf("write plugin zip failed: %v", err)
	}

	request := newPluginUploadRequest(t, zipPath, map[string]string{
		"activate": "false",
	})
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = request

	handler.UploadPluginPackage(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var plugin models.Plugin
	if err := db.Where("name = ?", "jsworker-upload-index-mjs-plugin").First(&plugin).Error; err != nil {
		t.Fatalf("query plugin failed: %v", err)
	}
	if plugin.Address != "index.mjs" {
		t.Fatalf("expected stored entry path index.mjs, got %s", plugin.Address)
	}
	resolvedAddress, err := service.ResolveJSWorkerScriptPath(plugin.Address, plugin.PackagePath)
	if err != nil {
		t.Fatalf("resolve stored js entry failed: %v", err)
	}
	if !strings.HasSuffix(filepath.ToSlash(resolvedAddress), "/index.mjs") {
		t.Fatalf("expected resolved entry to end with index.mjs, got %s", resolvedAddress)
	}
}

func TestUploadPluginPackageRejectsNameConflictWithoutTargetPlugin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := openPluginUploadTestDB(t)
	pluginManager := service.NewPluginManagerService(db, nil)
	uploadDir := filepath.Join(t.TempDir(), "uploads", "plugins")
	handler := NewPluginHandler(db, pluginManager, uploadDir)

	existing := models.Plugin{
		Name:            "duplicate-upload-plugin",
		DisplayName:     "Duplicate Upload Plugin",
		Description:     "existing",
		Type:            "custom",
		Runtime:         service.PluginRuntimeGRPC,
		Address:         "127.0.0.1:50100",
		Version:         "1.0.0",
		Config:          "{}",
		RuntimeParams:   "{}",
		Capabilities:    "{}",
		Enabled:         false,
		Status:          "unknown",
		LifecycleStatus: models.PluginLifecycleUploaded,
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("create existing plugin failed: %v", err)
	}

	manifest := map[string]interface{}{
		"name":         existing.Name,
		"display_name": "Duplicate Upload Plugin V2",
		"type":         "custom",
		"runtime":      "grpc",
		"address":      "127.0.0.1:50101",
		"version":      "2.0.0",
		"activate":     false,
	}
	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest failed: %v", err)
	}

	zipPath := filepath.Join(t.TempDir(), "plugin-duplicate-name.zip")
	if err := writeZipFile(zipPath, map[string]string{
		"manifest.json": string(manifestRaw),
	}); err != nil {
		t.Fatalf("write plugin zip failed: %v", err)
	}

	request := newPluginUploadRequest(t, zipPath, map[string]string{
		"activate": "false",
	})
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = request

	handler.UploadPluginPackage(ctx)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if got := strings.TrimSpace(fmt.Sprint(resp["error_key"])); got != "plugin.admin.http_409.plugin_name_conflicts_with_existing_plugin" {
		t.Fatalf("expected conflict error key, got %q, body=%s", got, rec.Body.String())
	}

	var versionCount int64
	if err := db.Model(&models.PluginVersion{}).Count(&versionCount).Error; err != nil {
		t.Fatalf("count plugin versions failed: %v", err)
	}
	if versionCount != 0 {
		t.Fatalf("expected no plugin version created, got %d", versionCount)
	}
	assertDirEmptyOrMissing(t, uploadDir)
}

func TestUploadPluginPackageRejectsFrontendPagePathConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := openPluginUploadTestDB(t)
	pluginManager := service.NewPluginManagerService(db, nil)
	uploadDir := filepath.Join(t.TempDir(), "uploads", "plugins")
	handler := NewPluginHandler(db, pluginManager, uploadDir)

	existingManifest := map[string]interface{}{
		"name":    "existing-page-plugin",
		"type":    "custom",
		"runtime": "grpc",
		"address": "127.0.0.1:50200",
		"frontend": map[string]interface{}{
			"admin_page": map[string]interface{}{
				"path":  "/admin/plugin-pages/shared-conflict-page",
				"title": "Shared Page",
			},
		},
	}
	existingManifestRaw, err := json.Marshal(existingManifest)
	if err != nil {
		t.Fatalf("marshal existing manifest failed: %v", err)
	}
	existing := models.Plugin{
		Name:            "existing-page-plugin",
		DisplayName:     "Existing Page Plugin",
		Description:     "existing",
		Type:            "custom",
		Runtime:         service.PluginRuntimeGRPC,
		Address:         "127.0.0.1:50200",
		Version:         "1.0.0",
		Config:          "{}",
		RuntimeParams:   "{}",
		Capabilities:    "{}",
		Manifest:        string(existingManifestRaw),
		Enabled:         false,
		Status:          "unknown",
		LifecycleStatus: models.PluginLifecycleUploaded,
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("create existing plugin failed: %v", err)
	}

	manifest := map[string]interface{}{
		"name":         "conflicting-page-plugin",
		"display_name": "Conflicting Page Plugin",
		"type":         "custom",
		"runtime":      "grpc",
		"address":      "127.0.0.1:50201",
		"version":      "1.0.0",
		"frontend": map[string]interface{}{
			"admin_page": map[string]interface{}{
				"path":  "/admin/plugin-pages/shared-conflict-page",
				"title": "Conflicting Page",
			},
		},
		"activate": false,
	}
	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest failed: %v", err)
	}

	zipPath := filepath.Join(t.TempDir(), "plugin-conflicting-page.zip")
	if err := writeZipFile(zipPath, map[string]string{
		"manifest.json": string(manifestRaw),
	}); err != nil {
		t.Fatalf("write plugin zip failed: %v", err)
	}

	request := newPluginUploadRequest(t, zipPath, map[string]string{
		"activate": "false",
	})
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = request

	handler.UploadPluginPackage(ctx)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if got := strings.TrimSpace(fmt.Sprint(resp["error_key"])); got != "plugin.admin.http_409.plugin_page_path_conflicts_with_existing_plugin" {
		t.Fatalf("expected page conflict error key, got %q, body=%s", got, rec.Body.String())
	}
	params, ok := resp["error_params"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected error_params object, body=%s", rec.Body.String())
	}
	if got := strings.TrimSpace(fmt.Sprint(params["area"])); got != "admin" {
		t.Fatalf("expected error_params.area=admin, got %q", got)
	}
	if got := strings.TrimSpace(fmt.Sprint(params["path"])); got != "/admin/plugin-pages/shared-conflict-page" {
		t.Fatalf("expected error_params.path conflict path, got %q", got)
	}

	var pluginCount int64
	if err := db.Model(&models.Plugin{}).Where("name = ?", "conflicting-page-plugin").Count(&pluginCount).Error; err != nil {
		t.Fatalf("count plugin failed: %v", err)
	}
	if pluginCount != 0 {
		t.Fatalf("expected conflicting plugin not created, got count=%d", pluginCount)
	}
	assertDirEmptyOrMissing(t, uploadDir)
}

func TestUploadPluginPackageExistingPluginWithoutActivateKeepsMainRecord(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := openPluginUploadTestDB(t)
	pluginManager := service.NewPluginManagerService(db, nil)
	uploadDir := filepath.Join(t.TempDir(), "uploads", "plugins")
	handler := NewPluginHandler(db, pluginManager, uploadDir)

	existing := models.Plugin{
		Name:            "upload-existing-plugin",
		DisplayName:     "Existing Plugin",
		Description:     "old-description",
		Type:            "custom",
		Runtime:         service.PluginRuntimeGRPC,
		Address:         "127.0.0.1:50051",
		Version:         "1.0.0",
		Config:          `{"mode":"old"}`,
		RuntimeParams:   `{"env":"prod"}`,
		Capabilities:    `{"allow_execute_api":true}`,
		PackagePath:     "uploads/plugins/old-package.zip",
		PackageChecksum: "old-checksum",
		Enabled:         true,
		Status:          "healthy",
		LifecycleStatus: models.PluginLifecycleRunning,
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("create existing plugin failed: %v", err)
	}

	manifest := map[string]interface{}{
		"name":         existing.Name,
		"display_name": "Existing Plugin V2",
		"description":  "new-description",
		"type":         "custom",
		"runtime":      "grpc",
		"address":      "127.0.0.1:50052",
		"version":      "2.0.0",
		"config":       map[string]interface{}{"mode": "new"},
		"runtime_params": map[string]interface{}{
			"env": "staging",
		},
		"capabilities": map[string]interface{}{
			"allow_execute_api": false,
		},
		"activate": false,
	}
	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest failed: %v", err)
	}

	zipPath := filepath.Join(t.TempDir(), "plugin-v2.zip")
	if err := writeZipFile(zipPath, map[string]string{
		"manifest.json": string(manifestRaw),
	}); err != nil {
		t.Fatalf("write plugin zip failed: %v", err)
	}

	request := newPluginUploadRequest(t, zipPath, map[string]string{
		"plugin_id": strconv.Itoa(int(existing.ID)),
		"activate":  "false",
	})
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = request

	handler.UploadPluginPackage(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var refreshed models.Plugin
	if err := db.First(&refreshed, existing.ID).Error; err != nil {
		t.Fatalf("reload plugin failed: %v", err)
	}

	if refreshed.Version != existing.Version {
		t.Fatalf("expected plugin version unchanged, got %s", refreshed.Version)
	}
	if refreshed.Address != existing.Address {
		t.Fatalf("expected plugin address unchanged, got %s", refreshed.Address)
	}
	if refreshed.Config != existing.Config {
		t.Fatalf("expected plugin config unchanged, got %s", refreshed.Config)
	}
	if refreshed.RuntimeParams != existing.RuntimeParams {
		t.Fatalf("expected plugin runtime_params unchanged, got %s", refreshed.RuntimeParams)
	}
	if refreshed.Capabilities != existing.Capabilities {
		t.Fatalf("expected plugin capabilities unchanged, got %s", refreshed.Capabilities)
	}
	if refreshed.PackagePath != existing.PackagePath {
		t.Fatalf("expected plugin package_path unchanged, got %s", refreshed.PackagePath)
	}
	if refreshed.PackageChecksum != existing.PackageChecksum {
		t.Fatalf("expected plugin checksum unchanged, got %s", refreshed.PackageChecksum)
	}
	if refreshed.LifecycleStatus != existing.LifecycleStatus {
		t.Fatalf("expected lifecycle unchanged, got %s", refreshed.LifecycleStatus)
	}

	var versions []models.PluginVersion
	if err := db.Where("plugin_id = ?", existing.ID).Order("id ASC").Find(&versions).Error; err != nil {
		t.Fatalf("query plugin versions failed: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected 1 plugin version, got %d", len(versions))
	}

	version := versions[0]
	if version.Version != "2.0.0" {
		t.Fatalf("expected staged version 2.0.0, got %s", version.Version)
	}
	if version.Runtime != service.PluginRuntimeGRPC {
		t.Fatalf("expected version runtime grpc, got %s", version.Runtime)
	}
	if version.Address != "127.0.0.1:50052" {
		t.Fatalf("expected staged address 127.0.0.1:50052, got %s", version.Address)
	}
	if version.IsActive {
		t.Fatalf("expected staged version to remain inactive")
	}

	configMap := decodeJSONMap(t, version.ConfigSnapshot)
	if got := configMap["mode"]; got != "new" {
		t.Fatalf("expected staged config.mode=new, got %v", got)
	}
	runtimeParamsMap := decodeJSONMap(t, version.RuntimeParams)
	if got := runtimeParamsMap["env"]; got != "staging" {
		t.Fatalf("expected staged runtime_params.env=staging, got %v", got)
	}
	capMap := decodeJSONMap(t, version.CapabilitiesSnapshot)
	if got, ok := capMap["allow_execute_api"].(bool); !ok || got {
		t.Fatalf("expected staged capabilities.allow_execute_api=false, got %v", capMap["allow_execute_api"])
	}

	entries, err := os.ReadDir(uploadDir)
	if err != nil {
		t.Fatalf("read upload dir failed: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("expected staged upload artifact to remain on disk")
	}
}

func TestUploadPluginPackageRejectsIncompatibleProtocolVersion(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := openPluginUploadTestDB(t)
	pluginManager := service.NewPluginManagerService(db, &config.Config{
		Plugin: config.PluginPlatformConfig{
			Enabled:         true,
			AllowedRuntimes: []string{service.PluginRuntimeGRPC, service.PluginRuntimeJSWorker},
			DefaultRuntime:  service.PluginRuntimeJSWorker,
		},
	})
	uploadDir := filepath.Join(t.TempDir(), "uploads", "plugins")
	handler := NewPluginHandler(db, pluginManager, uploadDir)

	manifest := map[string]interface{}{
		"name":             "incompatible-protocol-plugin",
		"type":             "custom",
		"runtime":          "js_worker",
		"version":          "1.0.0",
		"manifest_version": service.PluginHostManifestVersion,
		"protocol_version": "1.1.0",
		"activate":         false,
	}
	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest failed: %v", err)
	}

	zipPath := filepath.Join(t.TempDir(), "plugin-incompatible-protocol.zip")
	if err := writeZipFile(zipPath, map[string]string{
		"manifest.json": string(manifestRaw),
		"index.js":      "module.exports.execute = () => ({ success: true });",
	}); err != nil {
		t.Fatalf("write plugin zip failed: %v", err)
	}

	request := newPluginUploadRequest(t, zipPath, map[string]string{
		"activate": "false",
	})
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = request

	handler.UploadPluginPackage(ctx)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if got := strings.TrimSpace(fmt.Sprint(resp["error_key"])); got != "plugin.admin.http_400.invalid_package_manifest_schema" {
		t.Fatalf("expected invalid_package_manifest_schema, got %q, body=%s", got, rec.Body.String())
	}
	params, ok := resp["error_params"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected error_params object, body=%s", rec.Body.String())
	}
	if got := strings.TrimSpace(fmt.Sprint(params["path"])); got != "protocol_version" {
		t.Fatalf("expected protocol_version path, got %q", got)
	}
}

func TestBuildPluginPackagePathDisplay(t *testing.T) {
	artifactRoot := filepath.Join(t.TempDir(), "uploads", "plugins")
	insidePath := filepath.Join(artifactRoot, "jsworker", "demo", "package.zip")
	outsidePath := filepath.Join(t.TempDir(), "external", "package.zip")

	tests := []struct {
		name     string
		rawPath  string
		expected string
	}{
		{
			name:     "empty path",
			rawPath:  "",
			expected: "",
		},
		{
			name:     "relative path stays relative",
			rawPath:  filepath.Join("uploads", "plugins", "package.zip"),
			expected: "uploads/plugins/package.zip",
		},
		{
			name:     "absolute path within artifact root becomes relative",
			rawPath:  insidePath,
			expected: "jsworker/demo/package.zip",
		},
		{
			name:     "absolute path outside artifact root is redacted",
			rawPath:  outsidePath,
			expected: ".../package.zip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildPluginPackagePathDisplay(tt.rawPath, artifactRoot)
			if got != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestBuildPluginAddressDisplay(t *testing.T) {
	artifactRoot := filepath.Join(t.TempDir(), "uploads", "plugins")
	absoluteScriptPath := filepath.Join(artifactRoot, "jsworker", "pkg", "demo", "root", "index.js")

	tests := []struct {
		name     string
		runtime  string
		address  string
		expected string
	}{
		{
			name:     "grpc address stays unchanged",
			runtime:  "grpc",
			address:  "127.0.0.1:50051",
			expected: "127.0.0.1:50051",
		},
		{
			name:     "js worker relative entry stays relative",
			runtime:  "js_worker",
			address:  "index.js",
			expected: "index.js",
		},
		{
			name:     "js worker absolute entry is redacted to display path",
			runtime:  "js_worker",
			address:  absoluteScriptPath,
			expected: "jsworker/pkg/demo/root/index.js",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildPluginAddressDisplay(tt.runtime, tt.address, artifactRoot)
			if got != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func writeZipFile(zipPath string, files map[string]string) error {
	file, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer file.Close()

	zipWriter := zip.NewWriter(file)
	for name, content := range files {
		entryWriter, createErr := zipWriter.Create(name)
		if createErr != nil {
			_ = zipWriter.Close()
			return createErr
		}
		if _, writeErr := entryWriter.Write([]byte(content)); writeErr != nil {
			_ = zipWriter.Close()
			return writeErr
		}
	}
	return zipWriter.Close()
}

func assertDirEmptyOrMissing(t *testing.T, dir string) {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		t.Fatalf("read upload dir failed: %v", err)
	}
	if len(entries) == 0 {
		return
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	t.Fatalf("expected upload dir %s to be empty, found %v", dir, names)
}

func newPluginUploadRequest(t *testing.T, filePath string, fields map[string]string) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		t.Fatalf("create form file failed: %v", err)
	}

	src, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("open zip file failed: %v", err)
	}
	defer src.Close()

	if _, err := io.Copy(part, src); err != nil {
		t.Fatalf("copy zip file failed: %v", err)
	}

	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("write form field %s failed: %v", key, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/admin/plugins/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func openPluginUploadTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "plugin-upload-test.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db failed: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})
	if err := db.AutoMigrate(&models.Plugin{}, &models.PluginVersion{}, &models.PluginSecretEntry{}, &models.OperationLog{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	return db
}

func decodeJSONMap(t *testing.T, raw string) map[string]interface{} {
	t.Helper()

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("decode json map failed: %v, raw=%s", err, raw)
	}
	return out
}
