'use client'

import { Suspense, useState, useEffect, useRef } from 'react'
import { useSearchParams } from 'next/navigation'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  getInventories,
  getLowStockList,
  deleteInventory,
  getVirtualInventories,
  createVirtualInventory,
  deleteVirtualInventory,
  importVirtualInventoryStock,
  createVirtualInventoryStockManually
} from '@/lib/api'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Label } from '@/components/ui/label'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from '@/components/ui/tabs'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Package, AlertTriangle, Plus, Edit, Trash2, RefreshCw, Database, FileText, Upload } from 'lucide-react'
import Link from 'next/link'
import { useToast } from '@/hooks/use-toast'
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
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'

export default function InventoriesPage() {
  return (
    <Suspense fallback={
      <div className="flex items-center justify-center py-12">
        <div className="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full" />
      </div>
    }>
      <InventoriesContent />
    </Suspense>
  )
}

function InventoriesContent() {
  const searchParams = useSearchParams()
  const queryClient = useQueryClient()
  const toast = useToast()
  const fileInputRef = useRef<HTMLInputElement>(null)
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.adminInventories)

  const [page, setPage] = useState(1)
  const [virtualPage, setVirtualPage] = useState(1)
  const [limit] = useState(20)
  const [productId, setProductId] = useState<string>('')
  const [isActiveFilter, setIsActiveFilter] = useState<string>('all')
  const [showLowStock, setShowLowStock] = useState(false)
  const [activeTab, setActiveTab] = useState<string>('physical')
  const [virtualSearch, setVirtualSearch] = useState('')

  // 创建虚拟库存对话框状态
  const [createDialogOpen, setCreateDialogOpen] = useState(false)
  const [newVirtualInventory, setNewVirtualInventory] = useState({
    name: '',
    sku: '',
    description: '',
    is_active: true,
    notes: ''
  })

  // 导入库存对话框状态
  const [importDialogOpen, setImportDialogOpen] = useState(false)
  const [selectedVirtualInventoryId, setSelectedVirtualInventoryId] = useState<number | null>(null)
  const [importType, setImportType] = useState<'file' | 'text'>('text')
  const [textContent, setTextContent] = useState('')
  const [selectedFile, setSelectedFile] = useState<File | null>(null)

  // 手动创建库存项对话框
  const [manualCreateDialogOpen, setManualCreateDialogOpen] = useState(false)
  const [manualStockContent, setManualStockContent] = useState('')
  const [manualStockRemark, setManualStockRemark] = useState('')

  // 从URL参数中读取product_id和tab
  useEffect(() => {
    const productIdParam = searchParams.get('product_id')
    if (productIdParam) {
      setProductId(productIdParam)
    }
    const tabParam = searchParams.get('tab')
    if (tabParam === 'virtual') {
      setActiveTab('virtual')
    }
  }, [searchParams])

  // 获取实物库存列表
  const { data, isLoading, refetch } = useQuery({
    queryKey: ['inventories', page, limit, productId, isActiveFilter, showLowStock],
    queryFn: () => {
      if (showLowStock) {
        return getLowStockList()
      }

      const params: any = { page, limit }
      if (productId) params.product_id = parseInt(productId)
      if (isActiveFilter !== 'all') params.is_active = isActiveFilter === 'true'

      return getInventories(params)
    },
  })

  // 获取虚拟库存列表
  const { data: virtualData, isLoading: virtualLoading, refetch: refetchVirtual } = useQuery({
    queryKey: ['virtualInventories', virtualPage, limit, virtualSearch],
    queryFn: () => getVirtualInventories({ page: virtualPage, limit, search: virtualSearch }),
  })

  // 创建虚拟库存
  const createVirtualMutation = useMutation({
    mutationFn: createVirtualInventory,
    onSuccess: () => {
      toast.success(t.admin.virtualCreated)
      setCreateDialogOpen(false)
      setNewVirtualInventory({ name: '', sku: '', description: '', is_active: true, notes: '' })
      refetchVirtual()
    },
    onError: (error: Error) => {
      toast.error(`${t.admin.createFailed}: ${error.message}`)
    },
  })

  // 导入虚拟库存项
  const importMutation = useMutation({
    mutationFn: (data: { virtualInventoryId: number; import_type: 'file' | 'text'; file?: File; content?: string }) =>
      importVirtualInventoryStock(data.virtualInventoryId, data),
    onSuccess: (response: any) => {
      toast.success(t.admin.importSuccess.replace('{count}', String(response?.data?.count || 0)))
      setImportDialogOpen(false)
      setTextContent('')
      setSelectedFile(null)
      setSelectedVirtualInventoryId(null)
      refetchVirtual()
    },
    onError: (error: Error) => {
      toast.error(`${t.admin.createFailed}: ${error.message}`)
    },
  })

  // 手动创建单个库存项
  const manualCreateMutation = useMutation({
    mutationFn: (data: { virtualInventoryId: number; content: string; remark?: string }) =>
      createVirtualInventoryStockManually(data.virtualInventoryId, data),
    onSuccess: () => {
      toast.success(t.admin.stockItemCreated)
      setManualCreateDialogOpen(false)
      setManualStockContent('')
      setManualStockRemark('')
      setSelectedVirtualInventoryId(null)
      refetchVirtual()
    },
    onError: (error: Error) => {
      toast.error(`${t.admin.createFailed}: ${error.message}`)
    },
  })

  // 删除实物库存配置
  const deleteMutation = useMutation({
    mutationFn: deleteInventory,
    onSuccess: () => {
      toast.success(t.admin.deleteSuccess)
      queryClient.invalidateQueries({ queryKey: ['inventories'] })
    },
    onError: (error: Error) => {
      toast.error(error.message || t.admin.deleteFailed)
    },
  })

  // 删除虚拟库存
  const deleteVirtualMutation = useMutation({
    mutationFn: deleteVirtualInventory,
    onSuccess: () => {
      toast.success(t.admin.virtualDeleted)
      refetchVirtual()
    },
    onError: (error: Error) => {
      toast.error(error.message || t.admin.deleteFailed)
    },
  })

  // 数据处理
  const inventories = showLowStock ? (data?.data || []) : (data?.data?.items || [])
  const pagination = showLowStock ? null : data?.data?.pagination
  const virtualInventories = virtualData?.data?.list || []
  const virtualPagination = virtualData?.data

  // 计算剩余库存
  const getRemainingStock = (inventory: any) => {
    return inventory.stock - inventory.sold_quantity - inventory.reserved_quantity
  }

  // 是否低库存
  const isLowStock = (inventory: any) => {
    return getRemainingStock(inventory) <= inventory.safety_stock
  }

  // 处理文件选择
  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (file) {
      const validTypes = ['.xlsx', '.xls', '.csv', '.txt']
      const ext = file.name.substring(file.name.lastIndexOf('.')).toLowerCase()
      if (!validTypes.includes(ext)) {
        toast.error(t.admin.onlySupportedFormats)
        return
      }
      setSelectedFile(file)
    }
  }

  // 处理导入
  const handleImport = () => {
    if (!selectedVirtualInventoryId) {
      toast.error(t.admin.pleaseSelectVirtualInventory)
      return
    }

    if (importType === 'text') {
      if (!textContent.trim()) {
        toast.error(t.admin.pleaseInputContent)
        return
      }
      importMutation.mutate({
        virtualInventoryId: selectedVirtualInventoryId,
        import_type: 'text',
        content: textContent
      })
    } else {
      if (!selectedFile) {
        toast.error(t.admin.pleaseSelectFile)
        return
      }
      importMutation.mutate({
        virtualInventoryId: selectedVirtualInventoryId,
        import_type: 'file',
        file: selectedFile
      })
    }
  }

  // 处理手动创建
  const handleManualCreate = () => {
    if (!selectedVirtualInventoryId) {
      toast.error(t.admin.pleaseSelectVirtualInventory)
      return
    }
    if (!manualStockContent.trim()) {
      toast.error(t.admin.pleaseInputCardKey)
      return
    }
    manualCreateMutation.mutate({
      virtualInventoryId: selectedVirtualInventoryId,
      content: manualStockContent,
      remark: manualStockRemark
    })
  }

  // 处理创建虚拟库存
  const handleCreateVirtualInventory = () => {
    if (!newVirtualInventory.name.trim()) {
      toast.error(t.admin.pleaseInputInventoryName)
      return
    }
    createVirtualMutation.mutate(newVirtualInventory)
  }

  // 打开导入对话框
  const openImportDialog = (virtualInventoryId: number) => {
    setSelectedVirtualInventoryId(virtualInventoryId)
    setImportDialogOpen(true)
  }

  // 打开手动创建对话框
  const openManualCreateDialog = (virtualInventoryId: number) => {
    setSelectedVirtualInventoryId(virtualInventoryId)
    setManualCreateDialogOpen(true)
  }

  return (
    <div className="space-y-6">
      {/* 页面标题 */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">{t.admin.inventoryManagement}</h1>
          <p className="text-muted-foreground mt-1">
            {t.admin.inventoryDesc}
          </p>
        </div>
        <div className="flex gap-2">
          {activeTab === 'physical' && (
            <Button asChild>
              <Link href="/admin/inventories/new">
                <Plus className="mr-2 h-4 w-4" />
                {t.admin.createPhysical}
              </Link>
            </Button>
          )}
          {activeTab === 'virtual' && (
            <Button onClick={() => setCreateDialogOpen(true)}>
              <Plus className="mr-2 h-4 w-4" />
              {t.admin.createVirtual}
            </Button>
          )}
        </div>
      </div>

      {/* 标签页切换 */}
      <Tabs value={activeTab} onValueChange={setActiveTab}>
        <TabsList>
          <TabsTrigger value="physical" className="flex items-center gap-2">
            <Package className="h-4 w-4" />
            {t.admin.physicalInventory}
            {inventories.length > 0 && (
              <Badge variant="secondary" className="ml-1">{pagination?.total || inventories.length}</Badge>
            )}
          </TabsTrigger>
          <TabsTrigger value="virtual" className="flex items-center gap-2">
            <FileText className="h-4 w-4" />
            {t.admin.virtualInventory}
            {virtualInventories.length > 0 && (
              <Badge variant="secondary" className="ml-1">{virtualPagination?.total || virtualInventories.length}</Badge>
            )}
          </TabsTrigger>
        </TabsList>

        {/* 实物库存标签内容 */}
        <TabsContent value="physical" className="space-y-4">
          {/* 过滤器 */}
          <Card>
            <CardContent className="pt-6">
              <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
                <div>
                  <label className="text-sm font-medium mb-2 block">{t.admin.productId}</label>
                  <Input
                    placeholder={t.admin.productIdPlaceholder}
                    value={productId}
                    onChange={(e) => setProductId(e.target.value)}
                  />
                </div>
                <div>
                  <label className="text-sm font-medium mb-2 block">{t.admin.status}</label>
                  <Select value={isActiveFilter} onValueChange={setIsActiveFilter}>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="all">{t.admin.all}</SelectItem>
                      <SelectItem value="true">{t.admin.enabled}</SelectItem>
                      <SelectItem value="false">{t.admin.disabled}</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div>
                  <label className="text-sm font-medium mb-2 block">{t.admin.filter}</label>
                  <Select
                    value={showLowStock ? 'low' : 'all'}
                    onValueChange={(v) => setShowLowStock(v === 'low')}
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="all">{t.admin.allInventory}</SelectItem>
                      <SelectItem value="low">{t.admin.lowStockOnly}</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className="flex items-end">
                  <Button onClick={() => refetch()} variant="outline" className="w-full">
                    <RefreshCw className="mr-2 h-4 w-4" />
                    {t.admin.refresh}
                  </Button>
                </div>
              </div>
            </CardContent>
          </Card>

          {/* 库存列表 */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <Package className="h-5 w-5" />
                {t.admin.physicalInventoryList}
                {showLowStock && (
                  <Badge variant="destructive" className="ml-2">
                    <AlertTriangle className="h-3 w-3 mr-1" />
                    {t.admin.lowStock}
                  </Badge>
                )}
              </CardTitle>
              <CardDescription>
                {t.admin.physicalInventoryDesc}
              </CardDescription>
            </CardHeader>
            <CardContent>
              {isLoading ? (
                <div className="text-center py-8">{t.common.loading}</div>
              ) : inventories.length === 0 ? (
                <div className="text-center py-8 text-muted-foreground">
                  {t.admin.noInventoryConfig}
                </div>
              ) : (
                <>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>ID</TableHead>
                        <TableHead>{t.admin.inventoryName}</TableHead>
                        <TableHead>SKU</TableHead>
                        <TableHead>{t.admin.attributeCombo}</TableHead>
                        <TableHead>{t.admin.stock}</TableHead>
                        <TableHead>{t.admin.purchasable}</TableHead>
                        <TableHead>{t.admin.sold}</TableHead>
                        <TableHead>{t.admin.reserved}</TableHead>
                        <TableHead>{t.admin.remaining}</TableHead>
                        <TableHead>{t.admin.safetyStock}</TableHead>
                        <TableHead>{t.admin.status}</TableHead>
                        <TableHead>{t.admin.actions}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {inventories.map((inventory: any) => (
                        <TableRow key={inventory.id}>
                          <TableCell className="font-mono">{inventory.id}</TableCell>
                          <TableCell>
                            <div className="font-medium">
                              {inventory.name || 'N/A'}
                            </div>
                          </TableCell>
                          <TableCell>
                            <div className="text-sm text-muted-foreground">
                              {inventory.sku || '-'}
                            </div>
                          </TableCell>
                          <TableCell>
                            <div className="flex flex-wrap gap-1">
                              {Object.entries(inventory.attributes || {}).map(([key, value]) => (
                                <Badge key={key} variant="outline" className="text-xs">
                                  {key}: {value as string}
                                </Badge>
                              ))}
                            </div>
                          </TableCell>
                          <TableCell className="font-semibold">{inventory.stock}</TableCell>
                          <TableCell>{inventory.available_quantity}</TableCell>
                          <TableCell>{inventory.sold_quantity}</TableCell>
                          <TableCell>
                            {inventory.reserved_quantity > 0 ? (
                              <Badge variant="secondary">{inventory.reserved_quantity}</Badge>
                            ) : (
                              inventory.reserved_quantity
                            )}
                          </TableCell>
                          <TableCell>
                            <span className={isLowStock(inventory) ? 'text-red-600 font-semibold' : ''}>
                              {getRemainingStock(inventory)}
                            </span>
                          </TableCell>
                          <TableCell>{inventory.safety_stock}</TableCell>
                          <TableCell>
                            <div className="flex items-center gap-1.5 flex-nowrap">
                              {inventory.is_active ? (
                                <Badge variant="default">{t.admin.enabled}</Badge>
                              ) : (
                                <Badge variant="secondary">{t.admin.disabled}</Badge>
                              )}
                              {isLowStock(inventory) && (
                                <Badge variant="destructive" className="h-5 w-5 p-0 justify-center">
                                  <AlertTriangle className="h-3.5 w-3.5" />
                                </Badge>
                              )}
                            </div>
                          </TableCell>
                          <TableCell>
                            <div className="flex items-center gap-2">
                              <Button
                                asChild
                                size="sm"
                                variant="outline"
                              >
                                <Link href={`/admin/inventories/${inventory.id}`}>
                                  <Edit className="h-3 w-3" />
                                </Link>
                              </Button>

                              <AlertDialog>
                                <AlertDialogTrigger asChild>
                                  <Button size="sm" variant="destructive">
                                    <Trash2 className="h-3 w-3" />
                                  </Button>
                                </AlertDialogTrigger>
                                <AlertDialogContent>
                                  <AlertDialogHeader>
                                    <AlertDialogTitle>{t.admin.confirmDelete}</AlertDialogTitle>
                                    <AlertDialogDescription>
                                      {t.admin.deleteInventoryConfirm}
                                    </AlertDialogDescription>
                                  </AlertDialogHeader>
                                  <AlertDialogFooter>
                                    <AlertDialogCancel>{t.common.cancel}</AlertDialogCancel>
                                    <AlertDialogAction
                                      onClick={() => deleteMutation.mutate(inventory.id)}
                                    >
                                      {t.common.delete}
                                    </AlertDialogAction>
                                  </AlertDialogFooter>
                                </AlertDialogContent>
                              </AlertDialog>
                            </div>
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>

                  {/* 分页 */}
                  {pagination && (
                    <div className="flex items-center justify-between mt-4">
                      <div className="text-sm text-muted-foreground">
                        {t.admin.totalRecords.replace('{count}', String(pagination.total)) + ',' + t.admin.totalPages.replace('{count}', String(pagination.total_pages || Math.ceil(pagination.total / limit)))}
                      </div>
                      <div className="flex items-center gap-2">
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => setPage(p => Math.max(1, p - 1))}
                          disabled={!pagination.has_prev}
                        >
                          {t.admin.prevPage}
                        </Button>
                        <input
                          type="number"
                          min={1}
                          max={pagination.total_pages || Math.ceil(pagination.total / limit)}
                          defaultValue={page}
                          key={`phys-${page}`}
                          onBlur={(e) => {
                            const p = parseInt(e.target.value)
                            const total = pagination.total_pages || Math.ceil(pagination.total / limit)
                            if (p >= 1 && p <= total && p !== page) {
                              setPage(p)
                            }
                          }}
                          onKeyDown={(e) => {
                            if (e.key === 'Enter') {
                              const p = parseInt((e.target as HTMLInputElement).value)
                              const total = pagination.total_pages || Math.ceil(pagination.total / limit)
                              if (p >= 1 && p <= total && p !== page) {
                                setPage(p)
                              }
                              ;(e.target as HTMLInputElement).blur()
                            }
                          }}
                          className="w-12 h-8 text-center text-sm border rounded-md bg-background focus:outline-none focus:ring-2 focus:ring-ring [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
                        />
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => setPage(p => p + 1)}
                          disabled={!pagination.has_next}
                        >
                          {t.admin.nextPage}
                        </Button>
                      </div>
                    </div>
                  )}
                </>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        {/* 虚拟库存标签内容 */}
        <TabsContent value="virtual" className="space-y-4">
          {/* 搜索框 */}
          <Card>
            <CardContent className="pt-6">
              <div className="flex gap-4">
                <div className="flex-1">
                  <Input
                    placeholder={t.admin.searchVirtualPlaceholder}
                    value={virtualSearch}
                    onChange={(e) => setVirtualSearch(e.target.value)}
                  />
                </div>
                <Button onClick={() => refetchVirtual()} variant="outline">
                  <RefreshCw className="mr-2 h-4 w-4" />
                  {t.admin.refresh}
                </Button>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <Database className="h-5 w-5" />
                {t.admin.virtualInventoryList}
              </CardTitle>
              <CardDescription>
                {t.admin.virtualInventoryDesc}
              </CardDescription>
            </CardHeader>
            <CardContent>
              {virtualLoading ? (
                <div className="text-center py-8">{t.common.loading}</div>
              ) : virtualInventories.length === 0 ? (
                <div className="text-center py-8 text-muted-foreground">
                  <FileText className="h-12 w-12 mx-auto mb-4 opacity-50" />
                  <p>{t.admin.noVirtualInventory}</p>
                  <p className="text-sm mt-2">{t.admin.noVirtualInventoryHint}</p>
                  <Button
                    variant="outline"
                    className="mt-4"
                    onClick={() => setCreateDialogOpen(true)}
                  >
                    <Plus className="mr-2 h-4 w-4" />
                    {t.admin.createVirtual}
                  </Button>
                </div>
              ) : (
                <>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>ID</TableHead>
                        <TableHead>{t.admin.inventoryName}</TableHead>
                        <TableHead>SKU</TableHead>
                        <TableHead>{t.admin.totalStock}</TableHead>
                        <TableHead>{t.admin.available}</TableHead>
                        <TableHead>{t.admin.reservedStock}</TableHead>
                        <TableHead>{t.admin.soldOut}</TableHead>
                        <TableHead>{t.admin.status}</TableHead>
                        <TableHead>{t.admin.actions}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {virtualInventories.map((vi: any) => (
                        <TableRow key={vi.id}>
                          <TableCell className="font-mono">{vi.id}</TableCell>
                          <TableCell>
                            <div className="font-medium">{vi.name}</div>
                            {vi.description && (
                              <div className="text-sm text-muted-foreground">{vi.description}</div>
                            )}
                          </TableCell>
                          <TableCell className="text-sm text-muted-foreground">
                            {vi.sku || '-'}
                          </TableCell>
                          <TableCell className="font-semibold">{vi.total}</TableCell>
                          <TableCell>
                            <Badge variant={vi.available > 0 ? 'default' : 'destructive'}>
                              {vi.available}
                            </Badge>
                          </TableCell>
                          <TableCell>
                            {vi.reserved > 0 ? (
                              <Badge variant="secondary">{vi.reserved}</Badge>
                            ) : (
                              vi.reserved
                            )}
                          </TableCell>
                          <TableCell>{vi.sold}</TableCell>
                          <TableCell>
                            {vi.is_active ? (
                              <Badge variant="default">{t.admin.enabled}</Badge>
                            ) : (
                              <Badge variant="secondary">{t.admin.disabled}</Badge>
                            )}
                          </TableCell>
                          <TableCell>
                            <div className="flex items-center gap-2">
                              <Button
                                size="sm"
                                variant="outline"
                                onClick={() => openManualCreateDialog(vi.id)}
                                title={t.admin.addCardKey}
                              >
                                <Plus className="h-3 w-3" />
                              </Button>
                              <Button
                                size="sm"
                                variant="outline"
                                onClick={() => openImportDialog(vi.id)}
                                title={t.admin.batchImport}
                              >
                                <Upload className="h-3 w-3" />
                              </Button>
                              <Button
                                asChild
                                size="sm"
                                variant="outline"
                                title={t.admin.manageStockItems}
                              >
                                <Link href={`/admin/inventories/${vi.id}/virtual`}>
                                  <Edit className="h-3 w-3" />
                                </Link>
                              </Button>

                              <AlertDialog>
                                <AlertDialogTrigger asChild>
                                  <Button size="sm" variant="destructive">
                                    <Trash2 className="h-3 w-3" />
                                  </Button>
                                </AlertDialogTrigger>
                                <AlertDialogContent>
                                  <AlertDialogHeader>
                                    <AlertDialogTitle>{t.admin.confirmDelete}</AlertDialogTitle>
                                    <AlertDialogDescription>
                                      {t.admin.deleteVirtualConfirm}
                                    </AlertDialogDescription>
                                  </AlertDialogHeader>
                                  <AlertDialogFooter>
                                    <AlertDialogCancel>{t.common.cancel}</AlertDialogCancel>
                                    <AlertDialogAction
                                      onClick={() => deleteVirtualMutation.mutate(vi.id)}
                                    >
                                      {t.common.delete}
                                    </AlertDialogAction>
                                  </AlertDialogFooter>
                                </AlertDialogContent>
                              </AlertDialog>
                            </div>
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>

                  {/* 分页 */}
                  {virtualPagination && virtualPagination.total > limit && (
                    <div className="flex items-center justify-between mt-4">
                      <div className="text-sm text-muted-foreground">
                        {t.admin.totalRecords.replace('{count}', String(virtualPagination.total))}
                      </div>
                      <div className="flex items-center gap-2">
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => setVirtualPage(p => Math.max(1, p - 1))}
                          disabled={virtualPage <= 1}
                        >
                          {t.admin.prevPage}
                        </Button>
                        <input
                          type="number"
                          min={1}
                          defaultValue={virtualPage}
                          key={`virt-${virtualPage}`}
                          onBlur={(e) => {
                            const p = parseInt(e.target.value)
                            if (p >= 1 && p !== virtualPage) {
                              setVirtualPage(p)
                            }
                          }}
                          onKeyDown={(e) => {
                            if (e.key === 'Enter') {
                              const p = parseInt((e.target as HTMLInputElement).value)
                              if (p >= 1 && p !== virtualPage) {
                                setVirtualPage(p)
                              }
                              ;(e.target as HTMLInputElement).blur()
                            }
                          }}
                          className="w-12 h-8 text-center text-sm border rounded-md bg-background focus:outline-none focus:ring-2 focus:ring-ring [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
                        />
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => setVirtualPage(p => p + 1)}
                          disabled={virtualInventories.length < limit}
                        >
                          {t.admin.nextPage}
                        </Button>
                      </div>
                    </div>
                  )}
                </>
              )}
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>

      {/* 创建虚拟库存对话框 */}
      <Dialog open={createDialogOpen} onOpenChange={setCreateDialogOpen}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>{t.admin.createVirtualTitle}</DialogTitle>
            <DialogDescription>
              {t.admin.createVirtualDesc}
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="name">{t.admin.inventoryNameRequired}</Label>
              <Input
                id="name"
                placeholder={t.admin.inventoryNamePlaceholder}
                value={newVirtualInventory.name}
                onChange={(e) => setNewVirtualInventory({ ...newVirtualInventory, name: e.target.value })}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="sku">{t.admin.skuOptional}</Label>
              <Input
                id="sku"
                placeholder={t.admin.skuPlaceholder}
                value={newVirtualInventory.sku}
                onChange={(e) => setNewVirtualInventory({ ...newVirtualInventory, sku: e.target.value })}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="description">{t.admin.descriptionOptional}</Label>
              <Textarea
                id="description"
                placeholder={t.admin.descriptionPlaceholder}
                value={newVirtualInventory.description}
                onChange={(e) => setNewVirtualInventory({ ...newVirtualInventory, description: e.target.value })}
                rows={3}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="notes">{t.admin.notesOptional}</Label>
              <Textarea
                id="notes"
                placeholder={t.admin.notesPlaceholder}
                value={newVirtualInventory.notes}
                onChange={(e) => setNewVirtualInventory({ ...newVirtualInventory, notes: e.target.value })}
                rows={2}
              />
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateDialogOpen(false)}>
              {t.common.cancel}
            </Button>
            <Button
              onClick={handleCreateVirtualInventory}
              disabled={createVirtualMutation.isPending}
            >
              {createVirtualMutation.isPending ? t.admin.creating : t.common.confirm}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* 导入库存对话框 */}
      <Dialog open={importDialogOpen} onOpenChange={setImportDialogOpen}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>{t.admin.importVirtualTitle}</DialogTitle>
            <DialogDescription>
              {t.admin.importVirtualDesc}
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4">
            {/* 导入方式切换 */}
            <Tabs value={importType} onValueChange={(v) => setImportType(v as 'file' | 'text')}>
              <TabsList className="grid w-full grid-cols-2">
                <TabsTrigger value="text">
                  <FileText className="w-4 h-4 mr-2" />
                  {t.admin.textInput}
                </TabsTrigger>
                <TabsTrigger value="file">
                  <Upload className="w-4 h-4 mr-2" />
                  {t.admin.fileUpload}
                </TabsTrigger>
              </TabsList>

              <TabsContent value="text" className="space-y-4">
                <div>
                  <Textarea
                    placeholder={t.admin.textInputPlaceholder}
                    value={textContent}
                    onChange={(e) => setTextContent(e.target.value)}
                    rows={10}
                  />
                  <p className="text-sm text-muted-foreground mt-2">
                    {t.admin.textInputExample}<br />
                    ABCD-1234-EFGH<br />
                    WXYZ-5678-IJKL,VIP
                  </p>
                </div>
              </TabsContent>

              <TabsContent value="file" className="space-y-4">
                <div
                  className="border-2 border-dashed rounded-lg p-8 text-center cursor-pointer hover:border-primary"
                  onClick={() => fileInputRef.current?.click()}
                >
                  <input
                    ref={fileInputRef}
                    type="file"
                    accept=".xlsx,.xls,.csv,.txt"
                    onChange={handleFileChange}
                    className="hidden"
                  />
                  {selectedFile ? (
                    <div className="flex items-center justify-center gap-2">
                      <FileText className="w-6 h-6 text-primary" />
                      <span>{selectedFile.name}</span>
                    </div>
                  ) : (
                    <>
                      <Upload className="w-8 h-8 mx-auto mb-2 text-muted-foreground" />
                      <p className="text-muted-foreground">{t.admin.clickToSelectFile}</p>
                      <p className="text-sm text-muted-foreground">{t.admin.supportedFormats}</p>
                    </>
                  )}
                </div>
                <p className="text-sm text-muted-foreground">
                  {t.admin.fileFormatHint}
                </p>
              </TabsContent>
            </Tabs>
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setImportDialogOpen(false)}>
              {t.common.cancel}
            </Button>
            <Button
              onClick={handleImport}
              disabled={importMutation.isPending}
            >
              {importMutation.isPending ? (
                <>
                  <RefreshCw className="w-4 h-4 mr-2 animate-spin" />
                  {t.admin.importing}
                </>
              ) : (
                t.admin.confirmImport
              )}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* 手动创建库存项对话框 */}
      <Dialog open={manualCreateDialogOpen} onOpenChange={setManualCreateDialogOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{t.admin.addCardKeyTitle}</DialogTitle>
            <DialogDescription>
              {t.admin.addCardKeyDesc}
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="content">{t.admin.cardKeyContentRequired}</Label>
              <Input
                id="content"
                placeholder={t.admin.cardKeyContentPlaceholder}
                value={manualStockContent}
                onChange={(e) => setManualStockContent(e.target.value)}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="remark">{t.admin.remarkOptional}</Label>
              <Input
                id="remark"
                placeholder={t.admin.remarkPlaceholder}
                value={manualStockRemark}
                onChange={(e) => setManualStockRemark(e.target.value)}
              />
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setManualCreateDialogOpen(false)}>
              {t.common.cancel}
            </Button>
            <Button
              onClick={handleManualCreate}
              disabled={manualCreateMutation.isPending}
            >
              {manualCreateMutation.isPending ? t.admin.adding : t.admin.add}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
