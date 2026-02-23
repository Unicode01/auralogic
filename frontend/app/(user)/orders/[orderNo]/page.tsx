'use client'

import { Suspense, use, useState, useEffect } from 'react'
import { useSearchParams } from 'next/navigation'
import { useQuery } from '@tanstack/react-query'
import { useOrderDetail } from '@/hooks/use-orders'
import { OrderDetail } from '@/components/orders/order-detail'
import { PaymentMethodCard } from '@/components/orders/payment-method-card'
import { ShippingForm } from '@/components/forms/shipping-form'
import { Button } from '@/components/ui/button'
import { ArrowLeft, Loader2 } from 'lucide-react'
import Link from 'next/link'
import { getOrRefreshFormToken, getFormInfo, getOrderVirtualProducts } from '@/lib/api'
import { useLocale } from '@/hooks/use-locale'
import { usePageTitle } from '@/hooks/use-page-title'
import { getTranslations } from '@/lib/i18n'

function OrderDetailContent({ orderNo }: { orderNo: string }) {
  const searchParams = useSearchParams()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.orderDetail)
  const [paymentMethodSelected, setPaymentMethodSelected] = useState(false)
  const [formToken, setFormToken] = useState<string | null>(null)
  const [formData, setFormData] = useState<any>(null)
  const [formLoading, setFormLoading] = useState(false)

  const { data, isLoading, refetch } = useOrderDetail(orderNo, {
    refetchInterval: paymentMethodSelected ? 5000 : false,
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

  const handleFormSuccess = () => {
    setFormToken(null)
    setFormData(null)
    refetch()
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
      <div className="flex items-center gap-4">
        <Button asChild variant="outline" size="sm">
          <Link href="/orders">
            <ArrowLeft className="mr-1.5 h-4 w-4" />
            <span className="hidden md:inline">{t.order.backToList}</span>
          </Link>
        </Button>
        <h1 className="text-lg md:text-xl font-bold">{t.order.orderDetail}</h1>
      </div>

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

