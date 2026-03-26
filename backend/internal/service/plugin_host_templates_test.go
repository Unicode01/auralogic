package service

import (
	"os"
	"path/filepath"
	"testing"

	"auralogic/internal/database"
	"auralogic/internal/models"
)

func TestExecutePluginHostActionGetsAndSavesEmailTemplateWithDigest(t *testing.T) {
	rootDir := t.TempDir()
	restoreWorkingDir := chdirPluginHostTemplateTest(t, rootDir)
	defer restoreWorkingDir()

	templateDir := filepath.Join(rootDir, "templates", "email")
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		t.Fatalf("mkdir email template dir failed: %v", err)
	}
	initialContent := "Hello {{.Name}}"
	if err := os.WriteFile(filepath.Join(templateDir, "order_paid.html"), []byte(initialContent), 0o644); err != nil {
		t.Fatalf("write email template failed: %v", err)
	}

	db := openPluginManagerE2ETestDB(t)
	getResult, err := ExecutePluginHostAction(db, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostEmailTemplateRead},
		ScopeAuthenticated: true,
		ScopeSuperAdmin:    true,
	}, "host.email_template.get", map[string]interface{}{
		"key": "order_paid",
	})
	if err != nil {
		t.Fatalf("email_template.get returned error: %v", err)
	}
	if got := getResult["filename"]; got != "order_paid.html" {
		t.Fatalf("expected filename order_paid.html, got %#v", got)
	}
	expectedDigest, ok := getResult["digest"].(string)
	if !ok || expectedDigest == "" {
		t.Fatalf("expected template digest, got %#v", getResult["digest"])
	}

	saveResult, err := ExecutePluginHostAction(db, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostEmailTemplateWrite},
		ScopeAuthenticated: true,
		ScopeSuperAdmin:    true,
	}, "host.email_template.save", map[string]interface{}{
		"key":             "order_paid",
		"content":         "Updated {{.Name}}",
		"expected_digest": expectedDigest,
	})
	if err != nil {
		t.Fatalf("email_template.save returned error: %v", err)
	}
	if got := saveResult["saved"]; got != true {
		t.Fatalf("expected saved=true, got %#v", got)
	}

	savedBytes, err := os.ReadFile(filepath.Join(templateDir, "order_paid.html"))
	if err != nil {
		t.Fatalf("read saved email template failed: %v", err)
	}
	if string(savedBytes) != "Updated {{.Name}}" {
		t.Fatalf("expected updated email template content, got %q", string(savedBytes))
	}
}

func TestExecutePluginHostActionRejectsEmailTemplateSaveOnDigestConflict(t *testing.T) {
	rootDir := t.TempDir()
	restoreWorkingDir := chdirPluginHostTemplateTest(t, rootDir)
	defer restoreWorkingDir()

	templateDir := filepath.Join(rootDir, "templates", "email")
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		t.Fatalf("mkdir email template dir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(templateDir, "order_paid.html"), []byte("Hello {{.Name}}"), 0o644); err != nil {
		t.Fatalf("write email template failed: %v", err)
	}

	db := openPluginManagerE2ETestDB(t)
	_, err := ExecutePluginHostAction(db, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostEmailTemplateWrite},
		ScopeAuthenticated: true,
		ScopeSuperAdmin:    true,
	}, "host.email_template.save", map[string]interface{}{
		"key":             "order_paid",
		"content":         "Updated {{.Name}}",
		"expected_digest": "mismatch",
	})
	if err == nil {
		t.Fatal("expected digest conflict error")
	}
}

