type BootstrapPageSchema = {
  host_market_workspace?: boolean | string
}

type BootstrapPathItem = {
  path?: string
  page?: BootstrapPageSchema
}

type BootstrapPayload = {
  data?: {
    menus?: BootstrapPathItem[]
    routes?: BootstrapPathItem[]
  }
  menus?: BootstrapPathItem[]
  routes?: BootstrapPathItem[]
}

const ADMIN_MARKET_PATH = '/admin/plugin-pages/market'

function extractBootstrapRouteItems(payload: unknown): BootstrapPathItem[] {
  const root = payload as BootstrapPayload
  if (Array.isArray(root?.data?.routes)) {
    return root.data.routes
  }
  if (Array.isArray(root?.routes)) {
    return root.routes
  }
  return []
}

function isTrueLike(value: unknown): boolean {
  if (value === true) return true
  return String(value || '')
    .trim()
    .toLowerCase() === 'true'
}

function isAdminHostMarketWorkspaceRoute(item: BootstrapPathItem | null | undefined): boolean {
  const path = String(item?.path || '').trim()
  if (!path.startsWith('/admin/plugin-pages/')) {
    return false
  }
  return isTrueLike(item?.page?.host_market_workspace)
}

function extractBootstrapPaths(value: unknown): string[] {
  if (!Array.isArray(value)) return []
  return value
    .map((item) => String((item as BootstrapPathItem | null | undefined)?.path || '').trim())
    .filter((item) => item.length > 0)
}

export function extractBootstrapMenuPaths(payload: unknown): string[] {
  const root = payload as BootstrapPayload
  return extractBootstrapPaths(
    Array.isArray(root?.data?.menus)
      ? root.data.menus
      : Array.isArray(root?.menus)
        ? root.menus
        : []
  )
}

export function extractBootstrapRoutePaths(payload: unknown): string[] {
  const root = payload as BootstrapPayload
  return extractBootstrapPaths(
    Array.isArray(root?.data?.routes)
      ? root.data.routes
      : Array.isArray(root?.routes)
        ? root.routes
        : []
  )
}

export function findAdminMarketPluginBasePath(payload: unknown): string {
  const hostMarketRoute = extractBootstrapRouteItems(payload).find(isAdminHostMarketWorkspaceRoute)
  if (hostMarketRoute?.path) {
    return String(hostMarketRoute.path).trim()
  }

  return (
    [...extractBootstrapMenuPaths(payload), ...extractBootstrapRoutePaths(payload)].find(
      (path) => path === ADMIN_MARKET_PATH || path.startsWith(`${ADMIN_MARKET_PATH}?`)
    ) || ''
  )
}

export function buildAdminMarketPluginPageHref(
  basePath: string,
  extraQuery?: Record<string, string | null | undefined>
): string {
  const trimmed = String(basePath || '').trim()
  if (!trimmed) return ''

  const [pathname, queryString = ''] = trimmed.split('?', 2)
  const params = new URLSearchParams(queryString)
  Object.entries(extraQuery || {}).forEach(([key, value]) => {
    const normalized = String(value || '').trim()
    if (!normalized) {
      params.delete(key)
      return
    }
    params.set(key, normalized)
  })
  const nextQuery = params.toString()
  return nextQuery ? `${pathname}?${nextQuery}` : pathname
}
