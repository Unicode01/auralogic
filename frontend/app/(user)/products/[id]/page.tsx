import { HydrationBoundary, QueryClient, dehydrate } from '@tanstack/react-query'
import ProductDetailClient from './product-detail-client'
import {
  getProductQueryOptions,
  getProductStockQueryOptions,
  getPublicConfigQueryOptions,
} from '@/lib/product-detail-queries'
import {
  getServerProduct,
  getServerProductAvailableStock,
  getServerPublicConfig,
} from '@/lib/server-public-api'

function isPositiveInteger(value: number): boolean {
  return Number.isFinite(value) && value > 0
}

export default async function ProductDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params
  const productId = Number(id)
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 60 * 1000,
        refetchOnWindowFocus: false,
      },
    },
  })

  let allowGuestProductBrowse = false

  try {
    const publicConfig = await queryClient.fetchQuery({
      ...getPublicConfigQueryOptions(),
      queryFn: getServerPublicConfig,
    })
    allowGuestProductBrowse = publicConfig?.data?.allow_guest_product_browse === true
  } catch {
    // Fall back to the client fetch path when public config is unavailable.
  }

  if (allowGuestProductBrowse && isPositiveInteger(productId)) {
    try {
      await queryClient.fetchQuery({
        ...getProductQueryOptions(productId),
        queryFn: () => getServerProduct(productId),
      })
      await queryClient.prefetchQuery({
        ...getProductStockQueryOptions(productId),
        queryFn: () => getServerProductAvailableStock(productId),
      })
    } catch {
      // Preserve the existing client-side loading and error handling.
    }
  }

  return (
    <HydrationBoundary state={dehydrate(queryClient)}>
      <ProductDetailClient productId={productId} />
    </HydrationBoundary>
  )
}
