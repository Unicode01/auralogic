package repository

import (
	"auralogic/internal/models"
	"gorm.io/gorm"
)

type CartRepository struct {
	db *gorm.DB
}

func NewCartRepository(db *gorm.DB) *CartRepository {
	return &CartRepository{db: db}
}

// GetUserCart 获取用户购物车所有商品
func (r *CartRepository) GetUserCart(userID uint) ([]models.CartItem, error) {
	var items []models.CartItem
	err := r.db.Where("user_id = ?", userID).
		Preload("Product").
		Order("created_at DESC").
		Find(&items).Error
	return items, err
}

// GetCartItem 根据ID获取购物车项
func (r *CartRepository) GetCartItem(id uint) (*models.CartItem, error) {
	var item models.CartItem
	err := r.db.Preload("Product").First(&item, id).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// FindCartItem 根据用户ID、商品ID和属性查找购物车项
func (r *CartRepository) FindCartItem(userID, productID uint, attributes models.JSONMap) (*models.CartItem, error) {
	var item models.CartItem

	hash := models.GenerateAttributesHash(map[string]string(attributes))

	query := r.db.Where("user_id = ? AND product_id = ?", userID, productID)

	if hash != "" {
		query = query.Where("attributes_hash = ?", hash)
	} else {
		query = query.Where("attributes_hash = '' OR attributes_hash IS NULL")
	}

	err := query.First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// CreateCartItem 创建购物车项
func (r *CartRepository) CreateCartItem(item *models.CartItem) error {
	return r.db.Create(item).Error
}

// UpdateCartItem 更新购物车项
func (r *CartRepository) UpdateCartItem(item *models.CartItem) error {
	return r.db.Save(item).Error
}

// UpdateQuantity 更新购物车项数量
func (r *CartRepository) UpdateQuantity(id uint, quantity int) error {
	return r.db.Model(&models.CartItem{}).Where("id = ?", id).Update("quantity", quantity).Error
}

// DeleteCartItem 删除购物车项
func (r *CartRepository) DeleteCartItem(id uint) error {
	return r.db.Delete(&models.CartItem{}, id).Error
}

// DeleteUserCartItem 删除用户的购物车项（验证所有权）
func (r *CartRepository) DeleteUserCartItem(userID, itemID uint) error {
	return r.db.Where("id = ? AND user_id = ?", itemID, userID).Delete(&models.CartItem{}).Error
}

// ClearUserCart 清空用户购物车
func (r *CartRepository) ClearUserCart(userID uint) error {
	return r.db.Where("user_id = ?", userID).Delete(&models.CartItem{}).Error
}

// GetCartItemCount 获取用户购物车商品数量
func (r *CartRepository) GetCartItemCount(userID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.CartItem{}).Where("user_id = ?", userID).Count(&count).Error
	return count, err
}

// GetCartTotalQuantity 获取用户购物车商品总件数
func (r *CartRepository) GetCartTotalQuantity(userID uint) (int64, error) {
	var total int64
	err := r.db.Model(&models.CartItem{}).
		Where("user_id = ?", userID).
		Select("COALESCE(SUM(quantity), 0)").
		Scan(&total).Error
	return total, err
}

// DeleteCartItemsByProductID 根据商品ID删除购物车项（商品下架时使用）
func (r *CartRepository) DeleteCartItemsByProductID(productID uint) error {
	return r.db.Where("product_id = ?", productID).Delete(&models.CartItem{}).Error
}
