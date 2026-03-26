package service

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"auralogic/internal/models"
	"auralogic/internal/pkg/bizerr"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newVirtualInventoryServiceTestDB(t *testing.T) (*VirtualInventoryService, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if err := db.AutoMigrate(
		&models.Product{},
		&models.VirtualInventory{},
		&models.VirtualProductStock{},
		&models.ProductVirtualInventoryBinding{},
		&models.InventoryLog{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	return NewVirtualInventoryService(db), db
}

func requireBizErr(t *testing.T, err error, key string) *bizerr.Error {
	t.Helper()

	if err == nil {
		t.Fatalf("expected error %q, got nil", key)
	}

	var bizErr *bizerr.Error
	if !errors.As(err, &bizErr) {
		t.Fatalf("expected bizerr %q, got %T (%v)", key, err, err)
	}
	if bizErr.Key != key {
		t.Fatalf("expected bizerr key %q, got %q", key, bizErr.Key)
	}
	return bizErr
}

func TestDeleteVirtualInventoryReturnsBizErrors(t *testing.T) {
	svc, db := newVirtualInventoryServiceTestDB(t)

	inventory := &models.VirtualInventory{
		Name:     "Stock-backed inventory",
		Type:     models.VirtualInventoryTypeStatic,
		IsActive: true,
	}
	if err := db.Create(inventory).Error; err != nil {
		t.Fatalf("create inventory: %v", err)
	}
	if err := db.Create(&models.VirtualProductStock{
		VirtualInventoryID: inventory.ID,
		Content:            "CARD-001",
		Status:             models.VirtualStockStatusAvailable,
	}).Error; err != nil {
		t.Fatalf("create stock: %v", err)
	}

	stockErr := requireBizErr(t, svc.DeleteVirtualInventory(inventory.ID), "virtual_inventory.hasStockItems")
	if got := stockErr.Params["stock_count"]; got != int64(1) {
		t.Fatalf("expected stock_count=1, got %#v", stockErr.Params)
	}

	boundInventory := &models.VirtualInventory{
		Name:     "Bound inventory",
		Type:     models.VirtualInventoryTypeStatic,
		IsActive: true,
	}
	if err := db.Create(boundInventory).Error; err != nil {
		t.Fatalf("create bound inventory: %v", err)
	}
	product := &models.Product{
		SKU:         "virtual-product-1",
		Name:        "Virtual Product",
		ProductType: models.ProductTypeVirtual,
		Status:      models.ProductStatusActive,
		Stock:       1,
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}
	if err := db.Create(&models.ProductVirtualInventoryBinding{
		ProductID:          product.ID,
		VirtualInventoryID: boundInventory.ID,
		Priority:           1,
	}).Error; err != nil {
		t.Fatalf("create binding: %v", err)
	}

	bindingErr := requireBizErr(t, svc.DeleteVirtualInventory(boundInventory.ID), "virtual_inventory.hasProductBindings")
	if got := bindingErr.Params["binding_count"]; got != int64(1) {
		t.Fatalf("expected binding_count=1, got %#v", bindingErr.Params)
	}
}

func TestDeleteStockReturnsBizErrors(t *testing.T) {
	svc, db := newVirtualInventoryServiceTestDB(t)

	requireBizErr(t, svc.DeleteStock(999), "virtual_inventory.stockItemNotFound")

	inventory := &models.VirtualInventory{
		Name:     "Delete stock inventory",
		Type:     models.VirtualInventoryTypeStatic,
		IsActive: true,
	}
	if err := db.Create(inventory).Error; err != nil {
		t.Fatalf("create inventory: %v", err)
	}
	stock := &models.VirtualProductStock{
		VirtualInventoryID: inventory.ID,
		Content:            "CARD-DELETE",
		Status:             models.VirtualStockStatusSold,
	}
	if err := db.Create(stock).Error; err != nil {
		t.Fatalf("create stock: %v", err)
	}

	err := requireBizErr(t, svc.DeleteStock(stock.ID), "virtual_inventory.stockDeleteStatusInvalid")
	if got := err.Params["status"]; got != string(models.VirtualStockStatusSold) {
		t.Fatalf("expected status=%q, got %#v", models.VirtualStockStatusSold, err.Params)
	}
}

func TestManualStockOperationsReturnBizErrors(t *testing.T) {
	svc, db := newVirtualInventoryServiceTestDB(t)

	requireBizErr(t, svc.ManualReserveStock(999, ""), "virtual_inventory.stockItemNotFound")
	requireBizErr(t, svc.ManualReleaseStock(999), "virtual_inventory.stockItemNotFound")

	inventory := &models.VirtualInventory{
		Name:     "Manual stock inventory",
		Type:     models.VirtualInventoryTypeStatic,
		IsActive: true,
	}
	if err := db.Create(inventory).Error; err != nil {
		t.Fatalf("create inventory: %v", err)
	}

	reservedStock := &models.VirtualProductStock{
		VirtualInventoryID: inventory.ID,
		Content:            "CARD-RESERVED",
		Status:             models.VirtualStockStatusReserved,
	}
	if err := db.Create(reservedStock).Error; err != nil {
		t.Fatalf("create reserved stock: %v", err)
	}
	reserveErr := requireBizErr(t, svc.ManualReserveStock(reservedStock.ID, ""), "virtual_inventory.stockItemUnavailable")
	if got := reserveErr.Params["status"]; got != string(models.VirtualStockStatusReserved) {
		t.Fatalf("expected status=%q, got %#v", models.VirtualStockStatusReserved, reserveErr.Params)
	}

	availableStock := &models.VirtualProductStock{
		VirtualInventoryID: inventory.ID,
		Content:            "CARD-AVAILABLE",
		Status:             models.VirtualStockStatusAvailable,
	}
	if err := db.Create(availableStock).Error; err != nil {
		t.Fatalf("create available stock: %v", err)
	}
	releaseErr := requireBizErr(t, svc.ManualReleaseStock(availableStock.ID), "virtual_inventory.stockItemNotReserved")
	if got := releaseErr.Params["status"]; got != string(models.VirtualStockStatusAvailable) {
		t.Fatalf("expected status=%q, got %#v", models.VirtualStockStatusAvailable, releaseErr.Params)
	}
}

func TestVirtualInventoryValidationReturnsBizErrors(t *testing.T) {
	svc, _ := newVirtualInventoryServiceTestDB(t)

	_, err := svc.ImportFromText(1, strings.Repeat(" \n", 2), "tester")
	importErr := requireBizErr(t, err, "virtual_inventory.importNoValidData")
	if got := importErr.Params["source"]; got != "text" {
		t.Fatalf("expected source=text, got %#v", importErr.Params)
	}

	requireBizErr(t, func() error {
		_, err := svc.TestDeliveryScript("   ", nil, 1)
		return err
	}(), "virtual_inventory.scriptRequired")
}

func TestVirtualBindingErrorsAreStructured(t *testing.T) {
	svc, db := newVirtualInventoryServiceTestDB(t)

	product := &models.Product{
		SKU:         "virtual-product-binding-test",
		Name:        "Virtual Binding Product",
		ProductType: models.ProductTypeVirtual,
		Status:      models.ProductStatusActive,
		Stock:       1,
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}

	requireBizErr(t, func() error {
		_, err := svc.GetFirstBindingForProduct(product.ID)
		return err
	}(), "virtual_binding.noBoundInventory")

	requireBizErr(t, func() error {
		_, _, err := svc.SelectRandomVirtualInventory(product.ID, 1)
		return err
	}(), "virtual_binding.noBoundInventory")

	requireBizErr(t, func() error {
		_, _, err := svc.FindVirtualInventoryWithPartialMatch(product.ID, map[string]string{"region": "us"}, 1)
		return err
	}(), "virtual_binding.noBoundInventory")

	_, err := svc.ImportStockForProduct(product.ID, "CODE-001", "tester")
	requireBizErr(t, err, "virtual_binding.noBoundInventory")

	inventory := &models.VirtualInventory{
		Name:     "Virtual Binding Inventory",
		Type:     models.VirtualInventoryTypeStatic,
		IsActive: true,
	}
	if err := db.Create(inventory).Error; err != nil {
		t.Fatalf("create inventory: %v", err)
	}

	attrs := map[string]string{"region": "us"}
	if err := db.Create(&models.ProductVirtualInventoryBinding{
		ProductID:          product.ID,
		VirtualInventoryID: inventory.ID,
		Attributes:         models.JSONMap(attrs),
		AttributesHash:     models.GenerateAttributesHash(attrs),
		Priority:           1,
	}).Error; err != nil {
		t.Fatalf("create binding: %v", err)
	}

	requireBizErr(t, func() error {
		_, err := svc.GetBindingByAttributes(product.ID, map[string]string{"region": "eu"})
		return err
	}(), "virtual_binding.noMatchingInventory")

	requireBizErr(t, func() error {
		_, _, err := svc.SelectRandomVirtualInventory(product.ID, 1)
		return err
	}(), "virtual_binding.insufficientAvailable")

	requireBizErr(t, func() error {
		_, _, err := svc.FindVirtualInventoryWithPartialMatch(product.ID, map[string]string{"region": "eu"}, 1)
		return err
	}(), "virtual_binding.noMatchingInventory")

	partialInsufficientErr := requireBizErr(t, func() error {
		_, _, err := svc.FindVirtualInventoryWithPartialMatch(product.ID, map[string]string{"region": "us"}, 1)
		return err
	}(), "virtual_binding.insufficientAvailable")
	if got := partialInsufficientErr.Params["requested"]; got != 1 {
		t.Fatalf("expected requested=1, got %#v", partialInsufficientErr.Params)
	}

	allocateErr := requireBizErr(t, func() error {
		_, _, err := svc.AllocateStockForProductByAttributes(product.ID, 1, "ORDER-1", map[string]interface{}{"region": "us"})
		return err
	}(), "virtual_binding.insufficientAvailable")
	if got := allocateErr.Params["requested"]; got != 1 {
		t.Fatalf("expected requested=1, got %#v", allocateErr.Params)
	}
}
