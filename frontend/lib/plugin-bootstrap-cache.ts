import type { PluginFrontendBootstrapMenuItem } from '@/lib/api'

const ADMIN_BOOTSTRAP_MENU_CACHE_KEY_PREFIX = 'auralogic_plugin_bootstrap_admin_menu_v3'
const USER_BOOTSTRAP_MENU_CACHE_KEY_PREFIX = 'auralogic_plugin_bootstrap_user_menu_v3'
const ADMIN_BOOTSTRAP_MENU_CACHE_LEGACY_KEYS = [
  'auralogic_plugin_bootstrap_admin_menu_v2',
  'auralogic_plugin_bootstrap_admin_menu_v1',
]
const USER_BOOTSTRAP_MENU_CACHE_LEGACY_KEYS = [
  'auralogic_plugin_bootstrap_user_menu_v2',
  'auralogic_plugin_bootstrap_user_menu_v1',
]
const BOOTSTRAP_MENU_CACHE_TTL_MS = 10 * 60 * 1000

type BootstrapMenuCachePayload = {
  ts: number
  menus: PluginFrontendBootstrapMenuItem[]
}

export type CachedBootstrapMenusResult = {
  found: boolean
  menus: PluginFrontendBootstrapMenuItem[]
}

function normalizeMenuItems(value: unknown): PluginFrontendBootstrapMenuItem[] {
  if (!Array.isArray(value)) return []
  return value.filter(
    (item) => !!item && typeof item === 'object'
  ) as PluginFrontendBootstrapMenuItem[]
}

function normalizeBootstrapCacheScopeKey(scopeKey?: string): string {
  const normalized = String(scopeKey || 'default')
    .trim()
    .toLowerCase()
  if (!normalized) return 'default'
  return encodeURIComponent(normalized)
}

function normalizeBootstrapCacheContextKey(contextKey?: string): string {
  const normalized = String(contextKey || 'default').trim()
  if (!normalized) return 'default'
  return encodeURIComponent(normalized)
}

function buildScopedCacheKey(
  cacheKeyPrefix: string,
  scopeKey?: string,
  contextKey?: string
): string {
  return `${cacheKeyPrefix}:${normalizeBootstrapCacheScopeKey(scopeKey)}:${normalizeBootstrapCacheContextKey(contextKey)}`
}

function readCachedMenus(
  cacheKeyPrefix: string,
  scopeKey?: string,
  contextKey?: string
): PluginFrontendBootstrapMenuItem[] {
  return readCachedMenusResult(cacheKeyPrefix, scopeKey, contextKey).menus
}

function readCachedMenusResult(
  cacheKeyPrefix: string,
  scopeKey?: string,
  contextKey?: string
): CachedBootstrapMenusResult {
  if (typeof window === 'undefined') {
    return {
      found: false,
      menus: [],
    }
  }

  try {
    const raw = localStorage.getItem(buildScopedCacheKey(cacheKeyPrefix, scopeKey, contextKey))
    if (!raw) {
      return {
        found: false,
        menus: [],
      }
    }

    const parsed = JSON.parse(raw)
    if (Array.isArray(parsed)) {
      return {
        found: true,
        menus: normalizeMenuItems(parsed),
      }
    }
    if (!parsed || typeof parsed !== 'object') {
      return {
        found: false,
        menus: [],
      }
    }

    const payload = parsed as BootstrapMenuCachePayload
    if (typeof payload.ts === 'number' && Date.now() - payload.ts > BOOTSTRAP_MENU_CACHE_TTL_MS) {
      localStorage.removeItem(buildScopedCacheKey(cacheKeyPrefix, scopeKey, contextKey))
      return {
        found: false,
        menus: [],
      }
    }
    return {
      found: true,
      menus: normalizeMenuItems(payload.menus),
    }
  } catch {
    return {
      found: false,
      menus: [],
    }
  }
}

function writeCachedMenus(
  cacheKeyPrefix: string,
  menus: PluginFrontendBootstrapMenuItem[],
  scopeKey?: string,
  contextKey?: string
) {
  if (typeof window === 'undefined') return

  try {
    const payload: BootstrapMenuCachePayload = {
      ts: Date.now(),
      menus: normalizeMenuItems(menus),
    }
    localStorage.setItem(
      buildScopedCacheKey(cacheKeyPrefix, scopeKey, contextKey),
      JSON.stringify(payload)
    )
  } catch {
    // ignore storage failures
  }
}

