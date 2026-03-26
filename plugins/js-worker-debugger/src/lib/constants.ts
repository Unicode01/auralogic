import type {
  DebuggerConfig,
  FileSystemFormState,
  HostFormState,
  HookGroupKey,
  NetworkFormState,
  SimulatedHookState,
  SlotNoticeMode,
  StorageFormState,
  WorkerFormState
} from "./types";
import {
  OFFICIAL_FRONTEND_SLOTS,
  OFFICIAL_PLUGIN_HOOK_GROUPS,
  OFFICIAL_PLUGIN_HOOKS
} from "@auralogic/plugin-sdk";

export const HOOK_GROUPS = OFFICIAL_PLUGIN_HOOK_GROUPS;

export const PLUGIN_IDENTITY = "plugin-debugger";
export const PLUGIN_DISPLAY_NAME = "Plugin Debugger";
export const PLUGIN_PAGE_SLUG = "debugger";
export const ADMIN_PLUGIN_PAGE_PATH = `/admin/plugin-pages/${PLUGIN_PAGE_SLUG}`;
export const USER_PLUGIN_PAGE_PATH = `/plugin-pages/${PLUGIN_PAGE_SLUG}`;

export const CONFIG_STORAGE_KEY = "plugin_debugger.config.v3";
export const EVENT_STORAGE_KEY = "plugin_debugger.events.v2";
export const ACTION_TRACE_STORAGE_KEY = "plugin_debugger.actions.v1";
export const DEBUGGER_SECRET_SAMPLE_KEY = "debugger_token";
export const DEBUGGER_WEBHOOK_SAMPLE_KEY = "debugger.inspect";
export const RESERVED_STORAGE_KEYS = [CONFIG_STORAGE_KEY, EVENT_STORAGE_KEY, ACTION_TRACE_STORAGE_KEY];
export const DEFAULT_STORAGE_KEY = "lab.note";
export const DEFAULT_FS_PATH = "notes/debugger-note.txt";

export const GROUP_KEYS: HookGroupKey[] = [
  "frontend",
  "auth",
  "platform",
  "commerce",
  "catalog",
  "support",
  "content",
  "settings"
];

export const ALWAYS_ENABLED_HOOKS = ["frontend.bootstrap"];
export const SLOT_OPTIONS = Array.from(OFFICIAL_FRONTEND_SLOTS);

export const SLOT_NOTICE_MODE_OPTIONS: SlotNoticeMode[] = ["compact", "verbose"];
export const ALL_HOOK_OPTIONS = Array.from(
  new Set<string>([...ALWAYS_ENABLED_HOOKS, ...OFFICIAL_PLUGIN_HOOKS])
).sort((a, b) => a.localeCompare(b));

export const DEFAULT_DEBUGGER_CONFIG: DebuggerConfig = {
  enable_frontend: true,
  enable_auth: true,
  enable_platform: true,
  enable_commerce: true,
  enable_catalog: true,
  enable_support: true,
  enable_content: true,
  enable_settings: true,
  emit_frontend_extensions: false,
  emit_payload_marker: true,
  persist_events: true,
  max_events: 40,
  demo_block_before_hooks: false,
  block_keyword: "debug-block",
  slot_notice_mode: "compact"
};

export const DEFAULT_SIMULATION_STATE: SimulatedHookState = {
  simulate_hook: "frontend.slot.render",
  simulate_area: "admin",
  simulate_slot: "admin.dashboard.top",
  simulate_path: "/admin/dashboard",
  simulate_payload: JSON.stringify(
    {
      area: "admin",
      slot: "admin.dashboard.top",
      path: "/admin/dashboard",
      source: "manual-simulation"
    },
    null,
    2
  )
};

export const DEFAULT_STORAGE_FORM_STATE: StorageFormState = {
  storage_key: DEFAULT_STORAGE_KEY,
  storage_value: "Plugin Debugger lab note"
};

export const DEFAULT_FS_FORM_STATE: FileSystemFormState = {
  fs_path: DEFAULT_FS_PATH,
  fs_content: "Plugin Debugger filesystem probe",
  fs_format: "text"
};

export const DEFAULT_NETWORK_FORM_STATE: NetworkFormState = {
  network_method: "GET",
  network_url: "https://postman-echo.com/get?source=plugin-debugger",
  network_headers: JSON.stringify(
    {
      Accept: "application/json"
    },
    null,
    2
  ),
  network_body: JSON.stringify(
    {
      source: "plugin-debugger",
      ts: "manual"
    },
    null,
    2
  ),
  network_body_format: "json",
  network_timeout_ms: 5000
};

export const DEFAULT_WORKER_FORM_STATE: WorkerFormState = {
  worker_script: "./assets/worker-roundtrip.js",
  worker_request_value: 10,
  worker_second_value: 21,
  worker_message_value: 6
};

export const DEFAULT_HOST_FORM_STATE: HostFormState = {
  host_mode: "order.get",
  host_action: "host.order.get",
  host_payload: JSON.stringify(
    {
      id: 1
    },
    null,
    2
  )
};
