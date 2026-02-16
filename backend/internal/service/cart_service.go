package service

import (
	"errors"
	"fmt"

	"auralogic/internal/models"
	"auralogic/internal/repository"
	"gorm.io/gorm"
)

type CartService struct {
	cartRepo                *repository.CartRepository
	productRepo             *repository.ProductRepository
	bindingService          *BindingService
	virtualInventoryService *VirtualInventoryService
}

func NewCartService(cartRepo *repository.CartRepository, productRepo *repository.ProductRepository, bindingService *BindingService, virtualInventoryService *VirtualInventoryService) *CartService {
	return &CartService{
		cartRepo:                cartRepo,
		productRepo:             productRepo,
		bindingService:          bindingService,
		virtualInventoryService: virtualInventoryService,
	}
}

// AddToCartRequest 添加到购物车请求
type AddToCartRequest struct {
	ProductID  uint              `json:"product_id" binding:"required"`
	Quantity   int               `json:"quantity" binding:"required,min=1"`
	Attributes map[string]string `json:"attributes"`
}

// GetCart 获取用户购物车
func (s *CartService) GetCart(userID uint) ([]models.CartItemWithStock, error) {
	items, err := s.cartRepo.GetUserCart(userID)
	if err != nil {
		return nil, err
	}

	// 为每个商品添加库存信息
	result := make([]models.CartItemWithStock, 0, len(items))
	for _, item := range items {
		itemWithStock := models.CartItemWithStock{
			CartItem:    item,
			IsAvailable: true,
		}

		// 检查商品是否还存在且上架
		if item.Product == nil || item.Product.Status != models.ProductStatusActive {
			itemWithStock.IsAvailable = false
			itemWithStock.AvailableStock = 0
		} else {
			// 获取可用库存
			stock, err := s.getAvailableStock(item.ProductID, item.Attributes)
			if err != nil {
				itemWithStock.AvailableStock = 0
				itemWithStock.IsAvailable = false
			} else {
				itemWithStock.AvailableStock = stock
				itemWithStock.IsAvailable = stock >= item.Quantity
			}
		}

		result = append(result, itemWithStock)
	}

	return result, nil
}

// getAvailableStock 获取商品可用库存
func (s *CartService) getAvailableStock(productID uint, attributes models.JSONMap) (int, error) {
	product, err := s.productRepo.FindByID(productID)
	if err != nil {
		return 0, err
	}

	// 转换属性类型
	attrs := make(map[string]string)
	for k, v := range attributes {
		attrs[k] = v
	}

	if product.ProductType == models.ProductTypeVirtual {
		// 虚拟商品：从虚拟库存服务获取
		var stock int64
		if len(attrs) > 0 {
			stock, err = s.virtualInventoryService.GetAvailableCountForProductByAttributes(productID, attrs)
		} else {
			stock, err = s.virtualInventoryService.GetAvailableCountForProduct(productID)
		}
		if err != nil {
			return 0, err
		}
		return int(stock), nil
	}

	// 实体商品：使用BindingService获取库存
	return s.bindingService.GetAvailableStockByAttributes(productID, attrs)
}

