package service

import (
	"fmt"
	"strings"

	"auralogic/internal/models"
	"auralogic/internal/repository"
	"gorm.io/gorm"
)

// userPurchaseLimitStatuses defines order statuses that currently count toward per-account purchase limits.
// Keep this aligned with legacy behavior: all non-cancelled orders count, including refunded orders.
var userPurchaseLimitStatuses = []models.OrderStatus{
	models.OrderStatusPendingPayment,
	models.OrderStatusDraft,
	models.OrderStatusNeedResubmit,
	models.OrderStatusPending,
	models.OrderStatusShipped,
	models.OrderStatusCompleted,
	models.OrderStatusRefundPending,
	models.OrderStatusRefunded,
}

var userPurchaseLimitStatusSet = func() map[models.OrderStatus]struct{} {
	set := make(map[models.OrderStatus]struct{}, len(userPurchaseLimitStatuses))
	for _, status := range userPurchaseLimitStatuses {
		set[status] = struct{}{}
	}
	return set
}()

func isUserPurchaseLimitStatusTracked(status models.OrderStatus) bool {
	_, ok := userPurchaseLimitStatusSet[status]
	return ok
}

func buildOrderItemQuantityBySKU(items []models.OrderItem) map[string]int64 {
	quantities := make(map[string]int64)
	for _, item := range items {
		sku := strings.TrimSpace(item.SKU)
		if sku == "" || item.Quantity <= 0 {
			continue
		}
		quantities[sku] += int64(item.Quantity)
	}
	return quantities
}

func mergeUserPurchaseStatsDelta(target map[uint]map[string]int64, userID uint, factor int64, quantities map[string]int64) {
	if userID == 0 || factor == 0 || len(quantities) == 0 {
		return
	}

	userDelta, exists := target[userID]
	if !exists {
		userDelta = make(map[string]int64, len(quantities))
		target[userID] = userDelta
	}

	for sku, quantity := range quantities {
		if quantity == 0 {
			continue
		}
		userDelta[sku] += factor * quantity
		if userDelta[sku] == 0 {
			delete(userDelta, sku)
		}
	}

	if len(userDelta) == 0 {
		delete(target, userID)
	}
}

func buildUserPurchaseStatsDeltas(
	beforeUserID *uint,
	afterUserID *uint,
	beforeStatus models.OrderStatus,
	afterStatus models.OrderStatus,
	items []models.OrderItem,
) map[uint]map[string]int64 {
	quantities := buildOrderItemQuantityBySKU(items)
	if len(quantities) == 0 {
		return nil
	}

	deltas := make(map[uint]map[string]int64)
	if beforeUserID != nil && *beforeUserID > 0 && isUserPurchaseLimitStatusTracked(beforeStatus) {
		mergeUserPurchaseStatsDelta(deltas, *beforeUserID, -1, quantities)
	}
	if afterUserID != nil && *afterUserID > 0 && isUserPurchaseLimitStatusTracked(afterStatus) {
		mergeUserPurchaseStatsDelta(deltas, *afterUserID, 1, quantities)
	}
	return deltas
}

func syncUserPurchaseStats(orderRepo *repository.OrderRepository, userID uint) error {
	if orderRepo == nil || userID == 0 {
		return nil
	}

	quantities, err := orderRepo.GetUserPurchaseQuantitySummaryFromOrders(userID)
	if err != nil {
		return err
	}
	return orderRepo.ReplaceUserPurchaseStats(userID, quantities)
}

func syncUserPurchaseStatsBestEffort(orderRepo *repository.OrderRepository, userID *uint, scene string) {
	if orderRepo == nil || userID == nil || *userID == 0 {
		return
	}
	if err := syncUserPurchaseStats(orderRepo, *userID); err != nil {
		fmt.Printf("Warning: failed to sync user purchase stats (scene=%s, user_id=%d): %v\n", scene, *userID, err)
	}
}

func syncUserPurchaseStatsTransitionBestEffort(
	orderRepo *repository.OrderRepository,
	beforeUserID *uint,
	afterUserID *uint,
	beforeStatus models.OrderStatus,
	afterStatus models.OrderStatus,
	items []models.OrderItem,
	scene string,
) {
	if orderRepo == nil {
		return
	}

	deltas := buildUserPurchaseStatsDeltas(beforeUserID, afterUserID, beforeStatus, afterStatus, items)
	for userID, deltaBySKU := range deltas {
		if err := orderRepo.ApplyUserPurchaseStatsDelta(userID, deltaBySKU); err != nil {
			fmt.Printf("Warning: failed to apply user purchase stats delta (scene=%s, user_id=%d): %v\n", scene, userID, err)
			syncUserPurchaseStatsBestEffort(orderRepo, &userID, scene+"_resync")
		}
	}
}

func applyUserPurchaseStatsTransitionTx(
	tx *gorm.DB,
	beforeUserID *uint,
	afterUserID *uint,
	beforeStatus models.OrderStatus,
	afterStatus models.OrderStatus,
	items []models.OrderItem,
) error {
	if tx == nil {
		return fmt.Errorf("transaction is required")
	}

	orderRepo := repository.NewOrderRepository(tx)
	deltas := buildUserPurchaseStatsDeltas(beforeUserID, afterUserID, beforeStatus, afterStatus, items)
	for userID, deltaBySKU := range deltas {
		if err := orderRepo.ApplyUserPurchaseStatsDelta(userID, deltaBySKU); err != nil {
			return err
		}
	}
	return nil
}
