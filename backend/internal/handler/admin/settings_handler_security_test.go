package admin

import (
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"testing"

	"auralogic/internal/config"
	"auralogic/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

func TestGetSettingsHidesSensitiveSMSAndCaptchaValues(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{}
	cfg.App.DefaultTheme = "system"
	cfg.SMS.CustomHeaders = map[string]string{
		"Authorization": "Bearer secret-token",
		"X-Signature":   "signed-value",
	}
	cfg.Security.Captcha = config.CaptchaConfig{
		Provider:              "cloudflare",
		SiteKey:               "site-key",
		SecretKey:             "server-secret",
		EnableForLogin:        true,
		EnableForRegister:     true,
		EnableForBind:         true,
		EnableForSerialVerify: true,
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	handler := &SettingsHandler{cfg: cfg}
	handler.GetSettings(ctx)

	if recorder.Code != 200 {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	var resp response.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected response data object, got %T", resp.Data)
	}

	sms, ok := data["sms"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected sms settings object, got %T", data["sms"])
	}
	if _, exists := sms["custom_headers"]; exists {
		t.Fatalf("expected custom_headers to stay hidden, got %#v", sms["custom_headers"])
	}
	if configured, ok := sms["custom_headers_configured"].(bool); !ok || !configured {
		t.Fatalf("expected custom_headers_configured=true, got %#v", sms["custom_headers_configured"])
	}
	expectedHeaderKeys := []interface{}{"Authorization", "X-Signature"}
	if got, ok := sms["custom_header_keys"].([]interface{}); !ok || !reflect.DeepEqual(got, expectedHeaderKeys) {
		t.Fatalf("expected custom_header_keys=%#v, got %#v", expectedHeaderKeys, sms["custom_header_keys"])
	}

	security, ok := data["security"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected security settings object, got %T", data["security"])
	}
	captcha, ok := security["captcha"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected captcha settings object, got %T", security["captcha"])
	}
	if _, exists := captcha["secret_key"]; exists {
		t.Fatalf("expected secret_key to stay hidden, got %#v", captcha["secret_key"])
	}
	if configured, ok := captcha["secret_key_configured"].(bool); !ok || !configured {
		t.Fatalf("expected secret_key_configured=true, got %#v", captcha["secret_key_configured"])
	}
}

func TestResolveCaptchaSecretForUpdate(t *testing.T) {
	existing := map[string]interface{}{
		"provider":   "cloudflare",
		"secret_key": "current-secret",
	}

	t.Run("preserves existing secret when provider unchanged and secret not resubmitted", func(t *testing.T) {
		req := &settingsCaptchaUpdateRequest{
			Provider:           "cloudflare",
			SecretKeySubmitted: false,
		}
		if got := resolveCaptchaSecretForUpdate(existing, req); got != "current-secret" {
			t.Fatalf("expected current-secret, got %q", got)
		}
	})

	t.Run("clears secret when switching between providers without resubmission", func(t *testing.T) {
		req := &settingsCaptchaUpdateRequest{
			Provider:           "google",
			SecretKeySubmitted: false,
		}
		if got := resolveCaptchaSecretForUpdate(existing, req); got != "" {
			t.Fatalf("expected empty secret after provider change, got %q", got)
		}
	})

	t.Run("clears secret when new provider does not require one", func(t *testing.T) {
		req := &settingsCaptchaUpdateRequest{
			Provider:           "builtin",
			SecretKeySubmitted: false,
		}
		if got := resolveCaptchaSecretForUpdate(existing, req); got != "" {
			t.Fatalf("expected empty secret for builtin provider, got %q", got)
		}
	})
}
