'use client'

import type { QueryClient } from '@tanstack/react-query'

import { getPublicConfig } from '@/lib/api'
import { buildPluginBootstrapQueryOptions } from '@/lib/plugin-bootstrap-query'
import {
  buildPluginFullPath,
  normalizePluginPath,
  readPluginSearchParams,
} from '@/lib/plugin-frontend-routing'
import { sanitizePluginLinkUrl } from '@/lib/plugin-link-sanitize'
import { resolvePluginPlatformEnabled } from '@/lib/plugin-slot-behavior'
import { buildPluginSlotQueryOptions } from '@/lib/plugin-slot-query'
import { type PluginSlotRequestScope } from '@/lib/plugin-slot-request'

export type PluginPagePrefetchTarget = {
  href: string
  path: string
  queryParams: Record<string, string>
  scope: PluginSlotRequestScope
}

function resolvePluginPagePrefetchBaseURL(baseHref?: string): URL | null {
  try {
    if (baseHref) {
      return new URL(baseHref)
    }
    if (typeof window !== 'undefined') {
      return new URL(window.location.href)
    }
  } catch {
    return null
  }
  return null
}

export function resolvePluginPagePrefetchTarget(
  rawHref: string,
  baseHref?: string
): PluginPagePrefetchTarget | null {
  const sanitizedHref = sanitizePluginLinkUrl(rawHref)
  if (!sanitizedHref) {
    return null
  }

  const baseURL = resolvePluginPagePrefetchBaseURL(baseHref)
  if (!baseURL) {
    return null
  }

  try {
    const resolvedURL = new URL(sanitizedHref, baseURL)
    if (resolvedURL.origin !== baseURL.origin) {
      return null
    }

    const normalizedPath = normalizePluginPath(resolvedURL.pathname)
    const isAdminPluginPage =
      normalizedPath === '/admin/plugin-pages' || normalizedPath.startsWith('/admin/plugin-pages/')
    const isUserPluginPage =
      normalizedPath === '/plugin-pages' || normalizedPath.startsWith('/plugin-pages/')

    if (!isAdminPluginPage && !isUserPluginPage) {
      return null
    }

    const queryParams = readPluginSearchParams(resolvedURL.searchParams)

    return {
      href: buildPluginFullPath(normalizedPath, queryParams),
      path: normalizedPath,
      queryParams,
      scope: isAdminPluginPage ? 'admin' : 'public',
    }
  } catch {
    return null
  }
}

export async function prefetchPluginPageData(
  queryClient: QueryClient,
  target: PluginPagePrefetchTarget
) {
  const publicConfig =
    queryClient.getQueryData(['publicConfig']) ||
    (await queryClient.fetchQuery({
      queryKey: ['publicConfig'],
      queryFn: getPublicConfig,
      staleTime: 5 * 60 * 1000,
    }))
  if (!resolvePluginPlatformEnabled((publicConfig as any)?.data, true)) {
    return
  }

  const slotPrefix = target.scope === 'admin' ? 'admin' : 'user'

  await Promise.allSettled([
    queryClient.prefetchQuery(
      buildPluginBootstrapQueryOptions({
        scope: target.scope,
        path: target.path,
        queryParams: target.queryParams,
      })
    ),
    queryClient.prefetchQuery(
      buildPluginSlotQueryOptions({
        scope: target.scope,
        path: target.path,
        slot: `${slotPrefix}.plugin_page.top`,
        queryParams: target.queryParams,
      })
    ),
    queryClient.prefetchQuery(
      buildPluginSlotQueryOptions({
        scope: target.scope,
        path: target.path,
        slot: `${slotPrefix}.plugin_page.bottom`,
        queryParams: target.queryParams,
      })
    ),
  ])
}
