package service

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"auralogic/internal/config"
	"auralogic/internal/models"
)

func TestExecutePluginHostActionListsConfiguredMarketSources(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)

	marketPlugin := models.Plugin{
		Name:    "market-plugin",
		Runtime: PluginRuntimeJSWorker,
		Address: "index.js",
		Config: `{
			"market": {
				"sources": [
					{
						"source_id": "official",
						"name": "Official",
						"source_base_url": "https://d.auralogic.org",
						"source_public_key": "PUBKEY",
						"default_channel": "stable",
						"allowed_kinds": ["plugin_package", "email_template"]
					}
				]
			}
		}`,
	}
	if err := db.Create(&marketPlugin).Error; err != nil {
		t.Fatalf("create market plugin failed: %v", err)
	}

	result, err := ExecutePluginHostAction(db, &PluginHostAccessClaims{
		PluginID:           marketPlugin.ID,
		GrantedPermissions: []string{PluginPermissionHostMarketSourceRead},
		ScopeAuthenticated: true,
		ScopeSuperAdmin:    true,
	}, "host.market.source.list", map[string]interface{}{})
	if err != nil {
		t.Fatalf("ExecutePluginHostAction returned error: %v", err)
	}

	items, ok := result["items"].([]map[string]interface{})
	if !ok {
		rawItems, rawOK := result["items"].([]interface{})
		if !rawOK || len(rawItems) != 1 {
			t.Fatalf("expected single market source, got %#v", result["items"])
		}
		first, firstOK := rawItems[0].(map[string]interface{})
		if !firstOK {
			t.Fatalf("expected first market source as object, got %#v", rawItems[0])
		}
		if got := first["source_id"]; got != "official" {
			t.Fatalf("expected source_id=official, got %#v", got)
		}
		return
	}
	if len(items) != 1 {
		t.Fatalf("expected single market source, got %#v", items)
	}
	if got := items[0]["source_id"]; got != "official" {
		t.Fatalf("expected source_id=official, got %#v", got)
	}
}

func TestExecutePluginHostActionFetchesMarketCatalogFromConfiguredSource(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/catalog" {
			t.Fatalf("expected /v1/catalog path, got %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("kind"); got != "plugin_package" {
			t.Fatalf("expected kind query plugin_package, got %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"items": []map[string]interface{}{
					{
						"kind": "plugin_package",
						"name": "debugger",
					},
				},
				"pagination": map[string]interface{}{
					"offset":   0,
					"limit":    20,
					"total":    1,
					"has_more": false,
				},
			},
		})
	}))
	defer server.Close()

	db := openPluginManagerE2ETestDB(t)
	marketPlugin := models.Plugin{
		Name:    "market-plugin",
		Runtime: PluginRuntimeJSWorker,
		Address: "index.js",
		Config: `{
			"market": {
				"sources": [
					{
						"source_id": "official",
						"name": "Official",
						"source_base_url": "` + server.URL + `",
						"allowed_kinds": ["plugin_package"]
					}
				]
			}
		}`,
	}
	if err := db.Create(&marketPlugin).Error; err != nil {
		t.Fatalf("create market plugin failed: %v", err)
	}

	result, err := ExecutePluginHostAction(db, &PluginHostAccessClaims{
		PluginID: marketPlugin.ID,
		GrantedPermissions: []string{
			PluginPermissionHostMarketCatalogRead,
		},
		ScopeAuthenticated: true,
		ScopeSuperAdmin:    true,
	}, "host.market.catalog.list", map[string]interface{}{
		"source_id": "official",
		"kind":      "plugin_package",
		"limit":     20,
	})
	if err != nil {
		t.Fatalf("ExecutePluginHostAction returned error: %v", err)
	}

	source, ok := result["source"].(map[string]interface{})
	if !ok || source["source_id"] != "official" {
		t.Fatalf("expected embedded source summary, got %#v", result["source"])
	}
	items, ok := result["items"].([]interface{})
	if !ok || len(items) != 1 {
		t.Fatalf("expected single catalog item, got %#v", result["items"])
	}
	item, ok := items[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected catalog item object, got %#v", items[0])
	}
	if got := item["name"]; got != "debugger" {
		t.Fatalf("expected item name debugger, got %#v", got)
	}
}

