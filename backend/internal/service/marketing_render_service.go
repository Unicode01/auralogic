package service

import (
	"bytes"
	"fmt"
	"html"
	htmltemplate "html/template"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/models"
	"auralogic/internal/pkg/validator"
)

var (
	marketingOrderedListRe = regexp.MustCompile(`^\d+\.\s+(.+)$`)
	marketingLinkRe        = regexp.MustCompile(`\[(.*?)\]\((.*?)\)`)
	marketingBoldRe        = regexp.MustCompile(`\*\*(.+?)\*\*`)
	marketingItalicRe      = regexp.MustCompile(`\*(.+?)\*`)
	marketingInlineCodeRe  = regexp.MustCompile("`([^`]+)`")

	marketingTemplateDirOnce sync.Once
	marketingTemplateDir     string
	marketingTemplateDirErr  error
)

type MarketingRenderResult struct {
	Title          string            `json:"title"`
	EmailSubject   string            `json:"email_subject"`
	ContentHTML    string            `json:"content_html"`
	EmailHTML      string            `json:"email_html"`
	SMSText        string            `json:"sms_text"`
	Variables      map[string]string `json:"resolved_variables"`
	Placeholders   []string          `json:"supported_placeholders"`
	TemplateVars   []string          `json:"supported_template_variables"`
	ContentRawText string            `json:"-"`
}

// SupportedMarketingPlaceholders returns all placeholders supported in marketing title/content.
func SupportedMarketingPlaceholders() []string {
	return []string{
		"{{app_name}}",
		"{{app_url}}",
		"{{user_name}}",
		"{{user_email}}",
		"{{user_phone}}",
		"{{user_locale}}",
		"{{today}}",
	}
}

// SupportedMarketingTemplateVariables returns supported variables for marketing_*.html templates.
func SupportedMarketingTemplateVariables() []string {
	return []string{
		"{{.AppName}}",
		"{{.AppURL}}",
		"{{.Subject}}",
		"{{.Title}}",
		"{{.ContentHTML}}",
		"{{.ContentText}}",
		"{{.UserName}}",
		"{{.UserEmail}}",
		"{{.UserPhone}}",
		"{{.UserLocale}}",
		"{{.Today}}",
	}
}

// RenderMarketingContent renders one marketing message for preview/sending.
// This function is the single rendering source for both preview and real delivery.
func RenderMarketingContent(title, markdown string, user *models.User) MarketingRenderResult {
	vars := buildMarketingVariables(user)

	resolvedTitle := strings.TrimSpace(applyMarketingPlaceholders(title, vars))
	resolvedMarkdown := strings.TrimSpace(applyMarketingPlaceholders(markdown, vars))
	sanitizedMarkdown := validator.SanitizeMarkdown(resolvedMarkdown)

	contentHTML := renderMarketingMarkdownToHTML(sanitizedMarkdown)
	smsText := renderMarketingMarkdownToText(sanitizedMarkdown)

	subject := resolvedTitle
	if subject == "" {
		appName := vars["app_name"]
		if appName == "" {
			appName = "AuraLogic"
		}
		if strings.TrimSpace(vars["user_locale"]) == "zh" {
			subject = fmt.Sprintf("营销通知 - %s", appName)
		} else {
			subject = fmt.Sprintf("Marketing Message - %s", appName)
		}
	}

	emailHTML, err := renderMarketingEmailTemplate(subject, contentHTML, smsText, vars)
	if err != nil {
		emailHTML = wrapMarketingEmailHTML(subject, contentHTML, vars)
	}

	return MarketingRenderResult{
		Title:          resolvedTitle,
		EmailSubject:   subject,
		ContentHTML:    contentHTML,
		EmailHTML:      emailHTML,
		SMSText:        smsText,
		Variables:      vars,
		Placeholders:   SupportedMarketingPlaceholders(),
		TemplateVars:   SupportedMarketingTemplateVariables(),
		ContentRawText: sanitizedMarkdown,
	}
}

