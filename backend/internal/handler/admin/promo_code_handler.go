package admin

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"auralogic/internal/models"
	"auralogic/internal/pkg/logger"
	"auralogic/internal/pkg/response"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type PromoCodeHandler struct {
	promoCodeService *service.PromoCodeService
	pluginManager    *service.PluginManagerService
	db               *gorm.DB
}

func NewPromoCodeHandler(promoCodeService *service.PromoCodeService, pluginManager *service.PluginManagerService, db *gorm.DB) *PromoCodeHandler {
	return &PromoCodeHandler{
		promoCodeService: promoCodeService,
		pluginManager:    pluginManager,
		db:               db,
	}
}

// CreatePromoCodeRequest 创建优惠码请求
type CreatePromoCodeRequest struct {
	Code                string              `json:"code" binding:"required"`
	Name                string              `json:"name" binding:"required"`
	Description         string              `json:"description"`
	DiscountType        models.DiscountType `json:"discount_type" binding:"required"`
	DiscountValueMinor  int64               `json:"discount_value_minor" binding:"required,gt=0"`
	MaxDiscountMinor    int64               `json:"max_discount_minor"`
	MinOrderAmountMinor int64               `json:"min_order_amount_minor"`
	TotalQuantity       int                 `json:"total_quantity"`
	ProductIDs          []uint              `json:"product_ids"`
	ProductScope        string              `json:"product_scope"`
	Status              string              `json:"status"`
	ExpiresAt           *string             `json:"expires_at"`
}

// UpdatePromoCodeRequest 更新优惠码请求（不需要code字段）
type UpdatePromoCodeRequest struct {
	Name                string              `json:"name" binding:"required"`
	Description         string              `json:"description"`
	DiscountType        models.DiscountType `json:"discount_type" binding:"required"`
	DiscountValueMinor  int64               `json:"discount_value_minor" binding:"required,gt=0"`
	MaxDiscountMinor    int64               `json:"max_discount_minor"`
	MinOrderAmountMinor int64               `json:"min_order_amount_minor"`
	TotalQuantity       int                 `json:"total_quantity"`
	ProductIDs          []uint              `json:"product_ids"`
	ProductScope        string              `json:"product_scope"`
	Status              string              `json:"status"`
	ExpiresAt           *string             `json:"expires_at"`
}

type promoCodeImportResult struct {
	Message      string   `json:"message"`
	ConflictMode string   `json:"conflict_mode"`
	TotalRows    int      `json:"total_rows"`
	CreatedCount int      `json:"created_count"`
	UpdatedCount int      `json:"updated_count"`
	SkippedCount int      `json:"skipped_count"`
	ErrorCount   int      `json:"error_count"`
	Errors       []string `json:"errors,omitempty"`
}

const promoCodeImportMaxErrors = 100

func parsePromoCodeExpiryInput(value *string) (*time.Time, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}

	candidate := strings.TrimSpace(*value)
	t, err := time.Parse(time.RFC3339, candidate)
	if err != nil {
		t, err = time.Parse(adminCSVTimeFormat, candidate)
		if err != nil {
			t, err = time.Parse("2006-01-02T15:04:05", candidate)
			if err != nil {
				t, err = time.Parse("2006-01-02", candidate)
				if err != nil {
					return nil, err
				}
				t = t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
			}
		}
	}
	return &t, nil
}

func normalizePromoCodeImportHeader(value string) string {
	normalized := strings.TrimSpace(strings.TrimPrefix(value, "\uFEFF"))
	normalized = strings.ToLower(normalized)
	return strings.NewReplacer(" ", "", "_", "", "-", "", ".", "", "/", "").Replace(normalized)
}

func canonicalPromoCodeImportHeader(value string) string {
	switch normalizePromoCodeImportHeader(value) {
	case "id", "编号":
		return "id"
	case "code", "promocode", "优惠码":
		return "code"
	case "name", "名称":
		return "name"
	case "description", "desc", "描述":
		return "description"
	case "discounttype", "折扣类型":
		return "discount_type"
	case "discountvalueminor", "discountvalue", "折扣值", "折扣值分":
		return "discount_value_minor"
	case "maxdiscountminor", "maxdiscount", "最大折扣金额", "最大折扣金额分":
		return "max_discount_minor"
	case "minorderamountminor", "minorderamount", "最低订单金额", "最低订单金额分":
		return "min_order_amount_minor"
	case "totalquantity", "总数量":
		return "total_quantity"
	case "usedquantity", "已使用":
		return "used_quantity"
	case "reservedquantity", "预留中":
		return "reserved_quantity"
	case "productscope", "商品范围":
		return "product_scope"
	case "productids", "productid", "商品id", "商品ids":
		return "product_ids"
	case "status", "状态":
		return "status"
	case "expiresat", "expiry", "到期时间":
		return "expires_at"
	case "createdat", "创建时间":
		return "created_at"
	case "updatedat", "更新时间":
		return "updated_at"
	default:
		return ""
	}
}

