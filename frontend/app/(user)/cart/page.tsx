'use client'
/* eslint-disable @next/next/no-img-element */

import { useState, useMemo, useEffect, useCallback, useRef } from 'react'
import { useRouter } from 'next/navigation'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useCart } from '@/contexts/cart-context'
import {
  createOrder,
  validatePromoCode,
  getPublicConfig,
  getProduct,
  getProductAvailableStock,
} from '@/lib/api'
import { resolveApiErrorMessage } from '@/lib/api-error'
import { Card, CardContent, CardHeader, CardFooter } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import {
  Trash2,
  Minus,
  Plus,
  ShoppingCart,
  Package,
  AlertCircle,
  RefreshCw,
  LayoutGrid,
  LayoutList,
  Loader2,
} from 'lucide-react'
import Link from 'next/link'
import { useAuth } from '@/hooks/use-auth'
import { useLocale } from '@/hooks/use-locale'
import { useResponsiveLayout } from '@/hooks/use-mobile'
import { usePageTitle } from '@/hooks/use-page-title'
import { getTranslations } from '@/lib/i18n'
import toast from 'react-hot-toast'
import { Checkbox } from '@/components/ui/checkbox'
import { useCurrency, formatPrice } from '@/contexts/currency-context'
import { PluginSlot } from '@/components/plugins/plugin-slot'
import {
  getGuestCart,
  getGuestCartItemKey,
  setGuestCart,
  removeGuestCartItemByKey,
  updateGuestCartItemQuantityByKey,
} from '@/lib/guest-cart'
import {
  clearCachedGuestCartDisplayItems,
  getGuestDisplayItemId,
  getRestorableGuestCartDisplayItems,
  hydrateGuestCartDisplayItems,
  setCachedGuestCartDisplayItems,
  type GuestCartDisplayItem,
} from '@/lib/guest-cart-display'
import {
  clearAuthReturnState,
  readAuthReturnState,
  setAuthReturnState,
} from '@/lib/auth-return-state'

type CartConfirmAction =
  | {
      type: 'remove_item'
      itemId: number
      itemName: string
    }
  | {
      type: 'clear_selected'
      count: number
    }
  | null

