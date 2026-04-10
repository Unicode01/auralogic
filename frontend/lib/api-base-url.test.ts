describe('api-base-url helpers', () => {
  const originalAPIURL = process.env.NEXT_PUBLIC_API_URL

  afterEach(() => {
    jest.resetModules()
    if (originalAPIURL === undefined) {
      delete process.env.NEXT_PUBLIC_API_URL
    } else {
      process.env.NEXT_PUBLIC_API_URL = originalAPIURL
    }
  })

  it('uses relative URLs when NEXT_PUBLIC_API_URL is not configured', () => {
    delete process.env.NEXT_PUBLIC_API_URL

    jest.isolateModules(() => {
      const { getConfiguredPublicAPIBaseURL, resolvePublicAPIURL } = require('@/lib/api-base-url')

      expect(getConfiguredPublicAPIBaseURL()).toBe('')
      expect(resolvePublicAPIURL('/api/user/orders')).toBe('/api/user/orders')
      expect(resolvePublicAPIURL('api/user/orders')).toBe('/api/user/orders')
    })
  })

  it('falls back to window origin for absolute URLs when config is missing', () => {
    delete process.env.NEXT_PUBLIC_API_URL

    jest.isolateModules(() => {
      const { getPublicAbsoluteAPIBaseURL, resolvePublicAbsoluteAPIURL } = require(
        '@/lib/api-base-url'
      )

      expect(getPublicAbsoluteAPIBaseURL()).toBe(window.location.origin)
      expect(resolvePublicAbsoluteAPIURL('/api/user/invoice/token_123')).toBe(
        `${window.location.origin}/api/user/invoice/token_123`
      )
    })
  })

  it('prefers the configured public API URL when present', () => {
    process.env.NEXT_PUBLIC_API_URL = 'https://api.example.com/'

    jest.isolateModules(() => {
      const {
        getConfiguredPublicAPIBaseURL,
        getPublicAbsoluteAPIBaseURL,
        resolvePublicAPIURL,
      } = require('@/lib/api-base-url')

      expect(getConfiguredPublicAPIBaseURL()).toBe('https://api.example.com')
      expect(getPublicAbsoluteAPIBaseURL()).toBe('https://api.example.com')
      expect(resolvePublicAPIURL('/api/admin/orders/export')).toBe(
        'https://api.example.com/api/admin/orders/export'
      )
    })
  })
})