func buildPromoCodeImportHeaderMap(header []string) (map[string]int, error) {
	headerMap := make(map[string]int, len(header))
	for idx, value := range header {
		canonical := canonicalPromoCodeImportHeader(value)
		if canonical == "" {
			continue
		}
		if _, exists := headerMap[canonical]; !exists {
			headerMap[canonical] = idx
		}
	}

	required := []string{"code", "name", "discount_type", "discount_value_minor"}
	missing := make([]string, 0, len(required))
	for _, field := range required {
		if _, exists := headerMap[field]; !exists {
			missing = append(missing, field)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required columns: %s", strings.Join(missing, ", "))
	}

	return headerMap, nil
}

func promoCodeImportCell(record []string, headerMap map[string]int, field string) string {
	index, exists := headerMap[field]
	if !exists || index < 0 || index >= len(record) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(record[index], "\uFEFF"))
}

func parsePromoCodeImportConflictMode(value string) (string, error) {
	mode := strings.ToLower(strings.TrimSpace(value))
	if mode == "" {
		return "upsert", nil
	}
	switch mode {
	case "skip", "update", "upsert":
		return mode, nil
	default:
		return "", fmt.Errorf("invalid conflict_mode: %s", value)
	}
}

func parsePromoCodeImportStatus(value string) (models.PromoCodeStatus, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return models.PromoCodeStatusActive, nil
	}
	switch normalized {
	case string(models.PromoCodeStatusActive), "启用":
		return models.PromoCodeStatusActive, nil
	case string(models.PromoCodeStatusInactive), "停用":
		return models.PromoCodeStatusInactive, nil
	default:
		return "", fmt.Errorf("invalid status: %s", value)
	}
}

func parsePromoCodeImportDiscountType(value string) (models.DiscountType, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case string(models.DiscountTypePercentage), "percent", "percentagediscount", "百分比", "百分比折扣":
		return models.DiscountTypePercentage, nil
	case string(models.DiscountTypeFixed), "fixedamount", "固定金额":
		return models.DiscountTypeFixed, nil
	default:
		return "", fmt.Errorf("invalid discount_type: %s", value)
	}
}

func parsePromoCodeImportProductScope(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return "all", nil
	}
	switch normalized {
	case "all", "所有商品":
		return "all", nil
	case "specific", "specified", "指定商品":
		return "specific", nil
	case "exclude", "排除指定商品":
		return "exclude", nil
	default:
		return "", fmt.Errorf("invalid product_scope: %s", value)
	}
}

func parsePromoCodeImportInt64(value, field string, required bool) (int64, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		if required {
			return 0, fmt.Errorf("%s is required", field)
		}
		return 0, nil
	}

	parsed, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", field)
	}
	if parsed < 0 {
		return 0, fmt.Errorf("%s cannot be negative", field)
	}
	return parsed, nil
}

func parsePromoCodeImportInt(value, field string) (int, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, nil
	}

	parsed, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", field)
	}
	if parsed < 0 {
		return 0, fmt.Errorf("%s cannot be negative", field)
	}
	return parsed, nil
}

func parsePromoCodeImportProductIDs(value string) ([]uint, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}

	if strings.HasPrefix(trimmed, "[") {
		var ids []uint
		if err := json.Unmarshal([]byte(trimmed), &ids); err == nil {
			return ids, nil
		}

		var generic []int64
		if err := json.Unmarshal([]byte(trimmed), &generic); err == nil {
			result := make([]uint, 0, len(generic))
			for _, item := range generic {
				if item < 0 {
					return nil, fmt.Errorf("product_ids cannot contain negative values")
				}
				result = append(result, uint(item))
			}
			return result, nil
		}
	}

	parts := strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == ',' || r == '，' || r == ';' || r == '；' || r == '|' || r == '\n' || r == '\r'
	})
	result := make([]uint, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		parsed, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid product_id: %s", part)
		}
		result = append(result, uint(parsed))
	}
	return result, nil
}

