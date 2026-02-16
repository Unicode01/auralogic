import { Skeleton, TableSkeleton } from '@/components/ui/page-loading'

export default function Loading() {
  return (
    <div className="space-y-6">
      {/* 标题 */}
      <Skeleton className="h-8 w-32" />

      {/* 搜索和筛选 */}
      <div className="flex flex-col md:flex-row gap-3">
        <Skeleton className="h-10 flex-1" />
        <Skeleton className="h-10 w-40" />
      </div>

      {/* 表格 */}
      <TableSkeleton rows={10} cols={5} />
    </div>
  )
}
