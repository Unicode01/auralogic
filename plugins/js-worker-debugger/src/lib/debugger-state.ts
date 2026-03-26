import {
  getPluginSecret,
  getPluginAuthBranding,
  getPluginAnnouncement,
  getPluginEmailTemplate,
  getPluginFS,
  getPluginHost,
  getPluginHTTP,
  getPluginInventory,
  getPluginInventoryBinding,
  getPluginInvoiceTemplate,
  getPluginKnowledge,
  getPluginLandingPage,
  getPluginMarket,
  getPluginOrder,
  getPluginPageRulePack,
  getPluginPaymentMethod,
  getPluginProduct,
  getPluginPromo,
  getPluginSerial,
  getPluginTicket,
  getPluginUser,
  getPluginVirtualInventory,
  getPluginVirtualInventoryBinding,
  getPluginWebhook,
  getPluginWorkspace,
  normalizePluginFSUsage,
  type PluginFSAPI,
  type PluginHTTPAPI
} from "@auralogic/plugin-sdk";
import {
  ACTION_TRACE_STORAGE_KEY,
  DEFAULT_HOST_FORM_STATE,
  CONFIG_STORAGE_KEY,
  DEBUGGER_SECRET_SAMPLE_KEY,
  DEFAULT_FS_PATH,
  DEFAULT_NETWORK_FORM_STATE,
  DEFAULT_STORAGE_KEY,
  DEFAULT_WORKER_FORM_STATE,
  EVENT_STORAGE_KEY,
  RESERVED_STORAGE_KEYS
} from "./constants";
import { getStorageAPI, isReservedStorageKey } from "./debugger-config";
import type {
  DebuggerActionTrace,
  DebuggerEvent,
  DebuggerFileSummary,
  HostFormState,
  DebuggerNetworkResponse,
  DebuggerSecretSummary,
  DebuggerProfile,
  DebuggerRuntimeProbe,
  DebuggerStorageSummary,
  DebuggerWebhookSummary,
  FileSystemFormState,
  GenericRecord,
  NetworkFormState,
  StorageFormState,
  WorkerFormState
} from "./types";
import {
  asBool,
  asInteger,
  asRecord,
  asString,
  getRuntimePluginGlobal,
  normalizeHookName,
  nowISO,
  parseJSONValueIfPossible,
  prettyJSON,
  truncateText
} from "./utils";

function getFSAPI(): PluginFSAPI | null {
  return getPluginFS() || null;
}

function getHTTPAPI(): PluginHTTPAPI | null {
  const http = getPluginHTTP();
  if (!http) {
    return null;
  }
  if (
    typeof http.get !== "function" ||
    typeof http.post !== "function" ||
    typeof http.request !== "function"
  ) {
    return null;
  }
  return http;
}

function hasRuntimeFunction(name: string): boolean {
  const candidate = (globalThis as Record<string, unknown>)[name];
  return typeof candidate === "function";
}

function buildActionTraceID(action: string): string {
  return `${action}:${nowISO()}:${Math.random().toString(36).slice(2, 8)}`;
}

function normalizeEvent(value: unknown): DebuggerEvent | null {
  const source = asRecord(value);
  const id = asString(source.id);
  const hook = normalizeHookName(source.hook);
  const ts = asString(source.ts) || nowISO();
  if (!id || !hook) {
    return null;
  }
  return {
    id,
    ts,
    hook,
    group: asString(source.group) as DebuggerEvent["group"],
    area: asString(source.area) || undefined,
    slot: asString(source.slot) || undefined,
    path: asString(source.path) || undefined,
    user_id: typeof source.user_id === "number" ? source.user_id : undefined,
    order_id: typeof source.order_id === "number" ? source.order_id : undefined,
    session_id: asString(source.session_id) || undefined,
    blocked: typeof source.blocked === "boolean" ? source.blocked : undefined,
    note: asString(source.note) || undefined,
    payload_json: typeof source.payload_json === "string" ? source.payload_json : undefined,
    context_json: typeof source.context_json === "string" ? source.context_json : undefined
  };
}

export function readDebuggerEvents(): DebuggerEvent[] {
  const storage = getStorageAPI();
  if (!storage) {
    return [];
  }
  const raw = storage.get(EVENT_STORAGE_KEY);
  if (!raw || typeof raw !== "string") {
    return [];
  }
  try {
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) {
      return [];
    }
    return parsed
      .map((item) => normalizeEvent(item))
      .filter((item): item is DebuggerEvent => !!item)
      .sort((a, b) => b.ts.localeCompare(a.ts));
  } catch {
    return [];
  }
}

function writeDebuggerEvents(events: DebuggerEvent[]): boolean {
  const storage = getStorageAPI();
  if (!storage) {
    return false;
  }
  return Boolean(storage.set(EVENT_STORAGE_KEY, JSON.stringify(events)));
}

export function appendDebuggerEvent(event: DebuggerEvent, maxEvents: number): boolean {
  const storage = getStorageAPI();
  if (!storage) {
    return false;
  }
  const current = readDebuggerEvents();
  const next = [event, ...current].slice(0, maxEvents);
  return writeDebuggerEvents(next);
}

export function clearDebuggerEvents(): boolean {
  const storage = getStorageAPI();
  if (!storage) {
    return false;
  }
  return Boolean(storage.delete(EVENT_STORAGE_KEY));
}

function normalizeActionTrace(value: unknown): DebuggerActionTrace | null {
  const source = asRecord(value);
  const action = asString(source.action).toLowerCase();
  const ts = asString(source.ts) || nowISO();
  const id = asString(source.id) || buildActionTraceID(action || "debugger.unknown");
  if (!action) {
    return null;
  }
  return {
    id,
    ts,
    action,
    category: asString(source.category) || "other",
    ok: source.ok !== false,
    message: asString(source.message) || (source.ok === false ? "Action failed." : "Action completed."),
    error: asString(source.error) || undefined,
    current_action: asString(source.current_action) || undefined,
    declared_storage_access_mode: asString(source.declared_storage_access_mode) as DebuggerActionTrace["declared_storage_access_mode"],
    observed_storage_access_mode: asString(source.observed_storage_access_mode) as DebuggerActionTrace["observed_storage_access_mode"],
    user_id: typeof source.user_id === "number" ? source.user_id : undefined,
    order_id: typeof source.order_id === "number" ? source.order_id : undefined,
    session_id: asString(source.session_id) || undefined,
    request_summary: asString(source.request_summary) || undefined,
    response_summary: asString(source.response_summary) || undefined,
    request_json: typeof source.request_json === "string" ? source.request_json : undefined,
    response_json: typeof source.response_json === "string" ? source.response_json : undefined
  };
}

export function readActionTraces(): DebuggerActionTrace[] {
  const storage = getStorageAPI();
  if (!storage) {
    return [];
  }
  const raw = storage.get(ACTION_TRACE_STORAGE_KEY);
  if (!raw || typeof raw !== "string") {
    return [];
  }
  try {
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) {
      return [];
    }
    return parsed
      .map((item) => normalizeActionTrace(item))
      .filter((item): item is DebuggerActionTrace => Boolean(item))
      .sort((a, b) => b.ts.localeCompare(a.ts));
  } catch {
    return [];
  }
}

function writeActionTraces(traces: DebuggerActionTrace[]): boolean {
  const storage = getStorageAPI();
  if (!storage) {
    return false;
  }
  return Boolean(storage.set(ACTION_TRACE_STORAGE_KEY, JSON.stringify(traces)));
}

export function appendActionTrace(trace: DebuggerActionTrace, maxEntries = 60): boolean {
  const storage = getStorageAPI();
  if (!storage) {
    return false;
  }
  const current = readActionTraces();
  const next = [trace, ...current].slice(0, maxEntries);
  return writeActionTraces(next);
}

export function clearActionTraces(): boolean {
  const storage = getStorageAPI();
  if (!storage) {
    return false;
  }
  return Boolean(storage.delete(ACTION_TRACE_STORAGE_KEY));
}

