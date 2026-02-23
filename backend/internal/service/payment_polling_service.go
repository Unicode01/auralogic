package service

import (
	"container/heap"
	"encoding/json"
	"sync"
	"time"

	"auralogic/internal/models"
	"auralogic/internal/pkg/logger"
	"gorm.io/gorm"
)

// PollingTask 轮询任务
type PollingTask struct {
	OrderID         uint      `json:"order_id"`
	PaymentMethodID uint      `json:"payment_method_id"`
	AddedAt         time.Time `json:"added_at"`
	NextCheckAt     time.Time `json:"next_check_at"`    // 下次检查时间
	CheckInterval   int       `json:"check_interval"`   // 检查间隔(秒)
	RetryCount      int       `json:"retry_count"`
	index           int       // 堆中的索引
}

// TaskHeap 任务优先队列（最小堆，按 NextCheckAt 排序）
type TaskHeap []*PollingTask

func (h TaskHeap) Len() int           { return len(h) }
func (h TaskHeap) Less(i, j int) bool { return h[i].NextCheckAt.Before(h[j].NextCheckAt) }
func (h TaskHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *TaskHeap) Push(x interface{}) {
	n := len(*h)
	item := x.(*PollingTask)
	item.index = n
	*h = append(*h, item)
}

func (h *TaskHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*h = old[0 : n-1]
	return item
}

// PaymentPollingService 付款状态轮询服务（时间轮实现）
type PaymentPollingService struct {
	db                  *gorm.DB
	jsRuntime           *JSRuntimeService
	virtualInventorySvc *VirtualInventoryService
	emailService        *EmailService
	taskHeap            TaskHeap              // 时间轮：按下次检查时间排序的最小堆
	taskMap             map[uint]*PollingTask // orderID -> task 快速查找
	mutex               sync.Mutex
	stopChan            chan struct{}
	wakeupChan          chan struct{} // 用于唤醒主循环
	defaultInterval     int           // 默认检查间隔(秒)
	maxRetries          int           // 最大重试次数
	maxDuration         time.Duration // 最大轮询时长
}

// NewPaymentPollingService 创建付款轮询服务
func NewPaymentPollingService(db *gorm.DB, virtualInventorySvc *VirtualInventoryService, emailService *EmailService) *PaymentPollingService {
	return &PaymentPollingService{
		db:                  db,
		jsRuntime:           NewJSRuntimeService(db),
		virtualInventorySvc: virtualInventorySvc,
		emailService:        emailService,
		taskHeap:            make(TaskHeap, 0),
		taskMap:             make(map[uint]*PollingTask),
		stopChan:            make(chan struct{}),
		wakeupChan:          make(chan struct{}, 1),
		defaultInterval:     30,                // 默认30秒
		maxRetries:          480,               // 最多重试480次
		maxDuration:         4 * time.Hour,     // 最长轮询4小时
	}
}

// Start 启动轮询服务
func (s *PaymentPollingService) Start() {
	logger.LogSystemOperation(s.db, "payment_polling_start", "system", nil, map[string]interface{}{
		"default_interval": s.defaultInterval,
		"max_retries":      s.maxRetries,
		"max_duration":     s.maxDuration.String(),
		"algorithm":        "time_wheel",
	})
	// 从数据库恢复未完成的轮询任务
	s.recoverTasks()
	go s.timeWheelLoop()
}

// Stop 停止轮询服务
func (s *PaymentPollingService) Stop() {
	logger.LogSystemOperation(s.db, "payment_polling_stop", "system", nil, nil)
	close(s.stopChan)
}

// AddToQueue 添加订单到轮询队列
func (s *PaymentPollingService) AddToQueue(orderID, paymentMethodID uint) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 检查是否已在队列中
	if _, exists := s.taskMap[orderID]; exists {
		return
	}

	// 获取付款方式的轮询间隔
	interval := s.defaultInterval
	var pm models.PaymentMethod
	if err := s.db.First(&pm, paymentMethodID).Error; err == nil && pm.PollInterval > 0 {
		interval = pm.PollInterval
	}

	now := time.Now()
	task := &PollingTask{
		OrderID:         orderID,
		PaymentMethodID: paymentMethodID,
		AddedAt:         now,
		NextCheckAt:     now, // 立即检查一次
		CheckInterval:   interval,
		RetryCount:      0,
	}

	s.taskMap[orderID] = task
	heap.Push(&s.taskHeap, task)

	// 保存到数据库（用于服务重启后恢复）
	s.saveTaskToDB(task)

	logger.LogPaymentOperation(s.db, "payment_polling_add", orderID, map[string]interface{}{
		"payment_method_id": paymentMethodID,
		"check_interval":    interval,
	})

	// 唤醒主循环
	s.wakeup()
}

