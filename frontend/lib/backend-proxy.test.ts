/** @jest-environment node */

import { AUTH_COOKIE_DEFAULT_MAX_AGE_SECONDS } from '@/lib/auth'
import {
  copyProxyRequestHeaders,
  copyProxyResponseHeaders,
  deriveCookieMaxAge,
  stripPayloadToken,
} from '@/lib/backend-proxy'

function createJWT(exp: number): string {
  const header = Buffer.from(JSON.stringify({ alg: 'HS256', typ: 'JWT' })).toString('base64url')
  const payload = Buffer.from(JSON.stringify({ exp })).toString('base64url')
  return `${header}.${payload}.signature`
}

describe('backend-proxy helpers', () => {
  test('stripPayloadToken removes token fields without mutating the input payload', () => {
    const payload = {
      token: 'root-token',
      data: {
        token: 'nested-token',
        user: { id: 7 },
      },
    }

    const stripped = stripPayloadToken(payload)

    expect(stripped).toEqual({
      data: {
        user: { id: 7 },
      },
    })
    expect(payload.token).toBe('root-token')
    expect(payload.data.token).toBe('nested-token')
  })

  test('deriveCookieMaxAge clamps JWT expiration to the configured max age', () => {
    const nowMs = Date.UTC(2026, 0, 1, 0, 0, 0)
    const nowSeconds = Math.floor(nowMs / 1000)

    expect(deriveCookieMaxAge(createJWT(nowSeconds + 90), nowMs)).toBe(90)
    expect(
      deriveCookieMaxAge(createJWT(nowSeconds + AUTH_COOKIE_DEFAULT_MAX_AGE_SECONDS + 300), nowMs)
    ).toBe(AUTH_COOKIE_DEFAULT_MAX_AGE_SECONDS)
    expect(deriveCookieMaxAge(createJWT(nowSeconds - 10), nowMs)).toBe(0)
    expect(deriveCookieMaxAge('not-a-jwt', nowMs)).toBe(AUTH_COOKIE_DEFAULT_MAX_AGE_SECONDS)
  })

  test('copies allowlisted request headers and filters hop-by-hop response headers', () => {
    const request = new Request('https://frontend.example.com/api/_backend/api/orders', {
      headers: {
        accept: 'application/json',
        'accept-language': 'zh-CN',
        'cf-connecting-ip': '203.0.113.8',
        'content-type': 'application/json',
        cookie: 'ignored=1',
        'x-forwarded-for': '203.0.113.8, 10.0.0.2',
        'x-real-ip': '203.0.113.8',
        'x-session-id': 'session-123',
      },
    })

    const forwardedRequestHeaders = copyProxyRequestHeaders(request, 'token-123')
    expect(forwardedRequestHeaders.get('accept')).toBe('application/json')
    expect(forwardedRequestHeaders.get('accept-language')).toBe('zh-CN')
    expect(forwardedRequestHeaders.get('cf-connecting-ip')).toBe('203.0.113.8')
    expect(forwardedRequestHeaders.get('content-type')).toBe('application/json')
    expect(forwardedRequestHeaders.get('x-forwarded-for')).toBe('203.0.113.8, 10.0.0.2')
    expect(forwardedRequestHeaders.get('x-real-ip')).toBe('203.0.113.8')
    expect(forwardedRequestHeaders.get('x-session-id')).toBe('session-123')
    expect(forwardedRequestHeaders.get('authorization')).toBe('Bearer token-123')
    expect(forwardedRequestHeaders.get('cookie')).toBeNull()

    const upstreamHeaders = new Headers({
      connection: 'keep-alive',
      'content-type': 'application/json',
      'set-cookie': 'upstream=1',
      'x-trace-id': 'trace-123',
    })
    const proxiedResponseHeaders = copyProxyResponseHeaders(upstreamHeaders)

    expect(proxiedResponseHeaders.get('content-type')).toBe('application/json')
    expect(proxiedResponseHeaders.get('x-trace-id')).toBe('trace-123')
    expect(proxiedResponseHeaders.get('connection')).toBeNull()
    expect(proxiedResponseHeaders.get('set-cookie')).toBeNull()
  })
})
