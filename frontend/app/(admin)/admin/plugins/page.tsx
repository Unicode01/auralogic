'use client'

import { Suspense, useEffect, useMemo, useState } from 'react'
import { usePathname, useRouter, useSearchParams } from 'next/navigation'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  activateAdminPluginVersion,
  cancelAdminPluginExecutionTask,
  createAdminPlugin,
  deleteAdminPlugin,
  deleteAdminPluginVersion,
  enterAdminPluginWorkspaceTerminalLine,
  getAdminPluginDiagnostics,
  getAdminPluginHookCatalog,
  getAdminPluginExecutions,
  getAdminPluginSecrets,
  getAdminPluginWorkspace,
  getAdminPluginVersions,
  getAdminPlugins,
  installAdminPluginFromMarket,
  pluginLifecycleAction,
  previewAdminPluginMarketInstall,
  previewAdminPluginPackage,
  signalAdminPluginWorkspace,
  testAdminPlugin,
  updateAdminPlugin,
  updateAdminPluginSecrets,
  uploadAdminPluginPackage,
  type AdminPlugin,
  type AdminPluginDiagnostics,
  type AdminPluginHookCatalogGroup,
  type AdminPluginExecution,
  type AdminPluginMarketPreviewRequest,
  type AdminPluginSecretMeta,
  type AdminPluginWorkspaceResponse,
  type AdminPluginWorkspaceSnapshot,
  type AdminPluginVersion,
  type PluginPermissionRequest,
} from '@/lib/api'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'
import { usePermission } from '@/hooks/use-permission'
import { useToast } from '@/hooks/use-toast'
import { resolvePluginOperationErrorMessage } from '@/lib/api-error'
import { clearCachedBootstrapMenus } from '@/lib/plugin-bootstrap-cache'
import { CATEGORY_LABEL_KEYS, PERMISSIONS_BY_CATEGORY } from '@/lib/constants'
import {
  manifestString,
  parseManifestObject,
  parseManifestObjectSchema,
} from '@/lib/package-manifest-schema'
import { PluginDeleteAlert } from '@/components/admin/plugins/plugin-delete-alert'
import { PluginDiagnosticDialog } from '@/components/admin/plugins/plugin-diagnostic-dialog'
import { PluginEditorDialog } from '@/components/admin/plugins/plugin-editor-dialog'
import { PluginExecutionLogsDialog } from '@/components/admin/plugins/plugin-execution-logs-dialog'
import { PluginListPanel } from '@/components/admin/plugins/plugin-list-panel'
import { PluginWorkspaceDialog } from '@/components/admin/plugins/plugin-workspace-dialog'
import { resolvePluginUploadConflictSummary } from '@/components/admin/plugins/plugin-upload-conflicts'
import { PluginUploadDialog } from '@/components/admin/plugins/plugin-upload-dialog'
import { PluginVersionsDialog } from '@/components/admin/plugins/plugin-versions-dialog'
import { PluginSlot } from '@/components/plugins/plugin-slot'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { renderToastMessage } from '@/components/ui/toast-message'
import type { PluginCapabilityPermissionOption } from '@/components/admin/plugins/plugin-capability-policy-editor'
import type {
  PluginFrontendPermissionCatalogGroup,
  PluginFrontendSlotCatalogGroup,
} from '@/components/admin/plugins/plugin-frontend-access-editor'
import type {
  ActivatePayload,
  LifecyclePayload,
  MarketPluginInstallContext,
  PluginCapabilityPolicyState,
  PluginFrontendAccessState,
  PluginJSONSchema,
  PluginHookGroupKey,
  PluginLifecycleActionState,
  PluginForm,
  PluginHookAccessState,
  PluginManifestPreview,
  UploadForm,
  UploadPermissionPreview,
} from '@/components/admin/plugins/types'
import { Card, CardContent } from '@/components/ui/card'

const EMPTY_PLUGIN_FORM: PluginForm = {
  name: '',
  display_name: '',
  description: '',
  type: '',
  runtime: 'grpc',
  package_path: '',
  address: '',
  version: '0.0.0',
  config: '{}',
  runtime_params: '{}',
  capabilities: '{}',
  enabled: false,
}

const EMPTY_UPLOAD_FORM: UploadForm = {
  plugin_id: '',
  name: '',
  display_name: '',
  description: '',
  type: '',
  runtime: 'grpc',
  address: '',
  version: '',
  config: '',
  runtime_params: '{}',
  capabilities: '{}',
  changelog: '',
  activate: true,
  auto_start: false,
}

const RUNTIME_OPTIONS = ['grpc', 'js_worker'] as const

type PluginOperationFeedback = {
  tone: 'success' | 'warning'
  title: string
  summary: string
  detail?: string
  pluginId: number | null
  pluginSnapshot: AdminPlugin | null
  pluginName: string
  version: string
  sourceLabel?: string
  status?: string
  lifecycleStatus?: string
  healthStatus?: string
  warnings: string[]
  occurredAt: string
}

const DEFAULT_PLUGIN_HOOK_GROUPS: Record<PluginHookGroupKey, string[]> = {
  frontend: ['frontend.slot.render', 'frontend.bootstrap'],
  auth: [
    'auth.register.before',
    'auth.register.after',
    'auth.login.before',
    'auth.login.after',
    'auth.password.reset.before',
    'auth.password.reset.after',
  ],
  order: [
    'order.create.before',
    'order.create.after',
    'order.complete.before',
    'order.complete.after',
    'order.admin.complete.before',
    'order.admin.complete.after',
    'order.admin.cancel.before',
    'order.admin.cancel.after',
    'order.admin.refund.before',
    'order.admin.refund.after',
    'order.admin.mark_paid.before',
    'order.admin.mark_paid.after',
    'order.admin.deliver_virtual.before',
    'order.admin.deliver_virtual.after',
    'order.admin.update_shipping.before',
    'order.admin.update_shipping.after',
    'order.admin.update_price.before',
    'order.admin.update_price.after',
    'order.admin.delete.before',
    'order.admin.delete.after',
    'order.auto_cancel.before',
    'order.auto_cancel.after',
  ],
  payment: [
    'payment.method.select.before',
    'payment.method.select.after',
    'payment.confirm.before',
    'payment.confirm.after',
    'payment.polling.succeeded',
    'payment.polling.failed',
  ],
  ticket: [
    'ticket.create.before',
    'ticket.create.after',
    'ticket.message.user.before',
    'ticket.message.user.after',
    'ticket.message.admin.before',
    'ticket.message.admin.after',
    'ticket.status.user.before',
    'ticket.status.user.after',
    'ticket.update.admin.before',
    'ticket.update.admin.after',
    'ticket.assign.before',
    'ticket.assign.after',
    'ticket.attachment.upload.before',
    'ticket.attachment.upload.after',
    'ticket.message.read.user.after',
    'ticket.message.read.admin.after',
    'ticket.order.share.after',
    'ticket.auto_close.before',
    'ticket.auto_close.after',
  ],
  product_inventory: [
    'product.create.before',
    'product.create.after',
    'product.update.before',
    'product.update.after',
    'product.delete.before',
    'product.delete.after',
    'product.status.update.before',
    'product.status.update.after',
    'product.inventory_mode.update.before',
    'product.inventory_mode.update.after',
    'inventory.reserve.before',
    'inventory.reserve.after',
    'inventory.release.after',
  ],
  promo: ['promo.validate.before', 'promo.validate.after'],
}

const DEFAULT_CAPABILITIES_TEMPLATE = `{
  "hooks": [
    "*"
  ],
  "disabled_hooks": [],
  "requested_permissions": [],
  "granted_permissions": [],
  "allow_block": true,
  "allow_payload_patch": true,
  "allow_frontend_extensions": true,
  "frontend_min_scope": "guest",
  "frontend_allowed_areas": [],
  "frontend_required_permissions": [],
  "allowed_frontend_slots": [],
  "allow_execute_api": false,
  "allow_network": false,
  "allow_file_system": false
}`

const DEFAULT_PLUGIN_HOOK_ACCESS_STATE: PluginHookAccessState = {
  allowAllHooks: true,
  selectedHooks: [],
  disabledHooks: [],
}

const DEFAULT_PLUGIN_FRONTEND_ACCESS_STATE: PluginFrontendAccessState = {
  allowFrontendExtensions: true,
  allowAllFrontendAreas: true,
  selectedFrontendAreas: [],
  frontendMinScope: 'guest',
  allowAllFrontendSlots: true,
  selectedFrontendSlots: [],
  frontendRequiredPermissions: [],
}

const DEFAULT_PLUGIN_CAPABILITY_POLICY_STATE: PluginCapabilityPolicyState = {
  requestedPermissions: [],
  grantedPermissions: [],
  allowBlock: true,
  allowPayloadPatch: true,
  allowExecuteApi: false,
  allowNetwork: false,
  allowFileSystem: false,
  trustedHtmlMode: false,
}

type PluginSecretPatch = {
  upserts?: Record<string, string>
  delete_keys?: string[]
}

type SavePluginPayload = {
  pluginId?: number
  data: Partial<AdminPlugin>
  baseline: string | null
  secretPatch: PluginSecretPatch | null
}

type SavePluginResult = {
  pluginId: number | null
  response: unknown
  secretUpdated: boolean
}

const FRONTEND_SLOT_CATALOG_GROUPS = [
  {
    key: 'user',
    slots: [
      'user.cart.top',
      'user.cart.load_failed',
      'user.cart.empty',
      'user.cart.before_checkout',
      'user.cart.checkout.top',
      'user.cart.checkout.promo.after',
      'user.cart.checkout.submit.before',
      'user.cart.confirm_dialog.before',
      'user.cart.bottom',
      'user.products.top',
      'user.products.filters',
      'user.products.load_failed',
      'user.products.empty',
      'user.products.bottom',
      'user.product_detail.top',
      'user.product_detail.load_failed',
      'user.product_detail.not_found',
      'user.product_detail.selection.after',
      'user.product_detail.meta.after',
      'user.product_detail.promo.after',
      'user.product_detail.guest_hint.after',
      'user.product_detail.actions.before',
      'user.product_detail.description.before',
      'user.product_detail.description.after',
      'user.product_detail.buybox.after',
      'user.product_detail.content.after',
      'user.product_detail.bottom',
      'user.announcements.top',
      'user.announcements.load_failed',
      'user.announcements.empty',
      'user.announcements.list.after',
      'user.announcements.pagination.before',
      'user.announcements.pagination.after',
      'user.announcements.bottom',
      'user.announcement_detail.top',
      'user.announcement_detail.load_failed',
      'user.announcement_detail.not_found',
      'user.announcement_detail.meta.after',
      'user.announcement_detail.content.before',
      'user.announcement_detail.content.after',
      'user.announcement_detail.bottom',
      'user.knowledge.top',
      'user.knowledge.load_failed',
      'user.knowledge.empty',
      'user.knowledge.filters.after',
      'user.knowledge.list.after',
      'user.knowledge.pagination.before',
      'user.knowledge.pagination.after',
      'user.knowledge.bottom',
      'user.knowledge_detail.top',
      'user.knowledge_detail.load_failed',
      'user.knowledge_detail.not_found',
      'user.knowledge_detail.meta.after',
      'user.knowledge_detail.content.before',
      'user.knowledge_detail.content.after',
      'user.knowledge_detail.bottom',
      'user.profile.top',
      'user.profile.header.after',
      'user.profile.identity.after',
      'user.profile.stats.after',
      'user.profile.overview.after',
      'user.profile.quick_actions.before',
      'user.profile.quick_actions.after',
      'user.profile.logout.before',
      'user.profile.logout.dialog.before',
      'user.profile.bottom',
      'user.profile.preferences.top',
      'user.profile.preferences.display.after',
      'user.profile.preferences.notifications.top',
      'user.profile.preferences.notifications.save.before',
      'user.profile.preferences.bottom',
      'user.profile.settings.top',
      'user.profile.settings.account_info.after',
      'user.profile.settings.bind_email.after',
      'user.profile.settings.bind_email.unavailable',
      'user.profile.settings.bind_phone.after',
      'user.profile.settings.bind_phone.unavailable',
      'user.profile.settings.password.top',
      'user.profile.settings.password.submit.before',
      'user.profile.settings.bottom',
      'user.layout.content.top',
      'user.layout.content.bottom',
      'user.layout.guest_access_check_failed',
      'user.layout.login_required',
      'user.layout.sidebar.top',
      'user.layout.sidebar.menu.after',
      'user.layout.sidebar.runtime_menu.after',
      'user.layout.sidebar.bottom',
      'user.layout.sidebar.guest_actions.before',
      'user.layout.sidebar.authed_actions.before',
      'user.layout.mobile_nav.top',
      'user.layout.mobile_nav.more.before',
      'user.layout.mobile_nav.more.after',
      'user.layout.mobile_nav.bottom',
      'user.layout.announcement_popup.top',
      'user.layout.announcement_popup.load_failed',
      'user.layout.announcement_popup.list.after',
      'user.layout.announcement_popup.actions.before',
      'user.layout.announcement_popup.content.before',
      'user.layout.announcement_popup.bottom',
      'user.order_detail.top',
      'user.order_detail.load_failed',
      'user.order_detail.not_found',
      'user.order_detail.payment.top',
      'user.order_detail.payment.load_failed',
      'user.order_detail.payment.selected.after',
      'user.order_detail.payment.methods.after',
      'user.order_detail.payment.bottom',
      'user.order_detail.bottom',
      'user.order_detail.info.after',
      'user.order_detail.products.after',
      'user.order_detail.virtual_stocks.after',
      'user.order_detail.shipping.receiver.after',
      'user.order_detail.shipping.tracking.after',
      'user.order_detail.shipping.form.after',
      'user.order_detail.shipping.empty',
      'user.order_detail.remark.after',
      'user.order_detail.serials.after',
      'user.orders.load_failed',
      'user.orders.empty',
      'user.orders.filters',
      'user.orders.list.empty',
      'user.orders.list.grid.after',
      'user.orders.list.load_more.after',
      'user.orders.list.pagination.before',
      'user.orders.list.pagination.after',
      'user.orders.list.card.badges.after',
      'user.orders.list.card.product.after',
      'user.orders.list.card.summary.after',
      'user.orders.list.card.actions.before',
      'user.orders.list.card.actions.after',
      'user.order_detail.info_actions',
      'user.order_detail.product_actions',
      'user.order_detail.virtual_stock_actions',
      'user.order_detail.shipping_actions',
      'user.order_detail.serials_actions',
      'user.ticket_detail.top',
      'user.ticket_detail.load_failed',
      'user.ticket_detail.not_found',
      'user.ticket_detail.meta.after',
      'user.ticket_detail.message_actions',
      'user.ticket_detail.messages_load_failed',
      'user.ticket_detail.empty',
      'user.ticket_detail.composer.top',
      'user.ticket_detail.composer.toolbar.after',
      'user.ticket_detail.composer.preview.after',
      'user.ticket_detail.composer.editor.after',
      'user.ticket_detail.composer.bottom',
      'user.orders.top',
      'user.orders.before_list',
      'user.orders.bottom',
      'user.tickets.top',
      'user.tickets.disabled',
      'user.tickets.load_failed',
      'user.tickets.empty',
      'user.tickets.filters.after',
      'user.tickets.before_list',
      'user.tickets.list.after',
      'user.tickets.create.top',
      'user.tickets.create.related_order.after',
      'user.tickets.create.submit.before',
      'user.tickets.create.bottom',
      'user.tickets.pagination.before',
      'user.tickets.pagination.after',
      'user.tickets.bottom',
      'user.plugin_page.top',
      'user.plugin_page.bottom',
    ],
  },
  {
    key: 'admin',
    slots: [
      'admin.dashboard.top',
      'admin.dashboard.bottom',
      'admin.users.top',
      'admin.users.filters',
      'admin.users.row_actions',
      'admin.user_detail.top',
      'admin.products.top',
      'admin.products.filters',
      'admin.products.row_actions',
      'admin.product_detail.top',
      'admin.product_virtual_stock.top',
      'admin.inventories.top',
      'admin.inventory_new.top',
      'admin.inventory_detail.top',
      'admin.virtual_inventory_detail.top',
      'admin.payment_methods.top',
      'admin.payment_methods.row_actions',
      'admin.payment_methods.editor.top',
      'admin.payment_methods.import.top',
      'admin.payment_methods.market.top',
      'admin.payment_methods.market.filters',
      'admin.settings.top',
      'admin.settings.sections.after_basic',
      'admin.marketing.top',
      'admin.marketing.recipient_row_actions',
      'admin.marketing.batch_row_actions',
      'admin.promo_codes.top',
      'admin.promo_codes.filters',
      'admin.promo_codes.row_actions',
      'admin.promo_code_detail.top',
      'admin.promo_code_new.top',
      'admin.api_keys.top',
      'admin.api_keys.row_actions',
      'admin.api_keys.create.top',
      'admin.analytics.top',
      'admin.layout.content.top',
      'admin.layout.content.bottom',
      'admin.layout.sidebar.top',
      'admin.layout.sidebar.bottom',
      'admin.plugins.top',
      'admin.plugins.observability.top',
      'admin.plugins.version_actions',
      'admin.announcements.top',
      'admin.announcements.load_failed',
      'admin.announcements.empty',
      'admin.announcements.editor.empty',
      'admin.announcements.editor.meta.after',
      'admin.announcements.row_actions',
      'admin.announcement_detail.top',
      'admin.announcement_detail.load_failed',
      'admin.announcement_detail.not_found',
      'admin.announcement_detail.form.top',
      'admin.announcement_detail.submit.before',
      'admin.announcement_new.top',
      'admin.knowledge.top',
      'admin.knowledge.categories.load_failed',
      'admin.knowledge.categories.empty',
      'admin.knowledge.articles.load_failed',
      'admin.knowledge.articles.empty',
      'admin.knowledge.editor.empty',
      'admin.knowledge.editor.meta.after',
      'admin.knowledge.row_actions',
      'admin.knowledge_article_detail.top',
      'admin.knowledge_article_detail.load_failed',
      'admin.knowledge_article_detail.not_found',
      'admin.knowledge_article_detail.form.top',
      'admin.knowledge_article_detail.submit.before',
      'admin.knowledge_article_new.top',
      'admin.logs.top',
      'admin.orders.top',
      'admin.orders.actions',
      'admin.orders.before_table',
      'admin.orders.batch_actions',
      'admin.orders.row_actions',
      'admin.orders.bottom',
      'admin.order_detail.top',
      'admin.order_detail.actions',
      'admin.order_detail.info_actions',
      'admin.order_detail.product_actions',
      'admin.order_detail.virtual_stock_actions',
      'admin.order_detail.shipping_actions',
      'admin.order_detail.serials_actions',
      'admin.order_detail.info.after',
      'admin.order_detail.products.after',
      'admin.order_detail.virtual_stocks.after',
      'admin.order_detail.shipping.receiver.after',
      'admin.order_detail.shipping.tracking.after',
      'admin.order_detail.shipping.form.after',
      'admin.order_detail.shipping.empty',
      'admin.order_detail.remark.after',
      'admin.order_detail.serials.after',
      'admin.order_detail.bottom',
      'admin.tickets.top',
      'admin.tickets.row_actions',
      'admin.tickets.before_list',
      'admin.ticket_detail.top',
      'admin.ticket_detail.message_actions',
      'admin.ticket_detail.messages_load_failed',
      'admin.ticket_detail.empty',
      'admin.ticket_detail.composer.top',
      'admin.ticket_detail.composer.toolbar.after',
      'admin.ticket_detail.composer.preview.after',
      'admin.ticket_detail.composer.editor.after',
      'admin.ticket_detail.composer.bottom',
      'admin.tickets.bottom',
      'admin.serials.top',
      'admin.plugin_page.top',
      'admin.plugin_page.bottom',
    ],
  },
  {
    key: 'auth',
    slots: [
      'auth.login.top',
      'auth.login.return_hint.after',
      'auth.login.methods.after',
      'auth.login.password.submit.before',
      'auth.login.password.form.after',
      'auth.login.code.alert.after',
      'auth.login.code.request.after',
      'auth.login.code.form.after',
      'auth.login.phone.alert.after',
      'auth.login.phone.request.after',
      'auth.login.phone.form.after',
      'auth.login.bottom',
      'auth.login.footer.before',
      'auth.register.top',
      'auth.register.mode.after',
      'auth.register.phone.code.after',
      'auth.register.phone.submit.before',
      'auth.register.phone.form.after',
      'auth.register.email.submit.before',
      'auth.register.email.form.after',
      'auth.register.bottom',
      'auth.register.footer.before',
      'auth.forgot_password.top',
      'auth.forgot_password.mode.after',
      'auth.forgot_password.email.submit.before',
      'auth.forgot_password.email.alert.after',
      'auth.forgot_password.email.form.after',
      'auth.forgot_password.phone.code.after',
      'auth.forgot_password.phone.alert.after',
      'auth.forgot_password.phone.submit.before',
      'auth.forgot_password.phone.form.after',
      'auth.forgot_password.bottom',
      'auth.forgot_password.footer.before',
      'auth.reset_password.top',
      'auth.reset_password.form.top',
      'auth.reset_password.submit.before',
      'auth.reset_password.form.after',
      'auth.reset_password.success.after',
      'auth.reset_password.invalid_token.after',
      'auth.reset_password.bottom',
      'auth.reset_password.footer.before',
      'auth.verify_email.top',
      'auth.verify_email.status.after',
      'auth.verify_email.summary.after',
      'auth.verify_email.actions.before',
      'auth.verify_email.actions.after',
      'auth.verify_email.bottom',
      'auth.verify_email.footer.before',
      'auth.layout.branding.top',
      'auth.layout.branding.bottom',
    ],
  },
  {
    key: 'public',
    slots: [
      'public.shipping_form.top',
      'public.shipping_form.load_failed',
      'public.shipping_form.form.top',
      'public.shipping_form.fields.after',
      'public.shipping_form.submit.before',
      'public.shipping_form.form.bottom',
      'public.shipping_form.order_items.after',
      'public.shipping_form.bottom',
      'public.serial_verify.top',
      'public.serial_verify.form.after',
      'public.serial_verify.error',
      'public.serial_verify.idle',
      'public.serial_verify.result.summary.after',
      'public.serial_verify.result.product.after',
      'public.serial_verify.result.security.after',
      'public.serial_verify.result.history.after',
      'public.serial_verify.bottom',
    ],
  },
] as const

