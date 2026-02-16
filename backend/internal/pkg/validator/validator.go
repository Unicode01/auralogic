package validator

import (
	"regexp"
)

var (
	emailRegex    = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	phoneRegex    = regexp.MustCompile(`^1[3-9]\d{9}$`)
	phoneIntlRegex = regexp.MustCompile(`^\+?[1-9]\d{1,14}$`)
)

// IsValidEmail 验证Email格式
func IsValidEmail(email string) bool {
	return emailRegex.MatchString(email)
}

// IsValidPhone 验证Phone格式（中国Phone）
func IsValidPhone(phone string) bool {
	return phoneRegex.MatchString(phone)
}

// IsValidPhoneInternational 验证国际Phone
func IsValidPhoneInternational(phone string) bool {
	return phoneIntlRegex.MatchString(phone)
}

