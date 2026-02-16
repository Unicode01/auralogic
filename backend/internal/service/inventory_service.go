package service

import (
	"fmt"

	"auralogic/internal/models"
	"auralogic/internal/repository"
)

type InventoryService struct {
	inventoryRepo *repository.InventoryRepository
	productRepo   *repository.ProductRepository
}

func NewInventoryService(inventoryRepo *repository.InventoryRepository, productRepo *repository.ProductRepository) *InventoryService {
	return &InventoryService{
		inventoryRepo: inventoryRepo,
		productRepo:   productRepo,
	}
}

// CreateInventory 创建Inventory配置（独立创建，不依赖Product）
func (s *InventoryService) CreateInventory(name, sku string, attrs map[string]string, stock, availableQuantity, safetyStock int) (*models.Inventory, error) {
	// 1. 验证Inventory名称不能为空
	if name == "" {
		return nil, fmt.Errorf("Inventory name cannot be empty")
	}

	// 2. 检查SKU是否已存在（如果提供了SKU，则检查是否已存在）
	if sku != "" {
		var existing models.Inventory
		err := s.inventoryRepo.FindBySKU(sku, &existing)
		if err == nil && existing.ID > 0 {
			return nil, fmt.Errorf("SKU already exists")
		}
	}

	// 3. 创建Inventory记录
	inventory := &models.Inventory{
		Name:              name,
		SKU:               sku,
		Stock:             stock,
		AvailableQuantity: availableQuantity,
		SafetyStock:       safetyStock,
		IsActive:          true,
	}

	// 设置规格组合并计算哈希
	if err := inventory.SetAttributes(attrs); err != nil {
		return nil, fmt.Errorf("Failed to set attributes: %w", err)
	}

	if err := s.inventoryRepo.Create(inventory); err != nil {
		return nil, err
	}

	return inventory, nil
}

// UpdateInventory 更新Inventory记录
func (s *InventoryService) UpdateInventory(id uint, stock, availableQuantity, safetyStock int, isActive bool) error {
	inventory, err := s.inventoryRepo.FindByID(id)
	if err != nil {
		return fmt.Errorf("Inventory record does not exist: %w", err)
	}

	inventory.Stock = stock
	inventory.AvailableQuantity = availableQuantity
	inventory.SafetyStock = safetyStock
	inventory.IsActive = isActive

	return s.inventoryRepo.Update(inventory)
}

// GetInventory 获取Inventory详情
func (s *InventoryService) GetInventory(id uint) (*models.Inventory, error) {
	return s.inventoryRepo.FindByID(id)
}

// GetProductInventories 获取Product的所有Inventory记录（已废弃，使用BindingService代替）
// 此方法保留是为了向后兼容，但新系统应该通过BindingService获取Product的Inventory
func (s *InventoryService) GetProductInventories(productID uint) ([]models.Inventory, error) {
	// 返回空列表，提示使用新API
	return []models.Inventory{}, fmt.Errorf("This method is deprecated, please use GET /api/admin/products/{id}/inventory-bindings to get product inventory bindings")
}

// ListInventories 分页获取Inventory列表
func (s *InventoryService) ListInventories(page, limit int, filters map[string]interface{}) ([]models.Inventory, int64, error) {
	return s.inventoryRepo.List(page, limit, filters)
}

// DeleteInventory 删除Inventory记录
func (s *InventoryService) DeleteInventory(id uint) error {
	inventory, err := s.inventoryRepo.FindByID(id)
	if err != nil {
		return fmt.Errorf("Inventory record does not exist: %w", err)
	}

	// 检查是否有已售或预留库存
	if inventory.SoldQuantity > 0 || inventory.ReservedQuantity > 0 {
		return fmt.Errorf("This inventory record has sales or reservations, cannot delete")
	}

	return s.inventoryRepo.Delete(id)
}

// CheckAndReserve 检查Inventory并预留库存（下单时调用，直接使用InventoryID）
func (s *InventoryService) CheckAndReserve(inventoryID uint, quantity int, orderNo string) (*models.Inventory, error) {
	// 1. 查找Inventory记录
	inventory, err := s.inventoryRepo.FindByID(inventoryID)
	if err != nil {
		return nil, fmt.Errorf("Inventory record does not exist")
	}

	// 2. 检查是否可以购买
	if canPurchase, msg := inventory.CanPurchase(quantity); !canPurchase {
		return nil, fmt.Errorf("%s", msg)
	}

	// 3. 预留Inventory
	if err := s.inventoryRepo.Reserve(inventory.ID, quantity, orderNo); err != nil {
		return nil, err
	}

	// 4. 重新getUpdate后的InventoryInfo
	return s.inventoryRepo.FindByID(inventory.ID)
}

// ConfirmPurchase 确认购买（支付Success时调用，直接使用InventoryID）
func (s *InventoryService) ConfirmPurchase(inventoryID uint, quantity int, orderNo string) error {
	// 扣减Inventory
	return s.inventoryRepo.Deduct(inventoryID, quantity, orderNo)
}

// CancelReserve 取消预留（取消Order时调用，直接使用InventoryID）
func (s *InventoryService) CancelReserve(inventoryID uint, quantity int, orderNo string) error {
	return s.inventoryRepo.ReleaseReserve(inventoryID, quantity, orderNo)
}

// AdjustStock 调整库存（入库、盘点等）- 旧方法保留用于兼容
func (s *InventoryService) AdjustStock(id uint, newStock, newAvailable int, operator, reason string) error {
	return s.inventoryRepo.Adjust(id, newStock, newAvailable, operator, reason)
}

// AdjustStockByDelta 通过增量调整库存（推荐使用，避免并发问题）
func (s *InventoryService) AdjustStockByDelta(id uint, stockDelta, availableDelta int, operator, reason string) error {
	return s.inventoryRepo.AdjustByDelta(id, stockDelta, availableDelta, operator, reason)
}

// GetLowStockList get低Inventory列表
func (s *InventoryService) GetLowStockList() ([]models.Inventory, error) {
	return s.inventoryRepo.GetLowStockList()
}

// BatchCheckStock 批量检查Inventory（用于购物车等场景，直接使用InventoryID）
func (s *InventoryService) BatchCheckStock(items []struct {
	InventoryID uint
	Quantity    int
}) (map[uint]bool, []string) {
	result := make(map[uint]bool)
	errors := make([]string, 0)

	for _, item := range items {
		inventory, err := s.inventoryRepo.FindByID(item.InventoryID)
		if err != nil {
			result[item.InventoryID] = false
			errors = append(errors, fmt.Sprintf("Inventory ID %d: inventory record not found", item.InventoryID))
			continue
		}

		canPurchase, msg := inventory.CanPurchase(item.Quantity)
		result[item.InventoryID] = canPurchase
		if !canPurchase {
			errors = append(errors, fmt.Sprintf("InventoryID %d: %s", item.InventoryID, msg))
		}
	}

	return result, errors
}