function formatDateTime(value?: string, locale?: string): string {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString(locale === 'zh' ? 'zh-CN' : 'en-US', { hour12: false })
}

function extractArray<T>(data: unknown): T[] {
  if (Array.isArray(data)) return data as T[]
  if (data && typeof data === 'object') {
    const target = data as { data?: unknown }
    if (Array.isArray(target.data)) return target.data as T[]
  }
  return []
}

function extractObject<T extends object>(data: unknown): T | null {
  if (!data || typeof data !== 'object' || Array.isArray(data)) return null
  const target = data as { data?: unknown }
  if (target.data && typeof target.data === 'object' && !Array.isArray(target.data)) {
    return target.data as T
  }
  return data as T
}

function parseObject(value: string): Record<string, unknown> {
  const trimmed = value.trim()
  if (!trimmed) return {}
  const parsed = JSON.parse(trimmed)
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new Error('JSON must be object')
  }
  return parsed as Record<string, unknown>
}

function normalizeStringList(values: string[]): string[] {
  const seen = new Set<string>()
  const out: string[] = []
  values.forEach((item) => {
    const normalized = (item || '').trim().toLowerCase()
    if (!normalized || seen.has(normalized)) return
    seen.add(normalized)
    out.push(normalized)
  })
  return out
}

function normalizeUniqueTextList(value: unknown): string[] {
  if (!Array.isArray(value)) return []
  const seen = new Set<string>()
  const out: string[] = []
  value.forEach((item) => {
    if (typeof item !== 'string') return
    const normalized = item.trim()
    if (!normalized) return
    const key = normalized.toLowerCase()
    if (seen.has(key)) return
    seen.add(key)
    out.push(normalized)
  })
  return out
}

function extractNumberValue(value: unknown): number | null {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string' && value.trim()) {
    const parsed = Number(value)
    if (Number.isFinite(parsed)) return parsed
  }
  return null
}

function extractNumberField(
  target: Record<string, unknown> | null | undefined,
  keys: string[]
): number | null {
  if (!target) return null
  for (const key of keys) {
    const parsed = extractNumberValue(target[key])
    if (parsed !== null) return parsed
  }
  return null
}

function extractTextField(
  target: Record<string, unknown> | null | undefined,
  keys: string[]
): string {
  if (!target) return ''
  for (const key of keys) {
    const candidate = target[key]
    if (typeof candidate === 'string' && candidate.trim()) {
      return candidate.trim()
    }
  }
  return ''
}

function normalizeHookList(value: unknown): string[] {
  if (!Array.isArray(value)) return []
  return normalizeStringList(value.map((item) => String(item || '')))
}

function normalizeFrontendAreaList(value: unknown): string[] {
  if (!Array.isArray(value)) return []
  return normalizeStringList(
    value
      .map((item) => String(item || ''))
      .filter((item) =>
        ['user', 'admin', '*'].includes(
          String(item || '')
            .trim()
            .toLowerCase()
        )
      )
  )
}

function normalizePermissionList(value: unknown): string[] {
  if (!Array.isArray(value)) return []
  return normalizeStringList(value.map((item) => String(item || '')))
}

function manifestPrefersAutoStart(manifest: PluginManifestPreview | null): boolean {
  if (!manifest || !hasOwnKey(manifest, 'capabilities')) return false
  const capabilities = parseManifestObject(manifest.capabilities)
  if (!capabilities) return false

  const hooks = normalizeHookList(capabilities.hooks)
  if (
    hooks.includes('*') ||
    hooks.includes('frontend.bootstrap') ||
    hooks.includes('frontend.slot.render')
  ) {
    return true
  }

  const allowedAreas = normalizeFrontendAreaList(capabilities.frontend_allowed_areas)
  const allowedSlots = normalizeHookList(capabilities.allowed_frontend_slots)
  if (
    capabilities.allow_frontend_extensions === true &&
    (allowedAreas.length > 0 || allowedSlots.length > 0)
  ) {
    return true
  }

  return false
}

const HUMAN_READABLE_CAPABILITY_KEY_ORDER = [
  'hooks',
  'disabled_hooks',
  'requested_permissions',
  'granted_permissions',
  'allow_block',
  'allow_payload_patch',
  'allow_frontend_extensions',
  'frontend_min_scope',
  'frontend_allowed_areas',
  'frontend_required_permissions',
  'allowed_frontend_slots',
  'allow_execute_api',
  'allow_network',
  'allow_file_system',
  'frontend_html_mode',
]

function orderJSONObjectKeys(source: Record<string, unknown>): Record<string, unknown> {
  const out: Record<string, unknown> = {}
  Object.keys(source)
    .sort((a, b) => a.localeCompare(b))
    .forEach((key) => {
      out[key] = source[key]
    })
  return out
}

function normalizeJSONValueForDisplay(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map((item) => normalizeJSONValueForDisplay(item))
  }
  if (value && typeof value === 'object') {
    return orderJSONObjectKeys(
      Object.fromEntries(
        Object.entries(value as Record<string, unknown>).map(([key, item]) => [
          key,
          normalizeJSONValueForDisplay(item),
        ])
      )
    )
  }
  return value
}

function formatHumanReadableJSONText(value: string, fallback = '{}'): string {
  const trimmed = value.trim()
  if (!trimmed) return fallback
  const parsed = parseObject(trimmed)
  return JSON.stringify(normalizeJSONValueForDisplay(parsed), null, 2)
}

function formatCapabilitiesForDisplay(value: string, fallback = '{}'): string {
  const trimmed = value.trim()
  if (!trimmed) return fallback
  const parsed = parseObject(trimmed)
  const normalized: Record<string, unknown> = {
    ...parsed,
    hooks: normalizeHookList(parsed.hooks),
    disabled_hooks: normalizeHookList(parsed.disabled_hooks),
    requested_permissions: normalizePermissionList(parsed.requested_permissions),
    granted_permissions: normalizePermissionList(parsed.granted_permissions),
    frontend_allowed_areas: normalizeFrontendAreaList(parsed.frontend_allowed_areas),
    frontend_required_permissions: normalizePermissionList(parsed.frontend_required_permissions),
    allowed_frontend_slots: normalizeHookList(parsed.allowed_frontend_slots),
  }

  const out: Record<string, unknown> = {}
  HUMAN_READABLE_CAPABILITY_KEY_ORDER.forEach((key) => {
    if (Object.prototype.hasOwnProperty.call(normalized, key)) {
      out[key] = normalizeJSONValueForDisplay(normalized[key])
    }
  })
  Object.keys(normalized)
    .filter((key) => !HUMAN_READABLE_CAPABILITY_KEY_ORDER.includes(key))
    .sort((a, b) => a.localeCompare(b))
    .forEach((key) => {
      out[key] = normalizeJSONValueForDisplay(normalized[key])
    })

  return JSON.stringify(out, null, 2)
}

function normalizeFrontendMinScopeValue(value: unknown): 'guest' | 'authenticated' | 'super_admin' {
  const normalized = String(value || '')
    .trim()
    .toLowerCase()
  switch (normalized) {
    case 'authenticated':
    case 'auth':
    case 'user':
    case 'member':
      return 'authenticated'
    case 'super_admin':
    case 'superadmin':
    case 'root':
      return 'super_admin'
    default:
      return 'guest'
  }
}

function isTrustedHTMLModeEnabled(value: unknown): boolean {
  return (
    String(value || '')
      .trim()
      .toLowerCase() === 'trusted'
  )
}

function buildFallbackPluginHookCatalogGroups(): AdminPluginHookCatalogGroup[] {
  return (Object.entries(DEFAULT_PLUGIN_HOOK_GROUPS) as [PluginHookGroupKey, string[]][]).map(
    ([key, hooks]) => ({
      key,
      hooks: normalizeHookList(hooks),
    })
  )
}

function normalizePluginHookCatalogGroups(value: unknown): AdminPluginHookCatalogGroup[] {
  const root = asObject(value)
  const rawGroups = root?.groups
  if (!Array.isArray(rawGroups)) {
    return buildFallbackPluginHookCatalogGroups()
  }

  const groups = rawGroups
    .map((item) => {
      const group = asObject(item)
      if (!group) return null
      const key = String(group.key || '')
        .trim()
        .toLowerCase()
      const hooks = normalizeHookList(group.hooks)
      if (!key || hooks.length === 0) return null
      return {
        key,
        hooks,
      } satisfies AdminPluginHookCatalogGroup
    })
    .filter((item): item is AdminPluginHookCatalogGroup => !!item)

  return groups.length > 0 ? groups : buildFallbackPluginHookCatalogGroups()
}

function isJSONObjectText(value: string): boolean {
  try {
    parseObject(value)
    return true
  } catch {
    return false
  }
}

function isBlankOrJSONObjectText(value: string): boolean {
  return value.trim() === '' || isJSONObjectText(value)
}

function parsePluginHookAccessState(capabilitiesText: string): PluginHookAccessState {
  try {
    const capabilities = parseObject(capabilitiesText)
    const hooks = normalizeHookList(capabilities.hooks)
    const disabledHooks = normalizeHookList(capabilities.disabled_hooks)
    const denyAllHooks = disabledHooks.includes('*')
    const allowAllHooks = !denyAllHooks && (hooks.length === 0 || hooks.includes('*'))

    return {
      allowAllHooks,
      selectedHooks: hooks.filter((item) => item !== '*'),
      disabledHooks: allowAllHooks ? disabledHooks.filter((item) => item !== '*') : [],
    }
  } catch {
    return { ...DEFAULT_PLUGIN_HOOK_ACCESS_STATE }
  }
}

function parsePluginCapabilityPolicyState(capabilitiesText: string): PluginCapabilityPolicyState {
  try {
    const capabilities = parseObject(capabilitiesText)
    return {
      requestedPermissions: normalizePermissionList(capabilities.requested_permissions),
      grantedPermissions: normalizePermissionList(capabilities.granted_permissions),
      allowBlock: typeof capabilities.allow_block === 'boolean' ? capabilities.allow_block : true,
      allowPayloadPatch:
        typeof capabilities.allow_payload_patch === 'boolean'
          ? capabilities.allow_payload_patch
          : true,
      allowExecuteApi:
        typeof capabilities.allow_execute_api === 'boolean' ? capabilities.allow_execute_api : true,
      allowNetwork:
        typeof capabilities.allow_network === 'boolean' ? capabilities.allow_network : true,
      allowFileSystem:
        typeof capabilities.allow_file_system === 'boolean' ? capabilities.allow_file_system : true,
      trustedHtmlMode: isTrustedHTMLModeEnabled(
        capabilities.frontend_html_mode ?? capabilities.html_mode
      ),
    }
  } catch {
    return { ...DEFAULT_PLUGIN_CAPABILITY_POLICY_STATE }
  }
}

function parsePluginFrontendAccessState(capabilitiesText: string): PluginFrontendAccessState {
  try {
    const capabilities = parseObject(capabilitiesText)
    const frontendAllowedAreas = normalizeFrontendAreaList(capabilities.frontend_allowed_areas)
    const allowedFrontendSlots = normalizeHookList(capabilities.allowed_frontend_slots)
    const allowAllFrontendAreas =
      frontendAllowedAreas.length === 0 || frontendAllowedAreas.includes('*')
    const allowAllFrontendSlots =
      allowedFrontendSlots.length === 0 || allowedFrontendSlots.includes('*')

    return {
      allowFrontendExtensions:
        typeof capabilities.allow_frontend_extensions === 'boolean'
          ? capabilities.allow_frontend_extensions
          : true,
      allowAllFrontendAreas,
      selectedFrontendAreas: allowAllFrontendAreas
        ? frontendAllowedAreas.filter((item) => item !== '*')
        : frontendAllowedAreas,
      frontendMinScope: normalizeFrontendMinScopeValue(capabilities.frontend_min_scope),
      allowAllFrontendSlots,
      selectedFrontendSlots: allowAllFrontendSlots
        ? allowedFrontendSlots.filter((item) => item !== '*')
        : allowedFrontendSlots,
      frontendRequiredPermissions: normalizePermissionList(
        capabilities.frontend_required_permissions
      ),
    }
  } catch {
    return { ...DEFAULT_PLUGIN_FRONTEND_ACCESS_STATE }
  }
}

function mergePluginHookAccessState(
  capabilitiesText: string,
  hookAccessState: PluginHookAccessState
): string {
  const capabilities = parseObject(capabilitiesText)
  const nextCapabilities: Record<string, unknown> = { ...capabilities }

  if (hookAccessState.allowAllHooks) {
    nextCapabilities.hooks = ['*']
    nextCapabilities.disabled_hooks = normalizeHookList(hookAccessState.disabledHooks)
    return formatCapabilitiesForDisplay(JSON.stringify(nextCapabilities), '{}')
  }

  const selectedHooks = normalizeHookList(hookAccessState.selectedHooks)
  if (selectedHooks.length === 0) {
    nextCapabilities.hooks = ['*']
    nextCapabilities.disabled_hooks = ['*']
    return formatCapabilitiesForDisplay(JSON.stringify(nextCapabilities), '{}')
  }

  nextCapabilities.hooks = selectedHooks
  nextCapabilities.disabled_hooks = []
  return formatCapabilitiesForDisplay(JSON.stringify(nextCapabilities), '{}')
}

