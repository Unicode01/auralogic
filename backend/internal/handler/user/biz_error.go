package user

import (
	"errors"

	"auralogic/internal/pkg/bizerr"
	"auralogic/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

func respondUserBizError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}

	var bizErr *bizerr.Error
	if !errors.As(err, &bizErr) {
		return false
	}

	response.BizError(c, bizErr.Message, bizErr.Key, bizErr.Params)
	return true
}