// AddToCart 添加商品到购物车
func (s *CartService) AddToCart(userID uint, req AddToCartRequest) (*models.CartItem, error) {
	// 验证商品是否存在且上架
	product, err := s.productRepo.FindByID(req.ProductID)
	if err != nil {
		return nil, errors.New("商品不存在")
	}

	if product.Status != models.ProductStatusActive {
		return nil, errors.New("商品已下架")
	}

	// 验证属性是否有效（对于需要选择属性的商品）
	if len(product.Attributes) > 0 {
		for _, attr := range product.Attributes {
			// 只检查用户选择模式的属性
			if attr.Mode == models.AttributeModeUserSelect {
				val, exists := req.Attributes[attr.Name]
				if !exists || val == "" {
					return nil, fmt.Errorf("请选择%s", attr.Name)
				}
				// 验证属性值是否有效
				valid := false
				for _, v := range attr.Values {
					if v == val {
						valid = true
						break
					}
				}
				if !valid {
					return nil, fmt.Errorf("无效的%s选项", attr.Name)
				}
			}
		}
	}

	// 转换属性为 JSONMap
	attributes := make(models.JSONMap)
	for k, v := range req.Attributes {
		attributes[k] = v
	}

	// 检查购物车中是否已有相同商品和属性的项
	existingItem, err := s.cartRepo.FindCartItem(userID, req.ProductID, attributes)
	if err == nil && existingItem != nil {
		// 已存在，增加数量
		newQuantity := existingItem.Quantity + req.Quantity

		// 检查库存
		stock, err := s.getAvailableStock(req.ProductID, attributes)
		if err != nil {
			return nil, errors.New("获取库存失败")
		}
		if newQuantity > stock {
			return nil, fmt.Errorf("库存不足，当前可用库存: %d", stock)
		}

		// 检查购买限制
		if product.MaxPurchaseLimit > 0 && newQuantity > product.MaxPurchaseLimit {
			return nil, fmt.Errorf("超过购买限制，最多可购买 %d 件", product.MaxPurchaseLimit)
		}

		existingItem.Quantity = newQuantity
		if err := s.cartRepo.UpdateCartItem(existingItem); err != nil {
			return nil, err
		}
		return existingItem, nil
	}

	// 不存在，创建新项

	// 检查库存
	stock, err := s.getAvailableStock(req.ProductID, attributes)
	if err != nil {
		return nil, errors.New("获取库存失败")
	}
	if req.Quantity > stock {
		return nil, fmt.Errorf("库存不足，当前可用库存: %d", stock)
	}

	// 检查购买限制
	if product.MaxPurchaseLimit > 0 && req.Quantity > product.MaxPurchaseLimit {
		return nil, fmt.Errorf("超过购买限制，最多可购买 %d 件", product.MaxPurchaseLimit)
	}

	// 获取商品主图
	imageURL := ""
	if len(product.Images) > 0 {
		for _, img := range product.Images {
			if img.IsPrimary {
				imageURL = img.URL
				break
			}
		}
		if imageURL == "" {
			imageURL = product.Images[0].URL
		}
	}

	newItem := &models.CartItem{
		UserID:      userID,
		ProductID:   req.ProductID,
		SKU:         product.SKU,
		Name:        product.Name,
		Price:       product.Price,
		ImageURL:    imageURL,
		ProductType: product.ProductType,
		Quantity:    req.Quantity,
		Attributes:  attributes,
	}

	if err := s.cartRepo.CreateCartItem(newItem); err != nil {
		return nil, err
	}

	return newItem, nil
}

// UpdateQuantity 更新购物车项数量
func (s *CartService) UpdateQuantity(userID, itemID uint, quantity int) (*models.CartItem, error) {
	if quantity < 1 {
		return nil, errors.New("数量必须大于0")
	}

	item, err := s.cartRepo.GetCartItem(itemID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("购物车项不存在")
		}
		return nil, err
	}

	// 验证所有权
	if item.UserID != userID {
		return nil, errors.New("无权操作此购物车项")
	}

	// 检查库存
	stock, err := s.getAvailableStock(item.ProductID, item.Attributes)
	if err != nil {
		return nil, errors.New("获取库存失败")
	}
	if quantity > stock {
		return nil, fmt.Errorf("库存不足，当前可用库存: %d", stock)
	}

	// 检查购买限制
	product, _ := s.productRepo.FindByID(item.ProductID)
	if product != nil && product.MaxPurchaseLimit > 0 && quantity > product.MaxPurchaseLimit {
		return nil, fmt.Errorf("超过购买限制，最多可购买 %d 件", product.MaxPurchaseLimit)
	}

	item.Quantity = quantity
	if err := s.cartRepo.UpdateCartItem(item); err != nil {
		return nil, err
	}

	return item, nil
}

// RemoveFromCart 从购物车移除商品
func (s *CartService) RemoveFromCart(userID, itemID uint) error {
	item, err := s.cartRepo.GetCartItem(itemID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil // 已经不存在，视为成功
		}
		return err
	}

	// 验证所有权
	if item.UserID != userID {
		return errors.New("无权操作此购物车项")
	}

	return s.cartRepo.DeleteCartItem(itemID)
}

// ClearCart 清空购物车
func (s *CartService) ClearCart(userID uint) error {
	return s.cartRepo.ClearUserCart(userID)
}

// GetCartCount 获取购物车商品总件数
func (s *CartService) GetCartCount(userID uint) (int64, error) {
	return s.cartRepo.GetCartTotalQuantity(userID)
}

// GetCartItemCount 获取购物车商品种类数
func (s *CartService) GetCartItemCount(userID uint) (int64, error) {
	return s.cartRepo.GetCartItemCount(userID)
}
