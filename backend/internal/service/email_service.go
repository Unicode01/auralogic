package service

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"html/template"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/models"
	"auralogic/internal/pkg/cache"
	"auralogic/internal/pkg/money"
	"github.com/go-redis/redis/v8"
	"gopkg.in/gomail.v2"
	"gorm.io/gorm"
)

type EmailService struct {
	db                  *gorm.DB
	cfg                 *config.SMTPConfig
	appURL              string
	dialer              *gomail.Dialer
	pluginManager       *PluginManagerService
	mu                  sync.RWMutex
	workerMu            sync.Mutex
	workersRunning      bool
	workerStopChan      chan struct{}
	workerWG            sync.WaitGroup
	templateDir         string
	templates           map[string]*template.Template
	templateSourceState map[string]emailTemplateSourceState
}

type emailTemplateSourceState struct {
	Size    int64
	ModTime time.Time
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

	service.RefreshConfig()
	return service
}

func buildSMTPDialer(cfg *config.SMTPConfig) *gomail.Dialer {
	if cfg == nil || !cfg.Enabled {
		return nil
	}

	dialer := gomail.NewDialer(cfg.Host, cfg.Port, cfg.User, cfg.Password)
	dialer.TLSConfig = &tls.Config{
		ServerName:         cfg.Host,
		InsecureSkipVerify: false,
	}
	return dialer
}

// RefreshConfig 重建运行时 SMTP 配置，并按启用状态启停后台 worker。
func (s *EmailService) RefreshConfig() {
	if s == nil {
		return
	}

	appURL := s.appURL
	if runtimeCfg := config.GetConfig(); runtimeCfg != nil {
		appURL = runtimeCfg.App.URL
	}

	s.mu.Lock()
	s.appURL = appURL
	s.dialer = buildSMTPDialer(s.cfg)
	s.mu.Unlock()

	if s.IsEnabled() {
		s.Start()
		return
	}
	s.Stop()
}

func (s *EmailService) SetPluginManager(pluginManager *PluginManagerService) {
	s.pluginManager = pluginManager
}

func (s *EmailService) IsEnabled() bool {
	if s == nil {
		return false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg != nil && s.cfg.Enabled && s.dialer != nil
}

func (s *EmailService) Start() {
	if !s.IsEnabled() {
		return
	}

	s.workerMu.Lock()
	defer s.workerMu.Unlock()

	if s.workersRunning {
		return
	}

	stopChan := make(chan struct{})
	s.workerStopChan = stopChan
	s.workersRunning = true
	s.workerWG.Add(2)

	go func() {
		defer s.workerWG.Done()
		s.processEmailQueueLoop(stopChan)
	}()
	go func() {
		defer s.workerWG.Done()
		s.processDelayedEmailsLoop(stopChan)
	}()
}

func (s *EmailService) Stop() {
	s.workerMu.Lock()
	defer s.workerMu.Unlock()

	if !s.workersRunning {
		return
	}

	close(s.workerStopChan)
	s.workerStopChan = nil
	s.workersRunning = false
	s.workerWG.Wait()
}

// loadTemplates 加载邮件模板（支持多语言: {event}_{locale}.html）
func (s *EmailService) loadTemplates() error {
	return s.ReloadTemplates()
}

// ReloadTemplates 重新加载邮件模板，用于管理端编辑或运行时热更新后立即生效。
func (s *EmailService) ReloadTemplates() error {
	templateDir, err := s.resolveTemplateDir()
	if err != nil {
		return err
	}

	templates, sourceState, err := loadEmailTemplatesFromDir(templateDir)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.templateDir = templateDir
	s.templates = templates
	s.templateSourceState = sourceState
	s.mu.Unlock()

	log.Printf("Loaded %d email templates from %s", len(templates), templateDir)
	return nil
}

func (s *EmailService) resolveTemplateDir() (string, error) {
	s.mu.RLock()
	cachedDir := strings.TrimSpace(s.templateDir)
	s.mu.RUnlock()

	// 尝试多个路径查找模板目录
	candidates := make([]string, 0, 4)
	if cachedDir != "" {
		candidates = append(candidates, cachedDir)
	}
	candidates = append(candidates, "templates/email")

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
		trimmedDir := strings.TrimSpace(dir)
		if trimmedDir == "" {
			continue
		}
		p, err := filepath.Abs(trimmedDir)
		if err != nil {
			continue
		}
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			absPath = p
			break
		}
	}

	if absPath == "" {
		return "", fmt.Errorf("template directory not found, tried: %v", candidates)
	}
	return absPath, nil
}

