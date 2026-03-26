package admin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"auralogic/internal/config"
	"auralogic/internal/models"
	"auralogic/internal/pkg/response"
	"auralogic/internal/repository"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newAdminHandlerTestDeps(t *testing.T) (*AdminHandler, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if err := db.AutoMigrate(&models.User{}, &models.AdminPermission{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	cfg := &config.Config{}
	cfg.Security.PasswordPolicy.MinLength = 8
	cfg.Security.PasswordPolicy.RequireUppercase = true
	cfg.Security.PasswordPolicy.RequireLowercase = true
	cfg.Security.PasswordPolicy.RequireNumber = true
	cfg.Security.PasswordPolicy.RequireSpecial = true

	handler := NewAdminHandler(repository.NewUserRepository(db), db, cfg)
	return handler, db
}

func performAdminUserRequest(
	t *testing.T,
	handlerFunc func(*gin.Context),
	method string,
	target string,
	params gin.Params,
	body any,
	currentUserID uint,
) response.Response {
	t.Helper()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(payload)
	}

	ctx.Request = httptest.NewRequest(method, target, reader)
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Params = params
	ctx.Set("user_id", currentUserID)

	handlerFunc(ctx)

	var resp response.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

func adminErrorKey(t *testing.T, data interface{}) string {
	t.Helper()

	payload, ok := data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected response data object, got %#v", data)
	}
	key, _ := payload["error_key"].(string)
	return key
}

func TestCreateAdminDuplicateEmailReturnsBizError(t *testing.T) {
	handler, db := newAdminHandlerTestDeps(t)

	existing := models.User{
		UUID:          uuid.NewString(),
		Email:         "duplicate@example.com",
		Name:          "Existing",
		Role:          "admin",
		IsActive:      true,
		EmailVerified: true,
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("create existing user: %v", err)
	}

	resp := performAdminUserRequest(
		t,
		handler.CreateAdmin,
		http.MethodPost,
		"/admin/admins",
		nil,
		map[string]any{
			"email":    "duplicate@example.com",
			"password": "Password1!",
			"name":     "New Admin",
		},
		999,
	)

	if resp.Code != response.CodeBusinessError {
		t.Fatalf("expected business error code, got %d", resp.Code)
	}
	if key := adminErrorKey(t, resp.Data); key != "admin.emailAlreadyInUse" {
		t.Fatalf("expected admin.emailAlreadyInUse, got %q", key)
	}
}

func TestUpdateAdminSelfRoleChangeReturnsBizError(t *testing.T) {
	handler, db := newAdminHandlerTestDeps(t)

	admin := models.User{
		UUID:          uuid.NewString(),
		Email:         "self@example.com",
		Name:          "Self Admin",
		Role:          "admin",
		IsActive:      true,
		EmailVerified: true,
	}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}

	resp := performAdminUserRequest(
		t,
		handler.UpdateAdmin,
		http.MethodPatch,
		fmt.Sprintf("/admin/admins/%d", admin.ID),
		gin.Params{{Key: "id", Value: fmt.Sprintf("%d", admin.ID)}},
		map[string]any{"role": "super_admin"},
		admin.ID,
	)

	if resp.Code != response.CodeBusinessError {
		t.Fatalf("expected business error code, got %d", resp.Code)
	}
	if key := adminErrorKey(t, resp.Data); key != "admin.cannotModifySelfRoleOrStatus" {
		t.Fatalf("expected admin.cannotModifySelfRoleOrStatus, got %q", key)
	}
}

func TestDeleteAdminSelfReturnsBizError(t *testing.T) {
	handler, db := newAdminHandlerTestDeps(t)

	admin := models.User{
		UUID:          uuid.NewString(),
		Email:         "delete-self@example.com",
		Name:          "Delete Self",
		Role:          "admin",
		IsActive:      true,
		EmailVerified: true,
	}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}

	resp := performAdminUserRequest(
		t,
		handler.DeleteAdmin,
		http.MethodDelete,
		fmt.Sprintf("/admin/admins/%d", admin.ID),
		gin.Params{{Key: "id", Value: fmt.Sprintf("%d", admin.ID)}},
		nil,
		admin.ID,
	)

	if resp.Code != response.CodeBusinessError {
		t.Fatalf("expected business error code, got %d", resp.Code)
	}
	if key := adminErrorKey(t, resp.Data); key != "admin.cannotDeleteSelf" {
		t.Fatalf("expected admin.cannotDeleteSelf, got %q", key)
	}
}

func TestCreateAdminInvalidPasswordReturnsBizError(t *testing.T) {
	handler, _ := newAdminHandlerTestDeps(t)

	resp := performAdminUserRequest(
		t,
		handler.CreateAdmin,
		http.MethodPost,
		"/admin/admins",
		nil,
		map[string]any{
			"email":    "policy@example.com",
			"password": "password1!",
			"name":     "Policy Admin",
		},
		999,
	)

	if resp.Code != response.CodeBusinessError {
		t.Fatalf("expected business error code, got %d", resp.Code)
	}
	if key := adminErrorKey(t, resp.Data); key != "password.needUppercase" {
		t.Fatalf("expected password.needUppercase, got %q", key)
	}
}
