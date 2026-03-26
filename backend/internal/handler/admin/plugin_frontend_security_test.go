package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/models"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
)

func TestNormalizeCapabilitiesJSONNormalizesPermissionGovernance(t *testing.T) {
	normalized, err := normalizeCapabilitiesJSON(`{
		"frontend_html_mode": "trusted",
		"requested_permissions": ["runtime.network"],
		"permissions": [
			{"key": "api.execute", "required": true}
		],
		"granted_permissions": ["runtime.network", "api.execute", "frontend.html_trusted", "unknown.permission"]
	}`, "{}")
	if err != nil {
		t.Fatalf("normalizeCapabilitiesJSON returned error: %v", err)
	}

	var capabilities map[string]interface{}
	if err := json.Unmarshal([]byte(normalized), &capabilities); err != nil {
		t.Fatalf("decode normalized capabilities failed: %v", err)
	}

	assertStringSetEqual(t, parseStringListFromAny(capabilities["requested_permissions"]), []string{
		"api.execute",
		"frontend.html_trusted",
		"runtime.network",
	})
	assertStringSetEqual(t, parseStringListFromAny(capabilities["required_permissions"]), []string{
		"api.execute",
	})
	assertStringSetEqual(t, parseStringListFromAny(capabilities["granted_permissions"]), []string{
		"api.execute",
		"frontend.html_trusted",
		"runtime.network",
	})
}

