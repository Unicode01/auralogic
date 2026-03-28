'use client'

import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getAnnouncements, AnnouncementWithRead } from '@/lib/api'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/page-loading'
import { Megaphone, ChevronLeft, ChevronRight, AlertTriangle } from 'lucide-react'
import Link from 'next/link'
import { format } from 'date-fns'
import { zhCN } from 'date-fns/locale'
import { useLocale } from '@/hooks/use-locale'
import { usePageTitle } from '@/hooks/use-page-title'
import { getTranslations } from '@/lib/i18n'
import { PluginSlot } from '@/components/plugins/plugin-slot'

export default function AnnouncementsPage() {
  const [page, setPage] = useState(1)
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.announcements)
  const limit = 10

  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ['announcements', page],
    queryFn: () => getAnnouncements({ page, limit }),
  })

  const announcements: AnnouncementWithRead[] = data?.data?.items || []
  const total = Number(data?.data?.pagination?.total || 0)
  const totalPages = Number(data?.data?.pagination?.total_pages || 0)
  const unreadCount = announcements.filter((item) => !item.is_read).length
  const mandatoryCount = announcements.filter((item) => item.is_mandatory).length
  const userAnnouncementsPluginContext = {
    view: 'user_announcements',
    pagination: {
      page,
      total,
      total_pages: totalPages,
      limit,
    },
    summary: {
      current_page_count: announcements.length,
      unread_count: unreadCount,
      mandatory_count: mandatoryCount,
    },
    state: {
      load_failed: isError && announcements.length === 0,
      empty: !isLoading && !isError && announcements.length === 0,
      has_results: announcements.length > 0,
      has_pagination: totalPages > 1,
    },
  }
  return (
    <div className="space-y-6">
      <PluginSlot slot="user.announcements.top" context={userAnnouncementsPluginContext} />
      <div>
        <h1 className="text-3xl font-bold">{t.announcement.announcements}</h1>
      </div>

      {isLoading ? (
        <div className="space-y-4">
          {[...Array(3)].map((_, index) => (
            <Card key={index}>
              <CardContent className="p-4">
                <div className="flex items-center justify-between gap-2">
                  <div className="min-w-0 flex-1 space-y-2">
                    <Skeleton className="h-4 w-2/3" />
                    <Skeleton className="h-3 w-32" />
                  </div>
                  <div className="flex gap-2">
                    <Skeleton className="h-6 w-16 rounded-full" />
                    <Skeleton className="h-6 w-16 rounded-full" />
                  </div>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      ) : isError && announcements.length === 0 ? (
        <Card className="border-dashed bg-muted/15">
          <CardContent className="py-12 text-center">
            <Megaphone className="mx-auto mb-4 h-12 w-12 text-muted-foreground" />
            <p className="text-base font-medium">{t.announcement.loadFailed}</p>
            <p className="mt-2 text-sm text-muted-foreground">{t.announcement.loadFailedDesc}</p>
            <Button className="mt-4" variant="outline" onClick={() => refetch()}>
              {t.common.refresh}
            </Button>
            <PluginSlot
              slot="user.announcements.load_failed"
              context={{ ...userAnnouncementsPluginContext, section: 'list_state' }}
            />
          </CardContent>
        </Card>
      ) : announcements.length === 0 ? (
        <Card>
          <CardContent className="py-12 text-center">
            <Megaphone className="mx-auto mb-4 h-12 w-12 text-muted-foreground" />
            <p className="text-base font-medium">{t.announcement.noAnnouncements}</p>
            <PluginSlot
              slot="user.announcements.empty"
              context={{ ...userAnnouncementsPluginContext, section: 'list_state' }}
            />
          </CardContent>
        </Card>
      ) : (
        <>
          <div className="space-y-4">
            {announcements.map((item) => (
              <Link key={item.id} href={`/announcements/${item.id}`} className="block">
                <Card className="cursor-pointer border transition-colors hover:bg-accent/50">
                  <CardContent className="p-4">
                    <div className="flex items-center justify-between gap-2">
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center gap-2">
                          <h3 className="flex-1 truncate text-sm font-medium">{item.title}</h3>
                          <div className="flex shrink-0 items-center gap-1.5">
                            {item.is_mandatory && (
                              <Badge variant="destructive" className="text-xs">
                                <AlertTriangle className="mr-1 h-3 w-3" />
                                {t.announcement.mandatory}
                              </Badge>
                            )}
                            {item.is_read ? (
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
                          </div>
                        </div>
                        <p className="mt-1.5 text-xs text-muted-foreground">
                          {format(new Date(item.created_at), 'yyyy-MM-dd HH:mm', {
                            locale: locale === 'zh' ? zhCN : undefined,
                          })}
                        </p>
                      </div>
                    </div>
                  </CardContent>
                </Card>
              </Link>
            ))}
          </div>
          <PluginSlot
            slot="user.announcements.list.after"
            context={{ ...userAnnouncementsPluginContext, section: 'list' }}
          />

          {totalPages > 1 && (
            <>
              <PluginSlot
                slot="user.announcements.pagination.before"
                context={{ ...userAnnouncementsPluginContext, section: 'pagination' }}
              />
              <div className="flex items-center justify-center gap-2 pt-2">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setPage((p) => Math.max(1, p - 1))}
                  disabled={page === 1}
                  aria-label={t.common.prevPage}
                  title={t.common.prevPage}
                >
                  <ChevronLeft className="h-4 w-4" />
                  <span className="sr-only">{t.common.prevPage}</span>
                </Button>
                <span className="px-2 text-sm text-muted-foreground">
                  {t.common.pageInfo
                    .replace('{page}', String(page))
                    .replace('{totalPages}', String(totalPages))}
                </span>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                  disabled={page === totalPages}
                  aria-label={t.common.nextPage}
                  title={t.common.nextPage}
                >
                  <ChevronRight className="h-4 w-4" />
                  <span className="sr-only">{t.common.nextPage}</span>
                </Button>
              </div>
              <PluginSlot
                slot="user.announcements.pagination.after"
                context={{ ...userAnnouncementsPluginContext, section: 'pagination' }}
              />
            </>
          )}
        </>
      )}
      <PluginSlot slot="user.announcements.bottom" context={userAnnouncementsPluginContext} />
    </div>
  )
}
