'use client'

import { useState } from 'react'
import { useQuery, useMutation } from '@tanstack/react-query'
import { getAdminPromoCodes, deletePromoCode } from '@/lib/api'
import { DataTable } from '@/components/admin/data-table'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  Plus,
  RefreshCw,
  Pencil,
  Trash2,
} from 'lucide-react'
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

interface PromoCode {
  id: number
  code: string
  name: string
  discount_type: 'percentage' | 'fixed'
  discount_value: number
  total_quantity: number
  used_quantity: number
  reserved_quantity: number
  status: string
  expires_at: string | null
}

export default function AdminPromoCodesPage() {
  const [page, setPage] = useState(1)
  const [status, setStatus] = useState<string | undefined>()
  const [search, setSearch] = useState('')
  const [deleteId, setDeleteId] = useState<number | null>(null)
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.adminPromoCodes)

  const statusConfig: Record<string, { label: string; color: string }> = {
    active: { label: t.promoCode.active, color: 'bg-green-500/20 text-green-700 dark:text-green-400' },
    inactive: { label: t.promoCode.inactive, color: 'bg-gray-500/20 text-gray-700 dark:text-gray-400' },
    expired: { label: t.promoCode.expired, color: 'bg-red-500/20 text-red-700 dark:text-red-400' },
  }

  const { data, isLoading, refetch } = useQuery({
    queryKey: ['adminPromoCodes', page, status, search],
    queryFn: () => getAdminPromoCodes({
      page,
      limit: 20,
      status: status === 'all' ? undefined : status,
      search: search || undefined,
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
    onError: (error: Error) => {
      toast.error(`${t.promoCode.deleteFailed}: ${error.message}`)
    },
  })

  const columns = [
    {
      header: t.promoCode.code,
      cell: ({ row }: { row: { original: PromoCode } }) => {
        const promo = row.original
        return (
          <div className="font-mono font-medium">{promo.code}</div>
        )
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
              ? `${promo.discount_value}%`
              : `\u00a5${promo.discount_value}`}
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
            <div className="font-semibold tabular-nums">
              {promo.used_quantity}
            </div>
            <div className="text-xs text-muted-foreground tabular-nums">
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
        return (
          <Badge className={config.color}>
            {config.label}
          </Badge>
        )
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
        return (
          <div className="flex items-center gap-2">
            <Button asChild size="sm" variant="outline">
              <Link href={`/admin/promo-codes/${promo.id}`}>
                <Pencil className="w-4 h-4" />
              </Link>
            </Button>
            <Button
              size="sm"
              variant="destructive"
              onClick={() => setDeleteId(promo.id)}
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
        <h1 className="text-3xl font-bold">{t.promoCode.promoCodeManagement}</h1>
        <div className="flex gap-2">
          <Button asChild>
            <Link href="/admin/promo-codes/new">
              <Plus className="mr-2 h-4 w-4" />
              {t.promoCode.addPromoCode}
            </Link>
          </Button>
          <Button variant="outline" onClick={() => refetch()}>
            <RefreshCw className="mr-2 h-4 w-4" />
            {t.admin.refresh}
          </Button>
        </div>
      </div>

      {/* 筛选器 */}
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
              {t.promoCode.confirmDelete}
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
