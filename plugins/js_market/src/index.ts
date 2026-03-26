import {
  asBool,
  asInteger,
  asRecord,
  asString,
  getPluginAuthBranding,
  buildPluginAlertBlock,
  buildPluginBadgeListBlock,
  buildPluginJSONViewBlock,
  buildPluginKeyValueBlock,
  buildPluginLinkListBlock,
  buildPluginStatsGridBlock,
  buildPluginTableBlock,
  buildPluginTextBlock,
  definePlugin,
  defineWorkspaceCommand,
  defineWorkspaceCommands,
  errorResult,
  getPluginEmailTemplate,
  getPluginInvoiceTemplate,
  getPluginLandingPage,
  getPluginMarket,
  getPluginPageRulePack,
  getPluginWorkspace,
  prettyJSON,
  resolvePluginPageContext,
  successResult,
  type PluginActionData,
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
  ADMIN_PLUGIN_PAGE_PATH,
  PLUGIN_DISPLAY_NAME,
  PLUGIN_IDENTITY,
  SUPPORTED_MARKET_KINDS,
  TEMPLATE_MARKET_KINDS,
  type SupportedMarketKind,
  type TemplateMarketKind,
} from "./lib/constants";
import {
  buildBootstrapExtensions,
  buildDefaultMarketConsoleState,
  type MarketConsoleState
} from "./lib/frontend";
import { getTrustedWorkspaceAsset } from "./lib/trusted-workspace-assets";
import {
  marketMessage,
  localizeMarketExecuteResult,
  resolveMarketLocale
} from "./lib/i18n";

const t = marketMessage;

const MARKET_WORKSPACE_MIRRORED_ACTIONS = new Set<string>([
  "market.source.detail",
  "market.catalog.query",
  "market.artifact.detail",
  "market.release.detail",
  "market.release.preview",
  "market.install.execute",
  "market.install.task.get",
  "market.install.task.list",
  "market.install.history.list",
  "market.install.rollback"
]);

function normalizeMarketKind(value: unknown, fallback: SupportedMarketKind = "plugin_package"): SupportedMarketKind {
  const normalized = asString(value).toLowerCase();
  if ((SUPPORTED_MARKET_KINDS as readonly string[]).includes(normalized)) {
    return normalized as SupportedMarketKind;
  }
  return fallback;
}

function safeArray(value: unknown): unknown[] {
  return Array.isArray(value) ? value : [];
}

function safeObjectArray(value: unknown): UnknownMap[] {
  return safeArray(value).map((item) => asRecord(item)).filter((item) => Object.keys(item).length > 0);
}

