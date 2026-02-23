package repository

import (
	"errors"
	"fmt"

	"auralogic/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type InventoryRepository struct {
	db *gorm.DB
}

func NewInventoryRepository(db *gorm.DB) *InventoryRepository {
	return &InventoryRepository{db: db}
}

// Create CreateInventory记录
func (r *InventoryRepository) Create(inventory *models.Inventory) error {
	return r.db.Create(inventory).Error
}

// Update UpdateInventory记录
func (r *InventoryRepository) Update(inventory *models.Inventory) error {
	return r.db.Save(inventory).Error
}

// FindByID 根据ID查找
func (r *InventoryRepository) FindByID(id uint) (*models.Inventory, error) {
	var inventory models.Inventory
	err := r.db.Preload("ProductBindings").First(&inventory, id).Error
	return &inventory, err
}

// FindBySKU 根据SKU查找
func (r *InventoryRepository) FindBySKU(sku string, inventory *models.Inventory) error {
	return r.db.Where("sku = ?", sku).First(inventory).Error
}

// FindByAttributes 根据属性组合查找库存（不限制商品）
func (r *InventoryRepository) FindByAttributes(attrs map[string]string) (*models.Inventory, error) {
	// 标准化属性
	normalizedAttrs := models.NormalizeAttributes(attrs)

	// 计算属性哈希
	attrsHash := models.GenerateAttributesHash(normalizedAttrs)

	var inventory models.Inventory
	err := r.db.Where("attributes_hash = ? AND deleted_at IS NULL", attrsHash).
		First(&inventory).Error

	return &inventory, err
}

// ListByProductIDs 批量获取商品的库存记录
func (r *InventoryRepository) ListByProductIDs(productIDs []uint) ([]models.Inventory, error) {
	var inventories []models.Inventory
	err := r.db.Where("product_id IN ?", productIDs).
		Where("is_active = ?", true).
		Find(&inventories).Error
	return inventories, err
}

// List 分页列表
func (r *InventoryRepository) List(page, limit int, filters map[string]interface{}) ([]models.Inventory, int64, error) {
	var inventories []models.Inventory
	var total int64

	query := r.db.Model(&models.Inventory{})

	// 应用过滤条件
	if isActive, ok := filters["is_active"].(bool); ok {
		query = query.Where("is_active = ?", isActive)
	}
	if lowStock, ok := filters["low_stock"].(bool); ok && lowStock {
		query = query.Where("(stock - sold_quantity - reserved_quantity) <= safety_stock")
	}

	// 计算总数
	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// 分页请求
	offset := (page - 1) * limit
	err = query.Preload("ProductBindings.Product").
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&inventories).Error

	return inventories, total, err
}

// Delete 删除库存记录（软删除）
func (r *InventoryRepository) Delete(id uint) error {
	return r.db.Delete(&models.Inventory{}, id).Error
}

// Reserve 预留库存（下单但未支付）
func (r *InventoryRepository) Reserve(inventoryID uint, quantity int, orderNo string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var inventory models.Inventory
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&inventory, inventoryID).Error; err != nil {
			return err
		}

		// 检查是否可以预留
		if canReserve, msg := inventory.CanPurchase(quantity); !canReserve {
			return fmt.Errorf("%s", msg)
		}

		// 更新预留数量
		beforeReserved := inventory.ReservedQuantity
		inventory.ReservedQuantity += quantity

		if err := tx.Save(&inventory).Error; err != nil {
			return err
		}

		// 记录日志
		log := &models.InventoryLog{
			InventoryID: inventoryID,
			ProductID:   0,
			Type:        models.InventoryLogTypeReserve,
			Quantity:    quantity,
			BeforeStock: beforeReserved,
			AfterStock:  inventory.ReservedQuantity,
			OrderNo:     orderNo,
			Operator:    "system",
			Reason:      "Reserve inventory for order",
		}

		return tx.Create(log).Error
	})
}

// ReleaseReserve 释放预留库存（取消订单）
func (r *InventoryRepository) ReleaseReserve(inventoryID uint, quantity int, orderNo string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var inventory models.Inventory
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&inventory, inventoryID).Error; err != nil {
			return err
		}

		beforeReserved := inventory.ReservedQuantity
		inventory.ReservedQuantity -= quantity
		if inventory.ReservedQuantity < 0 {
			inventory.ReservedQuantity = 0
		}

		if err := tx.Save(&inventory).Error; err != nil {
			return err
		}

		// 记录日志（ProductID设为0，因为库存不再直接关联商品）
		log := &models.InventoryLog{
			InventoryID: inventoryID,
			ProductID:   0, // 库存不再直接关联商品
			Type:        models.InventoryLogTypeRelease,
			Quantity:    quantity,
			BeforeStock: beforeReserved,
			AfterStock:  inventory.ReservedQuantity,
			OrderNo:     orderNo,
			Operator:    "system",
			Reason:      "Release reserved inventory on order cancellation",
		}

		return tx.Create(log).Error
	})
}