function removeCachedMenus(
  cacheKeyPrefix: string,
  legacyKeys: string[],
  scopeKey?: string,
  contextKey?: string
) {
  if (typeof window === 'undefined') return

  try {
    if (
      typeof scopeKey === 'string' &&
      scopeKey.trim() !== '' &&
      typeof contextKey === 'string' &&
      contextKey.trim() !== ''
    ) {
      localStorage.removeItem(buildScopedCacheKey(cacheKeyPrefix, scopeKey, contextKey))
      return
    }

    const normalizedScopeKey =
      typeof scopeKey === 'string' && scopeKey.trim() !== ''
        ? normalizeBootstrapCacheScopeKey(scopeKey)
        : ''
    const keysToRemove: string[] = []
    for (let index = 0; index < localStorage.length; index += 1) {
      const key = localStorage.key(index)
      if (!key) continue
      const matchesCurrentPrefix = key.startsWith(`${cacheKeyPrefix}:`)
      const matchesLegacyPrefix = legacyKeys.some((legacyKey) => key.startsWith(`${legacyKey}:`))
      if (!matchesCurrentPrefix && !matchesLegacyPrefix) {
        continue
      }
      if (!normalizedScopeKey) {
        keysToRemove.push(key)
        continue
      }
      if (
        key === `${cacheKeyPrefix}:${normalizedScopeKey}` ||
        key.startsWith(`${cacheKeyPrefix}:${normalizedScopeKey}:`) ||
        legacyKeys.some(
          (legacyKey) =>
            key === `${legacyKey}:${normalizedScopeKey}` ||
            key.startsWith(`${legacyKey}:${normalizedScopeKey}:`)
        )
      ) {
        keysToRemove.push(key)
      }
    }
    keysToRemove.forEach((key) => localStorage.removeItem(key))
  } catch {
    // ignore storage failures
  }
}

export function extractBootstrapMenus(payload: unknown): PluginFrontendBootstrapMenuItem[] {
  const root = payload as any
  const source = Array.isArray(root?.data?.menus)
    ? root.data.menus
    : Array.isArray(root?.menus)
      ? root.menus
      : []
  return normalizeMenuItems(source)
}

export function getCachedAdminBootstrapMenus(
  scopeKey?: string,
  contextKey?: string
): PluginFrontendBootstrapMenuItem[] {
  return readCachedMenus(ADMIN_BOOTSTRAP_MENU_CACHE_KEY_PREFIX, scopeKey, contextKey)
}

export function getCachedAdminBootstrapMenusResult(
  scopeKey?: string,
  contextKey?: string
): CachedBootstrapMenusResult {
  return readCachedMenusResult(ADMIN_BOOTSTRAP_MENU_CACHE_KEY_PREFIX, scopeKey, contextKey)
}

export function setCachedAdminBootstrapMenus(
  menus: PluginFrontendBootstrapMenuItem[],
  scopeKey?: string,
  contextKey?: string
) {
  writeCachedMenus(ADMIN_BOOTSTRAP_MENU_CACHE_KEY_PREFIX, menus, scopeKey, contextKey)
}

export function clearCachedAdminBootstrapMenus(scopeKey?: string, contextKey?: string) {
  removeCachedMenus(
    ADMIN_BOOTSTRAP_MENU_CACHE_KEY_PREFIX,
    ADMIN_BOOTSTRAP_MENU_CACHE_LEGACY_KEYS,
    scopeKey,
    contextKey
  )
}

export function getCachedUserBootstrapMenus(
  scopeKey?: string,
  contextKey?: string
): PluginFrontendBootstrapMenuItem[] {
  return readCachedMenus(USER_BOOTSTRAP_MENU_CACHE_KEY_PREFIX, scopeKey, contextKey)
}

export function getCachedUserBootstrapMenusResult(
  scopeKey?: string,
  contextKey?: string
): CachedBootstrapMenusResult {
  return readCachedMenusResult(USER_BOOTSTRAP_MENU_CACHE_KEY_PREFIX, scopeKey, contextKey)
}

export function setCachedUserBootstrapMenus(
  menus: PluginFrontendBootstrapMenuItem[],
  scopeKey?: string,
  contextKey?: string
) {
  writeCachedMenus(USER_BOOTSTRAP_MENU_CACHE_KEY_PREFIX, menus, scopeKey, contextKey)
}

export function clearCachedUserBootstrapMenus(scopeKey?: string, contextKey?: string) {
  removeCachedMenus(
    USER_BOOTSTRAP_MENU_CACHE_KEY_PREFIX,
    USER_BOOTSTRAP_MENU_CACHE_LEGACY_KEYS,
    scopeKey,
    contextKey
  )
}

export function clearCachedBootstrapMenus() {
  clearCachedAdminBootstrapMenus()
  clearCachedUserBootstrapMenus()
}
