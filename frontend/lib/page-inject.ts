export const PAGE_INJECT_CACHE_KEY = 'auralogic-page-inject:v2'
const LEGACY_PAGE_INJECT_CACHE_KEYS = ['auralogic-page-inject']

export const DEFAULT_PAGE_INJECT_TTL = 5 * 60 * 1000

export interface PageInjectRule {
  name?: string
  pattern?: string
  match_type?: string
  css?: string
  js?: string
}

export interface PageInjectPayload {
  css: string
  js: string
  rules: PageInjectRule[]
}

export interface PageInjectCacheEntry extends PageInjectPayload {
  ts: number
}

export type PageInjectCache = Record<string, PageInjectCacheEntry>

function isRecordLike(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}

function normalizePageInjectRule(raw: unknown): PageInjectRule | null {
  if (!isRecordLike(raw)) {
    return null
  }
  return {
    name: typeof raw.name === 'string' ? raw.name : undefined,
    pattern: typeof raw.pattern === 'string' ? raw.pattern : undefined,
    match_type: typeof raw.match_type === 'string' ? raw.match_type : undefined,
    css: typeof raw.css === 'string' ? raw.css : '',
    js: typeof raw.js === 'string' ? raw.js : '',
  }
}

export function normalizePageInjectPayload(raw: unknown): PageInjectPayload {
  const record = isRecordLike(raw) ? raw : {}
  const rules = Array.isArray(record.rules)
    ? record.rules
        .map((item) => normalizePageInjectRule(item))
        .filter((item): item is PageInjectRule => item !== null)
    : []

  return {
    css: typeof record.css === 'string' ? record.css : '',
    js: typeof record.js === 'string' ? record.js : '',
    rules,
  }
}

export function normalizePageInjectCacheEntry(raw: unknown): PageInjectCacheEntry | null {
  const record = isRecordLike(raw) ? raw : null
  if (!record) {
    return null
  }

  const payload = normalizePageInjectPayload(record)
  const ts = typeof record.ts === 'number' && Number.isFinite(record.ts) ? record.ts : NaN
  if (!Number.isFinite(ts) || ts < 0) {
    return null
  }

  return {
    ...payload,
    ts,
  }
}

export function prunePageInjectCache(
  cache: PageInjectCache,
  now: number = Date.now(),
  ttl: number = DEFAULT_PAGE_INJECT_TTL
): PageInjectCache {
  const pruned: PageInjectCache = {}
  for (const [path, entry] of Object.entries(cache)) {
    if (!entry || typeof path !== 'string' || !path) {
      continue
    }
    if (now - entry.ts >= ttl) {
      continue
    }
    pruned[path] = entry
  }
  return pruned
}

export function clearStoredPageInjectCache(
  storage: Pick<Storage, 'removeItem'> | undefined | null
) {
  if (!storage) {
    return
  }
  for (const key of [PAGE_INJECT_CACHE_KEY, ...LEGACY_PAGE_INJECT_CACHE_KEYS]) {
    storage.removeItem(key)
  }
}

export function readStoredPageInjectCache(
  storage: Pick<Storage, 'getItem' | 'removeItem'> | undefined | null,
  now: number = Date.now(),
  ttl: number = DEFAULT_PAGE_INJECT_TTL
): PageInjectCache {
  if (!storage) {
    return {}
  }

  for (const key of LEGACY_PAGE_INJECT_CACHE_KEYS) {
    storage.removeItem(key)
  }

  const raw = storage.getItem(PAGE_INJECT_CACHE_KEY)
  if (!raw) {
    return {}
  }

  try {
    const parsed = JSON.parse(raw)
    if (!isRecordLike(parsed)) {
      storage.removeItem(PAGE_INJECT_CACHE_KEY)
      return {}
    }

    const cache: PageInjectCache = {}
    for (const [path, value] of Object.entries(parsed)) {
      const entry = normalizePageInjectCacheEntry(value)
      if (!entry || !path) {
        continue
      }
      cache[path] = entry
    }
    return prunePageInjectCache(cache, now, ttl)
  } catch {
    storage.removeItem(PAGE_INJECT_CACHE_KEY)
    return {}
  }
}

export function writeStoredPageInjectCache(
  storage: Pick<Storage, 'setItem' | 'removeItem'> | undefined | null,
  cache: PageInjectCache,
  now: number = Date.now(),
  ttl: number = DEFAULT_PAGE_INJECT_TTL
) {
  if (!storage) {
    return
  }

  const pruned = prunePageInjectCache(cache, now, ttl)
  if (Object.keys(pruned).length === 0) {
    storage.removeItem(PAGE_INJECT_CACHE_KEY)
    return
  }

  storage.setItem(PAGE_INJECT_CACHE_KEY, JSON.stringify(pruned))
}

export function isSamePageInjectPayload(a: PageInjectPayload, b: PageInjectPayload): boolean {
  return JSON.stringify(normalizePageInjectPayload(a)) === JSON.stringify(normalizePageInjectPayload(b))
}

export function buildPageInjectRuntimeScript(source: string, label: string): string {
  const serializedSource = JSON.stringify(String(source || ''))
  const serializedLabel = JSON.stringify(String(label || 'page-inject'))

  return [
    '(() => {',
    `  const source = ${serializedSource};`,
    `  const label = ${serializedLabel};`,
    '  try {',
    '    new Function(source).call(window);',
    '  } catch (error) {',
    "    console.error('[AuraLogic] Page inject JS failed (' + label + ')', error);",
    '  }',
    '})();',
  ].join('\n')
}
