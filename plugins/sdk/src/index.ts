import type { PluginFrontendExtension, PluginPageBlock } from "./frontend";
import {
  OFFICIAL_FRONTEND_SLOTS,
  OFFICIAL_PLUGIN_PERMISSION_KEYS,
  OFFICIAL_HOST_PERMISSION_KEYS,
  OFFICIAL_PLUGIN_HOOK_GROUPS,
  OFFICIAL_PLUGIN_HOOKS
} from "./generated-catalog";

export type JsonValue =
  | string
  | number
  | boolean
  | null
  | JsonValue[]
  | { [key: string]: JsonValue };

export type StringMap = Record<string, string>;
export type UnknownMap = Record<string, unknown>;
export type PluginStorageAccessMode = "unknown" | "none" | "read" | "write";
export type PluginExecuteActionStorageMap = Record<string, PluginStorageAccessMode>;
export type OfficialPluginHook = (typeof OFFICIAL_PLUGIN_HOOKS)[number];
export type OfficialPluginHookGroup = keyof typeof OFFICIAL_PLUGIN_HOOK_GROUPS;
export type OfficialFrontendSlot = (typeof OFFICIAL_FRONTEND_SLOTS)[number];
export type OfficialPluginPermissionKey = (typeof OFFICIAL_PLUGIN_PERMISSION_KEYS)[number];
export type OfficialHostPermissionKey = (typeof OFFICIAL_HOST_PERMISSION_KEYS)[number];

export interface PluginManifestPermissionEntry {
  key?: string;
  required?: boolean;
  reason?: string;
  title?: string;
  description?: string;
  default_granted?: boolean;
}

export interface PluginManifestCatalogValidation {
  valid: boolean;
  hooks: string[];
  invalid_hooks: string[];
  disabled_hooks: string[];
  invalid_disabled_hooks: string[];
  allowed_frontend_slots: string[];
  invalid_allowed_frontend_slots: string[];
  requested_permissions: string[];
  invalid_requested_permissions: string[];
  granted_permissions: string[];
  invalid_granted_permissions: string[];
  declared_permissions: string[];
  invalid_declared_permissions: string[];
  requested_permissions_missing_declaration: string[];
  granted_permissions_missing_declaration: string[];
  declared_permissions_missing_request: string[];
}

export interface PluginManifestValidationIssue {
  path: string;
  message: string;
}

export interface PluginManifestCompatibilityInspection {
  manifest_present: boolean;
  runtime: string;
  host_manifest_version: string;
  manifest_version: string;
  host_protocol_version: string;
  protocol_version: string;
  min_host_protocol_version: string;
  max_host_protocol_version: string;
  compatible: boolean;
  legacy_defaults_applied: boolean;
  reason_code: string;
  reason: string;
}

export interface PluginManifestSchemaValidation {
  valid: boolean;
  issues: PluginManifestValidationIssue[];
  compatibility: PluginManifestCompatibilityInspection;
}

export {
  OFFICIAL_FRONTEND_SLOTS,
  OFFICIAL_PLUGIN_PERMISSION_KEYS,
  OFFICIAL_HOST_PERMISSION_KEYS,
  OFFICIAL_PLUGIN_HOOK_GROUPS,
  OFFICIAL_PLUGIN_HOOKS
};

const PLUGIN_HOST_MANIFEST_VERSION = "1.0.0";
const DEFAULT_PLUGIN_HOST_PROTOCOL_VERSION = "1.0.0";

export interface PluginExecutionContext {
  user_id?: number;
  order_id?: number;
  session_id?: string;
  metadata?: StringMap;
}

export interface PluginPageContext {
  area: string;
  path: string;
  full_path: string;
  query_string: string;
  query_params: StringMap;
  route_params: StringMap;
}

export interface PluginStorageAPI {
  get(key: string): string | undefined;
  set(key: string, value: string): boolean;
  delete(key: string): boolean;
  list(): string[];
  clear(): boolean;
}

export interface PluginSecretAPI {
  enabled: boolean;
  get(key: string): string | undefined;
  has(key: string): boolean;
  list(): string[];
}

export interface PluginWebhookAPI {
  enabled: boolean;
  key: string;
  method: string;
  path: string;
  queryString: string;
  queryParams: StringMap;
  headers: StringMap;
  contentType: string;
  remoteAddr: string;
  bodyText: string;
  bodyBase64: string;
  header(name: string): string | undefined;
  query(name: string): string | undefined;
  text(): string;
  json<T = unknown>(): T | undefined;
}

export interface PluginFSListEntry {
  name: string;
  path: string;
  is_dir: boolean;
  size: number;
  mod_time: string;
}

export interface PluginFSStat {
  exists: boolean;
  path: string;
  name?: string;
  is_dir?: boolean;
  size?: number;
  mod_time?: string;
  layer?: "code" | "data";
}

export interface PluginFSUsage {
  file_count: number;
  total_bytes: number;
  max_files: number;
  max_bytes: number;
}

export interface PluginWorkspaceEntry {
  timestamp?: string;
  channel?: string;
  level?: string;
  message?: string;
  source?: string;
  metadata?: StringMap;
}

export interface PluginWorkspaceSnapshot {
  enabled: boolean;
  max_entries: number;
  entry_count: number;
  entries: PluginWorkspaceEntry[];
}

export interface PluginWorkspaceReadOptions {
  echo?: boolean;
  masked?: boolean;
}

export interface PluginWorkspaceCommandContext {
  name: string;
  entry: string;
  raw: string;
  argv: string[];
  command_id?: string;
  interactive: boolean;
}

export interface PluginWorkspaceAPI {
  enabled: boolean;
  commandName?: string;
  commandId?: string;
  write(message: string, metadata?: UnknownMap): void;
  writeln(message: string, metadata?: UnknownMap): void;
  info(message: string, metadata?: UnknownMap): void;
  warn(message: string, metadata?: UnknownMap): void;
  error(message: string, metadata?: UnknownMap): void;
  clear(): boolean;
  tail(limit?: number): PluginWorkspaceEntry[];
  snapshot(limit?: number): PluginWorkspaceSnapshot;
  read(options?: PluginWorkspaceReadOptions): string;
  readLine(prompt?: string, options?: PluginWorkspaceReadOptions): string;
}

export interface PluginOrderGetParams {
  id?: number;
  order_id?: number;
  order_no?: string;
}

export interface PluginOrderAssignTrackingParams {
  id?: number;
  order_id?: number;
  order_no?: string;
  tracking_no?: string;
  trackingNo?: string;
}

export interface PluginOrderRequestResubmitParams {
  id?: number;
  order_id?: number;
  order_no?: string;
  reason?: string;
}

export interface PluginOrderMarkPaidParams {
  id?: number;
  order_id?: number;
  order_no?: string;
  admin_remark?: string;
  adminRemark?: string;
  skip_auto_delivery?: boolean;
  skipAutoDelivery?: boolean;
}

export interface PluginOrderUpdatePriceParams {
  id?: number;
  order_id?: number;
  order_no?: string;
  total_amount_minor?: number;
  totalAmountMinor?: number;
}

export interface PluginOrderListParams extends UnknownMap {
  page?: number;
  page_size?: number;
  pageSize?: number;
  limit?: number;
  status?: string;
  search?: string;
  q?: string;
  country?: string;
  product_search?: string;
  productSearch?: string;
  promo_code?: string;
  promoCode?: string;
  promo_code_id?: number;
  promoCodeId?: number;
  user_id?: number;
  userId?: number;
}

export interface PluginUserGetParams {
  id?: number;
  user_id?: number;
  email?: string;
  uuid?: string;
}

export interface PluginUserListParams extends UnknownMap {
  page?: number;
  page_size?: number;
  pageSize?: number;
  limit?: number;
  search?: string;
  q?: string;
  role?: string;
  locale?: string;
  country?: string;
  is_active?: boolean;
  isActive?: boolean;
  email_verified?: boolean;
  emailVerified?: boolean;
  email_notify_marketing?: boolean;
  emailNotifyMarketing?: boolean;
  sms_notify_marketing?: boolean;
  smsNotifyMarketing?: boolean;
  has_phone?: boolean;
  hasPhone?: boolean;
}

export interface PluginProductGetParams {
  id?: number;
  product_id?: number;
  sku?: string;
}

export interface PluginProductListParams extends UnknownMap {
  page?: number;
  page_size?: number;
  pageSize?: number;
  limit?: number;
  status?: string;
  category?: string;
  search?: string;
  q?: string;
  is_featured?: boolean;
  isFeatured?: boolean;
  is_recommended?: boolean;
  isRecommended?: boolean;
  is_active?: boolean;
  isActive?: boolean;
}

export interface PluginInventoryGetParams {
  id?: number;
  inventory_id?: number;
  sku?: string;
}

export interface PluginInventoryListParams extends UnknownMap {
  page?: number;
  page_size?: number;
  pageSize?: number;
  limit?: number;
  is_active?: boolean;
  isActive?: boolean;
  low_stock?: boolean;
  lowStock?: boolean;
}

export interface PluginInventoryBindingGetParams {
  id?: number;
  binding_id?: number;
  inventory_binding_id?: number;
  inventoryBindingId?: number;
}

export interface PluginInventoryBindingListParams extends UnknownMap {
  page?: number;
  page_size?: number;
  pageSize?: number;
  limit?: number;
  product_id?: number;
  productId?: number;
  inventory_id?: number;
  inventoryId?: number;
  is_random?: boolean;
  isRandom?: boolean;
}

export interface PluginPromoGetParams {
  id?: number;
  promo_id?: number;
  promo_code_id?: number;
  code?: string;
  promo_code?: string;
  promoCode?: string;
}

export interface PluginPromoListParams extends UnknownMap {
  page?: number;
  page_size?: number;
  pageSize?: number;
  limit?: number;
  status?: string;
  search?: string;
  q?: string;
}

export interface PluginTicketGetParams {
  id?: number;
  ticket_id?: number;
  ticket_no?: string;
}

export interface PluginTicketReplyParams {
  id?: number;
  ticket_id?: number;
  ticket_no?: string;
  content?: string;
  content_type?: string;
  contentType?: string;
}

