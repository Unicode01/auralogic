import 'server-only'

import { cookies, headers } from 'next/headers'
import { AUTH_TOKEN_COOKIE_NAME } from '@/lib/auth'
import { resolveServerAPIBaseURL } from '@/lib/server-api-base-url'
import type { OrderQueryParams } from '@/types/order'

const APP_LOCALE_HEADER = 'X-AuraLogic-Locale'

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

async function resolveServerLocaleHeader(): Promise<string | undefined> {
  const requestHeaders = await headers()
  return (
    normalizeLocale(requestHeaders.get(APP_LOCALE_HEADER)) ||
    normalizeLocale(requestHeaders.get('accept-language'))
  )
}

export async function getServerAuthToken(): Promise<string | undefined> {
  const cookieStore = await cookies()
  const token = cookieStore.get(AUTH_TOKEN_COOKIE_NAME)?.value?.trim()
  return token || undefined
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

async function fetchServerAPI(path: string, options?: { auth?: boolean }) {
  const [baseURL, locale, authToken] = await Promise.all([
    resolveServerAPIBaseURL(),
    resolveServerLocaleHeader(),
    options?.auth ? getServerAuthToken() : Promise.resolve(undefined),
  ])
  if (options?.auth && !authToken) {
    const error: any = new Error('Authentication required')
    error.status = 401
    throw error
  }
  const response = await fetch(joinBaseURL(baseURL, path), {
    cache: 'no-store',
    headers: {
      Accept: 'application/json',
      ...(locale ? { [APP_LOCALE_HEADER]: locale } : {}),
      ...(authToken ? { Authorization: `Bearer ${authToken}` } : {}),
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

function buildOrderListPath(params?: OrderQueryParams): string {
  const query = new URLSearchParams()
  if (params?.page) query.set('page', String(params.page))
  if (params?.limit) query.set('limit', String(params.limit))
  if (params?.status) query.set('status', params.status)
  if (params?.search) query.set('search', params.search)
  const search = query.toString()
  return search ? `/api/user/orders?${search}` : '/api/user/orders'
}

export function getServerPublicConfig() {
  return fetchServerAPI('/api/config/public')
}

export function getServerProduct(productId: number) {
  return fetchServerAPI(`/api/user/products/${productId}`)
}

export function getServerProductAvailableStock(
  productId: number,
  attributes?: Record<string, string>
) {
  let path = `/api/user/products/${productId}/available-stock`
  if (attributes && Object.keys(attributes).length > 0) {
    path += `?attributes=${encodeURIComponent(JSON.stringify(attributes))}`
  }
  return fetchServerAPI(path)
}

export function getServerAnnouncement(announcementId: number) {
  return fetchServerAPI(`/api/user/announcements/${announcementId}`, { auth: true })
}

export function getServerKnowledgeArticle(articleId: number) {
  return fetchServerAPI(`/api/user/knowledge/articles/${articleId}`, { auth: true })
}

export function getServerOrders(params?: OrderQueryParams) {
  return fetchServerAPI(buildOrderListPath(params), { auth: true })
}

export function getServerOrder(orderNo: string) {
  return fetchServerAPI(`/api/user/orders/${orderNo}`, { auth: true })
}

export function getServerOrderVirtualProducts(orderNo: string) {
  return fetchServerAPI(`/api/user/orders/${orderNo}/virtual-products`, { auth: true })
}
