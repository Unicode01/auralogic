package service

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"auralogic/internal/config"
	"auralogic/internal/models"
)

func TestEmailServiceRenderTemplateHotReloadsOnFileChange(t *testing.T) {
	rootDir := t.TempDir()
	restoreWorkingDir := chdirPluginHostTemplateTest(t, rootDir)
	defer restoreWorkingDir()

	templateDir := filepath.Join(rootDir, "templates", "email")
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		t.Fatalf("mkdir email template dir failed: %v", err)
	}
	templatePath := filepath.Join(templateDir, "order_paid.html")
	if err := os.WriteFile(templatePath, []byte("Initial {{.OrderNo}}"), 0o644); err != nil {
		t.Fatalf("write email template failed: %v", err)
	}

	emailService := NewEmailService(nil, &config.SMTPConfig{}, "https://example.com")
	rendered, err := emailService.renderTemplate("order_paid", "en", map[string]interface{}{
		"OrderNo": "A-001",
	})
	if err != nil {
		t.Fatalf("render initial template failed: %v", err)
	}
	if rendered != "Initial A-001" {
		t.Fatalf("expected initial render, got %q", rendered)
	}

	if err := os.WriteFile(templatePath, []byte("Updated {{.OrderNo}}"), 0o644); err != nil {
		t.Fatalf("update email template failed: %v", err)
	}

	rendered, err = emailService.renderTemplate("order_paid", "en", map[string]interface{}{
		"OrderNo": "A-002",
	})
	if err != nil {
		t.Fatalf("render hot reloaded template failed: %v", err)
	}
	if rendered != "Updated A-002" {
		t.Fatalf("expected hot reloaded render, got %q", rendered)
	}
}

func TestImportTemplatePackageImportsEmailTemplateAndCreatesSnapshot(t *testing.T) {
	rootDir := t.TempDir()
	restoreWorkingDir := chdirPluginHostTemplateTest(t, rootDir)
	defer restoreWorkingDir()

	templateDir := filepath.Join(rootDir, "templates", "email")
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		t.Fatalf("mkdir email template dir failed: %v", err)
	}
	templatePath := filepath.Join(templateDir, "order_paid.html")
	if err := os.WriteFile(templatePath, []byte("Initial {{.OrderNo}}"), 0o644); err != nil {
		t.Fatalf("write initial email template failed: %v", err)
	}

	emailService := NewEmailService(nil, &config.SMTPConfig{}, "https://example.com")
	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.TemplateVersion{}, &models.OperationLog{}); err != nil {
		t.Fatalf("auto migrate template import tables failed: %v", err)
	}

	packageBytes := buildTemplatePackageZip(t, map[string]string{
		"manifest.json": `{
  "kind": "email_template",
  "name": "order_paid",
  "version": "1.2.0",
  "event": "order_paid",
  "engine": "go_template",
  "content_file": "template.html"
}`,
		"template.html": `Imported {{.OrderNo}}`,
	})

	result, err := ImportTemplatePackage(db, emailService, "order-paid.zip", packageBytes, TemplatePackageImportOptions{
		ExpectedKind: "email_template",
	})
	if err != nil {
		t.Fatalf("import template package failed: %v", err)
	}
	if got := result["kind"]; got != "email_template" {
		t.Fatalf("expected imported kind email_template, got %#v", got)
	}
	if got := result["hot_reloaded"]; got != true {
		t.Fatalf("expected hot_reloaded=true, got %#v", got)
	}

	savedBytes, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("read saved email template failed: %v", err)
	}
	if string(savedBytes) != "Imported {{.OrderNo}}" {
		t.Fatalf("expected imported email template content, got %q", string(savedBytes))
	}

	rendered, err := emailService.renderTemplate("order_paid", "en", map[string]interface{}{
		"OrderNo": "A-003",
	})
	if err != nil {
		t.Fatalf("render imported template failed: %v", err)
	}
	if rendered != "Imported A-003" {
		t.Fatalf("expected imported template render, got %q", rendered)
	}

	var activeVersion models.TemplateVersion
	if err := db.Where("resource_kind = ? AND target_key = ? AND is_active = ?", "email_template", "order_paid", true).First(&activeVersion).Error; err != nil {
		t.Fatalf("load active template version failed: %v", err)
	}
	if activeVersion.MarketArtifactVersion != "1.2.0" {
		t.Fatalf("expected template version 1.2.0, got %+v", activeVersion)
	}
}

