package service

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"html/template"
	"log"
	"os"
	"path/filepath"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/models"
	"auralogic/internal/pkg/cache"
	"gopkg.in/gomail.v2"
	"gorm.io/gorm"
)

type EmailService struct {
	db        *gorm.DB
	cfg       *config.SMTPConfig
	appURL    string
	dialer    *gomail.Dialer
	templates map[string]*template.Template
}

func NewEmailService(db *gorm.DB, cfg *config.SMTPConfig, appURL string) *EmailService {
	service := &EmailService{
		db:        db,
		cfg:       cfg,
		appURL:    appURL,
		templates: make(map[string]*template.Template),
	}

	// 加载邮件模板（无论SMTP是否启用，模板渲染都可能被调用）
	if err := service.loadTemplates(); err != nil {
		log.Printf("Warning: Failed to load email templates: %v", err)
	}

	if !cfg.Enabled {
		return service
	}

	dialer := gomail.NewDialer(cfg.Host, cfg.Port, cfg.User, cfg.Password)
	dialer.TLSConfig = &tls.Config{
		ServerName:         cfg.Host,
		InsecureSkipVerify: false,
	}
	service.dialer = dialer

	return service
}

// loadTemplates 加载邮件模板（支持多语言: {event}_{locale}.html）
func (s *EmailService) loadTemplates() error {
	// 尝试多个路径查找模板目录
	candidates := []string{
		"templates/email",
	}

	// 基于可执行文件位置查找
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		candidates = append(candidates,
			filepath.Join(execDir, "templates", "email"),
			filepath.Join(execDir, "..", "templates", "email"),
		)
	}

	var absPath string
	for _, dir := range candidates {
		p, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			absPath = p
			break
		}
	}

	if absPath == "" {
		return fmt.Errorf("template directory not found, tried: %v", candidates)
	}

	// 事件列表
	events := []string{
		"welcome",
		"email_verification",
		"order_created",
		"order_paid",
		"order_shipped",
		"order_completed",
		"order_cancelled",
		"order_resubmit",
		"ticket_created",
		"ticket_reply",
		"ticket_resolved",
	}

	locales := []string{"zh", "en"}

	for _, event := range events {
		for _, locale := range locales {
			key := event + "_" + locale
			filename := key + ".html"
			tmplPath := filepath.Join(absPath, filename)

			tmpl, err := template.ParseFiles(tmplPath)
			if err != nil {
				log.Printf("Warning: Failed to load template %s: %v", filename, err)
				continue
			}
			s.templates[key] = tmpl
		}

		// 兼容旧模板（无语言后缀），作为 en 的回退
		oldFilename := event + ".html"
		oldPath := filepath.Join(absPath, oldFilename)
		if _, err := os.Stat(oldPath); err == nil {
			tmpl, err := template.ParseFiles(oldPath)
			if err == nil {
				// 仅当对应语言模板不存在时用作回退
				if _, exists := s.templates[event+"_en"]; !exists {
					s.templates[event+"_en"] = tmpl
				}
				if _, exists := s.templates[event+"_zh"]; !exists {
					s.templates[event+"_zh"] = tmpl
				}
			}
		}
	}

	log.Printf("Loaded %d email templates from %s", len(s.templates), absPath)
	return nil
}

// resolveLocale 解析语言，默认 en
func resolveLocale(locale string) string {
	if locale == "zh" {
		return "zh"
	}
	return "en"
}

// renderTemplate 渲染模板（支持多语言）
func (s *EmailService) renderTemplate(event, locale string, data interface{}) (string, error) {
	key := event + "_" + resolveLocale(locale)
	tmpl, ok := s.templates[key]
	if !ok {
		// 回退到 en
		tmpl, ok = s.templates[event+"_en"]
		if !ok {
			return "", fmt.Errorf("template %s not found", key)
		}
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to render template: %w", err)
	}

	return buf.String(), nil
}

