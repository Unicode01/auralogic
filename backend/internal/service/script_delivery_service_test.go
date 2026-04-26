package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/models"

	"github.com/dop251/goja"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestScriptDeliveryResolveExecutionTimeoutMs(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		hostMaxMs  int
		configData map[string]interface{}
		wantMs     int
	}{
		{
			name:      "defaults to legacy timeout",
			hostMaxMs: 0,
			wantMs:    10000,
		},
		{
			name:      "uses host max when request absent",
			hostMaxMs: 25000,
			wantMs:    25000,
		},
		{
			name:      "honors shorter requested timeout",
			hostMaxMs: 25000,
			configData: map[string]interface{}{
				"timeout_ms": 1200,
			},
			wantMs: 1200,
		},
		{
			name:      "caps longer requested timeout",
			hostMaxMs: 1500,
			configData: map[string]interface{}{
				"timeout_ms": 9000,
			},
			wantMs: 1500,
		},
		{
			name:      "supports camel case key and string value",
			hostMaxMs: 8000,
			configData: map[string]interface{}{
				"timeoutMs": "6500",
			},
			wantMs: 6500,
		},
		{
			name:      "ignores invalid requested timeout",
			hostMaxMs: 1800,
			configData: map[string]interface{}{
				"timeout_ms": "abc",
			},
			wantMs: 1800,
		},
		{
			name:      "normalizes very small requested timeout",
			hostMaxMs: 1800,
			configData: map[string]interface{}{
				"timeout_ms": 1,
			},
			wantMs: 100,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := NewScriptDeliveryService(nil, &config.Config{
				Order: config.OrderConfig{
					VirtualScriptTimeoutMaxMs: tc.hostMaxMs,
				},
			})

			got := svc.resolveExecutionTimeoutMs(parseScriptDeliveryTimeoutMs(tc.configData))
			if got != tc.wantMs {
				t.Fatalf("expected timeout %dms, got %dms", tc.wantMs, got)
			}
		})
	}
}

func TestScriptDeliveryExecuteDeliveryScriptTimesOut(t *testing.T) {
	svc := NewScriptDeliveryService(nil, &config.Config{
		Order: config.OrderConfig{
			VirtualScriptTimeoutMaxMs: 150,
		},
	})

	inventory := &models.VirtualInventory{
		ID:           42,
		Script:       "function onDeliver(order, config) { while (true) {} }",
		ScriptConfig: `{"timeout_ms": 120}`,
	}
	order := &models.Order{
		ID:          7,
		OrderNo:     "ORD-TIMEOUT",
		Status:      models.OrderStatusPending,
		TotalAmount: 100,
		Currency:    "CNY",
		CreatedAt:   time.Now().UTC(),
	}

	_, err := svc.ExecuteDeliveryScript(inventory, order, 1)
	if err == nil {
		t.Fatalf("expected execution timeout error")
	}
	if !strings.Contains(err.Error(), "execution timeout") {
		t.Fatalf("expected execution timeout error, got %v", err)
	}
}

func TestScriptDeliveryHTTPRequestUsesExecutionDeadlineAndIgnoresBaseClientTimeout(t *testing.T) {
	var capturedDeadline time.Time

	svc := NewScriptDeliveryService(nil, &config.Config{
		Order: config.OrderConfig{
			VirtualScriptTimeoutMaxMs: 5000,
		},
	})
	svc.httpClientFactory = func() *http.Client {
		return &http.Client{
			Timeout: 10 * time.Millisecond,
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				deadline, ok := req.Context().Deadline()
				if !ok {
					t.Fatalf("expected request deadline to be attached")
				}
				capturedDeadline = deadline
				time.Sleep(25 * time.Millisecond)
				return &http.Response{
					StatusCode: http.StatusOK,
					Header: http.Header{
						"Content-Type": []string{"application/json"},
					},
					Body: io.NopCloser(strings.NewReader(`{"ok":true}`)),
				}, nil
			}),
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	vm := goja.New()
	startedAt := time.Now()
	resultValue := svc.doHTTPRequest(vm, ctx, http.MethodGet, "https://example.com/test", nil, map[string]string{})
	result, ok := resultValue.Export().(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %#v", resultValue.Export())
	}
	if status := fmt.Sprint(result["status"]); status != "200" {
		t.Fatalf("expected status 200, got %#v", result["status"])
	}
	if capturedDeadline.IsZero() {
		t.Fatalf("expected request deadline to be captured")
	}
	remaining := capturedDeadline.Sub(startedAt)
	if remaining <= 0 || remaining > 200*time.Millisecond {
		t.Fatalf("expected attached deadline near execution context, got %s", remaining)
	}
}

