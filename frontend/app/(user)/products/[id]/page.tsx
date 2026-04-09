'use client'
/* eslint-disable @next/next/no-img-element */

import { useParams, useRouter } from 'next/navigation'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  getProduct,
  createOrder,
  getProductAvailableStock,
  addToCart,
  validatePromoCode,
  getPublicConfig,
} from '@/lib/api'
import { resolveApiErrorMessage } from '@/lib/api-error'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import {
  Star,
  Package,
  Eye,
  ShoppingCart,
  Loader2,
  ArrowLeft,
  Key,
  Minus,
  Plus,
  Tag,
  AlertCircle,
} from 'lucide-react'
import { useState, useRef, useCallback, useMemo, useEffect } from 'react'
import { Input } from '@/components/ui/input'
import Link from 'next/link'
import { useToast } from '@/hooks/use-toast'
import { useAuth } from '@/hooks/use-auth'
import { useLocale } from '@/hooks/use-locale'
import { usePageTitle } from '@/hooks/use-page-title'
import { getTranslations } from '@/lib/i18n'
import { useCurrency, formatPrice, getCurrencySymbol } from '@/contexts/currency-context'
import { addToGuestCart } from '@/lib/guest-cart'
import {
  clearAuthReturnState,
  readAuthReturnState,
  setAuthReturnState,
} from '@/lib/auth-return-state'
import { buildProductListReturnPath, readProductBrowseState } from '@/lib/product-browse-state'
import { MarkdownMessage } from '@/components/ui/markdown-message'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { PluginSlot } from '@/components/plugins/plugin-slot'
import { useIsMobile } from '@/hooks/use-mobile'
import { cn } from '@/lib/utils'

