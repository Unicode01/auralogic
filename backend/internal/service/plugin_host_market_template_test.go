package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"auralogic/internal/models"
	"gorm.io/gorm"
)

func TestExecutePluginHostActionInstallsAndRollsBackEmailTemplateMarketArtifact(t *testing.T) {
	rootDir := t.TempDir()
	restoreWorkingDir := chdirPluginHostTemplateTest(t, rootDir)
	defer restoreWorkingDir()

	templateDir := filepath.Join(rootDir, "templates", "email")
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		t.Fatalf("mkdir email template dir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(templateDir, "order_paid.html"), []byte("Initial {{.OrderNo}}"), 0o644); err != nil {
		t.Fatalf("write initial email template failed: %v", err)
	}

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/artifacts/email_template/order_paid/releases/1.0.0":
			_ = json.NewEncoder(w).Encode(buildTemplateMarketReleaseEnvelope(
				"email_template",
				"order_paid",
				"1.0.0",
				"order_paid",
				"Email v1 {{.OrderNo}}",
			))
		case "/v1/artifacts/email_template/order_paid/releases/1.1.0":
			_ = json.NewEncoder(w).Encode(buildTemplateMarketReleaseEnvelope(
				"email_template",
				"order_paid",
				"1.1.0",
				"order_paid",
				"Email v2 {{.OrderNo}}",
			))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.TemplateVersion{}, &models.OperationLog{}); err != nil {
		t.Fatalf("auto migrate template market tables failed: %v", err)
	}

	runtime, executeClaims, readClaims, rollbackClaims := createTemplateMarketTestRuntime(t, db, server.URL, []string{"email_template"})

	if _, err := ExecutePluginHostActionWithRuntime(
		runtime,
		executeClaims,
		"host.market.install.execute",
		map[string]interface{}{
			"source_id": "official",
			"kind":      "email_template",
			"name":      "order_paid",
			"version":   "1.0.0",
			"email_key": "order_paid",
		},
	); err != nil {
		t.Fatalf("install email template 1.0.0 returned error: %v", err)
	}
	if _, err := ExecutePluginHostActionWithRuntime(
		runtime,
		executeClaims,
		"host.market.install.execute",
		map[string]interface{}{
			"source_id": "official",
			"kind":      "email_template",
			"name":      "order_paid",
			"version":   "1.1.0",
			"email_key": "order_paid",
		},
	); err != nil {
		t.Fatalf("install email template 1.1.0 returned error: %v", err)
	}

	emailBytes, err := os.ReadFile(filepath.Join(templateDir, "order_paid.html"))
	if err != nil {
		t.Fatalf("read current email template failed: %v", err)
	}
	if string(emailBytes) != "Email v2 {{.OrderNo}}" {
		t.Fatalf("expected latest email content, got %q", string(emailBytes))
	}

	historyResult, err := ExecutePluginHostActionWithRuntime(
		runtime,
		readClaims,
		"host.market.install.history.list",
		map[string]interface{}{
			"source_id":  "official",
			"kind":       "email_template",
			"name":       "order_paid",
			"email_key":  "order_paid",
			"target_key": "order_paid",
			"limit":      10,
		},
	)
	if err != nil {
		t.Fatalf("email history.list returned error: %v", err)
	}
	if typedItems, ok := historyResult["items"].([]map[string]interface{}); ok {
		if len(typedItems) != 2 {
			t.Fatalf("expected two email template history items, got %#v", typedItems)
		}
		if typedItems[0]["version"] != "1.1.0" || typedItems[0]["installed_target_key"] != "order_paid" {
			t.Fatalf("expected latest email template history item, got %#v", typedItems[0])
		}
	} else {
		historyItems, ok := historyResult["items"].([]interface{})
		if !ok || len(historyItems) != 2 {
			t.Fatalf("expected two email template history items, got %#v", historyResult["items"])
		}
		firstHistory, ok := historyItems[0].(map[string]interface{})
		if !ok || firstHistory["version"] != "1.1.0" || firstHistory["installed_target_key"] != "order_paid" {
			t.Fatalf("expected latest email template history item, got %#v", historyItems[0])
		}
	}

	rollbackResult, err := ExecutePluginHostActionWithRuntime(
		runtime,
		rollbackClaims,
		"host.market.install.rollback",
		map[string]interface{}{
			"source_id":  "official",
			"kind":       "email_template",
			"name":       "order_paid",
			"version":    "1.0.0",
			"email_key":  "order_paid",
			"target_key": "order_paid",
		},
	)
	if err != nil {
		t.Fatalf("email rollback returned error: %v", err)
	}
	if got := rollbackResult["status"]; got != "rolled_back" {
		t.Fatalf("expected email rollback status rolled_back, got %#v", got)
	}

	emailBytes, err = os.ReadFile(filepath.Join(templateDir, "order_paid.html"))
	if err != nil {
		t.Fatalf("read rolled back email template failed: %v", err)
	}
	if string(emailBytes) != "Email v1 {{.OrderNo}}" {
		t.Fatalf("expected rolled back email content, got %q", string(emailBytes))
	}

	var activeVersion models.TemplateVersion
	if err := db.Where("resource_kind = ? AND target_key = ? AND is_active = ?", "email_template", "order_paid", true).First(&activeVersion).Error; err != nil {
		t.Fatalf("load active email template version failed: %v", err)
	}
	if activeVersion.MarketArtifactVersion != "1.0.0" {
		t.Fatalf("expected active email template version 1.0.0 after rollback, got %+v", activeVersion)
	}

	assertTemplateMarketOperationLogs(
		t,
		db,
		"email_template",
		"order_paid",
		[]string{"plugin_market_install", "plugin_market_install", "plugin_market_rollback"},
		[]string{"1.0.0", "1.1.0", "1.0.0"},
	)
}

