export type WorkspaceStructuredPreviewEntry = {
  key: string
  value: WorkspaceStructuredPreview
}

export type WorkspaceStructuredPreview = {
  type: string
  summary: string
  className?: string
  value?: string | number | boolean | null
  length?: number
  keys: string[]
  entries: WorkspaceStructuredPreviewEntry[]
  truncated: boolean
}

export type WorkspaceParsedJSONOutput = {
  raw: string
  preview: WorkspaceStructuredPreview
}

const MAX_WORKSPACE_JSON_PREVIEW_DEPTH = 4
const MAX_WORKSPACE_JSON_PREVIEW_ENTRIES = 24
const MAX_WORKSPACE_JSON_STRING_LENGTH = 160

function truncateWorkspaceJSONString(value: string, maxLength = MAX_WORKSPACE_JSON_STRING_LENGTH) {
  if (value.length <= maxLength) {
    return {
      value,
      truncated: false,
    }
  }
  if (maxLength <= 3) {
    return {
      value: value.slice(0, maxLength),
      truncated: true,
    }
  }
  return {
    value: `${value.slice(0, maxLength - 3)}...`,
    truncated: true,
  }
}

function formatWorkspaceJSONPrimitiveSummary(value: unknown): string {
  if (value === null) {
    return 'null'
  }
  if (typeof value === 'string') {
    return JSON.stringify(truncateWorkspaceJSONString(value).value)
  }
  if (typeof value === 'number' || typeof value === 'boolean') {
    return String(value)
  }
  return JSON.stringify(value)
}

function formatWorkspaceStructuredPreview(preview: WorkspaceStructuredPreview): string {
  switch (preview.type) {
    case 'null':
    case 'boolean':
    case 'number':
    case 'string':
      return preview.summary
    case 'array': {
      const parts = preview.entries.map((entry) => formatWorkspaceStructuredPreview(entry.value))
      if (preview.truncated) {
        parts.push('...')
      }
      return `[${parts.join(', ')}]`
    }
    case 'object': {
      const parts = preview.entries.map(
        (entry) => `${entry.key}: ${formatWorkspaceStructuredPreview(entry.value)}`
      )
      if (preview.truncated) {
        parts.push('...')
      }
      return `{ ${parts.join(', ')} }`
    }
    default:
      return preview.summary
  }
}

function buildWorkspaceStructuredPreview(
  value: unknown,
  depth = MAX_WORKSPACE_JSON_PREVIEW_DEPTH
): WorkspaceStructuredPreview {
  if (value === null) {
    return {
      type: 'null',
      summary: 'null',
      keys: [],
      entries: [],
      truncated: false,
      value: null,
    }
  }
  if (typeof value === 'string') {
    const formatted = truncateWorkspaceJSONString(value)
    return {
      type: 'string',
      summary: JSON.stringify(formatted.value),
      value,
      length: value.length,
      keys: [],
      entries: [],
      truncated: formatted.truncated,
    }
  }
  if (typeof value === 'number') {
    return {
      type: 'number',
      summary: String(value),
      value,
      keys: [],
      entries: [],
      truncated: false,
    }
  }
  if (typeof value === 'boolean') {
    return {
      type: 'boolean',
      summary: String(value),
      value,
      keys: [],
      entries: [],
      truncated: false,
    }
  }
  if (Array.isArray(value)) {
    if (depth <= 0) {
      return {
        type: 'array',
        className: 'Array',
        summary: `[Array(${value.length})]`,
        length: value.length,
        keys: [],
        entries: [],
        truncated: false,
      }
    }
    const entries = value.slice(0, MAX_WORKSPACE_JSON_PREVIEW_ENTRIES).map((item, index) => ({
      key: String(index),
      value: buildWorkspaceStructuredPreview(item, depth - 1),
    }))
    const preview: WorkspaceStructuredPreview = {
      type: 'array',
      className: 'Array',
      summary: '',
      length: value.length,
      keys: [],
      entries,
      truncated: value.length > MAX_WORKSPACE_JSON_PREVIEW_ENTRIES,
    }
    preview.summary = formatWorkspaceStructuredPreview(preview)
    return preview
  }
  if (typeof value === 'object') {
    const record = value as Record<string, unknown>
    const keys = Object.keys(record)
    if (depth <= 0) {
      return {
        type: 'object',
        className: 'Object',
        summary: keys.length > 0 ? `Object {${keys.length} keys}` : '{}',
        keys,
        entries: [],
        truncated: false,
      }
    }
    const selectedKeys = keys.slice(0, MAX_WORKSPACE_JSON_PREVIEW_ENTRIES)
    const entries = selectedKeys.map((key) => ({
      key,
      value: buildWorkspaceStructuredPreview(record[key], depth - 1),
    }))
    const preview: WorkspaceStructuredPreview = {
      type: 'object',
      className: 'Object',
      summary: '',
      keys,
      entries,
      truncated: keys.length > MAX_WORKSPACE_JSON_PREVIEW_ENTRIES,
    }
    preview.summary = formatWorkspaceStructuredPreview(preview)
    return preview
  }
  return {
    type: typeof value,
    summary: formatWorkspaceJSONPrimitiveSummary(value),
    keys: [],
    entries: [],
    truncated: false,
  }
}

function safeParseWorkspaceJSON(value: string): unknown | undefined {
  try {
    return JSON.parse(value)
  } catch {
    return undefined
  }
}

function isLikelyWorkspaceJSONText(value: string): boolean {
  const trimmed = value.trim()
  if (!trimmed) {
    return false
  }
  return /^[\[{"]|^-?\d|^(true|false|null)\b/.test(trimmed)
}

function normalizeWorkspaceJSONCandidate(raw: string): string {
  const trimmed = raw.trim()
  if (!trimmed) {
    return ''
  }
  if (trimmed.startsWith('<')) {
    return trimmed.slice(1).trim()
  }
  return trimmed
}

export function tryParseWorkspaceJSONOutput(raw: string): WorkspaceParsedJSONOutput | null {
  const normalized = normalizeWorkspaceJSONCandidate(String(raw || ''))
  if (!isLikelyWorkspaceJSONText(normalized)) {
    return null
  }

  const parsed = safeParseWorkspaceJSON(normalized)
  if (typeof parsed === 'undefined') {
    return null
  }

  let resolvedRaw = normalized
  let resolvedValue = parsed
  if (typeof parsed === 'string') {
    const nested = parsed.trim()
    if (isLikelyWorkspaceJSONText(nested)) {
      const reparsed = safeParseWorkspaceJSON(nested)
      if (typeof reparsed !== 'undefined') {
        resolvedRaw = nested
        resolvedValue = reparsed
      }
    }
  }

  return {
    raw: resolvedRaw,
    preview: buildWorkspaceStructuredPreview(resolvedValue),
  }
}

export function formatWorkspaceJSONOutput(raw: string): string {
  const parsed = tryParseWorkspaceJSONOutput(raw)
  if (!parsed) {
    return String(raw || '')
  }
  try {
    return JSON.stringify(JSON.parse(parsed.raw), null, 2)
  } catch {
    return parsed.raw
  }
}
