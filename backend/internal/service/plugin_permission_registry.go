package service

import (
	"fmt"
	"sort"
	"strings"
)

const (
	PluginPermissionHookExecute                     = "hook.execute"
	PluginPermissionHookPayloadPatch                = "hook.payload_patch"
	PluginPermissionHookBlock                       = "hook.block"
	PluginPermissionFrontendExtension               = "frontend.extensions"
	PluginPermissionFrontendHTMLTrust               = "frontend.html_trusted"
	PluginPermissionExecuteAPI                      = "api.execute"
	PluginPermissionRuntimeNetwork                  = "runtime.network"
	PluginPermissionRuntimeFileSystem               = "runtime.file_system"
	PluginPermissionHostOrderRead                   = "host.order.read"
	PluginPermissionHostOrderPrivacy                = "host.order.read_privacy"
	PluginPermissionHostOrderList                   = "host.order.list"
	PluginPermissionHostOrderAssignTracking         = "host.order.assign_tracking"
	PluginPermissionHostOrderRequestResubmit        = "host.order.request_resubmit"
	PluginPermissionHostOrderMarkPaid               = "host.order.mark_paid"
	PluginPermissionHostOrderUpdatePrice            = "host.order.update_price"
	PluginPermissionHostUserRead                    = "host.user.read"
	PluginPermissionHostUserList                    = "host.user.list"
	PluginPermissionHostProductRead                 = "host.product.read"
	PluginPermissionHostProductList                 = "host.product.list"
	PluginPermissionHostInventoryRead               = "host.inventory.read"
	PluginPermissionHostInventoryList               = "host.inventory.list"
	PluginPermissionHostInventoryBindingRead        = "host.inventory_binding.read"
	PluginPermissionHostInventoryBindingList        = "host.inventory_binding.list"
	PluginPermissionHostPromoRead                   = "host.promo.read"
	PluginPermissionHostPromoList                   = "host.promo.list"
	PluginPermissionHostTicketRead                  = "host.ticket.read"
	PluginPermissionHostTicketList                  = "host.ticket.list"
	PluginPermissionHostTicketReply                 = "host.ticket.reply"
	PluginPermissionHostTicketUpdate                = "host.ticket.update"
	PluginPermissionHostSerialRead                  = "host.serial.read"
	PluginPermissionHostSerialList                  = "host.serial.list"
	PluginPermissionHostAnnouncementRead            = "host.announcement.read"
	PluginPermissionHostAnnouncementList            = "host.announcement.list"
	PluginPermissionHostKnowledgeRead               = "host.knowledge.read"
	PluginPermissionHostKnowledgeList               = "host.knowledge.list"
	PluginPermissionHostKnowledgeCategories         = "host.knowledge.categories"
	PluginPermissionHostPaymentMethodRead           = "host.payment_method.read"
	PluginPermissionHostPaymentMethodList           = "host.payment_method.list"
	PluginPermissionHostVirtualInventoryRead        = "host.virtual_inventory.read"
	PluginPermissionHostVirtualInventoryList        = "host.virtual_inventory.list"
	PluginPermissionHostVirtualInventoryBindingRead = "host.virtual_inventory_binding.read"
	PluginPermissionHostVirtualInventoryBindingList = "host.virtual_inventory_binding.list"
	PluginPermissionHostMarketSourceRead            = "host.market.source.read"
	PluginPermissionHostMarketCatalogRead           = "host.market.catalog.read"
	PluginPermissionHostMarketInstallPreview        = "host.market.install.preview"
	PluginPermissionHostMarketInstallExecute        = "host.market.install.execute"
	PluginPermissionHostMarketInstallRead           = "host.market.install.read"
	PluginPermissionHostMarketInstallRollback       = "host.market.install.rollback"
	PluginPermissionHostEmailTemplateRead           = "host.email_template.read"
	PluginPermissionHostEmailTemplateWrite          = "host.email_template.write"
	PluginPermissionHostLandingPageRead             = "host.landing_page.read"
	PluginPermissionHostLandingPageWrite            = "host.landing_page.write"
	PluginPermissionHostInvoiceTemplateRead         = "host.invoice_template.read"
	PluginPermissionHostInvoiceTemplateWrite        = "host.invoice_template.write"
	PluginPermissionHostAuthBrandingRead            = "host.auth_branding.read"
	PluginPermissionHostAuthBrandingWrite           = "host.auth_branding.write"
	PluginPermissionHostPageRulePackRead            = "host.page_rule_pack.read"
	PluginPermissionHostPageRulePackWrite           = "host.page_rule_pack.write"
	PluginPermissionHostPluginPageRuleRead          = "host.plugin_page_rule.read"
	PluginPermissionHostPluginPageRuleWrite         = "host.plugin_page_rule.write"
)

