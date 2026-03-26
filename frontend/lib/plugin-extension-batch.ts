'use client'

import { useEffect, useMemo } from 'react'
import { useQueries, useQueryClient, type QueryClient } from '@tanstack/react-query'

import {
  getAdminPluginExtensionsBatch,
  getPluginExtensionsBatch,
  type PluginFrontendExtension,
  type PluginFrontendExtensionBatchRequestItem,
  type PluginFrontendExtensionBatchResponseItem,
} from '@/lib/api'
import { useLocale } from '@/hooks/use-locale'
import {
  normalizePluginHostContext,
  normalizePluginPath,
  normalizePluginStringMap,
  stringifyPluginHostContext,
  stringifyPluginStringMap,
} from '@/lib/plugin-frontend-routing'
import { buildPluginSlotQueryKey } from '@/lib/plugin-slot-query'

export type PluginExtensionBatchScope = 'public' | 'admin'

export type PluginExtensionBatchItemInput = {
  key: string
  slot: string
  path?: string
  queryParams?: Record<string, string>
  hostContext?: Record<string, any>
}

type UsePluginExtensionBatchOptions = {
  scope: PluginExtensionBatchScope
  path: string
  items: PluginExtensionBatchItemInput[]
  enabled?: boolean
  primeSlotQueryCache?: boolean
}

export type UsePluginExtensionBatchResult = {
  extensionsByKey: Record<string, PluginFrontendExtension[]>
  hasData: boolean
  isLoading: boolean
  isFetching: boolean
  isError: boolean
  loadedKeys: Set<string>
  failedKeys: Set<string>
}

const PLUGIN_EXTENSION_BATCH_REQUEST_CHUNK_SIZE = 64

type NormalizedBatchItem = {
  input: PluginExtensionBatchItemInput
  request: PluginFrontendExtensionBatchRequestItem
}

function normalizeBatchItems(items: PluginExtensionBatchItemInput[]): NormalizedBatchItem[] {
  return items
    .map((item) => {
      const request = {
        key: String(item.key || '').trim(),
        slot: String(item.slot || '').trim(),
        path: item.path ? normalizePluginPath(item.path) : undefined,
        query_params: normalizePluginStringMap(item.queryParams),
        host_context: normalizePluginHostContext(item.hostContext),
      }
      return {
        input: item,
        request,
      }
    })
    .filter((item) => item.request.key && item.request.slot)
}

function buildPluginExtensionBatchKeyItems(
  items: PluginFrontendExtensionBatchRequestItem[]
): string[] {
  return items.map((item) =>
    JSON.stringify({
      key: item.key,
      slot: item.slot,
      path: item.path || '',
      query_params: stringifyPluginStringMap(item.query_params),
      host_context: stringifyPluginHostContext(item.host_context),
    })
  )
}

function buildBatchResponseMap(
  items?: PluginFrontendExtensionBatchResponseItem[]
): Record<string, PluginFrontendExtension[]> {
  if (!Array.isArray(items) || items.length === 0) {
    return {}
  }
  return items.reduce<Record<string, PluginFrontendExtension[]>>((acc, item) => {
    const key = String(item.key || '').trim()
    if (!key) {
      return acc
    }
    acc[key] = Array.isArray(item.extensions) ? item.extensions : []
    return acc
  }, {})
}

function chunkItems<T>(items: T[], size: number): T[][] {
  if (items.length === 0) {
    return []
  }
  const chunks: T[][] = []
  for (let index = 0; index < items.length; index += size) {
    chunks.push(items.slice(index, index + size))
  }
  return chunks
}

export function primePluginSlotBatchResponseInQueryCache(
  queryClient: QueryClient,
  {
    scope,
    defaultPath,
    locale,
    items,
    responseItems,
  }: {
    scope: PluginExtensionBatchScope
    defaultPath: string
    locale: string
    items: PluginExtensionBatchItemInput[]
    responseItems?: PluginFrontendExtensionBatchResponseItem[]
  }
) {
  const responseByKey = new Map<string, PluginFrontendExtensionBatchResponseItem>()
  for (const item of responseItems || []) {
    const key = String(item?.key || '').trim()
    if (!key) {
      continue
    }
    responseByKey.set(key, item)
  }

  for (const item of items) {
    const key = String(item.key || '').trim()
    const slot = String(item.slot || '').trim()
    if (!key || !slot) {
      continue
    }
    const matched = responseByKey.get(key)
    const path =
      String(matched?.path || item.path || defaultPath || '/').trim() || defaultPath || '/'
    const extensions = Array.isArray(matched?.extensions) ? matched.extensions : []
    queryClient.setQueryData(
      buildPluginSlotQueryKey({
        scope,
        path,
        slot: String(matched?.slot || slot),
        locale,
        queryParams: item.queryParams,
        hostContext: item.hostContext,
      }),
      {
        data: {
          path,
          slot: String(matched?.slot || slot),
          extensions,
        },
      }
    )
  }
}

