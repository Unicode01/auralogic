'use client'
/* eslint-disable @next/next/no-img-element */

import { usePathname, useRouter, useSearchParams } from 'next/navigation'
import { Suspense, useState, useEffect, useCallback, useRef } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getProducts, getProductCategories } from '@/lib/api'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Search, Package, Loader2, X } from 'lucide-react'
import Link from 'next/link'
import { Product } from '@/types/product'
import { useLocale } from '@/hooks/use-locale'
import { usePageTitle } from '@/hooks/use-page-title'
import { getTranslations } from '@/lib/i18n'
import { useIsMobile } from '@/hooks/use-mobile'
import { useInfiniteScroll } from '@/hooks/use-infinite-scroll'
import { useCurrency, formatPrice } from '@/contexts/currency-context'
import {
  buildUpdatedQueryString,
  normalizePositivePageQuery,
  normalizeQueryString,
} from '@/lib/query-state'
import {
  clearProductBrowseState,
  parseFocusedProductIdQuery,
  productListFocusParamKey,
  readProductBrowseState,
  setProductBrowseState,
  stripProductListFocusFromPath,
} from '@/lib/product-browse-state'
import { PluginSlot } from '@/components/plugins/plugin-slot'

const productListLimit = 12
const EMPTY_PRODUCTS: Product[] = []

