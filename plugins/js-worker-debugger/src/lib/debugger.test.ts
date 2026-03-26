import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";

import {
  OFFICIAL_FRONTEND_SLOTS,
  OFFICIAL_PLUGIN_HOOKS,
  inspectPluginManifestCompatibility,
  validatePluginManifestCatalog,
  validatePluginManifestSchema,
  type PluginAuthBrandingAPI,
  type PluginAnnouncementAPI,
  type PluginEmailTemplateAPI,
  type PluginEmailTemplateSaveParams,
  type PluginExecutionContext,
  type PluginExecuteStreamWriter,
  type PluginHostAPI,
  type PluginInventoryBindingAPI,
  type PluginInvoiceTemplateAPI,
  type PluginKnowledgeAPI,
  type PluginLandingPageAPI,
  type PluginMarketAPI,
  type PluginPaymentMethodAPI,
  type PluginPageRulePackAPI,
  type PluginOrderAPI,
  type PluginProductAPI,
  type PluginRuntimeGlobal,
  type PluginSandboxProfile,
  type PluginSecretAPI,
  type PluginStorageAPI,
  type PluginTicketAPI,
  type PluginUserAPI,
  type PluginWebhookAPI,
  type PluginWorkspaceAPI,
  type PluginWorkspaceCommandContext,
  type PluginVirtualInventoryAPI,
  type PluginVirtualInventoryBindingAPI
} from "@auralogic/plugin-sdk";

import { execute, executeStream, workspaceHandlers } from "./debugger";
import { readActionTraces, readDebuggerEvents } from "./debugger-state";
import type { GenericRecord, PluginPageBlock } from "./types";

type RuntimeGlobal = {
  Plugin?: PluginRuntimeGlobal;
  sandbox?: PluginSandboxProfile;
};

const patchedGlobalDescriptors = new Map<string, PropertyDescriptor | undefined>();

function getRuntime(): RuntimeGlobal {
  return globalThis as unknown as RuntimeGlobal;
}

function createMemoryStorage(): PluginStorageAPI {
  const values = new Map<string, string>();
  return {
    get(key) {
      return values.get(key);
    },
    set(key, value) {
      values.set(key, value);
      return true;
    },
    delete(key) {
      return values.delete(key);
    },
    list() {
      return Array.from(values.keys());
    },
    clear() {
      values.clear();
      return true;
    }
  };
}

function patchGlobalValue(key: string, value: unknown): void {
  if (!patchedGlobalDescriptors.has(key)) {
    patchedGlobalDescriptors.set(key, Object.getOwnPropertyDescriptor(globalThis, key));
  }
  Object.defineProperty(globalThis, key, {
    configurable: true,
    writable: true,
    value
  });
}

function installRuntime(
  pluginPatch?: Partial<PluginRuntimeGlobal>,
  globalPatch?: Record<string, unknown>
): PluginStorageAPI {
  const runtime = getRuntime();
  const storage = createMemoryStorage();
  runtime.Plugin = {
    storage,
    ...(pluginPatch || {})
  } as PluginRuntimeGlobal;
  runtime.sandbox = undefined;
  Object.entries(globalPatch || {}).forEach(([key, value]) => {
    patchGlobalValue(key, value);
  });
  return storage;
}

function resetRuntime(): void {
  const runtime = getRuntime();
  runtime.Plugin = undefined;
  runtime.sandbox = undefined;
  for (const [key, descriptor] of patchedGlobalDescriptors.entries()) {
    if (descriptor) {
      Object.defineProperty(globalThis, key, descriptor);
    } else {
      delete (globalThis as Record<string, unknown>)[key];
    }
  }
  patchedGlobalDescriptors.clear();
}

function createWorkspaceStub(): {
  workspace: PluginWorkspaceAPI;
  entries: Array<{ level: string; message: string }>;
} {
  const entries: Array<{ level: string; message: string }> = [];
  const push = (level: string, message: string) => {
    entries.push({ level, message });
  };
  return {
    workspace: {
      enabled: true,
      write(message) {
        push("write", String(message || ""));
      },
      writeln(message) {
        push("writeln", String(message || ""));
      },
      info(message) {
        push("info", String(message || ""));
      },
      warn(message) {
        push("warn", String(message || ""));
      },
      error(message) {
        push("error", String(message || ""));
      },
      clear() {
        entries.length = 0;
        return true;
      },
      tail() {
        return entries.map((entry) => ({
          level: entry.level,
          message: entry.message
        }));
      },
      snapshot() {
        return {
          enabled: true,
          max_entries: 200,
          entry_count: entries.length,
          entries: entries.map((entry) => ({
            level: entry.level,
            message: entry.message
          }))
        };
      },
      read() {
        return "";
      },
      readLine() {
        return "debug-input";
      }
    },
    entries
  };
}

function readRepoText(relativePath: string): string {
  return readFileSync(path.resolve(__dirname, "../../../../", relativePath), "utf8");
}

type CatalogSnapshot = {
  plugin_hooks: string[];
  frontend_slots: string[];
  plugin_permission_keys: string[];
  host_permission_keys: string[];
};

let cachedCatalogSnapshot: CatalogSnapshot | null = null;

function readCatalogSnapshot(): CatalogSnapshot {
  if (cachedCatalogSnapshot) {
    return cachedCatalogSnapshot;
  }
  const scriptPath = path.resolve(__dirname, "../../../../plugins/sdk/scripts/catalog-source.mjs");
  const output = execFileSync(process.execPath, [scriptPath, "--json"], {
    encoding: "utf8",
    cwd: path.resolve(__dirname, "../../../../plugins/sdk")
  });
  cachedCatalogSnapshot = JSON.parse(output) as CatalogSnapshot;
  return cachedCatalogSnapshot;
}

function buildContextMetadata(overrides?: Record<string, string>): Record<string, string> {
  return {
    request_path: "/api/admin/plugins/7/execute",
    plugin_page_path: "/admin/plugin-pages/debugger/orders/ORD-1001",
    plugin_page_full_path: "/admin/plugin-pages/debugger/orders/ORD-1001?order_id=123&tab=timeline",
    plugin_page_query_string: "order_id=123&tab=timeline",
    plugin_page_query_params: "{\"order_id\":\"123\",\"tab\":\"timeline\"}",
    plugin_page_route_params: "{\"orderNo\":\"ORD-1001\"}",
    bootstrap_area: "admin",
    ...overrides
  };
}

function buildContext(overrides?: Partial<PluginExecutionContext>): PluginExecutionContext {
  return {
    user_id: 7,
    order_id: 321,
    session_id: "sess-debugger",
    metadata: buildContextMetadata(),
    ...overrides
  };
}

function buildSandbox(overrides?: Partial<PluginSandboxProfile>): PluginSandboxProfile {
  return {
    level: "balanced",
    currentAction: "debugger.echo",
    declaredStorageAccessMode: "read",
    storageAccessMode: "read",
    allowExecuteAPI: true,
    allowFrontendExtensions: true,
    allowHookExecute: true,
    allowHookBlock: true,
    allowPayloadPatch: true,
    allowFileSystem: false,
    allowNetwork: false,
    requestedPermissions: ["api.execute", "frontend.extensions"],
    grantedPermissions: ["api.execute", "frontend.extensions"],
    executeActionStorage: {
      "debugger.echo": "read",
      "debugger.echo.stream": "read",
      "debugger.worker.roundtrip": "read",
      "hook.execute": "write"
    },
    defaultTimeoutMs: 5000,
    maxConcurrency: 1,
    maxMemoryMB: 64,
    fsMaxFiles: 0,
    fsMaxTotalBytes: 0,
    fsMaxReadBytes: 0,
    storageMaxKeys: 32,
    storageMaxTotalBytes: 65536,
    storageMaxValueBytes: 8192,
    ...overrides
  };
}

function extractBlocks(result: GenericRecord): PluginPageBlock[] {
  const data = result.data as GenericRecord | undefined;
  const blocks = data?.blocks;
  return Array.isArray(blocks) ? (blocks as PluginPageBlock[]) : [];
}

function findBlock(blocks: PluginPageBlock[], title: string): PluginPageBlock {
  const found = blocks.find((block) => block.title === title);
  assert.ok(found, `expected block ${title}`);
  return found;
}

function getKeyValueMap(block: PluginPageBlock): Record<string, unknown> {
  const data = block.data as GenericRecord | undefined;
  const items = Array.isArray(data?.items) ? data.items : [];
  return items.reduce<Record<string, unknown>>((acc, item) => {
    if (!item || typeof item !== "object" || Array.isArray(item)) {
      return acc;
    }
    const entry = item as Record<string, unknown>;
    const key = typeof entry.key === "string" ? entry.key : "";
    if (!key) {
      return acc;
    }
    acc[key] = entry.value;
    return acc;
  }, {});
}

