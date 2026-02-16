import { ProductGridSkeleton } from '@/components/ui/page-loading'

export default function Loading() {
  return (
    <div className="space-y-6">
      {/* 标题骨架 */}
      <div className="space-y-2">
        <div className="h-8 w-48 bg-muted rounded animate-pulse" />
        <div className="h-4 w-64 bg-muted rounded animate-pulse" />
      </div>

      {/* 搜索栏骨架 */}
      <div className="flex flex-col md:flex-row gap-3">
        <div className="flex gap-2 flex-1">
          <div className="h-10 flex-1 bg-muted rounded animate-pulse" />
          <div className="h-10 w-20 bg-muted rounded animate-pulse" />
        </div>
        <div className="h-10 w-full md:w-48 bg-muted rounded animate-pulse" />
      </div>

      {/* 商品网格骨架 */}
      <ProductGridSkeleton count={8} />
    </div>
  )
}
