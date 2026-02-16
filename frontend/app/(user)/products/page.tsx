'use client'

import { useState, useEffect, useCallback } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getProducts, getProductCategories } from '@/lib/api'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Search, Package, Loader2 } from 'lucide-react'
import Link from 'next/link'
import { Product } from '@/types/product'
import { useLocale } from '@/hooks/use-locale'
import { usePageTitle } from '@/hooks/use-page-title'
import { getTranslations } from '@/lib/i18n'
import { useIsMobile } from '@/hooks/use-mobile'
import { useInfiniteScroll } from '@/hooks/use-infinite-scroll'
import { useCurrency, formatPrice } from '@/contexts/currency-context'

export default function ProductsPage() {
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.products)
  const { currency } = useCurrency()
  const { isMobile, mounted } = useIsMobile()
  const [page, setPage] = useState(1)
  const [category, setCategory] = useState<string | undefined>()
  const [search, setSearch] = useState('')
  const [searchInput, setSearchInput] = useState('')

  // 移动端累积商品列表
  const [allProducts, setAllProducts] = useState<Product[]>([])
  const [isLoadingMore, setIsLoadingMore] = useState(false)

  // 获取商品列表
  const { data: productsData, isLoading, isFetching } = useQuery({
    queryKey: ['products', page, category, search],
    queryFn: () => getProducts({
      page,
      limit: 12,
      category: category === 'all' ? undefined : category,
      search: search || undefined,
    }),
  })

  // 获取分类列表
  const { data: categoriesData } = useQuery({
    queryKey: ['productCategories'],
    queryFn: getProductCategories,
  })

  const products = productsData?.data?.items || []
  const pagination = productsData?.data?.pagination
  const categories = categoriesData?.data?.categories || []
  const hasMore = pagination ? page < pagination.total_pages : false

  // 移动端：累积商品数据
  useEffect(() => {
    if (isMobile && products.length > 0) {
      if (page === 1) {
        setAllProducts(products)
      } else {
        setAllProducts(prev => {
          // 避免重复添加
          const existingIds = new Set(prev.map(p => p.id))
          const newProducts = products.filter((p: Product) => !existingIds.has(p.id))
          return [...prev, ...newProducts]
        })
      }
      setIsLoadingMore(false)
    }
  }, [products, page, isMobile])

  // 搜索或分类变化时重置
  const handleSearch = () => {
    setSearch(searchInput)
    setPage(1)
    setAllProducts([])
  }

  const handleCategoryChange = (value: string) => {
    setCategory(value === 'all' ? undefined : value)
    setPage(1)
    setAllProducts([])
  }

  // 加载更多
  const loadMore = useCallback(() => {
    if (!isLoadingMore && hasMore && !isFetching) {
      setIsLoadingMore(true)
      setPage(prev => prev + 1)
    }
  }, [isLoadingMore, hasMore, isFetching])

  // 无限滚动 hook
  const { loadMoreRef } = useInfiniteScroll({
    enabled: isMobile && mounted,
    isLoading: isLoading || isLoadingMore || isFetching,
    hasMore,
    onLoadMore: loadMore,
  })

  // 显示的商品列表
  const displayProducts = isMobile ? allProducts : products

  return (
    <div className="space-y-6">
      {/* 页面标题 */}
      <div>
        <h1 className="text-2xl md:text-3xl font-bold mb-2">{t.sidebar.productCenter}</h1>
        <p className="text-sm md:text-base text-muted-foreground">
          {locale === 'zh' ? '浏览我们的精选商品' : 'Browse our featured products'}
        </p>
      </div>

      {/* 搜索和筛选 */}
      <div className="space-y-4">
        {/* 移动端：搜索栏和分类栏分两行显示 */}
        <div className="flex flex-col md:flex-row gap-3 md:gap-4">
          {/* 搜索栏 */}
          <div className="flex gap-2 w-full md:flex-1">
            <Input
              placeholder={locale === 'zh' ? '搜索商品...' : 'Search products...'}
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
              onKeyPress={(e) => e.key === 'Enter' && handleSearch()}
              className="flex-1"
            />
            <Button onClick={handleSearch} className="shrink-0">
              <Search className="h-4 w-4 md:mr-2" />
              <span className="hidden md:inline">{t.common.search}</span>
            </Button>
          </div>
          {/* 分类选择器 */}
          {categories.length > 0 && (
            <Select
              value={category || 'all'}
              onValueChange={handleCategoryChange}
            >
              <SelectTrigger className="w-full md:w-[200px]">
                <SelectValue placeholder={locale === 'zh' ? '选择分类' : 'Select category'} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">{locale === 'zh' ? '全部分类' : 'All Categories'}</SelectItem>
                {categories.map((cat: string) => (
                  <SelectItem key={cat} value={cat}>
                    {cat}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        </div>
      </div>

      {/* 商品网格 */}
      {isLoading && page === 1 ? (
        <div className="grid grid-cols-2 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-3 md:gap-4">
          {[...Array(8)].map((_, i) => (
            <Card key={i} className="animate-pulse">
              <div className="aspect-square bg-muted" />
              <CardContent className="p-2 md:p-3 space-y-2">
                <div className="h-4 bg-muted rounded" />
                <div className="h-4 bg-muted rounded w-2/3" />
              </CardContent>
            </Card>
          ))}
        </div>
      ) : displayProducts.length > 0 ? (
        <>
          <div className="grid grid-cols-2 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-3 md:gap-4">
            {displayProducts.map((product: Product) => {
              const primaryImage = product.images?.find(img => img.is_primary || img.isPrimary)?.url
              const isFeatured = product.is_featured || product.isFeatured
              const hasDiscount = product.original_price && product.original_price > product.price
              const isVirtual = (product.product_type || product.productType) === 'virtual'

              return (
                <Link key={product.id} href={`/products/${product.id}`}>
                  <Card className="h-full hover:shadow-lg transition-shadow cursor-pointer">
                    <div className="relative aspect-square overflow-hidden rounded-t-lg">
                      {primaryImage ? (
                        <img
                          src={primaryImage}
                          alt={product.name}
                          className="w-full h-full object-cover hover:scale-105 transition-transform"
                          onError={(e) => {
                            const target = e.currentTarget
                            target.style.display = 'none'
                            target.parentElement?.querySelector('.img-fallback')?.classList.remove('hidden')
                          }}
                        />
                      ) : null}
                      <div className={`img-fallback w-full h-full bg-muted flex items-center justify-center ${primaryImage ? 'hidden' : ''}`}>
                        <Package className="w-16 h-16 text-muted-foreground" />
                      </div>
                      {/* 商品标签 - 精简版 */}
                      <div className="absolute top-1.5 left-1.5 flex flex-wrap gap-1">
                        {isVirtual && (
                          <span className="px-1.5 py-0.5 text-[10px] font-medium bg-purple-500 text-white rounded">
                            {locale === 'zh' ? '虚拟' : 'V'}
                          </span>
                        )}
                        {isFeatured && (
                          <span className="px-1.5 py-0.5 text-[10px] font-medium bg-yellow-500 text-white rounded">
                            {locale === 'zh' ? '精选' : 'HOT'}
                          </span>
                        )}
                        {hasDiscount && (
                          <span className="px-1.5 py-0.5 text-[10px] font-medium bg-red-500 text-white rounded">
                            {locale === 'zh' ? '惠' : 'SALE'}
                          </span>
                        )}
                      </div>
                    </div>
                    <CardContent className="p-2 md:p-3 space-y-1 md:space-y-1.5">
                      <h3 className="font-semibold text-sm md:text-base line-clamp-2 min-h-[2.5rem] md:min-h-[2.8rem]">
                        {product.name}
                      </h3>
                      {product.short_description && (
                        <p className="text-xs text-muted-foreground line-clamp-1 hidden md:block">
                          {product.short_description}
                        </p>
                      )}
                      <div className="flex items-baseline gap-1 md:gap-2 pt-1 flex-wrap">
                        <span className="text-base md:text-xl font-bold text-red-600">
                          {formatPrice(product.price, currency)}
                        </span>
                        {/* 原价：移动端隐藏，桌面端显示 */}
                        {hasDiscount && (
                          <span className="hidden md:inline text-xs text-muted-foreground line-through">
                            {formatPrice(product.original_price!, currency)}
                          </span>
                        )}
                      </div>
                      {product.category && (
                        <div className="pt-1 hidden md:block">
                          <Badge variant="outline" className="text-xs">{product.category}</Badge>
                        </div>
                      )}
                    </CardContent>
                  </Card>
                </Link>
              )
            })}
          </div>

          {/* 移动端：无限滚动加载指示器 */}
          {isMobile && mounted && (
            <div ref={loadMoreRef} className="flex justify-center py-4">
              {(isLoadingMore || isFetching) && hasMore ? (
                <div className="flex items-center gap-2 text-muted-foreground">
                  <Loader2 className="h-4 w-4 animate-spin" />
                  <span className="text-sm">{locale === 'zh' ? '加载中...' : 'Loading...'}</span>
                </div>
              ) : hasMore ? (
                <span className="text-sm text-muted-foreground">
                  {locale === 'zh' ? '向下滚动加载更多' : 'Scroll down to load more'}
                </span>
              ) : allProducts.length > 0 ? (
                <span className="text-sm text-muted-foreground">
                  {locale === 'zh' ? '没有更多商品了' : 'No more products'}
                </span>
              ) : null}
            </div>
          )}

          {/* PC端：分页 */}
          {!isMobile && pagination && (
            <div className="mt-8 flex items-center justify-between">
              <p className="text-sm text-muted-foreground">
                {locale === 'zh'
                  ? `第 ${page} 页，共 ${pagination.total_pages} 页`
                  : `Page ${page} of ${pagination.total_pages}`}
              </p>
              <div className="flex items-center gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  disabled={page === 1}
                  onClick={() => setPage(page - 1)}
                >
                  {locale === 'zh' ? '上一页' : 'Previous'}
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
                      setPage(p)
                    }
                  }}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') {
                      const p = parseInt((e.target as HTMLInputElement).value)
                      if (p >= 1 && p <= pagination.total_pages && p !== page) {
                        setPage(p)
                      }
                      ;(e.target as HTMLInputElement).blur()
                    }
                  }}
                  className="w-12 h-8 text-center text-sm border rounded-md bg-background focus:outline-none focus:ring-2 focus:ring-ring [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
                />
                <Button
                  variant="outline"
                  size="sm"
                  disabled={page >= pagination.total_pages}
                  onClick={() => setPage(page + 1)}
                >
                  {locale === 'zh' ? '下一页' : 'Next'}
                </Button>
              </div>
            </div>
          )}
        </>
      ) : (
        <div className="text-center py-12">
          <Package className="w-16 h-16 text-muted-foreground mx-auto mb-4" />
          <p className="text-muted-foreground">
            {locale === 'zh' ? '没有找到商品' : 'No products found'}
          </p>
        </div>
      )}
    </div>
  )
}
