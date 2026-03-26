'use client'

import Link from 'next/link'
import { Children, useMemo, useState, type ReactNode } from 'react'

import {
  Activity,
  ChevronDown,
  ChevronRight,
  FileUp,
  Loader2,
  MoreHorizontal,
  Pause,
  Pencil,
  Play,
  Plus,
  RefreshCw,
  RotateCcw,
  ShieldCheck,
  SlidersHorizontal,
  TerminalSquare,
  TestTube2,
  Trash2,
  Waypoints,
} from 'lucide-react'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import type { AdminPlugin, AdminPluginDeployment } from '@/lib/api'
import type { Translations } from '@/lib/i18n'
import { usePluginBootstrapQuery } from '@/lib/plugin-bootstrap-query'
import { findAdminMarketPluginBasePath } from '@/lib/plugin-market-route'
import { manifestString } from '@/lib/package-manifest-schema'

import type { PluginLifecycleActionState } from './types'

type PluginCapabilitySummary = {
  allowAllHooks: boolean
  allowedHookCount: number
  disabledHookCount: number
  requestedPermissionCount: number
  grantedPermissionCount: number
  requestedPermissions: string[]
  grantedPermissions: string[]
  frontendEnabled: boolean
  frontendMinScope: 'guest' | 'authenticated' | 'super_admin'
  frontendAreas: string[]
  frontendSlots: string[]
}

type PluginManifestSchemaSummary = {
  configSchemaFieldCount: number
  runtimeParamsSchemaFieldCount: number
}

type PluginAdminPageSummary = {
  path: string
}

type PluginAttentionIssueCode =
  | 'unhealthy'
  | 'last_error'
  | 'deployment_failed'
  | 'permission_gap'
  | 'generation_drift'
  | 'failure_count'

type PluginAttentionSummary = {
  needsAttention: boolean
  score: number
  issues: PluginAttentionIssueCode[]
  permissionGapCount: number
}

type PluginListRow = {
  plugin: AdminPlugin
  localizedDisplayName: string
  localizedDescription: string
  capabilitySummary: PluginCapabilitySummary
  manifestSummary: PluginManifestSchemaSummary
  adminPageSummary: PluginAdminPageSummary | null
  latestDeployment?: AdminPluginDeployment
  attentionSummary: PluginAttentionSummary
}

function parseObjectText(value?: string): Record<string, any> | null {
  const trimmed = String(value || '').trim()
  if (!trimmed) return null
  try {
    const parsed = JSON.parse(trimmed)
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      return parsed as Record<string, any>
    }
  } catch {}
  return null
}

function normalizeStringList(values: unknown): string[] {
  if (!Array.isArray(values)) return []
  const seen = new Set<string>()
  const out: string[] = []
  values.forEach((item) => {
    const normalized = String(item || '')
      .trim()
      .toLowerCase()
    if (!normalized || seen.has(normalized)) return
    seen.add(normalized)
    out.push(normalized)
  })
  return out
}

function resolvePluginCapabilitySummary(plugin: AdminPlugin): PluginCapabilitySummary {
  const capabilities = parseObjectText(plugin.capabilities)
  const effectivePolicy = plugin.effective_capability_policy
  const hooks = normalizeStringList(effectivePolicy?.hooks ?? capabilities?.hooks)
  const disabledHooks = normalizeStringList(
    effectivePolicy?.disabled_hooks ?? capabilities?.disabled_hooks
  )
  const requestedPermissions = normalizeStringList(
    effectivePolicy?.requested_permissions ?? capabilities?.requested_permissions
  )
  const grantedPermissions = normalizeStringList(
    effectivePolicy?.granted_permissions ?? capabilities?.granted_permissions
  )
  const frontendAreas = normalizeStringList(
    effectivePolicy?.frontend_allowed_areas ?? capabilities?.frontend_allowed_areas
  )
  const frontendSlots = normalizeStringList(
    effectivePolicy?.allowed_frontend_slots ?? capabilities?.allowed_frontend_slots
  )
  const frontendMinScopeRaw = String(
    effectivePolicy?.frontend_min_scope ?? capabilities?.frontend_min_scope ?? ''
  )
    .trim()
    .toLowerCase()
  const allowHookExecute =
    typeof effectivePolicy?.allow_hook_execute === 'boolean'
      ? effectivePolicy.allow_hook_execute
      : true
  const frontendEnabled =
    typeof effectivePolicy?.allow_frontend_extensions === 'boolean'
      ? effectivePolicy.allow_frontend_extensions
      : capabilities?.allow_frontend_extensions !== false

  let frontendMinScope: PluginCapabilitySummary['frontendMinScope'] = 'guest'
  if (['authenticated', 'auth', 'user', 'member'].includes(frontendMinScopeRaw)) {
    frontendMinScope = 'authenticated'
  } else if (['super_admin', 'superadmin', 'root'].includes(frontendMinScopeRaw)) {
    frontendMinScope = 'super_admin'
  }

  const allowAllHooks = allowHookExecute && (hooks.length === 0 || hooks.includes('*'))
  return {
    allowAllHooks,
    allowedHookCount: allowHookExecute
      ? allowAllHooks
        ? 0
        : hooks.filter((item) => item !== '*').length
      : 0,
    disabledHookCount: disabledHooks.filter((item) => item !== '*').length,
    requestedPermissionCount: requestedPermissions.length,
    grantedPermissionCount: grantedPermissions.length,
    requestedPermissions,
    grantedPermissions,
    frontendEnabled,
    frontendMinScope,
    frontendAreas,
    frontendSlots,
  }
}

function countSchemaFields(value: unknown): number {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return 0
  const fields = (value as { fields?: unknown }).fields
  return Array.isArray(fields) ? fields.length : 0
}

function resolvePluginManifestSchemaSummary(plugin: AdminPlugin): PluginManifestSchemaSummary {
  const manifest = parseObjectText(plugin.manifest)
  return {
    configSchemaFieldCount: countSchemaFields(manifest?.config_schema),
    runtimeParamsSchemaFieldCount: countSchemaFields(manifest?.runtime_params_schema),
  }
}

function normalizePluginAdminPagePath(value: unknown): string {
  const trimmed = String(value || '').trim()
  if (!trimmed) return ''
  const normalized = trimmed.startsWith('/') ? trimmed : `/${trimmed}`
  const clean = normalized.replace(/\/+$/, '') || '/'
  if (!clean.startsWith('/admin/plugin-pages/')) {
    return ''
  }
  return clean
}

function resolvePluginAdminPageSummary(plugin: AdminPlugin): PluginAdminPageSummary | null {
  const manifest = parseObjectText(plugin.manifest)
  if (!manifest) return null

  const frontend =
    manifest.frontend && typeof manifest.frontend === 'object' && !Array.isArray(manifest.frontend)
      ? (manifest.frontend as Record<string, any>)
      : null
  const adminPage =
    frontend?.admin_page &&
    typeof frontend.admin_page === 'object' &&
    !Array.isArray(frontend.admin_page)
      ? (frontend.admin_page as Record<string, any>)
      : manifest.admin_page &&
          typeof manifest.admin_page === 'object' &&
          !Array.isArray(manifest.admin_page)
        ? (manifest.admin_page as Record<string, any>)
        : null

  const path = normalizePluginAdminPagePath(
    adminPage?.path ?? frontend?.admin_page_path ?? manifest.admin_page_path
  )
  if (!path) return null

  return { path }
}

