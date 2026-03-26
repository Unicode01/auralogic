package artifactmanifest

import (
	"sort"
	"strings"
)

func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func String(manifest map[string]any, keys ...string) string {
	for _, key := range keys {
		text, _ := manifest[key].(string)
		if trimmed := strings.TrimSpace(text); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func LocalizedString(manifest map[string]any, keys ...string) string {
	for _, key := range keys {
		if trimmed := localizedTextValue(manifest[key]); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func Title(manifest map[string]any) string {
	return FirstNonEmpty(
		String(manifest, "title"),
		LocalizedString(manifest, "display_name", "displayName"),
		String(manifest, "display_name", "displayName"),
	)
}

func Summary(manifest map[string]any) string {
	return FirstNonEmpty(
		String(manifest, "summary"),
		LocalizedString(manifest, "summary"),
		LocalizedString(manifest, "description"),
		String(manifest, "description"),
	)
}

func Description(manifest map[string]any) string {
	return FirstNonEmpty(
		LocalizedString(manifest, "description"),
		String(manifest, "description"),
		Summary(manifest),
	)
}

func InferKind(manifest map[string]any) string {
	if kind := String(manifest, "kind"); kind != "" {
		return kind
	}

	switch strings.ToLower(String(manifest, "runtime")) {
	case "js_worker", "grpc":
		return "plugin_package"
	case "payment_js":
		return "payment_package"
	}

	contentFile := String(manifest, "content_file")
	rulesFile := String(manifest, "rules_file")
	templatePayload := mapValue(manifest["template"])
	pageRulesPayload := mapValue(manifest["page_rules"])
	if contentFile == "" &&
		rulesFile == "" &&
		String(templatePayload, "content") == "" &&
		String(pageRulesPayload, "content") == "" &&
		pageRulesPayload["rules"] == nil &&
		templatePayload["rules"] == nil {
		return ""
	}
	if String(manifest, "event") != "" {
		return "email_template"
	}
	switch strings.ToLower(FirstNonEmpty(
		String(manifest, "key", "target_key"),
		String(mapValue(manifest["targets"]), "key"),
		String(templatePayload, "key"),
		String(pageRulesPayload, "key"),
	)) {
	case "invoice":
		return "invoice_template"
	case "auth_branding":
		return "auth_branding_template"
	case "page_rules":
		return "page_rule_pack"
	}
	if String(manifest, "engine") != "" {
		return "landing_page_template"
	}
	return ""
}

func mapValue(value any) map[string]any {
	mapped, _ := value.(map[string]any)
	if mapped == nil {
		return map[string]any{}
	}
	return mapped
}

func localizedTextValue(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		return localizedTextMap(typed)
	case map[string]string:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[key] = item
		}
		return localizedTextMap(out)
	default:
		return ""
	}
}

func localizedTextMap(values map[string]any) string {
	if len(values) == 0 {
		return ""
	}
	preferredKeys := []string{"en", "en-US", "en_US", "zh-CN", "zh_CN", "zh"}
	for _, key := range preferredKeys {
		if text, ok := values[key].(string); ok {
			if trimmed := strings.TrimSpace(text); trimmed != "" {
				return trimmed
			}
		}
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if text, ok := values[key].(string); ok {
			if trimmed := strings.TrimSpace(text); trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}
