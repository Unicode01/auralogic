package bizerr

import "fmt"

// Error 业务错误，携带 i18n key 和动态参数，可安全返回给前端
type Error struct {
	Key     string                 `json:"key"`               // 前端 i18n key，如 "order.purchaseLimitReached"
	Message string                 `json:"message"`           // 英文 fallback
	Params  map[string]interface{} `json:"params,omitempty"`  // 动态参数，如 {product: "xxx", limit: 5}
}

func (e *Error) Error() string {
	return e.Message
}

// New 创建业务错误
func New(key, message string) *Error {
	return &Error{Key: key, Message: message}
}

// Newf 创建带格式化消息的业务错误
func Newf(key, format string, args ...interface{}) *Error {
	return &Error{Key: key, Message: fmt.Sprintf(format, args...)}
}

// WithParams 附加动态参数
func (e *Error) WithParams(params map[string]interface{}) *Error {
	e.Params = params
	return e
}
