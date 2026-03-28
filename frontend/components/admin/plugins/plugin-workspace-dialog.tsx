'use client'

import {
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent as ReactKeyboardEvent,
} from 'react'

import {
  Copy,
  Loader2,
  RefreshCw,
  RotateCcw,
  Search,
  Trash2,
  Wifi,
  WifiOff,
} from 'lucide-react'

import { useToast } from '@/hooks/use-toast'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import {
  cancelAdminPluginExecutionTask,
  claimAdminPluginWorkspaceControl,
  clearAdminPluginWorkspace,
  evaluateAdminPluginWorkspaceRuntime,
  getAdminPluginWorkspaceRuntimeState,
  inspectAdminPluginWorkspaceRuntime,
  resetAdminPluginWorkspace,
  resetAdminPluginWorkspaceRuntime,
  resolveAdminPluginWorkspaceWebSocketProtocols,
  resolveAdminPluginWorkspaceWebSocketURL,
  streamAdminPluginWorkspace,
  type AdminPlugin,
  type AdminPluginWorkspaceControlEvent,
  type AdminPluginWorkspaceEntry,
  type AdminPluginWorkspaceRuntimeState,
  type AdminPluginWorkspaceSnapshot,
  type AdminPluginWorkspaceWebSocketAck,
  type AdminPluginWorkspaceWebSocketClientFrame,
  type AdminPluginWorkspaceWebSocketServerFrame,
} from '@/lib/api'
import type { Translations } from '@/lib/i18n'
import { resolvePluginWorkspaceCompletion } from '@/lib/plugin-workspace-completion'
import {
  tryParseWorkspaceJSONOutput,
  type WorkspaceStructuredPreview,
} from '@/lib/plugin-workspace-json'
import {
  extractWorkspaceConsolePreviewsFromMetadata,
  extractWorkspaceRuntimePreviewFromMetadata,
  type WorkspaceRuntimePreview,
} from '@/lib/plugin-workspace-preview'
import {
  buildPluginWorkspaceHistoryStorageKey,
  parsePluginWorkspaceHistoryStorage,
  pushPluginWorkspaceHistoryEntry,
  resolvePluginWorkspaceHistoryNavigation,
  shouldHandlePluginWorkspaceHistoryNavigation,
} from '@/lib/plugin-workspace-history'
import {
  resolvePluginWorkspaceSubmission,
} from '@/lib/plugin-workspace-command'
import { parseWorkspaceRuntimeConsoleLine } from '@/lib/plugin-workspace-runtime'
import {
  applyWorkspaceStreamEvent,
  applyWorkspaceWebSocketFrame,
  extractWorkspaceRuntimeState,
  extractWorkspaceSnapshot,
  preferNewerWorkspaceSnapshot,
} from '@/lib/plugin-workspace-stream'
import {
  buildWorkspaceTranscriptBlocks,
  buildWorkspaceTerminalPlainText,
  buildWorkspaceTerminalScreen,
  resolveWorkspaceTranscriptKind,
  shouldInterruptWorkspaceTerminalInputShortcut,
  type WorkspaceTranscriptBlock,
  type WorkspaceTranscriptKind,
} from '@/lib/plugin-workspace-terminal'

type PluginWorkspaceDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  plugin?: AdminPlugin | null
  workspace?: AdminPluginWorkspaceSnapshot | null
  workspaceLoading: boolean
  terminalSubmitting: boolean
  signaling: boolean
  onSubmitTerminalLine: (payload: { line?: string }) => void
  onSignal: (payload: { task_id?: string; signal?: string }) => void
  onRefresh: () => void
  formatDateTime: (value?: string, locale?: string) => string
  locale: string
  t: Translations
}

type WorkspaceConnectionState = 'idle' | 'connecting' | 'live' | 'reconnecting' | 'error'
type WorkspaceTimelineFilter = 'all' | 'ownership' | 'actions'

type WorkspaceTerminalDisplayItem =
  | {
      type: 'text'
      key: string
      blocks: WorkspaceTranscriptBlock[]
    }
  | {
      type: 'json'
      key: string
      raw: string
      preview: WorkspaceStructuredPreview
      kind: WorkspaceTranscriptKind
      level?: string
    }
  | {
      type: 'console'
      key: string
      raw: string
      previews: WorkspaceRuntimePreview[]
    }
  | {
      type: 'runtime'
      key: string
      raw: string
      preview: WorkspaceRuntimePreview
      kind: WorkspaceTranscriptKind
      level?: string
      source?: string
    }

type WorkspaceCompletionOverlay = {
  token: string
  resolvedToken: string
  suggestions: string[]
}

function formatWorkspaceTerminalTimestamp(value?: string, locale?: string): string {
  const raw = String(value || '').trim()
  if (!raw) {
    return '--:--:--'
  }
  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) {
    return raw
  }
  try {
    return new Intl.DateTimeFormat(locale === 'zh' ? 'zh-CN' : 'en-US', {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hour12: false,
    }).format(date)
  } catch {
    return date.toISOString().slice(11, 19)
  }
}

function buildWorkspaceSearchIndex(entry: AdminPluginWorkspaceEntry): string {
  return [
    entry.seq,
    entry.timestamp,
    entry.channel,
    entry.level,
    entry.message,
    entry.source,
    entry.action,
    entry.hook,
    entry.task_id,
    ...Object.entries(entry.metadata || {}).flatMap(([key, value]) => [key, value]),
  ]
    .map((item) => String(item || '').toLowerCase())
    .join('\n')
}

function workspaceControlEventLabel(
  event: AdminPluginWorkspaceControlEvent,
  t: Translations
): string {
  const signal = String(event.signal || '')
    .trim()
    .toLowerCase()
  switch (
    String(event.type || '')
      .trim()
      .toLowerCase()
  ) {
    case 'control_assigned':
      return t.admin.pluginWorkspaceEventControlAssigned
    case 'viewer_attached':
      return t.admin.pluginWorkspaceEventViewerAttached
    case 'viewer_detached':
      return t.admin.pluginWorkspaceEventViewerDetached
    case 'control_claimed':
      return t.admin.pluginWorkspaceEventControlClaimed
    case 'control_transferred':
      return t.admin.pluginWorkspaceEventControlTransferred
    case 'control_released':
      return t.admin.pluginWorkspaceEventControlReleased
    case 'control_auto_transferred':
      return t.admin.pluginWorkspaceEventControlAutoTransferred
    case 'control_auto_released':
      return t.admin.pluginWorkspaceEventControlAutoReleased
    case 'command_started':
      return t.admin.pluginWorkspaceEventCommandStarted
    case 'command_executed':
      return t.admin.pluginWorkspaceEventCommandExecuted
    case 'command_finished':
      return t.admin.pluginWorkspaceEventCommandFinished
    case 'input_submitted':
      return t.admin.pluginWorkspaceEventInputSubmitted
    case 'workspace_reset':
      return t.admin.pluginWorkspaceEventWorkspaceReset
    case 'workspace_runtime_reset':
      return t.admin.pluginWorkspaceRuntimeRebuild
    case 'signal_sent':
      if (signal === 'interrupt') {
        return t.admin.pluginWorkspaceInterrupt
      }
      if (signal === 'terminate') {
        return t.admin.pluginWorkspaceTerminate
      }
      return t.admin.pluginWorkspaceEventSignalSent
    case 'workspace_cleared':
      return t.admin.pluginWorkspaceEventWorkspaceCleared
    default:
      return String(event.type || '').trim() || t.common.info
  }
}

function workspaceCompletionResultLabel(result: string | undefined, t: Translations): string {
  switch (
    String(result || '')
      .trim()
      .toLowerCase()
  ) {
    case 'completed':
      return t.admin.pluginWorkspaceStatusCompleted
    case 'failed':
      return t.admin.pluginWorkspaceStatusFailed
    case 'canceled':
      return t.admin.pluginWorkspaceStatusCanceled
    case 'timed_out':
      return t.admin.pluginWorkspaceStatusTimedOut
    case 'interrupted':
      return t.admin.pluginWorkspaceStatusInterrupted
    case 'terminated':
      return t.admin.pluginWorkspaceStatusTerminated
    default:
      return String(result || '').trim() || t.common.info
  }
}

function workspaceControlEventSummary(
  event: AdminPluginWorkspaceControlEvent,
  locale: string,
  t: Translations
): string {
  const actor = event.admin_id ? `#${event.admin_id}` : locale === 'zh' ? '系统' : 'System'
  const owner = event.owner_admin_id ? `#${event.owner_admin_id}` : locale === 'zh' ? '无' : 'None'
  const previousOwner = event.previous_owner_id
    ? `#${event.previous_owner_id}`
    : locale === 'zh'
      ? '无'
      : 'None'
  const signal = String(event.signal || '')
    .trim()
    .toLowerCase()
  const result = String(event.result || '')
    .trim()
    .toLowerCase()
  switch (
    String(event.type || '')
      .trim()
      .toLowerCase()
  ) {
    case 'control_assigned':
    case 'control_claimed':
      return locale === 'zh'
        ? `操作者 ${actor}，当前控制者 ${owner}`
        : `Actor ${actor}, current owner ${owner}`
    case 'viewer_attached':
    case 'viewer_detached':
      return locale === 'zh'
        ? `旁观管理员 ${actor}，当前控制者 ${owner}`
        : `Viewer ${actor}, current owner ${owner}`
    case 'control_transferred':
    case 'control_auto_transferred':
      return locale === 'zh'
        ? `从 ${previousOwner} 切换到 ${owner}`
        : `Transferred from ${previousOwner} to ${owner}`
    case 'control_released':
    case 'control_auto_released':
      return locale === 'zh' ? `前任控制者 ${previousOwner}` : `Previous owner ${previousOwner}`
    case 'command_started':
    case 'command_executed':
    case 'input_submitted':
    case 'workspace_cleared':
      return locale === 'zh' ? `操作者 ${actor}，控制者 ${owner}` : `Actor ${actor}, owner ${owner}`
    case 'command_finished': {
      const completionLabel = workspaceCompletionResultLabel(result, t)
      if (locale === 'zh') {
        return `系统确认命令已结束，结果 ${completionLabel}，控制者 ${owner}`
      }
      return `System confirmed the command finished with ${completionLabel}, owner ${owner}`
    }
    case 'workspace_reset':
      return locale === 'zh'
        ? `操作者 ${actor} 重置了当前工作台，控制者 ${owner}`
        : `Actor ${actor} reset the current workspace, owner ${owner}`
    case 'workspace_runtime_reset':
      return locale === 'zh'
        ? `操作者 ${actor} 重建了当前 JS 运行时，控制者 ${owner}`
        : `Actor ${actor} rebuilt the current JS runtime, owner ${owner}`
    case 'signal_sent':
      if (locale === 'zh') {
        if (signal === 'interrupt' && result === 'input_wait_interrupted') {
          return `操作者 ${actor} 中断了当前输入等待，控制者 ${owner}`
        }
        if (signal === 'interrupt' && result === 'already_stopped') {
          return `操作者 ${actor} 请求中断，但命令已经停止，控制者 ${owner}`
        }
        if (signal === 'terminate' && result === 'already_stopped') {
          return `操作者 ${actor} 请求终止，但命令已经停止，控制者 ${owner}`
        }
        if (signal === 'terminate') {
          return `操作者 ${actor} 请求终止当前命令，控制者 ${owner}`
        }
        if (signal === 'interrupt') {
          return `操作者 ${actor} 请求中断当前命令，控制者 ${owner}`
        }
      } else {
        if (signal === 'interrupt' && result === 'input_wait_interrupted') {
          return `Actor ${actor} interrupted the current input wait, owner ${owner}`
        }
        if (signal === 'interrupt' && result === 'already_stopped') {
          return `Actor ${actor} requested interrupt, but the command had already stopped, owner ${owner}`
        }
        if (signal === 'terminate' && result === 'already_stopped') {
          return `Actor ${actor} requested termination, but the command had already stopped, owner ${owner}`
        }
        if (signal === 'terminate') {
          return `Actor ${actor} requested termination for the active command, owner ${owner}`
        }
        if (signal === 'interrupt') {
          return `Actor ${actor} requested interrupt for the active command, owner ${owner}`
        }
      }
      return locale === 'zh' ? `操作者 ${actor}，控制者 ${owner}` : `Actor ${actor}, owner ${owner}`
    default:
      return String(event.message || '').trim()
  }
}

