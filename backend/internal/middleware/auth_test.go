package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestExtractBearerTokenFromWebSocketSubprotocol(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	request := httptest.NewRequest("GET", "/ws", nil)
	request.Header.Set("Connection", "Upgrade")
	request.Header.Set("Upgrade", "websocket")
	request.Header.Set(
		"Sec-WebSocket-Protocol",
		"auralogic.workspace.v1, auralogic.auth.bearer.test.jwt.token",
	)
	ctx.Request = request

	if got := extractBearerToken(ctx); got != "test.jwt.token" {
		t.Fatalf("expected websocket subprotocol token, got %q", got)
	}
}

func TestExtractBearerTokenRejectsWebSocketQueryToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	request := httptest.NewRequest("GET", "/ws?access_token=test.jwt.token", nil)
	request.Header.Set("Connection", "Upgrade")
	request.Header.Set("Upgrade", "websocket")
	ctx.Request = request

	if got := extractBearerToken(ctx); got != "" {
		t.Fatalf("expected websocket query token to be ignored, got %q", got)
	}
}
