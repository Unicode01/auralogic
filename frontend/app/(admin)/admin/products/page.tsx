'use client'
/* eslint-disable @next/next/no-img-element */

import { useDeferredValue, useRef, useState } from 'react'
import { useQuery, useMutation } from '@tanstack/react-query'
import {
  getAdminProducts,
  deleteProduct,
  toggleProductFeatured,
  updateProductStatus,
} from '@/lib/api'
import { DataTable } from '@/components/admin/data-table'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  Plus,
  RefreshCw,
  Pencil,
  Trash2,
  Star,
  Eye,
  ShoppingBag,
  Package,
  Database,
  Download,
  Upload,
} from 'lucide-react'
import Link from 'next/link'
import toast from 'react-hot-toast'
import { Input } from '@/components/ui/input'
import { useCurrency, formatPrice } from '@/contexts/currency-context'
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
import { Product } from '@/types/product'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'
import { resolveApiErrorMessage } from '@/lib/api-error'
import { resolveClientAPIProxyURL } from '@/lib/api-base-url'
import { PluginExtensionList } from '@/components/plugins/plugin-extension-list'
import { PluginSlot } from '@/components/plugins/plugin-slot'
import { usePluginExtensionBatch } from '@/lib/plugin-extension-batch'

function buildAdminProductRowSummary(product: Product) {
  return {
    id: product.id,
    name: product.name,
    sku: product.sku,
    category: product.category,
    status: product.status,
    product_type: product.product_type || product.productType,
    is_featured: Boolean(product.is_featured || product.isFeatured),
    price_minor: product.price_minor,
    original_price_minor: product.original_price_minor,
    stock: product.stock,
    view_count: product.view_count || product.viewCount || 0,
    sale_count: product.sale_count || product.saleCount || 0,
    created_at: product.created_at || product.createdAt,
    updated_at: product.updated_at || product.updatedAt,
  }
}

type ProductImportResult = {
  message?: string
  error_count?: number
}

function normalizeProductImportResult(result: unknown): ProductImportResult | null {
  if (!result || typeof result !== 'object') {
    return null
  }

  const payload = result as Record<string, unknown>
  return {
    message: typeof payload.message === 'string' ? payload.message : undefined,
    error_count: Number(payload.error_count || 0),
  }
}

