package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"auralogic/internal/config"
	"auralogic/internal/models"
	"auralogic/internal/pkg/password"
	"auralogic/internal/pkg/utils"
	"auralogic/internal/repository"
	"gorm.io/gorm"
)

type OrderService struct {
	OrderRepo         *repository.OrderRepository
	userRepo          *repository.UserRepository
	productRepo       *repository.ProductRepository
	inventoryRepo     *repository.InventoryRepository
	bindingService    *BindingService
	serialService     *SerialService
	virtualProductSvc *VirtualInventoryService
	promoCodeRepo     *repository.PromoCodeRepository
	cfg               *config.Config
	emailService      *EmailService
}

var (
	// Public, user-facing errors (safe to show to clients).
	ErrProductNotAvailable = errors.New("Product is not available")
)

func NewOrderService(
	orderRepo *repository.OrderRepository,
	userRepo *repository.UserRepository,
	productRepo *repository.ProductRepository,
	inventoryRepo *repository.InventoryRepository,
	bindingService *BindingService,
	serialService *SerialService,
	virtualProductSvc *VirtualInventoryService,
	promoCodeRepo *repository.PromoCodeRepository,
	cfg *config.Config,
	emailService *EmailService,
) *OrderService {
	return &OrderService{
		OrderRepo:         orderRepo,
		userRepo:          userRepo,
		productRepo:       productRepo,
		inventoryRepo:     inventoryRepo,
		bindingService:    bindingService,
		serialService:     serialService,
		virtualProductSvc: virtualProductSvc,
		promoCodeRepo:     promoCodeRepo,
		cfg:               cfg,
		emailService:      emailService,
	}
}

// CreateDraft CreateOrder草稿
func (s *OrderService) CreateDraft(items []models.OrderItem, externalUserID, externalOrderID, platform, userEmail, userName, remark string) (*models.Order, error) {
	// generateOrder号
	orderNo := utils.GenerateOrderNo(s.cfg.Order.NoPrefix)

	// generate表单Token
	formToken := uuid.New().String()
	formExpiresAt := models.NowFunc().Add(time.Duration(s.cfg.Form.ExpireHours) * time.Hour)

	// 计算订单总金额
	var totalAmount float64
	for _, item := range items {
		sku := strings.TrimSpace(item.SKU)
		if sku == "" {
			return nil, errors.New("Product SKU cannot be empty")
		}
		if item.Quantity <= 0 {
			return nil, errors.New("Quantity must be greater than 0")
		}
		product, err := s.productRepo.FindBySKU(sku)
		if err != nil {
			return nil, fmt.Errorf("Product %s does not exist", sku)
		}
		if product.Status != models.ProductStatusActive {
			return nil, ErrProductNotAvailable
		}
		totalAmount += product.Price * float64(item.Quantity)
	}

	// 获取货币单位
	currency := s.cfg.Order.Currency
	if currency == "" {
		currency = "CNY"
	}

	// CreateOrder
	order := &models.Order{
		OrderNo:                   orderNo,
		Items:                     items,
		Status:                    models.OrderStatusDraft,
		TotalAmount:               totalAmount,
		Currency:                  currency,
		FormToken:                 &formToken,
		FormExpiresAt:             &formExpiresAt,
		Source:                    "api",
		SourcePlatform:            platform,
		ExternalUserID:            externalUserID,
		ExternalUserName:          userName, // 保存第三方平台的User名
		ExternalOrderID:           externalOrderID,
		UserEmail:                 userEmail,
		EmailNotificationsEnabled: true,
		Remark:                    remark,
	}

	if err := s.OrderRepo.Create(order); err != nil {
		return nil, err
	}

	return order, nil
}

// AdminOrderRequest 管理员创建订单请求
type AdminOrderRequest struct {
	UserID           *uint
	Items            []AdminOrderItem
	ReceiverName     string
	PhoneCode        string
	ReceiverPhone    string
	ReceiverEmail    string
	ReceiverCountry  string
	ReceiverProvince string
	ReceiverCity     string
	ReceiverDistrict string
	ReceiverAddress  string
	ReceiverPostcode string
	Remark           string
	AdminRemark      string
	Status           string
	TotalAmount      *float64
	UserEmail        string
}

// AdminOrderItem 管理员订单商品项
type AdminOrderItem struct {
	SKU                string                 `json:"sku"`
	Name               string                 `json:"name"`
	Quantity           int                    `json:"quantity"`
	UnitPrice          float64                `json:"unit_price"`
	Attributes         map[string]interface{} `json:"attributes,omitempty"`
	ProductType        string                 `json:"product_type,omitempty"`
	VirtualInventoryID *uint                  `json:"virtual_inventory_id,omitempty"`
}

