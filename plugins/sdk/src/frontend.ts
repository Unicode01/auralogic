type FrontendStringMap = Record<string, string>;
type FrontendUnknownMap = Record<string, unknown>;

export type PluginFrontendArea = "admin" | "user";

export type PluginPageBlock = {
  type: string;
  title?: string;
  content?: string;
  execute_actions?: string[];
  stream_actions?: string[];
  execute_stream_actions?: string[];
  data?: FrontendUnknownMap;
};

export type PluginPageSchema = {
  title?: string;
  description?: string;
  host_header?: "show" | "hide";
  host_market_workspace?: boolean;
  execute_actions?: string[];
  stream_actions?: string[];
  execute_stream_actions?: string[];
  blocks?: PluginPageBlock[];
};

export type PluginAlertVariant = "default" | "info" | "success" | "warning" | "destructive";

export type PluginAlertBlockOptions = {
  title?: string;
  content: string;
  variant?: PluginAlertVariant;
};

export type PluginKeyValueItem = {
  key?: string;
  label: string;
  value: unknown;
  description?: string;
};

export type PluginKeyValueBlockOptions = {
  title?: string;
  items: PluginKeyValueItem[];
};

export type PluginJSONViewBlockOptions = {
  title?: string;
  value: unknown;
  summary?: string;
  collapsible?: boolean;
  collapsed?: boolean;
  preview_lines?: number;
  max_height?: number;
};

export type PluginTableBlockOptions = {
  title?: string;
  content?: string;
  columns: string[];
  rows: FrontendUnknownMap[];
  empty_text?: string;
};

export type PluginBadgeListBlockOptions = {
  title?: string;
  items: Array<string | number | boolean>;
};

export type PluginStatsGridItem = {
  label: string;
  value: string | number | boolean;
  description?: string;
};

export type PluginStatsGridBlockOptions = {
  title?: string;
  items: PluginStatsGridItem[];
};

export type PluginFrontendExtension = {
  type: string;
  slot?: string;
  title?: string;
  content?: string;
  link?: string;
  priority?: number;
  data?: FrontendUnknownMap;
};

export type PluginActionFormFieldOption = {
  label?: string;
  value?: string | number | boolean;
  key?: string;
};

export type PluginActionFormConditionMatcher = {
  field?: string;
  equals?: string | number | boolean;
  in?: Array<string | number | boolean>;
  not_equals?: string | number | boolean;
  not_in?: Array<string | number | boolean>;
  truthy?: boolean;
  falsy?: boolean;
};

export type PluginActionFormField = {
  key?: string;
  type?: string;
  label?: string;
  description?: string;
  placeholder?: string;
  rows?: number;
  options?: Array<PluginActionFormFieldOption | string | number | boolean>;
  required?: boolean;
  visible_when?: FrontendUnknownMap | PluginActionFormConditionMatcher | PluginActionFormConditionMatcher[];
  required_when?: FrontendUnknownMap | PluginActionFormConditionMatcher | PluginActionFormConditionMatcher[];
};

export type PluginActionFormExtraAction = {
  key?: string;
  label?: string;
  action?: string;
  variant?: "default" | "outline" | "secondary" | "destructive";
  include_fields?: boolean;
  mode?: "default" | "stream";
  stream?: boolean;
  execute_mode?: "default" | "stream";
  required_fields?: string[];
  visible_when?: FrontendUnknownMap | PluginActionFormConditionMatcher | PluginActionFormConditionMatcher[];
};

export type PluginActionFormPreset = {
  key?: string;
  label?: string;
  description?: string;
  values?: FrontendUnknownMap;
};

export type PluginActionFormBlockOptions = {
  title?: string;
  initial?: FrontendUnknownMap;
  autoload?: boolean;
  autoload_include_fields?: boolean;
  presets?: PluginActionFormPreset[];
  remember_recent?: boolean;
  recent_key?: string;
  recent_title?: string;
  recent_limit?: number;
  recent_label_fields?: string[];
  stream_actions?: string[];
  execute_stream_actions?: string[];
  load?: string;
  loadLabel?: string;
  loadMode?: "default" | "stream";
  loadStream?: boolean;
  loadRequiredFields?: string[];
  loadVisibleWhen?: FrontendUnknownMap | PluginActionFormConditionMatcher | PluginActionFormConditionMatcher[];
  save?: string;
  saveLabel?: string;
  saveMode?: "default" | "stream";
  saveStream?: boolean;
  saveRequiredFields?: string[];
  saveVisibleWhen?: FrontendUnknownMap | PluginActionFormConditionMatcher | PluginActionFormConditionMatcher[];
  reset?: string;
  resetLabel?: string;
  resetMode?: "default" | "stream";
  resetStream?: boolean;
  resetRequiredFields?: string[];
  resetVisibleWhen?: FrontendUnknownMap | PluginActionFormConditionMatcher | PluginActionFormConditionMatcher[];
  extra?: PluginActionFormExtraAction[];
  fields?: PluginActionFormField[];
};

