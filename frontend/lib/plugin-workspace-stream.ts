import type {
  AdminPluginWorkspaceEntry,
  AdminPluginWorkspaceRuntimeState,
  AdminPluginWorkspaceSnapshot,
  AdminPluginWorkspaceStreamEvent,
  AdminPluginWorkspaceWebSocketServerFrame,
} from './api'

type AnyRecord = Record<string, unknown>

function asRecord(value: unknown): AnyRecord | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return null
  }
  return value as AnyRecord
}

export function isWorkspaceSnapshotRecord(value: unknown): value is AdminPluginWorkspaceSnapshot {
  const record = asRecord(value)
  if (!record || !Array.isArray(record.entries)) {
    return false
  }
  return ['buffer_capacity', 'entry_count', 'last_seq', 'control_granted', 'status'].some((key) =>
    Object.prototype.hasOwnProperty.call(record, key)
  )
}

export function extractWorkspaceSnapshot(value: unknown): AdminPluginWorkspaceSnapshot | null {
  const root = asRecord(value)
  if (!root) {
    return null
  }
  if (isWorkspaceSnapshotRecord(root.workspace)) {
    return root.workspace
  }
  if (isWorkspaceSnapshotRecord(root)) {
    return root
  }
  return null
}

export function isWorkspaceRuntimeStateRecord(
  value: unknown
): value is AdminPluginWorkspaceRuntimeState {
  const record = asRecord(value)
  if (!record) {
    return false
  }
  return [
    'available',
    'exists',
    'instance_id',
    'script_path',
    'loaded',
    'busy',
    'current_action',
    'last_action',
    'created_at',
    'last_used_at',
    'boot_count',
    'total_requests',
    'execute_count',
    'eval_count',
    'inspect_count',
    'last_error',
    'completion_paths',
  ].some((key) => Object.prototype.hasOwnProperty.call(record, key))
}

export function extractWorkspaceRuntimeState(
  value: unknown
): AdminPluginWorkspaceRuntimeState | null {
  const root = asRecord(value)
  if (!root) {
    return null
  }
  if (isWorkspaceRuntimeStateRecord(root.runtime_state)) {
    return root.runtime_state
  }
  return isWorkspaceRuntimeStateRecord(root) ? root : null
}

export function mergeWorkspaceEntries(
  existing: AdminPluginWorkspaceEntry[],
  incoming: AdminPluginWorkspaceEntry[],
  maxEntries: number
): AdminPluginWorkspaceEntry[] {
  const bySeq = new Map<number, AdminPluginWorkspaceEntry>()
  existing.forEach((entry) => {
    bySeq.set(Number(entry.seq || 0), entry)
  })
  incoming.forEach((entry) => {
    bySeq.set(Number(entry.seq || 0), entry)
  })
  const merged = Array.from(bySeq.values()).sort(
    (left, right) => Number(left.seq || 0) - Number(right.seq || 0)
  )
  if (maxEntries > 0 && merged.length > maxEntries) {
    return merged.slice(-maxEntries)
  }
  return merged
}

export function applyWorkspaceStreamEvent(
  current: AdminPluginWorkspaceSnapshot | null,
  event: AdminPluginWorkspaceStreamEvent
): AdminPluginWorkspaceSnapshot | null {
  if (event.workspace) {
    return event.workspace
  }
  if (!current) {
    return null
  }
  let entries = current.entries || []
  if (event.cleared) {
    entries = []
  }
  if (Array.isArray(event.entries) && event.entries.length > 0) {
    entries = mergeWorkspaceEntries(entries, event.entries, current.buffer_capacity || 0)
  }
  const entryCount =
    typeof event.entry_count === 'number' && Number.isFinite(event.entry_count)
      ? event.entry_count
      : event.cleared
        ? entries.length
        : current.entry_count
  return {
    ...current,
    entries,
    entry_count: entryCount,
    has_more: entryCount > entries.length,
    last_seq:
      typeof event.last_seq === 'number' && Number.isFinite(event.last_seq)
        ? event.last_seq
        : current.last_seq,
    updated_at: event.updated_at || current.updated_at,
  }
}

export function applyWorkspaceWebSocketFrame(
  current: AdminPluginWorkspaceSnapshot | null,
  frame: AdminPluginWorkspaceWebSocketServerFrame
): AdminPluginWorkspaceSnapshot | null {
  if (!frame?.event) {
    return current
  }
  return applyWorkspaceStreamEvent(current, frame.event)
}

function workspaceSnapshotLastSeq(
  value: AdminPluginWorkspaceSnapshot | null | undefined
): number {
  const seq = Number(value?.last_seq || 0)
  return Number.isFinite(seq) && seq >= 0 ? seq : 0
}

export function preferNewerWorkspaceSnapshot(
  current: AdminPluginWorkspaceSnapshot | null,
  next: AdminPluginWorkspaceSnapshot | null
): AdminPluginWorkspaceSnapshot | null {
  if (!next) {
    return current
  }
  if (!current) {
    return next
  }
  return workspaceSnapshotLastSeq(next) >= workspaceSnapshotLastSeq(current) ? next : current
}
