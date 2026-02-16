import { Skeleton } from '@/components/ui/page-loading'

export default function Loading() {
  return (
    <div className="pb-8 space-y-4">
      {/* 返回按钮和标题 */}
      <div className="flex items-center gap-3">
        <Skeleton className="h-9 w-24" />
        <Skeleton className="h-6 w-64" />
      </div>

      {/* 移动端图片骨架 */}
      <div className="md:hidden">
        <Skeleton className="aspect-square w-full rounded-lg" />
        <div className="grid grid-cols-5 gap-2 mt-3">
          {Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} className="aspect-square rounded" />
          ))}
        </div>
      </div>

      <div className="flex gap-6 lg:gap-8">
        {/* 左侧图片骨架（桌面端） */}
        <div className="hidden md:block w-[400px] shrink-0 space-y-3">
          <Skeleton className="aspect-square rounded-lg" />
          <div className="grid grid-cols-4 gap-2">
            {Array.from({ length: 4 }).map((_, i) => (
              <Skeleton key={i} className="aspect-square rounded" />
            ))}
          </div>
        </div>

        {/* 右侧信息骨架 */}
        <div className="flex-1 space-y-4">
          <div className="space-y-3">
            <Skeleton className="h-6 w-20" />
            <Skeleton className="h-4 w-full" />
          </div>

          <div className="flex gap-6">
            <Skeleton className="h-4 w-24" />
            <Skeleton className="h-4 w-24" />
          </div>

          <Skeleton className="h-px w-full" />

          <div className="space-y-2">
            <Skeleton className="h-10 w-40" />
            <Skeleton className="h-4 w-32" />
          </div>

          <Skeleton className="h-px w-full" />

          <div className="space-y-2">
            <Skeleton className="h-4 w-48" />
            <Skeleton className="h-4 w-36" />
            <Skeleton className="h-4 w-40" />
          </div>

          <Skeleton className="h-px w-full" />

          <Skeleton className="h-12 w-full" />
        </div>
      </div>
    </div>
  )
}