export interface PluginTicketUpdateParams {
  id?: number;
  ticket_id?: number;
  ticket_no?: string;
  status?: string;
  priority?: string;
  assigned_to?: number;
  assignedTo?: number;
  clear_assignee?: boolean;
  clearAssignee?: boolean;
  unassigned?: boolean;
}

export interface PluginTicketListParams extends UnknownMap {
  page?: number;
  page_size?: number;
  pageSize?: number;
  limit?: number;
  status?: string;
  exclude_status?: string;
  excludeStatus?: string;
  search?: string;
  q?: string;
  assigned_to?: number | string;
  assignedTo?: number | string;
  assigned_to_me?: boolean;
  assignedToMe?: boolean;
  unassigned?: boolean;
}

export interface PluginSerialGetParams {
  id?: number;
  serial_id?: number;
  serial_number?: string;
  serialNumber?: string;
}

export interface PluginSerialListParams extends UnknownMap {
  page?: number;
  page_size?: number;
  pageSize?: number;
  limit?: number;
  product_id?: number;
  productId?: number;
  order_id?: number;
  orderId?: number;
  product_code?: string;
  productCode?: string;
  serial_number?: string;
  serialNumber?: string;
}

export interface PluginAnnouncementGetParams {
  id?: number;
  announcement_id?: number;
}

export interface PluginAnnouncementListParams extends UnknownMap {
  page?: number;
  page_size?: number;
  pageSize?: number;
  limit?: number;
  search?: string;
  q?: string;
  category?: string;
  is_mandatory?: boolean;
  isMandatory?: boolean;
}

export interface PluginKnowledgeGetParams {
  id?: number;
  article_id?: number;
}

export interface PluginKnowledgeListParams extends UnknownMap {
  page?: number;
  page_size?: number;
  pageSize?: number;
  limit?: number;
  search?: string;
  q?: string;
  category_id?: number;
  categoryId?: number;
}

export interface PluginPaymentMethodGetParams {
  id?: number;
  payment_method_id?: number;
  paymentMethodId?: number;
}

export interface PluginPaymentMethodListParams extends UnknownMap {
  page?: number;
  page_size?: number;
  pageSize?: number;
  limit?: number;
  search?: string;
  q?: string;
  type?: string;
  enabled_only?: boolean;
  enabledOnly?: boolean;
}

export interface PluginVirtualInventoryGetParams {
  id?: number;
  virtual_inventory_id?: number;
  virtualInventoryId?: number;
  sku?: string;
}

export interface PluginVirtualInventoryListParams extends UnknownMap {
  page?: number;
  page_size?: number;
  pageSize?: number;
  limit?: number;
  search?: string;
  q?: string;
  type?: string;
  is_active?: boolean;
  isActive?: boolean;
}

export interface PluginVirtualInventoryBindingGetParams {
  id?: number;
  binding_id?: number;
  virtual_binding_id?: number;
  virtualBindingId?: number;
}

export interface PluginVirtualInventoryBindingListParams extends UnknownMap {
  page?: number;
  page_size?: number;
  pageSize?: number;
  limit?: number;
  product_id?: number;
  productId?: number;
  virtual_inventory_id?: number;
  virtualInventoryId?: number;
  is_random?: boolean;
  isRandom?: boolean;
}

export interface PluginMarketSourceGetParams extends UnknownMap {
  source_id?: string;
  sourceId?: string;
}

export interface PluginMarketCatalogListParams extends UnknownMap {
  source_id?: string;
  sourceId?: string;
  kind?: string;
  channel?: string;
  q?: string;
  search?: string;
  offset?: number;
  limit?: number;
  page_size?: number;
  pageSize?: number;
  host_version?: string;
  hostVersion?: string;
  host_protocol_version?: string;
  hostProtocolVersion?: string;
  host_bridge_version?: string;
  hostBridgeVersion?: string;
  runtime?: string;
}

export interface PluginMarketArtifactGetParams extends UnknownMap {
  source_id?: string;
  sourceId?: string;
  kind: string;
  name: string;
}

export interface PluginMarketReleaseGetParams extends PluginMarketArtifactGetParams {
  version: string;
}

export interface PluginMarketInstallPreviewParams extends PluginMarketReleaseGetParams {}

export interface PluginMarketInstallExecuteParams extends PluginMarketInstallPreviewParams {
  activate?: boolean;
  auto_start?: boolean;
  autoStart?: boolean;
  granted_permissions?: string[];
  grantedPermissions?: string[];
  note?: string;
  detail?: string;
  options?: UnknownMap;
}

export interface PluginMarketInstallTaskGetParams extends UnknownMap {
  task_id?: string;
  taskId?: string;
  id?: number;
  deployment_id?: number;
  deploymentId?: number;
}

export interface PluginMarketInstallTaskListParams extends UnknownMap {
  source_id?: string;
  sourceId?: string;
  kind?: string;
  name?: string;
  version?: string;
  status?: string;
  offset?: number;
  limit?: number;
  page_size?: number;
  pageSize?: number;
}

export interface PluginMarketInstallHistoryListParams extends UnknownMap {
  source_id?: string;
  sourceId?: string;
  kind?: string;
  name?: string;
  version?: string;
  offset?: number;
  limit?: number;
  page_size?: number;
  pageSize?: number;
}

export interface PluginMarketInstallRollbackParams extends PluginMarketReleaseGetParams {
  auto_start?: boolean;
  autoStart?: boolean;
  note?: string;
  detail?: string;
  options?: UnknownMap;
}

export interface PluginEmailTemplateListParams extends UnknownMap {
  event?: string;
}

export interface PluginEmailTemplateGetParams {
  key?: string;
  filename?: string;
  name?: string;
}

export interface PluginEmailTemplateSaveParams extends PluginEmailTemplateGetParams {
  content: string;
  expected_digest?: string;
  expectedDigest?: string;
  expected_updated_at?: string;
  expectedUpdatedAt?: string;
}

export interface PluginLandingPageGetParams extends UnknownMap {
  slug?: string;
  page_key?: string;
  pageKey?: string;
}

export interface PluginLandingPageSaveParams extends PluginLandingPageGetParams {
  html_content?: string;
  htmlContent?: string;
  content?: string;
  expected_digest?: string;
  expectedDigest?: string;
  expected_updated_at?: string;
  expectedUpdatedAt?: string;
}

export interface PluginInvoiceTemplateGetParams extends UnknownMap {
  target_key?: string;
  targetKey?: string;
  key?: string;
}

export interface PluginInvoiceTemplateSaveParams extends PluginInvoiceTemplateGetParams {
  content: string;
  expected_digest?: string;
  expectedDigest?: string;
  expected_updated_at?: string;
  expectedUpdatedAt?: string;
}

export interface PluginAuthBrandingGetParams extends UnknownMap {
  target_key?: string;
  targetKey?: string;
}

export interface PluginAuthBrandingSaveParams extends PluginAuthBrandingGetParams {
  content: string;
  expected_digest?: string;
  expectedDigest?: string;
  expected_updated_at?: string;
  expectedUpdatedAt?: string;
}

export interface PluginPageRulePackGetParams extends UnknownMap {
  target_key?: string;
  targetKey?: string;
  key?: string;
}

export interface PluginPageRulePackSaveParams extends PluginPageRulePackGetParams {
  content?: string;
  json_content?: string;
  jsonContent?: string;
  rules?: unknown[];
  page_rules?: unknown[];
  pageRules?: unknown[];
  expected_digest?: string;
  expectedDigest?: string;
  expected_updated_at?: string;
  expectedUpdatedAt?: string;
}

export interface PluginHostListResult<T = UnknownMap> extends UnknownMap {
  items: T[];
  page: number;
  page_size: number;
  total: number;
  has_more: boolean;
}

export interface PluginOrderAPI {
  get<T = UnknownMap>(query: number | PluginOrderGetParams): T;
  list<T = PluginHostListResult>(query?: PluginOrderListParams): T;
  assignTracking<T = UnknownMap>(payload: PluginOrderAssignTrackingParams): T;
  requestResubmit<T = UnknownMap>(payload: PluginOrderRequestResubmitParams): T;
  markPaid<T = UnknownMap>(payload?: PluginOrderMarkPaidParams): T;
  updatePrice<T = UnknownMap>(payload: PluginOrderUpdatePriceParams): T;
}

export interface PluginUserAPI {
  get<T = UnknownMap>(query: number | PluginUserGetParams): T;
  list<T = PluginHostListResult>(query?: PluginUserListParams): T;
}

export interface PluginProductAPI {
  get<T = UnknownMap>(query: number | PluginProductGetParams): T;
  list<T = PluginHostListResult>(query?: PluginProductListParams): T;
}

export interface PluginInventoryAPI {
  get<T = UnknownMap>(query: number | PluginInventoryGetParams): T;
  list<T = PluginHostListResult>(query?: PluginInventoryListParams): T;
}

export interface PluginInventoryBindingAPI {
  get<T = UnknownMap>(query: number | PluginInventoryBindingGetParams): T;
  list<T = PluginHostListResult>(query?: PluginInventoryBindingListParams): T;
}

export interface PluginPromoAPI {
  get<T = UnknownMap>(query: number | PluginPromoGetParams): T;
  list<T = PluginHostListResult>(query?: PluginPromoListParams): T;
}

export interface PluginTicketAPI {
  get<T = UnknownMap>(query: number | PluginTicketGetParams): T;
  list<T = PluginHostListResult>(query?: PluginTicketListParams): T;
  reply<T = UnknownMap>(payload: PluginTicketReplyParams): T;
  update<T = UnknownMap>(payload: PluginTicketUpdateParams): T;
}

export interface PluginSerialAPI {
  get<T = UnknownMap>(query: number | PluginSerialGetParams): T;
  list<T = PluginHostListResult>(query?: PluginSerialListParams): T;
}

export interface PluginAnnouncementAPI {
  get<T = UnknownMap>(query: number | PluginAnnouncementGetParams): T;
  list<T = PluginHostListResult>(query?: PluginAnnouncementListParams): T;
}

