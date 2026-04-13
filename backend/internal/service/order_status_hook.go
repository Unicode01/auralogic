package service

import (
	"log"
	"time"

	"auralogic/internal/models"
)

func buildOrderStatusChangedHookPayload(
	order *models.Order,
	beforeStatus models.OrderStatus,
	afterStatus models.OrderStatus,
	extra map[string]interface{},
) map[string]interface{} {
	if order == nil {
		return map[string]interface{}{}
	}

	payload := map[string]interface{}{
		"order_id":           order.ID,
		"order_no":           order.OrderNo,
		"user_id":            order.UserID,
		"status":             afterStatus,
		"status_before":      beforeStatus,
		"status_after":       afterStatus,
		"total_amount_minor": order.TotalAmount,
		"currency":           order.Currency,
		"promo_code":         order.PromoCodeStr,
		"changed_at":         models.NowFunc().UTC().Format(time.RFC3339),
	}
	for key, value := range extra {
		payload[key] = value
	}
	return payload
}

func EmitOrderStatusChangedAfterHookAsync(
	pluginManager *PluginManagerService,
	execCtx *ExecutionContext,
	order *models.Order,
	beforeStatus models.OrderStatus,
	afterStatus models.OrderStatus,
	extra map[string]interface{},
) {
	if pluginManager == nil || order == nil || beforeStatus == afterStatus {
		return
	}

	clonedExecCtx := cloneServiceHookExecutionContext(execCtx)
	if clonedExecCtx == nil {
		clonedExecCtx = &ExecutionContext{}
	}
	if clonedExecCtx.OrderID == nil {
		orderID := order.ID
		clonedExecCtx.OrderID = &orderID
	}
	if clonedExecCtx.UserID == nil && order.UserID != nil {
		userID := *order.UserID
		clonedExecCtx.UserID = &userID
	}

	payload := buildOrderStatusChangedHookPayload(order, beforeStatus, afterStatus, extra)
	go func(execCtx *ExecutionContext, hookPayload map[string]interface{}, orderNo string) {
		_, hookErr := pluginManager.ExecuteHook(HookExecutionRequest{
			Hook:    "order.status.changed.after",
			Payload: hookPayload,
		}, execCtx)
		if hookErr != nil {
			log.Printf("order.status.changed.after hook execution failed: order=%s err=%v", orderNo, hookErr)
		}
	}(clonedExecCtx, payload, order.OrderNo)
}
