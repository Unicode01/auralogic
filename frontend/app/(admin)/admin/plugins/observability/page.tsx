'use client'

import Link from 'next/link'
import { usePathname, useRouter, useSearchParams } from 'next/navigation'
import { Suspense, useEffect, useMemo, useState, type KeyboardEvent, type ReactNode } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  Activity,
  ArrowLeft,
  Clock3,
  Copy,
  Gauge,
  ShieldAlert,
  TimerReset,
  Zap,
} from 'lucide-react'
import {
  Bar,
  BarChart,
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from '@/components/ui/lazy-recharts'

import { useTheme } from '@/contexts/theme-context'
import { useLocale } from '@/hooks/use-locale'
import { usePageTitle } from '@/hooks/use-page-title'
import { usePermission } from '@/hooks/use-permission'
import { useToast } from '@/hooks/use-toast'
import { getTranslations } from '@/lib/i18n'
import {
  getAdminPlugins,
  getAdminPluginObservability,
  type AdminPlugin,
  type PluginObservabilityExecutionCounters,
  type PluginObservabilityFrontendResolverSnapshot,
  type PluginObservabilitySnapshot,
} from '@/lib/api'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { PluginSlot } from '@/components/plugins/plugin-slot'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'

type CounterRow = {
  key: string
  counters: Required<PluginObservabilityExecutionCounters>
}

type BreakerRow = {
  plugin_id: number
  plugin_name: string
  runtime: string
  enabled: boolean
  lifecycle_status: string
  health_status: string
  breaker_state: string
  failure_count: number
  failure_threshold: number
  cooldown_active: boolean
  cooldown_until?: string
  cooldown_reason: string
  probe_in_flight: boolean
  probe_started_at?: string
  window_total_executions: number
  window_failed_executions: number
}

type ExecutionTrendRow = {
  hour_start: string
  total: number
  failed: number
}

type FailureGroupRow = {
  plugin_id: number
  plugin_name: string
  action: string
  hook: string
  failure_count: number
  last_failure_at?: string
}

type FailureSampleRow = {
  id: number
  plugin_id: number
  plugin_name: string
  action: string
  hook: string
  error: string
  duration: number
  created_at?: string
}

type HookGroupRow = {
  plugin_id: number
  plugin_name: string
  hook: string
  failure_count: number
  last_failure_at?: string
  last_error: string
}

type ErrorSignatureRow = {
  plugin_id: number
  plugin_name: string
  signature: string
  failure_count: number
  last_failure_at?: string
  sample_error: string
}

type FrontendResolverRow = {
  key: 'html_mode' | 'execute_api' | 'prepared_hook'
  label: string
  cache_hits: number
  cache_misses: number
  cache_hit_rate: number
  singleflight_waits: number
  catalog_hits: number
  db_fallbacks: number
}

type PluginOption = {
  id: number
  label: string
}

type RecentFailureFilter =
  | {
      kind: 'failure_group'
      plugin_id: number
      plugin_name: string
      action: string
      hook: string
    }
  | {
      kind: 'hook_group'
      plugin_id: number
      plugin_name: string
      hook: string
    }
  | {
      kind: 'error_signature'
      plugin_id: number
      plugin_name: string
      signature: string
    }

type DetailDialogState = {
  title: string
  content: string
} | null

const pluginObservabilityErrorSignatureMaxLength = 96
const observabilityPluginParamKey = 'plugin'
const observabilityWindowParamKey = 'window'

function normalizeCounters(
  input?: PluginObservabilityExecutionCounters
): Required<PluginObservabilityExecutionCounters> {
  return {
    total: Number(input?.total || 0),
    success: Number(input?.success || 0),
    failed: Number(input?.failed || 0),
    error_rate: Number(input?.error_rate || 0),
    timeout: Number(input?.timeout || 0),
    timeout_rate: Number(input?.timeout_rate || 0),
    avg_duration_ms: Number(input?.avg_duration_ms || 0),
    max_duration_ms: Number(input?.max_duration_ms || 0),
  }
}

function normalizeSnapshot(raw: unknown): PluginObservabilitySnapshot {
  if (raw && typeof raw === 'object') {
    const root = raw as Record<string, unknown>
    if (root.execution || root.generated_at) {
      return root as PluginObservabilitySnapshot
    }
    if (root.data && typeof root.data === 'object') {
      const nested = root.data as Record<string, unknown>
      if (nested.execution || nested.generated_at) {
        return nested as PluginObservabilitySnapshot
      }
    }
  }
  return {}
}

function normalizeFrontendResolverSnapshot(input?: PluginObservabilityFrontendResolverSnapshot) {
  return {
    cache_hits: Number(input?.cache_hits || 0),
    cache_misses: Number(input?.cache_misses || 0),
    cache_hit_rate: Number(input?.cache_hit_rate || 0),
    singleflight_waits: Number(input?.singleflight_waits || 0),
    catalog_hits: Number(input?.catalog_hits || 0),
    db_fallbacks: Number(input?.db_fallbacks || 0),
  }
}

function normalizeAdminPlugins(raw: unknown): AdminPlugin[] {
  if (Array.isArray(raw)) return raw as AdminPlugin[]
  if (raw && typeof raw === 'object') {
    const root = raw as Record<string, unknown>
    if (Array.isArray(root.data)) return root.data as AdminPlugin[]
    if (root.data && typeof root.data === 'object') {
      const nested = root.data as Record<string, unknown>
      if (Array.isArray(nested.data)) return nested.data as AdminPlugin[]
      if (Array.isArray(nested.items)) return nested.items as AdminPlugin[]
    }
    if (Array.isArray(root.items)) return root.items as AdminPlugin[]
  }
  return []
}

function normalizeObservabilityPluginParam(value: string | null): string {
  const trimmed = String(value || '').trim()
  if (!trimmed || trimmed === 'all') return 'all'
  if (!Number.isFinite(Number(trimmed)) || Number(trimmed) <= 0) return 'all'
  return String(Number(trimmed))
}

function normalizeObservabilityWindowParam(value: string | null): '24' | '168' {
  return value === '168' ? '168' : '24'
}

function normalizeDetailDialogContent(value?: string): string {
  const trimmed = String(value || '').trim()
  return trimmed || '-'
}

function toPercent(value: number): string {
  return `${(Number(value || 0) * 100).toFixed(2)}%`
}

function toDuration(value: number): string {
  return `${Number(value || 0).toFixed(2)} ms`
}

function toDateTime(value?: string, locale?: string): string {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString(locale === 'zh' ? 'zh-CN' : 'en-US', { hour12: false })
}

function pluginLifecycleText(value: string | undefined, t: ReturnType<typeof getTranslations>): string {
  switch (String(value || '').trim().toLowerCase()) {
    case 'draft':
      return t.admin.pluginLifecycleDraft
    case 'uploaded':
      return t.admin.pluginLifecycleUploaded
    case 'installed':
      return t.admin.pluginLifecycleInstalled
    case 'running':
      return t.admin.pluginLifecycleRunning
    case 'paused':
      return t.admin.pluginLifecyclePaused
    case 'degraded':
      return t.admin.pluginLifecycleDegraded
    case 'retired':
      return t.admin.pluginLifecycleRetired
    default:
      return String(value || '').trim() || '-'
  }
}

function pluginHealthText(value: string | undefined, t: ReturnType<typeof getTranslations>): string {
  switch (String(value || '').trim().toLowerCase()) {
    case 'healthy':
      return t.admin.pluginStatusHealthy
    case 'unhealthy':
      return t.admin.pluginStatusUnhealthy
    case 'unknown':
      return t.admin.pluginStatusUnknown
    default:
      return String(value || '').trim() || '-'
  }
}

function toHourAxisLabel(value?: string, locale?: string): string {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString(locale === 'zh' ? 'zh-CN' : 'en-US', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  })
}

