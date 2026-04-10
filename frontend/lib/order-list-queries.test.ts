import {
  getOrderListQueryOptions,
  normalizeOrderListQueryParams,
  parseOrderListSearchParams,
  ORDER_LIST_LIMIT,
} from '@/lib/order-list-queries'

describe('order-list query helpers', () => {
  test('parses order list search params from URLSearchParams and route records', () => {
    expect(
      parseOrderListSearchParams(
        new URLSearchParams('page=3&status=pending_payment&search=%20ORD-1001%20')
      )
    ).toEqual({
      page: 3,
      status: 'pending_payment',
      search: 'ORD-1001',
    })

    expect(
      parseOrderListSearchParams({
        page: ['0', '2'],
        status: ' completed ',
        search: [' ORD ', 'ignored'],
      })
    ).toEqual({
      page: 1,
      status: 'completed',
      search: 'ORD',
    })
  })

  test('normalizes list query params and builds stable query keys', () => {
    expect(
      normalizeOrderListQueryParams({
        page: 2,
        limit: 36,
        status: ' pending ',
        search: ' ORD-77 ',
      })
    ).toEqual({
      page: 2,
      limit: 36,
      status: 'pending',
      search: 'ORD-77',
    })

    expect(getOrderListQueryOptions({ page: 2, search: 'ORD-77' }).queryKey).toEqual([
      'orders',
      2,
      ORDER_LIST_LIMIT,
      '',
      'ORD-77',
    ])
  })
})