type PluginPermissionDefinition struct {
	Key            string
	Title          string
	Description    string
	DefaultGranted bool
}

type PluginPermissionRequest struct {
	Key            string `json:"key"`
	Required       bool   `json:"required"`
	Reason         string `json:"reason,omitempty"`
	Title          string `json:"title,omitempty"`
	Description    string `json:"description,omitempty"`
	DefaultGranted bool   `json:"default_granted"`
}

var pluginPermissionDefinitions = map[string]PluginPermissionDefinition{
	PluginPermissionHookExecute: {
		Key:            PluginPermissionHookExecute,
		Title:          "Hook Execute",
		Description:    "Allow the plugin to run in hook pipeline.",
		DefaultGranted: false,
	},
	PluginPermissionHookPayloadPatch: {
		Key:            PluginPermissionHookPayloadPatch,
		Title:          "Hook Payload Patch",
		Description:    "Allow the plugin to modify hook payload fields.",
		DefaultGranted: false,
	},
	PluginPermissionHookBlock: {
		Key:            PluginPermissionHookBlock,
		Title:          "Hook Block",
		Description:    "Allow the plugin to block workflow execution.",
		DefaultGranted: false,
	},
	PluginPermissionFrontendExtension: {
		Key:            PluginPermissionFrontendExtension,
		Title:          "Frontend Extension",
		Description:    "Allow the plugin to inject frontend extension blocks.",
		DefaultGranted: false,
	},
	PluginPermissionFrontendHTMLTrust: {
		Key:            PluginPermissionFrontendHTMLTrust,
		Title:          "Frontend Trusted HTML",
		Description:    "Allow the plugin to render trusted frontend HTML without sanitize fallback.",
		DefaultGranted: false,
	},
	PluginPermissionExecuteAPI: {
		Key:            PluginPermissionExecuteAPI,
		Title:          "Admin Execute API",
		Description:    "Allow the plugin to be invoked from admin execute API.",
		DefaultGranted: false,
	},
	PluginPermissionRuntimeNetwork: {
		Key:            PluginPermissionRuntimeNetwork,
		Title:          "Runtime Network Access",
		Description:    "Allow the plugin runtime sandbox to access network APIs.",
		DefaultGranted: false,
	},
	PluginPermissionRuntimeFileSystem: {
		Key:            PluginPermissionRuntimeFileSystem,
		Title:          "Runtime File-system Access",
		Description:    "Allow the plugin runtime sandbox to access file-system APIs.",
		DefaultGranted: false,
	},
	PluginPermissionHostOrderRead: {
		Key:            PluginPermissionHostOrderRead,
		Title:          "Host Order Read",
		Description:    "Allow the plugin to read a single native order through Plugin.order.get().",
		DefaultGranted: false,
	},
	PluginPermissionHostOrderPrivacy: {
		Key:            PluginPermissionHostOrderPrivacy,
		Title:          "Host Order Privacy Read",
		Description:    "Allow the plugin to read privacy-protected order receiver details when the operator also has order.view_privacy.",
		DefaultGranted: false,
	},
	PluginPermissionHostOrderList: {
		Key:            PluginPermissionHostOrderList,
		Title:          "Host Order List",
		Description:    "Allow the plugin to query native orders through Plugin.order.list().",
		DefaultGranted: false,
	},
	PluginPermissionHostOrderAssignTracking: {
		Key:            PluginPermissionHostOrderAssignTracking,
		Title:          "Host Order Assign Tracking",
		Description:    "Allow the plugin to assign native order tracking number through Plugin.order.assignTracking().",
		DefaultGranted: false,
	},
	PluginPermissionHostOrderRequestResubmit: {
		Key:            PluginPermissionHostOrderRequestResubmit,
		Title:          "Host Order Request Resubmit",
		Description:    "Allow the plugin to request native order shipping-info resubmission through Plugin.order.requestResubmit().",
		DefaultGranted: false,
	},
	PluginPermissionHostOrderMarkPaid: {
		Key:            PluginPermissionHostOrderMarkPaid,
		Title:          "Host Order Mark Paid",
		Description:    "Allow the plugin to mark native orders as paid through Plugin.order.markPaid().",
		DefaultGranted: false,
	},
	PluginPermissionHostOrderUpdatePrice: {
		Key:            PluginPermissionHostOrderUpdatePrice,
		Title:          "Host Order Update Price",
		Description:    "Allow the plugin to update native order price through Plugin.order.updatePrice().",
		DefaultGranted: false,
	},
	PluginPermissionHostUserRead: {
		Key:            PluginPermissionHostUserRead,
		Title:          "Host User Read",
		Description:    "Allow the plugin to read a single native user through Plugin.user.get().",
		DefaultGranted: false,
	},
	PluginPermissionHostUserList: {
		Key:            PluginPermissionHostUserList,
		Title:          "Host User List",
		Description:    "Allow the plugin to query native users through Plugin.user.list().",
		DefaultGranted: false,
	},
	PluginPermissionHostProductRead: {
		Key:            PluginPermissionHostProductRead,
		Title:          "Host Product Read",
		Description:    "Allow the plugin to read a single native product through Plugin.product.get().",
		DefaultGranted: false,
	},
	PluginPermissionHostProductList: {
		Key:            PluginPermissionHostProductList,
		Title:          "Host Product List",
		Description:    "Allow the plugin to query native products through Plugin.product.list().",
		DefaultGranted: false,
	},
	PluginPermissionHostInventoryRead: {
		Key:            PluginPermissionHostInventoryRead,
		Title:          "Host Inventory Read",
		Description:    "Allow the plugin to read a single native inventory record through Plugin.inventory.get().",
		DefaultGranted: false,
	},
	PluginPermissionHostInventoryList: {
		Key:            PluginPermissionHostInventoryList,
		Title:          "Host Inventory List",
		Description:    "Allow the plugin to query native inventory records through Plugin.inventory.list().",
		DefaultGranted: false,
	},
	PluginPermissionHostInventoryBindingRead: {
		Key:            PluginPermissionHostInventoryBindingRead,
		Title:          "Host Inventory Binding Read",
		Description:    "Allow the plugin to read a single native product-inventory binding through Plugin.inventoryBinding.get().",
		DefaultGranted: false,
	},
	PluginPermissionHostInventoryBindingList: {
		Key:            PluginPermissionHostInventoryBindingList,
		Title:          "Host Inventory Binding List",
		Description:    "Allow the plugin to query native product-inventory bindings through Plugin.inventoryBinding.list().",
		DefaultGranted: false,
	},
	PluginPermissionHostPromoRead: {
		Key:            PluginPermissionHostPromoRead,
		Title:          "Host Promo Read",
		Description:    "Allow the plugin to read a single native promo code through Plugin.promo.get().",
		DefaultGranted: false,
	},
	PluginPermissionHostPromoList: {
		Key:            PluginPermissionHostPromoList,
		Title:          "Host Promo List",
		Description:    "Allow the plugin to query native promo codes through Plugin.promo.list().",
		DefaultGranted: false,
	},
	PluginPermissionHostTicketRead: {
		Key:            PluginPermissionHostTicketRead,
		Title:          "Host Ticket Read",
		Description:    "Allow the plugin to read a single native ticket through Plugin.ticket.get().",
		DefaultGranted: false,
	},
	PluginPermissionHostTicketList: {
		Key:            PluginPermissionHostTicketList,
		Title:          "Host Ticket List",
		Description:    "Allow the plugin to query native tickets through Plugin.ticket.list().",
		DefaultGranted: false,
	},
	PluginPermissionHostTicketReply: {
		Key:            PluginPermissionHostTicketReply,
		Title:          "Host Ticket Reply",
		Description:    "Allow the plugin to reply to native tickets through Plugin.ticket.reply().",
		DefaultGranted: false,
	},
	PluginPermissionHostTicketUpdate: {
		Key:            PluginPermissionHostTicketUpdate,
		Title:          "Host Ticket Update",
		Description:    "Allow the plugin to update native ticket status, priority, and assignee through Plugin.ticket.update().",
		DefaultGranted: false,
	},
	PluginPermissionHostSerialRead: {
		Key:            PluginPermissionHostSerialRead,
		Title:          "Host Serial Read",
		Description:    "Allow the plugin to read a single native serial through Plugin.serial.get().",
		DefaultGranted: false,
	},
	PluginPermissionHostSerialList: {
		Key:            PluginPermissionHostSerialList,
		Title:          "Host Serial List",
		Description:    "Allow the plugin to query native serial records through Plugin.serial.list().",
		DefaultGranted: false,
	},
	PluginPermissionHostAnnouncementRead: {
		Key:            PluginPermissionHostAnnouncementRead,
		Title:          "Host Announcement Read",
		Description:    "Allow the plugin to read a single native announcement through Plugin.announcement.get().",
		DefaultGranted: false,
	},
	PluginPermissionHostAnnouncementList: {
		Key:            PluginPermissionHostAnnouncementList,
		Title:          "Host Announcement List",
		Description:    "Allow the plugin to query native announcements through Plugin.announcement.list().",
		DefaultGranted: false,
	},
	PluginPermissionHostKnowledgeRead: {
		Key:            PluginPermissionHostKnowledgeRead,
		Title:          "Host Knowledge Read",
		Description:    "Allow the plugin to read a single native knowledge article through Plugin.knowledge.get().",
		DefaultGranted: false,
	},
	PluginPermissionHostKnowledgeList: {
		Key:            PluginPermissionHostKnowledgeList,
		Title:          "Host Knowledge List",
		Description:    "Allow the plugin to query native knowledge articles through Plugin.knowledge.list().",
		DefaultGranted: false,
	},
	PluginPermissionHostKnowledgeCategories: {
		Key:            PluginPermissionHostKnowledgeCategories,
		Title:          "Host Knowledge Categories",
		Description:    "Allow the plugin to query native knowledge category tree through Plugin.knowledge.categories().",
		DefaultGranted: false,
	},
	PluginPermissionHostPaymentMethodRead: {
		Key:            PluginPermissionHostPaymentMethodRead,
		Title:          "Host Payment Method Read",
		Description:    "Allow the plugin to read a single native payment method summary through Plugin.paymentMethod.get().",
		DefaultGranted: false,
	},
	PluginPermissionHostPaymentMethodList: {
		Key:            PluginPermissionHostPaymentMethodList,
		Title:          "Host Payment Method List",
		Description:    "Allow the plugin to query native payment method summaries through Plugin.paymentMethod.list().",
		DefaultGranted: false,
	},
	PluginPermissionHostVirtualInventoryRead: {
		Key:            PluginPermissionHostVirtualInventoryRead,
		Title:          "Host Virtual Inventory Read",
		Description:    "Allow the plugin to read a single native virtual inventory summary through Plugin.virtualInventory.get().",
		DefaultGranted: false,
	},
	PluginPermissionHostVirtualInventoryList: {
		Key:            PluginPermissionHostVirtualInventoryList,
		Title:          "Host Virtual Inventory List",
		Description:    "Allow the plugin to query native virtual inventory summaries through Plugin.virtualInventory.list().",
		DefaultGranted: false,
	},
	PluginPermissionHostVirtualInventoryBindingRead: {
		Key:            PluginPermissionHostVirtualInventoryBindingRead,
		Title:          "Host Virtual Inventory Binding Read",
		Description:    "Allow the plugin to read a single native product-virtual-inventory binding through Plugin.virtualInventoryBinding.get().",
		DefaultGranted: false,
	},
	PluginPermissionHostVirtualInventoryBindingList: {
		Key:            PluginPermissionHostVirtualInventoryBindingList,
		Title:          "Host Virtual Inventory Binding List",
		Description:    "Allow the plugin to query native product-virtual-inventory bindings through Plugin.virtualInventoryBinding.list().",
		DefaultGranted: false,
	},
	PluginPermissionHostMarketSourceRead: {
		Key:            PluginPermissionHostMarketSourceRead,
		Title:          "Host Market Source Read",
		Description:    "Allow the plugin to read trusted market source bindings through Plugin.market.source.list() and Plugin.market.source.get().",
		DefaultGranted: false,
	},
	PluginPermissionHostMarketCatalogRead: {
		Key:            PluginPermissionHostMarketCatalogRead,
		Title:          "Host Market Catalog Read",
		Description:    "Allow the plugin to query trusted market catalog, artifact, and release metadata through Plugin.market.catalog / Plugin.market.artifact / Plugin.market.release.",
		DefaultGranted: false,
	},
	PluginPermissionHostMarketInstallPreview: {
		Key:            PluginPermissionHostMarketInstallPreview,
		Title:          "Host Market Install Preview",
		Description:    "Allow the plugin to request market installation previews through Plugin.market.install.preview().",
		DefaultGranted: false,
	},
	PluginPermissionHostMarketInstallExecute: {
		Key:            PluginPermissionHostMarketInstallExecute,
		Title:          "Host Market Install Execute",
		Description:    "Allow the plugin to execute trusted market installations through Plugin.market.install.execute().",
		DefaultGranted: false,
	},
	PluginPermissionHostMarketInstallRead: {
		Key:            PluginPermissionHostMarketInstallRead,
		Title:          "Host Market Install Read",
		Description:    "Allow the plugin to read market installation tasks and history through Plugin.market.install.task / Plugin.market.install.history.",
		DefaultGranted: false,
	},
	PluginPermissionHostMarketInstallRollback: {
		Key:            PluginPermissionHostMarketInstallRollback,
		Title:          "Host Market Install Rollback",
		Description:    "Allow the plugin to trigger market-managed rollback operations through Plugin.market.install.rollback().",
		DefaultGranted: false,
	},
	PluginPermissionHostEmailTemplateRead: {
		Key:            PluginPermissionHostEmailTemplateRead,
		Title:          "Host Email Template Read",
		Description:    "Allow the plugin to read native email templates through Plugin.emailTemplate.list() and Plugin.emailTemplate.get().",
		DefaultGranted: false,
	},
	PluginPermissionHostEmailTemplateWrite: {
		Key:            PluginPermissionHostEmailTemplateWrite,
		Title:          "Host Email Template Write",
		Description:    "Allow the plugin to save native email templates through Plugin.emailTemplate.save().",
		DefaultGranted: false,
	},
	PluginPermissionHostLandingPageRead: {
		Key:            PluginPermissionHostLandingPageRead,
		Title:          "Host Landing Page Read",
		Description:    "Allow the plugin to read native landing page content through Plugin.landingPage.get().",
		DefaultGranted: false,
	},
	PluginPermissionHostLandingPageWrite: {
		Key:            PluginPermissionHostLandingPageWrite,
		Title:          "Host Landing Page Write",
		Description:    "Allow the plugin to save or reset native landing page content through Plugin.landingPage.save() and Plugin.landingPage.reset().",
		DefaultGranted: false,
	},
	PluginPermissionHostInvoiceTemplateRead: {
		Key:            PluginPermissionHostInvoiceTemplateRead,
		Title:          "Host Invoice Template Read",
		Description:    "Allow the plugin to read the native invoice template through Plugin.invoiceTemplate.get().",
		DefaultGranted: false,
	},
	PluginPermissionHostInvoiceTemplateWrite: {
		Key:            PluginPermissionHostInvoiceTemplateWrite,
		Title:          "Host Invoice Template Write",
		Description:    "Allow the plugin to save or reset the native invoice template through Plugin.invoiceTemplate.save() and Plugin.invoiceTemplate.reset().",
		DefaultGranted: false,
	},
	PluginPermissionHostAuthBrandingRead: {
		Key:            PluginPermissionHostAuthBrandingRead,
		Title:          "Host Auth Branding Read",
		Description:    "Allow the plugin to read the native auth branding template through Plugin.authBranding.get().",
		DefaultGranted: false,
	},
	PluginPermissionHostAuthBrandingWrite: {
		Key:            PluginPermissionHostAuthBrandingWrite,
		Title:          "Host Auth Branding Write",
		Description:    "Allow the plugin to save or reset the native auth branding template through Plugin.authBranding.save() and Plugin.authBranding.reset().",
		DefaultGranted: false,
	},
	PluginPermissionHostPageRulePackRead: {
		Key:            PluginPermissionHostPageRulePackRead,
		Title:          "Host Page Rule Pack Read",
		Description:    "Allow the plugin to read native page rules through Plugin.pageRulePack.get().",
		DefaultGranted: false,
	},
	PluginPermissionHostPageRulePackWrite: {
		Key:            PluginPermissionHostPageRulePackWrite,
		Title:          "Host Page Rule Pack Write",
		Description:    "Allow the plugin to save or reset native page rules through Plugin.pageRulePack.save() and Plugin.pageRulePack.reset().",
		DefaultGranted: false,
	},
	PluginPermissionHostPluginPageRuleRead: {
		Key:            PluginPermissionHostPluginPageRuleRead,
		Title:          "Plugin Page Rule Read",
		Description:    "Allow the plugin to read its own injected page rules through Plugin.pageRules.list() and Plugin.pageRules.get().",
		DefaultGranted: false,
	},
	PluginPermissionHostPluginPageRuleWrite: {
		Key:            PluginPermissionHostPluginPageRuleWrite,
		Title:          "Plugin Page Rule Write",
		Description:    "Allow the plugin to create, update, delete, and activate its own injected page rules through Plugin.pageRules.upsert(), Plugin.pageRules.delete(), and Plugin.pageRules.reset().",
		DefaultGranted: false,
	},
}