func buildPromoCodeImportModel(record []string, headerMap map[string]int) (*models.PromoCode, bool, error) {
	blank := true
	for _, value := range record {
		if strings.TrimSpace(strings.TrimPrefix(value, "\uFEFF")) != "" {
			blank = false
			break
		}
	}
	if blank {
		return nil, true, nil
	}

	code := strings.ToUpper(strings.TrimSpace(promoCodeImportCell(record, headerMap, "code")))
	if code == "" {
		return nil, false, fmt.Errorf("code is required")
	}

	name := strings.TrimSpace(promoCodeImportCell(record, headerMap, "name"))
	if name == "" {
		return nil, false, fmt.Errorf("name is required")
	}

	discountType, err := parsePromoCodeImportDiscountType(promoCodeImportCell(record, headerMap, "discount_type"))
	if err != nil {
		return nil, false, err
	}

	discountValue, err := parsePromoCodeImportInt64(promoCodeImportCell(record, headerMap, "discount_value_minor"), "discount_value_minor", true)
	if err != nil {
		return nil, false, err
	}
	if discountValue <= 0 {
		return nil, false, fmt.Errorf("discount_value_minor must be greater than 0")
	}
	if discountType == models.DiscountTypePercentage && discountValue > 10000 {
		return nil, false, fmt.Errorf("discount_value_minor cannot exceed 10000 for percentage discounts")
	}

	maxDiscount, err := parsePromoCodeImportInt64(promoCodeImportCell(record, headerMap, "max_discount_minor"), "max_discount_minor", false)
	if err != nil {
		return nil, false, err
	}

	minOrderAmount, err := parsePromoCodeImportInt64(promoCodeImportCell(record, headerMap, "min_order_amount_minor"), "min_order_amount_minor", false)
	if err != nil {
		return nil, false, err
	}

	totalQuantity, err := parsePromoCodeImportInt(promoCodeImportCell(record, headerMap, "total_quantity"), "total_quantity")
	if err != nil {
		return nil, false, err
	}

	productScope, err := parsePromoCodeImportProductScope(promoCodeImportCell(record, headerMap, "product_scope"))
	if err != nil {
		return nil, false, err
	}

	productIDs, err := parsePromoCodeImportProductIDs(promoCodeImportCell(record, headerMap, "product_ids"))
	if err != nil {
		return nil, false, err
	}
	if productScope == "all" {
		productIDs = nil
	}
	if (productScope == "specific" || productScope == "exclude") && len(productIDs) == 0 {
		return nil, false, fmt.Errorf("product_ids are required when product_scope is %s", productScope)
	}

	status, err := parsePromoCodeImportStatus(promoCodeImportCell(record, headerMap, "status"))
	if err != nil {
		return nil, false, err
	}

	expiresAtRaw := promoCodeImportCell(record, headerMap, "expires_at")
	var expiresAtInput *string
	if expiresAtRaw != "" {
		expiresAtInput = &expiresAtRaw
	}
	expiresAt, err := parsePromoCodeExpiryInput(expiresAtInput)
	if err != nil {
		return nil, false, fmt.Errorf("invalid expires_at: %w", err)
	}

	if discountType == models.DiscountTypeFixed {
		maxDiscount = 0
	}

	return &models.PromoCode{
		Code:           code,
		Name:           name,
		Description:    strings.TrimSpace(promoCodeImportCell(record, headerMap, "description")),
		DiscountType:   discountType,
		DiscountValue:  discountValue,
		MaxDiscount:    maxDiscount,
		MinOrderAmount: minOrderAmount,
		TotalQuantity:  totalQuantity,
		ProductIDs:     productIDs,
		ProductScope:   productScope,
		Status:         status,
		ExpiresAt:      expiresAt,
	}, false, nil
}

func appendPromoCodeImportError(errors []string, message string) []string {
	if strings.TrimSpace(message) == "" || len(errors) >= promoCodeImportMaxErrors {
		return errors
	}
	return append(errors, message)
}

