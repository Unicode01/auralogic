'use client'

import { usePathname, useRouter, useSearchParams } from 'next/navigation'
import { Suspense, useState, useEffect, useCallback, useRef, useMemo } from 'react'
import { getOrders } from '@/lib/api'
import { useOrders } from '@/hooks/use-orders'
import { OrderList } from '@/components/orders/order-list'
import { OrderFilter } from '@/components/orders/order-filter'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { RefreshCw } from 'lucide-react'
import { useLocale } from '@/hooks/use-locale'
import { usePageTitle } from '@/hooks/use-page-title'
import { getTranslations } from '@/lib/i18n'
import { useIsMobile } from '@/hooks/use-mobile'
import { Order } from '@/types/order'
import { PluginSlot } from '@/components/plugins/plugin-slot'
import { useDebounce } from '@/hooks/use-debounce'
import {
  buildUpdatedQueryString,
  normalizePositivePageQuery,
  normalizeQueryString,
} from '@/lib/query-state'
import {
  clearListBrowseState,
  getListFocusParamKey,
  parseFocusedListItemQuery,
  readListBrowseState,
  setListBrowseState,
  stripListFocusFromPath,
} from '@/lib/list-browse-state'
import { readPluginSearchParams } from '@/lib/plugin-frontend-routing'

const orderListLimit = 18
const EMPTY_ORDERS: Order[] = []

