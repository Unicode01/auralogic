package admin

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"auralogic/internal/database"
	"auralogic/internal/models"
	"auralogic/internal/pkg/logger"
	"auralogic/internal/pkg/response"
	"auralogic/internal/repository"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type productImportResult struct {
	Message      string   `json:"message"`
	ConflictMode string   `json:"conflict_mode"`
	TotalRows    int      `json:"total_rows"`
	CreatedCount int      `json:"created_count"`
	UpdatedCount int      `json:"updated_count"`
	SkippedCount int      `json:"skipped_count"`
	ErrorCount   int      `json:"error_count"`
	Errors       []string `json:"errors,omitempty"`
}

type productImportRow struct {
	SourceRow           int
	SKU                 string
	Name                string
	ProductCode         string
	ProductType         models.ProductType
	Description         string
	ShortDescription    string
	Category            string
	Tags                []string
	PriceMinor          int64
	OriginalPriceMinor  int64
	Stock               int
	MaxPurchaseLimit    int
	Status              models.ProductStatus
	SortOrder           int
	IsFeatured          bool
	IsRecommended       bool
	InventoryMode       string
	AutoDelivery        bool
	ViewCount           int
	SaleCount           int
	HasViewCount        bool
	HasSaleCount        bool
	Remark              string
	Images              []models.ProductImage
	Attributes          []models.ProductAttribute
	PhysicalBindings    []productBindingImportItem
	VirtualBindings     []productVirtualBindingImportItem
	HasPhysicalBindings bool
	HasVirtualBindings  bool
}

type productBindingImportItem struct {
	InventoryID   uint              `json:"inventory_id,omitempty"`
	InventorySKU  string            `json:"inventory_sku,omitempty"`
	InventoryName string            `json:"inventory_name,omitempty"`
	Attributes    map[string]string `json:"attributes,omitempty"`
	IsRandom      bool              `json:"is_random"`
	Priority      int               `json:"priority"`
	Notes         string            `json:"notes,omitempty"`
}

type productVirtualBindingImportItem struct {
	VirtualInventoryID   uint              `json:"virtual_inventory_id,omitempty"`
	VirtualInventorySKU  string            `json:"virtual_inventory_sku,omitempty"`
	VirtualInventoryName string            `json:"virtual_inventory_name,omitempty"`
	Attributes           map[string]string `json:"attributes,omitempty"`
	IsRandom             bool              `json:"is_random"`
	Priority             int               `json:"priority"`
	Notes                string            `json:"notes,omitempty"`
}

type resolvedProductBindingImportItem struct {
	InventoryID uint
	Attributes  map[string]string
	IsRandom    bool
	Priority    int
	Notes       string
}

type resolvedProductVirtualBindingImportItem struct {
	VirtualInventoryID uint
	Attributes         map[string]string
	IsRandom           bool
	Priority           int
	Notes              string
}

const productImportMaxErrors = 100

func buildProductCSVHeaders() []string {
	return []string{
		"ID",
		"SKU",
		"Name",
		"Product Code",
		"Product Type",
		"Category",
		"Tags JSON",
		"Price Minor",
		"Original Price Minor",
		"Stock",
		"Max Purchase Limit",
		"Status",
		"Sort Order",
		"Is Featured",
		"Is Recommended",
		"Inventory Mode",
		"Auto Delivery",
		"View Count",
		"Sale Count",
		"Short Description",
		"Description",
		"Remark",
		"Images JSON",
		"Attributes JSON",
		"Inventory Bindings JSON",
		"Virtual Inventory Bindings JSON",
		"Created At",
		"Updated At",
	}
}

func normalizeProductImportHeader(value string) string {
	normalized := strings.TrimSpace(strings.TrimPrefix(value, "\uFEFF"))
	normalized = strings.ToLower(normalized)
	return strings.NewReplacer(" ", "", "_", "", "-", "", ".", "", "/", "", "(", "", ")", "").Replace(normalized)
}

