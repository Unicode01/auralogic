package admin

import (
	"errors"
	"strings"

	"auralogic/internal/middleware"
	"auralogic/internal/models"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	marketingAudiencePreviewDefaultSampleLimit = 6
	marketingAudiencePreviewMaxSampleLimit     = 20
)

func resolveMarketingAudienceMode(
	raw string,
	targetAll bool,
	userIDs []uint,
	audienceQuery *service.MarketingAudienceNode,
) (service.MarketingAudienceMode, error) {
	mode, err := service.NormalizeMarketingAudienceMode(raw)
	if err != nil {
		return "", err
	}
	if mode != "" {
		return mode, nil
	}
	if targetAll {
		return service.MarketingAudienceModeAll, nil
	}
	if len(userIDs) > 0 {
		return service.MarketingAudienceModeSelected, nil
	}
	if service.HasMeaningfulMarketingAudienceQuery(audienceQuery) {
		return service.MarketingAudienceModeRules, nil
	}
	return service.MarketingAudienceModeAll, nil
}

func buildMarketingAudienceSnapshot(
	mode service.MarketingAudienceMode,
	audienceQuery *service.MarketingAudienceNode,
) (map[string]interface{}, error) {
	if mode != service.MarketingAudienceModeRules || audienceQuery == nil {
		return nil, nil
	}
	return service.MarketingAudienceNodeToMap(audienceQuery)
}

func normalizeMarketingAudienceSampleLimit(raw *int) int {
	if raw == nil || *raw <= 0 {
		return marketingAudiencePreviewDefaultSampleLimit
	}
	if *raw > marketingAudiencePreviewMaxSampleLimit {
		return marketingAudiencePreviewMaxSampleLimit
	}
	return *raw
}

func (h *MarketingHandler) resolveMarketingAudienceUserQuery(
	mode service.MarketingAudienceMode,
	userIDs []uint,
	audienceQuery *service.MarketingAudienceNode,
	strict bool,
) (*gorm.DB, error) {
	query := h.db.Model(&models.User{}).Where("role = ? AND is_active = ?", "user", true)

	switch mode {
	case service.MarketingAudienceModeAll:
		return query, nil
	case service.MarketingAudienceModeSelected:
		userIDs = uniqueUserIDs(userIDs)
		if len(userIDs) == 0 {
			if strict {
				return nil, errors.New("user IDs are required when audience_mode is selected")
			}
			return query.Where("1 = 0"), nil
		}
		return query.Where("id IN ?", userIDs), nil
	case service.MarketingAudienceModeRules:
		if !service.HasMeaningfulMarketingAudienceQuery(audienceQuery) {
			if strict {
				return nil, errors.New("audience_query is required when audience_mode is rules")
			}
			return query.Where("1 = 0"), nil
		}
		return service.ApplyMarketingAudienceQuery(query, audienceQuery)
	default:
		return nil, errors.New("unsupported audience mode")
	}
}

func (h *MarketingHandler) buildMarketingAudiencePreview(
	query *gorm.DB,
	mode service.MarketingAudienceMode,
	sampleLimit int,
	includeSamples bool,
) (gin.H, error) {
	if query == nil {
		return nil, errors.New("audience query is required")
	}

	var matchedUsers int64
	if err := query.Session(&gorm.Session{}).Count(&matchedUsers).Error; err != nil {
		return nil, err
	}

	var emailableUsers int64
	if err := query.Session(&gorm.Session{}).
		Where("TRIM(email) <> ''").
		Where("email_verified = ? AND email_notify_marketing = ?", true, true).
		Count(&emailableUsers).Error; err != nil {
		return nil, err
	}

	var smsReachableUsers int64
	if err := query.Session(&gorm.Session{}).
		Where("phone IS NOT NULL AND TRIM(phone) <> ''").
		Where("sms_notify_marketing = ?", true).
		Count(&smsReachableUsers).Error; err != nil {
		return nil, err
	}

	sampleUsers := make([]gin.H, 0)
	if includeSamples {
		var users []models.User
		if err := query.Session(&gorm.Session{}).
			Select(
				"id",
				"name",
				"email",
				"phone",
				"is_active",
				"email_verified",
				"locale",
				"country",
				"email_notify_marketing",
				"sms_notify_marketing",
				"total_order_count",
				"total_spent_minor",
				"last_login_at",
				"created_at",
			).
			Order("id DESC").
			Limit(sampleLimit).
			Find(&users).Error; err != nil {
			return nil, err
		}

		sampleUsers = make([]gin.H, 0, len(users))
		for i := range users {
			sampleUsers = append(sampleUsers, buildMarketingAudienceSampleUser(users[i]))
		}
	}

	return gin.H{
		"mode":                mode,
		"matched_users":       matchedUsers,
		"emailable_users":     emailableUsers,
		"sms_reachable_users": smsReachableUsers,
		"sample_users":        sampleUsers,
	}, nil
}

func (h *MarketingHandler) canViewMarketingRecipientSamples(c *gin.Context) bool {
	if c == nil {
		return false
	}

	if middleware.IsAPIKeyAuth(c) {
		scopes, exists := c.Get("api_scopes")
		if !exists {
			return false
		}
		scopeList, ok := scopes.([]string)
		if !ok {
			return false
		}
		hasMarketingView := false
		hasUserView := false
		for _, scope := range scopeList {
			switch strings.TrimSpace(scope) {
			case "marketing.view":
				hasMarketingView = true
			case "user.view":
				hasUserView = true
			}
		}
		return hasMarketingView && hasUserView
	}

	role, exists := middleware.GetUserRole(c)
	if exists && role == "super_admin" {
		return true
	}

	userID, exists := middleware.GetUserID(c)
	if !exists {
		return false
	}

	var perm models.AdminPermission
	if err := h.db.Where("user_id = ?", userID).First(&perm).Error; err != nil {
		return false
	}

	return perm.HasPermission("marketing.view") && perm.HasPermission("user.view")
}

func buildMarketingAudienceSampleUser(user models.User) gin.H {
	item := gin.H{
		"id":                     user.ID,
		"name":                   strings.TrimSpace(user.Name),
		"email":                  strings.TrimSpace(user.Email),
		"is_active":              user.IsActive,
		"email_verified":         user.EmailVerified,
		"locale":                 strings.TrimSpace(user.Locale),
		"country":                strings.TrimSpace(user.Country),
		"email_notify_marketing": user.EmailNotifyMarketing,
		"sms_notify_marketing":   user.SMSNotifyMarketing,
		"total_order_count":      user.TotalOrderCount,
		"total_spent_minor":      user.TotalSpentMinor,
		"last_login_at":          user.LastLoginAt,
		"created_at":             user.CreatedAt,
	}
	if user.Phone != nil {
		item["phone"] = strings.TrimSpace(*user.Phone)
	}
	return item
}