// RemoveFromQueue 从队列中移除订单
func (s *PaymentPollingService) RemoveFromQueue(orderID uint) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.removeFromQueueLocked(orderID)
}

// removeFromQueueLocked 从队列中移除订单（需要持有锁）
func (s *PaymentPollingService) removeFromQueueLocked(orderID uint) {
	task, exists := s.taskMap[orderID]
	if !exists {
		return
	}

	delete(s.taskMap, orderID)
	if task.index >= 0 && task.index < len(s.taskHeap) {
		heap.Remove(&s.taskHeap, task.index)
	}
	s.removeTaskFromDB(orderID)
}

// GetQueueStatus 获取队列状态
func (s *PaymentPollingService) GetQueueStatus() []PollingTask {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	tasks := make([]PollingTask, 0, len(s.taskMap))
	for _, task := range s.taskMap {
		tasks = append(tasks, *task)
	}
	return tasks
}

// wakeup 唤醒主循环
func (s *PaymentPollingService) wakeup() {
	select {
	case s.wakeupChan <- struct{}{}:
	default:
	}
}

// timeWheelLoop 时间轮主循环
func (s *PaymentPollingService) timeWheelLoop() {
	for {
		s.mutex.Lock()

		// 计算下次需要唤醒的时间
		var sleepDuration time.Duration
		if len(s.taskHeap) == 0 {
			sleepDuration = time.Minute // 没有任务时，每分钟检查一次
		} else {
			nextTask := s.taskHeap[0]
			sleepDuration = time.Until(nextTask.NextCheckAt)
			if sleepDuration < 0 {
				sleepDuration = 0
			}
		}
		s.mutex.Unlock()

		// 等待直到下次检查时间或被唤醒
		timer := time.NewTimer(sleepDuration)
		select {
		case <-s.stopChan:
			timer.Stop()
			return
		case <-s.wakeupChan:
			timer.Stop()
			// 被唤醒，重新计算
			continue
		case <-timer.C:
			// 到达检查时间
		}

		// 处理到期的任务
		s.processDueTasks()
	}
}

// processDueTasks 处理到期的任务
func (s *PaymentPollingService) processDueTasks() {
	now := time.Now()

	for {
		s.mutex.Lock()

		// 检查是否有到期的任务
		if len(s.taskHeap) == 0 {
			s.mutex.Unlock()
			return
		}

		nextTask := s.taskHeap[0]
		if nextTask.NextCheckAt.After(now) {
			s.mutex.Unlock()
			return
		}

		// 取出任务
		task := heap.Pop(&s.taskHeap).(*PollingTask)
		s.mutex.Unlock()

		// 检查付款状态
		shouldContinue, newInterval := s.checkPaymentStatus(task)

		if shouldContinue {
			s.mutex.Lock()
			// 检查任务是否还在 map 中（可能被其他操作移除）
			if _, exists := s.taskMap[task.OrderID]; exists {
				// 更新检查间隔（如果有变化）
				if newInterval > 0 {
					task.CheckInterval = newInterval
				}
				// 计算下次检查时间
				task.NextCheckAt = time.Now().Add(time.Duration(task.CheckInterval) * time.Second)
				task.RetryCount++
				// 重新加入堆
				heap.Push(&s.taskHeap, task)
				// 更新数据库
				s.saveTaskToDB(task)
			}
			s.mutex.Unlock()
		}
	}
}

// checkPaymentStatus 检查付款状态
// 返回: shouldContinue 是否继续轮询, newInterval 新的检查间隔(0表示不变)
func (s *PaymentPollingService) checkPaymentStatus(task *PollingTask) (bool, int) {
	// 获取订单
	var order models.Order
	if err := s.db.First(&order, task.OrderID).Error; err != nil {
		s.mutex.Lock()
		s.removeFromQueueLocked(task.OrderID)
		s.mutex.Unlock()
		return false, 0
	}

	// 检查订单状态，如果不是待付款则移除
	if order.Status != models.OrderStatusPendingPayment {
		s.mutex.Lock()
		s.removeFromQueueLocked(task.OrderID)
		s.mutex.Unlock()
		return false, 0
	}

	// 检查是否超过最大轮询时长
	if time.Since(task.AddedAt) > s.maxDuration {
		logger.LogPaymentOperation(s.db, "payment_polling_timeout", task.OrderID, map[string]interface{}{
			"retry_count": task.RetryCount,
			"duration":    time.Since(task.AddedAt).String(),
		})
		s.mutex.Lock()
		s.removeFromQueueLocked(task.OrderID)
		s.mutex.Unlock()
		return false, 0
	}

	// 检查是否超过最大重试次数
	if task.RetryCount >= s.maxRetries {
		logger.LogPaymentOperation(s.db, "payment_polling_max_retries", task.OrderID, map[string]interface{}{
			"retry_count": task.RetryCount,
			"max_retries": s.maxRetries,
		})
		s.mutex.Lock()
		s.removeFromQueueLocked(task.OrderID)
		s.mutex.Unlock()
		return false, 0
	}

	// 获取付款方式
	var pm models.PaymentMethod
	if err := s.db.First(&pm, task.PaymentMethodID).Error; err != nil {
		s.mutex.Lock()
		s.removeFromQueueLocked(task.OrderID)
		s.mutex.Unlock()
		return false, 0
	}

	// 检查付款方式的间隔是否有变化
	newInterval := 0
	if pm.PollInterval > 0 && pm.PollInterval != task.CheckInterval {
		newInterval = pm.PollInterval
	}

	// 检查付款状态
	result, err := s.jsRuntime.CheckPaymentStatus(&pm, &order)
	if err == nil && result.Paid {
		// 付款成功，更新订单状态
		s.handlePaymentSuccess(task, &order, &pm, result)
		return false, 0
	}

	return true, newInterval
}

