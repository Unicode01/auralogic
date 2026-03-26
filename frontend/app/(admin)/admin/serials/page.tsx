'use client'

import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
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
import { DataTable } from '@/components/admin/data-table'
import { Package, Eye, ShoppingBag, RefreshCw, Search, Trash2 } from 'lucide-react'
import { formatDate } from '@/lib/utils'
import toast from 'react-hot-toast'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'
import { getToken } from '@/lib/auth'
import { PluginSlot } from '@/components/plugins/plugin-slot'

interface ProductSerial {
  id: number
  serial_number: string
  product_id: number
  order_id: number
  product_code: string
  sequence_number: number
  anti_counterfeit_code: string
  view_count: number
  first_viewed_at?: string
  last_viewed_at?: string
  created_at: string
  product?: {
    id: number
    name: string
    sku: string
  }
  order?: {
    id: number
    order_no: string
    user_email?: string
  }
}

async function getSerials(page: number, limit: number, filters: any) {
  const params = new URLSearchParams()
  params.append('page', page.toString())
  params.append('limit', limit.toString())

  if (filters.serial_number) params.append('serial_number', filters.serial_number)
  if (filters.product_code) params.append('product_code', filters.product_code)
  if (filters.product_id) params.append('product_id', filters.product_id)
  if (filters.order_id) params.append('order_id', filters.order_id)

  const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
  const token = getToken()
  const response = await fetch(`${API_BASE_URL}/api/admin/serials?${params}`, {
    headers: {
      Authorization: `Bearer ${token}`,
    },
  })

  if (!response.ok) {
    throw new Error('Failed to fetch serials')
  }

  return response.json()
}

async function getStatistics() {
  const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
  const token = getToken()
  const response = await fetch(`${API_BASE_URL}/api/admin/serials/statistics`, {
    headers: {
      Authorization: `Bearer ${token}`,
    },
  })

  if (!response.ok) {
    throw new Error('Failed to fetch statistics')
  }

  return response.json()
}

async function deleteSerial(id: number) {
  const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
  const token = getToken()
  const response = await fetch(`${API_BASE_URL}/api/admin/serials/${id}`, {
    method: 'DELETE',
    headers: {
      Authorization: `Bearer ${token}`,
    },
  })

  if (!response.ok) {
    throw new Error('Failed to delete serial')
  }

  return response.json()
}

