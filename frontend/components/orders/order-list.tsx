'use client'

import { useMemo } from 'react'
import type { Order } from '@/types/order'
import { OrderCard, buildOrderCardBatchItems } from './order-card'
import { Button } from '@/components/ui/button'
import { ChevronLeft, ChevronRight, Loader2, Package } from 'lucide-react'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { useInfiniteScroll } from '@/hooks/use-infinite-scroll'
import { PluginSlot } from '@/components/plugins/plugin-slot'
import { PluginSlotBatchBoundary, type PluginSlotBatchBoundaryItem } from '@/lib/plugin-slot-batch'

interface OrderListProps {
  orders: Order[]
  isLoading?: boolean
  highlightedOrderNo?: string
  onOpenOrder?: (orderNo: string) => void
  pagination?: {
    page: number
    total_pages: number
    onPageChange: (page: number) => void
  }
  // 移动端无限滚动相关
  isMobile?: boolean
  isPhone?: boolean
  isLoadingMore?: boolean
  hasMore?: boolean
  onLoadMore?: () => void
  emptyTitle?: string
  emptyDescription?: string
  emptyActionLabel?: string
  onEmptyAction?: () => void
  pluginSlotNamespace?: string
  pluginSlotContext?: Record<string, any>
  pluginSlotPath?: string
  pluginSlotQueryParams?: Record<string, string>
}

