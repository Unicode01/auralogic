'use client'

import type { Order } from '@/types/order'
import { OrderCard } from './order-card'
import { Button } from '@/components/ui/button'
import { ChevronLeft, ChevronRight, Loader2 } from 'lucide-react'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { useInfiniteScroll } from '@/hooks/use-infinite-scroll'

interface OrderListProps {
  orders: Order[]
  isLoading?: boolean
  pagination?: {
    page: number
    total_pages: number
    onPageChange: (page: number) => void
  }
  // 移动端无限滚动相关
  isMobile?: boolean
  isLoadingMore?: boolean
  hasMore?: boolean
  onLoadMore?: () => void
}

export function OrderList({
  orders,
  isLoading,
  pagination,
  isMobile,
  isLoadingMore,
  hasMore,
  onLoadMore,
}: OrderListProps) {
  const { locale } = useLocale()
  const t = getTranslations(locale)

  // 无限滚动 hook
  const { loadMoreRef } = useInfiniteScroll({
    enabled: isMobile && !!onLoadMore,
    isLoading: isLoading || isLoadingMore,
    hasMore: hasMore ?? false,
    onLoadMore: onLoadMore || (() => {}),
  })

  if (isLoading) {
    return (
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {[...Array(6)].map((_, i) => (
          <OrderCardSkeleton key={i} />
        ))}
      </div>
    )
  }

  if (orders.length === 0) {
    return (
      <div className="text-center py-12">
        <p className="text-muted-foreground">{t.order.noOrders}</p>
      </div>
    )
  }

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {orders.map((order) => (
          <OrderCard key={order.id} order={order} />
        ))}
      </div>

      {/* 移动端：无限滚动加载指示器 */}
      {isMobile && (
        <div ref={loadMoreRef} className="flex justify-center py-4">
          {isLoadingMore && hasMore ? (
            <div className="flex items-center gap-2 text-muted-foreground">
              <Loader2 className="h-4 w-4 animate-spin" />
              <span className="text-sm">{locale === 'zh' ? '加载中...' : 'Loading...'}</span>
            </div>
          ) : hasMore ? (
            <span className="text-sm text-muted-foreground">
              {locale === 'zh' ? '向下滚动加载更多' : 'Scroll down to load more'}
            </span>
          ) : orders.length > 0 ? (
            <span className="text-sm text-muted-foreground">
              {locale === 'zh' ? '没有更多订单了' : 'No more orders'}
            </span>
          ) : null}
        </div>
      )}

      {/* PC端：分页控件 */}
      {!isMobile && pagination && (
        <div className="flex items-center justify-between pt-4">
          <p className="text-sm text-muted-foreground">
            {locale === 'zh'
              ? `第 ${pagination.page} 页，共 ${pagination.total_pages} 页`
              : `Page ${pagination.page} of ${pagination.total_pages}`}
          </p>
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => pagination.onPageChange(pagination.page - 1)}
              disabled={pagination.page <= 1}
            >
              <ChevronLeft className="h-4 w-4" />
              {locale === 'zh' ? '上一页' : 'Previous'}
            </Button>
            <input
              type="number"
              min={1}
              max={pagination.total_pages}
              defaultValue={pagination.page}
              key={pagination.page}
              onBlur={(e) => {
                const p = parseInt(e.target.value)
                if (p >= 1 && p <= pagination.total_pages && p !== pagination.page) {
                  pagination.onPageChange(p)
                }
              }}
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  const p = parseInt((e.target as HTMLInputElement).value)
                  if (p >= 1 && p <= pagination.total_pages && p !== pagination.page) {
                    pagination.onPageChange(p)
                  }
                  ;(e.target as HTMLInputElement).blur()
                }
              }}
              className="w-12 h-8 text-center text-sm border rounded-md bg-background focus:outline-none focus:ring-2 focus:ring-ring [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
            />
            <Button
              variant="outline"
              size="sm"
              onClick={() => pagination.onPageChange(pagination.page + 1)}
              disabled={pagination.page >= pagination.total_pages}
            >
              {locale === 'zh' ? '下一页' : 'Next'}
              <ChevronRight className="h-4 w-4" />
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}

function OrderCardSkeleton() {
  return (
    <div className="border rounded-lg p-4 animate-pulse h-[280px]">
      <div className="h-4 bg-muted rounded w-3/4 mb-2"></div>
      <div className="h-3 bg-muted rounded w-1/2 mb-4"></div>
      <div className="space-y-2">
        <div className="flex gap-2">
          <div className="w-12 h-12 bg-muted rounded"></div>
          <div className="flex-1">
            <div className="h-3 bg-muted rounded mb-2"></div>
            <div className="h-3 bg-muted rounded w-2/3"></div>
          </div>
        </div>
      </div>
    </div>
  )
}
