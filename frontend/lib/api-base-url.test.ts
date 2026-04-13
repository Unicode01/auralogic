describe('api-base-url helpers', () => {
  const originalAPIURL = process.env.NEXT_PUBLIC_API_URL
  const originalAppURL = process.env.NEXT_PUBLIC_APP_URL

  afterEach(() => {
    jest.resetModules()
    if (originalAPIURL === undefined) {
      delete process.env.NEXT_PUBLIC_API_URL
    } else {
      process.env.NEXT_PUBLIC_API_URL = originalAPIURL
    }
    if (originalAppURL === undefined) {
      delete process.env.NEXT_PUBLIC_APP_URL
    } else {
      process.env.NEXT_PUBLIC_APP_URL = originalAppURL
    }
  })

  it('uses relative URLs when NEXT_PUBLIC_API_URL is not configured', () => {
    delete process.env.NEXT_PUBLIC_API_URL
    delete process.env.NEXT_PUBLIC_APP_URL

    jest.isolateModules(() => {
      const {
        getConfiguredPublicAPIBaseURL,
        getEffectivePublicAPIBaseURL,
        resolvePublicAPIURL,
      } = require('@/lib/api-base-url')

      expect(getConfiguredPublicAPIBaseURL()).toBe('')
      expect(getEffectivePublicAPIBaseURL()).toBe('')
      expect(resolvePublicAPIURL('/api/user/orders')).toBe('/api/user/orders')
      expect(resolvePublicAPIURL('api/user/orders')).toBe('/api/user/orders')
    })
  })

  it('falls back to window origin for absolute URLs when config is missing', () => {
    delete process.env.NEXT_PUBLIC_API_URL
    delete process.env.NEXT_PUBLIC_APP_URL

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
    process.env.NEXT_PUBLIC_APP_URL = 'https://store.example.com/'

    jest.isolateModules(() => {
      const {
        getConfiguredPublicAPIBaseURL,
        getEffectivePublicAPIBaseURL,
        getPublicAbsoluteAPIBaseURL,
        resolvePublicAPIURL,
      } = require('@/lib/api-base-url')

      expect(getConfiguredPublicAPIBaseURL()).toBe('https://api.example.com')
      expect(getEffectivePublicAPIBaseURL()).toBe('https://api.example.com')
      expect(getPublicAbsoluteAPIBaseURL()).toBe('https://api.example.com')
      expect(resolvePublicAPIURL('/api/admin/orders/export')).toBe(
        'https://api.example.com/api/admin/orders/export'
      )
    })
  })

  it('keeps browser requests relative when the public API base matches the app URL', () => {
    process.env.NEXT_PUBLIC_API_URL = 'http://192.168.1.127:8080/'
    process.env.NEXT_PUBLIC_APP_URL = 'http://192.168.1.127:8080/'

    jest.isolateModules(() => {
      const {
        getConfiguredPublicAPIBaseURL,
        getEffectivePublicAPIBaseURL,
        getPublicAbsoluteAPIBaseURL,
        resolvePublicAPIURL,
      } = require('@/lib/api-base-url')

      expect(getConfiguredPublicAPIBaseURL()).toBe('http://192.168.1.127:8080')
      expect(getEffectivePublicAPIBaseURL()).toBe('')
      expect(getPublicAbsoluteAPIBaseURL()).toBe(window.location.origin)
      expect(resolvePublicAPIURL('/api/user/auth/captcha')).toBe('/api/user/auth/captcha')
    })
  })

  it('keeps browser requests relative when a leaked private API host conflicts with a public app URL', () => {
    process.env.NEXT_PUBLIC_API_URL = 'http://192.168.1.127:8080/'
    process.env.NEXT_PUBLIC_APP_URL = 'https://store.example.com/'

    jest.isolateModules(() => {
      const { getEffectivePublicAPIBaseURL, resolvePublicAPIURL } = require('@/lib/api-base-url')

      expect(getEffectivePublicAPIBaseURL()).toBe('')
      expect(resolvePublicAPIURL('/api/user/auth/captcha')).toBe('/api/user/auth/captcha')
    })
  })
})
