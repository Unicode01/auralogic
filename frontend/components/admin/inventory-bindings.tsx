'use client'

import { useState, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  getProductBindings,
  createProductBinding,
  updateProductBinding,
  deleteProductBinding,
  getInventories,
  updateProductInventoryMode,
} from '@/lib/api'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Plus, Trash2, Link as LinkIcon, Package, Sparkles } from 'lucide-react'
import toast from 'react-hot-toast'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { resolveApiErrorMessage } from '@/lib/api-error'

interface InventoryBindingsProps {
  productId: number
  currentMode: 'fixed' | 'random'
}

export function InventoryBindings({ productId, currentMode }: InventoryBindingsProps) {
  const { locale } = useLocale()
  const t = getTranslations(locale)
  const queryClient = useQueryClient()
  const getErrorMessage = (error: unknown, fallback: string) =>
    resolveApiErrorMessage(error, t, fallback)

  // 所有状态声明
  const [inventoryMode, setInventoryMode] = useState<'fixed' | 'random'>(currentMode)
  const [selectKey, setSelectKey] = useState(0)
  const [showAddDialog, setShowAddDialog] = useState(false)
  const [selectedInventory, setSelectedInventory] = useState<string>('')
  const [isRandom, setIsRandom] = useState(false)
  const [priority, setPriority] = useState('1')
  const [notes, setNotes] = useState('')
  const [deleteBindingId, setDeleteBindingId] = useState<number | null>(null)

  // 当currentMode改变时，更新本地状态并强制Select重新渲染
  useEffect(() => {
    setInventoryMode(currentMode)
    setSelectKey(prev => prev + 1)
  }, [currentMode])

  // 获取商品的库存绑定
  const { data: bindingsData, isLoading } = useQuery({
    queryKey: ['productBindings', productId],
    queryFn: () => getProductBindings(productId),
  })

  // 获取所有库存列表（用于选择）
  const { data: inventoriesData } = useQuery({
    queryKey: ['inventories', 1, 100],
    queryFn: () => getInventories({ page: 1, limit: 100 }),
  })

  const bindings = bindingsData?.data || []
  const inventories = inventoriesData?.data?.items || []

  // 更新库存模式
  const updateModeMutation = useMutation({
    mutationFn: (mode: 'fixed' | 'random') => updateProductInventoryMode(productId, mode),
    onSuccess: () => {
      toast.success(t.admin.updateSuccess)
      queryClient.invalidateQueries({ queryKey: ['adminProduct', productId] })
    },
    onError: (error: unknown) => {
      toast.error(getErrorMessage(error, t.admin.updateFailed))
    },
  })

  // 创建绑定
  const createMutation = useMutation({
    mutationFn: ({ productId, data }: { productId: number; data: any }) =>
      createProductBinding(productId, data),
    onSuccess: () => {
      toast.success(t.admin.bindingSuccess)
      queryClient.invalidateQueries({ queryKey: ['productBindings', productId] })
      setShowAddDialog(false)
      resetAddForm()
    },
    onError: (error: unknown) => {
      toast.error(getErrorMessage(error, t.admin.bindingFailed))
    },
  })

  // 删除绑定
  const deleteMutation = useMutation({
    mutationFn: ({ bindingId }: { bindingId: number }) =>
      deleteProductBinding(productId, bindingId),
    onSuccess: () => {
      toast.success(t.admin.unbindSuccess)
      queryClient.invalidateQueries({ queryKey: ['productBindings', productId] })
      setDeleteBindingId(null)
    },
    onError: (error: unknown) => {
      toast.error(getErrorMessage(error, t.admin.deleteFailed))
    },
  })

  // 更新绑定
  const updateMutation = useMutation({
    mutationFn: ({
      bindingId,
      data,
    }: {
      bindingId: number
      data: { is_random: boolean; priority: number; notes?: string }
    }) => updateProductBinding(productId, bindingId, data),
    onSuccess: () => {
      toast.success(t.admin.updateSuccess)
      queryClient.invalidateQueries({ queryKey: ['productBindings', productId] })
    },
    onError: (error: unknown) => {
      toast.error(getErrorMessage(error, t.admin.updateFailed))
    },
  })

  const resetAddForm = () => {
    setSelectedInventory('')
    setIsRandom(false)
    setPriority('1')
    setNotes('')
  }

  const handleAddBinding = () => {
    if (!selectedInventory) {
      toast.error(t.admin.selectInventory)
      return
    }

    createMutation.mutate({
      productId,
      data: {
        inventory_id: parseInt(selectedInventory),
        is_random: isRandom,
        priority: parseInt(priority) || 1,
        notes,
      },
    })
  }

  const handleModeChange = (newMode: 'fixed' | 'random') => {
    if (newMode !== inventoryMode) {
      updateModeMutation.mutate(newMode)
      setInventoryMode(newMode)
    }
  }

  const totalStock = bindings.reduce((sum: number, binding: any) => {
    return sum + (binding.inventory?.remaining_stock || 0)
  }, 0)

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <LinkIcon className="h-5 w-5" />
          {t.admin.inventoryConfig}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-6">
        {/* 库存模式选择 */}
        <div className="space-y-2">
          <Label>{t.admin.inventoryModeLabel}</Label>
          <Select key={selectKey} value={inventoryMode} onValueChange={handleModeChange}>
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="fixed">
                <div className="flex items-center gap-2">
                  <Package className="h-4 w-4" />
                  {t.admin.inventoryModeFixedWithDesc}
                </div>
              </SelectItem>
              <SelectItem value="random">
                <div className="flex items-center gap-2">
                  <Sparkles className="h-4 w-4" />
                  {t.admin.inventoryModeRandomWithDesc}
                </div>
              </SelectItem>
            </SelectContent>
          </Select>
          <p className="text-sm text-muted-foreground">
            {inventoryMode === 'random'
              ? t.admin.inventoryModeRandomHint
              : t.admin.inventoryModeFixedHint}
          </p>
        </div>

        {/* 总库存统计 */}
        <div className="p-4 bg-muted rounded-lg">
          <div className="text-sm text-muted-foreground mb-1">{t.admin.totalAvailableStock}</div>
          <div className="text-2xl font-bold">{totalStock}</div>
        </div>

        {/* 库存绑定列表 */}
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <Label>{t.admin.boundInventoryConfigs}</Label>
            <Dialog open={showAddDialog} onOpenChange={setShowAddDialog}>
              <DialogTrigger asChild>
                <Button size="sm" variant="outline">
                  <Plus className="mr-2 h-4 w-4" />
                  {t.admin.addInventoryBinding}
                </Button>
              </DialogTrigger>
              <DialogContent>
                <DialogHeader>
                  <DialogTitle>{t.admin.addInventoryBinding}</DialogTitle>
                  <DialogDescription>
                    {t.admin.addInventoryBindingDesc}
                  </DialogDescription>
                </DialogHeader>
                <div className="space-y-4 py-4">
                  <div className="space-y-2">
                    <Label>{t.admin.inventoryConfigRequired}</Label>
                    <Select value={selectedInventory} onValueChange={setSelectedInventory}>
                      <SelectTrigger>
                        <SelectValue placeholder={t.admin.inventorySelectPlaceholder} />
                      </SelectTrigger>
                      <SelectContent>
                        {inventories.map((inv: any) => (
                          <SelectItem key={inv.id} value={inv.id.toString()}>
                            {inv.name} ({t.admin.inventoryRemainingInline.replace('{count}', String(inv.stock - inv.sold_quantity - inv.reserved_quantity))})
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>

                  <div className="flex items-center space-x-2">
                    <Switch
                      id="is_random"
                      checked={isRandom}
                      onCheckedChange={setIsRandom}
                    />
                    <Label htmlFor="is_random">{t.admin.randomAssignEnabledLabel}</Label>
                  </div>

                  {isRandom && (
                    <div className="space-y-2">
                      <Label>{t.admin.weight}</Label>
                      <Input
                        type="number"
                        min="1"
                        value={priority}
                        onChange={(e) => setPriority(e.target.value)}
                        placeholder={t.admin.weightPlaceholder}
                      />
                      <p className="text-xs text-muted-foreground">
                        {t.admin.weightHint}
                      </p>
                    </div>
                  )}

                  <div className="space-y-2">
                    <Label>{t.admin.bindingNotesLabel}</Label>
                    <Textarea
                      value={notes}
                      onChange={(e) => setNotes(e.target.value)}
                      placeholder={t.admin.bindingNotesPlaceholder}
                      rows={2}
                    />
                  </div>
                </div>
                <DialogFooter>
                  <Button variant="outline" onClick={() => setShowAddDialog(false)}>
                    {t.common.cancel}
                  </Button>
                  <Button onClick={handleAddBinding} disabled={createMutation.isPending}>
                    {createMutation.isPending ? t.admin.addBindingPending : t.admin.addInventoryBinding}
                  </Button>
                </DialogFooter>
              </DialogContent>
            </Dialog>
          </div>

          {isLoading ? (
            <div className="text-center py-8 text-muted-foreground">{t.common.loading}</div>
          ) : bindings.length === 0 ? (
            <div className="text-center py-8 text-muted-foreground border-2 border-dashed rounded-lg">
              {t.admin.noBoundInventoryConfig}
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t.admin.inventoryNameCol}</TableHead>
                  <TableHead>{t.product.attributes}</TableHead>
                  <TableHead>{t.admin.remainingStockCol}</TableHead>
                  <TableHead>{t.admin.randomParticipationCol}</TableHead>
                  <TableHead>{t.admin.weight}</TableHead>
                  <TableHead>{t.admin.actions}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {bindings.map((binding: any) => (
                  <TableRow key={binding.id}>
                    <TableCell>
                      <div>
                        <div className="font-medium">{binding.inventory?.name || t.common.noData}</div>
                        {binding.inventory?.sku && (
                          <div className="text-sm text-muted-foreground">
                            {t.admin.skuCol}: {binding.inventory.sku}
                          </div>
                        )}
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className="flex flex-wrap gap-1">
                        {Object.entries(binding.inventory?.attributes || {}).map(
                          ([key, value]) => (
                            <Badge key={key} variant="outline" className="text-xs">
                              {key}: {value as string}
                            </Badge>
                          )
                        )}
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge
                        variant={
                          (binding.inventory?.remaining_stock || 0) > 0
                            ? 'default'
                            : 'destructive'
                        }
                      >
                        {binding.inventory?.remaining_stock || 0}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <Switch
                        checked={binding.is_random}
                        onCheckedChange={(checked) => {
                          updateMutation.mutate({
                            bindingId: binding.id,
                            data: {
                              is_random: checked,
                              priority: binding.priority,
                              notes: binding.notes,
                            },
                          })
                        }}
                      />
                    </TableCell>
                    <TableCell>
                      <Input
                        type="number"
                        min="1"
                        value={binding.priority}
                        onChange={(e) => {
                          const newPriority = parseInt(e.target.value) || 1
                          updateMutation.mutate({
                            bindingId: binding.id,
                            data: {
                              is_random: binding.is_random,
                              priority: newPriority,
                              notes: binding.notes,
                            },
                          })
                        }}
                        className="w-20"
                        disabled={!binding.is_random}
                      />
                    </TableCell>
                    <TableCell>
                      <Button
                        size="sm"
                        variant="destructive"
                        onClick={() => setDeleteBindingId(binding.id)}
                      >
                        <Trash2 className="h-3 w-3" />
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </div>

        {/* 盲盒模式说明 */}
        {inventoryMode === 'random' && bindings.length > 0 && (
          <div className="p-4 bg-blue-50 dark:bg-blue-950 border border-blue-200 dark:border-blue-800 rounded-lg">
            <h4 className="font-medium text-blue-900 dark:text-blue-200 mb-2 flex items-center gap-2">
              <Sparkles className="h-4 w-4" />
              {t.admin.blindBoxRules}
            </h4>
            <div className="text-sm text-blue-800 dark:text-blue-200 space-y-1">
              {bindings
                .filter((b: any) => b.is_random)
                .map((binding: any) => {
                  const totalWeight = bindings
                    .filter((b: any) => b.is_random)
                    .reduce((sum: number, b: any) => sum + (b.priority || 1), 0)
                  const probability =
                    totalWeight > 0
                      ? ((binding.priority / totalWeight) * 100).toFixed(1)
                      : '0.0'
                  return (
                    <div key={binding.id}>
                      • {t.admin.blindBoxProbabilityLine
                        .replace('{name}', binding.inventory?.name || t.common.noData)
                        .replace('{probability}', probability)}
                    </div>
                  )
                })}
            </div>
          </div>
        )}
      </CardContent>

      {/* 删除确认对话框 */}
      <AlertDialog
        open={deleteBindingId !== null}
        onOpenChange={() => setDeleteBindingId(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t.admin.confirmDelete}</AlertDialogTitle>
            <AlertDialogDescription>
              {t.admin.deleteBindingConfirmDesc}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t.common.cancel}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() =>
                deleteBindingId && deleteMutation.mutate({ bindingId: deleteBindingId })
              }
              className="bg-red-600 hover:bg-red-700"
            >
              {t.common.delete}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Card>
  )
}
