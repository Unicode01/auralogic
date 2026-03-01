package service

import (
	"fmt"
	"log"
	"sync"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/models"
	"auralogic/internal/pkg/logger"
	"auralogic/internal/repository"
	"gorm.io/gorm"
)

const defaultAutoCancelHours = 72

// OrderCancelService 订单自动取消服务
type OrderCancelService struct {
	db                  *gorm.DB
	cfg                 *config.Config
	inventoryRepo       *repository.InventoryRepository
	promoCodeRepo       *repository.PromoCodeRepository
	virtualInventorySvc *VirtualInventoryService
	serialService       *SerialService
	stopChan            chan struct{}
	wg                  sync.WaitGroup
	checkInterval       time.Duration // 检查间隔
}

// NewOrderCancelService 创建订单自动取消服务
func NewOrderCancelService(
	db *gorm.DB,
	cfg *config.Config,
	inventoryRepo *repository.InventoryRepository,
	promoCodeRepo *repository.PromoCodeRepository,
	virtualInventorySvc *VirtualInventoryService,
	serialService *SerialService,
) *OrderCancelService {
	return &OrderCancelService{
		db:                  db,
		cfg:                 cfg,
		inventoryRepo:       inventoryRepo,
		promoCodeRepo:       promoCodeRepo,
		virtualInventorySvc: virtualInventorySvc,
		serialService:       serialService,
		stopChan:            make(chan struct{}),
		checkInterval:       5 * time.Minute, // 每5分钟检查一次
	}
}

// getAutoCancelHours 获取自动取消小时数，未配置时使用默认值
func (s *OrderCancelService) getAutoCancelHours() int {
	if h := s.cfg.Order.AutoCancelHours; h > 0 {
		return h
	}
	return defaultAutoCancelHours
}

// Start 启动自动取消服务
func (s *OrderCancelService) Start() {
	autoCancelHours := s.getAutoCancelHours()

	logger.LogSystemOperation(s.db, "order_cancel_service_start", "system", nil, map[string]interface{}{
		"auto_cancel_hours": autoCancelHours,
		"check_interval":    s.checkInterval.String(),
	})

	s.wg.Add(1)
	go s.cancelLoop()
}

// Stop 停止自动取消服务
func (s *OrderCancelService) Stop() {
	logger.LogSystemOperation(s.db, "order_cancel_service_stop", "system", nil, nil)
	close(s.stopChan)
	s.wg.Wait()
}

// cancelLoop 取消循环
func (s *OrderCancelService) cancelLoop() {
	defer s.wg.Done()

	// 启动时立即执行一次
	s.cancelExpiredOrders()

	ticker := time.NewTicker(s.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.cancelExpiredOrders()
		}
	}
}

// cancelExpiredOrders 取消过期订单
func (s *OrderCancelService) cancelExpiredOrders() {
	autoCancelHours := s.getAutoCancelHours()

	// 计算截止时间
	cutoffTime := time.Now().Add(-time.Duration(autoCancelHours) * time.Hour)

	// 分批查询需要取消的待付款订单，每次最多处理100条
	var orders []models.Order
	if err := s.db.Where("status = ? AND created_at < ?", models.OrderStatusPendingPayment, cutoffTime).
		Limit(100).Find(&orders).Error; err != nil {
		log.Printf("[OrderCancel] Error querying expired orders: %v", err)
		return
	}

	if len(orders) == 0 {
		return
	}

	cancelledCount := 0
	for _, order := range orders {
		if err := s.cancelOrder(&order, autoCancelHours); err != nil {
			log.Printf("[OrderCancel] Error cancelling order %s: %v", order.OrderNo, err)
			continue
		}
		cancelledCount++
	}

	if cancelledCount > 0 {
		logger.LogSystemOperation(s.db, "order_auto_cancel", "system", nil, map[string]interface{}{
			"cancelled_count":   cancelledCount,
			"auto_cancel_hours": autoCancelHours,
			"cutoff_time":       cutoffTime.Format(time.RFC3339),
		})
	}
}

// cancelOrder 取消单个订单
func (s *OrderCancelService) cancelOrder(order *models.Order, autoCancelHours int) error {
	// 先原子更新订单状态为已取消（WHERE status 条件防止并发重复处理）
	adminRemark := fmt.Sprintf("System auto-cancelled: order unpaid after %d hours", autoCancelHours)
	result := s.db.Model(order).
		Where("status = ?", models.OrderStatusPendingPayment).
		Updates(map[string]interface{}{
			"status":       models.OrderStatusCancelled,
			"admin_remark": adminRemark,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		// 订单状态已被其他流程修改，跳过
		return nil
	}

	// 状态已更新，开始释放资源（即使部分失败也不影响订单状态）

	// 释放物理商品库存
	for i := range order.Items {
		item := &order.Items[i]
		if inventoryID, exists := order.InventoryBindings[i]; exists && inventoryID > 0 {
			if err := s.inventoryRepo.ReleaseReserve(inventoryID, item.Quantity, order.OrderNo); err != nil {
				log.Printf("[OrderCancel] Order %s failed to release inventory %d: %v", order.OrderNo, inventoryID, err)
			}
		}
	}

	// 释放虚拟商品库存
	if s.virtualInventorySvc != nil {
		if err := s.virtualInventorySvc.ReleaseStock(order.OrderNo); err != nil {
			log.Printf("[OrderCancel] Order %s failed to release virtual stock: %v", order.OrderNo, err)
		}
	}

	// 释放优惠码
	if order.PromoCodeID != nil && s.promoCodeRepo != nil {
		if err := s.promoCodeRepo.ReleaseReserve(*order.PromoCodeID, order.OrderNo); err != nil {
			log.Printf("[OrderCancel] Order %s failed to release promo code: %v", order.OrderNo, err)
		}
	}

	// 删除关联的序列号
	if s.serialService != nil {
		if err := s.serialService.DeleteSerialsByOrderID(order.ID); err != nil {
			log.Printf("[OrderCancel] Order %s failed to delete serials: %v", order.OrderNo, err)
		}
	}

	logger.LogPaymentOperation(s.db, "order_auto_cancelled", order.ID, map[string]interface{}{
		"order_no":   order.OrderNo,
		"created_at": order.CreatedAt.Format(time.RFC3339),
		"reason":     "pending_payment_timeout",
	})

	return nil
}
