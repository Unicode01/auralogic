const PRODUCT_BROWSE_STATE_KEY = 'auralogic_product_browse_state_v1'
const PRODUCT_BROWSE_STATE_TTL_MS = 1000 * 60 * 30

export const productListFocusParamKey = 'focus_product'

export interface ProductBrowseState {
  listPath: string
  scrollTop: number
  focusedProductId?: number
}

type StoredProductBrowseState = ProductBrowseState & {
  ts: number
}

function isBrowser(): boolean {
  return typeof window !== 'undefined'
}

function normalizePositiveInt(value: number | undefined): number | undefined {
  if (!Number.isFinite(value)) return undefined
  const normalized = Math.floor(Number(value))
  return normalized > 0 ? normalized : undefined
}

function normalizeScrollTop(value: number | undefined): number {
  if (!Number.isFinite(value)) return 0
  return Math.max(0, Number(value))
}

function normalizeProductListPath(path: string | undefined | null): string | null {
  const trimmed = typeof path === 'string' ? path.trim() : ''
  if (!trimmed.startsWith('/products') || trimmed.startsWith('//')) {
    return null
  }

  try {
    const url = new URL(trimmed, 'https://auralogic.local')
    if (url.pathname !== '/products') {
      return null
    }
    url.searchParams.delete(productListFocusParamKey)
    return `${url.pathname}${url.search}${url.hash}`
  } catch {
    return null
  }
}

function validateStoredState(value: unknown): StoredProductBrowseState | null {
  if (!value || typeof value !== 'object') return null

  const candidate = value as Partial<StoredProductBrowseState>
  const listPath = normalizeProductListPath(candidate.listPath)
  if (!listPath || !Number.isFinite(candidate.ts)) {
    return null
  }

  return {
    listPath,
    scrollTop: normalizeScrollTop(candidate.scrollTop),
    ts: Number(candidate.ts),
    ...(normalizePositiveInt(candidate.focusedProductId)
      ? {
          focusedProductId: normalizePositiveInt(candidate.focusedProductId),
        }
      : {}),
  }
}

export function readProductBrowseState(): ProductBrowseState | null {
  if (!isBrowser()) return null

  try {
    const raw = sessionStorage.getItem(PRODUCT_BROWSE_STATE_KEY)
    if (!raw) return null

    const parsed = validateStoredState(JSON.parse(raw))
    if (!parsed) {
      clearProductBrowseState()
      return null
    }

    if (Date.now() - parsed.ts > PRODUCT_BROWSE_STATE_TTL_MS) {
      clearProductBrowseState()
      return null
    }

    const { ts: _ts, ...state } = parsed
    return state
  } catch {
    clearProductBrowseState()
    return null
  }
}

export function setProductBrowseState(state: ProductBrowseState): void {
  if (!isBrowser()) return

  const listPath = normalizeProductListPath(state.listPath)
  if (!listPath) {
    clearProductBrowseState()
    return
  }

  const payload: StoredProductBrowseState = {
    listPath,
    scrollTop: normalizeScrollTop(state.scrollTop),
    ts: Date.now(),
    ...(normalizePositiveInt(state.focusedProductId)
      ? {
          focusedProductId: normalizePositiveInt(state.focusedProductId),
        }
      : {}),
  }

  sessionStorage.setItem(PRODUCT_BROWSE_STATE_KEY, JSON.stringify(payload))
}

export function clearProductBrowseState(): void {
  if (!isBrowser()) return
  sessionStorage.removeItem(PRODUCT_BROWSE_STATE_KEY)
}

export function buildProductListReturnPath(
  listPath: string | undefined | null,
  focusedProductId?: number
): string {
  const normalizedPath = normalizeProductListPath(listPath) || '/products'
  const normalizedFocusedProductId = normalizePositiveInt(focusedProductId)
  if (!normalizedFocusedProductId) {
    return normalizedPath
  }

  // Focus is restored from session state; the URL stays clean after returning.
  return normalizedPath
}

export function parseFocusedProductIdQuery(value: string | null | undefined): number | undefined {
  if (value === null || value === undefined) return undefined
  return normalizePositiveInt(Number(value))
}

export function stripProductListFocusFromPath(path: string | undefined | null): string {
  return normalizeProductListPath(path) || '/products'
}
