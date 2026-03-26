import type { PluginPermissionRequest } from '@/lib/api'

export type PluginForm = {
  name: string
  display_name: string
  description: string
  type: string
  runtime: string
  package_path: string
  address: string
  version: string
  config: string
  runtime_params: string
  capabilities: string
  enabled: boolean
}

export type UploadForm = {
  plugin_id: string
  name: string
  display_name: string
  description: string
  type: string
  runtime: string
  address: string
  version: string
  config: string
  runtime_params: string
  capabilities: string
  changelog: string
  activate: boolean
  auto_start: boolean
}

export type PluginHookAccessState = {
  allowAllHooks: boolean
  selectedHooks: string[]
  disabledHooks: string[]
}

export type PluginFrontendAccessState = {
  allowFrontendExtensions: boolean
  allowAllFrontendAreas: boolean
  selectedFrontendAreas: string[]
  frontendMinScope: 'guest' | 'authenticated' | 'super_admin'
  allowAllFrontendSlots: boolean
  selectedFrontendSlots: string[]
  frontendRequiredPermissions: string[]
}

export type PluginCapabilityPolicyState = {
  requestedPermissions: string[]
  grantedPermissions: string[]
  allowBlock: boolean
  allowPayloadPatch: boolean
  allowExecuteApi: boolean
  allowNetwork: boolean
  allowFileSystem: boolean
  trustedHtmlMode: boolean
}

export type PluginJSONSchemaFieldType =
  | 'string'
  | 'secret'
  | 'textarea'
  | 'number'
  | 'boolean'
  | 'select'
  | 'json'

export type PluginJSONSchemaFieldOption = {
  label: string
  value: unknown
  description?: string
}

export type PluginJSONSchemaField = {
  key: string
  label: string
  description?: string
  type: PluginJSONSchemaFieldType
  placeholder?: string
  required?: boolean
  defaultValue?: unknown
  options?: PluginJSONSchemaFieldOption[]
}

export type PluginJSONSchema = {
  title?: string
  description?: string
  fields: PluginJSONSchemaField[]
}

export type LifecyclePayload = {
  pluginId: number
  action: string
}

export type ActivatePayload = {
  pluginId: number
  versionId: number
  autoStart: boolean
}

export type UploadPermissionPreview = {
  manifest?: Record<string, unknown> | null
  requested_permissions: PluginPermissionRequest[]
  default_granted_permissions: string[]
}

export type PluginManifestPreview = Record<string, unknown>

export type MarketPluginInstallContext = {
  source: {
    source_id: string
    name?: string
    base_url: string
    public_key?: string
    default_channel?: string
    allowed_kinds?: string[]
    enabled?: boolean
  }
  coordinates: {
    source_id: string
    kind: string
    name: string
    version: string
  }
  release?: Record<string, unknown> | null
  compatibility?: Record<string, unknown> | null
  target_state?: Record<string, unknown> | null
  warnings?: string[]
}

export type PluginHookGroupKey =
  | 'frontend'
  | 'auth'
  | 'order'
  | 'payment'
  | 'ticket'
  | 'product_inventory'
  | 'promo'

export type PluginLifecycleActionState = {
  install: boolean
  start: boolean
  pause: boolean
  restart: boolean
  hotReload: boolean
  resume: boolean
  retire: boolean
  test: boolean
  execute: boolean
  upload: boolean
  versions: boolean
  logs: boolean
  edit: boolean
  remove: boolean
}
