'use client'

import { Suspense, use, useCallback, useEffect, useState } from 'react'
import { useSearchParams } from 'next/navigation'
import { useQuery } from '@tanstack/react-query'
import { useOrderDetail } from '@/hooks/use-orders'
import { OrderDetail } from '@/components/orders/order-detail'
import { PaymentMethodCard } from '@/components/orders/payment-method-card'
import { ShippingForm } from '@/components/forms/shipping-form'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { PageLoading } from '@/components/ui/page-loading'
import { ArrowLeft, Loader2, FileText, AlertTriangle, Clock, RefreshCw } from 'lucide-react'
import Link from 'next/link'
import {
  getOrRefreshFormToken,
  getFormInfo,
  getOrderVirtualProducts,
  getPublicConfig,
  getInvoiceToken,
} from '@/lib/api'
import { useLocale } from '@/hooks/use-locale'
import { useIsMobile } from '@/hooks/use-mobile'
import { usePageTitle } from '@/hooks/use-page-title'
import { getTranslations } from '@/lib/i18n'
import { buildListReturnPath, readListBrowseState } from '@/lib/list-browse-state'
import { resolveApiErrorMessage } from '@/lib/api-error'
import toast from 'react-hot-toast'
import { PluginSlot } from '@/components/plugins/plugin-slot'

function usePaymentCountdown(createdAt: string | undefined, autoCancelHours: number) {
  const [remaining, setRemaining] = useState<{
    hours: number
    minutes: number
    expired: boolean
  } | null>(null)

  useEffect(() => {
    if (!createdAt || !autoCancelHours || autoCancelHours <= 0) {
      setRemaining(null)
      return
    }

    const calc = () => {
      const deadline = new Date(createdAt).getTime() + autoCancelHours * 60 * 60 * 1000
      const diff = deadline - Date.now()
      if (diff <= 0) {
        setRemaining({ hours: 0, minutes: 0, expired: true })
      } else {
        const hours = Math.floor(diff / (1000 * 60 * 60))
        const minutes = Math.floor((diff % (1000 * 60 * 60)) / (1000 * 60))
        setRemaining({ hours, minutes, expired: false })
      }
    }

    calc()
    const timer = setInterval(calc, 60_000)
    return () => clearInterval(timer)
  }, [createdAt, autoCancelHours])

  return remaining
}

