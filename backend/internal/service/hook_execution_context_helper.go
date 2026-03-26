package service

import (
	"fmt"
	"strings"
)

type hookBlockedError struct {
	reason string
}

func (e *hookBlockedError) Error() string {
	if e == nil {
		return "hook blocked"
	}
	reason := strings.TrimSpace(e.reason)
	if reason == "" {
		return "hook blocked"
	}
	return reason
}

func newHookBlockedError(reason string) error {
	return &hookBlockedError{reason: reason}
}

func isHookBlockedError(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*hookBlockedError)
	return ok
}

func IsHookBlockedError(err error) bool {
	return isHookBlockedError(err)
}

func buildServiceHookExecutionContext(userID *uint, orderID *uint, metadata map[string]string) *ExecutionContext {
	execCtx := &ExecutionContext{}
	if userID != nil {
		uid := *userID
		execCtx.UserID = &uid
	}
	if orderID != nil {
		oid := *orderID
		execCtx.OrderID = &oid
	}
	if len(metadata) > 0 {
		execCtx.Metadata = make(map[string]string, len(metadata))
		for key, value := range metadata {
			normalizedKey := strings.TrimSpace(key)
			if normalizedKey == "" {
				continue
			}
			execCtx.Metadata[normalizedKey] = value
		}
	}
	return execCtx
}

func cloneServiceHookExecutionContext(execCtx *ExecutionContext) *ExecutionContext {
	if execCtx == nil {
		return nil
	}

	cloned := &ExecutionContext{
		OperatorUserID: cloneOptionalExecutionUint(execCtx.OperatorUserID),
		SessionID:      execCtx.SessionID,
		RequestContext: execCtx.RequestContext,
		RequestCache:   execCtx.RequestCache,
	}
	if execCtx.UserID != nil {
		uid := *execCtx.UserID
		cloned.UserID = &uid
	}
	if execCtx.OrderID != nil {
		oid := *execCtx.OrderID
		cloned.OrderID = &oid
	}
	if len(execCtx.Metadata) > 0 {
		cloned.Metadata = make(map[string]string, len(execCtx.Metadata))
		for key, value := range execCtx.Metadata {
			cloned.Metadata[key] = value
		}
	}
	if execCtx.Webhook != nil {
		webhook := *execCtx.Webhook
		cloned.Webhook = &webhook
	}
	return cloned
}

func serviceHookValueToString(value interface{}) (string, error) {
	if value == nil {
		return "", nil
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("value must be string")
	}
	return text, nil
}
