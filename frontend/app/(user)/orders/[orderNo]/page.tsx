import { HydrationBoundary, dehydrate } from '@tanstack/react-query'
import OrderDetailClient from './order-detail-client'
import {
  getOrderDetailQueryOptions,
  getOrderVirtualProductsQueryOptions,
  shouldFetchOrderVirtualProducts,
} from '@/lib/order-detail-queries'
import { getPublicConfigQueryOptions } from '@/lib/product-detail-queries'
import {
  getServerAuthToken,
  getServerOrder,
  getServerOrderVirtualProducts,
  getServerPublicConfig,
} from '@/lib/server-api'
import { createServerQueryClient } from '@/lib/server-query-client'

function isPrefetchableOrderNo(value: string): boolean {
  return String(value || '').trim().length > 0
}

export default async function OrderDetailPage({
  params,
}: {
  params: Promise<{ orderNo: string }>
}) {
  const { orderNo } = await params
  const queryClient = createServerQueryClient()

  if ((await getServerAuthToken()) && isPrefetchableOrderNo(orderNo)) {
    try {
      await queryClient.prefetchQuery({
        ...getPublicConfigQueryOptions(),
        queryFn: getServerPublicConfig,
      })
    } catch {
      // Preserve the existing client-side loading and error handling.
    }

    try {
      const orderResponse = await queryClient.fetchQuery({
        ...getOrderDetailQueryOptions(orderNo),
        queryFn: () => getServerOrder(orderNo),
      })

      if (shouldFetchOrderVirtualProducts(orderResponse?.data)) {
        await queryClient.prefetchQuery({
          ...getOrderVirtualProductsQueryOptions(orderNo),
          queryFn: () => getServerOrderVirtualProducts(orderNo),
        })
      }
    } catch {
      // Preserve the existing client-side loading and error handling.
    }
  }

  return (
    <HydrationBoundary state={dehydrate(queryClient)}>
      <OrderDetailClient orderNo={orderNo} />
    </HydrationBoundary>
  )
}
