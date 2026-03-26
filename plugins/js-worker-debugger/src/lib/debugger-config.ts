import type { PluginStorageAPI } from "@auralogic/plugin-sdk";
import {
  ALWAYS_ENABLED_HOOKS,
  CONFIG_STORAGE_KEY,
  DEFAULT_DEBUGGER_CONFIG,
  GROUP_KEYS,
  HOOK_GROUPS,
  RESERVED_STORAGE_KEYS,
  SLOT_NOTICE_MODE_OPTIONS
} from "./constants";
import type { DebuggerConfig, GenericRecord, HookGroupKey, SlotNoticeMode } from "./types";
import {
  asBool,
  asInteger,
  asRecord,
  asString,
  getRuntimePluginGlobal,
  hasOwnKey,
  normalizeHookName
} from "./utils";

const CONFIG_KEYS: Array<keyof DebuggerConfig> = [
  "enable_frontend",
  "enable_auth",
  "enable_platform",
  "enable_commerce",
  "enable_catalog",
  "enable_support",
  "enable_content",
  "enable_settings",
  "emit_frontend_extensions",
  "emit_payload_marker",
  "persist_events",
  "max_events",
  "demo_block_before_hooks",
  "block_keyword",
  "slot_notice_mode"
];

const GROUP_FLAG_MAP: Record<HookGroupKey, keyof DebuggerConfig> = {
  frontend: "enable_frontend",
  auth: "enable_auth",
  platform: "enable_platform",
  commerce: "enable_commerce",
  catalog: "enable_catalog",
  support: "enable_support",
  content: "enable_content",
  settings: "enable_settings"
};

function normalizeSlotNoticeMode(value: unknown): SlotNoticeMode {
  const normalized = asString(value).toLowerCase();
  return SLOT_NOTICE_MODE_OPTIONS.includes(normalized as SlotNoticeMode)
    ? (normalized as SlotNoticeMode)
    : DEFAULT_DEBUGGER_CONFIG.slot_notice_mode;
}

export function getStorageAPI(): PluginStorageAPI | null {
  const runtimePlugin = getRuntimePluginGlobal();
  if (!runtimePlugin || !runtimePlugin.storage) {
    return null;
  }
  const storage = runtimePlugin.storage;
  if (
    typeof storage.get !== "function" ||
    typeof storage.set !== "function" ||
    typeof storage.delete !== "function" ||
    typeof storage.list !== "function"
  ) {
    return null;
  }
  return storage;
}

export function isReservedStorageKey(key: unknown): boolean {
  return RESERVED_STORAGE_KEYS.includes(asString(key));
}

