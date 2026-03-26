'use client'

import { useEffect, useMemo, useState } from 'react'

import { ChevronDown, ChevronRight, Copy, Loader2 } from 'lucide-react'

import { useToast } from '@/hooks/use-toast'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import type { AdminPlugin, AdminPluginExecution } from '@/lib/api'
import type { Translations } from '@/lib/i18n'

type ExecutionStatusFilter = 'all' | 'success' | 'failed'

type PluginExecutionLogsDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  plugin?: AdminPlugin | null
  logsLoading: boolean
  logs: AdminPluginExecution[]
  formatDateTime: (value?: string, locale?: string) => string
  locale: string
  t: Translations
}

function hasExecutionPayload(value?: string): boolean {
  return typeof value === 'string' && value.trim() !== ''
}

function formatExecutionPayload(value?: string): string {
  if (!hasExecutionPayload(value)) {
    return ''
  }
  const raw = String(value).trim()
  try {
    return JSON.stringify(JSON.parse(raw), null, 2)
  } catch {
    return raw
  }
}

function joinSummaryItems(items: Array<string | null | undefined | false>): string {
  return items
    .filter((item): item is string => typeof item === 'string' && item.trim() !== '')
    .join(' · ')
}

function ExecutionLogPayloadSection({
  title,
  value,
  emptyText,
  tone = 'default',
  onCopy,
  t,
}: {
  title: string
  value?: string
  emptyText: string
  tone?: 'default' | 'error'
  onCopy?: (content: string) => void
  t: Translations
}) {
  const content = formatExecutionPayload(value)
  const lines = useMemo(() => (content ? content.split('\n') : []), [content])
  const isLongContent = lines.length > 10 || content.length > 640
  const [expanded, setExpanded] = useState(tone === 'error' || !isLongContent)

  useEffect(() => {
    setExpanded(tone === 'error' || !isLongContent)
  }, [content, isLongContent, tone])

  const preview = useMemo(() => lines.slice(0, 8).join('\n'), [lines])
  return (
    <div className="space-y-2 rounded-md border border-input/60 bg-background p-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">{title}</p>
        {content ? (
          <div className="flex flex-wrap items-center gap-2">
            {isLongContent ? (
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="h-7 gap-1 px-2 text-xs"
                onClick={() => setExpanded((current) => !current)}
                aria-expanded={expanded}
              >
                {expanded ? (
                  <ChevronDown className="h-3.5 w-3.5" />
                ) : (
                  <ChevronRight className="h-3.5 w-3.5" />
                )}
                {expanded ? t.common.collapse : t.common.expand}
              </Button>
            ) : null}
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="h-7 gap-1 px-2 text-xs"
              onClick={() => onCopy?.(content)}
            >
              <Copy className="h-3.5 w-3.5" />
              {t.common.copy}
            </Button>
          </div>
        ) : null}
      </div>
      {content ? (
        expanded ? (
          <pre
            className={`max-h-56 overflow-auto whitespace-pre-wrap break-all rounded-md border p-2 text-xs ${
              tone === 'error'
                ? 'border-destructive/30 bg-destructive/10 text-destructive'
                : 'bg-muted/30'
            }`}
          >
            {content}
          </pre>
        ) : (
          <pre
            className={`max-h-48 overflow-auto whitespace-pre-wrap break-all rounded-md border p-2 text-xs ${
              tone === 'error'
                ? 'border-destructive/30 bg-destructive/10 text-destructive'
                : 'bg-muted/30'
            }`}
          >
            {preview}
            {'\n...'}
          </pre>
        )
      ) : (
        <p className="text-xs text-muted-foreground">{emptyText}</p>
      )}
    </div>
  )
}

function executionStatusVariant(
  success: boolean
): 'default' | 'secondary' | 'destructive' | 'outline' | 'active' {
  return success ? 'default' : 'destructive'
}