export default function CartPage() {
  const router = useRouter()
  const queryClient = useQueryClient()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.cart)
  const { isPhone, isTablet, isMobile } = useResponsiveLayout()
  const { currency } = useCurrency()
  const { isAuthenticated, isLoading: authLoading } = useAuth()
  const {
    items: serverItems,
    isLoading: serverCartLoading,
    isError: isServerCartError,
    updateQuantity,
    removeItem,
    removeItems,
    refetch,
  } = useCart()
  const { data: publicConfig } = useQuery({
    queryKey: ['publicConfig'],
    queryFn: getPublicConfig,
    staleTime: 1000 * 60 * 5,
  })
  const maxItemQuantity = publicConfig?.data?.max_item_quantity || 9999
  const maxOrderItems = publicConfig?.data?.max_order_items || 100
  const [guestItems, setGuestItems] = useState<GuestCartDisplayItem[]>([])
  const [isGuestItemsLoading, setIsGuestItemsLoading] = useState(false)
  const [hasGuestItemsLoaded, setHasGuestItemsLoaded] = useState(false)
  const [guestRefreshWarning, setGuestRefreshWarning] = useState(false)
  const [guestRefreshWarningMessage, setGuestRefreshWarningMessage] = useState('')
  const [selectedItems, setSelectedItems] = useState<Set<number>>(new Set())
  const guestRefreshRequestIdRef = useRef(0)
  const hasRestoredAuthReturnStateRef = useRef(false)
  const [viewMode, setViewMode] = useState<'list' | 'card'>(() => {
    if (typeof window !== 'undefined') {
      const saved = localStorage.getItem('cart-view-mode')
      if (saved === 'list' || saved === 'card') return saved
    }
    return 'list'
  })
  const [confirmAction, setConfirmAction] = useState<CartConfirmAction>(null)
  const [isConfirmPending, setIsConfirmPending] = useState(false)
  const isGuestMode = !isAuthenticated
  const cartLoadFailed = !isGuestMode && isServerCartError

  const getCartScrollTop = useCallback(() => {
    if (typeof document === 'undefined' || typeof window === 'undefined') {
      return 0
    }
    const mainElement = document.querySelector('main')
    if (mainElement instanceof HTMLElement) {
      return Math.max(0, mainElement.scrollTop)
    }
    return Math.max(0, window.scrollY)
  }, [])

  const restoreCartScrollTop = useCallback((scrollTop: number) => {
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

  const refreshGuestItems = useCallback(async () => {
    const requestId = guestRefreshRequestIdRef.current + 1
    guestRefreshRequestIdRef.current = requestId
    const localItems = getGuestCart()
    if (!localItems.length) {
      clearCachedGuestCartDisplayItems()
      setGuestItems([])
      setIsGuestItemsLoading(false)
      setGuestRefreshWarning(false)
      setGuestRefreshWarningMessage('')
      setHasGuestItemsLoaded(true)
      return
    }

    const restorableItems = getRestorableGuestCartDisplayItems(localItems)
    if (restorableItems.length > 0) {
      setGuestItems(restorableItems)
      setHasGuestItemsLoaded(true)
    }

    setIsGuestItemsLoading(true)
    setGuestRefreshWarning(false)
    setGuestRefreshWarningMessage('')
    try {
      const { items: enrichedItems, hasFailures } = await hydrateGuestCartDisplayItems({
        localItems,
        maxItemQuantity,
        concurrency: 4,
        fetchProduct: (productId) =>
          queryClient.fetchQuery({
            queryKey: ['guest-cart-product', productId],
            queryFn: async () => {
              const result = await getProduct(productId)
              return result?.data ?? null
            },
            staleTime: 1000 * 60 * 2,
            gcTime: 1000 * 60 * 10,
            retry: false,
          }),
        fetchStock: (productId, attributes) =>
          queryClient.fetchQuery({
            queryKey: ['guest-cart-stock', productId, JSON.stringify(attributes || {})],
            queryFn: async () => {
              const result = await getProductAvailableStock(productId, attributes)
              return result?.data ?? null
            },
            staleTime: 1000 * 30,
            gcTime: 1000 * 60 * 5,
            retry: false,
          }),
      })

      if (requestId !== guestRefreshRequestIdRef.current) return

      setGuestItems(enrichedItems)
      setGuestRefreshWarning(hasFailures)
      setGuestRefreshWarningMessage(hasFailures ? t.cart.guestRefreshWarning : '')
      setHasGuestItemsLoaded(true)
      if (!hasFailures) {
        setCachedGuestCartDisplayItems(enrichedItems)
      }
    } catch (error) {
      if (requestId !== guestRefreshRequestIdRef.current) return
      setGuestRefreshWarning(true)
      setGuestRefreshWarningMessage(resolveApiErrorMessage(error, t, t.cart.guestRefreshWarning))
      setHasGuestItemsLoaded(true)
    } finally {
      if (requestId === guestRefreshRequestIdRef.current) {
        setIsGuestItemsLoading(false)
      }
    }
  }, [maxItemQuantity, queryClient, t])

  useEffect(() => {
    if (!isGuestMode) return
    refreshGuestItems()
  }, [isGuestMode, refreshGuestItems])

  // Promo code state
  const [promoCodeInput, setPromoCodeInput] = useState('')
  const [promoCodeExpanded, setPromoCodeExpanded] = useState(false)
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
  const items = isGuestMode ? guestItems : serverItems
  const isLoading = authLoading || (isGuestMode ? !hasGuestItemsLoaded : serverCartLoading)

  const refetchCart = useCallback(() => {
    if (isGuestMode) {
      refreshGuestItems()
      return
    }
    refetch()
  }, [isGuestMode, refreshGuestItems, refetch])

  const redirectGuestToLogin = useCallback(() => {
    const selectedGuestKeys = items.flatMap((item) => {
      if (!selectedItems.has(item.id)) return []
      const guestItem = item as GuestCartDisplayItem | undefined
      return guestItem?.guest_key ? [guestItem.guest_key] : []
    })

    setAuthReturnState({
      redirectPath: '/cart',
      cart: {
        selectedGuestKeys,
        scrollTop: getCartScrollTop(),
      },
    })
    router.push('/login')
  }, [getCartScrollTop, items, router, selectedItems])

  useEffect(() => {
    setSelectedItems((prev) => {
      if (prev.size === 0) return prev
      const validIds = new Set(items.filter((item) => item.is_available).map((item) => item.id))
      const next = new Set<number>()
      for (const id of prev) {
        if (validIds.has(id)) next.add(id)
      }
      return next.size === prev.size ? prev : next
    })
  }, [items])

  useEffect(() => {
    if (isGuestMode || authLoading || serverCartLoading || hasRestoredAuthReturnStateRef.current) {
      return
    }

    const pendingReturnState = readAuthReturnState()
    if (!pendingReturnState || pendingReturnState.redirectPath !== '/cart') {
      return
    }

    hasRestoredAuthReturnStateRef.current = true
    const selectedGuestKeys = new Set(pendingReturnState.cart?.selectedGuestKeys || [])
    const nextSelectedItems = new Set<number>()
    for (const item of serverItems) {
      const itemKey = getGuestCartItemKey({
        product_id: item.product_id,
        quantity: item.quantity,
        attributes: item.attributes,
      })
      if (selectedGuestKeys.has(itemKey) && item.is_available) {
        nextSelectedItems.add(item.id)
      }
    }

    setSelectedItems(nextSelectedItems)
    restoreCartScrollTop(pendingReturnState.cart?.scrollTop || 0)
    clearAuthReturnState()
  }, [authLoading, isGuestMode, restoreCartScrollTop, serverCartLoading, serverItems])

  const handleViewModeChange = (mode: 'list' | 'card') => {
    setViewMode(mode)
    localStorage.setItem('cart-view-mode', mode)
  }

  // 应用优惠码
  const handleApplyPromoCode = async () => {
    if (isGuestMode) {
      toast.error(t.cart.loginForPromoCode)
      setTimeout(redirectGuestToLogin, 1000)
      return
    }

    if (!promoCodeInput.trim()) return

    setIsValidatingPromo(true)
    try {
      const selectedCartItems = items.filter(
        (item) => selectedItems.has(item.id) && item.is_available
      )
      const productIds = selectedCartItems.map((item) => item.product_id)
      const amount = selectedCartItems.reduce(
        (sum, item) => sum + item.price_minor * item.quantity,
        0
      )

      const response = await validatePromoCode({
        code: promoCodeInput.trim(),
        product_ids: productIds.length > 0 ? productIds : undefined,
        amount_minor: amount > 0 ? amount : undefined,
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

  // 创建订单
  const createOrderMutation = useMutation({
    mutationFn: createOrder,
    onSuccess: async (data) => {
      const orderNo = data?.data?.order_no
      toast.success(t.cart.orderSuccess)
      // 只清空已选中的商品
      const selectedItemIds = Array.from(selectedItems)
      await removeItems(selectedItemIds)
      setSelectedItems(new Set())
      // 清空优惠码状态
      setAppliedPromo(null)
      setPromoCodeInput('')
      queryClient.invalidateQueries({ queryKey: ['orders'] })
      router.push(`/orders/${orderNo}`)
    },
    onError: (error: any) => {
      toast.error(resolveApiErrorMessage(error, t, t.cart.orderFailed))
    },
  })

  const getItemMaxQuantity = (item: any) => {
    const productMaxPurchaseLimit =
      item?.product?.max_purchase_limit ?? item?.product?.maxPurchaseLimit ?? 0
    return Math.min(
      item?.available_stock ?? 0,
      maxItemQuantity,
      productMaxPurchaseLimit > 0 ? productMaxPurchaseLimit : Number.MAX_SAFE_INTEGER
    )
  }

  // 处理数量变化
  const handleQuantityChange = async (itemId: number, newQuantity: number) => {
    const item = items.find((i) => i.id === itemId)
    const itemMaxQuantity = item ? getItemMaxQuantity(item) : maxItemQuantity
    if (newQuantity < 1 || newQuantity > itemMaxQuantity) return
    if (isGuestMode) {
      const guestItem = item as GuestCartDisplayItem | undefined
      if (!guestItem) return
      const result = updateGuestCartItemQuantityByKey(
        guestItem.guest_key,
        newQuantity,
        maxItemQuantity
      )
      if (result.updated) {
        const nextGuestItems = guestItems.map((existing) =>
          existing.guest_key === guestItem.guest_key
            ? { ...existing, quantity: newQuantity }
            : existing
        )
        setGuestItems(nextGuestItems)
        setCachedGuestCartDisplayItems(nextGuestItems)
      }
      return
    }
    try {
      await updateQuantity(itemId, newQuantity)
    } catch (error) {
      // 错误已在 context 中处理
    }
  }

  // 处理删除
  const removeCartItem = async (itemId: number) => {
    if (isGuestMode) {
      const guestItem = items.find((i) => i.id === itemId) as GuestCartDisplayItem | undefined
      if (!guestItem) return
      const result = removeGuestCartItemByKey(guestItem.guest_key)
      if (result.removed) {
        const nextGuestItems = guestItems.filter(
          (existing) => existing.guest_key !== guestItem.guest_key
        )
        setGuestItems(nextGuestItems)
        if (nextGuestItems.length > 0) {
          setCachedGuestCartDisplayItems(nextGuestItems)
        } else {
          clearCachedGuestCartDisplayItems()
        }
      }
      setSelectedItems((prev) => {
        const next = new Set(prev)
        next.delete(itemId)
        return next
      })
      toast.success(t.cart.removedFromCart)
      return
    }
    try {
      await removeItem(itemId)
      setSelectedItems((prev) => {
        const next = new Set(prev)
        next.delete(itemId)
        return next
      })
    } catch (error) {
      // 错误已在 context 中处理
    }
  }

  const clearSelectedItems = async () => {
    const selectedItemSet = new Set(selectedItems)
    if (selectedItemSet.size === 0) {
      toast.error(t.cart.noItemsSelected)
      return
    }

    if (isGuestMode) {
      const remainingGuestItems = getGuestCart().filter((item) => {
        const itemId = getGuestDisplayItemId(getGuestCartItemKey(item))
        return !selectedItemSet.has(itemId)
      })
      setGuestCart(remainingGuestItems)
      const nextGuestItems = guestItems.filter((item) => !selectedItemSet.has(item.id))
      setGuestItems(nextGuestItems)
      if (nextGuestItems.length > 0) {
        setCachedGuestCartDisplayItems(nextGuestItems)
      } else {
        clearCachedGuestCartDisplayItems()
      }
      toast.success(t.cart.selectedItemsRemoved)
    } else {
      await removeItems(Array.from(selectedItemSet))
    }

    setSelectedItems(new Set())
  }

  const requestRemoveItem = (itemId: number) => {
    const item = items.find((cartItem) => cartItem.id === itemId)
    if (!item) return
    setConfirmAction({
      type: 'remove_item',
      itemId,
      itemName: item.name,
    })
  }

  const requestClearSelected = () => {
    if (selectedItems.size === 0) {
      toast.error(t.cart.noItemsSelected)
      return
    }
    setConfirmAction({
      type: 'clear_selected',
      count: selectedItems.size,
    })
  }

  const handleConfirmAction = async () => {
    if (!confirmAction) return
    setIsConfirmPending(true)
    try {
      if (confirmAction.type === 'remove_item') {
        await removeCartItem(confirmAction.itemId)
      } else {
        await clearSelectedItems()
      }
      setConfirmAction(null)
    } finally {
      setIsConfirmPending(false)
    }
  }

  // 处理全选
  const handleSelectAll = () => {
    const availableItems = items.filter((item) => item.is_available)
    const allAvailableSelected =
      availableItems.length > 0 && availableItems.every((item) => selectedItems.has(item.id))
    if (allAvailableSelected) {
      setSelectedItems(new Set())
    } else {
      setSelectedItems(new Set(availableItems.map((item) => item.id)))
    }
  }

  // 处理单选
  const handleSelectItem = (itemId: number) => {
    setSelectedItems((prev) => {
      const next = new Set(prev)
      if (next.has(itemId)) {
        next.delete(itemId)
      } else {
        next.add(itemId)
      }
      return next
    })
  }

  // 计算选中商品的总价
  const selectedTotalPrice = items
    .filter((item) => selectedItems.has(item.id) && item.is_available)
    .reduce((sum, item) => sum + item.price_minor * item.quantity, 0)

  const selectedTotalQuantity = items
    .filter((item) => selectedItems.has(item.id) && item.is_available)
    .reduce((sum, item) => sum + item.quantity, 0)
  const availableItemCount = items.filter((item) => item.is_available).length
  const unavailableItemCount = items.length - availableItemCount
  const allAvailableItemsSelected =
    availableItemCount > 0 &&
    items.filter((item) => item.is_available).every((item) => selectedItems.has(item.id))
  const clearSelectedLabel =
    selectedItems.size > 0
      ? `${t.cart.clearSelected} (${selectedItems.size})`
      : t.cart.clearSelected

  // 实时计算优惠码折扣
  const promoDiscount = useMemo(() => {
    if (!appliedPromo || selectedTotalPrice <= 0) return 0

    if (appliedPromo.discount_type === 'percentage') {
      let discount = (selectedTotalPrice * appliedPromo.discount_value_minor) / 10000
      if (appliedPromo.max_discount_minor > 0 && discount > appliedPromo.max_discount_minor) {
        discount = appliedPromo.max_discount_minor
      }
      return Math.min(discount, selectedTotalPrice)
    } else {
      return Math.min(appliedPromo.discount_value_minor, selectedTotalPrice)
    }
  }, [appliedPromo, selectedTotalPrice])
  const userCartPluginContext = {
    view: 'user_cart',
    summary: {
      item_count: items.length,
      available_item_count: availableItemCount,
      unavailable_item_count: unavailableItemCount,
      selected_item_count: selectedItems.size,
      selected_item_ids: Array.from(selectedItems),
      selected_total_quantity: selectedTotalQuantity,
      selected_total_amount_minor: selectedTotalPrice,
      payable_total_amount_minor: Math.max(0, selectedTotalPrice - promoDiscount),
      all_available_items_selected: allAvailableItemsSelected,
      is_guest_mode: isGuestMode,
      is_loading: isLoading,
    },
    promo: {
      expanded: promoCodeExpanded,
      applied: Boolean(appliedPromo),
      code: appliedPromo?.code,
      name: appliedPromo?.name,
      discount_type: appliedPromo?.discount_type,
      discount_value_minor: appliedPromo?.discount_value_minor,
      discount_amount_minor: promoDiscount,
      min_order_amount_minor: appliedPromo?.min_order_amount_minor,
      max_discount_minor: appliedPromo?.max_discount_minor,
    },
    display: {
      view_mode: viewMode,
      currency,
    },
    limits: {
      max_item_quantity: maxItemQuantity,
      max_order_items: maxOrderItems,
    },
    confirm: {
      open: Boolean(confirmAction),
      type: confirmAction?.type,
      item_id: confirmAction?.type === 'remove_item' ? confirmAction.itemId : undefined,
      item_name: confirmAction?.type === 'remove_item' ? confirmAction.itemName : undefined,
      count: confirmAction?.type === 'clear_selected' ? confirmAction.count : undefined,
    },
    state: {
      load_failed: cartLoadFailed && items.length === 0,
      empty: !cartLoadFailed && items.length === 0,
      has_selection: selectedItems.size > 0,
      promo_expanded: promoCodeExpanded,
      promo_applied: Boolean(appliedPromo),
      promo_validating: isValidatingPromo,
      checkout_submitting: createOrderMutation.isPending,
      confirm_open: Boolean(confirmAction),
      confirm_pending: isConfirmPending,
    },
  }

  // 提交订单
  const handleCheckout = () => {
    if (isGuestMode) {
      toast.error(t.cart.loginForCheckout)
      setTimeout(redirectGuestToLogin, 1000)
      return
    }

    const selectedCartItems = items.filter(
      (item) => selectedItems.has(item.id) && item.is_available
    )

    if (selectedCartItems.length === 0) {
      toast.error(t.cart.selectItems)
      return
    }

    if (selectedCartItems.length > maxOrderItems) {
      toast.error(t.cart.tooManyItems)
      return
    }

    const orderItems = selectedCartItems.map((item) => ({
      sku: item.sku,
      name: item.name,
      quantity: item.quantity,
      image_url: item.image_url,
      attributes: item.attributes,
      product_type: item.product_type,
    }))

    createOrderMutation.mutate({
      items: orderItems,
      ...(appliedPromo ? { promo_code: appliedPromo.code } : {}),
    })
  }

  if (isLoading) {
    return (
      <div className="space-y-6">
        <h1 className="text-2xl font-bold md:text-3xl">{t.cart.title}</h1>
        <div className="space-y-4">
          {[...Array(3)].map((_, i) => (
            <Card key={i} className="animate-pulse">
              <CardContent className="p-4">
                <div className="flex gap-4">
                  <div className="h-20 w-20 rounded bg-muted" />
                  <div className="flex-1 space-y-2">
                    <div className="h-4 w-3/4 rounded bg-muted" />
                    <div className="h-4 w-1/2 rounded bg-muted" />
                  </div>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      </div>
    )
  }

  if (cartLoadFailed && items.length === 0) {
    return (
      <div className="flex min-h-[60vh] items-center justify-center">
        <Card className="w-full max-w-xl border-dashed bg-muted/15">
          <CardContent className="py-12 text-center">
            <AlertCircle className="mx-auto mb-4 h-16 w-16 text-muted-foreground" />
            <h1 className="mb-2 text-xl font-bold md:text-2xl">{t.cart.cartLoadFailedTitle}</h1>
            <p className="mx-auto mb-6 max-w-md text-sm text-muted-foreground md:text-base">
              {t.cart.cartLoadFailedDesc}
            </p>
            <div className="flex flex-col justify-center gap-3 sm:flex-row">
              <Button variant="outline" onClick={refetchCart}>
                <RefreshCw className="mr-2 h-4 w-4" />
                {t.common.refresh}
              </Button>
              <Button asChild>
                <Link href="/products">{t.cart.goShopping}</Link>
              </Button>
            </div>
            <PluginSlot
              slot="user.cart.load_failed"
              context={{ ...userCartPluginContext, section: 'load_failed' }}
            />
          </CardContent>
        </Card>
      </div>
    )
  }

  if (items.length === 0) {
    return (
      <div className="flex min-h-[60vh] items-center justify-center">
        <Card className="w-full max-w-xl border-dashed bg-muted/15">
          <CardContent className="py-12 text-center">
            <ShoppingCart className="mx-auto mb-4 h-16 w-16 text-muted-foreground" />
            <h1 className="mb-2 text-xl font-bold md:text-2xl">{t.cart.empty}</h1>
            <div className="mt-6 flex flex-col justify-center gap-3 sm:flex-row">
              <Button asChild>
                <Link href="/products">{t.cart.goShopping}</Link>
              </Button>
              <Button asChild variant="outline">
                <Link href={isGuestMode ? '/login' : '/orders'}>
                  {isGuestMode ? t.cart.loginToCheckout : t.sidebar.myOrders}
                </Link>
              </Button>
            </div>
            <PluginSlot slot="user.cart.empty" context={{ ...userCartPluginContext, section: 'empty' }} />
          </CardContent>
        </Card>
      </div>
    )
  }

  return (
    <div className={isPhone ? 'space-y-6 pb-28' : 'space-y-6 pb-24'}>
      <div className="flex items-center justify-between">
        <h1 className={isMobile ? 'text-2xl font-bold' : 'text-2xl font-bold md:text-3xl'}>
          {t.cart.title}
          {!isMobile ? (
            <span className="ml-2 text-lg font-normal text-muted-foreground">
              ({items.length} {t.cart.items})
            </span>
          ) : null}
        </h1>
        <div className="flex items-center gap-2">
          {!isMobile ? (
            <div className="hidden items-center rounded-lg border p-0.5 md:flex">
              <Button
                variant={viewMode === 'list' ? 'secondary' : 'ghost'}
                size="sm"
                className="h-7 px-2"
                onClick={() => handleViewModeChange('list')}
                aria-pressed={viewMode === 'list'}
                aria-label={t.cart.listView}
                title={t.cart.listView}
              >
                <LayoutList className="h-4 w-4" />
                <span className="sr-only">{t.cart.listView}</span>
              </Button>
              <Button
                variant={viewMode === 'card' ? 'secondary' : 'ghost'}
                size="sm"
                className="h-7 px-2"
                onClick={() => handleViewModeChange('card')}
                aria-pressed={viewMode === 'card'}
                aria-label={t.cart.cardView}
                title={t.cart.cardView}
              >
                <LayoutGrid className="h-4 w-4" />
                <span className="sr-only">{t.cart.cardView}</span>
              </Button>
            </div>
          ) : null}
          <Button
            variant="outline"
            size="sm"
            onClick={refetchCart}
            disabled={isGuestMode && isGuestItemsLoading}
            aria-label={t.cart.refresh}
            title={t.cart.refresh}
          >
            <RefreshCw className={`h-4 w-4 ${!isMobile ? 'md:mr-2' : ''} ${isGuestItemsLoading ? 'animate-spin' : ''}`} />
            {isMobile ? <span className="sr-only">{t.cart.refresh}</span> : <span>{t.cart.refresh}</span>}
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={requestClearSelected}
            disabled={selectedItems.size === 0}
            aria-label={clearSelectedLabel}
            title={clearSelectedLabel}
          >
            <Trash2 className={`h-4 w-4 ${!isMobile ? 'md:mr-2' : ''}`} />
            {isMobile ? (
              <span className="sr-only">{clearSelectedLabel}</span>
            ) : (
              <span>{clearSelectedLabel}</span>
            )}
          </Button>
        </div>
      </div>

      <PluginSlot slot="user.cart.top" context={userCartPluginContext} />

      {unavailableItemCount > 0 ? (
        <div className="flex flex-wrap gap-2">
          <Badge variant="destructive">
            {t.cart.unavailableItems}: {unavailableItemCount}
          </Badge>
        </div>
      ) : null}

      {isGuestMode && guestRefreshWarning && (
        <Card className="border-amber-200 bg-amber-50/60 dark:border-amber-500/40 dark:bg-amber-950/30">
          <CardContent
            className={
              isMobile
                ? 'flex flex-col gap-3 p-3 text-sm text-amber-800 dark:text-amber-200'
                : 'flex flex-col gap-3 p-3 text-sm text-amber-800 dark:text-amber-200 md:flex-row md:items-start md:justify-between'
            }
          >
            <div className="flex items-start gap-2">
              <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
              <span>{guestRefreshWarningMessage || t.cart.guestRefreshWarning}</span>
            </div>
            <Button
              variant="outline"
              size="sm"
              className="border-amber-300 bg-transparent text-amber-800 hover:bg-amber-100 hover:text-amber-900 dark:border-amber-500/40 dark:text-amber-200 dark:hover:bg-amber-900/40 dark:hover:text-amber-100"
              onClick={refetchCart}
              disabled={isGuestItemsLoading}
            >
              {t.common.refresh}
            </Button>
          </CardContent>
        </Card>
      )}

      {viewMode === 'list' && (
        <div
          className={
            isPhone ? 'space-y-4' : isTablet ? 'grid grid-cols-2 gap-4' : 'space-y-4'
          }
        >
          {items.map((item) => (
            <Card
              key={item.id}
              className={`${isTablet ? 'h-full' : ''} ${!item.is_available ? 'opacity-60' : ''}`.trim()}
            >
              <CardContent className={isMobile ? 'p-3' : 'p-3 md:p-4'}>
                {isMobile ? (
                  <>
                    {/* 第一行：选择框 + 图片 + 商品信息 */}
                    <div className="flex gap-3">
                      <div className="flex items-start pt-1">
                        <Checkbox
                          checked={selectedItems.has(item.id)}
                          onCheckedChange={() => handleSelectItem(item.id)}
                          disabled={!item.is_available}
                        />
                      </div>
                      <Link href={`/products/${item.product_id}`} className="shrink-0">
                        {item.image_url ? (
                          <img
                            src={item.image_url}
                            alt={item.name}
                            className="h-16 w-16 rounded object-cover"
                            onError={(e) => {
                              e.currentTarget.style.display = 'none'
                              e.currentTarget.parentElement
                                ?.querySelector('.img-fallback')
                                ?.classList.remove('hidden')
                            }}
                          />
                        ) : null}
                        <div
                          className={`img-fallback flex h-16 w-16 items-center justify-center rounded bg-muted ${item.image_url ? 'hidden' : ''}`}
                        >
                          <Package className="h-6 w-6 text-muted-foreground" />
                        </div>
                      </Link>
                      <div className="min-w-0 flex-1">
                        <div className="flex items-start justify-between gap-2">
                          <Link href={`/products/${item.product_id}`} className="min-w-0 flex-1">
                            <h3 className="line-clamp-2 text-sm font-semibold hover:text-primary">
                              {item.name}
                            </h3>
                          </Link>
                          <Button
                            variant="ghost"
                            size="icon"
                            className="-mr-2 -mt-1 h-7 w-7 shrink-0 text-muted-foreground hover:text-red-500"
                            onClick={() => requestRemoveItem(item.id)}
                            aria-label={t.cart.removeItem}
                            title={t.cart.removeItem}
                          >
                            <Trash2 className="h-4 w-4" />
                            <span className="sr-only">{t.cart.removeItem}</span>
                          </Button>
                        </div>
                        {item.attributes && Object.keys(item.attributes).length > 0 && (
                          <div className="mt-1 flex flex-wrap gap-1">
                            {Object.entries(item.attributes).map(([key, value]) => (
                              <span key={key} className="rounded bg-muted px-1.5 py-0.5 text-xs">
                                {key}: {value}
                              </span>
                            ))}
                          </div>
                        )}
                        {!item.is_available && (
                          <div className="mt-1 flex items-center gap-1 text-xs text-red-500">
                            <AlertCircle className="h-3 w-3" />
                            {t.cart.outOfStock}
                          </div>
                        )}
                      </div>
                    </div>
                    {/* 第二行：价格 + 数量控制 */}
                    <div className="mt-2 flex items-center justify-between pl-7">
                      <span className="font-bold text-red-600">
                        {formatPrice(item.price_minor, currency)}
                      </span>
                      <div className="flex items-center gap-1">
                        <Button
                          variant="outline"
                          size="icon"
                          className="h-7 w-7"
                          onClick={() => handleQuantityChange(item.id, item.quantity - 1)}
                          disabled={item.quantity <= 1}
                          aria-label={t.cart.decreaseQuantity}
                          title={t.cart.decreaseQuantity}
                        >
                          <Minus className="h-3 w-3" />
                          <span className="sr-only">{t.cart.decreaseQuantity}</span>
                        </Button>
                        <Input
                          type="number"
                          value={item.quantity}
                          onChange={(e) => {
                            const val = parseInt(e.target.value)
                            if (!isNaN(val) && val >= 1 && val <= getItemMaxQuantity(item))
                              handleQuantityChange(item.id, val)
                          }}
                          className="h-7 w-10 px-0 text-center text-sm"
                          min={1}
                          max={getItemMaxQuantity(item)}
                          aria-label={t.cart.items}
                        />
                        <Button
                          variant="outline"
                          size="icon"
                          className="h-7 w-7"
                          onClick={() => handleQuantityChange(item.id, item.quantity + 1)}
                          disabled={item.quantity >= getItemMaxQuantity(item)}
                          aria-label={t.cart.increaseQuantity}
                          title={t.cart.increaseQuantity}
                        >
                          <Plus className="h-3 w-3" />
                          <span className="sr-only">{t.cart.increaseQuantity}</span>
                        </Button>
                      </div>
                    </div>
                  </>
                ) : (
                  <div className="flex gap-4">
                    <div className="flex items-center">
                      <Checkbox
                        checked={selectedItems.has(item.id)}
                        onCheckedChange={() => handleSelectItem(item.id)}
                        disabled={!item.is_available}
                      />
                    </div>
                    <Link href={`/products/${item.product_id}`} className="shrink-0">
                      {item.image_url ? (
                        <img
                          src={item.image_url}
                          alt={item.name}
                          className="h-20 w-20 rounded object-cover"
                          onError={(e) => {
                            e.currentTarget.style.display = 'none'
                            e.currentTarget.parentElement
                              ?.querySelector('.img-fallback')
                              ?.classList.remove('hidden')
                          }}
                        />
                      ) : null}
                      <div
                        className={`img-fallback flex h-20 w-20 items-center justify-center rounded bg-muted ${item.image_url ? 'hidden' : ''}`}
                      >
                        <Package className="h-8 w-8 text-muted-foreground" />
                      </div>
                    </Link>
                    <div className="min-w-0 flex-1">
                      <Link href={`/products/${item.product_id}`}>
                        <h3 className="line-clamp-2 text-base font-semibold hover:text-primary">
                          {item.name}
                        </h3>
                      </Link>
                      {item.attributes && Object.keys(item.attributes).length > 0 && (
                        <div className="mt-1 flex flex-wrap gap-1">
                          {Object.entries(item.attributes).map(([key, value]) => (
                            <span key={key} className="rounded bg-muted px-2 py-0.5 text-xs">
                              {key}: {value}
                            </span>
                          ))}
                        </div>
                      )}
                      {!item.is_available && (
                        <div className="mt-1 flex items-center gap-1 text-xs text-red-500">
                          <AlertCircle className="h-3 w-3" />
                          {t.cart.outOfStock}
                        </div>
                      )}
                      <div className="mt-2 flex items-center justify-between">
                        <span className="font-bold text-red-600">
                          {formatPrice(item.price_minor, currency)}
                        </span>
                        <div className="flex items-center gap-2">
                          <Button
                            variant="outline"
                            size="icon"
                            className="h-8 w-8"
                            onClick={() => handleQuantityChange(item.id, item.quantity - 1)}
                            disabled={item.quantity <= 1}
                            aria-label={t.cart.decreaseQuantity}
                            title={t.cart.decreaseQuantity}
                          >
                            <Minus className="h-4 w-4" />
                            <span className="sr-only">{t.cart.decreaseQuantity}</span>
                          </Button>
                          <Input
                            type="number"
                            value={item.quantity}
                            onChange={(e) => {
                              const val = parseInt(e.target.value)
                              if (!isNaN(val) && val >= 1 && val <= getItemMaxQuantity(item))
                                handleQuantityChange(item.id, val)
                            }}
                            className="h-8 w-16 text-center"
                            min={1}
                            max={getItemMaxQuantity(item)}
                            aria-label={t.cart.items}
                          />
                          <Button
                            variant="outline"
                            size="icon"
                            className="h-8 w-8"
                            onClick={() => handleQuantityChange(item.id, item.quantity + 1)}
                            disabled={item.quantity >= getItemMaxQuantity(item)}
                            aria-label={t.cart.increaseQuantity}
                            title={t.cart.increaseQuantity}
                          >
                            <Plus className="h-4 w-4" />
                            <span className="sr-only">{t.cart.increaseQuantity}</span>
                          </Button>
                        </div>
                      </div>
                    </div>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="shrink-0 text-muted-foreground hover:text-red-500"
                      onClick={() => requestRemoveItem(item.id)}
                      aria-label={t.cart.removeItem}
                      title={t.cart.removeItem}
                    >
                      <Trash2 className="h-4 w-4" />
                      <span className="sr-only">{t.cart.removeItem}</span>
                    </Button>
                  </div>
                )}
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      {/* 购物车列表 - 卡片视图 */}
      {viewMode === 'card' && (
        <div
          className={
            isPhone
              ? 'grid grid-cols-1 gap-4'
              : isTablet
                ? 'grid grid-cols-2 gap-4'
              : 'grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3'
          }
        >
          {items.map((item) => (
            <Card
              key={item.id}
              className={`flex h-full flex-col transition-all hover:shadow-lg ${!item.is_available ? 'opacity-60' : ''}`}
            >
              <CardHeader className="pb-3">
                <div className="flex items-center justify-between gap-2">
                  <label className="flex cursor-pointer items-center gap-2">
                    <Checkbox
                      checked={selectedItems.has(item.id)}
                      onCheckedChange={() => handleSelectItem(item.id)}
                      disabled={!item.is_available}
                    />
                    <span className="text-xs text-muted-foreground">{t.cart.select}</span>
                  </label>
                  <div className="flex items-center gap-1">
                    {!item.is_available && (
                      <span className="flex items-center gap-1 text-xs text-red-500">
                        <AlertCircle className="h-3 w-3" />
                        {t.cart.outOfStock}
                      </span>
                    )}
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-7 w-7 text-muted-foreground hover:text-red-500"
                      onClick={() => requestRemoveItem(item.id)}
                      aria-label={t.cart.removeItem}
                      title={t.cart.removeItem}
                    >
                      <Trash2 className="h-4 w-4" />
                      <span className="sr-only">{t.cart.removeItem}</span>
                    </Button>
                  </div>
                </div>
              </CardHeader>

              <CardContent className="flex-1 space-y-3">
                {/* 商品信息 */}
                <div className="flex items-start gap-3">
                  {/* 商品图片 */}
                  <Link href={`/products/${item.product_id}`} className="shrink-0">
                    {item.image_url ? (
                      <div className="h-16 w-16 overflow-hidden rounded bg-muted">
                        <img
                          src={item.image_url}
                          alt={item.name}
                          className="h-full w-full object-cover"
                          onError={(e) => {
                            e.currentTarget.style.display = 'none'
                            e.currentTarget.parentElement
                              ?.querySelector('.img-fallback')
                              ?.classList.remove('hidden')
                          }}
                        />
                        <div className="img-fallback flex hidden h-full w-full items-center justify-center">
                          <Package className="h-8 w-8 text-muted-foreground" />
                        </div>
                      </div>
                    ) : (
                      <div className="flex h-16 w-16 items-center justify-center rounded bg-muted">
                        <Package className="h-8 w-8 text-muted-foreground" />
                      </div>
                    )}
                  </Link>

                  {/* 商品详情 */}
                  <div className="min-w-0 flex-1">
                    <Link href={`/products/${item.product_id}`}>
                      <h3 className="mb-1 line-clamp-2 text-sm font-medium hover:text-primary">
                        {item.name}
                      </h3>
                    </Link>
                    {item.attributes && Object.keys(item.attributes).length > 0 && (
                      <div className="flex flex-wrap gap-1">
                        {Object.entries(item.attributes)
                          .slice(0, 2)
                          .map(([key, value]) => (
                            <span key={key} className="rounded bg-muted px-1.5 py-0.5 text-xs">
                              {key}: {value}
                            </span>
                          ))}
                      </div>
                    )}
                  </div>
                </div>

                {/* 价格和数量 */}
                <div className="flex items-center justify-between border-t pt-2">
                  <span className="font-bold text-red-600">
                    {formatPrice(item.price_minor, currency)}
                  </span>
                  <div className="flex items-center gap-1">
                    <Button
                      variant="outline"
                      size="icon"
                      className="h-7 w-7"
                      onClick={() => handleQuantityChange(item.id, item.quantity - 1)}
                      disabled={item.quantity <= 1}
                      aria-label={t.cart.decreaseQuantity}
                      title={t.cart.decreaseQuantity}
                    >
                      <Minus className="h-3 w-3" />
                      <span className="sr-only">{t.cart.decreaseQuantity}</span>
                    </Button>
                    <Input
                      type="number"
                      value={item.quantity}
                      onChange={(e) => {
                        const val = parseInt(e.target.value)
                        if (!isNaN(val) && val >= 1 && val <= getItemMaxQuantity(item))
                          handleQuantityChange(item.id, val)
                      }}
                      className="h-7 w-12 px-1 text-center text-sm"
                      min={1}
                      max={getItemMaxQuantity(item)}
                      aria-label={t.cart.items}
                    />
                    <Button
                      variant="outline"
                      size="icon"
                      className="h-7 w-7"
                      onClick={() => handleQuantityChange(item.id, item.quantity + 1)}
                      disabled={item.quantity >= getItemMaxQuantity(item)}
                      aria-label={t.cart.increaseQuantity}
                      title={t.cart.increaseQuantity}
                    >
                      <Plus className="h-3 w-3" />
                      <span className="sr-only">{t.cart.increaseQuantity}</span>
                    </Button>
                  </div>
                </div>

                {/* 小计 */}
                <div className="flex items-center justify-between text-sm">
                  <span className="text-muted-foreground">{t.cart.subtotal}</span>
                  <span className="font-semibold text-primary">
                    {formatPrice(item.price_minor * item.quantity, currency)}
                  </span>
                </div>
              </CardContent>

              <CardFooter className="pt-3">
                <Button asChild variant="outline" size="sm" className="w-full">
                  <Link href={`/products/${item.product_id}`}>{t.cart.viewProduct}</Link>
                </Button>
              </CardFooter>
            </Card>
          ))}
        </div>
      )}

      <PluginSlot slot="user.cart.before_checkout" context={userCartPluginContext} />

      {/* 结算栏 - 悬浮卡片固定在底部 */}
      <div
        className={`fixed left-[var(--user-layout-sidebar-offset,0px)] right-0 z-40 transition-[left] ${isPhone ? 'bottom-16 px-0' : 'bottom-6 px-6'}`}
      >
        <Card
          className={
            isPhone
              ? 'rounded-none border-x-0 shadow-lg'
              : 'rounded-lg border shadow-xl'
          }
        >
          <CardContent className={isMobile ? 'p-3' : 'p-3 md:p-4'}>
            <PluginSlot
              slot="user.cart.checkout.top"
              context={{ ...userCartPluginContext, section: 'checkout' }}
            />
            {/* 移动端：优惠码输入行（展开时显示在上方） */}
            {promoCodeExpanded && !appliedPromo && isMobile && (
              <div className="mb-2 flex items-center gap-2">
                <div className="relative flex-1">
                  <Input
                    value={promoCodeInput}
                    onChange={(e) => setPromoCodeInput(e.target.value)}
                    placeholder={t.promoCode.promoCodePlaceholder}
                    className="h-8 pr-20 text-sm"
                    maxLength={50}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') handleApplyPromoCode()
                    }}
                    autoFocus
                  />
                  <Button
                    onClick={handleApplyPromoCode}
                    disabled={isGuestMode || !promoCodeInput.trim() || isValidatingPromo}
                    size="sm"
                    className="absolute right-0.5 top-1/2 h-7 -translate-y-1/2 px-3 text-xs"
                  >
                    {isValidatingPromo ? (
                      <Loader2 className="h-3 w-3 animate-spin" />
                    ) : (
                      t.promoCode.apply
                    )}
                  </Button>
                </div>
                <button
                  className="shrink-0 text-xs text-muted-foreground hover:text-foreground"
                  onClick={() => {
                    setPromoCodeExpanded(false)
                    setPromoCodeInput('')
                  }}
                >
                  {t.common.cancel}
                </button>
              </div>
            )}
            {/* 移动端：已应用优惠码信息 */}
            {appliedPromo && isMobile && (
              <div className="mb-2 flex items-center gap-2">
                <span className="text-xs font-medium text-green-600 dark:text-green-400">
                  {appliedPromo.code}: -{formatPrice(promoDiscount, currency)}
                </span>
                <button
                  className="text-xs text-red-500 underline hover:text-red-600"
                  onClick={handleRemovePromoCode}
                >
                  {t.promoCode.remove}
                </button>
              </div>
            )}
            <PluginSlot
              slot="user.cart.checkout.promo.after"
              context={{ ...userCartPluginContext, section: 'checkout_promo' }}
            />
            <PluginSlot
              slot="user.cart.checkout.submit.before"
              context={{ ...userCartPluginContext, section: 'checkout_submit' }}
            />
            <div className={isMobile ? 'flex items-center justify-between gap-2' : 'flex items-center justify-between gap-2 md:gap-4'}>
              {/* 左侧：全选 + 优惠码 */}
              <div className={isMobile ? 'flex min-w-0 items-center gap-2' : 'flex min-w-0 items-center gap-2 md:gap-3'}>
                <label className="flex shrink-0 cursor-pointer items-center gap-2">
                  <Checkbox checked={allAvailableItemsSelected} onCheckedChange={handleSelectAll} />
                  <span className={isMobile ? 'text-xs' : 'text-xs md:text-sm'}>{t.cart.selectAll}</span>
                </label>
                {/* 优惠码触发文字（未展开且未应用时显示） */}
                {!promoCodeExpanded && !appliedPromo && (
                  <button
                    className={
                      isMobile
                        ? 'truncate whitespace-nowrap text-xs font-medium text-primary hover:text-primary/80'
                        : 'truncate whitespace-nowrap text-xs font-medium text-primary hover:text-primary/80 md:text-sm'
                    }
                    onClick={() => setPromoCodeExpanded(true)}
                  >
                    {t.cart.havePromoCode}
                  </button>
                )}
                {/* PC端：优惠码输入框内联显示 */}
                {promoCodeExpanded && !appliedPromo && !isMobile && (
                  <div className="hidden items-center gap-2 md:flex">
                    <div className="relative">
                      <Input
                        value={promoCodeInput}
                        onChange={(e) => setPromoCodeInput(e.target.value)}
                        placeholder={t.promoCode.promoCodePlaceholder}
                        className="h-8 w-52 pr-20 text-sm"
                        maxLength={50}
                        onKeyDown={(e) => {
                          if (e.key === 'Enter') handleApplyPromoCode()
                        }}
                        autoFocus
                      />
                      <Button
                        onClick={handleApplyPromoCode}
                        disabled={isGuestMode || !promoCodeInput.trim() || isValidatingPromo}
                        size="sm"
                        className="absolute right-0.5 top-1/2 h-7 -translate-y-1/2 px-3 text-xs"
                      >
                        {isValidatingPromo ? (
                          <Loader2 className="h-3 w-3 animate-spin" />
                        ) : (
                          t.promoCode.apply
                        )}
                      </Button>
                    </div>
                    <button
                      className="shrink-0 text-xs text-muted-foreground hover:text-foreground"
                      onClick={() => {
                        setPromoCodeExpanded(false)
                        setPromoCodeInput('')
                      }}
                    >
                      {t.common.cancel}
                    </button>
                  </div>
                )}
                {/* PC端：已应用优惠码内联显示 */}
                {appliedPromo && !isMobile && (
                  <div className="hidden items-center gap-2 md:flex">
                    <span className="text-sm font-medium text-green-600 dark:text-green-400">
                      {appliedPromo.code}: -{formatPrice(promoDiscount, currency)}
                    </span>
                    <button
                      className="text-xs text-red-500 underline hover:text-red-600"
                      onClick={handleRemovePromoCode}
                    >
                      {t.promoCode.remove}
                    </button>
                  </div>
                )}
              </div>

              {/* 右侧：合计和结算按钮 */}
              <div className={isMobile ? 'flex items-center gap-2' : 'flex items-center gap-2 md:gap-4'}>
                <div className="min-w-0 text-right">
                  <span className="hidden text-xs text-muted-foreground sm:inline">
                    {t.cart.selected}: {selectedItems.size}/{items.length}({selectedTotalQuantity}{' '}
                    {t.cart.pcs})
                  </span>
                  <div
                    className={
                      isMobile
                        ? 'whitespace-nowrap text-sm font-bold'
                        : 'whitespace-nowrap text-sm font-bold md:text-lg'
                    }
                  >
                    <span className="hidden sm:inline">{t.cart.total}:</span>
                    {appliedPromo ? (
                      <>
                        <span className="ml-1 text-xs font-normal text-muted-foreground line-through">
                          {formatPrice(selectedTotalPrice, currency)}
                        </span>
                        <span className="ml-1 text-red-600">
                          {formatPrice(Math.max(0, selectedTotalPrice - promoDiscount), currency)}
                        </span>
                      </>
                    ) : (
                      <span className="ml-1 text-red-600">
                        {formatPrice(selectedTotalPrice, currency)}
                      </span>
                    )}
                  </div>
                </div>
                <Button
                  size="default"
                  className="shrink-0"
                  onClick={handleCheckout}
                  disabled={
                    selectedItems.size === 0 || (!isGuestMode && createOrderMutation.isPending)
                  }
                >
                  {!isGuestMode && createOrderMutation.isPending
                    ? t.cart.submitting
                    : isGuestMode
                      ? t.cart.loginToCheckout
                      : t.cart.checkout}
                </Button>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
      <PluginSlot slot="user.cart.bottom" context={userCartPluginContext} />

      <AlertDialog
        open={!!confirmAction}
        onOpenChange={(open) => {
          if (!open && !isConfirmPending) {
            setConfirmAction(null)
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {confirmAction?.type === 'clear_selected'
                ? t.cart.confirmClearSelected
                : t.cart.confirmDeleteCartItem}
            </AlertDialogTitle>
          <AlertDialogDescription>
            {confirmAction?.type === 'remove_item'
              ? confirmAction.itemName
              : `${confirmAction?.count || 0} ${t.cart.items}`}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <PluginSlot
          slot="user.cart.confirm_dialog.before"
          context={{ ...userCartPluginContext, section: 'confirm_dialog' }}
        />
        <AlertDialogFooter>
          <AlertDialogCancel disabled={isConfirmPending}>{t.common.cancel}</AlertDialogCancel>
          <AlertDialogAction
              disabled={isConfirmPending}
              onClick={(event) => {
                event.preventDefault()
                void handleConfirmAction()
              }}
            >
              {isConfirmPending ? t.common.processing : t.common.confirm}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