func TestExecutePluginHostActionPreviewsPluginPackageRelease(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/artifacts/plugin_package/demo-market/releases/1.2.0" {
			t.Fatalf("unexpected release path %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"kind":    "plugin_package",
				"name":    "demo-market",
				"version": "1.2.0",
				"governance": map[string]interface{}{
					"mode": "host_managed",
				},
				"download": map[string]interface{}{
					"sha256": "abc123",
					"size":   1024,
				},
				"compatibility": map[string]interface{}{
					"min_host_bridge_version": "1.0.0",
				},
				"permissions": map[string]interface{}{
					"requested":       []string{PluginPermissionHostOrderRead, PluginPermissionHostUserRead},
					"default_granted": []string{PluginPermissionHostOrderRead},
				},
			},
		})
	}))
	defer server.Close()

	db := openPluginManagerE2ETestDB(t)

	marketPlugin := models.Plugin{
		Name:    "market-plugin",
		Runtime: PluginRuntimeJSWorker,
		Address: "index.js",
		Config: `{
			"sources": [
				{
					"source_id": "official",
					"source_base_url": "` + server.URL + `",
					"allowed_kinds": ["plugin_package"]
				}
			]
		}`,
	}
	if err := db.Create(&marketPlugin).Error; err != nil {
		t.Fatalf("create market plugin failed: %v", err)
	}

	capabilities := map[string]interface{}{
		"requested_permissions": []string{PluginPermissionHostOrderRead},
		"granted_permissions":   []string{PluginPermissionHostOrderRead},
	}
	capabilitiesRaw, err := json.Marshal(capabilities)
	if err != nil {
		t.Fatalf("marshal capabilities failed: %v", err)
	}
	targetPlugin := models.Plugin{
		Name:         "demo-market",
		Version:      "1.0.0",
		Runtime:      PluginRuntimeJSWorker,
		Address:      "index.js",
		Capabilities: string(capabilitiesRaw),
	}
	if err := db.Create(&targetPlugin).Error; err != nil {
		t.Fatalf("create target plugin failed: %v", err)
	}

	result, err := ExecutePluginHostAction(db, &PluginHostAccessClaims{
		PluginID: marketPlugin.ID,
		GrantedPermissions: []string{
			PluginPermissionHostMarketInstallPreview,
		},
		ScopeAuthenticated: true,
		ScopeSuperAdmin:    true,
	}, "host.market.install.preview", map[string]interface{}{
		"source_id": "official",
		"kind":      "plugin_package",
		"name":      "demo-market",
		"version":   "1.2.0",
	})
	if err != nil {
		t.Fatalf("ExecutePluginHostAction returned error: %v", err)
	}

	targetState, ok := result["target_state"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected target_state object, got %#v", result["target_state"])
	}
	if got := targetState["installed"]; got != true {
		t.Fatalf("expected installed target state, got %#v", got)
	}
	if got := targetState["current_version"]; got != "1.0.0" {
		t.Fatalf("expected current_version 1.0.0, got %#v", got)
	}

	permissions, ok := result["permissions"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected permissions preview, got %#v", result["permissions"])
	}
	if got := permissions["requires_reconfirm"]; got != true {
		t.Fatalf("expected requires_reconfirm=true, got %#v", got)
	}
	newPermissions, ok := permissions["new_permissions"].([]string)
	if !ok {
		rawValues, rawOK := permissions["new_permissions"].([]interface{})
		if !rawOK || len(rawValues) != 1 || rawValues[0] != PluginPermissionHostUserRead {
			t.Fatalf("expected host.user.read as new permission, got %#v", permissions["new_permissions"])
		}
		return
	}
	if len(newPermissions) != 1 || newPermissions[0] != PluginPermissionHostUserRead {
		t.Fatalf("expected host.user.read as new permission, got %#v", newPermissions)
	}
}

