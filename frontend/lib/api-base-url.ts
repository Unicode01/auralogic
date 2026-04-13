function normalizeBaseURL(value: string | undefined | null): string {
  return String(value || '')
    .trim()
    .replace(/\/+$/g, '')
}

function resolveURLOrigin(value: string): string {
  const normalized = normalizeBaseURL(value)
  if (!normalized) {
    return ''
  }
  try {
    return normalizeBaseURL(new URL(normalized).origin)
  } catch {
    return ''
  }
}

function resolveURLHostname(value: string): string {
  const normalized = normalizeBaseURL(value)
  if (!normalized) {
    return ''
  }
  try {
    return String(new URL(normalized).hostname || '')
      .trim()
      .toLowerCase()
  } catch {
    return ''
  }
}

function isPrivateHostname(value: string): boolean {
  const hostname = String(value || '')
    .trim()
    .toLowerCase()
  if (!hostname) {
    return false
  }
  if (hostname === 'localhost' || hostname === '::1') {
    return true
  }
  if (/^127(?:\.\d{1,3}){3}$/.test(hostname)) {
    return true
  }
  if (/^10(?:\.\d{1,3}){3}$/.test(hostname)) {
    return true
  }
  if (/^192\.168(?:\.\d{1,3}){2}$/.test(hostname)) {
    return true
  }
  const match = hostname.match(/^172\.(\d{1,3})(?:\.\d{1,3}){2}$/)
  if (!match) {
    return false
  }
  const secondOctet = Number.parseInt(match[1] || '', 10)
  return Number.isFinite(secondOctet) && secondOctet >= 16 && secondOctet <= 31
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
const CONFIGURED_PUBLIC_API_ORIGIN = resolveURLOrigin(CONFIGURED_PUBLIC_API_BASE_URL)
const CONFIGURED_PUBLIC_API_HOSTNAME = resolveURLHostname(CONFIGURED_PUBLIC_API_BASE_URL)
const CONFIGURED_PUBLIC_APP_BASE_URL = normalizeBaseURL(process.env.NEXT_PUBLIC_APP_URL)
const CONFIGURED_PUBLIC_APP_ORIGIN = resolveURLOrigin(CONFIGURED_PUBLIC_APP_BASE_URL)
const CONFIGURED_PUBLIC_APP_HOSTNAME = resolveURLHostname(CONFIGURED_PUBLIC_APP_BASE_URL)
const CLIENT_API_PROXY_BASE_URL = '/api/_backend'

export function getConfiguredPublicAPIBaseURL(): string {
  return CONFIGURED_PUBLIC_API_BASE_URL
}

function getCurrentWindowOrigin(): string {
  if (typeof window === 'undefined' || !window.location?.origin) {
    return ''
  }
  return normalizeBaseURL(window.location.origin)
}

function getCurrentWindowHostname(): string {
  return resolveURLHostname(getCurrentWindowOrigin())
}

function shouldPreferRelativePublicAPIBaseURL(): boolean {
  if (!CONFIGURED_PUBLIC_API_BASE_URL || typeof window === 'undefined') {
    return false
  }

  const currentOrigin = getCurrentWindowOrigin()
  if (currentOrigin && CONFIGURED_PUBLIC_API_ORIGIN && currentOrigin === CONFIGURED_PUBLIC_API_ORIGIN) {
    return true
  }

  if (
    CONFIGURED_PUBLIC_APP_BASE_URL &&
    CONFIGURED_PUBLIC_APP_BASE_URL === CONFIGURED_PUBLIC_API_BASE_URL
  ) {
    return true
  }

  if (!isPrivateHostname(CONFIGURED_PUBLIC_API_HOSTNAME)) {
    return false
  }

  const currentHostname = getCurrentWindowHostname()
  if (
    currentOrigin &&
    CONFIGURED_PUBLIC_API_ORIGIN &&
    currentOrigin !== CONFIGURED_PUBLIC_API_ORIGIN &&
    currentHostname &&
    !isPrivateHostname(currentHostname)
  ) {
    return true
  }

  if (
    CONFIGURED_PUBLIC_APP_ORIGIN &&
    CONFIGURED_PUBLIC_API_ORIGIN &&
    CONFIGURED_PUBLIC_APP_ORIGIN !== CONFIGURED_PUBLIC_API_ORIGIN &&
    CONFIGURED_PUBLIC_APP_HOSTNAME &&
    !isPrivateHostname(CONFIGURED_PUBLIC_APP_HOSTNAME)
  ) {
    return true
  }

  return false
}

export function getEffectivePublicAPIBaseURL(): string {
  if (!CONFIGURED_PUBLIC_API_BASE_URL) {
    return ''
  }
  if (shouldPreferRelativePublicAPIBaseURL()) {
    return ''
  }
  return CONFIGURED_PUBLIC_API_BASE_URL
}

export function getPublicAbsoluteAPIBaseURL(): string {
  const effectiveBaseURL = getEffectivePublicAPIBaseURL()
  if (effectiveBaseURL) {
    return effectiveBaseURL
  }
  const currentOrigin = getCurrentWindowOrigin()
  if (currentOrigin) {
    return currentOrigin
  }
  return CONFIGURED_PUBLIC_API_BASE_URL
}

export function resolvePublicAPIURL(path: string): string {
  return joinBaseURL(getEffectivePublicAPIBaseURL(), path)
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
