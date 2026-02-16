package form

import (
	"strings"

	"github.com/gin-gonic/gin"
	"auralogic/internal/config"
	"auralogic/internal/database"
	"auralogic/internal/pkg/constants"
	"auralogic/internal/pkg/logger"
	"auralogic/internal/pkg/response"
	"auralogic/internal/pkg/validator"
	"auralogic/internal/service"
)

type ShippingHandler struct {
	orderService *service.OrderService
	cfg          *config.Config
}

func NewShippingHandler(orderService *service.OrderService, cfg *config.Config) *ShippingHandler {
	return &ShippingHandler{
		orderService: orderService,
		cfg:          cfg,
	}
}

// GetForm Get form information
func (h *ShippingHandler) GetForm(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		response.BadRequest(c, "Form token is missing")
		return
	}

	order, err := h.orderService.OrderRepo.FindByFormToken(token)
	if err != nil {
		response.NotFound(c, "Form not found or expired")
		return
	}

	// Check if form has been submitted
	if order.FormSubmittedAt != nil {
		response.BadRequest(c, "Form has already been submitted")
		return
	}

	response.Success(c, gin.H{
		"order_no":         order.OrderNo,
		"items":            order.Items,
		"platform":         order.SourcePlatform,
		"external_user_id": order.ExternalUserID,
		"user_email":       order.UserEmail,        // Email from third-party platform, form will auto-fill and lock
		"user_name":        order.ExternalUserName, // Username from third-party platform, used as default receiver name
		"expires_at":       order.FormExpiresAt,
	})
}

// SubmitFormRequest Submit form request
type SubmitFormRequest struct {
	FormToken        string `json:"form_token" binding:"required"`
	ReceiverName     string `json:"receiver_name" binding:"required"`
	PhoneCode        string `json:"phone_code"` // Phone code (e.g.: +86)
	ReceiverPhone    string `json:"receiver_phone" binding:"required"`
	ReceiverEmail    string `json:"receiver_email" binding:"required,email"` // Will use order's user_email, cannot be modified
	ReceiverCountry  string `json:"receiver_country"`                        // Receiver country code (e.g.: CN)
	ReceiverProvince string `json:"receiver_province"`                       // Province/State (required for domestic)
	ReceiverCity     string `json:"receiver_city"`                           // City (required for domestic)
	ReceiverDistrict string `json:"receiver_district"`                       // District (required for domestic)
	ReceiverAddress  string `json:"receiver_address" binding:"required"`
	ReceiverPostcode string `json:"receiver_postcode"`
	PrivacyProtected bool   `json:"privacy_protected"`
	Password         string `json:"password"`
	UserRemark       string `json:"user_remark"` // User remark
}

