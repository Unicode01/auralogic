'use client'

import { Skeleton } from '@/components/ui/page-loading'
import { useIsMobile } from '@/hooks/use-mobile'

export default function Loading() {
  const { isMobile, mounted } = useIsMobile()
  const isCompactLayout = mounted ? isMobile : false

  return (
    <div className="space-y-6">
      {/* 标题和刷新按钮 */}
      <div className="flex items-center justify-between">
        <Skeleton className="h-9 w-32" />
        <Skeleton className="h-9 w-20" />
      </div>

      {/* 筛选栏 */}
      <div className={isCompactLayout ? 'flex flex-col gap-3' : 'flex flex-col gap-3 md:flex-row'}>
        <Skeleton className="h-10 flex-1" />
        <Skeleton className={isCompactLayout ? 'h-10 w-full' : 'h-10 w-full md:w-40'} />
      </div>

      {/* 订单列表 */}
      <div className="space-y-4">
        {Array.from({ length: 5 }).map((_, i) => (
          <div key={i} className="rounded-lg border bg-card p-4 space-y-3">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-3">
                <Skeleton className="h-5 w-32" />
                <Skeleton className="h-5 w-16" />
              </div>
              <Skeleton className="h-5 w-24" />
            </div>
            <div className="flex items-center gap-4">
              <Skeleton className="h-16 w-16 rounded" />
              <div className="flex-1 space-y-2">
                <Skeleton className="h-4 w-48" />
                <Skeleton className="h-4 w-32" />
              </div>
              <Skeleton className="h-6 w-20" />
            </div>
          </div>
        ))}
      </div>

      {/* 分页 */}
      <div className="flex justify-center gap-2">
        <Skeleton className="h-9 w-9" />
        <Skeleton className="h-9 w-9" />
        <Skeleton className="h-9 w-9" />
      </div>
    </div>
  )
}
