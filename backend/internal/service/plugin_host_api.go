package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/middleware"
	"auralogic/internal/models"
	"auralogic/internal/pkg/bizerr"
	"auralogic/internal/pkg/orderbiz"
	"auralogic/internal/pkg/pluginhost"
	"auralogic/internal/pkg/ticketbiz"
	"auralogic/internal/pkg/validator"
	"auralogic/internal/pluginipc"
	"auralogic/internal/repository"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

type PluginHostAccessClaims struct {
	PluginID           uint     `json:"plugin_id"`
	GrantedPermissions []string `json:"granted_permissions,omitempty"`
	OperatorUserID     uint     `json:"operator_user_id,omitempty"`
	ExecutionID        string   `json:"execution_id,omitempty"`
	WorkspaceCommand   string   `json:"workspace_command,omitempty"`
	ScopeAuthenticated bool     `json:"scope_authenticated,omitempty"`
	ScopeSuperAdmin    bool     `json:"scope_super_admin,omitempty"`
	ScopePermissions   []string `json:"scope_permissions,omitempty"`
	jwt.RegisteredClaims
}

type PluginHostActionError struct {
	Status  int
	Message string
}

func (e *PluginHostActionError) Error() string {
	if e == nil {
		return "plugin host action failed"
	}
	return strings.TrimSpace(e.Message)
}

type pluginHostActionDefinition struct {
	PluginPermissions   []string
	OperatorPermissions []string
}

var pluginHostActionDefinitions = buildPluginHostActionDefinitions()

func buildPluginHostActionDefinitions() map[string]pluginHostActionDefinition {
	definitions := map[string]pluginHostActionDefinition{
		"host.order.get": {
			PluginPermissions:   []string{PluginPermissionHostOrderRead},
			OperatorPermissions: []string{"order.view"},
		},
		"host.order.list": {
			PluginPermissions:   []string{PluginPermissionHostOrderList},
			OperatorPermissions: []string{"order.view"},
		},
		"host.order.assign_tracking": {
			PluginPermissions:   []string{PluginPermissionHostOrderAssignTracking},
			OperatorPermissions: []string{"order.assign_tracking"},
		},
		"host.order.request_resubmit": {
			PluginPermissions:   []string{PluginPermissionHostOrderRequestResubmit},
			OperatorPermissions: []string{"order.edit"},
		},
		"host.order.mark_paid": {
			PluginPermissions:   []string{PluginPermissionHostOrderMarkPaid},
			OperatorPermissions: []string{"order.status_update"},
		},
		"host.order.update_price": {
			PluginPermissions:   []string{PluginPermissionHostOrderUpdatePrice},
			OperatorPermissions: []string{"order.edit"},
		},
		"host.user.get": {
			PluginPermissions:   []string{PluginPermissionHostUserRead},
			OperatorPermissions: []string{"user.view"},
		},
		"host.user.list": {
			PluginPermissions:   []string{PluginPermissionHostUserList},
			OperatorPermissions: []string{"user.view"},
		},
		"host.product.get": {
			PluginPermissions:   []string{PluginPermissionHostProductRead},
			OperatorPermissions: []string{"product.view"},
		},
		"host.product.list": {
			PluginPermissions:   []string{PluginPermissionHostProductList},
			OperatorPermissions: []string{"product.view"},
		},
		"host.inventory.get": {
			PluginPermissions:   []string{PluginPermissionHostInventoryRead},
			OperatorPermissions: []string{"product.view"},
		},
		"host.inventory.list": {
			PluginPermissions:   []string{PluginPermissionHostInventoryList},
			OperatorPermissions: []string{"product.view"},
		},
		"host.inventory_binding.get": {
			PluginPermissions:   []string{PluginPermissionHostInventoryBindingRead},
			OperatorPermissions: []string{"product.view"},
		},
		"host.inventory_binding.list": {
			PluginPermissions:   []string{PluginPermissionHostInventoryBindingList},
			OperatorPermissions: []string{"product.view"},
		},
		"host.promo.get": {
			PluginPermissions:   []string{PluginPermissionHostPromoRead},
			OperatorPermissions: []string{"product.view"},
		},
		"host.promo.list": {
			PluginPermissions:   []string{PluginPermissionHostPromoList},
			OperatorPermissions: []string{"product.view"},
		},
		"host.ticket.get": {
			PluginPermissions:   []string{PluginPermissionHostTicketRead},
			OperatorPermissions: []string{"ticket.view"},
		},
		"host.ticket.list": {
			PluginPermissions:   []string{PluginPermissionHostTicketList},
			OperatorPermissions: []string{"ticket.view"},
		},
		"host.ticket.reply": {
			PluginPermissions:   []string{PluginPermissionHostTicketReply},
			OperatorPermissions: []string{"ticket.reply"},
		},
		"host.ticket.update": {
			PluginPermissions:   []string{PluginPermissionHostTicketUpdate},
			OperatorPermissions: []string{"ticket.status_update"},
		},
		"host.serial.get": {
			PluginPermissions:   []string{PluginPermissionHostSerialRead},
			OperatorPermissions: []string{"serial.view"},
		},
		"host.serial.list": {
			PluginPermissions:   []string{PluginPermissionHostSerialList},
			OperatorPermissions: []string{"serial.view"},
		},
		"host.announcement.get": {
			PluginPermissions:   []string{PluginPermissionHostAnnouncementRead},
			OperatorPermissions: []string{"announcement.view"},
		},
		"host.announcement.list": {
			PluginPermissions:   []string{PluginPermissionHostAnnouncementList},
			OperatorPermissions: []string{"announcement.view"},
		},
		"host.knowledge.get": {
			PluginPermissions:   []string{PluginPermissionHostKnowledgeRead},
			OperatorPermissions: []string{"knowledge.view"},
		},
		"host.knowledge.list": {
			PluginPermissions:   []string{PluginPermissionHostKnowledgeList},
			OperatorPermissions: []string{"knowledge.view"},
		},
		"host.knowledge.categories": {
			PluginPermissions:   []string{PluginPermissionHostKnowledgeCategories},
			OperatorPermissions: []string{"knowledge.view"},
		},
		"host.payment_method.get": {
			PluginPermissions:   []string{PluginPermissionHostPaymentMethodRead},
			OperatorPermissions: []string{"payment_method.view"},
		},
		"host.payment_method.list": {
			PluginPermissions:   []string{PluginPermissionHostPaymentMethodList},
			OperatorPermissions: []string{"payment_method.view"},
		},
		"host.virtual_inventory.get": {
			PluginPermissions:   []string{PluginPermissionHostVirtualInventoryRead},
			OperatorPermissions: []string{"product.view"},
		},
		"host.virtual_inventory.list": {
			PluginPermissions:   []string{PluginPermissionHostVirtualInventoryList},
			OperatorPermissions: []string{"product.view"},
		},
		"host.virtual_inventory_binding.get": {
			PluginPermissions:   []string{PluginPermissionHostVirtualInventoryBindingRead},
			OperatorPermissions: []string{"product.view"},
		},
		"host.virtual_inventory_binding.list": {
			PluginPermissions:   []string{PluginPermissionHostVirtualInventoryBindingList},
			OperatorPermissions: []string{"product.view"},
		},
		"host.workspace.append": {
			PluginPermissions:   nil,
			OperatorPermissions: nil,
		},
		"host.workspace.read_input": {
			PluginPermissions:   nil,
			OperatorPermissions: nil,
		},
	}
	for _, sharedDef := range pluginhost.ListSharedActionDefinitions() {
		definitions[sharedDef.Action] = pluginHostActionDefinition{
			PluginPermissions:   append([]string(nil), sharedDef.PluginPermissions...),
			OperatorPermissions: append([]string(nil), sharedDef.OperatorPermissions...),
		}
	}
	return definitions
}

func BuildPluginHostAccessClaims(plugin *models.Plugin, execCtx *ExecutionContext, ttl time.Duration) PluginHostAccessClaims {
	scope := resolveHookRequestAccessScope(execCtx)
	claims := PluginHostAccessClaims{
		ScopeAuthenticated: scope.Authenticated,
		ScopeSuperAdmin:    scope.SuperAdmin,
		ScopePermissions:   sortedHookRequestAccessScopePermissionKeys(scope.Permissions),
	}
	if plugin != nil {
		claims.PluginID = plugin.ID
		claims.GrantedPermissions = ResolveEffectivePluginCapabilityPolicy(plugin).GrantedPermissions
	}
	if execCtx != nil && execCtx.Metadata != nil {
		claims.ExecutionID = strings.TrimSpace(execCtx.Metadata[PluginExecutionMetadataID])
		claims.WorkspaceCommand = strings.TrimSpace(execCtx.Metadata["workspace_command"])
	}
	if execCtx != nil && execCtx.OperatorUserID != nil {
		claims.OperatorUserID = *execCtx.OperatorUserID
	} else if execCtx != nil && execCtx.UserID != nil {
		claims.OperatorUserID = *execCtx.UserID
	}
	if ttl <= 0 {
		ttl = 2 * time.Minute
	}
	now := time.Now()
	claims.RegisteredClaims = jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now.Add(-15 * time.Second)),
		ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
	}
	return claims
}

func sortedHookRequestAccessScopePermissionKeys(values map[string]struct{}) []string {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		normalized := strings.TrimSpace(key)
		if normalized == "" {
			continue
		}
		keys = append(keys, normalized)
	}
	sort.Strings(keys)
	return keys
}

