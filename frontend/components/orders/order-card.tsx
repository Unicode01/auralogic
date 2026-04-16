'use client'
/* eslint-disable @next/next/no-img-element */

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import Link from 'next/link'
import { Card, CardHeader, CardContent, CardFooter } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { OrderStatusBadge } from './order-status-badge'
import type { Order } from '@/types/order'
import { cn, formatDate, formatCurrency } from '@/lib/utils'
import {
  Package,
  Truck,
  Shield,
  FileEdit,
  AlertCircle,
  Loader2,
  Key,
  Headphones,
  CreditCard,
} from 'lucide-react'
import { getOrRefreshFormToken } from '@/lib/api'
import { resolveApiErrorMessage } from '@/lib/api-error'
import toast from 'react-hot-toast'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { PluginSlot } from '@/components/plugins/plugin-slot'
import type { PluginSlotBatchBoundaryItem } from '@/lib/plugin-slot-batch'

interface OrderCardProps {
  order: Order
  highlighted?: boolean
  onOpenDetail?: (orderNo: string) => void
  pluginSlotNamespace?: string
  pluginSlotContext?: Record<string, any>
  pluginSlotPath?: string
  rowIndex?: number
}

type BuildOrderCardPluginContextOptions = {
  order: Order
  highlighted?: boolean
  pluginSlotContext?: Record<string, any>
  rowIndex?: number
  isGettingToken?: boolean
}

type BuildOrderCardBatchItemsOptions = {
  order: Order
  highlighted?: boolean
  pluginSlotNamespace?: string
  pluginSlotContext?: Record<string, any>
  pluginSlotPath?: string
  rowIndex?: number
}

function getOrderCardFlags(order: Order) {
  const orderItems = Array.isArray(order.items) ? order.items : []
  const isDraft = order.status === 'draft'
  const isNeedResubmit = order.status === 'need_resubmit'
  const isPendingPayment = order.status === 'pending_payment'
  const isVirtualOnly = orderItems.every(
    (item) => (item.product_type || item.productType) === 'virtual'
  )
  const needsFilling = (isDraft || isNeedResubmit) && !isVirtualOnly

  return {
    orderItems,
    isDraft,
    isNeedResubmit,
    isPendingPayment,
    isVirtualOnly,
    needsFilling,
  }
}

export function buildOrderCardPluginContext({
  order,
  highlighted,
  pluginSlotContext,
  rowIndex,
  isGettingToken = false,
}: BuildOrderCardPluginContextOptions): Record<string, any> {
  const orderNo = order.orderNo || order.order_no || ''
  const { orderItems, isDraft, isNeedResubmit, isPendingPayment, isVirtualOnly, needsFilling } =
    getOrderCardFlags(order)

  return {
    ...(pluginSlotContext || {}),
    order: {
      id: order.id,
      order_no: orderNo || undefined,
      status: order.status,
      currency: order.currency,
      total_amount_minor: order.total_amount_minor ?? 0,
      item_count: orderItems.length,
      tracking_no: order.trackingNo || order.tracking_no || undefined,
      created_at: order.createdAt || order.created_at || undefined,
      privacy_protected: Boolean(order.privacyProtected || order.privacy_protected),
      shared_to_support: Boolean(order.sharedToSupport || order.shared_to_support),
    },
    row: {
      index: rowIndex,
      highlighted: Boolean(highlighted),
    },
    summary: {
      visible_item_count: Math.min(orderItems.length, 1),
      hidden_item_count: Math.max(orderItems.length - 1, 0),
      is_getting_form_token: isGettingToken,
    },
    state: {
      highlighted: Boolean(highlighted),
      needs_filling: needsFilling,
      pending_payment: isPendingPayment,
      virtual_only: isVirtualOnly,
      draft: isDraft,
      need_resubmit: isNeedResubmit,
      has_tracking: Boolean(order.trackingNo || order.tracking_no),
    },
  }
}

