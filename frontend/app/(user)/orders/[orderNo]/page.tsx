'use client'

import { Suspense, use, useState, useEffect } from 'react'
import { useSearchParams } from 'next/navigation'
import { useQuery } from '@tanstack/react-query'
import { useOrderDetail } from '@/hooks/use-orders'
import { OrderDetail } from '@/components/orders/order-detail'
import { PaymentMethodCard } from '@/components/orders/payment-method-card'
import { ShippingForm } from '@/components/forms/shipping-form'
import { Button } from '@/components/ui/button'
import { ArrowLeft, Loader2, FileText, AlertTriangle, Clock } from 'lucide-react'
import Link from 'next/link'
import { getOrRefreshFormToken, getFormInfo, getOrderVirtualProducts, getPublicConfig, getInvoiceToken } from '@/lib/api'
import { useLocale } from '@/hooks/use-locale'
import { usePageTitle } from '@/hooks/use-page-title'
import { getTranslations } from '@/lib/i18n'
import toast from 'react-hot-toast'

function usePaymentCountdown(createdAt: string | undefined, autoCancelHours: number) {
  const [remaining, setRemaining] = useState<{ hours: number; minutes: number; expired: boolean } | null>(null)

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
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.orderDetail)
  const [paymentMethodSelected, setPaymentMethodSelected] = useState(false)
  const [formToken, setFormToken] = useState<string | null>(null)
  const [formData, setFormData] = useState<any>(null)
  const [formLoading, setFormLoading] = useState(false)
  const [invoiceLoading, setInvoiceLoading] = useState(false)

  const { data, isLoading, refetch } = useOrderDetail(orderNo, {
    refetchInterval: paymentMethodSelected ? 5000 : false,
  })

  const { data: publicConfig } = useQuery({
    queryKey: ['publicConfig'],
    queryFn: getPublicConfig,
    staleTime: 1000 * 60 * 5,
  })

  const order = data?.data

  // 支付成功后（状态不再是 pending_payment），停止轮询
  useEffect(() => {
    if (order && order.status !== 'pending_payment' && paymentMethodSelected) {
      setPaymentMethodSelected(false)
    }
  }, [order?.status, paymentMethodSelected])

  // 获取虚拟产品内容（只有已付款后才能查看）
  const { data: virtualStocksData } = useQuery({
    queryKey: ['orderVirtualProducts', orderNo],
    queryFn: () => getOrderVirtualProducts(orderNo),
    enabled: !!order && order.status !== 'pending_payment' && order.status !== 'draft' && order.status !== 'need_resubmit',
  })

  // 如果从填表页返回，自动刷新数据
  useEffect(() => {
    if (searchParams.get('refresh') === 'true') {
      refetch()
    }
  }, [searchParams, refetch])

  const isDraft = order?.status === 'draft'
  const isNeedResubmit = order?.status === 'need_resubmit'
  const isVirtualOnly = order?.items?.every((item: any) =>
    (item.product_type || item.productType) === 'virtual'
  )
  const needsShippingForm = (isDraft || isNeedResubmit) && !isVirtualOnly

  // 付款倒计时（hook 必须在条件返回之前调用）
  const autoCancelHours = publicConfig?.data?.auto_cancel_hours || 0
  const countdown = usePaymentCountdown(
    order?.status === 'pending_payment' ? (order?.created_at || order?.createdAt) : undefined,
    autoCancelHours
  )

  // 自动获取表单令牌和表单数据
  useEffect(() => {
    if (!needsShippingForm || !order) return

    setFormLoading(true)
    getOrRefreshFormToken(orderNo)
      .then((response) => {
        const token = response.data.form_token
        if (token) {
          setFormToken(token)
          return getFormInfo(token)
        }
        throw new Error(t.order.getFormTokenFailed)
      })
      .then((res) => {
        setFormData(res.data)
      })
      .catch((error: any) => {
        console.error('Failed to load shipping form:', error.message)
      })
      .finally(() => {
        setFormLoading(false)
      })
  }, [needsShippingForm, order?.status, orderNo])

  if (isLoading) {
    return <div className="text-center py-12">{t.common.loading}</div>
  }

  if (!data?.data) {
    return <div className="text-center py-12">{t.order.orderNotFound}</div>
  }

  const isPendingPayment = order.status === 'pending_payment'
  const virtualStocks = virtualStocksData?.data?.stocks || []
  const invoiceEnabled = !!publicConfig?.data?.invoice_enabled

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
    } catch {
      toast.error(t.order.downloadInvoiceFailed || 'Failed to download invoice')
    } finally {
      setInvoiceLoading(false)
    }
  }

  const shippingFormNode = needsShippingForm && formToken && formData ? (
    <ShippingForm
      formToken={formToken}
      orderInfo={formData}
      lang={locale}
      onSuccess={handleFormSuccess}
      hideOrderItems
      hidePassword
    />
  ) : needsShippingForm && formLoading ? (
    <div className="flex items-center justify-center py-8">
      <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
    </div>
  ) : undefined

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <Button asChild variant="outline" size="sm">
            <Link href="/orders">
              <ArrowLeft className="h-4 w-4 md:mr-1.5" />
              <span className="hidden md:inline">{t.order.backToList}</span>
            </Link>
          </Button>
          <h1 className="text-lg md:text-xl font-bold">{t.order.orderDetail}</h1>
        </div>

        <div className="flex gap-2">
          {invoiceEnabled && order.status === 'completed' && (
            <Button
              variant="outline"
              size="sm"
              onClick={handleDownloadInvoice}
              disabled={invoiceLoading}
            >
              <FileText className="h-4 w-4 md:mr-1.5" />
              <span className="hidden md:inline">
                {invoiceLoading ? t.common.loading : t.order.downloadInvoice}
              </span>
            </Button>
          )}
        </div>
      </div>

      {/* 待付款紧迫感提示 */}
      {isPendingPayment && countdown && (
        <div className={`flex items-center gap-3 rounded-lg border p-4 ${
          countdown.expired
            ? 'border-l-4 border-l-red-500'
            : 'border-l-4 border-l-amber-500'
        }`}>
          <div className={`rounded-full p-2 flex-shrink-0 ${
            countdown.expired
              ? 'bg-red-100 dark:bg-red-900/30'
              : 'bg-amber-100 dark:bg-amber-900/30'
          }`}>
            {countdown.expired ? (
              <AlertTriangle className="h-4 w-4 text-red-500" />
            ) : (
              <Clock className="h-4 w-4 text-amber-500" />
            )}
          </div>
          <div className="flex-1 min-w-0">
            <p className="font-medium text-sm">
              {countdown.expired ? t.order.paymentExpired : t.order.paymentUrgencyTitle}
            </p>
            {!countdown.expired && (
              <p className="text-sm text-muted-foreground mt-0.5">
                {t.order.paymentUrgencyCountdown
                    .replace('{hours}', String(countdown.hours))
                    .replace('{minutes}', String(countdown.minutes))}
                {' — '}
                {t.order.paymentUrgencyDesc}
              </p>
            )}
          </div>
        </div>
      )}

      <OrderDetail
        order={order}
        virtualStocks={virtualStocks}
        isVirtualOnly={isVirtualOnly}
        paymentCard={isPendingPayment ? <PaymentMethodCard orderNo={orderNo} onPaymentSelected={() => { setPaymentMethodSelected(true); refetch() }} /> : undefined}
        shippingForm={shippingFormNode}
      />
    </div>
  )
}

export default function OrderDetailPage({
  params,
}: {
  params: Promise<{ orderNo: string }>
}) {
  const { orderNo } = use(params)

  return (
    <Suspense fallback={
      <div className="flex items-center justify-center min-h-screen">
        <Loader2 className="h-8 w-8 animate-spin" />
      </div>
    }>
      <OrderDetailContent orderNo={orderNo} />
    </Suspense>
  )
}