func TestExecutePluginHostActionGetsAndSavesLandingPage(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.LandingPage{}); err != nil {
		t.Fatalf("auto migrate landing page failed: %v", err)
	}

	page := models.LandingPage{
		Slug:        "home",
		HTMLContent: "<html>{{.AppName}}</html>",
		IsActive:    true,
	}
	if err := db.Create(&page).Error; err != nil {
		t.Fatalf("create landing page failed: %v", err)
	}

	getResult, err := ExecutePluginHostAction(db, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostLandingPageRead},
		ScopeAuthenticated: true,
		ScopeSuperAdmin:    true,
	}, "host.landing_page.get", map[string]interface{}{})
	if err != nil {
		t.Fatalf("landing_page.get returned error: %v", err)
	}
	expectedDigest, ok := getResult["digest"].(string)
	if !ok || expectedDigest == "" {
		t.Fatalf("expected landing page digest, got %#v", getResult["digest"])
	}

	saveResult, err := ExecutePluginHostAction(db, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostLandingPageWrite},
		ScopeAuthenticated: true,
		ScopeSuperAdmin:    true,
	}, "host.landing_page.save", map[string]interface{}{
		"html_content":    "<html>{{.PrimaryColor}}</html>",
		"expected_digest": expectedDigest,
	})
	if err != nil {
		t.Fatalf("landing_page.save returned error: %v", err)
	}
	if got := saveResult["saved"]; got != true {
		t.Fatalf("expected saved=true, got %#v", got)
	}

	var refreshed models.LandingPage
	if err := db.Where("slug = ?", "home").First(&refreshed).Error; err != nil {
		t.Fatalf("reload landing page failed: %v", err)
	}
	if refreshed.HTMLContent != "<html>{{.PrimaryColor}}</html>" {
		t.Fatalf("expected updated landing page content, got %q", refreshed.HTMLContent)
	}
}

func TestExecutePluginHostActionResetsLandingPage(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.LandingPage{}); err != nil {
		t.Fatalf("auto migrate landing page failed: %v", err)
	}

	page := models.LandingPage{
		Slug:        "home",
		HTMLContent: "<html>custom</html>",
		IsActive:    true,
	}
	if err := db.Create(&page).Error; err != nil {
		t.Fatalf("create landing page failed: %v", err)
	}

	previousDefault := database.GetDefaultLandingPageHTML()
	database.SetDefaultLandingPageHTML("<html>default</html>")
	defer database.SetDefaultLandingPageHTML(previousDefault)

	result, err := ExecutePluginHostAction(db, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostLandingPageWrite},
		ScopeAuthenticated: true,
		ScopeSuperAdmin:    true,
	}, "host.landing_page.reset", map[string]interface{}{})
	if err != nil {
		t.Fatalf("landing_page.reset returned error: %v", err)
	}
	if got := result["reset"]; got != true {
		t.Fatalf("expected reset=true, got %#v", got)
	}

	var refreshed models.LandingPage
	if err := db.Where("slug = ?", "home").First(&refreshed).Error; err != nil {
		t.Fatalf("reload landing page failed: %v", err)
	}
	if refreshed.HTMLContent != "<html>default</html>" {
		t.Fatalf("expected default landing page content, got %q", refreshed.HTMLContent)
	}
}

func TestExecutePluginHostActionGetsSavesAndResetsInvoiceTemplate(t *testing.T) {
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

	db := openPluginManagerE2ETestDB(t)
	runtime := NewPluginHostRuntime(db, runtimeConfig, nil)

	getResult, err := ExecutePluginHostActionWithRuntime(runtime, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostInvoiceTemplateRead},
		ScopeAuthenticated: true,
		ScopeSuperAdmin:    true,
	}, "host.invoice_template.get", map[string]interface{}{})
	if err != nil {
		t.Fatalf("invoice_template.get returned error: %v", err)
	}
	if got := getResult["target_key"]; got != pluginHostInvoiceTemplateTargetKey {
		t.Fatalf("expected target_key %q, got %#v", pluginHostInvoiceTemplateTargetKey, got)
	}
	if got := getResult["template_type"]; got != "builtin" {
		t.Fatalf("expected template_type builtin, got %#v", got)
	}

	saveResult, err := ExecutePluginHostActionWithRuntime(runtime, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostInvoiceTemplateWrite},
		ScopeAuthenticated: true,
		ScopeSuperAdmin:    true,
	}, "host.invoice_template.save", map[string]interface{}{
		"content": "<div>{{.OrderNo}}</div>",
	})
	if err != nil {
		t.Fatalf("invoice_template.save returned error: %v", err)
	}
	if got := saveResult["saved"]; got != true {
		t.Fatalf("expected saved=true, got %#v", got)
	}
	if got := saveResult["template_type"]; got != "custom" {
		t.Fatalf("expected template_type custom, got %#v", got)
	}
	if runtimeConfig.Order.Invoice.TemplateType != "custom" || runtimeConfig.Order.Invoice.CustomTemplate != "<div>{{.OrderNo}}</div>" {
		t.Fatalf("expected runtime config to be updated, got %+v", runtimeConfig.Order.Invoice)
	}

	savedDocument := readPluginHostConfigDocument(t, configPath)
	orderDocument := savedDocument["order"].(map[string]interface{})
	invoiceDocument := orderDocument["invoice"].(map[string]interface{})
	if got := invoiceDocument["template_type"]; got != "custom" {
		t.Fatalf("expected persisted template_type custom, got %#v", got)
	}
	if got := invoiceDocument["custom_template"]; got != "<div>{{.OrderNo}}</div>" {
		t.Fatalf("expected persisted custom template, got %#v", got)
	}

	resetResult, err := ExecutePluginHostActionWithRuntime(runtime, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostInvoiceTemplateWrite},
		ScopeAuthenticated: true,
		ScopeSuperAdmin:    true,
	}, "host.invoice_template.reset", map[string]interface{}{})
	if err != nil {
		t.Fatalf("invoice_template.reset returned error: %v", err)
	}
	if got := resetResult["reset"]; got != true {
		t.Fatalf("expected reset=true, got %#v", got)
	}
	if got := resetResult["template_type"]; got != "builtin" {
		t.Fatalf("expected reset template_type builtin, got %#v", got)
	}
	if runtimeConfig.Order.Invoice.TemplateType != "builtin" || runtimeConfig.Order.Invoice.CustomTemplate != "" {
		t.Fatalf("expected runtime config to be reset, got %+v", runtimeConfig.Order.Invoice)
	}
}

