'use client'

import { Suspense, useState, useEffect, useRef, useCallback } from 'react'
import { useSearchParams } from 'next/navigation'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { getAdminOrders, batchUpdateOrders } from '@/lib/api'
import { resolveApiErrorMessage } from '@/lib/api-error'
import { DataTable } from '@/components/admin/data-table'
import { OrderStatusBadge } from '@/components/orders/order-status-badge'
import { OrderFilter } from '@/components/orders/order-filter'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import { Card, CardContent } from '@/components/ui/card'
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
import {
  Shield,
  RefreshCw,
  Download,
  Upload,
  FileDown,
  Package,
  CheckCircle2,
  XCircle,
  Trash2,
  ChevronDown,
  X,
} from 'lucide-react'
import Link from 'next/link'
import { getToken } from '@/lib/auth'
import toast from 'react-hot-toast'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'
import { PluginExtensionList } from '@/components/plugins/plugin-extension-list'
import { PluginSlot } from '@/components/plugins/plugin-slot'
import { usePluginExtensionBatch } from '@/lib/plugin-extension-batch'

function buildAdminOrderRowSummary(order: any) {
  const items = Array.isArray(order?.items) ? order.items : []
  return {
    id: order?.id,
    order_no: order?.orderNo || order?.order_no,
    status: order?.status,
    user_id: order?.userId ?? order?.user_id,
    external_user_id: order?.externalUserID ?? order?.external_user_id,
    external_user_name: order?.externalUserName || order?.external_user_name,
    user_email: order?.userEmail || order?.user_email,
    tracking_no: order?.trackingNo || order?.tracking_no,
    receiver_name: order?.receiverName || order?.receiver_name,
    receiver_country: order?.receiverCountry || order?.receiver_country,
    privacy_protected: Boolean(order?.privacyProtected || order?.privacy_protected),
    created_at: order?.createdAt || order?.created_at,
    item_count: items.length,
    items_preview: items.slice(0, 3).map((item: any) => ({
      name: item?.name,
      quantity: item?.quantity,
      sku: item?.sku,
    })),
  }
}