func buildPromoCodeAdminHookPayload(promoCode *models.PromoCode) map[string]interface{} {
	if promoCode == nil {
		return map[string]interface{}{}
	}

	return map[string]interface{}{
		"promo_code_id":          promoCode.ID,
		"code":                   promoCode.Code,
		"name":                   promoCode.Name,
		"description":            promoCode.Description,
		"discount_type":          promoCode.DiscountType,
		"discount_value_minor":   promoCode.DiscountValue,
		"max_discount_minor":     promoCode.MaxDiscount,
		"min_order_amount_minor": promoCode.MinOrderAmount,
		"total_quantity":         promoCode.TotalQuantity,
		"used_quantity":          promoCode.UsedQuantity,
		"reserved_quantity":      promoCode.ReservedQuantity,
		"product_ids":            promoCode.ProductIDs,
		"product_scope":          promoCode.ProductScope,
		"status":                 promoCode.Status,
		"expires_at":             promoCode.ExpiresAt,
		"created_at":             promoCode.CreatedAt,
		"updated_at":             promoCode.UpdatedAt,
	}
}

// CreatePromoCode 创建优惠码
func (h *PromoCodeHandler) CreatePromoCode(c *gin.Context) {
	var req CreatePromoCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}
	if h.pluginManager != nil {
		originalReq := req
		hookPayload, payloadErr := adminHookStructToPayload(req)
		if payloadErr != nil {
			log.Printf("promo.admin.create.before payload build failed: admin=%d err=%v", adminIDValue, payloadErr)
		} else {
			hookPayload["admin_id"] = adminIDValue
			hookPayload["source"] = "admin_api"
			hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "promo.admin.create.before",
				Payload: hookPayload,
			}, buildAdminHookExecutionContext(c, adminID, map[string]string{
				"hook_resource": "promo_code",
				"hook_source":   "admin_api",
			}))
			if hookErr != nil {
				log.Printf("promo.admin.create.before hook execution failed: admin=%d err=%v", adminIDValue, hookErr)
			} else if hookResult != nil {
				if hookResult.Blocked {
					reason := strings.TrimSpace(hookResult.BlockReason)
					if reason == "" {
						reason = "Promo code creation rejected by plugin"
					}
					response.BadRequest(c, reason)
					return
				}
				if hookResult.Payload != nil {
					if mergeErr := mergeAdminHookStructPatch(&req, hookResult.Payload); mergeErr != nil {
						log.Printf("promo.admin.create.before payload apply failed, fallback to original request: admin=%d err=%v", adminIDValue, mergeErr)
						req = originalReq
					}
				}
			}
		}
	}

	promoCode := &models.PromoCode{
		Code:           req.Code,
		Name:           req.Name,
		Description:    req.Description,
		DiscountType:   req.DiscountType,
		DiscountValue:  req.DiscountValueMinor,
		MaxDiscount:    req.MaxDiscountMinor,
		MinOrderAmount: req.MinOrderAmountMinor,
		TotalQuantity:  req.TotalQuantity,
		ProductIDs:     req.ProductIDs,
		ProductScope:   req.ProductScope,
		Status:         models.PromoCodeStatusActive,
	}

	if req.Status != "" {
		promoCode.Status = models.PromoCodeStatus(req.Status)
	}

	if expiresAt, err := parsePromoCodeExpiryInput(req.ExpiresAt); err != nil {
		response.BadRequest(c, "Invalid expiry date format")
		return
	} else {
		promoCode.ExpiresAt = expiresAt
	}

	if err := h.promoCodeService.Create(promoCode); err != nil {
		if respondAdminBizError(c, err) {
			return
		}
		response.BadRequest(c, err.Error())
		return
	}

	if h.pluginManager != nil {
		afterPayload := buildPromoCodeAdminHookPayload(promoCode)
		afterPayload["admin_id"] = adminIDValue
		afterPayload["source"] = "admin_api"
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, promoCodeID uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "promo.admin.create.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("promo.admin.create.after hook execution failed: admin=%d promo=%d err=%v", adminIDValue, promoCodeID, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "promo_code",
			"hook_source":   "admin_api",
			"promo_code_id": strconv.FormatUint(uint64(promoCode.ID), 10),
		})), afterPayload, promoCode.ID)
	}

	response.Success(c, promoCode)
}

// GetPromoCode 获取优惠码详情
func (h *PromoCodeHandler) GetPromoCode(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}

	promoCode, err := h.promoCodeService.GetByID(uint(id))
	if err != nil {
		response.NotFound(c, "Promo code not found")
		return
	}

	response.Success(c, promoCode)
}