export function buildOrderCardBatchItems({
  order,
  highlighted,
  pluginSlotNamespace,
  pluginSlotContext,
  pluginSlotPath,
  rowIndex,
}: BuildOrderCardBatchItemsOptions): PluginSlotBatchBoundaryItem[] {
  if (!pluginSlotNamespace) {
    return []
  }

  const orderCardPluginContext = buildOrderCardPluginContext({
    order,
    highlighted,
    pluginSlotContext,
    rowIndex,
  })

  return [
    {
      slot: `${pluginSlotNamespace}.card.badges.after`,
      path: pluginSlotPath,
      hostContext: { ...orderCardPluginContext, section: 'badges' },
    },
    {
      slot: `${pluginSlotNamespace}.card.product.after`,
      path: pluginSlotPath,
      hostContext: { ...orderCardPluginContext, section: 'product' },
    },
    {
      slot: `${pluginSlotNamespace}.card.summary.after`,
      path: pluginSlotPath,
      hostContext: { ...orderCardPluginContext, section: 'summary' },
    },
    {
      slot: `${pluginSlotNamespace}.card.actions.before`,
      path: pluginSlotPath,
      hostContext: { ...orderCardPluginContext, section: 'actions' },
    },
    {
      slot: `${pluginSlotNamespace}.card.actions.after`,
      path: pluginSlotPath,
      hostContext: { ...orderCardPluginContext, section: 'actions' },
    },
  ]
}

