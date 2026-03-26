import {
  ADMIN_PLUGIN_PAGE_PATH,
  ALL_HOOK_OPTIONS,
  DEFAULT_FS_FORM_STATE,
  DEFAULT_HOST_FORM_STATE,
  DEFAULT_NETWORK_FORM_STATE,
  DEFAULT_SIMULATION_STATE,
  DEFAULT_STORAGE_FORM_STATE,
  DEFAULT_WORKER_FORM_STATE,
  PLUGIN_DISPLAY_NAME,
  SLOT_OPTIONS,
  USER_PLUGIN_PAGE_PATH
} from "./constants";
import {
  buildPluginAlertBlock,
  buildPluginActionFormBlock,
  buildPluginBadgeListBlock,
  buildPluginExecuteHTMLBridge,
  buildPluginHTMLBlock,
  buildPluginJSONViewBlock,
  buildPluginKeyValueBlock,
  buildPluginLinkListBlock,
  buildPluginStatsGridBlock,
  buildPluginTableBlock,
  OFFICIAL_PLUGIN_PERMISSION_KEYS,
  PLUGIN_EXECUTE_TEMPLATE_PLACEHOLDERS
} from "@auralogic/plugin-sdk";
import { latestEventSnapshot } from "./debugger-state";
import { renderDebuggerNpmComponentCard } from "./npm-demo-card";
import type {
  DebuggerEvent,
  DebuggerFileSummary,
  HostFormState,
  DebuggerNetworkResponse,
  DebuggerProfile,
  FileSystemFormState,
  HookExecutionResponse,
  NetworkFormState,
  PluginPageBlock,
  SimulatedHookState,
  StorageFormState,
  WorkerFormState
} from "./types";
import { parseJSONValueIfPossible, prettyJSON, truncateText } from "./utils";

function buildCollapsedJSONBlock(
  title: string,
  value: unknown,
  summary: string,
  previewLines = 8
): PluginPageBlock {
  return buildPluginJSONViewBlock({
    title,
    value,
    summary,
    collapsible: true,
    collapsed: true,
    preview_lines: previewLines,
    max_height: 480
  });
}

function buildStatsItems(profile: DebuggerProfile) {
  const lastFailure = profile.action_traces.find((trace) => !trace.ok);
  return [
    {
      label: "Enabled Hooks",
      value: profile.enabled_hooks.length,
      description: profile.source
    },
    {
      label: "Recent Events",
      value: profile.recent_events.length,
      description: profile.recent_events[0]?.hook || "No hook captured yet"
    },
    {
      label: "Debug Actions",
      value: profile.action_traces.length,
      description: profile.action_traces[0]?.action || "No debugger action recorded yet"
    },
    {
      label: "Warnings",
      value: profile.warnings.length,
      description: lastFailure?.action || "No recent debugger failure"
    },
    {
      label: "Storage Keys",
      value: profile.storage.key_count,
      description: profile.storage.lab_keys.length > 0 ? profile.storage.lab_keys.join(", ") : "Only reserved keys"
    },
    {
      label: "Storage Access",
      value: profile.sandbox.storageAccessMode,
      description: profile.sandbox.currentAction
        ? `${profile.sandbox.currentAction} / declared ${profile.sandbox.declaredStorageAccessMode}`
        : `${Object.keys(profile.sandbox.executeActionStorage).length} action profiles declared`
    },
    {
      label: "Filesystem",
      value: profile.fs.enabled ? "enabled" : "disabled",
      description: profile.fs.probe_error
        ? truncateText(profile.fs.probe_error, 96)
        : profile.fs.usage
          ? `${profile.fs.usage.file_count}/${profile.fs.usage.max_files} files`
          : truncateText(profile.runtime.interpretation, 96)
    },
    {
      label: "Network",
      value: profile.runtime.pluginHTTPEnabled ? "enabled" : "disabled",
      description: truncateText(profile.runtime.networkInterpretation, 96)
    },
    {
      label: "Host Bridge",
      value: profile.runtime.pluginHostEnabled ? "enabled" : "disabled",
      description: truncateText(profile.runtime.hostInterpretation, 96)
    },
    {
      label: "Workspace",
      value: profile.runtime.pluginWorkspaceEnabled ? "enabled" : "disabled",
      description: truncateText(profile.runtime.workspaceInterpretation, 96)
    },
    {
      label: "Secret",
      value: profile.secret.enabled ? "enabled" : profile.secret.present ? "present" : "missing",
      description: truncateText(profile.secret.interpretation, 96)
    },
    {
      label: "Webhook",
      value: profile.webhook.present ? profile.webhook.method || "attached" : "inactive",
      description: truncateText(profile.webhook.interpretation, 96)
    }
  ];
}

function buildCapabilityItems(profile: DebuggerProfile) {
  return [
    { key: "level", label: "Sandbox Level", value: profile.sandbox.level },
    { key: "currentAction", label: "Current Action", value: profile.sandbox.currentAction || "-" },
    {
      key: "declaredStorageAccessMode",
      label: "Declared Storage Access",
      value: profile.sandbox.declaredStorageAccessMode
    },
    {
      key: "storageAccessMode",
      label: "Observed Storage Access",
      value: profile.sandbox.storageAccessMode
    },
    { key: "allowHookExecute", label: "Hook Execute", value: profile.sandbox.allowHookExecute },
    { key: "allowHookBlock", label: "Hook Block", value: profile.sandbox.allowHookBlock },
    { key: "allowPayloadPatch", label: "Payload Patch", value: profile.sandbox.allowPayloadPatch },
    {
      key: "allowFrontendExtensions",
      label: "Frontend Extensions",
      value: profile.sandbox.allowFrontendExtensions
    },
    { key: "allowExecuteAPI", label: "Admin Execute API", value: profile.sandbox.allowExecuteAPI },
    { key: "allowFileSystem", label: "File System", value: profile.sandbox.allowFileSystem },
    { key: "allowNetwork", label: "Network", value: profile.sandbox.allowNetwork },
    { key: "defaultTimeoutMs", label: "Timeout(ms)", value: profile.sandbox.defaultTimeoutMs },
    { key: "maxConcurrency", label: "Concurrency", value: profile.sandbox.maxConcurrency },
    { key: "maxMemoryMB", label: "Memory(MB)", value: profile.sandbox.maxMemoryMB },
    { key: "storageMaxKeys", label: "Storage Keys", value: profile.sandbox.storageMaxKeys },
    { key: "storageMaxValueBytes", label: "Storage Value Bytes", value: profile.sandbox.storageMaxValueBytes },
    {
      key: "executeActionStorageCount",
      label: "Storage Profiles",
      value: Object.keys(profile.sandbox.executeActionStorage).length
    },
    { key: "fsMaxReadBytes", label: "FS Read Bytes", value: profile.sandbox.fsMaxReadBytes }
  ];
}