export function buildStorageSummary(): DebuggerStorageSummary {
  const storage = getStorageAPI();
  if (!storage) {
    return {
      key_count: 0,
      reserved_keys: [...RESERVED_STORAGE_KEYS],
      lab_keys: [],
      keys: []
    };
  }
  const keys = storage.list().map((item) => asString(item)).filter(Boolean).sort((a, b) => a.localeCompare(b));
  return {
    key_count: keys.length,
    reserved_keys: [...RESERVED_STORAGE_KEYS],
    lab_keys: keys.filter((item) => !RESERVED_STORAGE_KEYS.includes(item)),
    keys
  };
}

export function readStorageFormState(params: unknown): StorageFormState {
  const source = asRecord(params);
  return {
    storage_key: asString(source.storage_key) || DEFAULT_STORAGE_KEY,
    storage_value: typeof source.storage_value === "string" ? source.storage_value : ""
  };
}

export function inspectStorageValue(input: StorageFormState): {
  values: StorageFormState;
  summary: DebuggerStorageSummary;
} {
  const storage = getStorageAPI();
  const nextKey = input.storage_key || DEFAULT_STORAGE_KEY;
  const nextValue = storage?.get(nextKey) ?? input.storage_value ?? "";
  return {
    values: {
      storage_key: nextKey,
      storage_value: nextValue
    },
    summary: buildStorageSummary()
  };
}

export function upsertStorageValue(input: StorageFormState): {
  ok: boolean;
  message: string;
  values: StorageFormState;
  summary: DebuggerStorageSummary;
} {
  const storage = getStorageAPI();
  const key = input.storage_key || DEFAULT_STORAGE_KEY;
  if (!storage) {
    return {
      ok: false,
      message: "Plugin.storage is unavailable in current runtime.",
      values: { ...input, storage_key: key },
      summary: buildStorageSummary()
    };
  }
  if (isReservedStorageKey(key)) {
    return {
      ok: false,
      message: `Storage key ${key} is reserved by Plugin Debugger.`,
      values: { ...input, storage_key: key },
      summary: buildStorageSummary()
    };
  }
  const ok = Boolean(storage.set(key, input.storage_value));
  return {
    ok,
    message: ok ? `Stored ${key}.` : `Failed to store ${key}.`,
    values: { ...input, storage_key: key },
    summary: buildStorageSummary()
  };
}

export function deleteStorageValue(input: StorageFormState): {
  ok: boolean;
  message: string;
  values: StorageFormState;
  summary: DebuggerStorageSummary;
} {
  const storage = getStorageAPI();
  const key = input.storage_key || DEFAULT_STORAGE_KEY;
  if (!storage) {
    return {
      ok: false,
      message: "Plugin.storage is unavailable in current runtime.",
      values: { storage_key: key, storage_value: input.storage_value },
      summary: buildStorageSummary()
    };
  }
  if (isReservedStorageKey(key)) {
    return {
      ok: false,
      message: `Storage key ${key} is reserved by Plugin Debugger.`,
      values: { storage_key: key, storage_value: input.storage_value },
      summary: buildStorageSummary()
    };
  }
  const ok = Boolean(storage.delete(key));
  return {
    ok,
    message: ok ? `Deleted ${key}.` : `Failed to delete ${key}.`,
    values: {
      storage_key: key,
      storage_value: ""
    },
    summary: buildStorageSummary()
  };
}

export function clearLabStorageValues(): {
  ok: boolean;
  message: string;
  summary: DebuggerStorageSummary;
} {
  const storage = getStorageAPI();
  if (!storage) {
    return {
      ok: false,
      message: "Plugin.storage is unavailable in current runtime.",
      summary: buildStorageSummary()
    };
  }
  const keys = storage.list().map((item) => asString(item)).filter(Boolean);
  keys.forEach((key) => {
    if (!RESERVED_STORAGE_KEYS.includes(key)) {
      storage.delete(key);
    }
  });
  return {
    ok: true,
    message: "Cleared non-reserved storage keys.",
    summary: buildStorageSummary()
  };
}

export function seedStorageValue(): {
  ok: boolean;
  values: StorageFormState;
  message: string;
  summary: DebuggerStorageSummary;
} {
  const values: StorageFormState = {
    storage_key: DEFAULT_STORAGE_KEY,
    storage_value: `Plugin Debugger sample note @ ${nowISO()}`
  };
  const result = upsertStorageValue(values);
  return {
    ok: result.ok,
    values,
    message: result.message,
    summary: result.summary
  };
}

export function buildFileSummary(): DebuggerFileSummary {
  const fsAPI = getFSAPI();
  if (!fsAPI || !fsAPI.enabled) {
    return {
      enabled: false,
      entries: [],
      data_root: fsAPI?.dataRoot || "",
      max_read_bytes: fsAPI?.maxReadBytes || 0
    };
  }
  const errors: string[] = [];
  let usage = undefined;
  let entries: DebuggerFileSummary["entries"] = [];
  try {
    usage = normalizePluginFSUsage(fsAPI.usage(), fsAPI);
  } catch (error) {
    errors.push(`usage(): ${String((error as Error).message || error)}`);
  }
  try {
    entries = fsAPI.list();
  } catch (error) {
    errors.push(`list('.'): ${String((error as Error).message || error)}`);
  }
  return {
    enabled: true,
    usage,
    entries,
    probe_error: errors.join(" | ") || undefined,
    data_root: fsAPI.dataRoot || "",
    max_read_bytes: fsAPI.maxReadBytes || 0
  };
}

