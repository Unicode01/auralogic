import 'server-only'

import { headers } from 'next/headers'
import { getConfiguredPublicAPIBaseURL } from '@/lib/api-base-url'

const INTERNAL_API_BASE_URL = normalizeBaseURL(process.env.INTERNAL_API_BASE_URL)

function normalizeBaseURL(value: string | undefined | null): string {
  return String(value || '')
    .trim()
    .replace(/\/+$/g, '')
}

function firstForwardedValue(value: string | null | undefined): string {
  return String(value || '')
    .split(',')[0]
    .trim()
}

function resolveBaseURLFromHeaderSource(
  getHeader: (name: string) => string | null | undefined
): string {
  if (INTERNAL_API_BASE_URL) {
    return INTERNAL_API_BASE_URL
  }

  const configuredBaseURL = normalizeBaseURL(getConfiguredPublicAPIBaseURL())
  if (configuredBaseURL) {
    return configuredBaseURL
  }

  const forwardedHost = firstForwardedValue(getHeader('x-forwarded-host'))
  const host = firstForwardedValue(getHeader('host'))
  const resolvedHost = forwardedHost || host
  if (!resolvedHost) {
    throw new Error('Unable to resolve server API base URL')
  }

  const forwardedProto = firstForwardedValue(getHeader('x-forwarded-proto'))
  const protocol =
    forwardedProto ||
    (resolvedHost.startsWith('localhost') || resolvedHost.startsWith('127.0.0.1')
      ? 'http'
      : 'https')

  return `${protocol}://${resolvedHost}`
}

export async function resolveServerAPIBaseURL(): Promise<string> {
  const requestHeaders = await headers()
  return resolveBaseURLFromHeaderSource((name) => requestHeaders.get(name))
}

export function resolveServerAPIBaseURLFromRequest(request: Request): string {
  return resolveBaseURLFromHeaderSource((name) => request.headers.get(name))
}
