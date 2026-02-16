package user

import (
	"github.com/gin-gonic/gin"
	"auralogic/internal/pkg/response"
	"auralogic/internal/pkg/utils"
	"auralogic/internal/service"
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

// VerifySerial 验证序列号（用户端）
func (h *SerialHandler) VerifySerial(c *gin.Context) {
	var req struct {
		SerialNumber string `json:"serial_number" binding:"required"`
		CaptchaToken string `json:"captcha_token"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请输入序列号")
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

	serial, err := h.serialService.VerifySerial(req.SerialNumber)
	if err != nil {
		response.NotFound(c, "序列号不存在或无效")
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
		response.BadRequest(c, "请输入序列号")
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

	serial, err := h.serialService.VerifySerial(serialNumber)
	if err != nil {
		response.NotFound(c, "序列号不存在或无效")
		return
	}

	// 隐藏敏感信息（用户端不显示订单详细信息）
	serial.Order = nil

	response.Success(c, serial)
}