// CreateAdminOrder 管理员直接创建订单
func (s *OrderService) CreateAdminOrder(req AdminOrderRequest) (*models.Order, error) {
	if len(req.Items) == 0 {
		return nil, errors.New("Order items cannot be empty")
	}

	// 验证用户
	if req.UserID != nil {
		user, err := s.userRepo.FindByID(*req.UserID)
		if err != nil {
			return nil, errors.New("User not found")
		}
		if req.UserEmail == "" {
			req.UserEmail = user.Email
		}
	}

	// 构建订单商品并计算总金额
	var orderItems []models.OrderItem
	var totalAmount float64
	// 保存每个商品项指定的虚拟库存ID（管理员手动选择）
	virtualInventoryIDs := make(map[int]*uint)
	for idx, item := range req.Items {
		sku := strings.TrimSpace(item.SKU)
		if sku == "" {
			return nil, errors.New("Product SKU cannot be empty")
		}
		if item.Quantity <= 0 {
			return nil, errors.New("Quantity must be greater than 0")
		}

		name := item.Name
		var imageURL string
		productType := models.ProductType(item.ProductType)
		if name == "" || productType == "" {
			product, err := s.productRepo.FindBySKU(sku)
			if err != nil {
				if name == "" {
					return nil, fmt.Errorf("Product %s does not exist", sku)
				}
			} else {
				if name == "" {
					name = product.Name
				}
				if productType == "" {
					productType = product.ProductType
				}
				if len(product.Images) > 0 {
					imageURL = product.Images[0].URL
				}
			}
		}

		// 虚拟商品必须指定虚拟库存
		if productType == models.ProductTypeVirtual && item.VirtualInventoryID == nil {
			return nil, fmt.Errorf("Virtual product %s must select a virtual inventory", sku)
		}

		orderItems = append(orderItems, models.OrderItem{
			SKU:         sku,
			Name:        name,
			Quantity:    item.Quantity,
			Attributes:  item.Attributes,
			ProductType: productType,
			ImageURL:    imageURL,
		})
		totalAmount += item.UnitPrice * float64(item.Quantity)
		// 保存管理员指定的虚拟库存ID
		if item.VirtualInventoryID != nil {
			virtualInventoryIDs[idx] = item.VirtualInventoryID
		}
	}

	// 允许手动覆盖总金额
	if req.TotalAmount != nil {
		totalAmount = *req.TotalAmount
	}

	// 获取货币单位
	currency := s.cfg.Order.Currency
	if currency == "" {
		currency = "CNY"
	}

	// 生成订单号
	orderNo := utils.GenerateOrderNo(s.cfg.Order.NoPrefix)

	// 物理商品库存绑定
	inventoryBindings := make(map[int]uint)
	if s.bindingService != nil {
		for i := range orderItems {
			item := &orderItems[i]
			if item.ProductType == models.ProductTypeVirtual {
				continue
			}
			product, err := s.productRepo.FindBySKU(item.SKU)
			if err != nil {
				continue // 商品不在系统中，跳过库存绑定
			}

			attributesMap := make(map[string]string)
			if item.Attributes != nil {
				for k, v := range item.Attributes {
					if strVal, ok := v.(string); ok {
						attributesMap[k] = strVal
					}
				}
			}

			inventory, fullAttrs, invErr := s.bindingService.FindInventoryByAttributes(product.ID, attributesMap)
			if invErr != nil {
				continue // 没有匹配的库存配置，跳过
			}
			if canPurchase, _ := inventory.CanPurchase(item.Quantity); !canPurchase {
				continue // 库存不足，跳过
			}

			// 更新属性为完整属性
			for k, v := range fullAttrs {
				if item.Attributes == nil {
					item.Attributes = make(map[string]interface{})
				}
				item.Attributes[k] = v
			}
			if item.Attributes == nil {
				item.Attributes = make(map[string]interface{})
			}
			item.Attributes["_inventory_id"] = inventory.ID

			// 更新销量
			if err := s.productRepo.IncrementSaleCount(product.ID, item.Quantity); err != nil {
				fmt.Printf("Warning: Failed to update product sales count - ProductID: %d, Error: %v\n", product.ID, err)
			}
		}
	}

	// 预留物理库存
	for i := range orderItems {
		item := &orderItems[i]
		if inventoryIDVal, ok := item.Attributes["_inventory_id"]; ok {
			if inventoryID, ok := inventoryIDVal.(uint); ok {
				if err := s.inventoryRepo.Reserve(inventoryID, item.Quantity, orderNo); err != nil {
					// 回滚已预留的库存
					for j := 0; j < i; j++ {
						if prevID, exists := inventoryBindings[j]; exists {
							s.inventoryRepo.ReleaseReserve(prevID, orderItems[j].Quantity, orderNo)
						}
					}
					return nil, fmt.Errorf("Failed to reserve inventory: %v", err)
				}
				inventoryBindings[i] = inventoryID
				delete(item.Attributes, "_inventory_id")
			}
		}
	}

	// 订单状态：默认待付款
	status := models.OrderStatusPendingPayment
	if req.Status != "" {
		status = models.OrderStatus(req.Status)
	}

	// 生成表单Token（用于后续用户填写收货信息）
	// 仅实物/混合订单需要表单，虚拟商品订单不需要收货信息
	var formToken *string
	var formExpiresAt *time.Time
	hasShippingInfo := req.ReceiverName != "" && req.ReceiverAddress != ""
	isVirtualOnly := true
	for _, item := range orderItems {
		if item.ProductType != models.ProductTypeVirtual {
			isVirtualOnly = false
			break
		}
	}
	if !hasShippingInfo && !isVirtualOnly {
		token := uuid.New().String()
		formToken = &token
		expires := models.NowFunc().Add(time.Duration(s.cfg.Form.ExpireHours) * time.Hour)
		formExpiresAt = &expires
	}

	order := &models.Order{
		OrderNo:                   orderNo,
		UserID:                    req.UserID,
		Items:                     orderItems,
		InventoryBindings:         inventoryBindings,
		Status:                    status,
		TotalAmount:               totalAmount,
		Currency:                  currency,
		FormToken:                 formToken,
		FormExpiresAt:             formExpiresAt,
		Source:                    "admin",
		ReceiverName:              req.ReceiverName,
		PhoneCode:                 req.PhoneCode,
		ReceiverPhone:             req.ReceiverPhone,
		ReceiverEmail:             req.ReceiverEmail,
		ReceiverCountry:           req.ReceiverCountry,
		ReceiverProvince:          req.ReceiverProvince,
		ReceiverCity:              req.ReceiverCity,
		ReceiverDistrict:          req.ReceiverDistrict,
		ReceiverAddress:           req.ReceiverAddress,
		ReceiverPostcode:          req.ReceiverPostcode,
		UserEmail:                 req.UserEmail,
		EmailNotificationsEnabled: req.UserEmail != "",
		Remark:                    req.Remark,
		AdminRemark:               req.AdminRemark,
	}

	if err := s.OrderRepo.Create(order); err != nil {
		// 释放已预留的物理库存
		for i, inventoryID := range inventoryBindings {
			s.inventoryRepo.ReleaseReserve(inventoryID, orderItems[i].Quantity, orderNo)
		}
		return nil, err
	}

	// 虚拟产品预留库存（待付款状态，付款后才发货）
	if s.virtualProductSvc != nil {
		for i := range orderItems {
			item := &orderItems[i]
			if item.ProductType != models.ProductTypeVirtual {
				continue
			}

			// 如果管理员指定了虚拟库存ID，直接从该库存池分配
			if vid, ok := virtualInventoryIDs[i]; ok && vid != nil {
				_, err := s.virtualProductSvc.AllocateStockFromInventory(*vid, item.Quantity, orderNo)
				if err != nil {
					// 分配失败，回滚物理库存和订单
					for j, inventoryID := range inventoryBindings {
						s.inventoryRepo.ReleaseReserve(inventoryID, orderItems[j].Quantity, orderNo)
					}
					s.OrderRepo.Delete(order.ID)
					return nil, fmt.Errorf("Failed to allocate virtual product stock for %s: %v", item.Name, err)
				}
			} else {
				// 未指定虚拟库存ID，尝试通过商品绑定自动分配
				product, err := s.productRepo.FindBySKU(item.SKU)
				if err != nil {
					continue // 商品不在系统中，跳过虚拟库存绑定
				}

				allocAttrs := make(map[string]interface{})
				for k, v := range item.Attributes {
					allocAttrs[k] = v
				}
				_, err = s.virtualProductSvc.AllocateStockForProductByAttributes(product.ID, item.Quantity, orderNo, allocAttrs)
				if err != nil {
					// 分配失败，回滚物理库存和订单
					for j, inventoryID := range inventoryBindings {
						s.inventoryRepo.ReleaseReserve(inventoryID, orderItems[j].Quantity, orderNo)
					}
					s.OrderRepo.Delete(order.ID)
					return nil, fmt.Errorf("Failed to allocate virtual product stock for %s: %v", item.Name, err)
				}
			}

			// 更新虚拟商品销量
			product, err := s.productRepo.FindBySKU(item.SKU)
			if err == nil {
				if err := s.productRepo.IncrementSaleCount(product.ID, item.Quantity); err != nil {
					fmt.Printf("Warning: Failed to update virtual product sales count - ProductID: %d, Error: %v\n", product.ID, err)
				}
			}
		}
	}

	// 发送订单创建通知邮件
	if s.emailService != nil {
		go s.emailService.SendOrderCreatedEmail(order)
	}

	return order, nil
}

