package response

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"auralogic/internal/pkg/bizerr"

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
	CodeBusinessError      = 40010 // 业务逻辑错误（限购、库存不足等）
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

// GetPagination 从请求中解析并校验分页参数
func GetPagination(c *gin.Context) (page, limit int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ = strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	} else if limit > 100 {
		limit = 100
	}
	return
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

// BizError 业务逻辑错误响应（限购、库存不足等），携带 i18n key 和参数
func BizError(c *gin.Context, message string, key string, params map[string]interface{}) {
	c.JSON(http.StatusBadRequest, Response{
		Code:    CodeBusinessError,
		Message: message,
		Data: gin.H{
			"error_key": key,
			"params":    params,
		},
	})
}

// HandleError 统一错误处理：业务错误返回详细信息，其他错误仅记录日志并返回通用提示
func HandleError(c *gin.Context, fallbackMsg string, err error) {
	var bizErr *bizerr.Error
	if errors.As(err, &bizErr) {
		BizError(c, bizErr.Message, bizErr.Key, bizErr.Params)
		return
	}
	InternalServerError(c, fallbackMsg, err)
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