export type PluginLinkListItem = {
  label: string;
  url: string;
  target?: "_self" | "_blank";
};

export type PluginLinkListBlockOptions = {
  title?: string;
  links: PluginLinkListItem[];
};

export type PluginActionButtonVariant =
  | "default"
  | "active"
  | "outline"
  | "secondary"
  | "destructive"
  | "ghost"
  | "link";

export type PluginActionButtonSize = "default" | "sm" | "lg" | "icon";

export type PluginActionButtonExtensionOptions = {
  slot: string;
  title: string;
  href: string;
  priority?: number;
  icon?: string;
  variant?: PluginActionButtonVariant;
  size?: PluginActionButtonSize;
  target?: "_self" | "_blank" | "_parent" | "_top";
  external?: boolean;
  type?: "action_button" | "toolbar_button" | "button";
};

export type PluginMenuItemExtensionOptions = {
  area: PluginFrontendArea;
  path: string;
  title: string;
  priority?: number;
  icon?: string;
  required_permissions?: string[];
  super_admin_only?: boolean;
  guest_visible?: boolean;
  mobile_visible?: boolean;
};

export type PluginRoutePageExtensionOptions = {
  area: PluginFrontendArea;
  path: string;
  title: string;
  page: PluginPageSchema;
  priority?: number;
  execute_actions?: string[];
  stream_actions?: string[];
  execute_stream_actions?: string[];
  required_permissions?: string[];
  super_admin_only?: boolean;
  guest_visible?: boolean;
};

export type PluginPageBootstrapOptions = {
  area: PluginFrontendArea;
  path: string;
  title: string;
  page: PluginPageSchema;
  priority?: number;
  icon?: string;
  execute_actions?: string[];
  stream_actions?: string[];
  execute_stream_actions?: string[];
  required_permissions?: string[];
  super_admin_only?: boolean;
  guest_visible?: boolean;
  mobile_visible?: boolean;
};

export type PluginExecuteTemplateStaticKey =
  | "plugin.id"
  | "plugin.name"
  | "plugin.area"
  | "plugin.path"
  | "plugin.full_path"
  | "plugin.query_string"
  | "plugin.query_params_json"
  | "plugin.route_params_json"
  | "plugin.execute_api_url"
  | "plugin.execute_api_method"
  | "plugin.execute_api_scope"
  | "plugin.execute_api_requires_auth"
  | "plugin.execute_stream_url"
  | "plugin.execute_stream_format"
  | "plugin.execute_stream_actions"
  | "plugin.execute_api_json";

export type PluginExecuteTemplateKey =
  | PluginExecuteTemplateStaticKey
  | `plugin.query.${string}`
  | `plugin.route.${string}`;

export const PLUGIN_EXECUTE_TEMPLATE_PLACEHOLDERS: Record<PluginExecuteTemplateStaticKey, string> = {
  "plugin.id": "{{plugin.id}}",
  "plugin.name": "{{plugin.name}}",
  "plugin.area": "{{plugin.area}}",
  "plugin.path": "{{plugin.path}}",
  "plugin.full_path": "{{plugin.full_path}}",
  "plugin.query_string": "{{plugin.query_string}}",
  "plugin.query_params_json": "{{plugin.query_params_json}}",
  "plugin.route_params_json": "{{plugin.route_params_json}}",
  "plugin.execute_api_url": "{{plugin.execute_api_url}}",
  "plugin.execute_api_method": "{{plugin.execute_api_method}}",
  "plugin.execute_api_scope": "{{plugin.execute_api_scope}}",
  "plugin.execute_api_requires_auth": "{{plugin.execute_api_requires_auth}}",
  "plugin.execute_stream_url": "{{plugin.execute_stream_url}}",
  "plugin.execute_stream_format": "{{plugin.execute_stream_format}}",
  "plugin.execute_stream_actions": "{{plugin.execute_stream_actions}}",
  "plugin.execute_api_json": "{{plugin.execute_api_json}}"
};

export function buildPluginExecuteTemplatePlaceholder(key: PluginExecuteTemplateKey): string {
  return `{{${asTrimmedString(key)}}}`;
}