export interface PluginKnowledgeAPI {
  get<T = UnknownMap>(query: number | PluginKnowledgeGetParams): T;
  list<T = PluginHostListResult>(query?: PluginKnowledgeListParams): T;
  categories<T = UnknownMap>(query?: UnknownMap): T;
}

export interface PluginPaymentMethodAPI {
  get<T = UnknownMap>(query: number | PluginPaymentMethodGetParams): T;
  list<T = PluginHostListResult>(query?: PluginPaymentMethodListParams): T;
}

export interface PluginVirtualInventoryAPI {
  get<T = UnknownMap>(query: number | PluginVirtualInventoryGetParams): T;
  list<T = PluginHostListResult>(query?: PluginVirtualInventoryListParams): T;
}

export interface PluginVirtualInventoryBindingAPI {
  get<T = UnknownMap>(query: number | PluginVirtualInventoryBindingGetParams): T;
  list<T = PluginHostListResult>(query?: PluginVirtualInventoryBindingListParams): T;
}

export interface PluginMarketSourceAPI {
  list<T = UnknownMap>(query?: UnknownMap): T;
  get<T = UnknownMap>(query?: PluginMarketSourceGetParams): T;
}

export interface PluginMarketCatalogAPI {
  list<T = UnknownMap>(query?: PluginMarketCatalogListParams): T;
}

export interface PluginMarketArtifactAPI {
  get<T = UnknownMap>(query: PluginMarketArtifactGetParams): T;
}

export interface PluginMarketReleaseAPI {
  get<T = UnknownMap>(query: PluginMarketReleaseGetParams): T;
}

export interface PluginMarketInstallAPI {
  preview<T = UnknownMap>(payload: PluginMarketInstallPreviewParams): T;
  execute<T = UnknownMap>(payload: PluginMarketInstallExecuteParams): T;
  task?: PluginMarketInstallTaskAPI;
  history?: PluginMarketInstallHistoryAPI;
  rollback<T = UnknownMap>(payload: PluginMarketInstallRollbackParams): T;
}

export interface PluginMarketInstallTaskAPI {
  get<T = UnknownMap>(query: PluginMarketInstallTaskGetParams): T;
  list<T = UnknownMap>(query?: PluginMarketInstallTaskListParams): T;
}

export interface PluginMarketInstallHistoryAPI {
  list<T = UnknownMap>(query?: PluginMarketInstallHistoryListParams): T;
}

export interface PluginMarketAPI {
  source?: PluginMarketSourceAPI;
  catalog?: PluginMarketCatalogAPI;
  artifact?: PluginMarketArtifactAPI;
  release?: PluginMarketReleaseAPI;
  install?: PluginMarketInstallAPI;
}

export interface PluginEmailTemplateAPI {
  list<T = UnknownMap>(query?: PluginEmailTemplateListParams): T;
  get<T = UnknownMap>(query: string | PluginEmailTemplateGetParams): T;
  save<T = UnknownMap>(payload: PluginEmailTemplateSaveParams): T;
}

export interface PluginLandingPageAPI {
  get<T = UnknownMap>(query?: PluginLandingPageGetParams): T;
  save<T = UnknownMap>(payload: PluginLandingPageSaveParams): T;
  reset<T = UnknownMap>(payload?: PluginLandingPageGetParams): T;
}

export interface PluginInvoiceTemplateAPI {
  get<T = UnknownMap>(query?: PluginInvoiceTemplateGetParams): T;
  save<T = UnknownMap>(payload: PluginInvoiceTemplateSaveParams): T;
  reset<T = UnknownMap>(payload?: PluginInvoiceTemplateGetParams): T;
}

export interface PluginAuthBrandingAPI {
  get<T = UnknownMap>(query?: PluginAuthBrandingGetParams): T;
  save<T = UnknownMap>(payload: PluginAuthBrandingSaveParams): T;
  reset<T = UnknownMap>(payload?: PluginAuthBrandingGetParams): T;
}

export interface PluginPageRulePackAPI {
  get<T = UnknownMap>(query?: PluginPageRulePackGetParams): T;
  save<T = UnknownMap>(payload: PluginPageRulePackSaveParams): T;
  reset<T = UnknownMap>(payload?: PluginPageRulePackGetParams): T;
}

export interface PluginHostAPI {
  enabled: boolean;
  invoke<T = UnknownMap>(action: string, params?: UnknownMap): T;
  order?: PluginOrderAPI;
  user?: PluginUserAPI;
  product?: PluginProductAPI;
  inventory?: PluginInventoryAPI;
  inventoryBinding?: PluginInventoryBindingAPI;
  promo?: PluginPromoAPI;
  ticket?: PluginTicketAPI;
  serial?: PluginSerialAPI;
  announcement?: PluginAnnouncementAPI;
  knowledge?: PluginKnowledgeAPI;
  paymentMethod?: PluginPaymentMethodAPI;
  virtualInventory?: PluginVirtualInventoryAPI;
  virtualInventoryBinding?: PluginVirtualInventoryBindingAPI;
  market?: PluginMarketAPI;
  emailTemplate?: PluginEmailTemplateAPI;
  landingPage?: PluginLandingPageAPI;
  invoiceTemplate?: PluginInvoiceTemplateAPI;
  authBranding?: PluginAuthBrandingAPI;
  pageRulePack?: PluginPageRulePackAPI;
}

export type PluginHTTPHeaders = Record<string, string>;

export interface PluginHTTPRequestOptions {
  url: string;
  method?: string;
  headers?: PluginHTTPHeaders;
  body?: unknown;
  timeout_ms?: number;
}

export interface PluginHTTPResponse<T = unknown> {
  ok: boolean;
  url: string;
  status: number;
  statusText: string;
  headers: PluginHTTPHeaders;
  body: string;
  data?: T;
  error?: string;
  duration_ms: number;
  redirected?: boolean;
}

export interface PluginHTTPAPI {
  enabled: boolean;
  defaultTimeoutMs: number;
  maxResponseBytes: number;
  get(url: string, headers?: PluginHTTPHeaders): PluginHTTPResponse;
  post(url: string, body?: unknown, headers?: PluginHTTPHeaders): PluginHTTPResponse;
  request(options: PluginHTTPRequestOptions): PluginHTTPResponse;
}

export interface PluginFSAPI {
  enabled: boolean;
  root: string;
  codeRoot: string;
  dataRoot: string;
  pluginID: number;
  pluginName: string;
  maxFiles: number;
  maxTotalBytes: number;
  maxReadBytes: number;
  exists(path: string): boolean;
  readText(path: string): string;
  readBase64(path: string): string;
  readJSON<T = unknown>(path: string): T;
  writeText(path: string, content: string): void;
  writeJSON(path: string, value: unknown): void;
  writeBase64(path: string, payload: string): void;
  delete(path: string): boolean;
  mkdir(path: string): void;
  list(path?: string): PluginFSListEntry[];
  stat(path: string): PluginFSStat;
  usage(): PluginFSUsage;
  recalculateUsage(): PluginFSUsage;
}

export interface PluginRuntimeGlobal {
  workspace?: PluginWorkspaceAPI;
  storage?: PluginStorageAPI;
  secret?: PluginSecretAPI;
  webhook?: PluginWebhookAPI;
  http?: PluginHTTPAPI;
  fs?: PluginFSAPI;
  host?: PluginHostAPI;
  order?: PluginOrderAPI;
  user?: PluginUserAPI;
  product?: PluginProductAPI;
  inventory?: PluginInventoryAPI;
  inventoryBinding?: PluginInventoryBindingAPI;
  promo?: PluginPromoAPI;
  ticket?: PluginTicketAPI;
  serial?: PluginSerialAPI;
  announcement?: PluginAnnouncementAPI;
  knowledge?: PluginKnowledgeAPI;
  paymentMethod?: PluginPaymentMethodAPI;
  virtualInventory?: PluginVirtualInventoryAPI;
  virtualInventoryBinding?: PluginVirtualInventoryBindingAPI;
  market?: PluginMarketAPI;
  emailTemplate?: PluginEmailTemplateAPI;
  landingPage?: PluginLandingPageAPI;
  invoiceTemplate?: PluginInvoiceTemplateAPI;
  authBranding?: PluginAuthBrandingAPI;
  pageRulePack?: PluginPageRulePackAPI;
}

export interface PluginSandboxProfile {
  level?: string;
  currentAction?: string;
  declaredStorageAccessMode?: PluginStorageAccessMode;
  storageAccessMode?: PluginStorageAccessMode;
  allowNetwork?: boolean;
  allowFileSystem?: boolean;
  allowHookExecute?: boolean;
  allowHookBlock?: boolean;
  allowPayloadPatch?: boolean;
  allowFrontendExtensions?: boolean;
  allowExecuteAPI?: boolean;
  requestedPermissions?: string[];
  grantedPermissions?: string[];
  executeActionStorage?: PluginExecuteActionStorageMap;
  defaultTimeoutMs?: number;
  maxConcurrency?: number;
  maxMemoryMB?: number;
  fsMaxFiles?: number;
  fsMaxTotalBytes?: number;
  fsMaxReadBytes?: number;
  storageMaxKeys?: number;
  storageMaxTotalBytes?: number;
  storageMaxValueBytes?: number;
}

declare global {
  var Plugin: PluginRuntimeGlobal | undefined;
  var sandbox: PluginSandboxProfile | undefined;
}

export interface PluginHealthResult {
  healthy: boolean;
  version?: string;
  metadata?: UnknownMap;
}

export interface PluginActionData {
  source?: string;
  message?: string;
  notice?: string;
  values?: UnknownMap;
  blocks?: PluginPageBlock[];
}

export interface PluginExecuteStreamWriter {
  write(data?: unknown, metadata?: StringMap | UnknownMap): void;
  emit(data?: unknown, metadata?: StringMap | UnknownMap): void;
  progress(status: string, progress?: number, metadata?: StringMap | UnknownMap): void;
}

export interface PluginExecuteSuccessResult {
  success: true;
  message?: string;
  data?: PluginActionData | UnknownMap;
  payload?: UnknownMap;
  frontend_extensions?: PluginFrontendExtension[];
  blocked?: boolean;
  block_reason?: string;
  skipped?: boolean;
  reason?: string;
  hook?: string;
}

