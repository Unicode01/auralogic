'use client'

import { useQuery } from '@tanstack/react-query'
import { useEffect, useMemo } from 'react'
import { useLocale } from '@/hooks/use-locale'

import { type PluginFrontendExtension } from '@/lib/api'
import {
  normalizePluginHostContext,
  normalizePluginPath,
  normalizePluginStringMap,
  stringifyPluginHostContext,
  stringifyPluginStringMap,
} from '@/lib/plugin-frontend-routing'
import { fetchPluginSlotExtensions, type PluginSlotRequestScope } from '@/lib/plugin-slot-request'
import {
  buildPluginSlotKeepaliveKey,
  readPluginSlotKeepalive,
  writePluginSlotKeepalive,
} from '@/lib/plugin-slot-keepalive'

type PluginSlotQueryInput = {
  slot: string
  path?: string
  queryParams?: Record<string, string>
  hostContext?: Record<string, any>
  scope?: PluginSlotRequestScope
  locale?: string
}

type UsePluginSlotExtensionsQueryOptions = PluginSlotQueryInput & {
  enabled?: boolean
}

function shouldHideSlotExtension(extension: PluginFrontendExtension) {
  const title = String(extension.title || '')
    .trim()
    .toLowerCase()
  const pluginName = String(extension.plugin_name || '')
    .trim()
    .toLowerCase()
  const source =
    extension.data && typeof extension.data.source === 'string'
      ? extension.data.source.trim().toLowerCase()
      : ''

  return (
    source === 'plugin-debugger' ||
    pluginName === 'plugin debugger' ||
    title === 'open debugger' ||
    title === 'open plugin debugger'
  )
}

function normalizePluginSlot(input: PluginSlotQueryInput) {
  const normalizedSlot = String(input.slot || '').trim() || 'default'
  const inferredScope =
    normalizePluginPath(String(input.path || '').trim() || '/').startsWith('/admin') ||
    normalizedSlot.toLowerCase().startsWith('admin.')
      ? 'admin'
      : 'public'
  const scope = input.scope || inferredScope
  const path = normalizePluginPath(
    String(input.path || '').trim() || (scope === 'admin' ? '/admin' : '/')
  )
  const queryParams = normalizePluginStringMap(input.queryParams)
  const hostContext = normalizePluginHostContext(input.hostContext)

  return {
    scope,
    path,
    slot: normalizedSlot,
    locale:
      String(input.locale || '')
        .trim()
        .toLowerCase() || 'default',
    queryParams,
    hostContext,
    queryParamsKey: stringifyPluginStringMap(queryParams),
    hostContextKey: stringifyPluginHostContext(hostContext),
  }
}

export function resolvePluginSlotQueryScope(
  path?: string,
  slot?: string,
  scope?: PluginSlotRequestScope
): PluginSlotRequestScope {
  return normalizePluginSlot({ path, slot: slot || 'default', scope }).scope
}

export function buildPluginSlotQueryKey(input: PluginSlotQueryInput) {
  const normalized = normalizePluginSlot(input)

  return [
    'plugin-slot',
    normalized.scope,
    normalized.path,
    normalized.slot,
    normalized.locale,
    normalized.queryParamsKey,
    normalized.hostContextKey,
  ] as const
}

export function stringifyPluginSlotQueryKey(input: PluginSlotQueryInput): string {
  return JSON.stringify(buildPluginSlotQueryKey(input))
}

export function buildPluginSlotQueryOptions(input: PluginSlotQueryInput) {
  const normalized = normalizePluginSlot(input)

  return {
    queryKey: buildPluginSlotQueryKey(normalized),
    queryFn: ({ signal }: { signal: AbortSignal }) =>
      fetchPluginSlotExtensions(normalized.scope, {
        path: normalized.path,
        slot: normalized.slot,
        locale: normalized.locale,
        queryParams: normalized.queryParams,
        hostContext: normalized.hostContext,
        signal,
      }),
    staleTime: 30 * 1000,
    refetchOnWindowFocus: false as const,
    refetchOnReconnect: false as const,
  }
}

export function usePluginSlotExtensionsQuery({
  slot,
  path,
  queryParams,
  hostContext,
  scope,
  enabled = true,
}: UsePluginSlotExtensionsQueryOptions) {
  const { locale } = useLocale()
  const keepaliveScope = resolvePluginSlotQueryScope(path, slot, scope)
  const normalizedPath = normalizePluginPath(
    String(path || '').trim() || (keepaliveScope === 'admin' ? '/admin' : '/')
  )
  const keepaliveKey = useMemo(
    () => buildPluginSlotKeepaliveKey(keepaliveScope, normalizedPath, slot, locale),
    [keepaliveScope, locale, normalizedPath, slot]
  )
  const retainedData = useMemo(() => readPluginSlotKeepalive(keepaliveKey), [keepaliveKey])

  const query = useQuery({
    ...buildPluginSlotQueryOptions({
      slot,
      path,
      queryParams,
      hostContext,
      scope,
      locale,
    }),
    enabled,
    placeholderData: (previousData) => previousData ?? retainedData,
  })

  useEffect(() => {
    if (!query.data) {
      return
    }
    writePluginSlotKeepalive(keepaliveKey, query.data)
  }, [keepaliveKey, query.data])

  const extensions = Array.isArray(query.data?.data?.extensions)
    ? (query.data.data.extensions as PluginFrontendExtension[]).filter(
        (extension) => !shouldHideSlotExtension(extension)
      )
    : []

  return {
    ...query,
    extensions,
    hasData: query.data !== undefined,
  }
}