function ProductsPageContent() {
  const router = useRouter()
  const pathname = usePathname() || '/products'
  const searchParams = useSearchParams()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.products)
  const { currency } = useCurrency()
  const { isMobile, mounted } = useIsMobile()
  const searchParamsKey = searchParams.toString()
  const initialSearch = normalizeQueryString(searchParams.get('search'))
  const initialCategory = normalizeQueryString(searchParams.get('category')) || undefined
  const initialPage = normalizePositivePageQuery(searchParams.get('page'))
  const initialListPath = stripProductListFocusFromPath(
    searchParamsKey ? `${pathname}?${searchParamsKey}` : pathname
  )
  const initialBrowseState = readProductBrowseState()
  const initialFocusedProductId =
    parseFocusedProductIdQuery(searchParams.get(productListFocusParamKey)) ||
    (initialBrowseState?.listPath === initialListPath
      ? initialBrowseState.focusedProductId
      : undefined)
  const [page, setPage] = useState(initialPage)
  const [category, setCategory] = useState<string | undefined>(initialCategory)
  const [search, setSearch] = useState(initialSearch)
  const [searchInput, setSearchInput] = useState(initialSearch)
  const [highlightedProductId, setHighlightedProductId] = useState<number | undefined>(
    initialFocusedProductId
  )
  const stateRef = useRef({
    page: initialPage,
    category: initialCategory,
    search: initialSearch,
    searchInput: initialSearch,
  })
  const hasRestoredBrowseStateRef = useRef(false)

  // 移动端累积商品列表
  const [allProducts, setAllProducts] = useState<Product[]>([])
  const [isLoadingMore, setIsLoadingMore] = useState(false)
  const [isRestoringPages, setIsRestoringPages] = useState(false)
  const currentListPath = initialListPath

  const replaceQueryState = useCallback(
    (nextState: { search?: string; category?: string; page?: number }) => {
      const queryString = buildUpdatedQueryString(
        searchParams,
        {
          search: nextState.search || undefined,
          category: nextState.category || undefined,
          page: nextState.page,
          [productListFocusParamKey]: undefined,
        },
        { page: 1 }
      )
      router.replace(queryString ? `${pathname}?${queryString}` : pathname, { scroll: false })
    },
    [pathname, router, searchParams]
  )

  // 获取商品列表
  const {
    data: productsData,
    isLoading,
    isFetching,
    isError,
    refetch,
  } = useQuery({
    queryKey: ['products', page, category, search],
    queryFn: () =>
      getProducts({
        page,
        limit: productListLimit,
        category: category === 'all' ? undefined : category,
        search: search || undefined,
      }),
  })

  // 获取分类列表
  const { data: categoriesData } = useQuery({
    queryKey: ['productCategories'],
    queryFn: getProductCategories,
  })

  const products = productsData?.data?.items ?? EMPTY_PRODUCTS
  const pagination = productsData?.data?.pagination
  const categories = categoriesData?.data?.categories || []
  const hasMore = pagination ? page < pagination.total_pages : false

  useEffect(() => {
    stateRef.current = {
      page,
      category,
      search,
      searchInput,
    }
  }, [category, page, search, searchInput])

  useEffect(() => {
    const nextSearch = normalizeQueryString(searchParams.get('search'))
    const nextCategory = normalizeQueryString(searchParams.get('category')) || undefined
    const nextPage = normalizePositivePageQuery(searchParams.get('page'))
    const browseState = readProductBrowseState()
    const nextFocusedProductId =
      parseFocusedProductIdQuery(searchParams.get(productListFocusParamKey)) ||
      (browseState?.listPath === currentListPath ? browseState.focusedProductId : undefined)
    const currentState = stateRef.current
    const urlStateChanged =
      nextSearch !== currentState.search ||
      nextSearch !== currentState.searchInput ||
      nextCategory !== currentState.category ||
      nextPage !== currentState.page

    if (!urlStateChanged) {
      return
    }

    setSearch(nextSearch)
    setSearchInput(nextSearch)
    setCategory(nextCategory)
    setPage(nextPage)
    setHighlightedProductId(nextFocusedProductId)
    setAllProducts([])
    setIsLoadingMore(false)
    setIsRestoringPages(false)
    hasRestoredBrowseStateRef.current = false
  }, [currentListPath, searchParams, searchParamsKey])

  useEffect(() => {
    if (!searchParams.get(productListFocusParamKey)) {
      return
    }
    router.replace(currentListPath, { scroll: false })
  }, [currentListPath, router, searchParams, searchParamsKey])

  // 移动端：累积商品数据
  useEffect(() => {
    if (isMobile && !isRestoringPages && products.length > 0) {
      if (page === 1) {
        setAllProducts(products)
      } else {
        setAllProducts((prev) => {
          // 避免重复添加
          const existingIds = new Set(prev.map((p) => p.id))
          const newProducts = products.filter((p: Product) => !existingIds.has(p.id))
          return [...prev, ...newProducts]
        })
      }
      setIsLoadingMore(false)
    }
  }, [isMobile, isRestoringPages, page, products])

  useEffect(() => {
    if (!isMobile || !mounted || page <= 1 || allProducts.length > 0) {
      return
    }

    let cancelled = false
    setIsRestoringPages(true)
    setIsLoadingMore(true)

    const restoreProducts = async () => {
      try {
        const responses = await Promise.all(
          Array.from({ length: page }, (_item, index) =>
            getProducts({
              page: index + 1,
              limit: productListLimit,
              category,
              search: search || undefined,
            })
          )
        )
        if (cancelled) {
          return
        }
        const mergedProducts = responses.flatMap((response) => response?.data?.items || [])
        const existingIds = new Set<number>()
        const dedupedProducts = mergedProducts.filter((product: Product) => {
          if (existingIds.has(product.id)) {
            return false
          }
          existingIds.add(product.id)
          return true
        })
        setAllProducts(dedupedProducts)
      } finally {
        if (!cancelled) {
          setIsRestoringPages(false)
          setIsLoadingMore(false)
        }
      }
    }

    void restoreProducts()

    return () => {
      cancelled = true
    }
  }, [allProducts.length, category, isMobile, mounted, page, search])

  // 搜索或分类变化时重置
  const handleSearch = () => {
    const nextSearch = normalizeQueryString(searchInput)
    setSearch(nextSearch)
    setPage(1)
    setAllProducts([])
    setIsLoadingMore(false)
    replaceQueryState({
      search: nextSearch,
      category,
      page: 1,
    })
  }

  const handleClearSearch = useCallback(() => {
    setSearchInput('')
    if (!search) {
      return
    }
    setSearch('')
    setPage(1)
    setAllProducts([])
    setIsLoadingMore(false)
    replaceQueryState({
      search: undefined,
      category,
      page: 1,
    })
  }, [category, replaceQueryState, search])

  const handleCategoryChange = (value: string) => {
    const nextCategory = value === 'all' ? undefined : value
    setCategory(nextCategory)
    setPage(1)
    setAllProducts([])
    setIsLoadingMore(false)
    replaceQueryState({
      search,
      category: nextCategory,
      page: 1,
    })
  }

  const handlePageChange = (nextPage: number) => {
    setPage(nextPage)
    replaceQueryState({
      search,
      category,
      page: nextPage,
    })
  }

  // 加载更多
  const loadMore = useCallback(() => {
    if (!isLoadingMore && hasMore && !isFetching) {
      const nextPage = page + 1
      setIsLoadingMore(true)
      setPage(nextPage)
      replaceQueryState({
        search,
        category,
        page: nextPage,
      })
    }
  }, [category, hasMore, isFetching, isLoadingMore, page, replaceQueryState, search])

  // 无限滚动 hook
  const { loadMoreRef } = useInfiniteScroll({
    enabled: isMobile && mounted,
    isLoading: isLoading || isLoadingMore || isFetching,
    hasMore,
    onLoadMore: loadMore,
  })

  // 显示的商品列表
  const displayProducts = isMobile ? allProducts : products
  const initialLoading = (isLoading && page === 1) || (isMobile && page > 1 && isRestoringPages)
  const hasActiveFilters = Boolean(search || category)
  const productFilterBadges = [
    search ? `${t.common.search}: ${search}` : null,
    category ? `${t.product.category || t.product.selectCategory}: ${category}` : null,
  ].filter(Boolean) as string[]
  const userProductsPluginContext = {
    view: 'user_products',
    filters: {
      page,
      search: search || undefined,
      category: category === 'all' ? undefined : category,
      device: isMobile ? 'mobile' : 'desktop',
    },
    pagination: {
      page,
      total_pages: pagination?.total_pages,
      total: pagination?.total,
      limit: pagination?.limit,
    },
    summary: {
      current_page_count: displayProducts.length,
      category_count: categories.length,
      active_filter_count: productFilterBadges.length,
      has_more: hasMore,
    },
    active_filter_badges: productFilterBadges,
    state: {
      load_failed: isError && displayProducts.length === 0,
      empty: !initialLoading && !isError && displayProducts.length === 0,
    },
  }

  const getProductsScrollTop = useCallback(() => {
    if (typeof document === 'undefined' || typeof window === 'undefined') {
      return 0
    }
    const mainElement = document.querySelector('main')
    if (mainElement instanceof HTMLElement) {
      return Math.max(0, mainElement.scrollTop)
    }
    return Math.max(0, window.scrollY)
  }, [])

  const restoreProductsScrollTop = useCallback((scrollTop: number) => {
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

  const handleOpenProduct = useCallback(
    (productId: number) => {
      setProductBrowseState({
        listPath: currentListPath,
        scrollTop: getProductsScrollTop(),
        focusedProductId: productId,
      })
      setHighlightedProductId(productId)
    },
    [currentListPath, getProductsScrollTop]
  )

  const handleResetFilters = useCallback(() => {
    setSearch('')
    setSearchInput('')
    setCategory(undefined)
    setPage(1)
    setAllProducts([])
    setIsLoadingMore(false)
    replaceQueryState({
      search: undefined,
      category: undefined,
      page: 1,
    })
  }, [replaceQueryState])

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

    const browseState = readProductBrowseState()
    const shouldRestoreScroll = browseState?.listPath === currentListPath
    const focusedProductId = highlightedProductId || browseState?.focusedProductId
    const focusedProductElement = focusedProductId
      ? document.querySelector(`[data-product-id="${focusedProductId}"]`)
      : null

    if (!shouldRestoreScroll && !(focusedProductElement instanceof HTMLElement)) {
      return
    }

    hasRestoredBrowseStateRef.current = true

    if (shouldRestoreScroll && browseState) {
      if (browseState.focusedProductId && !highlightedProductId) {
        setHighlightedProductId(browseState.focusedProductId)
      }
      restoreProductsScrollTop(browseState.scrollTop)
      clearProductBrowseState()
      return
    }

    if (focusedProductElement instanceof HTMLElement) {
      focusedProductElement.scrollIntoView({
        block: 'center',
        behavior: 'smooth',
      })
    }
  }, [
    currentListPath,
    highlightedProductId,
    initialLoading,
    isFetching,
    isRestoringPages,
    mounted,
    restoreProductsScrollTop,
  ])

  return (
    <div className="space-y-6">
      <PluginSlot slot="user.products.top" context={userProductsPluginContext} />
      {/* 页面标题 */}
      <div>
        <h1 className={isMobile ? 'text-2xl font-bold' : 'text-2xl font-bold md:text-3xl'}>
          {t.sidebar.productCenter}
        </h1>
      </div>

      {/* 搜索和筛选 */}
      <div className="space-y-4">
        {/* 移动端：搜索栏和分类栏分两行显示 */}
        <div className={isMobile ? 'flex flex-col gap-3' : 'flex flex-col gap-3 md:flex-row md:gap-4'}>
          {/* 搜索栏 */}
          <div className={isMobile ? 'flex w-full gap-2' : 'flex w-full gap-2 md:flex-1'}>
            <div className="relative flex-1">
              <Input
                placeholder={t.product.searchPlaceholder}
                value={searchInput}
                onChange={(e) => setSearchInput(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
                className="flex-1 pr-9"
              />
              {searchInput && (
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="absolute right-1 top-1/2 h-7 w-7 -translate-y-1/2 rounded-full text-muted-foreground hover:text-foreground"
                  onClick={handleClearSearch}
                  aria-label={t.common.clear}
                  title={t.common.clear}
                >
                  <X className="h-4 w-4" />
                  <span className="sr-only">{t.common.clear}</span>
                </Button>
              )}
            </div>
            <Button onClick={handleSearch} className="shrink-0">
              <Search className={`h-4 w-4 ${!isMobile ? 'md:mr-2' : ''}`} />
              {isMobile ? (
                <span className="sr-only">{t.common.search}</span>
              ) : (
                <span>{t.common.search}</span>
              )}
            </Button>
          </div>
          {/* 分类选择器 */}
          {categories.length > 0 && (
            <Select value={category || 'all'} onValueChange={handleCategoryChange}>
              <SelectTrigger className={isMobile ? 'w-full' : 'w-full md:w-[200px]'}>
                <SelectValue placeholder={t.product.selectCategory} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">{t.product.allCategories}</SelectItem>
                {categories.map((cat: string) => (
                  <SelectItem key={cat} value={cat}>
                    {cat}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        </div>
        <PluginSlot
          slot="user.products.filters"
          context={{ ...userProductsPluginContext, section: 'filters' }}
        />
      </div>

      {hasActiveFilters ? (
        <div className="flex justify-end">
          <Button variant="ghost" size="sm" onClick={handleResetFilters}>
            {t.common.reset}
          </Button>
        </div>
      ) : null}

      {/* 商品网格 */}
      {initialLoading ? (
        <div
          className={
            isMobile
              ? 'grid grid-cols-2 gap-3'
              : 'grid grid-cols-2 gap-3 md:gap-4 lg:grid-cols-3 xl:grid-cols-4'
          }
        >
          {[...Array(8)].map((_, i) => (
            <Card key={i} className="animate-pulse">
              <div className="aspect-square bg-muted" />
              <CardContent className={isMobile ? 'space-y-2 p-2' : 'space-y-2 p-2 md:p-3'}>
                <div className="h-4 rounded bg-muted" />
                <div className="h-4 w-2/3 rounded bg-muted" />
              </CardContent>
            </Card>
          ))}
        </div>
      ) : isError && displayProducts.length === 0 ? (
        <Card className="border-dashed bg-muted/15">
          <CardContent className="py-12 text-center">
            <Package className="mx-auto mb-4 h-12 w-12 text-muted-foreground" />
            <p className="text-base font-medium">{t.product.productListLoadFailedTitle}</p>
            <p className="mt-2 text-sm text-muted-foreground">
              {t.product.productListLoadFailedDesc}
            </p>
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
              slot="user.products.load_failed"
              context={{ ...userProductsPluginContext, section: 'list_state' }}
            />
          </CardContent>
        </Card>
      ) : displayProducts.length > 0 ? (
        <>
          <div
            className={
              isMobile
                ? 'grid grid-cols-2 gap-3'
                : 'grid grid-cols-2 gap-3 md:gap-4 lg:grid-cols-3 xl:grid-cols-4'
            }
          >
            {displayProducts.map((product: Product) => {
              const primaryImage = product.images?.find(
                (img) => img.is_primary || img.isPrimary
              )?.url
              const isFeatured = product.is_featured || product.isFeatured
              const hasDiscount = product.original_price_minor > product.price_minor
              const isVirtual = (product.product_type || product.productType) === 'virtual'
              const isSoldOut = product.status === 'out_of_stock'
              const isHighlighted = highlightedProductId === product.id

              return (
                <Link
                  key={product.id}
                  href={`/products/${product.id}`}
                  data-product-id={product.id}
                  onClick={() => handleOpenProduct(product.id)}
                >
                  <Card
                    className={`h-full cursor-pointer transition-all hover:shadow-lg ${
                      isSoldOut ? 'opacity-70' : ''
                    } ${
                      isHighlighted
                        ? 'border-primary shadow-lg shadow-primary/10 ring-2 ring-primary/70'
                        : ''
                    }`}
                  >
                    <div className="relative aspect-square overflow-hidden rounded-t-lg">
                      {primaryImage ? (
                        <img
                          src={primaryImage}
                          alt={product.name}
                          className="h-full w-full object-cover transition-transform hover:scale-105"
                          onError={(e) => {
                            const target = e.currentTarget
                            target.style.display = 'none'
                            target.parentElement
                              ?.querySelector('.img-fallback')
                              ?.classList.remove('hidden')
                          }}
                        />
                      ) : null}
                      <div
                        className={`img-fallback flex h-full w-full items-center justify-center bg-muted ${primaryImage ? 'hidden' : ''}`}
                      >
                        <Package className="h-16 w-16 text-muted-foreground" />
                      </div>
                      {/* 售罄遮罩 */}
                      {isSoldOut && (
                        <div className="absolute inset-0 flex items-center justify-center bg-black/50">
                          <span
                            className={
                              isMobile
                                ? 'rounded bg-black/60 px-3 py-1 text-sm font-bold text-white'
                                : 'rounded bg-black/60 px-3 py-1 text-sm font-bold text-white md:text-base'
                            }
                          >
                            {t.product.soldOut}
                          </span>
                        </div>
                      )}
                      {/* 商品标签 - 精简版 */}
                      <div className="absolute left-1.5 top-1.5 flex flex-wrap gap-1">
                        {isVirtual && (
                          <span className="rounded bg-purple-500 px-1.5 py-0.5 text-[10px] font-medium text-white">
                            {t.product.virtualBadge}
                          </span>
                        )}
                        {isFeatured && (
                          <span className="rounded bg-yellow-500 px-1.5 py-0.5 text-[10px] font-medium text-white">
                            {t.product.featuredBadge}
                          </span>
                        )}
                        {hasDiscount && (
                          <span className="rounded bg-red-500 px-1.5 py-0.5 text-[10px] font-medium text-white">
                            {t.product.saleBadge}
                          </span>
                        )}
                      </div>
                    </div>
                    <CardContent
                      className={
                        isMobile ? 'space-y-1 p-2' : 'space-y-1 p-2 md:space-y-1.5 md:p-3'
                      }
                    >
                      <h3
                        className={
                          isMobile
                            ? 'line-clamp-2 min-h-[2.5rem] text-sm font-semibold'
                            : 'line-clamp-2 min-h-[2.5rem] text-sm font-semibold md:min-h-[2.8rem] md:text-base'
                        }
                      >
                        {product.name}
                      </h3>
                      {!isMobile && product.short_description && (
                        <p className="line-clamp-1 text-xs text-muted-foreground">
                          {product.short_description}
                        </p>
                      )}
                      <div
                        className={
                          isMobile
                            ? 'flex flex-wrap items-baseline gap-1 pt-1'
                            : 'flex flex-wrap items-baseline gap-1 pt-1 md:gap-2'
                        }
                      >
                        <span
                          className={
                            isMobile
                              ? 'text-base font-bold text-red-600'
                              : 'text-base font-bold text-red-600 md:text-xl'
                          }
                        >
                          {formatPrice(product.price_minor, currency)}
                        </span>
                        {/* 原价：移动端隐藏，桌面端显示 */}
                        {!isMobile && hasDiscount && (
                          <span className="text-xs text-muted-foreground line-through">
                            {formatPrice(product.original_price_minor, currency)}
                          </span>
                        )}
                      </div>
                      {!isMobile && product.category ? (
                        <p className="pt-1 text-xs text-muted-foreground">
                          {product.category}
                        </p>
                      ) : null}
                    </CardContent>
                  </Card>
                </Link>
              )
            })}
          </div>

          {/* 移动端：无限滚动加载指示器 */}
          {isMobile && mounted && (
            <div ref={loadMoreRef} className="flex justify-center py-4">
              {(isLoadingMore || isFetching || isRestoringPages) && hasMore ? (
                <div className="flex items-center gap-2 text-muted-foreground">
                  <Loader2 className="h-4 w-4 animate-spin" />
                  <span className="text-sm">{t.common.loading}</span>
                </div>
              ) : hasMore ? (
                <span className="text-sm text-muted-foreground">{t.common.scrollToLoadMore}</span>
              ) : allProducts.length > 0 ? (
                <span className="text-sm text-muted-foreground">{t.product.noMoreProducts}</span>
              ) : null}
            </div>
          )}

          {/* PC端：分页 */}
          {!isMobile && pagination && (
            <div className="mt-8 flex items-center justify-between">
              <p className="text-sm text-muted-foreground">
                {t.common.pageInfo
                  .replace('{page}', String(page))
                  .replace('{totalPages}', String(pagination.total_pages))}
              </p>
              <div className="flex items-center gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  disabled={page === 1}
                  onClick={() => handlePageChange(page - 1)}
                >
                  {t.common.prevPage}
                </Button>
                <input
                  type="number"
                  min={1}
                  max={pagination.total_pages}
                  defaultValue={page}
                  key={page}
                  onBlur={(e) => {
                    const p = parseInt(e.target.value)
                    if (p >= 1 && p <= pagination.total_pages && p !== page) {
                      handlePageChange(p)
                    }
                  }}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') {
                      const p = parseInt((e.target as HTMLInputElement).value)
                      if (p >= 1 && p <= pagination.total_pages && p !== page) {
                        handlePageChange(p)
                      }
                      ;(e.target as HTMLInputElement).blur()
                    }
                  }}
                  className="h-8 w-12 rounded-md border bg-background text-center text-sm [appearance:textfield] focus:outline-none focus:ring-2 focus:ring-ring [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none"
                />
                <Button
                  variant="outline"
                  size="sm"
                  disabled={page >= pagination.total_pages}
                  onClick={() => handlePageChange(page + 1)}
                >
                  {t.common.nextPage}
                </Button>
              </div>
            </div>
          )}
        </>
      ) : (
        <Card className="border-dashed bg-muted/15">
          <CardContent className="py-12 text-center">
            <Package className="mx-auto mb-4 h-16 w-16 text-muted-foreground" />
            <p className="text-base font-medium">
              {hasActiveFilters
                ? t.product.noProductsFilteredTitle
                : t.product.noProductsEmptyTitle}
            </p>
            {hasActiveFilters ? (
              <p className="mt-2 text-sm text-muted-foreground">
                {t.product.noProductsFilteredDesc}
              </p>
            ) : null}
            {hasActiveFilters && (
              <Button variant="outline" className="mt-4" onClick={handleResetFilters}>
                {t.common.reset}
              </Button>
            )}
            <PluginSlot
              slot="user.products.empty"
              context={{ ...userProductsPluginContext, section: 'list_state' }}
            />
          </CardContent>
        </Card>
      )}
      <PluginSlot slot="user.products.bottom" context={userProductsPluginContext} />
    </div>
  )
}

export default function ProductsPage() {
  return (
    <Suspense fallback={<div className="min-h-[40vh]" />}>
      <ProductsPageContent />
    </Suspense>
  )
}