function mergePluginCapabilityPolicyState(
  capabilitiesText: string,
  capabilityPolicyState: PluginCapabilityPolicyState
): string {
  const capabilities = parseObject(capabilitiesText)
  const nextCapabilities: Record<string, unknown> = { ...capabilities }

  nextCapabilities.requested_permissions = normalizePermissionList(
    capabilityPolicyState.requestedPermissions
  )
  nextCapabilities.granted_permissions = normalizePermissionList(
    capabilityPolicyState.grantedPermissions
  )
  nextCapabilities.allow_block = capabilityPolicyState.allowBlock
  nextCapabilities.allow_payload_patch = capabilityPolicyState.allowPayloadPatch
  nextCapabilities.allow_execute_api = capabilityPolicyState.allowExecuteApi
  nextCapabilities.allow_network = capabilityPolicyState.allowNetwork
  nextCapabilities.allow_file_system = capabilityPolicyState.allowFileSystem
  if (capabilityPolicyState.trustedHtmlMode) {
    nextCapabilities.frontend_html_mode = 'trusted'
  } else {
    delete nextCapabilities.frontend_html_mode
    delete nextCapabilities.html_mode
  }

  return formatCapabilitiesForDisplay(JSON.stringify(nextCapabilities), '{}')
}

function mergePluginFrontendAccessState(
  capabilitiesText: string,
  frontendAccessState: PluginFrontendAccessState
): string {
  const capabilities = parseObject(capabilitiesText)
  const nextCapabilities: Record<string, unknown> = { ...capabilities }

  nextCapabilities.allow_frontend_extensions = frontendAccessState.allowFrontendExtensions
  nextCapabilities.frontend_min_scope = normalizeFrontendMinScopeValue(
    frontendAccessState.frontendMinScope
  )
  nextCapabilities.frontend_allowed_areas = frontendAccessState.allowAllFrontendAreas
    ? []
    : normalizeFrontendAreaList(frontendAccessState.selectedFrontendAreas)
  nextCapabilities.allowed_frontend_slots = frontendAccessState.allowAllFrontendSlots
    ? []
    : normalizeHookList(frontendAccessState.selectedFrontendSlots)
  nextCapabilities.frontend_required_permissions = normalizePermissionList(
    frontendAccessState.frontendRequiredPermissions
  )

  return formatCapabilitiesForDisplay(JSON.stringify(nextCapabilities), '{}')
}

function validatePluginFrontendAccessState(
  frontendAccessState: PluginFrontendAccessState,
  t: ReturnType<typeof getTranslations>
): string | null {
  if (!frontendAccessState.allowFrontendExtensions) {
    return null
  }
  if (
    !frontendAccessState.allowAllFrontendAreas &&
    normalizeFrontendAreaList(frontendAccessState.selectedFrontendAreas).length === 0
  ) {
    return t.admin.pluginFrontendAreaRequired
  }
  if (
    !frontendAccessState.allowAllFrontendSlots &&
    normalizeHookList(frontendAccessState.selectedFrontendSlots).length === 0
  ) {
    return t.admin.pluginFrontendSlotRequired
  }
  return null
}

function buildFrontendSlotCatalog(extraSlots: string[]): PluginFrontendSlotCatalogGroup[] {
  const baseGroups = FRONTEND_SLOT_CATALOG_GROUPS.map((group) => ({
    key: group.key,
    slots: normalizeStringList([...group.slots]),
  }))
  const knownSlots = new Set(baseGroups.flatMap((group) => group.slots))
  const unknownSlots = normalizeStringList(extraSlots).filter((slot) => !knownSlots.has(slot))
  if (unknownSlots.length === 0) {
    return baseGroups
  }
  return [
    ...baseGroups,
    {
      key: 'other',
      slots: unknownSlots,
    },
  ]
}

function buildFrontendPermissionCatalog(
  extraPermissions: string[],
  t: ReturnType<typeof getTranslations>
): PluginFrontendPermissionCatalogGroup[] {
  const adminText = t.admin as unknown as Record<string, unknown>
  const readAdminText = (key: string, fallback: string): string => {
    const value = adminText[key]
    return typeof value === 'string' && value.trim() !== '' ? value : fallback
  }
  const groups = Object.entries(PERMISSIONS_BY_CATEGORY)
    .map(([category, permissions]) => {
      const categoryLabelKey = CATEGORY_LABEL_KEYS[category]
      const label = categoryLabelKey ? readAdminText(categoryLabelKey, category) : category
      return {
        key: category,
        label,
        permissions: permissions.map((permission) => ({
          value: permission.value,
          label: readAdminText(permission.labelKey, permission.value),
        })),
      }
    })
    .filter((group) => group.permissions.length > 0)

  const knownPermissions = new Set(
    groups.flatMap((group) => group.permissions.map((item) => item.value))
  )
  const unknownPermissions = normalizePermissionList(extraPermissions).filter(
    (permission) => !knownPermissions.has(permission)
  )
  if (unknownPermissions.length > 0) {
    groups.push({
      key: 'other',
      label: t.admin.pluginFrontendPermissionGroupOther,
      permissions: unknownPermissions.map((permission) => ({
        value: permission,
        label: permission,
      })),
    })
  }
  return groups
}

function hasOwnKey(obj: unknown, key: string): boolean {
  if (!obj || typeof obj !== 'object') return false
  return Object.prototype.hasOwnProperty.call(obj, key)
}

function asObject(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null
  return value as Record<string, unknown>
}

function stringifyJSONForTextarea(
  value: unknown,
  fallback: string,
  formatter?: (value: string, fallback: string) => string
): string {
  const obj = asObject(value)
  if (obj) {
    try {
      const raw = JSON.stringify(obj)
      return formatter ? formatter(raw, fallback) : JSON.stringify(obj, null, 2)
    } catch {
      return fallback
    }
  }
  if (typeof value === 'string') {
    const trimmed = value.trim()
    if (!trimmed) return fallback
    try {
      const parsed = JSON.parse(trimmed)
      const parsedObj = asObject(parsed)
      if (parsedObj) {
        const raw = JSON.stringify(parsedObj)
        return formatter ? formatter(raw, fallback) : JSON.stringify(parsedObj, null, 2)
      }
    } catch {
      // keep raw string
    }
    return trimmed
  }
  return fallback
}

function tryFormatTextareaJSON(
  value: string,
  fallback: string,
  formatter: (value: string, fallback: string) => string
): string {
  const trimmed = value.trim()
  if (!trimmed) return fallback
  try {
    return formatter(trimmed, fallback)
  } catch {
    return value
  }
}

function buildManifestSchemaDefaults(schema: PluginJSONSchema | null): Record<string, unknown> {
  if (!schema || schema.fields.length === 0) return {}
  const defaults: Record<string, unknown> = {}
  schema.fields.forEach((field) => {
    if (field.defaultValue !== undefined) {
      defaults[field.key] = field.defaultValue
    }
  })
  return defaults
}

function buildPluginSaveRequestData(form: PluginForm): Partial<AdminPlugin> {
  const runtime = form.runtime.trim()
  return {
    name: form.name.trim(),
    display_name: form.display_name.trim(),
    description: form.description.trim(),
    type: form.type.trim(),
    runtime,
    package_path: isJSWorkerRuntime(runtime) ? form.package_path.trim() : undefined,
    address: form.address.trim(),
    version: form.version.trim() || '0.0.0',
    config: JSON.stringify(parseObject(form.config)),
    runtime_params: JSON.stringify(parseObject(form.runtime_params)),
    capabilities: JSON.stringify(parseObject(form.capabilities)),
    enabled: form.enabled,
  }
}

function serializePluginSaveRequestData(data: Partial<AdminPlugin>): string {
  return JSON.stringify({
    name: data.name || '',
    display_name: data.display_name || '',
    description: data.description || '',
    type: data.type || '',
    runtime: data.runtime || '',
    package_path: data.package_path || '',
    address: data.address || '',
    version: data.version || '',
    config: data.config || '{}',
    runtime_params: data.runtime_params || '{}',
    capabilities: data.capabilities || '{}',
    enabled: data.enabled === true,
  })
}

function normalizePluginSecretKeyList(values: string[]): string[] {
  const seen = new Set<string>()
  const out: string[] = []
  values.forEach((value) => {
    const normalized = value.trim()
    if (!normalized || seen.has(normalized)) return
    seen.add(normalized)
    out.push(normalized)
  })
  return out
}

function buildPluginSecretPatch(
  drafts: Record<string, string>,
  deleteKeys: string[]
): { upserts?: Record<string, string>; delete_keys?: string[] } | null {
  const deleteKeySet = new Set(normalizePluginSecretKeyList(deleteKeys))
  const upserts: Record<string, string> = {}
  Object.entries(drafts).forEach(([key, value]) => {
    const normalizedKey = key.trim()
    if (!normalizedKey || deleteKeySet.has(normalizedKey)) return
    if (value.trim() === '') return
    upserts[normalizedKey] = value
  })
  const normalizedDeleteKeys = [...deleteKeySet].filter(
    (key) => !Object.prototype.hasOwnProperty.call(upserts, key)
  )
  if (Object.keys(upserts).length === 0 && normalizedDeleteKeys.length === 0) {
    return null
  }
  return {
    upserts: Object.keys(upserts).length > 0 ? upserts : undefined,
    delete_keys: normalizedDeleteKeys.length > 0 ? normalizedDeleteKeys : undefined,
  }
}

function buildPluginSecretMetaMap(
  items: AdminPluginSecretMeta[]
): Record<string, AdminPluginSecretMeta> {
  const out: Record<string, AdminPluginSecretMeta> = {}
  items.forEach((item) => {
    const key = String(item?.key || '').trim()
    if (!key) return
    out[key] = item
  })
  return out
}

function extractCreatedPluginID(value: unknown): number | null {
  const root = extractObject<Record<string, unknown>>(value)
  if (!root) return null
  const candidate = root.id
  if (typeof candidate === 'number' && Number.isFinite(candidate)) return candidate
  if (typeof candidate === 'string' && candidate.trim()) {
    const parsed = Number(candidate)
    if (Number.isFinite(parsed)) return parsed
  }
  return null
}

function mergeManifestObjectWithSchemaDefaults(
  manifest: PluginManifestPreview | null,
  valueKey: string,
  schemaKey: string,
  locale?: string
): Record<string, unknown> | null {
  const schema = parseManifestObjectSchema(manifest ? manifest[schemaKey] : null, locale)
  const defaults = buildManifestSchemaDefaults(schema)
  const rawValue = parseManifestObject(manifest ? manifest[valueKey] : null) || {}
  if (Object.keys(defaults).length === 0 && Object.keys(rawValue).length === 0) {
    return null
  }
  return {
    ...defaults,
    ...rawValue,
  }
}

function parsePluginManifestText(value?: string): PluginManifestPreview | null {
  if (!value || !value.trim()) return null
  try {
    const parsed = JSON.parse(value)
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed)
      ? (parsed as PluginManifestPreview)
      : null
  } catch {
    return null
  }
}

function resolvePluginDisplayMetadata(
  plugin:
    | Pick<AdminPlugin, 'name' | 'display_name' | 'description' | 'manifest'>
    | null
    | undefined,
  locale?: string
): { displayName: string; description: string } {
  const manifest = parsePluginManifestText(plugin?.manifest)
  return {
    displayName:
      manifestString(manifest, 'display_name', locale) ||
      String(plugin?.display_name || '').trim() ||
      manifestString(manifest, 'name', locale) ||
      String(plugin?.name || '').trim(),
    description:
      manifestString(manifest, 'description', locale) || String(plugin?.description || '').trim(),
  }
}

function normalizeRuntimeFromManifest(runtimeRaw: string, fallback: string): string {
  const normalized = runtimeRaw.trim().toLowerCase()
  if (normalized === 'grpc' || normalized === 'js_worker') {
    return normalized
  }
  return fallback
}

function toBoolean(value: unknown, fallback: boolean): boolean {
  if (typeof value === 'boolean') return value
  if (typeof value === 'string') {
    const normalized = value.trim().toLowerCase()
    if (['1', 'true', 'yes', 'on'].includes(normalized)) return true
    if (['0', 'false', 'no', 'off'].includes(normalized)) return false
  }
  if (typeof value === 'number') return value !== 0
  return fallback
}

function mergeUploadFormWithManifest(
  prev: UploadForm,
  manifest: PluginManifestPreview | null,
  locale?: string
): UploadForm {
  if (!manifest) return prev

  const name = manifestString(manifest, 'name', locale)
  const displayName = manifestString(manifest, 'display_name', locale)
  const description = manifestString(manifest, 'description', locale)
  const type = manifestString(manifest, 'type', locale)
  const runtime = normalizeRuntimeFromManifest(
    manifestString(manifest, 'runtime', locale),
    prev.runtime
  )
  const address =
    manifestString(manifest, 'address', locale) || manifestString(manifest, 'entry', locale)
  const version = manifestString(manifest, 'version', locale)
  const changelog = manifestString(manifest, 'changelog', locale)

  const next: UploadForm = {
    ...prev,
    name: name || prev.name,
    display_name: displayName || prev.display_name,
    description: description || prev.description,
    type: type || prev.type,
    runtime: runtime || prev.runtime,
    address: address || prev.address,
    version: version || prev.version,
    changelog: changelog || prev.changelog,
  }

  const manifestConfigValue = mergeManifestObjectWithSchemaDefaults(
    manifest,
    'config',
    'config_schema',
    locale
  )
  if (
    manifestConfigValue ||
    hasOwnKey(manifest, 'config') ||
    hasOwnKey(manifest, 'config_schema')
  ) {
    next.config = stringifyJSONForTextarea(
      manifestConfigValue || manifest.config,
      '{}',
      formatHumanReadableJSONText
    )
  }
  const manifestRuntimeParamsValue = mergeManifestObjectWithSchemaDefaults(
    manifest,
    'runtime_params',
    'runtime_params_schema',
    locale
  )
  if (
    manifestRuntimeParamsValue ||
    hasOwnKey(manifest, 'runtime_params') ||
    hasOwnKey(manifest, 'runtime_params_schema')
  ) {
    next.runtime_params = stringifyJSONForTextarea(
      manifestRuntimeParamsValue || manifest.runtime_params,
      '{}',
      formatHumanReadableJSONText
    )
  }
  if (hasOwnKey(manifest, 'capabilities')) {
    next.capabilities = stringifyJSONForTextarea(
      manifest.capabilities,
      '{}',
      formatCapabilitiesForDisplay
    )
  }
  if (hasOwnKey(manifest, 'activate')) {
    next.activate = toBoolean(manifest.activate, next.activate)
  }
  if (hasOwnKey(manifest, 'auto_start')) {
    next.auto_start = toBoolean(manifest.auto_start, next.auto_start)
  } else if (next.activate && manifestPrefersAutoStart(manifest)) {
    next.auto_start = true
  }
  return next
}