func canonicalProductImportHeader(value string) string {
	switch normalizeProductImportHeader(value) {
	case "id", "编号":
		return "id"
	case "sku":
		return "sku"
	case "name", "名称":
		return "name"
	case "productcode", "productserialcode", "产品码":
		return "product_code"
	case "producttype", "商品类型":
		return "product_type"
	case "category", "分类":
		return "category"
	case "tagsjson", "tags", "标签", "标签json":
		return "tags_json"
	case "priceminor", "price", "价格分", "价格":
		return "price_minor"
	case "originalpriceminor", "originalprice", "原价分", "原价":
		return "original_price_minor"
	case "stock", "库存":
		return "stock"
	case "maxpurchaselimit", "purchase_limit", "限购数量", "最大购买数量":
		return "max_purchase_limit"
	case "status", "状态":
		return "status"
	case "sortorder", "排序":
		return "sort_order"
	case "isfeatured", "精选":
		return "is_featured"
	case "isrecommended", "推荐":
		return "is_recommended"
	case "inventorymode", "库存模式":
		return "inventory_mode"
	case "autodelivery", "自动发货":
		return "auto_delivery"
	case "viewcount", "浏览量":
		return "view_count"
	case "salecount", "销量":
		return "sale_count"
	case "shortdescription", "shortdesc", "简短描述":
		return "short_description"
	case "description", "描述":
		return "description"
	case "remark", "备注":
		return "remark"
	case "imagesjson", "images", "图片json":
		return "images_json"
	case "attributesjson", "attributes", "specsjson", "规格json":
		return "attributes_json"
	case "inventorybindingsjson", "physicalbindingsjson", "bindingsjson", "库存绑定json":
		return "inventory_bindings_json"
	case "virtualinventorybindingsjson", "virtualbindingsjson", "虚拟库存绑定json":
		return "virtual_inventory_bindings_json"
	case "createdat", "创建时间":
		return "created_at"
	case "updatedat", "更新时间":
		return "updated_at"
	default:
		return ""
	}
}

func buildProductImportHeaderMap(header []string) (map[string]int, error) {
	headerMap := make(map[string]int, len(header))
	for idx, raw := range header {
		canonical := canonicalProductImportHeader(raw)
		if canonical == "" {
			continue
		}
		if _, exists := headerMap[canonical]; exists {
			return nil, fmt.Errorf("duplicate header: %s", strings.TrimSpace(raw))
		}
		headerMap[canonical] = idx
	}
	if _, ok := headerMap["sku"]; !ok {
		return nil, fmt.Errorf("missing required header: SKU")
	}
	return headerMap, nil
}

func productImportHeaderExists(headerMap map[string]int, key string) bool {
	_, ok := headerMap[key]
	return ok
}

func productImportCell(record []string, headerMap map[string]int, key string) string {
	idx, ok := headerMap[key]
	if !ok || idx < 0 || idx >= len(record) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(record[idx], "\uFEFF"))
}

func parseProductImportBool(value string) (bool, error) {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" {
		return false, nil
	}
	switch trimmed {
	case "1", "true", "t", "yes", "y", "on", "是", "启用", "开启":
		return true, nil
	case "0", "false", "f", "no", "n", "off", "否", "停用", "关闭":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean: %s", value)
	}
}

func parseProductImportInt64(value, field string) (int64, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", field)
	}
	if parsed < 0 {
		return 0, fmt.Errorf("%s cannot be negative", field)
	}
	return parsed, nil
}

func parseProductImportInt(value, field string) (int, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", field)
	}
	if parsed < 0 {
		return 0, fmt.Errorf("%s cannot be negative", field)
	}
	return parsed, nil
}

func parseProductImportStatus(value string) (models.ProductStatus, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return models.ProductStatusDraft, nil
	}
	switch normalized {
	case string(models.ProductStatusDraft), "草稿":
		return models.ProductStatusDraft, nil
	case string(models.ProductStatusActive), "上架":
		return models.ProductStatusActive, nil
	case string(models.ProductStatusInactive), "下架":
		return models.ProductStatusInactive, nil
	case string(models.ProductStatusOutOfStock), "缺货":
		return models.ProductStatusOutOfStock, nil
	default:
		return "", fmt.Errorf("invalid status: %s", value)
	}
}

func parseProductImportType(value string) (models.ProductType, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return models.ProductTypePhysical, nil
	}
	switch normalized {
	case string(models.ProductTypePhysical), "实体", "实物":
		return models.ProductTypePhysical, nil
	case string(models.ProductTypeVirtual), "虚拟":
		return models.ProductTypeVirtual, nil
	default:
		return "", fmt.Errorf("invalid product_type: %s", value)
	}
}

func parseProductImportInventoryMode(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return string(models.InventoryModeFixed), nil
	}
	switch normalized {
	case string(models.InventoryModeFixed), "固定":
		return string(models.InventoryModeFixed), nil
	case string(models.InventoryModeRandom), "盲盒", "随机":
		return string(models.InventoryModeRandom), nil
	default:
		return "", fmt.Errorf("invalid inventory_mode: %s", value)
	}
}

func normalizeProductImportStringSlice(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		candidate := strings.TrimSpace(value)
		if candidate != "" {
			out = append(out, candidate)
		}
	}
	return out
}

