package repository

import (
	"time"

	"auralogic/internal/models"
	"gorm.io/gorm"
)

type SerialRepository struct {
	db *gorm.DB
}

func NewSerialRepository(db *gorm.DB) *SerialRepository {
	return &SerialRepository{db: db}
}

// Create 创建序列号
func (r *SerialRepository) Create(serial *models.ProductSerial) error {
	return r.db.Create(serial).Error
}

// FindBySerialNumber 根据完整序列号查找
func (r *SerialRepository) FindBySerialNumber(serialNumber string) (*models.ProductSerial, error) {
	var serial models.ProductSerial
	err := r.db.Preload("Product").Preload("Order").
		Where("serial_number = ?", serialNumber).
		First(&serial).Error
	if err != nil {
		return nil, err
	}
	return &serial, nil
}

// FindByOrderID 根据订单ID查找所有序列号
func (r *SerialRepository) FindByOrderID(orderID uint) ([]models.ProductSerial, error) {
	var serials []models.ProductSerial
	err := r.db.Preload("Product").
		Where("order_id = ?", orderID).
		Find(&serials).Error
	return serials, err
}

// FindByProductID 根据商品ID查找所有序列号
func (r *SerialRepository) FindByProductID(productID uint) ([]models.ProductSerial, error) {
	var serials []models.ProductSerial
	err := r.db.Preload("Order").
		Where("product_id = ?", productID).
		Order("sequence_number DESC").
		Find(&serials).Error
	return serials, err
}

// GetNextSequenceNumber 获取商品的下一个序号
func (r *SerialRepository) GetNextSequenceNumber(productID uint) (int, error) {
	var maxSeq int
	err := r.db.Model(&models.ProductSerial{}).
		Where("product_id = ?", productID).
		Select("COALESCE(MAX(sequence_number), 0)").
		Scan(&maxSeq).Error
	if err != nil {
		return 0, err
	}
	return maxSeq + 1, nil
}

// IncrementViewCount 增加查看次数
func (r *SerialRepository) IncrementViewCount(serialNumber string) error {
	now := time.Now()
	return r.db.Model(&models.ProductSerial{}).
		Where("serial_number = ?", serialNumber).
		Updates(map[string]interface{}{
			"view_count":      gorm.Expr("view_count + 1"),
			"last_viewed_at":  now,
			"first_viewed_at": gorm.Expr("COALESCE(first_viewed_at, ?)", now),
		}).Error
}

// List 分页查询序列号列表
func (r *SerialRepository) List(page, limit int, filters map[string]interface{}) ([]models.ProductSerial, int64, error) {
	var serials []models.ProductSerial
	var total int64

	query := r.db.Model(&models.ProductSerial{}).Preload("Product").Preload("Order")

	// 应用筛选条件
	if productID, ok := filters["product_id"]; ok {
		query = query.Where("product_id = ?", productID)
	}
	if orderID, ok := filters["order_id"]; ok {
		query = query.Where("order_id = ?", orderID)
	}
	if productCode, ok := filters["product_code"]; ok {
		query = query.Where("product_code LIKE ?", "%"+productCode.(string)+"%")
	}
	if serialNumber, ok := filters["serial_number"]; ok {
		query = query.Where("serial_number LIKE ?", "%"+serialNumber.(string)+"%")
	}

	// 计算总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * limit
	err := query.Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&serials).Error

	return serials, total, err
}

// BatchCreate 批量创建序列号
func (r *SerialRepository) BatchCreate(serials []models.ProductSerial) error {
	return r.db.Create(&serials).Error
}

// GetStatistics 获取统计信息
func (r *SerialRepository) GetStatistics() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 总序列号数
	var totalCount int64
	if err := r.db.Model(&models.ProductSerial{}).Count(&totalCount).Error; err != nil {
		return nil, err
	}
	stats["total_count"] = totalCount

	// 已查看的序列号数
	var viewedCount int64
	if err := r.db.Model(&models.ProductSerial{}).Where("view_count > 0").Count(&viewedCount).Error; err != nil {
		return nil, err
	}
	stats["viewed_count"] = viewedCount

	// 总查看次数
	var totalViews int64
	if err := r.db.Model(&models.ProductSerial{}).Select("COALESCE(SUM(view_count), 0)").Scan(&totalViews).Error; err != nil {
		return nil, err
	}
	stats["total_views"] = totalViews

	return stats, nil
}

// Delete Delete a serial number by ID
func (r *SerialRepository) Delete(id uint) error {
	return r.db.Delete(&models.ProductSerial{}, id).Error
}

// DeleteByOrderID Delete all serial numbers for an order
func (r *SerialRepository) DeleteByOrderID(orderID uint) error {
	return r.db.Where("order_id = ?", orderID).Delete(&models.ProductSerial{}).Error
}

// BatchDelete Delete multiple serial numbers by IDs
func (r *SerialRepository) BatchDelete(ids []uint) error {
	return r.db.Delete(&models.ProductSerial{}, ids).Error
}
