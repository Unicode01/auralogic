'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { useMutation } from '@tanstack/react-query'
import { createInventory } from '@/lib/api'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { ArrowLeft, Plus, Trash2 } from 'lucide-react'
import Link from 'next/link'
import { useToast } from '@/hooks/use-toast'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'

export default function CreateInventoryPage() {
  const router = useRouter()
  const toast = useToast()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.adminInventoryNew)

  const [name, setName] = useState<string>('')
  const [sku, setSku] = useState<string>('')
  const [attributes, setAttributes] = useState<Array<{ key: string; value: string }>>([
    { key: '', value: '' }
  ])
  const [stock, setStock] = useState<string>('0')
  const [availableQuantity, setAvailableQuantity] = useState<string>('0')
  const [safetyStock, setSafetyStock] = useState<string>('0')
  const [alertEmail, setAlertEmail] = useState<string>('')
  const [notes, setNotes] = useState<string>('')

  const createMutation = useMutation({
    mutationFn: createInventory,
    onSuccess: () => {
      toast.success(t.admin.inventoryCreated)
      router.push('/admin/inventories')
    },
    onError: (error: Error) => {
      toast.error(error.message || t.admin.inventoryCreateFailed)
    },
  })

  const handleAddAttribute = () => {
    setAttributes([...attributes, { key: '', value: '' }])
  }

  const handleRemoveAttribute = (index: number) => {
    setAttributes(attributes.filter((_, i) => i !== index))
  }

  const handleAttributeChange = (index: number, field: 'key' | 'value', value: string) => {
    const newAttributes = [...attributes]
    newAttributes[index][field] = value
    setAttributes(newAttributes)
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()

    if (!name) {
      toast.error(t.admin.enterInventoryName)
      return
    }

    // 过滤掉空的属性
    const validAttributes = attributes.filter(attr => attr.key && attr.value)
    const attributesObj: Record<string, string> = {}
    validAttributes.forEach(attr => {
      attributesObj[attr.key] = attr.value
    })

    const stockNum = parseInt(stock)
    const availableNum = parseInt(availableQuantity)

    if (isNaN(stockNum) || stockNum < 0) {
      toast.error(t.admin.stockMustBeNonNeg)
      return
    }

    if (isNaN(availableNum) || availableNum < 0) {
      toast.error(t.admin.purchasableMustBeNonNeg)
      return
    }

    if (availableNum > stockNum) {
      toast.error(t.admin.purchasableCannotExceedStock)
      return
    }

    createMutation.mutate({
      name,
      sku: sku || undefined,
      attributes: attributesObj,
      stock: stockNum,
      available_quantity: availableNum,
      safety_stock: parseInt(safetyStock) || 0,
      alert_email: alertEmail || undefined,
      notes: notes || undefined,
    })
  }

  return (
    <div className="space-y-6">
      {/* 页面标题 */}
      <div className="flex items-center gap-4">
        <Button variant="outline" size="sm" asChild>
          <Link href="/admin/inventories">
            <ArrowLeft className="mr-1.5 h-4 w-4" />
            <span className="hidden md:inline">{t.common.back}</span>
          </Link>
        </Button>
        <div>
          <h1 className="text-lg md:text-xl font-bold">{t.admin.createInventoryTitle}</h1>
          <p className="text-muted-foreground mt-1">
            {t.admin.createInventoryDesc}
          </p>
        </div>
      </div>

      {/* 创建表单 */}
      <form onSubmit={handleSubmit}>
        <Card>
          <CardHeader>
            <CardTitle>{t.admin.basicInfoTitle}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-6">
            {/* 库存名称 */}
            <div className="space-y-2">
              <Label htmlFor="name">{t.admin.inventoryNameLabel}</Label>
              <Input
                id="name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder={t.admin.inventoryNameInputPlaceholder}
                required
              />
              <p className="text-sm text-muted-foreground">
                {t.admin.inventoryNameHint}
              </p>
            </div>

            {/* SKU */}
            <div className="space-y-2">
              <Label htmlFor="sku">{t.admin.skuOptionalLabel}</Label>
              <Input
                id="sku"
                value={sku}
                onChange={(e) => setSku(e.target.value)}
                placeholder={t.admin.skuInputPlaceholder}
              />
            </div>

            {/* 属性组合 */}
            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <Label>{t.admin.attributeComboLabel}</Label>
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  onClick={handleAddAttribute}
                >
                  <Plus className="mr-2 h-4 w-4" />
                  {t.admin.addAttribute}
                </Button>
              </div>
              <div className="space-y-2">
                {attributes.map((attr, index) => (
                  <div key={index} className="flex gap-2">
                    <Input
                      placeholder={t.admin.attrKeyPlaceholder}
                      value={attr.key}
                      onChange={(e) => handleAttributeChange(index, 'key', e.target.value)}
                    />
                    <Input
                      placeholder={t.admin.attrValuePlaceholder}
                      value={attr.value}
                      onChange={(e) => handleAttributeChange(index, 'value', e.target.value)}
                    />
                    {attributes.length > 1 && (
                      <Button
                        type="button"
                        size="icon"
                        variant="destructive"
                        onClick={() => handleRemoveAttribute(index)}
                      >
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    )}
                  </div>
                ))}
              </div>
            </div>

            {/* 库存数量 */}
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              <div className="space-y-2">
                <Label htmlFor="stock">{t.admin.stockQuantityLabel}</Label>
                <Input
                  id="stock"
                  type="number"
                  min="0"
                  value={stock}
                  onChange={(e) => setStock(e.target.value)}
                  placeholder="100"
                  required
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="available">{t.admin.purchasableLabel}</Label>
                <Input
                  id="available"
                  type="number"
                  min="0"
                  value={availableQuantity}
                  onChange={(e) => setAvailableQuantity(e.target.value)}
                  placeholder="80"
                  required
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="safety">{t.admin.safetyStockLabel}</Label>
                <Input
                  id="safety"
                  type="number"
                  min="0"
                  value={safetyStock}
                  onChange={(e) => setSafetyStock(e.target.value)}
                  placeholder="10"
                />
              </div>
            </div>

            {/* 告警邮箱 */}
            <div className="space-y-2">
              <Label htmlFor="email">{t.admin.alertEmailLabel}</Label>
              <Input
                id="email"
                type="email"
                value={alertEmail}
                onChange={(e) => setAlertEmail(e.target.value)}
                placeholder="admin@example.com"
              />
            </div>

            {/* 备注 */}
            <div className="space-y-2">
              <Label htmlFor="notes">{t.admin.notesLabel}</Label>
              <Textarea
                id="notes"
                value={notes}
                onChange={(e) => setNotes(e.target.value)}
                placeholder={t.admin.notesInputPlaceholder}
                rows={3}
              />
            </div>

            {/* 提交按钮 */}
            <div className="flex gap-4">
              <Button
                type="submit"
                disabled={createMutation.isPending}
              >
                {createMutation.isPending ? t.admin.creatingInventory : t.admin.createInventoryBtn}
              </Button>
              <Button
                type="button"
                variant="outline"
                asChild
              >
                <Link href="/admin/inventories">{t.common.cancel}</Link>
              </Button>
            </div>
          </CardContent>
        </Card>
      </form>
    </div>
  )
}