export function normalizeDebuggerConfig(raw: unknown): DebuggerConfig {
  const source = asRecord(raw);
  const hasLegacyCommerce =
    hasOwnKey(source, "enable_order") ||
    hasOwnKey(source, "enable_payment") ||
    hasOwnKey(source, "enable_promo");
  const hasLegacyCatalog = hasOwnKey(source, "enable_product_inventory");
  const hasLegacySupport = hasOwnKey(source, "enable_ticket");
  const legacyCommerceEnabled =
    asBool(source.enable_order, false) ||
    asBool(source.enable_payment, false) ||
    asBool(source.enable_promo, false);
  return {
    enable_frontend: asBool(source.enable_frontend, DEFAULT_DEBUGGER_CONFIG.enable_frontend),
    enable_auth: asBool(source.enable_auth, DEFAULT_DEBUGGER_CONFIG.enable_auth),
    enable_platform: asBool(source.enable_platform, DEFAULT_DEBUGGER_CONFIG.enable_platform),
    enable_commerce: hasOwnKey(source, "enable_commerce")
      ? asBool(source.enable_commerce, DEFAULT_DEBUGGER_CONFIG.enable_commerce)
      : hasLegacyCommerce
        ? legacyCommerceEnabled
        : DEFAULT_DEBUGGER_CONFIG.enable_commerce,
    enable_catalog: hasOwnKey(source, "enable_catalog")
      ? asBool(source.enable_catalog, DEFAULT_DEBUGGER_CONFIG.enable_catalog)
      : hasLegacyCatalog
        ? asBool(source.enable_product_inventory, DEFAULT_DEBUGGER_CONFIG.enable_catalog)
        : DEFAULT_DEBUGGER_CONFIG.enable_catalog,
    enable_support: hasOwnKey(source, "enable_support")
      ? asBool(source.enable_support, DEFAULT_DEBUGGER_CONFIG.enable_support)
      : hasLegacySupport
        ? asBool(source.enable_ticket, DEFAULT_DEBUGGER_CONFIG.enable_support)
        : DEFAULT_DEBUGGER_CONFIG.enable_support,
    enable_content: asBool(source.enable_content, DEFAULT_DEBUGGER_CONFIG.enable_content),
    enable_settings: asBool(source.enable_settings, DEFAULT_DEBUGGER_CONFIG.enable_settings),
    emit_frontend_extensions: asBool(
      source.emit_frontend_extensions,
      DEFAULT_DEBUGGER_CONFIG.emit_frontend_extensions
    ),
    emit_payload_marker: asBool(
      source.emit_payload_marker,
      DEFAULT_DEBUGGER_CONFIG.emit_payload_marker
    ),
    persist_events: asBool(source.persist_events, DEFAULT_DEBUGGER_CONFIG.persist_events),
    max_events: asInteger(source.max_events, DEFAULT_DEBUGGER_CONFIG.max_events, 5, 200),
    demo_block_before_hooks: asBool(
      source.demo_block_before_hooks,
      DEFAULT_DEBUGGER_CONFIG.demo_block_before_hooks
    ),
    block_keyword: asString(source.block_keyword) || DEFAULT_DEBUGGER_CONFIG.block_keyword,
    slot_notice_mode: normalizeSlotNoticeMode(source.slot_notice_mode)
  };
}

function extractManifestConfig(config: unknown): DebuggerConfig {
  const root = asRecord(config);
  const legacyNested = asRecord(root.debugger);
  const merged: GenericRecord = {};
  CONFIG_KEYS.forEach((key) => {
    if (hasOwnKey(legacyNested, key)) {
      merged[key] = legacyNested[key];
    }
  });
  CONFIG_KEYS.forEach((key) => {
    if (hasOwnKey(root, key)) {
      merged[key] = root[key];
    }
  });
  return normalizeDebuggerConfig(merged);
}

function readConfigFromStorage(): DebuggerConfig | null {
  const storage = getStorageAPI();
  if (!storage) {
    return null;
  }
  const raw = storage.get(CONFIG_STORAGE_KEY);
  if (!raw || typeof raw !== "string") {
    return null;
  }
  try {
    return normalizeDebuggerConfig(JSON.parse(raw));
  } catch {
    return null;
  }
}

function writeConfigToStorage(config: DebuggerConfig): boolean {
  const storage = getStorageAPI();
  if (!storage) return false;
  return Boolean(storage.set(CONFIG_STORAGE_KEY, JSON.stringify(normalizeDebuggerConfig(config))));
}

function removeConfigFromStorage(): boolean {
  const storage = getStorageAPI();
  if (!storage) return false;
  return Boolean(storage.delete(CONFIG_STORAGE_KEY));
}

export function resolveDebuggerConfigWithSource(config: unknown): {
  config: DebuggerConfig;
  source: string;
} {
  const fromStorage = readConfigFromStorage();
  if (fromStorage) {
    return {
      config: fromStorage,
      source: "plugin_storage"
    };
  }
  return {
    config: extractManifestConfig(config),
    source: "manifest_config"
  };
}

export function resolveDebuggerConfig(config: unknown): DebuggerConfig {
  return resolveDebuggerConfigWithSource(config).config;
}

