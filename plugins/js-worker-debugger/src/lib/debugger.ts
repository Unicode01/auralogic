import {
  defineWorkspaceCommand,
  defineWorkspaceCommands,
  getPluginWorkspace,
  resolvePluginPageContext,
  type PluginExecutionContext,
  type PluginWorkspaceAPI,
  type PluginWorkspaceCommandContext
} from "@auralogic/plugin-sdk";
import {
  DEFAULT_FS_FORM_STATE,
  DEFAULT_SIMULATION_STATE,
  DEFAULT_STORAGE_FORM_STATE,
  DEFAULT_WORKER_FORM_STATE,
  PLUGIN_DISPLAY_NAME,
  PLUGIN_IDENTITY
} from "./constants";
import * as debuggerConfig from "./debugger-config";
import * as debuggerState from "./debugger-state";
import * as frontend from "./frontend";
import type {
  ActionPayload,
  DebuggerEvent,
  DebuggerProfile,
  GenericRecord,
  HookExecutionResponse,
  RuntimeSandboxInput,
  RuntimeStreamWriter,
  SimulatedHookState,
} from "./types";
import {
  asRecord,
  asString,
  missingPermissions,
  normalizeHookName,
  nowISO,
  prettyJSON,
  resolveSandboxProfile,
  safeParseJSONObject,
  truncateText
} from "./utils";
import {
  buildNetworkBlocks,
  buildDebuggerDashboardBlocks,
  buildFileBlocks,
  buildHostBlocks,
  buildProfileSummaryBlocks,
  buildSelfTestBlocks,
  buildSimulationBlocks,
  buildStorageBlocks,
  buildWorkerBlocks
} from "./view";

function buildProfile(
  config: unknown,
  sandboxInput: RuntimeSandboxInput,
  currentAction?: string,
  context?: PluginExecutionContext
): DebuggerProfile {
  const resolved = debuggerConfig.resolveDebuggerConfigWithSource(config);
  const sandbox = resolveSandboxProfile(sandboxInput);
  if (!sandbox.currentAction) {
    sandbox.currentAction = asString(currentAction).toLowerCase();
  }
  if (
    sandbox.declaredStorageAccessMode === "unknown" &&
    sandbox.currentAction &&
    sandbox.executeActionStorage[sandbox.currentAction]
  ) {
    const declaredMode = sandbox.executeActionStorage[sandbox.currentAction];
    if (declaredMode) {
      sandbox.declaredStorageAccessMode = declaredMode;
    }
  }
  const recentEvents = debuggerState.readDebuggerEvents();
  const storage = debuggerState.buildStorageSummary();
  const fs = debuggerState.buildFileSummary();
  const runtime = debuggerState.buildRuntimeProbe(fs);
  const secret = debuggerState.buildSecretSummary();
  const webhook = debuggerState.buildWebhookSummary();
  const actionTraces = debuggerState.readActionTraces();
  const pageContext = resolvePluginPageContext(context);

  const capabilityGaps = missingPermissions(sandbox.requestedPermissions, sandbox.grantedPermissions);
  const warnings: string[] = [];
  if (!sandbox.allowFrontendExtensions) {
    warnings.push("frontend.extensions is not granted. Slot injections and debugger routes can be filtered by host.");
  }
  if (!sandbox.allowExecuteAPI) {
    warnings.push("api.execute is not granted. Admin action forms will fail until it is enabled.");
  }
  if (!sandbox.allowPayloadPatch) {
    warnings.push("hook.payload_patch is not granted. Payload marker demo becomes read-only.");
  }
  if (!sandbox.allowHookBlock) {
    warnings.push("hook.block is not granted. Blocking demo results will be ignored by host.");
  }
  if (!sandbox.allowFileSystem) {
    warnings.push("runtime.file_system is not granted. Plugin.fs lab is disabled.");
  }
  if (!sandbox.allowNetwork) {
    warnings.push("runtime.network is not granted. Plugin.http lab is disabled.");
  }
  if (
    sandbox.grantedPermissions.includes("runtime.file_system") &&
    !sandbox.allowFileSystem
  ) {
    warnings.push(
      "runtime.file_system is granted, but effective sandbox allowFileSystem=false. Check plugin allow_file_system or host request policy."
    );
  }
  if (
    sandbox.grantedPermissions.includes("runtime.network") &&
    !sandbox.allowNetwork
  ) {
    warnings.push(
      "runtime.network is granted, but effective sandbox allowNetwork=false. Check plugin allow_network or host request policy."
    );
  }
  if (runtime.fsProbeError) {
    warnings.push(`Plugin.fs runtime probe failed: ${runtime.fsProbeError}`);
  }
  if (runtime.hasPluginSecret && !runtime.pluginSecretEnabled) {
    warnings.push("Plugin.secret exists but enabled=false. Secret diagnostics are unavailable until the host injects configured secrets.");
  }
  if (secret.error) {
    warnings.push(`Plugin.secret inspection failed: ${secret.error}`);
  }
  if (runtime.hasPluginWebhook && !runtime.pluginWebhookEnabled) {
    warnings.push("Plugin.webhook exists but enabled=false. Webhook diagnostics only become active during webhook-triggered executions.");
  }
  if (!runtime.hasPluginWorkspace) {
    warnings.push("Plugin.workspace helper is missing in current JS worker runtime.");
  } else if (runtime.hasPluginWorkspace && !runtime.pluginWorkspaceEnabled) {
    warnings.push("Plugin.workspace exists but enabled=false. Host workspace support is currently disabled.");
  } else if (runtime.workspaceProbeError) {
    warnings.push(`Plugin.workspace runtime probe failed: ${runtime.workspaceProbeError}`);
  }
  if (!runtime.hasWorkerGlobal) {
    warnings.push("Worker global is missing in current JS worker runtime. Parallel child runtimes are unavailable.");
  }
  if (!runtime.hasStructuredCloneGlobal) {
    warnings.push("structuredClone is missing in current JS worker runtime. Deep-clone diagnostics are incomplete.");
  }
  if (!runtime.hasQueueMicrotaskGlobal || !runtime.hasSetTimeoutGlobal || !runtime.hasClearTimeoutGlobal) {
    warnings.push("Async scheduler globals are incomplete. queueMicrotask/setTimeout/clearTimeout should all be available.");
  }
  if (!runtime.hasTextEncoderGlobal || !runtime.hasTextDecoderGlobal) {
    warnings.push("TextEncoder/TextDecoder are missing in current JS worker runtime. Binary/text codec diagnostics are incomplete.");
  }
  if (!runtime.hasAtobGlobal || !runtime.hasBtoaGlobal) {
    warnings.push("atob/btoa are missing in current JS worker runtime. Base64 diagnostics are incomplete.");
  }
  if (sandbox.allowNetwork && !runtime.hasPluginHTTP) {
    warnings.push("runtime.network is enabled, but Plugin.http is missing in current JS worker runtime.");
  } else if (sandbox.allowNetwork && runtime.hasPluginHTTP && !runtime.pluginHTTPEnabled) {
    warnings.push("Plugin.http exists but enabled=false. Effective network access is still disabled.");
  }
  if (
    sandbox.grantedPermissions.some((item) => item.startsWith("host.")) &&
    !runtime.hasPluginHost
  ) {
    warnings.push("host.* permissions are granted, but Plugin.host is missing in current JS worker runtime.");
  } else if (
    sandbox.grantedPermissions.some((item) => item.startsWith("host.")) &&
    runtime.hasPluginHost &&
    !runtime.pluginHostEnabled
  ) {
    warnings.push("Plugin.host exists but enabled=false. Host bridge is not available for current runtime request.");
  }
  if (
    sandbox.grantedPermissions.some((item) => item.startsWith("host.order.")) &&
    !runtime.hasPluginOrder
  ) {
    warnings.push("host.order.* permissions are granted, but Plugin.order helper is missing in current JS worker runtime.");
  }
  if (
    sandbox.grantedPermissions.some((item) => item.startsWith("host.user.")) &&
    !runtime.hasPluginUser
  ) {
    warnings.push("host.user.* permissions are granted, but Plugin.user helper is missing in current JS worker runtime.");
  }
  if (
    sandbox.grantedPermissions.some((item) => item.startsWith("host.product.")) &&
    !runtime.hasPluginProduct
  ) {
    warnings.push("host.product.* permissions are granted, but Plugin.product helper is missing in current JS worker runtime.");
  }
  if (
    sandbox.grantedPermissions.some((item) => item.startsWith("host.inventory.")) &&
    !runtime.hasPluginInventory
  ) {
    warnings.push("host.inventory.* permissions are granted, but Plugin.inventory helper is missing in current JS worker runtime.");
  }
  if (
    sandbox.grantedPermissions.some((item) => item.startsWith("host.inventory_binding.")) &&
    !runtime.hasPluginInventoryBinding
  ) {
    warnings.push("host.inventory_binding.* permissions are granted, but Plugin.inventoryBinding helper is missing in current JS worker runtime.");
  }
  if (
    sandbox.grantedPermissions.some((item) => item.startsWith("host.promo.")) &&
    !runtime.hasPluginPromo
  ) {
    warnings.push("host.promo.* permissions are granted, but Plugin.promo helper is missing in current JS worker runtime.");
  }
  if (
    sandbox.grantedPermissions.some((item) => item.startsWith("host.ticket.")) &&
    !runtime.hasPluginTicket
  ) {
    warnings.push("host.ticket.* permissions are granted, but Plugin.ticket helper is missing in current JS worker runtime.");
  }
  if (
    sandbox.grantedPermissions.some((item) => item.startsWith("host.serial.")) &&
    !runtime.hasPluginSerial
  ) {
    warnings.push("host.serial.* permissions are granted, but Plugin.serial helper is missing in current JS worker runtime.");
  }
  if (
    sandbox.grantedPermissions.some((item) => item.startsWith("host.announcement.")) &&
    !runtime.hasPluginAnnouncement
  ) {
    warnings.push("host.announcement.* permissions are granted, but Plugin.announcement helper is missing in current JS worker runtime.");
  }
  if (
    sandbox.grantedPermissions.some((item) => item.startsWith("host.knowledge.")) &&
    !runtime.hasPluginKnowledge
  ) {
    warnings.push("host.knowledge.* permissions are granted, but Plugin.knowledge helper is missing in current JS worker runtime.");
  }
  if (
    sandbox.grantedPermissions.some((item) => item.startsWith("host.payment_method.")) &&
    !runtime.hasPluginPaymentMethod
  ) {
    warnings.push("host.payment_method.* permissions are granted, but Plugin.paymentMethod helper is missing in current JS worker runtime.");
  }
  if (
    sandbox.grantedPermissions.some((item) => item.startsWith("host.virtual_inventory.")) &&
    !runtime.hasPluginVirtualInventory
  ) {
    warnings.push("host.virtual_inventory.* permissions are granted, but Plugin.virtualInventory helper is missing in current JS worker runtime.");
  }
  if (
    sandbox.grantedPermissions.some((item) => item.startsWith("host.virtual_inventory_binding.")) &&
    !runtime.hasPluginVirtualInventoryBinding
  ) {
    warnings.push("host.virtual_inventory_binding.* permissions are granted, but Plugin.virtualInventoryBinding helper is missing in current JS worker runtime.");
  }
  if (
    sandbox.grantedPermissions.some((item) => item.startsWith("host.market.")) &&
    !runtime.hasPluginMarket
  ) {
    warnings.push("host.market.* permissions are granted, but Plugin.market helper is missing in current JS worker runtime.");
  }
  if (
    sandbox.grantedPermissions.some((item) => item.startsWith("host.email_template.")) &&
    !runtime.hasPluginEmailTemplate
  ) {
    warnings.push("host.email_template.* permissions are granted, but Plugin.emailTemplate helper is missing in current JS worker runtime.");
  }
  if (
    sandbox.grantedPermissions.some((item) => item.startsWith("host.landing_page.")) &&
    !runtime.hasPluginLandingPage
  ) {
    warnings.push("host.landing_page.* permissions are granted, but Plugin.landingPage helper is missing in current JS worker runtime.");
  }
  if (
    sandbox.grantedPermissions.some((item) => item.startsWith("host.invoice_template.")) &&
    !runtime.hasPluginInvoiceTemplate
  ) {
    warnings.push("host.invoice_template.* permissions are granted, but Plugin.invoiceTemplate helper is missing in current JS worker runtime.");
  }
  if (
    sandbox.grantedPermissions.some((item) => item.startsWith("host.auth_branding.")) &&
    !runtime.hasPluginAuthBranding
  ) {
    warnings.push("host.auth_branding.* permissions are granted, but Plugin.authBranding helper is missing in current JS worker runtime.");
  }
  if (
    sandbox.grantedPermissions.some((item) => item.startsWith("host.page_rule_pack.")) &&
    !runtime.hasPluginPageRulePack
  ) {
    warnings.push("host.page_rule_pack.* permissions are granted, but Plugin.pageRulePack helper is missing in current JS worker runtime.");
  }
  if (capabilityGaps.length > 0) {
    warnings.push(`Requested but not granted: ${capabilityGaps.join(", ")}`);
  }
  if (sandbox.currentAction && sandbox.declaredStorageAccessMode === "unknown") {
    warnings.push(
      `Current action ${sandbox.currentAction} has no capabilities.execute_action_storage declaration. Host will fall back to conservative serialized execution.`
    );
  }

  return {
    config: resolved.config,
    enabled_hooks: debuggerConfig.resolveEnabledHooks(resolved.config),
    source: resolved.source,
    recent_events: recentEvents,
    action_traces: actionTraces,
    sandbox,
    storage,
    fs,
    runtime,
    page_context: {
      area: pageContext.area || "",
      path: pageContext.path || "",
      full_path: pageContext.full_path || "",
      query_string: pageContext.query_string || "",
      query_params: pageContext.query_params || {},
      route_params: pageContext.route_params || {}
    },
    secret,
    webhook,
    capability_gaps: capabilityGaps,
    warnings
  };
}