function parseJSONBlock(block: PluginPageBlock): unknown {
  const data = block.data as GenericRecord | undefined;
  return data?.value;
}

function createStreamWriter() {
  const writes: Array<{ data: unknown; metadata?: Record<string, unknown> }> = [];
  const progress: Array<{ status: string; progress?: number; metadata?: Record<string, unknown> }> = [];
  const writer: PluginExecuteStreamWriter = {
    write(data, metadata) {
      writes.push({ data, metadata: (metadata ?? undefined) as Record<string, unknown> | undefined });
    },
    emit(data, metadata) {
      writes.push({ data, metadata: (metadata ?? undefined) as Record<string, unknown> | undefined });
    },
    progress(status, value, metadata) {
      progress.push({
        status,
        progress: typeof value === "number" ? value : undefined,
        metadata: (metadata ?? undefined) as Record<string, unknown> | undefined
      });
    }
  };
  return { writer, writes, progress };
}

test.afterEach(() => {
  resetRuntime();
});

test("debugger echo surfaces plugin page parameters in execution blocks", () => {
  installRuntime();

  const result = execute(
    "debugger.echo",
    {
      echo_message: "inspect plugin params"
    },
    buildContext(),
    {},
    buildSandbox()
  );

  assert.equal(result.success, true);
  const blocks = extractBlocks(result);
  const executionContextBlock = findBlock(blocks, "Execution Context");
  const executionContextValues = getKeyValueMap(executionContextBlock);
  assert.equal(
    executionContextValues.plugin_page_full_path,
    "/admin/plugin-pages/debugger/orders/ORD-1001?order_id=123&tab=timeline"
  );
  assert.equal(executionContextValues.plugin_page_query_string, "order_id=123&tab=timeline");
  assert.equal(
    executionContextValues.plugin_page_query_params,
    "{\"order_id\":\"123\",\"tab\":\"timeline\"}"
  );
  assert.equal(
    executionContextValues.plugin_page_route_params,
    "{\"orderNo\":\"ORD-1001\"}"
  );

  const contextMetadataBlock = findBlock(blocks, "Context Metadata");
  assert.deepEqual(parseJSONBlock(contextMetadataBlock), buildContextMetadata());
});

test("debugger hook execute persists frontend payload query and route params", () => {
  installRuntime();

  const result = execute(
    "hook.execute",
    {
      hook: "frontend.slot.render",
      payload: JSON.stringify({
        area: "admin",
        slot: "admin.plugin_page.top",
        path: "/admin/plugin-pages/debugger/orders/ORD-1001",
        full_path: "/admin/plugin-pages/debugger/orders/ORD-1001?order_id=123&tab=timeline",
        query_params: {
          order_id: "123",
          tab: "timeline"
        },
        route_params: {
          orderNo: "ORD-1001"
        }
      })
    },
    buildContext(),
    {},
    buildSandbox({
      currentAction: "hook.execute"
    })
  );

  assert.equal(result.success, true);
  const events = readDebuggerEvents();
  assert.equal(events.length, 1);
  const persistedEvent = events[0];
  assert.ok(persistedEvent);

  const persistedPayload = JSON.parse(persistedEvent.payload_json || "{}") as Record<string, unknown>;
  assert.deepEqual(persistedPayload.query_params, {
    order_id: "123",
    tab: "timeline"
  });
  assert.deepEqual(persistedPayload.route_params, {
    orderNo: "ORD-1001"
  });
  assert.equal(
    persistedPayload.full_path,
    "/admin/plugin-pages/debugger/orders/ORD-1001?order_id=123&tab=timeline"
  );

  const persistedContext = JSON.parse(persistedEvent.context_json || "{}") as Record<string, unknown>;
  assert.equal(
    (persistedContext.metadata as Record<string, unknown>).plugin_page_query_params,
    "{\"order_id\":\"123\",\"tab\":\"timeline\"}"
  );
  assert.equal(
    (persistedContext.metadata as Record<string, unknown>).plugin_page_route_params,
    "{\"orderNo\":\"ORD-1001\"}"
  );
});

test("debugger echo stream exposes plugin page params to stream bridge", () => {
  installRuntime();

  const stream = createStreamWriter();
  const result = executeStream(
    "debugger.echo.stream",
    {
      echo_message: "stream param check"
    },
    buildContext(),
    {},
    buildSandbox({
      currentAction: "debugger.echo.stream"
    }),
    stream.writer
  );

  assert.equal(result.success, true);
  assert.ok(stream.progress.length >= 2);
  assert.ok(stream.writes.length >= 1);

  const contextChunk = stream.writes.find((entry) => {
    const data = entry.data as Record<string, unknown> | undefined;
    return data?.status === "collecting-context";
  });
  assert.ok(contextChunk, "expected collecting-context stream chunk");

  const metadataKeys = Array.isArray((contextChunk.data as Record<string, unknown>).metadata_keys)
    ? ((contextChunk.data as Record<string, unknown>).metadata_keys as unknown[])
    : [];
  assert.ok(metadataKeys.includes("plugin_page_query_params"));
  assert.ok(metadataKeys.includes("plugin_page_route_params"));

  const blocks = extractBlocks(result);
  const contextMetadataBlock = findBlock(blocks, "Context Metadata");
  assert.deepEqual(parseJSONBlock(contextMetadataBlock), buildContextMetadata());
});

test("debugger host lab executes Plugin.order helper and surfaces host probe state", () => {
  const orderAPI: PluginOrderAPI = {
    get: <T = unknown>(query: number | Record<string, unknown>) =>
      ({
        id: typeof query === "number" ? query : Number((query as Record<string, unknown>).id || 0),
        order_no: "ORD-42",
        status: "pending"
      }) as T,
    list: <T = unknown>() =>
      ({
        items: [],
        page: 1,
        page_size: 20,
        total: 0,
        has_more: false
      }) as T,
    assignTracking: <T = unknown>(payload: Record<string, unknown>) =>
      ({
        id: Number(payload.id || 0),
        order_no: "ORD-42",
        tracking_no: String(payload.tracking_no || payload.trackingNo || ""),
        status: "shipped",
        shipped_at: "2026-03-13T12:00:00Z"
      }) as T,
    requestResubmit: <T = unknown>(payload: Record<string, unknown>) =>
      ({
        id: Number(payload.id || 0),
        order_no: "ORD-42",
        status: "need_resubmit",
        form_expires_at: "2026-03-14T12:00:00Z",
        reason: String(payload.reason || "")
      }) as T,
    markPaid: <T = unknown>() =>
      ({
        id: 42,
        order_no: "ORD-42",
        status: "pending",
        total_amount_minor: 1999
      }) as T,
    updatePrice: <T = unknown>(payload: Record<string, unknown>) =>
      ({
        id: 42,
        order_no: "ORD-42",
        status: "pending_payment",
        total_amount_minor: Number(payload.total_amount_minor || payload.totalAmountMinor || 0)
      }) as T
  };
  const userAPI: PluginUserAPI = {
    get: <T = unknown>() => ({ id: 7, email: "debugger@example.com" }) as T,
    list: <T = unknown>() =>
      ({
        items: [],
        page: 1,
        page_size: 20,
        total: 0,
        has_more: false
      }) as T
  };
  const hostAPI: PluginHostAPI = {
    enabled: true,
    invoke: <T = unknown>(action: string, params?: Record<string, unknown>) =>
      ({
        action,
        params: params || {}
      }) as T,
    order: orderAPI,
    user: userAPI
  };

  installRuntime({
    host: hostAPI,
    order: orderAPI,
    user: userAPI
  });

  const result = execute(
    "debugger.host.request",
    {
      host_mode: "order.get",
      host_payload: JSON.stringify({ id: 42 })
    },
    buildContext(),
    {},
    buildSandbox({
      currentAction: "debugger.host.request",
      requestedPermissions: ["api.execute", "host.order.read"],
      grantedPermissions: ["api.execute", "host.order.read"]
    })
  );

  assert.equal(result.success, true);
  const blocks = extractBlocks(result);
  const requestBlock = findBlock(blocks, "Host Request");
  const requestValues = getKeyValueMap(requestBlock);
  assert.equal(requestValues.plugin_host_present, true);
  assert.equal(requestValues.plugin_host_enabled, true);
  assert.equal(requestValues.plugin_order_present, true);
  assert.equal(requestValues.mode, "order.get");
  assert.equal(requestValues.resolved_action, "host.order.get");

  const responseBlock = findBlock(blocks, "Host Response");
  assert.deepEqual(parseJSONBlock(responseBlock), {
    id: 42,
    order_no: "ORD-42",
    status: "pending"
  });
});

