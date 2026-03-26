package user

import (
	"errors"
	"net/http"

	"auralogic/internal/pkg/bizerr"
	"auralogic/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

func respondAuthBizError(c *gin.Context, err error, extraData gin.H) bool {
	if err == nil {
		return false
	}

	var bizErr *bizerr.Error
	if !errors.As(err, &bizErr) {
		return false
	}

	data := gin.H{
		"error_key": bizErr.Key,
		"params":    bizErr.Params,
	}
	for key, value := range extraData {
		data[key] = value
	}

	switch bizErr.Key {
	case "auth.invalidEmailOrPassword", "auth.accountDisabled":
		response.ErrorWithData(c, http.StatusUnauthorized, response.CodeUnauthorized, bizErr.Message, data)
	case "auth.userNotFound":
		response.ErrorWithData(c, http.StatusNotFound, response.CodeUserNotFound, bizErr.Message, data)
	case "auth.passwordLoginDisabled":
		response.ErrorWithData(c, http.StatusForbidden, response.CodePasswordDisabled, bizErr.Message, data)
	case "auth.registrationDisabled",
		"auth.emailLoginDisabled",
		"auth.passwordResetDisabled",
		"auth.phoneLoginDisabled",
		"auth.phoneRegistrationDisabled",
		"auth.phonePasswordResetDisabled":
		response.ErrorWithData(c, http.StatusForbidden, response.CodeForbidden, bizErr.Message, data)
	case "auth.emailNotVerified":
		response.ErrorWithData(c, http.StatusForbidden, response.CodeEmailNotVerified, bizErr.Message, data)
	case "auth.emailAlreadyInUse", "auth.phoneAlreadyInUse":
		response.ErrorWithData(c, http.StatusConflict, response.CodeConflict, bizErr.Message, data)
	case "auth.emailLoginUnavailable", "auth.smsServiceUnavailable":
		response.ErrorWithData(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, bizErr.Message, data)
	case "auth.captchaRequired":
		response.ErrorWithData(c, http.StatusBadRequest, response.CodeParamMissing, bizErr.Message, data)
	case "auth.captchaFailed", "auth.invalidPhoneFormat":
		response.ErrorWithData(c, http.StatusBadRequest, response.CodeParamError, bizErr.Message, data)
	default:
		response.ErrorWithData(c, http.StatusBadRequest, response.CodeBusinessError, bizErr.Message, data)
	}

	return true
}