func TestExecutePluginHostActionInstallsAndRollsBackLandingTemplateMarketArtifact(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/artifacts/landing_page_template/home/releases/1.0.0":
			_ = json.NewEncoder(w).Encode(buildTemplateMarketReleaseEnvelope(
				"landing_page_template",
				"home",
				"1.0.0",
				"home",
				"<html>Landing v1</html>",
			))
		case "/v1/artifacts/landing_page_template/home/releases/1.1.0":
			_ = json.NewEncoder(w).Encode(buildTemplateMarketReleaseEnvelope(
				"landing_page_template",
				"home",
				"1.1.0",
				"home",
				"<html>Landing v2</html>",
			))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.TemplateVersion{}, &models.LandingPage{}, &models.OperationLog{}); err != nil {
		t.Fatalf("auto migrate landing template tables failed: %v", err)
	}
	if err := db.Create(&models.LandingPage{
		Slug:        "home",
		HTMLContent: "<html>Initial</html>",
		IsActive:    true,
	}).Error; err != nil {
		t.Fatalf("create initial landing page failed: %v", err)
	}

	runtime, executeClaims, readClaims, rollbackClaims := createTemplateMarketTestRuntime(t, db, server.URL, []string{"landing_page_template"})

	if _, err := ExecutePluginHostActionWithRuntime(
		runtime,
		executeClaims,
		"host.market.install.execute",
		map[string]interface{}{
			"source_id":    "official",
			"kind":         "landing_page_template",
			"name":         "home",
			"version":      "1.0.0",
			"landing_slug": "home",
		},
	); err != nil {
		t.Fatalf("install landing template 1.0.0 returned error: %v", err)
	}
	if _, err := ExecutePluginHostActionWithRuntime(
		runtime,
		executeClaims,
		"host.market.install.execute",
		map[string]interface{}{
			"source_id":    "official",
			"kind":         "landing_page_template",
			"name":         "home",
			"version":      "1.1.0",
			"landing_slug": "home",
		},
	); err != nil {
		t.Fatalf("install landing template 1.1.0 returned error: %v", err)
	}

	var currentPage models.LandingPage
	if err := db.Where("slug = ?", "home").First(&currentPage).Error; err != nil {
		t.Fatalf("load current landing page failed: %v", err)
	}
	if currentPage.HTMLContent != "<html>Landing v2</html>" {
		t.Fatalf("expected latest landing content, got %q", currentPage.HTMLContent)
	}

	historyResult, err := ExecutePluginHostActionWithRuntime(
		runtime,
		readClaims,
		"host.market.install.history.list",
		map[string]interface{}{
			"source_id":    "official",
			"kind":         "landing_page_template",
			"name":         "home",
			"landing_slug": "home",
			"target_key":   "home",
			"limit":        10,
		},
	)
	if err != nil {
		t.Fatalf("landing history.list returned error: %v", err)
	}
	if typedItems, ok := historyResult["items"].([]map[string]interface{}); ok {
		if len(typedItems) != 2 {
			t.Fatalf("expected two landing template history items, got %#v", typedItems)
		}
		if typedItems[0]["version"] != "1.1.0" || typedItems[0]["installed_target_key"] != "home" {
			t.Fatalf("expected latest landing template history item, got %#v", typedItems[0])
		}
	} else {
		historyItems, ok := historyResult["items"].([]interface{})
		if !ok || len(historyItems) != 2 {
			t.Fatalf("expected two landing template history items, got %#v", historyResult["items"])
		}
		firstHistory, ok := historyItems[0].(map[string]interface{})
		if !ok || firstHistory["version"] != "1.1.0" || firstHistory["installed_target_key"] != "home" {
			t.Fatalf("expected latest landing template history item, got %#v", historyItems[0])
		}
	}

	rollbackResult, err := ExecutePluginHostActionWithRuntime(
		runtime,
		rollbackClaims,
		"host.market.install.rollback",
		map[string]interface{}{
			"source_id":    "official",
			"kind":         "landing_page_template",
			"name":         "home",
			"version":      "1.0.0",
			"landing_slug": "home",
			"target_key":   "home",
		},
	)
	if err != nil {
		t.Fatalf("landing rollback returned error: %v", err)
	}
	if got := rollbackResult["status"]; got != "rolled_back" {
		t.Fatalf("expected landing rollback status rolled_back, got %#v", got)
	}

	if err := db.Where("slug = ?", "home").First(&currentPage).Error; err != nil {
		t.Fatalf("reload rolled back landing page failed: %v", err)
	}
	if currentPage.HTMLContent != "<html>Landing v1</html>" {
		t.Fatalf("expected rolled back landing content, got %q", currentPage.HTMLContent)
	}

	var activeVersion models.TemplateVersion
	if err := db.Where("resource_kind = ? AND target_key = ? AND is_active = ?", "landing_page_template", "home", true).First(&activeVersion).Error; err != nil {
		t.Fatalf("load active landing template version failed: %v", err)
	}
	if activeVersion.MarketArtifactVersion != "1.0.0" {
		t.Fatalf("expected active landing template version 1.0.0 after rollback, got %+v", activeVersion)
	}

	assertTemplateMarketOperationLogs(
		t,
		db,
		"landing_page",
		"home",
		[]string{"plugin_market_install", "plugin_market_install", "plugin_market_rollback"},
		[]string{"1.0.0", "1.1.0", "1.0.0"},
	)
}