export function buildRuntimeProbe(summary?: DebuggerFileSummary): DebuggerRuntimeProbe {
  const sandbox = asRecord(globalThis && globalThis.sandbox);
  const runtimeGlobal = getRuntimePluginGlobal();
  const workspaceAPI = getPluginWorkspace();
  const secretAPI = getPluginSecret();
  const webhookAPI = getPluginWebhook();
  const fsAPI = getFSAPI();
  const httpAPI = getHTTPAPI();
  const hostAPI = getPluginHost();
  const orderAPI = getPluginOrder();
  const userAPI = getPluginUser();
  const productAPI = getPluginProduct();
  const inventoryAPI = getPluginInventory();
  const inventoryBindingAPI = getPluginInventoryBinding();
  const promoAPI = getPluginPromo();
  const ticketAPI = getPluginTicket();
  const serialAPI = getPluginSerial();
  const announcementAPI = getPluginAnnouncement();
  const knowledgeAPI = getPluginKnowledge();
  const paymentMethodAPI = getPluginPaymentMethod();
  const virtualInventoryAPI = getPluginVirtualInventory();
  const virtualInventoryBindingAPI = getPluginVirtualInventoryBinding();
  const marketAPI = getPluginMarket();
  const emailTemplateAPI = getPluginEmailTemplate();
  const landingPageAPI = getPluginLandingPage();
  const invoiceTemplateAPI = getPluginInvoiceTemplate();
  const authBrandingAPI = getPluginAuthBranding();
  const pageRulePackAPI = getPluginPageRulePack();
  const sandboxAllowFileSystem = asBool(
    sandbox.allowFileSystem ?? sandbox.AllowFileSystem ?? sandbox.allow_file_system,
    false
  );
  const sandboxAllowNetwork = asBool(
    sandbox.allowNetwork ?? sandbox.AllowNetwork ?? sandbox.allow_network,
    false
  );
  const hasWorkerGlobal = hasRuntimeFunction("Worker");
  const hasStructuredCloneGlobal = hasRuntimeFunction("structuredClone");
  const hasQueueMicrotaskGlobal = hasRuntimeFunction("queueMicrotask");
  const hasSetTimeoutGlobal = hasRuntimeFunction("setTimeout");
  const hasClearTimeoutGlobal = hasRuntimeFunction("clearTimeout");
  const hasTextEncoderGlobal = hasRuntimeFunction("TextEncoder");
  const hasTextDecoderGlobal = hasRuntimeFunction("TextDecoder");
  const hasAtobGlobal = hasRuntimeFunction("atob");
  const hasBtoaGlobal = hasRuntimeFunction("btoa");
  const pluginGlobalKeys = Object.keys(runtimeGlobal || {}).sort((left, right) => left.localeCompare(right));
  const runtimeGlobalKeys = [
    hasWorkerGlobal ? "Worker" : "",
    hasStructuredCloneGlobal ? "structuredClone" : "",
    hasQueueMicrotaskGlobal ? "queueMicrotask" : "",
    hasSetTimeoutGlobal ? "setTimeout" : "",
    hasClearTimeoutGlobal ? "clearTimeout" : "",
    hasTextEncoderGlobal ? "TextEncoder" : "",
    hasTextDecoderGlobal ? "TextDecoder" : "",
    hasAtobGlobal ? "atob" : "",
    hasBtoaGlobal ? "btoa" : ""
  ]
    .filter(Boolean)
    .sort((left, right) => left.localeCompare(right));
  let workspaceEntryCount = 0;
  let workspaceMaxEntries = 0;
  let workspaceProbeError = "";
  if (workspaceAPI && typeof workspaceAPI.snapshot === "function") {
    try {
      const snapshot = workspaceAPI.snapshot(16);
      workspaceEntryCount = asInteger(snapshot?.entry_count, 0);
      workspaceMaxEntries = asInteger(snapshot?.max_entries, 0);
    } catch (error) {
      workspaceProbeError = String((error as Error).message || error);
    }
  }
  const fsProbeError = summary?.probe_error || "";

  let workspaceInterpretation = "Plugin.workspace object is missing in runtime.";
  if (workspaceAPI && !workspaceAPI.enabled) {
    workspaceInterpretation = "Plugin.workspace exists but enabled=false. Host workspace support is disabled for current runtime.";
  } else if (workspaceAPI && workspaceProbeError) {
    workspaceInterpretation = "Plugin.workspace is enabled, but snapshot probe failed. Check runtime bridge compatibility.";
  } else if (workspaceAPI && workspaceAPI.enabled) {
    workspaceInterpretation = `Plugin.workspace is enabled with ${workspaceEntryCount} retained entries.`;
  }

  let interpretation = "Plugin.fs object is missing in runtime.";
  if (fsAPI && !sandboxAllowFileSystem) {
    interpretation = "Sandbox allowFileSystem=false. FS was gated before file operations.";
  } else if (fsAPI && !fsAPI.enabled) {
    interpretation = "Plugin.fs exists but enabled=false. Host disabled effective FS access.";
  } else if (fsAPI && fsProbeError) {
    interpretation = "Plugin.fs is enabled, but runtime probe failed. Check data root mount or filesystem errors.";
  } else if (fsAPI && fsAPI.enabled) {
    interpretation = "Plugin.fs is enabled and runtime probe succeeded.";
  }

  let networkInterpretation = "Plugin.http object is missing in runtime.";
  if (httpAPI && !sandboxAllowNetwork) {
    networkInterpretation = "Sandbox allowNetwork=false. Network access was gated before requests.";
  } else if (httpAPI && !httpAPI.enabled) {
    networkInterpretation = "Plugin.http exists but enabled=false. Host disabled effective network access.";
  } else if (httpAPI && httpAPI.enabled) {
    networkInterpretation = "Plugin.http is enabled and ready for outbound requests.";
  }

  let hostInterpretation = "Plugin.host object is missing in runtime.";
  if (hostAPI && !hostAPI.enabled) {
    hostInterpretation = "Plugin.host exists but enabled=false. Host data bridge is disabled or permission-gated.";
  } else if (
    hostAPI &&
    hostAPI.enabled &&
    orderAPI &&
    userAPI &&
    productAPI &&
    inventoryAPI &&
    inventoryBindingAPI &&
    promoAPI &&
    ticketAPI &&
    serialAPI &&
    announcementAPI &&
    knowledgeAPI &&
    paymentMethodAPI &&
    virtualInventoryAPI &&
    virtualInventoryBindingAPI &&
    marketAPI &&
    emailTemplateAPI &&
    landingPageAPI &&
    invoiceTemplateAPI &&
    authBrandingAPI &&
    pageRulePackAPI
  ) {
    hostInterpretation = "Plugin.host bridge is enabled and all typed host helpers are available.";
  } else if (hostAPI && hostAPI.enabled) {
    hostInterpretation = "Plugin.host bridge is enabled, but some typed host helpers are missing.";
  }

  const missingRuntimeGlobals = [
    !hasWorkerGlobal ? "Worker" : "",
    !hasStructuredCloneGlobal ? "structuredClone" : "",
    !hasQueueMicrotaskGlobal ? "queueMicrotask" : "",
    !hasSetTimeoutGlobal ? "setTimeout" : "",
    !hasClearTimeoutGlobal ? "clearTimeout" : "",
    !hasTextEncoderGlobal ? "TextEncoder" : "",
    !hasTextDecoderGlobal ? "TextDecoder" : "",
    !hasAtobGlobal ? "atob" : "",
    !hasBtoaGlobal ? "btoa" : ""
  ].filter(Boolean);
  const jsRuntimeInterpretation =
    missingRuntimeGlobals.length === 0
      ? "Core JS runtime globals are available: Worker, timers, microtasks, encoding, base64, and structuredClone."
      : `Core JS runtime globals are incomplete. Missing: ${missingRuntimeGlobals.join(", ")}.`;

  return {
    hasPluginWorkspace: Boolean(workspaceAPI),
    hasPluginStorage: Boolean(runtimeGlobal?.storage),
    hasPluginSecret: Boolean(secretAPI),
    hasPluginWebhook: Boolean(webhookAPI),
    hasPluginHTTP: Boolean(httpAPI),
    hasPluginFS: Boolean(fsAPI),
    hasPluginHost: Boolean(hostAPI),
    hasPluginOrder: Boolean(orderAPI),
    hasPluginUser: Boolean(userAPI),
    hasPluginProduct: Boolean(productAPI),
    hasPluginInventory: Boolean(inventoryAPI),
    hasPluginInventoryBinding: Boolean(inventoryBindingAPI),
    hasPluginPromo: Boolean(promoAPI),
    hasPluginTicket: Boolean(ticketAPI),
    hasPluginSerial: Boolean(serialAPI),
    hasPluginAnnouncement: Boolean(announcementAPI),
    hasPluginKnowledge: Boolean(knowledgeAPI),
    hasPluginPaymentMethod: Boolean(paymentMethodAPI),
    hasPluginVirtualInventory: Boolean(virtualInventoryAPI),
    hasPluginVirtualInventoryBinding: Boolean(virtualInventoryBindingAPI),
    hasPluginMarket: Boolean(marketAPI),
    hasPluginEmailTemplate: Boolean(emailTemplateAPI),
    hasPluginLandingPage: Boolean(landingPageAPI),
    hasPluginInvoiceTemplate: Boolean(invoiceTemplateAPI),
    hasPluginAuthBranding: Boolean(authBrandingAPI),
    hasPluginPageRulePack: Boolean(pageRulePackAPI),
    hasWorkerGlobal,
    hasStructuredCloneGlobal,
    hasQueueMicrotaskGlobal,
    hasSetTimeoutGlobal,
    hasClearTimeoutGlobal,
    hasTextEncoderGlobal,
    hasTextDecoderGlobal,
    hasAtobGlobal,
    hasBtoaGlobal,
    pluginSecretEnabled: Boolean(secretAPI && secretAPI.enabled),
    pluginWebhookEnabled: Boolean(webhookAPI && webhookAPI.enabled),
    pluginWorkspaceEnabled: Boolean(workspaceAPI && workspaceAPI.enabled),
    pluginHTTPEnabled: Boolean(httpAPI && httpAPI.enabled),
    pluginFSEnabled: Boolean(fsAPI && fsAPI.enabled),
    pluginHostEnabled: Boolean(hostAPI && hostAPI.enabled),
    workspaceEntryCount,
    workspaceMaxEntries,
    sandboxAllowFileSystem,
    sandboxAllowNetwork,
    httpDefaultTimeoutMs: httpAPI && typeof httpAPI.defaultTimeoutMs === "number" ? httpAPI.defaultTimeoutMs : 0,
    httpMaxResponseBytes: httpAPI && typeof httpAPI.maxResponseBytes === "number" ? httpAPI.maxResponseBytes : 0,
    codeRoot: fsAPI && typeof fsAPI.codeRoot === "string" ? fsAPI.codeRoot : "",
    dataRoot: fsAPI && typeof fsAPI.dataRoot === "string" ? fsAPI.dataRoot : "",
    fsMaxFiles: fsAPI && typeof fsAPI.maxFiles === "number" ? fsAPI.maxFiles : 0,
    fsMaxTotalBytes: fsAPI && typeof fsAPI.maxTotalBytes === "number" ? fsAPI.maxTotalBytes : 0,
    fsMaxReadBytes: fsAPI && typeof fsAPI.maxReadBytes === "number" ? fsAPI.maxReadBytes : 0,
    pluginGlobalKeys,
    runtimeGlobalKeys,
    workspaceProbeError,
    workspaceInterpretation,
    fsProbeError,
    interpretation,
    networkInterpretation,
    hostInterpretation,
    jsRuntimeInterpretation
  };
}