test("debugger host lab executes Plugin.order.assignTracking helper", () => {
  const orderAPI: PluginOrderAPI = {
    get: <T = unknown>() => ({ id: 42, order_no: "ORD-42", status: "pending" }) as T,
    list: <T = unknown>() =>
      ({
        items: [],
        page: 1,
        page_size: 20,
        total: 0,
        has_more: false
      }) as T,
    assignTracking: <T = unknown>(payload: Record<string, unknown>) =>
      ({
        id: Number(payload.id || 0),
        order_no: "ORD-42",
        tracking_no: String(payload.tracking_no || payload.trackingNo || ""),
        status: "shipped",
        shipped_at: "2026-03-13T12:00:00Z"
      }) as T,
    requestResubmit: <T = unknown>(payload: Record<string, unknown>) =>
      ({
        id: Number(payload.id || 0),
        order_no: "ORD-42",
        status: "need_resubmit",
        form_expires_at: "2026-03-14T12:00:00Z",
        reason: String(payload.reason || "")
      }) as T,
    markPaid: <T = unknown>() =>
      ({
        id: 42,
        order_no: "ORD-42",
        status: "pending"
      }) as T,
    updatePrice: <T = unknown>(payload: Record<string, unknown>) =>
      ({
        id: 42,
        order_no: "ORD-42",
        status: "pending_payment",
        total_amount_minor: Number(payload.total_amount_minor || payload.totalAmountMinor || 0)
      }) as T
  };
  const hostAPI: PluginHostAPI = {
    enabled: true,
    invoke: <T = unknown>(action: string, params?: Record<string, unknown>) =>
      ({
        action,
        params: params || {}
      }) as T,
    order: orderAPI
  };

  installRuntime({
    host: hostAPI,
    order: orderAPI
  });

  const result = execute(
    "debugger.host.request",
    {
      host_mode: "order.assign_tracking",
      host_payload: JSON.stringify({ id: 42, tracking_no: "TRACK-42" })
    },
    buildContext(),
    {},
    buildSandbox({
      currentAction: "debugger.host.request",
      requestedPermissions: ["api.execute", "host.order.assign_tracking"],
      grantedPermissions: ["api.execute", "host.order.assign_tracking"]
    })
  );

  assert.equal(result.success, true);
  const blocks = extractBlocks(result);
  const requestBlock = findBlock(blocks, "Host Request");
  const requestValues = getKeyValueMap(requestBlock);
  assert.equal(requestValues.plugin_order_present, true);
  assert.equal(requestValues.mode, "order.assign_tracking");
  assert.equal(requestValues.resolved_action, "host.order.assign_tracking");

  const responseBlock = findBlock(blocks, "Host Response");
  assert.deepEqual(parseJSONBlock(responseBlock), {
    id: 42,
    order_no: "ORD-42",
    tracking_no: "TRACK-42",
    status: "shipped",
    shipped_at: "2026-03-13T12:00:00Z"
  });
});

test("debugger host lab executes Plugin.order.requestResubmit helper", () => {
  const orderAPI: PluginOrderAPI = {
    get: <T = unknown>() => ({ id: 42, order_no: "ORD-42", status: "pending" }) as T,
    list: <T = unknown>() =>
      ({
        items: [],
        page: 1,
        page_size: 20,
        total: 0,
        has_more: false
      }) as T,
    assignTracking: <T = unknown>(payload: Record<string, unknown>) =>
      ({
        id: Number(payload.id || 0),
        tracking_no: String(payload.tracking_no || payload.trackingNo || ""),
        status: "shipped"
      }) as T,
    requestResubmit: <T = unknown>(payload: Record<string, unknown>) =>
      ({
        id: Number(payload.id || 0),
        order_no: "ORD-42",
        status: "need_resubmit",
        form_expires_at: "2026-03-14T12:00:00Z",
        reason: String(payload.reason || "")
      }) as T,
    markPaid: <T = unknown>() =>
      ({
        id: 42,
        order_no: "ORD-42",
        status: "pending"
      }) as T,
    updatePrice: <T = unknown>(payload: Record<string, unknown>) =>
      ({
        id: 42,
        order_no: "ORD-42",
        status: "pending_payment",
        total_amount_minor: Number(payload.total_amount_minor || payload.totalAmountMinor || 0)
      }) as T
  };
  const hostAPI: PluginHostAPI = {
    enabled: true,
    invoke: <T = unknown>(action: string, params?: Record<string, unknown>) =>
      ({
        action,
        params: params || {}
      }) as T,
    order: orderAPI
  };

  installRuntime({
    host: hostAPI,
    order: orderAPI
  });

  const result = execute(
    "debugger.host.request",
    {
      host_mode: "order.request_resubmit",
      host_payload: JSON.stringify({ id: 42, reason: "Need updated address" })
    },
    buildContext(),
    {},
    buildSandbox({
      currentAction: "debugger.host.request",
      requestedPermissions: ["api.execute", "host.order.request_resubmit"],
      grantedPermissions: ["api.execute", "host.order.request_resubmit"]
    })
  );

  assert.equal(result.success, true);
  const blocks = extractBlocks(result);
  const requestBlock = findBlock(blocks, "Host Request");
  const requestValues = getKeyValueMap(requestBlock);
  assert.equal(requestValues.plugin_order_present, true);
  assert.equal(requestValues.mode, "order.request_resubmit");
  assert.equal(requestValues.resolved_action, "host.order.request_resubmit");

  const responseBlock = findBlock(blocks, "Host Response");
  assert.deepEqual(parseJSONBlock(responseBlock), {
    id: 42,
    order_no: "ORD-42",
    status: "need_resubmit",
    form_expires_at: "2026-03-14T12:00:00Z",
    reason: "Need updated address"
  });
});

test("debugger host lab executes Plugin.order.markPaid helper", () => {
  const orderAPI: PluginOrderAPI = {
    get: <T = unknown>() => ({ id: 42, order_no: "ORD-42", status: "pending_payment" }) as T,
    list: <T = unknown>() =>
      ({
        items: [],
        page: 1,
        page_size: 20,
        total: 0,
        has_more: false
      }) as T,
    assignTracking: <T = unknown>(payload: Record<string, unknown>) =>
      ({ id: Number(payload.id || 0), status: "shipped" }) as T,
    requestResubmit: <T = unknown>(payload: Record<string, unknown>) =>
      ({ id: Number(payload.id || 0), status: "need_resubmit" }) as T,
    markPaid: <T = unknown>() =>
      ({
        id: 42,
        order_no: "ORD-42",
        status: "pending",
        total_amount_minor: 1999
      }) as T,
    updatePrice: <T = unknown>(payload: Record<string, unknown>) =>
      ({
        id: 42,
        total_amount_minor: Number(payload.total_amount_minor || payload.totalAmountMinor || 0)
      }) as T
  };
  const hostAPI: PluginHostAPI = {
    enabled: true,
    invoke: <T = unknown>(action: string, params?: Record<string, unknown>) =>
      ({ action, params: params || {} }) as T,
    order: orderAPI
  };

  installRuntime({ host: hostAPI, order: orderAPI });

  const result = execute(
    "debugger.host.request",
    { host_mode: "order.mark_paid", host_payload: JSON.stringify({ id: 42 }) },
    buildContext(),
    {},
    buildSandbox({
      currentAction: "debugger.host.request",
      requestedPermissions: ["api.execute", "host.order.mark_paid"],
      grantedPermissions: ["api.execute", "host.order.mark_paid"]
    })
  );

  assert.equal(result.success, true);
  const blocks = extractBlocks(result);
  const requestValues = getKeyValueMap(findBlock(blocks, "Host Request"));
  assert.equal(requestValues.mode, "order.mark_paid");
  assert.equal(requestValues.resolved_action, "host.order.mark_paid");
  assert.deepEqual(parseJSONBlock(findBlock(blocks, "Host Response")), {
    id: 42,
    order_no: "ORD-42",
    status: "pending",
    total_amount_minor: 1999
  });
});

