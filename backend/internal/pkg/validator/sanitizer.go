package validator

import (
	"html"
	"regexp"
	"strings"
	"unicode/utf8"
)

// 危险标签和属性的正则表达式
var (
	scriptTagRe     = regexp.MustCompile(`(?i)<script[^>]*>.*?</script>`)
	eventHandlerRe  = regexp.MustCompile(`(?i)\s+on\w+\s*=`)
	javascriptRe    = regexp.MustCompile(`(?i)javascript:`)
	dataURIRe       = regexp.MustCompile(`(?i)data:`)
	iframeRe        = regexp.MustCompile(`(?i)<iframe[^>]*>.*?</iframe>`)
	styleTagRe      = regexp.MustCompile(`(?i)<style[^>]*>.*?</style>`)
	linkTagRe       = regexp.MustCompile(`(?i)<link[^>]*>`)
	metaTagRe       = regexp.MustCompile(`(?i)<meta[^>]*>`)
	baseTagRe       = regexp.MustCompile(`(?i)<base[^>]*>`)
	objectTagRe     = regexp.MustCompile(`(?i)<object[^>]*>.*?</object>`)
	embedTagRe      = regexp.MustCompile(`(?i)<embed[^>]*>`)
	formTagRe       = regexp.MustCompile(`(?i)<form[^>]*>.*?</form>`)
	inputTagRe      = regexp.MustCompile(`(?i)<input[^>]*>`)
	expressionRe    = regexp.MustCompile(`(?i)expression\s*\(`)
	vbscriptRe      = regexp.MustCompile(`(?i)vbscript:`)
)

// SanitizeInput 清理User输入，防止XSS
func SanitizeInput(input string) string {
	// 移除前后空白
	input = strings.TrimSpace(input)
	// HTML转义，防止XSS
	input = html.EscapeString(input)
	return input
}

// SanitizeText 清理文本字段（允许换行）
func SanitizeText(input string) string {
	// 先转义HTML
	input = html.EscapeString(input)
	// 移除前后空白但保留内部换行
	return strings.TrimSpace(input)
}

// SanitizeMarkdown 清理Markdown内容，移除危险的HTML但保留Markdown语法
func SanitizeMarkdown(input string) string {
	// 移除危险的脚本标签
	input = scriptTagRe.ReplaceAllString(input, "")
	// 移除事件处理器
	input = eventHandlerRe.ReplaceAllString(input, " ")
	// 移除javascript协议
	input = javascriptRe.ReplaceAllString(input, "")
	// 移除data URI
	input = dataURIRe.ReplaceAllString(input, "")
	// 移除iframe
	input = iframeRe.ReplaceAllString(input, "")
	// 移除style标签
	input = styleTagRe.ReplaceAllString(input, "")
	// 移除link标签
	input = linkTagRe.ReplaceAllString(input, "")
	// 移除meta标签
	input = metaTagRe.ReplaceAllString(input, "")
	// 移除base标签
	input = baseTagRe.ReplaceAllString(input, "")
	// 移除object标签
	input = objectTagRe.ReplaceAllString(input, "")
	// 移除embed标签
	input = embedTagRe.ReplaceAllString(input, "")
	// 移除form标签
	input = formTagRe.ReplaceAllString(input, "")
	// 移除input标签
	input = inputTagRe.ReplaceAllString(input, "")
	// 移除CSS expression
	input = expressionRe.ReplaceAllString(input, "")
	// 移除vbscript协议
	input = vbscriptRe.ReplaceAllString(input, "")

	return strings.TrimSpace(input)
}

// ValidateLength 验证字符串长度
func ValidateLength(s string, minLen, maxLen int) bool {
	length := utf8.RuneCountInString(s)
	if minLen > 0 && length < minLen {
		return false
	}
	if maxLen > 0 && length > maxLen {
		return false
	}
	return true
}

// ValidatePhone 验证电话号码格式（仅数字、空格、短横线、加号、括号）
func ValidatePhone(phone string) bool {
	// 允许数字、空格、+、-、()
	match, _ := regexp.MatchString(`^[\d\s\+\-\(\)]+$`, phone)
	return match
}

// ValidatePostcode 验证邮政编码格式
func ValidatePostcode(postcode string) bool {
	if postcode == "" {
		return true // 允许为空
	}
	// 允许字母数字和短横线
	match, _ := regexp.MatchString(`^[A-Za-z0-9\-\s]+$`, postcode)
	return match
}

// ValidateCountryCode 验证国家代码（2-3位字母）
func ValidateCountryCode(code string) bool {
	if code == "" {
		return true // 允许为空
	}
	match, _ := regexp.MatchString(`^[A-Z]{2,3}$`, code)
	return match
}

// ValidatePhoneCode 验证电话区号
func ValidatePhoneCode(code string) bool {
	if code == "" {
		return true // 允许为空
	}
	// 必须以+开头，后跟1-4位数字
	match, _ := regexp.MatchString(`^\+\d{1,4}$`, code)
	return match
}

// TruncateString 截断字符串到指定长度（字符数，非字节数）
func TruncateString(s string, maxLen int) string {
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}

	runes := []rune(s)
	if len(runes) > maxLen {
		return string(runes[:maxLen])
	}
	return s
}

// RemoveControlChars 移除控制字符（保留换行和制表符）
func RemoveControlChars(s string) string {
	var builder strings.Builder
	for _, r := range s {
		// 保留可打印字符、换行、制表符
		if r >= 32 || r == '\n' || r == '\t' || r == '\r' {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}
