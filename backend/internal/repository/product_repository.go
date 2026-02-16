package repository

import (
	"fmt"

	"auralogic/internal/models"
	"gorm.io/gorm"
)

type ProductRepository struct {
	db *gorm.DB
}

func NewProductRepository(db *gorm.DB) *ProductRepository {
	return &ProductRepository{db: db}
}

// Create CreateProduct
func (r *ProductRepository) Create(product *models.Product) error {
	return r.db.Create(product).Error
}

// Update UpdateProduct
func (r *ProductRepository) Update(product *models.Product) error {
	return r.db.Save(product).Error
}

// FindByID 根据ID查找商品
func (r *ProductRepository) FindByID(id uint) (*models.Product, error) {
	var product models.Product
	err := r.db.Preload("InventoryBindings.Inventory").First(&product, id).Error
	return &product, err
}

// FindBySKU 根据SKU查找商品
func (r *ProductRepository) FindBySKU(sku string) (*models.Product, error) {
	var product models.Product
	err := r.db.Where("sku = ?", sku).First(&product).Error
	return &product, err
}

// Delete 删除商品（软删除）
func (r *ProductRepository) Delete(id uint) error {
	return r.db.Delete(&models.Product{}, id).Error
}

// List 获取商品列表
func (r *ProductRepository) List(page, limit int, status, category, search string, isFeatured *bool, isRecommended *bool, isActive bool) ([]models.Product, int64, error) {
	var products []models.Product
	var total int64

	query := r.db.Model(&models.Product{})

	// 筛选条件（状态、分类、搜索、是否精选、是否上架）
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if category != "" {
		query = query.Where("category = ?", category)
	}
	if search != "" {
		query = query.Where("name LIKE ? OR sku LIKE ? OR description LIKE ?",
			"%"+search+"%", "%"+search+"%", "%"+search+"%")
	}
	if isFeatured != nil {
		query = query.Where("is_featured = ?", *isFeatured)
	}
	if isRecommended != nil {
		query = query.Where("is_recommended = ?", *isRecommended)
	}
	if isActive {
		query = query.Where("status = ?", models.ProductStatusActive)
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 排序和分页（按排序号降序、创建时间降序）
	offset := (page - 1) * limit
	err := query.Order("sort_order DESC, created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&products).Error

	return products, total, err
}

// IncrementViewCount 增加商品浏览次数
func (r *ProductRepository) IncrementViewCount(id uint) error {
	return r.db.Model(&models.Product{}).
		Where("id = ?", id).
		UpdateColumn("view_count", gorm.Expr("view_count + 1")).
		Error
}

// IncrementSaleCount 增加商品销量
func (r *ProductRepository) IncrementSaleCount(id uint, quantity int) error {
	return r.db.Model(&models.Product{}).
		Where("id = ?", id).
		UpdateColumn("sale_count", gorm.Expr("sale_count + ?", quantity)).
		Error
}

// UpdateStock 更新商品库存
func (r *ProductRepository) UpdateStock(id uint, quantity int) error {
	return r.db.Model(&models.Product{}).
		Where("id = ?", id).
		Update("stock", quantity).
		Error
}

// DecrementStock 减少商品库存
func (r *ProductRepository) DecrementStock(id uint, quantity int) error {
	// 使用 Updates 而不是 UpdateColumn，这样会Update updated_at 并触发 hooks
	result := r.db.Model(&models.Product{}).
		Where("id = ? AND stock >= ?", id, quantity).
		Updates(map[string]interface{}{
			"stock": gorm.Expr("stock - ?", quantity),
		})

	if result.Error != nil {
		return result.Error
	}

	// 检查是否真的Update了记录（商品库存是否足够）
	if result.RowsAffected == 0 {
		return fmt.Errorf("Insufficient inventory or product does not exist")
	}

	return nil
}

// GetCategories 获取所有分类
func (r *ProductRepository) GetCategories() ([]string, error) {
	var categories []string
	err := r.db.Model(&models.Product{}).
		Where("category IS NOT NULL AND category != ''").
		Distinct("category").
		Pluck("category", &categories).
		Error
	return categories, err
}
