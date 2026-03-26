import {
  clearCachedBootstrapMenus,
  clearCachedUserBootstrapMenus,
  getCachedUserBootstrapMenus,
  getCachedUserBootstrapMenusResult,
  setCachedUserBootstrapMenus,
} from '@/lib/plugin-bootstrap-cache'

describe('plugin-bootstrap-cache', () => {
  const scopeKey = 'user:42'
  const profileContext = '/profile?tab=overview'
  const orderContext = '/plugin-pages/logistics/orders/ORD-1001?tab=timeline'
  const menus = [
    {
      id: 'menu-1',
      title: 'Logistics',
      path: '/plugin-pages/logistics',
    },
  ]

  beforeEach(() => {
    localStorage.clear()
  })

  it('stores and reads bootstrap menus per scope and context', () => {
    setCachedUserBootstrapMenus(menus, scopeKey, profileContext)

    expect(getCachedUserBootstrapMenus(scopeKey, profileContext)).toEqual(menus)
    expect(getCachedUserBootstrapMenus(scopeKey, orderContext)).toEqual([])
  })

  it('clears all contexts for the same scope', () => {
    setCachedUserBootstrapMenus(menus, scopeKey, profileContext)
    setCachedUserBootstrapMenus(
      [
        {
          id: 'menu-2',
          title: 'Order Detail',
          path: '/plugin-pages/logistics/orders',
        },
      ],
      scopeKey,
      orderContext
    )

    clearCachedUserBootstrapMenus(scopeKey)

    expect(getCachedUserBootstrapMenus(scopeKey, profileContext)).toEqual([])
    expect(getCachedUserBootstrapMenus(scopeKey, orderContext)).toEqual([])
  })

  it('removes legacy scoped cache entries during a full clear', () => {
    localStorage.setItem(
      'auralogic_plugin_bootstrap_user_menu_v2:user%3A42:%2Fprofile%3Ftab%3Doverview',
      JSON.stringify({
        ts: Date.now(),
        menus,
      })
    )

    clearCachedBootstrapMenus()

    expect(
      localStorage.getItem(
        'auralogic_plugin_bootstrap_user_menu_v2:user%3A42:%2Fprofile%3Ftab%3Doverview'
      )
    ).toBeNull()
  })

  it('distinguishes cached empty menus from missing cache entries', () => {
    setCachedUserBootstrapMenus([], scopeKey, profileContext)

    expect(getCachedUserBootstrapMenusResult(scopeKey, profileContext)).toEqual({
      found: true,
      menus: [],
    })
    expect(getCachedUserBootstrapMenusResult(scopeKey, orderContext)).toEqual({
      found: false,
      menus: [],
    })
  })
})
