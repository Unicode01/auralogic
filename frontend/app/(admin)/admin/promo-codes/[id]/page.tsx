'use client'

import { useState, useEffect } from 'react'
import { useParams, useRouter } from 'next/navigation'
import { useQuery, useMutation } from '@tanstack/react-query'
import { getAdminPromoCode, updatePromoCode, getAdminProducts, getAdminOrders } from '@/lib/api'
import { DataTable } from '@/components/admin/data-table'
import { OrderStatusBadge } from '@/components/orders/order-status-badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import toast from 'react-hot-toast'
import { ArrowLeft, Save, Loader2 } from 'lucide-react'
import Link from 'next/link'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'

interface PromoCodeForm {
  code: string
  name: string
  description: string
  discount_type: 'percentage' | 'fixed'
  discount_value: number
  max_discount: number
  min_order_amount: number
  total_quantity: number
  status: string
  expires_at: string
  product_scope: 'all' | 'specific' | 'exclude'
  product_ids: number[]
}

export default function EditPromoCodePage() {
  const params = useParams()
  const router = useRouter()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.promoCode.editPromoCode)
  const promoCodeId = Number(params.id)
  const [ordersPage, setOrdersPage] = useState(1)

  const [form, setForm] = useState<PromoCodeForm>({
    code: '',
    name: '',
    description: '',
    discount_type: 'percentage',
    discount_value: 0,
    max_discount: 0,
    min_order_amount: 0,
    total_quantity: 0,
    status: 'active',
    expires_at: '',
    product_scope: 'all',
    product_ids: [],
  })

  // Fetch promo code details
  const { data: promoCodeData, isLoading } = useQuery({
    queryKey: ['adminPromoCode', promoCodeId],
    queryFn: () => getAdminPromoCode(promoCodeId),
    enabled: !!promoCodeId,
    refetchOnMount: 'always',
    staleTime: 0,
  })

  const promoDetail: any = promoCodeData?.data
  const promoCodeStr = String(promoDetail?.code || form.code || '').trim()

  const { data: relatedOrdersData, isLoading: relatedOrdersLoading } = useQuery({
    queryKey: ['adminOrdersByPromoCode', promoCodeId, ordersPage, promoCodeStr],
    queryFn: () =>
      getAdminOrders({
        page: ordersPage,
        limit: 10,
        promo_code_id: promoCodeId,
        promo_code: promoCodeStr || undefined,
      }),
    enabled: !!promoCodeStr,
  })

  // Fetch products for product selection
  const { data: productsData, isLoading: productsLoading } = useQuery({
    queryKey: ['adminProducts', 1, 100],
    queryFn: () => getAdminProducts({ page: 1, limit: 100 }),
    enabled: form.product_scope === 'specific' || form.product_scope === 'exclude',
  })

  const products = productsData?.data?.items || []

  // Populate form data
  useEffect(() => {
    if (promoCodeData?.data) {
      const promo = promoCodeData.data

      // Convert ISO date to datetime-local format
      let expiresAtLocal = ''
      if (promo.expires_at) {
        const date = new Date(promo.expires_at)
        // Format as YYYY-MM-DDTHH:MM for datetime-local input
        const year = date.getFullYear()
        const month = String(date.getMonth() + 1).padStart(2, '0')
        const day = String(date.getDate()).padStart(2, '0')
        const hours = String(date.getHours()).padStart(2, '0')
        const minutes = String(date.getMinutes()).padStart(2, '0')
        expiresAtLocal = `${year}-${month}-${day}T${hours}:${minutes}`
      }

      const productIds = promo.product_ids || []

      setForm({
        code: promo.code || '',
        name: promo.name || '',
        description: promo.description || '',
        discount_type: promo.discount_type || 'percentage',
        discount_value: promo.discount_value ?? 0,
        max_discount: promo.max_discount ?? 0,
        min_order_amount: promo.min_order_amount ?? 0,
        total_quantity: promo.total_quantity ?? 0,
        status: promo.status || 'active',
        expires_at: expiresAtLocal,
        product_scope: promo.product_scope || (productIds.length > 0 ? 'specific' : 'all'),
        product_ids: productIds,
      })
    }
  }, [promoCodeData])

  useEffect(() => {
    // Reset related orders pagination when switching promo code.
    setOrdersPage(1)
  }, [promoCodeId])

  const saveMutation = useMutation({
    mutationFn: async (data: PromoCodeForm) => {
      const submitData: any = {
        name: data.name,
        description: data.description || undefined,
        discount_type: data.discount_type || 'percentage',
        discount_value: data.discount_value,
        min_order_amount: data.min_order_amount,
        total_quantity: data.total_quantity,
        status: data.status,
      }

      if (data.discount_type === 'percentage' && data.max_discount > 0) {
        submitData.max_discount = data.max_discount
      } else {
        submitData.max_discount = 0
      }

      if (data.expires_at) {
        submitData.expires_at = new Date(data.expires_at).toISOString()
      } else {
        submitData.expires_at = undefined
      }

      if (data.product_scope === 'specific' || data.product_scope === 'exclude') {
        submitData.product_scope = data.product_scope
        if (data.product_ids.length > 0) {
          submitData.product_ids = data.product_ids
        }
      } else {
        submitData.product_ids = []
      }

      return updatePromoCode(promoCodeId, submitData)
    },
    onSuccess: () => {
      toast.success(t.promoCode.promoCodeUpdated)
      router.push('/admin/promo-codes')
    },
    onError: (error: Error) => {
      toast.error(`${t.promoCode.updateFailed}: ${error.message}`)
    },
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()

    if (!form.name) {
      toast.error(`${t.promoCode.name} is required`)
      return
    }

    if (form.discount_value <= 0) {
      toast.error(`${t.promoCode.discountValue} must be greater than 0`)
      return
    }

    if (form.discount_type === 'percentage' && form.discount_value > 100) {
      toast.error(`${t.promoCode.discountValue} cannot exceed 100%`)
      return
    }

    if ((form.product_scope === 'specific' || form.product_scope === 'exclude') && form.product_ids.length === 0) {
      toast.error(t.promoCode.selectProducts)
      return
    }

    saveMutation.mutate(form)
  }

  const handleProductToggle = (productId: number) => {
    setForm(prev => ({
      ...prev,
      product_ids: prev.product_ids.includes(productId)
        ? prev.product_ids.filter(id => id !== productId)
        : [...prev.product_ids, productId],
    }))
  }

  // Loading state
  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <div className="text-center">
          <Loader2 className="h-8 w-8 animate-spin mx-auto mb-4" />
          <p className="text-muted-foreground">{t.common.loading}</p>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Button variant="outline" size="sm" asChild>
          <Link href="/admin/promo-codes">
            <ArrowLeft className="mr-1.5 h-4 w-4" />
            <span className="hidden md:inline">{t.common.back}</span>
          </Link>
        </Button>
        <h1 className="text-lg md:text-xl font-bold">
          {t.promoCode.editPromoCode}
        </h1>
      </div>

      <form onSubmit={handleSubmit} className="space-y-6">
        {/* Basic Info */}
        <Card>
          <CardHeader>
            <CardTitle>{t.promoCode.editPromoCode}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label htmlFor="code">
                  {t.promoCode.code}
                </Label>
                <Input
                  id="code"
                  value={form.code}
                  readOnly
                  disabled
                  className="bg-muted"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="name">
                  {t.promoCode.name} <span className="text-red-500">*</span>
                </Label>
                <Input
                  id="name"
                  value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                  placeholder={t.promoCode.namePlaceholder}
                  required
                />
              </div>
            </div>

            <div className="space-y-2">
              <Label htmlFor="description">{t.promoCode.description}</Label>
              <Textarea
                id="description"
                value={form.description}
                onChange={(e) => setForm({ ...form, description: e.target.value })}
                placeholder={t.promoCode.descriptionPlaceholder}
                rows={3}
              />
            </div>
          </CardContent>
        </Card>

        {/* Discount Settings */}
        <Card>
          <CardHeader>
            <CardTitle>{t.promoCode.discountType}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label>{t.promoCode.discountType}</Label>
                <div className="flex items-center gap-2">
                  {([
                    { value: 'percentage' as const, label: t.promoCode.percentage },
                    { value: 'fixed' as const, label: t.promoCode.fixed },
                  ]).map((option) => (
                    <button
                      key={option.value}
                      type="button"
                      onClick={() => setForm({ ...form, discount_type: option.value })}
                      className={`px-4 py-2 text-sm rounded-lg border transition-all ${
                        form.discount_type === option.value
                          ? 'border-primary bg-primary text-primary-foreground font-medium'
                          : 'border-input hover:border-primary/50 text-foreground bg-background'
                      }`}
                    >
                      {option.label}
                    </button>
                  ))}
                </div>
              </div>
              <div className="space-y-2">
                <Label htmlFor="discount_value">
                  {t.promoCode.discountValue} <span className="text-red-500">*</span>
                </Label>
                <Input
                  id="discount_value"
                  type="number"
                  step="0.01"
                  min="0.01"
                  value={form.discount_value}
                  onChange={(e) => setForm({ ...form, discount_value: parseFloat(e.target.value) || 0 })}
                  required
                />
                <p className="text-xs text-muted-foreground">
                  {t.promoCode.discountValueHint}
                </p>
              </div>
            </div>

            {form.discount_type === 'percentage' && (
              <div className="space-y-2">
                <Label htmlFor="max_discount">{t.promoCode.maxDiscount}</Label>
                <Input
                  id="max_discount"
                  type="number"
                  step="0.01"
                  min="0"
                  value={form.max_discount}
                  onChange={(e) => setForm({ ...form, max_discount: parseFloat(e.target.value) || 0 })}
                  className="w-64"
                />
                <p className="text-xs text-muted-foreground">
                  {t.promoCode.maxDiscountHint}
                </p>
              </div>
            )}

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label htmlFor="min_order_amount">{t.promoCode.minOrderAmount}</Label>
                <Input
                  id="min_order_amount"
                  type="number"
                  step="0.01"
                  min="0"
                  value={form.min_order_amount}
                  onChange={(e) => setForm({ ...form, min_order_amount: parseFloat(e.target.value) || 0 })}
                />
                <p className="text-xs text-muted-foreground">
                  {t.promoCode.minOrderAmountHint}
                </p>
              </div>
              <div className="space-y-2">
                <Label htmlFor="total_quantity">{t.promoCode.totalQuantity}</Label>
                <Input
                  id="total_quantity"
                  type="number"
                  min="0"
                  value={form.total_quantity}
                  onChange={(e) => setForm({ ...form, total_quantity: parseInt(e.target.value) || 0 })}
                />
                <p className="text-xs text-muted-foreground">
                  {t.promoCode.totalQuantityHint}
                </p>
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Status & Expiry */}
        <Card>
          <CardHeader>
            <CardTitle>{t.promoCode.status}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label htmlFor="status">{t.promoCode.status}</Label>
                <Select
                  value={form.status}
                  onValueChange={(value) => setForm({ ...form, status: value })}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="active">{t.promoCode.active}</SelectItem>
                    <SelectItem value="inactive">{t.promoCode.inactive}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label htmlFor="expires_at">{t.promoCode.expiresAt}</Label>
                <Input
                  id="expires_at"
                  type="datetime-local"
                  value={form.expires_at}
                  onChange={(e) => setForm({ ...form, expires_at: e.target.value })}
                  className="dark:[color-scheme:dark]"
                />
                <p className="text-xs text-muted-foreground">
                  {t.promoCode.expiresAtHint}
                </p>
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Applicable Products */}
        <Card>
          <CardHeader>
            <CardTitle>{t.promoCode.applicableProducts}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex items-center gap-2 flex-wrap">
              {([
                { value: 'all' as const, label: t.promoCode.allProducts },
                { value: 'specific' as const, label: t.promoCode.specificProducts },
                { value: 'exclude' as const, label: t.promoCode.excludeProducts },
              ]).map((option) => (
                <button
                  key={option.value}
                  type="button"
                  onClick={() => setForm({ ...form, product_scope: option.value, ...(option.value === 'all' ? { product_ids: [] } : {}) })}
                  className={`px-4 py-2 text-sm rounded-lg border transition-all ${
                    form.product_scope === option.value
                      ? 'border-primary bg-primary text-primary-foreground font-medium'
                      : 'border-input hover:border-primary/50 text-foreground bg-background'
                  }`}
                >
                  {option.label}
                </button>
              ))}
            </div>

            {(form.product_scope === 'specific' || form.product_scope === 'exclude') && (
              <div className="space-y-3">
                {form.product_ids.length > 0 && (
                  <p className="text-sm text-muted-foreground">
                    {form.product_scope === 'exclude'
                      ? t.promoCode.excludedProducts.replace('{count}', form.product_ids.length.toString())
                      : t.promoCode.selectedProducts.replace('{count}', form.product_ids.length.toString())}
                  </p>
                )}
                <div className="border rounded-lg max-h-64 overflow-y-auto">
                  {productsLoading ? (
                    <div className="text-center py-8 text-muted-foreground">
                      {t.common.loading}
                    </div>
                  ) : products.length === 0 ? (
                    <div className="text-center py-8 text-muted-foreground">
                      {t.promoCode.noProducts}
                    </div>
                  ) : (
                    <div className="divide-y">
                      {products.map((product: any) => (
                        <label
                          key={product.id}
                          className="flex items-center gap-3 px-4 py-3 hover:bg-muted/50 cursor-pointer transition-colors"
                        >
                          <Checkbox
                            checked={form.product_ids.includes(product.id)}
                            onCheckedChange={() => handleProductToggle(product.id)}
                          />
                          <div className="flex-1 min-w-0">
                            <p className="text-sm font-medium truncate">{product.name}</p>
                            <p className="text-xs text-muted-foreground">{product.sku}</p>
                          </div>
                        </label>
                      ))}
                    </div>
                  )}
                </div>
              </div>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t.promoCode.usage}</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
              <div className="rounded-lg border bg-background p-4">
                <div className="text-xs text-muted-foreground">{t.promoCode.usedQuantity}</div>
                <div className="mt-1 text-2xl font-semibold tabular-nums">
                  {promoDetail?.used_quantity ?? 0}
                </div>
              </div>
              <div className="rounded-lg border bg-background p-4">
                <div className="text-xs text-muted-foreground">{t.promoCode.reservedQuantity}</div>
                <div className="mt-1 text-2xl font-semibold tabular-nums">
                  {promoDetail?.reserved_quantity ?? 0}
                </div>
              </div>
              <div className="rounded-lg border bg-background p-4">
                <div className="text-xs text-muted-foreground">{t.promoCode.totalQuantity}</div>
                <div className="mt-1 text-2xl font-semibold tabular-nums">
                  {(promoDetail?.total_quantity ?? 0) === 0 ? t.promoCode.unlimited : (promoDetail?.total_quantity ?? 0)}
                </div>
              </div>
              <div className="rounded-lg border bg-background p-4">
                <div className="text-xs text-muted-foreground">{t.promoCode.availableQuantity}</div>
                <div className="mt-1 text-2xl font-semibold tabular-nums">
                  {(promoDetail?.total_quantity ?? 0) === 0
                    ? t.promoCode.unlimited
                    : Math.max(
                      0,
                      (promoDetail?.total_quantity ?? 0) -
                        (promoDetail?.used_quantity ?? 0) -
                        (promoDetail?.reserved_quantity ?? 0),
                    )}
                </div>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between gap-3">
            <div>
              <CardTitle>{t.promoCode.relatedOrders}</CardTitle>
              <p className="text-sm text-muted-foreground mt-1">
                {t.promoCode.relatedOrdersHint}
              </p>
            </div>
            <Button asChild variant="outline" size="sm">
              <Link href={`/admin/orders?promo_code=${encodeURIComponent(promoCodeStr)}`}>
                {t.common.view}
              </Link>
            </Button>
          </CardHeader>
          <CardContent>
            <DataTable
              columns={[
                {
                  header: t.admin.orderNo,
                  cell: ({ row }: { row: { original: any } }) => (
                    <span className="font-mono text-sm">
                      {row.original.orderNo || row.original.order_no}
                    </span>
                  ),
                },
                {
                  header: t.admin.status,
                  cell: ({ row }: { row: { original: any } }) => (
                    <OrderStatusBadge status={row.original.status} />
                  ),
                },
                {
                  header: t.order.totalAmount,
                  cell: ({ row }: { row: { original: any } }) => {
                    const amount = row.original.totalAmount ?? row.original.total_amount
                    return <span className="tabular-nums">{amount ?? '-'}</span>
                  },
                },
                {
                  header: t.admin.createdAt,
                  cell: ({ row }: { row: { original: any } }) => {
                    const date = new Date(row.original.createdAt || row.original.created_at)
                    return (
                      <span className="text-sm">
                        {date.toLocaleDateString(locale === 'zh' ? 'zh-CN' : 'en-US')}
                      </span>
                    )
                  },
                },
                {
                  header: t.admin.actions,
                  cell: ({ row }: { row: { original: any } }) => (
                    <Button asChild size="sm" variant="outline">
                      <Link href={`/admin/orders/${row.original.id}`}>{t.admin.view}</Link>
                    </Button>
                  ),
                },
              ]}
              data={relatedOrdersData?.data?.items || []}
              isLoading={relatedOrdersLoading}
              pagination={{
                page: ordersPage,
                total_pages: relatedOrdersData?.data?.pagination?.total_pages || 1,
                onPageChange: setOrdersPage,
              }}
            />
          </CardContent>
        </Card>

        {/* Actions */}
        <div className="flex justify-end gap-4">
          <Button type="button" variant="outline" asChild>
            <Link href="/admin/promo-codes">{t.common.cancel}</Link>
          </Button>
          <Button type="submit" disabled={saveMutation.isPending}>
            <Save className="mr-2 h-4 w-4" />
            {saveMutation.isPending ? t.admin.saving : t.common.save}
          </Button>
        </div>
      </form>
    </div>
  )
}
