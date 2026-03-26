import { readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import { buildCatalogSnapshot } from "./catalog-source.mjs";

const args = process.argv.slice(2);
const pluginRootArg = args.find((value) => !value.startsWith("--")) || ".";
const profileArg = args.find((value) => value.startsWith("--profile="));
const profile = profileArg ? profileArg.slice("--profile=".length).trim() : "";
const checkOnly = args.includes("--check");

const pluginRoot = path.resolve(process.cwd(), pluginRootArg);
const manifestPath = path.join(pluginRoot, "manifest.json");
const manifest = JSON.parse(await readFile(manifestPath, "utf8"));
const snapshot = buildCatalogSnapshot();
const PROFILE_BUILDERS = {
  debugger: syncDebuggerManifest,
  template: syncTemplateManifest,
  market: syncMarketManifest
};

const sync = PROFILE_BUILDERS[profile];
if (!sync) {
  throw new Error(
    `unsupported manifest sync profile: ${profile || "(missing)"}. expected one of ${Object.keys(PROFILE_BUILDERS).join(", ")}`
  );
}

const syncedManifest = sync(manifest, snapshot);
const serialized = `${JSON.stringify(syncedManifest, null, 2)}\n`;
const pluginLabel = path.basename(pluginRoot);

if (checkOnly) {
  const currentSerialized = `${JSON.stringify(manifest, null, 2)}\n`;
  if (currentSerialized !== serialized) {
    throw new Error(`manifest drift detected for ${pluginLabel} (${profile})`);
  }
  console.log(`manifest sync check passed for ${pluginLabel} (${profile})`);
} else {
  await writeFile(manifestPath, serialized, "utf8");
  console.log(`synced manifest for ${pluginLabel} (${profile})`);
}

function cloneManifest(source) {
  return JSON.parse(JSON.stringify(source));
}

function applyHostCompatibility(manifestValue) {
  manifestValue.manifest_version = "1.0.0";
  manifestValue.protocol_version = "1.0.0";
  manifestValue.min_host_protocol_version = "1.0.0";
  manifestValue.max_host_protocol_version = "1.0.0";
}

function ensureRecord(value) {
  if (value && typeof value === "object" && !Array.isArray(value)) {
    return value;
  }
  return {};
}

function collectPermissionEntries(manifestValue) {
  return new Map(
    (Array.isArray(manifestValue.permissions) ? manifestValue.permissions : [])
      .filter((item) => item && typeof item === "object" && !Array.isArray(item))
      .map((item) => [String(item.key || "").trim(), item])
      .filter(([key]) => key.length > 0)
  );
}

function syncPermissionCatalog(manifestValue, permissions, reasonResolver) {
  const existingPermissions = collectPermissionEntries(manifestValue);
  const capabilities = ensureRecord(manifestValue.capabilities);

  capabilities.requested_permissions = [...permissions];
  capabilities.granted_permissions = [...permissions];
  manifestValue.capabilities = capabilities;
  manifestValue.permissions = permissions.map((key) =>
    buildPermissionEntry(key, existingPermissions.get(key), reasonResolver)
  );
}

function buildPermissionEntry(key, existing, reasonResolver) {
  const entry = existing && typeof existing === "object" && !Array.isArray(existing) ? existing : {};
  const normalized = {
    key,
    reason: resolvePermissionReason(key, entry.reason, reasonResolver)
  };

  if (typeof entry.required === "boolean") {
    normalized.required = entry.required;
  }
  if (typeof entry.title === "string" && entry.title.trim()) {
    normalized.title = entry.title.trim();
  }
  if (typeof entry.description === "string" && entry.description.trim()) {
    normalized.description = entry.description.trim();
  }
  if (typeof entry.default_granted === "boolean") {
    normalized.default_granted = entry.default_granted;
  }

  return normalized;
}

function resolvePermissionReason(key, currentReason, reasonResolver) {
  if (typeof currentReason === "string" && currentReason.trim()) {
    return currentReason.trim();
  }
  return reasonResolver(key);
}

function syncDebuggerManifest(sourceManifest, catalogSnapshot) {
  const manifestValue = cloneManifest(sourceManifest);
  const capabilities = ensureRecord(manifestValue.capabilities);

  applyHostCompatibility(manifestValue);
  capabilities.hooks = [...catalogSnapshot.plugin_hooks];
  capabilities.allowed_frontend_slots = [...catalogSnapshot.frontend_slots];
  manifestValue.capabilities = capabilities;

  syncPermissionCatalog(
    manifestValue,
    catalogSnapshot.plugin_permission_keys,
    debuggerPermissionReason
  );

  return manifestValue;
}

function syncTemplateManifest(sourceManifest) {
  const manifestValue = cloneManifest(sourceManifest);
  const capabilities = ensureRecord(manifestValue.capabilities);
  const permissions = [
    "hook.execute",
    "frontend.extensions",
    "api.execute",
    "host.order.read",
    "host.user.read"
  ];

  applyHostCompatibility(manifestValue);
  capabilities.hooks = ["frontend.bootstrap"];
  capabilities.disabled_hooks = [];
  capabilities.allowed_frontend_slots = [
    "user.plugin_page.top",
    "user.plugin_page.bottom",
    "admin.plugin_page.top",
    "admin.plugin_page.bottom"
  ];
  manifestValue.capabilities = capabilities;

  syncPermissionCatalog(manifestValue, permissions, templatePermissionReason);

  return manifestValue;
}

function syncMarketManifest(sourceManifest) {
  const manifestValue = cloneManifest(sourceManifest);
  const capabilities = ensureRecord(manifestValue.capabilities);
  const permissions = [
    "hook.execute",
    "frontend.extensions",
    "frontend.html_trusted",
    "api.execute",
    "host.market.source.read",
    "host.market.catalog.read",
    "host.market.install.preview",
    "host.market.install.execute",
    "host.market.install.read",
    "host.market.install.rollback",
    "host.email_template.read",
    "host.email_template.write",
    "host.landing_page.read",
    "host.landing_page.write",
    "host.invoice_template.read",
    "host.invoice_template.write",
    "host.auth_branding.read",
    "host.auth_branding.write",
    "host.page_rule_pack.read",
    "host.page_rule_pack.write"
  ];

  applyHostCompatibility(manifestValue);
  capabilities.hooks = ["frontend.bootstrap"];
  capabilities.disabled_hooks = [];
  capabilities.allowed_frontend_slots = [
    "admin.plugin_page.top",
    "admin.plugin_page.bottom"
  ];
  manifestValue.capabilities = capabilities;

  syncPermissionCatalog(manifestValue, permissions, marketPermissionReason);

  return manifestValue;
}

function debuggerPermissionReason(key) {
  if (key === "frontend.html_trusted") {
    return "Enable trusted HTML diagnostics so the debugger can verify trusted/sanitized page behavior when administrators explicitly grant it.";
  }
  if (key.startsWith("host.market.")) {
    return "Enable Plugin.market.* demonstrations for trusted market source, catalog, install, task, and rollback workflows.";
  }
  if (key.startsWith("host.email_template.")) {
    return "Enable Plugin.emailTemplate.* demonstrations for native email template retrieval and save workflows.";
  }
  if (key.startsWith("host.landing_page.")) {
    return "Enable Plugin.landingPage.* demonstrations for native landing page retrieval and save workflows.";
  }
  if (key.startsWith("host.invoice_template.")) {
    return "Enable Plugin.invoiceTemplate.* demonstrations for native invoice template retrieval and save workflows.";
  }
  if (key.startsWith("host.auth_branding.")) {
    return "Enable Plugin.authBranding.* demonstrations for native auth branding retrieval and save workflows.";
  }
  if (key.startsWith("host.page_rule_pack.")) {
    return "Enable Plugin.pageRulePack.* demonstrations for native page rule pack retrieval and save workflows.";
  }
  return `Enable Plugin Debugger coverage for ${key}.`;
}

function templatePermissionReason(key) {
  const reasons = {
    "hook.execute": "Run inside the hook pipeline and handle manual execute actions.",
    "frontend.extensions": "Register the template plugin's dedicated frontend pages through frontend.bootstrap.",
    "api.execute": "Allow the plugin page's visual form and HTML bridge to call execute actions.",
    "host.order.read": "Demonstrate Plugin.order.get through the template plugin's native host lookup form.",
    "host.user.read": "Demonstrate Plugin.user.get through the template plugin's native host lookup form."
  };
  return reasons[key] || `Enable JS Worker template coverage for ${key}.`;
}

function marketPermissionReason(key) {
  const reasons = {
    "hook.execute": "通过 frontend.bootstrap 注册市场管理页并处理页面动作。",
    "frontend.extensions": "向管理端暴露市场路由和菜单入口。",
    "frontend.html_trusted": "在宿主支持且未强制 sanitize 时，以 trusted HTML 渲染市场混合工作台。",
    "api.execute": "允许市场页执行浏览、预览、安装、导入、任务查询和回滚动作。",
    "host.market.source.read": "读取并查看可信市场源。",
    "host.market.catalog.read": "浏览市场目录项和版本元数据。",
    "host.market.install.preview": "在执行前预览插件包安装结果。",
    "host.market.install.execute": "通过宿主桥执行可信市场安装，包括插件包和宿主管理模板。",
    "host.market.install.read": "读取市场资源对应的宿主安装任务与安装历史。",
    "host.market.install.rollback": "触发已安装市场资源的宿主管理回滚。",
    "host.email_template.read": "在导入市场模板版本前读取原生邮件模板目标。",
    "host.email_template.write": "导入市场模板版本后保存原生邮件模板。",
    "host.landing_page.read": "在导入市场落地页版本前读取原生落地页内容。",
    "host.landing_page.write": "导入市场落地页版本后保存原生落地页内容。",
    "host.invoice_template.read": "在导入市场账单模板版本前读取原生账单模板。",
    "host.invoice_template.write": "导入市场账单模板版本后保存或重置原生账单模板。",
    "host.auth_branding.read": "在导入市场认证页品牌模板版本前读取原生认证页品牌内容。",
    "host.auth_branding.write": "导入市场认证页品牌模板版本后保存或重置原生认证页品牌内容。",
    "host.page_rule_pack.read": "在导入市场页面规则包前读取原生页面规则内容。",
    "host.page_rule_pack.write": "导入市场页面规则包后保存或重置原生页面规则内容。"
  };
  return reasons[key] || `为市场插件启用 ${key} 能力。`;
}
