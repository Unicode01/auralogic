'use client'

import { ProductGridSkeleton } from '@/components/ui/page-loading'
import { useIsMobile } from '@/hooks/use-mobile'

export default function Loading() {
  const { isMobile, mounted } = useIsMobile()
  const isCompactLayout = mounted ? isMobile : false

  return (
    <div className="space-y-6">
      {/* 标题骨架 */}
      <div className="space-y-2">
        <div className="h-8 w-48 bg-muted rounded animate-pulse" />
        <div className="h-4 w-64 bg-muted rounded animate-pulse" />
      </div>

      {/* 搜索栏骨架 */}
      <div className={isCompactLayout ? 'flex flex-col gap-3' : 'flex flex-col gap-3 md:flex-row'}>
        <div className="flex gap-2 flex-1">
          <div className="h-10 flex-1 bg-muted rounded animate-pulse" />
          <div className="h-10 w-20 bg-muted rounded animate-pulse" />
        </div>
        <div
          className={
            isCompactLayout
              ? 'h-10 w-full rounded bg-muted animate-pulse'
              : 'h-10 w-full rounded bg-muted animate-pulse md:w-48'
          }
        />
      </div>

      {/* 商品网格骨架 */}
      <ProductGridSkeleton count={8} />
    </div>
  )
}
