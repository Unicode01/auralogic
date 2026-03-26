package admin

import (
	"fmt"
	"net/http"
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

func newUserHandlerTestDeps(t *testing.T) (*UserHandler, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if err := db.AutoMigrate(&models.User{}, &models.AdminPermission{}, &models.Order{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	cfg := &config.Config{}
	cfg.Security.PasswordPolicy.MinLength = 8
	cfg.Security.PasswordPolicy.RequireUppercase = true
	cfg.Security.PasswordPolicy.RequireLowercase = true
	cfg.Security.PasswordPolicy.RequireNumber = true
	cfg.Security.PasswordPolicy.RequireSpecial = true

	handler := NewUserHandler(repository.NewUserRepository(db), db, cfg, nil)
	return handler, db
}

func TestCreateUserInvalidPasswordReturnsBizError(t *testing.T) {
	handler, _ := newUserHandlerTestDeps(t)

	resp := performAdminUserRequest(
		t,
		handler.CreateUser,
		http.MethodPost,
		"/admin/users",
		nil,
		map[string]any{
			"email":    "user-policy@example.com",
			"password": "password1!",
			"name":     "Policy User",
			"role":     "user",
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

func TestUpdateUserInvalidPasswordReturnsBizError(t *testing.T) {
	handler, db := newUserHandlerTestDeps(t)

	user := models.User{
		UUID:          uuid.NewString(),
		Email:         "update-user@example.com",
		Name:          "Update User",
		Role:          "user",
		IsActive:      true,
		EmailVerified: true,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	resp := performAdminUserRequest(
		t,
		handler.UpdateUser,
		http.MethodPatch,
		fmt.Sprintf("/admin/users/%d", user.ID),
		gin.Params{{Key: "id", Value: fmt.Sprintf("%d", user.ID)}},
		map[string]any{"password": "password1!"},
		999,
	)

	if resp.Code != response.CodeBusinessError {
		t.Fatalf("expected business error code, got %d", resp.Code)
	}
	if key := adminErrorKey(t, resp.Data); key != "password.needUppercase" {
		t.Fatalf("expected password.needUppercase, got %q", key)
	}
}