func loadEmailTemplatesFromDir(templateDir string) (map[string]*template.Template, map[string]emailTemplateSourceState, error) {
	entries, err := os.ReadDir(templateDir)
	if err != nil {
		return nil, nil, fmt.Errorf("read email template directory failed: %w", err)
	}

	templates := make(map[string]*template.Template)
	sourceState := make(map[string]emailTemplateSourceState)
	legacyFallbacks := make(map[string]*template.Template)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".html") {
			continue
		}
		fullPath := filepath.Join(templateDir, entry.Name())
		info, infoErr := entry.Info()
		if infoErr != nil {
			log.Printf("Warning: Failed to stat email template %s: %v", entry.Name(), infoErr)
			continue
		}
		tmpl, parseErr := template.ParseFiles(fullPath)
		if parseErr != nil {
			log.Printf("Warning: Failed to load template %s: %v", entry.Name(), parseErr)
			sourceState[entry.Name()] = emailTemplateSourceState{
				Size:    info.Size(),
				ModTime: info.ModTime().UTC(),
			}
			continue
		}

		key, event, locale := pluginHostParseEmailTemplateFilename(entry.Name())
		if locale == "" {
			legacyFallbacks[event] = tmpl
		} else {
			templates[key] = tmpl
		}
		sourceState[entry.Name()] = emailTemplateSourceState{
			Size:    info.Size(),
			ModTime: info.ModTime().UTC(),
		}
	}

	for event, tmpl := range legacyFallbacks {
		if _, exists := templates[event+"_en"]; !exists {
			templates[event+"_en"] = tmpl
		}
		if _, exists := templates[event+"_zh"]; !exists {
			templates[event+"_zh"] = tmpl
		}
	}

	return templates, sourceState, nil
}

func snapshotEmailTemplateSourceState(templateDir string) (map[string]emailTemplateSourceState, error) {
	entries, err := os.ReadDir(templateDir)
	if err != nil {
		return nil, err
	}

	sourceState := make(map[string]emailTemplateSourceState)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".html") {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return nil, infoErr
		}
		sourceState[entry.Name()] = emailTemplateSourceState{
			Size:    info.Size(),
			ModTime: info.ModTime().UTC(),
		}
	}
	return sourceState, nil
}

func emailTemplateSourceStateEqual(left map[string]emailTemplateSourceState, right map[string]emailTemplateSourceState) bool {
	if len(left) != len(right) {
		return false
	}
	for key, leftValue := range left {
		rightValue, exists := right[key]
		if !exists {
			return false
		}
		if leftValue.Size != rightValue.Size || !leftValue.ModTime.Equal(rightValue.ModTime) {
			return false
		}
	}
	return true
}

