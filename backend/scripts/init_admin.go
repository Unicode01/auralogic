package main

import (
	"fmt"
	"log"

	"github.com/google/uuid"
	"auralogic/internal/config"
	"auralogic/internal/database"
	adminHandler "auralogic/internal/handler/admin"
	"auralogic/internal/models"
	"auralogic/internal/pkg/password"
)

func main() {
	log.Println("Initializing Super Admin...")

	// 加载配置
	cfg, err := config.LoadConfig(config.GetConfigPath())
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 加载Admin配置
	adminCfg, err := config.LoadAdminConfig("config/admin.json")
	if err != nil {
		log.Fatalf("Failed to load admin config: %v", err)
	}

	// 初始化数据库
	if err := database.InitDatabase(&cfg.Database); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// 注入默认落地页 HTML
	database.SetDefaultLandingPageHTML(adminHandler.DefaultLandingPageHTML)

	// 自动迁移数据库
	if err := database.AutoMigrate(); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	db := database.GetDB()

	// 检查超级Admin是否已存在
	var existingUser models.User
	if err := db.Where("email = ?", adminCfg.SuperAdmin.Email).First(&existingUser).Error; err == nil {
		log.Printf("Super admin already exists: %s", adminCfg.SuperAdmin.Email)
		return
	}

	// 哈希Password
	hashedPassword, err := password.HashPassword(adminCfg.SuperAdmin.Password)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	// Create超级Admin
	admin := &models.User{
		UUID:          uuid.New().String(),
		Email:         adminCfg.SuperAdmin.Email,
		PasswordHash:  hashedPassword,
		Name:          adminCfg.SuperAdmin.Name,
		Role:          "super_admin",
		IsActive:      true,
		EmailVerified: true,
	}

	if err := db.Create(admin).Error; err != nil {
		log.Fatalf("Failed to create super admin: %v", err)
	}

	// CreateAdminPermission（超级Admin拥有所有Permission）
	permissions := []string{
		// Order
		"order.view",
		"order.view_privacy",
		"order.edit",
		"order.delete",
		"order.status_update",
		"order.assign_tracking",
		"order.request_resubmit",
		// Product
		"product.view",
		"product.edit",
		"product.delete",
		// Serial
		"serial.view",
		"serial.manage",
		// User
		"user.view",
		"user.edit",
		"user.permission",
		// Admin
		"admin.create",
		"admin.edit",
		"admin.delete",
		"admin.permission",
		// System
		"system.config",
		"system.logs",
		"api.manage",
		// Knowledge
		"knowledge.view",
		"knowledge.edit",
		// Announcement
		"announcement.view",
		"announcement.edit",
		// Ticket
		"ticket.view",
		"ticket.reply",
		"ticket.status_update",
	}

	adminPerm := &models.AdminPermission{
		UserID:      admin.ID,
		Permissions: permissions,
		CreatedBy:   &admin.ID,
	}

	if err := db.Create(adminPerm).Error; err != nil {
		log.Fatalf("Failed to create admin permissions: %v", err)
	}

	log.Printf("Super admin created successfully!")
	log.Printf("Email: %s", admin.Email)
	log.Printf("Please change the default password after first login.")
	fmt.Println("\n⚠️  IMPORTANT: Please change the default password immediately after first login!")
}
