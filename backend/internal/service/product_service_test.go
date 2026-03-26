package service

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"auralogic/internal/models"
	"auralogic/internal/pkg/bizerr"
	"auralogic/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newProductServiceTestDB(t *testing.T) (*ProductService, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if err := db.AutoMigrate(
		&models.Product{},
		&models.Inventory{},
		&models.ProductInventoryBinding{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	productRepo := repository.NewProductRepository(db)
	return NewProductService(productRepo, nil), db
}

func requireProductBizErr(t *testing.T, err error, key string) *bizerr.Error {
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

func TestCreateProductReturnsBizErrors(t *testing.T) {
	svc, db := newProductServiceTestDB(t)

	requireProductBizErr(t, svc.CreateProduct(&models.Product{
		SKU:    "sku-name-required",
		Status: models.ProductStatusDraft,
	}), "product.nameRequired")

	requireProductBizErr(t, svc.CreateProduct(&models.Product{
		Name:   "SKU Required",
		Status: models.ProductStatusDraft,
	}), "product.skuRequired")

	requireProductBizErr(t, svc.CreateProduct(&models.Product{
		SKU:    "sku-price-negative",
		Name:   "Negative Price",
		Price:  -1,
		Status: models.ProductStatusDraft,
	}), "product.priceNegative")

	existing := &models.Product{
		SKU:         "duplicate-sku",
		Name:        "Existing Product",
		Status:      models.ProductStatusActive,
		ProductType: models.ProductTypePhysical,
		Stock:       1,
	}
	if err := db.Create(existing).Error; err != nil {
		t.Fatalf("create existing product: %v", err)
	}

	requireProductBizErr(t, svc.CreateProduct(&models.Product{
		SKU:         "duplicate-sku",
		Name:        "Duplicate Product",
		Status:      models.ProductStatusActive,
		ProductType: models.ProductTypePhysical,
		Stock:       1,
	}), "product.skuAlreadyExists")
}

func TestProductStockOperationsReturnStructuredErrors(t *testing.T) {
	svc, db := newProductServiceTestDB(t)

	requireProductBizErr(t, svc.UpdateStock(1, -1), "product.stockNegative")

	product := &models.Product{
		SKU:         "stock-product",
		Name:        "Stock Product",
		Status:      models.ProductStatusActive,
		ProductType: models.ProductTypePhysical,
		Stock:       2,
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}

	requireProductBizErr(t, svc.DecrementStock(product.ID, 0), "product.quantityInvalid")

	stockErr := requireProductBizErr(t, svc.DecrementStock(product.ID, 3), "product.stockInsufficient")
	if got := stockErr.Params["available"]; got != 2 {
		t.Fatalf("expected available=2, got %#v", stockErr.Params)
	}
	if got := stockErr.Params["requested"]; got != 3 {
		t.Fatalf("expected requested=3, got %#v", stockErr.Params)
	}
}

func TestProductLookupReturnsSentinelError(t *testing.T) {
	svc, _ := newProductServiceTestDB(t)

	if _, err := svc.GetProductByID(999, false); !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("expected ErrProductNotFound, got %v", err)
	}
	if err := svc.UpdateProductStatus(999, models.ProductStatusActive); !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("expected ErrProductNotFound, got %v", err)
	}
	if err := svc.ToggleFeatured(999); !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("expected ErrProductNotFound, got %v", err)
	}
}