function orderStatusLabel(
  status: string | undefined,
  t: ReturnType<typeof getTranslations>
): string {
  switch (status) {
    case 'pending_payment':
      return t.order.status.pending_payment
    case 'draft':
      return t.order.status.draft
    case 'pending':
      return t.order.status.pending
    case 'need_resubmit':
      return t.order.status.need_resubmit
    case 'shipped':
      return t.order.status.shipped
    case 'completed':
      return t.order.status.completed
    case 'cancelled':
      return t.order.status.cancelled
    case 'refund_pending':
      return t.order.status.refund_pending
    case 'refunded':
      return t.order.status.refunded
    default:
      return status || t.common.all
  }
}

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

  const readFetchErrorMessage = useCallback(
    async (response: Response, fallback: string) => {
      try {
        const contentType = response.headers.get('content-type') || ''
        if (contentType.includes('application/json')) {
          const payload = await response.json()
          return resolveApiErrorMessage(payload, t, fallback)
        }

        const text = (await response.text()).trim()
        if (text) {
          try {
            return resolveApiErrorMessage(JSON.parse(text), t, fallback)
          } catch {
            return text
          }
        }
      } catch {
        // ignore parse errors and fall back to the provided message
      }
      return fallback
    },
    [t]
  )

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
    queryKey: [
      'adminOrders',
      page,
      status,
      search,
      productSearch,
      promoCodeId,
      promoCode,
      userId,
      country,
    ],
    queryFn: () =>
      getAdminOrders({
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
        Authorization: `Bearer ${token}`,
      },
    })
      .then(async (res) => {
        if (!res.ok) {
          throw new Error(await readFetchErrorMessage(res, t.admin.exportFailed))
        }
        return res.blob()
      })
      .then((blob) => {
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
      .catch((err) => {
        toast.error(`${t.admin.exportFailed}: ${err.message}`)
      })
  }

  const handleDownloadTemplate = () => {
    const token = getToken()
    const url = `${process.env.NEXT_PUBLIC_API_URL || 'https://auralogic.un1c0de.com'}/api/admin/orders/import-template`

    fetch(url, {
      headers: {
        Authorization: `Bearer ${token}`,
      },
    })
      .then(async (res) => {
        if (!res.ok) {
          throw new Error(await readFetchErrorMessage(res, t.admin.downloadFailed))
        }
        return res.blob()
      })
      .then((blob) => {
        const url = window.URL.createObjectURL(blob)
        const a = document.createElement('a')
        a.href = url
        a.download = 'order_logistics_import_template.xlsx'
        document.body.appendChild(a)
        a.click()
        document.body.removeChild(a)
        window.URL.revokeObjectURL(url)
      })
      .catch((err) => {
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
            Authorization: `Bearer ${token}`,
          },
          body: formData,
        }
      )

      if (!response.ok) {
        throw new Error(await readFetchErrorMessage(response, t.admin.importFailed))
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
    onError: (error: any) => {
      toast.dismiss()
      toast.error(resolveApiErrorMessage(error, t, t.admin.importFailed))
    },
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
    mutationFn: ({ orderIds, action }: { orderIds: number[]; action: string }) =>
      batchUpdateOrders(orderIds, action),
    onSuccess: (data) => {
      const result = data.data
      toast.success(
        t.admin.batchSuccess
          .replace('{success}', String(result.success_count))
          .replace('{failed}', String(result.failed_count))
      )
      setSelectedIds(new Set())
      refetch()
      queryClient.invalidateQueries({ queryKey: ['adminOrders'] })
    },
    onError: (error: any) => {
      toast.error(resolveApiErrorMessage(error, t, t.order.operationFailed))
    },
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
  const allSelected = currentItems.length > 0 && allCurrentIds.every((id) => selectedIds.has(id))
  const selectedIdList = Array.from(selectedIds).sort((left, right) => left - right)
  const totalPages = Number(data?.data?.pagination?.total_pages || 1)
  const selectedOrderPreview = currentItems
    .filter((item: any) => selectedIds.has(item.id))
    .slice(0, 5)
    .map((item: any) => String(item.orderNo || item.order_no || `#${item.id}`))
  const selectedOrderPreviewMore = Math.max(0, selectedIds.size - selectedOrderPreview.length)
  const adminOrdersFilters = {
    page,
    status: status === 'all' ? undefined : status,
    search: search || undefined,
    product_search: productSearch || undefined,
    promo_code: promoCode || undefined,
    promo_code_id: promoCodeId,
    user_id: userId,
    country: country === 'all' ? undefined : country,
  }
  const adminOrdersPagination = {
    page,
    total_pages: data?.data?.pagination?.total_pages,
    total: data?.data?.pagination?.total,
    limit: data?.data?.pagination?.limit,
  }
  const adminOrdersPluginContext = {
    view: 'admin_orders',
    filters: adminOrdersFilters,
    selection: {
      selected_count: selectedIdList.length,
      selected_ids:
        selectedIdList.length > 0 && selectedIdList.length <= 20 ? selectedIdList : undefined,
      selected_preview_ids: selectedIdList.slice(0, 20),
      current_page_ids: allCurrentIds,
      all_selected: allSelected,
    },
    pagination: adminOrdersPagination,
  }
  const adminOrderRowActionItems = currentItems.map((item: any) => ({
    key: String(item.id),
    slot: 'admin.orders.row_actions',
    path: '/admin/orders',
    hostContext: {
      view: 'admin_orders_row',
      order: buildAdminOrderRowSummary(item),
      row: {
        selected: selectedIds.has(item.id),
      },
      filters: adminOrdersFilters,
      selection: {
        selected_count: selectedIdList.length,
        all_selected: allSelected,
      },
      pagination: adminOrdersPagination,
    },
  }))
  const adminOrderRowActionExtensions = usePluginExtensionBatch({
    scope: 'admin',
    path: '/admin/orders',
    items: adminOrderRowActionItems,
    enabled: currentItems.length > 0,
  })

  const toggleSelectAll = () => {
    if (allSelected) {
      setSelectedIds(new Set())
    } else {
      setSelectedIds(new Set(allCurrentIds))
    }
  }

  const toggleSelectOne = (id: number) => {
    setSelectedIds((prev) => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      return next
    })
  }

  const batchActionLabel =
    batchAction === 'complete'
      ? t.admin.batchComplete
      : batchAction === 'cancel'
        ? t.admin.batchCancel
        : batchAction === 'delete'
          ? t.admin.batchDelete
          : t.admin.batchActions

  const columns = [
    {
      header: () => (
        <Checkbox checked={allSelected} onCheckedChange={toggleSelectAll} aria-label="Select all" />
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
        <span className="font-mono text-sm">{row.original.orderNo || row.original.order_no}</span>
      ),
    },
    {
      header: t.admin.product,
      cell: ({ row }: { row: { original: any } }) => {
        const items = row.original.items || []
        return (
          <div className="flex max-w-[200px] flex-col gap-1">
            {items.slice(0, 2).map((item: any, index: number) => (
              <div key={index} className="flex items-center gap-2 text-sm">
                <Package className="h-3 w-3 flex-shrink-0 text-muted-foreground" />
                <span className="truncate" title={item.name}>
                  {item.name}
                </span>
                <Badge variant="secondary" className="flex-shrink-0 text-xs">
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
        const externalUserID = row.original.externalUserID || row.original.external_user_id
        const displayName = userName || userEmail || externalUserID || '-'
        return (
          <div className="flex flex-col">
            <span className="font-medium">{displayName}</span>
            {userEmail && userEmail !== displayName && (
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
        const raw = row.original.createdAt || row.original.created_at
        if (!raw) return <span className="text-sm">-</span>
        const date = new Date(raw)
        if (isNaN(date.getTime())) return <span className="text-sm">-</span>
        return (
          <span className="text-sm">
            {date.toLocaleDateString(locale === 'zh' ? 'zh-CN' : 'en-US')}
          </span>
        )
      },
    },
    {
      header: t.admin.actions,
      cell: ({ row }: { row: { original: any } }) => {
        const rowExtensions = adminOrderRowActionExtensions[String(row.original.id)] || []
        return (
          <div className="flex flex-wrap items-center gap-2">
            <Button asChild size="sm" variant="outline">
              <Link href={`/admin/orders/${row.original.id}`}>{t.admin.view}</Link>
            </Button>
            <PluginExtensionList extensions={rowExtensions} display="inline" />
          </div>
        )
      },
    },
  ]

  return (
    <div className="space-y-6">
      <PluginSlot slot="admin.orders.top" context={adminOrdersPluginContext} />
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
          <PluginSlot
            slot="admin.orders.actions"
            context={adminOrdersPluginContext}
            display="inline"
          />
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
          setSelectedIds(new Set())
        }}
        onSearchChange={(newSearch) => {
          setSearch(newSearch)
          setPage(1)
          setSelectedIds(new Set())
        }}
        onProductSearchChange={(newProductSearch) => {
          setProductSearch(newProductSearch)
          setPage(1)
          setSelectedIds(new Set())
        }}
        onUserChange={(newUserId) => {
          setUserId(newUserId)
          setPage(1)
          setSelectedIds(new Set())
        }}
        onCountryChange={(newCountry) => {
          setCountry(newCountry)
          setPage(1)
          setSelectedIds(new Set())
        }}
      />
      {selectedIds.size > 0 && (
        <div className="space-y-3 rounded-lg border bg-muted/50 p-3">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="space-y-2">
              <div className="flex flex-wrap items-center gap-2">
                <Badge variant="secondary">
                  {t.admin.selectedCount.replace('{count}', String(selectedIds.size))}
                </Badge>
                {allSelected ? <Badge variant="outline">{t.admin.selectAll}</Badge> : null}
                {selectedOrderPreview.map((orderNo) => (
                  <Badge key={orderNo} variant="outline" className="font-mono">
                    {orderNo}
                  </Badge>
                ))}
                {selectedOrderPreviewMore > 0 ? (
                  <Badge variant="outline">
                    {t.admin.batchSelectionPreviewMore.replace(
                      '{count}',
                      String(selectedOrderPreviewMore)
                    )}
                  </Badge>
                ) : null}
              </div>
              <p className="text-xs text-muted-foreground">
                {t.admin.page
                  .replace('{current}', String(page))
                  .replace('{total}', String(totalPages))}
              </p>
            </div>
            <div className="flex flex-wrap items-center gap-2">
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
                  <DropdownMenuItem
                    onClick={() => handleBatchAction('delete')}
                    className="text-destructive"
                  >
                    <Trash2 className="mr-2 h-4 w-4" />
                    {t.admin.batchDelete}
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
              <Button variant="ghost" size="sm" onClick={() => setSelectedIds(new Set())}>
                <X className="mr-1 h-4 w-4" />
                {t.admin.deselectAll}
              </Button>
              <PluginSlot
                slot="admin.orders.batch_actions"
                context={adminOrdersPluginContext}
                display="inline"
              />
            </div>
          </div>
        </div>
      )}
      <PluginSlot slot="admin.orders.before_table" context={adminOrdersPluginContext} />

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
      <PluginSlot slot="admin.orders.bottom" context={adminOrdersPluginContext} />

      {/* 批量操作二次确认对话框 */}
      <AlertDialog open={batchDialogOpen} onOpenChange={setBatchDialogOpen}>
        <AlertDialogContent className="max-w-lg">
          <AlertDialogHeader>
            <AlertDialogTitle>{t.admin.batchConfirmTitle}</AlertDialogTitle>
            <AlertDialogDescription>{getBatchConfirmMsg()}</AlertDialogDescription>
          </AlertDialogHeader>
          <div className="space-y-3">
            <div
              className={`rounded-md border p-3 text-sm ${
                batchAction === 'delete'
                  ? 'border-destructive/30 bg-destructive/5'
                  : 'border-input/60 bg-muted/10'
              }`}
            >
              <div className="flex flex-wrap items-center gap-2">
                <Badge variant={batchAction === 'delete' ? 'destructive' : 'secondary'}>
                  {batchActionLabel}
                </Badge>
                <Badge variant="outline">
                  {t.admin.selectedCount.replace('{count}', String(selectedIds.size))}
                </Badge>
              </div>
            </div>
            {selectedOrderPreview.length > 0 ? (
              <div className="rounded-md border border-input/60 bg-muted/10 p-3 text-sm">
                <div className="flex flex-wrap gap-2">
                  {selectedOrderPreview.map((orderNo) => (
                    <Badge key={orderNo} variant="outline" className="font-mono">
                      {orderNo}
                    </Badge>
                  ))}
                  {selectedOrderPreviewMore > 0 ? (
                    <Badge variant="outline">
                      {t.admin.batchSelectionPreviewMore.replace(
                        '{count}',
                        String(selectedOrderPreviewMore)
                      )}
                    </Badge>
                  ) : null}
                </div>
              </div>
            ) : null}
          </div>
          <AlertDialogFooter>
            <AlertDialogCancel>{t.common.cancel}</AlertDialogCancel>
            <AlertDialogAction
              onClick={confirmBatchAction}
              className={
                batchAction === 'delete'
                  ? 'bg-destructive text-destructive-foreground hover:bg-destructive/90'
                  : ''
              }
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
    <Suspense
      fallback={
        <div className="flex min-h-screen items-center justify-center">
          <div className="h-10 w-10 animate-spin rounded-full border-4 border-muted border-t-primary" />
        </div>
      }
    >
      <AdminOrdersContent />
    </Suspense>
  )
}