func buildMarketingVariables(user *models.User) map[string]string {
	cfg := config.GetConfig()

	appName := "AuraLogic"
	appURL := ""
	if cfg != nil {
		if strings.TrimSpace(cfg.App.Name) != "" {
			appName = strings.TrimSpace(cfg.App.Name)
		}
		appURL = strings.TrimSpace(cfg.App.URL)
	}

	var userName, userEmail, userPhone, userLocale string
	if user != nil {
		userName = strings.TrimSpace(user.Name)
		userEmail = strings.TrimSpace(user.Email)
		if user.Phone != nil {
			userPhone = strings.TrimSpace(*user.Phone)
		}
		userLocale = strings.TrimSpace(user.Locale)
	}

	return map[string]string{
		"app_name":   appName,
		"app_url":    appURL,
		"user_name":  userName,
		"user_email": userEmail,
		"user_phone": userPhone,
		"user_locale": func() string {
			if userLocale == "" {
				return "en"
			}
			return userLocale
		}(),
		"today": time.Now().Format("2006-01-02"),
	}
}

func applyMarketingPlaceholders(raw string, vars map[string]string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(vars) == 0 {
		return raw
	}

	parts := make([]string, 0, len(vars)*4)
	for key, val := range vars {
		token := "{{" + key + "}}"
		parts = append(parts, token, val)
		parts = append(parts, strings.ToUpper(token), val)
	}
	return strings.NewReplacer(parts...).Replace(raw)
}

func resolveMarketingTemplateDir() (string, error) {
	marketingTemplateDirOnce.Do(func() {
		candidates := []string{
			"templates/email",
		}

		if execPath, err := os.Executable(); err == nil {
			execDir := filepath.Dir(execPath)
			candidates = append(candidates,
				filepath.Join(execDir, "templates", "email"),
				filepath.Join(execDir, "..", "templates", "email"),
			)
		}

		for _, dir := range candidates {
			p, err := filepath.Abs(dir)
			if err != nil {
				continue
			}
			if info, err := os.Stat(p); err == nil && info.IsDir() {
				marketingTemplateDir = p
				return
			}
		}

		marketingTemplateDirErr = fmt.Errorf("template directory not found")
	})

	if marketingTemplateDir == "" {
		if marketingTemplateDirErr != nil {
			return "", marketingTemplateDirErr
		}
		return "", fmt.Errorf("template directory not found")
	}
	return marketingTemplateDir, nil
}

func pickMarketingTemplateFile(locale string) (string, error) {
	dir, err := resolveMarketingTemplateDir()
	if err != nil {
		return "", err
	}

	normalizedLocale := "en"
	if strings.TrimSpace(strings.ToLower(locale)) == "zh" {
		normalizedLocale = "zh"
	}

	candidates := []string{
		filepath.Join(dir, "marketing_"+normalizedLocale+".html"),
		filepath.Join(dir, "marketing_en.html"),
		filepath.Join(dir, "marketing.html"),
	}

	for _, file := range candidates {
		if info, err := os.Stat(file); err == nil && !info.IsDir() {
			return file, nil
		}
	}

	return "", fmt.Errorf("marketing template file not found")
}