export function OrderList({
  orders,
  isLoading,
  highlightedOrderNo,
  onOpenOrder,
  pagination,
  isMobile,
  isPhone = false,
  isLoadingMore,
  hasMore,
  onLoadMore,
  emptyTitle,
  emptyDescription,
  emptyActionLabel,
  onEmptyAction,
  pluginSlotNamespace,
  pluginSlotContext,
  pluginSlotPath,
  pluginSlotQueryParams,
}: OrderListProps) {
  const { locale } = useLocale()
  const t = getTranslations(locale)
  const resolvedEmptyDescription =
    emptyDescription !== undefined ? emptyDescription : t.order.noOrdersDesc
  const orderGridClassName = isPhone
    ? 'grid grid-cols-1 gap-4'
    : isMobile
      ? 'grid grid-cols-2 gap-4'
      : 'grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3'
  const orderListPluginContext = useMemo(
    () => ({
      ...(pluginSlotContext || {}),
      list: {
        item_count: orders.length,
        highlighted_order_no: highlightedOrderNo || undefined,
      },
      pagination: pagination
        ? {
            page: pagination.page,
            total_pages: pagination.total_pages,
          }
        : undefined,
      infinite_scroll: {
        enabled: Boolean(isMobile && onLoadMore),
        has_more: Boolean(hasMore),
        loading_more: Boolean(isLoadingMore),
      },
      state: {
        loading: Boolean(isLoading),
        empty: orders.length === 0,
        has_orders: orders.length > 0,
        mobile: Boolean(isMobile),
        pagination_enabled: Boolean(!isMobile && pagination),
      },
    }),
    [
      hasMore,
      highlightedOrderNo,
      isLoading,
      isLoadingMore,
      isMobile,
      onLoadMore,
      orders.length,
      pagination,
      pluginSlotContext,
    ]
  )
  const slotScope =
    String(pluginSlotPath || '')
      .trim()
      .startsWith('/admin') ||
    String(pluginSlotNamespace || '')
      .trim()
      .toLowerCase()
      .startsWith('admin.')
      ? 'admin'
      : 'public'
  const resolvedPluginSlotPath = pluginSlotPath || (slotScope === 'admin' ? '/admin' : '/')
  const orderListBatchItems = useMemo<PluginSlotBatchBoundaryItem[]>(() => {
    if (!pluginSlotNamespace) {
      return []
    }

    const items: PluginSlotBatchBoundaryItem[] = []
    if (orders.length === 0) {
      items.push({
        slot: `${pluginSlotNamespace}.list.empty`,
        path: pluginSlotPath,
        hostContext: { ...orderListPluginContext, section: 'list_empty' },
      })
      return items
    }

    items.push({
      slot: `${pluginSlotNamespace}.list.grid.after`,
      path: pluginSlotPath,
      hostContext: { ...orderListPluginContext, section: 'list_grid' },
    })

    if (isMobile) {
      items.push({
        slot: `${pluginSlotNamespace}.list.load_more.after`,
        path: pluginSlotPath,
        hostContext: { ...orderListPluginContext, section: 'list_load_more' },
      })
    }

    if (!isMobile && pagination) {
      items.push(
        {
          slot: `${pluginSlotNamespace}.list.pagination.before`,
          path: pluginSlotPath,
          hostContext: { ...orderListPluginContext, section: 'list_pagination' },
        },
        {
          slot: `${pluginSlotNamespace}.list.pagination.after`,
          path: pluginSlotPath,
          hostContext: { ...orderListPluginContext, section: 'list_pagination' },
        }
      )
    }

    orders.forEach((order, index) => {
      items.push(
        ...buildOrderCardBatchItems({
          order,
          highlighted: highlightedOrderNo === (order.orderNo || order.order_no || ''),
          pluginSlotNamespace: `${pluginSlotNamespace}.list`,
          pluginSlotContext: orderListPluginContext,
          pluginSlotPath,
          rowIndex: index + 1,
        })
      )
    })

    return items
  }, [
    highlightedOrderNo,
    isMobile,
    orderListPluginContext,
    orders,
    pagination,
    pluginSlotNamespace,
    pluginSlotPath,
  ])

  // 无限滚动 hook
  const { loadMoreRef } = useInfiniteScroll({
    enabled: isMobile && !!onLoadMore,
    isLoading: isLoading || isLoadingMore,
    hasMore: hasMore ?? false,
    onLoadMore: onLoadMore || (() => {}),
  })

  if (isLoading) {
    return (
      <div className={orderGridClassName}>
        {[...Array(6)].map((_, i) => (
          <OrderCardSkeleton key={i} />
        ))}
      </div>
    )
  }

  const content =
    orders.length === 0 ? (
      <div className="flex flex-col items-center justify-center rounded-2xl border border-dashed bg-muted/10 px-6 py-12 text-center">
        <div className="rounded-full bg-muted p-3 text-muted-foreground">
          <Package className="h-6 w-6" />
        </div>
        <p className="mt-4 text-base font-medium">{emptyTitle || t.order.noOrders}</p>
        {resolvedEmptyDescription ? (
          <p className="mt-2 max-w-md text-sm text-muted-foreground">
            {resolvedEmptyDescription}
          </p>
        ) : null}
        {emptyActionLabel && onEmptyAction ? (
          <Button variant="outline" className="mt-4" onClick={onEmptyAction}>
            {emptyActionLabel}
          </Button>
        ) : null}
        {pluginSlotNamespace ? (
          <PluginSlot
            slot={`${pluginSlotNamespace}.list.empty`}
            path={pluginSlotPath}
            context={{ ...orderListPluginContext, section: 'list_empty' }}
          />
        ) : null}
      </div>
    ) : (
      <div className="space-y-4">
        <div className={orderGridClassName}>
          {orders.map((order, index) => (
            <OrderCard
              key={order.id}
              order={order}
              highlighted={highlightedOrderNo === (order.orderNo || order.order_no || '')}
              onOpenDetail={onOpenOrder}
              pluginSlotNamespace={
                pluginSlotNamespace ? `${pluginSlotNamespace}.list` : undefined
              }
              pluginSlotContext={orderListPluginContext}
              pluginSlotPath={pluginSlotPath}
              rowIndex={index + 1}
            />
          ))}
        </div>
        {pluginSlotNamespace ? (
          <PluginSlot
            slot={`${pluginSlotNamespace}.list.grid.after`}
            path={pluginSlotPath}
            context={{ ...orderListPluginContext, section: 'list_grid' }}
          />
        ) : null}

        {/* 移动端：无限滚动加载指示器 */}
        {isMobile && (
          <div ref={loadMoreRef} className="flex justify-center py-4">
            {isLoadingMore && hasMore ? (
              <div className="flex items-center gap-2 text-muted-foreground">
                <Loader2 className="h-4 w-4 animate-spin" />
                <span className="text-sm">{t.common.loading}</span>
              </div>
            ) : hasMore ? (
              <span className="text-sm text-muted-foreground">{t.common.scrollToLoadMore}</span>
            ) : orders.length > 0 ? (
              <span className="text-sm text-muted-foreground">{t.order.noMoreOrders}</span>
            ) : null}
          </div>
        )}
        {isMobile && pluginSlotNamespace ? (
          <PluginSlot
            slot={`${pluginSlotNamespace}.list.load_more.after`}
            path={pluginSlotPath}
            context={{ ...orderListPluginContext, section: 'list_load_more' }}
          />
        ) : null}

        {/* PC端：分页控件 */}
        {!isMobile && pagination && (
          <div className="space-y-3 pt-4">
            {pluginSlotNamespace ? (
              <PluginSlot
                slot={`${pluginSlotNamespace}.list.pagination.before`}
                path={pluginSlotPath}
                context={{ ...orderListPluginContext, section: 'list_pagination' }}
              />
            ) : null}
            <div className="flex items-center justify-between">
              <p className="text-sm text-muted-foreground">
                {t.common.pageInfo
                  .replace('{page}', String(pagination.page))
                  .replace('{totalPages}', String(pagination.total_pages))}
              </p>
              <div className="flex items-center gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => pagination.onPageChange(pagination.page - 1)}
                  disabled={pagination.page <= 1}
                >
                  <ChevronLeft className="h-4 w-4" />
                  {t.common.prevPage}
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
                  className="h-8 w-12 rounded-md border bg-background text-center text-sm [appearance:textfield] focus:outline-none focus:ring-2 focus:ring-ring [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none"
                />
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => pagination.onPageChange(pagination.page + 1)}
                  disabled={pagination.page >= pagination.total_pages}
                >
                  {t.common.nextPage}
                  <ChevronRight className="h-4 w-4" />
                </Button>
              </div>
            </div>
            {pluginSlotNamespace ? (
              <PluginSlot
                slot={`${pluginSlotNamespace}.list.pagination.after`}
                path={pluginSlotPath}
                context={{ ...orderListPluginContext, section: 'list_pagination' }}
              />
            ) : null}
          </div>
        )}
      </div>
    )

  if (!pluginSlotNamespace || orderListBatchItems.length === 0) {
    return content
  }

  return (
    <PluginSlotBatchBoundary
      scope={slotScope}
      path={resolvedPluginSlotPath}
      items={orderListBatchItems}
      queryParams={pluginSlotQueryParams}
    >
      {content}
    </PluginSlotBatchBoundary>
  )
}

function OrderCardSkeleton() {
  return (
    <div className="h-[280px] animate-pulse rounded-lg border p-4">
      <div className="mb-2 h-4 w-3/4 rounded bg-muted"></div>
      <div className="mb-4 h-3 w-1/2 rounded bg-muted"></div>
      <div className="space-y-2">
        <div className="flex gap-2">
          <div className="h-12 w-12 rounded bg-muted"></div>
          <div className="flex-1">
            <div className="mb-2 h-3 rounded bg-muted"></div>
            <div className="h-3 w-2/3 rounded bg-muted"></div>
          </div>
        </div>
      </div>
    </div>
  )
}
