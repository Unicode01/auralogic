package database

import (
	"fmt"
	"log"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/models"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// InitDatabase 初始化数据库连接
func InitDatabase(cfg *config.DatabaseConfig) error {
	var dialector gorm.Dialector

	switch cfg.Driver {
	case "postgres":
		dialector = postgres.Open(cfg.GetDSN())
	case "mysql":
		dialector = mysql.Open(cfg.GetDSN())
	case "sqlite":
		dialector = sqlite.Open(cfg.GetDSN())
	default:
		return fmt.Errorf("unsupported database driver: %s", cfg.Driver)
	}

	// GORM配置
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	}

	// 连接数据库
	db, err := gorm.Open(dialector, gormConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// get底层的sql.DB以配置连接池
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}

	// 连接池参数
	//
	// For SQLite, multiple pooled connections are a common source of "database is locked" stalls/errors under write
	// concurrency. We force a single connection and rely on WAL + busy_timeout to avoid lock contention.
	if cfg.Driver == "sqlite" {
		sqlDB.SetMaxIdleConns(1)
		sqlDB.SetMaxOpenConns(1)
		// Keep the single connection alive (PRAGMAs are per-connection).
		sqlDB.SetConnMaxLifetime(0)
	} else {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
		sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)
	}

	// 测试连接
	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	// SQLite: apply pragmas on the active connection to reduce lock contention.
	if cfg.Driver == "sqlite" {
		if err := applySQLitePragmas(db); err != nil {
			return fmt.Errorf("failed to apply sqlite pragmas: %w", err)
		}
		log.Println("SQLite pragmas applied: journal_mode=WAL, synchronous=NORMAL, foreign_keys=ON, busy_timeout=5000ms; connection pool forced to 1")
	}

	DB = db
	log.Println("Database connected successfully")
	return nil
}

func applySQLitePragmas(db *gorm.DB) error {
	// busy_timeout helps avoid immediate "database is locked" errors; WAL improves reader/writer concurrency.
	// Note: PRAGMAs are per-connection, so this works best with MaxOpenConns=1.
	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
	}
	for _, p := range pragmas {
		if err := db.Exec(p).Error; err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
	}
	return nil
}

// AutoMigrate 自动迁移数据库表
func AutoMigrate() error {
	// 先尝试删除可能冲突的旧索引（SQLite 的索引是全局的）
	// 这些索引可能在旧版本中创建，导致与新表冲突
	oldIndexes := []string{
		"idx_product_virtual_inventory", // 旧的 product_virtual_inventory_bindings 表索引
		"idx_virtual_inventory",         // 旧的索引名
	}
	for _, idx := range oldIndexes {
		// 忽略删除索引时的错误（如果索引不存在）
		DB.Exec("DROP INDEX IF EXISTS " + idx)
	}

	// 在AutoMigrate之前，先处理API密钥迁移（删除旧数据，避免NOT NULL冲突）
	if err := migrateAPIKeySecretToHash(); err != nil {
		log.Printf("Warning: failed to migrate API key secrets to hash: %v", err)
	}

	// 批量迁移所有模型（GORM会自动处理依赖顺序）
	if err := DB.AutoMigrate(
		&models.User{},
		&models.AdminPermission{},
		&models.Order{},
		&models.Product{},
		&models.Inventory{},
		&models.InventoryLog{},
		&models.ProductInventoryBinding{},
		&models.ProductSerial{},
		&models.MagicToken{},
		&models.APIKey{},
		&models.OperationLog{},
		&models.EmailLog{},
		&models.VirtualInventory{},
		&models.VirtualProductStock{},
		&models.ProductVirtualInventoryBinding{},
		&models.CartItem{},
		&models.PaymentMethod{},
		&models.OrderPaymentMethod{},
		&models.PaymentPollingTask{},
		&models.Ticket{},
		&models.TicketMessage{},
		&models.TicketOrderAccess{},
		&models.PromoCode{},
		&models.KnowledgeCategory{},
		&models.KnowledgeArticle{},
		&models.Announcement{},
		&models.AnnouncementRead{},
		&models.EmailVerificationToken{},
		&models.LandingPage{},
		&models.PageView{},
	); err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	log.Println("Database migration completed successfully")

	// 种子数据：默认落地页
	seedDefaultLandingPage()

	// 迁移：为现有的虚拟库存绑定记录计算 attributes_hash
	if err := migrateVirtualInventoryBindingsHash(); err != nil {
		log.Printf("Warning: failed to migrate virtual inventory bindings hash: %v", err)
	}

	// Migration: allow re-registering with the same email/phone after soft-delete.
	// This is done by "active-only" (deleted_at IS NULL) unique indexes, plus dropping old global unique indexes.
	if err := migrateUserActiveUniqueIndexes(); err != nil {
		log.Printf("Warning: failed to migrate users active-only unique indexes: %v", err)
	}

	// Migration: allow reusing product SKU after soft-delete by enforcing uniqueness only for active products.
	if err := migrateProductActiveUniqueIndexes(); err != nil {
		log.Printf("Warning: failed to migrate products active-only unique index: %v", err)
	}

	return nil
}

