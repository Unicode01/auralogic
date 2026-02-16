package service

import (
	"fmt"
	"sync"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/models"
	"auralogic/internal/pkg/logger"
	"auralogic/internal/repository"
	"gorm.io/gorm"
)

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

// Start 启动自动取消服务
func (s *OrderCancelService) Start() {
	autoCancelHours := s.cfg.Order.AutoCancelHours
	if autoCancelHours <= 0 {
		autoCancelHours = 72 // 默认72小时
	}

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
	// 获取最新的配置
	autoCancelHours := s.cfg.Order.AutoCancelHours
	if autoCancelHours <= 0 {
		return // 禁用自动取消
	}

	// 计算截止时间
	cutoffTime := time.Now().Add(-time.Duration(autoCancelHours) * time.Hour)

	// 查询需要取消的待付款订单
	var orders []models.Order
	if err := s.db.Where("status = ? AND created_at < ?", models.OrderStatusPendingPayment, cutoffTime).Find(&orders).Error; err != nil {
		fmt.Printf("Error querying expired orders: %v\n", err)
		return
	}

	if len(orders) == 0 {
		return
	}

	cancelledCount := 0
	for _, order := range orders {
		if err := s.cancelOrder(&order); err != nil {
			fmt.Printf("Error cancelling order %s: %v\n", order.OrderNo, err)
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
func (s *OrderCancelService) cancelOrder(order *models.Order) error {
	// 释放物理商品库存
	for i := range order.Items {
		item := &order.Items[i]
		if inventoryID, exists := order.InventoryBindings[i]; exists && inventoryID > 0 {
			if err := s.inventoryRepo.ReleaseReserve(inventoryID, item.Quantity, order.OrderNo); err != nil {
				fmt.Printf("Warning: Order %s failed to release reserved inventory: %v\n", order.OrderNo, err)
			}
		}
	}

	// 释放虚拟商品库存
	if s.virtualInventorySvc != nil {
		if err := s.virtualInventorySvc.ReleaseStock(order.OrderNo); err != nil {
			fmt.Printf("Warning: Order %s failed to release virtual product stock: %v\n", order.OrderNo, err)
		}
	}

	// 释放优惠码
	if order.PromoCodeID != nil && s.promoCodeRepo != nil {
		if err := s.promoCodeRepo.ReleaseReserve(*order.PromoCodeID, order.OrderNo); err != nil {
			fmt.Printf("Warning: Order %s failed to release promo code: %v\n", order.OrderNo, err)
		}
	}

	// 删除关联的序列号
	if s.serialService != nil {
		if err := s.serialService.DeleteSerialsByOrderID(order.ID); err != nil {
			fmt.Printf("Warning: Order %s failed to delete serial numbers: %v\n", order.OrderNo, err)
		}
	}

	// 更新订单状态
	order.Status = models.OrderStatusCancelled
	order.AdminRemark = fmt.Sprintf("系统自动取消：订单超过 %d 小时未付款", s.cfg.Order.AutoCancelHours)

	if err := s.db.Save(order).Error; err != nil {
		return err
	}

	logger.LogPaymentOperation(s.db, "order_auto_cancelled", order.ID, map[string]interface{}{
		"order_no":   order.OrderNo,
		"created_at": order.CreatedAt.Format(time.RFC3339),
		"reason":     "pending_payment_timeout",
	})

	return nil
}