// UpdatePromoCode 更新优惠码
func (h *PromoCodeHandler) UpdatePromoCode(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}
	currentPromoCode, err := h.promoCodeService.GetByID(uint(id))
	if err != nil {
		response.NotFound(c, "Promo code not found")
		return
	}

	var req UpdatePromoCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}
	if h.pluginManager != nil {
		originalReq := req
		hookPayload, payloadErr := adminHookStructToPayload(req)
		if payloadErr != nil {
			log.Printf("promo.admin.update.before payload build failed: admin=%d promo=%d err=%v", adminIDValue, uint(id), payloadErr)
		} else {
			hookPayload["promo_code_id"] = uint(id)
			hookPayload["current"] = buildPromoCodeAdminHookPayload(currentPromoCode)
			hookPayload["admin_id"] = adminIDValue
			hookPayload["source"] = "admin_api"
			hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "promo.admin.update.before",
				Payload: hookPayload,
			}, buildAdminHookExecutionContext(c, adminID, map[string]string{
				"hook_resource": "promo_code",
				"hook_source":   "admin_api",
				"promo_code_id": strconv.FormatUint(id, 10),
			}))
			if hookErr != nil {
				log.Printf("promo.admin.update.before hook execution failed: admin=%d promo=%d err=%v", adminIDValue, uint(id), hookErr)
			} else if hookResult != nil {
				if hookResult.Blocked {
					reason := strings.TrimSpace(hookResult.BlockReason)
					if reason == "" {
						reason = "Promo code update rejected by plugin"
					}
					response.BadRequest(c, reason)
					return
				}
				if hookResult.Payload != nil {
					if mergeErr := mergeAdminHookStructPatch(&req, hookResult.Payload); mergeErr != nil {
						log.Printf("promo.admin.update.before payload apply failed, fallback to original request: admin=%d promo=%d err=%v", adminIDValue, uint(id), mergeErr)
						req = originalReq
					}
				}
			}
		}
	}

	updates := &models.PromoCode{
		Name:           req.Name,
		Description:    req.Description,
		DiscountType:   req.DiscountType,
		DiscountValue:  req.DiscountValueMinor,
		MaxDiscount:    req.MaxDiscountMinor,
		MinOrderAmount: req.MinOrderAmountMinor,
		TotalQuantity:  req.TotalQuantity,
		ProductIDs:     req.ProductIDs,
		ProductScope:   req.ProductScope,
		Status:         models.PromoCodeStatusActive,
	}

	if req.Status != "" {
		updates.Status = models.PromoCodeStatus(req.Status)
	}

	if expiresAt, err := parsePromoCodeExpiryInput(req.ExpiresAt); err != nil {
		response.BadRequest(c, "Invalid expiry date format")
		return
	} else {
		updates.ExpiresAt = expiresAt
	}

	if err := h.promoCodeService.Update(uint(id), updates); err != nil {
		if respondAdminBizError(c, err) {
			return
		}
		response.BadRequest(c, err.Error())
		return
	}

	if h.pluginManager != nil {
		updatedPromoCode, reloadErr := h.promoCodeService.GetByID(uint(id))
		if reloadErr != nil {
			log.Printf("promo.admin.update.after reload failed: admin=%d promo=%d err=%v", adminIDValue, uint(id), reloadErr)
		} else {
			afterPayload := buildPromoCodeAdminHookPayload(updatedPromoCode)
			afterPayload["admin_id"] = adminIDValue
			afterPayload["source"] = "admin_api"
			go func(execCtx *service.ExecutionContext, payload map[string]interface{}, promoCodeID uint) {
				_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
					Hook:    "promo.admin.update.after",
					Payload: payload,
				}, execCtx)
				if hookErr != nil {
					log.Printf("promo.admin.update.after hook execution failed: admin=%d promo=%d err=%v", adminIDValue, promoCodeID, hookErr)
				}
			}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
				"hook_resource": "promo_code",
				"hook_source":   "admin_api",
				"promo_code_id": strconv.FormatUint(id, 10),
			})), afterPayload, updatedPromoCode.ID)
		}
	}

	response.Success(c, gin.H{"message": "updated"})
}