// getEmailNotifyConfig 获取邮件通知配置
func getEmailNotifyConfig() *config.EmailNotificationsConfig {
	cfg := config.GetConfig()
	return &cfg.EmailNotifications
}

// getAppName 获取应用名称
func getAppName() string {
	cfg := config.GetConfig()
	if cfg.App.Name != "" {
		return cfg.App.Name
	}
	return "AuraLogic"
}

// SendEmail 发送邮件
func (s *EmailService) SendEmail(to, subject, content string) error {
	if !s.cfg.Enabled {
		log.Printf("Email service is disabled, skipping email to %s", to)
		return nil
	}

	m := gomail.NewMessage()
	m.SetHeader("From", s.cfg.FromEmail)
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", content)

	return s.dialer.DialAndSend(m)
}

// QueueEmail 将邮件加入队列
func (s *EmailService) QueueEmail(to, subject, content, eventType string, orderID, userID *uint) error {
	emailLog := &models.EmailLog{
		ToEmail:   to,
		Subject:   subject,
		Content:   content,
		EventType: eventType,
		OrderID:   orderID,
		UserID:    userID,
		Status:    models.EmailLogStatusPending,
	}

	if err := s.db.Create(emailLog).Error; err != nil {
		return err
	}

	// 将邮件ID加入Redis队列
	if err := cache.RedisClient.RPush(cache.RedisClient.Context(), "email:queue", emailLog.ID).Err(); err != nil {
		log.Printf("Failed to queue email: %v", err)
	}

	return nil
}

// ProcessEmailQueue 处理邮件队列
func (s *EmailService) ProcessEmailQueue() {
	if !s.cfg.Enabled {
		log.Println("Email service is disabled")
		return
	}

	ctx := cache.RedisClient.Context()
	for {
		// 从队列中取出邮件ID
		result, err := cache.RedisClient.BLPop(ctx, 5*time.Second, "email:queue").Result()
		if err != nil {
			continue
		}

		if len(result) < 2 {
			continue
		}

		emailID := result[1]

		// Query邮件记录
		var emailLog models.EmailLog
		if err := s.db.First(&emailLog, emailID).Error; err != nil {
			log.Printf("Failed to find email log %s: %v", emailID, err)
			continue
		}

		// 发送邮件
		if err := s.SendEmail(emailLog.ToEmail, emailLog.Subject, emailLog.Content); err != nil {
			// 发送失败
			emailLog.Status = models.EmailLogStatusFailed
			emailLog.ErrorMessage = err.Error()
			emailLog.RetryCount++

			// 如果重试次数小于3，重新加入队列
			if emailLog.RetryCount < 3 {
				cache.RedisClient.RPush(ctx, "email:queue", emailLog.ID)
			}
		} else {
			// 发送成功
			emailLog.Status = models.EmailLogStatusSent
			now := models.NowFunc()
			emailLog.SentAt = &now
		}

		s.db.Save(&emailLog)
	}
}

// ========================
// 用户注册
// ========================

// SendRegistrationWelcomeEmail 发送用户自行注册的欢迎邮件
func (s *EmailService) SendRegistrationWelcomeEmail(email, name, locale string) error {
	if !getEmailNotifyConfig().UserRegister {
		return nil
	}

	appName := getAppName()
	locale = resolveLocale(locale)

	var subject string
	if locale == "zh" {
		subject = fmt.Sprintf("欢迎注册 %s", appName)
	} else {
		subject = fmt.Sprintf("Welcome to %s", appName)
	}

	data := map[string]interface{}{
		"Name":    name,
		"Email":   email,
		"AppURL":  s.appURL,
		"AppName": appName,
	}

	content, err := s.renderTemplate("welcome", locale, data)
	if err != nil {
		log.Printf("Failed to render welcome template, using fallback: %v", err)
		if locale == "zh" {
			content = fmt.Sprintf("欢迎注册 %s！\n\n您的账户已创建成功。\n邮箱: %s\n\n登录: %s/login", appName, email, s.appURL)
		} else {
			content = fmt.Sprintf("Welcome to %s!\n\nYour account has been created.\nEmail: %s\n\nLogin: %s/login", appName, email, s.appURL)
		}
	}

	return s.QueueEmail(email, subject, content, "user.register", nil, nil)
}

