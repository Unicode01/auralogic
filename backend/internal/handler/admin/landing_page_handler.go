package admin

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/middleware"
	"auralogic/internal/models"
	"auralogic/internal/pkg/response"
	"auralogic/internal/pkg/utils"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type LandingPageHandler struct {
	db            *gorm.DB
	cfg           *config.Config
	pluginManager *service.PluginManagerService
}

func NewLandingPageHandler(db *gorm.DB, cfg *config.Config, pluginManager *service.PluginManagerService) *LandingPageHandler {
	return &LandingPageHandler{db: db, cfg: cfg, pluginManager: pluginManager}
}

func matchPageRule(pagePath string, rule config.PageRule) bool {
	if rule.MatchType == "regex" {
		re, err := regexp.Compile(rule.Pattern)
		if err != nil {
			return false
		}
		return re.MatchString(pagePath)
	}
	return pagePath == rule.Pattern
}

func collectPageInjectContent(pagePath string, rules []config.PageRule) (string, string) {
	var css, js strings.Builder
	for _, rule := range rules {
		if !rule.Enabled || !matchPageRule(pagePath, rule) {
			continue
		}
		if rule.CSS != "" {
			css.WriteString(rule.CSS)
			css.WriteByte('\n')
		}
		if rule.JS != "" {
			js.WriteString(rule.JS)
			js.WriteByte('\n')
		}
	}
	return css.String(), js.String()
}

func injectPageContent(htmlContent, css, js string) string {
	if css == "" && js == "" {
		return htmlContent
	}

	result := htmlContent
	if css != "" {
		styleTag := `<style id="auralogic-page-inject-css">` + css + `</style>`
		if strings.Contains(result, "</head>") {
			result = strings.Replace(result, "</head>", styleTag+"\n</head>", 1)
		} else {
			result = styleTag + "\n" + result
		}
	}

	if js != "" {
		scriptTag := `<script id="auralogic-page-inject-js">` + js + `</script>`
		if strings.Contains(result, "</body>") {
			result = strings.Replace(result, "</body>", scriptTag+"\n</body>", 1)
		} else {
			result = result + "\n" + scriptTag
		}
	}

	return result
}

func injectLandingPageSessionScript(htmlContent string) string {
	sessionScript := `<script id="auralogic-session-bootstrap">(function(){try{var key='auralogic_session_id';var sid=localStorage.getItem(key);if(!sid){if(window.crypto&&typeof window.crypto.randomUUID==='function'){sid=window.crypto.randomUUID();}else{sid='web-'+Date.now().toString(36)+'-'+Math.random().toString(36).slice(2,12);}localStorage.setItem(key,sid);}window.__AURALOGIC_SESSION_ID__=sid;}catch(e){}})();</script>`
	if strings.Contains(htmlContent, "</head>") {
		return strings.Replace(htmlContent, "</head>", sessionScript+"\n</head>", 1)
	}
	return sessionScript + "\n" + htmlContent
}

// ServeLandingPage 公开 GET / — 渲染落地页
func (h *LandingPageHandler) ServeLandingPage(c *gin.Context) {
	var page models.LandingPage
	if err := h.db.Where("slug = ? AND is_active = ?", "home", true).First(&page).Error; err != nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	// 模板变量
	primaryColor := h.cfg.Customization.PrimaryColor
	if primaryColor == "" {
		primaryColor = "#3b82f6"
	}
	currency := h.cfg.Order.Currency
	if currency == "" {
		currency = "CNY"
	}
	appURL := h.cfg.App.URL
	if appURL == "" {
		appURL = fmt.Sprintf("http://localhost:%d", h.cfg.App.Port)
	}
	logoURL := h.cfg.Customization.LogoURL

	data := map[string]interface{}{
		"AppName":      h.cfg.App.Name,
		"AppURL":       appURL,
		"Currency":     currency,
		"LogoURL":      logoURL,
		"PrimaryColor": primaryColor,
		"Year":         time.Now().Year(),
	}

	tmpl, err := template.New("landing").Parse(page.HTMLContent)
	if err != nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	renderedHTML := buf.String()
	renderedHTML = injectLandingPageSessionScript(renderedHTML)
	pageInjectPayload, resolveErr := service.ResolvePageInjectPayload(h.db, h.cfg, "/")
	if resolveErr != nil {
		log.Printf("landing page inject resolve failed: %v", resolveErr)
	}
	renderedHTML = injectPageContent(renderedHTML, pageInjectPayload.CSS, pageInjectPayload.JS)

	// 异步记录 PageView
	ip := utils.GetRealIP(c)
	ua := c.GetHeader("User-Agent")
	referer := c.GetHeader("Referer")
	go func() {
		pv := models.PageView{
			Page:      "/",
			IP:        ip,
			UserAgent: ua,
			Referer:   referer,
		}
		if err := h.db.Create(&pv).Error; err != nil {
			log.Printf("Warning: failed to record page view: %v", err)
		}
	}()

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(renderedHTML))
}

// GetLandingPage 管理员 GET — 返回落地页 JSON
func (h *LandingPageHandler) GetLandingPage(c *gin.Context) {
	var page models.LandingPage
	if err := h.db.Where("slug = ?", "home").First(&page).Error; err != nil {
		response.Success(c, gin.H{
			"id":           0,
			"slug":         "home",
			"html_content": "",
			"is_active":    false,
		})
		return
	}
	response.Success(c, page)
}

