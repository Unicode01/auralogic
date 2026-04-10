import { NextResponse } from 'next/server'
import { AUTH_SESSION_HINT_COOKIE_NAME, AUTH_TOKEN_COOKIE_NAME } from '@/lib/auth'
import {
  TOKEN_ISSUING_PATHS,
  copyProxyRequestHeaders,
  copyProxyResponseHeaders,
  deriveCookieMaxAge,
  getBearerToken,
  isSecureRequest,
  joinBaseURL,
  readPayloadToken,
  readRequestCookie,
  shouldAdoptLegacyBearer,
  shouldClearSession,
  stripPayloadToken,
} from '@/lib/backend-proxy'
import { resolveServerAPIBaseURLFromRequest } from '@/lib/server-api-base-url'

function applySessionCookies(response: NextResponse, token: string, request: Request) {
  const secure = isSecureRequest(request)
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
  const secure = isSecureRequest(request)
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

  const authCookieToken = readRequestCookie(request, AUTH_TOKEN_COOKIE_NAME)
  const legacyBearerToken = getBearerToken(request.headers.get('authorization'))
  const bearerToken = authCookieToken || legacyBearerToken
  const hasBody = request.method !== 'GET' && request.method !== 'HEAD'
  const upstreamResponse = await fetch(upstreamURL, {
    method: request.method,
    headers: copyProxyRequestHeaders(request, bearerToken),
    body: hasBody ? await request.arrayBuffer() : undefined,
    cache: 'no-store',
    redirect: 'manual',
  })

  const contentType = upstreamResponse.headers.get('content-type') || ''
  let response: NextResponse

  if (contentType.includes('application/json')) {
    const payload = await upstreamResponse.json().catch(() => null)
    const issuedToken = TOKEN_ISSUING_PATHS.has(normalizedPath)
      ? readPayloadToken(payload)
      : undefined
    const adoptedLegacyToken = shouldAdoptLegacyBearer(
      normalizedPath,
      upstreamResponse.status,
      authCookieToken ? undefined : legacyBearerToken
    )
      ? legacyBearerToken
      : undefined
    const tokenToPersist = issuedToken || adoptedLegacyToken
    response = NextResponse.json(issuedToken ? stripPayloadToken(payload) : (payload ?? {}), {
      status: upstreamResponse.status,
      headers: copyProxyResponseHeaders(upstreamResponse.headers),
    })
    if (tokenToPersist) {
      applySessionCookies(response, tokenToPersist, request)
    }
  } else {
    response = new NextResponse(upstreamResponse.body, {
      status: upstreamResponse.status,
      headers: copyProxyResponseHeaders(upstreamResponse.headers),
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