test("debugger host lab executes Plugin.order.updatePrice helper", () => {
  const orderAPI: PluginOrderAPI = {
    get: <T = unknown>() => ({ id: 42, order_no: "ORD-42", status: "pending_payment" }) as T,
    list: <T = unknown>() =>
      ({
        items: [],
        page: 1,
        page_size: 20,
        total: 0,
        has_more: false
      }) as T,
    assignTracking: <T = unknown>(payload: Record<string, unknown>) =>
      ({ id: Number(payload.id || 0), status: "shipped" }) as T,
    requestResubmit: <T = unknown>(payload: Record<string, unknown>) =>
      ({ id: Number(payload.id || 0), status: "need_resubmit" }) as T,
    markPaid: <T = unknown>() =>
      ({ id: 42, status: "pending" }) as T,
    updatePrice: <T = unknown>(payload: Record<string, unknown>) =>
      ({
        id: 42,
        order_no: "ORD-42",
        status: "pending_payment",
        total_amount_minor: Number(payload.total_amount_minor || payload.totalAmountMinor || 0)
      }) as T
  };
  const hostAPI: PluginHostAPI = {
    enabled: true,
    invoke: <T = unknown>(action: string, params?: Record<string, unknown>) =>
      ({ action, params: params || {} }) as T,
    order: orderAPI
  };

  installRuntime({ host: hostAPI, order: orderAPI });

  const result = execute(
    "debugger.host.request",
    {
      host_mode: "order.update_price",
      host_payload: JSON.stringify({ id: 42, total_amount_minor: 2999 })
    },
    buildContext(),
    {},
    buildSandbox({
      currentAction: "debugger.host.request",
      requestedPermissions: ["api.execute", "host.order.update_price"],
      grantedPermissions: ["api.execute", "host.order.update_price"]
    })
  );

  assert.equal(result.success, true);
  const blocks = extractBlocks(result);
  const requestValues = getKeyValueMap(findBlock(blocks, "Host Request"));
  assert.equal(requestValues.mode, "order.update_price");
  assert.equal(requestValues.resolved_action, "host.order.update_price");
  assert.deepEqual(parseJSONBlock(findBlock(blocks, "Host Response")), {
    id: 42,
    order_no: "ORD-42",
    status: "pending_payment",
    total_amount_minor: 2999
  });
});

test("debugger host lab executes Plugin.user helper", () => {
  const orderAPI: PluginOrderAPI = {
    get: <T = unknown>() => ({ id: 42 }) as T,
    list: <T = unknown>() =>
      ({
        items: [],
        page: 1,
        page_size: 20,
        total: 0,
        has_more: false
      }) as T,
    assignTracking: <T = unknown>(payload: Record<string, unknown>) =>
      ({
        id: Number(payload.id || 0),
        tracking_no: String(payload.tracking_no || payload.trackingNo || ""),
        status: "shipped"
      }) as T,
    requestResubmit: <T = unknown>(payload: Record<string, unknown>) =>
      ({
        id: Number(payload.id || 0),
        status: "need_resubmit",
        reason: String(payload.reason || "")
      }) as T,
    markPaid: <T = unknown>() =>
      ({
        id: 42,
        status: "pending"
      }) as T,
    updatePrice: <T = unknown>(payload: Record<string, unknown>) =>
      ({
        id: 42,
        total_amount_minor: Number(payload.total_amount_minor || payload.totalAmountMinor || 0)
      }) as T
  };
  const userAPI: PluginUserAPI = {
    get: <T = unknown>(query: number | Record<string, unknown>) =>
      ({
        id: typeof query === "number" ? query : Number((query as Record<string, unknown>).id || 0),
        email: "debugger@example.com",
        role: "user"
      }) as T,
    list: <T = unknown>() =>
      ({
        items: [],
        page: 1,
        page_size: 20,
        total: 0,
        has_more: false
      }) as T
  };
  const hostAPI: PluginHostAPI = {
    enabled: true,
    invoke: <T = unknown>(action: string, params?: Record<string, unknown>) =>
      ({
        action,
        params: params || {}
      }) as T,
    order: orderAPI,
    user: userAPI
  };

  installRuntime({
    host: hostAPI,
    order: orderAPI,
    user: userAPI
  });

  const result = execute(
    "debugger.host.request",
    {
      host_mode: "user.get",
      host_payload: JSON.stringify({ id: 7 })
    },
    buildContext(),
    {},
    buildSandbox({
      currentAction: "debugger.host.request",
      requestedPermissions: ["api.execute", "host.user.read"],
      grantedPermissions: ["api.execute", "host.user.read"]
    })
  );

  assert.equal(result.success, true);
  const blocks = extractBlocks(result);
  const requestBlock = findBlock(blocks, "Host Request");
  const requestValues = getKeyValueMap(requestBlock);
  assert.equal(requestValues.plugin_user_present, true);
  assert.equal(requestValues.mode, "user.get");
  assert.equal(requestValues.resolved_action, "host.user.get");

  const responseBlock = findBlock(blocks, "Host Response");
  assert.deepEqual(parseJSONBlock(responseBlock), {
    id: 7,
    email: "debugger@example.com",
    role: "user"
  });
});

test("debugger host lab executes Plugin.ticket.reply helper", () => {
  const ticketAPI: PluginTicketAPI = {
    get: <T = unknown>(query: number | Record<string, unknown>) =>
      ({
        id: typeof query === "number" ? query : Number((query as Record<string, unknown>).id || 0),
        ticket_no: "TKT-42",
        status: "open"
      }) as T,
    list: <T = unknown>() =>
      ({
        items: [],
        page: 1,
        page_size: 20,
        total: 0,
        has_more: false
      }) as T,
    reply: <T = unknown>(payload: Record<string, unknown>) =>
      ({
        id: 81,
        ticket_id: Number(payload.id || 0),
        ticket_no: "TKT-42",
        status: "processing",
        assigned_to: 7,
        content_type: String(payload.content_type || payload.contentType || "text"),
        created_at: "2026-03-13T12:00:00Z"
      }) as T,
    update: <T = unknown>(payload: Record<string, unknown>) =>
      ({
        id: Number(payload.id || 0),
        ticket_no: "TKT-42",
        status: String(payload.status || "processing"),
        priority: String(payload.priority || "high"),
        assigned_to: payload.clear_assignee ? null : Number(payload.assigned_to || payload.assignedTo || 7)
      }) as T
  };
  const hostAPI: PluginHostAPI = {
    enabled: true,
    invoke: <T = unknown>(action: string, params?: Record<string, unknown>) =>
      ({
        action,
        params: params || {}
      }) as T,
    ticket: ticketAPI
  };

  installRuntime({
    host: hostAPI,
    ticket: ticketAPI
  });

  const result = execute(
    "debugger.host.request",
    {
      host_mode: "ticket.reply",
      host_payload: JSON.stringify({ id: 42, content: "Reply from debugger" })
    },
    buildContext(),
    {},
    buildSandbox({
      currentAction: "debugger.host.request",
      requestedPermissions: ["api.execute", "host.ticket.reply"],
      grantedPermissions: ["api.execute", "host.ticket.reply"]
    })
  );

  assert.equal(result.success, true);
  const blocks = extractBlocks(result);
  const requestBlock = findBlock(blocks, "Host Request");
  const requestValues = getKeyValueMap(requestBlock);
  assert.equal(requestValues.plugin_ticket_present, true);
  assert.equal(requestValues.mode, "ticket.reply");
  assert.equal(requestValues.resolved_action, "host.ticket.reply");

  const responseBlock = findBlock(blocks, "Host Response");
  assert.deepEqual(parseJSONBlock(responseBlock), {
    id: 81,
    ticket_id: 42,
    ticket_no: "TKT-42",
    status: "processing",
    assigned_to: 7,
    content_type: "text",
    created_at: "2026-03-13T12:00:00Z"
  });
});

test("debugger host lab executes Plugin.ticket.update helper", () => {
  const ticketAPI: PluginTicketAPI = {
    get: <T = unknown>(query: number | Record<string, unknown>) =>
      ({
        id: typeof query === "number" ? query : Number((query as Record<string, unknown>).id || 0),
        ticket_no: "TKT-42",
        status: "open"
      }) as T,
    list: <T = unknown>() =>
      ({
        items: [],
        page: 1,
        page_size: 20,
        total: 0,
        has_more: false
      }) as T,
    reply: <T = unknown>() =>
      ({ id: 81, ticket_id: 42, status: "processing" }) as T,
    update: <T = unknown>(payload: Record<string, unknown>) =>
      ({
        id: 42,
        ticket_no: "TKT-42",
        status: String(payload.status || "resolved"),
        priority: String(payload.priority || "high"),
        assigned_to: Number(payload.assigned_to || payload.assignedTo || 7)
      }) as T
  };
  const hostAPI: PluginHostAPI = {
    enabled: true,
    invoke: <T = unknown>(action: string, params?: Record<string, unknown>) =>
      ({ action, params: params || {} }) as T,
    ticket: ticketAPI
  };

  installRuntime({ host: hostAPI, ticket: ticketAPI });

  const result = execute(
    "debugger.host.request",
    {
      host_mode: "ticket.update",
      host_payload: JSON.stringify({ id: 42, status: "resolved", priority: "high", assigned_to: 7 })
    },
    buildContext(),
    {},
    buildSandbox({
      currentAction: "debugger.host.request",
      requestedPermissions: ["api.execute", "host.ticket.update"],
      grantedPermissions: ["api.execute", "host.ticket.update"]
    })
  );

  assert.equal(result.success, true);
  const blocks = extractBlocks(result);
  const requestValues = getKeyValueMap(findBlock(blocks, "Host Request"));
  assert.equal(requestValues.mode, "ticket.update");
  assert.equal(requestValues.resolved_action, "host.ticket.update");
  assert.deepEqual(parseJSONBlock(findBlock(blocks, "Host Response")), {
    id: 42,
    ticket_no: "TKT-42",
    status: "resolved",
    priority: "high",
    assigned_to: 7
  });
});

