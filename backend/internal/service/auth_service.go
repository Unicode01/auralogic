package service

import (
	crand "crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
	"auralogic/internal/config"
	"auralogic/internal/models"
	"auralogic/internal/pkg/cache"
	"auralogic/internal/pkg/jwt"
	"auralogic/internal/pkg/password"
	"auralogic/internal/repository"
	"gorm.io/gorm"
)

type AuthService struct {
	userRepo *repository.UserRepository
	cfg      *config.Config
}

var (
	// Public, user-facing errors (safe to show to clients).
	ErrEmailAlreadyInUse = errors.New("Email already in use")
	ErrPhoneAlreadyInUse = errors.New("Phone number already in use")

	// Internal marker for handlers to avoid leaking DB errors.
	ErrRegisterInternal = errors.New("REGISTER_INTERNAL")
)

func NewAuthService(userRepo *repository.UserRepository, cfg *config.Config) *AuthService {
	return &AuthService{
		userRepo: userRepo,
		cfg:      cfg,
	}
}

// Login 用户登录
func (s *AuthService) Login(email, pwd string) (string, *models.User, error) {
	email = normalizeEmail(email)
	// 查找用户
	user, err := s.userRepo.FindByEmail(email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil, errors.New("Invalid email or password")
		}
		return "", nil, err
	}

	// 检查密码登录是否禁用
	if !s.cfg.Security.Login.AllowPasswordLogin && !user.IsSuperAdmin() {
		return "", nil, errors.New("Password login is disabled, please use quick login or OAuth login")
	}

	// 验证密码
	if !password.CheckPassword(pwd, user.PasswordHash) {
		return "", nil, errors.New("Invalid email or password")
	}

	// 检查用户状态
	if !user.IsActive {
		return "", nil, errors.New("User account has been disabled")
	}

	// 检查邮箱是否已验证（管理员跳过）
	if !user.EmailVerified && !user.IsAdmin() && s.cfg.Security.Login.RequireEmailVerification {
		return "", nil, errors.New("EMAIL_NOT_VERIFIED")
	}

	// 生成JWT Token
	token, err := jwt.GenerateToken(user.ID, user.Email, user.Role, s.cfg.JWT.ExpireHours)
	if err != nil {
		return "", nil, err
	}

	// 更新最后登录时间
	now := models.NowFunc()
	user.LastLoginAt = &now
	s.userRepo.Update(user)

	return token, user, nil
}

// Register 注册用户（自动创建）
func (s *AuthService) Register(email, phone, name, pwd string) (*models.User, error) {
	email = normalizeEmail(email)
	name = strings.TrimSpace(name)

	// 检查邮箱是否已存在
	if email != "" {
		if _, err := s.userRepo.FindByEmail(email); err == nil {
			return nil, ErrEmailAlreadyInUse
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: failed to check email uniqueness: %v", ErrRegisterInternal, err)
		}
	}

	// 检查手机号是否已存在
	if phone != "" {
		if _, err := s.userRepo.FindByPhone(phone); err == nil {
			return nil, ErrPhoneAlreadyInUse
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: failed to check phone uniqueness: %v", ErrRegisterInternal, err)
		}
	}

	// 生成密码（如果未提供）
	if pwd == "" {
		var err error
		pwd, err = password.GenerateRandomPassword(12)
		if err != nil {
			return nil, err
		}
	}

	// 验证密码策略
	policy := s.cfg.Security.PasswordPolicy
	if err := password.ValidatePasswordPolicy(pwd, policy.MinLength, policy.RequireUppercase,
		policy.RequireLowercase, policy.RequireNumber, policy.RequireSpecial); err != nil {
		return nil, err
	}

	// 哈希密码
	hashedPassword, err := password.HashPassword(pwd)
	if err != nil {
		return nil, err
	}

	// 创建用户
	user := &models.User{
		UUID:          uuid.New().String(),
		Email:         email,
		Name:          name,
		PasswordHash:  hashedPassword,
		Role:          "user",
		IsActive:      true,
		EmailVerified: false,
	}

	// 只有手机号不为空时才设置
	if phone != "" {
		user.Phone = &phone
	}

	if err := s.userRepo.Create(user); err != nil {
		// A concurrent registration can still hit unique constraints.
		if isUniqueConstraintError(err) {
			msg := strings.ToLower(err.Error())
			switch {
			case strings.Contains(msg, "email"):
				return nil, ErrEmailAlreadyInUse
			case strings.Contains(msg, "phone"):
				return nil, ErrPhoneAlreadyInUse
			default:
				// Fall back to a generic conflict to avoid leaking DB internals.
				return nil, ErrEmailAlreadyInUse
			}
		}
		return nil, fmt.Errorf("%w: failed to create user: %v", ErrRegisterInternal, err)
	}

	return user, nil
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	// Portable-ish detection across sqlite/postgres/mysql.
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint") ||
		strings.Contains(msg, "duplicate key") ||
		strings.Contains(msg, "duplicate entry") ||
		strings.Contains(msg, "unique violation")
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// GetUserByID 根据ID获取用户
func (s *AuthService) GetUserByID(id uint) (*models.User, error) {
	return s.userRepo.FindByID(id)
}

// ChangePassword 修改密码
func (s *AuthService) ChangePassword(userID uint, oldPassword, newPassword string) error {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return err
	}

	// 验证旧密码
	if !password.CheckPassword(oldPassword, user.PasswordHash) {
		return errors.New("Incorrect old password")
	}

	// 验证新密码策略
	policy := s.cfg.Security.PasswordPolicy
	if err := password.ValidatePasswordPolicy(newPassword, policy.MinLength, policy.RequireUppercase,
		policy.RequireLowercase, policy.RequireNumber, policy.RequireSpecial); err != nil {
		return err
	}

	// 哈希新密码
	hashedPassword, err := password.HashPassword(newPassword)
	if err != nil {
		return err
	}

	user.PasswordHash = hashedPassword
	return s.userRepo.Update(user)
}