func TestImportTemplatePackageImportsLandingPageAndCreatesSnapshot(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.TemplateVersion{}, &models.LandingPage{}); err != nil {
		t.Fatalf("auto migrate landing template import tables failed: %v", err)
	}
	if err := db.Create(&models.LandingPage{
		Slug:        "home",
		HTMLContent: "<html>Initial landing</html>",
		IsActive:    true,
	}).Error; err != nil {
		t.Fatalf("create initial landing page failed: %v", err)
	}

	packageBytes := buildTemplatePackageZip(t, map[string]string{
		"manifest.json": `{
  "kind": "landing_page_template",
  "name": "home",
  "version": "2.0.0",
  "slug": "home",
  "engine": "go_template",
  "content_file": "landing.html"
}`,
		"landing.html": `<html>{{.AppName}}</html>`,
	})

	result, err := ImportTemplatePackage(db, nil, "landing-home.zip", packageBytes, TemplatePackageImportOptions{
		ExpectedKind: "landing_page_template",
	})
	if err != nil {
		t.Fatalf("import landing template package failed: %v", err)
	}
	if got := result["kind"]; got != "landing_page_template" {
		t.Fatalf("expected imported kind landing_page_template, got %#v", got)
	}

	var page models.LandingPage
	if err := db.Where("slug = ?", "home").First(&page).Error; err != nil {
		t.Fatalf("reload landing page failed: %v", err)
	}
	if page.HTMLContent != "<html>{{.AppName}}</html>" {
		t.Fatalf("expected imported landing page content, got %q", page.HTMLContent)
	}

	var activeVersion models.TemplateVersion
	if err := db.Where("resource_kind = ? AND target_key = ? AND is_active = ?", "landing_page_template", "home", true).First(&activeVersion).Error; err != nil {
		t.Fatalf("load active landing template version failed: %v", err)
	}
	if activeVersion.MarketArtifactVersion != "2.0.0" {
		t.Fatalf("expected landing template version 2.0.0, got %+v", activeVersion)
	}
}

