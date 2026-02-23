'use client'

import { useParams, useRouter } from 'next/navigation'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { getProduct, createOrder, getProductAvailableStock, addToCart, validatePromoCode, getPublicConfig } from '@/lib/api'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { Star, Package, Eye, ShoppingCart, Loader2, ArrowLeft, Key, Minus, Plus, Tag } from 'lucide-react'
import { useState, useRef, useCallback, useMemo } from 'react'
import { Input } from '@/components/ui/input'
import Link from 'next/link'
import { useToast } from '@/hooks/use-toast'
import { useAuth } from '@/hooks/use-auth'
import { useLocale } from '@/hooks/use-locale'
import { usePageTitle } from '@/hooks/use-page-title'
import { getTranslations } from '@/lib/i18n'
import { useCurrency, formatPrice, getCurrencySymbol } from '@/contexts/currency-context'
import { MarkdownMessage } from '@/components/ui/markdown-message'

export default function ProductDetailPage() {
  const params = useParams()
  const router = useRouter()
  const queryClient = useQueryClient()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.productDetail)
  const productId = Number(params.id)
  const [selectedImage, setSelectedImage] = useState(0)
  const [selectedAttributes, setSelectedAttributes] = useState<Record<string, string>>({})
  const [quantity, setQuantity] = useState(1)
  const [isAddingToCart, setIsAddingToCart] = useState(false)
  const toast = useToast()
  const { user } = useAuth()
  const { currency } = useCurrency()

  // Promo code state
  const [promoCodeInput, setPromoCodeInput] = useState('')
  const [isValidatingPromo, setIsValidatingPromo] = useState(false)
  const [appliedPromo, setAppliedPromo] = useState<{
    code: string
    promo_code_id: number
    name: string
    discount_type: string
    discount_value: number
    max_discount: number
    min_order_amount: number
  } | null>(null)

  const { data, isLoading } = useQuery({
    queryKey: ['product', productId],
    queryFn: () => getProduct(productId),
  })

  const product = data?.data
  const isVirtual = product && (product.product_type || product.productType) === 'virtual'

  const { data: stockData, refetch: refetchStock } = useQuery({
    queryKey: ['productStock', productId, selectedAttributes],
    queryFn: () => {
      if (Object.keys(selectedAttributes).length > 0) {
        return getProductAvailableStock(productId, selectedAttributes)
      }
      return getProductAvailableStock(productId)
    },
    refetchInterval: 30000,
    enabled: !!product,
  })

  const { data: publicConfig } = useQuery({
    queryKey: ['publicConfig'],
    queryFn: getPublicConfig,
    staleTime: 1000 * 60 * 5,
  })

  const createOrderMutation = useMutation({
    mutationFn: createOrder,
    onSuccess: (response) => {
      toast.success(`${t.product.orderCreated} ${response.data.order_no}`)
      queryClient.invalidateQueries({ queryKey: ['product', productId] })
      queryClient.invalidateQueries({ queryKey: ['products'] })
      queryClient.invalidateQueries({ queryKey: ['productStock', productId] })
      queryClient.invalidateQueries({ queryKey: ['orders'] })
      setTimeout(() => router.push('/orders'), 1000)
    },
    onError: (error: Error) => {
      toast.error(error.message || t.product.orderCreateFailed)
    },
  })

  // 触摸/拖拽滑动相关 (必须在 early return 之前调用)
  const touchStartX = useRef(0)
  const isDragging = useRef(false)
  const dragOffsetRef = useRef(0)
  const [dragOffset, setDragOffset] = useState(0)

  const handleTouchStart = useCallback((e: React.TouchEvent | React.MouseEvent) => {
    const clientX = 'touches' in e ? e.touches[0].clientX : e.clientX
    touchStartX.current = clientX
    isDragging.current = true
    dragOffsetRef.current = 0
    setDragOffset(0)
  }, [])

  const handleTouchMove = useCallback((e: React.TouchEvent | React.MouseEvent) => {
    if (!isDragging.current) return
    const clientX = 'touches' in e ? e.touches[0].clientX : e.clientX
    const offset = clientX - touchStartX.current
    dragOffsetRef.current = offset
    setDragOffset(offset)
  }, [])

  const handleTouchEnd = useCallback(() => {
    if (!isDragging.current) return
    isDragging.current = false
    const threshold = 50
    const delta = dragOffsetRef.current
    dragOffsetRef.current = 0
    setDragOffset(0)
    if (delta < -threshold) {
      setSelectedImage(prev => Math.min(prev + 1, (data?.data?.images?.length || 1) - 1))
    } else if (delta > threshold) {
      setSelectedImage(prev => Math.max(prev - 1, 0))
    }
  }, [data])

  // 缩略图栏鼠标拖拽横向滚动
  const thumbScrollRef = useRef<HTMLDivElement>(null)
  const thumbScrollRefDesktop = useRef<HTMLDivElement>(null)
  const thumbDragStartX = useRef(0)
  const thumbScrollLeft = useRef(0)
  const isThumbDragging = useRef(false)
  const thumbDragMoved = useRef(false)

  const handleThumbMouseDown = useCallback((e: React.MouseEvent, ref: React.RefObject<HTMLDivElement | null>) => {
    const el = ref.current
    if (!el) return
    isThumbDragging.current = true
    thumbDragMoved.current = false
    thumbDragStartX.current = e.clientX
    thumbScrollLeft.current = el.scrollLeft
    el.style.cursor = 'grabbing'
    el.style.scrollBehavior = 'auto'
  }, [])

  const handleThumbMouseMove = useCallback((e: React.MouseEvent, ref: React.RefObject<HTMLDivElement | null>) => {
    if (!isThumbDragging.current) return
    const el = ref.current
    if (!el) return
    const dx = e.clientX - thumbDragStartX.current
    if (Math.abs(dx) > 3) thumbDragMoved.current = true
    el.scrollLeft = thumbScrollLeft.current - dx
  }, [])

  const handleThumbMouseUp = useCallback((ref: React.RefObject<HTMLDivElement | null>) => {
    isThumbDragging.current = false
    const el = ref.current
    if (!el) return
    el.style.cursor = ''
    el.style.scrollBehavior = ''
  }, [])

  // 实时计算优惠码折扣（基于当前数量和单价）
  const subtotal = data?.data ? data.data.price * quantity : 0
  const promoDiscount = useMemo(() => {
    if (!appliedPromo || subtotal <= 0) return 0

    if (appliedPromo.discount_type === 'percentage') {
      let discount = subtotal * appliedPromo.discount_value / 100
      if (appliedPromo.max_discount > 0 && discount > appliedPromo.max_discount) {
        discount = appliedPromo.max_discount
      }
      return Math.min(discount, subtotal)
    } else {
      return Math.min(appliedPromo.discount_value, subtotal)
    }
  }, [appliedPromo, subtotal])

  if (isLoading) {
    return (
      <div className="pb-8">
        <div className="animate-pulse space-y-4">
          <div className="h-6 bg-muted rounded w-1/4" />
          <div className="flex gap-6">
            <div className="w-[400px] shrink-0 h-[400px] bg-muted rounded" />
            <div className="flex-1 space-y-3">
              <div className="h-6 bg-muted rounded" />
              <div className="h-4 bg-muted rounded w-3/4" />
              <div className="h-5 bg-muted rounded w-1/2" />
            </div>
          </div>
        </div>
      </div>
    )
  }

  if (!product) {
    return (
      <div className="space-y-6">
        <div className="text-center py-12">
          <Package className="w-16 h-16 text-muted-foreground mx-auto mb-4" />
          <p className="text-muted-foreground">{t.product.productNotFound}</p>
          <Button asChild className="mt-4">
            <Link href="/products">{t.product.backToProductList}</Link>
          </Button>
        </div>
      </div>
    )
  }

  const images = product.images || []
  const primaryImage = images.find((img: any) => img.is_primary || img.isPrimary)
  const displayImages = primaryImage
    ? [primaryImage, ...images.filter((img: any) => !(img.is_primary || img.isPrimary))]
    : images

  const isFeatured = product.is_featured || product.isFeatured
  const hasDiscount = product.original_price && product.original_price > product.price

  const availableStock = stockData?.data?.available_stock ?? 0
  const isAvailable = availableStock > 0

  // Stock display settings
  const stockDisplayMode = publicConfig?.data?.stock_display?.mode || 'exact'
  const lowThreshold = publicConfig?.data?.stock_display?.low_stock_threshold || 10
  const highThreshold = publicConfig?.data?.stock_display?.high_stock_threshold || 50

  // Calculate stock level
  const getStockLevel = () => {
    if (availableStock <= lowThreshold) return 'low'
    if (availableStock >= highThreshold) return 'high'
    return 'medium'
  }

  const stockLevel = getStockLevel()

  // Get stock display text
  const getStockDisplay = () => {
    if (!isAvailable) {
      return <Badge variant="destructive" className="text-xs">{t.product.outOfStock}</Badge>
    }

    if (stockDisplayMode === 'hidden') {
      return <Badge variant="default" className="text-xs">{t.product.inStock}</Badge>
    }

    if (stockDisplayMode === 'level') {
      const levelText = stockLevel === 'low' ? t.admin.stockLevelLow
        : stockLevel === 'high' ? t.admin.stockLevelHigh
        : t.admin.stockLevelMedium
      const variant = stockLevel === 'low' ? 'destructive' : 'default'
      return <Badge variant={variant} className="text-xs">{levelText}</Badge>
    }

    // exact mode
    return <Badge variant="default" className="text-xs">{availableStock} {t.product.piecesUnit}</Badge>
  }

  const selectableAttributes = (product.attributes || []).filter((attr: any) => attr.mode !== 'blind_box')
  const hasBlindBoxAttributes = (product.attributes || []).some((attr: any) => attr.mode === 'blind_box')

  const allAttributesSelected = selectableAttributes.length === 0 ||
    selectableAttributes.every((attr: any) => selectedAttributes[attr.name])

  const handleAttributeChange = (attrName: string, value: string) => {
    const newAttrs = {
      ...selectedAttributes,
      [attrName]: value
    }
    setSelectedAttributes(newAttrs)
    refetchStock()
  }

  const handleBuyNow = () => {
    if (!user) {
      toast.error(t.product.pleaseLoginFirst)
      setTimeout(() => router.push('/login'), 1000)
      return
    }

    if (!allAttributesSelected) {
      toast.error(t.product.pleaseSelectAllAttributes)
      return
    }

    createOrderMutation.mutate({
      items: [
        {
          sku: product.sku,
          name: product.name,
          quantity: quantity,
          image_url: product.images?.[0]?.url,
          attributes: selectedAttributes,
        },
      ],
      ...(appliedPromo ? { promo_code: appliedPromo.code } : {}),
    })
  }

  const handleAddToCart = async () => {
    if (!user) {
      toast.error(t.product.pleaseLoginFirst)
      setTimeout(() => router.push('/login'), 1000)
      return
    }

    if (!allAttributesSelected) {
      toast.error(t.product.pleaseSelectAllAttributes)
      return
    }

    setIsAddingToCart(true)
    try {
      await addToCart({
        product_id: productId,
        quantity: quantity,
        attributes: selectedAttributes,
      })
      queryClient.invalidateQueries({ queryKey: ['cart'] })
      toast.success(t.cart.addedToCart)
    } catch (error: any) {
      toast.error(error.message || t.cart.addFailed)
    } finally {
      setIsAddingToCart(false)
    }
  }

  const handleQuantityChange = (newQuantity: number) => {
    if (newQuantity >= 1 && newQuantity <= availableStock) {
      setQuantity(newQuantity)
    }
  }

  // 应用优惠码
  const handleApplyPromoCode = async () => {
    if (!promoCodeInput.trim()) return

    setIsValidatingPromo(true)
    try {
      const amount = product.price * quantity
      const response = await validatePromoCode({
        code: promoCodeInput.trim(),
        product_ids: [productId],
        amount,
      })

      const data = response.data
      setAppliedPromo({
        code: data.promo_code,
        promo_code_id: data.promo_code_id,
        name: data.name,
        discount_type: data.discount_type,
        discount_value: data.discount_value,
        max_discount: data.max_discount || 0,
        min_order_amount: data.min_order_amount || 0,
      })
      toast.success(
        t.promoCode.promoCodeApplied
          .replace('{code}', data.promo_code)
          .replace('{discount}', formatPrice(data.discount, currency))
      )
    } catch (error: any) {
      const msg = error?.message || t.promoCode.invalidCode
      toast.error(msg)
    } finally {
      setIsValidatingPromo(false)
    }
  }

  // 移除优惠码
  const handleRemovePromoCode = () => {
    setAppliedPromo(null)
    setPromoCodeInput('')
  }

  // 缩略图列表 memo 化，避免切换主图时重新渲染
  const thumbnailList: Array<{ url: string; alt: string }> = displayImages.map((image: any) => ({
    url: image.url,
    alt: image.alt || '',
  }))

  return (
    <div className="pb-8">
      {/* Header */}
      <div className="flex items-center gap-3 mb-6">
        <Button asChild variant="outline" size="sm">
          <Link href="/products">
            <ArrowLeft className="mr-1.5 h-4 w-4" />
            <span className="hidden md:inline">{t.product.backToList}</span>
          </Link>
        </Button>
        <h1 className="text-lg md:text-xl font-bold line-clamp-1">{t.product.productDetailTitle}</h1>
      </div>

      {/* Mobile image */}
      <div className="md:hidden mb-6">
        <div className="space-y-3">
          <div
            className="relative aspect-square rounded-xl overflow-hidden bg-muted touch-pan-y"
            onTouchStart={handleTouchStart}
            onTouchMove={handleTouchMove}
            onTouchEnd={handleTouchEnd}
          >
            {isFeatured && (
              <Badge className="absolute top-3 right-3 z-10 bg-yellow-500 shadow-lg">
                <Star className="w-3 h-3 mr-1" />
                {t.product.featured}
              </Badge>
            )}
            {hasDiscount && (
              <Badge variant="destructive" className="absolute top-3 left-3 z-10 shadow-lg">
                SALE
              </Badge>
            )}
            {displayImages.length > 0 ? (
              <div
                className="flex h-full"
                style={{
                  width: `${displayImages.length * 100}%`,
                  transform: `translateX(calc(-${selectedImage * (100 / displayImages.length)}% + ${dragOffset}px))`,
                  transition: isDragging.current ? 'none' : 'transform 0.3s ease-out',
                }}
              >
                {displayImages.map((image: any, index: number) => (
                  <img
                    key={index}
                    src={image.url}
                    alt={product.name}
                    className="w-full h-full object-cover shrink-0 pointer-events-none select-none"
                    draggable={false}
                    style={{ width: `${100 / displayImages.length}%` }}
                    onError={(e) => {
                      e.currentTarget.src = 'data:image/svg+xml,' + encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" width="80" height="80" viewBox="0 0 24 24" fill="none" stroke="%23999" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4"/></svg>')
                    }}
                  />
                ))}
              </div>
            ) : (
              <div className="w-full h-full flex items-center justify-center">
                <Package className="w-20 h-20 text-muted-foreground/50" />
              </div>
            )}
            {/* 图片指示器 */}
            {displayImages.length > 1 && (
              <div className="absolute bottom-3 left-1/2 -translate-x-1/2 flex gap-1.5">
                {displayImages.map((_: any, index: number) => (
                  <span
                    key={index}
                    className={`block w-1.5 h-1.5 rounded-full transition-all ${selectedImage === index ? 'bg-white w-3' : 'bg-white/50'}`}
                  />
                ))}
              </div>
            )}
          </div>
          {displayImages.length > 1 && (
            <div
              ref={thumbScrollRef}
              className="flex gap-2 overflow-x-auto pb-2 cursor-grab active:cursor-grabbing select-none"
              onMouseDown={(e) => handleThumbMouseDown(e, thumbScrollRef)}
              onMouseMove={(e) => handleThumbMouseMove(e, thumbScrollRef)}
              onMouseUp={() => handleThumbMouseUp(thumbScrollRef)}
              onMouseLeave={() => handleThumbMouseUp(thumbScrollRef)}
            >
              {thumbnailList.map((image, index: number) => (
                <button
                  key={index}
                  onClick={() => { if (!thumbDragMoved.current) setSelectedImage(index) }}
                  className={`aspect-square w-16 shrink-0 rounded-lg overflow-hidden border-2 transition-all ${selectedImage === index ? 'border-primary ring-1 ring-primary' : 'border-border hover:border-muted-foreground'}`}
                >
                  <img
                    src={image.url}
                    alt={image.alt || product.name}
                    className="w-full h-full object-cover pointer-events-none"
                    onError={(e) => {
                      e.currentTarget.src = 'data:image/svg+xml,' + encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="%23999" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4"/></svg>')
                    }}
                  />
                </button>
              ))}
            </div>
          )}
        </div>
      </div>

      <div className="flex gap-8">
        {/* Left: Image gallery (sticky) */}
        <div className="w-[520px] shrink-0 sticky top-4 self-start hidden md:block">
          <div className="space-y-3">
            <div
              className="relative aspect-square rounded-xl overflow-hidden bg-muted border border-border cursor-grab active:cursor-grabbing select-none"
              onMouseDown={handleTouchStart}
              onMouseMove={handleTouchMove}
              onMouseUp={handleTouchEnd}
              onMouseLeave={() => { if (isDragging.current) handleTouchEnd() }}
            >
              {isFeatured && (
                <Badge className="absolute top-3 right-3 z-10 bg-yellow-500 shadow-lg">
                  <Star className="w-3 h-3 mr-1" />
                  {t.product.featured}
                </Badge>
              )}
              {hasDiscount && (
                <Badge variant="destructive" className="absolute top-3 left-3 z-10 shadow-lg">
                  SALE
                </Badge>
              )}
              {displayImages.length > 0 ? (
                <div
                  className="flex h-full"
                  style={{
                    width: `${displayImages.length * 100}%`,
                    transform: `translateX(calc(-${selectedImage * (100 / displayImages.length)}% + ${dragOffset}px))`,
                    transition: isDragging.current ? 'none' : 'transform 0.3s ease-out',
                  }}
                >
                  {displayImages.map((image: any, index: number) => (
                    <img
                      key={index}
                      src={image.url}
                      alt={product.name}
                      className="w-full h-full object-cover shrink-0 pointer-events-none select-none"
                      draggable={false}
                      style={{ width: `${100 / displayImages.length}%` }}
                      onError={(e) => {
                        e.currentTarget.src = 'data:image/svg+xml,' + encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" width="80" height="80" viewBox="0 0 24 24" fill="none" stroke="%23999" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4"/></svg>')
                      }}
                    />
                  ))}
                </div>
              ) : (
                <div className="w-full h-full flex items-center justify-center">
                  <Package className="w-20 h-20 text-muted-foreground/50" />
                </div>
              )}
            </div>
            {displayImages.length > 1 && (
              <div
                ref={thumbScrollRefDesktop}
                className="flex gap-2 overflow-x-auto pb-2 cursor-grab active:cursor-grabbing select-none"
                onMouseDown={(e) => handleThumbMouseDown(e, thumbScrollRefDesktop)}
                onMouseMove={(e) => handleThumbMouseMove(e, thumbScrollRefDesktop)}
                onMouseUp={() => handleThumbMouseUp(thumbScrollRefDesktop)}
                onMouseLeave={() => handleThumbMouseUp(thumbScrollRefDesktop)}
              >
                {thumbnailList.map((image, index: number) => (
                  <button
                    key={index}
                    onClick={() => { if (!thumbDragMoved.current) setSelectedImage(index) }}
                    className={`aspect-square w-20 shrink-0 rounded-lg overflow-hidden border-2 transition-all ${selectedImage === index ? 'border-primary ring-1 ring-primary' : 'border-border hover:border-muted-foreground'}`}
                  >
                    <img
                      src={image.url}
                      alt={image.alt || product.name}
                      className="w-full h-full object-cover pointer-events-none"
                      onError={(e) => {
                        e.currentTarget.src = 'data:image/svg+xml,' + encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="%23999" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4"/></svg>')
                      }}
                    />
                  </button>
                ))}
              </div>
            )}
          </div>
        </div>

        {/* Right: Product info */}
        <div className="flex-1 min-w-0 space-y-4">
          {/* Title + stats */}
          <div>
            <h2 className="text-2xl font-bold leading-tight mb-2">{product.name}</h2>
            <div className="flex items-center gap-4 text-sm text-muted-foreground">
              <span className="flex items-center gap-1.5">
                <Eye className="w-3.5 h-3.5" />
                {product.view_count || 0} {t.product.views}
              </span>
              <span className="flex items-center gap-1.5">
                <ShoppingCart className="w-3.5 h-3.5" />
                {product.sale_count || 0} {t.product.sales}
              </span>
            </div>
          </div>

          {/* Price card */}
          <div className="rounded-xl bg-muted/50 border border-border p-4 space-y-2">
            <div className="flex flex-col sm:flex-row sm:items-baseline gap-1 sm:gap-3">
              <span className="text-3xl font-bold text-red-500">
                {formatPrice(product.price, currency)}
              </span>
              {hasDiscount && (
                <div className="flex items-center gap-2">
                  <span className="text-base text-muted-foreground line-through">
                    {formatPrice(product.original_price!, currency)}
                  </span>
                  <Badge variant="destructive" className="text-xs">
                    {t.product.save} {getCurrencySymbol(currency)}{(product.original_price! - product.price).toFixed(2)}
                  </Badge>
                </div>
              )}
            </div>
            {appliedPromo && promoDiscount > 0 && (
              <div className="flex items-baseline gap-3 pt-1 border-t border-border/50 mt-2">
                <div className="flex items-center gap-2">
                  <Tag className="h-3.5 w-3.5 text-green-600 dark:text-green-400" />
                  <span className="text-sm text-muted-foreground line-through">
                    {formatPrice(subtotal, currency)}
                  </span>
                </div>
                <span className="text-2xl font-bold text-green-600 dark:text-green-400">
                  {formatPrice(subtotal - promoDiscount, currency)}
                </span>
                <Badge variant="outline" className="text-xs text-green-600 dark:text-green-400 border-green-500/30">
                  -{appliedPromo.discount_type === 'percentage'
                    ? `${appliedPromo.discount_value}%`
                    : formatPrice(appliedPromo.discount_value, currency)}
                </Badge>
              </div>
            )}
            <div className="text-sm text-muted-foreground">
              SKU: {product.sku}
            </div>
          </div>

          {/* Virtual product notice */}
          {isVirtual && (
            <div className="rounded-xl bg-purple-500/10 border border-purple-500/20 p-4">
              <div className="flex items-start gap-3">
                <Key className="w-5 h-5 text-purple-500 mt-0.5 shrink-0" />
                <div className="text-sm text-purple-700 dark:text-purple-300 leading-relaxed">
                  {product.auto_delivery || product.autoDelivery
                    ? t.product.virtualProductNoticeInstant
                    : t.product.virtualProductNoticeManual}
                </div>
              </div>
            </div>
          )}

          {/* Product meta */}
          <div className="space-y-3">
            {stockDisplayMode !== 'hidden' && (
              <div className="flex items-center justify-between py-2 border-b border-border">
                <span className="text-sm text-muted-foreground">{t.product.stockLabel}:</span>
                {getStockDisplay()}
              </div>
            )}
            {product.category && (
              <div className="flex items-center justify-between py-2 border-b border-border">
                <span className="text-sm text-muted-foreground">{t.product.categoryLabel}:</span>
                <Badge variant="secondary" className="text-xs">{product.category}</Badge>
              </div>
            )}
            {product.tags && product.tags.length > 0 && (
              <div className="flex items-center justify-between py-2 border-b border-border">
                <span className="text-sm text-muted-foreground shrink-0">{t.product.tagsLabel}:</span>
                <div className="flex flex-wrap justify-end gap-1.5">
                  {product.tags.map((tag: string) => (
                    <Badge key={tag} variant="secondary" className="text-xs">{tag}</Badge>
                  ))}
                </div>
              </div>
            )}
            {product.max_purchase_limit && product.max_purchase_limit > 0 && (
              <div className="flex items-center justify-between py-2 border-b border-border">
                <span className="text-sm text-muted-foreground">{t.product.purchaseLimitLabel}:</span>
                <Badge variant="outline" className="text-xs text-orange-600 dark:text-orange-400 border-orange-300 dark:border-orange-700">
                  {t.product.maxPurchaseLimit} {product.max_purchase_limit} {t.product.piecesUnit}
                </Badge>
              </div>
            )}
          </div>

          {/* Product description */}
          {product.description && (
            <div className="rounded-xl border border-border p-4">
              <h3 className="font-semibold mb-3">{t.product.productDetailTitle}</h3>
              <MarkdownMessage content={product.description} className="text-sm text-muted-foreground leading-relaxed" allowHtml />
            </div>
          )}

          {/* Specs selection */}
          {(selectableAttributes.length > 0 || hasBlindBoxAttributes) && (
            <div className="rounded-xl border border-border p-4 space-y-4">
              {/* Blind box */}
              {hasBlindBoxAttributes && (
                <div className="bg-purple-500/10 rounded-lg p-3">
                  <div className="flex items-start gap-2">
                    <div className="text-xl">🎲</div>
                    <div className="flex-1">
                      <div className="font-medium text-purple-600 dark:text-purple-400 mb-1 text-sm">{t.product.blindBoxAttribute}</div>
                      <div className="text-xs text-purple-600/80 dark:text-purple-400/80">
                        {(product.attributes || [])
                          .filter((attr: any) => attr.mode === 'blind_box')
                          .map((attr: any) => (
                            <div key={attr.name} className="mb-1">
                              <span className="font-medium">{attr.name}</span>: {attr.values.join('、')}
                            </div>
                          ))}
                        <div className="mt-2 text-purple-500 dark:text-purple-400">
                          {t.product.blindBoxRandomTip}
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              )}

              {/* Selectable specs */}
              {selectableAttributes.length > 0 && (
                <div className="space-y-4">
                  <div className="text-sm font-semibold">
                    {t.product.selectSpec} <span className="text-destructive">*</span>
                  </div>
                  {selectableAttributes.map((attr: any, index: number) => (
                    <div key={index} className="space-y-2">
                      <div className="text-sm text-muted-foreground">{attr.name}:</div>
                      <div className="flex flex-wrap gap-2">
                        {attr.values.map((value: string) => {
                          const isSelected = selectedAttributes[attr.name] === value
                          return (
                            <button
                              key={value}
                              onClick={() => handleAttributeChange(attr.name, value)}
                              className={`px-4 py-1.5 text-sm rounded-lg border transition-all ${isSelected
                                ? 'border-primary bg-primary text-primary-foreground font-medium'
                                : 'border-border hover:border-primary/50 text-foreground'
                                }`}
                            >
                              {value}
                            </button>
                          )
                        })}
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}

          {/* Quantity */}
          <div className="flex items-center gap-4">
            <span className="text-sm text-muted-foreground">
              {t.product.quantity}:
            </span>
            <div className="flex items-center">
              <Button
                variant="outline"
                size="icon"
                className="h-9 w-9 rounded-r-none"
                onClick={() => handleQuantityChange(quantity - 1)}
                disabled={quantity <= 1}
              >
                <Minus className="h-3.5 w-3.5" />
              </Button>
              <Input
                type="number"
                value={quantity}
                onChange={(e) => {
                  const val = parseInt(e.target.value)
                  if (!isNaN(val)) {
                    handleQuantityChange(val)
                  }
                }}
                className="w-16 h-9 text-center rounded-none border-x-0 focus-visible:ring-0 focus-visible:ring-offset-0"
                min={1}
                max={availableStock}
              />
              <Button
                variant="outline"
                size="icon"
                className="h-9 w-9 rounded-l-none"
                onClick={() => handleQuantityChange(quantity + 1)}
                disabled={quantity >= availableStock}
              >
                <Plus className="h-3.5 w-3.5" />
              </Button>
            </div>
            <span className="text-xs text-muted-foreground">
              ({t.product.stockLabel}: {availableStock})
            </span>
          </div>

          {!allAttributesSelected && product.attributes && product.attributes.length > 0 && (
            <p className="text-xs text-amber-600 dark:text-amber-400">
              {t.product.pleaseSelectAllSpec}
            </p>
          )}

          {/* 优惠码区域 */}
          <div className="rounded-xl border border-border p-4 space-y-3">
            <div className="flex items-center gap-2 text-sm font-medium">
              <Tag className="h-4 w-4" />
              {t.promoCode.enterPromoCode}
            </div>
            {!appliedPromo ? (
              <div className="flex gap-2">
                <Input
                  value={promoCodeInput}
                  onChange={(e) => setPromoCodeInput(e.target.value)}
                  placeholder={t.promoCode.promoCodePlaceholder}
                  className="flex-1"
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') handleApplyPromoCode()
                  }}
                />
                <Button
                  onClick={handleApplyPromoCode}
                  disabled={!promoCodeInput.trim() || isValidatingPromo}
                  size="default"
                >
                  {isValidatingPromo ? t.promoCode.applying : t.promoCode.apply}
                </Button>
              </div>
            ) : (
              <div className="space-y-2">
                <div className="flex items-center justify-between rounded-lg bg-green-500/10 dark:bg-green-500/20 border border-green-500/20 dark:border-green-500/30 p-3">
                  <div>
                    <div className="text-sm font-medium text-green-700 dark:text-green-400">
                      {appliedPromo.name}
                    </div>
                    <div className="text-xs text-green-600 dark:text-green-500 mt-0.5">
                      {t.promoCode.applied} &mdash; {appliedPromo.code}
                    </div>
                  </div>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="text-red-500 hover:text-red-600 hover:bg-red-500/10"
                    onClick={handleRemovePromoCode}
                  >
                    {t.promoCode.remove}
                  </Button>
                </div>
                <div className="flex items-center justify-between text-sm">
                  <span className="text-muted-foreground">{t.promoCode.discount}</span>
                  <span className="text-green-600 dark:text-green-400 font-medium">
                    -{formatPrice(promoDiscount, currency)}
                  </span>
                </div>
              </div>
            )}
          </div>

          {/* Action buttons */}
          <div className="flex gap-3 pt-2">
            <Button
              variant="outline"
              className="flex-1 min-w-0 h-11"
              disabled={!isAvailable || !allAttributesSelected || isAddingToCart}
              onClick={handleAddToCart}
            >
              <span className="truncate inline-flex items-center">
                {isAddingToCart ? (
                  <>
                    <Loader2 className="mr-2 h-4 w-4 animate-spin shrink-0" />
                    {t.product.addingToCart}
                  </>
                ) : (
                  <>
                    <ShoppingCart className="mr-2 h-4 w-4 shrink-0" />
                    {t.product.addToCart}
                  </>
                )}
              </span>
            </Button>
            <Button
              className="flex-1 min-w-0 h-11"
              disabled={!isAvailable || !allAttributesSelected || createOrderMutation.isPending}
              onClick={handleBuyNow}
            >
              <span className="truncate inline-flex items-center">
                {createOrderMutation.isPending ? (
                  <>
                    <Loader2 className="mr-2 h-4 w-4 animate-spin shrink-0" />
                    {t.product.creatingOrder}
                  </>
                ) : !isAvailable ? (
                  t.product.soldOut
                ) : !allAttributesSelected ? (
                  t.product.pleaseSelectSpec
                ) : (
                  t.product.buyNow
                )}
              </span>
            </Button>
          </div>
        </div>
      </div>
    </div>
  )
}
