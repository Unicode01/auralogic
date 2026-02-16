package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"auralogic/internal/models"
	"auralogic/internal/pkg/logger"
	"gorm.io/gorm"
)

// PaymentMethodService 付款方式服务
type PaymentMethodService struct {
	db        *gorm.DB
	jsRuntime *JSRuntimeService
}

// NewPaymentMethodService 创建付款方式服务
func NewPaymentMethodService(db *gorm.DB) *PaymentMethodService {
	return &PaymentMethodService{
		db:        db,
		jsRuntime: NewJSRuntimeService(db),
	}
}

// InitBuiltinPaymentMethods 初始化内置付款方式
func (s *PaymentMethodService) InitBuiltinPaymentMethods() error {
	for _, pm := range models.BuiltinPaymentMethods {
		var existing models.PaymentMethod
		// 按名称查找，不限定类型（因为现在内置的也用JS脚本）
		if err := s.db.Where("name = ?", pm.Name).First(&existing).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// 创建新的付款方式
				if err := s.db.Create(&pm).Error; err != nil {
					return fmt.Errorf("failed to create builtin payment method %s: %w", pm.Name, err)
				}
				logger.LogSystemOperation(s.db, "payment_method_init", "payment_method", nil, map[string]interface{}{
					"name":   pm.Name,
					"type":   pm.Type,
					"action": "created",
				})
			} else {
				return err
			}
		} else {
			// 更新脚本（如果脚本为空则更新）
			if existing.Script == "" && pm.Script != "" {
				if err := s.db.Model(&existing).Updates(map[string]interface{}{
					"script": pm.Script,
					"type":   pm.Type,
				}).Error; err == nil {
					logger.LogSystemOperation(s.db, "payment_method_init", "payment_method", &existing.ID, map[string]interface{}{
						"name":   pm.Name,
						"action": "updated_script",
					})
				}
			}
		}
	}
	return nil
}

// List 获取所有付款方式
func (s *PaymentMethodService) List(enabledOnly bool) ([]models.PaymentMethod, error) {
	var methods []models.PaymentMethod
	query := s.db.Order("sort_order ASC, id ASC")
	if enabledOnly {
		query = query.Where("enabled = ?", true)
	}
	if err := query.Find(&methods).Error; err != nil {
		return nil, err
	}
	return methods, nil
}

// Get 获取单个付款方式
func (s *PaymentMethodService) Get(id uint) (*models.PaymentMethod, error) {
	var pm models.PaymentMethod
	if err := s.db.First(&pm, id).Error; err != nil {
		return nil, err
	}
	return &pm, nil
}

// Create 创建付款方式
func (s *PaymentMethodService) Create(pm *models.PaymentMethod) error {
	// 获取最大排序值
	var maxSort int
	s.db.Model(&models.PaymentMethod{}).Select("COALESCE(MAX(sort_order), 0)").Scan(&maxSort)
	pm.SortOrder = maxSort + 1

	return s.db.Create(pm).Error
}

// Update 更改付款方式
func (s *PaymentMethodService) Update(id uint, updates map[string]interface{}) error {
	return s.db.Model(&models.PaymentMethod{}).Where("id = ?", id).Updates(updates).Error
}

// Delete 删除付款方式
func (s *PaymentMethodService) Delete(id uint) error {
	return s.db.Delete(&models.PaymentMethod{}, id).Error
}

// ToggleEnabled 切换启用状态
func (s *PaymentMethodService) ToggleEnabled(id uint) error {
	return s.db.Exec("UPDATE payment_methods SET enabled = NOT enabled WHERE id = ?", id).Error
}