export function OrderCard({
  order,
  highlighted,
  onOpenDetail,
  pluginSlotNamespace,
  pluginSlotContext,
  pluginSlotPath,
  rowIndex,
}: OrderCardProps) {
  const router = useRouter()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  const orderNo = order.orderNo || order.order_no || ''
  const { orderItems, isDraft, isNeedResubmit, isPendingPayment, isVirtualOnly, needsFilling } =
    getOrderCardFlags(order)
  const isPrivacyProtected = Boolean(order.privacyProtected || order.privacy_protected)
  const isSharedToSupport = Boolean(order.sharedToSupport || order.shared_to_support)
  const needsFillingMessage = isDraft ? t.order.fillShippingPrompt : t.order.needResubmitShort
  const needsFillingBadgeLabel = isDraft ? t.order.fillShippingShort : t.order.needResubmitShort
  const pendingPaymentMessage = t.order.pendingPaymentPrompt
  const pendingPaymentBadgeLabel = t.order.status.pending_payment
  const shouldHidePrimaryStatusBadge = needsFilling || isPendingPayment
  const hasHeaderBadges =
    !shouldHidePrimaryStatusBadge ||
    needsFilling ||
    isPendingPayment ||
    isPrivacyProtected ||
    isSharedToSupport
  const [isGettingToken, setIsGettingToken] = useState(false)
  const headerStatusBadgeClassName =
    'h-5 shrink-0 rounded px-1.5 text-[11px] font-medium shadow-none'
  const headerMetaBadgeClassName =
    'flex h-5 w-5 shrink-0 items-center justify-center rounded border border-border/60 bg-transparent p-0 text-muted-foreground'

  const handleFillForm = async (e: React.MouseEvent) => {
    e.preventDefault()
    setIsGettingToken(true)

    try {
      const response = await getOrRefreshFormToken(orderNo)
      const formToken = response.data.form_token

      if (formToken) {
        // 跳转到填表页
        router.push(`/form/shipping?token=${formToken}`)
      } else {
        toast.error(t.order.getFormTokenFailed)
      }
    } catch (error: any) {
      toast.error(resolveApiErrorMessage(error, t, t.order.getFormTokenFailed))
    } finally {
      setIsGettingToken(false)
    }
  }

  const handleOpenDetail = () => {
    if (!orderNo) return
    onOpenDetail?.(orderNo)
  }
  const orderCardPluginContext = buildOrderCardPluginContext({
    order,
    highlighted,
    pluginSlotContext,
    rowIndex,
    isGettingToken,
  })

  return (
    <Card
      data-order-no={orderNo}
      className={`flex h-full flex-col border-border/70 shadow-none transition-colors ${
        needsFilling
          ? 'border-amber-300/60 bg-amber-50/20 dark:border-amber-800/50 dark:bg-amber-950/10'
          : isPendingPayment
            ? 'border-orange-300/60 bg-orange-50/20 dark:border-orange-800/50 dark:bg-orange-950/10'
            : ''
      } ${highlighted ? 'border-primary ring-2 ring-primary/70' : 'hover:border-border/90'}`}
    >
      <CardHeader className="pb-2">
        <div className="space-y-2">
          <div className="flex min-w-0 items-start justify-between gap-3">
            <div className="min-w-0 flex-1 space-y-1.5">
              <h3 className="min-w-0 truncate text-sm font-semibold text-foreground">
                {orderNo}
              </h3>
              <p className="text-xs text-muted-foreground">
                {formatDate(order.createdAt || order.created_at || '')}
              </p>
            </div>
            {hasHeaderBadges ? (
              <div className="flex max-w-[58%] shrink-0 flex-wrap items-center justify-end gap-1">
                {!shouldHidePrimaryStatusBadge ? (
                  <OrderStatusBadge status={order.status} className={headerStatusBadgeClassName} />
                ) : null}
                {needsFilling && (
                  <Badge
                    variant="outline"
                    className={cn(
                      headerStatusBadgeClassName,
                      'gap-1 border-amber-300 bg-transparent text-amber-700 dark:border-amber-800 dark:text-amber-300'
                    )}
                    title={needsFillingMessage}
                  >
                    <AlertCircle className="h-3 w-3" />
                    <span className="max-w-[8rem] truncate">{needsFillingBadgeLabel}</span>
                  </Badge>
                )}
                {isPendingPayment && (
                  <Badge
                    variant="outline"
                    className={cn(
                      headerStatusBadgeClassName,
                      'gap-1 border-orange-300 bg-transparent text-orange-700 dark:border-orange-800 dark:text-orange-300'
                    )}
                    title={pendingPaymentMessage}
                  >
                    <CreditCard className="h-3 w-3" />
                    <span className="max-w-[8rem] truncate">{pendingPaymentBadgeLabel}</span>
                  </Badge>
                )}
                {isPrivacyProtected && (
                  <Badge
                    variant="outline"
                    className={headerMetaBadgeClassName}
                    title={t.order.privacyProtected}
                  >
                    <Shield className="h-3 w-3" />
                    <span className="sr-only">{t.order.privacyProtected}</span>
                  </Badge>
                )}
                {isSharedToSupport && (
                  <Badge
                    variant="secondary"
                    className={cn(
                      headerMetaBadgeClassName,
                      'border-sky-300/70 text-sky-700 dark:border-sky-800/70 dark:text-sky-300'
                    )}
                    title={t.order.sharedToSupport}
                  >
                    <Headphones className="h-3 w-3" />
                    <span className="sr-only">{t.order.sharedToSupport}</span>
                  </Badge>
                )}
              </div>
            ) : null}
          </div>
          {pluginSlotNamespace ? (
            <PluginSlot
              slot={`${pluginSlotNamespace}.card.badges.after`}
              path={pluginSlotPath}
              context={{ ...orderCardPluginContext, section: 'badges' }}
            />
          ) : null}
        </div>
      </CardHeader>

      <CardContent className="flex-1 space-y-3">
        {/* 商品列表 */}
        <div className="space-y-2">
          {orderItems.slice(0, 1).map((item, index) => (
            <div key={index} className="flex items-start gap-3">
              {/* 商品图片 */}
              {item.imageUrl || item.image_url ? (
                <div className="h-16 w-16 flex-shrink-0 overflow-hidden rounded-md bg-muted">
                  <img
                    src={item.imageUrl || item.image_url || ''}
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
                <div className="flex h-16 w-16 flex-shrink-0 items-center justify-center rounded-md bg-muted">
                  <Package className="h-8 w-8 text-muted-foreground" />
                </div>
              )}

              {/* 商品信息 */}
              <div className="min-w-0 flex-1">
                <p className="mb-1 line-clamp-2 text-sm font-medium leading-5">{item.name}</p>
                {item.attributes && Object.keys(item.attributes).length > 0 && (
                  <div className="mb-1 flex flex-wrap gap-1">
                    {Object.entries(item.attributes)
                      .slice(0, 2)
                      .map(([key, value]) => (
                        <Badge key={key} variant="secondary" className="rounded px-1.5 text-xs">
                          {key}: {value as string}
                        </Badge>
                      ))}
                  </div>
                )}
                <p className="text-xs text-muted-foreground">x{item.quantity}</p>
              </div>
            </div>
          ))}
          {orderItems.length > 1 && (
            <p className="py-1 text-center text-xs text-muted-foreground">
              {t.order.totalItemsCount.replace('{count}', String(orderItems.length))}
            </p>
          )}
          {pluginSlotNamespace ? (
            <PluginSlot
              slot={`${pluginSlotNamespace}.card.product.after`}
              path={pluginSlotPath}
              context={{ ...orderCardPluginContext, section: 'product' }}
            />
          ) : null}
        </div>

        {/* 订单金额 */}
        <div className="flex items-center justify-between border-t pt-2">
          <span className="text-sm text-muted-foreground">{t.order.amountLabel}</span>
          <span className="text-sm font-semibold text-foreground">
            {formatCurrency(order.total_amount_minor ?? 0, order.currency)}
          </span>
        </div>
        {pluginSlotNamespace ? (
          <PluginSlot
            slot={`${pluginSlotNamespace}.card.summary.after`}
            path={pluginSlotPath}
            context={{ ...orderCardPluginContext, section: 'summary' }}
          />
        ) : null}

        {/* 物流信息 */}
        {(order.trackingNo || order.tracking_no) && (
          <div className="flex items-center gap-2 border-t pt-2">
            <Truck className="h-3.5 w-3.5 flex-shrink-0 text-muted-foreground" />
            <span className="truncate font-mono text-xs">
              {order.trackingNo || order.tracking_no}
            </span>
          </div>
        )}

        {/* 虚拟商品已发货提示 */}
        {isVirtualOnly && order.status === 'shipped' && (
          <div className="mt-auto flex items-start gap-2 border-t pt-2">
            <Key className="mt-0.5 h-3.5 w-3.5 flex-shrink-0 text-green-600 dark:text-green-400" />
            <p className="text-xs leading-5 text-green-700 dark:text-green-300">
              {t.order.virtualProductShipped}
            </p>
          </div>
        )}

        {/* 草稿提示（仅实物商品） */}
        {needsFilling && (
          <div className="mt-auto flex items-start gap-2 border-t pt-2">
            <FileEdit className="mt-0.5 h-3.5 w-3.5 flex-shrink-0 text-amber-600 dark:text-amber-400" />
            <p className="text-xs leading-5 text-amber-700 dark:text-amber-300">
              {needsFillingMessage}
            </p>
          </div>
        )}

        {/* 待付款提示 */}
        {isPendingPayment && (
          <div className="mt-auto flex items-start gap-2 border-t pt-2">
            <CreditCard className="mt-0.5 h-3.5 w-3.5 flex-shrink-0 text-orange-600 dark:text-orange-400" />
            <p className="text-xs leading-5 text-orange-700 dark:text-orange-300">
              {pendingPaymentMessage}
            </p>
          </div>
        )}
      </CardContent>

      <CardFooter className="flex-col gap-3 pt-3">
        {pluginSlotNamespace ? (
          <PluginSlot
            slot={`${pluginSlotNamespace}.card.actions.before`}
            path={pluginSlotPath}
            context={{ ...orderCardPluginContext, section: 'actions' }}
          />
        ) : null}
        <div className="flex w-full gap-2">
          {needsFilling ? (
            <>
              <Button
                size="sm"
                className="flex-1"
                onClick={handleFillForm}
                disabled={isGettingToken}
              >
                {isGettingToken ? (
                  <>
                    <Loader2 className="mr-1 h-3.5 w-3.5 animate-spin" />
                    {t.order.gettingTokenShort}
                  </>
                ) : (
                  <>
                    <FileEdit className="mr-1 h-3.5 w-3.5" />
                    {t.order.fillFormShort}
                  </>
                )}
              </Button>
              <Button asChild size="sm" variant="outline">
                <Link href={`/orders/${orderNo}`} onClick={handleOpenDetail}>
                  {t.common.detail}
                </Link>
              </Button>
            </>
          ) : isPendingPayment ? (
            <Button asChild size="sm" className="w-full">
              <Link href={`/orders/${orderNo}`} onClick={handleOpenDetail}>
                <CreditCard className="mr-1 h-3.5 w-3.5" />
                {t.order.payNow}
              </Link>
            </Button>
          ) : (
            <Button asChild size="sm" variant="outline" className="w-full">
              <Link href={`/orders/${orderNo}`} onClick={handleOpenDetail}>
                {t.order.viewOrder}
              </Link>
            </Button>
          )}
        </div>
        {pluginSlotNamespace ? (
          <PluginSlot
            slot={`${pluginSlotNamespace}.card.actions.after`}
            path={pluginSlotPath}
            context={{ ...orderCardPluginContext, section: 'actions' }}
          />
        ) : null}
      </CardFooter>
    </Card>
  )
}