func TestImportTemplatePackageImportsInvoiceTemplateAndCreatesSnapshot(t *testing.T) {
	_, configPath, restoreConfigPath := preparePluginHostConfigFile(t, map[string]interface{}{
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
	if err := db.AutoMigrate(&models.TemplateVersion{}); err != nil {
		t.Fatalf("auto migrate invoice template import tables failed: %v", err)
	}

	packageBytes := buildTemplatePackageZip(t, map[string]string{
		"manifest.json": `{
  "kind": "invoice_template",
  "name": "invoice",
  "version": "3.0.0",
  "key": "invoice",
  "content_file": "invoice.html"
}`,
		"invoice.html": `<html>Invoice {{.OrderNo}}</html>`,
	})

	result, err := ImportTemplatePackage(db, nil, "invoice-template.zip", packageBytes, TemplatePackageImportOptions{
		ExpectedKind: "invoice_template",
	})
	if err != nil {
		t.Fatalf("import invoice template package failed: %v", err)
	}
	if got := result["kind"]; got != "invoice_template" {
		t.Fatalf("expected imported kind invoice_template, got %#v", got)
	}

	document := readPluginHostConfigDocument(t, configPath)
	orderDocument := document["order"].(map[string]interface{})
	invoiceDocument := orderDocument["invoice"].(map[string]interface{})
	if got := invoiceDocument["template_type"]; got != "custom" {
		t.Fatalf("expected invoice template_type custom, got %#v", got)
	}
	if got := invoiceDocument["custom_template"]; got != "<html>Invoice {{.OrderNo}}</html>" {
		t.Fatalf("expected imported invoice template content, got %#v", got)
	}

	var activeVersion models.TemplateVersion
	if err := db.Where("resource_kind = ? AND target_key = ? AND is_active = ?", "invoice_template", pluginHostInvoiceTemplateTargetKey, true).First(&activeVersion).Error; err != nil {
		t.Fatalf("load active invoice template version failed: %v", err)
	}
	if activeVersion.MarketArtifactVersion != "3.0.0" {
		t.Fatalf("expected invoice template version 3.0.0, got %+v", activeVersion)
	}
}

func TestImportTemplatePackageImportsAuthBrandingTemplateAndCreatesSnapshot(t *testing.T) {
	_, configPath, restoreConfigPath := preparePluginHostConfigFile(t, map[string]interface{}{
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
	if err := db.AutoMigrate(&models.TemplateVersion{}); err != nil {
		t.Fatalf("auto migrate auth branding template import tables failed: %v", err)
	}

	packageBytes := buildTemplatePackageZip(t, map[string]string{
		"manifest.json": `{
  "kind": "auth_branding_template",
  "name": "auth_branding",
  "version": "4.0.0",
  "key": "auth_branding",
  "content_file": "branding.html"
}`,
		"branding.html": `<section>Branding {{.AppName}}</section>`,
	})

	result, err := ImportTemplatePackage(db, nil, "auth-branding-template.zip", packageBytes, TemplatePackageImportOptions{
		ExpectedKind: "auth_branding_template",
	})
	if err != nil {
		t.Fatalf("import auth branding template package failed: %v", err)
	}
	if got := result["kind"]; got != "auth_branding_template" {
		t.Fatalf("expected imported kind auth_branding_template, got %#v", got)
	}

	document := readPluginHostConfigDocument(t, configPath)
	customizationDocument := document["customization"].(map[string]interface{})
	authBrandingDocument := customizationDocument["auth_branding"].(map[string]interface{})
	if got := authBrandingDocument["mode"]; got != "custom" {
		t.Fatalf("expected auth branding mode custom, got %#v", got)
	}
	if got := authBrandingDocument["custom_html"]; got != "<section>Branding {{.AppName}}</section>" {
		t.Fatalf("expected imported auth branding template content, got %#v", got)
	}

	var activeVersion models.TemplateVersion
	if err := db.Where("resource_kind = ? AND target_key = ? AND is_active = ?", "auth_branding_template", pluginHostAuthBrandingTemplateTargetKey, true).First(&activeVersion).Error; err != nil {
		t.Fatalf("load active auth branding template version failed: %v", err)
	}
	if activeVersion.MarketArtifactVersion != "4.0.0" {
		t.Fatalf("expected auth branding template version 4.0.0, got %+v", activeVersion)
	}
}

func TestImportTemplatePackageImportsPageRulePackAndCreatesSnapshot(t *testing.T) {
	_, configPath, restoreConfigPath := preparePluginHostConfigFile(t, map[string]interface{}{
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

	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.TemplateVersion{}); err != nil {
		t.Fatalf("auto migrate page rule pack import tables failed: %v", err)
	}

	packageBytes := buildTemplatePackageZip(t, map[string]string{
		"manifest.json": `{
  "kind": "page_rule_pack",
  "name": "page_rules",
  "version": "5.0.0",
  "key": "page_rules",
  "rules_file": "page-rules.json"
}`,
		"page-rules.json": `[
  {
    "name": "Checkout enhancement",
    "pattern": "^/checkout",
    "match_type": "regex",
    "css": "body{color:#0f172a;}",
    "js": "console.log('checkout');",
    "enabled": true
  }
]`,
	})

	result, err := ImportTemplatePackage(db, nil, "page-rule-pack.zip", packageBytes, TemplatePackageImportOptions{
		ExpectedKind: "page_rule_pack",
	})
	if err != nil {
		t.Fatalf("import page rule pack failed: %v", err)
	}
	if got := result["kind"]; got != "page_rule_pack" {
		t.Fatalf("expected imported kind page_rule_pack, got %#v", got)
	}

	document := readPluginHostConfigDocument(t, configPath)
	customizationDocument := document["customization"].(map[string]interface{})
	pageRulesDocument := customizationDocument["page_rules"].([]interface{})
	if len(pageRulesDocument) != 1 {
		t.Fatalf("expected imported page rule pack content, got %#v", customizationDocument["page_rules"])
	}

	var activeVersion models.TemplateVersion
	if err := db.Where("resource_kind = ? AND target_key = ? AND is_active = ?", "page_rule_pack", pluginHostPageRulePackTargetKey, true).First(&activeVersion).Error; err != nil {
		t.Fatalf("load active page rule pack version failed: %v", err)
	}
	if activeVersion.MarketArtifactVersion != "5.0.0" {
		t.Fatalf("expected page rule pack version 5.0.0, got %+v", activeVersion)
	}
}

func buildTemplatePackageZip(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	for name, content := range files {
		writer, err := archive.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s failed: %v", name, err)
		}
		if _, err := writer.Write([]byte(content)); err != nil {
			t.Fatalf("write zip entry %s failed: %v", name, err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close zip archive failed: %v", err)
	}
	return buffer.Bytes()
}