func TestNormalizeCapabilitiesJSONRejectsMissingRequiredGrantWhenExplicitlyDenied(t *testing.T) {
	_, err := normalizeCapabilitiesJSON(`{
		"permissions": [
			{"key": "api.execute", "required": true}
		],
		"granted_permissions": []
	}`, "{}")
	if err == nil {
		t.Fatalf("expected normalizeCapabilitiesJSON to reject missing required grant")
	}
	if !strings.Contains(err.Error(), "required permission api.execute must be granted") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCollectFrontendRouteExecuteActionsDerivesDeclaredActions(t *testing.T) {
	actions := collectFrontendRouteExecuteActions(
		map[string]interface{}{
			"execute_actions": []interface{}{"route.extra"},
		},
		map[string]interface{}{
			"execute_actions": []interface{}{"page.extra"},
			"blocks": []interface{}{
				map[string]interface{}{
					"type": "action_form",
					"data": map[string]interface{}{
						"actions": map[string]interface{}{
							"load":  "demo.load",
							"save":  "demo.save",
							"reset": "demo.reset",
							"extra": []interface{}{
								map[string]interface{}{"action": "demo.extra"},
							},
						},
					},
				},
				map[string]interface{}{
					"type": "html",
					"data": map[string]interface{}{
						"execute_actions": []interface{}{"html.run"},
					},
				},
			},
		},
	)

	assertStringSetEqual(t, actions, []string{
		"demo.extra",
		"demo.load",
		"demo.reset",
		"demo.save",
		"html.run",
		"page.extra",
		"route.extra",
	})
}

func TestCollectFrontendRouteStreamActionsDerivesDeclaredActions(t *testing.T) {
	actions := collectFrontendRouteStreamActions(
		map[string]interface{}{
			"stream_actions": []interface{}{"route.stream"},
		},
		map[string]interface{}{
			"stream_actions": []interface{}{"page.stream"},
			"blocks": []interface{}{
				map[string]interface{}{
					"type": "action_form",
					"data": map[string]interface{}{
						"actions": map[string]interface{}{
							"load":        "demo.load",
							"load_stream": true,
							"save":        "demo.save",
							"save_mode":   "stream",
							"reset":       "demo.reset",
							"extra": []interface{}{
								map[string]interface{}{"action": "demo.extra", "stream": true},
							},
						},
					},
				},
				map[string]interface{}{
					"type": "html",
					"data": map[string]interface{}{
						"stream_actions": []interface{}{"html.stream"},
					},
				},
			},
		},
	)

	assertStringSetEqual(t, actions, []string{
		"demo.extra",
		"demo.load",
		"demo.save",
		"html.stream",
		"page.stream",
		"route.stream",
	})
}

func TestParseFrontendRouteExecuteAPIAvailabilityRequiresExplicitGrant(t *testing.T) {
	if parseFrontendRouteExecuteAPIAvailability("") {
		t.Fatalf("expected empty capabilities to disable route execute api")
	}
	if parseFrontendRouteExecuteAPIAvailability(`{"allow_execute_api": true}`) {
		t.Fatalf("expected missing api.execute grant to disable route execute api")
	}
	if !parseFrontendRouteExecuteAPIAvailability(`{
		"allow_execute_api": true,
		"granted_permissions": ["api.execute"]
	}`) {
		t.Fatalf("expected explicit legacy api.execute grant to enable route execute api")
	}
	if parseFrontendRouteExecuteAPIAvailability(`{
		"allow_execute_api": false,
		"granted_permissions": ["api.execute"]
	}`) {
		t.Fatalf("expected allow_execute_api=false to disable route execute api")
	}
}

func TestBuildFrontendRouteExecuteAPIRequiresDeclaredActions(t *testing.T) {
	if api := buildFrontendRouteExecuteAPI("user", frontendBootstrapRouteItem{PluginID: 1}); api != nil {
		t.Fatalf("expected nil execute api when route declares no actions")
	}

	api := buildFrontendRouteExecuteAPI("user", frontendBootstrapRouteItem{
		PluginID:       1,
		GuestVisible:   true,
		AllowedActions: []string{"demo.save", "demo.load"},
	})
	if api == nil {
		t.Fatalf("expected execute api when actions are declared")
	}
	assertStringSetEqual(t, api.AllowedActions, []string{"demo.load", "demo.save"})
	if api.StreamURL != "" {
		t.Fatalf("expected empty stream url without declared stream actions, got %q", api.StreamURL)
	}
}

func TestBuildFrontendRouteExecuteAPIIncludesStreamMetadata(t *testing.T) {
	api := buildFrontendRouteExecuteAPI("user", frontendBootstrapRouteItem{
		PluginID:             1,
		GuestVisible:         true,
		AllowedActions:       []string{"demo.save", "demo.load"},
		AllowedStreamActions: []string{"demo.load", "unknown.action"},
	})
	if api == nil {
		t.Fatalf("expected execute api when actions are declared")
	}
	if api.StreamURL != "/api/config/plugins/1/execute/stream" {
		t.Fatalf("expected user stream url, got %q", api.StreamURL)
	}
	if api.StreamFormat != "ndjson" {
		t.Fatalf("expected ndjson stream format, got %q", api.StreamFormat)
	}
	assertStringSetEqual(t, api.StreamActions, []string{"demo.load"})
}

func TestResolvePluginPublicCacheVaryKeyIncludesUserAgentAndClientIP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	requestA := httptest.NewRequest("GET", "/api/config/plugin-bootstrap?path=/plugin-pages/demo", nil)
	requestA.Header.Set("Accept-Language", "zh-CN")
	requestA.Header.Set("User-Agent", "ua-a")
	requestA.RemoteAddr = "10.0.0.1:12345"
	ctxA, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctxA.Request = requestA

	requestB := httptest.NewRequest("GET", "/api/config/plugin-bootstrap?path=/plugin-pages/demo", nil)
	requestB.Header.Set("Accept-Language", "zh-CN")
	requestB.Header.Set("User-Agent", "ua-b")
	requestB.RemoteAddr = "10.0.0.2:54321"
	ctxB, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctxB.Request = requestB

	scope := pluginAccessScope{}
	keyA := resolvePluginPublicCacheVaryKey(ctxA, scope, nil, "session-1")
	keyB := resolvePluginPublicCacheVaryKey(ctxB, scope, nil, "session-1")
	if keyA == keyB {
		t.Fatalf("expected cache vary key to include user agent and client ip, got %q vs %q", keyA, keyB)
	}
}

func TestParseFrontendHostContextNormalizesNestedObject(t *testing.T) {
	hostContext, err := parseFrontendHostContext(`{
		" view ": "admin_orders",
		"selection": {
			" selected_count ": 3,
			"": "ignored"
		},
		" flags ": {
			"has_tracking": true
		}
	}`)
	if err != nil {
		t.Fatalf("parseFrontendHostContext returned error: %v", err)
	}
	if got := marshalFrontendHostContextJSON(hostContext); got != `{"flags":{"has_tracking":true},"selection":{"selected_count":3},"view":"admin_orders"}` {
		t.Fatalf("unexpected normalized host context: %s", got)
	}
}

