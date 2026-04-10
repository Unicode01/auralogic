import 'server-only'

import { headers } from 'next/headers'
import { getConfiguredPublicAPIBaseURL } from '@/lib/api-base-url'

const APP_LOCALE_HEADER = 'X-AuraLogic-Locale'

function normalizeBaseURL(value: string | undefined | null): string {
  return String(value || '')
    .trim()
    .replace(/\/+$/g, '')
}

function normalizePath(path: string): string {
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
  const normalizedPath = normalizePath(path)
  if (!normalizedPath) {
    return baseURL
  }
  if (/^https?:\/\//i.test(normalizedPath)) {
    return normalizedPath
  }
  return `${baseURL}${normalizedPath}`
}

function normalizeLocale(value: string | null | undefined): string | undefined {
  const normalized = String(value || '')
    .trim()
    .toLowerCase()

  if (!normalized) {
    return undefined
  }
  if (normalized === 'zh' || normalized.startsWith('zh')) {
    return 'zh'
  }
  if (normalized === 'en' || normalized.startsWith('en')) {
    return 'en'
  }
  return undefined
}

function firstForwardedValue(value: string | null | undefined): string {
  return String(value || '')
    .split(',')[0]
    .trim()
}

async function resolveServerPublicAPIBaseURL(): Promise<string> {
  const configuredBaseURL = normalizeBaseURL(getConfiguredPublicAPIBaseURL())
  if (configuredBaseURL) {
    return configuredBaseURL
  }

  const requestHeaders = await headers()
  const forwardedHost = firstForwardedValue(requestHeaders.get('x-forwarded-host'))
  const host = firstForwardedValue(requestHeaders.get('host'))
  const resolvedHost = forwardedHost || host
  if (!resolvedHost) {
    throw new Error('Unable to resolve server public API base URL')
  }

  const forwardedProto = firstForwardedValue(requestHeaders.get('x-forwarded-proto'))
  const protocol =
    forwardedProto || (resolvedHost.startsWith('localhost') || resolvedHost.startsWith('127.0.0.1') ? 'http' : 'https')

  return `${protocol}://${resolvedHost}`
}

async function resolveServerLocaleHeader(): Promise<string | undefined> {
  const requestHeaders = await headers()
  return (
    normalizeLocale(requestHeaders.get(APP_LOCALE_HEADER)) ||
    normalizeLocale(requestHeaders.get('accept-language'))
  )
}

function extractServerErrorMessage(payload: any, statusText: string): string {
  return (
    payload?.message ||
    payload?.error ||
    payload?.data?.message ||
    payload?.data?.error ||
    statusText ||
    'Request failed'
  )
}

async function fetchServerPublicAPI(path: string) {
  const [baseURL, locale] = await Promise.all([
    resolveServerPublicAPIBaseURL(),
    resolveServerLocaleHeader(),
  ])
  const response = await fetch(joinBaseURL(baseURL, path), {
    cache: 'no-store',
    headers: {
      Accept: 'application/json',
      ...(locale ? { [APP_LOCALE_HEADER]: locale } : {}),
    },
  })

  const payload = await response.json().catch(() => null)
  if (!response.ok) {
    const error: any = new Error(extractServerErrorMessage(payload, response.statusText))
    error.status = response.status
    error.data = payload
    throw error
  }

  return payload
}

export function getServerPublicConfig() {
  return fetchServerPublicAPI('/api/config/public')
}

export function getServerProduct(productId: number) {
  return fetchServerPublicAPI(`/api/user/products/${productId}`)
}

export function getServerProductAvailableStock(
  productId: number,
  attributes?: Record<string, string>
) {
  let path = `/api/user/products/${productId}/available-stock`
  if (attributes && Object.keys(attributes).length > 0) {
    path += `?attributes=${encodeURIComponent(JSON.stringify(attributes))}`
  }
  return fetchServerPublicAPI(path)
}
