package admin

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"auralogic/internal/config"
	"auralogic/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCleanupUnreferencedFilesRemovesOnlyOrphans(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "cleanup-test.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db failed: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	if err := db.AutoMigrate(
		&models.Product{},
		&models.Ticket{},
		&models.TicketMessage{},
		&models.Plugin{},
		&models.PluginVersion{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	tempRoot := t.TempDir()
	uploadRoot := filepath.Join(tempRoot, "uploads")
	artifactRoot := filepath.Join(tempRoot, "artifacts")

	productRelDir := filepath.Join("2026", "03", "16")
	productRoot := filepath.Join(uploadRoot, "products", productRelDir)
	productRef := filepath.Join(productRoot, "ref.png")
	productOrphan := filepath.Join(productRoot, "orphan.png")
	mustWriteCleanupFile(t, productRef)
	mustWriteCleanupFile(t, productOrphan)

	ticketRelDir := filepath.Join("2026", "03", "16")
	ticketRoot := filepath.Join(uploadRoot, "tickets", ticketRelDir)
	ticketRef := filepath.Join(ticketRoot, "ref.png")
	ticketOrphan := filepath.Join(ticketRoot, "orphan.png")
	mustWriteCleanupFile(t, ticketRef)
	mustWriteCleanupFile(t, ticketOrphan)

	manualPluginRoot := filepath.Join(artifactRoot, "manual-plugin")
	manualPluginEntry := filepath.Join(manualPluginRoot, "index.js")
	mustWriteCleanupFile(t, filepath.Join(manualPluginRoot, "manifest.json"))
	mustWriteCleanupFile(t, manualPluginEntry)

	versionZip := filepath.Join(artifactRoot, "packages", "demo.zip")
	versionExtractRoot := filepath.Join(artifactRoot, "packages", "jsworker", "pkg", "demo", "root")
	versionExtractEntry := filepath.Join(versionExtractRoot, "main.js")
	mustWriteCleanupFile(t, versionZip)
	mustWriteCleanupFile(t, versionExtractEntry)

	pluginOrphanFile := filepath.Join(artifactRoot, "packages", "orphan.zip")
	pluginOrphanEntry := filepath.Join(artifactRoot, "packages", "jsworker", "pkg", "orphan", "root", "orphan.js")
	mustWriteCleanupFile(t, pluginOrphanFile)
	mustWriteCleanupFile(t, pluginOrphanEntry)

	productURL := fmt.Sprintf("https://example.com/uploads/products/%s/ref.png", filepath.ToSlash(productRelDir))
	if err := db.Create(&models.Product{
		SKU:         "cleanup-product",
		Name:        "Cleanup Product",
		ProductType: models.ProductTypePhysical,
		Description: fmt.Sprintf("![image](%s)", productURL),
		Status:      models.ProductStatusActive,
	}).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	ticket := models.Ticket{
		TicketNo: "TK-1001",
		UserID:   1,
		Subject:  "Cleanup Ticket",
		Content:  "initial",
		Status:   models.TicketStatusOpen,
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket failed: %v", err)
	}
	if err := db.Create(&models.TicketMessage{
		TicketID:    ticket.ID,
		SenderType:  "user",
		SenderID:    1,
		SenderName:  "user",
		Content:     fmt.Sprintf("![ticket](https://example.com/uploads/tickets/%s/ref.png)", filepath.ToSlash(ticketRelDir)),
		ContentType: "text",
	}).Error; err != nil {
		t.Fatalf("create ticket message failed: %v", err)
	}

	plugin := models.Plugin{
		Name:            "cleanup-plugin",
		DisplayName:     "Cleanup Plugin",
		Type:            "custom",
		Runtime:         "js_worker",
		Address:         "index.js",
		PackagePath:     manualPluginRoot,
		Version:         "1.0.0",
		LifecycleStatus: models.PluginLifecycleInstalled,
	}
	if err := db.Create(&plugin).Error; err != nil {
		t.Fatalf("create plugin failed: %v", err)
	}

	pluginDataFile := filepath.Join(artifactRoot, "data", fmt.Sprintf("plugin_%d", plugin.ID), "state.json")
	mustWriteCleanupFile(t, pluginDataFile)

	if err := db.Create(&models.PluginVersion{
		PluginID:        plugin.ID,
		Version:         "1.0.0",
		PackageName:     "demo.zip",
		PackagePath:     versionZip,
		Address:         versionExtractEntry,
		LifecycleStatus: models.PluginLifecycleInstalled,
		IsActive:        true,
	}).Error; err != nil {
		t.Fatalf("create plugin version failed: %v", err)
	}

	handler := &SettingsHandler{
		db: db,
		cfg: &config.Config{
			Upload: config.UploadConfig{Dir: uploadRoot},
			Plugin: config.PluginPlatformConfig{ArtifactDir: artifactRoot},
		},
	}

	stats, err := handler.cleanupUnreferencedFiles()
	if err != nil {
		t.Fatalf("cleanup unreferenced files failed: %v", err)
	}

	if stats.DeletedFiles != 4 {
		t.Fatalf("expected 4 deleted files, got %+v", stats)
	}
	if stats.DeletedDirs == 0 {
		t.Fatalf("expected empty orphan directories to be pruned, got %+v", stats)
	}

	assertCleanupPathExists(t, productRef, true)
	assertCleanupPathExists(t, ticketRef, true)
	assertCleanupPathExists(t, manualPluginEntry, true)
	assertCleanupPathExists(t, versionZip, true)
	assertCleanupPathExists(t, versionExtractEntry, true)
	assertCleanupPathExists(t, pluginDataFile, true)

	assertCleanupPathExists(t, productOrphan, false)
	assertCleanupPathExists(t, ticketOrphan, false)
	assertCleanupPathExists(t, pluginOrphanFile, false)
	assertCleanupPathExists(t, pluginOrphanEntry, false)
}

func mustWriteCleanupFile(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s failed: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte("cleanup"), 0o644); err != nil {
		t.Fatalf("write file %s failed: %v", path, err)
	}
}

func assertCleanupPathExists(t *testing.T, path string, shouldExist bool) {
	t.Helper()

	_, err := os.Stat(path)
	if shouldExist && err != nil {
		t.Fatalf("expected %s to exist, got err=%v", path, err)
	}
	if !shouldExist && !os.IsNotExist(err) {
		t.Fatalf("expected %s to be removed, got err=%v", path, err)
	}
}