// CreateUserOrder User直接CreateOrder（无需表单流程）
func (s *OrderService) CreateUserOrder(userID uint, items []models.OrderItem, remark string, promoCode string) (*models.Order, error) {
	// 查找UserInfo
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, errors.New("User not found")
	}

	// 盲盒属性跟踪：记录每个订单项中盲盒随机分配的属性名
	// key: 订单项索引, value: 盲盒属性名列表
	blindBoxAttrNames := make(map[int][]string)

	// 验证Product并处理Inventory（使用新的Inventory绑定机制）
	for i := range items {
		item := &items[i]
		item.SKU = strings.TrimSpace(item.SKU)
		if item.SKU == "" {
			return nil, errors.New("Product SKU cannot be empty")
		}
		if item.Quantity <= 0 {
			return nil, errors.New("Quantity must be greater than 0")
		}

		// 根据 SKU 查找Product
		product, err := s.productRepo.FindBySKU(item.SKU)
		if err != nil {
			return nil, fmt.Errorf("Product %s does not exist", item.SKU)
		}
		if product.Status != models.ProductStatusActive {
			return nil, ErrProductNotAvailable
		}

		// 收集盲盒属性名，并从用户输入中剔除（防止用户手动指定盲盒结果）
		var bbAttrNames []string
		for _, attr := range product.Attributes {
			if attr.Mode == models.AttributeModeBlindBox {
				bbAttrNames = append(bbAttrNames, attr.Name)
			}
		}
		if item.Attributes == nil {
			item.Attributes = make(map[string]interface{})
		}
		for _, name := range bbAttrNames {
			delete(item.Attributes, name)
		}

		// 检查购买限制
		if product.MaxPurchaseLimit > 0 {
			// QueryUser已购买的数量
			purchasedQty, err := s.OrderRepo.GetUserPurchaseQuantityBySKU(userID, item.SKU)
			if err != nil {
				return nil, fmt.Errorf("Failed to query purchase records: %v", err)
			}

			// 检查是否超过限制
			if purchasedQty+item.Quantity > product.MaxPurchaseLimit {
				remaining := product.MaxPurchaseLimit - purchasedQty
				if remaining <= 0 {
					return nil, fmt.Errorf("Product %s has reached purchase limit (maximum %d per account)", product.Name, product.MaxPurchaseLimit)
				}
				return nil, fmt.Errorf("Product %s purchase quantity exceeds limit, you can still purchase %d (maximum %d per account)",
					product.Name, remaining, product.MaxPurchaseLimit)
			}
		}

		// 新的Inventory处理逻辑：根据Product的Inventory模式和User选择的属性查找对应的Inventory
		var inventory *models.Inventory
		var inventoryErr error

		// 检查是否为虚拟商品
		if product.ProductType == models.ProductTypeVirtual {
			// 保存商品类型到订单项
			item.ProductType = models.ProductTypeVirtual

			// 虚拟商品盲盒处理逻辑
			hasBlindBox := len(bbAttrNames) > 0
			hasUserSelect := false
			for _, attr := range product.Attributes {
				if attr.Mode != models.AttributeModeBlindBox {
					hasUserSelect = true
					break
				}
			}

			// 转换属性类型（已剔除盲盒属性）
			attrStrMap := make(map[string]string)
			for k, v := range item.Attributes {
				if str, ok := v.(string); ok {
					attrStrMap[k] = str
				}
			}

			// 根据盲盒模式处理虚拟商品
			if s.virtualProductSvc != nil {
				if hasBlindBox {
					// 盲盒模式 或 混合模式
					if hasUserSelect && len(attrStrMap) > 0 {
						// 混合模式：部分属性用户选择，部分属性盲盒随机
						_, fullAttrs, err := s.virtualProductSvc.FindVirtualInventoryWithPartialMatch(product.ID, attrStrMap, item.Quantity)
						if err != nil {
							return nil, fmt.Errorf("Failed to allocate virtual inventory for product %s: %v", product.Name, err)
						}
						// 更新订单项的属性为完整属性（包括随机分配的）
						for k, v := range fullAttrs {
							item.Attributes[k] = v
						}
					} else {
						// 纯盲盒模式：全部随机
						_, fullAttrs, err := s.virtualProductSvc.SelectRandomVirtualInventory(product.ID, item.Quantity)
						if err != nil {
							return nil, fmt.Errorf("Failed to allocate virtual inventory for product %s: %v", product.Name, err)
						}
						// 更新订单项的属性为完整属性（随机分配的）
						for k, v := range fullAttrs {
							item.Attributes[k] = v
						}
					}
					// 记录盲盒属性名，后续从items中剥离
					blindBoxAttrNames[i] = bbAttrNames
				} else {
					var availableCount int64
					var err error
					if len(attrStrMap) > 0 {
						// 根据规格属性检查库存
						availableCount, err = s.virtualProductSvc.GetAvailableCountForProductByAttributes(product.ID, attrStrMap)
					} else {
						// 无规格属性，检查总库存
						availableCount, err = s.virtualProductSvc.GetAvailableCountForProduct(product.ID)
					}
					if err != nil {
						return nil, fmt.Errorf("Failed to check virtual product stock: %v", err)
					}
					if availableCount < int64(item.Quantity) {
						return nil, fmt.Errorf("Virtual product %s stock insufficient, only %d available", product.Name, availableCount)
					}
				}
			}

			// 虚拟商品也需要更新销量
			if err := s.productRepo.IncrementSaleCount(product.ID, item.Quantity); err != nil {
				fmt.Printf("Warning: Failed to update virtual product sales count - ProductID: %d, Error: %v\n", product.ID, err)
			}
			// 虚拟商品不需要处理物理库存绑定
			continue
		}

		// 保存商品类型到订单项（实物商品）
		item.ProductType = models.ProductTypePhysical

		// 物理商品的库存处理
		// 将 item.Attributes (map[string]interface{}) 转换为 map[string]string（已剔除盲盒属性）
		attributesMap := make(map[string]string)
		for k, v := range item.Attributes {
			if strVal, ok := v.(string); ok {
				attributesMap[k] = strVal
			}
		}

		// 检查Product是否有盲盒属性
		hasBlindBox := len(bbAttrNames) > 0
		hasUserSelect := false
		for _, attr := range product.Attributes {
			if attr.Mode != models.AttributeModeBlindBox {
				hasUserSelect = true
				break
			}
		}

		if product.InventoryMode == string(models.InventoryModeRandom) || hasBlindBox {
			// 盲盒模式 或 混合模式（有盲盒属性）
			if hasUserSelect && len(attributesMap) > 0 {
				// 混合模式：部分属性User选择，部分属性盲盒随机
				var fullAttrs map[string]string
				inventory, fullAttrs, inventoryErr = s.bindingService.FindInventoryWithPartialMatch(product.ID, attributesMap, item.Quantity)
				if inventoryErr != nil {
					return nil, fmt.Errorf("Failed to allocate inventory for product %s: %v", product.Name, inventoryErr)
				}
				// UpdateOrder项的属性为完整属性（包括随机分配的）
				for k, v := range fullAttrs {
					item.Attributes[k] = v
				}
			} else {
				// 纯盲盒模式：全部随机
				var fullAttrs map[string]string
				inventory, fullAttrs, inventoryErr = s.bindingService.SelectRandomInventory(product.ID, item.Quantity)
				if inventoryErr != nil {
					return nil, fmt.Errorf("Failed to allocate inventory for product %s: %v", product.Name, inventoryErr)
				}
				// UpdateOrder项的属性为完整属性（随机分配的）
				for k, v := range fullAttrs {
					item.Attributes[k] = v
				}
			}
			// 记录盲盒属性名，后续从items中剥离
			blindBoxAttrNames[i] = bbAttrNames
		} else {
			var fullAttrs map[string]string
			inventory, fullAttrs, inventoryErr = s.bindingService.FindInventoryByAttributes(product.ID, attributesMap)
			if inventoryErr != nil {
				return nil, fmt.Errorf("No matching inventory found for product %s: %v", product.Name, inventoryErr)
			}
			// UpdateOrder项的属性为完整属性
			for k, v := range fullAttrs {
				item.Attributes[k] = v
			}
		}

		// 检查Inventory是否足够
		if canPurchase, msg := inventory.CanPurchase(item.Quantity); !canPurchase {
			return nil, fmt.Errorf("Product %s %s", product.Name, msg)
		}

		// 预留Inventory（generateOrder号后Update）
		// 注意：这里先记录need预留的InventoryID，CreateOrder后再调用预留
		// 使用 _inventory_id 作为临时标记，预留Success后会改为 inventory_id 永久保存
		item.Attributes["_inventory_id"] = inventory.ID

		// Increment sales count
		if err := s.productRepo.IncrementSaleCount(product.ID, item.Quantity); err != nil {
			// Sales count update failure does not affect order creation, just log it
			fmt.Printf("Warning: Failed to update product sales count - ProductID: %d, Error: %v\n", product.ID, err)
		}
	}

	// generateOrder号
	orderNo := utils.GenerateOrderNo(s.cfg.Order.NoPrefix)

	// Inventory绑定映射（Order项索引 -> InventoryID）
	inventoryBindings := make(map[int]uint)

	// 预留Inventory（在CreateOrder前）
	for i := range items {
		item := &items[i]

		if inventoryIDVal, ok := item.Attributes["_inventory_id"]; ok {
			if inventoryID, ok := inventoryIDVal.(uint); ok {
				// 预留Inventory
				if err := s.inventoryRepo.Reserve(inventoryID, item.Quantity, orderNo); err != nil {
					// 预留Failed，need回滚之前已预留的Inventory
					for j := 0; j < i; j++ {
						if prevInvID, exists := inventoryBindings[j]; exists {
							s.inventoryRepo.ReleaseReserve(prevInvID, items[j].Quantity, orderNo)
						}
					}
					return nil, fmt.Errorf("Failed to reserve inventory: %v", err)
				}
				// 保存Inventory绑定关系（使用独立的映射表，不污染Product属性）
				inventoryBindings[i] = inventoryID
				// 从属性中移除临时标记
				delete(item.Attributes, "_inventory_id")
			}
		}
	}

	// 将盲盒分配结果提取到 ActualAttributes，并从 items 中剥离盲盒属性
	// ActualAttributes 格式: { "0": {"color": "red"}, "1": {"size": "L"} } (key 为订单项索引)
	var actualAttrsJSON models.JSON
	if len(blindBoxAttrNames) > 0 {
		actualAttrsMap := make(map[string]map[string]interface{})
		for idx, attrNames := range blindBoxAttrNames {
			item := &items[idx]
			blindBoxValues := make(map[string]interface{})
			for _, name := range attrNames {
				if val, ok := item.Attributes[name]; ok {
					blindBoxValues[name] = val
					delete(item.Attributes, name)
				}
			}
			if len(blindBoxValues) > 0 {
				actualAttrsMap[fmt.Sprintf("%d", idx)] = blindBoxValues
			}
		}
		if len(actualAttrsMap) > 0 {
			jsonBytes, _ := json.Marshal(actualAttrsMap)
			actualAttrsJSON = models.JSON(string(jsonBytes))
		}
	}

	// CreateOrder
	// 所有订单创建时都是待付款状态
	orderStatus := models.OrderStatusPendingPayment

	// 计算订单总金额
	var totalAmount float64
	for _, item := range items {
		product, err := s.productRepo.FindBySKU(item.SKU)
		if err == nil {
			totalAmount += product.Price * float64(item.Quantity)
		}
	}

	// 获取货币单位
	currency := s.cfg.Order.Currency
	if currency == "" {
		currency = "CNY"
	}

	// 处理优惠码
	var promoCodeID *uint
	var promoCodeStr string
	var discountAmount float64
	if promoCode != "" && s.promoCodeRepo != nil {
		promoCodeRepo := s.promoCodeRepo
		pc, err := promoCodeRepo.FindByCode(strings.ToUpper(strings.TrimSpace(promoCode)))
		if err != nil {
			// 释放已预留的库存
			for i, inventoryID := range inventoryBindings {
				s.inventoryRepo.ReleaseReserve(inventoryID, items[i].Quantity, orderNo)
			}
			return nil, fmt.Errorf("Promo code not found")
		}
		if !pc.IsAvailable() {
			for i, inventoryID := range inventoryBindings {
				s.inventoryRepo.ReleaseReserve(inventoryID, items[i].Quantity, orderNo)
			}
			return nil, fmt.Errorf("Promo code is not available")
		}
		// 收集订单中的商品ID
		var productIDs []uint
		for _, item := range items {
			if p, err := s.productRepo.FindBySKU(item.SKU); err == nil {
				productIDs = append(productIDs, p.ID)
			}
		}
		// 检查优惠码是否适用于订单中的商品
		if len(pc.ProductIDs) > 0 {
			applicable := false
			for _, pid := range productIDs {
				if pc.IsApplicableToProduct(pid) {
					applicable = true
					break
				}
			}
			if !applicable {
				for i, inventoryID := range inventoryBindings {
					s.inventoryRepo.ReleaseReserve(inventoryID, items[i].Quantity, orderNo)
				}
				return nil, fmt.Errorf("Promo code is not applicable to the selected products")
			}
		}
		discountAmount = pc.CalculateDiscount(totalAmount)
		// 预留优惠码
		if err := promoCodeRepo.Reserve(pc.ID, orderNo); err != nil {
			for i, inventoryID := range inventoryBindings {
				s.inventoryRepo.ReleaseReserve(inventoryID, items[i].Quantity, orderNo)
			}
			return nil, fmt.Errorf("Failed to reserve promo code: %v", err)
		}
		promoCodeID = &pc.ID
		promoCodeStr = pc.Code
	}

	order := &models.Order{
		OrderNo:                   orderNo,
		UserID:                    &userID,
		Items:                     items,
		ActualAttributes:          actualAttrsJSON,
		InventoryBindings:         inventoryBindings, // 保存Inventory绑定关系（内部使用）
		Status:                    orderStatus,
		TotalAmount:               totalAmount - discountAmount,
		Currency:                  currency,
		PromoCodeID:               promoCodeID,
		PromoCodeStr:              promoCodeStr,
		DiscountAmount:            discountAmount,
		Source:                    "web",
		UserEmail:                 user.Email,
		EmailNotificationsEnabled: true,
		Remark:                    remark,
		// FormToken 和 FormExpiresAt 在User点击填写时动态generate（仅非虚拟商品订单需要）
	}

	if err := s.OrderRepo.Create(order); err != nil {
		// CreateOrderFailed，释放已预留的Inventory
		for i, inventoryID := range inventoryBindings {
			s.inventoryRepo.ReleaseReserve(inventoryID, items[i].Quantity, orderNo)
		}
		// 释放优惠码
		if promoCodeID != nil && s.promoCodeRepo != nil {
			s.promoCodeRepo.ReleaseReserve(*promoCodeID, orderNo)
		}
		return nil, err
	}

	// 虚拟产品预留库存（待付款状态，付款后才发货）
	if s.virtualProductSvc != nil {
		for i := range items {
			item := &items[i]
			product, err := s.productRepo.FindBySKU(item.SKU)
			if err == nil && product.ProductType == models.ProductTypeVirtual {
				// 为虚拟产品分配库存（预留状态），传入完整规格属性
				// 需要从 ActualAttributes 中合并盲盒属性回来用于库存匹配
				allocAttrs := make(map[string]interface{})
				for k, v := range item.Attributes {
					allocAttrs[k] = v
				}
				if bbNames, ok := blindBoxAttrNames[i]; ok && len(actualAttrsJSON) > 0 {
					var actualMap map[string]map[string]interface{}
					if err := json.Unmarshal([]byte(actualAttrsJSON), &actualMap); err == nil {
						if bbVals, ok := actualMap[fmt.Sprintf("%d", i)]; ok {
							for _, name := range bbNames {
								if v, ok := bbVals[name]; ok {
									allocAttrs[name] = v
								}
							}
						}
					}
				}
				_, err := s.virtualProductSvc.AllocateStockForProductByAttributes(product.ID, item.Quantity, orderNo, allocAttrs)
				if err != nil {
					// 分配失败，需要回滚订单和已分配的物理库存
					for j, inventoryID := range inventoryBindings {
						s.inventoryRepo.ReleaseReserve(inventoryID, items[j].Quantity, orderNo)
					}
					s.OrderRepo.Delete(order.ID)
					return nil, fmt.Errorf("Failed to allocate virtual product stock: %v", err)
				}
			}
		}
		// 注意：待付款状态不自动发货，需要管理员标记付款后才发货
	}

	// 发送订单创建通知邮件
	if s.emailService != nil {
		go s.emailService.SendOrderCreatedEmail(order)
	}

	return order, nil
}

