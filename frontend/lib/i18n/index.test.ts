import {
  translateBizError,
  type Translations,
  zhTranslations,
} from '@/lib/i18n'
import { enTranslations } from '@/lib/i18n/en'

describe('translateBizError', () => {
  it('translates existing zh/en biz errors with parameter interpolation', () => {
    expect(
      translateBizError(zhTranslations, 'order.stockInsufficient', {
        product: '测试商品',
        available: 3,
      })
    ).toBe('商品 测试商品 库存不足，仅剩3件')

    expect(
      translateBizError(enTranslations, 'order.stockInsufficient', {
        product: 'Demo Product',
        available: 2,
      })
    ).toBe('Demo Product is out of stock, only 2 left')

    expect(
      translateBizError(zhTranslations, 'payment.pollingGlobalQueueLimitExceeded', {
        max: 10,
      })
    ).toBe('系统支付轮询队列已满（上限 10），请稍后重试')

    expect(
      translateBizError(enTranslations, 'payment.pollingGlobalQueueLimitExceeded', {
        max: 8,
      })
    ).toBe('Payment polling queue is full (limit: 8). Please try again later.')
  })

  it('discovers bizError dictionaries from nested namespaces automatically', () => {
    const customTranslations = {
      ...zhTranslations,
      order: {
        ...zhTranslations.order,
        bizError: {
          ...zhTranslations.order.bizError,
          'order.trackingMissing': '订单 {orderNo} 缺少物流单号',
        },
      },
    } as Translations

    expect(
      translateBizError(customTranslations, 'order.trackingMissing', {
        orderNo: 'ORD-1001',
      })
    ).toBe('订单 ORD-1001 缺少物流单号')
  })

  it('keeps fallback behavior when translation is missing', () => {
    expect(
      translateBizError(zhTranslations, 'order.unknown', undefined, '操作失败')
    ).toBe('操作失败')

    expect(translateBizError(zhTranslations, 'order.unknown')).toBe('order.unknown')
  })

  it('loads english translations on demand and caches them', async () => {
    const { getTranslations, hasLoadedTranslations, loadTranslations } = await import('@/lib/i18n')

    expect(hasLoadedTranslations('zh')).toBe(true)
    expect(getTranslations('en').common.loading).toBe(zhTranslations.common.loading)

    const loaded = await loadTranslations('en')

    expect(loaded.common.loading).toBe(enTranslations.common.loading)
    expect(hasLoadedTranslations('en')).toBe(true)
    expect(getTranslations('en').common.loading).toBe(enTranslations.common.loading)
  })
})
