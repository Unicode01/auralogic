package service

import (
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"auralogic/internal/models"
	"auralogic/internal/pkg/dbutil"
	"auralogic/internal/repository"
	"gorm.io/gorm"
)

type SerialService struct {
	serialRepo    *repository.SerialRepository
	productRepo   *repository.ProductRepository
	orderRepo     *repository.OrderRepository
	pluginManager *PluginManagerService
}

type orderSerialDemand struct {
	product  *models.Product
	quantity int
}

var serialProductLocks sync.Map
var serialRandomMu sync.Mutex
var serialRandomSource = rand.New(rand.NewSource(time.Now().UnixNano()))

func NewSerialService(
	serialRepo *repository.SerialRepository,
	productRepo *repository.ProductRepository,
	orderRepo *repository.OrderRepository,
) *SerialService {
	return &SerialService{
		serialRepo:  serialRepo,
		productRepo: productRepo,
		orderRepo:   orderRepo,
	}
}

func (s *SerialService) SetPluginManager(pluginManager *PluginManagerService) {
	s.pluginManager = pluginManager
}

func lockSerialProduct(productID uint) func() {
	lock, _ := serialProductLocks.LoadOrStore(productID, &sync.Mutex{})
	mutex := lock.(*sync.Mutex)
	mutex.Lock()
	return mutex.Unlock
}

func buildSerialHookPayload(serial *models.ProductSerial) map[string]interface{} {
	if serial == nil {
		return map[string]interface{}{}
	}

	payload := map[string]interface{}{
		"serial_id":             serial.ID,
		"serial_number":         serial.SerialNumber,
		"product_id":            serial.ProductID,
		"order_id":              serial.OrderID,
		"product_code":          serial.ProductCode,
		"sequence_number":       serial.SequenceNumber,
		"anti_counterfeit_code": serial.AntiCounterfeitCode,
		"view_count":            serial.ViewCount,
		"first_viewed_at":       serial.FirstViewedAt,
		"last_viewed_at":        serial.LastViewedAt,
		"created_at":            serial.CreatedAt,
		"updated_at":            serial.UpdatedAt,
	}
	if serial.Product != nil {
		payload["product_name"] = serial.Product.Name
		payload["product_sku"] = serial.Product.SKU
	}
	if serial.Order != nil {
		payload["order_no"] = serial.Order.OrderNo
		payload["order_status"] = serial.Order.Status
		payload["user_id"] = serial.Order.UserID
	}
	return payload
}

func buildSerialHookExecutionContext(execCtx *ExecutionContext, source string, serialNumber string) *ExecutionContext {
	cloned := cloneServiceHookExecutionContext(execCtx)
	if cloned == nil {
		cloned = &ExecutionContext{}
	}
	if cloned.Metadata == nil {
		cloned.Metadata = map[string]string{}
	}
	normalizedSource := strings.TrimSpace(source)
	if normalizedSource == "" {
		normalizedSource = "internal"
	}
	cloned.Metadata["hook_source"] = normalizedSource
	if trimmedSerial := strings.TrimSpace(serialNumber); trimmedSerial != "" {
		cloned.Metadata["serial_number"] = trimmedSerial
	}
	return cloned
}

func (s *SerialService) executeSerialVerifyBeforeHook(serialNumber string, execCtx *ExecutionContext, source string) (string, error) {
	normalizedSerialNumber := strings.ToUpper(strings.TrimSpace(serialNumber))
	if s.pluginManager == nil {
		return normalizedSerialNumber, nil
	}

	hookExecCtx := buildSerialHookExecutionContext(execCtx, source, normalizedSerialNumber)
	payload := map[string]interface{}{
		"serial_number": normalizedSerialNumber,
		"source":        strings.TrimSpace(source),
	}
	hookResult, hookErr := s.pluginManager.ExecuteHook(HookExecutionRequest{
		Hook:    "serial.verify.before",
		Payload: payload,
	}, hookExecCtx)
	if hookErr != nil {
		log.Printf("serial.verify.before hook execution failed: source=%s serial=%s err=%v", strings.TrimSpace(source), normalizedSerialNumber, hookErr)
		return normalizedSerialNumber, nil
	}
	if hookResult == nil {
		return normalizedSerialNumber, nil
	}
	if hookResult.Blocked {
		reason := strings.TrimSpace(hookResult.BlockReason)
		if reason == "" {
			reason = "Serial verification rejected by plugin"
		}
		return "", newHookBlockedError(reason)
	}
	if hookResult.Payload == nil {
		return normalizedSerialNumber, nil
	}
	if value, exists := hookResult.Payload["serial_number"]; exists {
		updated, convErr := serviceHookValueToString(value)
		if convErr != nil {
			log.Printf("serial.verify.before serial_number patch ignored: source=%s serial=%s err=%v", strings.TrimSpace(source), normalizedSerialNumber, convErr)
		} else {
			normalizedSerialNumber = strings.ToUpper(strings.TrimSpace(updated))
		}
	}
	return normalizedSerialNumber, nil
}