func parseProductImportStringSlice(value string) ([]string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.EqualFold(trimmed, "null") {
		return []string{}, nil
	}

	var out []string
	if strings.HasPrefix(trimmed, "[") {
		if err := json.Unmarshal([]byte(trimmed), &out); err != nil {
			return nil, fmt.Errorf("invalid tags_json")
		}
	} else {
		parts := strings.FieldsFunc(trimmed, func(r rune) bool {
			return r == ',' || r == '，' || r == ';' || r == '；'
		})
		out = make([]string, 0, len(parts))
		for _, part := range parts {
			candidate := strings.TrimSpace(part)
			if candidate != "" {
				out = append(out, candidate)
			}
		}
	}

	return normalizeProductImportStringSlice(out), nil
}

func parseProductImportImages(value string) ([]models.ProductImage, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.EqualFold(trimmed, "null") {
		return []models.ProductImage{}, nil
	}
	var out []models.ProductImage
	if err := json.Unmarshal([]byte(trimmed), &out); err != nil {
		return nil, fmt.Errorf("invalid images_json")
	}
	return out, nil
}

func parseProductImportAttributes(value string) ([]models.ProductAttribute, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.EqualFold(trimmed, "null") {
		return []models.ProductAttribute{}, nil
	}
	var out []models.ProductAttribute
	if err := json.Unmarshal([]byte(trimmed), &out); err != nil {
		return nil, fmt.Errorf("invalid attributes_json")
	}
	for idx := range out {
		out[idx].Name = strings.TrimSpace(out[idx].Name)
		out[idx].Values = normalizeProductImportStringSlice(out[idx].Values)
		if out[idx].Mode == "" {
			out[idx].Mode = models.AttributeModeUserSelect
		}
	}
	return out, nil
}

func parseProductImportPhysicalBindings(value string) ([]productBindingImportItem, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.EqualFold(trimmed, "null") {
		return []productBindingImportItem{}, nil
	}
	var out []productBindingImportItem
	if err := json.Unmarshal([]byte(trimmed), &out); err != nil {
		return nil, fmt.Errorf("invalid inventory_bindings_json")
	}
	return out, nil
}

func parseProductImportVirtualBindings(value string) ([]productVirtualBindingImportItem, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.EqualFold(trimmed, "null") {
		return []productVirtualBindingImportItem{}, nil
	}
	var out []productVirtualBindingImportItem
	if err := json.Unmarshal([]byte(trimmed), &out); err != nil {
		return nil, fmt.Errorf("invalid virtual_inventory_bindings_json")
	}
	return out, nil
}

func normalizeProductImportAttributesMap(attrs map[string]string) map[string]string {
	if len(attrs) == 0 {
		return map[string]string{}
	}
	normalized := make(map[string]string)
	for key, value := range attrs {
		trimmedKey := strings.TrimSpace(key)
		trimmedValue := strings.TrimSpace(value)
		if trimmedKey == "" || trimmedValue == "" {
			continue
		}
		normalized[trimmedKey] = trimmedValue
	}
	return models.NormalizeAttributes(normalized)
}

