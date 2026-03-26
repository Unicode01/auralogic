'use client'

import { usePathname, useRouter, useSearchParams } from 'next/navigation'
import { Suspense, useState, useEffect, useRef, useCallback, useMemo } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { getTickets, createTicket, getOrders, getPublicConfig, Ticket } from '@/lib/api'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/page-loading'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Plus,
  MessageSquare,
  Package,
  ChevronLeft,
  ChevronRight,
  Loader2,
  Search,
  X,
  XCircle,
} from 'lucide-react'
import { useToast } from '@/hooks/use-toast'
import { TICKET_STATUS_CONFIG, TICKET_PRIORITY_CONFIG } from '@/lib/constants'
import Link from 'next/link'
import { formatDistanceToNow } from 'date-fns'
import { zhCN } from 'date-fns/locale'
import { useLocale } from '@/hooks/use-locale'
import { usePageTitle } from '@/hooks/use-page-title'
import { getTranslations } from '@/lib/i18n'
import { PluginSlot } from '@/components/plugins/plugin-slot'
import { useDebounce } from '@/hooks/use-debounce'
import {
  buildUpdatedQueryString,
  normalizePositivePageQuery,
  normalizeQueryString,
} from '@/lib/query-state'
import {
  clearListBrowseState,
  getListFocusParamKey,
  parseFocusedListItemQuery,
  readListBrowseState,
  setListBrowseState,
  stripListFocusFromPath,
} from '@/lib/list-browse-state'
import { resolveApiErrorMessage } from '@/lib/api-error'
import { readPluginSearchParams } from '@/lib/plugin-frontend-routing'
import { PluginSlotBatchBoundary } from '@/lib/plugin-slot-batch'