// SubmitShippingForm 提交发货Info表单
func (s *OrderService) SubmitShippingForm(formToken string, receiverInfo map[string]interface{}, privacyProtected bool, userPassword string, userRemark string) (*models.Order, *models.User, bool, error) {
	// Find order
	order, err := s.OrderRepo.FindByFormToken(formToken)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, false, errors.New("Form not found or expired")
		}
		return nil, nil, false, err
	}

	// 保存原始状态，用于判断是否是重填操作
	isResubmit := order.Status == models.OrderStatusNeedResubmit

	// 检查订单状态：只有草稿和需要重填状态的订单可以提交表单
	// 防止待付款状态的订单通过表单提交跳过付款步骤
	if order.Status != models.OrderStatusDraft && order.Status != models.OrderStatusNeedResubmit {
		return nil, nil, false, errors.New("Order is not ready for shipping information submission")
	}

	// 检查表单是否已提交（重填时FormSubmittedAt会被清空，所以这里不检查重填的情况）
	if order.FormSubmittedAt != nil && !isResubmit {
		return nil, nil, false, errors.New("Form has already been submitted")
	}

	// Check if form has expired
	if order.FormExpiresAt != nil && models.NowFunc().After(*order.FormExpiresAt) {
		return nil, nil, false, errors.New("Form has expired")
	}

	// UpdateOrder收货Info
	order.ReceiverName = receiverInfo["receiver_name"].(string)
	if phoneCode, ok := receiverInfo["phone_code"].(string); ok {
		order.PhoneCode = phoneCode
	}
	order.ReceiverPhone = receiverInfo["receiver_phone"].(string)
	order.ReceiverEmail = receiverInfo["receiver_email"].(string)
	if country, ok := receiverInfo["receiver_country"].(string); ok {
		order.ReceiverCountry = country
	}
	if province, ok := receiverInfo["receiver_province"].(string); ok {
		order.ReceiverProvince = province
	}
	if city, ok := receiverInfo["receiver_city"].(string); ok {
		order.ReceiverCity = city
	}
	if district, ok := receiverInfo["receiver_district"].(string); ok {
		order.ReceiverDistrict = district
	}
	order.ReceiverAddress = receiverInfo["receiver_address"].(string)
	if postcode, ok := receiverInfo["receiver_postcode"].(string); ok {
		order.ReceiverPostcode = postcode
	}
	order.PrivacyProtected = privacyProtected

	// 保存User备注（追加到原有备注后）
	if userRemark != "" {
		if order.Remark != "" {
			// 如果已有平台备注，追加User备注
			order.Remark = order.Remark + "\n\n【User备注】\n" + userRemark
		} else {
			// 如果没有平台备注，直接保存User备注
			order.Remark = userRemark
		}
	}

	// UpdateOrder状态
	order.Status = models.OrderStatusPending
	now := models.NowFunc()
	order.FormSubmittedAt = &now

	// 查找或CreateUser
	var user *models.User
	isNewUser := false
	var generatedPassword string

	// If this is a resubmission, use the existing user, do not create a new one
	if isResubmit {
		// Resubmit operation: order already has user association
		if order.UserID == nil {
			return nil, nil, false, errors.New("Resubmit order missing user association")
		}

		// Find associated user
		user, err = s.userRepo.FindByID(*order.UserID)
		if err != nil {
			return nil, nil, false, errors.New("Cannot find user associated with order")
		}

		isNewUser = false // Resubmission is not a new user
	} else {
		// First submission: find or create user
		// 先尝试通过Email查找
		normalizedEmail := strings.ToLower(strings.TrimSpace(order.ReceiverEmail))
		order.ReceiverEmail = normalizedEmail
		user, err = s.userRepo.FindByEmail(normalizedEmail)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// Userdoes not exist，Create新User
				isNewUser = true

				// generate随机Password（如果未提供）
				if userPassword == "" {
					generatedPassword, err = password.GenerateRandomPassword(12)
					if err != nil {
						return nil, nil, false, err
					}
				} else {
					generatedPassword = userPassword
				}

				// Enforce the same password policy as normal registration.
				policy := s.cfg.Security.PasswordPolicy
				if err := password.ValidatePasswordPolicy(generatedPassword, policy.MinLength, policy.RequireUppercase,
					policy.RequireLowercase, policy.RequireNumber, policy.RequireSpecial); err != nil {
					return nil, nil, false, err
				}

				// 哈希Password
				hashedPassword, err := password.HashPassword(generatedPassword)
				if err != nil {
					return nil, nil, false, err
				}

				user = &models.User{
					UUID:         uuid.New().String(),
					Email:        normalizedEmail,
					Name:         order.ReceiverName,
					PasswordHash: hashedPassword,
					Role:         "user",
					IsActive:     true,
				}
				// 只有phone不为空时才设置
				if order.ReceiverPhone != "" {
					user.Phone = &order.ReceiverPhone
				}
				if err := s.userRepo.Create(user); err != nil {
					// Race: another request created the same user after our FindByEmail.
					if isUniqueConstraintError(err) {
						existing, findErr := s.userRepo.FindByEmail(normalizedEmail)
						if findErr != nil {
							return nil, nil, false, findErr
						}
						user = existing
						isNewUser = false
						generatedPassword = ""
					} else {
						return nil, nil, false, err
					}
				}
			} else {
				return nil, nil, false, err
			}
		} else {
			// User已存在，检查是否是Admin账户
			if user.Role == "admin" || user.Role == "super_admin" {
				// 如果是Admin账户，允许关联但不发送欢迎邮件
				// 这是正常情况，Admin可以为自己CreateOrder
			}
			// 普通User账户已存在，直接关联Order
			isNewUser = false
		}

		// 关联User（首次提交时）
		order.UserID = &user.ID
	}

	// UpdateOrder
	if err := s.OrderRepo.Update(order); err != nil {
		return nil, nil, false, err
	}

	// 注意：虚拟产品在 MarkAsPaid 时已经发货，这里不需要再次发货
	// SubmitShippingForm 只用于实物订单填写收货信息

	// 填写收货信息后立即生成产品序列号（非重填时）
	if !isResubmit && s.serialService != nil {
		for i := range order.Items {
			item := &order.Items[i]

			// 通过SKU查找商品并检查是否有产品码
			product, err := s.productRepo.FindBySKU(item.SKU)
			if err == nil && product.ProductCode != "" {
				// 为此商品生成序列号
				_, err := s.serialService.CreateSerialForOrder(
					order.ID,
					product.ID,
					item.Quantity,
				)
				if err != nil {
					// 记录错误但不阻止流程
					fmt.Printf("Warning: Failed to create serials for product %s (ID:%d) in order %s: %v\n",
						item.SKU, product.ID, order.OrderNo, err)
				}
			}
		}
	}

	// 发送邮件通知
	if s.emailService != nil {
		// 首次提交时发送OrderCreate邮件，重填时不发送
		if !isResubmit {
			go s.emailService.SendOrderCreatedEmail(order)
		}
	}

	return order, user, isNewUser, nil
}