export interface PluginExecuteErrorResult {
  success: false;
  error: string;
  data?: PluginActionData | UnknownMap;
}

export type PluginExecuteResult = PluginExecuteSuccessResult | PluginExecuteErrorResult;

export interface PluginDefinition {
  execute(
    action: unknown,
    params: unknown,
    context: PluginExecutionContext,
    config: UnknownMap,
    sandbox: PluginSandboxProfile
  ): PluginExecuteResult | UnknownMap;
  executeStream?(
    action: unknown,
    params: unknown,
    context: PluginExecutionContext,
    config: UnknownMap,
    sandbox: PluginSandboxProfile,
    stream: PluginExecuteStreamWriter
  ): PluginExecuteResult | UnknownMap;
  health?(config: UnknownMap, sandbox: PluginSandboxProfile): PluginHealthResult;
  workspace?: PluginWorkspaceHandlerMap;
}

export type PluginWorkspaceHandler = (
  command: PluginWorkspaceCommandContext,
  context: PluginExecutionContext,
  config: UnknownMap,
  sandbox: PluginSandboxProfile,
  workspace: PluginWorkspaceAPI
) => PluginExecuteResult | UnknownMap;

export interface PluginWorkspaceCommandCatalogEntry {
  name: string;
  entry: string;
  title?: string;
  description?: string;
  interactive: boolean;
  permissions?: string[];
}

export interface PluginWorkspaceCommandDefinition extends Partial<PluginWorkspaceCommandCatalogEntry> {
  handler: PluginWorkspaceHandler;
}

export type PluginWorkspaceCommandOptions = Omit<PluginWorkspaceCommandDefinition, "handler">;

export type PluginWorkspaceHandlerDefinition =
  | PluginWorkspaceHandler
  | PluginWorkspaceCommandDefinition;

export interface PluginWorkspaceHandlerMap {
  [entry: string]: PluginWorkspaceHandlerDefinition;
}

export function defineWorkspaceCommand(
  handler: PluginWorkspaceHandler
): PluginWorkspaceCommandDefinition;
export function defineWorkspaceCommand(
  options: PluginWorkspaceCommandOptions,
  handler: PluginWorkspaceHandler
): PluginWorkspaceCommandDefinition;
export function defineWorkspaceCommand(
  optionsOrHandler: PluginWorkspaceCommandOptions | PluginWorkspaceHandler,
  maybeHandler?: PluginWorkspaceHandler
): PluginWorkspaceCommandDefinition {
  if (typeof optionsOrHandler === "function") {
    return {
      handler: optionsOrHandler
    };
  }
  if (typeof maybeHandler !== "function") {
    throw new Error("defineWorkspaceCommand requires a handler function");
  }
  return {
    ...optionsOrHandler,
    handler: maybeHandler
  };
}

export function defineWorkspaceCommands<T extends PluginWorkspaceHandlerMap>(workspace: T): T {
  return workspace;
}

function isWorkspaceCommandAction(action: unknown): boolean {
  return asString(action).toLowerCase() === "workspace.command.execute";
}

function parseWorkspaceCommandContext(params: unknown): PluginWorkspaceCommandContext | null {
  const record = asRecord(params);
  const entry = asString(record.workspace_command_entry);
  const name = asString(record.workspace_command_name);
  if (!entry || !name) {
    return null;
  }
  const argvRaw = record.workspace_command_argv_json;
  const argvDecoded =
    typeof argvRaw === "string" && argvRaw.trim() !== ""
      ? JSON.parse(argvRaw)
      : Array.isArray(record.workspace_command_argv)
        ? record.workspace_command_argv
        : [];
  const argv = Array.isArray(argvDecoded)
    ? argvDecoded
        .map((item) => String(item ?? "").trim())
        .filter((item) => item !== "")
    : [];
  return {
    name,
    entry,
    raw: asString(record.workspace_command_raw) || [name, ...argv].join(" ").trim(),
    argv,
    command_id: asString(record.workspace_command_id) || undefined,
    interactive: asBool(record.workspace_command_interactive, false)
  };
}

function tryExecuteWorkspaceHandler(
  definition: PluginDefinition,
  action: unknown,
  params: unknown,
  context: PluginExecutionContext,
  config: UnknownMap,
  sandbox: PluginSandboxProfile
): PluginExecuteResult | UnknownMap | undefined {
  if (!isWorkspaceCommandAction(action) || !definition.workspace) {
    return undefined;
  }
  let command: PluginWorkspaceCommandContext | null = null;
  try {
    command = parseWorkspaceCommandContext(params);
  } catch (error) {
    return errorResult(
      `invalid workspace command payload: ${String((error as Error)?.message || error)}`
    );
  }
  if (!command) {
    return undefined;
  }
  const handlerDefinition = definition.workspace[command.entry];
  const handler = resolveWorkspaceHandler(handlerDefinition);
  if (typeof handler !== "function") {
    return undefined;
  }
  const workspace = getPluginWorkspace();
  if (!workspace || !workspace.enabled) {
    return errorResult("Plugin.workspace is unavailable for current runtime.");
  }
  return handler(command, context, config, sandbox, workspace);
}

function resolveWorkspaceHandler(
  definition: PluginWorkspaceHandlerDefinition | undefined
): PluginWorkspaceHandler | undefined {
  if (typeof definition === "function") {
    return definition;
  }
  if (!definition || typeof definition !== "object") {
    return undefined;
  }
  return typeof definition.handler === "function" ? definition.handler : undefined;
}

export function definePlugin(definition: PluginDefinition): PluginDefinition {
  return {
    ...definition,
    execute(action, params, context, config, sandbox) {
      const workspaceResult = tryExecuteWorkspaceHandler(
        definition,
        action,
        params,
        context,
        config,
        sandbox
      );
      if (workspaceResult !== undefined) {
        return workspaceResult;
      }
      return definition.execute(action, params, context, config, sandbox);
    },
    executeStream(action, params, context, config, sandbox, stream) {
      const workspaceResult = tryExecuteWorkspaceHandler(
        definition,
        action,
        params,
        context,
        config,
        sandbox
      );
      if (workspaceResult !== undefined) {
        return workspaceResult;
      }
      if (typeof definition.executeStream === "function") {
        return definition.executeStream(action, params, context, config, sandbox, stream);
      }
      return definition.execute(action, params, context, config, sandbox);
    },
    health(config, sandbox) {
      const base =
        typeof definition.health === "function"
          ? definition.health(config, sandbox)
          : { healthy: true };
      return {
        ...(base || { healthy: true }),
        healthy: asBool(base?.healthy, true)
      };
    }
  };
}

export function successResult(
  data?: PluginActionData | UnknownMap,
  extra: Omit<PluginExecuteSuccessResult, "success" | "data"> = {}
): PluginExecuteSuccessResult {
  return data === undefined
    ? { success: true, ...extra }
    : { success: true, ...extra, data };
}

export function errorResult(
  error: string,
  data?: PluginActionData | UnknownMap
): PluginExecuteErrorResult {
  return data === undefined
    ? { success: false, error }
    : { success: false, error, data };
}

export function asString(value: unknown): string {
  if (value === null || value === undefined) {
    return "";
  }
  return String(value).trim();
}

export function asBool(value: unknown, fallback = false): boolean {
  if (typeof value === "boolean") {
    return value;
  }
  if (typeof value === "string") {
    const normalized = value.trim().toLowerCase();
    if (normalized === "true" || normalized === "1" || normalized === "yes" || normalized === "on") {
      return true;
    }
    if (normalized === "false" || normalized === "0" || normalized === "no" || normalized === "off") {
      return false;
    }
  }
  if (typeof value === "number") {
    return value !== 0;
  }
  return fallback;
}

export function asRecord(value: unknown): UnknownMap {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return {};
  }
  return value as UnknownMap;
}

export function asInteger(value: unknown, fallback = 0): number {
  if (typeof value === "number" && Number.isFinite(value)) {
    return Math.trunc(value);
  }
  if (typeof value === "string" && value.trim() !== "") {
    const parsed = Number.parseInt(value.trim(), 10);
    if (!Number.isNaN(parsed)) {
      return parsed;
    }
  }
  return fallback;
}

export function safeParseJSON(raw: unknown): UnknownMap {
  if (typeof raw !== "string" || raw.trim() === "") {
    return {};
  }
  try {
    const decoded = JSON.parse(raw);
    return asRecord(decoded);
  } catch {
    return {};
  }
}

export function safeParseStringMap(raw: unknown): StringMap {
  const record =
    typeof raw === "string"
      ? safeParseJSON(raw)
      : asRecord(raw);
  const output: StringMap = {};
  Object.entries(record).forEach(([key, value]) => {
    const normalizedKey = asString(key);
    if (!normalizedKey) {
      return;
    }
    output[normalizedKey] = value === undefined || value === null ? "" : String(value);
  });
  return output;
}

export function normalizeHookName(hook: unknown): string {
  return asString(hook).toLowerCase();
}

export function normalizePluginPermissionKey(permission: unknown): string {
  return asString(permission).toLowerCase();
}

function normalizeStringList(
  value: unknown,
  normalizer: (item: unknown) => string
): string[] {
  if (!Array.isArray(value)) {
    return [];
  }
  const output: string[] = [];
  const seen = new Set<string>();
  value.forEach((item) => {
    const normalized = normalizer(item);
    if (!normalized || seen.has(normalized)) {
      return;
    }
    seen.add(normalized);
    output.push(normalized);
  });
  return output;
}

function normalizeManifestPermissionEntries(value: unknown): string[] {
  if (!Array.isArray(value)) {
    return [];
  }
  return normalizeStringList(
    value.map((item) => asRecord(item).key),
    normalizePluginPermissionKey
  );
}

export function listOfficialPluginHooks(group?: OfficialPluginHookGroup): string[] {
  if (!group) {
    return [...OFFICIAL_PLUGIN_HOOKS];
  }
  const hooks = OFFICIAL_PLUGIN_HOOK_GROUPS[group] || [];
  return [...hooks];
}