function buildActionResponse(payload: ActionPayload): GenericRecord {
  return {
    success: true,
    data: payload
  };
}

function buildActionErrorResponse(error: string, payload?: ActionPayload): GenericRecord {
  if (!payload) {
    return {
      success: false,
      error
    };
  }
  return {
    success: false,
    error,
    data: payload
  };
}

function isPromiseLike<T>(value: T | Promise<T>): value is Promise<T> {
  return Boolean(value && typeof (value as Promise<T>).then === "function");
}

function resolveActionCategory(action: string): string {
  if (action.startsWith("hook.")) {
    return "hook";
  }
  if (action.startsWith("workspace:") || action.startsWith("debugger.workspace.")) {
    return "workspace";
  }
  if (action.startsWith("debugger.network.")) {
    return "network";
  }
  if (action.startsWith("debugger.host.")) {
    return "host";
  }
  if (action.startsWith("debugger.fs.")) {
    return "filesystem";
  }
  if (action.startsWith("debugger.storage.")) {
    return "storage";
  }
  if (action.startsWith("debugger.config.")) {
    return "config";
  }
  if (action.startsWith("debugger.simulate.")) {
    return "simulation";
  }
  if (action.startsWith("debugger.echo")) {
    return "execute_api";
  }
  if (action.startsWith("debugger.")) {
    return "debugger";
  }
  return "other";
}

