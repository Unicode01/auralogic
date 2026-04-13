/** @jest-environment node */

import { AUTH_SESSION_HINT_COOKIE_NAME, AUTH_TOKEN_COOKIE_NAME } from '@/lib/auth'

jest.mock('@/lib/server-api-base-url', () => ({
  resolveServerAPIBaseURLFromRequest: jest.fn(() => 'https://backend.example.com'),
}))

import { GET, POST } from './route'

function createJWT(exp: number): string {
  const header = Buffer.from(JSON.stringify({ alg: 'HS256', typ: 'JWT' })).toString('base64url')
  const payload = Buffer.from(JSON.stringify({ exp })).toString('base64url')
  return `${header}.${payload}.signature`
}

describe('backend proxy route', () => {
  const originalFetch = global.fetch
  const fetchMock = jest.fn()

  beforeEach(() => {
    fetchMock.mockReset()
    global.fetch = fetchMock as unknown as typeof fetch
  })

  afterAll(() => {
    global.fetch = originalFetch
  })

  test('persists issued auth cookies, strips token from JSON payload and filters headers', async () => {
    const token = createJWT(Math.floor(Date.now() / 1000) + 600)
    fetchMock.mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          data: {
            token,
            user: { id: 1, role: 'admin' },
          },
        }),
        {
          status: 200,
          headers: {
            connection: 'keep-alive',
            'content-type': 'application/json; charset=utf-8',
            'set-cookie': 'upstream=1',
            'x-trace-id': 'trace-123',
          },
        }
      )
    )

    const request = new Request('https://frontend.example.com/api/_backend/api/user/auth/login', {
      method: 'POST',
      headers: {
        'content-type': 'application/json',
        cookie: 'theme=dark',
        'cf-connecting-ip': '203.0.113.8',
        'x-forwarded-for': '203.0.113.8, 10.0.0.2',
        'x-forwarded-proto': 'https',
        'x-real-ip': '203.0.113.8',
        'x-session-id': 'client-session',
      },
      body: JSON.stringify({ email: 'admin@example.com', password: 'secret' }),
    })

    const response = await POST(request, {
      params: Promise.resolve({ path: ['api', 'user', 'auth', 'login'] }),
    })

    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(fetchMock.mock.calls[0][0]).toBe('https://backend.example.com/api/user/auth/login')
    const upstreamInit = fetchMock.mock.calls[0][1] as RequestInit
    const upstreamHeaders = upstreamInit.headers as Headers
    expect(upstreamHeaders.get('content-type')).toBe('application/json')
    expect(upstreamHeaders.get('cf-connecting-ip')).toBe('203.0.113.8')
    expect(upstreamHeaders.get('x-forwarded-for')).toBe('203.0.113.8, 10.0.0.2')
    expect(upstreamHeaders.get('x-real-ip')).toBe('203.0.113.8')
    expect(upstreamHeaders.get('x-session-id')).toBe('client-session')
    expect(upstreamHeaders.get('cookie')).toBeNull()

    expect(response.headers.get('x-trace-id')).toBe('trace-123')
    expect(response.headers.get('connection')).toBeNull()
    expect(response.headers.getSetCookie().every((value) => !value.includes('upstream=1'))).toBe(
      true
    )

    expect(response.cookies.get(AUTH_TOKEN_COOKIE_NAME)?.value).toBe(token)
    expect(response.cookies.get(AUTH_SESSION_HINT_COOKIE_NAME)?.value).toBe('1')

    await expect(response.json()).resolves.toEqual({
      data: {
        user: { id: 1, role: 'admin' },
      },
    })
  })

  test('clears session cookies for unauthorized upstream responses', async () => {
    fetchMock.mockResolvedValueOnce(
      new Response(JSON.stringify({ message: 'unauthorized' }), {
        status: 401,
        headers: {
          'content-type': 'application/json',
        },
      })
    )

    const request = new Request('https://frontend.example.com/api/_backend/api/orders', {
      headers: {
        cookie: `${AUTH_TOKEN_COOKIE_NAME}=stale-token; ${AUTH_SESSION_HINT_COOKIE_NAME}=1`,
        'x-forwarded-proto': 'https',
      },
    })

    const response = await GET(request, {
      params: Promise.resolve({ path: ['api', 'orders'] }),
    })

    const setCookies = response.headers.getSetCookie()
    expect(setCookies).toEqual(
      expect.arrayContaining([
        expect.stringContaining(`${AUTH_TOKEN_COOKIE_NAME}=`),
        expect.stringContaining(`${AUTH_SESSION_HINT_COOKIE_NAME}=`),
      ])
    )
    expect(setCookies.every((value) => value.includes('Max-Age=0'))).toBe(true)
  })

  test('adopts a legacy bearer token into cookies on successful auth me responses', async () => {
    const token = createJWT(Math.floor(Date.now() / 1000) + 300)
    fetchMock.mockResolvedValueOnce(
      new Response(JSON.stringify({ data: { id: 9, email: 'legacy@example.com' } }), {
        status: 200,
        headers: {
          'content-type': 'application/json',
        },
      })
    )

    const request = new Request('https://frontend.example.com/api/_backend/api/user/auth/me', {
      headers: {
        authorization: `Bearer ${token}`,
        'x-forwarded-proto': 'https',
      },
    })

    const response = await GET(request, {
      params: Promise.resolve({ path: ['api', 'user', 'auth', 'me'] }),
    })

    const upstreamInit = fetchMock.mock.calls[0][1] as RequestInit
    const upstreamHeaders = upstreamInit.headers as Headers
    expect(upstreamHeaders.get('authorization')).toBe(`Bearer ${token}`)
    expect(response.cookies.get(AUTH_TOKEN_COOKIE_NAME)?.value).toBe(token)
    expect(response.cookies.get(AUTH_SESSION_HINT_COOKIE_NAME)?.value).toBe('1')
    await expect(response.json()).resolves.toEqual({
      data: { id: 9, email: 'legacy@example.com' },
    })
  })
})
