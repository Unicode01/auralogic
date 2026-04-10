'use client'

import { use, useCallback, useEffect, useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useRouter } from 'next/navigation'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'
import {
  getAdminOrderDetail,
  assignTracking,
  adminCompleteOrder,
  adminCancelOrder,
  adminDeleteOrder,
  updateOrderShippingInfo,
  requestOrderResubmit,
  getCountries,
  adminMarkOrderAsPaid,
  updateOrderPrice,
  adminDeliverVirtualStock,
  adminRefundOrder,
  adminConfirmRefund,
} from '@/lib/api'
import { resolveApiErrorMessage } from '@/lib/api-error'
import { OrderDetail } from '@/components/orders/order-detail'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
  DialogFooter,
  DialogDescription,
} from '@/components/ui/dialog'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  ArrowLeft,
  Truck,
  CheckCircle,
  ChevronDown,
  XCircle,
  Trash2,
  Edit,
  RotateCcw,
  CreditCard,
  Wallet,
  Clock,
  Coins,
  DollarSign,
  Key,
  Undo2,
} from 'lucide-react'
import Link from 'next/link'
import { useToast } from '@/hooks/use-toast'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { PluginSlot } from '@/components/plugins/plugin-slot'
import { formatDate, formatCurrency, minorToMajor, parseMajorToMinor } from '@/lib/utils'
import { resolvePaymentMethodIcon } from '@/lib/payment-method-icons'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { OrderStatusBadge } from '@/components/orders/order-status-badge'