function compactAxisLabel(value: string, maxLength = 16): string {
  const normalized = String(value || '').trim()
  if (normalized.length <= maxLength) return normalized
  return `${normalized.slice(0, maxLength - 1)}…`
}

function pluginLabel(pluginId: number, pluginName: string): string {
  return pluginName || `#${pluginId}`
}

function normalizeErrorText(value: string): string {
  const trimmed = String(value || '').trim()
  if (!trimmed) return 'unknown error'
  const [firstLine = ''] = trimmed.split(/\r?\n/, 1)
  const normalized = firstLine
    .trim()
    .split(/\s+/)
    .filter(Boolean)
    .join(' ')
  return normalized || 'unknown error'
}

function normalizeErrorSignature(value: string): string {
  const normalized = String(value || '')
    .trim()
    .toLowerCase()
    .replace(/\d+/g, '#')
    .split(/\s+/)
    .filter(Boolean)
    .join(' ')
  if (!normalized) return 'unknown error'
  if (normalized.length > pluginObservabilityErrorSignatureMaxLength) {
    return `${normalized.slice(0, pluginObservabilityErrorSignatureMaxLength - 1)}…`
  }
  return normalized
}

function isSameRecentFailureFilter(
  left: RecentFailureFilter | null,
  right: RecentFailureFilter | null
): boolean {
  if (!left || !right || left.kind !== right.kind || left.plugin_id !== right.plugin_id) {
    return false
  }
  switch (left.kind) {
    case 'failure_group':
      return (
        right.kind === 'failure_group' &&
        left.action === right.action &&
        left.hook === right.hook
      )
    case 'hook_group':
      return right.kind === 'hook_group' && left.hook === right.hook
    case 'error_signature':
      return right.kind === 'error_signature' && left.signature === right.signature
    default:
      return false
  }
}

function hotspotRowClassName(active: boolean): string {
  const base =
    'border-t cursor-pointer transition-colors hover:bg-muted/30 focus-visible:bg-muted/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring'
  return active ? `${base} bg-muted/40 hover:bg-muted/40` : base
}

function isHotspotToggleKey(event: KeyboardEvent<HTMLTableRowElement>): boolean {
  return event.key === 'Enter' || event.key === ' '
}

function byTotalDesc(a: CounterRow, b: CounterRow): number {
  if (a.counters.total === b.counters.total) {
    return a.key.localeCompare(b.key)
  }
  return b.counters.total - a.counters.total
}

function breakerStateVariant(state: string): 'active' | 'destructive' | 'outline' | 'secondary' {
  switch (state) {
    case 'open':
      return 'destructive'
    case 'half_open':
      return 'secondary'
    case 'closed':
      return 'active'
    default:
      return 'outline'
  }
}

function useChartTheme() {
  const { resolvedTheme } = useTheme()
  const isDark = resolvedTheme === 'dark'

  return {
    tickColor: isDark ? '#a1a1aa' : '#71717a',
    gridColor: isDark ? '#27272a' : '#e4e4e7',
    textColor: isDark ? '#e4e4e7' : '#18181b',
    tooltipBg: isDark ? '#1c1c22' : '#ffffff',
    tooltipBorder: isDark ? '#3f3f46' : '#e4e4e7',
    cursorFill: isDark ? 'rgba(255,255,255,0.06)' : 'rgba(0,0,0,0.06)',
  }
}

function MetricCard({
  title,
  value,
  description,
  icon,
}: {
  title: string
  value: string
  description?: string
  icon: ReactNode
}) {
  return (
    <Card>
      <CardHeader className="pb-2">
        <CardDescription className="flex items-center gap-2">
          {icon}
          <span>{title}</span>
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div className="text-2xl font-semibold">{value}</div>
        {description ? <p className="mt-1 text-xs text-muted-foreground">{description}</p> : null}
      </CardContent>
    </Card>
  )
}