// GetOrderByNo 根据Order号getOrder
func (s *OrderService) GetOrderByNo(orderNo string) (*models.Order, error) {
	return s.OrderRepo.FindByOrderNo(orderNo)
}

// GetOrderByID 根据IDgetOrder
func (s *OrderService) GetOrderByID(id uint) (*models.Order, error) {
	return s.OrderRepo.FindByID(id)
}

// ListOrders getOrder List
func (s *OrderService) ListOrders(page, limit int, status, search, country, productSearch string, promoCodeID *uint, promoCode string, userID *uint) ([]models.Order, int64, error) {
	return s.OrderRepo.List(page, limit, status, search, country, productSearch, promoCodeID, promoCode, userID)
}

// GetOrderCountries get所有有Order的国家列表
func (s *OrderService) GetOrderCountries() ([]string, error) {
	return s.OrderRepo.GetOrderCountries()
}

// ListUserOrders getUserOrder List
func (s *OrderService) ListUserOrders(userID uint, page, limit int, status string) ([]models.Order, int64, error) {
	return s.OrderRepo.FindByUserID(userID, page, limit, status)
}

// AssignTracking 分配物流单号
func (s *OrderService) AssignTracking(orderID uint, trackingNo string) error {
	order, err := s.OrderRepo.FindByID(orderID)
	if err != nil {
		return err
	}

	// 只有待发货状态的订单可以分配物流单号
	if order.Status != models.OrderStatusPending {
		return errors.New("Only pending orders can be assigned tracking number")
	}

	// 发货时将预留Inventory转为已售Inventory
	for i := range order.Items {
		item := &order.Items[i]

		// 从Inventory绑定映射中getInventoryID
		if inventoryID, exists := order.InventoryBindings[i]; exists && inventoryID > 0 {
			// 扣减Inventory：从预留转为已售
			if err := s.inventoryRepo.Deduct(inventoryID, item.Quantity, order.OrderNo); err != nil {
				// 扣减Failed但不阻止发货，记录Error日志
				fmt.Printf("Warning: Order %s Failed to deduct inventory: %v\n", order.OrderNo, err)
			}
		}
	}

	// 混合订单发货时，同时发货剩余的虚拟商品库存（非自动发货的部分）
	if s.virtualProductSvc != nil {
		hasPending, _ := s.virtualProductSvc.HasPendingVirtualStock(order.OrderNo)
		if hasPending {
			if err := s.virtualProductSvc.DeliverStock(order.ID, order.OrderNo, nil); err != nil {
				fmt.Printf("Warning: Order %s failed to deliver remaining virtual stock: %v\n", order.OrderNo, err)
			}
		}
	}

	order.TrackingNo = trackingNo
	order.Status = models.OrderStatusShipped
	now := models.NowFunc()
	order.ShippedAt = &now

	if err := s.OrderRepo.Update(order); err != nil {
		return err
	}

	// 发送发货邮件通知
	if s.emailService != nil {
		go s.emailService.SendOrderShippedEmail(order)
	}

	return nil
}

