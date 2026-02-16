'use client'

import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { DataTable } from '@/components/admin/data-table'
import { Package, Eye, ShoppingBag, RefreshCw, Search, Trash2 } from 'lucide-react'
import { formatDate } from '@/lib/utils'
import toast from 'react-hot-toast'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'

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
    const token = localStorage.getItem('auth_token')
    const response = await fetch(`${API_BASE_URL}/api/admin/serials?${params}`, {
        headers: {
            'Authorization': `Bearer ${token}`,
        },
    })

    if (!response.ok) {
        throw new Error('Failed to fetch serials')
    }

    return response.json()
}

async function getStatistics() {
    const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
    const token = localStorage.getItem('auth_token')
    const response = await fetch(`${API_BASE_URL}/api/admin/serials/statistics`, {
        headers: {
            'Authorization': `Bearer ${token}`,
        },
    })

    if (!response.ok) {
        throw new Error('Failed to fetch statistics')
    }

    return response.json()
}

async function deleteSerial(id: number) {
    const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
    const token = localStorage.getItem('auth_token')
    const response = await fetch(`${API_BASE_URL}/api/admin/serials/${id}`, {
        method: 'DELETE',
        headers: {
            'Authorization': `Bearer ${token}`,
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
        },
        onError: () => {
            toast.error(t.admin.serialDeleteFailed)
        },
    })

    const handleDelete = async (id: number, serialNumber: string) => {
        if (confirm(t.admin.confirmDeleteSerial.replace('{serial}', serialNumber))) {
            deleteMutation.mutate(id)
        }
    }

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
                        <div className="font-mono font-bold text-lg">{serial.serial_number}</div>
                        <div className="text-xs text-muted-foreground mt-1">
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
                        <div className="flex items-center gap-1 justify-center">
                            <Eye className="w-4 h-4" />
                            <span className="font-bold">{serial.view_count}</span>
                        </div>
                        {serial.first_viewed_at && (
                            <div className="text-xs text-muted-foreground mt-1">
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
                        onClick={() => handleDelete(serial.id, serial.serial_number)}
                        className="text-red-600 hover:text-red-700 hover:bg-red-50 dark:text-red-400 dark:hover:text-red-300 dark:hover:bg-red-950/30"
                    >
                        <Trash2 className="w-4 h-4" />
                    </Button>
                )
            },
        },
    ]

    const stats = statsData?.data || {}

    return (
        <div className="space-y-6">
            <div className="flex items-center justify-between">
                <h1 className="text-3xl font-bold">{t.admin.serialManagement}</h1>
                <Button variant="outline" size="sm" onClick={() => refetch()}>
                    <RefreshCw className="mr-2 h-4 w-4" />
                    {t.admin.refresh}
                </Button>
            </div>

            {/* Statistics Cards */}
            <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
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
                            {t.admin.viewRate}: {stats.total_count > 0
                                ? ((stats.viewed_count / stats.total_count) * 100).toFixed(1)
                                : 0}%
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
                            {t.admin.avgPerSerial.replace('{avg}', stats.viewed_count > 0
                                ? (stats.total_views / stats.viewed_count).toFixed(1)
                                : '0')}
                        </p>
                    </CardContent>
                </Card>
            </div>

            {/* Filters */}
            <Card>
                <CardHeader>
                    <CardTitle className="text-base">{t.admin.searchFilter}</CardTitle>
                </CardHeader>
                <CardContent>
                    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
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
                    <div className="flex gap-2 mt-4">
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
                data={data?.data?.items || []}
                isLoading={isLoading}
                pagination={{
                    page,
                    total_pages: data?.data?.pagination?.total_pages || 1,
                    onPageChange: setPage,
                }}
            />
        </div>
    )
}
