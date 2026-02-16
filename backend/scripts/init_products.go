package main

import (
	"fmt"
	"log"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/database"
	"auralogic/internal/models"
)

func main() {
	log.Println("Initializing sample products...")

	// Load configuration
	cfg, err := config.LoadConfig(config.GetConfigPath())
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database
	if err := database.InitDatabase(&cfg.Database); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	db := database.GetDB()

	// Auto migrate product table
	log.Println("Creating product table...")
	if err := db.AutoMigrate(&models.Product{}); err != nil {
		log.Fatalf("Failed to create product table: %v", err)
	}
	log.Println("✓ Product table created successfully")

	// 示例Product数据
	products := []models.Product{
		{
			SKU:              "DEMO-001",
			Name:             "智能手机 Pro Max",
			Description:      "这是一款高端智能手机，配备最新的处理器和摄像系统。\n\n特点：\n- 6.7英寸OLED显示屏\n- 5G网络支持\n- 48MP三摄系统\n- 5000mAh大电池\n- 快充技术",
			ShortDescription: "旗舰级智能手机，性能强劲，拍照出色",
			Category:         "电子产品",
			Tags:             []string{"热销", "新品", "5G"},
			Price:            4999.00,
			OriginalPrice:    5999.00,
			Stock:            50,
			Images: []models.ProductImage{
				{URL: "https://picsum.photos/seed/phone1/800/800", Alt: "手机正面", IsPrimary: true},
				{URL: "https://picsum.photos/seed/phone2/800/800", Alt: "手机背面", IsPrimary: false},
				{URL: "https://picsum.photos/seed/phone3/800/800", Alt: "手机侧面", IsPrimary: false},
			},
			Attributes: []models.ProductAttribute{
				{Name: "颜色", Values: []string{"星空黑", "冰川蓝", "薄雾金"}},
				{Name: "内存", Values: []string{"8GB", "12GB", "16GB"}},
				{Name: "存储", Values: []string{"128GB", "256GB", "512GB"}},
			},
			Status:        models.ProductStatusActive,
			SortOrder:     100,
			IsFeatured:    true,
			IsRecommended: true,
		},
		{
			SKU:              "DEMO-002",
			Name:             "无线蓝牙耳机",
			Description:      "轻便舒适的无线耳机，提供出色的音质和降噪体验。\n\n特点：\n- 主动降噪\n- 30小时续航\n- 快速充电\n- 触控操作",
			ShortDescription: "主动降噪，音质出众，续航持久",
			Category:         "电子产品",
			Tags:             []string{"热销", "降噪"},
			Price:            299.00,
			OriginalPrice:    399.00,
			Stock:            100,
			Images: []models.ProductImage{
				{URL: "https://picsum.photos/seed/earphone1/800/800", Alt: "耳机展示", IsPrimary: true},
				{URL: "https://picsum.photos/seed/earphone2/800/800", Alt: "耳机充电盒", IsPrimary: false},
			},
			Attributes: []models.ProductAttribute{
				{Name: "颜色", Values: []string{"纯白", "深黑"}},
			},
			Status:        models.ProductStatusActive,
			SortOrder:     90,
			IsFeatured:    true,
			IsRecommended: false,
		},
		{
			SKU:              "DEMO-003",
			Name:             "智能手表运动版",
			Description:      "全天候健康监测，支持多种运动模式的智能手表。\n\n特点：\n- 心率监测\n- 血氧检测\n- GPS定位\n- 防水50米\n- 14天续航",
			ShortDescription: "健康监测，运动追踪，超长续航",
			Category:         "智能穿戴",
			Tags:             []string{"运动", "健康"},
			Price:            899.00,
			OriginalPrice:    0,
			Stock:            80,
			Images: []models.ProductImage{
				{URL: "https://picsum.photos/seed/watch1/800/800", Alt: "手表正面", IsPrimary: true},
				{URL: "https://picsum.photos/seed/watch2/800/800", Alt: "运动模式", IsPrimary: false},
			},
			Attributes: []models.ProductAttribute{
				{Name: "表带颜色", Values: []string{"黑色", "橙色", "蓝色"}},
				{Name: "尺寸", Values: []string{"42mm", "46mm"}},
			},
			Status:        models.ProductStatusActive,
			SortOrder:     80,
			IsFeatured:    false,
			IsRecommended: true,
		},
		{
			SKU:              "DEMO-004",
			Name:             "便携式移动电源",
			Description:      "大容量移动电源，支持快充和多设备同时充电。\n\n特点：\n- 20000mAh容量\n- 双向快充\n- 3个USB接口\n- 轻薄便携",
			ShortDescription: "大容量，快充，多设备支持",
			Category:         "电子配件",
			Tags:             []string{"实用", "快充"},
			Price:            149.00,
			OriginalPrice:    199.00,
			Stock:            200,
			Images: []models.ProductImage{
				{URL: "https://picsum.photos/seed/powerbank1/800/800", Alt: "移动电源", IsPrimary: true},
			},
			Attributes: []models.ProductAttribute{
				{Name: "颜色", Values: []string{"白色", "黑色"}},
			},
			Status:     models.ProductStatusActive,
			SortOrder:  70,
			IsFeatured: false,
		},
		{
			SKU:              "DEMO-005",
			Name:             "机械键盘RGB版",
			Description:      "专业游戏机械键盘，RGB背光，青轴手感。\n\n特点：\n- 青轴机械开关\n- 全键无冲\n- RGB背光\n- 铝合金外壳",
			ShortDescription: "游戏利器，手感出色，灯效炫酷",
			Category:         "电脑配件",
			Tags:             []string{"游戏", "机械键盘"},
			Price:            499.00,
			OriginalPrice:    699.00,
			Stock:            60,
			Images: []models.ProductImage{
				{URL: "https://picsum.photos/seed/keyboard1/800/800", Alt: "键盘全景", IsPrimary: true},
				{URL: "https://picsum.photos/seed/keyboard2/800/800", Alt: "RGB灯效", IsPrimary: false},
			},
			Attributes: []models.ProductAttribute{
				{Name: "轴体", Values: []string{"青轴", "红轴", "茶轴"}},
				{Name: "布局", Values: []string{"87键", "104键"}},
			},
			Status:        models.ProductStatusActive,
			SortOrder:     60,
			IsFeatured:    false,
			IsRecommended: true,
		},
		{
			SKU:              "DEMO-006",
			Name:             "4K高清摄像头",
			Description:      "专业级4K摄像头，适合直播和视频会议。",
			ShortDescription: "4K画质，自动对焦，降噪麦克风",
			Category:         "电脑配件",
			Tags:             []string{"直播", "4K"},
			Price:            799.00,
			OriginalPrice:    0,
			Stock:            0, // 缺货状态
			Images: []models.ProductImage{
				{URL: "https://picsum.photos/seed/camera1/800/800", Alt: "摄像头", IsPrimary: true},
			},
			Status:     models.ProductStatusOutOfStock,
			SortOrder:  50,
			IsFeatured: false,
		},
		{
			SKU:              "DEMO-007",
			Name:             "办公鼠标无线版",
			Description:      "舒适的人体工学设计，适合长时间办公使用。",
			ShortDescription: "人体工学，静音按键，持久续航",
			Category:         "电脑配件",
			Tags:             []string{"办公", "静音"},
			Price:            89.00,
			OriginalPrice:    129.00,
			Stock:            150,
			Images: []models.ProductImage{
				{URL: "https://picsum.photos/seed/mouse1/800/800", Alt: "鼠标", IsPrimary: true},
			},
			Attributes: []models.ProductAttribute{
				{Name: "颜色", Values: []string{"黑色", "银色"}},
			},
			Status:     models.ProductStatusActive,
			SortOrder:  40,
			IsFeatured: false,
		},
		{
			SKU:              "DEMO-008",
			Name:             "USB-C多功能扩展坞",
			Description:      "一个接口，扩展所有可能。支持4K显示输出、USB 3.0、网口等多种接口。",
			ShortDescription: "多接口扩展，支持4K显示",
			Category:         "电子配件",
			Tags:             []string{"实用", "扩展坞"},
			Price:            199.00,
			OriginalPrice:    0,
			Stock:            100,
			Images: []models.ProductImage{
				{URL: "https://picsum.photos/seed/hub1/800/800", Alt: "扩展坞", IsPrimary: true},
			},
			Status:     models.ProductStatusActive,
			SortOrder:  30,
			IsFeatured: false,
		},
	}

	// 插入Product
	for i, product := range products {
		// Check if SKU already exists
		var existing models.Product
		result := db.Where("sku = ?", product.SKU).First(&existing)
		if result.Error == nil {
			log.Printf("[%d/%d] Product %s already exists, skipping", i+1, len(products), product.SKU)
			continue
		}

		// Create product
		if err := db.Create(&product).Error; err != nil {
			log.Printf("[%d/%d] Failed to create product %s: %v", i+1, len(products), product.SKU, err)
		} else {
			log.Printf("[%d/%d] ✓ Successfully created product: %s - %s", i+1, len(products), product.SKU, product.Name)
		}

		// Add small delay
		time.Sleep(100 * time.Millisecond)
	}

	fmt.Println("\n===========================================")
	fmt.Println("Sample Products Initialized Successfully!")
	fmt.Println("===========================================")
	fmt.Printf("Total products created: %d\n", len(products))
	fmt.Println("\nNext steps:")
	fmt.Println("1. Visit admin panel to view product list")
	fmt.Println("2. Visit user portal to browse products")
	fmt.Println("3. Modify or delete sample products as needed")
	fmt.Println("===========================================")
}