func GeneratePluginHostAccessToken(secret string, claims PluginHostAccessClaims) (string, error) {
	if strings.TrimSpace(secret) == "" {
		return "", errors.New("plugin host jwt secret is required")
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func ParsePluginHostAccessToken(secret string, tokenString string) (*PluginHostAccessClaims, error) {
	trimmedSecret := strings.TrimSpace(secret)
	if trimmedSecret == "" {
		return nil, errors.New("plugin host jwt secret is required")
	}
	trimmedToken := strings.TrimSpace(tokenString)
	if trimmedToken == "" {
		return nil, errors.New("plugin host token is required")
	}

	token, err := jwt.ParseWithClaims(trimmedToken, &PluginHostAccessClaims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(trimmedSecret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*PluginHostAccessClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid plugin host token")
	}
	return claims, nil
}

func ExecutePluginHostAction(db *gorm.DB, claims *PluginHostAccessClaims, action string, params map[string]interface{}) (map[string]interface{}, error) {
	return ExecutePluginHostActionWithRuntime(NewPluginHostRuntime(db, config.GetConfig(), nil), claims, action, params)
}

func ExecutePluginHostActionWithRuntime(runtime *PluginHostRuntime, claims *PluginHostAccessClaims, action string, params map[string]interface{}) (map[string]interface{}, error) {
	db := runtime.database()
	if db == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "plugin host database is unavailable"}
	}
	if claims == nil {
		return nil, &PluginHostActionError{Status: http.StatusUnauthorized, Message: "plugin host token is invalid"}
	}

	normalizedAction := strings.ToLower(strings.TrimSpace(action))
	if normalizedAction == "" {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "plugin host action is required"}
	}
	def, exists := pluginHostActionDefinitions[normalizedAction]
	if !exists {
		return nil, &PluginHostActionError{Status: http.StatusNotFound, Message: fmt.Sprintf("unsupported plugin host action %q", normalizedAction)}
	}
	if !pluginHostClaimsHasPluginPermissions(claims, def.PluginPermissions...) {
		return nil, &PluginHostActionError{Status: http.StatusForbidden, Message: "plugin host permission denied"}
	}
	if !pluginHostClaimsHasOperatorPermissions(claims, def.OperatorPermissions...) {
		return nil, &PluginHostActionError{Status: http.StatusForbidden, Message: "operator permission denied"}
	}
	if params == nil {
		params = map[string]interface{}{}
	}

	orderRepo := repository.NewOrderRepository(db)
	userRepo := repository.NewUserRepository(db)
	productRepo := repository.NewProductRepository(db)
	inventoryRepo := repository.NewInventoryRepository(db)
	promoRepo := repository.NewPromoCodeRepository(db)
	serialRepo := repository.NewSerialRepository(db)
	switch normalizedAction {
	case "host.workspace.append":
		return executePluginHostWorkspaceAppend(runtime, claims, params)
	case "host.workspace.read_input":
		return executePluginHostWorkspaceReadInput(runtime, claims, params)
	case "host.order.get":
		return executePluginHostOrderGet(db, orderRepo, claims, params)
	case "host.order.list":
		return executePluginHostOrderList(orderRepo, claims, params)
	case "host.order.assign_tracking":
		return executePluginHostOrderAssignTracking(orderRepo, params)
	case "host.order.request_resubmit":
		return executePluginHostOrderRequestResubmit(orderRepo, params)
	case "host.order.mark_paid":
		return executePluginHostOrderMarkPaid(db, orderRepo, params)
	case "host.order.update_price":
		return executePluginHostOrderUpdatePrice(orderRepo, params)
	case "host.user.get":
		return executePluginHostUserGet(db, userRepo, claims, params)
	case "host.user.list":
		return executePluginHostUserList(db, userRepo, claims, params)
	case "host.product.get":
		return executePluginHostProductGet(productRepo, params)
	case "host.product.list":
		return executePluginHostProductList(productRepo, params)
	case "host.inventory.get":
		return executePluginHostInventoryGet(inventoryRepo, params)
	case "host.inventory.list":
		return executePluginHostInventoryList(inventoryRepo, params)
	case "host.inventory_binding.get":
		return executePluginHostInventoryBindingGet(db, params)
	case "host.inventory_binding.list":
		return executePluginHostInventoryBindingList(db, params)
	case "host.promo.get":
		return executePluginHostPromoGet(promoRepo, params)
	case "host.promo.list":
		return executePluginHostPromoList(promoRepo, params)
	case "host.ticket.get":
		return executePluginHostTicketGet(db, params)
	case "host.ticket.list":
		return executePluginHostTicketList(db, claims, params)
	case "host.ticket.reply":
		return executePluginHostTicketReply(db, claims, params)
	case "host.ticket.update":
		return executePluginHostTicketUpdate(db, params)
	case "host.serial.get":
		return executePluginHostSerialGet(db, serialRepo, params)
	case "host.serial.list":
		return executePluginHostSerialList(serialRepo, params)
	case "host.announcement.get":
		return executePluginHostAnnouncementGet(db, params)
	case "host.announcement.list":
		return executePluginHostAnnouncementList(db, params)
	case "host.knowledge.get":
		return executePluginHostKnowledgeGet(db, params)
	case "host.knowledge.list":
		return executePluginHostKnowledgeList(db, params)
	case "host.knowledge.categories":
		return executePluginHostKnowledgeCategories(db)
	case "host.payment_method.get":
		return executePluginHostPaymentMethodGet(db, params)
	case "host.payment_method.list":
		return executePluginHostPaymentMethodList(db, params)
	case "host.virtual_inventory.get":
		return executePluginHostVirtualInventoryGet(db, params)
	case "host.virtual_inventory.list":
		return executePluginHostVirtualInventoryList(db, params)
	case "host.virtual_inventory_binding.get":
		return executePluginHostVirtualInventoryBindingGet(db, params)
	case "host.virtual_inventory_binding.list":
		return executePluginHostVirtualInventoryBindingList(db, params)
	case "host.market.source.list":
		return executePluginHostMarketSourceList(db, claims)
	case "host.market.source.get":
		return executePluginHostMarketSourceGet(db, claims, params)
	case "host.market.catalog.list":
		return executePluginHostMarketCatalogList(db, claims, params)
	case "host.market.artifact.get":
		return executePluginHostMarketArtifactGet(db, claims, params)
	case "host.market.release.get":
		return executePluginHostMarketReleaseGet(db, claims, params)
	case "host.market.install.preview":
		return executePluginHostMarketInstallPreview(db, claims, params)
	case "host.market.install.execute":
		return executePluginHostMarketInstallExecute(runtime, claims, params)
	case "host.market.install.task.get":
		return executePluginHostMarketInstallTaskGet(db, claims, params)
	case "host.market.install.task.list":
		return executePluginHostMarketInstallTaskList(db, claims, params)
	case "host.market.install.history.list":
		return executePluginHostMarketInstallHistoryList(db, claims, params)
	case "host.market.install.rollback":
		return executePluginHostMarketInstallRollback(runtime, claims, params)
	case "host.email_template.list":
		return executePluginHostEmailTemplateList(params)
	case "host.email_template.get":
		return executePluginHostEmailTemplateGet(params)
	case "host.email_template.save":
		return executePluginHostEmailTemplateSave(params)
	case "host.landing_page.get":
		return executePluginHostLandingPageGet(db, params)
	case "host.landing_page.save":
		return executePluginHostLandingPageSave(db, params)
	case "host.landing_page.reset":
		return executePluginHostLandingPageReset(db, params)
	case "host.invoice_template.get":
		return executePluginHostInvoiceTemplateGet(runtime, params)
	case "host.invoice_template.save":
		return executePluginHostInvoiceTemplateSave(runtime, params)
	case "host.invoice_template.reset":
		return executePluginHostInvoiceTemplateReset(runtime, params)
	case "host.auth_branding.get":
		return executePluginHostAuthBrandingGet(runtime, params)
	case "host.auth_branding.save":
		return executePluginHostAuthBrandingSave(runtime, params)
	case "host.auth_branding.reset":
		return executePluginHostAuthBrandingReset(runtime, params)
	case "host.page_rule_pack.get":
		return executePluginHostPageRulePackGet(runtime, params)
	case "host.page_rule_pack.save":
		return executePluginHostPageRulePackSave(runtime, params)
	case "host.page_rule_pack.reset":
		return executePluginHostPageRulePackReset(runtime, params)
	case "host.plugin_page_rule.list":
		return executePluginHostPluginPageRuleList(runtime, claims, params)
	case "host.plugin_page_rule.get":
		return executePluginHostPluginPageRuleGet(runtime, claims, params)
	case "host.plugin_page_rule.upsert":
		return executePluginHostPluginPageRuleUpsert(runtime, claims, params)
	case "host.plugin_page_rule.delete":
		return executePluginHostPluginPageRuleDelete(runtime, claims, params)
	case "host.plugin_page_rule.reset":
		return executePluginHostPluginPageRuleReset(runtime, claims, params)
	default:
		return nil, &PluginHostActionError{Status: http.StatusNotFound, Message: fmt.Sprintf("unsupported plugin host action %q", normalizedAction)}
	}
}

func executePluginHostOrderGet(
	db *gorm.DB,
	orderRepo *repository.OrderRepository,
	claims *PluginHostAccessClaims,
	params map[string]interface{},
) (map[string]interface{}, error) {
	if orderRepo == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "order repository is unavailable"}
	}

	orderID, hasID, err := parsePluginHostOptionalUint(params, "id", "order_id")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	orderNo := parsePluginHostOptionalString(params, "order_no")

	var order *models.Order
	switch {
	case hasID:
		order, err = orderRepo.FindByID(orderID)
	case orderNo != "":
		order, err = orderRepo.FindByOrderNo(orderNo)
	default:
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "id/order_id or order_no is required"}
	}
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &PluginHostActionError{Status: http.StatusNotFound, Message: "order not found"}
		}
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query order failed"}
	}

	hasPrivacyPermission := pluginHostClaimsCanReadOrderPrivacy(claims)
	cloned := clonePluginHostOrder(order)
	if cloned.PrivacyProtected && !hasPrivacyPermission {
		cloned.MaskSensitiveInfo()
	}

	return buildPluginHostOrderDetailResponse(&cloned, hasPrivacyPermission), nil
}

func executePluginHostOrderList(
	orderRepo *repository.OrderRepository,
	claims *PluginHostAccessClaims,
	params map[string]interface{},
) (map[string]interface{}, error) {
	if orderRepo == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "order repository is unavailable"}
	}

	page := parsePluginHostPositiveInt(params, 1, 1, 10000, "page")
	pageSize := parsePluginHostPositiveInt(params, 20, 1, 100, "page_size", "pageSize", "limit")
	status := parsePluginHostOptionalString(params, "status")
	search := parsePluginHostOptionalString(params, "search", "q")
	country := parsePluginHostOptionalString(params, "country")
	productSearch := parsePluginHostOptionalString(params, "product_search", "productSearch")
	promoCode := strings.ToUpper(parsePluginHostOptionalString(params, "promo_code", "promoCode"))

	var promoCodeID *uint
	if parsed, ok, err := parsePluginHostOptionalUint(params, "promo_code_id", "promoCodeId"); err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	} else if ok {
		promoCodeID = &parsed
	}

	var userID *uint
	if parsed, ok, err := parsePluginHostOptionalUint(params, "user_id", "userId"); err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	} else if ok {
		userID = &parsed
	}

	orders, total, err := orderRepo.List(page, pageSize, status, search, country, productSearch, promoCodeID, promoCode, userID)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query orders failed"}
	}

	hasPrivacyPermission := pluginHostClaimsCanReadOrderPrivacy(claims)
	items := make([]map[string]interface{}, 0, len(orders))
	for idx := range orders {
		cloned := clonePluginHostOrder(&orders[idx])
		if cloned.PrivacyProtected && !hasPrivacyPermission {
			cloned.MaskSensitiveInfo()
		}
		items = append(items, buildPluginHostOrderSummaryResponse(&cloned, hasPrivacyPermission))
	}

	return map[string]interface{}{
		"items":      items,
		"page":       page,
		"page_size":  pageSize,
		"total":      total,
		"has_more":   int64(page*pageSize) < total,
		"query":      search,
		"status":     status,
		"user_id":    derefPluginHostUint(userID),
		"promo_code": promoCode,
	}, nil
}

func executePluginHostOrderAssignTracking(
	orderRepo *repository.OrderRepository,
	params map[string]interface{},
) (map[string]interface{}, error) {
	if orderRepo == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "order repository is unavailable"}
	}

	orderID, hasID, err := parsePluginHostOptionalUint(params, "id", "order_id")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	orderNo := parsePluginHostOptionalString(params, "order_no")

	trackingNo := strings.TrimSpace(parsePluginHostOptionalString(params, "tracking_no", "trackingNo"))
	if trackingNo == "" {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "tracking_no/trackingNo is required"}
	}
	if len(trackingNo) > 100 {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "tracking number must be 1-100 characters"}
	}

	switch {
	case hasID:
	case orderNo != "":
		order, lookupErr := orderRepo.FindByOrderNo(orderNo)
		if lookupErr != nil {
			if errors.Is(lookupErr, gorm.ErrRecordNotFound) {
				return nil, &PluginHostActionError{Status: http.StatusNotFound, Message: "order not found"}
			}
			return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query order failed"}
		}
		orderID = order.ID
	default:
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "id/order_id or order_no is required"}
	}

	orderService := NewOrderService(orderRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if err := orderService.AssignTracking(orderID, trackingNo); err != nil {
		var typedBizErr *bizerr.Error
		if errors.As(err, &typedBizErr) {
			return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: typedBizErr.Message}
		}
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "assign tracking failed"}
	}

	order, err := orderRepo.FindByID(orderID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &PluginHostActionError{Status: http.StatusNotFound, Message: "order not found"}
		}
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query order failed"}
	}

	return map[string]interface{}{
		"id":          order.ID,
		"order_no":    order.OrderNo,
		"tracking_no": order.TrackingNo,
		"status":      string(order.Status),
		"shipped_at":  order.ShippedAt,
	}, nil
}

func pluginHostOrderServiceConfig() *config.Config {
	cfg := config.GetConfig()
	if cfg != nil {
		return cfg
	}
	return &config.Config{
		Form: config.FormConfig{
			ExpireHours: 24,
		},
	}
}

func pluginHostEmailService(db *gorm.DB) *EmailService {
	cfg := config.GetConfig()
	if db == nil || cfg == nil {
		return nil
	}
	return NewEmailService(db, &cfg.SMTP, cfg.App.URL)
}

func executePluginHostOrderRequestResubmit(
	orderRepo *repository.OrderRepository,
	params map[string]interface{},
) (map[string]interface{}, error) {
	if orderRepo == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "order repository is unavailable"}
	}

	orderID, hasID, err := parsePluginHostOptionalUint(params, "id", "order_id")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	orderNo := parsePluginHostOptionalString(params, "order_no")

	reason := validator.SanitizeText(parsePluginHostOptionalString(params, "reason"))
	if !validator.ValidateLength(reason, 1, 500) {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: orderbiz.ResubmitReasonLengthInvalid(1, 500).Message}
	}

	var order *models.Order
	switch {
	case hasID:
		order, err = orderRepo.FindByID(orderID)
	case orderNo != "":
		order, err = orderRepo.FindByOrderNo(orderNo)
	default:
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "id/order_id or order_no is required"}
	}
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &PluginHostActionError{Status: http.StatusNotFound, Message: "order not found"}
		}
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query order failed"}
	}

	if order.Status != models.OrderStatusPending {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: orderbiz.ResubmitStatusInvalid(order.Status).Message}
	}

	orderService := NewOrderService(orderRepo, nil, nil, nil, nil, nil, nil, nil, pluginHostOrderServiceConfig(), nil)
	if _, err := orderService.RequestResubmit(order.ID, reason); err != nil {
		var typedBizErr *bizerr.Error
		if errors.As(err, &typedBizErr) {
			return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: typedBizErr.Message}
		}
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "request resubmit failed"}
	}

	order, err = orderRepo.FindByID(order.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &PluginHostActionError{Status: http.StatusNotFound, Message: "order not found"}
		}
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query order failed"}
	}

	return map[string]interface{}{
		"id":              order.ID,
		"order_no":        order.OrderNo,
		"status":          string(order.Status),
		"form_expires_at": order.FormExpiresAt,
	}, nil
}

