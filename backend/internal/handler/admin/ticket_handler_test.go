package admin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"auralogic/internal/models"
	"auralogic/internal/pkg/response"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newAdminTicketHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Ticket{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func performAdminTicketRequest(
	t *testing.T,
	handlerFunc func(*gin.Context),
	method string,
	target string,
	pathParams gin.Params,
	body any,
) response.Response {
	t.Helper()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	var requestBody *bytes.Reader
	if body == nil {
		requestBody = bytes.NewReader(nil)
	} else {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		requestBody = bytes.NewReader(payload)
	}

	ctx.Request = httptest.NewRequest(method, target, requestBody)
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Params = pathParams
	ctx.Set("user_id", uint(100))

	handlerFunc(ctx)

	var resp response.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

func adminTicketErrorKey(t *testing.T, data interface{}) string {
	t.Helper()

	payload, ok := data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected response data object, got %#v", data)
	}
	key, _ := payload["error_key"].(string)
	return key
}

func TestAdminTicketSendMessageClosedReturnsBizError(t *testing.T) {
	db := newAdminTicketHandlerTestDB(t)
	handler := NewTicketHandler(db, nil, nil)

	ticket := models.Ticket{
		TicketNo: "TK-ADMIN-CLOSED",
		UserID:   1,
		Subject:  "Closed ticket",
		Content:  "content",
		Status:   models.TicketStatusClosed,
		Priority: models.TicketPriorityNormal,
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	resp := performAdminTicketRequest(
		t,
		handler.SendMessage,
		http.MethodPost,
		fmt.Sprintf("/admin/tickets/%d/messages", ticket.ID),
		gin.Params{{Key: "id", Value: fmt.Sprintf("%d", ticket.ID)}},
		map[string]any{"content": "hello"},
	)

	if resp.Code != response.CodeBusinessError {
		t.Fatalf("expected business error code, got %d", resp.Code)
	}
	if key := adminTicketErrorKey(t, resp.Data); key != "ticket.closedCannotSend" {
		t.Fatalf("expected ticket.closedCannotSend, got %q", key)
	}
}

func TestAdminTicketUpdateInvalidStatusReturnsBizError(t *testing.T) {
	db := newAdminTicketHandlerTestDB(t)
	handler := NewTicketHandler(db, nil, nil)

	ticket := models.Ticket{
		TicketNo: "TK-ADMIN-UPDATE",
		UserID:   1,
		Subject:  "Update ticket",
		Content:  "content",
		Status:   models.TicketStatusOpen,
		Priority: models.TicketPriorityNormal,
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	resp := performAdminTicketRequest(
		t,
		handler.UpdateTicket,
		http.MethodPatch,
		fmt.Sprintf("/admin/tickets/%d", ticket.ID),
		gin.Params{{Key: "id", Value: fmt.Sprintf("%d", ticket.ID)}},
		map[string]any{"status": "bad-status"},
	)

	if resp.Code != response.CodeBusinessError {
		t.Fatalf("expected business error code, got %d", resp.Code)
	}
	if key := adminTicketErrorKey(t, resp.Data); key != "ticket.statusInvalid" {
		t.Fatalf("expected ticket.statusInvalid, got %q", key)
	}
}

func TestAdminTicketUpdateClearsClosedAtWhenReopened(t *testing.T) {
	db := newAdminTicketHandlerTestDB(t)
	handler := NewTicketHandler(db, nil, nil)

	closedAt := time.Now().Add(-time.Hour)
	ticket := models.Ticket{
		TicketNo: "TK-ADMIN-REOPEN",
		UserID:   1,
		Subject:  "Resolved ticket",
		Content:  "content",
		Status:   models.TicketStatusResolved,
		Priority: models.TicketPriorityHigh,
		ClosedAt: &closedAt,
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	resp := performAdminTicketRequest(
		t,
		handler.UpdateTicket,
		http.MethodPatch,
		fmt.Sprintf("/admin/tickets/%d", ticket.ID),
		gin.Params{{Key: "id", Value: fmt.Sprintf("%d", ticket.ID)}},
		map[string]any{"status": "open", "priority": "urgent"},
	)

	if resp.Code != response.CodeSuccess {
		t.Fatalf("expected success code, got %d", resp.Code)
	}

	var updated models.Ticket
	if err := db.First(&updated, ticket.ID).Error; err != nil {
		t.Fatalf("reload ticket: %v", err)
	}
	if updated.Status != models.TicketStatusOpen {
		t.Fatalf("expected status open, got %s", updated.Status)
	}
	if updated.Priority != models.TicketPriorityUrgent {
		t.Fatalf("expected priority urgent, got %s", updated.Priority)
	}
	if updated.ClosedAt != nil {
		t.Fatalf("expected closed_at to be cleared, got %#v", updated.ClosedAt)
	}
}