// DeliverVirtualStock 手动发货虚拟商品库存（用于 auto_delivery=false 的商品）
func (s *OrderService) DeliverVirtualStock(orderID uint, deliveredBy uint) error {
	order, err := s.OrderRepo.FindByID(orderID)
	if err != nil {
		return err
	}

	// 只有待发货和已发货状态的订单可以手动发货虚拟商品
	if order.Status != models.OrderStatusPending && order.Status != models.OrderStatusShipped {
		return errors.New("Only pending or shipped orders can deliver virtual stock")
	}

	if s.virtualProductSvc == nil {
		return errors.New("Virtual product service not available")
	}

	// 检查是否有待发货的虚拟库存
	hasPending, err := s.virtualProductSvc.HasPendingVirtualStock(order.OrderNo)
	if err != nil {
		return fmt.Errorf("Failed to check pending stock: %v", err)
	}
	if !hasPending {
		return errors.New("No pending virtual stock to deliver")
	}

	// 发货所有剩余的预留虚拟库存
	if err := s.virtualProductSvc.DeliverStock(order.ID, order.OrderNo, &deliveredBy); err != nil {
		return fmt.Errorf("Failed to deliver virtual stock: %v", err)
	}

	// 判断订单是否为纯虚拟商品订单
	isVirtualOnly := true
	for _, item := range order.Items {
		if item.ProductType != models.ProductTypeVirtual {
			isVirtualOnly = false
			break
		}
	}

	// 纯虚拟订单且当前为待发货状态：自动转为已发货
	if isVirtualOnly && order.Status == models.OrderStatusPending {
		order.Status = models.OrderStatusShipped
		now := models.NowFunc()
		order.ShippedAt = &now

		if err := s.OrderRepo.Update(order); err != nil {
			return err
		}

		// 发送发货邮件通知
		if s.emailService != nil {
			go s.emailService.SendOrderShippedEmail(order)
		}
	}

	return nil
}

