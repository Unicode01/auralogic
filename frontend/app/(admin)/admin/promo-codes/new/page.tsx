'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { useMutation, useQuery } from '@tanstack/react-query'
import { createPromoCode, getAdminProducts } from '@/lib/api'
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

export default function CreatePromoCodePage() {
  const router = useRouter()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.promoCode.addPromoCode)

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

  // Fetch products for product selection
  const { data: productsData, isLoading: productsLoading } = useQuery({
    queryKey: ['adminProducts', 1, 100],
    queryFn: () => getAdminProducts({ page: 1, limit: 100 }),
    enabled: form.product_scope === 'specific' || form.product_scope === 'exclude',
  })

  const products = productsData?.data?.items || []

  const saveMutation = useMutation({
    mutationFn: async (data: PromoCodeForm) => {
      const submitData: any = {
        code: data.code,
        name: data.name,
        description: data.description || undefined,
        discount_type: data.discount_type,
        discount_value: data.discount_value,
        min_order_amount: data.min_order_amount,
        total_quantity: data.total_quantity,
        status: data.status,
      }

      if (data.discount_type === 'percentage' && data.max_discount > 0) {
        submitData.max_discount = data.max_discount
      }

      if (data.expires_at) {
        submitData.expires_at = new Date(data.expires_at).toISOString()
      }

      if (data.product_scope === 'specific' || data.product_scope === 'exclude') {
        submitData.product_scope = data.product_scope
        if (data.product_ids.length > 0) {
          submitData.product_ids = data.product_ids
        }
      }

      return createPromoCode(submitData)
    },
    onSuccess: () => {
      toast.success(t.promoCode.promoCodeCreated)
      router.push('/admin/promo-codes')
    },
    onError: (error: Error) => {
      toast.error(`${t.promoCode.createFailed}: ${error.message}`)
    },
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()

    if (!form.code) {
      toast.error(`${t.promoCode.code} is required`)
      return
    }

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
          {t.promoCode.addPromoCode}
        </h1>
      </div>

      <form onSubmit={handleSubmit} className="space-y-6">
        {/* Basic Info */}
        <Card>
          <CardHeader>
            <CardTitle>{t.promoCode.addPromoCode}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label htmlFor="code">
                  {t.promoCode.code} <span className="text-red-500">*</span>
                </Label>
                <Input
                  id="code"
                  value={form.code}
                  onChange={(e) => setForm({ ...form, code: e.target.value.toUpperCase() })}
                  placeholder={t.promoCode.codePlaceholder}
                  required
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

        {/* Actions */}
        <div className="flex justify-end gap-4">
          <Button type="button" variant="outline" asChild>
            <Link href="/admin/promo-codes">{t.common.cancel}</Link>
          </Button>
          <Button type="submit" disabled={saveMutation.isPending}>
            <Save className="mr-2 h-4 w-4" />
            {saveMutation.isPending ? t.admin.creating : t.common.save}
          </Button>
        </div>
      </form>
    </div>
  )
}
