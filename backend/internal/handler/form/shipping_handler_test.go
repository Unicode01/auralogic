package form

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

func newShippingHandlerTestEnv(t *testing.T) (*ShippingHandler, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Order{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	previousDB := database.DB
	database.DB = db
	t.Cleanup(func() {
		database.DB = previousDB
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	cfg := &config.Config{
		App: config.AppConfig{
			URL: "http://localhost:3000",
		},
		Form: config.FormConfig{
			ExpireHours: 24,
		},
		Security: config.SecurityConfig{
			PasswordPolicy: config.PasswordPolicyConfig{
				MinLength: 8,
			},
		},
	}

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

	return NewShippingHandler(orderService, cfg), db
}

func decodeShippingResponse(t *testing.T, recorder *httptest.ResponseRecorder) response.Response {
	t.Helper()

	var resp response.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

func TestGetFormRejectsExpiredToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := newShippingHandlerTestEnv(t)

	token := "expired-token"
	expiresAt := time.Now().Add(-time.Hour)
	order := &models.Order{
		OrderNo:       "ORD-EXPIRED",
		Status:        models.OrderStatusDraft,
		Items:         []models.OrderItem{},
		FormToken:     &token,
		FormExpiresAt: &expiresAt,
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("create order: %v", err)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/form/shipping?token="+token, nil)

	handler.GetForm(ctx)

	resp := decodeShippingResponse(t, recorder)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, recorder.Code)
	}
	if resp.Code != response.CodeNotFound {
		t.Fatalf("expected response code %d, got %d", response.CodeNotFound, resp.Code)
	}
}

func TestGetFormRejectsMismatchedAuthenticatedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := newShippingHandlerTestEnv(t)

	owner := &models.User{
		UUID:         "owner-uuid",
		Email:        "owner@example.com",
		Name:         "Owner",
		PasswordHash: "hash",
		Role:         "user",
		IsActive:     true,
	}
	if err := db.Create(owner).Error; err != nil {
		t.Fatalf("create owner: %v", err)
	}

	token := "owner-token"
	order := &models.Order{
		OrderNo:   "ORD-OWNER",
		Status:    models.OrderStatusDraft,
		Items:     []models.OrderItem{},
		FormToken: &token,
		UserID:    &owner.ID,
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("create order: %v", err)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/form/shipping?token="+token, nil)
	ctx.Set("user_id", owner.ID+1)

	handler.GetForm(ctx)

	resp := decodeShippingResponse(t, recorder)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, recorder.Code)
	}
	if resp.Code != response.CodeForbidden {
		t.Fatalf("expected response code %d, got %d", response.CodeForbidden, resp.Code)
	}
}

func TestSubmitFormRejectsMismatchedAuthenticatedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := newShippingHandlerTestEnv(t)

	owner := &models.User{
		UUID:         "submit-owner-uuid",
		Email:        "owner@example.com",
		Name:         "Owner",
		PasswordHash: "hash",
		Role:         "user",
		IsActive:     true,
	}
	if err := db.Create(owner).Error; err != nil {
		t.Fatalf("create owner: %v", err)
	}

	token := "submit-owner-token"
	order := &models.Order{
		OrderNo:   "ORD-SUBMIT",
		Status:    models.OrderStatusDraft,
		Items:     []models.OrderItem{},
		FormToken: &token,
		UserID:    &owner.ID,
		UserEmail: owner.Email,
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("create order: %v", err)
	}

	payload, err := json.Marshal(map[string]interface{}{
		"form_token":        token,
		"receiver_name":     "Receiver",
		"receiver_phone":    "13800138000",
		"receiver_email":    "owner@example.com",
		"receiver_country":  "CN",
		"receiver_province": "Shanghai",
		"receiver_city":     "Shanghai",
		"receiver_district": "Pudong",
		"receiver_address":  "No. 1 Test Road",
		"receiver_postcode": "200000",
		"privacy_protected": false,
		"user_remark":       "remark",
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/form/shipping", bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("user_id", owner.ID+1)

	handler.SubmitForm(ctx)

	resp := decodeShippingResponse(t, recorder)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, recorder.Code)
	}
	if resp.Code != response.CodeForbidden {
		t.Fatalf("expected response code %d, got %d", response.CodeForbidden, resp.Code)
	}
}