export function listOfficialFrontendSlots(): string[] {
  return [...OFFICIAL_FRONTEND_SLOTS];
}

export function listOfficialHostPermissionKeys(): string[] {
  return [...OFFICIAL_HOST_PERMISSION_KEYS];
}

export function listOfficialPluginPermissionKeys(): string[] {
  return [...OFFICIAL_PLUGIN_PERMISSION_KEYS];
}

export function isOfficialPluginHook(hook: unknown): hook is OfficialPluginHook {
  const normalized = normalizeHookName(hook);
  return normalized !== "" && (OFFICIAL_PLUGIN_HOOKS as readonly string[]).includes(normalized);
}

export function isOfficialFrontendSlot(slot: unknown): slot is OfficialFrontendSlot {
  const normalized = asString(slot);
  return normalized !== "" && (OFFICIAL_FRONTEND_SLOTS as readonly string[]).includes(normalized);
}

export function isOfficialPluginPermissionKey(
  permission: unknown
): permission is OfficialPluginPermissionKey {
  const normalized = normalizePluginPermissionKey(permission);
  return (
    normalized !== "" &&
    (OFFICIAL_PLUGIN_PERMISSION_KEYS as readonly string[]).includes(normalized)
  );
}

export function isOfficialHostPermissionKey(
  permission: unknown
): permission is OfficialHostPermissionKey {
  const normalized = normalizePluginPermissionKey(permission);
  return (
    normalized !== "" &&
    (OFFICIAL_HOST_PERMISSION_KEYS as readonly string[]).includes(normalized)
  );
}

function normalizePluginRuntime(runtime: unknown): string {
  return asString(runtime).toLowerCase();
}

function hostProtocolVersionForRuntime(runtime: unknown): string {
  switch (normalizePluginRuntime(runtime)) {
    case "grpc":
    case "js_worker":
      return "1.0.0";
    default:
      return DEFAULT_PLUGIN_HOST_PROTOCOL_VERSION;
  }
}

function parsePluginCompatVersion(
  value: unknown
): { major: number; minor: number; patch: number } | { error: string } {
  const raw = asString(value);
  if (!raw) {
    return { error: "version is required" };
  }
  const trimmed = raw.replace(/^v/i, "");
  const parts = trimmed.split(".");
  if (parts.length > 3) {
    return { error: `version "${raw}" has too many segments` };
  }

  const values: [number, number, number] = [0, 0, 0];
  for (let index = 0; index < parts.length; index += 1) {
    const segment = parts[index]?.trim() || "";
    if (!segment) {
      return { error: `version "${raw}" contains an empty segment` };
    }
    if (!/^\d+$/.test(segment)) {
      return { error: `version "${raw}" contains non-numeric segment "${segment}"` };
    }
    values[index] = Number.parseInt(segment, 10);
  }

  return {
    major: values[0],
    minor: values[1],
    patch: values[2]
  };
}

function stringifyPluginCompatVersion(version: { major: number; minor: number; patch: number }): string {
  return `${version.major}.${version.minor}.${version.patch}`;
}

function comparePluginCompatVersion(
  left: { major: number; minor: number; patch: number },
  right: { major: number; minor: number; patch: number }
): number {
  if (left.major !== right.major) {
    return left.major < right.major ? -1 : 1;
  }
  if (left.minor !== right.minor) {
    return left.minor < right.minor ? -1 : 1;
  }
  if (left.patch !== right.patch) {
    return left.patch < right.patch ? -1 : 1;
  }
  return 0;
}

function isScalarManifestValue(value: unknown): value is string | number | boolean {
  return (
    typeof value === "string" ||
    (typeof value === "number" && Number.isFinite(value)) ||
    typeof value === "boolean"
  );
}

function pushManifestIssue(
  issues: PluginManifestValidationIssue[],
  path: string,
  message: string
): void {
  issues.push({
    path,
    message
  });
}

function validateManifestOptionalString(
  source: UnknownMap,
  key: string,
  path: string,
  issues: PluginManifestValidationIssue[]
): void {
  const value = source[key];
  if (value === undefined || value === null) {
    return;
  }
  if (typeof value !== "string") {
    pushManifestIssue(issues, `${path}.${key}`, "must be a string");
  }
}

function validateManifestOptionalBoolean(
  source: UnknownMap,
  key: string,
  path: string,
  issues: PluginManifestValidationIssue[]
): void {
  const value = source[key];
  if (value === undefined || value === null) {
    return;
  }
  if (typeof value !== "boolean") {
    pushManifestIssue(issues, `${path}.${key}`, "must be a boolean");
  }
}

function normalizeManifestSchemaFieldType(value: unknown): string | "" {
  const normalized = asString(value).toLowerCase();
  if (!normalized) {
    return "string";
  }
  switch (normalized) {
    case "string":
    case "textarea":
    case "number":
    case "boolean":
    case "select":
    case "json":
    case "secret":
      return normalized;
    default:
      return "";
  }
}

function manifestSchemaValueFingerprint(value: unknown): string {
  if (value === undefined) {
    return "undefined";
  }
  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
  }
}

function validateManifestSchemaDefaultValue(
  value: unknown,
  fieldType: string,
  fieldPath: string,
  issues: PluginManifestValidationIssue[]
): void {
  if (value === undefined || value === null) {
    return;
  }
  switch (fieldType) {
    case "string":
    case "textarea":
    case "secret":
      if (typeof value !== "string") {
        pushManifestIssue(issues, `${fieldPath}.default`, "must be a string");
      }
      return;
    case "number":
      if (typeof value !== "number" || !Number.isFinite(value)) {
        pushManifestIssue(issues, `${fieldPath}.default`, "must be a number");
      }
      return;
    case "boolean":
      if (typeof value !== "boolean") {
        pushManifestIssue(issues, `${fieldPath}.default`, "must be a boolean");
      }
      return;
    default:
      return;
  }
}

function validateManifestSchemaFieldOptions(
  raw: unknown,
  fieldPath: string,
  issues: PluginManifestValidationIssue[]
): { fingerprints: string[]; has_options: boolean } {
  if (raw === undefined || raw === null) {
    return { fingerprints: [], has_options: false };
  }
  if (!Array.isArray(raw)) {
    pushManifestIssue(issues, `${fieldPath}.options`, "must be an array");
    return { fingerprints: [], has_options: true };
  }

  const fingerprints: string[] = [];
  raw.forEach((item, index) => {
    const optionPath = `${fieldPath}.options[${index}]`;
    if (item === null || item === undefined) {
      return;
    }
    if (isScalarManifestValue(item)) {
      fingerprints.push(manifestSchemaValueFingerprint(item));
      return;
    }
    if (typeof item !== "object" || Array.isArray(item)) {
      pushManifestIssue(issues, optionPath, "must be a scalar or object");
      return;
    }

    const option = asRecord(item);
    validateManifestOptionalString(option, "label", optionPath, issues);
    validateManifestOptionalString(option, "description", optionPath, issues);

    let value = option.value;
    if (value === undefined) {
      const keyValue = asString(option.key);
      if (!keyValue) {
        pushManifestIssue(issues, `${optionPath}.value`, "is required when key is absent");
        return;
      }
      value = keyValue;
    }
    fingerprints.push(manifestSchemaValueFingerprint(value));
  });

  return { fingerprints, has_options: true };
}

function validateManifestObjectSchema(
  schemaValue: unknown,
  schemaName: string,
  issues: PluginManifestValidationIssue[]
): void {
  if (schemaValue === undefined || schemaValue === null) {
    return;
  }
  if (typeof schemaValue !== "object" || Array.isArray(schemaValue)) {
    pushManifestIssue(issues, schemaName, "must be an object");
    return;
  }

  const schema = asRecord(schemaValue);
  validateManifestOptionalString(schema, "title", schemaName, issues);
  validateManifestOptionalString(schema, "description", schemaName, issues);

  if (!("fields" in schema)) {
    pushManifestIssue(issues, `${schemaName}.fields`, "is required");
    return;
  }
  if (!Array.isArray(schema.fields) || schema.fields.length === 0) {
    pushManifestIssue(issues, `${schemaName}.fields`, "must be a non-empty array");
    return;
  }

  const seenKeys = new Set<string>();
  schema.fields.forEach((item, index) => {
    const fieldPath = `${schemaName}.fields[${index}]`;
    if (!item || typeof item !== "object" || Array.isArray(item)) {
      pushManifestIssue(issues, fieldPath, "must be an object");
      return;
    }

    const field = asRecord(item);
    const key = asString(field.key);
    if (!key) {
      pushManifestIssue(issues, `${fieldPath}.key`, "is required and must be a non-empty string");
      return;
    }
    if (seenKeys.has(key)) {
      pushManifestIssue(issues, `${fieldPath}.key`, `duplicates "${key}"`);
    } else {
      seenKeys.add(key);
    }

    validateManifestOptionalString(field, "label", fieldPath, issues);
    validateManifestOptionalString(field, "description", fieldPath, issues);
    validateManifestOptionalString(field, "placeholder", fieldPath, issues);
    validateManifestOptionalBoolean(field, "required", fieldPath, issues);

    const fieldType = normalizeManifestSchemaFieldType(field.type);
    if (!fieldType) {
      pushManifestIssue(
        issues,
        `${fieldPath}.type`,
        `value ${field.type === undefined ? "undefined" : JSON.stringify(field.type)} is unsupported`
      );
      return;
    }

    validateManifestSchemaDefaultValue(field.default, fieldType, fieldPath, issues);
    const options = validateManifestSchemaFieldOptions(field.options, fieldPath, issues);
    if (fieldType === "select") {
      if (!options.has_options || options.fingerprints.length === 0) {
        pushManifestIssue(
          issues,
          `${fieldPath}.options`,
          "must be a non-empty array for select fields"
        );
      } else if (field.default !== undefined) {
        const defaultFingerprint = manifestSchemaValueFingerprint(field.default);
        if (!options.fingerprints.includes(defaultFingerprint)) {
          pushManifestIssue(
            issues,
            `${fieldPath}.default`,
            "must match one of the select options"
          );
        }
      }
    }
  });
}

