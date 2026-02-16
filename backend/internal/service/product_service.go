package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"auralogic/internal/models"
	"auralogic/internal/repository"
)

type ProductService struct {
	productRepo   *repository.ProductRepository
	inventoryRepo *repository.InventoryRepository
	uploadDir     string
	baseURL       string
}

var (
	// Public, user-facing errors (safe to show to clients).
	ErrSKUAlreadyExists = errors.New("SKU already exists")
)

func NewProductService(productRepo *repository.ProductRepository, inventoryRepo *repository.InventoryRepository) *ProductService {
	return &ProductService{
		productRepo:   productRepo,
		inventoryRepo: inventoryRepo,
		uploadDir:     "uploads", // 默认上传目录
		baseURL:       "",        // 从配置中get
	}
}

// SetUploadConfig 设置上传配置
func (s *ProductService) SetUploadConfig(uploadDir, baseURL string) {
	s.uploadDir = uploadDir
	s.baseURL = baseURL
}

// CreateProduct CreateProduct
func (s *ProductService) CreateProduct(product *models.Product) error {
	// 验证SKU唯一性
	existing, _ := s.productRepo.FindBySKU(product.SKU)
	if existing != nil && existing.ID != 0 {
		return ErrSKUAlreadyExists
	}

	// Validate required fields
	if product.Name == "" {
		return errors.New("Product name cannot be empty")
	}
	if product.SKU == "" {
		return errors.New("Product SKU cannot be empty")
	}

	// Price validation
	if product.Price <= 0 {
		return errors.New("Product price must be greater than 0")
	}

	// 设置默认状态
	if product.Status == "" {
		product.Status = models.ProductStatusDraft
	}

	// 设置默认商品类型
	if product.ProductType == "" {
		product.ProductType = models.ProductTypePhysical
	}

	// 设置默认Inventory模式
	if product.InventoryMode == "" {
		product.InventoryMode = string(models.InventoryModeFixed)
	}

	// CreateProduct
	if err := s.productRepo.Create(product); err != nil {
		// Concurrent create (or DB constraint) might still hit unique constraints.
		if isUniqueConstraintError(err) {
			msg := strings.ToLower(err.Error())
			if strings.Contains(msg, "sku") || strings.Contains(msg, "products.sku") {
				return ErrSKUAlreadyExists
			}
			return ErrSKUAlreadyExists
		}
		return err
	}
	return nil
}

// UpdateProduct UpdateProduct
func (s *ProductService) UpdateProduct(id uint, updates *models.Product) error {
	product, err := s.productRepo.FindByID(id)
	if err != nil {
		return errors.New("Product not found")
	}

	// 如果UpdateSKU，检查唯一性
	if updates.SKU != "" && updates.SKU != product.SKU {
		existing, _ := s.productRepo.FindBySKU(updates.SKU)
		if existing != nil && existing.ID != 0 {
			return ErrSKUAlreadyExists
		}
		product.SKU = updates.SKU
	}

	// Update字段
	if updates.Name != "" {
		product.Name = updates.Name
	}
	// ProductCode、Description、ShortDescription、Category、Remark 允许Update为空字符串
	product.ProductCode = updates.ProductCode
	product.Description = updates.Description
	product.ShortDescription = updates.ShortDescription
	product.Category = updates.Category
	product.Remark = updates.Remark

	// 更新商品类型（允许在 physical 和 virtual 之间切换）
	if updates.ProductType != "" {
		product.ProductType = updates.ProductType
	}

	if updates.Tags != nil {
		product.Tags = updates.Tags
	}
	product.Price = updates.Price
	product.OriginalPrice = updates.OriginalPrice
	product.Stock = updates.Stock
	product.MaxPurchaseLimit = updates.MaxPurchaseLimit

	if updates.Images != nil {
		product.Images = updates.Images
	}
	if updates.Attributes != nil {
		product.Attributes = updates.Attributes
	}
	if updates.Status != "" {
		product.Status = updates.Status
	}
	product.SortOrder = updates.SortOrder
	product.IsFeatured = updates.IsFeatured
	product.IsRecommended = updates.IsRecommended
	product.AutoDelivery = updates.AutoDelivery

	if err := s.productRepo.Update(product); err != nil {
		if isUniqueConstraintError(err) {
			msg := strings.ToLower(err.Error())
			if strings.Contains(msg, "sku") || strings.Contains(msg, "products.sku") {
				return ErrSKUAlreadyExists
			}
			return ErrSKUAlreadyExists
		}
		return err
	}
	return nil
}

// DeleteProduct DeleteProduct
func (s *ProductService) DeleteProduct(id uint) error {
	product, err := s.productRepo.FindByID(id)
	if err != nil {
		return errors.New("Product not found")
	}

	// Delete product associated image files
	if err := s.deleteProductImages(product); err != nil {
		// Log error but do not block product deletion
		fmt.Printf("Warning: Failed to delete product images: %v\n", err)
	}

	return s.productRepo.Delete(product.ID)
}