function scopeLabel(scope: PluginCapabilitySummary['frontendMinScope'], t: Translations): string {
  switch (scope) {
    case 'authenticated':
      return t.admin.pluginFrontendMinScopeAuthenticated
    case 'super_admin':
      return t.admin.pluginFrontendMinScopeSuperAdmin
    default:
      return t.admin.pluginFrontendMinScopeGuest
  }
}

function resolvePluginLocalizedDisplayName(plugin: AdminPlugin, locale?: string): string {
  return (
    manifestString(parseObjectText(plugin.manifest), 'display_name', locale) ||
    String(plugin.display_name || '').trim() ||
    manifestString(parseObjectText(plugin.manifest), 'name', locale) ||
    String(plugin.name || '').trim()
  )
}

function resolvePluginLocalizedDescription(plugin: AdminPlugin, locale?: string): string {
  return (
    manifestString(parseObjectText(plugin.manifest), 'description', locale) ||
    String(plugin.description || '').trim()
  )
}

function deploymentStatusVariant(
  status: string | undefined
): 'default' | 'secondary' | 'destructive' | 'outline' | 'active' {
  switch (status) {
    case 'succeeded':
      return 'default'
    case 'running':
      return 'active'
    case 'failed':
    case 'rolled_back':
      return 'destructive'
    case 'pending':
      return 'secondary'
    default:
      return 'outline'
  }
}

function deploymentStatusLabel(status: string | undefined, t: Translations): string {
  switch (status) {
    case 'pending':
      return t.admin.pluginDeploymentStatusPending
    case 'running':
      return t.admin.pluginDeploymentStatusRunning
    case 'succeeded':
      return t.admin.pluginDeploymentStatusSucceeded
    case 'failed':
      return t.admin.pluginDeploymentStatusFailed
    case 'rolled_back':
      return t.admin.pluginDeploymentStatusRolledBack
    default:
      return status || '-'
  }
}

function deploymentOperationLabel(operation: string | undefined, t: Translations): string {
  switch (operation) {
    case 'hot_reload':
      return t.admin.pluginHotReload
    case 'hot_update':
      return t.admin.pluginHotUpdate
    case 'start':
      return t.admin.pluginLifecycleStart
    case 'pause':
      return t.admin.pluginLifecyclePause
    case 'restart':
      return t.admin.pluginLifecycleRestart
    case 'retire':
      return t.admin.pluginLifecycleRetire
    default:
      return operation || '-'
  }
}

function generationLabel(plugin: AdminPlugin): string {
  const desired = Number(plugin.desired_generation || 0)
  const applied = Number(plugin.applied_generation || 0)
  if (desired <= 0 && applied <= 0) return '-'
  return `${applied}/${desired}`
}

function normalizePluginFilterValue(value: unknown): string {
  return String(value || '')
    .trim()
    .toLowerCase()
}

function resolvePluginRuntimeFilterValue(plugin: AdminPlugin): string {
  return normalizePluginFilterValue(plugin.runtime || 'grpc') || 'unknown'
}

function resolvePluginLifecycleFilterValue(plugin: AdminPlugin): string {
  return normalizePluginFilterValue(plugin.lifecycle_status || 'draft') || 'draft'
}

function resolvePluginHealthFilterValue(plugin: AdminPlugin): string {
  return normalizePluginFilterValue(plugin.status || 'unknown') || 'unknown'
}

function resolvePluginAttentionSummary(
  plugin: AdminPlugin,
  capabilitySummary: PluginCapabilitySummary
): PluginAttentionSummary {
  const issues: PluginAttentionIssueCode[] = []
  const health = resolvePluginHealthFilterValue(plugin)
  const latestDeployment = plugin.latest_deployment
  const grantedPermissionSet = new Set(capabilitySummary.grantedPermissions)
  const permissionGapCount = capabilitySummary.requestedPermissions.filter(
    (item) => !grantedPermissionSet.has(item)
  ).length
  const generationDrift =
    Number(plugin.desired_generation || 0) > Number(plugin.applied_generation || 0)

  if (health === 'unhealthy') {
    issues.push('unhealthy')
  }
  if (String(plugin.last_error || '').trim() !== '') {
    issues.push('last_error')
  }
  if (
    latestDeployment &&
    ['failed', 'rolled_back'].includes(String(latestDeployment.status || '').toLowerCase())
  ) {
    issues.push('deployment_failed')
  }
  if (permissionGapCount > 0) {
    issues.push('permission_gap')
  }
  if (generationDrift) {
    issues.push('generation_drift')
  }
  if (Number(plugin.fail_count || 0) > 0) {
    issues.push('failure_count')
  }

  let score = 0
  issues.forEach((issue) => {
    switch (issue) {
      case 'unhealthy':
        score += 5
        break
      case 'last_error':
        score += 4
        break
      case 'deployment_failed':
        score += 3
        break
      case 'permission_gap':
        score += 2
        break
      case 'generation_drift':
        score += 2
        break
      case 'failure_count':
        score += 1
        break
      default:
        break
    }
  })

  return {
    needsAttention: issues.length > 0,
    score,
    issues,
    permissionGapCount,
  }
}

function joinSummaryItems(items: Array<string | null | undefined | false>): string {
  return items
    .filter((item): item is string => typeof item === 'string' && item.trim() !== '')
    .join(' · ')
}

function PluginMetaItem({
  label,
  value,
  hint,
  mono,
  tone = 'default',
}: {
  label: string
  value: string
  hint?: string
  mono?: boolean
  tone?: 'default' | 'danger'
}) {
  return (
    <div className="rounded-md border border-input/60 bg-background p-3 text-sm">
      <p className="text-xs text-muted-foreground">{label}</p>
      <p
        className={`mt-1 min-w-0 break-words font-medium ${
          mono ? 'font-mono text-xs' : ''
        } ${tone === 'danger' ? 'text-destructive' : ''}`}
      >
        {value || '-'}
      </p>
      {hint ? <p className="mt-1 break-words text-xs text-muted-foreground">{hint}</p> : null}
    </div>
  )
}

function attentionIssueLabel(
  issue: PluginAttentionIssueCode,
  attentionSummary: PluginAttentionSummary,
  t: Translations
): string {
  switch (issue) {
    case 'unhealthy':
      return t.admin.pluginAttentionIssueUnhealthy
    case 'last_error':
      return t.admin.pluginAttentionIssueLastError
    case 'deployment_failed':
      return t.admin.pluginAttentionIssueDeployment
    case 'permission_gap':
      return t.admin.pluginAttentionIssuePermissionGap.replace(
        '{count}',
        String(attentionSummary.permissionGapCount)
      )
    case 'generation_drift':
      return t.admin.pluginAttentionIssueGenerationDrift
    default:
      return t.admin.pluginAttentionIssueFailureCount
  }
}

