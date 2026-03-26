package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"auralogic/internal/config"
	"auralogic/internal/models"
	"auralogic/internal/pkg/bizerr"
	"auralogic/internal/repository"
	"gorm.io/gorm"
)

type ProductService struct {
	productRepo   *repository.ProductRepository
	inventoryRepo *repository.InventoryRepository
	uploadDir     string
	baseURL       string
}

type DeleteProductOptions struct {
	DeleteImages bool
}

var (
	ErrProductNotFound = errors.New("Product not found")
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

func (s *ProductService) currentUploadRuntime() (string, string) {
	uploadDir := strings.TrimSpace(s.uploadDir)
	baseURL := strings.TrimRight(strings.TrimSpace(s.baseURL), "/")

	if runtimeCfg := config.GetConfig(); runtimeCfg != nil {
		if strings.TrimSpace(runtimeCfg.Upload.Dir) != "" {
			uploadDir = strings.TrimSpace(runtimeCfg.Upload.Dir)
		}
		baseURL = strings.TrimRight(strings.TrimSpace(runtimeCfg.App.URL), "/")
	}

	if uploadDir == "" {
		uploadDir = "uploads"
	}
	return uploadDir, baseURL
}

func extractProductImageRelativePath(imageURL string) (string, bool) {
	const marker = "/uploads/products/"
	idx := strings.Index(strings.TrimSpace(imageURL), marker)
	if idx < 0 {
		return "", false
	}

	relativePath := strings.TrimPrefix(imageURL[idx+len(marker):], "/")
	cleanRel := filepath.Clean(filepath.FromSlash(relativePath))
	if cleanRel == "." || strings.HasPrefix(cleanRel, "..") {
		return "", false
	}
	return filepath.ToSlash(cleanRel), true
}

func resolveProductImageFilePath(uploadDirs []string, relativePath string) (string, error) {
	cleanRel := filepath.Clean(relativePath)
	if cleanRel == "." || strings.HasPrefix(cleanRel, "..") {
		return "", fmt.Errorf("invalid image path")
	}

	seen := make(map[string]struct{})
	for _, uploadDir := range uploadDirs {
		trimmedDir := strings.TrimSpace(uploadDir)
		if trimmedDir == "" {
			continue
		}
		if _, exists := seen[trimmedDir]; exists {
			continue
		}
		seen[trimmedDir] = struct{}{}

		baseDir, err := filepath.Abs(filepath.Join(trimmedDir, "products"))
		if err != nil {
			continue
		}
		targetPath, err := filepath.Abs(filepath.Join(baseDir, cleanRel))
		if err != nil {
			continue
		}
		if targetPath != baseDir && !strings.HasPrefix(targetPath, baseDir+string(os.PathSeparator)) {
			continue
		}
		if _, err := os.Stat(targetPath); err == nil {
			return targetPath, nil
		}
	}

	return "", os.ErrNotExist
}

// CreateProduct CreateProduct
func (s *ProductService) CreateProduct(product *models.Product) error {
	// 验证SKU唯一性
	existing, err := s.productRepo.FindBySKU(product.SKU)
	if err == nil && existing != nil && existing.ID != 0 {
		return newProductSKUAlreadyExistsError()
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	// Validate required fields
	if product.Name == "" {
		return bizerr.New("product.nameRequired", "Product name cannot be empty")
	}
	if product.SKU == "" {
		return bizerr.New("product.skuRequired", "Product SKU cannot be empty")
	}

	// Price validation
	if product.Price < 0 {
		return bizerr.New("product.priceNegative", "Product price must be greater than or equal to 0")
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
			return newProductSKUAlreadyExistsError()
		}
		return err
	}
	return nil
}

// UpdateProduct UpdateProduct
func (s *ProductService) UpdateProduct(id uint, updates *models.Product) error {
	product, err := s.productRepo.FindByID(id)
	if err != nil {
		return ErrProductNotFound
	}

	// 如果UpdateSKU，检查唯一性
	if updates.SKU != "" && updates.SKU != product.SKU {
		existing, findErr := s.productRepo.FindBySKU(updates.SKU)
		if findErr == nil && existing != nil && existing.ID != 0 {
			return newProductSKUAlreadyExistsError()
		}
		if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return findErr
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
			return newProductSKUAlreadyExistsError()
		}
		return err
	}
	return nil
}

// DeleteProduct DeleteProduct
func (s *ProductService) DeleteProduct(id uint) error {
	return s.DeleteProductWithOptions(id, DeleteProductOptions{DeleteImages: true})
}

func (s *ProductService) DeleteProductWithOptions(id uint, options DeleteProductOptions) error {
	product, err := s.productRepo.FindByID(id)
	if err != nil {
		return ErrProductNotFound
	}

	// Delete product associated image files
	if options.DeleteImages {
		if err := s.deleteProductImages(product); err != nil {
			// Log error but do not block product deletion
			fmt.Printf("Warning: Failed to delete product images: %v\n", err)
		}
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
	uploadDir, _ := s.currentUploadRuntime()
	relativePath, ok := extractProductImageRelativePath(imageURL)
	if !ok {
		// 不是本地上传的图片（可能是外部URL），跳过
		return nil
	}

	filePath, err := resolveProductImageFilePath([]string{uploadDir, s.uploadDir}, relativePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

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
		return nil, ErrProductNotFound
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
		return ErrProductNotFound
	}

	product.Status = status
	return s.productRepo.Update(product)
}

// UpdateStock Update inventory
func (s *ProductService) UpdateStock(id uint, stock int) error {
	if stock < 0 {
		return bizerr.New("product.stockNegative", "Stock cannot be negative")
	}

	product, err := s.productRepo.FindByID(id)
	if err != nil {
		return ErrProductNotFound
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
		return bizerr.New("product.quantityInvalid", "Quantity must be greater than 0")
	}

	product, err := s.productRepo.FindByID(id)
	if err != nil {
		return ErrProductNotFound
	}

	if product.Stock < quantity {
		return bizerr.New("product.stockInsufficient", "Insufficient product stock").
			WithParams(map[string]interface{}{
				"available": product.Stock,
				"requested": quantity,
			})
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
		return ErrProductNotFound
	}

	product.IsFeatured = !product.IsFeatured
	return s.productRepo.Update(product)
}

func newProductSKUAlreadyExistsError() error {
	return bizerr.New("product.skuAlreadyExists", "SKU already exists")
}
