'use client'

import { useEffect, useRef, useCallback } from 'react'

interface UseInfiniteScrollOptions {
  /** 是否启用无限滚动 */
  enabled?: boolean
  /** 触发加载的阈值（距离底部的像素数） */
  threshold?: number
  /** 是否正在加载 */
  isLoading?: boolean
  /** 是否还有更多数据 */
  hasMore?: boolean
  /** 加载更多的回调 */
  onLoadMore: () => void
}

export function useInfiniteScroll({
  enabled = true,
  threshold = 200,
  isLoading = false,
  hasMore = true,
  onLoadMore,
}: UseInfiniteScrollOptions) {
  const observerRef = useRef<IntersectionObserver | null>(null)
  const loadMoreRef = useRef<HTMLDivElement | null>(null)

  const handleObserver = useCallback(
    (entries: IntersectionObserverEntry[]) => {
      const [target] = entries
      if (target.isIntersecting && !isLoading && hasMore && enabled) {
        onLoadMore()
      }
    },
    [isLoading, hasMore, enabled, onLoadMore]
  )

  useEffect(() => {
    if (!enabled) return

    const element = loadMoreRef.current
    if (!element) return

    observerRef.current = new IntersectionObserver(handleObserver, {
      root: null,
      rootMargin: `${threshold}px`,
      threshold: 0,
    })

    observerRef.current.observe(element)

    return () => {
      if (observerRef.current) {
        observerRef.current.disconnect()
      }
    }
  }, [enabled, threshold, handleObserver])

  return { loadMoreRef }
}