export function buildSecretSummary(sampleKey = DEBUGGER_SECRET_SAMPLE_KEY): DebuggerSecretSummary {
  const secretAPI = getPluginSecret();
  if (!secretAPI) {
    return {
      present: false,
      enabled: false,
      key_count: 0,
      keys: [],
      sample_key: sampleKey,
      sample_present: false,
      interpretation: "Plugin.secret object is missing in runtime."
    };
  }
  if (!secretAPI.enabled) {
    return {
      present: true,
      enabled: false,
      key_count: 0,
      keys: [],
      sample_key: sampleKey,
      sample_present: false,
      interpretation: "Plugin.secret exists but enabled=false. No secret values are available for current runtime."
    };
  }
  try {
    const keys = secretAPI.list().map((item) => asString(item)).filter(Boolean).sort((a, b) => a.localeCompare(b));
    const samplePresent = typeof secretAPI.has === "function"
      ? secretAPI.has(sampleKey)
      : keys.includes(sampleKey);
    return {
      present: true,
      enabled: true,
      key_count: keys.length,
      keys,
      sample_key: sampleKey,
      sample_present: samplePresent,
      interpretation: keys.length > 0
        ? "Plugin.secret is enabled. Keys are visible but raw secret values remain hidden."
        : "Plugin.secret is enabled, but no secret keys are currently configured."
    };
  } catch (error) {
    return {
      present: true,
      enabled: true,
      key_count: 0,
      keys: [],
      sample_key: sampleKey,
      sample_present: false,
      interpretation: "Plugin.secret is enabled, but listing configured keys failed.",
      error: String((error as Error).message || error)
    };
  }
}

export function buildWebhookSummary(): DebuggerWebhookSummary {
  const webhookAPI = getPluginWebhook();
  if (!webhookAPI) {
    return {
      present: false,
      enabled: false,
      key: "",
      method: "",
      path: "",
      query_string: "",
      query_params: {},
      headers: {},
      header_count: 0,
      content_type: "",
      remote_addr: "",
      body_text_preview: "",
      interpretation: "Plugin.webhook object is missing in current runtime. This is expected outside webhook invocations."
    };
  }
  return {
    present: true,
    enabled: Boolean(webhookAPI.enabled),
    key: webhookAPI.key || "",
    method: webhookAPI.method || "",
    path: webhookAPI.path || "",
    query_string: webhookAPI.queryString || "",
    query_params: webhookAPI.queryParams || {},
    headers: webhookAPI.headers || {},
    header_count: Object.keys(webhookAPI.headers || {}).length,
    content_type: webhookAPI.contentType || "",
    remote_addr: webhookAPI.remoteAddr || "",
    body_text_preview: truncateText(webhookAPI.bodyText || "", 1200),
    body_json_preview: parseJSONValueIfPossible(webhookAPI.bodyText || ""),
    interpretation: webhookAPI.enabled
      ? "Plugin.webhook is attached to the current invocation. This snapshot reflects the live inbound request."
      : "Plugin.webhook exists but enabled=false."
  };
}

export function readFileSystemFormState(params: unknown): FileSystemFormState {
  const source = asRecord(params);
  const fsFormat = asString(source.fs_format).toLowerCase() === "json" ? "json" : "text";
  return {
    fs_path: asString(source.fs_path) || DEFAULT_FS_PATH,
    fs_content: typeof source.fs_content === "string" ? source.fs_content : "",
    fs_format: fsFormat
  };
}

export function readNetworkFormState(params: unknown): NetworkFormState {
  const source = asRecord(params);
  const bodyFormat = asString(source.network_body_format).toLowerCase() === "text" ? "text" : "json";
  return {
    network_method: asString(source.network_method).toUpperCase() || DEFAULT_NETWORK_FORM_STATE.network_method,
    network_url: asString(source.network_url) || DEFAULT_NETWORK_FORM_STATE.network_url,
    network_headers:
      typeof source.network_headers === "string"
        ? source.network_headers
        : DEFAULT_NETWORK_FORM_STATE.network_headers,
    network_body:
      typeof source.network_body === "string"
        ? source.network_body
        : DEFAULT_NETWORK_FORM_STATE.network_body,
    network_body_format: bodyFormat,
    network_timeout_ms: asInteger(
      source.network_timeout_ms,
      DEFAULT_NETWORK_FORM_STATE.network_timeout_ms,
      0,
      30000
    )
  };
}

export function readWorkerFormState(params: unknown): WorkerFormState {
  const source = asRecord(params);
  return {
    worker_script: asString(source.worker_script) || DEFAULT_WORKER_FORM_STATE.worker_script,
    worker_request_value: asInteger(
      source.worker_request_value,
      DEFAULT_WORKER_FORM_STATE.worker_request_value
    ),
    worker_second_value: asInteger(
      source.worker_second_value,
      DEFAULT_WORKER_FORM_STATE.worker_second_value
    ),
    worker_message_value: asInteger(
      source.worker_message_value,
      DEFAULT_WORKER_FORM_STATE.worker_message_value
    )
  };
}

function normalizeHostActionMode(value: unknown): HostFormState["host_mode"] {
  switch (asString(value).toLowerCase()) {
    case "order.list":
      return "order.list";
    case "order.assign_tracking":
      return "order.assign_tracking";
    case "order.request_resubmit":
      return "order.request_resubmit";
    case "order.mark_paid":
      return "order.mark_paid";
    case "order.update_price":
      return "order.update_price";
    case "user.get":
      return "user.get";
    case "user.list":
      return "user.list";
    case "product.get":
      return "product.get";
    case "product.list":
      return "product.list";
    case "inventory.get":
      return "inventory.get";
    case "inventory.list":
      return "inventory.list";
    case "inventory_binding.get":
      return "inventory_binding.get";
    case "inventory_binding.list":
      return "inventory_binding.list";
    case "promo.get":
      return "promo.get";
    case "promo.list":
      return "promo.list";
    case "ticket.get":
      return "ticket.get";
    case "ticket.list":
      return "ticket.list";
    case "ticket.reply":
      return "ticket.reply";
    case "ticket.update":
      return "ticket.update";
    case "serial.get":
      return "serial.get";
    case "serial.list":
      return "serial.list";
    case "announcement.get":
      return "announcement.get";
    case "announcement.list":
      return "announcement.list";
    case "knowledge.get":
      return "knowledge.get";
    case "knowledge.list":
      return "knowledge.list";
    case "knowledge.categories":
      return "knowledge.categories";
    case "payment_method.get":
      return "payment_method.get";
    case "payment_method.list":
      return "payment_method.list";
    case "virtual_inventory.get":
      return "virtual_inventory.get";
    case "virtual_inventory.list":
      return "virtual_inventory.list";
    case "virtual_inventory_binding.get":
      return "virtual_inventory_binding.get";
    case "virtual_inventory_binding.list":
      return "virtual_inventory_binding.list";
    case "market.source.list":
      return "market.source.list";
    case "market.source.get":
      return "market.source.get";
    case "market.catalog.list":
      return "market.catalog.list";
    case "market.artifact.get":
      return "market.artifact.get";
    case "market.release.get":
      return "market.release.get";
    case "market.install.preview":
      return "market.install.preview";
    case "market.install.execute":
      return "market.install.execute";
    case "market.install.task.get":
      return "market.install.task.get";
    case "market.install.task.list":
      return "market.install.task.list";
    case "market.install.history.list":
      return "market.install.history.list";
    case "market.install.rollback":
      return "market.install.rollback";
    case "email_template.list":
      return "email_template.list";
    case "email_template.get":
      return "email_template.get";
    case "email_template.save":
      return "email_template.save";
    case "landing_page.get":
      return "landing_page.get";
    case "landing_page.save":
      return "landing_page.save";
    case "landing_page.reset":
      return "landing_page.reset";
    case "invoice_template.get":
      return "invoice_template.get";
    case "invoice_template.save":
      return "invoice_template.save";
    case "invoice_template.reset":
      return "invoice_template.reset";
    case "auth_branding.get":
      return "auth_branding.get";
    case "auth_branding.save":
      return "auth_branding.save";
    case "auth_branding.reset":
      return "auth_branding.reset";
    case "page_rule_pack.get":
      return "page_rule_pack.get";
    case "page_rule_pack.save":
      return "page_rule_pack.save";
    case "page_rule_pack.reset":
      return "page_rule_pack.reset";
    case "host.invoke":
      return "host.invoke";
    default:
      return "order.get";
  }
}

