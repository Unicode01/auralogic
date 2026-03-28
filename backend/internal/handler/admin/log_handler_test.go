package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"auralogic/internal/models"
	"auralogic/internal/pkg/response"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type listOperationLogsResponse struct {
	Code int `json:"code"`
	Data struct {
		Items      []models.OperationLog `json:"items"`
		Pagination response.Pagination   `json:"pagination"`
	} `json:"data"`
}

func newLogHandlerTestDeps(t *testing.T) (*LogHandler, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if err := db.AutoMigrate(&models.User{}, &models.Order{}, &models.OperationLog{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	return NewLogHandler(db, nil), db
}

func performListOperationLogsRequest(
	t *testing.T,
	handler *LogHandler,
	target string,
) listOperationLogsResponse {
	t.Helper()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, target, nil)

	handler.ListOperationLogs(ctx)

	var resp listOperationLogsResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

func TestListOperationLogsFiltersByResourceID(t *testing.T) {
	handler, db := newLogHandlerTestDeps(t)

	resourceIDOne := uint(101)
	resourceIDTwo := uint(202)
	logs := []models.OperationLog{
		{
			Action:       "update",
			ResourceType: "order",
			ResourceID:   &resourceIDOne,
			IPAddress:    "127.0.0.1",
		},
		{
			Action:       "update",
			ResourceType: "order",
			ResourceID:   &resourceIDTwo,
			IPAddress:    "127.0.0.2",
		},
	}
	if err := db.Create(&logs).Error; err != nil {
		t.Fatalf("create logs: %v", err)
	}

	resp := performListOperationLogsRequest(
		t,
		handler,
		"/api/admin/logs/operations?resource_id=202",
	)

	if resp.Code != response.CodeSuccess {
		t.Fatalf("expected success code, got %d", resp.Code)
	}
	if len(resp.Data.Items) != 1 {
		t.Fatalf("expected 1 log, got %d", len(resp.Data.Items))
	}
	if resp.Data.Items[0].ResourceID == nil || *resp.Data.Items[0].ResourceID != resourceIDTwo {
		t.Fatalf("expected resource id %d, got %#v", resourceIDTwo, resp.Data.Items[0].ResourceID)
	}
	if resp.Data.Pagination.Total != 1 {
		t.Fatalf("expected pagination total 1, got %d", resp.Data.Pagination.Total)
	}
}

func TestListOperationLogsFiltersByOrderNo(t *testing.T) {
	handler, db := newLogHandlerTestDeps(t)

	order := models.Order{
		OrderNo: "ORD-1001",
		Status:  models.OrderStatusPendingPayment,
		Items: []models.OrderItem{
			{SKU: "SKU-1", Name: "Test Product", Quantity: 1},
		},
		TotalAmount: 1000,
		Currency:    "CNY",
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("create order: %v", err)
	}

	orderResourceID := order.ID
	otherResourceID := uint(999)
	logs := []models.OperationLog{
		{
			Action:       "update_price",
			ResourceType: "order",
			ResourceID:   &orderResourceID,
			Details: map[string]interface{}{
				"message": "no embedded order number",
			},
			IPAddress: "10.0.0.1",
		},
		{
			Action:       "payment_success",
			ResourceType: "payment",
			ResourceID:   &otherResourceID,
			Details: map[string]interface{}{
				"order_no": "ORD-1001",
			},
			IPAddress: "10.0.0.2",
		},
		{
			Action:       "payment_success",
			ResourceType: "payment",
			ResourceID:   &otherResourceID,
			Details: map[string]interface{}{
				"order_no": "ORD-9999",
			},
			IPAddress: "10.0.0.3",
		},
	}
	if err := db.Create(&logs).Error; err != nil {
		t.Fatalf("create logs: %v", err)
	}

	resp := performListOperationLogsRequest(
		t,
		handler,
		"/api/admin/logs/operations?order_no=ORD-1001",
	)

	if resp.Code != response.CodeSuccess {
		t.Fatalf("expected success code, got %d", resp.Code)
	}
	if len(resp.Data.Items) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(resp.Data.Items))
	}
	if resp.Data.Pagination.Total != 2 {
		t.Fatalf("expected pagination total 2, got %d", resp.Data.Pagination.Total)
	}

	matchedIDs := make(map[uint]struct{}, len(resp.Data.Items))
	for _, item := range resp.Data.Items {
		matchedIDs[item.ID] = struct{}{}
	}
	if _, ok := matchedIDs[logs[0].ID]; !ok {
		t.Fatalf("expected order resource log %d to match order_no filter", logs[0].ID)
	}
	if _, ok := matchedIDs[logs[1].ID]; !ok {
		t.Fatalf("expected details order_no log %d to match order_no filter", logs[1].ID)
	}
	if _, ok := matchedIDs[logs[2].ID]; ok {
		t.Fatalf("did not expect unmatched order_no log %d in result", logs[2].ID)
	}
}
