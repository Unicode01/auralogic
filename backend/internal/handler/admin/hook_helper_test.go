package admin

import (
	"context"
	"testing"

	"auralogic/internal/service"
)

func TestCloneAdminHookExecutionContextDetachesRequestContext(t *testing.T) {
	operatorUserID := uint(42)
	subjectUserID := uint(7)
	orderID := uint(9)
	reqCtx, cancel := context.WithCancel(context.Background())

	cloned := cloneAdminHookExecutionContext(&service.ExecutionContext{
		OperatorUserID: &operatorUserID,
		UserID:         &subjectUserID,
		OrderID:        &orderID,
		SessionID:      "session-1",
		RequestContext: reqCtx,
		Metadata: map[string]string{
			"hook_source": "admin_api",
		},
	})
	cancel()

	if cloned == nil {
		t.Fatalf("expected cloned execution context")
	}
	if cloned.RequestContext == nil {
		t.Fatalf("expected detached request context")
	}
	if err := cloned.RequestContext.Err(); err != nil {
		t.Fatalf("expected detached request context to stay active, got %v", err)
	}
	if cloned.OperatorUserID == nil || *cloned.OperatorUserID != operatorUserID {
		t.Fatalf("expected operator user id %d, got %#v", operatorUserID, cloned.OperatorUserID)
	}
	if cloned.UserID == nil || *cloned.UserID != subjectUserID {
		t.Fatalf("expected subject user id %d, got %#v", subjectUserID, cloned.UserID)
	}
	if cloned.OrderID == nil || *cloned.OrderID != orderID {
		t.Fatalf("expected order id %d, got %#v", orderID, cloned.OrderID)
	}
	if cloned.Metadata["hook_source"] != "admin_api" {
		t.Fatalf("expected metadata to be preserved, got %#v", cloned.Metadata)
	}
}