// SendVerificationEmail 发送邮箱验证邮件
func (s *EmailService) SendVerificationEmail(email, name, token, locale string) error {
	appName := getAppName()
	locale = resolveLocale(locale)

	var subject string
	if locale == "zh" {
		subject = fmt.Sprintf("验证您的邮箱 - %s", appName)
	} else {
		subject = fmt.Sprintf("Verify your email - %s", appName)
	}

	verifyURL := fmt.Sprintf("%s/verify-email?token=%s", s.appURL, token)

	data := map[string]interface{}{
		"Name":      name,
		"Email":     email,
		"AppURL":    s.appURL,
		"AppName":   appName,
		"VerifyURL": verifyURL,
	}

	content, err := s.renderTemplate("email_verification", locale, data)
	if err != nil {
		log.Printf("Failed to render verification template, using fallback: %v", err)
		if locale == "zh" {
			content = fmt.Sprintf(`<html><body>
				<h2>验证您的邮箱</h2>
				<p>您好 %s，</p>
				<p>感谢注册 %s！请点击下方链接验证您的邮箱地址：</p>
				<p><a href="%s">点击验证邮箱</a></p>
				<p>此链接将在 24 小时后失效。</p>
				<p>如果您没有注册过，请忽略此邮件。</p>
			</body></html>`, name, appName, verifyURL)
		} else {
			content = fmt.Sprintf(`<html><body>
				<h2>Verify your email</h2>
				<p>Hi %s,</p>
				<p>Thanks for signing up for %s! Please click the link below to verify your email:</p>
				<p><a href="%s">Verify Email</a></p>
				<p>This link will expire in 24 hours.</p>
				<p>If you didn't sign up, please ignore this email.</p>
			</body></html>`, name, appName, verifyURL)
		}
	}

	return s.QueueEmail(email, subject, content, "user.verification", nil, nil)
}

// ========================
// 订单相关
// ========================

// SendOrderCreatedEmail 发送订单创建成功邮件
func (s *EmailService) SendOrderCreatedEmail(order *models.Order) error {
	if !getEmailNotifyConfig().OrderCreated {
		return nil
	}
	if order.UserEmail == "" || !order.EmailNotificationsEnabled {
		return nil
	}

	locale := s.getOrderLocale(order)
	appName := getAppName()

	var subject string
	if locale == "zh" {
		subject = fmt.Sprintf("订单提交成功 - %s", order.OrderNo)
	} else {
		subject = fmt.Sprintf("Order Submitted - %s", order.OrderNo)
	}

	data := map[string]interface{}{
		"OrderNo":   order.OrderNo,
		"CreatedAt": order.CreatedAt.Format("2006-01-02 15:04:05"),
		"AppURL":    s.appURL,
		"AppName":   appName,
	}

	content, err := s.renderTemplate("order_created", locale, data)
	if err != nil {
		log.Printf("Failed to render template, using fallback: %v", err)
		if locale == "zh" {
			content = fmt.Sprintf("订单创建成功！\n\n订单号: %s\n创建时间: %s\n\n查看详情: %s/orders",
				order.OrderNo, order.CreatedAt.Format("2006-01-02 15:04:05"), s.appURL)
		} else {
			content = fmt.Sprintf("Order Created Successfully!\n\nOrder No: %s\nCreated At: %s\n\nView details: %s/orders",
				order.OrderNo, order.CreatedAt.Format("2006-01-02 15:04:05"), s.appURL)
		}
	}

	return s.QueueEmail(order.UserEmail, subject, content, "order.created", &order.ID, order.UserID)
}

