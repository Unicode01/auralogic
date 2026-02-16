package service

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
	"auralogic/internal/config"
	"auralogic/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type VirtualInventoryService struct {
	db  *gorm.DB
	cfg *config.Config
}

func NewVirtualInventoryService(db *gorm.DB) *VirtualInventoryService {
	return &VirtualInventoryService{
		db:  db,
		cfg: config.GetConfig(),
	}
}

// CreateVirtualInventory 创建虚拟库存
func (s *VirtualInventoryService) CreateVirtualInventory(inventory *models.VirtualInventory) error {
	return s.db.Create(inventory).Error
}

// GetVirtualInventory 获取虚拟库存详情
func (s *VirtualInventoryService) GetVirtualInventory(id uint) (*models.VirtualInventory, error) {
	var inventory models.VirtualInventory
	if err := s.db.First(&inventory, id).Error; err != nil {
		return nil, err
	}
	return &inventory, nil
}

// UpdateVirtualInventory 更新虚拟库存
func (s *VirtualInventoryService) UpdateVirtualInventory(id uint, updates map[string]interface{}) error {
	return s.db.Model(&models.VirtualInventory{}).Where("id = ?", id).Updates(updates).Error
}

// DeleteVirtualInventory 删除虚拟库存（只有没有关联的可用库存项时才能删除）
func (s *VirtualInventoryService) DeleteVirtualInventory(id uint) error {
	// 检查是否有关联的库存项
	var count int64
	if err := s.db.Model(&models.VirtualProductStock{}).
		Where("virtual_inventory_id = ?", id).
		Count(&count).Error; err != nil {
		return err
	}

	if count > 0 {
		return errors.New("cannot delete virtual inventory with existing stock items")
	}

	// 检查是否有绑定的商品
	if err := s.db.Model(&models.ProductVirtualInventoryBinding{}).
		Where("virtual_inventory_id = ?", id).
		Count(&count).Error; err != nil {
		return err
	}

	if count > 0 {
		return errors.New("cannot delete virtual inventory with product bindings")
	}

	return s.db.Delete(&models.VirtualInventory{}, id).Error
}

// ListVirtualInventories 获取虚拟库存列表
func (s *VirtualInventoryService) ListVirtualInventories(page, limit int, search string) ([]models.VirtualInventoryWithStats, int64, error) {
	var inventories []models.VirtualInventory
	var total int64

	query := s.db.Model(&models.VirtualInventory{})

	if search != "" {
		query = query.Where("name LIKE ? OR sku LIKE ?", "%"+search+"%", "%"+search+"%")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&inventories).Error; err != nil {
		return nil, 0, err
	}

	// 获取每个库存的统计信息
	var result []models.VirtualInventoryWithStats
	for _, inv := range inventories {
		stats, _ := s.GetStockStats(inv.ID)
		result = append(result, models.VirtualInventoryWithStats{
			ID:          inv.ID,
			Name:        inv.Name,
			SKU:         inv.SKU,
			Description: inv.Description,
			IsActive:    inv.IsActive,
			Notes:       inv.Notes,
			Total:       stats["total"],
			Available:   stats["available"],
			Reserved:    stats["reserved"],
			Sold:        stats["sold"],
			CreatedAt:   inv.CreatedAt,
		})
	}

	return result, total, nil
}

// GetStockStats 获取库存统计
func (s *VirtualInventoryService) GetStockStats(virtualInventoryID uint) (map[string]int64, error) {
	stats := make(map[string]int64)

	// 总数
	var total int64
	if err := s.db.Model(&models.VirtualProductStock{}).
		Where("virtual_inventory_id = ?", virtualInventoryID).
		Count(&total).Error; err != nil {
		return nil, err
	}
	stats["total"] = total

	// 可用
	var available int64
	if err := s.db.Model(&models.VirtualProductStock{}).
		Where("virtual_inventory_id = ? AND status = ?", virtualInventoryID, models.VirtualStockStatusAvailable).
		Count(&available).Error; err != nil {
		return nil, err
	}
	stats["available"] = available

	// 已预留
	var reserved int64
	if err := s.db.Model(&models.VirtualProductStock{}).
		Where("virtual_inventory_id = ? AND status = ?", virtualInventoryID, models.VirtualStockStatusReserved).
		Count(&reserved).Error; err != nil {
		return nil, err
	}
	stats["reserved"] = reserved

	// 已售出
	var sold int64
	if err := s.db.Model(&models.VirtualProductStock{}).
		Where("virtual_inventory_id = ? AND status = ?", virtualInventoryID, models.VirtualStockStatusSold).
		Count(&sold).Error; err != nil {
		return nil, err
	}
	stats["sold"] = sold

	return stats, nil
}

