'use client'

import { useEffect } from 'react'
import { useParams } from 'next/navigation'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { getAnnouncement, markAnnouncementAsRead } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import { ArrowLeft, AlertTriangle } from 'lucide-react'
import Link from 'next/link'
import { format } from 'date-fns'
import { zhCN } from 'date-fns/locale'
import { useLocale } from '@/hooks/use-locale'
import { usePageTitle } from '@/hooks/use-page-title'
import { getTranslations } from '@/lib/i18n'
import { MarkdownMessage } from '@/components/ui/markdown-message'

export default function AnnouncementDetailPage() {
  const params = useParams()
  const id = Number(params.id)
  const queryClient = useQueryClient()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.announcementDetail)

  const { data, isLoading } = useQuery({
    queryKey: ['announcement', id],
    queryFn: () => getAnnouncement(id),
    enabled: !!id,
  })

  const markReadMutation = useMutation({
    mutationFn: () => markAnnouncementAsRead(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['announcements'] })
      queryClient.invalidateQueries({ queryKey: ['announcement', id] })
    },
  })

  const announcement = data?.data

  useEffect(() => {
    if (announcement && !announcement.is_read) {
      markReadMutation.mutate()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [announcement?.id, announcement?.is_read])

  if (isLoading) {
    return <div className="text-center py-12 text-muted-foreground">{t.common.loading}</div>
  }

  if (!announcement) {
    return <div className="text-center py-12 text-muted-foreground">{t.announcement.announcementNotFound}</div>
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        <Button asChild variant="outline" size="sm">
          <Link href="/announcements">
            <ArrowLeft className="mr-1.5 h-4 w-4" />
            <span className="hidden md:inline">{t.announcement.backToList}</span>
          </Link>
        </Button>
        <h1 className="text-lg md:text-xl font-bold line-clamp-1">{announcement.title}</h1>
      </div>

      <Card>
        <CardContent className="p-4 md:p-6">
          <div className="space-y-4">
            <div className="flex items-center gap-2 flex-wrap">
              {announcement.is_mandatory && (
                <Badge variant="destructive" className="text-xs">
                  <AlertTriangle className="h-3 w-3 mr-1" />
                  {t.announcement.mandatory}
                </Badge>
              )}
              {announcement.is_read ? (
                <Badge variant="secondary" className="bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300 text-xs">
                  {t.announcement.read}
                </Badge>
              ) : (
                <Badge variant="secondary" className="bg-yellow-100 text-yellow-700 dark:bg-yellow-900 dark:text-yellow-300 text-xs">
                  {t.announcement.unread}
                </Badge>
              )}
              <span className="text-sm text-muted-foreground">
                {format(new Date(announcement.created_at), 'yyyy-MM-dd HH:mm', { locale: locale === 'zh' ? zhCN : undefined })}
              </span>
            </div>

            <MarkdownMessage
              content={announcement.content}
              allowHtml
              className="prose dark:prose-invert max-w-none text-base [&_*]:text-foreground"
            />
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
