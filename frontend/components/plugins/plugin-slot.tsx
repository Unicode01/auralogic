'use client'

import { Suspense, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { usePathname, useSearchParams } from 'next/navigation'

import {
  PluginSlotContent,
  shouldShowPluginSlotRefreshing,
} from '@/components/plugins/plugin-slot-content'
import { PluginSlotSkeleton } from '@/components/plugins/plugin-loading'
import { useVisibilityActivation } from '@/hooks/use-visibility-activation'
import { getPublicConfig } from '@/lib/api'
import { readPluginSearchParams } from '@/lib/plugin-frontend-routing'
import {
  resolvePluginPlatformEnabled,
  resolvePluginGlobalSlotAnimationsEnabled,
  resolvePluginGlobalSlotLoadingEnabled,
  resolvePluginSlotAnimationDefault,
  resolvePluginSlotSkeletonVariant,
  shouldAutoDeferPluginSlot,
} from '@/lib/plugin-slot-behavior'
import { resolvePluginSlotQueryScope, usePluginSlotExtensionsQuery } from '@/lib/plugin-slot-query'
import { usePluginSlotBatchLookup } from '@/lib/plugin-slot-batch'
import { useLocale } from '@/hooks/use-locale'

type PluginSlotProps = {
  slot: string
  path?: string
  className?: string
  context?: Record<string, any>
  display?: 'stack' | 'inline'
  animate?: boolean
  deferUntilVisible?: boolean
  deferRootMargin?: string
  showDeferredSkeleton?: boolean
}

export function PluginSlot({
  ...props
}: PluginSlotProps) {
  return (
    <Suspense fallback={null}>
      <PluginSlotResolved {...props} />
    </Suspense>
  )
}

function PluginSlotResolved({
  slot,
  path,
  className,
  context,
  display = 'stack',
  animate,
  deferUntilVisible = false,
  deferRootMargin = '480px 0px',
  showDeferredSkeleton = true,
}: PluginSlotProps) {
  const pathname = usePathname()
  const searchParams = useSearchParams()
  const { locale } = useLocale()
  const { data: publicConfig, isFetched: publicConfigLoaded } = useQuery({
    queryKey: ['publicConfig'],
    queryFn: getPublicConfig,
    staleTime: 5 * 60 * 1000,
  })
  const resolvedPath = path || pathname || '/'
  const queryParams = useMemo(() => readPluginSearchParams(searchParams), [searchParams])
  const resolvedScope = useMemo(
    () => resolvePluginSlotQueryScope(resolvedPath, slot),
    [resolvedPath, slot]
  )
  const globalAnimate = useMemo(
    () => resolvePluginGlobalSlotAnimationsEnabled(publicConfig?.data, true),
    [publicConfig]
  )
  const pluginPlatformEnabled = useMemo(
    () => resolvePluginPlatformEnabled(publicConfig?.data, true),
    [publicConfig]
  )
  const globalSlotLoadingEnabled = useMemo(
    () => resolvePluginGlobalSlotLoadingEnabled(publicConfig?.data, false),
    [publicConfig]
  )
  const effectiveAnimate = useMemo(
    () => resolvePluginSlotAnimationDefault(animate, globalAnimate),
    [animate, globalAnimate]
  )
  const shouldDeferUntilVisible = deferUntilVisible || shouldAutoDeferPluginSlot(slot, display)
  const skeletonVariant = resolvePluginSlotSkeletonVariant(slot, display)
  const { targetRef, activated } = useVisibilityActivation({
    enabled: shouldDeferUntilVisible,
    rootMargin: deferRootMargin,
  })
  const batchedSlot = usePluginSlotBatchLookup({
    scope: resolvedScope,
    path: resolvedPath,
    slot,
    locale,
    queryParams,
    hostContext: context,
  })
  const shouldUseBatchedSlot = batchedSlot.managed

  const { extensions, hasData, isLoading, isFetching } = usePluginSlotExtensionsQuery({
    slot,
    path: resolvedPath,
    queryParams,
    hostContext: context,
    scope: resolvedScope,
    enabled:
      publicConfigLoaded &&
      pluginPlatformEnabled &&
      !shouldUseBatchedSlot &&
      (!shouldDeferUntilVisible || activated),
  })
  const effectiveExtensions = shouldUseBatchedSlot ? batchedSlot.extensions : extensions
  const effectiveHasData = shouldUseBatchedSlot ? batchedSlot.hasData : hasData
  const effectiveIsLoading = shouldUseBatchedSlot ? batchedSlot.isLoading : isLoading
  const effectiveIsFetching = shouldUseBatchedSlot ? batchedSlot.isFetching : isFetching
  const shouldRenderSkeleton =
    globalSlotLoadingEnabled &&
    shouldDeferUntilVisible &&
    showDeferredSkeleton &&
    !effectiveHasData &&
    (!activated ||
      (effectiveExtensions.length === 0 && (effectiveIsLoading || effectiveIsFetching)))
  const showRefreshingIndicator = shouldShowPluginSlotRefreshing(
    effectiveHasData,
    effectiveIsFetching,
    effectiveExtensions
  )
  const shouldRenderInitialSkeleton =
    globalSlotLoadingEnabled &&
    !effectiveHasData &&
    effectiveExtensions.length === 0 &&
    (effectiveIsLoading || effectiveIsFetching)

  if (publicConfigLoaded && !pluginPlatformEnabled) {
    return null
  }

  if (shouldDeferUntilVisible) {
    return (
      <div ref={targetRef}>
        {effectiveExtensions.length > 0 ? (
          <PluginSlotContent
            extensions={effectiveExtensions}
            className={className}
            display={display}
            animate={effectiveAnimate}
            refreshing={showRefreshingIndicator}
          />
        ) : shouldRenderSkeleton ? (
          <PluginSlotSkeleton className={className} variant={skeletonVariant} />
        ) : null}
      </div>
    )
  }

  if (shouldRenderInitialSkeleton) {
    return <PluginSlotSkeleton className={className} variant={skeletonVariant} />
  }

  if (effectiveExtensions.length === 0) {
    return null
  }

  return (
    <PluginSlotContent
      extensions={effectiveExtensions}
      className={className}
      display={display}
      animate={effectiveAnimate}
      refreshing={showRefreshingIndicator}
    />
  )
}
