package admin

import "testing"

func TestResolveFrontendBootstrapErrorFallbackReturnsEmptyPayloadForPublicEndpoint(t *testing.T) {
	payload, ok := resolveFrontendBootstrapErrorFallback("user", "/orders", true, assertTestError("boom"))
	if !ok {
		t.Fatalf("expected public bootstrap failure to degrade")
	}
	if payload.Area != frontendBootstrapAreaUser {
		t.Fatalf("expected user area, got %q", payload.Area)
	}
	if payload.Path != "/orders" {
		t.Fatalf("expected normalized path /orders, got %q", payload.Path)
	}
	if len(payload.Menus) != 0 {
		t.Fatalf("expected no menus in degraded payload, got %+v", payload.Menus)
	}
	if len(payload.Routes) != 0 {
		t.Fatalf("expected no routes in degraded payload, got %+v", payload.Routes)
	}
}

func TestResolveFrontendBootstrapErrorFallbackKeepsAdminStrict(t *testing.T) {
	if payload, ok := resolveFrontendBootstrapErrorFallback("admin", "/admin/orders", false, assertTestError("boom")); ok {
		t.Fatalf("expected admin bootstrap failure to remain strict, got %+v", payload)
	}
}

func TestResolveFrontendBootstrapErrorFallbackRequiresError(t *testing.T) {
	if payload, ok := resolveFrontendBootstrapErrorFallback("user", "/orders", true, nil); ok {
		t.Fatalf("expected nil error to skip fallback, got %+v", payload)
	}
}

type assertTestError string

func (e assertTestError) Error() string {
	return string(e)
}