func renderMarketingEmailTemplate(subject, contentHTML, contentText string, vars map[string]string) (string, error) {
	templateFile, err := pickMarketingTemplateFile(vars["user_locale"])
	if err != nil {
		return "", err
	}

	tmpl, err := htmltemplate.ParseFiles(templateFile)
	if err != nil {
		return "", err
	}

	data := map[string]interface{}{
		"AppName":     strings.TrimSpace(vars["app_name"]),
		"AppURL":      strings.TrimSpace(vars["app_url"]),
		"Subject":     strings.TrimSpace(subject),
		"Title":       strings.TrimSpace(subject),
		"ContentHTML": htmltemplate.HTML(contentHTML),
		"ContentText": contentText,
		"UserName":    strings.TrimSpace(vars["user_name"]),
		"UserEmail":   strings.TrimSpace(vars["user_email"]),
		"UserPhone":   strings.TrimSpace(vars["user_phone"]),
		"UserLocale":  strings.TrimSpace(vars["user_locale"]),
		"Today":       strings.TrimSpace(vars["today"]),
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func wrapMarketingEmailHTML(subject, contentHTML string, vars map[string]string) string {
	appName := html.EscapeString(strings.TrimSpace(vars["app_name"]))
	if appName == "" {
		appName = "AuraLogic"
	}
	safeSubject := html.EscapeString(strings.TrimSpace(subject))
	appURL := html.EscapeString(strings.TrimSpace(vars["app_url"]))
	sentAt := html.EscapeString(strings.TrimSpace(vars["today"]))
	if sentAt == "" {
		sentAt = time.Now().Format("2006-01-02")
	}
	ctaHTML := ""
	if appURL != "" {
		ctaHTML = fmt.Sprintf(`<p style="text-align: center; margin: 24px 0 6px;"><a href="%s" class="button" style="color: white;">Explore %s</a></p>`, appURL, appName)
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>%s</title>
  <style>
    * { box-sizing: border-box; }
    body {
      margin: 0;
      padding: 30px 10px;
      background:
        radial-gradient(circle at 8%% 0%%, #dbe8ff 0%%, transparent 40%%),
        radial-gradient(circle at 92%% 14%%, #e3f1ff 0%%, transparent 36%%),
        #eef3ff;
      color: #0f172a;
      line-height: 1.66;
      font-family: 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', 'Helvetica Neue', Arial, sans-serif;
    }
    .container {
      max-width: 700px;
      margin: 0 auto;
      background: #ffffff;
      border: 1px solid #dbe4f0;
      border-radius: 16px;
      overflow: hidden;
      box-shadow: 0 12px 30px rgba(15, 23, 42, 0.08);
    }
    .hero {
      padding: 24px 26px 20px;
      color: #ffffff;
      background: linear-gradient(130deg, #0b1220 0%%, #102040 56%%, #1e40af 100%%);
    }
    .hero h1 {
      margin: 0;
      font-size: 24px;
      line-height: 1.3;
      font-weight: 750;
      letter-spacing: 0.2px;
    }
    .hero p {
      margin: 8px 0 0;
      font-size: 14px;
      color: rgba(255, 255, 255, 0.88);
    }
    .content {
      padding: 24px 26px;
      color: #42526a;
      font-size: 15px;
    }
    .rich-content {
      margin: 14px 0;
      padding: 13px 14px;
      border-radius: 10px;
      border: 1px solid #dbe4f0;
      border-left: 3px solid #94a3b8;
      background: #f8fafc;
      color: #42526a;
    }
    .rich-content a {
      color: #1e40af;
      text-decoration: underline;
      font-weight: 600;
    }
    .button {
      display: inline-block;
      padding: 11px 20px;
      border-radius: 999px;
      border: 1px solid #3b82f6;
      background: linear-gradient(135deg, #3b82f6 0%%, #1e40af 100%%);
      color: #ffffff !important;
      text-decoration: none;
      font-weight: 700;
      letter-spacing: 0.2px;
      box-shadow: 0 8px 20px rgba(59, 130, 246, 0.25);
    }
    .footer {
      border-top: 1px solid #dbe4f0;
      background: #f8fafc;
      color: #64748b;
      font-size: 12px;
      line-height: 1.6;
      text-align: center;
      padding: 14px 26px 18px;
    }
    .footer p { margin: 0; }
    @media (max-width: 640px) {
      body { padding: 12px 6px; }
      .hero, .content, .footer {
        padding-left: 14px;
        padding-right: 14px;
      }
      .hero h1 { font-size: 20px; }
    }
  </style>
</head>
<body>
  <div class="container">
    <div class="hero">
      <h1>%s</h1>
      <p>%s curated this message for you.</p>
    </div>
    <div class="content">
      <div class="rich-content">%s</div>
      %s
    </div>
    <div class="footer">
      <p>Sent by %s on %s</p>
    </div>
  </div>
</body>
</html>`, safeSubject, safeSubject, appName, contentHTML, ctaHTML, appName, sentAt)
}

func renderMarketingMarkdownToHTML(markdown string) string {
	markdown = strings.ReplaceAll(markdown, "\r\n", "\n")
	lines := strings.Split(markdown, "\n")

	var b strings.Builder
	inUL := false
	inOL := false

	closeLists := func() {
		if inUL {
			b.WriteString("</ul>")
			inUL = false
		}
		if inOL {
			b.WriteString("</ol>")
			inOL = false
		}
	}

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			closeLists()
			continue
		}

		switch {
		case strings.HasPrefix(line, "### "):
			closeLists()
			b.WriteString("<h3>")
			b.WriteString(renderMarketingInlineToHTML(strings.TrimSpace(line[4:])))
			b.WriteString("</h3>")
		case strings.HasPrefix(line, "## "):
			closeLists()
			b.WriteString("<h2>")
			b.WriteString(renderMarketingInlineToHTML(strings.TrimSpace(line[3:])))
			b.WriteString("</h2>")
		case strings.HasPrefix(line, "# "):
			closeLists()
			b.WriteString("<h1>")
			b.WriteString(renderMarketingInlineToHTML(strings.TrimSpace(line[2:])))
			b.WriteString("</h1>")
		case strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* "):
			if !inUL {
				closeLists()
				b.WriteString("<ul>")
				inUL = true
			}
			b.WriteString("<li>")
			b.WriteString(renderMarketingInlineToHTML(strings.TrimSpace(line[2:])))
			b.WriteString("</li>")
		case marketingOrderedListRe.MatchString(line):
			match := marketingOrderedListRe.FindStringSubmatch(line)
			item := line
			if len(match) == 2 {
				item = strings.TrimSpace(match[1])
			}
			if !inOL {
				closeLists()
				b.WriteString("<ol>")
				inOL = true
			}
			b.WriteString("<li>")
			b.WriteString(renderMarketingInlineToHTML(item))
			b.WriteString("</li>")
		default:
			closeLists()
			b.WriteString("<p>")
			b.WriteString(renderMarketingInlineToHTML(line))
			b.WriteString("</p>")
		}
	}

	closeLists()

	return b.String()
}

func renderMarketingInlineToHTML(input string) string {
	safe := html.EscapeString(input)

	safe = marketingInlineCodeRe.ReplaceAllString(safe, "<code>$1</code>")
	safe = marketingBoldRe.ReplaceAllString(safe, "<strong>$1</strong>")
	safe = marketingItalicRe.ReplaceAllString(safe, "<em>$1</em>")

	safe = marketingLinkRe.ReplaceAllStringFunc(safe, func(m string) string {
		matches := marketingLinkRe.FindStringSubmatch(m)
		if len(matches) != 3 {
			return m
		}

		label := strings.TrimSpace(matches[1])
		url := strings.TrimSpace(html.UnescapeString(matches[2]))
		if !isSafeMarketingURL(url) {
			return label
		}

		return fmt.Sprintf(`<a href="%s" target="_blank" rel="noopener noreferrer">%s</a>`, html.EscapeString(url), label)
	})

	return safe
}

func isSafeMarketingURL(url string) bool {
	if url == "" {
		return false
	}
	lower := strings.ToLower(strings.TrimSpace(url))
	return strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "mailto:") ||
		strings.HasPrefix(lower, "/")
}

func renderMarketingMarkdownToText(markdown string) string {
	markdown = strings.ReplaceAll(markdown, "\r\n", "\n")
	lines := strings.Split(markdown, "\n")
	cleaned := make([]string, 0, len(lines))

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			cleaned = append(cleaned, "")
			continue
		}

		if strings.HasPrefix(line, "### ") {
			line = strings.TrimSpace(line[4:])
		} else if strings.HasPrefix(line, "## ") {
			line = strings.TrimSpace(line[3:])
		} else if strings.HasPrefix(line, "# ") {
			line = strings.TrimSpace(line[2:])
		} else if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			line = strings.TrimSpace(line[2:])
		} else if marketingOrderedListRe.MatchString(line) {
			match := marketingOrderedListRe.FindStringSubmatch(line)
			if len(match) == 2 {
				line = strings.TrimSpace(match[1])
			}
		}

		line = marketingLinkRe.ReplaceAllString(line, "$1 ($2)")
		line = strings.ReplaceAll(line, "**", "")
		line = strings.ReplaceAll(line, "*", "")
		line = strings.ReplaceAll(line, "`", "")
		line = strings.ReplaceAll(line, "__", "")
		line = strings.ReplaceAll(line, "_", "")
		line = strings.ReplaceAll(line, "~~", "")

		cleaned = append(cleaned, strings.TrimSpace(line))
	}

	text := strings.Join(cleaned, "\n")
	text = strings.TrimSpace(text)
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}

	return text
}