// handlePaymentSuccess 处理付款成功
func (s *PaymentPollingService) handlePaymentSuccess(task *PollingTask, order *models.Order, pm *models.PaymentMethod, result *PaymentCheckResult) {
	// 判断是否为纯虚拟商品订单
	isVirtualOnly := true
	for _, item := range order.Items {
		if item.ProductType != models.ProductTypeVirtual {
			isVirtualOnly = false
			break
		}
	}

	// 根据订单类型设置状态
	updates := map[string]interface{}{}

	if isVirtualOnly {
		// 纯虚拟商品订单
		if s.virtualInventorySvc != nil {
			// 检查是否所有虚拟库存都可以自动发货
			canAuto, err := s.virtualInventorySvc.CanAutoDeliver(order.OrderNo)
			if err != nil {
				logger.LogPaymentOperation(s.db, "check_auto_delivery_failed", task.OrderID, map[string]interface{}{
					"error": err.Error(),
				})
			}

			if canAuto {
				// 所有库存都属于自动发货商品，执行自动发货
				if err := s.virtualInventorySvc.DeliverAutoDeliveryStock(order.ID, order.OrderNo, nil); err != nil {
					logger.LogPaymentOperation(s.db, "virtual_delivery_failed", task.OrderID, map[string]interface{}{
						"error": err.Error(),
					})
					// 自动发货失败，回退到手动发货
					updates["status"] = models.OrderStatusPending
				} else {
					now := time.Now()
					updates["status"] = models.OrderStatusShipped
					updates["shipped_at"] = now
				}
			} else {
				// 存在非自动发货的库存，全部交给管理员手动发货
				updates["status"] = models.OrderStatusPending
			}
		} else {
			updates["status"] = models.OrderStatusPending
		}
	} else {
		// 实物或混合订单：设置为草稿状态，等待填写收货信息
		updates["status"] = models.OrderStatusDraft

		// 混合订单：仅当所有虚拟库存都可自动发货时才自动发货，否则全部留给管理员
		if s.virtualInventorySvc != nil {
			canAuto, _ := s.virtualInventorySvc.CanAutoDeliver(order.OrderNo)
			if canAuto {
				if err := s.virtualInventorySvc.DeliverAutoDeliveryStock(order.ID, order.OrderNo, nil); err != nil {
					logger.LogPaymentOperation(s.db, "virtual_delivery_failed", task.OrderID, map[string]interface{}{
						"error":      err.Error(),
						"order_type": "mixed",
					})
				}
			}
		}
	}

	// 在事务中原子更新订单状态和付款记录，防止部分失败导致状态不一致
	paymentData := map[string]interface{}{
		"paid_at": time.Now().Format(time.RFC3339),
	}
	if result.TransactionID != "" {
		paymentData["transaction_id"] = result.TransactionID
	}
	if result.Data != nil {
		for k, v := range result.Data {
			paymentData[k] = v
		}
	}
	paymentDataJSON, _ := json.Marshal(paymentData)

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(order).Updates(updates).Error; err != nil {
			return err
		}
		return tx.Model(&models.OrderPaymentMethod{}).
			Where("order_id = ?", task.OrderID).
			Update("payment_data", string(paymentDataJSON)).Error
	}); err != nil {
		logger.LogPaymentOperation(s.db, "payment_update_failed", task.OrderID, map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	// 记录付款成功日志
	logger.LogPaymentOperation(s.db, "payment_success", task.OrderID, map[string]interface{}{
		"order_no":          order.OrderNo,
		"payment_method":    pm.Name,
		"transaction_id":    result.TransactionID,
		"total_amount":      order.TotalAmount,
		"currency":          order.Currency,
		"polling_attempts":  task.RetryCount,
		"check_interval":    task.CheckInterval,
		"new_status":        updates["status"],
		"is_virtual_only":   isVirtualOnly,
	})

	// 发送付款成功邮件
	if s.emailService != nil {
		go s.emailService.SendOrderPaidEmail(order, isVirtualOnly)
	}

	// 从队列移除
	s.mutex.Lock()
	s.removeFromQueueLocked(task.OrderID)
	s.mutex.Unlock()
}

// saveTaskToDB 保存任务到数据库
func (s *PaymentPollingService) saveTaskToDB(task *PollingTask) {
	data, _ := json.Marshal(task)

	record := models.PaymentPollingTask{
		OrderID: task.OrderID,
		Data:    string(data),
	}

	s.db.Where("order_id = ?", task.OrderID).
		Assign(models.PaymentPollingTask{Data: string(data)}).
		FirstOrCreate(&record)
}

// removeTaskFromDB 从数据库移除任务
func (s *PaymentPollingService) removeTaskFromDB(orderID uint) {
	s.db.Where("order_id = ?", orderID).Delete(&models.PaymentPollingTask{})
}

// recoverTasks 从数据库恢复任务
func (s *PaymentPollingService) recoverTasks() {
	var records []models.PaymentPollingTask
	if err := s.db.Find(&records).Error; err != nil {
		return
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	recoveredCount := 0
	removedCount := 0

	for _, record := range records {
		var task PollingTask
		if err := json.Unmarshal([]byte(record.Data), &task); err != nil {
			continue
		}

		// 检查订单是否仍在待付款状态
		var order models.Order
		if err := s.db.First(&order, task.OrderID).Error; err != nil {
			s.removeTaskFromDB(task.OrderID)
			removedCount++
			continue
		}

		if order.Status != models.OrderStatusPendingPayment {
			s.removeTaskFromDB(task.OrderID)
			removedCount++
			continue
		}

		// 检查是否超时
		if time.Since(task.AddedAt) > s.maxDuration {
			s.removeTaskFromDB(task.OrderID)
			removedCount++
			continue
		}

		// 获取最新的轮询间隔
		var pm models.PaymentMethod
		if err := s.db.First(&pm, task.PaymentMethodID).Error; err == nil && pm.PollInterval > 0 {
			task.CheckInterval = pm.PollInterval
		}

		// 设置下次检查时间为立即
		task.NextCheckAt = time.Now()

		s.taskMap[task.OrderID] = &task
		heap.Push(&s.taskHeap, &task)
		recoveredCount++
	}

	// 扫描所有待付款订单，确保都在队列中
	addedCount := s.scanPendingPaymentOrders()

	if recoveredCount > 0 || removedCount > 0 || addedCount > 0 {
		logger.LogSystemOperation(s.db, "payment_polling_recover", "system", nil, map[string]interface{}{
			"recovered": recoveredCount,
			"removed":   removedCount,
			"added":     addedCount,
		})
	}
}

// scanPendingPaymentOrders 扫描待付款订单，确保都在轮询队列中
func (s *PaymentPollingService) scanPendingPaymentOrders() int {
	// 查询所有待付款且已选择付款方式的订单
	var orderPayments []models.OrderPaymentMethod
	if err := s.db.Find(&orderPayments).Error; err != nil {
		return 0
	}

	addedCount := 0
	now := time.Now()

	for _, op := range orderPayments {
		// 检查是否已在队列中
		if _, exists := s.taskMap[op.OrderID]; exists {
			continue
		}

		// 检查订单是否仍在待付款状态
		var order models.Order
		if err := s.db.First(&order, op.OrderID).Error; err != nil {
			continue
		}

		if order.Status != models.OrderStatusPendingPayment {
			continue
		}

		// 获取付款方式的轮询间隔
		interval := s.defaultInterval
		var pm models.PaymentMethod
		if err := s.db.First(&pm, op.PaymentMethodID).Error; err == nil && pm.PollInterval > 0 {
			interval = pm.PollInterval
		}

		// 添加到队列
		task := &PollingTask{
			OrderID:         op.OrderID,
			PaymentMethodID: op.PaymentMethodID,
			AddedAt:         now,
			NextCheckAt:     now, // 立即检查
			CheckInterval:   interval,
			RetryCount:      0,
		}
		s.taskMap[op.OrderID] = task
		heap.Push(&s.taskHeap, task)
		s.saveTaskToDB(task)
		addedCount++
	}

	return addedCount
}