func (s *SerialService) emitSerialVerifyAfterHook(requestedSerialNumber string, source string, serial *models.ProductSerial, verifyErr error, execCtx *ExecutionContext) {
	if s.pluginManager == nil {
		return
	}

	normalizedRequested := strings.ToUpper(strings.TrimSpace(requestedSerialNumber))
	payload := map[string]interface{}{
		"requested_serial_number": normalizedRequested,
		"serial_number":           normalizedRequested,
		"source":                  strings.TrimSpace(source),
		"success":                 verifyErr == nil && serial != nil,
	}
	if serial != nil {
		for key, value := range buildSerialHookPayload(serial) {
			payload[key] = value
		}
	}
	if verifyErr != nil {
		payload["error"] = verifyErr.Error()
	}

	go func(hookExecCtx *ExecutionContext, hookPayload map[string]interface{}, serialNumber string) {
		_, hookErr := s.pluginManager.ExecuteHook(HookExecutionRequest{
			Hook:    "serial.verify.after",
			Payload: hookPayload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("serial.verify.after hook execution failed: source=%s serial=%s err=%v", strings.TrimSpace(source), serialNumber, hookErr)
		}
	}(buildSerialHookExecutionContext(execCtx, source, normalizedRequested), payload, normalizedRequested)
}

func (s *SerialService) emitSerialCreateAfterHook(serials []models.ProductSerial, source string, orderUserID *uint, orderID uint) {
	if s.pluginManager == nil || len(serials) == 0 {
		return
	}

	orderIDCopy := orderID
	baseExecCtx := buildServiceHookExecutionContext(orderUserID, &orderIDCopy, map[string]string{
		"hook_source": strings.TrimSpace(source),
		"order_id":    strconv.FormatUint(uint64(orderID), 10),
	})
	for i := range serials {
		serial := serials[i]
		payload := buildSerialHookPayload(&serial)
		payload["source"] = strings.TrimSpace(source)
		payload["serial_count"] = len(serials)
		go func(hookExecCtx *ExecutionContext, hookPayload map[string]interface{}, serialID uint) {
			_, hookErr := s.pluginManager.ExecuteHook(HookExecutionRequest{
				Hook:    "serial.create.after",
				Payload: hookPayload,
			}, hookExecCtx)
			if hookErr != nil {
				log.Printf("serial.create.after hook execution failed: serial=%d err=%v", serialID, hookErr)
			}
		}(cloneServiceHookExecutionContext(baseExecCtx), payload, serial.ID)
	}
}

// GenerateAntiCounterfeitCode 生成4位防伪码 (0-9 A-Z)
func (s *SerialService) GenerateAntiCounterfeitCode() string {
	const charset = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

	code := make([]byte, 4)
	serialRandomMu.Lock()
	defer serialRandomMu.Unlock()
	for i := range code {
		code[i] = charset[serialRandomSource.Intn(len(charset))]
	}
	return string(code)
}

// FormatSequenceNumber 格式化序号 (001, 002, ..., 1000, ...)
func (s *SerialService) FormatSequenceNumber(seq int) string {
	if seq < 1000 {
		return fmt.Sprintf("%03d", seq)
	} else if seq < 10000 {
		return fmt.Sprintf("%04d", seq)
	} else if seq < 100000 {
		return fmt.Sprintf("%05d", seq)
	}
	return fmt.Sprintf("%d", seq)
}

// GenerateSerialNumber 生成完整序列号: 产品码+序号+防伪码
func (s *SerialService) GenerateSerialNumber(productCode string, sequenceNumber int, antiCounterfeitCode string) string {
	seqStr := s.FormatSequenceNumber(sequenceNumber)
	return fmt.Sprintf("%s%s%s", productCode, seqStr, antiCounterfeitCode)
}

func (s *SerialService) loadProductsForItemsTx(tx *gorm.DB, items []models.OrderItem) (map[string]*models.Product, error) {
	if tx == nil {
		return nil, fmt.Errorf("transaction is required")
	}
	return repository.NewProductRepository(tx).FindBySKUs(collectOrderItemSKUs(items))
}

func buildOrderSerialDemands(items []models.OrderItem, productBySKU map[string]*models.Product) []orderSerialDemand {
	demands := make([]orderSerialDemand, 0, len(items))
	demandIndexByProductID := make(map[uint]int, len(items))

	for _, item := range items {
		product := productBySKU[item.SKU]
		if product == nil || product.ProductCode == "" || product.ProductType != models.ProductTypePhysical || item.Quantity <= 0 {
			continue
		}
		if index, exists := demandIndexByProductID[product.ID]; exists {
			demands[index].quantity += item.Quantity
			continue
		}
		demandIndexByProductID[product.ID] = len(demands)
		demands = append(demands, orderSerialDemand{
			product:  product,
			quantity: item.Quantity,
		})
	}

	return demands
}

func (s *SerialService) createMissingOrderSerialsTx(tx *gorm.DB, order *models.Order) ([]models.ProductSerial, bool, error) {
	if tx == nil {
		return nil, false, fmt.Errorf("transaction is required")
	}
	if order == nil {
		return nil, false, fmt.Errorf("order is required")
	}

	productBySKU, err := s.loadProductsForItemsTx(tx, order.Items)
	if err != nil {
		return nil, false, err
	}
	demands := buildOrderSerialDemands(order.Items, productBySKU)
	if len(demands) == 0 {
		return []models.ProductSerial{}, false, nil
	}

	existingCounts, err := repository.NewSerialRepository(tx).CountByOrderIDGroupedByProduct(order.ID)
	if err != nil {
		return nil, true, fmt.Errorf("count existing order serials: %w", err)
	}

	created := make([]models.ProductSerial, 0)
	for _, demand := range demands {
		if demand.product == nil || demand.quantity <= 0 {
			continue
		}
		existingQty := existingCounts[demand.product.ID]
		if existingQty >= demand.quantity {
			continue
		}

		missingQty := demand.quantity - existingQty
		serials, createErr := s.createSerialForOrderTxLocked(tx, order, demand.product, missingQty)
		if createErr != nil {
			return nil, true, createErr
		}
		created = append(created, serials...)
		existingCounts[demand.product.ID] = demand.quantity
	}

	return created, true, nil
}

func (s *SerialService) allocateSerialSequenceRangeTx(tx *gorm.DB, product *models.Product, quantity int) (int, error) {
	if tx == nil {
		return 0, fmt.Errorf("transaction is required")
	}
	if product == nil {
		return 0, fmt.Errorf("product is required")
	}
	if quantity <= 0 {
		return 0, nil
	}

	if err := dbutil.LockForUpdate(tx, &models.Product{}, "id = ?", product.ID); err != nil {
		return 0, fmt.Errorf("failed to lock product for serial allocation: %w", err)
	}

	currentSeq := product.LastSerialSequence
	if currentSeq <= 0 {
		if err := tx.Model(&models.ProductSerial{}).
			Where("product_id = ?", product.ID).
			Select("COALESCE(MAX(sequence_number), 0)").
			Scan(&currentSeq).Error; err != nil {
			return 0, fmt.Errorf("failed to get next sequence number: %w", err)
		}
	}

	nextSeq := currentSeq + 1
	lastAllocatedSeq := currentSeq + quantity
	if err := tx.Model(&models.Product{}).
		Where("id = ?", product.ID).
		UpdateColumn("last_serial_sequence", lastAllocatedSeq).Error; err != nil {
		return 0, fmt.Errorf("failed to update product serial sequence: %w", err)
	}

	product.LastSerialSequence = lastAllocatedSeq
	return nextSeq, nil
}

func (s *SerialService) createSerialForOrderTxLocked(tx *gorm.DB, order *models.Order, product *models.Product, quantity int) ([]models.ProductSerial, error) {
	if tx == nil {
		return nil, fmt.Errorf("transaction is required")
	}
	if order == nil {
		return nil, fmt.Errorf("order is required")
	}
	if product == nil {
		return nil, fmt.Errorf("product is required")
	}
	if quantity <= 0 {
		return []models.ProductSerial{}, nil
	}
	if product.ProductCode == "" {
		return nil, fmt.Errorf("product code not set for product %d", product.ID)
	}

	nextSeq, err := s.allocateSerialSequenceRangeTx(tx, product, quantity)
	if err != nil {
		return nil, err
	}

	serials := make([]models.ProductSerial, 0, quantity)
	for i := 0; i < quantity; i++ {
		antiCounterfeitCode := s.GenerateAntiCounterfeitCode()
		serialNumber := s.GenerateSerialNumber(product.ProductCode, nextSeq, antiCounterfeitCode)
		serials = append(serials, models.ProductSerial{
			SerialNumber:        serialNumber,
			ProductID:           product.ID,
			OrderID:             order.ID,
			ProductCode:         product.ProductCode,
			SequenceNumber:      nextSeq,
			AntiCounterfeitCode: antiCounterfeitCode,
			ViewCount:           0,
		})
		nextSeq++
	}

	if err := repository.NewSerialRepository(tx).BatchCreate(serials); err != nil {
		return nil, fmt.Errorf("failed to create serials: %w", err)
	}

	return serials, nil
}

func (s *SerialService) createSerialForOrderTx(tx *gorm.DB, order *models.Order, product *models.Product, quantity int) ([]models.ProductSerial, error) {
	if product == nil {
		return nil, fmt.Errorf("product is required")
	}

	unlock := lockSerialProduct(product.ID)
	defer unlock()

	return s.createSerialForOrderTxLocked(tx, order, product, quantity)
}

// CreateSerialForOrder 为订单商品创建序列号
func (s *SerialService) CreateSerialForOrder(orderID uint, productID uint, quantity int) ([]models.ProductSerial, error) {
	unlock := lockSerialProduct(productID)
	defer unlock()

	var (
		order   *models.Order
		serials []models.ProductSerial
	)

	if err := s.orderRepo.WithTransaction(func(tx *gorm.DB) error {
		var currentProduct models.Product
		if err := tx.Select("id", "product_code", "last_serial_sequence").First(&currentProduct, productID).Error; err != nil {
			return fmt.Errorf("product not found: %w", err)
		}

		var currentOrder models.Order
		if err := tx.Select("id", "user_id").First(&currentOrder, orderID).Error; err != nil {
			return fmt.Errorf("order not found: %w", err)
		}

		order = &currentOrder
		var err error
		serials, err = s.createSerialForOrderTxLocked(tx, &currentOrder, &currentProduct, quantity)
		return err
	}); err != nil {
		return nil, err
	}

	s.emitSerialCreateAfterHook(serials, "order_service", order.UserID, orderID)

	return serials, nil
}

// VerifySerial 验证序列号并增加查看次数
func (s *SerialService) VerifySerial(serialNumber string) (*models.ProductSerial, error) {
	return s.VerifySerialWithContext(serialNumber, nil, "internal")
}

func (s *SerialService) VerifySerialWithContext(serialNumber string, execCtx *ExecutionContext, source string) (*models.ProductSerial, error) {
	requestedSerialNumber := strings.ToUpper(strings.TrimSpace(serialNumber))
	finalSerialNumber, hookErr := s.executeSerialVerifyBeforeHook(requestedSerialNumber, execCtx, source)
	if hookErr != nil {
		s.emitSerialVerifyAfterHook(requestedSerialNumber, source, nil, hookErr, execCtx)
		return nil, hookErr
	}
	if finalSerialNumber == "" {
		verifyErr := fmt.Errorf("serial number not found")
		s.emitSerialVerifyAfterHook(requestedSerialNumber, source, nil, verifyErr, execCtx)
		return nil, verifyErr
	}

	// 查找序列号
	serial, err := s.serialRepo.FindBySerialNumber(finalSerialNumber)
	if err != nil {
		verifyErr := fmt.Errorf("serial number not found")
		s.emitSerialVerifyAfterHook(finalSerialNumber, source, nil, verifyErr, execCtx)
		return nil, verifyErr
	}

	// 增加查看次数
	if err := s.serialRepo.IncrementViewCount(serial.SerialNumber); err != nil {
		// 记录错误但不影响返回结果
	}

	// 重新加载以获取最新的查看次数
	serial, _ = s.serialRepo.FindBySerialNumber(serial.SerialNumber)
	s.emitSerialVerifyAfterHook(finalSerialNumber, source, serial, nil, execCtx)

	return serial, nil
}

// GetSerialsByOrderID 获取订单的所有序列号
func (s *SerialService) GetSerialsByOrderID(orderID uint) ([]models.ProductSerial, error) {
	return s.serialRepo.FindByOrderID(orderID)
}

// GetSerialsByProductID 获取商品的所有序列号
func (s *SerialService) GetSerialsByProductID(productID uint) ([]models.ProductSerial, error) {
	return s.serialRepo.FindByProductID(productID)
}

// ListSerials 分页查询序列号
func (s *SerialService) ListSerials(page, limit int, filters map[string]interface{}) ([]models.ProductSerial, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	return s.serialRepo.List(page, limit, filters)
}

// GetStatistics 获取统计信息
func (s *SerialService) GetStatistics() (map[string]interface{}, error) {
	return s.serialRepo.GetStatistics()
}

func (s *SerialService) GetSerialByID(id uint) (*models.ProductSerial, error) {
	return s.serialRepo.FindByID(id)
}

func (s *SerialService) GetSerialsByIDs(ids []uint) ([]models.ProductSerial, error) {
	return s.serialRepo.FindByIDs(ids)
}

// DeleteSerial Delete a single serial number by ID
func (s *SerialService) DeleteSerial(id uint) error {
	return s.serialRepo.Delete(id)
}

// DeleteSerialsByOrderID Delete all serial numbers for an order
func (s *SerialService) DeleteSerialsByOrderID(orderID uint) error {
	return s.serialRepo.DeleteByOrderID(orderID)
}

// BatchDeleteSerials Delete multiple serial numbers by IDs
func (s *SerialService) BatchDeleteSerials(ids []uint) error {
	return s.serialRepo.BatchDelete(ids)
}
