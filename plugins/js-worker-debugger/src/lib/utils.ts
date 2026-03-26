import type { PluginRuntimeGlobal, PluginSandboxProfile } from "@auralogic/plugin-sdk";
import type {
  DebuggerSandboxProfile,
  DebuggerStorageAccessMode,
  GenericRecord,
  RuntimeSandboxInput
} from "./types";

export function asString(value: unknown): string {
  if (value === null || value === undefined) {
    return "";
  }
  return String(value).trim();
}

export function asBool(value: unknown, fallback: boolean): boolean {
  if (typeof value === "boolean") {
    return value;
  }
  if (typeof value === "string") {
    const normalized = value.trim().toLowerCase();
    if (["1", "true", "yes", "on"].includes(normalized)) return true;
    if (["0", "false", "no", "off"].includes(normalized)) return false;
  }
  if (typeof value === "number") {
    return value !== 0;
  }
  return fallback;
}

export function asInteger(
  value: unknown,
  fallback: number,
  min?: number,
  max?: number
): number {
  let numeric = fallback;
  if (typeof value === "number" && Number.isFinite(value)) {
    numeric = Math.trunc(value);
  } else if (typeof value === "string" && value.trim() !== "") {
    const parsed = Number.parseInt(value.trim(), 10);
    if (!Number.isNaN(parsed)) {
      numeric = parsed;
    }
  }
  if (typeof min === "number" && numeric < min) {
    numeric = min;
  }
  if (typeof max === "number" && numeric > max) {
    numeric = max;
  }
  return numeric;
}

export function asRecord(value: unknown): GenericRecord {
  return value && typeof value === "object" && !Array.isArray(value)
    ? (value as GenericRecord)
    : {};
}

export function hasOwnKey(value: unknown, key: string): boolean {
  return Boolean(value && typeof value === "object" && Object.prototype.hasOwnProperty.call(value, key));
}

export function normalizeHookName(hook: unknown): string {
  return asString(hook).toLowerCase();
}

export function normalizeStringList(value: unknown): string[] {
  if (!Array.isArray(value)) return [];
  const seen = new Set<string>();
  const out: string[] = [];
  value.forEach((item) => {
    const normalized = asString(item).toLowerCase();
    if (!normalized || seen.has(normalized)) return;
    seen.add(normalized);
    out.push(normalized);
  });
  return out;
}

export function normalizeStorageAccessMode(
  value: unknown,
  fallback: DebuggerStorageAccessMode = "unknown"
): DebuggerStorageAccessMode {
  const normalized = asString(value).toLowerCase();
  switch (normalized) {
    case "none":
      return "none";
    case "read":
      return "read";
    case "write":
      return "write";
    case "unknown":
      return "unknown";
    default:
      return fallback;
  }
}

function normalizeStorageAccessMap(value: unknown): Record<string, DebuggerStorageAccessMode> {
  const record = asRecord(value);
  const output: Record<string, DebuggerStorageAccessMode> = {};
  Object.entries(record).forEach(([action, mode]) => {
    const normalizedAction = asString(action).toLowerCase();
    if (!normalizedAction) {
      return;
    }
    output[normalizedAction] = normalizeStorageAccessMode(mode);
  });
  return output;
}

export function safeParseJSON(raw: unknown): unknown {
  if (typeof raw !== "string" || raw.trim() === "") {
    return {};
  }
  try {
    return JSON.parse(raw);
  } catch {
    return {};
  }
}

export function safeParseJSONObject(raw: unknown): GenericRecord {
  const parsed = safeParseJSON(raw);
  return asRecord(parsed);
}

export function prettyJSON(value: unknown): string {
  try {
    return JSON.stringify(value ?? {}, null, 2);
  } catch {
    return String(value ?? "");
  }
}

export function truncateText(value: unknown, maxLength: number): string {
  const text = typeof value === "string" ? value : prettyJSON(value);
  if (text.length <= maxLength) {
    return text;
  }
  return `${text.slice(0, maxLength)}…`;
}

export function nowISO(): string {
  return new Date().toISOString();
}

export function parseJSONValueIfPossible(value: unknown): unknown {
  if (typeof value !== "string") {
    return value;
  }
  try {
    return JSON.parse(value);
  } catch {
    return value;
  }
}

function toRuntimeSandboxRecords(value: RuntimeSandboxInput): GenericRecord[] {
  const records: GenericRecord[] = [];
  if (globalThis && globalThis.sandbox && typeof globalThis.sandbox === "object") {
    records.push(asRecord(globalThis.sandbox));
  }
  if (value && typeof value === "object") {
    records.push(asRecord(value));
  }
  if (records.length === 0) {
    records.push({});
  }
  return records;
}

function pickSandboxValue(records: GenericRecord[], ...keys: string[]): unknown {
  for (const record of records) {
    for (const key of keys) {
      if (hasOwnKey(record, key)) {
        return record[key];
      }
    }
  }
  return undefined;
}

