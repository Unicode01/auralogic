'use client'

import Link from 'next/link'
import { useRouter } from 'next/navigation'
import { useQueryClient } from '@tanstack/react-query'
import { forwardRef, type ComponentProps, useMemo, useRef } from 'react'

import { prefetchPluginPageData, resolvePluginPagePrefetchTarget } from '@/lib/plugin-page-prefetch'

type PluginPageLinkProps = Omit<ComponentProps<typeof Link>, 'href'> & {
  href: string
  prefetchPluginData?: boolean
}

export const PluginPageLink = forwardRef<HTMLAnchorElement, PluginPageLinkProps>(
  (
    {
      href,
      onMouseEnter,
      onFocus,
      onTouchStart,
      prefetchPluginData = true,
      ...props
    },
    ref
  ) => {
    const router = useRouter()
    const queryClient = useQueryClient()
    const prefetchedRef = useRef(false)
    const target = useMemo(() => resolvePluginPagePrefetchTarget(href), [href])

    const triggerPrefetch = () => {
      if (!prefetchPluginData || prefetchedRef.current || !target) {
        return
      }
      prefetchedRef.current = true
      router.prefetch(target.href)
      void prefetchPluginPageData(queryClient, target).catch(() => {
        prefetchedRef.current = false
      })
    }

    return (
      <Link
        {...props}
        ref={ref}
        href={href}
        onMouseEnter={(event) => {
          onMouseEnter?.(event)
          triggerPrefetch()
        }}
        onFocus={(event) => {
          onFocus?.(event)
          triggerPrefetch()
        }}
        onTouchStart={(event) => {
          onTouchStart?.(event)
          triggerPrefetch()
        }}
      />
    )
  }
)

PluginPageLink.displayName = 'PluginPageLink'
