'use client'

import { useAuth } from '@/hooks/use-auth'
import { useIsMobile } from '@/hooks/use-mobile'
import { UserSidebar } from '@/components/layout/user-sidebar'
import { MobileBottomNav } from '@/components/layout/mobile-bottom-nav'
import { Skeleton } from '@/components/ui/page-loading'

function SerialVerifyLoadingContent() {
  const { isAuthenticated } = useAuth()
  const { isMobile } = useIsMobile()

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
    return <div className="px-4 py-10 md:px-6 md:py-12">{content}</div>
  }

  return (
    <div className="sidebar-layout flex h-screen">
      {!isMobile ? <UserSidebar /> : null}
      <main className={`flex-1 overflow-y-auto p-4 md:p-8 ${isMobile ? 'pb-20' : ''}`}>
        {content}
      </main>
      {isMobile ? <MobileBottomNav /> : null}
    </div>
  )
}

export default function Loading() {
  return <SerialVerifyLoadingContent />
}
