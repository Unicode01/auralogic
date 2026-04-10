function normalizeBaseURL(value: string | undefined | null): string {
  return String(value || '')
    .trim()
    .replace(/\/+$/g, '')
}

function normalizeAPIPath(path: string): string {
  const trimmed = String(path || '').trim()
  if (!trimmed) {
    return ''
  }
  if (/^https?:\/\//i.test(trimmed)) {
    return trimmed
  }
  return trimmed.startsWith('/') ? trimmed : `/${trimmed}`
}

function joinBaseURL(baseURL: string, path: string): string {
  const normalizedPath = normalizeAPIPath(path)
  if (!normalizedPath) {
    return baseURL
  }
  if (/^https?:\/\//i.test(normalizedPath)) {
    return normalizedPath
  }
  return `${baseURL}${normalizedPath}`
}

const CONFIGURED_PUBLIC_API_BASE_URL = normalizeBaseURL(process.env.NEXT_PUBLIC_API_URL)
const CLIENT_API_PROXY_BASE_URL = '/api/_backend'

export function getConfiguredPublicAPIBaseURL(): string {
  return CONFIGURED_PUBLIC_API_BASE_URL
}

export function getPublicAbsoluteAPIBaseURL(): string {
  if (CONFIGURED_PUBLIC_API_BASE_URL) {
    return CONFIGURED_PUBLIC_API_BASE_URL
  }
  if (typeof window !== 'undefined' && window.location?.origin) {
    return normalizeBaseURL(window.location.origin)
  }
  return ''
}

export function resolvePublicAPIURL(path: string): string {
  return joinBaseURL(CONFIGURED_PUBLIC_API_BASE_URL, path)
}

export function resolvePublicAbsoluteAPIURL(path: string): string {
  return joinBaseURL(getPublicAbsoluteAPIBaseURL(), path)
}

export function getClientAPIProxyBaseURL(): string {
  return CLIENT_API_PROXY_BASE_URL
}

export function resolveClientAPIProxyURL(path: string): string {
  return joinBaseURL(CLIENT_API_PROXY_BASE_URL, path)
}
