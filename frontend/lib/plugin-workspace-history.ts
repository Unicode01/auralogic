export const DEFAULT_PLUGIN_WORKSPACE_HISTORY_LIMIT = 80

export type PluginWorkspaceHistoryDirection = 'previous' | 'next'

export type PluginWorkspaceHistoryNavigationResult = {
  index: number
  value: string
  draft: string
}

export function shouldHandlePluginWorkspaceHistoryNavigation(payload: {
  value: string
  selectionStart: number
  selectionEnd?: number
  direction: PluginWorkspaceHistoryDirection
}): boolean {
  const value = String(payload.value || '')
  const selectionStart = Math.max(0, Math.min(value.length, Math.trunc(payload.selectionStart || 0)))
  const selectionEnd = Math.max(
    selectionStart,
    Math.min(value.length, Math.trunc(payload.selectionEnd ?? selectionStart))
  )
  if (selectionStart !== selectionEnd) {
    return false
  }
  if (payload.direction === 'previous') {
    return !value.slice(0, selectionStart).includes('\n')
  }
  return !value.slice(selectionStart).includes('\n')
}

export function buildPluginWorkspaceHistoryStorageKey(pluginId?: number | null): string {
  const normalizedPluginID = Number(pluginId || 0)
  if (!Number.isFinite(normalizedPluginID) || normalizedPluginID <= 0) {
    return ''
  }
  return `auralogic.plugin-workspace.history.${Math.trunc(normalizedPluginID)}`
}

export function normalizePluginWorkspaceHistoryEntries(
  value: unknown,
  limit = DEFAULT_PLUGIN_WORKSPACE_HISTORY_LIMIT
): string[] {
  if (!Array.isArray(value) || limit <= 0) {
    return []
  }
  const normalizedLimit = Math.max(1, Math.trunc(limit))
  const seen = new Set<string>()
  const out: string[] = []
  value.forEach((item) => {
    const normalized = String(item || '').trim()
    if (!normalized || seen.has(normalized)) {
      return
    }
    seen.add(normalized)
    out.push(normalized)
  })
  return out.slice(-normalizedLimit)
}

export function parsePluginWorkspaceHistoryStorage(
  raw: string | null | undefined,
  limit = DEFAULT_PLUGIN_WORKSPACE_HISTORY_LIMIT
): string[] {
  const value = String(raw || '').trim()
  if (!value) {
    return []
  }
  try {
    return normalizePluginWorkspaceHistoryEntries(JSON.parse(value), limit)
  } catch {
    return []
  }
}

export function pushPluginWorkspaceHistoryEntry(
  entries: string[],
  value: string,
  limit = DEFAULT_PLUGIN_WORKSPACE_HISTORY_LIMIT
): string[] {
  const normalizedValue = String(value || '').trim()
  if (!normalizedValue) {
    return normalizePluginWorkspaceHistoryEntries(entries, limit)
  }
  const nextEntries = normalizePluginWorkspaceHistoryEntries(entries, limit).filter(
    (entry) => entry !== normalizedValue
  )
  nextEntries.push(normalizedValue)
  return normalizePluginWorkspaceHistoryEntries(nextEntries, limit)
}

export function resolvePluginWorkspaceHistoryNavigation(payload: {
  entries: string[]
  currentValue: string
  index: number
  draft: string
  direction: PluginWorkspaceHistoryDirection
}): PluginWorkspaceHistoryNavigationResult {
  const entries = normalizePluginWorkspaceHistoryEntries(payload.entries)
  const currentValue = String(payload.currentValue || '')
  const currentIndex = Math.trunc(payload.index || 0)
  const currentDraft = String(payload.draft || '')

  if (entries.length === 0) {
    return {
      index: -1,
      value: currentValue,
      draft: currentDraft,
    }
  }

  if (payload.direction === 'previous') {
    const nextDraft = currentIndex >= 0 ? currentDraft : currentValue
    const nextIndex = currentIndex >= 0 ? Math.max(0, currentIndex - 1) : entries.length - 1
    return {
      index: nextIndex,
      value: entries[nextIndex] || nextDraft,
      draft: nextDraft,
    }
  }

  if (currentIndex < 0) {
    return {
      index: -1,
      value: currentValue,
      draft: currentDraft,
    }
  }

  if (currentIndex >= entries.length - 1) {
    return {
      index: -1,
      value: currentDraft,
      draft: currentDraft,
    }
  }

  const nextIndex = currentIndex + 1
  return {
    index: nextIndex,
    value: entries[nextIndex] || currentDraft,
    draft: currentDraft,
  }
}