// Reorder 重新排序
func (s *PaymentMethodService) Reorder(ids []uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		for i, id := range ids {
			if err := tx.Model(&models.PaymentMethod{}).Where("id = ?", id).Update("sort_order", i+1).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// GetEnabledMethods 获取启用的付款方式
func (s *PaymentMethodService) GetEnabledMethods() ([]models.PaymentMethod, error) {
	return s.List(true)
}

// GeneratePaymentCard 生成付款卡片HTML
func (s *PaymentMethodService) GeneratePaymentCard(paymentMethodID uint, order *models.Order) (*PaymentCardResult, error) {
	pm, err := s.Get(paymentMethodID)
	if err != nil {
		return nil, err
	}
	if !pm.Enabled {
		return nil, errors.New("payment method is disabled")
	}
	return s.jsRuntime.ExecutePaymentCard(pm, order)
}

// CachePaymentCard 缓存付款卡片到订单
func (s *PaymentMethodService) CachePaymentCard(orderID uint, result *PaymentCardResult) error {
	cacheJSON, err := json.Marshal(result)
	if err != nil {
		return err
	}

	// 计算缓存过期时间
	var expiresAt *time.Time
	if result.CacheTTL > 0 {
		// 设置具体的过期时间
		expiry := time.Now().Add(time.Duration(result.CacheTTL) * time.Second)
		expiresAt = &expiry
	} else if result.CacheTTL == 0 {
		// 不缓存，设置为立即过期
		expiry := time.Now()
		expiresAt = &expiry
	}
	// result.CacheTTL == -1 或未设置时，expiresAt 为 nil，表示永久缓存

	return s.db.Model(&models.OrderPaymentMethod{}).Where("order_id = ?", orderID).
		Updates(map[string]interface{}{
			"payment_card_cache": string(cacheJSON),
			"cache_expires_at":   expiresAt,
		}).Error
}

// GetCachedPaymentCard 获取缓存的付款卡片
func (s *PaymentMethodService) GetCachedPaymentCard(orderID uint) (*PaymentCardResult, error) {
	var opm models.OrderPaymentMethod
	if err := s.db.Where("order_id = ?", orderID).First(&opm).Error; err != nil {
		return nil, err
	}
	if opm.PaymentCardCache == "" {
		return nil, nil
	}

	// 检查缓存是否过期
	if opm.CacheExpiresAt != nil && time.Now().After(*opm.CacheExpiresAt) {
		// 缓存已过期，返回 nil
		return nil, nil
	}

	var result PaymentCardResult
	if err := json.Unmarshal([]byte(opm.PaymentCardCache), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// SelectPaymentMethod 为订单选择付款方式
func (s *PaymentMethodService) SelectPaymentMethod(orderID, paymentMethodID uint) error {
	// 验证付款方式存在且启用
	pm, err := s.Get(paymentMethodID)
	if err != nil {
		return err
	}
	if !pm.Enabled {
		return errors.New("payment method is disabled")
	}

	// 验证订单状态
	var order models.Order
	if err := s.db.First(&order, orderID).Error; err != nil {
		return err
	}
	if order.Status != models.OrderStatusPendingPayment {
		return errors.New("order is not in pending payment status")
	}

	// 创建或更新订单付款方式
	opm := models.OrderPaymentMethod{
		OrderID:         orderID,
		PaymentMethodID: paymentMethodID,
	}

	err = s.db.Where("order_id = ?", orderID).
		Assign(models.OrderPaymentMethod{PaymentMethodID: paymentMethodID}).
		FirstOrCreate(&opm).Error

	if err == nil {
		logger.LogPaymentOperation(s.db, "payment_method_selected", orderID, map[string]interface{}{
			"order_no":          order.OrderNo,
			"payment_method_id": paymentMethodID,
			"payment_method":    pm.Name,
		})
	}

	return err
}

// GetOrderPaymentMethod 获取订单选择的付款方式
func (s *PaymentMethodService) GetOrderPaymentMethod(orderID uint) (*models.PaymentMethod, *models.OrderPaymentMethod, error) {
	var opm models.OrderPaymentMethod
	if err := s.db.Where("order_id = ?", orderID).First(&opm).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, nil
		}
		return nil, nil, err
	}

	pm, err := s.Get(opm.PaymentMethodID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 付款方式已被删除，清理旧的关联记录，让用户重新选择
			s.db.Where("order_id = ?", orderID).Delete(&models.OrderPaymentMethod{})
			return nil, nil, nil
		}
		return nil, nil, err
	}

	return pm, &opm, nil
}

// UpdatePaymentConfig 更新付款方式配置
func (s *PaymentMethodService) UpdatePaymentConfig(id uint, config map[string]interface{}) error {
	configJSON, err := json.Marshal(config)
	if err != nil {
		return err
	}
	return s.db.Model(&models.PaymentMethod{}).Where("id = ?", id).Update("config", string(configJSON)).Error
}

// TestScript 测试JS脚本
func (s *PaymentMethodService) TestScript(script string, config map[string]interface{}) (*PaymentCardResult, error) {
	configJSON, _ := json.Marshal(config)
	pm := &models.PaymentMethod{
		Type:   models.PaymentMethodTypeCustom,
		Script: script,
		Config: string(configJSON),
	}

	// 创建一个测试订单
	testOrder := &models.Order{
		OrderNo:     "TEST-ORDER-001",
		Status:      models.OrderStatusPendingPayment,
		TotalAmount: 99.99,
		Currency:    "CNY",
	}

	return s.jsRuntime.ExecutePaymentCard(pm, testOrder)
}