function normalizeManifestPagePath(path: unknown): string {
  const raw = asString(path);
  if (!raw) {
    return "/";
  }
  const normalized = raw.startsWith("/") ? raw : `/${raw}`;
  const segments = normalized.split("/");
  const output: string[] = [];
  segments.forEach((segment) => {
    if (!segment || segment === ".") {
      return;
    }
    if (segment === "..") {
      output.pop();
      return;
    }
    output.push(segment);
  });
  return output.length === 0 ? "/" : `/${output.join("/")}`;
}

function validateManifestFrontendPage(
  pageValue: unknown,
  area: "admin" | "user",
  fieldPath: string,
  issues: PluginManifestValidationIssue[]
): void {
  if (pageValue === undefined || pageValue === null) {
    return;
  }
  if (typeof pageValue !== "object" || Array.isArray(pageValue)) {
    pushManifestIssue(issues, fieldPath, "must be an object");
    return;
  }

  const page = asRecord(pageValue);
  const path = asString(page.path);
  if (!path) {
    pushManifestIssue(issues, `${fieldPath}.path`, "is required");
    return;
  }

  const normalizedPath = normalizeManifestPagePath(path);
  const expectedPrefix = area === "admin" ? "/admin/plugin-pages/" : "/plugin-pages/";
  if (!normalizedPath.startsWith(expectedPrefix)) {
    pushManifestIssue(issues, `${fieldPath}.path`, `must start with "${expectedPrefix}"`);
  }
}

function normalizeManifestWebhookMethod(raw: unknown): string | "" {
  const method = asString(raw).toUpperCase();
  if (!method) {
    return "POST";
  }
  if (method === "*" || method === "ANY") {
    return "*";
  }
  switch (method) {
    case "GET":
    case "POST":
    case "PUT":
    case "PATCH":
    case "DELETE":
      return method;
    default:
      return "";
  }
}

function normalizeManifestWebhookAuthMode(raw: unknown): string | "" {
  const mode = asString(raw).toLowerCase();
  if (!mode) {
    return "none";
  }
  switch (mode) {
    case "none":
    case "query":
    case "header":
    case "hmac_sha256":
      return mode;
    default:
      return "";
  }
}

function normalizeManifestFrontendMinScope(raw: unknown): string | "" {
  const value = asString(raw).toLowerCase();
  if (!value) {
    return "";
  }
  switch (value) {
    case "guest":
      return "guest";
    case "authenticated":
    case "auth":
    case "user":
    case "member":
      return "authenticated";
    case "super_admin":
    case "superadmin":
    case "root":
      return "super_admin";
    default:
      return "";
  }
}

function normalizeManifestFrontendHTMLMode(raw: unknown): string | "" {
  const value = asString(raw).toLowerCase();
  if (!value) {
    return "";
  }
  switch (value) {
    case "sanitize":
    case "trusted":
      return value;
    default:
      return "";
  }
}

function normalizeManifestFrontendArea(raw: unknown): string | "" {
  switch (asString(raw).toLowerCase()) {
    case "admin":
      return "admin";
    case "user":
      return "user";
    case "*":
      return "*";
    default:
      return "";
  }
}

function validateManifestPermissionsShape(
  permissionsValue: unknown,
  issues: PluginManifestValidationIssue[]
): void {
  if (permissionsValue === undefined || permissionsValue === null) {
    return;
  }
  if (!Array.isArray(permissionsValue)) {
    pushManifestIssue(issues, "permissions", "must be an array");
    return;
  }

  const seenKeys = new Set<string>();
  permissionsValue.forEach((item, index) => {
    const fieldPath = `permissions[${index}]`;
    if (!item || typeof item !== "object" || Array.isArray(item)) {
      pushManifestIssue(issues, fieldPath, "must be an object");
      return;
    }
    const permission = asRecord(item);
    const key = asString(permission.key);
    if (!key) {
      pushManifestIssue(issues, `${fieldPath}.key`, "is required and must be a non-empty string");
      return;
    }
    if (seenKeys.has(key)) {
      pushManifestIssue(issues, `${fieldPath}.key`, `duplicates "${key}"`);
    } else {
      seenKeys.add(key);
    }
    validateManifestOptionalBoolean(permission, "required", fieldPath, issues);
    validateManifestOptionalBoolean(permission, "default_granted", fieldPath, issues);
  });
}

function validateManifestCapabilitiesShape(
  capabilitiesValue: unknown,
  issues: PluginManifestValidationIssue[]
): void {
  if (capabilitiesValue === undefined || capabilitiesValue === null) {
    return;
  }
  if (typeof capabilitiesValue !== "object" || Array.isArray(capabilitiesValue)) {
    pushManifestIssue(issues, "capabilities", "must be an object");
    return;
  }

  const capabilities = asRecord(capabilitiesValue);
  const executeActionStorage = capabilities.execute_action_storage;
  if (executeActionStorage !== undefined && executeActionStorage !== null) {
    if (typeof executeActionStorage !== "object" || Array.isArray(executeActionStorage)) {
      pushManifestIssue(issues, "capabilities.execute_action_storage", "must be an object");
    } else {
      Object.entries(asRecord(executeActionStorage)).forEach(([action, mode]) => {
        const normalizedAction = normalizeHookName(action);
        if (!normalizedAction) {
          pushManifestIssue(
            issues,
            "capabilities.execute_action_storage",
            "keys must be non-empty action names"
          );
          return;
        }
        const normalizedMode = asString(mode).toLowerCase();
        if (!["unknown", "none", "read", "write"].includes(normalizedMode)) {
          pushManifestIssue(
            issues,
            `capabilities.execute_action_storage.${normalizedAction}`,
            "must be one of unknown/none/read/write"
          );
        }
      });
    }
  }

  ["allow_block", "allow_payload_patch", "allow_frontend_extensions", "allow_execute_api", "allow_network", "allow_file_system"].forEach((key) => {
    validateManifestOptionalBoolean(capabilities, key, "capabilities", issues);
  });

  if (
    capabilities.frontend_min_scope !== undefined &&
    capabilities.frontend_min_scope !== null &&
    !normalizeManifestFrontendMinScope(capabilities.frontend_min_scope)
  ) {
    pushManifestIssue(
      issues,
      "capabilities.frontend_min_scope",
      "must be one of guest/authenticated/super_admin or their documented aliases"
    );
  }

  ["frontend_html_mode", "html_mode"].forEach((key) => {
    const value = capabilities[key];
    if (value === undefined || value === null) {
      return;
    }
    if (!normalizeManifestFrontendHTMLMode(value)) {
      pushManifestIssue(
        issues,
        `capabilities.${key}`,
        "must be one of sanitize/trusted"
      );
    }
  });

  ["frontend_required_permissions", "frontend_allowed_areas"].forEach((key) => {
    const value = capabilities[key];
    if (value === undefined || value === null) {
      return;
    }
    if (!Array.isArray(value)) {
      pushManifestIssue(issues, `capabilities.${key}`, "must be an array");
      return;
    }
    value.forEach((item, index) => {
      if (!asString(item)) {
        pushManifestIssue(
          issues,
          `capabilities.${key}[${index}]`,
          "must be a non-empty string"
        );
      }
    });
  });

  if (Array.isArray(capabilities.frontend_allowed_areas)) {
    capabilities.frontend_allowed_areas.forEach((item, index) => {
      if (!normalizeManifestFrontendArea(item)) {
        pushManifestIssue(
          issues,
          `capabilities.frontend_allowed_areas[${index}]`,
          "must be one of admin/user/*"
        );
      }
    });
  }
}

function compatibilityPathFromReasonCode(reasonCode: string): string {
  switch (reasonCode) {
    case "invalid_manifest_version":
    case "manifest_version_unsupported":
      return "manifest_version";
    case "invalid_protocol_version":
    case "protocol_version_unsupported":
      return "protocol_version";
    case "invalid_min_host_protocol_version":
    case "host_protocol_too_old":
      return "min_host_protocol_version";
    case "invalid_max_host_protocol_version":
    case "host_protocol_too_new":
      return "max_host_protocol_version";
    case "invalid_host_protocol_range":
      return "min_host_protocol_version";
    default:
      return "manifest";
  }
}