func TestExecutePluginHostActionExecutesPluginPackageInstall(t *testing.T) {
	packageBytes := buildPluginMarketTestZip(t, map[string]string{
		"manifest.json": `{
			"name": "demo-market",
			"display_name": "Demo Market Plugin",
			"description": "Installed from market bridge test",
			"type": "custom",
			"runtime": "js_worker",
			"entry": "index.js",
			"version": "1.2.0",
			"capabilities": {}
		}`,
		"index.js": `
module.exports.health = function health() {
  return {
    healthy: true,
    version: "market-test/1.2.0",
    metadata: { runtime: "goja" }
  };
};
module.exports.execute = function execute(action) {
  return { success: true, data: { action: action, source: "market" } };
};
`,
	})

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/artifacts/plugin_package/demo-market/releases/1.2.0":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"data": map[string]interface{}{
					"kind":    "plugin_package",
					"name":    "demo-market",
					"version": "1.2.0",
					"governance": map[string]interface{}{
						"mode": "host_managed",
					},
					"download": map[string]interface{}{
						"url":    server.URL + "/download/demo-market-1.2.0.zip",
						"size":   len(packageBytes),
						"sha256": computeSHA256Hex(packageBytes),
					},
					"install": map[string]interface{}{
						"package_format":        "zip",
						"entry":                 "index.js",
						"auto_activate_default": true,
						"auto_start_default":    true,
					},
					"compatibility": map[string]interface{}{
						"min_host_bridge_version": "1.0.0",
					},
				},
			})
		case "/download/demo-market-1.2.0.zip":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(packageBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.PluginVersion{}, &models.PluginDeployment{}); err != nil {
		t.Fatalf("auto migrate plugin versions failed: %v", err)
	}

	artifactRoot := t.TempDir()
	workerAddr := startTestJSWorker(t, artifactRoot)
	manager := NewPluginManagerService(db, &config.Config{
		Plugin: config.PluginPlatformConfig{
			Enabled:         true,
			AllowedRuntimes: []string{PluginRuntimeJSWorker},
			DefaultRuntime:  PluginRuntimeJSWorker,
			ArtifactDir:     artifactRoot,
			Sandbox: config.PluginSandboxConfig{
				ExecTimeoutMs:      30000,
				MaxMemoryMB:        128,
				MaxConcurrency:     4,
				JSWorkerSocketPath: "tcp://" + workerAddr,
				JSWorkerAutoStart:  false,
			},
		},
	})
	manager.Start()
	defer manager.Stop()

	marketPlugin := models.Plugin{
		Name:    "market-plugin",
		Runtime: PluginRuntimeJSWorker,
		Address: "index.js",
		Enabled: false,
		Config: `{
			"sources": [
				{
					"source_id": "official",
					"source_base_url": "` + server.URL + `",
					"allowed_kinds": ["plugin_package"]
				}
			]
		}`,
	}
	if err := db.Create(&marketPlugin).Error; err != nil {
		t.Fatalf("create market plugin failed: %v", err)
	}

	result, err := ExecutePluginHostActionWithRuntime(
		NewPluginHostRuntime(db, manager.cfg, manager),
		&PluginHostAccessClaims{
			PluginID: marketPlugin.ID,
			GrantedPermissions: []string{
				PluginPermissionHostMarketInstallExecute,
			},
			ScopeAuthenticated: true,
			ScopeSuperAdmin:    true,
			OperatorUserID:     1,
		},
		"host.market.install.execute",
		map[string]interface{}{
			"source_id": "official",
			"kind":      "plugin_package",
			"name":      "demo-market",
			"version":   "1.2.0",
			"options": map[string]interface{}{
				"activate":   true,
				"auto_start": true,
			},
		},
	)
	if err != nil {
		t.Fatalf("ExecutePluginHostActionWithRuntime returned error: %v", err)
	}

	if got := result["status"]; got != "activated" {
		t.Fatalf("expected activated status, got %#v", got)
	}
	taskID, ok := result["task_id"].(string)
	if !ok || taskID == "" {
		t.Fatalf("expected task_id in install result, got %#v", result["task_id"])
	}

	pluginResp, ok := result["plugin"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected plugin response object, got %#v", result["plugin"])
	}
	deploymentResp, ok := result["deployment"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected deployment response object, got %#v", result["deployment"])
	}
	pluginID := uint(interfaceToTestInt64(pluginResp["id"]))
	if pluginID == 0 {
		t.Fatalf("expected installed plugin id, got %#v", pluginResp)
	}

	var installed models.Plugin
	if err := db.First(&installed, pluginID).Error; err != nil {
		t.Fatalf("load installed plugin failed: %v", err)
	}
	if installed.Name != "demo-market" || installed.Runtime != PluginRuntimeJSWorker {
		t.Fatalf("unexpected installed plugin snapshot: %+v", installed)
	}
	if !installed.Enabled {
		t.Fatalf("expected installed plugin to be enabled after auto_start")
	}
	if _, err := os.Stat(filepath.FromSlash(installed.PackagePath)); err != nil {
		t.Fatalf("expected downloaded package to exist, got %v", err)
	}

	var version models.PluginVersion
	if err := db.Where("plugin_id = ?", installed.ID).First(&version).Error; err != nil {
		t.Fatalf("load installed plugin version failed: %v", err)
	}
	if !version.IsActive {
		t.Fatalf("expected installed plugin version to be active, got %+v", version)
	}
	if version.MarketSourceID != "official" || version.MarketArtifactKind != "plugin_package" || version.MarketArtifactName != "demo-market" || version.MarketArtifactVersion != "1.2.0" {
		t.Fatalf("expected persisted market coordinates, got %+v", version)
	}

	var deployment models.PluginDeployment
	if err := db.First(&deployment, interfaceToTestInt64(deploymentResp["id"])).Error; err != nil {
		t.Fatalf("load deployment failed: %v", err)
	}
	if deployment.MarketSourceID != "official" || deployment.MarketArtifactVersion != "1.2.0" {
		t.Fatalf("expected deployment market metadata, got %+v", deployment)
	}

	health, err := manager.TestPlugin(installed.ID)
	if err != nil {
		t.Fatalf("TestPlugin failed: %v", err)
	}
	if health == nil || !health.Healthy {
		t.Fatalf("expected installed plugin healthy, got %+v", health)
	}
	if health.Version != "market-test/1.2.0" {
		t.Fatalf("expected health version market-test/1.2.0, got %q", health.Version)
	}
}