function isWorkspaceOwnershipEvent(event: AdminPluginWorkspaceControlEvent): boolean {
  switch (
    String(event.type || '')
      .trim()
      .toLowerCase()
  ) {
    case 'control_assigned':
    case 'viewer_attached':
    case 'viewer_detached':
    case 'control_claimed':
    case 'control_transferred':
    case 'control_released':
    case 'control_auto_transferred':
    case 'control_auto_released':
      return true
    default:
      return false
  }
}

function buildWorkspaceTerminalDisplayItems(
  blocks: WorkspaceTranscriptBlock[]
): WorkspaceTerminalDisplayItem[] {
  const items: WorkspaceTerminalDisplayItem[] = []
  let pendingTextBlocks: WorkspaceTranscriptBlock[] = []

  const flushPendingTextBlocks = () => {
    if (pendingTextBlocks.length === 0) {
      return
    }
    items.push({
      type: 'text',
      key: pendingTextBlocks.map((block) => block.key).join('|'),
      blocks: pendingTextBlocks,
    })
    pendingTextBlocks = []
  }

  for (const block of blocks) {
    const canVisualize =
      block.kind === 'stdout' || block.kind === 'stderr' || block.kind === 'log'
    const consolePreviews = canVisualize
      ? extractWorkspaceConsolePreviewsFromMetadata(block.metadata)
      : []
    if (consolePreviews.length > 0) {
      flushPendingTextBlocks()
      items.push({
        type: 'console',
        key: `console-${block.key}`,
        raw: block.text,
        previews: consolePreviews,
      })
      continue
    }
    const runtimePreview = canVisualize
      ? extractWorkspaceRuntimePreviewFromMetadata(block.metadata)
      : null
    if (runtimePreview) {
      flushPendingTextBlocks()
      items.push({
        type: 'runtime',
        key: `runtime-${block.key}`,
        raw: block.text,
        preview: runtimePreview,
        kind: block.kind,
        level: block.level,
        source: block.source,
      })
      continue
    }
    const parsed = canVisualize ? tryParseWorkspaceJSONOutput(block.text) : null
    if (!parsed) {
      pendingTextBlocks.push(block)
      continue
    }
    flushPendingTextBlocks()
    items.push({
      type: 'json',
      key: `json-${block.key}`,
      raw: parsed.raw,
      preview: parsed.preview,
      kind: block.kind,
      level: block.level,
    })
  }

  flushPendingTextBlocks()
  return items
}

function resolveWorkspaceConsoleValueClass(type: string): string {
  switch (type) {
    case 'error':
      return 'text-rose-300'
    case 'string':
      return 'text-emerald-300'
    case 'number':
      return 'text-amber-300'
    case 'boolean':
      return 'text-amber-200'
    case 'function':
      return 'text-violet-300'
    case 'null':
      return 'text-neutral-500'
    default:
      return 'text-neutral-100'
  }
}

function formatWorkspaceConsoleEntryLabel(parentType: string | undefined, key: string): string {
  if (parentType === 'array') {
    return `[${key}]`
  }
  return /^[$A-Z_a-z][$\w]*$/.test(key) ? key : JSON.stringify(key)
}

function resolveWorkspaceConsolePrefix(raw: string, source?: string): string {
  const trimmed = String(raw || '').trimStart()
  if (trimmed.startsWith('<')) {
    return '<'
  }
  const normalizedSource = String(source || '')
    .trim()
    .toLowerCase()
  if (normalizedSource.startsWith('host.workspace.runtime.')) {
    return '<'
  }
  return ''
}

function WorkspaceRuntimePreviewNode({
  preview,
  t,
  depth = 0,
  label,
  parentType,
  rootPrefix,
  defaultExpanded = false,
}: {
  preview: WorkspaceRuntimePreview
  t: Translations
  depth?: number
  label?: string
  parentType?: string
  rootPrefix?: string
  defaultExpanded?: boolean
}) {
  const expandable = preview.entries.length > 0
  const [expanded, setExpanded] = useState(defaultExpanded && expandable)
  const displayLabel = label ? formatWorkspaceConsoleEntryLabel(parentType, label) : ''

  return (
    <div className="font-mono text-[12px] leading-6 text-neutral-100">
      <div
        className="flex items-start gap-1 whitespace-pre-wrap break-words"
        style={depth > 0 ? { paddingLeft: `${depth * 16}px` } : undefined}
      >
        {expandable ? (
          <button
            type="button"
            className="inline-flex h-6 w-4 shrink-0 items-center justify-center text-neutral-500 hover:text-neutral-200"
            onClick={() => setExpanded((current) => !current)}
          >
            {expanded ? '▾' : '▸'}
          </button>
        ) : (
          <span className="inline-flex h-6 w-4 shrink-0" />
        )}
        {rootPrefix ? <span className="shrink-0 text-neutral-500">{rootPrefix}</span> : null}
        {displayLabel ? (
          <>
            <span className="break-all text-sky-300">{displayLabel}</span>
            <span className="shrink-0 text-neutral-500">:</span>
          </>
        ) : null}
        <span className={resolveWorkspaceConsoleValueClass(preview.type)}>{preview.summary}</span>
      </div>
      {expanded ? (
        <div>
          {preview.entries.map((entry) => (
            <WorkspaceRuntimePreviewNode
              key={entry.key}
              preview={entry.value}
              t={t}
              depth={depth + 1}
              label={entry.key}
              parentType={preview.type}
            />
          ))}
          {preview.truncated ? (
            <div
              className="flex items-start gap-1 whitespace-pre-wrap break-words text-neutral-500"
              style={{ paddingLeft: `${(depth + 1) * 16}px` }}
            >
              <span className="inline-flex h-6 w-4 shrink-0" />
              <span>{`… ${t.admin.pluginWorkspaceInspectorTruncated}`}</span>
            </div>
          ) : null}
        </div>
      ) : null}
    </div>
  )
}

function WorkspaceTerminalJSONCard({
  raw,
  preview,
  t,
}: {
  raw: string
  preview: WorkspaceStructuredPreview
  t: Translations
}) {
  const rootPrefix = resolveWorkspaceConsolePrefix(raw)

  return (
    <div className="py-0.5">
      <WorkspaceRuntimePreviewNode
        preview={preview}
        t={t}
        rootPrefix={rootPrefix}
        defaultExpanded={false}
      />
    </div>
  )
}

function summarizeWorkspaceConsolePreview(preview: WorkspaceRuntimePreview): string {
  if (preview.type === 'string' && typeof preview.value === 'string') {
    return preview.value
  }
  return preview.summary
}

function WorkspaceTerminalConsoleCard({
  previews,
  t,
}: {
  previews: WorkspaceRuntimePreview[]
  raw: string
  t: Translations
}) {
  if (previews.length === 1) {
    return (
      <div className="py-0.5">
        <WorkspaceRuntimePreviewNode preview={previews[0]} t={t} defaultExpanded={false} />
      </div>
    )
  }

  const rootPreview: WorkspaceRuntimePreview = {
    type: 'console',
    summary: previews.map((preview) => summarizeWorkspaceConsolePreview(preview)).join(' '),
    keys: [],
    entries: previews.map((preview, index) => ({
      key: String(index),
      value: preview,
    })),
    truncated: false,
  }

  return (
    <div className="py-0.5">
      <WorkspaceRuntimePreviewNode preview={rootPreview} t={t} defaultExpanded={false} />
    </div>
  )
}

function WorkspaceTerminalRuntimeCard({
  raw,
  preview,
  source,
  t,
}: {
  raw: string
  preview: WorkspaceRuntimePreview
  source?: string
  t: Translations
}) {
  const rootPrefix = resolveWorkspaceConsolePrefix(raw, source) || '<'

  return (
    <div className="py-0.5">
      <WorkspaceRuntimePreviewNode
        preview={preview}
        t={t}
        rootPrefix={rootPrefix}
        defaultExpanded={false}
      />
    </div>
  )
}

function isEditableWorkspaceTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) {
    return false
  }
  return Boolean(
    target.closest(
      'input, textarea, [contenteditable="true"], [contenteditable=""], [role="textbox"]'
    )
  )
}

function createWorkspaceRequestID(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  return `ws_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 10)}`
}

function createWorkspaceTaskID(): string {
  return `pex_ws_${createWorkspaceRequestID().replace(/[^a-zA-Z0-9_-]/g, '')}`
}

function isAbortError(error: unknown): boolean {
  return (
    (error instanceof DOMException && error.name === 'AbortError') ||
    (typeof error === 'object' &&
      error !== null &&
      'name' in error &&
      String((error as { name?: string }).name || '') === 'AbortError')
  )
}

function isWorkspaceCancellationErrorText(value: string): boolean {
  const normalized = String(value || '')
    .trim()
    .toLowerCase()
  if (!normalized) {
    return false
  }
  return (
    normalized.includes('context canceled') ||
    normalized.includes('context cancelled') ||
    normalized.includes('execution canceled') ||
    normalized.includes('execution cancelled')
  )
}

function workspaceRequestIDMatches(expected: string, actual?: string): boolean {
  const normalizedExpected = String(expected || '').trim()
  const normalizedActual = String(actual || '').trim()
  if (!normalizedExpected || !normalizedActual) {
    return false
  }
  return normalizedExpected === normalizedActual
}

function workspaceConnectionVariant(
  state: WorkspaceConnectionState
): 'default' | 'secondary' | 'destructive' | 'outline' | 'active' {
  switch (state) {
    case 'live':
      return 'default'
    case 'connecting':
    case 'reconnecting':
      return 'active'
    case 'error':
      return 'destructive'
    default:
      return 'outline'
  }
}

function workspaceConnectionLabel(state: WorkspaceConnectionState, t: Translations): string {
  switch (state) {
    case 'connecting':
      return t.admin.pluginWorkspaceConnecting
    case 'live':
      return t.admin.pluginWorkspaceLive
    case 'reconnecting':
      return t.admin.pluginWorkspaceReconnecting
    case 'error':
      return t.admin.pluginWorkspaceConnectionError
    default:
      return t.admin.pluginWorkspaceConnectionIdle
  }
}

function workspaceRuntimeStatusVariant(
  status?: string
): 'default' | 'secondary' | 'destructive' | 'outline' | 'active' {
  switch (
    String(status || '')
      .trim()
      .toLowerCase()
  ) {
    case 'running':
      return 'active'
    case 'waiting_input':
      return 'default'
    case 'failed':
    case 'canceled':
    case 'timed_out':
      return 'destructive'
    case 'completed':
      return 'secondary'
    default:
      return 'outline'
  }
}

function workspaceCompletionReason(
  workspace: AdminPluginWorkspaceSnapshot | null | undefined
): string {
  return String(workspace?.completion_reason || '')
    .trim()
    .toLowerCase()
}

function workspaceRuntimeStatusLabel(
  workspace: AdminPluginWorkspaceSnapshot | null | undefined,
  t: Translations
): string {
  const status = String(workspace?.status || '')
    .trim()
    .toLowerCase()
  const completionReason = workspaceCompletionReason(workspace)
  switch (status) {
    case 'running':
      return t.admin.pluginWorkspaceStatusRunning
    case 'waiting_input':
      return t.admin.pluginWorkspaceStatusWaitingInput
    case 'completed':
      return t.admin.pluginWorkspaceStatusCompleted
    case 'failed':
      return t.admin.pluginWorkspaceStatusFailed
    case 'canceled':
      if (completionReason === 'interrupted') {
        return t.admin.pluginWorkspaceStatusInterrupted
      }
      if (completionReason === 'terminated') {
        return t.admin.pluginWorkspaceStatusTerminated
      }
      return t.admin.pluginWorkspaceStatusCanceled
    case 'timed_out':
      return t.admin.pluginWorkspaceStatusTimedOut
    default:
      return t.admin.pluginWorkspaceStatusIdle
  }
}

