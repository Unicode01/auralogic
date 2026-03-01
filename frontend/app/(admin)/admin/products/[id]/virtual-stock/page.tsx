'use client'

import { useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
import { useQuery, useMutation } from '@tanstack/react-query'
import {
  getAdminProduct,
  getVirtualStockList,
  getVirtualStockStats,
  deleteVirtualStock,
  deleteStockBatch,
  getProductVirtualInventoryBindings,
} from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
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
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  ArrowLeft,
  Trash2,
  RefreshCw,
  Package,
  CheckCircle,
  Clock,
  XCircle,
  Copy,
  Eye,
  EyeOff,
  Code2,
} from 'lucide-react'
import toast from 'react-hot-toast'
import { VirtualProductStock, VirtualStockStats, VirtualStockStatus } from '@/types/product'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'

const statusConfig: Record<VirtualStockStatus, { color: string; icon: React.ReactNode }> = {
  available: { color: 'bg-green-100 text-green-800 dark:bg-green-900/50 dark:text-green-300', icon: <CheckCircle className="w-3 h-3" /> },
  reserved: { color: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/50 dark:text-yellow-300', icon: <Clock className="w-3 h-3" /> },
  sold: { color: 'bg-blue-100 text-blue-800 dark:bg-blue-900/50 dark:text-blue-300', icon: <Package className="w-3 h-3" /> },
  invalid: { color: 'bg-red-100 text-red-800 dark:bg-red-900/50 dark:text-red-300', icon: <XCircle className="w-3 h-3" /> },
}

const statusLabelKeys: Record<VirtualStockStatus, 'statusAvailable' | 'statusReserved' | 'statusSold' | 'statusInvalid'> = {
  available: 'statusAvailable',
  reserved: 'statusReserved',
  sold: 'statusSold',
  invalid: 'statusInvalid',
}

export default function VirtualStockPage() {
  const params = useParams()
  const router = useRouter()
  const productId = Number(params.id)
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.adminVirtualStock)

  const [page, setPage] = useState(1)
  const [statusFilter, setStatusFilter] = useState<string>('all')
  const [deleteId, setDeleteId] = useState<number | null>(null)
  const [deleteBatchNo, setDeleteBatchNo] = useState<string | null>(null)
  const [showContent, setShowContent] = useState<Record<number, boolean>>({})

  // 获取商品信息
  const { data: productData, isLoading: productLoading } = useQuery({
    queryKey: ['adminProduct', productId],
    queryFn: () => getAdminProduct(productId),
    enabled: !!productId,
  })

  // 获取库存列表
  const { data: stockData, isLoading: stockLoading, refetch: refetchStocks } = useQuery({
    queryKey: ['virtualStocks', productId, page, statusFilter],
    queryFn: () => getVirtualStockList(productId, {
      page,
      limit: 20,
      status: statusFilter === 'all' ? undefined : statusFilter,
    }),
    enabled: !!productId,
  })

  // 获取库存统计
  const { data: statsData, refetch: refetchStats } = useQuery({
    queryKey: ['virtualStockStats', productId],
    queryFn: () => getVirtualStockStats(productId),
    enabled: !!productId,
  })

  // 获取虚拟库存绑定信息（判断是否为脚本类型）
  const { data: bindingsData } = useQuery({
    queryKey: ['productVirtualBindings', productId],
    queryFn: () => getProductVirtualInventoryBindings(productId),
    enabled: !!productId,
  })

  const bindings: any[] = bindingsData?.data || []
  const isAllScript = bindings.length > 0 && bindings.every((b: any) => b.virtual_inventory?.type === 'script')

  // 删除单个库存
  const deleteMutation = useMutation({
    mutationFn: deleteVirtualStock,
    onSuccess: () => {
      toast.success(t.virtualStock.stockDeleted)
      setDeleteId(null)
      refetchStocks()
      refetchStats()
    },
    onError: (error: Error) => {
      toast.error(t.virtualStock.deleteFailed.replace('{msg}', error.message))
    },
  })

  // 删除批次
  const deleteBatchMutation = useMutation({
    mutationFn: deleteStockBatch,
    onSuccess: (response: any) => {
      toast.success(t.virtualStock.batchDeleteSuccess.replace('{n}', String(response?.data?.count || 0)))
      setDeleteBatchNo(null)
      refetchStocks()
      refetchStats()
    },
    onError: (error: Error) => {
      toast.error(t.virtualStock.deleteFailed.replace('{msg}', error.message))
    },
  })

  const copyToClipboard = (content: string) => {
    navigator.clipboard.writeText(content)
    toast.success(t.virtualStock.copiedToClipboard)
  }

  const toggleContentVisibility = (id: number) => {
    setShowContent(prev => ({ ...prev, [id]: !prev[id] }))
  }

  const product = productData?.data
  const stocks: VirtualProductStock[] = stockData?.data?.items || []
  const total = stockData?.data?.pagination?.total || 0
  const stats: VirtualStockStats = statsData?.data || { total: 0, available: 0, reserved: 0, sold: 0 }
  const totalPages = Math.ceil(total / 20)

  // 获取所有批次号
  const batchNos = [...new Set(stocks.filter(s => s.batch_no).map(s => s.batch_no!))]

  if (productLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <RefreshCw className="w-6 h-6 animate-spin" />
      </div>
    )
  }

  if (!product) {
    return (
      <div className="text-center py-12">
        <p className="text-muted-foreground">{t.virtualStock.productNotFound}</p>
        <Button variant="link" onClick={() => router.push('/admin/products')}>
          {t.virtualStock.backToProducts}
        </Button>
      </div>
    )
  }

  if (product.product_type !== 'virtual') {
    return (
      <div className="text-center py-12">
        <p className="text-muted-foreground">{t.virtualStock.notVirtualProduct}</p>
        <Button variant="link" onClick={() => router.push('/admin/products')}>
          {t.virtualStock.backToProducts}
        </Button>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* 头部 */}
      <div className="flex items-center gap-4">
        <Button variant="outline" size="sm" onClick={() => router.push('/admin/products')}>
          <ArrowLeft className="h-4 w-4 mr-1.5" />
          <span className="hidden md:inline">{t.virtualStock.back}</span>
        </Button>
        <div>
          <h1 className="text-lg md:text-xl font-bold">{t.virtualStock.title}</h1>
          <p className="text-muted-foreground">{product.name} ({product.sku})</p>
        </div>
      </div>

      {/* 统计卡片 */}
      {isAllScript ? (
        <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
          {bindings.map((b: any) => {
            const vi = b.virtual_inventory
            if (!vi) return null
            const limit = vi.total_limit || 0
            const sold = vi.sold || 0
            const remaining = limit > 0 ? Math.max(0, limit - sold) : -1
            return (
              <Card key={b.id} className="col-span-2 md:col-span-3">
                <CardHeader className="pb-2">
                  <CardDescription className="flex items-center gap-1.5">
                    <Code2 className="w-3.5 h-3.5 text-purple-500" />
                    {vi.name}
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  <div className="flex items-center gap-6">
                    <div>
                      <div className="text-2xl font-bold text-blue-600 dark:text-blue-400">
                        {limit > 0 ? limit : t.virtualStock.unlimited}
                      </div>
                      <p className="text-xs text-muted-foreground">{t.virtualStock.deliveryLimit}</p>
                    </div>
                    <div>
                      <div className="text-2xl font-bold text-gray-500 dark:text-gray-400">{sold}</div>
                      <p className="text-xs text-muted-foreground">{t.virtualStock.statusSold}</p>
                    </div>
                    {remaining >= 0 && (
                      <div>
                        <div className={`text-2xl font-bold ${remaining <= 0 ? 'text-red-600 dark:text-red-400' : 'text-green-600 dark:text-green-400'}`}>
                          {remaining}
                        </div>
                        <p className="text-xs text-muted-foreground">{t.virtualStock.remaining}</p>
                      </div>
                    )}
                  </div>
                </CardContent>
              </Card>
            )
          })}
        </div>
      ) : (
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{t.virtualStock.totalStock}</CardDescription>
            <CardTitle className="text-2xl">{stats.total}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{t.virtualStock.statusAvailable}</CardDescription>
            <CardTitle className="text-2xl text-green-600">{stats.available}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{t.virtualStock.statusReserved}</CardDescription>
            <CardTitle className="text-2xl text-yellow-600">{stats.reserved}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{t.virtualStock.statusSold}</CardDescription>
            <CardTitle className="text-2xl text-blue-600">{stats.sold}</CardTitle>
          </CardHeader>
        </Card>
      </div>
      )}

      {/* 操作栏、库存列表、分页、批次管理 — 仅静态类型 */}
      {!isAllScript && (<>
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <Select
            value={statusFilter}
            onValueChange={(value) => {
              setStatusFilter(value)
              setPage(1)
            }}
          >
            <SelectTrigger className="w-[150px]">
              <SelectValue placeholder={t.virtualStock.statusFilter} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t.virtualStock.allStatus}</SelectItem>
              <SelectItem value="available">{t.virtualStock.statusAvailable}</SelectItem>
              <SelectItem value="reserved">{t.virtualStock.statusReserved}</SelectItem>
              <SelectItem value="sold">{t.virtualStock.statusSold}</SelectItem>
              <SelectItem value="invalid">{t.virtualStock.statusInvalid}</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" onClick={() => { refetchStocks(); refetchStats() }}>
            <RefreshCw className="w-4 h-4 mr-2" />
            {t.virtualStock.refresh}
          </Button>
        </div>
      </div>

      {/* 库存列表 */}
      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-[50px]">ID</TableHead>
                <TableHead>{t.virtualStock.cardContent}</TableHead>
                <TableHead>{t.virtualStock.remark}</TableHead>
                <TableHead>{t.virtualStock.status}</TableHead>
                <TableHead>{t.virtualStock.linkedOrder}</TableHead>
                <TableHead>{t.virtualStock.batchNo}</TableHead>
                <TableHead>{t.virtualStock.createdAt}</TableHead>
                <TableHead className="text-right">{t.virtualStock.actions}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {stockLoading ? (
                <TableRow>
                  <TableCell colSpan={8} className="text-center py-8">
                    <RefreshCw className="w-5 h-5 animate-spin inline-block mr-2" />
                    {t.virtualStock.loading}
                  </TableCell>
                </TableRow>
              ) : stocks.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={8} className="text-center py-8 text-muted-foreground">
                    {t.virtualStock.noData}
                  </TableCell>
                </TableRow>
              ) : (
                stocks.map((stock) => (
                  <TableRow key={stock.id}>
                    <TableCell className="font-mono text-sm">{stock.id}</TableCell>
                    <TableCell>
                      <div className="flex items-center gap-2">
                        <code className="bg-muted px-2 py-1 rounded text-sm max-w-[200px] truncate">
                          {showContent[stock.id] ? stock.content : '••••••••••'}
                        </code>
                        <Button
                          variant="ghost"
                          size="sm"
                          className="h-6 w-6 p-0"
                          onClick={() => toggleContentVisibility(stock.id)}
                        >
                          {showContent[stock.id] ? <EyeOff className="w-3 h-3" /> : <Eye className="w-3 h-3" />}
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          className="h-6 w-6 p-0"
                          onClick={() => copyToClipboard(stock.content)}
                        >
                          <Copy className="w-3 h-3" />
                        </Button>
                      </div>
                    </TableCell>
                    <TableCell className="max-w-[150px] truncate">
                      {stock.remark || '-'}
                    </TableCell>
                    <TableCell>
                      <Badge className={statusConfig[stock.status].color}>
                        {statusConfig[stock.status].icon}
                        <span className="ml-1">{t.virtualStock[statusLabelKeys[stock.status]]}</span>
                      </Badge>
                    </TableCell>
                    <TableCell>
                      {stock.order_no ? (
                        <a
                          href={`/admin/orders?search=${stock.order_no}`}
                          className="text-blue-600 hover:underline font-mono text-sm"
                        >
                          {stock.order_no}
                        </a>
                      ) : (
                        '-'
                      )}
                    </TableCell>
                    <TableCell>
                      {stock.batch_no ? (
                        <span className="font-mono text-sm">{stock.batch_no}</span>
                      ) : (
                        '-'
                      )}
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {new Date(stock.created_at).toLocaleString(locale === 'zh' ? 'zh-CN' : 'en-US')}
                    </TableCell>
                    <TableCell className="text-right">
                      {stock.status === 'available' && (
                        <Button
                          variant="ghost"
                          size="sm"
                          className="text-red-600 hover:text-red-700"
                          onClick={() => setDeleteId(stock.id)}
                        >
                          <Trash2 className="w-4 h-4" />
                        </Button>
                      )}
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      {/* 分页 */}
      {totalPages > 1 && (
        <div className="flex items-center justify-between">
          <p className="text-sm text-muted-foreground">
            {t.virtualStock.totalRecords.replace('{total}', String(total)).replace('{page}', String(page)).replace('{totalPages}', String(totalPages))}
          </p>
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              disabled={page <= 1}
              onClick={() => setPage(p => p - 1)}
            >
              {t.virtualStock.prevPage}
            </Button>
            <Button
              variant="outline"
              size="sm"
              disabled={page >= totalPages}
              onClick={() => setPage(p => p + 1)}
            >
              {t.virtualStock.nextPage}
            </Button>
          </div>
        </div>
      )}

      {/* 批次管理 */}
      {batchNos.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle className="text-lg">{t.virtualStock.batchManagement}</CardTitle>
            <CardDescription>{t.virtualStock.batchDescription}</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="flex flex-wrap gap-2">
              {batchNos.map((batchNo) => (
                <Badge
                  key={batchNo}
                  variant="outline"
                  className="cursor-pointer hover:bg-red-50 dark:hover:bg-red-950"
                  onClick={() => setDeleteBatchNo(batchNo)}
                >
                  {batchNo}
                  <Trash2 className="w-3 h-3 ml-1" />
                </Badge>
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      </>)}

      {/* 删除确认对话框 */}
      <AlertDialog open={deleteId !== null} onOpenChange={() => setDeleteId(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t.virtualStock.confirmDelete}</AlertDialogTitle>
            <AlertDialogDescription>
              {t.virtualStock.deleteConfirmMsg}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t.virtualStock.cancel}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => deleteId && deleteMutation.mutate(deleteId)}
              className="bg-red-600 hover:bg-red-700"
            >
              {t.virtualStock.delete}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* 删除批次确认对话框 */}
      <AlertDialog open={deleteBatchNo !== null} onOpenChange={() => setDeleteBatchNo(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t.virtualStock.confirmDeleteBatch}</AlertDialogTitle>
            <AlertDialogDescription>
              {t.virtualStock.deleteBatchConfirmMsg.replace('{batch}', deleteBatchNo || '')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t.virtualStock.cancel}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => deleteBatchNo && deleteBatchMutation.mutate(deleteBatchNo)}
              className="bg-red-600 hover:bg-red-700"
            >
              {t.virtualStock.deleteBatch}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
