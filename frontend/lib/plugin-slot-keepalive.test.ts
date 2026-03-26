import {
  buildPluginSlotKeepaliveKey,
  readPluginSlotKeepalive,
  resetPluginSlotKeepaliveForTest,
  writePluginSlotKeepalive,
} from '@/lib/plugin-slot-keepalive'

describe('plugin-slot-keepalive', () => {
  beforeEach(() => {
    resetPluginSlotKeepaliveForTest()
    jest.useRealTimers()
  })

  it('stores keepalive entries by normalized scope path and slot', () => {
    const key = buildPluginSlotKeepaliveKey('admin', '/admin/orders/', 'admin.orders.bottom')
    writePluginSlotKeepalive(key, {
      data: {
        path: '/admin/orders',
        slot: 'admin.orders.bottom',
        extensions: [{ id: 'ext-1' }],
      },
    })

    expect(readPluginSlotKeepalive(key)).toEqual({
      data: {
        path: '/admin/orders',
        slot: 'admin.orders.bottom',
        extensions: [{ id: 'ext-1' }],
      },
    })
  })

  it('expires keepalive entries after ttl', () => {
    jest.useFakeTimers()
    const key = buildPluginSlotKeepaliveKey('public', '/orders', 'user.orders.bottom')
    writePluginSlotKeepalive(
      key,
      {
        data: {
          path: '/orders',
          slot: 'user.orders.bottom',
          extensions: [],
        },
      },
      1000
    )

    expect(readPluginSlotKeepalive(key)).toBeDefined()
    jest.advanceTimersByTime(1001)
    expect(readPluginSlotKeepalive(key)).toBeUndefined()
  })
})