// ImportFromExcel 从Excel导入虚拟产品库存
func (s *VirtualInventoryService) ImportFromExcel(virtualInventoryID uint, filePath string, importedBy string) (int, error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to open excel file: %w", err)
	}
	defer f.Close()

	// 读取第一个工作表
	sheetName := f.GetSheetName(0)
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return 0, fmt.Errorf("failed to read excel rows: %w", err)
	}

	if len(rows) == 0 {
		return 0, errors.New("excel file is empty")
	}

	// 生成批次号
	batchNo := fmt.Sprintf("BATCH-%s", time.Now().Format("20060102150405"))

	var stocks []models.VirtualProductStock

	// 跳过第一行标题（如果有）
	startRow := 0
	if len(rows) > 0 && (strings.ToLower(rows[0][0]) == "content" || strings.ToLower(rows[0][0]) == "卡密" || strings.ToLower(rows[0][0]) == "激活码") {
		startRow = 1
	}

	for i := startRow; i < len(rows); i++ {
		row := rows[i]
		if len(row) == 0 || row[0] == "" {
			continue
		}

		content := strings.TrimSpace(row[0])
		remark := ""
		if len(row) > 1 {
			remark = strings.TrimSpace(row[1])
		}

		stock := models.VirtualProductStock{
			VirtualInventoryID: virtualInventoryID,
			Content:            content,
			Remark:             remark,
			Status:             models.VirtualStockStatusAvailable,
			BatchNo:            batchNo,
			ImportedBy:         importedBy,
		}
		stocks = append(stocks, stock)
	}

	if len(stocks) == 0 {
		return 0, errors.New("no valid data found in excel file")
	}

	// 批量插入
	if err := s.db.Create(&stocks).Error; err != nil {
		return 0, fmt.Errorf("failed to insert stocks: %w", err)
	}

	return len(stocks), nil
}

// ImportFromText 从文本文件导入（每行一个卡密）
func (s *VirtualInventoryService) ImportFromText(virtualInventoryID uint, content string, importedBy string) (int, error) {
	lines := strings.Split(content, "\n")

	// 生成批次号
	batchNo := fmt.Sprintf("BATCH-%s", time.Now().Format("20060102150405"))

	var stocks []models.VirtualProductStock

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 支持 "卡密,备注" 格式
		parts := strings.SplitN(line, ",", 2)
		contentVal := strings.TrimSpace(parts[0])
		remark := ""
		if len(parts) > 1 {
			remark = strings.TrimSpace(parts[1])
		}

		stock := models.VirtualProductStock{
			VirtualInventoryID: virtualInventoryID,
			Content:            contentVal,
			Remark:             remark,
			Status:             models.VirtualStockStatusAvailable,
			BatchNo:            batchNo,
			ImportedBy:         importedBy,
		}
		stocks = append(stocks, stock)
	}

	if len(stocks) == 0 {
		return 0, errors.New("no valid data found in text content")
	}

	// 批量插入
	if err := s.db.Create(&stocks).Error; err != nil {
		return 0, fmt.Errorf("failed to insert stocks: %w", err)
	}

	return len(stocks), nil
}

// ImportFromCSV 从CSV导入
func (s *VirtualInventoryService) ImportFromCSV(virtualInventoryID uint, reader io.Reader, importedBy string) (int, error) {
	csvReader := csv.NewReader(reader)

	// 生成批次号
	batchNo := fmt.Sprintf("BATCH-%s", time.Now().Format("20060102150405"))

	var stocks []models.VirtualProductStock
	isFirstRow := true

	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, fmt.Errorf("failed to read csv: %w", err)
		}

		// 跳过标题行
		if isFirstRow {
			isFirstRow = false
			if strings.ToLower(record[0]) == "content" || strings.ToLower(record[0]) == "卡密" {
				continue
			}
		}

		if len(record) == 0 || record[0] == "" {
			continue
		}

		contentVal := strings.TrimSpace(record[0])
		remark := ""
		if len(record) > 1 {
			remark = strings.TrimSpace(record[1])
		}

		stock := models.VirtualProductStock{
			VirtualInventoryID: virtualInventoryID,
			Content:            contentVal,
			Remark:             remark,
			Status:             models.VirtualStockStatusAvailable,
			BatchNo:            batchNo,
			ImportedBy:         importedBy,
		}
		stocks = append(stocks, stock)
	}

	if len(stocks) == 0 {
		return 0, errors.New("no valid data found in csv file")
	}

	// 批量插入
	if err := s.db.Create(&stocks).Error; err != nil {
		return 0, fmt.Errorf("failed to insert stocks: %w", err)
	}

	return len(stocks), nil
}

// CreateStockManually 手动创建单个库存项
func (s *VirtualInventoryService) CreateStockManually(virtualInventoryID uint, content, remark, importedBy string) (*models.VirtualProductStock, error) {
	stock := &models.VirtualProductStock{
		VirtualInventoryID: virtualInventoryID,
		Content:            content,
		Remark:             remark,
		Status:             models.VirtualStockStatusAvailable,
		BatchNo:            fmt.Sprintf("MANUAL-%s", time.Now().Format("20060102150405")),
		ImportedBy:         importedBy,
	}

	if err := s.db.Create(stock).Error; err != nil {
		return nil, err
	}

	return stock, nil
}

// ListStocks 获取库存列表
func (s *VirtualInventoryService) ListStocks(virtualInventoryID uint, status string, page, limit int) ([]models.VirtualProductStock, int64, error) {
	var stocks []models.VirtualProductStock
	var total int64

	query := s.db.Model(&models.VirtualProductStock{}).Where("virtual_inventory_id = ?", virtualInventoryID)

	if status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&stocks).Error; err != nil {
		return nil, 0, err
	}

	return stocks, total, nil
}

// DeleteStock 删除库存（允许删除可用和已预留状态的）
func (s *VirtualInventoryService) DeleteStock(id uint) error {
	var stock models.VirtualProductStock
	if err := s.db.First(&stock, id).Error; err != nil {
		return err
	}

	if stock.Status != models.VirtualStockStatusAvailable && stock.Status != models.VirtualStockStatusReserved {
		return errors.New("only available or reserved stock can be deleted")
	}

	return s.db.Delete(&stock).Error
}