func TestBuildPublicExtensionsCacheKeyIncludesHostContext(t *testing.T) {
	scope := pluginAccessScope{}
	left := buildPublicExtensionsCacheKey(
		"admin.orders.actions",
		"/admin/orders",
		map[string]string{"page": "1"},
		map[string]interface{}{
			"view": "admin_orders",
			"selection": map[string]interface{}{
				"selected_count": 2,
			},
		},
		scope,
		"vary",
	)
	same := buildPublicExtensionsCacheKey(
		"admin.orders.actions",
		"/admin/orders",
		map[string]string{"page": "1"},
		map[string]interface{}{
			"selection": map[string]interface{}{
				"selected_count": 2,
			},
			"view": "admin_orders",
		},
		scope,
		"vary",
	)
	other := buildPublicExtensionsCacheKey(
		"admin.orders.actions",
		"/admin/orders",
		map[string]string{"page": "1"},
		map[string]interface{}{
			"view": "admin_orders",
			"selection": map[string]interface{}{
				"selected_count": 3,
			},
		},
		scope,
		"vary",
	)
	if left != same {
		t.Fatalf("expected cache key to be stable for equivalent host context, got %q vs %q", left, same)
	}
	if left == other {
		t.Fatalf("expected cache key to change when host context changes")
	}
}

func TestFrontendBootstrapRouteMatchSupportsNamedAndWildcardParams(t *testing.T) {
	matched, routeParams := frontendBootstrapRouteMatch(
		"/admin/plugin-pages/logistics/orders/:orderNo",
		"/admin/plugin-pages/logistics/orders/ORD-1001",
	)
	if !matched {
		t.Fatalf("expected named route to match current path")
	}
	assertStringMapEqual(t, routeParams, map[string]string{
		"orderNo": "ORD-1001",
	})

	matched, routeParams = frontendBootstrapRouteMatch(
		"/plugin-pages/demo/*",
		"/plugin-pages/demo/history/detail",
	)
	if !matched {
		t.Fatalf("expected legacy wildcard route to keep matching descendant paths")
	}
	assertStringMapEqual(t, routeParams, map[string]string{})

	matched, routeParams = frontendBootstrapRouteMatch(
		"/plugin-pages/logistics/*rest",
		"/plugin-pages/logistics/orders/detail/1",
	)
	if !matched {
		t.Fatalf("expected named wildcard route to match descendant paths")
	}
	assertStringMapEqual(t, routeParams, map[string]string{
		"rest": "orders/detail/1",
	})

	if matched, _ := frontendBootstrapRouteMatch("/plugin-pages/demo/*", "/plugin-pages/demo"); matched {
		t.Fatalf("expected legacy wildcard route to require a descendant path segment")
	}
}

func TestBuildFrontendPluginExecutionContextIncludesQueryAndRouteMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)

	request := httptest.NewRequest(http.MethodGet, "/api/admin/plugin-bootstrap?path=/admin/plugin-pages/logistics/orders/ORD-1001", nil)
	request.Header.Set("User-Agent", "plugin-test")
	request.Header.Set("Accept-Language", "zh-CN")
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = request

	handler := NewPluginHandler(nil, nil, "")
	execCtx := handler.buildFrontendPluginExecutionContext(
		ctx,
		pluginAccessScope{
			authenticated: true,
			superAdmin:    true,
			permissions: map[string]struct{}{
				"orders.read": {},
			},
		},
		frontendBootstrapAreaAdmin,
		"/admin/plugin-pages/logistics/orders/ORD-1001",
		map[string]string{
			"tab":      "timeline",
			"order_id": "123",
		},
		map[string]string{
			"orderNo": "ORD-1001",
		},
		"session-1",
		nil,
	)

	if execCtx == nil {
		t.Fatalf("expected execution context")
	}
	if execCtx.Metadata["plugin_page_path"] != "/admin/plugin-pages/logistics/orders/ORD-1001" {
		t.Fatalf("unexpected plugin_page_path: %q", execCtx.Metadata["plugin_page_path"])
	}
	if execCtx.Metadata["plugin_page_full_path"] != "/admin/plugin-pages/logistics/orders/ORD-1001?order_id=123&tab=timeline" {
		t.Fatalf("unexpected plugin_page_full_path: %q", execCtx.Metadata["plugin_page_full_path"])
	}
	if execCtx.Metadata["plugin_page_query_string"] != "order_id=123&tab=timeline" {
		t.Fatalf("unexpected plugin_page_query_string: %q", execCtx.Metadata["plugin_page_query_string"])
	}
	if execCtx.Metadata["plugin_page_query_params"] != `{"order_id":"123","tab":"timeline"}` {
		t.Fatalf("unexpected plugin_page_query_params: %q", execCtx.Metadata["plugin_page_query_params"])
	}
	if execCtx.Metadata["plugin_page_route_params"] != `{"orderNo":"ORD-1001"}` {
		t.Fatalf("unexpected plugin_page_route_params: %q", execCtx.Metadata["plugin_page_route_params"])
	}
}