function summarizeTraceValue(value: unknown, maxLength = 140): string {
  if (value === undefined || value === null) {
    return "-";
  }
  if (typeof value === "string") {
    return truncateText(value, maxLength);
  }
  if (typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  return truncateText(prettyJSON(value), maxLength);
}

function extractActionResultMessage(result: GenericRecord): string {
  const direct = asString(result.message);
  if (direct) {
    return direct;
  }
  const data = asRecord(result.data);
  const nested = asString(data.message);
  if (nested) {
    return nested;
  }
  const error = asString(result.error);
  if (error) {
    return error;
  }
  return result.success === false ? "Action failed." : "Action completed.";
}

function shouldPersistActionTrace(action: string): boolean {
  if (!action) {
    return false;
  }
  return ![
    "debugger.profile",
    "debugger.config.get",
    "debugger.action_traces.clear"
  ].includes(action);
}

function canPersistActionTraceForSandbox(
  sandbox: RuntimeSandboxInput,
  action: string
): boolean {
  const profile = resolveSandboxProfile(sandbox);
  const declaredMode =
    profile.declaredStorageAccessMode === "unknown" && action
      ? profile.executeActionStorage[action] || "unknown"
      : profile.declaredStorageAccessMode;
  return declaredMode === "write" || declaredMode === "unknown";
}

function recordActionTrace(
  action: string,
  params: GenericRecord,
  context: PluginExecutionContext,
  sandboxInput: RuntimeSandboxInput,
  result: GenericRecord
): void {
  if (!shouldPersistActionTrace(action)) {
    return;
  }
  const sandbox = resolveSandboxProfile(sandboxInput);
  if (!canPersistActionTraceForSandbox(sandboxInput, action)) {
    return;
  }
  const responsePayload = result.data !== undefined ? result.data : result;
  debuggerState.appendActionTrace(
    {
      id: `${action}:${nowISO()}:${Math.random().toString(36).slice(2, 8)}`,
      ts: nowISO(),
      action,
      category: resolveActionCategory(action),
      ok: result.success !== false,
      message: extractActionResultMessage(result),
      error: asString(result.error) || undefined,
      current_action: sandbox.currentAction || undefined,
      declared_storage_access_mode: sandbox.declaredStorageAccessMode,
      observed_storage_access_mode: sandbox.storageAccessMode,
      user_id: typeof context.user_id === "number" ? context.user_id : undefined,
      order_id: typeof context.order_id === "number" ? context.order_id : undefined,
      session_id: asString(context.session_id) || undefined,
      request_summary: summarizeTraceValue(params),
      response_summary: summarizeTraceValue(responsePayload),
      request_json: truncateText(prettyJSON(params), 6000),
      response_json: truncateText(prettyJSON(responsePayload), 10000)
    },
    60
  );
}

function buildEvent(
  hook: string,
  payload: GenericRecord,
  context: PluginExecutionContext,
  result: HookExecutionResponse
): DebuggerEvent {
  const group = debuggerConfig.findHookGroup(hook);
  return {
    id: `${hook}:${nowISO()}:${Math.random().toString(36).slice(2, 8)}`,
    ts: nowISO(),
    hook,
    group,
    area: asString(payload.area) || undefined,
    slot: asString(payload.slot) || undefined,
    path: asString(payload.path) || undefined,
    user_id: typeof context.user_id === "number" ? context.user_id : undefined,
    order_id: typeof context.order_id === "number" ? context.order_id : undefined,
    session_id: asString(context.session_id) || undefined,
    blocked: result.blocked || false,
    note: result.blocked
      ? result.block_reason || "blocked by debugger rule"
      : result.skipped
        ? result.reason || "hook skipped"
        : truncateText(prettyJSON(payload), 120),
    payload_json: prettyJSON(payload),
    context_json: prettyJSON(context || {})
  };
}

function shouldBlockHook(hook: string, payload: GenericRecord, profile: DebuggerProfile): boolean {
  if (!profile.config.demo_block_before_hooks) {
    return false;
  }
  if (!hook.endsWith(".before")) {
    return false;
  }
  const keyword = profile.config.block_keyword.trim().toLowerCase();
  if (!keyword) {
    return false;
  }
  const haystack = [
    hook,
    prettyJSON(payload),
    asString(payload.slot),
    asString(payload.path),
    asString(payload.code),
    asString(payload.keyword)
  ]
    .join("\n")
    .toLowerCase();
  return haystack.includes(keyword);
}

function buildPayloadPatch(hook: string): GenericRecord {
  return {
    plugin_debugger: {
      plugin: PLUGIN_IDENTITY,
      hook,
      ts: nowISO(),
      message: `${PLUGIN_DISPLAY_NAME} patched payload for ${hook}`
    }
  };
}

function emitWorkspaceTrace(actionName: string, message: string, metadata?: GenericRecord): void {
  const workspace = getPluginWorkspace();
  if (!workspace || !workspace.enabled || typeof workspace.info !== "function") {
    return;
  }
  try {
    workspace.info(message, {
      plugin: PLUGIN_IDENTITY,
      action: actionName,
      ...(metadata || {})
    });
  } catch {
    // Workspace tracing is best-effort and must never break the main action.
  }
}

function executeHookInternal(
  hook: string,
  payload: GenericRecord,
  context: PluginExecutionContext,
  profile: DebuggerProfile,
  options?: { persistEvent?: boolean }
): HookExecutionResponse {
  if (hook === "frontend.bootstrap") {
    const result: HookExecutionResponse = {
      success: true,
      frontend_extensions: frontend.buildBootstrapExtensions(payload, profile)
    };
    if (profile.config.persist_events && options?.persistEvent !== false) {
      debuggerState.appendDebuggerEvent(buildEvent(hook, payload, context, result), profile.config.max_events);
    }
    return result;
  }

  if (!debuggerConfig.isHookEnabled(profile.config, hook)) {
    return {
      success: true,
      skipped: true,
      reason: "hook group disabled",
      hook
    };
  }

  const result: HookExecutionResponse = {
    success: true
  };

  if (shouldBlockHook(hook, payload, profile)) {
    result.blocked = true;
    result.block_reason = `Plugin Debugger blocked ${hook} because payload matched keyword "${profile.config.block_keyword}".`;
  }

  if (profile.config.emit_payload_marker && !hook.endsWith(".before")) {
    result.payload = buildPayloadPatch(hook);
  }

  if (hook === "frontend.slot.render") {
    result.frontend_extensions = frontend.buildSlotExtensions(hook, payload, profile);
  }

  if (profile.config.persist_events && options?.persistEvent !== false) {
    debuggerState.appendDebuggerEvent(buildEvent(hook, payload, context, result), profile.config.max_events);
  }

  return result;
}

function buildSimulationState(params: GenericRecord): SimulatedHookState {
  const payload = safeParseJSONObject(params.simulate_payload);
  const state: SimulatedHookState = {
    simulate_hook: normalizeHookName(params.simulate_hook) || DEFAULT_SIMULATION_STATE.simulate_hook,
    simulate_area:
      asString(params.simulate_area).toLowerCase() === "user" ? "user" : "admin",
    simulate_slot: asString(params.simulate_slot) || DEFAULT_SIMULATION_STATE.simulate_slot,
    simulate_path: asString(params.simulate_path) || DEFAULT_SIMULATION_STATE.simulate_path,
    simulate_payload: prettyJSON(
      Object.keys(payload).length > 0 ? payload : safeParseJSONObject(DEFAULT_SIMULATION_STATE.simulate_payload)
    )
  };
  return state;
}

function simulateHook(
  params: GenericRecord,
  config: unknown,
  sandboxInput: RuntimeSandboxInput,
  context?: PluginExecutionContext
): GenericRecord {
  const state = buildSimulationState(params);
  const mergedPayload = safeParseJSONObject(state.simulate_payload);
  if (state.simulate_hook.startsWith("frontend.")) {
    mergedPayload.area = state.simulate_area;
    mergedPayload.slot = state.simulate_slot;
    mergedPayload.path = state.simulate_path;
  }
  const profile = buildProfile(config, sandboxInput, "debugger.simulate.hook", context);
  const result = executeHookInternal(
    state.simulate_hook,
    mergedPayload,
    { metadata: { simulated: "true" } },
    profile,
    { persistEvent: false }
  );
  emitWorkspaceTrace("debugger.simulate.hook", `Simulated hook ${state.simulate_hook}.`, {
    hook: state.simulate_hook,
    area: state.simulate_area,
    slot: state.simulate_slot,
    path: state.simulate_path
  });
  return buildActionResponse({
    source: "simulation",
    message: "Simulation finished.",
    values: state as unknown as GenericRecord,
    blocks: buildSimulationBlocks(profile, state, result)
  });
}

function configGet(config: unknown, sandboxInput: RuntimeSandboxInput): GenericRecord {
  const profile = buildProfile(config, sandboxInput, "debugger.config.get");
  return buildActionResponse({
    source: profile.source,
    values: profile.config as unknown as GenericRecord,
    blocks: buildProfileSummaryBlocks(profile)
  });
}

function configSet(params: GenericRecord, config: unknown, sandboxInput: RuntimeSandboxInput): GenericRecord {
  const setResult = debuggerConfig.setDebuggerConfig(params, config);
  const profile = buildProfile(setResult.config, sandboxInput, "debugger.config.set");
  return buildActionResponse({
    source: setResult.source,
    message: setResult.persisted
      ? "Debugger config saved to Plugin.storage."
      : "Debugger config updated for current runtime only.",
    values: profile.config as unknown as GenericRecord,
    blocks: buildProfileSummaryBlocks(profile)
  });
}

function configReset(config: unknown, sandboxInput: RuntimeSandboxInput): GenericRecord {
  const resetResult = debuggerConfig.resetDebuggerConfig(config);
  const profile = buildProfile(resetResult.config, sandboxInput, "debugger.config.reset");
  return buildActionResponse({
    source: resetResult.source,
    message: resetResult.reset ? "Debugger config reset to manifest defaults." : "Config reset not persisted.",
    values: profile.config as unknown as GenericRecord,
    blocks: buildProfileSummaryBlocks(profile)
  });
}

function clearEvents(config: unknown, sandboxInput: RuntimeSandboxInput): GenericRecord {
  const cleared = debuggerState.clearDebuggerEvents();
  const profile = buildProfile(config, sandboxInput, "debugger.events.clear");
  return buildActionResponse({
    source: profile.source,
    message: cleared ? "Hook events cleared." : "Hook event log was already empty.",
    values: profile.config as unknown as GenericRecord,
    blocks: buildProfileSummaryBlocks(profile)
  });
}

function clearActionTraceHistory(config: unknown, sandboxInput: RuntimeSandboxInput): GenericRecord {
  const cleared = debuggerState.clearActionTraces();
  const profile = buildProfile(config, sandboxInput, "debugger.action_traces.clear");
  return buildActionResponse({
    source: profile.source,
    message: cleared ? "Debugger action traces cleared." : "Debugger action trace history was already empty.",
    values: profile.config as unknown as GenericRecord,
    blocks: buildSelfTestBlocks(profile, cleared ? "Action trace history cleared." : "No action trace history to clear.")
  });
}

function selfTest(
  config: unknown,
  sandboxInput: RuntimeSandboxInput,
  context?: PluginExecutionContext
): GenericRecord {
  const profile = buildProfile(config, sandboxInput, "debugger.selftest", context);
  return buildActionResponse({
    source: profile.source,
    message: "Debugger self-test completed.",
    values: {
      current_action: profile.sandbox.currentAction,
      warning_count: profile.warnings.length,
      capability_gap_count: profile.capability_gaps.length,
      action_trace_count: profile.action_traces.length,
      page_path: profile.page_context.full_path || profile.page_context.path || ""
    },
    blocks: buildSelfTestBlocks(profile)
  });
}

function storageInspect(params: GenericRecord, config: unknown, sandboxInput: RuntimeSandboxInput): GenericRecord {
  const values = debuggerState.readStorageFormState(params);
  const inspected = debuggerState.inspectStorageValue(values);
  const profile = buildProfile(config, sandboxInput, "debugger.storage.inspect");
  profile.storage = inspected.summary;
  return buildActionResponse({
    source: profile.source,
    values: inspected.values as unknown as GenericRecord,
    blocks: buildStorageBlocks(profile, inspected.values, "Loaded storage snapshot.")
  });
}

function storageUpsert(params: GenericRecord, config: unknown, sandboxInput: RuntimeSandboxInput): GenericRecord {
  const values = debuggerState.readStorageFormState(params);
  const result = debuggerState.upsertStorageValue(values);
  const profile = buildProfile(config, sandboxInput, "debugger.storage.upsert");
  profile.storage = result.summary;
  if (!result.ok) {
    return buildActionErrorResponse(`Storage Lab: ${result.message}`, {
      source: profile.source,
      message: result.message,
      values: result.values as unknown as GenericRecord,
      blocks: buildStorageBlocks(profile, result.values, `Storage write failed. ${result.message}`)
    });
  }
  return buildActionResponse({
    source: profile.source,
    message: result.message,
    values: result.values as unknown as GenericRecord,
    blocks: buildStorageBlocks(profile, result.values, result.message)
  });
}

function storageDelete(params: GenericRecord, config: unknown, sandboxInput: RuntimeSandboxInput): GenericRecord {
  const values = debuggerState.readStorageFormState(params);
  const result = debuggerState.deleteStorageValue(values);
  const profile = buildProfile(config, sandboxInput, "debugger.storage.delete");
  profile.storage = result.summary;
  if (!result.ok) {
    return buildActionErrorResponse(`Storage Lab: ${result.message}`, {
      source: profile.source,
      message: result.message,
      values: result.values as unknown as GenericRecord,
      blocks: buildStorageBlocks(profile, result.values, `Storage delete failed. ${result.message}`)
    });
  }
  return buildActionResponse({
    source: profile.source,
    message: result.message,
    values: result.values as unknown as GenericRecord,
    blocks: buildStorageBlocks(profile, result.values, result.message)
  });
}

function storageClearLab(config: unknown, sandboxInput: RuntimeSandboxInput): GenericRecord {
  const result = debuggerState.clearLabStorageValues();
  const profile = buildProfile(config, sandboxInput, "debugger.storage.clear_lab");
  profile.storage = result.summary;
  if (!result.ok) {
    return buildActionErrorResponse(`Storage Lab: ${result.message}`, {
      source: profile.source,
      message: result.message,
      values: DEFAULT_STORAGE_FORM_STATE as unknown as GenericRecord,
      blocks: buildStorageBlocks(profile, DEFAULT_STORAGE_FORM_STATE, `Storage cleanup failed. ${result.message}`)
    });
  }
  return buildActionResponse({
    source: profile.source,
    message: result.message,
    values: DEFAULT_STORAGE_FORM_STATE as unknown as GenericRecord,
    blocks: buildStorageBlocks(profile, DEFAULT_STORAGE_FORM_STATE, result.message)
  });
}

function storageSeed(config: unknown, sandboxInput: RuntimeSandboxInput): GenericRecord {
  const result = debuggerState.seedStorageValue();
  const profile = buildProfile(config, sandboxInput, "debugger.storage.seed");
  profile.storage = result.summary;
  if (!result.ok) {
    return buildActionErrorResponse(`Storage Lab: ${result.message}`, {
      source: profile.source,
      message: result.message,
      values: result.values as unknown as GenericRecord,
      blocks: buildStorageBlocks(profile, result.values, `Storage sample seed failed. ${result.message}`)
    });
  }
  return buildActionResponse({
    source: profile.source,
    message: result.message,
    values: result.values as unknown as GenericRecord,
    blocks: buildStorageBlocks(profile, result.values, result.message)
  });
}

function fsRead(params: GenericRecord, config: unknown, sandboxInput: RuntimeSandboxInput): GenericRecord {
  const values = debuggerState.readFileSystemFormState(params);
  const result = debuggerState.inspectFile(values);
  const profile = buildProfile(config, sandboxInput, "debugger.fs.read");
  if (!result.ok) {
    return buildActionErrorResponse(`Filesystem Lab: ${result.message}`, {
      source: profile.source,
      message: result.message,
      values: result.values as unknown as GenericRecord,
      blocks: buildFileBlocks(result.summary, result.values, `Filesystem read failed. ${result.message}`)
    });
  }
  return buildActionResponse({
    source: profile.source,
    message: result.message,
    values: result.values as unknown as GenericRecord,
    blocks: buildFileBlocks(result.summary, result.values, result.message)
  });
}

function fsWrite(params: GenericRecord, config: unknown, sandboxInput: RuntimeSandboxInput): GenericRecord {
  const values = debuggerState.readFileSystemFormState(params);
  const result = debuggerState.upsertFile(values);
  const profile = buildProfile(config, sandboxInput, "debugger.fs.write");
  if (!result.ok) {
    return buildActionErrorResponse(`Filesystem Lab: ${result.message}`, {
      source: profile.source,
      message: result.message,
      values: result.values as unknown as GenericRecord,
      blocks: buildFileBlocks(result.summary, result.values, `Filesystem write failed. ${result.message}`)
    });
  }
  return buildActionResponse({
    source: profile.source,
    message: result.message,
    values: result.values as unknown as GenericRecord,
    blocks: buildFileBlocks(result.summary, result.values, result.message)
  });
}

function fsDelete(params: GenericRecord, config: unknown, sandboxInput: RuntimeSandboxInput): GenericRecord {
  const values = debuggerState.readFileSystemFormState(params);
  const result = debuggerState.deleteFile(values);
  const profile = buildProfile(config, sandboxInput, "debugger.fs.delete");
  if (!result.ok) {
    return buildActionErrorResponse(`Filesystem Lab: ${result.message}`, {
      source: profile.source,
      message: result.message,
      values: result.values as unknown as GenericRecord,
      blocks: buildFileBlocks(result.summary, result.values, `Filesystem delete failed. ${result.message}`)
    });
  }
  return buildActionResponse({
    source: profile.source,
    message: result.message,
    values: result.values as unknown as GenericRecord,
    blocks: buildFileBlocks(result.summary, result.values, result.message)
  });
}

function fsInspect(config: unknown, sandboxInput: RuntimeSandboxInput): GenericRecord {
  const result = debuggerState.inspectFileSystem();
  const profile = buildProfile(config, sandboxInput, "debugger.fs.inspect");
  if (!result.ok) {
    return buildActionErrorResponse(`Filesystem Lab: ${result.message}`, {
      source: profile.source,
      message: result.message,
      values: DEFAULT_FS_FORM_STATE as unknown as GenericRecord,
      blocks: buildFileBlocks(result.summary, DEFAULT_FS_FORM_STATE, `Filesystem inspect failed. ${result.message}`)
    });
  }
  return buildActionResponse({
    source: profile.source,
    message: result.message,
    values: DEFAULT_FS_FORM_STATE as unknown as GenericRecord,
    blocks: buildFileBlocks(result.summary, DEFAULT_FS_FORM_STATE, result.message)
  });
}

function fsSeed(config: unknown, sandboxInput: RuntimeSandboxInput): GenericRecord {
  const profile = buildProfile(config, sandboxInput, "debugger.fs.seed");
  const result = debuggerState.seedFileSystem(profile);
  if (!result.ok) {
    return buildActionErrorResponse(`Filesystem Lab: ${result.message}`, {
      source: profile.source,
      message: result.message,
      values: result.values as unknown as GenericRecord,
      blocks: buildFileBlocks(result.summary, result.values, `Filesystem sample seed failed. ${result.message}`)
    });
  }
  return buildActionResponse({
    source: profile.source,
    message: result.message,
    values: result.values as unknown as GenericRecord,
    blocks: buildFileBlocks(result.summary, result.values, result.message)
  });
}

function networkRequest(params: GenericRecord, config: unknown, sandboxInput: RuntimeSandboxInput): GenericRecord {
  const values = debuggerState.readNetworkFormState(params);
  const result = debuggerState.executeNetworkRequest(values);
  const profile = buildProfile(config, sandboxInput, "debugger.network.request");
  return buildActionResponse({
    source: profile.source,
    message: result.message,
    values: result.values as unknown as GenericRecord,
    blocks: buildNetworkBlocks(profile, result.values, result.response, result.message)
  });
}

function hostRequest(params: GenericRecord, config: unknown, sandboxInput: RuntimeSandboxInput): GenericRecord {
  const values = debuggerState.readHostFormState(params);
  const result = debuggerState.executeHostRequest(values);
  const profile = buildProfile(config, sandboxInput, "debugger.host.request");
  if (!result.ok) {
    return buildActionErrorResponse(`Host Lab: ${result.message}`, {
      source: profile.source,
      message: result.message,
      values: result.values as unknown as GenericRecord,
      blocks: buildHostBlocks(
        profile,
        result.values,
        result.action,
        result.request_payload,
        result.response,
        `Host request failed. ${result.message}`
      )
    });
  }
  return buildActionResponse({
    source: profile.source,
    message: result.message,
    values: result.values as unknown as GenericRecord,
    blocks: buildHostBlocks(
      profile,
      result.values,
      result.action,
      result.request_payload,
      result.response,
      result.message
    )
  });
}

async function workerRoundTrip(
  params: GenericRecord,
  context: PluginExecutionContext,
  config: unknown,
  sandboxInput: RuntimeSandboxInput
): Promise<GenericRecord> {
  const values = debuggerState.readWorkerFormState(params);
  const profile = buildProfile(config, sandboxInput, "debugger.worker.roundtrip", context);
  const workerCtor = (globalThis as Record<string, unknown>).Worker;
  if (typeof workerCtor !== "function") {
    return buildActionErrorResponse("Worker Lab: Worker global is unavailable in current runtime.", {
      source: profile.source,
      message: "Worker global is unavailable in current runtime.",
      values: values as unknown as GenericRecord,
      blocks: buildWorkerBlocks(profile, values, null, "Worker roundtrip failed. Worker global is unavailable in current runtime.")
    });
  }

  let worker: Record<string, unknown> | null = null;
  let roundtrip: Record<string, unknown> | null = null;

  try {
    worker = new (workerCtor as new (script: string) => Record<string, unknown>)(values.worker_script);
    const support = {
      request: typeof worker.request === "function",
      postMessage: typeof worker.postMessage === "function",
      terminate: typeof worker.terminate === "function"
    };
    if (!support.request || !support.postMessage || !support.terminate) {
      return buildActionErrorResponse("Worker Lab: Worker bridge is incomplete for current runtime.", {
        source: profile.source,
        message: "Worker bridge is incomplete for current runtime.",
        values: values as unknown as GenericRecord,
        blocks: buildWorkerBlocks(profile, values, {
          support,
          worker_id: asString(worker.id),
          worker_script_path: asString(worker.scriptPath)
        }, "Worker roundtrip failed. worker.request(), worker.postMessage(), and worker.terminate() must all be available.")
      });
    }

    emitWorkspaceTrace("debugger.worker.roundtrip", "Worker roundtrip started.", {
      worker_script: values.worker_script
    });

    const first = await (worker.request as (payload: Record<string, unknown>) => Promise<unknown>)({
      mode: "request",
      value: values.worker_request_value
    });
    const second = await (worker.request as (payload: Record<string, unknown>) => Promise<unknown>)({
      mode: "request",
      value: values.worker_second_value
    });
    const messageEvent = await new Promise<Record<string, unknown>>((resolve, reject) => {
      const handleMessage = (event: unknown) => {
        resolve(asRecord(event));
      };
      const handleError = (event: unknown) => {
        const errorEvent = asRecord(event);
        reject(new Error(asString(errorEvent.error) || "Worker postMessage dispatch failed."));
      };
      worker!.onmessage = handleMessage;
      worker!.onerror = handleError;
      (worker!.postMessage as (payload: Record<string, unknown>) => void)({
        mode: "postMessage",
        value: values.worker_message_value
      });
    });

    const terminatedBefore = Boolean(worker.terminated);
    (worker.terminate as () => void)();
    const terminatedAfter = Boolean(worker.terminated);
    roundtrip = {
      support,
      worker_id: asString(worker.id),
      worker_script_path: asString(worker.scriptPath),
      terminated_before: terminatedBefore,
      terminated_after: terminatedAfter,
      requests: {
        first: first as Record<string, unknown>,
        second: second as Record<string, unknown>
      },
      message_event: messageEvent
    };

    emitWorkspaceTrace("debugger.worker.roundtrip", "Worker roundtrip completed.", {
      worker_id: asString(worker.id),
      worker_script: values.worker_script
    });

    return buildActionResponse({
      source: profile.source,
      message: "Worker roundtrip completed.",
      values: {
        ...values,
        worker_id: roundtrip.worker_id,
        worker_script_path: roundtrip.worker_script_path,
        terminated: roundtrip.terminated_after
      },
      blocks: buildWorkerBlocks(profile, values, roundtrip, "Worker roundtrip completed.")
    });
  } catch (error) {
    const message = asString((error as Error).message || error) || "Worker roundtrip failed.";
    return buildActionErrorResponse(`Worker Lab: ${message}`, {
      source: profile.source,
      message,
      values: values as unknown as GenericRecord,
      blocks: buildWorkerBlocks(profile, values, roundtrip, `Worker roundtrip failed. ${message}`)
    });
  } finally {
    if (worker && typeof worker.terminate === "function" && !worker.terminated) {
      try {
        (worker.terminate as () => void)();
      } catch {
        // Ignore cleanup failures.
      }
    }
  }
}

function executeHookAction(
  params: GenericRecord,
  context: PluginExecutionContext,
  config: unknown,
  sandboxInput: RuntimeSandboxInput
): GenericRecord {
  const hook = normalizeHookName(params.hook);
  if (!hook) {
    return {
      success: false,
      error: "hook.execute requires params.hook"
    };
  }
  const payload = safeParseJSONObject(params.payload);
  const profile = buildProfile(config, sandboxInput, "hook.execute", context);
  const result = executeHookInternal(hook, payload, context, profile, { persistEvent: true });
  emitWorkspaceTrace("hook.execute", `Processed hook ${hook}.`, {
    hook,
    blocked: result.blocked ? "true" : "false",
    slot: asString(payload.slot),
    path: asString(payload.path),
    area: asString(payload.area)
  });
  return result as unknown as GenericRecord;
}

function executeEchoAction(
  params: GenericRecord,
  context: PluginExecutionContext,
  config: unknown,
  sandboxInput: RuntimeSandboxInput,
  actionName = "debugger.echo"
): GenericRecord {
  const profile = buildProfile(config, sandboxInput, actionName, context);
  const message = asString(params.echo_message) || "Plugin page executed its own exec API.";
  const rawJSON = asString(params.echo_json);
  const parsedJSON = rawJSON ? safeParseJSONObject(rawJSON) : {};
  const echoPayload =
    rawJSON && Object.keys(parsedJSON).length === 0
      ? rawJSON
      : Object.keys(parsedJSON).length > 0
        ? parsedJSON
        : {};
  const streamRequested = actionName === "debugger.echo.stream";
  const transport = streamRequested
    ? "stream endpoint requested"
    : "standard execute endpoint";
  const delivery = streamRequested
    ? "incremental js_worker chunks are emitted through the worker stream bridge."
    : "single response payload";

  emitWorkspaceTrace(actionName, `Debugger echo executed via ${transport}.`, {
    stream: streamRequested ? "true" : "false",
    request_path: asString(context.metadata?.request_path),
    plugin_page_path: asString(context.metadata?.plugin_page_path),
    session_id: asString(context.session_id)
  });

  return buildActionResponse({
    source: profile.source,
    message: streamRequested
      ? "Executed through host stream endpoint with real incremental js_worker chunks."
      : "Plugin self-exec completed.",
    values: {
      echo_message: message,
      echo_json: rawJSON
    },
    blocks: [
      {
        type: "alert",
        title: "Exec API Echo",
        content: message,
        data: {
          variant: "success"
        }
      },
      {
        type: "key_value",
        title: "Execution Context",
        data: {
          items: [
            { key: "action", label: "Action", value: actionName },
            { key: "transport", label: "Requested Transport", value: transport },
            { key: "delivery", label: "Runtime Delivery", value: delivery },
            { key: "declared_storage_access", label: "Declared Storage Access", value: profile.sandbox.declaredStorageAccessMode },
            { key: "observed_storage_access", label: "Observed Storage Access", value: profile.sandbox.storageAccessMode },
            { key: "user_id", label: "User ID", value: context.user_id ?? "-" },
            { key: "order_id", label: "Order ID", value: context.order_id ?? "-" },
            { key: "session_id", label: "Session ID", value: context.session_id || "-" },
            { key: "request_path", label: "Request Path", value: context.metadata?.request_path || "-" },
            { key: "plugin_page_path", label: "Plugin Page Path", value: context.metadata?.plugin_page_path || "-" },
            { key: "plugin_page_full_path", label: "Plugin Page Full Path", value: context.metadata?.plugin_page_full_path || "-" },
            { key: "plugin_page_query_string", label: "Plugin Page Query String", value: context.metadata?.plugin_page_query_string || "-" },
            { key: "plugin_page_query_params", label: "Plugin Page Query Params", value: context.metadata?.plugin_page_query_params || "-" },
            { key: "plugin_page_route_params", label: "Plugin Page Route Params", value: context.metadata?.plugin_page_route_params || "-" },
            { key: "bootstrap_area", label: "Bootstrap Area", value: context.metadata?.bootstrap_area || "-" },
            { key: "plugin_execution_id", label: "Host Task ID", value: context.metadata?.plugin_execution_id || "-" },
            { key: "plugin_execution_status", label: "Host Task Status", value: context.metadata?.plugin_execution_status || "-" },
            { key: "plugin_execution_runtime", label: "Host Runtime", value: context.metadata?.plugin_execution_runtime || "-" }
          ]
        }
      },
      {
        type: "json_view",
        title: "Context Metadata",
        data: {
          value: context.metadata || {},
          summary: "Raw plugin page metadata injected by the host execute route.",
          collapsible: true,
          collapsed: true,
          preview_lines: 8,
          max_height: 480
        }
      },
      {
        type: "json_view",
        title: "Sandbox Snapshot",
        data: {
          value: profile.sandbox,
          summary: "Resolved sandbox profile at the moment this exec action ran.",
          collapsible: true,
          collapsed: true,
          preview_lines: 8,
          max_height: 480
        }
      },
      {
        type: "json_view",
        title: "Echo Params",
        data: {
          value: params,
          summary: "Raw params received by the debugger echo action.",
          collapsible: true,
          collapsed: true,
          preview_lines: 8,
          max_height: 480
        }
      },
      {
        type: "json_view",
        title: "Echo JSON Payload",
        data: {
          value: echoPayload,
          summary: "Parsed JSON payload extracted from echo_json, if provided.",
          collapsible: true,
          collapsed: true,
          preview_lines: 8,
          max_height: 480
        }
      }
    ]
  });
}

function executeEchoStreamAction(
  params: GenericRecord,
  context: PluginExecutionContext,
  config: unknown,
  sandboxInput: RuntimeSandboxInput,
  stream: RuntimeStreamWriter
): GenericRecord {
  if (stream && typeof stream.progress === "function") {
    stream.progress("preparing", 15, { phase: "prepare" });
  }
  if (stream && typeof stream.write === "function") {
    stream.write(
      {
        status: "collecting-context",
        progress: 55,
        action: "debugger.echo.stream",
        metadata_keys: Object.keys(context.metadata || {})
      },
      { phase: "context" }
    );
  }
  if (stream && typeof stream.progress === "function") {
    stream.progress("finalizing", 85, { phase: "finalize" });
  }
  return executeEchoAction(params, context, config, sandboxInput, "debugger.echo.stream");
}

export function execute(
  action: "debugger.worker.roundtrip",
  params: unknown,
  context: unknown,
  config: unknown,
  sandboxInput: RuntimeSandboxInput
): Promise<GenericRecord>;
export function execute(
  action: unknown,
  params: unknown,
  context: unknown,
  config: unknown,
  sandboxInput: RuntimeSandboxInput
): any;
export function execute(
  action: unknown,
  params: unknown,
  context: unknown,
  config: unknown,
  sandboxInput: RuntimeSandboxInput
): GenericRecord | Promise<GenericRecord> {
  const normalizedAction = asString(action);
  const paramsRecord = asRecord(params);
  const contextRecord = asRecord(context) as PluginExecutionContext;
  let result: GenericRecord | Promise<GenericRecord>;

  switch (normalizedAction) {
    case "debugger.profile": {
      const profile = buildProfile(config, sandboxInput, "debugger.profile", contextRecord);
      result = buildActionResponse({
        source: profile.source,
        blocks: buildDebuggerDashboardBlocks("admin", profile)
      });
      break;
    }
    case "debugger.config.get":
      result = configGet(config, sandboxInput);
      break;
    case "debugger.config.set":
      result = configSet(paramsRecord, config, sandboxInput);
      break;
    case "debugger.config.reset":
      result = configReset(config, sandboxInput);
      break;
    case "debugger.events.clear":
      result = clearEvents(config, sandboxInput);
      break;
    case "debugger.action_traces.clear":
      result = clearActionTraceHistory(config, sandboxInput);
      break;
    case "debugger.selftest":
      result = selfTest(config, sandboxInput, contextRecord);
      break;
    case "debugger.simulate.hook":
      result = simulateHook(paramsRecord, config, sandboxInput, contextRecord);
      break;
    case "debugger.storage.inspect":
      result = storageInspect(paramsRecord, config, sandboxInput);
      break;
    case "debugger.storage.upsert":
      result = storageUpsert(paramsRecord, config, sandboxInput);
      break;
    case "debugger.storage.delete":
      result = storageDelete(paramsRecord, config, sandboxInput);
      break;
    case "debugger.storage.clear_lab":
      result = storageClearLab(config, sandboxInput);
      break;
    case "debugger.storage.seed":
      result = storageSeed(config, sandboxInput);
      break;
    case "debugger.fs.read":
      result = fsRead(paramsRecord, config, sandboxInput);
      break;
    case "debugger.fs.write":
      result = fsWrite(paramsRecord, config, sandboxInput);
      break;
    case "debugger.fs.delete":
      result = fsDelete(paramsRecord, config, sandboxInput);
      break;
    case "debugger.fs.inspect":
      result = fsInspect(config, sandboxInput);
      break;
    case "debugger.fs.seed":
      result = fsSeed(config, sandboxInput);
      break;
    case "debugger.network.request":
      result = networkRequest(paramsRecord, config, sandboxInput);
      break;
    case "debugger.host.request":
      result = hostRequest(paramsRecord, config, sandboxInput);
      break;
    case "debugger.worker.roundtrip":
      result = workerRoundTrip(paramsRecord, contextRecord, config, sandboxInput);
      break;
    case "debugger.echo":
      result = executeEchoAction(paramsRecord, contextRecord, config, sandboxInput, "debugger.echo");
      break;
    case "debugger.echo.stream":
      result = executeEchoAction(paramsRecord, contextRecord, config, sandboxInput, "debugger.echo.stream");
      break;
    case "hook.execute":
      result = executeHookAction(paramsRecord, contextRecord, config, sandboxInput);
      break;
    default:
      result = {
        success: true,
        message: `${PLUGIN_IDENTITY} alive`
      };
      break;
  }

  if (isPromiseLike(result)) {
    return result
      .then((resolved) => {
        recordActionTrace(normalizedAction, paramsRecord, contextRecord, sandboxInput, resolved);
        return resolved;
      })
      .catch((error) => {
        const failure = buildActionErrorResponse(asString((error as Error).message || error) || "Action failed.");
        recordActionTrace(normalizedAction, paramsRecord, contextRecord, sandboxInput, failure);
        return failure;
      });
  }
  recordActionTrace(normalizedAction, paramsRecord, contextRecord, sandboxInput, result);
  return result;
}

export function executeStream(
  action: unknown,
  params: unknown,
  context: unknown,
  config: unknown,
  sandboxInput: RuntimeSandboxInput,
  stream: RuntimeStreamWriter
): GenericRecord {
  const normalizedAction = asString(action);
  const paramsRecord = asRecord(params);
  const contextRecord = asRecord(context) as PluginExecutionContext;
  let result: GenericRecord;

  switch (normalizedAction) {
    case "debugger.echo.stream":
      result = executeEchoStreamAction(paramsRecord, contextRecord, config, sandboxInput, stream);
      break;
    default:
      return execute(action, params, context, config, sandboxInput);
  }

  recordActionTrace(normalizedAction, paramsRecord, contextRecord, sandboxInput, result);
  return result;
}

function executeWorkspaceCatalogCommand(
  command: PluginWorkspaceCommandContext,
  context: PluginExecutionContext,
  config: unknown,
  sandboxInput: RuntimeSandboxInput,
  workspace: PluginWorkspaceAPI
): GenericRecord {
  const profile = buildProfile(config, sandboxInput, `workspace:${command.name}`, context);
  workspace.info(`Workspace command ${command.name} started.`, {
    command: command.name,
    argv: command.argv.join(" ")
  });
  workspace.info(`Current runtime action: ${profile.sandbox.currentAction || "workspace.command.execute"}`, {
    command: command.name
  });
  workspace.info(`Recent hook events retained: ${String(profile.recent_events.length)}`, {
    command: command.name
  });
  const result = buildActionResponse({
    source: "workspace.command",
    message: `Workspace command ${command.name} completed.`,
    values: {
      command: command.name,
      argv: command.argv.join(" "),
      user_id: context.user_id ?? null,
      session_id: context.session_id || ""
    }
  });
  recordActionTrace(`workspace:${command.entry}`, { argv: command.argv, raw: command.raw }, context, sandboxInput, result);
  return result;
}

function executeWorkspacePromptCommand(
  command: PluginWorkspaceCommandContext,
  context: PluginExecutionContext,
  config: unknown,
  sandboxInput: RuntimeSandboxInput,
  workspace: PluginWorkspaceAPI
): GenericRecord {
  const profile = buildProfile(config, sandboxInput, `workspace:${command.name}`, context);
  workspace.info(`Workspace input demo started for ${command.name}.`, {
    command: command.name,
    interactive: "true"
  });
  let inputValue = "";
  try {
    inputValue = workspace.readLine("debugger> ", { echo: true });
  } catch (error) {
    workspace.error(`Workspace input demo failed: ${String((error as Error).message || error)}`, {
      command: command.name
    });
    const failure = buildActionErrorResponse(String((error as Error).message || error), {
      source: "workspace.command",
      message: "Workspace input demo failed.",
      values: {
        command: command.name
      }
    });
    recordActionTrace(`workspace:${command.entry}`, { argv: command.argv, raw: command.raw }, context, sandboxInput, failure);
    return failure;
  }

  workspace.info(`Workspace input received: ${inputValue}`, {
    command: command.name,
    user_id: String(context.user_id ?? "")
  });
  const result = buildActionResponse({
    source: "workspace.command",
    message: `Workspace input captured: ${inputValue}`,
    values: {
      command: command.name,
      input: inputValue,
      recent_events: profile.recent_events.length
    }
  });
  recordActionTrace(`workspace:${command.entry}`, { argv: command.argv, raw: command.raw }, context, sandboxInput, result);
  return result;
}

function executeWorkspaceContextCommand(
  command: PluginWorkspaceCommandContext,
  context: PluginExecutionContext,
  config: unknown,
  sandboxInput: RuntimeSandboxInput,
  workspace: PluginWorkspaceAPI
): GenericRecord {
  const profile = buildProfile(config, sandboxInput, `workspace:${command.name}`, context);
  const pagePath =
    asString(context.metadata?.plugin_page_full_path) ||
    asString(context.metadata?.plugin_page_path) ||
    asString(context.metadata?.request_path) ||
    "";

  workspace.info(`Workspace context demo started for ${command.name}.`, {
    command: command.name,
    raw: command.raw
  });
  workspace.info(`Expanded argv: ${command.argv.join(" | ") || "(empty)"}`, {
    command: command.name
  });
  if (pagePath) {
    workspace.info(`Current page path: ${pagePath}`, {
      command: command.name
    });
  }
  if (command.argv.length === 0) {
    workspace.warn("No argv was provided. Try debugger/context $PLUGIN_NAME $WORKSPACE_STATUS $TASK_ID.", {
      command: command.name
    });
  }

  const result = buildActionResponse({
    source: "workspace.command",
    message: `Workspace context summary generated for ${command.name}.`,
    values: {
      command: command.name,
      entry: command.entry,
      raw: command.raw,
      argv: command.argv,
      user_id: context.user_id ?? null,
      order_id: context.order_id ?? null,
      session_id: context.session_id || "",
      current_action: profile.sandbox.currentAction || "workspace.command.execute",
      page_path: pagePath,
      recent_events: profile.recent_events.length,
      plugin_workspace_enabled: profile.runtime.pluginWorkspaceEnabled
    }
  });
  recordActionTrace(`workspace:${command.entry}`, { argv: command.argv, raw: command.raw }, context, sandboxInput, result);
  return result;
}

function executeWorkspaceReportCommand(
  command: PluginWorkspaceCommandContext,
  context: PluginExecutionContext,
  config: unknown,
  sandboxInput: RuntimeSandboxInput,
  workspace: PluginWorkspaceAPI
): GenericRecord {
  const profile = buildProfile(config, sandboxInput, `workspace:${command.name}`, context);
  workspace.info(`${PLUGIN_DISPLAY_NAME} runtime report`, { command: command.name });
  workspace.write(`hooks=${profile.enabled_hooks.length} recent_events=${profile.recent_events.length} action_traces=${profile.action_traces.length}\n`);
  workspace.write(`warnings=${profile.warnings.length} capability_gaps=${profile.capability_gaps.length} granted_permissions=${profile.sandbox.grantedPermissions.length}\n`);
  workspace.write(`workspace=${profile.runtime.pluginWorkspaceEnabled} fs=${profile.runtime.pluginFSEnabled} http=${profile.runtime.pluginHTTPEnabled} host=${profile.runtime.pluginHostEnabled}\n`);
  workspace.write(`runtime_globals=${profile.runtime.runtimeGlobalKeys.join(",") || "(none)"} plugin_keys=${profile.runtime.pluginGlobalKeys.join(",") || "(none)"}\n`);
  workspace.write(`secret=${profile.secret.present}/${profile.secret.enabled} webhook=${profile.webhook.present}/${profile.webhook.enabled}\n`);
  if (profile.page_context.full_path || profile.page_context.path) {
    workspace.write(`page=${profile.page_context.full_path || profile.page_context.path}\n`);
  }
  if (profile.warnings.length > 0) {
    workspace.warn(`Warnings: ${profile.warnings.join(" | ")}`, { command: command.name });
  }
  const result = buildActionResponse({
    source: "workspace.command",
    message: `Workspace runtime report generated for ${command.name}.`,
    values: {
      command: command.name,
      warning_count: profile.warnings.length,
      capability_gap_count: profile.capability_gaps.length,
      action_trace_count: profile.action_traces.length,
      full_path: profile.page_context.full_path || profile.page_context.path
    }
  });
  recordActionTrace(`workspace:${command.entry}`, { argv: command.argv, raw: command.raw }, context, sandboxInput, result);
  return result;
}

function executeWorkspaceSelfTestCommand(
  command: PluginWorkspaceCommandContext,
  context: PluginExecutionContext,
  config: unknown,
  sandboxInput: RuntimeSandboxInput,
  workspace: PluginWorkspaceAPI
): GenericRecord {
  const profile = buildProfile(config, sandboxInput, `workspace:${command.name}`, context);
  workspace.info(`Running debugger self-test for ${command.name}.`, { command: command.name });
  workspace.write(`runtime_keys=${profile.runtime.runtimeGlobalKeys.join(",") || "(none)"}\n`);
  workspace.write(`plugin_keys=${profile.runtime.pluginGlobalKeys.join(",") || "(none)"}\n`);
  workspace.write(`granted_permissions=${profile.sandbox.grantedPermissions.length} requested_permissions=${profile.sandbox.requestedPermissions.length}\n`);
  workspace.write(`missing_permissions=${profile.capability_gaps.join(",") || "(none)"}\n`);
  workspace.write(`secret_keys=${profile.secret.keys.join(",") || "(none)"} sample_secret=${profile.secret.sample_present}\n`);
  if (profile.webhook.present) {
    workspace.write(`webhook=${profile.webhook.method} ${profile.webhook.path} key=${profile.webhook.key}\n`);
  }
  if (profile.warnings.length > 0) {
    profile.warnings.forEach((warning) => {
      workspace.warn(warning, { command: command.name });
    });
  } else {
    workspace.info("Self-test found no runtime warnings.", { command: command.name });
  }
  const result = buildActionResponse({
    source: "workspace.command",
    message: `Workspace self-test finished for ${command.name}.`,
    values: {
      command: command.name,
      warning_count: profile.warnings.length,
      missing_permissions: profile.capability_gaps,
      runtime_keys: profile.runtime.runtimeGlobalKeys,
      plugin_keys: profile.runtime.pluginGlobalKeys
    }
  });
  recordActionTrace(`workspace:${command.entry}`, { argv: command.argv, raw: command.raw }, context, sandboxInput, result);
  return result;
}

function executeWorkspaceTraceCommand(
  command: PluginWorkspaceCommandContext,
  context: PluginExecutionContext,
  _config: unknown,
  sandboxInput: RuntimeSandboxInput,
  workspace: PluginWorkspaceAPI
): GenericRecord {
  const traces = debuggerState.readActionTraces().slice(0, 12);
  workspace.info(`Recent debugger actions for ${command.name}.`, { command: command.name });
  if (traces.length === 0) {
    workspace.warn("No debugger action traces recorded yet.", { command: command.name });
  } else {
    traces.forEach((trace) => {
      workspace.write(
        `[${trace.ts}] ${trace.ok ? "OK" : "ERR"} ${trace.category} ${trace.action} :: ${trace.message}\n`
      );
    });
  }
  const result = buildActionResponse({
    source: "workspace.command",
    message: `Workspace trace dump generated for ${command.name}.`,
    values: {
      command: command.name,
      trace_count: traces.length,
      latest_action: traces[0]?.action || ""
    }
  });
  recordActionTrace(`workspace:${command.entry}`, { argv: command.argv, raw: command.raw }, context, sandboxInput, result);
  return result;
}

export const workspaceHandlers = defineWorkspaceCommands({
  "debugger.catalog": defineWorkspaceCommand(
    {
      name: "debugger/catalog",
      title: "Workspace Catalog",
      description: "Write a concise runtime and capability summary into the plugin workspace buffer.",
      interactive: false
    },
    executeWorkspaceCatalogCommand
  ),
  "debugger.prompt": defineWorkspaceCommand(
    {
      name: "debugger/prompt",
      title: "Workspace Prompt Demo",
      description: "Consume preloaded workspace input through Plugin.workspace.readLine() and echo the captured value.",
      interactive: true
    },
    executeWorkspacePromptCommand
  ),
  "debugger.context": defineWorkspaceCommand(
    {
      name: "debugger/context",
      title: "Workspace Context Demo",
      description: "Echo the current workspace raw command, expanded argv, and page context. Useful for validating shell variable expansion such as $PLUGIN_NAME or $WORKSPACE_STATUS.",
      interactive: false
    },
    executeWorkspaceContextCommand
  ),
  "debugger.report": defineWorkspaceCommand(
    {
      name: "debugger/report",
      title: "Workspace Runtime Report",
      description: "Write a concise runtime, permission, page-context, secret, and webhook summary into the workspace buffer.",
      interactive: false
    },
    executeWorkspaceReportCommand
  ),
  "debugger.selftest": defineWorkspaceCommand(
    {
      name: "debugger/selftest",
      title: "Workspace Self-Test",
      description: "Run the debugger self-test and emit warnings, missing permissions, and runtime bridge status into the workspace buffer.",
      interactive: false
    },
    executeWorkspaceSelfTestCommand
  ),
  "debugger.traces": defineWorkspaceCommand(
    {
      name: "debugger/traces",
      title: "Workspace Trace Dump",
      description: "Print recent debugger execute/workspace action traces into the workspace buffer for quick terminal-style inspection.",
      interactive: false
    },
    executeWorkspaceTraceCommand
  )
});
