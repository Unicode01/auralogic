package admin

import (
	"net/http"
	"strings"

	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
)

type adminPluginHookCatalogGroup struct {
	Key   string   `json:"key"`
	Hooks []string `json:"hooks"`
}

type adminPluginHookCatalogResponse struct {
	Groups []adminPluginHookCatalogGroup `json:"groups"`
	Hooks  []string                      `json:"hooks"`
}

var adminPluginHookCatalogGroupOrder = []string{
	"frontend",
	"auth",
	"user",
	"cart",
	"order",
	"payment",
	"plugin",
	"template_content",
	"ticket",
	"product_inventory",
	"promo",
	"other",
}

func resolveAdminPluginHookCatalogGroupKey(hook string) string {
	normalized := strings.ToLower(strings.TrimSpace(hook))
	switch {
	case strings.HasPrefix(normalized, "frontend."):
		return "frontend"
	case strings.HasPrefix(normalized, "auth."):
		return "auth"
	case strings.HasPrefix(normalized, "user."):
		return "user"
	case strings.HasPrefix(normalized, "cart."):
		return "cart"
	case strings.HasPrefix(normalized, "order."):
		return "order"
	case strings.HasPrefix(normalized, "payment."):
		return "payment"
	case strings.HasPrefix(normalized, "plugin."):
		return "plugin"
	case strings.HasPrefix(normalized, "settings."),
		strings.HasPrefix(normalized, "email_template."),
		strings.HasPrefix(normalized, "landing_page."),
		strings.HasPrefix(normalized, "template."),
		strings.HasPrefix(normalized, "announcement."),
		strings.HasPrefix(normalized, "knowledge."),
		strings.HasPrefix(normalized, "invoice_template."),
		strings.HasPrefix(normalized, "auth_branding."),
		strings.HasPrefix(normalized, "page_rule_pack."):
		return "template_content"
	case strings.HasPrefix(normalized, "ticket."):
		return "ticket"
	case strings.HasPrefix(normalized, "product."),
		strings.HasPrefix(normalized, "inventory."),
		strings.HasPrefix(normalized, "virtual_inventory."):
		return "product_inventory"
	case strings.HasPrefix(normalized, "promo."):
		return "promo"
	default:
		return "other"
	}
}

func buildAdminPluginHookCatalog() adminPluginHookCatalogResponse {
	hooks := service.ListSupportedPluginHooks()
	grouped := make(map[string][]string, len(adminPluginHookCatalogGroupOrder))
	for _, hook := range hooks {
		groupKey := resolveAdminPluginHookCatalogGroupKey(hook)
		grouped[groupKey] = append(grouped[groupKey], hook)
	}

	groups := make([]adminPluginHookCatalogGroup, 0, len(adminPluginHookCatalogGroupOrder))
	for _, key := range adminPluginHookCatalogGroupOrder {
		groupHooks := grouped[key]
		if len(groupHooks) == 0 {
			continue
		}
		groups = append(groups, adminPluginHookCatalogGroup{
			Key:   key,
			Hooks: groupHooks,
		})
	}

	return adminPluginHookCatalogResponse{
		Groups: groups,
		Hooks:  hooks,
	}
}

func (h *PluginHandler) GetHookCatalog(c *gin.Context) {
	c.JSON(http.StatusOK, buildAdminPluginHookCatalog())
}
