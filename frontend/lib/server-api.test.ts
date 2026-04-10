/** @jest-environment node */

import { AUTH_TOKEN_COOKIE_NAME } from '@/lib/auth'

type HeaderInitMap = Record<string, string | undefined>
type CookieValueMap = Record<string, string | undefined>

function createHeaders(init?: HeaderInitMap): Headers {
  const headers = new Headers()
  for (const [key, value] of Object.entries(init || {})) {
    if (typeof value === 'string') {
      headers.set(key, value)
    }
  }
  return headers
}

function createCookieStore(values?: CookieValueMap) {
  return {
    get(name: string) {
      const value = values?.[name]
      if (typeof value !== 'string') {
        return undefined
      }
      return { name, value }
    },
  }
}

function loadServerAPIModule(options?: {
  requestHeaders?: HeaderInitMap
  cookieValues?: CookieValueMap
  baseURL?: string
}) {
  const headersMock = jest.fn(async () => createHeaders(options?.requestHeaders))
  const cookiesMock = jest.fn(async () => createCookieStore(options?.cookieValues))
  const resolveServerAPIBaseURLMock = jest.fn(
    async () => options?.baseURL || 'https://backend.example.com'
  )

  jest.resetModules()
  jest.doMock('server-only', () => ({}))
  jest.doMock('next/headers', () => ({
    cookies: cookiesMock,
    headers: headersMock,
  }))
  jest.doMock('@/lib/server-api-base-url', () => ({
    resolveServerAPIBaseURL: resolveServerAPIBaseURLMock,
  }))

  let loadedModule: typeof import('@/lib/server-api')
  jest.isolateModules(() => {
    loadedModule = require('@/lib/server-api')
  })

  return {
    module: loadedModule!,
    headersMock,
    cookiesMock,
    resolveServerAPIBaseURLMock,
  }
}

