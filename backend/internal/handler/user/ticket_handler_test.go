package user

import (
	"bytes"
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

func newUserTicketHandlerTestDB(t *testing.T) *gorm.DB {
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

func performUserTicketRequest(
	t *testing.T,
	handlerFunc func(*gin.Context),
	method string,
	target string,
	pathParams gin.Params,
	body any,
	userID uint,
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
	ctx.Set("user_id", userID)

	handlerFunc(ctx)

	var resp response.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

func userTicketErrorKey(t *testing.T, data interface{}) string {
	t.Helper()

	payload, ok := data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected response data object, got %#v", data)
	}
	key, _ := payload["error_key"].(string)
	return key
}

func TestUserTicketCreateInvalidPriorityReturnsBizError(t *testing.T) {
	db := newUserTicketHandlerTestDB(t)
	handler := NewTicketHandler(db, nil, nil)

	resp := performUserTicketRequest(
		t,
		handler.CreateTicket,
		http.MethodPost,
		"/tickets",
		nil,
		map[string]any{
			"subject":  "subject",
			"content":  "content",
			"priority": "invalid-priority",
		},
		1,
	)

	if resp.Code != response.CodeBusinessError {
		t.Fatalf("expected business error code, got %d", resp.Code)
	}
	if key := userTicketErrorKey(t, resp.Data); key != "ticket.priorityInvalid" {
		t.Fatalf("expected ticket.priorityInvalid, got %q", key)
	}
}

func TestUserTicketSendMessageClosedReturnsBizError(t *testing.T) {
	db := newUserTicketHandlerTestDB(t)
	handler := NewTicketHandler(db, nil, nil)

	ticket := models.Ticket{
		TicketNo: "TK-USER-CLOSED",
		UserID:   7,
		Subject:  "Closed ticket",
		Content:  "content",
		Status:   models.TicketStatusClosed,
		Priority: models.TicketPriorityNormal,
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	resp := performUserTicketRequest(
		t,
		handler.SendMessage,
		http.MethodPost,
		fmt.Sprintf("/tickets/%d/messages", ticket.ID),
		gin.Params{{Key: "id", Value: fmt.Sprintf("%d", ticket.ID)}},
		map[string]any{"content": "hello"},
		7,
	)

	if resp.Code != response.CodeBusinessError {
		t.Fatalf("expected business error code, got %d", resp.Code)
	}
	if key := userTicketErrorKey(t, resp.Data); key != "ticket.closedCannotSend" {
		t.Fatalf("expected ticket.closedCannotSend, got %q", key)
	}
}

func TestUserTicketUpdateInvalidStatusReturnsBizError(t *testing.T) {
	db := newUserTicketHandlerTestDB(t)
	handler := NewTicketHandler(db, nil, nil)

	ticket := models.Ticket{
		TicketNo: "TK-USER-UPDATE",
		UserID:   9,
		Subject:  "Update ticket",
		Content:  "content",
		Status:   models.TicketStatusOpen,
		Priority: models.TicketPriorityNormal,
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	resp := performUserTicketRequest(
		t,
		handler.UpdateTicketStatus,
		http.MethodPatch,
		fmt.Sprintf("/tickets/%d/status", ticket.ID),
		gin.Params{{Key: "id", Value: fmt.Sprintf("%d", ticket.ID)}},
		map[string]any{"status": "processing"},
		9,
	)

	if resp.Code != response.CodeBusinessError {
		t.Fatalf("expected business error code, got %d", resp.Code)
	}
	if key := userTicketErrorKey(t, resp.Data); key != "ticket.statusInvalid" {
		t.Fatalf("expected ticket.statusInvalid, got %q", key)
	}
}