// UpdateLandingPage 管理员 PUT — 更新落地页
func (h *LandingPageHandler) UpdateLandingPage(c *gin.Context) {
	var req struct {
		HTMLContent string `json:"html_content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "html_content is required")
		return
	}
	adminID, adminIDOK := middleware.RequireUserID(c)
	if !adminIDOK {
		return
	}
	if h.pluginManager != nil {
		hookPayload := map[string]interface{}{
			"slug":         "home",
			"html_content": req.HTMLContent,
			"admin_id":     adminID,
			"source":       "admin_api",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "landing_page.update.before",
			Payload: hookPayload,
		}, buildAdminHookExecutionContext(c, &adminID, map[string]string{
			"hook_resource": "landing_page",
			"hook_source":   "admin_api",
			"page_slug":     "home",
		}))
		if hookErr != nil {
			log.Printf("landing_page.update.before hook execution failed: admin=%d err=%v", adminID, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Landing page update rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if value, exists := hookResult.Payload["html_content"]; exists {
					req.HTMLContent = parseStringFromAny(value)
				}
			}
		}
	}

	// 验证 HTML 可被 Go template 解析
	if _, err := template.New("validate").Parse(req.HTMLContent); err != nil {
		response.BadRequest(c, fmt.Sprintf("Invalid template syntax: %v", err))
		return
	}

	uid := adminID

	var page models.LandingPage
	created := false
	err := h.db.Where("slug = ?", "home").First(&page).Error
	if err != nil {
		// 不存在则创建
		created = true
		page = models.LandingPage{
			Slug:        "home",
			HTMLContent: req.HTMLContent,
			IsActive:    true,
			UpdatedBy:   uid,
		}
		if err := h.db.Create(&page).Error; err != nil {
			response.InternalError(c, "Failed to create landing page")
			return
		}
	} else {
		page.HTMLContent = req.HTMLContent
		page.UpdatedBy = uid
		if err := h.db.Save(&page).Error; err != nil {
			response.InternalError(c, "Failed to update landing page")
			return
		}
	}

	// 记录操作日志
	go func() {
		pageID := page.ID
		opLog := models.OperationLog{
			UserID:       &uid,
			Action:       "update",
			ResourceType: "landing_page",
			ResourceID:   &pageID,
			IPAddress:    utils.GetRealIP(c),
			UserAgent:    c.GetHeader("User-Agent"),
		}
		h.db.Create(&opLog)
	}()
	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"page_id":        page.ID,
			"slug":           page.Slug,
			"html_content":   page.HTMLContent,
			"content_length": len(page.HTMLContent),
			"is_active":      page.IsActive,
			"updated_by":     uid,
			"created":        created,
			"admin_id":       adminID,
			"source":         "admin_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, pageID uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "landing_page.update.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("landing_page.update.after hook execution failed: admin=%d page=%d err=%v", adminID, pageID, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, &adminID, map[string]string{
			"hook_resource": "landing_page",
			"hook_source":   "admin_api",
			"page_slug":     "home",
		})), afterPayload, page.ID)
	}

	response.Success(c, page)
}

// ResetLandingPage 管理员 POST — 重置落地页为默认内容
func (h *LandingPageHandler) ResetLandingPage(c *gin.Context) {
	defaultHTML := DefaultLandingPageHTML

	adminID, adminIDOK := middleware.RequireUserID(c)
	if !adminIDOK {
		return
	}
	if h.pluginManager != nil {
		hookPayload := map[string]interface{}{
			"slug":         "home",
			"html_content": defaultHTML,
			"admin_id":     adminID,
			"source":       "admin_api",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "landing_page.reset.before",
			Payload: hookPayload,
		}, buildAdminHookExecutionContext(c, &adminID, map[string]string{
			"hook_resource": "landing_page",
			"hook_source":   "admin_api",
			"page_slug":     "home",
		}))
		if hookErr != nil {
			log.Printf("landing_page.reset.before hook execution failed: admin=%d err=%v", adminID, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Landing page reset rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if value, exists := hookResult.Payload["html_content"]; exists {
					defaultHTML = parseStringFromAny(value)
				}
			}
		}
	}

	uid := adminID

	var page models.LandingPage
	created := false
	err := h.db.Where("slug = ?", "home").First(&page).Error
	if err != nil {
		created = true
		page = models.LandingPage{
			Slug:        "home",
			HTMLContent: defaultHTML,
			IsActive:    true,
			UpdatedBy:   uid,
		}
		if err := h.db.Create(&page).Error; err != nil {
			response.InternalError(c, "Failed to reset landing page")
			return
		}
	} else {
		page.HTMLContent = defaultHTML
		page.UpdatedBy = uid
		if err := h.db.Save(&page).Error; err != nil {
			response.InternalError(c, "Failed to reset landing page")
			return
		}
	}

	go func() {
		pageID := page.ID
		opLog := models.OperationLog{
			UserID:       &uid,
			Action:       "reset",
			ResourceType: "landing_page",
			ResourceID:   &pageID,
			IPAddress:    utils.GetRealIP(c),
			UserAgent:    c.GetHeader("User-Agent"),
		}
		h.db.Create(&opLog)
	}()
	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"page_id":        page.ID,
			"slug":           page.Slug,
			"html_content":   page.HTMLContent,
			"content_length": len(page.HTMLContent),
			"is_active":      page.IsActive,
			"updated_by":     uid,
			"created":        created,
			"admin_id":       adminID,
			"source":         "admin_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, pageID uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "landing_page.reset.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("landing_page.reset.after hook execution failed: admin=%d page=%d err=%v", adminID, pageID, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, &adminID, map[string]string{
			"hook_resource": "landing_page",
			"hook_source":   "admin_api",
			"page_slug":     "home",
		})), afterPayload, page.ID)
	}

	response.Success(c, page)
}
