import type { AxiosResponse } from 'axios'

import {
  fetchPluginSlotExtensions,
  resetPluginSlotRequestQueueForTest,
} from '@/lib/plugin-slot-request'
import {
  getAdminPluginExtensions,
  getAdminPluginExtensionsBatch,
  getPluginExtensions,
  getPluginExtensionsBatch,
} from '@/lib/api'

jest.mock('@/lib/api', () => ({
  getPluginExtensions: jest.fn(),
  getAdminPluginExtensions: jest.fn(),
  getPluginExtensionsBatch: jest.fn(),
  getAdminPluginExtensionsBatch: jest.fn(),
}))

const getPluginExtensionsMock = jest.mocked(getPluginExtensions)
const getAdminPluginExtensionsMock = jest.mocked(getAdminPluginExtensions)
const getPluginExtensionsBatchMock = jest.mocked(getPluginExtensionsBatch)
const getAdminPluginExtensionsBatchMock = jest.mocked(getAdminPluginExtensionsBatch)

function mockAxiosResponse<T>(data: T): AxiosResponse<T> {
  return {
    data,
    status: 200,
    statusText: 'OK',
    headers: {},
    config: {} as AxiosResponse<T>['config'],
  }
}

describe('plugin-slot-request', () => {
  beforeEach(() => {
    resetPluginSlotRequestQueueForTest()
    jest.clearAllMocks()
  })

  it('batches concurrent slot requests in the same scope', async () => {
    getAdminPluginExtensionsBatchMock.mockImplementation(async (_path, items) =>
      mockAxiosResponse({
        items: items.map((item) => ({
          key: item.key,
          path: item.path,
          slot: item.slot,
          extensions: [
            {
              id: `ext:${item.slot}`,
              type: 'text',
              content: String(item.slot),
            },
          ],
        })),
      })
    )

    const firstPromise = fetchPluginSlotExtensions('admin', {
      path: '/admin/orders',
      slot: 'admin.orders.top',
      queryParams: { tab: 'open' },
    })
    const secondPromise = fetchPluginSlotExtensions('admin', {
      path: '/admin/orders',
      slot: 'admin.orders.bottom',
      queryParams: { tab: 'open' },
    })

    const [first, second] = await Promise.all([firstPromise, secondPromise])

    expect(getAdminPluginExtensionsBatchMock).toHaveBeenCalledTimes(1)
    expect(getAdminPluginExtensionsMock).not.toHaveBeenCalled()
    expect(getAdminPluginExtensionsBatchMock.mock.calls[0]?.[1]).toHaveLength(2)
    expect(first.data.slot).toBe('admin.orders.top')
    expect(first.data.extensions[0]?.content).toBe('admin.orders.top')
    expect(second.data.slot).toBe('admin.orders.bottom')
    expect(second.data.extensions[0]?.content).toBe('admin.orders.bottom')
  })

  it('falls back to individual slot requests when batch loading fails', async () => {
    getPluginExtensionsBatchMock.mockRejectedValue(new Error('batch failed'))
    getPluginExtensionsMock.mockImplementation(async (path, slot) =>
      mockAxiosResponse({
        path,
        slot,
        extensions: [
          {
            id: `single:${slot}`,
            type: 'text',
            content: String(slot),
          },
        ],
      })
    )

    const firstPromise = fetchPluginSlotExtensions('public', {
      path: '/orders',
      slot: 'user.orders.top',
    })
    const secondPromise = fetchPluginSlotExtensions('public', {
      path: '/orders',
      slot: 'user.orders.bottom',
    })

    const [first, second] = await Promise.all([firstPromise, secondPromise])

    expect(getPluginExtensionsBatchMock).toHaveBeenCalledTimes(1)
    expect(getPluginExtensionsMock).toHaveBeenCalledTimes(2)
    expect(first.data.extensions[0]?.id).toBe('single:user.orders.top')
    expect(second.data.extensions[0]?.id).toBe('single:user.orders.bottom')
  })

  it('splits concurrent batch requests by locale', async () => {
    getPluginExtensionsBatchMock.mockImplementation(async (_path, items, _signal, locale) =>
      mockAxiosResponse({
        items: items.map((item) => ({
          key: item.key,
          path: item.path,
          slot: item.slot,
          extensions: [
            {
              id: `${locale}:${item.slot}`,
              type: 'text',
              content: String(locale),
            },
          ],
        })),
      })
    )

    const zhTopPromise = fetchPluginSlotExtensions('public', {
      path: '/orders',
      slot: 'user.orders.top',
      locale: 'zh',
    })
    const zhBottomPromise = fetchPluginSlotExtensions('public', {
      path: '/orders',
      slot: 'user.orders.bottom',
      locale: 'zh',
    })
    const enTopPromise = fetchPluginSlotExtensions('public', {
      path: '/orders',
      slot: 'user.orders.filters.after',
      locale: 'en',
    })
    const enBottomPromise = fetchPluginSlotExtensions('public', {
      path: '/orders',
      slot: 'user.orders.after_list',
      locale: 'en',
    })

    const [zhTopResult, zhBottomResult, enTopResult, enBottomResult] = await Promise.all([
      zhTopPromise,
      zhBottomPromise,
      enTopPromise,
      enBottomPromise,
    ])

    expect(getPluginExtensionsBatchMock).toHaveBeenCalledTimes(2)
    expect(getPluginExtensionsMock).not.toHaveBeenCalled()
    expect(zhTopResult.data.extensions[0]?.content).toBe('zh')
    expect(zhBottomResult.data.extensions[0]?.content).toBe('zh')
    expect(enTopResult.data.extensions[0]?.content).toBe('en')
    expect(enBottomResult.data.extensions[0]?.content).toBe('en')
  })

  it('uses a single request directly when only one slot is pending', async () => {
    getPluginExtensionsMock.mockResolvedValue(
      mockAxiosResponse({
        path: '/tickets',
        slot: 'user.tickets.top',
        extensions: [],
      })
    )

    const result = await fetchPluginSlotExtensions('public', {
      path: '/tickets',
      slot: 'user.tickets.top',
    })

    expect(getPluginExtensionsMock).toHaveBeenCalledTimes(1)
    expect(getPluginExtensionsBatchMock).not.toHaveBeenCalled()
    expect(result.data.slot).toBe('user.tickets.top')
  })

  it('drops aborted slot requests before the queue flushes', async () => {
    const controller = new AbortController()
    const promise = fetchPluginSlotExtensions('public', {
      path: '/tickets',
      slot: 'user.tickets.top',
      signal: controller.signal,
    })
    controller.abort()

    await expect(promise).rejects.toMatchObject({ name: 'AbortError' })
    expect(getPluginExtensionsMock).not.toHaveBeenCalled()
    expect(getPluginExtensionsBatchMock).not.toHaveBeenCalled()
  })
})