export type PluginExecuteTemplateValuesInput = {
  pluginID?: number | string;
  pluginName?: string;
  area?: PluginFrontendArea | string;
  path?: string;
  full_path?: string;
  query_string?: string;
  query_params?: FrontendStringMap | FrontendUnknownMap;
  query_params_json?: string;
  route_params?: FrontendStringMap | FrontendUnknownMap;
  route_params_json?: string;
  execute_api_url?: string;
  execute_api_method?: string;
  execute_api_scope?: string;
  execute_api_requires_auth?: boolean | string;
  execute_stream_url?: string;
  execute_stream_format?: string;
  execute_stream_actions?: string;
  execute_api_json?: string;
};

export type PluginExecuteHTMLBridgeOptions = {
  action: string;
  mode?: "default" | "stream";
  target?: string;
  intro?: string;
  messageLabel?: string;
  messageField?: string;
  messageValue?: string;
  jsonLabel?: string;
  jsonField?: string;
  jsonValue?: unknown;
  submitLabel?: string;
  quickActionLabel?: string;
  quickAction?: string;
  quickActionMode?: "default" | "stream";
  quickActionParams?: FrontendUnknownMap;
  showMetadata?: boolean;
};

export function buildPluginActionFormBlock(options: PluginActionFormBlockOptions): PluginPageBlock {
  const streamActions = normalizeStringList([
    ...(Array.isArray(options.stream_actions) ? options.stream_actions : []),
    ...(Array.isArray(options.execute_stream_actions) ? options.execute_stream_actions : [])
  ]);
  return {
    type: "action_form",
    title: asTrimmedString(options.title) || undefined,
    data: compactRecord({
      initial: options.initial,
      autoload: options.autoload,
      autoload_include_fields: options.autoload_include_fields,
      presets: Array.isArray(options.presets) ? options.presets : undefined,
      remember_recent: options.remember_recent === true ? true : undefined,
      recent_key: asTrimmedString(options.recent_key) || undefined,
      recent_title: asTrimmedString(options.recent_title) || undefined,
      recent_limit:
        typeof options.recent_limit === "number" && Number.isFinite(options.recent_limit)
          ? Math.max(1, Math.min(12, Math.trunc(options.recent_limit)))
          : undefined,
      recent_label_fields:
        Array.isArray(options.recent_label_fields) && options.recent_label_fields.length > 0
          ? options.recent_label_fields
          : undefined,
      stream_actions: streamActions,
      actions: compactRecord({
        load: asTrimmedString(options.load) || undefined,
        load_label: asTrimmedString(options.loadLabel) || undefined,
        load_mode: normalizeExecuteMode(options.loadMode),
        load_stream: options.loadStream === true ? true : undefined,
        load_required_fields:
          Array.isArray(options.loadRequiredFields) && options.loadRequiredFields.length > 0
            ? options.loadRequiredFields
            : undefined,
        load_visible_when: options.loadVisibleWhen,
        save: asTrimmedString(options.save) || undefined,
        save_label: asTrimmedString(options.saveLabel) || undefined,
        save_mode: normalizeExecuteMode(options.saveMode),
        save_stream: options.saveStream === true ? true : undefined,
        save_required_fields:
          Array.isArray(options.saveRequiredFields) && options.saveRequiredFields.length > 0
            ? options.saveRequiredFields
            : undefined,
        save_visible_when: options.saveVisibleWhen,
        reset: asTrimmedString(options.reset) || undefined,
        reset_label: asTrimmedString(options.resetLabel) || undefined,
        reset_mode: normalizeExecuteMode(options.resetMode),
        reset_stream: options.resetStream === true ? true : undefined,
        reset_required_fields:
          Array.isArray(options.resetRequiredFields) && options.resetRequiredFields.length > 0
            ? options.resetRequiredFields
            : undefined,
        reset_visible_when: options.resetVisibleWhen,
        extra: Array.isArray(options.extra) ? options.extra : undefined
      }),
      fields: Array.isArray(options.fields) ? options.fields : undefined
    })
  };
}

export function buildPluginHTMLBlock(
  content: string,
  title?: string,
  data?: FrontendUnknownMap
): PluginPageBlock {
  return {
    type: "html",
    title: asTrimmedString(title) || undefined,
    content,
    data: data && typeof data === "object" ? compactRecord(data) : undefined
  };
}

export function buildPluginTextBlock(content: string, title?: string): PluginPageBlock {
  return {
    type: "text",
    title: asTrimmedString(title) || undefined,
    content: asTrimmedString(content) || undefined
  };
}

export function buildPluginAlertBlock(options: PluginAlertBlockOptions): PluginPageBlock {
  return {
    type: "alert",
    title: asTrimmedString(options.title) || undefined,
    content: asTrimmedString(options.content) || undefined,
    data: compactRecord({
      variant: asTrimmedString(options.variant) || undefined
    })
  };
}

