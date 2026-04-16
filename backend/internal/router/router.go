package router

import (
	"auralogic/internal/config"
	adminHandler "auralogic/internal/handler/admin"
	formHandler "auralogic/internal/handler/form"
	userHandler "auralogic/internal/handler/user"
	"auralogic/internal/middleware"
	"auralogic/internal/pluginobs"
	"auralogic/internal/repository"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SetupRouter 设置路由
func SetupRouter(
	cfg *config.Config,
	authService *service.AuthService,
	orderService *service.OrderService,
	productService *service.ProductService,
	emailService *service.EmailService,
	userRepo *repository.UserRepository,
	db *gorm.DB,
	paymentPollingService *service.PaymentPollingService,
	pluginManagerService *service.PluginManagerService,
	version string,
) *gin.Engine {
	// 设置Gin模式
	if cfg.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()

	// 全局中间件
	r.Use(gin.Recovery())
	r.Use(middleware.Logger())
	r.Use(middleware.CORS(&cfg.Security.CORS))
	r.Use(middleware.SecurityHeaders()) // 添加安全响应头

	// CreateRepository
	inventoryRepo := repository.NewInventoryRepository(db)
	productRepo := repository.NewProductRepository(db)
	bindingRepo := repository.NewBindingRepository(db)
	serialRepo := repository.NewSerialRepository(db)
	orderRepo := repository.NewOrderRepository(db)
	cartRepo := repository.NewCartRepository(db)
	promoCodeRepo := repository.NewPromoCodeRepository(db)

	// CreateService
	inventoryService := service.NewInventoryService(inventoryRepo, productRepo)
	bindingService := service.NewBindingService(bindingRepo, inventoryRepo, productRepo)
	serialService := service.NewSerialService(serialRepo, productRepo, orderRepo)
	virtualInventoryService := service.NewVirtualInventoryService(db)
	cartService := service.NewCartService(cartRepo, productRepo, bindingService, virtualInventoryService)
	promoCodeService := service.NewPromoCodeService(promoCodeRepo, productRepo)

	// CreateService - SMS
	smsService := service.NewSMSService(cfg, db)
	marketingService := service.NewMarketingService(db, emailService, smsService)
	smsService.SetPluginManager(pluginManagerService)
	marketingService.SetPluginManager(pluginManagerService)
	serialService.SetPluginManager(pluginManagerService)
	if emailService != nil {
		emailService.SetPluginManager(pluginManagerService)
	}

	// CreateHandler
	userAuthHandler := userHandler.NewAuthHandler(authService, emailService, smsService, pluginManagerService)
	userOrderHandler := userHandler.NewOrderHandler(orderService, bindingService, virtualInventoryService, pluginManagerService, cfg)
	userProductHandler := userHandler.NewProductHandler(productService, orderService, bindingService, virtualInventoryService, pluginManagerService)
	formShippingHandler := formHandler.NewShippingHandler(orderService, cfg)
	jsRuntimeService := service.NewJSRuntimeService(db, cfg)
	adminOrderHandler := adminHandler.NewOrderHandler(orderService, serialService, virtualInventoryService, jsRuntimeService, pluginManagerService, cfg)
	adminProductHandler := adminHandler.NewProductHandler(productService, virtualInventoryService, pluginManagerService)
	adminUserHandler := adminHandler.NewUserHandler(userRepo, db, cfg, pluginManagerService)
	adminPermissionHandler := adminHandler.NewPermissionHandler(db, pluginManagerService)
	adminAPIKeyHandler := adminHandler.NewAPIKeyHandler(db, pluginManagerService)
	adminAdminHandler := adminHandler.NewAdminHandler(userRepo, db, cfg)
	adminLogHandler := adminHandler.NewLogHandler(db, pluginManagerService)
	adminDashboardHandler := adminHandler.NewDashboardHandler(db, cfg, version)
	adminAnalyticsHandler := adminHandler.NewAnalyticsHandler(db, cfg)
	adminSettingsHandler := adminHandler.NewSettingsHandler(db, cfg, smsService, emailService, pluginManagerService)
	adminUploadHandler := adminHandler.NewUploadHandler(cfg.Upload.Dir, cfg.App.URL, pluginManagerService)
	adminInventoryHandler := adminHandler.NewInventoryHandler(inventoryService, db, pluginManagerService)
	adminBindingHandler := adminHandler.NewBindingHandler(bindingService, db, pluginManagerService)
	adminInventoryLogHandler := adminHandler.NewInventoryLogHandler(db)
	adminSerialHandler := adminHandler.NewSerialHandler(serialService, pluginManagerService)
	userSerialHandler := userHandler.NewSerialHandler(serialService)
	userCartHandler := userHandler.NewCartHandler(cartService, pluginManagerService)
	adminVirtualInventoryHandler := adminHandler.NewVirtualInventoryHandler(virtualInventoryService, db, pluginManagerService)
	adminPaymentMethodHandler := adminHandler.NewPaymentMethodHandler(db, cfg, pluginManagerService)
	userPaymentMethodHandler := userHandler.NewPaymentMethodHandler(db, paymentPollingService, pluginManagerService, cfg)
	userTicketHandler := userHandler.NewTicketHandler(db, emailService, pluginManagerService)
	adminTicketHandler := adminHandler.NewTicketHandler(db, emailService, pluginManagerService)
	adminPromoCodeHandler := adminHandler.NewPromoCodeHandler(promoCodeService, pluginManagerService, db)
	userPromoCodeHandler := userHandler.NewPromoCodeHandler(promoCodeService, pluginManagerService)
	adminKnowledgeHandler := adminHandler.NewKnowledgeHandler(db, pluginManagerService)
	adminAnnouncementHandler := adminHandler.NewAnnouncementHandler(db, emailService, smsService, pluginManagerService)
	adminMarketingHandler := adminHandler.NewMarketingHandler(db, marketingService, pluginManagerService)
	adminLandingPageHandler := adminHandler.NewLandingPageHandler(db, cfg, pluginManagerService)
	userKnowledgeHandler := userHandler.NewKnowledgeHandler(db, pluginManagerService)
	userAnnouncementHandler := userHandler.NewAnnouncementHandler(db, pluginManagerService)
	adminPluginHandler := adminHandler.NewPluginHandler(db, pluginManagerService, cfg.Plugin.ArtifactDir)

	// ========== 表单API（支持匿名 token 访问，登录态会附带所有权校验） ==========
	form := r.Group("/api/form")
	form.Use(middleware.OptionalAuthMiddleware())
	{
		form.GET("/shipping", formShippingHandler.GetForm)
		form.POST("/shipping", formShippingHandler.SubmitForm)
		form.GET("/countries", formShippingHandler.GetCountries) // get国家列表
	}

	// ========== 序列号查询API（公开，无需登录） ==========
	serialAPI := r.Group("/api/serial")
	serialAPI.Use(middleware.RequireSerialEnabled())
	serialAPI.Use(middleware.RateLimitMiddleware(10, time.Minute)) // 每分钟最多10次，防止暴力枚举
	{
		serialAPI.POST("/verify", userSerialHandler.VerifySerial)
		serialAPI.GET("/:serial_number", userSerialHandler.GetSerialByNumber)
	}

	// ========== 公开配置API（无需登录） ==========
	configAPI := r.Group("/api/config")
	publicPluginMiddlewares := []gin.HandlerFunc{
		func(c *gin.Context) {
			endpoint := strings.TrimSpace(c.FullPath())
			if endpoint == "" {
				endpoint = strings.TrimSpace(c.Request.URL.Path)
			}
			switch endpoint {
			case "/api/config/plugin-extensions", "/api/config/plugin-bootstrap":
				pluginobs.RecordPublicRequest(endpoint)
			}
			c.Next()
		},
		middleware.OptionalAuthMiddleware(),
	}
	publicPluginMiddlewares = append([]gin.HandlerFunc{
		middleware.DynamicRateLimitMiddlewareWithObserver(resolveRateLimit(func(runtimeCfg *config.Config) int {
			return runtimeCfg.RateLimit.UserRequest
		}, 60), time.Minute, func(c *gin.Context, blocked bool, _ int64, _ int, _ time.Duration) {
			endpoint := strings.TrimSpace(c.FullPath())
			if endpoint == "" {
				endpoint = strings.TrimSpace(c.Request.URL.Path)
			}
			switch endpoint {
			case "/api/config/plugin-extensions", "/api/config/plugin-bootstrap":
				pluginobs.RecordPublicRateLimit(endpoint, blocked)
			}
		}),
	}, publicPluginMiddlewares...)
	{
		configAPI.GET("/public", adminSettingsHandler.GetPublicConfig)
		configAPI.GET("/page-inject", adminSettingsHandler.GetPageInject)
		configAPI.GET("/plugin-extensions", append(publicPluginMiddlewares, adminPluginHandler.GetPublicExtensions)...)
		configAPI.POST("/plugin-extensions/batch", append(publicPluginMiddlewares, adminPluginHandler.GetPublicExtensionsBatch)...)
		configAPI.GET("/plugin-bootstrap", append(publicPluginMiddlewares, adminPluginHandler.GetPublicFrontendBootstrap)...)
		configAPI.POST("/plugins/:id/execute", append(publicPluginMiddlewares, adminPluginHandler.ExecutePublicPlugin)...)
		configAPI.POST("/plugins/:id/execute/stream", append(publicPluginMiddlewares, adminPluginHandler.ExecutePublicPluginStream)...)
	}
	pluginPublicAPI := r.Group("/api/plugins")
	{
		pluginPublicAPI.Any("/:name/webhooks/:hook", append(publicPluginMiddlewares, adminPluginHandler.HandlePluginWebhook)...)
	}
	paymentWebhookMiddlewares := []gin.HandlerFunc{
		middleware.DynamicRateLimitMiddleware(resolveRateLimit(func(runtimeCfg *config.Config) int {
			return runtimeCfg.RateLimit.PaymentInfo
		}, 120), time.Minute),
	}
	paymentPublicAPI := r.Group("/api/payment-methods")
	{
		paymentPublicAPI.Any("/:id/webhooks/:hook", append(paymentWebhookMiddlewares, userPaymentMethodHandler.HandleWebhook)...)
	}

	// ========== User端API ==========
	userAPI := r.Group("/api/user")
	userAPI.Use(middleware.DynamicRateLimitMiddleware(resolveRateLimit(func(runtimeCfg *config.Config) int {
		return runtimeCfg.RateLimit.UserRequest
	}, 0), time.Minute))
	{
		// 认证
		auth := userAPI.Group("/auth")
		auth.Use(middleware.DynamicRateLimitMiddleware(resolveRateLimit(func(runtimeCfg *config.Config) int {
			return runtimeCfg.RateLimit.UserLogin
		}, 0), time.Minute))
		{
			auth.POST("/login", userAuthHandler.Login)
			auth.POST("/register", userAuthHandler.Register)
			auth.GET("/captcha", userAuthHandler.GetCaptcha)
			auth.GET("/verify-email", userAuthHandler.VerifyEmail)
			auth.POST("/resend-verification", userAuthHandler.ResendVerification)
			auth.POST("/send-login-code", userAuthHandler.SendLoginCode)
			auth.POST("/login-with-code", userAuthHandler.LoginWithCode)
			auth.POST("/forgot-password", userAuthHandler.ForgotPassword)
			auth.POST("/reset-password", userAuthHandler.ResetPassword)
			auth.POST("/send-phone-code", userAuthHandler.SendPhoneLoginCode)
			auth.POST("/login-with-phone-code", userAuthHandler.LoginWithPhoneCode)
			auth.POST("/send-phone-register-code", userAuthHandler.SendPhoneRegisterCode)
			auth.POST("/phone-register", userAuthHandler.PhoneRegister)
			auth.POST("/phone-forgot-password", userAuthHandler.PhoneForgotPassword)
			auth.POST("/phone-reset-password", userAuthHandler.PhoneResetPassword)
			auth.POST("/logout", middleware.AuthMiddleware(), userAuthHandler.Logout)
			auth.GET("/me", middleware.AuthMiddleware(), userAuthHandler.GetMe)
			auth.POST("/change-password", middleware.AuthMiddleware(), userAuthHandler.ChangePassword)
			auth.PUT("/preferences", middleware.AuthMiddleware(), userAuthHandler.UpdatePreferences)
			auth.POST("/send-bind-email-code", middleware.AuthMiddleware(), userAuthHandler.SendBindEmailCode)
			auth.POST("/bind-email", middleware.AuthMiddleware(), userAuthHandler.BindEmail)
			auth.POST("/send-bind-phone-code", middleware.AuthMiddleware(), userAuthHandler.SendBindPhoneCode)
			auth.POST("/bind-phone", middleware.AuthMiddleware(), userAuthHandler.BindPhone)
		}

		// Order
		orders := userAPI.Group("/orders")
		orders.Use(middleware.AuthMiddleware())
		{
			orders.POST("", middleware.DynamicRateLimitMiddleware(resolveRateLimit(func(runtimeCfg *config.Config) int {
				return runtimeCfg.RateLimit.OrderCreate
			}, 30), time.Minute), userOrderHandler.CreateOrder)
			orders.GET("", userOrderHandler.ListOrders)
			orders.GET("/:order_no", userOrderHandler.GetOrder)
			orders.GET("/:order_no/form-token", userOrderHandler.GetOrRefreshFormToken)
			orders.GET("/:order_no/virtual-products", userOrderHandler.GetVirtualProducts)
			orders.POST("/:order_no/complete", userOrderHandler.CompleteOrder)
			orders.GET("/:order_no/invoice", userOrderHandler.DownloadInvoice)
			orders.GET("/:order_no/invoice-token", userOrderHandler.GetInvoiceToken)
		}

		// 账单公开访问（通过一次性令牌认证）
		userAPI.GET("/invoice/:token", userOrderHandler.ViewInvoiceByToken)

		// Product（推荐商品公开访问；列表/详情按配置动态控制是否需要登录）
		productsPublic := userAPI.Group("/products")
		{
			productsPublic.GET("/featured", userProductHandler.GetFeaturedProducts)
			productsPublic.GET("/recommended", userProductHandler.GetRecommendedProducts)
		}

		products := userAPI.Group("/products")
		products.Use(middleware.ProductBrowseAuthMiddleware(cfg))
		{
			products.GET("", userProductHandler.ListProducts)
			products.GET("/categories", userProductHandler.GetCategories)
			products.GET("/:id", userProductHandler.GetProduct)
			products.GET("/:id/available-stock", userProductHandler.GetProductAvailableStock)
		}

		// 购物车
		cart := userAPI.Group("/cart")
		cart.Use(middleware.AuthMiddleware())
		{
			cart.GET("", userCartHandler.GetCart)
			cart.GET("/count", userCartHandler.GetCartCount)
			cart.POST("/items", userCartHandler.AddToCart)
			cart.PUT("/items/:id", userCartHandler.UpdateQuantity)
			cart.DELETE("/items/:id", userCartHandler.RemoveFromCart)
			cart.DELETE("", userCartHandler.ClearCart)
		}

		// 优惠码验证
		promoCodes := userAPI.Group("/promo-codes")
		promoCodes.Use(middleware.AuthMiddleware())
		{
			promoCodes.POST("/validate", userPromoCodeHandler.ValidatePromoCode)
		}

		// 付款方式（需要登录）
		payment := userAPI.Group("/payment-methods")
		payment.Use(middleware.AuthMiddleware())
		{
			payment.GET("", userPaymentMethodHandler.List)
		}
		paymentAuth := userAPI.Group("/orders")
		paymentAuth.Use(middleware.AuthMiddleware())
		{
			paymentAuth.GET("/:order_no/payment-info", middleware.DynamicRateLimitMiddleware(resolveRateLimit(func(runtimeCfg *config.Config) int {
				return runtimeCfg.RateLimit.PaymentInfo
			}, 120), time.Minute), userPaymentMethodHandler.GetOrderPaymentInfo)
			paymentAuth.GET("/:order_no/payment-card", middleware.DynamicRateLimitMiddleware(resolveRateLimit(func(runtimeCfg *config.Config) int {
				return runtimeCfg.RateLimit.PaymentInfo
			}, 120), time.Minute), userPaymentMethodHandler.GetPaymentCard)
			paymentAuth.POST("/:order_no/select-payment", middleware.DynamicRateLimitMiddleware(resolveRateLimit(func(runtimeCfg *config.Config) int {
				return runtimeCfg.RateLimit.PaymentSelect
			}, 60), time.Minute), userPaymentMethodHandler.SelectPaymentMethod)
		}

		// 工单/客服中心
		tickets := userAPI.Group("/tickets")
		tickets.Use(middleware.AuthMiddleware(), middleware.RequireTicketEnabled())
		{
			tickets.POST("", userTicketHandler.CreateTicket)
			tickets.GET("", userTicketHandler.ListTickets)
			tickets.GET("/:id", userTicketHandler.GetTicket)
			tickets.GET("/:id/messages", userTicketHandler.GetTicketMessages)
			tickets.POST("/:id/messages", userTicketHandler.SendMessage)
			tickets.PUT("/:id/status", userTicketHandler.UpdateTicketStatus)
			tickets.POST("/:id/share-order", userTicketHandler.ShareOrder)
			tickets.GET("/:id/shared-orders", userTicketHandler.GetSharedOrders)
			tickets.DELETE("/:id/shared-orders/:orderId", userTicketHandler.RevokeOrderAccess)
			tickets.POST("/:id/upload", userTicketHandler.UploadFile)
		}

		// 知识库
		knowledge := userAPI.Group("/knowledge")
		knowledge.Use(middleware.AuthMiddleware())
		{
			knowledge.GET("/categories", userKnowledgeHandler.GetCategoryTree)
			knowledge.GET("/articles", userKnowledgeHandler.ListArticles)
			knowledge.GET("/articles/:id", userKnowledgeHandler.GetArticle)
		}

		// 公告
		announcements := userAPI.Group("/announcements")
		announcements.Use(middleware.AuthMiddleware())
		{
			announcements.GET("", userAnnouncementHandler.ListAnnouncements)
			announcements.GET("/unread-mandatory", userAnnouncementHandler.GetUnreadMandatory)
			announcements.GET("/:id", userAnnouncementHandler.GetAnnouncement)
			announcements.POST("/:id/read", userAnnouncementHandler.MarkAsRead)
		}
	}

	// ========== AdminAPI ==========
	adminAPI := r.Group("/api/admin")
	adminAPI.Use(middleware.DynamicRateLimitMiddleware(resolveRateLimit(func(runtimeCfg *config.Config) int {
		return runtimeCfg.RateLimit.AdminRequest
	}, 0), time.Minute))
	{
		adminAPI.GET("/plugin-bootstrap", middleware.AuthMiddleware(), middleware.RequireAdmin(), middleware.RequirePermission("plugin.view"), adminPluginHandler.GetAdminFrontendBootstrap)
		adminAPI.GET("/plugin-extensions", middleware.AuthMiddleware(), middleware.RequireAdmin(), middleware.RequirePermission("plugin.view"), adminPluginHandler.GetAdminExtensions)
		adminAPI.POST("/plugin-extensions/batch", middleware.AuthMiddleware(), middleware.RequireAdmin(), middleware.RequirePermission("plugin.view"), adminPluginHandler.GetAdminExtensionsBatch)

		// 仪表盘（仅超级管理员）
		dashboard := adminAPI.Group("/dashboard")
		dashboard.Use(middleware.AuthMiddleware(), middleware.RequireSuperAdmin())
		{
			dashboard.GET("/statistics", adminDashboardHandler.GetStatistics)
			dashboard.GET("/activities", adminDashboardHandler.GetRecentActivities)
		}

		// 数据分析（仅超级管理员）
		analytics := adminAPI.Group("/analytics")
		analytics.Use(middleware.AuthMiddleware(), middleware.RequireSuperAdmin())
		{
			analytics.GET("/users", adminAnalyticsHandler.GetUserAnalytics)
			analytics.GET("/orders", adminAnalyticsHandler.GetOrderAnalytics)
			analytics.GET("/revenue", adminAnalyticsHandler.GetRevenueAnalytics)
			analytics.GET("/devices", adminAnalyticsHandler.GetDeviceAnalytics)
			analytics.GET("/pageviews", adminAnalyticsHandler.GetPageViewAnalytics)
		}

		// Order管理（needAdminPermission）
		orders := adminAPI.Group("/orders")
		orders.Use(middleware.AuthMiddleware(), middleware.RequireAdmin())
		{
			orders.GET("", middleware.RequirePermission("order.view"), adminOrderHandler.ListOrders)
			orders.GET("/countries", middleware.RequirePermission("order.view"), adminOrderHandler.GetOrderCountries)
			orders.GET("/:id", middleware.RequirePermission("order.view"), adminOrderHandler.GetOrder)
			orders.POST("/draft", middleware.RequirePermission("order.edit"), adminOrderHandler.CreateDraft)
			orders.POST("", middleware.RequirePermission("order.edit"), adminOrderHandler.CreateOrderForUser)
			orders.POST("/:id/assign-shipping", middleware.RequirePermission("order.assign_tracking"), adminOrderHandler.AssignTracking)
			orders.PUT("/:id/shipping-info", middleware.RequirePermission("order.edit"), adminOrderHandler.UpdateShippingInfo)
			orders.POST("/:id/request-resubmit", middleware.RequirePermission("order.edit"), adminOrderHandler.RequestResubmit)
			orders.POST("/:id/complete", middleware.RequirePermission("order.status_update"), adminOrderHandler.CompleteOrder)
			orders.POST("/:id/cancel", middleware.RequirePermission("order.status_update"), adminOrderHandler.CancelOrder)
			orders.POST("/:id/refund", middleware.RequirePermission("order.refund"), adminOrderHandler.RefundOrder)
			orders.POST("/:id/confirm-refund", middleware.RequirePermission("order.refund"), adminOrderHandler.ConfirmRefund)
			orders.POST("/:id/mark-paid", middleware.RequirePermission("order.status_update"), adminOrderHandler.MarkAsPaid)
			orders.POST("/:id/deliver-virtual", middleware.RequirePermission("order.status_update"), adminOrderHandler.DeliverVirtualStock)
			orders.PUT("/:id/price", middleware.RequirePermission("order.edit"), adminOrderHandler.UpdateOrderPrice)
			orders.DELETE("/:id", middleware.RequirePermission("order.delete"), adminOrderHandler.DeleteOrder)

			// 批量操作
			orders.POST("/batch/complete-shipped", middleware.RequirePermission("order.status_update"), adminOrderHandler.CompleteAllShippedOrders)
			orders.POST("/batch/update", middleware.RequirePermission("order.status_update"), adminOrderHandler.BatchUpdateOrders)

			// Excel导出导入
			orders.GET("/export", middleware.RequirePermission("order.view"), adminOrderHandler.ExportOrders)
			orders.POST("/import", middleware.RequirePermission("order.assign_tracking"), adminOrderHandler.ImportOrders)
			orders.GET("/import-template", middleware.RequirePermission("order.view"), adminOrderHandler.DownloadTemplate)
		}

		// User管理
		users := adminAPI.Group("/users")
		users.Use(middleware.AuthMiddleware(), middleware.RequireAdmin())
		{
			users.GET("", middleware.RequirePermission("user.view"), adminUserHandler.ListUsers)
			users.GET("/export", middleware.RequirePermission("user.view"), adminUserHandler.ExportUsers)
			users.GET("/countries", middleware.RequirePermission("user.view"), adminUserHandler.ListUserCountries)
			users.POST("", middleware.RequirePermission("user.edit"), adminUserHandler.CreateUser)
			users.GET("/:id", middleware.RequirePermission("user.view"), adminUserHandler.GetUser)
			users.PUT("/:id", middleware.RequirePermission("user.edit"), adminUserHandler.UpdateUser)
			users.DELETE("/:id", middleware.RequirePermission("user.edit"), adminUserHandler.DeleteUser)
			users.GET("/:id/orders", middleware.RequirePermission("user.view"), adminUserHandler.GetUserOrders)
		}

		// Product管理
		products := adminAPI.Group("/products")
		products.Use(middleware.AuthMiddleware(), middleware.RequireAdmin())
		{
			products.GET("", middleware.RequirePermission("product.view"), adminProductHandler.ListProducts)
			products.GET("/export", middleware.RequirePermission("product.view"), adminProductHandler.ExportProducts)
			products.GET("/import-template", middleware.RequirePermission("product.edit"), adminProductHandler.DownloadProductImportTemplate)
			products.POST("/import", middleware.RequirePermission("product.edit"), adminProductHandler.ImportProducts)
			products.POST("", middleware.RequirePermission("product.edit"), adminProductHandler.CreateProduct)
			products.GET("/categories", middleware.RequirePermission("product.view"), adminProductHandler.GetCategories)
			products.GET("/:id", middleware.RequirePermission("product.view"), adminProductHandler.GetProduct)
			products.PUT("/:id", middleware.RequirePermission("product.edit"), adminProductHandler.UpdateProduct)
			products.DELETE("/:id", middleware.RequirePermission("product.delete"), adminProductHandler.DeleteProduct)
			products.PUT("/:id/status", middleware.RequirePermission("product.edit"), adminProductHandler.UpdateProductStatus)
			products.PUT("/:id/stock", middleware.RequirePermission("product.edit"), adminProductHandler.UpdateStock)
			products.POST("/:id/toggle-featured", middleware.RequirePermission("product.edit"), adminProductHandler.ToggleFeatured)
			products.PUT("/:id/inventory-mode", middleware.RequirePermission("product.edit"), adminProductHandler.UpdateInventoryMode)

			// Product-Inventory绑定管理
			products.GET("/:id/inventory-bindings", middleware.RequirePermission("product.view"), adminBindingHandler.GetProductBindings)
			products.POST("/:id/inventory-bindings", middleware.RequirePermission("product.edit"), adminBindingHandler.CreateBinding)
			products.POST("/:id/inventory-bindings/batch", middleware.RequirePermission("product.edit"), adminBindingHandler.BatchCreateBindings)
			products.PUT("/:id/inventory-bindings/:bindingId", middleware.RequirePermission("product.edit"), adminBindingHandler.UpdateBinding)
			products.DELETE("/:id/inventory-bindings/:bindingId", middleware.RequirePermission("product.edit"), adminBindingHandler.DeleteBinding)
			products.DELETE("/:id/inventory-bindings", middleware.RequirePermission("product.edit"), adminBindingHandler.DeleteAllProductBindings)
			products.PUT("/:id/inventory-bindings/replace", middleware.RequirePermission("product.edit"), adminBindingHandler.ReplaceProductBindings)
		}

		// Inventory管理
		inventories := adminAPI.Group("/inventories")
		inventories.Use(middleware.AuthMiddleware(), middleware.RequireAdmin())
		{
			inventories.GET("", middleware.RequirePermission("product.view"), adminInventoryHandler.ListInventories)
			inventories.POST("", middleware.RequirePermission("product.edit"), adminInventoryHandler.CreateInventory)
			inventories.GET("/low-stock", middleware.RequirePermission("product.view"), adminInventoryHandler.GetLowStockList)
			inventories.GET("/:id", middleware.RequirePermission("product.view"), adminInventoryHandler.GetInventory)
			inventories.PUT("/:id", middleware.RequirePermission("product.edit"), adminInventoryHandler.UpdateInventory)
			inventories.POST("/:id/adjust", middleware.RequirePermission("product.edit"), adminInventoryHandler.AdjustStock)
			inventories.DELETE("/:id", middleware.RequirePermission("product.delete"), adminInventoryHandler.DeleteInventory)

			// getInventory绑定的所有Product
			inventories.GET("/:id/products", middleware.RequirePermission("product.view"), adminBindingHandler.GetInventoryProducts)
		}

		// Permission管理（仅超级Admin）
		permissions := adminAPI.Group("/permissions")
		permissions.Use(middleware.AuthMiddleware(), middleware.RequireSuperAdmin())
		{
			permissions.GET("/all", adminPermissionHandler.ListAllPermissions)
			permissions.GET("/users/:id", adminPermissionHandler.GetUserPermissions)
			permissions.PUT("/users/:id", middleware.RequirePermission("admin.permission"), adminPermissionHandler.UpdateUserPermissions)
		}

		// API密钥管理
		apiKeys := adminAPI.Group("/api-keys")
		apiKeys.Use(middleware.AuthMiddleware(), middleware.RequireAdmin())
		{
			apiKeys.GET("", middleware.RequirePermission("api.manage"), adminAPIKeyHandler.ListAPIKeys)
			apiKeys.POST("", middleware.RequirePermission("api.manage"), adminAPIKeyHandler.CreateAPIKey)
			apiKeys.PUT("/:id", middleware.RequirePermission("api.manage"), adminAPIKeyHandler.UpdateAPIKey)
			apiKeys.DELETE("/:id", middleware.RequirePermission("api.manage"), adminAPIKeyHandler.DeleteAPIKey)
		}

		// Admin管理（仅超级Admin）
		admins := adminAPI.Group("/admins")
		admins.Use(middleware.AuthMiddleware(), middleware.RequireSuperAdmin())
		{
			admins.GET("", adminAdminHandler.ListAdmins)
			admins.GET("/:id", adminAdminHandler.GetAdmin)
			admins.POST("", middleware.RequirePermission("admin.create"), adminAdminHandler.CreateAdmin)
			admins.PUT("/:id", middleware.RequirePermission("admin.edit"), adminAdminHandler.UpdateAdmin)
			admins.DELETE("/:id", middleware.RequirePermission("admin.delete"), adminAdminHandler.DeleteAdmin)
		}

		// 日志管理
		logs := adminAPI.Group("/logs")
		logs.Use(middleware.AuthMiddleware(), middleware.RequireAdmin())
		{
			logs.GET("/operations", middleware.RequirePermission("system.logs"), adminLogHandler.ListOperationLogs)
			logs.GET("/operations/export", middleware.RequirePermission("system.logs"), adminLogHandler.ExportOperationLogs)
			logs.GET("/emails", middleware.RequirePermission("system.logs"), adminLogHandler.ListEmailLogs)
			logs.GET("/emails/export", middleware.RequirePermission("system.logs"), adminLogHandler.ExportEmailLogs)
			logs.GET("/sms", middleware.RequirePermission("system.logs"), adminLogHandler.ListSmsLogs)
			logs.GET("/sms/export", middleware.RequirePermission("system.logs"), adminLogHandler.ExportSmsLogs)
			logs.GET("/statistics", middleware.RequirePermission("system.logs"), adminLogHandler.GetLogStatistics)
			logs.POST("/emails/retry", middleware.RequirePermission("system.logs"), adminLogHandler.RetryFailedEmails)
			logs.GET("/inventories", middleware.RequirePermission("system.logs"), adminInventoryLogHandler.ListInventoryLogs)
			logs.GET("/inventories/export", middleware.RequirePermission("system.logs"), adminInventoryLogHandler.ExportInventoryLogs)
			logs.GET("/inventories/statistics", middleware.RequirePermission("system.logs"), adminInventoryLogHandler.GetInventoryLogStatistics)
		}

		// 系统设置（仅超级Admin）
		settings := adminAPI.Group("/settings")
		settings.Use(middleware.AuthMiddleware(), middleware.RequireSuperAdmin())
		{
			settings.GET("", middleware.RequirePermission("system.config"), adminSettingsHandler.GetSettings)
			settings.PUT("", middleware.RequirePermission("system.config"), adminSettingsHandler.UpdateSettings)
			settings.POST("/smtp/test", middleware.RequirePermission("system.config"), adminSettingsHandler.TestSMTP)
			settings.POST("/sms/test", middleware.RequirePermission("system.config"), adminSettingsHandler.TestSMS)
			settings.GET("/email-templates", middleware.RequirePermission("system.config"), adminSettingsHandler.ListEmailTemplates)
			settings.GET("/email-templates/:filename", middleware.RequirePermission("system.config"), adminSettingsHandler.GetEmailTemplate)
			settings.PUT("/email-templates/:filename", middleware.RequirePermission("system.config"), adminSettingsHandler.UpdateEmailTemplate)
			settings.POST("/template-packages/import", middleware.RequirePermission("system.config"), adminSettingsHandler.ImportTemplatePackage)
			settings.GET("/landing-page", middleware.RequirePermission("system.config"), adminLandingPageHandler.GetLandingPage)
			settings.PUT("/landing-page", middleware.RequirePermission("system.config"), adminLandingPageHandler.UpdateLandingPage)
			settings.POST("/landing-page/reset", middleware.RequirePermission("system.config"), adminLandingPageHandler.ResetLandingPage)
		}

		// 付款方式管理
		paymentMethods := adminAPI.Group("/payment-methods")
		paymentMethods.Use(middleware.AuthMiddleware(), middleware.RequireAdmin())
		{
			paymentMethods.GET("", middleware.RequireAnyPermission("payment_method.view", "system.config"), adminPaymentMethodHandler.List)
			paymentMethods.POST("", middleware.RequirePermission("system.config"), adminPaymentMethodHandler.Create)
			paymentMethods.GET("/market/sources", middleware.RequirePermission("system.config"), adminPaymentMethodHandler.ListMarketSources)
			paymentMethods.GET("/market/catalog", middleware.RequirePermission("system.config"), adminPaymentMethodHandler.ListMarketCatalog)
			paymentMethods.GET("/market/artifacts/:name", middleware.RequirePermission("system.config"), adminPaymentMethodHandler.GetMarketArtifact)
			paymentMethods.POST("/market/preview", middleware.RequirePermission("system.config"), adminPaymentMethodHandler.PreviewMarketPackage)
			paymentMethods.POST("/market/import", middleware.RequirePermission("system.config"), adminPaymentMethodHandler.ImportPackageFromMarket)
			paymentMethods.POST("/preview-package", middleware.RequirePermission("system.config"), adminPaymentMethodHandler.PreviewPackage)
			paymentMethods.POST("/upload-package", middleware.RequirePermission("system.config"), adminPaymentMethodHandler.UploadPackage)
			paymentMethods.GET("/:id", middleware.RequireAnyPermission("payment_method.view", "system.config"), adminPaymentMethodHandler.Get)
			paymentMethods.PUT("/:id", middleware.RequirePermission("system.config"), adminPaymentMethodHandler.Update)
			paymentMethods.DELETE("/:id", middleware.RequirePermission("system.config"), adminPaymentMethodHandler.Delete)
			paymentMethods.POST("/:id/toggle", middleware.RequirePermission("system.config"), adminPaymentMethodHandler.ToggleEnabled)
			paymentMethods.POST("/reorder", middleware.RequirePermission("system.config"), adminPaymentMethodHandler.Reorder)
			paymentMethods.POST("/test-script", middleware.RequirePermission("system.config"), adminPaymentMethodHandler.TestScript)
			paymentMethods.POST("/init-builtin", middleware.RequirePermission("system.config"), adminPaymentMethodHandler.InitBuiltinMethods)
		}

		// 优惠码管理
		promoCodesAdmin := adminAPI.Group("/promo-codes")
		promoCodesAdmin.Use(middleware.AuthMiddleware(), middleware.RequireAdmin())
		{
			promoCodesAdmin.GET("", middleware.RequirePermission("product.view"), adminPromoCodeHandler.ListPromoCodes)
			promoCodesAdmin.GET("/export", middleware.RequirePermission("product.view"), adminPromoCodeHandler.ExportPromoCodes)
			promoCodesAdmin.POST("/import", middleware.RequirePermission("product.edit"), adminPromoCodeHandler.ImportPromoCodes)
			promoCodesAdmin.POST("", middleware.RequirePermission("product.edit"), adminPromoCodeHandler.CreatePromoCode)
			promoCodesAdmin.GET("/:id", middleware.RequirePermission("product.view"), adminPromoCodeHandler.GetPromoCode)
			promoCodesAdmin.PUT("/:id", middleware.RequirePermission("product.edit"), adminPromoCodeHandler.UpdatePromoCode)
			promoCodesAdmin.DELETE("/:id", middleware.RequirePermission("product.delete"), adminPromoCodeHandler.DeletePromoCode)
		}

		// 序列号管理
		serials := adminAPI.Group("/serials")
		serials.Use(middleware.AuthMiddleware(), middleware.RequireAdmin())
		{
			serials.GET("", middleware.RequirePermission("serial.view"), adminSerialHandler.ListSerials)
			serials.GET("/statistics", middleware.RequirePermission("serial.view"), adminSerialHandler.GetStatistics)
			serials.GET("/:serial_number", middleware.RequirePermission("serial.view"), adminSerialHandler.GetSerialByNumber)
			serials.GET("/order/:order_id", middleware.RequirePermission("serial.view"), adminSerialHandler.GetSerialsByOrder)
			serials.GET("/product/:product_id", middleware.RequirePermission("serial.view"), adminSerialHandler.GetSerialsByProduct)
			serials.DELETE("/:id", middleware.RequirePermission("serial.manage"), adminSerialHandler.DeleteSerial)
			serials.POST("/batch-delete", middleware.RequirePermission("serial.manage"), adminSerialHandler.BatchDeleteSerials)
		}

		// 虚拟库存管理（新版API，类似实体库存）
		virtualInventories := adminAPI.Group("/virtual-inventories")
		virtualInventories.Use(middleware.AuthMiddleware(), middleware.RequireAdmin())
		{
			// 虚拟库存CRUD
			virtualInventories.GET("", middleware.RequirePermission("product.view"), adminVirtualInventoryHandler.ListVirtualInventories)
			virtualInventories.POST("", middleware.RequirePermission("product.edit"), adminVirtualInventoryHandler.CreateVirtualInventory)
			virtualInventories.GET("/:id", middleware.RequirePermission("product.view"), adminVirtualInventoryHandler.GetVirtualInventory)
			virtualInventories.PUT("/:id", middleware.RequirePermission("product.edit"), adminVirtualInventoryHandler.UpdateVirtualInventory)
			virtualInventories.DELETE("/:id", middleware.RequirePermission("product.delete"), adminVirtualInventoryHandler.DeleteVirtualInventory)

			// 脚本测试
			virtualInventories.POST("/test-script", middleware.RequirePermission("product.edit"), adminVirtualInventoryHandler.TestDeliveryScript)

			// 库存项管理
			virtualInventories.POST("/:id/import", middleware.RequirePermission("product.edit"), adminVirtualInventoryHandler.ImportStock)
			virtualInventories.POST("/:id/stocks", middleware.RequirePermission("product.edit"), adminVirtualInventoryHandler.CreateStockManually)
			virtualInventories.GET("/:id/stocks", middleware.RequirePermission("product.view"), adminVirtualInventoryHandler.GetStockList)
			virtualInventories.GET("/:id/stats", middleware.RequirePermission("product.view"), adminVirtualInventoryHandler.GetStockStats)
			virtualInventories.DELETE("/:id/stocks/:stock_id", middleware.RequirePermission("product.edit"), adminVirtualInventoryHandler.DeleteStock)
			virtualInventories.POST("/:id/stocks/:stock_id/reserve", middleware.RequirePermission("product.edit"), adminVirtualInventoryHandler.ReserveStock)
			virtualInventories.POST("/:id/stocks/:stock_id/release", middleware.RequirePermission("product.edit"), adminVirtualInventoryHandler.ReleaseStockItem)
			virtualInventories.DELETE("/batch", middleware.RequirePermission("product.edit"), adminVirtualInventoryHandler.DeleteBatch)

			// 获取虚拟库存绑定的商品
			virtualInventories.GET("/:id/products", middleware.RequirePermission("product.view"), adminVirtualInventoryHandler.GetInventoryProducts)
		}

		// 商品-虚拟库存绑定管理
		products.GET("/:id/virtual-inventory-bindings", middleware.RequirePermission("product.view"), adminVirtualInventoryHandler.GetProductBindings)
		products.POST("/:id/virtual-inventory-bindings", middleware.RequirePermission("product.edit"), adminVirtualInventoryHandler.CreateBinding)
		products.PUT("/:id/virtual-inventory-bindings", middleware.RequirePermission("product.edit"), adminVirtualInventoryHandler.SaveVariantBindings)
		products.PUT("/:id/virtual-inventory-bindings/:bindingId", middleware.RequirePermission("product.edit"), adminVirtualInventoryHandler.UpdateBinding)
		products.DELETE("/:id/virtual-inventory-bindings/:bindingId", middleware.RequirePermission("product.edit"), adminVirtualInventoryHandler.DeleteBinding)

		// 基于产品ID的虚拟库存管理（兼容前端API）
		virtualProducts := adminAPI.Group("/virtual-products")
		virtualProducts.Use(middleware.AuthMiddleware(), middleware.RequireAdmin())
		{
			virtualProducts.GET("/:id/stocks", middleware.RequirePermission("product.view"), adminVirtualInventoryHandler.GetStockListForProduct)
			virtualProducts.GET("/:id/stats", middleware.RequirePermission("product.view"), adminVirtualInventoryHandler.GetStockStatsForProduct)
			virtualProducts.POST("/:id/import", middleware.RequirePermission("product.edit"), adminVirtualInventoryHandler.ImportStockForProduct)
			virtualProducts.DELETE("/stocks/:id", middleware.RequirePermission("product.edit"), adminVirtualInventoryHandler.DeleteStockByID)
			virtualProducts.DELETE("/batch", middleware.RequirePermission("product.edit"), adminVirtualInventoryHandler.DeleteBatch)
		}

		// 文件上传（needAdminPermission）
		upload := adminAPI.Group("/upload")
		upload.Use(middleware.AuthMiddleware(), middleware.RequireAdmin())
		{
			upload.POST("/image", middleware.RequirePermission("product.edit"), adminUploadHandler.UploadImage)
			upload.POST("/image/delete", middleware.RequirePermission("product.edit"), adminUploadHandler.DeleteImage)
		}

		// 工单管理
		tickets := adminAPI.Group("/tickets")
		tickets.Use(middleware.AuthMiddleware(), middleware.RequireAdmin())
		{
			tickets.GET("", middleware.RequirePermission("ticket.view"), adminTicketHandler.ListTickets)
			tickets.GET("/stats", middleware.RequirePermission("ticket.view"), adminTicketHandler.GetTicketStats)
			tickets.GET("/:id", middleware.RequirePermission("ticket.view"), adminTicketHandler.GetTicket)
			tickets.GET("/:id/messages", middleware.RequirePermission("ticket.view"), adminTicketHandler.GetTicketMessages)
			tickets.POST("/:id/messages", middleware.RequirePermission("ticket.reply"), adminTicketHandler.SendMessage)
			tickets.PUT("/:id", middleware.RequirePermission("ticket.status_update"), adminTicketHandler.UpdateTicket)
			tickets.GET("/:id/shared-orders", middleware.RequirePermission("ticket.view"), adminTicketHandler.GetSharedOrders)
			tickets.GET("/:id/shared-orders/:orderId", middleware.RequirePermission("ticket.view"), adminTicketHandler.GetSharedOrder)
			tickets.POST("/:id/upload", middleware.RequirePermission("ticket.reply"), adminTicketHandler.UploadFile)
		}

		// 知识库管理
		knowledgeAdmin := adminAPI.Group("/knowledge")
		knowledgeAdmin.Use(middleware.AuthMiddleware(), middleware.RequireAdmin())
		{
			knowledgeAdmin.GET("/export", middleware.RequirePermission("knowledge.view"), adminKnowledgeHandler.ExportKnowledge)
			knowledgeAdmin.POST("/import", middleware.RequirePermission("knowledge.edit"), adminKnowledgeHandler.ImportKnowledge)
			// 分类管理
			knowledgeAdmin.GET("/categories", middleware.RequirePermission("knowledge.view"), adminKnowledgeHandler.ListCategories)
			knowledgeAdmin.POST("/categories", middleware.RequirePermission("knowledge.edit"), adminKnowledgeHandler.CreateCategory)
			knowledgeAdmin.PUT("/categories/:id", middleware.RequirePermission("knowledge.edit"), adminKnowledgeHandler.UpdateCategory)
			knowledgeAdmin.DELETE("/categories/:id", middleware.RequirePermission("knowledge.edit"), adminKnowledgeHandler.DeleteCategory)
			// 文章管理
			knowledgeAdmin.GET("/articles", middleware.RequirePermission("knowledge.view"), adminKnowledgeHandler.ListArticles)
			knowledgeAdmin.POST("/articles", middleware.RequirePermission("knowledge.edit"), adminKnowledgeHandler.CreateArticle)
			knowledgeAdmin.GET("/articles/:id", middleware.RequirePermission("knowledge.view"), adminKnowledgeHandler.GetArticle)
			knowledgeAdmin.PUT("/articles/:id", middleware.RequirePermission("knowledge.edit"), adminKnowledgeHandler.UpdateArticle)
			knowledgeAdmin.DELETE("/articles/:id", middleware.RequirePermission("knowledge.edit"), adminKnowledgeHandler.DeleteArticle)
		}

		// 公告管理
		announcementsAdmin := adminAPI.Group("/announcements")
		announcementsAdmin.Use(middleware.AuthMiddleware(), middleware.RequireAdmin())
		{
			announcementsAdmin.GET("", middleware.RequirePermission("announcement.view"), adminAnnouncementHandler.ListAnnouncements)
			announcementsAdmin.GET("/export", middleware.RequirePermission("announcement.view"), adminAnnouncementHandler.ExportAnnouncements)
			announcementsAdmin.POST("", middleware.RequirePermission("announcement.edit"), adminAnnouncementHandler.CreateAnnouncement)
			announcementsAdmin.GET("/:id", middleware.RequirePermission("announcement.view"), adminAnnouncementHandler.GetAnnouncement)
			announcementsAdmin.PUT("/:id", middleware.RequirePermission("announcement.edit"), adminAnnouncementHandler.UpdateAnnouncement)
			announcementsAdmin.DELETE("/:id", middleware.RequirePermission("announcement.edit"), adminAnnouncementHandler.DeleteAnnouncement)
		}

		marketingAdmin := adminAPI.Group("/marketing")
		marketingAdmin.Use(middleware.AuthMiddleware(), middleware.RequireAdmin())
		{
			marketingAdmin.GET("/countries", middleware.RequireAllPermissions("marketing.view", "user.view"), adminMarketingHandler.ListRecipientCountries)
			marketingAdmin.GET("/users", middleware.RequireAllPermissions("marketing.view", "user.view"), adminMarketingHandler.ListRecipients)
			marketingAdmin.POST("/preview", middleware.RequirePermission("marketing.send"), adminMarketingHandler.PreviewMarketing)
			marketingAdmin.GET("/batches", middleware.RequirePermission("marketing.view"), adminMarketingHandler.ListBatches)
			marketingAdmin.GET("/batches/:id", middleware.RequirePermission("marketing.view"), adminMarketingHandler.GetBatch)
			marketingAdmin.GET("/batches/:id/tasks", middleware.RequirePermission("marketing.view"), adminMarketingHandler.ListBatchTasks)
			marketingAdmin.POST("/send", middleware.RequirePermission("marketing.send"), adminMarketingHandler.SendMarketing)
		}

		// 插件管理
		plugins := adminAPI.Group("/plugins")
		plugins.Use(middleware.AuthMiddleware(), middleware.RequireSuperAdmin())
		{
			plugins.GET("", middleware.RequirePermission("plugin.view"), adminPluginHandler.ListPlugins)
			plugins.GET("/hook-catalog", middleware.RequirePermission("plugin.view"), adminPluginHandler.GetHookCatalog)
			plugins.GET("/observability", middleware.RequirePermission("plugin.diagnostics"), adminPluginHandler.GetPluginObservability)
			plugins.GET("/:id/diagnostics", middleware.RequirePermission("plugin.diagnostics"), adminPluginHandler.GetPluginDiagnostics)
			plugins.GET("/:id/workspace", middleware.RequirePermission("plugin.diagnostics"), adminPluginHandler.GetPluginWorkspace)
			plugins.GET("/:id/workspace/runtime", middleware.RequirePermission("plugin.diagnostics"), adminPluginHandler.GetPluginWorkspaceRuntimeState)
			plugins.GET("/:id/workspace/ws", middleware.RequirePermission("plugin.diagnostics"), adminPluginHandler.WebSocketPluginWorkspace)
			plugins.GET("/:id/workspace/stream", middleware.RequirePermission("plugin.diagnostics"), adminPluginHandler.StreamPluginWorkspace)
			plugins.POST("/:id/workspace/control/claim", middleware.RequirePermission("plugin.execute"), adminPluginHandler.ClaimPluginWorkspaceControl)
			plugins.POST("/:id/workspace/execute", middleware.RequirePermission("plugin.execute"), adminPluginHandler.ExecutePluginWorkspaceCommand)
			plugins.POST("/:id/workspace/terminal", middleware.RequirePermission("plugin.execute"), adminPluginHandler.EnterPluginWorkspaceTerminalLine)
			plugins.POST("/:id/workspace/runtime/reset", middleware.RequirePermission("plugin.execute"), adminPluginHandler.ResetPluginWorkspaceRuntime)
			plugins.POST("/:id/workspace/runtime/eval", middleware.RequirePermission("plugin.execute"), adminPluginHandler.EvaluatePluginWorkspaceRuntime)
			plugins.POST("/:id/workspace/runtime/inspect", middleware.RequirePermission("plugin.execute"), adminPluginHandler.InspectPluginWorkspaceRuntime)
			plugins.POST("/:id/workspace/input", middleware.RequirePermission("plugin.execute"), adminPluginHandler.SubmitPluginWorkspaceInput)
			plugins.POST("/:id/workspace/signal", middleware.RequirePermission("plugin.execute"), adminPluginHandler.SignalPluginWorkspace)
			plugins.POST("/:id/workspace/reset", middleware.RequirePermission("plugin.execute"), adminPluginHandler.ResetPluginWorkspace)
			plugins.POST("/:id/workspace/clear", middleware.RequirePermission("plugin.execute"), adminPluginHandler.ClearPluginWorkspace)
			plugins.GET("/:id/tasks", middleware.RequirePermission("plugin.diagnostics"), adminPluginHandler.GetPluginExecutionTasks)
			plugins.GET("/:id/tasks/:task_id", middleware.RequirePermission("plugin.diagnostics"), adminPluginHandler.GetPluginExecutionTask)
			plugins.POST("/:id/tasks/:task_id/cancel", middleware.RequirePermission("plugin.execute"), adminPluginHandler.CancelPluginExecutionTask)
			plugins.GET("/:id/secrets", middleware.RequirePermission("plugin.view"), adminPluginHandler.GetPluginSecrets)
			plugins.POST("", middleware.RequirePermission("plugin.edit"), adminPluginHandler.CreatePlugin)
			plugins.POST("/upload/preview", middleware.RequirePermission("plugin.upload"), adminPluginHandler.PreviewPluginPackage)
			plugins.POST("/upload", middleware.RequirePermission("plugin.upload"), adminPluginHandler.UploadPluginPackage)
			plugins.POST("/market/preview", middleware.RequirePermission("market.install"), adminPluginHandler.PreviewPluginMarketInstall)
			plugins.POST("/market/install", middleware.RequirePermission("market.install"), adminPluginHandler.InstallPluginFromMarket)
			plugins.GET("/:id", middleware.RequirePermission("plugin.view"), adminPluginHandler.GetPlugin)
			plugins.PUT("/:id", middleware.RequirePermission("plugin.edit"), adminPluginHandler.UpdatePlugin)
			plugins.PUT("/:id/secrets", middleware.RequirePermission("plugin.edit"), adminPluginHandler.UpdatePluginSecrets)
			plugins.DELETE("/:id", middleware.RequirePermission("plugin.edit"), adminPluginHandler.DeletePlugin)
			plugins.POST("/:id/lifecycle", middleware.RequirePermission("plugin.lifecycle"), adminPluginHandler.HandleLifecycleAction)
			plugins.POST("/:id/test", middleware.RequirePermission("plugin.execute"), adminPluginHandler.TestPlugin)
			plugins.POST("/:id/execute", middleware.RequirePermission("plugin.execute"), adminPluginHandler.ExecutePlugin)
			plugins.POST("/:id/execute/stream", middleware.RequirePermission("plugin.execute"), adminPluginHandler.ExecutePluginStream)
			plugins.GET("/:id/versions", middleware.RequirePermission("plugin.view"), adminPluginHandler.GetPluginVersions)
			plugins.DELETE("/:id/versions/:version_id", middleware.RequirePermission("plugin.lifecycle"), adminPluginHandler.DeletePluginVersion)
			plugins.POST("/:id/versions/:version_id/activate", middleware.RequirePermission("plugin.lifecycle"), adminPluginHandler.ActivatePluginVersion)
			plugins.GET("/:id/executions", middleware.RequirePermission("plugin.diagnostics"), adminPluginHandler.GetPluginExecutions)
		}
	}

	// 按业务类型分别暴露上传静态资源，避免整个 upload 根目录直出。
	uploadsGroup := r.Group("/uploads")
	{
		productUploadHandler := buildDynamicUploadFileHandler("products", cfg.Upload.Dir)
		ticketUploadHandler := buildDynamicUploadFileHandler("tickets", cfg.Upload.Dir)
		uploadsGroup.GET("/products/*filepath", productUploadHandler)
		uploadsGroup.HEAD("/products/*filepath", productUploadHandler)
		uploadsGroup.GET("/tickets/*filepath", ticketUploadHandler)
		uploadsGroup.HEAD("/tickets/*filepath", ticketUploadHandler)
	}

	// 落地页（公开）
	r.GET("/", adminLandingPageHandler.ServeLandingPage)

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	return r
}

func resolveRateLimit(limitSelector func(*config.Config) int, fallback int) middleware.RateLimitResolver {
	return func(c *gin.Context) (int, bool) {
		runtimeCfg := config.GetConfig()
		if runtimeCfg == nil {
			return 0, false
		}

		limit := 0
		if limitSelector != nil {
			limit = limitSelector(runtimeCfg)
		}
		if limit <= 0 {
			limit = fallback
		}
		return limit, runtimeCfg.RateLimit.Enabled && limit > 0
	}
}

func buildDynamicUploadFileHandler(area string, fallbackUploadDir string) gin.HandlerFunc {
	normalizedArea := strings.TrimSpace(area)
	return func(c *gin.Context) {
		runtimeCfg := config.GetConfig()
		currentUploadDir := "uploads"
		if runtimeCfg != nil && strings.TrimSpace(runtimeCfg.Upload.Dir) != "" {
			currentUploadDir = runtimeCfg.Upload.Dir
		}

		relativePath := strings.TrimPrefix(strings.TrimSpace(c.Param("filepath")), "/")
		cleanRel := filepath.Clean(filepath.FromSlash(relativePath))
		if cleanRel == "." || cleanRel == "" || strings.HasPrefix(cleanRel, "..") {
			c.Status(http.StatusNotFound)
			return
		}

		seen := make(map[string]struct{}, 2)
		uploadDirs := []string{currentUploadDir, fallbackUploadDir}
		for _, uploadDir := range uploadDirs {
			trimmedDir := strings.TrimSpace(uploadDir)
			if trimmedDir == "" {
				continue
			}
			if _, exists := seen[trimmedDir]; exists {
				continue
			}
			seen[trimmedDir] = struct{}{}

			baseDir, err := filepath.Abs(filepath.Join(trimmedDir, normalizedArea))
			if err != nil {
				continue
			}
			filePath, err := filepath.Abs(filepath.Join(baseDir, cleanRel))
			if err != nil {
				continue
			}
			if filePath != baseDir && !strings.HasPrefix(filePath, baseDir+string(os.PathSeparator)) {
				continue
			}

			info, err := os.Stat(filePath)
			if err != nil || info.IsDir() {
				continue
			}

			c.File(filePath)
			return
		}

		c.Status(http.StatusNotFound)
	}
}