function buildRuntimeProbeItems(profile: DebuggerProfile) {
  const items: Array<Record<string, unknown>> = [
    {
      key: "sandboxAllowFileSystem",
      label: "sandbox.allowFileSystem",
      value: profile.runtime.sandboxAllowFileSystem
    },
    {
      key: "sandboxAllowNetwork",
      label: "sandbox.allowNetwork",
      value: profile.runtime.sandboxAllowNetwork
    },
    { key: "hasWorkerGlobal", label: "Worker Global", value: profile.runtime.hasWorkerGlobal },
    {
      key: "hasStructuredCloneGlobal",
      label: "structuredClone Global",
      value: profile.runtime.hasStructuredCloneGlobal
    },
    {
      key: "hasQueueMicrotaskGlobal",
      label: "queueMicrotask Global",
      value: profile.runtime.hasQueueMicrotaskGlobal
    },
    { key: "hasSetTimeoutGlobal", label: "setTimeout Global", value: profile.runtime.hasSetTimeoutGlobal },
    { key: "hasClearTimeoutGlobal", label: "clearTimeout Global", value: profile.runtime.hasClearTimeoutGlobal },
    { key: "hasTextEncoderGlobal", label: "TextEncoder Global", value: profile.runtime.hasTextEncoderGlobal },
    { key: "hasTextDecoderGlobal", label: "TextDecoder Global", value: profile.runtime.hasTextDecoderGlobal },
    { key: "hasAtobGlobal", label: "atob Global", value: profile.runtime.hasAtobGlobal },
    { key: "hasBtoaGlobal", label: "btoa Global", value: profile.runtime.hasBtoaGlobal },
    { key: "pluginHostEnabled", label: "Plugin.host.enabled", value: profile.runtime.pluginHostEnabled },
    { key: "pluginHTTPEnabled", label: "Plugin.http.enabled", value: profile.runtime.pluginHTTPEnabled },
    { key: "pluginFSEnabled", label: "Plugin.fs.enabled", value: profile.runtime.pluginFSEnabled },
    { key: "pluginWorkspaceEnabled", label: "Plugin.workspace.enabled", value: profile.runtime.pluginWorkspaceEnabled },
    { key: "pluginSecretEnabled", label: "Plugin.secret.enabled", value: profile.runtime.pluginSecretEnabled },
    { key: "pluginWebhookEnabled", label: "Plugin.webhook.enabled", value: profile.runtime.pluginWebhookEnabled },
    { key: "workspaceEntryCount", label: "Workspace Entry Count", value: profile.runtime.workspaceEntryCount },
    { key: "workspaceMaxEntries", label: "Workspace Max Entries", value: profile.runtime.workspaceMaxEntries },
    { key: "httpDefaultTimeoutMs", label: "HTTP Default Timeout", value: profile.runtime.httpDefaultTimeoutMs },
    { key: "httpMaxResponseBytes", label: "HTTP Max Response Bytes", value: profile.runtime.httpMaxResponseBytes },
    { key: "dataRoot", label: "Data Root", value: profile.runtime.dataRoot || "-" },
    { key: "runtimeGlobalKeys", label: "JS Runtime Globals", value: profile.runtime.runtimeGlobalKeys.join(", ") || "-" },
    { key: "pluginGlobalKeys", label: "Plugin Global Keys", value: profile.runtime.pluginGlobalKeys.join(", ") || "-" },
    { key: "jsRuntimeInterpretation", label: "JS Runtime Interpretation", value: profile.runtime.jsRuntimeInterpretation },
    { key: "workspaceInterpretation", label: "Workspace Interpretation", value: profile.runtime.workspaceInterpretation },
    { key: "fsInterpretation", label: "FS Interpretation", value: profile.runtime.interpretation },
    { key: "networkInterpretation", label: "Network Interpretation", value: profile.runtime.networkInterpretation },
    { key: "hostInterpretation", label: "Host Interpretation", value: profile.runtime.hostInterpretation }
  ];

  if (!profile.runtime.hasPluginWorkspace) {
    items.unshift({ key: "hasPluginWorkspace", label: "Plugin.workspace Present", value: false });
  }
  if (!profile.runtime.hasPluginWebhook) {
    items.unshift({ key: "hasPluginWebhook", label: "Plugin.webhook Present", value: false });
  }
  if (!profile.runtime.hasPluginSecret) {
    items.unshift({ key: "hasPluginSecret", label: "Plugin.secret Present", value: false });
  }
  if (!profile.runtime.hasPluginUser) {
    items.unshift({ key: "hasPluginUser", label: "Plugin.user Present", value: false });
  }
  if (!profile.runtime.hasPluginOrder) {
    items.unshift({ key: "hasPluginOrder", label: "Plugin.order Present", value: false });
  }
  if (!profile.runtime.hasPluginProduct) {
    items.unshift({ key: "hasPluginProduct", label: "Plugin.product Present", value: false });
  }
  if (!profile.runtime.hasPluginInventory) {
    items.unshift({ key: "hasPluginInventory", label: "Plugin.inventory Present", value: false });
  }
  if (!profile.runtime.hasPluginInventoryBinding) {
    items.unshift({ key: "hasPluginInventoryBinding", label: "Plugin.inventoryBinding Present", value: false });
  }
  if (!profile.runtime.hasPluginPromo) {
    items.unshift({ key: "hasPluginPromo", label: "Plugin.promo Present", value: false });
  }
  if (!profile.runtime.hasPluginTicket) {
    items.unshift({ key: "hasPluginTicket", label: "Plugin.ticket Present", value: false });
  }
  if (!profile.runtime.hasPluginSerial) {
    items.unshift({ key: "hasPluginSerial", label: "Plugin.serial Present", value: false });
  }
  if (!profile.runtime.hasPluginAnnouncement) {
    items.unshift({ key: "hasPluginAnnouncement", label: "Plugin.announcement Present", value: false });
  }
  if (!profile.runtime.hasPluginKnowledge) {
    items.unshift({ key: "hasPluginKnowledge", label: "Plugin.knowledge Present", value: false });
  }
  if (!profile.runtime.hasPluginPaymentMethod) {
    items.unshift({ key: "hasPluginPaymentMethod", label: "Plugin.paymentMethod Present", value: false });
  }
  if (!profile.runtime.hasPluginVirtualInventory) {
    items.unshift({ key: "hasPluginVirtualInventory", label: "Plugin.virtualInventory Present", value: false });
  }
  if (!profile.runtime.hasPluginVirtualInventoryBinding) {
    items.unshift({ key: "hasPluginVirtualInventoryBinding", label: "Plugin.virtualInventoryBinding Present", value: false });
  }
  if (!profile.runtime.hasPluginMarket) {
    items.unshift({ key: "hasPluginMarket", label: "Plugin.market Present", value: false });
  }
  if (!profile.runtime.hasPluginEmailTemplate) {
    items.unshift({ key: "hasPluginEmailTemplate", label: "Plugin.emailTemplate Present", value: false });
  }
  if (!profile.runtime.hasPluginLandingPage) {
    items.unshift({ key: "hasPluginLandingPage", label: "Plugin.landingPage Present", value: false });
  }
  if (!profile.runtime.hasPluginInvoiceTemplate) {
    items.unshift({ key: "hasPluginInvoiceTemplate", label: "Plugin.invoiceTemplate Present", value: false });
  }
  if (!profile.runtime.hasPluginAuthBranding) {
    items.unshift({ key: "hasPluginAuthBranding", label: "Plugin.authBranding Present", value: false });
  }
  if (!profile.runtime.hasPluginPageRulePack) {
    items.unshift({ key: "hasPluginPageRulePack", label: "Plugin.pageRulePack Present", value: false });
  }
  if (!profile.runtime.hasPluginHost) {
    items.unshift({ key: "hasPluginHost", label: "Plugin.host Present", value: false });
  }
  if (!profile.runtime.hasPluginFS) {
    items.unshift({ key: "hasPluginFS", label: "Plugin.fs Present", value: false });
  }
  if (!profile.runtime.hasPluginHTTP) {
    items.unshift({ key: "hasPluginHTTP", label: "Plugin.http Present", value: false });
  }
  if (!profile.runtime.hasPluginStorage) {
    items.unshift({ key: "hasPluginStorage", label: "Plugin.storage Present", value: false });
  }
  if (profile.runtime.workspaceProbeError) {
    items.splice(items.length - 4, 0, {
      key: "workspaceProbeError",
      label: "Workspace Probe Error",
      value: profile.runtime.workspaceProbeError
    });
  }
  if (profile.runtime.fsProbeError) {
    items.splice(items.length - 1, 0, {
      key: "fsProbeError",
      label: "FS Probe Error",
      value: profile.runtime.fsProbeError
    });
  }

  return items;
}

function buildRecentEventRows(events: DebuggerEvent[]): Record<string, unknown>[] {
  return events.map((event) => ({
    time: event.ts,
    hook: event.hook,
    group: event.group || "-",
    area: event.area || "-",
    slot: event.slot || "-",
    path: event.path || "-",
    user_id: event.user_id ?? "-",
    order_id: event.order_id ?? "-",
    blocked: event.blocked ? "yes" : "no",
    note: event.note || "-"
  }));
}

function buildFileRows(summary: DebuggerFileSummary): Record<string, unknown>[] {
  return summary.entries.map((entry) => ({
    path: entry.path,
    name: entry.name,
    is_dir: entry.is_dir ? "yes" : "no",
    size: entry.size,
    mod_time: entry.mod_time
  }));
}

function buildHeaderRows(headers: Record<string, string>): Record<string, unknown>[] {
  return Object.entries(headers)
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([key, value]) => ({
      key,
      value
    }));
}

function buildExecuteActionStorageRows(profile: DebuggerProfile): Record<string, unknown>[] {
  return Object.entries(profile.sandbox.executeActionStorage)
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([action, mode]) => ({
      action,
      mode,
      current: action === profile.sandbox.currentAction ? "yes" : ""
    }));
}

function buildPermissionCoverageRows(profile: DebuggerProfile): Record<string, unknown>[] {
  const requestedSet = new Set(profile.sandbox.requestedPermissions);
  const grantedSet = new Set(profile.sandbox.grantedPermissions);
  return Array.from(new Set([...requestedSet, ...grantedSet]))
    .sort((left, right) => left.localeCompare(right))
    .map((permission) => {
      const segments = permission.split(".");
      const scope = segments[0] || "custom";
      const subject = segments.slice(0, Math.max(segments.length - 1, 1)).join(".");
      return {
        permission,
        requested: requestedSet.has(permission) ? "yes" : "no",
        granted: grantedSet.has(permission) ? "yes" : "no",
        scope,
        subject
      };
    });
}

