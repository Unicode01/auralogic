package user

import (
	"strings"

	"auralogic/internal/pkg/response"
	"auralogic/internal/pkg/utils"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
)

type SerialHandler struct {
	serialService  *service.SerialService
	captchaService *service.CaptchaService
}

func NewSerialHandler(serialService *service.SerialService) *SerialHandler {
	return &SerialHandler{
		serialService:  serialService,
		captchaService: service.NewCaptchaService(),
	}
}

func buildPublicSerialHookExecutionContext(c *gin.Context, source string) *service.ExecutionContext {
	if c == nil {
		return nil
	}

	normalizedSource := strings.TrimSpace(source)
	if normalizedSource == "" {
		normalizedSource = "public_api"
	}
	return &service.ExecutionContext{
		SessionID: strings.TrimSpace(c.GetHeader("X-Session-ID")),
		Metadata: map[string]string{
			"request_path":    c.Request.URL.Path,
			"route":           c.FullPath(),
			"method":          c.Request.Method,
			"client_ip":       c.ClientIP(),
			"user_agent":      c.GetHeader("User-Agent"),
			"accept_language": c.GetHeader("Accept-Language"),
			"operator_type":   "public",
			"hook_source":     normalizedSource,
		},
		RequestContext: c.Request.Context(),
	}
}

// VerifySerial 验证序列号（用户端）
func (h *SerialHandler) VerifySerial(c *gin.Context) {
	var req struct {
		SerialNumber string `json:"serial_number" binding:"required"`
		CaptchaToken string `json:"captcha_token"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Please enter a serial number")
		return
	}

	if h.captchaService.NeedCaptcha("serial_verify") {
		if req.CaptchaToken == "" {
			response.Error(c, 400, response.CodeParamMissing, "Captcha is required")
			return
		}
		if err := h.captchaService.VerifyCaptcha(req.CaptchaToken, utils.GetRealIP(c)); err != nil {
			response.Error(c, 400, response.CodeParamError, "Captcha verification failed")
			return
		}
	}

	serial, err := h.serialService.VerifySerialWithContext(req.SerialNumber, buildPublicSerialHookExecutionContext(c, "public_api"), "public_api")
	if err != nil {
		if service.IsHookBlockedError(err) {
			response.BadRequest(c, err.Error())
			return
		}
		response.NotFound(c, "Serial number not found or invalid")
		return
	}

	// 隐藏敏感信息（用户端不显示订单详细信息）
	serial.Order = nil

	response.Success(c, serial)
}

// GetSerialByNumber 根据序列号查询（GET方式，用于扫码）
func (h *SerialHandler) GetSerialByNumber(c *gin.Context) {
	serialNumber := c.Param("serial_number")
	captchaToken := c.Query("captcha_token")

	if serialNumber == "" {
		response.BadRequest(c, "Please enter a serial number")
		return
	}

	if h.captchaService.NeedCaptcha("serial_verify") {
		if captchaToken == "" {
			response.Error(c, 400, response.CodeParamMissing, "Captcha is required")
			return
		}
		if err := h.captchaService.VerifyCaptcha(captchaToken, utils.GetRealIP(c)); err != nil {
			response.Error(c, 400, response.CodeParamError, "Captcha verification failed")
			return
		}
	}

	serial, err := h.serialService.VerifySerialWithContext(serialNumber, buildPublicSerialHookExecutionContext(c, "public_api"), "public_api")
	if err != nil {
		if service.IsHookBlockedError(err) {
			response.BadRequest(c, err.Error())
			return
		}
		response.NotFound(c, "Serial number not found or invalid")
		return
	}

	// 隐藏敏感信息（用户端不显示订单详细信息）
	serial.Order = nil

	response.Success(c, serial)
}
