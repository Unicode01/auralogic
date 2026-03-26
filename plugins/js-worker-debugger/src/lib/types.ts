import type {
  PluginFSListEntry,
  PluginFSUsage,
  PluginHTTPResponse,
  PluginExecuteStreamWriter,
  PluginSandboxProfile as RuntimeSandboxProfile,
  PluginStorageAccessMode
} from "@auralogic/plugin-sdk";

export type HookGroupKey =
  | "frontend"
  | "auth"
  | "platform"
  | "commerce"
  | "catalog"
  | "support"
  | "content"
  | "settings";

export type SlotNoticeMode = "compact" | "verbose";
export type NetworkBodyFormat = "json" | "text";
export type DebuggerStorageAccessMode = PluginStorageAccessMode;
export type HostActionMode =
  | "order.get"
  | "order.list"
  | "order.assign_tracking"
  | "order.request_resubmit"
  | "order.mark_paid"
  | "order.update_price"
  | "user.get"
  | "user.list"
  | "product.get"
  | "product.list"
  | "inventory.get"
  | "inventory.list"
  | "inventory_binding.get"
  | "inventory_binding.list"
  | "promo.get"
  | "promo.list"
  | "ticket.get"
  | "ticket.list"
  | "ticket.reply"
  | "ticket.update"
  | "serial.get"
  | "serial.list"
  | "announcement.get"
  | "announcement.list"
  | "knowledge.get"
  | "knowledge.list"
  | "knowledge.categories"
  | "payment_method.get"
  | "payment_method.list"
  | "virtual_inventory.get"
  | "virtual_inventory.list"
  | "virtual_inventory_binding.get"
  | "virtual_inventory_binding.list"
  | "market.source.list"
  | "market.source.get"
  | "market.catalog.list"
  | "market.artifact.get"
  | "market.release.get"
  | "market.install.preview"
  | "market.install.execute"
  | "market.install.task.get"
  | "market.install.task.list"
  | "market.install.history.list"
  | "market.install.rollback"
  | "email_template.list"
  | "email_template.get"
  | "email_template.save"
  | "landing_page.get"
  | "landing_page.save"
  | "landing_page.reset"
  | "invoice_template.get"
  | "invoice_template.save"
  | "invoice_template.reset"
  | "auth_branding.get"
  | "auth_branding.save"
  | "auth_branding.reset"
  | "page_rule_pack.get"
  | "page_rule_pack.save"
  | "page_rule_pack.reset"
  | "host.invoke";

export interface DebuggerConfig {
  enable_frontend: boolean;
  enable_auth: boolean;
  enable_platform: boolean;
  enable_commerce: boolean;
  enable_catalog: boolean;
  enable_support: boolean;
  enable_content: boolean;
  enable_settings: boolean;
  emit_frontend_extensions: boolean;
  emit_payload_marker: boolean;
  persist_events: boolean;
  max_events: number;
  demo_block_before_hooks: boolean;
  block_keyword: string;
  slot_notice_mode: SlotNoticeMode;
}

export interface DebuggerEvent {
  id: string;
  ts: string;
  hook: string;
  group: HookGroupKey | "";
  area?: string;
  slot?: string;
  path?: string;
  user_id?: number;
  order_id?: number;
  session_id?: string;
  blocked?: boolean;
  note?: string;
  payload_json?: string;
  context_json?: string;
}

export interface DebuggerStorageSummary {
  key_count: number;
  reserved_keys: string[];
  lab_keys: string[];
  keys: string[];
}

export interface DebuggerFileSummary {
  enabled: boolean;
  usage?: PluginFSUsage;
  entries: PluginFSListEntry[];
  probe_error?: string;
  data_root?: string;
  max_read_bytes?: number;
}

export interface DebuggerRuntimeProbe {
  hasPluginWorkspace: boolean;
  hasPluginStorage: boolean;
  hasPluginSecret: boolean;
  hasPluginWebhook: boolean;
  hasPluginHTTP: boolean;
  hasPluginFS: boolean;
  hasPluginHost: boolean;
  hasPluginOrder: boolean;
  hasPluginUser: boolean;
  hasPluginProduct: boolean;
  hasPluginInventory: boolean;
  hasPluginInventoryBinding: boolean;
  hasPluginPromo: boolean;
  hasPluginTicket: boolean;
  hasPluginSerial: boolean;
  hasPluginAnnouncement: boolean;
  hasPluginKnowledge: boolean;
  hasPluginPaymentMethod: boolean;
  hasPluginVirtualInventory: boolean;
  hasPluginVirtualInventoryBinding: boolean;
  hasPluginMarket: boolean;
  hasPluginEmailTemplate: boolean;
  hasPluginLandingPage: boolean;
  hasPluginInvoiceTemplate: boolean;
  hasPluginAuthBranding: boolean;
  hasPluginPageRulePack: boolean;
  hasWorkerGlobal: boolean;
  hasStructuredCloneGlobal: boolean;
  hasQueueMicrotaskGlobal: boolean;
  hasSetTimeoutGlobal: boolean;
  hasClearTimeoutGlobal: boolean;
  hasTextEncoderGlobal: boolean;
  hasTextDecoderGlobal: boolean;
  hasAtobGlobal: boolean;
  hasBtoaGlobal: boolean;
  pluginSecretEnabled: boolean;
  pluginWebhookEnabled: boolean;
  pluginWorkspaceEnabled: boolean;
  pluginHTTPEnabled: boolean;
  pluginFSEnabled: boolean;
  pluginHostEnabled: boolean;
  workspaceEntryCount: number;
  workspaceMaxEntries: number;
  sandboxAllowFileSystem: boolean;
  sandboxAllowNetwork: boolean;
  httpDefaultTimeoutMs: number;
  httpMaxResponseBytes: number;
  codeRoot: string;
  dataRoot: string;
  fsMaxFiles: number;
  fsMaxTotalBytes: number;
  fsMaxReadBytes: number;
  pluginGlobalKeys: string[];
  runtimeGlobalKeys: string[];
  workspaceProbeError: string;
  workspaceInterpretation: string;
  fsProbeError: string;
  interpretation: string;
  networkInterpretation: string;
  hostInterpretation: string;
  jsRuntimeInterpretation: string;
}