func TestBuildFrontendPluginExecutionContextSetsAuthenticatedOperatorUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adminID := uint(42)
	subjectUserID := uint(7)
	request := httptest.NewRequest(http.MethodGet, "/api/admin/plugin-bootstrap?path=/admin/plugin-pages/demo", nil)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = request
	ctx.Set("user_id", adminID)
	ctx.Set("user_role", "admin")

	handler := NewPluginHandler(nil, nil, "")
	execCtx := handler.buildFrontendPluginExecutionContext(
		ctx,
		pluginAccessScope{authenticated: true},
		frontendBootstrapAreaAdmin,
		"/admin/plugin-pages/demo",
		nil,
		nil,
		"session-1",
		&subjectUserID,
	)
	if execCtx == nil {
		t.Fatalf("expected execution context")
	}
	if execCtx.UserID == nil || *execCtx.UserID != subjectUserID {
		t.Fatalf("expected subject user id %d to remain, got %#v", subjectUserID, execCtx.UserID)
	}
	if execCtx.OperatorUserID == nil || *execCtx.OperatorUserID != adminID {
		t.Fatalf("expected operator user id %d, got %#v", adminID, execCtx.OperatorUserID)
	}

	claims := service.BuildPluginHostAccessClaims(nil, execCtx, time.Minute)
	if claims.OperatorUserID != adminID {
		t.Fatalf("expected operator claims user id %d, got %d", adminID, claims.OperatorUserID)
	}
}

func TestFrontendBootstrapCacheKeysIncludeQueryParams(t *testing.T) {
	scope := pluginAccessScope{}
	varyKey := "session-1"

	extensionsA := buildPublicExtensionsCacheKey("default", "/plugin-pages/demo", map[string]string{
		"tab": "overview",
	}, nil, scope, varyKey)
	extensionsB := buildPublicExtensionsCacheKey("default", "/plugin-pages/demo", map[string]string{
		"tab": "metrics",
	}, nil, scope, varyKey)
	if extensionsA == extensionsB {
		t.Fatalf("expected public extension cache keys to vary by query params")
	}

	bootstrapA := buildPublicBootstrapCacheKey(frontendBootstrapAreaUser, "/plugin-pages/demo", map[string]string{
		"tab": "overview",
		"id":  "42",
	}, scope, varyKey)
	bootstrapB := buildPublicBootstrapCacheKey(frontendBootstrapAreaUser, "/plugin-pages/demo", map[string]string{
		"id":  "42",
		"tab": "overview",
	}, scope, varyKey)
	if bootstrapA != bootstrapB {
		t.Fatalf("expected public bootstrap cache key to stay stable regardless of map iteration order")
	}

	internalA := buildFrontendBootstrapInternalCacheKey(frontendBootstrapAreaUser, "/plugin-pages/demo", map[string]string{
		"id": "42",
	}, scope, varyKey)
	internalB := buildFrontendBootstrapInternalCacheKey(frontendBootstrapAreaUser, "/plugin-pages/demo", map[string]string{
		"id": "43",
	}, scope, varyKey)
	if internalA == internalB {
		t.Fatalf("expected internal bootstrap cache keys to vary by query params")
	}
}

