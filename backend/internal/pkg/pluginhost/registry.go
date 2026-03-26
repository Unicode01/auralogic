package pluginhost

import "strings"

type ActionDefinition struct {
	Action              string
	PluginPermissions   []string
	OperatorPermissions []string
	JSImportPath        []string
}

var sharedActionDefinitions = []ActionDefinition{
	{
		Action:              "host.market.source.list",
		PluginPermissions:   []string{"host.market.source.read"},
		OperatorPermissions: []string{"market.view"},
		JSImportPath:        []string{"market", "source", "list"},
	},
	{
		Action:              "host.market.source.get",
		PluginPermissions:   []string{"host.market.source.read"},
		OperatorPermissions: []string{"market.view"},
		JSImportPath:        []string{"market", "source", "get"},
	},
	{
		Action:              "host.market.catalog.list",
		PluginPermissions:   []string{"host.market.catalog.read"},
		OperatorPermissions: []string{"market.view"},
		JSImportPath:        []string{"market", "catalog", "list"},
	},
	{
		Action:              "host.market.artifact.get",
		PluginPermissions:   []string{"host.market.catalog.read"},
		OperatorPermissions: []string{"market.view"},
		JSImportPath:        []string{"market", "artifact", "get"},
	},
	{
		Action:              "host.market.release.get",
		PluginPermissions:   []string{"host.market.catalog.read"},
		OperatorPermissions: []string{"market.view"},
		JSImportPath:        []string{"market", "release", "get"},
	},
	{
		Action:              "host.market.install.preview",
		PluginPermissions:   []string{"host.market.install.preview"},
		OperatorPermissions: []string{"market.install"},
		JSImportPath:        []string{"market", "install", "preview"},
	},
	{
		Action:              "host.market.install.execute",
		PluginPermissions:   []string{"host.market.install.execute"},
		OperatorPermissions: []string{"market.install"},
		JSImportPath:        []string{"market", "install", "execute"},
	},
	{
		Action:              "host.market.install.task.get",
		PluginPermissions:   []string{"host.market.install.read"},
		OperatorPermissions: []string{"market.history"},
		JSImportPath:        []string{"market", "install", "task", "get"},
	},
	{
		Action:              "host.market.install.task.list",
		PluginPermissions:   []string{"host.market.install.read"},
		OperatorPermissions: []string{"market.history"},
		JSImportPath:        []string{"market", "install", "task", "list"},
	},
	{
		Action:              "host.market.install.history.list",
		PluginPermissions:   []string{"host.market.install.read"},
		OperatorPermissions: []string{"market.history"},
		JSImportPath:        []string{"market", "install", "history", "list"},
	},
	{
		Action:              "host.market.install.rollback",
		PluginPermissions:   []string{"host.market.install.rollback"},
		OperatorPermissions: []string{"market.install"},
		JSImportPath:        []string{"market", "install", "rollback"},
	},
	{
		Action:              "host.email_template.list",
		PluginPermissions:   []string{"host.email_template.read"},
		OperatorPermissions: []string{"email_template.view"},
		JSImportPath:        []string{"emailTemplate", "list"},
	},
	{
		Action:              "host.email_template.get",
		PluginPermissions:   []string{"host.email_template.read"},
		OperatorPermissions: []string{"email_template.view"},
		JSImportPath:        []string{"emailTemplate", "get"},
	},
	{
		Action:              "host.email_template.save",
		PluginPermissions:   []string{"host.email_template.write"},
		OperatorPermissions: []string{"email_template.edit"},
		JSImportPath:        []string{"emailTemplate", "save"},
	},
	{
		Action:              "host.landing_page.get",
		PluginPermissions:   []string{"host.landing_page.read"},
		OperatorPermissions: []string{"landing_page.view"},
		JSImportPath:        []string{"landingPage", "get"},
	},
	{
		Action:              "host.landing_page.save",
		PluginPermissions:   []string{"host.landing_page.write"},
		OperatorPermissions: []string{"landing_page.edit"},
		JSImportPath:        []string{"landingPage", "save"},
	},
	{
		Action:              "host.landing_page.reset",
		PluginPermissions:   []string{"host.landing_page.write"},
		OperatorPermissions: []string{"landing_page.edit"},
		JSImportPath:        []string{"landingPage", "reset"},
	},
	{
		Action:              "host.invoice_template.get",
		PluginPermissions:   []string{"host.invoice_template.read"},
		OperatorPermissions: []string{"invoice_template.view"},
		JSImportPath:        []string{"invoiceTemplate", "get"},
	},
	{
		Action:              "host.invoice_template.save",
		PluginPermissions:   []string{"host.invoice_template.write"},
		OperatorPermissions: []string{"invoice_template.edit"},
		JSImportPath:        []string{"invoiceTemplate", "save"},
	},
	{
		Action:              "host.invoice_template.reset",
		PluginPermissions:   []string{"host.invoice_template.write"},
		OperatorPermissions: []string{"invoice_template.edit"},
		JSImportPath:        []string{"invoiceTemplate", "reset"},
	},
	{
		Action:              "host.auth_branding.get",
		PluginPermissions:   []string{"host.auth_branding.read"},
		OperatorPermissions: []string{"auth_branding.view"},
		JSImportPath:        []string{"authBranding", "get"},
	},
	{
		Action:              "host.auth_branding.save",
		PluginPermissions:   []string{"host.auth_branding.write"},
		OperatorPermissions: []string{"auth_branding.edit"},
		JSImportPath:        []string{"authBranding", "save"},
	},
	{
		Action:              "host.auth_branding.reset",
		PluginPermissions:   []string{"host.auth_branding.write"},
		OperatorPermissions: []string{"auth_branding.edit"},
		JSImportPath:        []string{"authBranding", "reset"},
	},
	{
		Action:              "host.page_rule_pack.get",
		PluginPermissions:   []string{"host.page_rule_pack.read"},
		OperatorPermissions: []string{"page_rule_pack.view"},
		JSImportPath:        []string{"pageRulePack", "get"},
	},
	{
		Action:              "host.page_rule_pack.save",
		PluginPermissions:   []string{"host.page_rule_pack.write"},
		OperatorPermissions: []string{"page_rule_pack.edit"},
		JSImportPath:        []string{"pageRulePack", "save"},
	},
	{
		Action:              "host.page_rule_pack.reset",
		PluginPermissions:   []string{"host.page_rule_pack.write"},
		OperatorPermissions: []string{"page_rule_pack.edit"},
		JSImportPath:        []string{"pageRulePack", "reset"},
	},
}

