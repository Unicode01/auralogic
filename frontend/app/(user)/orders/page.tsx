'use client'

import { useState, useEffect, useCallback } from 'react'
import { useOrders } from '@/hooks/use-orders'
import { OrderList } from '@/components/orders/order-list'
import { OrderFilter } from '@/components/orders/order-filter'
import { Button } from '@/components/ui/button'
import { RefreshCw } from 'lucide-react'
import { useLocale } from '@/hooks/use-locale'
import { usePageTitle } from '@/hooks/use-page-title'
import { getTranslations } from '@/lib/i18n'
import { useIsMobile } from '@/hooks/use-mobile'
import { Order } from '@/types/order'

export default function OrdersPage() {
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.orders)
  const { isMobile, mounted } = useIsMobile()
  const [page, setPage] = useState(1)
  const [status, setStatus] = useState<string | undefined>()
  const [search, setSearch] = useState('')

  // 移动端累积订单列表
  const [allOrders, setAllOrders] = useState<Order[]>([])
  const [isLoadingMore, setIsLoadingMore] = useState(false)

  const { data, isLoading, isFetching, refetch } = useOrders({
    page,
    limit: 18,
    status,
    search,
  })

  const orders = data?.data?.items || []
  const pagination = data?.data?.pagination
  const hasMore = pagination ? page < pagination.total_pages : false

  // 移动端：累积订单数据
  useEffect(() => {
    if (isMobile && orders.length > 0) {
      if (page === 1) {
        setAllOrders(orders)
      } else {
        setAllOrders(prev => {
          // 避免重复添加
          const existingIds = new Set(prev.map(o => o.id))
          const newOrders = orders.filter((o: Order) => !existingIds.has(o.id))
          return [...prev, ...newOrders]
        })
      }
      setIsLoadingMore(false)
    }
  }, [orders, page, isMobile])

  // 筛选变化时重置
  const handleStatusChange = (newStatus: string | undefined) => {
    setStatus(newStatus)
    setPage(1)
    setAllOrders([])
  }

  const handleSearchChange = (newSearch: string) => {
    setSearch(newSearch)
    setPage(1)
    setAllOrders([])
  }

  const handleRefresh = () => {
    setPage(1)
    setAllOrders([])
    refetch()
  }

  // 加载更多
  const loadMore = useCallback(() => {
    if (!isLoadingMore && hasMore && !isFetching) {
      setIsLoadingMore(true)
      setPage(prev => prev + 1)
    }
  }, [isLoadingMore, hasMore, isFetching])

  // 显示的订单列表
  const displayOrders = isMobile ? allOrders : orders

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-bold">{t.order.myOrders}</h1>
        <Button variant="outline" size="sm" onClick={handleRefresh}>
          <RefreshCw className="mr-2 h-4 w-4" />
          {locale === 'zh' ? '刷新' : 'Refresh'}
        </Button>
      </div>

      <OrderFilter
        status={status}
        search={search}
        onStatusChange={handleStatusChange}
        onSearchChange={handleSearchChange}
      />

      <OrderList
        orders={displayOrders}
        isLoading={isLoading && page === 1}
        pagination={isMobile ? undefined : {
          page,
          total_pages: pagination?.total_pages || 1,
          onPageChange: setPage,
        }}
        // 移动端无限滚动相关 props
        isMobile={isMobile && mounted}
        isLoadingMore={isLoadingMore || isFetching}
        hasMore={hasMore}
        onLoadMore={loadMore}
      />
    </div>
  )
}