export function inspectPluginManifestCompatibility(
  manifestOrMetadata: unknown,
  runtimeOverride?: string
): PluginManifestCompatibilityInspection {
  const manifest = asRecord(manifestOrMetadata);
  const runtime = normalizePluginRuntime(runtimeOverride || manifest.runtime);
  const hostManifestVersionParsed = parsePluginCompatVersion(PLUGIN_HOST_MANIFEST_VERSION);
  const hostProtocolVersionParsed = parsePluginCompatVersion(hostProtocolVersionForRuntime(runtime));

  const hostManifestVersion =
    "error" in hostManifestVersionParsed ? PLUGIN_HOST_MANIFEST_VERSION : stringifyPluginCompatVersion(hostManifestVersionParsed);
  const hostProtocolVersion =
    "error" in hostProtocolVersionParsed ? DEFAULT_PLUGIN_HOST_PROTOCOL_VERSION : stringifyPluginCompatVersion(hostProtocolVersionParsed);

  const inspection: PluginManifestCompatibilityInspection = {
    manifest_present: Object.keys(manifest).length > 0,
    runtime,
    host_manifest_version: hostManifestVersion,
    manifest_version: "",
    host_protocol_version: hostProtocolVersion,
    protocol_version: "",
    min_host_protocol_version: "",
    max_host_protocol_version: "",
    compatible: true,
    legacy_defaults_applied: false,
    reason_code: "compatible",
    reason: "plugin manifest compatibility metadata matches the current host"
  };

  if (!inspection.manifest_present) {
    inspection.legacy_defaults_applied = true;
    inspection.manifest_version = hostManifestVersion;
    inspection.protocol_version = hostProtocolVersion;
    inspection.reason_code = "manifest_missing_assumed_legacy";
    inspection.reason =
      "plugin manifest is missing, so host compatibility defaults are assumed";
    return inspection;
  }

  const hostManifestParsed =
    "error" in hostManifestVersionParsed
      ? { major: 1, minor: 0, patch: 0 }
      : hostManifestVersionParsed;
  const hostProtocolParsed =
    "error" in hostProtocolVersionParsed
      ? { major: 1, minor: 0, patch: 0 }
      : hostProtocolVersionParsed;

  let manifestVersionRaw = asString(manifest.manifest_version);
  if (!manifestVersionRaw) {
    inspection.legacy_defaults_applied = true;
    manifestVersionRaw = PLUGIN_HOST_MANIFEST_VERSION;
  }
  const manifestVersionParsed = parsePluginCompatVersion(manifestVersionRaw);
  if ("error" in manifestVersionParsed) {
    inspection.compatible = false;
    inspection.reason_code = "invalid_manifest_version";
    inspection.reason = `manifest_version must be a numeric semantic version: ${manifestVersionParsed.error}`;
    return inspection;
  }
  inspection.manifest_version = stringifyPluginCompatVersion(manifestVersionParsed);
  if (
    manifestVersionParsed.major !== hostManifestParsed.major ||
    comparePluginCompatVersion(manifestVersionParsed, hostManifestParsed) > 0
  ) {
    inspection.compatible = false;
    inspection.reason_code = "manifest_version_unsupported";
    inspection.reason =
      `manifest_version ${inspection.manifest_version} is not supported by host manifest schema ${stringifyPluginCompatVersion(hostManifestParsed)}`;
    return inspection;
  }

  let protocolVersionRaw = asString(manifest.protocol_version);
  if (!protocolVersionRaw) {
    inspection.legacy_defaults_applied = true;
    protocolVersionRaw = hostProtocolVersion;
  }
  const protocolVersionParsed = parsePluginCompatVersion(protocolVersionRaw);
  if ("error" in protocolVersionParsed) {
    inspection.compatible = false;
    inspection.reason_code = "invalid_protocol_version";
    inspection.reason = `protocol_version must be a numeric semantic version: ${protocolVersionParsed.error}`;
    return inspection;
  }
  inspection.protocol_version = stringifyPluginCompatVersion(protocolVersionParsed);
  if (
    protocolVersionParsed.major !== hostProtocolParsed.major ||
    comparePluginCompatVersion(protocolVersionParsed, hostProtocolParsed) > 0
  ) {
    inspection.compatible = false;
    inspection.reason_code = "protocol_version_unsupported";
    inspection.reason =
      `plugin protocol_version ${inspection.protocol_version} is newer than host protocol ${stringifyPluginCompatVersion(hostProtocolParsed)}`;
    return inspection;
  }

  const minHostProtocolRaw = asString(manifest.min_host_protocol_version);
  if (minHostProtocolRaw) {
    const minHostProtocolParsed = parsePluginCompatVersion(minHostProtocolRaw);
    if ("error" in minHostProtocolParsed) {
      inspection.compatible = false;
      inspection.reason_code = "invalid_min_host_protocol_version";
      inspection.reason =
        `min_host_protocol_version must be a numeric semantic version: ${minHostProtocolParsed.error}`;
      return inspection;
    }
    inspection.min_host_protocol_version = stringifyPluginCompatVersion(minHostProtocolParsed);
    if (comparePluginCompatVersion(hostProtocolParsed, minHostProtocolParsed) < 0) {
      inspection.compatible = false;
      inspection.reason_code = "host_protocol_too_old";
      inspection.reason =
        `host protocol ${stringifyPluginCompatVersion(hostProtocolParsed)} is older than required minimum ${inspection.min_host_protocol_version}`;
      return inspection;
    }
  }

  const maxHostProtocolRaw = asString(manifest.max_host_protocol_version);
  if (maxHostProtocolRaw) {
    const maxHostProtocolParsed = parsePluginCompatVersion(maxHostProtocolRaw);
    if ("error" in maxHostProtocolParsed) {
      inspection.compatible = false;
      inspection.reason_code = "invalid_max_host_protocol_version";
      inspection.reason =
        `max_host_protocol_version must be a numeric semantic version: ${maxHostProtocolParsed.error}`;
      return inspection;
    }
    inspection.max_host_protocol_version = stringifyPluginCompatVersion(maxHostProtocolParsed);
    if (comparePluginCompatVersion(hostProtocolParsed, maxHostProtocolParsed) > 0) {
      inspection.compatible = false;
      inspection.reason_code = "host_protocol_too_new";
      inspection.reason =
        `host protocol ${stringifyPluginCompatVersion(hostProtocolParsed)} is newer than declared maximum ${inspection.max_host_protocol_version}`;
      return inspection;
    }
  }

  if (inspection.min_host_protocol_version && inspection.max_host_protocol_version) {
    const minHostProtocolParsed = parsePluginCompatVersion(inspection.min_host_protocol_version);
    const maxHostProtocolParsed = parsePluginCompatVersion(inspection.max_host_protocol_version);
    if (
      !("error" in minHostProtocolParsed) &&
      !("error" in maxHostProtocolParsed) &&
      comparePluginCompatVersion(minHostProtocolParsed, maxHostProtocolParsed) > 0
    ) {
      inspection.compatible = false;
      inspection.reason_code = "invalid_host_protocol_range";
      inspection.reason =
        `min_host_protocol_version ${inspection.min_host_protocol_version} cannot be greater than max_host_protocol_version ${inspection.max_host_protocol_version}`;
      return inspection;
    }
  }

  if (inspection.legacy_defaults_applied) {
    inspection.reason_code = "compatible_assumed_legacy";
    inspection.reason =
      "plugin uses compatibility defaults because manifest version metadata is incomplete";
  }

  return inspection;
}

export function validatePluginManifestSchema(
  manifestValue: unknown
): PluginManifestSchemaValidation {
  const issues: PluginManifestValidationIssue[] = [];

  if (!manifestValue || typeof manifestValue !== "object" || Array.isArray(manifestValue)) {
    pushManifestIssue(issues, "manifest", "must be an object");
  }

  const manifest = asRecord(manifestValue);

  validateManifestObjectSchema(manifest.config_schema, "config_schema", issues);
  validateManifestObjectSchema(manifest.secret_schema, "secret_schema", issues);
  validateManifestObjectSchema(manifest.runtime_params_schema, "runtime_params_schema", issues);
  validateManifestPermissionsShape(manifest.permissions, issues);
  validateManifestCapabilitiesShape(manifest.capabilities, issues);

  if (manifest.frontend !== undefined && manifest.frontend !== null) {
    if (typeof manifest.frontend !== "object" || Array.isArray(manifest.frontend)) {
      pushManifestIssue(issues, "frontend", "must be an object");
    } else {
      const frontend = asRecord(manifest.frontend);
      validateManifestFrontendPage(frontend.admin_page, "admin", "frontend.admin_page", issues);
      validateManifestFrontendPage(frontend.user_page, "user", "frontend.user_page", issues);
    }
  }

  if (manifest.webhooks !== undefined && manifest.webhooks !== null) {
    if (!Array.isArray(manifest.webhooks)) {
      pushManifestIssue(issues, "webhooks", "must be an array");
    } else {
      const seenWebhookKeys = new Set<string>();
      manifest.webhooks.forEach((item, index) => {
        const fieldPath = `webhooks[${index}]`;
        if (!item || typeof item !== "object" || Array.isArray(item)) {
          pushManifestIssue(issues, fieldPath, "must be an object");
          return;
        }
        const webhook = asRecord(item);
        const key = asString(webhook.key);
        if (!key) {
          pushManifestIssue(issues, `${fieldPath}.key`, "is required");
        } else if (seenWebhookKeys.has(key)) {
          pushManifestIssue(issues, `${fieldPath}.key`, `duplicates "${key}"`);
        } else {
          seenWebhookKeys.add(key);
        }

        if (!normalizeManifestWebhookMethod(webhook.method)) {
          pushManifestIssue(
            issues,
            `${fieldPath}.method`,
            "must be one of GET/POST/PUT/PATCH/DELETE/*"
          );
        }

        const authMode = normalizeManifestWebhookAuthMode(webhook.auth_mode);
        if (!authMode) {
          pushManifestIssue(
            issues,
            `${fieldPath}.auth_mode`,
            "must be one of none/query/header/hmac_sha256"
          );
        } else if (authMode !== "none" && !asString(webhook.secret_key)) {
          pushManifestIssue(
            issues,
            `${fieldPath}.secret_key`,
            `is required when auth_mode is "${authMode}"`
          );
        }
      });
    }
  }

  const compatibility = inspectPluginManifestCompatibility(manifest);
  if (!compatibility.compatible) {
    pushManifestIssue(
      issues,
      compatibilityPathFromReasonCode(compatibility.reason_code),
      compatibility.reason
    );
  }

  return {
    valid: issues.length === 0,
    issues,
    compatibility
  };
}

