package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/models"
	"auralogic/internal/pkg/cache"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

type SMSService struct {
	cfg           *config.Config
	db            *gorm.DB
	pluginManager *PluginManagerService
}

func NewSMSService(cfg *config.Config, db *gorm.DB) *SMSService {
	return &SMSService{cfg: cfg, db: db}
}

func (s *SMSService) SetPluginManager(pluginManager *PluginManagerService) {
	s.pluginManager = pluginManager
}

func (s *SMSService) executeSMSBeforeHook(
	phone *string,
	phoneCode *string,
	code *string,
	message *string,
	eventType string,
	userID *uint,
	batchID *uint,
) error {
	if s.pluginManager == nil {
		return nil
	}

	payload := map[string]interface{}{
		"phone":      derefString(phone),
		"phone_code": derefString(phoneCode),
		"code":       derefString(code),
		"message":    derefString(message),
		"event_type": strings.TrimSpace(eventType),
		"user_id":    userID,
		"batch_id":   batchID,
		"provider":   s.cfg.SMS.Provider,
		"source":     "sms_service",
	}
	execCtx := buildServiceHookExecutionContext(userID, nil, map[string]string{
		"hook_source": "sms_service",
		"event_type":  strings.TrimSpace(eventType),
	})
	if batchID != nil {
		execCtx.Metadata["batch_id"] = strconv.FormatUint(uint64(*batchID), 10)
	}
	hookResult, hookErr := s.pluginManager.ExecuteHook(HookExecutionRequest{
		Hook:    "sms.send.before",
		Payload: payload,
	}, execCtx)
	if hookErr != nil {
		log.Printf("sms.send.before hook execution failed: event=%s err=%v", eventType, hookErr)
		return nil
	}
	if hookResult == nil {
		return nil
	}
	if hookResult.Blocked {
		reason := strings.TrimSpace(hookResult.BlockReason)
		if reason == "" {
			reason = "SMS sending rejected by plugin"
		}
		return newHookBlockedError(reason)
	}
	if hookResult.Payload == nil {
		return nil
	}
	if phone != nil {
		if value, exists := hookResult.Payload["phone"]; exists {
			updated, convErr := serviceHookValueToString(value)
			if convErr != nil {
				log.Printf("sms.send.before phone patch ignored: event=%s err=%v", eventType, convErr)
			} else {
				*phone = strings.TrimSpace(updated)
			}
		}
	}
	if phoneCode != nil {
		if value, exists := hookResult.Payload["phone_code"]; exists {
			updated, convErr := serviceHookValueToString(value)
			if convErr != nil {
				log.Printf("sms.send.before phone_code patch ignored: event=%s err=%v", eventType, convErr)
			} else {
				*phoneCode = strings.TrimSpace(updated)
			}
		}
	}
	if code != nil {
		if value, exists := hookResult.Payload["code"]; exists {
			updated, convErr := serviceHookValueToString(value)
			if convErr != nil {
				log.Printf("sms.send.before code patch ignored: event=%s err=%v", eventType, convErr)
			} else {
				*code = strings.TrimSpace(updated)
			}
		}
	}
	if message != nil {
		if value, exists := hookResult.Payload["message"]; exists {
			updated, convErr := serviceHookValueToString(value)
			if convErr != nil {
				log.Printf("sms.send.before message patch ignored: event=%s err=%v", eventType, convErr)
			} else {
				*message = strings.TrimSpace(updated)
			}
		}
	}
	return nil
}

