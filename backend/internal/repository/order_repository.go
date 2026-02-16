package repository

import (
	"auralogic/internal/models"
	"gorm.io/gorm"
)

type OrderRepository struct {
	db *gorm.DB
}

func NewOrderRepository(db *gorm.DB) *OrderRepository {
	return &OrderRepository{db: db}
}

// Create 创建订单
func (r *OrderRepository) Create(order *models.Order) error {
	return r.db.Create(order).Error
}

// FindByID 根据ID查找订单
func (r *OrderRepository) FindByID(id uint) (*models.Order, error) {
	var order models.Order
	err := r.db.Preload("User").First(&order, id).Error
	return &order, err
}

// FindByOrderNo 根据订单号查找订单
func (r *OrderRepository) FindByOrderNo(orderNo string) (*models.Order, error) {
	var order models.Order
	err := r.db.Preload("User").Where("order_no = ?", orderNo).First(&order).Error
	return &order, err
}

// FindByFormToken 根据表单Token查找订单
func (r *OrderRepository) FindByFormToken(token string) (*models.Order, error) {
	var order models.Order
	err := r.db.Where("form_token = ?", token).First(&order).Error
	return &order, err
}

// FindByExternalUserID 根据外部UserID查找订单列表
func (r *OrderRepository) FindByExternalUserID(externalUserID string, page, limit int) ([]models.Order, int64, error) {
	var orders []models.Order
	var total int64

	query := r.db.Model(&models.Order{}).Where("external_user_id = ?", externalUserID)

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页Query
	offset := (page - 1) * limit
	err := query.Offset(offset).Limit(limit).Order("created_at DESC").Find(&orders).Error

	return orders, total, err
}

// FindByUserID 根据UserID查找订单列表
func (r *OrderRepository) FindByUserID(userID uint, page, limit int, status string) ([]models.Order, int64, error) {
	var orders []models.Order
	var total int64

	query := r.db.Model(&models.Order{}).Where("user_id = ?", userID)

	if status != "" {
		query = query.Where("status = ?", status)
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页Query
	offset := (page - 1) * limit
	err := query.Offset(offset).Limit(limit).Order("created_at DESC").Find(&orders).Error

	return orders, total, err
}

// List 获取订单列表
func (r *OrderRepository) List(page, limit int, status, search, country, productSearch string, promoCodeID *uint, promoCode string, userID *uint) ([]models.Order, int64, error) {
	var orders []models.Order
	var total int64

	query := r.db.Model(&models.Order{}).Preload("User")

	if status != "" {
		query = query.Where("status = ?", status)
	}

	if search != "" {
		query = query.Where("order_no LIKE ? OR receiver_name LIKE ? OR receiver_email LIKE ?",
			"%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	// 按商品SKU/名称筛选（搜索订单项的JSON字段）
	// 使用数据库无关的 LIKE Query（兼容 SQLite、PostgreSQL、MySQL）
	if productSearch != "" {
		// 直接在 JSON 字符串中搜索（兼容所有数据库）
		// SQLite/MySQL/PostgreSQL 都支持这种方式
		searchPattern := "%" + productSearch + "%"
		query = query.Where("items LIKE ?", searchPattern)
	}

	// 按国家筛选
	if country != "" {
		query = query.Where("receiver_country = ?", country)
	}

	// 按UserID筛选
	if userID != nil && *userID > 0 {
		query = query.Where("user_id = ?", *userID)
	}

	if promoCodeID != nil && *promoCodeID > 0 && promoCode != "" {
		query = query.Where("(promo_code_id = ? OR promo_code_str = ?)", *promoCodeID, promoCode)
	} else if promoCodeID != nil && *promoCodeID > 0 {
		query = query.Where("promo_code_id = ?", *promoCodeID)
	} else if promoCode != "" {
		query = query.Where("promo_code_str = ?", promoCode)
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页Query
	offset := (page - 1) * limit
	err := query.Offset(offset).Limit(limit).Order("created_at DESC").Find(&orders).Error

	return orders, total, err
}

// Update 更新订单
func (r *OrderRepository) Update(order *models.Order) error {
	return r.db.Save(order).Error
}

// UpdateStatus 更新订单状态
func (r *OrderRepository) UpdateStatus(orderID uint, status models.OrderStatus) error {
	return r.db.Model(&models.Order{}).Where("id = ?", orderID).Update("status", status).Error
}

// Delete 删除订单（软删除）
func (r *OrderRepository) Delete(orderID uint) error {
	return r.db.Delete(&models.Order{}, orderID).Error
}

// GetUserPurchaseQuantityBySKU 获取用户购买某个商品的总数量（不包括已取消的订单）
func (r *OrderRepository) GetUserPurchaseQuantityBySKU(userID uint, sku string) (int, error) {
	var orders []models.Order
	// QueryUser的所有非取消状态的订单
	err := r.db.Where("user_id = ? AND status != ?", userID, models.OrderStatusCancelled).
		Find(&orders).Error

	if err != nil {
		return 0, err
	}

	// 统计商品数量
	totalQuantity := 0
	for _, order := range orders {
		for _, item := range order.Items {
			if item.SKU == sku {
				totalQuantity += item.Quantity
			}
		}
	}

	return totalQuantity, nil
}

// GetOrderCountries 获取所有有订单的国家列表
func (r *OrderRepository) GetOrderCountries() ([]string, error) {
	var countries []string

	err := r.db.Model(&models.Order{}).
		Where("receiver_country != '' AND receiver_country IS NOT NULL").
		Distinct("receiver_country").
		Order("receiver_country").
		Pluck("receiver_country", &countries).Error

	if err != nil {
		return nil, err
	}

	return countries, nil
}

// IsOrderSharedToSupport 检查订单是否被分享到客服工单
func (r *OrderRepository) IsOrderSharedToSupport(orderID uint) (bool, error) {
	var count int64
	err := r.db.Model(&models.TicketOrderAccess{}).
		Where("order_id = ?", orderID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetSharedOrderIDs 获取指定订单ID列表中被分享到客服的订单ID集合
func (r *OrderRepository) GetSharedOrderIDs(orderIDs []uint) (map[uint]bool, error) {
	if len(orderIDs) == 0 {
		return make(map[uint]bool), nil
	}

	var sharedOrderIDs []uint
	err := r.db.Model(&models.TicketOrderAccess{}).
		Where("order_id IN ?", orderIDs).
		Distinct("order_id").
		Pluck("order_id", &sharedOrderIDs).Error
	if err != nil {
		return nil, err
	}

	result := make(map[uint]bool)
	for _, id := range sharedOrderIDs {
		result[id] = true
	}
	return result, nil
}