export function resolveSandboxProfile(value: RuntimeSandboxInput): DebuggerSandboxProfile {
  const sandboxRecords = toRuntimeSandboxRecords(value);
  const executeActionStorage = normalizeStorageAccessMap(
    pickSandboxValue(
      sandboxRecords,
      "executeActionStorage",
      "ExecuteActionStorage",
      "execute_action_storage"
    )
  );
  return {
    level: asString(pickSandboxValue(sandboxRecords, "level", "Level")) || "balanced",
    currentAction: asString(
      pickSandboxValue(sandboxRecords, "currentAction", "CurrentAction", "current_action")
    ).toLowerCase(),
    declaredStorageAccessMode: normalizeStorageAccessMode(
      pickSandboxValue(
        sandboxRecords,
        "declaredStorageAccessMode",
        "DeclaredStorageAccessMode",
        "declared_storage_access_mode"
      )
    ),
    storageAccessMode: normalizeStorageAccessMode(
      pickSandboxValue(
        sandboxRecords,
        "storageAccessMode",
        "StorageAccessMode",
        "storage_access_mode"
      ),
      "none"
    ),
    allowNetwork: asBool(
      pickSandboxValue(sandboxRecords, "allowNetwork", "AllowNetwork", "allow_network"),
      false
    ),
    allowFileSystem: asBool(
      pickSandboxValue(sandboxRecords, "allowFileSystem", "AllowFileSystem", "allow_file_system"),
      false
    ),
    allowHookExecute: asBool(
      pickSandboxValue(sandboxRecords, "allowHookExecute", "AllowHookExecute", "allow_hook_execute"),
      false
    ),
    allowHookBlock: asBool(
      pickSandboxValue(sandboxRecords, "allowHookBlock", "AllowHookBlock", "allow_hook_block"),
      false
    ),
    allowPayloadPatch: asBool(
      pickSandboxValue(sandboxRecords, "allowPayloadPatch", "AllowPayloadPatch", "allow_payload_patch"),
      false
    ),
    allowFrontendExtensions: asBool(
      pickSandboxValue(
        sandboxRecords,
        "allowFrontendExtensions",
        "AllowFrontendExtensions",
        "allow_frontend_extensions"
      ),
      false
    ),
    allowExecuteAPI: asBool(
      pickSandboxValue(sandboxRecords, "allowExecuteAPI", "AllowExecuteAPI", "allow_execute_api"),
      false
    ),
    requestedPermissions: normalizeStringList(
      pickSandboxValue(
        sandboxRecords,
        "requestedPermissions",
        "RequestedPermissions",
        "requested_permissions"
      )
    ),
    grantedPermissions: normalizeStringList(
      pickSandboxValue(
        sandboxRecords,
        "grantedPermissions",
        "GrantedPermissions",
        "granted_permissions"
      )
    ),
    executeActionStorage,
    defaultTimeoutMs: asInteger(
      pickSandboxValue(sandboxRecords, "defaultTimeoutMs", "TimeoutMs", "timeout_ms"),
      0,
      0
    ),
    maxConcurrency: asInteger(
      pickSandboxValue(sandboxRecords, "maxConcurrency", "MaxConcurrency", "max_concurrency"),
      0,
      0
    ),
    maxMemoryMB: asInteger(
      pickSandboxValue(sandboxRecords, "maxMemoryMB", "MaxMemoryMB", "max_memory_mb"),
      0,
      0
    ),
    fsMaxFiles: asInteger(
      pickSandboxValue(sandboxRecords, "fsMaxFiles", "FSMaxFiles", "fs_max_files"),
      0,
      0
    ),
    fsMaxTotalBytes: asInteger(
      pickSandboxValue(sandboxRecords, "fsMaxTotalBytes", "FSMaxTotalBytes", "fs_max_total_bytes"),
      0,
      0
    ),
    fsMaxReadBytes: asInteger(
      pickSandboxValue(sandboxRecords, "fsMaxReadBytes", "FSMaxReadBytes", "fs_max_read_bytes"),
      0,
      0
    ),
    storageMaxKeys: asInteger(
      pickSandboxValue(sandboxRecords, "storageMaxKeys", "StorageMaxKeys", "storage_max_keys"),
      0,
      0
    ),
    storageMaxTotalBytes: asInteger(
      pickSandboxValue(
        sandboxRecords,
        "storageMaxTotalBytes",
        "StorageMaxTotalBytes",
        "storage_max_total_bytes"
      ),
      0,
      0
    ),
    storageMaxValueBytes: asInteger(
      pickSandboxValue(
        sandboxRecords,
        "storageMaxValueBytes",
        "StorageMaxValueBytes",
        "storage_max_value_bytes"
      ),
      0,
      0
    )
  };
}

export function missingPermissions(requested: string[], granted: string[]): string[] {
  const grantedSet = new Set(normalizeStringList(granted));
  return normalizeStringList(requested).filter((item) => !grantedSet.has(item));
}

export function getRuntimePluginGlobal(): PluginRuntimeGlobal | undefined {
  const candidate = (globalThis as unknown as { Plugin?: PluginRuntimeGlobal }).Plugin;
  return candidate && typeof candidate === "object" ? candidate : undefined;
}

export function pluginStorageAvailable(): boolean {
  return Boolean(getRuntimePluginGlobal()?.storage);
}

export function pluginFSAvailable(): boolean {
  return Boolean(getRuntimePluginGlobal()?.fs);
}