test("debugger host lab executes Plugin.product helper", () => {
  const productAPI: PluginProductAPI = {
    get: <T = unknown>(query: number | Record<string, unknown>) =>
      ({
        id: typeof query === "number" ? query : Number((query as Record<string, unknown>).id || 0),
        sku: "SKU-DEBUG-1",
        name: "Debugger Product"
      }) as T,
    list: <T = unknown>() =>
      ({
        items: [],
        page: 1,
        page_size: 20,
        total: 0,
        has_more: false
      }) as T
  };
  const hostAPI: PluginHostAPI = {
    enabled: true,
    invoke: <T = unknown>(action: string, params?: Record<string, unknown>) =>
      ({
        action,
        params: params || {}
      }) as T,
    product: productAPI
  };

  installRuntime({
    host: hostAPI,
    product: productAPI
  });

  const result = execute(
    "debugger.host.request",
    {
      host_mode: "product.get",
      host_payload: JSON.stringify({ id: 9 })
    },
    buildContext(),
    {},
    buildSandbox({
      currentAction: "debugger.host.request",
      requestedPermissions: ["api.execute", "host.product.read"],
      grantedPermissions: ["api.execute", "host.product.read"]
    })
  );

  assert.equal(result.success, true);
  const blocks = extractBlocks(result);
  const requestBlock = findBlock(blocks, "Host Request");
  const requestValues = getKeyValueMap(requestBlock);
  assert.equal(requestValues.plugin_product_present, true);
  assert.equal(requestValues.mode, "product.get");
  assert.equal(requestValues.resolved_action, "host.product.get");

  const responseBlock = findBlock(blocks, "Host Response");
  assert.deepEqual(parseJSONBlock(responseBlock), {
    id: 9,
    sku: "SKU-DEBUG-1",
    name: "Debugger Product"
  });
});

test("debugger host lab executes Plugin.announcement helper", () => {
  const announcementAPI: PluginAnnouncementAPI = {
    get: <T = unknown>(query: number | Record<string, unknown>) =>
      ({
        id: typeof query === "number" ? query : Number((query as Record<string, unknown>).id || 0),
        title: "Debugger Announcement",
        category: "marketing"
      }) as T,
    list: <T = unknown>() =>
      ({
        items: [],
        page: 1,
        page_size: 20,
        total: 0,
        has_more: false
      }) as T
  };
  const knowledgeAPI: PluginKnowledgeAPI = {
    get: <T = unknown>(query: number | Record<string, unknown>) =>
      ({
        id: typeof query === "number" ? query : Number((query as Record<string, unknown>).id || 0),
        title: "Debugger Knowledge"
      }) as T,
    list: <T = unknown>() =>
      ({
        items: [],
        page: 1,
        page_size: 20,
        total: 0,
        has_more: false
      }) as T,
    categories: <T = unknown>() =>
      ({
        items: [
          {
            id: 1,
            name: "Debugger Category"
          }
        ],
        total_roots: 1,
        total_entries: 1
      }) as T
  };
  const paymentMethodAPI: PluginPaymentMethodAPI = {
    get: <T = unknown>(query: number | Record<string, unknown>) =>
      ({
        id: typeof query === "number" ? query : Number((query as Record<string, unknown>).id || 0),
        name: "Debugger Payment Method",
        enabled: true
      }) as T,
    list: <T = unknown>() =>
      ({
        items: [],
        page: 1,
        page_size: 20,
        total: 0,
        has_more: false
      }) as T
  };
  const hostAPI: PluginHostAPI = {
    enabled: true,
    invoke: <T = unknown>(action: string, params?: Record<string, unknown>) =>
      ({
        action,
        params: params || {}
    }) as T,
    announcement: announcementAPI,
    knowledge: knowledgeAPI,
    paymentMethod: paymentMethodAPI
  };

  installRuntime({
    host: hostAPI,
    announcement: announcementAPI,
    knowledge: knowledgeAPI,
    paymentMethod: paymentMethodAPI
  });

  const result = execute(
    "debugger.host.request",
    {
      host_mode: "announcement.get",
      host_payload: JSON.stringify({ id: 15 })
    },
    buildContext(),
    {},
    buildSandbox({
      currentAction: "debugger.host.request",
      requestedPermissions: ["api.execute", "host.announcement.read", "host.knowledge.read"],
      grantedPermissions: ["api.execute", "host.announcement.read", "host.knowledge.read"]
    })
  );

  assert.equal(result.success, true);
  const blocks = extractBlocks(result);
  const requestBlock = findBlock(blocks, "Host Request");
  const requestValues = getKeyValueMap(requestBlock);
  assert.equal(requestValues.plugin_announcement_present, true);
  assert.equal(requestValues.plugin_knowledge_present, true);
  assert.equal(requestValues.mode, "announcement.get");
  assert.equal(requestValues.resolved_action, "host.announcement.get");

  const responseBlock = findBlock(blocks, "Host Response");
  assert.deepEqual(parseJSONBlock(responseBlock), {
    id: 15,
    title: "Debugger Announcement",
    category: "marketing"
  });
});

test("debugger host lab executes Plugin.paymentMethod helper", () => {
  const paymentMethodAPI: PluginPaymentMethodAPI = {
    get: <T = unknown>(query: number | Record<string, unknown>) =>
      ({
        id: typeof query === "number" ? query : Number((query as Record<string, unknown>).id || 0),
        name: "Debugger Payment Method",
        enabled: true
      }) as T,
    list: <T = unknown>() =>
      ({
        items: [],
        page: 1,
        page_size: 20,
        total: 0,
        has_more: false
      }) as T
  };
  const hostAPI: PluginHostAPI = {
    enabled: true,
    invoke: <T = unknown>(action: string, params?: Record<string, unknown>) =>
      ({
        action,
        params: params || {}
      }) as T,
    paymentMethod: paymentMethodAPI
  };

  installRuntime({
    host: hostAPI,
    paymentMethod: paymentMethodAPI
  });

  const result = execute(
    "debugger.host.request",
    {
      host_mode: "payment_method.get",
      host_payload: JSON.stringify({ id: 21 })
    },
    buildContext(),
    {},
    buildSandbox({
      currentAction: "debugger.host.request",
      requestedPermissions: ["api.execute", "host.payment_method.read"],
      grantedPermissions: ["api.execute", "host.payment_method.read"]
    })
  );

  assert.equal(result.success, true);
  const blocks = extractBlocks(result);
  const requestBlock = findBlock(blocks, "Host Request");
  const requestValues = getKeyValueMap(requestBlock);
  assert.equal(requestValues.plugin_payment_method_present, true);
  assert.equal(requestValues.mode, "payment_method.get");
  assert.equal(requestValues.resolved_action, "host.payment_method.get");

  const responseBlock = findBlock(blocks, "Host Response");
  assert.deepEqual(parseJSONBlock(responseBlock), {
    id: 21,
    name: "Debugger Payment Method",
    enabled: true
  });
});

test("debugger host lab executes Plugin.inventoryBinding helper", () => {
  const inventoryBindingAPI: PluginInventoryBindingAPI = {
    get: <T = unknown>(query: number | Record<string, unknown>) =>
      ({
        id: typeof query === "number" ? query : Number((query as Record<string, unknown>).id || 0),
        product_id: 9,
        inventory_id: 5
      }) as T,
    list: <T = unknown>() =>
      ({
        items: [],
        page: 1,
        page_size: 20,
        total: 0,
        has_more: false
      }) as T
  };
  const hostAPI: PluginHostAPI = {
    enabled: true,
    invoke: <T = unknown>(action: string, params?: Record<string, unknown>) =>
      ({
        action,
        params: params || {}
      }) as T,
    inventoryBinding: inventoryBindingAPI
  };

  installRuntime({
    host: hostAPI,
    inventoryBinding: inventoryBindingAPI
  });

  const result = execute(
    "debugger.host.request",
    {
      host_mode: "inventory_binding.get",
      host_payload: JSON.stringify({ id: 31 })
    },
    buildContext(),
    {},
    buildSandbox({
      currentAction: "debugger.host.request",
      requestedPermissions: ["api.execute", "host.inventory_binding.read"],
      grantedPermissions: ["api.execute", "host.inventory_binding.read"]
    })
  );

  assert.equal(result.success, true);
  const blocks = extractBlocks(result);
  const requestBlock = findBlock(blocks, "Host Request");
  const requestValues = getKeyValueMap(requestBlock);
  assert.equal(requestValues.plugin_inventory_binding_present, true);
  assert.equal(requestValues.mode, "inventory_binding.get");
  assert.equal(requestValues.resolved_action, "host.inventory_binding.get");

  const responseBlock = findBlock(blocks, "Host Response");
  assert.deepEqual(parseJSONBlock(responseBlock), {
    id: 31,
    product_id: 9,
    inventory_id: 5
  });
});