function applyConfigPatch(baseConfig: DebuggerConfig, params: unknown): DebuggerConfig {
  const base = normalizeDebuggerConfig(baseConfig);
  const patch = asRecord(params);
  if (typeof patch.config === "string" && patch.config.trim() !== "") {
    try {
      return normalizeDebuggerConfig(JSON.parse(patch.config));
    } catch {
      return base;
    }
  }
  return {
    enable_frontend: hasOwnKey(patch, "enable_frontend")
      ? asBool(patch.enable_frontend, base.enable_frontend)
      : base.enable_frontend,
    enable_auth: hasOwnKey(patch, "enable_auth")
      ? asBool(patch.enable_auth, base.enable_auth)
      : base.enable_auth,
    enable_platform: hasOwnKey(patch, "enable_platform")
      ? asBool(patch.enable_platform, base.enable_platform)
      : base.enable_platform,
    enable_commerce: hasOwnKey(patch, "enable_commerce")
      ? asBool(patch.enable_commerce, base.enable_commerce)
      : base.enable_commerce,
    enable_catalog: hasOwnKey(patch, "enable_catalog")
      ? asBool(patch.enable_catalog, base.enable_catalog)
      : base.enable_catalog,
    enable_support: hasOwnKey(patch, "enable_support")
      ? asBool(patch.enable_support, base.enable_support)
      : base.enable_support,
    enable_content: hasOwnKey(patch, "enable_content")
      ? asBool(patch.enable_content, base.enable_content)
      : base.enable_content,
    enable_settings: hasOwnKey(patch, "enable_settings")
      ? asBool(patch.enable_settings, base.enable_settings)
      : base.enable_settings,
    emit_frontend_extensions: hasOwnKey(patch, "emit_frontend_extensions")
      ? asBool(patch.emit_frontend_extensions, base.emit_frontend_extensions)
      : base.emit_frontend_extensions,
    emit_payload_marker: hasOwnKey(patch, "emit_payload_marker")
      ? asBool(patch.emit_payload_marker, base.emit_payload_marker)
      : base.emit_payload_marker,
    persist_events: hasOwnKey(patch, "persist_events")
      ? asBool(patch.persist_events, base.persist_events)
      : base.persist_events,
    max_events: hasOwnKey(patch, "max_events")
      ? asInteger(patch.max_events, base.max_events, 5, 200)
      : base.max_events,
    demo_block_before_hooks: hasOwnKey(patch, "demo_block_before_hooks")
      ? asBool(patch.demo_block_before_hooks, base.demo_block_before_hooks)
      : base.demo_block_before_hooks,
    block_keyword: hasOwnKey(patch, "block_keyword")
      ? asString(patch.block_keyword) || base.block_keyword
      : base.block_keyword,
    slot_notice_mode: hasOwnKey(patch, "slot_notice_mode")
      ? normalizeSlotNoticeMode(patch.slot_notice_mode)
      : base.slot_notice_mode
  };
}

export function setDebuggerConfig(
  params: unknown,
  config: unknown
): { config: DebuggerConfig; source: string; persisted: boolean } {
  const resolved = resolveDebuggerConfigWithSource(config);
  const next = applyConfigPatch(resolved.config, params);
  const persisted = writeConfigToStorage(next);
  return {
    config: next,
    source: persisted ? "plugin_storage" : resolved.source,
    persisted
  };
}

export function resetDebuggerConfig(
  config: unknown
): { config: DebuggerConfig; source: string; reset: boolean } {
  const reset = removeConfigFromStorage();
  const resolved = resolveDebuggerConfigWithSource(config);
  return {
    config: resolved.config,
    source: resolved.source,
    reset
  };
}

export function resolveEnabledHooks(config: DebuggerConfig): string[] {
  const hooks = [...ALWAYS_ENABLED_HOOKS];
  GROUP_KEYS.forEach((groupKey) => {
    if (!config[GROUP_FLAG_MAP[groupKey]]) {
      return;
    }
    HOOK_GROUPS[groupKey].forEach((hook: string) => hooks.push(normalizeHookName(hook)));
  });
  return Array.from(new Set(hooks.map((item) => normalizeHookName(item)).filter(Boolean)));
}

export function isHookEnabled(config: DebuggerConfig, hook: unknown): boolean {
  return resolveEnabledHooks(config).includes(normalizeHookName(hook));
}

export function findHookGroup(hook: unknown): HookGroupKey | "" {
  const normalized = normalizeHookName(hook);
  if (!normalized) {
    return "";
  }
  for (const groupKey of GROUP_KEYS) {
    if ((HOOK_GROUPS[groupKey] as readonly string[]).includes(normalized)) {
      return groupKey;
    }
  }
  return "";
}