// DeleteBatch 删除整个批次
func (s *VirtualInventoryService) DeleteBatch(batchNo string) (int64, error) {
	result := s.db.Where("batch_no = ? AND status = ?", batchNo, models.VirtualStockStatusAvailable).
		Delete(&models.VirtualProductStock{})

	if result.Error != nil {
		return 0, result.Error
	}

	return result.RowsAffected, nil
}

// GetVirtualInventoryWithStats 获取带统计信息的虚拟库存
func (s *VirtualInventoryService) GetVirtualInventoryWithStats(id uint) (*models.VirtualInventoryWithStats, error) {
	inventory, err := s.GetVirtualInventory(id)
	if err != nil {
		return nil, err
	}

	stats, _ := s.GetStockStats(id)

	return &models.VirtualInventoryWithStats{
		ID:          inventory.ID,
		Name:        inventory.Name,
		SKU:         inventory.SKU,
		Description: inventory.Description,
		IsActive:    inventory.IsActive,
		Notes:       inventory.Notes,
		Total:       stats["total"],
		Available:   stats["available"],
		Reserved:    stats["reserved"],
		Sold:        stats["sold"],
		CreatedAt:   inventory.CreatedAt,
	}, nil
}

// ========== 商品绑定相关 ==========

// CreateBinding 创建商品-虚拟库存绑定
// 注意：此方法用于简单创建，不带规格属性。带规格属性请使用 CreateBindingWithAttributes
func (s *VirtualInventoryService) CreateBinding(productID, virtualInventoryID uint, isRandom bool, priority int, notes string) (*models.ProductVirtualInventoryBinding, error) {
	return s.CreateBindingWithAttributes(productID, virtualInventoryID, nil, isRandom, priority, notes)
}

// CreateBindingWithAttributes 创建带规格属性的商品-虚拟库存绑定
// 与实体库存采用相同的绑定制设计，使用 AttributesHash 确保唯一性
func (s *VirtualInventoryService) CreateBindingWithAttributes(productID, virtualInventoryID uint, attributes map[string]string, isRandom bool, priority int, notes string) (*models.ProductVirtualInventoryBinding, error) {
	// 规范化属性并计算哈希
	normalizedAttrs := models.NormalizeAttributes(attributes)
	attributesHash := models.GenerateAttributesHash(normalizedAttrs)

	// 检查该商品的该规格组合是否已绑定
	var existingCount int64
	if err := s.db.Model(&models.ProductVirtualInventoryBinding{}).
		Where("product_id = ? AND attributes_hash = ?", productID, attributesHash).
		Count(&existingCount).Error; err != nil {
		return nil, err
	}

	if existingCount > 0 {
		return nil, errors.New("this specification combination is already bound to virtual inventory")
	}

	binding := &models.ProductVirtualInventoryBinding{
		ProductID:          productID,
		VirtualInventoryID: virtualInventoryID,
		Attributes:         models.JSONMap(normalizedAttrs),
		AttributesHash:     attributesHash,
		IsRandom:           isRandom,
		Priority:           priority,
		Notes:              notes,
	}

	if err := s.db.Create(binding).Error; err != nil {
		return nil, err
	}

	return binding, nil
}

// UpdateBinding 更新绑定
func (s *VirtualInventoryService) UpdateBinding(bindingID uint, isRandom bool, priority int, notes string) error {
	return s.db.Model(&models.ProductVirtualInventoryBinding{}).
		Where("id = ?", bindingID).
		Updates(map[string]interface{}{
			"is_random": isRandom,
			"priority":  priority,
			"notes":     notes,
		}).Error
}

// DeleteBinding 删除绑定
func (s *VirtualInventoryService) DeleteBinding(bindingID uint) error {
	return s.db.Delete(&models.ProductVirtualInventoryBinding{}, bindingID).Error
}

// GetProductBindings 获取商品的虚拟库存绑定列表
func (s *VirtualInventoryService) GetProductBindings(productID uint) ([]models.BindingWithVirtualInventoryInfo, error) {
	var bindings []models.ProductVirtualInventoryBinding
	if err := s.db.Where("product_id = ?", productID).
		Preload("VirtualInventory").
		Find(&bindings).Error; err != nil {
		return nil, err
	}

	var result []models.BindingWithVirtualInventoryInfo
	for _, binding := range bindings {
		stats, _ := s.GetStockStats(binding.VirtualInventoryID)

		var invWithStats *models.VirtualInventoryWithStats
		if binding.VirtualInventory != nil {
			invWithStats = &models.VirtualInventoryWithStats{
				ID:          binding.VirtualInventory.ID,
				Name:        binding.VirtualInventory.Name,
				SKU:         binding.VirtualInventory.SKU,
				Description: binding.VirtualInventory.Description,
				IsActive:    binding.VirtualInventory.IsActive,
				Notes:       binding.VirtualInventory.Notes,
				Total:       stats["total"],
				Available:   stats["available"],
				Reserved:    stats["reserved"],
				Sold:        stats["sold"],
				CreatedAt:   binding.VirtualInventory.CreatedAt,
			}
		}

		result = append(result, models.BindingWithVirtualInventoryInfo{
			ID:                 binding.ID,
			ProductID:          binding.ProductID,
			VirtualInventoryID: binding.VirtualInventoryID,
			Attributes:         binding.Attributes,
			AttributesHash:     binding.AttributesHash,
			IsRandom:           binding.IsRandom,
			Priority:           binding.Priority,
			Notes:              binding.Notes,
			VirtualInventory:   invWithStats,
			CreatedAt:          binding.CreatedAt,
		})
	}

	return result, nil
}

