package auth

import (
	"testing"
	"time"
)

func TestServiceLoginAndValidateToken(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	svc := NewServiceWithConfig(Config{
		AdminUsername: "operator",
		AdminPassword: "secret",
		TokenTTL:      30 * time.Minute,
		Now: func() time.Time {
			return now
		},
	})

	token, err := svc.Login("operator", "secret")
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	user, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken returned error: %v", err)
	}
	if user.Username != "operator" {
		t.Fatalf("expected operator user, got %#v", user)
	}
}

func TestServiceRejectsExpiredToken(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	svc := NewServiceWithConfig(Config{
		AdminPassword: "admin",
		TokenTTL:      time.Minute,
		Now: func() time.Time {
			return now
		},
	})

	token, err := svc.Login("admin", "admin")
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}

	now = now.Add(2 * time.Minute)
	if _, err := svc.ValidateToken(token); err == nil {
		t.Fatal("expected expired token to be rejected")
	}
	if _, err := svc.ValidateToken(token); err == nil {
		t.Fatal("expected expired token to be removed after rejection")
	}
}

func TestServiceRejectsInvalidCredentials(t *testing.T) {
	svc := NewServiceWithConfig(Config{
		AdminUsername: "operator",
		AdminPassword: "secret",
	})

	if _, err := svc.Login("operator", "wrong"); err == nil {
		t.Fatal("expected invalid password to be rejected")
	}
	if _, err := svc.Login("missing", "secret"); err == nil {
		t.Fatal("expected missing user to be rejected")
	}
}