export function buildPluginKeyValueBlock(options: PluginKeyValueBlockOptions): PluginPageBlock {
  return {
    type: "key_value",
    title: asTrimmedString(options.title) || undefined,
    data: compactRecord({
      items: Array.isArray(options.items)
        ? options.items.map((item) =>
            compactRecord({
              key: asTrimmedString(item.key) || undefined,
              label: asTrimmedString(item.label),
              value: item.value,
              description: asTrimmedString(item.description) || undefined
            })
          )
        : undefined
    })
  };
}

export function buildPluginJSONViewBlock(options: PluginJSONViewBlockOptions): PluginPageBlock {
  return {
    type: "json_view",
    title: asTrimmedString(options.title) || undefined,
    data: compactRecord({
      value: options.value,
      summary: asTrimmedString(options.summary) || undefined,
      collapsible: options.collapsible === true ? true : undefined,
      collapsed: options.collapsed === true ? true : undefined,
      preview_lines: typeof options.preview_lines === "number" ? options.preview_lines : undefined,
      max_height: typeof options.max_height === "number" ? options.max_height : undefined
    })
  };
}

export function buildPluginTableBlock(options: PluginTableBlockOptions): PluginPageBlock {
  return {
    type: "table",
    title: asTrimmedString(options.title) || undefined,
    content: asTrimmedString(options.content) || undefined,
    data: compactRecord({
      columns: Array.isArray(options.columns) ? options.columns.map((item) => asTrimmedString(item)) : undefined,
      rows: Array.isArray(options.rows) ? options.rows : undefined,
      empty_text: asTrimmedString(options.empty_text) || undefined
    })
  };
}

export function buildPluginBadgeListBlock(options: PluginBadgeListBlockOptions): PluginPageBlock {
  return {
    type: "badge_list",
    title: asTrimmedString(options.title) || undefined,
    data: compactRecord({
      items: Array.isArray(options.items)
        ? options.items.filter((item) => item !== undefined && item !== null && String(item).trim() !== "")
        : undefined
    })
  };
}

export function buildPluginLinkListBlock(options: PluginLinkListBlockOptions): PluginPageBlock {
  return {
    type: "link_list",
    title: asTrimmedString(options.title) || undefined,
    data: compactRecord({
      links: Array.isArray(options.links)
        ? options.links.map((item) =>
            compactRecord({
              label: asTrimmedString(item.label),
              url: asTrimmedString(item.url),
              target: asTrimmedString(item.target) || undefined
            })
          )
        : undefined
    })
  };
}

export function buildPluginStatsGridBlock(options: PluginStatsGridBlockOptions): PluginPageBlock {
  return {
    type: "stats_grid",
    title: asTrimmedString(options.title) || undefined,
    data: compactRecord({
      items: Array.isArray(options.items)
        ? options.items.map((item) =>
            compactRecord({
              label: asTrimmedString(item.label),
              value: item.value,
              description: asTrimmedString(item.description) || undefined
            })
          )
        : undefined
    })
  };
}

export function buildPluginActionButtonExtension(
  options: PluginActionButtonExtensionOptions
): PluginFrontendExtension {
  return compactRecord({
    type: asTrimmedString(options.type) || "action_button",
    slot: asTrimmedString(options.slot) || undefined,
    title: asTrimmedString(options.title) || undefined,
    link: asTrimmedString(options.href) || undefined,
    priority: options.priority,
    data: compactRecord({
      label: asTrimmedString(options.title) || undefined,
      href: asTrimmedString(options.href) || undefined,
      icon: asTrimmedString(options.icon) || undefined,
      variant: asTrimmedString(options.variant) || undefined,
      size: asTrimmedString(options.size) || undefined,
      target: asTrimmedString(options.target) || undefined,
      external: options.external === true ? true : undefined
    })
  }) as PluginFrontendExtension;
}

export function buildPluginMenuItemExtension(
  options: PluginMenuItemExtensionOptions
): PluginFrontendExtension {
  return {
    type: "menu_item",
    data: compactRecord({
      area: options.area,
      path: normalizeFrontendPath(options.path),
      title: asTrimmedString(options.title) || normalizeFrontendPath(options.path),
      priority: options.priority,
      icon: asTrimmedString(options.icon) || undefined,
      required_permissions: normalizeStringList(options.required_permissions),
      super_admin_only: options.super_admin_only,
      guest_visible: options.guest_visible,
      mobile_visible: options.mobile_visible
    })
  };
}

