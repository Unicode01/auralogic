package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	Username string `json:"username"`
	Role     string `json:"role"`
}

type Config struct {
	AdminUsername     string
	AdminPassword     string
	AdminPasswordHash string
	TokenTTL          time.Duration
	Now               func() time.Time
}

type tokenEntry struct {
	User      *User
	ExpiresAt time.Time
}

type Service struct {
	mu     sync.RWMutex
	users  map[string]string
	tokens map[string]tokenEntry
	now    func() time.Time
	ttl    time.Duration
}

func NewService() *Service {
	return NewServiceWithConfig(Config{})
}

func NewServiceWithConfig(cfg Config) *Service {
	adminUsername := cfg.AdminUsername
	if adminUsername == "" {
		adminUsername = "admin"
	}

	passwordHash := cfg.AdminPasswordHash
	if passwordHash == "" {
		password := cfg.AdminPassword
		if password == "" {
			password = "admin"
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		passwordHash = string(hash)
	}

	nowFn := cfg.Now
	if nowFn == nil {
		nowFn = time.Now
	}
	tokenTTL := cfg.TokenTTL
	if tokenTTL <= 0 {
		tokenTTL = 12 * time.Hour
	}

	return &Service{
		users: map[string]string{
			adminUsername: passwordHash,
		},
		tokens: make(map[string]tokenEntry),
		now:    nowFn,
		ttl:    tokenTTL,
	}
}

func (s *Service) Login(username, password string) (string, error) {
	s.mu.RLock()
	hash, ok := s.users[username]
	s.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return "", fmt.Errorf("invalid credentials")
	}

	token := generateToken()
	entry := tokenEntry{
		User: &User{
			Username: username,
			Role:     "admin",
		},
		ExpiresAt: s.now().Add(s.ttl),
	}
	s.mu.Lock()
	s.tokens[token] = entry
	s.mu.Unlock()

	return token, nil
}

func (s *Service) ValidateToken(token string) (*User, error) {
	s.mu.RLock()
	entry, ok := s.tokens[token]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("invalid token")
	}
	if !entry.ExpiresAt.IsZero() && s.now().After(entry.ExpiresAt) {
		s.mu.Lock()
		delete(s.tokens, token)
		s.mu.Unlock()
		return nil, fmt.Errorf("token expired")
	}
	return entry.User, nil
}

func generateToken() string {
	bytes := make([]byte, 32)
	_, _ = rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
