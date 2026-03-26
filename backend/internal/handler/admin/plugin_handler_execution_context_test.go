package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
)

func TestEnsureOperatorUserIDPreservesSubjectUserForAdminRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adminID := uint(42)
	subjectUserID := uint(7)
	request := httptest.NewRequest(http.MethodPost, "/api/admin/plugins/7/execute", nil)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = request
	ctx.Set("user_id", adminID)
	ctx.Set("user_role", "admin")

	execCtx := ensureOperatorUserID(ctx, &service.ExecutionContext{
		UserID: &subjectUserID,
	})
	if execCtx == nil {
		t.Fatalf("expected execution context")
	}
	if execCtx.UserID == nil || *execCtx.UserID != subjectUserID {
		t.Fatalf("expected subject user id %d to remain, got %#v", subjectUserID, execCtx.UserID)
	}
	if execCtx.OperatorUserID == nil || *execCtx.OperatorUserID != adminID {
		t.Fatalf("expected operator user id %d, got %#v", adminID, execCtx.OperatorUserID)
	}

	claims := service.BuildPluginHostAccessClaims(nil, execCtx, time.Minute)
	if claims.OperatorUserID != adminID {
		t.Fatalf("expected operator claims user id %d, got %d", adminID, claims.OperatorUserID)
	}
}

func TestEnsureOperatorUserIDUsesExplicitZeroForAPIKeyRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)

	subjectUserID := uint(7)
	request := httptest.NewRequest(http.MethodPost, "/api/admin/plugins/7/execute", nil)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = request
	ctx.Set("auth_type", "api_key")
	ctx.Set("api_scopes", []string{"plugin.execute"})

	execCtx := ensureOperatorUserID(ctx, &service.ExecutionContext{
		UserID: &subjectUserID,
	})
	if execCtx == nil {
		t.Fatalf("expected execution context")
	}
	if execCtx.OperatorUserID == nil {
		t.Fatalf("expected explicit operator user id marker for api key request")
	}
	if *execCtx.OperatorUserID != 0 {
		t.Fatalf("expected api key operator marker to be zero, got %d", *execCtx.OperatorUserID)
	}

	claims := service.BuildPluginHostAccessClaims(nil, execCtx, time.Minute)
	if claims.OperatorUserID != 0 {
		t.Fatalf("expected api key claims operator user id to stay empty, got %d", claims.OperatorUserID)
	}
}

func TestMergePluginExecutionContextDefaultsPreservesOperatorUserID(t *testing.T) {
	subjectUserID := uint(7)
	operatorUserID := uint(42)
	fallbackOperatorUserID := uint(99)

	merged := mergePluginExecutionContextDefaults(
		&service.ExecutionContext{
			UserID:         &subjectUserID,
			OperatorUserID: &operatorUserID,
		},
		&service.ExecutionContext{
			OperatorUserID: &fallbackOperatorUserID,
		},
	)
	if merged == nil {
		t.Fatalf("expected merged execution context")
	}
	if merged.OperatorUserID == nil || *merged.OperatorUserID != operatorUserID {
		t.Fatalf("expected merged operator user id %d, got %#v", operatorUserID, merged.OperatorUserID)
	}
}