test("debugger host lab executes Plugin.virtualInventory helper", () => {
  const virtualInventoryAPI: PluginVirtualInventoryAPI = {
    get: <T = unknown>(query: number | Record<string, unknown>) =>
      ({
        id: typeof query === "number" ? query : Number((query as Record<string, unknown>).id || 0),
        name: "Debugger Virtual Inventory",
        type: "script"
      }) as T,
    list: <T = unknown>() =>
      ({
        items: [],
        page: 1,
        page_size: 20,
        total: 0,
        has_more: false
      }) as T
  };
  const hostAPI: PluginHostAPI = {
    enabled: true,
    invoke: <T = unknown>(action: string, params?: Record<string, unknown>) =>
      ({
        action,
        params: params || {}
      }) as T,
    virtualInventory: virtualInventoryAPI
  };

  installRuntime({
    host: hostAPI,
    virtualInventory: virtualInventoryAPI
  });

  const result = execute(
    "debugger.host.request",
    {
      host_mode: "virtual_inventory.get",
      host_payload: JSON.stringify({ id: 33 })
    },
    buildContext(),
    {},
    buildSandbox({
      currentAction: "debugger.host.request",
      requestedPermissions: ["api.execute", "host.virtual_inventory.read"],
      grantedPermissions: ["api.execute", "host.virtual_inventory.read"]
    })
  );

  assert.equal(result.success, true);
  const blocks = extractBlocks(result);
  const requestBlock = findBlock(blocks, "Host Request");
  const requestValues = getKeyValueMap(requestBlock);
  assert.equal(requestValues.plugin_virtual_inventory_present, true);
  assert.equal(requestValues.mode, "virtual_inventory.get");
  assert.equal(requestValues.resolved_action, "host.virtual_inventory.get");

  const responseBlock = findBlock(blocks, "Host Response");
  assert.deepEqual(parseJSONBlock(responseBlock), {
    id: 33,
    name: "Debugger Virtual Inventory",
    type: "script"
  });
});

test("debugger host lab executes Plugin.virtualInventoryBinding helper", () => {
  const virtualInventoryBindingAPI: PluginVirtualInventoryBindingAPI = {
    get: <T = unknown>(query: number | Record<string, unknown>) =>
      ({
        id: typeof query === "number" ? query : Number((query as Record<string, unknown>).id || 0),
        product_id: 9,
        virtual_inventory_id: 12
      }) as T,
    list: <T = unknown>() =>
      ({
        items: [],
        page: 1,
        page_size: 20,
        total: 0,
        has_more: false
      }) as T
  };
  const hostAPI: PluginHostAPI = {
    enabled: true,
    invoke: <T = unknown>(action: string, params?: Record<string, unknown>) =>
      ({
        action,
        params: params || {}
      }) as T,
    virtualInventoryBinding: virtualInventoryBindingAPI
  };

  installRuntime({
    host: hostAPI,
    virtualInventoryBinding: virtualInventoryBindingAPI
  });

  const result = execute(
    "debugger.host.request",
    {
      host_mode: "virtual_inventory_binding.get",
      host_payload: JSON.stringify({ id: 35 })
    },
    buildContext(),
    {},
    buildSandbox({
      currentAction: "debugger.host.request",
      requestedPermissions: ["api.execute", "host.virtual_inventory_binding.read"],
      grantedPermissions: ["api.execute", "host.virtual_inventory_binding.read"]
    })
  );

  assert.equal(result.success, true);
  const blocks = extractBlocks(result);
  const requestBlock = findBlock(blocks, "Host Request");
  const requestValues = getKeyValueMap(requestBlock);
  assert.equal(requestValues.plugin_virtual_inventory_binding_present, true);
  assert.equal(requestValues.mode, "virtual_inventory_binding.get");
  assert.equal(requestValues.resolved_action, "host.virtual_inventory_binding.get");

  const responseBlock = findBlock(blocks, "Host Response");
  assert.deepEqual(parseJSONBlock(responseBlock), {
    id: 35,
    product_id: 9,
    virtual_inventory_id: 12
  });
});

test("debugger host lab executes Plugin.market helper", () => {
  const marketAPI: PluginMarketAPI = {
    source: {
      list: <T = unknown>() =>
        ({
          items: [{ id: "official", name: "Official", channel: "stable" }],
          total: 1
        }) as T,
      get: <T = unknown>(query?: Record<string, unknown>) =>
        ({
          id: String(query?.source_id || "official"),
          name: "Official",
          base_url: "https://market.auralogic.org"
        }) as T
    },
    catalog: {
      list: <T = unknown>() =>
        ({
          items: [{ kind: "plugin_package", name: "js-market", latest_version: "0.1.14" }],
          total: 1
        }) as T
    },
    artifact: {
      get: <T = unknown>() => ({ name: "js-market", kind: "plugin_package" }) as T
    },
    release: {
      get: <T = unknown>() => ({ name: "js-market", version: "0.1.14" }) as T
    },
    install: {
      preview: <T = unknown>() => ({ allowed: true, operations: ["install"] }) as T,
      execute: <T = unknown>() => ({ task_id: "task-1", status: "queued" }) as T,
      rollback: <T = unknown>() => ({ task_id: "task-2", status: "queued" }) as T,
      task: {
        get: <T = unknown>() => ({ task_id: "task-1", status: "completed" }) as T,
        list: <T = unknown>() => ({ items: [{ task_id: "task-1", status: "completed" }] }) as T
      },
      history: {
        list: <T = unknown>() => ({ items: [{ version: "0.1.14", status: "installed" }] }) as T
      }
    }
  };
  const hostAPI: PluginHostAPI = {
    enabled: true,
    invoke: <T = unknown>(action: string, params?: Record<string, unknown>) =>
      ({ action, params: params || {} }) as T,
    market: marketAPI
  };

  installRuntime({
    host: hostAPI,
    market: marketAPI
  });

  const result = execute(
    "debugger.host.request",
    {
      host_mode: "market.source.list",
      host_payload: JSON.stringify({ channel: "stable" })
    },
    buildContext(),
    {},
    buildSandbox({
      currentAction: "debugger.host.request",
      requestedPermissions: ["api.execute", "host.market.source.read"],
      grantedPermissions: ["api.execute", "host.market.source.read"]
    })
  );

  assert.equal(result.success, true);
  const blocks = extractBlocks(result);
  const requestValues = getKeyValueMap(findBlock(blocks, "Host Request"));
  assert.equal(requestValues.plugin_market_present, true);
  assert.equal(requestValues.mode, "market.source.list");
  assert.equal(requestValues.resolved_action, "host.market.source.list");
  assert.deepEqual(parseJSONBlock(findBlock(blocks, "Host Response")), {
    items: [{ id: "official", name: "Official", channel: "stable" }],
    total: 1
  });
});

