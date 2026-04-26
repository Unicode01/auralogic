package service

import (
	"container/heap"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/models"
	"auralogic/internal/pkg/bizerr"
	"auralogic/internal/pkg/logger"
	"auralogic/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const paymentPollingStartupBatchSize = 200
const paymentPollingPersistBatchSize = 200

// PollingTask 轮询任务
type PollingTask struct {
	OrderID         uint      `json:"order_id"`
	UserID          uint      `json:"user_id"`
	PaymentMethodID uint      `json:"payment_method_id"`
	AddedAt         time.Time `json:"added_at"`
	NextCheckAt     time.Time `json:"next_check_at"`  // 下次检查时间
	CheckInterval   int       `json:"check_interval"` // 检查间隔(秒)
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
	cfg                 *config.Config
	orderRepo           *repository.OrderRepository
	userRepo            *repository.UserRepository
	jsRuntime           *JSRuntimeService
	virtualInventorySvc *VirtualInventoryService
	emailService        *EmailService
	pluginManager       *PluginManagerService
	taskHeap            TaskHeap              // 时间轮：按下次检查时间排序的最小堆
	taskMap             map[uint]*PollingTask // orderID -> task 快速查找
	userTaskCounts      map[uint]int          // userID -> queue task count
	pendingBackfill     bool
	lifecycleMu         sync.Mutex
	running             bool
	mutex               sync.Mutex
	stopChan            chan struct{}
	doneChan            chan struct{}
	wakeupChan          chan struct{} // 用于唤醒主循环
	defaultInterval     int           // 默认检查间隔(秒)
	maxRetries          int           // 最大重试次数
	maxDuration         time.Duration // 最大轮询时长
	maxTasksPerUser     int           // 每用户轮询任务上限
	maxTasksGlobal      int           // 全局轮询任务上限
}

type pendingPaymentPollingOrderRow struct {
	CursorID        uint  `gorm:"column:cursor_id"`
	OrderID         uint  `gorm:"column:order_id"`
	UserID          *uint `gorm:"column:user_id"`
	PaymentMethodID uint  `gorm:"column:payment_method_id"`
	PollInterval    int   `gorm:"column:poll_interval"`
}

// NewPaymentPollingService 创建付款轮询服务
func NewPaymentPollingService(db *gorm.DB, virtualInventorySvc *VirtualInventoryService, emailService *EmailService, cfg *config.Config) *PaymentPollingService {
	maxTasksPerUser := 20
	maxTasksGlobal := 2000
	if cfg != nil {
		if cfg.Order.MaxPaymentPollingTasksPerUser > 0 {
			maxTasksPerUser = cfg.Order.MaxPaymentPollingTasksPerUser
		}
		if cfg.Order.MaxPaymentPollingTasksGlobal > 0 {
			maxTasksGlobal = cfg.Order.MaxPaymentPollingTasksGlobal
		}
	}

	return &PaymentPollingService{
		db:                  db,
		cfg:                 cfg,
		orderRepo:           repository.NewOrderRepository(db),
		userRepo:            repository.NewUserRepository(db),
		jsRuntime:           NewJSRuntimeService(db, cfg),
		virtualInventorySvc: virtualInventorySvc,
		emailService:        emailService,
		taskHeap:            make(TaskHeap, 0),
		taskMap:             make(map[uint]*PollingTask),
		userTaskCounts:      make(map[uint]int),
		wakeupChan:          make(chan struct{}, 1),
		defaultInterval:     30,            // 默认30秒
		maxRetries:          480,           // 最多重试480次
		maxDuration:         4 * time.Hour, // 最长轮询4小时
		maxTasksPerUser:     maxTasksPerUser,
		maxTasksGlobal:      maxTasksGlobal,
	}
}

func (s *PaymentPollingService) SetPluginManager(pluginManager *PluginManagerService) {
	s.pluginManager = pluginManager
}

func (s *PaymentPollingService) currentQueueLimits() (int, int) {
	maxTasksPerUser := s.maxTasksPerUser
	maxTasksGlobal := s.maxTasksGlobal
	if s.cfg != nil {
		if s.cfg.Order.MaxPaymentPollingTasksPerUser > 0 {
			maxTasksPerUser = s.cfg.Order.MaxPaymentPollingTasksPerUser
		}
		if s.cfg.Order.MaxPaymentPollingTasksGlobal > 0 {
			maxTasksGlobal = s.cfg.Order.MaxPaymentPollingTasksGlobal
		}
	}
	return maxTasksPerUser, maxTasksGlobal
}

// Start 启动轮询服务
func (s *PaymentPollingService) Start() {
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

	logger.LogSystemOperation(s.db, "payment_polling_start", "system", nil, map[string]interface{}{
		"default_interval": s.defaultInterval,
		"max_retries":      s.maxRetries,
		"max_duration":     s.maxDuration.String(),
		"max_tasks_user":   s.maxTasksPerUser,
		"max_tasks_global": s.maxTasksGlobal,
		"algorithm":        "time_wheel",
	})

	s.resetQueueState()
	// 从数据库恢复未完成的轮询任务
	s.recoverTasks()
	go func() {
		defer close(doneChan)
		runBackgroundServiceWithStopChan("payment_polling.timeWheelLoop", stopChan, s.timeWheelLoop)
	}()
}

// Stop 停止轮询服务
func (s *PaymentPollingService) Stop() {
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

	logger.LogSystemOperation(s.db, "payment_polling_stop", "system", nil, nil)
	close(stopChan)
	<-doneChan
	s.lifecycleMu.Unlock()
}

// AddToQueue 添加订单到轮询队列
func (s *PaymentPollingService) AddToQueue(orderID, paymentMethodID uint) error {
	var order models.Order
	if err := s.db.Select("id", "user_id", "status").First(&order, orderID).Error; err != nil {
		return err
	}

	if order.Status != models.OrderStatusPendingPayment {
		return bizerr.Newf(
			"payment.pollingInvalidOrderStatus",
			"Order status %s does not support payment polling",
			order.Status,
		).WithParams(map[string]interface{}{
			"order_id": orderID,
			"status":   order.Status,
		})
	}
	queueUserID := uint(0)
	if order.UserID != nil {
		queueUserID = *order.UserID
	}

	// 获取付款方式的轮询间隔
	interval := s.defaultInterval
	var pm models.PaymentMethod
	if err := s.db.Select("id", "poll_interval").First(&pm, paymentMethodID).Error; err == nil && pm.PollInterval > 0 {
		interval = pm.PollInterval
	}

	s.mutex.Lock()
	now := time.Now()
	if task, exists := s.taskMap[orderID]; exists {
		task.PaymentMethodID = paymentMethodID
		task.AddedAt = now
		task.NextCheckAt = now
		task.CheckInterval = interval
		task.RetryCount = 0
		clonedTask := s.cloneTask(task)
		if task.index >= 0 && task.index < len(s.taskHeap) {
			heap.Fix(&s.taskHeap, task.index)
		}
		s.mutex.Unlock()
		s.saveTaskToDB(clonedTask)
		s.wakeup()
		return nil
	}

	if err := s.checkQueueQuotaLocked(queueUserID); err != nil {
		s.mutex.Unlock()
		return err
	}

	task := &PollingTask{
		OrderID:         orderID,
		UserID:          queueUserID,
		PaymentMethodID: paymentMethodID,
		AddedAt:         now,
		NextCheckAt:     now, // 立即检查一次
		CheckInterval:   interval,
		RetryCount:      0,
	}

	s.taskMap[orderID] = task
	s.incrementUserTaskLocked(queueUserID)
	heap.Push(&s.taskHeap, task)
	queueSize := len(s.taskMap)
	s.mutex.Unlock()

	// 保存到数据库（用于服务重启后恢复）
	s.saveTaskToDB(s.cloneTask(task))

	logger.LogPaymentOperation(s.db, "payment_polling_add", orderID, map[string]interface{}{
		"payment_method_id": paymentMethodID,
		"check_interval":    interval,
		"user_id":           queueUserID,
		"queue_size":        queueSize,
	})

	// 唤醒主循环
	s.wakeup()
	return nil
}

func (s *PaymentPollingService) checkQueueQuotaLocked(userID uint) error {
	maxTasksPerUser, maxTasksGlobal := s.currentQueueLimits()
	if maxTasksGlobal > 0 && len(s.taskMap) >= maxTasksGlobal {
		return bizerr.Newf(
			"payment.pollingGlobalQueueLimitExceeded",
			"Payment polling queue has reached global limit (%d)",
			maxTasksGlobal,
		).WithParams(map[string]interface{}{
			"current": len(s.taskMap),
			"max":     maxTasksGlobal,
		})
	}

	if userID > 0 && maxTasksPerUser > 0 {
		current := s.userTaskCounts[userID]
		if current >= maxTasksPerUser {
			return bizerr.Newf(
				"payment.pollingUserQueueLimitExceeded",
				"You already have %d payment polling tasks, maximum is %d",
				current,
				maxTasksPerUser,
			).WithParams(map[string]interface{}{
				"user_id": userID,
				"current": current,
				"max":     maxTasksPerUser,
			})
		}
	}

	return nil
}

func (s *PaymentPollingService) incrementUserTaskLocked(userID uint) {
	if userID == 0 {
		return
	}
	s.userTaskCounts[userID]++
}

func (s *PaymentPollingService) decrementUserTaskLocked(userID uint) {
	if userID == 0 {
		return
	}
	current := s.userTaskCounts[userID]
	if current <= 1 {
		delete(s.userTaskCounts, userID)
		return
	}
	s.userTaskCounts[userID] = current - 1
}

func clonePaymentExecutionContext(execCtx *ExecutionContext) *ExecutionContext {
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

func (s *PaymentPollingService) buildPaymentHookExecutionContext(order *models.Order, task *PollingTask, source string) *ExecutionContext {
	var userID *uint
	var orderID *uint
	normalizedSource := strings.TrimSpace(source)
	if normalizedSource == "" {
		normalizedSource = "payment_polling"
	}
	metadata := map[string]string{
		"source": normalizedSource,
	}
	if order != nil {
		if order.UserID != nil {
			uid := *order.UserID
			userID = &uid
			metadata["user_id"] = strconv.FormatUint(uint64(uid), 10)
		}
		oid := order.ID
		orderID = &oid
		metadata["order_no"] = order.OrderNo
	}
	if task != nil {
		metadata["retry_count"] = strconv.Itoa(task.RetryCount)
		metadata["check_interval"] = strconv.Itoa(task.CheckInterval)
		metadata["payment_method_id"] = strconv.FormatUint(uint64(task.PaymentMethodID), 10)
	}
	return &ExecutionContext{
		UserID:   userID,
		OrderID:  orderID,
		Metadata: metadata,
	}
}

func (s *PaymentPollingService) emitPaymentHookAsync(hook string, payload map[string]interface{}, execCtx *ExecutionContext) {
	if s.pluginManager == nil {
		return
	}
	go func(hookName string, hookPayload map[string]interface{}, hookExecCtx *ExecutionContext) {
		_, hookErr := s.pluginManager.ExecuteHook(HookExecutionRequest{
			Hook:    hookName,
			Payload: hookPayload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("%s hook execution failed: err=%v", hookName, hookErr)
		}
	}(hook, payload, clonePaymentExecutionContext(execCtx))
}

func paymentHookValueToOptionalString(value interface{}) (string, error) {
	if value == nil {
		return "", nil
	}
	str, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("value must be string")
	}
	return str, nil
}

func paymentHookValueToPayloadMap(value interface{}) (map[string]interface{}, error) {
	if value == nil {
		return nil, nil
	}
	body, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var out map[string]interface{}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func applyPaymentConfirmHookPayload(result *PaymentCheckResult, payload map[string]interface{}) error {
	if result == nil || payload == nil {
		return nil
	}

	if raw, exists := payload["transaction_id"]; exists {
		transactionID, err := paymentHookValueToOptionalString(raw)
		if err != nil {
			return fmt.Errorf("decode transaction_id: %w", err)
		}
		result.TransactionID = strings.TrimSpace(transactionID)
	}
	if raw, exists := payload["payment_result"]; exists {
		paymentResult, err := paymentHookValueToPayloadMap(raw)
		if err != nil {
			return fmt.Errorf("decode payment_result: %w", err)
		}
		result.Data = paymentResult
	}

	return nil
}

func buildPaymentTaskForOrder(order *models.Order, paymentMethodID uint, pollInterval int) *PollingTask {
	if order == nil {
		return &PollingTask{PaymentMethodID: paymentMethodID}
	}
	userID := uint(0)
	if order.UserID != nil {
		userID = *order.UserID
	}
	return &PollingTask{
		OrderID:         order.ID,
		UserID:          userID,
		PaymentMethodID: paymentMethodID,
		CheckInterval:   pollInterval,
		RetryCount:      0,
		AddedAt:         time.Now(),
		NextCheckAt:     time.Now(),
	}
}

func (s *PaymentPollingService) applyPaymentConfirmBeforeHook(
	order *models.Order,
	task *PollingTask,
	pm *models.PaymentMethod,
	result *PaymentCheckResult,
	source string,
) (bool, error) {
	if order == nil || pm == nil || result == nil || s.pluginManager == nil {
		return false, nil
	}

	execCtx := s.buildPaymentHookExecutionContext(order, task, source)
	beforePayload := map[string]interface{}{
		"order_id":          order.ID,
		"order_no":          order.OrderNo,
		"user_id":           task.UserID,
		"payment_method_id": pm.ID,
		"payment_method":    pm.Name,
		"transaction_id":    result.TransactionID,
		"status_before":     order.Status,
		"retry_count":       task.RetryCount,
		"source":            strings.TrimSpace(source),
	}
	if beforePayload["source"] == "" {
		beforePayload["source"] = "payment_polling"
	}
	if result.Data != nil {
		beforePayload["payment_result"] = result.Data
	}
	hookResult, hookErr := s.pluginManager.ExecuteHook(HookExecutionRequest{
		Hook:    "payment.confirm.before",
		Payload: beforePayload,
	}, execCtx)
	if hookErr != nil {
		log.Printf("payment.confirm.before hook execution failed: order=%s err=%v", order.OrderNo, hookErr)
		return false, nil
	}
	if hookResult == nil {
		return false, nil
	}
	if hookResult.Blocked {
		reason := strings.TrimSpace(hookResult.BlockReason)
		if reason == "" {
			reason = "payment confirmation blocked by plugin"
		}
		return true, fmt.Errorf("%s", reason)
	}
	if hookResult.Payload != nil {
		originalResult := &PaymentCheckResult{
			Paid:          result.Paid,
			TransactionID: result.TransactionID,
			Message:       result.Message,
			Data:          clonePayloadMap(result.Data),
		}
		if applyErr := applyPaymentConfirmHookPayload(result, hookResult.Payload); applyErr != nil {
			log.Printf("payment.confirm.before payload apply failed, fallback to original result: order=%s err=%v", order.OrderNo, applyErr)
			result.TransactionID = originalResult.TransactionID
			result.Message = originalResult.Message
			result.Data = originalResult.Data
		}
	}
	return false, nil
}

func (s *PaymentPollingService) ConfirmPaymentResult(
	orderID uint,
	paymentMethodID uint,
	result *PaymentCheckResult,
	source string,
) (bool, error) {
	normalizedSource := strings.TrimSpace(source)
	if normalizedSource == "" {
		normalizedSource = "payment_polling"
	}
	if result == nil || !result.Paid {
		return false, fmt.Errorf("payment result is not confirmed")
	}

	var order models.Order
	if err := s.db.First(&order, orderID).Error; err != nil {
		return false, err
	}

	var pm models.PaymentMethod
	if err := s.db.First(&pm, paymentMethodID).Error; err != nil {
		return false, err
	}
	var opm models.OrderPaymentMethod
	if err := s.db.Where("order_id = ?", orderID).First(&opm).Error; err != nil {
		return false, err
	}
	if opm.PaymentMethodID != paymentMethodID {
		return false, fmt.Errorf("order payment method mismatch")
	}

	task := buildPaymentTaskForOrder(&order, paymentMethodID, pm.PollInterval)
	if blocked, err := s.applyPaymentConfirmBeforeHook(&order, task, &pm, result, normalizedSource); blocked {
		s.emitPaymentHookAsync("payment.polling.failed", map[string]interface{}{
			"order_id":          order.ID,
			"order_no":          order.OrderNo,
			"user_id":           task.UserID,
			"payment_method_id": pm.ID,
			"reason":            "confirm_blocked",
			"block_reason":      err.Error(),
			"retry_count":       task.RetryCount,
			"source":            normalizedSource,
		}, s.buildPaymentHookExecutionContext(&order, task, normalizedSource))
		return false, err
	}

	if order.Status != models.OrderStatusPendingPayment {
		s.mutex.Lock()
		s.removeFromQueueLocked(order.ID)
		s.mutex.Unlock()
		return true, nil
	}

	if err := s.handlePaymentSuccess(task, &order, &pm, result, normalizedSource); err != nil {
		if isOrderHighConcurrencyBusyError(err) {
			return false, newOrderHighConcurrencyBusyError()
		}
		return false, err
	}
	return false, nil
}

// RemoveFromQueue 从队列中移除订单
func (s *PaymentPollingService) RemoveFromQueue(orderID uint) {
	s.removeFromQueue(orderID)
}

// removeFromQueueLocked 从队列中移除订单（需要持有锁）
func (s *PaymentPollingService) removeFromQueueLocked(orderID uint) bool {
	task, exists := s.taskMap[orderID]
	if !exists {
		return false
	}

	delete(s.taskMap, orderID)
	s.decrementUserTaskLocked(task.UserID)
	if task.index >= 0 && task.index < len(s.taskHeap) {
		heap.Remove(&s.taskHeap, task.index)
	}
	return true
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
func (s *PaymentPollingService) timeWheelLoop(stopChan <-chan struct{}) {
	for {
		s.runPendingQueueBackfillIfNeeded()

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
		case <-stopChan:
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
		shouldContinue, newInterval := s.safeCheckPaymentStatus(task)

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
			}
			s.mutex.Unlock()
			// Task persistence is only used for crash recovery.
			// We intentionally skip persisting every retry to avoid continuous write amplification;
			// startup recovery and pending-order scanning will rebuild active tasks.
		}
	}
}

func (s *PaymentPollingService) safeCheckPaymentStatus(task *PollingTask) (shouldContinue bool, newInterval int) {
	defer func() {
		if recovered := recover(); recovered != nil {
			orderID := uint(0)
			retryCount := 0
			userID := uint(0)
			if task != nil {
				orderID = task.OrderID
				retryCount = task.RetryCount
				userID = task.UserID
			}
			logger.LogPaymentOperation(s.db, "payment_polling_panic", orderID, map[string]interface{}{
				"panic":       fmt.Sprint(recovered),
				"retry_count": retryCount,
			})
			s.emitPaymentHookAsync("payment.polling.failed", map[string]interface{}{
				"order_id":    orderID,
				"user_id":     userID,
				"reason":      "panic",
				"panic":       fmt.Sprint(recovered),
				"retry_count": retryCount,
				"source":      "payment_polling",
			}, s.buildPaymentHookExecutionContext(nil, task, "payment_polling"))
			if orderID > 0 {
				s.removeFromQueue(orderID)
			}
			shouldContinue = false
			newInterval = 0
		}
	}()

	return s.checkPaymentStatus(task)
}

// checkPaymentStatus 检查付款状态
// 返回: shouldContinue 是否继续轮询, newInterval 新的检查间隔(0表示不变)
func (s *PaymentPollingService) checkPaymentStatus(task *PollingTask) (bool, int) {
	// 获取订单
	var order models.Order
	if err := s.db.First(&order, task.OrderID).Error; err != nil {
		s.emitPaymentHookAsync("payment.polling.failed", map[string]interface{}{
			"order_id":    task.OrderID,
			"user_id":     task.UserID,
			"reason":      "order_not_found",
			"error":       err.Error(),
			"retry_count": task.RetryCount,
			"source":      "payment_polling",
		}, s.buildPaymentHookExecutionContext(nil, task, "payment_polling"))
		s.removeFromQueue(task.OrderID)
		return false, 0
	}

	// 检查订单状态，如果不是待付款则移除
	if order.Status != models.OrderStatusPendingPayment {
		s.emitPaymentHookAsync("payment.polling.failed", map[string]interface{}{
			"order_id":    order.ID,
			"order_no":    order.OrderNo,
			"user_id":     task.UserID,
			"reason":      "order_status_changed",
			"status":      order.Status,
			"retry_count": task.RetryCount,
			"source":      "payment_polling",
		}, s.buildPaymentHookExecutionContext(&order, task, "payment_polling"))
		s.removeFromQueue(task.OrderID)
		return false, 0
	}

	// 检查是否超过最大轮询时长
	if time.Since(task.AddedAt) > s.maxDuration {
		logger.LogPaymentOperation(s.db, "payment_polling_timeout", task.OrderID, map[string]interface{}{
			"retry_count": task.RetryCount,
			"duration":    time.Since(task.AddedAt).String(),
		})
		s.emitPaymentHookAsync("payment.polling.failed", map[string]interface{}{
			"order_id":    order.ID,
			"order_no":    order.OrderNo,
			"user_id":     task.UserID,
			"reason":      "timeout",
			"retry_count": task.RetryCount,
			"duration":    time.Since(task.AddedAt).String(),
			"source":      "payment_polling",
		}, s.buildPaymentHookExecutionContext(&order, task, "payment_polling"))
		s.removeFromQueue(task.OrderID)
		return false, 0
	}

	// 检查是否超过最大重试次数
	if task.RetryCount >= s.maxRetries {
		logger.LogPaymentOperation(s.db, "payment_polling_max_retries", task.OrderID, map[string]interface{}{
			"retry_count": task.RetryCount,
			"max_retries": s.maxRetries,
		})
		s.emitPaymentHookAsync("payment.polling.failed", map[string]interface{}{
			"order_id":    order.ID,
			"order_no":    order.OrderNo,
			"user_id":     task.UserID,
			"reason":      "max_retries",
			"retry_count": task.RetryCount,
			"max_retries": s.maxRetries,
			"source":      "payment_polling",
		}, s.buildPaymentHookExecutionContext(&order, task, "payment_polling"))
		s.removeFromQueue(task.OrderID)
		return false, 0
	}

	// 获取付款方式
	var pm models.PaymentMethod
	if err := s.db.First(&pm, task.PaymentMethodID).Error; err != nil {
		s.emitPaymentHookAsync("payment.polling.failed", map[string]interface{}{
			"order_id":          order.ID,
			"order_no":          order.OrderNo,
			"user_id":           task.UserID,
			"payment_method_id": task.PaymentMethodID,
			"reason":            "payment_method_not_found",
			"error":             err.Error(),
			"retry_count":       task.RetryCount,
			"source":            "payment_polling",
		}, s.buildPaymentHookExecutionContext(&order, task, "payment_polling"))
		s.removeFromQueue(task.OrderID)
		return false, 0
	}

	// 检查付款方式的间隔是否有变化
	newInterval := 0
	if pm.PollInterval > 0 && pm.PollInterval != task.CheckInterval {
		newInterval = pm.PollInterval
	}

	// 检查付款状态
	result, err := s.jsRuntime.CheckPaymentStatus(&pm, &order)
	if err != nil {
		logger.LogPaymentOperation(s.db, "payment_polling_check_failed", task.OrderID, map[string]interface{}{
			"error":             err.Error(),
			"payment_method_id": pm.ID,
			"retry_count":       task.RetryCount,
		})
	}
	if err == nil && result.Paid {
		if blocked, blockErr := s.applyPaymentConfirmBeforeHook(&order, task, &pm, result, "payment_polling"); blocked {
			s.emitPaymentHookAsync("payment.polling.failed", map[string]interface{}{
				"order_id":          order.ID,
				"order_no":          order.OrderNo,
				"user_id":           task.UserID,
				"payment_method_id": pm.ID,
				"reason":            "confirm_blocked",
				"block_reason":      blockErr.Error(),
				"retry_count":       task.RetryCount,
				"source":            "payment_polling",
			}, s.buildPaymentHookExecutionContext(&order, task, "payment_polling"))
			s.removeFromQueue(task.OrderID)
			return false, 0
		}

		// 付款成功，更新订单状态
		if finalizeErr := s.handlePaymentSuccess(task, &order, &pm, result, "payment_polling"); finalizeErr == nil {
			return false, 0
		} else if !shouldRetryPaymentFinalizeError(finalizeErr) {
			logger.LogPaymentOperation(s.db, "payment_polling_finalize_stopped", task.OrderID, map[string]interface{}{
				"error":             finalizeErr.Error(),
				"payment_method_id": pm.ID,
				"retry_count":       task.RetryCount,
			})
			s.removeFromQueue(task.OrderID)
			return false, 0
		}
		return true, newInterval
	}

	return true, newInterval
}

// handlePaymentSuccess 处理付款成功
func (s *PaymentPollingService) handlePaymentSuccess(task *PollingTask, order *models.Order, pm *models.PaymentMethod, result *PaymentCheckResult, source string) error {
	releaseHotPath, err := acquireOrderHighConcurrencyProtection(s.cfg, orderHotPathConfirmPaymentResult)
	if err != nil {
		logAction := "payment_hot_path_protection_failed"
		if isOrderHighConcurrencyBusyError(err) {
			logAction = "payment_hot_path_busy"
		}
		logger.LogPaymentOperation(s.db, logAction, task.OrderID, map[string]interface{}{
			"error": err.Error(),
		})
		return err
	}
	defer releaseHotPath()

	normalizedSource := strings.TrimSpace(source)
	if normalizedSource == "" {
		normalizedSource = "payment_polling"
	}
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
	var (
		lockedOrder    *models.Order
		finalizeResult *paidOrderFinalizeResult
	)

	if err := s.orderRepo.WithTransaction(func(tx *gorm.DB) error {
		txOrderRepo := repository.NewOrderRepository(tx)
		currentOrder, err := txOrderRepo.FindByIDForUpdate(tx, task.OrderID)
		if err != nil {
			return err
		}
		lockedOrder = currentOrder
		finalizeResult, err = finalizePendingPaymentOrderTx(tx, currentOrder, s.virtualInventorySvc, paidOrderFinalizeOptions{})
		if err != nil {
			return err
		}
		if !finalizeResult.Updated {
			return nil
		}
		return tx.Model(&models.OrderPaymentMethod{}).
			Where("order_id = ?", task.OrderID).
			Update("payment_data", string(paymentDataJSON)).Error
	}); err != nil {
		logger.LogPaymentOperation(s.db, "payment_update_failed", task.OrderID, map[string]interface{}{
			"error": err.Error(),
		})
		s.emitPaymentHookAsync("payment.polling.failed", map[string]interface{}{
			"order_id":          order.ID,
			"order_no":          order.OrderNo,
			"user_id":           task.UserID,
			"payment_method_id": pm.ID,
			"reason":            "confirm_update_failed",
			"error":             err.Error(),
			"retry_count":       task.RetryCount,
			"source":            normalizedSource,
		}, s.buildPaymentHookExecutionContext(order, task, normalizedSource))
		return err
	}

	if finalizeResult == nil || !finalizeResult.Updated {
		s.removeFromQueue(task.OrderID)
		return nil
	}

	if finalizeResult.AutoDeliveryCheckErr != nil {
		logger.LogPaymentOperation(s.db, "check_auto_delivery_failed", task.OrderID, map[string]interface{}{
			"error": finalizeResult.AutoDeliveryCheckErr.Error(),
		})
	}

	if finalizeResult.VirtualDeliveryErr != nil {
		logData := map[string]interface{}{
			"error": finalizeResult.VirtualDeliveryErr.Error(),
		}
		if !finalizeResult.IsVirtualOnly {
			logData["order_type"] = "mixed"
		}
		logger.LogPaymentOperation(s.db, "virtual_delivery_failed", task.OrderID, logData)
	}

	order = lockedOrder
	s.syncUserConsumptionStatsBestEffort(
		task.OrderID,
		order.UserID,
		models.OrderStatusPendingPayment,
		finalizeResult.FinalStatus,
		order.TotalAmount,
		"payment_polling_success",
	)

	// 记录付款成功日志
	logger.LogPaymentOperation(s.db, "payment_success", task.OrderID, map[string]interface{}{
		"order_no":           order.OrderNo,
		"payment_method":     pm.Name,
		"transaction_id":     result.TransactionID,
		"total_amount_minor": order.TotalAmount,
		"currency":           order.Currency,
		"polling_attempts":   task.RetryCount,
		"check_interval":     task.CheckInterval,
		"new_status":         finalizeResult.FinalStatus,
		"is_virtual_only":    finalizeResult.IsVirtualOnly,
	})

	// 发送付款成功邮件
	if s.emailService != nil {
		go s.emailService.SendOrderPaidEmail(order, finalizeResult.IsVirtualOnly)
	}

	hookExecCtx := s.buildPaymentHookExecutionContext(order, task, normalizedSource)
	s.emitPaymentHookAsync("payment.confirm.after", map[string]interface{}{
		"order_id":          order.ID,
		"order_no":          order.OrderNo,
		"user_id":           task.UserID,
		"payment_method_id": pm.ID,
		"payment_method":    pm.Name,
		"transaction_id":    result.TransactionID,
		"status_before":     models.OrderStatusPendingPayment,
		"status_after":      finalizeResult.FinalStatus,
		"retry_count":       task.RetryCount,
		"source":            normalizedSource,
	}, hookExecCtx)
	EmitOrderStatusChangedAfterHookAsync(s.pluginManager, hookExecCtx, order, models.OrderStatusPendingPayment, finalizeResult.FinalStatus, map[string]interface{}{
		"source":            normalizedSource,
		"trigger_action":    "payment.confirm",
		"payment_method_id": pm.ID,
		"payment_method":    pm.Name,
		"transaction_id":    result.TransactionID,
		"retry_count":       task.RetryCount,
	})
	s.emitPaymentHookAsync("payment.polling.succeeded", map[string]interface{}{
		"order_id":          order.ID,
		"order_no":          order.OrderNo,
		"user_id":           task.UserID,
		"payment_method_id": pm.ID,
		"payment_method":    pm.Name,
		"transaction_id":    result.TransactionID,
		"status_after":      finalizeResult.FinalStatus,
		"retry_count":       task.RetryCount,
		"source":            normalizedSource,
	}, hookExecCtx)

	// 从队列移除
	s.removeFromQueue(task.OrderID)
	return nil
}

func shouldRetryPaymentFinalizeError(err error) bool {
	if err == nil {
		return false
	}
	if isOrderHighConcurrencyBusyError(err) {
		return true
	}

	lowerErr := strings.ToLower(err.Error())
	retryableFragments := []string{
		"database is locked",
		"deadlock",
		"lock wait timeout",
		"timeout",
		"temporarily unavailable",
		"connection reset",
		"connection refused",
		"broken pipe",
		"too many connections",
	}
	for _, fragment := range retryableFragments {
		if strings.Contains(lowerErr, fragment) {
			return true
		}
	}
	return false
}

func (s *PaymentPollingService) syncUserConsumptionStatsBestEffort(
	orderID uint,
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
		logger.LogPaymentOperation(s.db, "sync_user_consumption_stats_failed", orderID, map[string]interface{}{
			"scene":             scene,
			"user_id":           userIDValue(userID),
			"total_spent_delta": totalSpentMinorDelta,
			"order_count_delta": totalOrderCountDelta,
			"error":             err.Error(),
		})
	}
}

func (s *PaymentPollingService) resetQueueState() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.taskHeap = make(TaskHeap, 0)
	s.taskMap = make(map[uint]*PollingTask)
	s.userTaskCounts = make(map[uint]int)
	s.pendingBackfill = false
}

