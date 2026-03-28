package admin

import (
	"fmt"
	"net/http"
	"strings"
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
		&models.PaymentMethodStorageEntry{},
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

	jsRuntimeService := service.NewJSRuntimeService(db, cfg)
	handler := NewOrderHandler(orderService, nil, nil, jsRuntimeService, nil, cfg)
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

func TestUpdateOrderPriceClearsPaymentArtifacts(t *testing.T) {
	handler, db := newOrderHandlerTestDeps(t)
	order := createOrderForHandlerTest(t, db, models.OrderStatusPendingPayment)

	paymentMethod := models.PaymentMethod{
		Name:    "Test Payment",
		Type:    models.PaymentMethodTypeCustom,
		Enabled: true,
	}
	if err := db.Create(&paymentMethod).Error; err != nil {
		t.Fatalf("create payment method: %v", err)
	}

	if err := db.Create(&models.OrderPaymentMethod{
		OrderID:          order.ID,
		PaymentMethodID:  paymentMethod.ID,
		PaymentData:      `{"usdt_amount":"12.34"}`,
		PaymentCardCache: `{"html":"cached"}`,
	}).Error; err != nil {
		t.Fatalf("create order payment method: %v", err)
	}

	storageEntries := []models.PaymentMethodStorageEntry{
		{PaymentMethodID: paymentMethod.ID, Key: fmt.Sprintf("order_%d_amount", order.ID), Value: "12.34"},
		{PaymentMethodID: paymentMethod.ID, Key: fmt.Sprintf("order_%d_time", order.ID), Value: "123456"},
		{PaymentMethodID: paymentMethod.ID, Key: fmt.Sprintf("order_%d_address", order.ID), Value: "addr"},
	}
	if err := db.Create(&storageEntries).Error; err != nil {
		t.Fatalf("create storage entries: %v", err)
	}

	resp := performAdminUserRequest(
		t,
		handler.UpdateOrderPrice,
		http.MethodPut,
		fmt.Sprintf("/admin/orders/%d/price", order.ID),
		gin.Params{{Key: "id", Value: fmt.Sprintf("%d", order.ID)}},
		map[string]any{"total_amount_minor": 2000},
		1,
	)

	if resp.Code != response.CodeSuccess {
		t.Fatalf("expected success code, got %d", resp.Code)
	}

	var updatedOrder models.Order
	if err := db.First(&updatedOrder, order.ID).Error; err != nil {
		t.Fatalf("query order: %v", err)
	}
	if updatedOrder.TotalAmount != 2000 {
		t.Fatalf("expected total amount 2000, got %d", updatedOrder.TotalAmount)
	}

	var updatedOPM models.OrderPaymentMethod
	if err := db.Where("order_id = ?", order.ID).First(&updatedOPM).Error; err != nil {
		t.Fatalf("query order payment method: %v", err)
	}
	if updatedOPM.PaymentData != "" {
		t.Fatalf("expected payment data cleared, got %q", updatedOPM.PaymentData)
	}
	if updatedOPM.PaymentCardCache != "" {
		t.Fatalf("expected payment card cache cleared, got %q", updatedOPM.PaymentCardCache)
	}
	if updatedOPM.CacheExpiresAt != nil {
		t.Fatalf("expected cache expires at cleared, got %#v", updatedOPM.CacheExpiresAt)
	}

	var storageCount int64
	if err := db.Model(&models.PaymentMethodStorageEntry{}).
		Where("payment_method_id = ?", paymentMethod.ID).
		Count(&storageCount).Error; err != nil {
		t.Fatalf("count storage entries: %v", err)
	}
	if storageCount != 0 {
		t.Fatalf("expected payment storage cleared, got %d rows", storageCount)
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

func TestRefundOrderMarksManualRefundPending(t *testing.T) {
	handler, db := newOrderHandlerTestDeps(t)
	order := createOrderForHandlerTest(t, db, models.OrderStatusPending)

	paymentMethod := models.PaymentMethod{
		Name:    "Manual Refund",
		Type:    models.PaymentMethodTypeCustom,
		Enabled: true,
		Script: `
function onRefund() {
  return {
    success: true,
    pending: true,
    message: "manual refund required",
    data: {
      network: "manual"
    }
  };
}
`,
	}
	if err := db.Create(&paymentMethod).Error; err != nil {
		t.Fatalf("create payment method: %v", err)
	}
	if err := db.Create(&models.OrderPaymentMethod{
		OrderID:         order.ID,
		PaymentMethodID: paymentMethod.ID,
	}).Error; err != nil {
		t.Fatalf("create order payment method: %v", err)
	}

	resp := performAdminUserRequest(
		t,
		handler.RefundOrder,
		http.MethodPost,
		fmt.Sprintf("/admin/orders/%d/refund", order.ID),
		gin.Params{{Key: "id", Value: fmt.Sprintf("%d", order.ID)}},
		map[string]any{"reason": "Customer requested"},
		1,
	)

	if resp.Code != response.CodeSuccess {
		t.Fatalf("expected success code, got %d", resp.Code)
	}

	var updatedOrder models.Order
	if err := db.First(&updatedOrder, order.ID).Error; err != nil {
		t.Fatalf("query order: %v", err)
	}
	if updatedOrder.Status != models.OrderStatusRefundPending {
		t.Fatalf("expected status %q, got %q", models.OrderStatusRefundPending, updatedOrder.Status)
	}
}

func TestConfirmRefundInvalidStatusReturnsBizError(t *testing.T) {
	handler, db := newOrderHandlerTestDeps(t)
	order := createOrderForHandlerTest(t, db, models.OrderStatusCompleted)

	resp := performAdminUserRequest(
		t,
		handler.ConfirmRefund,
		http.MethodPost,
		fmt.Sprintf("/admin/orders/%d/confirm-refund", order.ID),
		gin.Params{{Key: "id", Value: fmt.Sprintf("%d", order.ID)}},
		map[string]any{"transaction_id": "REF-1"},
		1,
	)

	if resp.Code != response.CodeBusinessError {
		t.Fatalf("expected business error code, got %d", resp.Code)
	}
	if key := adminErrorKey(t, resp.Data); key != "order.refundFinalizeStatusInvalid" {
		t.Fatalf("expected order.refundFinalizeStatusInvalid, got %q", key)
	}
}

func TestConfirmRefundMarksOrderRefunded(t *testing.T) {
	handler, db := newOrderHandlerTestDeps(t)
	order := createOrderForHandlerTest(t, db, models.OrderStatusRefundPending)

	paymentMethod := models.PaymentMethod{
		Name:    "Manual Refund",
		Type:    models.PaymentMethodTypeCustom,
		Enabled: true,
	}
	if err := db.Create(&paymentMethod).Error; err != nil {
		t.Fatalf("create payment method: %v", err)
	}
	if err := db.Create(&models.OrderPaymentMethod{
		OrderID:         order.ID,
		PaymentMethodID: paymentMethod.ID,
		PaymentData:     `{"paid_at":"2026-01-01T00:00:00Z"}`,
	}).Error; err != nil {
		t.Fatalf("create order payment method: %v", err)
	}

	resp := performAdminUserRequest(
		t,
		handler.ConfirmRefund,
		http.MethodPost,
		fmt.Sprintf("/admin/orders/%d/confirm-refund", order.ID),
		gin.Params{{Key: "id", Value: fmt.Sprintf("%d", order.ID)}},
		map[string]any{
			"transaction_id": "REF-2026-0001",
			"remark":         "Refund completed manually",
		},
		1,
	)

	if resp.Code != response.CodeSuccess {
		t.Fatalf("expected success code, got %d", resp.Code)
	}

	var updatedOrder models.Order
	if err := db.First(&updatedOrder, order.ID).Error; err != nil {
		t.Fatalf("query order: %v", err)
	}
	if updatedOrder.Status != models.OrderStatusRefunded {
		t.Fatalf("expected status %q, got %q", models.OrderStatusRefunded, updatedOrder.Status)
	}
	if !strings.Contains(updatedOrder.AdminRemark, "REF-2026-0001") {
		t.Fatalf("expected admin remark to contain refund transaction id, got %q", updatedOrder.AdminRemark)
	}

	var updatedOPM models.OrderPaymentMethod
	if err := db.Where("order_id = ?", order.ID).First(&updatedOPM).Error; err != nil {
		t.Fatalf("query order payment method: %v", err)
	}
	if !strings.Contains(updatedOPM.PaymentData, "\"refund_transaction_id\":\"REF-2026-0001\"") {
		t.Fatalf("expected payment data to contain refund transaction id, got %q", updatedOPM.PaymentData)
	}
	if !strings.Contains(updatedOPM.PaymentData, "\"refund_confirm_remark\":\"Refund completed manually\"") {
		t.Fatalf("expected payment data to contain refund remark, got %q", updatedOPM.PaymentData)
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