function parseJSONArray(value: unknown): unknown[] {
  if (Array.isArray(value)) {
    return value;
  }
  if (typeof value !== "string") {
    return [];
  }
  const trimmed = value.trim();
  if (!trimmed) {
    return [];
  }
  try {
    const parsed = JSON.parse(trimmed);
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}

function parseJSONRecord(value: unknown): UnknownMap {
  if (value && typeof value === "object" && !Array.isArray(value)) {
    return asRecord(value);
  }
  if (typeof value !== "string") {
    return {};
  }
  const trimmed = value.trim();
  if (!trimmed) {
    return {};
  }
  try {
    return asRecord(JSON.parse(trimmed));
  } catch {
    return {};
  }
}

function scalarText(value: unknown): string {
  if (typeof value === "string") {
    return value.trim();
  }
  if (typeof value === "number" && Number.isFinite(value)) {
    return String(value);
  }
  if (typeof value === "boolean") {
    return value ? "true" : "false";
  }
  return "";
}

function resolveVersionText(value: unknown): string {
  const direct = scalarText(value);
  if (direct) {
    return direct;
  }
  const mapped = asRecord(value);
  if (Object.keys(mapped).length === 0) {
    return "";
  }
  return (
    scalarText(mapped.version) ||
    scalarText(mapped.market_artifact_version) ||
    scalarText(mapped.latest_version) ||
    scalarText(mapped.tag)
  );
}

function normalizeStringArray(value: unknown): string[] {
  const seen = new Set<string>();
  const output: string[] = [];
  parseJSONArray(value).forEach((item) => {
    const normalized = asString(item).toLowerCase();
    if (!normalized || seen.has(normalized)) {
      return;
    }
    seen.add(normalized);
    output.push(normalized);
  });
  return output;
}

function normalizeDisplayStringArray(value: unknown): string[] {
  const seen = new Set<string>();
  const output: string[] = [];
  parseJSONArray(value).forEach((item) => {
    const normalized = asString(item).trim();
    if (!normalized) {
      return;
    }
    const dedupeKey = normalized.toLowerCase();
    if (seen.has(dedupeKey)) {
      return;
    }
    seen.add(dedupeKey);
    output.push(normalized);
  });
  return output;
}

function uniqueDisplayStrings(values: unknown[]): string[] {
  const seen = new Set<string>();
  const output: string[] = [];
  values.forEach((item) => {
    const normalized = asString(item).trim();
    if (!normalized) {
      return;
    }
    const dedupeKey = normalized.toLowerCase();
    if (seen.has(dedupeKey)) {
      return;
    }
    seen.add(dedupeKey);
    output.push(normalized);
  });
  return output;
}

function stringifyStringArray(values: string[]): string {
  return values.length === 0 ? "[]" : JSON.stringify(values, null, 2);
}

function normalizeBooleanString(value: unknown, fallback = false): boolean {
  return asBool(value, fallback);
}

function formatBooleanState(value: unknown, truthy = "Yes", falsy = "No"): string {
  return asBool(value) ? truthy : falsy;
}

function countMatching<T>(items: T[], predicate: (item: T) => boolean): number {
  let total = 0;
  items.forEach((item) => {
    if (predicate(item)) {
      total += 1;
    }
  });
  return total;
}

function prettyJSONArray(value: unknown): unknown[] {
  if (Array.isArray(value)) {
    return value;
  }
  if (typeof value === "string") {
    try {
      const parsed = JSON.parse(value);
      return Array.isArray(parsed) ? parsed : [];
    } catch {
      return [];
    }
  }
  return [];
}

function errorToString(error: unknown): string {
  if (error instanceof Error && error.message) {
    return error.message;
  }
  return asString(error) || "unknown error";
}

function truncateText(value: unknown, maxLength = 240): string {
  const text = asString(value);
  if (!text) {
    return "";
  }
  if (text.length <= maxLength) {
    return text;
  }
  return `${text.slice(0, Math.max(0, maxLength - 3))}...`;
}

function resolveMarketWorkspace(): PluginWorkspaceAPI | null {
  const workspace = getPluginWorkspace();
  if (!workspace || workspace.enabled !== true) {
    return null;
  }
  return workspace;
}

function isSupportedMarketKindToken(value: unknown): value is SupportedMarketKind {
  const normalized = asString(value).toLowerCase();
  return (SUPPORTED_MARKET_KINDS as readonly string[]).includes(normalized);
}

function compactMarketWorkspaceLines(lines: string[], limit = 18): string[] {
  const output: string[] = [];
  lines.forEach((line) => {
    const normalized = String(line || "").trim();
    if (!normalized) {
      return;
    }
    if (output.length > 0 && output[output.length - 1] === normalized) {
      return;
    }
    output.push(normalized);
  });
  if (output.length <= limit) {
    return output;
  }
  return [...output.slice(0, limit), `... ${output.length - limit} more line(s)`];
}

function stringifyMarketWorkspaceValue(value: unknown): string {
  if (value === null || value === undefined) {
    return "-";
  }
  if (typeof value === "string") {
    return value.trim() || "-";
  }
  if (typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  if (Array.isArray(value)) {
    const items = value
      .map((item) => stringifyMarketWorkspaceValue(item))
      .filter((item) => item && item !== "-");
    if (items.length === 0) {
      return "[]";
    }
    return truncateText(items.join(", "), 160);
  }
  return truncateText(prettyJSON(value), 160) || "-";
}

function buildMarketWorkspaceMetadata(
  action: string,
  params: UnknownMap,
  extras: UnknownMap = {}
): UnknownMap {
  const metadata: UnknownMap = {
    source: "market.workspace",
    action
  };
  const sourceID = asString(params.source_id);
  const kind = asString(params.kind);
  const channel = asString(params.channel);
  const name = asString(params.name);
  const version = asString(params.version);
  const taskID = asString(params.task_id);
  if (sourceID) {
    metadata.source_id = sourceID;
  }
  if (kind) {
    metadata.kind = kind;
  }
  if (channel) {
    metadata.channel = channel;
  }
  if (name) {
    metadata.name = name;
  }
  if (version) {
    metadata.version = version;
  }
  if (taskID) {
    metadata.task_id = taskID;
  }
  return {
    ...metadata,
    ...extras
  };
}

function formatMarketWorkspaceStateSummary(candidate: unknown): string {
  const state = asRecord(candidate);
  const parts: string[] = [];
  const sourceID = asString(state.source_id);
  const kind = asString(state.kind);
  const channel = asString(state.channel);
  const query = asString(state.q);
  const name = asString(state.name);
  const version = asString(state.version);
  const taskID = asString(state.task_id);
  if (sourceID) {
    parts.push(`source=${sourceID}`);
  }
  if (kind) {
    parts.push(`kind=${kind}`);
  }
  if (channel) {
    parts.push(`channel=${channel}`);
  }
  if (name) {
    parts.push(`name=${name}`);
  }
  if (version) {
    parts.push(`version=${version}`);
  }
  if (query) {
    parts.push(`q=${query}`);
  }
  if (taskID) {
    parts.push(`task=${taskID}`);
  }
  return parts.join(" ");
}

function resolveMarketResultData(result: PluginExecuteResult): UnknownMap {
  return asRecord(result.data);
}

function resolveMarketResultMessage(result: PluginExecuteResult, fallback: string): string {
  const data = resolveMarketResultData(result);
  const directMessage = asString(asRecord(result).message);
  return directMessage || asString(data.message) || fallback;
}

function renderMarketWorkspaceBlock(block: PluginPageBlock): string[] {
  const type = asString(block.type).toLowerCase();
  const title = asString(block.title);
  const content = asString(block.content);
  const data = asRecord(block.data);
  const lines: string[] = [];

  switch (type) {
    case "alert":
    case "text":
      if (title && content) {
        lines.push(`${title}: ${content}`);
      } else if (title) {
        lines.push(title);
      } else if (content) {
        lines.push(content);
      }
      break;
    case "key_value":
    case "stats_grid": {
      const items = safeObjectArray(data.items);
      if (title) {
        lines.push(`${title}:`);
      }
      items.slice(0, 6).forEach((item) => {
        const label = asString(item.label) || asString(item.key) || "-";
        lines.push(`${label}: ${stringifyMarketWorkspaceValue(item.value)}`);
      });
      if (items.length > 6) {
        lines.push(`... ${items.length - 6} more item(s)`);
      }
      break;
    }
    case "table": {
      const columns = Array.isArray(data.columns)
        ? data.columns.map((item) => asString(item)).filter((item) => item !== "")
        : [];
      const rows = safeObjectArray(data.rows);
      if (title) {
        lines.push(`${title}:`);
      }
      if (rows.length === 0) {
        if (asString(data.empty_text)) {
          lines.push(asString(data.empty_text));
        } else if (content) {
          lines.push(content);
        }
        break;
      }
      rows.slice(0, 5).forEach((row) => {
        const keys = columns.length > 0 ? columns : Object.keys(row).slice(0, 4);
        const summary = keys
          .map((key) => `${key}=${stringifyMarketWorkspaceValue(row[key])}`)
          .join(" | ");
        lines.push(`- ${summary}`);
      });
      if (rows.length > 5) {
        lines.push(`... ${rows.length - 5} more row(s)`);
      }
      break;
    }
    case "badge_list": {
      const items = normalizeDisplayStringArray(data.items);
      if (title) {
        lines.push(`${title}: ${items.join(", ") || "-"}`);
      } else if (items.length > 0) {
        lines.push(items.join(", "));
      }
      break;
    }
    case "link_list": {
      const links = safeObjectArray(data.links);
      if (title) {
        lines.push(`${title}:`);
      }
      links.slice(0, 5).forEach((item) => {
        const label = asString(item.label) || asString(item.url) || "-";
        lines.push(`- ${label}`);
      });
      if (links.length > 5) {
        lines.push(`... ${links.length - 5} more link(s)`);
      }
      break;
    }
    case "json_view": {
      const summary = asString(data.summary);
      if (title && summary) {
        lines.push(`${title}: ${summary}`);
      } else if (title) {
        lines.push(title);
      } else if (summary) {
        lines.push(summary);
      } else if (data.value !== undefined) {
        lines.push(truncateText(prettyJSON(data.value), 220));
      }
      break;
    }
    default:
      if (title && content) {
        lines.push(`${title}: ${content}`);
      } else if (title) {
        lines.push(title);
      } else if (content) {
        lines.push(content);
      }
      break;
  }

  return lines;
}

function buildMarketWorkspaceResultLines(result: PluginExecuteResult): string[] {
  const data = resolveMarketResultData(result);
  const blocks = Array.isArray(data.blocks) ? (data.blocks as PluginPageBlock[]) : [];
  const lines: string[] = [];
  blocks.slice(0, 4).forEach((block) => {
    lines.push(...renderMarketWorkspaceBlock(block));
  });
  return compactMarketWorkspaceLines(lines, 16);
}

function maybeMirrorMarketActionToWorkspace(
  action: string,
  params: UnknownMap,
  result: PluginExecuteResult
): void {
  if (!MARKET_WORKSPACE_MIRRORED_ACTIONS.has(action) || asBool(params.__workspace_command__)) {
    return;
  }
  const workspace = resolveMarketWorkspace();
  if (!workspace) {
    return;
  }

  const metadata = buildMarketWorkspaceMetadata(action, params, {
    mirrored_from_action: true
  });
  const summary = formatMarketWorkspaceStateSummary(params);

  if (result.success) {
    workspace.info(resolveMarketResultMessage(result, action), {
      ...metadata,
      success: true
    });
    if (summary) {
      workspace.writeln(summary, metadata);
    }
    return;
  }

  workspace.error(asString(result.error) || action, {
    ...metadata,
    success: false
  });
  if (summary) {
    workspace.writeln(summary, metadata);
  }
}

type ParsedMarketWorkspaceArgs = {
  params: UnknownMap;
  positional: string[];
};

function applyMarketWorkspaceArg(params: UnknownMap, rawKey: string, rawValue: unknown): void {
  const key = asString(rawKey).toLowerCase().replace(/-/g, "_");
  const value = typeof rawValue === "string" ? rawValue : asString(rawValue);

  switch (key) {
    case "source":
    case "source_id":
      params.source_id = asString(value);
      return;
    case "kind":
      params.kind = normalizeMarketKind(value);
      return;
    case "channel":
      params.channel = asString(value);
      return;
    case "q":
    case "query":
    case "keyword":
    case "search":
      params.q = asString(value);
      return;
    case "artifact":
    case "name":
      params.name = asString(value);
      return;
    case "version":
    case "release":
      params.version = asString(value);
      return;
    case "task":
    case "task_id":
      params.task_id = asString(value);
      return;
    case "activate":
      params.activate = asBool(value, true);
      return;
    case "auto_start":
    case "autostart":
      params.auto_start = asBool(value, false);
      return;
    case "note":
      params.note = asString(value);
      return;
    case "granted":
    case "granted_permissions":
    case "granted_permissions_json": {
      const normalized = asString(value);
      if (!normalized) {
        params.granted_permissions_json = "[]";
      } else if (normalized.startsWith("[")) {
        params.granted_permissions_json = normalized;
      } else {
        params.granted_permissions_json = stringifyStringArray(
          normalized
            .split(",")
            .map((item) => item.trim())
            .filter((item) => item !== "")
        );
      }
      return;
    }
    case "email":
    case "email_key":
    case "template_key":
      params.email_key = asString(value);
      return;
    case "landing":
    case "landing_slug":
    case "slug":
    case "page_key":
      params.landing_slug = asString(value);
      return;
    case "stage":
    case "workflow_stage":
      params.workflow_stage = asString(value);
      return;
    default:
      params[key] = value;
      return;
  }
}

function parseMarketWorkspaceCommandArgs(argv: string[]): ParsedMarketWorkspaceArgs {
  const params: UnknownMap = {};
  const positional: string[] = [];

  argv.forEach((item) => {
    const token = String(item || "").trim();
    if (!token) {
      return;
    }
    const eqIndex = token.indexOf("=");
    if (eqIndex > 0) {
      applyMarketWorkspaceArg(params, token.slice(0, eqIndex), token.slice(eqIndex + 1));
      return;
    }
    positional.push(token);
  });

  return {
    params,
    positional
  };
}

function writeMarketWorkspaceResult(
  workspace: PluginWorkspaceAPI,
  command: PluginWorkspaceCommandContext,
  action: string,
  params: UnknownMap,
  result: PluginExecuteResult
): void {
  const metadata = buildMarketWorkspaceMetadata(action, params, {
    workspace_command: command.entry,
    workspace_command_name: command.name
  });
  if (!result.success) {
    workspace.error(asString(result.error) || action, {
      ...metadata,
      success: false
    });
    const failedSummary = formatMarketWorkspaceStateSummary(params);
    if (failedSummary) {
      workspace.writeln(failedSummary, metadata);
    }
    return;
  }

  workspace.info(resolveMarketResultMessage(result, command.name), {
    ...metadata,
    success: true
  });

  const data = resolveMarketResultData(result);
  const stateSummary = formatMarketWorkspaceStateSummary(data.values || params);
  const detailLines = buildMarketWorkspaceResultLines(result);
  compactMarketWorkspaceLines(
    [...(stateSummary ? [stateSummary] : []), ...detailLines],
    18
  ).forEach((line) => {
    workspace.writeln(line, metadata);
  });
}

function executeMarketWorkspaceActionCommand(
  command: PluginWorkspaceCommandContext,
  context: PluginExecutionContext,
  config: UnknownMap,
  workspace: PluginWorkspaceAPI,
  action: string,
  params: UnknownMap
): PluginExecuteResult {
  const locale = resolveMarketLocale(context, params);
  const actionParams: UnknownMap = {
    ...params,
    __workspace_command__: true,
    locale
  };
  workspace.info(
    t("index.workspace.command.start", {
      command: command.name
    }),
    buildMarketWorkspaceMetadata(action, actionParams, {
      workspace_command: command.entry,
      phase: "start"
    })
  );
  const result = executeAction(action, actionParams, context, config);
  writeMarketWorkspaceResult(workspace, command, action, actionParams, result);
  return result;
}

function resolveMarketSourceWorkspaceParams(command: PluginWorkspaceCommandContext): UnknownMap {
  const parsed = parseMarketWorkspaceCommandArgs(command.argv);
  if (!parsed.params.source_id && parsed.positional.length > 0) {
    applyMarketWorkspaceArg(parsed.params, "source_id", parsed.positional[0]);
  }
  return parsed.params;
}

function resolveMarketCatalogWorkspaceParams(command: PluginWorkspaceCommandContext): UnknownMap {
  const parsed = parseMarketWorkspaceCommandArgs(command.argv);
  if (parsed.positional.length > 0) {
    if (!parsed.params.kind && isSupportedMarketKindToken(parsed.positional[0])) {
      applyMarketWorkspaceArg(parsed.params, "kind", parsed.positional[0]);
      if (parsed.positional[1]) {
        applyMarketWorkspaceArg(parsed.params, "q", parsed.positional[1]);
      }
    } else if (!parsed.params.q) {
      applyMarketWorkspaceArg(parsed.params, "q", parsed.positional[0]);
    }
  }
  return parsed.params;
}

function resolveMarketArtifactWorkspaceParams(command: PluginWorkspaceCommandContext): UnknownMap {
  const parsed = parseMarketWorkspaceCommandArgs(command.argv);
  if (!parsed.params.name && parsed.positional.length > 0) {
    applyMarketWorkspaceArg(parsed.params, "name", parsed.positional[0]);
  }
  if (!parsed.params.version && parsed.positional.length > 1) {
    applyMarketWorkspaceArg(parsed.params, "version", parsed.positional[1]);
  }
  return parsed.params;
}

function resolveMarketTaskWorkspaceParams(command: PluginWorkspaceCommandContext): UnknownMap {
  const parsed = parseMarketWorkspaceCommandArgs(command.argv);
  if (!parsed.params.task_id && parsed.positional.length > 0) {
    applyMarketWorkspaceArg(parsed.params, "task_id", parsed.positional[0]);
  }
  return parsed.params;
}

function resolvePackageInstallMode(targetState: UnknownMap): string {
  if (targetState.installed !== true) {
    return "Fresh install";
  }
  if (targetState.update_available === true) {
    return "Upgrade existing install";
  }
  return "Reapply existing version";
}

function resolveTemplateChangeLabel(resolved: UnknownMap, targetState: UnknownMap): string {
  const incomingDigest = asString(resolved.content_digest);
  const currentDigest = asString(targetState.current_digest);
  if (!incomingDigest || !currentDigest) {
    return targetState.target_exists === true ? "Target exists but digest is unavailable" : "New native target";
  }
  return incomingDigest === currentDigest ? "No content diff detected" : "Content will change";
}

function resolveTaskStatusBucket(task: UnknownMap): "failed" | "completed" | "running" {
  const summary = `${asString(task.status)} ${asString(task.phase)}`.toLowerCase();
  if (summary.includes("fail") || summary.includes("error") || summary.includes("reject")) {
    return "failed";
  }
  if (
    summary.includes("success") ||
    summary.includes("complete") ||
    summary.includes("activated") ||
    summary.includes("finished") ||
    summary.includes("rolled_back") ||
    summary.includes("already_active") ||
    summary.includes("imported")
  ) {
    return "completed";
  }
  return "running";
}

function isTemplateMarketKind(kind: unknown): kind is TemplateMarketKind {
  const normalized = asString(kind).toLowerCase();
  return (TEMPLATE_MARKET_KINDS as readonly string[]).includes(normalized);
}

function getTemplateTargetKey(state: MarketConsoleState): string {
  if (state.kind === "email_template") {
    return state.email_key || state.name;
  }
  if (state.kind === "landing_page_template") {
    return state.landing_slug || "home";
  }
  if (state.kind === "invoice_template") {
    return "invoice";
  }
  if (state.kind === "auth_branding_template") {
    return "auth_branding";
  }
  if (state.kind === "page_rule_pack") {
    return "page_rules";
  }
  return "";
}

function getTemplateTargetLabel(state: MarketConsoleState): string {
  if (state.kind === "email_template") {
    return t("index.load.context.emailKey.label");
  }
  if (state.kind === "landing_page_template") {
    return t("index.load.context.landingSlug.label");
  }
  if (state.kind === "invoice_template") {
    return "Target Key";
  }
  if (state.kind === "auth_branding_template") {
    return "Target Key";
  }
  if (state.kind === "page_rule_pack") {
    return "Target Key";
  }
  return "Target";
}

function getErrorStatusCode(error: unknown): number {
  if (!error || typeof error !== "object") {
    return 0;
  }
  const record = asRecord(error);
  return asInteger(record.status ?? record.status_code ?? record.statusCode, 0);
}

function getErrorMessage(error: unknown): string {
  if (error instanceof Error && error.message) {
    return error.message;
  }
  if (error && typeof error === "object") {
    const record = asRecord(error);
    return (
      asString(record.message) ||
      asString(record.error) ||
      asString(record.detail) ||
      asString(record.reason)
    );
  }
  return asString(error);
}

function isMissingEmailTemplateSnapshotError(error: unknown): boolean {
  const message = getErrorMessage(error).toLowerCase();
  if (!message.includes("email template not found")) {
    return false;
  }
  const statusCode = getErrorStatusCode(error);
  return statusCode === 0 || statusCode === 404;
}

function buildMissingTemplateTargetSnapshot(state: MarketConsoleState): UnknownMap {
  const targetKey = getTemplateTargetKey(state);
  if (state.kind === "email_template") {
    return {
      key: targetKey,
      event: targetKey,
      locale: "",
      filename: targetKey ? `${targetKey}.html` : "",
      content: "",
      digest: "",
      updated_at: null,
      size: 0,
      exists: false
    };
  }
  if (state.kind === "landing_page_template") {
    const slug = targetKey || "home";
    return {
      page_key: slug,
      slug,
      html_content: "",
      digest: "",
      updated_at: null,
      exists: false
    };
  }
  return {};
}

function getTemplateSnapshotContent(state: MarketConsoleState, snapshot: UnknownMap): string {
  if (state.kind === "email_template") {
    return asString(snapshot.content);
  }
  if (state.kind === "landing_page_template") {
    return asString(snapshot.html_content);
  }
  if (state.kind === "invoice_template") {
    return asString(snapshot.custom_template || snapshot.content);
  }
  if (state.kind === "auth_branding_template") {
    return asString(snapshot.custom_html || snapshot.content);
  }
  if (state.kind === "page_rule_pack") {
    return asString(snapshot.content || snapshot.json_content);
  }
  return "";
}

function getTemplateSnapshotLocator(state: MarketConsoleState, snapshot: UnknownMap): string {
  if (state.kind === "email_template") {
    return asString(snapshot.filename || snapshot.key || snapshot.event);
  }
  if (state.kind === "landing_page_template") {
    return asString(snapshot.slug || snapshot.page_key);
  }
  if (state.kind === "invoice_template") {
    return asString(snapshot.target_key || snapshot.file_path || snapshot.key);
  }
  if (state.kind === "auth_branding_template") {
    return asString(snapshot.target_key || snapshot.file_path || snapshot.key);
  }
  if (state.kind === "page_rule_pack") {
    return asString(snapshot.file_path || snapshot.target_key || snapshot.key);
  }
  return "";
}

function listTemplateSwitchKinds(current: TemplateMarketKind): TemplateMarketKind[] {
  return TEMPLATE_MARKET_KINDS.filter((kind) => kind !== current);
}

function buildMarketConsoleURL(state: MarketConsoleState, overrides: Partial<MarketConsoleState> = {}): string {
  const nextState: MarketConsoleState = {
    ...state,
    ...overrides
  };
  const params = new URLSearchParams();
  if (nextState.source_id) {
    params.set("source_id", nextState.source_id);
  }
  if (nextState.kind) {
    params.set("kind", nextState.kind);
  }
  if (nextState.channel) {
    params.set("channel", nextState.channel);
  }
  if (nextState.q) {
    params.set("q", nextState.q);
  }
  if (nextState.name) {
    params.set("name", nextState.name);
  }
  if (nextState.version) {
    params.set("version", nextState.version);
  }
  if (nextState.workflow_stage && nextState.workflow_stage !== "source") {
    params.set("workflow_stage", nextState.workflow_stage);
  }
  if (nextState.task_id) {
    params.set("task_id", nextState.task_id);
  }
  if (nextState.kind === "email_template" && nextState.email_key) {
    params.set("email_key", nextState.email_key);
  }
  if (nextState.kind === "landing_page_template" && nextState.landing_slug) {
    params.set("landing_slug", nextState.landing_slug);
  }
  const query = params.toString();
  return query ? `${ADMIN_PLUGIN_PAGE_PATH}?${query}` : ADMIN_PLUGIN_PAGE_PATH;
}

function buildArtifactContextURLFromItem(
  state: MarketConsoleState,
  item: UnknownMap,
  overrides: Partial<MarketConsoleState> = {}
): string {
  const coordinates = asRecord(item.coordinates);
  const resolved = asRecord(item.resolved);
  const template = asRecord(item.template);
  const historyEntry = asRecord(item.history_entry);
  const targetState = asRecord(item.target_state);
  const targetKey =
    asString(item.installed_target_key) ||
    asString(resolved.target_key) ||
    asString(template.target_key) ||
    asString(historyEntry.installed_target_key) ||
    asString(targetState.installed_target_key);
  return buildMarketConsoleURL(state, {
    name: asString(item.name) || asString(coordinates.name) || state.name,
    version:
      resolveVersionText(item.version) ||
      resolveVersionText(item.latest_version) ||
      resolveVersionText(coordinates.version) ||
      state.version,
    task_id: asString(item.task_id) || state.task_id,
    email_key:
      state.kind === "email_template" ? targetKey || state.email_key : state.email_key,
    landing_slug:
      state.kind === "landing_page_template" ? targetKey || state.landing_slug : state.landing_slug,
    ...overrides
  });
}

function marketAPI() {
  const api = getPluginMarket();
  if (!api) {
    throw new Error("Plugin.market is unavailable");
  }
  return api;
}

function emailTemplateAPI() {
  const api = getPluginEmailTemplate();
  if (!api) {
    throw new Error("Plugin.emailTemplate is unavailable");
  }
  return api;
}

function landingPageAPI() {
  const api = getPluginLandingPage();
  if (!api) {
    throw new Error("Plugin.landingPage is unavailable");
  }
  return api;
}

function invoiceTemplateAPI() {
  const api = getPluginInvoiceTemplate();
  if (!api) {
    throw new Error("Plugin.invoiceTemplate is unavailable");
  }
  return api;
}

function authBrandingAPI() {
  const api = getPluginAuthBranding();
  if (!api) {
    throw new Error("Plugin.authBranding is unavailable");
  }
  return api;
}

function pageRulePackAPI() {
  const api = getPluginPageRulePack();
  if (!api) {
    throw new Error("Plugin.pageRulePack is unavailable");
  }
  return api;
}

function inferWorkflowStage(
  kind: SupportedMarketKind,
  input: { name?: string; version?: string; task_id?: string }
): string {
  if (kind === "plugin_package" && asString(input.task_id)) {
    return "task";
  }
  if (asString(input.name) && asString(input.version)) {
    return "release";
  }
  if (asString(input.name)) {
    return "artifact";
  }
  return "source";
}

function setWorkflowStage(state: MarketConsoleState, stage: string): MarketConsoleState {
  state.workflow_stage = asString(stage) || inferWorkflowStage(state.kind, state);
  return state;
}

function buildConsoleState(
  input: UnknownMap,
  config: UnknownMap,
  context: PluginExecutionContext
): MarketConsoleState {
  const defaults = buildDefaultMarketConsoleState(config);
  const pageContext = resolvePluginPageContext(context);
  const query = pageContext.query_params;
  const kind = normalizeMarketKind(
    input.kind || query.kind || defaults.kind,
    normalizeMarketKind(defaults.kind)
  );
  const name = asString(input.name) || asString(query.name);
  const emailKey =
    asString(input.email_key) ||
    asString(query.email_key) ||
    asString(query.template_key) ||
    (kind === "email_template" ? name : defaults.email_key);
  const landingSlug =
    asString(input.landing_slug) ||
    asString(query.landing_slug) ||
    asString(query.page_key) ||
    asString(query.slug) ||
    defaults.landing_slug;

  return {
    source_id: asString(input.source_id) || asString(query.source_id) || defaults.source_id,
    kind,
    channel: asString(input.channel) || asString(query.channel) || defaults.channel,
    q: asString(input.q) || asString(query.q) || asString(query.search),
    name,
    version: asString(input.version) || asString(query.version),
    workflow_stage:
      asString(input.workflow_stage) ||
      asString(query.workflow_stage) ||
      inferWorkflowStage(kind, {
        name,
        version: asString(input.version) || asString(query.version),
        task_id: asString(input.task_id) || asString(query.task_id)
      }),
    activate: normalizeBooleanString(input.activate, defaults.activate),
    auto_start: normalizeBooleanString(input.auto_start, defaults.auto_start),
    granted_permissions_json:
      typeof input.granted_permissions_json === "string" && input.granted_permissions_json.trim()
        ? input.granted_permissions_json
        : defaults.granted_permissions_json,
    note: asString(input.note),
    email_key: emailKey,
    landing_slug: landingSlug,
    task_id: asString(input.task_id) || asString(query.task_id)
  };
}

function buildPackageModeState(state: MarketConsoleState): MarketConsoleState {
  const nextState: MarketConsoleState = {
    ...state
  };
  nextState.kind = nextState.kind === "payment_package" ? "payment_package" : "plugin_package";
  return nextState;
}

function buildTemplateModeState(state: MarketConsoleState): MarketConsoleState {
  const nextState: MarketConsoleState = {
    ...state
  };
  nextState.kind = isTemplateMarketKind(nextState.kind) ? nextState.kind : "email_template";
  if (nextState.kind === "email_template") {
    nextState.email_key = nextState.email_key || nextState.name;
  } else if (nextState.kind === "landing_page_template") {
    nextState.landing_slug = nextState.landing_slug || nextState.name || "home";
  }
  return nextState;
}

function buildPackageResetState(
  state: MarketConsoleState,
  kind: "plugin_package" | "payment_package" = state.kind === "payment_package"
    ? "payment_package"
    : "plugin_package"
): MarketConsoleState {
  const nextState = buildPackageModeState({
    ...state,
    kind
  });
  nextState.q = "";
  nextState.name = "";
  nextState.version = "";
  nextState.task_id = "";
  nextState.note = "";
  nextState.activate = true;
  nextState.auto_start = false;
  nextState.granted_permissions_json = "[]";
  return nextState;
}

function buildTemplateResetState(
  state: MarketConsoleState,
  kind: TemplateMarketKind = isTemplateMarketKind(state.kind) ? state.kind : "email_template"
): MarketConsoleState {
  const nextState = buildTemplateModeState({
    ...state,
    kind
  });
  nextState.q = "";
  nextState.name = "";
  nextState.version = "";
  nextState.task_id = "";
  nextState.note = "";
  return nextState;
}

function buildPackageResetURL(
  state: MarketConsoleState,
  overrides: Partial<MarketConsoleState> = {}
): string {
  return buildMarketConsoleURL(buildPackageResetState(state), overrides);
}

function buildTemplateResetURL(
  state: MarketConsoleState,
  overrides: Partial<MarketConsoleState> = {}
): string {
  return buildMarketConsoleURL(buildTemplateResetState(state), overrides);
}

function sourceList(): UnknownMap[] {
  const response = marketAPI().source?.list() || {};
  return safeObjectArray(asRecord(response).items);
}

function findSourceSummary(sourceID: string, sources: UnknownMap[]): UnknownMap | null {
  const normalizedSourceID = sourceID.toLowerCase();
  for (let idx = 0; idx < sources.length; idx += 1) {
    const source = sources[idx];
    if (asString(source.source_id).toLowerCase() === normalizedSourceID) {
      return source;
    }
  }
  return null;
}

function resolveSelectedVersion(state: MarketConsoleState): string {
  if (state.version) {
    return state.version;
  }
  if (!state.name) {
    throw new Error("Artifact name is required");
  }
  const artifact = asRecord(
    marketAPI().artifact?.get({
      source_id: state.source_id,
      kind: state.kind,
      name: state.name
    }) || {}
  );
  const latestVersion = asString(artifact.latest_version);
  if (latestVersion) {
    return latestVersion;
  }
  const versions = safeObjectArray(artifact.versions);
  if (versions.length > 0) {
    return asString(versions[0].version);
  }
  throw new Error("Artifact version could not be resolved");
}

function resolvePluginPackageRecommendedGrantedFromRelease(releaseValue: unknown): string[] {
  const release = asRecord(releaseValue);
  const permissions = asRecord(release.permissions);
  const defaultGranted = normalizeStringArray(prettyJSONArray(permissions.default_granted));
  const required = normalizeStringArray(prettyJSONArray(permissions.required));
  return normalizeStringArray([...defaultGranted, ...required]);
}

function resolvePluginPackageRecommendedGrantedFromPreview(previewValue: unknown): string[] {
  const preview = asRecord(previewValue);
  const permissions = asRecord(preview.permissions);
  const release = asRecord(preview.release);
  const defaultGranted = normalizeStringArray(prettyJSONArray(permissions.default_granted));
  const required = normalizeStringArray(prettyJSONArray(asRecord(release.permissions).required));
  return normalizeStringArray([...defaultGranted, ...required]);
}

function getRevisionBackendLabel(): string {
  return "host.market.install";
}

function syncTemplateTargetState(state: MarketConsoleState, payload: UnknownMap): MarketConsoleState {
  if (!isTemplateMarketKind(state.kind)) {
    return state;
  }
  const resolved = asRecord(payload.resolved);
  const template = asRecord(payload.template);
  const targetState = asRecord(payload.target_state);
  const targetKey =
    asString(resolved.target_key) ||
    asString(template.target_key) ||
    asString(targetState.installed_target_key);
  if (!targetKey) {
    return state;
  }
  if (state.kind === "email_template") {
    state.email_key = targetKey;
  } else {
    state.landing_slug = targetKey;
  }
  return state;
}

function loadTemplateTargetSnapshot(state: MarketConsoleState): UnknownMap {
  try {
    if (state.kind === "email_template") {
      return asRecord(
        emailTemplateAPI().get({
          key: getTemplateTargetKey(state)
        })
      );
    }
    if (state.kind === "landing_page_template") {
      return asRecord(
        landingPageAPI().get({
          slug: getTemplateTargetKey(state) || "home"
        })
      );
    }
    if (state.kind === "invoice_template") {
      return asRecord(
        invoiceTemplateAPI().get({
          target_key: getTemplateTargetKey(state)
        })
      );
    }
    if (state.kind === "auth_branding_template") {
      return asRecord(
        authBrandingAPI().get({
          target_key: getTemplateTargetKey(state)
        })
      );
    }
    if (state.kind === "page_rule_pack") {
      return asRecord(
        pageRulePackAPI().get({
          target_key: getTemplateTargetKey(state)
        })
      );
    }
    return {};
  } catch (error) {
    if (state.kind === "email_template" && isMissingEmailTemplateSnapshotError(error)) {
      return buildMissingTemplateTargetSnapshot(state);
    }
    throw error;
  }
}

function loadTemplateTargetHistory(state: MarketConsoleState, limit = 6): UnknownMap[] {
  if (!isTemplateMarketKind(state.kind) || !state.source_id) {
    return [];
  }
  const result = asRecord(
    marketAPI().install?.history?.list({
      source_id: state.source_id,
      kind: state.kind,
      name: state.name || undefined,
      ...buildTemplateMarketTargetParams(state),
      limit
    }) || {}
  );
  return safeObjectArray(result.items);
}

function buildTemplateTargetInsightBlocks(state: MarketConsoleState): PluginPageBlock[] {
  if (!isTemplateMarketKind(state.kind)) {
    return [];
  }

  const blocks: PluginPageBlock[] = [];
  const targetKey = getTemplateTargetKey(state);
  const targetLabel = getTemplateTargetLabel(state);

  try {
    const snapshot = loadTemplateTargetSnapshot(state);
    const content = getTemplateSnapshotContent(state, snapshot);
    blocks.push(
      buildPluginKeyValueBlock({
        title: t("index.templateTarget.snapshot.title"),
        items: [
          { label: t("index.load.context.kind.label"), value: state.kind },
          { label: targetLabel, value: targetKey || "-" },
          {
            label: t("index.templateTarget.snapshot.fileOrSlug.label"),
            value: getTemplateSnapshotLocator(state, snapshot) || "-"
          },
          { label: t("index.templateTarget.snapshot.digest.label"), value: asString(snapshot.digest) || "-" },
          { label: t("index.templateTarget.snapshot.updatedAt.label"), value: asString(snapshot.updated_at) || "-" },
          {
            label: t("index.templateTarget.snapshot.nativeBytes.label"),
            value: String(asInteger(snapshot.content_bytes ?? snapshot.size, content.length))
          }
        ]
      })
    );
    if (content) {
      blocks.push(
        buildPluginTextBlock(truncateText(content, 220), t("index.templateTarget.snapshot.contentSnippet.title"))
      );
    }
  } catch (error) {
    blocks.push(
      buildPluginAlertBlock({
        title: t("index.templateTarget.snapshot.title"),
        content: t("index.templateTarget.snapshot.failed", {
          error: errorToString(error)
        }),
        variant: "warning"
      })
    );
  }

  try {
    const historyItems = loadTemplateTargetHistory(state, 6);
    const activeItem =
      historyItems.find((item) => item.is_active === true) ||
      historyItems[0] ||
      {};
    blocks.push(
      buildPluginKeyValueBlock({
        title: t("index.templateTarget.status.title"),
        items: [
          { label: targetLabel, value: targetKey || "-" },
          { label: t("index.templateTarget.status.historyCount.label"), value: String(historyItems.length) },
          { label: t("index.templateTarget.status.activeVersion.label"), value: asString(activeItem.version) || "-" },
          { label: t("index.templateTarget.status.activeArtifact.label"), value: asString(activeItem.name) || state.name || "-" },
          { label: t("index.sourceSummary.sourceId.label"), value: asString(activeItem.source_id) || state.source_id || "-" }
        ]
      })
    );
    blocks.push(
      buildPluginTableBlock({
        title: t("index.templateTarget.recentVersions.title"),
        columns: ["name", "version", "target_key", "is_active", "installed_at"],
        rows: historyItems.map((item) => ({
          name: asString(item.name) || "-",
          version: asString(item.version) || "-",
          target_key: asString(item.installed_target_key) || targetKey || "-",
          is_active: item.is_active === true ? "true" : "false",
          installed_at: asString(item.installed_at) || "-"
        })),
        empty_text: t("index.templateTarget.recentVersions.empty")
      })
    );
    if (historyItems.length > 0) {
      blocks.push(
        buildPluginLinkListBlock({
          title: t("index.history.links.releaseContext.title"),
          links: historyItems.slice(0, 6).map((item) => ({
            label:
              item.is_active === true
                ? t("index.templateTarget.recentVersions.activeSuffix", {
                    release: `${asString(item.name) || state.name || "template"}@${asString(item.version) || "-"}`
                  })
                : `${asString(item.name) || state.name || "template"}@${asString(item.version) || "-"}`,
            url: buildMarketConsoleURL(state, {
              name: asString(item.name) || state.name,
              version: asString(item.version) || state.version,
              email_key:
                state.kind === "email_template"
                  ? asString(item.installed_target_key) || state.email_key
                  : state.email_key,
              landing_slug:
                state.kind === "landing_page_template"
                  ? asString(item.installed_target_key) || state.landing_slug
                  : state.landing_slug
            })
          }))
        })
      );
    }
  } catch (error) {
    blocks.push(
      buildPluginAlertBlock({
        title: t("index.templateTarget.status.title"),
        content: t("index.templateTarget.history.failed", {
          error: errorToString(error)
        }),
        variant: "warning"
      })
    );
  }

  return blocks;
}

function buildSelectedSourceSummaryBlock(
  state: MarketConsoleState,
  sources: UnknownMap[],
  title: string
): PluginPageBlock {
  const selectedSource = findSourceSummary(state.source_id, sources);
  return buildPluginKeyValueBlock({
    title,
    items: [
      { label: t("index.sourceSummary.sourceId.label"), value: state.source_id || "-" },
      { label: t("index.sourceSummary.configuredSources.label"), value: String(sources.length) },
      {
        label: t("index.sourceSummary.sourceName.label"),
        value: asString(selectedSource?.name) || asString(selectedSource?.source_id) || "-"
      },
      { label: t("index.sourceSummary.baseUrl.label"), value: asString(selectedSource?.base_url) || "-" },
      { label: t("index.sourceSummary.defaultChannel.label"), value: asString(selectedSource?.default_channel) || "-" },
      {
        label: t("index.sourceSummary.signature.label"),
        value: selectedSource
          ? formatBooleanState(
              selectedSource.supports_signature,
              t("index.sourceSummary.signature.supported"),
              t("index.sourceSummary.signature.notExposed")
            )
          : "-"
      },
      {
        label: t("index.sourceSummary.enabledStatus.label"),
        value: selectedSource
          ? formatBooleanState(
              selectedSource.enabled,
              t("index.sourceSummary.enabledStatus.enabled"),
              t("index.sourceSummary.enabledStatus.disabled")
            )
          : "-"
      },
      {
        label: t("index.sourceSummary.allowedKinds.label"),
        value: safeArray(selectedSource?.allowed_kinds).join(", ") || "-"
      }
    ]
  });
}

function sourceAllowsKind(source: UnknownMap, kind: SupportedMarketKind): boolean {
  const allowedKinds = normalizeStringArray(prettyJSONArray(source.allowed_kinds));
  return allowedKinds.length > 0 && allowedKinds.includes(kind);
}

function normalizeReleaseTargetLabels(value: unknown): string[] {
  return uniqueDisplayStrings(
    safeArray(value)
      .map((item) => {
        if (typeof item === "string") {
          return item;
        }
        const record = asRecord(item);
        return (
          asString(record.target_key) ||
          asString(record.slug) ||
          asString(record.key) ||
          asString(record.name)
        );
      })
      .filter((item) => item.length > 0)
  );
}

function formatMarketKindLabel(kind: string): string {
  switch (kind) {
    case "plugin_package":
      return "Plugin Package";
    case "payment_package":
      return "Payment Package";
    case "email_template":
      return "Email Template";
    case "landing_page_template":
      return "Landing Page";
    case "invoice_template":
      return "Invoice Template";
    case "auth_branding_template":
      return "Auth Branding Template";
    case "page_rule_pack":
      return "Page Rule Pack";
    default:
      return kind || "Artifact";
  }
}

function buildCatalogBrowserItems(items: UnknownMap[]): UnknownMap[] {
  return items.map((item) => {
    const latestRelease = asRecord(item.latest_release);
    const releaseDownload = asRecord(latestRelease.download);
    const directDownload = asRecord(item.download);
    return {
      kind: asString(item.kind),
      name: asString(item.name),
      title: asString(item.title),
      latest_version: asString(item.latest_version),
      channel: asString(item.channel),
      publisher: asString(asRecord(item.publisher).name || asRecord(item.publisher).id),
      published_at: asString(item.published_at || latestRelease.published_at),
      size: asInteger(releaseDownload.size ?? directDownload.size, 0),
    };
  });
}

function buildCatalogBrowserPayload(
  state: MarketConsoleState,
  source: UnknownMap,
  pagination: UnknownMap,
  items: UnknownMap[]
): UnknownMap {
  return {
    source_id: state.source_id,
    source_name: asString(source.name) || asString(source.source_id) || state.source_id,
    kind: state.kind,
    channel: state.channel,
    query: state.q,
    total: asInteger(pagination.total, items.length),
    has_more: Boolean(pagination.has_more),
    items: buildCatalogBrowserItems(items),
  };
}

function buildReleaseLinks(release: UnknownMap): Array<{ label: string; url: string }> {
  const download = asRecord(release.download);
  const docs = asRecord(release.docs);
  const ui = asRecord(release.ui);
  const links: Array<{ label: string; url: string }> = [];
  const seen = new Set<string>();

  const appendLink = (label: string, url: unknown) => {
    const normalized = asString(url).trim();
    if (!normalized || seen.has(normalized)) {
      return;
    }
    seen.add(normalized);
    links.push({ label, url: normalized });
  };

  appendLink(t("index.releaseLinks.downloadArtifact"), download.url);
  appendLink(t("index.releaseLinks.openDocs"), docs.docs_url);
  appendLink(t("index.releaseLinks.openSupport"), docs.support_url);
  appendLink(t("index.releaseLinks.openIcon"), ui.icon_url);

  safeArray(ui.screenshots)
    .slice(0, 3)
    .forEach((item, index) => {
      if (typeof item === "string") {
        appendLink(
          t("index.releaseLinks.screenshot", {
            index: index + 1
          }),
          item
        );
        return;
      }
      const record = asRecord(item);
      appendLink(
        t("index.releaseLinks.screenshot", {
          index: index + 1
        }),
        record.url || record.src
      );
    });

  return links;
}

function buildNativeMarketInstallURLFromCoordinates(
  sourceValue: unknown,
  kindValue: unknown,
  nameValue: unknown,
  versionValue: unknown
): string {
  const source = asRecord(sourceValue);
  const baseURL = asString(source.base_url);
  const kind = asString(kindValue || "plugin_package");
  const name = asString(nameValue);
  const version = asString(versionValue);
  if (!baseURL || !name || !version) {
    return "";
  }
  if (kind !== "plugin_package" && kind !== "payment_package") {
    return "";
  }
  const params = new URLSearchParams();
  params.set(kind === "payment_package" ? "market_import" : "market_install", "1");
  params.set("market_source_id", asString(source.source_id) || "official");
  if (asString(source.name)) {
    params.set("market_source_name", asString(source.name));
  }
  params.set("market_source_base_url", baseURL);
  if (asString(source.default_channel)) {
    params.set("market_source_channel", asString(source.default_channel));
  }
  const allowedKinds = normalizeStringArray(prettyJSONArray(source.allowed_kinds));
  if (allowedKinds.length > 0) {
    params.set("market_source_allowed_kinds", allowedKinds.join(","));
  }
  params.set("market_kind", kind);
  params.set("market_name", name);
  params.set("market_version", version);
  return kind === "payment_package"
    ? `/admin/payment-methods?${params.toString()}`
    : `/admin/plugins?${params.toString()}`;
}

function buildArtifactDetailBlocks(artifact: UnknownMap, state: MarketConsoleState): PluginPageBlock[] {
  const source = asRecord(artifact.source);
  const governance = asRecord(artifact.governance);
  const versions = safeObjectArray(artifact.versions);
  const channels = uniqueDisplayStrings(
    versions.map((item) => asString(item.channel)).concat(normalizeDisplayStringArray(artifact.channels))
  );
  const latestVersion = asString(artifact.latest_version);
  const latestContextURL = latestVersion
    ? buildMarketConsoleURL(state, {
        version: latestVersion,
        task_id: ""
      })
    : "";
  const artifactName = asString(artifact.name) || state.name || "-";
  const blocks: PluginPageBlock[] = [
    buildPluginAlertBlock({
      title: t("index.artifactDetail.title"),
      content:
        versions.length === 0
          ? t("index.artifactDetail.content.noVersions")
          : t("index.artifactDetail.content.withVersions", {
              count: versions.length
            }),
      variant: versions.length === 0 ? "info" : "success"
    }),
    buildPluginStatsGridBlock({
      title: t("index.artifactDetail.overview.title"),
      items: [
        { label: t("index.artifactDetail.overview.kind.label"), value: asString(artifact.kind) || state.kind },
        { label: t("index.artifactDetail.overview.name.label"), value: artifactName },
        { label: t("index.artifactDetail.overview.latestVersion.label"), value: latestVersion || "-" },
        { label: t("index.artifactDetail.overview.versionCount.label"), value: versions.length },
        { label: t("index.artifactDetail.overview.channelCount.label"), value: channels.length },
        { label: t("index.artifactDetail.overview.governanceFields.label"), value: Object.keys(governance).length }
      ]
    }),
    buildPluginKeyValueBlock({
      title: t("index.artifactDetail.metadata.title"),
      items: [
        { label: t("index.sourceSummary.sourceId.label"), value: asString(source.source_id) || state.source_id || "-" },
        { label: t("index.sourceSummary.sourceName.label"), value: asString(source.name) || "-" },
        { label: t("index.artifactDetail.metadata.displayTitle.label"), value: asString(artifact.title) || "-" },
        { label: t("index.artifactDetail.metadata.summary.label"), value: truncateText(artifact.summary, 220) || "-" },
        { label: t("index.artifactDetail.metadata.description.label"), value: truncateText(artifact.description, 220) || "-" },
        { label: t("index.artifactDetail.metadata.governanceMode.label"), value: asString(governance.mode) || "-" },
        { label: t("index.artifactDetail.metadata.installStrategy.label"), value: asString(governance.install_strategy) || "-" },
        { label: t("index.artifactDetail.metadata.supportsRollback.label"), value: formatBooleanState(governance.supports_rollback) }
      ]
    })
  ];

  if (channels.length > 0) {
    blocks.push(
      buildPluginBadgeListBlock({
        title: t("index.artifactDetail.channels.title"),
        items: channels
      })
    );
  }

  blocks.push(
    buildPluginTableBlock({
      title: t("index.artifactDetail.versions.title"),
      columns: ["version", "channel", "published_at"],
      rows: versions.map((item) => ({
        version: asString(item.version),
        channel: asString(item.channel),
        published_at: asString(item.published_at)
      })),
      empty_text: t("index.artifactDetail.versions.empty")
    })
  );

  if (versions.length > 0) {
    blocks.push(
      buildPluginLinkListBlock({
        title: t("index.artifactDetail.quickContext.title"),
        links: versions.slice(0, 8).map((item) => ({
          label: `${asString(artifact.name) || state.name || "artifact"}@${asString(item.version) || "-"}`,
          url: buildMarketConsoleURL(state, {
            version: asString(item.version) || state.version,
            task_id: ""
          })
        }))
      })
    );
  }

  const nextLinks: Array<{ label: string; url: string }> = [
    {
      label: t("index.common.backToSourceContext"),
      url: buildMarketConsoleURL(state, { version: "", task_id: "" })
    }
  ];
  if (latestContextURL) {
    nextLinks.push({
      label: t("index.common.inspectRelease", {
        release: `${asString(artifact.name) || state.name || "artifact"}@${latestVersion}`
      }),
      url: latestContextURL
    });
  }
  const nativeInstallURL = buildNativeMarketInstallURLFromCoordinates(
    source,
    asString(artifact.kind) || state.kind,
    asString(artifact.name) || state.name,
    latestVersion
  );
  if (nativeInstallURL) {
    nextLinks.push({
      label:
        (asString(artifact.kind) || state.kind) === "payment_package"
          ? t("index.common.openNativeImportPage")
          : t("index.common.openNativeInstallPage"),
      url: nativeInstallURL
    });
  }
  blocks.push(
    buildPluginLinkListBlock({
      title: t("index.common.nextSteps.title"),
      links: nextLinks
    })
  );

  blocks.push(
    buildCollapsedPayloadBlock(
      t("index.artifactDetail.payload.title"),
      artifact,
      t("index.artifactDetail.payload.summary")
    )
  );

  return blocks;
}

function buildSourceDetailBlocks(source: UnknownMap, state: MarketConsoleState): PluginPageBlock[] {
  const allowedKinds = normalizeStringArray(prettyJSONArray(source.allowed_kinds));
  const enabled = asBool(source.enabled, true);
  const kindAllowed = sourceAllowsKind(source, state.kind);
  const blocks: PluginPageBlock[] = [
    buildPluginAlertBlock({
      title: t("index.sourceDetail.title"),
      content:
        enabled !== true
          ? t("index.sourceDetail.content.disabled")
          : kindAllowed
            ? t("index.sourceDetail.content.allowed")
            : t("index.sourceDetail.content.disallowed", {
                kind: state.kind
              }),
      variant: enabled !== true || !kindAllowed ? "warning" : "success"
    }),
    buildPluginKeyValueBlock({
      title: t("index.sourceDetail.summary.title"),
      items: [
        { label: t("index.sourceSummary.sourceId.label"), value: asString(source.source_id) || state.source_id || "-" },
        { label: t("index.sourceSummary.sourceName.label"), value: asString(source.name) || "-" },
        { label: t("index.sourceDetail.summary.baseUrl.label"), value: asString(source.base_url) || "-" },
        { label: t("index.sourceDetail.summary.currentKind.label"), value: state.kind || "-" },
        { label: t("index.sourceDetail.summary.kindAllowed.label"), value: formatBooleanState(kindAllowed) },
        { label: t("index.sourceDetail.summary.enabled.label"), value: formatBooleanState(enabled) },
        { label: t("index.sourceDetail.summary.supportsSignature.label"), value: formatBooleanState(source.supports_signature) },
        { label: t("index.sourceSummary.defaultChannel.label"), value: asString(source.default_channel) || "-" },
        { label: t("index.sourceDetail.summary.currentChannel.label"), value: state.channel || "-" },
        { label: t("index.sourceDetail.summary.currentArtifact.label"), value: state.name || "-" },
        { label: t("index.sourceDetail.summary.currentVersion.label"), value: state.version || "-" }
      ]
    })
  ];

  if (allowedKinds.length > 0) {
    blocks.push(
      buildPluginBadgeListBlock({
        title: t("index.sourceDetail.allowedKinds.title"),
        items: allowedKinds
      })
    );
    blocks.push(
      buildPluginLinkListBlock({
        title: t("index.sourceDetail.switchContext.title"),
        links: allowedKinds.map((kind) => ({
          label: formatMarketKindLabel(kind),
          url: buildMarketConsoleURL(state, {
            kind: normalizeMarketKind(kind, state.kind),
            q: "",
            name: "",
            version: "",
            task_id: ""
          })
        }))
      })
    );
  }

  blocks.push(
    buildCollapsedPayloadBlock(
      t("index.sourceDetail.payload.title"),
      source,
      t("index.sourceDetail.payload.summary")
    )
  );

  return blocks;
}

function buildReleaseDetailBlocks(release: UnknownMap, state: MarketConsoleState): PluginPageBlock[] {
  const source = asRecord(release.source);
  const governance = asRecord(release.governance);
  const download = asRecord(release.download);
  const signature = asRecord(release.signature);
  const compatibility = asRecord(release.compatibility);
  const install = asRecord(release.install);
  const permissions = asRecord(release.permissions);
  const requestedPermissions = normalizeStringArray(prettyJSONArray(permissions.requested));
  const defaultGranted = normalizeStringArray(prettyJSONArray(permissions.default_granted));
  const targets = normalizeReleaseTargetLabels(release.targets);
  const links = buildReleaseLinks(release);
  const channel = asString(release.channel);
  const selectedChannel = state.channel || asString(source.default_channel);
  const channelMismatch = Boolean(selectedChannel && channel && selectedChannel !== channel);
  const artifactContextURL = buildMarketConsoleURL(state, {
    version: "",
    task_id: ""
  });
  const releaseContextURL = buildMarketConsoleURL(state, {
    version: asString(release.version) || state.version,
    task_id: ""
  });
  const releaseName = asString(release.name) || state.name || "artifact";
  const releaseVersion = asString(release.version) || state.version || "-";
  const nativeInstallURL = buildNativeMarketInstallURLFromCoordinates(
    source,
    asString(release.kind) || state.kind,
    asString(release.name) || state.name,
    asString(release.version) || state.version
  );
  const blocks: PluginPageBlock[] = [
    buildPluginAlertBlock({
      title: t("index.releaseDetail.title"),
      content: channelMismatch
        ? t("index.releaseDetail.content.channelMismatch", {
            channel,
            selectedChannel
          })
        : t("index.releaseDetail.content.default"),
      variant: channelMismatch ? "info" : "success"
    }),
    buildPluginStatsGridBlock({
      title: t("index.releaseDetail.overview.title"),
      items: [
        { label: t("index.artifactDetail.overview.kind.label"), value: asString(release.kind) || state.kind },
        { label: t("index.load.context.version.label"), value: releaseVersion },
        { label: t("index.catalog.context.channel.label"), value: channel || "-" },
        { label: t("index.releaseDetail.overview.downloadSize.label"), value: asInteger(download.size, 0) },
        { label: t("index.releaseDetail.overview.requestedPermissions.label"), value: requestedPermissions.length },
        { label: t("index.releaseDetail.overview.defaultGranted.label"), value: defaultGranted.length },
        { label: t("index.releaseDetail.overview.targetCount.label"), value: targets.length },
        {
          label: t("index.releaseDetail.overview.hostCompatibility.label"),
          value:
            compatibility.compatible === false
              ? t("index.releaseDetail.overview.hostCompatibility.rejected")
              : t("index.releaseDetail.overview.hostCompatibility.ready")
        }
      ]
    }),
    buildPluginKeyValueBlock({
      title: t("index.releaseDetail.context.title"),
      items: [
        { label: t("index.sourceSummary.sourceId.label"), value: asString(source.source_id) || state.source_id || "-" },
        { label: t("index.sourceSummary.sourceName.label"), value: asString(source.name) || "-" },
        { label: t("index.artifactDetail.overview.name.label"), value: releaseName },
        { label: t("index.artifactDetail.metadata.displayTitle.label"), value: asString(release.title) || "-" },
        { label: t("index.releaseDetail.context.publishedAt.label"), value: asString(release.published_at) || "-" },
        { label: t("index.artifactDetail.metadata.summary.label"), value: truncateText(release.summary, 220) || "-" },
        { label: t("index.artifactDetail.metadata.description.label"), value: truncateText(release.description, 220) || "-" },
        { label: t("index.releaseDetail.context.releaseNotes.label"), value: truncateText(release.release_notes, 220) || "-" }
      ]
    }),
    buildPluginKeyValueBlock({
      title: t("index.releaseDetail.compatibility.title"),
      items: [
        { label: t("index.artifactDetail.metadata.governanceMode.label"), value: asString(governance.mode) || "-" },
        { label: t("index.artifactDetail.metadata.installStrategy.label"), value: asString(governance.install_strategy) || "-" },
        { label: t("index.releaseDetail.compatibility.runtime.label"), value: asString(compatibility.runtime) || "-" },
        { label: t("index.releaseDetail.compatibility.minBridge.label"), value: asString(compatibility.min_host_bridge_version) || "-" },
        { label: t("index.releaseDetail.compatibility.packageFormat.label"), value: asString(install.package_format) || "-" },
        { label: t("index.releaseDetail.compatibility.entry.label"), value: asString(install.entry) || "-" },
        { label: t("index.releaseDetail.compatibility.sha256.label"), value: asString(download.sha256) || "-" },
        { label: t("index.releaseDetail.compatibility.signatureAlgorithm.label"), value: asString(signature.algorithm) || "-" },
        { label: t("index.artifactDetail.metadata.supportsRollback.label"), value: formatBooleanState(governance.supports_rollback) }
      ]
    })
  ];

  if (requestedPermissions.length > 0) {
    blocks.push(
      buildPluginBadgeListBlock({
        title: t("index.releaseDetail.permissions.requested.title"),
        items: requestedPermissions
      })
    );
  }

  if (defaultGranted.length > 0) {
    blocks.push(
      buildPluginBadgeListBlock({
        title: t("index.releaseDetail.permissions.defaultGranted.title"),
        items: defaultGranted
      })
    );
  }

  if (targets.length > 0) {
    blocks.push(
      buildPluginBadgeListBlock({
        title: t("index.releaseDetail.targets.title"),
        items: targets
      })
    );
  }

  if (links.length > 0) {
    blocks.push(
      buildPluginLinkListBlock({
        title: t("index.releaseDetail.links.title"),
        links
      })
    );
  }

  const nextLinks: Array<{ label: string; url: string }> = [
    {
      label: t("index.common.backToSourceContext"),
      url: buildMarketConsoleURL(state, { version: "", task_id: "" })
    },
    {
      label: t("index.common.backToArtifactContext"),
      url: artifactContextURL
    },
    {
      label: t("index.releaseDetail.next.reuse", {
        release: `${releaseName}@${releaseVersion}`
      }),
      url: releaseContextURL
    }
  ];
  if (nativeInstallURL) {
    nextLinks.push({
      label:
        (asString(release.kind) || state.kind) === "payment_package"
          ? t("index.common.openNativeImportPage")
          : t("index.common.openNativeInstallPage"),
      url: nativeInstallURL
    });
  }
  blocks.push(
    buildPluginLinkListBlock({
      title: t("index.common.nextSteps.title"),
      links: nextLinks
    })
  );

  blocks.push(
    buildCollapsedPayloadBlock(
      t("index.releaseDetail.payload.title"),
      release,
      t("index.releaseDetail.payload.summary")
    )
  );

  return blocks;
}

function buildPackageWorkflowBlocks(state: MarketConsoleState): PluginPageBlock[] {
  const nextKind = state.kind === "payment_package" ? "plugin_package" : "payment_package";
  const links: Array<{ label: string; url: string }> = [
    {
      label: t("index.workflow.package.reset"),
      url: buildPackageResetURL(state)
    },
    {
      label:
        state.kind === "payment_package"
          ? t("index.workflow.package.switchToPlugin")
          : t("index.workflow.package.switchToPayment"),
      url: buildPackageResetURL(state, {
        kind: nextKind
      })
    }
  ];

  if (state.name) {
    links.push({
      label: t("index.workflow.common.reuse", {
        label: `${state.name}${state.version ? `@${state.version}` : ""}`
      }),
      url: buildMarketConsoleURL(state, { task_id: state.kind === "plugin_package" ? state.task_id : "" })
    });
  }

  if (state.task_id) {
    links.push({
      label: t("index.workflow.package.clearTask", {
        taskId: state.task_id
      }),
      url: buildMarketConsoleURL(state, { task_id: "" })
    });
  }

  return [
    buildPluginLinkListBlock({
      title: t("index.workflow.package.title"),
      links: dedupeLinks(links)
    })
  ];
}

function buildTemplateWorkflowBlocks(state: MarketConsoleState): PluginPageBlock[] {
  const links: Array<{ label: string; url: string }> = [
    {
      label: t("index.workflow.template.searchOther"),
      url: buildTemplateResetURL(state)
    }
  ];

  if (isTemplateMarketKind(state.kind)) {
    listTemplateSwitchKinds(state.kind).forEach((kind) => {
      links.push({
        label: `Switch to ${formatMarketKindLabel(kind)}`,
        url: buildTemplateResetURL(state, { kind })
      });
    });
  }

  if (state.name) {
    links.push({
      label: t("index.workflow.common.reuse", {
        label: `${state.name}${state.version ? `@${state.version}` : ""}`
      }),
      url: buildMarketConsoleURL(state)
    });
  }

  return [
    buildPluginLinkListBlock({
      title: t("index.workflow.template.title"),
      links: dedupeLinks(links)
    })
  ];
}

function buildCatalogSelectionLinks(
  state: MarketConsoleState,
  items: UnknownMap[],
  catalogSource: UnknownMap = {}
): PluginPageBlock[] {
  if (items.length === 0) {
    return [];
  }
  const catalogItems = items.slice(0, 8);
  const blocks: PluginPageBlock[] = [
    buildPluginLinkListBlock({
      title: t("index.catalog.links.artifactContext.title"),
      links: catalogItems.map((item) => {
        const name = asString(item.name) || "artifact";
        return {
          label: name,
          url: buildMarketConsoleURL(state, {
            name,
            version: "",
            task_id: ""
          })
        };
      })
    }),
    buildPluginLinkListBlock({
      title:
        state.kind === "plugin_package" || state.kind === "payment_package"
          ? t("index.catalog.links.latestPackageContext.title")
          : t("index.catalog.links.latestTemplateContext.title"),
      links: catalogItems.map((item) => {
        const name = asString(item.name) || "artifact";
        const version = asString(item.latest_version) || "latest";
        return {
          label: `${name}@${version}`,
          url: buildArtifactContextURLFromItem(state, item, { task_id: "" })
        };
      })
    })
  ];

  if (state.kind === "plugin_package" || state.kind === "payment_package") {
    const nativeLinks = catalogItems
      .map((item) => {
        const source = asRecord(item.source);
        const name = asString(item.name);
        const version = asString(item.latest_version);
        const url = buildNativeMarketInstallURLFromCoordinates(
          Object.keys(source).length > 0
            ? source
            : catalogSource,
          state.kind,
          name,
          version
        );
        if (!name || !version || !url) {
          return null;
        }
        return {
          label:
            state.kind === "payment_package"
              ? t("index.catalog.links.importArtifact", { release: `${name}@${version}` })
              : t("index.catalog.links.installArtifact", { release: `${name}@${version}` }),
          url
        };
      })
      .filter((item): item is { label: string; url: string } => Boolean(item));
    if (nativeLinks.length > 0) {
      blocks.push(
        buildPluginLinkListBlock({
          title:
            state.kind === "payment_package"
              ? t("index.catalog.links.openNativeImportFlow")
              : t("index.catalog.links.openNativeInstallFlow"),
          links: nativeLinks
        })
      );
    }
  }

  return blocks;
}

function buildTaskContextLinks(state: MarketConsoleState, items: UnknownMap[]): PluginPageBlock[] {
  if (items.length === 0) {
    return [];
  }
  return [
    buildPluginLinkListBlock({
      title: t("index.task.context.links.title"),
      links: items.slice(0, 8).map((item) => {
        const coordinates = asRecord(item.coordinates);
        const taskID = asString(item.task_id) || "-";
        const name = asString(coordinates.name) || state.name || "artifact";
        const version = resolveVersionText(coordinates.version) || state.version || "-";
        return {
          label: `${name}@${version} / ${taskID}`,
          url: buildArtifactContextURLFromItem(state, item)
        };
      })
    })
  ];
}

function dedupeLinks(
  links: Array<{ label: string; url: string }>
): Array<{ label: string; url: string }> {
  const seen = new Set<string>();
  return links.filter((item) => {
    const normalizedURL = asString(item.url).trim();
    if (!normalizedURL || seen.has(normalizedURL)) {
      return false;
    }
    seen.add(normalizedURL);
    return true;
  });
}

function buildRecommendedActionBlocks(
  primaryAction: string,
  rationale: string,
  links: Array<{ label: string; url: string }> = []
): PluginPageBlock[] {
  const uniqueLinks = dedupeLinks(links);

  const blocks: PluginPageBlock[] = [
    buildPluginKeyValueBlock({
      title: t("index.recommendation.title"),
      items: [
        { label: t("index.recommendation.primary.label"), value: primaryAction },
        { label: t("index.recommendation.rationale.label"), value: rationale }
      ]
    })
  ];

  if (uniqueLinks.length > 0) {
    blocks.push(
      buildPluginLinkListBlock({
        title: t("index.recommendation.links.title"),
        links: uniqueLinks
      })
    );
  }

  return blocks;
}

function buildConsoleLandingGuidanceBlocks(
  state: MarketConsoleState,
  sources: UnknownMap[],
  selectedSource: UnknownMap | null
): PluginPageBlock[] {
  if (sources.length === 0) {
    return buildRecommendedActionBlocks(
      t("index.guidance.landing.noSource.action"),
      t("index.guidance.landing.noSource.rationale")
    );
  }

  if (!selectedSource) {
    return buildRecommendedActionBlocks(
      t("index.guidance.landing.invalidSource.action"),
      t("index.guidance.landing.invalidSource.rationale"),
      sources.slice(0, 6).map((source) => ({
        label: t("index.guidance.landing.invalidSource.useSource", {
          sourceId: asString(source.source_id) || "source"
        }),
        url: buildMarketConsoleURL(state, {
          source_id: asString(source.source_id) || state.source_id,
          q: "",
          name: "",
          version: "",
          task_id: ""
        })
      }))
    );
  }

  return buildRecommendedActionBlocks(
    t("index.guidance.landing.ready.action"),
    t("index.guidance.landing.ready.rationale"),
    [
      {
        label: t("index.guidance.landing.ready.openPlugin"),
        url: buildPackageResetURL(state, { kind: "plugin_package" })
      },
      {
        label: t("index.guidance.landing.ready.openPayment"),
        url: buildPackageResetURL(state, { kind: "payment_package" })
      },
      {
        label: t("index.guidance.landing.ready.openEmail"),
        url: buildTemplateResetURL(state, { kind: "email_template" })
      },
      {
        label: t("index.guidance.landing.ready.openLanding"),
        url: buildTemplateResetURL(state, { kind: "landing_page_template" })
      },
      {
        label: "Open Invoice Template",
        url: buildTemplateResetURL(state, { kind: "invoice_template" })
      },
      {
        label: "Open Auth Branding Template",
        url: buildTemplateResetURL(state, { kind: "auth_branding_template" })
      },
      {
        label: "Open Page Rule Pack",
        url: buildTemplateResetURL(state, { kind: "page_rule_pack" })
      }
    ]
  );
}

function buildPackageLoadGuidanceBlocks(
  state: MarketConsoleState,
  sources: UnknownMap[],
  selectedSource: UnknownMap | null
): PluginPageBlock[] {
  if (sources.length === 0 || !selectedSource) {
    return buildConsoleLandingGuidanceBlocks(state, sources, selectedSource);
  }

  if (state.task_id) {
    return buildRecommendedActionBlocks(
      t("index.guidance.package.task.action", {
        taskId: state.task_id
      }),
      t("index.guidance.package.task.rationale"),
      [
        {
          label: t("index.guidance.package.task.openTask", {
            taskId: state.task_id
          }),
          url: buildMarketConsoleURL(state)
        },
        {
          label: t("index.guidance.package.openHistory"),
          url: buildMarketConsoleURL(state, { task_id: "" })
        }
      ]
    );
  }

  if (!state.name) {
    return buildRecommendedActionBlocks(
      t("index.guidance.package.noName.action", {
        kind: formatMarketKindLabel(state.kind)
      }),
      t("index.guidance.package.noName.rationale"),
      [
        {
          label: t("index.guidance.package.viewSource"),
          url: buildPackageResetURL(state)
        },
        {
          label:
            state.kind === "payment_package"
              ? t("index.workflow.package.switchToPlugin")
              : t("index.workflow.package.switchToPayment"),
          url: buildPackageResetURL(state, {
            kind: state.kind === "payment_package" ? "plugin_package" : "payment_package"
          })
        }
      ]
    );
  }

  if (!state.version) {
    return buildRecommendedActionBlocks(
      t("index.guidance.package.noVersion.action"),
      t("index.guidance.package.noVersion.rationale"),
      [
        {
          label: t("index.guidance.package.viewArtifact"),
          url: buildMarketConsoleURL(state, { version: "", task_id: "" })
        },
        {
          label: t("index.workflow.common.reuse", {
            label: state.name
          }),
          url: buildMarketConsoleURL(state, { task_id: "" })
        }
      ]
    );
  }

  return buildRecommendedActionBlocks(
    t("index.guidance.package.ready.action", {
      release: `${state.name}@${state.version}`
    }),
    state.kind === "payment_package"
      ? t("index.guidance.package.ready.rationale.payment")
      : t("index.guidance.package.ready.rationale.plugin"),
    [
      {
        label: t("index.guidance.package.viewVersion"),
        url: buildMarketConsoleURL(state, { task_id: "" })
      },
      {
        label: t("index.guidance.package.openHistory"),
        url: buildMarketConsoleURL(state, { task_id: "" })
      }
    ]
  );
}

function buildTemplateLoadGuidanceBlocks(
  state: MarketConsoleState,
  sources: UnknownMap[],
  selectedSource: UnknownMap | null
): PluginPageBlock[] {
  if (sources.length === 0 || !selectedSource) {
    return buildConsoleLandingGuidanceBlocks(state, sources, selectedSource);
  }

  const targetKey = getTemplateTargetKey(state);
  if (!targetKey) {
    return buildRecommendedActionBlocks(
      t("index.guidance.template.noTarget.action"),
      t("index.guidance.template.noTarget.rationale"),
      [
        {
          label: t("index.guidance.template.viewSource"),
          url: buildTemplateResetURL(state)
        }
      ]
    );
  }

  if (!state.name) {
    return buildRecommendedActionBlocks(
      t("index.guidance.template.noName.action"),
      t("index.guidance.template.noName.rationale"),
      [
        {
          label: t("index.guidance.template.viewSource"),
          url: buildTemplateResetURL(state)
        }
      ]
    );
  }

  if (!state.version) {
    return buildRecommendedActionBlocks(
      t("index.guidance.template.noVersion.action"),
      t("index.guidance.template.noVersion.rationale"),
      [
        {
          label: t("index.guidance.template.viewArtifact"),
          url: buildMarketConsoleURL(state, { version: "" })
        },
        {
          label: t("index.workflow.common.reuse", {
            label: state.name
          }),
          url: buildMarketConsoleURL(state)
        }
      ]
    );
  }

  return buildRecommendedActionBlocks(
    t("index.guidance.template.ready.action", {
      targetKey,
      release: `${state.name}@${state.version}`
    }),
    t("index.guidance.template.ready.rationale"),
    [
      {
        label: t("index.guidance.template.viewVersion"),
        url: buildMarketConsoleURL(state)
      },
      {
        label: t("index.guidance.package.openHistory"),
        url: buildMarketConsoleURL(state, { task_id: "" })
      }
    ]
  );
}

function buildCollapsedPayloadBlock(
  title: string,
  value: unknown,
  summary: string,
  previewLines = 10
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

function buildLoadBlocks(state: MarketConsoleState, context: PluginExecutionContext): PluginPageBlock[] {
  const sources = sourceList();
  const pageContext = resolvePluginPageContext(context);
  const selectedSource = findSourceSummary(state.source_id, sources);
  const blocks: PluginPageBlock[] = [
    buildPluginStatsGridBlock({
      title: t("index.load.overview.sourceStatus.title"),
      items: [
        { label: t("index.load.overview.sourceStatus.configuredSources.label"), value: sources.length },
        { label: t("index.load.overview.sourceStatus.currentSource.label"), value: state.source_id || "-" },
        { label: t("index.load.overview.sourceStatus.currentKind.label"), value: state.kind },
        { label: t("index.load.overview.sourceStatus.revisionBackend.label"), value: getRevisionBackendLabel() }
      ]
    }),
    buildPluginKeyValueBlock({
      title: t("index.load.overview.currentContext.title"),
      items: [
        { label: t("index.load.overview.currentContext.adminRoute.label"), value: ADMIN_PLUGIN_PAGE_PATH },
        { label: t("index.load.overview.currentContext.currentPath.label"), value: pageContext.full_path || ADMIN_PLUGIN_PAGE_PATH },
        { label: t("index.load.overview.currentContext.queryParams.label"), value: prettyJSON(pageContext.query_params) },
        { label: t("index.load.overview.currentContext.routeParams.label"), value: prettyJSON(pageContext.route_params) },
        {
          label: t("index.load.overview.currentContext.selectedSourceSummary.label"),
          value:
            selectedSource
              ? prettyJSON(selectedSource)
              : t("index.load.overview.currentContext.selectedSourceSummary.missing")
        }
      ]
    }),
    buildPluginTableBlock({
      title: t("index.load.overview.trustedSources.title"),
      columns: ["source_id", "name", "base_url", "default_channel", "allowed_kinds"],
      rows: sources.map((source) => ({
        source_id: asString(source.source_id),
        name: asString(source.name),
        base_url: asString(source.base_url),
        default_channel: asString(source.default_channel),
        allowed_kinds: safeArray(source.allowed_kinds).join(", ")
      })),
      empty_text: t("index.load.overview.trustedSources.empty")
    })
  ];

  blocks.push(...buildConsoleLandingGuidanceBlocks(state, sources, selectedSource));

  if (isTemplateMarketKind(state.kind)) {
    blocks.push(...buildTemplateTargetInsightBlocks(state));
  }

  if (state.kind === "email_template") {
    try {
      const emailTargets = safeObjectArray(asRecord(emailTemplateAPI().list()).items);
      blocks.push(
        buildPluginTableBlock({
          title: t("index.emailTargets.title"),
          columns: ["key", "event", "locale", "filename", "updated_at"],
          rows: emailTargets.map((item) => ({
            key: asString(item.key),
            event: asString(item.event),
            locale: asString(item.locale) || "-",
            filename: asString(item.filename),
            updated_at: asString(item.updated_at) || "-"
          })),
          empty_text: t("index.emailTargets.empty")
        })
      );
    } catch (error) {
      blocks.push(
        buildPluginAlertBlock({
          title: t("index.emailTargets.title"),
          content: t("index.emailTargets.failed", {
            error: errorToString(error)
          }),
          variant: "warning"
        })
      );
    }
  }

  return blocks;
}

function buildPackageLoadBlocks(
  state: MarketConsoleState,
  context: PluginExecutionContext
): PluginPageBlock[] {
  const pageContext = resolvePluginPageContext(context);
  const sources = sourceList();
  const selectedSource = findSourceSummary(state.source_id, sources);
  return [
    buildPluginAlertBlock({
      title: t("index.load.package.title"),
      content:
        state.kind === "payment_package"
          ? t("index.load.package.content.payment")
          : t("index.load.package.content.plugin"),
      variant: "info"
    }),
    buildPluginKeyValueBlock({
      title: t("index.load.package.context.title"),
      items: [
        { label: t("index.load.context.currentPath.label"), value: pageContext.full_path || ADMIN_PLUGIN_PAGE_PATH },
        { label: t("index.load.context.sourceId.label"), value: state.source_id || "-" },
        { label: t("index.load.context.kind.label"), value: state.kind },
        { label: t("index.load.context.name.label"), value: state.name || "-" },
        { label: t("index.load.context.version.label"), value: state.version || "-" },
        { label: t("index.load.context.taskId.label"), value: state.task_id || "-" }
      ]
    }),
    buildSelectedSourceSummaryBlock(state, sources, t("index.load.context.selectedSource.label")),
    ...buildPackageLoadGuidanceBlocks(state, sources, selectedSource),
    ...buildPackageWorkflowBlocks(state)
  ];
}

function buildTemplateLoadBlocks(
  state: MarketConsoleState,
  context: PluginExecutionContext
): PluginPageBlock[] {
  const pageContext = resolvePluginPageContext(context);
  const sources = sourceList();
  const selectedSource = findSourceSummary(state.source_id, sources);
  const targetKey = getTemplateTargetKey(state);
  const targetLabel = getTemplateTargetLabel(state);
  return [
    buildPluginAlertBlock({
      title: t("index.load.template.title"),
      content: t("index.load.template.content"),
      variant: "info"
    }),
    buildPluginKeyValueBlock({
      title: t("index.load.template.context.title"),
      items: [
        { label: t("index.load.context.currentPath.label"), value: pageContext.full_path || ADMIN_PLUGIN_PAGE_PATH },
        { label: t("index.load.context.sourceId.label"), value: state.source_id || "-" },
        { label: t("index.load.context.kind.label"), value: state.kind },
        { label: t("index.load.context.name.label"), value: state.name || "-" },
        { label: targetLabel, value: targetKey || "-" },
        { label: t("index.load.context.version.label"), value: state.version || "-" }
      ]
    }),
    buildSelectedSourceSummaryBlock(state, sources, t("index.load.context.selectedSource.label")),
    ...buildTemplateLoadGuidanceBlocks(state, sources, selectedSource),
    ...buildTemplateWorkflowBlocks(state),
    ...buildTemplateTargetInsightBlocks(state)
  ];
}

function executeConsoleLoad(
  params: UnknownMap,
  config: UnknownMap,
  context: PluginExecutionContext
): PluginExecuteResult {
  const state = setWorkflowStage(buildConsoleState(params, config, context), "source");
  return successResult({
    source: "market.console.load",
    message: t("index.message.consoleLoaded"),
    values: state as unknown as UnknownMap,
    blocks: buildLoadBlocks(state, context)
  });
}

function executePackageConsoleLoad(
  params: UnknownMap,
  config: UnknownMap,
  context: PluginExecutionContext
): PluginExecuteResult {
  const baseState = buildPackageModeState(buildConsoleState(params, config, context));
  const state = setWorkflowStage(baseState, inferWorkflowStage(baseState.kind, baseState));
  return successResult({
    source: "market.package.load",
    message: t("index.message.packageLoaded"),
    values: state as unknown as UnknownMap,
    blocks: buildPackageLoadBlocks(state, context)
  });
}

function executePackageConsoleReset(
  params: UnknownMap,
  config: UnknownMap,
  context: PluginExecutionContext
): PluginExecuteResult {
  const state = buildPackageResetState(buildPackageModeState(buildConsoleState(params, config, context)));
  setWorkflowStage(state, "source");
  return successResult({
    source: "market.package.reset",
    message: t("index.message.packageReset"),
    values: state as unknown as UnknownMap,
    blocks: buildPackageLoadBlocks(state, context)
  });
}

function executeTemplateConsoleLoad(
  params: UnknownMap,
  config: UnknownMap,
  context: PluginExecutionContext
): PluginExecuteResult {
  const baseState = buildTemplateModeState(buildConsoleState(params, config, context));
  const state = setWorkflowStage(baseState, inferWorkflowStage(baseState.kind, baseState));
  return successResult({
    source: "market.template.load",
    message: t("index.message.templateLoaded"),
    values: state as unknown as UnknownMap,
    blocks: buildTemplateLoadBlocks(state, context)
  });
}

function executeTemplateConsoleReset(
  params: UnknownMap,
  config: UnknownMap,
  context: PluginExecutionContext
): PluginExecuteResult {
  const state = buildTemplateResetState(buildTemplateModeState(buildConsoleState(params, config, context)));
  setWorkflowStage(state, "source");
  return successResult({
    source: "market.template.reset",
    message: t("index.message.templateReset"),
    values: state as unknown as UnknownMap,
    blocks: buildTemplateLoadBlocks(state, context)
  });
}

function executeSourceDetail(
  params: UnknownMap,
  config: UnknownMap,
  context: PluginExecutionContext
): PluginExecuteResult {
  const state = setWorkflowStage(buildConsoleState(params, config, context), "source");
  if (!state.source_id) {
    return errorResult("source_id is required");
  }

  const detail = asRecord(
    marketAPI().source?.get({
      source_id: state.source_id
    }) || {}
  );

  return successResult({
    source: "host.market.source.get",
    message: t("index.message.sourceDetailLoaded", {
      sourceId: state.source_id
    }),
    values: state as unknown as UnknownMap,
    blocks: buildSourceDetailBlocks(detail, state)
  });
}

function executeCatalogQuery(
  params: UnknownMap,
  config: UnknownMap,
  context: PluginExecutionContext
): PluginExecuteResult {
  const state = buildConsoleState(params, config, context);
  setWorkflowStage(
    state,
    state.name ? (state.version ? "release" : "artifact") : "catalog"
  );
  if (!state.source_id) {
    return errorResult("source_id is required");
  }

  const catalog = asRecord(
    marketAPI().catalog?.list({
      source_id: state.source_id,
      kind: state.kind,
      channel: state.channel,
      q: state.q,
      limit: 20,
      offset: 0
    }) || {}
  );
  const items = safeObjectArray(catalog.items);
  const pagination = asRecord(catalog.pagination);
  const source = asRecord(catalog.source);
  const publishers = uniqueDisplayStrings(
    items.map((item) => asString(asRecord(item.publisher).name || asRecord(item.publisher).id))
  );
  const channels = uniqueDisplayStrings(items.map((item) => asString(item.channel)));
  const totalItems = asInteger(pagination.total, items.length);
  const blocks: PluginPageBlock[] = [
    buildPluginAlertBlock({
      title: t("index.catalog.title"),
      content:
        items.length === 0
          ? t("index.catalog.content.empty")
          : state.name
            ? t("index.catalog.content.named", {
                count: items.length,
                name: state.name
              })
            : t("index.catalog.content.default"),
      variant: items.length === 0 ? "warning" : "success"
    }),
    buildPluginStatsGridBlock({
      title: t("index.catalog.overview.title"),
      items: [
        { label: t("index.catalog.overview.returnedCount.label"), value: items.length },
        { label: t("index.catalog.overview.totalCount.label"), value: totalItems },
        { label: t("index.catalog.overview.publisherCount.label"), value: publishers.length },
        { label: t("index.catalog.overview.channelCount.label"), value: channels.length },
        { label: t("index.catalog.overview.hasMore.label"), value: formatBooleanState(pagination.has_more) },
        { label: t("index.catalog.overview.kind.label"), value: state.kind },
        { label: t("index.catalog.overview.currentArtifact.label"), value: state.name || "-" }
      ]
    }),
    buildPluginKeyValueBlock({
      title: t("index.catalog.context.title"),
      items: [
        { label: t("index.sourceSummary.sourceId.label"), value: state.source_id },
        { label: t("index.sourceSummary.sourceName.label"), value: asString(source.name) || asString(source.source_id) || "-" },
        { label: t("index.catalog.context.sourceBaseUrl.label"), value: asString(source.base_url) || "-" },
        { label: t("index.catalog.context.channel.label"), value: state.channel || "-" },
        { label: t("index.catalog.context.keyword.label"), value: state.q || "-" },
        { label: t("index.catalog.overview.currentArtifact.label"), value: state.name || "-" }
      ]
    }),
    ...(channels.length > 0
      ? [
          buildPluginBadgeListBlock({
            title: t("index.catalog.channels.title"),
            items: channels
          })
        ]
      : []),
    buildPluginTableBlock({
      title: t("index.catalog.table.title"),
      columns: [
        "kind",
        "name",
        "title",
        "latest_version",
        "channel",
        "publisher",
        "published_at"
      ],
      rows: items.map((item) => ({
        kind: asString(item.kind),
        name: asString(item.name),
        title: asString(item.title),
        latest_version: asString(item.latest_version),
        channel: asString(item.channel),
        publisher: asString(asRecord(item.publisher).name || asRecord(item.publisher).id),
        published_at: asString(item.published_at)
      })),
      empty_text: t("index.catalog.table.empty")
    }),
    ...buildCatalogSelectionLinks(state, items, source)
  ];

  if (items.length === 0) {
    blocks.push(
      ...buildRecommendedActionBlocks(
        t("index.catalog.empty.action"),
        state.q
          ? t("index.catalog.empty.rationale.keyword")
          : t("index.catalog.empty.rationale.default"),
        [
          {
            label: t("index.catalog.empty.clearKeyword"),
            url: buildMarketConsoleURL(state, { q: "", task_id: "" })
          },
          {
            label: t("index.catalog.empty.clearArtifactVersion"),
            url: buildMarketConsoleURL(state, { name: "", version: "", task_id: "" })
          },
          {
            label: t("index.catalog.empty.backToSource"),
            url: buildMarketConsoleURL(state, { task_id: "" })
          }
        ]
      )
    );
  }

  if (state.name) {
    try {
      const artifact = asRecord(
        marketAPI().artifact?.get({
          source_id: state.source_id,
          kind: state.kind,
          name: state.name
        }) || {}
      );
      blocks.push(...buildArtifactDetailBlocks(artifact, state));
    } catch (error) {
      blocks.push(
        buildPluginAlertBlock({
          title: t("index.artifactDetail.title"),
          content: t("index.artifactDetail.failed", {
            error: errorToString(error)
          }),
          variant: "warning"
        })
      );
    }
  }

  return successResult({
    source: "host.market.catalog.list",
    message: `Loaded ${items.length} catalog item(s).`,
    values: state as unknown as UnknownMap,
    blocks,
    browser: buildCatalogBrowserPayload(state, source, pagination, items),
  });
}

function executeArtifactDetail(
  params: UnknownMap,
  config: UnknownMap,
  context: PluginExecutionContext
): PluginExecuteResult {
  const state = setWorkflowStage(buildConsoleState(params, config, context), "artifact");
  if (!state.source_id) {
    return errorResult("source_id is required");
  }
  if (!state.name) {
    return errorResult("name is required");
  }

  const artifact = asRecord(
    marketAPI().artifact?.get({
      source_id: state.source_id,
      kind: state.kind,
      name: state.name
    }) || {}
  );

  return successResult({
    source: "host.market.artifact.get",
    message: t("index.message.artifactDetailLoaded", {
      name: state.name
    }),
    values: state as unknown as UnknownMap,
    blocks: buildArtifactDetailBlocks(artifact, state)
  });
}

function executeReleaseDetail(
  params: UnknownMap,
  config: UnknownMap,
  context: PluginExecutionContext
): PluginExecuteResult {
  const state = setWorkflowStage(buildConsoleState(params, config, context), "release");
  if (!state.source_id) {
    return errorResult("source_id is required");
  }
  if (!state.name) {
    return errorResult("name is required");
  }

  const version = resolveSelectedVersion(state);
  state.version = version;

  const release = asRecord(
    marketAPI().release?.get({
      source_id: state.source_id,
      kind: state.kind,
      name: state.name,
      version
    }) || {}
  );

  return successResult({
    source: "host.market.release.get",
    message: t("index.message.releaseDetailLoaded", {
      release: `${state.name}@${version}`
    }),
    values: state as unknown as UnknownMap,
    blocks: buildReleaseDetailBlocks(release, state)
  });
}

function executeReleasePreview(
  params: UnknownMap,
  config: UnknownMap,
  context: PluginExecutionContext
): PluginExecuteResult {
  const state = setWorkflowStage(buildConsoleState(params, config, context), "preview");
  if (!state.source_id) {
    return errorResult("source_id is required");
  }
  if (!state.name) {
    return errorResult("name is required");
  }

  const version = resolveSelectedVersion(state);
  state.version = version;

  if (state.kind === "plugin_package" || state.kind === "payment_package") {
    const preview = asRecord(
      marketAPI().install?.preview({
        source_id: state.source_id,
        kind: state.kind,
        name: state.name,
        version
      }) || {}
    );
    if (state.kind === "plugin_package" && normalizeStringArray(state.granted_permissions_json).length === 0) {
      const recommendedPermissions = resolvePluginPackageRecommendedGrantedFromPreview(preview);
      if (recommendedPermissions.length > 0) {
        state.granted_permissions_json = stringifyStringArray(recommendedPermissions);
      }
    }
    return successResult({
      source: "host.market.install.preview",
      message:
        state.kind === "payment_package"
          ? t("index.message.paymentPreviewLoaded")
          : t("index.message.pluginPreviewLoaded"),
      values: state as unknown as UnknownMap,
      blocks:
        state.kind === "payment_package"
          ? buildPaymentPackagePreviewBlocks(preview, state)
          : buildPluginPackagePreviewBlocks(preview, state)
    });
  }

  if (isTemplateMarketKind(state.kind)) {
    const preview = asRecord(
      marketAPI().install?.preview({
        source_id: state.source_id,
        kind: state.kind,
        name: state.name,
        version,
        ...buildTemplateMarketTargetParams(state)
      }) || {}
    );
    syncTemplateTargetState(state, preview);
    return successResult({
      source: "host.market.install.preview",
      message: t("index.message.templatePreviewLoaded"),
      values: state as unknown as UnknownMap,
      blocks: [...buildTemplateMarketPreviewBlocks(preview, state), ...buildTemplateTargetInsightBlocks(state)]
    });
  }

  return errorResult("unsupported market preview kind");
}

function executeInstallOrImport(
  params: UnknownMap,
  config: UnknownMap,
  context: PluginExecutionContext
): PluginExecuteResult {
  const state = buildConsoleState(params, config, context);
  if (!state.source_id) {
    return errorResult("source_id is required");
  }
  if (!state.name) {
    return errorResult("name is required");
  }

  const version = resolveSelectedVersion(state);
  state.version = version;

  if (state.kind === "payment_package") {
    setWorkflowStage(state, "preview");
    const preview = asRecord(
      marketAPI().install?.preview({
        source_id: state.source_id,
        kind: state.kind,
        name: state.name,
        version
      }) || {}
    );
    return successResult({
      source: "host.market.install.preview",
      message: t("index.message.paymentImportFlow"),
      values: state as unknown as UnknownMap,
      blocks: buildPaymentPackagePreviewBlocks(preview, state)
    });
  }

  if (state.kind === "plugin_package") {
    let grantedPermissions = normalizeStringArray(state.granted_permissions_json);
    if (grantedPermissions.length === 0) {
      const release = asRecord(
        marketAPI().release?.get({
          source_id: state.source_id,
          kind: state.kind,
          name: state.name,
          version
        }) || {}
      );
      grantedPermissions = resolvePluginPackageRecommendedGrantedFromRelease(release);
    }
    if (grantedPermissions.length > 0) {
      state.granted_permissions_json = stringifyStringArray(grantedPermissions);
    }
    const result = asRecord(
      marketAPI().install?.execute({
        source_id: state.source_id,
        kind: state.kind,
        name: state.name,
        version,
        options: {
          activate: state.activate,
          auto_start: state.auto_start,
          granted_permissions: grantedPermissions,
          note: state.note || undefined
        }
      }) || {}
    );
    if (asString(result.task_id)) {
      state.task_id = asString(result.task_id);
    }
    setWorkflowStage(state, state.task_id ? "task" : "history");
    return successResult({
      source: "host.market.install.execute",
      message: t("index.message.pluginInstallSubmitted"),
      values: {
        ...(state as unknown as UnknownMap),
        granted_permissions_json: stringifyStringArray(grantedPermissions)
      },
      blocks: buildPluginPackageInstallBlocks(result, state)
    });
  }

  if (isTemplateMarketKind(state.kind)) {
    const result = asRecord(
      marketAPI().install?.execute({
        source_id: state.source_id,
        kind: state.kind,
        name: state.name,
        version,
        note: state.note || undefined,
        ...buildTemplateMarketTargetParams(state)
      }) || {}
    );
    syncTemplateTargetState(state, result);
    setWorkflowStage(state, "history");
    return successResult({
      source: "host.market.install.execute",
      message: t("index.message.templateImported", {
        kind: state.kind
      }),
      values: state as unknown as UnknownMap,
      blocks: [
        ...buildTemplateHostInstallBlocks(result, state),
        ...buildTemplateTargetInsightBlocks(state)
      ]
    });
  }

  return errorResult("unsupported market install kind");
}

function resolveLatestTaskID(state: MarketConsoleState): string {
  if (state.task_id) {
    return state.task_id;
  }
  const taskAPI = marketAPI().install?.task;
  if (!taskAPI || !state.name) {
    return "";
  }
  const listResult = asRecord(
    taskAPI.list({
      source_id: state.source_id,
      kind: state.kind,
      name: state.name,
      version: state.version,
      limit: 1
    }) || {}
  );
  const items = safeObjectArray(listResult.items);
  if (items.length === 0) {
    return "";
  }
  return asString(items[0].task_id);
}

function executeTaskGet(
  params: UnknownMap,
  config: UnknownMap,
  context: PluginExecutionContext
): PluginExecuteResult {
  const state = setWorkflowStage(buildConsoleState(params, config, context), "task");
  if (state.kind !== "plugin_package") {
    return successResult({
      source: "market.install.task.get",
      message: t("index.message.taskOnlyPlugin"),
      values: state as unknown as UnknownMap,
      blocks: [
        buildPluginAlertBlock({
          title: t("index.task.none.title"),
          content: t("index.task.none.content.instant"),
          variant: "info"
        })
      ]
    });
  }

  const taskID = resolveLatestTaskID(state);
  if (!taskID) {
    return errorResult("task_id is required or no matching task could be resolved from the current coordinates");
  }
  state.task_id = taskID;

  const task = asRecord(
    marketAPI().install?.task?.get({
      task_id: taskID
    }) || {}
  );
  return successResult({
    source: "host.market.install.task.get",
    message: t("index.message.taskLoaded", {
      taskId: taskID
    }),
    values: state as unknown as UnknownMap,
    blocks: buildTaskBlocks(task, state)
  });
}

function executeTaskList(
  params: UnknownMap,
  config: UnknownMap,
  context: PluginExecutionContext
): PluginExecuteResult {
  const state = setWorkflowStage(buildConsoleState(params, config, context), "task");
  if (state.kind !== "plugin_package") {
    return successResult({
      source: "market.install.task.list",
      message: t("index.message.taskListOnlyPlugin"),
      values: state as unknown as UnknownMap,
      blocks: [
        buildPluginAlertBlock({
          title: t("index.task.none.title"),
          content: t("index.task.none.content.pluginOnly"),
          variant: "info"
        })
      ]
    });
  }

  const result = asRecord(
    marketAPI().install?.task?.list({
      source_id: state.source_id,
      kind: state.kind,
      name: state.name || undefined,
      version: state.version || undefined,
      limit: 20
    }) || {}
  );
  const items = safeObjectArray(result.items);
  if (items.length > 0 && !state.task_id) {
    state.task_id = asString(items[0].task_id);
  }
  return successResult({
    source: "host.market.install.task.list",
    message: t("index.message.taskListLoaded", {
      count: items.length
    }),
    values: state as unknown as UnknownMap,
    blocks: buildTaskListBlocks(items, asRecord(result.pagination), state)
  });
}

function executeHistoryList(
  params: UnknownMap,
  config: UnknownMap,
  context: PluginExecutionContext
): PluginExecuteResult {
  const state = setWorkflowStage(buildConsoleState(params, config, context), "history");
  if (
    state.kind === "plugin_package" ||
    state.kind === "payment_package" ||
    isTemplateMarketKind(state.kind)
  ) {
    const result = asRecord(
      marketAPI().install?.history?.list({
        source_id: state.source_id,
        kind: state.kind,
        name: state.name || undefined,
        version: state.version || undefined,
        ...buildTemplateMarketTargetParams(state),
        limit: 20
      }) || {}
    );
    const items = safeObjectArray(result.items);
      return successResult({
        source: "host.market.install.history.list",
        message: t("index.message.historyLoaded", {
          count: items.length
        }),
        values: state as unknown as UnknownMap,
        blocks: [
          ...buildInstallHistoryBlocks(items, asRecord(result.pagination), state),
        ...(isTemplateMarketKind(state.kind) ? buildTemplateTargetInsightBlocks(state) : [])
      ]
    });
  }
  return errorResult("unsupported market history kind");
}

function executeRollback(
  params: UnknownMap,
  config: UnknownMap,
  context: PluginExecutionContext
): PluginExecuteResult {
  const state = buildConsoleState(params, config, context);
  if (
    state.kind === "plugin_package" ||
    state.kind === "payment_package" ||
    isTemplateMarketKind(state.kind)
  ) {
    if (!state.source_id || !state.name || !state.version) {
      return errorResult("source_id, name, and version are required for host-managed rollback");
    }
    const result = asRecord(
      marketAPI().install?.rollback({
        source_id: state.source_id,
        kind: state.kind,
        name: state.name,
        version: state.version,
        ...buildTemplateMarketTargetParams(state),
        auto_start: state.auto_start,
        note: state.note || undefined
      }) || {}
    );
    if (asString(result.task_id)) {
      state.task_id = asString(result.task_id);
    }
    syncTemplateTargetState(state, result);
    setWorkflowStage(
      state,
      state.kind === "plugin_package" && state.task_id ? "task" : "rollback"
    );
    return successResult({
      source: "host.market.install.rollback",
      message:
        state.kind === "payment_package"
          ? t("index.message.paymentRollbackExecuted")
          : isTemplateMarketKind(state.kind)
            ? t("index.message.templateRollbackExecuted")
            : t("index.message.pluginRollbackSubmitted"),
      values: state as unknown as UnknownMap,
      blocks: [
        ...buildPluginPackageRollbackBlocks(result, state),
        ...(isTemplateMarketKind(state.kind) ? buildTemplateTargetInsightBlocks(state) : [])
      ]
    });
  }
  return errorResult("unsupported market rollback kind");
}

function buildPluginPackagePreviewBlocks(preview: UnknownMap, state: MarketConsoleState): PluginPageBlock[] {
  const coordinates = asRecord(preview.coordinates);
  const source = asRecord(preview.source);
  const release = asRecord(preview.release);
  const permissions = asRecord(preview.permissions);
  const targetState = asRecord(preview.target_state);
  const compatibility = asRecord(preview.compatibility);
  const requestedPermissions = normalizeStringArray(prettyJSONArray(permissions.requested));
  const newPermissions = normalizeStringArray(prettyJSONArray(permissions.new_permissions));
  const warnings = uniqueDisplayStrings(
    normalizeDisplayStringArray(preview.warnings).concat(normalizeDisplayStringArray(compatibility.warnings))
  );
  const compatibilityWarnings = normalizeDisplayStringArray(compatibility.warnings);
  const installMode = resolvePackageInstallMode(targetState);
  const nativeInstallURL = buildNativeMarketInstallURL(preview);
  const releaseName = asString(release.title) || asString(coordinates.name) || "-";
  const releaseVersion = asString(coordinates.version) || "-";
  const compatibilityLabel =
    compatibility.compatible === false
      ? t("index.releaseDetail.overview.hostCompatibility.rejected")
      : t("index.releaseDetail.overview.hostCompatibility.ready");
  const blocks: PluginPageBlock[] = [
    buildPluginAlertBlock({
      title: t("index.pluginPreview.title"),
      content:
        compatibility.compatible === false
          ? t("index.pluginPreview.content.incompatible")
          : newPermissions.length > 0
            ? t("index.pluginPreview.content.newPermissions")
            : targetState.update_available === true
              ? t("index.pluginPreview.content.upgrade")
              : t("index.pluginPreview.content.ready"),
      variant:
        compatibility.compatible === false ? "warning" : warnings.length > 0 || newPermissions.length > 0 ? "info" : "success"
    }),
    buildPluginStatsGridBlock({
      title: t("index.preview.common.overview.title"),
      items: [
        { label: t("index.pluginPreview.overview.package.label"), value: releaseName },
        { label: t("index.load.context.version.label"), value: releaseVersion },
        { label: t("index.pluginPreview.overview.installMode.label"), value: installMode },
        { label: t("index.preview.common.currentVersion.label"), value: asString(targetState.current_version) || "-" },
        { label: t("index.pluginPreview.overview.requestedPermissions.label"), value: requestedPermissions.length },
        { label: t("index.pluginPreview.overview.newPermissions.label"), value: newPermissions.length },
        { label: t("index.preview.common.warningCount.label"), value: warnings.length },
        { label: t("index.preview.common.hostCompatibility.label"), value: compatibilityLabel }
      ]
    }),
    buildPluginKeyValueBlock({
      title: t("index.pluginPreview.context.title"),
      items: [
        { label: t("index.sourceSummary.sourceId.label"), value: asString(source.source_id) || asString(coordinates.source_id) || "-" },
        { label: t("index.sourceSummary.sourceName.label"), value: asString(source.name) || "-" },
        { label: t("index.load.context.kind.label"), value: asString(coordinates.kind) || "plugin_package" },
        { label: t("index.pluginPreview.context.packageName.label"), value: asString(coordinates.name) || "-" },
        { label: t("index.preview.common.runtime.label"), value: asString(compatibility.runtime) || "-" },
        { label: t("index.preview.common.requiredBridge.label"), value: asString(compatibility.min_host_bridge_version) || "-" },
        { label: t("index.preview.common.currentBridge.label"), value: asString(compatibility.host_bridge_version) || "-" }
      ]
    }),
    buildPluginKeyValueBlock({
      title: t("index.pluginPreview.impact.title"),
      items: [
        { label: t("index.pluginPreview.impact.installed.label"), value: formatBooleanState(targetState.installed) },
        { label: t("index.pluginPreview.impact.updateAvailable.label"), value: formatBooleanState(targetState.update_available) },
        { label: t("index.pluginPreview.impact.installTarget.label"), value: asString(targetState.installed_target) || "plugin" },
        { label: t("index.pluginPreview.impact.targetId.label"), value: asString(targetState.installed_target_id) || "-" },
        {
          label: t("index.pluginPreview.impact.compatibilityNotes.label"),
          value: compatibilityWarnings.length > 0 ? compatibilityWarnings.join("; ") : "-"
        }
      ]
    }),
    ...(requestedPermissions.length > 0
      ? [
          buildPluginBadgeListBlock({
            title: t("index.releaseDetail.permissions.requested.title"),
            items: requestedPermissions
          })
        ]
      : [
          buildPluginAlertBlock({
            title: t("index.releaseDetail.permissions.requested.title"),
            content: t("index.pluginPreview.requestedPermissions.none"),
            variant: "success"
          })
        ]),
    ...(newPermissions.length > 0
      ? [
          buildPluginBadgeListBlock({
            title: t("index.pluginPreview.newPermissions.title"),
            items: newPermissions
          })
        ]
      : [
          buildPluginAlertBlock({
            title: t("index.pluginPreview.newPermissions.noneTitle"),
            content: t("index.pluginPreview.newPermissions.none"),
            variant: "info"
          })
        ]),
    ...(warnings.length > 0
      ? [
          buildPluginBadgeListBlock({
            title: t("index.preview.common.warningTitle"),
            items: warnings
          })
        ]
      : [
          buildPluginAlertBlock({
            title: t("index.preview.common.warningTitle"),
            content: t("index.pluginPreview.warnings.none"),
            variant: "success"
          })
        ]),
    ...buildRecommendedActionBlocks(
      compatibility.compatible === false
        ? t("index.pluginPreview.guidance.incompatible.action")
        : t("index.pluginPreview.guidance.ready.action"),
      compatibility.compatible === false
        ? t("index.pluginPreview.guidance.incompatible.rationale")
        : newPermissions.length > 0
          ? t("index.pluginPreview.guidance.ready.permissions")
          : targetState.update_available === true
            ? t("index.pluginPreview.guidance.ready.upgrade")
            : t("index.pluginPreview.guidance.ready.default"),
      [
        {
          label: t("index.preview.common.backToSource"),
          url: buildMarketConsoleURL(state, { name: "", version: "", task_id: "" })
        },
        {
          label: t("index.common.backToArtifactContext"),
          url: buildMarketConsoleURL(state, { version: "", task_id: "" })
        },
        {
          label: t("index.workflow.common.reuse", {
            label: `${state.name || asString(coordinates.name) || "artifact"}@${state.version || asString(coordinates.version) || "-"}`
          }),
          url: buildMarketConsoleURL(state, { task_id: "" })
        },
        {
          label: t("index.preview.common.openAdminInstall"),
          url: nativeInstallURL
        }
      ]
    )
  ];
  blocks.push(
    buildCollapsedPayloadBlock(
      t("index.preview.common.rawPayload.title"),
      preview,
      t("index.pluginPreview.payload.summary")
    )
  );
  return blocks;
}

function buildPaymentPackagePreviewBlocks(preview: UnknownMap, state: MarketConsoleState): PluginPageBlock[] {
  const coordinates = asRecord(preview.coordinates);
  const source = asRecord(preview.source);
  const targetState = asRecord(preview.target_state);
  const compatibility = asRecord(preview.compatibility);
  const resolved = asRecord(preview.resolved);
  const warnings = uniqueDisplayStrings(
    normalizeDisplayStringArray(preview.warnings).concat(normalizeDisplayStringArray(compatibility.warnings))
  );
  const nativeInstallURL = buildNativeMarketInstallURL(preview);
  const compatibilityLabel =
    compatibility.compatible === false
      ? t("index.releaseDetail.overview.hostCompatibility.rejected")
      : t("index.releaseDetail.overview.hostCompatibility.ready");
  const blocks: PluginPageBlock[] = [
    buildPluginAlertBlock({
      title: t("index.paymentPreview.title"),
      content:
        compatibility.compatible === false
          ? t("index.paymentPreview.content.incompatible")
          : targetState.installed === true
            ? t("index.paymentPreview.content.existing")
            : t("index.paymentPreview.content.new"),
      variant: compatibility.compatible === false ? "warning" : warnings.length > 0 ? "info" : "success"
    }),
    buildPluginStatsGridBlock({
      title: t("index.preview.common.overview.title"),
      items: [
        { label: t("index.pluginPreview.overview.package.label"), value: asString(coordinates.name) || "-" },
        { label: t("index.load.context.version.label"), value: resolveVersionText(coordinates.version) || "-" },
        { label: t("index.pluginPreview.impact.installed.label"), value: formatBooleanState(targetState.installed) },
        { label: t("index.preview.common.currentVersion.label"), value: asString(targetState.current_version) || "-" },
        { label: t("index.pluginPreview.impact.updateAvailable.label"), value: formatBooleanState(targetState.update_available) },
        { label: t("index.paymentPreview.overview.scriptBytes.label"), value: asInteger(resolved.script_bytes, 0) },
        { label: t("index.preview.common.warningCount.label"), value: warnings.length },
        { label: t("index.preview.common.hostCompatibility.label"), value: compatibilityLabel }
      ]
    }),
    buildPluginKeyValueBlock({
      title: t("index.paymentPreview.context.title"),
      items: [
        { label: t("index.sourceSummary.sourceId.label"), value: asString(source.source_id) || asString(coordinates.source_id) || "-" },
        { label: t("index.sourceSummary.sourceName.label"), value: asString(source.name) || "-" },
        { label: t("index.load.context.kind.label"), value: asString(coordinates.kind) || "payment_package" },
        { label: t("index.paymentPreview.context.packageName.label"), value: asString(coordinates.name) || "-" },
        { label: t("index.preview.common.requiredBridge.label"), value: asString(compatibility.min_host_bridge_version) || "-" },
        { label: t("index.preview.common.currentBridge.label"), value: asString(compatibility.host_bridge_version) || "-" }
      ]
    }),
    buildPluginKeyValueBlock({
      title: t("index.paymentPreview.defaults.title"),
      items: [
        { label: t("index.paymentPreview.defaults.name.label"), value: asString(resolved.name) || "-" },
        { label: t("index.paymentPreview.defaults.entry.label"), value: asString(resolved.entry) || "-" }
      ]
    }),
    buildPluginKeyValueBlock({
      title: t("index.paymentPreview.impact.title"),
      items: [
        { label: t("index.paymentPreview.impact.icon.label"), value: asString(resolved.icon) || "-" },
        { label: t("index.paymentPreview.impact.pollInterval.label"), value: String(asInteger(resolved.poll_interval, 0)) || "-" },
        { label: t("index.paymentPreview.impact.scriptBytes.label"), value: String(asInteger(resolved.script_bytes, 0)) || "-" },
        { label: t("index.paymentPreview.impact.checksum.label"), value: asString(resolved.checksum) || "-" },
        { label: t("index.paymentPreview.impact.importTarget.label"), value: asString(targetState.installed_target) || "payment_method" },
        { label: t("index.paymentPreview.impact.targetId.label"), value: asString(targetState.installed_target_id) || "-" }
      ]
    }),
    ...(warnings.length > 0
      ? [
          buildPluginBadgeListBlock({
            title: t("index.preview.common.warningTitle"),
            items: warnings
          })
        ]
      : [
          buildPluginAlertBlock({
            title: t("index.preview.common.warningTitle"),
            content: t("index.paymentPreview.warnings.none"),
            variant: "success"
          })
        ]),
    ...buildRecommendedActionBlocks(
      compatibility.compatible === false
        ? t("index.paymentPreview.guidance.incompatible.action")
        : t("index.paymentPreview.guidance.ready.action"),
      compatibility.compatible === false
        ? t("index.paymentPreview.guidance.incompatible.rationale")
        : targetState.installed === true
          ? t("index.paymentPreview.guidance.ready.existing")
          : t("index.paymentPreview.guidance.ready.default"),
      [
        {
          label: t("index.preview.common.backToSource"),
          url: buildMarketConsoleURL(state, { name: "", version: "", task_id: "" })
        },
        {
          label: t("index.common.backToArtifactContext"),
          url: buildMarketConsoleURL(state, { version: "", task_id: "" })
        },
        {
          label: t("index.workflow.common.reuse", {
            label: `${state.name || asString(coordinates.name) || "payment-package"}@${state.version || asString(coordinates.version) || "-"}`
          }),
          url: buildMarketConsoleURL(state, { task_id: "" })
        },
        {
          label: t("index.preview.common.openPaymentImport"),
          url: nativeInstallURL
        }
      ]
    )
  ];
  blocks.push(
    buildCollapsedPayloadBlock(
      t("index.preview.common.rawPayload.title"),
      preview,
      t("index.paymentPreview.payload.summary")
    )
  );
  return blocks;
}

function buildNativeMarketInstallURL(preview: UnknownMap): string {
  const source = asRecord(preview.source);
  const coordinates = asRecord(preview.coordinates);
  return buildNativeMarketInstallURLFromCoordinates(
    source,
    coordinates.kind || "plugin_package",
    coordinates.name,
    coordinates.version
  );
}

function buildTemplateMarketTargetParams(state: MarketConsoleState): UnknownMap {
  if (state.kind === "email_template") {
    return {
      email_key: state.email_key || undefined,
      target_key: state.email_key || undefined
    };
  }
  if (state.kind === "landing_page_template") {
    return {
      landing_slug: state.landing_slug || undefined,
      slug: state.landing_slug || undefined,
      target_key: state.landing_slug || undefined
    };
  }
  if (state.kind === "invoice_template") {
    return {
      target_key: "invoice"
    };
  }
  if (state.kind === "auth_branding_template") {
    return {
      target_key: "auth_branding"
    };
  }
  if (state.kind === "page_rule_pack") {
    return {
      target_key: "page_rules"
    };
  }
  return {};
}

function buildTemplateMarketPreviewBlocks(preview: UnknownMap, state: MarketConsoleState): PluginPageBlock[] {
  const coordinates = asRecord(preview.coordinates);
  const source = asRecord(preview.source);
  const release = asRecord(preview.release);
  const targetState = asRecord(preview.target_state);
  const resolved = asRecord(preview.resolved);
  const compatibility = asRecord(preview.compatibility);
  const warnings = uniqueDisplayStrings(
    normalizeDisplayStringArray(preview.warnings).concat(normalizeDisplayStringArray(compatibility.warnings))
  );
  const changeSummary = resolveTemplateChangeLabel(resolved, targetState);
  const compatibilityLabel =
    compatibility.compatible === false
      ? t("index.releaseDetail.overview.hostCompatibility.rejected")
      : t("index.releaseDetail.overview.hostCompatibility.ready");
  return [
    buildPluginAlertBlock({
      title: t("index.templatePreview.title"),
      content:
        compatibility.compatible === false
          ? t("index.templatePreview.content.incompatible")
          : targetState.target_exists !== true
            ? t("index.templatePreview.content.createFirst")
            : changeSummary === "No content diff detected"
              ? t("index.templatePreview.content.noDiff")
              : t("index.templatePreview.content.update"),
      variant: compatibility.compatible === false ? "warning" : warnings.length > 0 ? "info" : "success"
    }),
    buildPluginStatsGridBlock({
      title: t("index.preview.common.overview.title"),
      items: [
        { label: t("index.load.context.kind.label"), value: asString(release.kind) || asString(coordinates.kind) || "-" },
        { label: t("index.load.context.name.label"), value: asString(release.name) || asString(coordinates.name) || "-" },
        { label: t("index.load.context.version.label"), value: asString(release.version) || asString(coordinates.version) || "-" },
        { label: t("index.templatePreview.overview.targetExists.label"), value: formatBooleanState(targetState.target_exists) },
        { label: t("index.templatePreview.overview.hostManaged.label"), value: formatBooleanState(targetState.installed) },
        { label: t("index.preview.common.currentVersion.label"), value: asString(targetState.current_version) || "-" },
        { label: t("index.templatePreview.overview.contentBytes.label"), value: asInteger(resolved.content_bytes, 0) },
        { label: t("index.preview.common.warningCount.label"), value: warnings.length }
      ]
    }),
    buildPluginKeyValueBlock({
      title: t("index.templatePreview.context.title"),
      items: [
        { label: t("index.sourceSummary.sourceId.label"), value: asString(source.source_id) || asString(coordinates.source_id) || "-" },
        { label: t("index.sourceSummary.sourceName.label"), value: asString(source.name) || "-" },
        { label: t("index.load.context.kind.label"), value: asString(release.kind) || asString(coordinates.kind) || "-" },
        { label: t("index.load.context.name.label"), value: asString(release.name) || asString(coordinates.name) || "-" },
        { label: t("index.load.context.version.label"), value: asString(release.version) || asString(coordinates.version) || "-" },
        { label: t("index.templatePreview.context.targetKey.label"), value: asString(resolved.target_key) || asString(targetState.installed_target_key) || "-" },
        { label: t("index.templatePreview.context.hostCompatibility.label"), value: compatibilityLabel }
      ]
    }),
    buildPluginKeyValueBlock({
      title: t("index.templatePreview.diff.title"),
      items: [
        { label: t("index.templatePreview.overview.contentBytes.label"), value: String(asInteger(resolved.content_bytes, 0)) },
        { label: t("index.templatePreview.diff.newDigest.label"), value: asString(resolved.content_digest) || "-" },
        { label: t("index.templatePreview.diff.currentDigest.label"), value: asString(targetState.current_digest) || "-" },
        { label: t("index.templatePreview.diff.currentBytes.label"), value: String(asInteger(targetState.current_content_bytes, 0)) || "-" },
        { label: t("index.templatePreview.diff.changeSummary.label"), value: changeSummary },
        { label: t("index.templatePreview.diff.updateAvailable.label"), value: formatBooleanState(targetState.update_available) }
      ]
    }),
    ...(warnings.length > 0
      ? [
          buildPluginBadgeListBlock({
            title: t("index.preview.common.warningTitle"),
            items: warnings
          })
        ]
      : [
          buildPluginAlertBlock({
            title: t("index.preview.common.warningTitle"),
            content: t("index.templatePreview.warnings.none"),
            variant: "success"
          })
        ]),
    ...buildRecommendedActionBlocks(
      compatibility.compatible === false
        ? t("index.templatePreview.guidance.incompatible.action")
        : t("index.templatePreview.guidance.ready.action"),
      compatibility.compatible === false
        ? t("index.templatePreview.guidance.incompatible.rationale")
        : targetState.target_exists !== true
          ? t("index.templatePreview.guidance.ready.createFirst")
          : changeSummary === "No content diff detected"
            ? t("index.templatePreview.guidance.ready.noDiff")
            : t("index.templatePreview.guidance.ready.update"),
      [
        {
          label: t("index.preview.common.backToSource"),
          url: buildMarketConsoleURL(state, { name: "", version: "" })
        },
        {
          label: t("index.common.backToArtifactContext"),
          url: buildMarketConsoleURL(state, { version: "" })
        },
        {
          label: t("index.workflow.common.reuse", {
            label: `${state.name || asString(coordinates.name) || "template"}@${state.version || asString(coordinates.version) || "-"}`
          }),
          url: buildMarketConsoleURL(state)
        }
      ]
    ),
    buildCollapsedPayloadBlock(
      t("index.preview.common.rawPayload.title"),
      preview,
      t("index.templatePreview.payload.summary")
    )
  ];
}

function buildTemplateHostInstallBlocks(
  result: UnknownMap,
  state?: MarketConsoleState
): PluginPageBlock[] {
  const resolved = asRecord(result.resolved);
  const targetState = asRecord(result.target_state);
  const historyEntry = asRecord(result.history_entry);
  const blocks: PluginPageBlock[] = [
    buildPluginAlertBlock({
      title: t("index.templateInstall.title"),
      content: t("index.templateInstall.content.status", {
        status: asString(result.status) || "unknown"
      }),
      variant: asString(result.status) === "imported" ? "success" : "info"
    }),
    buildPluginKeyValueBlock({
      title: t("index.templateInstall.result.title"),
      items: [
        { label: t("index.load.context.kind.label"), value: asString(asRecord(result.coordinates).kind) || "-" },
        { label: t("index.templatePreview.context.targetKey.label"), value: asString(resolved.target_key) || asString(targetState.installed_target_key) || "-" },
        { label: t("index.templateInstall.result.importVersion.label"), value: asString(asRecord(result.coordinates).version) || "-" },
        { label: t("index.preview.common.currentVersion.label"), value: asString(targetState.current_version) || "-" },
        { label: t("index.templateInstall.result.historyId.label"), value: asString(historyEntry.id) || "-" },
        { label: t("index.templateInstall.result.contentDigest.label"), value: asString(resolved.content_digest) || "-" }
      ]
    }),
    buildCollapsedPayloadBlock(
      t("index.templateInstall.payload.title"),
      result,
      t("index.templateInstall.payload.summary")
    )
  ];
  if (state) {
    blocks.push(
      ...buildRecommendedActionBlocks(
        t("index.templateInstall.guidance.action"),
        t("index.templateInstall.guidance.rationale"),
        [
          {
            label: t("index.templateInstall.guidance.openImported"),
            url: buildArtifactContextURLFromItem(state, result, { task_id: "" })
          },
          {
            label: t("index.guidance.package.openHistory"),
            url: buildMarketConsoleURL(state, { task_id: "" })
          }
        ]
      )
    );
  }
  return blocks;
}

function buildPluginPackageInstallBlocks(
  result: UnknownMap,
  state?: MarketConsoleState
): PluginPageBlock[] {
  const blocks: PluginPageBlock[] = [
    buildPluginAlertBlock({
      title: t("index.pluginInstall.title"),
      content: t("index.pluginInstall.content.status", {
        status: asString(result.status) || "unknown"
      }),
      variant:
        asString(result.status) === "activate_failed"
          ? "warning"
          : asString(result.status) === "activated"
            ? "success"
            : "info"
    }),
    buildPluginKeyValueBlock({
      title: t("index.pluginInstall.result.title"),
      items: [
        { label: t("index.task.overview.taskId.label"), value: asString(result.task_id) || "-" },
        { label: t("index.task.overview.status.label"), value: asString(result.status) || "-" },
        { label: t("index.pluginInstall.result.activateRequested.label"), value: result.activate_requested === true ? "true" : "false" },
        { label: t("index.pluginInstall.result.autoStart.label"), value: result.auto_start === true ? "true" : "false" }
      ]
    }),
    buildCollapsedPayloadBlock(
      t("index.pluginInstall.payload.title"),
      result,
      t("index.pluginInstall.payload.summary")
    )
  ];
  if (state) {
    const taskID = asString(result.task_id);
    blocks.push(
      ...buildRecommendedActionBlocks(
        taskID
          ? t("index.pluginInstall.guidance.withTask.action", {
              taskId: taskID
            })
          : t("index.pluginInstall.guidance.noTask.action"),
        taskID
          ? t("index.pluginInstall.guidance.withTask.rationale")
          : t("index.pluginInstall.guidance.noTask.rationale"),
        [
          {
            label: taskID
              ? t("index.guidance.package.task.openTask", {
                  taskId: taskID
                })
              : t("index.pluginInstall.guidance.openInstalled"),
            url: buildArtifactContextURLFromItem(state, result, { task_id: taskID || "" })
          },
          {
            label: t("index.guidance.package.openHistory"),
            url: buildMarketConsoleURL(state, { task_id: "" })
          }
        ]
      )
    );
  }
  return blocks;
}

function buildPluginPackageRollbackBlocks(
  result: UnknownMap,
  state?: MarketConsoleState
): PluginPageBlock[] {
  const coordinates = asRecord(result.coordinates);
  const kind = asString(coordinates.kind);
  const target = asRecord(result.template);
  const title =
    kind === "payment_package"
      ? t("index.rollback.title.payment")
      : isTemplateMarketKind(kind)
        ? t("index.rollback.title.template")
        : t("index.rollback.title.plugin");
  const blocks: PluginPageBlock[] = [
    buildPluginAlertBlock({
      title,
      content: t("index.rollback.content.status", {
        status: asString(result.status) || "unknown"
      }),
      variant:
        asString(result.status) === "rollback_failed"
          ? "warning"
          : asString(result.status) === "rolled_back" || asString(result.status) === "already_active"
            ? "success"
            : "info"
    }),
    buildPluginKeyValueBlock({
      title: t("index.rollback.result.title"),
      items: [
        { label: t("index.task.overview.taskId.label"), value: asString(result.task_id) || "-" },
        { label: t("index.task.overview.status.label"), value: asString(result.status) || "-" },
        { label: t("index.rollback.result.coordinates.label"), value: prettyJSON(asRecord(result.coordinates)) },
        { label: t("index.rollback.result.targetKey.label"), value: asString(target.target_key) || "-" },
        { label: t("index.rollback.result.error.label"), value: asString(result.error) || "-" }
      ]
    }),
    buildCollapsedPayloadBlock(
      t("index.rollback.payload.title"),
      result,
      t("index.rollback.payload.summary")
    )
  ];
  if (state) {
    const templateRollback = isTemplateMarketKind(kind);
    blocks.push(
      ...buildRecommendedActionBlocks(
        templateRollback
          ? t("index.rollback.guidance.template.action")
          : t("index.rollback.guidance.package.action"),
        templateRollback
          ? t("index.rollback.guidance.template.rationale")
          : asString(result.task_id)
            ? t("index.rollback.guidance.package.withTask")
            : t("index.rollback.guidance.package.noTask"),
        [
          {
            label:
              templateRollback
                ? t("index.rollback.guidance.openRestoredTemplate")
                : t("index.rollback.guidance.openRestoredPackage"),
            url: buildArtifactContextURLFromItem(state, result, { task_id: asString(result.task_id) || "" })
          },
          {
            label: t("index.guidance.package.openHistory"),
            url: buildMarketConsoleURL(state, { task_id: "" })
          }
        ]
      )
    );
  }
  return blocks;
}

function buildTaskBlocks(task: UnknownMap, state?: MarketConsoleState): PluginPageBlock[] {
  const coordinates = asRecord(task.coordinates);
  const statusBucket = resolveTaskStatusBucket(task);
  const blocks: PluginPageBlock[] = [
    buildPluginAlertBlock({
      title: t("index.task.status.title"),
      content:
        statusBucket === "failed"
          ? t("index.task.status.content.failed")
          : statusBucket === "completed"
            ? t("index.task.status.content.completed")
            : t("index.task.status.content.running"),
      variant: statusBucket === "failed" ? "warning" : statusBucket === "completed" ? "success" : "info"
    }),
    buildPluginStatsGridBlock({
      title: t("index.task.overview.title"),
      items: [
        { label: t("index.task.overview.taskId.label"), value: asString(task.task_id) || "-" },
        { label: t("index.task.overview.status.label"), value: asString(task.status) || "-" },
        { label: t("index.task.overview.phase.label"), value: asString(task.phase) || "-" },
        { label: t("index.task.overview.progress.label"), value: String(asInteger(task.progress, 0)) },
        { label: t("index.task.overview.artifact.label"), value: asString(coordinates.name) || "-" },
        { label: t("index.task.overview.updatedAt.label"), value: asString(task.updated_at) || "-" }
      ]
    }),
    buildPluginKeyValueBlock({
      title: t("index.task.coordinates.title"),
      items: [
        { label: t("index.sourceSummary.sourceId.label"), value: asString(coordinates.source_id) || "-" },
        { label: t("index.load.context.kind.label"), value: asString(coordinates.kind) || "-" },
        { label: t("index.load.context.name.label"), value: asString(coordinates.name) || "-" },
        { label: t("index.load.context.version.label"), value: asString(coordinates.version) || "-" },
        { label: t("index.task.coordinates.createdAt.label"), value: asString(task.created_at) || "-" }
      ]
    }),
    buildCollapsedPayloadBlock(
      t("index.task.payload.title"),
      task,
      t("index.task.payload.summary")
    )
  ];
  if (state) {
    blocks.push(
      ...buildRecommendedActionBlocks(
        statusBucket === "failed"
          ? t("index.task.guidance.failed.action")
          : statusBucket === "completed"
            ? t("index.task.guidance.completed.action")
            : t("index.task.guidance.running.action"),
        statusBucket === "failed"
          ? t("index.task.guidance.failed.rationale")
          : statusBucket === "completed"
            ? t("index.task.guidance.completed.rationale")
            : t("index.task.guidance.running.rationale"),
        [
          {
            label: t("index.task.guidance.openCurrent"),
            url: buildArtifactContextURLFromItem(state, task)
          },
          {
            label: t("index.guidance.package.openHistory"),
            url: buildMarketConsoleURL(state, { task_id: "" })
          }
        ]
      )
    );
    blocks.push(...buildTaskContextLinks(state, [task]));
  }
  return blocks;
}

function buildTaskListBlocks(
  items: UnknownMap[],
  pagination: UnknownMap,
  state?: MarketConsoleState
): PluginPageBlock[] {
  const failedCount = countMatching(items, (item) => resolveTaskStatusBucket(item) === "failed");
  const completedCount = countMatching(items, (item) => resolveTaskStatusBucket(item) === "completed");
  const runningCount = Math.max(0, items.length - failedCount - completedCount);
  const uniqueArtifacts = uniqueDisplayStrings(items.map((item) => asString(asRecord(item.coordinates).name)));
  const blocks: PluginPageBlock[] = [
    buildPluginAlertBlock({
      title: t("index.taskList.title"),
      content:
        items.length === 0
          ? t("index.taskList.content.empty")
          : failedCount > 0
            ? t("index.taskList.content.failed", {
                count: items.length,
                failed: failedCount
              })
            : runningCount > 0
              ? t("index.taskList.content.running", {
                  count: items.length,
                  running: runningCount
                })
              : t("index.taskList.content.default"),
      variant: items.length === 0 ? "info" : failedCount > 0 ? "warning" : "success"
    }),
    buildPluginStatsGridBlock({
      title: t("index.taskList.stats.title"),
      items: [
        { label: t("index.taskList.stats.count.label"), value: items.length },
        { label: t("index.taskList.stats.total.label"), value: asInteger(pagination.total, items.length) },
        { label: t("index.taskList.stats.running.label"), value: runningCount },
        { label: t("index.taskList.stats.completed.label"), value: completedCount },
        { label: t("index.taskList.stats.failed.label"), value: failedCount },
        { label: t("index.taskList.stats.artifactCount.label"), value: uniqueArtifacts.length }
      ]
    }),
    buildPluginTableBlock({
      title: t("index.taskList.table.title"),
      columns: [
        "task_id",
        "status",
        "phase",
        "progress",
        "name",
        "version",
        "created_at"
      ],
      rows: items.map((item) => {
        const coordinates = asRecord(item.coordinates);
        return {
          task_id: asString(item.task_id),
          status: asString(item.status),
          phase: asString(item.phase),
          progress: String(asInteger(item.progress, 0)),
          name: asString(coordinates.name),
          version: resolveVersionText(coordinates.version),
          created_at: asString(item.created_at)
        };
      }),
      empty_text: t("index.taskList.table.empty")
    })
  ];
  if (state && items.length > 0) {
    blocks.push(
      ...buildRecommendedActionBlocks(
        failedCount > 0
          ? t("index.taskList.guidance.failed.action")
          : runningCount > 0
            ? t("index.taskList.guidance.running.action")
            : t("index.taskList.guidance.completed.action"),
        failedCount > 0
          ? t("index.taskList.guidance.failed.rationale")
          : runningCount > 0
            ? t("index.taskList.guidance.running.rationale")
            : t("index.taskList.guidance.completed.rationale"),
        [
          {
            label:
              failedCount > 0
                ? t("index.taskList.guidance.openFirstFailed")
                : runningCount > 0
                  ? t("index.taskList.guidance.openLatestRunning")
                  : t("index.taskList.guidance.openLatestContext"),
            url: buildArtifactContextURLFromItem(
              state,
              failedCount > 0
                ? items.find((item) => resolveTaskStatusBucket(item) === "failed") || items[0]
                : runningCount > 0
                  ? items.find((item) => resolveTaskStatusBucket(item) === "running") || items[0]
                  : items[0]
            )
          },
          {
            label: t("index.guidance.package.openHistory"),
            url: buildMarketConsoleURL(state, { task_id: "" })
          }
        ]
      )
    );
  }
  if (state && items.length === 0) {
    blocks.push(
      ...buildRecommendedActionBlocks(
        t("index.taskList.guidance.empty.action"),
        state.version
          ? t("index.taskList.guidance.empty.rationale.version")
          : t("index.taskList.guidance.empty.rationale.default"),
        [
          {
            label: t("index.taskList.guidance.empty.clearTaskVersion"),
            url: buildMarketConsoleURL(state, { task_id: "", version: "" })
          },
          {
            label: t("index.taskList.guidance.empty.backToPackage"),
            url: buildMarketConsoleURL(state, { task_id: "" })
          }
        ]
      )
    );
  }
  if (state && items.length > 0) {
    blocks.push(...buildTaskContextLinks(state, items));
  }
  return blocks;
}

function buildInstallHistoryBlocks(
  items: UnknownMap[],
  pagination: UnknownMap,
  state?: MarketConsoleState
): PluginPageBlock[] {
  const activeCount = countMatching(items, (item) => item.is_active === true);
  const uniqueArtifacts = uniqueDisplayStrings(items.map((item) => asString(item.name)));
  const uniqueVersions = uniqueDisplayStrings(items.map((item) => asString(item.version)));
  const matchingTargets = uniqueDisplayStrings(
    items.map((item) => {
      const targetKey = asString(item.installed_target_key);
      const targetID = asInteger(item.installed_target_id, 0);
      const targetType = asString(item.installed_target_type);
      return targetKey ? `${targetType || "target"}:${targetKey}` : targetID > 0 ? `${targetType || "target"}:${targetID}` : "";
    })
  );
  const blocks: PluginPageBlock[] = [
    buildPluginAlertBlock({
      title: t("index.history.title"),
      content:
        items.length === 0
          ? t("index.history.content.empty")
          : activeCount > 0
            ? t("index.history.content.active", {
                count: items.length,
                active: activeCount
              })
            : t("index.history.content.noActive", {
                count: items.length
              }),
      variant: items.length === 0 ? "info" : activeCount > 0 ? "success" : "warning"
    }),
    buildPluginStatsGridBlock({
      title: t("index.history.stats.title"),
      items: [
        { label: t("index.history.stats.count.label"), value: items.length },
        { label: t("index.history.stats.total.label"), value: asInteger(pagination.total, items.length) },
        { label: t("index.history.stats.active.label"), value: activeCount },
        { label: t("index.history.stats.artifactCount.label"), value: uniqueArtifacts.length },
        { label: t("index.history.stats.versionCount.label"), value: uniqueVersions.length },
        { label: t("index.history.stats.targetCount.label"), value: matchingTargets.length }
      ]
    }),
    buildPluginTableBlock({
      title: t("index.history.table.title"),
      columns: [
        "source_id",
        "kind",
        "name",
        "version",
        "installed_target",
        "is_active",
        "installed_at"
      ],
      rows: items.map((item) => {
        const targetKey = asString(item.installed_target_key);
        const targetID = asInteger(item.installed_target_id, 0);
        const targetType = asString(item.installed_target_type);
        return {
          source_id: asString(item.source_id),
          kind: asString(item.kind),
          name: asString(item.name),
          version: asString(item.version),
          installed_target:
            targetKey
              ? `${targetType || "target"}:${targetKey}`
              : `${targetType || "target"}:${targetID}`,
          is_active: item.is_active === true ? "true" : "false",
          installed_at: asString(item.installed_at)
        };
      }),
      empty_text: t("index.history.table.empty")
    })
  ];

  if (state && items.length > 0) {
    const activeItem = items.find((item) => item.is_active === true) || items[0];
    blocks.push(
      ...buildRecommendedActionBlocks(
        activeCount > 0
          ? t("index.history.guidance.active.action")
          : t("index.history.guidance.noActive.action"),
        activeCount > 0
          ? t("index.history.guidance.active.rationale")
          : t("index.history.guidance.noActive.rationale"),
        [
          {
            label:
              activeCount > 0
                ? t("index.history.guidance.openActive")
                : t("index.history.guidance.openLatest"),
            url: buildArtifactContextURLFromItem(state, activeItem, { task_id: "" })
          },
          {
            label: t("index.common.backToArtifactContext"),
            url: buildMarketConsoleURL(state, { task_id: "" })
          }
        ]
      )
    );
  }

  if (state && items.length === 0) {
    blocks.push(
      ...buildRecommendedActionBlocks(
        t("index.history.guidance.empty.action"),
        state.version
          ? t("index.history.guidance.empty.rationale.version")
          : isTemplateMarketKind(state.kind)
            ? t("index.history.guidance.empty.rationale.template")
            : t("index.history.guidance.empty.rationale.default"),
        [
          {
            label: t("index.history.guidance.empty.clearVersion"),
            url: buildMarketConsoleURL(state, { version: "", task_id: "" })
          },
          {
            label: t("index.history.guidance.empty.clearArtifact"),
            url: buildMarketConsoleURL(state, { name: "", version: "", task_id: "" })
          }
        ]
      )
    );
  }

  if (state && items.length > 0) {
    blocks.push(
      buildPluginLinkListBlock({
        title:
          isTemplateMarketKind(state.kind)
            ? t("index.history.links.releaseContext.title")
            : t("index.history.links.operationContext.title"),
        links: items.slice(0, 8).map((item) => ({
          label:
            item.is_active === true
              ? t("index.templateTarget.recentVersions.activeSuffix", {
                  release: `${asString(item.name) || state.name || "template"}@${asString(item.version) || "-"}`
                })
              : `${asString(item.name) || state.name || "template"}@${asString(item.version) || "-"}`,
          url: buildArtifactContextURLFromItem(state, item, { task_id: "" })
        }))
      })
    );
  }

  return blocks;
}

function executeHook(params: UnknownMap, config: UnknownMap): PluginExecuteResult {
  const hook = asString(params.hook).toLowerCase();
  const payload = parseJSONRecord(params.payload);
  if (hook === "frontend.bootstrap") {
    const area = asString(payload.area).toLowerCase();
    if (area !== "admin") {
      return successResult({
        frontend_extensions: []
      });
    }
    return {
      success: true,
      frontend_extensions: buildBootstrapExtensions(config, asRecord(payload.query_params))
    };
  }
  return successResult({
    message: `Unhandled hook ${hook || "unknown"}.`
  });
}

function executeTrustedWorkspaceAsset(params: UnknownMap): PluginExecuteResult {
  const asset = getTrustedWorkspaceAsset(asString(params.asset));
  if (!asset) {
    return errorResult(
      `trusted workspace asset unavailable: ${asString(params.asset) || "unknown"}`
    );
  }
  return successResult({
    asset,
  });
}

function executeAction(
  action: string,
  params: UnknownMap,
  context: PluginExecutionContext,
  config: UnknownMap
): PluginExecuteResult {
  const locale = resolveMarketLocale(context, params);
  try {
    let result: PluginExecuteResult;
    switch (action) {
      case "hook.execute":
        result = executeHook(params, config);
        break;
      case "market.console.load":
        result = executeConsoleLoad(params, config, context);
        break;
      case "market.package.load":
        result = executePackageConsoleLoad(params, config, context);
        break;
      case "market.package.reset":
        result = executePackageConsoleReset(params, config, context);
        break;
      case "market.template.load":
        result = executeTemplateConsoleLoad(params, config, context);
        break;
      case "market.template.reset":
        result = executeTemplateConsoleReset(params, config, context);
        break;
      case "market.trusted.asset":
        result = executeTrustedWorkspaceAsset(params);
        break;
      case "market.source.detail":
        result = executeSourceDetail(params, config, context);
        break;
      case "market.catalog.query":
        result = executeCatalogQuery(params, config, context);
        break;
      case "market.artifact.detail":
        result = executeArtifactDetail(params, config, context);
        break;
      case "market.release.detail":
        result = executeReleaseDetail(params, config, context);
        break;
      case "market.release.preview":
        result = executeReleasePreview(params, config, context);
        break;
      case "market.install.execute":
        result = executeInstallOrImport(params, config, context);
        break;
      case "market.install.task.get":
        result = executeTaskGet(params, config, context);
        break;
      case "market.install.task.list":
        result = executeTaskList(params, config, context);
        break;
      case "market.install.history.list":
        result = executeHistoryList(params, config, context);
        break;
      case "market.install.rollback":
        result = executeRollback(params, config, context);
        break;
      default:
        result = successResult({
          message: "AuraLogic Market is ready.",
          values: {
            action,
            params,
            context
          }
        });
        break;
    }
    const localized = localizeMarketExecuteResult(result, locale);
    maybeMirrorMarketActionToWorkspace(action, params, localized);
    return localized;
  } catch (error) {
    const localized = localizeMarketExecuteResult(errorResult(errorToString(error)), locale);
    maybeMirrorMarketActionToWorkspace(action, params, localized);
    return localized;
  }
}

function executeMarketWorkspaceContextCommand(
  command: PluginWorkspaceCommandContext,
  context: PluginExecutionContext,
  config: UnknownMap,
  sandbox: PluginSandboxProfile,
  workspace: PluginWorkspaceAPI
): PluginExecuteResult {
  const defaults = buildDefaultMarketConsoleState(config);
  const pageContext = resolvePluginPageContext(context);
  const snapshot = workspace.snapshot(8);
  const allowedKinds = normalizeDisplayStringArray(config.allowed_kinds);
  let sources: UnknownMap[] = [];
  let sourceError = "";

  try {
    sources = sourceList();
  } catch (error) {
    sourceError = errorToString(error);
  }

  const metadata = buildMarketWorkspaceMetadata("workspace.command.market.context", defaults as unknown as UnknownMap, {
    workspace_command: command.entry
  });
  const summaryLines = compactMarketWorkspaceLines([
    formatMarketWorkspaceStateSummary(defaults as unknown as UnknownMap),
    `page=${pageContext.full_path || pageContext.path || "-"}`,
    `sources=${sources.length}`,
    `workspace_entries=${snapshot.entry_count}/${snapshot.max_entries}`,
    `current_action=${asString(sandbox.currentAction) || "workspace.command.execute"}`,
    allowedKinds.length > 0 ? `allowed_kinds=${allowedKinds.join(", ")}` : "allowed_kinds=(all)"
  ]);

  workspace.info(
    t("index.workspace.context.loadedNotice"),
    metadata
  );
  summaryLines.forEach((line) => {
    workspace.writeln(line, metadata);
  });
  if (sourceError) {
    workspace.warn(
      t("index.workspace.context.loadSourcesFailed", {
        error: sourceError
      }),
      metadata
    );
  }

  const data: PluginActionData = {
    source: "workspace.command",
    message: t("index.workspace.context.bufferWritten"),
    values: {
      command: command.name,
      entry: command.entry,
      raw: command.raw,
      argv: command.argv,
      current_action: asString(sandbox.currentAction) || "workspace.command.execute",
      default_state: defaults,
      page_context: pageContext,
      source_count: sources.length,
      source_error: sourceError,
      allowed_kinds: allowedKinds,
      workspace_snapshot: snapshot
    },
    blocks: [
      buildPluginStatsGridBlock({
        title: t("index.workspace.overview.title"),
        items: [
          { label: t("index.workspace.overview.sourceCount.label"), value: sources.length },
          { label: t("index.workspace.overview.entryCount.label"), value: snapshot.entry_count },
          { label: t("index.workspace.overview.bufferCapacity.label"), value: snapshot.max_entries },
          { label: t("index.workspace.overview.defaultChannel.label"), value: defaults.channel || "-" }
        ]
      }),
      buildPluginKeyValueBlock({
        title: t("index.workspace.context.title"),
        items: [
          { label: t("index.workspace.context.command.label"), value: command.name },
          { label: t("index.sourceSummary.sourceId.label"), value: defaults.source_id || "-" },
          { label: t("index.workspace.context.defaultKind.label"), value: defaults.kind || "-" },
          { label: t("index.workspace.context.pagePath.label"), value: pageContext.full_path || pageContext.path || "-" },
          {
            label: t("index.workspace.context.currentAction.label"),
            value: asString(sandbox.currentAction) || "workspace.command.execute"
          }
        ]
      }),
      buildPluginTextBlock(
        allowedKinds.length > 0
          ? t("index.workspace.context.allowedKinds.named", {
              kinds: allowedKinds.join(", ")
            })
          : t("index.workspace.context.allowedKinds.all"),
        t("index.workspace.context.commandHint.title")
      )
    ]
  };

  return successResult(data);
}

const marketWorkspaceHandlers = defineWorkspaceCommands({
  "market.context": defineWorkspaceCommand(
    {
      name: "market/context",
      title: "Market Context",
      description:
        "Write the current market defaults, page context, source count, and workspace snapshot into the plugin workspace buffer.",
      interactive: false
    },
    executeMarketWorkspaceContextCommand
  ),
  "market.source": defineWorkspaceCommand(
    {
      name: "market/source",
      title: "Market Source Detail",
      description:
        "Read one trusted source into the workspace buffer. Positional usage: market/source official",
      interactive: false
    },
    (command, context, config, _sandbox, workspace) =>
      executeMarketWorkspaceActionCommand(
        command,
        context,
        config,
        workspace,
        "market.source.detail",
        resolveMarketSourceWorkspaceParams(command)
      )
  ),
  "market.catalog": defineWorkspaceCommand(
    {
      name: "market/catalog",
      title: "Market Catalog",
      description:
        "List catalog items in the workspace buffer. Positional usage: market/catalog plugin_package debugger",
      interactive: false
    },
    (command, context, config, _sandbox, workspace) =>
      executeMarketWorkspaceActionCommand(
        command,
        context,
        config,
        workspace,
        "market.catalog.query",
        resolveMarketCatalogWorkspaceParams(command)
      )
  ),
  "market.release": defineWorkspaceCommand(
    {
      name: "market/release",
      title: "Market Release Detail",
      description:
        "Load release metadata into the workspace buffer. Positional usage: market/release js-worker-template 0.2.2",
      interactive: false
    },
    (command, context, config, _sandbox, workspace) =>
      executeMarketWorkspaceActionCommand(
        command,
        context,
        config,
        workspace,
        "market.release.detail",
        resolveMarketArtifactWorkspaceParams(command)
      )
  ),
  "market.preview": defineWorkspaceCommand(
    {
      name: "market/preview",
      title: "Market Preview",
      description:
        "Preview an install or import in the workspace buffer. Positional usage: market/preview js-worker-template 0.2.2",
      interactive: false
    },
    (command, context, config, _sandbox, workspace) =>
      executeMarketWorkspaceActionCommand(
        command,
        context,
        config,
        workspace,
        "market.release.preview",
        resolveMarketArtifactWorkspaceParams(command)
      )
  ),
  "market.install": defineWorkspaceCommand(
    {
      name: "market/install",
      title: "Market Install Or Import",
      description:
        "Execute a host install/import from the workspace. Supports key=value overrides such as kind=, source=, activate=, auto_start=, granted=, note=.",
      interactive: false
    },
    (command, context, config, _sandbox, workspace) =>
      executeMarketWorkspaceActionCommand(
        command,
        context,
        config,
        workspace,
        "market.install.execute",
        resolveMarketArtifactWorkspaceParams(command)
      )
  ),
  "market.tasks": defineWorkspaceCommand(
    {
      name: "market/tasks",
      title: "Market Tasks",
      description:
        "List host install tasks, or pass a task id to inspect one task. Positional usage: market/tasks task_123",
      interactive: false
    },
    (command, context, config, _sandbox, workspace) => {
      const params = resolveMarketTaskWorkspaceParams(command);
      return executeMarketWorkspaceActionCommand(
        command,
        context,
        config,
        workspace,
        asString(params.task_id) ? "market.install.task.get" : "market.install.task.list",
        params
      );
    }
  ),
  "market.history": defineWorkspaceCommand(
    {
      name: "market/history",
      title: "Market History",
      description:
        "List host install/import history in the workspace buffer. Positional usage: market/history js-worker-template",
      interactive: false
    },
    (command, context, config, _sandbox, workspace) =>
      executeMarketWorkspaceActionCommand(
        command,
        context,
        config,
        workspace,
        "market.install.history.list",
        resolveMarketArtifactWorkspaceParams(command)
      )
  ),
  "market.rollback": defineWorkspaceCommand(
    {
      name: "market/rollback",
      title: "Market Rollback",
      description:
        "Trigger a host-managed rollback from the workspace. Positional usage: market/rollback js-worker-template 0.2.1",
      interactive: false
    },
    (command, context, config, _sandbox, workspace) =>
      executeMarketWorkspaceActionCommand(
        command,
        context,
        config,
        workspace,
        "market.install.rollback",
        resolveMarketArtifactWorkspaceParams(command)
      )
  )
});

module.exports = definePlugin({
  execute(
    action: unknown,
    params: unknown,
    context: PluginExecutionContext,
    config: UnknownMap,
    _sandbox: PluginSandboxProfile
  ): PluginExecuteResult | UnknownMap {
    return executeAction(asString(action), asRecord(params), asRecord(context) as PluginExecutionContext, config);
  },

  health(config: UnknownMap): PluginHealthResult {
    const allowedKinds = normalizeDisplayStringArray(config.allowed_kinds);
    return {
      healthy: true,
      version: "0.1.34",
      metadata: {
        plugin: PLUGIN_IDENTITY,
        display_name: PLUGIN_DISPLAY_NAME,
        admin_path: ADMIN_PLUGIN_PAGE_PATH,
        source_id: asString(config.source_id) || "official",
        source_base_url: asString(config.source_base_url) || "https://market.auralogic.org",
        default_channel: asString(config.default_channel) || "stable",
        allowed_kinds: allowedKinds,
        trusted_workspace_assets: ["assets/trusted-workspace.css", "assets/trusted-workspace.js"],
        revision_backend: getRevisionBackendLabel()
      }
    };
  },

  workspace: marketWorkspaceHandlers
});
