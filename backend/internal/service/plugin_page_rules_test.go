package service

import (
	"strings"
	"testing"

	"auralogic/internal/config"
	"auralogic/internal/models"
)

func TestExecutePluginHostActionManagesPluginOwnedPageRules(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	plugin := &models.Plugin{
		Name:            "aff-plugin",
		DisplayName:     "AFF Plugin",
		Enabled:         true,
		LifecycleStatus: models.PluginLifecycleInstalled,
	}
	if err := db.Create(plugin).Error; err != nil {
		t.Fatalf("create plugin failed: %v", err)
	}

	runtime := NewPluginHostRuntime(db, nil, nil)
	claims := &PluginHostAccessClaims{
		PluginID:           plugin.ID,
		GrantedPermissions: []string{PluginPermissionHostPluginPageRuleRead, PluginPermissionHostPluginPageRuleWrite},
		ScopeAuthenticated: true,
		ScopeSuperAdmin:    true,
	}

	saveResult, err := ExecutePluginHostActionWithRuntime(runtime, claims, "host.plugin_page_rule.upsert", map[string]interface{}{
		"key":        "landing-tracker",
		"name":       "Landing Tracker",
		"pattern":    "^/$",
		"match_type": "regex",
		"css":        "body { color: red; }",
		"js":         "window.__AFF__ = true;",
		"enabled":    true,
		"priority":   5,
	})
	if err != nil {
		t.Fatalf("plugin_page_rule.upsert returned error: %v", err)
	}
	if got := saveResult["saved"]; got != true {
		t.Fatalf("expected saved=true, got %#v", got)
	}
	if got := saveResult["created"]; got != true {
		t.Fatalf("expected created=true, got %#v", got)
	}
	rule, ok := saveResult["rule"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected rule object, got %#v", saveResult["rule"])
	}
	if got := rule["namespace"]; got != "aff-plugin" {
		t.Fatalf("expected namespace aff-plugin, got %#v", got)
	}
	if got := rule["key"]; got != "landing-tracker" {
		t.Fatalf("expected key landing-tracker, got %#v", got)
	}

	listResult, err := ExecutePluginHostActionWithRuntime(runtime, claims, "host.plugin_page_rule.list", map[string]interface{}{})
	if err != nil {
		t.Fatalf("plugin_page_rule.list returned error: %v", err)
	}
	if got := listResult["count"]; got != 1 {
		t.Fatalf("expected count=1, got %#v", got)
	}

	getResult, err := ExecutePluginHostActionWithRuntime(runtime, claims, "host.plugin_page_rule.get", map[string]interface{}{
		"key": "landing-tracker",
	})
	if err != nil {
		t.Fatalf("plugin_page_rule.get returned error: %v", err)
	}
	gotRule, ok := getResult["rule"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected get rule object, got %#v", getResult["rule"])
	}
	if got := interfaceToTestInt64(gotRule["priority"]); got != 5 {
		t.Fatalf("expected priority=5, got %#v", gotRule["priority"])
	}

	deleteResult, err := ExecutePluginHostActionWithRuntime(runtime, claims, "host.plugin_page_rule.delete", map[string]interface{}{
		"key": "landing-tracker",
	})
	if err != nil {
		t.Fatalf("plugin_page_rule.delete returned error: %v", err)
	}
	if got := deleteResult["deleted"]; got != true {
		t.Fatalf("expected deleted=true, got %#v", got)
	}

	_, err = ExecutePluginHostActionWithRuntime(runtime, claims, "host.plugin_page_rule.upsert", map[string]interface{}{
		"key":     "checkout-aff",
		"pattern": "^/checkout",
		"css":     ".checkout { display: block; }",
	})
	if err != nil {
		t.Fatalf("recreate first page rule failed: %v", err)
	}
	_, err = ExecutePluginHostActionWithRuntime(runtime, claims, "host.plugin_page_rule.upsert", map[string]interface{}{
		"key":     "product-aff",
		"pattern": "^/products",
		"css":     ".product { display: block; }",
	})
	if err != nil {
		t.Fatalf("create second page rule failed: %v", err)
	}

	resetResult, err := ExecutePluginHostActionWithRuntime(runtime, claims, "host.plugin_page_rule.reset", map[string]interface{}{})
	if err != nil {
		t.Fatalf("plugin_page_rule.reset returned error: %v", err)
	}
	if got := resetResult["reset"]; got != true {
		t.Fatalf("expected reset=true, got %#v", got)
	}
	if got := interfaceToTestInt64(resetResult["deleted_count"]); got != 2 {
		t.Fatalf("expected deleted_count=2, got %#v", resetResult["deleted_count"])
	}
}

