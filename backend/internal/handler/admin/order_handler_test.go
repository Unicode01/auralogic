package admin

import (
	"fmt"
	"net/http"
	"testing"

	"auralogic/internal/config"
	"auralogic/internal/database"
	"auralogic/internal/models"
	"auralogic/internal/pkg/response"
	"auralogic/internal/repository"
	"auralogic/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newOrderHandlerTestDeps(t *testing.T) (*OrderHandler, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if err := db.AutoMigrate(
		&models.User{},
		&models.AdminPermission{},
		&models.Order{},
		&models.OrderPaymentMethod{},
		&models.PaymentMethod{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	oldDB := database.DB
	database.DB = db
	t.Cleanup(func() {
		database.DB = oldDB
	})

	cfg := &config.Config{}
	cfg.Form.ExpireHours = 24

	orderService := service.NewOrderService(
		repository.NewOrderRepository(db),
		repository.NewUserRepository(db),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		cfg,
		nil,
	)

	handler := NewOrderHandler(orderService, nil, nil, nil, nil, cfg)
	return handler, db
}

func createOrderForHandlerTest(t *testing.T, db *gorm.DB, status models.OrderStatus) models.Order {
	t.Helper()

	order := models.Order{
		OrderNo: fmt.Sprintf("ORD-%s", t.Name()),
		Status:  status,
		Items: []models.OrderItem{
			{SKU: "SKU-1", Name: "Test Product", Quantity: 1},
		},
		TotalAmount: 1000,
		Currency:    "CNY",
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("create order: %v", err)
	}
	return order
}

func TestGetOrderInvalidOrderIDReturnsBizError(t *testing.T) {
	handler, _ := newOrderHandlerTestDeps(t)

	resp := performAdminUserRequest(
		t,
		handler.GetOrder,
		http.MethodGet,
		"/admin/orders/invalid",
		gin.Params{{Key: "id", Value: "invalid"}},
		nil,
		1,
	)

	if resp.Code != response.CodeBusinessError {
		t.Fatalf("expected business error code, got %d", resp.Code)
	}
	if key := adminErrorKey(t, resp.Data); key != "order.invalidOrderID" {
		t.Fatalf("expected order.invalidOrderID, got %q", key)
	}
}

func TestUpdateOrderPriceInvalidRequestReturnsBizError(t *testing.T) {
	handler, db := newOrderHandlerTestDeps(t)
	order := createOrderForHandlerTest(t, db, models.OrderStatusPendingPayment)

	resp := performAdminUserRequest(
		t,
		handler.UpdateOrderPrice,
		http.MethodPost,
		fmt.Sprintf("/admin/orders/%d/update-price", order.ID),
		gin.Params{{Key: "id", Value: fmt.Sprintf("%d", order.ID)}},
		map[string]any{},
		1,
	)

	if resp.Code != response.CodeBusinessError {
		t.Fatalf("expected business error code, got %d", resp.Code)
	}
	if key := adminErrorKey(t, resp.Data); key != "order.invalidRequestParameters" {
		t.Fatalf("expected order.invalidRequestParameters, got %q", key)
	}
}

func TestUpdateOrderPriceInvalidStatusReturnsBizError(t *testing.T) {
	handler, db := newOrderHandlerTestDeps(t)
	order := createOrderForHandlerTest(t, db, models.OrderStatusPending)

	resp := performAdminUserRequest(
		t,
		handler.UpdateOrderPrice,
		http.MethodPost,
		fmt.Sprintf("/admin/orders/%d/update-price", order.ID),
		gin.Params{{Key: "id", Value: fmt.Sprintf("%d", order.ID)}},
		map[string]any{"total_amount_minor": 2000},
		1,
	)

	if resp.Code != response.CodeBusinessError {
		t.Fatalf("expected business error code, got %d", resp.Code)
	}
	if key := adminErrorKey(t, resp.Data); key != "order.updatePriceStatusInvalid" {
		t.Fatalf("expected order.updatePriceStatusInvalid, got %q", key)
	}
}

func TestUpdateShippingInfoInvalidPhoneCodeReturnsBizError(t *testing.T) {
	handler, db := newOrderHandlerTestDeps(t)
	order := createOrderForHandlerTest(t, db, models.OrderStatusPending)

	resp := performAdminUserRequest(
		t,
		handler.UpdateShippingInfo,
		http.MethodPatch,
		fmt.Sprintf("/admin/orders/%d/shipping", order.ID),
		gin.Params{{Key: "id", Value: fmt.Sprintf("%d", order.ID)}},
		map[string]any{"phone_code": "abc"},
		1,
	)

	if resp.Code != response.CodeBusinessError {
		t.Fatalf("expected business error code, got %d", resp.Code)
	}
	if key := adminErrorKey(t, resp.Data); key != "order.phoneCodeInvalid" {
		t.Fatalf("expected order.phoneCodeInvalid, got %q", key)
	}
}

func TestRequestResubmitInvalidStatusReturnsBizError(t *testing.T) {
	handler, db := newOrderHandlerTestDeps(t)
	order := createOrderForHandlerTest(t, db, models.OrderStatusCompleted)

	resp := performAdminUserRequest(
		t,
		handler.RequestResubmit,
		http.MethodPost,
		fmt.Sprintf("/admin/orders/%d/request-resubmit", order.ID),
		gin.Params{{Key: "id", Value: fmt.Sprintf("%d", order.ID)}},
		map[string]any{"reason": "Need update"},
		1,
	)

	if resp.Code != response.CodeBusinessError {
		t.Fatalf("expected business error code, got %d", resp.Code)
	}
	if key := adminErrorKey(t, resp.Data); key != "order.resubmitStatusInvalid" {
		t.Fatalf("expected order.resubmitStatusInvalid, got %q", key)
	}
}

func TestRefundOrderMissingPaymentMethodReturnsBizError(t *testing.T) {
	handler, db := newOrderHandlerTestDeps(t)
	order := createOrderForHandlerTest(t, db, models.OrderStatusPending)

	resp := performAdminUserRequest(
		t,
		handler.RefundOrder,
		http.MethodPost,
		fmt.Sprintf("/admin/orders/%d/refund", order.ID),
		gin.Params{{Key: "id", Value: fmt.Sprintf("%d", order.ID)}},
		map[string]any{"reason": "Customer requested"},
		1,
	)

	if resp.Code != response.CodeBusinessError {
		t.Fatalf("expected business error code, got %d", resp.Code)
	}
	if key := adminErrorKey(t, resp.Data); key != "order.orderPaymentMethodNotFound" {
		t.Fatalf("expected order.orderPaymentMethodNotFound, got %q", key)
	}
}

func TestCreateOrderForUserEmptyItemsReturnsBizError(t *testing.T) {
	handler, _ := newOrderHandlerTestDeps(t)

	resp := performAdminUserRequest(
		t,
		handler.CreateOrderForUser,
		http.MethodPost,
		"/admin/orders/create-for-user",
		nil,
		map[string]any{"items": []any{}},
		1,
	)

	if resp.Code != response.CodeBusinessError {
		t.Fatalf("expected business error code, got %d", resp.Code)
	}
	if key := adminErrorKey(t, resp.Data); key != "order.itemsEmpty" {
		t.Fatalf("expected order.itemsEmpty, got %q", key)
	}
}

func TestBatchUpdateOrdersLimitReturnsBizError(t *testing.T) {
	handler, _ := newOrderHandlerTestDeps(t)

	orderIDs := make([]uint, 101)
	for i := range orderIDs {
		orderIDs[i] = uint(i + 1)
	}

	resp := performAdminUserRequest(
		t,
		handler.BatchUpdateOrders,
		http.MethodPost,
		"/admin/orders/batch",
		nil,
		map[string]any{
			"order_ids": orderIDs,
			"action":    "complete",
		},
		1,
	)

	if resp.Code != response.CodeBusinessError {
		t.Fatalf("expected business error code, got %d", resp.Code)
	}
	if key := adminErrorKey(t, resp.Data); key != "order.batchLimitExceeded" {
		t.Fatalf("expected order.batchLimitExceeded, got %q", key)
	}
}
