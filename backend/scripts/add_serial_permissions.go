package main

import (
	"log"

	"auralogic/internal/config"
	"auralogic/internal/database"
	"auralogic/internal/models"
)

func main() {
	log.Println("Adding serial permissions to super admin...")

	// 加载配置
	cfg, err := config.LoadConfig(config.GetConfigPath())
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 初始化数据库
	if err := database.InitDatabase(&cfg.Database); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	db := database.GetDB()

	// 查找所有超级管理员
	var superAdmins []models.User
	if err := db.Where("role = ?", "super_admin").Find(&superAdmins).Error; err != nil {
		log.Fatalf("Failed to find super admins: %v", err)
	}

	if len(superAdmins) == 0 {
		log.Println("No super admin found")
		return
	}

	// 为每个超级管理员添加序列号权限
	for _, admin := range superAdmins {
		var perm models.AdminPermission
		err := db.Where("user_id = ?", admin.ID).First(&perm).Error

		if err == nil {
			// 权限记录存在，检查是否已有序列号权限
			hasSerialView := false
			hasSerialManage := false

			for _, p := range perm.Permissions {
				if p == "serial.view" {
					hasSerialView = true
				}
				if p == "serial.manage" {
					hasSerialManage = true
				}
			}

			// 添加缺失的权限
			if !hasSerialView {
				perm.Permissions = append(perm.Permissions, "serial.view")
			}
			if !hasSerialManage {
				perm.Permissions = append(perm.Permissions, "serial.manage")
			}

			// 保存
			if !hasSerialView || !hasSerialManage {
				if err := db.Save(&perm).Error; err != nil {
					log.Printf("Failed to update permissions for admin %s: %v", admin.Email, err)
				} else {
					log.Printf("✓ Added serial permissions to admin: %s", admin.Email)
				}
			} else {
				log.Printf("○ Admin %s already has serial permissions", admin.Email)
			}
		}
	}

	log.Println("Serial permissions added successfully!")
}