export default function AdminProductsPage() {
  const fileInputRef = useRef<HTMLInputElement | null>(null)
  const [page, setPage] = useState(1)
  const [status, setStatus] = useState<string | undefined>()
  const [category, setCategory] = useState<string | undefined>()
  const [search, setSearch] = useState('')
  const [deleteId, setDeleteId] = useState<number | null>(null)
  const deferredSearch = useDeferredValue(search)
  const { currency } = useCurrency()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.adminProducts)

  const formatAdminError = (error: unknown, fallback: string) => {
    const detail = resolveApiErrorMessage(error, t, fallback)
    return detail === fallback ? fallback : `${fallback}: ${detail}`
  }

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
      // Ignore parse errors and fall back to the provided message.
    }
    return fallback
  }

  const statusConfig: Record<string, { label: string; color: string }> = {
    draft: { label: t.admin.draft, color: 'bg-muted text-muted-foreground' },
    active: { label: t.admin.onSale, color: 'bg-green-500/20 text-green-700 dark:text-green-400' },
    inactive: {
      label: t.admin.offSale,
      color: 'bg-yellow-500/20 text-yellow-700 dark:text-yellow-400',
    },
    out_of_stock: {
      label: t.admin.outOfStock,
      color: 'bg-red-500/20 text-red-700 dark:text-red-400',
    },
  }

  const defaultStatusConfig = { label: '-', color: 'bg-muted text-muted-foreground' }

  const { data, isLoading, refetch } = useQuery({
    queryKey: ['adminProducts', page, status, category, deferredSearch],
    queryFn: () =>
      getAdminProducts({
        page,
        limit: 20,
        status: status === 'all' ? undefined : status,
        category: category === 'all' ? undefined : category,
        search: deferredSearch || undefined,
      }),
  })

  const deleteMutation = useMutation({
    mutationFn: deleteProduct,
    onSuccess: () => {
      toast.success(t.admin.productDeleted)
      refetch()
      setDeleteId(null)
    },
    onError: (error: unknown) => {
      toast.error(formatAdminError(error, t.admin.deleteFailed))
    },
  })

  const toggleFeaturedMutation = useMutation({
    mutationFn: toggleProductFeatured,
    onSuccess: () => {
      toast.success(t.admin.featuredUpdated)
      refetch()
    },
    onError: (error: unknown) => {
      toast.error(formatAdminError(error, t.admin.operationFailed))
    },
  })

  const updateStatusMutation = useMutation({
    mutationFn: ({ id, status }: { id: number; status: string }) => updateProductStatus(id, status),
    onSuccess: () => {
      toast.success(t.admin.statusUpdated)
      refetch()
    },
    onError: (error: unknown) => {
      toast.error(formatAdminError(error, t.admin.operationFailed))
    },
  })

  const importMutation = useMutation({
    mutationFn: async (file: File) => {
      const formData = new FormData()
      formData.append('file', file)
      formData.append('conflict_mode', 'upsert')

      const response = await fetch(resolveClientAPIProxyURL('/api/admin/products/import'), {
        method: 'POST',
        body: formData,
      })

      if (!response.ok) {
        throw new Error(await readFetchErrorMessage(response, t.admin.importFailed))
      }

      return (await response.json()) as { data?: unknown }
    },
    onSuccess: (data) => {
      toast.dismiss()
      const result = normalizeProductImportResult(data?.data)
      if (result?.error_count) {
        toast.error(result.message || t.admin.importFailed, { duration: 6000 })
      } else {
        toast.success(result?.message || t.admin.productsImportSuccess)
      }
      refetch()
    },
    onError: (error: unknown) => {
      toast.dismiss()
      toast.error(resolveApiErrorMessage(error, t, t.admin.importFailed))
    },
  })

  const products = data?.data?.items || []
  const totalProducts = Number(data?.data?.pagination?.total || 0)
  const productFilterBadges = [
    deferredSearch.trim() ? `${t.common.search}: ${deferredSearch.trim()}` : null,
    status ? `${t.admin.status}: ${(statusConfig[status] || defaultStatusConfig).label}` : null,
    category ? `${t.admin.category}: ${category}` : null,
  ].filter(Boolean) as string[]
  const adminProductsFilters = {
    page,
    status: status === 'all' ? undefined : status,
    category: category === 'all' ? undefined : category,
    search: deferredSearch || undefined,
  }
  const adminProductsPagination = {
    page,
    total_pages: data?.data?.pagination?.total_pages,
    total: totalProducts,
    limit: data?.data?.pagination?.limit,
  }
  const adminProductsPluginContext = {
    view: 'admin_products',
    filters: adminProductsFilters,
    pagination: adminProductsPagination,
    summary: {
      total: totalProducts,
      current_page_count: products.length,
      active_filter_count: productFilterBadges.length,
    },
    active_filter_badges: productFilterBadges,
  }
  const adminProductRowActionItems = products.map((item: Product, index: number) => ({
    key: String(item.id),
    slot: 'admin.products.row_actions',
    path: '/admin/products',
    hostContext: {
      view: 'admin_products_row',
      product: buildAdminProductRowSummary(item),
      row: {
        index: index + 1,
        absolute_index: (page - 1) * 20 + index + 1,
      },
      filters: adminProductsFilters,
      pagination: adminProductsPagination,
      summary: {
        current_page_count: products.length,
        total: totalProducts,
      },
    },
  }))
  const adminProductRowActionExtensions = usePluginExtensionBatch({
    scope: 'admin',
    path: '/admin/products',
    items: adminProductRowActionItems,
    enabled: products.length > 0,
  })

  const handleExport = () => {
    const params = new URLSearchParams()
    if (status) params.append('status', status)
    if (category) params.append('category', category)
    if (deferredSearch.trim()) params.append('search', deferredSearch.trim())

    const url = resolveClientAPIProxyURL(
      `/api/admin/products/export${params.toString() ? `?${params.toString()}` : ''}`
    )

    fetch(url)
      .then(async (res) => {
        if (!res.ok) {
          throw new Error(await readFetchErrorMessage(res, t.admin.exportFailed))
        }
        return res.blob()
      })
      .then((blob) => {
        const blobUrl = window.URL.createObjectURL(blob)
        const anchor = document.createElement('a')
        anchor.href = blobUrl
        anchor.download = `products_${new Date().toISOString().slice(0, 10)}.xlsx`
        document.body.appendChild(anchor)
        anchor.click()
        document.body.removeChild(anchor)
        window.URL.revokeObjectURL(blobUrl)
        toast.success(t.admin.productsExportSuccess)
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
      header: t.admin.productInfo,
      cell: ({ row }: { row: { original: Product } }) => {
        const product = row.original
        const primaryImage = product.images?.find((img) => img.is_primary || img.isPrimary)?.url
        return (
          <div className="flex items-center gap-3">
            {primaryImage ? (
              <img
                src={primaryImage}
                alt={product.name}
                className="h-12 w-12 rounded object-cover"
              />
            ) : (
              <div className="flex h-12 w-12 items-center justify-center rounded bg-muted">
                <Package className="h-6 w-6 text-muted-foreground" />
              </div>
            )}
            <div>
              <div className="font-medium">{product.name}</div>
              <div className="text-sm text-muted-foreground">SKU: {product.sku}</div>
            </div>
          </div>
        )
      },
    },
    {
      header: t.admin.category,
      accessorKey: 'category',
      cell: ({ row }: { row: { original: Product } }) => {
        const product = row.original
        return (
          <div>
            <div>{product.category || '-'}</div>
            {product.attributes && product.attributes.length > 0 && (
              <div className="mt-1 text-xs text-muted-foreground">
                {product.attributes.length}
                {t.admin.specs}
              </div>
            )}
          </div>
        )
      },
    },
    {
      header: t.admin.price,
      cell: ({ row }: { row: { original: Product } }) => {
        const product = row.original
        return (
          <div>
            <div className="font-medium">{formatPrice(product.price_minor, currency)}</div>
            {product.original_price_minor > product.price_minor && (
              <div className="text-sm text-muted-foreground line-through">
                {formatPrice(product.original_price_minor, currency)}
              </div>
            )}
          </div>
        )
      },
    },
    {
      header: t.admin.status,
      cell: ({ row }: { row: { original: Product } }) => {
        const product = row.original
        const productStatus = product.status
        const config = statusConfig[productStatus] || defaultStatusConfig
        return (
          <Badge
            className={`${config.color} cursor-pointer transition-opacity hover:opacity-80`}
            onClick={() =>
              updateStatusMutation.mutate({
                id: product.id,
                status: productStatus === 'active' ? 'inactive' : 'active',
              })
            }
            title={productStatus === 'active' ? t.admin.takeOffSale : t.admin.putOnSale}
          >
            {config.label}
          </Badge>
        )
      },
    },
    {
      header: t.admin.statistics,
      cell: ({ row }: { row: { original: Product } }) => {
        const product = row.original
        return (
          <div className="text-sm">
            <div className="flex items-center gap-1">
              <Eye className="h-3 w-3" />
              {product.view_count || 0}
            </div>
            <div className="flex items-center gap-1">
              <ShoppingBag className="h-3 w-3" />
              {product.sale_count || 0}
            </div>
          </div>
        )
      },
    },
    {
      header: t.admin.actions,
      cell: ({ row }: { row: { original: Product } }) => {
        const product = row.original
        const isFeatured = product.is_featured || product.isFeatured
        const isVirtual = product.product_type === 'virtual'
        const rowExtensions = adminProductRowActionExtensions[String(product.id)] || []
        return (
          <div className="flex flex-wrap items-center gap-2">
            <Button
              size="sm"
              variant="ghost"
              onClick={() => toggleFeaturedMutation.mutate(product.id)}
              title={isFeatured ? t.admin.cancelFeatured : t.admin.setFeatured}
            >
              <Star className={`h-4 w-4 ${isFeatured ? 'fill-yellow-500 text-yellow-500' : ''}`} />
            </Button>
            {isVirtual && (
              <Button asChild size="sm" variant="outline" title={t.admin.virtualStockManage}>
                <Link href={`/admin/products/${product.id}/virtual-stock`}>
                  <Database className="h-4 w-4" />
                </Link>
              </Button>
            )}
            <Button asChild size="sm" variant="outline">
              <Link href={`/admin/products/${product.id}`}>
                <Pencil className="h-4 w-4" />
              </Link>
            </Button>
            <Button size="sm" variant="destructive" onClick={() => setDeleteId(product.id)}>
              <Trash2 className="h-4 w-4" />
            </Button>
            <PluginExtensionList extensions={rowExtensions} display="inline" />
          </div>
        )
      },
    },
  ]

  const deleteTarget = deleteId ? products.find((item: Product) => item.id === deleteId) : null

  return (
    <div className="space-y-6">
      <PluginSlot slot="admin.products.top" context={adminProductsPluginContext} />
      <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
        <div>
          <h1 className="text-3xl font-bold">{t.admin.productManagement}</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            {t.admin.totalRecords.replace('{count}', String(totalProducts))}
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button asChild>
            <Link href="/admin/products/new">
              <Plus className="mr-2 h-4 w-4" />
              {t.admin.addProduct}
            </Link>
          </Button>
          <input
            ref={fileInputRef}
            type="file"
            accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
            className="hidden"
            onChange={handleFileChange}
          />
          <Button variant="outline" onClick={handleImportClick} disabled={importMutation.isPending}>
            <Upload className="mr-2 h-4 w-4" />
            {t.admin.importProducts}
          </Button>
          <Button variant="outline" onClick={handleExport}>
            <Download className="mr-2 h-4 w-4" />
            {t.admin.exportProducts}
          </Button>
          <Button variant="outline" onClick={() => refetch()}>
            <RefreshCw className="mr-2 h-4 w-4" />
            {t.admin.refresh}
          </Button>
        </div>
      </div>

      <PluginSlot
        slot="admin.products.filters"
        context={{ ...adminProductsPluginContext, section: 'filters' }}
      />
      <div className="flex flex-col gap-3">
        <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
          <div className="flex gap-4">
            <Input
              placeholder={t.admin.searchProduct}
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
                <SelectValue placeholder={t.admin.status} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">{t.admin.allStatus}</SelectItem>
                <SelectItem value="draft">{t.admin.draft}</SelectItem>
                <SelectItem value="active">{t.admin.onSale}</SelectItem>
                <SelectItem value="inactive">{t.admin.offSale}</SelectItem>
                <SelectItem value="out_of_stock">{t.admin.outOfStock}</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
        {productFilterBadges.length > 0 ? (
          <p className="text-sm text-muted-foreground">{productFilterBadges.join(' · ')}</p>
        ) : null}
      </div>

      <DataTable
        columns={columns}
        data={products}
        isLoading={isLoading}
        pagination={{
          page,
          total_pages: data?.data?.pagination?.total_pages || 1,
          onPageChange: setPage,
        }}
      />

      <AlertDialog open={deleteId !== null} onOpenChange={() => setDeleteId(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t.admin.confirmDelete}</AlertDialogTitle>
            <AlertDialogDescription>
              <div className="space-y-3">
                <p>{t.admin.deleteProductConfirm}</p>
                {deleteTarget ? (
                  <div className="rounded-md border border-dashed bg-muted/20 p-3 text-xs text-foreground">
                    <div className="font-medium">{deleteTarget.name}</div>
                    <div className="mt-1 flex flex-wrap items-center gap-2 text-muted-foreground">
                      <span>SKU: {deleteTarget.sku}</span>
                      <span>{deleteTarget.category || '-'}</span>
                      <span>{formatPrice(deleteTarget.price_minor, currency)}</span>
                    </div>
                  </div>
                ) : null}
              </div>
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t.common.cancel}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => deleteId && deleteMutation.mutate(deleteId)}
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