export function buildPluginRoutePageExtension(
  options: PluginRoutePageExtensionOptions
): PluginFrontendExtension {
  const executeActions =
    normalizeStringList(options.execute_actions) || collectPluginPageExecuteActions(options.page);
  const streamActions =
    normalizeStringList([
      ...(Array.isArray(options.stream_actions) ? options.stream_actions : []),
      ...(Array.isArray(options.execute_stream_actions) ? options.execute_stream_actions : [])
    ]) || collectPluginPageStreamActions(options.page);
  return {
    type: "route_page",
    data: compactRecord({
      area: options.area,
      path: normalizeFrontendPath(options.path),
      title: asTrimmedString(options.title) || normalizeFrontendPath(options.path),
      priority: options.priority,
      execute_actions: executeActions,
      stream_actions: streamActions,
      required_permissions: normalizeStringList(options.required_permissions),
      super_admin_only: options.super_admin_only,
      guest_visible: options.guest_visible,
      page: options.page
    })
  };
}

export function buildPluginPageBootstrap(
  options: PluginPageBootstrapOptions
): PluginFrontendExtension[] {
  return [
    buildPluginMenuItemExtension(options),
    buildPluginRoutePageExtension(options)
  ];
}

export function buildPluginExecuteTemplateValues(
  input: PluginExecuteTemplateValuesInput
): FrontendStringMap {
  const path = asTrimmedString(input.path);
  const queryParams = resolveTemplateStringMap(input.query_params, input.query_params_json);
  const routeParams = resolveTemplateStringMap(input.route_params, input.route_params_json);
  const queryString = asTrimmedString(input.query_string) || buildTemplateQueryString(queryParams);
  const fullPath = asTrimmedString(input.full_path) || buildTemplateFullPath(path, queryString);
  const queryParamsJSON =
    asTrimmedString(input.query_params_json) || stringifyTemplateStringMap(queryParams);
  const routeParamsJSON =
    asTrimmedString(input.route_params_json) || stringifyTemplateStringMap(routeParams);

  const output: FrontendStringMap = {
    "plugin.id": String(input.pluginID ?? ""),
    "plugin.name": asTrimmedString(input.pluginName),
    "plugin.area": asTrimmedString(input.area),
    "plugin.path": path,
    "plugin.full_path": fullPath,
    "plugin.query_string": queryString,
    "plugin.query_params_json": queryParamsJSON,
    "plugin.route_params_json": routeParamsJSON,
    "plugin.execute_api_url": asTrimmedString(input.execute_api_url),
    "plugin.execute_api_method": asTrimmedString(input.execute_api_method).toUpperCase(),
    "plugin.execute_api_scope": asTrimmedString(input.execute_api_scope),
    "plugin.execute_api_requires_auth":
      typeof input.execute_api_requires_auth === "boolean"
        ? String(input.execute_api_requires_auth)
        : asTrimmedString(input.execute_api_requires_auth),
    "plugin.execute_stream_url": asTrimmedString(input.execute_stream_url),
    "plugin.execute_stream_format": asTrimmedString(input.execute_stream_format),
    "plugin.execute_stream_actions": asTrimmedString(input.execute_stream_actions),
    "plugin.execute_api_json": asTrimmedString(input.execute_api_json)
  };

  Object.entries(queryParams).forEach(([key, value]) => {
    output[`plugin.query.${key}`] = value;
  });
  Object.entries(routeParams).forEach(([key, value]) => {
    output[`plugin.route.${key}`] = value;
  });

  return output;
}

function normalizeTemplateStringMap(
  value?: FrontendStringMap | FrontendUnknownMap
): FrontendStringMap {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return {};
  }
  const output: FrontendStringMap = {};
  Object.entries(value).forEach(([key, item]) => {
    const normalizedKey = asTrimmedString(key);
    if (!normalizedKey) {
      return;
    }
    output[normalizedKey] = item === undefined || item === null ? "" : String(item);
  });
  return output;
}

function resolveTemplateStringMap(
  value?: FrontendStringMap | FrontendUnknownMap,
  fallbackJSON?: string
): FrontendStringMap {
  const normalized = normalizeTemplateStringMap(value);
  if (Object.keys(normalized).length > 0) {
    return normalized;
  }
  return normalizeTemplateStringMap(parseTemplateJSONObject(fallbackJSON));
}

function parseTemplateJSONObject(raw?: string): FrontendUnknownMap {
  if (typeof raw !== "string" || raw.trim() === "") {
    return {};
  }
  try {
    const parsed = JSON.parse(raw);
    return parsed && typeof parsed === "object" && !Array.isArray(parsed)
      ? (parsed as FrontendUnknownMap)
      : {};
  } catch {
    return {};
  }
}

function stringifyTemplateStringMap(value: FrontendStringMap): string {
  const normalized = Object.keys(value)
    .sort((left, right) => left.localeCompare(right))
    .reduce<FrontendStringMap>((acc, key) => {
      acc[key] = value[key] ?? "";
      return acc;
    }, {});
  return JSON.stringify(normalized);
}

