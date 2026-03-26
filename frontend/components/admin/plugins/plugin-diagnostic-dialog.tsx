'use client'

import type { ReactNode } from 'react'

import { Loader2 } from 'lucide-react'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import type {
  AdminPlugin,
  AdminPluginDeployment,
  AdminPluginDiagnostics,
  AdminPluginExecutionFailureGroup,
  AdminPluginExecutionFailureSample,
  AdminPluginExecutionTaskSnapshot,
  AdminPluginStorageActionProfileDiagnostic,
} from '@/lib/api'
import type { Translations } from '@/lib/i18n'

type PluginDiagnosticDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  plugin: AdminPlugin | null
  diagnosticsLoading: boolean
  diagnostics: AdminPluginDiagnostics | null
  locale: string
  cancelingTaskID?: string | null
  onCancelTask?: (taskID: string) => void
  t: Translations
}

type PriorityFinding = {
  id: string
  severity: 'error' | 'warn' | 'info'
  title: string
  detail?: string
  sectionId?: string
}

type DiagnosticNavTone = 'default' | 'attention' | 'error' | 'active'

function diagnosticNavToneClass(tone: DiagnosticNavTone): string {
  switch (tone) {
    case 'error':
      return 'border-destructive/30 bg-destructive/5 text-destructive hover:bg-destructive/10'
    case 'attention':
      return 'border-input bg-secondary/60 text-secondary-foreground hover:bg-secondary/80'
    case 'active':
      return 'border-primary/30 bg-primary/5 text-primary hover:bg-primary/10'
    default:
      return 'border-input/70 bg-background text-foreground hover:bg-muted/40'
  }
}

function scopeLabel(scope: string, t: Translations): string {
  switch (scope) {
    case 'authenticated':
      return t.admin.pluginFrontendMinScopeAuthenticated
    case 'super_admin':
      return t.admin.pluginFrontendMinScopeSuperAdmin
    default:
      return t.admin.pluginFrontendMinScopeGuest
  }
}

function areaLabel(area: string, t: Translations): string {
  return area === 'admin' ? t.admin.pluginFrontendAreaAdmin : t.admin.pluginFrontendAreaUser
}

function joinSummaryItems(items: Array<string | null | undefined | false>): string {
  return items
    .filter((item): item is string => typeof item === 'string' && item.trim() !== '')
    .join(' · ')
}

function formatDateTimeValue(value?: string, locale?: string): string {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString(locale)
}

function stateVariant(
  state: string
): 'default' | 'secondary' | 'destructive' | 'outline' | 'active' {
  switch (state) {
    case 'ok':
      return 'default'
    case 'error':
      return 'destructive'
    case 'warn':
    case 'disabled':
    case 'restricted':
      return 'secondary'
    default:
      return 'outline'
  }
}

function stateLabel(state: string, t: Translations): string {
  switch (state) {
    case 'ok':
      return t.common.success
    case 'error':
      return t.common.error
    case 'warn':
      return t.common.warning
    case 'disabled':
      return t.admin.pluginDiagnosticsDisabled
    case 'restricted':
      return t.admin.pluginDiagnosticsRestricted
    default:
      return state || '-'
  }
}

function registrationStateLabel(state: string | undefined, t: Translations): string {
  switch (state) {
    case 'success':
      return t.common.success
    case 'error':
      return t.common.error
    case 'never_attempted':
      return t.admin.pluginDiagnosticsRegistrationStateNeverAttempted
    case 'unavailable':
      return t.admin.pluginDiagnosticsRegistrationStateUnavailable
    default:
      return state || '-'
  }
}

function registrationStateVariant(
  state: string | undefined
): 'default' | 'secondary' | 'destructive' | 'outline' | 'active' {
  switch (state) {
    case 'success':
      return 'default'
    case 'error':
      return 'destructive'
    case 'never_attempted':
    case 'unavailable':
      return 'secondary'
    default:
      return 'outline'
  }
}

function registrationTriggerLabel(trigger: string | undefined, t: Translations): string {
  switch (trigger) {
    case 'startup_load':
      return t.admin.pluginDiagnosticsRegistrationTriggerStartupLoad
    case 'reload':
      return t.admin.pluginDiagnosticsRegistrationTriggerReload
    case 'start':
      return t.admin.pluginDiagnosticsRegistrationTriggerStart
    case 'execute_auto_reload':
      return t.admin.pluginDiagnosticsRegistrationTriggerExecuteAutoReload
    case 'healthcheck_auto_reload':
      return t.admin.pluginDiagnosticsRegistrationTriggerHealthcheckAutoReload
    default:
      return trigger ? trigger.replace(/_/g, ' ') : '-'
  }
}

function compatibilityVariant(
  compatible: boolean | undefined,
  legacyDefaultsApplied: boolean | undefined
): 'default' | 'secondary' | 'destructive' | 'outline' | 'active' {
  if (!compatible) return 'destructive'
  if (legacyDefaultsApplied) return 'secondary'
  return 'default'
}

function compatibilityLabel(
  compatible: boolean | undefined,
  legacyDefaultsApplied: boolean | undefined,
  t: Translations
): string {
  if (!compatible) return t.admin.pluginDiagnosticsCompatibilityStateIncompatible
  if (legacyDefaultsApplied) return t.admin.pluginDiagnosticsCompatibilityStateLegacy
  return t.admin.pluginDiagnosticsCompatibilityStateCompatible
}

function connectionLabel(state: string | undefined, t: Translations): string {
  switch (state) {
    case 'connected':
      return t.admin.pluginDiagnosticsConnectionConnected
    case 'disconnected':
      return t.admin.pluginDiagnosticsConnectionDisconnected
    case 'stateless':
      return t.admin.pluginDiagnosticsConnectionStateless
    case 'unsupported':
      return t.admin.pluginDiagnosticsConnectionUnsupported
    case 'unavailable':
      return t.admin.pluginDiagnosticsConnectionUnavailable
    default:
      return t.admin.pluginDiagnosticsConnectionUnknown
  }
}

function lifecycleLabel(state: string | undefined, t: Translations): string {
  switch (state) {
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
      return state || '-'
  }
}

function healthLabel(state: string | undefined, t: Translations): string {
  switch (state) {
    case 'healthy':
      return t.admin.pluginStatusHealthy
    case 'unhealthy':
      return t.admin.pluginStatusUnhealthy
    case 'unknown':
      return t.admin.pluginStatusUnknown
    default:
      return state || '-'
  }
}

function breakerStateLabel(state: string | undefined, t: Translations): string {
  switch (state) {
    case 'open':
      return t.admin.pluginDiagnosticsBreakerStateOpen
    case 'half_open':
      return t.admin.pluginDiagnosticsBreakerStateHalfOpen
    case 'closed':
    default:
      return t.admin.pluginDiagnosticsBreakerStateClosed
  }
}

function severityVariant(
  severity: string
): 'default' | 'secondary' | 'destructive' | 'outline' | 'active' {
  switch (severity) {
    case 'error':
      return 'destructive'
    case 'warn':
      return 'secondary'
    default:
      return 'outline'
  }
}

function severityLabel(severity: string, t: Translations): string {
  switch (severity) {
    case 'error':
      return t.common.error
    case 'warn':
      return t.common.warning
    case 'info':
      return t.common.info
    default:
      return severity || '-'
  }
}

function taskStatusVariant(
  status: string | undefined
): 'default' | 'secondary' | 'destructive' | 'outline' | 'active' {
  switch (status) {
    case 'running':
      return 'active'
    case 'completed':
      return 'default'
    case 'failed':
    case 'timed_out':
      return 'destructive'
    case 'canceled':
      return 'secondary'
    default:
      return 'outline'
  }
}

function taskStatusLabel(status: string | undefined, t: Translations): string {
  switch (status) {
    case 'running':
      return t.admin.pluginDiagnosticsTaskStatusRunning
    case 'completed':
      return t.admin.pluginDiagnosticsTaskStatusCompleted
    case 'failed':
      return t.admin.pluginDiagnosticsTaskStatusFailed
    case 'canceled':
      return t.admin.pluginDiagnosticsTaskStatusCanceled
    case 'timed_out':
      return t.admin.pluginDiagnosticsTaskStatusTimedOut
    default:
      return status || '-'
  }
}

function storageAccessVariant(
  mode: string | undefined
): 'default' | 'secondary' | 'destructive' | 'outline' | 'active' {
  switch (mode) {
    case 'write':
      return 'default'
    case 'read':
      return 'secondary'
    case 'none':
      return 'outline'
    case 'unknown':
    default:
      return 'secondary'
  }
}

function storageAccessLabel(mode: string | undefined, t: Translations): string {
  switch (mode) {
    case 'write':
      return t.admin.pluginDiagnosticsStorageAccessWrite
    case 'read':
      return t.admin.pluginDiagnosticsStorageAccessRead
    case 'none':
      return t.admin.pluginDiagnosticsStorageAccessNone
    case 'unknown':
      return t.admin.pluginDiagnosticsStorageAccessUnknown
    default:
      return mode || '-'
  }
}

