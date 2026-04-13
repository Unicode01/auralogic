import {
  buildPageInjectRuntimeScript,
  clearStoredPageInjectCache,
  invalidatePageInjectRuntime,
  isSamePageInjectPayload,
  PAGE_INJECT_CACHE_KEY,
  PAGE_INJECT_INVALIDATE_EVENT,
  readStoredPageInjectCache,
  writeStoredPageInjectCache,
} from '@/lib/page-inject'

describe('page inject helpers', () => {
  beforeEach(() => {
    window.localStorage.clear()
  })

  it('drops legacy cache keys and prunes expired entries', () => {
    window.localStorage.setItem('auralogic-page-inject', '{"legacy":true}')
    window.localStorage.setItem(
      PAGE_INJECT_CACHE_KEY,
      JSON.stringify({
        '/orders': {
          css: '.orders{}',
          js: '',
          rules: [],
          ts: 9500,
        },
        '/admin/analytics': {
          css: '.analytics{}',
          js: '',
          rules: [],
          ts: 1000,
        },
      })
    )

    const cache = readStoredPageInjectCache(window.localStorage, 10000, 1000)

    expect(window.localStorage.getItem('auralogic-page-inject')).toBeNull()
    expect(cache).toEqual({
      '/orders': {
        css: '.orders{}',
        js: '',
        rules: [],
        ts: 9500,
      },
    })
  })

  it('clears stored cache entries', () => {
    window.localStorage.setItem('auralogic-page-inject', '{"legacy":true}')
    window.localStorage.setItem(PAGE_INJECT_CACHE_KEY, '{"current":true}')

    clearStoredPageInjectCache(window.localStorage)

    expect(window.localStorage.getItem('auralogic-page-inject')).toBeNull()
    expect(window.localStorage.getItem(PAGE_INJECT_CACHE_KEY)).toBeNull()
  })

  it('invalidates runtime cache and dispatches a refresh event', () => {
    const listener = jest.fn()
    window.addEventListener(PAGE_INJECT_INVALIDATE_EVENT, listener)
    window.localStorage.setItem(PAGE_INJECT_CACHE_KEY, '{"current":true}')

    invalidatePageInjectRuntime(window.localStorage)

    expect(window.localStorage.getItem(PAGE_INJECT_CACHE_KEY)).toBeNull()
    expect(listener).toHaveBeenCalledTimes(1)

    window.removeEventListener(PAGE_INJECT_INVALIDATE_EVENT, listener)
  })

  it('writes only fresh cache entries', () => {
    writeStoredPageInjectCache(
      window.localStorage,
      {
        '/orders': {
          css: '.orders{}',
          js: '',
          rules: [],
          ts: 5000,
        },
        '/admin/analytics': {
          css: '.analytics{}',
          js: '',
          rules: [],
          ts: 1000,
        },
      },
      5500,
      1000
    )

    expect(window.localStorage.getItem(PAGE_INJECT_CACHE_KEY)).toBe(
      JSON.stringify({
        '/orders': {
          css: '.orders{}',
          js: '',
          rules: [],
          ts: 5000,
        },
      })
    )
  })

  it('compares normalized payloads', () => {
    expect(
      isSamePageInjectPayload(
        {
          css: '',
          js: '',
          rules: [{ name: 'orders', pattern: '^/orders', match_type: 'regex', css: '', js: '' }],
        },
        {
          css: '',
          js: '',
          rules: [
            {
              name: 'orders',
              pattern: '^/orders',
              match_type: 'regex',
              css: '',
              js: '',
              extra: true,
            } as never,
          ],
        }
      )
    ).toBe(true)
  })

  it('wraps injected javascript in a safe runtime executor', () => {
    const source = buildPageInjectRuntimeScript('window.__AURALOGIC__ = true;', 'orders-debug')

    expect(source).toContain('new Function(source).call(window);')
    expect(source).toContain('orders-debug')
    expect(source).toContain('window.__AURALOGIC__ = true;')
  })
})
