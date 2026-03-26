package user

import (
	"auralogic/internal/pkg/password"
	"auralogic/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

func respondAuthValidationOrInternalError(c *gin.Context, err error, fallback string) {
	if err == nil {
		return
	}

	if bizErr := password.ToBizError(err); bizErr != nil {
		response.BizError(c, bizErr.Message, bizErr.Key, bizErr.Params)
		return
	}

	response.InternalServerError(c, fallback, err)
}