// SendOrderPaidEmail 发送付款确认邮件
func (s *EmailService) SendOrderPaidEmail(order *models.Order, isVirtualOnly bool) error {
	if !getEmailNotifyConfig().OrderPaid {
		return nil
	}
	if order.UserEmail == "" || !order.EmailNotificationsEnabled {
		return nil
	}

	locale := s.getOrderLocale(order)
	appName := getAppName()

	var subject string
	if locale == "zh" {
		subject = fmt.Sprintf("付款确认 - %s", order.OrderNo)
	} else {
		subject = fmt.Sprintf("Payment Confirmed - %s", order.OrderNo)
	}

	data := map[string]interface{}{
		"OrderNo":       order.OrderNo,
		"TotalAmount":   fmt.Sprintf("%.2f", order.TotalAmount),
		"Currency":      order.Currency,
		"IsVirtualOnly": isVirtualOnly,
		"AppURL":        s.appURL,
		"AppName":       appName,
		"PaidAt":        models.NowFunc().Format("2006-01-02 15:04:05"),
	}

	content, err := s.renderTemplate("order_paid", locale, data)
	if err != nil {
		log.Printf("Failed to render order_paid template, using fallback: %v", err)
		if locale == "zh" {
			if isVirtualOnly {
				content = fmt.Sprintf("付款成功！\n\n订单号: %s\n您的虚拟商品已自动发货，请登录查看。\n\n查看: %s/orders/%s", order.OrderNo, s.appURL, order.OrderNo)
			} else {
				content = fmt.Sprintf("付款成功！\n\n订单号: %s\n请填写收货信息以便我们发货。\n\n查看: %s/orders/%s", order.OrderNo, s.appURL, order.OrderNo)
			}
		} else {
			if isVirtualOnly {
				content = fmt.Sprintf("Payment Confirmed!\n\nOrder No: %s\nYour virtual products have been delivered. Please login to view.\n\nView: %s/orders/%s", order.OrderNo, s.appURL, order.OrderNo)
			} else {
				content = fmt.Sprintf("Payment Confirmed!\n\nOrder No: %s\nPlease submit your shipping information.\n\nView: %s/orders/%s", order.OrderNo, s.appURL, order.OrderNo)
			}
		}
	}

	return s.QueueEmail(order.UserEmail, subject, content, "order.paid", &order.ID, order.UserID)
}

// SendOrderShippedEmail 发送订单发货成功邮件
func (s *EmailService) SendOrderShippedEmail(order *models.Order) error {
	if !getEmailNotifyConfig().OrderShipped {
		return nil
	}
	if order.UserEmail == "" || !order.EmailNotificationsEnabled {
		return nil
	}

	locale := s.getOrderLocale(order)
	appName := getAppName()

	var subject string
	if locale == "zh" {
		subject = fmt.Sprintf("订单已发货 - %s", order.OrderNo)
	} else {
		subject = fmt.Sprintf("Order Shipped - %s", order.OrderNo)
	}

	shippedAt := ""
	if order.ShippedAt != nil {
		shippedAt = order.ShippedAt.Format("2006-01-02 15:04:05")
	}

	data := map[string]interface{}{
		"ReceiverName": order.ReceiverName,
		"OrderNo":      order.OrderNo,
		"TrackingNo":   order.TrackingNo,
		"ShippedAt":    shippedAt,
		"AppURL":       s.appURL,
		"AppName":      appName,
	}

	content, err := s.renderTemplate("order_shipped", locale, data)
	if err != nil {
		log.Printf("Failed to render template, using fallback: %v", err)
		if locale == "zh" {
			content = fmt.Sprintf("您的订单已发货！\n\n订单号: %s\n物流单号: %s\n发货时间: %s\n\n查看: %s/orders/%s",
				order.OrderNo, order.TrackingNo, shippedAt, s.appURL, order.OrderNo)
		} else {
			content = fmt.Sprintf("Your Order Has Been Shipped!\n\nOrder No: %s\nTracking No: %s\nShipped At: %s\n\nView: %s/orders/%s",
				order.OrderNo, order.TrackingNo, shippedAt, s.appURL, order.OrderNo)
		}
	}

	return s.QueueEmail(order.UserEmail, subject, content, "order.shipped", &order.ID, order.UserID)
}

