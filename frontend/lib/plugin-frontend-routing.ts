export type PluginRouteMatch = {
  matched: boolean
  routeParams: Record<string, string>
}

export type PluginHostContextValue =
  | string
  | number
  | boolean
  | null
  | PluginHostContextValue[]
  | { [key: string]: PluginHostContextValue }

type SearchParamsLike = {
  entries(): IterableIterator<[string, string]>
}

export function normalizePluginPath(path: string): string {
  const trimmed = (path || '').trim()
  if (!trimmed) return '/'
  const withLeading = trimmed.startsWith('/') ? trimmed : `/${trimmed}`
  const segments = withLeading.split('/').reduce<string[]>((acc, segment, index) => {
    if (index === 0 || segment === '' || segment === '.') {
      return acc
    }
    if (segment === '..') {
      acc.pop()
      return acc
    }
    acc.push(segment)
    return acc
  }, [])
  return `/${segments.join('/')}` || '/'
}

export function normalizePluginStringMap(
  value?: Record<string, string> | null
): Record<string, string> {
  if (!value) {
    return {}
  }
  return Object.entries(value).reduce<Record<string, string>>((acc, [key, item]) => {
    const normalizedKey = String(key || '').trim()
    if (!normalizedKey) return acc
    acc[normalizedKey] = String(item ?? '')
    return acc
  }, {})
}

export function stringifyPluginStringMap(value?: Record<string, string> | null): string {
  const normalized = normalizePluginStringMap(value)
  const ordered = Object.keys(normalized)
    .sort()
    .reduce<Record<string, string>>((acc, key) => {
      acc[key] = normalized[key]
      return acc
    }, {})
  return JSON.stringify(ordered)
}

function normalizePluginHostContextValue(value: unknown): PluginHostContextValue | undefined {
  if (value === undefined) {
    return undefined
  }
  if (value === null) {
    return null
  }
  if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean') {
    return value
  }
  if (Array.isArray(value)) {
    return value
      .map((item) => normalizePluginHostContextValue(item))
      .filter((item): item is PluginHostContextValue => item !== undefined)
  }
  if (typeof value === 'object') {
    const entries = Object.entries(value as Record<string, unknown>)
      .map(([key, item]) => [String(key || '').trim(), normalizePluginHostContextValue(item)] as const)
      .filter((entry): entry is readonly [string, PluginHostContextValue] => {
        const [key, item] = entry
        return key.length > 0 && item !== undefined
      })
      .sort(([left], [right]) => left.localeCompare(right))

    return entries.reduce<Record<string, PluginHostContextValue>>((acc, [key, item]) => {
      acc[key] = item
      return acc
    }, {})
  }
  return undefined
}

export function normalizePluginHostContext(
  value?: Record<string, unknown> | null
): Record<string, PluginHostContextValue> {
  const normalized = normalizePluginHostContextValue(value)
  if (!normalized || Array.isArray(normalized) || typeof normalized !== 'object') {
    return {}
  }
  return normalized as Record<string, PluginHostContextValue>
}

export function stringifyPluginHostContext(value?: Record<string, unknown> | null): string {
  return JSON.stringify(normalizePluginHostContext(value))
}

function splitPluginPathSegments(path: string): string[] {
  const normalized = normalizePluginPath(path)
  if (normalized === '/') {
    return []
  }
  return normalized.slice(1).split('/')
}

export function buildPluginQueryString(queryParams?: Record<string, string> | null): string {
  const normalized = normalizePluginStringMap(queryParams)
  const query = new URLSearchParams()
  Object.keys(normalized)
    .sort()
    .forEach((key) => {
      query.append(key, normalized[key])
    })
  return query.toString()
}

export function buildPluginFullPath(
  path: string,
  queryParams?: Record<string, string> | null
): string {
  const normalizedPath = normalizePluginPath(path)
  const queryString = buildPluginQueryString(queryParams)
  if (!queryString) {
    return normalizedPath
  }
  return `${normalizedPath}?${queryString}`
}

export function buildPluginBootstrapContextKey(
  path: string,
  queryParams?: Record<string, string> | null
): string {
  return buildPluginFullPath(path, queryParams)
}

export function matchPluginRoute(routePath: string, currentPath: string): PluginRouteMatch {
  const normalizedRoute = normalizePluginPath(routePath)
  const normalizedCurrent = normalizePluginPath(currentPath)
  if (normalizedRoute === normalizedCurrent) {
    return { matched: true, routeParams: {} }
  }

  const routeSegments = splitPluginPathSegments(normalizedRoute)
  const currentSegments = splitPluginPathSegments(normalizedCurrent)
  const routeParams: Record<string, string> = {}

  for (
    let routeIndex = 0, currentIndex = 0;
    routeIndex < routeSegments.length;
    routeIndex += 1, currentIndex += 1
  ) {
    const routeSegment = routeSegments[routeIndex]
    if (routeSegment.startsWith('*')) {
      if (routeIndex !== routeSegments.length - 1 || currentIndex >= currentSegments.length) {
        return { matched: false, routeParams: {} }
      }
      const wildcardName = routeSegment.slice(1).trim()
      if (wildcardName) {
        routeParams[wildcardName] = currentSegments.slice(currentIndex).join('/')
      }
      return { matched: true, routeParams }
    }
    if (currentIndex >= currentSegments.length) {
      return { matched: false, routeParams: {} }
    }
    const currentSegment = currentSegments[currentIndex]
    if (routeSegment === currentSegment) {
      continue
    }
    if (routeSegment.startsWith(':') && routeSegment.length > 1) {
      routeParams[routeSegment.slice(1)] = currentSegment
      continue
    }
    return { matched: false, routeParams: {} }
  }

  if (routeSegments.length !== currentSegments.length) {
    return { matched: false, routeParams: {} }
  }
  return { matched: true, routeParams }
}

export function isPluginRouteMatch(routePath: string, currentPath: string): boolean {
  return matchPluginRoute(routePath, currentPath).matched
}

export function isPluginMenuPathActive(currentPath: string, itemPath: string): boolean {
  const normalizedCurrent = normalizePluginPath(currentPath)
  const normalizedItem = normalizePluginPath(itemPath)
  return normalizedCurrent === normalizedItem || normalizedCurrent.startsWith(`${normalizedItem}/`)
}

export function readPluginSearchParams(
  searchParams?: SearchParamsLike | null
): Record<string, string> {
  if (!searchParams) {
    return {}
  }
  return Array.from(searchParams.entries()).reduce<Record<string, string>>((acc, [key, value]) => {
    const normalizedKey = String(key || '').trim()
    if (!normalizedKey) return acc
    acc[normalizedKey] = String(value ?? '')
    return acc
  }, {})
}
