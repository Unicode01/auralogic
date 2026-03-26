package publish

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

func deriveCompatibility(req Request, manifest map[string]any) map[string]any {
	compatibility := cloneMap(req.Metadata.Compatibility)
	setStringIfMissing(compatibility, "min_host_version", manifest["min_host_version"])
	setStringIfMissing(compatibility, "max_host_version", manifest["max_host_version"])
	setStringIfMissing(compatibility, "runtime", manifest["runtime"])
	setStringIfMissing(compatibility, "manifest_version", manifest["manifest_version"])
	setStringIfMissing(compatibility, "protocol_version", manifest["protocol_version"])
	setStringIfMissing(compatibility, "min_host_protocol_version", manifest["min_host_protocol_version"])
	setStringIfMissing(compatibility, "max_host_protocol_version", manifest["max_host_protocol_version"])
	setStringIfMissing(compatibility, "min_host_bridge_version", manifest["min_host_bridge_version"])
	if stringValue(compatibility["min_host_bridge_version"]) == "" {
		compatibility["min_host_bridge_version"] = "1.0.0"
	}
	if req.Kind == "email_template" || req.Kind == "landing_page_template" || req.Kind == "invoice_template" || req.Kind == "auth_branding_template" || req.Kind == "page_rule_pack" {
		setStringIfMissing(compatibility, "engine", manifest["engine"])
	}
	return compatibility
}

func derivePermissions(req Request, manifest map[string]any) map[string]any {
	permissions := cloneMap(req.Metadata.Permissions)
	capabilities := mapValue(manifest["capabilities"])
	if len(stringSliceValue(permissions["requested"])) == 0 {
		permissions["requested"] = stringSliceValue(capabilities["requested_permissions"])
	}
	if len(stringSliceValue(permissions["default_granted"])) == 0 {
		permissions["default_granted"] = stringSliceValue(capabilities["granted_permissions"])
	}
	if _, exists := permissions["requires_reconfirm_on_upgrade"]; !exists {
		permissions["requires_reconfirm_on_upgrade"] = false
	}
	return permissions
}

func deriveLabels(req Request, manifest map[string]any) []string {
	if len(req.Metadata.Labels) > 0 {
		return append([]string(nil), req.Metadata.Labels...)
	}
	return stringSliceValue(manifest["labels"])
}

func deriveGovernance(kind string, manifest map[string]any) map[string]any {
	governance := cloneMap(mapValue(manifest["governance"]))
	if stringValue(governance["mode"]) == "" {
		governance["mode"] = "host_managed"
	}
	if stringValue(governance["install_strategy"]) == "" {
		if kind == "email_template" || kind == "landing_page_template" || kind == "invoice_template" || kind == "auth_branding_template" || kind == "page_rule_pack" {
			governance["install_strategy"] = "host_template_install"
		} else {
			governance["install_strategy"] = "host_bridge"
		}
	}
	if _, exists := governance["supports_rollback"]; !exists {
		governance["supports_rollback"] = kind != "payment_package"
	}
	return governance
}

func deriveInstall(kind string, manifest map[string]any) map[string]any {
	install := cloneMap(mapValue(manifest["install"]))
	if stringValue(install["package_format"]) == "" {
		install["package_format"] = "zip"
	}
	switch kind {
	case "plugin_package":
		setStringIfMissing(install, "entry", manifest["entry"])
		setBoolIfMissing(install, "requires_host_download", true)
		setBoolIfMissing(install, "auto_activate_default", true)
		setBoolIfMissing(install, "auto_start_default", false)
	case "payment_package":
		setStringIfMissing(install, "entry", manifest["entry"])
		setBoolIfMissing(install, "requires_host_download", true)
	case "email_template", "landing_page_template", "invoice_template", "auth_branding_template", "page_rule_pack":
		setBoolIfMissing(install, "requires_host_download", false)
		setBoolIfMissing(install, "inline_content", true)
		setStringIfMissing(install, "save_mode", manifest["save_mode"], "replace")
	}
	return install
}

func deriveTargets(kind string, name string, manifest map[string]any) map[string]any {
	targets := cloneMap(mapValue(manifest["targets"]))
	switch kind {
	case "payment_package":
		setStringIfMissing(targets, "target", manifest["target"], "payment_method")
	case "email_template":
		setStringIfMissing(targets, "event", manifest["event"])
		setStringIfMissing(targets, "key", manifest["key"], manifest["name"], name)
		setStringIfMissing(targets, "engine", manifest["engine"])
	case "landing_page_template":
		setStringIfMissing(targets, "page_key", manifest["page_key"], manifest["name"], name)
		setStringIfMissing(targets, "slug", manifest["slug"], manifest["name"], name)
		setStringIfMissing(targets, "engine", manifest["engine"])
	case "invoice_template":
		targets["key"] = "invoice"
		setStringIfMissing(targets, "engine", manifest["engine"])
	case "auth_branding_template":
		targets["key"] = "auth_branding"
		setStringIfMissing(targets, "engine", manifest["engine"])
	case "page_rule_pack":
		targets["key"] = "page_rules"
	}
	return targets
}