var sharedActionDefinitionIndex = buildActionDefinitionIndex(sharedActionDefinitions)

func ListSharedActionDefinitions() []ActionDefinition {
	defs := make([]ActionDefinition, 0, len(sharedActionDefinitions))
	for _, def := range sharedActionDefinitions {
		defs = append(defs, cloneActionDefinition(def))
	}
	return defs
}

func LookupSharedActionDefinition(action string) (ActionDefinition, bool) {
	def, ok := sharedActionDefinitionIndex[normalizeAction(action)]
	if !ok {
		return ActionDefinition{}, false
	}
	return cloneActionDefinition(def), true
}

func buildActionDefinitionIndex(defs []ActionDefinition) map[string]ActionDefinition {
	index := make(map[string]ActionDefinition, len(defs))
	for _, def := range defs {
		normalized := normalizeAction(def.Action)
		if normalized == "" {
			continue
		}
		index[normalized] = cloneActionDefinition(def)
	}
	return index
}

func normalizeAction(action string) string {
	return strings.ToLower(strings.TrimSpace(action))
}

func cloneActionDefinition(def ActionDefinition) ActionDefinition {
	return ActionDefinition{
		Action:              def.Action,
		PluginPermissions:   cloneStrings(def.PluginPermissions),
		OperatorPermissions: cloneStrings(def.OperatorPermissions),
		JSImportPath:        cloneStrings(def.JSImportPath),
	}
}

func cloneStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		cloned = append(cloned, trimmed)
	}
	if len(cloned) == 0 {
		return nil
	}
	return cloned
}
