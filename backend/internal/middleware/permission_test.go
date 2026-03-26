package middleware

import "testing"

func TestJWTAnyPermissionAllowsSuperAdminForNonSpecialPermission(t *testing.T) {
	entry := &permCacheEntry{Role: "super_admin"}
	if !jwtHasPermissions(entry, []string{"order.view_privacy", "plugin.view"}, permissionMatchAny) {
		t.Fatalf("expected super admin to satisfy any-permission check via non-special permission")
	}
}

func TestJWTAllPermissionsRequireExplicitSpecialPermissionForSuperAdmin(t *testing.T) {
	entry := &permCacheEntry{Role: "super_admin"}
	if jwtHasPermissions(entry, []string{"order.view_privacy", "plugin.view"}, permissionMatchAll) {
		t.Fatalf("expected special permission to require explicit grant")
	}

	entry.Permissions = []string{"order.view_privacy"}
	if !jwtHasPermissions(entry, []string{"order.view_privacy", "plugin.view"}, permissionMatchAll) {
		t.Fatalf("expected super admin to pass once special permission is explicitly granted")
	}
}

func TestPermissionSetMatchesSupportsAnyAndAll(t *testing.T) {
	permissionSet := buildPermissionSet([]string{"plugin.view", "plugin.execute"})
	if !permissionSetMatches(permissionSet, []string{"plugin.lifecycle", "plugin.execute"}, permissionMatchAny) {
		t.Fatalf("expected any-permission check to pass")
	}
	if permissionSetMatches(permissionSet, []string{"plugin.view", "plugin.lifecycle"}, permissionMatchAll) {
		t.Fatalf("expected all-permission check to fail when one permission is missing")
	}
}