func (s *PaymentPollingService) cloneTask(task *PollingTask) *PollingTask {
	if task == nil {
		return nil
	}
	cloned := *task
	cloned.index = -1
	return &cloned
}

func (s *PaymentPollingService) addRecoveredTask(task *PollingTask) (bool, error) {
	if task == nil {
		return false, fmt.Errorf("polling task is nil")
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.taskMap[task.OrderID]; exists {
		return false, nil
	}
	if err := s.checkQueueQuotaLocked(task.UserID); err != nil {
		return false, err
	}

	task.index = -1
	s.taskMap[task.OrderID] = task
	s.incrementUserTaskLocked(task.UserID)
	heap.Push(&s.taskHeap, task)
	return true, nil
}

func (s *PaymentPollingService) removeFromQueue(orderID uint) {
	s.mutex.Lock()
	removed := s.removeFromQueueLocked(orderID)
	s.mutex.Unlock()

	if removed {
		s.removeTaskFromDB(orderID)
		s.requestPendingQueueBackfill()
	}
}

func (s *PaymentPollingService) requestPendingQueueBackfill() {
	if s.db == nil {
		return
	}

	s.lifecycleMu.Lock()
	running := s.running
	s.lifecycleMu.Unlock()

	if !running {
		s.scanPendingPaymentOrders()
		return
	}

	s.mutex.Lock()
	if s.pendingBackfill {
		s.mutex.Unlock()
		return
	}
	_, maxTasksGlobal := s.currentQueueLimits()
	if maxTasksGlobal > 0 && len(s.taskMap) >= maxTasksGlobal {
		s.mutex.Unlock()
		return
	}
	s.pendingBackfill = true
	s.mutex.Unlock()
	s.wakeup()
}

func (s *PaymentPollingService) consumePendingQueueBackfillRequest() bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.pendingBackfill {
		return false
	}
	s.pendingBackfill = false
	return true
}