function TicketsPageContent() {
  const router = useRouter()
  const pathname = usePathname() || '/tickets'
  const searchParams = useSearchParams()
  const searchParamsKey = searchParams.toString()
  const pluginQueryParams = readPluginSearchParams(searchParams)
  const initialStatus = normalizeQueryString(searchParams.get('status'))
  const initialSearch = normalizeQueryString(searchParams.get('search'))
  const initialPage = normalizePositivePageQuery(searchParams.get('page'))
  const initialListPath = stripListFocusFromPath(
    'tickets',
    searchParamsKey ? `${pathname}?${searchParamsKey}` : pathname
  )
  const initialBrowseState = readListBrowseState('tickets')
  const initialFocusedTicketId =
    parseFocusedListItemQuery(searchParams.get(getListFocusParamKey('tickets'))) ||
    (initialBrowseState?.listPath === initialListPath
      ? initialBrowseState.focusedItemKey
      : undefined)
  const [openCreate, setOpenCreate] = useState(false)
  const [status, setStatus] = useState(initialStatus)
  const [searchText, setSearchText] = useState(initialSearch)
  const [page, setPage] = useState(initialPage)
  const [highlightedTicketId, setHighlightedTicketId] = useState(initialFocusedTicketId)
  const [selectedOrderId, setSelectedOrderId] = useState<number | null>(null)
  const [draftContent, setDraftContent] = useState('')
  const [createFormKey, setCreateFormKey] = useState(0)
  const stateRef = useRef({
    status: initialStatus,
    searchText: initialSearch,
    page: initialPage,
  })
  const hasRestoredBrowseStateRef = useRef(false)
  const queryClient = useQueryClient()
  const toast = useToast()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.tickets)
  const limit = 10
  const debouncedSearch = useDebounce(searchText, 300)
  const currentListPath = initialListPath

  const replaceQueryState = (nextState: { status?: string; search?: string; page?: number }) => {
    const queryString = buildUpdatedQueryString(
      searchParams,
      {
        status: nextState.status || undefined,
        search: nextState.search || undefined,
        page: nextState.page,
        [getListFocusParamKey('tickets')]: undefined,
      },
      { page: 1 }
    )
    router.replace(queryString ? `${pathname}?${queryString}` : pathname, { scroll: false })
  }

  const { data: publicConfigData, isLoading: configLoading } = useQuery({
    queryKey: ['publicConfig'],
    queryFn: getPublicConfig,
    staleTime: 5 * 60 * 1000,
  })
  const ticketEnabled = publicConfigData?.data?.ticket?.enabled ?? true
  const maxContentLength = publicConfigData?.data?.ticket?.max_content_length || 0

  useEffect(() => {
    stateRef.current = {
      status,
      searchText,
      page,
    }
  }, [page, searchText, status])

  useEffect(() => {
    const nextStatus = normalizeQueryString(searchParams.get('status'))
    const nextSearch = normalizeQueryString(searchParams.get('search'))
    const nextPage = normalizePositivePageQuery(searchParams.get('page'))
    const browseState = readListBrowseState('tickets')
    const nextFocusedTicketId =
      parseFocusedListItemQuery(searchParams.get(getListFocusParamKey('tickets'))) ||
      (browseState?.listPath === currentListPath ? browseState.focusedItemKey : undefined)
    const currentState = stateRef.current
    const urlStateChanged =
      nextStatus !== currentState.status ||
      nextSearch !== currentState.searchText ||
      nextPage !== currentState.page

    if (!urlStateChanged) {
      return
    }

    setStatus(nextStatus)
    setSearchText(nextSearch)
    setPage(nextPage)
    setHighlightedTicketId(nextFocusedTicketId)
    hasRestoredBrowseStateRef.current = false
  }, [currentListPath, searchParams, searchParamsKey])

  useEffect(() => {
    if (!searchParams.get(getListFocusParamKey('tickets'))) {
      return
    }
    router.replace(currentListPath, { scroll: false })
  }, [currentListPath, router, searchParams, searchParamsKey])

  const {
    data: ticketsData,
    isLoading: ticketsLoading,
    isError: ticketsLoadFailed,
    refetch: refetchTickets,
  } = useQuery({
    queryKey: ['userTickets', status, debouncedSearch, page],
    queryFn: () =>
      getTickets({
        status: status || undefined,
        search: debouncedSearch || undefined,
        page,
        limit,
      }),
  })

  const {
    data: ordersData,
    isLoading: isOrdersLoading,
    isError: isOrdersError,
  } = useQuery({
    queryKey: ['userOrdersForTicket'],
    queryFn: () => getOrders({ limit: 50 }),
    enabled: openCreate,
  })

  const createMutation = useMutation({
    mutationFn: createTicket,
    onSuccess: () => {
      toast.success(t.ticket.createSuccess)
      queryClient.invalidateQueries({ queryKey: ['userTickets'] })
      setOpenCreate(false)
      setSelectedOrderId(null)
      setDraftContent('')
      setCreateFormKey((current) => current + 1)
    },
    onError: (error: any) => {
      toast.error(resolveApiErrorMessage(error, t, t.ticket.createFailed))
    },
  })

  const tickets: Ticket[] = ticketsData?.data?.items || []
  const total = ticketsData?.data?.total || 0
  const totalPages = Math.ceil(total / limit)
  const orders = ordersData?.data?.items || []
  const contentLength = draftContent.length
  const normalizedSearchText = normalizeQueryString(searchText)
  const hasActiveFilters = Boolean(status || normalizedSearchText)
  const userTicketsPluginContext = {
    view: 'user_tickets',
    filters: {
      status: status || undefined,
      search: normalizedSearchText || undefined,
      search_input: searchText || undefined,
      page,
      has_active_filters: hasActiveFilters,
    },
    pagination: {
      page,
      limit,
      total,
      total_pages: totalPages,
    },
    summary: {
      current_page_count: tickets.length,
      highlighted_ticket_id: highlightedTicketId || undefined,
      create_dialog_open: openCreate,
      ticket_enabled: ticketEnabled,
      active_filter_count: Number(Boolean(status)) + Number(Boolean(normalizedSearchText)),
    },
    draft: {
      related_order_id: selectedOrderId || undefined,
      content_length: contentLength,
      max_content_length: maxContentLength || undefined,
      selectable_order_count: orders.length,
    },
    state: {
      disabled: !configLoading && !ticketEnabled,
      load_failed: ticketsLoadFailed,
      empty: !ticketsLoading && !ticketsLoadFailed && tickets.length === 0,
      has_results: tickets.length > 0,
      has_pagination: totalPages > 1,
      has_related_orders: orders.length > 0,
      has_active_filters: hasActiveFilters,
      create_open: openCreate,
      create_submitting: createMutation.isPending,
      related_orders_loading: openCreate && isOrdersLoading,
      related_orders_load_failed: openCreate && isOrdersError,
    },
  }
  const ticketBatchItems = useMemo(
    () => [
      {
        slot: 'user.tickets.top',
        hostContext: userTicketsPluginContext,
      },
      {
        slot: 'user.tickets.disabled',
        hostContext: { ...userTicketsPluginContext, section: 'list_state' },
      },
      {
        slot: 'user.tickets.create.top',
        hostContext: { ...userTicketsPluginContext, section: 'create_dialog' },
      },
      {
        slot: 'user.tickets.create.related_order.after',
        hostContext: { ...userTicketsPluginContext, section: 'create_dialog' },
      },
      {
        slot: 'user.tickets.create.submit.before',
        hostContext: { ...userTicketsPluginContext, section: 'create_dialog' },
      },
      {
        slot: 'user.tickets.create.bottom',
        hostContext: { ...userTicketsPluginContext, section: 'create_dialog' },
      },
      {
        slot: 'user.tickets.filters.after',
        hostContext: userTicketsPluginContext,
      },
      {
        slot: 'user.tickets.before_list',
        hostContext: userTicketsPluginContext,
      },
      {
        slot: 'user.tickets.load_failed',
        hostContext: { ...userTicketsPluginContext, section: 'list_state' },
      },
      {
        slot: 'user.tickets.empty',
        hostContext: { ...userTicketsPluginContext, section: 'list_state' },
      },
      {
        slot: 'user.tickets.pagination.before',
        hostContext: { ...userTicketsPluginContext, section: 'pagination' },
      },
      {
        slot: 'user.tickets.pagination.after',
        hostContext: { ...userTicketsPluginContext, section: 'pagination' },
      },
    ],
    [userTicketsPluginContext]
  )
  const relatedOrderHelperText = isOrdersLoading
    ? t.ticket.loadingOrdersDesc
    : isOrdersError
      ? t.ticket.relatedOrderLoadFailed
      : orders.length > 0
        ? t.ticket.relatedOrderTip
        : t.ticket.noOrdersToLink

  const handleSubmit = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    const formData = new FormData(e.currentTarget)
    const content = formData.get('content') as string
    if (maxContentLength > 0 && content.length > maxContentLength) {
      toast.error(t.ticket.contentTooLong.replace('{max}', String(maxContentLength)))
      return
    }
    createMutation.mutate({
      subject: formData.get('subject') as string,
      content,
      category: formData.get('category') as string,
      priority: formData.get('priority') as string,
      order_id: selectedOrderId || undefined,
    })
  }

  const getStatusBadge = (ticketStatus: string) => {
    const config = TICKET_STATUS_CONFIG[ticketStatus as keyof typeof TICKET_STATUS_CONFIG]
    if (!config) return <Badge variant="secondary">{ticketStatus}</Badge>
    const label =
      t.ticket.ticketStatus[ticketStatus as keyof typeof t.ticket.ticketStatus] || config.label
    return <Badge variant={config.color as any}>{label}</Badge>
  }

  const handleStatusChange = (newStatus: string) => {
    const nextStatus = newStatus === 'all' ? '' : newStatus
    setStatus(nextStatus)
    setPage(1)
    replaceQueryState({
      status: nextStatus,
      search: searchText,
      page: 1,
    })
  }

  const handleSearchChange = (value: string) => {
    const nextSearch = value
    setSearchText(nextSearch)
    setPage(1)
    replaceQueryState({
      status,
      search: normalizeQueryString(nextSearch),
      page: 1,
    })
  }

  const handlePageChange = (nextPage: number) => {
    setPage(nextPage)
    replaceQueryState({
      status,
      search: normalizeQueryString(searchText),
      page: nextPage,
    })
  }

  const handleResetFilters = () => {
    setStatus('')
    setSearchText('')
    setPage(1)
    replaceQueryState({
      status: undefined,
      search: undefined,
      page: 1,
    })
  }

  const getTicketsScrollTop = useCallback(() => {
    if (typeof document === 'undefined' || typeof window === 'undefined') {
      return 0
    }
    const mainElement = document.querySelector('main')
    if (mainElement instanceof HTMLElement) {
      return Math.max(0, mainElement.scrollTop)
    }
    return Math.max(0, window.scrollY)
  }, [])

  const restoreTicketsScrollTop = useCallback((scrollTop: number) => {
    if (typeof document === 'undefined' || typeof window === 'undefined') {
      return
    }
    const nextScrollTop = Math.max(0, Number(scrollTop) || 0)
    window.requestAnimationFrame(() => {
      const mainElement = document.querySelector('main')
      if (mainElement instanceof HTMLElement) {
        mainElement.scrollTo({ top: nextScrollTop })
        return
      }
      window.scrollTo({ top: nextScrollTop })
    })
  }, [])

  const handleOpenTicket = useCallback(
    (ticketId: number) => {
      const focusedItemKey = String(ticketId)
      setListBrowseState('tickets', {
        listPath: currentListPath,
        scrollTop: getTicketsScrollTop(),
        focusedItemKey,
      })
      setHighlightedTicketId(focusedItemKey)
    },
    [currentListPath, getTicketsScrollTop]
  )

  useEffect(() => {
    if (ticketsLoading || hasRestoredBrowseStateRef.current) {
      return
    }

    const browseState = readListBrowseState('tickets')
    const shouldRestoreScroll = browseState?.listPath === currentListPath
    const focusedTicketId = highlightedTicketId || browseState?.focusedItemKey
    const focusedTicketElement = focusedTicketId
      ? document.querySelector(`[data-ticket-id="${focusedTicketId}"]`)
      : null

    if (!shouldRestoreScroll && !(focusedTicketElement instanceof HTMLElement)) {
      return
    }

    hasRestoredBrowseStateRef.current = true

    if (shouldRestoreScroll && browseState) {
      if (browseState.focusedItemKey && !highlightedTicketId) {
        setHighlightedTicketId(browseState.focusedItemKey)
      }
      restoreTicketsScrollTop(browseState.scrollTop)
      clearListBrowseState('tickets')
      return
    }

    if (focusedTicketElement instanceof HTMLElement) {
      focusedTicketElement.scrollIntoView({
        block: 'center',
        behavior: 'smooth',
      })
    }
  }, [currentListPath, highlightedTicketId, ticketsLoading, restoreTicketsScrollTop])

  if (!configLoading && !ticketEnabled) {
    return (
      <div className="flex flex-col items-center justify-center py-20 text-center">
        <XCircle className="mb-4 h-12 w-12 text-muted-foreground" />
        <h2 className="mb-2 text-lg font-medium">{t.ticket.disabledTitle}</h2>
        <p className="text-sm text-muted-foreground">{t.ticket.disabledDesc}</p>
        <PluginSlot
          slot="user.tickets.disabled"
          context={{ ...userTicketsPluginContext, section: 'list_state' }}
        />
      </div>
    )
  }

  return (
    <PluginSlotBatchBoundary
      scope="public"
      path="/tickets"
      items={ticketBatchItems}
      queryParams={pluginQueryParams}
    >
      <div className="space-y-6">
        <PluginSlot slot="user.tickets.top" context={userTicketsPluginContext} />

        <div className="flex items-center justify-between gap-3">
          <div>
            <h1 className="text-3xl font-bold">{t.ticket.supportCenter}</h1>
          </div>
          <Dialog
            open={openCreate}
            onOpenChange={(open) => {
              setOpenCreate(open)
              if (!open) {
                setSelectedOrderId(null)
                setDraftContent('')
                setCreateFormKey((current) => current + 1)
              }
            }}
          >
            <DialogTrigger asChild>
              <Button
                size="sm"
                className="gap-2 shrink-0"
                aria-label={t.ticket.createTicket}
                title={t.ticket.createTicket}
              >
                <Plus className="h-4 w-4" />
                <span className="hidden md:inline">{t.ticket.createTicket}</span>
                <span className="sr-only md:hidden">{t.ticket.createTicket}</span>
              </Button>
            </DialogTrigger>
            <DialogContent className="max-h-[90vh] max-w-lg overflow-y-auto">
              <DialogHeader>
                <DialogTitle>{t.ticket.createTicket}</DialogTitle>
                <DialogDescription>{t.ticket.createTicketDesc}</DialogDescription>
              </DialogHeader>
              <PluginSlot
                slot="user.tickets.create.top"
                context={{ ...userTicketsPluginContext, section: 'create_dialog' }}
              />
              <form key={createFormKey} onSubmit={handleSubmit} className="space-y-4">
                <div>
                  <label className="text-sm font-medium">{t.ticket.subjectRequired}</label>
                  <Input
                    name="subject"
                    placeholder={t.ticket.subjectPlaceholder}
                    className="mt-1.5"
                    required
                  />
                </div>

                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label className="text-sm font-medium">{t.ticket.category}</label>
                    <Select name="category" defaultValue="general">
                      <SelectTrigger className="mt-1.5">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="general">{t.ticket.generalInquiry}</SelectItem>
                        <SelectItem value="order">{t.ticket.orderIssue}</SelectItem>
                        <SelectItem value="product">{t.ticket.productIssue}</SelectItem>
                        <SelectItem value="shipping">{t.ticket.logisticsIssue}</SelectItem>
                        <SelectItem value="refund">{t.ticket.refundIssue}</SelectItem>
                        <SelectItem value="other">{t.ticket.other}</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>

                  <div>
                    <label className="text-sm font-medium">{t.ticket.priority}</label>
                    <Select name="priority" defaultValue="normal">
                      <SelectTrigger className="mt-1.5">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {Object.entries(TICKET_PRIORITY_CONFIG).map(([key, config]) => (
                          <SelectItem key={key} value={key}>
                            {t.ticket.ticketPriority[key as keyof typeof t.ticket.ticketPriority] ||
                              config.label}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                </div>

                <div>
                  <label className="text-sm font-medium">{t.ticket.descriptionRequired}</label>
                  <Textarea
                    name="content"
                    placeholder={t.ticket.descriptionPlaceholder}
                    className="mt-1.5 min-h-[120px]"
                    value={draftContent}
                    onChange={(e) => setDraftContent(e.target.value)}
                    required
                    maxLength={maxContentLength > 0 ? maxContentLength : undefined}
                  />
                  <div className="mt-1 flex items-center justify-between gap-3 text-xs">
                    <span className="text-muted-foreground">
                      {maxContentLength > 0
                        ? t.ticket.contentLimitDesc.replace('{max}', String(maxContentLength))
                        : t.ticket.contentUnlimited}
                    </span>
                    <span
                      className={
                        maxContentLength > 0 && contentLength >= Math.max(1, maxContentLength - 20)
                          ? 'font-medium text-amber-600'
                          : 'text-muted-foreground'
                      }
                    >
                      {t.ticket.contentLengthLabel}: {contentLength}
                      {maxContentLength > 0 ? `/${maxContentLength}` : ''}
                    </span>
                  </div>
                </div>

                <div>
                  <label className="flex items-center gap-2 text-sm font-medium">
                    <Package className="h-4 w-4" />
                    {t.ticket.relatedOrder}
                  </label>
                  <Select
                    value={selectedOrderId?.toString() || 'none'}
                    onValueChange={(value) =>
                      setSelectedOrderId(value === 'none' ? null : Number(value))
                    }
                  >
                    <SelectTrigger className="mt-1.5" disabled={isOrdersLoading}>
                      <SelectValue placeholder={t.ticket.selectOrder} />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="none">{t.ticket.noRelatedOrder}</SelectItem>
                      {isOrdersLoading ? (
                        <SelectItem value="loading" disabled>
                          {t.common.loading}
                        </SelectItem>
                      ) : orders.length === 0 ? (
                        <SelectItem value="empty" disabled>
                          {t.ticket.noOrdersToLink}
                        </SelectItem>
                      ) : (
                        orders.map((order: any) => (
                          <SelectItem key={order.id} value={order.id.toString()}>
                            {order.order_no} - {order.product?.name || t.ticket.items}
                          </SelectItem>
                        ))
                      )}
                    </SelectContent>
                  </Select>
                  <p className="mt-1 text-xs text-muted-foreground">{relatedOrderHelperText}</p>
                  <PluginSlot
                    slot="user.tickets.create.related_order.after"
                    context={{ ...userTicketsPluginContext, section: 'create_dialog' }}
                  />
                </div>

                <PluginSlot
                  slot="user.tickets.create.submit.before"
                  context={{ ...userTicketsPluginContext, section: 'create_dialog' }}
                />
                <div className="flex gap-2">
                  <Button type="submit" disabled={createMutation.isPending} className="flex-1">
                    {createMutation.isPending ? (
                      <>
                        <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                        {t.ticket.submitting}
                      </>
                    ) : (
                      t.ticket.submitTicket
                    )}
                  </Button>
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => {
                      setOpenCreate(false)
                      setSelectedOrderId(null)
                      setDraftContent('')
                      setCreateFormKey((current) => current + 1)
                    }}
                  >
                    {t.common.cancel}
                  </Button>
                </div>
              </form>
              <PluginSlot
                slot="user.tickets.create.bottom"
                context={{ ...userTicketsPluginContext, section: 'create_dialog' }}
              />
            </DialogContent>
          </Dialog>
        </div>

        <div className="flex flex-col gap-2 md:flex-row">
          <div className="relative flex-1">
            <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              placeholder={t.ticket.searchPlaceholder}
              value={searchText}
              onChange={(e) => handleSearchChange(e.target.value)}
              className="pl-9 pr-9"
            />
            {searchText ? (
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="absolute right-1 top-1/2 h-7 w-7 -translate-y-1/2 rounded-full text-muted-foreground hover:text-foreground"
                onClick={() => handleSearchChange('')}
              >
                <X className="h-4 w-4" />
                <span className="sr-only">{t.common.clear}</span>
              </Button>
            ) : null}
          </div>
          <Select value={status || 'all'} onValueChange={handleStatusChange}>
            <SelectTrigger className="w-full md:w-40">
              <SelectValue placeholder={t.ticket.status} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t.ticket.allStatus}</SelectItem>
              {Object.entries(TICKET_STATUS_CONFIG).map(([key, config]) => (
                <SelectItem key={key} value={key}>
                  {t.ticket.ticketStatus[key as keyof typeof t.ticket.ticketStatus] || config.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        {hasActiveFilters ? (
          <div className="flex justify-end">
            <Button variant="ghost" size="sm" onClick={handleResetFilters}>
              {t.common.reset}
            </Button>
          </div>
        ) : null}
        <PluginSlot slot="user.tickets.filters.after" context={userTicketsPluginContext} />

        <PluginSlot slot="user.tickets.before_list" context={userTicketsPluginContext} />

        {ticketsLoading ? (
          <div className="space-y-4">
            {[...Array(3)].map((_, index) => (
              <Card key={index}>
                <CardContent className="p-4">
                  <div className="flex items-center justify-between gap-2">
                    <div className="min-w-0 flex-1 space-y-2">
                      <div className="flex items-center gap-2">
                        <Skeleton className="h-4 w-40" />
                        <Skeleton className="h-6 w-20 rounded-full" />
                      </div>
                      <Skeleton className="h-3 w-full" />
                      <Skeleton className="h-3 w-40" />
                    </div>
                    <Skeleton className="h-6 w-8 rounded-full" />
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>
        ) : ticketsLoadFailed ? (
          <Card className="border-dashed bg-muted/15">
            <CardContent className="space-y-4 py-10 text-center">
              <Alert className="mx-auto max-w-xl text-left" variant="destructive">
                <AlertTitle>{t.ticket.ticketListLoadFailed}</AlertTitle>
                <AlertDescription>{t.ticket.ticketListLoadFailedDesc}</AlertDescription>
              </Alert>
              <div className="flex flex-wrap justify-center gap-2">
                <Button variant="outline" onClick={() => refetchTickets()}>
                  {t.common.refresh}
                </Button>
                {hasActiveFilters ? (
                  <Button variant="ghost" onClick={handleResetFilters}>
                    {t.common.reset}
                  </Button>
                ) : null}
              </div>
              <PluginSlot
                slot="user.tickets.load_failed"
                context={{ ...userTicketsPluginContext, section: 'list_state' }}
              />
            </CardContent>
          </Card>
        ) : tickets.length === 0 ? (
          <Card>
            <CardContent className="py-12 text-center">
              <MessageSquare className="mx-auto mb-4 h-12 w-12 text-muted-foreground" />
              <p className="text-base font-medium">
                {hasActiveFilters ? t.ticket.noTicketsFilteredTitle : t.ticket.noTickets}
              </p>
              {hasActiveFilters ? (
                <p className="mt-2 text-sm text-muted-foreground">
                  {t.ticket.noTicketsFilteredDesc}
                </p>
              ) : null}
              {hasActiveFilters ? (
                <Button className="mt-4" variant="outline" onClick={handleResetFilters}>
                  {t.common.reset}
                </Button>
              ) : (
                <Button className="mt-4" onClick={() => setOpenCreate(true)}>
                  {t.ticket.createFirst}
                </Button>
              )}
              <PluginSlot
                slot="user.tickets.empty"
                context={{ ...userTicketsPluginContext, section: 'list_state' }}
              />
            </CardContent>
          </Card>
        ) : (
          <>
            <div className="space-y-3">
              {tickets.map((ticket) => (
                <Link
                  key={ticket.id}
                  href={`/tickets/${ticket.id}`}
                  className="block"
                  data-ticket-id={ticket.id}
                  onClick={() => handleOpenTicket(ticket.id)}
                >
                  <Card
                    className={`border transition-colors ${
                      highlightedTicketId === String(ticket.id)
                        ? 'border-primary bg-accent/20'
                        : 'hover:bg-accent/40'
                    }`}
                  >
                    <CardContent className="p-4">
                      <div className="flex items-start justify-between gap-3">
                        <div className="min-w-0 flex-1">
                          <div className="flex flex-wrap items-start gap-2">
                            <h3 className="min-w-0 flex-1 truncate text-sm font-medium md:text-base">
                              {ticket.subject}
                            </h3>
                            <div className="flex shrink-0 items-center gap-1.5">
                              {getStatusBadge(ticket.status)}
                              {ticket.unread_count_user > 0 ? (
                                <Badge variant="destructive" className="text-xs">
                                  {ticket.unread_count_user}
                                </Badge>
                              ) : null}
                            </div>
                          </div>
                          <p className="mt-2 line-clamp-2 text-sm text-muted-foreground">
                            {ticket.last_message_preview || ticket.content}
                          </p>
                          <p className="mt-2 text-xs text-muted-foreground">
                            {ticket.last_message_at
                              ? formatDistanceToNow(new Date(ticket.last_message_at), {
                                  addSuffix: true,
                                  locale: locale === 'zh' ? zhCN : undefined,
                                })
                              : formatDistanceToNow(new Date(ticket.created_at), {
                                  addSuffix: true,
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
              slot="user.tickets.list.after"
              context={{ ...userTicketsPluginContext, section: 'list' }}
            />

            {totalPages > 1 ? (
              <>
                <PluginSlot
                  slot="user.tickets.pagination.before"
                  context={{ ...userTicketsPluginContext, section: 'pagination' }}
                />
                <div className="flex items-center justify-center gap-2 pt-2">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => handlePageChange(Math.max(1, page - 1))}
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
                    onClick={() => handlePageChange(Math.min(totalPages, page + 1))}
                    disabled={page === totalPages}
                    aria-label={t.common.nextPage}
                    title={t.common.nextPage}
                  >
                    <ChevronRight className="h-4 w-4" />
                    <span className="sr-only">{t.common.nextPage}</span>
                  </Button>
                </div>
                <PluginSlot
                  slot="user.tickets.pagination.after"
                  context={{ ...userTicketsPluginContext, section: 'pagination' }}
                />
              </>
            ) : null}
          </>
        )}

        <PluginSlot slot="user.tickets.bottom" context={userTicketsPluginContext} />
      </div>
    </PluginSlotBatchBoundary>
  )
}

export default function TicketsPage() {
  return (
    <Suspense fallback={<div className="min-h-[40vh]" />}>
      <TicketsPageContent />
    </Suspense>
  )
}