func (s *SMSService) emitSMSAfterHook(phone string, phoneCode string, code string, message string, eventType string, provider string, userID *uint, batchID *uint, sendErr error) {
	if s.pluginManager == nil {
		return
	}

	payload := map[string]interface{}{
		"phone":         strings.TrimSpace(phone),
		"phone_code":    strings.TrimSpace(phoneCode),
		"code":          strings.TrimSpace(code),
		"message":       strings.TrimSpace(message),
		"event_type":    strings.TrimSpace(eventType),
		"user_id":       userID,
		"batch_id":      batchID,
		"provider":      strings.TrimSpace(provider),
		"status":        models.SmsLogStatusSent,
		"error_message": "",
		"source":        "sms_service",
	}
	if sendErr != nil {
		payload["status"] = models.SmsLogStatusFailed
		payload["error_message"] = strings.TrimSpace(sendErr.Error())
	}

	go func(execCtx *ExecutionContext, hookPayload map[string]interface{}, hookEvent string) {
		_, hookErr := s.pluginManager.ExecuteHook(HookExecutionRequest{
			Hook:    "sms.send.after",
			Payload: hookPayload,
		}, execCtx)
		if hookErr != nil {
			log.Printf("sms.send.after hook execution failed: event=%s err=%v", hookEvent, hookErr)
		}
	}(cloneServiceHookExecutionContext(buildServiceHookExecutionContext(userID, nil, map[string]string{
		"hook_source": "sms_service",
		"event_type":  strings.TrimSpace(eventType),
	})), payload, eventType)
}

func (s *SMSService) SendVerificationCode(phone, phoneCode, code, eventType string) error {
	smsCfg := s.cfg.SMS
	if !smsCfg.Enabled {
		return fmt.Errorf("SMS service is not enabled")
	}

	rl := config.GetConfig().SMSRateLimit
	recipient := phoneCode + phone
	allowed, availableAt, rateLimitErr := reserveMessageRateLimitSlot("sms", recipient, rl)
	if rateLimitErr != nil {
		log.Printf("Warning: SMS rate limit reservation failed for %s: %v", recipient, rateLimitErr)
		allowed = true
	}
	if !allowed {
		if rl.ExceedAction == "delay" {
			// Store in delayed sorted set and re-check the quota when it becomes ready.
			ctx := cache.RedisClient.Context()
			payload, _ := json.Marshal(map[string]interface{}{
				"phone":      phone,
				"phone_code": phoneCode,
				"code":       code,
				"event_type": eventType,
				"created_at": time.Now().Unix(),
			})
			cache.RedisClient.ZAdd(ctx, "sms:delayed", &redis.Z{
				Score:  float64(availableAt.Unix()),
				Member: string(payload),
			})
			log.Printf("SMS rate limited for %s, delayed", recipient)
			return nil
		}
		log.Printf("SMS rate limited for %s, cancelled", recipient)
		return fmt.Errorf("SMS rate limit exceeded")
	}

	return s.sendDirect(phone, phoneCode, code, eventType)
}

// SendMarketingSMS sends marketing SMS to one user and respects user opt-in.
func (s *SMSService) SendMarketingSMS(user *models.User, content string) error {
	return s.SendMarketingSMSWithBatch(user, content, nil)
}

// SendMarketingSMSWithBatch sends marketing SMS to one user and respects user opt-in.
// batchID is optional and is used for marketing batch tracking.
func (s *SMSService) SendMarketingSMSWithBatch(user *models.User, content string, batchID *uint) error {
	if user == nil || !user.SMSNotifyMarketing || user.Phone == nil || *user.Phone == "" {
		return nil
	}
	message := strings.TrimSpace(content)
	if message == "" {
		return nil
	}

	smsCfg := s.cfg.SMS
	if !smsCfg.Enabled {
		return fmt.Errorf("SMS service is not enabled")
	}

	phone := strings.TrimSpace(*user.Phone)
	rl := config.GetConfig().SMSRateLimit
	allowed, _, rateLimitErr := reserveMessageRateLimitSlot("sms_marketing", phone, rl)
	if rateLimitErr != nil {
		log.Printf("Warning: marketing SMS rate limit reservation failed for %s: %v", phone, rateLimitErr)
		allowed = true
	}
	if !allowed {
		return fmt.Errorf("SMS rate limit exceeded")
	}

	userID := user.ID
	return s.sendMarketingDirect(phone, "", message, &userID, batchID)
}