// migrateUserActiveUniqueIndexes ensures uniqueness for active (non-deleted) users while allowing
// reusing email/phone after soft-delete.
func migrateUserActiveUniqueIndexes() error {
	dialect := DB.Dialector.Name()
	switch dialect {
	case "sqlite":
		// Create partial unique indexes.
		// Exclude empty email/phone to avoid blocking multiple "email-less" rows (if any).
		if err := DB.Exec(`
CREATE UNIQUE INDEX IF NOT EXISTS uidx_users_email_active
ON users(email)
WHERE deleted_at IS NULL AND email <> ''`).Error; err != nil {
			return err
		}
		if err := DB.Exec(`
CREATE UNIQUE INDEX IF NOT EXISTS uidx_users_phone_active
ON users(phone)
WHERE deleted_at IS NULL AND phone IS NOT NULL AND phone <> ''`).Error; err != nil {
			return err
		}

		// Drop old global unique indexes (created by previous gorm tags).
		// Ignore errors: index might not exist or might have a different name.
		old := []string{
			"idx_users_email",
			"idx_user_email",
			"idx_users_phone",
			"idx_user_phone",
		}
		for _, idx := range old {
			DB.Exec("DROP INDEX IF EXISTS " + idx)
		}
		return nil

	case "postgres":
		// Create partial unique indexes.
		if err := DB.Exec(`
CREATE UNIQUE INDEX IF NOT EXISTS uidx_users_email_active
ON users(email)
WHERE deleted_at IS NULL AND email <> ''`).Error; err != nil {
			return err
		}
		if err := DB.Exec(`
CREATE UNIQUE INDEX IF NOT EXISTS uidx_users_phone_active
ON users(phone)
WHERE deleted_at IS NULL AND phone IS NOT NULL AND phone <> ''`).Error; err != nil {
			return err
		}

		// Drop old indexes if they exist.
		old := []string{
			"idx_users_email",
			"idx_user_email",
			"idx_users_phone",
			"idx_user_phone",
		}
		for _, idx := range old {
			DB.Exec(`DROP INDEX IF EXISTS ` + idx)
		}
		return nil
	default:
		// MySQL doesn't support partial indexes; keeping old behavior for now.
		// If you need this on MySQL, the safest approach is to anonymize email/phone on delete.
		return nil
	}
}

