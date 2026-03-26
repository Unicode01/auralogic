package utils

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"auralogic/internal/config"
	"github.com/gin-gonic/gin"
)

func loadIPTestConfig(t *testing.T) *config.Config {
	t.Helper()

	configPath := filepath.Join(t.TempDir(), "config.json")
	configJSON := `{
  "app": {"name": "test", "port": 8080},
  "database": {"driver": "sqlite", "name": "test.db"},
  "jwt": {"secret": "12345678901234567890123456789012"},
  "security": {
    "ip_header": "",
    "trusted_proxies": [],
    "cors": {},
    "login": {},
    "password_policy": {"min_length": 8},
    "captcha": {}
  }
}`
	if err := os.WriteFile(configPath, []byte(configJSON), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	return cfg
}

func newIPTestContext(remoteAddr string, headers map[string]string) *gin.Context {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = remoteAddr
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	ctx.Request = req
	return ctx
}

func TestGetRealIPIgnoresForwardedHeadersWithoutTrustedProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := loadIPTestConfig(t)
	cfg.Security.IPHeader = ""
	cfg.Security.TrustedProxies = nil

	ctx := newIPTestContext("203.0.113.10:12345", map[string]string{
		"X-Forwarded-For": "198.51.100.20",
		"X-Real-IP":       "198.51.100.21",
	})

	if got := GetRealIP(ctx); got != "203.0.113.10" {
		t.Fatalf("expected peer IP to be used, got %q", got)
	}
}

func TestGetRealIPUsesForwardedHeaderFromTrustedProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := loadIPTestConfig(t)
	cfg.Security.IPHeader = "X-Forwarded-For"
	cfg.Security.TrustedProxies = []string{"10.0.0.0/8"}

	ctx := newIPTestContext("10.1.2.3:8080", map[string]string{
		"X-Forwarded-For": "198.51.100.20, 10.1.2.3",
	})

	if got := GetRealIP(ctx); got != "198.51.100.20" {
		t.Fatalf("expected forwarded IP to be used, got %q", got)
	}
}
