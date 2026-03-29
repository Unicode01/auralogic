package admin

import (
	"fmt"
	"strings"

	"auralogic/internal/pkg/utils"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
)

const pluginLocaleHeader = "X-AuraLogic-Locale"

func resolvePluginRequestLocaleMetadata(c *gin.Context) (locale string, acceptLanguage string) {
	if c == nil {
		return "", ""
	}
	acceptLanguage = strings.TrimSpace(c.GetHeader("Accept-Language"))
	locale = strings.TrimSpace(c.GetHeader(pluginLocaleHeader))
	if locale == "" {
		locale = acceptLanguage
	}
	return locale, acceptLanguage
}

func resolvePluginPublicCacheLocaleVaryKey(c *gin.Context) string {
	locale, acceptLanguage := resolvePluginRequestLocaleMetadata(c)
	if locale == "" && acceptLanguage == "" {
		return ""
	}
	return fmt.Sprintf("locale=%s|accept=%s", locale, acceptLanguage)
}

func enrichPluginExecutionContextWithRequestMetadata(
	execCtx *service.ExecutionContext,
	c *gin.Context,
) *service.ExecutionContext {
	if execCtx == nil {
		execCtx = &service.ExecutionContext{}
	}
	if c == nil {
		return execCtx
	}
	if execCtx.Metadata == nil {
		execCtx.Metadata = make(map[string]string)
	}

	locale, acceptLanguage := resolvePluginRequestLocaleMetadata(c)
	defaults := map[string]string{
		"request_path": c.Request.URL.Path,
		"route":        c.FullPath(),
		"client_ip":    utils.GetRealIP(c),
		"user_agent":   c.GetHeader("User-Agent"),
	}
	if acceptLanguage != "" {
		defaults["accept_language"] = acceptLanguage
	}
	if locale != "" {
		defaults["locale"] = locale
	}
	for key, value := range defaults {
		if strings.TrimSpace(execCtx.Metadata[key]) == "" && strings.TrimSpace(value) != "" {
			execCtx.Metadata[key] = strings.TrimSpace(value)
		}
	}
	if execCtx.RequestContext == nil && c.Request != nil {
		execCtx.RequestContext = c.Request.Context()
	}
	return execCtx
}