func NormalizePluginPermissionKey(key string) string {
	return strings.ToLower(strings.TrimSpace(key))
}

func NormalizePluginPermissionList(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		normalized := NormalizePluginPermissionKey(value)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}

func BuildPluginPermissionRequests(requested []string, required []string, reasonByKey map[string]string) []PluginPermissionRequest {
	normalizedRequested := NormalizePluginPermissionList(requested)
	normalizedRequired := NormalizePluginPermissionList(required)
	requiredSet := make(map[string]struct{}, len(normalizedRequired))
	for _, key := range normalizedRequired {
		requiredSet[key] = struct{}{}
	}

	requestSet := make(map[string]struct{}, len(normalizedRequested)+len(normalizedRequired))
	merged := make([]string, 0, len(normalizedRequested)+len(normalizedRequired))
	for _, key := range normalizedRequested {
		requestSet[key] = struct{}{}
		merged = append(merged, key)
	}
	for _, key := range normalizedRequired {
		if _, exists := requestSet[key]; exists {
			continue
		}
		requestSet[key] = struct{}{}
		merged = append(merged, key)
	}
	sort.Strings(merged)

	out := make([]PluginPermissionRequest, 0, len(merged))
	for _, key := range merged {
		def := permissionDefinitionByKey(key)
		_, required := requiredSet[key]
		reason := ""
		if reasonByKey != nil {
			reason = strings.TrimSpace(reasonByKey[key])
		}
		out = append(out, PluginPermissionRequest{
			Key:            key,
			Required:       required,
			Reason:         reason,
			Title:          def.Title,
			Description:    def.Description,
			DefaultGranted: required || def.DefaultGranted,
		})
	}
	return out
}