// CompleteOrder 完成Order
func (s *OrderService) CompleteOrder(orderID uint, completedBy uint, feedback, adminRemark string) error {
	order, err := s.OrderRepo.FindByID(orderID)
	if err != nil {
		return err
	}

	if order.Status != models.OrderStatusShipped {
		return errors.New("Order not shipped, cannot mark as completed")
	}

	order.Status = models.OrderStatusCompleted
	now := models.NowFunc()
	order.CompletedAt = &now
	order.CompletedBy = &completedBy
	if feedback != "" {
		order.UserFeedback = feedback
	}
	if adminRemark != "" {
		order.AdminRemark = adminRemark
	}

	// 扣减优惠码（从预留转为已使用）
	if order.PromoCodeID != nil && s.promoCodeRepo != nil {
		if err := s.promoCodeRepo.Deduct(*order.PromoCodeID, order.OrderNo); err != nil {
			fmt.Printf("Warning: Order %s Failed to deduct promo code: %v\n", order.OrderNo, err)
		}
	}

	if err := s.OrderRepo.Update(order); err != nil {
		return err
	}

	// 发送完成邮件通知
	if s.emailService != nil {
		go s.emailService.SendOrderCompletedEmail(order)
	}

	return nil
}

// RequestResubmit 要求重填Info
func (s *OrderService) RequestResubmit(orderID uint, reason string) (string, error) {
	order, err := s.OrderRepo.FindByID(orderID)
	if err != nil {
		return "", err
	}

	// generate新的表单Token
	formToken := uuid.New().String()
	formExpiresAt := models.NowFunc().Add(time.Duration(s.cfg.Form.ExpireHours) * time.Hour)

	order.Status = models.OrderStatusNeedResubmit
	order.FormToken = &formToken
	order.FormExpiresAt = &formExpiresAt
	order.FormSubmittedAt = nil // 清空提交时间，允许重新提交
	order.AdminRemark = reason

	if err := s.OrderRepo.Update(order); err != nil {
		return "", err
	}

	// 发送重填通知邮件
	if s.emailService != nil {
		formURL := s.cfg.App.URL + "/form/shipping?token=" + formToken
		go s.emailService.SendOrderResubmitEmail(order, formURL)
	}

	return formToken, nil
}

// DeleteOrder DeleteOrder（软Delete）
func (s *OrderService) DeleteOrder(orderID uint) error {
	order, err := s.OrderRepo.FindByID(orderID)
	if err != nil {
		return err
	}

	// 只有待付款、草稿和已取消的Order可以Delete
	if order.Status != models.OrderStatusPendingPayment && order.Status != models.OrderStatusDraft && order.Status != models.OrderStatusCancelled {
		return errors.New("Only pending payment, draft or cancelled orders can be deleted")
	}

	// 删除待付款订单时释放预留库存
	if order.Status == models.OrderStatusPendingPayment {
		// 释放物理商品库存
		for i := range order.Items {
			item := &order.Items[i]
			if inventoryID, exists := order.InventoryBindings[i]; exists && inventoryID > 0 {
				if err := s.inventoryRepo.ReleaseReserve(inventoryID, item.Quantity, order.OrderNo); err != nil {
					fmt.Printf("Warning: Order %s Failed to release reserved inventory: %v\n", order.OrderNo, err)
				}
			}
		}
		// 释放虚拟商品库存
		if s.virtualProductSvc != nil {
			if err := s.virtualProductSvc.ReleaseStock(order.OrderNo); err != nil {
				fmt.Printf("Warning: Order %s Failed to release virtual product stock: %v\n", order.OrderNo, err)
			}
		}
		// 释放优惠码
		if order.PromoCodeID != nil && s.promoCodeRepo != nil {
			if err := s.promoCodeRepo.ReleaseReserve(*order.PromoCodeID, order.OrderNo); err != nil {
				fmt.Printf("Warning: Order %s Failed to release promo code: %v\n", order.OrderNo, err)
			}
		}
	}

	// Delete serial numbers associated with this order before deleting the order
	if s.serialService != nil {
		if err := s.serialService.DeleteSerialsByOrderID(orderID); err != nil {
			// Log error but don't block deletion process
			fmt.Printf("Warning: Order %s failed to delete serial numbers: %v\n", order.OrderNo, err)
		}
	}

	return s.OrderRepo.Delete(orderID)
}

// UpdateOrder UpdateOrderInfo
func (s *OrderService) UpdateOrder(order *models.Order) error {
	return s.OrderRepo.Update(order)
}

