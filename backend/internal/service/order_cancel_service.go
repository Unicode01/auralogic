package service

import (
	"fmt"
	"log"
	"strconv"
	"strings"
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
	pluginManager       *PluginManagerService
	lifecycleMu         sync.Mutex
	running             bool
	stopChan            chan struct{}
	doneChan            chan struct{}
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
		checkInterval:       5 * time.Minute, // 每5分钟检查一次
	}
}

func (s *OrderCancelService) SetPluginManager(pluginManager *PluginManagerService) {
	s.pluginManager = pluginManager
}

func cloneOrderCancelExecutionContext(execCtx *ExecutionContext) *ExecutionContext {
	if execCtx == nil {
		return nil
	}
	cloned := &ExecutionContext{
		SessionID:      execCtx.SessionID,
		RequestContext: execCtx.RequestContext,
	}
	if execCtx.UserID != nil {
		userID := *execCtx.UserID
		cloned.UserID = &userID
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

func orderCancelHookValueToOptionalString(value interface{}) (string, error) {
	if value == nil {
		return "", nil
	}
	str, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("value must be string")
	}
	return str, nil
}

func (s *OrderCancelService) buildInventoryHookExecutionContext(order *models.Order) *ExecutionContext {
	if order == nil {
		return nil
	}
	orderID := order.ID
	metadata := map[string]string{
		"source":   "order_auto_cancel",
		"order_no": order.OrderNo,
	}
	var userID *uint
	if order.UserID != nil {
		uid := *order.UserID
		userID = &uid
		metadata["user_id"] = strconv.FormatUint(uint64(uid), 10)
	}
	return &ExecutionContext{
		UserID:   userID,
		OrderID:  &orderID,
		Metadata: metadata,
	}
}

func (s *OrderCancelService) releaseReservedInventoryWithHook(order *models.Order, inventoryID uint, quantity int) error {
	releaseErr := s.inventoryRepo.ReleaseReserve(inventoryID, quantity, order.OrderNo)
	if s.pluginManager != nil {
		payload := map[string]interface{}{
			"order_id":     order.ID,
			"order_no":     order.OrderNo,
			"user_id":      order.UserID,
			"inventory_id": inventoryID,
			"quantity":     quantity,
			"success":      releaseErr == nil,
			"source":       "order_auto_cancel",
		}
		if releaseErr != nil {
			payload["error"] = releaseErr.Error()
		}
		go func(execCtx *ExecutionContext, hookPayload map[string]interface{}, orderNo string, inventory uint) {
			_, hookErr := s.pluginManager.ExecuteHook(HookExecutionRequest{
				Hook:    "inventory.release.after",
				Payload: hookPayload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("inventory.release.after hook execution failed: order=%s inventory=%d err=%v", orderNo, inventory, hookErr)
			}
		}(cloneOrderCancelExecutionContext(s.buildInventoryHookExecutionContext(order)), payload, order.OrderNo, inventoryID)
	}
	return releaseErr
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
	s.lifecycleMu.Lock()
	if s.running {
		s.lifecycleMu.Unlock()
		return
	}
	stopChan := make(chan struct{})
	doneChan := make(chan struct{})
	s.stopChan = stopChan
	s.doneChan = doneChan
	s.running = true
	s.lifecycleMu.Unlock()

	autoCancelHours := s.getAutoCancelHours()

	logger.LogSystemOperation(s.db, "order_cancel_service_start", "system", nil, map[string]interface{}{
		"auto_cancel_hours": autoCancelHours,
		"check_interval":    s.checkInterval.String(),
	})

	go s.cancelLoop(stopChan, doneChan)
}

// Stop 停止自动取消服务
func (s *OrderCancelService) Stop() {
	s.lifecycleMu.Lock()
	if !s.running {
		s.lifecycleMu.Unlock()
		return
	}
	stopChan := s.stopChan
	doneChan := s.doneChan
	s.stopChan = nil
	s.doneChan = nil
	s.running = false

	logger.LogSystemOperation(s.db, "order_cancel_service_stop", "system", nil, nil)
	close(stopChan)
	<-doneChan
	s.lifecycleMu.Unlock()
}

// cancelLoop 取消循环
func (s *OrderCancelService) cancelLoop(stopChan <-chan struct{}, doneChan chan struct{}) {
	defer close(doneChan)

	// 启动时立即执行一次
	s.cancelExpiredOrders()

	ticker := time.NewTicker(s.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stopChan:
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
		cancelled, err := s.cancelOrder(&order, autoCancelHours)
		if err != nil {
			log.Printf("[OrderCancel] Error cancelling order %s: %v", order.OrderNo, err)
			continue
		}
		if cancelled {
			cancelledCount++
		}
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
func (s *OrderCancelService) cancelOrder(order *models.Order, autoCancelHours int) (bool, error) {
	if order == nil {
		return false, fmt.Errorf("order is nil")
	}

	beforeStatus := order.Status
	// 先原子更新订单状态为已取消（WHERE status 条件防止并发重复处理）
	adminRemark := fmt.Sprintf("System auto-cancelled: order unpaid after %d hours", autoCancelHours)
	hookExecCtx := cloneOrderCancelExecutionContext(s.buildInventoryHookExecutionContext(order))
	if s.pluginManager != nil {
		hookPayload := map[string]interface{}{
			"order_id":          order.ID,
			"order_no":          order.OrderNo,
			"user_id":           order.UserID,
			"status_before":     beforeStatus,
			"auto_cancel_hours": autoCancelHours,
			"admin_remark":      adminRemark,
			"source":            "order_auto_cancel",
		}
		hookResult, hookErr := s.pluginManager.ExecuteHook(HookExecutionRequest{
			Hook:    "order.auto_cancel.before",
			Payload: hookPayload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("order.auto_cancel.before hook execution failed: order=%s err=%v", order.OrderNo, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "order auto-cancel blocked by plugin"
				}
				log.Printf("[OrderCancel] Skip auto-cancel order %s: %s", order.OrderNo, reason)
				return false, nil
			}
			if hookResult.Payload != nil {
				if rawRemark, exists := hookResult.Payload["admin_remark"]; exists {
					remark, convErr := orderCancelHookValueToOptionalString(rawRemark)
					if convErr != nil {
						log.Printf("order.auto_cancel.before payload admin_remark decode failed, fallback to default: order=%s err=%v", order.OrderNo, convErr)
					} else {
						adminRemark = strings.TrimSpace(remark)
					}
				} else if rawReason, exists := hookResult.Payload["reason"]; exists {
					reason, convErr := orderCancelHookValueToOptionalString(rawReason)
					if convErr != nil {
						log.Printf("order.auto_cancel.before payload reason decode failed, fallback to default: order=%s err=%v", order.OrderNo, convErr)
					} else {
						adminRemark = strings.TrimSpace(reason)
					}
				}
				if adminRemark == "" {
					adminRemark = fmt.Sprintf("System auto-cancelled: order unpaid after %d hours", autoCancelHours)
				}
			}
		}
	}

	result := s.db.Model(order).
		Where("status = ?", models.OrderStatusPendingPayment).
		Updates(map[string]interface{}{
			"status":       models.OrderStatusCancelled,
			"admin_remark": adminRemark,
		})
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 0 {
		// 订单状态已被其他流程修改，跳过
		return false, nil
	}

	syncUserPurchaseStatsTransitionBestEffort(
		repository.NewOrderRepository(s.db),
		order.UserID,
		order.UserID,
		beforeStatus,
		models.OrderStatusCancelled,
		order.Items,
		"auto_cancel_order",
	)

	// 状态已更新，开始释放资源（即使部分失败也不影响订单状态）

	// 释放物理商品库存
	for i := range order.Items {
		item := &order.Items[i]
		if inventoryID, exists := order.InventoryBindings[i]; exists && inventoryID > 0 {
			if err := s.releaseReservedInventoryWithHook(order, inventoryID, item.Quantity); err != nil {
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
	EmitOrderStatusChangedAfterHookAsync(s.pluginManager, hookExecCtx, order, beforeStatus, models.OrderStatusCancelled, map[string]interface{}{
		"source":            "order_auto_cancel",
		"trigger_action":    "order.auto_cancel",
		"auto_cancel_hours": autoCancelHours,
		"admin_remark":      adminRemark,
	})

	if s.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"order_id":          order.ID,
			"order_no":          order.OrderNo,
			"user_id":           order.UserID,
			"status_before":     beforeStatus,
			"status_after":      models.OrderStatusCancelled,
			"auto_cancel_hours": autoCancelHours,
			"admin_remark":      adminRemark,
			"source":            "order_auto_cancel",
		}
		go func(execCtx *ExecutionContext, payload map[string]interface{}, orderNo string) {
			_, hookErr := s.pluginManager.ExecuteHook(HookExecutionRequest{
				Hook:    "order.auto_cancel.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("order.auto_cancel.after hook execution failed: order=%s err=%v", orderNo, hookErr)
			}
		}(hookExecCtx, afterPayload, order.OrderNo)
	}

	return true, nil
}
