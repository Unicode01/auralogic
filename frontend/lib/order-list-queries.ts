import { getOrders } from '@/lib/api'
import { normalizePositivePageQuery, normalizeQueryString } from '@/lib/query-state'
import type { OrderQueryParams } from '@/types/order'

type OrderListSearchParamSource =
  | URLSearchParams
  | Record<string, string | string[] | undefined>
  | undefined

export const ORDER_LIST_LIMIT = 18

function readSearchParam(source: OrderListSearchParamSource, name: string): string | undefined {
  if (!source) {
    return undefined
  }
  if (source instanceof URLSearchParams) {
    return source.get(name) || undefined
  }
  const value = source[name]
  return Array.isArray(value) ? value[0] : value
}

export function normalizeOrderListQueryParams(params: Partial<OrderQueryParams> = {}): Required<
  Pick<OrderQueryParams, 'page' | 'limit'>
> & {
  search: string
  status?: string
} {
  return {
    page:
      typeof params.page === 'number' && Number.isInteger(params.page) && params.page > 0
        ? params.page
        : 1,
    limit:
      typeof params.limit === 'number' && Number.isInteger(params.limit) && params.limit > 0
        ? params.limit
        : ORDER_LIST_LIMIT,
    search: normalizeQueryString(params.search),
    status: normalizeQueryString(params.status) || undefined,
  }
}

export function parseOrderListSearchParams(source: OrderListSearchParamSource): {
  page: number
  search: string
  status?: string
} {
  return {
    page: normalizePositivePageQuery(readSearchParam(source, 'page')),
    search: normalizeQueryString(readSearchParam(source, 'search')),
    status: normalizeQueryString(readSearchParam(source, 'status')) || undefined,
  }
}

export function getOrderListQueryOptions(params: Partial<OrderQueryParams> = {}) {
  const normalized = normalizeOrderListQueryParams(params)
  return {
    queryKey: [
      'orders',
      normalized.page,
      normalized.limit,
      normalized.status || '',
      normalized.search,
    ] as const,
    queryFn: () =>
      getOrders({
        page: normalized.page,
        limit: normalized.limit,
        status: normalized.status,
        search: normalized.search || undefined,
      }),
  }
}