export function PluginExecutionLogsDialog({
  open,
  onOpenChange,
  plugin,
  logsLoading,
  logs,
  formatDateTime,
  locale,
  t,
}: PluginExecutionLogsDialogProps) {
  const toast = useToast()
  const [statusFilter, setStatusFilter] = useState<ExecutionStatusFilter>('all')
  const [actionFilter, setActionFilter] = useState('all')
  const [searchText, setSearchText] = useState('')

  useEffect(() => {
    if (open) {
      setStatusFilter('all')
      setActionFilter('all')
      setSearchText('')
    }
  }, [open])

  const successCount = logs.filter((item) => item.success).length
  const failedCount = logs.length - successCount
  const actionOptions = useMemo(
    () =>
      Array.from(
        logs.reduce((acc, item) => {
          const action = String(item.action || '').trim()
          if (!action) {
            return acc
          }
          acc.set(action, (acc.get(action) || 0) + 1)
          return acc
        }, new Map<string, number>())
      )
        .sort(([left], [right]) => left.localeCompare(right))
        .map(([action, count]) => ({ action, count })),
    [logs]
  )
  const normalizedSearchText = searchText.trim().toLowerCase()
  const hasActiveFilters =
    statusFilter !== 'all' || actionFilter !== 'all' || normalizedSearchText !== ''
  const activeFilterSummary = joinSummaryItems([
    statusFilter !== 'all'
      ? `${t.admin.pluginExecutionStatusFilter}: ${
          statusFilter === 'success' ? t.common.success : t.common.failed
        }`
      : null,
    actionFilter !== 'all' ? `${t.admin.pluginExecutionActionFilter}: ${actionFilter}` : null,
    normalizedSearchText ? `${t.common.search}: ${searchText.trim()}` : null,
  ])

  const filteredLogs = useMemo(
    () =>
      logs.filter((item) => {
        if (statusFilter === 'success' && !item.success) {
          return false
        }
        if (statusFilter === 'failed' && item.success) {
          return false
        }
        if (actionFilter !== 'all' && item.action !== actionFilter) {
          return false
        }
        if (!normalizedSearchText) {
          return true
        }
        const haystack = [item.action, item.params, item.result, item.error]
          .map((value) => String(value || '').toLowerCase())
          .join('\n')
        return haystack.includes(normalizedSearchText)
      }),
    [actionFilter, logs, normalizedSearchText, statusFilter]
  )
  const copyPayload = async (content: string) => {
    if (!content) return
    try {
      await navigator.clipboard.writeText(content)
      toast.success(t.common.copiedToClipboard)
    } catch {
      toast.error(locale === 'zh' ? '复制失败' : 'Copy failed')
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] max-w-6xl overflow-y-auto [&_.grid>*]:min-w-0">
        <DialogHeader>
          <DialogTitle>{t.admin.pluginExecutions}</DialogTitle>
          <DialogDescription>
            {plugin
              ? `${plugin.display_name || plugin.name}${plugin.display_name && plugin.name ? ` (${plugin.name})` : ''} · ${t.admin.pluginExecutionsSubtitle}`
              : t.admin.pluginExecutionsSubtitle}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-3 rounded-lg border border-input bg-background/95 p-3">
          {plugin?.address_display || plugin?.address ? (
            <p className="text-xs text-muted-foreground">
              {plugin.address_display || plugin.address}
            </p>
          ) : null}
          <p className="text-xs text-muted-foreground">
            {t.admin.pluginExecutionSummaryTotal.replace('{count}', '').trim()}: {logs.length} ·{' '}
            {t.admin.pluginExecutionSummarySuccess.replace('{count}', '').trim()}: {successCount} ·{' '}
            {t.admin.pluginExecutionSummaryFailed.replace('{count}', '').trim()}: {failedCount}
          </p>
          <div className="grid gap-3 md:grid-cols-3">
            <div className="space-y-2">
              <Label htmlFor="plugin-execution-status-filter">
                {t.admin.pluginExecutionStatusFilter}
              </Label>
              <Select
                value={statusFilter}
                onValueChange={(value) => setStatusFilter(value as ExecutionStatusFilter)}
                disabled={logsLoading || logs.length === 0}
              >
                <SelectTrigger id="plugin-execution-status-filter">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">{`${t.common.all} (${logs.length})`}</SelectItem>
                  <SelectItem value="success">{`${t.common.success} (${successCount})`}</SelectItem>
                  <SelectItem value="failed">{`${t.common.failed} (${failedCount})`}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="plugin-execution-action-filter">
                {t.admin.pluginExecutionActionFilter}
              </Label>
              <Select
                value={actionFilter}
                onValueChange={setActionFilter}
                disabled={logsLoading || actionOptions.length === 0}
              >
                <SelectTrigger id="plugin-execution-action-filter">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">{t.admin.pluginExecutionActionFilterAll}</SelectItem>
                  {actionOptions.map((item) => (
                    <SelectItem key={item.action} value={item.action}>
                      {`${item.action} (${item.count})`}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="plugin-execution-search">{t.common.search}</Label>
              <Input
                id="plugin-execution-search"
                value={searchText}
                onChange={(event) => setSearchText(event.target.value)}
                placeholder={t.admin.pluginExecutionSearchPlaceholder}
                disabled={logsLoading || logs.length === 0}
              />
            </div>
          </div>
          <div className="flex flex-wrap items-center justify-between gap-2">
            <p className="text-xs text-muted-foreground">
              {hasActiveFilters ? activeFilterSummary : t.admin.pluginExecutionActionFilterAll}
            </p>
            {hasActiveFilters ? (
              <Button
                variant="ghost"
                size="sm"
                onClick={() => {
                  setStatusFilter('all')
                  setActionFilter('all')
                  setSearchText('')
                }}
              >
                {t.common.reset}
              </Button>
            ) : null}
          </div>
        </div>
        {logsLoading ? (
          <div className="py-6 text-center text-sm text-muted-foreground">
            <Loader2 className="mr-2 inline-block h-4 w-4 animate-spin" />
            {t.common.loading}
          </div>
        ) : logs.length === 0 ? (
          <div className="py-6 text-center text-sm text-muted-foreground">{t.admin.pluginNoExecutions}</div>
        ) : filteredLogs.length === 0 ? (
          <div className="py-6 text-center text-sm text-muted-foreground">
            {t.admin.pluginNoExecutionsForFilter}
          </div>
        ) : (
          <div className="space-y-3">
            {filteredLogs.map((item) => (
              <Card key={item.id} className={item.success ? undefined : 'border-destructive/30'}>
                <CardContent className="space-y-4 p-4">
                  <div className="min-w-0 space-y-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <p className="font-mono text-[11px] text-muted-foreground">#{item.id}</p>
                      <p className="break-all font-mono text-sm font-semibold">{item.action}</p>
                      <Badge variant={executionStatusVariant(item.success)}>
                        {item.success ? t.common.success : t.common.failed}
                      </Badge>
                    </div>
                    <p className="text-xs text-muted-foreground">
                      {joinSummaryItems([
                        formatDateTime(item.created_at, locale),
                        t.admin.pluginExecutionDuration.replace(
                          '{duration}',
                          String(item.duration || 0)
                        ),
                        typeof item.user_id === 'number'
                          ? t.admin.pluginExecutionUserID.replace('{id}', String(item.user_id))
                          : null,
                        typeof item.order_id === 'number'
                          ? t.admin.pluginExecutionOrderID.replace('{id}', String(item.order_id))
                          : null,
                      ])}
                    </p>
                  </div>
                  <div className="grid gap-3 xl:grid-cols-3">
                    <ExecutionLogPayloadSection
                      title={t.admin.pluginExecutionSectionParams}
                      value={item.params}
                      emptyText={t.admin.pluginExecutionNoPayload}
                      onCopy={copyPayload}
                      t={t}
                    />
                    <ExecutionLogPayloadSection
                      title={t.admin.pluginExecutionSectionResult}
                      value={item.result}
                      emptyText={t.admin.pluginExecutionNoPayload}
                      onCopy={copyPayload}
                      t={t}
                    />
                    <ExecutionLogPayloadSection
                      title={t.admin.pluginExecutionSectionError}
                      value={item.error}
                      emptyText={t.admin.pluginExecutionNoPayload}
                      tone="error"
                      onCopy={copyPayload}
                      t={t}
                    />
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}