func (s *PaymentPollingService) runPendingQueueBackfillIfNeeded() {
	if !s.consumePendingQueueBackfillRequest() {
		return
	}
	addedCount := s.scanPendingPaymentOrders()
	if addedCount > 0 {
		logger.LogSystemOperation(s.db, "payment_polling_backfill", "system", nil, map[string]interface{}{
			"added": addedCount,
		})
	}
}

func (s *PaymentPollingService) remainingQueueCapacity() int {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	_, maxTasksGlobal := s.currentQueueLimits()
	if maxTasksGlobal <= 0 {
		return paymentPollingStartupBatchSize
	}

	remaining := maxTasksGlobal - len(s.taskMap)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// saveTaskToDB 保存任务到数据库
func (s *PaymentPollingService) saveTaskToDB(task *PollingTask) {
	if task == nil {
		return
	}
	s.saveTasksToDB([]*PollingTask{task})
}

func (s *PaymentPollingService) saveTasksToDB(tasks []*PollingTask) {
	if s.db == nil || len(tasks) == 0 {
		return
	}

	records := make([]models.PaymentPollingTask, 0, len(tasks))
	for _, task := range tasks {
		if task == nil || task.OrderID == 0 {
			continue
		}
		data, err := json.Marshal(task)
		if err != nil {
			log.Printf("payment polling task marshal failed: order=%d err=%v", task.OrderID, err)
			continue
		}
		records = append(records, models.PaymentPollingTask{
			OrderID: task.OrderID,
			Data:    string(data),
		})
	}
	if len(records) == 0 {
		return
	}

	if err := s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "order_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"data", "updated_at"}),
	}).CreateInBatches(records, paymentPollingPersistBatchSize).Error; err != nil {
		log.Printf("payment polling task batch save failed: count=%d err=%v", len(records), err)
	}
}