// Deduct 扣减库存（发货成功）
// 执行以下操作：
// 1. 增加已售数量（SoldQuantity）
// 2. 减少预留数量（ReservedQuantity）
// 3. 减少总库存（Stock）
// 4. 减少可购买数（AvailableQuantity）
func (r *InventoryRepository) Deduct(inventoryID uint, quantity int, orderNo string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var inventory models.Inventory
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&inventory, inventoryID).Error; err != nil {
			return err
		}

		// 记录扣减前的状态
		beforeStock := inventory.Stock

		// 验证库存充足
		if inventory.Stock < quantity {
			return fmt.Errorf("insufficient stock: available %d, required %d", inventory.Stock, quantity)
		}

		// 1. 增加已售数量
		inventory.SoldQuantity += quantity

		// 2. 释放预留（如果有）
		if inventory.ReservedQuantity >= quantity {
			inventory.ReservedQuantity -= quantity
		} else {
			inventory.ReservedQuantity = 0
		}

		// 3. 减少总Inventory
		inventory.Stock -= quantity

		// 4. 减少可购买数
		if inventory.AvailableQuantity >= quantity {
			inventory.AvailableQuantity -= quantity
		} else {
			inventory.AvailableQuantity = 0
		}

		if err := tx.Save(&inventory).Error; err != nil {
			return err
		}

		// 记录出库日志
		log := &models.InventoryLog{
			InventoryID: inventoryID,
			ProductID:   0, // 库存不再直接关联商品
			Type:        models.InventoryLogTypeOut,
			Quantity:    quantity,
			BeforeStock: beforeStock,
			AfterStock:  inventory.Stock,
			OrderNo:     orderNo,
			Operator:    "system",
			Reason:      "Deduct inventory on order shipment",
		}

		return tx.Create(log).Error
	})
}

// Adjust 调整库存（入库、盘点等）- 旧方法保留用于兼容
func (r *InventoryRepository) Adjust(inventoryID uint, newStock, newAvailable int, operator, reason string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var inventory models.Inventory
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&inventory, inventoryID).Error; err != nil {
			return err
		}

		beforeStock := inventory.Stock
		inventory.Stock = newStock
		inventory.AvailableQuantity = newAvailable

		if err := tx.Save(&inventory).Error; err != nil {
			return err
		}

		// 记录调整日志（ProductID设为0，因为库存不再直接关联商品）
		log := &models.InventoryLog{
			InventoryID: inventoryID,
			ProductID:   0, // 库存不再直接关联商品
			Type:        models.InventoryLogTypeAdjust,
			Quantity:    newStock - beforeStock,
			BeforeStock: beforeStock,
			AfterStock:  newStock,
			Operator:    operator,
			Reason:      reason,
		}

		return tx.Create(log).Error
	})
}

// AdjustByDelta 通过增量调整库存（推荐使用，避免并发问题）- 旧方法保留用于兼容
func (r *InventoryRepository) AdjustByDelta(inventoryID uint, stockDelta, availableDelta int, operator, reason string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var inventory models.Inventory
		// 锁定行以避免并发问题
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&inventory, inventoryID).Error; err != nil {
			return err
		}

		beforeStock := inventory.Stock

		// 应用增量
		newStock := inventory.Stock + stockDelta
		newAvailable := inventory.AvailableQuantity + availableDelta

		// 验证：库存不能为负
		if newStock < 0 {
			return errors.New("Adjusted inventory cannot be negative")
		}

		// 验证：可售数量不能为负
		if newAvailable < 0 {
			return errors.New("Adjusted available quantity cannot be negative")
		}

		// 验证：可售数量不能超过库存
		if newAvailable > newStock {
			return errors.New("Available quantity cannot exceed total stock")
		}

		inventory.Stock = newStock
		inventory.AvailableQuantity = newAvailable

		if err := tx.Save(&inventory).Error; err != nil {
			return err
		}

		// 记录调整日志
		log := &models.InventoryLog{
			InventoryID: inventoryID,
			ProductID:   0,
			Type:        models.InventoryLogTypeAdjust,
			Quantity:    stockDelta,
			BeforeStock: beforeStock,
			AfterStock:  newStock,
			Operator:    operator,
			Reason:      reason,
		}

		return tx.Create(log).Error
	})
}

// GetLowStockList 获取低库存列表
func (r *InventoryRepository) GetLowStockList() ([]models.Inventory, error) {
	var inventories []models.Inventory
	err := r.db.Preload("ProductBindings.Product").
		Where("is_active = ?", true).
		Where("(stock - sold_quantity - reserved_quantity) <= safety_stock").
		Order("(stock - sold_quantity - reserved_quantity) ASC").
		Find(&inventories).Error
	return inventories, err
}
