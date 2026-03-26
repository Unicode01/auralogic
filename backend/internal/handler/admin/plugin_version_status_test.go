package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"auralogic/internal/config"
	"auralogic/internal/models"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
)

func TestGetPluginVersionsUsesCurrentPluginLifecycleForActiveVersion(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := openPluginUploadTestDB(t)
	handler := NewPluginHandler(db, service.NewPluginManagerService(db, &config.Config{
		Plugin: config.PluginPlatformConfig{
			Enabled:         true,
			AllowedRuntimes: []string{service.PluginRuntimeGRPC, service.PluginRuntimeJSWorker},
			DefaultRuntime:  service.PluginRuntimeJSWorker,
		},
	}), filepath.Join(t.TempDir(), "uploads", "plugins"))

	plugin := models.Plugin{
		Name:            "versions-lifecycle-plugin",
		DisplayName:     "Versions Lifecycle Plugin",
		Type:            "custom",
		Runtime:         service.PluginRuntimeJSWorker,
		Address:         "index.js",
		Enabled:         true,
		LifecycleStatus: models.PluginLifecycleRunning,
	}
	if err := db.Create(&plugin).Error; err != nil {
		t.Fatalf("create plugin failed: %v", err)
	}

	activeVersion := models.PluginVersion{
		PluginID:        plugin.ID,
		Version:         "1.0.0",
		PackageName:     "versions-lifecycle-plugin",
		LifecycleStatus: models.PluginLifecyclePaused,
		IsActive:        true,
	}
	if err := db.Create(&activeVersion).Error; err != nil {
		t.Fatalf("create active version failed: %v", err)
	}

	inactiveVersion := models.PluginVersion{
		PluginID:        plugin.ID,
		Version:         "0.9.0",
		PackageName:     "versions-lifecycle-plugin",
		LifecycleStatus: models.PluginLifecycleUploaded,
		IsActive:        false,
	}
	if err := db.Create(&inactiveVersion).Error; err != nil {
		t.Fatalf("create inactive version failed: %v", err)
	}

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(int(plugin.ID))}}
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/admin/plugins/"+strconv.Itoa(int(plugin.ID))+"/versions", nil)

	handler.GetPluginVersions(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var response []pluginVersionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response failed: %v, body=%s", err, rec.Body.String())
	}
	if len(response) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(response))
	}

	var gotActive, gotInactive *pluginVersionResponse
	for idx := range response {
		if response[idx].IsActive {
			gotActive = &response[idx]
			continue
		}
		if response[idx].Version == inactiveVersion.Version {
			gotInactive = &response[idx]
		}
	}
	if gotActive == nil {
		t.Fatalf("expected an active version in response: %+v", response)
	}
	if gotActive.LifecycleStatus != models.PluginLifecycleRunning {
		t.Fatalf("expected active version lifecycle running, got %s", gotActive.LifecycleStatus)
	}
	if gotInactive == nil {
		t.Fatalf("expected inactive version in response: %+v", response)
	}
	if gotInactive.LifecycleStatus != models.PluginLifecycleUploaded {
		t.Fatalf("expected inactive version lifecycle uploaded, got %s", gotInactive.LifecycleStatus)
	}
}