function defaultHostActionForMode(mode: HostFormState["host_mode"]): string {
  switch (mode) {
    case "order.list":
      return "host.order.list";
    case "order.assign_tracking":
      return "host.order.assign_tracking";
    case "order.request_resubmit":
      return "host.order.request_resubmit";
    case "order.mark_paid":
      return "host.order.mark_paid";
    case "order.update_price":
      return "host.order.update_price";
    case "user.get":
      return "host.user.get";
    case "user.list":
      return "host.user.list";
    case "product.get":
      return "host.product.get";
    case "product.list":
      return "host.product.list";
    case "inventory.get":
      return "host.inventory.get";
    case "inventory.list":
      return "host.inventory.list";
    case "inventory_binding.get":
      return "host.inventory_binding.get";
    case "inventory_binding.list":
      return "host.inventory_binding.list";
    case "promo.get":
      return "host.promo.get";
    case "promo.list":
      return "host.promo.list";
    case "ticket.get":
      return "host.ticket.get";
    case "ticket.list":
      return "host.ticket.list";
    case "ticket.reply":
      return "host.ticket.reply";
    case "ticket.update":
      return "host.ticket.update";
    case "serial.get":
      return "host.serial.get";
    case "serial.list":
      return "host.serial.list";
    case "announcement.get":
      return "host.announcement.get";
    case "announcement.list":
      return "host.announcement.list";
    case "knowledge.get":
      return "host.knowledge.get";
    case "knowledge.list":
      return "host.knowledge.list";
    case "knowledge.categories":
      return "host.knowledge.categories";
    case "payment_method.get":
      return "host.payment_method.get";
    case "payment_method.list":
      return "host.payment_method.list";
    case "virtual_inventory.get":
      return "host.virtual_inventory.get";
    case "virtual_inventory.list":
      return "host.virtual_inventory.list";
    case "virtual_inventory_binding.get":
      return "host.virtual_inventory_binding.get";
    case "virtual_inventory_binding.list":
      return "host.virtual_inventory_binding.list";
    case "market.source.list":
      return "host.market.source.list";
    case "market.source.get":
      return "host.market.source.get";
    case "market.catalog.list":
      return "host.market.catalog.list";
    case "market.artifact.get":
      return "host.market.artifact.get";
    case "market.release.get":
      return "host.market.release.get";
    case "market.install.preview":
      return "host.market.install.preview";
    case "market.install.execute":
      return "host.market.install.execute";
    case "market.install.task.get":
      return "host.market.install.task.get";
    case "market.install.task.list":
      return "host.market.install.task.list";
    case "market.install.history.list":
      return "host.market.install.history.list";
    case "market.install.rollback":
      return "host.market.install.rollback";
    case "email_template.list":
      return "host.email_template.list";
    case "email_template.get":
      return "host.email_template.get";
    case "email_template.save":
      return "host.email_template.save";
    case "landing_page.get":
      return "host.landing_page.get";
    case "landing_page.save":
      return "host.landing_page.save";
    case "landing_page.reset":
      return "host.landing_page.reset";
    case "invoice_template.get":
      return "host.invoice_template.get";
    case "invoice_template.save":
      return "host.invoice_template.save";
    case "invoice_template.reset":
      return "host.invoice_template.reset";
    case "auth_branding.get":
      return "host.auth_branding.get";
    case "auth_branding.save":
      return "host.auth_branding.save";
    case "auth_branding.reset":
      return "host.auth_branding.reset";
    case "page_rule_pack.get":
      return "host.page_rule_pack.get";
    case "page_rule_pack.save":
      return "host.page_rule_pack.save";
    case "page_rule_pack.reset":
      return "host.page_rule_pack.reset";
    case "host.invoke":
      return "host.order.get";
    case "order.get":
    default:
      return "host.order.get";
  }
}

function parseHostPayload(raw: string): { ok: boolean; payload: GenericRecord; error?: string } {
  if (raw.trim() === "") {
    return {
      ok: true,
      payload: {}
    };
  }
  try {
    const decoded = JSON.parse(raw);
    if (!decoded || typeof decoded !== "object" || Array.isArray(decoded)) {
      return {
        ok: false,
        payload: {},
        error: "Host payload must be a JSON object."
      };
    }
    return {
      ok: true,
      payload: asRecord(decoded)
    };
  } catch {
    return {
      ok: false,
      payload: {},
      error: "Host payload must be valid JSON."
    };
  }
}

export function readHostFormState(params: unknown): HostFormState {
  const source = asRecord(params);
  const hostMode = normalizeHostActionMode(source.host_mode);
  return {
    host_mode: hostMode,
    host_action: asString(source.host_action) || defaultHostActionForMode(hostMode),
    host_payload:
      typeof source.host_payload === "string"
        ? source.host_payload
        : DEFAULT_HOST_FORM_STATE.host_payload
  };
}

function parseNetworkHeaders(raw: string): { ok: boolean; headers: Record<string, string>; error?: string } {
  if (raw.trim() === "") {
    return {
      ok: true,
      headers: {}
    };
  }
  try {
    const decoded = JSON.parse(raw);
    if (!decoded || typeof decoded !== "object" || Array.isArray(decoded)) {
      return {
        ok: false,
        headers: {},
        error: "Network headers must be a JSON object."
      };
    }
    const parsed = asRecord(decoded);
    const headers: Record<string, string> = {};
    Object.entries(parsed).forEach(([key, value]) => {
      const normalizedKey = asString(key);
      if (!normalizedKey) {
        return;
      }
      headers[normalizedKey] = asString(value);
    });
    return {
      ok: true,
      headers
    };
  } catch {
    return {
      ok: false,
      headers: {},
      error: "Network headers must be a JSON object."
    };
  }
}

function parseNetworkBody(input: NetworkFormState): { ok: boolean; body?: unknown; error?: string } {
  if (input.network_body.trim() === "") {
    return { ok: true };
  }
  if (input.network_body_format === "text") {
    return {
      ok: true,
      body: input.network_body
    };
  }
  try {
    return {
      ok: true,
      body: JSON.parse(input.network_body)
    };
  } catch {
    return {
      ok: false,
      error: "Network body is not valid JSON."
    };
  }
}

function normalizeNetworkResponse(value: unknown): DebuggerNetworkResponse {
  const source = asRecord(value);
  const rawHeaders = asRecord(source.headers);
  const headers: Record<string, string> = {};
  Object.entries(rawHeaders).forEach(([key, headerValue]) => {
    const normalizedKey = asString(key);
    if (!normalizedKey) {
      return;
    }
    headers[normalizedKey] = asString(headerValue);
  });

  return {
    ok: asBool(source.ok, false),
    url: asString(source.url),
    status: asInteger(source.status, 0, 0),
    statusText: asString(source.statusText),
    headers,
    body: typeof source.body === "string" ? source.body : prettyJSON(source.body ?? ""),
    data: source.data,
    error: asString(source.error) || undefined,
    duration_ms: asInteger(source.duration_ms, 0, 0),
    redirected: asBool(source.redirected, false)
  };
}

