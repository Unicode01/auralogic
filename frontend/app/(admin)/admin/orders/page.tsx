'use client'

import { Suspense, useState, useEffect, useRef, useCallback } from 'react'
import { useSearchParams } from 'next/navigation'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { getAdminOrders, batchUpdateOrders } from '@/lib/api'
import { DataTable } from '@/components/admin/data-table'
import { OrderStatusBadge } from '@/components/orders/order-status-badge'
import { OrderFilter } from '@/components/orders/order-filter'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
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
import { Shield, RefreshCw, Download, Upload, FileDown, Package, CheckCircle2, XCircle, Trash2, ChevronDown, X } from 'lucide-react'
import Link from 'next/link'
import { getToken } from '@/lib/auth'
import toast from 'react-hot-toast'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations, translateBizError } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'

function AdminOrdersContent() {
  const searchParams = useSearchParams()
  const queryClient = useQueryClient()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.adminOrders)
  const [page, setPage] = useState(1)
  const [status, setStatus] = useState<string | undefined>()
  const [search, setSearch] = useState('')
  const [productSearch, setProductSearch] = useState('')
  const [promoCode, setPromoCode] = useState('')
  const [promoCodeId, setPromoCodeId] = useState<number | undefined>()
  const [userId, setUserId] = useState<number | undefined>()
  const [country, setCountry] = useState<string | undefined>()
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set())
  const [batchDialogOpen, setBatchDialogOpen] = useState(false)
  const [batchAction, setBatchAction] = useState('')
  const fileInputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    const userIdParam = searchParams.get('user_id')
    if (userIdParam) {
      setUserId(Number(userIdParam))
    }
    const searchParam = searchParams.get('search')
    if (searchParam) {
      setSearch(searchParam)
    }
    const promoCodeParam = searchParams.get('promo_code')
    if (promoCodeParam) {
      setPromoCode(promoCodeParam)
    }
    const promoCodeIdParam = searchParams.get('promo_code_id')
    if (promoCodeIdParam) {
      const n = Number(promoCodeIdParam)
      if (!Number.isNaN(n)) setPromoCodeId(n)
    }
  }, [searchParams])

  const { data, isLoading, refetch } = useQuery({
    queryKey: ['adminOrders', page, status, search, productSearch, promoCodeId, promoCode, userId, country],
    queryFn: () => getAdminOrders({
      page,
      limit: 20,
      status: status === 'all' ? undefined : status,
      search: search || undefined,
      product_search: productSearch || undefined,
      promo_code_id: promoCodeId,
      promo_code: promoCode || undefined,
      user_id: userId,
      country: country === 'all' ? undefined : country,
    }),
  })

  const handleExport = () => {
    const token = getToken()
    const params = new URLSearchParams()
    if (status && status !== 'all') params.append('status', status)
    if (search) params.append('search', search)
    if (productSearch) params.append('product_search', productSearch)
    if (country && country !== 'all') params.append('country', country)
    if (promoCodeId) params.append('promo_code_id', String(promoCodeId))
    if (promoCode) params.append('promo_code', promoCode)

    const url = `${process.env.NEXT_PUBLIC_API_URL || 'https://auralogic.un1c0de.com'}/api/admin/orders/export?${params.toString()}`

    fetch(url, {
      headers: {
        'Authorization': `Bearer ${token}`
      }
    })
      .then(res => {
        if (!res.ok) throw new Error(t.admin.exportFailed)
        return res.blob()
      })
      .then(blob => {
        const url = window.URL.createObjectURL(blob)
        const a = document.createElement('a')
        a.href = url
        a.download = `orders_${new Date().toISOString().slice(0, 10)}.xlsx`
        document.body.appendChild(a)
        a.click()
        document.body.removeChild(a)
        window.URL.revokeObjectURL(url)

        toast.success(t.admin.exportSuccess)
      })
      .catch(err => {
        toast.error(`${t.admin.exportFailed}: ${err.message}`)
      })
  }

  const handleDownloadTemplate = () => {
    const token = getToken()
    const url = `${process.env.NEXT_PUBLIC_API_URL || 'https://auralogic.un1c0de.com'}/api/admin/orders/import-template`

    fetch(url, {
      headers: {
        'Authorization': `Bearer ${token}`
      }
    })
      .then(res => {
        if (!res.ok) throw new Error(t.admin.downloadFailed)
        return res.blob()
      })
      .then(blob => {
        const url = window.URL.createObjectURL(blob)
        const a = document.createElement('a')
        a.href = url
        a.download = 'order_logistics_import_template.xlsx'
        document.body.appendChild(a)
        a.click()
        document.body.removeChild(a)
        window.URL.revokeObjectURL(url)
      })
      .catch(err => {
        toast.error(`${t.admin.downloadFailed}: ${err.message}`)
      })
  }

  const importMutation = useMutation({
    mutationFn: async (file: File) => {
      const token = getToken()
      const formData = new FormData()
      formData.append('file', file)

      const response = await fetch(
        `${process.env.NEXT_PUBLIC_API_URL || 'https://auralogic.un1c0de.com'}/api/admin/orders/import`,
        {
          method: 'POST',
          headers: {
            'Authorization': `Bearer ${token}`
          },
          body: formData
        }
      )

      if (!response.ok) {
        const data = await response.json()
        throw new Error(data.message || t.admin.importFailed)
      }

      return response.json()
    },
    onSuccess: (data) => {
      toast.dismiss()
      const result = data.data
      if (result.error_count > 0) {
        toast.error(`${result.message}`, {
          duration: 6000,
        })
        if (result.errors && result.errors.length > 0) {
          console.error('Import error details:', result.errors)
        }
      } else {
        toast.success(result.message, {
          duration: 3000,
        })
      }
      refetch()
    },
    onError: (error: Error) => {
      toast.dismiss()
      toast.error(`${t.admin.importFailed}: ${error.message}`)
    }
  })

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (file) {
      if (!file.name.endsWith('.xlsx')) {
        toast.error(t.admin.fileFormatError)
        return
      }

      toast.loading(t.admin.importLoading)
      importMutation.mutate(file)

      if (fileInputRef.current) {
        fileInputRef.current.value = ''
      }
    }
  }

  const batchMutation = useMutation({
    mutationFn: ({ orderIds, action }: { orderIds: number[], action: string }) =>
      batchUpdateOrders(orderIds, action),
    onSuccess: (data) => {
      const result = data.data
      toast.success(t.admin.batchSuccess
        .replace('{success}', String(result.success_count))
        .replace('{failed}', String(result.failed_count)))
      setSelectedIds(new Set())
      refetch()
      queryClient.invalidateQueries({ queryKey: ['adminOrders'] })
    },
    onError: (error: any) => {
      if (error.code === 40010 && error.data?.error_key) {
        toast.error(translateBizError(t, error.data.error_key, error.data.params, error.message))
      } else {
        toast.error(error.message || t.order.operationFailed)
      }
    }
  })

  const handleBatchAction = (action: string) => {
    setBatchAction(action)
    setBatchDialogOpen(true)
  }

  const confirmBatchAction = () => {
    const ids = Array.from(selectedIds)
    batchMutation.mutate({ orderIds: ids, action: batchAction })
    setBatchDialogOpen(false)
  }

  const getBatchConfirmMsg = () => {
    const count = selectedIds.size
    switch (batchAction) {
      case 'complete':
        return t.admin.confirmBatchComplete.replace('{count}', String(count))
      case 'cancel':
        return t.admin.confirmBatchCancel.replace('{count}', String(count))
      case 'delete':
        return t.admin.confirmBatchDelete.replace('{count}', String(count))
      default:
        return ''
    }
  }

  const handlePageChange = useCallback((newPage: number) => {
    setPage(newPage)
    setSelectedIds(new Set())
  }, [])

  const currentItems: any[] = data?.data?.items || []
  const allCurrentIds = currentItems.map((item: any) => item.id as number)
  const allSelected = currentItems.length > 0 && allCurrentIds.every(id => selectedIds.has(id))

  const toggleSelectAll = () => {
    if (allSelected) {
      setSelectedIds(new Set())
    } else {
      setSelectedIds(new Set(allCurrentIds))
    }
  }

  const toggleSelectOne = (id: number) => {
    setSelectedIds(prev => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      return next
    })
  }

  const columns = [
    {
      header: () => (
        <Checkbox
          checked={allSelected}
          onCheckedChange={toggleSelectAll}
          aria-label="Select all"
        />
      ),
      accessorKey: 'select',
      cell: ({ row }: { row: { original: any } }) => (
        <Checkbox
          checked={selectedIds.has(row.original.id)}
          onCheckedChange={() => toggleSelectOne(row.original.id)}
          aria-label="Select row"
        />
      ),
    },
    {
      header: t.admin.orderNo,
      accessorKey: 'orderNo',
      cell: ({ row }: { row: { original: any } }) => (
        <span className="font-mono text-sm">
          {row.original.orderNo || row.original.order_no}
        </span>
      ),
    },
    {
      header: t.admin.product,
      cell: ({ row }: { row: { original: any } }) => {
        const items = row.original.items || []
        return (
          <div className="flex flex-col gap-1 max-w-[200px]">
            {items.slice(0, 2).map((item: any, index: number) => (
              <div key={index} className="flex items-center gap-2 text-sm">
                <Package className="h-3 w-3 text-muted-foreground flex-shrink-0" />
                <span className="truncate" title={item.name}>
                  {item.name}
                </span>
                <Badge variant="secondary" className="text-xs flex-shrink-0">
                  x{item.quantity}
                </Badge>
              </div>
            ))}
            {items.length > 2 && (
              <span className="text-xs text-muted-foreground">
                {t.admin.moreProducts.replace('{count}', String(items.length - 2))}
              </span>
            )}
          </div>
        )
      },
    },
    {
      header: t.admin.status,
      cell: ({ row }: { row: { original: any } }) => (
        <OrderStatusBadge status={row.original.status} />
      ),
    },
    {
      header: t.admin.platformUser,
      cell: ({ row }: { row: { original: any } }) => {
        const userName = row.original.externalUserName || row.original.external_user_name
        const userEmail = row.original.userEmail || row.original.user_email
        return (
          <div className="flex flex-col">
            <span className="font-medium">{userName || '-'}</span>
            {userEmail && (
              <span className="text-xs text-muted-foreground">{userEmail}</span>
            )}
          </div>
        )
      },
    },
    {
      header: t.admin.receiver,
      cell: ({ row }: { row: { original: any } }) => {
        const isPrivate = row.original.privacyProtected || row.original.privacy_protected
        const receiverName = row.original.receiverName || row.original.receiver_name
        return (
          <div className="flex items-center gap-2">
            {isPrivate ? (
              <span className="text-muted-foreground">***</span>
            ) : (
              <span>{receiverName || '-'}</span>
            )}
            {isPrivate && (
              <Badge variant="outline" className="flex items-center gap-1 text-xs">
                <Shield className="h-3 w-3" />
                {t.admin.privacy}
              </Badge>
            )}
          </div>
        )
      },
    },
    {
      header: t.admin.trackingNo,
      accessorKey: 'trackingNo',
      cell: ({ row }: { row: { original: any } }) => (
        <span className="font-mono">
          {row.original.trackingNo || row.original.tracking_no || '-'}
        </span>
      ),
    },
    {
      header: t.admin.createdAt,
      cell: ({ row }: { row: { original: any } }) => {
        const date = new Date(row.original.createdAt || row.original.created_at)
        return <span className="text-sm">{date.toLocaleDateString(locale === 'zh' ? 'zh-CN' : 'en-US')}</span>
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
  ]

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-bold">{t.admin.orderManagement}</h1>
        <div className="flex gap-2">
          <Button variant="outline" size="sm" onClick={handleDownloadTemplate}>
            <FileDown className="mr-2 h-4 w-4" />
            {t.admin.downloadTemplate}
          </Button>
          <Button variant="outline" size="sm" onClick={() => fileInputRef.current?.click()}>
            <Upload className="mr-2 h-4 w-4" />
            {t.admin.importLogistics}
          </Button>
          <input
            ref={fileInputRef}
            type="file"
            accept=".xlsx"
            onChange={handleFileChange}
            style={{ display: 'none' }}
          />
          <Button variant="outline" size="sm" onClick={handleExport}>
            <Download className="mr-2 h-4 w-4" />
            {t.admin.exportOrders}
          </Button>
          <Button variant="outline" size="sm" onClick={() => refetch()}>
            <RefreshCw className="mr-2 h-4 w-4" />
            {t.admin.refresh}
          </Button>
        </div>
      </div>

      <OrderFilter
        status={status}
        search={search}
        productSearch={productSearch}
        userId={userId}
        country={country}
        useSmartCountryFilter={true}
        onStatusChange={(newStatus) => {
          setStatus(newStatus)
          setPage(1)
        }}
        onSearchChange={(newSearch) => {
          setSearch(newSearch)
          setPage(1)
        }}
        onProductSearchChange={(newProductSearch) => {
          setProductSearch(newProductSearch)
          setPage(1)
        }}
        onUserChange={(newUserId) => {
          setUserId(newUserId)
          setPage(1)
        }}
        onCountryChange={(newCountry) => {
          setCountry(newCountry)
          setPage(1)
        }}
      />

      {promoCode || promoCodeId ? (
        <div className="flex items-center gap-2">
          <Badge variant="secondary" className="font-mono">
            {t.promoCode.code}: {promoCode || '-'}
            {promoCodeId ? ` (#${promoCodeId})` : ''}
          </Badge>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => {
              setPromoCode('')
              setPromoCodeId(undefined)
              setPage(1)
            }}
          >
            {t.common.reset}
          </Button>
        </div>
      ) : null}

      {selectedIds.size > 0 && (
        <div className="flex items-center gap-3 p-3 bg-muted/50 border rounded-lg">
          <span className="text-sm font-medium">
            {t.admin.selectedCount.replace('{count}', String(selectedIds.size))}
          </span>
          <Button variant="outline" size="sm" onClick={toggleSelectAll}>
            {allSelected ? t.admin.deselectAll : t.admin.selectAll}
          </Button>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button size="sm" disabled={batchMutation.isPending}>
                {batchMutation.isPending ? t.admin.processing : t.admin.batchActions}
                <ChevronDown className="ml-2 h-4 w-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent>
              <DropdownMenuItem onClick={() => handleBatchAction('complete')}>
                <CheckCircle2 className="mr-2 h-4 w-4" />
                {t.admin.batchComplete}
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => handleBatchAction('cancel')}>
                <XCircle className="mr-2 h-4 w-4" />
                {t.admin.batchCancel}
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => handleBatchAction('delete')} className="text-destructive">
                <Trash2 className="mr-2 h-4 w-4" />
                {t.admin.batchDelete}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
          <Button variant="ghost" size="sm" onClick={() => setSelectedIds(new Set())}>
            <X className="mr-1 h-4 w-4" />
            {t.admin.deselectAll}
          </Button>
        </div>
      )}

      <DataTable
        columns={columns}
        data={currentItems}
        isLoading={isLoading}
        pagination={{
          page,
          total_pages: data?.data?.pagination?.total_pages || 1,
          onPageChange: handlePageChange,
        }}
      />

      {/* 批量操作二次确认对话框 */}
      <AlertDialog open={batchDialogOpen} onOpenChange={setBatchDialogOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t.admin.batchConfirmTitle}</AlertDialogTitle>
            <AlertDialogDescription>
              {getBatchConfirmMsg()}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t.common.cancel}</AlertDialogCancel>
            <AlertDialogAction
              onClick={confirmBatchAction}
              className={batchAction === 'delete' ? 'bg-destructive text-destructive-foreground hover:bg-destructive/90' : ''}
            >
              {batchMutation.isPending ? t.admin.processing : t.common.confirm}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

export default function AdminOrdersPage() {
  return (
    <Suspense fallback={
      <div className="flex items-center justify-center min-h-screen">
        <div className="h-10 w-10 rounded-full border-4 border-muted border-t-primary animate-spin" />
      </div>
    }>
      <AdminOrdersContent />
    </Suspense>
  )
}