func executePluginHostOrderMarkPaid(
	db *gorm.DB,
	orderRepo *repository.OrderRepository,
	params map[string]interface{},
) (map[string]interface{}, error) {
	if db == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "plugin host database is unavailable"}
	}
	if orderRepo == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "order repository is unavailable"}
	}

	orderID, hasID, err := parsePluginHostOptionalUint(params, "id", "order_id")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	orderNo := parsePluginHostOptionalString(params, "order_no")
	adminRemark := validator.SanitizeText(parsePluginHostOptionalString(params, "admin_remark", "adminRemark"))
	if !validator.ValidateLength(adminRemark, 0, 1000) {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: orderbiz.AdminRemarkTooLong(1000).Message}
	}
	skipAutoDelivery, err := parsePluginHostOptionalBool(params, "skip_auto_delivery", "skipAutoDelivery")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}

	switch {
	case hasID:
	case orderNo != "":
		order, lookupErr := orderRepo.FindByOrderNo(orderNo)
		if lookupErr != nil {
			if errors.Is(lookupErr, gorm.ErrRecordNotFound) {
				return nil, &PluginHostActionError{Status: http.StatusNotFound, Message: "order not found"}
			}
			return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query order failed"}
		}
		orderID = order.ID
	default:
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "id/order_id or order_no is required"}
	}

	options := MarkAsPaidOptions{
		AdminRemark: adminRemark,
	}
	if skipAutoDelivery != nil {
		options.SkipAutoDelivery = *skipAutoDelivery
	}
	userRepo := repository.NewUserRepository(db)

	orderService := NewOrderService(
		orderRepo,
		userRepo,
		nil,
		nil,
		nil,
		nil,
		NewVirtualInventoryService(db),
		nil,
		pluginHostOrderServiceConfig(),
		pluginHostEmailService(db),
	)
	if err := orderService.MarkAsPaidWithOptions(orderID, options); err != nil {
		var typedBizErr *bizerr.Error
		if errors.As(err, &typedBizErr) {
			return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: typedBizErr.Message}
		}
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "mark paid failed"}
	}

	order, err := orderRepo.FindByID(orderID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &PluginHostActionError{Status: http.StatusNotFound, Message: "order not found"}
		}
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query order failed"}
	}

	return map[string]interface{}{
		"id":                 order.ID,
		"order_no":           order.OrderNo,
		"status":             string(order.Status),
		"total_amount_minor": order.TotalAmount,
		"shipped_at":         order.ShippedAt,
	}, nil
}

func executePluginHostOrderUpdatePrice(
	orderRepo *repository.OrderRepository,
	params map[string]interface{},
) (map[string]interface{}, error) {
	if orderRepo == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "order repository is unavailable"}
	}

	orderID, hasID, err := parsePluginHostOptionalUint(params, "id", "order_id")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	orderNo := parsePluginHostOptionalString(params, "order_no")
	totalAmountMinor, hasAmount, err := parsePluginHostOptionalInt64(params, "total_amount_minor", "totalAmountMinor")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	if !hasAmount {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "total_amount_minor/totalAmountMinor is required"}
	}
	if totalAmountMinor < 0 {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: orderbiz.TotalAmountNegative().Message}
	}

	var order *models.Order
	switch {
	case hasID:
		order, err = orderRepo.FindByID(orderID)
	case orderNo != "":
		order, err = orderRepo.FindByOrderNo(orderNo)
	default:
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "id/order_id or order_no is required"}
	}
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &PluginHostActionError{Status: http.StatusNotFound, Message: "order not found"}
		}
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query order failed"}
	}

	if order.Status != models.OrderStatusPendingPayment {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: orderbiz.UpdatePriceStatusInvalid(order.Status).Message}
	}

	order.TotalAmount = totalAmountMinor
	if err := orderRepo.Update(order); err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "update order price failed"}
	}

	return map[string]interface{}{
		"id":                 order.ID,
		"order_no":           order.OrderNo,
		"status":             string(order.Status),
		"total_amount_minor": order.TotalAmount,
	}, nil
}

func executePluginHostUserGet(
	db *gorm.DB,
	userRepo *repository.UserRepository,
	claims *PluginHostAccessClaims,
	params map[string]interface{},
) (map[string]interface{}, error) {
	if userRepo == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "user repository is unavailable"}
	}

	userID, hasID, err := parsePluginHostOptionalUint(params, "id", "user_id")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	email := parsePluginHostOptionalString(params, "email")
	uuid := parsePluginHostOptionalString(params, "uuid")

	var user *models.User
	switch {
	case hasID:
		user, err = userRepo.FindByID(userID)
	case email != "":
		user, err = userRepo.FindByEmail(email)
	case uuid != "":
		user, err = userRepo.FindByUUID(uuid)
	default:
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "id/user_id, email or uuid is required"}
	}
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &PluginHostActionError{Status: http.StatusNotFound, Message: "user not found"}
		}
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query user failed"}
	}

	return buildPluginHostUserDetailResponse(db, user), nil
}

func executePluginHostUserList(
	db *gorm.DB,
	userRepo *repository.UserRepository,
	claims *PluginHostAccessClaims,
	params map[string]interface{},
) (map[string]interface{}, error) {
	if userRepo == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "user repository is unavailable"}
	}

	page := parsePluginHostPositiveInt(params, 1, 1, 10000, "page")
	pageSize := parsePluginHostPositiveInt(params, 20, 1, 100, "page_size", "pageSize", "limit")
	search := parsePluginHostOptionalString(params, "search", "q")
	role := parsePluginHostOptionalString(params, "role")
	locale := parsePluginHostOptionalString(params, "locale")
	country := parsePluginHostOptionalString(params, "country")

	isActive, err := parsePluginHostOptionalBool(params, "is_active", "isActive")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	emailVerified, err := parsePluginHostOptionalBool(params, "email_verified", "emailVerified")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	emailNotifyMarketing, err := parsePluginHostOptionalBool(params, "email_notify_marketing", "emailNotifyMarketing")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	smsNotifyMarketing, err := parsePluginHostOptionalBool(params, "sms_notify_marketing", "smsNotifyMarketing")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	hasPhone, err := parsePluginHostOptionalBool(params, "has_phone", "hasPhone")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}

	users, total, err := userRepo.List(page, pageSize, repository.UserListFilters{
		Search:               search,
		Role:                 role,
		IsActive:             isActive,
		EmailVerified:        emailVerified,
		EmailNotifyMarketing: emailNotifyMarketing,
		SMSNotifyMarketing:   smsNotifyMarketing,
		HasPhone:             hasPhone,
		Locale:               locale,
		Country:              country,
	})
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query users failed"}
	}

	items := make([]map[string]interface{}, 0, len(users))
	for idx := range users {
		items = append(items, buildPluginHostUserSummaryResponse(db, &users[idx]))
	}

	return map[string]interface{}{
		"items":     items,
		"page":      page,
		"page_size": pageSize,
		"total":     total,
		"has_more":  int64(page*pageSize) < total,
		"query":     search,
		"role":      role,
		"country":   country,
		"locale":    locale,
	}, nil
}

func executePluginHostProductGet(
	productRepo *repository.ProductRepository,
	params map[string]interface{},
) (map[string]interface{}, error) {
	if productRepo == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "product repository is unavailable"}
	}

	productID, hasID, err := parsePluginHostOptionalUint(params, "id", "product_id")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	sku := parsePluginHostOptionalString(params, "sku")

	var product *models.Product
	switch {
	case hasID:
		product, err = productRepo.FindByID(productID)
	case sku != "":
		product, err = productRepo.FindBySKU(sku)
	default:
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "id/product_id or sku is required"}
	}
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &PluginHostActionError{Status: http.StatusNotFound, Message: "product not found"}
		}
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query product failed"}
	}

	return buildPluginHostProductDetailResponse(product), nil
}

func executePluginHostProductList(
	productRepo *repository.ProductRepository,
	params map[string]interface{},
) (map[string]interface{}, error) {
	if productRepo == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "product repository is unavailable"}
	}

	page := parsePluginHostPositiveInt(params, 1, 1, 10000, "page")
	pageSize := parsePluginHostPositiveInt(params, 20, 1, 100, "page_size", "pageSize", "limit")
	status := parsePluginHostOptionalString(params, "status")
	category := parsePluginHostOptionalString(params, "category")
	search := parsePluginHostOptionalString(params, "search", "q")
	isFeatured, err := parsePluginHostOptionalBool(params, "is_featured", "isFeatured")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	isRecommended, err := parsePluginHostOptionalBool(params, "is_recommended", "isRecommended")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	isActive, err := parsePluginHostOptionalBool(params, "is_active", "isActive")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}

	products, total, err := productRepo.List(
		page,
		pageSize,
		status,
		category,
		search,
		isFeatured,
		isRecommended,
		isActive != nil && *isActive,
	)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query products failed"}
	}

	items := make([]map[string]interface{}, 0, len(products))
	for idx := range products {
		items = append(items, buildPluginHostProductSummaryResponse(&products[idx]))
	}

	return map[string]interface{}{
		"items":          items,
		"page":           page,
		"page_size":      pageSize,
		"total":          total,
		"has_more":       int64(page*pageSize) < total,
		"query":          search,
		"status":         status,
		"category":       category,
		"is_featured":    isFeatured != nil && *isFeatured,
		"is_recommended": isRecommended != nil && *isRecommended,
		"is_active":      isActive != nil && *isActive,
	}, nil
}

func executePluginHostInventoryGet(
	inventoryRepo *repository.InventoryRepository,
	params map[string]interface{},
) (map[string]interface{}, error) {
	if inventoryRepo == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "inventory repository is unavailable"}
	}

	inventoryID, hasID, err := parsePluginHostOptionalUint(params, "id", "inventory_id")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	sku := parsePluginHostOptionalString(params, "sku")

	var inventory *models.Inventory
	switch {
	case hasID:
		inventory, err = inventoryRepo.FindByID(inventoryID)
	case sku != "":
		var bySKU models.Inventory
		if err = inventoryRepo.FindBySKU(sku, &bySKU); err == nil {
			inventory = &bySKU
		}
	default:
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "id/inventory_id or sku is required"}
	}
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &PluginHostActionError{Status: http.StatusNotFound, Message: "inventory not found"}
		}
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query inventory failed"}
	}

	return buildPluginHostInventoryDetailResponse(inventory), nil
}

func executePluginHostInventoryList(
	inventoryRepo *repository.InventoryRepository,
	params map[string]interface{},
) (map[string]interface{}, error) {
	if inventoryRepo == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "inventory repository is unavailable"}
	}

	page := parsePluginHostPositiveInt(params, 1, 1, 10000, "page")
	pageSize := parsePluginHostPositiveInt(params, 20, 1, 100, "page_size", "pageSize", "limit")
	isActive, err := parsePluginHostOptionalBool(params, "is_active", "isActive")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	lowStock, err := parsePluginHostOptionalBool(params, "low_stock", "lowStock")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}

	filters := map[string]interface{}{}
	if isActive != nil {
		filters["is_active"] = *isActive
	}
	if lowStock != nil {
		filters["low_stock"] = *lowStock
	}

	inventories, total, err := inventoryRepo.List(page, pageSize, filters)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query inventories failed"}
	}

	items := make([]map[string]interface{}, 0, len(inventories))
	for idx := range inventories {
		items = append(items, buildPluginHostInventorySummaryResponse(&inventories[idx]))
	}

	return map[string]interface{}{
		"items":     items,
		"page":      page,
		"page_size": pageSize,
		"total":     total,
		"has_more":  int64(page*pageSize) < total,
		"is_active": isActive != nil && *isActive,
		"low_stock": lowStock != nil && *lowStock,
	}, nil
}

func executePluginHostInventoryBindingGet(
	db *gorm.DB,
	params map[string]interface{},
) (map[string]interface{}, error) {
	if db == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "inventory binding database is unavailable"}
	}

	bindingID, hasID, err := parsePluginHostOptionalUint(params, "id", "binding_id", "inventory_binding_id", "inventoryBindingId")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	if !hasID {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "id/binding_id/inventory_binding_id is required"}
	}

	var binding models.ProductInventoryBinding
	if err := db.Preload("Product").Preload("Inventory").First(&binding, bindingID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &PluginHostActionError{Status: http.StatusNotFound, Message: "inventory binding not found"}
		}
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query inventory binding failed"}
	}

	return buildPluginHostInventoryBindingDetailResponse(&binding), nil
}

