'use client'

import { useDeferredValue, useRef, useState } from 'react'
import { useQuery, useMutation } from '@tanstack/react-query'
import { getAdminPromoCodes, deletePromoCode } from '@/lib/api'
import { DataTable } from '@/components/admin/data-table'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Download, Plus, RefreshCw, Pencil, Trash2, Upload } from 'lucide-react'
import Link from 'next/link'
import toast from 'react-hot-toast'
import { Input } from '@/components/ui/input'
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
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'
import { resolveApiErrorMessage } from '@/lib/api-error'
import { resolveClientAPIProxyURL } from '@/lib/api-base-url'
import { PluginExtensionList } from '@/components/plugins/plugin-extension-list'
import { PluginSlot } from '@/components/plugins/plugin-slot'
import { usePluginExtensionBatch } from '@/lib/plugin-extension-batch'
import { usePermission } from '@/hooks/use-permission'

interface PromoCode {
  id: number
  code: string
  name: string
  discount_type: 'percentage' | 'fixed'
  discount_value_minor: number
  max_discount_minor?: number
  min_order_amount_minor?: number
  total_quantity: number
  used_quantity: number
  reserved_quantity: number
  status: string
  expires_at: string | null
}

function buildAdminPromoCodeSummary(promo: PromoCode) {
  return {
    id: promo.id,
    code: promo.code,
    name: promo.name,
    discount_type: promo.discount_type,
    discount_value_minor: promo.discount_value_minor,
    max_discount_minor: promo.max_discount_minor,
    min_order_amount_minor: promo.min_order_amount_minor,
    total_quantity: promo.total_quantity,
    used_quantity: promo.used_quantity,
    reserved_quantity: promo.reserved_quantity,
    status: promo.status,
    expires_at: promo.expires_at,
  }
}

