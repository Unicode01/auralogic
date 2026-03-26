package middleware

import "strings"

type AdminPermissionGroup struct {
	Name        string
	Permissions []string
}

var adminPermissionGroups = []AdminPermissionGroup{
	{
		Name: "OrderPermission",
		Permissions: []string{
			"order.view",
			"order.view_privacy",
			"order.edit",
			"order.delete",
			"order.status_update",
			"order.refund",
			"order.assign_tracking",
			"order.request_resubmit",
		},
	},
	{
		Name: "ProductPermission",
		Permissions: []string{
			"product.view",
			"product.edit",
			"product.delete",
		},
	},
	{
		Name: "SerialPermission",
		Permissions: []string{
			"serial.view",
			"serial.manage",
		},
	},
	{
		Name: "UserPermission",
		Permissions: []string{
			"user.view",
			"user.edit",
			"user.permission",
		},
	},
	{
		Name: "AdminPermission",
		Permissions: []string{
			"admin.create",
			"admin.edit",
			"admin.delete",
			"admin.permission",
		},
	},
	{
		Name: "SystemPermission",
		Permissions: []string{
			"system.config",
			"system.logs",
			"api.manage",
		},
	},
	{
		Name: "PaymentMethodPermission",
		Permissions: []string{
			"payment_method.view",
			"payment_method.edit",
		},
	},
	{
		Name: "MarketPermission",
		Permissions: []string{
			"market.view",
			"market.install",
			"market.history",
			"market.review",
			"market.manage",
		},
	},
	{
		Name: "PluginPermission",
		Permissions: []string{
			"plugin.view",
			"plugin.edit",
			"plugin.execute",
			"plugin.lifecycle",
			"plugin.diagnostics",
			"plugin.upload",
		},
	},
	{
		Name: "KnowledgePermission",
		Permissions: []string{
			"knowledge.view",
			"knowledge.edit",
		},
	},
	{
		Name: "AnnouncementPermission",
		Permissions: []string{
			"announcement.view",
			"announcement.edit",
		},
	},
	{
		Name: "MarketingPermission",
		Permissions: []string{
			"marketing.view",
			"marketing.send",
		},
	},
	{
		Name: "TicketPermission",
		Permissions: []string{
			"ticket.view",
			"ticket.reply",
			"ticket.status_update",
		},
	},
	{
		Name: "EmailTemplatePermission",
		Permissions: []string{
			"email_template.view",
			"email_template.edit",
		},
	},
	{
		Name: "LandingPagePermission",
		Permissions: []string{
			"landing_page.view",
			"landing_page.edit",
		},
	},
	{
		Name: "InvoiceTemplatePermission",
		Permissions: []string{
			"invoice_template.view",
			"invoice_template.edit",
		},
	},
	{
		Name: "AuthBrandingPermission",
		Permissions: []string{
			"auth_branding.view",
			"auth_branding.edit",
		},
	},
	{
		Name: "PageRulePackPermission",
		Permissions: []string{
			"page_rule_pack.view",
			"page_rule_pack.edit",
		},
	},
}

func RegisteredAdminPermissionGroups() []AdminPermissionGroup {
	groups := make([]AdminPermissionGroup, 0, len(adminPermissionGroups))
	for _, group := range adminPermissionGroups {
		groups = append(groups, AdminPermissionGroup{
			Name:        group.Name,
			Permissions: clonePermissionList(group.Permissions),
		})
	}
	return groups
}

func RegisteredAdminPermissionsMap() map[string][]string {
	result := make(map[string][]string, len(adminPermissionGroups))
	for _, group := range adminPermissionGroups {
		result[group.Name] = clonePermissionList(group.Permissions)
	}
	return result
}

func RegisteredAdminPermissions() []string {
	all := make([]string, 0, 32)
	for _, group := range adminPermissionGroups {
		all = append(all, group.Permissions...)
	}
	return uniqueNormalizedPermissions(all)
}

func DefaultAdminPermissionsForRole(role string) []string {
	if strings.TrimSpace(role) != "super_admin" {
		return []string{}
	}

	defaults := make([]string, 0, 32)
	for _, permission := range RegisteredAdminPermissions() {
		if IsSpecialPermission(permission) {
			continue
		}
		defaults = append(defaults, permission)
	}
	return defaults
}

func EffectiveAdminPermissions(role string, explicit []string) []string {
	if strings.TrimSpace(role) == "super_admin" {
		return mergePermissionLists(DefaultAdminPermissionsForRole(role), explicit)
	}
	return uniqueNormalizedPermissions(explicit)
}

func clonePermissionList(permissions []string) []string {
	if len(permissions) == 0 {
		return []string{}
	}
	cloned := make([]string, 0, len(permissions))
	cloned = append(cloned, permissions...)
	return cloned
}

func mergePermissionLists(base []string, extras []string) []string {
	merged := make([]string, 0, len(base)+len(extras))
	merged = append(merged, base...)
	merged = append(merged, extras...)
	return uniqueNormalizedPermissions(merged)
}

func uniqueNormalizedPermissions(permissions []string) []string {
	if len(permissions) == 0 {
		return []string{}
	}

	result := make([]string, 0, len(permissions))
	seen := make(map[string]struct{}, len(permissions))
	for _, permission := range permissions {
		normalized := strings.TrimSpace(permission)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}
