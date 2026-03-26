package middleware

import "testing"

func TestRegisteredAdminPermissionsMapIncludesPluginPermissionGroup(t *testing.T) {
	permissions := RegisteredAdminPermissionsMap()
	group, ok := permissions["PluginPermission"]
	if !ok {
		t.Fatalf("expected PluginPermission group to exist")
	}

	expected := []string{
		"plugin.view",
		"plugin.edit",
		"plugin.execute",
		"plugin.lifecycle",
		"plugin.diagnostics",
		"plugin.upload",
	}
	for _, permission := range expected {
		if !containsPermission(group, permission) {
			t.Fatalf("expected plugin permission %q to be registered", permission)
		}
	}

	group[0] = "mutated"
	refetched := RegisteredAdminPermissionsMap()["PluginPermission"]
	if refetched[0] != "plugin.view" {
		t.Fatalf("expected returned permission groups to be defensive copies")
	}
}

func TestRegisteredAdminPermissionsMapIncludesPaymentMethodPermissionGroup(t *testing.T) {
	permissions := RegisteredAdminPermissionsMap()
	group, ok := permissions["PaymentMethodPermission"]
	if !ok {
		t.Fatalf("expected PaymentMethodPermission group to exist")
	}
	if !containsPermission(group, "payment_method.view") {
		t.Fatalf("expected payment_method.view to be registered")
	}
	if !containsPermission(group, "payment_method.edit") {
		t.Fatalf("expected payment_method.edit to be registered")
	}
}

func TestRegisteredAdminPermissionsMapIncludesMarketAndTemplatePermissionGroups(t *testing.T) {
	permissions := RegisteredAdminPermissionsMap()

	marketGroup, ok := permissions["MarketPermission"]
	if !ok {
		t.Fatalf("expected MarketPermission group to exist")
	}
	for _, permission := range []string{"market.view", "market.install", "market.history", "market.review", "market.manage"} {
		if !containsPermission(marketGroup, permission) {
			t.Fatalf("expected market permission %q to be registered", permission)
		}
	}

	emailTemplateGroup, ok := permissions["EmailTemplatePermission"]
	if !ok {
		t.Fatalf("expected EmailTemplatePermission group to exist")
	}
	for _, permission := range []string{"email_template.view", "email_template.edit"} {
		if !containsPermission(emailTemplateGroup, permission) {
			t.Fatalf("expected email template permission %q to be registered", permission)
		}
	}

	landingPageGroup, ok := permissions["LandingPagePermission"]
	if !ok {
		t.Fatalf("expected LandingPagePermission group to exist")
	}
	for _, permission := range []string{"landing_page.view", "landing_page.edit"} {
		if !containsPermission(landingPageGroup, permission) {
			t.Fatalf("expected landing page permission %q to be registered", permission)
		}
	}

	invoiceTemplateGroup, ok := permissions["InvoiceTemplatePermission"]
	if !ok {
		t.Fatalf("expected InvoiceTemplatePermission group to exist")
	}
	for _, permission := range []string{"invoice_template.view", "invoice_template.edit"} {
		if !containsPermission(invoiceTemplateGroup, permission) {
			t.Fatalf("expected invoice template permission %q to be registered", permission)
		}
	}

	authBrandingGroup, ok := permissions["AuthBrandingPermission"]
	if !ok {
		t.Fatalf("expected AuthBrandingPermission group to exist")
	}
	for _, permission := range []string{"auth_branding.view", "auth_branding.edit"} {
		if !containsPermission(authBrandingGroup, permission) {
			t.Fatalf("expected auth branding permission %q to be registered", permission)
		}
	}
}

func TestEffectiveAdminPermissionsMergesSuperAdminDefaultsAndExplicitPermissions(t *testing.T) {
	permissions := EffectiveAdminPermissions("super_admin", []string{"order.view_privacy", "plugin.view"})

	if !containsPermission(permissions, "plugin.lifecycle") {
		t.Fatalf("expected default super admin permissions to include plugin.lifecycle")
	}
	if !containsPermission(permissions, "order.view_privacy") {
		t.Fatalf("expected explicit special permission to be preserved")
	}
	if countPermission(permissions, "plugin.view") != 1 {
		t.Fatalf("expected merged permissions to de-duplicate plugin.view")
	}
	if containsPermission(DefaultAdminPermissionsForRole("super_admin"), "order.view_privacy") {
		t.Fatalf("expected special permissions to be excluded from default super admin permissions")
	}
}

func containsPermission(permissions []string, target string) bool {
	for _, permission := range permissions {
		if permission == target {
			return true
		}
	}
	return false
}

func countPermission(permissions []string, target string) int {
	count := 0
	for _, permission := range permissions {
		if permission == target {
			count++
		}
	}
	return count
}
