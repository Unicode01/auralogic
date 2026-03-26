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
import { Skeleton } from '@/components/ui/page-loading'
import { PluginSlot } from '@/components/plugins/plugin-slot'

export default function AnnouncementDetailPage() {
  const params = useParams()
  const id = Number(params.id)
  const queryClient = useQueryClient()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.announcementDetail)

  const { data, isLoading, isError, refetch } = useQuery({
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
  const userAnnouncementDetailPluginContext = {
    view: 'user_announcement_detail',
    announcement: announcement
      ? {
          id: announcement.id,
          title: announcement.title,
          category: announcement.category || undefined,
          created_at: announcement.created_at,
          updated_at: announcement.updated_at,
          is_mandatory: announcement.is_mandatory,
          require_full_read: announcement.require_full_read,
          is_read: announcement.is_read,
        }
      : {
          id: Number.isFinite(id) ? id : undefined,
        },
    summary: {
      content_length: announcement?.content.length || 0,
      has_category: Boolean(announcement?.category),
    },
    state: {
      load_failed: isError && !announcement,
      not_found: !isLoading && !isError && !announcement,
      is_read: Boolean(announcement?.is_read),
    },
  }

  useEffect(() => {
    if (announcement && !announcement.is_read) {
      markReadMutation.mutate()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [announcement?.id, announcement?.is_read])

  if (isLoading) {
    return (
      <div className="space-y-4">
        <div className="flex items-center gap-3">
          <Skeleton className="h-9 w-24" />
          <Skeleton className="h-7 w-64" />
        </div>
        <Card>
          <CardContent className="space-y-4 p-4 md:p-6">
            <div className="flex flex-wrap gap-2">
              <Skeleton className="h-6 w-16 rounded-full" />
              <Skeleton className="h-6 w-20 rounded-full" />
              <Skeleton className="h-5 w-32" />
            </div>
            <div className="space-y-2">
              <Skeleton className="h-4 w-full" />
              <Skeleton className="h-4 w-full" />
              <Skeleton className="h-4 w-5/6" />
              <Skeleton className="h-4 w-3/4" />
            </div>
          </CardContent>
        </Card>
      </div>
    )
  }

  if (isError && !announcement) {
    return (
      <div className="flex flex-col items-center justify-center py-20 text-center">
        <AlertTriangle className="mb-4 h-12 w-12 text-muted-foreground" />
        <p className="text-base font-medium">{t.announcement.detailLoadFailed}</p>
        <p className="mb-4 mt-2 max-w-md text-sm text-muted-foreground">
          {t.announcement.detailLoadFailedDesc}
        </p>
        <div className="flex flex-wrap justify-center gap-2">
          <Button variant="outline" onClick={() => refetch()}>
            {t.common.refresh}
          </Button>
          <Button variant="ghost" asChild>
            <Link href="/announcements">{t.announcement.backToList}</Link>
          </Button>
        </div>
        <PluginSlot
          slot="user.announcement_detail.load_failed"
          context={{ ...userAnnouncementDetailPluginContext, section: 'detail_state' }}
        />
      </div>
    )
  }

  if (!announcement) {
    return (
      <div className="flex flex-col items-center justify-center py-20 text-center">
        <AlertTriangle className="mb-4 h-12 w-12 text-muted-foreground" />
        <p className="mb-4 text-muted-foreground">{t.announcement.announcementNotFound}</p>
        <Button variant="outline" asChild>
          <Link href="/announcements">
            <ArrowLeft className="mr-2 h-4 w-4" />
            {t.announcement.backToList}
          </Link>
        </Button>
        <PluginSlot
          slot="user.announcement_detail.not_found"
          context={{ ...userAnnouncementDetailPluginContext, section: 'detail_state' }}
        />
      </div>
    )
  }

  return (
    <div className="space-y-4">
      <PluginSlot
        slot="user.announcement_detail.top"
        context={userAnnouncementDetailPluginContext}
      />
      <div className="flex items-center gap-3">
        <Button asChild variant="outline" size="sm">
          <Link href="/announcements">
            <ArrowLeft className="h-4 w-4 md:mr-1.5" />
            <span className="hidden md:inline">{t.announcement.backToList}</span>
            <span className="sr-only md:hidden">{t.announcement.backToList}</span>
          </Link>
        </Button>
        <h1 className="line-clamp-1 text-lg font-bold md:text-xl">{announcement.title}</h1>
      </div>

      <Card>
        <CardContent className="p-4 md:p-6">
          <div className="space-y-4">
            <div className="flex flex-wrap items-center gap-2">
              {announcement.is_mandatory && (
                <Badge variant="destructive" className="text-xs">
                  <AlertTriangle className="mr-1 h-3 w-3" />
                  {t.announcement.mandatory}
                </Badge>
              )}
              {announcement.is_read ? (
                <Badge
                  variant="secondary"
                  className="bg-green-100 text-xs text-green-700 dark:bg-green-900 dark:text-green-300"
                >
                  {t.announcement.read}
                </Badge>
              ) : (
                <Badge
                  variant="secondary"
                  className="bg-yellow-100 text-xs text-yellow-700 dark:bg-yellow-900 dark:text-yellow-300"
                >
                  {t.announcement.unread}
                </Badge>
              )}
              <span className="text-sm text-muted-foreground">
                {format(new Date(announcement.created_at), 'yyyy-MM-dd HH:mm', {
                  locale: locale === 'zh' ? zhCN : undefined,
                })}
              </span>
            </div>
            <PluginSlot
              slot="user.announcement_detail.meta.after"
              context={{ ...userAnnouncementDetailPluginContext, section: 'meta' }}
            />

            <PluginSlot
              slot="user.announcement_detail.content.before"
              context={{ ...userAnnouncementDetailPluginContext, section: 'content' }}
            />
            <MarkdownMessage content={announcement.content} allowHtml className="markdown-body" />
            <PluginSlot
              slot="user.announcement_detail.content.after"
              context={{ ...userAnnouncementDetailPluginContext, section: 'content' }}
            />
          </div>
        </CardContent>
      </Card>
      <PluginSlot
        slot="user.announcement_detail.bottom"
        context={userAnnouncementDetailPluginContext}
      />
    </div>
  )
}