describe('server-api', () => {
  const originalFetch = global.fetch

  afterEach(() => {
    global.fetch = originalFetch
    jest.resetModules()
    jest.clearAllMocks()
  })

  test('reads and trims the auth token from server cookies', async () => {
    const { module, cookiesMock } = loadServerAPIModule({
      cookieValues: {
        [AUTH_TOKEN_COOKIE_NAME]: '  server-token  ',
      },
    })

    await expect(module.getServerAuthToken()).resolves.toBe('server-token')
    expect(cookiesMock).toHaveBeenCalledTimes(1)
  })

  test('forwards locale headers for public server fetches', async () => {
    const fetchMock = jest.fn().mockResolvedValue(
      new Response(JSON.stringify({ data: { allow_guest_product_browse: true } }), {
        status: 200,
        headers: {
          'content-type': 'application/json',
        },
      })
    )
    global.fetch = fetchMock as unknown as typeof fetch

    const { module, headersMock, resolveServerAPIBaseURLMock } = loadServerAPIModule({
      requestHeaders: {
        'x-auralogic-locale': 'EN-US',
        'accept-language': 'zh-CN,zh;q=0.9',
      },
    })

    await expect(module.getServerPublicConfig()).resolves.toEqual({
      data: { allow_guest_product_browse: true },
    })

    expect(resolveServerAPIBaseURLMock).toHaveBeenCalledTimes(1)
    expect(headersMock).toHaveBeenCalledTimes(1)
    expect(fetchMock).toHaveBeenCalledWith('https://backend.example.com/api/config/public', {
      cache: 'no-store',
      headers: {
        Accept: 'application/json',
        'X-AuraLogic-Locale': 'en',
      },
    })
  })

  test('requires auth for protected server fetches and includes authorization when present', async () => {
    const missingAuthModule = loadServerAPIModule().module
    await expect(missingAuthModule.getServerAnnouncement(12)).rejects.toMatchObject({
      message: 'Authentication required',
      status: 401,
    })

    const fetchMock = jest.fn().mockResolvedValue(
      new Response(JSON.stringify({ data: { id: 12, title: 'Announcement' } }), {
        status: 200,
        headers: {
          'content-type': 'application/json',
        },
      })
    )
    global.fetch = fetchMock as unknown as typeof fetch

    const { module } = loadServerAPIModule({
      requestHeaders: {
        'accept-language': 'zh-CN,zh;q=0.9',
      },
      cookieValues: {
        [AUTH_TOKEN_COOKIE_NAME]: 'secure-cookie-token',
      },
    })

    await expect(module.getServerAnnouncement(12)).resolves.toEqual({
      data: { id: 12, title: 'Announcement' },
    })
    expect(fetchMock).toHaveBeenCalledWith(
      'https://backend.example.com/api/user/announcements/12',
      {
        cache: 'no-store',
        headers: {
          Accept: 'application/json',
          Authorization: 'Bearer secure-cookie-token',
          'X-AuraLogic-Locale': 'zh',
        },
      }
    )
  })

  test('encodes attribute filters for stock lookups and surfaces nested backend errors', async () => {
    const encodedAttributes = encodeURIComponent(JSON.stringify({ region: 'cn', edition: 'pro' }))
    const fetchMock = jest
      .fn()
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ data: { stock: 9 } }), {
          status: 200,
          headers: {
            'content-type': 'application/json',
          },
        })
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            data: {
              error: 'Not allowed',
            },
          }),
          {
            status: 403,
            statusText: 'Forbidden',
            headers: {
              'content-type': 'application/json',
            },
          }
        )
      )
    global.fetch = fetchMock as unknown as typeof fetch

    const { module } = loadServerAPIModule({
      requestHeaders: {
        'accept-language': 'en-US,en;q=0.9',
      },
    })

    await expect(
      module.getServerProductAvailableStock(9, { region: 'cn', edition: 'pro' })
    ).resolves.toEqual({
      data: { stock: 9 },
    })
    expect(fetchMock.mock.calls[0][0]).toBe(
      `https://backend.example.com/api/user/products/9/available-stock?attributes=${encodedAttributes}`
    )

    await expect(module.getServerKnowledgeArticle(88)).rejects.toMatchObject({
      message: 'Authentication required',
      status: 401,
    })

    const authedModule = loadServerAPIModule({
      requestHeaders: {
        'accept-language': 'en-US,en;q=0.9',
      },
      cookieValues: {
        [AUTH_TOKEN_COOKIE_NAME]: 'cookie-token',
      },
    }).module

    await expect(authedModule.getServerKnowledgeArticle(88)).rejects.toMatchObject({
      message: 'Not allowed',
      status: 403,
      data: {
        data: {
          error: 'Not allowed',
        },
      },
    })
  })

  test('fetches protected order resources with the server auth cookie', async () => {
    const fetchMock = jest
      .fn()
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ data: { order_no: 'ORD-1001', status: 'completed' } }), {
          status: 200,
          headers: {
            'content-type': 'application/json',
          },
        })
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ data: { stocks: [{ id: 1, content: 'KEY-001' }] } }), {
          status: 200,
          headers: {
            'content-type': 'application/json',
          },
        })
      )
    global.fetch = fetchMock as unknown as typeof fetch

    const { module } = loadServerAPIModule({
      requestHeaders: {
        'accept-language': 'en-US,en;q=0.9',
      },
      cookieValues: {
        [AUTH_TOKEN_COOKIE_NAME]: 'order-cookie-token',
      },
    })

    await expect(module.getServerOrder('ORD-1001')).resolves.toEqual({
      data: { order_no: 'ORD-1001', status: 'completed' },
    })
    await expect(module.getServerOrderVirtualProducts('ORD-1001')).resolves.toEqual({
      data: { stocks: [{ id: 1, content: 'KEY-001' }] },
    })

    expect(fetchMock.mock.calls[0][0]).toBe('https://backend.example.com/api/user/orders/ORD-1001')
    expect(fetchMock.mock.calls[1][0]).toBe(
      'https://backend.example.com/api/user/orders/ORD-1001/virtual-products'
    )
    expect(fetchMock.mock.calls[0][1]).toEqual({
      cache: 'no-store',
      headers: {
        Accept: 'application/json',
        Authorization: 'Bearer order-cookie-token',
        'X-AuraLogic-Locale': 'en',
      },
    })
  })
})