export function executeNetworkRequest(input: NetworkFormState): {
  ok: boolean;
  message: string;
  values: NetworkFormState;
  response: DebuggerNetworkResponse;
} {
  const httpAPI = getHTTPAPI();
  const disabledResponse: DebuggerNetworkResponse = {
    ok: false,
    url: input.network_url,
    status: 0,
    statusText: "",
    headers: {},
    body: "",
    error: "Plugin.http is disabled. Grant runtime.network and allow_network first.",
    duration_ms: 0,
    redirected: false
  };

  if (!httpAPI || !httpAPI.enabled) {
    return {
      ok: false,
      message: disabledResponse.error || "Plugin.http is unavailable.",
      values: input,
      response: disabledResponse
    };
  }

  const parsedHeaders = parseNetworkHeaders(input.network_headers);
  if (!parsedHeaders.ok) {
    return {
      ok: false,
      message: parsedHeaders.error || "Invalid network headers.",
      values: input,
      response: {
        ...disabledResponse,
        error: parsedHeaders.error
      }
    };
  }

  const parsedBody = parseNetworkBody(input);
  if (!parsedBody.ok) {
    return {
      ok: false,
      message: parsedBody.error || "Invalid network body.",
      values: input,
      response: {
        ...disabledResponse,
        error: parsedBody.error
      }
    };
  }

  const method = input.network_method.toUpperCase() || DEFAULT_NETWORK_FORM_STATE.network_method;
  const body = method === "GET" || method === "HEAD" ? undefined : parsedBody.body;
  const runtimeResponse = httpAPI.request({
    url: input.network_url,
    method,
    headers: parsedHeaders.headers,
    body,
    timeout_ms: input.network_timeout_ms > 0 ? input.network_timeout_ms : undefined
  });
  const response = normalizeNetworkResponse(runtimeResponse);

  return {
    ok: response.ok,
    message: response.error || `HTTP ${method} completed with status ${response.status}.`,
    values: input,
    response
  };
}

