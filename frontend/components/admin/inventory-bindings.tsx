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
import { useToast } from '@/hooks/use-toast'

interface InventoryBindingsProps {
  productId: number
  currentMode: 'fixed' | 'random'
}

export function InventoryBindings({ productId, currentMode }: InventoryBindingsProps) {
  const queryClient = useQueryClient()

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
      toast.success('库存模式已更新')
      queryClient.invalidateQueries({ queryKey: ['adminProduct', productId] })
    },
    onError: (error: Error) => {
      toast.error(error.message || '更新失败')
    },
  })

  // 创建绑定
  const createMutation = useMutation({
    mutationFn: ({ productId, data }: { productId: number; data: any }) =>
      createProductBinding(productId, data),
    onSuccess: () => {
      toast.success('库存绑定已添加')
      queryClient.invalidateQueries({ queryKey: ['productBindings', productId] })
      setShowAddDialog(false)
      resetAddForm()
    },
    onError: (error: Error) => {
      toast.error(error.message || '添加失败')
    },
  })

  // 删除绑定
  const deleteMutation = useMutation({
    mutationFn: ({ bindingId }: { bindingId: number }) =>
      deleteProductBinding(productId, bindingId),
    onSuccess: () => {
      toast.success('绑定已删除')
      queryClient.invalidateQueries({ queryKey: ['productBindings', productId] })
      setDeleteBindingId(null)
    },
    onError: (error: Error) => {
      toast.error(error.message || '删除失败')
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
      toast.success('绑定已更新')
      queryClient.invalidateQueries({ queryKey: ['productBindings', productId] })
    },
    onError: (error: Error) => {
      toast.error(error.message || '更新失败')
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
      toast.error('请选择库存配置')
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
        <CardTitle className="flex items-center justify-between">
          <span className="flex items-center gap-2">
            <LinkIcon className="h-5 w-5" />
            库存配置
          </span>
          <Badge variant={inventoryMode === 'random' ? 'default' : 'secondary'}>
            {inventoryMode === 'random' ? (
              <>
                <Sparkles className="h-3 w-3 mr-1" />
                盲盒模式
              </>
            ) : (
              <>
                <Package className="h-3 w-3 mr-1" />
                固定模式
              </>
            )}
          </Badge>
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-6">
        {/* 库存模式选择 */}
        <div className="space-y-2">
          <Label>库存模式</Label>
          <Select key={selectKey} value={inventoryMode} onValueChange={handleModeChange}>
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="fixed">
                <div className="flex items-center gap-2">
                  <Package className="h-4 w-4" />
                  固定模式 - 用户选择属性
                </div>
              </SelectItem>
              <SelectItem value="random">
                <div className="flex items-center gap-2">
                  <Sparkles className="h-4 w-4" />
                  盲盒模式 - 系统随机分配
                </div>
              </SelectItem>
            </SelectContent>
          </Select>
          <p className="text-sm text-muted-foreground">
            {inventoryMode === 'random'
              ? '盲盒模式：购买时系统会根据权重随机分配库存属性'
              : '固定模式：用户选择具体属性，系统匹配对应库存'}
          </p>
        </div>

        {/* 总库存统计 */}
        <div className="p-4 bg-muted rounded-lg">
          <div className="text-sm text-muted-foreground mb-1">总可用库存</div>
          <div className="text-2xl font-bold">{totalStock}</div>
        </div>

        {/* 库存绑定列表 */}
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <Label>已绑定的库存配置</Label>
            <Dialog open={showAddDialog} onOpenChange={setShowAddDialog}>
              <DialogTrigger asChild>
                <Button size="sm" variant="outline">
                  <Plus className="mr-2 h-4 w-4" />
                  添加库存绑定
                </Button>
              </DialogTrigger>
              <DialogContent>
                <DialogHeader>
                  <DialogTitle>添加库存绑定</DialogTitle>
                  <DialogDescription>
                    选择一个库存配置并设置绑定参数
                  </DialogDescription>
                </DialogHeader>
                <div className="space-y-4 py-4">
                  <div className="space-y-2">
                    <Label>库存配置 *</Label>
                    <Select value={selectedInventory} onValueChange={setSelectedInventory}>
                      <SelectTrigger>
                        <SelectValue placeholder="选择库存配置" />
                      </SelectTrigger>
                      <SelectContent>
                        {inventories.map((inv: any) => (
                          <SelectItem key={inv.id} value={inv.id.toString()}>
                            {inv.name} (剩余: {inv.stock - inv.sold_quantity - inv.reserved_quantity})
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
                    <Label htmlFor="is_random">参与盲盒随机分配</Label>
                  </div>

                  {isRandom && (
                    <div className="space-y-2">
                      <Label>权重</Label>
                      <Input
                        type="number"
                        min="1"
                        value={priority}
                        onChange={(e) => setPriority(e.target.value)}
                        placeholder="权重值（越大概率越高）"
                      />
                      <p className="text-xs text-muted-foreground">
                        权重用于盲盒随机分配，数值越大被抽中的概率越高
                      </p>
                    </div>
                  )}

                  <div className="space-y-2">
                    <Label>备注</Label>
                    <Textarea
                      value={notes}
                      onChange={(e) => setNotes(e.target.value)}
                      placeholder="绑定说明（可选）"
                      rows={2}
                    />
                  </div>
                </div>
                <DialogFooter>
                  <Button variant="outline" onClick={() => setShowAddDialog(false)}>
                    取消
                  </Button>
                  <Button onClick={handleAddBinding} disabled={createMutation.isPending}>
                    {createMutation.isPending ? '添加中...' : '添加'}
                  </Button>
                </DialogFooter>
              </DialogContent>
            </Dialog>
          </div>

          {isLoading ? (
            <div className="text-center py-8 text-muted-foreground">加载中...</div>
          ) : bindings.length === 0 ? (
            <div className="text-center py-8 text-muted-foreground border-2 border-dashed rounded-lg">
              暂无绑定的库存配置，请先添加
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>库存名称</TableHead>
                  <TableHead>属性</TableHead>
                  <TableHead>剩余库存</TableHead>
                  <TableHead>参与随机</TableHead>
                  <TableHead>权重</TableHead>
                  <TableHead>操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {bindings.map((binding: any) => (
                  <TableRow key={binding.id}>
                    <TableCell>
                      <div>
                        <div className="font-medium">{binding.inventory?.name || 'N/A'}</div>
                        {binding.inventory?.sku && (
                          <div className="text-sm text-muted-foreground">
                            SKU: {binding.inventory.sku}
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
              盲盒随机分配规则
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
                      • {binding.inventory?.name}: {probability}% 概率
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
            <AlertDialogTitle>确认删除</AlertDialogTitle>
            <AlertDialogDescription>
              确定要删除这个库存绑定吗？删除后该库存将不再与此商品关联。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction
              onClick={() =>
                deleteBindingId && deleteMutation.mutate({ bindingId: deleteBindingId })
              }
              className="bg-red-600 hover:bg-red-700"
            >
              删除
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Card>
  )
}
