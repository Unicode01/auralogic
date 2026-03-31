import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { existsSync, readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";

import {
  buildPluginActionButtonExtension,
  buildPluginExecuteTemplatePlaceholder,
  buildPluginExecuteTemplateValues
} from "./frontend";
import {
  getPluginAuthBranding,
  getPluginAnnouncement,
  definePlugin,
  defineWorkspaceCommand,
  defineWorkspaceCommands,
  getPluginEmailTemplate,
  getPluginHost,
  getPluginInventory,
  getPluginInventoryBinding,
  getPluginInvoiceTemplate,
  getPluginKnowledge,
  getPluginLandingPage,
  getPluginMarket,
  getPluginOrder,
  getPluginPageRulePack,
  getPluginPaymentMethod,
  isOfficialFrontendSlot,
  isOfficialPluginPermissionKey,
  isOfficialHostPermissionKey,
  isOfficialPluginHook,
  inspectPluginManifestCompatibility,
  listOfficialFrontendSlots,
  listOfficialPluginPermissionKeys,
  listOfficialHostPermissionKeys,
  listOfficialPluginHooks,
  normalizeHookName,
  normalizePluginPermissionKey,
  OFFICIAL_FRONTEND_SLOTS,
  OFFICIAL_PLUGIN_PERMISSION_KEYS,
  OFFICIAL_HOST_PERMISSION_KEYS,
  OFFICIAL_PLUGIN_HOOK_GROUPS,
  OFFICIAL_PLUGIN_HOOKS,
  validatePluginManifestCatalog,
  validatePluginManifestSchema,
  getPluginProduct,
  getPluginPromo,
  getPluginSecret,
  getPluginSerial,
  getPluginTicket,
  getPluginUser,
  getPluginVirtualInventory,
  getPluginVirtualInventoryBinding,
  getPluginWebhook,
  resolvePluginPageContext,
  safeParseStringMap,
  type PluginAnnouncementAPI,
  type PluginAuthBrandingAPI,
  type PluginEmailTemplateAPI,
  type PluginEmailTemplateSaveParams,
  type PluginExecutionContext,
  type PluginHostAPI,
  type PluginHostListResult,
  type PluginInventoryAPI,
  type PluginInventoryBindingAPI,
  type PluginInvoiceTemplateAPI,
  type PluginKnowledgeAPI,
  type PluginLandingPageAPI,
  type PluginMarketAPI,
  type PluginOrderAPI,
  type PluginPageRulePackAPI,
  type PluginPaymentMethodAPI,
  type PluginProductAPI,
  type PluginPromoAPI,
  type PluginSecretAPI,
  type PluginSerialAPI,
  type PluginTicketAPI,
  type PluginUserAPI,
  type PluginVirtualInventoryAPI,
  type PluginVirtualInventoryBindingAPI,
  type PluginWebhookAPI,
  type PluginWorkspaceAPI,
  type PluginWorkspaceCommandContext
} from "./index";

function readRepoText(relativePath: string): string {
  return readFileSync(path.resolve(__dirname, "../../../", relativePath), "utf8");
}

function repoPathExists(relativePath: string): boolean {
  return existsSync(path.resolve(__dirname, "../../../", relativePath));
}

function listExistingSampleManifestPaths(): string[] {
  return [
    "plugins/js-worker-debugger/manifest.json",
    "plugins/js-worker-template/manifest.json",
    "plugins/js_market/manifest.json"
  ].filter(repoPathExists);
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
  const scriptPath = path.resolve(__dirname, "../scripts/catalog-source.mjs");
  const output = execFileSync(process.execPath, [scriptPath, "--json"], {
    encoding: "utf8",
    cwd: path.resolve(__dirname, "..")
  });
  cachedCatalogSnapshot = JSON.parse(output) as CatalogSnapshot;
  return cachedCatalogSnapshot;
}

function createOrderAPI(id: number, page = 1, pageSize = 20, total = 0): PluginOrderAPI {
  return {
    get: <T = unknown>() => ({ id } as T),
    list: <T = PluginHostListResult>() =>
      ({
        items: [],
        page,
        page_size: pageSize,
        total,
        has_more: false
      }) as T,
    assignTracking: <T = unknown>(payload: Record<string, unknown>) =>
      ({
        id,
        tracking_no: payload.tracking_no ?? payload.trackingNo ?? ""
      }) as T,
    requestResubmit: <T = unknown>() =>
      ({
        id,
        status: "need_resubmit"
      }) as T,
    markPaid: <T = unknown>() =>
      ({
        id,
        status: "pending"
      }) as T,
    updatePrice: <T = unknown>(payload: Record<string, unknown>) =>
      ({
        id,
        total_amount_minor: payload.total_amount_minor ?? payload.totalAmountMinor ?? 0
      }) as T
  };
}

function createUserAPI(id: number, page = 1, pageSize = 20, total = 0): PluginUserAPI {
  return {
    get: <T = unknown>() => ({ id } as T),
    list: <T = PluginHostListResult>() =>
      ({
        items: [],
        page,
        page_size: pageSize,
        total,
        has_more: false
      }) as T
  };
}

function createProductAPI(id: number, page = 1, pageSize = 20, total = 0): PluginProductAPI {
  return {
    get: <T = unknown>() => ({ id } as T),
    list: <T = PluginHostListResult>() =>
      ({
        items: [],
        page,
        page_size: pageSize,
        total,
        has_more: false
      }) as T
  };
}

function createInventoryAPI(id: number, page = 1, pageSize = 20, total = 0): PluginInventoryAPI {
  return {
    get: <T = unknown>() => ({ id } as T),
    list: <T = PluginHostListResult>() =>
      ({
        items: [],
        page,
        page_size: pageSize,
        total,
        has_more: false
      }) as T
  };
}

function createInventoryBindingAPI(id: number, page = 1, pageSize = 20, total = 0): PluginInventoryBindingAPI {
  return {
    get: <T = unknown>() => ({ id } as T),
    list: <T = PluginHostListResult>() =>
      ({
        items: [],
        page,
        page_size: pageSize,
        total,
        has_more: false
      }) as T
  };
}

function createPromoAPI(id: number, page = 1, pageSize = 20, total = 0): PluginPromoAPI {
  return {
    get: <T = unknown>() => ({ id } as T),
    list: <T = PluginHostListResult>() =>
      ({
        items: [],
        page,
        page_size: pageSize,
        total,
        has_more: false
      }) as T
  };
}

function createTicketAPI(id: number, page = 1, pageSize = 20, total = 0): PluginTicketAPI {
  return {
    get: <T = unknown>() => ({ id } as T),
    list: <T = PluginHostListResult>() =>
      ({
        items: [],
        page,
        page_size: pageSize,
        total,
        has_more: false
      }) as T,
    reply: <T = unknown>(payload: Record<string, unknown>) =>
      ({
        id,
        content_type: payload.content_type ?? payload.contentType ?? "text"
      }) as T,
    update: <T = unknown>(payload: Record<string, unknown>) =>
      ({
        id,
        status: payload.status ?? "open",
        priority: payload.priority ?? "normal"
      }) as T
  };
}

function createSerialAPI(id: number, page = 1, pageSize = 20, total = 0): PluginSerialAPI {
  return {
    get: <T = unknown>() => ({ id } as T),
    list: <T = PluginHostListResult>() =>
      ({
        items: [],
        page,
        page_size: pageSize,
        total,
        has_more: false
      }) as T
  };
}

function createAnnouncementAPI(id: number, page = 1, pageSize = 20, total = 0): PluginAnnouncementAPI {
  return {
    get: <T = unknown>() => ({ id } as T),
    list: <T = PluginHostListResult>() =>
      ({
        items: [],
        page,
        page_size: pageSize,
        total,
        has_more: false
      }) as T
  };
}

function createKnowledgeAPI(id: number, page = 1, pageSize = 20, total = 0): PluginKnowledgeAPI {
  return {
    get: <T = unknown>() => ({ id } as T),
    list: <T = PluginHostListResult>() =>
      ({
        items: [],
        page,
        page_size: pageSize,
        total,
        has_more: false
      }) as T,
    categories: <T = unknown>() =>
      ({
        items: [{ id }],
        total_roots: 1,
        total_entries: 1
      }) as T
  };
}

function createPaymentMethodAPI(id: number, page = 1, pageSize = 20, total = 0): PluginPaymentMethodAPI {
  return {
    get: <T = unknown>() => ({ id } as T),
    list: <T = PluginHostListResult>() =>
      ({
        items: [],
        page,
        page_size: pageSize,
        total,
        has_more: false
      }) as T
  };
}

function createVirtualInventoryAPI(id: number, page = 1, pageSize = 20, total = 0): PluginVirtualInventoryAPI {
  return {
    get: <T = unknown>() => ({ id } as T),
    list: <T = PluginHostListResult>() =>
      ({
        items: [],
        page,
        page_size: pageSize,
        total,
        has_more: false
      }) as T
  };
}

function createVirtualInventoryBindingAPI(
  id: number,
  page = 1,
  pageSize = 20,
  total = 0
): PluginVirtualInventoryBindingAPI {
  return {
    get: <T = unknown>() => ({ id } as T),
    list: <T = PluginHostListResult>() =>
      ({
        items: [],
        page,
        page_size: pageSize,
        total,
        has_more: false
      }) as T
  };
}

function createMarketAPI(label: string): PluginMarketAPI {
  return {
    source: {
      list: <T = unknown>() =>
        ({
          items: [{ source_id: `${label}-source` }]
        }) as T,
      get: <T = unknown>() =>
        ({
          source_id: `${label}-source`,
          name: `${label}-market`
        }) as T
    },
    catalog: {
      list: <T = unknown>() =>
        ({
          items: [{ name: `${label}-catalog` }]
        }) as T
    },
    artifact: {
      get: <T = unknown>() =>
        ({
          name: `${label}-artifact`
        }) as T
    },
    release: {
      get: <T = unknown>() =>
        ({
          version: `${label}-release`
        }) as T
    },
    install: {
      preview: <T = unknown>() =>
        ({
          ok: true,
          label
        }) as T,
      execute: <T = unknown>() =>
        ({
          status: "activated",
          label
        }) as T,
      task: {
        get: <T = unknown>() =>
          ({
            task_id: `${label}-task`,
            status: "succeeded"
          }) as T,
        list: <T = unknown>() =>
          ({
            items: [{ task_id: `${label}-task` }]
          }) as T
      },
      history: {
        list: <T = unknown>() =>
          ({
            items: [{ version: `${label}-history` }]
          }) as T
      },
      rollback: <T = unknown>() =>
        ({
          status: "rolled_back",
          label
        }) as T
    }
  };
}

function createEmailTemplateAPI(label: string): PluginEmailTemplateAPI {
  return {
    list: <T = unknown>() =>
      ({
        items: [{ key: `${label}-order_paid` }]
      }) as T,
    get: <T = unknown>() =>
      ({
        key: `${label}-order_paid`,
        digest: `${label}-digest`
      }) as T,
    save: <T = unknown>(payload: PluginEmailTemplateSaveParams) =>
      ({
        key: payload.key ?? `${label}-order_paid`,
        saved: true
      }) as T
  };
}

function createLandingPageAPI(label: string): PluginLandingPageAPI {
  return {
    get: <T = unknown>() =>
      ({
        page_key: `${label}-home`
      }) as T,
    save: <T = unknown>() =>
      ({
        saved: true,
        page_key: `${label}-home`
      }) as T,
    reset: <T = unknown>() =>
      ({
        reset: true,
        page_key: `${label}-home`
      }) as T
  };
}

function createInvoiceTemplateAPI(label: string): PluginInvoiceTemplateAPI {
  return {
    get: <T = unknown>() =>
      ({
        target_key: "invoice",
        digest: `${label}-invoice-digest`
      }) as T,
    save: <T = unknown>() =>
      ({
        target_key: "invoice",
        saved: true
      }) as T,
    reset: <T = unknown>() =>
      ({
        target_key: "invoice",
        reset: true
      }) as T
  };
}

function createAuthBrandingAPI(label: string): PluginAuthBrandingAPI {
  return {
    get: <T = unknown>() =>
      ({
        target_key: "auth_branding",
        digest: `${label}-auth-digest`
      }) as T,
    save: <T = unknown>() =>
      ({
        target_key: "auth_branding",
        saved: true
      }) as T,
    reset: <T = unknown>() =>
      ({
        target_key: "auth_branding",
        reset: true
      }) as T
  };
}

function createPageRulePackAPI(label: string): PluginPageRulePackAPI {
  return {
    get: <T = unknown>() =>
      ({
        target_key: "page_rules",
        digest: `${label}-page-rules-digest`
      }) as T,
    save: <T = unknown>() =>
      ({
        target_key: "page_rules",
        saved: true
      }) as T,
    reset: <T = unknown>() =>
      ({
        target_key: "page_rules",
        reset: true
      }) as T
  };
}

test("buildPluginExecuteTemplateValues includes plugin page path/query/route placeholders", () => {
  const values = buildPluginExecuteTemplateValues({
    pluginID: 7,
    pluginName: "logistics-debugger",
    area: "admin",
    path: "/admin/plugin-pages/logistics/orders/ORD-1001",
    query_params: {
      order_id: "123",
      tab: "timeline"
    },
    route_params: {
      orderNo: "ORD-1001"
    },
    execute_api_url: "/api/admin/plugins/7/execute",
    execute_stream_url: "/api/admin/plugins/7/execute/stream"
  });

  assert.equal(
    values["plugin.full_path"],
    "/admin/plugin-pages/logistics/orders/ORD-1001?order_id=123&tab=timeline"
  );
  assert.equal(values["plugin.query_string"], "order_id=123&tab=timeline");
  assert.equal(values["plugin.query_params_json"], "{\"order_id\":\"123\",\"tab\":\"timeline\"}");
  assert.equal(values["plugin.route_params_json"], "{\"orderNo\":\"ORD-1001\"}");
  assert.equal(values["plugin.query.order_id"], "123");
  assert.equal(values["plugin.route.orderNo"], "ORD-1001");
  assert.equal(
    buildPluginExecuteTemplatePlaceholder("plugin.query.order_id"),
    "{{plugin.query.order_id}}"
  );
  assert.equal(
    buildPluginExecuteTemplatePlaceholder("plugin.route.orderNo"),
    "{{plugin.route.orderNo}}"
  );

  const jsonOnlyValues = buildPluginExecuteTemplateValues({
    path: "/plugin-pages/logistics/orders/ORD-1001",
    query_params_json: "{\"order_id\":\"123\"}",
    route_params_json: "{\"orderNo\":\"ORD-1001\"}"
  });
  assert.equal(
    jsonOnlyValues["plugin.full_path"],
    "/plugin-pages/logistics/orders/ORD-1001?order_id=123"
  );
  assert.equal(jsonOnlyValues["plugin.query.order_id"], "123");
  assert.equal(jsonOnlyValues["plugin.route.orderNo"], "ORD-1001");
});

test("resolvePluginPageContext parses structured query and route params from metadata", () => {
  const context: PluginExecutionContext = {
    metadata: {
      bootstrap_area: "admin",
      plugin_page_path: "/admin/plugin-pages/logistics/orders/ORD-1001",
      plugin_page_query_params: "{\"order_id\":\"123\",\"tab\":\"timeline\"}",
      plugin_page_route_params: "{\"orderNo\":\"ORD-1001\"}"
    }
  };

  const pageContext = resolvePluginPageContext(context);
  assert.equal(pageContext.area, "admin");
  assert.equal(pageContext.path, "/admin/plugin-pages/logistics/orders/ORD-1001");
  assert.equal(
    pageContext.full_path,
    "/admin/plugin-pages/logistics/orders/ORD-1001?order_id=123&tab=timeline"
  );
  assert.equal(pageContext.query_string, "order_id=123&tab=timeline");
  assert.deepEqual(pageContext.query_params, {
    order_id: "123",
    tab: "timeline"
  });
  assert.deepEqual(pageContext.route_params, {
    orderNo: "ORD-1001"
  });
});

test("official hook catalog exports stay in sync with backend registry", () => {
  const backendHooks = readCatalogSnapshot().plugin_hooks;
  assert.deepEqual([...OFFICIAL_PLUGIN_HOOKS], backendHooks);
  assert.deepEqual(listOfficialPluginHooks(), backendHooks);
  assert.deepEqual(
    listOfficialPluginHooks("commerce"),
    backendHooks.filter((hook) =>
      ["order.", "cart.", "payment.", "promo.", "serial."].some((prefix) => hook.startsWith(prefix))
    )
  );
  assert.equal(isOfficialPluginHook("order.create.before"), true);
  assert.equal(isOfficialPluginHook("plugin.market.rollback.before"), false);
  assert.equal(normalizeHookName(" Order.Create.Before "), "order.create.before");
});

test("official slot and host permission catalogs stay in sync with host source", () => {
  const snapshot = readCatalogSnapshot();
  const frontendSlots = snapshot.frontend_slots;
  const pluginPermissions = snapshot.plugin_permission_keys;
  const hostPermissions = snapshot.host_permission_keys;

  assert.deepEqual([...OFFICIAL_FRONTEND_SLOTS], frontendSlots);
  assert.deepEqual(listOfficialFrontendSlots(), frontendSlots);
  assert.equal(isOfficialFrontendSlot("admin.orders.top"), true);
  assert.equal(isOfficialFrontendSlot("admin.orders.unknown"), false);

  assert.deepEqual([...OFFICIAL_PLUGIN_PERMISSION_KEYS], pluginPermissions);
  assert.deepEqual(listOfficialPluginPermissionKeys(), pluginPermissions);
  assert.equal(isOfficialPluginPermissionKey("hook.execute"), true);
  assert.equal(isOfficialPluginPermissionKey("hook.execute.unknown"), false);

  assert.deepEqual([...OFFICIAL_HOST_PERMISSION_KEYS], hostPermissions);
  assert.deepEqual(listOfficialHostPermissionKeys(), hostPermissions);
  assert.equal(isOfficialHostPermissionKey("host.market.catalog.read"), true);
  assert.equal(isOfficialHostPermissionKey("host.market.catalog.write"), false);
  assert.equal(
    normalizePluginPermissionKey(" HOST.MARKET.CATALOG.READ "),
    "host.market.catalog.read"
  );
});

test("official hook groups cover every exported hook exactly once", () => {
  const grouped = Object.values(OFFICIAL_PLUGIN_HOOK_GROUPS).flatMap((items) => items);
  const unique = Array.from(new Set(grouped));
  assert.deepEqual(unique.sort(), [...OFFICIAL_PLUGIN_HOOKS]);
});

test("validatePluginManifestCatalog accepts current sample manifest", () => {
  const [manifestPath] = listExistingSampleManifestPaths();
  assert.ok(manifestPath, "expected at least one sample manifest in the current branch");

  const manifest = JSON.parse(readRepoText(manifestPath)) as Record<string, unknown>;
  const validation = validatePluginManifestCatalog(manifest);

  assert.equal(validation.valid, true);
  assert.equal(validation.invalid_hooks.length, 0);
  assert.equal(validation.invalid_allowed_frontend_slots.length, 0);
  assert.equal(validation.invalid_requested_permissions.length, 0);
  assert.equal(validation.invalid_granted_permissions.length, 0);
  assert.equal(validation.invalid_declared_permissions.length, 0);
});

test("validatePluginManifestCatalog reports invalid manifest catalog entries", () => {
  const validation = validatePluginManifestCatalog({
    permissions: [
      { key: "host.market.catalog.read" },
      { key: "host.market.catalog.write" }
    ],
    capabilities: {
      hooks: ["order.create.before", "plugin.market.rollback.before"],
      disabled_hooks: ["ticket.create.after", "ticket.missing.after"],
      allowed_frontend_slots: ["admin.orders.top", "admin.orders.unknown"],
      requested_permissions: [
        "host.market.catalog.read",
        "host.market.catalog.write"
      ],
      granted_permissions: [
        "host.market.catalog.read",
        "host.market.catalog.write"
      ]
    }
  });

  assert.equal(validation.valid, false);
  assert.deepEqual(validation.invalid_hooks, ["plugin.market.rollback.before"]);
  assert.deepEqual(validation.invalid_disabled_hooks, ["ticket.missing.after"]);
  assert.deepEqual(validation.invalid_allowed_frontend_slots, ["admin.orders.unknown"]);
  assert.deepEqual(validation.invalid_requested_permissions, ["host.market.catalog.write"]);
  assert.deepEqual(validation.invalid_granted_permissions, ["host.market.catalog.write"]);
  assert.deepEqual(validation.invalid_declared_permissions, ["host.market.catalog.write"]);
  assert.deepEqual(validation.requested_permissions_missing_declaration, []);
  assert.deepEqual(validation.granted_permissions_missing_declaration, []);
  assert.deepEqual(validation.declared_permissions_missing_request, []);
});

test("validatePluginManifestCatalog reports permission declaration mismatches", () => {
  const validation = validatePluginManifestCatalog({
    permissions: [{ key: "host.order.read" }],
    capabilities: {
      requested_permissions: ["host.order.read", "host.user.read"],
      granted_permissions: ["host.order.read", "host.user.read"]
    }
  });

  assert.equal(validation.valid, false);
  assert.deepEqual(validation.requested_permissions_missing_declaration, ["host.user.read"]);
  assert.deepEqual(validation.granted_permissions_missing_declaration, ["host.user.read"]);
  assert.deepEqual(validation.declared_permissions_missing_request, []);
});

test("inspectPluginManifestCompatibility mirrors current host compatibility rules", () => {
  const compatible = inspectPluginManifestCompatibility({
    runtime: "js_worker",
    manifest_version: "1.0.0",
    protocol_version: "1.0.0",
    min_host_protocol_version: "1.0.0",
    max_host_protocol_version: "1.0.0"
  });

  assert.equal(compatible.compatible, true);
  assert.equal(compatible.reason_code, "compatible");
  assert.equal(compatible.host_manifest_version, "1.0.0");
  assert.equal(compatible.host_protocol_version, "1.0.0");

  const legacy = inspectPluginManifestCompatibility({
    runtime: "js_worker"
  });
  assert.equal(legacy.compatible, true);
  assert.equal(legacy.legacy_defaults_applied, true);
  assert.equal(legacy.reason_code, "compatible_assumed_legacy");

  const incompatible = inspectPluginManifestCompatibility({
    runtime: "js_worker",
    manifest_version: "1.0.0",
    protocol_version: "2.0.0"
  });
  assert.equal(incompatible.compatible, false);
  assert.equal(incompatible.reason_code, "protocol_version_unsupported");
});

test("validatePluginManifestSchema accepts current sample manifests", () => {
  const manifests = listExistingSampleManifestPaths();
  assert.ok(manifests.length > 0, "expected at least one sample manifest in the current branch");

  manifests.forEach((manifestPath) => {
    const manifest = JSON.parse(readRepoText(manifestPath)) as Record<string, unknown>;
    const validation = validatePluginManifestSchema(manifest);
    assert.equal(
      validation.valid,
      true,
      `expected ${manifestPath} to be schema-valid, issues: ${JSON.stringify(validation.issues)}`
    );
    assert.equal(validation.compatibility.compatible, true);
  });
});

test("official sample manifests stay aligned with sync profiles", () => {
  const scriptPath = path.resolve(__dirname, "../scripts/check-sample-manifests.mjs");
  const output = execFileSync(process.execPath, [scriptPath], {
    encoding: "utf8",
    cwd: path.resolve(__dirname, "../../../")
  });

  assert.match(output, /all current-branch sample plugin manifests are in sync/);
});

test("validatePluginManifestSchema reports schema, capability, webhook, and compatibility issues", () => {
  const validation = validatePluginManifestSchema({
    runtime: "js_worker",
    manifest_version: "1.0.0",
    protocol_version: "1.0.0",
    max_host_protocol_version: "0.9.0",
    config_schema: {
      fields: [
        {
          key: "mode",
          type: "select",
          default: "prod",
          options: [{ value: "dev" }]
        },
        {
          key: "mode"
        }
      ]
    },
    permissions: [
      {},
      { key: "custom.demo", required: "yes" },
      { key: "custom.demo" }
    ],
    frontend: {
      admin_page: {
        path: "/plugin-pages/demo"
      }
    },
    webhooks: [
      {
        key: "notify",
        method: "TRACE",
        auth_mode: "header"
      }
    ],
    capabilities: {
      execute_action_storage: {
        "": "read",
        "demo.action": "broken"
      },
      frontend_min_scope: "staff",
      frontend_allowed_areas: ["admin", "ops"],
      frontend_html_mode: "unsafe"
    }
  });

  assert.equal(validation.valid, false);
  const issuePaths = validation.issues.map((issue) => issue.path);
  assert.ok(issuePaths.includes("config_schema.fields[0].default"));
  assert.ok(issuePaths.includes("config_schema.fields[1].key"));
  assert.ok(issuePaths.includes("permissions[0].key"));
  assert.ok(issuePaths.includes("permissions[1].required"));
  assert.ok(issuePaths.includes("permissions[2].key"));
  assert.ok(issuePaths.includes("frontend.admin_page.path"));
  assert.ok(issuePaths.includes("webhooks[0].method"));
  assert.ok(issuePaths.includes("webhooks[0].secret_key"));
  assert.ok(issuePaths.includes("capabilities.execute_action_storage"));
  assert.ok(issuePaths.includes("capabilities.execute_action_storage.demo.action"));
  assert.ok(issuePaths.includes("capabilities.frontend_min_scope"));
  assert.ok(issuePaths.includes("capabilities.frontend_allowed_areas[1]"));
  assert.ok(issuePaths.includes("capabilities.frontend_html_mode"));
  assert.ok(issuePaths.includes("max_host_protocol_version"));
  assert.equal(validation.compatibility.compatible, false);
  assert.equal(validation.compatibility.reason_code, "host_protocol_too_new");
});

test("safeParseStringMap normalizes non-string values and ignores invalid payloads", () => {
  assert.deepEqual(safeParseStringMap("{\"order_id\":123,\"enabled\":true}"), {
    order_id: "123",
    enabled: "true"
  });
  assert.deepEqual(safeParseStringMap("{"), {});
  assert.deepEqual(
    safeParseStringMap({
      order_id: 123,
      tab: "timeline",
      ignored: null
    }),
    {
      order_id: "123",
      tab: "timeline",
      ignored: ""
    }
  );
});

test("getPluginSecret and getPluginWebhook expose secret and webhook runtime APIs", () => {
  const previousPlugin = globalThis.Plugin;
  const secretAPI: PluginSecretAPI = {
    enabled: true,
    get: (key) => (key === "token" ? "abc123" : undefined),
    has: (key) => key === "token",
    list: () => ["token"]
  };
  const webhookAPI: PluginWebhookAPI = {
    enabled: true,
    key: "payment.notify",
    method: "POST",
    path: "/api/plugins/demo/webhooks/payment.notify",
    queryString: "source=stripe",
    queryParams: {
      source: "stripe"
    },
    headers: {
      "x-signature": "sig-1"
    },
    contentType: "application/json",
    remoteAddr: "127.0.0.1",
    bodyText: "{\"ok\":true}",
    bodyBase64: "eyJvayI6dHJ1ZX0=",
    header: (name) => (name.toLowerCase() === "x-signature" ? "sig-1" : undefined),
    query: (name) => (name === "source" ? "stripe" : undefined),
    text: () => "{\"ok\":true}",
    json: <T = unknown>() => ({ ok: true } as T)
  };

  try {
    globalThis.Plugin = {
      secret: secretAPI,
      webhook: webhookAPI
    };

    assert.equal(getPluginSecret(), secretAPI);
    assert.equal(getPluginWebhook(), webhookAPI);
    assert.equal(getPluginSecret()?.get("token"), "abc123");
    assert.equal(getPluginSecret()?.has("token"), true);
    assert.deepEqual(getPluginSecret()?.list(), ["token"]);
    assert.equal(getPluginWebhook()?.header("x-signature"), "sig-1");
    assert.equal(getPluginWebhook()?.query("source"), "stripe");
    assert.equal(getPluginWebhook()?.text(), "{\"ok\":true}");
    assert.deepEqual(getPluginWebhook()?.json(), { ok: true });
  } finally {
    globalThis.Plugin = previousPlugin;
  }
});

test("definePlugin dispatches object-style workspace handlers without injecting health metadata", () => {
  const previousPlugin = globalThis.Plugin;
  const workspaceEntries: string[] = [];
  const workspaceAPI: PluginWorkspaceAPI = {
    enabled: true,
    write(message) {
      workspaceEntries.push(String(message || ""));
    },
    writeln(message) {
      workspaceEntries.push(String(message || ""));
    },
    info(message) {
      workspaceEntries.push(String(message || ""));
    },
    warn(message) {
      workspaceEntries.push(String(message || ""));
    },
    error(message) {
      workspaceEntries.push(String(message || ""));
    },
    clear() {
      workspaceEntries.length = 0;
      return true;
    },
    tail() {
      return [];
    },
    snapshot() {
      return {
        enabled: true,
        max_entries: 32,
        entry_count: workspaceEntries.length,
        entries: []
      };
    },
    read() {
      return "";
    },
    readLine() {
      return "input";
    }
  };

  const plugin = definePlugin({
    execute() {
      return { success: true };
    },
    workspace: {
      "debugger.prompt": {
        name: "debugger/prompt",
        title: "Workspace Prompt",
        description: "Interactive prompt command",
        interactive: true,
        permissions: ["runtime.file_system"],
        handler(command, _context, _config, _sandbox, workspace) {
          workspace.info(`handled ${command.name}`);
          return {
            success: true,
            data: {
              command: command.name
            }
          };
        }
      },
      "debugger.report": {
        handler(command, _context, _config, _sandbox, workspace) {
          workspace.info(`handled ${command.entry}`);
          return {
            success: true,
            data: {
              command: command.entry
            }
          };
        }
      }
    }
  });

  try {
    globalThis.Plugin = {
      workspace: workspaceAPI
    };

    const health = plugin.health?.({}, {
      level: "balanced",
      requestedPermissions: [],
      grantedPermissions: []
    } as any);
    assert.equal(health?.healthy, true);
    const metadata = (health?.metadata || {}) as Record<string, unknown>;
    assert.equal(typeof metadata.workspace_commands_json, "undefined");

    const result = plugin.execute(
      "workspace.command.execute",
      {
        workspace_command_name: "debugger/prompt",
        workspace_command_entry: "debugger.prompt",
        workspace_command_raw: "debugger/prompt",
        workspace_command_argv_json: "[]",
        workspace_command_interactive: "true"
      },
      {} as PluginExecutionContext,
      {},
      {} as any
    ) as Record<string, unknown>;
    assert.equal(result.success, true);
    assert.equal(workspaceEntries.includes("handled debugger/prompt"), true);
  } finally {
    globalThis.Plugin = previousPlugin;
  }
});

test("defineWorkspaceCommand and defineWorkspaceCommands preserve function-first workspace definitions", () => {
  const prompt = defineWorkspaceCommand(
    {
      name: "debugger/prompt",
      title: "Prompt",
      interactive: true
    },
    (command) => ({
      success: true,
      data: {
        command: command.name
      }
    })
  );
  const report = defineWorkspaceCommand((command) => ({
    success: true,
    data: {
      command: command.entry
    }
  }));
  const workspace = defineWorkspaceCommands({
    "debugger.prompt": prompt,
    "debugger.report": report
  });

  assert.equal(typeof workspace["debugger.prompt"], "object");
  assert.equal(typeof workspace["debugger.report"], "object");
  assert.equal(workspace["debugger.prompt"].interactive, true);
  assert.equal(workspace["debugger.prompt"].name, "debugger/prompt");
  assert.equal(typeof workspace["debugger.prompt"].handler, "function");
  assert.equal(typeof workspace["debugger.report"].handler, "function");
});

test("buildPluginActionButtonExtension emits toolbar-ready frontend extension", () => {
  const extension = buildPluginActionButtonExtension({
    slot: "admin.orders.actions",
    title: "Open Logistics",
    href: "/admin/plugin-pages/logistics?source=orders",
    icon: "truck",
    variant: "outline",
    size: "sm",
    target: "_self"
  });

  assert.deepEqual(extension, {
    type: "action_button",
    slot: "admin.orders.actions",
    title: "Open Logistics",
    link: "/admin/plugin-pages/logistics?source=orders",
    data: {
      label: "Open Logistics",
      href: "/admin/plugin-pages/logistics?source=orders",
      icon: "truck",
      variant: "outline",
      size: "sm",
      target: "_self"
    }
  });
});

test("getPluginHost and typed host getters resolve nested host runtime APIs", () => {
  const previousPlugin = globalThis.Plugin;
  const orderAPI = createOrderAPI(1);
  const userAPI = createUserAPI(2);
  const productAPI = createProductAPI(3);
  const inventoryAPI = createInventoryAPI(4);
  const inventoryBindingAPI = createInventoryBindingAPI(11);
  const promoAPI = createPromoAPI(5);
  const ticketAPI = createTicketAPI(6);
  const serialAPI = createSerialAPI(7);
  const announcementAPI = createAnnouncementAPI(8);
  const knowledgeAPI = createKnowledgeAPI(9);
  const paymentMethodAPI = createPaymentMethodAPI(10);
  const virtualInventoryAPI = createVirtualInventoryAPI(12);
  const virtualInventoryBindingAPI = createVirtualInventoryBindingAPI(13);
  const marketAPI = createMarketAPI("nested");
  const emailTemplateAPI = createEmailTemplateAPI("nested");
  const landingPageAPI = createLandingPageAPI("nested");
  const invoiceTemplateAPI = createInvoiceTemplateAPI("nested");
  const authBrandingAPI = createAuthBrandingAPI("nested");
  const pageRulePackAPI = createPageRulePackAPI("nested");
  const hostAPI: PluginHostAPI = {
    enabled: true,
    invoke: <T = unknown>() => ({ ok: true } as T),
    order: orderAPI,
    user: userAPI,
    product: productAPI,
    inventory: inventoryAPI,
    inventoryBinding: inventoryBindingAPI,
    promo: promoAPI,
    ticket: ticketAPI,
    serial: serialAPI,
    announcement: announcementAPI,
    knowledge: knowledgeAPI,
    paymentMethod: paymentMethodAPI,
    virtualInventory: virtualInventoryAPI,
    virtualInventoryBinding: virtualInventoryBindingAPI,
    market: marketAPI,
    emailTemplate: emailTemplateAPI,
    landingPage: landingPageAPI,
    invoiceTemplate: invoiceTemplateAPI,
    authBranding: authBrandingAPI,
    pageRulePack: pageRulePackAPI
  };

  try {
    globalThis.Plugin = {
      host: hostAPI
    };

    assert.equal(getPluginHost(), hostAPI);
    assert.equal(getPluginOrder(), orderAPI);
    assert.equal(getPluginUser(), userAPI);
    assert.equal(getPluginProduct(), productAPI);
    assert.equal(getPluginInventory(), inventoryAPI);
    assert.equal(getPluginInventoryBinding(), inventoryBindingAPI);
    assert.equal(getPluginPromo(), promoAPI);
    assert.equal(getPluginTicket(), ticketAPI);
    assert.equal(getPluginSerial(), serialAPI);
    assert.equal(getPluginAnnouncement(), announcementAPI);
    assert.equal(getPluginKnowledge(), knowledgeAPI);
    assert.equal(getPluginPaymentMethod(), paymentMethodAPI);
    assert.equal(getPluginVirtualInventory(), virtualInventoryAPI);
    assert.equal(getPluginVirtualInventoryBinding(), virtualInventoryBindingAPI);
    assert.equal(getPluginMarket(), marketAPI);
    assert.equal(getPluginEmailTemplate(), emailTemplateAPI);
    assert.equal(getPluginLandingPage(), landingPageAPI);
    assert.equal(getPluginInvoiceTemplate(), invoiceTemplateAPI);
    assert.equal(getPluginAuthBranding(), authBrandingAPI);
    assert.equal(getPluginPageRulePack(), pageRulePackAPI);
    assert.deepEqual(getPluginMarket()?.install?.execute({ name: "demo", kind: "plugin_package", source_id: "official", version: "1.2.0" }), {
      status: "activated",
      label: "nested"
    });
    assert.deepEqual(getPluginOrder()?.markPaid({ id: 1 }), {
      id: 1,
      status: "pending"
    });
    assert.deepEqual(getPluginOrder()?.updatePrice({ totalAmountMinor: 2888 }), {
      id: 1,
      total_amount_minor: 2888
    });
    assert.deepEqual(getPluginTicket()?.update({ id: 6, status: "resolved", priority: "high" }), {
      id: 6,
      status: "resolved",
      priority: "high"
    });
    assert.deepEqual(getPluginMarket()?.install?.preview({ source_id: "nested-source", kind: "plugin_package", name: "demo", version: "1.0.0" }), {
      ok: true,
      label: "nested"
    });
    assert.deepEqual(getPluginMarket()?.install?.task?.get({ task_id: "nested-task" }), {
      task_id: "nested-task",
      status: "succeeded"
    });
    assert.deepEqual(getPluginMarket()?.install?.history?.list({ source_id: "nested-source", kind: "plugin_package", name: "demo" }), {
      items: [{ version: "nested-history" }]
    });
    assert.deepEqual(getPluginMarket()?.install?.rollback({ source_id: "nested-source", kind: "plugin_package", name: "demo", version: "1.0.0" }), {
      status: "rolled_back",
      label: "nested"
    });
    assert.deepEqual(getPluginEmailTemplate()?.save({ key: "nested-order_paid", content: "<html />" }), {
      key: "nested-order_paid",
      saved: true
    });
    assert.deepEqual(getPluginLandingPage()?.reset({ page_key: "nested-home" }), {
      reset: true,
      page_key: "nested-home"
    });
    assert.deepEqual(getPluginInvoiceTemplate()?.reset({ target_key: "invoice" }), {
      target_key: "invoice",
      reset: true
    });
    assert.deepEqual(getPluginAuthBranding()?.save({ target_key: "auth_branding", content: "<section />" }), {
      target_key: "auth_branding",
      saved: true
    });
    assert.deepEqual(getPluginPageRulePack()?.get({ target_key: "page_rules" }), {
      target_key: "page_rules",
      digest: "nested-page-rules-digest"
    });
  } finally {
    globalThis.Plugin = previousPlugin;
  }
});

test("typed host getters prefer top-level runtime aliases when available", () => {
  const previousPlugin = globalThis.Plugin;
  const nestedOrderAPI = createOrderAPI(1);
  const nestedUserAPI = createUserAPI(2);
  const nestedProductAPI = createProductAPI(3);
  const nestedInventoryAPI = createInventoryAPI(4);
  const nestedInventoryBindingAPI = createInventoryBindingAPI(10);
  const nestedPromoAPI = createPromoAPI(5);
  const nestedTicketAPI = createTicketAPI(6);
  const nestedSerialAPI = createSerialAPI(7);
  const nestedAnnouncementAPI = createAnnouncementAPI(8);
  const nestedKnowledgeAPI = createKnowledgeAPI(9);
  const nestedPaymentMethodAPI = createPaymentMethodAPI(10);
  const nestedVirtualInventoryAPI = createVirtualInventoryAPI(11);
  const nestedVirtualInventoryBindingAPI = createVirtualInventoryBindingAPI(12);
  const nestedMarketAPI = createMarketAPI("nested");
  const nestedEmailTemplateAPI = createEmailTemplateAPI("nested");
  const nestedLandingPageAPI = createLandingPageAPI("nested");
  const topLevelProductAPI = createProductAPI(8, 2, 10, 1);
  const topLevelInventoryAPI = createInventoryAPI(9, 2, 10, 1);
  const topLevelInventoryBindingAPI = createInventoryBindingAPI(16, 2, 10, 1);
  const topLevelPromoAPI = createPromoAPI(10, 2, 10, 1);
  const topLevelTicketAPI = createTicketAPI(11, 2, 10, 1);
  const topLevelSerialAPI = createSerialAPI(12, 2, 10, 1);
  const topLevelAnnouncementAPI = createAnnouncementAPI(13, 2, 10, 1);
  const topLevelKnowledgeAPI = createKnowledgeAPI(14, 2, 10, 1);
  const topLevelPaymentMethodAPI = createPaymentMethodAPI(15, 2, 10, 1);
  const topLevelVirtualInventoryAPI = createVirtualInventoryAPI(17, 2, 10, 1);
  const topLevelVirtualInventoryBindingAPI = createVirtualInventoryBindingAPI(18, 2, 10, 1);
  const topLevelMarketAPI = createMarketAPI("top");
  const topLevelEmailTemplateAPI = createEmailTemplateAPI("top");
  const topLevelLandingPageAPI = createLandingPageAPI("top");
  const nestedInvoiceTemplateAPI = createInvoiceTemplateAPI("nested");
  const nestedAuthBrandingAPI = createAuthBrandingAPI("nested");
  const nestedPageRulePackAPI = createPageRulePackAPI("nested");
  const topLevelInvoiceTemplateAPI = createInvoiceTemplateAPI("top");
  const topLevelAuthBrandingAPI = createAuthBrandingAPI("top");
  const topLevelPageRulePackAPI = createPageRulePackAPI("top");
  const topLevelOrderAPI = createOrderAPI(3, 2, 10, 1);
  const topLevelUserAPI = createUserAPI(4, 2, 10, 1);
  const hostAPI: PluginHostAPI = {
    enabled: true,
    invoke: <T = unknown>() => ({ ok: true } as T),
    order: nestedOrderAPI,
    user: nestedUserAPI,
    product: nestedProductAPI,
    inventory: nestedInventoryAPI,
    inventoryBinding: nestedInventoryBindingAPI,
    promo: nestedPromoAPI,
    ticket: nestedTicketAPI,
    serial: nestedSerialAPI,
    announcement: nestedAnnouncementAPI,
    knowledge: nestedKnowledgeAPI,
    paymentMethod: nestedPaymentMethodAPI,
    virtualInventory: nestedVirtualInventoryAPI,
    virtualInventoryBinding: nestedVirtualInventoryBindingAPI,
    market: nestedMarketAPI,
    emailTemplate: nestedEmailTemplateAPI,
    landingPage: nestedLandingPageAPI,
    invoiceTemplate: nestedInvoiceTemplateAPI,
    authBranding: nestedAuthBrandingAPI,
    pageRulePack: nestedPageRulePackAPI
  };

  try {
    globalThis.Plugin = {
      host: hostAPI,
      order: topLevelOrderAPI,
      user: topLevelUserAPI,
      product: topLevelProductAPI,
      inventory: topLevelInventoryAPI,
      inventoryBinding: topLevelInventoryBindingAPI,
      promo: topLevelPromoAPI,
      ticket: topLevelTicketAPI,
      serial: topLevelSerialAPI,
      announcement: topLevelAnnouncementAPI,
      knowledge: topLevelKnowledgeAPI,
      paymentMethod: topLevelPaymentMethodAPI,
      virtualInventory: topLevelVirtualInventoryAPI,
      virtualInventoryBinding: topLevelVirtualInventoryBindingAPI,
      market: topLevelMarketAPI,
      emailTemplate: topLevelEmailTemplateAPI,
      landingPage: topLevelLandingPageAPI,
      invoiceTemplate: topLevelInvoiceTemplateAPI,
      authBranding: topLevelAuthBrandingAPI,
      pageRulePack: topLevelPageRulePackAPI
    };

    assert.equal(getPluginOrder(), topLevelOrderAPI);
    assert.equal(getPluginUser(), topLevelUserAPI);
    assert.equal(getPluginProduct(), topLevelProductAPI);
    assert.equal(getPluginInventory(), topLevelInventoryAPI);
    assert.equal(getPluginInventoryBinding(), topLevelInventoryBindingAPI);
    assert.equal(getPluginPromo(), topLevelPromoAPI);
    assert.equal(getPluginTicket(), topLevelTicketAPI);
    assert.equal(getPluginSerial(), topLevelSerialAPI);
    assert.equal(getPluginAnnouncement(), topLevelAnnouncementAPI);
    assert.equal(getPluginKnowledge(), topLevelKnowledgeAPI);
    assert.equal(getPluginPaymentMethod(), topLevelPaymentMethodAPI);
    assert.equal(getPluginVirtualInventory(), topLevelVirtualInventoryAPI);
    assert.equal(getPluginVirtualInventoryBinding(), topLevelVirtualInventoryBindingAPI);
    assert.equal(getPluginMarket(), topLevelMarketAPI);
    assert.equal(getPluginEmailTemplate(), topLevelEmailTemplateAPI);
    assert.equal(getPluginLandingPage(), topLevelLandingPageAPI);
    assert.equal(getPluginInvoiceTemplate(), topLevelInvoiceTemplateAPI);
    assert.equal(getPluginAuthBranding(), topLevelAuthBrandingAPI);
    assert.equal(getPluginPageRulePack(), topLevelPageRulePackAPI);
  } finally {
    globalThis.Plugin = previousPlugin;
  }
});
