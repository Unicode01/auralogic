const AUTH_RETURN_STATE_KEY = 'auralogic_auth_return_state_v1'
const AUTH_RETURN_STATE_TTL_MS = 1000 * 60 * 30

export interface AuthReturnState {
  redirectPath: string
  cart?: {
    selectedGuestKeys: string[]
    scrollTop: number
  }
  product?: {
    productId: number
    selectedAttributes: Record<string, string>
    quantity: number
    scrollTop: number
  }
}

type StoredAuthReturnState = AuthReturnState & {
  ts: number
}

function isBrowser(): boolean {
  return typeof window !== 'undefined'
}

function normalizeRedirectPath(path: string): string | null {
  const trimmed = typeof path === 'string' ? path.trim() : ''
  if (!trimmed.startsWith('/') || trimmed.startsWith('//')) {
    return null
  }
  return trimmed
}

function normalizeSelectedGuestKeys(keys?: string[]): string[] {
  if (!Array.isArray(keys)) return []

  const normalized = new Set<string>()
  for (const key of keys) {
    if (typeof key !== 'string') continue
    const trimmed = key.trim()
    if (!trimmed) continue
    normalized.add(trimmed)
  }
  return Array.from(normalized)
}

function normalizeStringMap(map?: Record<string, string>): Record<string, string> {
  if (!map || typeof map !== 'object') return {}

  const normalized: Record<string, string> = {}
  for (const key of Object.keys(map).sort()) {
    const value = map[key]
    if (value === undefined || value === null) continue
    const normalizedKey = String(key).trim()
    const normalizedValue = String(value).trim()
    if (!normalizedKey || !normalizedValue) continue
    normalized[normalizedKey] = normalizedValue
  }
  return normalized
}

function normalizeScrollTop(value: number | undefined): number {
  if (!Number.isFinite(value)) return 0
  return Math.max(0, Number(value))
}

function normalizePositiveInt(value: number | undefined, fallback = 1): number {
  if (!Number.isFinite(value)) return fallback
  return Math.max(fallback, Math.floor(Number(value)))
}

function normalizeProductState(
  product?: AuthReturnState['product']
): AuthReturnState['product'] | undefined {
  if (!product || !Number.isFinite(product.productId) || Number(product.productId) <= 0) {
    return undefined
  }

  return {
    productId: Math.floor(Number(product.productId)),
    selectedAttributes: normalizeStringMap(product.selectedAttributes),
    quantity: normalizePositiveInt(product.quantity, 1),
    scrollTop: normalizeScrollTop(product.scrollTop),
  }
}

function validateStoredState(value: unknown): StoredAuthReturnState | null {
  if (!value || typeof value !== 'object') return null

  const candidate = value as Partial<StoredAuthReturnState>
  const redirectPath = normalizeRedirectPath(candidate.redirectPath || '')
  if (!redirectPath || !Number.isFinite(candidate.ts)) return null

  return {
    redirectPath,
    ts: Number(candidate.ts),
    ...(candidate.cart
      ? {
          cart: {
            selectedGuestKeys: normalizeSelectedGuestKeys(candidate.cart.selectedGuestKeys),
            scrollTop: normalizeScrollTop(candidate.cart.scrollTop),
          },
        }
      : {}),
    ...(normalizeProductState(candidate.product)
      ? {
          product: normalizeProductState(candidate.product),
        }
      : {}),
  }
}

export function readAuthReturnState(): AuthReturnState | null {
  if (!isBrowser()) return null

  try {
    const raw = sessionStorage.getItem(AUTH_RETURN_STATE_KEY)
    if (!raw) return null

    const parsed = validateStoredState(JSON.parse(raw))
    if (!parsed) {
      clearAuthReturnState()
      return null
    }

    if (Date.now() - parsed.ts > AUTH_RETURN_STATE_TTL_MS) {
      clearAuthReturnState()
      return null
    }

    const { ts: _ts, ...state } = parsed
    return state
  } catch {
    clearAuthReturnState()
    return null
  }
}

export function setAuthReturnState(state: AuthReturnState): void {
  if (!isBrowser()) return

  const redirectPath = normalizeRedirectPath(state.redirectPath)
  if (!redirectPath) {
    clearAuthReturnState()
    return
  }

  const payload: StoredAuthReturnState = {
    redirectPath,
    ts: Date.now(),
    ...(state.cart
      ? {
          cart: {
            selectedGuestKeys: normalizeSelectedGuestKeys(state.cart.selectedGuestKeys),
            scrollTop: normalizeScrollTop(state.cart.scrollTop),
          },
        }
      : {}),
    ...(normalizeProductState(state.product)
      ? {
          product: normalizeProductState(state.product),
        }
      : {}),
  }

  sessionStorage.setItem(AUTH_RETURN_STATE_KEY, JSON.stringify(payload))
}

export function clearAuthReturnState(): void {
  if (!isBrowser()) return
  sessionStorage.removeItem(AUTH_RETURN_STATE_KEY)
}