export function usePluginExtensionBatchQuery({
  scope,
  path,
  items,
  enabled = true,
  primeSlotQueryCache = false,
}: UsePluginExtensionBatchOptions): UsePluginExtensionBatchResult {
  const { locale } = useLocale()
  const queryClient = useQueryClient()
  const normalizedItems = useMemo(() => normalizeBatchItems(items), [items])
  const normalizedItemChunks = useMemo(
    () => chunkItems(normalizedItems, PLUGIN_EXTENSION_BATCH_REQUEST_CHUNK_SIZE),
    [normalizedItems]
  )
  const normalizedPath = normalizePluginPath(
    String(path || '').trim() || (scope === 'admin' ? '/admin' : '/')
  )

  const batchQueries = useQueries({
    queries: normalizedItemChunks.map((chunk, chunkIndex) => {
      const requestItems = chunk.map((item) => item.request)
      const batchKeyItems = buildPluginExtensionBatchKeyItems(requestItems)
      return {
        queryKey: [
          'plugin-extension-batch',
          scope,
          normalizedPath,
          locale,
          chunkIndex,
          ...batchKeyItems,
        ],
        queryFn: ({ signal }) =>
          scope === 'admin'
            ? getAdminPluginExtensionsBatch(normalizedPath, requestItems, signal, locale)
            : getPluginExtensionsBatch(normalizedPath, requestItems, signal, locale),
        enabled: enabled && requestItems.length > 0,
        staleTime: 30 * 1000,
        refetchOnWindowFocus: false,
        refetchOnReconnect: false,
      }
    }),
  })

  const extensionsByKey = useMemo(() => {
    const merged: Record<string, PluginFrontendExtension[]> = {}
    batchQueries.forEach((query) => {
      const responseItems = query.data?.data?.items as
        | PluginFrontendExtensionBatchResponseItem[]
        | undefined
      Object.assign(merged, buildBatchResponseMap(responseItems))
    })
    return merged
  }, [batchQueries])

  const loadedKeys = useMemo(() => {
    const keys = new Set<string>()
    normalizedItemChunks.forEach((chunk, chunkIndex) => {
      const query = batchQueries[chunkIndex]
      if (!query?.data) {
        return
      }
      chunk.forEach((item) => {
        const key = String(item.request.key || '').trim()
        if (key) {
          keys.add(key)
        }
      })
    })
    return keys
  }, [batchQueries, normalizedItemChunks])

  const failedKeys = useMemo(() => {
    const keys = new Set<string>()
    normalizedItemChunks.forEach((chunk, chunkIndex) => {
      const query = batchQueries[chunkIndex]
      if (!query?.isError) {
        return
      }
      chunk.forEach((item) => {
        const key = String(item.request.key || '').trim()
        if (key) {
          keys.add(key)
        }
      })
    })
    return keys
  }, [batchQueries, normalizedItemChunks])

  useEffect(() => {
    if (!primeSlotQueryCache) {
      return
    }
    normalizedItemChunks.forEach((chunk, chunkIndex) => {
      const query = batchQueries[chunkIndex]
      if (!query?.data) {
        return
      }
      const responseItems = query.data?.data?.items as
        | PluginFrontendExtensionBatchResponseItem[]
        | undefined
      primePluginSlotBatchResponseInQueryCache(queryClient, {
        scope,
        defaultPath: normalizedPath,
        locale,
        items: chunk.map((item) => item.input),
        responseItems,
      })
    })
  }, [
    batchQueries,
    locale,
    normalizedPath,
    normalizedItemChunks,
    primeSlotQueryCache,
    queryClient,
    scope,
  ])

  return {
    extensionsByKey,
    hasData: loadedKeys.size > 0,
    isLoading: batchQueries.some((query) => query.isLoading),
    isFetching: batchQueries.some((query) => query.isFetching),
    isError: batchQueries.some((query) => query.isError),
    loadedKeys,
    failedKeys,
  }
}

export function usePluginExtensionBatch(
  options: UsePluginExtensionBatchOptions
): Record<string, PluginFrontendExtension[]> {
  return usePluginExtensionBatchQuery(options).extensionsByKey
}
