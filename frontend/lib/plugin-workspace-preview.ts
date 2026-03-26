export type WorkspaceRuntimePreviewEntry = {
  key: string
  value: WorkspaceRuntimePreview
}

export type WorkspaceRuntimePreview = {
  type: string
  summary: string
  className?: string
  value?: string | number | boolean | null
  length?: number
  keys: string[]
  entries: WorkspaceRuntimePreviewEntry[]
  truncated: boolean
}

type AnyRecord = Record<string, unknown>

function asRecord(value: unknown): AnyRecord | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return null
  }
  return value as AnyRecord
}

export function parseWorkspaceRuntimePreview(value: unknown): WorkspaceRuntimePreview | null {
  const record = asRecord(value)
  if (!record) {
    return null
  }
  const type = String(record.type || '').trim()
  const summary = String(record.summary || '').trim()
  const className = String(record.class_name || '').trim()
  const keys = Array.isArray(record.keys)
    ? record.keys.map((item) => String(item || '').trim()).filter(Boolean)
    : []
  const entries = Array.isArray(record.entries)
    ? record.entries
        .map((entry) => {
          const entryRecord = asRecord(entry)
          if (!entryRecord) {
            return null
          }
          const key = String(entryRecord.key || '').trim()
          const nextValue = parseWorkspaceRuntimePreview(entryRecord.value)
          if (!key || !nextValue) {
            return null
          }
          return {
            key,
            value: nextValue,
          } satisfies WorkspaceRuntimePreviewEntry
        })
        .filter((entry): entry is WorkspaceRuntimePreviewEntry => Boolean(entry))
    : []
  if (!type && !summary && !className && keys.length === 0 && entries.length === 0) {
    return null
  }
  const lengthValue = Number(record.length)
  const normalizedLength =
    Number.isFinite(lengthValue) && lengthValue >= 0 ? Math.trunc(lengthValue) : undefined
  const rawValue = record.value
  return {
    type: type || 'value',
    summary: summary || type || String(rawValue || ''),
    className: className || undefined,
    value:
      typeof rawValue === 'string' || typeof rawValue === 'number' || typeof rawValue === 'boolean'
        ? rawValue
        : undefined,
    length: normalizedLength,
    keys,
    entries,
    truncated: Boolean(record.truncated),
  }
}

export function extractWorkspaceRuntimePreviewFromMetadata(
  metadata?: Record<string, string>
): WorkspaceRuntimePreview | null {
  const raw = String(metadata?.workspace_runtime_preview_json || '').trim()
  if (!raw) {
    return null
  }
  try {
    return parseWorkspaceRuntimePreview(JSON.parse(raw))
  } catch {
    return null
  }
}

export function extractWorkspaceConsolePreviewsFromMetadata(
  metadata?: Record<string, string>
): WorkspaceRuntimePreview[] {
  const raw = String(metadata?.workspace_console_previews_json || '').trim()
  if (!raw) {
    return []
  }
  try {
    const parsed = JSON.parse(raw)
    if (!Array.isArray(parsed)) {
      return []
    }
    return parsed
      .map((item) => parseWorkspaceRuntimePreview(item))
      .filter((item): item is WorkspaceRuntimePreview => Boolean(item))
  } catch {
    return []
  }
}