func TestExecutePluginHostActionInstallsAndRollsBackInvoiceTemplateMarketArtifact(t *testing.T) {
	runtimeConfig, configPath, restoreConfigPath := preparePluginHostConfigFile(t, map[string]interface{}{
		"order": map[string]interface{}{
			"invoice": map[string]interface{}{
				"enabled":         true,
				"template_type":   "builtin",
				"custom_template": "",
			},
		},
	})
	defer restoreConfigPath()

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/artifacts/invoice_template/invoice/releases/1.0.0":
			_ = json.NewEncoder(w).Encode(buildTemplateMarketReleaseEnvelope(
				"invoice_template",
				"invoice",
				"1.0.0",
				pluginHostInvoiceTemplateTargetKey,
				"<html>Invoice v1 {{.OrderNo}}</html>",
			))
		case "/v1/artifacts/invoice_template/invoice/releases/1.1.0":
			_ = json.NewEncoder(w).Encode(buildTemplateMarketReleaseEnvelope(
				"invoice_template",
				"invoice",
				"1.1.0",
				pluginHostInvoiceTemplateTargetKey,
				"<html>Invoice v2 {{.OrderNo}}</html>",
			))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.TemplateVersion{}, &models.OperationLog{}); err != nil {
		t.Fatalf("auto migrate invoice template tables failed: %v", err)
	}

	runtime, executeClaims, readClaims, rollbackClaims := createTemplateMarketTestRuntime(t, db, server.URL, []string{"invoice_template"})
	runtime.Config = runtimeConfig

	if _, err := ExecutePluginHostActionWithRuntime(
		runtime,
		executeClaims,
		"host.market.install.execute",
		map[string]interface{}{
			"source_id": "official",
			"kind":      "invoice_template",
			"name":      "invoice",
			"version":   "1.0.0",
		},
	); err != nil {
		t.Fatalf("install invoice template 1.0.0 returned error: %v", err)
	}
	if _, err := ExecutePluginHostActionWithRuntime(
		runtime,
		executeClaims,
		"host.market.install.execute",
		map[string]interface{}{
			"source_id": "official",
			"kind":      "invoice_template",
			"name":      "invoice",
			"version":   "1.1.0",
		},
	); err != nil {
		t.Fatalf("install invoice template 1.1.0 returned error: %v", err)
	}

	currentConfig := readPluginHostConfigDocument(t, configPath)
	orderDocument := currentConfig["order"].(map[string]interface{})
	invoiceDocument := orderDocument["invoice"].(map[string]interface{})
	if got := invoiceDocument["template_type"]; got != "custom" {
		t.Fatalf("expected invoice template_type custom, got %#v", got)
	}
	if got := invoiceDocument["custom_template"]; got != "<html>Invoice v2 {{.OrderNo}}</html>" {
		t.Fatalf("expected latest invoice template content, got %#v", got)
	}

	historyResult, err := ExecutePluginHostActionWithRuntime(
		runtime,
		readClaims,
		"host.market.install.history.list",
		map[string]interface{}{
			"source_id":  "official",
			"kind":       "invoice_template",
			"name":       "invoice",
			"target_key": pluginHostInvoiceTemplateTargetKey,
			"limit":      10,
		},
	)
	if err != nil {
		t.Fatalf("invoice history.list returned error: %v", err)
	}
	if typedItems, ok := historyResult["items"].([]map[string]interface{}); ok {
		if len(typedItems) != 2 {
			t.Fatalf("expected two invoice template history items, got %#v", typedItems)
		}
	} else {
		historyItems, ok := historyResult["items"].([]interface{})
		if !ok || len(historyItems) != 2 {
			t.Fatalf("expected two invoice template history items, got %#v", historyResult["items"])
		}
	}

	rollbackResult, err := ExecutePluginHostActionWithRuntime(
		runtime,
		rollbackClaims,
		"host.market.install.rollback",
		map[string]interface{}{
			"source_id":  "official",
			"kind":       "invoice_template",
			"name":       "invoice",
			"version":    "1.0.0",
			"target_key": pluginHostInvoiceTemplateTargetKey,
		},
	)
	if err != nil {
		t.Fatalf("invoice rollback returned error: %v", err)
	}
	if got := rollbackResult["status"]; got != "rolled_back" {
		t.Fatalf("expected invoice rollback status rolled_back, got %#v", got)
	}

	currentConfig = readPluginHostConfigDocument(t, configPath)
	orderDocument = currentConfig["order"].(map[string]interface{})
	invoiceDocument = orderDocument["invoice"].(map[string]interface{})
	if got := invoiceDocument["custom_template"]; got != "<html>Invoice v1 {{.OrderNo}}</html>" {
		t.Fatalf("expected rolled back invoice template content, got %#v", got)
	}

	var activeVersion models.TemplateVersion
	if err := db.Where("resource_kind = ? AND target_key = ? AND is_active = ?", "invoice_template", pluginHostInvoiceTemplateTargetKey, true).First(&activeVersion).Error; err != nil {
		t.Fatalf("load active invoice template version failed: %v", err)
	}
	if activeVersion.MarketArtifactVersion != "1.0.0" {
		t.Fatalf("expected active invoice template version 1.0.0 after rollback, got %+v", activeVersion)
	}

	assertTemplateMarketOperationLogs(
		t,
		db,
		"invoice_template",
		pluginHostInvoiceTemplateTargetKey,
		[]string{"plugin_market_install", "plugin_market_install", "plugin_market_rollback"},
		[]string{"1.0.0", "1.1.0", "1.0.0"},
	)
}