// DeletePromoCode 删除优惠码
func (h *PromoCodeHandler) DeletePromoCode(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}
	promoCode, err := h.promoCodeService.GetByID(uint(id))
	if err != nil {
		response.NotFound(c, "Promo code not found")
		return
	}
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}
	if h.pluginManager != nil {
		hookPayload := buildPromoCodeAdminHookPayload(promoCode)
		hookPayload["admin_id"] = adminIDValue
		hookPayload["source"] = "admin_api"
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "promo.admin.delete.before",
			Payload: hookPayload,
		}, buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "promo_code",
			"hook_source":   "admin_api",
			"promo_code_id": strconv.FormatUint(id, 10),
		}))
		if hookErr != nil {
			log.Printf("promo.admin.delete.before hook execution failed: admin=%d promo=%d err=%v", adminIDValue, uint(id), hookErr)
		} else if hookResult != nil && hookResult.Blocked {
			reason := strings.TrimSpace(hookResult.BlockReason)
			if reason == "" {
				reason = "Promo code deletion rejected by plugin"
			}
			response.BadRequest(c, reason)
			return
		}
	}

	if err := h.promoCodeService.Delete(uint(id)); err != nil {
		if respondAdminBizError(c, err) {
			return
		}
		response.BadRequest(c, err.Error())
		return
	}

	if h.pluginManager != nil {
		afterPayload := buildPromoCodeAdminHookPayload(promoCode)
		afterPayload["admin_id"] = adminIDValue
		afterPayload["source"] = "admin_api"
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, promoCodeID uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "promo.admin.delete.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("promo.admin.delete.after hook execution failed: admin=%d promo=%d err=%v", adminIDValue, promoCodeID, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "promo_code",
			"hook_source":   "admin_api",
			"promo_code_id": strconv.FormatUint(id, 10),
		})), afterPayload, promoCode.ID)
	}

	response.Success(c, gin.H{"message": "deleted"})
}

// ListPromoCodes 列表
func (h *PromoCodeHandler) ListPromoCodes(c *gin.Context) {
	page, limit := response.GetPagination(c)
	status := c.Query("status")
	search := c.Query("search")

	promoCodes, total, err := h.promoCodeService.List(page, limit, status, search)
	if err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	response.Paginated(c, promoCodes, page, limit, total)
}

// ExportPromoCodes 导出优惠码
func (h *PromoCodeHandler) ExportPromoCodes(c *gin.Context) {
	status := strings.TrimSpace(c.Query("status"))
	search := strings.TrimSpace(c.Query("search"))

	promoCodes, total, err := h.promoCodeService.List(1, adminCSVExportMaxRows+1, status, search)
	if err != nil {
		response.InternalError(c, "Query failed")
		return
	}
	if total > adminCSVExportMaxRows {
		response.BadRequest(c, fmt.Sprintf("Too many records to export (max %d). Please narrow the filters.", adminCSVExportMaxRows))
		return
	}

	rows := make([][]string, 0, len(promoCodes))
	for _, item := range promoCodes {
		productIDs := make([]string, 0, len(item.ProductIDs))
		for _, id := range item.ProductIDs {
			productIDs = append(productIDs, strconv.FormatUint(uint64(id), 10))
		}

		rows = append(rows, []string{
			strconv.FormatUint(uint64(item.ID), 10),
			item.Code,
			item.Name,
			item.Description,
			string(item.DiscountType),
			strconv.FormatInt(item.DiscountValue, 10),
			strconv.FormatInt(item.MaxDiscount, 10),
			strconv.FormatInt(item.MinOrderAmount, 10),
			strconv.Itoa(item.TotalQuantity),
			strconv.Itoa(item.UsedQuantity),
			strconv.Itoa(item.ReservedQuantity),
			item.ProductScope,
			strings.Join(productIDs, ","),
			string(item.Status),
			csvTimePtrValue(item.ExpiresAt),
			csvTimeValue(item.CreatedAt),
			csvTimeValue(item.UpdatedAt),
		})
	}

	if h.db != nil {
		logger.LogOperation(h.db, c, "export", "promo_code", nil, map[string]interface{}{
			"count":   len(rows),
			"status":  status,
			"search":  search,
			"format":  "xlsx",
			"subject": "promo_codes",
		})
	}

	writeXLSXAttachment(c, buildAdminXLSXFileName("promo_codes"), "Promo Codes", []string{
		"ID",
		"Code",
		"Name",
		"Description",
		"Discount Type",
		"Discount Value Minor",
		"Max Discount Minor",
		"Min Order Amount Minor",
		"Total Quantity",
		"Used Quantity",
		"Reserved Quantity",
		"Product Scope",
		"Product IDs",
		"Status",
		"Expires At",
		"Created At",
		"Updated At",
	}, rows)
}