func executePluginHostInventoryBindingList(
	db *gorm.DB,
	params map[string]interface{},
) (map[string]interface{}, error) {
	if db == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "inventory binding database is unavailable"}
	}

	page := parsePluginHostPositiveInt(params, 1, 1, 10000, "page")
	pageSize := parsePluginHostPositiveInt(params, 20, 1, 100, "page_size", "pageSize", "limit")

	query := db.Model(&models.ProductInventoryBinding{})

	var productIDValue interface{}
	if productID, ok, err := parsePluginHostOptionalUint(params, "product_id", "productId"); err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	} else if ok {
		query = query.Where("product_id = ?", productID)
		productIDValue = productID
	}

	var inventoryIDValue interface{}
	if inventoryID, ok, err := parsePluginHostOptionalUint(params, "inventory_id", "inventoryId"); err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	} else if ok {
		query = query.Where("inventory_id = ?", inventoryID)
		inventoryIDValue = inventoryID
	}

	isRandom, err := parsePluginHostOptionalBool(params, "is_random", "isRandom")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	if isRandom != nil {
		query = query.Where("is_random = ?", *isRandom)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "count inventory bindings failed"}
	}

	var bindings []models.ProductInventoryBinding
	offset := (page - 1) * pageSize
	if err := query.Preload("Product").
		Preload("Inventory").
		Order("created_at ASC, id ASC").
		Offset(offset).
		Limit(pageSize).
		Find(&bindings).Error; err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query inventory bindings failed"}
	}

	items := make([]map[string]interface{}, 0, len(bindings))
	for idx := range bindings {
		items = append(items, buildPluginHostInventoryBindingSummaryResponse(&bindings[idx]))
	}

	return map[string]interface{}{
		"items":        items,
		"page":         page,
		"page_size":    pageSize,
		"total":        total,
		"has_more":     int64(page*pageSize) < total,
		"product_id":   productIDValue,
		"inventory_id": inventoryIDValue,
		"is_random":    isRandom != nil && *isRandom,
	}, nil
}

func executePluginHostPromoGet(
	promoRepo *repository.PromoCodeRepository,
	params map[string]interface{},
) (map[string]interface{}, error) {
	if promoRepo == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "promo repository is unavailable"}
	}

	promoID, hasID, err := parsePluginHostOptionalUint(params, "id", "promo_id", "promo_code_id")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	code := strings.ToUpper(parsePluginHostOptionalString(params, "code", "promo_code", "promoCode"))

	var promo *models.PromoCode
	switch {
	case hasID:
		promo, err = promoRepo.FindByID(promoID)
	case code != "":
		promo, err = promoRepo.FindByCode(code)
	default:
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "id/promo_id/promo_code_id or code is required"}
	}
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &PluginHostActionError{Status: http.StatusNotFound, Message: "promo code not found"}
		}
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query promo code failed"}
	}

	return buildPluginHostPromoDetailResponse(promo), nil
}

func executePluginHostPromoList(
	promoRepo *repository.PromoCodeRepository,
	params map[string]interface{},
) (map[string]interface{}, error) {
	if promoRepo == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "promo repository is unavailable"}
	}

	page := parsePluginHostPositiveInt(params, 1, 1, 10000, "page")
	pageSize := parsePluginHostPositiveInt(params, 20, 1, 100, "page_size", "pageSize", "limit")
	status := parsePluginHostOptionalString(params, "status")
	search := parsePluginHostOptionalString(params, "search", "q")

	promos, total, err := promoRepo.List(page, pageSize, status, search)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query promo codes failed"}
	}

	items := make([]map[string]interface{}, 0, len(promos))
	for idx := range promos {
		items = append(items, buildPluginHostPromoSummaryResponse(&promos[idx]))
	}

	return map[string]interface{}{
		"items":     items,
		"page":      page,
		"page_size": pageSize,
		"total":     total,
		"has_more":  int64(page*pageSize) < total,
		"query":     search,
		"status":    status,
	}, nil
}

func executePluginHostTicketGet(
	db *gorm.DB,
	params map[string]interface{},
) (map[string]interface{}, error) {
	if db == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "ticket database is unavailable"}
	}

	ticketID, hasID, err := parsePluginHostOptionalUint(params, "id", "ticket_id")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	ticketNo := parsePluginHostOptionalString(params, "ticket_no")

	query := db.Preload("User").Preload("AssignedUser")
	var ticket models.Ticket
	switch {
	case hasID:
		err = query.First(&ticket, ticketID).Error
	case ticketNo != "":
		err = query.Where("ticket_no = ?", ticketNo).First(&ticket).Error
	default:
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "id/ticket_id or ticket_no is required"}
	}
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &PluginHostActionError{Status: http.StatusNotFound, Message: "ticket not found"}
		}
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query ticket failed"}
	}

	return buildPluginHostTicketDetailResponse(&ticket), nil
}

func executePluginHostTicketList(
	db *gorm.DB,
	claims *PluginHostAccessClaims,
	params map[string]interface{},
) (map[string]interface{}, error) {
	if db == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "ticket database is unavailable"}
	}

	page := parsePluginHostPositiveInt(params, 1, 1, 10000, "page")
	pageSize := parsePluginHostPositiveInt(params, 20, 1, 100, "page_size", "pageSize", "limit")
	status := parsePluginHostOptionalString(params, "status")
	excludeStatus := parsePluginHostOptionalString(params, "exclude_status", "excludeStatus")
	search := parsePluginHostOptionalString(params, "search", "q")
	assignedTo := parsePluginHostOptionalString(params, "assigned_to", "assignedTo")
	assignedToMe, err := parsePluginHostOptionalBool(params, "assigned_to_me", "assignedToMe")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	unassigned, err := parsePluginHostOptionalBool(params, "unassigned")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}

	query := db.Model(&models.Ticket{}).Preload("User").Preload("AssignedUser")
	if status != "" {
		query = query.Where("status = ?", status)
	} else if excludeStatus != "" {
		query = query.Where("status <> ?", excludeStatus)
	}
	if search != "" {
		query = query.Where("ticket_no LIKE ? OR subject LIKE ?", "%"+search+"%", "%"+search+"%")
	}

	switch {
	case assignedToMe != nil && *assignedToMe:
		if claims == nil || claims.OperatorUserID == 0 {
			return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "assigned_to_me requires operator user context"}
		}
		query = query.Where("assigned_to = ?", claims.OperatorUserID)
	case unassigned != nil && *unassigned:
		query = query.Where("assigned_to IS NULL")
	case assignedTo != "":
		switch strings.ToLower(assignedTo) {
		case "me":
			if claims == nil || claims.OperatorUserID == 0 {
				return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "assigned_to=me requires operator user context"}
			}
			query = query.Where("assigned_to = ?", claims.OperatorUserID)
		case "unassigned":
			query = query.Where("assigned_to IS NULL")
		default:
			parsed, parseErr := strconv.ParseUint(assignedTo, 10, 64)
			if parseErr != nil {
				return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "assigned_to must be unsigned integer, me or unassigned"}
			}
			query = query.Where("assigned_to = ?", uint(parsed))
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "count tickets failed"}
	}

	orderClause := `
		CASE status
			WHEN 'open' THEN 0
			WHEN 'processing' THEN 1
			WHEN 'resolved' THEN 2
			WHEN 'closed' THEN 3
			ELSE 4
		END ASC,
		CASE WHEN unread_count_admin > 0 THEN 0 ELSE 1 END ASC,
		last_message_at DESC`

	var tickets []models.Ticket
	offset := (page - 1) * pageSize
	if err := query.Order(orderClause).Offset(offset).Limit(pageSize).Find(&tickets).Error; err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query tickets failed"}
	}

	items := make([]map[string]interface{}, 0, len(tickets))
	for idx := range tickets {
		items = append(items, buildPluginHostTicketSummaryResponse(&tickets[idx]))
	}

	return map[string]interface{}{
		"items":          items,
		"page":           page,
		"page_size":      pageSize,
		"total":          total,
		"has_more":       int64(page*pageSize) < total,
		"query":          search,
		"status":         status,
		"exclude_status": excludeStatus,
		"assigned_to":    assignedTo,
		"assigned_to_me": assignedToMe != nil && *assignedToMe,
		"unassigned":     unassigned != nil && *unassigned,
	}, nil
}

func executePluginHostTicketReply(
	db *gorm.DB,
	claims *PluginHostAccessClaims,
	params map[string]interface{},
) (map[string]interface{}, error) {
	if db == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "ticket database is unavailable"}
	}
	if claims == nil || claims.OperatorUserID == 0 {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "ticket reply requires operator user context"}
	}

	ticketID, hasID, err := parsePluginHostOptionalUint(params, "id", "ticket_id")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	ticketNo := parsePluginHostOptionalString(params, "ticket_no")
	rawContent := parsePluginHostOptionalString(params, "content")
	if strings.TrimSpace(rawContent) == "" {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "content is required"}
	}
	contentType := strings.TrimSpace(parsePluginHostOptionalString(params, "content_type", "contentType"))
	if contentType == "" {
		contentType = "text"
	}

	var operator models.User
	if err := db.Select("id", "name").First(&operator, claims.OperatorUserID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &PluginHostActionError{Status: http.StatusNotFound, Message: "operator not found"}
		}
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query operator failed"}
	}

	cfg := config.GetConfig()
	sanitizedContent := validator.SanitizeMarkdown(rawContent)
	if strings.TrimSpace(sanitizedContent) == "" {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "content is required"}
	}
	if cfg != nil && cfg.Ticket.MaxContentLength > 0 && len([]rune(sanitizedContent)) > cfg.Ticket.MaxContentLength {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: ticketbiz.ContentTooLong(cfg.Ticket.MaxContentLength).Message}
	}

	var replyMessage models.TicketMessage
	var ticket models.Ticket
	afterStatus := models.TicketStatusOpen
	var afterAssignedTo *uint

	if err := db.Transaction(func(tx *gorm.DB) error {
		query := tx.Model(&models.Ticket{})
		switch {
		case hasID:
			if txErr := query.First(&ticket, ticketID).Error; txErr != nil {
				return txErr
			}
		case ticketNo != "":
			if txErr := query.Where("ticket_no = ?", ticketNo).First(&ticket).Error; txErr != nil {
				return txErr
			}
		default:
			return &PluginHostActionError{Status: http.StatusBadRequest, Message: "id/ticket_id or ticket_no is required"}
		}

		if ticket.Status == models.TicketStatusClosed {
			return ticketbiz.ClosedCannotSend()
		}

		replyMessage = models.TicketMessage{
			TicketID:      ticket.ID,
			SenderType:    "admin",
			SenderID:      claims.OperatorUserID,
			SenderName:    operator.Name,
			Content:       sanitizedContent,
			ContentType:   contentType,
			IsReadByUser:  false,
			IsReadByAdmin: true,
		}
		if strings.TrimSpace(replyMessage.SenderName) == "" {
			replyMessage.SenderName = "Admin"
		}
		if txErr := tx.Create(&replyMessage).Error; txErr != nil {
			return txErr
		}

		now := time.Now()
		updates := map[string]interface{}{
			"last_message_at":      now,
			"last_message_preview": truncateString(sanitizedContent, 200),
			"last_message_by":      "admin",
			"unread_count_user":    gorm.Expr("unread_count_user + 1"),
		}
		afterStatus = ticket.Status
		if ticket.AssignedTo == nil {
			updates["assigned_to"] = claims.OperatorUserID
			assignedTo := claims.OperatorUserID
			afterAssignedTo = &assignedTo
		} else {
			assignedTo := *ticket.AssignedTo
			afterAssignedTo = &assignedTo
		}
		if ticket.Status == models.TicketStatusOpen {
			updates["status"] = models.TicketStatusProcessing
			afterStatus = models.TicketStatusProcessing
		}
		if txErr := tx.Model(&ticket).Updates(updates).Error; txErr != nil {
			return txErr
		}

		if afterAssignedTo == nil {
			afterAssignedTo = ticket.AssignedTo
		}
		return nil
	}); err != nil {
		var hostErr *PluginHostActionError
		if errors.As(err, &hostErr) {
			return nil, hostErr
		}
		var typedBizErr *bizerr.Error
		if errors.As(err, &typedBizErr) {
			return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: typedBizErr.Message}
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &PluginHostActionError{Status: http.StatusNotFound, Message: "ticket not found"}
		}
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "reply ticket failed"}
	}

	if emailService := pluginHostEmailService(db); emailService != nil {
		go emailService.SendTicketAdminReplyEmail(&ticket, replyMessage.SenderName, truncateString(sanitizedContent, 200))
	}

	return map[string]interface{}{
		"id":           replyMessage.ID,
		"ticket_id":    ticket.ID,
		"ticket_no":    ticket.TicketNo,
		"status":       string(afterStatus),
		"assigned_to":  derefPluginHostUint(afterAssignedTo),
		"content_type": replyMessage.ContentType,
		"created_at":   replyMessage.CreatedAt,
	}, nil
}

