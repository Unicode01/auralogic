import { getProduct, getProductAvailableStock, getPublicConfig } from '@/lib/api'

const DEFAULT_PRODUCT_STOCK_ATTRIBUTES: Record<string, string> = {}

export const publicConfigQueryKey = ['publicConfig'] as const

export function getPublicConfigQueryOptions() {
  return {
    queryKey: publicConfigQueryKey,
    queryFn: getPublicConfig,
    staleTime: 5 * 60 * 1000,
  }
}

export function getProductQueryOptions(productId: number) {
  return {
    queryKey: ['product', productId] as const,
    queryFn: () => getProduct(productId),
  }
}

export function getProductStockQueryOptions(
  productId: number,
  selectedAttributes: Record<string, string> = DEFAULT_PRODUCT_STOCK_ATTRIBUTES
) {
  return {
    queryKey: ['productStock', productId, selectedAttributes] as const,
    queryFn: () => {
      if (Object.keys(selectedAttributes).length > 0) {
        return getProductAvailableStock(productId, selectedAttributes)
      }
      return getProductAvailableStock(productId)
    },
    refetchInterval: 30000,
  }
}