function buildHelperCoverageRows(profile: DebuggerProfile): Record<string, unknown>[] {
  return [
    { helper: "Plugin.storage", present: profile.runtime.hasPluginStorage ? "yes" : "no", enabled: "n/a", note: "Plugin.storage bridge" },
    { helper: "Plugin.workspace", present: profile.runtime.hasPluginWorkspace ? "yes" : "no", enabled: profile.runtime.pluginWorkspaceEnabled ? "yes" : "no", note: truncateText(profile.runtime.workspaceInterpretation, 120) },
    { helper: "Plugin.secret", present: profile.runtime.hasPluginSecret ? "yes" : "no", enabled: profile.runtime.pluginSecretEnabled ? "yes" : "no", note: truncateText(profile.secret.interpretation, 120) },
    { helper: "Plugin.webhook", present: profile.runtime.hasPluginWebhook ? "yes" : "no", enabled: profile.runtime.pluginWebhookEnabled ? "yes" : "no", note: truncateText(profile.webhook.interpretation, 120) },
    { helper: "Plugin.fs", present: profile.runtime.hasPluginFS ? "yes" : "no", enabled: profile.runtime.pluginFSEnabled ? "yes" : "no", note: truncateText(profile.runtime.interpretation, 120) },
    { helper: "Plugin.http", present: profile.runtime.hasPluginHTTP ? "yes" : "no", enabled: profile.runtime.pluginHTTPEnabled ? "yes" : "no", note: truncateText(profile.runtime.networkInterpretation, 120) },
    { helper: "Plugin.host", present: profile.runtime.hasPluginHost ? "yes" : "no", enabled: profile.runtime.pluginHostEnabled ? "yes" : "no", note: truncateText(profile.runtime.hostInterpretation, 120) },
    { helper: "Plugin.order", present: profile.runtime.hasPluginOrder ? "yes" : "no", enabled: profile.runtime.hasPluginOrder ? "yes" : "no", note: "Typed host helper" },
    { helper: "Plugin.user", present: profile.runtime.hasPluginUser ? "yes" : "no", enabled: profile.runtime.hasPluginUser ? "yes" : "no", note: "Typed host helper" },
    { helper: "Plugin.product", present: profile.runtime.hasPluginProduct ? "yes" : "no", enabled: profile.runtime.hasPluginProduct ? "yes" : "no", note: "Typed host helper" },
    { helper: "Plugin.inventory", present: profile.runtime.hasPluginInventory ? "yes" : "no", enabled: profile.runtime.hasPluginInventory ? "yes" : "no", note: "Typed host helper" },
    { helper: "Plugin.inventoryBinding", present: profile.runtime.hasPluginInventoryBinding ? "yes" : "no", enabled: profile.runtime.hasPluginInventoryBinding ? "yes" : "no", note: "Typed host helper" },
    { helper: "Plugin.promo", present: profile.runtime.hasPluginPromo ? "yes" : "no", enabled: profile.runtime.hasPluginPromo ? "yes" : "no", note: "Typed host helper" },
    { helper: "Plugin.ticket", present: profile.runtime.hasPluginTicket ? "yes" : "no", enabled: profile.runtime.hasPluginTicket ? "yes" : "no", note: "Typed host helper" },
    { helper: "Plugin.serial", present: profile.runtime.hasPluginSerial ? "yes" : "no", enabled: profile.runtime.hasPluginSerial ? "yes" : "no", note: "Typed host helper" },
    { helper: "Plugin.announcement", present: profile.runtime.hasPluginAnnouncement ? "yes" : "no", enabled: profile.runtime.hasPluginAnnouncement ? "yes" : "no", note: "Typed host helper" },
    { helper: "Plugin.knowledge", present: profile.runtime.hasPluginKnowledge ? "yes" : "no", enabled: profile.runtime.hasPluginKnowledge ? "yes" : "no", note: "Typed host helper" },
    { helper: "Plugin.paymentMethod", present: profile.runtime.hasPluginPaymentMethod ? "yes" : "no", enabled: profile.runtime.hasPluginPaymentMethod ? "yes" : "no", note: "Typed host helper" },
    { helper: "Plugin.virtualInventory", present: profile.runtime.hasPluginVirtualInventory ? "yes" : "no", enabled: profile.runtime.hasPluginVirtualInventory ? "yes" : "no", note: "Typed host helper" },
    { helper: "Plugin.virtualInventoryBinding", present: profile.runtime.hasPluginVirtualInventoryBinding ? "yes" : "no", enabled: profile.runtime.hasPluginVirtualInventoryBinding ? "yes" : "no", note: "Typed host helper" },
    { helper: "Plugin.market", present: profile.runtime.hasPluginMarket ? "yes" : "no", enabled: profile.runtime.hasPluginMarket ? "yes" : "no", note: "Typed host helper" },
    { helper: "Plugin.emailTemplate", present: profile.runtime.hasPluginEmailTemplate ? "yes" : "no", enabled: profile.runtime.hasPluginEmailTemplate ? "yes" : "no", note: "Typed host helper" },
    { helper: "Plugin.landingPage", present: profile.runtime.hasPluginLandingPage ? "yes" : "no", enabled: profile.runtime.hasPluginLandingPage ? "yes" : "no", note: "Typed host helper" },
    { helper: "Plugin.invoiceTemplate", present: profile.runtime.hasPluginInvoiceTemplate ? "yes" : "no", enabled: profile.runtime.hasPluginInvoiceTemplate ? "yes" : "no", note: "Typed host helper" },
    { helper: "Plugin.authBranding", present: profile.runtime.hasPluginAuthBranding ? "yes" : "no", enabled: profile.runtime.hasPluginAuthBranding ? "yes" : "no", note: "Typed host helper" },
    { helper: "Plugin.pageRulePack", present: profile.runtime.hasPluginPageRulePack ? "yes" : "no", enabled: profile.runtime.hasPluginPageRulePack ? "yes" : "no", note: "Typed host helper" },
    { helper: "Worker", present: profile.runtime.hasWorkerGlobal ? "yes" : "no", enabled: "n/a", note: "Parallel child JS runtime constructor" },
    { helper: "structuredClone", present: profile.runtime.hasStructuredCloneGlobal ? "yes" : "no", enabled: "n/a", note: "Deep clone helper for JSON-serializable values" },
    { helper: "queueMicrotask", present: profile.runtime.hasQueueMicrotaskGlobal ? "yes" : "no", enabled: "n/a", note: "Microtask scheduler inside the current invocation" },
    { helper: "setTimeout", present: profile.runtime.hasSetTimeoutGlobal ? "yes" : "no", enabled: "n/a", note: "Timer scheduler inside the current invocation" },
    { helper: "clearTimeout", present: profile.runtime.hasClearTimeoutGlobal ? "yes" : "no", enabled: "n/a", note: "Timer cancellation helper" },
    { helper: "TextEncoder", present: profile.runtime.hasTextEncoderGlobal ? "yes" : "no", enabled: "n/a", note: "UTF-8 encoder helper" },
    { helper: "TextDecoder", present: profile.runtime.hasTextDecoderGlobal ? "yes" : "no", enabled: "n/a", note: "UTF-8 decoder helper" },
    { helper: "atob", present: profile.runtime.hasAtobGlobal ? "yes" : "no", enabled: "n/a", note: "Base64 decode helper" },
    { helper: "btoa", present: profile.runtime.hasBtoaGlobal ? "yes" : "no", enabled: "n/a", note: "Base64 encode helper" }
  ];
}

function buildActionTraceRows(profile: DebuggerProfile): Record<string, unknown>[] {
  return profile.action_traces.map((trace) => ({
    time: trace.ts,
    status: trace.ok ? "ok" : "error",
    category: trace.category,
    action: trace.action,
    current_action: trace.current_action || "-",
    request: trace.request_summary || "-",
    response: trace.response_summary || "-"
  }));
}