test("debugger host lab executes template helpers", () => {
  const emailTemplateAPI: PluginEmailTemplateAPI = {
    list: <T = unknown>() => ({ items: [{ template_key: "order_paid" }] }) as T,
    get: <T = unknown>(query: string | Record<string, unknown>) =>
      ({
        key: typeof query === "string" ? query : String(query.key || "order_paid"),
        subject: "Order paid"
      }) as T,
    save: <T = unknown>(payload: PluginEmailTemplateSaveParams) =>
      ({
        key: payload.key || "order_paid",
        content: payload.content,
        updated: true
      }) as T
  };
  const landingPageAPI: PluginLandingPageAPI = {
    get: <T = unknown>() => ({ slug: "home", html_content: "<h1>Home</h1>" }) as T,
    save: <T = unknown>(payload: Record<string, unknown>) =>
      ({ slug: String(payload.slug || "home"), updated: true }) as T,
    reset: <T = unknown>() => ({ slug: "home", reset: true }) as T
  };
  const invoiceTemplateAPI: PluginInvoiceTemplateAPI = {
    get: <T = unknown>() => ({ html: "<div>invoice</div>" }) as T,
    save: <T = unknown>(payload: Record<string, unknown>) => ({ saved: true, payload }) as T,
    reset: <T = unknown>() => ({ reset: true }) as T
  };
  const authBrandingAPI: PluginAuthBrandingAPI = {
    get: <T = unknown>() => ({ theme: "clean" }) as T,
    save: <T = unknown>(payload: Record<string, unknown>) => ({ saved: true, payload }) as T,
    reset: <T = unknown>() => ({ reset: true }) as T
  };
  const pageRulePackAPI: PluginPageRulePackAPI = {
    get: <T = unknown>() => ({ rules: [] }) as T,
    save: <T = unknown>(payload: Record<string, unknown>) => ({ saved: true, payload }) as T,
    reset: <T = unknown>() => ({ reset: true }) as T
  };
  const hostAPI: PluginHostAPI = {
    enabled: true,
    invoke: <T = unknown>(action: string, params?: Record<string, unknown>) =>
      ({ action, params: params || {} }) as T,
    emailTemplate: emailTemplateAPI,
    landingPage: landingPageAPI,
    invoiceTemplate: invoiceTemplateAPI,
    authBranding: authBrandingAPI,
    pageRulePack: pageRulePackAPI
  };

  installRuntime({
    host: hostAPI,
    emailTemplate: emailTemplateAPI,
    landingPage: landingPageAPI,
    invoiceTemplate: invoiceTemplateAPI,
    authBranding: authBrandingAPI,
    pageRulePack: pageRulePackAPI
  });

  const result = execute(
    "debugger.host.request",
    {
      host_mode: "email_template.get",
      host_payload: JSON.stringify({ template_key: "order_paid" })
    },
    buildContext(),
    {},
    buildSandbox({
      currentAction: "debugger.host.request",
      requestedPermissions: ["api.execute", "host.email_template.read"],
      grantedPermissions: ["api.execute", "host.email_template.read"]
    })
  );

  assert.equal(result.success, true);
  const blocks = extractBlocks(result);
  const requestValues = getKeyValueMap(findBlock(blocks, "Host Request"));
  assert.equal(requestValues.plugin_email_template_present, true);
  assert.equal(requestValues.plugin_landing_page_present, true);
  assert.equal(requestValues.plugin_invoice_template_present, true);
  assert.equal(requestValues.plugin_auth_branding_present, true);
  assert.equal(requestValues.plugin_page_rule_pack_present, true);
  assert.equal(requestValues.mode, "email_template.get");
  assert.equal(requestValues.resolved_action, "host.email_template.get");
  assert.deepEqual(parseJSONBlock(findBlock(blocks, "Host Response")), {
    key: "order_paid",
    subject: "Order paid"
  });
});

test("debugger workspace context command echoes expanded argv and page path", () => {
  const { workspace, entries } = createWorkspaceStub();
  const command: PluginWorkspaceCommandContext = {
    name: "debugger/context",
    entry: "debugger.context",
    raw: "debugger/context plugin-debugger waiting_input pex_123",
    argv: ["plugin-debugger", "waiting_input", "pex_123"],
    interactive: false
  };

  const result = workspaceHandlers["debugger.context"].handler(
    command,
    buildContext(),
    {},
    buildSandbox({
      currentAction: "workspace.command.execute"
    }),
    workspace
  );

  assert.equal(result.success, true);
  const values = ((result.data as GenericRecord).values || {}) as GenericRecord;
  assert.deepEqual(values.argv, ["plugin-debugger", "waiting_input", "pex_123"]);
  assert.equal(values.page_path, "/admin/plugin-pages/debugger/orders/ORD-1001?order_id=123&tab=timeline");
  assert.equal(
    entries.some((entry) => entry.message.includes("Expanded argv: plugin-debugger | waiting_input | pex_123")),
    true
  );
});

test("debugger self-test surfaces secret, webhook, and recent debug action state", () => {
  const secretAPI: PluginSecretAPI = {
    enabled: true,
    get: (key) => (key === "debugger_token" ? "token-1" : undefined),
    has: (key) => key === "debugger_token",
    list: () => ["debugger_token"]
  };
  const webhookAPI: PluginWebhookAPI = {
    enabled: true,
    key: "debugger.inspect",
    method: "POST",
    path: "/api/plugins/plugin-debugger/webhooks/debugger.inspect",
    queryString: "source=test",
    queryParams: {
      source: "test"
    },
    headers: {
      "x-debugger": "1"
    },
    contentType: "application/json",
    remoteAddr: "127.0.0.1",
    bodyText: "{\"ok\":true}",
    bodyBase64: "eyJvayI6dHJ1ZX0=",
    header: (name) => (name.toLowerCase() === "x-debugger" ? "1" : undefined),
    query: (name) => (name === "source" ? "test" : undefined),
    text: () => "{\"ok\":true}",
    json: <T = unknown>() => ({ ok: true } as T)
  };

  installRuntime({
    secret: secretAPI,
    webhook: webhookAPI
  });

  execute(
    "debugger.selftest",
    {},
    buildContext(),
    {},
    buildSandbox({
      currentAction: "debugger.selftest",
      declaredStorageAccessMode: "unknown"
    })
  );

  const result = execute(
    "debugger.selftest",
    {},
    buildContext(),
    {},
    buildSandbox({
      currentAction: "debugger.selftest",
      declaredStorageAccessMode: "unknown"
    })
  );

  assert.equal(result.success, true);
  const blocks = extractBlocks(result);
  const secretSnapshot = getKeyValueMap(findBlock(blocks, "Secret Snapshot"));
  assert.equal(secretSnapshot.sample_present, true);
  assert.equal(secretSnapshot.key_count, 1);

  const webhookSnapshot = getKeyValueMap(findBlock(blocks, "Webhook Snapshot"));
  assert.equal(webhookSnapshot.present, true);
  assert.equal(webhookSnapshot.method, "POST");
  assert.equal(webhookSnapshot.path, "/api/plugins/plugin-debugger/webhooks/debugger.inspect");

  const debugActionTable = findBlock(blocks, "Recent Debug Actions");
  const traceRows = Array.isArray((debugActionTable.data as GenericRecord | undefined)?.rows)
    ? (((debugActionTable.data as GenericRecord).rows as unknown[]) || [])
    : [];
  assert.ok(traceRows.length >= 1);
  assert.equal(readActionTraces().length >= 2, true);
});

test("debugger read-only actions do not persist action traces into Plugin.storage", () => {
  installRuntime();

  const result = execute(
    "debugger.echo",
    {
      echo_message: "read-only trace check"
    },
    buildContext(),
    {},
    buildSandbox({
      currentAction: "debugger.echo"
    })
  );

  assert.equal(result.success, true);
  assert.equal(readActionTraces().length, 0);
});

test("debugger self-test surfaces explicit JS runtime global probes", () => {
  installRuntime(
    {},
    {
      Worker: function Worker() {
        return undefined;
      },
      structuredClone: <T>(value: T) => value,
      queueMicrotask: (callback: () => void) => callback(),
      setTimeout: () => 1,
      clearTimeout: () => undefined,
      TextEncoder: class TextEncoderStub {},
      TextDecoder: class TextDecoderStub {},
      atob: (value: string) => value,
      btoa: (value: string) => value
    }
  );

  const result = execute(
    "debugger.selftest",
    {},
    buildContext(),
    {},
    buildSandbox({
      currentAction: "debugger.selftest"
    })
  );

  assert.equal(result.success, true);
  const blocks = extractBlocks(result);
  const runtimeProbe = getKeyValueMap(findBlock(blocks, "Runtime Probe"));
  assert.equal(runtimeProbe.hasWorkerGlobal, true);
  assert.equal(runtimeProbe.hasStructuredCloneGlobal, true);
  assert.equal(runtimeProbe.hasQueueMicrotaskGlobal, true);
  assert.equal(runtimeProbe.hasSetTimeoutGlobal, true);
  assert.equal(runtimeProbe.hasClearTimeoutGlobal, true);
  assert.equal(runtimeProbe.hasTextEncoderGlobal, true);
  assert.equal(runtimeProbe.hasTextDecoderGlobal, true);
  assert.equal(runtimeProbe.hasAtobGlobal, true);
  assert.equal(runtimeProbe.hasBtoaGlobal, true);
  assert.equal(
    runtimeProbe.runtimeGlobalKeys,
    "atob, btoa, clearTimeout, queueMicrotask, setTimeout, structuredClone, TextDecoder, TextEncoder, Worker"
  );
  assert.equal(runtimeProbe.pluginGlobalKeys, "storage");
});

