import { AUTH_COOKIE_DEFAULT_MAX_AGE_SECONDS } from '@/lib/auth'

type RecordLike = Record<string, unknown>

export const TOKEN_ISSUING_PATHS = new Set([
  '/api/user/auth/login',
  '/api/user/auth/register',
  '/api/user/auth/login-with-code',
  '/api/user/auth/login-with-phone-code',
  '/api/user/auth/phone-register',
  '/api/user/auth/verify-email',
])

export const FORWARDED_REQUEST_HEADERS = [
  'accept',
  'accept-language',
  'cf-connecting-ip',
  'content-type',
  'user-agent',
  'x-auralogic-locale',
  'x-forwarded-for',
  'x-real-ip',
  'x-session-id',
]

export const HOP_BY_HOP_RESPONSE_HEADERS = new Set([
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

function isRecordLike(value: unknown): value is RecordLike {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}

export function joinBaseURL(baseURL: string, path: string): string {
  return `${String(baseURL || '').replace(/\/+$/g, '')}${path.startsWith('/') ? path : `/${path}`}`
}

export function getBearerToken(value: string | null | undefined): string | undefined {
  const normalized = String(value || '').trim()
  if (!normalized) return undefined
  const parts = normalized.split(/\s+/, 2)
  if (parts.length !== 2 || parts[0].toLowerCase() !== 'bearer') {
    return undefined
  }
  const token = parts[1].trim()
  return token || undefined
}

export function readRequestCookie(
  request: Pick<Request, 'headers'>,
  name: string
): string | undefined {
  const cookieHeader = request.headers.get('cookie') || ''
  for (const part of cookieHeader.split(';')) {
    const normalized = part.trim()
    if (!normalized.startsWith(`${name}=`)) continue
    const value = normalized.slice(name.length + 1).trim()
    return value ? decodeURIComponent(value) : undefined
  }
  return undefined
}

export function readPayloadToken(payload: unknown): string | undefined {
  if (!isRecordLike(payload)) {
    return undefined
  }
  const nestedData = isRecordLike(payload.data) ? payload.data : undefined
  const token =
    (typeof nestedData?.token === 'string' ? nestedData.token : undefined) ||
    (typeof payload.token === 'string' ? payload.token : undefined)
  return token?.trim() || undefined
}

export function stripPayloadToken<T>(payload: T): T {
  if (!isRecordLike(payload)) {
    return payload
  }
  const root: RecordLike = { ...payload }
  delete root.token
  if (isRecordLike(root.data)) {
    const nextData: RecordLike = { ...root.data }
    delete nextData.token
    root.data = nextData
  }
  return root as T
}

export function deriveCookieMaxAge(token: string, nowMs: number = Date.now()): number {
  try {
    const payloadSegment = token.split('.')[1]
    if (!payloadSegment) {
      return AUTH_COOKIE_DEFAULT_MAX_AGE_SECONDS
    }
    const decoded = JSON.parse(Buffer.from(payloadSegment, 'base64url').toString('utf8'))
    const exp = Number((decoded as RecordLike)?.exp)
    if (!Number.isFinite(exp)) {
      return AUTH_COOKIE_DEFAULT_MAX_AGE_SECONDS
    }
    return Math.max(
      0,
      Math.min(AUTH_COOKIE_DEFAULT_MAX_AGE_SECONDS, exp - Math.floor(nowMs / 1000))
    )
  } catch {
    return AUTH_COOKIE_DEFAULT_MAX_AGE_SECONDS
  }
}

export function isSecureRequest(request: Pick<Request, 'headers' | 'url'>): boolean {
  return request.headers.get('x-forwarded-proto') === 'https' || request.url.startsWith('https://')
}

export function copyProxyRequestHeaders(
  request: Pick<Request, 'headers'>,
  bearerToken?: string
): Headers {
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

export function copyProxyResponseHeaders(source: Headers): Headers {
  const headers = new Headers()
  source.forEach((value, key) => {
    if (!HOP_BY_HOP_RESPONSE_HEADERS.has(key.toLowerCase())) {
      headers.set(key, value)
    }
  })
  return headers
}

export function shouldAdoptLegacyBearer(
  path: string,
  status: number,
  legacyBearerToken?: string
): boolean {
  return Boolean(legacyBearerToken && status < 400 && path === '/api/user/auth/me')
}

export function shouldClearSession(path: string, status: number): boolean {
  return status === 401 || path === '/api/user/auth/logout'
}