function storageObservationSourceLabel(source: string | undefined, t: Translations): string {
  switch (source) {
    case 'plugin_executions.latest':
      return t.admin.pluginDiagnosticsStorageSourcePersisted
    case 'execution_tasks.recent':
      return t.admin.pluginDiagnosticsStorageSourceRecentTasks
    default:
      return source || '-'
  }
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

function formatGenerationList(value: number[] | undefined): string {
  if (!Array.isArray(value) || value.length === 0) return '-'
  return (
    value
      .map((item) => Number(item || 0))
      .filter((item) => Number.isFinite(item) && item > 0)
      .join(', ') || '-'
  )
}

function formatDuration(durationMs?: number, startedAt?: string): string {
  let resolved = typeof durationMs === 'number' && durationMs >= 0 ? durationMs : NaN
  if (!Number.isFinite(resolved) && startedAt) {
    const started = new Date(startedAt)
    if (!Number.isNaN(started.getTime())) {
      resolved = Date.now() - started.getTime()
    }
  }
  if (!Number.isFinite(resolved) || resolved < 0) {
    return '-'
  }
  if (resolved < 1000) {
    return `${Math.round(resolved)} ms`
  }
  if (resolved < 60_000) {
    return `${(resolved / 1000).toFixed(resolved >= 10_000 ? 0 : 1)} s`
  }
  const minutes = Math.floor(resolved / 60_000)
  const seconds = Math.round((resolved % 60_000) / 1000)
  return `${minutes}m ${seconds}s`
}

function renderTaskMeta(label: string, value?: string | number | null, mono?: boolean) {
  if (value === undefined || value === null || value === '') return null
  return (
    <p className="min-w-0">
      {label}:{' '}
      <span className={mono ? 'break-all font-mono text-foreground' : 'break-words text-foreground'}>
        {String(value)}
      </span>
    </p>
  )
}

function renderTaskList(
  tasks: AdminPluginExecutionTaskSnapshot[],
  emptyText: string,
  cancelingTaskID: string | null | undefined,
  onCancelTask: ((taskID: string) => void) | undefined,
  formatDateTime: (value?: string) => string,
  t: Translations
) {
  if (tasks.length === 0) {
    return <p className="text-sm text-muted-foreground">{emptyText}</p>
  }

  return tasks.map((task) => {
    const canceling = cancelingTaskID === task.id
    const observedStorageAccess = task.metadata?.storage_access_mode
    return (
      <div key={task.id} className="rounded-md border border-input/60 p-3 text-sm">
        <div className="flex flex-wrap items-start justify-between gap-2">
          <div className="min-w-0 space-y-2">
            <div className="flex flex-wrap items-center gap-2">
              <p className="font-medium">{task.action || '-'}</p>
              <Badge variant={taskStatusVariant(task.status)}>
                {taskStatusLabel(task.status, t)}
              </Badge>
              {task.stream ? (
                <Badge variant="outline">{t.admin.pluginDiagnosticsTaskStream}</Badge>
              ) : null}
              {task.runtime ? <Badge variant="secondary">{task.runtime}</Badge> : null}
              {observedStorageAccess ? (
                <Badge variant={storageAccessVariant(observedStorageAccess)}>
                  {t.admin.pluginDiagnosticsStorageObservedAccess}:{' '}
                  {storageAccessLabel(observedStorageAccess, t)}
                </Badge>
              ) : null}
            </div>
            {task.hook ? (
              <p className="break-all text-xs text-muted-foreground">
                {t.admin.pluginDiagnosticsTaskHook}:{' '}
                <span className="font-mono text-foreground">{task.hook}</span>
              </p>
            ) : null}
          </div>

          {task.cancelable && onCancelTask ? (
            <Button
              size="sm"
              variant="outline"
              onClick={() => onCancelTask(task.id)}
              disabled={canceling}
            >
              {canceling ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  {t.common.processing}
                </>
              ) : (
                t.common.cancel
              )}
            </Button>
          ) : null}
        </div>

        <div className="mt-3 grid gap-2 text-xs text-muted-foreground md:grid-cols-2 xl:grid-cols-3">
          {renderTaskMeta(t.admin.pluginDiagnosticsTaskId, task.id, true)}
          {renderTaskMeta(t.admin.pluginDiagnosticsTaskStartedAt, formatDateTime(task.started_at))}
          {renderTaskMeta(t.admin.pluginDiagnosticsTaskUpdatedAt, formatDateTime(task.updated_at))}
          {renderTaskMeta(
            t.admin.pluginDiagnosticsTaskCompletedAt,
            formatDateTime(task.completed_at)
          )}
          {renderTaskMeta(
            t.admin.pluginDiagnosticsTaskDuration,
            formatDuration(task.duration_ms, task.started_at)
          )}
          {renderTaskMeta(t.admin.pluginDiagnosticsTaskChunks, task.chunk_count ?? 0)}
          {renderTaskMeta(t.admin.pluginDiagnosticsTaskSession, task.session_id, true)}
          {renderTaskMeta(t.admin.pluginDiagnosticsTaskUser, task.user_id)}
          {renderTaskMeta(t.admin.pluginDiagnosticsTaskOrder, task.order_id)}
          {renderTaskMeta(t.admin.pluginDiagnosticsTaskRequestPath, task.request_path, true)}
          {renderTaskMeta(t.admin.pluginDiagnosticsTaskPluginPagePath, task.plugin_page_path, true)}
        </div>

        {task.error ? (
          <div className="mt-3 rounded-md border border-destructive/20 bg-destructive/5 p-3">
            <p className="text-xs text-muted-foreground">{t.admin.pluginDiagnosticsTaskError}</p>
            <p className="mt-1 break-all text-xs font-medium">{task.error}</p>
          </div>
        ) : null}
      </div>
    )
  })
}

function renderStorageProfileList(
  profiles: AdminPluginStorageActionProfileDiagnostic[],
  latestAction: string | undefined,
  emptyText: string,
  latestLabel: string,
  t: Translations
) {
  if (profiles.length === 0) {
    return <p className="text-sm text-muted-foreground">{emptyText}</p>
  }

  return profiles.map((profile) => {
    const isLatest = latestAction && profile.action === latestAction
    return (
      <div
        key={`${profile.action}-${profile.mode}`}
        className="flex flex-wrap items-center gap-2 rounded-md border border-input/60 p-3 text-sm"
      >
        <p className="font-mono text-xs text-foreground">{profile.action}</p>
        <Badge variant={storageAccessVariant(profile.mode)}>
          {storageAccessLabel(profile.mode, t)}
        </Badge>
        {isLatest ? <Badge variant="outline">{latestLabel}</Badge> : null}
      </div>
    )
  })
}

function renderFailureGroupList(
  groups: AdminPluginExecutionFailureGroup[],
  emptyText: string,
  formatDateTime: (value?: string) => string,
  t: Translations
) {
  if (groups.length === 0) {
    return <p className="text-sm text-muted-foreground">{emptyText}</p>
  }

  return groups.map((group, index) => (
    <div
      key={`${group.action}-${group.hook || 'none'}-${index}`}
      className="rounded-md border border-input/60 p-3 text-sm"
    >
      <div className="flex flex-wrap items-center gap-2">
        <p className="font-medium">{group.action || '-'}</p>
        {group.hook ? <Badge variant="secondary">{group.hook}</Badge> : null}
        <Badge variant="outline">{group.failure_count}</Badge>
      </div>
      <p className="mt-2 text-xs text-muted-foreground">
        {t.admin.pluginDiagnosticsLastFailureAt}: {formatDateTime(group.last_failure_at)}
      </p>
    </div>
  ))
}

function renderFailureSampleList(
  samples: AdminPluginExecutionFailureSample[],
  emptyText: string,
  formatDateTime: (value?: string) => string,
  t: Translations
) {
  if (samples.length === 0) {
    return <p className="text-sm text-muted-foreground">{emptyText}</p>
  }

  return samples.map((sample) => (
    <div key={sample.id} className="rounded-md border border-input/60 p-3 text-sm">
      <div className="flex flex-wrap items-center gap-2">
        <p className="font-medium">{sample.action || '-'}</p>
        {sample.hook ? <Badge variant="secondary">{sample.hook}</Badge> : null}
        <Badge variant="outline">{formatDuration(sample.duration)}</Badge>
      </div>
      <p className="mt-2 text-xs text-muted-foreground">{formatDateTime(sample.created_at)}</p>
      {sample.error ? (
        <p className="mt-2 break-all text-xs text-muted-foreground">{sample.error}</p>
      ) : null}
    </div>
  ))
}

function renderExpandableList<T>({
  items,
  initialCount,
  emptyText,
  summaryLabel,
  renderItems,
  className = 'space-y-3',
}: {
  items: T[]
  initialCount: number
  emptyText: string
  summaryLabel: string
  renderItems: (items: T[]) => ReactNode
  className?: string
}) {
  if (items.length === 0) {
    return <p className="text-sm text-muted-foreground">{emptyText}</p>
  }

  const visibleItems = items.slice(0, initialCount)
  const remainingItems = items.slice(initialCount)

  return (
    <div className={className}>
      {renderItems(visibleItems)}
      {remainingItems.length > 0 ? (
        <details className="rounded-md border border-input/60 bg-muted/10 p-3">
          <summary className="cursor-pointer text-sm font-medium">
            {summaryLabel} ({remainingItems.length})
          </summary>
          <div className={`mt-3 ${className}`}>{renderItems(remainingItems)}</div>
        </details>
      ) : null}
    </div>
  )
}