function PluginCapabilityCard({
  label,
  meta,
  children,
  tone = 'default',
}: {
  label: string
  meta?: ReactNode
  children: ReactNode
  tone?: 'default' | 'warning' | 'danger'
}) {
  const toneClassName =
    tone === 'danger'
      ? 'border-destructive/30 bg-destructive/5'
      : tone === 'warning'
        ? 'border-amber-500/30 bg-amber-500/5 dark:border-amber-500/40 dark:bg-amber-950/20'
        : 'border-input/60 bg-background'

  return (
    <div className={`rounded-md border p-3 text-sm ${toneClassName}`}>
      <div className="space-y-1">
        <p className="font-medium">{label}</p>
        {meta ? <div className="text-xs text-muted-foreground">{meta}</div> : null}
      </div>
      <div className="mt-3 space-y-2">{children}</div>
    </div>
  )
}

function PluginActionGroup({
  title,
  description,
  emptyText,
  childrenClassName = 'flex flex-wrap gap-2',
  children,
}: {
  title: string
  description?: ReactNode
  emptyText?: ReactNode
  childrenClassName?: string
  children: ReactNode
}) {
  const actionItems = Children.toArray(children).filter(Boolean)
  return (
    <div className="rounded-md border border-input/60 bg-muted/10 p-3">
      <div className="mb-3 space-y-1">
        <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">{title}</p>
        {description ? <div className="text-xs text-muted-foreground">{description}</div> : null}
      </div>
      {actionItems.length > 0 ? (
        <div className={childrenClassName}>{actionItems}</div>
      ) : (
        <div className="rounded-md border border-dashed border-input/60 bg-background/80 px-3 py-2 text-xs text-muted-foreground">
          {emptyText}
        </div>
      )}
    </div>
  )
}

type PluginListPanelProps = {
  t: Translations
  locale: string
  pluginsQueryLoading: boolean
  pluginsQueryFetching: boolean
  plugins: AdminPlugin[]
  lifecycleLabel: Record<string, string>
  healthLabel: Record<string, string>
  formatDateTime: (value?: string, locale?: string) => string
  runtimeLabel: (runtime: string, t: Translations) => string
  runtimeAddressLabel: (runtime: string, t: Translations) => string
  resolvePluginLifecycleActionState: (plugin: AdminPlugin) => PluginLifecycleActionState
  isLifecycleBusy: (pluginId: number) => boolean
  isDeleteBusy: (pluginId: number) => boolean
  isTestBusy: (pluginId: number) => boolean
  onRefresh: () => void
  onOpenUpload: (plugin?: AdminPlugin) => void
  onOpenCreate: () => void
  onLifecycleAction: (pluginId: number, action: string) => void
  onOpenVersions: (plugin: AdminPlugin) => void
  onTest: (pluginId: number) => void
  onOpenWorkspace: (plugin: AdminPlugin) => void
  onOpenDiagnostics: (plugin: AdminPlugin) => void
  onOpenLogs: (plugin: AdminPlugin) => void
  onOpenEdit: (plugin: AdminPlugin) => void
  onOpenDelete: (plugin: AdminPlugin) => void
}

