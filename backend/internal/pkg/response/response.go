package response

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response 统一响应格式
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Errors  interface{} `json:"errors,omitempty"`
}

// Pagination 分页Info
type Pagination struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
	HasNext    bool  `json:"has_next"`
	HasPrev    bool  `json:"has_prev"`
}

// PaginatedResponse 分页响应
type PaginatedResponse struct {
	Items      interface{} `json:"items"`
	Pagination Pagination  `json:"pagination"`
}

// Error码定义
const (
	CodeSuccess            = 0
	CodeParamError         = 10001
	CodeParamMissing       = 10002
	CodeUnauthorized       = 20001
	CodeTokenExpired       = 20002
	CodeTokenInvalid       = 20003
	CodeAPIKeyInvalid      = 20004
	CodeForbidden          = 30001
	CodePasswordDisabled   = 30002
	CodeEmailNotVerified   = 30003
	CodeNotFound           = 40001
	CodeOrderNotFound      = 40002
	CodeUserNotFound       = 40003
	CodeConflict           = 40901
	CodeOrderDuplicate     = 40902
	CodeTooManyRequests    = 42901
	CodeCooldown          = 42902
	CodeInternalError      = 50001
	CodeDatabaseError      = 50002
	CodeCacheError         = 50003
)

// Success Success响应
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    CodeSuccess,
		Message: "success",
		Data:    data,
	})
}

// Error Error响应
func Error(c *gin.Context, httpCode, code int, message string) {
	c.JSON(httpCode, Response{
		Code:    code,
		Message: message,
	})
}

// ErrorWithData 带数据的Error响应
func ErrorWithData(c *gin.Context, httpCode, code int, message string, data interface{}) {
	c.JSON(httpCode, Response{
		Code:    code,
		Message: message,
		Data:    data,
	})
}

// ValidationError 验证Error响应
func ValidationError(c *gin.Context, errors interface{}) {
	c.JSON(http.StatusBadRequest, Response{
		Code:    CodeParamError,
		Message: "Validation failed",
		Errors:  errors,
	})
}

// Paginated 分页响应
func Paginated(c *gin.Context, items interface{}, page, limit int, total int64) {
	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}

	Success(c, PaginatedResponse{
		Items: items,
		Pagination: Pagination{
			Page:       page,
			Limit:      limit,
			Total:      total,
			TotalPages: totalPages,
			HasNext:    page < totalPages,
			HasPrev:    page > 1,
		},
	})
}

// Unauthorized 未授权
func Unauthorized(c *gin.Context, message string) {
	if message == "" {
		message = "Unauthorized"
	}
	Error(c, http.StatusUnauthorized, CodeUnauthorized, message)
}

// Forbidden 禁止访问
func Forbidden(c *gin.Context, message string) {
	if message == "" {
		message = "No permission to access"
	}
	Error(c, http.StatusForbidden, CodeForbidden, message)
}

// NotFound 资源does not exist
func NotFound(c *gin.Context, message string) {
	if message == "" {
		message = "Resource not found"
	}
	Error(c, http.StatusNotFound, CodeNotFound, message)
}

// InternalError 服务器内部Error
func InternalError(c *gin.Context, message string) {
	if message == "" {
		message = "Internal server error"
	}
	Error(c, http.StatusInternalServerError, CodeInternalError, message)
}

// InternalServerError 服务器内部Error（记录详细错误日志，仅返回安全消息给客户端）
func InternalServerError(c *gin.Context, userMessage string, err error) {
	if err != nil {
		log.Printf("[ERROR] %s %s: %s - %v", c.Request.Method, c.Request.URL.Path, userMessage, err)
	}
	if userMessage == "" {
		userMessage = "Internal server error"
	}
	Error(c, http.StatusInternalServerError, CodeInternalError, userMessage)
}

// BadRequest Error请求
func BadRequest(c *gin.Context, message string) {
	if message == "" {
		message = "Invalid request parameters"
	}
	Error(c, http.StatusBadRequest, CodeParamError, message)
}

// Conflict 资源冲突
func Conflict(c *gin.Context, message string) {
	if message == "" {
		message = "Resource already exists"
	}
	Error(c, http.StatusConflict, CodeConflict, message)
}