function DiagnosticGroupCard({
  title,
  summary,
  children,
  className = '',
}: {
  title: string
  summary?: ReactNode
  children: ReactNode
  className?: string
}) {
  return (
    <div className={`rounded-md border border-input/60 bg-muted/10 p-3 ${className}`.trim()}>
      <div className="space-y-1">
        <p className="text-sm font-medium">{title}</p>
        {summary ? <div className="text-xs text-muted-foreground">{summary}</div> : null}
      </div>
      <div className="mt-3">{children}</div>
    </div>
  )
}

function DiagnosticValueItem({
  label,
  value,
  mono = false,
}: {
  label: string
  value: ReactNode
  mono?: boolean
}) {
  return (
    <div className="min-w-0 space-y-1">
      <p className="text-xs text-muted-foreground">{label}</p>
      <div className={mono ? 'break-all font-mono text-sm font-medium' : 'break-all text-sm font-medium'}>
        {value}
      </div>
    </div>
  )
}

export function PluginDiagnosticDialog({
  open,
  onOpenChange,
  plugin,
  diagnosticsLoading,
  diagnostics,
  locale,
  cancelingTaskID,
  onCancelTask,
  t,
}: PluginDiagnosticDialogProps) {
  const formatDateTime = (value?: string) => formatDateTimeValue(value, locale)
  const runtime = diagnostics?.runtime
  const diagnosticPlugin = diagnostics?.plugin
  const compatibility = diagnostics?.compatibility
  const registration = diagnostics?.registration
  const recentDeployments = Array.isArray(diagnostics?.recent_deployments)
    ? (diagnostics.recent_deployments as AdminPluginDeployment[])
    : []
  const executionTasks = diagnostics?.execution_tasks
  const executionObservability = diagnostics?.execution_observability
  const storageDiagnostics = diagnostics?.storage_diagnostics
  const publicCache = diagnostics?.public_cache
  const checks = Array.isArray(diagnostics?.checks) ? diagnostics.checks : []
  const issues = Array.isArray(diagnostics?.issues) ? diagnostics.issues : []
  const missingPermissions = Array.isArray(diagnostics?.missing_permissions)
    ? diagnostics.missing_permissions
    : []
  const routes = Array.isArray(diagnostics?.frontend_routes) ? diagnostics.frontend_routes : []
  const activeTasks = Array.isArray(executionTasks?.active) ? executionTasks.active : []
  const recentTasks = Array.isArray(executionTasks?.recent) ? executionTasks.recent : []
  const failureGroups = Array.isArray(executionObservability?.failure_groups)
    ? executionObservability.failure_groups
    : []
  const recentFailures = Array.isArray(executionObservability?.recent_failures)
    ? executionObservability.recent_failures
    : []
  const declaredStorageProfiles = Array.isArray(storageDiagnostics?.declared_profiles)
    ? storageDiagnostics.declared_profiles
    : []
  const lastObservedStorage = storageDiagnostics?.last_observed
  const extensionCache = publicCache?.extensions
  const bootstrapCache = publicCache?.bootstrap
  const issueErrorCount = issues.filter((issue) => issue.severity === 'error').length
  const issueWarnCount = issues.filter((issue) => issue.severity === 'warn').length
  const issueInfoCount = issues.filter((issue) => issue.severity === 'info').length
  const failedCheckCount = checks.filter((check) => check.state === 'error').length
  const warningCheckCount = checks.filter(
    (check) => check.state === 'warn' || check.state === 'restricted'
  ).length
  const checkOkCount = checks.filter((check) => check.state === 'ok').length
  const executeApiAvailableCount = routes.filter((route) => route.execute_api_available).length
  const unavailableRouteCount = routes.filter((route) => !route.execute_api_available).length
  const unavailableRoutes = routes.filter((route) => !route.execute_api_available)
  const routeScopeStats = routes.reduce(
    (acc, route) => {
      const scopeChecks = Array.isArray(route.scope_checks) ? route.scope_checks : []
      acc.visible += scopeChecks.filter((scopeCheck) => scopeCheck.frontend_visible).length
      acc.eligible += scopeChecks.filter(
        (scopeCheck) => scopeCheck.eligible && !scopeCheck.frontend_visible
      ).length
      acc.unavailable += scopeChecks.filter((scopeCheck) => !scopeCheck.eligible).length
      return acc
    },
    { visible: 0, eligible: 0, unavailable: 0 }
  )
  const cacheTotalEntryCount =
    (extensionCache?.total_entries ?? 0) + (bootstrapCache?.total_entries ?? 0)
  const cacheMatchingEntryCount =
    (extensionCache?.matching_entries ?? 0) + (bootstrapCache?.matching_entries ?? 0)
  const summaryIsClear =
    issueErrorCount === 0 &&
    issueWarnCount === 0 &&
    failedCheckCount === 0 &&
    warningCheckCount === 0 &&
    missingPermissions.length === 0 &&
    unavailableRouteCount === 0 &&
    (runtime?.health_status || 'unknown') === 'healthy'
  const priorityFindings: PriorityFinding[] =
    issues.length > 0
        ? issues.slice(0, 3).map((issue, index) => ({
            id: `${issue.code}-${index}-summary`,
            severity:
              issue.severity === 'error' || issue.severity === 'warn' ? issue.severity : 'info',
            title: issue.summary,
            detail: issue.hint || issue.detail,
            sectionId: 'plugin-diagnostics-issues',
          }))
        : []

  if (priorityFindings.length === 0 && !summaryIsClear) {
    const runtimeHealth = runtime?.health_status || 'unknown'
    if (runtimeHealth !== 'healthy') {
        priorityFindings.push({
          id: 'runtime-health',
          severity: runtimeHealth === 'unhealthy' ? 'error' : 'warn',
          title: `${t.admin.pluginDiagnosticsHealth}: ${healthLabel(runtime?.health_status, t)}`,
          detail: `${t.admin.pluginDiagnosticsConnection}: ${connectionLabel(runtime?.connection_state, t)}`,
          sectionId: 'plugin-diagnostics-runtime',
        })
      }

    checks
      .filter((check) => check.state === 'error')
      .slice(0, 2)
      .forEach((check) => {
        priorityFindings.push({
          id: `check-error-${check.key}`,
          severity: 'error',
          title: check.summary,
          detail: check.detail || check.key,
          sectionId: 'plugin-diagnostics-checks',
        })
      })

    checks
      .filter((check) => check.state === 'warn' || check.state === 'restricted')
      .slice(0, 1)
      .forEach((check) => {
        priorityFindings.push({
          id: `check-warn-${check.key}`,
          severity: 'warn',
          title: check.summary,
          detail: check.detail || check.key,
          sectionId: 'plugin-diagnostics-checks',
        })
      })

    if (missingPermissions.length > 0) {
      priorityFindings.push({
        id: 'missing-permissions',
        severity: 'warn',
        title: t.admin.pluginDiagnosticsMissingPermissions,
        detail: missingPermissions.slice(0, 6).join(', '),
        sectionId: 'plugin-diagnostics-permissions',
      })
    }

    if (unavailableRouteCount > 0) {
      priorityFindings.push({
        id: 'route-issues',
        severity: 'warn',
        title: t.admin.pluginDiagnosticsSummaryRouteIssues.replace(
          '{count}',
          String(unavailableRouteCount)
        ),
        detail: unavailableRoutes
          .slice(0, 3)
          .map((route) => route.path)
          .join(', '),
        sectionId: 'plugin-diagnostics-routes',
      })
    }
  }

  const quickNavItems: Array<{
    id: string
    label: string
    count?: number
    tone: DiagnosticNavTone
  }> = [
    {
      id: 'plugin-diagnostics-runtime',
      label: t.admin.pluginDiagnosticsRuntime,
      tone:
        runtime?.health_status === 'unhealthy'
          ? 'error'
          : runtime?.health_status === 'healthy'
            ? 'default'
            : 'attention',
    },
    {
      id: 'plugin-diagnostics-compatibility',
      label: t.admin.pluginDiagnosticsCompatibility,
      tone: compatibility?.compatible ? 'default' : 'error',
    },
    {
      id: 'plugin-diagnostics-registration',
      label: t.admin.pluginDiagnosticsRegistration,
      tone: registration?.state === 'error' ? 'error' : 'default',
    },
    {
      id: 'plugin-diagnostics-deployments',
      label: t.admin.pluginDiagnosticsDeployments,
      tone:
        diagnosticPlugin?.latest_deployment?.status === 'failed' ||
        diagnosticPlugin?.latest_deployment?.status === 'rolled_back'
          ? 'error'
          : 'default',
    },
    {
      id: 'plugin-diagnostics-tasks',
      label: t.admin.pluginDiagnosticsExecutionTasks,
      count: activeTasks.length,
      tone: activeTasks.length > 0 ? 'active' : 'default',
    },
    {
      id: 'plugin-diagnostics-storage',
      label: t.admin.pluginDiagnosticsStorageAccess,
      tone:
        lastObservedStorage?.declared_access_mode === 'unknown' &&
        !!lastObservedStorage?.observed_access_mode
          ? 'attention'
          : 'default',
    },
    {
      id: 'plugin-diagnostics-observability',
      label: t.admin.pluginDiagnosticsObservability,
      count: executionObservability?.failed_executions ?? 0,
      tone: (executionObservability?.failed_executions ?? 0) > 0 ? 'attention' : 'default',
    },
    {
      id: 'plugin-diagnostics-public-cache',
      label: t.admin.pluginDiagnosticsPublicCache,
      tone: 'default',
    },
    {
      id: 'plugin-diagnostics-checks',
      label: t.admin.pluginDiagnosticsChecks,
      count: failedCheckCount + warningCheckCount,
      tone:
        failedCheckCount > 0
          ? 'error'
          : warningCheckCount > 0
            ? 'attention'
            : 'default',
    },
    {
      id: 'plugin-diagnostics-permissions',
      label: t.admin.pluginDiagnosticsMissingPermissions,
      count: missingPermissions.length,
      tone: missingPermissions.length > 0 ? 'attention' : 'default',
    },
    {
      id: 'plugin-diagnostics-routes',
      label: t.admin.pluginDiagnosticsFrontendRoutes,
      count: unavailableRouteCount,
      tone: unavailableRouteCount > 0 ? 'attention' : 'default',
    },
    {
      id: 'plugin-diagnostics-issues',
      label: t.admin.pluginDiagnosticsIssues,
      count: issues.length,
      tone: issueErrorCount > 0 ? 'error' : issueWarnCount > 0 ? 'attention' : 'default',
    },
  ]
  const quickNavLabelById = quickNavItems.reduce<Record<string, string>>((acc, item) => {
    acc[item.id] = item.label
    return acc
  }, {})
  const scrollToSection = (id: string) => {
    document.getElementById(id)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] max-w-5xl overflow-y-auto [&_.flex>*]:min-w-0 [&_.grid>*]:min-w-0 [&_p]:break-words [&_span]:break-words">
        <DialogHeader>
          <DialogTitle>{t.admin.pluginDiagnosticsTitle}</DialogTitle>
          <DialogDescription>{plugin?.display_name || plugin?.name}</DialogDescription>
        </DialogHeader>

        {diagnosticsLoading ? (
          <div className="py-8 text-center text-sm text-muted-foreground">
            <Loader2 className="mr-2 inline-block h-4 w-4 animate-spin" />
            {t.common.loading}
          </div>
        ) : !diagnostics ? (
          <div className="py-8 text-center text-sm text-muted-foreground">{t.common.noData}</div>
        ) : (
          <div className="space-y-4">
            <div className="space-y-3 rounded-lg border border-input bg-background/95 p-3">
              <p className="text-xs text-muted-foreground">
                {[
                  `${t.admin.pluginDiagnosticsHealth}: ${healthLabel(runtime?.health_status, t)}`,
                  `${t.admin.pluginDiagnosticsConnection}: ${connectionLabel(
                    runtime?.connection_state,
                    t
                  )}`,
                  `${t.admin.pluginDiagnosticsCompatibilityState}: ${compatibilityLabel(
                    compatibility?.compatible,
                    compatibility?.legacy_defaults_applied,
                    t
                  )}`,
                  summaryIsClear ? t.admin.pluginDiagnosticsSummaryClear : null,
                  issueErrorCount > 0
                    ? t.admin.pluginDiagnosticsSummaryIssueErrors.replace(
                        '{count}',
                        String(issueErrorCount)
                      )
                    : null,
                  failedCheckCount > 0
                    ? t.admin.pluginDiagnosticsSummaryFailedChecks.replace(
                        '{count}',
                        String(failedCheckCount)
                      )
                    : null,
                  missingPermissions.length > 0
                    ? t.admin.pluginDiagnosticsSummaryMissingPermissionCount.replace(
                        '{count}',
                        String(missingPermissions.length)
                      )
                    : null,
                  activeTasks.length > 0
                    ? t.admin.pluginDiagnosticsSummaryActiveTaskCount.replace(
                        '{count}',
                        String(activeTasks.length)
                      )
                    : null,
                ]
                  .filter(Boolean)
                  .join(' · ')}
              </p>
              <div className="flex flex-wrap items-center gap-2 border-t border-input/60 pt-3">
                <p className="mr-1 text-xs font-medium text-muted-foreground">
                  {t.admin.pluginDiagnosticsQuickNav}
                </p>
                {quickNavItems.map((item) => (
                  <button
                    key={item.id}
                    type="button"
                    onClick={() => scrollToSection(item.id)}
                    className={`inline-flex items-center gap-2 rounded-full border px-3 py-1.5 text-xs font-medium transition-colors ${diagnosticNavToneClass(item.tone)}`}
                  >
                    <span>{item.label}</span>
                    {typeof item.count === 'number' && item.count > 0 ? (
                      <span className="rounded-full bg-background/80 px-1.5 py-0.5 text-[11px]">
                        {item.count}
                      </span>
                    ) : null}
                  </button>
                ))}
              </div>
              <div className="border-t border-input/60 pt-3">
                <div className="space-y-3 rounded-md border border-input/60 bg-muted/10 p-3">
                  <p className="text-sm font-medium">{t.admin.pluginDiagnosticsPriorityFindings}</p>
                  {summaryIsClear ? (
                    <p className="text-sm text-muted-foreground">
                      {t.admin.pluginDiagnosticsPriorityFindingsEmpty}
                    </p>
                  ) : (
                    <div className="space-y-2">
                      {priorityFindings.map((finding) => (
                        <div
                          key={finding.id}
                          className="rounded-md border border-input/60 bg-background p-3 text-sm transition-colors hover:bg-muted/20"
                        >
                          <div className="space-y-2">
                            <div className="flex flex-wrap items-center gap-2">
                              <Badge variant={severityVariant(finding.severity)}>
                                {severityLabel(finding.severity, t)}
                              </Badge>
                              {finding.sectionId ? (
                                <button
                                  type="button"
                                  onClick={() => scrollToSection(finding.sectionId!)}
                                  className="inline-flex max-w-full items-center rounded-full border border-input/70 bg-muted/20 px-2.5 py-1 text-xs text-muted-foreground transition-colors hover:bg-muted/60 hover:text-foreground"
                                >
                                  <span className="truncate">
                                    {quickNavLabelById[finding.sectionId] || finding.sectionId}
                                  </span>
                                </button>
                              ) : null}
                            </div>
                            <p className="font-medium">{finding.title}</p>
                          </div>
                          {finding.detail ? (
                            <p className="mt-2 text-xs text-muted-foreground">{finding.detail}</p>
                          ) : null}
                        </div>
                      ))}
                      {priorityFindings.length === 0 && missingPermissions.length > 0 ? (
                        <div className="rounded-md border border-input/60 bg-background p-3 text-sm">
                          <p className="font-medium">{t.admin.pluginDiagnosticsMissingPermissions}</p>
                          <div className="mt-2 flex flex-wrap gap-2">
                            {missingPermissions.slice(0, 6).map((item) => (
                              <Badge key={item} variant="outline">
                                {item}
                              </Badge>
                            ))}
                          </div>
                        </div>
                      ) : null}
                    </div>
                  )}
                </div>
              </div>
            </div>

            <Card id="plugin-diagnostics-runtime" className="scroll-mt-24">
              <CardHeader className="pb-3">
                <CardTitle>{t.admin.pluginDiagnosticsRuntime}</CardTitle>
              </CardHeader>
              <CardContent className="space-y-3 text-sm">
                <DiagnosticGroupCard
                  title={`${t.admin.pluginDiagnosticsConfiguredRuntime} / ${t.admin.pluginDiagnosticsResolvedRuntime}`}
                  summary={joinSummaryItems([
                    `${t.admin.pluginDiagnosticsConnection}: ${connectionLabel(runtime?.connection_state, t)}`,
                    `${t.admin.pluginDiagnosticsLifecycle}: ${lifecycleLabel(runtime?.lifecycle_status, t)}`,
                    `${t.admin.pluginDiagnosticsHealth}: ${healthLabel(runtime?.health_status, t)}`,
                    `${t.admin.pluginDiagnosticsReady}: ${runtime?.ready ? t.common.yes : t.common.no}`,
                  ])}
                >
                  <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
                    <DiagnosticValueItem
                      label={t.admin.pluginDiagnosticsConfiguredRuntime}
                      value={runtime?.configured_runtime || '-'}
                    />
                    <DiagnosticValueItem
                      label={t.admin.pluginDiagnosticsResolvedRuntime}
                      value={runtime?.resolved_runtime || '-'}
                    />
                    <DiagnosticValueItem
                      label={t.admin.pluginDiagnosticsLifecycle}
                      value={lifecycleLabel(runtime?.lifecycle_status, t)}
                    />
                    <DiagnosticValueItem
                      label={t.admin.pluginDiagnosticsHealth}
                      value={healthLabel(runtime?.health_status, t)}
                    />
                  </div>
                </DiagnosticGroupCard>

                <div className="grid gap-3 xl:grid-cols-2">
                  <DiagnosticGroupCard
                    title={`${t.admin.pluginDiagnosticsBreakerState} / ${t.admin.pluginDiagnosticsCooldown}`}
                    summary={joinSummaryItems([
                      `${t.admin.pluginDiagnosticsBreakerState}: ${breakerStateLabel(runtime?.breaker_state, t)}`,
                      runtime?.cooldown_active
                        ? t.admin.pluginDiagnosticsCooldownActive
                        : t.admin.pluginDiagnosticsCooldownInactive,
                      `${t.admin.pluginDiagnosticsProbeInFlight}: ${
                        runtime?.probe_in_flight ? t.common.yes : t.common.no
                      }`,
                    ])}
                  >
                    <div className="grid gap-3 sm:grid-cols-2">
                      <DiagnosticValueItem
                        label={t.admin.pluginDiagnosticsFailureCount}
                        value={runtime?.failure_count ?? 0}
                      />
                      <DiagnosticValueItem
                        label={t.admin.pluginDiagnosticsFailureThreshold}
                        value={runtime?.failure_threshold ?? '-'}
                      />
                      <DiagnosticValueItem
                        label={t.admin.pluginDiagnosticsCooldownUntil}
                        value={formatDateTime(runtime?.cooldown_until)}
                      />
                      <DiagnosticValueItem
                        label={t.admin.pluginDiagnosticsCooldownReason}
                        value={runtime?.cooldown_reason || '-'}
                      />
                    </div>
                  </DiagnosticGroupCard>

                  <DiagnosticGroupCard
                    title={`${t.admin.pluginDiagnosticsActiveGeneration} / ${t.admin.pluginDiagnosticsDrainingGenerations}`}
                    summary={joinSummaryItems([
                      `${t.admin.pluginDiagnosticsActiveGeneration}: ${runtime?.active_generation ?? '-'}`,
                      `${t.admin.pluginDiagnosticsDrainingSlots}: ${runtime?.draining_slot_count ?? 0}`,
                    ])}
                  >
                    <div className="grid gap-3 sm:grid-cols-2">
                      <DiagnosticValueItem
                        label={t.admin.pluginDiagnosticsActiveInFlight}
                        value={runtime?.active_in_flight ?? 0}
                      />
                      <DiagnosticValueItem
                        label={t.admin.pluginDiagnosticsDrainingInFlight}
                        value={runtime?.draining_in_flight ?? 0}
                      />
                      <DiagnosticValueItem
                        label={t.admin.pluginDiagnosticsDrainingSlots}
                        value={runtime?.draining_slot_count ?? 0}
                      />
                      <DiagnosticValueItem
                        label={t.admin.pluginDiagnosticsDrainingGenerations}
                        value={formatGenerationList(runtime?.draining_generations)}
                      />
                    </div>
                  </DiagnosticGroupCard>
                </div>

                <DiagnosticGroupCard title={t.admin.pluginDiagnosticsLastError}>
                  <DiagnosticValueItem
                    label={t.admin.pluginDiagnosticsLastError}
                    value={runtime?.last_error || '-'}
                  />
                </DiagnosticGroupCard>
              </CardContent>
            </Card>

            <Card id="plugin-diagnostics-compatibility" className="scroll-mt-24">
              <CardHeader className="pb-3">
                <CardTitle>{t.admin.pluginDiagnosticsCompatibility}</CardTitle>
              </CardHeader>
              <CardContent className="space-y-3 text-sm">
                <DiagnosticGroupCard
                  title={t.admin.pluginDiagnosticsCompatibilityState}
                  summary={joinSummaryItems([
                    compatibilityLabel(
                      compatibility?.compatible,
                      compatibility?.legacy_defaults_applied,
                      t
                    ),
                    `${t.admin.pluginDiagnosticsManifestPresent}: ${
                      compatibility?.manifest_present ? t.common.yes : t.common.no
                    }`,
                    `${t.admin.pluginDiagnosticsCompatibilityLegacyDefaults}: ${
                      compatibility?.legacy_defaults_applied ? t.common.yes : t.common.no
                    }`,
                  ])}
                >
                  <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
                    <DiagnosticValueItem
                      label={t.admin.pluginDiagnosticsCompatibilityState}
                      value={compatibilityLabel(
                        compatibility?.compatible,
                        compatibility?.legacy_defaults_applied,
                        t
                      )}
                    />
                    <DiagnosticValueItem
                      label={t.admin.pluginDiagnosticsManifestPresent}
                      value={compatibility?.manifest_present ? t.common.yes : t.common.no}
                    />
                    <DiagnosticValueItem
                      label={t.admin.pluginDiagnosticsCompatibilityLegacyDefaults}
                      value={compatibility?.legacy_defaults_applied ? t.common.yes : t.common.no}
                    />
                  </div>
                </DiagnosticGroupCard>

                <div className="grid gap-3 xl:grid-cols-2">
                  <DiagnosticGroupCard
                    title={`${t.admin.pluginDiagnosticsHostManifestVersion} / ${t.admin.pluginDiagnosticsManifestVersion}`}
                  >
                    <div className="grid gap-3 sm:grid-cols-2">
                      <DiagnosticValueItem
                        label={t.admin.pluginDiagnosticsHostManifestVersion}
                        value={compatibility?.host_manifest_version || '-'}
                      />
                      <DiagnosticValueItem
                        label={t.admin.pluginDiagnosticsManifestVersion}
                        value={compatibility?.manifest_version || '-'}
                      />
                    </div>
                  </DiagnosticGroupCard>

                  <DiagnosticGroupCard
                    title={`${t.admin.pluginDiagnosticsHostProtocolVersion} / ${t.admin.pluginDiagnosticsProtocolVersion}`}
                  >
                    <div className="grid gap-3 sm:grid-cols-2">
                      <DiagnosticValueItem
                        label={t.admin.pluginDiagnosticsHostProtocolVersion}
                        value={compatibility?.host_protocol_version || '-'}
                      />
                      <DiagnosticValueItem
                        label={t.admin.pluginDiagnosticsProtocolVersion}
                        value={compatibility?.protocol_version || '-'}
                      />
                      <DiagnosticValueItem
                        label={t.admin.pluginDiagnosticsMinHostProtocolVersion}
                        value={compatibility?.min_host_protocol_version || '-'}
                      />
                      <DiagnosticValueItem
                        label={t.admin.pluginDiagnosticsMaxHostProtocolVersion}
                        value={compatibility?.max_host_protocol_version || '-'}
                      />
                    </div>
                  </DiagnosticGroupCard>
                </div>

                <DiagnosticGroupCard
                  title={t.admin.pluginDiagnosticsCompatibilityReason}
                  summary={
                    compatibility?.runtime
                      ? `${t.admin.pluginRuntime}: ${compatibility.runtime}`
                      : undefined
                  }
                >
                  <DiagnosticValueItem
                    label={t.admin.pluginDiagnosticsCompatibilityReason}
                    value={compatibility?.reason || '-'}
                  />
                </DiagnosticGroupCard>
              </CardContent>
            </Card>

            <Card id="plugin-diagnostics-registration" className="scroll-mt-24">
              <CardHeader className="pb-3">
                <CardTitle>{t.admin.pluginDiagnosticsRegistration}</CardTitle>
              </CardHeader>
              <CardContent className="grid gap-3 text-sm md:grid-cols-2 xl:grid-cols-3">
                <div className="rounded-md border border-input/60 p-3">
                  <p className="text-xs text-muted-foreground">
                    {t.admin.pluginDiagnosticsRegistrationState}
                  </p>
                  <div className="mt-1">
                    <Badge variant={registrationStateVariant(registration?.state)}>
                      {registrationStateLabel(registration?.state, t)}
                    </Badge>
                  </div>
                </div>
                <div className="rounded-md border border-input/60 p-3">
                  <p className="text-xs text-muted-foreground">
                    {t.admin.pluginDiagnosticsRegistrationTrigger}
                  </p>
                  <p className="mt-1 font-medium">
                    {registrationTriggerLabel(registration?.trigger, t)}
                  </p>
                </div>
                <div className="rounded-md border border-input/60 p-3">
                  <p className="text-xs text-muted-foreground">
                    {t.admin.pluginDiagnosticsRegistrationRuntime}
                  </p>
                  <p className="mt-1 font-medium">{registration?.runtime || '-'}</p>
                </div>
                <div className="rounded-md border border-input/60 p-3">
                  <p className="text-xs text-muted-foreground">
                    {t.admin.pluginDiagnosticsRegistrationAttemptedAt}
                  </p>
                  <p className="mt-1 font-medium">{formatDateTime(registration?.attempted_at)}</p>
                </div>
                <div className="rounded-md border border-input/60 p-3">
                  <p className="text-xs text-muted-foreground">
                    {t.admin.pluginDiagnosticsRegistrationCompletedAt}
                  </p>
                  <p className="mt-1 font-medium">{formatDateTime(registration?.completed_at)}</p>
                </div>
                <div className="rounded-md border border-input/60 p-3">
                  <p className="text-xs text-muted-foreground">
                    {t.admin.pluginDiagnosticsRegistrationDuration}
                  </p>
                  <p className="mt-1 font-medium">
                    {typeof registration?.duration_ms === 'number'
                      ? `${registration.duration_ms} ms`
                      : '-'}
                  </p>
                </div>
                <div className="rounded-md border border-input/60 p-3 md:col-span-2 xl:col-span-3">
                  <p className="text-xs text-muted-foreground">
                    {t.admin.pluginDiagnosticsRegistrationDetail}
                  </p>
                  <p className="mt-1 break-all font-medium">{registration?.detail || '-'}</p>
                </div>
              </CardContent>
            </Card>

            <Card id="plugin-diagnostics-deployments" className="scroll-mt-24">
              <CardHeader className="pb-3">
                <CardTitle>{t.admin.pluginDiagnosticsDeployments}</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="grid gap-3 text-sm md:grid-cols-2 xl:grid-cols-4">
                  <div className="rounded-md border border-input/60 p-3">
                    <p className="text-xs text-muted-foreground">
                      {t.admin.pluginDiagnosticsGenerationDesired}
                    </p>
                    <p className="mt-1 font-medium">
                      {diagnosticPlugin?.desired_generation ?? '-'}
                    </p>
                  </div>
                  <div className="rounded-md border border-input/60 p-3">
                    <p className="text-xs text-muted-foreground">
                      {t.admin.pluginDiagnosticsGenerationApplied}
                    </p>
                    <p className="mt-1 font-medium">
                      {diagnosticPlugin?.applied_generation ?? '-'}
                    </p>
                  </div>
                  <div className="rounded-md border border-input/60 p-3">
                    <p className="text-xs text-muted-foreground">
                      {t.admin.pluginDiagnosticsLatestDeployment}
                    </p>
                    <p className="mt-1 font-medium">
                      {diagnosticPlugin?.latest_deployment
                        ? deploymentOperationLabel(diagnosticPlugin.latest_deployment.operation, t)
                        : '-'}
                    </p>
                  </div>
                  <div className="rounded-md border border-input/60 p-3">
                    <p className="text-xs text-muted-foreground">
                      {t.admin.pluginDiagnosticsLatestDeploymentStatus}
                    </p>
                    <div className="mt-1">
                      <Badge
                        variant={deploymentStatusVariant(
                          diagnosticPlugin?.latest_deployment?.status
                        )}
                      >
                        {deploymentStatusLabel(diagnosticPlugin?.latest_deployment?.status, t)}
                      </Badge>
                    </div>
                  </div>
                </div>

                {renderExpandableList({
                  items: recentDeployments,
                  initialCount: 4,
                  emptyText: t.admin.pluginDiagnosticsNoDeployments,
                  summaryLabel: t.common.more,
                  renderItems: (deployments) =>
                    deployments.map((deployment) => (
                      <div
                        key={deployment.id}
                        className="rounded-md border border-input/60 p-3 text-sm"
                      >
                        <div className="flex flex-wrap items-center gap-2">
                          <Badge variant={deploymentStatusVariant(deployment.status)}>
                            {deploymentStatusLabel(deployment.status, t)}
                          </Badge>
                          <Badge variant="outline">
                            {deploymentOperationLabel(deployment.operation, t)}
                          </Badge>
                          <span className="text-xs text-muted-foreground">
                            {t.admin.pluginDiagnosticsDeploymentGeneration.replace(
                              '{value}',
                              `${deployment.applied_generation || 0}/${deployment.requested_generation || 0}`
                            )}
                          </span>
                        </div>
                        <p className="mt-2 text-xs text-muted-foreground">
                          {formatDateTime(deployment.created_at)}
                        </p>
                        <p className="mt-2 break-all text-xs text-muted-foreground">
                          {deployment.detail || deployment.error || '-'}
                        </p>
                      </div>
                    )),
                })}
              </CardContent>
            </Card>

            <Card id="plugin-diagnostics-tasks" className="scroll-mt-24">
              <CardHeader className="pb-3">
                <CardTitle>{t.admin.pluginDiagnosticsExecutionTasks}</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="flex flex-wrap gap-2">
                  <Badge variant="active">
                    {t.admin.pluginDiagnosticsExecutionTasksActiveCount.replace(
                      '{count}',
                      String(executionTasks?.active_count ?? activeTasks.length)
                    )}
                  </Badge>
                  <Badge variant="outline">
                    {t.admin.pluginDiagnosticsExecutionTasksRecentCount.replace(
                      '{count}',
                      String(executionTasks?.recent_count ?? recentTasks.length)
                    )}
                  </Badge>
                </div>

                <div className="grid gap-4 xl:grid-cols-2">
                  <div className="space-y-3">
                    <p className="text-sm font-medium">
                      {t.admin.pluginDiagnosticsExecutionTasksActive}
                    </p>
                    {renderExpandableList({
                      items: activeTasks,
                      initialCount: 2,
                      emptyText: t.admin.pluginDiagnosticsExecutionTasksNoActive,
                      summaryLabel: t.common.more,
                      renderItems: (tasks) =>
                        renderTaskList(
                          tasks,
                          t.admin.pluginDiagnosticsExecutionTasksNoActive,
                          cancelingTaskID,
                          onCancelTask,
                          formatDateTime,
                          t
                        ),
                    })}
                  </div>

                  <div className="space-y-3">
                    <p className="text-sm font-medium">
                      {t.admin.pluginDiagnosticsExecutionTasksRecent}
                    </p>
                    {renderExpandableList({
                      items: recentTasks,
                      initialCount: 3,
                      emptyText: t.admin.pluginDiagnosticsExecutionTasksNoRecent,
                      summaryLabel: t.common.more,
                      renderItems: (tasks) =>
                        renderTaskList(
                          tasks,
                          t.admin.pluginDiagnosticsExecutionTasksNoRecent,
                          cancelingTaskID,
                          onCancelTask,
                          formatDateTime,
                          t
                        ),
                    })}
                  </div>
                </div>
              </CardContent>
            </Card>

            <Card id="plugin-diagnostics-storage" className="scroll-mt-24">
              <CardHeader className="pb-3">
                <CardTitle>{t.admin.pluginDiagnosticsStorageAccess}</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="grid gap-3 text-sm md:grid-cols-2 xl:grid-cols-4">
                  <div className="rounded-md border border-input/60 p-3">
                    <p className="text-xs text-muted-foreground">
                      {t.admin.pluginDiagnosticsStorageProfileCount}
                    </p>
                    <p className="mt-1 font-medium">{storageDiagnostics?.profile_count ?? 0}</p>
                  </div>
                  <div className="rounded-md border border-input/60 p-3">
                    <p className="text-xs text-muted-foreground">
                      {t.admin.pluginDiagnosticsStorageLastAction}
                    </p>
                    <p className="mt-1 font-mono text-xs font-medium">
                      {lastObservedStorage?.action || '-'}
                    </p>
                  </div>
                  <div className="rounded-md border border-input/60 p-3">
                    <p className="text-xs text-muted-foreground">
                      {t.admin.pluginDiagnosticsStorageDeclaredAccess}
                    </p>
                    <div className="mt-1">
                      <Badge
                        variant={storageAccessVariant(lastObservedStorage?.declared_access_mode)}
                      >
                        {storageAccessLabel(lastObservedStorage?.declared_access_mode, t)}
                      </Badge>
                    </div>
                  </div>
                  <div className="rounded-md border border-input/60 p-3">
                    <p className="text-xs text-muted-foreground">
                      {t.admin.pluginDiagnosticsStorageObservedAccess}
                    </p>
                    <div className="mt-1">
                      <Badge
                        variant={storageAccessVariant(lastObservedStorage?.observed_access_mode)}
                      >
                        {storageAccessLabel(lastObservedStorage?.observed_access_mode, t)}
                      </Badge>
                    </div>
                  </div>
                  <div className="rounded-md border border-input/60 p-3">
                    <p className="text-xs text-muted-foreground">
                      {t.admin.pluginDiagnosticsStorageLastUpdatedAt}
                    </p>
                    <p className="mt-1 font-medium">
                      {formatDateTime(lastObservedStorage?.updated_at)}
                    </p>
                  </div>
                  <div className="rounded-md border border-input/60 p-3">
                    <p className="text-xs text-muted-foreground">
                      {t.admin.pluginDiagnosticsStorageStatus}
                    </p>
                    <p className="mt-1 font-medium">
                      {taskStatusLabel(lastObservedStorage?.status, t)}
                    </p>
                  </div>
                  <div className="rounded-md border border-input/60 p-3">
                    <p className="text-xs text-muted-foreground">
                      {t.admin.pluginDiagnosticsStorageTaskId}
                    </p>
                    <p className="mt-1 font-mono text-xs font-medium">
                      {lastObservedStorage?.task_id || '-'}
                    </p>
                  </div>
                  <div className="rounded-md border border-input/60 p-3">
                    <p className="text-xs text-muted-foreground">
                      {t.admin.pluginDiagnosticsStorageSource}
                    </p>
                    <p className="mt-1 font-medium">
                      {storageObservationSourceLabel(lastObservedStorage?.source, t)}
                    </p>
                  </div>
                </div>

                {lastObservedStorage ? (
                  <div className="rounded-md border border-input/60 p-3 text-sm">
                    <div className="flex flex-wrap items-center gap-2">
                      {lastObservedStorage.hook ? (
                        <Badge variant="secondary">{lastObservedStorage.hook}</Badge>
                      ) : null}
                      {lastObservedStorage.stream ? (
                        <Badge variant="outline">{t.admin.pluginDiagnosticsTaskStream}</Badge>
                      ) : null}
                    </div>
                    {lastObservedStorage.declared_access_mode === 'unknown' ? (
                      <p className="mt-3 text-xs text-muted-foreground">
                        {t.admin.pluginDiagnosticsStorageUndeclaredHint}
                      </p>
                    ) : null}
                  </div>
                ) : (
                  <p className="text-sm text-muted-foreground">
                    {t.admin.pluginDiagnosticsStorageNoObserved}
                  </p>
                )}

                <div className="space-y-3">
                  <p className="text-sm font-medium">{t.admin.pluginDiagnosticsStorageProfiles}</p>
                  {renderStorageProfileList(
                    declaredStorageProfiles,
                    lastObservedStorage?.action,
                    t.admin.pluginDiagnosticsStorageNoProfiles,
                    t.admin.pluginDiagnosticsStorageLatest,
                    t
                  )}
                </div>
              </CardContent>
            </Card>

            <Card id="plugin-diagnostics-observability" className="scroll-mt-24">
              <CardHeader className="pb-3">
                <CardTitle>{t.admin.pluginDiagnosticsObservability}</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="grid gap-3 text-sm md:grid-cols-2 xl:grid-cols-4">
                  <div className="rounded-md border border-input/60 p-3">
                    <p className="text-xs text-muted-foreground">
                      {t.admin.pluginDiagnosticsObservationWindow}
                    </p>
                    <p className="mt-1 font-medium">
                      {executionObservability?.window_hours ?? 24}h
                    </p>
                  </div>
                  <div className="rounded-md border border-input/60 p-3">
                    <p className="text-xs text-muted-foreground">
                      {t.admin.pluginDiagnosticsTotalExecutions}
                    </p>
                    <p className="mt-1 font-medium">
                      {executionObservability?.total_executions ?? 0}
                    </p>
                  </div>
                  <div className="rounded-md border border-input/60 p-3">
                    <p className="text-xs text-muted-foreground">
                      {t.admin.pluginDiagnosticsFailedExecutions}
                    </p>
                    <p className="mt-1 font-medium">
                      {executionObservability?.failed_executions ?? 0}
                    </p>
                  </div>
                  <div className="rounded-md border border-input/60 p-3">
                    <p className="text-xs text-muted-foreground">
                      {t.admin.pluginDiagnosticsHookFailedExecutions}
                    </p>
                    <p className="mt-1 font-medium">
                      {executionObservability?.hook_failed_executions ?? 0}
                    </p>
                  </div>
                  <div className="rounded-md border border-input/60 p-3">
                    <p className="text-xs text-muted-foreground">
                      {t.admin.pluginDiagnosticsActionFailedExecutions}
                    </p>
                    <p className="mt-1 font-medium">
                      {executionObservability?.action_failed_executions ?? 0}
                    </p>
                  </div>
                  <div className="rounded-md border border-input/60 p-3">
                    <p className="text-xs text-muted-foreground">
                      {t.admin.pluginDiagnosticsLastFailureAt}
                    </p>
                    <p className="mt-1 font-medium">
                      {formatDateTime(executionObservability?.last_failure_at)}
                    </p>
                  </div>
                  <div className="rounded-md border border-input/60 p-3">
                    <p className="text-xs text-muted-foreground">
                      {t.admin.pluginDiagnosticsLastSuccessAt}
                    </p>
                    <p className="mt-1 font-medium">
                      {formatDateTime(executionObservability?.last_success_at)}
                    </p>
                  </div>
                </div>

                <div className="grid gap-4 xl:grid-cols-2">
                  <div className="space-y-3">
                    <p className="text-sm font-medium">{t.admin.pluginDiagnosticsFailureGroups}</p>
                    {renderExpandableList({
                      items: failureGroups,
                      initialCount: 4,
                      emptyText: t.admin.pluginDiagnosticsNoFailureGroups,
                      summaryLabel: t.common.more,
                      renderItems: (groups) =>
                        renderFailureGroupList(
                          groups,
                          t.admin.pluginDiagnosticsNoFailureGroups,
                          formatDateTime,
                          t
                        ),
                    })}
                  </div>

                  <div className="space-y-3">
                    <p className="text-sm font-medium">{t.admin.pluginDiagnosticsRecentFailures}</p>
                    {renderExpandableList({
                      items: recentFailures,
                      initialCount: 4,
                      emptyText: t.admin.pluginDiagnosticsNoRecentFailures,
                      summaryLabel: t.common.more,
                      renderItems: (samples) =>
                        renderFailureSampleList(
                          samples,
                          t.admin.pluginDiagnosticsNoRecentFailures,
                          formatDateTime,
                          t
                        ),
                    })}
                  </div>
                </div>
              </CardContent>
            </Card>

            <Card id="plugin-diagnostics-public-cache" className="scroll-mt-24">
              <CardHeader className="pb-3">
                <CardTitle>{t.admin.pluginDiagnosticsPublicCache}</CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                <p className="text-xs text-muted-foreground">
                  {joinSummaryItems([
                    t.admin.pluginDiagnosticsPublicCacheTotalEntries.replace(
                      '{count}',
                      String(cacheTotalEntryCount)
                    ),
                    t.admin.pluginDiagnosticsPublicCacheMatchingEntries.replace(
                      '{count}',
                      String(cacheMatchingEntryCount)
                    ),
                  ])}
                </p>
                <div className="grid gap-3 text-sm md:grid-cols-2">
                  <div className="rounded-md border border-input/60 p-3">
                    <p className="text-xs text-muted-foreground">
                      {t.admin.pluginDiagnosticsPublicCacheTtl}
                    </p>
                    <p className="mt-1 font-medium">
                      {typeof publicCache?.ttl_seconds === 'number'
                        ? `${publicCache.ttl_seconds}s`
                        : '-'}
                    </p>
                  </div>
                  <div className="rounded-md border border-input/60 p-3">
                    <p className="text-xs text-muted-foreground">
                      {t.admin.pluginDiagnosticsPublicCacheMaxEntries}
                    </p>
                    <p className="mt-1 font-medium">{publicCache?.max_entries ?? '-'}</p>
                  </div>
                </div>

                <div className="grid gap-3 xl:grid-cols-2">
                  {[
                    {
                      key: 'extensions',
                      title: t.admin.pluginDiagnosticsPublicCacheExtensions,
                      bucket: extensionCache,
                    },
                    {
                      key: 'bootstrap',
                      title: t.admin.pluginDiagnosticsPublicCacheBootstrap,
                      bucket: bootstrapCache,
                    },
                  ].map(({ key, title, bucket }) => (
                    <div key={key} className="rounded-md border border-input/60 p-3">
                      <div className="space-y-1">
                        <p className="font-medium">{title}</p>
                        <p className="text-xs text-muted-foreground">
                          {joinSummaryItems([
                            t.admin.pluginDiagnosticsPublicCacheTotalEntries.replace(
                              '{count}',
                              String(bucket?.total_entries ?? 0)
                            ),
                            t.admin.pluginDiagnosticsPublicCacheMatchingEntries.replace(
                              '{count}',
                              String(bucket?.matching_entries ?? 0)
                            ),
                          ])}
                        </p>
                      </div>
                      <div className="mt-3">
                        {renderExpandableList({
                          items: Array.isArray(bucket?.entries) ? bucket.entries : [],
                          initialCount: 3,
                          emptyText: t.admin.pluginDiagnosticsPublicCacheNoEntries,
                          summaryLabel: t.common.more,
                          renderItems: (entries) =>
                            entries.map((entry) => (
                              <div
                                key={entry.key}
                                className="rounded-md border border-input/50 bg-background/70 p-3 text-sm transition-colors hover:bg-muted/20"
                              >
                                <div className="flex flex-wrap items-start justify-between gap-3">
                                  <div className="min-w-0 flex-1 space-y-2">
                                    <p className="min-w-0 break-all font-medium">
                                      {entry.path || entry.key}
                                    </p>
                                    <p className="text-xs text-muted-foreground">
                                      {joinSummaryItems([
                                        entry.area ? areaLabel(entry.area, t) : null,
                                        entry.slot || null,
                                      ])}
                                    </p>
                                    {entry.path && entry.path !== entry.key ? (
                                      <p className="break-all font-mono text-xs text-muted-foreground">
                                        {entry.key}
                                      </p>
                                    ) : null}
                                  </div>
                                  <p className="text-xs text-muted-foreground">
                                    {joinSummaryItems([
                                      `${t.admin.pluginDiagnosticsPublicCacheExtensionCount}: ${
                                        entry.extension_count ?? 0
                                      }`,
                                      `${t.admin.pluginDiagnosticsPublicCacheMenuCount}: ${
                                        entry.menu_count ?? 0
                                      }`,
                                      `${t.admin.pluginDiagnosticsPublicCacheRouteCount}: ${
                                        entry.route_count ?? 0
                                      }`,
                                    ])}
                                  </p>
                                </div>
                                <div className="mt-3 grid gap-2 text-xs text-muted-foreground md:grid-cols-2">
                                  <p>
                                    {t.admin.pluginDiagnosticsPublicCachePath}: {entry.path || '-'}
                                  </p>
                                  <p>
                                    {t.admin.pluginDiagnosticsPublicCacheArea}: {entry.area || '-'}
                                  </p>
                                  <p>
                                    {t.admin.pluginDiagnosticsPublicCacheSlot}: {entry.slot || '-'}
                                  </p>
                                  <p>
                                    {t.admin.pluginDiagnosticsPublicCacheCreatedAt}:{' '}
                                    {formatDateTime(entry.created_at)}
                                  </p>
                                  <p>
                                    {t.admin.pluginDiagnosticsPublicCacheExpiresAt}:{' '}
                                    {formatDateTime(entry.expires_at)}
                                  </p>
                                </div>
                              </div>
                            )),
                        })}
                      </div>
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>

            <Card id="plugin-diagnostics-checks" className="scroll-mt-24">
              <CardHeader className="pb-3">
                <CardTitle>{t.admin.pluginDiagnosticsChecks}</CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                <p className="text-xs text-muted-foreground">
                  {joinSummaryItems([
                    `${t.common.error}: ${failedCheckCount}`,
                    `${t.common.warning}: ${warningCheckCount}`,
                    `${t.common.success}: ${checkOkCount}`,
                  ])}
                </p>
                {renderExpandableList({
                  items: checks,
                  initialCount: 6,
                  emptyText: t.common.noData,
                  summaryLabel: t.common.more,
                  renderItems: (items) => (
                    <div className="grid gap-3 md:grid-cols-2">
                      {items.map((check) => (
                        <div
                          key={check.key}
                          className="rounded-md border border-input/60 bg-background/70 p-3 text-sm transition-colors hover:bg-muted/20"
                        >
                          <div className="space-y-2">
                            <div className="flex flex-wrap items-center gap-2">
                              <Badge variant={stateVariant(check.state)}>
                                {stateLabel(check.state, t)}
                              </Badge>
                              <p className="font-mono text-[11px] text-muted-foreground">
                                {check.key}
                              </p>
                            </div>
                            <p className="font-medium">{check.summary}</p>
                          </div>
                          {check.detail ? (
                            <p className="mt-2 break-all text-xs text-muted-foreground">
                              {check.detail}
                            </p>
                          ) : null}
                        </div>
                      ))}
                    </div>
                  ),
                })}
              </CardContent>
            </Card>

            <Card id="plugin-diagnostics-permissions" className="scroll-mt-24">
              <CardHeader className="pb-3">
                <CardTitle>{t.admin.pluginDiagnosticsMissingPermissions}</CardTitle>
              </CardHeader>
              <CardContent>
                {renderExpandableList({
                  items: missingPermissions,
                  initialCount: 10,
                  emptyText: t.admin.pluginDiagnosticsNoMissingPermissions,
                  summaryLabel: t.common.more,
                  className: 'space-y-2',
                  renderItems: (items) => (
                    <div className="flex flex-wrap gap-2">
                      {items.map((item) => (
                        <Badge key={item} variant="outline">
                          {item}
                        </Badge>
                      ))}
                    </div>
                  ),
                })}
              </CardContent>
            </Card>

            <Card id="plugin-diagnostics-routes" className="scroll-mt-24">
              <CardHeader className="pb-3">
                <CardTitle>{t.admin.pluginDiagnosticsFrontendRoutes}</CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                <p className="text-xs text-muted-foreground">
                  {joinSummaryItems([
                    `${t.admin.pluginDiagnosticsFrontendRoutes}: ${routes.length}`,
                    `${t.admin.pluginDiagnosticsExecuteApiAvailable}: ${executeApiAvailableCount}`,
                    `${t.admin.pluginDiagnosticsExecuteApiUnavailable}: ${unavailableRouteCount}`,
                    `${t.admin.pluginDiagnosticsVisible}: ${routeScopeStats.visible}`,
                    `${t.admin.pluginDiagnosticsEligible}: ${routeScopeStats.eligible}`,
                    `${t.admin.pluginDiagnosticsUnavailable}: ${routeScopeStats.unavailable}`,
                  ])}
                </p>
                {renderExpandableList({
                  items: routes,
                  initialCount: 4,
                  emptyText: t.common.noData,
                  summaryLabel: t.common.more,
                  renderItems: (items) => (
                    <div className="space-y-3">
                      {items.map((route) => {
                        const scopeChecks = Array.isArray(route.scope_checks) ? route.scope_checks : []
                        const visibleCount = scopeChecks.filter(
                          (scopeCheck) => scopeCheck.frontend_visible
                        ).length
                        const eligibleCount = scopeChecks.filter(
                          (scopeCheck) => scopeCheck.eligible && !scopeCheck.frontend_visible
                        ).length
                        const unavailableCount = scopeChecks.filter(
                          (scopeCheck) => !scopeCheck.eligible
                        ).length

                        return (
                          <div
                            key={`${route.area}-${route.path}`}
                            className="rounded-md border border-input/60 bg-background/70 p-3 transition-colors hover:bg-muted/20"
                          >
                            <div className="space-y-2">
                              <p className="min-w-0 break-all font-medium">{route.path}</p>
                              <p className="text-xs text-muted-foreground">
                                {joinSummaryItems([
                                  areaLabel(route.area, t),
                                  route.execute_api_available
                                    ? t.admin.pluginDiagnosticsExecuteApiAvailable
                                    : t.admin.pluginDiagnosticsExecuteApiUnavailable,
                                  `${t.admin.pluginDiagnosticsVisible}: ${visibleCount}`,
                                  `${t.admin.pluginDiagnosticsEligible}: ${eligibleCount}`,
                                  `${t.admin.pluginDiagnosticsUnavailable}: ${unavailableCount}`,
                                ])}
                              </p>
                            </div>
                            <div className="mt-3 grid gap-3 md:grid-cols-2 xl:grid-cols-3">
                              {scopeChecks.length === 0 ? (
                                <p className="text-sm text-muted-foreground">{t.common.noData}</p>
                              ) : (
                                scopeChecks.map((scopeCheck) => (
                                  <div
                                    key={`${route.area}-${route.path}-${scopeCheck.scope}`}
                                    className="rounded-md border border-input/50 bg-muted/10 p-3 text-sm"
                                  >
                                    <div className="space-y-2">
                                      <div className="flex flex-wrap items-center justify-between gap-2">
                                        <p className="font-medium">
                                          {scopeLabel(scopeCheck.scope, t)}
                                        </p>
                                        <div className="flex flex-wrap items-center gap-2">
                                          <Badge
                                            variant={
                                              scopeCheck.frontend_visible
                                                ? 'default'
                                                : scopeCheck.eligible
                                                  ? 'outline'
                                                  : 'secondary'
                                            }
                                          >
                                            {scopeCheck.frontend_visible
                                              ? t.admin.pluginDiagnosticsVisible
                                              : scopeCheck.eligible
                                                ? t.admin.pluginDiagnosticsEligible
                                                : t.admin.pluginDiagnosticsUnavailable}
                                          </Badge>
                                        </div>
                                      </div>
                                      {scopeCheck.reason_code ? (
                                        <p className="font-mono text-[11px] text-muted-foreground">
                                          {scopeCheck.reason_code}
                                        </p>
                                      ) : null}
                                    </div>
                                    {(scopeCheck.reason || scopeCheck.reason_code) &&
                                    !scopeCheck.frontend_visible ? (
                                      <p className="mt-2 break-all text-xs text-muted-foreground">
                                        {scopeCheck.reason || scopeCheck.reason_code}
                                      </p>
                                    ) : null}
                                  </div>
                                ))
                              )}
                            </div>
                          </div>
                        )
                      })}
                    </div>
                  ),
                })}
              </CardContent>
            </Card>

            <Card id="plugin-diagnostics-issues" className="scroll-mt-24">
              <CardHeader className="pb-3">
                <CardTitle>{t.admin.pluginDiagnosticsIssues}</CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                <p className="text-xs text-muted-foreground">
                  {joinSummaryItems([
                    `${t.common.error}: ${issueErrorCount}`,
                    `${t.common.warning}: ${issueWarnCount}`,
                    `${t.common.info}: ${issueInfoCount}`,
                  ])}
                </p>
                {renderExpandableList({
                  items: issues,
                  initialCount: 6,
                  emptyText: t.admin.pluginDiagnosticsNoIssues,
                  summaryLabel: t.common.more,
                  renderItems: (items) => (
                    <div className="space-y-3">
                      {items.map((issue, index) => (
                        <div
                          key={`${issue.code}-${index}`}
                          className="rounded-md border border-input/60 bg-background/70 p-3 text-sm transition-colors hover:bg-muted/20"
                        >
                          <div className="space-y-2">
                            <div className="flex flex-wrap items-center gap-2">
                              <Badge variant={severityVariant(issue.severity)}>
                                {severityLabel(issue.severity, t)}
                              </Badge>
                              {issue.code ? (
                                <p className="font-mono text-[11px] text-muted-foreground">
                                  {issue.code}
                                </p>
                              ) : null}
                            </div>
                            <p className="font-medium">{issue.summary}</p>
                          </div>
                          {issue.detail ? (
                            <p className="mt-2 break-all text-xs text-muted-foreground">
                              {issue.detail}
                            </p>
                          ) : null}
                          {issue.hint ? (
                            <div className="mt-3 rounded-md border border-input/50 bg-muted/10 p-2 text-xs text-muted-foreground">
                              {issue.hint}
                            </div>
                          ) : null}
                        </div>
                      ))}
                    </div>
                  ),
                })}
              </CardContent>
            </Card>
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}