function resolvePluginPermissionTitle(
  permission: PluginPermissionRequest,
  t: ReturnType<typeof getTranslations>
): string {
  const key = String(permission.key || '')
    .trim()
    .toLowerCase()
  switch (key) {
    case 'hook.execute':
      return t.admin.pluginPermissionHookExecuteTitle
    case 'hook.payload_patch':
      return t.admin.pluginPermissionHookPayloadPatchTitle
    case 'hook.block':
      return t.admin.pluginPermissionHookBlockTitle
    case 'frontend.extensions':
      return t.admin.pluginPermissionFrontendExtensionsTitle
    case 'frontend.html_trusted':
      return t.admin.pluginPermissionFrontendHtmlTrustedTitle
    case 'api.execute':
      return t.admin.pluginPermissionExecuteApiTitle
    case 'runtime.network':
      return t.admin.pluginPermissionRuntimeNetworkTitle
    case 'runtime.file_system':
      return t.admin.pluginPermissionRuntimeFileSystemTitle
    case 'host.order.read':
      return t.admin.pluginPermissionHostOrderReadTitle
    case 'host.order.read_privacy':
      return t.admin.pluginPermissionHostOrderPrivacyTitle
    case 'host.order.list':
      return t.admin.pluginPermissionHostOrderListTitle
    case 'host.order.assign_tracking':
      return t.admin.pluginPermissionHostOrderAssignTrackingTitle
    case 'host.order.request_resubmit':
      return t.admin.pluginPermissionHostOrderRequestResubmitTitle
    case 'host.order.mark_paid':
      return t.admin.pluginPermissionHostOrderMarkPaidTitle
    case 'host.order.update_price':
      return t.admin.pluginPermissionHostOrderUpdatePriceTitle
    case 'host.user.read':
      return t.admin.pluginPermissionHostUserReadTitle
    case 'host.user.list':
      return t.admin.pluginPermissionHostUserListTitle
    case 'host.product.read':
      return t.admin.pluginPermissionHostProductReadTitle
    case 'host.product.list':
      return t.admin.pluginPermissionHostProductListTitle
    case 'host.inventory.read':
      return t.admin.pluginPermissionHostInventoryReadTitle
    case 'host.inventory.list':
      return t.admin.pluginPermissionHostInventoryListTitle
    case 'host.inventory_binding.read':
      return t.admin.pluginPermissionHostInventoryBindingReadTitle
    case 'host.inventory_binding.list':
      return t.admin.pluginPermissionHostInventoryBindingListTitle
    case 'host.promo.read':
      return t.admin.pluginPermissionHostPromoReadTitle
    case 'host.promo.list':
      return t.admin.pluginPermissionHostPromoListTitle
    case 'host.ticket.read':
      return t.admin.pluginPermissionHostTicketReadTitle
    case 'host.ticket.list':
      return t.admin.pluginPermissionHostTicketListTitle
    case 'host.ticket.reply':
      return t.admin.pluginPermissionHostTicketReplyTitle
    case 'host.ticket.update':
      return t.admin.pluginPermissionHostTicketUpdateTitle
    case 'host.serial.read':
      return t.admin.pluginPermissionHostSerialReadTitle
    case 'host.serial.list':
      return t.admin.pluginPermissionHostSerialListTitle
    case 'host.announcement.read':
      return t.admin.pluginPermissionHostAnnouncementReadTitle
    case 'host.announcement.list':
      return t.admin.pluginPermissionHostAnnouncementListTitle
    case 'host.knowledge.read':
      return t.admin.pluginPermissionHostKnowledgeReadTitle
    case 'host.knowledge.list':
      return t.admin.pluginPermissionHostKnowledgeListTitle
    case 'host.knowledge.categories':
      return t.admin.pluginPermissionHostKnowledgeCategoriesTitle
    case 'host.payment_method.read':
      return t.admin.pluginPermissionHostPaymentMethodReadTitle
    case 'host.payment_method.list':
      return t.admin.pluginPermissionHostPaymentMethodListTitle
    case 'host.virtual_inventory.read':
      return t.admin.pluginPermissionHostVirtualInventoryReadTitle
    case 'host.virtual_inventory.list':
      return t.admin.pluginPermissionHostVirtualInventoryListTitle
    case 'host.virtual_inventory_binding.read':
      return t.admin.pluginPermissionHostVirtualInventoryBindingReadTitle
    case 'host.virtual_inventory_binding.list':
      return t.admin.pluginPermissionHostVirtualInventoryBindingListTitle
    case 'host.market.source.read':
      return t.admin.pluginPermissionHostMarketSourceReadTitle
    case 'host.market.catalog.read':
      return t.admin.pluginPermissionHostMarketCatalogReadTitle
    case 'host.market.install.preview':
      return t.admin.pluginPermissionHostMarketInstallPreviewTitle
    case 'host.market.install.execute':
      return t.admin.pluginPermissionHostMarketInstallExecuteTitle
    case 'host.market.install.read':
      return t.admin.pluginPermissionHostMarketInstallReadTitle
    case 'host.market.install.rollback':
      return t.admin.pluginPermissionHostMarketInstallRollbackTitle
    case 'host.email_template.read':
      return t.admin.pluginPermissionHostEmailTemplateReadTitle
    case 'host.email_template.write':
      return t.admin.pluginPermissionHostEmailTemplateWriteTitle
    case 'host.landing_page.read':
      return t.admin.pluginPermissionHostLandingPageReadTitle
    case 'host.landing_page.write':
      return t.admin.pluginPermissionHostLandingPageWriteTitle
    case 'host.invoice_template.read':
      return t.admin.pluginPermissionHostInvoiceTemplateReadTitle
    case 'host.invoice_template.write':
      return t.admin.pluginPermissionHostInvoiceTemplateWriteTitle
    case 'host.auth_branding.read':
      return t.admin.pluginPermissionHostAuthBrandingReadTitle
    case 'host.auth_branding.write':
      return t.admin.pluginPermissionHostAuthBrandingWriteTitle
    case 'host.page_rule_pack.read':
      return t.admin.pluginPermissionHostPageRulePackReadTitle
    case 'host.page_rule_pack.write':
      return t.admin.pluginPermissionHostPageRulePackWriteTitle
    default:
      return String(permission.title || key || t.admin.pluginPermissionCustomTitle)
  }
}

function resolvePluginPermissionDescription(
  permission: PluginPermissionRequest,
  t: ReturnType<typeof getTranslations>
): string {
  const key = String(permission.key || '')
    .trim()
    .toLowerCase()
  switch (key) {
    case 'hook.execute':
      return t.admin.pluginPermissionHookExecuteDesc
    case 'hook.payload_patch':
      return t.admin.pluginPermissionHookPayloadPatchDesc
    case 'hook.block':
      return t.admin.pluginPermissionHookBlockDesc
    case 'frontend.extensions':
      return t.admin.pluginPermissionFrontendExtensionsDesc
    case 'frontend.html_trusted':
      return t.admin.pluginPermissionFrontendHtmlTrustedDesc
    case 'api.execute':
      return t.admin.pluginPermissionExecuteApiDesc
    case 'runtime.network':
      return t.admin.pluginPermissionRuntimeNetworkDesc
    case 'runtime.file_system':
      return t.admin.pluginPermissionRuntimeFileSystemDesc
    case 'host.order.read':
      return t.admin.pluginPermissionHostOrderReadDesc
    case 'host.order.read_privacy':
      return t.admin.pluginPermissionHostOrderPrivacyDesc
    case 'host.order.list':
      return t.admin.pluginPermissionHostOrderListDesc
    case 'host.order.assign_tracking':
      return t.admin.pluginPermissionHostOrderAssignTrackingDesc
    case 'host.order.request_resubmit':
      return t.admin.pluginPermissionHostOrderRequestResubmitDesc
    case 'host.order.mark_paid':
      return t.admin.pluginPermissionHostOrderMarkPaidDesc
    case 'host.order.update_price':
      return t.admin.pluginPermissionHostOrderUpdatePriceDesc
    case 'host.user.read':
      return t.admin.pluginPermissionHostUserReadDesc
    case 'host.user.list':
      return t.admin.pluginPermissionHostUserListDesc
    case 'host.product.read':
      return t.admin.pluginPermissionHostProductReadDesc
    case 'host.product.list':
      return t.admin.pluginPermissionHostProductListDesc
    case 'host.inventory.read':
      return t.admin.pluginPermissionHostInventoryReadDesc
    case 'host.inventory.list':
      return t.admin.pluginPermissionHostInventoryListDesc
    case 'host.inventory_binding.read':
      return t.admin.pluginPermissionHostInventoryBindingReadDesc
    case 'host.inventory_binding.list':
      return t.admin.pluginPermissionHostInventoryBindingListDesc
    case 'host.promo.read':
      return t.admin.pluginPermissionHostPromoReadDesc
    case 'host.promo.list':
      return t.admin.pluginPermissionHostPromoListDesc
    case 'host.ticket.read':
      return t.admin.pluginPermissionHostTicketReadDesc
    case 'host.ticket.list':
      return t.admin.pluginPermissionHostTicketListDesc
    case 'host.ticket.reply':
      return t.admin.pluginPermissionHostTicketReplyDesc
    case 'host.ticket.update':
      return t.admin.pluginPermissionHostTicketUpdateDesc
    case 'host.serial.read':
      return t.admin.pluginPermissionHostSerialReadDesc
    case 'host.serial.list':
      return t.admin.pluginPermissionHostSerialListDesc
    case 'host.announcement.read':
      return t.admin.pluginPermissionHostAnnouncementReadDesc
    case 'host.announcement.list':
      return t.admin.pluginPermissionHostAnnouncementListDesc
    case 'host.knowledge.read':
      return t.admin.pluginPermissionHostKnowledgeReadDesc
    case 'host.knowledge.list':
      return t.admin.pluginPermissionHostKnowledgeListDesc
    case 'host.knowledge.categories':
      return t.admin.pluginPermissionHostKnowledgeCategoriesDesc
    case 'host.payment_method.read':
      return t.admin.pluginPermissionHostPaymentMethodReadDesc
    case 'host.payment_method.list':
      return t.admin.pluginPermissionHostPaymentMethodListDesc
    case 'host.virtual_inventory.read':
      return t.admin.pluginPermissionHostVirtualInventoryReadDesc
    case 'host.virtual_inventory.list':
      return t.admin.pluginPermissionHostVirtualInventoryListDesc
    case 'host.virtual_inventory_binding.read':
      return t.admin.pluginPermissionHostVirtualInventoryBindingReadDesc
    case 'host.virtual_inventory_binding.list':
      return t.admin.pluginPermissionHostVirtualInventoryBindingListDesc
    case 'host.market.source.read':
      return t.admin.pluginPermissionHostMarketSourceReadDesc
    case 'host.market.catalog.read':
      return t.admin.pluginPermissionHostMarketCatalogReadDesc
    case 'host.market.install.preview':
      return t.admin.pluginPermissionHostMarketInstallPreviewDesc
    case 'host.market.install.execute':
      return t.admin.pluginPermissionHostMarketInstallExecuteDesc
    case 'host.market.install.read':
      return t.admin.pluginPermissionHostMarketInstallReadDesc
    case 'host.market.install.rollback':
      return t.admin.pluginPermissionHostMarketInstallRollbackDesc
    case 'host.email_template.read':
      return t.admin.pluginPermissionHostEmailTemplateReadDesc
    case 'host.email_template.write':
      return t.admin.pluginPermissionHostEmailTemplateWriteDesc
    case 'host.landing_page.read':
      return t.admin.pluginPermissionHostLandingPageReadDesc
    case 'host.landing_page.write':
      return t.admin.pluginPermissionHostLandingPageWriteDesc
    case 'host.invoice_template.read':
      return t.admin.pluginPermissionHostInvoiceTemplateReadDesc
    case 'host.invoice_template.write':
      return t.admin.pluginPermissionHostInvoiceTemplateWriteDesc
    case 'host.auth_branding.read':
      return t.admin.pluginPermissionHostAuthBrandingReadDesc
    case 'host.auth_branding.write':
      return t.admin.pluginPermissionHostAuthBrandingWriteDesc
    case 'host.page_rule_pack.read':
      return t.admin.pluginPermissionHostPageRulePackReadDesc
    case 'host.page_rule_pack.write':
      return t.admin.pluginPermissionHostPageRulePackWriteDesc
    default:
      return String(permission.description || t.admin.pluginPermissionCustomDesc)
  }
}

function pretty(value: unknown): string {
  try {
    return JSON.stringify(value, null, 2)
  } catch {
    return String(value)
  }
}

function runtimeLabel(runtime: string, t: ReturnType<typeof getTranslations>): string {
  if (runtime === 'grpc') return t.admin.pluginRuntimeGrpc
  if (runtime === 'js_worker') return t.admin.pluginRuntimeJsWorker
  return runtime
}

function isJSWorkerRuntime(runtime: string): boolean {
  return runtime.trim().toLowerCase() === 'js_worker'
}

function runtimeAddressLabel(runtime: string, t: ReturnType<typeof getTranslations>): string {
  if (isJSWorkerRuntime(runtime)) return t.admin.pluginEntryScript
  return t.admin.pluginAddress
}

function runtimeAddressPlaceholder(runtime: string, t: ReturnType<typeof getTranslations>): string {
  if (isJSWorkerRuntime(runtime)) return t.admin.pluginEntryScriptPlaceholder
  return t.admin.pluginAddressPlaceholder
}

function runtimeAddressHint(
  runtime: string,
  mode: 'editor' | 'upload',
  t: ReturnType<typeof getTranslations>
): string {
  if (isJSWorkerRuntime(runtime)) {
    return mode === 'upload'
      ? t.admin.pluginEntryScriptHintUpload
      : t.admin.pluginEntryScriptHintEditor
  }
  return mode === 'upload' ? t.admin.pluginAddressHintGrpcUpload : t.admin.pluginAddressHintGrpc
}

function resolvePluginLifecycleActionState(plugin: AdminPlugin): PluginLifecycleActionState {
  const lifecycleRaw = String(plugin.lifecycle_status || 'draft')
    .trim()
    .toLowerCase()
  const knownLifecycle = new Set([
    'draft',
    'uploaded',
    'installed',
    'running',
    'paused',
    'degraded',
    'retired',
  ])
  const lifecycle = knownLifecycle.has(lifecycleRaw) ? lifecycleRaw : ''
  const enabled = !!plugin.enabled

  const isDraft = lifecycle === 'draft'
  const isUploaded = lifecycle === 'uploaded'
  const isInstalled = lifecycle === 'installed'
  const isRunning = lifecycle === 'running'
  const isPaused = lifecycle === 'paused'
  const isDegraded = lifecycle === 'degraded'
  const isRetired = lifecycle === 'retired'

  const hasRuntimeConnection = isRunning || isDegraded || (isInstalled && enabled)

  const install =
    isDraft || isUploaded || isPaused || isDegraded || isRetired || (lifecycle === '' && !enabled)
  const start = isInstalled || isPaused || isDegraded || (lifecycle === '' && !enabled)
  const pause = enabled && (isRunning || isDegraded || lifecycle === '')
  const restart = enabled && (isRunning || isDegraded || lifecycle === '')
  const hotReload = enabled && hasRuntimeConnection
  const resume = isPaused || isRetired || (lifecycle === '' && !enabled)
  const retire = !isRetired
  const test = enabled && hasRuntimeConnection
  const execute = enabled && hasRuntimeConnection
  const upload = true
  const versions = true
  const logs = true
  const edit = !isRunning && !isDegraded
  const remove = !enabled && !isRunning && !isDegraded

  return {
    install,
    start,
    pause,
    restart,
    hotReload,
    resume,
    retire,
    test,
    execute,
    upload,
    versions,
    logs,
    edit,
    remove,
  }
}

function resolvePluginErrorMessage(
  error: unknown,
  t: ReturnType<typeof getTranslations>,
  fallbackMessage: string
): string {
  return resolvePluginOperationErrorMessage(error, t, fallbackMessage)
}

function resolvePluginResponseMessage(
  payload: unknown,
  t: ReturnType<typeof getTranslations>,
  fallbackMessage: string
): string {
  return resolvePluginOperationErrorMessage(payload, t, fallbackMessage)
}

function normalizeMarketQueryKinds(value: string): string[] {
  const seen = new Set<string>()
  return value
    .split(',')
    .map((item) => item.trim().toLowerCase())
    .filter((item) => {
      if (!item || seen.has(item)) return false
      seen.add(item)
      return true
    })
}

function parseMarketUploadRequest(
  searchParams: ReturnType<typeof useSearchParams>
): AdminPluginMarketPreviewRequest | null {
  const enabled = String(searchParams?.get('market_install') || '')
    .trim()
    .toLowerCase()
  if (!enabled || !['1', 'true', 'yes'].includes(enabled)) {
    return null
  }
  const sourceID = String(searchParams?.get('market_source_id') || '').trim()
  const baseURL = String(searchParams?.get('market_source_base_url') || '').trim()
  const kind = String(searchParams?.get('market_kind') || '')
    .trim()
    .toLowerCase()
  const name = String(searchParams?.get('market_name') || '').trim()
  const version = String(searchParams?.get('market_version') || '').trim()
  if (!baseURL || !kind || !name || !version) {
    return null
  }
  return {
    source: {
      source_id: sourceID || 'market',
      name: String(searchParams?.get('market_source_name') || '').trim() || undefined,
      base_url: baseURL,
      public_key: String(searchParams?.get('market_source_public_key') || '').trim() || undefined,
      default_channel: String(searchParams?.get('market_source_channel') || '').trim() || undefined,
      allowed_kinds: normalizeMarketQueryKinds(
        String(searchParams?.get('market_source_allowed_kinds') || kind)
      ),
      enabled: true,
    },
    kind,
    name,
    version,
  }
}

function buildMarketUploadRequestKey(request: AdminPluginMarketPreviewRequest | null): string {
  if (!request) return ''
  return [
    request.source.source_id,
    request.source.base_url,
    request.kind,
    request.name,
    request.version,
  ].join('|')
}