type GuestActionHint = 'cart_added' | 'login_for_checkout' | 'login_for_promo' | null

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
  const [productListBackHref, setProductListBackHref] = useState('/products')
  const [guestActionHint, setGuestActionHint] = useState<GuestActionHint>(null)
  const hasRestoredAuthReturnStateRef = useRef(false)
  const toast = useToast()
  const { user, isAuthenticated, isLoading: authLoading } = useAuth()
  const { currency } = useCurrency()
  const { isMobile, mounted: mobileMounted } = useIsMobile()

  // Promo code state
  const [promoCodeInput, setPromoCodeInput] = useState('')
  const [isValidatingPromo, setIsValidatingPromo] = useState(false)
  const [appliedPromo, setAppliedPromo] = useState<{
    code: string
    promo_code_id: number
    name: string
    discount_type: string
    discount_value_minor: number
    max_discount_minor: number
    min_order_amount_minor: number
  } | null>(null)

  const {
    data,
    isLoading,
    error: productError,
    refetch: refetchProduct,
  } = useQuery({
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
  const maxItemQuantity = publicConfig?.data?.max_item_quantity || 9999

  const createOrderMutation = useMutation({
    mutationFn: createOrder,
    onSuccess: (response) => {
      const orderNo = response.data?.order_no
      toast.success(orderNo ? `${t.product.orderCreated} ${orderNo}` : t.product.orderCreated)
      queryClient.invalidateQueries({ queryKey: ['product', productId] })
      queryClient.invalidateQueries({ queryKey: ['products'] })
      queryClient.invalidateQueries({ queryKey: ['productStock', productId] })
      queryClient.invalidateQueries({ queryKey: ['orders'] })
      router.push(orderNo ? `/orders/${orderNo}` : '/orders')
    },
    onError: (error: any) => {
      toast.error(resolveApiErrorMessage(error, t, t.product.orderCreateFailed))
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
      setSelectedImage((prev) => Math.min(prev + 1, (data?.data?.images?.length || 1) - 1))
    } else if (delta > threshold) {
      setSelectedImage((prev) => Math.max(prev - 1, 0))
    }
  }, [data])

  // 缩略图栏鼠标拖拽横向滚动
  const thumbScrollRef = useRef<HTMLDivElement>(null)
  const thumbScrollRefDesktop = useRef<HTMLDivElement>(null)
  const thumbDragStartX = useRef(0)
  const thumbScrollLeft = useRef(0)
  const isThumbDragging = useRef(false)
  const thumbDragMoved = useRef(false)

  const handleThumbMouseDown = useCallback(
    (e: React.MouseEvent, ref: React.RefObject<HTMLDivElement | null>) => {
      const el = ref.current
      if (!el) return
      isThumbDragging.current = true
      thumbDragMoved.current = false
      thumbDragStartX.current = e.clientX
      thumbScrollLeft.current = el.scrollLeft
      el.style.cursor = 'grabbing'
      el.style.scrollBehavior = 'auto'
    },
    []
  )

  const handleThumbMouseMove = useCallback(
    (e: React.MouseEvent, ref: React.RefObject<HTMLDivElement | null>) => {
      if (!isThumbDragging.current) return
      const el = ref.current
      if (!el) return
      const dx = e.clientX - thumbDragStartX.current
      if (Math.abs(dx) > 3) thumbDragMoved.current = true
      el.scrollLeft = thumbScrollLeft.current - dx
    },
    []
  )

  const handleThumbMouseUp = useCallback((ref: React.RefObject<HTMLDivElement | null>) => {
    isThumbDragging.current = false
    const el = ref.current
    if (!el) return
    el.style.cursor = ''
    el.style.scrollBehavior = ''
  }, [])

  const getProductScrollTop = useCallback(() => {
    if (typeof document === 'undefined' || typeof window === 'undefined') {
      return 0
    }
    const mainElement = document.querySelector('main')
    if (mainElement instanceof HTMLElement) {
      return Math.max(0, mainElement.scrollTop)
    }
    return Math.max(0, window.scrollY)
  }, [])

  const restoreProductScrollTop = useCallback((scrollTop: number) => {
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

  const redirectToLoginWithProductReturn = useCallback(() => {
    if (!Number.isFinite(productId) || productId <= 0) {
      router.push('/login')
      return
    }

    setAuthReturnState({
      redirectPath: `/products/${productId}`,
      product: {
        productId,
        selectedAttributes,
        quantity,
        scrollTop: getProductScrollTop(),
      },
    })
    router.push('/login')
  }, [getProductScrollTop, productId, quantity, router, selectedAttributes])

  useEffect(() => {
    if (user) {
      setGuestActionHint(null)
    }
  }, [user])

  useEffect(() => {
    if (!Number.isFinite(productId) || productId <= 0) {
      setProductListBackHref('/products')
      return
    }

    const browseState = readProductBrowseState()
    setProductListBackHref(buildProductListReturnPath(browseState?.listPath, productId))
  }, [productId])

  // 实时计算优惠码折扣（基于当前数量和单价）
  const subtotal = data?.data ? (data.data.price_minor || 0) * quantity : 0
  const promoDiscount = useMemo(() => {
    if (!appliedPromo || subtotal <= 0) return 0

    if (appliedPromo.discount_type === 'percentage') {
      let discount = (subtotal * appliedPromo.discount_value_minor) / 10000
      if (appliedPromo.max_discount_minor > 0 && discount > appliedPromo.max_discount_minor) {
        discount = appliedPromo.max_discount_minor
      }
      return Math.min(discount, subtotal)
    } else {
      return Math.min(appliedPromo.discount_value_minor, subtotal)
    }
  }, [appliedPromo, subtotal])

  const images = product?.images || []
  const primaryImage = images.find((img: any) => img.is_primary || img.isPrimary)
  const displayImages = primaryImage
    ? [primaryImage, ...images.filter((img: any) => !(img.is_primary || img.isPrimary))]
    : images

  const isFeatured = Boolean(product?.is_featured || product?.isFeatured)
  const hasDiscount = Number(product?.original_price_minor || 0) > Number(product?.price_minor || 0)

  const availableStock = stockData?.data?.available_stock ?? 0
  const isUnlimitedStock = !!stockData?.data?.is_unlimited
  const isAvailable = availableStock > 0
  const isGuestMode = !authLoading && !isAuthenticated
  const productMaxPurchaseLimit = product?.max_purchase_limit ?? product?.maxPurchaseLimit ?? 0
  const maxSelectableQuantity = Math.min(
    availableStock,
    maxItemQuantity,
    productMaxPurchaseLimit > 0 ? productMaxPurchaseLimit : Number.MAX_SAFE_INTEGER
  )

  useEffect(() => {
    if (maxSelectableQuantity > 0 && quantity > maxSelectableQuantity) {
      setQuantity(maxSelectableQuantity)
    }
  }, [maxSelectableQuantity, quantity])

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
      return (
        <Badge variant="destructive" className="text-xs">
          {t.product.outOfStock}
        </Badge>
      )
    }

    if (stockDisplayMode === 'hidden') {
      return (
        <Badge variant="default" className="text-xs">
          {t.product.inStock}
        </Badge>
      )
    }

    if (stockDisplayMode === 'level') {
      const levelText =
        stockLevel === 'low'
          ? t.admin.stockLevelLow
          : stockLevel === 'high'
            ? t.admin.stockLevelHigh
            : t.admin.stockLevelMedium
      const variant = stockLevel === 'low' ? 'destructive' : 'default'
      return (
        <Badge variant={variant} className="text-xs">
          {levelText}
        </Badge>
      )
    }

    // exact mode
    return (
      <Badge variant="default" className="text-xs">
        {availableStock} {t.product.piecesUnit}
      </Badge>
    )
  }

  const selectableAttributes = (product?.attributes || []).filter(
    (attr: any) => attr.mode !== 'blind_box'
  )
  const hasBlindBoxAttributes = (product?.attributes || []).some(
    (attr: any) => attr.mode === 'blind_box'
  )

  const allAttributesSelected =
    selectableAttributes.length === 0 ||
    selectableAttributes.every((attr: any) => selectedAttributes[attr.name])
  const joinSummaryItems = (items: Array<string | null | undefined | false>) =>
    items
      .filter((item): item is string => typeof item === 'string' && item.trim() !== '')
      .join(' · ')
  const productOverviewSummary = joinSummaryItems([
    `${product?.view_count || 0} ${t.product.views}`,
    `${product?.sale_count || 0} ${t.product.sales}`,
  ])
  const discountAmount = (
    ((product?.original_price_minor || 0) - (product?.price_minor || 0)) /
    100
  ).toFixed(2)
  const promoDiscountLabel =
    appliedPromo && promoDiscount > 0
      ? appliedPromo.discount_type === 'percentage'
        ? `${(appliedPromo.discount_value_minor / 100).toFixed(2)}%`
        : formatPrice(appliedPromo.discount_value_minor, currency)
      : null

  useEffect(() => {
    if (
      authLoading ||
      !isAuthenticated ||
      isLoading ||
      hasRestoredAuthReturnStateRef.current ||
      !Number.isFinite(productId) ||
      productId <= 0
    ) {
      return
    }

    const pendingReturnState = readAuthReturnState()
    if (
      !pendingReturnState ||
      pendingReturnState.redirectPath !== `/products/${productId}` ||
      pendingReturnState.product?.productId !== productId
    ) {
      return
    }

    hasRestoredAuthReturnStateRef.current = true
    setSelectedAttributes(pendingReturnState.product.selectedAttributes || {})
    setQuantity(Math.max(1, pendingReturnState.product.quantity || 1))
    restoreProductScrollTop(pendingReturnState.product.scrollTop || 0)
    clearAuthReturnState()
  }, [authLoading, isAuthenticated, isLoading, productId, restoreProductScrollTop])

  if (isLoading || !mobileMounted) {
    return (
      <div className="pb-8">
        <div className="animate-pulse space-y-4">
          <div className="h-6 w-1/4 rounded bg-muted" />
          <div className={cn('gap-6', isMobile ? 'space-y-4' : 'flex')}>
            {!isMobile ? <div className="h-[400px] w-[400px] shrink-0 rounded bg-muted" /> : null}
            <div className="flex-1 space-y-3">
              {isMobile ? <div className="aspect-square w-full rounded bg-muted" /> : null}
              <div className="h-6 rounded bg-muted" />
              <div className="h-4 w-3/4 rounded bg-muted" />
              <div className="h-5 w-1/2 rounded bg-muted" />
            </div>
          </div>
        </div>
      </div>
    )
  }

  if (productError && !product) {
    return (
      <Card className="border-dashed bg-muted/15">
        <CardContent className="py-12 text-center">
          <AlertCircle className="mx-auto mb-4 h-16 w-16 text-muted-foreground" />
          <p className="text-base font-medium">{t.product.productLoadFailedTitle}</p>
          <p className="mt-2 text-sm text-muted-foreground">{t.product.productLoadFailedDesc}</p>
          <div className="mt-4 flex flex-col justify-center gap-3 sm:flex-row">
            <Button variant="outline" onClick={() => void refetchProduct()}>
              {t.common.refresh}
            </Button>
            <Button asChild>
              <Link href="/products">{t.product.backToProductList}</Link>
            </Button>
          </div>
          <PluginSlot
            slot="user.product_detail.load_failed"
            context={{
              view: 'user_product_detail',
              product: {
                id: Number.isFinite(productId) ? productId : undefined,
              },
              state: {
                load_failed: true,
              },
            }}
          />
        </CardContent>
      </Card>
    )
  }

  if (!product) {
    return (
      <Card className="border-dashed bg-muted/15">
        <CardContent className="py-12 text-center">
          <Package className="mx-auto mb-4 h-16 w-16 text-muted-foreground" />
          <p className="text-base font-medium">{t.product.productNotFound}</p>
          <p className="mt-2 text-sm text-muted-foreground">{t.product.productNotFoundDesc}</p>
          <Button asChild className="mt-4">
            <Link href="/products">{t.product.backToProductList}</Link>
          </Button>
          <PluginSlot
            slot="user.product_detail.not_found"
            context={{
              view: 'user_product_detail',
              product: {
                id: Number.isFinite(productId) ? productId : undefined,
              },
              state: {
                not_found: true,
              },
            }}
          />
        </CardContent>
      </Card>
    )
  }

  const handleAttributeChange = (attrName: string, value: string) => {
    const newAttrs = {
      ...selectedAttributes,
      [attrName]: value,
    }
    setSelectedAttributes(newAttrs)
    refetchStock()
  }

  const handleBuyNow = () => {
    if (authLoading) {
      return
    }

    if (!isAuthenticated) {
      setGuestActionHint('login_for_checkout')
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
    if (authLoading) {
      return
    }

    if (!allAttributesSelected) {
      toast.error(t.product.pleaseSelectAllAttributes)
      return
    }

    if (!isAuthenticated) {
      addToGuestCart(
        {
          product_id: productId,
          quantity: quantity,
          attributes: selectedAttributes,
        },
        maxItemQuantity
      )
      setGuestActionHint('cart_added')
      toast.success(t.product.guestCartAdded)
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
      queryClient.invalidateQueries({ queryKey: ['cartCount'] })
      setGuestActionHint(null)
      toast.success(t.cart.addedToCart)
    } catch (error: any) {
      toast.error(resolveApiErrorMessage(error, t, t.cart.addFailed))
    } finally {
      setIsAddingToCart(false)
    }
  }

  const handleQuantityChange = (newQuantity: number) => {
    if (newQuantity >= 1 && newQuantity <= maxSelectableQuantity) {
      setQuantity(newQuantity)
    }
  }

  // 应用优惠码
  const handleApplyPromoCode = async () => {
    if (authLoading) {
      return
    }

    if (!isAuthenticated) {
      setGuestActionHint('login_for_promo')
      return
    }

    if (!promoCodeInput.trim()) return

    setIsValidatingPromo(true)
    try {
      const amountMinor = (product.price_minor || 0) * quantity
      const response = await validatePromoCode({
        code: promoCodeInput.trim(),
        product_ids: [productId],
        amount_minor: amountMinor,
      })

      const data = response.data
      setAppliedPromo({
        code: data.promo_code,
        promo_code_id: data.promo_code_id,
        name: data.name,
        discount_type: data.discount_type,
        discount_value_minor: data.discount_value_minor,
        max_discount_minor: data.max_discount_minor || 0,
        min_order_amount_minor: data.min_order_amount_minor || 0,
      })
      toast.success(
        t.promoCode.promoCodeApplied
          .replace('{code}', data.promo_code)
          .replace('{discount}', formatPrice(data.discount_minor, currency))
      )
    } catch (error: any) {
      toast.error(resolveApiErrorMessage(error, t, t.promoCode.invalidCode))
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
  const guestActionTitle =
    guestActionHint === 'cart_added' ? t.product.guestCartAdded : t.auth.loginRequiredTitle
  const guestActionDescription =
    guestActionHint === 'cart_added'
      ? t.product.guestCartAddedHint
      : guestActionHint === 'login_for_promo'
        ? t.product.loginForPromoCode
        : t.product.pleaseLoginFirst
  const showGuestPurchaseHint = isGuestMode && !guestActionHint
  const userProductDetailPluginContext = {
    view: 'user_product_detail',
    product: {
      id: product.id || productId,
      name: product.name,
      sku: product.sku,
      category: product.category || undefined,
      status: product.status,
      product_type: product.product_type || product.productType,
      is_featured: Boolean(isFeatured),
      price_minor: product.price_minor,
      original_price_minor: product.original_price_minor,
      tag_count: Array.isArray(product.tags) ? product.tags.length : 0,
    },
    pricing: {
      currency,
      subtotal_minor: subtotal,
      promo_discount_minor: promoDiscount,
      final_amount_minor: subtotal - promoDiscount,
      has_discount: hasDiscount,
      promo_code: appliedPromo?.code || undefined,
    },
    selection: {
      quantity,
      selected_attributes: selectedAttributes,
      all_attributes_selected: allAttributesSelected,
      max_selectable_quantity: maxSelectableQuantity,
    },
    summary: {
      image_count: displayImages.length,
      available_stock: isUnlimitedStock ? undefined : availableStock,
      stock_display_mode: stockDisplayMode,
      is_available: isAvailable,
      is_virtual: Boolean(isVirtual),
      has_description: Boolean(product.description),
      show_guest_purchase_hint: showGuestPurchaseHint,
    },
    auth: {
      is_authenticated: isAuthenticated,
      guest_action_hint: guestActionHint || undefined,
    },
    state: {
      out_of_stock: !isAvailable,
      all_attributes_selected: allAttributesSelected,
      has_selectable_attributes: selectableAttributes.length > 0,
      has_category: Boolean(product.category),
      has_tags: Boolean(product.tags && product.tags.length > 0),
      has_discount: hasDiscount,
      promo_applied: Boolean(appliedPromo),
      adding_to_cart: isAddingToCart,
      creating_order: createOrderMutation.isPending,
      guest_purchase_hint_visible: showGuestPurchaseHint,
      guest_hint_visible: isGuestMode && Boolean(guestActionHint),
    },
  }

  return (
    <div className="pb-8">
      <PluginSlot slot="user.product_detail.top" context={userProductDetailPluginContext} />
      {/* Header */}
      <div className="mb-6 flex items-center gap-3">
        <Button asChild variant="outline" size="sm">
          <Link href={productListBackHref}>
            <ArrowLeft className={cn('h-4 w-4', !isMobile && 'md:mr-1.5')} />
            {isMobile ? (
              <span className="sr-only">{t.product.backToList}</span>
            ) : (
              <span>{t.product.backToList}</span>
            )}
          </Link>
        </Button>
        <h1 className={cn('line-clamp-1 text-lg font-bold', !isMobile && 'md:text-xl')}>
          {t.product.productDetailTitle}
        </h1>
      </div>

      {/* Mobile image */}
      {isMobile ? <div className="mb-6">
        <div className="space-y-3">
          <div
            className="relative aspect-square touch-pan-y overflow-hidden rounded-xl bg-muted"
            onTouchStart={handleTouchStart}
            onTouchMove={handleTouchMove}
            onTouchEnd={handleTouchEnd}
          >
            {isFeatured && (
              <Badge className="absolute right-3 top-3 z-10 bg-yellow-500 shadow-lg">
                <Star className="mr-1 h-3 w-3" />
                {t.product.featured}
              </Badge>
            )}
            {hasDiscount && (
              <Badge variant="destructive" className="absolute left-3 top-3 z-10 shadow-lg">
                {t.product.saleBadge}
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
                    className="pointer-events-none h-full w-full shrink-0 select-none object-cover"
                    draggable={false}
                    style={{ width: `${100 / displayImages.length}%` }}
                    onError={(e) => {
                      e.currentTarget.src =
                        'data:image/svg+xml,' +
                        encodeURIComponent(
                          '<svg xmlns="http://www.w3.org/2000/svg" width="80" height="80" viewBox="0 0 24 24" fill="none" stroke="%23999" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4"/></svg>'
                        )
                    }}
                  />
                ))}
              </div>
            ) : (
              <div className="flex h-full w-full items-center justify-center">
                <Package className="h-20 w-20 text-muted-foreground/50" />
              </div>
            )}
            {/* 图片指示器 */}
            {displayImages.length > 1 && (
              <div className="absolute bottom-3 left-1/2 flex -translate-x-1/2 gap-1.5">
                {displayImages.map((_: any, index: number) => (
                  <span
                    key={index}
                    className={`block h-1.5 w-1.5 rounded-full transition-all ${selectedImage === index ? 'w-3 bg-white' : 'bg-white/50'}`}
                  />
                ))}
              </div>
            )}
          </div>
          {displayImages.length > 1 && (
            <div
              ref={thumbScrollRef}
              className="flex cursor-grab select-none gap-2 overflow-x-auto pb-2 active:cursor-grabbing"
              onMouseDown={(e) => handleThumbMouseDown(e, thumbScrollRef)}
              onMouseMove={(e) => handleThumbMouseMove(e, thumbScrollRef)}
              onMouseUp={() => handleThumbMouseUp(thumbScrollRef)}
              onMouseLeave={() => handleThumbMouseUp(thumbScrollRef)}
            >
              {thumbnailList.map((image, index: number) => (
                <button
                  key={index}
                  type="button"
                  onClick={() => {
                    if (!thumbDragMoved.current) setSelectedImage(index)
                  }}
                  aria-label={`${t.common.view} ${product.name} ${index + 1}`}
                  aria-pressed={selectedImage === index}
                  title={`${t.common.view} ${product.name} ${index + 1}`}
                  className={`aspect-square w-16 shrink-0 overflow-hidden rounded-lg border-2 transition-all ${selectedImage === index ? 'border-primary ring-1 ring-primary' : 'border-border hover:border-muted-foreground'}`}
                >
                  <img
                    src={image.url}
                    alt={image.alt || product.name}
                    className="pointer-events-none h-full w-full object-cover"
                    onError={(e) => {
                      e.currentTarget.src =
                        'data:image/svg+xml,' +
                        encodeURIComponent(
                          '<svg xmlns="http://www.w3.org/2000/svg" width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="%23999" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4"/></svg>'
                        )
                    }}
                  />
                </button>
              ))}
            </div>
          )}
        </div>
      </div> : null}

      <div className={cn(isMobile ? 'space-y-6' : 'flex items-start gap-8')}>
        {/* Left: Image gallery (sticky) */}
        {!isMobile ? <div className="sticky top-4 w-[520px] shrink-0 self-start">
          <div className="space-y-3">
            <div
              className="relative aspect-square cursor-grab select-none overflow-hidden rounded-xl border border-border bg-muted active:cursor-grabbing"
              onMouseDown={handleTouchStart}
              onMouseMove={handleTouchMove}
              onMouseUp={handleTouchEnd}
              onMouseLeave={() => {
                if (isDragging.current) handleTouchEnd()
              }}
            >
              {isFeatured && (
                <Badge className="absolute right-3 top-3 z-10 bg-yellow-500 shadow-lg">
                  <Star className="mr-1 h-3 w-3" />
                  {t.product.featured}
                </Badge>
              )}
              {hasDiscount && (
                <Badge variant="destructive" className="absolute left-3 top-3 z-10 shadow-lg">
                  {t.product.saleBadge}
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
                      className="pointer-events-none h-full w-full shrink-0 select-none object-cover"
                      draggable={false}
                      style={{ width: `${100 / displayImages.length}%` }}
                      onError={(e) => {
                        e.currentTarget.src =
                          'data:image/svg+xml,' +
                          encodeURIComponent(
                            '<svg xmlns="http://www.w3.org/2000/svg" width="80" height="80" viewBox="0 0 24 24" fill="none" stroke="%23999" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4"/></svg>'
                          )
                      }}
                    />
                  ))}
                </div>
              ) : (
                <div className="flex h-full w-full items-center justify-center">
                  <Package className="h-20 w-20 text-muted-foreground/50" />
                </div>
              )}
            </div>
            {displayImages.length > 1 && (
              <div
                ref={thumbScrollRefDesktop}
                className="flex cursor-grab select-none gap-2 overflow-x-auto pb-2 active:cursor-grabbing"
                onMouseDown={(e) => handleThumbMouseDown(e, thumbScrollRefDesktop)}
                onMouseMove={(e) => handleThumbMouseMove(e, thumbScrollRefDesktop)}
                onMouseUp={() => handleThumbMouseUp(thumbScrollRefDesktop)}
                onMouseLeave={() => handleThumbMouseUp(thumbScrollRefDesktop)}
              >
                {thumbnailList.map((image, index: number) => (
                  <button
                    key={index}
                    type="button"
                    onClick={() => {
                      if (!thumbDragMoved.current) setSelectedImage(index)
                    }}
                    aria-label={`${t.common.view} ${product.name} ${index + 1}`}
                    aria-pressed={selectedImage === index}
                    title={`${t.common.view} ${product.name} ${index + 1}`}
                    className={`aspect-square w-20 shrink-0 overflow-hidden rounded-lg border-2 transition-all ${selectedImage === index ? 'border-primary ring-1 ring-primary' : 'border-border hover:border-muted-foreground'}`}
                  >
                    <img
                      src={image.url}
                      alt={image.alt || product.name}
                      className="pointer-events-none h-full w-full object-cover"
                      onError={(e) => {
                        e.currentTarget.src =
                          'data:image/svg+xml,' +
                          encodeURIComponent(
                            '<svg xmlns="http://www.w3.org/2000/svg" width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="%23999" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4"/></svg>'
                          )
                      }}
                    />
                  </button>
                ))}
              </div>
            )}
          </div>
        </div> : null}

        {/* Right: Product info */}
        <div className="min-w-0 flex-1 self-start">
          <div className={cn('space-y-4', !isMobile && 'md:sticky md:top-2 md:space-y-5')}>
            {/* Summary card */}
            <div className="overflow-hidden rounded-2xl border border-border bg-card shadow-sm">
              <div
                className={cn(
                  'border-b border-border/70 bg-gradient-to-br from-muted/50 via-background to-background p-5',
                  !isMobile && 'md:p-6'
                )}
              >
                <h2 className="mb-3 text-2xl font-bold leading-tight">{product.name}</h2>
                <p className="flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
                  <Eye className="h-3.5 w-3.5" />
                  <span>{productOverviewSummary}</span>
                </p>
              </div>

              <div className={cn('space-y-4 p-5', !isMobile && 'md:p-6')}>
                {/* Price card */}
                <div className="space-y-2 rounded-xl border border-border bg-muted/40 p-4">
                  <div className="flex flex-col gap-1 sm:flex-row sm:items-baseline sm:gap-3">
                    <span className="text-3xl font-bold text-red-500">
                      {formatPrice(product.price_minor, currency)}
                    </span>
                    {hasDiscount && (
                      <div className="flex items-center gap-2 text-sm text-muted-foreground">
                        <span className="text-base text-muted-foreground line-through">
                          {formatPrice(product.original_price_minor, currency)}
                        </span>
                        <span>
                          {t.product.save} {getCurrencySymbol(currency)}
                          {discountAmount}
                        </span>
                      </div>
                    )}
                  </div>
                  {appliedPromo && promoDiscount > 0 && (
                    <div className="mt-2 flex items-baseline gap-3 border-t border-border/50 pt-1">
                      <div className="flex items-center gap-2">
                        <Tag className="h-3.5 w-3.5 text-green-600 dark:text-green-400" />
                        <span className="text-sm text-muted-foreground line-through">
                          {formatPrice(subtotal, currency)}
                        </span>
                      </div>
                      <span className="text-2xl font-bold text-green-600 dark:text-green-400">
                        {formatPrice(subtotal - promoDiscount, currency)}
                      </span>
                      <span className="text-xs text-green-600 dark:text-green-400">
                        -{promoDiscountLabel}
                      </span>
                    </div>
                  )}
                  <div className="text-sm text-muted-foreground">
                    {t.product.sku}: {product.sku}
                  </div>
                </div>

                {/* Virtual product notice */}
                {isVirtual && (
                  <div className="rounded-xl border border-purple-500/20 bg-purple-500/10 p-4">
                    <div className="flex items-start gap-3">
                      <Key className="mt-0.5 h-5 w-5 shrink-0 text-purple-500" />
                      <div className="text-sm leading-relaxed text-purple-700 dark:text-purple-300">
                        {product.auto_delivery || product.autoDelivery
                          ? t.product.virtualProductNoticeInstant
                          : t.product.virtualProductNoticeManual}
                      </div>
                    </div>
                  </div>
                )}

                {/* Product meta */}
                <div className="grid gap-3 sm:grid-cols-2">
                  {stockDisplayMode !== 'hidden' && !isUnlimitedStock && (
                    <div className="space-y-1.5 rounded-xl border border-border p-3">
                      <div className="text-xs text-muted-foreground">{t.product.stockLabel}</div>
                      {getStockDisplay()}
                    </div>
                  )}
                  {product.category && (
                    <div className="space-y-1.5 rounded-xl border border-border p-3">
                      <div className="text-xs text-muted-foreground">{t.product.categoryLabel}</div>
                      <div className="text-sm font-medium">{product.category}</div>
                    </div>
                  )}
                  {productMaxPurchaseLimit > 0 && (
                    <div className="space-y-1.5 rounded-xl border border-border p-3">
                      <div className="text-xs text-muted-foreground">
                        {t.product.purchaseLimitLabel}
                      </div>
                      <div className="text-sm font-medium text-orange-600 dark:text-orange-400">
                        {t.product.maxPurchaseLimit} {productMaxPurchaseLimit}{' '}
                        {t.product.piecesUnit}
                      </div>
                    </div>
                  )}
                  {product.tags && product.tags.length > 0 && (
                    <div className="space-y-1.5 rounded-xl border border-border p-3 sm:col-span-2">
                      <div className="text-xs text-muted-foreground">{t.product.tagsLabel}</div>
                      <div className="text-sm text-foreground">{product.tags.join(' · ')}</div>
                    </div>
                  )}
                </div>
                <PluginSlot
                  slot="user.product_detail.meta.after"
                  context={{ ...userProductDetailPluginContext, section: 'meta' }}
                />
              </div>
            </div>

            {/* Purchase card */}
            <div className="rounded-2xl border border-border bg-card shadow-sm">
              <div className={cn('space-y-5 p-5', !isMobile && 'md:p-6')}>
                {showGuestPurchaseHint && (
                  <Alert className="border-primary/20 bg-primary/5">
                    <Key className="h-4 w-4 text-primary" />
                    <AlertTitle>{t.auth.loginRequiredTitle}</AlertTitle>
                    <AlertDescription className="space-y-3">
                      <p>{t.product.guestPurchaseHint}</p>
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={redirectToLoginWithProductReturn}
                      >
                        {t.auth.loginToContinue}
                      </Button>
                    </AlertDescription>
                  </Alert>
                )}

                {!isAvailable && (
                  <Alert className="border-destructive/20 bg-destructive/5">
                    <Package className="h-4 w-4 text-destructive" />
                    <AlertTitle>{t.product.soldOut}</AlertTitle>
                    <AlertDescription>{t.product.soldOutHint}</AlertDescription>
                  </Alert>
                )}

                {/* Specs selection */}
                {(selectableAttributes.length > 0 || hasBlindBoxAttributes) && (
                  <div className="space-y-4">
                    {/* Blind box */}
                    {hasBlindBoxAttributes && (
                      <div className="rounded-lg bg-purple-500/10 p-3">
                        <div className="flex items-start gap-2">
                          <div className="text-xl">🎲</div>
                          <div className="flex-1">
                            <div className="mb-1 text-sm font-medium text-purple-600 dark:text-purple-400">
                              {t.product.blindBoxAttribute}
                            </div>
                            <div className="text-xs text-purple-600/80 dark:text-purple-400/80">
                              {(product.attributes || [])
                                .filter((attr: any) => attr.mode === 'blind_box')
                                .map((attr: any) => (
                                  <div key={attr.name} className="mb-1">
                                    <span className="font-medium">{attr.name}</span>:{' '}
                                    {attr.values.join('、')}
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
                                    type="button"
                                    onClick={() => handleAttributeChange(attr.name, value)}
                                    aria-pressed={isSelected}
                                    title={`${attr.name}: ${value}`}
                                    className={`rounded-lg border px-4 py-1.5 text-sm transition-all ${
                                      isSelected
                                        ? 'border-primary bg-primary font-medium text-primary-foreground'
                                        : 'border-border text-foreground hover:border-primary/50'
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
                <div className="space-y-2 rounded-xl border border-border bg-muted/20 p-4">
                  <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                    <div className="space-y-1">
                      <div className="text-sm text-muted-foreground">{t.product.quantity}:</div>
                      {stockDisplayMode === 'exact' && !isUnlimitedStock ? (
                        <div className="text-xs text-muted-foreground">
                          {t.product.stockLabel}: {availableStock}
                        </div>
                      ) : null}
                    </div>
                    <div className="flex items-center self-end sm:self-auto">
                      <Button
                        variant="outline"
                        size="icon"
                        className="h-9 w-9 rounded-r-none"
                        onClick={() => handleQuantityChange(quantity - 1)}
                        disabled={quantity <= 1}
                        aria-label={t.product.decreaseQuantity}
                        title={t.product.decreaseQuantity}
                      >
                        <Minus className="h-3.5 w-3.5" />
                        <span className="sr-only">{t.product.decreaseQuantity}</span>
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
                        className="h-9 w-16 rounded-none border-x-0 text-center focus-visible:ring-0 focus-visible:ring-offset-0"
                        min={1}
                        max={maxSelectableQuantity}
                        aria-label={t.product.quantity}
                      />
                      <Button
                        variant="outline"
                        size="icon"
                        className="h-9 w-9 rounded-l-none"
                        onClick={() => handleQuantityChange(quantity + 1)}
                        disabled={quantity >= maxSelectableQuantity}
                        aria-label={t.product.increaseQuantity}
                        title={t.product.increaseQuantity}
                      >
                        <Plus className="h-3.5 w-3.5" />
                        <span className="sr-only">{t.product.increaseQuantity}</span>
                      </Button>
                    </div>
                  </div>
                </div>

                {!allAttributesSelected && product.attributes && product.attributes.length > 0 && (
                  <p className="text-xs text-amber-600 dark:text-amber-400">
                    {t.product.pleaseSelectAllSpec}
                  </p>
                )}
                <PluginSlot
                  slot="user.product_detail.selection.after"
                  context={{ ...userProductDetailPluginContext, section: 'selection' }}
                />

                {/* Promo code */}
                <div className="space-y-3 rounded-xl border border-border bg-muted/10 p-4">
                  <div className="flex items-center gap-2 text-sm font-medium">
                    <Tag className="h-4 w-4" />
                    {t.promoCode.enterPromoCode}
                  </div>
                  {!appliedPromo ? (
                    <>
                      <div className="flex gap-2">
                        <Input
                          value={promoCodeInput}
                          onChange={(e) => setPromoCodeInput(e.target.value)}
                          placeholder={t.promoCode.promoCodePlaceholder}
                          className="flex-1"
                          maxLength={50}
                          onKeyDown={(e) => {
                            if (e.key === 'Enter') handleApplyPromoCode()
                          }}
                        />
                        <Button
                          onClick={handleApplyPromoCode}
                          disabled={authLoading || !isAuthenticated || !promoCodeInput.trim() || isValidatingPromo}
                          size="default"
                        >
                          {isValidatingPromo ? t.promoCode.applying : t.promoCode.apply}
                        </Button>
                      </div>
                      {isGuestMode && (
                        <p className="text-xs text-muted-foreground">
                          {t.product.loginForPromoCode}
                        </p>
                      )}
                    </>
                  ) : (
                    <div className="space-y-2">
                      <div className="flex items-center justify-between rounded-lg border border-green-500/20 bg-green-500/10 p-3 dark:border-green-500/30 dark:bg-green-500/20">
                        <div>
                          <div className="text-sm font-medium text-green-700 dark:text-green-400">
                            {appliedPromo.name}
                          </div>
                          <div className="mt-0.5 text-xs text-green-600 dark:text-green-500">
                            {t.promoCode.applied} &mdash; {appliedPromo.code}
                          </div>
                        </div>
                        <Button
                          variant="ghost"
                          size="sm"
                          className="text-red-500 hover:bg-red-500/10 hover:text-red-600"
                          onClick={handleRemovePromoCode}
                        >
                          {t.promoCode.remove}
                        </Button>
                      </div>
                      <div className="flex items-center justify-between text-sm">
                        <span className="text-muted-foreground">{t.promoCode.discount}</span>
                        <span className="font-medium text-green-600 dark:text-green-400">
                          -{formatPrice(promoDiscount, currency)}
                        </span>
                      </div>
                    </div>
                  )}
                </div>
                <PluginSlot
                  slot="user.product_detail.promo.after"
                  context={{ ...userProductDetailPluginContext, section: 'promo' }}
                />
                {isGuestMode && guestActionHint && (
                  <Alert className="border-primary/20 bg-primary/5">
                    {guestActionHint === 'cart_added' ? (
                      <ShoppingCart className="h-4 w-4 text-primary" />
                    ) : (
                      <Key className="h-4 w-4 text-primary" />
                    )}
                    <AlertTitle>{guestActionTitle}</AlertTitle>
                    <AlertDescription className="space-y-3">
                      <p>{guestActionDescription}</p>
                      <div className="flex flex-col gap-2 sm:flex-row">
                        {guestActionHint === 'cart_added' && (
                          <Button asChild size="sm" variant="outline">
                            <Link href="/cart">{t.sidebar.cart}</Link>
                          </Button>
                        )}
                        <Button size="sm" onClick={redirectToLoginWithProductReturn}>
                          {t.auth.loginToContinue}
                        </Button>
                        <Button size="sm" variant="ghost" onClick={() => setGuestActionHint(null)}>
                          {t.product.continueBrowsing}
                        </Button>
                      </div>
                    </AlertDescription>
                  </Alert>
                )}
                <PluginSlot
                  slot="user.product_detail.guest_hint.after"
                  context={{ ...userProductDetailPluginContext, section: 'guest_hint' }}
                />
                <PluginSlot
                  slot="user.product_detail.actions.before"
                  context={{ ...userProductDetailPluginContext, section: 'purchase_actions' }}
                />

                {/* Action buttons */}
                <div className="flex flex-col gap-3 sm:flex-row">
                  <Button
                    variant="outline"
                    className="h-11 min-w-0 flex-1"
                    disabled={authLoading || !isAvailable || !allAttributesSelected || isAddingToCart}
                    onClick={handleAddToCart}
                  >
                    <span className="inline-flex items-center truncate">
                      {isAddingToCart ? (
                        <>
                          <Loader2 className="mr-2 h-4 w-4 shrink-0 animate-spin" />
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
                    className="h-11 min-w-0 flex-1"
                    disabled={
                      authLoading ||
                      !isAvailable ||
                      !allAttributesSelected ||
                      createOrderMutation.isPending
                    }
                    onClick={handleBuyNow}
                  >
                    <span className="inline-flex items-center truncate">
                      {createOrderMutation.isPending ? (
                        <>
                          <Loader2 className="mr-2 h-4 w-4 shrink-0 animate-spin" />
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
            <PluginSlot
              slot="user.product_detail.buybox.after"
              context={{ ...userProductDetailPluginContext, section: 'buybox' }}
            />

            {/* Product description */}
            {product.description && (
              <div className="rounded-2xl border border-border bg-card shadow-sm">
                <div className={cn('p-5', !isMobile && 'md:p-6')}>
                  <h3 className="mb-3 font-semibold">{t.product.productDescription}</h3>
                  <PluginSlot
                    slot="user.product_detail.description.before"
                    context={{ ...userProductDetailPluginContext, section: 'description' }}
                  />
                  <MarkdownMessage
                    content={product.description}
                    className="markdown-body text-sm"
                    allowHtml
                  />
                  <PluginSlot
                    slot="user.product_detail.description.after"
                    context={{ ...userProductDetailPluginContext, section: 'description' }}
                  />
                </div>
              </div>
            )}
            <PluginSlot
              slot="user.product_detail.content.after"
              context={{ ...userProductDetailPluginContext, section: 'content' }}
            />
          </div>
        </div>
      </div>
      <PluginSlot slot="user.product_detail.bottom" context={userProductDetailPluginContext} />
    </div>
  )
}