function buildTemplateQueryString(value: FrontendStringMap): string {
  const query = new URLSearchParams();
  Object.keys(value)
    .sort((left, right) => left.localeCompare(right))
    .forEach((key) => {
      query.append(key, value[key] ?? "");
    });
  return query.toString();
}

function buildTemplateFullPath(path: string, queryString: string): string {
  if (!path) {
    return queryString ? `?${queryString}` : "";
  }
  if (!queryString) {
    return path;
  }
  return `${path}?${queryString}`;
}

export function renderPluginTemplate(template: string, values: FrontendStringMap): string {
  const source = String(template || "");
  return source.replace(/\{\{\s*([a-zA-Z0-9._-]+)\s*\}\}/g, (matched, key: string) => {
    if (!Object.prototype.hasOwnProperty.call(values, key)) {
      return matched;
    }
    return values[key] ?? "";
  });
}

export function buildPluginExecuteHTMLBridge(options: PluginExecuteHTMLBridgeOptions): string {
  const target = asTrimmedString(options.target) || "plugin-exec";
  const action = asTrimmedString(options.action);
  const mode = normalizeExecuteMode(options.mode);
  const quickAction = asTrimmedString(options.quickAction) || action;
  const quickActionMode = normalizeExecuteMode(options.quickActionMode) || mode;
  const intro =
    asTrimmedString(options.intro) ||
    "This HTML block uses route-provided execute API metadata and the plugin execute bridge.";
  const messageField = asTrimmedString(options.messageField) || "echo_message";
  const messageLabel = asTrimmedString(options.messageLabel) || "Message";
  const messageValue =
    asTrimmedString(options.messageValue) ||
    `Plugin execute on ${PLUGIN_EXECUTE_TEMPLATE_PLACEHOLDERS["plugin.full_path"]}`;
  const jsonField = asTrimmedString(options.jsonField) || "echo_json";
  const jsonLabel = asTrimmedString(options.jsonLabel) || "JSON Payload";
  const jsonValue = prettyJSON(
    options.jsonValue ?? {
      source: "html-bridge",
      plugin_id: PLUGIN_EXECUTE_TEMPLATE_PLACEHOLDERS["plugin.id"],
      area: PLUGIN_EXECUTE_TEMPLATE_PLACEHOLDERS["plugin.area"],
      path: PLUGIN_EXECUTE_TEMPLATE_PLACEHOLDERS["plugin.path"],
      full_path: PLUGIN_EXECUTE_TEMPLATE_PLACEHOLDERS["plugin.full_path"],
      query: PLUGIN_EXECUTE_TEMPLATE_PLACEHOLDERS["plugin.query_params_json"],
      route: PLUGIN_EXECUTE_TEMPLATE_PLACEHOLDERS["plugin.route_params_json"]
    }
  );
  const submitLabel = asTrimmedString(options.submitLabel) || "Run Exec";
  const quickActionLabel = asTrimmedString(options.quickActionLabel);
  const quickActionParams = prettyJSON(options.quickActionParams ?? { source: "html-button" });

  const metadataHTML = options.showMetadata === false
    ? ""
    : [
        "<div>",
        `<div>URL: <code>${escapeHTML(PLUGIN_EXECUTE_TEMPLATE_PLACEHOLDERS["plugin.execute_api_url"])}</code></div>`,
        `<div>Method: <code>${escapeHTML(PLUGIN_EXECUTE_TEMPLATE_PLACEHOLDERS["plugin.execute_api_method"])}</code></div>`,
        `<div>Stream URL: <code>${escapeHTML(PLUGIN_EXECUTE_TEMPLATE_PLACEHOLDERS["plugin.execute_stream_url"])}</code></div>`,
        `<div>Stream Format: <code>${escapeHTML(PLUGIN_EXECUTE_TEMPLATE_PLACEHOLDERS["plugin.execute_stream_format"])}</code></div>`,
        `<div>Area: <code>${escapeHTML(PLUGIN_EXECUTE_TEMPLATE_PLACEHOLDERS["plugin.area"])}</code></div>`,
        `<div>Path: <code>${escapeHTML(PLUGIN_EXECUTE_TEMPLATE_PLACEHOLDERS["plugin.path"])}</code></div>`,
        `<div>Full Path: <code>${escapeHTML(PLUGIN_EXECUTE_TEMPLATE_PLACEHOLDERS["plugin.full_path"])}</code></div>`,
        `<div>Query String: <code>${escapeHTML(PLUGIN_EXECUTE_TEMPLATE_PLACEHOLDERS["plugin.query_string"])}</code></div>`,
        `<div>Query Params: <code>${escapeHTML(PLUGIN_EXECUTE_TEMPLATE_PLACEHOLDERS["plugin.query_params_json"])}</code></div>`,
        `<div>Route Params: <code>${escapeHTML(PLUGIN_EXECUTE_TEMPLATE_PLACEHOLDERS["plugin.route_params_json"])}</code></div>`,
        "</div>"
      ].join("");

  const quickButtonHTML = quickActionLabel
    ? ` <button type="button" data-plugin-exec-action="${escapeHTMLAttribute(quickAction)}" data-plugin-exec-target="${escapeHTMLAttribute(target)}"${quickActionMode ? ` data-plugin-exec-mode="${escapeHTMLAttribute(quickActionMode)}"` : ""} data-plugin-exec-params='${escapeHTMLAttribute(
        quickActionParams
      )}'>${escapeHTML(quickActionLabel)}</button>`
    : "";

  return [
    "<div>",
    `<p>${escapeHTML(intro)}</p>`,
    metadataHTML,
    `<form data-plugin-exec-form="true" data-plugin-exec-action="${escapeHTMLAttribute(action)}" data-plugin-exec-target="${escapeHTMLAttribute(target)}"${mode ? ` data-plugin-exec-mode="${escapeHTMLAttribute(mode)}"` : ""}>`,
    `<p><label>${escapeHTML(messageLabel)}<br /><input name="${escapeHTMLAttribute(messageField)}" value="${escapeHTMLAttribute(messageValue)}" /></label></p>`,
    `<p><label>${escapeHTML(jsonLabel)}<br /><textarea name="${escapeHTMLAttribute(jsonField)}" rows="8" cols="80">${escapeHTML(jsonValue)}</textarea></label></p>`,
    `<p><button type="submit">${escapeHTML(submitLabel)}</button>${quickButtonHTML}</p>`,
    "</form>",
    "<div>",
    "<div>Status</div>",
    `<pre data-plugin-exec-status="${escapeHTMLAttribute(target)}">Idle</pre>`,
    "<div>Error</div>",
    `<pre data-plugin-exec-error="${escapeHTMLAttribute(target)}"></pre>`,
    "<div>Result</div>",
    `<pre data-plugin-exec-result="${escapeHTMLAttribute(target)}"></pre>`,
    "</div>",
    "</div>"
  ].join("");
}