func buildProductImportRow(record []string, headerMap map[string]int) (*productImportRow, bool, error) {
	blank := true
	for _, value := range record {
		if strings.TrimSpace(strings.TrimPrefix(value, "\uFEFF")) != "" {
			blank = false
			break
		}
	}
	if blank {
		return nil, true, nil
	}

	row := &productImportRow{
		SourceRow:           0,
		SKU:                 strings.TrimSpace(productImportCell(record, headerMap, "sku")),
		Name:                strings.TrimSpace(productImportCell(record, headerMap, "name")),
		ProductCode:         strings.TrimSpace(productImportCell(record, headerMap, "product_code")),
		Description:         productImportCell(record, headerMap, "description"),
		ShortDescription:    productImportCell(record, headerMap, "short_description"),
		Category:            strings.TrimSpace(productImportCell(record, headerMap, "category")),
		Remark:              productImportCell(record, headerMap, "remark"),
		HasViewCount:        productImportHeaderExists(headerMap, "view_count"),
		HasSaleCount:        productImportHeaderExists(headerMap, "sale_count"),
		HasPhysicalBindings: productImportHeaderExists(headerMap, "inventory_bindings_json"),
		HasVirtualBindings:  productImportHeaderExists(headerMap, "virtual_inventory_bindings_json"),
	}

	if row.SKU == "" {
		return nil, false, fmt.Errorf("sku is required")
	}
	if row.Name == "" {
		return nil, false, fmt.Errorf("name is required")
	}

	productType, err := parseProductImportType(productImportCell(record, headerMap, "product_type"))
	if err != nil {
		return nil, false, err
	}
	row.ProductType = productType

	tags, err := parseProductImportStringSlice(productImportCell(record, headerMap, "tags_json"))
	if err != nil {
		return nil, false, err
	}
	row.Tags = tags

	priceMinor, err := parseProductImportInt64(productImportCell(record, headerMap, "price_minor"), "price_minor")
	if err != nil {
		return nil, false, err
	}
	row.PriceMinor = priceMinor

	originalPriceMinor, err := parseProductImportInt64(productImportCell(record, headerMap, "original_price_minor"), "original_price_minor")
	if err != nil {
		return nil, false, err
	}
	row.OriginalPriceMinor = originalPriceMinor

	stock, err := parseProductImportInt(productImportCell(record, headerMap, "stock"), "stock")
	if err != nil {
		return nil, false, err
	}
	row.Stock = stock

	maxPurchaseLimit, err := parseProductImportInt(productImportCell(record, headerMap, "max_purchase_limit"), "max_purchase_limit")
	if err != nil {
		return nil, false, err
	}
	row.MaxPurchaseLimit = maxPurchaseLimit

	status, err := parseProductImportStatus(productImportCell(record, headerMap, "status"))
	if err != nil {
		return nil, false, err
	}
	row.Status = status

	sortOrder, err := parseProductImportInt(productImportCell(record, headerMap, "sort_order"), "sort_order")
	if err != nil {
		return nil, false, err
	}
	row.SortOrder = sortOrder

	isFeatured, err := parseProductImportBool(productImportCell(record, headerMap, "is_featured"))
	if err != nil {
		return nil, false, err
	}
	row.IsFeatured = isFeatured

	isRecommended, err := parseProductImportBool(productImportCell(record, headerMap, "is_recommended"))
	if err != nil {
		return nil, false, err
	}
	row.IsRecommended = isRecommended

	inventoryMode, err := parseProductImportInventoryMode(productImportCell(record, headerMap, "inventory_mode"))
	if err != nil {
		return nil, false, err
	}
	row.InventoryMode = inventoryMode

	autoDelivery, err := parseProductImportBool(productImportCell(record, headerMap, "auto_delivery"))
	if err != nil {
		return nil, false, err
	}
	row.AutoDelivery = autoDelivery

	viewCount, err := parseProductImportInt(productImportCell(record, headerMap, "view_count"), "view_count")
	if err != nil {
		return nil, false, err
	}
	row.ViewCount = viewCount

	saleCount, err := parseProductImportInt(productImportCell(record, headerMap, "sale_count"), "sale_count")
	if err != nil {
		return nil, false, err
	}
	row.SaleCount = saleCount

	images, err := parseProductImportImages(productImportCell(record, headerMap, "images_json"))
	if err != nil {
		return nil, false, err
	}
	row.Images = images

	attributes, err := parseProductImportAttributes(productImportCell(record, headerMap, "attributes_json"))
	if err != nil {
		return nil, false, err
	}
	row.Attributes = attributes

	physicalBindings, err := parseProductImportPhysicalBindings(productImportCell(record, headerMap, "inventory_bindings_json"))
	if err != nil {
		return nil, false, err
	}
	row.PhysicalBindings = physicalBindings

	virtualBindings, err := parseProductImportVirtualBindings(productImportCell(record, headerMap, "virtual_inventory_bindings_json"))
	if err != nil {
		return nil, false, err
	}
	row.VirtualBindings = virtualBindings

	return row, false, nil
}

func appendProductImportError(errors []string, message string) []string {
	if strings.TrimSpace(message) == "" || len(errors) >= productImportMaxErrors {
		return errors
	}
	return append(errors, message)
}

func loadImportedInventoryLookup(db *gorm.DB, rows []*productImportRow) (map[string]uint, map[uint]struct{}, error) {
	skus := make([]string, 0)
	ids := make([]uint, 0)
	seenSKUs := make(map[string]struct{})
	seenIDs := make(map[uint]struct{})

	for _, row := range rows {
		for _, binding := range row.PhysicalBindings {
			sku := strings.TrimSpace(binding.InventorySKU)
			if sku != "" {
				if _, exists := seenSKUs[sku]; !exists {
					seenSKUs[sku] = struct{}{}
					skus = append(skus, sku)
				}
			}
			if binding.InventoryID > 0 {
				if _, exists := seenIDs[binding.InventoryID]; !exists {
					seenIDs[binding.InventoryID] = struct{}{}
					ids = append(ids, binding.InventoryID)
				}
			}
		}
	}

	skuToID := make(map[string]uint, len(skus))
	idSet := make(map[uint]struct{}, len(ids))

	if len(skus) > 0 {
		var inventories []models.Inventory
		if err := db.Select("id, sku").Where("sku IN ?", skus).Find(&inventories).Error; err != nil {
			return nil, nil, err
		}
		for _, inventory := range inventories {
			sku := strings.TrimSpace(inventory.SKU)
			if sku != "" {
				if existingID, exists := skuToID[sku]; exists && existingID != inventory.ID {
					return nil, nil, fmt.Errorf("duplicate inventory SKU for import mapping: %s", sku)
				}
				skuToID[sku] = inventory.ID
			}
		}
	}

	if len(ids) > 0 {
		var inventoryIDs []uint
		if err := db.Model(&models.Inventory{}).Where("id IN ?", ids).Pluck("id", &inventoryIDs).Error; err != nil {
			return nil, nil, err
		}
		for _, id := range inventoryIDs {
			idSet[id] = struct{}{}
		}
	}

	return skuToID, idSet, nil
}