func executePluginHostTicketUpdate(
	db *gorm.DB,
	params map[string]interface{},
) (map[string]interface{}, error) {
	if db == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "ticket database is unavailable"}
	}

	ticketID, hasID, err := parsePluginHostOptionalUint(params, "id", "ticket_id")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	ticketNo := parsePluginHostOptionalString(params, "ticket_no")
	statusText := parsePluginHostOptionalString(params, "status")
	priorityText := parsePluginHostOptionalString(params, "priority")
	assignedTo, hasAssignedTo, err := parsePluginHostOptionalUint(params, "assigned_to", "assignedTo")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	clearAssignee, err := parsePluginHostOptionalBool(params, "clear_assignee", "clearAssignee", "unassigned")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}

	updates := make(map[string]interface{})
	if strings.TrimSpace(statusText) != "" {
		status, ok := ticketbiz.ParseStatus(statusText)
		if !ok {
			return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: ticketbiz.StatusInvalid().Message}
		}
		updates["status"] = status
		if status == models.TicketStatusClosed || status == models.TicketStatusResolved {
			now := time.Now()
			updates["closed_at"] = now
		} else {
			updates["closed_at"] = nil
		}
	}
	if strings.TrimSpace(priorityText) != "" {
		priority, ok := ticketbiz.ParsePriority(priorityText)
		if !ok {
			return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: ticketbiz.PriorityInvalid().Message}
		}
		updates["priority"] = priority
	}
	if clearAssignee != nil && *clearAssignee {
		updates["assigned_to"] = nil
	} else if hasAssignedTo {
		updates["assigned_to"] = assignedTo
	}
	if len(updates) == 0 {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "at least one of status, priority, assigned_to, or clear_assignee is required"}
	}

	query := db.Model(&models.Ticket{})
	var ticket models.Ticket
	switch {
	case hasID:
		if err := query.First(&ticket, ticketID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, &PluginHostActionError{Status: http.StatusNotFound, Message: "ticket not found"}
			}
			return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query ticket failed"}
		}
	case ticketNo != "":
		if err := query.Where("ticket_no = ?", ticketNo).First(&ticket).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, &PluginHostActionError{Status: http.StatusNotFound, Message: "ticket not found"}
			}
			return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query ticket failed"}
		}
	default:
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "id/ticket_id or ticket_no is required"}
	}

	if err := db.Model(&ticket).Updates(updates).Error; err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "update ticket failed"}
	}
	if err := db.Preload("User").Preload("AssignedUser").First(&ticket, ticket.ID).Error; err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "reload ticket failed"}
	}

	return map[string]interface{}{
		"id":          ticket.ID,
		"ticket_no":   ticket.TicketNo,
		"status":      string(ticket.Status),
		"priority":    string(ticket.Priority),
		"assigned_to": derefPluginHostUint(ticket.AssignedTo),
		"closed_at":   ticket.ClosedAt,
	}, nil
}

func executePluginHostSerialGet(
	db *gorm.DB,
	serialRepo *repository.SerialRepository,
	params map[string]interface{},
) (map[string]interface{}, error) {
	if db == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "serial database is unavailable"}
	}
	if serialRepo == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "serial repository is unavailable"}
	}

	serialID, hasID, err := parsePluginHostOptionalUint(params, "id", "serial_id")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	serialNumber := parsePluginHostOptionalString(params, "serial_number", "serialNumber")

	var serial models.ProductSerial
	switch {
	case hasID:
		err = db.Preload("Product").Preload("Order").First(&serial, serialID).Error
	case serialNumber != "":
		found, findErr := serialRepo.FindBySerialNumber(serialNumber)
		if findErr == nil && found != nil {
			serial = *found
		}
		err = findErr
	default:
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "id/serial_id or serial_number is required"}
	}
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &PluginHostActionError{Status: http.StatusNotFound, Message: "serial not found"}
		}
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query serial failed"}
	}

	return buildPluginHostSerialDetailResponse(&serial), nil
}

func executePluginHostSerialList(
	serialRepo *repository.SerialRepository,
	params map[string]interface{},
) (map[string]interface{}, error) {
	if serialRepo == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "serial repository is unavailable"}
	}

	page := parsePluginHostPositiveInt(params, 1, 1, 10000, "page")
	pageSize := parsePluginHostPositiveInt(params, 20, 1, 100, "page_size", "pageSize", "limit")
	filters := make(map[string]interface{})

	if productID, ok, err := parsePluginHostOptionalUint(params, "product_id", "productId"); err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	} else if ok {
		filters["product_id"] = productID
	}
	if orderID, ok, err := parsePluginHostOptionalUint(params, "order_id", "orderId"); err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	} else if ok {
		filters["order_id"] = orderID
	}
	if productCode := parsePluginHostOptionalString(params, "product_code", "productCode"); productCode != "" {
		filters["product_code"] = productCode
	}
	if serialNumber := parsePluginHostOptionalString(params, "serial_number", "serialNumber"); serialNumber != "" {
		filters["serial_number"] = serialNumber
	}

	serials, total, err := serialRepo.List(page, pageSize, filters)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query serials failed"}
	}

	items := make([]map[string]interface{}, 0, len(serials))
	for idx := range serials {
		items = append(items, buildPluginHostSerialSummaryResponse(&serials[idx]))
	}

	return map[string]interface{}{
		"items":         items,
		"page":          page,
		"page_size":     pageSize,
		"total":         total,
		"has_more":      int64(page*pageSize) < total,
		"product_id":    filters["product_id"],
		"order_id":      filters["order_id"],
		"product_code":  filters["product_code"],
		"serial_number": filters["serial_number"],
	}, nil
}

func executePluginHostAnnouncementGet(
	db *gorm.DB,
	params map[string]interface{},
) (map[string]interface{}, error) {
	if db == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "announcement database is unavailable"}
	}

	announcementID, hasID, err := parsePluginHostOptionalUint(params, "id", "announcement_id")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	if !hasID {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "id/announcement_id is required"}
	}

	var announcement models.Announcement
	if err := db.First(&announcement, announcementID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &PluginHostActionError{Status: http.StatusNotFound, Message: "announcement not found"}
		}
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query announcement failed"}
	}

	return buildPluginHostAnnouncementDetailResponse(&announcement), nil
}

func executePluginHostAnnouncementList(
	db *gorm.DB,
	params map[string]interface{},
) (map[string]interface{}, error) {
	if db == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "announcement database is unavailable"}
	}

	page := parsePluginHostPositiveInt(params, 1, 1, 10000, "page")
	pageSize := parsePluginHostPositiveInt(params, 20, 1, 100, "page_size", "pageSize", "limit")
	search := parsePluginHostOptionalString(params, "search", "q")
	category := parsePluginHostOptionalString(params, "category")
	isMandatory, err := parsePluginHostOptionalBool(params, "is_mandatory", "isMandatory")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}

	query := db.Model(&models.Announcement{})
	if search != "" {
		query = query.Where("title LIKE ? OR content LIKE ?", "%"+search+"%", "%"+search+"%")
	}
	if category != "" {
		query = query.Where("category = ?", category)
	}
	if isMandatory != nil {
		query = query.Where("is_mandatory = ?", *isMandatory)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "count announcements failed"}
	}

	var announcements []models.Announcement
	offset := (page - 1) * pageSize
	if err := query.Order("id DESC").Offset(offset).Limit(pageSize).Find(&announcements).Error; err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query announcements failed"}
	}

	items := make([]map[string]interface{}, 0, len(announcements))
	for idx := range announcements {
		items = append(items, buildPluginHostAnnouncementSummaryResponse(&announcements[idx]))
	}

	return map[string]interface{}{
		"items":        items,
		"page":         page,
		"page_size":    pageSize,
		"total":        total,
		"has_more":     int64(page*pageSize) < total,
		"query":        search,
		"category":     category,
		"is_mandatory": isMandatory != nil && *isMandatory,
	}, nil
}

func executePluginHostKnowledgeGet(
	db *gorm.DB,
	params map[string]interface{},
) (map[string]interface{}, error) {
	if db == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "knowledge database is unavailable"}
	}

	articleID, hasID, err := parsePluginHostOptionalUint(params, "id", "article_id")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	if !hasID {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "id/article_id is required"}
	}

	var article models.KnowledgeArticle
	if err := db.Preload("Category").First(&article, articleID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &PluginHostActionError{Status: http.StatusNotFound, Message: "knowledge article not found"}
		}
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query knowledge article failed"}
	}

	return buildPluginHostKnowledgeDetailResponse(&article), nil
}

func executePluginHostKnowledgeList(
	db *gorm.DB,
	params map[string]interface{},
) (map[string]interface{}, error) {
	if db == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "knowledge database is unavailable"}
	}

	page := parsePluginHostPositiveInt(params, 1, 1, 10000, "page")
	pageSize := parsePluginHostPositiveInt(params, 20, 1, 100, "page_size", "pageSize", "limit")
	search := parsePluginHostOptionalString(params, "search", "q")

	query := db.Model(&models.KnowledgeArticle{})
	var categoryIDValue interface{}
	if categoryID, ok, err := parsePluginHostOptionalUint(params, "category_id", "categoryId"); err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	} else if ok {
		ids, descendantErr := pluginHostCollectKnowledgeCategoryDescendantIDs(db, categoryID)
		if descendantErr != nil {
			return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query knowledge categories failed"}
		}
		query = query.Where("category_id IN ?", ids)
		categoryIDValue = categoryID
	}
	if search != "" {
		query = query.Where("title LIKE ?", "%"+search+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "count knowledge articles failed"}
	}

	var articles []models.KnowledgeArticle
	offset := (page - 1) * pageSize
	if err := query.Preload("Category").
		Order("sort_order ASC, id DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&articles).Error; err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query knowledge articles failed"}
	}

	items := make([]map[string]interface{}, 0, len(articles))
	for idx := range articles {
		items = append(items, buildPluginHostKnowledgeSummaryResponse(&articles[idx]))
	}

	return map[string]interface{}{
		"items":       items,
		"page":        page,
		"page_size":   pageSize,
		"total":       total,
		"has_more":    int64(page*pageSize) < total,
		"query":       search,
		"category_id": categoryIDValue,
	}, nil
}

func executePluginHostKnowledgeCategories(db *gorm.DB) (map[string]interface{}, error) {
	if db == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "knowledge database is unavailable"}
	}

	categories, err := pluginHostLoadKnowledgeCategoryTree(db)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query knowledge categories failed"}
	}
	pluginHostPopulateKnowledgeCategoryArticleCounts(db, categories)
	items := pluginHostMarshalToSlice(categories)
	return map[string]interface{}{
		"items":         items,
		"total_roots":   len(items),
		"total_entries": pluginHostCountKnowledgeCategories(categories),
	}, nil
}

func executePluginHostPaymentMethodGet(
	db *gorm.DB,
	params map[string]interface{},
) (map[string]interface{}, error) {
	if db == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "payment method database is unavailable"}
	}

	paymentMethodID, hasID, err := parsePluginHostOptionalUint(params, "id", "payment_method_id", "paymentMethodId")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	if !hasID {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "id/payment_method_id is required"}
	}

	var method models.PaymentMethod
	if err := db.First(&method, paymentMethodID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &PluginHostActionError{Status: http.StatusNotFound, Message: "payment method not found"}
		}
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query payment method failed"}
	}

	return buildPluginHostPaymentMethodDetailResponse(&method), nil
}

func executePluginHostPaymentMethodList(
	db *gorm.DB,
	params map[string]interface{},
) (map[string]interface{}, error) {
	if db == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "payment method database is unavailable"}
	}

	page := parsePluginHostPositiveInt(params, 1, 1, 10000, "page")
	pageSize := parsePluginHostPositiveInt(params, 20, 1, 100, "page_size", "pageSize", "limit")
	search := parsePluginHostOptionalString(params, "search", "q")
	methodType := parsePluginHostOptionalString(params, "type")
	enabledOnly, err := parsePluginHostOptionalBool(params, "enabled_only", "enabledOnly")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}

	query := db.Model(&models.PaymentMethod{})
	if search != "" {
		query = query.Where("name LIKE ? OR description LIKE ?", "%"+search+"%", "%"+search+"%")
	}
	if methodType != "" {
		query = query.Where("type = ?", methodType)
	}
	if enabledOnly != nil && *enabledOnly {
		query = query.Where("enabled = ?", true)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "count payment methods failed"}
	}

	var methods []models.PaymentMethod
	offset := (page - 1) * pageSize
	if err := query.Order("sort_order ASC, id ASC").Offset(offset).Limit(pageSize).Find(&methods).Error; err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query payment methods failed"}
	}

	items := make([]map[string]interface{}, 0, len(methods))
	for idx := range methods {
		items = append(items, buildPluginHostPaymentMethodSummaryResponse(&methods[idx]))
	}

	return map[string]interface{}{
		"items":        items,
		"page":         page,
		"page_size":    pageSize,
		"total":        total,
		"has_more":     int64(page*pageSize) < total,
		"query":        search,
		"type":         methodType,
		"enabled_only": enabledOnly != nil && *enabledOnly,
	}, nil
}

