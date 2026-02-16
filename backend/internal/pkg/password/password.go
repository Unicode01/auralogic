package password

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"regexp"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword 哈希Password
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// CheckPassword 验证Password
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateRandomPassword generate随机Password
func GenerateRandomPassword(length int) (string, error) {
	if length < 8 {
		length = 12
	}

	const (
		lowercase = "abcdefghijklmnopqrstuvwxyz"
		uppercase = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		digits    = "0123456789"
		special   = "!@#$%^&*"
	)

	allChars := lowercase + uppercase + digits + special

	// 确保至少包含每种类型的字符
	password := make([]byte, length)

	// 至少一个小写字母
	password[0] = lowercase[mustRandomInt(len(lowercase))]
	// 至少一个大写字母
	password[1] = uppercase[mustRandomInt(len(uppercase))]
	// 至少一个数字
	password[2] = digits[mustRandomInt(len(digits))]
	// 至少一个特殊字符
	password[3] = special[mustRandomInt(len(special))]

	// 填充剩余字符
	for i := 4; i < length; i++ {
		password[i] = allChars[mustRandomInt(len(allChars))]
	}

	// 打乱顺序
	for i := range password {
		j := mustRandomInt(len(password))
		password[i], password[j] = password[j], password[i]
	}

	return string(password), nil
}

// mustRandomInt generate随机整数
func mustRandomInt(max int) int {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		panic(err)
	}
	return int(n.Int64())
}

// ValidatePasswordPolicy 验证Password策略
func ValidatePasswordPolicy(password string, minLength int, requireUpper, requireLower, requireNumber, requireSpecial bool) error {
	if len(password) < minLength {
		return fmt.Errorf("Password must be at least %d characters", minLength)
	}

	if requireUpper && !regexp.MustCompile(`[A-Z]`).MatchString(password) {
		return fmt.Errorf("Password must contain at least one uppercase letter")
	}

	if requireLower && !regexp.MustCompile(`[a-z]`).MatchString(password) {
		return fmt.Errorf("Password must contain at least one lowercase letter")
	}

	if requireNumber && !regexp.MustCompile(`[0-9]`).MatchString(password) {
		return fmt.Errorf("Password must contain at least one digit")
	}

	if requireSpecial && !regexp.MustCompile(`[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>\/?]`).MatchString(password) {
		return fmt.Errorf("Password must contain at least one special character")
	}

	return nil
}

