'use client'

import { createContext, useContext, useMemo, type ReactNode } from 'react'
import { useQuery } from '@tanstack/react-query'

import { getPublicConfig, type PluginFrontendExtension } from '@/lib/api'
import {
  usePluginExtensionBatchQuery,
  type PluginExtensionBatchItemInput,
  type PluginExtensionBatchScope,
} from '@/lib/plugin-extension-batch'
import { normalizePluginPath } from '@/lib/plugin-frontend-routing'
import { stringifyPluginSlotQueryKey } from '@/lib/plugin-slot-query'
import { useLocale } from '@/hooks/use-locale'
import { resolvePluginPlatformEnabled } from '@/lib/plugin-slot-behavior'

export type PluginSlotBatchBoundaryItem = {
  slot: string
  path?: string
  queryParams?: Record<string, string>
  hostContext?: Record<string, any>
}

type PluginSlotBatchContextValue = {
  managedKeys: Set<string>
  extensionsByKey: Map<string, PluginFrontendExtension[]>
  loadedKeys: Set<string>
  failedKeys: Set<string>
  isLoading: boolean
  isFetching: boolean
}

const PluginSlotBatchContext = createContext<PluginSlotBatchContextValue | null>(null)

function buildPluginSlotBatchItemKey(
  scope: PluginExtensionBatchScope,
  locale: string,
  item: PluginSlotBatchBoundaryItem,
  fallbackPath: string,
  inheritedQueryParams?: Record<string, string>
) {
  return stringifyPluginSlotQueryKey({
    scope,
    path: item.path || fallbackPath,
    slot: item.slot,
    locale,
    queryParams: item.queryParams || inheritedQueryParams,
    hostContext: item.hostContext,
  })
}

type PluginSlotBatchBoundaryProps = {
  scope: PluginExtensionBatchScope
  path: string
  items: PluginSlotBatchBoundaryItem[]
  queryParams?: Record<string, string>
  enabled?: boolean
  children: ReactNode
}

export function PluginSlotBatchBoundary({
  scope,
  path,
  items,
  queryParams,
  enabled = true,
  children,
}: PluginSlotBatchBoundaryProps) {
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
  const normalizedPath = normalizePluginPath(
    String(path || '').trim() || (scope === 'admin' ? '/admin' : '/')
  )
  const batchItems = useMemo<PluginExtensionBatchItemInput[]>(
    () =>
      items.map((item) => ({
        key: buildPluginSlotBatchItemKey(scope, locale, item, normalizedPath, queryParams),
        slot: item.slot,
        path: item.path || normalizedPath,
        queryParams: item.queryParams || queryParams,
        hostContext: item.hostContext,
      })),
    [items, locale, normalizedPath, queryParams, scope]
  )
  const batchQuery = usePluginExtensionBatchQuery({
    scope,
    path: normalizedPath,
    items: batchItems,
    enabled: enabled && publicConfigLoaded && pluginPlatformEnabled && batchItems.length > 0,
    primeSlotQueryCache: true,
  })
  const value = useMemo<PluginSlotBatchContextValue>(() => {
    const managedKeys = new Set<string>()
    const extensionsByKey = new Map<string, PluginFrontendExtension[]>()

    for (const item of batchItems) {
      const queryKey = String(item.key || '').trim()
      if (!queryKey) {
        continue
      }
      managedKeys.add(queryKey)
      if (batchQuery.loadedKeys.has(queryKey)) {
        extensionsByKey.set(queryKey, batchQuery.extensionsByKey[queryKey] || [])
      }
    }

    return {
      managedKeys,
      loadedKeys: batchQuery.loadedKeys,
      failedKeys: batchQuery.failedKeys,
      extensionsByKey,
      isLoading: batchQuery.isLoading,
      isFetching: batchQuery.isFetching,
    }
  }, [
    batchItems,
    batchQuery.extensionsByKey,
    batchQuery.failedKeys,
    batchQuery.isFetching,
    batchQuery.isLoading,
    batchQuery.loadedKeys,
  ])

  if (!enabled || !publicConfigLoaded || !pluginPlatformEnabled || batchItems.length === 0) {
    return <>{children}</>
  }

  return <PluginSlotBatchContext.Provider value={value}>{children}</PluginSlotBatchContext.Provider>
}

export function usePluginSlotBatchLookup({
  scope,
  path,
  slot,
  locale,
  queryParams,
  hostContext,
}: {
  scope: PluginExtensionBatchScope
  path: string
  slot: string
  locale: string
  queryParams?: Record<string, string>
  hostContext?: Record<string, any>
}) {
  const context = useContext(PluginSlotBatchContext)
  const queryKey = useMemo(
    () =>
      stringifyPluginSlotQueryKey({
        scope,
        path,
        slot,
        locale,
        queryParams,
        hostContext,
      }),
    [scope, path, slot, locale, queryParams, hostContext]
  )

  return useMemo(() => {
    if (!context || !context.managedKeys.has(queryKey)) {
      return {
        managed: false,
        hasData: false,
        isLoading: false,
        isFetching: false,
        extensions: [] as PluginFrontendExtension[],
      }
    }

    const hasData = context.loadedKeys.has(queryKey)
    if (!hasData && context.failedKeys.has(queryKey)) {
      return {
        managed: false,
        hasData: false,
        isLoading: false,
        isFetching: false,
        extensions: [] as PluginFrontendExtension[],
      }
    }

    return {
      managed: true,
      hasData,
      isLoading: !hasData && context.isLoading,
      isFetching: context.isFetching,
      extensions: hasData ? context.extensionsByKey.get(queryKey) || [] : [],
    }
  }, [context, queryKey])
}