func TestExecutePluginHostActionExecutesPluginPackageInstallFromCanonicalDownloadRoute(t *testing.T) {
	packageBytes := buildPluginMarketTestZip(t, map[string]string{
		"manifest.json": `{
			"name": "demo-market",
			"display_name": "Demo Market Plugin",
			"description": "Installed from canonical download route",
			"type": "custom",
			"runtime": "js_worker",
			"entry": "index.js",
			"version": "1.2.1",
			"capabilities": {}
		}`,
		"index.js": `
module.exports.health = function health() {
  return {
    healthy: true,
    version: "market-test/1.2.1",
    metadata: { runtime: "goja" }
  };
};
`,
	})

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/artifacts/plugin_package/demo-market/releases/1.2.1":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"data": map[string]interface{}{
					"kind":    "plugin_package",
					"name":    "demo-market",
					"version": "1.2.1",
					"governance": map[string]interface{}{
						"mode": "host_managed",
					},
					"download": map[string]interface{}{
						"url":          server.URL + "/v1/artifacts/plugin_package/demo-market/releases/1.2.1/download",
						"size":         len(packageBytes),
						"content_type": "application/zip",
						"sha256":       computeSHA256Hex(packageBytes),
					},
					"install": map[string]interface{}{
						"package_format":        "zip",
						"entry":                 "index.js",
						"auto_activate_default": true,
						"auto_start_default":    true,
					},
					"compatibility": map[string]interface{}{
						"min_host_bridge_version": "1.0.0",
					},
				},
			})
		case "/v1/artifacts/plugin_package/demo-market/releases/1.2.1/download":
			w.Header().Set("Content-Type", "application/zip")
			w.Header().Set("Content-Disposition", `attachment; filename="demo-market-1.2.1.zip"`)
			_, _ = w.Write(packageBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.PluginVersion{}, &models.PluginDeployment{}); err != nil {
		t.Fatalf("auto migrate plugin versions failed: %v", err)
	}

	artifactRoot := t.TempDir()
	workerAddr := startTestJSWorker(t, artifactRoot)
	manager := NewPluginManagerService(db, &config.Config{
		Plugin: config.PluginPlatformConfig{
			Enabled:         true,
			AllowedRuntimes: []string{PluginRuntimeJSWorker},
			DefaultRuntime:  PluginRuntimeJSWorker,
			ArtifactDir:     artifactRoot,
			Sandbox: config.PluginSandboxConfig{
				ExecTimeoutMs:      30000,
				MaxMemoryMB:        128,
				MaxConcurrency:     4,
				JSWorkerSocketPath: "tcp://" + workerAddr,
				JSWorkerAutoStart:  false,
			},
		},
	})
	manager.Start()
	defer manager.Stop()

	marketPlugin := models.Plugin{
		Name:    "market-plugin",
		Runtime: PluginRuntimeJSWorker,
		Address: "index.js",
		Enabled: false,
		Config: `{
			"sources": [
				{
					"source_id": "official",
					"source_base_url": "` + server.URL + `",
					"allowed_kinds": ["plugin_package"]
				}
			]
		}`,
	}
	if err := db.Create(&marketPlugin).Error; err != nil {
		t.Fatalf("create market plugin failed: %v", err)
	}

	_, err := ExecutePluginHostActionWithRuntime(
		NewPluginHostRuntime(db, manager.cfg, manager),
		&PluginHostAccessClaims{
			PluginID: marketPlugin.ID,
			GrantedPermissions: []string{
				PluginPermissionHostMarketInstallExecute,
			},
			ScopeAuthenticated: true,
			ScopeSuperAdmin:    true,
			OperatorUserID:     1,
		},
		"host.market.install.execute",
		map[string]interface{}{
			"source_id": "official",
			"kind":      "plugin_package",
			"name":      "demo-market",
			"version":   "1.2.1",
			"options": map[string]interface{}{
				"activate":   true,
				"auto_start": true,
			},
		},
	)
	if err != nil {
		t.Fatalf("ExecutePluginHostActionWithRuntime returned error: %v", err)
	}

	var installed models.Plugin
	if err := db.Where("name = ?", "demo-market").First(&installed).Error; err != nil {
		t.Fatalf("load installed plugin failed: %v", err)
	}
	if got := filepath.Base(filepath.FromSlash(installed.PackagePath)); got != "demo-market-1.2.1.zip" {
		t.Fatalf("expected normalized package filename demo-market-1.2.1.zip, got %q", got)
	}
	if got := filepath.Ext(filepath.FromSlash(installed.PackagePath)); got != ".zip" {
		t.Fatalf("expected downloaded package extension .zip, got %q", got)
	}
	if _, err := os.Stat(filepath.FromSlash(installed.PackagePath)); err != nil {
		t.Fatalf("expected downloaded package to exist, got %v", err)
	}

	health, err := manager.TestPlugin(installed.ID)
	if err != nil {
		t.Fatalf("TestPlugin failed: %v", err)
	}
	if health == nil || !health.Healthy {
		t.Fatalf("expected installed plugin healthy, got %+v", health)
	}
	if health.Version != "market-test/1.2.1" {
		t.Fatalf("expected health version market-test/1.2.1, got %q", health.Version)
	}
}