// CancelOrder 取消Order
func (s *OrderService) CancelOrder(orderID uint, reason string) error {
	order, err := s.OrderRepo.FindByID(orderID)
	if err != nil {
		return err
	}

	// 已发货和已完成的Order不能取消
	if order.Status == models.OrderStatusShipped || order.Status == models.OrderStatusCompleted {
		return errors.New("Shipped or completed orders cannot be cancelled")
	}

	// 取消Order时释放预留Inventory
	// 待付款、草稿状态和待发货状态的Order有预留Inventoryneed释放
	if order.Status == models.OrderStatusPendingPayment || order.Status == models.OrderStatusDraft || order.Status == models.OrderStatusPending || order.Status == models.OrderStatusNeedResubmit {
		// 释放物理商品库存
		for i := range order.Items {
			item := &order.Items[i]

			// 从Inventory绑定映射中getInventoryID
			if inventoryID, exists := order.InventoryBindings[i]; exists && inventoryID > 0 {
				// 释放预留Inventory
				if err := s.inventoryRepo.ReleaseReserve(inventoryID, item.Quantity, order.OrderNo); err != nil {
					// 释放Failed但不阻止取消流程，记录Error日志
					fmt.Printf("Warning: Order %s Failed to release reserved inventory: %v\n", order.OrderNo, err)
				}
			}
		}

		// 释放虚拟商品库存
		if s.virtualProductSvc != nil {
			if err := s.virtualProductSvc.ReleaseStock(order.OrderNo); err != nil {
				fmt.Printf("Warning: Order %s Failed to release virtual product stock: %v\n", order.OrderNo, err)
			}
		}

		// 释放优惠码
		if order.PromoCodeID != nil && s.promoCodeRepo != nil {
			if err := s.promoCodeRepo.ReleaseReserve(*order.PromoCodeID, order.OrderNo); err != nil {
				fmt.Printf("Warning: Order %s Failed to release promo code: %v\n", order.OrderNo, err)
			}
		}
	}

	// Delete serial numbers associated with this order
	if s.serialService != nil {
		if err := s.serialService.DeleteSerialsByOrderID(orderID); err != nil {
			// Log error but don't block cancellation process
			fmt.Printf("Warning: Order %s failed to delete serial numbers: %v\n", order.OrderNo, err)
		}
	}

	order.Status = models.OrderStatusCancelled
	if reason != "" {
		order.AdminRemark = reason
	}

	if err := s.OrderRepo.Update(order); err != nil {
		return err
	}

	// 发送Order取消邮件
	if s.emailService != nil {
		go s.emailService.SendOrderCancelledEmail(order)
	}

	return nil
}

// MarkAsPaid 标记订单为已付款
func (s *OrderService) MarkAsPaid(orderID uint) error {
	order, err := s.OrderRepo.FindByID(orderID)
	if err != nil {
		return err
	}

	// 只有待付款状态的订单可以标记为已付款
	if order.Status != models.OrderStatusPendingPayment {
		return errors.New("Only pending payment orders can be marked as paid")
	}

	// 判断是否为纯虚拟商品订单
	isVirtualOnly := true
	for _, item := range order.Items {
		if item.ProductType != models.ProductTypeVirtual {
			isVirtualOnly = false
			break
		}
	}

	// 根据订单类型设置状态
	if isVirtualOnly {
		// 纯虚拟商品订单
		if s.virtualProductSvc != nil {
			// 检查是否所有虚拟库存都可以自动发货
			canAuto, err := s.virtualProductSvc.CanAutoDeliver(order.OrderNo)
			if err != nil {
				return fmt.Errorf("Failed to check auto delivery: %v", err)
			}

			if canAuto {
				// 所有库存都属于自动发货商品，执行自动发货
				if err := s.virtualProductSvc.DeliverAutoDeliveryStock(order.ID, order.OrderNo, nil); err != nil {
					return fmt.Errorf("Failed to deliver virtual product: %v", err)
				}
				order.Status = models.OrderStatusShipped
				now := models.NowFunc()
				order.ShippedAt = &now
			} else {
				// 存在非自动发货的库存，全部交给管理员手动发货
				order.Status = models.OrderStatusPending
			}
		} else {
			order.Status = models.OrderStatusPending
		}
	} else {
		// 实物或混合订单
		// 如果已经有收货信息，直接进入待发货状态；否则设置为草稿状态，等待填写收货信息
		hasShippingInfo := order.ReceiverName != "" && order.ReceiverAddress != ""
		if hasShippingInfo {
			order.Status = models.OrderStatusPending
		} else {
			order.Status = models.OrderStatusDraft
		}

		// 混合订单：仅当所有虚拟库存都可自动发货时才自动发货，否则全部留给管理员
		if s.virtualProductSvc != nil {
			canAuto, _ := s.virtualProductSvc.CanAutoDeliver(order.OrderNo)
			if canAuto {
				if err := s.virtualProductSvc.DeliverAutoDeliveryStock(order.ID, order.OrderNo, nil); err != nil {
					fmt.Printf("Warning: Failed to deliver virtual products for mixed order %s: %v\n", order.OrderNo, err)
				}
			}
		}
	}

	if err := s.OrderRepo.Update(order); err != nil {
		return err
	}

	// 发送付款成功邮件
	if s.emailService != nil {
		go s.emailService.SendOrderPaidEmail(order, isVirtualOnly)
	}

	return nil
}

// MaskOrderIfNeeded 如果need，打码Order敏感Info
func (s *OrderService) MaskOrderIfNeeded(order *models.Order, hasPrivacyPermission bool) {
	if order.PrivacyProtected && !hasPrivacyPermission {
		order.MaskSensitiveInfo()
	}
}

// GetOrRefreshFormToken - Get or refresh form token
// 如果Tokendoes not exist或已过期，则generate新的Token
func (s *OrderService) GetOrRefreshFormToken(order *models.Order) (string, *time.Time, error) {
	now := models.NowFunc()

	// 检查是否needgenerate新Token
	needNewToken := false

	// 情况1：没有Token
	if order.FormToken == nil || *order.FormToken == "" {
		needNewToken = true
	}

	// 情况2：Token已过期
	if !needNewToken && order.FormExpiresAt != nil && now.After(*order.FormExpiresAt) {
		needNewToken = true
	}

	// 情况3：过期时间为空（数据异常）
	if !needNewToken && order.FormToken != nil && *order.FormToken != "" && order.FormExpiresAt == nil {
		needNewToken = true
	}

	// 情况4：即将过期（剩余时间少于1小时），提前刷新
	if !needNewToken && order.FormExpiresAt != nil {
		timeLeft := order.FormExpiresAt.Sub(now)
		if timeLeft < time.Hour {
			needNewToken = true
		}
	}

	// 如果need新Token，则generate并Update
	if needNewToken {
		// generate新的 UUID Token
		newToken := uuid.New().String()
		newExpiresAt := now.Add(time.Duration(s.cfg.Form.ExpireHours) * time.Hour)

		// UpdateOrder
		order.FormToken = &newToken
		order.FormExpiresAt = &newExpiresAt

		// 持久化到数据库
		if err := s.OrderRepo.Update(order); err != nil {
			return "", nil, errors.New("Failed to update form token")
		}

		return newToken, &newExpiresAt, nil
	}

	// 返回现有的有效Token
	return *order.FormToken, order.FormExpiresAt, nil
}

// IsOrderSharedToSupport 检查订单是否被分享到客服工单
func (s *OrderService) IsOrderSharedToSupport(orderID uint) (bool, error) {
	return s.OrderRepo.IsOrderSharedToSupport(orderID)
}

// GetSharedOrderIDs 获取指定订单ID列表中被分享到客服的订单ID集合
func (s *OrderService) GetSharedOrderIDs(orderIDs []uint) (map[uint]bool, error) {
	return s.OrderRepo.GetSharedOrderIDs(orderIDs)
}