function workspaceLastErrorText(
  workspace: AdminPluginWorkspaceSnapshot | null | undefined
): string {
  const lastError = String(workspace?.last_error || '').trim()
  if (!lastError) {
    return ''
  }
  const status = String(workspace?.status || '')
    .trim()
    .toLowerCase()
  const completionReason = workspaceCompletionReason(workspace)
  const normalizedError = lastError.toLowerCase()
  if (
    status === 'canceled' &&
    (completionReason === 'interrupted' || completionReason === 'terminated') &&
    (normalizedError.includes('context canceled') ||
      normalizedError.includes('context cancelled') ||
      normalizedError === 'canceled' ||
      normalizedError === 'cancelled')
  ) {
    return ''
  }
  return lastError
}

export function PluginWorkspaceDialog({
  open,
  onOpenChange,
  plugin,
  workspace,
  workspaceLoading,
  terminalSubmitting,
  signaling,
  onSubmitTerminalLine,
  onSignal,
  onRefresh,
  formatDateTime,
  locale,
  t,
}: PluginWorkspaceDialogProps) {
  const toast = useToast()
  const [searchText, setSearchText] = useState('')
  const [liveWorkspace, setLiveWorkspace] = useState<AdminPluginWorkspaceSnapshot | null>(
    workspace || null
  )
  const [connectionState, setConnectionState] = useState<WorkspaceConnectionState>('idle')
  const [streamError, setStreamError] = useState('')
  const [clearing, setClearing] = useState(false)
  const [resetting, setResetting] = useState(false)
  const [claimingControl, setClaimingControl] = useState(false)
  const [showTerminalSearch, setShowTerminalSearch] = useState(false)
  const [showActivityPanel, setShowActivityPanel] = useState(false)
  const [showRuntimePanel, setShowRuntimePanel] = useState(false)
  const [followTerminalOutput, setFollowTerminalOutput] = useState(true)
  const [pendingTerminalOutput, setPendingTerminalOutput] = useState(false)
  const [timelineFilter, setTimelineFilter] = useState<WorkspaceTimelineFilter>('all')
  const [commandLineText, setCommandLineText] = useState('')
  const [commandHistory, setCommandHistory] = useState<string[]>([])
  const [commandHistoryIndex, setCommandHistoryIndex] = useState(-1)
  const [commandHistoryDraft, setCommandHistoryDraft] = useState('')
  const [, setCompletionStatus] = useState('')
  const [completionSelectedIndex, setCompletionSelectedIndex] = useState(0)
  const [completionOverlay, setCompletionOverlay] = useState<WorkspaceCompletionOverlay | null>(
    null
  )
  const [runtimeState, setRuntimeState] = useState<AdminPluginWorkspaceRuntimeState | null>(null)
  const [runtimeStateLoading, setRuntimeStateLoading] = useState(false)
  const [runtimeStateResetting, setRuntimeStateResetting] = useState(false)
  const [runtimeStateError, setRuntimeStateError] = useState('')
  const [runtimeCommandPending, setRuntimeCommandPending] = useState(false)
  const [pendingRuntimeTaskId, setPendingRuntimeTaskId] = useState('')
  const [socketInputPending, setSocketInputPending] = useState(false)
  const [socketSignalPending, setSocketSignalPending] = useState<'interrupt' | 'terminate' | null>(
    null
  )
  const [runtimeSignalPending, setRuntimeSignalPending] = useState<'interrupt' | 'terminate' | null>(
    null
  )
  const workspaceSocketRef = useRef<WebSocket | null>(null)
  const shellInputRef = useRef<HTMLTextAreaElement | null>(null)
  const pendingShellSelectionRef = useRef<{ start: number; end: number } | null>(null)
  const searchInputRef = useRef<HTMLInputElement | null>(null)
  const terminalViewportRef = useRef<HTMLDivElement | null>(null)
  const terminalContentRef = useRef<HTMLDivElement | null>(null)
  const terminalScrollFrameRef = useRef<number | null>(null)
  const followTerminalOutputRef = useRef(true)
  const terminalUserScrollIntentRef = useRef(0)
  const terminalLastSeqRef = useRef(0)
  const pendingTerminalRequestIDRef = useRef('')
  const pendingTerminalLineRef = useRef('')
  const pendingTerminalActionRef = useRef<
    'terminal_line' | 'runtime_eval' | 'runtime_inspect' | ''
  >('')
  const pendingRuntimeTaskIdRef = useRef('')
  const runtimeCommandPendingRef = useRef(false)
  const runtimeSignalPendingRef = useRef<'interrupt' | 'terminate' | null>(null)
  const pendingSignalRequestRef = useRef<{ id: string; signal: 'interrupt' | 'terminate' } | null>(
    null
  )
  const workspaceHistoryStorageKey = useMemo(
    () => buildPluginWorkspaceHistoryStorageKey(plugin?.id),
    [plugin?.id]
  )

  const clearSocketInputPending = () => {
    pendingTerminalRequestIDRef.current = ''
    pendingTerminalLineRef.current = ''
    pendingTerminalActionRef.current = ''
    setSocketInputPending(false)
  }

  const clearSocketSignalPending = () => {
    pendingSignalRequestRef.current = null
    setSocketSignalPending(null)
  }

  const clearPendingRuntimeTask = () => {
    pendingRuntimeTaskIdRef.current = ''
    setPendingRuntimeTaskId('')
  }

  const setPendingRuntimeTask = (taskId: string) => {
    const normalizedTaskId = String(taskId || '').trim()
    pendingRuntimeTaskIdRef.current = normalizedTaskId
    setPendingRuntimeTaskId(normalizedTaskId)
  }

  const setRuntimeCommandPendingState = (next: boolean) => {
    runtimeCommandPendingRef.current = next
    setRuntimeCommandPending(next)
  }

  const setRuntimeSignalPendingState = (next: 'interrupt' | 'terminate' | null) => {
    runtimeSignalPendingRef.current = next
    setRuntimeSignalPending(next)
  }

  const hasPendingTerminalSubmission = () =>
    terminalSubmitting ||
    socketInputPending ||
    runtimeCommandPendingRef.current ||
    pendingTerminalRequestIDRef.current.trim().length > 0

  const setFollowTerminalOutputState = (next: boolean) => {
    followTerminalOutputRef.current = next
    setFollowTerminalOutput(next)
  }

  const markTerminalUserScrollIntent = () => {
    terminalUserScrollIntentRef.current = Date.now()
  }

  const hasRecentTerminalUserScrollIntent = () =>
    Date.now() - terminalUserScrollIntentRef.current <= 400

  const isViewportNearBottom = (viewport: HTMLDivElement | null) => {
    if (!viewport) {
      return true
    }
    const remaining = viewport.scrollHeight - viewport.scrollTop - viewport.clientHeight
    return remaining <= 96
  }

  const cancelScheduledTerminalScroll = () => {
    if (typeof window === 'undefined') {
      return
    }
    if (terminalScrollFrameRef.current !== null) {
      window.cancelAnimationFrame(terminalScrollFrameRef.current)
      terminalScrollFrameRef.current = null
    }
  }

  const scrollTerminalToBottom = (options?: {
    preserveFollow?: boolean
    frameCount?: number
  }) => {
    if (typeof window === 'undefined') {
      return
    }
    cancelScheduledTerminalScroll()
    const preserveFollow = options?.preserveFollow ?? false
    let remainingFrames = Math.max(1, options?.frameCount ?? 2)
    const flushScroll = () => {
      const viewport = terminalViewportRef.current
      if (!viewport) {
        terminalScrollFrameRef.current = null
        return
      }
      viewport.scrollTop = viewport.scrollHeight
      remainingFrames -= 1
      if (remainingFrames > 0) {
        terminalScrollFrameRef.current = window.requestAnimationFrame(flushScroll)
        return
      }
      terminalScrollFrameRef.current = null
    }
    terminalScrollFrameRef.current = window.requestAnimationFrame(flushScroll)
    if (!preserveFollow) {
      setFollowTerminalOutputState(true)
    }
    setPendingTerminalOutput(false)
  }

  const setSocketInputPendingRequest = (
    requestID: string,
    line: string,
    action: 'terminal_line' | 'runtime_eval' | 'runtime_inspect'
  ) => {
    pendingTerminalRequestIDRef.current = requestID
    pendingTerminalLineRef.current = line
    pendingTerminalActionRef.current = action
    setSocketInputPending(true)
  }

  const setSocketSignalPendingRequest = (requestID: string, signal: 'interrupt' | 'terminate') => {
    pendingSignalRequestRef.current = { id: requestID, signal }
    setSocketSignalPending(signal)
  }

  const handleWorkspaceSocketAck = (ack: AdminPluginWorkspaceWebSocketAck | undefined) => {
    if (!ack) {
      return
    }
    if (ack.workspace) {
      setLiveWorkspace((current) => preferNewerWorkspaceSnapshot(current, ack.workspace || null))
    }
    const action = String(ack.action || '')
      .trim()
      .toLowerCase()
    if (action === 'terminal_line' || action === 'runtime_eval' || action === 'runtime_inspect') {
      const nextRuntimeState = extractWorkspaceRuntimeState(ack)
      if (nextRuntimeState) {
        setRuntimeState(nextRuntimeState)
        setRuntimeStateError('')
      }
      if (
        pendingTerminalRequestIDRef.current &&
        workspaceRequestIDMatches(pendingTerminalRequestIDRef.current, ack.request_id) &&
        action === pendingTerminalActionRef.current
      ) {
        const wasRuntimeSignalPending = Boolean(runtimeSignalPendingRef.current)
        const submittedLine = pendingTerminalLineRef.current
        clearSocketInputPending()
        if (action === 'runtime_eval' || action === 'runtime_inspect') {
          clearPendingRuntimeTask()
          setRuntimeSignalPendingState(null)
        }
        if (!ack.success) {
          if (submittedLine) {
            setCommandLineText((current) => (current ? current : submittedLine))
          }
          const errorText = String(ack.error || t.admin.pluginWorkspaceStreamError)
          setStreamError(errorText)
          if (!(wasRuntimeSignalPending && isWorkspaceCancellationErrorText(errorText))) {
            toast.error(errorText)
          }
          return
        }
        setStreamError('')
      }
      return
    }
    if (action === 'signal') {
      const pendingSignal = pendingSignalRequestRef.current
      if (pendingSignal && workspaceRequestIDMatches(pendingSignal.id, ack.request_id)) {
        clearSocketSignalPending()
        if (!ack.success) {
          const errorText = String(ack.error || t.admin.pluginWorkspaceStreamError)
          setStreamError(errorText)
          toast.error(errorText)
          return
        }
        setStreamError('')
      }
    }
  }

  const sendWorkspaceSocketFrame = (frame: AdminPluginWorkspaceWebSocketClientFrame): boolean => {
    const socket = workspaceSocketRef.current
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      return false
    }
    try {
      socket.send(JSON.stringify(frame))
      return true
    } catch (error: any) {
      setStreamError(String(error?.message || t.admin.pluginWorkspaceStreamError))
      return false
    }
  }

  useEffect(() => {
    if (open) {
      setSearchText('')
      setTimelineFilter('all')
      setShowTerminalSearch(false)
      setShowActivityPanel(false)
      setShowRuntimePanel(false)
      setCompletionStatus('')
      setCompletionOverlay(null)
      setFollowTerminalOutputState(true)
      setPendingTerminalOutput(false)
      setRuntimeCommandPendingState(false)
      terminalUserScrollIntentRef.current = 0
      terminalLastSeqRef.current = 0
    }
  }, [open])

  useEffect(() => {
    if (!open) {
      return
    }
    setCommandLineText('')
    setCommandHistoryIndex(-1)
    setCommandHistoryDraft('')
    setCompletionStatus('')
    setCompletionOverlay(null)
    setRuntimeState(null)
    setRuntimeStateError('')
    clearSocketInputPending()
    clearSocketSignalPending()
    clearPendingRuntimeTask()
    setRuntimeSignalPendingState(null)
  }, [open, plugin?.id])

  useEffect(() => {
    setCompletionSelectedIndex(0)
  }, [completionOverlay])

  useEffect(() => {
    if (typeof window === 'undefined') {
      pendingShellSelectionRef.current = null
      return
    }
    const selection = pendingShellSelectionRef.current
    if (!selection) {
      return
    }
    pendingShellSelectionRef.current = null
    window.requestAnimationFrame(() => {
      const input = shellInputRef.current
      if (!input) {
        return
      }
      input.focus()
      input.setSelectionRange(selection.start, selection.end)
    })
  }, [commandLineText])

  useEffect(() => {
    const input = shellInputRef.current
    if (!input) {
      return
    }
    input.style.height = '0px'
    const nextHeight = Math.min(Math.max(input.scrollHeight, 24), 168)
    input.style.height = `${nextHeight}px`
    input.style.overflowY = input.scrollHeight > nextHeight ? 'auto' : 'hidden'
  }, [commandLineText, open])

  useEffect(() => {
    if (typeof window === 'undefined') {
      return
    }
    if (!workspaceHistoryStorageKey) {
      setCommandHistory([])
      setCommandHistoryIndex(-1)
      setCommandHistoryDraft('')
      return
    }
    setCommandHistory(
      parsePluginWorkspaceHistoryStorage(window.localStorage.getItem(workspaceHistoryStorageKey))
    )
    setCommandHistoryIndex(-1)
    setCommandHistoryDraft('')
  }, [workspaceHistoryStorageKey])

  useEffect(() => {
    if (typeof window === 'undefined' || !workspaceHistoryStorageKey) {
      return
    }
    if (commandHistory.length === 0) {
      window.localStorage.removeItem(workspaceHistoryStorageKey)
      return
    }
    window.localStorage.setItem(workspaceHistoryStorageKey, JSON.stringify(commandHistory))
  }, [workspaceHistoryStorageKey, commandHistory])

  useEffect(() => {
    if (!open) {
      setConnectionState('idle')
      setStreamError('')
      setRuntimeCommandPendingState(false)
      clearSocketInputPending()
      clearSocketSignalPending()
      cancelScheduledTerminalScroll()
      return
    }
    setLiveWorkspace(workspace || null)
  }, [open, plugin?.id])

  useEffect(() => {
    if (!open || !workspace) return
    setLiveWorkspace((current) => {
      return preferNewerWorkspaceSnapshot(current, workspace)
    })
  }, [open, workspace])

  useEffect(() => {
    if (searchText.trim()) {
      setShowTerminalSearch(true)
    }
  }, [searchText])

  useEffect(() => {
    if (!open || !plugin?.id) {
      return
    }
    const controller = new AbortController()
    let closed = false
    let preferWebSocket = typeof window !== 'undefined' && typeof window.WebSocket !== 'undefined'
    workspaceSocketRef.current = null

    const sleep = async (ms: number) => {
      await new Promise<void>((resolve) => {
        const timer = window.setTimeout(() => {
          window.clearTimeout(timer)
          resolve()
        }, ms)
      })
    }

    const connectWebSocket = async (): Promise<'closed' | 'fallback'> => {
      return await new Promise<'closed' | 'fallback'>((resolve) => {
        const url = resolveAdminPluginWorkspaceWebSocketURL(plugin.id, {
          limit: 200,
          locale,
        })
        const socket = new WebSocket(url, resolveAdminPluginWorkspaceWebSocketProtocols())
        let opened = false
        let settled = false

        const settle = (mode: 'closed' | 'fallback') => {
          if (settled) return
          settled = true
          if (workspaceSocketRef.current === socket) {
            workspaceSocketRef.current = null
          }
          resolve(mode)
        }

        socket.addEventListener('open', () => {
          if (closed) {
            socket.close(1000, 'workspace dialog closed')
            return
          }
          opened = true
          workspaceSocketRef.current = socket
          setConnectionState('live')
          setStreamError('')
        })

        socket.addEventListener('message', (messageEvent) => {
          if (closed) return
          try {
            const frame = JSON.parse(
              String(messageEvent.data || '{}')
            ) as AdminPluginWorkspaceWebSocketServerFrame
            if (frame.type === 'workspace_request_ack') {
              handleWorkspaceSocketAck(frame.ack)
              return
            }
            if (frame.type === 'workspace_error') {
              clearSocketInputPending()
              clearSocketSignalPending()
              setStreamError(String(frame.message || t.admin.pluginWorkspaceStreamError))
              return
            }
            if (frame.event) {
              setConnectionState('live')
              setLiveWorkspace((current) => applyWorkspaceWebSocketFrame(current, frame))
            }
          } catch (error: any) {
            clearSocketInputPending()
            clearSocketSignalPending()
            setStreamError(String(error?.message || t.admin.pluginWorkspaceStreamError))
          }
        })

        socket.addEventListener('error', () => {
          clearSocketInputPending()
          clearSocketSignalPending()
          if (!opened) {
            settle('fallback')
          }
        })

        socket.addEventListener('close', () => {
          if (closed) {
            settle('closed')
            return
          }
          settle(opened ? 'closed' : 'fallback')
        })
      })
    }

    const run = async () => {
      let firstAttempt = true
      while (!closed) {
        setConnectionState(firstAttempt ? 'connecting' : 'reconnecting')
        setStreamError('')
        if (preferWebSocket) {
          const mode = await connectWebSocket()
          if (closed || controller.signal.aborted) {
            return
          }
          if (mode === 'fallback') {
            preferWebSocket = false
            continue
          }
          setConnectionState('reconnecting')
          await sleep(900)
          firstAttempt = false
          continue
        }
        try {
          await streamAdminPluginWorkspace(plugin.id, {
            limit: 200,
            signal: controller.signal,
            locale,
            onEvent: async (event) => {
              if (closed) return
              setConnectionState('live')
              clearSocketInputPending()
              clearSocketSignalPending()
              setLiveWorkspace((current) => applyWorkspaceStreamEvent(current, event))
            },
          })
          if (closed || controller.signal.aborted) {
            return
          }
          setConnectionState('reconnecting')
          await sleep(900)
        } catch (error: any) {
          clearSocketInputPending()
          clearSocketSignalPending()
          if (closed || controller.signal.aborted || isAbortError(error)) {
            return
          }
          setConnectionState('error')
          setStreamError(String(error?.message || t.admin.pluginWorkspaceStreamError))
          await sleep(1500)
        }
        firstAttempt = false
      }
    }

    void run()
    return () => {
      closed = true
      cancelScheduledTerminalScroll()
      const socket = workspaceSocketRef.current
      workspaceSocketRef.current = null
      if (socket && socket.readyState === WebSocket.OPEN) {
        socket.close(1000, 'workspace dialog closed')
      } else if (socket && socket.readyState === WebSocket.CONNECTING) {
        socket.close()
      }
      clearSocketInputPending()
      clearSocketSignalPending()
      controller.abort()
    }
  }, [open, plugin?.id, locale, t.admin.pluginWorkspaceStreamError])

  const normalizedSearchText = searchText.trim().toLowerCase()
  const effectiveWorkspace = liveWorkspace || workspace || null
  const activeTaskId = String(effectiveWorkspace?.active_task_id || '').trim()
  const cancelableTaskId = activeTaskId || pendingRuntimeTaskId
  const hasCancelableTask = Boolean(cancelableTaskId)
  const activeStatus = String(effectiveWorkspace?.status || '').trim()
  const activeLastError = workspaceLastErrorText(effectiveWorkspace)
  const activePrompt = String(effectiveWorkspace?.prompt || '').trim()
  const activeTerminalSession =
    Boolean(activeTaskId) && (activeStatus === 'running' || activeStatus === 'waiting_input')
  const ownerAdminId = Number(effectiveWorkspace?.owner_admin_id || 0) || 0
  const viewerCount = Number(effectiveWorkspace?.viewer_count || 0) || 0
  const controlGranted = Boolean(effectiveWorkspace?.control_granted)
  const effectiveInputSubmitting = terminalSubmitting || socketInputPending || runtimeCommandPending
  const interruptPending =
    signaling || socketSignalPending === 'interrupt' || runtimeSignalPending === 'interrupt'
  const terminatePending =
    signaling || socketSignalPending === 'terminate' || runtimeSignalPending === 'terminate'
  const effectiveSignaling = interruptPending || terminatePending
  const ownerLabel = ownerAdminId
    ? `#${ownerAdminId}`
    : locale === 'zh'
      ? '未知管理员'
      : 'Unknown admin'
  const controlStatusMessage = controlGranted
    ? t.admin.pluginWorkspaceControlHint
    : t.admin.pluginWorkspaceReadOnlyHint.replace('{owner}', ownerLabel)
  const recentControlEvents = useMemo(
    () => effectiveWorkspace?.recent_control_events || [],
    [effectiveWorkspace?.recent_control_events]
  )
  const canClaimControl =
    open && !!plugin?.id && !controlGranted && connectionState === 'live' && !claimingControl
  const entries = useMemo(() => effectiveWorkspace?.entries || [], [effectiveWorkspace?.entries])
  const filteredControlEvents = useMemo(() => {
    const sorted = recentControlEvents
      .slice()
      .sort((left, right) => Number(right.seq || 0) - Number(left.seq || 0))
    if (timelineFilter === 'ownership') {
      return sorted.filter((event) => isWorkspaceOwnershipEvent(event))
    }
    if (timelineFilter === 'actions') {
      return sorted.filter((event) => !isWorkspaceOwnershipEvent(event))
    }
    return sorted
  }, [recentControlEvents, timelineFilter])
  const terminalSourceEntries = useMemo(
    () => entries.filter((entry) => resolveWorkspaceTranscriptKind(entry) !== 'system'),
    [entries]
  )
  const filteredEntries = useMemo(
    () =>
      entries.filter((entry) => {
        if (!normalizedSearchText) return true
        return buildWorkspaceSearchIndex(entry).includes(normalizedSearchText)
      }),
    [entries, normalizedSearchText]
  )
  const terminalEntries = useMemo(
    () => filteredEntries.filter((entry) => resolveWorkspaceTranscriptKind(entry) !== 'system'),
    [filteredEntries]
  )
  const hasTerminalTranscript = terminalSourceEntries.length > 0
  const searchSummaryLabel = normalizedSearchText
    ? t.admin.pluginWorkspaceSearchSummary
        .replace('{matched}', String(terminalEntries.length))
        .replace('{total}', String(terminalSourceEntries.length))
    : ''
  const copyButtonLabel = normalizedSearchText
    ? t.admin.pluginWorkspaceCopyFiltered
    : t.admin.pluginWorkspaceCopyAll
  const showWorkspaceLoadingOverlay = workspaceLoading && !effectiveWorkspace
  const transcriptBlocks = useMemo(
    () => buildWorkspaceTranscriptBlocks(terminalEntries),
    [terminalEntries]
  )
  const terminalDisplayItems = useMemo(
    () => buildWorkspaceTerminalDisplayItems(transcriptBlocks),
    [transcriptBlocks]
  )
  const terminalCopyText = useMemo(
    () => buildWorkspaceTerminalPlainText(transcriptBlocks),
    [transcriptBlocks]
  )
  const runtimeConsoleCommand = useMemo(
    () => parseWorkspaceRuntimeConsoleLine(commandLineText),
    [commandLineText]
  )
  const workspaceCompletionPaths = useMemo(
    () => (Array.isArray(runtimeState?.completion_paths) ? runtimeState.completion_paths : []),
    [runtimeState?.completion_paths]
  )
  const terminalInputValue = commandLineText
  const terminalSubmitBusy = effectiveInputSubmitting
  const terminalPlaceholder = !controlGranted
    ? t.admin.pluginWorkspaceTerminalReadonlyPlaceholder
    : activeTerminalSession
      ? activePrompt || t.admin.pluginWorkspaceTerminalWaitingPlaceholder
      : t.admin.pluginWorkspaceJSTerminalPlaceholder
  const terminalStatusClass = !controlGranted
    ? 'text-amber-300'
    : streamError || activeLastError
      ? 'text-rose-300'
      : activeTerminalSession
        ? 'text-amber-300'
        : 'text-emerald-300'
  const terminalActionButtonClass =
    'h-8 rounded-sm border border-white/12 bg-transparent px-2 text-neutral-300 shadow-none hover:border-white/20 hover:bg-white/[0.06] hover:text-white disabled:border-white/8'
  const terminalPromptLabel = activeTerminalSession ? activePrompt || '>' : '>'
  const terminalPromptClass = activeTerminalSession ? 'text-amber-300' : 'text-sky-400'
  const terminalFollowStatus =
    !followTerminalOutput && !normalizedSearchText ? t.admin.pluginWorkspaceOutputPaused : ''
  const runtimeExists = Boolean(runtimeState?.exists)
  const runtimeBusy = Boolean(runtimeState?.busy)
  const runtimeLoaded = Boolean(runtimeState?.loaded)
  const runtimeStatusTone = runtimeStateError
    ? 'text-rose-300'
    : runtimeBusy
      ? 'text-amber-300'
      : runtimeExists && runtimeLoaded
        ? 'text-emerald-300'
        : 'text-neutral-400'
  const runtimeStatusLabel = runtimeStateError
    ? runtimeStateError
    : !runtimeState?.available
      ? t.admin.pluginWorkspaceRuntimeUnavailable
      : !runtimeExists
        ? t.admin.pluginWorkspaceRuntimeNotStarted
        : runtimeBusy
          ? t.admin.pluginWorkspaceRuntimeBusy
          : runtimeLoaded
            ? t.admin.pluginWorkspaceRuntimeLoaded
            : t.admin.pluginWorkspaceRuntimeCold
  const terminalHelperMessage = [t.admin.pluginWorkspaceJSHint, runtimeStatusLabel]
    .filter((item) => String(item || '').trim())
    .join(' ')
  const terminalStatusMessage = !controlGranted
    ? controlStatusMessage
    : streamError
      ? streamError
      : activeLastError
        ? `${t.common.error}: ${activeLastError}`
        : activeTerminalSession
          ? activePrompt || t.admin.pluginWorkspaceWaitingHint
          : commandLineText.trim()
            ? runtimeConsoleCommand.action === 'inspect'
              ? t.admin.pluginWorkspaceJSInspectHint.replace(
                  '{depth}',
                  String(runtimeConsoleCommand.depth)
                )
              : t.admin.pluginWorkspaceJSHint
            : terminalHelperMessage
  const completionSuggestions = completionOverlay?.suggestions || []
  const completionVisibleStart =
    completionSuggestions.length <= 8
      ? 0
      : Math.max(
          0,
          Math.min(completionSelectedIndex - 3, completionSuggestions.length - 8)
        )
  const completionVisibleSuggestions = completionSuggestions.slice(
    completionVisibleStart,
    completionVisibleStart + 8
  )
  const completionHiddenBefore = completionVisibleStart
  const completionHiddenAfter = Math.max(
    0,
    completionSuggestions.length -
      (completionVisibleStart + completionVisibleSuggestions.length)
  )

  const copyAll = async () => {
    if (!terminalCopyText) return
    try {
      await navigator.clipboard.writeText(terminalCopyText)
      toast.success(t.common.copiedToClipboard)
    } catch {
      toast.error(locale === 'zh' ? '复制失败' : 'Copy failed')
    }
  }

  const clearWorkspace = async () => {
    if (!plugin?.id || clearing) return
    if (!controlGranted) {
      toast.error(controlStatusMessage)
      return
    }
    setClearing(true)
    try {
      const response = await clearAdminPluginWorkspace(plugin.id)
      const next = extractWorkspaceSnapshot(response)
      if (next) {
        setLiveWorkspace((current) => preferNewerWorkspaceSnapshot(current, next))
      }
      scrollTerminalToBottom()
      onRefresh()
      toast.success(t.admin.pluginWorkspaceClearSuccess)
    } catch (error: any) {
      toast.error(String(error?.message || t.admin.pluginWorkspaceStreamError))
    } finally {
      setClearing(false)
    }
  }

  const resetWorkspace = async () => {
    if (!plugin?.id || resetting) return
    if (!controlGranted) {
      toast.error(controlStatusMessage)
      return
    }
    setResetting(true)
    try {
      const response = await resetAdminPluginWorkspace(plugin.id)
      const next = extractWorkspaceSnapshot(response)
      if (next) {
        setLiveWorkspace((current) => preferNewerWorkspaceSnapshot(current, next))
      }
      scrollTerminalToBottom()
      onRefresh()
      toast.success(t.admin.pluginWorkspaceResetSuccess)
    } catch (error: any) {
      toast.error(String(error?.message || t.admin.pluginWorkspaceStreamError))
    } finally {
      setResetting(false)
    }
  }

  const claimWorkspaceControl = async () => {
    if (!plugin?.id || claimingControl || controlGranted) return
    setClaimingControl(true)
    try {
      const response = await claimAdminPluginWorkspaceControl(plugin.id)
      const next = extractWorkspaceSnapshot(response)
      if (next) {
        setLiveWorkspace((current) => preferNewerWorkspaceSnapshot(current, next))
      }
      onRefresh()
      toast.success(t.admin.pluginWorkspaceClaimSuccess)
    } catch (error: any) {
      toast.error(String(error?.message || t.admin.pluginWorkspaceClaimFailed))
    } finally {
      setClaimingControl(false)
    }
  }

  const focusShellInput = () => {
    if (typeof window === 'undefined') {
      return
    }
    window.requestAnimationFrame(() => {
      const input = shellInputRef.current
      if (!input) {
        return
      }
      input.scrollIntoView({ block: 'nearest' })
      input.focus()
      const length = input.value.length
      input.setSelectionRange(length, length)
    })
  }

  const replaceCommandLineText = (
    value: string,
    options?: {
      preserveCompletionStatus?: boolean
      selection?: { start: number; end: number }
    }
  ) => {
    if (!options?.preserveCompletionStatus) {
      setCompletionStatus('')
      setCompletionOverlay(null)
    }
    pendingShellSelectionRef.current = options?.selection || null
    setCommandLineText(value)
  }

  const rememberSubmittedCommandLine = (value: string) => {
    const normalized = String(value || '').trim()
    if (!normalized) {
      setCommandHistoryIndex(-1)
      setCommandHistoryDraft('')
      return
    }
    setCommandHistory((current) => pushPluginWorkspaceHistoryEntry(current, normalized))
    setCommandHistoryIndex(-1)
    setCommandHistoryDraft('')
  }

  const navigateCommandHistory = (direction: 'previous' | 'next') => {
    if (activeTerminalSession || commandHistory.length === 0) {
      return
    }
    const next = resolvePluginWorkspaceHistoryNavigation({
      entries: commandHistory,
      currentValue: commandLineText,
      index: commandHistoryIndex,
      draft: commandHistoryDraft,
      direction,
    })
    if (next.index === commandHistoryIndex && next.value === commandLineText) {
      return
    }
    setCommandHistoryIndex(next.index)
    setCommandHistoryDraft(next.draft)
    replaceCommandLineText(next.value, {
      selection: {
        start: next.value.length,
        end: next.value.length,
      },
    })
  }

  const applyCompletionSuggestion = (suggestion: string) => {
    const normalizedSuggestion = String(suggestion || '').trim()
    if (!normalizedSuggestion) {
      return
    }
    const input = shellInputRef.current
    const selectionStart = input?.selectionStart ?? commandLineText.length
    const selectionEnd = input?.selectionEnd ?? selectionStart
    const token = String(completionOverlay?.resolvedToken || completionOverlay?.token || '').trim()
    const tokenLength = token.length
    const replaceStart = tokenLength > 0 ? Math.max(0, selectionStart - tokenLength) : selectionStart
    const nextValue = `${commandLineText.slice(0, replaceStart)}${normalizedSuggestion}${commandLineText.slice(selectionEnd)}`
    setCompletionStatus('')
    setCompletionOverlay(null)
    replaceCommandLineText(nextValue, {
      preserveCompletionStatus: true,
      selection: {
        start: replaceStart + normalizedSuggestion.length,
        end: replaceStart + normalizedSuggestion.length,
      },
    })
  }

  const moveCompletionSelection = (direction: 'previous' | 'next') => {
    if (!completionOverlay?.suggestions.length) {
      return
    }
    setCompletionSelectedIndex((current) => {
      const total = completionOverlay.suggestions.length
      if (direction === 'next') {
        return current >= total - 1 ? 0 : current + 1
      }
      return current <= 0 ? total - 1 : current - 1
    })
  }

  const fetchRuntimeState = async (options?: { silent?: boolean }) => {
    const pluginID = plugin?.id
    if (!pluginID) {
      return
    }
    const shouldShowLoading = !options?.silent || !runtimeState
    if (shouldShowLoading) {
      setRuntimeStateLoading(true)
    }
    try {
      const response = await getAdminPluginWorkspaceRuntimeState(pluginID)
      const nextState = extractWorkspaceRuntimeState(response)
      setRuntimeState(nextState)
      setRuntimeStateError('')
    } catch (error: any) {
      const message = String(error?.message || t.admin.pluginWorkspaceStreamError)
      setRuntimeStateError(message)
      if (!options?.silent) {
        toast.error(message)
      }
    } finally {
      if (shouldShowLoading) {
        setRuntimeStateLoading(false)
      }
    }
  }

  const rebuildRuntime = async () => {
    const pluginID = plugin?.id
    if (!pluginID || runtimeStateResetting) {
      return
    }
    if (!controlGranted) {
      toast.error(controlStatusMessage)
      return
    }
    setRuntimeStateResetting(true)
    try {
      const response = await resetAdminPluginWorkspaceRuntime(pluginID)
      const nextWorkspace = extractWorkspaceSnapshot(response)
      if (nextWorkspace) {
        setLiveWorkspace((current) => preferNewerWorkspaceSnapshot(current, nextWorkspace))
      }
      const nextState = extractWorkspaceRuntimeState(response)
      setRuntimeState(nextState)
      setRuntimeStateError('')
      toast.success(t.admin.pluginWorkspaceRuntimeRebuildSuccess)
      onRefresh()
    } catch (error: any) {
      const message = String(error?.message || t.admin.pluginWorkspaceStreamError)
      setRuntimeStateError(message)
      toast.error(message)
    } finally {
      setRuntimeStateResetting(false)
    }
  }

  const submitTerminalLine = async () => {
    setCompletionStatus('')
    if (!controlGranted) {
      toast.error(controlStatusMessage)
      return
    }
    const pluginID = plugin?.id
    if (!pluginID) {
      toast.error(t.admin.pluginWorkspaceStreamError)
      return
    }
    if (activeTerminalSession) {
      if (hasPendingTerminalSubmission()) return
      const value = commandLineText
      setFollowTerminalOutputState(true)
      setPendingTerminalOutput(false)
      const requestID = createWorkspaceRequestID()
      if (
        sendWorkspaceSocketFrame({
          type: 'terminal_line',
          request_id: requestID,
          line: value,
        })
      ) {
        setSocketInputPendingRequest(requestID, value, 'terminal_line')
        setStreamError('')
        setCommandLineText('')
        return
      }
      onSubmitTerminalLine({
        line: value,
      })
      setCommandLineText('')
      return
    }

    const runtimeLine = commandLineText
    const resolvedSubmission = resolvePluginWorkspaceSubmission(
      runtimeLine,
      plugin?.workspace_commands
    )
    if (hasPendingTerminalSubmission() || resolvedSubmission.mode === 'noop') {
      return
    }
    rememberSubmittedCommandLine(runtimeLine)
    setFollowTerminalOutputState(true)
    setPendingTerminalOutput(false)

    if (resolvedSubmission.mode === 'terminal_line') {
      const requestID = createWorkspaceRequestID()
      if (
        sendWorkspaceSocketFrame({
          type: 'terminal_line',
          request_id: requestID,
          line: resolvedSubmission.line,
        })
      ) {
        setSocketInputPendingRequest(requestID, runtimeLine, 'terminal_line')
        setStreamError('')
        setCommandLineText('')
        return
      }
      onSubmitTerminalLine({
        line: resolvedSubmission.line,
      })
      setCommandLineText('')
      return
    }

    const requestID = createWorkspaceRequestID()
    const requestAction: 'runtime_eval' | 'runtime_inspect' =
      resolvedSubmission.mode === 'runtime_inspect' ? 'runtime_inspect' : 'runtime_eval'
    const runtimeTaskID = createWorkspaceTaskID()
    setPendingRuntimeTask(runtimeTaskID)
    if (
      sendWorkspaceSocketFrame({
        type: requestAction,
        request_id: requestID,
        task_id: runtimeTaskID,
        line: resolvedSubmission.line,
        depth: resolvedSubmission.mode === 'runtime_inspect' ? resolvedSubmission.depth : undefined,
      })
    ) {
      setSocketInputPendingRequest(requestID, runtimeLine, requestAction)
      setStreamError('')
      setCommandLineText('')
      return
    }
    setRuntimeCommandPendingState(true)
    try {
      const response =
        resolvedSubmission.mode === 'runtime_inspect'
          ? await inspectAdminPluginWorkspaceRuntime(pluginID, {
              line: resolvedSubmission.line,
              depth: resolvedSubmission.depth,
              task_id: runtimeTaskID,
            })
          : await evaluateAdminPluginWorkspaceRuntime(pluginID, {
              line: resolvedSubmission.line,
              task_id: runtimeTaskID,
            })
      const next = extractWorkspaceSnapshot(response)
      if (next) {
        setLiveWorkspace((current) => preferNewerWorkspaceSnapshot(current, next))
      }
      const nextRuntimeState = extractWorkspaceRuntimeState(response)
      if (nextRuntimeState) {
        setRuntimeState(nextRuntimeState)
        setRuntimeStateError('')
      }
      setStreamError('')
      setCommandLineText('')
      scrollTerminalToBottom()
    } catch (error: any) {
      const errorText = String(error?.message || t.admin.pluginWorkspaceStreamError)
      setStreamError(errorText)
      if (!(runtimeSignalPendingRef.current && isWorkspaceCancellationErrorText(errorText))) {
        toast.error(errorText)
      }
    } finally {
      setRuntimeCommandPendingState(false)
      clearPendingRuntimeTask()
      setRuntimeSignalPendingState(null)
    }
  }

  const signalWorkspace = async (signal: 'interrupt' | 'terminate') => {
    if (effectiveSignaling) return
    if (!controlGranted) {
      toast.error(controlStatusMessage)
      return
    }
    if (!hasCancelableTask) return
    if (!activeTaskId) {
      const pluginID = plugin?.id
      const runtimeTaskID = pendingRuntimeTaskIdRef.current.trim()
      if (!pluginID || !runtimeTaskID) {
        return
      }
      setRuntimeSignalPendingState(signal)
      try {
        await cancelAdminPluginExecutionTask(pluginID, runtimeTaskID)
        setStreamError('')
      } catch (error: any) {
        const errorText = String(error?.message || t.admin.pluginWorkspaceStreamError)
        setRuntimeSignalPendingState(null)
        setStreamError(errorText)
        toast.error(errorText)
      }
      return
    }
    const requestID = createWorkspaceRequestID()
    if (
      sendWorkspaceSocketFrame({
        type: 'signal',
        request_id: requestID,
        signal,
      })
    ) {
      setSocketSignalPendingRequest(requestID, signal)
      setStreamError('')
      return
    }
    onSignal({
      signal,
    })
  }

  const handleTerminalInputKeyDown = (event: ReactKeyboardEvent<HTMLTextAreaElement>) => {
    if (
      !event.ctrlKey &&
      !event.metaKey &&
      !event.altKey &&
      completionOverlay?.suggestions.length
    ) {
      if (event.key === 'ArrowUp') {
        event.preventDefault()
        moveCompletionSelection('previous')
        return
      }
      if (event.key === 'ArrowDown') {
        event.preventDefault()
        moveCompletionSelection('next')
        return
      }
      if (event.key === 'Escape') {
        event.preventDefault()
        setCompletionStatus('')
        setCompletionOverlay(null)
        return
      }
    }
    if (
      !event.ctrlKey &&
      !event.metaKey &&
      !event.altKey &&
      !activeTerminalSession &&
      commandHistory.length > 0 &&
      event.key === 'ArrowUp' &&
      shouldHandlePluginWorkspaceHistoryNavigation({
        value: commandLineText,
        selectionStart: event.currentTarget.selectionStart ?? commandLineText.length,
        selectionEnd: event.currentTarget.selectionEnd ?? event.currentTarget.selectionStart ?? 0,
        direction: 'previous',
      })
    ) {
      event.preventDefault()
      navigateCommandHistory('previous')
      return
    }
    if (
      !event.ctrlKey &&
      !event.metaKey &&
      !event.altKey &&
      !activeTerminalSession &&
      commandHistory.length > 0 &&
      event.key === 'ArrowDown' &&
      shouldHandlePluginWorkspaceHistoryNavigation({
        value: commandLineText,
        selectionStart: event.currentTarget.selectionStart ?? commandLineText.length,
        selectionEnd: event.currentTarget.selectionEnd ?? event.currentTarget.selectionStart ?? 0,
        direction: 'next',
      })
    ) {
      event.preventDefault()
      navigateCommandHistory('next')
      return
    }
    if (!event.ctrlKey && !event.metaKey && !event.altKey && event.key === 'Tab') {
      event.preventDefault()
      if (!activeTerminalSession && completionOverlay?.suggestions.length) {
        applyCompletionSuggestion(
          completionOverlay.suggestions[
            Math.max(0, Math.min(completionSelectedIndex, completionOverlay.suggestions.length - 1))
          ] || completionOverlay.suggestions[0]
        )
        return
      }
      const selectionStart = event.currentTarget.selectionStart ?? commandLineText.length
      const selectionEnd = event.currentTarget.selectionEnd ?? selectionStart
      if (activeTerminalSession) {
        replaceCommandLineText(
          `${commandLineText.slice(0, selectionStart)}\t${commandLineText.slice(selectionEnd)}`,
          {
            selection: {
              start: selectionStart + 1,
              end: selectionStart + 1,
            },
          }
        )
        return
      }
      const completion = resolvePluginWorkspaceCompletion({
        value: commandLineText,
        selectionStart,
        selectionEnd,
        dynamicPaths: workspaceCompletionPaths,
      })
      if (completion.kind === 'none') {
        setCompletionOverlay(null)
        setCompletionStatus(t.admin.pluginWorkspaceCompletionNoMatch)
        return
      }
      if (completion.kind === 'suggestions') {
        const preview = completion.suggestions.slice(0, 6).join(', ')
        const suffix = completion.suggestions.length > 6 ? ', ...' : ''
        setCompletionOverlay({
          token: completion.token,
          resolvedToken: completion.resolvedToken,
          suggestions: completion.suggestions,
        })
        setCompletionStatus(
          t.admin.pluginWorkspaceCompletionSuggestions.replace('{items}', `${preview}${suffix}`)
        )
        return
      }
      if (completion.suggestions.length > 1) {
        const preview = completion.suggestions.slice(0, 6).join(', ')
        const suffix = completion.suggestions.length > 6 ? ', ...' : ''
        setCompletionOverlay({
          token: completion.token,
          resolvedToken: completion.resolvedToken,
          suggestions: completion.suggestions,
        })
        setCompletionStatus(
          t.admin.pluginWorkspaceCompletionSuggestions.replace('{items}', `${preview}${suffix}`)
        )
      } else {
        setCompletionStatus('')
        setCompletionOverlay(null)
      }
      replaceCommandLineText(completion.nextValue, {
        preserveCompletionStatus: true,
        selection: {
          start: completion.nextSelectionStart,
          end: completion.nextSelectionEnd,
        },
      })
      return
    }
    if (
      (event.ctrlKey || event.metaKey) &&
      !event.altKey &&
      !event.shiftKey &&
      event.key.toLowerCase() === 'c'
    ) {
      const target = event.currentTarget
      const hasSelection = target.selectionStart !== target.selectionEnd
      if (
        shouldInterruptWorkspaceTerminalInputShortcut({
          hasCancelableTask,
          hasSelection,
        })
      ) {
        event.preventDefault()
        void signalWorkspace('interrupt')
      }
      return
    }
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault()
      void submitTerminalLine()
    }
  }

  const handleSearchInputKeyDown = (event: ReactKeyboardEvent<HTMLInputElement>) => {
    if (
      (event.ctrlKey || event.metaKey) &&
      !event.altKey &&
      !event.shiftKey &&
      event.key.toLowerCase() === 'f'
    ) {
      event.preventDefault()
      return
    }
    if (event.key === 'Escape') {
      event.preventDefault()
      if (searchText.trim()) {
        setSearchText('')
        return
      }
      setShowTerminalSearch(false)
      focusShellInput()
    }
  }

  const handleTerminalViewportScroll = () => {
    const viewport = terminalViewportRef.current
    const atBottom = isViewportNearBottom(viewport)
    if (atBottom) {
      setFollowTerminalOutputState(true)
      setPendingTerminalOutput(false)
      return
    }
    if (followTerminalOutputRef.current && !hasRecentTerminalUserScrollIntent()) {
      return
    }
    setFollowTerminalOutputState(false)
  }

  useEffect(() => {
    if (!open || !hasCancelableTask || !controlGranted) {
      return
    }
    const onKeyDown = (event: KeyboardEvent) => {
      if (
        (event.ctrlKey || event.metaKey) &&
        !event.altKey &&
        !event.shiftKey &&
        event.key.toLowerCase() === 'c'
      ) {
        if (isEditableWorkspaceTarget(event.target)) {
          return
        }
        const selection = typeof window !== 'undefined' ? window.getSelection() : null
        const selectedText = selection ? selection.toString() : ''
        if (selectedText) {
          return
        }
        event.preventDefault()
        void signalWorkspace('interrupt')
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => {
      window.removeEventListener('keydown', onKeyDown)
    }
  }, [open, hasCancelableTask, controlGranted, effectiveSignaling])

  useEffect(() => {
    if (!open) {
      return
    }
    const onKeyDown = (event: KeyboardEvent) => {
      if (
        (event.ctrlKey || event.metaKey) &&
        !event.altKey &&
        !event.shiftKey &&
        event.key.toLowerCase() === 'f'
      ) {
        event.preventDefault()
        setShowTerminalSearch(true)
        return
      }
      if (
        (event.ctrlKey || event.metaKey) &&
        !event.altKey &&
        !event.shiftKey &&
        event.key.toLowerCase() === 'k'
      ) {
        event.preventDefault()
        focusShellInput()
        return
      }
      if (
        (event.ctrlKey || event.metaKey) &&
        !event.altKey &&
        !event.shiftKey &&
        event.key.toLowerCase() === 'j'
      ) {
        event.preventDefault()
        scrollTerminalToBottom()
        focusShellInput()
        return
      }
      if (
        event.key === 'Escape' &&
        showTerminalSearch &&
        !searchText.trim() &&
        !isEditableWorkspaceTarget(event.target)
      ) {
        setShowTerminalSearch(false)
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => {
      window.removeEventListener('keydown', onKeyDown)
    }
  }, [open, searchText, showTerminalSearch])

  useEffect(() => {
    if (!open || !controlGranted) {
      return
    }
    if (typeof window === 'undefined') {
      return
    }
    window.requestAnimationFrame(() => {
      const input = shellInputRef.current
      if (!input) {
        return
      }
      input.focus()
      const length = input.value.length
      input.setSelectionRange(length, length)
    })
  }, [open, controlGranted, activeTerminalSession])

  useEffect(() => {
    if (!open || !showTerminalSearch || typeof window === 'undefined') {
      return
    }
    window.requestAnimationFrame(() => {
      searchInputRef.current?.focus()
    })
  }, [open, showTerminalSearch])

  useEffect(() => {
    if (!open || !plugin?.id || !showRuntimePanel) {
      return
    }
    void fetchRuntimeState({ silent: true })
  }, [open, plugin?.id, showRuntimePanel])

  useLayoutEffect(() => {
    if (!open) {
      terminalLastSeqRef.current = effectiveWorkspace?.last_seq || 0
      return
    }
    const currentLastSeq = effectiveWorkspace?.last_seq || 0
    const previousLastSeq = terminalLastSeqRef.current
    const hasNewOutput = currentLastSeq > previousLastSeq
    terminalLastSeqRef.current = currentLastSeq
    if (typeof window === 'undefined' || normalizedSearchText) {
      return
    }
    if (followTerminalOutput || previousLastSeq === 0) {
      scrollTerminalToBottom({ preserveFollow: true })
      return
    }
    if (hasNewOutput) {
      setPendingTerminalOutput(true)
    }
  }, [
    open,
    normalizedSearchText,
    terminalDisplayItems.length,
    effectiveWorkspace?.last_seq,
    followTerminalOutput,
  ])

  useEffect(() => {
    if (
      !open ||
      normalizedSearchText ||
      !followTerminalOutput ||
      typeof ResizeObserver === 'undefined'
    ) {
      return
    }
    const content = terminalContentRef.current
    if (!content) {
      return
    }
    const observer = new ResizeObserver(() => {
      scrollTerminalToBottom({ preserveFollow: true })
    })
    observer.observe(content)
    return () => {
      observer.disconnect()
    }
  }, [open, normalizedSearchText, followTerminalOutput, terminalDisplayItems.length])

  return (
      <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex h-[88vh] max-w-6xl flex-col overflow-hidden border border-neutral-800 bg-neutral-950 p-0 text-neutral-100 shadow-2xl">
        <div className="flex min-h-0 flex-1 flex-col">
          <DialogHeader className="border-b border-white/10 px-4 py-3 text-left">
            <DialogTitle className="text-base font-semibold text-neutral-50">
              {t.admin.pluginWorkspace}
            </DialogTitle>
            <DialogDescription className="text-sm text-neutral-400">
              {plugin
                ? `${plugin.display_name || plugin.name}${plugin.display_name && plugin.name ? ` (${plugin.name})` : ''} · ${t.admin.pluginWorkspaceSubtitle}`
                : t.admin.pluginWorkspaceSubtitle}
            </DialogDescription>
          </DialogHeader>

          <div className="flex min-h-0 flex-1 flex-col">
            <div className="border-b border-white/10 bg-neutral-950 px-4 py-2.5">
              <div className="flex flex-wrap items-center gap-x-4 gap-y-1.5 text-[11px] leading-5 text-neutral-400">
                <span className="inline-flex items-center gap-1 text-neutral-200">
                  {connectionState === 'live' ? (
                    <Wifi className="h-3.5 w-3.5" />
                  ) : (
                    <WifiOff className="h-3.5 w-3.5" />
                  )}
                  {workspaceConnectionLabel(connectionState, t)}
                </span>
                <span>{workspaceRuntimeStatusLabel(effectiveWorkspace, t)}</span>
                <span>
                  {controlGranted
                    ? t.admin.pluginWorkspaceRoleOwner
                    : t.admin.pluginWorkspaceRoleViewer}
                </span>
                {ownerAdminId ? (
                  <span>{t.admin.pluginWorkspaceOwnerBadge.replace('{owner}', ownerLabel)}</span>
                ) : null}
                {viewerCount > 0 ? (
                  <span>
                    {t.admin.pluginWorkspaceViewerCount.replace('{count}', String(viewerCount))}
                  </span>
                ) : null}
                {cancelableTaskId ? <span>{`#${cancelableTaskId}`}</span> : null}
                {effectiveWorkspace?.active_command ? (
                  <span className="break-all">
                    {`${t.admin.pluginWorkspaceActiveCommand}: ${effectiveWorkspace.active_command}`}
                  </span>
                ) : null}
              </div>
              <div className="mt-3 flex flex-wrap items-center gap-2">
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className={terminalActionButtonClass}
                  onClick={resetWorkspace}
                  disabled={resetting || !controlGranted || !activeTaskId}
                >
                  {resetting ? (
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  ) : (
                    <RotateCcw className="mr-2 h-4 w-4" />
                  )}
                  {t.admin.pluginWorkspaceReset}
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className={terminalActionButtonClass}
                  onClick={onRefresh}
                  disabled={workspaceLoading}
                >
                  {workspaceLoading ? (
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  ) : (
                    <RefreshCw className="mr-2 h-4 w-4" />
                  )}
                  {t.common.refresh}
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className={terminalActionButtonClass}
                  onClick={clearWorkspace}
                  disabled={
                    clearing ||
                    !controlGranted ||
                    (!entries.length && !effectiveWorkspace?.entry_count)
                  }
                >
                  {clearing ? (
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  ) : (
                    <Trash2 className="mr-2 h-4 w-4" />
                  )}
                  {t.admin.pluginWorkspaceClear}
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className={terminalActionButtonClass}
                  onClick={copyAll}
                  disabled={terminalEntries.length === 0}
                >
                  <Copy className="mr-2 h-4 w-4" />
                  {copyButtonLabel}
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className={terminalActionButtonClass}
                  onClick={() => setShowRuntimePanel((current) => !current)}
                >
                  {showRuntimePanel ? t.common.collapse : t.admin.pluginWorkspaceRuntimePanel}
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className={terminalActionButtonClass}
                  onClick={() => setShowTerminalSearch((current) => !current)}
                  disabled={!hasTerminalTranscript}
                >
                  <Search className="mr-2 h-4 w-4" />
                  {showTerminalSearch ? t.common.collapse : t.common.search}
                </Button>
                {recentControlEvents.length > 0 ? (
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    className={terminalActionButtonClass}
                    onClick={() => setShowActivityPanel((current) => !current)}
                  >
                    {showActivityPanel ? t.common.collapse : t.common.expand}
                    <span className="ml-2 text-xs text-neutral-400">
                      {`${t.admin.pluginWorkspaceControlTimeline} (${recentControlEvents.length})`}
                    </span>
                  </Button>
                ) : null}
                {!controlGranted ? (
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    className={terminalActionButtonClass}
                    onClick={claimWorkspaceControl}
                    disabled={!canClaimControl}
                  >
                    {claimingControl ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}
                    {t.admin.pluginWorkspaceClaimControl}
                  </Button>
                ) : null}
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className={terminalActionButtonClass}
                  onClick={() => void signalWorkspace('interrupt')}
                  disabled={!hasCancelableTask || effectiveSignaling || !controlGranted}
                >
                  {interruptPending ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}
                  {t.admin.pluginWorkspaceInterrupt}
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className={terminalActionButtonClass}
                  onClick={() => void signalWorkspace('terminate')}
                  disabled={!hasCancelableTask || effectiveSignaling || !controlGranted}
                >
                  {terminatePending ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}
                  {t.admin.pluginWorkspaceTerminate}
                </Button>
              </div>
              {showRuntimePanel ? (
                <div className="mt-3 border-t border-white/10 pt-3">
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div className="space-y-1">
                      <div className="text-[11px] font-medium uppercase tracking-[0.16em] text-neutral-500">
                        {t.admin.pluginWorkspaceRuntimePanel}
                      </div>
                      <div className={`text-xs ${runtimeStatusTone}`}>
                        {runtimeStateLoading && !runtimeState
                          ? t.common.loading
                          : runtimeStatusLabel}
                      </div>
                    </div>
                    <div className="flex flex-wrap items-center gap-2">
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        className={terminalActionButtonClass}
                        onClick={() => {
                          void fetchRuntimeState()
                        }}
                        disabled={runtimeStateLoading}
                      >
                        {runtimeStateLoading ? (
                          <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                        ) : (
                          <RefreshCw className="mr-2 h-4 w-4" />
                        )}
                        {t.admin.pluginWorkspaceRuntimeRefresh}
                      </Button>
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        className={terminalActionButtonClass}
                        onClick={() => {
                          void rebuildRuntime()
                        }}
                        disabled={!controlGranted || runtimeStateResetting || activeTerminalSession}
                      >
                        {runtimeStateResetting ? (
                          <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                        ) : (
                          <RotateCcw className="mr-2 h-4 w-4" />
                        )}
                        {t.admin.pluginWorkspaceRuntimeRebuild}
                      </Button>
                    </div>
                  </div>
                  {runtimeState ? (
                    <div className="mt-3 flex flex-wrap gap-2 text-[11px] text-neutral-400">
                      {runtimeState.instance_id ? (
                        <span className="border border-white/10 px-2 py-1 font-mono">
                          {`${t.admin.pluginWorkspaceRuntimeInstance}: ${runtimeState.instance_id}`}
                        </span>
                      ) : null}
                      {runtimeState.current_action ? (
                        <span className="border border-white/10 px-2 py-1 font-mono">
                          {`${t.admin.pluginWorkspaceRuntimeCurrentAction}: ${runtimeState.current_action}`}
                        </span>
                      ) : runtimeState.last_action ? (
                        <span className="border border-white/10 px-2 py-1 font-mono">
                          {`${t.admin.pluginWorkspaceRuntimeLastAction}: ${runtimeState.last_action}`}
                        </span>
                      ) : null}
                      {typeof runtimeState.boot_count === 'number' ? (
                        <span className="border border-white/10 px-2 py-1">
                          {`${t.admin.pluginWorkspaceRuntimeBootCount}: ${runtimeState.boot_count}`}
                        </span>
                      ) : null}
                      {typeof runtimeState.total_requests === 'number' ? (
                        <span className="border border-white/10 px-2 py-1">
                          {`${t.admin.pluginWorkspaceRuntimeRequestCount}: ${runtimeState.total_requests}`}
                        </span>
                      ) : null}
                      {typeof runtimeState.execute_count === 'number' ? (
                        <span className="border border-white/10 px-2 py-1">
                          {`${t.admin.pluginWorkspaceRuntimeExecuteCount}: ${runtimeState.execute_count}`}
                        </span>
                      ) : null}
                      {typeof runtimeState.eval_count === 'number' ? (
                        <span className="border border-white/10 px-2 py-1">
                          {`${t.admin.pluginWorkspaceRuntimeEvalCount}: ${runtimeState.eval_count}`}
                        </span>
                      ) : null}
                      {typeof runtimeState.inspect_count === 'number' ? (
                        <span className="border border-white/10 px-2 py-1">
                          {`${t.admin.pluginWorkspaceRuntimeInspectCount}: ${runtimeState.inspect_count}`}
                        </span>
                      ) : null}
                      {runtimeState.created_at ? (
                        <span className="border border-white/10 px-2 py-1">
                          {`${t.admin.pluginWorkspaceRuntimeCreatedAt}: ${formatDateTime(runtimeState.created_at, locale)}`}
                        </span>
                      ) : null}
                      {runtimeState.last_used_at ? (
                        <span className="border border-white/10 px-2 py-1">
                          {`${t.admin.pluginWorkspaceRuntimeLastUsedAt}: ${formatDateTime(runtimeState.last_used_at, locale)}`}
                        </span>
                      ) : null}
                    </div>
                  ) : null}
                  {runtimeState?.script_path ? (
                    <div className="mt-2 break-all font-mono text-[11px] text-neutral-500">
                      {runtimeState.script_path}
                    </div>
                  ) : null}
                  {runtimeState?.last_error ? (
                    <div className="mt-2 rounded-md border border-rose-500/20 bg-rose-500/10 px-3 py-2 text-xs leading-5 text-rose-200">
                      {`${t.admin.pluginWorkspaceRuntimeLastError}: ${runtimeState.last_error}`}
                    </div>
                  ) : null}
                </div>
              ) : null}
              {showTerminalSearch ? (
                <div className="mt-3 max-w-sm">
                  <div className="relative">
                    <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-neutral-500" />
                    <Input
                      ref={searchInputRef}
                      value={searchText}
                      onChange={(event) => setSearchText(event.target.value)}
                      onKeyDown={handleSearchInputKeyDown}
                      placeholder={t.admin.pluginWorkspaceSearchPlaceholder}
                      className="h-9 border-white/10 bg-neutral-950 pl-9 font-mono text-sm text-neutral-100 placeholder:text-neutral-500"
                      disabled={showWorkspaceLoadingOverlay || !hasTerminalTranscript}
                    />
                  </div>
                  {normalizedSearchText ? (
                    <div className="mt-2 flex items-center justify-between gap-2 text-[11px] text-neutral-500">
                      <span>{searchSummaryLabel}</span>
                      <button
                        type="button"
                        className="rounded-sm px-1.5 py-0.5 text-neutral-400 transition hover:bg-white/[0.05] hover:text-neutral-200"
                        onClick={() => setSearchText('')}
                      >
                        {t.common.clear}
                      </button>
                    </div>
                  ) : null}
                </div>
              ) : null}
            </div>

            <div
              ref={terminalViewportRef}
              className="min-h-0 flex-1 overflow-y-auto bg-[#0a0a0a] px-4 py-3 font-mono text-xs leading-6"
              onClick={() => shellInputRef.current?.focus()}
              onWheel={markTerminalUserScrollIntent}
              onPointerDown={markTerminalUserScrollIntent}
              onTouchMove={markTerminalUserScrollIntent}
              onScroll={handleTerminalViewportScroll}
            >
              {showWorkspaceLoadingOverlay ? (
                <div className="flex min-h-full items-center justify-center gap-2 text-sm text-neutral-500">
                  <Loader2 className="h-4 w-4 animate-spin" />
                  {t.common.loading}
                </div>
              ) : (
                <div ref={terminalContentRef} className="flex min-h-full flex-col">
                  {terminalDisplayItems.length === 0 ? (
                    <div className="flex flex-1 items-center justify-center px-6 text-center text-sm text-neutral-500">
                      {terminalSourceEntries.length === 0
                        ? t.admin.pluginWorkspaceTerminalNoOutput
                        : t.admin.pluginWorkspaceNoMatches}
                    </div>
                  ) : (
                    <div className="space-y-0">
                      {terminalDisplayItems.map((item) => {
                        if (item.type === 'console') {
                          return (
                            <WorkspaceTerminalConsoleCard
                              key={item.key}
                              previews={item.previews}
                              raw={item.raw}
                              t={t}
                            />
                          )
                        }
                        if (item.type === 'runtime') {
                          return (
                            <WorkspaceTerminalRuntimeCard
                              key={item.key}
                              raw={item.raw}
                              preview={item.preview}
                              source={item.source}
                              t={t}
                            />
                          )
                        }
                        if (item.type === 'json') {
                          return (
                            <WorkspaceTerminalJSONCard
                              key={item.key}
                              raw={item.raw}
                              preview={item.preview}
                              t={t}
                            />
                          )
                        }
                        const screen = buildWorkspaceTerminalScreen(item.blocks)
                        return (
                          <div key={item.key} className="space-y-0">
                            {screen.lines.map((line) => (
                              <div
                                key={line.key}
                                className="min-h-6 whitespace-pre-wrap break-words py-0 text-[12px] leading-6"
                              >
                                {line.segments.length === 0 ? (
                                  <span className="opacity-0">_</span>
                                ) : (
                                  line.segments.map((segment) => (
                                    <span key={segment.key} style={segment.style}>
                                      {segment.text}
                                    </span>
                                  ))
                                )}
                              </div>
                            ))}
                          </div>
                        )
                      })}
                    </div>
                  )}

                  <div className="mt-2 pt-1">
                    <div className="relative flex items-start gap-3 py-0.5">
                      <span
                        className={`mt-0.5 shrink-0 select-none font-mono text-[12px] font-semibold leading-6 ${terminalPromptClass}`}
                      >
                        {terminalPromptLabel}
                      </span>
                      <div className="min-w-0 flex-1">
                        <Textarea
                          ref={shellInputRef}
                          value={terminalInputValue}
                          onChange={(event) => {
                            const nextValue = event.target.value
                            if (commandHistoryIndex >= 0) {
                              setCommandHistoryIndex(-1)
                              setCommandHistoryDraft(nextValue)
                            }
                            replaceCommandLineText(nextValue)
                          }}
                          onKeyDown={handleTerminalInputKeyDown}
                          placeholder={terminalPlaceholder}
                          className="min-h-0 flex-1 resize-none overflow-hidden border-0 bg-transparent px-0 py-0 font-mono text-[12px] leading-6 text-neutral-100 caret-sky-400 shadow-none placeholder:text-neutral-600 focus-visible:ring-0 focus-visible:ring-offset-0"
                          rows={1}
                          disabled={!controlGranted}
                          spellCheck={false}
                          autoCapitalize="off"
                          autoCorrect="off"
                        />
                        {completionOverlay && completionSuggestions.length > 0 ? (
                          <div className="absolute bottom-full left-0 mb-2 min-w-[320px] max-w-[min(560px,calc(100%-0.5rem))] overflow-hidden border border-white/10 bg-[#111111]/95 shadow-2xl backdrop-blur-sm">
                            {completionHiddenBefore > 0 ? (
                              <div className="border-b border-white/10 px-3 py-1 text-[10px] text-neutral-500">
                                {`… ${completionHiddenBefore}`}
                              </div>
                            ) : null}
                            <div className="max-h-52 overflow-y-auto py-1">
                              {completionVisibleSuggestions.map((suggestion, index) => {
                                const suggestionIndex = completionVisibleStart + index
                                const selected = suggestionIndex === completionSelectedIndex
                                return (
                                  <button
                                    key={suggestion}
                                    type="button"
                                    className={
                                      selected
                                        ? 'flex w-full items-center bg-sky-500/15 px-3 py-1.5 text-left font-mono text-[12px] leading-5 text-sky-100'
                                        : 'flex w-full items-center px-3 py-1.5 text-left font-mono text-[12px] leading-5 text-neutral-200 hover:bg-white/[0.06] hover:text-white'
                                    }
                                    onMouseDown={(event) => {
                                      event.preventDefault()
                                    }}
                                    onMouseEnter={() => setCompletionSelectedIndex(suggestionIndex)}
                                    onClick={() => applyCompletionSuggestion(suggestion)}
                                  >
                                    {suggestion}
                                  </button>
                                )
                              })}
                            </div>
                            <div className="flex items-center justify-between gap-3 border-t border-white/10 px-3 py-1.5 text-[10px] text-neutral-500">
                              <span className="truncate">
                                {completionOverlay.resolvedToken ||
                                  completionOverlay.token ||
                                  terminalPromptLabel}
                              </span>
                              <span>{`${completionSelectedIndex + 1}/${completionSuggestions.length}`}</span>
                            </div>
                            {completionHiddenAfter > 0 ? (
                              <div className="border-t border-white/10 px-3 py-1 text-[10px] text-neutral-500">
                                {`… ${completionHiddenAfter}`}
                              </div>
                            ) : null}
                          </div>
                        ) : null}
                      </div>
                      {terminalSubmitBusy ? (
                        <Loader2 className="mt-1 h-4 w-4 shrink-0 animate-spin text-neutral-500" />
                      ) : null}
                    </div>
                  </div>
                </div>
              )}
            </div>

            {showActivityPanel && recentControlEvents.length > 0 ? (
              <div className="border-t border-white/10 bg-neutral-900/60 px-4 py-3">
                <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
                  <div className="text-xs font-medium uppercase tracking-[0.16em] text-neutral-500">
                    {t.admin.pluginWorkspaceControlTimeline}
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    {(
                      [
                        ['all', t.admin.pluginWorkspaceTimelineFilterAll],
                        ['ownership', t.admin.pluginWorkspaceTimelineFilterOwnership],
                        ['actions', t.admin.pluginWorkspaceTimelineFilterActions],
                      ] as const
                    ).map(([value, label]) => {
                      const active = timelineFilter === value
                      return (
                        <Button
                          key={value}
                          type="button"
                          variant="outline"
                          size="sm"
                          className={
                            active
                              ? 'h-7 border-white/20 bg-white/10 px-2 text-[11px] text-white hover:bg-white/15'
                              : 'h-7 border-white/10 bg-transparent px-2 text-[11px] text-neutral-400 hover:bg-white/[0.05] hover:text-neutral-200'
                          }
                          onClick={() => setTimelineFilter(value)}
                        >
                          {label}
                        </Button>
                      )
                    })}
                  </div>
                </div>
                {filteredControlEvents.length === 0 ? (
                  <div className="border-white/8 rounded-md border bg-black/10 px-3 py-2 text-xs text-neutral-500">
                    {t.admin.pluginWorkspaceControlTimelineEmpty}
                  </div>
                ) : (
                  <div className="max-h-32 space-y-2 overflow-y-auto pr-1">
                    {filteredControlEvents.map((event) => (
                      <div
                        key={`${event.seq}-${event.timestamp || ''}`}
                        className="border-white/8 rounded-md border bg-black/10 px-3 py-2"
                      >
                        <div className="flex flex-wrap items-center justify-between gap-2">
                          <span className="text-xs font-medium text-neutral-200">
                            {workspaceControlEventLabel(event, t)}
                          </span>
                          <span className="text-[11px] text-neutral-500">
                            {formatWorkspaceTerminalTimestamp(event.timestamp, locale)}
                          </span>
                        </div>
                        <div className="mt-1 text-[11px] leading-5 text-neutral-500">
                          {workspaceControlEventSummary(event, locale, t)}
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            ) : null}

            <div className="border-t border-white/10 bg-neutral-900/90 px-4 py-3">
              <div className="mb-2 flex flex-wrap items-center gap-x-4 gap-y-2 text-xs">
                <span className={terminalStatusClass}>{terminalStatusMessage}</span>
                {terminalFollowStatus ? (
                  <span className="text-amber-300">{terminalFollowStatus}</span>
                ) : null}
                {plugin?.address_display || plugin?.address ? (
                  <span className="break-all text-neutral-500">
                    {plugin.address_display || plugin.address}
                  </span>
                ) : null}
                {effectiveWorkspace?.has_more ? (
                  <span className="text-neutral-500">
                    {t.admin.pluginWorkspaceLatestNote
                      .replace('{shown}', String(entries.length))
                      .replace('{total}', String(effectiveWorkspace?.entry_count ?? 0))}
                  </span>
                ) : null}
                {!followTerminalOutput || pendingTerminalOutput ? (
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    className="h-7 border-white/10 bg-transparent px-2 text-[11px] text-neutral-300 hover:bg-white/[0.05] hover:text-white"
                    onClick={() => {
                      scrollTerminalToBottom()
                      focusShellInput()
                    }}
                  >
                    {t.admin.pluginWorkspaceJumpToLatest}
                  </Button>
                ) : null}
              </div>

            </div>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