// migrateProductActiveUniqueIndexes ensures SKU uniqueness for active (non-deleted) products while allowing
// reusing SKU after soft-delete.
func migrateProductActiveUniqueIndexes() error {
	dialect := DB.Dialector.Name()
	switch dialect {
	case "sqlite":
		if err := DB.Exec(`
CREATE UNIQUE INDEX IF NOT EXISTS uidx_products_sku_active
ON products(sku)
WHERE deleted_at IS NULL AND sku <> ''`).Error; err != nil {
			return err
		}

		// Drop old global unique indexes (created by previous gorm tags).
		old := []string{
			"idx_products_sku",
			"idx_product_sku",
			"uidx_products_sku",
			"uidx_product_sku",
		}
		for _, idx := range old {
			DB.Exec("DROP INDEX IF EXISTS " + idx)
		}
		return nil

	case "postgres":
		if err := DB.Exec(`
CREATE UNIQUE INDEX IF NOT EXISTS uidx_products_sku_active
ON products(sku)
WHERE deleted_at IS NULL AND sku <> ''`).Error; err != nil {
			return err
		}

		old := []string{
			"idx_products_sku",
			"idx_product_sku",
			"uidx_products_sku",
			"uidx_product_sku",
		}
		for _, idx := range old {
			DB.Exec(`DROP INDEX IF EXISTS ` + idx)
		}
		return nil

	default:
		// MySQL doesn't support partial indexes. Keeping current behavior for now.
		return nil
	}
}

func migrateVirtualInventoryBindingsHash() error {
	var bindings []models.ProductVirtualInventoryBinding
	if err := DB.Where("attributes_hash = '' OR attributes_hash IS NULL").Find(&bindings).Error; err != nil {
		return err
	}

	if len(bindings) == 0 {
		return nil
	}

	log.Printf("Migrating %d virtual inventory bindings to add attributes_hash...", len(bindings))

	for _, binding := range bindings {
		// 将 JSONMap 转为 map[string]string
		attrs := map[string]string(binding.Attributes)
		normalizedAttrs := models.NormalizeAttributes(attrs)
		attributesHash := models.GenerateAttributesHash(normalizedAttrs)

		if err := DB.Model(&binding).Update("attributes_hash", attributesHash).Error; err != nil {
			log.Printf("Warning: failed to update binding %d: %v", binding.ID, err)
			continue
		}
	}

	log.Printf("Successfully migrated %d virtual inventory bindings", len(bindings))
	return nil
}

// migrateAPIKeySecretToHash 将现有API密钥从明文迁移到哈希存储
// 注意：此迁移会使现有的API密钥失效，需要重新生成
func migrateAPIKeySecretToHash() error {
	// 检查api_keys表是否存在
	if !DB.Migrator().HasTable("api_keys") {
		return nil // 表不存在，跳过迁移
	}

	// 检查是否存在旧的 api_secret 列（明文存储）
	if DB.Migrator().HasColumn(&models.APIKey{}, "api_secret") {
		log.Println("Migrating API keys from plaintext to hashed storage...")
		log.Println("WARNING: Existing API keys will be invalidated. Please regenerate them after migration.")

		// 删除所有现有的API密钥（因为明文密钥无法转换为哈希）
		if err := DB.Exec("DELETE FROM api_keys").Error; err != nil {
			log.Printf("Warning: failed to clear old API keys: %v", err)
		} else {
			log.Println("All existing API keys have been deleted. Please create new API keys.")
		}

		// 尝试删除旧列（某些数据库可能不支持）
		_ = DB.Migrator().DropColumn(&models.APIKey{}, "api_secret")
	}

	return nil
}

// defaultLandingPageHTML 由外部注入，避免 database 包依赖 handler 包
var defaultLandingPageHTML string

// SetDefaultLandingPageHTML 设置默认落地页 HTML（应在 AutoMigrate 之前调用）
func SetDefaultLandingPageHTML(html string) {
	defaultLandingPageHTML = html
}

// seedDefaultLandingPage 创建默认落地页
func seedDefaultLandingPage() {
	var count int64
	DB.Model(&models.LandingPage{}).Where("slug = ?", "home").Count(&count)
	if count > 0 {
		return
	}

	page := models.LandingPage{
		Slug:        "home",
		HTMLContent: defaultLandingPageHTML,
		IsActive:    true,
	}
	if err := DB.Create(&page).Error; err != nil {
		log.Printf("Warning: failed to seed default landing page: %v", err)
	} else {
		log.Println("Default landing page seeded successfully")
	}
}

// GetDB get数据库实例
func GetDB() *gorm.DB {
	return DB
}

// Close 关闭数据库连接
func Close() error {
	if DB != nil {
		sqlDB, err := DB.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	}
	return nil
}
