import { Skeleton } from '@/components/ui/page-loading'

export default function Loading() {
  return (
    <div className="space-y-6">
      {/* 标题 */}
      <div className="flex items-center justify-between">
        <Skeleton className="h-8 w-32" />
        <Skeleton className="h-10 w-20 hidden md:block" />
      </div>

      {/* 基本信息卡片 */}
      <div className="rounded-lg border bg-card">
        <div className="p-6 space-y-1.5">
          <Skeleton className="h-6 w-24" />
          <Skeleton className="h-4 w-40" />
        </div>
        <div className="p-6 pt-0 space-y-6">
          {Array.from({ length: 4 }).map((_, i) => (
            <div key={i} className="flex items-start gap-3">
              <Skeleton className="h-5 w-5 mt-0.5" />
              <div className="flex-1 space-y-1">
                <Skeleton className="h-4 w-16" />
                <Skeleton className="h-5 w-32" />
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* 快捷操作卡片 */}
      <div className="rounded-lg border bg-card">
        <div className="p-6 space-y-1.5">
          <Skeleton className="h-6 w-24" />
        </div>
        <div className="divide-y">
          {Array.from({ length: 5 }).map((_, i) => (
            <div key={i} className="flex items-center justify-between p-4">
              <div className="flex items-center gap-3">
                <Skeleton className="h-5 w-5" />
                <Skeleton className="h-5 w-24" />
              </div>
              <Skeleton className="h-5 w-5" />
            </div>
          ))}
        </div>
      </div>

      {/* 退出按钮 */}
      <div className="rounded-lg border bg-card">
        <div className="flex items-center gap-3 p-4">
          <Skeleton className="h-5 w-5" />
          <Skeleton className="h-5 w-20" />
        </div>
      </div>
    </div>
  )
}
