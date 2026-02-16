package repository

import (
	"auralogic/internal/models"
	"gorm.io/gorm"
)

type BindingRepository struct {
	db *gorm.DB
}

func NewBindingRepository(db *gorm.DB) *BindingRepository {
	return &BindingRepository{db: db}
}

// Create Create绑定关系
func (r *BindingRepository) Create(binding *models.ProductInventoryBinding) error {
	return r.db.Create(binding).Error
}

// Update Update绑定关系
func (r *BindingRepository) Update(binding *models.ProductInventoryBinding) error {
	return r.db.Save(binding).Error
}

// Delete Delete绑定关系（硬删除）
func (r *BindingRepository) Delete(id uint) error {
	return r.db.Delete(&models.ProductInventoryBinding{}, id).Error
}

// DeleteByProductID 删除商品的所有绑定关系（批量删除）
func (r *BindingRepository) DeleteByProductID(productID uint) error {
	return r.db.Where("product_id = ?", productID).Delete(&models.ProductInventoryBinding{}).Error
}

// FindByID 根据ID查找
func (r *BindingRepository) FindByID(id uint) (*models.ProductInventoryBinding, error) {
	var binding models.ProductInventoryBinding
	err := r.db.Preload("Product").Preload("Inventory").First(&binding, id).Error
	return &binding, err
}

// FindByProductID 获取商品的所有库存绑定
func (r *BindingRepository) FindByProductID(productID uint) ([]models.ProductInventoryBinding, error) {
	var bindings []models.ProductInventoryBinding
	err := r.db.Where("product_id = ?", productID).
		Preload("Inventory").
		Order("created_at ASC").
		Find(&bindings).Error
	return bindings, err
}

// FindByInventoryID 获取库存的所有商品绑定
func (r *BindingRepository) FindByInventoryID(inventoryID uint) ([]models.ProductInventoryBinding, error) {
	var bindings []models.ProductInventoryBinding
	err := r.db.Where("inventory_id = ?", inventoryID).
		Preload("Product").
		Order("created_at ASC").
		Find(&bindings).Error
	return bindings, err
}

// FindRandomBindings 获取商品的所有参与随机分配的库存绑定
func (r *BindingRepository) FindRandomBindings(productID uint) ([]models.ProductInventoryBinding, error) {
	var bindings []models.ProductInventoryBinding
	err := r.db.Where("product_id = ? AND is_random = ?", productID, true).
		Preload("Inventory").
		Find(&bindings).Error
	return bindings, err
}

// FindFixedBindings 获取商品的固定（非随机）库存绑定
func (r *BindingRepository) FindFixedBindings(productID uint) ([]models.ProductInventoryBinding, error) {
	var bindings []models.ProductInventoryBinding
	err := r.db.Where("product_id = ? AND is_random = ?", productID, false).
		Preload("Inventory").
		Find(&bindings).Error
	return bindings, err
}

// FindByProductAndInventory 查找特定的库存绑定关系
func (r *BindingRepository) FindByProductAndInventory(productID, inventoryID uint) (*models.ProductInventoryBinding, error) {
	var binding models.ProductInventoryBinding
	err := r.db.Where("product_id = ? AND inventory_id = ?", productID, inventoryID).
		First(&binding).Error
	return &binding, err
}

// Exists 检查库存绑定关系是否存在（基于product_id和inventory_id）
func (r *BindingRepository) Exists(productID, inventoryID uint) (bool, error) {
	var count int64
	err := r.db.Model(&models.ProductInventoryBinding{}).
		Where("product_id = ? AND inventory_id = ?", productID, inventoryID).
		Count(&count).Error
	return count > 0, err
}

// ExistsByAttributesHash 检查该商品的该规格组合是否已绑定
func (r *BindingRepository) ExistsByAttributesHash(productID uint, attributesHash string) (bool, error) {
	var count int64
	err := r.db.Model(&models.ProductInventoryBinding{}).
		Where("product_id = ? AND attributes_hash = ?", productID, attributesHash).
		Count(&count).Error
	return count > 0, err
}

// GetBindingWithStockInfo 获取库存绑定关系及库存详情
func (r *BindingRepository) GetBindingWithStockInfo(productID uint) ([]models.BindingWithInventoryInfo, error) {
	var results []models.BindingWithInventoryInfo

	err := r.db.Table("product_inventory_bindings").
		Select(`
			product_inventory_bindings.id,
			product_inventory_bindings.product_id,
			product_inventory_bindings.inventory_id,
			product_inventory_bindings.is_random,
			product_inventory_bindings.priority,
			product_inventory_bindings.notes,
			product_inventory_bindings.created_at,
			inventories.id as "inventory.id",
			inventories.name as "inventory.name",
			inventories.sku as "inventory.sku",
			inventories.attributes as "inventory.attributes",
			inventories.stock as "inventory.stock",
			inventories.available_quantity as "inventory.available_quantity",
			inventories.sold_quantity as "inventory.sold_quantity",
			inventories.reserved_quantity as "inventory.reserved_quantity",
			inventories.is_active as "inventory.is_active",
			(inventories.stock - inventories.sold_quantity - inventories.reserved_quantity) as "inventory.remaining_stock"
		`).
		Joins("LEFT JOIN inventories ON product_inventory_bindings.inventory_id = inventories.id").
		Where("product_inventory_bindings.product_id = ?", productID).
		Scan(&results).Error

	return results, err
}