// SaveVariantBindings 批量保存商品的规格-虚拟库存绑定
// 与实体库存采用相同的绑定制设计，使用 AttributesHash 确保唯一性
func (s *VirtualInventoryService) SaveVariantBindings(productID uint, bindings []struct {
	Attributes         map[string]string `json:"attributes"`
	VirtualInventoryID *uint             `json:"virtual_inventory_id"`
	IsRandom           bool              `json:"is_random"`
	Priority           int               `json:"priority"`
}) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// 先删除该商品的所有现有绑定
		if err := tx.Where("product_id = ?", productID).Delete(&models.ProductVirtualInventoryBinding{}).Error; err != nil {
			return err
		}

		// 创建新的绑定
		for _, b := range bindings {
			if b.VirtualInventoryID == nil || *b.VirtualInventoryID == 0 {
				continue // 跳过未配置库存的规格
			}

			priority := b.Priority
			if priority <= 0 {
				priority = 1
			}

			// 规范化属性并计算哈希
			normalizedAttrs := models.NormalizeAttributes(b.Attributes)
			attributesHash := models.GenerateAttributesHash(normalizedAttrs)

			binding := &models.ProductVirtualInventoryBinding{
				ProductID:          productID,
				VirtualInventoryID: *b.VirtualInventoryID,
				Attributes:         models.JSONMap(normalizedAttrs),
				AttributesHash:     attributesHash,
				IsRandom:           b.IsRandom,
				Priority:           priority,
			}

			if err := tx.Create(binding).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// GetBindingByAttributes 根据规格属性获取绑定
// 使用 AttributesHash 进行高效的精确匹配，与实体库存采用相同的绑定制设计
func (s *VirtualInventoryService) GetBindingByAttributes(productID uint, attributes map[string]string) (*models.ProductVirtualInventoryBinding, error) {
	// 规范化属性并计算哈希
	normalizedAttrs := models.NormalizeAttributes(attributes)
	attributesHash := models.GenerateAttributesHash(normalizedAttrs)

	var binding models.ProductVirtualInventoryBinding
	err := s.db.Where("product_id = ? AND attributes_hash = ?", productID, attributesHash).
		Preload("VirtualInventory").
		First(&binding).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("no binding found for the given attributes")
		}
		return nil, err
	}

	return &binding, nil
}

// GetInventoryProducts 获取虚拟库存绑定的商品列表
func (s *VirtualInventoryService) GetInventoryProducts(virtualInventoryID uint) ([]models.ProductVirtualInventoryBinding, error) {
	var bindings []models.ProductVirtualInventoryBinding
	if err := s.db.Where("virtual_inventory_id = ?", virtualInventoryID).
		Preload("Product").
		Find(&bindings).Error; err != nil {
		return nil, err
	}
	return bindings, nil
}

// AllocateStockForProduct 为商品分配虚拟库存（通过绑定关系）
// 注意：此方法已废弃，请使用 AllocateStockForProductByAttributes
func (s *VirtualInventoryService) AllocateStockForProduct(productID uint, quantity int, orderNo string) ([]models.VirtualProductStock, error) {
	return s.AllocateStockForProductByAttributes(productID, quantity, orderNo, nil)
}

