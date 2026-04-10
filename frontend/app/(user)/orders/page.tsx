import { HydrationBoundary, dehydrate } from '@tanstack/react-query'
import OrdersClient from './orders-client'
import {
  getOrderListQueryOptions,
  ORDER_LIST_LIMIT,
  parseOrderListSearchParams,
} from '@/lib/order-list-queries'
import { getServerAuthToken, getServerOrders } from '@/lib/server-api'
import { createServerQueryClient } from '@/lib/server-query-client'

export default async function OrdersPage({
  searchParams,
}: {
  searchParams?: Promise<Record<string, string | string[] | undefined>>
}) {
  const resolvedSearchParams = searchParams ? await searchParams : undefined
  const queryClient = createServerQueryClient()

  if (await getServerAuthToken()) {
    const initialQuery = parseOrderListSearchParams(resolvedSearchParams)

    try {
      await queryClient.prefetchQuery({
        ...getOrderListQueryOptions({
          page: initialQuery.page,
          limit: ORDER_LIST_LIMIT,
          status: initialQuery.status,
          search: initialQuery.search,
        }),
        queryFn: () =>
          getServerOrders({
            page: initialQuery.page,
            limit: ORDER_LIST_LIMIT,
            status: initialQuery.status,
            search: initialQuery.search || undefined,
          }),
      })
    } catch {
      // Preserve the existing client-side loading and error handling.
    }
  }

  return (
    <HydrationBoundary state={dehydrate(queryClient)}>
      <OrdersClient />
    </HydrationBoundary>
  )
}