export function prettyJSON(value: unknown): string {
  if (typeof value === "string") {
    try {
      return JSON.stringify(JSON.parse(value), null, 2);
    } catch {
      return value;
    }
  }
  try {
    return JSON.stringify(value ?? {}, null, 2);
  } catch {
    return String(value ?? "");
  }
}

function compactRecord<T extends FrontendUnknownMap>(value: T): T {
  const output: FrontendUnknownMap = {};
  Object.entries(value).forEach(([key, item]) => {
    if (item === undefined) {
      return;
    }
    if (Array.isArray(item) && item.length === 0) {
      return;
    }
    if (item && typeof item === "object" && !Array.isArray(item) && Object.keys(item).length === 0) {
      return;
    }
    output[key] = item;
  });
  return output as T;
}

function normalizeFrontendPath(path: string): string {
  const trimmed = asTrimmedString(path);
  if (!trimmed) {
    return "/";
  }
  return trimmed.startsWith("/") ? trimmed : `/${trimmed}`;
}

function normalizeStringList(values?: string[]): string[] | undefined {
  if (!Array.isArray(values) || values.length === 0) {
    return undefined;
  }
  const seen = new Set<string>();
  const output: string[] = [];
  values.forEach((item) => {
    const normalized = asTrimmedString(item);
    if (!normalized || seen.has(normalized)) {
      return;
    }
    seen.add(normalized);
    output.push(normalized);
  });
  return output.length > 0 ? output : undefined;
}

function collectPluginPageExecuteActions(page?: PluginPageSchema): string[] | undefined {
  if (!page || typeof page !== "object") {
    return undefined;
  }

  const output: string[] = [];
  if (Array.isArray(page.execute_actions)) {
    output.push(...page.execute_actions);
  }

  if (Array.isArray(page.blocks)) {
    page.blocks.forEach((block) => {
      output.push(...collectPluginBlockExecuteActions(block));
    });
  }

  return normalizeStringList(output);
}

function collectPluginPageStreamActions(page?: PluginPageSchema): string[] | undefined {
  if (!page || typeof page !== "object") {
    return undefined;
  }

  const output: string[] = [];
  if (Array.isArray(page.stream_actions)) {
    output.push(...page.stream_actions);
  }
  if (Array.isArray(page.execute_stream_actions)) {
    output.push(...page.execute_stream_actions);
  }

  if (Array.isArray(page.blocks)) {
    page.blocks.forEach((block) => {
      output.push(...collectPluginBlockStreamActions(block));
    });
  }

  return normalizeStringList(output);
}