func TestExecutePluginHostActionGetsSavesAndResetsAuthBrandingTemplate(t *testing.T) {
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

	db := openPluginManagerE2ETestDB(t)
	runtime := NewPluginHostRuntime(db, runtimeConfig, nil)

	getResult, err := ExecutePluginHostActionWithRuntime(runtime, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostAuthBrandingRead},
		ScopeAuthenticated: true,
		ScopeSuperAdmin:    true,
	}, "host.auth_branding.get", map[string]interface{}{})
	if err != nil {
		t.Fatalf("auth_branding.get returned error: %v", err)
	}
	if got := getResult["target_key"]; got != pluginHostAuthBrandingTemplateTargetKey {
		t.Fatalf("expected target_key %q, got %#v", pluginHostAuthBrandingTemplateTargetKey, got)
	}
	if got := getResult["mode"]; got != "default" {
		t.Fatalf("expected mode default, got %#v", got)
	}

	saveResult, err := ExecutePluginHostActionWithRuntime(runtime, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostAuthBrandingWrite},
		ScopeAuthenticated: true,
		ScopeSuperAdmin:    true,
	}, "host.auth_branding.save", map[string]interface{}{
		"content": "<section>{{.AppName}}</section>",
	})
	if err != nil {
		t.Fatalf("auth_branding.save returned error: %v", err)
	}
	if got := saveResult["saved"]; got != true {
		t.Fatalf("expected saved=true, got %#v", got)
	}
	if got := saveResult["mode"]; got != "custom" {
		t.Fatalf("expected mode custom, got %#v", got)
	}
	if runtimeConfig.Customization.AuthBranding.Mode != "custom" || runtimeConfig.Customization.AuthBranding.CustomHTML != "<section>{{.AppName}}</section>" {
		t.Fatalf("expected runtime auth branding to be updated, got %+v", runtimeConfig.Customization.AuthBranding)
	}

	savedDocument := readPluginHostConfigDocument(t, configPath)
	customizationDocument := savedDocument["customization"].(map[string]interface{})
	authBrandingDocument := customizationDocument["auth_branding"].(map[string]interface{})
	if got := authBrandingDocument["mode"]; got != "custom" {
		t.Fatalf("expected persisted mode custom, got %#v", got)
	}
	if got := authBrandingDocument["custom_html"]; got != "<section>{{.AppName}}</section>" {
		t.Fatalf("expected persisted custom html, got %#v", got)
	}

	resetResult, err := ExecutePluginHostActionWithRuntime(runtime, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostAuthBrandingWrite},
		ScopeAuthenticated: true,
		ScopeSuperAdmin:    true,
	}, "host.auth_branding.reset", map[string]interface{}{})
	if err != nil {
		t.Fatalf("auth_branding.reset returned error: %v", err)
	}
	if got := resetResult["reset"]; got != true {
		t.Fatalf("expected reset=true, got %#v", got)
	}
	if got := resetResult["mode"]; got != "default" {
		t.Fatalf("expected reset mode default, got %#v", got)
	}
	if runtimeConfig.Customization.AuthBranding.Mode != "default" || runtimeConfig.Customization.AuthBranding.CustomHTML != "" {
		t.Fatalf("expected runtime auth branding to be reset, got %+v", runtimeConfig.Customization.AuthBranding)
	}
}

