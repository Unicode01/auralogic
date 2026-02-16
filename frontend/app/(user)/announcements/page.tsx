'use client'

import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getAnnouncements, AnnouncementWithRead } from '@/lib/api'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Megaphone, ChevronLeft, ChevronRight, AlertTriangle } from 'lucide-react'
import Link from 'next/link'
import { format } from 'date-fns'
import { zhCN } from 'date-fns/locale'
import { useLocale } from '@/hooks/use-locale'
import { usePageTitle } from '@/hooks/use-page-title'
import { getTranslations } from '@/lib/i18n'

export default function AnnouncementsPage() {
  const [page, setPage] = useState(1)
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.announcements)
  const limit = 10

  const { data, isLoading } = useQuery({
    queryKey: ['announcements', page],
    queryFn: () => getAnnouncements({ page, limit }),
  })

  const announcements: AnnouncementWithRead[] = data?.data?.items || []
  const total = data?.data?.total || 0
  const totalPages = Math.ceil(total / limit)

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-xl md:text-2xl font-bold">{t.announcement.announcements}</h1>
      </div>

      {isLoading ? (
        <div className="text-center py-12 text-muted-foreground">{t.common.loading}</div>
      ) : announcements.length === 0 ? (
        <Card>
          <CardContent className="text-center py-12">
            <Megaphone className="h-12 w-12 mx-auto text-muted-foreground mb-4" />
            <p className="text-muted-foreground">{t.announcement.noAnnouncements}</p>
          </CardContent>
        </Card>
      ) : (
        <>
          <div className="space-y-4">
            {announcements.map((item) => (
              <Link key={item.id} href={`/announcements/${item.id}`} className="block">
                <Card className="hover:bg-accent/50 transition-colors cursor-pointer border">
                  <CardContent className="p-4">
                    <div className="flex items-center justify-between gap-2">
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2">
                          <h3 className="font-medium text-sm truncate flex-1">{item.title}</h3>
                          <div className="flex items-center gap-1.5 shrink-0">
                            {item.is_mandatory && (
                              <Badge variant="destructive" className="text-xs">
                                <AlertTriangle className="h-3 w-3 mr-1" />
                                {t.announcement.mandatory}
                              </Badge>
                            )}
                            {item.is_read ? (
                              <Badge variant="secondary" className="bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300 text-xs">
                                {t.announcement.read}
                              </Badge>
                            ) : (
                              <Badge variant="secondary" className="bg-yellow-100 text-yellow-700 dark:bg-yellow-900 dark:text-yellow-300 text-xs">
                                {t.announcement.unread}
                              </Badge>
                            )}
                          </div>
                        </div>
                        <p className="text-xs text-muted-foreground mt-1.5">
                          {format(new Date(item.created_at), 'yyyy-MM-dd HH:mm', { locale: locale === 'zh' ? zhCN : undefined })}
                        </p>
                      </div>
                    </div>
                  </CardContent>
                </Card>
              </Link>
            ))}
          </div>

          {totalPages > 1 && (
            <div className="flex items-center justify-center gap-2 pt-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => setPage(p => Math.max(1, p - 1))}
                disabled={page === 1}
              >
                <ChevronLeft className="h-4 w-4" />
              </Button>
              <span className="text-sm text-muted-foreground px-2">
                {page} / {totalPages}
              </span>
              <Button
                variant="outline"
                size="sm"
                onClick={() => setPage(p => Math.min(totalPages, p + 1))}
                disabled={page === totalPages}
              >
                <ChevronRight className="h-4 w-4" />
              </Button>
            </div>
          )}
        </>
      )}
    </div>
  )
}
