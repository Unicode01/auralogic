package service

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/models"

	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type VirtualInventoryService struct {
	db                    *gorm.DB
	cfg                   *config.Config
	scriptDeliveryService *ScriptDeliveryService
}

func NewVirtualInventoryService(db *gorm.DB) *VirtualInventoryService {
	return &VirtualInventoryService{
		db:                    db,
		cfg:                   config.GetConfig(),
		scriptDeliveryService: NewScriptDeliveryService(db),
	}
}

// createVirtualInventoryLog 记录虚拟库存变动日志
func (s *VirtualInventoryService) createVirtualInventoryLog(tx *gorm.DB, virtualInventoryID uint, logType string, quantity int, orderNo, batchNo, operator, reason string) {
	log := &models.InventoryLog{
		Source:      models.InventoryLogSourceVirtual,
		InventoryID: virtualInventoryID,
		Type:        logType,
		Quantity:    quantity,
		OrderNo:     orderNo,
		BatchNo:     batchNo,
		Operator:    operator,
		Reason:      reason,
	}
	tx.Create(log)
}

// scriptPendingItem 脚本类型虚拟库存的待发货条目
type scriptPendingItem struct {
	InventoryID uint
	Quantity    int
}

type inventorySoldCountRow struct {
	VirtualInventoryID uint
	Sold               int64
}

type inventoryStatusCountRow struct {
	VirtualInventoryID uint
	Status             string
	Count              int64
}

// getScriptPendingItems 获取订单中脚本类型虚拟库存的待发货条目
// 通过 VirtualInventoryBindings 记录的绑定关系，减去已有的 sold 记录数
func (s *VirtualInventoryService) getScriptPendingItems(orderNo string) ([]scriptPendingItem, error) {
	return s.getScriptPendingItemsWithDB(s.db, orderNo)
}

func (s *VirtualInventoryService) getScriptPendingItemsWithDB(db *gorm.DB, orderNo string) ([]scriptPendingItem, error) {
	var order models.Order
	if err := db.Select("id, virtual_inventory_bindings, items").
		Where("order_no = ?", orderNo).
		First(&order).Error; err != nil {
		return nil, err
	}

	if len(order.VirtualInventoryBindings) == 0 {
		return nil, nil
	}

	// 按 inventoryID 分组汇总 quantity
	inventoryQty := make(map[uint]int)
	for idx, invID := range order.VirtualInventoryBindings {
		if idx < len(order.Items) {
			inventoryQty[invID] += order.Items[idx].Quantity
		}
	}

	inventoryIDs := make([]uint, 0, len(inventoryQty))
	for invID := range inventoryQty {
		inventoryIDs = append(inventoryIDs, invID)
	}

	soldCountMap := make(map[uint]int64)
	if len(inventoryIDs) > 0 {
		var soldRows []inventorySoldCountRow
		if err := db.Model(&models.VirtualProductStock{}).
			Select("virtual_inventory_id, COUNT(*) as sold").
			Where("order_no = ? AND status = ? AND virtual_inventory_id IN ?",
				orderNo, models.VirtualStockStatusSold, inventoryIDs).
			Group("virtual_inventory_id").
			Scan(&soldRows).Error; err != nil {
			return nil, err
		}
		for _, row := range soldRows {
			soldCountMap[row.VirtualInventoryID] = row.Sold
		}
	}

	var result []scriptPendingItem
	for invID, totalQty := range inventoryQty {
		// 减去已有的 sold 记录数
		soldCount := soldCountMap[invID]
		pending := totalQty - int(soldCount)
		if pending > 0 {
			result = append(result, scriptPendingItem{
				InventoryID: invID,
				Quantity:    pending,
			})
		}
	}

	return result, nil
}

// getRandomOrderClause 根据数据库类型返回正确的随机排序SQL
func (s *VirtualInventoryService) getRandomOrderClause() string {
	if s.cfg.Database.Driver == "mysql" {
		return "RAND()"
	}
	return "RANDOM()"
}