func executePluginHostVirtualInventoryGet(
	db *gorm.DB,
	params map[string]interface{},
) (map[string]interface{}, error) {
	if db == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "virtual inventory database is unavailable"}
	}

	virtualInventoryID, hasID, err := parsePluginHostOptionalUint(params, "id", "virtual_inventory_id", "virtualInventoryId")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	sku := parsePluginHostOptionalString(params, "sku")

	query := db.Model(&models.VirtualInventory{})
	if hasID {
		query = query.Where("id = ?", virtualInventoryID)
	} else if sku != "" {
		query = query.Where("sku = ?", sku)
	} else {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "id/virtual_inventory_id or sku is required"}
	}

	var inventory models.VirtualInventory
	if err := query.First(&inventory).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &PluginHostActionError{Status: http.StatusNotFound, Message: "virtual inventory not found"}
		}
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query virtual inventory failed"}
	}

	virtualInventoryService := NewVirtualInventoryService(db)
	stats, err := virtualInventoryService.GetStockStats(inventory.ID)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query virtual inventory stats failed"}
	}

	return buildPluginHostVirtualInventoryDetailResponse(&inventory, stats), nil
}

func executePluginHostVirtualInventoryList(
	db *gorm.DB,
	params map[string]interface{},
) (map[string]interface{}, error) {
	if db == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "virtual inventory database is unavailable"}
	}

	page := parsePluginHostPositiveInt(params, 1, 1, 10000, "page")
	pageSize := parsePluginHostPositiveInt(params, 20, 1, 100, "page_size", "pageSize", "limit")
	search := parsePluginHostOptionalString(params, "search", "q")
	virtualInventoryType := parsePluginHostOptionalString(params, "type")
	isActive, err := parsePluginHostOptionalBool(params, "is_active", "isActive")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}

	query := db.Model(&models.VirtualInventory{})
	if search != "" {
		query = query.Where("name LIKE ? OR sku LIKE ?", "%"+search+"%", "%"+search+"%")
	}
	if virtualInventoryType != "" {
		query = query.Where("type = ?", virtualInventoryType)
	}
	if isActive != nil {
		query = query.Where("is_active = ?", *isActive)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "count virtual inventories failed"}
	}

	var inventories []models.VirtualInventory
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC, id DESC").Offset(offset).Limit(pageSize).Find(&inventories).Error; err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query virtual inventories failed"}
	}

	inventoryIDs := make([]uint, 0, len(inventories))
	for _, inventory := range inventories {
		inventoryIDs = append(inventoryIDs, inventory.ID)
	}
	virtualInventoryService := NewVirtualInventoryService(db)
	statsByInventory, err := virtualInventoryService.getStockStatsForInventories(inventoryIDs)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query virtual inventory stats failed"}
	}

	items := make([]map[string]interface{}, 0, len(inventories))
	for idx := range inventories {
		items = append(items, buildPluginHostVirtualInventorySummaryResponse(&inventories[idx], statsByInventory[inventories[idx].ID]))
	}

	return map[string]interface{}{
		"items":     items,
		"page":      page,
		"page_size": pageSize,
		"total":     total,
		"has_more":  int64(page*pageSize) < total,
		"query":     search,
		"type":      virtualInventoryType,
		"is_active": isActive != nil && *isActive,
	}, nil
}

func executePluginHostVirtualInventoryBindingGet(
	db *gorm.DB,
	params map[string]interface{},
) (map[string]interface{}, error) {
	if db == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "virtual inventory binding database is unavailable"}
	}

	bindingID, hasID, err := parsePluginHostOptionalUint(params, "id", "binding_id", "virtual_binding_id", "virtualBindingId")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	if !hasID {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "id/binding_id/virtual_binding_id is required"}
	}

	var binding models.ProductVirtualInventoryBinding
	if err := db.Preload("Product").Preload("VirtualInventory").First(&binding, bindingID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &PluginHostActionError{Status: http.StatusNotFound, Message: "virtual inventory binding not found"}
		}
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query virtual inventory binding failed"}
	}

	virtualInventoryService := NewVirtualInventoryService(db)
	stats, err := virtualInventoryService.GetStockStats(binding.VirtualInventoryID)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query virtual inventory stats failed"}
	}

	return buildPluginHostVirtualInventoryBindingDetailResponse(&binding, stats), nil
}

func executePluginHostVirtualInventoryBindingList(
	db *gorm.DB,
	params map[string]interface{},
) (map[string]interface{}, error) {
	if db == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "virtual inventory binding database is unavailable"}
	}

	page := parsePluginHostPositiveInt(params, 1, 1, 10000, "page")
	pageSize := parsePluginHostPositiveInt(params, 20, 1, 100, "page_size", "pageSize", "limit")

	query := db.Model(&models.ProductVirtualInventoryBinding{})

	var productIDValue interface{}
	if productID, ok, err := parsePluginHostOptionalUint(params, "product_id", "productId"); err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	} else if ok {
		query = query.Where("product_id = ?", productID)
		productIDValue = productID
	}

	var virtualInventoryIDValue interface{}
	if virtualInventoryID, ok, err := parsePluginHostOptionalUint(params, "virtual_inventory_id", "virtualInventoryId"); err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	} else if ok {
		query = query.Where("virtual_inventory_id = ?", virtualInventoryID)
		virtualInventoryIDValue = virtualInventoryID
	}

	isRandom, err := parsePluginHostOptionalBool(params, "is_random", "isRandom")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	if isRandom != nil {
		query = query.Where("is_random = ?", *isRandom)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "count virtual inventory bindings failed"}
	}

	var bindings []models.ProductVirtualInventoryBinding
	offset := (page - 1) * pageSize
	if err := query.Preload("Product").
		Preload("VirtualInventory").
		Order("created_at ASC, id ASC").
		Offset(offset).
		Limit(pageSize).
		Find(&bindings).Error; err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query virtual inventory bindings failed"}
	}

	inventoryIDs := make([]uint, 0, len(bindings))
	seenInventoryIDs := make(map[uint]struct{}, len(bindings))
	for _, binding := range bindings {
		if _, exists := seenInventoryIDs[binding.VirtualInventoryID]; exists {
			continue
		}
		seenInventoryIDs[binding.VirtualInventoryID] = struct{}{}
		inventoryIDs = append(inventoryIDs, binding.VirtualInventoryID)
	}
	virtualInventoryService := NewVirtualInventoryService(db)
	statsByInventory, err := virtualInventoryService.getStockStatsForInventories(inventoryIDs)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query virtual inventory stats failed"}
	}

	items := make([]map[string]interface{}, 0, len(bindings))
	for idx := range bindings {
		items = append(items, buildPluginHostVirtualInventoryBindingSummaryResponse(&bindings[idx], statsByInventory[bindings[idx].VirtualInventoryID]))
	}

	return map[string]interface{}{
		"items":                items,
		"page":                 page,
		"page_size":            pageSize,
		"total":                total,
		"has_more":             int64(page*pageSize) < total,
		"product_id":           productIDValue,
		"virtual_inventory_id": virtualInventoryIDValue,
		"is_random":            isRandom != nil && *isRandom,
	}, nil
}

func pluginHostMarshalToMap(value interface{}) map[string]interface{} {
	if value == nil {
		return map[string]interface{}{}
	}
	body, err := json.Marshal(value)
	if err != nil {
		return map[string]interface{}{}
	}
	var out map[string]interface{}
	if err := json.Unmarshal(body, &out); err != nil {
		return map[string]interface{}{}
	}
	return out
}

func pluginHostMarshalToSlice(value interface{}) []map[string]interface{} {
	if value == nil {
		return []map[string]interface{}{}
	}
	body, err := json.Marshal(value)
	if err != nil {
		return []map[string]interface{}{}
	}
	var out []map[string]interface{}
	if err := json.Unmarshal(body, &out); err != nil {
		return []map[string]interface{}{}
	}
	return out
}

func buildPluginHostProductSummaryResponse(product *models.Product) map[string]interface{} {
	resp := pluginHostMarshalToMap(product)
	if product == nil {
		return resp
	}
	resp["primary_image"] = product.GetPrimaryImage()
	resp["is_available"] = product.IsAvailable()
	return resp
}

func buildPluginHostProductDetailResponse(product *models.Product) map[string]interface{} {
	return buildPluginHostProductSummaryResponse(product)
}

func buildPluginHostInventorySummaryResponse(inventory *models.Inventory) map[string]interface{} {
	resp := pluginHostMarshalToMap(inventory)
	if inventory == nil {
		return resp
	}
	resp["remaining_stock"] = inventory.GetRemainingStock()
	resp["available_stock"] = inventory.GetAvailableStock()
	resp["low_stock"] = inventory.IsLowStock()
	return resp
}

func buildPluginHostInventoryDetailResponse(inventory *models.Inventory) map[string]interface{} {
	return buildPluginHostInventorySummaryResponse(inventory)
}

func buildPluginHostInventoryBindingSummaryResponse(binding *models.ProductInventoryBinding) map[string]interface{} {
	if binding == nil {
		return map[string]interface{}{}
	}

	resp := map[string]interface{}{
		"id":              binding.ID,
		"product_id":      binding.ProductID,
		"inventory_id":    binding.InventoryID,
		"attributes":      binding.Attributes,
		"attributes_hash": binding.AttributesHash,
		"is_random":       binding.IsRandom,
		"priority":        binding.Priority,
		"notes":           binding.Notes,
		"created_at":      binding.CreatedAt,
		"updated_at":      binding.UpdatedAt,
	}
	if binding.Product != nil {
		resp["product"] = buildPluginHostProductSummaryResponse(binding.Product)
	}
	if binding.Inventory != nil {
		resp["inventory"] = buildPluginHostInventorySummaryResponse(binding.Inventory)
	}
	return resp
}

func buildPluginHostInventoryBindingDetailResponse(binding *models.ProductInventoryBinding) map[string]interface{} {
	return buildPluginHostInventoryBindingSummaryResponse(binding)
}

func buildPluginHostPromoSummaryResponse(promo *models.PromoCode) map[string]interface{} {
	resp := pluginHostMarshalToMap(promo)
	if promo == nil {
		return resp
	}
	resp["available_quantity"] = promo.GetAvailableQuantity()
	resp["is_available"] = promo.IsAvailable()
	resp["is_expired"] = promo.IsExpired()
	return resp
}

func buildPluginHostPromoDetailResponse(promo *models.PromoCode) map[string]interface{} {
	return buildPluginHostPromoSummaryResponse(promo)
}

func buildPluginHostTicketSummaryResponse(ticket *models.Ticket) map[string]interface{} {
	resp := pluginHostMarshalToMap(ticket)
	if ticket == nil {
		return resp
	}
	resp["is_closed"] = ticket.Status == models.TicketStatusClosed
	resp["is_unassigned"] = ticket.AssignedTo == nil
	return resp
}

func buildPluginHostTicketDetailResponse(ticket *models.Ticket) map[string]interface{} {
	return buildPluginHostTicketSummaryResponse(ticket)
}

func buildPluginHostSerialSummaryResponse(serial *models.ProductSerial) map[string]interface{} {
	resp := pluginHostMarshalToMap(serial)
	if serial == nil {
		return resp
	}
	resp["has_been_viewed"] = serial.ViewCount > 0
	return resp
}

func buildPluginHostSerialDetailResponse(serial *models.ProductSerial) map[string]interface{} {
	return buildPluginHostSerialSummaryResponse(serial)
}

func buildPluginHostAnnouncementSummaryResponse(announcement *models.Announcement) map[string]interface{} {
	resp := pluginHostMarshalToMap(announcement)
	if announcement == nil {
		return resp
	}
	resp["has_content"] = strings.TrimSpace(announcement.Content) != ""
	return resp
}

func buildPluginHostAnnouncementDetailResponse(announcement *models.Announcement) map[string]interface{} {
	return buildPluginHostAnnouncementSummaryResponse(announcement)
}

func buildPluginHostKnowledgeSummaryResponse(article *models.KnowledgeArticle) map[string]interface{} {
	resp := pluginHostMarshalToMap(article)
	if article == nil {
		return resp
	}
	resp["has_category"] = article.CategoryID != nil
	if article.Category != nil {
		resp["category_name"] = article.Category.Name
	}
	return resp
}

func buildPluginHostKnowledgeDetailResponse(article *models.KnowledgeArticle) map[string]interface{} {
	return buildPluginHostKnowledgeSummaryResponse(article)
}