func loadImportedVirtualInventoryLookup(db *gorm.DB, rows []*productImportRow) (map[string]uint, map[uint]struct{}, error) {
	skus := make([]string, 0)
	ids := make([]uint, 0)
	seenSKUs := make(map[string]struct{})
	seenIDs := make(map[uint]struct{})

	for _, row := range rows {
		for _, binding := range row.VirtualBindings {
			sku := strings.TrimSpace(binding.VirtualInventorySKU)
			if sku != "" {
				if _, exists := seenSKUs[sku]; !exists {
					seenSKUs[sku] = struct{}{}
					skus = append(skus, sku)
				}
			}
			if binding.VirtualInventoryID > 0 {
				if _, exists := seenIDs[binding.VirtualInventoryID]; !exists {
					seenIDs[binding.VirtualInventoryID] = struct{}{}
					ids = append(ids, binding.VirtualInventoryID)
				}
			}
		}
	}

	skuToID := make(map[string]uint, len(skus))
	idSet := make(map[uint]struct{}, len(ids))

	if len(skus) > 0 {
		var inventories []models.VirtualInventory
		if err := db.Select("id, sku").Where("sku IN ?", skus).Find(&inventories).Error; err != nil {
			return nil, nil, err
		}
		for _, inventory := range inventories {
			sku := strings.TrimSpace(inventory.SKU)
			if sku != "" {
				if existingID, exists := skuToID[sku]; exists && existingID != inventory.ID {
					return nil, nil, fmt.Errorf("duplicate virtual inventory SKU for import mapping: %s", sku)
				}
				skuToID[sku] = inventory.ID
			}
		}
	}

	if len(ids) > 0 {
		var inventoryIDs []uint
		if err := db.Model(&models.VirtualInventory{}).Where("id IN ?", ids).Pluck("id", &inventoryIDs).Error; err != nil {
			return nil, nil, err
		}
		for _, id := range inventoryIDs {
			idSet[id] = struct{}{}
		}
	}

	return skuToID, idSet, nil
}

func resolveImportedPhysicalBindings(bindings []productBindingImportItem, skuToID map[string]uint, idSet map[uint]struct{}) ([]resolvedProductBindingImportItem, error) {
	resolved := make([]resolvedProductBindingImportItem, 0, len(bindings))
	seenHashes := make(map[string]struct{}, len(bindings))

	for idx, binding := range bindings {
		inventoryID := uint(0)
		if sku := strings.TrimSpace(binding.InventorySKU); sku != "" {
			inventoryID = skuToID[sku]
			if inventoryID == 0 {
				return nil, fmt.Errorf("inventory_bindings_json[%d]: inventory SKU not found: %s", idx, sku)
			}
		} else if binding.InventoryID > 0 {
			if _, exists := idSet[binding.InventoryID]; !exists {
				return nil, fmt.Errorf("inventory_bindings_json[%d]: inventory ID not found: %d", idx, binding.InventoryID)
			}
			inventoryID = binding.InventoryID
		} else {
			return nil, fmt.Errorf("inventory_bindings_json[%d]: inventory_sku or inventory_id is required", idx)
		}

		attributes := normalizeProductImportAttributesMap(binding.Attributes)
		hash := models.GenerateAttributesHash(attributes)
		if _, exists := seenHashes[hash]; exists {
			return nil, fmt.Errorf("inventory_bindings_json[%d]: duplicate attribute combination", idx)
		}
		seenHashes[hash] = struct{}{}

		priority := binding.Priority
		if priority <= 0 {
			priority = 1
		}

		resolved = append(resolved, resolvedProductBindingImportItem{
			InventoryID: inventoryID,
			Attributes:  attributes,
			IsRandom:    binding.IsRandom,
			Priority:    priority,
			Notes:       strings.TrimSpace(binding.Notes),
		})
	}

	return resolved, nil
}

