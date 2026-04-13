package service

import (
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
	"auralogic/internal/pkg/password"
	"auralogic/internal/pkg/utils"
	"auralogic/internal/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type OrderService struct {
	OrderRepo         *repository.OrderRepository
	userRepo          *repository.UserRepository
	productRepo       *repository.ProductRepository
	inventoryRepo     *repository.InventoryRepository
	bindingService    *BindingService
	serialService     *SerialService
	serialTaskService *SerialGenerationService
	virtualProductSvc *VirtualInventoryService
	promoCodeRepo     *repository.PromoCodeRepository
	cfg               *config.Config
	emailService      *EmailService
	pluginManager     *PluginManagerService
	userOrderLocks    sync.Map
}

type MarkAsPaidOptions struct {
	AdminRemark      string
	SkipAutoDelivery bool
}

const (
	maxAttributeKeys = 20 // 单个商品项最大属性数
)

var (
	// Public, user-facing errors (safe to show to clients).
	ErrProductNotAvailable      = bizerr.New("order.productNotAvailable", "Product is not available")
	ErrShippingFormAccessDenied = errors.New("shipping form access denied")
	ErrShippingFormNotFound     = errors.New("shipping form not found")
)

func newOrderAttributesTooManyError(max int) error {
	return bizerr.Newf("order.attributesTooMany", "Product attributes cannot exceed %d keys", max).
		WithParams(map[string]interface{}{"max": max})
}

func newOrderTotalAmountNegativeError() error {
	return bizerr.New("order.totalAmountNegative", "Total amount cannot be negative")
}

func newOrderUserNotFoundError() error {
	return bizerr.New("order.userNotFound", "User not found")
}

func newOrderVirtualInventoryRequiredError(sku string) error {
	return bizerr.Newf("order.virtualInventoryRequired", "Virtual product %s must select a virtual inventory", sku).
		WithParams(map[string]interface{}{"sku": sku})
}

func newOrderStatusInvalidError(status string) error {
	return bizerr.Newf("order.statusInvalid", "Invalid order status: %s", status).
		WithParams(map[string]interface{}{"status": status})
}

func newOrderNotFoundError() error {
	return bizerr.New("order.notFound", "Order not found")
}

func newOrderAssignTrackingStatusInvalidError(status models.OrderStatus) error {
	return bizerr.Newf("order.assignTrackingStatusInvalid", "Only pending orders can be assigned tracking number (current status: %s)", status).
		WithParams(map[string]interface{}{"status": status})
}

func newOrderCompleteStatusInvalidError(status models.OrderStatus) error {
	return bizerr.Newf("order.completeStatusInvalid", "Order status %s cannot be marked as completed", status).
		WithParams(map[string]interface{}{"status": status})
}

func newOrderCancelStatusInvalidError(status models.OrderStatus) error {
	return bizerr.Newf("order.cancelStatusInvalid", "Order status %s cannot be cancelled", status).
		WithParams(map[string]interface{}{"status": status})
}

func newOrderDeleteStatusInvalidError(status models.OrderStatus) error {
	return bizerr.Newf("order.deleteStatusInvalid", "Order status %s cannot be deleted", status).
		WithParams(map[string]interface{}{"status": status})
}

func newOrderMarkPaidStatusInvalidError(status models.OrderStatus) error {
	return bizerr.Newf("order.markPaidStatusInvalid", "Only pending payment orders can be marked as paid (current status: %s)", status).
		WithParams(map[string]interface{}{"status": status})
}

func newOrderDeliverVirtualStatusInvalidError(status models.OrderStatus) error {
	return bizerr.Newf("order.deliverVirtualStatusInvalid", "Order status %s cannot deliver virtual stock", status).
		WithParams(map[string]interface{}{"status": status})
}

func newOrderVirtualServiceUnavailableError() error {
	return bizerr.New("order.virtualServiceUnavailable", "Virtual product service is not available")
}

func newOrderNoPendingVirtualStockError() error {
	return bizerr.New("order.noPendingVirtualStock", "No pending virtual stock to deliver")
}

func newOrderHighConcurrencyBusyError() error {
	return bizerr.New("order.systemBusy", "System is busy, please retry shortly")
}

func normalizeOrderLookupError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return newOrderNotFoundError()
	}
	return err
}

func normalizeOrderInventoryAvailabilityError(productName, rawMessage string) error {
	message := strings.TrimSpace(rawMessage)
	if message == "" {
		return nil
	}

	switch {
	case message == "This specification is unavailable":
		return bizerr.New("binding.specUnavailable", message)
	case message == "This specification is sold out":
		return bizerr.Newf("order.stockInsufficient", "Product %s stock insufficient, only %d available", productName, 0).
			WithParams(map[string]interface{}{"product": productName, "available": 0})
	case message == "Insufficient stock":
		return bizerr.Newf("order.stockInsufficient", "Product %s stock insufficient, only %d available", productName, 0).
			WithParams(map[string]interface{}{"product": productName, "available": 0})
	case strings.HasPrefix(message, "Insufficient stock, available quantity:"):
		availableText := strings.TrimSpace(strings.TrimPrefix(message, "Insufficient stock, available quantity:"))
		available, err := strconv.ParseInt(availableText, 10, 64)
		if err != nil {
			return nil
		}
		return bizerr.Newf("order.stockInsufficient", "Product %s stock insufficient, only %d available", productName, available).
			WithParams(map[string]interface{}{"product": productName, "available": available})
	default:
		return nil
	}
}

func normalizeOrderInventoryOperationError(productName string, err error) error {
	if err == nil {
		return nil
	}

	var bizErr *bizerr.Error
	if errors.As(err, &bizErr) {
		return err
	}

	if normalized := normalizeOrderInventoryAvailabilityError(productName, err.Error()); normalized != nil {
		return normalized
	}

	return err
}

// validateOrderItems 校验订单商品项的基本参数合理性
func (s *OrderService) validateOrderItems(items []models.OrderItem) error {
	maxItems := s.cfg.Order.MaxOrderItems
	maxQty := s.cfg.Order.MaxItemQuantity
	if len(items) == 0 {
		return bizerr.New("order.itemsEmpty", "Order items cannot be empty")
	}
	if len(items) > maxItems {
		return bizerr.Newf("order.tooManyItems", "Order items cannot exceed %d", maxItems).
			WithParams(map[string]interface{}{"max": maxItems})
	}
	for i := range items {
		item := &items[i]
		item.SKU = strings.TrimSpace(item.SKU)
		if item.SKU == "" {
			return bizerr.New("order.skuEmpty", "Product SKU cannot be empty")
		}
		if item.Quantity <= 0 {
			return bizerr.New("order.quantityInvalid", "Quantity must be greater than 0")
		}
		if item.Quantity > maxQty {
			return bizerr.Newf("order.quantityExceeded", "Quantity cannot exceed %d", maxQty).
				WithParams(map[string]interface{}{"max": maxQty})
		}
		if len(item.Attributes) > maxAttributeKeys {
			return newOrderAttributesTooManyError(maxAttributeKeys)
		}
	}
	return nil
}

func NewOrderService(
	orderRepo *repository.OrderRepository,
	userRepo *repository.UserRepository,
	productRepo *repository.ProductRepository,
	inventoryRepo *repository.InventoryRepository,
	bindingService *BindingService,
	serialService *SerialService,
	virtualProductSvc *VirtualInventoryService,
	promoCodeRepo *repository.PromoCodeRepository,
	cfg *config.Config,
	emailService *EmailService,
) *OrderService {
	return &OrderService{
		OrderRepo:         orderRepo,
		userRepo:          userRepo,
		productRepo:       productRepo,
		inventoryRepo:     inventoryRepo,
		bindingService:    bindingService,
		serialService:     serialService,
		virtualProductSvc: virtualProductSvc,
		promoCodeRepo:     promoCodeRepo,
		cfg:               cfg,
		emailService:      emailService,
	}
}

func (s *OrderService) SetPluginManager(pluginManager *PluginManagerService) {
	s.pluginManager = pluginManager
}

func (s *OrderService) SetSerialGenerationService(serialTaskService *SerialGenerationService) {
	s.serialTaskService = serialTaskService
}

func cloneOrderHookExecutionContext(execCtx *ExecutionContext) *ExecutionContext {
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

func (s *OrderService) buildInventoryHookExecutionContext(orderID *uint, userID *uint, source string, orderNo string) *ExecutionContext {
	metadata := map[string]string{
		"source": source,
	}
	if orderNo != "" {
		metadata["order_no"] = orderNo
	}
	if userID != nil {
		metadata["user_id"] = strconv.FormatUint(uint64(*userID), 10)
	}
	return &ExecutionContext{
		UserID:   userID,
		OrderID:  orderID,
		Metadata: metadata,
	}
}

func orderHookValueToUint(value interface{}) (uint, error) {
	switch typed := value.(type) {
	case uint:
		return typed, nil
	case uint32:
		return uint(typed), nil
	case uint64:
		return uint(typed), nil
	case int:
		if typed < 0 {
			return 0, fmt.Errorf("value must be non-negative")
		}
		return uint(typed), nil
	case int64:
		if typed < 0 {
			return 0, fmt.Errorf("value must be non-negative")
		}
		return uint(typed), nil
	case float64:
		if typed < 0 {
			return 0, fmt.Errorf("value must be non-negative")
		}
		out := uint(typed)
		if float64(out) != typed {
			return 0, fmt.Errorf("value must be integer")
		}
		return out, nil
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return 0, nil
		}
		parsed, err := strconv.ParseUint(trimmed, 10, 32)
		if err != nil {
			return 0, fmt.Errorf("invalid uint string")
		}
		return uint(parsed), nil
	default:
		return 0, fmt.Errorf("value must be uint")
	}
}

func applyInventoryReserveHookPayload(inventoryID uint, payload map[string]interface{}) (uint, error) {
	if payload == nil {
		return inventoryID, nil
	}
	if raw, exists := payload["inventory_id"]; exists {
		updatedInventoryID, err := orderHookValueToUint(raw)
		if err != nil {
			return inventoryID, fmt.Errorf("decode inventory_id: %w", err)
		}
		if updatedInventoryID == 0 {
			return inventoryID, fmt.Errorf("inventory_id must be greater than 0")
		}
		inventoryID = updatedInventoryID
	}
	return inventoryID, nil
}

