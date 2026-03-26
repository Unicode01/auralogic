package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"auralogic/internal/config"
	"auralogic/internal/models"
	"auralogic/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

func TestCreatePaymentMethodRejectsBuiltinDirectCreate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := openPaymentMethodPackageTestDB(t)
	handler := NewPaymentMethodHandler(db, &config.Config{}, nil)

	payload := map[string]interface{}{
		"name":          "Builtin Direct Create",
		"type":          "builtin",
		"script":        "function onGeneratePaymentCard() { return { html: '<div>bad</div>' } }",
		"config":        "{}",
		"icon":          "CreditCard",
		"poll_interval": 30,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload failed: %v", err)
	}

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/admin/payment-methods", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	handler.Create(ctx)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp response.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response failed: %v, body=%s", err, rec.Body.String())
	}
	if resp.Code != response.CodeParamError {
		t.Fatalf("expected param error code, got %d", resp.Code)
	}
	if !strings.Contains(strings.ToLower(resp.Message), "package governance") {
		t.Fatalf("expected package governance hint, got %q", resp.Message)
	}

	var count int64
	if err := db.Model(&models.PaymentMethod{}).Count(&count).Error; err != nil {
		t.Fatalf("count payment methods failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no payment method to be created, got %d", count)
	}
}