func buildPluginHostPaymentMethodSummaryResponse(method *models.PaymentMethod) map[string]interface{} {
	if method == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":            method.ID,
		"name":          method.Name,
		"description":   method.Description,
		"type":          method.Type,
		"enabled":       method.Enabled,
		"icon":          method.Icon,
		"sort_order":    method.SortOrder,
		"poll_interval": method.PollInterval,
		"is_custom":     method.Type == models.PaymentMethodTypeCustom,
		"has_script":    strings.TrimSpace(method.Script) != "",
		"has_config":    strings.TrimSpace(method.Config) != "",
		"supports_poll": method.PollInterval > 0,
		"created_at":    method.CreatedAt,
		"updated_at":    method.UpdatedAt,
	}
}

func buildPluginHostPaymentMethodDetailResponse(method *models.PaymentMethod) map[string]interface{} {
	return buildPluginHostPaymentMethodSummaryResponse(method)
}

func buildPluginHostVirtualInventorySummaryResponse(inventory *models.VirtualInventory, stats map[string]int64) map[string]interface{} {
	if inventory == nil {
		return map[string]interface{}{}
	}
	if stats == nil {
		stats = map[string]int64{}
	}

	return map[string]interface{}{
		"id":                    inventory.ID,
		"name":                  inventory.Name,
		"sku":                   inventory.SKU,
		"type":                  inventory.Type,
		"description":           inventory.Description,
		"total_limit":           inventory.TotalLimit,
		"is_active":             inventory.IsActive,
		"notes":                 inventory.Notes,
		"total":                 stats["total"],
		"available":             stats["available"],
		"reserved":              stats["reserved"],
		"sold":                  stats["sold"],
		"has_script":            strings.TrimSpace(inventory.Script) != "",
		"has_script_config":     strings.TrimSpace(inventory.ScriptConfig) != "",
		"is_scripted":           inventory.Type == models.VirtualInventoryTypeScript,
		"supports_manual_stock": inventory.Type != models.VirtualInventoryTypeScript,
		"created_at":            inventory.CreatedAt,
		"updated_at":            inventory.UpdatedAt,
	}
}

func buildPluginHostVirtualInventoryDetailResponse(inventory *models.VirtualInventory, stats map[string]int64) map[string]interface{} {
	return buildPluginHostVirtualInventorySummaryResponse(inventory, stats)
}

func buildPluginHostVirtualInventoryBindingSummaryResponse(binding *models.ProductVirtualInventoryBinding, stats map[string]int64) map[string]interface{} {
	if binding == nil {
		return map[string]interface{}{}
	}

	resp := map[string]interface{}{
		"id":                   binding.ID,
		"product_id":           binding.ProductID,
		"virtual_inventory_id": binding.VirtualInventoryID,
		"attributes":           binding.Attributes,
		"attributes_hash":      binding.AttributesHash,
		"is_random":            binding.IsRandom,
		"priority":             binding.Priority,
		"notes":                binding.Notes,
		"created_at":           binding.CreatedAt,
		"updated_at":           binding.UpdatedAt,
	}
	if binding.Product != nil {
		resp["product"] = buildPluginHostProductSummaryResponse(binding.Product)
	}
	if binding.VirtualInventory != nil {
		resp["virtual_inventory"] = buildPluginHostVirtualInventorySummaryResponse(binding.VirtualInventory, stats)
	}
	return resp
}

func buildPluginHostVirtualInventoryBindingDetailResponse(binding *models.ProductVirtualInventoryBinding, stats map[string]int64) map[string]interface{} {
	return buildPluginHostVirtualInventoryBindingSummaryResponse(binding, stats)
}

func buildPluginHostOrderSummaryResponse(order *models.Order, hasPrivacyPermission bool) map[string]interface{} {
	if order == nil {
		return map[string]interface{}{}
	}
	resp := map[string]interface{}{
		"id":                    order.ID,
		"order_no":              order.OrderNo,
		"user_id":               order.UserID,
		"status":                string(order.Status),
		"items":                 order.Items,
		"privacy_protected":     order.PrivacyProtected,
		"privacy_masked":        order.PrivacyProtected && !hasPrivacyPermission,
		"tracking_no":           order.TrackingNo,
		"receiver_name":         order.ReceiverName,
		"phone_code":            order.PhoneCode,
		"receiver_phone":        order.ReceiverPhone,
		"receiver_email":        order.ReceiverEmail,
		"receiver_country":      order.ReceiverCountry,
		"receiver_province":     order.ReceiverProvince,
		"receiver_city":         order.ReceiverCity,
		"receiver_district":     order.ReceiverDistrict,
		"receiver_address":      order.ReceiverAddress,
		"receiver_postcode":     order.ReceiverPostcode,
		"currency":              order.Currency,
		"total_amount_minor":    order.TotalAmount,
		"discount_amount_minor": order.DiscountAmount,
		"source":                order.Source,
		"source_platform":       order.SourcePlatform,
		"external_user_id":      order.ExternalUserID,
		"external_user_name":    order.ExternalUserName,
		"external_order_id":     order.ExternalOrderID,
		"remark":                order.Remark,
		"admin_remark":          order.AdminRemark,
		"assigned_to":           order.AssignedTo,
		"assigned_at":           order.AssignedAt,
		"shipped_at":            order.ShippedAt,
		"completed_at":          order.CompletedAt,
		"created_at":            order.CreatedAt,
		"updated_at":            order.UpdatedAt,
	}
	if order.User != nil {
		resp["user"] = buildPluginHostEmbeddedUserResponse(order.User)
	}
	return resp
}

func buildPluginHostOrderDetailResponse(order *models.Order, hasPrivacyPermission bool) map[string]interface{} {
	resp := buildPluginHostOrderSummaryResponse(order, hasPrivacyPermission)
	if order == nil {
		return resp
	}
	resp["actual_attributes"] = order.ActualAttributes
	resp["form_submitted_at"] = order.FormSubmittedAt
	resp["form_expires_at"] = order.FormExpiresAt
	resp["user_email"] = order.UserEmail
	resp["email_notifications_enabled"] = order.EmailNotificationsEnabled
	resp["promo_code_id"] = order.PromoCodeID
	resp["promo_code"] = order.PromoCodeStr
	resp["completed_by"] = order.CompletedBy
	resp["user_feedback"] = order.UserFeedback
	return resp
}

func buildPluginHostUserSummaryResponse(db *gorm.DB, user *models.User) map[string]interface{} {
	resp := buildPluginHostEmbeddedUserResponse(user)
	if user == nil {
		return resp
	}
	resp["last_login_at"] = user.LastLoginAt
	resp["created_at"] = user.CreatedAt
	resp["updated_at"] = user.UpdatedAt
	if user.IsAdmin() {
		resp["permissions"] = loadPluginHostTargetUserPermissions(db, user)
	}
	return resp
}

func buildPluginHostUserDetailResponse(db *gorm.DB, user *models.User) map[string]interface{} {
	if user == nil {
		return map[string]interface{}{}
	}
	resp := map[string]interface{}{
		"id":                     user.ID,
		"uuid":                   user.UUID,
		"email":                  user.Email,
		"name":                   user.Name,
		"avatar":                 user.Avatar,
		"role":                   user.Role,
		"is_active":              user.IsActive,
		"email_verified":         user.EmailVerified,
		"locale":                 user.Locale,
		"country":                user.Country,
		"email_notify_order":     user.EmailNotifyOrder,
		"email_notify_ticket":    user.EmailNotifyTicket,
		"email_notify_marketing": user.EmailNotifyMarketing,
		"sms_notify_marketing":   user.SMSNotifyMarketing,
		"last_login_ip":          user.LastLoginIP,
		"register_ip":            user.RegisterIP,
		"last_login_at":          user.LastLoginAt,
		"total_spent_minor":      user.TotalSpentMinor,
		"total_order_count":      user.TotalOrderCount,
		"created_at":             user.CreatedAt,
		"updated_at":             user.UpdatedAt,
	}
	if user.Phone != nil {
		resp["phone"] = *user.Phone
	}
	if user.IsAdmin() {
		resp["permissions"] = loadPluginHostTargetUserPermissions(db, user)
	}
	return resp
}

func buildPluginHostEmbeddedUserResponse(user *models.User) map[string]interface{} {
	if user == nil {
		return map[string]interface{}{}
	}
	resp := map[string]interface{}{
		"id":                user.ID,
		"uuid":              user.UUID,
		"email":             user.Email,
		"name":              user.Name,
		"avatar":            user.Avatar,
		"role":              user.Role,
		"is_active":         user.IsActive,
		"email_verified":    user.EmailVerified,
		"locale":            user.Locale,
		"country":           user.Country,
		"total_spent_minor": user.TotalSpentMinor,
		"total_order_count": user.TotalOrderCount,
	}
	if user.Phone != nil {
		resp["phone"] = *user.Phone
	}
	return resp
}

func loadPluginHostTargetUserPermissions(db *gorm.DB, user *models.User) []string {
	if user == nil || !user.IsAdmin() {
		return []string{}
	}
	if user.IsSuperAdmin() {
		var perm models.AdminPermission
		if db != nil && db.Where("user_id = ?", user.ID).First(&perm).Error == nil {
			return middleware.EffectiveAdminPermissions(user.Role, perm.Permissions)
		}
		return middleware.EffectiveAdminPermissions(user.Role, nil)
	}
	if db == nil {
		return []string{}
	}
	var perm models.AdminPermission
	if err := db.Where("user_id = ?", user.ID).First(&perm).Error; err != nil {
		return []string{}
	}
	return middleware.EffectiveAdminPermissions(user.Role, perm.Permissions)
}

func pluginHostClaimsHasPluginPermissions(claims *PluginHostAccessClaims, required ...string) bool {
	if claims == nil {
		return false
	}
	grantedSet := make(map[string]struct{}, len(claims.GrantedPermissions))
	for _, permission := range NormalizePluginPermissionList(claims.GrantedPermissions) {
		grantedSet[permission] = struct{}{}
	}
	for _, permission := range NormalizePluginPermissionList(required) {
		if _, exists := grantedSet[permission]; !exists {
			return false
		}
	}
	return true
}

func pluginHostClaimsHasOperatorPermissions(claims *PluginHostAccessClaims, required ...string) bool {
	normalizedRequired := make([]string, 0, len(required))
	for _, permission := range required {
		normalized := strings.TrimSpace(permission)
		if normalized == "" {
			continue
		}
		normalizedRequired = append(normalizedRequired, normalized)
	}
	if len(normalizedRequired) == 0 {
		return true
	}
	if claims == nil || !claims.ScopeAuthenticated {
		return false
	}
	role := "admin"
	if claims.ScopeSuperAdmin {
		role = "super_admin"
	}
	grantedSet := make(map[string]struct{}, len(normalizedRequired))
	for _, permission := range middleware.EffectiveAdminPermissions(role, claims.ScopePermissions) {
		grantedSet[permission] = struct{}{}
	}
	for _, permission := range normalizedRequired {
		if _, exists := grantedSet[permission]; !exists {
			return false
		}
	}
	return true
}

func pluginHostClaimsCanReadOrderPrivacy(claims *PluginHostAccessClaims) bool {
	return pluginHostClaimsHasPluginPermissions(claims, PluginPermissionHostOrderPrivacy) &&
		pluginHostClaimsHasOperatorPermissions(claims, "order.view_privacy")
}

type pluginHostKnowledgeCategoryCountRow struct {
	CategoryID uint  `gorm:"column:category_id"`
	Cnt        int64 `gorm:"column:cnt"`
}

func pluginHostLoadKnowledgeCategoryTree(db *gorm.DB) ([]models.KnowledgeCategory, error) {
	if db == nil {
		return nil, errors.New("knowledge database is unavailable")
	}

	var flat []models.KnowledgeCategory
	if err := db.Order("sort_order ASC, id ASC").Find(&flat).Error; err != nil {
		return nil, err
	}

	return pluginHostBuildKnowledgeCategoryTree(flat), nil
}

func pluginHostBuildKnowledgeCategoryTree(flat []models.KnowledgeCategory) []models.KnowledgeCategory {
	if len(flat) == 0 {
		return []models.KnowledgeCategory{}
	}

	byID := make(map[uint]models.KnowledgeCategory, len(flat))
	rootIDs := make([]uint, 0, len(flat))
	childIDsByParent := make(map[uint][]uint, len(flat))
	for _, category := range flat {
		cloned := category
		cloned.Children = nil
		byID[cloned.ID] = cloned
		if cloned.ParentID == nil {
			rootIDs = append(rootIDs, cloned.ID)
			continue
		}
		parentID := *cloned.ParentID
		childIDsByParent[parentID] = append(childIDsByParent[parentID], cloned.ID)
	}

	var build func(id uint) models.KnowledgeCategory
	build = func(id uint) models.KnowledgeCategory {
		category := byID[id]
		childIDs := childIDsByParent[id]
		if len(childIDs) == 0 {
			category.Children = []models.KnowledgeCategory{}
			return category
		}

		children := make([]models.KnowledgeCategory, 0, len(childIDs))
		for _, childID := range childIDs {
			children = append(children, build(childID))
		}
		category.Children = children
		return category
	}

	roots := make([]models.KnowledgeCategory, 0, len(rootIDs))
	for _, rootID := range rootIDs {
		roots = append(roots, build(rootID))
	}
	return roots
}