func TestExecutePluginHostActionGetsSavesAndResetsPageRulePack(t *testing.T) {
	runtimeConfig, configPath, restoreConfigPath := preparePluginHostConfigFile(t, map[string]interface{}{
		"customization": map[string]interface{}{
			"page_rules": []map[string]interface{}{
				{
					"name":       "Initial rule",
					"pattern":    "/products",
					"match_type": "exact",
					"css":        ".page { color: red; }",
					"js":         "",
					"enabled":    true,
				},
			},
		},
	})
	defer restoreConfigPath()

	db := openPluginManagerE2ETestDB(t)
	runtime := NewPluginHostRuntime(db, runtimeConfig, nil)

	getResult, err := ExecutePluginHostActionWithRuntime(runtime, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostPageRulePackRead},
		ScopeAuthenticated: true,
		ScopeSuperAdmin:    true,
	}, "host.page_rule_pack.get", map[string]interface{}{})
	if err != nil {
		t.Fatalf("page_rule_pack.get returned error: %v", err)
	}
	if got := getResult["target_key"]; got != pluginHostPageRulePackTargetKey {
		t.Fatalf("expected target_key %q, got %#v", pluginHostPageRulePackTargetKey, got)
	}
	if got := getResult["count"]; got != 1 {
		t.Fatalf("expected count 1, got %#v", got)
	}

	saveResult, err := ExecutePluginHostActionWithRuntime(runtime, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostPageRulePackWrite},
		ScopeAuthenticated: true,
		ScopeSuperAdmin:    true,
	}, "host.page_rule_pack.save", map[string]interface{}{
		"content": `[{"name":"Checkout tracker","pattern":"^/checkout","match_type":"regex","css":"body{background:#000;}","js":"console.log('checkout');","enabled":true}]`,
	})
	if err != nil {
		t.Fatalf("page_rule_pack.save returned error: %v", err)
	}
	if got := saveResult["saved"]; got != true {
		t.Fatalf("expected saved=true, got %#v", got)
	}
	if len(runtimeConfig.Customization.PageRules) != 1 {
		t.Fatalf("expected runtime config page rules to be updated, got %+v", runtimeConfig.Customization.PageRules)
	}
	if runtimeConfig.Customization.PageRules[0].Pattern != "^/checkout" || runtimeConfig.Customization.PageRules[0].MatchType != "regex" {
		t.Fatalf("expected runtime page rule to be updated, got %+v", runtimeConfig.Customization.PageRules[0])
	}

	savedDocument := readPluginHostConfigDocument(t, configPath)
	customizationDocument := savedDocument["customization"].(map[string]interface{})
	pageRulesDocument := customizationDocument["page_rules"].([]interface{})
	if len(pageRulesDocument) != 1 {
		t.Fatalf("expected persisted page rule pack content, got %#v", customizationDocument["page_rules"])
	}

	resetResult, err := ExecutePluginHostActionWithRuntime(runtime, &PluginHostAccessClaims{
		GrantedPermissions: []string{PluginPermissionHostPageRulePackWrite},
		ScopeAuthenticated: true,
		ScopeSuperAdmin:    true,
	}, "host.page_rule_pack.reset", map[string]interface{}{})
	if err != nil {
		t.Fatalf("page_rule_pack.reset returned error: %v", err)
	}
	if got := resetResult["reset"]; got != true {
		t.Fatalf("expected reset=true, got %#v", got)
	}
	if len(runtimeConfig.Customization.PageRules) != 0 {
		t.Fatalf("expected runtime config page rules to be reset, got %+v", runtimeConfig.Customization.PageRules)
	}
}

func chdirPluginHostTemplateTest(t *testing.T, targetDir string) func() {
	t.Helper()
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	if err := os.Chdir(targetDir); err != nil {
		t.Fatalf("chdir to %s failed: %v", targetDir, err)
	}
	return func() {
		if chdirErr := os.Chdir(workingDir); chdirErr != nil {
			t.Fatalf("restore working dir failed: %v", chdirErr)
		}
	}
}