func DefaultGrantedPluginPermissions(requests []PluginPermissionRequest) []string {
	out := make([]string, 0, len(requests))
	for _, request := range requests {
		if request.Required || request.DefaultGranted {
			out = append(out, request.Key)
		}
	}
	return NormalizePluginPermissionList(out)
}

func ValidateGrantedPluginPermissions(requests []PluginPermissionRequest, granted []string) ([]string, error) {
	normalizedGranted := NormalizePluginPermissionList(granted)
	grantedSet := make(map[string]struct{}, len(normalizedGranted))
	for _, key := range normalizedGranted {
		grantedSet[key] = struct{}{}
	}

	requestSet := make(map[string]struct{}, len(requests))
	for _, request := range requests {
		requestSet[request.Key] = struct{}{}
		if request.Required {
			if _, exists := grantedSet[request.Key]; !exists {
				return nil, fmt.Errorf("required permission %s must be granted", request.Key)
			}
		}
	}

	filtered := make([]string, 0, len(normalizedGranted))
	for _, key := range normalizedGranted {
		if _, exists := requestSet[key]; exists {
			filtered = append(filtered, key)
		}
	}
	return filtered, nil
}

func IsPluginPermissionGranted(requested []string, granted []string, permission string) bool {
	normalizedPermission := NormalizePluginPermissionKey(permission)
	if normalizedPermission == "" {
		return false
	}
	normalizedRequested := NormalizePluginPermissionList(requested)
	normalizedGranted := NormalizePluginPermissionList(granted)

	requestedSet := make(map[string]struct{}, len(normalizedRequested))
	for _, key := range normalizedRequested {
		requestedSet[key] = struct{}{}
	}
	grantedSet := make(map[string]struct{}, len(normalizedGranted))
	for _, key := range normalizedGranted {
		grantedSet[key] = struct{}{}
	}

	// Legacy fallback: if plugin did not declare requested permissions,
	// only explicit grants can unlock a permission.
	if len(requestedSet) == 0 {
		_, grantedExists := grantedSet[normalizedPermission]
		return grantedExists
	}

	if _, requestedExists := requestedSet[normalizedPermission]; !requestedExists {
		return false
	}
	_, grantedExists := grantedSet[normalizedPermission]
	return grantedExists
}

func permissionDefinitionByKey(key string) PluginPermissionDefinition {
	normalized := NormalizePluginPermissionKey(key)
	if def, exists := pluginPermissionDefinitions[normalized]; exists {
		return def
	}
	return PluginPermissionDefinition{
		Key:            normalized,
		Title:          strings.ToUpper(normalized),
		Description:    "Custom plugin permission.",
		DefaultGranted: false,
	}
}