func (s *OrderService) reserveInventoryWithHook(orderID *uint, userID *uint, orderNo string, inventoryID uint, quantity int, source string) (uint, error) {
	execCtx := s.buildInventoryHookExecutionContext(orderID, userID, source, orderNo)
	reservedInventoryID := inventoryID
	if s.pluginManager != nil {
		beforePayload := map[string]interface{}{
			"order_id":     orderID,
			"user_id":      userID,
			"order_no":     orderNo,
			"inventory_id": inventoryID,
			"quantity":     quantity,
			"source":       source,
		}
		hookResult, hookErr := s.pluginManager.ExecuteHook(HookExecutionRequest{
			Hook:    "inventory.reserve.before",
			Payload: beforePayload,
		}, execCtx)
		if hookErr != nil {
			log.Printf("inventory.reserve.before hook execution failed: order_no=%s inventory=%d err=%v", orderNo, inventoryID, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "inventory reserve blocked by plugin"
				}
				return 0, errors.New(reason)
			}
			if hookResult.Payload != nil {
				updatedInventoryID, applyErr := applyInventoryReserveHookPayload(inventoryID, hookResult.Payload)
				if applyErr != nil {
					log.Printf("inventory.reserve.before payload apply failed, fallback to original inventory: order_no=%s inventory=%d err=%v", orderNo, inventoryID, applyErr)
				} else {
					reservedInventoryID = updatedInventoryID
				}
			}
		}
	}

	reserveErr := s.inventoryRepo.Reserve(reservedInventoryID, quantity, orderNo)
	if s.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"order_id":     orderID,
			"user_id":      userID,
			"order_no":     orderNo,
			"inventory_id": reservedInventoryID,
			"quantity":     quantity,
			"success":      reserveErr == nil,
			"source":       source,
		}
		if reserveErr != nil {
			afterPayload["error"] = reserveErr.Error()
		}
		go func(execCtx *ExecutionContext, payload map[string]interface{}) {
			_, hookErr := s.pluginManager.ExecuteHook(HookExecutionRequest{
				Hook:    "inventory.reserve.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("inventory.reserve.after hook execution failed: order_no=%s inventory=%d err=%v", orderNo, reservedInventoryID, hookErr)
			}
		}(cloneOrderHookExecutionContext(execCtx), afterPayload)
	}

	if reserveErr != nil {
		return 0, reserveErr
	}
	return reservedInventoryID, nil
}