func (s *EmailService) refreshTemplatesIfChanged() {
	s.mu.RLock()
	templateDir := strings.TrimSpace(s.templateDir)
	currentState := make(map[string]emailTemplateSourceState, len(s.templateSourceState))
	for key, value := range s.templateSourceState {
		currentState[key] = value
	}
	s.mu.RUnlock()

	if templateDir == "" {
		if err := s.ReloadTemplates(); err != nil {
			log.Printf("Warning: Failed to reload email templates: %v", err)
		}
		return
	}

	nextState, err := snapshotEmailTemplateSourceState(templateDir)
	if err != nil {
		log.Printf("Warning: Failed to stat email templates for hot reload: %v", err)
		return
	}
	if emailTemplateSourceStateEqual(currentState, nextState) {
		return
	}
	if err := s.ReloadTemplates(); err != nil {
		log.Printf("Warning: Failed to hot reload email templates: %v", err)
	}
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
	s.refreshTemplatesIfChanged()

	key := event + "_" + resolveLocale(locale)
	s.mu.RLock()
	tmpl, ok := s.templates[key]
	if !ok {
		// 回退到 en
		tmpl, ok = s.templates[event+"_en"]
	}
	s.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("template %s not found", key)
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

func (s *EmailService) canSendOrderEmail(order *models.Order) bool {
	if order.UserEmail == "" || !order.EmailNotificationsEnabled {
		return false
	}
	if order.UserID == nil {
		return true
	}

	var user models.User
	if err := s.db.Select("email_notify_order").First(&user, *order.UserID).Error; err != nil {
		// Fail-open to avoid dropping transactional emails when user record lookup fails.
		return true
	}
	return user.EmailNotifyOrder
}

func (s *EmailService) canSendTicketEmail(userID uint) bool {
	var user models.User
	if err := s.db.Select("email_notify_ticket").First(&user, userID).Error; err != nil {
		// Fail-open to avoid dropping transactional emails when user record lookup fails.
		return true
	}
	return user.EmailNotifyTicket
}

// SendMarketingAnnouncementEmail sends a marketing message by email for one user,
// respecting user-level marketing opt-in.
func (s *EmailService) SendMarketingAnnouncementEmail(user *models.User, title, content string) error {
	return s.SendMarketingAnnouncementEmailWithBatch(user, title, content, nil)
}

// SendMarketingAnnouncementEmailWithBatch sends a marketing message by email for one user,
// respecting user-level marketing opt-in and attaching marketing batch id.
func (s *EmailService) SendMarketingAnnouncementEmailWithBatch(user *models.User, title, content string, batchID *uint) error {
	if user == nil || user.Email == "" || !user.EmailVerified || !user.EmailNotifyMarketing {
		return nil
	}
	if strings.TrimSpace(content) == "" {
		return nil
	}

	subject := strings.TrimSpace(title)
	if subject == "" {
		appName := getAppName()
		if resolveLocale(user.Locale) == "zh" {
			subject = fmt.Sprintf("营销通知 - %s", appName)
		} else {
			subject = fmt.Sprintf("Marketing Message - %s", appName)
		}
	}

	userID := user.ID
	return s.queueEmail(user.Email, subject, content, "marketing.announcement", nil, &userID, batchID)
}

// SendEmail 发送邮件
func (s *EmailService) SendEmail(to, subject, content string) error {
	s.mu.RLock()
	enabled := s.cfg != nil && s.cfg.Enabled && s.dialer != nil
	fromEmail := ""
	var dialer *gomail.Dialer
	if s.cfg != nil {
		fromEmail = s.cfg.FromEmail
	}
	dialer = s.dialer
	s.mu.RUnlock()

	if !enabled {
		log.Printf("Email service is disabled, skipping email to %s", to)
		return nil
	}

	m := gomail.NewMessage()
	m.SetHeader("From", fromEmail)
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", content)

	return dialer.DialAndSend(m)
}

func buildEmailHookPayload(emailLog *models.EmailLog) map[string]interface{} {
	if emailLog == nil {
		return map[string]interface{}{}
	}

	return map[string]interface{}{
		"email_id":      emailLog.ID,
		"to":            emailLog.ToEmail,
		"subject":       emailLog.Subject,
		"content":       emailLog.Content,
		"event_type":    emailLog.EventType,
		"order_id":      emailLog.OrderID,
		"user_id":       emailLog.UserID,
		"batch_id":      emailLog.BatchID,
		"status":        emailLog.Status,
		"error_message": emailLog.ErrorMessage,
		"retry_count":   emailLog.RetryCount,
		"expire_at":     emailLog.ExpireAt,
		"sent_at":       emailLog.SentAt,
		"created_at":    emailLog.CreatedAt,
		"updated_at":    emailLog.UpdatedAt,
	}
}

func (s *EmailService) applyEmailSendBeforeHook(emailLog *models.EmailLog) error {
	if s.pluginManager == nil || emailLog == nil {
		return nil
	}

	payload := buildEmailHookPayload(emailLog)
	payload["source"] = "email_queue"
	execCtx := buildServiceHookExecutionContext(emailLog.UserID, emailLog.OrderID, map[string]string{
		"hook_source": "email_queue",
		"email_id":    strconv.FormatUint(uint64(emailLog.ID), 10),
		"event_type":  strings.TrimSpace(emailLog.EventType),
	})
	hookResult, hookErr := s.pluginManager.ExecuteHook(HookExecutionRequest{
		Hook:    "email.send.before",
		Payload: payload,
	}, execCtx)
	if hookErr != nil {
		log.Printf("email.send.before hook execution failed: email=%d err=%v", emailLog.ID, hookErr)
		return nil
	}
	if hookResult == nil {
		return nil
	}
	if hookResult.Blocked {
		reason := strings.TrimSpace(hookResult.BlockReason)
		if reason == "" {
			reason = "Email sending rejected by plugin"
		}
		return newHookBlockedError(reason)
	}
	if hookResult.Payload == nil {
		return nil
	}
	if value, exists := hookResult.Payload["to"]; exists {
		updated, convErr := serviceHookValueToString(value)
		if convErr != nil {
			log.Printf("email.send.before to patch ignored: email=%d err=%v", emailLog.ID, convErr)
		} else {
			emailLog.ToEmail = strings.TrimSpace(updated)
		}
	}
	if value, exists := hookResult.Payload["subject"]; exists {
		updated, convErr := serviceHookValueToString(value)
		if convErr != nil {
			log.Printf("email.send.before subject patch ignored: email=%d err=%v", emailLog.ID, convErr)
		} else {
			emailLog.Subject = strings.TrimSpace(updated)
		}
	}
	if value, exists := hookResult.Payload["content"]; exists {
		updated, convErr := serviceHookValueToString(value)
		if convErr != nil {
			log.Printf("email.send.before content patch ignored: email=%d err=%v", emailLog.ID, convErr)
		} else {
			emailLog.Content = updated
		}
	}
	return nil
}

func (s *EmailService) emitEmailSendAfterHook(emailLog *models.EmailLog) {
	if s.pluginManager == nil || emailLog == nil {
		return
	}

	payload := buildEmailHookPayload(emailLog)
	payload["source"] = "email_queue"
	go func(execCtx *ExecutionContext, hookPayload map[string]interface{}, emailID uint) {
		_, hookErr := s.pluginManager.ExecuteHook(HookExecutionRequest{
			Hook:    "email.send.after",
			Payload: hookPayload,
		}, execCtx)
		if hookErr != nil {
			log.Printf("email.send.after hook execution failed: email=%d err=%v", emailID, hookErr)
		}
	}(cloneServiceHookExecutionContext(buildServiceHookExecutionContext(emailLog.UserID, emailLog.OrderID, map[string]string{
		"hook_source": "email_queue",
		"email_id":    strconv.FormatUint(uint64(emailLog.ID), 10),
		"event_type":  strings.TrimSpace(emailLog.EventType),
	})), payload, emailLog.ID)
}

// QueueEmail 将邮件加入队列
func (s *EmailService) QueueEmail(to, subject, content, eventType string, orderID, userID *uint) error {
	return s.queueEmail(to, subject, content, eventType, orderID, userID, nil)
}

func (s *EmailService) queueEmail(to, subject, content, eventType string, orderID, userID, batchID *uint) error {
	if !s.IsEnabled() {
		return nil
	}
	if s.db == nil {
		return fmt.Errorf("email log database is not initialized")
	}
	if cache.RedisClient == nil {
		return fmt.Errorf("email queue redis client is not initialized")
	}

	rl := config.GetConfig().EmailRateLimit

	allowed, availableAt, rateLimitErr := reserveMessageRateLimitSlot("email", to, rl)
	if rateLimitErr != nil {
		log.Printf("Warning: email rate limit reservation failed for %s: %v", to, rateLimitErr)
		allowed = true
	}

	if !allowed {
		if rl.ExceedAction == "delay" {
			// Push to delayed sorted set, use same 30-min ExpireAt as normal emails
			expireAt := time.Now().Add(30 * time.Minute)
			emailLog := &models.EmailLog{
				ToEmail:   to,
				Subject:   subject,
				Content:   content,
				EventType: eventType,
				OrderID:   orderID,
				UserID:    userID,
				BatchID:   batchID,
				Status:    models.EmailLogStatusPending,
				ExpireAt:  &expireAt,
			}
			if err := s.db.Create(emailLog).Error; err != nil {
				return err
			}
			ctx := cache.RedisClient.Context()
			cache.RedisClient.ZAdd(ctx, "email:delayed", &redis.Z{
				Score:  float64(availableAt.Unix()),
				Member: emailLog.ID,
			})
			return nil
		}
		// cancel: silent skip
		log.Printf("Email rate limited for %s, skipping", to)
		return nil
	}

	expireAt := time.Now().Add(30 * time.Minute)
	emailLog := &models.EmailLog{
		ToEmail:   to,
		Subject:   subject,
		Content:   content,
		EventType: eventType,
		OrderID:   orderID,
		UserID:    userID,
		BatchID:   batchID,
		Status:    models.EmailLogStatusPending,
		ExpireAt:  &expireAt,
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

// ProcessDelayedEmails periodically moves ready items from the delayed set to the main queue.
func (s *EmailService) ProcessDelayedEmails() {
	s.processDelayedEmailsLoop(nil)
}

func (s *EmailService) processDelayedEmailsLoop(stopChan <-chan struct{}) {
	if !s.IsEnabled() || cache.RedisClient == nil {
		return
	}

	ctx := cache.RedisClient.Context()
	for {
		timer := time.NewTimer(30 * time.Second)
		select {
		case <-stopChan:
			timer.Stop()
			return
		case <-timer.C:
		}

		now := float64(time.Now().Unix())
		results, err := cache.RedisClient.ZRangeByScore(ctx, "email:delayed", &redis.ZRangeBy{
			Min: "-inf", Max: fmt.Sprintf("%f", now), Count: 50,
		}).Result()
		if err != nil || len(results) == 0 {
			continue
		}
		for _, idStr := range results {
			var emailLog models.EmailLog
			if err := s.db.First(&emailLog, idStr).Error; err != nil {
				cache.RedisClient.ZRem(ctx, "email:delayed", idStr)
				continue
			}

			allowed, availableAt, rateLimitErr := reserveMessageRateLimitSlot("email", emailLog.ToEmail, config.GetConfig().EmailRateLimit)
			if rateLimitErr != nil {
				log.Printf("Warning: delayed email rate limit reservation failed for email=%s id=%s: %v", emailLog.ToEmail, idStr, rateLimitErr)
				allowed = true
			}
			if !allowed {
				cache.RedisClient.ZAdd(ctx, "email:delayed", &redis.Z{
					Score:  float64(availableAt.Unix()),
					Member: idStr,
				})
				continue
			}

			cache.RedisClient.ZRem(ctx, "email:delayed", idStr)
			cache.RedisClient.RPush(ctx, "email:queue", idStr)
		}
	}
}

// ProcessEmailQueue 处理邮件队列
func (s *EmailService) ProcessEmailQueue() {
	s.processEmailQueueLoop(nil)
}

func (s *EmailService) processEmailQueueLoop(stopChan <-chan struct{}) {
	if !s.IsEnabled() || cache.RedisClient == nil {
		log.Println("Email service is disabled")
		return
	}

	ctx := cache.RedisClient.Context()
	for {
		select {
		case <-stopChan:
			return
		default:
		}

		// 从队列中取出邮件ID
		result, err := cache.RedisClient.BLPop(ctx, 5*time.Second, "email:queue").Result()
		if err != nil {
			select {
			case <-stopChan:
				return
			default:
			}
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

		// TTL检查：如果邮件已过期则跳过发送
		if emailLog.ExpireAt != nil && time.Now().After(*emailLog.ExpireAt) {
			emailLog.Status = models.EmailLogStatusExpired
			if err := s.db.Save(&emailLog).Error; err != nil {
				log.Printf("Failed to update expired email log %s: %v", emailID, err)
				continue
			}
			s.emitEmailSendAfterHook(&emailLog)
			s.syncMarketingTaskStatus(&emailLog)
			continue
		}

		if err := s.applyEmailSendBeforeHook(&emailLog); err != nil {
			emailLog.Status = models.EmailLogStatusFailed
			emailLog.ErrorMessage = err.Error()
			if isHookBlockedError(err) {
				emailLog.RetryCount = 3
			} else {
				emailLog.RetryCount++
			}
			if saveErr := s.db.Save(&emailLog).Error; saveErr != nil {
				log.Printf("Failed to save blocked email log %s: %v", emailID, saveErr)
				continue
			}
			s.emitEmailSendAfterHook(&emailLog)
			s.syncMarketingTaskStatus(&emailLog)
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

		if err := s.db.Save(&emailLog).Error; err != nil {
			log.Printf("Failed to save email log %s: %v", emailID, err)
			continue
		}
		s.emitEmailSendAfterHook(&emailLog)
		s.syncMarketingTaskStatus(&emailLog)
	}
}

func (s *EmailService) syncMarketingTaskStatus(emailLog *models.EmailLog) {
	if s.db == nil || emailLog == nil || emailLog.BatchID == nil || emailLog.UserID == nil {
		return
	}
	if emailLog.EventType != "marketing.announcement" {
		return
	}

	var (
		taskStatus  models.MarketingTaskStatus
		errMessage  string
		shouldApply bool
	)

	switch emailLog.Status {
	case models.EmailLogStatusSent:
		taskStatus = models.MarketingTaskStatusSent
		shouldApply = true
	case models.EmailLogStatusFailed:
		// Failed with retries remaining is not a final state yet.
		if emailLog.RetryCount >= 3 {
			taskStatus = models.MarketingTaskStatusFailed
			errMessage = trimEmailError(emailLog.ErrorMessage)
			shouldApply = true
		}
	case models.EmailLogStatusExpired:
		taskStatus = models.MarketingTaskStatusFailed
		errMessage = trimEmailError(emailLog.ErrorMessage)
		if errMessage == "" {
			errMessage = "email expired"
		}
		shouldApply = true
	}

	if !shouldApply {
		return
	}

	updates := map[string]interface{}{
		"status":        taskStatus,
		"processed_at":  models.NowFunc(),
		"error_message": "",
	}
	if taskStatus == models.MarketingTaskStatusFailed {
		updates["error_message"] = errMessage
	}

	if err := s.db.Model(&models.MarketingBatchTask{}).
		Where("batch_id = ? AND user_id = ? AND channel = ? AND status IN ?", *emailLog.BatchID, *emailLog.UserID, models.MarketingTaskChannelEmail, []models.MarketingTaskStatus{
			models.MarketingTaskStatusPending,
			models.MarketingTaskStatusQueued,
		}).
		Updates(updates).Error; err != nil {
		log.Printf("Failed to sync marketing email task status, batch=%d user=%d: %v", *emailLog.BatchID, *emailLog.UserID, err)
		return
	}

	s.refreshMarketingBatchStats(*emailLog.BatchID)
}

func (s *EmailService) refreshMarketingBatchStats(batchID uint) {
	if batchID == 0 || s.db == nil {
		return
	}

	type aggRow struct {
		Channel models.MarketingTaskChannel `gorm:"column:channel"`
		Status  models.MarketingTaskStatus  `gorm:"column:status"`
		Count   int64                       `gorm:"column:count"`
	}

	var total int64
	if err := s.db.Model(&models.MarketingBatchTask{}).Where("batch_id = ?", batchID).Count(&total).Error; err != nil {
		log.Printf("Failed to count marketing batch tasks, batch=%d: %v", batchID, err)
		return
	}

	var processed int64
	if err := s.db.Model(&models.MarketingBatchTask{}).
		Where("batch_id = ? AND status IN ?", batchID, []models.MarketingTaskStatus{
			models.MarketingTaskStatusSent,
			models.MarketingTaskStatusFailed,
			models.MarketingTaskStatusSkipped,
		}).
		Count(&processed).Error; err != nil {
		log.Printf("Failed to count processed marketing tasks, batch=%d: %v", batchID, err)
		return
	}

	var unresolved int64
	if err := s.db.Model(&models.MarketingBatchTask{}).
		Where("batch_id = ? AND status IN ?", batchID, []models.MarketingTaskStatus{
			models.MarketingTaskStatusPending,
			models.MarketingTaskStatusQueued,
		}).
		Count(&unresolved).Error; err != nil {
		log.Printf("Failed to count unresolved marketing tasks, batch=%d: %v", batchID, err)
		return
	}

	var rows []aggRow
	if err := s.db.Model(&models.MarketingBatchTask{}).
		Select("channel, status, COUNT(*) as count").
		Where("batch_id = ?", batchID).
		Group("channel, status").
		Scan(&rows).Error; err != nil {
		log.Printf("Failed to aggregate marketing batch stats, batch=%d: %v", batchID, err)
		return
	}

	emailSent := int64(0)
	emailFailed := int64(0)
	emailSkipped := int64(0)
	smsSent := int64(0)
	smsFailed := int64(0)
	smsSkipped := int64(0)
	for _, row := range rows {
		switch row.Channel {
		case models.MarketingTaskChannelEmail:
			switch row.Status {
			case models.MarketingTaskStatusSent:
				emailSent = row.Count
			case models.MarketingTaskStatusFailed:
				emailFailed = row.Count
			case models.MarketingTaskStatusSkipped:
				emailSkipped = row.Count
			}
		case models.MarketingTaskChannelSMS:
			switch row.Status {
			case models.MarketingTaskStatusSent:
				smsSent = row.Count
			case models.MarketingTaskStatusFailed:
				smsFailed = row.Count
			case models.MarketingTaskStatusSkipped:
				smsSkipped = row.Count
			}
		}
	}

	var batch models.MarketingBatch
	if err := s.db.Select("id", "status").First(&batch, batchID).Error; err != nil {
		log.Printf("Failed to query marketing batch, batch=%d: %v", batchID, err)
		return
	}

	updates := map[string]interface{}{
		"total_tasks":     int(total),
		"processed_tasks": int(processed),
		"email_sent":      int(emailSent),
		"email_failed":    int(emailFailed),
		"email_skipped":   int(emailSkipped),
		"sms_sent":        int(smsSent),
		"sms_failed":      int(smsFailed),
		"sms_skipped":     int(smsSkipped),
	}

	if batch.Status != models.MarketingBatchStatusFailed {
		if unresolved == 0 {
			now := models.NowFunc()
			updates["status"] = models.MarketingBatchStatusCompleted
			updates["completed_at"] = now
		} else {
			updates["status"] = models.MarketingBatchStatusRunning
			updates["completed_at"] = nil
		}
	}

	if err := s.db.Model(&models.MarketingBatch{}).Where("id = ?", batchID).Updates(updates).Error; err != nil {
		log.Printf("Failed to update marketing batch stats from email queue, batch=%d: %v", batchID, err)
	}
}

func trimEmailError(msg string) string {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return ""
	}
	const maxLen = 1000
	if len(msg) <= maxLen {
		return msg
	}
	return msg[:maxLen]
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

// SendLoginCodeEmail 发送登录验证码邮件
func (s *EmailService) SendLoginCodeEmail(email, code, locale string) error {
	if !s.cfg.Enabled {
		log.Printf("Email service is disabled, skipping login code to %s", email)
		return nil
	}

	appName := getAppName()
	locale = resolveLocale(locale)

	var subject string
	if locale == "zh" {
		subject = fmt.Sprintf("登录验证码 - %s", appName)
	} else {
		subject = fmt.Sprintf("Login Verification Code - %s", appName)
	}

	data := map[string]interface{}{
		"Code":    code,
		"AppName": appName,
		"AppURL":  s.appURL,
	}

	content, err := s.renderTemplate("login_code", locale, data)
	if err != nil {
		if locale == "zh" {
			content = fmt.Sprintf("<h2>登录验证码</h2><p>您的验证码是：<strong>%s</strong></p><p>验证码有效期为 10 分钟。</p>", code)
		} else {
			content = fmt.Sprintf("<h2>Login Verification Code</h2><p>Your code is: <strong>%s</strong></p><p>This code expires in 10 minutes.</p>", code)
		}
	}

	return s.QueueEmail(email, subject, content, "user.login_code", nil, nil)
}

// SendPasswordResetEmail 发送密码重置邮件
func (s *EmailService) SendPasswordResetEmail(email, token, locale string) error {
	if !s.cfg.Enabled {
		return nil
	}

	appName := getAppName()
	locale = resolveLocale(locale)

	var subject string
	if locale == "zh" {
		subject = fmt.Sprintf("重置密码 - %s", appName)
	} else {
		subject = fmt.Sprintf("Reset Password - %s", appName)
	}

	resetURL := fmt.Sprintf("%s/reset-password?token=%s", s.appURL, token)

	data := map[string]interface{}{
		"ResetURL": resetURL,
		"AppName":  appName,
		"AppURL":   s.appURL,
	}

	content, err := s.renderTemplate("password_reset", locale, data)
	if err != nil {
		if locale == "zh" {
			content = fmt.Sprintf("<h2>重置密码</h2><p>请点击以下链接重置您的密码：</p><p><a href=\"%s\">重置密码</a></p><p>此链接将在 30 分钟后失效。</p>", resetURL)
		} else {
			content = fmt.Sprintf("<h2>Reset Password</h2><p>Click the link below to reset your password:</p><p><a href=\"%s\">Reset Password</a></p><p>This link expires in 30 minutes.</p>", resetURL)
		}
	}

	return s.QueueEmail(email, subject, content, "user.password_reset", nil, nil)
}

// ========================
// 订单相关
// ========================

// SendOrderCreatedEmail 发送订单创建成功邮件
func (s *EmailService) SendOrderCreatedEmail(order *models.Order) error {
	if !getEmailNotifyConfig().OrderCreated {
		return nil
	}
	if !s.canSendOrderEmail(order) {
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
	if !s.canSendOrderEmail(order) {
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
		"TotalAmount":   money.MinorToString(order.TotalAmount),
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
	if !s.canSendOrderEmail(order) {
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
	if !s.canSendOrderEmail(order) {
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
	if !s.canSendOrderEmail(order) {
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
	if !s.canSendOrderEmail(order) {
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
		if admin.Email == "" || !admin.EmailNotifyTicket {
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
	if !s.canSendTicketEmail(user.ID) {
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
		if admin.Email == "" || !admin.EmailNotifyTicket {
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
	if !s.canSendTicketEmail(user.ID) {
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