// getStockStatsForInventories 批量获取库存统计，避免 N+1 查询
func (s *VirtualInventoryService) getStockStatsForInventories(inventoryIDs []uint) (map[uint]map[string]int64, error) {
	statsByInventory := make(map[uint]map[string]int64, len(inventoryIDs))
	if len(inventoryIDs) == 0 {
		return statsByInventory, nil
	}

	var inventories []models.VirtualInventory
	if err := s.db.Select("id, type, total_limit").
		Where("id IN ?", inventoryIDs).
		Find(&inventories).Error; err != nil {
		return nil, err
	}

	if len(inventories) == 0 {
		return statsByInventory, nil
	}

	var countRows []inventoryStatusCountRow
	if err := s.db.Model(&models.VirtualProductStock{}).
		Select("virtual_inventory_id, status, COUNT(*) as count").
		Where("virtual_inventory_id IN ?", inventoryIDs).
		Group("virtual_inventory_id, status").
		Scan(&countRows).Error; err != nil {
		return nil, err
	}

	statusCounts := make(map[uint]map[string]int64, len(inventoryIDs))
	for _, row := range countRows {
		if _, ok := statusCounts[row.VirtualInventoryID]; !ok {
			statusCounts[row.VirtualInventoryID] = make(map[string]int64)
		}
		statusCounts[row.VirtualInventoryID][row.Status] = row.Count
	}

	for _, inv := range inventories {
		counts := statusCounts[inv.ID]
		if counts == nil {
			counts = make(map[string]int64)
		}

		stats := map[string]int64{
			"total":     0,
			"available": 0,
			"reserved":  0,
			"sold":      0,
		}

		if inv.Type == models.VirtualInventoryTypeScript {
			sold := counts[string(models.VirtualStockStatusSold)]
			stats["sold"] = sold
			stats["reserved"] = 0
			if inv.TotalLimit > 0 {
				stats["total"] = inv.TotalLimit
				remaining := inv.TotalLimit - sold
				if remaining < 0 {
					remaining = 0
				}
				stats["available"] = remaining
			}
		} else {
			var total int64
			for _, count := range counts {
				total += count
			}
			stats["total"] = total
			stats["available"] = counts[string(models.VirtualStockStatusAvailable)]
			stats["reserved"] = counts[string(models.VirtualStockStatusReserved)]
			stats["sold"] = counts[string(models.VirtualStockStatusSold)]
		}

		statsByInventory[inv.ID] = stats
	}

	return statsByInventory, nil
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

	inventoryIDs := make([]uint, 0, len(inventories))
	for _, inv := range inventories {
		inventoryIDs = append(inventoryIDs, inv.ID)
	}
	statsByInventory, err := s.getStockStatsForInventories(inventoryIDs)
	if err != nil {
		return nil, 0, err
	}

	// 获取每个库存的统计信息
	var result []models.VirtualInventoryWithStats
	for _, inv := range inventories {
		stats := statsByInventory[inv.ID]
		if stats == nil {
			stats = map[string]int64{
				"total":     0,
				"available": 0,
				"reserved":  0,
				"sold":      0,
			}
		}
		result = append(result, models.VirtualInventoryWithStats{
			ID:           inv.ID,
			Name:         inv.Name,
			SKU:          inv.SKU,
			Type:         inv.Type,
			Script:       inv.Script,
			ScriptConfig: inv.ScriptConfig,
			Description:  inv.Description,
			TotalLimit:   inv.TotalLimit,
			IsActive:     inv.IsActive,
			Notes:        inv.Notes,
			Total:        stats["total"],
			Available:    stats["available"],
			Reserved:     stats["reserved"],
			Sold:         stats["sold"],
			CreatedAt:    inv.CreatedAt,
		})
	}

	return result, total, nil
}

// GetStockStats 获取库存统计
func (s *VirtualInventoryService) GetStockStats(virtualInventoryID uint) (map[string]int64, error) {
	statsByInventory, err := s.getStockStatsForInventories([]uint{virtualInventoryID})
	if err != nil {
		return nil, err
	}
	if stats, ok := statsByInventory[virtualInventoryID]; ok {
		return stats, nil
	}
	return map[string]int64{
		"total":     0,
		"available": 0,
		"reserved":  0,
		"sold":      0,
	}, nil
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

	s.createVirtualInventoryLog(s.db, virtualInventoryID, models.InventoryLogTypeImport, len(stocks), "", batchNo, importedBy, "Import from Excel")

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

	s.createVirtualInventoryLog(s.db, virtualInventoryID, models.InventoryLogTypeImport, len(stocks), "", batchNo, importedBy, "Import from text")

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

	s.createVirtualInventoryLog(s.db, virtualInventoryID, models.InventoryLogTypeImport, len(stocks), "", batchNo, importedBy, "Import from CSV")

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

	s.createVirtualInventoryLog(s.db, virtualInventoryID, models.InventoryLogTypeImport, 1, "", stock.BatchNo, importedBy, "Create stock manually")

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

	if err := s.db.Delete(&stock).Error; err != nil {
		return err
	}

	s.createVirtualInventoryLog(s.db, stock.VirtualInventoryID, models.InventoryLogTypeDelete, 1, stock.OrderNo, "", "admin", "Delete stock item")

	return nil
}

