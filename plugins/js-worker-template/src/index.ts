import {
  asRecord,
  asString,
  buildPluginAlertBlock,
  buildPluginJSONViewBlock,
  buildPluginKeyValueBlock,
  definePlugin,
  defineWorkspaceCommand,
  defineWorkspaceCommands,
  errorResult,
  getPluginOrder,
  getPluginStorage,
  getPluginUser,
  getPluginWorkspace,
  normalizeHookName,
  prettyJSON,
  resolvePluginPageContext,
  safeParseJSON,
  successResult,
  type PluginExecutionContext,
  type PluginExecuteResult,
  type PluginHealthResult,
  type PluginPageBlock,
  type PluginSandboxProfile,
  type PluginWorkspaceAPI,
  type PluginWorkspaceCommandContext,
  type UnknownMap
} from "@auralogic/plugin-sdk";

import {
  PAGE_STATE_STORAGE_KEY,
  PLUGIN_IDENTITY,
  PLUGIN_DISPLAY_NAME,
  WORKSPACE_LAST_INPUT_STORAGE_KEY
} from "./lib/constants";
import {
  buildBootstrapExtensions,
  buildDefaultTemplatePageState,
  type TemplatePageState
} from "./lib/frontend";

type TemplateRuntimeAvailability = {
  has_workspace: boolean;
  workspace_enabled: boolean;
  has_order_helper: boolean;
  has_user_helper: boolean;
};

function parseJSONObject(value: unknown): UnknownMap {
  if (typeof value === "string") {
    return asRecord(safeParseJSON(value));
  }
  return asRecord(value);
}

function parseJSONValueIfPossible(value: string): unknown {
  try {
    return JSON.parse(value);
  } catch {
    return value;
  }
}