func (s *OrderService) releaseReservedInventoryWithHook(orderID *uint, userID *uint, orderNo string, inventoryID uint, quantity int, source string) error {
	releaseErr := s.inventoryRepo.ReleaseReserve(inventoryID, quantity, orderNo)
	if s.pluginManager != nil {
		execCtx := s.buildInventoryHookExecutionContext(orderID, userID, source, orderNo)
		afterPayload := map[string]interface{}{
			"order_id":     orderID,
			"user_id":      userID,
			"order_no":     orderNo,
			"inventory_id": inventoryID,
			"quantity":     quantity,
			"success":      releaseErr == nil,
			"source":       source,
		}
		if releaseErr != nil {
			afterPayload["error"] = releaseErr.Error()
		}
		go func(execCtx *ExecutionContext, payload map[string]interface{}) {
			_, hookErr := s.pluginManager.ExecuteHook(HookExecutionRequest{
				Hook:    "inventory.release.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("inventory.release.after hook execution failed: order_no=%s inventory=%d err=%v", orderNo, inventoryID, hookErr)
			}
		}(cloneOrderHookExecutionContext(execCtx), afterPayload)
	}
	return releaseErr
}

func (s *OrderService) ensurePendingPaymentLimit(userID uint) error {
	limit := s.cfg.Order.MaxPendingPaymentOrdersPerUser
	if limit <= 0 {
		return nil
	}

	count, err := s.OrderRepo.CountByUserAndStatus(userID, models.OrderStatusPendingPayment)
	if err != nil {
		return fmt.Errorf("failed to count pending payment orders: %w", err)
	}
	if count < int64(limit) {
		return nil
	}

	return bizerr.Newf(
		"order.pendingPaymentLimitExceeded",
		"You already have %d unpaid orders. The maximum is %d. Please complete or cancel existing unpaid orders first.",
		count, limit,
	).WithParams(map[string]interface{}{
		"current": count,
		"max":     limit,
	})
}

func (s *OrderService) getUserOrderLock(userID uint) *sync.Mutex {
	if userID == 0 {
		return &sync.Mutex{}
	}
	if lock, ok := s.userOrderLocks.Load(userID); ok {
		return lock.(*sync.Mutex)
	}
	newLock := &sync.Mutex{}
	actual, _ := s.userOrderLocks.LoadOrStore(userID, newLock)
	return actual.(*sync.Mutex)
}

func (s *OrderService) lockUserOrderCreation(userID uint) func() {
	lock := s.getUserOrderLock(userID)
	lock.Lock()
	return lock.Unlock
}

// CreateDraft CreateOrder草稿
func (s *OrderService) CreateDraft(items []models.OrderItem, externalUserID, externalOrderID, platform, userEmail, userName, remark string) (*models.Order, error) {
	// generateOrder号
	orderNo := utils.GenerateOrderNo(s.cfg.Order.NoPrefix)

	// generate表单Token
	formToken := uuid.New().String()
	formExpiresAt := models.NowFunc().Add(time.Duration(s.cfg.Form.ExpireHours) * time.Hour)

	// 校验订单商品项
	if err := s.validateOrderItems(items); err != nil {
		return nil, err
	}

	productBySKU, err := s.loadProductsForOrderItems(items)
	if err != nil {
		return nil, err
	}

	// 计算订单总金额
	var totalAmount int64
	for _, item := range items {
		product, exists := productBySKU[item.SKU]
		if !exists || product == nil {
			return nil, bizerr.Newf("order.productNotFound", "Product %s does not exist", item.SKU).
				WithParams(map[string]interface{}{"sku": item.SKU})
		}
		if product.Status != models.ProductStatusActive {
			return nil, ErrProductNotAvailable
		}
		totalAmount += product.Price * int64(item.Quantity)
	}

	// 获取货币单位
	currency := s.cfg.Order.Currency
	if currency == "" {
		currency = "CNY"
	}

	// CreateOrder
	order := &models.Order{
		OrderNo:                   orderNo,
		Items:                     items,
		Status:                    models.OrderStatusDraft,
		TotalAmount:               totalAmount,
		Currency:                  currency,
		FormToken:                 &formToken,
		FormExpiresAt:             &formExpiresAt,
		Source:                    "api",
		SourcePlatform:            platform,
		ExternalUserID:            externalUserID,
		ExternalUserName:          userName, // 保存第三方平台的User名
		ExternalOrderID:           externalOrderID,
		UserEmail:                 userEmail,
		EmailNotificationsEnabled: true,
		Remark:                    remark,
	}

	if err := s.OrderRepo.Create(order); err != nil {
		return nil, err
	}

	return order, nil
}

// AdminOrderRequest 管理员创建订单请求
type AdminOrderRequest struct {
	UserID           *uint
	Items            []AdminOrderItem
	ReceiverName     string
	PhoneCode        string
	ReceiverPhone    string
	ReceiverEmail    string
	ReceiverCountry  string
	ReceiverProvince string
	ReceiverCity     string
	ReceiverDistrict string
	ReceiverAddress  string
	ReceiverPostcode string
	Remark           string
	AdminRemark      string
	Status           string
	TotalAmount      *int64
	UserEmail        string
}

// AdminOrderItem 管理员订单商品项
type AdminOrderItem struct {
	SKU                string                 `json:"sku"`
	Name               string                 `json:"name"`
	Quantity           int                    `json:"quantity"`
	UnitPrice          int64                  `json:"unit_price_minor"`
	Attributes         map[string]interface{} `json:"attributes,omitempty"`
	ProductType        string                 `json:"product_type,omitempty"`
	VirtualInventoryID *uint                  `json:"virtual_inventory_id,omitempty"`
}

// CreateAdminOrder 管理员直接创建订单
func (s *OrderService) CreateAdminOrder(req AdminOrderRequest) (*models.Order, error) {
	if len(req.Items) == 0 {
		return nil, bizerr.New("order.itemsEmpty", "Order items cannot be empty")
	}
	if len(req.Items) > s.cfg.Order.MaxOrderItems {
		return nil, bizerr.Newf("order.tooManyItems", "Order items cannot exceed %d", s.cfg.Order.MaxOrderItems).
			WithParams(map[string]interface{}{"max": s.cfg.Order.MaxOrderItems})
	}

	// 验证管理员覆盖金额
	if req.TotalAmount != nil && *req.TotalAmount < 0 {
		return nil, newOrderTotalAmountNegativeError()
	}

	// 验证用户
	if req.UserID != nil {
		user, err := s.userRepo.FindByID(*req.UserID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, newOrderUserNotFoundError()
			}
			return nil, err
		}
		if req.UserEmail == "" {
			req.UserEmail = user.Email
		}
	}

	// 构建订单商品并计算总金额
	var orderItems []models.OrderItem
	var totalAmount int64
	saleCountAdjustments := make(map[uint]int)
	// 保存每个商品项指定的虚拟库存ID（管理员手动选择）
	virtualInventoryIDs := make(map[int]*uint)
	for idx, item := range req.Items {
		sku := strings.TrimSpace(item.SKU)
		if sku == "" {
			return nil, bizerr.New("order.skuEmpty", "Product SKU cannot be empty")
		}
		if item.Quantity <= 0 {
			return nil, bizerr.New("order.quantityInvalid", "Quantity must be greater than 0")
		}
		if item.Quantity > s.cfg.Order.MaxItemQuantity {
			return nil, bizerr.Newf("order.quantityExceeded", "Quantity cannot exceed %d", s.cfg.Order.MaxItemQuantity).
				WithParams(map[string]interface{}{"max": s.cfg.Order.MaxItemQuantity})
		}

		name := item.Name
		var imageURL string
		productType := models.ProductType(item.ProductType)
		if name == "" || productType == "" {
			product, err := s.productRepo.FindBySKU(sku)
			if err != nil {
				if name == "" {
					return nil, bizerr.Newf("order.productNotFound", "Product %s does not exist", sku).
						WithParams(map[string]interface{}{"sku": sku})
				}
			} else {
				if name == "" {
					name = product.Name
				}
				if productType == "" {
					productType = product.ProductType
				}
				if len(product.Images) > 0 {
					imageURL = product.Images[0].URL
				}
			}
		}

		// 虚拟商品必须指定虚拟库存
		if productType == models.ProductTypeVirtual && item.VirtualInventoryID == nil {
			return nil, newOrderVirtualInventoryRequiredError(sku)
		}

		orderItems = append(orderItems, models.OrderItem{
			SKU:         sku,
			Name:        name,
			Quantity:    item.Quantity,
			Attributes:  item.Attributes,
			ProductType: productType,
			ImageURL:    imageURL,
		})
		totalAmount += item.UnitPrice * int64(item.Quantity)
		// 保存管理员指定的虚拟库存ID
		if item.VirtualInventoryID != nil {
			virtualInventoryIDs[idx] = item.VirtualInventoryID
		}
	}

	// 允许手动覆盖总金额
	if req.TotalAmount != nil {
		totalAmount = *req.TotalAmount
	}

	// 获取货币单位
	currency := s.cfg.Order.Currency
	if currency == "" {
		currency = "CNY"
	}

	// 生成订单号
	orderNo := utils.GenerateOrderNo(s.cfg.Order.NoPrefix)

	// 物理商品库存绑定
	inventoryBindings := make(map[int]uint)
	if s.bindingService != nil {
		for i := range orderItems {
			item := &orderItems[i]
			if item.ProductType == models.ProductTypeVirtual {
				continue
			}
			product, err := s.productRepo.FindBySKU(item.SKU)
			if err != nil {
				continue // 商品不在系统中，跳过库存绑定
			}

			attributesMap := make(map[string]string)
			if item.Attributes != nil {
				for k, v := range item.Attributes {
					if strVal, ok := v.(string); ok {
						attributesMap[k] = strVal
					}
				}
			}

			inventory, fullAttrs, invErr := s.bindingService.FindInventoryByAttributes(product.ID, attributesMap)
			if invErr != nil {
				continue // 没有匹配的库存配置，跳过
			}
			if canPurchase, _ := inventory.CanPurchase(item.Quantity); !canPurchase {
				continue // 库存不足，跳过
			}

			// 更新属性为完整属性
			for k, v := range fullAttrs {
				if item.Attributes == nil {
					item.Attributes = make(map[string]interface{})
				}
				item.Attributes[k] = v
			}
			if item.Attributes == nil {
				item.Attributes = make(map[string]interface{})
			}
			item.Attributes["_inventory_id"] = inventory.ID

			saleCountAdjustments[product.ID] += item.Quantity
		}
	}

	// 预留物理库存
	for i := range orderItems {
		item := &orderItems[i]
		if inventoryIDVal, ok := item.Attributes["_inventory_id"]; ok {
			if inventoryID, ok := inventoryIDVal.(uint); ok {
				reservedInventoryID, err := s.reserveInventoryWithHook(nil, req.UserID, orderNo, inventoryID, item.Quantity, "admin_create_order")
				if err != nil {
					// 回滚已预留的库存
					for j := 0; j < i; j++ {
						if prevID, exists := inventoryBindings[j]; exists {
							_ = s.releaseReservedInventoryWithHook(nil, req.UserID, orderNo, prevID, orderItems[j].Quantity, "admin_create_order_rollback")
						}
					}
					return nil, normalizeOrderInventoryOperationError(item.Name, err)
				}
				inventoryBindings[i] = reservedInventoryID
				delete(item.Attributes, "_inventory_id")
			}
		}
	}

	// 订单状态：默认待付款
	status := models.OrderStatusPendingPayment
	if req.Status != "" {
		validStatuses := map[models.OrderStatus]bool{
			models.OrderStatusPendingPayment: true,
			models.OrderStatusDraft:          true,
			models.OrderStatusPending:        true,
			models.OrderStatusNeedResubmit:   true,
			models.OrderStatusShipped:        true,
			models.OrderStatusCompleted:      true,
			models.OrderStatusCancelled:      true,
			models.OrderStatusRefunded:       true,
		}
		requested := models.OrderStatus(req.Status)
		if !validStatuses[requested] {
			return nil, newOrderStatusInvalidError(req.Status)
		}
		status = requested
	}

	// 生成表单Token（用于后续用户填写收货信息）
	// 仅实物/混合订单需要表单，虚拟商品订单不需要收货信息
	var formToken *string
	var formExpiresAt *time.Time
	hasShippingInfo := req.ReceiverName != "" && req.ReceiverAddress != ""
	isVirtualOnly := true
	for _, item := range orderItems {
		if item.ProductType != models.ProductTypeVirtual {
			isVirtualOnly = false
			break
		}
	}
	if !hasShippingInfo && !isVirtualOnly {
		token := uuid.New().String()
		formToken = &token
		expires := models.NowFunc().Add(time.Duration(s.cfg.Form.ExpireHours) * time.Hour)
		formExpiresAt = &expires
	}

	order := &models.Order{
		OrderNo:                   orderNo,
		UserID:                    req.UserID,
		Items:                     orderItems,
		InventoryBindings:         inventoryBindings,
		Status:                    status,
		TotalAmount:               totalAmount,
		Currency:                  currency,
		FormToken:                 formToken,
		FormExpiresAt:             formExpiresAt,
		Source:                    "admin",
		ReceiverName:              req.ReceiverName,
		PhoneCode:                 req.PhoneCode,
		ReceiverPhone:             req.ReceiverPhone,
		ReceiverEmail:             req.ReceiverEmail,
		ReceiverCountry:           req.ReceiverCountry,
		ReceiverProvince:          req.ReceiverProvince,
		ReceiverCity:              req.ReceiverCity,
		ReceiverDistrict:          req.ReceiverDistrict,
		ReceiverAddress:           req.ReceiverAddress,
		ReceiverPostcode:          req.ReceiverPostcode,
		UserEmail:                 req.UserEmail,
		EmailNotificationsEnabled: req.UserEmail != "",
		Remark:                    req.Remark,
		AdminRemark:               req.AdminRemark,
	}

	if err := s.OrderRepo.Create(order); err != nil {
		// 释放已预留的物理库存
		for i, inventoryID := range inventoryBindings {
			_ = s.releaseReservedInventoryWithHook(nil, req.UserID, orderNo, inventoryID, orderItems[i].Quantity, "admin_create_order_rollback")
		}
		return nil, err
	}
	createdOrderID := order.ID

	// 虚拟产品预留库存（待付款状态，付款后才发货）
	virtualInventoryBindings := make(map[int]uint)
	if s.virtualProductSvc != nil {
		for i := range orderItems {
			item := &orderItems[i]
			if item.ProductType != models.ProductTypeVirtual {
				continue
			}

			// 如果管理员指定了虚拟库存ID，直接从该库存池分配
			if vid, ok := virtualInventoryIDs[i]; ok && vid != nil {
				_, scriptInvID, err := s.virtualProductSvc.AllocateStockFromInventory(*vid, item.Quantity, orderNo)
				if err != nil {
					// 分配失败，回滚物理库存和订单
					if releaseErr := s.virtualProductSvc.ReleaseStock(orderNo); releaseErr != nil {
						fmt.Printf("Warning: Failed to rollback virtual stock for order %s: %v\n", orderNo, releaseErr)
					}
					for j, inventoryID := range inventoryBindings {
						_ = s.releaseReservedInventoryWithHook(&createdOrderID, req.UserID, orderNo, inventoryID, orderItems[j].Quantity, "admin_create_order_rollback")
					}
					s.OrderRepo.Delete(order.ID)
					return nil, fmt.Errorf("failed to allocate virtual product stock for %s: %w", item.Name, err)
				}
				if scriptInvID != nil {
					virtualInventoryBindings[i] = *scriptInvID
				}
			} else {
				// 未指定虚拟库存ID，尝试通过商品绑定自动分配
				product, err := s.productRepo.FindBySKU(item.SKU)
				if err != nil {
					continue // 商品不在系统中，跳过虚拟库存绑定
				}

				allocAttrs := make(map[string]interface{})
				for k, v := range item.Attributes {
					allocAttrs[k] = v
				}
				_, scriptInvID, err := s.virtualProductSvc.AllocateStockForProductByAttributes(product.ID, item.Quantity, orderNo, allocAttrs)
				if err != nil {
					// 分配失败，回滚物理库存和订单
					if releaseErr := s.virtualProductSvc.ReleaseStock(orderNo); releaseErr != nil {
						fmt.Printf("Warning: Failed to rollback virtual stock for order %s: %v\n", orderNo, releaseErr)
					}
					for j, inventoryID := range inventoryBindings {
						_ = s.releaseReservedInventoryWithHook(&createdOrderID, req.UserID, orderNo, inventoryID, orderItems[j].Quantity, "admin_create_order_rollback")
					}
					s.OrderRepo.Delete(order.ID)
					return nil, fmt.Errorf("failed to allocate virtual product stock for %s: %w", item.Name, err)
				}
				if scriptInvID != nil {
					virtualInventoryBindings[i] = *scriptInvID
				}
			}

			// 更新虚拟商品销量
			product, err := s.productRepo.FindBySKU(item.SKU)
			if err == nil {
				saleCountAdjustments[product.ID] += item.Quantity
			}
		}
	}

	// 保存脚本类型虚拟库存绑定
	if len(virtualInventoryBindings) > 0 {
		order.VirtualInventoryBindings = virtualInventoryBindings
		s.OrderRepo.Update(order)
	}
	for productID, quantity := range saleCountAdjustments {
		if err := s.productRepo.IncrementSaleCount(productID, quantity); err != nil {
			fmt.Printf("Warning: Failed to update product sales count - ProductID: %d, Error: %v\n", productID, err)
		}
	}
	syncUserPurchaseStatsTransitionBestEffort(s.OrderRepo, nil, order.UserID, "", order.Status, order.Items, "create_admin_order")
	s.syncUserConsumptionStatusTransitionBestEffort(order.UserID, "", order.Status, order.TotalAmount, "create_admin_order")

	// 发送订单创建通知邮件
	if s.emailService != nil {
		go s.emailService.SendOrderCreatedEmail(order)
	}

	return order, nil
}

