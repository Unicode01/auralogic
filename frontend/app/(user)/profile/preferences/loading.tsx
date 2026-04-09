'use client'

import { Skeleton } from '@/components/ui/page-loading'
import { useIsMobile } from '@/hooks/use-mobile'

export default function Loading() {
  const { isMobile, mounted } = useIsMobile()
  const isCompactLayout = mounted ? isMobile : false

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        {isCompactLayout ? <Skeleton className="h-9 w-9" /> : null}
        <Skeleton className="h-8 w-40" />
      </div>

      <div className="rounded-lg border bg-card p-6">
        <div className="flex flex-wrap gap-2">
          <Skeleton className="h-6 w-40 rounded-full" />
          <Skeleton className="h-6 w-36 rounded-full" />
        </div>
      </div>

      <div className={isCompactLayout ? 'grid gap-6' : 'grid gap-6 xl:grid-cols-[1fr_1.4fr]'}>
        <div className="rounded-lg border bg-card">
          <div className="space-y-1.5 p-6">
            <Skeleton className="h-6 w-40" />
            <Skeleton className="h-4 w-60" />
          </div>
          <div className="space-y-6 p-6 pt-0">
            <div className="space-y-3">
              <Skeleton className="h-4 w-20" />
              <div className="grid grid-cols-2 gap-2">
                <Skeleton className="h-10 w-full" />
                <Skeleton className="h-10 w-full" />
              </div>
            </div>
            <div className="space-y-2">
              {Array.from({ length: 3 }).map((_, i) => (
                <Skeleton key={i} className="h-16 w-full" />
              ))}
            </div>
          </div>
        </div>

        <div className="rounded-lg border bg-card">
          <div className="space-y-1.5 p-6">
            <Skeleton className="h-6 w-44" />
            <Skeleton className="h-4 w-72" />
          </div>
          <div className="space-y-4 p-6 pt-0">
            <Skeleton className="h-40 w-full" />
            <Skeleton className="h-24 w-full" />
            <div className="flex justify-end">
              <Skeleton className="h-10 w-40" />
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