func TestGetAdminExtensionsBatchReturnsPerItemPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := strings.NewReader(`{
		"path": "/admin/orders",
		"items": [
			{
				"key": "row-101",
				"slot": "admin.orders.row_actions",
				"host_context": {
					"order": {
						"id": 101,
						"status": "pending"
					}
				}
			}
		]
	}`)
	request := httptest.NewRequest(http.MethodPost, "/api/admin/plugin-extensions/batch", body)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = request

	handler := NewPluginHandler(nil, nil, "")
	handler.GetAdminExtensionsBatch(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Items []frontendExtensionsBatchItemResponse `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if resp.Code != 0 {
		t.Fatalf("expected success response, got %+v", resp)
	}
	if len(resp.Data.Items) != 1 {
		t.Fatalf("expected one batch item, got %+v", resp.Data.Items)
	}
	item := resp.Data.Items[0]
	if item.Key != "row-101" || item.Path != "/admin/orders" || item.Slot != "admin.orders.row_actions" {
		t.Fatalf("unexpected batch item payload: %+v", item)
	}
	if len(item.Extensions) != 0 {
		t.Fatalf("expected empty extensions without plugin manager, got %+v", item.Extensions)
	}
}

func TestGetAdminExtensionsBatchRejectsOversizedHostContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := strings.NewReader(fmt.Sprintf(`{
		"path": "/admin/orders",
		"items": [
			{
				"slot": "admin.orders.row_actions",
				"host_context": {
					"payload": "%s"
				}
			}
		]
	}`, strings.Repeat("a", maxFrontendHostContextBytes)))
	request := httptest.NewRequest(http.MethodPost, "/api/admin/plugin-extensions/batch", body)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = request

	handler := NewPluginHandler(nil, nil, "")
	handler.GetAdminExtensionsBatch(ctx)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "host_context is too long") {
		t.Fatalf("expected host_context length error, got body=%s", recorder.Body.String())
	}
}

func TestBuildFrontendExtensionsRequestKeyStableAcrossMapOrder(t *testing.T) {
	keyA := buildFrontendExtensionsRequestKey(frontendExtensionsRequest{
		Slot: "admin.orders.row_actions",
		Path: "/admin/orders",
		QueryParams: map[string]string{
			"page": "1",
			"tab":  "ops",
		},
		HostContext: map[string]interface{}{
			"selection": map[string]interface{}{
				"selected_count": 2,
				"selected_ids":   []interface{}{101, 102},
			},
			"view": "admin_orders_row",
		},
	})
	keyB := buildFrontendExtensionsRequestKey(frontendExtensionsRequest{
		Slot: "admin.orders.row_actions",
		Path: "/admin/orders",
		QueryParams: map[string]string{
			"tab":  "ops",
			"page": "1",
		},
		HostContext: map[string]interface{}{
			"view": "admin_orders_row",
			"selection": map[string]interface{}{
				"selected_ids":   []interface{}{101, 102},
				"selected_count": 2,
			},
		},
	})
	if keyA != keyB {
		t.Fatalf("expected stable request key across map iteration order, got %q vs %q", keyA, keyB)
	}
}

func TestResolveFrontendExtensionsBatchWorkersCapsConcurrency(t *testing.T) {
	if got := resolveFrontendExtensionsBatchWorkers(0); got != 0 {
		t.Fatalf("expected 0 workers for empty batch, got %d", got)
	}
	if got := resolveFrontendExtensionsBatchWorkers(3); got != 3 {
		t.Fatalf("expected workers to match batch size, got %d", got)
	}
	if got := resolveFrontendExtensionsBatchWorkers(128); got != maxFrontendExtensionsBatchWorkers {
		t.Fatalf("expected workers to cap at %d, got %d", maxFrontendExtensionsBatchWorkers, got)
	}
}

func TestResolveAccessibleFrontendPluginRouteUsesInternalBootstrapCacheWithoutPluginManager(t *testing.T) {
	gin.SetMode(gin.TestMode)

	request := httptest.NewRequest(http.MethodPost, "/api/config/plugins/42/execute", nil)
	request.Header.Set("Accept-Language", "zh-CN")
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = request

	handler := NewPluginHandler(nil, nil, "")
	scope := pluginAccessScope{}
	sessionID := "session-1"
	cacheVaryKey := resolvePluginPublicCacheVaryKey(ctx, scope, nil, sessionID)
	cachedRoute := frontendBootstrapRouteItem{
		ID:                   "route-42",
		Area:                 frontendBootstrapAreaUser,
		Title:                "Demo Page",
		Path:                 "/plugin-pages/logistics/orders/:orderNo",
		GuestVisible:         true,
		AllowedActions:       []string{"demo.run"},
		AllowedStreamActions: []string{"demo.run"},
		PluginID:             42,
		PluginName:           "demo",
	}
	cachedRoute.ExecuteAPI = buildFrontendRouteExecuteAPI(frontendBootstrapAreaUser, cachedRoute)
	handler.cacheFrontendBootstrap(frontendBootstrapResponseData{
		Area:  frontendBootstrapAreaUser,
		Path:  "/plugin-pages/logistics/orders/ORD-1001",
		Menus: []frontendBootstrapMenuItem{},
		Routes: []frontendBootstrapRouteItem{
			cachedRoute,
		},
	}, nil, scope, cacheVaryKey)

	route, err := handler.resolveAccessibleFrontendPluginRoute(
		ctx,
		frontendBootstrapAreaUser,
		"/plugin-pages/logistics/orders/ORD-1001",
		42,
		nil,
		scope,
		sessionID,
		nil,
	)
	if err != nil {
		t.Fatalf("resolveAccessibleFrontendPluginRoute returned error: %v", err)
	}
	if route == nil {
		t.Fatalf("expected cached route to be resolved")
	}
	if route.ExecuteAPI == nil {
		t.Fatalf("expected cached route execute api to be preserved")
	}
	if route.ExecuteAPI.URL != "/api/config/plugins/42/execute" {
		t.Fatalf("expected cached route execute api url, got %+v", route.ExecuteAPI)
	}
	assertStringMapEqual(t, route.RouteParams, map[string]string{
		"orderNo": "ORD-1001",
	})
}

func TestResolvePluginExecuteAPIAvailabilityUsesExecutionCatalogFirst(t *testing.T) {
	db := openPluginDiagnosticTestDB(t)
	plugins := []models.Plugin{
		{
			Name:            "catalog-allow",
			DisplayName:     "Catalog Allow",
			Type:            "custom",
			Runtime:         service.PluginRuntimeGRPC,
			Address:         "127.0.0.1:9201",
			Enabled:         true,
			LifecycleStatus: models.PluginLifecycleRunning,
			Capabilities: mustMarshalFrontendCapabilityMap(t, map[string]interface{}{
				"allow_execute_api":     true,
				"requested_permissions": []string{service.PluginPermissionExecuteAPI},
				"granted_permissions":   []string{service.PluginPermissionExecuteAPI},
			}),
		},
		{
			Name:            "catalog-deny",
			DisplayName:     "Catalog Deny",
			Type:            "custom",
			Runtime:         service.PluginRuntimeGRPC,
			Address:         "127.0.0.1:9202",
			Enabled:         true,
			LifecycleStatus: models.PluginLifecycleRunning,
			Capabilities: mustMarshalFrontendCapabilityMap(t, map[string]interface{}{
				"allow_execute_api":     true,
				"requested_permissions": []string{service.PluginPermissionExecuteAPI},
				"granted_permissions":   []string{},
			}),
		},
	}
	for i := range plugins {
		if err := db.Create(&plugins[i]).Error; err != nil {
			t.Fatalf("create plugin %s failed: %v", plugins[i].Name, err)
		}
	}

	pluginManager := service.NewPluginManagerService(db, &config.Config{
		Plugin: config.PluginPlatformConfig{
			Enabled:         true,
			AllowedRuntimes: []string{service.PluginRuntimeGRPC, service.PluginRuntimeJSWorker},
			DefaultRuntime:  service.PluginRuntimeGRPC,
		},
	})
	if err := pluginManager.RefreshPluginExecutionCatalog(); err != nil {
		t.Fatalf("RefreshPluginExecutionCatalog returned error: %v", err)
	}

	handler := NewPluginHandler(nil, pluginManager, "")
	availability := handler.resolvePluginExecuteAPIAvailability([]uint{
		plugins[0].ID,
		plugins[1].ID,
		9999,
	}, nil)
	if !availability[plugins[0].ID] {
		t.Fatalf("expected execute api availability to be resolved from catalog for plugin %d", plugins[0].ID)
	}
	if availability[plugins[1].ID] {
		t.Fatalf("expected denied execute api availability to stay false for plugin %d", plugins[1].ID)
	}
	if availability[9999] {
		t.Fatalf("expected unknown plugin availability to stay false")
	}
}

func TestResolvePluginFrontendHTMLModesUsesExecutionCatalogFirst(t *testing.T) {
	db := openPluginDiagnosticTestDB(t)
	plugins := []models.Plugin{
		{
			Name:            "catalog-trusted",
			DisplayName:     "Catalog Trusted",
			Type:            "custom",
			Runtime:         service.PluginRuntimeGRPC,
			Address:         "127.0.0.1:9301",
			Enabled:         true,
			LifecycleStatus: models.PluginLifecycleRunning,
			Capabilities: mustMarshalFrontendCapabilityMap(t, map[string]interface{}{
				"frontend_html_mode":    "trusted",
				"requested_permissions": []string{service.PluginPermissionFrontendHTMLTrust},
				"granted_permissions":   []string{service.PluginPermissionFrontendHTMLTrust},
			}),
		},
		{
			Name:            "catalog-sanitized",
			DisplayName:     "Catalog Sanitized",
			Type:            "custom",
			Runtime:         service.PluginRuntimeGRPC,
			Address:         "127.0.0.1:9302",
			Enabled:         true,
			LifecycleStatus: models.PluginLifecycleRunning,
			Capabilities: mustMarshalFrontendCapabilityMap(t, map[string]interface{}{
				"frontend_html_mode":    "trusted",
				"requested_permissions": []string{service.PluginPermissionFrontendHTMLTrust},
				"granted_permissions":   []string{},
			}),
		},
	}
	for i := range plugins {
		if err := db.Create(&plugins[i]).Error; err != nil {
			t.Fatalf("create plugin %s failed: %v", plugins[i].Name, err)
		}
	}

	pluginManager := service.NewPluginManagerService(db, &config.Config{
		Plugin: config.PluginPlatformConfig{
			Enabled:         true,
			AllowedRuntimes: []string{service.PluginRuntimeGRPC, service.PluginRuntimeJSWorker},
			DefaultRuntime:  service.PluginRuntimeGRPC,
		},
	})
	if err := pluginManager.RefreshPluginExecutionCatalog(); err != nil {
		t.Fatalf("RefreshPluginExecutionCatalog returned error: %v", err)
	}

	handler := NewPluginHandler(nil, pluginManager, "")
	modes := handler.resolvePluginFrontendHTMLModes([]uint{
		plugins[0].ID,
		plugins[1].ID,
		9999,
	}, nil)
	if modes[plugins[0].ID] != pluginFrontendHTMLModeTrusted {
		t.Fatalf("expected trusted html mode from catalog, got %q", modes[plugins[0].ID])
	}
	if modes[plugins[1].ID] != pluginFrontendHTMLModeSanitize {
		t.Fatalf("expected sanitized html mode without trust grant, got %q", modes[plugins[1].ID])
	}
	if modes[9999] != pluginFrontendHTMLModeSanitize {
		t.Fatalf("expected unknown plugin html mode to stay sanitize, got %q", modes[9999])
	}
}

func TestFrontendPluginCapabilityResolveCacheCachesHTMLModes(t *testing.T) {
	cache := newFrontendPluginCapabilityResolveCache()
	calls := 0

	first := cache.resolveHTMLModes([]uint{1, 2, 1}, func(ids []uint) map[uint]string {
		calls++
		if len(ids) != 2 {
			t.Fatalf("expected de-duplicated ids, got %v", ids)
		}
		return map[uint]string{
			1: pluginFrontendHTMLModeTrusted,
		}
	})
	second := cache.resolveHTMLModes([]uint{1, 2}, func(ids []uint) map[uint]string {
		calls++
		return map[uint]string{}
	})

	if calls != 1 {
		t.Fatalf("expected html mode resolver to run once, got %d", calls)
	}
	if first[1] != pluginFrontendHTMLModeTrusted || second[1] != pluginFrontendHTMLModeTrusted {
		t.Fatalf("expected trusted html mode to be cached, got first=%q second=%q", first[1], second[1])
	}
	if first[2] != pluginFrontendHTMLModeSanitize || second[2] != pluginFrontendHTMLModeSanitize {
		t.Fatalf("expected missing html mode to be cached as sanitize, got first=%q second=%q", first[2], second[2])
	}
}

func TestFrontendPluginCapabilityResolveCacheResolveHTMLModesDoesNotBlockDisjointIDs(t *testing.T) {
	cache := newFrontendPluginCapabilityResolveCache()
	releaseFirst := make(chan struct{})
	firstStarted := make(chan struct{})
	firstDone := make(chan map[uint]string, 1)

	go func() {
		firstDone <- cache.resolveHTMLModes([]uint{1}, func(ids []uint) map[uint]string {
			close(firstStarted)
			<-releaseFirst
			return map[uint]string{
				1: pluginFrontendHTMLModeTrusted,
			}
		})
	}()

	<-firstStarted

	secondDone := make(chan map[uint]string, 1)
	go func() {
		secondDone <- cache.resolveHTMLModes([]uint{2}, func(ids []uint) map[uint]string {
			return map[uint]string{
				2: pluginFrontendHTMLModeTrusted,
			}
		})
	}()

	select {
	case result := <-secondDone:
		if result[2] != pluginFrontendHTMLModeTrusted {
			t.Fatalf("expected disjoint html mode to resolve without blocking, got %q", result[2])
		}
	case <-time.After(time.Second):
		t.Fatalf("expected disjoint html mode resolution to avoid unrelated inflight wait")
	}

	close(releaseFirst)
	select {
	case result := <-firstDone:
		if result[1] != pluginFrontendHTMLModeTrusted {
			t.Fatalf("expected first html mode result after release, got %q", result[1])
		}
	case <-time.After(time.Second):
		t.Fatalf("expected first html mode resolution to complete after release")
	}
}

func TestFrontendPluginCapabilityResolveCacheCachesExecuteAPIAvailability(t *testing.T) {
	cache := newFrontendPluginCapabilityResolveCache()
	calls := 0

	first := cache.resolveExecuteAPIAvailability([]uint{10, 20, 10}, func(ids []uint) map[uint]bool {
		calls++
		if len(ids) != 2 {
			t.Fatalf("expected de-duplicated ids, got %v", ids)
		}
		return map[uint]bool{
			10: true,
		}
	})
	second := cache.resolveExecuteAPIAvailability([]uint{10, 20}, func(ids []uint) map[uint]bool {
		calls++
		return map[uint]bool{}
	})

	if calls != 1 {
		t.Fatalf("expected execute api resolver to run once, got %d", calls)
	}
	if !first[10] || !second[10] {
		t.Fatalf("expected execute api true result to be cached")
	}
	if first[20] || second[20] {
		t.Fatalf("expected missing execute api availability to be cached as false")
	}
}

func TestFrontendPluginCapabilityResolveCacheResolveExecuteAPIAvailabilityDeduplicatesConcurrentIDs(t *testing.T) {
	cache := newFrontendPluginCapabilityResolveCache()
	releaseResolver := make(chan struct{})
	resolverStarted := make(chan struct{})
	resolverCalls := make(chan struct{}, 2)
	done := make(chan map[uint]bool, 2)

	resolver := func(ids []uint) map[uint]bool {
		resolverCalls <- struct{}{}
		select {
		case <-resolverStarted:
		default:
			close(resolverStarted)
		}
		<-releaseResolver
		return map[uint]bool{
			10: true,
		}
	}

	go func() {
		done <- cache.resolveExecuteAPIAvailability([]uint{10}, resolver)
	}()
	<-resolverStarted
	go func() {
		done <- cache.resolveExecuteAPIAvailability([]uint{10}, resolver)
	}()

	close(releaseResolver)
	for i := 0; i < 2; i++ {
		select {
		case result := <-done:
			if !result[10] {
				t.Fatalf("expected execute api availability true, got %+v", result)
			}
		case <-time.After(time.Second):
			t.Fatalf("expected concurrent execute api resolutions to complete")
		}
	}
	if len(resolverCalls) != 1 {
		t.Fatalf("expected concurrent execute api resolution to use single inflight resolver, got %d calls", len(resolverCalls))
	}
}

func assertStringSetEqual(t *testing.T, actual []string, expected []string) {
	t.Helper()

	normalizedActual := normalizeLowerStringListValues(actual)
	normalizedExpected := normalizeLowerStringListValues(expected)
	sort.Strings(normalizedActual)
	sort.Strings(normalizedExpected)
	if len(normalizedActual) != len(normalizedExpected) {
		t.Fatalf("expected %v, got %v", normalizedExpected, normalizedActual)
	}
	for idx := range normalizedActual {
		if normalizedActual[idx] != normalizedExpected[idx] {
			t.Fatalf("expected %v, got %v", normalizedExpected, normalizedActual)
		}
	}
}

func assertStringMapEqual(t *testing.T, actual map[string]string, expected map[string]string) {
	t.Helper()

	if len(actual) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, actual)
	}
	for key, value := range expected {
		if actual[key] != value {
			t.Fatalf("expected %v, got %v", expected, actual)
		}
	}
}

func mustMarshalFrontendCapabilityMap(t *testing.T, payload map[string]interface{}) string {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal capabilities failed: %v", err)
	}
	return string(body)
}