func resolveImportedVirtualBindings(bindings []productVirtualBindingImportItem, skuToID map[string]uint, idSet map[uint]struct{}) ([]resolvedProductVirtualBindingImportItem, error) {
	resolved := make([]resolvedProductVirtualBindingImportItem, 0, len(bindings))
	seenHashes := make(map[string]struct{}, len(bindings))

	for idx, binding := range bindings {
		virtualInventoryID := uint(0)
		if sku := strings.TrimSpace(binding.VirtualInventorySKU); sku != "" {
			virtualInventoryID = skuToID[sku]
			if virtualInventoryID == 0 {
				return nil, fmt.Errorf("virtual_inventory_bindings_json[%d]: virtual inventory SKU not found: %s", idx, sku)
			}
		} else if binding.VirtualInventoryID > 0 {
			if _, exists := idSet[binding.VirtualInventoryID]; !exists {
				return nil, fmt.Errorf("virtual_inventory_bindings_json[%d]: virtual inventory ID not found: %d", idx, binding.VirtualInventoryID)
			}
			virtualInventoryID = binding.VirtualInventoryID
		} else {
			return nil, fmt.Errorf("virtual_inventory_bindings_json[%d]: virtual_inventory_sku or virtual_inventory_id is required", idx)
		}

		attributes := normalizeProductImportAttributesMap(binding.Attributes)
		hash := models.GenerateAttributesHash(attributes)
		if _, exists := seenHashes[hash]; exists {
			return nil, fmt.Errorf("virtual_inventory_bindings_json[%d]: duplicate attribute combination", idx)
		}
		seenHashes[hash] = struct{}{}

		priority := binding.Priority
		if priority <= 0 {
			priority = 1
		}

		resolved = append(resolved, resolvedProductVirtualBindingImportItem{
			VirtualInventoryID: virtualInventoryID,
			Attributes:         attributes,
			IsRandom:           binding.IsRandom,
			Priority:           priority,
			Notes:              strings.TrimSpace(binding.Notes),
		})
	}

	return resolved, nil
}