function AdminPluginObservabilityPageContent() {
  const router = useRouter()
  const pathname = usePathname() || '/admin/plugins/observability'
  const searchParams = useSearchParams()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  const toast = useToast()
  usePageTitle(t.pageTitle.adminPluginObservability)
  const { hasPermission, isSuperAdmin } = usePermission()
  const chart = useChartTheme()
  const selectedPluginParam = normalizeObservabilityPluginParam(searchParams.get(observabilityPluginParamKey))
  const selectedWindowParam = normalizeObservabilityWindowParam(searchParams.get(observabilityWindowParamKey))
  const [selectedPluginId, setSelectedPluginId] = useState<string>(selectedPluginParam)
  const [windowHours, setWindowHours] = useState<'24' | '168'>(selectedWindowParam)
  const [recentFailureFilter, setRecentFailureFilter] = useState<RecentFailureFilter | null>(null)
  const [detailDialog, setDetailDialog] = useState<DetailDialogState>(null)

  const [permissionReady, setPermissionReady] = useState(false)
  useEffect(() => {
    setPermissionReady(true)
  }, [])

  useEffect(() => {
    setSelectedPluginId(selectedPluginParam)
  }, [selectedPluginParam])

  useEffect(() => {
    setWindowHours(selectedWindowParam)
  }, [selectedWindowParam])

  useEffect(() => {
    setRecentFailureFilter(null)
  }, [selectedPluginId, windowHours])

  const canManage = permissionReady && isSuperAdmin() && hasPermission('system.config')

  const selectedPluginIdNumber =
    selectedPluginId !== 'all' && Number.isFinite(Number(selectedPluginId))
      ? Number(selectedPluginId)
      : undefined

  const adminPluginsQuery = useQuery({
    queryKey: ['adminPluginsForObservability'],
    queryFn: getAdminPlugins,
    enabled: canManage,
    staleTime: 60000,
  })

  const observabilityQuery = useQuery({
    queryKey: ['adminPluginObservability', selectedPluginIdNumber || 0, windowHours],
    queryFn: () =>
      getAdminPluginObservability({
        plugin_id: selectedPluginIdNumber,
        hours: Number(windowHours),
      }),
    enabled: canManage,
    refetchInterval: 15000,
  })

  const pluginOptions = useMemo<PluginOption[]>(() => {
    return normalizeAdminPlugins(adminPluginsQuery.data)
      .map((item) => ({
        id: Number(item.id || 0),
        label: String(item.display_name || item.name || '').trim(),
      }))
      .filter((item) => item.id > 0 && item.label)
      .sort((a, b) => a.label.localeCompare(b.label))
  }, [adminPluginsQuery.data])

  const snapshot = useMemo(() => normalizeSnapshot(observabilityQuery.data), [observabilityQuery.data])
  const overall = normalizeCounters(snapshot.execution?.overall)

  const runtimeRows = useMemo<CounterRow[]>(() => {
    const source = snapshot.execution?.by_runtime || {}
    return Object.entries(source)
      .map(([key, counters]) => ({ key, counters: normalizeCounters(counters) }))
      .sort(byTotalDesc)
  }, [snapshot.execution?.by_runtime])

  const actionRows = useMemo<CounterRow[]>(() => {
    const source = snapshot.execution?.by_action || {}
    return Object.entries(source)
      .map(([key, counters]) => ({ key, counters: normalizeCounters(counters) }))
      .sort(byTotalDesc)
  }, [snapshot.execution?.by_action])

  const pluginRows = useMemo(() => {
    const source = Array.isArray(snapshot.execution?.by_plugin) ? snapshot.execution.by_plugin : []
    return source
      .map((item) => ({
        plugin_id: Number(item.plugin_id || 0),
        plugin_name: String(item.plugin_name || ''),
        runtime: String(item.runtime || ''),
        counters: normalizeCounters(item),
      }))
      .sort((a, b) => {
        if (a.counters.total === b.counters.total) {
          return a.plugin_id - b.plugin_id
        }
        return b.counters.total - a.counters.total
      })
  }, [snapshot.execution?.by_plugin])

  const hookRows = useMemo(() => {
    const source = snapshot.hook_limiter?.by_hook || {}
    return Object.entries(source)
      .map(([hook, hits]) => ({ hook, hits: Number(hits || 0) }))
      .sort((a, b) => {
        if (a.hits === b.hits) return a.hook.localeCompare(b.hook)
        return b.hits - a.hits
      })
  }, [snapshot.hook_limiter?.by_hook])

  const endpointRows = useMemo(() => {
    const source = snapshot.public_access || {}
    return Object.entries(source)
      .map(([endpoint, metrics]) => ({
        endpoint,
        requests: Number(metrics?.requests || 0),
        rate_limited: Number(metrics?.rate_limited || 0),
        rate_limit_hit_rate: Number(metrics?.rate_limit_hit_rate || 0),
        cache_hits: Number(metrics?.cache_hits || 0),
        cache_misses: Number(metrics?.cache_misses || 0),
        cache_hit_rate: Number(metrics?.cache_hit_rate || 0),
      }))
      .sort((a, b) => a.endpoint.localeCompare(b.endpoint))
  }, [snapshot.public_access])

  const frontendResolverRows = useMemo<FrontendResolverRow[]>(() => {
    return [
      {
        key: 'prepared_hook',
        label: t.admin.pluginObservabilityPreparedHook,
        ...normalizeFrontendResolverSnapshot(snapshot.frontend?.prepared_hook),
      },
      {
        key: 'html_mode',
        label: t.admin.pluginObservabilityHtmlMode,
        ...normalizeFrontendResolverSnapshot(snapshot.frontend?.html_mode),
      },
      {
        key: 'execute_api',
        label: t.admin.pluginObservabilityExecuteApi,
        ...normalizeFrontendResolverSnapshot(snapshot.frontend?.execute_api),
      },
    ]
  }, [
    snapshot.frontend?.execute_api,
    snapshot.frontend?.html_mode,
    snapshot.frontend?.prepared_hook,
    t.admin.pluginObservabilityExecuteApi,
    t.admin.pluginObservabilityHtmlMode,
    t.admin.pluginObservabilityPreparedHook,
  ])

  const breakerRows = useMemo<BreakerRow[]>(() => {
    const source = Array.isArray(snapshot.breaker_overview?.rows) ? snapshot.breaker_overview.rows : []
    return source.map((row) => ({
      plugin_id: Number(row.plugin_id || 0),
      plugin_name: String(row.plugin_name || ''),
      runtime: String(row.runtime || ''),
      enabled: Boolean(row.enabled),
      lifecycle_status: String(row.lifecycle_status || ''),
      health_status: String(row.health_status || ''),
      breaker_state: String(row.breaker_state || ''),
      failure_count: Number(row.failure_count || 0),
      failure_threshold: Number(row.failure_threshold || 0),
      cooldown_active: Boolean(row.cooldown_active),
      cooldown_until: row.cooldown_until || undefined,
      cooldown_reason: String(row.cooldown_reason || ''),
      probe_in_flight: Boolean(row.probe_in_flight),
      probe_started_at: row.probe_started_at || undefined,
      window_total_executions: Number(row.window_total_executions || 0),
      window_failed_executions: Number(row.window_failed_executions || 0),
    }))
  }, [snapshot.breaker_overview?.rows])

  const executionTrendRows = useMemo<ExecutionTrendRow[]>(() => {
    const source = Array.isArray(snapshot.execution_window?.by_hour) ? snapshot.execution_window.by_hour : []
    return source.map((item) => ({
      hour_start: String(item.hour_start || ''),
      total: Number(item.total_executions || 0),
      failed: Number(item.failed_executions || 0),
    }))
  }, [snapshot.execution_window?.by_hour])

  const failureGroupRows = useMemo<FailureGroupRow[]>(() => {
    const source = Array.isArray(snapshot.execution_window?.failure_groups)
      ? snapshot.execution_window.failure_groups
      : []
    return source.map((item) => ({
      plugin_id: Number(item.plugin_id || 0),
      plugin_name: String(item.plugin_name || ''),
      action: String(item.action || ''),
      hook: String(item.hook || ''),
      failure_count: Number(item.failure_count || 0),
      last_failure_at: item.last_failure_at || undefined,
    }))
  }, [snapshot.execution_window?.failure_groups])

  const hookGroupRows = useMemo<HookGroupRow[]>(() => {
    const source = Array.isArray(snapshot.execution_window?.hook_groups)
      ? snapshot.execution_window.hook_groups
      : []
    return source.map((item) => ({
      plugin_id: Number(item.plugin_id || 0),
      plugin_name: String(item.plugin_name || ''),
      hook: String(item.hook || ''),
      failure_count: Number(item.failure_count || 0),
      last_failure_at: item.last_failure_at || undefined,
      last_error: String(item.last_error || ''),
    }))
  }, [snapshot.execution_window?.hook_groups])

  const errorSignatureRows = useMemo<ErrorSignatureRow[]>(() => {
    const source = Array.isArray(snapshot.execution_window?.error_signatures)
      ? snapshot.execution_window.error_signatures
      : []
    return source.map((item) => ({
      plugin_id: Number(item.plugin_id || 0),
      plugin_name: String(item.plugin_name || ''),
      signature: String(item.signature || ''),
      failure_count: Number(item.failure_count || 0),
      last_failure_at: item.last_failure_at || undefined,
      sample_error: String(item.sample_error || ''),
    }))
  }, [snapshot.execution_window?.error_signatures])

  const recentFailureRows = useMemo<FailureSampleRow[]>(() => {
    const source = Array.isArray(snapshot.execution_window?.recent_failures)
      ? snapshot.execution_window.recent_failures
      : []
    return source.map((item) => ({
      id: Number(item.id || 0),
      plugin_id: Number(item.plugin_id || 0),
      plugin_name: String(item.plugin_name || ''),
      action: String(item.action || ''),
      hook: String(item.hook || ''),
      error: String(item.error || ''),
      duration: Number(item.duration || 0),
      created_at: item.created_at || undefined,
    }))
  }, [snapshot.execution_window?.recent_failures])

  const filteredRecentFailureRows = useMemo<FailureSampleRow[]>(() => {
    if (!recentFailureFilter) return recentFailureRows
    return recentFailureRows.filter((row) => {
      if (row.plugin_id !== recentFailureFilter.plugin_id) return false
      switch (recentFailureFilter.kind) {
        case 'failure_group':
          return row.action === recentFailureFilter.action && row.hook === recentFailureFilter.hook
        case 'hook_group':
          return row.hook === recentFailureFilter.hook
        case 'error_signature':
          return (
            normalizeErrorSignature(normalizeErrorText(row.error)) === recentFailureFilter.signature
          )
        default:
          return true
      }
    })
  }, [recentFailureFilter, recentFailureRows])

  const runtimeChartData = runtimeRows.slice(0, 10).map((item) => ({
    name: item.key,
    total: item.counters.total,
    failed: item.counters.failed,
    timeout: item.counters.timeout,
  }))
  const actionChartData = actionRows.slice(0, 10).map((item) => ({
    name: item.key,
    total: item.counters.total,
    failed: item.counters.failed,
  }))

  const selectedWindowLabel =
    windowHours === '168' ? t.admin.pluginObservabilityWindow7d : t.admin.pluginObservabilityWindow24h
  const selectedPluginLabel =
    selectedPluginId === 'all'
      ? t.admin.pluginObservabilityAllPlugins
      : pluginOptions.find((option) => String(option.id) === selectedPluginId)?.label ||
        `#${selectedPluginId}`

  const recentFailureFilterLabel = recentFailureFilter
    ? (() => {
        const currentPluginLabel = pluginLabel(
          recentFailureFilter.plugin_id,
          recentFailureFilter.plugin_name
        )
        switch (recentFailureFilter.kind) {
          case 'failure_group':
            return `${currentPluginLabel} · ${t.admin.pluginObservabilityAction}: ${recentFailureFilter.action || '-'} · ${t.admin.pluginObservabilityHook}: ${recentFailureFilter.hook || '-'}`
          case 'hook_group':
            return `${currentPluginLabel} · ${t.admin.pluginObservabilityHook}: ${recentFailureFilter.hook || '-'}`
          case 'error_signature':
            return `${currentPluginLabel} · ${t.admin.pluginObservabilitySignature}: ${recentFailureFilter.signature || '-'}`
          default:
            return currentPluginLabel
        }
      })()
    : ''
  const recentFailureFilterSummary = recentFailureFilter
    ? t.admin.pluginObservabilityRecentFailuresFilteredBy.replace(
        '{scope}',
        recentFailureFilterLabel
      )
    : ''
  const recentFailureFilterCount = `${filteredRecentFailureRows.length}/${recentFailureRows.length}`

  const breakerStateLabels = {
    open: t.admin.pluginObservabilityBreakerOpen,
    half_open: t.admin.pluginObservabilityBreakerHalfOpen,
    closed: t.admin.pluginObservabilityBreakerClosed,
    unknown: t.admin.pluginObservabilityBreakerUnknown,
  }

  const openBreakers = Number(snapshot.breaker_overview?.open_count || 0)
  const halfOpenBreakers = Number(snapshot.breaker_overview?.half_open_count || 0)
  const probeInFlightCount = Number(snapshot.breaker_overview?.probe_in_flight_count || 0)
  const cooldownActiveCount = Number(snapshot.breaker_overview?.cooldown_active_count || 0)
  const enabledPlugins = Number(snapshot.breaker_overview?.enabled_plugins || 0)
  const totalPlugins = Number(snapshot.breaker_overview?.total_plugins || 0)
  const windowFailedExecutions = Number(snapshot.execution_window?.failed_executions || 0)
  const hookFailedExecutions = Number(snapshot.execution_window?.hook_failed_executions || 0)
  const actionFailedExecutions = Number(snapshot.execution_window?.action_failed_executions || 0)
  const frontendSlotRequests = Number(snapshot.frontend?.slot_requests || 0)
  const frontendBatchRequests = Number(snapshot.frontend?.batch_requests || 0)
  const frontendBootstrapRequests = Number(snapshot.frontend?.bootstrap_requests || 0)
  const frontendBatchItems = Number(snapshot.frontend?.batch_items || 0)
  const frontendBatchUniqueItems = Number(snapshot.frontend?.batch_unique_items || 0)
  const frontendBatchDedupedItems = Number(snapshot.frontend?.batch_deduped_items || 0)
  const frontendBatchDedupeRate =
    frontendBatchItems > 0 ? frontendBatchDedupedItems / frontendBatchItems : 0
  const adminPluginObservabilityPluginContext = {
    view: 'admin_plugin_observability',
    filters: {
      plugin_id: selectedPluginIdNumber,
      plugin_param: selectedPluginId,
      window_hours: Number(windowHours),
    },
    selection: {
      plugin_count: pluginOptions.length,
      selected_plugin_label: selectedPluginLabel,
    },
    summary: {
      generated_at: snapshot.generated_at,
      total_executions: overall.total,
      failed_executions: overall.failed,
      timeout_count: overall.timeout,
      open_breakers: openBreakers,
      half_open_breakers: halfOpenBreakers,
      enabled_plugins: enabledPlugins,
      total_plugins: totalPlugins,
      frontend_slot_requests: frontendSlotRequests,
      frontend_batch_requests: frontendBatchRequests,
      frontend_bootstrap_requests: frontendBootstrapRequests,
      frontend_batch_dedupe_rate: frontendBatchDedupeRate,
    },
  }

  const tooltipStyle = {
    contentStyle: {
      backgroundColor: chart.tooltipBg,
      border: `1px solid ${chart.tooltipBorder}`,
      borderRadius: '8px',
    },
    labelStyle: { color: chart.textColor },
    itemStyle: { color: chart.textColor },
    cursor: { fill: chart.cursorFill },
  }

  const updateFilterURL = (nextPluginId: string, nextWindowHours: '24' | '168') => {
    const params = new URLSearchParams(searchParams.toString())
    if (nextPluginId === 'all') {
      params.delete(observabilityPluginParamKey)
    } else {
      params.set(observabilityPluginParamKey, nextPluginId)
    }
    if (nextWindowHours === '24') {
      params.delete(observabilityWindowParamKey)
    } else {
      params.set(observabilityWindowParamKey, nextWindowHours)
    }
    const queryString = params.toString()
    router.replace(queryString ? `${pathname}?${queryString}` : pathname, { scroll: false })
  }

  const handlePluginFilterChange = (value: string) => {
    const nextValue = normalizeObservabilityPluginParam(value)
    setSelectedPluginId(nextValue)
    updateFilterURL(nextValue, windowHours)
  }

  const handleWindowHoursChange = (value: string) => {
    const nextValue = normalizeObservabilityWindowParam(value)
    setWindowHours(nextValue)
    updateFilterURL(selectedPluginId, nextValue)
  }

  const openDetailDialog = (title: string, content?: string) => {
    setDetailDialog({
      title,
      content: normalizeDetailDialogContent(content),
    })
  }

  const copyDetailDialogContent = async () => {
    if (!detailDialog) return
    try {
      await navigator.clipboard.writeText(detailDialog.content)
      toast.success(t.common.copiedToClipboard)
    } catch {
      toast.error(t.common.failed)
    }
  }

  const renderDetailCell = (
    value: string | undefined,
    detailTitle: string,
    maxWidthClassName = 'max-w-[320px]'
  ) => {
    const normalizedValue = normalizeDetailDialogContent(value)
    const canOpen = normalizedValue !== '-'
    return (
      <div className="flex items-center gap-2">
        <div className={`min-w-0 flex-1 truncate ${maxWidthClassName}`} title={normalizedValue}>
          {normalizedValue}
        </div>
        {canOpen ? (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="h-7 shrink-0 px-2"
            onClick={(event) => {
              event.preventDefault()
              event.stopPropagation()
              openDetailDialog(detailTitle, normalizedValue)
            }}
          >
            {t.common.detail}
          </Button>
        ) : null}
      </div>
    )
  }

  const toggleRecentFailureFilter = (nextFilter: RecentFailureFilter) => {
    setRecentFailureFilter((current) =>
      isSameRecentFailureFilter(current, nextFilter) ? null : nextFilter
    )
  }

  const handleHotspotRowKeyDown = (
    event: KeyboardEvent<HTMLTableRowElement>,
    nextFilter: RecentFailureFilter
  ) => {
    if (!isHotspotToggleKey(event)) return
    event.preventDefault()
    toggleRecentFailureFilter(nextFilter)
  }

  const isFailureGroupFilterActive = (row: FailureGroupRow): boolean =>
    recentFailureFilter?.kind === 'failure_group' &&
    recentFailureFilter.plugin_id === row.plugin_id &&
    recentFailureFilter.action === row.action &&
    recentFailureFilter.hook === row.hook

  const isHookGroupFilterActive = (row: HookGroupRow): boolean =>
    recentFailureFilter?.kind === 'hook_group' &&
    recentFailureFilter.plugin_id === row.plugin_id &&
    recentFailureFilter.hook === row.hook

  const isErrorSignatureFilterActive = (row: ErrorSignatureRow): boolean =>
    recentFailureFilter?.kind === 'error_signature' &&
    recentFailureFilter.plugin_id === row.plugin_id &&
    recentFailureFilter.signature === row.signature

  if (permissionReady && !canManage) {
    return (
      <div className="space-y-6">
        <h1 className="text-3xl font-bold">{t.pageTitle.adminPluginObservability}</h1>
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
      <PluginSlot
        slot="admin.plugins.observability.top"
        context={adminPluginObservabilityPluginContext}
      />
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="text-3xl font-bold">{t.pageTitle.adminPluginObservability}</h1>
          <p className="mt-1 text-muted-foreground">{t.admin.pluginObservabilitySubtitle}</p>
          <div className="mt-3 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
            <span>
              {t.admin.pluginObservabilityGeneratedAt}: {toDateTime(snapshot.generated_at, locale)}
            </span>
            {observabilityQuery.isFetching ? <span>{t.common.processing}</span> : null}
          </div>
          <div className="mt-2 space-y-1">
            <p className="text-xs text-muted-foreground">{t.admin.pluginObservabilityScopedHint}</p>
          </div>
        </div>
        <div className="flex min-w-0 flex-col gap-3 sm:items-end">
          <div className="flex flex-wrap gap-2">
            <Button
              variant="outline"
              onClick={() => observabilityQuery.refetch()}
              disabled={observabilityQuery.isFetching}
            >
              {t.admin.refresh}
            </Button>
            <Button asChild variant="outline">
              <Link href="/admin/plugins">
                <ArrowLeft className="mr-2 h-4 w-4" />
                {t.admin.pluginObservabilityBackToPlugins}
              </Link>
            </Button>
          </div>
          <div className="grid gap-3 sm:grid-cols-[minmax(220px,260px)_auto]">
            <div className="min-w-0 space-y-2">
              <p className="text-xs text-muted-foreground">
                {t.admin.pluginObservabilityFilterPlugin}
              </p>
              <Select value={selectedPluginId} onValueChange={handlePluginFilterChange}>
                <SelectTrigger>
                  <SelectValue placeholder={t.admin.pluginObservabilityAllPlugins} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">{t.admin.pluginObservabilityAllPlugins}</SelectItem>
                  {pluginOptions.map((option) => (
                    <SelectItem key={option.id} value={String(option.id)}>
                      {option.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="min-w-0 space-y-2">
              <p className="text-xs text-muted-foreground">
                {t.admin.pluginObservabilityFilterWindow}
              </p>
              <Tabs value={windowHours} onValueChange={handleWindowHoursChange}>
                <TabsList className="grid w-full grid-cols-2 sm:w-[220px]">
                  <TabsTrigger value="24">{t.admin.pluginObservabilityWindow24h}</TabsTrigger>
                  <TabsTrigger value="168">{t.admin.pluginObservabilityWindow7d}</TabsTrigger>
                </TabsList>
              </Tabs>
            </div>
          </div>
        </div>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
        <MetricCard
          title={t.admin.pluginObservabilityTotalExecutions}
          value={String(overall.total)}
          icon={<Activity className="h-4 w-4" />}
        />
        <MetricCard
          title={t.admin.pluginObservabilityErrorRate}
          value={toPercent(overall.error_rate)}
          description={`${t.admin.pluginObservabilityFailed}: ${overall.failed}`}
          icon={<ShieldAlert className="h-4 w-4" />}
        />
        <MetricCard
          title={t.admin.pluginObservabilityTimeoutRate}
          value={toPercent(overall.timeout_rate)}
          description={`${t.admin.pluginObservabilityTimeout}: ${overall.timeout}`}
          icon={<TimerReset className="h-4 w-4" />}
        />
        <MetricCard
          title={t.admin.pluginObservabilityAvgDuration}
          value={toDuration(overall.avg_duration_ms)}
          icon={<Clock3 className="h-4 w-4" />}
        />
        <MetricCard
          title={t.admin.pluginObservabilityMaxDuration}
          value={toDuration(overall.max_duration_ms)}
          icon={<Gauge className="h-4 w-4" />}
        />
        <MetricCard
          title={t.admin.pluginObservabilityLimiterHits}
          value={String(Number(snapshot.hook_limiter?.total_hits || 0))}
          icon={<Zap className="h-4 w-4" />}
        />
      </div>

      <Card>
        <CardHeader className="space-y-3">
          <div className="space-y-1">
            <CardTitle>{t.admin.pluginObservabilityFrontend}</CardTitle>
            <CardDescription>
              {t.admin.pluginObservabilityFrontendScopedHint.replace(
                '{rate}',
                toPercent(frontendBatchDedupeRate)
              )}
            </CardDescription>
          </div>
          <p className="text-xs text-muted-foreground">
            {[
              `${t.admin.pluginObservabilitySlotRequests}: ${frontendSlotRequests}`,
              `${t.admin.pluginObservabilityBootstrapRequests}: ${frontendBootstrapRequests}`,
              `${t.admin.pluginObservabilityBatchDedupedItems}: ${frontendBatchDedupedItems}`,
              `${t.admin.pluginObservabilityCacheHitRate}: ${toPercent(
                Number(snapshot.frontend?.prepared_hook?.cache_hit_rate || 0)
              )}`,
              `${t.admin.pluginObservabilityBatchUniqueItems}: ${frontendBatchUniqueItems} / ${t.admin.pluginObservabilityBatchItems}: ${frontendBatchItems}`,
            ].join(' · ')}
          </p>
        </CardHeader>
        <CardContent>
          {frontendResolverRows.every(
            (row) =>
              row.cache_hits === 0 &&
              row.cache_misses === 0 &&
              row.singleflight_waits === 0 &&
              row.catalog_hits === 0 &&
              row.db_fallbacks === 0
          ) ? (
            <p className="text-sm text-muted-foreground">{t.admin.pluginObservabilityNoData}</p>
          ) : (
            <div className="overflow-auto rounded-md border">
              <table className="w-full text-sm">
                <thead className="bg-muted/40">
                  <tr className="text-left">
                    <th className="px-3 py-2">{t.admin.pluginObservabilityResolver}</th>
                    <th className="px-3 py-2">{t.admin.pluginObservabilityCacheHits}</th>
                    <th className="px-3 py-2">{t.admin.pluginObservabilityCacheMisses}</th>
                    <th className="px-3 py-2">{t.admin.pluginObservabilityCacheHitRate}</th>
                    <th className="px-3 py-2">{t.admin.pluginObservabilitySingleflightWaits}</th>
                    <th className="px-3 py-2">{t.admin.pluginObservabilityCatalogHits}</th>
                    <th className="px-3 py-2">{t.admin.pluginObservabilityDbFallbacks}</th>
                  </tr>
                </thead>
                <tbody>
                  {frontendResolverRows.map((row) => (
                    <tr key={row.key} className="border-t">
                      <td className="px-3 py-2">{row.label}</td>
                      <td className="px-3 py-2">{row.cache_hits}</td>
                      <td className="px-3 py-2">{row.cache_misses}</td>
                      <td className="px-3 py-2">{toPercent(row.cache_hit_rate)}</td>
                      <td className="px-3 py-2">{row.singleflight_waits}</td>
                      <td className="px-3 py-2">{row.catalog_hits}</td>
                      <td className="px-3 py-2">{row.db_fallbacks}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>
            {selectedWindowLabel} · {t.admin.pluginObservabilityExecutionTrendTitle}
          </CardTitle>
          <CardDescription>
            {t.admin.pluginObservabilityLastSuccess}:{' '}
            {toDateTime(snapshot.execution_window?.last_success_at, locale)} ·{' '}
            {t.admin.pluginObservabilityLastFailure}:{' '}
            {toDateTime(snapshot.execution_window?.last_failure_at, locale)}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {executionTrendRows.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t.admin.pluginObservabilityNoData}</p>
          ) : (
            <ResponsiveContainer width="100%" height={320}>
              <LineChart data={executionTrendRows} margin={{ left: 8, right: 8 }}>
                <CartesianGrid strokeDasharray="3 3" stroke={chart.gridColor} />
                <XAxis
                  dataKey="hour_start"
                  tick={{ fontSize: 12, fill: chart.tickColor }}
                  stroke={chart.gridColor}
                  tickFormatter={(value) =>
                    compactAxisLabel(toHourAxisLabel(String(value), locale), 12)
                  }
                />
                <YAxis
                  tick={{ fontSize: 12, fill: chart.tickColor }}
                  stroke={chart.gridColor}
                />
                <Tooltip
                  {...tooltipStyle}
                  labelFormatter={(value) => toDateTime(String(value), locale)}
                />
                <Line
                  type="monotone"
                  dataKey="total"
                  stroke="#2563eb"
                  strokeWidth={2}
                  dot={false}
                  name={t.admin.pluginObservabilityTotal}
                />
                <Line
                  type="monotone"
                  dataKey="failed"
                  stroke="#dc2626"
                  strokeWidth={2}
                  dot={false}
                  name={t.admin.pluginObservabilityFailed}
                />
              </LineChart>
            </ResponsiveContainer>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="space-y-3">
          <CardTitle>{t.admin.pluginObservabilityBreakerOverview}</CardTitle>
          <p className="text-xs text-muted-foreground">
            {[
              `${enabledPlugins}/${totalPlugins} ${t.admin.pluginObservabilityEnabledPlugins}`,
              `${t.admin.pluginObservabilityOpenBreakers}: ${openBreakers}`,
              `${t.admin.pluginObservabilityHalfOpenBreakers}: ${halfOpenBreakers}`,
              `${t.admin.pluginObservabilityRecoveryProbes}: ${probeInFlightCount}`,
              `${selectedWindowLabel} · ${t.admin.pluginObservabilityWindowFailuresTitle}: ${windowFailedExecutions}`,
              `${cooldownActiveCount} ${t.admin.pluginObservabilityCooldownActive}`,
              `${t.admin.pluginObservabilityHookFailed}: ${hookFailedExecutions} / ${t.admin.pluginObservabilityActionFailed}: ${actionFailedExecutions}`,
            ].join(' · ')}
          </p>
        </CardHeader>
        <CardContent>
          {breakerRows.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t.admin.pluginObservabilityNoData}</p>
          ) : (
            <div className="overflow-auto rounded-md border">
              <table className="w-full text-sm">
                <thead className="bg-muted/40">
                  <tr className="text-left">
                    <th className="px-3 py-2">{t.admin.pluginObservabilityPlugin}</th>
                    <th className="px-3 py-2">{t.admin.pluginObservabilityState}</th>
                    <th className="px-3 py-2">{t.admin.pluginObservabilityRuntime}</th>
                    <th className="px-3 py-2">{t.admin.pluginObservabilityFailureCount}</th>
                    <th className="px-3 py-2">{t.admin.pluginObservabilityThreshold}</th>
                    <th className="px-3 py-2">{t.admin.pluginObservabilityWindowFailed}</th>
                    <th className="px-3 py-2">{t.admin.pluginObservabilityWindowTotal}</th>
                    <th className="px-3 py-2">{t.admin.pluginObservabilityCooldownUntil}</th>
                  </tr>
                </thead>
                <tbody>
                  {breakerRows.map((row) => {
                    const pluginLabel = row.plugin_name || `#${row.plugin_id}`
                    const stateLabel =
                      breakerStateLabels[row.breaker_state as keyof typeof breakerStateLabels] ||
                      t.admin.pluginObservabilityBreakerUnknown
                    const pluginStateSummary = [
                      row.lifecycle_status ? pluginLifecycleText(row.lifecycle_status, t) : '',
                      row.health_status ? pluginHealthText(row.health_status, t) : '',
                    ]
                      .filter(Boolean)
                      .join(' / ')
                    const stateDetail =
                      row.cooldown_reason ||
                      (row.probe_in_flight ? t.admin.pluginObservabilityRecoveryProbes : '')

                    return (
                      <tr key={`${row.plugin_id}-${row.runtime}-${row.breaker_state}`} className="border-t">
                        <td className="px-3 py-2 align-top">
                          <div className="font-medium">{pluginLabel}</div>
                          <div className="text-xs text-muted-foreground">
                            #{row.plugin_id}
                            {pluginStateSummary
                              ? ` · ${pluginStateSummary}`
                              : ''}
                          </div>
                        </td>
                        <td className="px-3 py-2 align-top">
                          <Badge variant={breakerStateVariant(row.breaker_state)}>{stateLabel}</Badge>
                          {stateDetail ? (
                            <p
                              className="mt-1 max-w-[280px] text-xs text-muted-foreground"
                              title={stateDetail}
                            >
                              {compactAxisLabel(stateDetail, 48)}
                            </p>
                          ) : null}
                        </td>
                        <td className="px-3 py-2 align-top">{row.runtime || '-'}</td>
                        <td className="px-3 py-2 align-top">{row.failure_count}</td>
                        <td className="px-3 py-2 align-top">{row.failure_threshold || '-'}</td>
                        <td className="px-3 py-2 align-top">{row.window_failed_executions}</td>
                        <td className="px-3 py-2 align-top">{row.window_total_executions}</td>
                        <td className="px-3 py-2 align-top">
                          {toDateTime(row.cooldown_until, locale)}
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>

      <div className="grid gap-4 xl:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>{t.admin.pluginObservabilityFailureHotspots}</CardTitle>
          </CardHeader>
          <CardContent>
            {failureGroupRows.length === 0 ? (
              <p className="text-sm text-muted-foreground">{t.admin.pluginObservabilityNoData}</p>
            ) : (
              <div className="overflow-auto rounded-md border">
                <table className="w-full text-sm">
                  <thead className="bg-muted/40">
                    <tr className="text-left">
                      <th className="px-3 py-2">{t.admin.pluginObservabilityPlugin}</th>
                      <th className="px-3 py-2">{t.admin.pluginObservabilityAction}</th>
                      <th className="px-3 py-2">{t.admin.pluginObservabilityHook}</th>
                      <th className="px-3 py-2">{t.admin.pluginObservabilityFailed}</th>
                      <th className="px-3 py-2">{t.admin.pluginObservabilityLastFailure}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {failureGroupRows.map((row) => {
                      const active = isFailureGroupFilterActive(row)
                      const nextFilter: RecentFailureFilter = {
                        kind: 'failure_group',
                        plugin_id: row.plugin_id,
                        plugin_name: row.plugin_name,
                        action: row.action,
                        hook: row.hook,
                      }
                      return (
                        <tr
                          key={`${row.plugin_id}-${row.action}-${row.hook || 'none'}`}
                          className={hotspotRowClassName(active)}
                          role="button"
                          tabIndex={0}
                          aria-pressed={active}
                          title={t.admin.pluginObservabilityFilterRecentFailures}
                          onClick={() => toggleRecentFailureFilter(nextFilter)}
                          onKeyDown={(event) => handleHotspotRowKeyDown(event, nextFilter)}
                        >
                          <td className="px-3 py-2">
                            {pluginLabel(row.plugin_id, row.plugin_name)}
                            {row.plugin_name ? (
                              <span className="ml-1 text-xs text-muted-foreground">
                                #{row.plugin_id}
                              </span>
                            ) : null}
                          </td>
                          <td className="px-3 py-2">{row.action || '-'}</td>
                          <td className="px-3 py-2">{row.hook || '-'}</td>
                          <td className="px-3 py-2">{row.failure_count}</td>
                          <td className="px-3 py-2">{toDateTime(row.last_failure_at, locale)}</td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
              </div>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="gap-3">
            <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
              <div className="space-y-1">
                <CardTitle>{t.admin.pluginObservabilityRecentFailures}</CardTitle>
                {recentFailureFilter ? (
                  <CardDescription>{recentFailureFilterSummary}</CardDescription>
                ) : null}
              </div>
              {recentFailureFilter ? (
                <div className="flex items-center gap-2">
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={() => setRecentFailureFilter(null)}
                  >
                    {t.admin.pluginObservabilityClearRecentFailureFilter}
                  </Button>
                </div>
              ) : null}
            </div>
          </CardHeader>
          <CardContent>
            {filteredRecentFailureRows.length === 0 ? (
              <p className="text-sm text-muted-foreground">
                {recentFailureFilter
                  ? t.admin.pluginObservabilityRecentFailuresNoMatch
                  : t.admin.pluginObservabilityNoData}
              </p>
            ) : (
              <div className="overflow-auto rounded-md border">
                <table className="w-full text-sm">
                  <thead className="bg-muted/40">
                    <tr className="text-left">
                      <th className="px-3 py-2">{t.admin.pluginObservabilityOccurredAt}</th>
                      <th className="px-3 py-2">{t.admin.pluginObservabilityPlugin}</th>
                      <th className="px-3 py-2">{t.admin.pluginObservabilityAction}</th>
                      <th className="px-3 py-2">{t.admin.pluginObservabilityHook}</th>
                      <th className="px-3 py-2">{t.admin.pluginObservabilityReason}</th>
                      <th className="px-3 py-2">{t.admin.pluginObservabilityAvgDuration}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {filteredRecentFailureRows.map((row) => (
                      <tr key={row.id || `${row.plugin_id}-${row.created_at}`} className="border-t">
                        <td className="px-3 py-2">{toDateTime(row.created_at, locale)}</td>
                        <td className="px-3 py-2">
                          {pluginLabel(row.plugin_id, row.plugin_name)}
                          {row.plugin_name ? (
                            <span className="ml-1 text-xs text-muted-foreground">
                              #{row.plugin_id}
                            </span>
                          ) : null}
                        </td>
                        <td className="px-3 py-2">{row.action || '-'}</td>
                        <td className="px-3 py-2">{row.hook || '-'}</td>
                        <td className="px-3 py-2">
                          {renderDetailCell(
                            row.error,
                            `${t.admin.pluginObservabilityReason} · ${pluginLabel(
                              row.plugin_id,
                              row.plugin_name
                            )}`
                          )}
                        </td>
                        <td className="px-3 py-2">{toDuration(row.duration)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      <div className="grid gap-4 xl:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>{t.admin.pluginObservabilityHookHotspots}</CardTitle>
          </CardHeader>
          <CardContent>
            {hookGroupRows.length === 0 ? (
              <p className="text-sm text-muted-foreground">{t.admin.pluginObservabilityNoData}</p>
            ) : (
              <div className="overflow-auto rounded-md border">
                <table className="w-full text-sm">
                  <thead className="bg-muted/40">
                    <tr className="text-left">
                      <th className="px-3 py-2">{t.admin.pluginObservabilityPlugin}</th>
                      <th className="px-3 py-2">{t.admin.pluginObservabilityHook}</th>
                      <th className="px-3 py-2">{t.admin.pluginObservabilityFailed}</th>
                      <th className="px-3 py-2">{t.admin.pluginObservabilityLastFailure}</th>
                      <th className="px-3 py-2">{t.admin.pluginObservabilityLastError}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {hookGroupRows.map((row) => {
                      const active = isHookGroupFilterActive(row)
                      const nextFilter: RecentFailureFilter = {
                        kind: 'hook_group',
                        plugin_id: row.plugin_id,
                        plugin_name: row.plugin_name,
                        hook: row.hook,
                      }
                      return (
                        <tr
                          key={`${row.plugin_id}-${row.hook}`}
                          className={hotspotRowClassName(active)}
                          role="button"
                          tabIndex={0}
                          aria-pressed={active}
                          title={t.admin.pluginObservabilityFilterRecentFailures}
                          onClick={() => toggleRecentFailureFilter(nextFilter)}
                          onKeyDown={(event) => handleHotspotRowKeyDown(event, nextFilter)}
                        >
                          <td className="px-3 py-2">
                            {pluginLabel(row.plugin_id, row.plugin_name)}
                            {row.plugin_name ? (
                              <span className="ml-1 text-xs text-muted-foreground">
                                #{row.plugin_id}
                              </span>
                            ) : null}
                          </td>
                          <td className="px-3 py-2">{row.hook || '-'}</td>
                          <td className="px-3 py-2">{row.failure_count}</td>
                          <td className="px-3 py-2">{toDateTime(row.last_failure_at, locale)}</td>
                          <td className="px-3 py-2">
                            {renderDetailCell(
                              row.last_error,
                              `${t.admin.pluginObservabilityLastError} · ${pluginLabel(
                                row.plugin_id,
                                row.plugin_name
                              )}`
                            )}
                          </td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
              </div>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t.admin.pluginObservabilityErrorSignatures}</CardTitle>
          </CardHeader>
          <CardContent>
            {errorSignatureRows.length === 0 ? (
              <p className="text-sm text-muted-foreground">{t.admin.pluginObservabilityNoData}</p>
            ) : (
              <div className="overflow-auto rounded-md border">
                <table className="w-full text-sm">
                  <thead className="bg-muted/40">
                    <tr className="text-left">
                      <th className="px-3 py-2">{t.admin.pluginObservabilityPlugin}</th>
                      <th className="px-3 py-2">{t.admin.pluginObservabilitySignature}</th>
                      <th className="px-3 py-2">{t.admin.pluginObservabilityFailed}</th>
                      <th className="px-3 py-2">{t.admin.pluginObservabilityLastFailure}</th>
                      <th className="px-3 py-2">{t.admin.pluginObservabilitySampleError}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {errorSignatureRows.map((row) => {
                      const active = isErrorSignatureFilterActive(row)
                      const nextFilter: RecentFailureFilter = {
                        kind: 'error_signature',
                        plugin_id: row.plugin_id,
                        plugin_name: row.plugin_name,
                        signature: row.signature,
                      }
                      return (
                        <tr
                          key={`${row.plugin_id}-${row.signature}`}
                          className={hotspotRowClassName(active)}
                          role="button"
                          tabIndex={0}
                          aria-pressed={active}
                          title={t.admin.pluginObservabilityFilterRecentFailures}
                          onClick={() => toggleRecentFailureFilter(nextFilter)}
                          onKeyDown={(event) => handleHotspotRowKeyDown(event, nextFilter)}
                        >
                          <td className="px-3 py-2">
                            {pluginLabel(row.plugin_id, row.plugin_name)}
                            {row.plugin_name ? (
                              <span className="ml-1 text-xs text-muted-foreground">
                                #{row.plugin_id}
                              </span>
                            ) : null}
                          </td>
                          <td className="px-3 py-2">
                            {renderDetailCell(
                              row.signature,
                              `${t.admin.pluginObservabilitySignature} · ${pluginLabel(
                                row.plugin_id,
                                row.plugin_name
                              )}`,
                              'max-w-[240px]'
                            )}
                          </td>
                          <td className="px-3 py-2">{row.failure_count}</td>
                          <td className="px-3 py-2">{toDateTime(row.last_failure_at, locale)}</td>
                          <td className="px-3 py-2">
                            {renderDetailCell(
                              row.sample_error,
                              `${t.admin.pluginObservabilitySampleError} · ${pluginLabel(
                                row.plugin_id,
                                row.plugin_name
                              )}`
                            )}
                          </td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      <div className="grid gap-4 xl:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>{t.admin.pluginObservabilityByRuntime}</CardTitle>
          </CardHeader>
          <CardContent>
            {runtimeChartData.length === 0 ? (
              <p className="text-sm text-muted-foreground">{t.admin.pluginObservabilityNoData}</p>
            ) : (
              <ResponsiveContainer width="100%" height={320}>
                <BarChart data={runtimeChartData} layout="vertical" margin={{ left: 8, right: 8 }}>
                  <CartesianGrid strokeDasharray="3 3" stroke={chart.gridColor} />
                  <XAxis
                    type="number"
                    tick={{ fontSize: 12, fill: chart.tickColor }}
                    stroke={chart.gridColor}
                  />
                  <YAxis
                    dataKey="name"
                    type="category"
                    width={96}
                    tick={{ fontSize: 12, fill: chart.tickColor }}
                    stroke={chart.gridColor}
                    tickFormatter={(value) => compactAxisLabel(String(value))}
                  />
                  <Tooltip {...tooltipStyle} />
                  <Bar dataKey="total" fill="#2563eb" name={t.admin.pluginObservabilityTotal} />
                  <Bar dataKey="failed" fill="#dc2626" name={t.admin.pluginObservabilityFailed} />
                  <Bar
                    dataKey="timeout"
                    fill="#f59e0b"
                    name={t.admin.pluginObservabilityTimeout}
                  />
                </BarChart>
              </ResponsiveContainer>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t.admin.pluginObservabilityByAction}</CardTitle>
          </CardHeader>
          <CardContent>
            {actionChartData.length === 0 ? (
              <p className="text-sm text-muted-foreground">{t.admin.pluginObservabilityNoData}</p>
            ) : (
              <ResponsiveContainer width="100%" height={320}>
                <BarChart data={actionChartData} layout="vertical" margin={{ left: 8, right: 8 }}>
                  <CartesianGrid strokeDasharray="3 3" stroke={chart.gridColor} />
                  <XAxis
                    type="number"
                    tick={{ fontSize: 12, fill: chart.tickColor }}
                    stroke={chart.gridColor}
                  />
                  <YAxis
                    dataKey="name"
                    type="category"
                    width={128}
                    tick={{ fontSize: 12, fill: chart.tickColor }}
                    stroke={chart.gridColor}
                    tickFormatter={(value) => compactAxisLabel(String(value), 22)}
                  />
                  <Tooltip {...tooltipStyle} />
                  <Bar dataKey="total" fill="#0891b2" name={t.admin.pluginObservabilityTotal} />
                  <Bar dataKey="failed" fill="#dc2626" name={t.admin.pluginObservabilityFailed} />
                </BarChart>
              </ResponsiveContainer>
            )}
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{t.admin.pluginObservabilityByPlugin}</CardTitle>
        </CardHeader>
        <CardContent>
          {pluginRows.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t.admin.pluginObservabilityNoData}</p>
          ) : (
            <div className="overflow-auto rounded-md border">
              <table className="w-full text-sm">
                <thead className="bg-muted/40">
                  <tr className="text-left">
                    <th className="px-3 py-2">{t.admin.pluginObservabilityPlugin}</th>
                    <th className="px-3 py-2">{t.admin.pluginObservabilityRuntime}</th>
                    <th className="px-3 py-2">{t.admin.pluginObservabilityTotal}</th>
                    <th className="px-3 py-2">{t.admin.pluginObservabilityFailed}</th>
                    <th className="px-3 py-2">{t.admin.pluginObservabilityErrorRate}</th>
                    <th className="px-3 py-2">{t.admin.pluginObservabilityTimeoutRate}</th>
                    <th className="px-3 py-2">{t.admin.pluginObservabilityAvgMs}</th>
                    <th className="px-3 py-2">{t.admin.pluginObservabilityMaxMs}</th>
                  </tr>
                </thead>
                <tbody>
                  {pluginRows.map((row) => (
                    <tr key={`${row.plugin_id}-${row.runtime}`} className="border-t">
                      <td className="px-3 py-2">
                        {row.plugin_name || `#${row.plugin_id}`}
                        {row.plugin_name ? (
                          <span className="ml-1 text-xs text-muted-foreground">
                            #{row.plugin_id}
                          </span>
                        ) : null}
                      </td>
                      <td className="px-3 py-2">{row.runtime || '-'}</td>
                      <td className="px-3 py-2">{row.counters.total}</td>
                      <td className="px-3 py-2">{row.counters.failed}</td>
                      <td className="px-3 py-2">{toPercent(row.counters.error_rate)}</td>
                      <td className="px-3 py-2">{toPercent(row.counters.timeout_rate)}</td>
                      <td className="px-3 py-2">{row.counters.avg_duration_ms.toFixed(2)}</td>
                      <td className="px-3 py-2">{row.counters.max_duration_ms.toFixed(2)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>

      <div className="grid gap-4 xl:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>{t.admin.pluginObservabilityHookLimiter}</CardTitle>
          </CardHeader>
          <CardContent>
            {hookRows.length === 0 ? (
              <p className="text-sm text-muted-foreground">{t.admin.pluginObservabilityNoData}</p>
            ) : (
              <div className="overflow-auto rounded-md border">
                <table className="w-full text-sm">
                  <thead className="bg-muted/40">
                    <tr className="text-left">
                      <th className="px-3 py-2">{t.admin.pluginObservabilityHook}</th>
                      <th className="px-3 py-2">{t.admin.pluginObservabilityLimiterHits}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {hookRows.map((row) => (
                      <tr key={row.hook} className="border-t">
                        <td className="px-3 py-2">{row.hook}</td>
                        <td className="px-3 py-2">{row.hits}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t.admin.pluginObservabilityPublicEndpoints}</CardTitle>
          </CardHeader>
          <CardContent>
            {endpointRows.length === 0 ? (
              <p className="text-sm text-muted-foreground">{t.admin.pluginObservabilityNoData}</p>
            ) : (
              <div className="overflow-auto rounded-md border">
                <table className="w-full text-sm">
                  <thead className="bg-muted/40">
                    <tr className="text-left">
                      <th className="px-3 py-2">{t.admin.pluginObservabilityEndpoint}</th>
                      <th className="px-3 py-2">{t.admin.pluginObservabilityRequests}</th>
                      <th className="px-3 py-2">{t.admin.pluginObservabilityRateLimited}</th>
                      <th className="px-3 py-2">{t.admin.pluginObservabilityRateLimitHitRate}</th>
                      <th className="px-3 py-2">{t.admin.pluginObservabilityCacheHits}</th>
                      <th className="px-3 py-2">{t.admin.pluginObservabilityCacheMisses}</th>
                      <th className="px-3 py-2">{t.admin.pluginObservabilityCacheHitRate}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {endpointRows.map((row) => (
                      <tr key={row.endpoint} className="border-t">
                        <td className="px-3 py-2">{row.endpoint}</td>
                        <td className="px-3 py-2">{row.requests}</td>
                        <td className="px-3 py-2">{row.rate_limited}</td>
                        <td className="px-3 py-2">{toPercent(row.rate_limit_hit_rate)}</td>
                        <td className="px-3 py-2">{row.cache_hits}</td>
                        <td className="px-3 py-2">{row.cache_misses}</td>
                        <td className="px-3 py-2">{toPercent(row.cache_hit_rate)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      <Dialog open={!!detailDialog} onOpenChange={(open) => (!open ? setDetailDialog(null) : null)}>
        <DialogContent className="max-h-[85vh] max-w-3xl overflow-y-auto">
          <DialogHeader>
            <DialogTitle>{detailDialog?.title || t.common.detail}</DialogTitle>
            <DialogDescription>{t.pageTitle.adminPluginObservability}</DialogDescription>
          </DialogHeader>
          <div className="flex justify-end">
            <Button type="button" variant="outline" size="sm" onClick={() => void copyDetailDialogContent()}>
              <Copy className="mr-2 h-4 w-4" />
              {t.common.copy}
            </Button>
          </div>
          <pre className="overflow-x-auto rounded-md border bg-muted/30 p-3 text-xs whitespace-pre-wrap break-all">
            {detailDialog?.content || '-'}
          </pre>
        </DialogContent>
      </Dialog>
    </div>
  )
}

export default function AdminPluginObservabilityPage() {
  return (
    <Suspense fallback={<div className="min-h-[40vh]" />}>
      <AdminPluginObservabilityPageContent />
    </Suspense>
  )
}