// CreateUserOrder User直接CreateOrder（无需表单流程）
func (s *OrderService) CreateUserOrder(userID uint, items []models.OrderItem, remark string, promoCode string) (*models.Order, error) {
	releaseHotPath, err := acquireOrderHighConcurrencyProtection(s.cfg, orderHotPathCreateUserOrder)
	if err != nil {
		if isOrderHighConcurrencyBusyError(err) {
			return nil, newOrderHighConcurrencyBusyError()
		}
		return nil, err
	}
	defer releaseHotPath()

	// 查找UserInfo
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, newOrderUserNotFoundError()
		}
		return nil, err
	}

	unlock := s.lockUserOrderCreation(userID)
	defer unlock()

	if err := s.ensurePendingPaymentLimit(userID); err != nil {
		return nil, err
	}

	// 校验订单商品项
	if err := s.validateOrderItems(items); err != nil {
		return nil, err
	}

	productBySKU, err := s.loadProductsForOrderItems(items)
	if err != nil {
		return nil, err
	}

	// 盲盒属性跟踪：记录每个订单项中盲盒随机分配的属性名
	// key: 订单项索引, value: 盲盒属性名列表
	blindBoxAttrNames := make(map[int][]string)
	saleCountAdjustments := make(map[uint]int)

	// 购买限制：同一SKU可能以多条订单项出现（不同属性/规格）
	// 需要累计本次订单中该SKU的总数量，避免“拆成多行”绕过限购。
	requestedQtyBySKU := make(map[string]int)
	for _, item := range items {
		product, exists := productBySKU[item.SKU]
		if !exists || product == nil {
			return nil, bizerr.Newf("order.productNotFound", "Product %s does not exist", item.SKU).
				WithParams(map[string]interface{}{"sku": item.SKU})
		}
		if product.Status != models.ProductStatusActive {
			return nil, ErrProductNotAvailable
		}
		if product.MaxPurchaseLimit > 0 {
			requestedQtyBySKU[item.SKU] += item.Quantity
		}
	}

	purchasedQtyBySKU, err := s.OrderRepo.GetUserPurchaseQuantityBySKUs(userID, collectRequestedSKUs(requestedQtyBySKU))
	if err != nil {
		return nil, fmt.Errorf("Failed to query purchase records: %v", err)
	}
	for sku, requestedQty := range requestedQtyBySKU {
		product := productBySKU[sku]
		if product == nil || product.MaxPurchaseLimit <= 0 {
			continue
		}
		purchasedQty := purchasedQtyBySKU[sku]
		if purchasedQty+requestedQty <= product.MaxPurchaseLimit {
			continue
		}
		remaining := product.MaxPurchaseLimit - purchasedQty
		if remaining <= 0 {
			return nil, bizerr.Newf("order.purchaseLimitReached",
				"Product %s has reached purchase limit (maximum %d per account)", product.Name, product.MaxPurchaseLimit).
				WithParams(map[string]interface{}{"product": product.Name, "limit": product.MaxPurchaseLimit})
		}
		return nil, bizerr.Newf("order.purchaseLimitExceeded",
			"Product %s purchase quantity exceeds limit, you can still purchase %d (maximum %d per account)",
			product.Name, remaining, product.MaxPurchaseLimit).
			WithParams(map[string]interface{}{"product": product.Name, "remaining": remaining, "limit": product.MaxPurchaseLimit})
	}

	// 验证Product并处理Inventory（使用新的Inventory绑定机制）
	for i := range items {
		item := &items[i]

		// 根据 SKU 查找Product
		product := productBySKU[item.SKU]

		// 收集盲盒属性名，并从用户输入中剔除（防止用户手动指定盲盒结果）
		var bbAttrNames []string
		for _, attr := range product.Attributes {
			if attr.Mode == models.AttributeModeBlindBox {
				bbAttrNames = append(bbAttrNames, attr.Name)
			}
		}
		if item.Attributes == nil {
			item.Attributes = make(map[string]interface{})
		}
		for _, name := range bbAttrNames {
			delete(item.Attributes, name)
		}

		// 新的Inventory处理逻辑：根据Product的Inventory模式和User选择的属性查找对应的Inventory
		var inventory *models.Inventory
		var inventoryErr error

		// 检查是否为虚拟商品
		if product.ProductType == models.ProductTypeVirtual {
			// 保存商品类型到订单项
			item.ProductType = models.ProductTypeVirtual

			// 虚拟商品盲盒处理逻辑
			hasBlindBox := len(bbAttrNames) > 0
			hasUserSelect := false
			for _, attr := range product.Attributes {
				if attr.Mode != models.AttributeModeBlindBox {
					hasUserSelect = true
					break
				}
			}

			// 转换属性类型（已剔除盲盒属性）
			attrStrMap := make(map[string]string)
			for k, v := range item.Attributes {
				if str, ok := v.(string); ok {
					attrStrMap[k] = str
				}
			}

			// 根据盲盒模式处理虚拟商品
			if s.virtualProductSvc != nil {
				if hasBlindBox {
					// 盲盒模式 或 混合模式
					if hasUserSelect && len(attrStrMap) > 0 {
						// 混合模式：部分属性用户选择，部分属性盲盒随机
						_, fullAttrs, err := s.virtualProductSvc.FindVirtualInventoryWithPartialMatch(product.ID, attrStrMap, item.Quantity)
						if err != nil {
							return nil, fmt.Errorf("failed to allocate virtual inventory for product %s: %w", product.Name, err)
						}
						// 更新订单项的属性为完整属性（包括随机分配的）
						for k, v := range fullAttrs {
							item.Attributes[k] = v
						}
					} else {
						// 纯盲盒模式：全部随机
						_, fullAttrs, err := s.virtualProductSvc.SelectRandomVirtualInventory(product.ID, item.Quantity)
						if err != nil {
							return nil, fmt.Errorf("failed to allocate virtual inventory for product %s: %w", product.Name, err)
						}
						// 更新订单项的属性为完整属性（随机分配的）
						for k, v := range fullAttrs {
							item.Attributes[k] = v
						}
					}
					// 记录盲盒属性名，后续从items中剥离
					blindBoxAttrNames[i] = bbAttrNames
				} else {
					var availableCount int64
					var err error
					if len(attrStrMap) > 0 {
						// 根据规格属性检查库存
						availableCount, err = s.virtualProductSvc.GetAvailableCountForProductByAttributes(product.ID, attrStrMap)
					} else {
						// 无规格属性，检查总库存
						availableCount, err = s.virtualProductSvc.GetAvailableCountForProduct(product.ID)
					}
					if err != nil {
						return nil, fmt.Errorf("Failed to check virtual product stock: %v", err)
					}
					if availableCount < int64(item.Quantity) {
						return nil, bizerr.Newf("order.stockInsufficient",
							"Virtual product %s stock insufficient, only %d available", product.Name, availableCount).
							WithParams(map[string]interface{}{"product": product.Name, "available": availableCount})
					}
				}
			}

			saleCountAdjustments[product.ID] += item.Quantity
			// 虚拟商品不需要处理物理库存绑定
			continue
		}

		// 保存商品类型到订单项（实物商品）
		item.ProductType = models.ProductTypePhysical

		// 物理商品的库存处理
		// 将 item.Attributes (map[string]interface{}) 转换为 map[string]string（已剔除盲盒属性）
		attributesMap := make(map[string]string)
		for k, v := range item.Attributes {
			if strVal, ok := v.(string); ok {
				attributesMap[k] = strVal
			}
		}

		// 检查Product是否有盲盒属性
		hasBlindBox := len(bbAttrNames) > 0
		hasUserSelect := false
		for _, attr := range product.Attributes {
			if attr.Mode != models.AttributeModeBlindBox {
				hasUserSelect = true
				break
			}
		}

		if product.InventoryMode == string(models.InventoryModeRandom) || hasBlindBox {
			// 盲盒模式 或 混合模式（有盲盒属性）
			if hasUserSelect && len(attributesMap) > 0 {
				// 混合模式：部分属性User选择，部分属性盲盒随机
				var fullAttrs map[string]string
				inventory, fullAttrs, inventoryErr = s.bindingService.FindInventoryWithPartialMatch(product.ID, attributesMap, item.Quantity)
				if inventoryErr != nil {
					return nil, fmt.Errorf("failed to allocate inventory for product %s: %w", product.Name, inventoryErr)
				}
				// UpdateOrder项的属性为完整属性（包括随机分配的）
				for k, v := range fullAttrs {
					item.Attributes[k] = v
				}
			} else {
				// 纯盲盒模式：全部随机
				var fullAttrs map[string]string
				inventory, fullAttrs, inventoryErr = s.bindingService.SelectRandomInventory(product.ID, item.Quantity)
				if inventoryErr != nil {
					return nil, fmt.Errorf("failed to allocate inventory for product %s: %w", product.Name, inventoryErr)
				}
				// UpdateOrder项的属性为完整属性（随机分配的）
				for k, v := range fullAttrs {
					item.Attributes[k] = v
				}
			}
			// 记录盲盒属性名，后续从items中剥离
			blindBoxAttrNames[i] = bbAttrNames
		} else {
			var fullAttrs map[string]string
			inventory, fullAttrs, inventoryErr = s.bindingService.FindInventoryByAttributes(product.ID, attributesMap)
			if inventoryErr != nil {
				return nil, fmt.Errorf("find inventory for product %s: %w", product.Name, inventoryErr)
			}
			// UpdateOrder项的属性为完整属性
			for k, v := range fullAttrs {
				item.Attributes[k] = v
			}
		}

		// 检查Inventory是否足够
		if canPurchase, msg := inventory.CanPurchase(item.Quantity); !canPurchase {
			if normalized := normalizeOrderInventoryAvailabilityError(product.Name, msg); normalized != nil {
				return nil, normalized
			}
			return nil, fmt.Errorf("product %s %s", product.Name, msg)
		}

		// 预留Inventory（generateOrder号后Update）
		// 注意：这里先记录need预留的InventoryID，CreateOrder后再调用预留
		// 使用 _inventory_id 作为临时标记，预留Success后会改为 inventory_id 永久保存
		item.Attributes["_inventory_id"] = inventory.ID

		saleCountAdjustments[product.ID] += item.Quantity
	}

	// generateOrder号
	orderNo := utils.GenerateOrderNo(s.cfg.Order.NoPrefix)

	// Inventory绑定映射（Order项索引 -> InventoryID）
	inventoryBindings := make(map[int]uint)

	// 预留Inventory（在CreateOrder前）
	for i := range items {
		item := &items[i]

		if inventoryIDVal, ok := item.Attributes["_inventory_id"]; ok {
			if inventoryID, ok := inventoryIDVal.(uint); ok {
				// 预留Inventory
				reservedInventoryID, err := s.reserveInventoryWithHook(nil, &userID, orderNo, inventoryID, item.Quantity, "user_create_order")
				if err != nil {
					// 预留Failed，need回滚之前已预留的Inventory
					for j := 0; j < i; j++ {
						if prevInvID, exists := inventoryBindings[j]; exists {
							_ = s.releaseReservedInventoryWithHook(nil, &userID, orderNo, prevInvID, items[j].Quantity, "user_create_order_rollback")
						}
					}
					return nil, normalizeOrderInventoryOperationError(item.Name, err)
				}
				// 保存Inventory绑定关系（使用独立的映射表，不污染Product属性）
				inventoryBindings[i] = reservedInventoryID
				// 从属性中移除临时标记
				delete(item.Attributes, "_inventory_id")
			}
		}
	}

	// 将盲盒分配结果提取到 ActualAttributes，并从 items 中剥离盲盒属性
	// ActualAttributes 格式: { "0": {"color": "red"}, "1": {"size": "L"} } (key 为订单项索引)
	var actualAttrsJSON models.JSON
	if len(blindBoxAttrNames) > 0 {
		actualAttrsMap := make(map[string]map[string]interface{})
		for idx, attrNames := range blindBoxAttrNames {
			item := &items[idx]
			blindBoxValues := make(map[string]interface{})
			for _, name := range attrNames {
				if val, ok := item.Attributes[name]; ok {
					blindBoxValues[name] = val
					delete(item.Attributes, name)
				}
			}
			if len(blindBoxValues) > 0 {
				actualAttrsMap[fmt.Sprintf("%d", idx)] = blindBoxValues
			}
		}
		if len(actualAttrsMap) > 0 {
			jsonBytes, _ := json.Marshal(actualAttrsMap)
			actualAttrsJSON = models.JSON(string(jsonBytes))
		}
	}

	// CreateOrder
	// 所有订单创建时都是待付款状态
	orderStatus := models.OrderStatusPendingPayment

	// 计算订单总金额
	var totalAmount int64
	for _, item := range items {
		if product := productBySKU[item.SKU]; product != nil {
			totalAmount += product.Price * int64(item.Quantity)
		}
	}

	// 获取货币单位
	currency := s.cfg.Order.Currency
	if currency == "" {
		currency = "CNY"
	}

	// 处理优惠码
	var promoCodeID *uint
	var promoCodeStr string
	var discountAmount int64
	if promoCode != "" && s.promoCodeRepo != nil {
		promoCodeRepo := s.promoCodeRepo
		pc, err := promoCodeRepo.FindByCode(strings.ToUpper(strings.TrimSpace(promoCode)))
		if err != nil {
			// 释放已预留的库存
			for i, inventoryID := range inventoryBindings {
				_ = s.releaseReservedInventoryWithHook(nil, &userID, orderNo, inventoryID, items[i].Quantity, "user_create_order_rollback")
			}
			return nil, fmt.Errorf("Promo code not found")
		}
		if !pc.IsAvailable() {
			for i, inventoryID := range inventoryBindings {
				_ = s.releaseReservedInventoryWithHook(nil, &userID, orderNo, inventoryID, items[i].Quantity, "user_create_order_rollback")
			}
			return nil, fmt.Errorf("Promo code is not available")
		}
		// 收集订单中的商品ID
		var productIDs []uint
		for _, item := range items {
			if product := productBySKU[item.SKU]; product != nil {
				productIDs = append(productIDs, product.ID)
			}
		}
		// 检查优惠码是否适用于订单中的商品
		if len(pc.ProductIDs) > 0 {
			applicable := false
			for _, pid := range productIDs {
				if pc.IsApplicableToProduct(pid) {
					applicable = true
					break
				}
			}
			if !applicable {
				for i, inventoryID := range inventoryBindings {
					_ = s.releaseReservedInventoryWithHook(nil, &userID, orderNo, inventoryID, items[i].Quantity, "user_create_order_rollback")
				}
				return nil, fmt.Errorf("Promo code is not applicable to the selected products")
			}
		}
		discountAmount = pc.CalculateDiscount(totalAmount)
		// 预留优惠码
		if err := promoCodeRepo.Reserve(pc.ID, orderNo); err != nil {
			for i, inventoryID := range inventoryBindings {
				_ = s.releaseReservedInventoryWithHook(nil, &userID, orderNo, inventoryID, items[i].Quantity, "user_create_order_rollback")
			}
			return nil, fmt.Errorf("Failed to reserve promo code: %v", err)
		}
		promoCodeID = &pc.ID
		promoCodeStr = pc.Code
	}

	order := &models.Order{
		OrderNo:                   orderNo,
		UserID:                    &userID,
		Items:                     items,
		ActualAttributes:          actualAttrsJSON,
		InventoryBindings:         inventoryBindings, // 保存Inventory绑定关系（内部使用）
		Status:                    orderStatus,
		TotalAmount:               totalAmount - discountAmount,
		Currency:                  currency,
		PromoCodeID:               promoCodeID,
		PromoCodeStr:              promoCodeStr,
		DiscountAmount:            discountAmount,
		Source:                    "web",
		UserEmail:                 user.Email,
		EmailNotificationsEnabled: true,
		Remark:                    remark,
		// FormToken 和 FormExpiresAt 在User点击填写时动态generate（仅非虚拟商品订单需要）
	}

	if err := s.OrderRepo.WithTransaction(func(tx *gorm.DB) error {
		if err := s.ensurePendingPaymentLimitTx(tx, userID); err != nil {
			return err
		}
		if err := s.ensurePurchaseLimitsTx(tx, userID, requestedQtyBySKU); err != nil {
			return err
		}
		if err := tx.Create(order).Error; err != nil {
			return err
		}
		return applyUserPurchaseStatsTransitionTx(tx, nil, order.UserID, "", order.Status, order.Items)
	}); err != nil {
		// CreateOrderFailed，释放已预留的Inventory
		for i, inventoryID := range inventoryBindings {
			_ = s.releaseReservedInventoryWithHook(nil, &userID, orderNo, inventoryID, items[i].Quantity, "user_create_order_rollback")
		}
		// 释放优惠码
		if promoCodeID != nil && s.promoCodeRepo != nil {
			s.promoCodeRepo.ReleaseReserve(*promoCodeID, orderNo)
		}
		return nil, err
	}

	// 虚拟产品预留库存（待付款状态，付款后才发货）
	userVirtualInventoryBindings := make(map[int]uint)
	if s.virtualProductSvc != nil {
		for i := range items {
			item := &items[i]
			product := productBySKU[item.SKU]
			if product != nil && product.ProductType == models.ProductTypeVirtual {
				// 为虚拟产品分配库存（预留状态），传入完整规格属性
				// 需要从 ActualAttributes 中合并盲盒属性回来用于库存匹配
				allocAttrs := make(map[string]interface{})
				for k, v := range item.Attributes {
					allocAttrs[k] = v
				}
				if bbNames, ok := blindBoxAttrNames[i]; ok && len(actualAttrsJSON) > 0 {
					var actualMap map[string]map[string]interface{}
					if err := json.Unmarshal([]byte(actualAttrsJSON), &actualMap); err == nil {
						if bbVals, ok := actualMap[fmt.Sprintf("%d", i)]; ok {
							for _, name := range bbNames {
								if v, ok := bbVals[name]; ok {
									allocAttrs[name] = v
								}
							}
						}
					}
				}
				_, scriptInvID, err := s.virtualProductSvc.AllocateStockForProductByAttributes(product.ID, item.Quantity, orderNo, allocAttrs)
				if err != nil {
					// 分配失败，需要回滚订单和已分配的物理库存
					if releaseErr := s.virtualProductSvc.ReleaseStock(orderNo); releaseErr != nil {
						fmt.Printf("Warning: Failed to rollback virtual stock for order %s: %v\n", orderNo, releaseErr)
					}
					for j, inventoryID := range inventoryBindings {
						orderIDForHook := order.ID
						_ = s.releaseReservedInventoryWithHook(&orderIDForHook, &userID, orderNo, inventoryID, items[j].Quantity, "user_create_order_rollback")
					}
					if promoCodeID != nil && s.promoCodeRepo != nil {
						if releaseErr := s.promoCodeRepo.ReleaseReserve(*promoCodeID, orderNo); releaseErr != nil {
							fmt.Printf("Warning: Failed to rollback promo code reserve for order %s: %v\n", orderNo, releaseErr)
						}
					}
					s.OrderRepo.Delete(order.ID)
					return nil, fmt.Errorf("failed to allocate virtual product stock: %w", err)
				}
				if scriptInvID != nil {
					userVirtualInventoryBindings[i] = *scriptInvID
				}
			}
		}
		// 注意：待付款状态不自动发货，需要管理员标记付款后才发货
	}

	// 保存脚本类型虚拟库存绑定
	if len(userVirtualInventoryBindings) > 0 {
		order.VirtualInventoryBindings = userVirtualInventoryBindings
		s.OrderRepo.Update(order)
	}

	for productID, quantity := range saleCountAdjustments {
		if err := s.productRepo.IncrementSaleCount(productID, quantity); err != nil {
			fmt.Printf("Warning: Failed to update product sales count - ProductID: %d, Error: %v\n", productID, err)
		}
	}

	// 零金额订单自动完成支付（如100%优惠码或价格为0的商品）
	if order.TotalAmount == 0 {
		s.MarkAsPaid(order.ID)
		order, _ = s.OrderRepo.FindByID(order.ID)
		return order, nil
	}

	// 发送订单创建通知邮件
	if s.emailService != nil {
		go s.emailService.SendOrderCreatedEmail(order)
	}

	return order, nil
}