// DeleteBatch 删除整个批次
func (s *VirtualInventoryService) DeleteBatch(batchNo string) (int64, error) {
	// 查找该批次对应的虚拟库存ID
	var invID uint
	s.db.Model(&models.VirtualProductStock{}).
		Select("virtual_inventory_id").
		Where("batch_no = ? AND status = ?", batchNo, models.VirtualStockStatusAvailable).
		Limit(1).Pluck("virtual_inventory_id", &invID)

	result := s.db.Where("batch_no = ? AND status = ?", batchNo, models.VirtualStockStatusAvailable).
		Delete(&models.VirtualProductStock{})

	if result.Error != nil {
		return 0, result.Error
	}

	if result.RowsAffected > 0 && invID > 0 {
		s.createVirtualInventoryLog(s.db, invID, models.InventoryLogTypeDelete, int(result.RowsAffected), "", batchNo, "admin", "Delete batch")
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
		ID:           inventory.ID,
		Name:         inventory.Name,
		SKU:          inventory.SKU,
		Type:         inventory.Type,
		Script:       inventory.Script,
		ScriptConfig: inventory.ScriptConfig,
		Description:  inventory.Description,
		TotalLimit:   inventory.TotalLimit,
		IsActive:     inventory.IsActive,
		Notes:        inventory.Notes,
		Total:        stats["total"],
		Available:    stats["available"],
		Reserved:     stats["reserved"],
		Sold:         stats["sold"],
		CreatedAt:    inventory.CreatedAt,
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

	inventoryIDSet := make(map[uint]struct{}, len(bindings))
	inventoryIDs := make([]uint, 0, len(bindings))
	for _, binding := range bindings {
		if _, exists := inventoryIDSet[binding.VirtualInventoryID]; exists {
			continue
		}
		inventoryIDSet[binding.VirtualInventoryID] = struct{}{}
		inventoryIDs = append(inventoryIDs, binding.VirtualInventoryID)
	}
	statsByInventory, err := s.getStockStatsForInventories(inventoryIDs)
	if err != nil {
		return nil, err
	}

	var result []models.BindingWithVirtualInventoryInfo
	for _, binding := range bindings {
		stats := statsByInventory[binding.VirtualInventoryID]
		if stats == nil {
			stats = map[string]int64{
				"total":     0,
				"available": 0,
				"reserved":  0,
				"sold":      0,
			}
		}

		var invWithStats *models.VirtualInventoryWithStats
		if binding.VirtualInventory != nil {
			invWithStats = &models.VirtualInventoryWithStats{
				ID:           binding.VirtualInventory.ID,
				Name:         binding.VirtualInventory.Name,
				SKU:          binding.VirtualInventory.SKU,
				Type:         binding.VirtualInventory.Type,
				Script:       binding.VirtualInventory.Script,
				ScriptConfig: binding.VirtualInventory.ScriptConfig,
				Description:  binding.VirtualInventory.Description,
				TotalLimit:   binding.VirtualInventory.TotalLimit,
				IsActive:     binding.VirtualInventory.IsActive,
				Notes:        binding.VirtualInventory.Notes,
				Total:        stats["total"],
				Available:    stats["available"],
				Reserved:     stats["reserved"],
				Sold:         stats["sold"],
				CreatedAt:    binding.VirtualInventory.CreatedAt,
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
func (s *VirtualInventoryService) AllocateStockForProduct(productID uint, quantity int, orderNo string) ([]models.VirtualProductStock, *uint, error) {
	return s.AllocateStockForProductByAttributes(productID, quantity, orderNo, nil)
}

// AllocateStockForProductByAttributes 根据规格属性为商品分配虚拟库存
// 使用事务和行锁确保并发安全，防止超售
// 返回值: (分配的库存项, 脚本类型时选中的virtualInventoryID, error)
func (s *VirtualInventoryService) AllocateStockForProductByAttributes(productID uint, quantity int, orderNo string, attributes map[string]interface{}) ([]models.VirtualProductStock, *uint, error) {
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
			return nil, nil, err
		}
	}

	if len(bindings) == 0 {
		return nil, nil, errors.New("no virtual inventory bound to this product")
	}

	var allocatedStocks []models.VirtualProductStock
	var scriptInventoryID *uint

	// 使用事务确保并发安全
	err = s.db.Transaction(func(tx *gorm.DB) error {
		remainingQuantity := quantity

		// 按优先级从绑定的库存中分配
		for _, binding := range bindings {
			if remainingQuantity <= 0 {
				break
			}

			// 检查是否为脚本类型库存
			var inv models.VirtualInventory
			if err := tx.Select("id, type, total_limit").First(&inv, binding.VirtualInventoryID).Error; err != nil {
				continue
			}

			if inv.Type == models.VirtualInventoryTypeScript {
				// 脚本类型：检查发货次数限制
				if inv.TotalLimit > 0 {
					var sold int64
					tx.Model(&models.VirtualProductStock{}).
						Where("virtual_inventory_id = ? AND status = ?", binding.VirtualInventoryID, models.VirtualStockStatusSold).
						Count(&sold)
					if sold+int64(remainingQuantity) > inv.TotalLimit {
						continue // 超过限制，跳过
					}
				}
				// 不创建占位记录，仅记录选中的 virtualInventoryID
				id := binding.VirtualInventoryID
				scriptInventoryID = &id
				remainingQuantity = 0
				continue
			}

			// 静态类型：从已有库存中分配
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
				query = query.Order(s.getRandomOrderClause())
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

			s.createVirtualInventoryLog(tx, binding.VirtualInventoryID, models.InventoryLogTypeReserve, len(stocks), orderNo, "", "system", "Reserve stock for order")

			allocatedStocks = append(allocatedStocks, stocks...)
			remainingQuantity -= len(stocks)
		}

		if remainingQuantity > 0 && scriptInventoryID == nil {
			// 分配数量不足，事务回滚
			return fmt.Errorf("insufficient stock, need %d but only %d available", quantity, len(allocatedStocks))
		}

		return nil
	})

	if err != nil {
		return nil, nil, err
	}

	return allocatedStocks, scriptInventoryID, nil
}

// AllocateStockFromInventory 从指定虚拟库存池直接分配库存（管理员创建订单时使用）
// 返回值: (分配的库存项, 脚本类型时选中的virtualInventoryID, error)
func (s *VirtualInventoryService) AllocateStockFromInventory(virtualInventoryID uint, quantity int, orderNo string) ([]models.VirtualProductStock, *uint, error) {
	var allocatedStocks []models.VirtualProductStock

	err := s.db.Transaction(func(tx *gorm.DB) error {
		// 检查是否为脚本类型库存
		var inv models.VirtualInventory
		if err := tx.Select("id, type, total_limit").First(&inv, virtualInventoryID).Error; err != nil {
			return err
		}

		if inv.Type == models.VirtualInventoryTypeScript {
			// 脚本类型：检查发货次数限制
			if inv.TotalLimit > 0 {
				var sold int64
				tx.Model(&models.VirtualProductStock{}).
					Where("virtual_inventory_id = ? AND status = ?", virtualInventoryID, models.VirtualStockStatusSold).
					Count(&sold)
				if sold+int64(quantity) > inv.TotalLimit {
					return fmt.Errorf("script inventory %d delivery limit exceeded: limit %d, already sold %d, requested %d", virtualInventoryID, inv.TotalLimit, sold, quantity)
				}
			}
			// 不创建占位记录，直接返回
			return nil
		}

		// 静态类型：从已有库存中分配
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
			query = query.Order(s.getRandomOrderClause())
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

		s.createVirtualInventoryLog(tx, virtualInventoryID, models.InventoryLogTypeReserve, len(stocks), orderNo, "", "system", "Reserve stock for order (direct)")

		allocatedStocks = stocks
		return nil
	})

	if err != nil {
		return nil, nil, err
	}

	// 检查是否为脚本类型，返回 inventoryID
	var inv models.VirtualInventory
	if err := s.db.Select("id, type").First(&inv, virtualInventoryID).Error; err == nil && inv.Type == models.VirtualInventoryTypeScript {
		id := virtualInventoryID
		return allocatedStocks, &id, nil
	}

	return allocatedStocks, nil, nil
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
			// 检查是否为脚本类型库存
			var inv models.VirtualInventory
			if err := s.db.Select("type, total_limit").First(&inv, binding.VirtualInventoryID).Error; err == nil && inv.Type == models.VirtualInventoryTypeScript {
				if inv.TotalLimit > 0 {
					var sold int64
					s.db.Model(&models.VirtualProductStock{}).
						Where("virtual_inventory_id = ? AND status = ?", binding.VirtualInventoryID, models.VirtualStockStatusSold).
						Count(&sold)
					remaining := inv.TotalLimit - sold
					if remaining < 0 {
						remaining = 0
					}
					return remaining, nil
				}
				return 9999, nil
			}

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

		// 检查绑定是否包含用户选择的所有属性
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

				// 检查是否为脚本类型库存
				var inv models.VirtualInventory
				if err := s.db.Select("type, total_limit").First(&inv, binding.VirtualInventoryID).Error; err == nil && inv.Type == models.VirtualInventoryTypeScript {
					if inv.TotalLimit > 0 {
						var sold int64
						s.db.Model(&models.VirtualProductStock{}).
							Where("virtual_inventory_id = ? AND status = ?", binding.VirtualInventoryID, models.VirtualStockStatusSold).
							Count(&sold)
						remaining := inv.TotalLimit - sold
						if remaining > 0 {
							totalCount += remaining
						}
					} else {
						totalCount += 9999
					}
					continue
				}

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

// HasUnlimitedScriptInventoryForProduct returns whether the product is backed by any
// script virtual inventory without total_limit (i.e. unlimited stock).
func (s *VirtualInventoryService) HasUnlimitedScriptInventoryForProduct(productID uint) (bool, error) {
	bindings, err := s.GetProductBindings(productID)
	if err != nil {
		return false, err
	}
	for _, binding := range bindings {
		var inv models.VirtualInventory
		if err := s.db.Select("type, total_limit").First(&inv, binding.VirtualInventoryID).Error; err != nil {
			continue
		}
		if inv.Type == models.VirtualInventoryTypeScript && inv.TotalLimit <= 0 {
			return true, nil
		}
	}
	return false, nil
}

// HasUnlimitedScriptInventoryForProductByAttributes returns whether the matched
// virtual inventory set contains unlimited script inventory for the selected attributes.
func (s *VirtualInventoryService) HasUnlimitedScriptInventoryForProductByAttributes(productID uint, attributes map[string]string) (bool, error) {
	if len(attributes) == 0 {
		return s.HasUnlimitedScriptInventoryForProduct(productID)
	}

	normalizedAttrs := models.NormalizeAttributes(attributes)
	attrsHash := models.GenerateAttributesHash(normalizedAttrs)

	var bindings []models.ProductVirtualInventoryBinding
	if err := s.db.Where("product_id = ?", productID).Find(&bindings).Error; err != nil {
		return false, err
	}

	// Exact match first.
	for _, binding := range bindings {
		if binding.AttributesHash != attrsHash {
			continue
		}
		var inv models.VirtualInventory
		if err := s.db.Select("type, total_limit").First(&inv, binding.VirtualInventoryID).Error; err != nil {
			continue
		}
		if inv.Type == models.VirtualInventoryTypeScript && inv.TotalLimit <= 0 {
			return true, nil
		}
		return false, nil
	}

	// Partial match (same behavior as stock calculation path).
	inventoryMap := make(map[uint]bool)
	for _, binding := range bindings {
		if len(binding.Attributes) == 0 {
			continue
		}
		isMatch := true
		for userKey, userValue := range normalizedAttrs {
			if bindingValue, exists := binding.Attributes[userKey]; !exists || bindingValue != userValue {
				isMatch = false
				break
			}
		}
		if !isMatch || inventoryMap[binding.VirtualInventoryID] {
			continue
		}
		inventoryMap[binding.VirtualInventoryID] = true

		var inv models.VirtualInventory
		if err := s.db.Select("type, total_limit").First(&inv, binding.VirtualInventoryID).Error; err != nil {
			continue
		}
		if inv.Type == models.VirtualInventoryTypeScript && inv.TotalLimit <= 0 {
			return true, nil
		}
	}
	return false, nil
}

// DeliverStock 发货（将预留转为已售）
func (s *VirtualInventoryService) DeliverStock(orderID uint, orderNo string, deliveredBy *uint) error {
	// 先处理脚本类型的库存
	if err := s.executeScriptDelivery(orderID, orderNo, nil, deliveredBy); err != nil {
		return fmt.Errorf("script delivery failed: %w", err)
	}

	// 查询待发货的预留项（用于日志）
	var reservedStocks []models.VirtualProductStock
	s.db.Select("id, virtual_inventory_id").
		Where("order_no = ? AND status = ?", orderNo, models.VirtualStockStatusReserved).
		Find(&reservedStocks)

	now := models.NowFunc()

	updates := map[string]interface{}{
		"status":       models.VirtualStockStatusSold,
		"order_id":     orderID,
		"delivered_at": now,
	}

	if deliveredBy != nil {
		updates["delivered_by"] = *deliveredBy
	}

	if err := s.db.Model(&models.VirtualProductStock{}).
		Where("order_no = ? AND status = ?", orderNo, models.VirtualStockStatusReserved).
		Updates(updates).Error; err != nil {
		return err
	}

	// 按虚拟库存ID分组记录日志
	invCounts := make(map[uint]int)
	for _, stock := range reservedStocks {
		invCounts[stock.VirtualInventoryID]++
	}
	operator := "system"
	for invID, count := range invCounts {
		s.createVirtualInventoryLog(s.db, invID, models.InventoryLogTypeDeliver, count, orderNo, "", operator, "Deliver stock")
	}

	return nil
}

// CanAutoDeliver 检查订单的所有预留虚拟库存是否都属于自动发货商品
// 如果存在任何非自动发货的预留库存，返回 false（不允许部分自动发货）
func (s *VirtualInventoryService) CanAutoDeliver(orderNo string) (bool, error) {
	// 统计该订单所有预留的虚拟库存数量（旧流程）
	var totalReserved int64
	if err := s.db.Model(&models.VirtualProductStock{}).
		Where("order_no = ? AND status = ?", orderNo, models.VirtualStockStatusReserved).
		Count(&totalReserved).Error; err != nil {
		return false, err
	}

	// 统计新流程的脚本待发货数量
	pendingItems, err := s.getScriptPendingItems(orderNo)
	if err != nil {
		return false, err
	}
	var totalScriptPending int64
	for _, item := range pendingItems {
		totalScriptPending += int64(item.Quantity)
	}

	totalPending := totalReserved + totalScriptPending
	if totalPending == 0 {
		return false, nil
	}

	// 统计属于自动发货商品且库存已启用的预留库存数量（旧流程）
	subQuery := s.db.Table("product_virtual_inventory_bindings pvib").
		Select("pvib.virtual_inventory_id").
		Joins("JOIN products p ON p.id = pvib.product_id").
		Joins("JOIN virtual_inventories vi ON vi.id = pvib.virtual_inventory_id").
		Where("p.auto_delivery = ? AND p.deleted_at IS NULL AND vi.is_active = ?", true, true)

	var autoDeliveryReserved int64
	if totalReserved > 0 {
		if err := s.db.Model(&models.VirtualProductStock{}).
			Where("order_no = ? AND status = ? AND virtual_inventory_id IN (?)",
				orderNo, models.VirtualStockStatusReserved, subQuery).
			Count(&autoDeliveryReserved).Error; err != nil {
			return false, err
		}
	}

	// 统计新流程中属于自动发货商品且库存已启用的脚本待发货数量
	var autoDeliveryScriptPending int64
	if totalScriptPending > 0 {
		// 查询所有 auto_delivery 且 is_active 的库存ID
		var autoDeliveryInvIDs []uint
		if err := s.db.Table("product_virtual_inventory_bindings pvib").
			Select("DISTINCT pvib.virtual_inventory_id").
			Joins("JOIN products p ON p.id = pvib.product_id").
			Joins("JOIN virtual_inventories vi ON vi.id = pvib.virtual_inventory_id").
			Where("p.auto_delivery = ? AND p.deleted_at IS NULL AND vi.is_active = ?", true, true).
			Pluck("pvib.virtual_inventory_id", &autoDeliveryInvIDs).Error; err != nil {
			return false, err
		}
		autoInvSet := make(map[uint]bool)
		for _, id := range autoDeliveryInvIDs {
			autoInvSet[id] = true
		}
		for _, item := range pendingItems {
			if autoInvSet[item.InventoryID] {
				autoDeliveryScriptPending += int64(item.Quantity)
			}
		}
	}

	// 只有当所有待发货库存都属于自动发货商品时，才允许自动发货
	return (autoDeliveryReserved + autoDeliveryScriptPending) == totalPending, nil
}

// DeliverAutoDeliveryStock 仅发货启用自动发货的商品库存
func (s *VirtualInventoryService) DeliverAutoDeliveryStock(orderID uint, orderNo string, deliveredBy *uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		return s.deliverAutoDeliveryStockWithDB(tx, orderID, orderNo, deliveredBy)
	})
}

// DeliverAutoDeliveryStockWithTx 在外部事务中发货启用自动发货的商品库存
func (s *VirtualInventoryService) DeliverAutoDeliveryStockWithTx(tx *gorm.DB, orderID uint, orderNo string, deliveredBy *uint) error {
	if tx == nil {
		return errors.New("transaction is required")
	}
	return s.deliverAutoDeliveryStockWithDB(tx, orderID, orderNo, deliveredBy)
}

func (s *VirtualInventoryService) deliverAutoDeliveryStockWithDB(db *gorm.DB, orderID uint, orderNo string, deliveredBy *uint) error {
	// 构建自动发货库存ID过滤器（仅启用的库存）
	autoDeliveryFilter := db.Table("product_virtual_inventory_bindings pvib").
		Select("DISTINCT pvib.virtual_inventory_id AS virtual_inventory_id").
		Joins("JOIN products p ON p.id = pvib.product_id").
		Joins("JOIN virtual_inventories vi ON vi.id = pvib.virtual_inventory_id").
		Where("p.auto_delivery = ? AND p.deleted_at IS NULL AND vi.is_active = ?", true, true)

	// 只处理属于自动发货商品的脚本类型库存
	if err := s.executeScriptDeliveryWithDB(db, orderID, orderNo, autoDeliveryFilter, deliveredBy); err != nil {
		return fmt.Errorf("script delivery failed: %w", err)
	}

	// 子查询：找出所有绑定到 auto_delivery=true 且库存已启用的虚拟库存ID
	subQuery := db.Table("product_virtual_inventory_bindings pvib").
		Select("pvib.virtual_inventory_id").
		Joins("JOIN products p ON p.id = pvib.product_id").
		Joins("JOIN virtual_inventories vi ON vi.id = pvib.virtual_inventory_id").
		Where("p.auto_delivery = ? AND p.deleted_at IS NULL AND vi.is_active = ?", true, true)

	// 查询待发货的预留项（用于日志）
	var reservedStocks []models.VirtualProductStock
	db.Select("id, virtual_inventory_id").
		Where("order_no = ? AND status = ? AND virtual_inventory_id IN (?)",
			orderNo, models.VirtualStockStatusReserved, subQuery).
		Find(&reservedStocks)

	now := models.NowFunc()

	updates := map[string]interface{}{
		"status":       models.VirtualStockStatusSold,
		"order_id":     orderID,
		"delivered_at": now,
	}

	if deliveredBy != nil {
		updates["delivered_by"] = *deliveredBy
	}

	if err := db.Model(&models.VirtualProductStock{}).
		Where("order_no = ? AND status = ? AND virtual_inventory_id IN (?)",
			orderNo, models.VirtualStockStatusReserved, subQuery).
		Updates(updates).Error; err != nil {
		return err
	}

	// 按虚拟库存ID分组记录日志
	invCounts := make(map[uint]int)
	for _, stock := range reservedStocks {
		invCounts[stock.VirtualInventoryID]++
	}
	for invID, count := range invCounts {
		s.createVirtualInventoryLog(db, invID, models.InventoryLogTypeDeliver, count, orderNo, "", "system", "Auto deliver stock")
	}

	return nil
}

// HasPendingVirtualStock 检查订单是否还有待发货的虚拟库存
func (s *VirtualInventoryService) HasPendingVirtualStock(orderNo string) (bool, error) {
	// 检查旧流程的预留记录
	var count int64
	err := s.db.Model(&models.VirtualProductStock{}).
		Where("order_no = ? AND status = ?", orderNo, models.VirtualStockStatusReserved).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	if count > 0 {
		return true, nil
	}

	// 检查新流程的脚本待发货条目
	pendingItems, err := s.getScriptPendingItems(orderNo)
	if err != nil {
		return false, err
	}
	return len(pendingItems) > 0, nil
}

// executeScriptDelivery 执行脚本发货：查找该订单中属于脚本类型库存的预留项，执行脚本并填充内容
// inventoryIDFilter: 可选的库存ID过滤子查询，为nil时处理所有脚本类型库存
// deliveredBy: 发货操作人ID（可选）
func (s *VirtualInventoryService) executeScriptDelivery(orderID uint, orderNo string, inventoryIDFilter *gorm.DB, deliveredBy *uint) error {
	return s.executeScriptDeliveryWithDB(s.db, orderID, orderNo, inventoryIDFilter, deliveredBy)
}

func (s *VirtualInventoryService) executeScriptDeliveryWithDB(db *gorm.DB, orderID uint, orderNo string, inventoryIDFilter *gorm.DB, deliveredBy *uint) error {
	// === 旧流程：处理已有占位记录（兼容旧订单） ===
	scriptInvSubQuery := db.Table("virtual_inventories").
		Select("id").
		Where("type = ?", models.VirtualInventoryTypeScript)

	query := db.Where("order_no = ? AND status = ? AND virtual_inventory_id IN (?)",
		orderNo, models.VirtualStockStatusReserved, scriptInvSubQuery)

	if inventoryIDFilter != nil {
		query = query.Where("virtual_inventory_id IN (?)", inventoryIDFilter)
	}

	var reservedStocks []models.VirtualProductStock
	if err := query.Find(&reservedStocks).Error; err != nil {
		return err
	}

	// 获取订单信息
	var order models.Order
	if err := db.First(&order, orderID).Error; err != nil {
		return fmt.Errorf("order not found: %w", err)
	}

	// 处理旧流程的占位记录
	if len(reservedStocks) > 0 {
		grouped := make(map[uint][]models.VirtualProductStock)
		for _, stock := range reservedStocks {
			grouped[stock.VirtualInventoryID] = append(grouped[stock.VirtualInventoryID], stock)
		}

		for inventoryID, stocks := range grouped {
			var inventory models.VirtualInventory
			if err := db.First(&inventory, inventoryID).Error; err != nil {
				return fmt.Errorf("inventory %d not found: %w", inventoryID, err)
			}

			result, err := s.scriptDeliveryService.ExecuteDeliveryScript(&inventory, &order, len(stocks))
			if err != nil {
				return fmt.Errorf("script execution failed for inventory %d: %w", inventoryID, err)
			}

			if len(result.Items) < len(stocks) {
				return fmt.Errorf("script for inventory %d returned %d items, expected %d", inventoryID, len(result.Items), len(stocks))
			}

			for i, stock := range stocks {
				if i < len(result.Items) {
					updates := map[string]interface{}{
						"content": result.Items[i].Content,
					}
					if result.Items[i].Remark != "" {
						updates["remark"] = result.Items[i].Remark
					}
					if err := db.Model(&models.VirtualProductStock{}).
						Where("id = ?", stock.ID).
						Updates(updates).Error; err != nil {
						return fmt.Errorf("failed to update stock content: %w", err)
					}
				}
			}
		}
	}

	// === 新流程：处理 VirtualInventoryBindings（无占位记录） ===
	pendingItems, err := s.getScriptPendingItemsWithDB(db, orderNo)
	if err != nil || len(pendingItems) == 0 {
		return err
	}

	// 如果有过滤条件，获取允许的库存ID集合
	var allowedInvIDs map[uint]bool
	if inventoryIDFilter != nil {
		var ids []uint
		if err := db.Table("(?) AS inventory_filter", inventoryIDFilter).
			Select("DISTINCT inventory_filter.virtual_inventory_id").
			Pluck("inventory_filter.virtual_inventory_id", &ids).Error; err != nil {
			return fmt.Errorf("failed to resolve inventory filter: %w", err)
		}
		if len(ids) > 0 {
			allowedInvIDs = make(map[uint]bool, len(ids))
			for _, id := range ids {
				allowedInvIDs[id] = true
			}
		}
	}

	now := models.NowFunc()
	for _, item := range pendingItems {
		// 应用过滤
		if allowedInvIDs != nil && !allowedInvIDs[item.InventoryID] {
			continue
		}

		var inventory models.VirtualInventory
		if err := db.First(&inventory, item.InventoryID).Error; err != nil {
			return fmt.Errorf("inventory %d not found: %w", item.InventoryID, err)
		}

		result, err := s.scriptDeliveryService.ExecuteDeliveryScript(&inventory, &order, item.Quantity)
		if err != nil {
			return fmt.Errorf("script execution failed for inventory %d: %w", item.InventoryID, err)
		}

		if len(result.Items) < item.Quantity {
			return fmt.Errorf("script for inventory %d returned %d items, expected %d", item.InventoryID, len(result.Items), item.Quantity)
		}

		// 直接批量创建 sold 记录
		soldStocks := make([]models.VirtualProductStock, 0, item.Quantity)
		for i := 0; i < item.Quantity; i++ {
			stock := models.VirtualProductStock{
				VirtualInventoryID: item.InventoryID,
				Content:            result.Items[i].Content,
				Status:             models.VirtualStockStatusSold,
				OrderNo:            orderNo,
				OrderID:            &orderID,
				DeliveredAt:        &now,
				DeliveredBy:        deliveredBy,
				CreatedAt:          now,
			}
			if result.Items[i].Remark != "" {
				stock.Remark = result.Items[i].Remark
			}
			soldStocks = append(soldStocks, stock)
		}
		if len(soldStocks) > 0 {
			if err := db.CreateInBatches(&soldStocks, 200).Error; err != nil {
				return fmt.Errorf("failed to create sold stock: %w", err)
			}
		}
	}

	return nil
}

// ReleaseStock 释放预留库存（取消订单时）
// 脚本类型的库存项直接删除（无论content是否已填充），静态库存项恢复为可用
func (s *VirtualInventoryService) ReleaseStock(orderNo string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// 查询所有预留项（用于日志）
		var reservedStocks []models.VirtualProductStock
		tx.Select("id, virtual_inventory_id").
			Where("order_no = ? AND status = ?", orderNo, models.VirtualStockStatusReserved).
			Find(&reservedStocks)

		// 找出属于脚本类型库存的预留项并删除
		scriptInvSubQuery := tx.Table("virtual_inventories").
			Select("id").
			Where("type = ?", models.VirtualInventoryTypeScript)

		if err := tx.Where("order_no = ? AND status = ? AND virtual_inventory_id IN (?)",
			orderNo, models.VirtualStockStatusReserved, scriptInvSubQuery).
			Delete(&models.VirtualProductStock{}).Error; err != nil {
			return err
		}

		// 将静态库存项恢复为可用
		if err := tx.Model(&models.VirtualProductStock{}).
			Where("order_no = ? AND status = ?", orderNo, models.VirtualStockStatusReserved).
			Updates(map[string]interface{}{
				"status":   models.VirtualStockStatusAvailable,
				"order_no": "",
			}).Error; err != nil {
			return err
		}

		// 按虚拟库存ID分组记录日志
		invCounts := make(map[uint]int)
		for _, stock := range reservedStocks {
			invCounts[stock.VirtualInventoryID]++
		}
		for invID, count := range invCounts {
			s.createVirtualInventoryLog(tx, invID, models.InventoryLogTypeRelease, count, orderNo, "", "system", "Release stock on order cancellation")
		}

		return nil
	})
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

	if err := s.db.Model(&stock).Updates(updates).Error; err != nil {
		return err
	}

	s.createVirtualInventoryLog(s.db, stock.VirtualInventoryID, models.InventoryLogTypeReserve, 1, "MANUAL-RESERVE", "", "admin", "Manual reserve stock")

	return nil
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

	if err := s.db.Model(&stock).Updates(map[string]interface{}{
		"status":   models.VirtualStockStatusAvailable,
		"order_no": "",
	}).Error; err != nil {
		return err
	}

	s.createVirtualInventoryLog(s.db, stock.VirtualInventoryID, models.InventoryLogTypeRelease, 1, "", "", "admin", "Manual release stock")

	return nil
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
		// 检查是否为脚本类型库存（使用TotalLimit限制）
		var inv models.VirtualInventory
		if err := s.db.Select("type, total_limit").First(&inv, binding.VirtualInventoryID).Error; err == nil && inv.Type == models.VirtualInventoryTypeScript {
			if inv.TotalLimit > 0 {
				var sold int64
				s.db.Model(&models.VirtualProductStock{}).
					Where("virtual_inventory_id = ? AND status = ?", binding.VirtualInventoryID, models.VirtualStockStatusSold).
					Count(&sold)
				remaining := inv.TotalLimit - sold
				if remaining > 0 {
					totalAvailable += remaining
				}
			} else {
				totalAvailable += 9999
			}
			continue
		}

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
		if binding.VirtualInventory == nil {
			continue
		}
		stats["total"] += binding.VirtualInventory.Total
		stats["available"] += binding.VirtualInventory.Available
		stats["reserved"] += binding.VirtualInventory.Reserved
		stats["sold"] += binding.VirtualInventory.Sold
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
		if binding.VirtualInventory == nil {
			continue
		}
		// 脚本类型：有限制时检查剩余量，无限制时直接可用
		if binding.VirtualInventory.Type == models.VirtualInventoryTypeScript {
			if binding.VirtualInventory.TotalLimit > 0 && binding.VirtualInventory.Available < int64(quantity) {
				continue
			}
			availableBindings = append(availableBindings, binding)
		} else if binding.VirtualInventory.Available >= int64(quantity) {
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
		if binding.VirtualInventory == nil {
			continue
		}
		// 脚本类型：有限制时检查剩余量，无限制时直接可用；静态类型检查库存数量
		isScriptType := binding.VirtualInventory.Type == models.VirtualInventoryTypeScript
		if isScriptType {
			if binding.VirtualInventory.TotalLimit > 0 && binding.VirtualInventory.Available < int64(quantity) {
				continue
			}
		} else if binding.VirtualInventory.Available < int64(quantity) {
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

// TestDeliveryScript 测试发货脚本（使用模拟订单数据）
func (s *VirtualInventoryService) TestDeliveryScript(script string, config map[string]interface{}, quantity int) (*ScriptDeliveryResult, error) {
	if strings.TrimSpace(script) == "" {
		return nil, errors.New("script content is required")
	}
	if quantity <= 0 {
		quantity = 1
	}

	configJSON, _ := json.Marshal(config)

	inventory := &models.VirtualInventory{
		Type:         models.VirtualInventoryTypeScript,
		Script:       script,
		ScriptConfig: string(configJSON),
	}

	testOrder := &models.Order{
		OrderNo:     "TEST-ORDER-001",
		Status:      models.OrderStatusPendingPayment,
		TotalAmount: 9999,
		Currency:    "CNY",
	}

	return s.scriptDeliveryService.ExecuteDeliveryScript(inventory, testOrder, quantity)
}