func deriveTemplate(kind string, name string, manifest map[string]any, artifactZip []byte) (map[string]any, error) {
	template := cloneMap(mapValue(manifest["template"]))
	contentFile := firstNonEmpty(
		stringValue(manifest["content_file"]),
		stringValue(manifest["template_file"]),
		stringValue(manifest["rules_file"]),
	)
	switch kind {
	case "email_template":
		setStringIfMissing(template, "key", manifest["key"], manifest["name"], name)
		setStringIfMissing(template, "filename", contentFile, name+".html")
		setStringIfMissing(template, "subject", manifest["subject"])
		if len(stringSliceValue(template["variables"])) == 0 {
			template["variables"] = stringSliceValue(manifest["variables"])
		}
	case "landing_page_template":
		setStringIfMissing(template, "page_key", manifest["page_key"], manifest["name"], name)
		setStringIfMissing(template, "slug", manifest["slug"], manifest["name"], name)
		setStringIfMissing(template, "filename", contentFile, name+".html")
	case "invoice_template":
		template["key"] = "invoice"
		setStringIfMissing(template, "filename", contentFile, name+".html")
		if len(stringSliceValue(template["variables"])) == 0 {
			template["variables"] = stringSliceValue(manifest["variables"])
		}
	case "auth_branding_template":
		template["key"] = "auth_branding"
		setStringIfMissing(template, "filename", contentFile, name+".html")
		if len(stringSliceValue(template["variables"])) == 0 {
			template["variables"] = stringSliceValue(manifest["variables"])
		}
	case "page_rule_pack":
		template["key"] = "page_rules"
		setStringIfMissing(template, "filename", contentFile, stringValue(manifest["rules_file"]), name+".json")
	}
	if contentFile != "" {
		content, err := readZipTextFile(artifactZip, contentFile)
		if err != nil {
			return nil, err
		}
		template["content"] = content
	}
	return template, nil
}

func derivePageRules(kind string, manifest map[string]any, artifactZip []byte) map[string]any {
	if kind != "page_rule_pack" {
		return map[string]any{}
	}

	pageRules := cloneMap(mapValue(manifest["page_rules"]))
	pageRules["key"] = "page_rules"

	content := firstNonEmpty(
		stringValue(pageRules["content"]),
		stringValue(mapValue(manifest["template"])["content"]),
	)
	if content == "" {
		if rules := pageRules["rules"]; rules != nil {
			if body, err := json.Marshal(rules); err == nil {
				content = string(body)
			}
		}
	}
	if content == "" {
		contentFile := firstNonEmpty(
			stringValue(manifest["rules_file"]),
			stringValue(manifest["content_file"]),
			stringValue(manifest["template_file"]),
		)
		if contentFile != "" {
			if loadedContent, err := readZipTextFile(artifactZip, contentFile); err == nil {
				content = loadedContent
			}
		}
	}
	if strings.TrimSpace(content) != "" {
		pageRules["content"] = content
		var parsed []map[string]any
		if err := json.Unmarshal([]byte(content), &parsed); err == nil {
			pageRules["rules"] = parsed
		}
	}
	return pageRules
}

func deriveDocs(manifest map[string]any) map[string]any {
	docs := cloneMap(mapValue(manifest["docs"]))
	setStringIfMissing(docs, "docs_url", manifest["docs_url"], manifest["documentation_url"], manifest["homepage"])
	setStringIfMissing(docs, "support_url", manifest["support_url"])
	setStringIfMissing(docs, "changelog_url", manifest["changelog_url"])
	return docs
}

func deriveUI(manifest map[string]any) map[string]any {
	ui := cloneMap(mapValue(manifest["ui"]))
	setStringIfMissing(ui, "icon", manifest["icon"])
	setStringIfMissing(ui, "icon_url", manifest["icon_url"])
	return ui
}

func readZipTextFile(zipData []byte, target string) (string, error) {
	target = strings.Trim(strings.TrimSpace(target), "/")
	if target == "" {
		return "", nil
	}
	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return "", newRequestError(fmt.Sprintf("open zip: %v", err))
	}
	for _, file := range reader.File {
		name := strings.Trim(strings.TrimSpace(strings.ReplaceAll(file.Name, "\\", "/")), "/")
		if name != target {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return "", newRequestError(fmt.Sprintf("open %s: %v", target, err))
		}
		defer rc.Close()
		data, err := io.ReadAll(rc)
		if err != nil {
			return "", newRequestError(fmt.Sprintf("read %s: %v", target, err))
		}
		return string(data), nil
	}
	return "", newRequestError(fmt.Sprintf("%s not found in zip", target))
}
