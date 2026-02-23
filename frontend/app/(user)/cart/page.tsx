'use client'

import { useState, useMemo } from 'react'
import { useRouter } from 'next/navigation'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useCart } from '@/contexts/cart-context'
import { createOrder, validatePromoCode } from '@/lib/api'
import { Card, CardContent, CardHeader, CardFooter } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Trash2, Minus, Plus, ShoppingCart, Package, AlertCircle, RefreshCw, LayoutGrid, LayoutList, Loader2 } from 'lucide-react'
import Link from 'next/link'
import { useLocale } from '@/hooks/use-locale'
import { usePageTitle } from '@/hooks/use-page-title'
import { getTranslations } from '@/lib/i18n'
import toast from 'react-hot-toast'
import { Checkbox } from '@/components/ui/checkbox'
import { useCurrency, formatPrice } from '@/contexts/currency-context'

export default function CartPage() {
  const router = useRouter()
  const queryClient = useQueryClient()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.cart)
  const { currency } = useCurrency()
  const { items, totalPrice, totalQuantity, isLoading, updateQuantity, removeItem, removeItems, refetch } = useCart()
  const [selectedItems, setSelectedItems] = useState<Set<number>>(new Set())
  const [viewMode, setViewMode] = useState<'list' | 'card'>(() => {
    if (typeof window !== 'undefined') {
      const saved = localStorage.getItem('cart-view-mode')
      if (saved === 'list' || saved === 'card') return saved
    }
    return 'list'
  })

  // Promo code state
  const [promoCodeInput, setPromoCodeInput] = useState('')
  const [promoCodeExpanded, setPromoCodeExpanded] = useState(false)
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

  const handleViewModeChange = (mode: 'list' | 'card') => {
    setViewMode(mode)
    localStorage.setItem('cart-view-mode', mode)
  }

  // 应用优惠码
  const handleApplyPromoCode = async () => {
    if (!promoCodeInput.trim()) return

    setIsValidatingPromo(true)
    try {
      const selectedCartItems = items.filter(item => selectedItems.has(item.id) && item.is_available)
      const productIds = selectedCartItems.map(item => item.product_id)
      const amount = selectedCartItems.reduce((sum, item) => sum + item.price * item.quantity, 0)

      const response = await validatePromoCode({
        code: promoCodeInput.trim(),
        product_ids: productIds.length > 0 ? productIds : undefined,
        amount: amount > 0 ? amount : undefined,
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
    onError: (error: Error) => {
      toast.error(error.message || t.cart.orderFailed)
    },
  })

  // 处理数量变化
  const handleQuantityChange = async (itemId: number, newQuantity: number) => {
    if (newQuantity < 1) return
    try {
      await updateQuantity(itemId, newQuantity)
    } catch (error) {
      // 错误已在 context 中处理
    }
  }

  // 处理删除
  const handleRemove = async (itemId: number) => {
    try {
      await removeItem(itemId)
      setSelectedItems(prev => {
        const next = new Set(prev)
        next.delete(itemId)
        return next
      })
    } catch (error) {
      // 错误已在 context 中处理
    }
  }

  // 处理全选
  const handleSelectAll = () => {
    if (selectedItems.size === items.length) {
      setSelectedItems(new Set())
    } else {
      setSelectedItems(new Set(items.map(item => item.id)))
    }
  }

  // 处理单选
  const handleSelectItem = (itemId: number) => {
    setSelectedItems(prev => {
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
    .filter(item => selectedItems.has(item.id) && item.is_available)
    .reduce((sum, item) => sum + item.price * item.quantity, 0)

  const selectedTotalQuantity = items
    .filter(item => selectedItems.has(item.id) && item.is_available)
    .reduce((sum, item) => sum + item.quantity, 0)

  // 实时计算优惠码折扣
  const promoDiscount = useMemo(() => {
    if (!appliedPromo || selectedTotalPrice <= 0) return 0

    if (appliedPromo.discount_type === 'percentage') {
      let discount = selectedTotalPrice * appliedPromo.discount_value / 100
      if (appliedPromo.max_discount > 0 && discount > appliedPromo.max_discount) {
        discount = appliedPromo.max_discount
      }
      return Math.min(discount, selectedTotalPrice)
    } else {
      return Math.min(appliedPromo.discount_value, selectedTotalPrice)
    }
  }, [appliedPromo, selectedTotalPrice])

  // 提交订单
  const handleCheckout = () => {
    const selectedCartItems = items.filter(item => selectedItems.has(item.id) && item.is_available)

    if (selectedCartItems.length === 0) {
      toast.error(t.cart.selectItems)
      return
    }

    const orderItems = selectedCartItems.map(item => ({
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
        <h1 className="text-2xl md:text-3xl font-bold">
          {t.cart.title}
        </h1>
        <div className="space-y-4">
          {[...Array(3)].map((_, i) => (
            <Card key={i} className="animate-pulse">
              <CardContent className="p-4">
                <div className="flex gap-4">
                  <div className="w-20 h-20 bg-muted rounded" />
                  <div className="flex-1 space-y-2">
                    <div className="h-4 bg-muted rounded w-3/4" />
                    <div className="h-4 bg-muted rounded w-1/2" />
                  </div>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      </div>
    )
  }

  if (items.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center min-h-[60vh]">
        <ShoppingCart className="w-16 h-16 text-muted-foreground mb-4" />
        <h1 className="text-xl md:text-2xl font-bold mb-2">
          {t.cart.empty}
        </h1>
        <p className="text-muted-foreground mb-6">
          {t.cart.emptyDesc}
        </p>
        <Button asChild>
          <Link href="/products">
            {t.cart.goShopping}
          </Link>
        </Button>
      </div>
    )
  }

  return (
    <div className="space-y-6 pb-28 md:pb-24">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl md:text-3xl font-bold">
          {t.cart.title}
          <span className="hidden md:inline text-lg font-normal text-muted-foreground ml-2">
            ({items.length} {t.cart.items})
          </span>
        </h1>
        <div className="flex items-center gap-2">
          <div className="hidden md:flex items-center border rounded-lg p-0.5">
            <Button
              variant={viewMode === 'list' ? 'secondary' : 'ghost'}
              size="sm"
              className="h-7 px-2"
              onClick={() => handleViewModeChange('list')}
            >
              <LayoutList className="h-4 w-4" />
            </Button>
            <Button
              variant={viewMode === 'card' ? 'secondary' : 'ghost'}
              size="sm"
              className="h-7 px-2"
              onClick={() => handleViewModeChange('card')}
            >
              <LayoutGrid className="h-4 w-4" />
            </Button>
          </div>
          <Button variant="outline" size="sm" onClick={() => refetch()}>
            <RefreshCw className="h-4 w-4 md:mr-2" />
            <span className="hidden md:inline">{t.cart.refresh}</span>
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              if (selectedItems.size === 0) {
                toast.error(t.cart.noItemsSelected)
                return
              }
              removeItems(Array.from(selectedItems))
              setSelectedItems(new Set())
            }}
            disabled={selectedItems.size === 0}
          >
            <Trash2 className="h-4 w-4 md:mr-2" />
            <span className="hidden md:inline">{t.cart.clearSelected}</span>
          </Button>
        </div>
      </div>

      {/* 购物车列表 - 列表视图 */}
      {viewMode === 'list' && (
        <div className="space-y-4">
          {items.map((item) => (
            <Card key={item.id} className={!item.is_available ? 'opacity-60' : ''}>
              <CardContent className="p-3 md:p-4">
                {/* 移动端布局 */}
                <div className="md:hidden">
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
                        <img src={item.image_url} alt={item.name} className="w-16 h-16 object-cover rounded"
                          onError={(e) => {
                            e.currentTarget.style.display = 'none'
                            e.currentTarget.parentElement?.querySelector('.img-fallback')?.classList.remove('hidden')
                          }}
                        />
                      ) : null}
                      <div className={`img-fallback w-16 h-16 bg-muted rounded flex items-center justify-center ${item.image_url ? 'hidden' : ''}`}>
                        <Package className="w-6 h-6 text-muted-foreground" />
                      </div>
                    </Link>
                    <div className="flex-1 min-w-0">
                      <div className="flex justify-between items-start gap-2">
                        <Link href={`/products/${item.product_id}`} className="flex-1 min-w-0">
                          <h3 className="font-semibold text-sm line-clamp-2 hover:text-primary">{item.name}</h3>
                        </Link>
                        <Button variant="ghost" size="icon" className="shrink-0 text-muted-foreground hover:text-red-500 -mt-1 -mr-2 h-7 w-7" onClick={() => handleRemove(item.id)}>
                          <Trash2 className="h-4 w-4" />
                        </Button>
                      </div>
                      {item.attributes && Object.keys(item.attributes).length > 0 && (
                        <div className="flex flex-wrap gap-1 mt-1">
                          {Object.entries(item.attributes).map(([key, value]) => (
                            <span key={key} className="text-xs px-1.5 py-0.5 bg-muted rounded">{key}: {value}</span>
                          ))}
                        </div>
                      )}
                      {!item.is_available && (
                        <div className="flex items-center gap-1 mt-1 text-red-500 text-xs">
                          <AlertCircle className="w-3 h-3" />
                          {t.cart.outOfStock}
                        </div>
                      )}
                    </div>
                  </div>
                  {/* 第二行：价格 + 数量控制 */}
                  <div className="flex items-center justify-between mt-2 pl-7">
                    <span className="text-red-600 font-bold">{formatPrice(item.price, currency)}</span>
                    <div className="flex items-center gap-1">
                      <Button variant="outline" size="icon" className="h-7 w-7" onClick={() => handleQuantityChange(item.id, item.quantity - 1)} disabled={item.quantity <= 1}>
                        <Minus className="h-3 w-3" />
                      </Button>
                      <Input
                        type="number"
                        value={item.quantity}
                        onChange={(e) => { const val = parseInt(e.target.value); if (!isNaN(val) && val >= 1) handleQuantityChange(item.id, val) }}
                        className="w-10 h-7 text-center px-0 text-sm"
                        min={1}
                        max={item.available_stock}
                      />
                      <Button variant="outline" size="icon" className="h-7 w-7" onClick={() => handleQuantityChange(item.id, item.quantity + 1)} disabled={item.quantity >= item.available_stock}>
                        <Plus className="h-3 w-3" />
                      </Button>
                    </div>
                  </div>
                </div>

                {/* 桌面端布局 */}
                <div className="hidden md:flex gap-4">
                  <div className="flex items-center">
                    <Checkbox
                      checked={selectedItems.has(item.id)}
                      onCheckedChange={() => handleSelectItem(item.id)}
                      disabled={!item.is_available}
                    />
                  </div>
                  <Link href={`/products/${item.product_id}`} className="shrink-0">
                    {item.image_url ? (
                      <img src={item.image_url} alt={item.name} className="w-20 h-20 object-cover rounded"
                        onError={(e) => {
                          e.currentTarget.style.display = 'none'
                          e.currentTarget.parentElement?.querySelector('.img-fallback')?.classList.remove('hidden')
                        }}
                      />
                    ) : null}
                    <div className={`img-fallback w-20 h-20 bg-muted rounded flex items-center justify-center ${item.image_url ? 'hidden' : ''}`}>
                      <Package className="w-8 h-8 text-muted-foreground" />
                    </div>
                  </Link>
                  <div className="flex-1 min-w-0">
                    <Link href={`/products/${item.product_id}`}>
                      <h3 className="font-semibold text-base line-clamp-2 hover:text-primary">{item.name}</h3>
                    </Link>
                    {item.attributes && Object.keys(item.attributes).length > 0 && (
                      <div className="flex flex-wrap gap-1 mt-1">
                        {Object.entries(item.attributes).map(([key, value]) => (
                          <span key={key} className="text-xs px-2 py-0.5 bg-muted rounded">{key}: {value}</span>
                        ))}
                      </div>
                    )}
                    {!item.is_available && (
                      <div className="flex items-center gap-1 mt-1 text-red-500 text-xs">
                        <AlertCircle className="w-3 h-3" />
                        {t.cart.outOfStock}
                      </div>
                    )}
                    <div className="flex items-center justify-between mt-2">
                      <span className="text-red-600 font-bold">{formatPrice(item.price, currency)}</span>
                      <div className="flex items-center gap-2">
                        <Button variant="outline" size="icon" className="h-8 w-8" onClick={() => handleQuantityChange(item.id, item.quantity - 1)} disabled={item.quantity <= 1}>
                          <Minus className="h-4 w-4" />
                        </Button>
                        <Input
                          type="number"
                          value={item.quantity}
                          onChange={(e) => { const val = parseInt(e.target.value); if (!isNaN(val) && val >= 1) handleQuantityChange(item.id, val) }}
                          className="w-16 h-8 text-center"
                          min={1}
                          max={item.available_stock}
                        />
                        <Button variant="outline" size="icon" className="h-8 w-8" onClick={() => handleQuantityChange(item.id, item.quantity + 1)} disabled={item.quantity >= item.available_stock}>
                          <Plus className="h-4 w-4" />
                        </Button>
                      </div>
                    </div>
                  </div>
                  <Button variant="ghost" size="icon" className="shrink-0 text-muted-foreground hover:text-red-500" onClick={() => handleRemove(item.id)}>
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      {/* 购物车列表 - 卡片视图 */}
      {viewMode === 'card' && (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {items.map((item) => (
            <Card key={item.id} className={`hover:shadow-lg transition-all flex flex-col h-full ${!item.is_available ? 'opacity-60' : ''}`}>
              <CardHeader className="pb-3">
                <div className="flex items-center justify-between gap-2">
                  <label className="flex items-center gap-2 cursor-pointer">
                    <Checkbox
                      checked={selectedItems.has(item.id)}
                      onCheckedChange={() => handleSelectItem(item.id)}
                      disabled={!item.is_available}
                    />
                    <span className="text-xs text-muted-foreground">
                      {t.cart.select}
                    </span>
                  </label>
                  <div className="flex items-center gap-1">
                    {!item.is_available && (
                      <span className="text-xs text-red-500 flex items-center gap-1">
                        <AlertCircle className="w-3 h-3" />
                        {t.cart.outOfStock}
                      </span>
                    )}
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-7 w-7 text-muted-foreground hover:text-red-500"
                      onClick={() => handleRemove(item.id)}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                </div>
              </CardHeader>

              <CardContent className="space-y-3 flex-1">
                {/* 商品信息 */}
                <div className="flex items-start gap-3">
                  {/* 商品图片 */}
                  <Link href={`/products/${item.product_id}`} className="shrink-0">
                    {item.image_url ? (
                      <div className="w-16 h-16 rounded overflow-hidden bg-muted">
                        <img src={item.image_url} alt={item.name} className="w-full h-full object-cover"
                          onError={(e) => {
                            e.currentTarget.style.display = 'none'
                            e.currentTarget.parentElement?.querySelector('.img-fallback')?.classList.remove('hidden')
                          }}
                        />
                        <div className="img-fallback w-full h-full flex items-center justify-center hidden">
                          <Package className="w-8 h-8 text-muted-foreground" />
                        </div>
                      </div>
                    ) : (
                      <div className="w-16 h-16 rounded bg-muted flex items-center justify-center">
                        <Package className="w-8 h-8 text-muted-foreground" />
                      </div>
                    )}
                  </Link>

                  {/* 商品详情 */}
                  <div className="flex-1 min-w-0">
                    <Link href={`/products/${item.product_id}`}>
                      <h3 className="font-medium text-sm line-clamp-2 hover:text-primary mb-1">{item.name}</h3>
                    </Link>
                    {item.attributes && Object.keys(item.attributes).length > 0 && (
                      <div className="flex flex-wrap gap-1">
                        {Object.entries(item.attributes).slice(0, 2).map(([key, value]) => (
                          <span key={key} className="text-xs px-1.5 py-0.5 bg-muted rounded">{key}: {value}</span>
                        ))}
                      </div>
                    )}
                  </div>
                </div>

                {/* 价格和数量 */}
                <div className="flex items-center justify-between pt-2 border-t">
                  <span className="text-red-600 font-bold">{formatPrice(item.price, currency)}</span>
                  <div className="flex items-center gap-1">
                    <Button variant="outline" size="icon" className="h-7 w-7" onClick={() => handleQuantityChange(item.id, item.quantity - 1)} disabled={item.quantity <= 1}>
                      <Minus className="h-3 w-3" />
                    </Button>
                    <Input
                      type="number"
                      value={item.quantity}
                      onChange={(e) => { const val = parseInt(e.target.value); if (!isNaN(val) && val >= 1) handleQuantityChange(item.id, val) }}
                      className="w-12 h-7 text-center px-1 text-sm"
                      min={1}
                      max={item.available_stock}
                    />
                    <Button variant="outline" size="icon" className="h-7 w-7" onClick={() => handleQuantityChange(item.id, item.quantity + 1)} disabled={item.quantity >= item.available_stock}>
                      <Plus className="h-3 w-3" />
                    </Button>
                  </div>
                </div>

                {/* 小计 */}
                <div className="flex items-center justify-between text-sm">
                  <span className="text-muted-foreground">{t.cart.subtotal}</span>
                  <span className="font-semibold text-primary">{formatPrice(item.price * item.quantity, currency)}</span>
                </div>
              </CardContent>

              <CardFooter className="pt-3">
                <Button asChild variant="outline" size="sm" className="w-full">
                  <Link href={`/products/${item.product_id}`}>
                    {t.cart.viewProduct}
                  </Link>
                </Button>
              </CardFooter>
            </Card>
          ))}
        </div>
      )}

      {/* 结算栏 - 悬浮卡片固定在底部 */}
      <div className="fixed bottom-16 md:bottom-6 left-0 md:left-64 right-0 z-40 px-0 md:px-6">
        <Card className="rounded-none md:rounded-lg border-x-0 md:border shadow-lg md:shadow-xl">
          <CardContent className="p-3 md:p-4">
            {/* 移动端：优惠码输入行（展开时显示在上方） */}
            {promoCodeExpanded && !appliedPromo && (
              <div className="flex items-center gap-2 mb-2 md:hidden">
                <div className="relative flex-1">
                  <Input
                    value={promoCodeInput}
                    onChange={(e) => setPromoCodeInput(e.target.value)}
                    placeholder={t.promoCode.promoCodePlaceholder}
                    className="pr-20 h-8 text-sm"
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') handleApplyPromoCode()
                    }}
                    autoFocus
                  />
                  <Button
                    onClick={handleApplyPromoCode}
                    disabled={!promoCodeInput.trim() || isValidatingPromo}
                    size="sm"
                    className="absolute right-0.5 top-1/2 -translate-y-1/2 h-7 text-xs px-3"
                  >
                    {isValidatingPromo ? <Loader2 className="h-3 w-3 animate-spin" /> : t.promoCode.apply}
                  </Button>
                </div>
                <button
                  className="text-xs text-muted-foreground hover:text-foreground shrink-0"
                  onClick={() => { setPromoCodeExpanded(false); setPromoCodeInput('') }}
                >
                  {t.common.cancel}
                </button>
              </div>
            )}
            {/* 移动端：已应用优惠码信息 */}
            {appliedPromo && (
              <div className="flex items-center gap-2 mb-2 md:hidden">
                <span className="text-xs text-green-600 dark:text-green-400 font-medium">
                  {appliedPromo.code}: -{formatPrice(promoDiscount, currency)}
                </span>
                <button
                  className="text-xs text-red-500 hover:text-red-600 underline"
                  onClick={handleRemovePromoCode}
                >
                  {t.promoCode.remove}
                </button>
              </div>
            )}
            <div className="flex items-center justify-between gap-2 md:gap-4">
              {/* 左侧：全选 + 优惠码 */}
              <div className="flex items-center gap-2 md:gap-3 min-w-0">
                <label className="flex items-center gap-2 cursor-pointer shrink-0">
                  <Checkbox
                    checked={selectedItems.size === items.length && items.length > 0}
                    onCheckedChange={handleSelectAll}
                  />
                  <span className="text-xs md:text-sm">
                    {t.cart.selectAll}
                  </span>
                </label>
                {/* 优惠码触发文字（未展开且未应用时显示） */}
                {!promoCodeExpanded && !appliedPromo && (
                  <button
                    className="text-xs md:text-sm text-primary hover:text-primary/80 font-medium whitespace-nowrap truncate"
                    onClick={() => setPromoCodeExpanded(true)}
                  >
                    {t.cart.havePromoCode}
                  </button>
                )}
                {/* PC端：优惠码输入框内联显示 */}
                {promoCodeExpanded && !appliedPromo && (
                  <div className="hidden md:flex items-center gap-2">
                    <div className="relative">
                      <Input
                        value={promoCodeInput}
                        onChange={(e) => setPromoCodeInput(e.target.value)}
                        placeholder={t.promoCode.promoCodePlaceholder}
                        className="pr-20 h-8 text-sm w-52"
                        onKeyDown={(e) => {
                          if (e.key === 'Enter') handleApplyPromoCode()
                        }}
                        autoFocus
                      />
                      <Button
                        onClick={handleApplyPromoCode}
                        disabled={!promoCodeInput.trim() || isValidatingPromo}
                        size="sm"
                        className="absolute right-0.5 top-1/2 -translate-y-1/2 h-7 text-xs px-3"
                      >
                        {isValidatingPromo ? <Loader2 className="h-3 w-3 animate-spin" /> : t.promoCode.apply}
                      </Button>
                    </div>
                    <button
                      className="text-xs text-muted-foreground hover:text-foreground shrink-0"
                      onClick={() => { setPromoCodeExpanded(false); setPromoCodeInput('') }}
                    >
                      {t.common.cancel}
                    </button>
                  </div>
                )}
                {/* PC端：已应用优惠码内联显示 */}
                {appliedPromo && (
                  <div className="hidden md:flex items-center gap-2">
                    <span className="text-sm text-green-600 dark:text-green-400 font-medium">
                      {appliedPromo.code}: -{formatPrice(promoDiscount, currency)}
                    </span>
                    <button
                      className="text-xs text-red-500 hover:text-red-600 underline"
                      onClick={handleRemovePromoCode}
                    >
                      {t.promoCode.remove}
                    </button>
                  </div>
                )}
              </div>

              {/* 右侧：合计和结算按钮 */}
              <div className="flex items-center gap-2 md:gap-4">
                <div className="text-right min-w-0">
                  <span className="text-xs text-muted-foreground hidden sm:inline">
                    {t.cart.selected}: {selectedItems.size}/{items.length}
                    ({selectedTotalQuantity} {t.cart.pcs})
                  </span>
                  <div className="text-sm md:text-lg font-bold whitespace-nowrap">
                    <span className="hidden sm:inline">{t.cart.total}:</span>
                    {appliedPromo ? (
                      <>
                        <span className="text-muted-foreground line-through text-xs ml-1 font-normal">
                          {formatPrice(selectedTotalPrice, currency)}
                        </span>
                        <span className="text-red-600 ml-1">
                          {formatPrice(Math.max(0, selectedTotalPrice - promoDiscount), currency)}
                        </span>
                      </>
                    ) : (
                      <span className="text-red-600 ml-1">{formatPrice(selectedTotalPrice, currency)}</span>
                    )}
                  </div>
                </div>
                <Button
                  size="default"
                  className="shrink-0"
                  onClick={handleCheckout}
                  disabled={selectedItems.size === 0 || createOrderMutation.isPending}
                >
                  {createOrderMutation.isPending
                    ? t.cart.submitting
                    : t.cart.checkout}
                </Button>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
