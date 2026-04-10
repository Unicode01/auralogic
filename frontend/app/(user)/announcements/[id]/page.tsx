import { HydrationBoundary, dehydrate } from '@tanstack/react-query'
import AnnouncementDetailClient from './announcement-detail-client'
import { getAnnouncementQueryOptions } from '@/lib/content-detail-queries'
import { getServerAnnouncement, getServerAuthToken } from '@/lib/server-api'
import { createServerQueryClient } from '@/lib/server-query-client'

function isPositiveInteger(value: number): boolean {
  return Number.isFinite(value) && value > 0
}

export default async function AnnouncementDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params
  const announcementId = Number(id)
  const queryClient = createServerQueryClient()

  if ((await getServerAuthToken()) && isPositiveInteger(announcementId)) {
    try {
      await queryClient.prefetchQuery({
        ...getAnnouncementQueryOptions(announcementId),
        queryFn: () => getServerAnnouncement(announcementId),
      })
    } catch {
      // Preserve the existing client-side loading and error handling.
    }
  }

  return (
    <HydrationBoundary state={dehydrate(queryClient)}>
      <AnnouncementDetailClient announcementId={announcementId} />
    </HydrationBoundary>
  )
}
