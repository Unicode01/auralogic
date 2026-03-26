package service

import (
	"strings"
	"testing"
	"time"
)

func TestBuildPluginExecutionAuditPayloadSanitizesSensitiveValues(t *testing.T) {
	userID := uint(7)
	orderID := uint(11)
	svc := &PluginManagerService{}

	payload := svc.buildPluginExecutionAuditPayload(pluginExecutionAuditEntry{
		Timestamp:  time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC),
		PluginID:   1,
		PluginName: "audit-plugin",
		Runtime:    "grpc",
		Action:     "hook.execute",
		DurationMs: 42,
		Params: map[string]string{
			"token":   "secret-token",
			"payload": `{"message":"ok","nested":{"password":"hidden","keep":"value"}}`,
		},
		HasContext: true,
		SessionID:  "session-1",
		ContextMetadata: map[string]string{
			"authorization": "Bearer abc",
			"trace":         "trace-1",
		},
		UserID:  &userID,
		OrderID: &orderID,
		Success: true,
		ResultData: map[string]interface{}{
			"token": "secret-result",
			"nested": map[string]interface{}{
				"password": "result-hidden",
				"keep":     "value",
			},
		},
	})

	params, ok := payload["params"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected params map, got %#v", payload["params"])
	}
	if got := params["token"]; got != "[REDACTED]" {
		t.Fatalf("expected token redacted, got %#v", got)
	}
	payloadJSON, ok := params["payload"].(string)
	if !ok {
		t.Fatalf("expected payload param string, got %#v", params["payload"])
	}
	if !strings.Contains(payloadJSON, "[REDACTED]") {
		t.Fatalf("expected payload param to include redacted marker, got %q", payloadJSON)
	}
	if strings.Contains(payloadJSON, "hidden") {
		t.Fatalf("expected payload param secret to be removed, got %q", payloadJSON)
	}

	contextPayload, ok := payload["context"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected context map, got %#v", payload["context"])
	}
	metadata, ok := contextPayload["metadata"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected context metadata map, got %#v", contextPayload["metadata"])
	}
	if got := metadata["authorization"]; got != "[REDACTED]" {
		t.Fatalf("expected authorization redacted, got %#v", got)
	}
	if got := metadata["trace"]; got != "trace-1" {
		t.Fatalf("expected trace preserved, got %#v", got)
	}

	result, ok := payload["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result map, got %#v", payload["result"])
	}
	if got := result["token"]; got != "[REDACTED]" {
		t.Fatalf("expected result token redacted, got %#v", got)
	}
	nested, ok := result["nested"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected nested result map, got %#v", result["nested"])
	}
	if got := nested["password"]; got != "[REDACTED]" {
		t.Fatalf("expected nested password redacted, got %#v", got)
	}
	if got := nested["keep"]; got != "value" {
		t.Fatalf("expected nested keep preserved, got %#v", got)
	}
}

func TestTryEnqueuePluginExecutionAuditEntryFallsBackWhenQueueUnavailable(t *testing.T) {
	svc := &PluginManagerService{}
	status := svc.tryEnqueuePluginExecutionAuditEntry(pluginExecutionAuditEntry{PluginID: 1})
	if status != pluginExecutionAuditQueueSyncFallback {
		t.Fatalf("expected sync fallback, got %d", status)
	}
	if dropped := svc.auditLogDropped.Load(); dropped != 0 {
		t.Fatalf("expected no dropped counter increment, got %d", dropped)
	}
}

func TestTryEnqueuePluginExecutionAuditEntryDropsWhenQueueFull(t *testing.T) {
	svc := &PluginManagerService{
		auditLogQueue: make(chan pluginExecutionAuditEntry, 1),
	}
	svc.auditLogQueue <- pluginExecutionAuditEntry{PluginID: 1}

	status := svc.tryEnqueuePluginExecutionAuditEntry(pluginExecutionAuditEntry{PluginID: 2})
	if status != pluginExecutionAuditQueueDropped {
		t.Fatalf("expected dropped status, got %d", status)
	}
	if dropped := svc.auditLogDropped.Load(); dropped != 1 {
		t.Fatalf("expected dropped counter=1, got %d", dropped)
	}
}