export function PluginListPanel({
  t,
  locale,
  pluginsQueryLoading,
  pluginsQueryFetching,
  plugins,
  lifecycleLabel,
  healthLabel,
  formatDateTime,
  runtimeLabel,
  runtimeAddressLabel,
  resolvePluginLifecycleActionState,
  isLifecycleBusy,
  isDeleteBusy,
  isTestBusy,
  onRefresh,
  onOpenUpload,
  onOpenCreate,
  onLifecycleAction,
  onOpenVersions,
  onTest,
  onOpenWorkspace,
  onOpenDiagnostics,
  onOpenLogs,
  onOpenEdit,
  onOpenDelete,
}: PluginListPanelProps) {
  const [searchText, setSearchText] = useState('')
  const [runtimeFilter, setRuntimeFilter] = useState('all')
  const [lifecycleFilter, setLifecycleFilter] = useState('all')
  const [healthFilter, setHealthFilter] = useState('all')
  const [sortMode, setSortMode] = useState<'attention' | 'name'>('attention')
  const [attentionOnly, setAttentionOnly] = useState(false)
  const [pluginDetailVisibility, setPluginDetailVisibility] = useState<Record<number, boolean>>({})

  const pluginRows = useMemo<PluginListRow[]>(
    () =>
      plugins.map((plugin) => {
        const capabilitySummary = resolvePluginCapabilitySummary(plugin)
        return {
          plugin,
          localizedDisplayName: resolvePluginLocalizedDisplayName(plugin, locale),
          localizedDescription: resolvePluginLocalizedDescription(plugin, locale),
          capabilitySummary,
          manifestSummary: resolvePluginManifestSchemaSummary(plugin),
          adminPageSummary: resolvePluginAdminPageSummary(plugin),
          latestDeployment: plugin.latest_deployment,
          attentionSummary: resolvePluginAttentionSummary(plugin, capabilitySummary),
        }
      }),
    [locale, plugins]
  )

  const runtimeOptions = useMemo(
    () =>
      Array.from(new Set(pluginRows.map(({ plugin }) => resolvePluginRuntimeFilterValue(plugin))))
        .filter(Boolean)
        .sort((left, right) => left.localeCompare(right)),
    [pluginRows]
  )
  const lifecycleOptions = useMemo(
    () =>
      Array.from(new Set(pluginRows.map(({ plugin }) => resolvePluginLifecycleFilterValue(plugin))))
        .filter(Boolean)
        .sort((left, right) => left.localeCompare(right)),
    [pluginRows]
  )
  const healthOptions = useMemo(
    () =>
      Array.from(new Set(pluginRows.map(({ plugin }) => resolvePluginHealthFilterValue(plugin))))
        .filter(Boolean)
        .sort((left, right) => left.localeCompare(right)),
    [pluginRows]
  )

  const normalizedSearchText = normalizePluginFilterValue(searchText)
  const hasActiveFilters =
    normalizedSearchText !== '' ||
    runtimeFilter !== 'all' ||
    lifecycleFilter !== 'all' ||
    healthFilter !== 'all' ||
    attentionOnly
  const activeFilterCount = [
    normalizedSearchText !== '',
    runtimeFilter !== 'all',
    lifecycleFilter !== 'all',
    healthFilter !== 'all',
    attentionOnly,
  ].filter(Boolean).length
  const activeFilterSummary = [
    searchText.trim(),
    runtimeFilter !== 'all' ? runtimeLabel(runtimeFilter, t) : '',
    lifecycleFilter !== 'all' ? lifecycleLabel[lifecycleFilter] || lifecycleFilter : '',
    healthFilter !== 'all' ? healthLabel[healthFilter] || healthFilter : '',
    attentionOnly ? t.admin.pluginAttentionOnly : '',
  ]
    .filter(Boolean)
    .join(' / ')
  const sortModeLabel =
    sortMode === 'attention'
      ? t.admin.pluginAttentionViewModeAttention
      : t.admin.pluginAttentionViewModeName
  const adminBootstrapQuery = usePluginBootstrapQuery({
    scope: 'admin',
    path: '/admin',
    staleTime: 5 * 60 * 1000,
  })
  const marketPluginPath = useMemo(() => {
    return findAdminMarketPluginBasePath(adminBootstrapQuery.data)
  }, [adminBootstrapQuery.data])

  const attentionStats = useMemo(
    () =>
      pluginRows.reduce(
        (acc, row) => {
          if (row.attentionSummary.needsAttention) acc.attentionCount += 1
          if (row.attentionSummary.issues.includes('unhealthy')) acc.unhealthyCount += 1
          if (row.attentionSummary.issues.includes('permission_gap')) acc.permissionGapCount += 1
          if (row.attentionSummary.issues.includes('generation_drift'))
            acc.generationDriftCount += 1
          if (row.attentionSummary.issues.includes('deployment_failed'))
            acc.deploymentFailedCount += 1
          return acc
        },
        {
          attentionCount: 0,
          unhealthyCount: 0,
          permissionGapCount: 0,
          generationDriftCount: 0,
          deploymentFailedCount: 0,
        }
      ),
    [pluginRows]
  )
  const adminPageCount = useMemo(
    () => pluginRows.filter((row) => !!row.adminPageSummary).length,
    [pluginRows]
  )

  const filteredRows = useMemo(
    () =>
      pluginRows.filter(
        ({
          plugin,
          localizedDisplayName,
          localizedDescription,
          attentionSummary,
          adminPageSummary,
        }) => {
          if (attentionOnly && !attentionSummary.needsAttention) {
            return false
          }
          if (
            runtimeFilter !== 'all' &&
            resolvePluginRuntimeFilterValue(plugin) !== runtimeFilter
          ) {
            return false
          }
          if (
            lifecycleFilter !== 'all' &&
            resolvePluginLifecycleFilterValue(plugin) !== lifecycleFilter
          ) {
            return false
          }
          if (healthFilter !== 'all' && resolvePluginHealthFilterValue(plugin) !== healthFilter) {
            return false
          }
          if (!normalizedSearchText) {
            return true
          }
          const searchableText = [
            plugin.id,
            plugin.name,
            localizedDisplayName,
            localizedDescription,
            plugin.display_name,
            plugin.description,
            plugin.type,
            plugin.runtime,
            plugin.version,
            plugin.status,
            plugin.lifecycle_status,
            plugin.address_display,
            plugin.address,
            adminPageSummary?.path,
            plugin.latest_deployment?.operation,
            plugin.latest_deployment?.status,
          ]
            .map((item) => normalizePluginFilterValue(item))
            .join(' ')
          return searchableText.includes(normalizedSearchText)
        }
      ),
    [attentionOnly, healthFilter, lifecycleFilter, normalizedSearchText, pluginRows, runtimeFilter]
  )

  const displayRows = useMemo(() => {
    const rows = [...filteredRows]
    rows.sort((left, right) => {
      if (sortMode === 'attention') {
        if (right.attentionSummary.score !== left.attentionSummary.score) {
          return right.attentionSummary.score - left.attentionSummary.score
        }
        if (right.attentionSummary.issues.length !== left.attentionSummary.issues.length) {
          return right.attentionSummary.issues.length - left.attentionSummary.issues.length
        }
      }
      const leftName = String(left.localizedDisplayName || left.plugin.name || '')
        .trim()
        .toLowerCase()
      const rightName = String(right.localizedDisplayName || right.plugin.name || '')
        .trim()
        .toLowerCase()
      return leftName.localeCompare(rightName)
    })
    return rows
  }, [filteredRows, sortMode])

  const resetFilters = () => {
    setSearchText('')
    setRuntimeFilter('all')
    setLifecycleFilter('all')
    setHealthFilter('all')
    setSortMode('attention')
    setAttentionOnly(false)
  }

  const setVisiblePluginDetails = (expanded: boolean) => {
    setPluginDetailVisibility((current) => {
      const next = { ...current }
      displayRows.forEach(({ plugin }) => {
        next[plugin.id] = expanded
      })
      return next
    })
  }

  return (
    <>
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0 max-w-3xl">
          <h1 className="text-2xl font-bold sm:text-3xl">{t.pageTitle.adminPlugins}</h1>
          <p className="mt-1 text-sm leading-6 text-muted-foreground">{t.admin.pluginSubtitle}</p>
        </div>
        <div className="flex w-full flex-wrap items-center gap-2 sm:w-auto sm:justify-end">
          <div className="flex flex-wrap gap-2">
            <Button variant="outline" onClick={onRefresh} disabled={pluginsQueryFetching}>
              {pluginsQueryFetching ? (
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              ) : (
                <RefreshCw className="mr-2 h-4 w-4" />
              )}
              {t.admin.refresh}
            </Button>
            <Button variant="outline" onClick={() => onOpenUpload()}>
              <FileUp className="mr-2 h-4 w-4" />
              {t.admin.pluginUpload}
            </Button>
            <Button onClick={onOpenCreate}>
              <Plus className="mr-2 h-4 w-4" />
              {t.admin.pluginAdd}
            </Button>
          </div>
          <div className="flex flex-wrap gap-2">
            {marketPluginPath ? (
              <Button variant="outline" asChild>
                <Link href={marketPluginPath}>
                  <Waypoints className="mr-2 h-4 w-4" />
                  {t.admin.pluginMarket}
                </Link>
              </Button>
            ) : null}
            <Button variant="outline" asChild>
              <Link href="/admin/plugins/observability">
                <Activity className="mr-2 h-4 w-4" />
                {t.admin.pluginObservability}
              </Link>
            </Button>
          </div>
        </div>
      </div>

      {!pluginsQueryLoading && plugins.length > 0 ? (
        <Card>
          <CardContent className="space-y-4 pt-6">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div className="min-w-0 flex-1 space-y-2">
                <p className="text-xs text-muted-foreground">
                  {[
                    `${displayRows.length}/${plugins.length}`,
                    hasActiveFilters ? `${t.common.filter} ${activeFilterCount}` : t.common.all,
                    `${t.admin.pluginAttentionSummary}: ${attentionStats.attentionCount}`,
                    `${t.admin.pluginAttentionUnhealthy}: ${attentionStats.unhealthyCount}`,
                    `${t.admin.pluginAttentionPermissionGap}: ${attentionStats.permissionGapCount}`,
                    `${t.admin.pluginAttentionGenerationDrift}: ${attentionStats.generationDriftCount}`,
                    `${t.admin.pluginAttentionDeployment}: ${attentionStats.deploymentFailedCount}`,
                    `${t.admin.pluginSummaryAdminPage}: ${adminPageCount}`,
                    sortModeLabel,
                  ].join(' · ')}
                </p>
                <p className="text-sm font-medium">{t.common.filter}</p>
                <p className="text-xs text-muted-foreground">
                  {hasActiveFilters ? activeFilterSummary : t.admin.pluginSearchPlaceholder}
                </p>
              </div>
              <div className="flex flex-wrap gap-2">
                <Button
                  variant={attentionOnly ? 'secondary' : 'outline'}
                  size="sm"
                  onClick={() => setAttentionOnly((prev) => !prev)}
                >
                  {t.admin.pluginAttentionOnly}
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setVisiblePluginDetails(true)}
                  disabled={displayRows.length === 0}
                >
                  <ChevronDown className="mr-2 h-4 w-4" />
                  {t.admin.pluginExpandVisible}
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setVisiblePluginDetails(false)}
                  disabled={displayRows.length === 0}
                >
                  <ChevronRight className="mr-2 h-4 w-4" />
                  {t.admin.pluginCollapseVisible}
                </Button>
                {hasActiveFilters ? (
                  <Button variant="ghost" size="sm" onClick={resetFilters}>
                    {t.common.reset}
                  </Button>
                ) : null}
              </div>
            </div>
            <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-[minmax(0,2fr)_repeat(4,minmax(0,1fr))]">
              <div className="space-y-2">
                <p className="text-xs text-muted-foreground">{t.common.search}</p>
                <Input
                  value={searchText}
                  onChange={(event) => setSearchText(event.target.value)}
                  placeholder={t.admin.pluginSearchPlaceholder}
                />
              </div>
              <div className="space-y-2">
                <p className="text-xs text-muted-foreground">{t.admin.pluginRuntime}</p>
                <Select value={runtimeFilter} onValueChange={setRuntimeFilter}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="all">{t.common.all}</SelectItem>
                    {runtimeOptions.map((runtime) => (
                      <SelectItem key={runtime} value={runtime}>
                        {runtimeLabel(runtime, t)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <p className="text-xs text-muted-foreground">{t.admin.pluginFilterLifecycle}</p>
                <Select value={lifecycleFilter} onValueChange={setLifecycleFilter}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="all">{t.common.all}</SelectItem>
                    {lifecycleOptions.map((status) => (
                      <SelectItem key={status} value={status}>
                        {lifecycleLabel[status] || status}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <p className="text-xs text-muted-foreground">{t.admin.pluginFilterHealth}</p>
                <Select value={healthFilter} onValueChange={setHealthFilter}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="all">{t.common.all}</SelectItem>
                    {healthOptions.map((status) => (
                      <SelectItem key={status} value={status}>
                        {healthLabel[status] || status}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <p className="text-xs text-muted-foreground">{t.admin.pluginAttentionViewMode}</p>
                <Select
                  value={sortMode}
                  onValueChange={(value) => setSortMode(value as 'attention' | 'name')}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="attention">
                      {t.admin.pluginAttentionViewModeAttention}
                    </SelectItem>
                    <SelectItem value="name">{t.admin.pluginAttentionViewModeName}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
            <div className="text-sm text-muted-foreground">
              {hasActiveFilters ? activeFilterSummary : t.common.all}
            </div>
          </CardContent>
        </Card>
      ) : null}

      {pluginsQueryLoading ? (
        <Card>
          <CardContent className="py-10 text-center text-muted-foreground">
            <Loader2 className="mr-2 inline-block h-4 w-4 animate-spin" />
            {t.common.loading}
          </CardContent>
        </Card>
      ) : plugins.length === 0 ? (
        <Card>
          <CardContent className="space-y-4 py-10 text-center text-muted-foreground">
            <div className="space-y-2">
              <p>{t.admin.pluginNoData}</p>
              <p className="text-xs">{t.admin.pluginSubtitle}</p>
            </div>
            <div className="flex flex-wrap justify-center gap-2">
              <Button variant="outline" onClick={() => onOpenUpload()}>
                <FileUp className="mr-2 h-4 w-4" />
                {t.admin.pluginUpload}
              </Button>
              <Button onClick={onOpenCreate}>
                <Plus className="mr-2 h-4 w-4" />
                {t.admin.pluginAdd}
              </Button>
              {marketPluginPath ? (
                <Button variant="outline" asChild>
                  <Link href={marketPluginPath}>
                    <Waypoints className="mr-2 h-4 w-4" />
                    {t.admin.pluginMarket}
                  </Link>
                </Button>
              ) : null}
              <Button variant="outline" asChild>
                <Link href="/admin/plugins/observability">
                  <Activity className="mr-2 h-4 w-4" />
                  {t.admin.pluginObservability}
                </Link>
              </Button>
            </div>
          </CardContent>
        </Card>
      ) : displayRows.length === 0 ? (
        <Card>
          <CardContent className="space-y-3 py-10 text-center text-muted-foreground">
            <p>{t.admin.pluginNoMatches}</p>
            {hasActiveFilters ? (
              <p className="text-xs text-muted-foreground">{activeFilterSummary}</p>
            ) : null}
            <div>
              <Button variant="outline" onClick={resetFilters}>
                {t.common.reset}
              </Button>
            </div>
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-4">
          {displayRows.map((row) => {
            const {
              plugin,
              localizedDisplayName,
              localizedDescription,
              capabilitySummary,
              manifestSummary,
              adminPageSummary,
              latestDeployment,
              attentionSummary,
            } = row
            const lifecycle = (plugin.lifecycle_status || 'draft').toLowerCase()
            const health = (plugin.status || 'unknown').toLowerCase()
            const lifecycleActions = resolvePluginLifecycleActionState(plugin)
            const busyLifecycle = isLifecycleBusy(plugin.id)
            const busyDelete = isDeleteBusy(plugin.id)
            const busyTest = isTestBusy(plugin.id)
            const busyManage = busyTest || busyDelete
            const frontendAreasLabel =
              capabilitySummary.frontendAreas.length === 0 ||
              capabilitySummary.frontendAreas.includes('*')
                ? t.admin.pluginSummaryAll
                : capabilitySummary.frontendAreas.join(', ')
            const frontendSlotsLabel =
              capabilitySummary.frontendSlots.length === 0 ||
              capabilitySummary.frontendSlots.includes('*')
                ? t.admin.pluginSummaryAll
                : String(capabilitySummary.frontendSlots.length)
            const pluginAddress = plugin.address_display || plugin.address || '-'
            const lifecycleText = lifecycleLabel[lifecycle] || lifecycle
            const healthText = healthLabel[health] || health
            const isJSWorkerPlugin =
              String(plugin.runtime || '')
                .trim()
                .toLowerCase() === 'js_worker'
            const schemaFieldCount =
              manifestSummary.configSchemaFieldCount + manifestSummary.runtimeParamsSchemaFieldCount
            const lastErrorText = String(plugin.last_error || '').trim()
            const lastErrorHeadline =
              lastErrorText
                .split(/\r?\n/)
                .map((line) => line.trim())
                .find(Boolean) || lastErrorText
            const latestDeploymentText = latestDeployment
              ? `${deploymentOperationLabel(latestDeployment.operation, t)} / ${deploymentStatusLabel(
                  latestDeployment.status,
                  t
                )}`
              : t.common.noData
            const defaultDetailsExpanded =
              attentionSummary.needsAttention || lastErrorText !== '' || busyLifecycle
            const detailsExpanded =
              typeof pluginDetailVisibility[plugin.id] === 'boolean'
                ? pluginDetailVisibility[plugin.id]
                : defaultDetailsExpanded
            const quickAccessButtons = (
              <>
                <Button
                  size="sm"
                  variant="outline"
                  className="justify-start"
                  onClick={() => onOpenDiagnostics(plugin)}
                >
                  <ShieldCheck className="mr-2 h-4 w-4" />
                  {t.admin.pluginDiagnostics}
                </Button>
                {adminPageSummary ? (
                  lifecycleActions.execute ? (
                    <Button size="sm" variant="outline" className="justify-start" asChild>
                      <Link href={adminPageSummary.path}>
                        <SlidersHorizontal className="mr-2 h-4 w-4" />
                        {t.admin.pluginOpenAdminPage}
                      </Link>
                    </Button>
                  ) : (
                    <Button size="sm" variant="outline" className="justify-start" disabled>
                      <SlidersHorizontal className="mr-2 h-4 w-4" />
                      {t.admin.pluginOpenAdminPage}
                    </Button>
                  )
                ) : null}
                {lifecycleActions.versions ? (
                  <Button
                    size="sm"
                    variant="outline"
                    className="justify-start"
                    onClick={() => onOpenVersions(plugin)}
                    disabled={busyLifecycle}
                  >
                    <Waypoints className="mr-2 h-4 w-4" />
                    {t.admin.pluginVersions}
                  </Button>
                ) : null}
                {lifecycleActions.logs && (detailsExpanded || attentionSummary.needsAttention) ? (
                  <Button
                    size="sm"
                    variant="outline"
                    className="justify-start"
                    onClick={() => onOpenLogs(plugin)}
                    disabled={busyLifecycle}
                  >
                    <Activity className="mr-2 h-4 w-4" />
                    {t.admin.pluginLogs}
                  </Button>
                ) : null}
                {isJSWorkerPlugin && lifecycleActions.execute && detailsExpanded ? (
                  <Button
                    size="sm"
                    variant="outline"
                    className="justify-start"
                    onClick={() => onOpenWorkspace(plugin)}
                    disabled={busyLifecycle}
                  >
                    <TerminalSquare className="mr-2 h-4 w-4" />
                    {t.admin.pluginWorkspace}
                  </Button>
                ) : null}
              </>
            )
            const lifecycleActionButtons = (
              <>
                {lifecycleActions.install ? (
                  <Button
                    size="sm"
                    variant="outline"
                    className="justify-start"
                    onClick={() => onLifecycleAction(plugin.id, 'install')}
                    disabled={busyLifecycle}
                  >
                    <Waypoints className="mr-2 h-4 w-4" />
                    {t.admin.pluginLifecycleInstall}
                  </Button>
                ) : null}
                {lifecycleActions.start ? (
                  <Button
                    size="sm"
                    variant="outline"
                    className="justify-start"
                    onClick={() => onLifecycleAction(plugin.id, 'start')}
                    disabled={busyLifecycle}
                  >
                    <Play className="mr-2 h-4 w-4" />
                    {t.admin.pluginLifecycleStart}
                  </Button>
                ) : null}
                {lifecycleActions.pause ? (
                  <Button
                    size="sm"
                    variant="outline"
                    className="justify-start"
                    onClick={() => onLifecycleAction(plugin.id, 'pause')}
                    disabled={busyLifecycle}
                  >
                    <Pause className="mr-2 h-4 w-4" />
                    {t.admin.pluginLifecyclePause}
                  </Button>
                ) : null}
                {lifecycleActions.restart ? (
                  <Button
                    size="sm"
                    variant="outline"
                    className="justify-start"
                    onClick={() => onLifecycleAction(plugin.id, 'restart')}
                    disabled={busyLifecycle}
                  >
                    <RotateCcw className="mr-2 h-4 w-4" />
                    {t.admin.pluginLifecycleRestart}
                  </Button>
                ) : null}
                {lifecycleActions.hotReload ? (
                  <Button
                    size="sm"
                    variant="outline"
                    className="justify-start"
                    onClick={() => onLifecycleAction(plugin.id, 'hot_reload')}
                    disabled={busyLifecycle}
                  >
                    <RefreshCw className="mr-2 h-4 w-4" />
                    {t.admin.pluginHotReload}
                  </Button>
                ) : null}
                {lifecycleActions.resume ? (
                  <Button
                    size="sm"
                    variant="outline"
                    className="justify-start"
                    onClick={() => onLifecycleAction(plugin.id, 'resume')}
                    disabled={busyLifecycle}
                  >
                    <Play className="mr-2 h-4 w-4" />
                    {t.admin.pluginLifecycleResume}
                  </Button>
                ) : null}
                {lifecycleActions.retire ? (
                  <Button
                    size="sm"
                    variant="destructive"
                    className="justify-start"
                    onClick={() => onLifecycleAction(plugin.id, 'retire')}
                    disabled={busyLifecycle}
                  >
                    <Trash2 className="mr-2 h-4 w-4" />
                    {t.admin.pluginLifecycleRetire}
                  </Button>
                ) : null}
              </>
            )
            const manageActionButtons = (
              <>
                {lifecycleActions.upload ? (
                  <Button
                    size="sm"
                    variant="outline"
                    className="justify-start"
                    onClick={() => onOpenUpload(plugin)}
                    disabled={busyLifecycle}
                  >
                    <FileUp className="mr-2 h-4 w-4" />
                    {t.admin.pluginUpload}
                  </Button>
                ) : null}
                {lifecycleActions.test ? (
                  <Button
                    size="sm"
                    variant="outline"
                    className="justify-start"
                    onClick={() => onTest(plugin.id)}
                    disabled={busyTest}
                  >
                    {busyTest ? (
                      <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    ) : (
                      <TestTube2 className="mr-2 h-4 w-4" />
                    )}
                    {t.admin.pluginTest}
                  </Button>
                ) : null}
                {lifecycleActions.edit || lifecycleActions.remove ? (
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button size="sm" variant="outline" className="justify-start">
                        <MoreHorizontal className="mr-2 h-4 w-4" />
                        {t.admin.pluginMoreActions}
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end" className="w-56">
                      {lifecycleActions.edit ? (
                        <DropdownMenuItem
                          onSelect={() => onOpenEdit(plugin)}
                          disabled={busyLifecycle}
                        >
                          <Pencil className="mr-2 h-4 w-4" />
                          {t.admin.pluginEdit}
                        </DropdownMenuItem>
                      ) : null}
                      {lifecycleActions.edit && lifecycleActions.remove ? (
                        <DropdownMenuSeparator />
                      ) : null}
                      {lifecycleActions.remove ? (
                        <DropdownMenuItem
                          className="text-destructive focus:text-destructive"
                          onSelect={() => onOpenDelete(plugin)}
                          disabled={busyDelete}
                        >
                          <Trash2 className="mr-2 h-4 w-4" />
                          {t.common.delete}
                        </DropdownMenuItem>
                      ) : null}
                    </DropdownMenuContent>
                  </DropdownMenu>
                ) : null}
              </>
            )

            return (
              <Card key={plugin.id}>
                <CardHeader className="pb-3">
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div className="min-w-0 flex-1">
                      <CardTitle className="break-words">
                        {localizedDisplayName || plugin.name}
                      </CardTitle>
                      <CardDescription className="break-all">{plugin.name}</CardDescription>
                      <p className="mt-2 text-xs text-muted-foreground">
                        {[
                          attentionSummary.needsAttention
                            ? `${t.admin.pluginAttentionSummary}: ${attentionSummary.issues.length}`
                            : null,
                          adminPageSummary ? t.admin.pluginOpenAdminPage : null,
                          busyLifecycle || busyManage ? t.common.processing : null,
                          plugin.type,
                          `${t.admin.pluginRuntime}: ${runtimeLabel(plugin.runtime || 'grpc', t)}`,
                          `${t.admin.pluginVersionLabel}: ${plugin.version || '0.0.0'}`,
                        ]
                          .filter(Boolean)
                          .join(' · ')}
                      </p>
                      {localizedDescription ? (
                        <p className="mt-2 line-clamp-2 text-sm text-muted-foreground">
                          {localizedDescription}
                        </p>
                      ) : null}
                    </div>
                  </div>
                </CardHeader>
                <CardContent className="space-y-4 [&_.grid>*]:min-w-0">
                  {attentionSummary.needsAttention ? (
                    <div className="rounded-md border border-amber-500/30 bg-amber-500/10 p-3 dark:border-amber-500/40 dark:bg-amber-950/20">
                      <p className="text-sm font-medium">{t.admin.pluginAttentionSummary}</p>
                      <p className="mt-1 text-xs text-muted-foreground">
                        {detailsExpanded
                          ? t.admin.pluginAttentionNeedsAttention
                          : joinSummaryItems(
                              attentionSummary.issues.map((issue) =>
                                attentionIssueLabel(issue, attentionSummary, t)
                              )
                            )}
                      </p>
                      {detailsExpanded ? (
                        <p className="mt-2 text-xs text-muted-foreground">
                          {joinSummaryItems(
                            attentionSummary.issues.map((issue) =>
                              attentionIssueLabel(issue, attentionSummary, t)
                            )
                          )}
                        </p>
                      ) : null}
                    </div>
                  ) : null}

                  <PluginActionGroup
                    title={t.admin.pluginQuickAccess}
                    description={detailsExpanded ? t.admin.pluginQuickAccessHint : undefined}
                    emptyText={t.admin.pluginQuickAccessEmpty}
                    childrenClassName={
                      detailsExpanded
                        ? 'grid gap-2 md:grid-cols-2 xl:grid-cols-5'
                        : 'grid gap-2 md:grid-cols-2 xl:grid-cols-4'
                    }
                  >
                    {quickAccessButtons}
                  </PluginActionGroup>

                  <div className="rounded-md border border-input/60 bg-muted/10 p-3">
                    <div className="flex flex-wrap items-start justify-between gap-3">
                      <div className="min-w-0 flex-1 space-y-1">
                        <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                          {t.admin.pluginDetailsSection}
                        </p>
                        <p className="text-xs text-muted-foreground">
                          {detailsExpanded
                            ? t.admin.pluginDetailsHint
                            : t.admin.pluginDetailsCollapsedHint}
                        </p>
                      </div>
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        className="shrink-0"
                        onClick={() =>
                          setPluginDetailVisibility((current) => ({
                            ...current,
                            [plugin.id]: !detailsExpanded,
                          }))
                        }
                      >
                        {detailsExpanded ? (
                          <ChevronDown className="mr-2 h-4 w-4" />
                        ) : (
                          <ChevronRight className="mr-2 h-4 w-4" />
                        )}
                        {detailsExpanded ? t.common.collapse : t.common.expand}
                      </Button>
                    </div>
                    <p className="mt-3 text-xs text-muted-foreground">
                      {joinSummaryItems([
                        `${t.admin.pluginLifecycle}: ${lifecycleText}`,
                        `${t.admin.pluginDiagnosticsHealth}: ${healthText}`,
                        plugin.enabled ? t.admin.enabled : t.admin.disabled,
                        `${t.admin.pluginSummaryPermissions}: ${capabilitySummary.requestedPermissionCount}`,
                        `${t.admin.pluginSummaryFrontend}: ${
                          capabilitySummary.frontendEnabled ? t.admin.enabled : t.admin.disabled
                        }`,
                        `${t.admin.pluginSummarySchema}: ${schemaFieldCount}`,
                      ])}
                    </p>
                    <div className="mt-3 space-y-1">
                      {lastErrorText ? (
                        <p className="line-clamp-2 break-words text-xs text-destructive">
                          {lastErrorHeadline}
                        </p>
                      ) : adminPageSummary?.path ? (
                        <p className="break-all font-mono text-[11px] text-muted-foreground">
                          {adminPageSummary.path}
                        </p>
                      ) : (
                        <p className="text-xs text-muted-foreground">
                          {t.admin.pluginLatestDeploymentSummary.replace(
                            '{value}',
                            latestDeployment ? latestDeploymentText : t.common.noData
                          )}
                        </p>
                      )}
                    </div>
                  </div>

                  {detailsExpanded ? (
                    <>
                      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
                        <PluginMetaItem
                          label={runtimeAddressLabel(plugin.runtime || 'grpc', t)}
                          value={pluginAddress}
                          mono
                        />
                        <PluginMetaItem
                          label={t.admin.pluginSummaryAdminPage}
                          value={adminPageSummary?.path || t.common.noData}
                          mono
                        />
                        <PluginMetaItem
                          label={t.admin.pluginLastHealthy}
                          value={formatDateTime(plugin.last_healthy, locale)}
                          hint={`${t.admin.pluginLatestDeploymentAt}: ${formatDateTime(
                            latestDeployment?.created_at,
                            locale
                          )}`}
                        />
                        <PluginMetaItem
                          label={t.admin.pluginFailCount}
                          value={String(plugin.fail_count || 0)}
                          tone={Number(plugin.fail_count || 0) > 0 ? 'danger' : 'default'}
                        />
                        <PluginMetaItem
                          label={t.admin.createdAt}
                          value={formatDateTime(plugin.created_at, locale)}
                        />
                        <PluginMetaItem
                          label={t.admin.pluginSummaryDeployment}
                          value={t.admin.pluginGenerationSummary.replace(
                            '{value}',
                            generationLabel(plugin)
                          )}
                          hint={t.admin.pluginLatestDeploymentSummary.replace(
                            '{value}',
                            latestDeployment ? latestDeploymentText : '-'
                          )}
                        />
                      </div>

                      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
                        <PluginCapabilityCard
                          label={t.admin.pluginSummaryHooks}
                          meta={
                            capabilitySummary.allowAllHooks
                              ? t.admin.pluginSummaryAll
                              : capabilitySummary.allowedHookCount > 0
                                ? t.admin.enabled
                                : t.admin.disabled
                          }
                        >
                          <p className="text-muted-foreground">
                            {capabilitySummary.allowAllHooks
                              ? t.admin.pluginSummaryHookAllowAll
                              : t.admin.pluginSummaryHookAllowSelected.replace(
                                  '{count}',
                                  String(capabilitySummary.allowedHookCount)
                                )}
                          </p>
                          <p className="text-xs text-muted-foreground">
                            {t.admin.pluginSummaryHookDisabled.replace(
                              '{count}',
                              String(capabilitySummary.disabledHookCount)
                            )}
                          </p>
                        </PluginCapabilityCard>
                        <PluginCapabilityCard
                          label={t.admin.pluginSummaryPermissions}
                          meta={
                            attentionSummary.permissionGapCount > 0
                              ? t.admin.pluginSummaryPermissionMissing.replace(
                                  '{count}',
                                  String(attentionSummary.permissionGapCount)
                                )
                              : capabilitySummary.requestedPermissionCount > 0
                                ? t.admin.enabled
                                : t.common.noData
                          }
                          tone={attentionSummary.permissionGapCount > 0 ? 'danger' : 'default'}
                        >
                          <p className="text-muted-foreground">
                            {t.admin.pluginSummaryPermissionRequested.replace(
                              '{count}',
                              String(capabilitySummary.requestedPermissionCount)
                            )}
                          </p>
                          <p className="text-xs text-muted-foreground">
                            {t.admin.pluginSummaryPermissionGranted.replace(
                              '{count}',
                              String(capabilitySummary.grantedPermissionCount)
                            )}
                          </p>
                          <p
                            className={`text-xs ${
                              attentionSummary.permissionGapCount > 0
                                ? 'text-destructive'
                                : 'text-muted-foreground'
                            }`}
                          >
                            {t.admin.pluginSummaryPermissionMissing.replace(
                              '{count}',
                              String(attentionSummary.permissionGapCount)
                            )}
                          </p>
                        </PluginCapabilityCard>
                        <PluginCapabilityCard
                          label={t.admin.pluginSummaryFrontend}
                          meta={
                            capabilitySummary.frontendEnabled ? t.admin.enabled : t.admin.disabled
                          }
                        >
                          <p className="text-muted-foreground">
                            {t.admin.pluginSummaryFrontendScope.replace(
                              '{scope}',
                              scopeLabel(capabilitySummary.frontendMinScope, t)
                            )}
                          </p>
                          <p className="text-xs text-muted-foreground">
                            {t.admin.pluginSummaryFrontendAreas.replace(
                              '{value}',
                              frontendAreasLabel
                            )}
                          </p>
                          <p className="text-xs text-muted-foreground">
                            {t.admin.pluginSummaryFrontendSlots.replace(
                              '{value}',
                              frontendSlotsLabel
                            )}
                          </p>
                        </PluginCapabilityCard>
                        <PluginCapabilityCard
                          label={t.admin.pluginSummarySchema}
                          meta={schemaFieldCount > 0 ? String(schemaFieldCount) : t.common.noData}
                        >
                          <p className="text-muted-foreground">
                            {t.admin.pluginSummarySchemaConfig.replace(
                              '{count}',
                              String(manifestSummary.configSchemaFieldCount)
                            )}
                          </p>
                          <p className="text-xs text-muted-foreground">
                            {t.admin.pluginSummarySchemaRuntime.replace(
                              '{count}',
                              String(manifestSummary.runtimeParamsSchemaFieldCount)
                            )}
                          </p>
                        </PluginCapabilityCard>
                      </div>

                      {lastErrorText ? (
                        <div className="rounded-md border border-destructive/30 bg-destructive/5 p-3">
                          <div className="flex flex-wrap items-start justify-between gap-3">
                            <div className="min-w-0 flex-1">
                              <p className="text-sm font-medium text-destructive">
                                {t.admin.pluginDiagnosticsLastError}
                              </p>
                              <p className="mt-1 text-xs text-muted-foreground">
                                {t.admin.pluginAttentionIssueLastError}
                              </p>
                              <p className="mt-2 break-words text-xs text-muted-foreground">
                                {lastErrorHeadline}
                              </p>
                            </div>
                            <div className="flex flex-wrap gap-2">
                              <Button
                                size="sm"
                                variant="outline"
                                onClick={() => onOpenDiagnostics(plugin)}
                              >
                                {t.admin.pluginDiagnostics}
                              </Button>
                              <Button
                                size="sm"
                                variant="outline"
                                onClick={() => onOpenLogs(plugin)}
                                disabled={busyLifecycle || !lifecycleActions.logs}
                              >
                                {t.admin.pluginLogs}
                              </Button>
                            </div>
                          </div>
                          <div className="mt-3 max-h-48 overflow-auto rounded-md border border-destructive/20 bg-background/80 p-3">
                            <pre className="whitespace-pre-wrap break-words text-xs text-destructive">
                              {lastErrorText}
                            </pre>
                          </div>
                        </div>
                      ) : null}
                    </>
                  ) : null}

                  {detailsExpanded ? (
                    <div className="grid gap-3 xl:grid-cols-[minmax(0,2fr)_minmax(0,3fr)]">
                      <PluginActionGroup
                        title={t.admin.pluginLifecycleAction}
                        description={joinSummaryItems([
                          t.admin.pluginLifecycleActionHint,
                          busyLifecycle ? t.common.processing : null,
                        ])}
                        emptyText={t.admin.pluginLifecycleNoAvailableActions}
                        childrenClassName="grid gap-2 md:grid-cols-2 xl:grid-cols-3"
                      >
                        {lifecycleActionButtons}
                      </PluginActionGroup>

                      <PluginActionGroup
                        title={t.admin.actions}
                        description={joinSummaryItems([
                          t.admin.pluginManageActionHint,
                          busyManage ? t.common.processing : null,
                        ])}
                        emptyText={t.admin.pluginManageNoAvailableActions}
                        childrenClassName="grid gap-2 md:grid-cols-2 xl:grid-cols-3"
                      >
                        {manageActionButtons}
                      </PluginActionGroup>
                    </div>
                  ) : (
                    <PluginActionGroup
                      title={t.admin.actions}
                      description={joinSummaryItems([
                        t.admin.pluginCompactActionHint,
                        busyLifecycle || busyManage ? t.common.processing : null,
                      ])}
                      emptyText={t.admin.pluginManageNoAvailableActions}
                    >
                      {lifecycleActionButtons}
                      {manageActionButtons}
                    </PluginActionGroup>
                  )}
                </CardContent>
              </Card>
            )
          })}
        </div>
      )}
    </>
  )
}