func TestExecutePluginHostActionManagesPluginPackageTasksHistoryAndRollback(t *testing.T) {
	packageV110 := buildPluginMarketTestZip(t, map[string]string{
		"manifest.json": `{
			"name": "demo-market",
			"display_name": "Demo Market Plugin",
			"description": "Installed from market bridge test",
			"type": "custom",
			"runtime": "js_worker",
			"entry": "index.js",
			"version": "1.1.0",
			"capabilities": {}
		}`,
		"index.js": `
module.exports.health = function health() {
  return {
    healthy: true,
    version: "market-test/1.1.0",
    metadata: { runtime: "goja" }
  };
};
module.exports.execute = function execute(action) {
  return { success: true, data: { action: action, source: "market", version: "1.1.0" } };
};
`,
	})
	packageV120 := buildPluginMarketTestZip(t, map[string]string{
		"manifest.json": `{
			"name": "demo-market",
			"display_name": "Demo Market Plugin",
			"description": "Installed from market bridge test",
			"type": "custom",
			"runtime": "js_worker",
			"entry": "index.js",
			"version": "1.2.0",
			"capabilities": {}
		}`,
		"index.js": `
module.exports.health = function health() {
  return {
    healthy: true,
    version: "market-test/1.2.0",
    metadata: { runtime: "goja" }
  };
};
module.exports.execute = function execute(action) {
  return { success: true, data: { action: action, source: "market", version: "1.2.0" } };
};
`,
	})

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/artifacts/plugin_package/demo-market/releases/1.1.0":
			_ = json.NewEncoder(w).Encode(buildPluginMarketReleaseEnvelope(server.URL, "1.1.0", packageV110))
		case "/v1/artifacts/plugin_package/demo-market/releases/1.2.0":
			_ = json.NewEncoder(w).Encode(buildPluginMarketReleaseEnvelope(server.URL, "1.2.0", packageV120))
		case "/download/demo-market-1.1.0.zip":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(packageV110)
		case "/download/demo-market-1.2.0.zip":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(packageV120)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.PluginVersion{}, &models.PluginDeployment{}); err != nil {
		t.Fatalf("auto migrate plugin versions failed: %v", err)
	}

	artifactRoot := t.TempDir()
	workerAddr := startTestJSWorker(t, artifactRoot)
	manager := NewPluginManagerService(db, &config.Config{
		Plugin: config.PluginPlatformConfig{
			Enabled:         true,
			AllowedRuntimes: []string{PluginRuntimeJSWorker},
			DefaultRuntime:  PluginRuntimeJSWorker,
			ArtifactDir:     artifactRoot,
			Sandbox: config.PluginSandboxConfig{
				ExecTimeoutMs:      30000,
				MaxMemoryMB:        128,
				MaxConcurrency:     4,
				JSWorkerSocketPath: "tcp://" + workerAddr,
				JSWorkerAutoStart:  false,
			},
		},
	})
	manager.Start()
	defer manager.Stop()

	marketPlugin := models.Plugin{
		Name:    "market-plugin",
		Runtime: PluginRuntimeJSWorker,
		Address: "index.js",
		Enabled: false,
		Config: `{
			"sources": [
				{
					"source_id": "official",
					"source_base_url": "` + server.URL + `",
					"allowed_kinds": ["plugin_package"]
				}
			]
		}`,
	}
	if err := db.Create(&marketPlugin).Error; err != nil {
		t.Fatalf("create market plugin failed: %v", err)
	}

	runtime := NewPluginHostRuntime(db, manager.cfg, manager)
	executeClaims := &PluginHostAccessClaims{
		PluginID: marketPlugin.ID,
		GrantedPermissions: []string{
			PluginPermissionHostMarketInstallExecute,
		},
		ScopeAuthenticated: true,
		ScopeSuperAdmin:    true,
		OperatorUserID:     1,
	}
	readClaims := &PluginHostAccessClaims{
		PluginID: marketPlugin.ID,
		GrantedPermissions: []string{
			PluginPermissionHostMarketInstallRead,
		},
		ScopeAuthenticated: true,
		ScopeSuperAdmin:    true,
		OperatorUserID:     1,
	}
	rollbackClaims := &PluginHostAccessClaims{
		PluginID: marketPlugin.ID,
		GrantedPermissions: []string{
			PluginPermissionHostMarketInstallRollback,
		},
		ScopeAuthenticated: true,
		ScopeSuperAdmin:    true,
		OperatorUserID:     1,
	}

	install := func(version string) map[string]interface{} {
		t.Helper()
		result, err := ExecutePluginHostActionWithRuntime(
			runtime,
			executeClaims,
			"host.market.install.execute",
			map[string]interface{}{
				"source_id": "official",
				"kind":      "plugin_package",
				"name":      "demo-market",
				"version":   version,
				"options": map[string]interface{}{
					"activate":   true,
					"auto_start": true,
				},
			},
		)
		if err != nil {
			t.Fatalf("install %s returned error: %v", version, err)
		}
		return result
	}

	result110 := install("1.1.0")
	if got := result110["status"]; got != "activated" {
		t.Fatalf("expected first install activated, got %#v", got)
	}

	result120 := install("1.2.0")
	if got := result120["status"]; got != "activated" {
		t.Fatalf("expected second install activated, got %#v", got)
	}

	taskID, ok := result120["task_id"].(string)
	if !ok || taskID == "" {
		t.Fatalf("expected second install task_id, got %#v", result120["task_id"])
	}

	taskResult, err := ExecutePluginHostActionWithRuntime(
		runtime,
		readClaims,
		"host.market.install.task.get",
		map[string]interface{}{
			"task_id": taskID,
		},
	)
	if err != nil {
		t.Fatalf("task.get returned error: %v", err)
	}
	if got := taskResult["status"]; got != "succeeded" {
		t.Fatalf("expected task status succeeded, got %#v", got)
	}
	taskCoords, ok := taskResult["coordinates"].(map[string]interface{})
	if !ok || taskCoords["version"] != "1.2.0" {
		t.Fatalf("expected task coordinates version 1.2.0, got %#v", taskResult["coordinates"])
	}

	taskListResult, err := ExecutePluginHostActionWithRuntime(
		runtime,
		readClaims,
		"host.market.install.task.list",
		map[string]interface{}{
			"source_id": "official",
			"kind":      "plugin_package",
			"name":      "demo-market",
			"limit":     10,
		},
	)
	if err != nil {
		t.Fatalf("task.list returned error: %v", err)
	}
	taskItems, ok := taskListResult["items"].([]map[string]interface{})
	if !ok {
		rawItems, rawOK := taskListResult["items"].([]interface{})
		if !rawOK || len(rawItems) < 2 {
			t.Fatalf("expected at least two task items, got %#v", taskListResult["items"])
		}
		firstItem, firstOK := rawItems[0].(map[string]interface{})
		if !firstOK || firstItem["task_id"] != taskID {
			t.Fatalf("expected first task item to match latest task_id, got %#v", rawItems[0])
		}
	} else {
		if len(taskItems) < 2 {
			t.Fatalf("expected at least two task items, got %#v", taskItems)
		}
		if taskItems[0]["task_id"] != taskID {
			t.Fatalf("expected latest task_id first, got %#v", taskItems[0])
		}
	}

	historyResult, err := ExecutePluginHostActionWithRuntime(
		runtime,
		readClaims,
		"host.market.install.history.list",
		map[string]interface{}{
			"source_id": "official",
			"kind":      "plugin_package",
			"name":      "demo-market",
			"limit":     10,
		},
	)
	if err != nil {
		t.Fatalf("history.list returned error: %v", err)
	}
	historyItems, ok := historyResult["items"].([]map[string]interface{})
	if !ok {
		rawItems, rawOK := historyResult["items"].([]interface{})
		if !rawOK || len(rawItems) < 2 {
			t.Fatalf("expected at least two history items, got %#v", historyResult["items"])
		}
		firstHistory, firstOK := rawItems[0].(map[string]interface{})
		if !firstOK || firstHistory["version"] != "1.2.0" {
			t.Fatalf("expected latest history item version 1.2.0, got %#v", rawItems[0])
		}
	} else {
		if len(historyItems) < 2 {
			t.Fatalf("expected at least two history items, got %#v", historyItems)
		}
		if historyItems[0]["version"] != "1.2.0" {
			t.Fatalf("expected latest history item version 1.2.0, got %#v", historyItems[0])
		}
	}

	rollbackResult, err := ExecutePluginHostActionWithRuntime(
		runtime,
		rollbackClaims,
		"host.market.install.rollback",
		map[string]interface{}{
			"source_id": "official",
			"kind":      "plugin_package",
			"name":      "demo-market",
			"version":   "1.1.0",
		},
	)
	if err != nil {
		t.Fatalf("rollback returned error: %v", err)
	}
	if got := rollbackResult["status"]; got != "rolled_back" {
		t.Fatalf("expected rolled_back status, got %#v", got)
	}

	var installed models.Plugin
	if err := db.Where("name = ?", "demo-market").First(&installed).Error; err != nil {
		t.Fatalf("load installed plugin failed: %v", err)
	}
	if installed.Version != "1.1.0" {
		t.Fatalf("expected plugin version rolled back to 1.1.0, got %+v", installed)
	}

	var activeVersion models.PluginVersion
	if err := db.Where("plugin_id = ? AND is_active = ?", installed.ID, true).First(&activeVersion).Error; err != nil {
		t.Fatalf("load active plugin version failed: %v", err)
	}
	if activeVersion.Version != "1.1.0" {
		t.Fatalf("expected active plugin version 1.1.0 after rollback, got %+v", activeVersion)
	}

	health, err := manager.TestPlugin(installed.ID)
	if err != nil {
		t.Fatalf("TestPlugin failed after rollback: %v", err)
	}
	if health == nil || !health.Healthy {
		t.Fatalf("expected rolled back plugin healthy, got %+v", health)
	}
	if health.Version != "market-test/1.1.0" {
		t.Fatalf("expected health version market-test/1.1.0 after rollback, got %q", health.Version)
	}
}