// ImportPromoCodes 导入优惠码表格文件
func (h *PromoCodeHandler) ImportPromoCodes(c *gin.Context) {
	conflictMode, err := parsePromoCodeImportConflictMode(c.DefaultPostForm("conflict_mode", c.Query("conflict_mode")))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "Please select an import file to upload")
		return
	}

	fileFormat, tableRows, err := readAdminTabularRows(file)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unsupported format") {
			response.BadRequest(c, "Only .xlsx Excel format is supported")
			return
		}
		response.BadRequest(c, "Failed to parse import file")
		return
	}
	if len(tableRows) == 0 {
		response.BadRequest(c, "Import file is empty")
		return
	}

	header := tableRows[0]
	headerMap, err := buildPromoCodeImportHeaderMap(header)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	result := promoCodeImportResult{
		ConflictMode: conflictMode,
		Errors:       make([]string, 0),
	}

	for idx, record := range tableRows[1:] {
		rowIndex := idx + 2
		model, blank, buildErr := buildPromoCodeImportModel(record, headerMap)
		if blank {
			continue
		}

		result.TotalRows++
		if result.TotalRows > adminCSVExportMaxRows {
			response.BadRequest(c, fmt.Sprintf("Too many rows to import (max %d). Please split the file.", adminCSVExportMaxRows))
			return
		}

		if buildErr != nil {
			result.ErrorCount++
			result.Errors = appendPromoCodeImportError(result.Errors, fmt.Sprintf("Row %d: %v", rowIndex, buildErr))
			continue
		}

		existing, found, lookupErr := h.promoCodeService.LookupByCode(model.Code)
		if lookupErr != nil {
			result.ErrorCount++
			result.Errors = appendPromoCodeImportError(result.Errors, fmt.Sprintf("Row %d: lookup failed: %v", rowIndex, lookupErr))
			continue
		}

		switch {
		case found && conflictMode == "skip":
			result.SkippedCount++
		case found:
			updates := &models.PromoCode{
				Name:           model.Name,
				Description:    model.Description,
				DiscountType:   model.DiscountType,
				DiscountValue:  model.DiscountValue,
				MaxDiscount:    model.MaxDiscount,
				MinOrderAmount: model.MinOrderAmount,
				TotalQuantity:  model.TotalQuantity,
				ProductIDs:     model.ProductIDs,
				ProductScope:   model.ProductScope,
				Status:         model.Status,
				ExpiresAt:      model.ExpiresAt,
			}
			if updateErr := h.promoCodeService.Update(existing.ID, updates); updateErr != nil {
				result.ErrorCount++
				result.Errors = appendPromoCodeImportError(result.Errors, fmt.Sprintf("Row %d: %v", rowIndex, updateErr))
				continue
			}
			result.UpdatedCount++
		case !found && conflictMode == "update":
			result.SkippedCount++
		default:
			if createErr := h.promoCodeService.Create(model); createErr != nil {
				result.ErrorCount++
				result.Errors = appendPromoCodeImportError(result.Errors, fmt.Sprintf("Row %d: %v", rowIndex, createErr))
				continue
			}
			result.CreatedCount++
		}
	}

	if result.TotalRows == 0 {
		response.BadRequest(c, "No data rows found in import file")
		return
	}

	result.Message = fmt.Sprintf(
		"Promo code import completed: created %d, updated %d, skipped %d, errors %d",
		result.CreatedCount,
		result.UpdatedCount,
		result.SkippedCount,
		result.ErrorCount,
	)

	if h.db != nil {
		logger.LogOperation(h.db, c, "import", "promo_code", nil, map[string]interface{}{
			"filename":      strings.TrimSpace(file.Filename),
			"conflict_mode": conflictMode,
			"total_rows":    result.TotalRows,
			"created_count": result.CreatedCount,
			"updated_count": result.UpdatedCount,
			"skipped_count": result.SkippedCount,
			"error_count":   result.ErrorCount,
			"format":        fileFormat,
		})
	}

	response.Success(c, result)
}
