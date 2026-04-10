import {
  getOrderDetailQueryOptions,
  getOrderVirtualProductsQueryOptions,
  shouldFetchOrderVirtualProducts,
} from '@/lib/order-detail-queries'

describe('order-detail query helpers', () => {
  test('builds stable order detail and virtual products query keys', () => {
    expect(getOrderDetailQueryOptions('ORD-1001').queryKey).toEqual(['order', 'ORD-1001'])
    expect(getOrderVirtualProductsQueryOptions('ORD-1001').queryKey).toEqual([
      'orderVirtualProducts',
      'ORD-1001',
    ])
  })

  test('only fetches virtual products after the order leaves draft and payment-pending states', () => {
    expect(shouldFetchOrderVirtualProducts({ status: 'pending_payment' })).toBe(false)
    expect(shouldFetchOrderVirtualProducts({ status: 'draft' })).toBe(false)
    expect(shouldFetchOrderVirtualProducts({ status: 'need_resubmit' })).toBe(false)
    expect(shouldFetchOrderVirtualProducts({ status: 'completed' })).toBe(true)
    expect(shouldFetchOrderVirtualProducts({ status: 'shipped' })).toBe(true)
    expect(shouldFetchOrderVirtualProducts(null)).toBe(false)
  })
})