// SubmitShippingForm 提交发货Info表单
func (s *OrderService) SubmitShippingForm(formToken string, receiverInfo map[string]interface{}, privacyProtected bool, userPassword string, userRemark string, actorUserID *uint) (*models.Order, *models.User, bool, error) {
	type preparedNewUserCredentials struct {
		normalizedEmail string
		passwordHash    string
	}

	var preparedNewUser *preparedNewUserCredentials
	if actorUserID == nil && s.userRepo != nil {
		normalizedEmail := strings.ToLower(strings.TrimSpace(receiverInfo["receiver_email"].(string)))
		if normalizedEmail != "" {
			_, findErr := s.userRepo.FindByEmail(normalizedEmail)
			switch {
			case findErr == nil:
				// User already exists; keep historical behavior and ignore the provided password.
			case errors.Is(findErr, gorm.ErrRecordNotFound):
				generatedPassword := userPassword
				if generatedPassword == "" {
					var genErr error
					generatedPassword, genErr = password.GenerateRandomPassword(12)
					if genErr != nil {
						return nil, nil, false, genErr
					}
				}

				policy := s.cfg.Security.PasswordPolicy
				if err := password.ValidatePasswordPolicy(generatedPassword, policy.MinLength, policy.RequireUppercase,
					policy.RequireLowercase, policy.RequireNumber, policy.RequireSpecial); err != nil {
					if bizErr := password.ToBizError(err); bizErr != nil {
						return nil, nil, false, bizErr
					}
					return nil, nil, false, err
				}

				hashedPassword, hashErr := password.HashPassword(generatedPassword)
				if hashErr != nil {
					return nil, nil, false, hashErr
				}
				preparedNewUser = &preparedNewUserCredentials{
					normalizedEmail: normalizedEmail,
					passwordHash:    hashedPassword,
				}
			default:
				return nil, nil, false, findErr
			}
		}
	}

	releaseHotPath, err := acquireOrderHighConcurrencyProtection(s.cfg, orderHotPathSubmitShippingForm)
	if err != nil {
		if isOrderHighConcurrencyBusyError(err) {
			return nil, nil, false, newOrderHighConcurrencyBusyError()
		}
		return nil, nil, false, err
	}
	defer releaseHotPath()

	var (
		order             *models.Order
		user              *models.User
		isNewUser         bool
		isResubmit        bool
		statusHookBefore  models.OrderStatus
		serialHookSerials []models.ProductSerial
		serialTaskQueued  bool
	)

	err = s.OrderRepo.WithTransaction(func(tx *gorm.DB) error {
		txOrderRepo := repository.NewOrderRepository(tx)
		lockedOrder, err := txOrderRepo.FindByFormTokenForUpdate(tx, formToken)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrShippingFormNotFound
			}
			return err
		}
		if actorUserID != nil && lockedOrder.UserID != nil && *lockedOrder.UserID != *actorUserID {
			return ErrShippingFormAccessDenied
		}

		isResubmit = lockedOrder.Status == models.OrderStatusNeedResubmit
		if lockedOrder.Status != models.OrderStatusDraft && lockedOrder.Status != models.OrderStatusNeedResubmit {
			return errors.New("Order is not ready for shipping information submission")
		}
		if lockedOrder.FormSubmittedAt != nil && !isResubmit {
			return errors.New("Form has already been submitted")
		}
		if lockedOrder.FormExpiresAt != nil && models.NowFunc().After(*lockedOrder.FormExpiresAt) {
			return errors.New("Form has expired")
		}

		beforeStatus := lockedOrder.Status
		beforeUserID := lockedOrder.UserID
		statusHookBefore = beforeStatus

		lockedOrder.ReceiverName = receiverInfo["receiver_name"].(string)
		if phoneCode, ok := receiverInfo["phone_code"].(string); ok {
			lockedOrder.PhoneCode = phoneCode
		}
		lockedOrder.ReceiverPhone = receiverInfo["receiver_phone"].(string)
		lockedOrder.ReceiverEmail = receiverInfo["receiver_email"].(string)
		if country, ok := receiverInfo["receiver_country"].(string); ok {
			lockedOrder.ReceiverCountry = country
		}
		if province, ok := receiverInfo["receiver_province"].(string); ok {
			lockedOrder.ReceiverProvince = province
		}
		if city, ok := receiverInfo["receiver_city"].(string); ok {
			lockedOrder.ReceiverCity = city
		}
		if district, ok := receiverInfo["receiver_district"].(string); ok {
			lockedOrder.ReceiverDistrict = district
		}
		lockedOrder.ReceiverAddress = receiverInfo["receiver_address"].(string)
		if postcode, ok := receiverInfo["receiver_postcode"].(string); ok {
			lockedOrder.ReceiverPostcode = postcode
		}
		lockedOrder.PrivacyProtected = privacyProtected

		if userRemark != "" {
			if lockedOrder.Remark != "" {
				lockedOrder.Remark = lockedOrder.Remark + "\n\n[User Remark]\n" + userRemark
			} else {
				lockedOrder.Remark = userRemark
			}
		}

		lockedOrder.Status = models.OrderStatusPending
		now := models.NowFunc()
		lockedOrder.FormSubmittedAt = &now

		if isResubmit {
			if lockedOrder.UserID == nil {
				return errors.New("Resubmit order missing user association")
			}
			var existingUser models.User
			if err := tx.First(&existingUser, *lockedOrder.UserID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return errors.New("Cannot find user associated with order")
				}
				return err
			}
			user = &existingUser
			isNewUser = false
		} else {
			if actorUserID != nil {
				var existingUser models.User
				if err := tx.First(&existingUser, *actorUserID).Error; err != nil {
					if errors.Is(err, gorm.ErrRecordNotFound) {
						return newOrderUserNotFoundError()
					}
					return err
				}
				user = &existingUser
				isNewUser = false
			} else {
				normalizedEmail := strings.ToLower(strings.TrimSpace(lockedOrder.ReceiverEmail))
				lockedOrder.ReceiverEmail = normalizedEmail

				var existingUser models.User
				findErr := tx.Where("email = ?", normalizedEmail).First(&existingUser).Error
				switch {
				case findErr == nil:
					user = &existingUser
					isNewUser = false
				case errors.Is(findErr, gorm.ErrRecordNotFound):
					isNewUser = true
					hashedPassword := ""
					if preparedNewUser != nil && preparedNewUser.normalizedEmail == normalizedEmail {
						hashedPassword = preparedNewUser.passwordHash
					} else {
						generatedPassword := userPassword
						if generatedPassword == "" {
							generatedPassword, err = password.GenerateRandomPassword(12)
							if err != nil {
								return err
							}
						}

						policy := s.cfg.Security.PasswordPolicy
						if err := password.ValidatePasswordPolicy(generatedPassword, policy.MinLength, policy.RequireUppercase,
							policy.RequireLowercase, policy.RequireNumber, policy.RequireSpecial); err != nil {
							if bizErr := password.ToBizError(err); bizErr != nil {
								return bizErr
							}
							return err
						}

						hashedPassword, err = password.HashPassword(generatedPassword)
						if err != nil {
							return err
						}
					}

					createdUser := &models.User{
						UUID:                 uuid.New().String(),
						Email:                normalizedEmail,
						Name:                 lockedOrder.ReceiverName,
						PasswordHash:         hashedPassword,
						Role:                 "user",
						IsActive:             true,
						EmailNotifyMarketing: true,
						SMSNotifyMarketing:   true,
					}
					if lockedOrder.ReceiverPhone != "" {
						createdUser.Phone = &lockedOrder.ReceiverPhone
					}
					if err := tx.Create(createdUser).Error; err != nil {
						if !isUniqueConstraintError(err) {
							return err
						}
						if err := tx.Where("email = ?", normalizedEmail).First(&existingUser).Error; err != nil {
							return err
						}
						user = &existingUser
						isNewUser = false
					} else {
						user = createdUser
					}
				default:
					return findErr
				}
			}

			lockedOrder.UserID = &user.ID
		}

		userUpdateValue := interface{}(nil)
		if lockedOrder.UserID != nil {
			userUpdateValue = *lockedOrder.UserID
		}

		orderUpdates := map[string]interface{}{
			"receiver_name":     lockedOrder.ReceiverName,
			"phone_code":        lockedOrder.PhoneCode,
			"receiver_phone":    lockedOrder.ReceiverPhone,
			"receiver_email":    lockedOrder.ReceiverEmail,
			"receiver_country":  lockedOrder.ReceiverCountry,
			"receiver_province": lockedOrder.ReceiverProvince,
			"receiver_city":     lockedOrder.ReceiverCity,
			"receiver_district": lockedOrder.ReceiverDistrict,
			"receiver_address":  lockedOrder.ReceiverAddress,
			"receiver_postcode": lockedOrder.ReceiverPostcode,
			"privacy_protected": lockedOrder.PrivacyProtected,
			"remark":            lockedOrder.Remark,
			"status":            lockedOrder.Status,
			"form_submitted_at": lockedOrder.FormSubmittedAt,
			"user_id":           userUpdateValue,
		}
		if err := tx.Model(lockedOrder).Updates(orderUpdates).Error; err != nil {
			return err
		}
		if err := applyUserPurchaseStatsTransitionTx(tx, beforeUserID, lockedOrder.UserID, beforeStatus, lockedOrder.Status, lockedOrder.Items); err != nil {
			return err
		}

		if !isResubmit && s.serialService != nil {
			txProductRepo := repository.NewProductRepository(tx)
			productBySKU, findErr := txProductRepo.FindBySKUs(collectOrderItemSKUs(lockedOrder.Items))
			if findErr != nil {
				return findErr
			}

			hasSerialEligibleProduct := false
			for i := range lockedOrder.Items {
				item := &lockedOrder.Items[i]
				product := productBySKU[item.SKU]
				if product == nil || product.ProductCode == "" || product.ProductType != models.ProductTypePhysical || item.Quantity <= 0 {
					continue
				}
				hasSerialEligibleProduct = true
				break
			}

			switch {
			case !hasSerialEligibleProduct:
				lockedOrder.SerialGenerationStatus = models.SerialGenerationStatusNotRequired
				lockedOrder.SerialGenerationError = ""
				lockedOrder.SerialGeneratedAt = nil
				if err := tx.Model(lockedOrder).Updates(map[string]interface{}{
					"serial_generation_status": lockedOrder.SerialGenerationStatus,
					"serial_generation_error":  lockedOrder.SerialGenerationError,
					"serial_generated_at":      lockedOrder.SerialGeneratedAt,
				}).Error; err != nil {
					return err
				}
			case s.serialTaskService != nil:
				lockedOrder.SerialGenerationStatus = models.SerialGenerationStatusQueued
				lockedOrder.SerialGenerationError = ""
				lockedOrder.SerialGeneratedAt = nil
				if err := tx.Model(lockedOrder).Updates(map[string]interface{}{
					"serial_generation_status": lockedOrder.SerialGenerationStatus,
					"serial_generation_error":  lockedOrder.SerialGenerationError,
					"serial_generated_at":      lockedOrder.SerialGeneratedAt,
				}).Error; err != nil {
					return err
				}
				if err := s.serialTaskService.EnqueueOrderTx(tx, lockedOrder.ID); err != nil {
					return err
				}
				serialTaskQueued = true
			default:
				createdSerials, _, createErr := s.serialService.createMissingOrderSerialsTx(tx, lockedOrder)
				if createErr != nil {
					return createErr
				}
				completedAt := models.NowFunc()
				lockedOrder.SerialGenerationStatus = models.SerialGenerationStatusCompleted
				lockedOrder.SerialGenerationError = ""
				lockedOrder.SerialGeneratedAt = &completedAt
				if err := tx.Model(lockedOrder).Updates(map[string]interface{}{
					"serial_generation_status": lockedOrder.SerialGenerationStatus,
					"serial_generation_error":  lockedOrder.SerialGenerationError,
					"serial_generated_at":      lockedOrder.SerialGeneratedAt,
				}).Error; err != nil {
					return err
				}
				serialHookSerials = append(serialHookSerials, createdSerials...)
			}
		}

		order = lockedOrder
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrShippingFormNotFound) {
			return nil, nil, false, errors.New("Form not found or expired")
		}
		return nil, nil, false, err
	}
	if serialTaskQueued && s.serialTaskService != nil {
		s.serialTaskService.NotifyOrderQueued(order.ID)
	}
	if len(serialHookSerials) > 0 {
		s.serialService.emitSerialCreateAfterHook(serialHookSerials, "order_service", order.UserID, order.ID)
	}
	EmitOrderStatusChangedAfterHookAsync(s.pluginManager, nil, order, statusHookBefore, order.Status, map[string]interface{}{
		"source":         "shipping_form_submit",
		"form_resubmit":  isResubmit,
		"is_new_user":    isNewUser,
		"trigger_action": "shipping_form.submit",
	})

	// 发送邮件通知
	if s.emailService != nil {
		// 首次提交时发送OrderCreate邮件，重填时不发送
		if !isResubmit {
			go s.emailService.SendOrderCreatedEmail(order)
		}
	}

	return order, user, isNewUser, nil
}