// SendOrderCompletedEmail 发送订单完成邮件
func (s *EmailService) SendOrderCompletedEmail(order *models.Order) error {
	if !getEmailNotifyConfig().OrderCompleted {
		return nil
	}
	if order.UserEmail == "" || !order.EmailNotificationsEnabled {
		return nil
	}

	locale := s.getOrderLocale(order)
	appName := getAppName()

	var subject string
	if locale == "zh" {
		subject = fmt.Sprintf("订单已完成 - %s", order.OrderNo)
	} else {
		subject = fmt.Sprintf("Order Completed - %s", order.OrderNo)
	}

	completedAt := ""
	if order.CompletedAt != nil {
		completedAt = order.CompletedAt.Format("2006-01-02 15:04:05")
	}

	data := map[string]interface{}{
		"ReceiverName": order.ReceiverName,
		"OrderNo":      order.OrderNo,
		"CompletedAt":  completedAt,
		"AppURL":       s.appURL,
		"AppName":      appName,
	}

	content, err := s.renderTemplate("order_completed", locale, data)
	if err != nil {
		log.Printf("Failed to render template, using fallback: %v", err)
		if locale == "zh" {
			content = fmt.Sprintf("订单已完成！\n\n订单号: %s\n完成时间: %s\n\n感谢您的使用！", order.OrderNo, completedAt)
		} else {
			content = fmt.Sprintf("Order Completed!\n\nOrder No: %s\nCompleted At: %s\n\nThank you!", order.OrderNo, completedAt)
		}
	}

	return s.QueueEmail(order.UserEmail, subject, content, "order.completed", &order.ID, order.UserID)
}

// SendOrderResubmitEmail 发送需要重填信息邮件
func (s *EmailService) SendOrderResubmitEmail(order *models.Order, formURL string) error {
	if !getEmailNotifyConfig().OrderResubmit {
		return nil
	}
	if order.UserEmail == "" || !order.EmailNotificationsEnabled {
		return nil
	}

	locale := s.getOrderLocale(order)
	appName := getAppName()

	var subject string
	if locale == "zh" {
		subject = fmt.Sprintf("订单信息需要更正 - %s", order.OrderNo)
	} else {
		subject = fmt.Sprintf("Order Info Needs Correction - %s", order.OrderNo)
	}

	data := map[string]interface{}{
		"OrderNo": order.OrderNo,
		"Reason":  order.AdminRemark,
		"FormURL": formURL,
		"AppURL":  s.appURL,
		"AppName": appName,
	}

	content, err := s.renderTemplate("order_resubmit", locale, data)
	if err != nil {
		log.Printf("Failed to render template, using fallback: %v", err)
		if locale == "zh" {
			content = fmt.Sprintf("订单信息需要更正\n\n订单号: %s\n原因: %s\n\n重新填写: %s", order.OrderNo, order.AdminRemark, formURL)
		} else {
			content = fmt.Sprintf("Order Information Needs Correction\n\nOrder No: %s\nReason: %s\n\nResubmit: %s", order.OrderNo, order.AdminRemark, formURL)
		}
	}

	return s.QueueEmail(order.UserEmail, subject, content, "order.need_resubmit", &order.ID, order.UserID)
}

