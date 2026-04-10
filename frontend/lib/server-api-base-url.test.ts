/** @jest-environment node */

type HeaderInitMap = Record<string, string | undefined>

function createHeaders(init?: HeaderInitMap): Headers {
  const headers = new Headers()
  for (const [key, value] of Object.entries(init || {})) {
    if (typeof value === 'string') {
      headers.set(key, value)
    }
  }
  return headers
}

function loadServerAPIBaseURLModule(nextHeaders?: HeaderInitMap) {
  const headersMock = jest.fn(async () => createHeaders(nextHeaders))

  jest.resetModules()
  jest.doMock('server-only', () => ({}))
  jest.doMock('next/headers', () => ({
    headers: headersMock,
  }))

  let loadedModule: typeof import('@/lib/server-api-base-url')
  jest.isolateModules(() => {
    loadedModule = require('@/lib/server-api-base-url')
  })

  return {
    module: loadedModule!,
    headersMock,
  }
}

describe('server-api-base-url', () => {
  const originalAPIURL = process.env.NEXT_PUBLIC_API_URL

  afterEach(() => {
    jest.resetModules()
    jest.clearAllMocks()
    if (originalAPIURL === undefined) {
      delete process.env.NEXT_PUBLIC_API_URL
    } else {
      process.env.NEXT_PUBLIC_API_URL = originalAPIURL
    }
  })

  test('prefers the configured public API URL over request headers', async () => {
    process.env.NEXT_PUBLIC_API_URL = 'https://api.example.com/'

    const { module } = loadServerAPIBaseURLModule({
      host: 'frontend.example.com',
      'x-forwarded-host': 'proxy.example.com',
      'x-forwarded-proto': 'http',
    })

    expect(
      module.resolveServerAPIBaseURLFromRequest(
        new Request('https://frontend.example.com/products', {
          headers: {
            host: 'frontend.example.com',
            'x-forwarded-host': 'proxy.example.com',
            'x-forwarded-proto': 'http',
          },
        })
      )
    ).toBe('https://api.example.com')
    await expect(module.resolveServerAPIBaseURL()).resolves.toBe('https://api.example.com')
  })

  test('uses forwarded host and protocol when config is missing', () => {
    delete process.env.NEXT_PUBLIC_API_URL

    const { module } = loadServerAPIBaseURLModule()

    expect(
      module.resolveServerAPIBaseURLFromRequest(
        new Request('https://frontend.example.com/products', {
          headers: {
            host: 'frontend.example.com',
            'x-forwarded-host': 'shop.example.com, proxy.internal',
            'x-forwarded-proto': 'http, https',
          },
        })
      )
    ).toBe('http://shop.example.com')
  })

  test('falls back to localhost http and external https when forwarded proto is absent', async () => {
    delete process.env.NEXT_PUBLIC_API_URL

    const localhostModule = loadServerAPIBaseURLModule({
      host: 'localhost:3000',
    }).module
    expect(
      localhostModule.resolveServerAPIBaseURLFromRequest(
        new Request('http://localhost:3000/products', {
          headers: {
            host: 'localhost:3000',
          },
        })
      )
    ).toBe('http://localhost:3000')

    const { module, headersMock } = loadServerAPIBaseURLModule({
      host: 'store.example.com',
    })
    await expect(module.resolveServerAPIBaseURL()).resolves.toBe('https://store.example.com')
    expect(headersMock).toHaveBeenCalledTimes(1)
  })

  test('throws when no host headers are available', () => {
    delete process.env.NEXT_PUBLIC_API_URL

    const { module } = loadServerAPIBaseURLModule()

    expect(() =>
      module.resolveServerAPIBaseURLFromRequest(
        new Request('https://frontend.example.com/products')
      )
    ).toThrow('Unable to resolve server API base URL')
  })
})