// AllocateStockForProductByAttributes 根据规格属性为商品分配虚拟库存
// 使用事务和行锁确保并发安全，防止超售
func (s *VirtualInventoryService) AllocateStockForProductByAttributes(productID uint, quantity int, orderNo string, attributes map[string]interface{}) ([]models.VirtualProductStock, error) {
	var bindings []models.BindingWithVirtualInventoryInfo
	var err error

	// 如果有属性，先尝试精确匹配
	if len(attributes) > 0 {
		// 转换 map[string]interface{} 为 map[string]string
		attrStrMap := make(map[string]string)
		for k, v := range attributes {
			if str, ok := v.(string); ok {
				attrStrMap[k] = str
			}
		}

		if len(attrStrMap) > 0 {
			// 根据属性查找精确匹配的绑定
			binding, err := s.GetBindingByAttributes(productID, attrStrMap)
			if err == nil && binding != nil {
				// 找到精确匹配，只从这个绑定中分配
				stats, _ := s.GetStockStats(binding.VirtualInventoryID)
				bindings = []models.BindingWithVirtualInventoryInfo{{
					ID:                 binding.ID,
					ProductID:          binding.ProductID,
					VirtualInventoryID: binding.VirtualInventoryID,
					Attributes:         binding.Attributes,
					AttributesHash:     binding.AttributesHash,
					IsRandom:           binding.IsRandom,
					Priority:           binding.Priority,
					VirtualInventory: &models.VirtualInventoryWithStats{
						Available: stats["available"],
					},
				}}
			}
		}
	}

	// 如果没有找到精确匹配，获取所有绑定
	if len(bindings) == 0 {
		bindings, err = s.GetProductBindings(productID)
		if err != nil {
			return nil, err
		}
	}

	if len(bindings) == 0 {
		return nil, errors.New("no virtual inventory bound to this product")
	}

	var allocatedStocks []models.VirtualProductStock

	// 使用事务确保并发安全
	err = s.db.Transaction(func(tx *gorm.DB) error {
		remainingQuantity := quantity

		// 按优先级从绑定的库存中分配
		for _, binding := range bindings {
			if remainingQuantity <= 0 {
				break
			}

			var stocks []models.VirtualProductStock
			// 使用 FOR UPDATE 行锁防止并发超售
			query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("virtual_inventory_id = ? AND status = ?", binding.VirtualInventoryID, models.VirtualStockStatusAvailable)

			// 根据配置决定发货顺序
			deliveryOrder := s.cfg.Order.VirtualDeliveryOrder
			if deliveryOrder == "newest" {
				query = query.Order("created_at DESC")
			} else if deliveryOrder == "oldest" {
				query = query.Order("created_at ASC")
			} else {
				query = query.Order("RANDOM()")
			}

			err := query.Limit(remainingQuantity).Find(&stocks).Error

			if err != nil {
				continue
			}

			if len(stocks) == 0 {
				continue
			}

			// 标记为已预留
			var ids []uint
			for i := range stocks {
				ids = append(ids, stocks[i].ID)
				stocks[i].MarkAsReserved(orderNo)
			}

			// 批量更新状态（在事务内）
			if err := tx.Model(&models.VirtualProductStock{}).
				Where("id IN ?", ids).
				Updates(map[string]interface{}{
					"status":   models.VirtualStockStatusReserved,
					"order_no": orderNo,
				}).Error; err != nil {
				continue
			}

			allocatedStocks = append(allocatedStocks, stocks...)
			remainingQuantity -= len(stocks)
		}

		if len(allocatedStocks) < quantity {
			// 分配数量不足，事务回滚
			return fmt.Errorf("insufficient stock, need %d but only %d available", quantity, len(allocatedStocks))
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return allocatedStocks, nil
}

// AllocateStockFromInventory 从指定虚拟库存池直接分配库存（管理员创建订单时使用）
func (s *VirtualInventoryService) AllocateStockFromInventory(virtualInventoryID uint, quantity int, orderNo string) ([]models.VirtualProductStock, error) {
	var allocatedStocks []models.VirtualProductStock

	err := s.db.Transaction(func(tx *gorm.DB) error {
		var stocks []models.VirtualProductStock
		query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("virtual_inventory_id = ? AND status = ?", virtualInventoryID, models.VirtualStockStatusAvailable)

		// 根据配置决定发货顺序
		deliveryOrder := s.cfg.Order.VirtualDeliveryOrder
		if deliveryOrder == "newest" {
			query = query.Order("created_at DESC")
		} else if deliveryOrder == "oldest" {
			query = query.Order("created_at ASC")
		} else {
			query = query.Order("RANDOM()")
		}

		if err := query.Limit(quantity).Find(&stocks).Error; err != nil {
			return err
		}

		if len(stocks) < quantity {
			return fmt.Errorf("insufficient stock in virtual inventory %d: need %d, available %d", virtualInventoryID, quantity, len(stocks))
		}

		// 标记为已预留
		var ids []uint
		for i := range stocks {
			ids = append(ids, stocks[i].ID)
			stocks[i].MarkAsReserved(orderNo)
		}

		if err := tx.Model(&models.VirtualProductStock{}).
			Where("id IN ?", ids).
			Updates(map[string]interface{}{
				"status":   models.VirtualStockStatusReserved,
				"order_no": orderNo,
			}).Error; err != nil {
			return err
		}

		allocatedStocks = stocks
		return nil
	})

	if err != nil {
		return nil, err
	}

	return allocatedStocks, nil
}

// GetAvailableCountForProductByAttributes 根据规格属性获取商品可用库存数量
func (s *VirtualInventoryService) GetAvailableCountForProductByAttributes(productID uint, attributes map[string]string) (int64, error) {
	if len(attributes) == 0 {
		// 无属性，返回总库存
		return s.GetAvailableCountForProduct(productID)
	}

	// 规范化属性
	normalizedAttrs := models.NormalizeAttributes(attributes)
	attrsHash := models.GenerateAttributesHash(normalizedAttrs)

	// 获取该商品的所有虚拟库存绑定
	var bindings []models.ProductVirtualInventoryBinding
	if err := s.db.Where("product_id = ?", productID).Find(&bindings).Error; err != nil {
		return 0, err
	}

	// 先尝试精确匹配
	for _, binding := range bindings {
		if binding.AttributesHash == attrsHash {
			var count int64
			if err := s.db.Model(&models.VirtualProductStock{}).
				Where("virtual_inventory_id = ? AND status = ?", binding.VirtualInventoryID, models.VirtualStockStatusAvailable).
				Count(&count).Error; err != nil {
				return 0, err
			}
			return count, nil
		}
	}

	// 精确匹配失败，尝试部分匹配（用于混合模式：用户只选了部分属性）
	var totalCount int64
	inventoryMap := make(map[uint]bool) // 去重
	for _, binding := range bindings {
		if len(binding.Attributes) == 0 {
			continue
		}

		// 检查绑定是��包含用户选择的所有属性
		isMatch := true
		for userKey, userValue := range normalizedAttrs {
			if bindingValue, exists := binding.Attributes[userKey]; !exists || bindingValue != userValue {
				isMatch = false
				break
			}
		}

		if isMatch {
			if _, exists := inventoryMap[binding.VirtualInventoryID]; !exists {
				inventoryMap[binding.VirtualInventoryID] = true
				var count int64
				if err := s.db.Model(&models.VirtualProductStock{}).
					Where("virtual_inventory_id = ? AND status = ?", binding.VirtualInventoryID, models.VirtualStockStatusAvailable).
					Count(&count).Error; err != nil {
					continue
				}
				totalCount += count
			}
		}
	}

	return totalCount, nil
}

// DeliverStock 发货（将预留转为已售）
func (s *VirtualInventoryService) DeliverStock(orderID uint, orderNo string, deliveredBy *uint) error {
	now := models.NowFunc()

	updates := map[string]interface{}{
		"status":       models.VirtualStockStatusSold,
		"order_id":     orderID,
		"delivered_at": now,
	}

	if deliveredBy != nil {
		updates["delivered_by"] = *deliveredBy
	}

	return s.db.Model(&models.VirtualProductStock{}).
		Where("order_no = ? AND status = ?", orderNo, models.VirtualStockStatusReserved).
		Updates(updates).Error
}

// CanAutoDeliver 检查订单的所有预留虚拟库存是否都属于自动发货商品
// 如果存在任何非自动发货的预留库存，返回 false（不允许部分自动发货）
func (s *VirtualInventoryService) CanAutoDeliver(orderNo string) (bool, error) {
	// 统计该订单所有预留的虚拟库存数量
	var totalReserved int64
	if err := s.db.Model(&models.VirtualProductStock{}).
		Where("order_no = ? AND status = ?", orderNo, models.VirtualStockStatusReserved).
		Count(&totalReserved).Error; err != nil {
		return false, err
	}

	if totalReserved == 0 {
		return false, nil
	}

	// 统计属于自动发货商品的预留库存数量
	subQuery := s.db.Table("product_virtual_inventory_bindings pvib").
		Select("pvib.virtual_inventory_id").
		Joins("JOIN products p ON p.id = pvib.product_id").
		Where("p.auto_delivery = ? AND p.deleted_at IS NULL", true)

	var autoDeliveryReserved int64
	if err := s.db.Model(&models.VirtualProductStock{}).
		Where("order_no = ? AND status = ? AND virtual_inventory_id IN (?)",
			orderNo, models.VirtualStockStatusReserved, subQuery).
		Count(&autoDeliveryReserved).Error; err != nil {
		return false, err
	}

	// 只有当所有预留库存都属于自动发货商品时，才允许自动发货
	return autoDeliveryReserved == totalReserved, nil
}

// DeliverAutoDeliveryStock 仅发货启用自动发货的商品库存
func (s *VirtualInventoryService) DeliverAutoDeliveryStock(orderID uint, orderNo string, deliveredBy *uint) error {
	now := models.NowFunc()

	updates := map[string]interface{}{
		"status":       models.VirtualStockStatusSold,
		"order_id":     orderID,
		"delivered_at": now,
	}

	if deliveredBy != nil {
		updates["delivered_by"] = *deliveredBy
	}

	// 子查询：找出所有绑定到 auto_delivery=true 商品的虚拟库存ID
	subQuery := s.db.Table("product_virtual_inventory_bindings pvib").
		Select("pvib.virtual_inventory_id").
		Joins("JOIN products p ON p.id = pvib.product_id").
		Where("p.auto_delivery = ? AND p.deleted_at IS NULL", true)

	return s.db.Model(&models.VirtualProductStock{}).
		Where("order_no = ? AND status = ? AND virtual_inventory_id IN (?)",
			orderNo, models.VirtualStockStatusReserved, subQuery).
		Updates(updates).Error
}

// HasPendingVirtualStock 检查订单是否还有待发货的虚拟库存
func (s *VirtualInventoryService) HasPendingVirtualStock(orderNo string) (bool, error) {
	var count int64
	err := s.db.Model(&models.VirtualProductStock{}).
		Where("order_no = ? AND status = ?", orderNo, models.VirtualStockStatusReserved).
		Count(&count).Error
	return count > 0, err
}

// ReleaseStock 释放预留库存（取消订单时）
func (s *VirtualInventoryService) ReleaseStock(orderNo string) error {
	return s.db.Model(&models.VirtualProductStock{}).
		Where("order_no = ? AND status = ?", orderNo, models.VirtualStockStatusReserved).
		Updates(map[string]interface{}{
			"status":   models.VirtualStockStatusAvailable,
			"order_no": "",
		}).Error
}

// ManualReserveStock 手动预留单个库存项
func (s *VirtualInventoryService) ManualReserveStock(stockID uint, remark string) error {
	var stock models.VirtualProductStock
	if err := s.db.First(&stock, stockID).Error; err != nil {
		return errors.New("stock item not found")
	}

	if stock.Status != models.VirtualStockStatusAvailable {
		return errors.New("stock item is not available")
	}

	updates := map[string]interface{}{
		"status":   models.VirtualStockStatusReserved,
		"order_no": "MANUAL-RESERVE",
	}
	if remark != "" {
		updates["remark"] = remark
	}

	return s.db.Model(&stock).Updates(updates).Error
}

// ManualReleaseStock 手动释放单个库存项
func (s *VirtualInventoryService) ManualReleaseStock(stockID uint) error {
	var stock models.VirtualProductStock
	if err := s.db.First(&stock, stockID).Error; err != nil {
		return errors.New("stock item not found")
	}

	if stock.Status != models.VirtualStockStatusReserved {
		return errors.New("stock item is not reserved")
	}

	return s.db.Model(&stock).Updates(map[string]interface{}{
		"status":   models.VirtualStockStatusAvailable,
		"order_no": "",
	}).Error
}

// GetStockByOrderID 获取订单的虚拟产品库存
func (s *VirtualInventoryService) GetStockByOrderID(orderID uint) ([]models.VirtualProductStock, error) {
	var stocks []models.VirtualProductStock
	err := s.db.Where("order_id = ?", orderID).Find(&stocks).Error
	return stocks, err
}

// GetStockByOrderNo 根据订单号获取库存
func (s *VirtualInventoryService) GetStockByOrderNo(orderNo string) ([]models.VirtualProductStock, error) {
	var stocks []models.VirtualProductStock
	err := s.db.Where("order_no = ?", orderNo).Find(&stocks).Error
	return stocks, err
}

// GetAvailableCountForProduct 获取商品可用库存数量（通过绑定关系）
func (s *VirtualInventoryService) GetAvailableCountForProduct(productID uint) (int64, error) {
	bindings, err := s.GetProductBindings(productID)
	if err != nil {
		return 0, err
	}

	var totalAvailable int64
	for _, binding := range bindings {
		var count int64
		if err := s.db.Model(&models.VirtualProductStock{}).
			Where("virtual_inventory_id = ? AND status = ?", binding.VirtualInventoryID, models.VirtualStockStatusAvailable).
			Count(&count).Error; err != nil {
			continue
		}
		totalAvailable += count
	}

	return totalAvailable, nil
}

// GetStockStatsForProduct 获取商品的虚拟库存统计（通过绑定关系）
func (s *VirtualInventoryService) GetStockStatsForProduct(productID uint) (map[string]int64, error) {
	bindings, err := s.GetProductBindings(productID)
	if err != nil {
		return nil, err
	}

	stats := map[string]int64{
		"total":     0,
		"available": 0,
		"reserved":  0,
		"sold":      0,
	}

	for _, binding := range bindings {
		invStats, err := s.GetStockStats(binding.VirtualInventoryID)
		if err != nil {
			continue
		}
		stats["total"] += invStats["total"]
		stats["available"] += invStats["available"]
		stats["reserved"] += invStats["reserved"]
		stats["sold"] += invStats["sold"]
	}

	return stats, nil
}

// ListStocksForProduct 获取商品的虚拟库存列表（通过绑定关系）
func (s *VirtualInventoryService) ListStocksForProduct(productID uint, status string, page, limit int) ([]models.VirtualProductStock, int64, error) {
	bindings, err := s.GetProductBindings(productID)
	if err != nil {
		return nil, 0, err
	}

	if len(bindings) == 0 {
		return []models.VirtualProductStock{}, 0, nil
	}

	// 收集所有虚拟库存 ID
	var inventoryIDs []uint
	for _, binding := range bindings {
		inventoryIDs = append(inventoryIDs, binding.VirtualInventoryID)
	}

	var stocks []models.VirtualProductStock
	var total int64

	query := s.db.Model(&models.VirtualProductStock{}).Where("virtual_inventory_id IN ?", inventoryIDs)

	if status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&stocks).Error; err != nil {
		return nil, 0, err
	}

	return stocks, total, nil
}

// ImportStockForProduct 为商品导入虚拟库存（使用第一个绑定的库存）
func (s *VirtualInventoryService) ImportStockForProduct(productID uint, content string, importedBy string) (int, error) {
	bindings, err := s.GetProductBindings(productID)
	if err != nil {
		return 0, err
	}

	if len(bindings) == 0 {
		return 0, errors.New("no virtual inventory bound to this product")
	}

	// 使用第一个绑定的虚拟库存
	return s.ImportFromText(bindings[0].VirtualInventoryID, content, importedBy)
}

// ImportStockFromFileForProduct 为商品从文件导入虚拟库存
func (s *VirtualInventoryService) ImportStockFromFileForProduct(productID uint, filePath string, importedBy string) (int, error) {
	bindings, err := s.GetProductBindings(productID)
	if err != nil {
		return 0, err
	}

	if len(bindings) == 0 {
		return 0, errors.New("no virtual inventory bound to this product")
	}

	// 使用第一个绑定的虚拟库存
	return s.ImportFromExcel(bindings[0].VirtualInventoryID, filePath, importedBy)
}

// GetFirstBindingForProduct 获取商品的第一个虚拟库存绑定
func (s *VirtualInventoryService) GetFirstBindingForProduct(productID uint) (*models.ProductVirtualInventoryBinding, error) {
	var binding models.ProductVirtualInventoryBinding
	if err := s.db.Where("product_id = ?", productID).First(&binding).Error; err != nil {
		return nil, err
	}
	return &binding, nil
}

// SelectRandomVirtualInventory 盲盒模式：根据权重随机选择虚拟库存
// 返回：选中的虚拟库存绑定 + 完整的属性组合
func (s *VirtualInventoryService) SelectRandomVirtualInventory(productID uint, quantity int) (*models.ProductVirtualInventoryBinding, map[string]string, error) {
	// 1. 获取所有绑定
	bindings, err := s.GetProductBindings(productID)
	if err != nil {
		return nil, nil, err
	}

	if len(bindings) == 0 {
		return nil, nil, errors.New("no virtual inventory bound to this product")
	}

	// 2. 筛选出有足够库存的绑定
	var availableBindings []models.BindingWithVirtualInventoryInfo
	for _, binding := range bindings {
		if binding.VirtualInventory != nil && binding.VirtualInventory.Available >= int64(quantity) {
			availableBindings = append(availableBindings, binding)
		}
	}

	if len(availableBindings) == 0 {
		return nil, nil, errors.New("not enough virtual inventory available for allocation")
	}

	// 辅助函数：从绑定中提取完整属性
	getFullAttributes := func(binding models.BindingWithVirtualInventoryInfo) map[string]string {
		attrs := make(map[string]string)
		for k, v := range binding.Attributes {
			attrs[k] = v
		}
		return attrs
	}

	// 3. 根据权重随机选择
	totalWeight := 0
	for _, binding := range availableBindings {
		totalWeight += binding.Priority
	}

	var selectedBinding *models.BindingWithVirtualInventoryInfo

	if totalWeight == 0 {
		// 如果所有权重都是0，则随机选择
		randomIndex := rand.Intn(len(availableBindings))
		selectedBinding = &availableBindings[randomIndex]
	} else {
		// 使用加权随机算法
		randomValue := rand.Intn(totalWeight)
		currentWeight := 0

		for i := range availableBindings {
			currentWeight += availableBindings[i].Priority
			if randomValue < currentWeight {
				selectedBinding = &availableBindings[i]
				break
			}
		}

		// 兜底：返回最后一个
		if selectedBinding == nil {
			selectedBinding = &availableBindings[len(availableBindings)-1]
		}
	}

	// 返回绑定的完整信息
	fullAttrs := getFullAttributes(*selectedBinding)

	// 转换为 ProductVirtualInventoryBinding
	result := &models.ProductVirtualInventoryBinding{
		ID:                 selectedBinding.ID,
		ProductID:          selectedBinding.ProductID,
		VirtualInventoryID: selectedBinding.VirtualInventoryID,
		Attributes:         selectedBinding.Attributes,
		AttributesHash:     selectedBinding.AttributesHash,
		IsRandom:           selectedBinding.IsRandom,
		Priority:           selectedBinding.Priority,
	}

	return result, fullAttrs, nil
}

// FindVirtualInventoryWithPartialMatch 混合模式：部分属性匹配 + 盲盒随机
// 用于处理：用户选择部分属性，其他属性由系统随机分配
// 返回：选中的虚拟库存绑定 + 完整的属性组合（包括随机分配的）
func (s *VirtualInventoryService) FindVirtualInventoryWithPartialMatch(productID uint, userAttributes map[string]string, quantity int) (*models.ProductVirtualInventoryBinding, map[string]string, error) {
	// 1. 获取所有绑定
	bindings, err := s.GetProductBindings(productID)
	if err != nil {
		return nil, nil, err
	}

	if len(bindings) == 0 {
		return nil, nil, errors.New("no virtual inventory bound to this product")
	}

	// 2. 筛选出包含用户选择属性的绑定（部分匹配）
	var matchedBindings []models.BindingWithVirtualInventoryInfo
	for _, binding := range bindings {
		if binding.VirtualInventory == nil || binding.VirtualInventory.Available < int64(quantity) {
			continue
		}

		// 解析绑定的 attributes
		bindingAttrs := make(map[string]string)
		for k, v := range binding.Attributes {
			bindingAttrs[k] = v
		}

		// 检查是否包含用户选择的所有属性（部分匹配）
		isMatch := true
		for userKey, userValue := range userAttributes {
			if bindingValue, exists := bindingAttrs[userKey]; !exists || bindingValue != userValue {
				isMatch = false
				break
			}
		}

		if isMatch {
			matchedBindings = append(matchedBindings, binding)
		}
	}

	if len(matchedBindings) == 0 {
		return nil, nil, errors.New("no matching virtual inventory configuration found")
	}

	// 辅助函数：从绑定中提取完整属性
	getFullAttributes := func(binding models.BindingWithVirtualInventoryInfo) map[string]string {
		attrs := make(map[string]string)
		for k, v := range binding.Attributes {
			attrs[k] = v
		}
		return attrs
	}

	// 3. 如果只有一个匹配，直接返回
	if len(matchedBindings) == 1 {
		selectedBinding := matchedBindings[0]
		fullAttrs := getFullAttributes(selectedBinding)

		result := &models.ProductVirtualInventoryBinding{
			ID:                 selectedBinding.ID,
			ProductID:          selectedBinding.ProductID,
			VirtualInventoryID: selectedBinding.VirtualInventoryID,
			Attributes:         selectedBinding.Attributes,
			AttributesHash:     selectedBinding.AttributesHash,
			IsRandom:           selectedBinding.IsRandom,
			Priority:           selectedBinding.Priority,
		}

		return result, fullAttrs, nil
	}

	// 4. 如果有多个匹配，根据权重随机选择（盲盒属性）
	totalWeight := 0
	for _, binding := range matchedBindings {
		totalWeight += binding.Priority
	}

	var selectedBinding *models.BindingWithVirtualInventoryInfo

	if totalWeight == 0 {
		// 权重都是0，随机选择
		randomIndex := rand.Intn(len(matchedBindings))
		selectedBinding = &matchedBindings[randomIndex]
	} else {
		// 加权随机
		randomValue := rand.Intn(totalWeight)
		currentWeight := 0

		for i := range matchedBindings {
			currentWeight += matchedBindings[i].Priority
			if randomValue < currentWeight {
				selectedBinding = &matchedBindings[i]
				break
			}
		}

		// 兜底
		if selectedBinding == nil {
			selectedBinding = &matchedBindings[len(matchedBindings)-1]
		}
	}

	fullAttrs := getFullAttributes(*selectedBinding)

	result := &models.ProductVirtualInventoryBinding{
		ID:                 selectedBinding.ID,
		ProductID:          selectedBinding.ProductID,
		VirtualInventoryID: selectedBinding.VirtualInventoryID,
		Attributes:         selectedBinding.Attributes,
		AttributesHash:     selectedBinding.AttributesHash,
		IsRandom:           selectedBinding.IsRandom,
		Priority:           selectedBinding.Priority,
	}

	return result, fullAttrs, nil
}