func (s *OrderService) GetShippingFormOrder(formToken string, actorUserID *uint) (*models.Order, error) {
	order, err := s.OrderRepo.FindByFormToken(formToken)
	if err != nil {
		return nil, err
	}
	if actorUserID != nil && order.UserID != nil && *order.UserID != *actorUserID {
		return nil, ErrShippingFormAccessDenied
	}
	return order, nil
}

// GetOrderByNo 根据Order号getOrder
func (s *OrderService) GetOrderByNo(orderNo string) (*models.Order, error) {
	return s.OrderRepo.FindByOrderNo(orderNo)
}

// GetOrderByID 根据IDgetOrder
func (s *OrderService) GetOrderByID(id uint) (*models.Order, error) {
	return s.OrderRepo.FindByID(id)
}

// ListOrders getOrder List
func (s *OrderService) ListOrders(page, limit int, status, search, country, productSearch string, promoCodeID *uint, promoCode string, userID *uint) ([]models.Order, int64, error) {
	return s.OrderRepo.List(page, limit, status, search, country, productSearch, promoCodeID, promoCode, userID)
}

// GetOrderCountries get所有有Order的国家列表
func (s *OrderService) GetOrderCountries() ([]string, error) {
	return s.OrderRepo.GetOrderCountries()
}

