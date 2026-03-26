package service

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"auralogic/internal/config"
	"auralogic/internal/models"
	"auralogic/internal/pkg/bizerr"
	"auralogic/internal/pkg/jwt"
	"auralogic/internal/pkg/password"
	"auralogic/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newAuthServiceTestDB(t *testing.T) (*AuthService, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:      "test-secret",
			ExpireHours: 24,
		},
		Security: config.SecurityConfig{
			Login: config.LoginConfig{
				AllowPasswordLogin:       true,
				RequireEmailVerification: true,
			},
			PasswordPolicy: config.PasswordPolicyConfig{
				MinLength:        8,
				RequireUppercase: true,
				RequireLowercase: true,
				RequireNumber:    true,
				RequireSpecial:   true,
			},
		},
	}
	jwt.InitJWT(&cfg.JWT)
	return NewAuthService(repository.NewUserRepository(db), cfg), db
}

func requireAuthBizErr(t *testing.T, err error, key string) *bizerr.Error {
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

func TestAuthLoginReturnsBizErrors(t *testing.T) {
	svc, db := newAuthServiceTestDB(t)

	requireAuthBizErr(t, func() error {
		_, _, err := svc.Login("missing@example.com", "Password1!")
		return err
	}(), "auth.invalidEmailOrPassword")

	hash, err := password.HashPassword("Password1!")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	disabledUser := models.User{
		UUID:          "disabled-user",
		Email:         "disabled@example.com",
		PasswordHash:  hash,
		Name:          "Disabled",
		Role:          "user",
		IsActive:      true,
		EmailVerified: true,
	}
	if err := db.Create(&disabledUser).Error; err != nil {
		t.Fatalf("create disabled user: %v", err)
	}
	if err := db.Model(&models.User{}).
		Where("id = ?", disabledUser.ID).
		Update("is_active", false).Error; err != nil {
		t.Fatalf("disable user: %v", err)
	}

	requireAuthBizErr(t, func() error {
		_, _, err := svc.Login(disabledUser.Email, "Password1!")
		return err
	}(), "auth.accountDisabled")

	unverifiedUser := models.User{
		UUID:          "unverified-user",
		Email:         "unverified@example.com",
		PasswordHash:  hash,
		Name:          "Unverified",
		Role:          "user",
		IsActive:      true,
		EmailVerified: false,
	}
	if err := db.Create(&unverifiedUser).Error; err != nil {
		t.Fatalf("create unverified user: %v", err)
	}

	requireAuthBizErr(t, func() error {
		_, _, err := svc.Login(unverifiedUser.Email, "Password1!")
		return err
	}(), "auth.emailNotVerified")
}

func TestAuthRegisterAndChangePasswordReturnBizErrors(t *testing.T) {
	svc, db := newAuthServiceTestDB(t)

	existing := models.User{
		UUID:          "existing-user",
		Email:         "exists@example.com",
		Name:          "Existing",
		Role:          "user",
		IsActive:      true,
		EmailVerified: true,
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("create existing user: %v", err)
	}

	requireAuthBizErr(t, func() error {
		_, err := svc.Register("exists@example.com", "", "Duplicate", "Password1!")
		return err
	}(), "auth.emailAlreadyInUse")

	hash, err := password.HashPassword("Password1!")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	user := models.User{
		UUID:          "password-user",
		Email:         "password@example.com",
		PasswordHash:  hash,
		Name:          "Password User",
		Role:          "user",
		IsActive:      true,
		EmailVerified: true,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	requireAuthBizErr(t, svc.ChangePassword(user.ID, "wrong-password", "Password2!"), "auth.incorrectOldPassword")
}

func TestGeneratePasswordResetTokenReturnsBizErrorForDisabledUser(t *testing.T) {
	svc, db := newAuthServiceTestDB(t)

	user := models.User{
		UUID:          "reset-disabled-user",
		Email:         "reset-disabled@example.com",
		Name:          "Reset Disabled",
		Role:          "user",
		IsActive:      true,
		EmailVerified: true,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.Model(&models.User{}).
		Where("id = ?", user.ID).
		Update("is_active", false).Error; err != nil {
		t.Fatalf("disable user: %v", err)
	}

	requireAuthBizErr(t, func() error {
		_, err := svc.GeneratePasswordResetToken(user.Email)
		return err
	}(), "auth.accountDisabled")
}

func TestChangePasswordReturnsUserNotFoundBizError(t *testing.T) {
	svc, _ := newAuthServiceTestDB(t)

	requireAuthBizErr(t, svc.ChangePassword(9999, "Password1!", "Password2!"), "auth.userNotFound")
}