// SendOrderCancelledEmail 发送订单取消邮件
func (s *EmailService) SendOrderCancelledEmail(order *models.Order) error {
	if !getEmailNotifyConfig().OrderCancelled {
		return nil
	}
	if order.UserEmail == "" || !order.EmailNotificationsEnabled {
		return nil
	}

	locale := s.getOrderLocale(order)
	appName := getAppName()

	var subject string
	if locale == "zh" {
		subject = fmt.Sprintf("订单已取消 - %s", order.OrderNo)
	} else {
		subject = fmt.Sprintf("Order Cancelled - %s", order.OrderNo)
	}

	data := map[string]interface{}{
		"OrderNo":     order.OrderNo,
		"CancelledAt": models.NowFunc().Format("2006-01-02 15:04:05"),
		"Reason":      order.AdminRemark,
		"AppURL":      s.appURL,
		"AppName":     appName,
	}

	content, err := s.renderTemplate("order_cancelled", locale, data)
	if err != nil {
		log.Printf("Failed to render template, using fallback: %v", err)
		if locale == "zh" {
			content = fmt.Sprintf("订单已取消\n\n订单号: %s\n取消时间: %s\n原因: %s\n\n如有疑问请联系客服。",
				order.OrderNo, data["CancelledAt"], order.AdminRemark)
		} else {
			content = fmt.Sprintf("Order Cancelled\n\nOrder No: %s\nCancelled At: %s\nReason: %s\n\nPlease contact support if you have questions.",
				order.OrderNo, data["CancelledAt"], order.AdminRemark)
		}
	}

	return s.QueueEmail(order.UserEmail, subject, content, "order.cancelled", &order.ID, order.UserID)
}

// ========================
// 工单相关
// ========================

// getAdminsWithTicketPermission 获取拥有工单权限的管理员（超级管理员 + 拥有 ticket.view 权限的普通管理员）
func (s *EmailService) getAdminsWithTicketPermission() []models.User {
	var admins []models.User
	// 超级管理员拥有所有权限
	s.db.Where("role = ? AND is_active = ?", "super_admin", true).Find(&admins)

	// 查找拥有 ticket.view 权限的普通管理员
	var permAdmins []models.User
	s.db.Joins("JOIN admin_permissions ON admin_permissions.user_id = users.id AND admin_permissions.deleted_at IS NULL").
		Where("users.role = ? AND users.is_active = ? AND admin_permissions.permissions LIKE ?", "admin", true, "%ticket.view%").
		Find(&permAdmins)

	admins = append(admins, permAdmins...)
	return admins
}

// SendTicketCreatedEmail 发送新工单通知邮件给管理员
func (s *EmailService) SendTicketCreatedEmail(ticket *models.Ticket, userEmail string) error {
	if !getEmailNotifyConfig().TicketCreated {
		return nil
	}

	// 查找拥有工单权限的管理员
	admins := s.getAdminsWithTicketPermission()

	appName := getAppName()

	for _, admin := range admins {
		if admin.Email == "" {
			continue
		}

		locale := resolveLocale(admin.Locale)
		var subject string
		if locale == "zh" {
			subject = fmt.Sprintf("[新工单] %s - %s", ticket.TicketNo, ticket.Subject)
		} else {
			subject = fmt.Sprintf("[New Ticket] %s - %s", ticket.TicketNo, ticket.Subject)
		}

		data := map[string]interface{}{
			"TicketNo":  ticket.TicketNo,
			"Subject":   ticket.Subject,
			"Content":   ticket.Content,
			"Category":  ticket.Category,
			"Priority":  string(ticket.Priority),
			"UserEmail": userEmail,
			"AppURL":    s.appURL,
			"AppName":   appName,
		}

		content, err := s.renderTemplate("ticket_created", locale, data)
		if err != nil {
			log.Printf("Failed to render ticket_created template, using fallback: %v", err)
			if locale == "zh" {
				content = fmt.Sprintf("收到新工单\n\n工单号: %s\n标题: %s\n用户: %s\n优先级: %s\n\n查看: %s/admin/tickets",
					ticket.TicketNo, ticket.Subject, userEmail, ticket.Priority, s.appURL)
			} else {
				content = fmt.Sprintf("New Ticket Received\n\nTicket: %s\nSubject: %s\nUser: %s\nPriority: %s\n\nView: %s/admin/tickets",
					ticket.TicketNo, ticket.Subject, userEmail, ticket.Priority, s.appURL)
			}
		}

		adminID := admin.ID
		s.QueueEmail(admin.Email, subject, content, "ticket.created", nil, &adminID)
	}

	return nil
}

