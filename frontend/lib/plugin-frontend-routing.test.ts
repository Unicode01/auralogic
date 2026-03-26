import {
  buildPluginBootstrapContextKey,
  buildPluginFullPath,
  buildPluginQueryString,
  isPluginMenuPathActive,
  matchPluginRoute,
  normalizePluginPath,
  normalizePluginHostContext,
  readPluginSearchParams,
  stringifyPluginHostContext,
} from '@/lib/plugin-frontend-routing'

describe('plugin-frontend-routing', () => {
  it('normalizes plugin paths and removes traversal segments', () => {
    expect(normalizePluginPath(' plugin-pages/demo/../orders/42/ ')).toBe('/plugin-pages/orders/42')
    expect(normalizePluginPath('/plugin-pages/demo///')).toBe('/plugin-pages/demo')
    expect(normalizePluginPath('')).toBe('/')
  })

  it('matches exact, named, legacy wildcard, and named wildcard plugin routes', () => {
    expect(matchPluginRoute('/plugin-pages/demo', '/plugin-pages/demo')).toEqual({
      matched: true,
      routeParams: {},
    })

    expect(
      matchPluginRoute(
        '/admin/plugin-pages/logistics/orders/:orderNo',
        '/admin/plugin-pages/logistics/orders/ORD-1001'
      )
    ).toEqual({
      matched: true,
      routeParams: {
        orderNo: 'ORD-1001',
      },
    })

    expect(matchPluginRoute('/plugin-pages/demo/*', '/plugin-pages/demo/history/detail')).toEqual({
      matched: true,
      routeParams: {},
    })

    expect(
      matchPluginRoute('/plugin-pages/logistics/*rest', '/plugin-pages/logistics/orders/detail/1')
    ).toEqual({
      matched: true,
      routeParams: {
        rest: 'orders/detail/1',
      },
    })
  })

  it('rejects plugin routes that do not match the current path', () => {
    expect(matchPluginRoute('/plugin-pages/demo/:id', '/plugin-pages/demo')).toEqual({
      matched: false,
      routeParams: {},
    })

    expect(matchPluginRoute('/plugin-pages/demo/*rest/more', '/plugin-pages/demo/history')).toEqual(
      {
        matched: false,
        routeParams: {},
      }
    )
  })

  it('builds stable query strings and full paths regardless of map order', () => {
    const a = buildPluginQueryString({
      tab: 'timeline',
      order_id: '123',
    })
    const b = buildPluginQueryString({
      order_id: '123',
      tab: 'timeline',
    })

    expect(a).toBe('order_id=123&tab=timeline')
    expect(a).toBe(b)
    expect(
      buildPluginFullPath('/plugin-pages/logistics/orders/ORD-1001', {
        tab: 'timeline',
        order_id: '123',
      })
    ).toBe('/plugin-pages/logistics/orders/ORD-1001?order_id=123&tab=timeline')
    expect(
      buildPluginBootstrapContextKey('/plugin-pages/logistics/orders/ORD-1001', {
        order_id: '123',
        tab: 'timeline',
      })
    ).toBe('/plugin-pages/logistics/orders/ORD-1001?order_id=123&tab=timeline')
  })

  it('marks plugin menu items active for exact and descendant matches only', () => {
    expect(
      isPluginMenuPathActive('/plugin-pages/logistics/orders/ORD-1001', '/plugin-pages/logistics')
    ).toBe(true)
    expect(isPluginMenuPathActive('/profile/settings', '/profile')).toBe(true)
    expect(isPluginMenuPathActive('/profiled', '/profile')).toBe(false)
  })

  it('reads query params from URLSearchParams into a plain object', () => {
    expect(readPluginSearchParams(new URLSearchParams('tab=timeline&order_id=123'))).toEqual({
      order_id: '123',
      tab: 'timeline',
    })
  })

  it('normalizes and stringifies host context with stable nested ordering', () => {
    expect(
      normalizePluginHostContext({
        ' view ': 'admin_orders',
        selection: {
          selected_count: 2,
          selected_ids: [3, undefined, 1],
        },
        ignored: undefined,
      })
    ).toEqual({
      selection: {
        selected_count: 2,
        selected_ids: [3, 1],
      },
      view: 'admin_orders',
    })

    expect(
      stringifyPluginHostContext({
        selection: {
          current_page_ids: [11, 12],
          selected_count: 2,
        },
        view: 'admin_orders',
      })
    ).toBe(
      '{"selection":{"current_page_ids":[11,12],"selected_count":2},"view":"admin_orders"}'
    )

    expect(
      stringifyPluginHostContext({
        view: 'admin_orders',
        selection: {
          selected_count: 2,
          current_page_ids: [11, 12],
        },
      })
    ).toBe(
      '{"selection":{"current_page_ids":[11,12],"selected_count":2},"view":"admin_orders"}'
    )
  })
})