export function buildProfileSummaryBlocks(profile: DebuggerProfile): PluginPageBlock[] {
  const latestFailure = profile.action_traces.find((trace) => !trace.ok);
  const blocks: PluginPageBlock[] = [
    buildPluginStatsGridBlock({
      title: "Runtime Snapshot",
      items: buildStatsItems(profile)
    }),
    buildPluginBadgeListBlock({
      title: "Granted Permissions",
      items:
        profile.sandbox.grantedPermissions.length > 0
          ? profile.sandbox.grantedPermissions
          : ["no-granted-permissions"]
    }),
    buildPluginBadgeListBlock({
      title: "Enabled Hook Groups",
      items: [
        profile.config.enable_frontend ? "frontend" : "",
        profile.config.enable_auth ? "auth" : "",
        profile.config.enable_platform ? "platform" : "",
        profile.config.enable_commerce ? "commerce" : "",
        profile.config.enable_catalog ? "catalog" : "",
        profile.config.enable_support ? "support" : "",
        profile.config.enable_content ? "content" : "",
        profile.config.enable_settings ? "settings" : ""
      ].filter(Boolean)
    }),
    buildPluginKeyValueBlock({
      title: "Sandbox Capabilities",
      items: buildCapabilityItems(profile).map((item) => ({
        key: String(item.key || ""),
        label: String(item.label || ""),
        value: item.value
      }))
    }),
    buildPluginKeyValueBlock({
      title: "Runtime Probe",
      items: buildRuntimeProbeItems(profile).map((item) => ({
        key: String(item.key || ""),
        label: String(item.label || ""),
        value: item.value
      }))
    }),
    buildPluginKeyValueBlock({
      title: "Plugin Page Context",
      items: [
        { key: "area", label: "Area", value: profile.page_context.area || "-" },
        { key: "path", label: "Path", value: profile.page_context.path || "-" },
        { key: "full_path", label: "Full Path", value: profile.page_context.full_path || "-" },
        { key: "query_string", label: "Query String", value: profile.page_context.query_string || "-" },
        { key: "query_count", label: "Query Param Count", value: Object.keys(profile.page_context.query_params).length },
        { key: "route_count", label: "Route Param Count", value: Object.keys(profile.page_context.route_params).length }
      ]
    }),
    buildCollapsedJSONBlock(
      "Plugin Page Query Params",
      profile.page_context.query_params,
      "Structured query params resolved from the current plugin page execution context."
    ),
    buildCollapsedJSONBlock(
      "Plugin Page Route Params",
      profile.page_context.route_params,
      "Structured route params resolved from the current plugin page execution context."
    ),
    buildPluginKeyValueBlock({
      title: "Secret Snapshot",
      items: [
        { key: "present", label: "Plugin.secret Present", value: profile.secret.present },
        { key: "enabled", label: "Plugin.secret Enabled", value: profile.secret.enabled },
        { key: "key_count", label: "Configured Secret Keys", value: profile.secret.key_count },
        { key: "sample_key", label: "Sample Secret Key", value: profile.secret.sample_key },
        { key: "sample_present", label: "Sample Secret Present", value: profile.secret.sample_present },
        { key: "interpretation", label: "Interpretation", value: profile.secret.interpretation },
        ...(profile.secret.error ? [{ key: "error", label: "Error", value: profile.secret.error }] : [])
      ]
    }),
    buildPluginBadgeListBlock({
      title: "Secret Keys",
      items: profile.secret.keys.length > 0 ? profile.secret.keys : ["no-secret-keys"]
    }),
    buildPluginKeyValueBlock({
      title: "Webhook Snapshot",
      items: [
        { key: "present", label: "Plugin.webhook Present", value: profile.webhook.present },
        { key: "enabled", label: "Plugin.webhook Enabled", value: profile.webhook.enabled },
        { key: "key", label: "Webhook Key", value: profile.webhook.key || "-" },
        { key: "method", label: "Method", value: profile.webhook.method || "-" },
        { key: "path", label: "Path", value: profile.webhook.path || "-" },
        { key: "content_type", label: "Content Type", value: profile.webhook.content_type || "-" },
        { key: "remote_addr", label: "Remote Addr", value: profile.webhook.remote_addr || "-" },
        { key: "header_count", label: "Header Count", value: profile.webhook.header_count },
        { key: "interpretation", label: "Interpretation", value: profile.webhook.interpretation }
      ]
    }),
    buildCollapsedJSONBlock(
      "Webhook Query Params",
      profile.webhook.query_params,
      "Structured webhook query params captured from the current inbound request."
    ),
    buildCollapsedJSONBlock(
      "Webhook Headers",
      profile.webhook.headers,
      "Inbound webhook headers visible to the current runtime."
    ),
    buildCollapsedJSONBlock(
      "Webhook Body Preview",
      profile.webhook.body_json_preview !== undefined ? profile.webhook.body_json_preview : profile.webhook.body_text_preview,
      "Parsed webhook body preview when JSON decoding succeeds, otherwise raw text preview."
    ),
    buildPluginTableBlock({
      title: "Execute Action Storage Profiles",
      content: profile.sandbox.currentAction
        ? `Current action ${profile.sandbox.currentAction} is highlighted below. These declarations drive host-side Plugin.storage locking and validation.`
        : "Per-action Plugin.storage declarations resolved from capabilities.execute_action_storage.",
      columns: ["action", "mode", "current"],
      rows: buildExecuteActionStorageRows(profile),
      empty_text: "No execute_action_storage profiles declared"
    }),
    buildPluginTableBlock({
      title: "Permission Coverage",
      content: `Requested/granted matrix for the ${buildPermissionCoverageRows(profile).length} permissions currently in play.`,
      columns: ["permission", "requested", "granted", "scope", "subject"],
      rows: buildPermissionCoverageRows(profile),
      empty_text: "No permission coverage rows"
    }),
    buildPluginTableBlock({
      title: "Runtime Helper Coverage",
      content: "Presence and enablement status for runtime bridges and typed host helpers.",
      columns: ["helper", "present", "enabled", "note"],
      rows: buildHelperCoverageRows(profile),
      empty_text: "No runtime helper coverage rows"
    })
  ];

  if (profile.capability_gaps.length > 0) {
    blocks.push(buildPluginAlertBlock({
      title: "Capability Gaps",
      content: profile.capability_gaps.join("\n"),
      variant: "warning"
    }));
  }
  if (profile.warnings.length > 0) {
    blocks.push(buildPluginAlertBlock({
      title: "Runtime Notes",
      content: profile.warnings.join("\n"),
      variant: "info"
    }));
  }

  blocks.push(buildPluginTableBlock({
    title: "Recent Hook Events",
    content:
      profile.recent_events.length > 0
        ? "Latest real hook executions captured by Plugin Debugger."
        : "No hook events captured yet. Trigger login/order/ticket/frontend activity, then refresh.",
    columns: [
      "time",
      "hook",
      "group",
      "area",
      "slot",
      "path",
      "user_id",
      "order_id",
      "blocked",
      "note"
    ],
    rows: buildRecentEventRows(profile.recent_events),
    empty_text: "No hook events yet"
  }));

  blocks.push(buildPluginTableBlock({
    title: "Recent Debug Actions",
    content:
      profile.action_traces.length > 0
        ? "Most recent debugger execute/workspace actions captured by Plugin Debugger."
        : "No debugger action traces yet. Run self-test, host requests, network lab, or workspace console helpers to populate this table.",
    columns: ["time", "status", "category", "action", "current_action", "request", "response"],
    rows: buildActionTraceRows(profile),
    empty_text: "No debugger action traces yet"
  }));

  const latest = latestEventSnapshot(profile.recent_events);
  if (latest) {
    blocks.push(buildCollapsedJSONBlock(
      "Latest Hook Snapshot",
      latest,
      "Raw snapshot of the most recent captured hook event for low-level debugger inspection."
    ));
  }
  if (latestFailure) {
    blocks.push(buildCollapsedJSONBlock(
      "Latest Failed Debug Action",
      latestFailure,
      "Structured snapshot of the most recent failed debugger execute/workspace action."
    ));
  }

  return blocks;
}

export function buildSelfTestBlocks(
  profile: DebuggerProfile,
  message = "Debugger self-test completed."
): PluginPageBlock[] {
  const latestFailure = profile.action_traces.find((trace) => !trace.ok);
  const overallOK = profile.capability_gaps.length === 0 && profile.warnings.length === 0;
  return [
    buildPluginAlertBlock({
      title: "Debugger Self-Test",
      content: message,
      variant: overallOK ? "success" : "warning"
    }),
    buildPluginStatsGridBlock({
      title: "Self-Test Summary",
      items: [
        { label: "Runtime Warnings", value: profile.warnings.length, description: overallOK ? "No runtime warnings detected." : "Review Runtime Notes below." },
        { label: "Capability Gaps", value: profile.capability_gaps.length, description: profile.capability_gaps.join(", ") || "All requested permissions are granted." },
        { label: "Permission Coverage", value: buildPermissionCoverageRows(profile).length, description: `${profile.sandbox.grantedPermissions.length}/${OFFICIAL_PLUGIN_PERMISSION_KEYS.length} catalog permissions granted` },
        { label: "Runtime Helpers", value: buildHelperCoverageRows(profile).filter((row) => row.present === "yes").length, description: "Helpers currently present in runtime." },
        { label: "Secret Keys", value: profile.secret.key_count, description: profile.secret.sample_present ? "Sample secret is configured." : "Sample secret is not configured." },
        { label: "Webhook", value: profile.webhook.present ? profile.webhook.method || "attached" : "inactive", description: profile.webhook.path || "No live webhook request attached." },
        { label: "Recent Actions", value: profile.action_traces.length, description: profile.action_traces[0]?.action || "No action trace yet." },
        { label: "Last Failure", value: latestFailure ? "present" : "none", description: latestFailure?.action || "No recent failed debugger action." }
      ]
    }),
    ...buildProfileSummaryBlocks(profile)
  ];
}

