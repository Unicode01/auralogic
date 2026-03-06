package service

import (
	"fmt"

	"auralogic/internal/models"
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

// SyncUserConsumptionStats 重新计算并写回用户消费统计（消费金额 + 订单数）。
func (s *OrderService) SyncUserConsumptionStats(userID uint) error {
	if userID == 0 {
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