// removeTaskFromDB 从数据库移除任务
func (s *PaymentPollingService) removeTaskFromDB(orderID uint) {
	if orderID == 0 {
		return
	}
	s.removeTasksFromDB([]uint{orderID})
}

func (s *PaymentPollingService) removeTasksFromDB(orderIDs []uint) {
	if s.db == nil || len(orderIDs) == 0 {
		return
	}

	uniqueOrderIDs := make([]uint, 0, len(orderIDs))
	seen := make(map[uint]struct{}, len(orderIDs))
	for _, orderID := range orderIDs {
		if orderID == 0 {
			continue
		}
		if _, exists := seen[orderID]; exists {
			continue
		}
		seen[orderID] = struct{}{}
		uniqueOrderIDs = append(uniqueOrderIDs, orderID)
	}
	if len(uniqueOrderIDs) == 0 {
		return
	}

	if err := s.db.Where("order_id IN ?", uniqueOrderIDs).Delete(&models.PaymentPollingTask{}).Error; err != nil {
		log.Printf("payment polling task batch delete failed: count=%d err=%v", len(uniqueOrderIDs), err)
	}
}

func isPaymentPollingQueueQuotaError(err error) bool {
	if err == nil {
		return false
	}
	var bizErr *bizerr.Error
	if !errors.As(err, &bizErr) {
		return false
	}
	return bizErr.Key == "payment.pollingUserQueueLimitExceeded" ||
		bizErr.Key == "payment.pollingGlobalQueueLimitExceeded"
}