export function buildStorageBlocks(
  profile: DebuggerProfile,
  values: StorageFormState,
  message?: string
): PluginPageBlock[] {
  const blocks: PluginPageBlock[] = [];
  if (message) {
    blocks.push(buildPluginAlertBlock({
      title: "Storage Result",
      content: message,
      variant: "info"
    }));
  }
  blocks.push(buildPluginKeyValueBlock({
    title: "Storage Summary",
    items: [
      { label: "Current Action", value: profile.sandbox.currentAction || "-" },
      { label: "Declared Access", value: profile.sandbox.declaredStorageAccessMode },
      { label: "Observed Access", value: profile.sandbox.storageAccessMode },
      { label: "Reserved Keys", value: profile.storage.reserved_keys.join(", ") || "-" },
      { label: "Lab Keys", value: profile.storage.lab_keys.join(", ") || "-" },
      { label: "Selected Key", value: values.storage_key || "-" }
    ]
  }));
  blocks.push(buildPluginJSONViewBlock({
    title: "Selected Storage Value",
    value: parseJSONValueIfPossible(values.storage_value)
  }));
  blocks.push(buildPluginTableBlock({
    title: "All Storage Keys",
    columns: ["key", "reserved"],
    rows: profile.storage.keys.map((key) => ({
      key,
      reserved: profile.storage.reserved_keys.includes(key) ? "yes" : "no"
    })),
    empty_text: "No storage keys"
  }));
  return blocks;
}

export function buildFileBlocks(
  summary: DebuggerFileSummary,
  values: FileSystemFormState,
  message?: string
): PluginPageBlock[] {
  const blocks: PluginPageBlock[] = [];
  if (message) {
    blocks.push(buildPluginAlertBlock({
      title: "Filesystem Result",
      content: message,
      variant: summary.enabled ? "info" : "warning"
    }));
  }
  blocks.push(buildPluginKeyValueBlock({
    title: "Filesystem Summary",
    items: [
      { label: "Enabled", value: summary.enabled },
      { label: "Selected Path", value: values.fs_path || "-" },
      { label: "Format", value: values.fs_format },
      { label: "File Count", value: summary.usage?.file_count ?? "-" },
      { label: "Total Bytes", value: summary.usage?.total_bytes ?? "-" },
      { label: "Max Files", value: summary.usage?.max_files ?? "-" },
      { label: "Quota", value: summary.usage?.max_bytes ?? "-" },
      { label: "Max Read Bytes", value: summary.max_read_bytes ?? "-" },
      { label: "Data Root", value: summary.data_root || "-" },
      ...(summary.probe_error ? [{ label: "Probe Error", value: summary.probe_error }] : [])
    ]
  }));
  blocks.push(buildPluginJSONViewBlock({
    title: "Selected File Content",
    value: values.fs_format === "json" ? parseJSONValueIfPossible(values.fs_content) : values.fs_content
  }));
  blocks.push(buildPluginTableBlock({
    title: "Filesystem Entries",
    columns: ["path", "name", "is_dir", "size", "mod_time"],
    rows: buildFileRows(summary),
    empty_text: summary.enabled ? "No files inside plugin data root" : "Plugin.fs disabled"
  }));
  return blocks;
}

export function buildNetworkBlocks(
  profile: DebuggerProfile,
  values: NetworkFormState,
  response: DebuggerNetworkResponse,
  message?: string
): PluginPageBlock[] {
  const blocks: PluginPageBlock[] = [];
  if (message) {
    blocks.push(buildPluginAlertBlock({
      title: "Network Result",
      content: message,
      variant: response.ok ? "success" : "warning"
    }));
  }
  blocks.push(buildPluginKeyValueBlock({
    title: "Network Request",
    items: [
      { label: "Enabled", value: profile.runtime.pluginHTTPEnabled },
      { label: "Method", value: values.network_method },
      { label: "URL", value: values.network_url || "-" },
      { label: "Timeout (ms)", value: values.network_timeout_ms || profile.runtime.httpDefaultTimeoutMs || "-" },
      { label: "Body Format", value: values.network_body_format }
    ]
  }));
  blocks.push(buildCollapsedJSONBlock(
    "Request Headers",
    parseJSONValueIfPossible(values.network_headers || "{}"),
    "Raw outbound request headers after JSON parsing.",
    6
  ));
  blocks.push(buildCollapsedJSONBlock(
    "Request Body",
    values.network_body_format === "json" ? parseJSONValueIfPossible(values.network_body) : values.network_body,
    "Raw outbound request body submitted by the debugger network lab.",
    8
  ));
  blocks.push(buildPluginKeyValueBlock({
    title: "Network Response",
    items: [
      { label: "OK", value: response.ok },
      { label: "Status", value: response.status || "-" },
      { label: "Status Text", value: response.statusText || "-" },
      { label: "Duration (ms)", value: response.duration_ms },
      { label: "Redirected", value: response.redirected ? "yes" : "no" },
      { label: "Final URL", value: response.url || values.network_url || "-" },
      { label: "Error", value: response.error || "-" }
    ]
  }));
  blocks.push(buildPluginTableBlock({
    title: "Response Headers",
    columns: ["key", "value"],
    rows: buildHeaderRows(response.headers),
    empty_text: "No response headers"
  }));
  if (response.data !== undefined) {
    blocks.push(buildCollapsedJSONBlock(
      "Response Data",
      response.data,
      "Parsed response payload when the host could decode the response body."
    ));
  }
  blocks.push(buildCollapsedJSONBlock(
    "Response Body",
    parseJSONValueIfPossible(response.body),
    "Raw response body as returned by the network bridge."
  ));
  return blocks;
}

export function buildHostBlocks(
  profile: DebuggerProfile,
  values: HostFormState,
  resolvedAction: string,
  requestPayload: Record<string, unknown>,
  response: unknown,
  message?: string
): PluginPageBlock[] {
  const blocks: PluginPageBlock[] = [];
  if (message) {
    blocks.push(buildPluginAlertBlock({
      title: "Host Result",
      content: message,
      variant: response ? "success" : "warning"
    }));
  }
  blocks.push(buildPluginKeyValueBlock({
    title: "Host Request",
    items: [
      { key: "plugin_host_present", label: "Plugin.host Present", value: profile.runtime.hasPluginHost },
      { key: "plugin_host_enabled", label: "Plugin.host Enabled", value: profile.runtime.pluginHostEnabled },
      { key: "plugin_order_present", label: "Plugin.order Present", value: profile.runtime.hasPluginOrder },
      { key: "plugin_user_present", label: "Plugin.user Present", value: profile.runtime.hasPluginUser },
      { key: "plugin_product_present", label: "Plugin.product Present", value: profile.runtime.hasPluginProduct },
      { key: "plugin_inventory_present", label: "Plugin.inventory Present", value: profile.runtime.hasPluginInventory },
      { key: "plugin_inventory_binding_present", label: "Plugin.inventoryBinding Present", value: profile.runtime.hasPluginInventoryBinding },
      { key: "plugin_promo_present", label: "Plugin.promo Present", value: profile.runtime.hasPluginPromo },
      { key: "plugin_ticket_present", label: "Plugin.ticket Present", value: profile.runtime.hasPluginTicket },
      { key: "plugin_serial_present", label: "Plugin.serial Present", value: profile.runtime.hasPluginSerial },
      { key: "plugin_announcement_present", label: "Plugin.announcement Present", value: profile.runtime.hasPluginAnnouncement },
      { key: "plugin_knowledge_present", label: "Plugin.knowledge Present", value: profile.runtime.hasPluginKnowledge },
      { key: "plugin_payment_method_present", label: "Plugin.paymentMethod Present", value: profile.runtime.hasPluginPaymentMethod },
      { key: "plugin_virtual_inventory_present", label: "Plugin.virtualInventory Present", value: profile.runtime.hasPluginVirtualInventory },
      { key: "plugin_virtual_inventory_binding_present", label: "Plugin.virtualInventoryBinding Present", value: profile.runtime.hasPluginVirtualInventoryBinding },
      { key: "plugin_market_present", label: "Plugin.market Present", value: profile.runtime.hasPluginMarket },
      { key: "plugin_email_template_present", label: "Plugin.emailTemplate Present", value: profile.runtime.hasPluginEmailTemplate },
      { key: "plugin_landing_page_present", label: "Plugin.landingPage Present", value: profile.runtime.hasPluginLandingPage },
      { key: "plugin_invoice_template_present", label: "Plugin.invoiceTemplate Present", value: profile.runtime.hasPluginInvoiceTemplate },
      { key: "plugin_auth_branding_present", label: "Plugin.authBranding Present", value: profile.runtime.hasPluginAuthBranding },
      { key: "plugin_page_rule_pack_present", label: "Plugin.pageRulePack Present", value: profile.runtime.hasPluginPageRulePack },
      { key: "mode", label: "Mode", value: values.host_mode },
      { key: "resolved_action", label: "Resolved Action", value: resolvedAction || "-" },
      { key: "host_interpretation", label: "Host Interpretation", value: profile.runtime.hostInterpretation }
    ]
  }));
  blocks.push(buildCollapsedJSONBlock(
    "Host Request Payload",
    requestPayload,
    "Raw Plugin.host request payload dispatched by the debugger."
  ));
  blocks.push(buildCollapsedJSONBlock(
    "Host Response",
    response,
    "Raw Plugin.host response returned by the current bridge action."
  ));
  return blocks;
}

