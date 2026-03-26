'use client'

import { Suspense, useCallback, useEffect, useMemo, useRef, useState } from 'react'
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
import { PluginSlot } from '@/components/plugins/plugin-slot'
import { ArrowLeft, List, Loader2, Megaphone, RefreshCw, ShieldAlert } from 'lucide-react'
import * as VisuallyHidden from '@radix-ui/react-visually-hidden'

const EMPTY_ANNOUNCEMENTS: Announcement[] = []

export function AnnouncementPopup() {
  const { isAuthenticated } = useAuth()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  const queryClient = useQueryClient()

  const contentRef = useRef<HTMLDivElement>(null)
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const [canMarkRead, setCanMarkRead] = useState(false)
  const [mobilePane, setMobilePane] = useState<'list' | 'content'>('content')

  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ['unreadMandatoryAnnouncements'],
    queryFn: getUnreadMandatoryAnnouncements,
    enabled: isAuthenticated,
    staleTime: 60000,
  })

  const announcements: Announcement[] = data?.data ?? EMPTY_ANNOUNCEMENTS

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
  }, [current, checkScrollable])

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
  if (!isError && announcements.length === 0) return null

  const remaining = announcements.length
  const announcementPopupPluginContext = {
    view: 'user_announcement_popup',
    locale,
    summary: {
      remaining_count: remaining,
      is_error: isError,
      is_loading: isLoading,
      can_mark_read: canMarkRead,
    },
    selection: {
      selected_id: current?.id,
      mobile_pane: mobilePane,
    },
    announcement: current
      ? {
          id: current.id,
          title: current.title,
          require_full_read: current.require_full_read,
        }
      : undefined,
    ui: {
      mobile_pane: mobilePane,
    },
    state: {
      load_failed: isError,
      loading: isLoading,
      has_items: announcements.length > 0,
      require_full_read: Boolean(current?.require_full_read),
      can_mark_read: canMarkRead,
    },
  }

  return (
    <Dialog open={isError || announcements.length > 0} onOpenChange={() => {}}>
      <DialogContent
        hideClose
        className="flex h-[min(85dvh,48rem)] w-[min(96vw,72rem)] max-w-5xl flex-col overflow-hidden p-0"
        onPointerDownOutside={(e) => e.preventDefault()}
        onEscapeKeyDown={(e) => e.preventDefault()}
      >
        <VisuallyHidden.Root>
          <DialogTitle>
            {isError ? t.announcement.loadFailed : `${t.announcement.mandatory} (${remaining})`}
          </DialogTitle>
        </VisuallyHidden.Root>
        <Suspense fallback={null}>
          <PluginSlot
            slot="user.layout.announcement_popup.top"
            context={announcementPopupPluginContext}
          />
        </Suspense>
        {isError ? (
          <div className="flex h-full flex-col items-center justify-center p-6 text-center md:p-10">
            <div className="mb-4 flex h-14 w-14 items-center justify-center rounded-full bg-muted">
              <ShieldAlert className="h-7 w-7 text-muted-foreground" />
            </div>
            <div className="text-lg font-semibold">{t.announcement.loadFailed}</div>
            <p className="mt-2 max-w-md text-sm text-muted-foreground">
              {t.announcement.loadFailedDesc}
            </p>
            <div className="mt-5 flex flex-col gap-3 sm:flex-row">
              <Button onClick={() => void refetch()}>
                <RefreshCw className="mr-2 h-4 w-4" />
                {t.common.refresh}
              </Button>
            </div>
            <PluginSlot
              slot="user.layout.announcement_popup.load_failed"
              context={{ ...announcementPopupPluginContext, section: 'popup_state' }}
            />
          </div>
        ) : (
          <div className="grid min-h-0 flex-1 grid-cols-1 md:grid-cols-[300px_1fr]">
            <div
              className={`flex min-h-0 flex-col border-b p-3 md:border-b-0 md:border-r md:p-4 ${mobilePane === 'list' ? '' : 'hidden md:flex'}`}
            >
              <div className="flex items-center justify-between gap-2">
                <div className="flex items-center gap-2">
                  <ShieldAlert className="h-4 w-4 text-destructive" />
                  <div className="text-sm font-semibold">{t.announcement.mandatory}</div>
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
                    aria-label={t.common.back}
                    title={t.common.back}
                  >
                    <ArrowLeft className="h-4 w-4" />
                    <span className="sr-only">{t.common.back}</span>
                  </Button>
                </div>
              </div>
              <div className="mt-3 min-h-0 flex-1">
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
                          aria-pressed={active}
                          title={a.title}
                          className={`w-full rounded-md border px-3 py-2.5 text-left transition-colors ${
                            active
                              ? 'border-border bg-muted/60'
                              : 'border-transparent bg-transparent hover:bg-muted/40'
                          }`}
                        >
                          <div className="truncate text-sm font-medium">{a.title}</div>
                          <div className="mt-1 flex items-center gap-2">
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
              <PluginSlot
                slot="user.layout.announcement_popup.list.after"
                context={{ ...announcementPopupPluginContext, section: 'list' }}
              />
            </div>

            <div
              className={`flex min-h-0 flex-col p-3 md:p-4 ${mobilePane === 'list' ? 'hidden md:flex' : ''}`}
            >
              <div className="flex min-w-0 items-start justify-between gap-3">
                <div className="min-w-0">
                  <div className="truncate text-base font-semibold">{current?.title}</div>
                  {current?.require_full_read && !canMarkRead ? (
                    <div className="mt-1 text-xs text-muted-foreground">
                      {t.announcement.scrollToRead}
                    </div>
                  ) : null}
                </div>
                <div className="flex max-w-full flex-shrink-0 flex-col items-stretch gap-2">
                  <PluginSlot
                    slot="user.layout.announcement_popup.actions.before"
                    context={{ ...announcementPopupPluginContext, section: 'actions' }}
                  />
                  <div className="flex items-center gap-2">
                    <Button
                      type="button"
                      variant="outline"
                      size="icon"
                      className="md:hidden"
                      onClick={() => setMobilePane('list')}
                      aria-label={t.announcement.announcements}
                      title={t.announcement.announcements}
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
              </div>

              {isLoading ? (
                <div className="flex min-h-0 flex-1 items-center justify-center">
                  <Loader2 className="h-8 w-8 animate-spin" />
                </div>
              ) : (
                <>
                  <PluginSlot
                    slot="user.layout.announcement_popup.content.before"
                    context={{ ...announcementPopupPluginContext, section: 'content' }}
                  />
                  <div
                    ref={contentRef}
                    className="mt-3 min-h-0 flex-1 overflow-y-auto rounded-md border bg-background p-4"
                    onScroll={handleScroll}
                  >
                    <MarkdownMessage
                      content={current?.content || ''}
                      allowHtml
                      className="markdown-body"
                    />
                  </div>
                </>
              )}
            </div>
          </div>
        )}
        <Suspense fallback={null}>
          <PluginSlot
            slot="user.layout.announcement_popup.bottom"
            context={announcementPopupPluginContext}
          />
        </Suspense>
      </DialogContent>
    </Dialog>
  )
}