func TestExecutePluginHostActionInstallsAndRollsBackAuthBrandingTemplateMarketArtifact(t *testing.T) {
	runtimeConfig, configPath, restoreConfigPath := preparePluginHostConfigFile(t, map[string]interface{}{
		"customization": map[string]interface{}{
			"auth_branding": map[string]interface{}{
				"mode":        "default",
				"title":       "AuraLogic",
				"title_en":    "AuraLogic",
				"subtitle":    "Default",
				"subtitle_en": "Default",
				"custom_html": "",
			},
		},
	})
	defer restoreConfigPath()

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/artifacts/auth_branding_template/auth_branding/releases/1.0.0":
			_ = json.NewEncoder(w).Encode(buildTemplateMarketReleaseEnvelope(
				"auth_branding_template",
				"auth_branding",
				"1.0.0",
				pluginHostAuthBrandingTemplateTargetKey,
				"<section>Branding v1 {{.AppName}}</section>",
			))
		case "/v1/artifacts/auth_branding_template/auth_branding/releases/1.1.0":
			_ = json.NewEncoder(w).Encode(buildTemplateMarketReleaseEnvelope(
				"auth_branding_template",
				"auth_branding",
				"1.1.0",
				pluginHostAuthBrandingTemplateTargetKey,
				"<section>Branding v2 {{.AppName}}</section>",
			))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.TemplateVersion{}, &models.OperationLog{}); err != nil {
		t.Fatalf("auto migrate auth branding template tables failed: %v", err)
	}

	runtime, executeClaims, readClaims, rollbackClaims := createTemplateMarketTestRuntime(t, db, server.URL, []string{"auth_branding_template"})
	runtime.Config = runtimeConfig

	if _, err := ExecutePluginHostActionWithRuntime(
		runtime,
		executeClaims,
		"host.market.install.execute",
		map[string]interface{}{
			"source_id": "official",
			"kind":      "auth_branding_template",
			"name":      "auth_branding",
			"version":   "1.0.0",
		},
	); err != nil {
		t.Fatalf("install auth branding template 1.0.0 returned error: %v", err)
	}
	if _, err := ExecutePluginHostActionWithRuntime(
		runtime,
		executeClaims,
		"host.market.install.execute",
		map[string]interface{}{
			"source_id": "official",
			"kind":      "auth_branding_template",
			"name":      "auth_branding",
			"version":   "1.1.0",
		},
	); err != nil {
		t.Fatalf("install auth branding template 1.1.0 returned error: %v", err)
	}

	currentConfig := readPluginHostConfigDocument(t, configPath)
	customizationDocument := currentConfig["customization"].(map[string]interface{})
	authBrandingDocument := customizationDocument["auth_branding"].(map[string]interface{})
	if got := authBrandingDocument["mode"]; got != "custom" {
		t.Fatalf("expected auth branding mode custom, got %#v", got)
	}
	if got := authBrandingDocument["custom_html"]; got != "<section>Branding v2 {{.AppName}}</section>" {
		t.Fatalf("expected latest auth branding template content, got %#v", got)
	}

	historyResult, err := ExecutePluginHostActionWithRuntime(
		runtime,
		readClaims,
		"host.market.install.history.list",
		map[string]interface{}{
			"source_id":  "official",
			"kind":       "auth_branding_template",
			"name":       "auth_branding",
			"target_key": pluginHostAuthBrandingTemplateTargetKey,
			"limit":      10,
		},
	)
	if err != nil {
		t.Fatalf("auth branding history.list returned error: %v", err)
	}
	if typedItems, ok := historyResult["items"].([]map[string]interface{}); ok {
		if len(typedItems) != 2 {
			t.Fatalf("expected two auth branding history items, got %#v", typedItems)
		}
	} else {
		historyItems, ok := historyResult["items"].([]interface{})
		if !ok || len(historyItems) != 2 {
			t.Fatalf("expected two auth branding history items, got %#v", historyResult["items"])
		}
	}

	rollbackResult, err := ExecutePluginHostActionWithRuntime(
		runtime,
		rollbackClaims,
		"host.market.install.rollback",
		map[string]interface{}{
			"source_id":  "official",
			"kind":       "auth_branding_template",
			"name":       "auth_branding",
			"version":    "1.0.0",
			"target_key": pluginHostAuthBrandingTemplateTargetKey,
		},
	)
	if err != nil {
		t.Fatalf("auth branding rollback returned error: %v", err)
	}
	if got := rollbackResult["status"]; got != "rolled_back" {
		t.Fatalf("expected auth branding rollback status rolled_back, got %#v", got)
	}

	currentConfig = readPluginHostConfigDocument(t, configPath)
	customizationDocument = currentConfig["customization"].(map[string]interface{})
	authBrandingDocument = customizationDocument["auth_branding"].(map[string]interface{})
	if got := authBrandingDocument["custom_html"]; got != "<section>Branding v1 {{.AppName}}</section>" {
		t.Fatalf("expected rolled back auth branding template content, got %#v", got)
	}

	var activeVersion models.TemplateVersion
	if err := db.Where("resource_kind = ? AND target_key = ? AND is_active = ?", "auth_branding_template", pluginHostAuthBrandingTemplateTargetKey, true).First(&activeVersion).Error; err != nil {
		t.Fatalf("load active auth branding template version failed: %v", err)
	}
	if activeVersion.MarketArtifactVersion != "1.0.0" {
		t.Fatalf("expected active auth branding template version 1.0.0 after rollback, got %+v", activeVersion)
	}

	assertTemplateMarketOperationLogs(
		t,
		db,
		"auth_branding",
		pluginHostAuthBrandingTemplateTargetKey,
		[]string{"plugin_market_install", "plugin_market_install", "plugin_market_rollback"},
		[]string{"1.0.0", "1.1.0", "1.0.0"},
	)
}