// SendTicketAdminReplyEmail 发送客服回复通知邮件给用户
func (s *EmailService) SendTicketAdminReplyEmail(ticket *models.Ticket, adminName, messagePreview string) error {
	if !getEmailNotifyConfig().TicketAdminReply {
		return nil
	}

	// 防抖：同一工单5分钟内只发一次管理员回复通知
	debounceKey := fmt.Sprintf("ticket_notify:admin_reply:%d", ticket.ID)
	if ok, err := cache.SetNX(debounceKey, 1, 5*time.Minute); err == nil && !ok {
		return nil // key已存在，跳过本次发送
	}

	// 获取工单用户
	var user models.User
	if err := s.db.First(&user, ticket.UserID).Error; err != nil {
		return err
	}
	if user.Email == "" {
		return nil
	}

	locale := resolveLocale(user.Locale)
	appName := getAppName()

	var subject string
	if locale == "zh" {
		subject = fmt.Sprintf("[工单回复] %s - %s", ticket.TicketNo, ticket.Subject)
	} else {
		subject = fmt.Sprintf("[Ticket Reply] %s - %s", ticket.TicketNo, ticket.Subject)
	}

	data := map[string]interface{}{
		"TicketNo":       ticket.TicketNo,
		"Subject":        ticket.Subject,
		"AdminName":      adminName,
		"MessagePreview": messagePreview,
		"AppURL":         s.appURL,
		"AppName":        appName,
	}

	content, err := s.renderTemplate("ticket_reply", locale, data)
	if err != nil {
		log.Printf("Failed to render ticket_reply template, using fallback: %v", err)
		if locale == "zh" {
			content = fmt.Sprintf("您的工单有新回复\n\n工单号: %s\n客服: %s\n\n消息预览:\n%s\n\n查看: %s/tickets/%d",
				ticket.TicketNo, adminName, messagePreview, s.appURL, ticket.ID)
		} else {
			content = fmt.Sprintf("New reply on your ticket\n\nTicket: %s\nAgent: %s\n\nMessage:\n%s\n\nView: %s/tickets/%d",
				ticket.TicketNo, adminName, messagePreview, s.appURL, ticket.ID)
		}
	}

	return s.QueueEmail(user.Email, subject, content, "ticket.admin_reply", nil, &user.ID)
}

