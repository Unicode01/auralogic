'use client'

import { useState } from 'react'
import { useQuery, useMutation } from '@tanstack/react-query'
import { getAdminProducts, deleteProduct, toggleProductFeatured, updateProductStatus } from '@/lib/api'
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
  Database
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
import { Product, ProductStatus } from '@/types/product'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'

export default function AdminProductsPage() {
  const [page, setPage] = useState(1)
  const [status, setStatus] = useState<string | undefined>()
  const [category, setCategory] = useState<string | undefined>()
  const [search, setSearch] = useState('')
  const [deleteId, setDeleteId] = useState<number | null>(null)
  const { currency } = useCurrency()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.adminProducts)

  const statusConfig = {
    draft: { label: t.admin.draft, color: 'bg-muted text-muted-foreground' },
    active: { label: t.admin.onSale, color: 'bg-green-500/20 text-green-700 dark:text-green-400' },
    inactive: { label: t.admin.offSale, color: 'bg-yellow-500/20 text-yellow-700 dark:text-yellow-400' },
    out_of_stock: { label: t.admin.outOfStock, color: 'bg-red-500/20 text-red-700 dark:text-red-400' },
  }

  const { data, isLoading, refetch } = useQuery({
    queryKey: ['adminProducts', page, status, category, search],
    queryFn: () => getAdminProducts({
      page,
      limit: 20,
      status: status === 'all' ? undefined : status,
      category: category === 'all' ? undefined : category,
      search: search || undefined,
    }),
  })

  // 删除商品
  const deleteMutation = useMutation({
    mutationFn: deleteProduct,
    onSuccess: () => {
      toast.success(t.admin.productDeleted)
      refetch()
      setDeleteId(null)
    },
    onError: (error: Error) => {
      toast.error(`${t.admin.deleteFailed}: ${error.message}`)
    },
  })

  // 切换精选状态
  const toggleFeaturedMutation = useMutation({
    mutationFn: toggleProductFeatured,
    onSuccess: () => {
      toast.success(t.admin.featuredUpdated)
      refetch()
    },
    onError: (error: Error) => {
      toast.error(`${t.admin.operationFailed}: ${error.message}`)
    },
  })

  // 更新商品状态
  const updateStatusMutation = useMutation({
    mutationFn: ({ id, status }: { id: number; status: string }) =>
      updateProductStatus(id, status),
    onSuccess: () => {
      toast.success(t.admin.statusUpdated)
      refetch()
    },
    onError: (error: Error) => {
      toast.error(`${t.admin.operationFailed}: ${error.message}`)
    },
  })

  const columns = [
    {
      header: t.admin.productInfo,
      cell: ({ row }: { row: { original: Product } }) => {
        const product = row.original
        const primaryImage = product.images?.find(img => img.is_primary || img.isPrimary)?.url
        return (
          <div className="flex items-center gap-3">
            {primaryImage ? (
              <img
                src={primaryImage}
                alt={product.name}
                className="w-12 h-12 object-cover rounded"
              />
            ) : (
              <div className="w-12 h-12 bg-muted rounded flex items-center justify-center">
                <Package className="w-6 h-6 text-muted-foreground" />
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
              <div className="text-xs text-muted-foreground mt-1">
                {product.attributes.length}{t.admin.specs}
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
            <div className="font-medium">{formatPrice(product.price, currency)}</div>
            {product.original_price && product.original_price > product.price && (
              <div className="text-sm text-muted-foreground line-through">
                {formatPrice(product.original_price, currency)}
              </div>
            )}
          </div>
        )
      },
    },
    {
      header: t.admin.status,
      cell: ({ row }: { row: { original: Product } }) => {
        const status = row.original.status
        const config = statusConfig[status]
        return (
          <Badge className={config.color}>
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
              <Eye className="w-3 h-3" />
              {product.view_count || 0}
            </div>
            <div className="flex items-center gap-1">
              <ShoppingBag className="w-3 h-3" />
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
        return (
          <div className="flex items-center gap-2">
            <Button
              size="sm"
              variant="ghost"
              onClick={() => toggleFeaturedMutation.mutate(product.id)}
              title={isFeatured ? t.admin.cancelFeatured : t.admin.setFeatured}
            >
              <Star
                className={`w-4 h-4 ${isFeatured ? 'fill-yellow-500 text-yellow-500' : ''}`}
              />
            </Button>
            {isVirtual && (
              <Button asChild size="sm" variant="outline" title={t.admin.virtualStockManage}>
                <Link href={`/admin/products/${product.id}/virtual-stock`}>
                  <Database className="w-4 h-4" />
                </Link>
              </Button>
            )}
            <Button asChild size="sm" variant="outline">
              <Link href={`/admin/products/${product.id}`}>
                <Pencil className="w-4 h-4" />
              </Link>
            </Button>
            <Button
              size="sm"
              variant="destructive"
              onClick={() => setDeleteId(product.id)}
            >
              <Trash2 className="w-4 h-4" />
            </Button>
          </div>
        )
      },
    },
  ]

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-bold">{t.admin.productManagement}</h1>
        <div className="flex gap-2">
          <Button asChild>
            <Link href="/admin/products/new">
              <Plus className="mr-2 h-4 w-4" />
              {t.admin.addProduct}
            </Link>
          </Button>
          <Button variant="outline" size="sm" onClick={() => refetch()}>
            <RefreshCw className="mr-2 h-4 w-4" />
            {t.admin.refresh}
          </Button>
        </div>
      </div>

      {/* 筛选器 */}
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

      <DataTable
        columns={columns}
        data={data?.data?.items || []}
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
              {t.admin.deleteProductConfirm}
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