// UpdateLoginIP 更新用户登录IP
func (s *AuthService) UpdateLoginIP(user *models.User) {
	s.userRepo.Update(user)
}

// UpdatePreferences 更新用户偏好设置（语言、国家等）
func (s *AuthService) UpdatePreferences(userID uint, locale, country string) error {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return err
	}

	if locale != "" {
		user.Locale = locale
	}
	if country != "" {
		user.Country = country
	}

	return s.userRepo.Update(user)
}

// GenerateMagicToken 生成快速登录Token
func (s *AuthService) GenerateMagicToken(userID uint, expiresIn int) (string, time.Time, error) {
	token := uuid.New().String()
	expiresAt := models.NowFunc().Add(time.Duration(expiresIn) * time.Second)

	// 这里需要保存到数据库的magic_tokens表
	// 暂时先返回token和过期时间
	return token, expiresAt, nil
}

// GenerateToken 生成JWT Token
func (s *AuthService) GenerateToken(user *models.User) (string, error) {
	// 检查用户状态
	if !user.IsActive {
		return "", errors.New("User account has been disabled")
	}

	// 生成JWT Token
	token, err := jwt.GenerateToken(user.ID, user.Email, user.Role, s.cfg.JWT.ExpireHours)
	if err != nil {
		return "", err
	}

	// 更新最后登录时间
	now := models.NowFunc()
	user.LastLoginAt = &now
	s.userRepo.Update(user)

	return token, nil
}

// SendLoginCode 生成邮箱登录验证码并存入Redis
func (s *AuthService) SendLoginCode(email string) (string, error) {
	email = normalizeEmail(email)
	user, err := s.userRepo.FindByEmail(email)
	if err != nil {
		return "", errors.New("User not found")
	}
	if !user.IsActive {
		return "", errors.New("User account has been disabled")
	}

	n, err := crand.Int(crand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", fmt.Errorf("failed to generate code: %w", err)
	}
	code := fmt.Sprintf("%06d", n.Int64())

	if err := cache.Set("email_login_code:"+email, code, 10*time.Minute); err != nil {
		return "", fmt.Errorf("failed to store login code: %w", err)
	}
	return code, nil
}

// GeneratePasswordResetToken 生成密码重置token并存入Redis
func (s *AuthService) GeneratePasswordResetToken(email string) (string, error) {
	email = normalizeEmail(email)
	user, err := s.userRepo.FindByEmail(email)
	if err != nil {
		return "", err
	}
	if !user.IsActive {
		return "", errors.New("User account has been disabled")
	}

	b := make([]byte, 32)
	if _, err := crand.Read(b); err != nil {
		return "", err
	}
	token := fmt.Sprintf("%x", b)

	if err := cache.Set("password_reset:"+token, email, 30*time.Minute); err != nil {
		return "", err
	}
	return token, nil
}

// ResetPassword 使用token重置密码
func (s *AuthService) ResetPassword(token, newPassword string) error {
	key := "password_reset:" + token
	email, err := cache.Get(key)
	if err != nil {
		return errors.New("Reset token expired or invalid")
	}

	user, err := s.userRepo.FindByEmail(email)
	if err != nil {
		return errors.New("User not found")
	}

	policy := s.cfg.Security.PasswordPolicy
	if err := password.ValidatePasswordPolicy(newPassword, policy.MinLength, policy.RequireUppercase,
		policy.RequireLowercase, policy.RequireNumber, policy.RequireSpecial); err != nil {
		return err
	}

	hashedPassword, err := password.HashPassword(newPassword)
	if err != nil {
		return err
	}

	user.PasswordHash = hashedPassword
	if err := s.userRepo.Update(user); err != nil {
		return err
	}

	_ = cache.Del(key)
	return nil
}

// LoginWithCode 使用邮箱验证码登录
func (s *AuthService) LoginWithCode(email, code string) (string, *models.User, error) {
	email = normalizeEmail(email)
	key := "email_login_code:" + email

	storedCode, err := cache.Get(key)
	if err != nil {
		return "", nil, errors.New("Verification code expired or invalid")
	}
	if storedCode != code {
		return "", nil, errors.New("Invalid verification code")
	}
	_ = cache.Del(key)

	user, err := s.userRepo.FindByEmail(email)
	if err != nil {
		return "", nil, errors.New("User not found")
	}
	if !user.IsActive {
		return "", nil, errors.New("User account has been disabled")
	}

	token, err := jwt.GenerateToken(user.ID, user.Email, user.Role, s.cfg.JWT.ExpireHours)
	if err != nil {
		return "", nil, err
	}

	now := models.NowFunc()
	user.LastLoginAt = &now
	s.userRepo.Update(user)

	return token, user, nil
}
