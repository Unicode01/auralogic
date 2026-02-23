'use client'

import { useState } from 'react'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Button } from '@/components/ui/button'
import { ChevronLeft, ChevronRight } from 'lucide-react'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'

interface DataTableProps<T> {
  columns: {
    header: string | (() => React.ReactNode)
    accessorKey?: string
    cell?: ({ row }: { row: { original: T } }) => React.ReactNode
  }[]
  data: T[]
  isLoading?: boolean
  pagination?: {
    page: number
    total_pages: number
    onPageChange: (page: number) => void
  }
}

export function DataTable<T>({
  columns,
  data,
  isLoading,
  pagination,
}: DataTableProps<T>) {
  const { locale } = useLocale()
  const t = getTranslations(locale)
  const [pageInput, setPageInput] = useState('')
  // 确保 data 是数组
  const safeData = Array.isArray(data) ? data : []

  if (isLoading) {
    return <div className="text-center py-8">{t.common.loading}</div>
  }

  return (
    <div className="space-y-4">
      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              {columns.map((column, index) => (
                <TableHead key={index}>{typeof column.header === 'function' ? column.header() : column.header}</TableHead>
              ))}
            </TableRow>
          </TableHeader>

          <TableBody>
            {safeData.length === 0 ? (
              <TableRow>
                <TableCell
                  colSpan={columns.length}
                  className="text-center py-8 text-muted-foreground"
                >
                  {t.common.noData}
                </TableCell>
              </TableRow>
            ) : (
              safeData.map((row, rowIndex) => (
                <TableRow key={rowIndex}>
                  {columns.map((column, colIndex) => (
                    <TableCell key={colIndex}>
                      {column.cell
                        ? column.cell({ row: { original: row } })
                        : column.accessorKey
                        ? (row as any)[column.accessorKey]
                        : null}
                    </TableCell>
                  ))}
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>

      {/* 分页 */}
      {pagination && (
        <div className="flex items-center justify-between">
          <p className="text-sm text-muted-foreground">
            {t.common.pageInfo
              .replace('{page}', String(pagination.page))
              .replace('{totalPages}', String(pagination.total_pages))}
          </p>

          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              type="button"
              onClick={() => pagination.onPageChange(pagination.page - 1)}
              disabled={pagination.page === 1}
            >
              <ChevronLeft className="h-4 w-4" />
              {t.common.prevPage}
            </Button>

            <input
              type="number"
              min={1}
              max={pagination.total_pages}
              value={pageInput || pagination.page}
              onChange={(e) => setPageInput(e.target.value)}
              onBlur={() => {
                const p = parseInt(pageInput)
                if (p >= 1 && p <= pagination.total_pages && p !== pagination.page) {
                  pagination.onPageChange(p)
                }
                setPageInput('')
              }}
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  // Prevent nested forms from being submitted when DataTable is rendered inside a <form>.
                  e.preventDefault()
                  e.stopPropagation()
                  const p = parseInt(pageInput)
                  if (p >= 1 && p <= pagination.total_pages && p !== pagination.page) {
                    pagination.onPageChange(p)
                  }
                  setPageInput('')
                  ;(e.target as HTMLInputElement).blur()
                }
              }}
              className="w-12 h-8 text-center text-sm border rounded-md bg-background focus:outline-none focus:ring-2 focus:ring-ring [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
            />

            <Button
              variant="outline"
              size="sm"
              type="button"
              onClick={() => pagination.onPageChange(pagination.page + 1)}
              disabled={pagination.page === pagination.total_pages}
            >
              {t.common.nextPage}
              <ChevronRight className="h-4 w-4" />
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}