export function executeHostRequest(input: HostFormState): {
  ok: boolean;
  message: string;
  values: HostFormState;
  action: string;
  request_payload: GenericRecord;
  response: unknown;
} {
  const hostAPI = getPluginHost();
  const orderAPI = getPluginOrder();
  const userAPI = getPluginUser();
  const productAPI = getPluginProduct();
  const inventoryAPI = getPluginInventory();
  const inventoryBindingAPI = getPluginInventoryBinding();
  const promoAPI = getPluginPromo();
  const ticketAPI = getPluginTicket();
  const serialAPI = getPluginSerial();
  const announcementAPI = getPluginAnnouncement();
  const knowledgeAPI = getPluginKnowledge();
  const paymentMethodAPI = getPluginPaymentMethod();
  const virtualInventoryAPI = getPluginVirtualInventory();
  const virtualInventoryBindingAPI = getPluginVirtualInventoryBinding();
  const marketAPI = getPluginMarket();
  const emailTemplateAPI = getPluginEmailTemplate();
  const landingPageAPI = getPluginLandingPage();
  const invoiceTemplateAPI = getPluginInvoiceTemplate();
  const authBrandingAPI = getPluginAuthBranding();
  const pageRulePackAPI = getPluginPageRulePack();
  const parsedPayload = parseHostPayload(input.host_payload);
  const resolvedAction =
    input.host_mode === "host.invoke"
      ? asString(input.host_action)
      : defaultHostActionForMode(input.host_mode);

  if (!hostAPI || !hostAPI.enabled) {
    return {
      ok: false,
      message: "Plugin.host is disabled. Grant host.* permissions and ensure host bridge is injected first.",
      values: {
        ...input,
        host_action: resolvedAction || input.host_action
      },
      action: resolvedAction,
      request_payload: parsedPayload.payload,
      response: null
    };
  }
  if (!parsedPayload.ok) {
    return {
      ok: false,
      message: parsedPayload.error || "Invalid host payload.",
      values: {
        ...input,
        host_action: resolvedAction || input.host_action
      },
      action: resolvedAction,
      request_payload: {},
      response: null
    };
  }
  if (!resolvedAction) {
    return {
      ok: false,
      message: "Host action is required when using raw host.invoke mode.",
      values: input,
      action: "",
      request_payload: parsedPayload.payload,
      response: null
    };
  }

  try {
    let response: unknown = null;
    switch (input.host_mode) {
      case "order.get":
        if (!orderAPI || typeof orderAPI.get !== "function") {
          throw new Error("Plugin.order.get is unavailable in current runtime.");
        }
        response = orderAPI.get(parsedPayload.payload);
        break;
      case "order.list":
        if (!orderAPI || typeof orderAPI.list !== "function") {
          throw new Error("Plugin.order.list is unavailable in current runtime.");
        }
        response = orderAPI.list(parsedPayload.payload);
        break;
      case "order.assign_tracking":
        if (!orderAPI || typeof orderAPI.assignTracking !== "function") {
          throw new Error("Plugin.order.assignTracking is unavailable in current runtime.");
        }
        response = orderAPI.assignTracking(parsedPayload.payload);
        break;
      case "order.request_resubmit":
        if (!orderAPI || typeof orderAPI.requestResubmit !== "function") {
          throw new Error("Plugin.order.requestResubmit is unavailable in current runtime.");
        }
        response = orderAPI.requestResubmit(parsedPayload.payload);
        break;
      case "order.mark_paid":
        if (!orderAPI || typeof orderAPI.markPaid !== "function") {
          throw new Error("Plugin.order.markPaid is unavailable in current runtime.");
        }
        response = orderAPI.markPaid(parsedPayload.payload);
        break;
      case "order.update_price":
        if (!orderAPI || typeof orderAPI.updatePrice !== "function") {
          throw new Error("Plugin.order.updatePrice is unavailable in current runtime.");
        }
        response = orderAPI.updatePrice(parsedPayload.payload);
        break;
      case "user.get":
        if (!userAPI || typeof userAPI.get !== "function") {
          throw new Error("Plugin.user.get is unavailable in current runtime.");
        }
        response = userAPI.get(parsedPayload.payload);
        break;
      case "user.list":
        if (!userAPI || typeof userAPI.list !== "function") {
          throw new Error("Plugin.user.list is unavailable in current runtime.");
        }
        response = userAPI.list(parsedPayload.payload);
        break;
      case "product.get":
        if (!productAPI || typeof productAPI.get !== "function") {
          throw new Error("Plugin.product.get is unavailable in current runtime.");
        }
        response = productAPI.get(parsedPayload.payload);
        break;
      case "product.list":
        if (!productAPI || typeof productAPI.list !== "function") {
          throw new Error("Plugin.product.list is unavailable in current runtime.");
        }
        response = productAPI.list(parsedPayload.payload);
        break;
      case "inventory.get":
        if (!inventoryAPI || typeof inventoryAPI.get !== "function") {
          throw new Error("Plugin.inventory.get is unavailable in current runtime.");
        }
        response = inventoryAPI.get(parsedPayload.payload);
        break;
      case "inventory.list":
        if (!inventoryAPI || typeof inventoryAPI.list !== "function") {
          throw new Error("Plugin.inventory.list is unavailable in current runtime.");
        }
        response = inventoryAPI.list(parsedPayload.payload);
        break;
      case "inventory_binding.get":
        if (!inventoryBindingAPI || typeof inventoryBindingAPI.get !== "function") {
          throw new Error("Plugin.inventoryBinding.get is unavailable in current runtime.");
        }
        response = inventoryBindingAPI.get(parsedPayload.payload);
        break;
      case "inventory_binding.list":
        if (!inventoryBindingAPI || typeof inventoryBindingAPI.list !== "function") {
          throw new Error("Plugin.inventoryBinding.list is unavailable in current runtime.");
        }
        response = inventoryBindingAPI.list(parsedPayload.payload);
        break;
      case "promo.get":
        if (!promoAPI || typeof promoAPI.get !== "function") {
          throw new Error("Plugin.promo.get is unavailable in current runtime.");
        }
        response = promoAPI.get(parsedPayload.payload);
        break;
      case "promo.list":
        if (!promoAPI || typeof promoAPI.list !== "function") {
          throw new Error("Plugin.promo.list is unavailable in current runtime.");
        }
        response = promoAPI.list(parsedPayload.payload);
        break;
      case "ticket.get":
        if (!ticketAPI || typeof ticketAPI.get !== "function") {
          throw new Error("Plugin.ticket.get is unavailable in current runtime.");
        }
        response = ticketAPI.get(parsedPayload.payload);
        break;
      case "ticket.list":
        if (!ticketAPI || typeof ticketAPI.list !== "function") {
          throw new Error("Plugin.ticket.list is unavailable in current runtime.");
        }
        response = ticketAPI.list(parsedPayload.payload);
        break;
      case "ticket.reply":
        if (!ticketAPI || typeof ticketAPI.reply !== "function") {
          throw new Error("Plugin.ticket.reply is unavailable in current runtime.");
        }
        response = ticketAPI.reply(parsedPayload.payload);
        break;
      case "ticket.update":
        if (!ticketAPI || typeof ticketAPI.update !== "function") {
          throw new Error("Plugin.ticket.update is unavailable in current runtime.");
        }
        response = ticketAPI.update(parsedPayload.payload);
        break;
      case "serial.get":
        if (!serialAPI || typeof serialAPI.get !== "function") {
          throw new Error("Plugin.serial.get is unavailable in current runtime.");
        }
        response = serialAPI.get(parsedPayload.payload);
        break;
      case "serial.list":
        if (!serialAPI || typeof serialAPI.list !== "function") {
          throw new Error("Plugin.serial.list is unavailable in current runtime.");
        }
        response = serialAPI.list(parsedPayload.payload);
        break;
      case "announcement.get":
        if (!announcementAPI || typeof announcementAPI.get !== "function") {
          throw new Error("Plugin.announcement.get is unavailable in current runtime.");
        }
        response = announcementAPI.get(parsedPayload.payload);
        break;
      case "announcement.list":
        if (!announcementAPI || typeof announcementAPI.list !== "function") {
          throw new Error("Plugin.announcement.list is unavailable in current runtime.");
        }
        response = announcementAPI.list(parsedPayload.payload);
        break;
      case "knowledge.get":
        if (!knowledgeAPI || typeof knowledgeAPI.get !== "function") {
          throw new Error("Plugin.knowledge.get is unavailable in current runtime.");
        }
        response = knowledgeAPI.get(parsedPayload.payload);
        break;
      case "knowledge.list":
        if (!knowledgeAPI || typeof knowledgeAPI.list !== "function") {
          throw new Error("Plugin.knowledge.list is unavailable in current runtime.");
        }
        response = knowledgeAPI.list(parsedPayload.payload);
        break;
      case "knowledge.categories":
        if (!knowledgeAPI || typeof knowledgeAPI.categories !== "function") {
          throw new Error("Plugin.knowledge.categories is unavailable in current runtime.");
        }
        response = knowledgeAPI.categories(parsedPayload.payload);
        break;
      case "payment_method.get":
        if (!paymentMethodAPI || typeof paymentMethodAPI.get !== "function") {
          throw new Error("Plugin.paymentMethod.get is unavailable in current runtime.");
        }
        response = paymentMethodAPI.get(parsedPayload.payload);
        break;
      case "payment_method.list":
        if (!paymentMethodAPI || typeof paymentMethodAPI.list !== "function") {
          throw new Error("Plugin.paymentMethod.list is unavailable in current runtime.");
        }
        response = paymentMethodAPI.list(parsedPayload.payload);
        break;
      case "virtual_inventory.get":
        if (!virtualInventoryAPI || typeof virtualInventoryAPI.get !== "function") {
          throw new Error("Plugin.virtualInventory.get is unavailable in current runtime.");
        }
        response = virtualInventoryAPI.get(parsedPayload.payload);
        break;
      case "virtual_inventory.list":
        if (!virtualInventoryAPI || typeof virtualInventoryAPI.list !== "function") {
          throw new Error("Plugin.virtualInventory.list is unavailable in current runtime.");
        }
        response = virtualInventoryAPI.list(parsedPayload.payload);
        break;
      case "virtual_inventory_binding.get":
        if (!virtualInventoryBindingAPI || typeof virtualInventoryBindingAPI.get !== "function") {
          throw new Error("Plugin.virtualInventoryBinding.get is unavailable in current runtime.");
        }
        response = virtualInventoryBindingAPI.get(parsedPayload.payload);
        break;
      case "virtual_inventory_binding.list":
        if (!virtualInventoryBindingAPI || typeof virtualInventoryBindingAPI.list !== "function") {
          throw new Error("Plugin.virtualInventoryBinding.list is unavailable in current runtime.");
        }
        response = virtualInventoryBindingAPI.list(parsedPayload.payload);
        break;
      case "market.source.list":
        if (!marketAPI?.source || typeof marketAPI.source.list !== "function") {
          throw new Error("Plugin.market.source.list is unavailable in current runtime.");
        }
        response = marketAPI.source.list(parsedPayload.payload);
        break;
      case "market.source.get":
        if (!marketAPI?.source || typeof marketAPI.source.get !== "function") {
          throw new Error("Plugin.market.source.get is unavailable in current runtime.");
        }
        response = marketAPI.source.get(parsedPayload.payload);
        break;
      case "market.catalog.list":
        if (!marketAPI?.catalog || typeof marketAPI.catalog.list !== "function") {
          throw new Error("Plugin.market.catalog.list is unavailable in current runtime.");
        }
        response = marketAPI.catalog.list(parsedPayload.payload);
        break;
      case "market.artifact.get":
        if (!marketAPI?.artifact || typeof marketAPI.artifact.get !== "function") {
          throw new Error("Plugin.market.artifact.get is unavailable in current runtime.");
        }
        response = marketAPI.artifact.get(parsedPayload.payload as any);
        break;
      case "market.release.get":
        if (!marketAPI?.release || typeof marketAPI.release.get !== "function") {
          throw new Error("Plugin.market.release.get is unavailable in current runtime.");
        }
        response = marketAPI.release.get(parsedPayload.payload as any);
        break;
      case "market.install.preview":
        if (!marketAPI?.install || typeof marketAPI.install.preview !== "function") {
          throw new Error("Plugin.market.install.preview is unavailable in current runtime.");
        }
        response = marketAPI.install.preview(parsedPayload.payload as any);
        break;
      case "market.install.execute":
        if (!marketAPI?.install || typeof marketAPI.install.execute !== "function") {
          throw new Error("Plugin.market.install.execute is unavailable in current runtime.");
        }
        response = marketAPI.install.execute(parsedPayload.payload as any);
        break;
      case "market.install.task.get":
        if (!marketAPI?.install?.task || typeof marketAPI.install.task.get !== "function") {
          throw new Error("Plugin.market.install.task.get is unavailable in current runtime.");
        }
        response = marketAPI.install.task.get(parsedPayload.payload);
        break;
      case "market.install.task.list":
        if (!marketAPI?.install?.task || typeof marketAPI.install.task.list !== "function") {
          throw new Error("Plugin.market.install.task.list is unavailable in current runtime.");
        }
        response = marketAPI.install.task.list(parsedPayload.payload);
        break;
      case "market.install.history.list":
        if (!marketAPI?.install?.history || typeof marketAPI.install.history.list !== "function") {
          throw new Error("Plugin.market.install.history.list is unavailable in current runtime.");
        }
        response = marketAPI.install.history.list(parsedPayload.payload);
        break;
      case "market.install.rollback":
        if (!marketAPI?.install || typeof marketAPI.install.rollback !== "function") {
          throw new Error("Plugin.market.install.rollback is unavailable in current runtime.");
        }
        response = marketAPI.install.rollback(parsedPayload.payload as any);
        break;
      case "email_template.list":
        if (!emailTemplateAPI || typeof emailTemplateAPI.list !== "function") {
          throw new Error("Plugin.emailTemplate.list is unavailable in current runtime.");
        }
        response = emailTemplateAPI.list(parsedPayload.payload);
        break;
      case "email_template.get":
        if (!emailTemplateAPI || typeof emailTemplateAPI.get !== "function") {
          throw new Error("Plugin.emailTemplate.get is unavailable in current runtime.");
        }
        response = emailTemplateAPI.get(parsedPayload.payload);
        break;
      case "email_template.save":
        if (!emailTemplateAPI || typeof emailTemplateAPI.save !== "function") {
          throw new Error("Plugin.emailTemplate.save is unavailable in current runtime.");
        }
        response = emailTemplateAPI.save(parsedPayload.payload as any);
        break;
      case "landing_page.get":
        if (!landingPageAPI || typeof landingPageAPI.get !== "function") {
          throw new Error("Plugin.landingPage.get is unavailable in current runtime.");
        }
        response = landingPageAPI.get(parsedPayload.payload);
        break;
      case "landing_page.save":
        if (!landingPageAPI || typeof landingPageAPI.save !== "function") {
          throw new Error("Plugin.landingPage.save is unavailable in current runtime.");
        }
        response = landingPageAPI.save(parsedPayload.payload);
        break;
      case "landing_page.reset":
        if (!landingPageAPI || typeof landingPageAPI.reset !== "function") {
          throw new Error("Plugin.landingPage.reset is unavailable in current runtime.");
        }
        response = landingPageAPI.reset(parsedPayload.payload);
        break;
      case "invoice_template.get":
        if (!invoiceTemplateAPI || typeof invoiceTemplateAPI.get !== "function") {
          throw new Error("Plugin.invoiceTemplate.get is unavailable in current runtime.");
        }
        response = invoiceTemplateAPI.get(parsedPayload.payload);
        break;
      case "invoice_template.save":
        if (!invoiceTemplateAPI || typeof invoiceTemplateAPI.save !== "function") {
          throw new Error("Plugin.invoiceTemplate.save is unavailable in current runtime.");
        }
        response = invoiceTemplateAPI.save(parsedPayload.payload as any);
        break;
      case "invoice_template.reset":
        if (!invoiceTemplateAPI || typeof invoiceTemplateAPI.reset !== "function") {
          throw new Error("Plugin.invoiceTemplate.reset is unavailable in current runtime.");
        }
        response = invoiceTemplateAPI.reset(parsedPayload.payload);
        break;
      case "auth_branding.get":
        if (!authBrandingAPI || typeof authBrandingAPI.get !== "function") {
          throw new Error("Plugin.authBranding.get is unavailable in current runtime.");
        }
        response = authBrandingAPI.get(parsedPayload.payload);
        break;
      case "auth_branding.save":
        if (!authBrandingAPI || typeof authBrandingAPI.save !== "function") {
          throw new Error("Plugin.authBranding.save is unavailable in current runtime.");
        }
        response = authBrandingAPI.save(parsedPayload.payload as any);
        break;
      case "auth_branding.reset":
        if (!authBrandingAPI || typeof authBrandingAPI.reset !== "function") {
          throw new Error("Plugin.authBranding.reset is unavailable in current runtime.");
        }
        response = authBrandingAPI.reset(parsedPayload.payload);
        break;
      case "page_rule_pack.get":
        if (!pageRulePackAPI || typeof pageRulePackAPI.get !== "function") {
          throw new Error("Plugin.pageRulePack.get is unavailable in current runtime.");
        }
        response = pageRulePackAPI.get(parsedPayload.payload);
        break;
      case "page_rule_pack.save":
        if (!pageRulePackAPI || typeof pageRulePackAPI.save !== "function") {
          throw new Error("Plugin.pageRulePack.save is unavailable in current runtime.");
        }
        response = pageRulePackAPI.save(parsedPayload.payload);
        break;
      case "page_rule_pack.reset":
        if (!pageRulePackAPI || typeof pageRulePackAPI.reset !== "function") {
          throw new Error("Plugin.pageRulePack.reset is unavailable in current runtime.");
        }
        response = pageRulePackAPI.reset(parsedPayload.payload);
        break;
      case "host.invoke":
      default:
        if (typeof hostAPI.invoke !== "function") {
          throw new Error("Plugin.host.invoke is unavailable in current runtime.");
        }
        response = hostAPI.invoke(resolvedAction, parsedPayload.payload);
        break;
    }

    return {
      ok: true,
      message: `${resolvedAction} completed successfully.`,
      values: {
        ...input,
        host_action: resolvedAction
      },
      action: resolvedAction,
      request_payload: parsedPayload.payload,
      response
    };
  } catch (error) {
    return {
      ok: false,
      message: String((error as Error).message || error),
      values: {
        ...input,
        host_action: resolvedAction || input.host_action
      },
      action: resolvedAction,
      request_payload: parsedPayload.payload,
      response: null
    };
  }
}