export default function AdminPromoCodesPage() {
  const [page, setPage] = useState(1)
  const [status, setStatus] = useState<string | undefined>()
  const [search, setSearch] = useState('')
  const [deleteId, setDeleteId] = useState<number | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const deferredSearch = useDeferredValue(search)
  const { locale } = useLocale()
  const { hasPermission } = usePermission()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.adminPromoCodes)
  const canEditPromoCodes = hasPermission('product.edit')
  const canDeletePromoCodes = hasPermission('product.delete')

  const readFetchErrorMessage = async (response: Response, fallback: string) => {
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
  }

  const statusConfig: Record<string, { label: string; color: string }> = {
    active: {
      label: t.promoCode.active,
      color: 'bg-green-500/20 text-green-700 dark:text-green-400',
    },
    inactive: {
      label: t.promoCode.inactive,
      color: 'bg-gray-500/20 text-gray-700 dark:text-gray-400',
    },
    expired: { label: t.promoCode.expired, color: 'bg-red-500/20 text-red-700 dark:text-red-400' },
  }

  const { data, isLoading, refetch } = useQuery({
    queryKey: ['adminPromoCodes', page, status, deferredSearch],
    queryFn: () =>
      getAdminPromoCodes({
        page,
        limit: 20,
        status: status === 'all' ? undefined : status,
        search: deferredSearch || undefined,
      }),
  })

  // 删除优惠码
  const deleteMutation = useMutation({
    mutationFn: deletePromoCode,
    onSuccess: () => {
      toast.success(t.promoCode.promoCodeDeleted)
      refetch()
      setDeleteId(null)
    },
    onError: (error: unknown) => {
      toast.error(resolveApiErrorMessage(error, t, t.promoCode.deleteFailed))
    },
  })

  const importMutation = useMutation({
    mutationFn: async (file: File) => {
      const formData = new FormData()
      formData.append('file', file)
      formData.append('conflict_mode', 'upsert')

      const response = await fetch(resolveClientAPIProxyURL('/api/admin/promo-codes/import'), {
        method: 'POST',
        body: formData,
      })

      if (!response.ok) {
        throw new Error(await readFetchErrorMessage(response, t.admin.importFailed))
      }

      return response.json()
    },
    onSuccess: (data) => {
      toast.dismiss()
      const result = data?.data
      if (result?.error_count > 0) {
        toast.error(result.message || t.promoCode.importFailed, {
          duration: 6000,
        })
        if (Array.isArray(result.errors) && result.errors.length > 0) {
          console.error('Promo code import errors:', result.errors)
        }
      } else {
        toast.success(result?.message || t.promoCode.importSuccess)
      }
      refetch()
    },
    onError: (error: unknown) => {
      toast.dismiss()
      toast.error(resolveApiErrorMessage(error, t, t.admin.importFailed))
    },
  })

  const handleExport = () => {
    const params = new URLSearchParams()
    if (status && status !== 'all') params.append('status', status)
    if (deferredSearch.trim()) params.append('search', deferredSearch.trim())

    const url = resolveClientAPIProxyURL(`/api/admin/promo-codes/export?${params.toString()}`)

    fetch(url)
      .then(async (res) => {
        if (!res.ok) {
          throw new Error(await readFetchErrorMessage(res, t.admin.exportFailed))
        }
        return res.blob()
      })
      .then((blob) => {
        const blobUrl = window.URL.createObjectURL(blob)
        const a = document.createElement('a')
        a.href = blobUrl
        a.download = `promo_codes_${new Date().toISOString().slice(0, 10)}.xlsx`
        document.body.appendChild(a)
        a.click()
        document.body.removeChild(a)
        window.URL.revokeObjectURL(blobUrl)
        toast.success(t.promoCode.exportSuccess)
      })
      .catch((err: Error) => {
        toast.error(`${t.admin.exportFailed}: ${err.message}`)
      })
  }

  const handleImportClick = () => {
    fileInputRef.current?.click()
  }

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) {
      return
    }

    if (!file.name.toLowerCase().endsWith('.xlsx')) {
      toast.error(t.admin.fileFormatError)
      if (fileInputRef.current) {
        fileInputRef.current.value = ''
      }
      return
    }

    toast.loading(t.admin.importLoading)
    importMutation.mutate(file)

    if (fileInputRef.current) {
      fileInputRef.current.value = ''
    }
  }

  const columns = [
    {
      header: t.promoCode.code,
      cell: ({ row }: { row: { original: PromoCode } }) => {
        const promo = row.original
        return <div className="font-mono font-medium">{promo.code}</div>
      },
    },
    {
      header: t.promoCode.name,
      accessorKey: 'name',
    },
    {
      header: t.promoCode.discountType,
      cell: ({ row }: { row: { original: PromoCode } }) => {
        const promo = row.original
        return (
          <div>
            {promo.discount_type === 'percentage'
              ? `${((promo.discount_value_minor ?? 0) / 100).toFixed(2)}%`
              : `\u00a5${((promo.discount_value_minor ?? 0) / 100).toFixed(2)}`}
          </div>
        )
      },
    },
    {
      header: t.promoCode.usedQuantity,
      cell: ({ row }: { row: { original: PromoCode } }) => {
        const promo = row.original
        const total = promo.total_quantity === 0 ? t.promoCode.unlimited : promo.total_quantity
        return (
          <div className="flex flex-col gap-0.5">
            <div className="font-semibold tabular-nums">{promo.used_quantity}</div>
            <div className="text-xs tabular-nums text-muted-foreground">
              {t.promoCode.reservedQuantity}: {promo.reserved_quantity}
              {' · '}
              {t.promoCode.totalQuantity}: {total}
            </div>
          </div>
        )
      },
    },
    {
      header: t.promoCode.status,
      cell: ({ row }: { row: { original: PromoCode } }) => {
        const promoStatus = row.original.status
        const config = statusConfig[promoStatus] || statusConfig['inactive']
        return <Badge className={config.color}>{config.label}</Badge>
      },
    },
    {
      header: t.promoCode.expiresAt,
      cell: ({ row }: { row: { original: PromoCode } }) => {
        const promo = row.original
        if (!promo.expires_at) {
          return <span className="text-muted-foreground">{t.promoCode.noExpiry}</span>
        }
        return <div>{new Date(promo.expires_at).toLocaleDateString()}</div>
      },
    },
    {
      header: t.admin.actions,
      cell: ({ row }: { row: { original: PromoCode } }) => {
        const promo = row.original
        const rowExtensions = adminPromoCodeRowActionExtensions[String(promo.id)] || []
        return (
          <div className="flex items-center gap-2">
            {canEditPromoCodes ? (
              <Button asChild size="sm" variant="outline">
                <Link href={`/admin/promo-codes/${promo.id}`}>
                  <Pencil className="h-4 w-4" />
                </Link>
              </Button>
            ) : null}
            {canDeletePromoCodes ? (
              <Button size="sm" variant="destructive" onClick={() => setDeleteId(promo.id)}>
                <Trash2 className="h-4 w-4" />
              </Button>
            ) : null}
            {rowExtensions.length > 0 ? (
              <PluginExtensionList extensions={rowExtensions} display="inline" />
            ) : null}
          </div>
        )
      },
    },
  ]
  const promoCodes = data?.data?.items || []
  const totalPromoCodes = Number(data?.data?.pagination?.total || 0)
  const promoFilterBadges = [
    deferredSearch.trim() ? `${t.common.search}: ${deferredSearch.trim()}` : null,
    status
      ? `${t.promoCode.status}: ${(statusConfig[status] || statusConfig.inactive).label}`
      : null,
  ].filter(Boolean) as string[]
  const deleteTarget = deleteId ? promoCodes.find((item: PromoCode) => item.id === deleteId) : null
  const adminPromoCodesPluginContext = {
    view: 'admin_promo_codes',
    filters: {
      search: deferredSearch.trim() || undefined,
      status: status || undefined,
    },
    pagination: {
      page,
      total_pages: data?.data?.pagination?.total_pages || 1,
      total: totalPromoCodes,
    },
    summary: {
      total_records: totalPromoCodes,
      filtered_count: promoCodes.length,
      active_filter_count: promoFilterBadges.length,
    },
  }
  const adminPromoCodeRowActionItems = promoCodes.map((promo: PromoCode, index: number) => ({
    key: String(promo.id),
    slot: 'admin.promo_codes.row_actions',
    path: '/admin/promo-codes',
    hostContext: {
      view: 'admin_promo_codes_row',
      promo_code: buildAdminPromoCodeSummary(promo),
      row: {
        index: index + 1,
      },
      filters: adminPromoCodesPluginContext.filters,
      summary: adminPromoCodesPluginContext.summary,
    },
  }))
  const adminPromoCodeRowActionExtensions = usePluginExtensionBatch({
    scope: 'admin',
    path: '/admin/promo-codes',
    items: adminPromoCodeRowActionItems,
    enabled: adminPromoCodeRowActionItems.length > 0,
  })

  return (
    <div className="space-y-6">
      <PluginSlot slot="admin.promo_codes.top" context={adminPromoCodesPluginContext} />
      <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
        <div>
          <h1 className="text-3xl font-bold">{t.promoCode.promoCodeManagement}</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            {t.admin.totalRecords.replace('{count}', String(totalPromoCodes))}
          </p>
          <p className="mt-2 text-xs text-muted-foreground">{t.promoCode.importHint}</p>
        </div>
        <div className="flex gap-2">
          {canEditPromoCodes ? (
            <Button asChild>
              <Link href="/admin/promo-codes/new">
                <Plus className="mr-2 h-4 w-4" />
                {t.promoCode.addPromoCode}
              </Link>
            </Button>
          ) : null}
          {canEditPromoCodes ? (
            <>
              <input
                ref={fileInputRef}
                type="file"
                accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
                className="hidden"
                onChange={handleFileChange}
              />
              <Button
                variant="outline"
                onClick={handleImportClick}
                disabled={importMutation.isPending}
              >
                <Upload className="mr-2 h-4 w-4" />
                {t.promoCode.importPromoCodes}
              </Button>
            </>
          ) : null}
          <Button variant="outline" onClick={handleExport}>
            <Download className="mr-2 h-4 w-4" />
            {t.promoCode.exportPromoCodes}
          </Button>
          <Button variant="outline" onClick={() => refetch()}>
            <RefreshCw className="mr-2 h-4 w-4" />
            {t.admin.refresh}
          </Button>
        </div>
      </div>

      {/* 筛选器 */}
      <div className="flex flex-col gap-3">
        <PluginSlot slot="admin.promo_codes.filters" context={adminPromoCodesPluginContext} />
        <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
          <div className="flex gap-4">
            <Input
              placeholder={t.promoCode.searchPlaceholder}
              value={search}
              onChange={(e) => {
                setSearch(e.target.value)
                setPage(1)
              }}
              className="max-w-xs"
            />
            <Select
              value={status || 'all'}
              onValueChange={(value) => {
                setStatus(value === 'all' ? undefined : value)
                setPage(1)
              }}
            >
              <SelectTrigger className="w-[150px]">
                <SelectValue placeholder={t.promoCode.status} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">{t.promoCode.allStatus}</SelectItem>
                <SelectItem value="active">{t.promoCode.active}</SelectItem>
                <SelectItem value="inactive">{t.promoCode.inactive}</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
        {promoFilterBadges.length > 0 ? (
          <p className="text-sm text-muted-foreground">{promoFilterBadges.join(' · ')}</p>
        ) : null}
      </div>

      <DataTable
        columns={columns}
        data={promoCodes}
        isLoading={isLoading}
        pagination={{
          page,
          total_pages: data?.data?.pagination?.total_pages || 1,
          onPageChange: setPage,
        }}
      />

      {/* 删除确认对话框 */}
      <AlertDialog open={deleteId !== null} onOpenChange={() => setDeleteId(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t.admin.confirmDelete}</AlertDialogTitle>
            <AlertDialogDescription>
              <div className="space-y-3">
                <p>{t.promoCode.confirmDelete}</p>
                {deleteTarget ? (
                  <div className="rounded-md border border-dashed bg-muted/20 p-3 text-xs text-foreground">
                    <div className="font-medium">{deleteTarget.code}</div>
                    <div className="mt-1 flex flex-wrap items-center gap-2 text-muted-foreground">
                      <span>{deleteTarget.name || '-'}</span>
                      <span>
                        {(statusConfig[deleteTarget.status] || statusConfig.inactive).label}
                      </span>
                      <span>
                        {t.promoCode.usedQuantity}: {deleteTarget.used_quantity}
                      </span>
                    </div>
                  </div>
                ) : null}
              </div>
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t.common.cancel}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => deleteId && canDeletePromoCodes && deleteMutation.mutate(deleteId)}
              className="bg-red-600 hover:bg-red-700"
            >
              {t.common.delete}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
