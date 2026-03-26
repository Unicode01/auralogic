'use client'

import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useLocale } from '@/hooks/use-locale'

import {
  getPublicConfig,
  getAdminPluginFrontendBootstrap,
  getPluginFrontendBootstrap,
} from '@/lib/api'
import {
  buildPluginBootstrapContextKey,
  normalizePluginPath,
  normalizePluginStringMap,
} from '@/lib/plugin-frontend-routing'
import { resolvePluginPlatformEnabled } from '@/lib/plugin-slot-behavior'

export type PluginBootstrapQueryScope = 'public' | 'admin'

type UsePluginBootstrapQueryOptions = {
  scope: PluginBootstrapQueryScope
  path?: string
  queryParams?: Record<string, string>
  enabled?: boolean
  staleTime?: number
}

type PluginBootstrapQueryOptionsInput = {
  scope: PluginBootstrapQueryScope
  path?: string
  queryParams?: Record<string, string>
  staleTime?: number
  locale?: string
}

export function buildPluginBootstrapQueryKey(
  scope: PluginBootstrapQueryScope,
  path?: string,
  queryParams?: Record<string, string>,
  locale?: string
) {
  const normalizedPath = normalizePluginPath(
    String(path || '').trim() || (scope === 'admin' ? '/admin' : '/')
  )
  const normalizedQueryParams = normalizePluginStringMap(queryParams)

  return [
    'plugin-bootstrap',
    scope,
    String(locale || '').trim().toLowerCase() || 'default',
    buildPluginBootstrapContextKey(normalizedPath, normalizedQueryParams),
  ] as const
}

export function buildPluginBootstrapQueryOptions({
  scope,
  path,
  queryParams,
  staleTime = 30 * 1000,
  locale,
}: PluginBootstrapQueryOptionsInput) {
  const normalizedPath = normalizePluginPath(
    String(path || '').trim() || (scope === 'admin' ? '/admin' : '/')
  )
  const normalizedQueryParams = normalizePluginStringMap(queryParams)

  return {
    queryKey: buildPluginBootstrapQueryKey(scope, normalizedPath, normalizedQueryParams, locale),
    queryFn: ({ signal }: { signal: AbortSignal }) =>
      scope === 'admin'
        ? getAdminPluginFrontendBootstrap(normalizedPath, normalizedQueryParams, signal, locale)
        : getPluginFrontendBootstrap(normalizedPath, normalizedQueryParams, signal, locale),
    staleTime,
    refetchOnWindowFocus: false as const,
    refetchOnReconnect: false as const,
  }
}

export function usePluginBootstrapQuery({
  scope,
  path,
  queryParams,
  enabled = true,
  staleTime = 30 * 1000,
}: UsePluginBootstrapQueryOptions) {
  const { locale } = useLocale()
  const { data: publicConfig, isFetched: publicConfigLoaded } = useQuery({
    queryKey: ['publicConfig'],
    queryFn: getPublicConfig,
    staleTime: 5 * 60 * 1000,
  })
  const pluginPlatformEnabled = useMemo(
    () => resolvePluginPlatformEnabled(publicConfig?.data, true),
    [publicConfig]
  )
  const query = useQuery({
    ...buildPluginBootstrapQueryOptions({
      scope,
      path,
      queryParams,
      staleTime,
      locale,
    }),
    enabled: enabled && publicConfigLoaded && pluginPlatformEnabled,
  })

  if (!enabled || !publicConfigLoaded || !pluginPlatformEnabled) {
    return {
      ...query,
      data: undefined,
      error: null,
      isError: false,
    }
  }

  return query
}
