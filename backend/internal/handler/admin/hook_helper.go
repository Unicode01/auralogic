package admin

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
)

func buildAdminHookExecutionContext(
	c *gin.Context,
	adminID *uint,
	extra map[string]string,
) *service.ExecutionContext {
	if c == nil {
		return nil
	}

	metadata := map[string]string{
		"request_path":    c.Request.URL.Path,
		"route":           c.FullPath(),
		"method":          c.Request.Method,
		"client_ip":       c.ClientIP(),
		"user_agent":      c.GetHeader("User-Agent"),
		"accept_language": c.GetHeader("Accept-Language"),
		"operator_type":   "admin",
	}
	for key, value := range extra {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "" {
			continue
		}
		metadata[normalizedKey] = value
	}

	execCtx := &service.ExecutionContext{
		OperatorUserID: getOptionalOperatorUserID(c),
		SessionID:      strings.TrimSpace(c.GetHeader("X-Session-ID")),
		Metadata:       metadata,
		RequestContext: c.Request.Context(),
	}
	if adminID != nil {
		uid := *adminID
		execCtx.UserID = &uid
	}
	return execCtx
}

func cloneAdminHookExecutionContext(execCtx *service.ExecutionContext) *service.ExecutionContext {
	if execCtx == nil {
		return nil
	}

	cloned := &service.ExecutionContext{
		OperatorUserID: cloneOptionalUint(execCtx.OperatorUserID),
		SessionID:      execCtx.SessionID,
		// This helper is used for async after-hooks. Detach from the original HTTP
		// request so the hook is not canceled as soon as the response is written.
		RequestContext: context.Background(),
	}
	if execCtx.UserID != nil {
		uid := *execCtx.UserID
		cloned.UserID = &uid
	}
	if execCtx.OrderID != nil {
		orderID := *execCtx.OrderID
		cloned.OrderID = &orderID
	}
	if len(execCtx.Metadata) > 0 {
		cloned.Metadata = make(map[string]string, len(execCtx.Metadata))
		for key, value := range execCtx.Metadata {
			cloned.Metadata[key] = value
		}
	}
	return cloned
}

func mergeAdminHookStructPatch(target interface{}, patch map[string]interface{}) error {
	if target == nil || len(patch) == 0 {
		return nil
	}

	baseJSON, err := json.Marshal(target)
	if err != nil {
		return err
	}

	var base map[string]interface{}
	if err := json.Unmarshal(baseJSON, &base); err != nil {
		return err
	}
	for key, value := range patch {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "" {
			continue
		}
		base[normalizedKey] = value
	}

	mergedJSON, err := json.Marshal(base)
	if err != nil {
		return err
	}
	return json.Unmarshal(mergedJSON, target)
}

func adminHookStructToPayload(value interface{}) (map[string]interface{}, error) {
	if value == nil {
		return map[string]interface{}{}, nil
	}

	body, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	payload := make(map[string]interface{})
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func adminHookValueToString(value interface{}) (string, error) {
	if value == nil {
		return "", nil
	}
	text, ok := value.(string)
	if !ok {
		return "", errors.New("value must be string")
	}
	return text, nil
}
