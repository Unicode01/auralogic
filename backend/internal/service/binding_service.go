package service

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"auralogic/internal/models"
	"auralogic/internal/repository"
)

type BindingService struct {
	bindingRepo   *repository.BindingRepository
	inventoryRepo *repository.InventoryRepository
	productRepo   *repository.ProductRepository
}

func NewBindingService(
	bindingRepo *repository.BindingRepository,
	inventoryRepo *repository.InventoryRepository,
	productRepo *repository.ProductRepository,
) *BindingService {
	return &BindingService{
		bindingRepo:   bindingRepo,
		inventoryRepo: inventoryRepo,
		productRepo:   productRepo,
	}
}

// CreateBinding 创建Product-Inventory绑定
func (s *BindingService) CreateBinding(productID, inventoryID uint, isRandom bool, priority int, notes string) (*models.ProductInventoryBinding, error) {
	// 1. 验证Product是否存在
	product, err := s.productRepo.FindByID(productID)
	if err != nil {
		return nil, fmt.Errorf("Productdoes not exist: %w", err)
	}

	// 2. 验证Inventory是否存在
	inventory, err := s.inventoryRepo.FindByID(inventoryID)
	if err != nil {
		return nil, fmt.Errorf("Inventory configuration does not exist: %w", err)
	}

	// 3. 从备注中提取规格组合（兼容旧格式和新格式）
	var attributes map[string]string

	if notes != "" {
		// 尝试直接解析JSON
		if err := json.Unmarshal([]byte(notes), &attributes); err != nil {
			attributes = make(map[string]string)
		}
	}

	if attributes == nil {
		attributes = make(map[string]string)
	}

	// 计算规格组合哈希
	normalizedAttrs := models.NormalizeAttributes(attributes)
	attributesHash := models.GenerateAttributesHash(normalizedAttrs)

	// 4. 检查该商品的该库存组合是否已绑定
	exists, err := s.bindingRepo.ExistsByAttributesHash(productID, attributesHash)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("This specification combination is already bound to inventory, do not bind again")
	}

	// 5. 将规格组合转换为JSON类型
	var attributesJSON models.JSON
	if len(normalizedAttrs) > 0 {
		jsonData, _ := json.Marshal(normalizedAttrs)
		attributesJSON = models.JSON(jsonData)
	} else {
		attributesJSON = models.JSON("{}")
	}

	// 6. Create绑定
	binding := &models.ProductInventoryBinding{
		ProductID:      productID,
		InventoryID:    inventoryID,
		Attributes:     attributesJSON,
		AttributesHash: attributesHash,
		IsRandom:       isRandom,
		Priority:       priority,
		Notes:          "", // notes现在用于真正的备注，不再存储规格组合（旧格式）
	}

	if err := s.bindingRepo.Create(binding); err != nil {
		return nil, err
	}

	// 预加载关联数据（商品和库存）
	binding.Product = product
	binding.Inventory = inventory

	return binding, nil
}

// UpdateBinding 更新绑定关系
func (s *BindingService) UpdateBinding(id uint, isRandom bool, priority int, notes string) error {
	binding, err := s.bindingRepo.FindByID(id)
	if err != nil {
		return fmt.Errorf("Binding relationship does not exist: %w", err)
	}

	binding.IsRandom = isRandom
	binding.Priority = priority
	binding.Notes = notes

	return s.bindingRepo.Update(binding)
}

// DeleteBinding 删除绑定关系
func (s *BindingService) DeleteBinding(id uint) error {
	binding, err := s.bindingRepo.FindByID(id)
	if err != nil {
		return fmt.Errorf("Binding relationship does not exist: %w", err)
	}

	return s.bindingRepo.Delete(binding.ID)
}

// DeleteAllProductBindings 删除商品的所有绑定关系（批量删除）
func (s *BindingService) DeleteAllProductBindings(productID uint) (int, error) {
	// 获取所有绑定
	bindings, err := s.bindingRepo.FindByProductID(productID)
	if err != nil {
		return 0, err
	}

	// 批量删除
	count := 0
	for _, binding := range bindings {
		if err := s.bindingRepo.Delete(binding.ID); err != nil {
			return count, err
		}
		count++
	}

	return count, nil
}

// GetProductBindings 获取商品的所有库存绑定（包含库存详情）
func (s *BindingService) GetProductBindings(productID uint) ([]models.ProductInventoryBinding, error) {
	return s.bindingRepo.FindByProductID(productID)
}

// GetInventoryBindings 获取库存的所有商品绑定
func (s *BindingService) GetInventoryBindings(inventoryID uint) ([]models.ProductInventoryBinding, error) {
	return s.bindingRepo.FindByInventoryID(inventoryID)
}

