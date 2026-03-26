package user

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"auralogic/internal/pkg/authbiz"
	"auralogic/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

func respondAuthBizErrorForTest(t *testing.T, err error) (int, response.Response) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/auth/test", nil)

	if !respondAuthBizError(ctx, err, nil) {
		t.Fatalf("expected biz error to be handled")
	}

	var resp response.Response
	if unmarshalErr := json.Unmarshal(recorder.Body.Bytes(), &resp); unmarshalErr != nil {
		t.Fatalf("decode response: %v", unmarshalErr)
	}
	return recorder.Code, resp
}

func TestRespondAuthBizErrorReturnsServiceUnavailable(t *testing.T) {
	httpCode, resp := respondAuthBizErrorForTest(t, authbiz.EmailLoginUnavailable())

	if httpCode != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, httpCode)
	}
	if resp.Code != response.CodeServiceUnavailable {
		t.Fatalf("expected response code %d, got %d", response.CodeServiceUnavailable, resp.Code)
	}
}

func TestRespondAuthBizErrorReturnsForbiddenForFeatureDisabled(t *testing.T) {
	httpCode, resp := respondAuthBizErrorForTest(t, authbiz.PhoneRegistrationDisabled())

	if httpCode != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, httpCode)
	}
	if resp.Code != response.CodeForbidden {
		t.Fatalf("expected response code %d, got %d", response.CodeForbidden, resp.Code)
	}
}

func TestRespondAuthBizErrorReturnsUserNotFound(t *testing.T) {
	httpCode, resp := respondAuthBizErrorForTest(t, authbiz.UserNotFound())

	if httpCode != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, httpCode)
	}
	if resp.Code != response.CodeUserNotFound {
		t.Fatalf("expected response code %d, got %d", response.CodeUserNotFound, resp.Code)
	}
}