func TestExecutePluginHostActionInstallsAndRollsBackPageRulePackMarketArtifact(t *testing.T) {
	runtimeConfig, configPath, restoreConfigPath := preparePluginHostConfigFile(t, map[string]interface{}{
		"customization": map[string]interface{}{
			"page_rules": []map[string]interface{}{
				{
					"name":       "Initial",
					"pattern":    "/",
					"match_type": "exact",
					"css":        "body{color:#111;}",
					"js":         "",
					"enabled":    true,
				},
			},
		},
	})
	defer restoreConfigPath()

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/artifacts/page_rule_pack/page_rules/releases/1.0.0":
			_ = json.NewEncoder(w).Encode(buildTemplateMarketReleaseEnvelope(
				"page_rule_pack",
				"page_rules",
				"1.0.0",
				pluginHostPageRulePackTargetKey,
				`[{"name":"Rules v1","pattern":"^/products","match_type":"regex","css":"body{color:red;}","js":"","enabled":true}]`,
			))
		case "/v1/artifacts/page_rule_pack/page_rules/releases/1.1.0":
			_ = json.NewEncoder(w).Encode(buildTemplateMarketReleaseEnvelope(
				"page_rule_pack",
				"page_rules",
				"1.1.0",
				pluginHostPageRulePackTargetKey,
				`[{"name":"Rules v2","pattern":"^/checkout","match_type":"regex","css":"body{color:blue;}","js":"console.log('checkout')","enabled":true}]`,
			))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.TemplateVersion{}, &models.OperationLog{}); err != nil {
		t.Fatalf("auto migrate page rule pack tables failed: %v", err)
	}

	runtime, executeClaims, readClaims, rollbackClaims := createTemplateMarketTestRuntime(t, db, server.URL, []string{"page_rule_pack"})
	runtime.Config = runtimeConfig

	if _, err := ExecutePluginHostActionWithRuntime(
		runtime,
		executeClaims,
		"host.market.install.execute",
		map[string]interface{}{
			"source_id": "official",
			"kind":      "page_rule_pack",
			"name":      "page_rules",
			"version":   "1.0.0",
		},
	); err != nil {
		t.Fatalf("install page rule pack 1.0.0 returned error: %v", err)
	}
	if _, err := ExecutePluginHostActionWithRuntime(
		runtime,
		executeClaims,
		"host.market.install.execute",
		map[string]interface{}{
			"source_id": "official",
			"kind":      "page_rule_pack",
			"name":      "page_rules",
			"version":   "1.1.0",
		},
	); err != nil {
		t.Fatalf("install page rule pack 1.1.0 returned error: %v", err)
	}

	currentConfig := readPluginHostConfigDocument(t, configPath)
	customizationDocument := currentConfig["customization"].(map[string]interface{})
	pageRulesDocument := customizationDocument["page_rules"].([]interface{})
	if len(pageRulesDocument) != 1 {
		t.Fatalf("expected latest page rule pack content, got %#v", customizationDocument["page_rules"])
	}

	historyResult, err := ExecutePluginHostActionWithRuntime(
		runtime,
		readClaims,
		"host.market.install.history.list",
		map[string]interface{}{
			"source_id":  "official",
			"kind":       "page_rule_pack",
			"name":       "page_rules",
			"target_key": pluginHostPageRulePackTargetKey,
			"limit":      10,
		},
	)
	if err != nil {
		t.Fatalf("page rule pack history.list returned error: %v", err)
	}
	if typedItems, ok := historyResult["items"].([]map[string]interface{}); ok {
		if len(typedItems) != 2 {
			t.Fatalf("expected two page rule pack history items, got %#v", typedItems)
		}
	} else {
		historyItems, ok := historyResult["items"].([]interface{})
		if !ok || len(historyItems) != 2 {
			t.Fatalf("expected two page rule pack history items, got %#v", historyResult["items"])
		}
	}

	rollbackResult, err := ExecutePluginHostActionWithRuntime(
		runtime,
		rollbackClaims,
		"host.market.install.rollback",
		map[string]interface{}{
			"source_id":  "official",
			"kind":       "page_rule_pack",
			"name":       "page_rules",
			"version":    "1.0.0",
			"target_key": pluginHostPageRulePackTargetKey,
		},
	)
	if err != nil {
		t.Fatalf("page rule pack rollback returned error: %v", err)
	}
	if got := rollbackResult["status"]; got != "rolled_back" {
		t.Fatalf("expected page rule pack rollback status rolled_back, got %#v", got)
	}

	currentConfig = readPluginHostConfigDocument(t, configPath)
	customizationDocument = currentConfig["customization"].(map[string]interface{})
	pageRulesDocument = customizationDocument["page_rules"].([]interface{})
	if len(pageRulesDocument) != 1 {
		t.Fatalf("expected rolled back page rule pack content, got %#v", customizationDocument["page_rules"])
	}

	var activeVersion models.TemplateVersion
	if err := db.Where("resource_kind = ? AND target_key = ? AND is_active = ?", "page_rule_pack", pluginHostPageRulePackTargetKey, true).First(&activeVersion).Error; err != nil {
		t.Fatalf("load active page rule pack version failed: %v", err)
	}
	if activeVersion.MarketArtifactVersion != "1.0.0" {
		t.Fatalf("expected active page rule pack version 1.0.0 after rollback, got %+v", activeVersion)
	}

	assertTemplateMarketOperationLogs(
		t,
		db,
		"page_rule_pack",
		pluginHostPageRulePackTargetKey,
		[]string{"plugin_market_install", "plugin_market_install", "plugin_market_rollback"},
		[]string{"1.0.0", "1.1.0", "1.0.0"},
	)
}