function OrderDetailContent({ orderNo }: { orderNo: string }) {
  const searchParams = useSearchParams()
  const { locale } = useLocale()
  const { isMobile, mounted } = useIsMobile()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.orderDetail)
  const isCompactLayout = mounted ? isMobile : false
  const [shouldAutoRefresh, setShouldAutoRefresh] = useState(true)
  const [formToken, setFormToken] = useState<string | null>(null)
  const [formData, setFormData] = useState<any>(null)
  const [formLoading, setFormLoading] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)
  const [invoiceLoading, setInvoiceLoading] = useState(false)
  const [orderListBackHref, setOrderListBackHref] = useState('/orders')

  const {
    data,
    isLoading,
    isFetching,
    error: orderError,
    refetch,
  } = useOrderDetail(orderNo, {
    refetchInterval: shouldAutoRefresh ? 5000 : false,
  })

  const { data: publicConfig } = useQuery({
    queryKey: ['publicConfig'],
    queryFn: getPublicConfig,
    staleTime: 1000 * 60 * 5,
  })

  const order = data?.data

  useEffect(() => {
    const browseState = readListBrowseState('orders')
    setOrderListBackHref(buildListReturnPath('orders', browseState?.listPath, orderNo))
  }, [orderNo])

  // 订单非终态时自动轮询状态，终态自动停止
  useEffect(() => {
    if (!order?.status) return
    const activeStatuses = [
      'pending_payment',
      'draft',
      'need_resubmit',
      'pending',
      'shipped',
      'refund_pending',
    ]
    setShouldAutoRefresh(activeStatuses.includes(order.status))
  }, [order?.status])

  // 获取虚拟产品内容（只有已付款后才能查看）
  const { data: virtualStocksData } = useQuery({
    queryKey: ['orderVirtualProducts', orderNo],
    queryFn: () => getOrderVirtualProducts(orderNo),
    enabled:
      !!order &&
      order.status !== 'pending_payment' &&
      order.status !== 'draft' &&
      order.status !== 'need_resubmit',
  })

  // 如果从填表页返回，自动刷新数据
  useEffect(() => {
    if (searchParams.get('refresh') === 'true') {
      refetch()
    }
  }, [searchParams, refetch])

  const isDraft = order?.status === 'draft'
  const isNeedResubmit = order?.status === 'need_resubmit'
  const isVirtualOnly = order?.items?.every(
    (item: any) => (item.product_type || item.productType) === 'virtual'
  )
  const needsShippingForm = (isDraft || isNeedResubmit) && !isVirtualOnly
  const loadShippingForm = useCallback(async () => {
    if (!needsShippingForm || !order) return

    setFormLoading(true)
    setFormError(null)
    setFormToken(null)
    setFormData(null)

    try {
      const response = await getOrRefreshFormToken(orderNo)
      const token = response.data.form_token
      if (!token) {
        throw new Error(t.order.getFormTokenFailed)
      }

      setFormToken(token)
      const res = await getFormInfo(token)
      setFormData(res.data)
    } catch (error: any) {
      const message = resolveApiErrorMessage(error, t, t.order.getFormTokenFailed)
      setFormError(message)
      console.error('Failed to load shipping form:', message)
    } finally {
      setFormLoading(false)
    }
  }, [needsShippingForm, order, orderNo, t])

  // 付款倒计时（hook 必须在条件返回之前调用）
  const autoCancelHours = publicConfig?.data?.auto_cancel_hours || 0
  const countdown = usePaymentCountdown(
    order?.status === 'pending_payment' ? order?.created_at || order?.createdAt : undefined,
    autoCancelHours
  )
  const isPendingPayment = order?.status === 'pending_payment'
  const virtualStocks = virtualStocksData?.data?.stocks || []
  const invoiceEnabled = !!publicConfig?.data?.invoice_enabled
  const showVirtualStockRemark = !!publicConfig?.data?.show_virtual_stock_remark
  const userOrderDetailPluginContext = {
    view: 'user_order_detail',
    order: order
      ? {
          id: order.id,
          order_no: order.orderNo || order.order_no,
          status: order.status,
          currency: order.currency,
          total_amount_minor: order.total_amount_minor ?? 0,
        }
      : {
          order_no: orderNo || undefined,
        },
    summary: {
      item_count: Array.isArray(order?.items) ? order.items.length : 0,
      virtual_stock_count: virtualStocks.length,
      is_pending_payment: Boolean(isPendingPayment),
      is_virtual_only: Boolean(isVirtualOnly),
      needs_shipping_form: needsShippingForm,
      invoice_enabled: invoiceEnabled,
      show_virtual_stock_remark: showVirtualStockRemark,
    },
    state: {
      load_failed: Boolean(orderError && !order),
      not_found: !isLoading && !orderError && !order,
      shipping_form_loading: needsShippingForm && formLoading,
      shipping_form_load_failed: needsShippingForm && Boolean(formError),
    },
  }

  // 自动获取表单令牌和表单数据
  useEffect(() => {
    void loadShippingForm()
  }, [loadShippingForm])

  useEffect(() => {
    if (needsShippingForm) return
    setFormLoading(false)
    setFormError(null)
    setFormToken(null)
    setFormData(null)
  }, [needsShippingForm])

  if (isLoading) {
    return <PageLoading text={t.common.loading} />
  }

  if (orderError && !data?.data) {
    return (
      <Card className="border-dashed bg-muted/15">
        <CardContent className="py-12 text-center">
          <AlertTriangle className="mx-auto mb-4 h-16 w-16 text-muted-foreground" />
          <p className="text-base font-medium">{t.order.orderLoadFailedTitle}</p>
          <p className="mt-2 text-sm text-muted-foreground">{t.order.orderLoadFailedDesc}</p>
          <div className="mt-4 flex flex-col justify-center gap-3 sm:flex-row">
            <Button variant="outline" onClick={() => refetch()}>
              {t.common.refresh}
            </Button>
            <Button asChild>
              <Link href={orderListBackHref}>{t.order.backToList}</Link>
            </Button>
          </div>
          <PluginSlot
            slot="user.order_detail.load_failed"
            context={{ ...userOrderDetailPluginContext, section: 'order_state' }}
          />
        </CardContent>
      </Card>
    )
  }

  if (!data?.data) {
    return (
      <Card className="border-dashed bg-muted/15">
        <CardContent className="py-12 text-center">
          <FileText className="mx-auto mb-4 h-16 w-16 text-muted-foreground" />
          <p className="text-base font-medium">{t.order.orderNotFound}</p>
          <p className="mt-2 text-sm text-muted-foreground">{t.order.orderNotFoundDesc}</p>
          <div className="mt-4 flex flex-col justify-center gap-3 sm:flex-row">
            <Button variant="outline" onClick={() => refetch()}>
              {t.common.refresh}
            </Button>
            <Button asChild>
              <Link href={orderListBackHref}>{t.order.backToList}</Link>
            </Button>
          </div>
          <PluginSlot
            slot="user.order_detail.not_found"
            context={{ ...userOrderDetailPluginContext, section: 'order_state' }}
          />
        </CardContent>
      </Card>
    )
  }

  const handleFormSuccess = () => {
    setFormToken(null)
    setFormData(null)
    refetch()
  }

  const handleDownloadInvoice = async () => {
    if (invoiceLoading) return
    setInvoiceLoading(true)
    try {
      const res = await getInvoiceToken(orderNo)
      const token = res.data.token
      const baseUrl = process.env.NEXT_PUBLIC_API_URL || ''
      window.open(`${baseUrl}/api/user/invoice/${token}`, '_blank')
    } catch (error: any) {
      toast.error(resolveApiErrorMessage(error, t, t.order.downloadInvoiceFailed))
    } finally {
      setInvoiceLoading(false)
    }
  }

  const shippingFormNode =
    needsShippingForm && formToken && formData ? (
      <ShippingForm
        formToken={formToken}
        orderInfo={formData}
        lang={locale}
        onSuccess={handleFormSuccess}
        hideOrderItems
        hidePassword
      />
    ) : needsShippingForm && formLoading ? (
      <Card>
        <CardContent className="flex flex-col items-center justify-center gap-3 py-8 text-center text-sm">
          <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
          <div className="space-y-1">
            <p className="font-medium text-foreground">{t.order.fillShippingInfo}</p>
            <p className="text-muted-foreground">{t.order.shippingFormLoading}</p>
          </div>
        </CardContent>
      </Card>
    ) : needsShippingForm && formError ? (
      <Card className="border-destructive/30 bg-destructive/5">
        <CardContent className="space-y-3 p-4 text-sm">
          <div className="flex items-start gap-3">
            <div className="rounded-full bg-destructive/10 p-2 text-destructive">
              <AlertTriangle className="h-4 w-4" />
            </div>
            <div className="space-y-1">
              <div className="font-medium text-destructive">{t.order.shippingFormLoadFailed}</div>
              <div className="text-muted-foreground">{t.order.shippingFormLoadFailedDesc}</div>
            </div>
          </div>
          <div className="rounded-md border border-destructive/20 bg-background/70 p-3 text-xs text-muted-foreground">
            {formError}
          </div>
          <div className="flex flex-wrap gap-2">
            <Button variant="outline" size="sm" onClick={() => void loadShippingForm()}>
              <RefreshCw className="mr-2 h-4 w-4" />
              {t.order.retryLoadShippingForm}
            </Button>
            <Button variant="ghost" size="sm" onClick={() => refetch()}>
              {t.common.refresh}
            </Button>
          </div>
        </CardContent>
      </Card>
    ) : undefined

  return (
    <div className="space-y-6">
      <PluginSlot slot="user.order_detail.top" context={userOrderDetailPluginContext} />
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-3">
          <Button asChild variant="outline" size={isCompactLayout ? 'icon' : 'sm'}>
            <Link href={orderListBackHref}>
              <ArrowLeft className={isCompactLayout ? 'h-4 w-4' : 'h-4 w-4 md:mr-1.5'} />
              {isCompactLayout ? (
                <span className="sr-only">{t.order.backToList}</span>
              ) : (
                <span>{t.order.backToList}</span>
              )}
            </Link>
          </Button>
          <div>
            <h1 className={isCompactLayout ? 'text-lg font-bold' : 'text-lg font-bold md:text-xl'}>
              {t.order.orderDetail}
            </h1>
          </div>
        </div>

        <div className="flex shrink-0 items-center gap-2">
          <Button
            variant="outline"
            size={isCompactLayout ? 'icon' : 'sm'}
            onClick={() => refetch()}
            disabled={isFetching}
            aria-label={t.common.refresh}
            title={t.common.refresh}
          >
            <RefreshCw
              className={`${isCompactLayout ? 'h-4 w-4' : 'h-4 w-4 md:mr-1.5'} ${isFetching ? 'animate-spin' : ''}`}
            />
            {isCompactLayout ? (
              <span className="sr-only">{t.common.refresh}</span>
            ) : (
              <span>{t.common.refresh}</span>
            )}
          </Button>
          {invoiceEnabled && order.status === 'completed' && (
            <Button
              variant="outline"
              size={isCompactLayout ? 'icon' : 'sm'}
              onClick={handleDownloadInvoice}
              disabled={invoiceLoading}
            >
              <FileText className={isCompactLayout ? 'h-4 w-4' : 'h-4 w-4 md:mr-1.5'} />
              {isCompactLayout ? (
                <span className="sr-only">
                  {invoiceLoading ? t.common.loading : t.order.downloadInvoice}
                </span>
              ) : (
                <span>{invoiceLoading ? t.common.loading : t.order.downloadInvoice}</span>
              )}
            </Button>
          )}
        </div>
      </div>

      {isPendingPayment && countdown && (
        <div
          className={`flex items-center gap-3 rounded-lg border p-4 ${
            countdown.expired ? 'border-l-4 border-l-red-500' : 'border-l-4 border-l-amber-500'
          }`}
        >
          <div
            className={`flex-shrink-0 rounded-full p-2 ${
              countdown.expired
                ? 'bg-red-100 dark:bg-red-900/30'
                : 'bg-amber-100 dark:bg-amber-900/30'
            }`}
          >
            {countdown.expired ? (
              <AlertTriangle className="h-4 w-4 text-red-500" />
            ) : (
              <Clock className="h-4 w-4 text-amber-500" />
            )}
          </div>
          <div className="min-w-0 flex-1">
            <p className="text-sm font-medium">
              {countdown.expired ? t.order.paymentExpired : t.order.paymentUrgencyTitle}
            </p>
            {!countdown.expired && (
              <p className="mt-0.5 text-sm text-muted-foreground">
                {t.order.paymentUrgencyCountdown
                  .replace('{hours}', String(countdown.hours))
                  .replace('{minutes}', String(countdown.minutes))}
                {' - '}
                {t.order.paymentUrgencyDesc}
              </p>
            )}
          </div>
        </div>
      )}

      {order.status === 'refund_pending' && (
        <div className="rounded-lg border border-amber-300 bg-amber-50/80 p-4 text-sm text-amber-900 dark:border-amber-500/40 dark:bg-amber-950/30 dark:text-amber-200">
          <p className="font-medium">{t.order.refundPendingTitle}</p>
          <p className="mt-1 text-muted-foreground dark:text-amber-100/80">
            {t.order.refundPendingDesc}
          </p>
        </div>
      )}

      <OrderDetail
        order={order}
        virtualStocks={virtualStocks}
        isVirtualOnly={isVirtualOnly}
        compactLayout={isCompactLayout}
        showVirtualStockRemark={showVirtualStockRemark}
        pluginSlotNamespace="user.order_detail"
        pluginSlotContext={userOrderDetailPluginContext}
        pluginSlotPath={`/orders/${orderNo}`}
        paymentCard={
          isPendingPayment ? (
            <PaymentMethodCard
              orderNo={orderNo}
              onPaymentSelected={() => {
                refetch()
              }}
              pluginSlotNamespace="user.order_detail.payment"
              pluginSlotContext={userOrderDetailPluginContext}
              pluginSlotPath={`/orders/${orderNo}`}
            />
          ) : undefined
        }
        shippingForm={shippingFormNode}
      />
      <PluginSlot slot="user.order_detail.bottom" context={userOrderDetailPluginContext} />
    </div>
  )
}

export default function OrderDetailPage({ params }: { params: Promise<{ orderNo: string }> }) {
  const { orderNo } = use(params)

  return (
    <Suspense
      fallback={
        <div className="flex min-h-screen items-center justify-center">
          <Loader2 className="h-8 w-8 animate-spin" />
        </div>
      }
    >
      <OrderDetailContent orderNo={orderNo} />
    </Suspense>
  )
}
