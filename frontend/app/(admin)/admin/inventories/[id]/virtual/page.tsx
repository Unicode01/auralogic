'use client'

import { useState, useRef } from 'react'
import { useParams, useRouter } from 'next/navigation'
import { useQuery, useMutation } from '@tanstack/react-query'
import {
  getVirtualInventory,
  updateVirtualInventory,
  getVirtualInventoryStocks,
  importVirtualInventoryStock,
  deleteVirtualInventoryStock,
  createVirtualInventoryStockManually,
  reserveVirtualInventoryStock,
  releaseVirtualInventoryStock
} from '@/lib/api'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
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
import { ArrowLeft, Save, Plus, Trash2, RefreshCw, Database, FileText, Upload, Loader2, Lock, Unlock } from 'lucide-react'
import Link from 'next/link'
import { useToast } from '@/hooks/use-toast'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'

export default function VirtualInventoryEditPage() {
  const params = useParams()
  const router = useRouter()
  const toast = useToast()
  const fileInputRef = useRef<HTMLInputElement>(null)
  const inventoryId = Number(params.id)
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.adminVirtualInventory)

  const [page, setPage] = useState(1)
  const [limit] = useState(20)
  const [statusFilter, setStatusFilter] = useState<string>('all')

  const [editForm, setEditForm] = useState({
    name: '',
    sku: '',
    description: '',
    is_active: true,
    notes: ''
  })
  const [isFormLoaded, setIsFormLoaded] = useState(false)

  const [importDialogOpen, setImportDialogOpen] = useState(false)
  const [importType, setImportType] = useState<'file' | 'text'>('text')
  const [textContent, setTextContent] = useState('')
  const [selectedFile, setSelectedFile] = useState<File | null>(null)

  const [manualDialogOpen, setManualDialogOpen] = useState(false)
  const [manualContent, setManualContent] = useState('')
  const [manualRemark, setManualRemark] = useState('')

  const { data: inventoryData, isLoading: inventoryLoading, refetch: refetchInventory } = useQuery({
    queryKey: ['virtualInventory', inventoryId],
    queryFn: () => getVirtualInventory(inventoryId),
    enabled: !!inventoryId,
  })

  if (inventoryData?.data && !isFormLoaded) {
    const inv = inventoryData.data
    setEditForm({
      name: inv.name || '',
      sku: inv.sku || '',
      description: inv.description || '',
      is_active: inv.is_active ?? true,
      notes: inv.notes || ''
    })
    setIsFormLoaded(true)
  }

  const { data: stocksData, isLoading: stocksLoading, refetch: refetchStocks } = useQuery({
    queryKey: ['virtualInventoryStocks', inventoryId, page, limit, statusFilter],
    queryFn: () => getVirtualInventoryStocks(inventoryId, {
      page,
      limit,
      status: statusFilter === 'all' ? undefined : statusFilter
    }),
    enabled: !!inventoryId,
  })

  const updateMutation = useMutation({
    mutationFn: (data: typeof editForm) => updateVirtualInventory(inventoryId, data),
    onSuccess: () => {
      toast.success(t.admin.saveSuccess)
      refetchInventory()
    },
    onError: (error: Error) => {
      toast.error(`${t.admin.saveFailed}: ${error.message}`)
    },
  })

  const importMutation = useMutation({
    mutationFn: (data: { import_type: 'file' | 'text'; file?: File; content?: string }) =>
      importVirtualInventoryStock(inventoryId, data),
    onSuccess: (response: any) => {
      toast.success(t.admin.importSuccessCount.replace('{count}', String(response?.data?.count || 0)))
      setImportDialogOpen(false)
      setTextContent('')
      setSelectedFile(null)
      refetchStocks()
      refetchInventory()
    },
    onError: (error: Error) => {
      toast.error(`${t.admin.importFailedMsg}: ${error.message}`)
    },
  })

  const manualCreateMutation = useMutation({
    mutationFn: (data: { content: string; remark?: string }) =>
      createVirtualInventoryStockManually(inventoryId, data),
    onSuccess: () => {
      toast.success(t.admin.addSuccess)
      setManualDialogOpen(false)
      setManualContent('')
      setManualRemark('')
      refetchStocks()
      refetchInventory()
    },
    onError: (error: Error) => {
      toast.error(`${t.admin.addFailed}: ${error.message}`)
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (stockId: number) => deleteVirtualInventoryStock(inventoryId, stockId),
    onSuccess: () => {
      toast.success(t.admin.deleteSuccessMsg)
      refetchStocks()
      refetchInventory()
    },
    onError: (error: Error) => {
      toast.error(`${t.admin.deleteFailedMsg}: ${error.message}`)
    },
  })

  const reserveMutation = useMutation({
    mutationFn: (stockId: number) => reserveVirtualInventoryStock(inventoryId, stockId),
    onSuccess: () => {
      toast.success(t.admin.reserveSuccess)
      refetchStocks()
      refetchInventory()
    },
    onError: (error: Error) => {
      toast.error(`${t.admin.reserveFailed}: ${error.message}`)
    },
  })

  const releaseMutation = useMutation({
    mutationFn: (stockId: number) => releaseVirtualInventoryStock(inventoryId, stockId),
    onSuccess: () => {
      toast.success(t.admin.releaseSuccess)
      refetchStocks()
      refetchInventory()
    },
    onError: (error: Error) => {
      toast.error(`${t.admin.releaseFailed}: ${error.message}`)
    },
  })

  const inventory = inventoryData?.data
  const stocks = stocksData?.data?.list || []
  const total = stocksData?.data?.total || 0
  const totalPages = Math.ceil(total / limit)

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (file) {
      const validTypes = ['.xlsx', '.xls', '.csv', '.txt']
      const ext = file.name.substring(file.name.lastIndexOf('.')).toLowerCase()
      if (!validTypes.includes(ext)) {
        toast.error(t.admin.onlySupportedFormatsError)
        return
      }
      setSelectedFile(file)
    }
  }

  const handleImport = () => {
    if (importType === 'text') {
      if (!textContent.trim()) {
        toast.error(t.admin.pleaseInputContent)
        return
      }
      importMutation.mutate({ import_type: 'text', content: textContent })
    } else {
      if (!selectedFile) {
        toast.error(t.admin.pleaseSelectFile)
        return
      }
      importMutation.mutate({ import_type: 'file', file: selectedFile })
    }
  }

  const handleManualCreate = () => {
    if (!manualContent.trim()) {
      toast.error(t.admin.pleaseInputCardKey)
      return
    }
    manualCreateMutation.mutate({ content: manualContent, remark: manualRemark })
  }

  const handleSave = () => {
    if (!editForm.name.trim()) {
      toast.error(t.admin.pleaseInputInventoryName)
      return
    }
    updateMutation.mutate(editForm)
  }

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'available':
        return <Badge variant="default">{t.admin.statusAvailable}</Badge>
      case 'reserved':
        return <Badge variant="secondary">{t.admin.statusReserved}</Badge>
      case 'sold':
        return <Badge variant="outline">{t.admin.statusSold}</Badge>
      case 'invalid':
        return <Badge variant="destructive">{t.admin.statusInvalid}</Badge>
      default:
        return <Badge variant="outline">{status}</Badge>
    }
  }

  if (inventoryLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <div className="text-center">
          <Loader2 className="h-8 w-8 animate-spin mx-auto mb-4" />
          <p className="text-muted-foreground">{t.common.loading}</p>
        </div>
      </div>
    )
  }

  if (!inventory) {
    return (
      <div className="text-center py-12">
        <p className="text-muted-foreground">{t.admin.virtualNotExist}</p>
        <Button asChild className="mt-4">
          <Link href="/admin/inventories?tab=virtual">{t.admin.backToList}</Link>
        </Button>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <Button variant="outline" size="sm" asChild>
            <Link href="/admin/inventories?tab=virtual">
              <ArrowLeft className="mr-1.5 h-4 w-4" />
              <span className="hidden md:inline">{t.common.back}</span>
            </Link>
          </Button>
          <div>
            <h1 className="text-lg md:text-xl font-bold">{inventory.name}</h1>
            <p className="text-muted-foreground">{t.admin.virtualInventoryEditSubtitle}</p>
          </div>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => setManualDialogOpen(true)}>
            <Plus className="mr-2 h-4 w-4" />
            {t.admin.addCardKeyBtn}
          </Button>
          <Button variant="outline" onClick={() => setImportDialogOpen(true)}>
            <Upload className="mr-2 h-4 w-4" />
            {t.admin.batchImportBtn}
          </Button>
        </div>
      </div>

      <div className="grid grid-cols-4 gap-4">
        <Card>
          <CardContent className="pt-6">
            <div className="text-2xl font-bold">{inventory.total || 0}</div>
            <p className="text-sm text-muted-foreground">{t.admin.totalInventory}</p>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="pt-6">
            <div className="text-2xl font-bold text-green-600 dark:text-green-400">{inventory.available || 0}</div>
            <p className="text-sm text-muted-foreground">{t.admin.statusAvailable}</p>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="pt-6">
            <div className="text-2xl font-bold text-yellow-600 dark:text-yellow-400">{inventory.reserved || 0}</div>
            <p className="text-sm text-muted-foreground">{t.admin.statusReserved}</p>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="pt-6">
            <div className="text-2xl font-bold text-gray-500 dark:text-gray-400">{inventory.sold || 0}</div>
            <p className="text-sm text-muted-foreground">{t.admin.statusSold}</p>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Database className="h-5 w-5" />
            {t.admin.basicInfo}
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label htmlFor="name">{t.admin.inventoryNameRequired}</Label>
              <Input
                id="name"
                value={editForm.name}
                onChange={(e) => setEditForm({ ...editForm, name: e.target.value })}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="sku">SKU</Label>
              <Input
                id="sku"
                value={editForm.sku}
                onChange={(e) => setEditForm({ ...editForm, sku: e.target.value })}
              />
            </div>
          </div>
          <div className="space-y-2">
            <Label htmlFor="description">{t.admin.descriptionLabel}</Label>
            <Textarea
              id="description"
              value={editForm.description}
              onChange={(e) => setEditForm({ ...editForm, description: e.target.value })}
              rows={2}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="notes">{t.admin.notesLabel}</Label>
            <Textarea
              id="notes"
              value={editForm.notes}
              onChange={(e) => setEditForm({ ...editForm, notes: e.target.value })}
              rows={2}
            />
          </div>
          <div className="flex items-center space-x-2">
            <Switch
              id="is_active"
              checked={editForm.is_active}
              onCheckedChange={(checked) => setEditForm({ ...editForm, is_active: checked })}
            />
            <Label htmlFor="is_active">{t.admin.activeStatusLabel}</Label>
          </div>
          <div className="flex justify-end">
            <Button onClick={handleSave} disabled={updateMutation.isPending}>
              <Save className="mr-2 h-4 w-4" />
              {updateMutation.isPending ? t.admin.savingText : t.common.save}
            </Button>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle className="flex items-center gap-2">
                <FileText className="h-5 w-5" />
                {t.admin.stockItemList}
              </CardTitle>
              <CardDescription>
                {t.admin.totalRecordsCount.replace('{count}', String(total))}
              </CardDescription>
            </div>
            <div className="flex items-center gap-2">
              <Select value={statusFilter} onValueChange={setStatusFilter}>
                <SelectTrigger className="w-32">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">{t.admin.allStatusFilter}</SelectItem>
                  <SelectItem value="available">{t.admin.statusAvailable}</SelectItem>
                  <SelectItem value="reserved">{t.admin.statusReserved}</SelectItem>
                  <SelectItem value="sold">{t.admin.statusSold}</SelectItem>
                  <SelectItem value="invalid">{t.admin.statusInvalid}</SelectItem>
                </SelectContent>
              </Select>
              <Button variant="outline" onClick={() => refetchStocks()}>
                <RefreshCw className="h-4 w-4" />
              </Button>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          {stocksLoading ? (
            <div className="text-center py-8">{t.common.loading}</div>
          ) : stocks.length === 0 ? (
            <div className="text-center py-8 text-muted-foreground">
              <FileText className="h-12 w-12 mx-auto mb-4 opacity-50" />
              <p>{t.admin.noStockItems}</p>
              <p className="text-sm mt-2">{t.admin.noStockItemsHint}</p>
            </div>
          ) : (
            <>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>ID</TableHead>
                    <TableHead>{t.admin.contentColumn}</TableHead>
                    <TableHead>{t.admin.remarkColumn}</TableHead>
                    <TableHead>{t.admin.statusColumn}</TableHead>
                    <TableHead>{t.admin.orderNoColumn}</TableHead>
                    <TableHead>{t.admin.batchNoColumn}</TableHead>
                    <TableHead>{t.admin.createdAtColumn}</TableHead>
                    <TableHead>{t.admin.operationsColumn}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {stocks.map((stock: any) => (
                    <TableRow key={stock.id}>
                      <TableCell className="font-mono">{stock.id}</TableCell>
                      <TableCell className="font-mono max-w-xs truncate" title={stock.content}>
                        {stock.content}
                      </TableCell>
                      <TableCell className="text-sm text-muted-foreground max-w-xs truncate">
                        {stock.remark || '-'}
                      </TableCell>
                      <TableCell>{getStatusBadge(stock.status)}</TableCell>
                      <TableCell className="text-sm">
                        {stock.order_no || '-'}
                      </TableCell>
                      <TableCell className="text-sm text-muted-foreground">
                        {stock.batch_no || '-'}
                      </TableCell>
                      <TableCell className="text-sm text-muted-foreground">
                        {new Date(stock.created_at).toLocaleString()}
                      </TableCell>
                      <TableCell>
                        <div className="flex gap-1">
                          {stock.status === 'available' && (
                            <>
                              <Button
                                size="sm"
                                variant="outline"
                                onClick={() => reserveMutation.mutate(stock.id)}
                                disabled={reserveMutation.isPending}
                                title={t.admin.reserve}
                              >
                                <Lock className="h-3 w-3" />
                              </Button>
                              <AlertDialog>
                                <AlertDialogTrigger asChild>
                                  <Button size="sm" variant="destructive" title={t.common.delete}>
                                    <Trash2 className="h-3 w-3" />
                                  </Button>
                                </AlertDialogTrigger>
                                <AlertDialogContent>
                                  <AlertDialogHeader>
                                    <AlertDialogTitle>{t.admin.confirmDeleteTitle}</AlertDialogTitle>
                                    <AlertDialogDescription>
                                      {t.admin.confirmDeleteStockItem}
                                    </AlertDialogDescription>
                                  </AlertDialogHeader>
                                  <AlertDialogFooter>
                                    <AlertDialogCancel>{t.common.cancel}</AlertDialogCancel>
                                    <AlertDialogAction onClick={() => deleteMutation.mutate(stock.id)}>
                                      {t.common.delete}
                                    </AlertDialogAction>
                                  </AlertDialogFooter>
                                </AlertDialogContent>
                              </AlertDialog>
                            </>
                          )}
                          {stock.status === 'reserved' && (
                            <>
                              <Button
                                size="sm"
                                variant="outline"
                                onClick={() => releaseMutation.mutate(stock.id)}
                                disabled={releaseMutation.isPending}
                                title={t.admin.release}
                              >
                                <Unlock className="h-3 w-3" />
                              </Button>
                              <AlertDialog>
                                <AlertDialogTrigger asChild>
                                  <Button size="sm" variant="destructive" title={t.common.delete}>
                                    <Trash2 className="h-3 w-3" />
                                  </Button>
                                </AlertDialogTrigger>
                                <AlertDialogContent>
                                  <AlertDialogHeader>
                                    <AlertDialogTitle>{t.admin.confirmDeleteTitle}</AlertDialogTitle>
                                    <AlertDialogDescription>
                                      {t.admin.confirmDeleteStockItem}
                                    </AlertDialogDescription>
                                  </AlertDialogHeader>
                                  <AlertDialogFooter>
                                    <AlertDialogCancel>{t.common.cancel}</AlertDialogCancel>
                                    <AlertDialogAction onClick={() => deleteMutation.mutate(stock.id)}>
                                      {t.common.delete}
                                    </AlertDialogAction>
                                  </AlertDialogFooter>
                                </AlertDialogContent>
                              </AlertDialog>
                            </>
                          )}
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>

              {total > 0 && (
                <div className="flex items-center justify-between mt-4">
                  <div className="text-sm text-muted-foreground">
                    {t.admin.paginationInfo
                      .replace('{total}', String(total))
                      .replace('{page}', String(page))
                      .replace('{totalPages}', String(totalPages))}
                  </div>
                  <div className="flex gap-2">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => setPage(1)}
                      disabled={page <= 1}
                    >
                      {t.admin.firstPage}
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => setPage(p => Math.max(1, p - 1))}
                      disabled={page <= 1}
                    >
                      {t.admin.prevPageBtn}
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => setPage(p => p + 1)}
                      disabled={page >= totalPages}
                    >
                      {t.admin.nextPageBtn}
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => setPage(totalPages)}
                      disabled={page >= totalPages}
                    >
                      {t.admin.lastPage}
                    </Button>
                  </div>
                </div>
              )}
            </>
          )}
        </CardContent>
      </Card>

      <Dialog open={importDialogOpen} onOpenChange={setImportDialogOpen}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>{t.admin.batchImportTitle}</DialogTitle>
            <DialogDescription>
              {t.admin.batchImportDesc}
            </DialogDescription>
          </DialogHeader>

          <Tabs value={importType} onValueChange={(v) => setImportType(v as 'file' | 'text')}>
            <TabsList className="grid w-full grid-cols-2">
              <TabsTrigger value="text">
                <FileText className="w-4 h-4 mr-2" />
                {t.admin.textInputTab}
              </TabsTrigger>
              <TabsTrigger value="file">
                <Upload className="w-4 h-4 mr-2" />
                {t.admin.fileUploadTab}
              </TabsTrigger>
            </TabsList>

            <TabsContent value="text" className="space-y-4">
              <div>
                <Textarea
                  placeholder={t.admin.textInputPlaceholderEdit}
                  value={textContent}
                  onChange={(e) => setTextContent(e.target.value)}
                  rows={10}
                />
                <p className="text-sm text-muted-foreground mt-2">
                  {t.admin.textInputExampleLabel}<br />
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
                    <p className="text-muted-foreground">{t.admin.clickSelectFile}</p>
                    <p className="text-sm text-muted-foreground">{t.admin.supportedFormatsText}</p>
                  </>
                )}
              </div>
            </TabsContent>
          </Tabs>

          <DialogFooter>
            <Button variant="outline" onClick={() => setImportDialogOpen(false)}>
              {t.common.cancel}
            </Button>
            <Button onClick={handleImport} disabled={importMutation.isPending}>
              {importMutation.isPending ? (
                <>
                  <RefreshCw className="w-4 h-4 mr-2 animate-spin" />
                  {t.admin.importingText}
                </>
              ) : (
                t.admin.confirmImportBtn
              )}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={manualDialogOpen} onOpenChange={setManualDialogOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{t.admin.addCardKeyDialogTitle}</DialogTitle>
            <DialogDescription>
              {t.admin.addCardKeyDialogDesc}
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="content">{t.admin.cardKeyContentLabel}</Label>
              <Input
                id="content"
                placeholder={t.admin.cardKeyContentInputPlaceholder}
                value={manualContent}
                onChange={(e) => setManualContent(e.target.value)}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="remark">{t.admin.remarkOptionalLabel}</Label>
              <Input
                id="remark"
                placeholder={t.admin.remarkInputPlaceholder}
                value={manualRemark}
                onChange={(e) => setManualRemark(e.target.value)}
              />
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setManualDialogOpen(false)}>
              {t.common.cancel}
            </Button>
            <Button onClick={handleManualCreate} disabled={manualCreateMutation.isPending}>
              {manualCreateMutation.isPending ? t.admin.addingText : t.admin.addBtn}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