test("debugger worker roundtrip action verifies request, postMessage, and terminate flow", async () => {
  class LinkedWorkerStub {
    id: string;
    scriptPath: string;
    terminated: boolean;
    callCount: number;
    onmessage?: (event: Record<string, unknown>) => void;
    onerror?: (event: Record<string, unknown>) => void;

    constructor(scriptPath: string) {
      this.id = "worker-1";
      this.scriptPath = scriptPath;
      this.terminated = false;
      this.callCount = 0;
    }

    async request(payload: Record<string, unknown>) {
      this.callCount += 1;
      return {
        mode: "request",
        doubled: Number(payload.value || 0) * 2,
        calls: this.callCount,
        worker_id: this.id,
        script_path: this.scriptPath
      };
    }

    postMessage(payload: Record<string, unknown>) {
      this.callCount += 1;
      const dispatch = () => {
        this.onmessage?.({
          type: "message",
          worker_id: this.id,
          script_path: this.scriptPath,
          data: {
            mode: "postMessage",
            value: Number(payload.value || 0) + 1,
            calls: this.callCount
          }
        });
      };
      if (typeof queueMicrotask === "function") {
        queueMicrotask(dispatch);
        return;
      }
      setTimeout(dispatch, 0);
    }

    terminate() {
      this.terminated = true;
    }
  }

  installRuntime(
    {},
    {
      Worker: LinkedWorkerStub,
      queueMicrotask: (callback: () => void) => callback(),
      setTimeout: (callback: () => void) => {
        callback();
        return 1;
      }
    }
  );

  const result = await execute(
    "debugger.worker.roundtrip",
    {},
    buildContext(),
    {},
    buildSandbox({
      currentAction: "debugger.worker.roundtrip"
    })
  );

  assert.equal(result.success, true);
  const blocks = extractBlocks(result);
  const runtimeBlock = getKeyValueMap(findBlock(blocks, "Worker Runtime"));
  assert.equal(runtimeBlock.worker_global_present, true);
  assert.equal(runtimeBlock.worker_script, "./assets/worker-roundtrip.js");
  assert.equal(runtimeBlock.request_supported, true);
  assert.equal(runtimeBlock.post_message_supported, true);
  assert.equal(runtimeBlock.terminate_supported, true);
  assert.equal(runtimeBlock.worker_id, "worker-1");
  assert.equal(runtimeBlock.worker_script_path, "./assets/worker-roundtrip.js");
  assert.equal(runtimeBlock.first_doubled, 20);
  assert.equal(runtimeBlock.second_doubled, 42);
  assert.equal(runtimeBlock.request_calls, 2);
  assert.equal(runtimeBlock.message_value, 7);
  assert.equal(runtimeBlock.message_calls, 3);
  assert.equal(runtimeBlock.terminated_after, true);

  assert.deepEqual(parseJSONBlock(findBlock(blocks, "Worker Request Responses")), {
    first: {
      mode: "request",
      doubled: 20,
      calls: 1,
      worker_id: "worker-1",
      script_path: "./assets/worker-roundtrip.js"
    },
    second: {
      mode: "request",
      doubled: 42,
      calls: 2,
      worker_id: "worker-1",
      script_path: "./assets/worker-roundtrip.js"
    }
  });
  assert.deepEqual(parseJSONBlock(findBlock(blocks, "Worker Message Event")), {
    type: "message",
    worker_id: "worker-1",
    script_path: "./assets/worker-roundtrip.js",
    data: {
      mode: "postMessage",
      value: 7,
      calls: 3
    }
  });
});

test("debugger workspace report command writes runtime summary and records a workspace trace", () => {
  installRuntime(
    {},
    {
      Worker: function Worker() {
        return undefined;
      },
      structuredClone: <T>(value: T) => value,
      queueMicrotask: (callback: () => void) => callback(),
      setTimeout: () => 1,
      clearTimeout: () => undefined,
      TextEncoder: class TextEncoderStub {},
      TextDecoder: class TextDecoderStub {},
      atob: (value: string) => value,
      btoa: (value: string) => value
    }
  );
  const { workspace, entries } = createWorkspaceStub();
  const command: PluginWorkspaceCommandContext = {
    name: "debugger/report",
    entry: "debugger.report",
    raw: "debugger/report",
    argv: [],
    interactive: false
  };

  const result = workspaceHandlers["debugger.report"].handler(
    command,
    buildContext(),
    {},
    buildSandbox({
      currentAction: "workspace.command.execute",
      declaredStorageAccessMode: "unknown"
    }),
    workspace
  );

  assert.equal(result.success, true);
  assert.equal(entries.some((entry) => entry.message.includes("Plugin Debugger runtime report")), true);
  assert.equal(entries.some((entry) => entry.message.includes("hooks=")), true);
  assert.equal(
    entries.some((entry) => entry.message.includes("runtime_globals=atob,btoa,clearTimeout,queueMicrotask,setTimeout,structuredClone,TextDecoder,TextEncoder,Worker")),
    true
  );
  assert.equal(entries.some((entry) => entry.message.includes("plugin_keys=storage")), true);
  const traces = readActionTraces();
  assert.equal(traces.length, 1);
  assert.equal(traces[0]?.action, "workspace:debugger.report");
});

test("generated debugger hook catalog stays in sync with backend registry", () => {
  assert.deepEqual([...OFFICIAL_PLUGIN_HOOKS], readCatalogSnapshot().plugin_hooks);
});

test("generated debugger slot catalog stays in sync with host frontend slots", () => {
  assert.deepEqual([...OFFICIAL_FRONTEND_SLOTS], readCatalogSnapshot().frontend_slots);
});

test("debugger manifest requests and default-grants the full official permission catalog", () => {
  const manifestPath = path.resolve(__dirname, "../../manifest.json");
  const manifest = JSON.parse(readFileSync(manifestPath, "utf8")) as {
    permissions?: Array<{ key?: unknown; reason?: unknown }>;
    secret_schema?: {
      fields?: Array<{ key?: unknown }>;
    };
    webhooks?: Array<{ key?: unknown }>;
    workspace?: {
      enabled?: unknown;
      title?: unknown;
      commands?: Array<{ entry?: unknown }>;
    };
    capabilities?: {
      hooks?: unknown;
      allowed_frontend_slots?: unknown;
      requested_permissions?: unknown;
      granted_permissions?: unknown;
    };
  };
  const capabilities = manifest.capabilities || {};
  const permissionEntries = Array.isArray(manifest.permissions) ? manifest.permissions : [];

  const hooks = Array.isArray(capabilities.hooks)
    ? capabilities.hooks.map((item) => String(item)).sort()
    : [];
  const slots = Array.isArray(capabilities.allowed_frontend_slots)
    ? capabilities.allowed_frontend_slots
        .map((item) => String(item))
        .sort()
    : [];
  const requested = Array.isArray(capabilities.requested_permissions)
    ? capabilities.requested_permissions.map((item) => String(item))
    : [];
  const granted = Array.isArray(capabilities.granted_permissions)
    ? capabilities.granted_permissions.map((item) => String(item))
    : [];
  const catalogValidation = validatePluginManifestCatalog(manifest);
  const schemaValidation = validatePluginManifestSchema(manifest);
  const compatibility = inspectPluginManifestCompatibility(manifest);

  const snapshot = readCatalogSnapshot();
  assert.deepEqual(hooks, snapshot.plugin_hooks);
  assert.deepEqual(slots, snapshot.frontend_slots);
  assert.equal(catalogValidation.valid, true);
  assert.equal(schemaValidation.valid, true);
  assert.equal(compatibility.compatible, true);
  assert.equal(compatibility.reason_code, "compatible");
  assert.equal(Array.isArray(manifest.secret_schema?.fields), true);
  assert.equal(manifest.secret_schema?.fields?.some((field) => String(field.key || "") === "debugger_token"), true);
  assert.equal(Array.isArray(manifest.webhooks), true);
  assert.equal(manifest.webhooks?.some((item) => String(item.key || "") === "debugger.inspect"), true);
  assert.equal(Boolean(manifest.workspace), false);

  const permissionCatalog = snapshot.plugin_permission_keys;
  assert.deepEqual(requested, permissionCatalog);
  assert.deepEqual(granted, permissionCatalog);

  for (const permission of permissionCatalog) {
    assert.ok(requested.includes(permission), `expected manifest requested_permissions to include ${permission}`);
    assert.ok(granted.includes(permission), `expected manifest granted_permissions to include ${permission}`);
    const entry = permissionEntries.find((item) => String(item.key || "") === permission);
    assert.ok(entry, `expected manifest permissions to include ${permission}`);
    assert.ok(String(entry?.reason || "").trim().length > 0, `expected manifest reason for ${permission}`);
  }
});
