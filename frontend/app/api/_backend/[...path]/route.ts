import { NextResponse } from 'next/server'
import {
  AUTH_COOKIE_DEFAULT_MAX_AGE_SECONDS,
  AUTH_SESSION_HINT_COOKIE_NAME,
  AUTH_TOKEN_COOKIE_NAME,
} from '@/lib/auth'
import { resolveServerAPIBaseURLFromRequest } from '@/lib/server-api-base-url'

const TOKEN_ISSUING_PATHS = new Set([
  '/api/user/auth/login',
  '/api/user/auth/register',
  '/api/user/auth/login-with-code',
  '/api/user/auth/login-with-phone-code',
  '/api/user/auth/phone-register',
  '/api/user/auth/verify-email',
])

const FORWARDED_REQUEST_HEADERS = [
  'accept',
  'accept-language',
  'content-type',
  'user-agent',
  'x-auralogic-locale',
  'x-session-id',
]

const HOP_BY_HOP_RESPONSE_HEADERS = new Set([
  'connection',
  'content-length',
  'keep-alive',
  'proxy-authenticate',
  'proxy-authorization',
  'set-cookie',
  'te',
  'trailer',
  'transfer-encoding',
  'upgrade',
])

function joinBaseURL(baseURL: string, path: string): string {
  return `${String(baseURL || '').replace(/\/+$/g, '')}${path.startsWith('/') ? path : `/${path}`}`
}

function getBearerToken(value: string | null | undefined): string | undefined {
  const normalized = String(value || '').trim()
  if (!normalized) return undefined
  const parts = normalized.split(/\s+/, 2)
  if (parts.length !== 2 || parts[0].toLowerCase() !== 'bearer') {
    return undefined
  }
  const token = parts[1].trim()
  return token || undefined
}

function readCookieToken(request: Request, name: string): string | undefined {
  const cookieHeader = request.headers.get('cookie') || ''
  for (const part of cookieHeader.split(';')) {
    const normalized = part.trim()
    if (!normalized.startsWith(`${name}=`)) continue
    const value = normalized.slice(name.length + 1).trim()
    return value ? decodeURIComponent(value) : undefined
  }
  return undefined
}

function readPayloadToken(payload: any): string | undefined {
  const token =
    (typeof payload?.data?.token === 'string' ? payload.data.token : undefined) ||
    (typeof payload?.token === 'string' ? payload.token : undefined)
  return token?.trim() || undefined
}

function stripPayloadToken(payload: any): any {
  if (!payload || typeof payload !== 'object') {
    return payload
  }
  const root = { ...payload }
  if ('token' in root) {
    delete root.token
  }
  if (root.data && typeof root.data === 'object' && !Array.isArray(root.data)) {
    const nextData = { ...root.data }
    if ('token' in nextData) {
      delete nextData.token
    }
    root.data = nextData
  }
  return root
}

function deriveCookieMaxAge(token: string): number {
  try {
    const payloadSegment = token.split('.')[1]
    if (!payloadSegment) {
      return AUTH_COOKIE_DEFAULT_MAX_AGE_SECONDS
    }
    const decoded = JSON.parse(Buffer.from(payloadSegment, 'base64url').toString('utf8'))
    const exp = Number(decoded?.exp)
    if (!Number.isFinite(exp)) {
      return AUTH_COOKIE_DEFAULT_MAX_AGE_SECONDS
    }
    return Math.max(
      0,
      Math.min(
        AUTH_COOKIE_DEFAULT_MAX_AGE_SECONDS,
        exp - Math.floor(Date.now() / 1000)
      )
    )
  } catch {
    return AUTH_COOKIE_DEFAULT_MAX_AGE_SECONDS
  }
}

function applySessionCookies(response: NextResponse, token: string, request: Request) {
  const secure =
    request.headers.get('x-forwarded-proto') === 'https' || request.url.startsWith('https://')
  const maxAge = deriveCookieMaxAge(token)
  response.cookies.set({
    name: AUTH_TOKEN_COOKIE_NAME,
    value: token,
    httpOnly: true,
    sameSite: 'lax',
    secure,
    path: '/',
    maxAge,
  })
  response.cookies.set({
    name: AUTH_SESSION_HINT_COOKIE_NAME,
    value: '1',
    httpOnly: false,
    sameSite: 'lax',
    secure,
    path: '/',
    maxAge,
  })
}