function writeFSValue(input: FileSystemFormState): { ok: boolean; message: string } {
  const fsAPI = getFSAPI();
  if (!fsAPI || !fsAPI.enabled) {
    return {
      ok: false,
      message: "Plugin.fs is disabled. Grant runtime.file_system and allow_file_system first."
    };
  }
  try {
    if (input.fs_format === "json") {
      fsAPI.writeJSON(input.fs_path, parseJSONValueIfPossible(input.fs_content));
    } else {
      fsAPI.writeText(input.fs_path, input.fs_content);
    }
    return {
      ok: true,
      message: `Wrote ${input.fs_path}.`
    };
  } catch (error) {
    return {
      ok: false,
      message: String((error as Error).message || error)
    };
  }
}

export function inspectFile(input: FileSystemFormState): {
  ok: boolean;
  message: string;
  values: FileSystemFormState;
  summary: DebuggerFileSummary;
} {
  const fsAPI = getFSAPI();
  if (!fsAPI || !fsAPI.enabled) {
    return {
      ok: false,
      message: "Plugin.fs is disabled. Grant runtime.file_system and allow_file_system first.",
      values: input,
      summary: buildFileSummary()
    };
  }
  try {
    const content = fsAPI.exists(input.fs_path) ? fsAPI.readText(input.fs_path) : "";
    return {
      ok: true,
      message: fsAPI.exists(input.fs_path) ? `Loaded ${input.fs_path}.` : `${input.fs_path} does not exist yet.`,
      values: {
        ...input,
        fs_content: content
      },
      summary: buildFileSummary()
    };
  } catch (error) {
    return {
      ok: false,
      message: String((error as Error).message || error),
      values: input,
      summary: buildFileSummary()
    };
  }
}

export function upsertFile(input: FileSystemFormState): {
  ok: boolean;
  message: string;
  values: FileSystemFormState;
  summary: DebuggerFileSummary;
} {
  const writeResult = writeFSValue(input);
  return {
    ok: writeResult.ok,
    message: writeResult.message,
    values: input,
    summary: buildFileSummary()
  };
}

export function deleteFile(input: FileSystemFormState): {
  ok: boolean;
  message: string;
  values: FileSystemFormState;
  summary: DebuggerFileSummary;
} {
  const fsAPI = getFSAPI();
  if (!fsAPI || !fsAPI.enabled) {
    return {
      ok: false,
      message: "Plugin.fs is disabled. Grant runtime.file_system and allow_file_system first.",
      values: input,
      summary: buildFileSummary()
    };
  }
  try {
    const removed = fsAPI.delete(input.fs_path);
    return {
      ok: removed,
      message: removed ? `Deleted ${input.fs_path}.` : `Nothing deleted for ${input.fs_path}.`,
      values: {
        ...input,
        fs_content: ""
      },
      summary: buildFileSummary()
    };
  } catch (error) {
    return {
      ok: false,
      message: String((error as Error).message || error),
      values: input,
      summary: buildFileSummary()
    };
  }
}

export function inspectFileSystem(): {
  ok: boolean;
  message: string;
  summary: DebuggerFileSummary;
} {
  const summary = buildFileSummary();
  return {
    ok: summary.enabled,
    message: summary.enabled
      ? "Loaded filesystem snapshot."
      : "Plugin.fs is disabled. Grant runtime.file_system and allow_file_system first.",
    summary
  };
}

export function seedFileSystem(profile: DebuggerProfile): {
  ok: boolean;
  message: string;
  values: FileSystemFormState;
  summary: DebuggerFileSummary;
} {
  const input: FileSystemFormState = {
    fs_path: DEFAULT_FS_PATH,
    fs_format: "json",
    fs_content: prettyJSON({
      generated_at: nowISO(),
      storage_keys: profile.storage.keys,
      recent_hook: profile.recent_events[0]?.hook || null,
      missing_permissions: profile.capability_gaps
    })
  };
  const result = upsertFile(input);
  return {
    ok: result.ok,
    message: result.message,
    values: input,
    summary: result.summary
  };
}

export function latestEventSnapshot(events: DebuggerEvent[]): GenericRecord | null {
  const latest = events[0];
  if (!latest) {
    return null;
  }
  return {
    hook: latest.hook,
    time: latest.ts,
    blocked: latest.blocked || false,
    note: latest.note || "",
    payload: parseJSONValueIfPossible(latest.payload_json || ""),
    context: parseJSONValueIfPossible(latest.context_json || "")
  };
}

export const DEBUGGER_RESERVED_KEYS = {
  config: CONFIG_STORAGE_KEY,
  events: EVENT_STORAGE_KEY
};
