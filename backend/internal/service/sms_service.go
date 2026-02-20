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
	"strings"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/models"
	"auralogic/internal/pkg/cache"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

type SMSService struct {
	cfg *config.Config
	db  *gorm.DB
}

func NewSMSService(cfg *config.Config, db *gorm.DB) *SMSService {
	return &SMSService{cfg: cfg, db: db}
}

func (s *SMSService) SendVerificationCode(phone, phoneCode, code, eventType string) error {
	smsCfg := s.cfg.SMS
	if !smsCfg.Enabled {
		return fmt.Errorf("SMS service is not enabled")
	}

	// Check rate limit
	rl := config.GetConfig().SMSRateLimit
	recipient := phoneCode + phone
	if checkMessageRateLimit("sms", recipient, rl) {
		if rl.ExceedAction == "delay" {
			// Store in delayed sorted set with creation timestamp as score
			ctx := cache.RedisClient.Context()
			payload, _ := json.Marshal(map[string]string{
				"phone": phone, "phone_code": phoneCode, "code": code, "event_type": eventType,
			})
			cache.RedisClient.ZAdd(ctx, "sms:delayed", &redis.Z{
				Score:  float64(time.Now().Unix()),
				Member: string(payload),
			})
			log.Printf("SMS rate limited for %s, delayed", recipient)
			return nil
		}
		log.Printf("SMS rate limited for %s, cancelled", recipient)
		return fmt.Errorf("SMS rate limit exceeded")
	}

	incrMessageRateCounters("sms", recipient)
	return s.sendDirect(phone, phoneCode, code, eventType)
}

// sendDirect sends the SMS without rate limit checks (used by delayed processing).
func (s *SMSService) sendDirect(phone, phoneCode, code, eventType string) error {
	smsCfg := s.cfg.SMS

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

	s.logSms(phone, fmt.Sprintf("Verification code: %s", code), eventType, smsCfg.Provider, sendErr)
	return sendErr
}

// ProcessDelayedSMS periodically moves ready items from the delayed set and sends them.
// Skips items older than 10 minutes since verification codes expire by then.
func (s *SMSService) ProcessDelayedSMS(ctx context.Context) {
	redisCtx := cache.RedisClient.Context()
	for {
		select {
		case <-ctx.Done():
			log.Println("ProcessDelayedSMS shutting down")
			return
		case <-time.After(30 * time.Second):
		}
		now := time.Now()
		results, err := cache.RedisClient.ZRangeByScoreWithScores(redisCtx, "sms:delayed", &redis.ZRangeBy{
			Min: "-inf", Max: fmt.Sprintf("%f", float64(now.Unix())), Count: 50,
		}).Result()
		if err != nil || len(results) == 0 {
			continue
		}
		for _, z := range results {
			cache.RedisClient.ZRem(redisCtx, "sms:delayed", z.Member)
			// Skip if older than 10 minutes (verification code already expired)
			createdAt := time.Unix(int64(z.Score), 0)
			if now.Sub(createdAt) > 10*time.Minute {
				log.Printf("Delayed SMS expired (created %v ago), skipping", now.Sub(createdAt))
				continue
			}
			var data map[string]string
			if err := json.Unmarshal([]byte(z.Member.(string)), &data); err != nil {
				continue
			}
			s.sendDirect(data["phone"], data["phone_code"], data["code"], data["event_type"])
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

func (s *SMSService) logSms(phone, content, eventType, provider string, sendErr error) {
	if s.db == nil {
		return
	}
	expireAt := time.Now().Add(10 * time.Minute)
	log := models.SmsLog{
		Phone:     phone,
		Content:   content,
		EventType: eventType,
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
	smsCfg := s.cfg.SMS
	apiURL := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", smsCfg.TwilioAccountSID)

	data := url.Values{}
	data.Set("To", phone)
	data.Set("From", smsCfg.TwilioFromNumber)
	data.Set("Body", fmt.Sprintf("Your verification code is: %s", code))

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
	smsCfg := s.cfg.SMS
	method := smsCfg.CustomMethod
	if method == "" {
		method = "POST"
	}

	body := smsCfg.CustomBodyTemplate
	body = strings.ReplaceAll(body, "{{phone}}", phone)
	body = strings.ReplaceAll(body, "{{phone_code}}", phoneCode)
	body = strings.ReplaceAll(body, "{{code}}", code)

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