func TestScriptDeliveryHTTPRequestHonorsExecutionDeadline(t *testing.T) {
	svc := NewScriptDeliveryService(nil, &config.Config{
		Order: config.OrderConfig{
			VirtualScriptTimeoutMaxMs: 5000,
		},
	})
	svc.httpClientFactory = func() *http.Client {
		return &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				<-req.Context().Done()
				return nil, req.Context().Err()
			}),
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()

	vm := goja.New()
	resultValue := svc.doHTTPRequest(vm, ctx, http.MethodGet, "https://example.com/slow", nil, map[string]string{})
	result, ok := resultValue.Export().(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %#v", resultValue.Export())
	}
	if status := fmt.Sprint(result["status"]); status != "0" {
		t.Fatalf("expected status 0 on timeout, got %#v", result["status"])
	}
	if message := fmt.Sprint(result["error"]); !strings.Contains(message, "deadline exceeded") {
		t.Fatalf("expected deadline exceeded error, got %q", message)
	}
}

func TestParseDeliveryResultNormalizesInlineIframePresentation(t *testing.T) {
	svc := NewScriptDeliveryService(nil, &config.Config{})
	vm := goja.New()

	result, err := svc.parseDeliveryResult(vm.ToValue(map[string]interface{}{
		"success": true,
		"items": []interface{}{
			map[string]interface{}{
				"content": "ACCOUNT-READY",
				"remark":  "Open the panel below",
				"presentation": map[string]interface{}{
					"inlineIframe": map[string]interface{}{
						"title":       "Console",
						"height":      "520",
						"scope":       "user",
						"buttonLabel": "Open Panel",
						"src":         "/plugin-pages/demo?embedded=1",
					},
				},
			},
		},
	}), 1)
	if err != nil {
		t.Fatalf("parse delivery result: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result.Items))
	}

	panel := result.Items[0].Presentation
	if panel == nil || panel.InlineIframe == nil {
		t.Fatalf("expected inline iframe presentation, got %#v", panel)
	}
	if panel.InlineIframe.Height != 520 {
		t.Fatalf("expected height 520, got %d", panel.InlineIframe.Height)
	}
	if panel.InlineIframe.Scope != "user" {
		t.Fatalf("expected scope user, got %q", panel.InlineIframe.Scope)
	}
	if panel.InlineIframe.ButtonLabel != "Open Panel" {
		t.Fatalf("expected button label, got %q", panel.InlineIframe.ButtonLabel)
	}
}

func TestParseDeliveryResultDropsDangerousInlineIframeSrc(t *testing.T) {
	svc := NewScriptDeliveryService(nil, &config.Config{})
	vm := goja.New()

	result, err := svc.parseDeliveryResult(vm.ToValue(map[string]interface{}{
		"success": true,
		"items": []interface{}{
			map[string]interface{}{
				"content": "ACCOUNT-READY",
				"presentation": map[string]interface{}{
					"inline_iframe": map[string]interface{}{
						"src":    "javascript:alert(1)",
						"height": 100,
						"scope":  "unknown",
					},
				},
			},
		},
	}), 1)
	if err != nil {
		t.Fatalf("parse delivery result: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result.Items))
	}
	if result.Items[0].Presentation != nil {
		t.Fatalf("expected dangerous inline iframe src to be ignored, got %#v", result.Items[0].Presentation)
	}
}

func TestExecuteDeliveryScriptRecoversFromHostPanic(t *testing.T) {
	svc := NewScriptDeliveryService(nil, &config.Config{})

	userID := uint(7)
	inventory := &models.VirtualInventory{
		ID:     99,
		Script: "function onDeliver(order, config) { AuraLogic.order.getUser(); return { success: true, items: [{ content: 'OK' }] }; }",
	}
	order := &models.Order{
		ID:          1,
		OrderNo:     "ORDER-PANIC",
		UserID:      &userID,
		Status:      models.OrderStatusPendingPayment,
		TotalAmount: 100,
		Currency:    "USD",
		CreatedAt:   time.Now().UTC(),
	}

	result, err := svc.ExecuteDeliveryScript(inventory, order, 1)
	if err == nil {
		t.Fatalf("expected panic to be converted into error")
	}
	if result != nil {
		t.Fatalf("expected nil result on recovered panic, got %#v", result)
	}
	if !strings.Contains(err.Error(), "script delivery panic") {
		t.Fatalf("expected recovered panic error, got %v", err)
	}
}