function AdminPluginsPageContent() {
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.adminPlugins)
  const toast = useToast()
  const router = useRouter()
  const pathname = usePathname() || '/admin/plugins'
  const searchParams = useSearchParams()
  const showPluginErrorToast = (message: string) => {
    toast.error(renderToastMessage(message))
  }
  const queryClient = useQueryClient()
  const { hasPermission, isSuperAdmin } = usePermission()

  const [permissionReady, setPermissionReady] = useState(false)
  useEffect(() => {
    setPermissionReady(true)
  }, [])
  const canManage = permissionReady && isSuperAdmin() && hasPermission('system.config')

  const [editorOpen, setEditorOpen] = useState(false)
  const [editingPlugin, setEditingPlugin] = useState<AdminPlugin | null>(null)
  const [pluginForm, setPluginForm] = useState<PluginForm>(EMPTY_PLUGIN_FORM)
  const [pluginHookAccessState, setPluginHookAccessState] = useState<PluginHookAccessState>(
    DEFAULT_PLUGIN_HOOK_ACCESS_STATE
  )
  const [pluginCapabilityPolicyState, setPluginCapabilityPolicyState] =
    useState<PluginCapabilityPolicyState>(DEFAULT_PLUGIN_CAPABILITY_POLICY_STATE)
  const [pluginFrontendAccessState, setPluginFrontendAccessState] =
    useState<PluginFrontendAccessState>(DEFAULT_PLUGIN_FRONTEND_ACCESS_STATE)
  const [pluginSaveBaseline, setPluginSaveBaseline] = useState<string | null>(null)
  const [pluginSecretDrafts, setPluginSecretDrafts] = useState<Record<string, string>>({})
  const [pluginSecretDeleteKeys, setPluginSecretDeleteKeys] = useState<string[]>([])

  const [uploadOpen, setUploadOpen] = useState(false)
  const [uploadForm, setUploadForm] = useState<UploadForm>(EMPTY_UPLOAD_FORM)
  const [uploadHookAccessState, setUploadHookAccessState] = useState<PluginHookAccessState>(
    DEFAULT_PLUGIN_HOOK_ACCESS_STATE
  )
  const [uploadCapabilityPolicyState, setUploadCapabilityPolicyState] =
    useState<PluginCapabilityPolicyState>(DEFAULT_PLUGIN_CAPABILITY_POLICY_STATE)
  const [uploadFrontendAccessState, setUploadFrontendAccessState] =
    useState<PluginFrontendAccessState>(DEFAULT_PLUGIN_FRONTEND_ACCESS_STATE)
  const [uploadFile, setUploadFile] = useState<File | null>(null)
  const [uploadInputKey, setUploadInputKey] = useState(0)
  const [uploadPreview, setUploadPreview] = useState<UploadPermissionPreview | null>(null)
  const [uploadGrantedPermissions, setUploadGrantedPermissions] = useState<string[]>([])
  const [marketUploadContext, setMarketUploadContext] = useState<MarketPluginInstallContext | null>(
    null
  )
  const marketUploadRequest = useMemo(() => parseMarketUploadRequest(searchParams), [searchParams])
  const marketUploadRequestKey = useMemo(
    () => buildMarketUploadRequestKey(marketUploadRequest),
    [marketUploadRequest]
  )
  const [handledMarketUploadRequestKey, setHandledMarketUploadRequestKey] = useState('')

  const [deletePlugin, setDeletePlugin] = useState<AdminPlugin | null>(null)
  const [versionPlugin, setVersionPlugin] = useState<AdminPlugin | null>(null)
  const [activateAutoStart, setActivateAutoStart] = useState(false)
  const [logPlugin, setLogPlugin] = useState<AdminPlugin | null>(null)
  const [workspacePlugin, setWorkspacePlugin] = useState<AdminPlugin | null>(null)
  const [diagnosticPlugin, setDiagnosticPlugin] = useState<AdminPlugin | null>(null)
  const [latestPluginOperation, setLatestPluginOperation] =
    useState<PluginOperationFeedback | null>(null)

  const editingPluginManifest = useMemo(
    () => parsePluginManifestText(editingPlugin?.manifest),
    [editingPlugin?.manifest]
  )
  const editingConfigSchema = useMemo(
    () => parseManifestObjectSchema(editingPluginManifest?.config_schema, locale),
    [editingPluginManifest?.config_schema, locale]
  )
  const editingSecretSchema = useMemo(
    () => parseManifestObjectSchema(editingPluginManifest?.secret_schema, locale),
    [editingPluginManifest?.secret_schema, locale]
  )
  const editingRuntimeParamsSchema = useMemo(
    () => parseManifestObjectSchema(editingPluginManifest?.runtime_params_schema, locale),
    [editingPluginManifest?.runtime_params_schema, locale]
  )

  const invalidatePluginBootstrapMenus = () => {
    clearCachedBootstrapMenus()
    queryClient.invalidateQueries({ queryKey: ['plugin-bootstrap'] })
    queryClient.invalidateQueries({ queryKey: ['plugin-slot'] })
    queryClient.invalidateQueries({ queryKey: ['plugin-extension-batch'] })
  }
  const clearMarketInstallQuery = () => {
    if (!marketUploadRequestKey) return
    setHandledMarketUploadRequestKey(marketUploadRequestKey)
    router.replace(pathname, { scroll: false })
  }

  const hookCatalogQuery = useQuery({
    queryKey: ['adminPluginHookCatalog'],
    queryFn: getAdminPluginHookCatalog,
    enabled: canManage,
  })
  const hookCatalogGroups = useMemo(
    () => normalizePluginHookCatalogGroups(hookCatalogQuery.data),
    [hookCatalogQuery.data]
  )
  const frontendSlotCatalog = useMemo(
    () =>
      buildFrontendSlotCatalog([
        ...pluginFrontendAccessState.selectedFrontendSlots,
        ...uploadFrontendAccessState.selectedFrontendSlots,
      ]),
    [
      pluginFrontendAccessState.selectedFrontendSlots,
      uploadFrontendAccessState.selectedFrontendSlots,
    ]
  )
  const frontendPermissionCatalog = useMemo(
    () =>
      buildFrontendPermissionCatalog(
        [
          ...pluginFrontendAccessState.frontendRequiredPermissions,
          ...uploadFrontendAccessState.frontendRequiredPermissions,
        ],
        t
      ),
    [
      pluginFrontendAccessState.frontendRequiredPermissions,
      uploadFrontendAccessState.frontendRequiredPermissions,
      t,
    ]
  )
  const capabilityPermissionOptions = useMemo<PluginCapabilityPermissionOption[]>(() => {
    const known = [
      'hook.execute',
      'hook.payload_patch',
      'hook.block',
      'frontend.extensions',
      'frontend.html_trusted',
      'api.execute',
      'runtime.network',
      'runtime.file_system',
      'host.order.read',
      'host.order.read_privacy',
      'host.order.list',
      'host.order.assign_tracking',
      'host.order.request_resubmit',
      'host.order.mark_paid',
      'host.order.update_price',
      'host.user.read',
      'host.user.list',
      'host.product.read',
      'host.product.list',
      'host.inventory.read',
      'host.inventory.list',
      'host.inventory_binding.read',
      'host.inventory_binding.list',
      'host.promo.read',
      'host.promo.list',
      'host.ticket.read',
      'host.ticket.list',
      'host.ticket.reply',
      'host.ticket.update',
      'host.serial.read',
      'host.serial.list',
      'host.announcement.read',
      'host.announcement.list',
      'host.knowledge.read',
      'host.knowledge.list',
      'host.knowledge.categories',
      'host.payment_method.read',
      'host.payment_method.list',
      'host.virtual_inventory.read',
      'host.virtual_inventory.list',
      'host.virtual_inventory_binding.read',
      'host.virtual_inventory_binding.list',
    ]
    const extras = normalizePermissionList([
      ...pluginCapabilityPolicyState.requestedPermissions,
      ...pluginCapabilityPolicyState.grantedPermissions,
      ...uploadCapabilityPolicyState.requestedPermissions,
      ...uploadCapabilityPolicyState.grantedPermissions,
    ])
    return normalizeStringList([...known, ...extras]).map((key) => ({
      value: key,
      title: resolvePluginPermissionTitle({ key, required: false }, t),
      description: resolvePluginPermissionDescription({ key, required: false }, t),
    }))
  }, [
    pluginCapabilityPolicyState.grantedPermissions,
    pluginCapabilityPolicyState.requestedPermissions,
    t,
    uploadCapabilityPolicyState.grantedPermissions,
    uploadCapabilityPolicyState.requestedPermissions,
  ])

  const pluginCapabilitiesValid = useMemo(
    () => isJSONObjectText(pluginForm.capabilities),
    [pluginForm.capabilities]
  )
  const pluginConfigValid = useMemo(
    () => isBlankOrJSONObjectText(pluginForm.config),
    [pluginForm.config]
  )
  const pluginRuntimeParamsValid = useMemo(
    () => isBlankOrJSONObjectText(pluginForm.runtime_params),
    [pluginForm.runtime_params]
  )
  const uploadCapabilitiesValid = useMemo(
    () => isJSONObjectText(uploadForm.capabilities),
    [uploadForm.capabilities]
  )
  const uploadConfigValid = useMemo(
    () => isBlankOrJSONObjectText(uploadForm.config),
    [uploadForm.config]
  )
  const uploadRuntimeParamsValid = useMemo(
    () => isBlankOrJSONObjectText(uploadForm.runtime_params),
    [uploadForm.runtime_params]
  )
  const pluginFrontendValidationMessage = useMemo(
    () => validatePluginFrontendAccessState(pluginFrontendAccessState, t),
    [pluginFrontendAccessState, t]
  )
  const uploadFrontendValidationMessage = useMemo(
    () => validatePluginFrontendAccessState(uploadFrontendAccessState, t),
    [uploadFrontendAccessState, t]
  )

  const resolvePluginHookGroupLabel = (groupKey: string): string => {
    switch (groupKey) {
      case 'frontend':
        return t.admin.pluginCheckerGroupFrontend
      case 'auth':
        return t.admin.pluginCheckerGroupAuth
      case 'order':
        return t.admin.pluginCheckerGroupOrder
      case 'payment':
        return t.admin.pluginCheckerGroupPayment
      case 'ticket':
        return t.admin.pluginCheckerGroupTicket
      case 'product_inventory':
        return t.admin.pluginCheckerGroupProductInventory
      case 'promo':
        return t.admin.pluginCheckerGroupPromo
      case 'other':
        return t.admin.pluginHookAccessGroupOther
      default:
        return groupKey
    }
  }

  const resolveFrontendSlotGroupLabel = (groupKey: string): string => {
    switch (groupKey) {
      case 'user':
        return t.admin.pluginFrontendSlotGroupUser
      case 'admin':
        return t.admin.pluginFrontendSlotGroupAdmin
      case 'auth':
        return t.admin.pluginFrontendSlotGroupAuth
      case 'public':
        return t.admin.pluginFrontendSlotGroupPublic
      case 'other':
        return t.admin.pluginFrontendSlotGroupOther
      default:
        return groupKey
    }
  }

  const handlePluginCapabilitiesChange = (value: string) => {
    setPluginForm((prev) => ({ ...prev, capabilities: value }))
    if (isJSONObjectText(value)) {
      setPluginHookAccessState(parsePluginHookAccessState(value))
      setPluginCapabilityPolicyState(parsePluginCapabilityPolicyState(value))
      setPluginFrontendAccessState(parsePluginFrontendAccessState(value))
    }
  }

  const handlePluginConfigBlur = () => {
    setPluginForm((prev) => ({
      ...prev,
      config: tryFormatTextareaJSON(prev.config, '{}', formatHumanReadableJSONText),
    }))
  }

  const handlePluginRuntimeParamsBlur = () => {
    setPluginForm((prev) => ({
      ...prev,
      runtime_params: tryFormatTextareaJSON(prev.runtime_params, '{}', formatHumanReadableJSONText),
    }))
  }

  const handlePluginCapabilitiesBlur = () => {
    setPluginForm((prev) => {
      const capabilities = tryFormatTextareaJSON(
        prev.capabilities,
        '{}',
        formatCapabilitiesForDisplay
      )
      if (isJSONObjectText(capabilities)) {
        setPluginHookAccessState(parsePluginHookAccessState(capabilities))
        setPluginCapabilityPolicyState(parsePluginCapabilityPolicyState(capabilities))
        setPluginFrontendAccessState(parsePluginFrontendAccessState(capabilities))
      }
      return {
        ...prev,
        capabilities,
      }
    })
  }

  const handlePluginHookAccessChange = (state: PluginHookAccessState) => {
    if (!pluginCapabilitiesValid) return
    setPluginHookAccessState(state)
    setPluginForm((prev) => ({
      ...prev,
      capabilities: mergePluginHookAccessState(prev.capabilities, state),
    }))
  }

  const handlePluginCapabilityPolicyChange = (state: PluginCapabilityPolicyState) => {
    if (!pluginCapabilitiesValid) return
    setPluginCapabilityPolicyState(state)
    setPluginForm((prev) => ({
      ...prev,
      capabilities: mergePluginCapabilityPolicyState(prev.capabilities, state),
    }))
  }

  const handlePluginFrontendAccessChange = (state: PluginFrontendAccessState) => {
    if (!pluginCapabilitiesValid) return
    setPluginFrontendAccessState(state)
    setPluginForm((prev) => ({
      ...prev,
      capabilities: mergePluginFrontendAccessState(prev.capabilities, state),
    }))
  }

  const handlePluginSecretDraftChange = (key: string, value: string) => {
    const normalizedKey = key.trim()
    if (!normalizedKey) return
    setPluginSecretDrafts((prev) => ({
      ...prev,
      [normalizedKey]: value,
    }))
    if (value.trim()) {
      setPluginSecretDeleteKeys((prev) => prev.filter((item) => item !== normalizedKey))
    }
  }

  const handlePluginSecretDeleteToggle = (key: string, checked: boolean) => {
    const normalizedKey = key.trim()
    if (!normalizedKey) return
    setPluginSecretDeleteKeys((prev) =>
      checked
        ? normalizePluginSecretKeyList([...prev, normalizedKey])
        : prev.filter((item) => item !== normalizedKey)
    )
    if (checked) {
      setPluginSecretDrafts((prev) => {
        if (!Object.prototype.hasOwnProperty.call(prev, normalizedKey)) {
          return prev
        }
        const next = { ...prev }
        delete next[normalizedKey]
        return next
      })
    }
  }

  const handleUploadCapabilitiesChange = (value: string) => {
    setUploadForm((prev) => ({ ...prev, capabilities: value }))
    if (isJSONObjectText(value)) {
      setUploadHookAccessState(parsePluginHookAccessState(value))
      setUploadCapabilityPolicyState(parsePluginCapabilityPolicyState(value))
      setUploadFrontendAccessState(parsePluginFrontendAccessState(value))
    }
  }

  const handleUploadConfigBlur = () => {
    setUploadForm((prev) => ({
      ...prev,
      config: tryFormatTextareaJSON(prev.config, '', formatHumanReadableJSONText),
    }))
  }

  const handleUploadRuntimeParamsBlur = () => {
    setUploadForm((prev) => ({
      ...prev,
      runtime_params: tryFormatTextareaJSON(prev.runtime_params, '{}', formatHumanReadableJSONText),
    }))
  }

  const handleUploadCapabilitiesBlur = () => {
    setUploadForm((prev) => {
      const capabilities = tryFormatTextareaJSON(
        prev.capabilities,
        '{}',
        formatCapabilitiesForDisplay
      )
      if (isJSONObjectText(capabilities)) {
        setUploadHookAccessState(parsePluginHookAccessState(capabilities))
        setUploadCapabilityPolicyState(parsePluginCapabilityPolicyState(capabilities))
        setUploadFrontendAccessState(parsePluginFrontendAccessState(capabilities))
      }
      return {
        ...prev,
        capabilities,
      }
    })
  }

  const handleUploadHookAccessChange = (state: PluginHookAccessState) => {
    if (!uploadCapabilitiesValid) return
    setUploadHookAccessState(state)
    setUploadForm((prev) => ({
      ...prev,
      capabilities: mergePluginHookAccessState(prev.capabilities, state),
    }))
  }

  const handleUploadCapabilityPolicyChange = (state: PluginCapabilityPolicyState) => {
    if (!uploadCapabilitiesValid) return
    setUploadCapabilityPolicyState(state)
    setUploadForm((prev) => ({
      ...prev,
      capabilities: mergePluginCapabilityPolicyState(prev.capabilities, state),
    }))
  }

  const handleUploadFrontendAccessChange = (state: PluginFrontendAccessState) => {
    if (!uploadCapabilitiesValid) return
    setUploadFrontendAccessState(state)
    setUploadForm((prev) => ({
      ...prev,
      capabilities: mergePluginFrontendAccessState(prev.capabilities, state),
    }))
  }

  const pluginsQuery = useQuery({
    queryKey: ['adminPlugins'],
    queryFn: getAdminPlugins,
    enabled: canManage,
  })
  const plugins = useMemo(() => extractArray<AdminPlugin>(pluginsQuery.data), [pluginsQuery.data])
  const buildPluginOperationFeedback = (
    kind: 'upload' | 'market_install' | 'activate',
    response: unknown,
    options?: {
      sourceLabel?: string
      fallbackPluginId?: number | null
      fallbackPluginName?: string
      fallbackVersion?: string
      fallbackWarnings?: string[]
      autoStart?: boolean
    }
  ): PluginOperationFeedback => {
    const payload = extractObject<Record<string, unknown>>(response)
    const pluginPayload = extractObject<Record<string, unknown>>(payload?.plugin)
    const versionPayload = extractObject<Record<string, unknown>>(payload?.version)
    const activateFailed = !!payload?.activate_failed
    const pluginId =
      extractNumberField(pluginPayload, ['id']) ??
      extractNumberField(payload, ['plugin_id', 'id']) ??
      options?.fallbackPluginId ??
      null
    const matchedPlugin =
      pluginId !== null ? plugins.find((item) => item.id === pluginId) || null : null
    const pluginSnapshot =
      pluginPayload || matchedPlugin
        ? ({
            ...(matchedPlugin || {}),
            ...((pluginPayload || {}) as Partial<AdminPlugin>),
          } as AdminPlugin)
        : null
    const pluginMetadata = resolvePluginDisplayMetadata(pluginSnapshot || matchedPlugin, locale)
    const pluginName =
      pluginMetadata.displayName ||
      extractTextField(pluginPayload, ['display_name', 'name']) ||
      matchedPlugin?.display_name?.trim() ||
      matchedPlugin?.name?.trim() ||
      options?.fallbackPluginName?.trim() ||
      (locale === 'zh' ? '未命名插件' : 'Unnamed plugin')
    const version =
      extractTextField(versionPayload, ['version']) ||
      extractTextField(pluginPayload, ['version']) ||
      options?.fallbackVersion?.trim() ||
      matchedPlugin?.version?.trim() ||
      '-'
    const warnings = normalizeUniqueTextList([
      ...normalizeUniqueTextList(payload?.warnings),
      ...normalizeUniqueTextList(options?.fallbackWarnings),
    ])
    const status = extractTextField(payload, ['status']) || undefined
    const lifecycleStatus =
      extractTextField(pluginPayload, ['lifecycle_status']) ||
      extractTextField(versionPayload, ['lifecycle_status']) ||
      undefined
    const healthStatus = extractTextField(pluginPayload, ['status']) || undefined
    const sourceLabel =
      options?.sourceLabel?.trim() ||
      extractTextField(versionPayload, ['market_source_id']) ||
      undefined
    const errorDetail =
      extractTextField(payload, ['error', 'message']) ||
      extractTextField(versionPayload, ['error']) ||
      undefined

    let title = ''
    let summary = ''
    let detail = errorDetail
    let tone: PluginOperationFeedback['tone'] =
      activateFailed || warnings.length > 0 ? 'warning' : 'success'

    if (kind === 'upload') {
      title = activateFailed
        ? locale === 'zh'
          ? '上传完成，激活失败'
          : 'Upload completed, activation failed'
        : t.admin.pluginUploadSuccess
      summary =
        locale === 'zh'
          ? `${pluginName} 已写入版本 ${version}`
          : `${pluginName} uploaded as version ${version}`
      if (activateFailed && !detail) {
        detail =
          locale === 'zh'
            ? '版本已经上传，但自动激活失败，可在版本管理或诊断中继续处理。'
            : 'The version was uploaded, but activation failed. Continue in Versions or Diagnostics.'
      }
    } else if (kind === 'market_install') {
      title = activateFailed
        ? locale === 'zh'
          ? '市场导入完成，激活失败'
          : 'Market import completed, activation failed'
        : locale === 'zh'
          ? '市场导入完成'
          : 'Market import completed'
      summary = sourceLabel
        ? locale === 'zh'
          ? `${pluginName} ${version} 已从 ${sourceLabel} 导入`
          : `${pluginName} ${version} imported from ${sourceLabel}`
        : locale === 'zh'
          ? `${pluginName} 已导入版本 ${version}`
          : `${pluginName} imported version ${version}`
      if (activateFailed && !detail) {
        detail =
          locale === 'zh'
            ? '插件已导入，但激活失败，可直接进入版本管理或诊断继续处理。'
            : 'The package was imported, but activation failed. Continue in Versions or Diagnostics.'
      }
    } else {
      title = t.admin.pluginHotUpdateSuccess
      summary =
        locale === 'zh'
          ? `${pluginName} 已切换到版本 ${version}`
          : `${pluginName} switched to version ${version}`
      if (options?.autoStart) {
        detail =
          locale === 'zh'
            ? '已同时请求自动启动运行时。'
            : 'Auto start was requested with activation.'
      }
    }

    return {
      tone,
      title,
      summary,
      detail,
      pluginId,
      pluginSnapshot,
      pluginName,
      version,
      sourceLabel,
      status,
      lifecycleStatus,
      healthStatus,
      warnings,
      occurredAt: new Date().toISOString(),
    }
  }
  const uploadManifest = useMemo(() => {
    if (uploadPreview?.manifest) return uploadPreview.manifest
    const target = plugins.find((plugin) => String(plugin.id) === uploadForm.plugin_id)
    return parsePluginManifestText(target?.manifest)
  }, [plugins, uploadForm.plugin_id, uploadPreview?.manifest])
  const uploadConfigSchema = useMemo(
    () => parseManifestObjectSchema(uploadManifest?.config_schema, locale),
    [locale, uploadManifest?.config_schema]
  )
  const uploadRuntimeParamsSchema = useMemo(
    () => parseManifestObjectSchema(uploadManifest?.runtime_params_schema, locale),
    [locale, uploadManifest?.runtime_params_schema]
  )
  const uploadConflictSummary = useMemo(
    () =>
      resolvePluginUploadConflictSummary({
        manifest: uploadPreview?.manifest ?? null,
        plugins,
        targetPluginID: uploadForm.plugin_id,
        pluginName: uploadForm.name,
      }),
    [plugins, uploadForm.name, uploadForm.plugin_id, uploadPreview?.manifest]
  )

  const versionsQuery = useQuery({
    queryKey: ['adminPluginVersions', versionPlugin?.id],
    queryFn: () => getAdminPluginVersions((versionPlugin as AdminPlugin).id),
    enabled: !!versionPlugin,
  })
  const versions = useMemo(
    () => extractArray<AdminPluginVersion>(versionsQuery.data),
    [versionsQuery.data]
  )

  const logsQuery = useQuery({
    queryKey: ['adminPluginExecutions', logPlugin?.id],
    queryFn: () => getAdminPluginExecutions((logPlugin as AdminPlugin).id),
    enabled: !!logPlugin,
  })
  const logs = useMemo(() => extractArray<AdminPluginExecution>(logsQuery.data), [logsQuery.data])
  const workspaceQuery = useQuery({
    queryKey: ['adminPluginWorkspace', workspacePlugin?.id],
    queryFn: () => getAdminPluginWorkspace((workspacePlugin as AdminPlugin).id, { limit: 200 }),
    enabled: !!workspacePlugin,
  })
  const workspaceResponse = useMemo(
    () => extractObject<AdminPluginWorkspaceResponse>(workspaceQuery.data),
    [workspaceQuery.data]
  )
  const workspaceSnapshot = useMemo(
    () => extractObject<AdminPluginWorkspaceSnapshot>(workspaceResponse?.workspace),
    [workspaceResponse]
  )

  const diagnosticsQuery = useQuery({
    queryKey: ['adminPluginDiagnostics', diagnosticPlugin?.id],
    queryFn: () => getAdminPluginDiagnostics((diagnosticPlugin as AdminPlugin).id),
    enabled: !!diagnosticPlugin,
  })
  const pluginDiagnostics = useMemo(
    () => extractObject<AdminPluginDiagnostics>(diagnosticsQuery.data),
    [diagnosticsQuery.data]
  )
  const pluginSecretsQuery = useQuery({
    queryKey: ['adminPluginSecrets', editingPlugin?.id],
    queryFn: () => getAdminPluginSecrets((editingPlugin as AdminPlugin).id),
    enabled: editorOpen && !!editingPlugin?.id && !!editingSecretSchema,
  })
  const editingSecretMeta = useMemo(
    () => extractArray<AdminPluginSecretMeta>(pluginSecretsQuery.data),
    [pluginSecretsQuery.data]
  )
  const editingSecretMetaMap = useMemo(
    () => buildPluginSecretMetaMap(editingSecretMeta),
    [editingSecretMeta]
  )

  const resetPluginEditorState = () => {
    setEditorOpen(false)
    setEditingPlugin(null)
    setPluginForm(EMPTY_PLUGIN_FORM)
    setPluginHookAccessState(DEFAULT_PLUGIN_HOOK_ACCESS_STATE)
    setPluginCapabilityPolicyState(DEFAULT_PLUGIN_CAPABILITY_POLICY_STATE)
    setPluginFrontendAccessState(DEFAULT_PLUGIN_FRONTEND_ACCESS_STATE)
    setPluginSaveBaseline(null)
    setPluginSecretDrafts({})
    setPluginSecretDeleteKeys([])
  }

  const handleEditorOpenChange = (open: boolean) => {
    if (!open) {
      resetPluginEditorState()
      return
    }
    setEditorOpen(true)
  }

  const cancelTaskMutation = useMutation({
    mutationFn: (payload: { pluginId: number; taskId: string }) =>
      cancelAdminPluginExecutionTask(payload.pluginId, payload.taskId),
    onSuccess: (_resp, payload) => {
      toast.success(t.admin.pluginDiagnosticsTaskCancelRequested)
      queryClient.invalidateQueries({
        queryKey: ['adminPluginDiagnostics', payload.pluginId],
      })
    },
    onError: (error: any) =>
      showPluginErrorToast(resolvePluginErrorMessage(error, t, t.admin.operationFailed)),
  })

  const saveMutation = useMutation<SavePluginResult, any, SavePluginPayload>({
    mutationFn: async (payload) => {
      const pluginChanged =
        !payload.pluginId ||
        serializePluginSaveRequestData(payload.data) !== String(payload.baseline || '')
      if (payload.pluginId) {
        let response: unknown = null
        if (pluginChanged) {
          response = await updateAdminPlugin(payload.pluginId, payload.data)
        }
        if (payload.secretPatch) {
          await updateAdminPluginSecrets(payload.pluginId, payload.secretPatch)
        }
        return {
          pluginId: payload.pluginId,
          response,
          secretUpdated: !!payload.secretPatch,
        }
      }

      const response = await createAdminPlugin(payload.data)
      const createdPluginId = extractCreatedPluginID(response)
      if (payload.secretPatch) {
        if (!createdPluginId) {
          throw new Error('plugin created but failed to resolve plugin id for secret update')
        }
        await updateAdminPluginSecrets(createdPluginId, payload.secretPatch)
      }
      return {
        pluginId: createdPluginId,
        response,
        secretUpdated: !!payload.secretPatch,
      }
    },
    onSuccess: (result, payload) => {
      toast.success(payload.pluginId ? t.admin.pluginUpdateSuccess : t.admin.pluginCreateSuccess)
      queryClient.invalidateQueries({ queryKey: ['adminPlugins'] })
      if (result.pluginId) {
        queryClient.invalidateQueries({ queryKey: ['adminPluginSecrets', result.pluginId] })
      }
      invalidatePluginBootstrapMenus()
      resetPluginEditorState()
    },
    onError: (error: any) =>
      showPluginErrorToast(resolvePluginErrorMessage(error, t, t.admin.operationFailed)),
  })

  const deleteMutation = useMutation({
    mutationFn: (pluginId: number) => deleteAdminPlugin(pluginId),
    onSuccess: () => {
      toast.success(t.admin.pluginDeleteSuccess)
      queryClient.invalidateQueries({ queryKey: ['adminPlugins'] })
      invalidatePluginBootstrapMenus()
      setDeletePlugin(null)
    },
    onError: (error: any) =>
      showPluginErrorToast(resolvePluginErrorMessage(error, t, t.admin.operationFailed)),
  })

  const resetUploadDialogState = (clearMarketQuery = false) => {
    setUploadOpen(false)
    setUploadForm(EMPTY_UPLOAD_FORM)
    setUploadHookAccessState(DEFAULT_PLUGIN_HOOK_ACCESS_STATE)
    setUploadCapabilityPolicyState(DEFAULT_PLUGIN_CAPABILITY_POLICY_STATE)
    setUploadFrontendAccessState(DEFAULT_PLUGIN_FRONTEND_ACCESS_STATE)
    setUploadFile(null)
    setUploadPreview(null)
    setUploadGrantedPermissions([])
    setMarketUploadContext(null)
    setUploadInputKey((n) => n + 1)
    if (clearMarketQuery) {
      clearMarketInstallQuery()
    }
  }

  const applyUploadPreviewPayload = (
    payload: any,
    nextMarketContext: MarketPluginInstallContext | null = null
  ) => {
    const requestedPermissions = Array.isArray(payload?.requested_permissions)
      ? (payload.requested_permissions as PluginPermissionRequest[])
      : []
    const defaultGranted = Array.isArray(payload?.default_granted_permissions)
      ? normalizeStringList(payload.default_granted_permissions.map((item: any) => String(item)))
      : []
    const requiredGranted = requestedPermissions
      .filter((item) => !!item?.required)
      .map((item) => String(item?.key || ''))
    const manifestObj =
      payload?.manifest && typeof payload.manifest === 'object'
        ? (payload.manifest as PluginManifestPreview)
        : null

    setUploadPreview({
      manifest: manifestObj,
      requested_permissions: requestedPermissions,
      default_granted_permissions: defaultGranted,
    })
    setUploadGrantedPermissions(normalizeStringList([...defaultGranted, ...requiredGranted]))
    setUploadForm((prev) => {
      const next = mergeUploadFormWithManifest(prev, manifestObj, locale)
      setUploadHookAccessState(parsePluginHookAccessState(next.capabilities))
      setUploadCapabilityPolicyState(parsePluginCapabilityPolicyState(next.capabilities))
      setUploadFrontendAccessState(parsePluginFrontendAccessState(next.capabilities))
      return next
    })
    setMarketUploadContext(nextMarketContext)
  }

  const uploadMutation = useMutation({
    mutationFn: (formData: FormData) => uploadAdminPluginPackage(formData),
    onSuccess: (resp: any) => {
      const payload =
        resp && typeof resp === 'object' && resp.data && typeof resp.data === 'object'
          ? resp.data
          : resp
      const activateFailed = !!payload?.activate_failed
      if (activateFailed) {
        showPluginErrorToast(
          resolvePluginResponseMessage(payload, t, t.admin.pluginUploadActivateFailed)
        )
      } else {
        toast.success(t.admin.pluginUploadSuccess)
      }
      queryClient.invalidateQueries({ queryKey: ['adminPlugins'] })
      if (uploadForm.plugin_id) {
        queryClient.invalidateQueries({
          queryKey: ['adminPluginVersions', Number(uploadForm.plugin_id)],
        })
      }
      setLatestPluginOperation(
        buildPluginOperationFeedback('upload', payload, {
          fallbackPluginId: uploadForm.plugin_id ? Number(uploadForm.plugin_id) : null,
          fallbackPluginName: uploadForm.display_name.trim() || uploadForm.name.trim(),
          fallbackVersion: uploadForm.version.trim(),
        })
      )
      invalidatePluginBootstrapMenus()
      resetUploadDialogState()
    },
    onError: (error: any) =>
      showPluginErrorToast(resolvePluginErrorMessage(error, t, t.admin.pluginUploadFailed)),
  })

  const previewUploadMutation = useMutation({
    mutationFn: (file: File) => {
      const formData = new FormData()
      formData.append('file', file)
      return previewAdminPluginPackage(formData)
    },
    onSuccess: (resp: any) => {
      const payload =
        resp && typeof resp === 'object' && resp.data && typeof resp.data === 'object'
          ? resp.data
          : resp
      applyUploadPreviewPayload(payload)
    },
    onError: (error: any) => {
      setUploadPreview(null)
      setUploadGrantedPermissions([])
      showPluginErrorToast(resolvePluginErrorMessage(error, t, t.admin.operationFailed))
    },
  })

  const previewMarketUploadMutation = useMutation({
    mutationFn: (request: AdminPluginMarketPreviewRequest) =>
      previewAdminPluginMarketInstall(request),
    onSuccess: (resp: any, request) => {
      const payload =
        resp && typeof resp === 'object' && resp.data && typeof resp.data === 'object'
          ? resp.data
          : resp
      applyUploadPreviewPayload(payload, {
        source: request.source,
        coordinates: {
          source_id: request.source.source_id,
          kind: request.kind,
          name: request.name,
          version: request.version,
        },
        release: payload?.release && typeof payload.release === 'object' ? payload.release : null,
        compatibility:
          payload?.compatibility && typeof payload.compatibility === 'object'
            ? payload.compatibility
            : null,
        target_state:
          payload?.target_state && typeof payload.target_state === 'object'
            ? payload.target_state
            : null,
        warnings: Array.isArray(payload?.warnings)
          ? payload.warnings.filter((item: unknown): item is string => typeof item === 'string')
          : [],
      })
      setHandledMarketUploadRequestKey(buildMarketUploadRequestKey(request))
      setUploadFile(null)
      setUploadOpen(true)
    },
    onError: (error: any) => {
      setMarketUploadContext(null)
      setUploadPreview(null)
      setUploadGrantedPermissions([])
      clearMarketInstallQuery()
      showPluginErrorToast(resolvePluginErrorMessage(error, t, t.admin.operationFailed))
    },
  })

  const installMarketUploadMutation = useMutation({
    mutationFn: (request: {
      source: MarketPluginInstallContext['source']
      kind: string
      name: string
      version: string
      granted_permissions: string[]
      activate: boolean
      auto_start: boolean
    }) =>
      installAdminPluginFromMarket({
        source: request.source,
        kind: request.kind,
        name: request.name,
        version: request.version,
        granted_permissions: request.granted_permissions,
        activate: request.activate,
        auto_start: request.auto_start,
      }),
    onSuccess: (resp: any, request) => {
      const payload =
        resp && typeof resp === 'object' && resp.data && typeof resp.data === 'object'
          ? resp.data
          : resp
      const activateFailed = !!payload?.activate_failed
      if (activateFailed) {
        showPluginErrorToast(
          resolvePluginResponseMessage(payload, t, t.admin.pluginUploadActivateFailed)
        )
      } else {
        toast.success(t.admin.pluginUploadSuccess)
      }
      setLatestPluginOperation(
        buildPluginOperationFeedback('market_install', payload, {
          sourceLabel: request.source.name || request.source.source_id,
          fallbackPluginName: request.name,
          fallbackVersion: request.version,
        })
      )
      queryClient.invalidateQueries({ queryKey: ['adminPlugins'] })
      invalidatePluginBootstrapMenus()
      resetUploadDialogState(true)
    },
    onError: (error: any) =>
      showPluginErrorToast(resolvePluginErrorMessage(error, t, t.admin.pluginUploadFailed)),
  })

  useEffect(() => {
    if (!marketUploadRequestKey) {
      if (handledMarketUploadRequestKey) {
        setHandledMarketUploadRequestKey('')
      }
      return
    }
  }, [handledMarketUploadRequestKey, marketUploadRequestKey])

  useEffect(() => {
    if (!canManage || !marketUploadRequest || !marketUploadRequestKey) {
      return
    }
    if (handledMarketUploadRequestKey === marketUploadRequestKey) {
      return
    }
    const currentContextKey = marketUploadContext
      ? buildMarketUploadRequestKey({
          source: marketUploadContext.source,
          kind: marketUploadContext.coordinates.kind,
          name: marketUploadContext.coordinates.name,
          version: marketUploadContext.coordinates.version,
        })
      : ''
    if (previewMarketUploadMutation.isPending || currentContextKey === marketUploadRequestKey) {
      return
    }
    previewMarketUploadMutation.mutate(marketUploadRequest)
  }, [
    canManage,
    handledMarketUploadRequestKey,
    marketUploadContext,
    marketUploadRequest,
    marketUploadRequestKey,
    previewMarketUploadMutation,
  ])

  const lifecycleMutation = useMutation<any, any, LifecyclePayload>({
    mutationFn: ({ pluginId, action }) => pluginLifecycleAction(pluginId, { action }),
    onSuccess: (_resp, payload) => {
      toast.success(
        payload.action === 'hot_reload'
          ? t.admin.pluginHotReloadSuccess
          : t.admin.pluginLifecycleRunSuccess
      )
      queryClient.invalidateQueries({ queryKey: ['adminPlugins'] })
      queryClient.invalidateQueries({ queryKey: ['adminPluginVersions', payload.pluginId] })
      queryClient.invalidateQueries({ queryKey: ['adminPluginDiagnostics', payload.pluginId] })
      invalidatePluginBootstrapMenus()
    },
    onError: (error: any) =>
      showPluginErrorToast(resolvePluginErrorMessage(error, t, t.admin.pluginLifecycleRunFailed)),
  })

  const testMutation = useMutation({
    mutationFn: (pluginId: number) => testAdminPlugin(pluginId),
    onSuccess: (resp: any) => {
      if (resp?.success) toast.success(t.admin.pluginTestOk)
      else showPluginErrorToast(resolvePluginResponseMessage(resp, t, t.admin.pluginTestFail))
      queryClient.invalidateQueries({ queryKey: ['adminPlugins'] })
    },
    onError: (error: any) =>
      showPluginErrorToast(resolvePluginErrorMessage(error, t, t.admin.pluginTestFail)),
  })

  const workspaceTerminalMutation = useMutation<
    any,
    any,
    {
      pluginId: number
      line?: string
    }
  >({
    mutationFn: ({ pluginId, ...payload }) =>
      enterAdminPluginWorkspaceTerminalLine(pluginId, payload),
    onSuccess: (resp, payload) => {
      if (resp?.success === false) {
        showPluginErrorToast(resolvePluginResponseMessage(resp, t, t.admin.operationFailed))
      }
      queryClient.invalidateQueries({ queryKey: ['adminPluginWorkspace', payload.pluginId] })
      queryClient.invalidateQueries({ queryKey: ['adminPluginExecutions', payload.pluginId] })
      queryClient.invalidateQueries({ queryKey: ['adminPluginDiagnostics', payload.pluginId] })
    },
    onError: (error: any) =>
      showPluginErrorToast(resolvePluginErrorMessage(error, t, t.admin.operationFailed)),
  })
  const workspaceSignalMutation = useMutation<
    any,
    any,
    {
      pluginId: number
      task_id?: string
      signal?: string
    }
  >({
    mutationFn: ({ pluginId, ...payload }) => signalAdminPluginWorkspace(pluginId, payload),
    onSuccess: (_resp, payload) => {
      queryClient.invalidateQueries({ queryKey: ['adminPluginWorkspace', payload.pluginId] })
      queryClient.invalidateQueries({ queryKey: ['adminPluginDiagnostics', payload.pluginId] })
      queryClient.invalidateQueries({ queryKey: ['adminPluginExecutions', payload.pluginId] })
    },
    onError: (error: any) =>
      showPluginErrorToast(resolvePluginErrorMessage(error, t, t.admin.operationFailed)),
  })

  const activateMutation = useMutation<any, any, ActivatePayload>({
    mutationFn: ({ pluginId, versionId, autoStart }) =>
      activateAdminPluginVersion(pluginId, versionId, { auto_start: autoStart }),
    onSuccess: (resp, payload) => {
      toast.success(t.admin.pluginHotUpdateSuccess)
      setLatestPluginOperation(
        buildPluginOperationFeedback('activate', resp, {
          fallbackPluginId: payload.pluginId,
          autoStart: payload.autoStart,
        })
      )
      queryClient.invalidateQueries({ queryKey: ['adminPlugins'] })
      queryClient.invalidateQueries({ queryKey: ['adminPluginVersions', payload.pluginId] })
      queryClient.invalidateQueries({ queryKey: ['adminPluginDiagnostics', payload.pluginId] })
      invalidatePluginBootstrapMenus()
    },
    onError: (error: any) =>
      showPluginErrorToast(
        resolvePluginErrorMessage(error, t, t.admin.pluginVersionActivateFailed)
      ),
  })

  const deleteVersionMutation = useMutation<any, any, { pluginId: number; versionId: number }>({
    mutationFn: ({ pluginId, versionId }) => deleteAdminPluginVersion(pluginId, versionId),
    onSuccess: (_resp, payload) => {
      toast.success(t.admin.pluginVersionDeleteSuccess)
      queryClient.invalidateQueries({ queryKey: ['adminPlugins'] })
      queryClient.invalidateQueries({ queryKey: ['adminPluginVersions', payload.pluginId] })
      invalidatePluginBootstrapMenus()
    },
    onError: (error: any) =>
      showPluginErrorToast(resolvePluginErrorMessage(error, t, t.admin.pluginVersionDeleteFailed)),
  })

  const lifecycleLabel: Record<string, string> = {
    draft: t.admin.pluginLifecycleDraft,
    uploaded: t.admin.pluginLifecycleUploaded,
    installed: t.admin.pluginLifecycleInstalled,
    running: t.admin.pluginLifecycleRunning,
    paused: t.admin.pluginLifecyclePaused,
    degraded: t.admin.pluginLifecycleDegraded,
    retired: t.admin.pluginLifecycleRetired,
  }

  const healthLabel: Record<string, string> = {
    healthy: t.admin.pluginStatusHealthy,
    unhealthy: t.admin.pluginStatusUnhealthy,
    unknown: t.admin.pluginStatusUnknown,
  }

  const resolveOperationStatusLabel = (value?: string): string => {
    const normalized = String(value || '')
      .trim()
      .toLowerCase()
    if (!normalized) return ''
    if (normalized === 'activated') {
      return locale === 'zh' ? '已激活' : 'Activated'
    }
    if (normalized === 'activate_failed') {
      return locale === 'zh' ? '激活失败' : 'Activation failed'
    }
    if (normalized === 'uploaded') {
      return locale === 'zh' ? '已上传' : 'Uploaded'
    }
    if (normalized === 'installed') {
      return t.admin.pluginLifecycleInstalled
    }
    if (normalized in lifecycleLabel) {
      return lifecycleLabel[normalized]
    }
    return normalized.replace(/_/g, ' ')
  }

  const latestPluginOperationTarget = useMemo(() => {
    if (!latestPluginOperation) return null
    const currentMatch =
      latestPluginOperation.pluginId !== null
        ? plugins.find((item) => item.id === latestPluginOperation.pluginId) || null
        : null
    return currentMatch || latestPluginOperation.pluginSnapshot || null
  }, [latestPluginOperation, plugins])

  const latestPluginOperationWarningsPreview = useMemo(() => {
    if (!latestPluginOperation?.warnings?.length) return []
    return latestPluginOperation.warnings.slice(0, 3)
  }, [latestPluginOperation])
  const latestPluginOperationHiddenWarningCount = useMemo(() => {
    if (!latestPluginOperation?.warnings?.length) return 0
    return Math.max(
      0,
      latestPluginOperation.warnings.length - latestPluginOperationWarningsPreview.length
    )
  }, [latestPluginOperation, latestPluginOperationWarningsPreview.length])
  const adminPluginsPluginContext = useMemo(() => {
    const enabledCount = plugins.filter((item) => item.enabled).length
    const jsWorkerCount = plugins.filter((item) => item.runtime === 'js_worker').length
    const grpcCount = plugins.filter((item) => item.runtime === 'grpc').length
    const runningCount = plugins.filter(
      (item) =>
        String(item.lifecycle_status || '')
          .trim()
          .toLowerCase() === 'running'
    ).length
    const degradedCount = plugins.filter(
      (item) =>
        String(item.lifecycle_status || '')
          .trim()
          .toLowerCase() === 'degraded'
    ).length
    const errorCount = plugins.filter((item) => String(item.last_error || '').trim() !== '').length
    return {
      view: 'admin_plugins',
      summary: {
        total: plugins.length,
        enabled_count: enabledCount,
        disabled_count: Math.max(plugins.length - enabledCount, 0),
        js_worker_count: jsWorkerCount,
        grpc_count: grpcCount,
        running_count: runningCount,
        degraded_count: degradedCount,
        error_count: errorCount,
      },
      query: {
        loading: pluginsQuery.isLoading,
        fetching: pluginsQuery.isFetching,
      },
      dialogs: {
        editor_open: editorOpen,
        upload_open: uploadOpen,
        versions_open: Boolean(versionPlugin),
        diagnostics_open: Boolean(diagnosticPlugin),
        logs_open: Boolean(logPlugin),
      },
      latest_operation: latestPluginOperation
        ? {
            plugin_id: latestPluginOperation.pluginId,
            plugin_name: latestPluginOperation.pluginName,
            version: latestPluginOperation.version,
            tone: latestPluginOperation.tone,
            status: latestPluginOperation.status,
            lifecycle_status: latestPluginOperation.lifecycleStatus,
            health_status: latestPluginOperation.healthStatus,
            warning_count: latestPluginOperation.warnings.length,
            occurred_at: latestPluginOperation.occurredAt,
          }
        : undefined,
    }
  }, [
    diagnosticPlugin,
    editorOpen,
    latestPluginOperation,
    logPlugin,
    plugins,
    pluginsQuery.isFetching,
    pluginsQuery.isLoading,
    uploadOpen,
    versionPlugin,
  ])

  const openCreate = () => {
    setEditingPlugin(null)
    const nextForm = { ...EMPTY_PLUGIN_FORM, capabilities: DEFAULT_CAPABILITIES_TEMPLATE }
    setPluginForm(nextForm)
    setPluginHookAccessState(parsePluginHookAccessState(nextForm.capabilities))
    setPluginCapabilityPolicyState(parsePluginCapabilityPolicyState(nextForm.capabilities))
    setPluginFrontendAccessState(parsePluginFrontendAccessState(nextForm.capabilities))
    setPluginSaveBaseline(serializePluginSaveRequestData(buildPluginSaveRequestData(nextForm)))
    setPluginSecretDrafts({})
    setPluginSecretDeleteKeys([])
    setEditorOpen(true)
  }

  const openEdit = (plugin: AdminPlugin) => {
    const metadata = resolvePluginDisplayMetadata(plugin, locale)
    const nextForm: PluginForm = {
      name: plugin.name || '',
      display_name: metadata.displayName,
      description: metadata.description,
      type: plugin.type || '',
      runtime: plugin.runtime || 'grpc',
      package_path: plugin.package_path || '',
      address: plugin.address || '',
      version: plugin.version || '0.0.0',
      config: stringifyJSONForTextarea(plugin.config, '{}', formatHumanReadableJSONText),
      runtime_params: stringifyJSONForTextarea(
        plugin.runtime_params,
        '{}',
        formatHumanReadableJSONText
      ),
      capabilities: stringifyJSONForTextarea(
        plugin.capabilities,
        DEFAULT_CAPABILITIES_TEMPLATE,
        formatCapabilitiesForDisplay
      ),
      enabled: !!plugin.enabled,
    }
    setEditingPlugin(plugin)
    setPluginForm(nextForm)
    setPluginHookAccessState(parsePluginHookAccessState(nextForm.capabilities))
    setPluginCapabilityPolicyState(parsePluginCapabilityPolicyState(nextForm.capabilities))
    setPluginFrontendAccessState(parsePluginFrontendAccessState(nextForm.capabilities))
    setPluginSaveBaseline(serializePluginSaveRequestData(buildPluginSaveRequestData(nextForm)))
    setPluginSecretDrafts({})
    setPluginSecretDeleteKeys([])
    setEditorOpen(true)
  }

  const openUpload = (plugin?: AdminPlugin) => {
    const metadata = plugin ? resolvePluginDisplayMetadata(plugin, locale) : null
    const nextForm: UploadForm = plugin
      ? {
          plugin_id: String(plugin.id),
          name: plugin.name || '',
          display_name: metadata?.displayName || '',
          description: metadata?.description || '',
          type: plugin.type || '',
          runtime: plugin.runtime || 'grpc',
          address: plugin.address || '',
          version: plugin.version || '',
          config: stringifyJSONForTextarea(plugin.config, '', formatHumanReadableJSONText),
          runtime_params: stringifyJSONForTextarea(
            plugin.runtime_params,
            '{}',
            formatHumanReadableJSONText
          ),
          capabilities: stringifyJSONForTextarea(
            plugin.capabilities,
            DEFAULT_CAPABILITIES_TEMPLATE,
            formatCapabilitiesForDisplay
          ),
          changelog: '',
          activate: true,
          auto_start: false,
        }
      : { ...EMPTY_UPLOAD_FORM, capabilities: DEFAULT_CAPABILITIES_TEMPLATE }

    if (plugin) {
      setUploadForm(nextForm)
    } else {
      setUploadForm(nextForm)
    }
    setUploadHookAccessState(parsePluginHookAccessState(nextForm.capabilities))
    setUploadCapabilityPolicyState(parsePluginCapabilityPolicyState(nextForm.capabilities))
    setUploadFrontendAccessState(parsePluginFrontendAccessState(nextForm.capabilities))
    setMarketUploadContext(null)
    setUploadFile(null)
    setUploadPreview(null)
    setUploadGrantedPermissions([])
    setUploadInputKey((n) => n + 1)
    setUploadOpen(true)
    clearMarketInstallQuery()
  }

  const handleUploadFilePicked = (file: File | null) => {
    setUploadFile(file)
    setUploadPreview(null)
    setUploadGrantedPermissions([])
    if (file) {
      previewUploadMutation.mutate(file)
    }
  }

  const submitPlugin = () => {
    const name = pluginForm.name.trim()
    const type = pluginForm.type.trim()
    const runtime = pluginForm.runtime.trim()
    const packagePath = pluginForm.package_path.trim()
    const address = pluginForm.address.trim()
    const originalRuntime = String(editingPlugin?.runtime || 'grpc').trim()
    const originalAddress = String(editingPlugin?.address || '').trim()
    const originalPackagePath = String(editingPlugin?.package_path || '').trim()
    const requirePackagePath =
      isJSWorkerRuntime(runtime) &&
      (!editingPlugin ||
        runtime !== originalRuntime ||
        address !== originalAddress ||
        packagePath !== originalPackagePath)
    if (!name) return toast.error(t.admin.pluginRequiredName)
    if (!type) return toast.error(t.admin.pluginRequiredType)
    if (!runtime) return toast.error(t.admin.pluginRequiredRuntime)
    if (requirePackagePath && !packagePath) {
      return toast.error(t.admin.pluginRequiredPackagePath)
    }
    if (!address) {
      if (isJSWorkerRuntime(runtime)) {
        return toast.error(t.admin.pluginRequiredEntryScript)
      }
      return toast.error(t.admin.pluginRequiredAddress)
    }

    try {
      parseObject(pluginForm.config)
      parseObject(pluginForm.runtime_params)
      parseObject(pluginForm.capabilities)
      const frontendValidation = validatePluginFrontendAccessState(pluginFrontendAccessState, t)
      if (frontendValidation) {
        toast.error(frontendValidation)
        return
      }
      const saveData = buildPluginSaveRequestData(pluginForm)
      const secretPatch = buildPluginSecretPatch(pluginSecretDrafts, pluginSecretDeleteKeys)
      const pluginChanged =
        !editingPlugin ||
        serializePluginSaveRequestData(saveData) !== String(pluginSaveBaseline || '')
      if (editingPlugin && !pluginChanged && !secretPatch) {
        handleEditorOpenChange(false)
        return
      }
      saveMutation.mutate({
        pluginId: editingPlugin?.id,
        data: saveData,
        baseline: pluginSaveBaseline,
        secretPatch,
      })
    } catch {
      toast.error(t.admin.pluginSaveInvalidJson)
    }
  }

  const submitUpload = () => {
    if (marketUploadContext) {
      if (
        uploadPreview &&
        Array.isArray(uploadPreview.requested_permissions) &&
        uploadPreview.requested_permissions.length > 0
      ) {
        const granted = normalizeStringList(uploadGrantedPermissions)
        const missingRequired = uploadPreview.requested_permissions
          .filter((item) => !!item.required)
          .map((item) => (item.key || '').trim().toLowerCase())
          .filter((item) => item && !granted.includes(item))
        if (missingRequired.length > 0) {
          return toast.error(
            t.admin.pluginPermissionMissingRequired.replace('{list}', missingRequired.join(', '))
          )
        }
      }
      installMarketUploadMutation.mutate({
        source: marketUploadContext.source,
        kind: marketUploadContext.coordinates.kind,
        name: marketUploadContext.coordinates.name,
        version: marketUploadContext.coordinates.version,
        granted_permissions: normalizeStringList(uploadGrantedPermissions),
        activate: uploadForm.activate,
        auto_start: uploadForm.auto_start,
      })
      return
    }
    if (!uploadFile) return toast.error(t.admin.pluginUploadFile)
    try {
      if (uploadForm.config.trim()) parseObject(uploadForm.config)
      if (uploadForm.runtime_params.trim()) parseObject(uploadForm.runtime_params)
      if (uploadForm.capabilities.trim()) parseObject(uploadForm.capabilities)
    } catch {
      return toast.error(t.admin.pluginSaveInvalidJson)
    }
    const uploadFrontendValidation = validatePluginFrontendAccessState(uploadFrontendAccessState, t)
    if (uploadFrontendValidation) {
      return toast.error(uploadFrontendValidation)
    }
    if (uploadConflictSummary.hasConflict) {
      return toast.error(t.admin.pluginUploadConflictDetected)
    }
    if (
      uploadPreview &&
      Array.isArray(uploadPreview.requested_permissions) &&
      uploadPreview.requested_permissions.length > 0
    ) {
      const granted = normalizeStringList(uploadGrantedPermissions)
      const missingRequired = uploadPreview.requested_permissions
        .filter((item) => !!item.required)
        .map((item) => (item.key || '').trim().toLowerCase())
        .filter((item) => item && !granted.includes(item))
      if (missingRequired.length > 0) {
        return toast.error(
          t.admin.pluginPermissionMissingRequired.replace('{list}', missingRequired.join(', '))
        )
      }
    }

    const formData = new FormData()
    formData.append('file', uploadFile)
    if (uploadForm.plugin_id.trim()) formData.append('plugin_id', uploadForm.plugin_id.trim())
    if (uploadForm.name.trim()) formData.append('name', uploadForm.name.trim())
    if (uploadForm.display_name.trim())
      formData.append('display_name', uploadForm.display_name.trim())
    if (uploadForm.description.trim()) formData.append('description', uploadForm.description.trim())
    if (uploadForm.type.trim()) formData.append('type', uploadForm.type.trim())
    if (uploadForm.runtime.trim()) formData.append('runtime', uploadForm.runtime.trim())
    if (uploadForm.address.trim()) formData.append('address', uploadForm.address.trim())
    if (uploadForm.version.trim()) formData.append('version', uploadForm.version.trim())
    if (uploadForm.config.trim()) formData.append('config', uploadForm.config.trim())
    if (uploadForm.runtime_params.trim())
      formData.append('runtime_params', uploadForm.runtime_params.trim())
    if (uploadForm.capabilities.trim())
      formData.append('capabilities', uploadForm.capabilities.trim())
    if (
      uploadPreview &&
      Array.isArray(uploadPreview.requested_permissions) &&
      uploadPreview.requested_permissions.length > 0
    ) {
      formData.append(
        'granted_permissions',
        JSON.stringify(normalizeStringList(uploadGrantedPermissions))
      )
    }
    if (uploadForm.changelog.trim()) formData.append('changelog', uploadForm.changelog.trim())
    formData.append('activate', String(uploadForm.activate))
    formData.append('auto_start', String(uploadForm.auto_start))

    uploadMutation.mutate(formData)
  }

  if (permissionReady && !canManage) {
    return (
      <div className="space-y-6">
        <h1 className="text-3xl font-bold">{t.pageTitle.adminPlugins}</h1>
        <Card>
          <CardContent className="py-10 text-center text-muted-foreground">
            {t.admin.pluginNoPermission}
          </CardContent>
        </Card>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <PluginSlot slot="admin.plugins.top" context={adminPluginsPluginContext} />
      {latestPluginOperation ? (
        <Card
          className={
            latestPluginOperation.tone === 'warning'
              ? 'border-amber-300 bg-amber-50/70 dark:border-amber-500/40 dark:bg-amber-950/30'
              : 'border-emerald-300 bg-emerald-50/70 dark:border-emerald-500/40 dark:bg-emerald-950/30'
          }
        >
          <CardContent className="space-y-3 p-4">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div className="min-w-0 flex-1 space-y-2">
                <p className="text-xs text-muted-foreground">
                  {[
                    t.admin.pluginLatestResult,
                    latestPluginOperation.tone === 'warning' ? t.common.warning : t.common.success,
                    latestPluginOperation.status
                      ? resolveOperationStatusLabel(latestPluginOperation.status)
                      : null,
                    latestPluginOperation.warnings.length > 0
                      ? `${t.common.warning}: ${latestPluginOperation.warnings.length}`
                      : null,
                  ]
                    .filter(Boolean)
                    .join(' · ')}
                </p>
                <div className="space-y-1">
                  <p className="text-sm font-semibold text-foreground">
                    {latestPluginOperation.title}
                  </p>
                  <p className="break-words text-sm text-foreground/90">
                    {latestPluginOperation.summary}
                  </p>
                  <p className="pt-1 text-xs text-muted-foreground">
                    {[
                      latestPluginOperation.pluginName,
                      `${t.admin.pluginVersionLabel}: ${latestPluginOperation.version}`,
                      latestPluginOperation.sourceLabel
                        ? `${t.admin.pluginMarketInstallSource}: ${latestPluginOperation.sourceLabel}`
                        : `${t.admin.pluginLatestResultPluginId}: ${
                            latestPluginOperation.pluginId !== null
                              ? String(latestPluginOperation.pluginId)
                              : '-'
                          }`,
                      latestPluginOperation.lifecycleStatus
                        ? `${t.admin.pluginLifecycle}: ${
                            lifecycleLabel[latestPluginOperation.lifecycleStatus] ||
                            latestPluginOperation.lifecycleStatus
                          }`
                        : null,
                      latestPluginOperation.healthStatus
                        ? `${t.admin.pluginDiagnosticsHealth}: ${
                            healthLabel[latestPluginOperation.healthStatus] ||
                            latestPluginOperation.healthStatus
                          }`
                        : null,
                      `${t.admin.pluginLatestResultRecordedAt}: ${formatDateTime(
                        latestPluginOperation.occurredAt,
                        locale
                      )}`,
                    ]
                      .filter(Boolean)
                      .join(' · ')}
                  </p>
                </div>
              </div>
              <div className="flex flex-wrap items-center gap-2">
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  disabled={!latestPluginOperationTarget?.id}
                  onClick={() => {
                    if (!latestPluginOperationTarget?.id) return
                    setVersionPlugin(latestPluginOperationTarget)
                  }}
                >
                  {t.admin.pluginVersions}
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  disabled={!latestPluginOperationTarget?.id}
                  onClick={() => {
                    if (!latestPluginOperationTarget?.id) return
                    setDiagnosticPlugin(latestPluginOperationTarget)
                  }}
                >
                  {t.admin.pluginDiagnostics}
                </Button>
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  onClick={() => setLatestPluginOperation(null)}
                >
                  {t.common.close}
                </Button>
              </div>
            </div>
            {latestPluginOperation.detail || latestPluginOperationWarningsPreview.length > 0 ? (
              <details className="rounded-md border border-input/60 bg-background/75 p-3">
                <summary className="cursor-pointer text-sm font-medium">
                  {t.admin.pluginLatestResultDetailTitle}
                </summary>
                <div className="mt-3 space-y-3">
                  {latestPluginOperation.detail ? (
                    <p className="break-words text-xs text-muted-foreground">
                      {latestPluginOperation.detail}
                    </p>
                  ) : null}
                  {latestPluginOperationWarningsPreview.length > 0 ? (
                    <div className="space-y-2 rounded-md border border-amber-200/70 bg-amber-50/40 p-3 dark:border-amber-500/30 dark:bg-amber-950/20">
                      <p className="text-xs font-medium text-foreground">
                        {t.common.warning}
                        {latestPluginOperation.warnings.length > 1
                          ? ` (${latestPluginOperation.warnings.length})`
                          : ''}
                      </p>
                      <div className="space-y-1">
                        {latestPluginOperationWarningsPreview.map((warning, index) => (
                          <p
                            key={`${warning}-${index}`}
                            className="break-words text-xs text-muted-foreground"
                          >
                            {warning}
                          </p>
                        ))}
                        {latestPluginOperationHiddenWarningCount > 0 ? (
                          <p className="text-xs text-muted-foreground">
                            {t.admin.pluginLatestResultWarningsMore.replace(
                              '{count}',
                              String(latestPluginOperationHiddenWarningCount)
                            )}
                          </p>
                        ) : null}
                      </div>
                    </div>
                  ) : null}
                </div>
              </details>
            ) : null}
          </CardContent>
        </Card>
      ) : null}

      <PluginListPanel
        t={t}
        locale={locale}
        pluginsQueryLoading={pluginsQuery.isLoading}
        pluginsQueryFetching={pluginsQuery.isFetching}
        plugins={plugins}
        lifecycleLabel={lifecycleLabel}
        healthLabel={healthLabel}
        formatDateTime={formatDateTime}
        runtimeLabel={runtimeLabel}
        runtimeAddressLabel={runtimeAddressLabel}
        resolvePluginLifecycleActionState={resolvePluginLifecycleActionState}
        isLifecycleBusy={(pluginId) =>
          lifecycleMutation.isPending && lifecycleMutation.variables?.pluginId === pluginId
        }
        isDeleteBusy={(pluginId) => deleteMutation.isPending && deletePlugin?.id === pluginId}
        isTestBusy={(pluginId) => testMutation.isPending && testMutation.variables === pluginId}
        onRefresh={() => pluginsQuery.refetch()}
        onOpenUpload={openUpload}
        onOpenCreate={openCreate}
        onLifecycleAction={(pluginId, action) => lifecycleMutation.mutate({ pluginId, action })}
        onOpenVersions={setVersionPlugin}
        onTest={(pluginId) => testMutation.mutate(pluginId)}
        onOpenWorkspace={setWorkspacePlugin}
        onOpenDiagnostics={setDiagnosticPlugin}
        onOpenLogs={setLogPlugin}
        onOpenEdit={openEdit}
        onOpenDelete={setDeletePlugin}
      />

      <PluginEditorDialog
        open={editorOpen}
        onOpenChange={handleEditorOpenChange}
        editingPlugin={editingPlugin}
        pluginForm={pluginForm}
        setPluginForm={setPluginForm}
        runtimeOptions={RUNTIME_OPTIONS}
        runtimeLabel={runtimeLabel}
        runtimeAddressLabel={runtimeAddressLabel}
        runtimeAddressPlaceholder={runtimeAddressPlaceholder}
        runtimeAddressHint={runtimeAddressHint}
        configSchema={editingConfigSchema}
        secretSchema={editingSecretSchema}
        secretMeta={editingSecretMetaMap}
        secretDrafts={pluginSecretDrafts}
        secretDeleteKeys={pluginSecretDeleteKeys}
        onSecretDraftChange={handlePluginSecretDraftChange}
        onSecretDeleteToggle={handlePluginSecretDeleteToggle}
        runtimeParamsSchema={editingRuntimeParamsSchema}
        hookCatalog={hookCatalogGroups}
        hookAccessState={pluginHookAccessState}
        onHookAccessChange={handlePluginHookAccessChange}
        resolveHookGroupLabel={resolvePluginHookGroupLabel}
        capabilityPermissionOptions={capabilityPermissionOptions}
        capabilityPolicyState={pluginCapabilityPolicyState}
        onCapabilityPolicyChange={handlePluginCapabilityPolicyChange}
        frontendSlotCatalog={frontendSlotCatalog}
        frontendPermissionCatalog={frontendPermissionCatalog}
        frontendAccessState={pluginFrontendAccessState}
        onFrontendAccessChange={handlePluginFrontendAccessChange}
        resolveFrontendSlotGroupLabel={resolveFrontendSlotGroupLabel}
        frontendValidationMessage={pluginFrontendValidationMessage}
        capabilitiesValid={pluginCapabilitiesValid}
        configValid={pluginConfigValid}
        runtimeParamsValid={pluginRuntimeParamsValid}
        onCapabilitiesChange={handlePluginCapabilitiesChange}
        onConfigBlur={handlePluginConfigBlur}
        onRuntimeParamsBlur={handlePluginRuntimeParamsBlur}
        onCapabilitiesBlur={handlePluginCapabilitiesBlur}
        submitPlugin={submitPlugin}
        isSaving={saveMutation.isPending}
        t={t}
      />

      <PluginUploadDialog
        open={uploadOpen}
        onOpenChange={(open) => {
          if (open) {
            setUploadOpen(true)
            return
          }
          resetUploadDialogState(!!marketUploadContext)
        }}
        uploadInputKey={uploadInputKey}
        uploadFile={uploadFile}
        onUploadFilePicked={handleUploadFilePicked}
        previewPending={previewUploadMutation.isPending || previewMarketUploadMutation.isPending}
        uploadPending={uploadMutation.isPending || installMarketUploadMutation.isPending}
        uploadPreview={uploadPreview}
        uploadConflictSummary={uploadConflictSummary}
        uploadGrantedPermissions={uploadGrantedPermissions}
        setUploadGrantedPermissions={setUploadGrantedPermissions}
        normalizeStringList={normalizeStringList}
        resolvePluginPermissionTitle={resolvePluginPermissionTitle}
        resolvePluginPermissionDescription={resolvePluginPermissionDescription}
        uploadForm={uploadForm}
        setUploadForm={setUploadForm}
        plugins={plugins}
        runtimeOptions={RUNTIME_OPTIONS}
        runtimeLabel={runtimeLabel}
        runtimeAddressLabel={runtimeAddressLabel}
        runtimeAddressPlaceholder={runtimeAddressPlaceholder}
        runtimeAddressHint={runtimeAddressHint}
        configSchema={uploadConfigSchema}
        runtimeParamsSchema={uploadRuntimeParamsSchema}
        hookCatalog={hookCatalogGroups}
        hookAccessState={uploadHookAccessState}
        onHookAccessChange={handleUploadHookAccessChange}
        resolveHookGroupLabel={resolvePluginHookGroupLabel}
        capabilityPermissionOptions={capabilityPermissionOptions}
        capabilityPolicyState={uploadCapabilityPolicyState}
        onCapabilityPolicyChange={handleUploadCapabilityPolicyChange}
        frontendSlotCatalog={frontendSlotCatalog}
        frontendPermissionCatalog={frontendPermissionCatalog}
        frontendAccessState={uploadFrontendAccessState}
        onFrontendAccessChange={handleUploadFrontendAccessChange}
        resolveFrontendSlotGroupLabel={resolveFrontendSlotGroupLabel}
        frontendValidationMessage={uploadFrontendValidationMessage}
        capabilitiesValid={uploadCapabilitiesValid}
        configValid={uploadConfigValid}
        runtimeParamsValid={uploadRuntimeParamsValid}
        onCapabilitiesChange={handleUploadCapabilitiesChange}
        onConfigBlur={handleUploadConfigBlur}
        onRuntimeParamsBlur={handleUploadRuntimeParamsBlur}
        onCapabilitiesBlur={handleUploadCapabilitiesBlur}
        submitUpload={submitUpload}
        marketInstallContext={marketUploadContext}
        locale={locale}
        t={t}
      />

      <PluginVersionsDialog
        open={!!versionPlugin}
        onOpenChange={(open) => {
          if (!open) {
            setVersionPlugin(null)
            setActivateAutoStart(false)
          }
        }}
        versionPlugin={versionPlugin}
        activateAutoStart={activateAutoStart}
        setActivateAutoStart={setActivateAutoStart}
        versionsLoading={versionsQuery.isLoading}
        versions={versions}
        isActivating={(pluginId, versionId) =>
          activateMutation.isPending &&
          activateMutation.variables?.pluginId === pluginId &&
          activateMutation.variables?.versionId === versionId
        }
        isDeleting={(pluginId, versionId) =>
          deleteVersionMutation.isPending &&
          deleteVersionMutation.variables?.pluginId === pluginId &&
          deleteVersionMutation.variables?.versionId === versionId
        }
        activateVersion={(pluginId, versionId, autoStart) => {
          activateMutation.mutate({ pluginId, versionId, autoStart })
        }}
        deleteVersion={(pluginId, versionId) => {
          deleteVersionMutation.mutate({ pluginId, versionId })
        }}
        formatDateTime={formatDateTime}
        locale={locale}
        t={t}
      />

      <PluginExecutionLogsDialog
        open={!!logPlugin}
        onOpenChange={(open) => (!open ? setLogPlugin(null) : null)}
        plugin={logPlugin}
        logsLoading={logsQuery.isLoading}
        logs={logs}
        formatDateTime={formatDateTime}
        locale={locale}
        t={t}
      />

      <PluginWorkspaceDialog
        open={!!workspacePlugin}
        onOpenChange={(open) => (!open ? setWorkspacePlugin(null) : null)}
        plugin={workspacePlugin}
        workspace={workspaceSnapshot}
        workspaceLoading={workspaceQuery.isLoading || workspaceQuery.isFetching}
        terminalSubmitting={workspaceTerminalMutation.isPending}
        signaling={workspaceSignalMutation.isPending}
        onSubmitTerminalLine={(payload) => {
          if (!workspacePlugin) return
          workspaceTerminalMutation.mutate({
            pluginId: workspacePlugin.id,
            ...payload,
          })
        }}
        onSignal={(payload) => {
          if (!workspacePlugin) return
          workspaceSignalMutation.mutate({
            pluginId: workspacePlugin.id,
            ...payload,
          })
        }}
        onRefresh={() => {
          if (!workspacePlugin) return
          queryClient.invalidateQueries({ queryKey: ['adminPluginWorkspace', workspacePlugin.id] })
        }}
        formatDateTime={formatDateTime}
        locale={locale}
        t={t}
      />

      <PluginDiagnosticDialog
        open={!!diagnosticPlugin}
        onOpenChange={(open) => (!open ? setDiagnosticPlugin(null) : null)}
        plugin={diagnosticPlugin}
        diagnosticsLoading={diagnosticsQuery.isLoading}
        diagnostics={pluginDiagnostics}
        locale={locale}
        cancelingTaskID={
          cancelTaskMutation.isPending ? (cancelTaskMutation.variables?.taskId ?? null) : null
        }
        onCancelTask={(taskID) => {
          if (!diagnosticPlugin) return
          cancelTaskMutation.mutate({
            pluginId: diagnosticPlugin.id,
            taskId: taskID,
          })
        }}
        t={t}
      />

      <PluginDeleteAlert
        deletePlugin={deletePlugin}
        setDeletePlugin={setDeletePlugin}
        deletePending={deleteMutation.isPending}
        onConfirmDelete={(pluginId) => deleteMutation.mutate(pluginId)}
        t={t}
      />
    </div>
  )
}

export default function AdminPluginsPage() {
  return (
    <Suspense fallback={<div className="min-h-[40vh]" />}>
      <AdminPluginsPageContent />
    </Suspense>
  )
}