// sendDirect sends the SMS without rate limit checks (used by delayed processing).
func (s *SMSService) sendDirect(phone, phoneCode, code, eventType string) error {
	smsCfg := s.cfg.SMS
	message := fmt.Sprintf("Verification code: %s", code)
	originalMessage := message
	if err := s.executeSMSBeforeHook(&phone, &phoneCode, &code, &message, eventType, nil, nil); err != nil {
		s.emitSMSAfterHook(phone, phoneCode, code, message, eventType, smsCfg.Provider, nil, nil, err)
		return err
	}
	if message == originalMessage {
		message = fmt.Sprintf("Verification code: %s", code)
	}

	// Strip '+' prefix from phoneCode for providers that need bare country code
	countryCode := strings.TrimPrefix(phoneCode, "+")

	var sendErr error
	switch smsCfg.Provider {
	case "aliyun":
		sendErr = s.sendAliyun(phone, countryCode, code, eventType)
	case "aliyun_dypns":
		sendErr = s.sendAliyunDYPNS(phone, countryCode, code, eventType)
	case "twilio":
		sendErr = s.sendTwilio(phoneCode+phone, code)
	case "custom":
		sendErr = s.sendCustomHTTP(phone, phoneCode, code)
	default:
		sendErr = fmt.Errorf("unknown SMS provider: %s", smsCfg.Provider)
	}

	s.logSms(phone, message, eventType, smsCfg.Provider, sendErr, nil, nil)
	s.emitSMSAfterHook(phone, phoneCode, code, message, eventType, smsCfg.Provider, nil, nil, sendErr)
	return sendErr
}

func (s *SMSService) sendMarketingDirect(phone, phoneCode, message string, userID, batchID *uint) error {
	smsCfg := s.cfg.SMS
	code := ""
	if err := s.executeSMSBeforeHook(&phone, &phoneCode, &code, &message, "marketing", userID, batchID); err != nil {
		s.emitSMSAfterHook(phone, phoneCode, code, message, "marketing", smsCfg.Provider, userID, batchID, err)
		return err
	}

	var sendErr error
	switch smsCfg.Provider {
	case "twilio":
		to := phone
		if phoneCode != "" {
			to = phoneCode + phone
		}
		sendErr = s.sendTwilioMessage(to, message)
	case "custom":
		sendErr = s.sendCustomHTTPMessage(phone, phoneCode, message)
	default:
		sendErr = fmt.Errorf("provider %s does not support marketing SMS", smsCfg.Provider)
	}

	s.logSms(phone, message, "marketing", smsCfg.Provider, sendErr, userID, batchID)
	s.emitSMSAfterHook(phone, phoneCode, code, message, "marketing", smsCfg.Provider, userID, batchID, sendErr)
	return sendErr
}

// ProcessDelayedSMS periodically moves ready items from the delayed set and sends them.
// Skips items older than 10 minutes since verification codes expire by then.
func (s *SMSService) ProcessDelayedSMS(ctx context.Context) {
	runBackgroundServiceWithContext("sms.ProcessDelayedSMS", ctx, s.processDelayedSMSLoop)
}

func (s *SMSService) processDelayedSMSLoop(ctx context.Context) {
	type delayedSMSPayload struct {
		Phone     string `json:"phone"`
		PhoneCode string `json:"phone_code"`
		Code      string `json:"code"`
		EventType string `json:"event_type"`
		CreatedAt int64  `json:"created_at,omitempty"`
	}

	if cache.RedisClient == nil {
		log.Println("ProcessDelayedSMS skipped: redis client is not initialized")
		return
	}

	redisCtx := cache.RedisClient.Context()
	for {
		select {
		case <-ctx.Done():
			log.Println("ProcessDelayedSMS shutting down")
			return
		case <-time.After(30 * time.Second):
		}
		if s.cfg == nil || !s.cfg.SMS.Enabled {
			continue
		}
		now := time.Now()
		results, err := cache.RedisClient.ZRangeByScoreWithScores(redisCtx, "sms:delayed", &redis.ZRangeBy{
			Min: "-inf", Max: fmt.Sprintf("%f", float64(now.Unix())), Count: 50,
		}).Result()
		if err != nil || len(results) == 0 {
			continue
		}
		for _, z := range results {
			payload, ok := z.Member.(string)
			if !ok {
				cache.RedisClient.ZRem(redisCtx, "sms:delayed", z.Member)
				continue
			}

			var data delayedSMSPayload
			if err := json.Unmarshal([]byte(payload), &data); err != nil {
				cache.RedisClient.ZRem(redisCtx, "sms:delayed", z.Member)
				continue
			}

			createdAtUnix := data.CreatedAt
			if createdAtUnix <= 0 {
				createdAtUnix = int64(z.Score)
			}
			createdAt := time.Unix(createdAtUnix, 0)
			if now.Sub(createdAt) > 10*time.Minute {
				log.Printf("Delayed SMS expired (created %v ago), skipping", now.Sub(createdAt))
				cache.RedisClient.ZRem(redisCtx, "sms:delayed", z.Member)
				continue
			}

			recipient := data.PhoneCode + data.Phone
			allowed, availableAt, rateLimitErr := reserveMessageRateLimitSlot("sms", recipient, config.GetConfig().SMSRateLimit)
			if rateLimitErr != nil {
				log.Printf("Warning: delayed SMS rate limit reservation failed for %s: %v", recipient, rateLimitErr)
				allowed = true
			}
			if !allowed {
				cache.RedisClient.ZAdd(redisCtx, "sms:delayed", &redis.Z{
					Score:  float64(availableAt.Unix()),
					Member: payload,
				})
				continue
			}

			cache.RedisClient.ZRem(redisCtx, "sms:delayed", z.Member)
			s.sendDirect(data.Phone, data.PhoneCode, data.Code, data.EventType)
		}
	}
}