func pluginHostCollectKnowledgeCategoryDescendantIDs(db *gorm.DB, rootID uint) ([]uint, error) {
	categories, err := pluginHostLoadKnowledgeCategoryTree(db)
	if err != nil {
		return nil, err
	}

	ids := []uint{rootID}
	var appendIDs func(category models.KnowledgeCategory)
	appendIDs = func(category models.KnowledgeCategory) {
		for _, child := range category.Children {
			ids = append(ids, child.ID)
			appendIDs(child)
		}
	}

	var find func(list []models.KnowledgeCategory) bool
	find = func(list []models.KnowledgeCategory) bool {
		for _, category := range list {
			if category.ID == rootID {
				appendIDs(category)
				return true
			}
			if find(category.Children) {
				return true
			}
		}
		return false
	}
	find(categories)
	return ids, nil
}

func pluginHostPopulateKnowledgeCategoryArticleCounts(db *gorm.DB, categories []models.KnowledgeCategory) {
	if db == nil || len(categories) == 0 {
		return
	}

	ids := make([]uint, 0, len(categories))
	var collect func(list []models.KnowledgeCategory)
	collect = func(list []models.KnowledgeCategory) {
		for _, category := range list {
			ids = append(ids, category.ID)
			if len(category.Children) > 0 {
				collect(category.Children)
			}
		}
	}
	collect(categories)
	if len(ids) == 0 {
		return
	}

	var rows []pluginHostKnowledgeCategoryCountRow
	db.Model(&models.KnowledgeArticle{}).
		Select("category_id, COUNT(*) as cnt").
		Where("category_id IN ?", ids).
		Group("category_id").
		Scan(&rows)

	counts := make(map[uint]int64, len(rows))
	for _, row := range rows {
		counts[row.CategoryID] = row.Cnt
	}

	var apply func(category *models.KnowledgeCategory) int64
	apply = func(category *models.KnowledgeCategory) int64 {
		if category == nil {
			return 0
		}
		direct := counts[category.ID]
		category.ArticleCount = direct
		total := direct
		for idx := range category.Children {
			total += apply(&category.Children[idx])
		}
		category.TotalArticleCount = total
		return total
	}

	for idx := range categories {
		apply(&categories[idx])
	}
}

func pluginHostCountKnowledgeCategories(categories []models.KnowledgeCategory) int {
	total := 0
	var visit func(list []models.KnowledgeCategory)
	visit = func(list []models.KnowledgeCategory) {
		for _, category := range list {
			total++
			if len(category.Children) > 0 {
				visit(category.Children)
			}
		}
	}
	visit(categories)
	return total
}

func clonePluginHostOrder(order *models.Order) models.Order {
	if order == nil {
		return models.Order{}
	}
	cloned := *order
	if order.Items != nil {
		cloned.Items = append([]models.OrderItem{}, order.Items...)
	}
	if order.User != nil {
		userCopy := *order.User
		cloned.User = &userCopy
	}
	if order.UserID != nil {
		userID := *order.UserID
		cloned.UserID = &userID
	}
	if order.PromoCodeID != nil {
		promoCodeID := *order.PromoCodeID
		cloned.PromoCodeID = &promoCodeID
	}
	if order.AssignedTo != nil {
		assignedTo := *order.AssignedTo
		cloned.AssignedTo = &assignedTo
	}
	if order.AssignedAt != nil {
		assignedAt := *order.AssignedAt
		cloned.AssignedAt = &assignedAt
	}
	if order.ShippedAt != nil {
		shippedAt := *order.ShippedAt
		cloned.ShippedAt = &shippedAt
	}
	if order.CompletedAt != nil {
		completedAt := *order.CompletedAt
		cloned.CompletedAt = &completedAt
	}
	if order.CompletedBy != nil {
		completedBy := *order.CompletedBy
		cloned.CompletedBy = &completedBy
	}
	if order.FormToken != nil {
		formToken := *order.FormToken
		cloned.FormToken = &formToken
	}
	if order.FormSubmittedAt != nil {
		formSubmittedAt := *order.FormSubmittedAt
		cloned.FormSubmittedAt = &formSubmittedAt
	}
	if order.FormExpiresAt != nil {
		formExpiresAt := *order.FormExpiresAt
		cloned.FormExpiresAt = &formExpiresAt
	}
	return cloned
}

func executePluginHostWorkspaceAppend(
	runtime *PluginHostRuntime,
	claims *PluginHostAccessClaims,
	params map[string]interface{},
) (map[string]interface{}, error) {
	if runtime == nil || runtime.PluginManager == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "plugin workspace manager is unavailable"}
	}
	if claims == nil || claims.PluginID == 0 {
		return nil, &PluginHostActionError{Status: http.StatusUnauthorized, Message: "plugin workspace context is unavailable"}
	}

	commandID := parsePluginHostOptionalString(params, "command_id", "commandId")
	if commandID == "" {
		commandID = strings.TrimSpace(claims.ExecutionID)
	}
	if commandID == "" {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "command_id is required"}
	}
	if strings.TrimSpace(claims.ExecutionID) != "" && strings.TrimSpace(claims.ExecutionID) != commandID {
		return nil, &PluginHostActionError{Status: http.StatusForbidden, Message: "plugin workspace command context mismatch"}
	}

	cleared := false
	if parsed, err := parsePluginHostOptionalBool(params, "clear", "cleared"); err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	} else if parsed != nil {
		cleared = *parsed
	}
	entries := decodePluginHostWorkspaceEntries(params["entries"])
	if !cleared && len(entries) == 0 {
		return map[string]interface{}{
			"appended": 0,
			"cleared":  false,
		}, nil
	}
	if err := runtime.PluginManager.AppendPluginWorkspaceSessionEntries(claims.PluginID, commandID, entries, cleared); err != nil {
		return nil, &PluginHostActionError{Status: http.StatusConflict, Message: err.Error()}
	}
	return map[string]interface{}{
		"appended": len(entries),
		"cleared":  cleared,
	}, nil
}

func executePluginHostWorkspaceReadInput(
	runtime *PluginHostRuntime,
	claims *PluginHostAccessClaims,
	params map[string]interface{},
) (map[string]interface{}, error) {
	if runtime == nil || runtime.PluginManager == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "plugin workspace manager is unavailable"}
	}
	if claims == nil || claims.PluginID == 0 {
		return nil, &PluginHostActionError{Status: http.StatusUnauthorized, Message: "plugin workspace context is unavailable"}
	}

	commandID := parsePluginHostOptionalString(params, "command_id", "commandId")
	if commandID == "" {
		commandID = strings.TrimSpace(claims.ExecutionID)
	}
	if commandID == "" {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "command_id is required"}
	}
	if strings.TrimSpace(claims.ExecutionID) != "" && strings.TrimSpace(claims.ExecutionID) != commandID {
		return nil, &PluginHostActionError{Status: http.StatusForbidden, Message: "plugin workspace command context mismatch"}
	}

	prompt := parsePluginHostOptionalString(params, "prompt")
	timeoutMs := parsePluginHostPositiveInt(
		params,
		int(defaultPluginWorkspaceInputWaitTimeout/time.Millisecond),
		1,
		int(maxPluginHostBridgeConnTimeout/time.Millisecond),
		"timeout_ms",
		"timeoutMs",
	)
	value, err := runtime.PluginManager.WaitPluginWorkspaceInput(
		claims.PluginID,
		commandID,
		prompt,
		time.Duration(timeoutMs)*time.Millisecond,
	)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusConflict, Message: err.Error()}
	}
	return map[string]interface{}{
		"value": value,
	}, nil
}

func decodePluginHostWorkspaceEntries(value interface{}) []pluginipc.WorkspaceBufferEntry {
	if value == nil {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var entries []pluginipc.WorkspaceBufferEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil
	}
	return entries
}

func parsePluginHostOptionalString(params map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		value, exists := params[key]
		if !exists || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			trimmed := strings.TrimSpace(typed)
			if trimmed != "" {
				return trimmed
			}
		case fmt.Stringer:
			trimmed := strings.TrimSpace(typed.String())
			if trimmed != "" {
				return trimmed
			}
		default:
			trimmed := strings.TrimSpace(fmt.Sprintf("%v", typed))
			if trimmed != "" && trimmed != "<nil>" {
				return trimmed
			}
		}
	}
	return ""
}

func parsePluginHostOptionalUint(params map[string]interface{}, keys ...string) (uint, bool, error) {
	for _, key := range keys {
		value, exists := params[key]
		if !exists || value == nil {
			continue
		}
		switch typed := value.(type) {
		case uint:
			return typed, true, nil
		case uint64:
			return uint(typed), true, nil
		case int:
			if typed < 0 {
				return 0, false, fmt.Errorf("%s must be unsigned integer", key)
			}
			return uint(typed), true, nil
		case int64:
			if typed < 0 {
				return 0, false, fmt.Errorf("%s must be unsigned integer", key)
			}
			return uint(typed), true, nil
		case float64:
			if typed < 0 || typed != float64(uint64(typed)) {
				return 0, false, fmt.Errorf("%s must be unsigned integer", key)
			}
			return uint(typed), true, nil
		case string:
			trimmed := strings.TrimSpace(typed)
			if trimmed == "" {
				continue
			}
			parsed, err := strconv.ParseUint(trimmed, 10, 64)
			if err != nil {
				return 0, false, fmt.Errorf("%s must be unsigned integer", key)
			}
			return uint(parsed), true, nil
		default:
			return 0, false, fmt.Errorf("%s must be unsigned integer", key)
		}
	}
	return 0, false, nil
}

func parsePluginHostOptionalInt64(params map[string]interface{}, keys ...string) (int64, bool, error) {
	for _, key := range keys {
		value, exists := params[key]
		if !exists || value == nil {
			continue
		}
		switch typed := value.(type) {
		case int:
			return int64(typed), true, nil
		case int64:
			return typed, true, nil
		case uint:
			return int64(typed), true, nil
		case uint64:
			if typed > uint64(^uint64(0)>>1) {
				return 0, false, fmt.Errorf("%s must be signed integer", key)
			}
			return int64(typed), true, nil
		case float64:
			if typed != float64(int64(typed)) {
				return 0, false, fmt.Errorf("%s must be signed integer", key)
			}
			return int64(typed), true, nil
		case string:
			trimmed := strings.TrimSpace(typed)
			if trimmed == "" {
				continue
			}
			parsed, err := strconv.ParseInt(trimmed, 10, 64)
			if err != nil {
				return 0, false, fmt.Errorf("%s must be signed integer", key)
			}
			return parsed, true, nil
		default:
			return 0, false, fmt.Errorf("%s must be signed integer", key)
		}
	}
	return 0, false, nil
}

func parsePluginHostOptionalBool(params map[string]interface{}, keys ...string) (*bool, error) {
	for _, key := range keys {
		value, exists := params[key]
		if !exists || value == nil {
			continue
		}
		switch typed := value.(type) {
		case bool:
			valueCopy := typed
			return &valueCopy, nil
		case string:
			normalized := strings.ToLower(strings.TrimSpace(typed))
			switch normalized {
			case "":
				continue
			case "1", "true", "yes", "y":
				valueCopy := true
				return &valueCopy, nil
			case "0", "false", "no", "n":
				valueCopy := false
				return &valueCopy, nil
			default:
				return nil, fmt.Errorf("%s must be boolean", key)
			}
		default:
			return nil, fmt.Errorf("%s must be boolean", key)
		}
	}
	return nil, nil
}

func parsePluginHostPositiveInt(params map[string]interface{}, defaultValue int, min int, max int, keys ...string) int {
	if defaultValue < min {
		defaultValue = min
	}
	if defaultValue > max {
		defaultValue = max
	}
	for _, key := range keys {
		value, exists := params[key]
		if !exists || value == nil {
			continue
		}
		parsed := defaultValue
		switch typed := value.(type) {
		case int:
			parsed = typed
		case int64:
			parsed = int(typed)
		case float64:
			parsed = int(typed)
		case string:
			trimmed := strings.TrimSpace(typed)
			if trimmed == "" {
				continue
			}
			if converted, err := strconv.Atoi(trimmed); err == nil {
				parsed = converted
			}
		}
		if parsed < min {
			parsed = min
		}
		if parsed > max {
			parsed = max
		}
		return parsed
	}
	return defaultValue
}

func derefPluginHostUint(value *uint) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

func truncateString(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if maxLen <= 0 || len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