// ListUserOrders getUserOrder List
func (s *OrderService) ListUserOrders(userID uint, page, limit int, status string) ([]models.Order, int64, error) {
	return s.OrderRepo.FindByUserID(userID, page, limit, status)
}

// AssignTracking 分配物流单号
func (s *OrderService) AssignTracking(orderID uint, trackingNo string) error {
	order, err := s.OrderRepo.FindByID(orderID)
	if err != nil {
		return normalizeOrderLookupError(err)
	}
	beforeStatus := order.Status

	// 只有待发货状态的订单可以分配物流单号
	if order.Status != models.OrderStatusPending {
		return newOrderAssignTrackingStatusInvalidError(order.Status)
	}

	// 发货时将预留Inventory转为已售Inventory
	for i := range order.Items {
		item := &order.Items[i]

		// 从Inventory绑定映射中getInventoryID
		if inventoryID, exists := order.InventoryBindings[i]; exists && inventoryID > 0 {
			// 扣减Inventory：从预留转为已售
			if err := s.inventoryRepo.Deduct(inventoryID, item.Quantity, order.OrderNo); err != nil {
				// 扣减Failed但不阻止发货，记录Error日志
				fmt.Printf("Warning: Order %s Failed to deduct inventory: %v\n", order.OrderNo, err)
			}
		}
	}

	// 混合订单发货时，同时发货剩余的虚拟商品库存（非自动发货的部分）
	if s.virtualProductSvc != nil {
		hasPending, _ := s.virtualProductSvc.HasPendingVirtualStock(order.OrderNo)
		if hasPending {
			if err := s.virtualProductSvc.DeliverStock(order.ID, order.OrderNo, nil); err != nil {
				fmt.Printf("Warning: Order %s failed to deliver remaining virtual stock: %v\n", order.OrderNo, err)
			}
		}
	}

	order.TrackingNo = trackingNo
	order.Status = models.OrderStatusShipped
	now := models.NowFunc()
	order.ShippedAt = &now

	if err := s.OrderRepo.Update(order); err != nil {
		return err
	}
	EmitOrderStatusChangedAfterHookAsync(s.pluginManager, nil, order, beforeStatus, order.Status, map[string]interface{}{
		"source":         "assign_tracking",
		"trigger_action": "order.assign_tracking",
		"tracking_no":    trackingNo,
	})

	// 发送发货邮件通知
	if s.emailService != nil {
		go s.emailService.SendOrderShippedEmail(order)
	}

	return nil
}

// DeliverVirtualStock 手动发货虚拟商品库存（用于 auto_delivery=false 的商品）
// markOnlyShipped=true 时仅标记脚本虚拟库存为已发货，不执行脚本。
func (s *OrderService) DeliverVirtualStock(orderID uint, deliveredBy uint, markOnlyShipped bool) error {
	order, err := s.OrderRepo.FindByID(orderID)
	if err != nil {
		return normalizeOrderLookupError(err)
	}
	beforeStatus := order.Status

	// 只有待发货和已发货状态的订单可以手动发货虚拟商品
	if order.Status != models.OrderStatusPending && order.Status != models.OrderStatusShipped {
		return newOrderDeliverVirtualStatusInvalidError(order.Status)
	}

	if s.virtualProductSvc == nil {
		return newOrderVirtualServiceUnavailableError()
	}

	// 检查是否有待发货的虚拟库存
	hasPending, err := s.virtualProductSvc.HasPendingVirtualStock(order.OrderNo)
	if err != nil {
		return fmt.Errorf("Failed to check pending stock: %v", err)
	}
	if !hasPending {
		return newOrderNoPendingVirtualStockError()
	}

	if markOnlyShipped {
		// 仅标记脚本虚拟库存为已发货（跳过脚本执行）
		if err := s.virtualProductSvc.MarkScriptStockDeliveredWithoutExecution(order.ID, order.OrderNo, &deliveredBy); err != nil {
			return fmt.Errorf("Failed to mark script virtual stock as shipped: %v", err)
		}
	} else {
		// 发货所有剩余的预留虚拟库存
		if err := s.virtualProductSvc.DeliverStock(order.ID, order.OrderNo, &deliveredBy); err != nil {
			return fmt.Errorf("Failed to deliver virtual stock: %v", err)
		}
	}

	// 判断订单是否为纯虚拟商品订单
	isVirtualOnly := true
	for _, item := range order.Items {
		if item.ProductType != models.ProductTypeVirtual {
			isVirtualOnly = false
			break
		}
	}

	// 纯虚拟订单且当前为待发货状态：自动转为已发货
	if isVirtualOnly && order.Status == models.OrderStatusPending {
		order.Status = models.OrderStatusShipped
		now := models.NowFunc()
		order.ShippedAt = &now

		if err := s.OrderRepo.Update(order); err != nil {
			return err
		}
		EmitOrderStatusChangedAfterHookAsync(s.pluginManager, nil, order, beforeStatus, order.Status, map[string]interface{}{
			"source":                "deliver_virtual_stock",
			"trigger_action":        "order.deliver_virtual",
			"delivered_by":          deliveredBy,
			"mark_only_shipped":     markOnlyShipped,
			"virtual_delivery_auto": true,
		})

		// 发送发货邮件通知
		if s.emailService != nil {
			go s.emailService.SendOrderShippedEmail(order)
		}
	}

	return nil
}

// CompleteOrder 完成Order
func (s *OrderService) CompleteOrder(orderID uint, completedBy uint, feedback, adminRemark string) error {
	order, err := s.OrderRepo.FindByID(orderID)
	if err != nil {
		return normalizeOrderLookupError(err)
	}
	beforeStatus := order.Status

	if order.Status != models.OrderStatusShipped {
		return newOrderCompleteStatusInvalidError(order.Status)
	}

	order.Status = models.OrderStatusCompleted
	now := models.NowFunc()
	order.CompletedAt = &now
	order.CompletedBy = &completedBy
	if feedback != "" {
		order.UserFeedback = feedback
	}
	if adminRemark != "" {
		if order.AdminRemark != "" {
			order.AdminRemark += "\n"
		}
		order.AdminRemark += "[Complete] " + adminRemark
	}

	// 扣减优惠码（从预留转为已使用）
	if order.PromoCodeID != nil && s.promoCodeRepo != nil {
		if err := s.promoCodeRepo.Deduct(*order.PromoCodeID, order.OrderNo); err != nil {
			fmt.Printf("Warning: Order %s Failed to deduct promo code: %v\n", order.OrderNo, err)
		}
	}

	if err := s.OrderRepo.Update(order); err != nil {
		return err
	}
	EmitOrderStatusChangedAfterHookAsync(s.pluginManager, nil, order, beforeStatus, order.Status, map[string]interface{}{
		"source":         "complete_order",
		"trigger_action": "order.complete",
		"completed_by":   completedBy,
	})

	// 发送完成邮件通知
	if s.emailService != nil {
		go s.emailService.SendOrderCompletedEmail(order)
	}

	return nil
}

// RequestResubmit 要求重填Info
func (s *OrderService) RequestResubmit(orderID uint, reason string) (string, error) {
	order, err := s.OrderRepo.FindByID(orderID)
	if err != nil {
		return "", err
	}
	beforeStatus := order.Status

	// generate新的表单Token
	formToken := uuid.New().String()
	formExpiresAt := models.NowFunc().Add(time.Duration(s.cfg.Form.ExpireHours) * time.Hour)

	order.Status = models.OrderStatusNeedResubmit
	order.FormToken = &formToken
	order.FormExpiresAt = &formExpiresAt
	order.FormSubmittedAt = nil // 清空提交时间，允许重新提交
	if reason != "" {
		if order.AdminRemark != "" {
			order.AdminRemark += "\n"
		}
		order.AdminRemark += "[Resubmit] " + reason
	}

	if err := s.OrderRepo.Update(order); err != nil {
		return "", err
	}
	EmitOrderStatusChangedAfterHookAsync(s.pluginManager, nil, order, beforeStatus, order.Status, map[string]interface{}{
		"source":         "request_resubmit",
		"trigger_action": "order.request_resubmit",
		"reason":         reason,
	})

	// 发送重填通知邮件
	if s.emailService != nil {
		formURL := s.cfg.App.URL + "/form/shipping?token=" + formToken
		go s.emailService.SendOrderResubmitEmail(order, formURL)
	}

	return formToken, nil
}

// DeleteOrder DeleteOrder（软Delete）
func (s *OrderService) DeleteOrder(orderID uint) error {
	order, err := s.OrderRepo.FindByID(orderID)
	if err != nil {
		return normalizeOrderLookupError(err)
	}

	// 只有待付款、草稿、已取消和已退款的Order可以Delete
	if order.Status != models.OrderStatusPendingPayment && order.Status != models.OrderStatusDraft && order.Status != models.OrderStatusCancelled && order.Status != models.OrderStatusRefunded {
		return newOrderDeleteStatusInvalidError(order.Status)
	}

	// 删除待付款订单时释放预留库存
	if order.Status == models.OrderStatusPendingPayment {
		orderIDRef := order.ID
		// 释放物理商品库存
		for i := range order.Items {
			item := &order.Items[i]
			if inventoryID, exists := order.InventoryBindings[i]; exists && inventoryID > 0 {
				if err := s.releaseReservedInventoryWithHook(&orderIDRef, order.UserID, order.OrderNo, inventoryID, item.Quantity, "delete_order"); err != nil {
					fmt.Printf("Warning: Order %s Failed to release reserved inventory: %v\n", order.OrderNo, err)
				}
			}
		}
		// 释放虚拟商品库存
		if s.virtualProductSvc != nil {
			if err := s.virtualProductSvc.ReleaseStock(order.OrderNo); err != nil {
				fmt.Printf("Warning: Order %s Failed to release virtual product stock: %v\n", order.OrderNo, err)
			}
		}
		// 释放优惠码
		if order.PromoCodeID != nil && s.promoCodeRepo != nil {
			if err := s.promoCodeRepo.ReleaseReserve(*order.PromoCodeID, order.OrderNo); err != nil {
				fmt.Printf("Warning: Order %s Failed to release promo code: %v\n", order.OrderNo, err)
			}
		}
	}

	// Delete serial numbers associated with this order before deleting the order
	if s.serialService != nil {
		if err := s.serialService.DeleteSerialsByOrderID(orderID); err != nil {
			// Log error but don't block deletion process
			fmt.Printf("Warning: Order %s failed to delete serial numbers: %v\n", order.OrderNo, err)
		}
	}
	if s.serialTaskService != nil {
		if err := s.serialTaskService.DeleteOrderTask(orderID); err != nil {
			fmt.Printf("Warning: Order %s failed to delete serial generation task: %v\n", order.OrderNo, err)
		}
	}
	if err := s.OrderRepo.Delete(orderID); err != nil {
		return err
	}
	syncUserPurchaseStatsTransitionBestEffort(s.OrderRepo, order.UserID, nil, order.Status, "", order.Items, "delete_order")
	s.syncUserConsumptionStatusTransitionBestEffort(order.UserID, order.Status, "", order.TotalAmount, "delete_order")
	return nil
}

