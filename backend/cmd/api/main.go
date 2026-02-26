package main

import (
	"fmt"
	"log"

	"auralogic/internal/config"
	"auralogic/internal/database"
	adminHandler "auralogic/internal/handler/admin"
	"auralogic/internal/pkg/cache"
	"auralogic/internal/pkg/jwt"
	"auralogic/internal/repository"
	"auralogic/internal/router"
	"auralogic/internal/service"
)

// GitCommit is set at compile time via:
//
//	go build -ldflags "-X main.GitCommit=$(git rev-parse --short HEAD)" ./cmd/api
var GitCommit = ""

func main() {
	if GitCommit == "" {
		GitCommit = "dev"
	}
	log.Printf("Starting AuraLogic API Server (version: %s)...", GitCommit)

	// 加载配置
	cfg, err := config.LoadConfig(config.GetConfigPath())
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	log.Printf("Config loaded from: %s", config.GetConfigPath())

	// 初始化数据库
	if err := database.InitDatabase(&cfg.Database); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// 注入默认落地页 HTML（在 AutoMigrate 之前）
	database.SetDefaultLandingPageHTML(adminHandler.DefaultLandingPageHTML)

	// 自动迁移数据库
	if err := database.AutoMigrate(); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}
	log.Println("Database migrated successfully")

	// 初始化Redis
	if err := cache.InitRedis(&cfg.Redis); err != nil {
		log.Fatalf("Failed to initialize redis: %v", err)
	}
	defer cache.Close()

	// 初始化JWT
	jwt.InitJWT(&cfg.JWT)

	// 初始化Repository
	db := database.GetDB()
	userRepo := repository.NewUserRepository(db)
	orderRepo := repository.NewOrderRepository(db)
	productRepo := repository.NewProductRepository(db)
	inventoryRepo := repository.NewInventoryRepository(db)
	bindingRepo := repository.NewBindingRepository(db)
	serialRepo := repository.NewSerialRepository(db)
	promoCodeRepo := repository.NewPromoCodeRepository(db)

	// 初始化Service
	authService := service.NewAuthService(userRepo, cfg)
	emailService := service.NewEmailService(db, &cfg.SMTP, cfg.App.URL)
	bindingService := service.NewBindingService(bindingRepo, inventoryRepo, productRepo)
	serialService := service.NewSerialService(serialRepo, productRepo, orderRepo)
	virtualInventoryService := service.NewVirtualInventoryService(db)
	orderService := service.NewOrderService(orderRepo, userRepo, productRepo, inventoryRepo, bindingService, serialService, virtualInventoryService, promoCodeRepo, cfg, emailService)
	productService := service.NewProductService(productRepo, inventoryRepo)
	productService.SetUploadConfig(cfg.Upload.Dir, cfg.App.URL)

	// 启动邮件队列处理（如果启用）
	if cfg.SMTP.Enabled {
		go emailService.ProcessEmailQueue()
		log.Println("Email service started")
	}

	// 初始化内置付款方式
	paymentMethodService := service.NewPaymentMethodService(db)
	if err := paymentMethodService.InitBuiltinPaymentMethods(); err != nil {
		log.Printf("Warning: Failed to initialize builtin payment methods: %v", err)
	}

	// 启动付款状态轮询服务
	paymentPollingService := service.NewPaymentPollingService(db, virtualInventoryService, emailService)
	paymentPollingService.Start()
	defer paymentPollingService.Stop()

	// 启动订单自动取消服务
	orderCancelService := service.NewOrderCancelService(db, cfg, inventoryRepo, promoCodeRepo, virtualInventoryService, serialService)
	orderCancelService.Start()
	defer orderCancelService.Stop()
	log.Println("Order auto-cancel service started")

	// 启动工单附件自动清理服务
	ticketAttachmentCleanupService := service.NewTicketAttachmentCleanupService(db, cfg)
	ticketAttachmentCleanupService.Start()
	defer ticketAttachmentCleanupService.Stop()
	log.Println("Ticket attachment cleanup service started")

	// 启动工单超时自动关闭服务
	ticketAutoCloseService := service.NewTicketAutoCloseService(db, cfg)
	ticketAutoCloseService.Start()
	defer ticketAutoCloseService.Stop()
	log.Println("Ticket auto-close service started")

	// 设置路由
	r := router.SetupRouter(cfg, authService, orderService, productService, emailService, userRepo, db, paymentPollingService, GitCommit)

	// 启动服务器
	addr := fmt.Sprintf(":%d", cfg.App.Port)
	log.Printf("Server is running on %s", addr)
	log.Printf("Environment: %s", cfg.App.Env)

	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