func buildPluginMarketReleaseEnvelope(baseURL string, version string, packageBytes []byte) map[string]interface{} {
	return map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"kind":    "plugin_package",
			"name":    "demo-market",
			"version": version,
			"governance": map[string]interface{}{
				"mode": "host_managed",
			},
			"download": map[string]interface{}{
				"url":    baseURL + "/download/demo-market-" + version + ".zip",
				"size":   len(packageBytes),
				"sha256": computeSHA256Hex(packageBytes),
			},
			"install": map[string]interface{}{
				"package_format":        "zip",
				"entry":                 "index.js",
				"auto_activate_default": true,
				"auto_start_default":    true,
			},
			"compatibility": map[string]interface{}{
				"min_host_bridge_version": "1.0.0",
			},
		},
	}
}

func buildPluginMarketTestZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, body := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s failed: %v", name, err)
		}
		if _, err := entry.Write([]byte(body)); err != nil {
			t.Fatalf("write zip entry %s failed: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip writer failed: %v", err)
	}
	return buffer.Bytes()
}

func computeSHA256Hex(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func TestPluginHostMarketPrepareJSWorkerPackageSupportsExtensionlessZipPath(t *testing.T) {
	packageBytes := buildPluginMarketTestZip(t, map[string]string{
		"manifest.json": `{
			"name": "extensionless-demo",
			"display_name": "Extensionless Demo",
			"runtime": "js_worker",
			"entry": "index.js",
			"version": "1.0.0"
		}`,
		"index.js": `module.exports = {};`,
	})

	packagePath := filepath.Join(t.TempDir(), "download")
	if err := os.WriteFile(packagePath, packageBytes, 0o644); err != nil {
		t.Fatalf("write extensionless package failed: %v", err)
	}

	expectedPackagePath, err := NormalizeJSWorkerPackagePath(packagePath)
	if err != nil {
		t.Fatalf("NormalizeJSWorkerPackagePath returned error: %v", err)
	}

	address, normalizedPackagePath, extractRoot, err := pluginHostMarketPrepareJSWorkerPackage(packagePath, "")
	if err != nil {
		t.Fatalf("pluginHostMarketPrepareJSWorkerPackage returned error: %v", err)
	}
	if address != "index.js" {
		t.Fatalf("expected detected entry index.js, got %q", address)
	}
	if normalizedPackagePath != expectedPackagePath {
		t.Fatalf("expected normalized package path %q, got %q", expectedPackagePath, normalizedPackagePath)
	}
	if filepath.Clean(filepath.FromSlash(extractRoot)) == filepath.Clean(expectedPackagePath) {
		t.Fatalf("expected extract root to differ from package path, got %q", extractRoot)
	}
	if _, err := os.Stat(expectedPackagePath); err != nil {
		t.Fatalf("expected original package file to remain after prepare, got %v", err)
	}
	if info, err := os.Stat(filepath.FromSlash(extractRoot)); err != nil {
		t.Fatalf("expected extract root to exist, got %v", err)
	} else if !info.IsDir() {
		t.Fatalf("expected extract root directory, got file at %q", extractRoot)
	}
	if _, err := os.Stat(filepath.Join(filepath.FromSlash(extractRoot), "index.js")); err != nil {
		t.Fatalf("expected extracted entry script to exist, got %v", err)
	}
}

func TestPluginHostMarketPrepareJSWorkerPackageAutoDetectsIndexMJS(t *testing.T) {
	packageBytes := buildPluginMarketTestZip(t, map[string]string{
		"manifest.json": `{
			"name": "index-mjs-demo",
			"display_name": "Index MJS Demo",
			"runtime": "js_worker",
			"version": "1.0.0"
		}`,
		"helpers/bootstrap.js": `module.exports = {};`,
		"index.mjs":            `export function execute() { return { success: true }; }`,
	})

	packagePath := filepath.Join(t.TempDir(), "index-mjs.zip")
	if err := os.WriteFile(packagePath, packageBytes, 0o644); err != nil {
		t.Fatalf("write package failed: %v", err)
	}

	address, _, extractRoot, err := pluginHostMarketPrepareJSWorkerPackage(packagePath, "")
	if err != nil {
		t.Fatalf("pluginHostMarketPrepareJSWorkerPackage returned error: %v", err)
	}
	if address != "index.mjs" {
		t.Fatalf("expected detected entry index.mjs, got %q", address)
	}
	if _, err := os.Stat(filepath.Join(filepath.FromSlash(extractRoot), "index.mjs")); err != nil {
		t.Fatalf("expected extracted entry script to exist, got %v", err)
	}
}