export interface DebuggerSandboxProfile {
  level: string;
  currentAction: string;
  declaredStorageAccessMode: DebuggerStorageAccessMode;
  storageAccessMode: DebuggerStorageAccessMode;
  allowNetwork: boolean;
  allowFileSystem: boolean;
  allowHookExecute: boolean;
  allowHookBlock: boolean;
  allowPayloadPatch: boolean;
  allowFrontendExtensions: boolean;
  allowExecuteAPI: boolean;
  requestedPermissions: string[];
  grantedPermissions: string[];
  executeActionStorage: Record<string, DebuggerStorageAccessMode>;
  defaultTimeoutMs: number;
  maxConcurrency: number;
  maxMemoryMB: number;
  fsMaxFiles: number;
  fsMaxTotalBytes: number;
  fsMaxReadBytes: number;
  storageMaxKeys: number;
  storageMaxTotalBytes: number;
  storageMaxValueBytes: number;
}

export interface DebuggerPageContextSummary {
  area: string;
  path: string;
  full_path: string;
  query_string: string;
  query_params: Record<string, string>;
  route_params: Record<string, string>;
}

export interface DebuggerSecretSummary {
  present: boolean;
  enabled: boolean;
  key_count: number;
  keys: string[];
  sample_key: string;
  sample_present: boolean;
  interpretation: string;
  error?: string;
}

export interface DebuggerWebhookSummary {
  present: boolean;
  enabled: boolean;
  key: string;
  method: string;
  path: string;
  query_string: string;
  query_params: Record<string, string>;
  headers: Record<string, string>;
  header_count: number;
  content_type: string;
  remote_addr: string;
  body_text_preview: string;
  body_json_preview?: unknown;
  interpretation: string;
}

export interface DebuggerActionTrace {
  id: string;
  ts: string;
  action: string;
  category: string;
  ok: boolean;
  message: string;
  error?: string;
  current_action?: string;
  declared_storage_access_mode?: DebuggerStorageAccessMode;
  observed_storage_access_mode?: DebuggerStorageAccessMode;
  user_id?: number;
  order_id?: number;
  session_id?: string;
  request_summary?: string;
  response_summary?: string;
  request_json?: string;
  response_json?: string;
}

export interface DebuggerProfile {
  config: DebuggerConfig;
  enabled_hooks: string[];
  source: string;
  recent_events: DebuggerEvent[];
  action_traces: DebuggerActionTrace[];
  sandbox: DebuggerSandboxProfile;
  storage: DebuggerStorageSummary;
  fs: DebuggerFileSummary;
  runtime: DebuggerRuntimeProbe;
  page_context: DebuggerPageContextSummary;
  secret: DebuggerSecretSummary;
  webhook: DebuggerWebhookSummary;
  capability_gaps: string[];
  warnings: string[];
}

export interface HookExecutionResponse {
  success: boolean;
  blocked?: boolean;
  block_reason?: string;
  payload?: GenericRecord;
  frontend_extensions?: FrontendExtension[];
  skipped?: boolean;
  reason?: string;
  hook?: string;
}

export interface SimulatedHookState {
  simulate_hook: string;
  simulate_area: "admin" | "user";
  simulate_slot: string;
  simulate_path: string;
  simulate_payload: string;
}

export interface StorageFormState {
  storage_key: string;
  storage_value: string;
}

export interface FileSystemFormState {
  fs_path: string;
  fs_content: string;
  fs_format: "text" | "json";
}

export interface NetworkFormState {
  network_method: string;
  network_url: string;
  network_headers: string;
  network_body: string;
  network_body_format: NetworkBodyFormat;
  network_timeout_ms: number;
}

export interface WorkerFormState {
  worker_script: string;
  worker_request_value: number;
  worker_second_value: number;
  worker_message_value: number;
}

export interface HostFormState {
  host_mode: HostActionMode;
  host_action: string;
  host_payload: string;
}

export interface DebuggerNetworkResponse extends PluginHTTPResponse {
  headers: Record<string, string>;
}

export interface FrontendExtension {
  type: string;
  slot?: string;
  title?: string;
  content?: string;
  priority?: number;
  data?: GenericRecord;
}

export interface PluginPageBlock {
  type: string;
  title?: string;
  content?: string;
  data?: GenericRecord;
}

export type GenericRecord = Record<string, unknown>;

export type ActionPayload = {
  source?: string;
  message?: string;
  notice?: string;
  values?: GenericRecord;
  blocks?: PluginPageBlock[];
};

export type RuntimeSandboxInput = RuntimeSandboxProfile | GenericRecord | undefined | null;
export type RuntimeStreamWriter = PluginExecuteStreamWriter;
