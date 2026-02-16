'use client'

import { use, useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useRouter } from 'next/navigation'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'
import { getAdminOrderDetail, assignTracking, adminCompleteOrder, adminCancelOrder, adminDeleteOrder, updateOrderShippingInfo, requestOrderResubmit, getCountries, adminMarkOrderAsPaid, updateOrderPrice, adminDeliverVirtualStock } from '@/lib/api'
import { OrderDetail } from '@/components/orders/order-detail'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
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
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { ArrowLeft, Truck, CheckCircle, XCircle, Trash2, Edit, RotateCcw, CreditCard, Wallet, Clock, Coins, DollarSign, Key } from 'lucide-react'
import * as LucideIcons from 'lucide-react'
import Link from 'next/link'
import { useToast } from '@/hooks/use-toast'
import { useEffect } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { formatDate } from '@/lib/utils'

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
  const [resubmitReason, setResubmitReason] = useState('')
  const [openTracking, setOpenTracking] = useState(false)
  const [openComplete, setOpenComplete] = useState(false)
  const [openCancel, setOpenCancel] = useState(false)
  const [openEdit, setOpenEdit] = useState(false)
  const [openResubmit, setOpenResubmit] = useState(false)
  const [openUpdatePrice, setOpenUpdatePrice] = useState(false)
  const [newPrice, setNewPrice] = useState('')

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

  const { data, isLoading } = useQuery({
    queryKey: ['adminOrderDetail', orderId],
    queryFn: () => getAdminOrderDetail(orderId),
    enabled: !!orderId,
  })

  const assignMutation = useMutation({
    mutationFn: () => assignTracking(orderId, { tracking_no: trackingNo }),
    onSuccess: () => {
      toast.success(t.order.trackingAssigned)
      queryClient.invalidateQueries({ queryKey: ['adminOrderDetail', orderId] })
      setOpenTracking(false)
      setTrackingNo('')
    },
    onError: (error: any) => {
      toast.error(error.message || t.order.assignFailed)
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
      toast.error(error.message || t.order.operationFailed)
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
      toast.error(error.message || t.order.cancelFailed)
    },
  })

  const deleteMutation = useMutation({
    mutationFn: () => adminDeleteOrder(orderId),
    onSuccess: () => {
      toast.success(t.order.orderDeleted)
      router.push('/admin/orders')
    },
    onError: (error: any) => {
      toast.error(error.message || t.order.deleteFailed)
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
      toast.error(error.message || t.order.updateFailed)
    },
  })

  const resubmitMutation = useMutation({
    mutationFn: () => requestOrderResubmit(orderId, resubmitReason),
    onSuccess: () => {
      toast.success(t.order.resubmitRequested)
      queryClient.invalidateQueries({ queryKey: ['adminOrderDetail', orderId] })
      setOpenResubmit(false)
      setResubmitReason('')
    },
    onError: (error: any) => {
      toast.error(error.message || t.order.operationFailed)
    },
  })

  const markPaidMutation = useMutation({
    mutationFn: () => adminMarkOrderAsPaid(orderId),
    onSuccess: () => {
      toast.success(t.order.orderMarkedPaid)
      queryClient.invalidateQueries({ queryKey: ['adminOrderDetail', orderId] })
    },
    onError: (error: any) => {
      toast.error(error.message || t.order.operationFailed)
    },
  })

  const updatePriceMutation = useMutation({
    mutationFn: (price: number) => updateOrderPrice(orderId, price),
    onSuccess: () => {
      toast.success(t.order.priceUpdated)
      queryClient.invalidateQueries({ queryKey: ['adminOrderDetail', orderId] })
      setOpenUpdatePrice(false)
      setNewPrice('')
    },
    onError: (error: any) => {
      toast.error(error.message || t.order.updateFailed)
    },
  })

  const deliverVirtualMutation = useMutation({
    mutationFn: () => adminDeliverVirtualStock(orderId),
    onSuccess: () => {
      toast.success(t.order.virtualDelivered)
      queryClient.invalidateQueries({ queryKey: ['adminOrderDetail', orderId] })
    },
    onError: (error: any) => {
      toast.error(error.message || t.order.deliverFailed)
    },
  })

  // 获取国家列表
  useEffect(() => {
    getCountries()
      .then((response: any) => {
        setCountries(response.data || [])
      })
      .catch(err => {
        console.error('获取国家列表失败:', err)
      })
  }, [])

  // 辅助函数：检查字段是否被打码
  const isMaskedValue = (value: string | undefined) => {
    if (!value) return false
    // 检查是否为打码标记
    return value === '***' || value.includes('****')
  }

  // 辅助函数：处理可能被打码的字段
  const handleMaskedField = (value: string | undefined, defaultValue: string = '') => {
    if (!value) return defaultValue
    // 如果是打码值，返回空字符串（让管理员重新填写）
    if (isMaskedValue(value)) return ''
    // 否则返回原值
    return value
  }

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
      setNewPrice(order.total_amount?.toString() || order.totalAmount?.toString() || '')
    }
  }, [data])

  if (isLoading) {
    return <div className="text-center py-12">{locale === 'zh' ? '加载中...' : 'Loading...'}</div>
  }

  if (!data?.data) {
    return <div className="text-center py-12">{t.order.orderNotFound}</div>
  }

  // 处理新的数据结构：{order, serials, virtual_stocks} 或 旧结构直接是order
  const order = data.data.order || data.data
  const serials = data.data.serials || []
  const virtualStocks = data.data.virtual_stocks || []
  const paymentInfo = data.data.payment_info

  // 判断是否为纯虚拟商品订单
  const isVirtualOnly = order.items?.every((item: any) =>
    (item.product_type || item.productType) === 'virtual'
  ) ?? false

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

  // 获取付款方式图标
  const getPaymentIcon = () => {
    const iconName = paymentInfo?.payment_method?.icon
    if (!iconName) return <Wallet className="h-5 w-5" />

    // 检查是否是 lucide 图标名
    const IconComponent = (LucideIcons as any)[iconName]
    if (IconComponent) {
      return <IconComponent className="h-5 w-5" />
    }

    return <Wallet className="h-5 w-5" />
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
          <div className="flex items-center justify-center h-10 w-10 rounded-full bg-primary/10">
            {getPaymentIcon()}
          </div>
          <div>
            <p className="font-medium">{paymentInfo.payment_method?.name || t.order.unknownPaymentMethod}</p>
            <p className="text-sm text-muted-foreground">
              {paymentInfo.payment_method?.type === 'custom' ? t.order.customPaymentMethod : t.order.builtinPaymentMethod}
            </p>
          </div>
        </div>

        <div className="grid grid-cols-2 gap-4 text-sm">
          <div>
            <dt className="text-muted-foreground flex items-center gap-1">
              <Clock className="h-3.5 w-3.5" />
              {t.order.selectedAt}
            </dt>
            <dd>{formatDate(paymentInfo.selected_at)}</dd>
          </div>
          {paymentData?.paid_at && (
            <div>
              <dt className="text-muted-foreground flex items-center gap-1">
                <CheckCircle className="h-3.5 w-3.5" />
                {t.order.paidAt}
              </dt>
              <dd>{formatDate(paymentData.paid_at)}</dd>
            </div>
          )}
          {paymentData?.transaction_id && (
            <div className="col-span-2">
              <dt className="text-muted-foreground">{t.order.transactionId}</dt>
              <dd className="font-mono text-xs break-all">{paymentData.transaction_id}</dd>
            </div>
          )}
        </div>
      </CardContent>
    </Card>
  ) : null

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <Button asChild variant="outline" size="sm">
            <Link href="/admin/orders">
              <ArrowLeft className="mr-1.5 h-4 w-4" />
              <span className="hidden md:inline">{t.order.backToListShort}</span>
            </Link>
          </Button>
          <h1 className="text-lg md:text-xl font-bold">{t.order.orderDetail}</h1>
        </div>

        <div className="flex gap-2">
          {/* 标记已付款 */}
          {order.status === 'pending_payment' && (
            <Button
              onClick={() => markPaidMutation.mutate()}
              disabled={markPaidMutation.isPending}
            >
              <CreditCard className="mr-2 h-4 w-4" />
              {markPaidMutation.isPending ? t.admin.processing : t.order.markPaid}
            </Button>
          )}

          {/* 修改订单价格 */}
          {order.status === 'pending_payment' && (
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
                  <DialogDescription>
                    {t.order.updatePriceDesc}
                  </DialogDescription>
                </DialogHeader>
                <div className="space-y-4 py-4">
                  <div className="space-y-2">
                    <Label>{t.order.currentAmount}</Label>
                    <div className="text-lg font-semibold">
                      {order.currency || 'CNY'} {order.total_amount || order.totalAmount || 0}
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
                      const price = parseFloat(newPrice)
                      if (isNaN(price) || price < 0) {
                        toast.error(t.order.invalidPrice)
                        return
                      }
                      updatePriceMutation.mutate(price)
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
          {order.status === 'pending' && isVirtualOnly && virtualStocks.length === 0 && (
            <AlertDialog>
              <AlertDialogTrigger asChild>
                <Button>
                  <Key className="mr-2 h-4 w-4" />
                  {t.order.deliverVirtual}
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>{t.order.deliverVirtualTitle}</AlertDialogTitle>
                  <AlertDialogDescription>
                    {t.order.deliverVirtualDesc}
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>{t.common.cancel}</AlertDialogCancel>
                  <AlertDialogAction
                    onClick={() => deliverVirtualMutation.mutate()}
                    disabled={deliverVirtualMutation.isPending}
                  >
                    {deliverVirtualMutation.isPending ? t.order.delivering : t.order.confirmDeliver}
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          )}

          {/* 编辑收货信息 - 虚拟商品不显示 */}
          {(order.status === 'pending' || order.status === 'need_resubmit') && !isVirtualOnly && (
            <Dialog open={openEdit} onOpenChange={setOpenEdit}>
              <DialogTrigger asChild>
                <Button variant="outline">
                  <Edit className="mr-2 h-4 w-4" />
                  {t.order.editShippingInfo}
                </Button>
              </DialogTrigger>
              <DialogContent className="max-w-2xl max-h-[80vh] overflow-y-auto">
                <DialogHeader>
                  <DialogTitle>{t.order.editShippingInfo}</DialogTitle>
                </DialogHeader>
                <div className="space-y-4 py-4">
                  <div className="grid grid-cols-2 gap-4">
                    <div className="space-y-2">
                      <Label>{t.order.receiverNameLabel} *</Label>
                      <Input
                        value={editForm.receiver_name}
                        onChange={(e) => setEditForm({ ...editForm, receiver_name: e.target.value })}
                      />
                    </div>
                    <div className="space-y-2">
                      <Label>{t.order.emailLabel} *</Label>
                      <Input
                        type="email"
                        value={editForm.receiver_email}
                        onChange={(e) => setEditForm({ ...editForm, receiver_email: e.target.value })}
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
                        onChange={(e) => setEditForm({ ...editForm, receiver_phone: e.target.value })}
                      />
                    </div>
                  </div>

                  <div className="space-y-2">
                    <Label>{t.order.countryRegion} *</Label>
                    <Select
                      value={editForm.receiver_country}
                      onValueChange={(value) => setEditForm({ ...editForm, receiver_country: value })}
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
                        onChange={(e) => setEditForm({ ...editForm, receiver_province: e.target.value })}
                      />
                    </div>
                    <div className="space-y-2">
                      <Label>{t.order.city}</Label>
                      <Input
                        value={editForm.receiver_city}
                        onChange={(e) => setEditForm({ ...editForm, receiver_city: e.target.value })}
                      />
                    </div>
                    <div className="space-y-2">
                      <Label>{t.order.district}</Label>
                      <Input
                        value={editForm.receiver_district}
                        onChange={(e) => setEditForm({ ...editForm, receiver_district: e.target.value })}
                      />
                    </div>
                  </div>

                  <div className="space-y-2">
                    <Label>{t.order.detailAddress} *</Label>
                    <Textarea
                      value={editForm.receiver_address}
                      onChange={(e) => setEditForm({ ...editForm, receiver_address: e.target.value })}
                      rows={3}
                    />
                  </div>

                  <div className="space-y-2">
                    <Label>{t.order.postalCode}</Label>
                    <Input
                      value={editForm.receiver_postcode}
                      onChange={(e) => setEditForm({ ...editForm, receiver_postcode: e.target.value })}
                    />
                  </div>
                </div>
                <DialogFooter>
                  <Button variant="outline" onClick={() => setOpenEdit(false)}>
                    {t.common.cancel}
                  </Button>
                  <Button
                    onClick={() => editMutation.mutate()}
                    disabled={editMutation.isPending}
                  >
                    {editMutation.isPending ? t.admin.saving : t.order.save}
                  </Button>
                </DialogFooter>
              </DialogContent>
            </Dialog>
          )}

          {/* 要求重填 - 虚拟商品不显示 */}
          {order.status === 'pending' && !isVirtualOnly && (
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
                  <DialogDescription>
                    {t.order.requestResubmitDesc}
                  </DialogDescription>
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
          {order.status === 'pending' && !isVirtualOnly && !order.trackingNo && !order.tracking_no && (
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
          {order.status === 'shipped' && (
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

          {/* 取消订单 */}
          {(order.status === 'pending_payment' || order.status === 'draft' || order.status === 'pending' || order.status === 'need_resubmit') && (
            <Dialog open={openCancel} onOpenChange={setOpenCancel}>
              <DialogTrigger asChild>
                <Button variant="outline">
                  <XCircle className="mr-2 h-4 w-4" />
                  {t.order.cancelOrder}
                </Button>
              </DialogTrigger>
              <DialogContent>
                <DialogHeader>
                  <DialogTitle>{t.order.cancelOrderTitle}</DialogTitle>
                  <DialogDescription>
                    {t.order.cancelOrderDesc}
                  </DialogDescription>
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

          {/* 删除订单 */}
          {(order.status === 'pending_payment' || order.status === 'draft' || order.status === 'cancelled') && (
            <AlertDialog>
              <AlertDialogTrigger asChild>
                <Button variant="destructive" size="sm">
                  <Trash2 className="mr-2 h-4 w-4" />
                  {t.order.delete}
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>{t.order.confirmDeleteOrder}</AlertDialogTitle>
                  <AlertDialogDescription>
                    {t.order.confirmDeleteOrderDesc}
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>{t.order.cancel}</AlertDialogCancel>
                  <AlertDialogAction
                    onClick={() => deleteMutation.mutate()}
                    className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                  >
                    {deleteMutation.isPending ? t.order.deleting : t.order.confirmDeleteBtn}
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          )}
        </div>
      </div>

      <OrderDetail order={order} serials={serials} virtualStocks={virtualStocks} isVirtualOnly={isVirtualOnly} paymentCard={paymentCard} />
    </div>
  )
}