// UpdateOrder UpdateOrderInfo
func (s *OrderService) UpdateOrder(order *models.Order) error {
	return s.OrderRepo.Update(order)
}

// CancelOrder 取消Order
func (s *OrderService) CancelOrder(orderID uint, reason string) error {
	order, err := s.OrderRepo.FindByID(orderID)
	if err != nil {
		return normalizeOrderLookupError(err)
	}
	previousStatus := order.Status

	// 只有仍处于可取消流程中的订单允许取消
	if order.Status != models.OrderStatusPendingPayment &&
		order.Status != models.OrderStatusDraft &&
		order.Status != models.OrderStatusPending &&
		order.Status != models.OrderStatusNeedResubmit {
		return newOrderCancelStatusInvalidError(order.Status)
	}

	// 取消Order时释放预留Inventory
	// 待付款、草稿状态和待发货状态的Order有预留Inventoryneed释放
	if order.Status == models.OrderStatusPendingPayment || order.Status == models.OrderStatusDraft || order.Status == models.OrderStatusPending || order.Status == models.OrderStatusNeedResubmit {
		orderIDRef := order.ID
		// 释放物理商品库存
		for i := range order.Items {
			item := &order.Items[i]

			// 从Inventory绑定映射中getInventoryID
			if inventoryID, exists := order.InventoryBindings[i]; exists && inventoryID > 0 {
				// 释放预留Inventory
				if err := s.releaseReservedInventoryWithHook(&orderIDRef, order.UserID, order.OrderNo, inventoryID, item.Quantity, "cancel_order"); err != nil {
					// 释放Failed但不阻止取消流程，记录Error日志
					fmt.Printf("Warning: Order %s Failed to release reserved inventory: %v\n", order.OrderNo, err)
				}
			}
		}

		// 释放虚拟商品库存
		if s.virtualProductSvc != nil {
			if err := s.virtualProductSvc.ReleaseStock(order.OrderNo); err != nil {
				fmt.Printf("Warning: Order %s Failed to release virtual product stock: %v\n", order.OrderNo, err)
			}
		}

		// 释放优惠码
		if order.PromoCodeID != nil && s.promoCodeRepo != nil {
			if err := s.promoCodeRepo.ReleaseReserve(*order.PromoCodeID, order.OrderNo); err != nil {
				fmt.Printf("Warning: Order %s Failed to release promo code: %v\n", order.OrderNo, err)
			}
		}
	}

	// Delete serial numbers associated with this order
	if s.serialService != nil {
		if err := s.serialService.DeleteSerialsByOrderID(orderID); err != nil {
			// Log error but don't block cancellation process
			fmt.Printf("Warning: Order %s failed to delete serial numbers: %v\n", order.OrderNo, err)
		}
	}

	order.Status = models.OrderStatusCancelled
	if reason != "" {
		if order.AdminRemark != "" {
			order.AdminRemark += "\n"
		}
		order.AdminRemark += "[Cancel] " + reason
	}

	if err := s.OrderRepo.Update(order); err != nil {
		return err
	}
	if s.serialTaskService != nil {
		if err := s.serialTaskService.CancelOrder(orderID); err != nil {
			fmt.Printf("Warning: Order %s failed to cancel serial generation task: %v\n", order.OrderNo, err)
		}
	}
	syncUserPurchaseStatsTransitionBestEffort(s.OrderRepo, order.UserID, order.UserID, previousStatus, order.Status, order.Items, "cancel_order")
	s.syncUserConsumptionStatusTransitionBestEffort(order.UserID, previousStatus, order.Status, order.TotalAmount, "cancel_order")
	EmitOrderStatusChangedAfterHookAsync(s.pluginManager, nil, order, previousStatus, order.Status, map[string]interface{}{
		"source":         "cancel_order",
		"trigger_action": "order.cancel",
		"reason":         reason,
	})

	// 发送Order取消邮件
	if s.emailService != nil {
		go s.emailService.SendOrderCancelledEmail(order)
	}

	return nil
}

// ReleaseOrderReserves 释放订单预留的库存和优惠码（用于退款/取消等场景）
func (s *OrderService) ReleaseOrderReserves(order *models.Order) {
	orderIDRef := order.ID
	// 释放物理商品库存
	for i := range order.Items {
		item := &order.Items[i]
		if inventoryID, exists := order.InventoryBindings[i]; exists && inventoryID > 0 {
			if err := s.releaseReservedInventoryWithHook(&orderIDRef, order.UserID, order.OrderNo, inventoryID, item.Quantity, "release_order_reserves"); err != nil {
				fmt.Printf("Warning: Order %s failed to release reserved inventory: %v\n", order.OrderNo, err)
			}
		}
	}

	// 释放虚拟商品库存
	if s.virtualProductSvc != nil {
		if err := s.virtualProductSvc.ReleaseStock(order.OrderNo); err != nil {
			fmt.Printf("Warning: Order %s failed to release virtual product stock: %v\n", order.OrderNo, err)
		}
	}

	// 释放优惠码
	if order.PromoCodeID != nil && s.promoCodeRepo != nil {
		if err := s.promoCodeRepo.ReleaseReserve(*order.PromoCodeID, order.OrderNo); err != nil {
			fmt.Printf("Warning: Order %s failed to release promo code: %v\n", order.OrderNo, err)
		}
	}
}

// MarkAsPaid 标记订单为已付款
func (s *OrderService) MarkAsPaid(orderID uint) error {
	return s.MarkAsPaidWithOptions(orderID, MarkAsPaidOptions{})
}

func (s *OrderService) MarkAsPaidWithOptions(orderID uint, options MarkAsPaidOptions) error {
	var (
		order          *models.Order
		finalizeResult *paidOrderFinalizeResult
	)

	err := s.OrderRepo.WithTransaction(func(tx *gorm.DB) error {
		txOrderRepo := repository.NewOrderRepository(tx)
		lockedOrder, err := txOrderRepo.FindByIDForUpdate(tx, orderID)
		if err != nil {
			return err
		}
		order = lockedOrder
		finalizeResult, err = finalizePendingPaymentOrderTx(tx, lockedOrder, s.virtualProductSvc, paidOrderFinalizeOptions{
			AdminRemark:             options.AdminRemark,
			SkipAutoDelivery:        options.SkipAutoDelivery,
			StrictAutoDeliveryCheck: true,
		})
		return err
	})
	if err != nil {
		return normalizeOrderLookupError(err)
	}
	if finalizeResult == nil || !finalizeResult.Updated {
		return newOrderMarkPaidStatusInvalidError(order.Status)
	}

	if finalizeResult.VirtualDeliveryErr != nil {
		if finalizeResult.IsVirtualOnly {
			fmt.Printf("Warning: Failed to auto deliver virtual order %s: %v\n", order.OrderNo, finalizeResult.VirtualDeliveryErr)
		} else {
			fmt.Printf("Warning: Failed to deliver virtual products for mixed order %s: %v\n", order.OrderNo, finalizeResult.VirtualDeliveryErr)
		}
	}

	s.syncUserConsumptionStatusTransitionBestEffort(
		order.UserID,
		models.OrderStatusPendingPayment,
		finalizeResult.FinalStatus,
		order.TotalAmount,
		"mark_as_paid",
	)
	EmitOrderStatusChangedAfterHookAsync(s.pluginManager, nil, order, models.OrderStatusPendingPayment, finalizeResult.FinalStatus, map[string]interface{}{
		"source":             "mark_as_paid",
		"trigger_action":     "order.mark_paid",
		"skip_auto_delivery": options.SkipAutoDelivery,
	})

	// 发送付款成功邮件
	if s.emailService != nil {
		go s.emailService.SendOrderPaidEmail(order, finalizeResult.IsVirtualOnly)
	}

	return nil
}

// MaskOrderIfNeeded 如果need，打码Order敏感Info
func (s *OrderService) MaskOrderIfNeeded(order *models.Order, hasPrivacyPermission bool) {
	if order.PrivacyProtected && !hasPrivacyPermission {
		order.MaskSensitiveInfo()
	}
}

// GetOrRefreshFormToken - Get or refresh form token
// 如果Tokendoes not exist或已过期，则generate新的Token
func (s *OrderService) GetOrRefreshFormToken(order *models.Order) (string, *time.Time, error) {
	now := models.NowFunc()

	// 检查是否needgenerate新Token
	needNewToken := false

	// 情况1：没有Token
	if order.FormToken == nil || *order.FormToken == "" {
		needNewToken = true
	}

	// 情况2：Token已过期
	if !needNewToken && order.FormExpiresAt != nil && now.After(*order.FormExpiresAt) {
		needNewToken = true
	}

	// 情况3：过期时间为空（数据异常）
	if !needNewToken && order.FormToken != nil && *order.FormToken != "" && order.FormExpiresAt == nil {
		needNewToken = true
	}

	// 情况4：即将过期（剩余时间少于1小时），提前刷新
	if !needNewToken && order.FormExpiresAt != nil {
		timeLeft := order.FormExpiresAt.Sub(now)
		if timeLeft < time.Hour {
			needNewToken = true
		}
	}

	// 如果need新Token，则generate并Update
	if needNewToken {
		// generate新的 UUID Token
		newToken := uuid.New().String()
		newExpiresAt := now.Add(time.Duration(s.cfg.Form.ExpireHours) * time.Hour)

		// UpdateOrder
		order.FormToken = &newToken
		order.FormExpiresAt = &newExpiresAt

		// 持久化到数据库
		if err := s.OrderRepo.Update(order); err != nil {
			return "", nil, errors.New("Failed to update form token")
		}

		return newToken, &newExpiresAt, nil
	}

	// 返回现有的有效Token
	return *order.FormToken, order.FormExpiresAt, nil
}

// IsOrderSharedToSupport 检查订单是否被分享到客服工单
func (s *OrderService) IsOrderSharedToSupport(orderID uint) (bool, error) {
	return s.OrderRepo.IsOrderSharedToSupport(orderID)
}

// GetSharedOrderIDs 获取指定订单ID列表中被分享到客服的订单ID集合
func (s *OrderService) GetSharedOrderIDs(orderIDs []uint) (map[uint]bool, error) {
	return s.OrderRepo.GetSharedOrderIDs(orderIDs)
}