func TestResolvePageInjectPayloadMergesHostAndActivePluginRules(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)

	allowedPlugin := &models.Plugin{
		Name:            "aff-plugin",
		Enabled:         true,
		LifecycleStatus: models.PluginLifecycleInstalled,
		Capabilities: `{
			"requested_permissions": ["host.plugin_page_rule.write"],
			"granted_permissions": ["host.plugin_page_rule.write"]
		}`,
	}
	if err := db.Create(allowedPlugin).Error; err != nil {
		t.Fatalf("create allowed plugin failed: %v", err)
	}
	deniedPlugin := &models.Plugin{
		Name:            "denied-plugin",
		Enabled:         true,
		LifecycleStatus: models.PluginLifecycleInstalled,
		Capabilities:    `{}`,
	}
	if err := db.Create(deniedPlugin).Error; err != nil {
		t.Fatalf("create denied plugin failed: %v", err)
	}
	pausedPlugin := &models.Plugin{
		Name:            "paused-plugin",
		Enabled:         true,
		LifecycleStatus: models.PluginLifecyclePaused,
		Capabilities: `{
			"requested_permissions": ["host.plugin_page_rule.write"],
			"granted_permissions": ["host.plugin_page_rule.write"]
		}`,
	}
	if err := db.Create(pausedPlugin).Error; err != nil {
		t.Fatalf("create paused plugin failed: %v", err)
	}

	entries := []models.PluginPageRuleEntry{
		{
			PluginID:  allowedPlugin.ID,
			Key:       "landing-aff",
			Name:      "Landing AFF",
			Pattern:   "/checkout",
			MatchType: "exact",
			CSS:       "body { background: #111; }",
			JS:        "window.__AFF_ALLOWED__ = true;",
			Enabled:   true,
			Priority:  10,
		},
		{
			PluginID:  deniedPlugin.ID,
			Key:       "landing-denied",
			Name:      "Denied",
			Pattern:   "/checkout",
			MatchType: "exact",
			CSS:       "body { background: #222; }",
			JS:        "window.__AFF_DENIED__ = true;",
			Enabled:   true,
			Priority:  1,
		},
		{
			PluginID:  pausedPlugin.ID,
			Key:       "landing-paused",
			Name:      "Paused",
			Pattern:   "/checkout",
			MatchType: "exact",
			CSS:       "body { background: #333; }",
			JS:        "window.__AFF_PAUSED__ = true;",
			Enabled:   true,
			Priority:  1,
		},
	}
	if err := db.Create(&entries).Error; err != nil {
		t.Fatalf("create plugin page rules failed: %v", err)
	}

	cfg := &config.Config{}
	cfg.Customization.PageRules = []config.PageRule{
		{
			Name:      "Host Checkout",
			Pattern:   "/checkout",
			MatchType: "exact",
			CSS:       "html { color: blue; }",
			JS:        "window.__HOST_RULE__ = true;",
			Enabled:   true,
		},
	}

	payload, err := ResolvePageInjectPayload(db, cfg, "/checkout")
	if err != nil {
		t.Fatalf("ResolvePageInjectPayload returned error: %v", err)
	}
	if payload.MatchedCount != 2 {
		t.Fatalf("expected 2 matched rules, got %+v", payload)
	}
	if len(payload.Rules) != 2 {
		t.Fatalf("expected 2 rule blocks, got %+v", payload.Rules)
	}
	if payload.Rules[0].Source != "host" {
		t.Fatalf("expected host rule first, got %+v", payload.Rules[0])
	}
	if payload.Rules[1].Source != "plugin" || payload.Rules[1].Namespace != "aff-plugin" {
		t.Fatalf("expected allowed plugin rule second, got %+v", payload.Rules[1])
	}
	if payload.Rules[1].Key != "landing-aff" {
		t.Fatalf("expected landing-aff key, got %+v", payload.Rules[1])
	}
	if payload.CSS == "" || payload.JS == "" {
		t.Fatalf("expected aggregated css/js content, got %+v", payload)
	}
	if contains := strings.Contains(payload.JS, "__AFF_DENIED__"); contains {
		t.Fatalf("expected denied plugin rule to be filtered, got %+v", payload)
	}
	if contains := strings.Contains(payload.JS, "__AFF_PAUSED__"); contains {
		t.Fatalf("expected paused plugin rule to be filtered, got %+v", payload)
	}
}
