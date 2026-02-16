'use client'

import { use, useState } from 'react'
import { useRouter } from 'next/navigation'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { getInventory, updateInventory, adjustStock } from '@/lib/api'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { ArrowLeft, Save, Package, Edit } from 'lucide-react'
import Link from 'next/link'
import { useToast } from '@/hooks/use-toast'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'

export default function InventoryDetailPage({
  params,
}: {
  params: Promise<{ id: string }>
}) {
  const { id } = use(params)
  const router = useRouter()
  const queryClient = useQueryClient()
  const toast = useToast()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.adminInventoryEdit)

  // 获取库存详情
  const { data, isLoading } = useQuery({
    queryKey: ['inventory', id],
    queryFn: () => getInventory(parseInt(id)),
  })

  const inventory = data?.data

  // 表单状态
  const [stock, setStock] = useState<string>('')
  const [availableQuantity, setAvailableQuantity] = useState<string>('')
  const [safetyStock, setSafetyStock] = useState<string>('')
  const [isActive, setIsActive] = useState(true)
  const [alertEmail, setAlertEmail] = useState<string>('')
  const [notes, setNotes] = useState<string>('')

  // 调整库存状态
  const [adjustStockValue, setAdjustStockValue] = useState<string>('')
  const [adjustAvailableValue, setAdjustAvailableValue] = useState<string>('')
  const [adjustReason, setAdjustReason] = useState<string>('')
  const [adjustNotes, setAdjustNotes] = useState<string>('')

  // 初始化表单
  useState(() => {
    if (inventory) {
      setStock(inventory.stock.toString())
      setAvailableQuantity(inventory.available_quantity.toString())
      setSafetyStock(inventory.safety_stock.toString())
      setIsActive(inventory.is_active)
      setAlertEmail(inventory.alert_email || '')
      setNotes(inventory.notes || '')
    }
  })

  // 更新库存配置
  const updateMutation = useMutation({
    mutationFn: (data: any) => updateInventory(parseInt(id), data),
    onSuccess: () => {
      toast.success(t.admin.invUpdateSuccess)
      queryClient.invalidateQueries({ queryKey: ['inventory', id] })
    },
    onError: (error: Error) => {
      toast.error(error.message || t.admin.invUpdateFailed)
    },
  })

  // 调整库存
  const adjustMutation = useMutation({
    mutationFn: (data: any) => adjustStock(parseInt(id), data),
    onSuccess: () => {
      toast.success(t.admin.invAdjustSuccess)
      queryClient.invalidateQueries({ queryKey: ['inventory', id] })
      setAdjustStockValue('')
      setAdjustAvailableValue('')
      setAdjustReason('')
      setAdjustNotes('')
    },
    onError: (error: Error) => {
      toast.error(error.message || t.admin.invAdjustFailed)
    },
  })

  const handleUpdate = (e: React.FormEvent) => {
    e.preventDefault()

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

    updateMutation.mutate({
      stock: stockNum,
      available_quantity: availableNum,
      safety_stock: parseInt(safetyStock) || 0,
      is_active: isActive,
      alert_email: alertEmail || undefined,
      notes: notes || undefined,
    })
  }

  const handleAdjust = (e: React.FormEvent) => {
    e.preventDefault()

    if (!adjustReason) {
      toast.error(t.admin.invEnterReason)
      return
    }

    const stockDelta = parseInt(adjustStockValue)
    const availableDelta = parseInt(adjustAvailableValue)

    if (isNaN(stockDelta)) {
      toast.error(t.admin.invEnterStockDelta)
      return
    }

    if (isNaN(availableDelta)) {
      toast.error(t.admin.invEnterAvailableDelta)
      return
    }

    // 验证调整后的值不会为负
    const newStock = inventory.stock + stockDelta
    const newAvailable = inventory.available_quantity + availableDelta

    if (newStock < 0) {
      toast.error(t.admin.invStockInsufficient.replace('{current}', inventory.stock).replace('{delta}', Math.abs(stockDelta).toString()))
      return
    }

    if (newAvailable < 0) {
      toast.error(t.admin.invAvailableInsufficient.replace('{current}', inventory.available_quantity).replace('{delta}', Math.abs(availableDelta).toString()))
      return
    }

    if (newAvailable > newStock) {
      toast.error(t.admin.invAvailableExceedAfterAdjust)
      return
    }

    adjustMutation.mutate({
      stock_delta: stockDelta,
      available_quantity_delta: availableDelta,
      reason: adjustReason,
      notes: adjustNotes || undefined,
    })
  }

  if (isLoading) {
    return <div className="flex items-center justify-center py-12">{t.common.loading}</div>
  }

  if (!inventory) {
    return <div className="flex items-center justify-center py-12">{t.admin.invNotFound}</div>
  }

  const getRemainingStock = () => {
    return inventory.stock - inventory.sold_quantity - inventory.reserved_quantity
  }

  const getAvailableStock = () => {
    const available = inventory.available_quantity - inventory.sold_quantity - inventory.reserved_quantity
    return Math.max(0, available)
  }

  const isLowStock = () => {
    return getRemainingStock() <= inventory.safety_stock
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
          <h1 className="text-lg md:text-xl font-bold">{t.admin.invDetailTitle.replace('{id}', id)}</h1>
          <p className="text-muted-foreground mt-1">
            {inventory.product?.name} - {inventory.sku}
          </p>
        </div>
      </div>

      {/* 库存统计 */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">{t.admin.invActualStock}</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{inventory.stock}</div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">{t.admin.invRemainingStock}</CardTitle>
          </CardHeader>
          <CardContent>
            <div className={`text-2xl font-bold ${isLowStock() ? 'text-red-600' : ''}`}>
              {getRemainingStock()}
            </div>
            {isLowStock() && (
              <Badge variant="destructive" className="mt-2">{t.admin.invLowStock}</Badge>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">{t.admin.invSoldQty}</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{inventory.sold_quantity}</div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">{t.admin.invReservedQty}</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{inventory.reserved_quantity}</div>
          </CardContent>
        </Card>
      </div>

      {/* 属性组合 */}
      <Card>
        <CardHeader>
          <CardTitle>{t.admin.invAttributeCombo}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex flex-wrap gap-2">
            {Object.entries(inventory.attributes || {}).map(([key, value]) => (
              <Badge key={key} variant="outline" className="text-base px-4 py-2">
                {key}: {value as string}
              </Badge>
            ))}
          </div>
        </CardContent>
      </Card>

      {/* 操作标签页 */}
      <Tabs defaultValue="edit" className="w-full">
        <TabsList className="grid w-full grid-cols-2">
          <TabsTrigger value="edit">
            <Edit className="mr-2 h-4 w-4" />
            {t.admin.invEditConfig}
          </TabsTrigger>
          <TabsTrigger value="adjust">
            <Package className="mr-2 h-4 w-4" />
            {t.admin.invAdjustStock}
          </TabsTrigger>
        </TabsList>

        {/* 编辑配置 */}
        <TabsContent value="edit">
          <form onSubmit={handleUpdate}>
            <Card>
              <CardHeader>
                <CardTitle>{t.admin.invEditConfigTitle}</CardTitle>
              </CardHeader>
              <CardContent className="space-y-6">
                <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                  <div className="space-y-2">
                    <Label htmlFor="stock">{t.admin.invStockQty}</Label>
                    <Input
                      id="stock"
                      type="number"
                      min="0"
                      value={stock}
                      onChange={(e) => setStock(e.target.value)}
                    />
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="available">{t.admin.invPurchasableQty}</Label>
                    <Input
                      id="available"
                      type="number"
                      min="0"
                      value={availableQuantity}
                      onChange={(e) => setAvailableQuantity(e.target.value)}
                    />
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="safety">{t.admin.invSafetyStock}</Label>
                    <Input
                      id="safety"
                      type="number"
                      min="0"
                      value={safetyStock}
                      onChange={(e) => setSafetyStock(e.target.value)}
                    />
                  </div>
                </div>

                <div className="space-y-2">
                  <Label htmlFor="email">{t.admin.invAlertEmail}</Label>
                  <Input
                    id="email"
                    type="email"
                    value={alertEmail}
                    onChange={(e) => setAlertEmail(e.target.value)}
                    placeholder="admin@example.com"
                  />
                </div>

                <div className="space-y-2">
                  <Label htmlFor="notes">{t.admin.invNotes}</Label>
                  <Textarea
                    id="notes"
                    value={notes}
                    onChange={(e) => setNotes(e.target.value)}
                    rows={3}
                  />
                </div>

                <div className="flex items-center space-x-2">
                  <Switch
                    id="active"
                    checked={isActive}
                    onCheckedChange={setIsActive}
                  />
                  <Label htmlFor="active">{t.admin.invEnableConfig}</Label>
                </div>

                <Button type="submit" disabled={updateMutation.isPending}>
                  <Save className="mr-2 h-4 w-4" />
                  {updateMutation.isPending ? t.admin.invSaving : t.admin.invSaveChanges}
                </Button>
              </CardContent>
            </Card>
          </form>
        </TabsContent>

        {/* 调整库存 */}
        <TabsContent value="adjust">
          <form onSubmit={handleAdjust}>
            <Card>
              <CardHeader>
                <CardTitle>{t.admin.invAdjustTitle}</CardTitle>
                <CardDescription>
                  {t.admin.invAdjustDesc}
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-6">
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <Label htmlFor="adjust-stock">{t.admin.invStockDelta}</Label>
                    <Input
                      id="adjust-stock"
                      type="number"
                      value={adjustStockValue}
                      onChange={(e) => setAdjustStockValue(e.target.value)}
                      placeholder={t.admin.invStockDeltaPlaceholder}
                    />
                    <p className="text-sm text-muted-foreground">
                      {t.admin.invCurrentStock.replace('{current}', inventory.stock).replace('{after}', (inventory.stock + (parseInt(adjustStockValue) || 0)).toString())}
                    </p>
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="adjust-available">{t.admin.invAvailableDelta}</Label>
                    <Input
                      id="adjust-available"
                      type="number"
                      value={adjustAvailableValue}
                      onChange={(e) => setAdjustAvailableValue(e.target.value)}
                      placeholder={t.admin.invStockDeltaPlaceholder}
                    />
                    <p className="text-sm text-muted-foreground">
                      {t.admin.invCurrentAvailable.replace('{current}', inventory.available_quantity).replace('{after}', (inventory.available_quantity + (parseInt(adjustAvailableValue) || 0)).toString())}
                    </p>
                  </div>
                </div>

                <div className="space-y-2">
                  <Label htmlFor="reason">{t.admin.invAdjustReason}</Label>
                  <Input
                    id="reason"
                    value={adjustReason}
                    onChange={(e) => setAdjustReason(e.target.value)}
                    placeholder={t.admin.invReasonPlaceholder}
                  />
                </div>

                <div className="space-y-2">
                  <Label htmlFor="adjust-notes">{t.admin.invAdjustNotes}</Label>
                  <Textarea
                    id="adjust-notes"
                    value={adjustNotes}
                    onChange={(e) => setAdjustNotes(e.target.value)}
                    placeholder={t.admin.invAdjustNotesPlaceholder}
                    rows={3}
                  />
                </div>

                <Button type="submit" disabled={adjustMutation.isPending}>
                  <Package className="mr-2 h-4 w-4" />
                  {adjustMutation.isPending ? t.admin.invAdjusting : t.admin.invConfirmAdjust}
                </Button>
              </CardContent>
            </Card>
          </form>
        </TabsContent>
      </Tabs>
    </div>
  )
}
