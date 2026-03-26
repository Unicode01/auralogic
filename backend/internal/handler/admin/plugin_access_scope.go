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

type pluginAccessScope struct {
	authenticated bool
	superAdmin    bool
	permissions   map[string]struct{}
}

func buildPermissionSet(values []string) map[string]struct{} {
	if len(values) == 0 {
		return map[string]struct{}{}
	}
	set := make(map[string]struct{}, len(values))
	for _, value := range normalizeLowerStringListValues(values) {
		set[value] = struct{}{}
	}
	return set
}

func hasAllPermissions(permissions map[string]struct{}, required []string) bool {
	if len(required) == 0 {
		return true
	}
	if len(permissions) == 0 {
		return false
	}
	for _, item := range required {
		key := strings.ToLower(strings.TrimSpace(item))
		if key == "" {
			continue
		}
		if _, ok := permissions[key]; !ok {
			return false
		}
	}
	return true
}

func (s pluginAccessScope) canAccess(requiredPermissions []string, superAdminOnly bool) bool {
	if superAdminOnly && !s.superAdmin {
		return false
	}
	if s.superAdmin {
		return true
	}
	return hasAllPermissions(s.permissions, requiredPermissions)
}

func (h *PluginHandler) resolvePluginAccessScope(c *gin.Context) pluginAccessScope {
	scope := pluginAccessScope{
		authenticated: false,
		superAdmin:    false,
		permissions:   map[string]struct{}{},
	}
	if c == nil {
		return scope
	}

	if middleware.IsAPIKeyAuth(c) {
		scope.authenticated = true
		if rawScopes, exists := c.Get("api_scopes"); exists {
			switch typed := rawScopes.(type) {
			case []string:
				scope.permissions = buildPermissionSet(typed)
			case []interface{}:
				values := make([]string, 0, len(typed))
				for _, item := range typed {
					values = append(values, parseStringFromAny(item))
				}
				scope.permissions = buildPermissionSet(values)
			}
		}
		return scope
	}

	role, roleExists := middleware.GetUserRole(c)
	normalizedRole := strings.ToLower(strings.TrimSpace(role))
	if roleExists && normalizedRole != "" {
		scope.authenticated = true
		scope.superAdmin = normalizedRole == "super_admin"
	}

	if scope.superAdmin {
		return scope
	}

	userID, hasUser := middleware.GetUserID(c)
	if !hasUser {
		return scope
	}
	scope.authenticated = true
	if h == nil || h.db == nil {
		return scope
	}

	var perm models.AdminPermission
	if err := h.db.Where("user_id = ?", userID).First(&perm).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return scope
		}
		return scope
	}
	scope.permissions = buildPermissionSet(perm.Permissions)
	return scope
}

func filterFrontendBootstrapByScope(
	area string,
	menus []frontendBootstrapMenuItem,
	routes []frontendBootstrapRouteItem,
	scope pluginAccessScope,
) ([]frontendBootstrapMenuItem, []frontendBootstrapRouteItem) {
	normalizedArea := normalizeFrontendBootstrapArea(area)

	filteredMenus := make([]frontendBootstrapMenuItem, 0, len(menus))
	for _, item := range menus {
		if normalizedArea == frontendBootstrapAreaUser && !scope.authenticated && !item.GuestVisible {
			continue
		}
		if !scope.canAccess(item.RequiredPermissions, item.SuperAdminOnly) {
			continue
		}
		filteredMenus = append(filteredMenus, item)
	}

	filteredRoutes := make([]frontendBootstrapRouteItem, 0, len(routes))
	for _, item := range routes {
		if normalizedArea == frontendBootstrapAreaUser && !scope.authenticated && !item.GuestVisible {
			continue
		}
		if !scope.canAccess(item.RequiredPermissions, item.SuperAdminOnly) {
			continue
		}
		filteredRoutes = append(filteredRoutes, item)
	}
	return filteredMenus, filteredRoutes
}

func filterFrontendExtensionsByScope(
	extensions []service.FrontendExtension,
	requestSlot string,
	scope pluginAccessScope,
	publicEndpoint bool,
) []service.FrontendExtension {
	if len(extensions) == 0 {
		return []service.FrontendExtension{}
	}

	reqSlot := normalizedSlotValue(requestSlot)
	filtered := make([]service.FrontendExtension, 0, len(extensions))
	for _, item := range extensions {
		extSlot := normalizedSlotValue(item.Slot)
		if extSlot == "" {
			extSlot = reqSlot
		}
		if publicEndpoint && strings.HasPrefix(extSlot, "admin.") {
			continue
		}

		data := item.Data
		required := parseStringListFromAny(data["required_permissions"])
		superAdminOnly := parseBoolFromAny(data["super_admin_only"], false)
		guestVisible := parseBoolFromAny(data["guest_visible"], false)

		if !scope.authenticated && !guestVisible {
			continue
		}
		if !scope.canAccess(required, superAdminOnly) {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}