export default function AdminOrderDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params)
  const orderId = parseInt(id)
  const router = useRouter()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.adminOrderDetail)
  const [trackingNo, setTrackingNo] = useState('')
  const [adminRemark, setAdminRemark] = useState('')
  const [cancelReason, setCancelReason] = useState('')
  const [refundReason, setRefundReason] = useState('')
  const [confirmRefundRemark, setConfirmRefundRemark] = useState('')
  const [confirmRefundTransactionId, setConfirmRefundTransactionId] = useState('')
  const [resubmitReason, setResubmitReason] = useState('')
  const [openTracking, setOpenTracking] = useState(false)
  const [openComplete, setOpenComplete] = useState(false)
  const [openCancel, setOpenCancel] = useState(false)
  const [openRefund, setOpenRefund] = useState(false)
  const [openConfirmRefund, setOpenConfirmRefund] = useState(false)
  const [openDelete, setOpenDelete] = useState(false)
  const [openEdit, setOpenEdit] = useState(false)
  const [openResubmit, setOpenResubmit] = useState(false)
  const [openDeliverVirtual, setOpenDeliverVirtual] = useState(false)
  const [openUpdatePrice, setOpenUpdatePrice] = useState(false)
  const [markOnlyShipped, setMarkOnlyShipped] = useState(false)
  const [newPrice, setNewPrice] = useState('')
  const [formAccess, setFormAccess] = useState<{
    form_url?: string
    form_token?: string
    form_expires_at?: string
  } | null>(null)

  // 编辑收货信息的表单状态
  const [editForm, setEditForm] = useState({
    receiver_name: '',
    phone_code: '+86',
    receiver_phone: '',
    receiver_email: '',
    receiver_country: 'CN',
    receiver_province: '',
    receiver_city: '',
    receiver_district: '',
    receiver_address: '',
    receiver_postcode: '',
  })

  const [countries, setCountries] = useState<any[]>([])

  const queryClient = useQueryClient()
  const toast = useToast()

  const showOrderError = useCallback(
    (error: unknown, fallback: string) => {
      toast.error(resolveApiErrorMessage(error, t, fallback))
    },
    [t, toast]
  )

  const { data, isLoading } = useQuery({
    queryKey: ['adminOrderDetail', orderId],
    queryFn: () => getAdminOrderDetail(orderId),
    enabled: !!orderId,
    staleTime: 0,
  })
  const orderWarnings: string[] = Array.isArray(data?.data?.warnings)
    ? data.data.warnings.filter(
        (item: unknown): item is string => typeof item === 'string' && item.trim() !== ''
      )
    : []

  const assignMutation = useMutation({
    mutationFn: () => assignTracking(orderId, { tracking_no: trackingNo }),
    onSuccess: () => {
      toast.success(t.order.trackingAssigned)
      queryClient.invalidateQueries({ queryKey: ['adminOrderDetail', orderId] })
      setOpenTracking(false)
      setTrackingNo('')
    },
    onError: (error: any) => {
      showOrderError(error, t.order.assignFailed)
    },
  })

  const completeMutation = useMutation({
    mutationFn: () => adminCompleteOrder(orderId, adminRemark),
    onSuccess: () => {
      toast.success(t.order.orderCompleted)
      queryClient.invalidateQueries({ queryKey: ['adminOrderDetail', orderId] })
      setOpenComplete(false)
      setAdminRemark('')
    },
    onError: (error: any) => {
      showOrderError(error, t.order.operationFailed)
    },
  })

  const cancelMutation = useMutation({
    mutationFn: () => adminCancelOrder(orderId, cancelReason),
    onSuccess: () => {
      toast.success(t.order.orderCancelled)
      queryClient.invalidateQueries({ queryKey: ['adminOrderDetail', orderId] })
      setOpenCancel(false)
      setCancelReason('')
    },
    onError: (error: any) => {
      showOrderError(error, t.order.cancelFailed)
    },
  })

  const refundMutation = useMutation({
    mutationFn: () => adminRefundOrder(orderId, refundReason),
    onSuccess: (response: any) => {
      toast.success(
        response?.data?.status === 'refund_pending'
          ? t.order.orderRefundPending
          : t.order.orderRefunded
      )
      queryClient.invalidateQueries({ queryKey: ['adminOrderDetail', orderId] })
      setOpenRefund(false)
      setRefundReason('')
    },
    onError: (error: any) => {
      showOrderError(error, t.order.refundFailed)
    },
  })

  const confirmRefundMutation = useMutation({
    mutationFn: () =>
      adminConfirmRefund(orderId, {
        transaction_id: confirmRefundTransactionId,
        remark: confirmRefundRemark,
      }),
    onSuccess: () => {
      toast.success(t.order.refundConfirmed)
      queryClient.invalidateQueries({ queryKey: ['adminOrderDetail', orderId] })
      setOpenConfirmRefund(false)
      setConfirmRefundRemark('')
      setConfirmRefundTransactionId('')
    },
    onError: (error: any) => {
      showOrderError(error, t.order.refundConfirmFailed)
    },
  })

  const deleteMutation = useMutation({
    mutationFn: () => adminDeleteOrder(orderId),
    onSuccess: () => {
      toast.success(t.order.orderDeleted)
      router.push('/admin/orders')
    },
    onError: (error: any) => {
      showOrderError(error, t.order.deleteFailed)
    },
  })

  const editMutation = useMutation({
    mutationFn: () => updateOrderShippingInfo(orderId, editForm),
    onSuccess: () => {
      toast.success(t.order.shippingUpdated)
      queryClient.invalidateQueries({ queryKey: ['adminOrderDetail', orderId] })
      setOpenEdit(false)
    },
    onError: (error: any) => {
      showOrderError(error, t.order.updateFailed)
    },
  })

  const resubmitMutation = useMutation({
    mutationFn: () => requestOrderResubmit(orderId, resubmitReason),
    onSuccess: (response: any) => {
      setFormAccess({
        form_url: response?.data?.new_form_url,
        form_token: response?.data?.new_form_token,
        form_expires_at: response?.data?.form_expires_at,
      })
      toast.success(response?.data?.message || t.order.resubmitRequested)
      queryClient.invalidateQueries({ queryKey: ['adminOrderDetail', orderId] })
      setOpenResubmit(false)
      setResubmitReason('')
    },
    onError: (error: any) => {
      showOrderError(error, t.order.operationFailed)
    },
  })

  const markPaidMutation = useMutation({
    mutationFn: () => adminMarkOrderAsPaid(orderId),
    onSuccess: () => {
      toast.success(t.order.orderMarkedPaid)
      queryClient.invalidateQueries({ queryKey: ['adminOrderDetail', orderId] })
    },
    onError: (error: any) => {
      showOrderError(error, t.order.operationFailed)
    },
  })

  const updatePriceMutation = useMutation({
    mutationFn: (amountMinor: number) => updateOrderPrice(orderId, amountMinor),
    onSuccess: (response: any) => {
      toast.success(
        response?.data?.payment_artifacts_reset
          ? t.order.priceUpdatedAndPaymentReset
          : t.order.priceUpdated
      )
      queryClient.invalidateQueries({ queryKey: ['adminOrderDetail', orderId] })
      setOpenUpdatePrice(false)
      setNewPrice('')
    },
    onError: (error: any) => {
      showOrderError(error, t.order.updateFailed)
    },
  })

  const deliverVirtualMutation = useMutation({
    mutationFn: (onlyMarkShipped: boolean) =>
      adminDeliverVirtualStock(orderId, { mark_only_shipped: onlyMarkShipped }),
    onSuccess: (response: any, onlyMarkShipped) => {
      toast.success(
        response?.data?.message ||
          (onlyMarkShipped ? t.order.virtualMarkedCompleteOnly : t.order.virtualDelivered)
      )
      queryClient.invalidateQueries({ queryKey: ['adminOrderDetail', orderId] })
      setOpenDeliverVirtual(false)
      setMarkOnlyShipped(false)
    },
    onError: (error: any) => {
      showOrderError(error, t.order.deliverFailed)
    },
  })

  // 获取国家列表
  useEffect(() => {
    getCountries()
      .then((response: any) => {
        setCountries(response.data || [])
      })
      .catch((err) => {
        showOrderError(err, t.common.failed)
        console.error('Failed to load countries:', err)
      })
  }, [showOrderError, t.common.failed])

  // 辅助函数：检查字段是否被打码
  const isMaskedValue = useCallback((value: string | undefined) => {
    if (!value) return false
    // 检查是否为打码标记
    return value === '***' || value.includes('****')
  }, [])

  // 辅助函数：处理可能被打码的字段
  const handleMaskedField = useCallback(
    (value: string | undefined, defaultValue: string = '') => {
      if (!value) return defaultValue
      // 如果是打码值，返回空字符串（让管理员重新填写）
      if (isMaskedValue(value)) return ''
      // 否则返回原值
      return value
    },
    [isMaskedValue]
  )

  // 当订单数据加载后，初始化编辑表单
  useEffect(() => {
    if (data?.data) {
      const order = data.data.order || data.data
      setEditForm({
        // 对于可能被打码的敏感字段，使用处理函数
        receiver_name: handleMaskedField(order.receiver_name || order.receiverName),
        phone_code: order.phone_code || order.phoneCode || '+86',
        receiver_phone: handleMaskedField(order.receiver_phone || order.receiverPhone),
        receiver_email: order.receiver_email || order.receiverEmail || '',
        receiver_country: order.receiver_country || order.receiverCountry || 'CN',
        receiver_province: order.receiver_province || order.receiverProvince || '',
        receiver_city: order.receiver_city || order.receiverCity || '',
        receiver_district: order.receiver_district || order.receiverDistrict || '',
        receiver_address: handleMaskedField(order.receiver_address || order.receiverAddress),
        receiver_postcode: order.receiver_postcode || order.receiverPostcode || '',
      })
      // 初始化新价格为当前订单总价
      const amountMinor = order.total_amount_minor ?? 0
      setNewPrice(minorToMajor(amountMinor).toString())
    }
  }, [data, handleMaskedField])

  useEffect(() => {
    setFormAccess(null)
  }, [orderId])

  if (isLoading) {
    return <div className="py-12 text-center">{t.common.loading}</div>
  }

  if (!data?.data) {
    return <div className="py-12 text-center">{t.order.orderNotFound}</div>
  }

  // 处理新的数据结构：{order, serials, virtual_stocks} 或 旧结构直接是order
  const order = data.data.order || data.data
  const serials = data.data.serials || []
  const virtualStocks = data.data.virtual_stocks || []
  const paymentInfo = data.data.payment_info
  const orderNumber = order.orderNo || order.order_no
  const hasTracking = Boolean(order.trackingNo || order.tracking_no)
  const orderFormURL = String(formAccess?.form_url || data.data.form_url || '').trim()
  const orderFormToken = String(
    formAccess?.form_token || order.formToken || order.form_token || ''
  ).trim()
  const orderFormExpiresAt = String(
    formAccess?.form_expires_at || order.formExpiresAt || order.form_expires_at || ''
  ).trim()

  // 判断是否为纯虚拟商品订单
  const isVirtualOnly =
    order.items?.every((item: any) => (item.product_type || item.productType) === 'virtual') ??
    false
  const hasVirtualItems =
    order.items?.some((item: any) => (item.product_type || item.productType) === 'virtual') ??
    false
  const canMarkPaid = order.status === 'pending_payment'
  const canUpdatePrice = canMarkPaid
  const canDeliverVirtual =
    hasVirtualItems && (order.status === 'pending' || order.status === 'shipped')
  const canEditShipping =
    (order.status === 'pending' || order.status === 'need_resubmit') && !isVirtualOnly
  const canRequestResubmit = order.status === 'pending' && !isVirtualOnly
  const canAssignTracking = order.status === 'pending' && !isVirtualOnly && !hasTracking
  const canMarkComplete = order.status === 'shipped'
  const canCancel =
    order.status === 'pending_payment' ||
    order.status === 'draft' ||
    order.status === 'pending' ||
    order.status === 'need_resubmit'
  const canRefund =
    order.status === 'draft' ||
    order.status === 'pending' ||
    order.status === 'need_resubmit' ||
    order.status === 'shipped' ||
    order.status === 'completed'
  const canConfirmRefund = order.status === 'refund_pending'
  const canDelete =
    order.status === 'pending_payment' ||
    order.status === 'draft' ||
    order.status === 'cancelled' ||
    order.status === 'refunded'
  const secondaryActionCount =
    Number(canCancel) + Number(canRefund) + Number(canConfirmRefund) + Number(canDelete)
  const adminOrderDetailPluginContext = {
    view: 'admin_order_detail',
    order: {
      id: order.id,
      order_no: orderNumber,
      status: order.status,
      serial_generation_status: order.serial_generation_status || order.serialGenerationStatus,
      user_id: order.user_id ?? order.userId,
      external_user_id: order.external_user_id ?? order.externalUserID,
      external_user_name: order.external_user_name || order.externalUserName,
      total_amount_minor: order.total_amount_minor,
      currency: order.currency,
      tracking_no: order.tracking_no || order.trackingNo,
      receiver_country: order.receiver_country || order.receiverCountry,
      privacy_protected: Boolean(order.privacy_protected || order.privacyProtected),
      item_count: Array.isArray(order.items) ? order.items.length : 0,
    },
    flags: {
      is_virtual_only: isVirtualOnly,
      has_tracking: Boolean(order.tracking_no || order.trackingNo),
      has_payment_info: Boolean(paymentInfo),
      has_serials: serials.length > 0,
      serial_generation_pending: ['queued', 'processing'].includes(
        String(order.serial_generation_status || order.serialGenerationStatus || '')
      ),
      has_virtual_stocks: virtualStocks.length > 0,
    },
  }

  // 解析付款数据
  const parsePaymentData = () => {
    if (!paymentInfo?.payment_data) return null
    try {
      return typeof paymentInfo.payment_data === 'string'
        ? JSON.parse(paymentInfo.payment_data)
        : paymentInfo.payment_data
    } catch {
      return null
    }
  }

  const paymentData = parsePaymentData()
  const paymentDataEntries = paymentData
    ? Object.entries(paymentData).filter(([key, value]) => {
        if (key === 'paid_at' || key === 'transaction_id') return false
        return value !== undefined && value !== null && String(value).trim() !== ''
      })
    : []
  const formatPaymentFieldLabel = (key: string) =>
    key
      .split('_')
      .filter(Boolean)
      .map((segment) => segment.charAt(0).toUpperCase() + segment.slice(1))
      .join(' ')
  const formatPaymentFieldValue = (value: unknown) => {
    if (typeof value === 'string') return value
    if (typeof value === 'number' || typeof value === 'boolean') return String(value)
    try {
      return JSON.stringify(value)
    } catch {
      return String(value)
    }
  }

  // 获取付款方式图标
  const getPaymentIcon = () => {
    const iconName = paymentInfo?.payment_method?.icon
    if (!iconName) return <Wallet className="h-5 w-5" />

    const IconComponent = resolvePaymentMethodIcon(iconName, Wallet)
    return <IconComponent className="h-5 w-5" />
  }

  // 构建付款信息卡片
  const paymentCard = paymentInfo ? (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Wallet className="h-5 w-5" />
          {t.order.paymentInfo}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex items-center gap-3">
          <div className="flex h-10 w-10 items-center justify-center rounded-full bg-primary/10">
            {getPaymentIcon()}
          </div>
          <div>
            <p className="font-medium">
              {paymentInfo.payment_method?.name || t.order.unknownPaymentMethod}
            </p>
            <p className="text-sm text-muted-foreground">
              {paymentInfo.payment_method?.type === 'custom'
                ? t.order.customPaymentMethod
                : t.order.builtinPaymentMethod}
            </p>
          </div>
        </div>

        <div className="grid grid-cols-1 gap-4 text-sm md:grid-cols-2">
          <div>
            <dt className="flex items-center gap-1 text-muted-foreground">
              <Clock className="h-3.5 w-3.5" />
              {t.order.selectedAt}
            </dt>
            <dd>{formatDate(paymentInfo.selected_at)}</dd>
          </div>
          {paymentInfo.updated_at && (
            <div>
              <dt className="text-muted-foreground">{t.order.paymentUpdatedAt}</dt>
              <dd>{formatDate(paymentInfo.updated_at)}</dd>
            </div>
          )}
          {paymentData?.paid_at && (
            <div>
              <dt className="flex items-center gap-1 text-muted-foreground">
                <CheckCircle className="h-3.5 w-3.5" />
                {t.order.paidAt}
              </dt>
              <dd>{formatDate(paymentData.paid_at)}</dd>
            </div>
          )}
          {paymentData?.transaction_id && (
            <div className="md:col-span-2">
              <dt className="text-muted-foreground">{t.order.transactionId}</dt>
              <dd className="break-all font-mono text-xs">{paymentData.transaction_id}</dd>
            </div>
          )}
          <div>
            <dt className="text-muted-foreground">{t.order.paymentCacheStatus}</dt>
            <dd className="text-xs font-medium">
              {paymentInfo.payment_card_cached
                ? t.order.paymentCacheReady
                : t.order.paymentCacheEmpty}
            </dd>
          </div>
          {paymentInfo.payment_card_cache_expires_at && (
            <div>
              <dt className="text-muted-foreground">{t.order.paymentCacheExpiresAt}</dt>
              <dd>{formatDate(paymentInfo.payment_card_cache_expires_at)}</dd>
            </div>
          )}
        </div>
        {paymentDataEntries.length > 0 && (
          <div className="space-y-2 border-t pt-4">
            <p className="text-sm font-medium">{t.order.paymentRawData}</p>
            <dl className="grid grid-cols-1 gap-3 text-sm md:grid-cols-2">
              {paymentDataEntries.map(([key, value]) => (
                <div key={key}>
                  <dt className="text-muted-foreground">{formatPaymentFieldLabel(key)}</dt>
                  <dd className="break-all font-mono text-xs">
                    {formatPaymentFieldValue(value)}
                  </dd>
                </div>
              ))}
            </dl>
          </div>
        )}
      </CardContent>
    </Card>
  ) : null

  return (
    <div className="space-y-6">
      <PluginSlot slot="admin.order_detail.top" context={adminOrderDetailPluginContext} />
      {orderWarnings.length > 0 && (
        <Alert className="border-amber-300 bg-amber-50/70 text-amber-900 dark:border-amber-500/40 dark:bg-amber-950/30 dark:text-amber-200">
          <AlertTitle>{t.common.warning}</AlertTitle>
          <AlertDescription className="space-y-1">
            {orderWarnings.map((warning, index) => (
              <p key={`${warning}-${index}`}>{warning}</p>
            ))}
          </AlertDescription>
        </Alert>
      )}
      <div className="flex flex-col gap-3 xl:flex-row xl:items-start xl:justify-between">
        <div className="flex items-center gap-4">
          <Button asChild variant="outline" size="sm">
            <Link href="/admin/orders">
              <ArrowLeft className="h-4 w-4 md:mr-1.5" />
              <span className="hidden md:inline">{t.order.backToListShort}</span>
            </Link>
          </Button>
          <h1 className="text-lg font-bold md:text-xl">{t.order.orderDetail}</h1>
        </div>

        <div className="xl:max-w-[60%]">
          <div className="flex flex-wrap gap-2 xl:justify-end">
            <PluginSlot
              slot="admin.order_detail.actions"
              context={adminOrderDetailPluginContext}
              display="inline"
            />
            {/* 标记已付款 */}
            {canMarkPaid && (
              <AlertDialog>
                <AlertDialogTrigger asChild>
                  <Button disabled={markPaidMutation.isPending}>
                    <CreditCard className="mr-2 h-4 w-4" />
                    {markPaidMutation.isPending ? t.admin.processing : t.order.markPaid}
                  </Button>
                </AlertDialogTrigger>
                <AlertDialogContent>
                  <AlertDialogHeader>
                    <AlertDialogTitle>{t.order.confirmMarkPaidTitle}</AlertDialogTitle>
                    <AlertDialogDescription>{t.order.confirmMarkPaidDesc}</AlertDialogDescription>
                  </AlertDialogHeader>
                  <AlertDialogFooter>
                    <AlertDialogCancel>{t.common.cancel}</AlertDialogCancel>
                    <AlertDialogAction
                      onClick={() => markPaidMutation.mutate()}
                      disabled={markPaidMutation.isPending}
                    >
                      {markPaidMutation.isPending ? t.admin.processing : t.order.markPaid}
                    </AlertDialogAction>
                  </AlertDialogFooter>
                </AlertDialogContent>
              </AlertDialog>
            )}

            {/* 修改订单价格 */}
            {canUpdatePrice && (
              <Dialog open={openUpdatePrice} onOpenChange={setOpenUpdatePrice}>
                <DialogTrigger asChild>
                  <Button variant="outline">
                    <DollarSign className="mr-2 h-4 w-4" />
                    {t.order.updatePrice}
                  </Button>
                </DialogTrigger>
                <DialogContent>
                  <DialogHeader>
                    <DialogTitle>{t.order.updatePriceTitle}</DialogTitle>
                    <DialogDescription>{t.order.updatePriceDesc}</DialogDescription>
                  </DialogHeader>
                  <div className="space-y-4 py-4">
                    <div className="space-y-2">
                      <Label>{t.order.currentAmount}</Label>
                      <div className="text-lg font-semibold">
                        {formatCurrency(order.total_amount_minor ?? 0, order.currency || 'CNY')}
                      </div>
                    </div>
                    <div className="space-y-2">
                      <Label>{t.order.newAmount} *</Label>
                      <Input
                        type="number"
                        step="0.01"
                        min="0"
                        placeholder={t.order.newAmountPlaceholder}
                        value={newPrice}
                        onChange={(e) => setNewPrice(e.target.value)}
                      />
                    </div>
                  </div>
                  <DialogFooter>
                    <Button variant="outline" onClick={() => setOpenUpdatePrice(false)}>
                      {t.common.cancel}
                    </Button>
                    <Button
                      onClick={() => {
                        const nextPriceMinor = parseMajorToMinor(newPrice)
                        if (nextPriceMinor === null || nextPriceMinor < 0) {
                          toast.error(t.order.invalidPrice)
                          return
                        }
                        updatePriceMutation.mutate(nextPriceMinor)
                      }}
                      disabled={!newPrice || updatePriceMutation.isPending}
                    >
                      {updatePriceMutation.isPending ? t.admin.saving : t.order.confirmUpdate}
                    </Button>
                  </DialogFooter>
                </DialogContent>
              </Dialog>
            )}

            {/* 发货虚拟商品 */}
            {canDeliverVirtual && (
              <Dialog
                open={openDeliverVirtual}
                onOpenChange={(open) => {
                  setOpenDeliverVirtual(open)
                  if (!open) {
                    setMarkOnlyShipped(false)
                  }
                }}
              >
                <DialogTrigger asChild>
                  <Button>
                    <Key className="mr-2 h-4 w-4" />
                    {t.order.deliverVirtual}
                  </Button>
                </DialogTrigger>
                <DialogContent>
                  <DialogHeader>
                    <DialogTitle>{t.order.deliverVirtualTitle}</DialogTitle>
                    <DialogDescription>{t.order.deliverVirtualDesc}</DialogDescription>
                  </DialogHeader>
                  <div className="py-2">
                    <div className="flex items-start space-x-2 rounded-md border p-3">
                      <Checkbox
                        id="mark-only-shipped"
                        checked={markOnlyShipped}
                        onCheckedChange={(checked) => setMarkOnlyShipped(checked === true)}
                      />
                      <div className="space-y-1">
                        <Label htmlFor="mark-only-shipped" className="cursor-pointer">
                          {t.order.markOnlyCompleteLabel}
                        </Label>
                        <p className="text-xs text-muted-foreground">
                          {t.order.markOnlyCompleteDesc}
                        </p>
                      </div>
                    </div>
                  </div>
                  <DialogFooter>
                    <Button variant="outline" onClick={() => setOpenDeliverVirtual(false)}>
                      {t.common.cancel}
                    </Button>
                    <Button
                      onClick={() => deliverVirtualMutation.mutate(markOnlyShipped)}
                      disabled={deliverVirtualMutation.isPending}
                    >
                      {deliverVirtualMutation.isPending
                        ? t.order.delivering
                        : t.order.confirmDeliver}
                    </Button>
                  </DialogFooter>
                </DialogContent>
              </Dialog>
            )}

            {/* 编辑收货信息 - 虚拟商品不显示 */}
            {canEditShipping && (
              <Dialog open={openEdit} onOpenChange={setOpenEdit}>
                <DialogTrigger asChild>
                  <Button variant="outline">
                    <Edit className="mr-2 h-4 w-4" />
                    {t.order.editShippingInfo}
                  </Button>
                </DialogTrigger>
                <DialogContent className="max-h-[80vh] max-w-2xl overflow-y-auto">
                  <DialogHeader>
                    <DialogTitle>{t.order.editShippingInfo}</DialogTitle>
                  </DialogHeader>
                  <div className="space-y-4 py-4">
                    <div className="grid grid-cols-2 gap-4">
                      <div className="space-y-2">
                        <Label>{t.order.receiverNameLabel} *</Label>
                        <Input
                          value={editForm.receiver_name}
                          onChange={(e) =>
                            setEditForm({ ...editForm, receiver_name: e.target.value })
                          }
                        />
                      </div>
                      <div className="space-y-2">
                        <Label>{t.order.emailLabel} *</Label>
                        <Input
                          type="email"
                          value={editForm.receiver_email}
                          onChange={(e) =>
                            setEditForm({ ...editForm, receiver_email: e.target.value })
                          }
                        />
                      </div>
                    </div>

                    <div className="grid grid-cols-[120px_1fr] gap-2">
                      <div className="space-y-2">
                        <Label>{t.order.areaCode} *</Label>
                        <Input
                          value={editForm.phone_code}
                          onChange={(e) => setEditForm({ ...editForm, phone_code: e.target.value })}
                        />
                      </div>
                      <div className="space-y-2">
                        <Label>{t.order.phoneLabel} *</Label>
                        <Input
                          value={editForm.receiver_phone}
                          onChange={(e) =>
                            setEditForm({ ...editForm, receiver_phone: e.target.value })
                          }
                        />
                      </div>
                    </div>

                    <div className="space-y-2">
                      <Label>{t.order.countryRegion} *</Label>
                      <Select
                        value={editForm.receiver_country}
                        onValueChange={(value) =>
                          setEditForm({ ...editForm, receiver_country: value })
                        }
                      >
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent className="max-h-[300px]">
                          {countries.map((country) => (
                            <SelectItem key={country.code} value={country.code}>
                              {country.name_zh}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>

                    <div className="grid grid-cols-3 gap-4">
                      <div className="space-y-2">
                        <Label>{t.order.province}</Label>
                        <Input
                          value={editForm.receiver_province}
                          onChange={(e) =>
                            setEditForm({ ...editForm, receiver_province: e.target.value })
                          }
                        />
                      </div>
                      <div className="space-y-2">
                        <Label>{t.order.city}</Label>
                        <Input
                          value={editForm.receiver_city}
                          onChange={(e) =>
                            setEditForm({ ...editForm, receiver_city: e.target.value })
                          }
                        />
                      </div>
                      <div className="space-y-2">
                        <Label>{t.order.district}</Label>
                        <Input
                          value={editForm.receiver_district}
                          onChange={(e) =>
                            setEditForm({ ...editForm, receiver_district: e.target.value })
                          }
                        />
                      </div>
                    </div>

                    <div className="space-y-2">
                      <Label>{t.order.detailAddress} *</Label>
                      <Textarea
                        value={editForm.receiver_address}
                        onChange={(e) =>
                          setEditForm({ ...editForm, receiver_address: e.target.value })
                        }
                        rows={3}
                      />
                    </div>

                    <div className="space-y-2">
                      <Label>{t.order.postalCode}</Label>
                      <Input
                        value={editForm.receiver_postcode}
                        onChange={(e) =>
                          setEditForm({ ...editForm, receiver_postcode: e.target.value })
                        }
                      />
                    </div>
                  </div>
                  <DialogFooter>
                    <Button variant="outline" onClick={() => setOpenEdit(false)}>
                      {t.common.cancel}
                    </Button>
                    <Button onClick={() => editMutation.mutate()} disabled={editMutation.isPending}>
                      {editMutation.isPending ? t.admin.saving : t.order.save}
                    </Button>
                  </DialogFooter>
                </DialogContent>
              </Dialog>
            )}

            {/* 要求重填 - 虚拟商品不显示 */}
            {canRequestResubmit && (
              <Dialog open={openResubmit} onOpenChange={setOpenResubmit}>
                <DialogTrigger asChild>
                  <Button variant="outline">
                    <RotateCcw className="mr-2 h-4 w-4" />
                    {t.order.requestResubmit}
                  </Button>
                </DialogTrigger>
                <DialogContent>
                  <DialogHeader>
                    <DialogTitle>{t.order.requestResubmitTitle}</DialogTitle>
                    <DialogDescription>{t.order.requestResubmitDesc}</DialogDescription>
                  </DialogHeader>
                  <div className="space-y-4 py-4">
                    <div className="space-y-2">
                      <Label>{t.order.reasonLabel} *</Label>
                      <Textarea
                        placeholder={t.order.reasonPlaceholder}
                        value={resubmitReason}
                        onChange={(e) => setResubmitReason(e.target.value)}
                        rows={4}
                      />
                    </div>
                  </div>
                  <DialogFooter>
                    <Button variant="outline" onClick={() => setOpenResubmit(false)}>
                      {t.common.cancel}
                    </Button>
                    <Button
                      onClick={() => resubmitMutation.mutate()}
                      disabled={!resubmitReason || resubmitMutation.isPending}
                    >
                      {resubmitMutation.isPending ? t.admin.processing : t.order.confirmResubmit}
                    </Button>
                  </DialogFooter>
                </DialogContent>
              </Dialog>
            )}

            {/* 分配物流单号 - 虚拟商品不显示 */}
            {canAssignTracking && (
              <Dialog open={openTracking} onOpenChange={setOpenTracking}>
                <DialogTrigger asChild>
                  <Button>
                    <Truck className="mr-2 h-4 w-4" />
                    {t.order.assignTracking}
                  </Button>
                </DialogTrigger>
                <DialogContent>
                  <DialogHeader>
                    <DialogTitle>{t.order.assignTracking}</DialogTitle>
                  </DialogHeader>
                  <div className="space-y-4 py-4">
                    <div className="space-y-2">
                      <Label>{t.order.trackingNoLabel}</Label>
                      <Input
                        placeholder={t.order.trackingNoPlaceholder}
                        value={trackingNo}
                        onChange={(e) => setTrackingNo(e.target.value)}
                      />
                    </div>
                  </div>
                  <DialogFooter>
                    <Button variant="outline" onClick={() => setOpenTracking(false)}>
                      {t.common.cancel}
                    </Button>
                    <Button
                      onClick={() => assignMutation.mutate()}
                      disabled={!trackingNo || assignMutation.isPending}
                    >
                      {assignMutation.isPending ? t.order.assigning : t.order.confirmAssign}
                    </Button>
                  </DialogFooter>
                </DialogContent>
              </Dialog>
            )}

            {/* 标记完成 */}
            {canMarkComplete && (
              <Dialog open={openComplete} onOpenChange={setOpenComplete}>
                <DialogTrigger asChild>
                  <Button variant="default">
                    <CheckCircle className="mr-2 h-4 w-4" />
                    {t.order.markComplete}
                  </Button>
                </DialogTrigger>
                <DialogContent>
                  <DialogHeader>
                    <DialogTitle>{t.order.markCompleteTitle}</DialogTitle>
                  </DialogHeader>
                  <div className="space-y-4 py-4">
                    <div className="space-y-2">
                      <Label>{t.order.remarkOptional}</Label>
                      <Input
                        placeholder={t.order.remarkPlaceholder}
                        value={adminRemark}
                        onChange={(e) => setAdminRemark(e.target.value)}
                      />
                    </div>
                  </div>
                  <DialogFooter>
                    <Button variant="outline" onClick={() => setOpenComplete(false)}>
                      {t.common.cancel}
                    </Button>
                    <Button
                      onClick={() => completeMutation.mutate()}
                      disabled={completeMutation.isPending}
                    >
                      {completeMutation.isPending ? t.admin.processing : t.order.confirmComplete}
                    </Button>
                  </DialogFooter>
                </DialogContent>
              </Dialog>
            )}
            {secondaryActionCount > 0 ? (
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button variant="outline" className="gap-2">
                    {t.common.more}
                    <ChevronDown className="h-4 w-4" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" className="w-52">
                  {canCancel && (
                    <DropdownMenuItem
                      className="cursor-pointer gap-2"
                      onSelect={() => setOpenCancel(true)}
                    >
                      <XCircle className="h-4 w-4" />
                      {t.order.cancelOrder}
                    </DropdownMenuItem>
                  )}
                  {canRefund && (
                    <DropdownMenuItem
                      className="cursor-pointer gap-2"
                      onSelect={() => setOpenRefund(true)}
                    >
                      <Undo2 className="h-4 w-4" />
                      {t.order.refundOrder}
                    </DropdownMenuItem>
                  )}
                  {canConfirmRefund && (
                    <DropdownMenuItem
                      className="cursor-pointer gap-2"
                      onSelect={() => setOpenConfirmRefund(true)}
                    >
                      <CheckCircle className="h-4 w-4" />
                      {t.order.confirmRefundPending}
                    </DropdownMenuItem>
                  )}
                  {canDelete && (canCancel || canRefund || canConfirmRefund) ? (
                    <DropdownMenuSeparator />
                  ) : null}
                  {canDelete && (
                    <DropdownMenuItem
                      className="cursor-pointer gap-2 text-destructive focus:bg-destructive/10 focus:text-destructive"
                      onSelect={() => setOpenDelete(true)}
                    >
                      <Trash2 className="h-4 w-4" />
                      {t.order.delete}
                    </DropdownMenuItem>
                  )}
                </DropdownMenuContent>
              </DropdownMenu>
            ) : null}
          </div>

          {canCancel && (
            <Dialog open={openCancel} onOpenChange={setOpenCancel}>
              <DialogContent>
                <DialogHeader>
                  <DialogTitle>{t.order.cancelOrderTitle}</DialogTitle>
                  <DialogDescription>{t.order.cancelOrderDesc}</DialogDescription>
                </DialogHeader>
                <div className="space-y-4 py-4">
                  <div className="space-y-2">
                    <Label>{t.order.cancelReasonLabel}</Label>
                    <Textarea
                      placeholder={t.order.cancelReasonPlaceholder}
                      value={cancelReason}
                      onChange={(e) => setCancelReason(e.target.value)}
                      rows={3}
                    />
                  </div>
                </div>
                <DialogFooter>
                  <Button variant="outline" onClick={() => setOpenCancel(false)}>
                    {t.order.back}
                  </Button>
                  <Button
                    variant="destructive"
                    onClick={() => cancelMutation.mutate()}
                    disabled={cancelMutation.isPending}
                  >
                    {cancelMutation.isPending ? t.order.cancelling : t.order.confirmCancel}
                  </Button>
                </DialogFooter>
              </DialogContent>
            </Dialog>
          )}

          {canRefund && (
            <Dialog open={openRefund} onOpenChange={setOpenRefund}>
              <DialogContent>
                <DialogHeader>
                  <DialogTitle>{t.order.refundOrderTitle}</DialogTitle>
                  <DialogDescription>{t.order.refundOrderDesc}</DialogDescription>
                </DialogHeader>
                <div className="space-y-4 py-4">
                  <div className="space-y-2">
                    <Label>{t.order.refundReasonLabel}</Label>
                    <Textarea
                      placeholder={t.order.refundReasonPlaceholder}
                      value={refundReason}
                      onChange={(e) => setRefundReason(e.target.value)}
                      rows={3}
                    />
                  </div>
                </div>
                <DialogFooter>
                  <Button variant="outline" onClick={() => setOpenRefund(false)}>
                    {t.order.back}
                  </Button>
                  <Button
                    variant="destructive"
                    onClick={() => refundMutation.mutate()}
                    disabled={refundMutation.isPending}
                  >
                    {refundMutation.isPending ? t.order.refunding : t.order.confirmRefund}
                  </Button>
                </DialogFooter>
              </DialogContent>
            </Dialog>
          )}

          {canConfirmRefund && (
            <Dialog open={openConfirmRefund} onOpenChange={setOpenConfirmRefund}>
              <DialogContent>
                <DialogHeader>
                  <DialogTitle>{t.order.confirmRefundPendingTitle}</DialogTitle>
                  <DialogDescription>{t.order.confirmRefundPendingDesc}</DialogDescription>
                </DialogHeader>
                <div className="space-y-4 py-4">
                  <div className="space-y-2">
                    <Label>{t.order.refundTransactionIdLabel}</Label>
                    <Input
                      placeholder={t.order.refundTransactionIdPlaceholder}
                      value={confirmRefundTransactionId}
                      onChange={(e) => setConfirmRefundTransactionId(e.target.value)}
                    />
                  </div>
                  <div className="space-y-2">
                    <Label>{t.order.remarkOptional}</Label>
                    <Textarea
                      placeholder={t.order.refundConfirmRemarkPlaceholder}
                      value={confirmRefundRemark}
                      onChange={(e) => setConfirmRefundRemark(e.target.value)}
                      rows={3}
                    />
                  </div>
                </div>
                <DialogFooter>
                  <Button variant="outline" onClick={() => setOpenConfirmRefund(false)}>
                    {t.common.cancel}
                  </Button>
                  <Button
                    onClick={() => confirmRefundMutation.mutate()}
                    disabled={confirmRefundMutation.isPending}
                  >
                    {confirmRefundMutation.isPending
                      ? t.order.confirmingRefundPending
                      : t.order.confirmRefundPending}
                  </Button>
                </DialogFooter>
              </DialogContent>
            </Dialog>
          )}

          {canDelete && (
            <AlertDialog open={openDelete} onOpenChange={setOpenDelete}>
              <AlertDialogContent className="max-w-lg">
                <AlertDialogHeader>
                  <AlertDialogTitle>{t.order.confirmDeleteOrder}</AlertDialogTitle>
                  <AlertDialogDescription>{t.order.confirmDeleteOrderDesc}</AlertDialogDescription>
                </AlertDialogHeader>
                <div className="space-y-3">
                  <div className="rounded-md border border-destructive/30 bg-destructive/5 p-3 text-sm">
                    <div className="flex flex-wrap items-center gap-2">
                      <OrderStatusBadge status={order.status} />
                      <p className="text-xs text-muted-foreground">
                        {[t.common.delete, orderNumber].join(' · ')}
                      </p>
                    </div>
                    <p className="mt-2 font-medium">
                      {formatCurrency(order.total_amount_minor ?? 0, order.currency || 'CNY')}
                    </p>
                  </div>
                  <div className="rounded-md border border-input/60 bg-muted/10 p-3 text-sm">
                    <p className="text-xs text-muted-foreground">{t.order.createdAt}</p>
                    <p className="mt-1">{formatDate(order.createdAt || order.created_at || '')}</p>
                  </div>
                </div>
                <AlertDialogFooter>
                  <AlertDialogCancel>{t.order.cancel}</AlertDialogCancel>
                  <AlertDialogAction
                    onClick={() => deleteMutation.mutate()}
                    className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                    disabled={deleteMutation.isPending}
                  >
                    {deleteMutation.isPending ? t.order.deleting : t.order.confirmDeleteBtn}
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          )}
        </div>
      </div>

      <OrderDetail
        order={order}
        serials={serials}
        virtualStocks={virtualStocks}
        isVirtualOnly={isVirtualOnly}
        paymentCard={paymentCard}
        shippingFormURL={orderFormURL || undefined}
        shippingFormToken={orderFormToken || undefined}
        shippingFormExpiresAt={orderFormExpiresAt || undefined}
        showVirtualStockRemark
        showOperationalMeta
        pluginSlotNamespace="admin.order_detail"
        pluginSlotContext={adminOrderDetailPluginContext}
        pluginSlotPath={`/admin/orders/${orderId}`}
      />
      <PluginSlot slot="admin.order_detail.bottom" context={adminOrderDetailPluginContext} />
    </div>
  )
}
