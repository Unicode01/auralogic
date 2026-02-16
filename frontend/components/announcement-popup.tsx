'use client'

import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  getUnreadMandatoryAnnouncements,
  markAnnouncementAsRead,
  type Announcement,
} from '@/lib/api'
import { useAuth } from '@/hooks/use-auth'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { Dialog, DialogContent, DialogTitle } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { MarkdownMessage } from '@/components/ui/markdown-message'
import { ArrowLeft, List, Loader2, Megaphone, ShieldAlert } from 'lucide-react'
import * as VisuallyHidden from '@radix-ui/react-visually-hidden'

export function AnnouncementPopup() {
  const { isAuthenticated } = useAuth()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  const queryClient = useQueryClient()

  const contentRef = useRef<HTMLDivElement>(null)
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const [canMarkRead, setCanMarkRead] = useState(false)
  const [mobilePane, setMobilePane] = useState<'list' | 'content'>('content')

  const { data, isLoading } = useQuery({
    queryKey: ['unreadMandatoryAnnouncements'],
    queryFn: getUnreadMandatoryAnnouncements,
    enabled: isAuthenticated,
    staleTime: 60000,
  })

  const announcements: Announcement[] = data?.data || []

  useEffect(() => {
    if (announcements.length === 0) {
      setSelectedId(null)
      return
    }

    setSelectedId((prev) => {
      if (prev && announcements.some((a) => a.id === prev)) return prev
      return announcements[0].id
    })
  }, [announcements])

  const current = useMemo(() => {
    if (announcements.length === 0) return undefined
    return announcements.find((a) => a.id === selectedId) || announcements[0]
  }, [announcements, selectedId])

  const markReadMutation = useMutation({
    mutationFn: (id: number) => markAnnouncementAsRead(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['unreadMandatoryAnnouncements'] })
      queryClient.invalidateQueries({ queryKey: ['announcements'] })
      setCanMarkRead(false)
    },
  })

  const checkScrollable = useCallback(() => {
    const el = contentRef.current
    if (!el) return
    if (el.scrollHeight <= el.clientHeight + 10) {
      setCanMarkRead(true)
    }
  }, [])

  useEffect(() => {
    if (!current) return

    // Reset read gating when switching items.
    setCanMarkRead(!current.require_full_read)

    const timer = setTimeout(checkScrollable, 100)
    return () => clearTimeout(timer)
  }, [current?.id, current?.require_full_read, checkScrollable])

  const handleScroll = useCallback(() => {
    const el = contentRef.current
    if (!el || !current?.require_full_read) return
    if (el.scrollTop + el.clientHeight >= el.scrollHeight - 10) {
      setCanMarkRead(true)
    }
  }, [current?.require_full_read])

  const handleMarkRead = () => {
    if (!current) return
    markReadMutation.mutate(current.id)
  }

  if (!isAuthenticated) return null
  if (announcements.length === 0) return null

  const remaining = announcements.length

  return (
    <Dialog open={announcements.length > 0} onOpenChange={() => {}}>
      <DialogContent
        hideClose
        className="max-w-5xl w-[min(96vw,72rem)] h-[min(85dvh,48rem)] p-0 overflow-hidden flex flex-col"
        onPointerDownOutside={(e) => e.preventDefault()}
        onEscapeKeyDown={(e) => e.preventDefault()}
      >
        <VisuallyHidden.Root>
          <DialogTitle>
            {t.announcement.mandatory} ({remaining})
          </DialogTitle>
        </VisuallyHidden.Root>
        <div className="grid grid-cols-1 md:grid-cols-[300px_1fr] flex-1 min-h-0">
          <div className={`border-b md:border-b-0 md:border-r p-3 md:p-4 flex flex-col min-h-0 ${mobilePane === 'list' ? '' : 'hidden md:flex'}`}>
            <div className="flex items-center justify-between gap-2">
              <div className="flex items-center gap-2">
                <ShieldAlert className="h-4 w-4 text-destructive" />
                <div className="font-semibold text-sm">
                  {t.announcement.mandatory}
                </div>
              </div>
              <div className="flex items-center gap-2">
                <Badge variant="secondary" className="text-xs">
                  {remaining}
                </Badge>
                <Button
                  type="button"
                  variant="outline"
                  size="icon"
                  className="md:hidden"
                  onClick={() => setMobilePane('content')}
                >
                  <ArrowLeft className="h-4 w-4" />
                  <span className="sr-only">{t.common.back}</span>
                </Button>
              </div>
            </div>
            <div className="text-xs text-muted-foreground mt-1">
              {t.announcement.mandatoryHint}
            </div>

            <div className="mt-3 flex-1 min-h-0">
              <ScrollArea className="h-full">
                <div className="space-y-1 pr-2">
                  {announcements.map((a) => {
                    const active = a.id === current?.id
                    return (
                      <button
                        key={a.id}
                        type="button"
                        onClick={() => {
                          setSelectedId(a.id)
                          setMobilePane('content')
                        }}
                        className={`w-full text-left rounded-md px-3 py-2.5 transition-colors border ${
                          active
                            ? 'bg-muted/60 border-border'
                            : 'bg-transparent border-transparent hover:bg-muted/40'
                        }`}
                      >
                        <div className="font-medium text-sm truncate">
                          {a.title}
                        </div>
                        <div className="flex items-center gap-2 mt-1">
                          {a.require_full_read ? (
                            <Badge variant="secondary" className="text-[11px]">
                              {t.announcement.requireFullRead}
                            </Badge>
                          ) : null}
                          <Badge variant="destructive" className="text-[11px]">
                            {t.announcement.unread}
                          </Badge>
                        </div>
                      </button>
                    )
                  })}
                </div>
              </ScrollArea>
            </div>
          </div>

          <div className={`p-3 md:p-4 flex flex-col min-h-0 ${mobilePane === 'list' ? 'hidden md:flex' : ''}`}>
            <div className="flex items-start justify-between gap-3 min-w-0">
              <div className="min-w-0">
                <div className="text-base font-semibold truncate">
                  {current?.title}
                </div>
                {current?.require_full_read && !canMarkRead ? (
                  <div className="text-xs text-muted-foreground mt-1">
                    {t.announcement.scrollToRead}
                  </div>
                ) : null}
              </div>
              <div className="flex items-center gap-2 flex-shrink-0">
                <Button
                  type="button"
                  variant="outline"
                  size="icon"
                  className="md:hidden"
                  onClick={() => setMobilePane('list')}
                >
                  <List className="h-4 w-4" />
                  <span className="sr-only">{t.announcement.announcements}</span>
                </Button>
                <Button
                  type="button"
                  onClick={handleMarkRead}
                  disabled={!canMarkRead || markReadMutation.isPending}
                >
                  {markReadMutation.isPending ? (
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  ) : (
                    <Megaphone className="mr-2 h-4 w-4" />
                  )}
                  {t.announcement.markAsRead}
                </Button>
              </div>
            </div>

            {isLoading ? (
              <div className="flex-1 min-h-0 flex items-center justify-center">
                <Loader2 className="h-8 w-8 animate-spin" />
              </div>
            ) : (
              <>
                <div
                  ref={contentRef}
                  className="flex-1 min-h-0 mt-3 overflow-y-auto rounded-md border bg-background p-4"
                  onScroll={handleScroll}
                >
                  <MarkdownMessage
                    content={current?.content || ''}
                    allowHtml
                    className="prose prose-sm dark:prose-invert max-w-none [&_*]:text-foreground"
                  />
                </div>
              </>
            )}
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
