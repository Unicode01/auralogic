'use client'

import { useAuth } from '@/hooks/use-auth'
import { useResponsiveLayout } from '@/hooks/use-mobile'
import { UserSidebar } from '@/components/layout/user-sidebar'
import { MobileBottomNav } from '@/components/layout/mobile-bottom-nav'
import { Skeleton } from '@/components/ui/page-loading'
import { cn } from '@/lib/utils'

function SerialVerifyLoadingContent() {
  const { isAuthenticated } = useAuth()
  const { isPhone, isTablet } = useResponsiveLayout()

  const content = (
    <div className="mx-auto max-w-2xl space-y-6 py-2">
      <div className="flex justify-end">
        <Skeleton className="h-9 w-20" />
      </div>

      <div className="space-y-2 text-center">
        <Skeleton className="mx-auto h-8 w-48" />
        <Skeleton className="mx-auto h-4 w-64" />
      </div>

      <div className="rounded-lg border bg-card p-6 space-y-4">
        <div className="space-y-2">
          <Skeleton className="h-5 w-28" />
          <Skeleton className="h-10 w-full" />
        </div>
        <Skeleton className="h-10 w-28" />
        <Skeleton className="h-4 w-full" />
      </div>
    </div>
  )

  if (!isAuthenticated) {
    return (
      <div
        className={cn(
          'px-4',
          isPhone ? 'py-10' : isTablet ? 'py-8' : 'px-6 py-12'
        )}
      >
        {content}
      </div>
    )
  }

  return (
    <div className="sidebar-layout flex h-screen">
      {!isPhone ? <UserSidebar compact={isTablet} /> : null}
      <main
        className={cn(
          'flex-1 overflow-y-auto',
          isPhone ? 'p-4 pb-20' : isTablet ? 'bg-muted/10 p-4' : 'p-6 xl:p-8'
        )}
      >
        <div className={cn('w-full', isTablet && 'mx-auto max-w-[39rem]')}>{content}</div>
      </main>
      {isPhone ? <MobileBottomNav /> : null}
    </div>
  )
}

export default function Loading() {
  return <SerialVerifyLoadingContent />
}