export function buildWorkerBlocks(
  profile: DebuggerProfile,
  values: WorkerFormState,
  roundtrip: Record<string, unknown> | null,
  message?: string
): PluginPageBlock[] {
  const blocks: PluginPageBlock[] = [];
  const requestResponses =
    roundtrip && typeof roundtrip.requests === "object" && roundtrip.requests && !Array.isArray(roundtrip.requests)
      ? (roundtrip.requests as Record<string, unknown>)
      : {};
  const messageEvent =
    roundtrip && typeof roundtrip.message_event === "object" && roundtrip.message_event && !Array.isArray(roundtrip.message_event)
      ? (roundtrip.message_event as Record<string, unknown>)
      : {};
  const support =
    roundtrip && typeof roundtrip.support === "object" && roundtrip.support && !Array.isArray(roundtrip.support)
      ? (roundtrip.support as Record<string, unknown>)
      : {};
  if (message) {
    blocks.push(buildPluginAlertBlock({
      title: "Worker Roundtrip",
      content: message,
      variant: roundtrip ? "success" : "warning"
    }));
  }
  blocks.push(buildPluginKeyValueBlock({
    title: "Worker Runtime",
    items: [
      { key: "worker_global_present", label: "Worker Global", value: profile.runtime.hasWorkerGlobal },
      { key: "worker_script", label: "Worker Script", value: values.worker_script || "-" },
      { key: "request_supported", label: "worker.request()", value: support.request ?? false },
      { key: "post_message_supported", label: "worker.postMessage()", value: support.postMessage ?? false },
      { key: "terminate_supported", label: "worker.terminate()", value: support.terminate ?? false },
      { key: "worker_id", label: "Worker ID", value: roundtrip?.worker_id || "-" },
      { key: "worker_script_path", label: "Resolved Script Path", value: roundtrip?.worker_script_path || "-" },
      { key: "first_doubled", label: "First Request Result", value: (requestResponses.first as Record<string, unknown> | undefined)?.doubled ?? "-" },
      { key: "second_doubled", label: "Second Request Result", value: (requestResponses.second as Record<string, unknown> | undefined)?.doubled ?? "-" },
      { key: "request_calls", label: "Child Call Count", value: (requestResponses.second as Record<string, unknown> | undefined)?.calls ?? "-" },
      { key: "message_value", label: "postMessage Event Value", value: (messageEvent.data as Record<string, unknown> | undefined)?.value ?? "-" },
      { key: "message_calls", label: "postMessage Event Calls", value: (messageEvent.data as Record<string, unknown> | undefined)?.calls ?? "-" },
      { key: "terminated_before", label: "Terminated Before", value: roundtrip?.terminated_before ?? "-" },
      { key: "terminated_after", label: "Terminated After", value: roundtrip?.terminated_after ?? "-" },
      { key: "runtime_interpretation", label: "Runtime Interpretation", value: profile.runtime.jsRuntimeInterpretation }
    ]
  }));
  blocks.push(buildCollapsedJSONBlock(
    "Worker Input",
    values,
    "Worker roundtrip input passed by the debugger action form."
  ));
  blocks.push(buildCollapsedJSONBlock(
    "Worker Request Responses",
    requestResponses,
    "Two sequential worker.request() calls to verify child runtime state retention."
  ));
  blocks.push(buildCollapsedJSONBlock(
    "Worker Message Event",
    messageEvent,
    "worker.postMessage() event delivered back to the parent runtime through onmessage."
  ));
  return blocks;
}

function buildSimulationCapabilityNotes(profile: DebuggerProfile, result: HookExecutionResponse): string[] {
  const notes: string[] = [];
  if (result.blocked && !profile.sandbox.allowHookBlock) {
    notes.push("Simulation produced blocked=true, but host would ignore it because hook.block is not granted.");
  }
  if (result.payload && !profile.sandbox.allowPayloadPatch) {
    notes.push("Simulation produced payload patch, but host would ignore it because hook.payload_patch is not granted.");
  }
  if (
    Array.isArray(result.frontend_extensions) &&
    result.frontend_extensions.length > 0 &&
    !profile.sandbox.allowFrontendExtensions
  ) {
    notes.push("Simulation produced frontend extensions, but host would ignore them because frontend.extensions is not granted.");
  }
  if (notes.length === 0) {
    notes.push("Simulation matches current granted capabilities.");
  }
  return notes;
}

export function buildSimulationBlocks(
  profile: DebuggerProfile,
  state: SimulatedHookState,
  result: HookExecutionResponse
): PluginPageBlock[] {
  return [
    buildPluginAlertBlock({
      title: "Simulation Result",
      content: result.blocked
        ? result.block_reason || `Hook ${state.simulate_hook} would be blocked.`
        : result.skipped
          ? result.reason || `Hook ${state.simulate_hook} is skipped.`
          : `Simulated ${state.simulate_hook} successfully.`,
      variant: result.blocked ? "warning" : result.skipped ? "info" : "success"
    }),
    buildPluginKeyValueBlock({
      title: "Capability Interpretation",
      items: buildSimulationCapabilityNotes(profile, result).map((item, index) => ({
        key: `note_${index + 1}`,
        label: `Note ${index + 1}`,
        value: item
      }))
    }),
    buildCollapsedJSONBlock(
      "Simulation Input Payload",
      parseJSONValueIfPossible(state.simulate_payload),
      "Raw simulated hook payload before sandbox execution."
    ),
    buildCollapsedJSONBlock(
      "Simulation Output",
      result,
      "Raw simulated hook execution result including payload patches and frontend extensions."
    )
  ];
}

function buildExecDemoInitial(area: "admin" | "user"): Record<string, unknown> {
  const path = area === "admin" ? ADMIN_PLUGIN_PAGE_PATH : USER_PLUGIN_PAGE_PATH;
  return {
    echo_message: `${PLUGIN_DISPLAY_NAME} self-exec from ${area} page`,
    echo_json: prettyJSON(
      {
        source: "visual-action-form",
        area,
        path
      }
    )
  };
}

function buildExecDemoActionForm(area: "admin" | "user"): PluginPageBlock {
  return buildPluginActionFormBlock({
    title: "Visual Exec Demo",
    initial: buildExecDemoInitial(area),
    autoload: false,
    save: "debugger.echo",
    saveLabel: "Run Exec",
    extra: [
      {
        key: "run-stream-exec",
        label: "Run Stream Exec",
        action: "debugger.echo.stream",
        variant: "secondary",
        include_fields: true,
        mode: "stream"
      }
    ],
    fields: [
      {
        key: "echo_message",
        type: "string",
        label: "Message",
        description: "This visual form calls the plugin page route's own execute API. Use the extra button to validate real progressive /execute/stream chunks."
      },
      {
        key: "echo_json",
        type: "json",
        label: "JSON Payload",
        rows: 8,
        description: "Optional JSON body passed through params for debugger.echo and debugger.echo.stream."
      }
    ]
  });
}

function buildExecHTMLDemoBlock(): PluginPageBlock {
  return buildPluginHTMLBlock(
    buildPluginExecuteHTMLBridge({
      action: "debugger.echo.stream",
      mode: "stream",
      target: "html-stream-demo",
      intro:
        "This HTML block validates the host /execute/stream bridge with real js_worker chunk emission. It exercises route declaration, API selection, progressive status updates, and the final structured result.",
      messageValue: `HTML bridge stream exec on ${PLUGIN_EXECUTE_TEMPLATE_PLACEHOLDERS["plugin.path"]}`,
      submitLabel: "Run HTML Stream",
      quickActionLabel: "Quick Stream Button",
      quickAction: "debugger.echo.stream",
      quickActionMode: "stream",
      quickActionParams: {
        echo_message: "Button-triggered stream exec",
        echo_json: prettyJSON({ source: "html-button" })
      },
      jsonValue: {
        source: "html-stream-bridge",
        plugin_id: PLUGIN_EXECUTE_TEMPLATE_PLACEHOLDERS["plugin.id"],
        area: PLUGIN_EXECUTE_TEMPLATE_PLACEHOLDERS["plugin.area"],
        path: PLUGIN_EXECUTE_TEMPLATE_PLACEHOLDERS["plugin.path"],
        stream_url: PLUGIN_EXECUTE_TEMPLATE_PLACEHOLDERS["plugin.execute_stream_url"],
        stream_actions: PLUGIN_EXECUTE_TEMPLATE_PLACEHOLDERS["plugin.execute_stream_actions"]
      }
    }),
    "HTML Stream Bridge",
    {
      theme: "host",
      stream_actions: ["debugger.echo.stream"]
    }
  );
}

function buildSelfTestActionForm(): PluginPageBlock {
  return buildPluginActionFormBlock({
    title: "Debugger Self-Test",
    initial: {},
    autoload: false,
    save: "debugger.selftest",
    saveLabel: "Run Self-Test",
    extra: [
      {
        key: "clear-action-traces",
        label: "Clear Action Traces",
        action: "debugger.action_traces.clear",
        variant: "secondary",
        include_fields: false
      },
      {
        key: "clear-hook-events",
        label: "Clear Hook Events",
        action: "debugger.events.clear",
        variant: "secondary",
        include_fields: false
      }
    ],
    fields: []
  });
}