function collectPluginBlockExecuteActions(block?: PluginPageBlock): string[] {
  if (!block || typeof block !== "object") {
    return [];
  }

  const output: string[] = [];
  const data = block.data && typeof block.data === "object" ? block.data : undefined;

  if (Array.isArray((block as PluginPageBlock & { execute_actions?: string[] }).execute_actions)) {
    output.push(...(((block as PluginPageBlock & { execute_actions?: string[] }).execute_actions) || []));
  }
  if (data && Array.isArray((data as FrontendUnknownMap & { execute_actions?: string[] }).execute_actions)) {
    output.push(...(((data as FrontendUnknownMap & { execute_actions?: string[] }).execute_actions) || []));
  }

  if (asTrimmedString(block.type).toLowerCase() !== "action_form" || !data) {
    return normalizeStringList(output) || [];
  }

  const actions = data.actions && typeof data.actions === "object"
    ? data.actions as FrontendUnknownMap
    : undefined;
  if (!actions) {
    return normalizeStringList(output) || [];
  }

  output.push(
    asTrimmedString(actions.load),
    asTrimmedString(actions.save),
    asTrimmedString(actions.reset)
  );

  const extras = Array.isArray(actions.extra)
    ? actions.extra
    : (Array.isArray(actions.buttons) ? actions.buttons : []);
  extras.forEach((item) => {
    if (!item || typeof item !== "object") {
      return;
    }
    output.push(asTrimmedString((item as FrontendUnknownMap).action));
  });

  return normalizeStringList(output) || [];
}

function collectPluginBlockStreamActions(block?: PluginPageBlock): string[] {
  if (!block || typeof block !== "object") {
    return [];
  }

  const output: string[] = [];
  const data = block.data && typeof block.data === "object" ? block.data : undefined;

  if (Array.isArray(block.stream_actions)) {
    output.push(...block.stream_actions);
  }
  if (Array.isArray(block.execute_stream_actions)) {
    output.push(...block.execute_stream_actions);
  }
  if (data && Array.isArray((data as FrontendUnknownMap & { stream_actions?: string[] }).stream_actions)) {
    output.push(...(((data as FrontendUnknownMap & { stream_actions?: string[] }).stream_actions) || []));
  }
  if (
    data &&
    Array.isArray((data as FrontendUnknownMap & { execute_stream_actions?: string[] }).execute_stream_actions)
  ) {
    output.push(
      ...(((data as FrontendUnknownMap & { execute_stream_actions?: string[] }).execute_stream_actions) || [])
    );
  }

  if (asTrimmedString(block.type).toLowerCase() !== "action_form" || !data) {
    return normalizeStringList(output) || [];
  }

  const actions = data.actions && typeof data.actions === "object"
    ? data.actions as FrontendUnknownMap
    : undefined;
  if (!actions) {
    return normalizeStringList(output) || [];
  }

  const load = asTrimmedString(actions.load);
  const save = asTrimmedString(actions.save);
  const reset = asTrimmedString(actions.reset);
  if (isStreamActionMode(actions.load_mode, actions.load_stream)) {
    output.push(load);
  }
  if (isStreamActionMode(actions.save_mode, actions.save_stream)) {
    output.push(save);
  }
  if (isStreamActionMode(actions.reset_mode, actions.reset_stream)) {
    output.push(reset);
  }

  const extras = Array.isArray(actions.extra)
    ? actions.extra
    : (Array.isArray(actions.buttons) ? actions.buttons : []);
  extras.forEach((item) => {
    if (!item || typeof item !== "object") {
      return;
    }
    const extra = item as FrontendUnknownMap;
    if (!isStreamActionMode(extra.mode, extra.stream, extra.execute_mode)) {
      return;
    }
    output.push(asTrimmedString(extra.action));
  });

  return normalizeStringList(output) || [];
}

function isStreamActionMode(...values: unknown[]): boolean {
  for (const value of values) {
    if (value === true) {
      return true;
    }
    if (asTrimmedString(value).toLowerCase() === "stream") {
      return true;
    }
  }
  return false;
}

function normalizeExecuteMode(value: unknown): "stream" | undefined {
  return asTrimmedString(value).toLowerCase() === "stream" ? "stream" : undefined;
}

function asTrimmedString(value: unknown): string {
  if (value === null || value === undefined) {
    return "";
  }
  return String(value).trim();
}

function escapeHTML(value: string): string {
  return value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}

function escapeHTMLAttribute(value: string): string {
  return escapeHTML(value).replace(/"/g, "&quot;").replace(/'/g, "&#39;");
}
