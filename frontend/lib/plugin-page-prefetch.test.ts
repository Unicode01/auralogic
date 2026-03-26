import { resolvePluginPagePrefetchTarget } from '@/lib/plugin-page-prefetch'

describe('plugin-page-prefetch', () => {
  it('resolves user and admin plugin page links with stable query params', () => {
    expect(
      resolvePluginPagePrefetchTarget(
        '/plugin-pages/logistics/orders/ORD-1001?tab=timeline&order_id=123',
        'http://localhost/orders'
      )
    ).toEqual({
      href: '/plugin-pages/logistics/orders/ORD-1001?order_id=123&tab=timeline',
      path: '/plugin-pages/logistics/orders/ORD-1001',
      queryParams: {
        order_id: '123',
        tab: 'timeline',
      },
      scope: 'public',
    })

    expect(
      resolvePluginPagePrefetchTarget(
        '/admin/plugin-pages/logistics/orders/ORD-1001?tab=timeline&order_id=123',
        'http://localhost/admin/orders'
      )
    ).toEqual({
      href: '/admin/plugin-pages/logistics/orders/ORD-1001?order_id=123&tab=timeline',
      path: '/admin/plugin-pages/logistics/orders/ORD-1001',
      queryParams: {
        order_id: '123',
        tab: 'timeline',
      },
      scope: 'admin',
    })
  })

  it('resolves relative plugin page links against the current location', () => {
    expect(
      resolvePluginPagePrefetchTarget(
        '../tracking?tab=timeline&shipment=SF-1',
        'http://localhost/plugin-pages/logistics/orders/ORD-1001'
      )
    ).toEqual({
      href: '/plugin-pages/logistics/tracking?shipment=SF-1&tab=timeline',
      path: '/plugin-pages/logistics/tracking',
      queryParams: {
        shipment: 'SF-1',
        tab: 'timeline',
      },
      scope: 'public',
    })
  })

  it('rejects non-plugin and cross-origin links', () => {
    expect(
      resolvePluginPagePrefetchTarget('/orders/ORD-1001', 'http://localhost/plugin-pages/demo')
    ).toBeNull()
    expect(
      resolvePluginPagePrefetchTarget(
        'https://example.com/plugin-pages/logistics',
        'http://localhost/plugin-pages/demo'
      )
    ).toBeNull()
  })
})