function buildConfigActionForm(profile: DebuggerProfile): PluginPageBlock {
  return buildPluginActionFormBlock({
    title: "Debugger Controls",
    initial: { ...profile.config },
    load: "debugger.config.get",
    loadLabel: "Load Config",
    save: "debugger.config.set",
    saveLabel: "Save Config",
    reset: "debugger.config.reset",
    resetLabel: "Restore Defaults",
    extra: [
      {
        key: "clear-events",
        label: "Clear Events",
        action: "debugger.events.clear",
        variant: "secondary",
        include_fields: false
      }
    ],
    fields: [
      { key: "enable_frontend", type: "boolean", label: "Frontend Hooks", description: "Keep frontend.slot.render active. frontend.bootstrap stays available so the debugger page never disappears." },
      { key: "enable_auth", type: "boolean", label: "Auth & User Hooks", description: "Capture auth, account binding, password, preferences, and admin user management hooks." },
      { key: "enable_platform", type: "boolean", label: "Platform Hooks", description: "Capture plugin lifecycle, package upload, secret, API key, upload, and log hooks." },
      { key: "enable_commerce", type: "boolean", label: "Commerce Hooks", description: "Capture cart, order, payment, promo, and serial hooks." },
      { key: "enable_catalog", type: "boolean", label: "Catalog Hooks", description: "Capture product, inventory, and virtual inventory hooks." },
      { key: "enable_support", type: "boolean", label: "Support Hooks", description: "Capture ticket lifecycle hooks." },
      { key: "enable_content", type: "boolean", label: "Content Hooks", description: "Capture announcement, knowledge, marketing, email, and SMS hooks." },
      { key: "enable_settings", type: "boolean", label: "Settings Hooks", description: "Capture settings, landing page, email template, and template package hooks." },
      { key: "emit_frontend_extensions", type: "boolean", label: "Emit Slot Extensions", description: "Inject cards into allowed frontend slots." },
      { key: "emit_payload_marker", type: "boolean", label: "Emit Payload Marker", description: "Attach debugger metadata to non-before hook payloads." },
      { key: "persist_events", type: "boolean", label: "Persist Events", description: "Store recent hook events in Plugin.storage." },
      { key: "max_events", type: "number", label: "Max Events", description: "Retention for recent hook events in storage." },
      { key: "demo_block_before_hooks", type: "boolean", label: "Demo Blocking", description: "Block .before hooks when payload contains the keyword below." },
      { key: "block_keyword", type: "string", label: "Block Keyword", description: "Keyword used by the blocking demo.", placeholder: "debug-block" },
      {
        key: "slot_notice_mode",
        type: "select",
        label: "Slot Notice Mode",
        description: "Choose how much text the slot injection renders.",
        options: [
          { label: "Compact", value: "compact" },
          { label: "Verbose", value: "verbose" }
        ]
      }
    ]
  });
}

function buildSimulatorActionForm(): PluginPageBlock {
  return buildPluginActionFormBlock({
    title: "Hook Simulator",
    initial: { ...DEFAULT_SIMULATION_STATE },
    autoload: false,
    save: "debugger.simulate.hook",
    saveLabel: "Run Simulation",
    fields: [
      {
        key: "simulate_hook",
        type: "select",
        label: "Hook",
        description: "Plugin-local simulation. Real hook pipeline still depends on host capability gates.",
        options: ALL_HOOK_OPTIONS.map((hook) => ({ label: hook, value: hook }))
      },
      {
        key: "simulate_area",
        type: "select",
        label: "Area",
        options: [
          { label: "Admin", value: "admin" },
          { label: "User", value: "user" }
        ]
      },
      {
        key: "simulate_slot",
        type: "select",
        label: "Frontend Slot",
        options: SLOT_OPTIONS.map((slot) => ({ label: slot, value: slot }))
      },
      {
        key: "simulate_path",
        type: "string",
        label: "Path",
        placeholder: "/admin/dashboard"
      },
      {
        key: "simulate_payload",
        type: "json",
        label: "Payload JSON",
        rows: 10,
        description: "Used as hook payload. For frontend hooks, area/path/slot are also merged in."
      }
    ]
  });
}

function buildStorageActionForm(): PluginPageBlock {
  return buildPluginActionFormBlock({
    title: "Storage Lab",
    initial: { ...DEFAULT_STORAGE_FORM_STATE },
    autoload: false,
    load: "debugger.storage.inspect",
    loadLabel: "Load Key",
    save: "debugger.storage.upsert",
    saveLabel: "Write Storage",
    reset: "debugger.storage.delete",
    resetLabel: "Delete Key",
    extra: [
      {
        key: "storage-seed",
        label: "Seed Sample",
        action: "debugger.storage.seed",
        variant: "secondary",
        include_fields: false
      },
      {
        key: "storage-clear",
        label: "Clear Lab Keys",
        action: "debugger.storage.clear_lab",
        variant: "destructive",
        include_fields: false
      }
    ],
    fields: [
      {
        key: "storage_key",
        type: "string",
        label: "Storage Key",
        description: "Anything except reserved keys used by the debugger itself."
      },
      {
        key: "storage_value",
        type: "textarea",
        label: "Storage Value",
        rows: 6,
        description: "Raw string value. JSON is allowed but not required."
      }
    ]
  });
}

function buildFSActionForm(): PluginPageBlock {
  return buildPluginActionFormBlock({
    title: "Filesystem Lab",
    initial: { ...DEFAULT_FS_FORM_STATE },
    autoload: false,
    load: "debugger.fs.read",
    loadLabel: "Load File",
    save: "debugger.fs.write",
    saveLabel: "Write File",
    reset: "debugger.fs.delete",
    resetLabel: "Delete File",
    extra: [
      {
        key: "fs-inspect",
        label: "Inspect FS",
        action: "debugger.fs.inspect",
        variant: "secondary",
        include_fields: false
      },
      {
        key: "fs-seed",
        label: "Seed Sample",
        action: "debugger.fs.seed",
        variant: "secondary",
        include_fields: false
      }
    ],
    fields: [
      {
        key: "fs_path",
        type: "string",
        label: "Path",
        description: "Path is relative to the plugin virtual filesystem root."
      },
      {
        key: "fs_format",
        type: "select",
        label: "Write Format",
        options: [
          { label: "Text", value: "text" },
          { label: "JSON", value: "json" }
        ]
      },
      {
        key: "fs_content",
        type: "textarea",
        label: "Content",
        rows: 8,
        description: "Write/read content inside the plugin data root."
      }
    ]
  });
}

function buildNetworkActionForm(): PluginPageBlock {
  return buildPluginActionFormBlock({
    title: "Network Lab",
    initial: { ...DEFAULT_NETWORK_FORM_STATE },
    autoload: false,
    save: "debugger.network.request",
    saveLabel: "Send Request",
    fields: [
      {
        key: "network_method",
        type: "select",
        label: "Method",
        options: [
          { label: "GET", value: "GET" },
          { label: "POST", value: "POST" },
          { label: "PUT", value: "PUT" },
          { label: "PATCH", value: "PATCH" },
          { label: "DELETE", value: "DELETE" },
          { label: "HEAD", value: "HEAD" }
        ]
      },
      {
        key: "network_url",
        type: "string",
        label: "URL",
        description: "Only public http/https targets are allowed. localhost, private IPs, and internal suffixes are blocked."
      },
      {
        key: "network_timeout_ms",
        type: "number",
        label: "Timeout (ms)",
        description: "Clamped by the JS worker runtime timeout."
      },
      {
        key: "network_headers",
        type: "json",
        label: "Headers JSON",
        rows: 8,
        description: "JSON object of request headers."
      },
      {
        key: "network_body_format",
        type: "select",
        label: "Body Format",
        options: [
          { label: "JSON", value: "json" },
          { label: "Text", value: "text" }
        ]
      },
      {
        key: "network_body",
        type: "textarea",
        label: "Body",
        rows: 8,
        description: "Ignored for GET/HEAD requests."
      }
    ]
  });
}

function buildWorkerActionForm(): PluginPageBlock {
  return buildPluginActionFormBlock({
    title: "Worker Lab",
    initial: { ...DEFAULT_WORKER_FORM_STATE },
    autoload: false,
    save: "debugger.worker.roundtrip",
    saveLabel: "Run Worker Roundtrip",
    fields: [
      {
        key: "worker_script",
        type: "string",
        label: "Worker Script",
        description: "Relative path from the plugin package root. The bundled sample worker lives under ./assets."
      },
      {
        key: "worker_request_value",
        type: "number",
        label: "First Request Value",
        description: "Sent through worker.request() to verify child runtime request/response."
      },
      {
        key: "worker_second_value",
        type: "number",
        label: "Second Request Value",
        description: "Sent through a second worker.request() call to verify state retention inside the child runtime."
      },
      {
        key: "worker_message_value",
        type: "number",
        label: "postMessage Value",
        description: "Sent through worker.postMessage() to verify parent onmessage delivery."
      }
    ]
  });
}

