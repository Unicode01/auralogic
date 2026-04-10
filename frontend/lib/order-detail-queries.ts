import { getOrder, getOrderVirtualProducts } from '@/lib/api'

const PENDING_VIRTUAL_PRODUCT_STATUSES = new Set(['pending_payment', 'draft', 'need_resubmit'])

export function getOrderDetailQueryOptions(orderNo: string) {
  return {
    queryKey: ['order', orderNo] as const,
    queryFn: () => getOrder(orderNo),
  }
}

export function getOrderVirtualProductsQueryOptions(orderNo: string) {
  return {
    queryKey: ['orderVirtualProducts', orderNo] as const,
    queryFn: () => getOrderVirtualProducts(orderNo),
  }
}

export function shouldFetchOrderVirtualProducts(order: any): boolean {
  const status = String(order?.status || '').trim()
  if (!status) {
    return false
  }
  return !PENDING_VIRTUAL_PRODUCT_STATUSES.has(status)
}
