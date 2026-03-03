package utils

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync/atomic"
	"time"
)

var orderNoCounter uint32

// GenerateToken generate随机Token
func GenerateToken(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b)[:length], nil
}

// GenerateOrderNo generateOrder号
func GenerateOrderNo(prefix string) string {
	now := time.Now()
	seq := atomic.AddUint32(&orderNoCounter, 1) % 10000
	return fmt.Sprintf("%s%s%06d%04d",
		prefix,
		now.Format("20060102150405"),
		now.Nanosecond()/1000,
		seq)
}

// GenerateAPIKey generateAPI密钥
func GenerateAPIKey(prefix string) (string, error) {
	token, err := GenerateToken(32)
	if err != nil {
		return "", err
	}
	return prefix + "_" + token, nil
}