function OrdersPageContent() {
  const router = useRouter()
  const pathname = usePathname() || '/orders'
  const searchParams = useSearchParams()
  const pluginQueryParams = useMemo(() => readPluginSearchParams(searchParams), [searchParams])
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.orders)
  const { isMobile, mounted } = useIsMobile()
  const searchParamsKey = searchParams.toString()
  const initialSearch = normalizeQueryString(searchParams.get('search'))
  const initialStatus = normalizeQueryString(searchParams.get('status')) || undefined
  const initialPage = normalizePositivePageQuery(searchParams.get('page'))
  const initialListPath = stripListFocusFromPath(
    'orders',
    searchParamsKey ? `${pathname}?${searchParamsKey}` : pathname
  )
  const initialBrowseState = readListBrowseState('orders')
  const initialFocusedOrderNo =
    parseFocusedListItemQuery(searchParams.get(getListFocusParamKey('orders'))) ||
    (initialBrowseState?.listPath === initialListPath
      ? initialBrowseState.focusedItemKey
      : undefined)
  const [page, setPage] = useState(initialPage)
  const [status, setStatus] = useState<string | undefined>(initialStatus)
  const [search, setSearch] = useState(initialSearch)
  const [searchInput, setSearchInput] = useState(initialSearch)
  const [highlightedOrderNo, setHighlightedOrderNo] = useState(initialFocusedOrderNo)
  const stateRef = useRef({
    page: initialPage,
    status: initialStatus,
    search: initialSearch,
    searchInput: initialSearch,
  })
  const hasRestoredBrowseStateRef = useRef(false)
  const debouncedSearchInput = useDebounce(searchInput, 300)

  // 移动端累积订单列表
  const [allOrders, setAllOrders] = useState<Order[]>([])
  const [isLoadingMore, setIsLoadingMore] = useState(false)
  const [isRestoringPages, setIsRestoringPages] = useState(false)
  const currentListPath = initialListPath

  const replaceQueryState = useCallback(
    (nextState: { page?: number; status?: string; search?: string }) => {
      const queryString = buildUpdatedQueryString(
        searchParams,
        {
          page: nextState.page,
          status: nextState.status || undefined,
          search: nextState.search || undefined,
          [getListFocusParamKey('orders')]: undefined,
        },
        { page: 1 }
      )
      router.replace(queryString ? `${pathname}?${queryString}` : pathname, { scroll: false })
    },
    [pathname, router, searchParams]
  )

  const { data, isLoading, isFetching, isError, refetch } = useOrders({
    page,
    limit: orderListLimit,
    status,
    search,
  })

  const orders = data?.data?.items ?? EMPTY_ORDERS
  const pagination = data?.data?.pagination
  const hasMore = pagination ? page < pagination.total_pages : false

  useEffect(() => {
    stateRef.current = {
      page,
      status,
      search,
      searchInput,
    }
  }, [page, search, searchInput, status])

  useEffect(() => {
    const nextSearch = normalizeQueryString(searchParams.get('search'))
    const nextStatus = normalizeQueryString(searchParams.get('status')) || undefined
    const nextPage = normalizePositivePageQuery(searchParams.get('page'))
    const browseState = readListBrowseState('orders')
    const nextFocusedOrderNo =
      parseFocusedListItemQuery(searchParams.get(getListFocusParamKey('orders'))) ||
      (browseState?.listPath === currentListPath ? browseState.focusedItemKey : undefined)
    const currentState = stateRef.current
    const urlStateChanged =
      nextSearch !== currentState.search ||
      nextSearch !== currentState.searchInput ||
      nextStatus !== currentState.status ||
      nextPage !== currentState.page

    if (!urlStateChanged) {
      return
    }

    setSearch(nextSearch)
    setSearchInput(nextSearch)
    setStatus(nextStatus)
    setPage(nextPage)
    setHighlightedOrderNo(nextFocusedOrderNo)
    setAllOrders([])
    setIsLoadingMore(false)
    setIsRestoringPages(false)
    hasRestoredBrowseStateRef.current = false
  }, [currentListPath, searchParams, searchParamsKey])

  useEffect(() => {
    if (!searchParams.get(getListFocusParamKey('orders'))) {
      return
    }
    router.replace(currentListPath, { scroll: false })
  }, [currentListPath, router, searchParams, searchParamsKey])

  useEffect(() => {
    const nextSearch = normalizeQueryString(debouncedSearchInput)
    if (nextSearch === search) {
      return
    }

    setSearch(nextSearch)
    setPage(1)
    setAllOrders([])
    setIsLoadingMore(false)
    replaceQueryState({
      page: 1,
      status,
      search: nextSearch,
    })
  }, [debouncedSearchInput, replaceQueryState, search, status])

  // 移动端：累积订单数据
  useEffect(() => {
    if (isMobile && !isRestoringPages && orders.length > 0) {
      if (page === 1) {
        setAllOrders(orders)
      } else {
        setAllOrders((prev) => {
          // 避免重复添加
          const existingIds = new Set(prev.map((o) => o.id))
          const newOrders = orders.filter((o: Order) => !existingIds.has(o.id))
          return [...prev, ...newOrders]
        })
      }
      setIsLoadingMore(false)
    }
  }, [isMobile, isRestoringPages, orders, page])

  useEffect(() => {
    if (!isMobile || !mounted || page <= 1 || allOrders.length > 0) {
      return
    }

    let cancelled = false
    setIsRestoringPages(true)
    setIsLoadingMore(true)

    const restoreOrders = async () => {
      try {
        const responses = await Promise.all(
          Array.from({ length: page }, (_item, index) =>
            getOrders({
              page: index + 1,
              limit: orderListLimit,
              status,
              search: search || undefined,
            })
          )
        )
        if (cancelled) {
          return
        }
        const mergedOrders = responses.flatMap((response) => response?.data?.items || [])
        const existingIds = new Set<number>()
        const dedupedOrders = mergedOrders.filter((order: Order) => {
          if (existingIds.has(order.id)) {
            return false
          }
          existingIds.add(order.id)
          return true
        })
        setAllOrders(dedupedOrders)
      } finally {
        if (!cancelled) {
          setIsRestoringPages(false)
          setIsLoadingMore(false)
        }
      }
    }

    void restoreOrders()

    return () => {
      cancelled = true
    }
  }, [allOrders.length, isMobile, mounted, page, search, status])

  // 筛选变化时重置
  const handleStatusChange = (newStatus: string | undefined) => {
    const nextStatus = newStatus || undefined
    const nextSearch = normalizeQueryString(searchInput)
    setStatus(nextStatus)
    setSearch(nextSearch)
    setPage(1)
    setAllOrders([])
    setIsLoadingMore(false)
    replaceQueryState({
      page: 1,
      status: nextStatus,
      search: nextSearch,
    })
  }

  const handleSearchChange = (newSearch: string) => {
    setSearchInput(newSearch)
  }

  const handleRefresh = () => {
    const nextSearch = normalizeQueryString(searchInput)
    setSearch(nextSearch)
    setPage(1)
    setAllOrders([])
    setIsLoadingMore(false)
    replaceQueryState({
      page: 1,
      status,
      search: nextSearch,
    })
    refetch()
  }

  const handleResetFilters = useCallback(() => {
    setStatus(undefined)
    setSearch('')
    setSearchInput('')
    setPage(1)
    setAllOrders([])
    setIsLoadingMore(false)
    replaceQueryState({
      page: 1,
      status: undefined,
      search: undefined,
    })
  }, [replaceQueryState])

  const handlePageChange = useCallback(
    (nextPage: number) => {
      setPage(nextPage)
      replaceQueryState({
        page: nextPage,
        status,
        search,
      })
    },
    [replaceQueryState, search, status]
  )

  // 加载更多
  const loadMore = useCallback(() => {
    if (!isLoadingMore && hasMore && !isFetching) {
      const nextPage = page + 1
      setIsLoadingMore(true)
      setPage(nextPage)
      replaceQueryState({
        page: nextPage,
        status,
        search,
      })
    }
  }, [hasMore, isFetching, isLoadingMore, page, replaceQueryState, search, status])

  // 显示的订单列表
  const displayOrders = isMobile ? allOrders : orders
  const initialLoading = (isLoading && page === 1) || (isMobile && page > 1 && isRestoringPages)
  const hasActiveFilters = Boolean(search || status)
  const userOrdersPluginContext = {
    view: 'user_orders',
    filters: {
      page,
      status: status || undefined,
      search: search || undefined,
      search_input: searchInput || undefined,
      has_active_filters: hasActiveFilters,
    },
    pagination: {
      page,
      limit: pagination?.limit || orderListLimit,
      total: pagination?.total || 0,
      total_pages: pagination?.total_pages || 1,
      has_more: hasMore,
    },
    summary: {
      current_page_count: displayOrders.length,
      highlighted_order_no: highlightedOrderNo || undefined,
      initial_loading: initialLoading,
      is_fetching: isFetching,
      is_mobile: Boolean(isMobile && mounted),
      is_restoring_pages: isRestoringPages,
    },
    state: {
      load_failed: isError && displayOrders.length === 0 && !initialLoading,
      empty: !isError && !initialLoading && displayOrders.length === 0,
    },
  }

  const getOrdersScrollTop = useCallback(() => {
    if (typeof document === 'undefined' || typeof window === 'undefined') {
      return 0
    }
    const mainElement = document.querySelector('main')
    if (mainElement instanceof HTMLElement) {
      return Math.max(0, mainElement.scrollTop)
    }
    return Math.max(0, window.scrollY)
  }, [])

  const restoreOrdersScrollTop = useCallback((scrollTop: number) => {
    if (typeof document === 'undefined' || typeof window === 'undefined') {
      return
    }
    const nextScrollTop = Math.max(0, Number(scrollTop) || 0)
    window.requestAnimationFrame(() => {
      const mainElement = document.querySelector('main')
      if (mainElement instanceof HTMLElement) {
        mainElement.scrollTo({ top: nextScrollTop })
        return
      }
      window.scrollTo({ top: nextScrollTop })
    })
  }, [])

  const handleOpenOrder = useCallback(
    (orderNo: string) => {
      setListBrowseState('orders', {
        listPath: currentListPath,
        scrollTop: getOrdersScrollTop(),
        focusedItemKey: orderNo,
      })
      setHighlightedOrderNo(orderNo)
    },
    [currentListPath, getOrdersScrollTop]
  )

  useEffect(() => {
    if (
      !mounted ||
      initialLoading ||
      isFetching ||
      isRestoringPages ||
      hasRestoredBrowseStateRef.current
    ) {
      return
    }

    const browseState = readListBrowseState('orders')
    const shouldRestoreScroll = browseState?.listPath === currentListPath
    const focusedOrderNo = highlightedOrderNo || browseState?.focusedItemKey
    const focusedOrderElement = focusedOrderNo
      ? document.querySelector(`[data-order-no="${focusedOrderNo}"]`)
      : null

    if (!shouldRestoreScroll && !(focusedOrderElement instanceof HTMLElement)) {
      return
    }

    hasRestoredBrowseStateRef.current = true

    if (shouldRestoreScroll && browseState) {
      if (browseState.focusedItemKey && !highlightedOrderNo) {
        setHighlightedOrderNo(browseState.focusedItemKey)
      }
      restoreOrdersScrollTop(browseState.scrollTop)
      clearListBrowseState('orders')
      return
    }

    if (focusedOrderElement instanceof HTMLElement) {
      focusedOrderElement.scrollIntoView({
        block: 'center',
        behavior: 'smooth',
      })
    }
  }, [
    currentListPath,
    highlightedOrderNo,
    initialLoading,
    isFetching,
    isRestoringPages,
    mounted,
    restoreOrdersScrollTop,
  ])

  return (
    <div className="space-y-6">
      <PluginSlot slot="user.orders.top" context={userOrdersPluginContext} />
      <div className="flex items-center justify-between gap-3">
        <div>
          <h1 className="text-3xl font-bold">{t.order.myOrders}</h1>
        </div>
        <Button
          variant="outline"
          size="sm"
          onClick={handleRefresh}
          disabled={isFetching}
          aria-label={t.common.refresh}
          title={t.common.refresh}
          className="shrink-0"
        >
          <RefreshCw className={`h-4 w-4 md:mr-2 ${isFetching ? 'animate-spin' : ''}`} />
          <span className="hidden md:inline">{t.common.refresh}</span>
          <span className="sr-only md:hidden">{t.common.refresh}</span>
        </Button>
      </div>

      <OrderFilter
        status={status}
        search={searchInput}
        onStatusChange={handleStatusChange}
        onSearchChange={handleSearchChange}
        pluginSlotNamespace="user.orders"
        pluginSlotContext={userOrdersPluginContext}
        pluginSlotPath="/orders"
      />
      {hasActiveFilters ? (
        <div className="flex justify-end">
          <Button variant="ghost" size="sm" onClick={handleResetFilters}>
            {t.common.reset}
          </Button>
        </div>
      ) : null}
      <PluginSlot slot="user.orders.before_list" context={userOrdersPluginContext} />

      {isError && displayOrders.length === 0 && !initialLoading ? (
        <Card className="border-dashed bg-muted/15">
          <CardContent className="py-12 text-center">
            <RefreshCw className="mx-auto mb-4 h-12 w-12 text-muted-foreground" />
            <p className="text-base font-medium">{t.order.orderListLoadFailedTitle}</p>
            <p className="mt-2 text-sm text-muted-foreground">{t.order.orderListLoadFailedDesc}</p>
            <div className="mt-4 flex flex-wrap justify-center gap-2">
              <Button variant="outline" onClick={() => refetch()}>
                {t.common.refresh}
              </Button>
              {hasActiveFilters ? (
                <Button variant="ghost" onClick={handleResetFilters}>
                  {t.common.reset}
                </Button>
              ) : null}
            </div>
            <PluginSlot
              slot="user.orders.load_failed"
              context={{ ...userOrdersPluginContext, section: 'list_state' }}
            />
          </CardContent>
        </Card>
      ) : (
        <>
          <OrderList
            orders={displayOrders}
            isLoading={initialLoading}
            highlightedOrderNo={highlightedOrderNo}
            onOpenOrder={handleOpenOrder}
            emptyTitle={hasActiveFilters ? t.order.noOrdersFilteredTitle : t.order.noOrders}
            emptyDescription={hasActiveFilters ? t.order.noOrdersFilteredDesc : ''}
            emptyActionLabel={hasActiveFilters ? t.common.reset : t.pageTitle.products}
            onEmptyAction={hasActiveFilters ? handleResetFilters : () => router.push('/products')}
            pagination={
              isMobile
                ? undefined
                : {
                    page,
                    total_pages: pagination?.total_pages || 1,
                    onPageChange: handlePageChange,
                  }
            }
            // 移动端无限滚动相关 props
            isMobile={isMobile && mounted}
            isLoadingMore={isLoadingMore || isFetching || isRestoringPages}
            hasMore={hasMore}
            onLoadMore={loadMore}
            pluginSlotNamespace="user.orders"
            pluginSlotContext={userOrdersPluginContext}
            pluginSlotPath="/orders"
            pluginSlotQueryParams={pluginQueryParams}
          />
          {!initialLoading && !isError && displayOrders.length === 0 ? (
            <PluginSlot
              slot="user.orders.empty"
              context={{ ...userOrdersPluginContext, section: 'list_state' }}
            />
          ) : null}
        </>
      )}
      <PluginSlot slot="user.orders.bottom" context={userOrdersPluginContext} />
    </div>
  )
}

export default function OrdersPage() {
  return (
    <Suspense fallback={<div className="min-h-[40vh]" />}>
      <OrdersPageContent />
    </Suspense>
  )
}