export default function SerialsPage() {
  const queryClient = useQueryClient()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.adminSerials)
  const [page, setPage] = useState(1)
  const [filters, setFilters] = useState({
    serial_number: '',
    product_code: '',
    product_id: '',
    order_id: '',
  })
  const [searchFilters, setSearchFilters] = useState(filters)
  const [deleteTarget, setDeleteTarget] = useState<ProductSerial | null>(null)

  const { data, isLoading, refetch } = useQuery({
    queryKey: ['adminSerials', page, searchFilters],
    queryFn: () => getSerials(page, 20, searchFilters),
  })

  const { data: statsData } = useQuery({
    queryKey: ['serialStatistics'],
    queryFn: getStatistics,
  })

  const deleteMutation = useMutation({
    mutationFn: deleteSerial,
    onSuccess: () => {
      toast.success(t.admin.serialDeleted)
      queryClient.invalidateQueries({ queryKey: ['adminSerials'] })
      queryClient.invalidateQueries({ queryKey: ['serialStatistics'] })
      setDeleteTarget(null)
    },
    onError: () => {
      toast.error(t.admin.serialDeleteFailed)
    },
  })

  const handleSearch = () => {
    setSearchFilters(filters)
    setPage(1)
  }

  const handleReset = () => {
    setFilters({
      serial_number: '',
      product_code: '',
      product_id: '',
      order_id: '',
    })
    setSearchFilters({
      serial_number: '',
      product_code: '',
      product_id: '',
      order_id: '',
    })
    setPage(1)
  }

  const columns = [
    {
      header: t.admin.serialNumber,
      cell: ({ row }: { row: { original: ProductSerial } }) => {
        const serial = row.original
        return (
          <div>
            <div className="font-mono text-lg font-bold">{serial.serial_number}</div>
            <div className="mt-1 text-xs text-muted-foreground">
              {t.admin.serialNumberDetail
                .replace('{code}', serial.product_code)
                .replace('{seq}', String(serial.sequence_number))
                .replace('{anti}', serial.anti_counterfeit_code)}
            </div>
          </div>
        )
      },
    },
    {
      header: t.admin.productInfoLabel,
      cell: ({ row }: { row: { original: ProductSerial } }) => {
        const serial = row.original
        return serial.product ? (
          <div>
            <div className="font-medium">{serial.product.name}</div>
            <div className="text-sm text-muted-foreground">SKU: {serial.product.sku}</div>
          </div>
        ) : (
          <span className="text-muted-foreground">-</span>
        )
      },
    },
    {
      header: t.admin.orderInfo,
      cell: ({ row }: { row: { original: ProductSerial } }) => {
        const serial = row.original
        return serial.order ? (
          <div>
            <div className="font-mono text-sm">{serial.order.order_no}</div>
            {serial.order.user_email && (
              <div className="text-xs text-muted-foreground">{serial.order.user_email}</div>
            )}
          </div>
        ) : (
          <span className="text-muted-foreground">-</span>
        )
      },
    },
    {
      header: t.admin.viewCount,
      cell: ({ row }: { row: { original: ProductSerial } }) => {
        const serial = row.original
        return (
          <div className="text-center">
            <div className="flex items-center justify-center gap-1">
              <Eye className="h-4 w-4" />
              <span className="font-bold">{serial.view_count}</span>
            </div>
            {serial.first_viewed_at && (
              <div className="mt-1 text-xs text-muted-foreground">
                {t.admin.firstView}: {formatDate(serial.first_viewed_at)}
              </div>
            )}
            {serial.last_viewed_at && serial.view_count > 1 && (
              <div className="text-xs text-muted-foreground">
                {t.admin.lastView}: {formatDate(serial.last_viewed_at)}
              </div>
            )}
          </div>
        )
      },
    },
    {
      header: t.admin.generateTime,
      cell: ({ row }: { row: { original: ProductSerial } }) => {
        return formatDate(row.original.created_at)
      },
    },
    {
      header: t.admin.actions,
      cell: ({ row }: { row: { original: ProductSerial } }) => {
        const serial = row.original
        return (
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setDeleteTarget(serial)}
            className="text-red-600 hover:bg-red-50 hover:text-red-700 dark:text-red-400 dark:hover:bg-red-950/30 dark:hover:text-red-300"
          >
            <Trash2 className="h-4 w-4" />
          </Button>
        )
      },
    },
  ]

  const stats = statsData?.data || {}
  const serialItems: ProductSerial[] = data?.data?.items || []
  const totalSerialCount = Number(data?.data?.pagination?.total || stats.total_count || 0)
  const visibleStart = totalSerialCount === 0 ? 0 : (page - 1) * 20 + 1
  const visibleEnd = totalSerialCount === 0 ? 0 : visibleStart + serialItems.length - 1
  const serialRangeSummary = t.admin.serialsRangeSummary
    .replace('{start}', String(visibleStart))
    .replace('{end}', String(Math.max(visibleEnd, 0)))
    .replace('{total}', String(totalSerialCount))
  const activeFilterBadges = [
    searchFilters.serial_number ? `${t.admin.serialNumber}: ${searchFilters.serial_number}` : null,
    searchFilters.product_code ? `${t.admin.productCode}: ${searchFilters.product_code}` : null,
    searchFilters.product_id ? `${t.admin.productIdLabel}: ${searchFilters.product_id}` : null,
    searchFilters.order_id ? `${t.admin.orderId}: ${searchFilters.order_id}` : null,
  ].filter(Boolean) as string[]
  const adminSerialsPluginContext = {
    view: 'admin_serials',
    filters: {
      page,
      serial_number: searchFilters.serial_number || undefined,
      product_code: searchFilters.product_code || undefined,
      product_id: searchFilters.product_id || undefined,
      order_id: searchFilters.order_id || undefined,
    },
    pagination: {
      page,
      total_pages: data?.data?.pagination?.total_pages,
      total: totalSerialCount,
      limit: data?.data?.pagination?.limit,
    },
    summary: {
      total_serials: stats.total_count || 0,
      viewed_count: stats.viewed_count || 0,
      total_views: stats.total_views || 0,
      current_page_count: serialItems.length,
      active_filter_count: activeFilterBadges.length,
    },
    active_filter_badges: activeFilterBadges,
  }

  return (
    <div className="space-y-6">
      <PluginSlot slot="admin.serials.top" context={adminSerialsPluginContext} />
      <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
        <div>
          <h1 className="text-3xl font-bold">{t.admin.serialManagement}</h1>
          <p className="mt-1 text-sm text-muted-foreground">{serialRangeSummary}</p>
        </div>
        <Button variant="outline" onClick={() => refetch()}>
          <RefreshCw className="mr-2 h-4 w-4" />
          {t.admin.refresh}
        </Button>
      </div>

      {/* Statistics Cards */}
      <div className="grid grid-cols-1 gap-6 md:grid-cols-3">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">{t.admin.totalSerials}</CardTitle>
            <Package className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{stats.total_count || 0}</div>
            <p className="text-xs text-muted-foreground">{t.admin.totalSerialsGenerated}</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">{t.admin.viewedSerials}</CardTitle>
            <Eye className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{stats.viewed_count || 0}</div>
            <p className="text-xs text-muted-foreground">
              {t.admin.viewRate}:{' '}
              {stats.total_count > 0
                ? ((stats.viewed_count / stats.total_count) * 100).toFixed(1)
                : 0}
              %
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">{t.admin.totalViews}</CardTitle>
            <ShoppingBag className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{stats.total_views || 0}</div>
            <p className="text-xs text-muted-foreground">
              {t.admin.avgPerSerial.replace(
                '{avg}',
                stats.viewed_count > 0 ? (stats.total_views / stats.viewed_count).toFixed(1) : '0'
              )}
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Filters */}
      <Card>
        <CardHeader className="gap-3">
          <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
            <div>
              <CardTitle className="text-base">{t.admin.searchFilter}</CardTitle>
              <CardDescription>
                {activeFilterBadges.length > 0
                  ? t.admin.serialsFilterSummary.replace(
                      '{count}',
                      String(activeFilterBadges.length)
                    )
                  : t.admin.serialsFilterHint}
              </CardDescription>
            </div>
            {activeFilterBadges.length > 0 ? (
              <p className="text-xs text-muted-foreground">{activeFilterBadges.join(' · ')}</p>
            ) : null}
          </div>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-4">
            <div className="space-y-2">
              <label className="text-sm font-medium">{t.admin.serialNumber}</label>
              <Input
                placeholder={t.admin.serialNumberPlaceholder}
                value={filters.serial_number}
                onChange={(e) => setFilters({ ...filters, serial_number: e.target.value })}
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t.admin.productCode}</label>
              <Input
                placeholder={t.admin.productCodePlaceholder}
                value={filters.product_code}
                onChange={(e) => setFilters({ ...filters, product_code: e.target.value })}
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t.admin.productIdLabel}</label>
              <Input
                placeholder={t.admin.productIdInputPlaceholder}
                type="number"
                value={filters.product_id}
                onChange={(e) => setFilters({ ...filters, product_id: e.target.value })}
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">{t.admin.orderId}</label>
              <Input
                placeholder={t.admin.orderIdPlaceholder}
                type="number"
                value={filters.order_id}
                onChange={(e) => setFilters({ ...filters, order_id: e.target.value })}
              />
            </div>
          </div>
          <div className="mt-4 flex gap-2">
            <Button onClick={handleSearch}>
              <Search className="mr-2 h-4 w-4" />
              {t.admin.search}
            </Button>
            <Button variant="outline" onClick={handleReset}>
              {t.admin.reset}
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* Serials List */}
      <DataTable
        columns={columns}
        data={serialItems}
        isLoading={isLoading}
        pagination={{
          page,
          total_pages: data?.data?.pagination?.total_pages || 1,
          onPageChange: setPage,
        }}
      />

      <AlertDialog
        open={!!deleteTarget}
        onOpenChange={(open) => {
          if (!open) {
            setDeleteTarget(null)
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t.admin.confirmDelete}</AlertDialogTitle>
            <AlertDialogDescription>
              <div className="space-y-3">
                <p>
                  {deleteTarget
                    ? t.admin.confirmDeleteSerial.replace('{serial}', deleteTarget.serial_number)
                    : t.admin.confirmDelete}
                </p>
                {deleteTarget ? (
                  <div className="rounded-md border border-dashed bg-muted/20 p-3 text-xs text-foreground">
                    <div className="font-mono font-medium">{deleteTarget.serial_number}</div>
                    <div className="mt-1 flex flex-wrap items-center gap-2 text-muted-foreground">
                      <span>{deleteTarget.product?.name || deleteTarget.product_code}</span>
                      <span>{deleteTarget.order?.order_no || '-'}</span>
                      <span>
                        {t.admin.viewCount}: {deleteTarget.view_count}
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
              onClick={() => {
                if (deleteTarget) {
                  deleteMutation.mutate(deleteTarget.id)
                }
              }}
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
