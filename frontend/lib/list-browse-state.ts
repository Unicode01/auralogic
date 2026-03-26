const LIST_BROWSE_STATE_PREFIX = 'auralogic_list_browse_state_v1'
const LIST_BROWSE_STATE_TTL_MS = 1000 * 60 * 30

const LIST_SCOPE_CONFIG = {
  orders: {
    pathname: '/orders',
    focusParamKey: 'focus_order',
  },
  tickets: {
    pathname: '/tickets',
    focusParamKey: 'focus_ticket',
  },
} as const

export type ListBrowseScope = keyof typeof LIST_SCOPE_CONFIG

export interface ListBrowseState {
  listPath: string
  scrollTop: number
  focusedItemKey?: string
}

type StoredListBrowseState = ListBrowseState & {
  ts: number
}

function isBrowser(): boolean {
  return typeof window !== 'undefined'
}

function getScopeConfig(scope: ListBrowseScope) {
  return LIST_SCOPE_CONFIG[scope]
}

function getStorageKey(scope: ListBrowseScope): string {
  return `${LIST_BROWSE_STATE_PREFIX}:${scope}`
}

function normalizeFocusedItemKey(value: string | undefined | null): string | undefined {
  if (typeof value !== 'string') return undefined
  const trimmed = value.trim()
  return trimmed ? trimmed : undefined
}

function normalizeScrollTop(value: number | undefined): number {
  if (!Number.isFinite(value)) return 0
  return Math.max(0, Number(value))
}

function normalizeListPath(scope: ListBrowseScope, path: string | undefined | null): string | null {
  const trimmed = typeof path === 'string' ? path.trim() : ''
  if (!trimmed.startsWith('/') || trimmed.startsWith('//')) {
    return null
  }

  const config = getScopeConfig(scope)

  try {
    const url = new URL(trimmed, 'https://auralogic.local')
    if (url.pathname !== config.pathname) {
      return null
    }
    url.searchParams.delete(config.focusParamKey)
    return `${url.pathname}${url.search}${url.hash}`
  } catch {
    return null
  }
}

function validateStoredState(
  scope: ListBrowseScope,
  value: unknown
): StoredListBrowseState | null {
  if (!value || typeof value !== 'object') return null

  const candidate = value as Partial<StoredListBrowseState>
  const listPath = normalizeListPath(scope, candidate.listPath)
  if (!listPath || !Number.isFinite(candidate.ts)) {
    return null
  }

  return {
    listPath,
    scrollTop: normalizeScrollTop(candidate.scrollTop),
    ts: Number(candidate.ts),
    ...(normalizeFocusedItemKey(candidate.focusedItemKey)
      ? {
          focusedItemKey: normalizeFocusedItemKey(candidate.focusedItemKey),
        }
      : {}),
  }
}

export function getListFocusParamKey(scope: ListBrowseScope): string {
  return getScopeConfig(scope).focusParamKey
}

export function readListBrowseState(scope: ListBrowseScope): ListBrowseState | null {
  if (!isBrowser()) return null

  try {
    const raw = sessionStorage.getItem(getStorageKey(scope))
    if (!raw) return null

    const parsed = validateStoredState(scope, JSON.parse(raw))
    if (!parsed) {
      clearListBrowseState(scope)
      return null
    }

    if (Date.now() - parsed.ts > LIST_BROWSE_STATE_TTL_MS) {
      clearListBrowseState(scope)
      return null
    }

    const { ts: _ts, ...state } = parsed
    return state
  } catch {
    clearListBrowseState(scope)
    return null
  }
}

export function setListBrowseState(scope: ListBrowseScope, state: ListBrowseState): void {
  if (!isBrowser()) return

  const listPath = normalizeListPath(scope, state.listPath)
  if (!listPath) {
    clearListBrowseState(scope)
    return
  }

  const payload: StoredListBrowseState = {
    listPath,
    scrollTop: normalizeScrollTop(state.scrollTop),
    ts: Date.now(),
    ...(normalizeFocusedItemKey(state.focusedItemKey)
      ? {
          focusedItemKey: normalizeFocusedItemKey(state.focusedItemKey),
        }
      : {}),
  }

  sessionStorage.setItem(getStorageKey(scope), JSON.stringify(payload))
}

export function clearListBrowseState(scope: ListBrowseScope): void {
  if (!isBrowser()) return
  sessionStorage.removeItem(getStorageKey(scope))
}

export function parseFocusedListItemQuery(value: string | null | undefined): string | undefined {
  return normalizeFocusedItemKey(value)
}

export function stripListFocusFromPath(scope: ListBrowseScope, path: string | undefined | null): string {
  return normalizeListPath(scope, path) || getScopeConfig(scope).pathname
}

export function buildListReturnPath(
  scope: ListBrowseScope,
  listPath: string | undefined | null,
  focusedItemKey?: string
): string {
  const normalizedPath = normalizeListPath(scope, listPath) || getScopeConfig(scope).pathname
  const normalizedFocusedItemKey = normalizeFocusedItemKey(focusedItemKey)
  if (!normalizedFocusedItemKey) {
    return normalizedPath
  }

  // Focus is restored from session state; the URL stays clean after returning.
  return normalizedPath
}