func createTemplateMarketTestRuntime(
	t *testing.T,
	db *gorm.DB,
	serverURL string,
	allowedKinds []string,
) (*PluginHostRuntime, *PluginHostAccessClaims, *PluginHostAccessClaims, *PluginHostAccessClaims) {
	t.Helper()

	marketPlugin := models.Plugin{
		Name:    "market-plugin",
		Runtime: PluginRuntimeJSWorker,
		Address: "index.js",
		Config: `{
			"sources": [
				{
					"source_id": "official",
					"source_base_url": "` + serverURL + `",
					"allowed_kinds": ` + mustTemplateMarketJSON(t, allowedKinds) + `
				}
			]
		}`,
	}
	if err := db.Create(&marketPlugin).Error; err != nil {
		t.Fatalf("create market plugin failed: %v", err)
	}

	runtime := NewPluginHostRuntime(db, nil, nil)
	executeClaims := &PluginHostAccessClaims{
		PluginID:       marketPlugin.ID,
		OperatorUserID: 9001,
		GrantedPermissions: []string{
			PluginPermissionHostMarketInstallExecute,
		},
		ScopeAuthenticated: true,
		ScopeSuperAdmin:    true,
	}
	readClaims := &PluginHostAccessClaims{
		PluginID:       marketPlugin.ID,
		OperatorUserID: 9001,
		GrantedPermissions: []string{
			PluginPermissionHostMarketInstallRead,
		},
		ScopeAuthenticated: true,
		ScopeSuperAdmin:    true,
	}
	rollbackClaims := &PluginHostAccessClaims{
		PluginID:       marketPlugin.ID,
		OperatorUserID: 9001,
		GrantedPermissions: []string{
			PluginPermissionHostMarketInstallRollback,
		},
		ScopeAuthenticated: true,
		ScopeSuperAdmin:    true,
	}
	return runtime, executeClaims, readClaims, rollbackClaims
}