func mustMarshalJSON(value interface{}) string {
	if value == nil {
		return "{}"
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

func applyImportedPhysicalBindings(tx *gorm.DB, productID uint, bindings []resolvedProductBindingImportItem) error {
	if err := tx.Where("product_id = ?", productID).Delete(&models.ProductInventoryBinding{}).Error; err != nil {
		return err
	}

	for _, binding := range bindings {
		attributes := normalizeProductImportAttributesMap(binding.Attributes)
		if err := tx.Create(&models.ProductInventoryBinding{
			ProductID:      productID,
			InventoryID:    binding.InventoryID,
			Attributes:     models.JSON(mustMarshalJSON(attributes)),
			AttributesHash: models.GenerateAttributesHash(attributes),
			IsRandom:       binding.IsRandom,
			Priority:       binding.Priority,
			Notes:          binding.Notes,
		}).Error; err != nil {
			return err
		}
	}

	return nil
}

func applyImportedVirtualBindings(tx *gorm.DB, productID uint, bindings []resolvedProductVirtualBindingImportItem) error {
	if err := tx.Where("product_id = ?", productID).Delete(&models.ProductVirtualInventoryBinding{}).Error; err != nil {
		return err
	}

	for _, binding := range bindings {
		attributes := normalizeProductImportAttributesMap(binding.Attributes)
		if err := tx.Create(&models.ProductVirtualInventoryBinding{
			ProductID:          productID,
			VirtualInventoryID: binding.VirtualInventoryID,
			Attributes:         models.JSONMap(attributes),
			AttributesHash:     models.GenerateAttributesHash(attributes),
			IsRandom:           binding.IsRandom,
			Priority:           binding.Priority,
			Notes:              binding.Notes,
		}).Error; err != nil {
			return err
		}
	}

	return nil
}

func buildProductExportPhysicalBindings(bindings []models.ProductInventoryBinding) []productBindingImportItem {
	out := make([]productBindingImportItem, 0, len(bindings))
	for _, binding := range bindings {
		item := productBindingImportItem{
			InventoryID: binding.InventoryID,
			IsRandom:    binding.IsRandom,
			Priority:    binding.Priority,
			Notes:       strings.TrimSpace(binding.Notes),
			Attributes:  map[string]string{},
		}
		if binding.Inventory != nil {
			item.InventorySKU = strings.TrimSpace(binding.Inventory.SKU)
			item.InventoryName = strings.TrimSpace(binding.Inventory.Name)
		}
		if len(binding.Attributes) > 0 {
			var attrs map[string]string
			if err := json.Unmarshal([]byte(binding.Attributes), &attrs); err == nil {
				item.Attributes = normalizeProductImportAttributesMap(attrs)
			}
		}
		out = append(out, item)
	}
	return out
}

func buildProductExportVirtualBindings(bindings []models.ProductVirtualInventoryBinding) []productVirtualBindingImportItem {
	out := make([]productVirtualBindingImportItem, 0, len(bindings))
	for _, binding := range bindings {
		item := productVirtualBindingImportItem{
			VirtualInventoryID: binding.VirtualInventoryID,
			IsRandom:           binding.IsRandom,
			Priority:           binding.Priority,
			Notes:              strings.TrimSpace(binding.Notes),
			Attributes:         normalizeProductImportAttributesMap(map[string]string(binding.Attributes)),
		}
		if binding.VirtualInventory != nil {
			item.VirtualInventorySKU = strings.TrimSpace(binding.VirtualInventory.SKU)
			item.VirtualInventoryName = strings.TrimSpace(binding.VirtualInventory.Name)
		}
		out = append(out, item)
	}
	return out
}

func (h *ProductHandler) importProductRowWithTx(tx *gorm.DB, row *productImportRow, conflictMode string, existing *models.Product, physicalBindings []resolvedProductBindingImportItem, virtualBindings []resolvedProductVirtualBindingImportItem) (string, uint, error) {
	txProductRepo := repository.NewProductRepository(tx)
	txInventoryRepo := repository.NewInventoryRepository(tx)
	txProductService := service.NewProductService(txProductRepo, txInventoryRepo)

	productModel := &models.Product{
		SKU:              row.SKU,
		Name:             row.Name,
		ProductCode:      row.ProductCode,
		ProductType:      row.ProductType,
		Description:      row.Description,
		ShortDescription: row.ShortDescription,
		Category:         row.Category,
		Tags:             row.Tags,
		Price:            row.PriceMinor,
		OriginalPrice:    row.OriginalPriceMinor,
		Stock:            row.Stock,
		MaxPurchaseLimit: row.MaxPurchaseLimit,
		Images:           row.Images,
		Attributes:       row.Attributes,
		Status:           row.Status,
		SortOrder:        row.SortOrder,
		IsFeatured:       row.IsFeatured,
		IsRecommended:    row.IsRecommended,
		Remark:           row.Remark,
		AutoDelivery:     row.AutoDelivery,
	}

	var (
		productID uint
		action    string
	)

	switch {
	case existing != nil && conflictMode == "skip":
		return "skipped", existing.ID, nil
	case existing != nil:
		if err := txProductService.UpdateProduct(existing.ID, productModel); err != nil {
			return "", 0, err
		}
		productID = existing.ID
		action = "updated"
	case existing == nil && conflictMode == "update":
		return "skipped", 0, nil
	default:
		if err := txProductService.CreateProduct(productModel); err != nil {
			return "", 0, err
		}
		productID = productModel.ID
		action = "created"
	}

	if row.InventoryMode != "" {
		if err := txProductService.UpdateInventoryMode(productID, row.InventoryMode); err != nil {
			return "", 0, err
		}
	}

	counterUpdates := make(map[string]interface{}, 2)
	if row.HasViewCount {
		counterUpdates["view_count"] = row.ViewCount
	}
	if row.HasSaleCount {
		counterUpdates["sale_count"] = row.SaleCount
	}
	if len(counterUpdates) > 0 {
		if err := tx.Model(&models.Product{}).Where("id = ?", productID).Updates(counterUpdates).Error; err != nil {
			return "", 0, err
		}
	}

	if row.HasPhysicalBindings {
		if err := applyImportedPhysicalBindings(tx, productID, physicalBindings); err != nil {
			return "", 0, err
		}
	}
	if row.HasVirtualBindings {
		if err := applyImportedVirtualBindings(tx, productID, virtualBindings); err != nil {
			return "", 0, err
		}
	}

	return action, productID, nil
}

// ImportProducts 导入商品表格文件
func (h *ProductHandler) ImportProducts(c *gin.Context) {
	conflictMode, err := parsePromoCodeImportConflictMode(c.DefaultPostForm("conflict_mode", c.Query("conflict_mode")))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "Please select an import file to upload")
		return
	}

	fileFormat, tableRows, err := readAdminTabularRows(file)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unsupported format") {
			response.BadRequest(c, "Only .xlsx Excel format is supported")
			return
		}
		response.BadRequest(c, "Failed to parse import file")
		return
	}
	if len(tableRows) == 0 {
		response.BadRequest(c, "Import file is empty")
		return
	}

	header := tableRows[0]
	headerMap, err := buildProductImportHeaderMap(header)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	rows := make([]*productImportRow, 0)
	result := productImportResult{
		ConflictMode: conflictMode,
		Errors:       make([]string, 0),
	}

	for idx, record := range tableRows[1:] {
		rowIndex := idx + 2
		row, blank, buildErr := buildProductImportRow(record, headerMap)
		if blank {
			continue
		}

		result.TotalRows++
		if result.TotalRows > adminCSVExportMaxRows {
			response.BadRequest(c, fmt.Sprintf("Too many rows to import (max %d). Please split the file.", adminCSVExportMaxRows))
			return
		}

		if buildErr != nil {
			result.ErrorCount++
			result.Errors = appendProductImportError(result.Errors, fmt.Sprintf("Row %d: %v", rowIndex, buildErr))
			continue
		}

		row.SourceRow = rowIndex
		rows = append(rows, row)
	}

	if result.TotalRows == 0 {
		response.BadRequest(c, "No data rows found in import file")
		return
	}

	db := database.GetDB()
	if db == nil {
		response.InternalError(c, "Database unavailable")
		return
	}

	productSKUs := make([]string, 0, len(rows))
	seenProductSKUs := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		if _, exists := seenProductSKUs[row.SKU]; exists {
			continue
		}
		seenProductSKUs[row.SKU] = struct{}{}
		productSKUs = append(productSKUs, row.SKU)
	}

	existingProductsBySKU := make(map[string]*models.Product, len(productSKUs))
	if len(productSKUs) > 0 {
		var products []models.Product
		if err := db.Where("sku IN ?", productSKUs).Find(&products).Error; err != nil {
			response.InternalError(c, "Failed to load existing products")
			return
		}
		for idx := range products {
			existingProductsBySKU[products[idx].SKU] = &products[idx]
		}
	}

	inventorySKUToID, inventoryIDSet, err := loadImportedInventoryLookup(db, rows)
	if err != nil {
		response.InternalError(c, "Failed to load inventory mapping")
		return
	}
	virtualInventorySKUToID, virtualInventoryIDSet, err := loadImportedVirtualInventoryLookup(db, rows)
	if err != nil {
		response.InternalError(c, "Failed to load virtual inventory mapping")
		return
	}

	for _, row := range rows {
		existing := existingProductsBySKU[row.SKU]

		physicalBindings, resolveErr := resolveImportedPhysicalBindings(row.PhysicalBindings, inventorySKUToID, inventoryIDSet)
		if resolveErr != nil {
			result.ErrorCount++
			result.Errors = appendProductImportError(result.Errors, fmt.Sprintf("Row %d: %v", row.SourceRow, resolveErr))
			continue
		}

		virtualBindings, resolveErr := resolveImportedVirtualBindings(row.VirtualBindings, virtualInventorySKUToID, virtualInventoryIDSet)
		if resolveErr != nil {
			result.ErrorCount++
			result.Errors = appendProductImportError(result.Errors, fmt.Sprintf("Row %d: %v", row.SourceRow, resolveErr))
			continue
		}

		var (
			action    string
			productID uint
		)
		txErr := db.Transaction(func(tx *gorm.DB) error {
			rowAction, rowProductID, err := h.importProductRowWithTx(tx, row, conflictMode, existing, physicalBindings, virtualBindings)
			if err != nil {
				return err
			}
			action = rowAction
			productID = rowProductID
			return nil
		})
		if txErr != nil {
			result.ErrorCount++
			result.Errors = appendProductImportError(result.Errors, fmt.Sprintf("Row %d: %v", row.SourceRow, txErr))
			continue
		}

		switch action {
		case "created":
			result.CreatedCount++
			existingProductsBySKU[row.SKU] = &models.Product{ID: productID, SKU: row.SKU}
		case "updated":
			result.UpdatedCount++
			existingProductsBySKU[row.SKU] = &models.Product{ID: productID, SKU: row.SKU}
		default:
			result.SkippedCount++
		}
	}

	result.Message = fmt.Sprintf(
		"Product import completed: created %d, updated %d, skipped %d, errors %d",
		result.CreatedCount,
		result.UpdatedCount,
		result.SkippedCount,
		result.ErrorCount,
	)

	logger.LogOperation(db, c, "import", "product", nil, map[string]interface{}{
		"filename":      strings.TrimSpace(file.Filename),
		"conflict_mode": conflictMode,
		"total_rows":    result.TotalRows,
		"created_count": result.CreatedCount,
		"updated_count": result.UpdatedCount,
		"skipped_count": result.SkippedCount,
		"error_count":   result.ErrorCount,
		"format":        fileFormat,
	})

	response.Success(c, result)
}

// DownloadProductImportTemplate 下载商品导入模板
func (h *ProductHandler) DownloadProductImportTemplate(c *gin.Context) {
	writeXLSXAttachment(c, "product_import_template.xlsx", "Products", buildProductCSVHeaders(), [][]string{})
}