function clearSessionCookies(response: NextResponse, request: Request) {
  const secure =
    request.headers.get('x-forwarded-proto') === 'https' || request.url.startsWith('https://')
  response.cookies.set({
    name: AUTH_TOKEN_COOKIE_NAME,
    value: '',
    httpOnly: true,
    sameSite: 'lax',
    secure,
    path: '/',
    maxAge: 0,
  })
  response.cookies.set({
    name: AUTH_SESSION_HINT_COOKIE_NAME,
    value: '',
    httpOnly: false,
    sameSite: 'lax',
    secure,
    path: '/',
    maxAge: 0,
  })
}

function copyRequestHeaders(request: Request, bearerToken?: string): Headers {
  const headers = new Headers()
  for (const name of FORWARDED_REQUEST_HEADERS) {
    const value = request.headers.get(name)
    if (value) {
      headers.set(name, value)
    }
  }
  if (bearerToken) {
    headers.set('authorization', `Bearer ${bearerToken}`)
  }
  return headers
}

function copyResponseHeaders(source: Headers): Headers {
  const headers = new Headers()
  source.forEach((value, key) => {
    if (!HOP_BY_HOP_RESPONSE_HEADERS.has(key.toLowerCase())) {
      headers.set(key, value)
    }
  })
  return headers
}

function shouldAdoptLegacyBearer(path: string, status: number, legacyBearerToken?: string): boolean {
  return Boolean(legacyBearerToken && status < 400 && path === '/api/user/auth/me')
}

function shouldClearSession(path: string, status: number): boolean {
  return status === 401 || path === '/api/user/auth/logout'
}

async function handleProxyRequest(
  request: Request,
  { params }: { params: Promise<{ path: string[] }> }
) {
  const { path } = await params
  const normalizedPath = `/${(path || []).join('/')}`
  const requestURL = new URL(request.url)
  const upstreamURL = joinBaseURL(
    resolveServerAPIBaseURLFromRequest(request),
    `${normalizedPath}${requestURL.search}`
  )

  const authCookieToken = readCookieToken(request, AUTH_TOKEN_COOKIE_NAME)
  const legacyBearerToken = getBearerToken(request.headers.get('authorization'))
  const bearerToken = authCookieToken || legacyBearerToken
  const hasBody = request.method !== 'GET' && request.method !== 'HEAD'
  const upstreamResponse = await fetch(upstreamURL, {
    method: request.method,
    headers: copyRequestHeaders(request, bearerToken),
    body: hasBody ? await request.arrayBuffer() : undefined,
    cache: 'no-store',
    redirect: 'manual',
  })

  const contentType = upstreamResponse.headers.get('content-type') || ''
  let response: NextResponse

  if (contentType.includes('application/json')) {
    const payload = await upstreamResponse.json().catch(() => null)
    const issuedToken = TOKEN_ISSUING_PATHS.has(normalizedPath) ? readPayloadToken(payload) : undefined
    const adoptedLegacyToken = shouldAdoptLegacyBearer(
      normalizedPath,
      upstreamResponse.status,
      authCookieToken ? undefined : legacyBearerToken
    )
      ? legacyBearerToken
      : undefined
    const tokenToPersist = issuedToken || adoptedLegacyToken
    response = NextResponse.json(issuedToken ? stripPayloadToken(payload) : payload ?? {}, {
      status: upstreamResponse.status,
      headers: copyResponseHeaders(upstreamResponse.headers),
    })
    if (tokenToPersist) {
      applySessionCookies(response, tokenToPersist, request)
    }
  } else {
    response = new NextResponse(upstreamResponse.body, {
      status: upstreamResponse.status,
      headers: copyResponseHeaders(upstreamResponse.headers),
    })
    if (
      shouldAdoptLegacyBearer(
        normalizedPath,
        upstreamResponse.status,
        authCookieToken ? undefined : legacyBearerToken
      ) &&
      legacyBearerToken
    ) {
      applySessionCookies(response, legacyBearerToken, request)
    }
  }

  if (shouldClearSession(normalizedPath, upstreamResponse.status)) {
    clearSessionCookies(response, request)
  }

  return response
}

export const GET = handleProxyRequest
export const POST = handleProxyRequest
export const PUT = handleProxyRequest
export const PATCH = handleProxyRequest
export const DELETE = handleProxyRequest
export const OPTIONS = handleProxyRequest