func buildTemplateMarketReleaseEnvelope(kind string, name string, version string, targetKey string, content string) map[string]interface{} {
	targets := map[string]interface{}{}
	templatePayload := map[string]interface{}{}
	switch kind {
	case "email_template":
		targets["key"] = targetKey
		targets["event"] = targetKey
		templatePayload["key"] = targetKey
		templatePayload["filename"] = targetKey + ".html"
		templatePayload["content"] = content
	case "landing_page_template":
		targets["slug"] = targetKey
		targets["page_key"] = targetKey
		templatePayload["slug"] = targetKey
		templatePayload["page_key"] = targetKey
		templatePayload["content"] = content
	case "invoice_template", "auth_branding_template":
		targets["key"] = targetKey
		templatePayload["key"] = targetKey
		templatePayload["content"] = content
	case "page_rule_pack":
		targets["key"] = targetKey
		templatePayload["key"] = targetKey
		templatePayload["content"] = content
	}
	envelope := map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"kind":    kind,
			"name":    name,
			"version": version,
			"governance": map[string]interface{}{
				"mode": "host_managed",
			},
			"compatibility": map[string]interface{}{
				"min_host_bridge_version": "1.0.0",
			},
			"install": map[string]interface{}{
				"inline_content": true,
			},
			"targets":  targets,
			"template": templatePayload,
		},
	}
	if kind == "page_rule_pack" {
		envelopeData := envelope["data"].(map[string]interface{})
		envelopeData["page_rules"] = map[string]interface{}{
			"key":     targetKey,
			"content": content,
		}
	}
	return envelope
}

