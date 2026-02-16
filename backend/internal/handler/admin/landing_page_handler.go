package admin

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"auralogic/internal/config"
	"auralogic/internal/models"
	"auralogic/internal/pkg/response"
	"auralogic/internal/pkg/utils"
	"gorm.io/gorm"
)

type LandingPageHandler struct {
	db  *gorm.DB
	cfg *config.Config
}

func NewLandingPageHandler(db *gorm.DB, cfg *config.Config) *LandingPageHandler {
	return &LandingPageHandler{db: db, cfg: cfg}
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

	c.Data(http.StatusOK, "text/html; charset=utf-8", buf.Bytes())
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

	// 验证 HTML 可被 Go template 解析
	if _, err := template.New("validate").Parse(req.HTMLContent); err != nil {
		response.BadRequest(c, fmt.Sprintf("Invalid template syntax: %v", err))
		return
	}

	// 获取当前用户 ID
	userID, _ := c.Get("userID")
	uid, _ := userID.(uint)

	var page models.LandingPage
	err := h.db.Where("slug = ?", "home").First(&page).Error
	if err != nil {
		// 不存在则创建
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

	response.Success(c, page)
}

// ResetLandingPage 管理员 POST — 重置落地页为默认内容
func (h *LandingPageHandler) ResetLandingPage(c *gin.Context) {
	defaultHTML := DefaultLandingPageHTML

	userID, _ := c.Get("userID")
	uid, _ := userID.(uint)

	var page models.LandingPage
	err := h.db.Where("slug = ?", "home").First(&page).Error
	if err != nil {
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

	response.Success(c, page)
}
