import {
  clearAuthReturnState,
  readAuthReturnState,
  setAuthReturnState,
} from '@/lib/auth-return-state'

describe('auth-return-state', () => {
  beforeEach(() => {
    sessionStorage.clear()
    jest.restoreAllMocks()
  })

  it('stores and reads cart return state from session storage', () => {
    setAuthReturnState({
      redirectPath: '/cart',
      cart: {
        selectedGuestKeys: ['2:{}', '1:{"color":"blue"}', '2:{}'],
        scrollTop: 240,
      },
    })

    expect(readAuthReturnState()).toEqual({
      redirectPath: '/cart',
      cart: {
        selectedGuestKeys: ['2:{}', '1:{"color":"blue"}'],
        scrollTop: 240,
      },
    })
  })

  it('stores and reads product detail return state with normalized config', () => {
    setAuthReturnState({
      redirectPath: '/products/42',
      product: {
        productId: 42,
        selectedAttributes: {
          edition: ' deluxe ',
          region: '  global',
        },
        quantity: 3.8,
        scrollTop: 360,
      },
    })

    expect(readAuthReturnState()).toEqual({
      redirectPath: '/products/42',
      product: {
        productId: 42,
        selectedAttributes: {
          edition: 'deluxe',
          region: 'global',
        },
        quantity: 3,
        scrollTop: 360,
      },
    })
  })

  it('rejects invalid redirect paths', () => {
    setAuthReturnState({
      redirectPath: 'https://example.com/cart',
      cart: {
        selectedGuestKeys: ['1:{}'],
        scrollTop: 10,
      },
    })

    expect(readAuthReturnState()).toBeNull()
  })

  it('expires stale entries', () => {
    const now = 1_700_000_000_000
    jest.spyOn(Date, 'now').mockReturnValue(now)

    sessionStorage.setItem(
      'auralogic_auth_return_state_v1',
      JSON.stringify({
        redirectPath: '/cart',
        cart: {
          selectedGuestKeys: ['1:{}'],
          scrollTop: 10,
        },
        ts: now - 1000 * 60 * 31,
      })
    )

    expect(readAuthReturnState()).toBeNull()
    expect(sessionStorage.getItem('auralogic_auth_return_state_v1')).toBeNull()
  })

  it('drops invalid product state while keeping redirect path', () => {
    setAuthReturnState({
      redirectPath: '/products/0',
      product: {
        productId: 0,
        selectedAttributes: {
          edition: 'standard',
        },
        quantity: 0,
        scrollTop: -1,
      },
    })

    expect(readAuthReturnState()).toEqual({
      redirectPath: '/products/0',
    })
  })

  it('clears stored state', () => {
    setAuthReturnState({
      redirectPath: '/cart',
    })

    clearAuthReturnState()

    expect(readAuthReturnState()).toBeNull()
  })
})