func mustTemplateMarketJSON(t *testing.T, value interface{}) string {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal template market test config failed: %v", err)
	}
	return string(body)
}

func assertTemplateMarketOperationLogs(
	t *testing.T,
	db *gorm.DB,
	resourceType string,
	targetKey string,
	expectedActions []string,
	expectedVersions []string,
) {
	t.Helper()

	var logs []models.OperationLog
	if err := db.Order("id ASC").Find(&logs).Error; err != nil {
		t.Fatalf("query template market operation logs failed: %v", err)
	}
	if len(logs) != len(expectedActions) {
		t.Fatalf("expected %d operation logs, got %#v", len(expectedActions), logs)
	}

	for idx := range logs {
		entry := logs[idx]
		if entry.Action != expectedActions[idx] {
			t.Fatalf("expected log %d action %q, got %+v", idx, expectedActions[idx], entry)
		}
		if entry.ResourceType != resourceType {
			t.Fatalf("expected log %d resource_type %q, got %+v", idx, resourceType, entry)
		}
		if entry.UserID == nil || *entry.UserID != 9001 {
			t.Fatalf("expected log %d user_id 9001, got %+v", idx, entry)
		}
		if entry.OperatorName != "plugin:market-plugin" {
			t.Fatalf("expected log %d operator_name plugin:market-plugin, got %+v", idx, entry)
		}
		if got := pluginMarketStringFromAny(entry.Details["origin"]); got != "plugin_host_bridge" {
			t.Fatalf("expected log %d origin plugin_host_bridge, got %#v", idx, entry.Details)
		}
		if got := pluginMarketStringFromAny(entry.Details["plugin_name"]); got != "market-plugin" {
			t.Fatalf("expected log %d plugin_name market-plugin, got %#v", idx, entry.Details)
		}
		if got := pluginMarketStringFromAny(entry.Details["source_id"]); got != "official" {
			t.Fatalf("expected log %d source_id official, got %#v", idx, entry.Details)
		}
		if got := pluginMarketStringFromAny(entry.Details["target_key"]); got != targetKey {
			t.Fatalf("expected log %d target_key %q, got %#v", idx, targetKey, entry.Details)
		}
		if got := pluginHostMarketNormalizeVersion(pluginMarketStringFromAny(entry.Details["version"])); got != expectedVersions[idx] {
			t.Fatalf("expected log %d version %q, got %#v", idx, expectedVersions[idx], entry.Details)
		}
	}
}