// SubmitForm Submit shipping information form
func (h *ShippingHandler) SubmitForm(c *gin.Context) {
	var req SubmitFormRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	// ============= Input validation and sanitization (prevent XSS and oversized input) =============

	// 1. Sanitize and validate receiver name (max 100 characters)
	req.ReceiverName = validator.SanitizeInput(req.ReceiverName)
	if !validator.ValidateLength(req.ReceiverName, 1, 100) {
		response.BadRequest(c, "Receiver name length must be between 1-100 characters")
		return
	}

	// 2. Validate and sanitize phone code
	req.PhoneCode = validator.SanitizeInput(req.PhoneCode)
	if req.PhoneCode != "" && !validator.ValidatePhoneCode(req.PhoneCode) {
		response.BadRequest(c, "Invalid phone code format (e.g.: +86)")
		return
	}

	// 3. Sanitize and validate phone number (max 50 characters)
	req.ReceiverPhone = validator.SanitizeInput(req.ReceiverPhone)
	if !validator.ValidateLength(req.ReceiverPhone, 1, 50) {
		response.BadRequest(c, "Phone number length must be between 1-50 characters")
		return
	}
	if !validator.ValidatePhone(req.ReceiverPhone) {
		response.BadRequest(c, "Invalid phone number format")
		return
	}

	// 4. Sanitize email (max 255 characters)
	req.ReceiverEmail = validator.SanitizeInput(req.ReceiverEmail)
	if !validator.ValidateLength(req.ReceiverEmail, 0, 255) {
		response.BadRequest(c, "Email length cannot exceed 255 characters")
		return
	}

	// 5. Validate and sanitize country code
	req.ReceiverCountry = strings.ToUpper(validator.SanitizeInput(req.ReceiverCountry))
	if !validator.ValidateCountryCode(req.ReceiverCountry) && req.ReceiverCountry != "" {
		response.BadRequest(c, "Invalid country code format")
		return
	}

	// 6. Sanitize province (max 50 characters)
	req.ReceiverProvince = validator.SanitizeInput(req.ReceiverProvince)
	if !validator.ValidateLength(req.ReceiverProvince, 0, 50) {
		response.BadRequest(c, "Province length cannot exceed 50 characters")
		return
	}

	// 7. Sanitize city (max 50 characters)
	req.ReceiverCity = validator.SanitizeInput(req.ReceiverCity)
	if !validator.ValidateLength(req.ReceiverCity, 0, 50) {
		response.BadRequest(c, "City length cannot exceed 50 characters")
		return
	}

	// 8. Sanitize district (max 50 characters)
	req.ReceiverDistrict = validator.SanitizeInput(req.ReceiverDistrict)
	if !validator.ValidateLength(req.ReceiverDistrict, 0, 50) {
		response.BadRequest(c, "District length cannot exceed 50 characters")
		return
	}

	// 9. Sanitize detailed address (max 500 characters)
	req.ReceiverAddress = validator.SanitizeText(req.ReceiverAddress)
	if !validator.ValidateLength(req.ReceiverAddress, 1, 500) {
		response.BadRequest(c, "Detailed address length must be between 1-500 characters")
		return
	}

	// 10. Sanitize and validate postal code (max 20 characters)
	req.ReceiverPostcode = validator.SanitizeInput(req.ReceiverPostcode)
	if !validator.ValidateLength(req.ReceiverPostcode, 0, 20) {
		response.BadRequest(c, "Postal code length cannot exceed 20 characters")
		return
	}
	if req.ReceiverPostcode != "" && !validator.ValidatePostcode(req.ReceiverPostcode) {
		response.BadRequest(c, "Invalid postal code format")
		return
	}

	// 11. Sanitize user remark (max 1000 characters)
	req.UserRemark = validator.SanitizeText(req.UserRemark)
	if !validator.ValidateLength(req.UserRemark, 0, 1000) {
		response.BadRequest(c, "User remark length cannot exceed 1000 characters")
		return
	}

	// 12. Sanitize password (if provided, max 100 characters)
	if req.Password != "" {
		// Password doesn't need HTML escaping, but needs length limitation
		if !validator.ValidateLength(req.Password, 6, 100) {
			response.BadRequest(c, "Password length must be between 6-100 characters")
			return
		}
	}

	// ============= Business logic processing =============

	// Query order first, validate email
	order, err := h.orderService.OrderRepo.FindByFormToken(req.FormToken)
	if err != nil {
		response.BadRequest(c, "Form not found or expired")
		return
	}

	// Force use of user_email from order, ignore receiver_email from frontend
	// This ensures email cannot be tampered with
	actualEmail := order.UserEmail
	if actualEmail == "" {
		// If user_email was not provided when order was created, use the one from frontend
		actualEmail = req.ReceiverEmail
	}

	// Default country is China
	receiverCountry := req.ReceiverCountry
	if receiverCountry == "" {
		receiverCountry = "CN"
	}

	// Default phone code is +86
	phoneCode := req.PhoneCode
	if phoneCode == "" {
		phoneCode = "+86"
	}

	// Build receiver information
	receiverInfo := map[string]interface{}{
		"receiver_name":     req.ReceiverName,
		"phone_code":        phoneCode, // Phone code
		"receiver_phone":    req.ReceiverPhone,
		"receiver_email":    actualEmail, // Use order's user_email
		"receiver_country":  receiverCountry,
		"receiver_province": req.ReceiverProvince,
		"receiver_city":     req.ReceiverCity,
		"receiver_district": req.ReceiverDistrict,
		"receiver_address":  req.ReceiverAddress,
		"receiver_postcode": req.ReceiverPostcode,
	}

	// Submit form
	submittedOrder, user, isNewUser, err := h.orderService.SubmitShippingForm(
		req.FormToken,
		receiverInfo,
		req.PrivacyProtected,
		req.Password,
		req.UserRemark,
	)
	if err != nil {
		// Log failure
		db := database.GetDB()
		logger.LogOperation(db, c, "form_submit_failed", "order", nil, map[string]interface{}{
			"form_token": req.FormToken,
			"error":      err.Error(),
		})
		response.BadRequest(c, err.Error())
		return
	}

	// Log success
	db := database.GetDB()
	logger.LogOrderOperation(db, c, "form_submit", submittedOrder.ID, map[string]interface{}{
		"order_no":          submittedOrder.OrderNo,
		"user_id":           user.ID,
		"user_email":        user.Email,
		"is_new_user":       isNewUser,
		"privacy_protected": req.PrivacyProtected,
		"fixed_email":       actualEmail, // Log the fixed email used
	})

	// If new user was created, also log user creation
	if isNewUser {
		logger.LogUserOperation(db, c, "auto_create", user.ID, map[string]interface{}{
			"email":    user.Email,
			"name":     user.Name,
			"source":   "form_submission",
			"order_no": submittedOrder.OrderNo,
		})
	}

	message := "Shipping information submitted successfully"
	if req.PrivacyProtected {
		message = "Shipping information submitted successfully. Privacy protection enabled, only shipping managers can view complete information"
	}
	if isNewUser {
		message += ". Initial password has been sent to your email"
	}

	response.Success(c, gin.H{
		"order_no": submittedOrder.OrderNo,
		"user": gin.H{
			"user_id": user.ID,
			"email":   user.Email,
			"is_new":  isNewUser,
		},
		"privacy_protected": req.PrivacyProtected,
		"message":           message,
		"login_url":         h.cfg.App.URL + "/login",
	})
}

// GetCountries Get list of supported countries
func (h *ShippingHandler) GetCountries(c *gin.Context) {
	response.Success(c, constants.Countries)
}