export function validatePluginManifestCatalog(
  manifestOrCapabilities: unknown
): PluginManifestCatalogValidation {
  const root = asRecord(manifestOrCapabilities);
  const capabilities =
    "capabilities" in root ? asRecord(root.capabilities) : root;

  const hooks = normalizeStringList(capabilities.hooks, normalizeHookName);
  const disabledHooks = normalizeStringList(capabilities.disabled_hooks, normalizeHookName);
  const allowedFrontendSlots = normalizeStringList(
    capabilities.allowed_frontend_slots,
    asString
  );
  const requestedPermissions = normalizeStringList(
    capabilities.requested_permissions,
    normalizePluginPermissionKey
  );
  const grantedPermissions = normalizeStringList(
    capabilities.granted_permissions,
    normalizePluginPermissionKey
  );
  const declaredPermissions = normalizeManifestPermissionEntries(root.permissions);

  const invalidHooks = hooks.filter((hook) => !isOfficialPluginHook(hook));
  const invalidDisabledHooks = disabledHooks.filter((hook) => !isOfficialPluginHook(hook));
  const invalidAllowedFrontendSlots = allowedFrontendSlots.filter(
    (slot) => !isOfficialFrontendSlot(slot)
  );
  const invalidRequestedPermissions = requestedPermissions.filter(
    (permission) => !isOfficialPluginPermissionKey(permission)
  );
  const invalidGrantedPermissions = grantedPermissions.filter(
    (permission) => !isOfficialPluginPermissionKey(permission)
  );
  const invalidDeclaredPermissions = declaredPermissions.filter(
    (permission) => !isOfficialPluginPermissionKey(permission)
  );

  const validDeclaredPermissionSet = new Set(
    declaredPermissions.filter((permission) => !invalidDeclaredPermissions.includes(permission))
  );
  const validRequestedPermissionSet = new Set(
    requestedPermissions.filter((permission) => !invalidRequestedPermissions.includes(permission))
  );

  const requestedPermissionsMissingDeclaration = requestedPermissions.filter(
    (permission) =>
      !invalidRequestedPermissions.includes(permission) &&
      declaredPermissions.length > 0 &&
      !validDeclaredPermissionSet.has(permission)
  );
  const grantedPermissionsMissingDeclaration = grantedPermissions.filter(
    (permission) =>
      !invalidGrantedPermissions.includes(permission) &&
      declaredPermissions.length > 0 &&
      !validDeclaredPermissionSet.has(permission)
  );
  const declaredPermissionsMissingRequest = declaredPermissions.filter(
    (permission) =>
      !invalidDeclaredPermissions.includes(permission) &&
      requestedPermissions.length > 0 &&
      !validRequestedPermissionSet.has(permission)
  );

  const valid =
    invalidHooks.length === 0 &&
    invalidDisabledHooks.length === 0 &&
    invalidAllowedFrontendSlots.length === 0 &&
    invalidRequestedPermissions.length === 0 &&
    invalidGrantedPermissions.length === 0 &&
    invalidDeclaredPermissions.length === 0 &&
    requestedPermissionsMissingDeclaration.length === 0 &&
    grantedPermissionsMissingDeclaration.length === 0 &&
    declaredPermissionsMissingRequest.length === 0;

  return {
    valid,
    hooks,
    invalid_hooks: invalidHooks,
    disabled_hooks: disabledHooks,
    invalid_disabled_hooks: invalidDisabledHooks,
    allowed_frontend_slots: allowedFrontendSlots,
    invalid_allowed_frontend_slots: invalidAllowedFrontendSlots,
    requested_permissions: requestedPermissions,
    invalid_requested_permissions: invalidRequestedPermissions,
    granted_permissions: grantedPermissions,
    invalid_granted_permissions: invalidGrantedPermissions,
    declared_permissions: declaredPermissions,
    invalid_declared_permissions: invalidDeclaredPermissions,
    requested_permissions_missing_declaration: requestedPermissionsMissingDeclaration,
    granted_permissions_missing_declaration: grantedPermissionsMissingDeclaration,
    declared_permissions_missing_request: declaredPermissionsMissingRequest
  };
}

export function resolvePluginPageContext(context?: PluginExecutionContext): PluginPageContext {
  const metadata = context?.metadata || {};
  const path = asString(metadata.plugin_page_path);
  const fullPath =
    asString(metadata.plugin_page_full_path) ||
    buildPluginPageFullPath(path, metadata.plugin_page_query_string, metadata.plugin_page_query_params);
  return {
    area: asString(metadata.bootstrap_area),
    path,
    full_path: fullPath,
    query_string:
      asString(metadata.plugin_page_query_string) ||
      extractQueryStringFromFullPath(fullPath),
    query_params: safeParseStringMap(metadata.plugin_page_query_params),
    route_params: safeParseStringMap(metadata.plugin_page_route_params)
  };
}

export function getPluginRuntime(): PluginRuntimeGlobal | undefined {
  if (!globalThis || typeof globalThis !== "object") {
    return undefined;
  }
  return globalThis.Plugin;
}

function buildPluginPageFullPath(
  path: string,
  queryStringValue?: string,
  queryParamsValue?: string
): string {
  const queryString =
    asString(queryStringValue) ||
    buildPluginPageQueryString(safeParseStringMap(queryParamsValue));
  if (!path) {
    return queryString ? `?${queryString}` : "";
  }
  if (!queryString) {
    return path;
  }
  return `${path}?${queryString}`;
}

function buildPluginPageQueryString(queryParams: StringMap): string {
  const query = new URLSearchParams();
  Object.keys(queryParams)
    .sort((left, right) => left.localeCompare(right))
    .forEach((key) => {
      query.append(key, queryParams[key] ?? "");
    });
  return query.toString();
}

function extractQueryStringFromFullPath(fullPath: string): string {
  if (!fullPath) {
    return "";
  }
  const queryStart = fullPath.indexOf("?");
  if (queryStart < 0 || queryStart >= fullPath.length - 1) {
    return "";
  }
  return fullPath.slice(queryStart + 1);
}

export function getPluginStorage(): PluginStorageAPI | undefined {
  return getPluginRuntime()?.storage;
}

export function getPluginWorkspace(): PluginWorkspaceAPI | undefined {
  return getPluginRuntime()?.workspace;
}

export function getPluginSecret(): PluginSecretAPI | undefined {
  return getPluginRuntime()?.secret;
}

export function getPluginWebhook(): PluginWebhookAPI | undefined {
  return getPluginRuntime()?.webhook;
}

export function getPluginHTTP(): PluginHTTPAPI | undefined {
  return getPluginRuntime()?.http;
}

export function getPluginFS(): PluginFSAPI | undefined {
  return getPluginRuntime()?.fs;
}

export function getPluginHost(): PluginHostAPI | undefined {
  return getPluginRuntime()?.host;
}

export function getPluginOrder(): PluginOrderAPI | undefined {
  const runtime = getPluginRuntime();
  return runtime?.order ?? runtime?.host?.order;
}

export function getPluginUser(): PluginUserAPI | undefined {
  const runtime = getPluginRuntime();
  return runtime?.user ?? runtime?.host?.user;
}

export function getPluginProduct(): PluginProductAPI | undefined {
  const runtime = getPluginRuntime();
  return runtime?.product ?? runtime?.host?.product;
}

export function getPluginInventory(): PluginInventoryAPI | undefined {
  const runtime = getPluginRuntime();
  return runtime?.inventory ?? runtime?.host?.inventory;
}

export function getPluginInventoryBinding(): PluginInventoryBindingAPI | undefined {
  const runtime = getPluginRuntime();
  return runtime?.inventoryBinding ?? runtime?.host?.inventoryBinding;
}

export function getPluginPromo(): PluginPromoAPI | undefined {
  const runtime = getPluginRuntime();
  return runtime?.promo ?? runtime?.host?.promo;
}

export function getPluginTicket(): PluginTicketAPI | undefined {
  const runtime = getPluginRuntime();
  return runtime?.ticket ?? runtime?.host?.ticket;
}

export function getPluginSerial(): PluginSerialAPI | undefined {
  const runtime = getPluginRuntime();
  return runtime?.serial ?? runtime?.host?.serial;
}

export function getPluginAnnouncement(): PluginAnnouncementAPI | undefined {
  const runtime = getPluginRuntime();
  return runtime?.announcement ?? runtime?.host?.announcement;
}

export function getPluginKnowledge(): PluginKnowledgeAPI | undefined {
  const runtime = getPluginRuntime();
  return runtime?.knowledge ?? runtime?.host?.knowledge;
}

export function getPluginPaymentMethod(): PluginPaymentMethodAPI | undefined {
  const runtime = getPluginRuntime();
  return runtime?.paymentMethod ?? runtime?.host?.paymentMethod;
}

export function getPluginVirtualInventory(): PluginVirtualInventoryAPI | undefined {
  const runtime = getPluginRuntime();
  return runtime?.virtualInventory ?? runtime?.host?.virtualInventory;
}

export function getPluginVirtualInventoryBinding(): PluginVirtualInventoryBindingAPI | undefined {
  const runtime = getPluginRuntime();
  return runtime?.virtualInventoryBinding ?? runtime?.host?.virtualInventoryBinding;
}

export function getPluginMarket(): PluginMarketAPI | undefined {
  const runtime = getPluginRuntime();
  return runtime?.market ?? runtime?.host?.market;
}

export function getPluginEmailTemplate(): PluginEmailTemplateAPI | undefined {
  const runtime = getPluginRuntime();
  return runtime?.emailTemplate ?? runtime?.host?.emailTemplate;
}

export function getPluginLandingPage(): PluginLandingPageAPI | undefined {
  const runtime = getPluginRuntime();
  return runtime?.landingPage ?? runtime?.host?.landingPage;
}

export function getPluginInvoiceTemplate(): PluginInvoiceTemplateAPI | undefined {
  const runtime = getPluginRuntime();
  return runtime?.invoiceTemplate ?? runtime?.host?.invoiceTemplate;
}

export function getPluginAuthBranding(): PluginAuthBrandingAPI | undefined {
  const runtime = getPluginRuntime();
  return runtime?.authBranding ?? runtime?.host?.authBranding;
}

export function getPluginPageRulePack(): PluginPageRulePackAPI | undefined {
  const runtime = getPluginRuntime();
  return runtime?.pageRulePack ?? runtime?.host?.pageRulePack;
}

export function normalizePluginFSUsage(
  value: unknown,
  fs?: Pick<PluginFSAPI, "maxFiles" | "maxTotalBytes">
): PluginFSUsage {
  const record = asRecord(value);
  return {
    file_count: asInteger(record.file_count ?? record.FileCount, 0),
    total_bytes: asInteger(record.total_bytes ?? record.TotalBytes, 0),
    max_files: asInteger(record.max_files ?? record.MaxFiles, fs?.maxFiles ?? 0),
    max_bytes: asInteger(record.max_bytes ?? record.MaxBytes, fs?.maxTotalBytes ?? 0)
  };
}

export function isPluginExecuteSuccess(value: unknown): value is PluginExecuteSuccessResult {
  const record = asRecord(value);
  return record.success === true;
}

export function isPluginExecuteError(value: unknown): value is PluginExecuteErrorResult {
  const record = asRecord(value);
  return record.success === false && typeof record.error === "string";
}

export * from "./frontend";

export {};
