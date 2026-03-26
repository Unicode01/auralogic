package admin

import (
	"auralogic/internal/pkg/password"
	"github.com/gin-gonic/gin"
)

func respondAdminPasswordPolicyBizError(c *gin.Context, err error) bool {
	return respondAdminBizError(c, password.ToBizError(err))
}