function parseOptionalInteger(value: unknown): number | undefined {
  const text = asString(value);
  if (!text) {
    return undefined;
  }
  const parsed = Number.parseInt(text, 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : undefined;
}

function getGreeting(config: UnknownMap): string {
  const greeting = asString(config.greeting);
  return greeting || "hello from template";
}

function canUseStorage(config: UnknownMap): boolean {
  return config.enable_storage_demo !== false;
}

function getTemplateStorage() {
  return getPluginStorage();
}

function buildTemplateRuntimeAvailability(): TemplateRuntimeAvailability {
  const workspace = getPluginWorkspace();
  const order = getPluginOrder();
  const user = getPluginUser();
  return {
    has_workspace: Boolean(workspace),
    workspace_enabled: Boolean(workspace?.enabled),
    has_order_helper: Boolean(order && typeof order.get === "function"),
    has_user_helper: Boolean(user && typeof user.get === "function")
  };
}

function normalizePayloadJSONString(value: unknown, fallback: string): string {
  if (typeof value === "string") {
    const trimmed = value.trim();
    if (!trimmed) {
      return fallback;
    }
    try {
      return JSON.stringify(JSON.parse(trimmed), null, 2);
    } catch {
      return value;
    }
  }
  if (value === null || value === undefined) {
    return fallback;
  }
  return prettyJSON(value);
}

function mergeTemplatePageState(
  input: UnknownMap,
  base: TemplatePageState
): TemplatePageState {
  return {
    greeting: asString(input.greeting) || base.greeting,
    note: asString(input.note) || base.note,
    payload_json: normalizePayloadJSONString(input.payload_json, base.payload_json)
  };
}

function readTemplatePageState(config: UnknownMap): {
  source: string;
  state: TemplatePageState;
} {
  const defaults = buildDefaultTemplatePageState(config);
  const storage = getTemplateStorage();
  const raw = storage?.get(PAGE_STATE_STORAGE_KEY);
  if (!raw) {
    return {
      source: "manifest.config",
      state: defaults
    };
  }
  return {
    source: "plugin.storage",
    state: mergeTemplatePageState(parseJSONObject(raw), defaults)
  };
}

function resolveTemplatePageState(params: UnknownMap, config: UnknownMap): TemplatePageState {
  const current = readTemplatePageState(config).state;
  return mergeTemplatePageState(params, current);
}

function buildTemplateHostLookupSeed(context: PluginExecutionContext): UnknownMap {
  const pageContext = resolvePluginPageContext(context);
  return {
    order_id:
      pageContext.query_params.order_id ||
      (context.order_id !== undefined && context.order_id !== null ? String(context.order_id) : ""),
    order_no: pageContext.route_params.orderNo || pageContext.query_params.order_no || "",
    user_id:
      pageContext.query_params.user_id ||
      (context.user_id !== undefined && context.user_id !== null ? String(context.user_id) : ""),
    user_email: pageContext.query_params.user_email || ""
  };
}

function resolveTemplateHostLookupInput(
  params: UnknownMap,
  context: PluginExecutionContext
): {
  order_id: string;
  order_no: string;
  user_id: string;
  user_email: string;
} {
  const defaults = buildTemplateHostLookupSeed(context);
  return {
    order_id: asString(params.order_id) || asString(defaults.order_id),
    order_no: asString(params.order_no) || asString(defaults.order_no),
    user_id: asString(params.user_id) || asString(defaults.user_id),
    user_email: asString(params.user_email) || asString(defaults.user_email)
  };
}

function resolveSandboxStorageMode(
  sandbox: PluginSandboxProfile,
  actionName: string,
  key: "declaredStorageAccessMode" | "storageAccessMode"
): string {
  const direct = asString(sandbox[key]);
  if (direct) {
    return direct;
  }
  if (key === "declaredStorageAccessMode") {
    const mapping = sandbox.executeActionStorage || {};
    const mapped = mapping[actionName];
    return asString(mapped) || "unknown";
  }
  return "unknown";
}

function buildContextBlocks(
  title: string,
  message: string,
  state: TemplatePageState,
  context: PluginExecutionContext,
  sandbox: PluginSandboxProfile,
  actionName: string
): PluginPageBlock[] {
  const pageContext = resolvePluginPageContext(context);
  const storage = getTemplateStorage();
  const runtimeAvailability = buildTemplateRuntimeAvailability();
  return [
    buildPluginAlertBlock({
      title,
      content: message,
      variant: "success"
    }),
    buildPluginKeyValueBlock({
      title: "Execution Context",
      items: [
        { label: "User ID", value: context.user_id ?? "-" },
        { label: "Order ID", value: context.order_id ?? "-" },
        { label: "Session ID", value: context.session_id || "-" },
        { label: "Request Path", value: context.metadata?.request_path || "-" },
        { label: "Plugin Page Path", value: pageContext.path || "-" },
        { label: "Plugin Page Full Path", value: pageContext.full_path || "-" },
        { label: "Plugin Page Query String", value: pageContext.query_string || "-" },
        { label: "Bootstrap Area", value: pageContext.area || "-" },
        { label: "Current Action", value: asString(sandbox.currentAction) || actionName || "-" },
        {
          label: "Declared Storage Access",
          value: resolveSandboxStorageMode(sandbox, actionName, "declaredStorageAccessMode")
        },
        {
          label: "Observed Storage Access",
          value: resolveSandboxStorageMode(sandbox, actionName, "storageAccessMode")
        },
        {
          label: "Route Param orderNo",
          value: pageContext.route_params.orderNo || "-"
        },
        {
          label: "Query Param order_id",
          value: pageContext.query_params.order_id || "-"
        },
        {
          label: "Plugin.workspace Enabled",
          value: runtimeAvailability.workspace_enabled ? "yes" : "no"
        },
        {
          label: "Plugin.order Helper",
          value: runtimeAvailability.has_order_helper ? "available" : "missing"
        },
        {
          label: "Plugin.user Helper",
          value: runtimeAvailability.has_user_helper ? "available" : "missing"
        },
        {
          label: "Last Workspace Input",
          value: storage?.get(WORKSPACE_LAST_INPUT_STORAGE_KEY) || "-"
        }
      ]
    }),
    buildPluginJSONViewBlock({
      title: "Plugin Page Context",
      value: pageContext
    }),
    buildPluginJSONViewBlock({
      title: "Current State",
      value: {
        greeting: state.greeting,
        note: state.note,
        payload_json: parseJSONValueIfPossible(state.payload_json)
      }
    })
  ];
}

function executeHook(
  params: UnknownMap,
  config: UnknownMap
): PluginExecuteResult | UnknownMap {
  const hook = normalizeHookName(params.hook);
  const payload = parseJSONObject(params.payload);

  if (hook === "frontend.bootstrap") {
    const area = asString(payload.area).toLowerCase() === "admin" ? "admin" : "user";
    const state = readTemplatePageState(config).state;
    return {
      success: true,
      frontend_extensions: buildBootstrapExtensions(area, state)
    };
  }

  return successResult({
    message: `Handled hook ${hook || "unknown"}.`,
    values: {
      hook,
      payload,
      greeting: getGreeting(config)
    }
  });
}

function executePageGet(
  config: UnknownMap,
  context: PluginExecutionContext,
  sandbox: PluginSandboxProfile,
  actionName: string
): PluginExecuteResult {
  const current = readTemplatePageState(config);
  return successResult({
    source: current.source,
    message: "Loaded template page state.",
    values: {
      ...(current.state as unknown as UnknownMap),
      ...buildTemplateHostLookupSeed(context)
    },
    blocks: buildContextBlocks(
      "Template State",
      "State loaded for the visual form.",
      current.state,
      context,
      sandbox,
      actionName
    )
  });
}

function executePageSave(
  params: UnknownMap,
  config: UnknownMap,
  context: PluginExecutionContext,
  sandbox: PluginSandboxProfile,
  actionName: string
): PluginExecuteResult {
  if (!canUseStorage(config)) {
    return errorResult("Storage demo is disabled by config.enable_storage_demo");
  }

  const storage = getTemplateStorage();
  if (!storage || typeof storage.set !== "function") {
    return errorResult("Plugin.storage is unavailable");
  }

  const state = resolveTemplatePageState(params, config);
  const saved = storage.set(PAGE_STATE_STORAGE_KEY, JSON.stringify(state));
  if (!saved) {
    return errorResult("Plugin.storage.set returned false");
  }

  return successResult({
    source: "plugin.storage",
    message: "Template page state saved to Plugin.storage.",
    values: state as unknown as UnknownMap,
    blocks: buildContextBlocks(
      "Template State Saved",
      "The visual form wrote its values through the plugin execute API.",
      state,
      context,
      sandbox,
      actionName
    )
  });
}

function executeEcho(
  params: UnknownMap,
  config: UnknownMap,
  context: PluginExecutionContext,
  sandbox: PluginSandboxProfile,
  actionName: string
): PluginExecuteResult {
  const state = mergeTemplatePageState(params, buildDefaultTemplatePageState(config));
  return successResult({
    source: "execute.echo",
    message: "Template exec echo finished.",
    values: state as unknown as UnknownMap,
    blocks: buildContextBlocks(
      "Exec Echo",
      `${PLUGIN_DISPLAY_NAME} executed its own route action.`,
      state,
      context,
      sandbox,
      actionName
    )
  });
}

function executeHostLookup(
  params: UnknownMap,
  context: PluginExecutionContext
): PluginExecuteResult {
  const pageContext = resolvePluginPageContext(context);
  const runtimeAvailability = buildTemplateRuntimeAvailability();
  const lookup = resolveTemplateHostLookupInput(params, context);
  const orderID = parseOptionalInteger(lookup.order_id);
  const userID = parseOptionalInteger(lookup.user_id);
  const orderQuery = orderID ? { id: orderID } : lookup.order_no ? { order_no: lookup.order_no } : null;
  const userQuery = userID ? { id: userID } : lookup.user_email ? { email: lookup.user_email } : null;

  if (!orderQuery && !userQuery) {
    return errorResult("Provide order_id / order_no or user_id / user_email before running host lookup.", {
      source: "template.host.lookup",
      message: "No native lookup target was resolved from the current fields or page context.",
      values: {
        ...lookup,
        ...runtimeAvailability,
        page_context: pageContext
      },
      blocks: [
        buildPluginAlertBlock({
          title: "Host Lookup Missing Input",
          content:
            "Fill order_id / order_no or user_id / user_email first. The form auto-fills from ?order_id and :orderNo when the host route provides them.",
          variant: "warning"
        }),
        buildPluginJSONViewBlock({
          title: "Resolved Page Context",
          value: pageContext
        })
      ]
    });
  }

  const orderAPI = getPluginOrder();
  const userAPI = getPluginUser();
  let orderResult: unknown = null;
  let userResult: unknown = null;
  const problems: string[] = [];

  if (orderQuery) {
    if (!orderAPI || typeof orderAPI.get !== "function") {
      problems.push("Plugin.order.get is unavailable in the current runtime or permission set.");
    } else {
      try {
        orderResult = orderAPI.get(orderQuery);
      } catch (error) {
        problems.push(`Order lookup failed: ${String((error as Error)?.message || error)}`);
      }
    }
  }

  if (userQuery) {
    if (!userAPI || typeof userAPI.get !== "function") {
      problems.push("Plugin.user.get is unavailable in the current runtime or permission set.");
    } else {
      try {
        userResult = userAPI.get(userQuery);
      } catch (error) {
        problems.push(`User lookup failed: ${String((error as Error)?.message || error)}`);
      }
    }
  }

  const blocks: PluginPageBlock[] = [
    buildPluginAlertBlock({
      title: "Native Host Lookup",
      content:
        problems.length > 0
          ? problems.join(" ")
          : "Template plugin executed typed Plugin.order / Plugin.user lookups through the host bridge.",
      variant: problems.length > 0 ? "warning" : "success"
    }),
    buildPluginKeyValueBlock({
      title: "Lookup Request",
      items: [
        { label: "Resolved Order Query", value: orderQuery ? prettyJSON(orderQuery) : "-" },
        { label: "Resolved User Query", value: userQuery ? prettyJSON(userQuery) : "-" },
        {
          label: "Plugin.order Helper",
          value: runtimeAvailability.has_order_helper ? "available" : "missing"
        },
        {
          label: "Plugin.user Helper",
          value: runtimeAvailability.has_user_helper ? "available" : "missing"
        },
        { label: "Page Query order_id", value: pageContext.query_params.order_id || "-" },
        { label: "Page Route orderNo", value: pageContext.route_params.orderNo || "-" }
      ]
    }),
    buildPluginJSONViewBlock({
      title: "Resolved Page Context",
      value: pageContext
    })
  ];

  if (orderQuery) {
    blocks.push(
      buildPluginJSONViewBlock({
        title: "Order Result",
        value: orderResult ?? { missing: true }
      })
    );
  }
  if (userQuery) {
    blocks.push(
      buildPluginJSONViewBlock({
        title: "User Result",
        value: userResult ?? { missing: true }
      })
    );
  }

  const data = {
    source: "template.host.lookup",
    message:
      problems.length > 0
        ? "Native host lookup finished with warnings."
        : "Native host lookup completed successfully.",
    values: {
      ...lookup,
      ...runtimeAvailability,
      order_query: orderQuery,
      user_query: userQuery,
      order_result: orderResult,
      user_result: userResult,
      page_context: pageContext
    },
    blocks
  };

  return problems.length > 0 ? errorResult(problems.join(" "), data) : successResult(data);
}

function executeWorkspaceContextCommand(
  command: PluginWorkspaceCommandContext,
  context: PluginExecutionContext,
  configInput: unknown,
  sandbox: PluginSandboxProfile,
  workspace: PluginWorkspaceAPI
): PluginExecuteResult {
  const config = asRecord(configInput);
  const pageContext = resolvePluginPageContext(context);
  const runtimeAvailability = buildTemplateRuntimeAvailability();
  const state = readTemplatePageState(config).state;

  workspace.info(`Workspace command ${command.name} started.`, {
    command: command.name,
    raw: command.raw
  });
  workspace.info(`Expanded argv: ${command.argv.join(" | ") || "(empty)"}`, {
    command: command.name
  });
  workspace.info(`Current page path: ${pageContext.full_path || pageContext.path || "-"}`, {
    command: command.name
  });

  return successResult({
    source: "workspace.command",
    message: `Template workspace summary generated for ${command.name}.`,
    values: {
      command: command.name,
      entry: command.entry,
      raw: command.raw,
      argv: command.argv,
      user_id: context.user_id ?? null,
      order_id: context.order_id ?? null,
      session_id: context.session_id || "",
      current_action: asString(sandbox.currentAction) || "workspace.command.execute",
      page_context: pageContext,
      helper_availability: runtimeAvailability,
      current_state: state
    }
  });
}

function executeWorkspacePromptCommand(
  command: PluginWorkspaceCommandContext,
  _context: PluginExecutionContext,
  configInput: unknown,
  _sandbox: PluginSandboxProfile,
  workspace: PluginWorkspaceAPI
): PluginExecuteResult {
  const config = asRecord(configInput);
  workspace.info(`Interactive workspace prompt started for ${command.name}.`, {
    command: command.name,
    interactive: "true"
  });

  let inputValue = "";
  try {
    inputValue = workspace.readLine("template> ", { echo: true });
  } catch (error) {
    return errorResult(String((error as Error)?.message || error), {
      source: "workspace.command",
      message: "Template workspace prompt failed.",
      values: {
        command: command.name
      }
    });
  }

  const storage = getTemplateStorage();
  const savedToStorage =
    canUseStorage(config) &&
    Boolean(storage && typeof storage.set === "function" && storage.set(WORKSPACE_LAST_INPUT_STORAGE_KEY, inputValue));

  workspace.info(`Captured workspace input: ${inputValue}`, {
    command: command.name,
    saved_to_storage: savedToStorage ? "true" : "false"
  });

  return successResult({
    source: "workspace.command",
    message: `Template workspace prompt captured: ${inputValue}`,
    values: {
      command: command.name,
      input: inputValue,
      saved_to_storage: savedToStorage
    }
  });
}

module.exports = definePlugin({
  execute(
    action: unknown,
    params: unknown,
    context: PluginExecutionContext,
    config: UnknownMap,
    sandbox: PluginSandboxProfile
  ): PluginExecuteResult | UnknownMap {
    const actionName = asString(action);
    const paramsRecord = asRecord(params);
    const contextRecord = asRecord(context) as PluginExecutionContext;

    switch (actionName) {
      case "hook.execute":
        return executeHook(paramsRecord, config);
      case "template.page.get":
      case "template.config.get":
        return executePageGet(config, contextRecord, sandbox, actionName);
      case "template.page.save":
      case "template.config.save":
        return executePageSave(paramsRecord, config, contextRecord, sandbox, actionName);
      case "template.echo":
        return executeEcho(paramsRecord, config, contextRecord, sandbox, actionName);
      case "template.host.lookup":
        return executeHostLookup(paramsRecord, contextRecord);
      default:
        return successResult({
          message: getGreeting(config),
          values: {
            action: actionName,
            params: paramsRecord,
            context: contextRecord,
            sandbox
          }
        });
    }
  },

  health(config: UnknownMap): PluginHealthResult {
    const runtimeAvailability = buildTemplateRuntimeAvailability();
    return {
      healthy: true,
      version: "0.2.2",
      metadata: {
        plugin: PLUGIN_IDENTITY,
        has_storage: Boolean(globalThis.Plugin?.storage),
        has_fs: Boolean(globalThis.Plugin?.fs),
        has_workspace: runtimeAvailability.has_workspace,
        workspace_enabled: runtimeAvailability.workspace_enabled,
        has_order_helper: runtimeAvailability.has_order_helper,
        has_user_helper: runtimeAvailability.has_user_helper,
        greeting: getGreeting(config),
        config_keys: Object.keys(config || {})
      }
    };
  },

  workspace: defineWorkspaceCommands({
    "template.context": defineWorkspaceCommand(
      {
        name: "template/context",
        title: "Template Context Summary",
        description:
          "Write a concise page-context, runtime, and helper summary into the plugin workspace buffer. Useful with shell variables like $PLUGIN_NAME and $WORKSPACE_STATUS.",
        interactive: false
      },
      executeWorkspaceContextCommand
    ),
    "template.prompt": defineWorkspaceCommand(
      {
        name: "template/prompt",
        title: "Template Prompt Demo",
        description:
          "Consume one workspace input line through Plugin.workspace.readLine() and persist the captured value into plugin storage when storage demo is enabled.",
        interactive: true
      },
      executeWorkspacePromptCommand
    )
  })
});