// getTemplateCode 根据事件类型获取对应的模板代码
func (s *SMSService) getTemplateCode(eventType string) string {
	smsCfg := s.cfg.SMS
	switch eventType {
	case "login":
		if smsCfg.Templates.Login != "" {
			return smsCfg.Templates.Login
		}
	case "register":
		if smsCfg.Templates.Register != "" {
			return smsCfg.Templates.Register
		}
	case "reset_password":
		if smsCfg.Templates.ResetPassword != "" {
			return smsCfg.Templates.ResetPassword
		}
	case "bind_phone":
		if smsCfg.Templates.BindPhone != "" {
			return smsCfg.Templates.BindPhone
		}
	}
	// Fallback to the global template code
	return smsCfg.AliyunTemplateCode
}

// TestSMS 发送测试短信
func (s *SMSService) TestSMS(phone string) error {
	return s.SendVerificationCode(phone, "", "123456", "test")
}

func (s *SMSService) logSms(phone, content, eventType, provider string, sendErr error, userID, batchID *uint) {
	if s.db == nil {
		return
	}
	expireAt := time.Now().Add(10 * time.Minute)
	log := models.SmsLog{
		Phone:     phone,
		Content:   content,
		EventType: eventType,
		UserID:    userID,
		BatchID:   batchID,
		Provider:  provider,
		Status:    models.SmsLogStatusSent,
		ExpireAt:  &expireAt,
	}
	if sendErr != nil {
		log.Status = models.SmsLogStatusFailed
		log.ErrorMessage = sendErr.Error()
	} else {
		now := time.Now()
		log.SentAt = &now
	}
	s.db.Create(&log)
}

