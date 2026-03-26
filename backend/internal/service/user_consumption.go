package service

import (
	"fmt"

	"auralogic/internal/models"
	"auralogic/internal/repository"
)

// userConsumptionStatuses 定义“计入消费统计”的订单状态。
// 约定：除待付款/已取消/已退款外，均视为已形成消费。
var userConsumptionStatuses = []models.OrderStatus{
	models.OrderStatusDraft,
	models.OrderStatusNeedResubmit,
	models.OrderStatusPending,
	models.OrderStatusShipped,
	models.OrderStatusCompleted,
}

var userConsumptionStatusSet = func() map[models.OrderStatus]struct{} {
	set := make(map[models.OrderStatus]struct{}, len(userConsumptionStatuses))
	for _, status := range userConsumptionStatuses {
		set[status] = struct{}{}
	}
	return set
}()

func isUserConsumptionStatusTracked(status models.OrderStatus) bool {
	_, ok := userConsumptionStatusSet[status]
	return ok
}

func buildUserConsumptionStatsDelta(before models.OrderStatus, after models.OrderStatus, totalAmountMinor int64) (int64, int64) {
	beforeTracked := isUserConsumptionStatusTracked(before)
	afterTracked := isUserConsumptionStatusTracked(after)

	switch {
	case !beforeTracked && afterTracked:
		return totalAmountMinor, 1
	case beforeTracked && !afterTracked:
		return -totalAmountMinor, -1
	default:
		return 0, 0
	}
}

func applyUserConsumptionStatsDelta(userRepo *repository.UserRepository, userID *uint, totalSpentMinorDelta int64, totalOrderCountDelta int64) error {
	if userRepo == nil || userID == nil || *userID == 0 {
		return nil
	}
	return userRepo.ApplyConsumptionStatsDelta(*userID, totalSpentMinorDelta, totalOrderCountDelta)
}

func userIDValue(userID *uint) uint {
	if userID == nil {
		return 0
	}
	return *userID
}

// SyncUserConsumptionStats 重新计算并写回用户消费统计（消费金额 + 订单数）。
func (s *OrderService) SyncUserConsumptionStats(userID uint) error {
	if userID == 0 {
		return nil
	}
	if s == nil || s.OrderRepo == nil || s.userRepo == nil {
		return nil
	}

	orderCount, totalSpentMinor, err := s.OrderRepo.GetUserConsumptionSummary(userID, userConsumptionStatuses)
	if err != nil {
		return err
	}

	return s.userRepo.UpdateConsumptionStats(userID, totalSpentMinor, orderCount)
}

func (s *OrderService) syncUserConsumptionStatsBestEffort(userID *uint, scene string) {
	if userID == nil || *userID == 0 {
		return
	}
	if err := s.SyncUserConsumptionStats(*userID); err != nil {
		fmt.Printf("Warning: failed to sync user consumption stats (scene=%s, user_id=%d): %v\n", scene, *userID, err)
	}
}

func (s *OrderService) syncUserConsumptionStatusTransitionBestEffort(
	userID *uint,
	before models.OrderStatus,
	after models.OrderStatus,
	totalAmountMinor int64,
	scene string,
) {
	totalSpentMinorDelta, totalOrderCountDelta := buildUserConsumptionStatsDelta(before, after, totalAmountMinor)
	if totalSpentMinorDelta == 0 && totalOrderCountDelta == 0 {
		return
	}
	if err := applyUserConsumptionStatsDelta(s.userRepo, userID, totalSpentMinorDelta, totalOrderCountDelta); err != nil {
		fmt.Printf(
			"Warning: failed to apply user consumption stats delta (scene=%s, user_id=%d, spent_delta=%d, order_delta=%d): %v\n",
			scene,
			*userID,
			totalSpentMinorDelta,
			totalOrderCountDelta,
			err,
		)
	}
}
