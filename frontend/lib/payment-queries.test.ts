import {
  adminPaymentMethodsQueryKey,
  getOrderPaymentInfoQueryKey,
  getUserPaymentMethodsQueryOptions,
  mergeSelectedOrderPaymentInfo,
  userPaymentMethodsQueryKey,
} from '@/lib/payment-queries'

describe('payment query helpers', () => {
  test('separates admin and user payment method cache keys', () => {
    expect(adminPaymentMethodsQueryKey).toEqual(['adminPaymentMethods'])
    expect(userPaymentMethodsQueryKey).toEqual(['userPaymentMethods'])
    expect(adminPaymentMethodsQueryKey).not.toEqual(userPaymentMethodsQueryKey)
    expect(getUserPaymentMethodsQueryOptions().queryKey).toEqual(userPaymentMethodsQueryKey)
    expect(getOrderPaymentInfoQueryKey('ORD-1001')).toEqual(['orderPaymentInfo', 'ORD-1001'])
  })

  test('merges selected payment info without dropping existing fields', () => {
    const merged = mergeSelectedOrderPaymentInfo(
      {
        data: {
          selected: false,
          available_methods: [{ id: 1, name: 'USDT' }],
          order_payment: {
            payment_card_cached: true,
            payment_card_cache_expires_at: '2026-04-10T00:00:00Z',
          },
        },
      },
      {
        id: 2,
        name: 'Stripe',
        icon: 'CreditCard',
      },
      {
        html: '<div>Pay now</div>',
      }
    )

    expect(merged).toEqual({
      data: {
        selected: true,
        available_methods: [{ id: 1, name: 'USDT' }],
        order_payment: {
          payment_card_cached: true,
          payment_card_cache_expires_at: '2026-04-10T00:00:00Z',
        },
        payment_method: {
          id: 2,
          name: 'Stripe',
          icon: 'CreditCard',
        },
        payment_card: {
          html: '<div>Pay now</div>',
        },
      },
    })
  })
})