func (s *SMSService) sendAliyun(phone, countryCode, code, eventType string) error {
	smsCfg := s.cfg.SMS
	params := url.Values{}
	params.Set("AccessKeyId", smsCfg.AliyunAccessKeyID)
	params.Set("Action", "SendSms")
	params.Set("Format", "JSON")
	params.Set("PhoneNumbers", phone)
	if countryCode != "" && countryCode != "86" {
		params.Set("CountryCode", countryCode)
	}
	params.Set("SignName", smsCfg.AliyunSignName)
	params.Set("SignatureMethod", "HMAC-SHA1")
	params.Set("SignatureNonce", fmt.Sprintf("%d", time.Now().UnixNano()))
	params.Set("SignatureVersion", "1.0")
	params.Set("TemplateCode", s.getTemplateCode(eventType))
	params.Set("TemplateParam", fmt.Sprintf(`{"code":"%s"}`, code))
	params.Set("Timestamp", time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	params.Set("Version", "2017-05-25")

	params.Set("Signature", s.signAliyunParams(params))

	resp, err := http.Get("https://dysmsapi.aliyuncs.com/?" + params.Encode())
	if err != nil {
		return fmt.Errorf("aliyun SMS request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("aliyun SMS read response failed: %w", err)
	}

	var result struct {
		Code      string `json:"Code"`
		Message   string `json:"Message"`
		RequestId string `json:"RequestId"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("aliyun SMS parse response failed: %w", err)
	}
	if result.Code != "OK" {
		return fmt.Errorf("aliyun SMS failed: %s - %s (RequestId: %s)", result.Code, result.Message, result.RequestId)
	}

	return nil
}

// signAliyunParams 计算阿里云API签名
func (s *SMSService) signAliyunParams(params url.Values) string {
	smsCfg := s.cfg.SMS
	stringToSign := "GET&" + url.QueryEscape("/") + "&" + url.QueryEscape(params.Encode())
	mac := hmac.New(sha1.New, []byte(smsCfg.AliyunAccessSecret+"&"))
	mac.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func (s *SMSService) sendAliyunDYPNS(phone, countryCode, code, eventType string) error {
	smsCfg := s.cfg.SMS

	params := url.Values{}
	params.Set("AccessKeyId", smsCfg.AliyunAccessKeyID)
	params.Set("Action", "SendSmsVerifyCode")
	params.Set("Format", "JSON")
	params.Set("PhoneNumber", phone)
	if countryCode != "" && countryCode != "86" {
		params.Set("CountryCode", countryCode)
	}
	if smsCfg.DYPNSCodeLength > 0 {
		params.Set("CodeLength", fmt.Sprintf("%d", smsCfg.DYPNSCodeLength))
	}
	params.Set("SignatureMethod", "HMAC-SHA1")
	params.Set("SignatureNonce", fmt.Sprintf("%d", time.Now().UnixNano()))
	params.Set("SignatureVersion", "1.0")
	params.Set("SignName", smsCfg.AliyunSignName)
	params.Set("TemplateCode", s.getTemplateCode(eventType))
	params.Set("TemplateParam", fmt.Sprintf(`{"code":"%s","min":"10"}`, code))
	params.Set("Timestamp", time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	params.Set("Version", "2017-05-25")

	params.Set("Signature", s.signAliyunParams(params))

	resp, err := http.Get("https://dypnsapi.aliyuncs.com/?" + params.Encode())
	if err != nil {
		return fmt.Errorf("aliyun DYPNS request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("aliyun DYPNS read response failed: %w", err)
	}

	var result struct {
		Code      string `json:"Code"`
		Message   string `json:"Message"`
		RequestId string `json:"RequestId"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("aliyun DYPNS parse response failed: %w", err)
	}
	if result.Code != "OK" {
		return fmt.Errorf("aliyun DYPNS failed: %s - %s (RequestId: %s)", result.Code, result.Message, result.RequestId)
	}

	return nil
}

func (s *SMSService) sendTwilio(phone, code string) error {
	return s.sendTwilioMessage(phone, fmt.Sprintf("Your verification code is: %s", code))
}

func (s *SMSService) sendTwilioMessage(phone, body string) error {
	smsCfg := s.cfg.SMS
	apiURL := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", smsCfg.TwilioAccountSID)

	data := url.Values{}
	data.Set("To", phone)
	data.Set("From", smsCfg.TwilioFromNumber)
	data.Set("Body", body)

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}
	req.SetBasicAuth(smsCfg.TwilioAccountSID, smsCfg.TwilioAuthToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("twilio SMS request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("twilio SMS failed: %s", string(body))
	}
	return nil
}

func (s *SMSService) sendCustomHTTP(phone, phoneCode, code string) error {
	return s.sendCustomHTTPMessage(phone, phoneCode, code)
}

func (s *SMSService) sendCustomHTTPMessage(phone, phoneCode, message string) error {
	smsCfg := s.cfg.SMS
	method := smsCfg.CustomMethod
	if method == "" {
		method = "POST"
	}

	body := smsCfg.CustomBodyTemplate
	body = strings.ReplaceAll(body, "{{phone}}", phone)
	body = strings.ReplaceAll(body, "{{phone_code}}", phoneCode)
	body = strings.ReplaceAll(body, "{{code}}", message)
	body = strings.ReplaceAll(body, "{{message}}", message)

	req, err := http.NewRequest(method, smsCfg.CustomURL, bytes.NewBufferString(body))
	if err != nil {
		return err
	}
	for k, v := range smsCfg.CustomHeaders {
		req.Header.Set(k, v)
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("custom SMS request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("custom SMS failed: %s", string(respBody))
	}
	return nil
}
