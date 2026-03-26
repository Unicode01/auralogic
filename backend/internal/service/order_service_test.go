package service

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"auralogic/internal/config"
	"auralogic/internal/models"
	"auralogic/internal/pkg/bizerr"
	"auralogic/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newOrderServiceTestDB(t *testing.T) (*OrderService, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Product{}, &models.Order{}, &models.UserPurchaseStat{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	cfg := &config.Config{}
	cfg.Order.MaxOrderItems = 20
	cfg.Order.MaxItemQuantity = 10
	cfg.Form.ExpireHours = 24

	svc := NewOrderService(
		repository.NewOrderRepository(db),
		repository.NewUserRepository(db),
		repository.NewProductRepository(db),
		repository.NewInventoryRepository(db),
		nil,
		nil,
		nil,
		nil,
		cfg,
		nil,
	)
	return svc, db
}

func requireOrderBizErr(t *testing.T, err error, key string) *bizerr.Error {
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

func TestCreateAdminOrderReturnsBizErrors(t *testing.T) {
	svc, _ := newOrderServiceTestDB(t)

	baseItem := AdminOrderItem{
		SKU:         "SKU-1",
		Name:        "Demo Product",
		Quantity:    1,
		UnitPrice:   100,
		ProductType: string(models.ProductTypePhysical),
	}

	totalAmount := int64(-1)
	requireOrderBizErr(t, func() error {
		_, err := svc.CreateAdminOrder(AdminOrderRequest{
			Items:       []AdminOrderItem{baseItem},
			TotalAmount: &totalAmount,
		})
		return err
	}(), "order.totalAmountNegative")

	userID := uint(999)
	requireOrderBizErr(t, func() error {
		_, err := svc.CreateAdminOrder(AdminOrderRequest{
			UserID: &userID,
			Items:  []AdminOrderItem{baseItem},
		})
		return err
	}(), "order.userNotFound")

	requireOrderBizErr(t, func() error {
		_, err := svc.CreateAdminOrder(AdminOrderRequest{
			Items: []AdminOrderItem{{
				SKU:         "VSKU-1",
				Name:        "Virtual Demo",
				Quantity:    1,
				UnitPrice:   100,
				ProductType: string(models.ProductTypeVirtual),
			}},
		})
		return err
	}(), "order.virtualInventoryRequired")

	requireOrderBizErr(t, func() error {
		_, err := svc.CreateAdminOrder(AdminOrderRequest{
			Items:  []AdminOrderItem{baseItem},
			Status: "invalid_status",
		})
		return err
	}(), "order.statusInvalid")
}

func TestCreateAdminOrderProductNotFoundReturnsBizError(t *testing.T) {
	svc, _ := newOrderServiceTestDB(t)

	requireOrderBizErr(t, func() error {
		_, err := svc.CreateAdminOrder(AdminOrderRequest{
			Items: []AdminOrderItem{{
				SKU:       "MISSING-SKU",
				Quantity:  1,
				UnitPrice: 100,
			}},
		})
		return err
	}(), "order.productNotFound")
}

func TestCreateDraftAttributesTooManyReturnsBizError(t *testing.T) {
	svc, _ := newOrderServiceTestDB(t)

	attrs := make(map[string]interface{}, maxAttributeKeys+1)
	for i := 0; i < maxAttributeKeys+1; i++ {
		attrs[fmt.Sprintf("k%d", i)] = "v"
	}

	requireOrderBizErr(t, func() error {
		_, err := svc.CreateDraft([]models.OrderItem{{
			SKU:        "SKU-ATTR",
			Quantity:   1,
			Attributes: attrs,
		}}, "", "", "", "", "", "")
		return err
	}(), "order.attributesTooMany")
}

func createOrderServiceTestOrder(t *testing.T, db *gorm.DB, orderNo string, status models.OrderStatus) models.Order {
	t.Helper()

	order := models.Order{
		OrderNo: orderNo,
		Status:  status,
		Items: []models.OrderItem{{
			SKU:         "SKU-TEST",
			Name:        "Demo",
			Quantity:    1,
			ProductType: models.ProductTypePhysical,
		}},
		TotalAmount: 100,
		Currency:    "CNY",
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("create order: %v", err)
	}
	return order
}

func TestOrderStateOperationsReturnBizErrors(t *testing.T) {
	svc, db := newOrderServiceTestDB(t)

	assignOrder := createOrderServiceTestOrder(t, db, "ORD-ASSIGN", models.OrderStatusDraft)
	requireOrderBizErr(t, svc.AssignTracking(assignOrder.ID, "TRACK-1"), "order.assignTrackingStatusInvalid")

	completeOrder := createOrderServiceTestOrder(t, db, "ORD-COMPLETE", models.OrderStatusPending)
	requireOrderBizErr(t, svc.CompleteOrder(completeOrder.ID, 1, "", ""), "order.completeStatusInvalid")

	cancelOrder := createOrderServiceTestOrder(t, db, "ORD-CANCEL", models.OrderStatusShipped)
	requireOrderBizErr(t, svc.CancelOrder(cancelOrder.ID, "reason"), "order.cancelStatusInvalid")

	deleteOrder := createOrderServiceTestOrder(t, db, "ORD-DELETE", models.OrderStatusPending)
	requireOrderBizErr(t, svc.DeleteOrder(deleteOrder.ID), "order.deleteStatusInvalid")

	markPaidOrder := createOrderServiceTestOrder(t, db, "ORD-PAID", models.OrderStatusDraft)
	requireOrderBizErr(t, svc.MarkAsPaidWithOptions(markPaidOrder.ID, MarkAsPaidOptions{}), "order.markPaidStatusInvalid")

	deliverOrder := createOrderServiceTestOrder(t, db, "ORD-DELIVER", models.OrderStatusDraft)
	requireOrderBizErr(t, svc.DeliverVirtualStock(deliverOrder.ID, 1, false), "order.deliverVirtualStatusInvalid")
}

func TestDeliverVirtualStockReturnsServiceUnavailableBizError(t *testing.T) {
	svc, db := newOrderServiceTestDB(t)
	order := createOrderServiceTestOrder(t, db, "ORD-VIRTUAL-SVC", models.OrderStatusPending)

	requireOrderBizErr(t, svc.DeliverVirtualStock(order.ID, 1, false), "order.virtualServiceUnavailable")
}

func TestAssignTrackingReturnsOrderNotFoundBizError(t *testing.T) {
	svc, _ := newOrderServiceTestDB(t)

	requireOrderBizErr(t, svc.AssignTracking(9999, "TRACK-404"), "order.notFound")
}