// recoverTasks 从数据库恢复任务
func (s *PaymentPollingService) recoverTasks() {
	var records []models.PaymentPollingTask
	if err := s.db.Find(&records).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("payment polling recover query failed: %v", err)
		}
		return
	}

	recoveredCount := 0
	removedCount := 0
	deferredBackfillCount := 0
	parsedTasks := make([]PollingTask, 0, len(records))
	orderIDs := make([]uint, 0, len(records))
	paymentMethodIDs := make([]uint, 0, len(records))
	invalidOrderIDs := make([]uint, 0)
	backfillOrderIDs := make([]uint, 0)

	for _, record := range records {
		var task PollingTask
		if err := json.Unmarshal([]byte(record.Data), &task); err != nil {
			invalidOrderIDs = append(invalidOrderIDs, record.OrderID)
			removedCount++
			continue
		}
		parsedTasks = append(parsedTasks, task)
		orderIDs = append(orderIDs, task.OrderID)
		paymentMethodIDs = append(paymentMethodIDs, task.PaymentMethodID)
	}

	orderByID := make(map[uint]models.Order, len(orderIDs))
	if len(orderIDs) > 0 {
		var orders []models.Order
		if err := s.db.Select("id", "user_id", "status").Where("id IN ?", orderIDs).Find(&orders).Error; err != nil {
			log.Printf("payment polling recover order query failed: %v", err)
			return
		}
		for _, order := range orders {
			orderByID[order.ID] = order
		}
	}

	paymentMethodByID := make(map[uint]models.PaymentMethod, len(paymentMethodIDs))
	if len(paymentMethodIDs) > 0 {
		var paymentMethods []models.PaymentMethod
		if err := s.db.Select("id", "poll_interval").Where("id IN ?", paymentMethodIDs).Find(&paymentMethods).Error; err != nil {
			log.Printf("payment polling recover payment method query failed: %v", err)
			return
		}
		for _, paymentMethod := range paymentMethods {
			paymentMethodByID[paymentMethod.ID] = paymentMethod
		}
	}

	now := time.Now()
	for _, task := range parsedTasks {
		order, exists := orderByID[task.OrderID]
		if !exists {
			invalidOrderIDs = append(invalidOrderIDs, task.OrderID)
			removedCount++
			continue
		}

		// 检查订单是否仍在待付款状态
		if order.Status != models.OrderStatusPendingPayment {
			invalidOrderIDs = append(invalidOrderIDs, task.OrderID)
			removedCount++
			continue
		}

		// 检查是否超时
		if time.Since(task.AddedAt) > s.maxDuration {
			invalidOrderIDs = append(invalidOrderIDs, task.OrderID)
			removedCount++
			continue
		}

		if order.UserID != nil {
			task.UserID = *order.UserID
		} else {
			task.UserID = 0
		}

		// 获取最新的轮询间隔
		if pm, exists := paymentMethodByID[task.PaymentMethodID]; exists && pm.PollInterval > 0 {
			task.CheckInterval = pm.PollInterval
		}

		// 设置下次检查时间为立即
		task.NextCheckAt = now

		if added, err := s.addRecoveredTask(s.cloneTask(&task)); err != nil {
			if isPaymentPollingQueueQuotaError(err) {
				backfillOrderIDs = append(backfillOrderIDs, task.OrderID)
				deferredBackfillCount++
				continue
			}
			invalidOrderIDs = append(invalidOrderIDs, task.OrderID)
			removedCount++
			continue
		} else if !added {
			continue
		}
		recoveredCount++
	}
	if len(invalidOrderIDs) > 0 {
		s.removeTasksFromDB(invalidOrderIDs)
	}
	if len(backfillOrderIDs) > 0 {
		s.removeTasksFromDB(backfillOrderIDs)
	}

	// 扫描所有待付款订单，确保都在队列中
	addedCount := s.scanPendingPaymentOrders()

	if recoveredCount > 0 || removedCount > 0 || addedCount > 0 || deferredBackfillCount > 0 {
		logger.LogSystemOperation(s.db, "payment_polling_recover", "system", nil, map[string]interface{}{
			"recovered":         recoveredCount,
			"removed":           removedCount,
			"added":             addedCount,
			"deferred_backfill": deferredBackfillCount,
		})
	}
}