// SelectRandomInventory 盲盒模式：根据权重随机选择库存
// 返回：选中的库存 + 完整的属性组合
func (s *BindingService) SelectRandomInventory(productID uint, quantity int) (*models.Inventory, map[string]string, error) {
	// 1. 获取所有参与随机分配的绑定
	allBindings, err := s.bindingRepo.FindByProductID(productID)
	if err != nil {
		return nil, nil, err
	}

	if len(allBindings) == 0 {
		return nil, nil, fmt.Errorf("This product has no inventory configured")
	}

	// 2. 筛选出有足够库存的绑定（盲盒模式或包含盲盒属性的）
	var availableBindings []models.ProductInventoryBinding
	for _, binding := range allBindings {
		if binding.Inventory == nil {
			continue
		}
		// 检查库存是否足够
		if binding.Inventory.IsActive && binding.Inventory.GetAvailableStock() >= quantity {
			availableBindings = append(availableBindings, binding)
		}
	}

	if len(availableBindings) == 0 {
		return nil, nil, fmt.Errorf("Not enough inventory available for allocation")
	}

	// 辅助函数：从绑定中提取完整规格组合
	getFullAttributes := func(binding models.ProductInventoryBinding) map[string]string {
		var attrs map[string]string
		if len(binding.Attributes) > 0 {
			json.Unmarshal([]byte(binding.Attributes), &attrs)
		}
		return attrs
	}

	// 3. 根据权重随机选择
	totalWeight := 0
	for _, binding := range availableBindings {
		totalWeight += binding.Priority
	}

	var selectedBinding *models.ProductInventoryBinding

	if totalWeight == 0 {
		// 如果所有权重都是0，则随机选择
		rand.Seed(time.Now().UnixNano())
		randomIndex := rand.Intn(len(availableBindings))
		selectedBinding = &availableBindings[randomIndex]
	} else {
		// 使用加权随机算法
		rand.Seed(time.Now().UnixNano())
		randomValue := rand.Intn(totalWeight)
		currentWeight := 0

		for _, binding := range availableBindings {
			currentWeight += binding.Priority
			if randomValue < currentWeight {
				selectedBinding = &binding
				break
			}
		}

		// 兜底：返回最后一个
		if selectedBinding == nil {
			selectedBinding = &availableBindings[len(availableBindings)-1]
		}
	}

	fullAttrs := getFullAttributes(*selectedBinding)
	return selectedBinding.Inventory, fullAttrs, nil
}

// FindInventoryWithPartialMatch 混合模式：部分属性匹配 + 盲盒随机
// 用于处理：用户选择部分属性，其他属性由系统随机分配
// 返回：选中的库存 + 完整的属性组合（包括随机分配的）
func (s *BindingService) FindInventoryWithPartialMatch(productID uint, userAttributes map[string]string, quantity int) (*models.Inventory, map[string]string, error) {
	// 1. 获取所有绑定
	bindings, err := s.bindingRepo.FindByProductID(productID)
	if err != nil {
		return nil, nil, err
	}

	if len(bindings) == 0 {
		return nil, nil, fmt.Errorf("This product has no inventory configured")
	}

	// 2. 筛选出包含用户选择属性的绑定（部分匹配）
	var matchedBindings []models.ProductInventoryBinding
	for _, binding := range bindings {
		if binding.Inventory == nil || !binding.Inventory.IsActive {
			continue
		}

		// 解析绑定的规格组合
		var bindingAttrs map[string]string
		if len(binding.Attributes) > 0 {
			if err := json.Unmarshal([]byte(binding.Attributes), &bindingAttrs); err != nil {
				continue
			}

			// 检查是否包含用户选择的所有属性（部分匹配）
			isMatch := true
			for userKey, userValue := range userAttributes {
				if bindingValue, exists := bindingAttrs[userKey]; !exists || bindingValue != userValue {
					isMatch = false
					break
				}
			}

			if isMatch && binding.Inventory.GetAvailableStock() >= quantity {
				matchedBindings = append(matchedBindings, binding)
			}
		}
	}

	if len(matchedBindings) == 0 {
		return nil, nil, fmt.Errorf("No matching inventory configuration found")
	}

	// 辅助函数：从绑定中提取完整规格组合
	getFullAttributes := func(binding models.ProductInventoryBinding) map[string]string {
		var attrs map[string]string
		if len(binding.Attributes) > 0 {
			json.Unmarshal([]byte(binding.Attributes), &attrs)
		}
		return attrs
	}

	// 3. 如果只有一个匹配，直接返回（固定模式）
	if len(matchedBindings) == 1 {
		selectedBinding := matchedBindings[0]
		fullAttrs := getFullAttributes(selectedBinding)
		return selectedBinding.Inventory, fullAttrs, nil
	}

	// 4. 如果有多个匹配，根据权重随机选择（盲盒模式）
	totalWeight := 0
	for _, binding := range matchedBindings {
		totalWeight += binding.Priority
	}

	var selectedBinding *models.ProductInventoryBinding

	if totalWeight == 0 {
		// 如果所有权重都是0，则随机选择
		rand.Seed(time.Now().UnixNano())
		randomIndex := rand.Intn(len(matchedBindings))
		selectedBinding = &matchedBindings[randomIndex]
	} else {
		// 使用加权随机算法
		rand.Seed(time.Now().UnixNano())
		randomValue := rand.Intn(totalWeight)
		currentWeight := 0

		for _, binding := range matchedBindings {
			currentWeight += binding.Priority
			if randomValue < currentWeight {
				selectedBinding = &binding
				break
			}
		}

		// 兜底：返回最后一个
		if selectedBinding == nil {
			selectedBinding = &matchedBindings[len(matchedBindings)-1]
		}
	}

	fullAttrs := getFullAttributes(*selectedBinding)
	return selectedBinding.Inventory, fullAttrs, nil
}