// deleteProductImages DeleteProduct的所有图片文件
func (s *ProductService) deleteProductImages(product *models.Product) error {
	if product == nil || len(product.Images) == 0 {
		return nil
	}

	var errors []string
	for _, image := range product.Images {
		if err := s.deleteImageFile(image.URL); err != nil {
			errors = append(errors, err.Error())
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("Some images failed to delete: %s", strings.Join(errors, "; "))
	}

	return nil
}

// deleteImageFile Delete单个图片文件
func (s *ProductService) deleteImageFile(imageURL string) error {
	// 从URL中提取文件路径
	// 例如：http://localhost:8080/uploads/products/2026/01/07/uuid.jpg
	// 提取：products/2026/01/07/uuid.jpg
	prefix := s.baseURL + "/uploads/"
	if !strings.HasPrefix(imageURL, prefix) {
		// 不是本地上传的图片（可能是外部URL），跳过
		return nil
	}

	relativePath := strings.TrimPrefix(imageURL, prefix)
	cleanRel := filepath.Clean(relativePath)
	baseDir, err := filepath.Abs(s.uploadDir)
	if err != nil {
		return fmt.Errorf("failed to resolve upload directory: %w", err)
	}
	targetPath, err := filepath.Abs(filepath.Join(baseDir, cleanRel))
	if err != nil {
		return fmt.Errorf("failed to resolve image path: %w", err)
	}
	if targetPath != baseDir && !strings.HasPrefix(targetPath, baseDir+string(os.PathSeparator)) {
		return fmt.Errorf("invalid image path")
	}
	filePath := targetPath

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// 文件does not exist，跳过
		return nil
	}

	// Delete文件
	return os.Remove(filePath)
}

// GetProductByID 根据IDgetProduct详情
func (s *ProductService) GetProductByID(id uint, incrementView bool) (*models.Product, error) {
	product, err := s.productRepo.FindByID(id)
	if err != nil {
		return nil, errors.New("Product not found")
	}

	// 增加浏览次数
	if incrementView {
		_ = s.productRepo.IncrementViewCount(id)
		product.ViewCount++
	}

	return product, nil
}

// GetProductBySKU 根据SKUgetProduct详情
func (s *ProductService) GetProductBySKU(sku string) (*models.Product, error) {
	return s.productRepo.FindBySKU(sku)
}

// ListProducts getProduct列表
func (s *ProductService) ListProducts(page, limit int, status, category, search string, isFeatured *bool, isRecommended *bool, isActive bool) ([]models.Product, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	products, total, err := s.productRepo.List(page, limit, status, category, search, isFeatured, isRecommended, isActive)
	if err != nil {
		return nil, 0, err
	}

	return products, total, nil
}

// UpdateProductStatus UpdateProduct状态
func (s *ProductService) UpdateProductStatus(id uint, status models.ProductStatus) error {
	product, err := s.productRepo.FindByID(id)
	if err != nil {
		return errors.New("Product not found")
	}

	product.Status = status
	return s.productRepo.Update(product)
}

// UpdateStock Update inventory
func (s *ProductService) UpdateStock(id uint, stock int) error {
	if stock < 0 {
		return errors.New("Stock cannot be negative")
	}

	product, err := s.productRepo.FindByID(id)
	if err != nil {
		return errors.New("Product not found")
	}

	// Auto set to out of stock if inventory becomes 0
	if stock == 0 && product.Status == models.ProductStatusActive {
		product.Status = models.ProductStatusOutOfStock
	} else if stock > 0 && product.Status == models.ProductStatusOutOfStock {
		product.Status = models.ProductStatusActive
	}

	product.Stock = stock
	return s.productRepo.Update(product)
}

// DecrementStock Decrease inventory (used for orders)
func (s *ProductService) DecrementStock(id uint, quantity int) error {
	if quantity <= 0 {
		return errors.New("Quantity must be greater than 0")
	}

	product, err := s.productRepo.FindByID(id)
	if err != nil {
		return errors.New("Product not found")
	}

	if product.Stock < quantity {
		return fmt.Errorf("Inventoryinsufficient, currentInventory：%d", product.Stock)
	}

	return s.productRepo.DecrementStock(id, quantity)
}

// GetCategories get所有分类
func (s *ProductService) GetCategories() ([]string, error) {
	return s.productRepo.GetCategories()
}

// ToggleFeatured 切换精选状态
func (s *ProductService) ToggleFeatured(id uint) error {
	product, err := s.productRepo.FindByID(id)
	if err != nil {
		return errors.New("Product not found")
	}

	product.IsFeatured = !product.IsFeatured
	return s.productRepo.Update(product)
}