// scanPendingPaymentOrders 扫描待付款订单，确保都在轮询队列中
func (s *PaymentPollingService) scanPendingPaymentOrders() int {
	addedCount := 0
	now := time.Now()
	lastCursor := uint(0)
	tasksToPersist := make([]*PollingTask, 0, paymentPollingPersistBatchSize)
	for {
		remaining := s.remainingQueueCapacity()
		if remaining == 0 {
			break
		}

		batchSize := paymentPollingStartupBatchSize
		if remaining > 0 && remaining < batchSize {
			batchSize = remaining
		}

		var rows []pendingPaymentPollingOrderRow
		err := s.db.Table("order_payment_methods AS opm").
			Select("opm.id AS cursor_id, opm.order_id, orders.user_id, opm.payment_method_id, COALESCE(payment_methods.poll_interval, 0) AS poll_interval").
			Joins("JOIN orders ON orders.id = opm.order_id").
			Joins("LEFT JOIN payment_methods ON payment_methods.id = opm.payment_method_id").
			Joins("LEFT JOIN payment_polling_tasks AS ppt ON ppt.order_id = opm.order_id").
			Where("opm.id > ? AND orders.status = ? AND ppt.order_id IS NULL", lastCursor, models.OrderStatusPendingPayment).
			Order("opm.id ASC").
			Limit(batchSize).
			Scan(&rows).Error
		if err != nil {
			log.Printf("payment polling pending-order scan failed: %v", err)
			break
		}
		if len(rows) == 0 {
			break
		}

		for _, row := range rows {
			lastCursor = row.CursorID

			queueUserID := uint(0)
			if row.UserID != nil {
				queueUserID = *row.UserID
			}
			interval := s.defaultInterval
			if row.PollInterval > 0 {
				interval = row.PollInterval
			}

			task := &PollingTask{
				OrderID:         row.OrderID,
				UserID:          queueUserID,
				PaymentMethodID: row.PaymentMethodID,
				AddedAt:         now,
				NextCheckAt:     now,
				CheckInterval:   interval,
				RetryCount:      0,
			}
			added, err := s.addRecoveredTask(task)
			if err != nil {
				continue
			}
			if !added {
				continue
			}
			tasksToPersist = append(tasksToPersist, s.cloneTask(task))
			addedCount++
		}
		if len(tasksToPersist) > 0 {
			s.saveTasksToDB(tasksToPersist)
			tasksToPersist = tasksToPersist[:0]
		}

		if len(rows) < batchSize {
			break
		}
	}
	if len(tasksToPersist) > 0 {
		s.saveTasksToDB(tasksToPersist)
	}

	return addedCount
}