// SendTicketUserReplyEmail 发送用户回复通知邮件给管理员
func (s *EmailService) SendTicketUserReplyEmail(ticket *models.Ticket, userName, messagePreview string) error {
	if !getEmailNotifyConfig().TicketUserReply {
		return nil
	}

	// 防抖：同一工单5分钟内只发一次用户回复通知
	debounceKey := fmt.Sprintf("ticket_notify:user_reply:%d", ticket.ID)
	if ok, err := cache.SetNX(debounceKey, 1, 5*time.Minute); err == nil && !ok {
		return nil // key已存在，跳过本次发送
	}

	appName := getAppName()

	// 如果工单已分配，通知分配的管理员；否则通知所有管理员
	var admins []models.User
	if ticket.AssignedTo != nil {
		var admin models.User
		if err := s.db.First(&admin, *ticket.AssignedTo).Error; err == nil && admin.Email != "" {
			admins = append(admins, admin)
		}
	}
	if len(admins) == 0 {
		admins = s.getAdminsWithTicketPermission()
	}

	for _, admin := range admins {
		if admin.Email == "" {
			continue
		}

		locale := resolveLocale(admin.Locale)
		var subject string
		if locale == "zh" {
			subject = fmt.Sprintf("[用户回复] %s - %s", ticket.TicketNo, ticket.Subject)
		} else {
			subject = fmt.Sprintf("[User Reply] %s - %s", ticket.TicketNo, ticket.Subject)
		}

		data := map[string]interface{}{
			"TicketNo":       ticket.TicketNo,
			"Subject":        ticket.Subject,
			"UserName":       userName,
			"MessagePreview": messagePreview,
			"AppURL":         s.appURL,
			"AppName":        appName,
		}

		content, err := s.renderTemplate("ticket_reply", locale, data)
		if err != nil {
			log.Printf("Failed to render ticket_reply template, using fallback: %v", err)
			if locale == "zh" {
				content = fmt.Sprintf("工单有新用户回复\n\n工单号: %s\n用户: %s\n\n消息:\n%s\n\n查看: %s/admin/tickets",
					ticket.TicketNo, userName, messagePreview, s.appURL)
			} else {
				content = fmt.Sprintf("New user reply on ticket\n\nTicket: %s\nUser: %s\n\nMessage:\n%s\n\nView: %s/admin/tickets",
					ticket.TicketNo, userName, messagePreview, s.appURL)
			}
		}

		adminID := admin.ID
		s.QueueEmail(admin.Email, subject, content, "ticket.user_reply", nil, &adminID)
	}

	return nil
}

// SendTicketResolvedEmail 发送工单已解决通知邮件给用户
func (s *EmailService) SendTicketResolvedEmail(ticket *models.Ticket) error {
	if !getEmailNotifyConfig().TicketResolved {
		return nil
	}

	var user models.User
	if err := s.db.First(&user, ticket.UserID).Error; err != nil {
		return err
	}
	if user.Email == "" {
		return nil
	}

	locale := resolveLocale(user.Locale)
	appName := getAppName()

	var subject string
	if locale == "zh" {
		subject = fmt.Sprintf("[工单已解决] %s - %s", ticket.TicketNo, ticket.Subject)
	} else {
		subject = fmt.Sprintf("[Ticket Resolved] %s - %s", ticket.TicketNo, ticket.Subject)
	}

	data := map[string]interface{}{
		"TicketNo":   ticket.TicketNo,
		"Subject":    ticket.Subject,
		"ResolvedAt": models.NowFunc().Format("2006-01-02 15:04:05"),
		"AppURL":     s.appURL,
		"AppName":    appName,
	}

	content, err := s.renderTemplate("ticket_resolved", locale, data)
	if err != nil {
		log.Printf("Failed to render ticket_resolved template, using fallback: %v", err)
		if locale == "zh" {
			content = fmt.Sprintf("您的工单已解决\n\n工单号: %s\n标题: %s\n\n如有其他问题，欢迎随时联系。\n\n查看: %s/tickets/%d",
				ticket.TicketNo, ticket.Subject, s.appURL, ticket.ID)
		} else {
			content = fmt.Sprintf("Your ticket has been resolved\n\nTicket: %s\nSubject: %s\n\nFeel free to contact us if you have any questions.\n\nView: %s/tickets/%d",
				ticket.TicketNo, ticket.Subject, s.appURL, ticket.ID)
		}
	}

	return s.QueueEmail(user.Email, subject, content, "ticket.resolved", nil, &user.ID)
}

// ========================
// 辅助方法
// ========================

// getOrderLocale 获取订单对应用户的语言偏好
func (s *EmailService) getOrderLocale(order *models.Order) string {
	if order.UserID != nil {
		var user models.User
		if err := s.db.Select("locale").First(&user, *order.UserID).Error; err == nil && user.Locale != "" {
			return resolveLocale(user.Locale)
		}
	}
	return "en"
}