// FindInventoryByAttributes 固定模式：根据规格组合查找对应的库存（完全匹配）
// 返回：选中的库存 + 完整的规格组合
func (s *BindingService) FindInventoryByAttributes(productID uint, attributes map[string]string) (*models.Inventory, map[string]string, error) {
	// 1. 获取商品的所有非随机绑定
	bindings, err := s.bindingRepo.FindFixedBindings(productID)
	if err != nil {
		return nil, nil, err
	}

	if len(bindings) == 0 {
		return nil, nil, fmt.Errorf("This product has no inventory configured with fixed attributes")
	}

	// 2. 规范化用户选择的规格组合
	normalizedAttrs := models.NormalizeAttributes(attributes)
	attrsHash := models.GenerateAttributesHash(normalizedAttrs)

	// 3. 查找匹配的库存
	for _, binding := range bindings {
		if binding.Inventory == nil {
			continue
		}

		// 检查绑定的规格组合哈希是否匹配
		if binding.AttributesHash == attrsHash {
			// 验证Inventory是否可用
			if !binding.Inventory.IsActive {
				return nil, nil, fmt.Errorf("This specification is unavailable")
			}

			// 获取完整规格组合
			var fullAttrs map[string]string
			if len(binding.Attributes) > 0 {
				json.Unmarshal([]byte(binding.Attributes), &fullAttrs)
			}
			if fullAttrs == nil {
				fullAttrs = make(map[string]string)
			}

			return binding.Inventory, fullAttrs, nil
		}
	}
	return nil, nil, fmt.Errorf("No matching inventory configuration found")
}

// GetTotalAvailableStock 获取商品的总可用库存（去重：同一个库存只计算一次）
func (s *BindingService) GetTotalAvailableStock(productID uint) (int, error) {
	bindings, err := s.bindingRepo.FindByProductID(productID)
	if err != nil {
		return 0, err
	}

	// 使用 map 去重：同一个库存ID只计算一次
	inventoryMap := make(map[uint]*models.Inventory)
	for _, binding := range bindings {
		if binding.Inventory != nil && binding.Inventory.IsActive {
			// 只有当这个库存ID还没被记录时，才添加到map
			if _, exists := inventoryMap[binding.InventoryID]; !exists {
				inventoryMap[binding.InventoryID] = binding.Inventory
			}
		}
	}

	// 累加去重后的库存
	totalStock := 0
	for _, inventory := range inventoryMap {
		totalStock += inventory.GetAvailableStock()
	}

	return totalStock, nil
}

// GetAvailableStockByAttributes 根据规格组合获取商品可用库存
func (s *BindingService) GetAvailableStockByAttributes(productID uint, attributes map[string]string) (int, error) {
	// 如果没有规格组合，返回总库存
	if len(attributes) == 0 {
		return s.GetTotalAvailableStock(productID)
	}

	// 规范化规格组合并计算哈希
	normalizedAttrs := models.NormalizeAttributes(attributes)
	attrsHash := models.GenerateAttributesHash(normalizedAttrs)

	// 获取所有绑定
	bindings, err := s.bindingRepo.FindByProductID(productID)
	if err != nil {
		return 0, err
	}

	// 查找精确匹配的库存绑定
	for _, binding := range bindings {
		if binding.AttributesHash == attrsHash {
			if binding.Inventory != nil && binding.Inventory.IsActive {
				return binding.Inventory.GetAvailableStock(), nil
			}
			return 0, nil
		}
	}

	// 如果没有精确匹配，尝试部分匹配（用于混合模式：用户选择部分属性，其他属性由系统随机分配）
	totalStock := 0
	inventoryMap := make(map[uint]bool) // 用于去重
	for _, binding := range bindings {
		if binding.Inventory == nil || !binding.Inventory.IsActive {
			continue
		}

		// 解析绑定的规格组合
		var bindingAttrs map[string]string
		if len(binding.Attributes) > 0 {
			if err := json.Unmarshal([]byte(binding.Attributes), &bindingAttrs); err != nil {
				continue
			}
		}

		// 检查是否包含用户选择的所有属性（部分匹配）
		isMatch := true
		for userKey, userValue := range normalizedAttrs {
			if bindingValue, exists := bindingAttrs[userKey]; !exists || bindingValue != userValue {
				isMatch = false
				break
			}
		}

		if isMatch {
			// 去重：同一个库存只计算一次
			if _, exists := inventoryMap[binding.InventoryID]; !exists {
				inventoryMap[binding.InventoryID] = true
				totalStock += binding.Inventory.GetAvailableStock()
			}
		}
	}

	return totalStock, nil
}