function buildHostActionForm(): PluginPageBlock {
  return buildPluginActionFormBlock({
    title: "Host Data Lab",
    initial: { ...DEFAULT_HOST_FORM_STATE },
    autoload: false,
    save: "debugger.host.request",
    saveLabel: "Run Host Request",
    fields: [
      {
        key: "host_mode",
        type: "select",
        label: "Mode",
        description: "Prefer typed helpers first. The debugger now covers native data, market install, and template management helpers. Use host.invoke only for raw action dispatch checks.",
        options: [
          { label: "Order Get", value: "order.get" },
          { label: "Order List", value: "order.list" },
          { label: "Order Assign Tracking", value: "order.assign_tracking" },
          { label: "Order Request Resubmit", value: "order.request_resubmit" },
          { label: "Order Mark Paid", value: "order.mark_paid" },
          { label: "Order Update Price", value: "order.update_price" },
          { label: "User Get", value: "user.get" },
          { label: "User List", value: "user.list" },
          { label: "Product Get", value: "product.get" },
          { label: "Product List", value: "product.list" },
          { label: "Inventory Get", value: "inventory.get" },
          { label: "Inventory List", value: "inventory.list" },
          { label: "Inventory Binding Get", value: "inventory_binding.get" },
          { label: "Inventory Binding List", value: "inventory_binding.list" },
          { label: "Promo Get", value: "promo.get" },
          { label: "Promo List", value: "promo.list" },
          { label: "Ticket Get", value: "ticket.get" },
          { label: "Ticket List", value: "ticket.list" },
          { label: "Ticket Reply", value: "ticket.reply" },
          { label: "Ticket Update", value: "ticket.update" },
          { label: "Serial Get", value: "serial.get" },
          { label: "Serial List", value: "serial.list" },
          { label: "Announcement Get", value: "announcement.get" },
          { label: "Announcement List", value: "announcement.list" },
          { label: "Knowledge Get", value: "knowledge.get" },
          { label: "Knowledge List", value: "knowledge.list" },
          { label: "Knowledge Categories", value: "knowledge.categories" },
          { label: "Payment Method Get", value: "payment_method.get" },
          { label: "Payment Method List", value: "payment_method.list" },
          { label: "Virtual Inventory Get", value: "virtual_inventory.get" },
          { label: "Virtual Inventory List", value: "virtual_inventory.list" },
          { label: "Virtual Inventory Binding Get", value: "virtual_inventory_binding.get" },
          { label: "Virtual Inventory Binding List", value: "virtual_inventory_binding.list" },
          { label: "Market Source List", value: "market.source.list" },
          { label: "Market Source Get", value: "market.source.get" },
          { label: "Market Catalog List", value: "market.catalog.list" },
          { label: "Market Artifact Get", value: "market.artifact.get" },
          { label: "Market Release Get", value: "market.release.get" },
          { label: "Market Install Preview", value: "market.install.preview" },
          { label: "Market Install Execute", value: "market.install.execute" },
          { label: "Market Install Task Get", value: "market.install.task.get" },
          { label: "Market Install Task List", value: "market.install.task.list" },
          { label: "Market Install History List", value: "market.install.history.list" },
          { label: "Market Install Rollback", value: "market.install.rollback" },
          { label: "Email Template List", value: "email_template.list" },
          { label: "Email Template Get", value: "email_template.get" },
          { label: "Email Template Save", value: "email_template.save" },
          { label: "Landing Page Get", value: "landing_page.get" },
          { label: "Landing Page Save", value: "landing_page.save" },
          { label: "Landing Page Reset", value: "landing_page.reset" },
          { label: "Invoice Template Get", value: "invoice_template.get" },
          { label: "Invoice Template Save", value: "invoice_template.save" },
          { label: "Invoice Template Reset", value: "invoice_template.reset" },
          { label: "Auth Branding Get", value: "auth_branding.get" },
          { label: "Auth Branding Save", value: "auth_branding.save" },
          { label: "Auth Branding Reset", value: "auth_branding.reset" },
          { label: "Page Rule Pack Get", value: "page_rule_pack.get" },
          { label: "Page Rule Pack Save", value: "page_rule_pack.save" },
          { label: "Page Rule Pack Reset", value: "page_rule_pack.reset" },
          { label: "Raw Host Invoke", value: "host.invoke" }
        ]
      },
      {
        key: "host_action",
        type: "string",
        label: "Raw Host Action",
        description: "Only used when Mode is Raw Host Invoke.",
        placeholder: "host.order.get"
      },
      {
        key: "host_payload",
        type: "json",
        label: "Payload JSON",
        rows: 10,
        description: "JSON object passed to typed Plugin.* host helpers or Plugin.host.invoke."
      }
    ]
  });
}

export function buildDebuggerDashboardBlocks(
  area: "admin" | "user",
  profile: DebuggerProfile
): PluginPageBlock[] {
  const intro = profile.capability_gaps.length === 0
    ? `${PLUGIN_DISPLAY_NAME} is active. This page is injected by frontend.bootstrap and reflects live JS worker runtime state.`
    : `${PLUGIN_DISPLAY_NAME} loaded with restricted capabilities. Review the warnings below before using interactive labs.`;

  const blocks: PluginPageBlock[] = [
    buildPluginAlertBlock({
      title: PLUGIN_DISPLAY_NAME,
      content: intro,
      variant: profile.capability_gaps.length === 0 ? "success" : "warning"
    }),
    buildSelfTestActionForm(),
    ...buildProfileSummaryBlocks(profile),
    buildPluginLinkListBlock({
      title: "Debugger Routes",
      links: [
        {
          label: "Admin Debugger Page",
          url: ADMIN_PLUGIN_PAGE_PATH
        },
        {
          label: "User Debugger Page",
          url: USER_PLUGIN_PAGE_PATH
        }
      ]
    }),
    buildPluginHTMLBlock(renderDebuggerNpmComponentCard(area, profile), undefined, {
      chrome: "bare",
      theme: "host"
    }),
    buildExecDemoActionForm(area),
    buildExecHTMLDemoBlock()
  ];

  if (area === "admin") {
    blocks.push(buildConfigActionForm(profile));
    blocks.push(buildSimulatorActionForm());
    blocks.push(buildStorageActionForm());
    blocks.push(buildFSActionForm());
    blocks.push(buildNetworkActionForm());
    blocks.push(buildWorkerActionForm());
    blocks.push(buildHostActionForm());
  } else {
    blocks.push(buildPluginAlertBlock({
      title: "Admin Labs",
      content: "Interactive config, storage, filesystem, network, host data, and simulation labs are available on the admin debugger page.",
      variant: "info"
    }));
  }

  blocks.push(buildPluginKeyValueBlock({
    title: "What To Test Next",
    items: [
      {
        label: "Frontend",
        value: "Open dashboard/orders/plugin pages and watch slot injections update."
      },
      {
        label: "Business Hooks",
        value: "Trigger login/order/payment/ticket/product flows and refresh the recent event table."
      },
      {
        label: "Storage & FS",
        value: "Use admin labs to verify Plugin.storage and Plugin.fs persistence."
      },
      {
        label: "Network",
        value: "Use Network Lab to verify Plugin.http permissions, SSRF guards, and live outbound responses."
      },
      {
        label: "Workers",
        value: "Use Worker Lab to verify new Worker(), worker.request(), worker.postMessage(), parent onmessage delivery, and terminate behavior."
      },
      {
        label: "Host Data",
        value: "Use Host Data Lab to verify Plugin.host and typed helpers such as Plugin.order / Plugin.user / Plugin.market / Plugin.emailTemplate / Plugin.landingPage / Plugin.invoiceTemplate / Plugin.authBranding / Plugin.pageRulePack."
      },
      {
        label: "Execute Stream",
        value: "Use Run Stream Exec and HTML Stream Bridge to verify declared stream actions, host task IDs, and real progressive js_worker chunk delivery."
      },
      {
        label: "Secrets & Webhooks",
        value: "Configure debugger secrets in plugin settings and invoke the sample webhook to verify Plugin.secret / Plugin.webhook diagnostics."
      },
      {
        label: "Workspace Reports",
        value: "Run debugger/report, debugger/selftest, and debugger/traces in the workspace to emit concise text diagnostics into the terminal buffer."
      }
    ]
  }));

  return blocks;
}

export function buildSlotExtensionContent(
  profile: DebuggerProfile,
  hook: string,
  slot: string,
  path: string
): string {
  const latestHook = profile.recent_events[0]?.hook || "no recent hook";
  if (profile.config.slot_notice_mode === "verbose") {
    return [
      `${PLUGIN_DISPLAY_NAME} observed ${hook}.`,
      `slot=${slot}`,
      `path=${path}`,
      `recent=${profile.recent_events.length}`,
      `latest=${latestHook}`
    ].join(" ");
  }
  return `${PLUGIN_DISPLAY_NAME}: ${hook} @ ${slot} (${truncateText(path, 48)})`;
}
