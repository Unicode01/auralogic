'use client'

import {
  Suspense,
  useDeferredValue,
  useState,
  useEffect,
  useRef,
  useMemo,
  useCallback,
} from 'react'
import { useQuery, useInfiniteQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  getAdminTickets,
  getAdminTicket,
  getAdminTicketMessages,
  sendAdminTicketMessage,
  updateAdminTicket,
  getAdminTicketSharedOrders,
  getAdminTicketSharedOrder,
  getTicketStats,
  uploadAdminTicketFile,
  getPublicConfig,
  Ticket,
  TicketMessage,
} from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { MarkdownMessage } from '@/components/ui/markdown-message'
import { MessageToolbar } from '@/components/ticket/message-toolbar'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Search,
  Package,
  User,
  Clock,
  Check,
  CheckCheck,
  MapPin,
  Truck,
  MessageSquare,
} from 'lucide-react'
import { useToast } from '@/hooks/use-toast'
import { TICKET_STATUS_CONFIG, TICKET_PRIORITY_CONFIG } from '@/lib/constants'
import { format, formatDistanceToNow } from 'date-fns'
import { zhCN } from 'date-fns/locale'
import { cn } from '@/lib/utils'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { formatCurrency, formatDate } from '@/lib/utils'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'
import { PluginExtensionList } from '@/components/plugins/plugin-extension-list'
import { PluginSlot } from '@/components/plugins/plugin-slot'
import { resolveApiErrorMessage } from '@/lib/api-error'
import { usePluginExtensionBatch } from '@/lib/plugin-extension-batch'

function resolveTicketMessageOrderID(msg: TicketMessage): number | null {
  if (msg.content_type !== 'order' || !msg.metadata) return null
  try {
    const meta = typeof msg.metadata === 'string' ? JSON.parse(msg.metadata) : msg.metadata
    return meta.order_id || null
  } catch {
    return null
  }
}

function buildAdminTicketSummary(ticket: Ticket | null | undefined) {
  if (!ticket) {
    return null
  }
  return {
    id: ticket.id,
    ticket_no: ticket.ticket_no,
    user_id: ticket.user_id,
    subject: ticket.subject,
    content: ticket.content,
    category: ticket.category,
    priority: ticket.priority,
    status: ticket.status,
    assigned_to: ticket.assigned_to,
    unread_count_user: ticket.unread_count_user,
    unread_count_admin: ticket.unread_count_admin,
    last_message_at: ticket.last_message_at,
    last_message_preview: ticket.last_message_preview,
    last_message_by: ticket.last_message_by,
    created_at: ticket.created_at,
    updated_at: ticket.updated_at,
    closed_at: ticket.closed_at,
    user: ticket.user
      ? {
          id: ticket.user.id,
          name: ticket.user.name,
          email: ticket.user.email,
        }
      : undefined,
    assigned_user: ticket.assigned_user
      ? {
          id: ticket.assigned_user.id,
          name: ticket.assigned_user.name,
          email: ticket.assigned_user.email,
        }
      : undefined,
  }
}

function buildAdminTicketMessageSummary(msg: TicketMessage) {
  const orderId = resolveTicketMessageOrderID(msg)
  return {
    id: msg.id,
    ticket_id: msg.ticket_id,
    sender_type: msg.sender_type,
    sender_id: msg.sender_id,
    sender_name: msg.sender_name,
    content: msg.content,
    content_type: msg.content_type,
    order_id: orderId || undefined,
    is_read_by_user: msg.is_read_by_user,
    is_read_by_admin: msg.is_read_by_admin,
    created_at: msg.created_at,
  }
}

function AdminTicketsPageContent() {
  const [selectedTicketId, setSelectedTicketId] = useState<number | null>(null)
  const [status, setStatus] = useState('')
  const [search, setSearch] = useState('')
  const [assignedTo, setAssignedTo] = useState('')
  const [message, setMessage] = useState('')
  const [viewingOrderId, setViewingOrderId] = useState<number | null>(null)
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const ticketListRef = useRef<HTMLDivElement>(null)
  const sentinelRef = useRef<HTMLDivElement>(null)
  const queryClient = useQueryClient()
  const toast = useToast()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.adminTickets)
  const resolveTicketError = (error: unknown, fallback: string) =>
    resolveApiErrorMessage(error, t, fallback)
  const deferredSearch = useDeferredValue(search)

  // 获取工单列表 - 无状态筛选时: 自动加载所有非关闭工单 + 分页加载关闭工单
  const {
    data: nonClosedInfiniteData,
    fetchNextPage: fetchMoreNonClosed,
    hasNextPage: hasMoreNonClosed,
    isFetchingNextPage: loadingMoreNonClosed,
    isLoading: nonClosedInitialLoading,
  } = useInfiniteQuery({
    queryKey: ['adminTickets', 'nonClosed', deferredSearch, assignedTo],
    queryFn: ({ pageParam }) =>
      getAdminTickets({
        exclude_status: 'closed',
        page: pageParam,
        limit: 100,
        search: deferredSearch || undefined,
        assigned_to: assignedTo || undefined,
      }),
    getNextPageParam: (lastPage: any) => {
      const pagination = lastPage?.data?.pagination
      return pagination?.has_next ? pagination.page + 1 : undefined
    },
    initialPageParam: 1,
    enabled: !status,
  })

  // 自动加载所有非关闭工单页
  useEffect(() => {
    if (!status && hasMoreNonClosed && !loadingMoreNonClosed) {
      fetchMoreNonClosed()
    }
  }, [status, hasMoreNonClosed, loadingMoreNonClosed, fetchMoreNonClosed])

  const nonClosedFullyLoaded =
    !nonClosedInitialLoading && !loadingMoreNonClosed && hasMoreNonClosed === false

  const {
    data: closedInfiniteData,
    fetchNextPage: fetchMoreClosed,
    hasNextPage: hasMoreClosed,
    isFetchingNextPage: loadingMoreClosed,
    isLoading: closedInitialLoading,
  } = useInfiniteQuery({
    queryKey: ['adminTickets', 'closed', deferredSearch, assignedTo],
    queryFn: ({ pageParam }) =>
      getAdminTickets({
        status: 'closed',
        page: pageParam,
        limit: 20,
        search: deferredSearch || undefined,
        assigned_to: assignedTo || undefined,
      }),
    getNextPageParam: (lastPage: any) => {
      const pagination = lastPage?.data?.pagination
      return pagination?.has_next ? pagination.page + 1 : undefined
    },
    initialPageParam: 1,
    enabled: !status && nonClosedFullyLoaded,
  })

  // 有状态筛选时: 分页加载该状态工单
  const {
    data: filteredInfiniteData,
    fetchNextPage: fetchMoreFiltered,
    hasNextPage: hasMoreFiltered,
    isFetchingNextPage: loadingMoreFiltered,
    isLoading: filteredInitialLoading,
  } = useInfiniteQuery({
    queryKey: ['adminTickets', 'filtered', status, deferredSearch, assignedTo],
    queryFn: ({ pageParam }) =>
      getAdminTickets({
        status: status || undefined,
        page: pageParam,
        limit: 20,
        search: deferredSearch || undefined,
        assigned_to: assignedTo || undefined,
      }),
    getNextPageParam: (lastPage: any) => {
      const pagination = lastPage?.data?.pagination
      return pagination?.has_next ? pagination.page + 1 : undefined
    },
    initialPageParam: 1,
    enabled: !!status,
  })

  // 获取统计
  const { data: statsData } = useQuery({
    queryKey: ['ticketStats'],
    queryFn: getTicketStats,
  })

  // 获取选中工单详情
  const { data: ticketData } = useQuery({
    queryKey: ['adminTicket', selectedTicketId],
    queryFn: () => getAdminTicket(selectedTicketId!),
    enabled: !!selectedTicketId,
  })

  // 获取消息列表
  const {
    data: messagesData,
    isLoading: messagesLoading,
    isError: messagesLoadFailed,
    refetch: refetchMessages,
  } = useQuery({
    queryKey: ['adminTicketMessages', selectedTicketId],
    queryFn: () => getAdminTicketMessages(selectedTicketId!),
    enabled: !!selectedTicketId,
    refetchInterval: 5000,
    refetchIntervalInBackground: false,
  })

  // 获取分享的订单
  const { data: sharedOrdersData } = useQuery({
    queryKey: ['adminTicketSharedOrders', selectedTicketId],
    queryFn: () => getAdminTicketSharedOrders(selectedTicketId!),
    enabled: !!selectedTicketId,
  })

  // 获取分享的订单详情
  const { data: sharedOrderDetailData, isLoading: orderDetailLoading } = useQuery({
    queryKey: ['adminTicketSharedOrder', selectedTicketId, viewingOrderId],
    queryFn: () => getAdminTicketSharedOrder(selectedTicketId!, viewingOrderId!),
    enabled: !!selectedTicketId && !!viewingOrderId,
  })

  // 获取工单附件配置
  const { data: publicConfigData } = useQuery({
    queryKey: ['publicConfig'],
    queryFn: getPublicConfig,
    staleTime: 5 * 60 * 1000,
  })

  // 发送消息
  const sendMessageMutation = useMutation({
    mutationFn: (content: string) => sendAdminTicketMessage(selectedTicketId!, { content }),
    onSuccess: () => {
      setMessage('')
      queryClient.invalidateQueries({ queryKey: ['adminTicketMessages', selectedTicketId] })
      queryClient.invalidateQueries({ queryKey: ['adminTicket', selectedTicketId] })
      queryClient.invalidateQueries({ queryKey: ['adminTickets'] })
    },
    onError: (error: unknown) => {
      toast.error(resolveTicketError(error, t.ticket.sendFailed))
    },
  })

  // 更新工单
  const updateTicketMutation = useMutation({
    mutationFn: (data: { status?: string; priority?: string }) =>
      updateAdminTicket(selectedTicketId!, data),
    onSuccess: () => {
      toast.success(t.ticket.updateSuccess)
      queryClient.invalidateQueries({ queryKey: ['adminTicket', selectedTicketId] })
      queryClient.invalidateQueries({ queryKey: ['adminTickets'] })
      queryClient.invalidateQueries({ queryKey: ['ticketStats'] })
    },
    onError: (error: unknown) => {
      toast.error(resolveTicketError(error, t.ticket.updateFailed))
    },
  })

  const tickets: Ticket[] = useMemo(() => {
    if (status) {
      return filteredInfiniteData?.pages.flatMap((p: any) => p?.data?.items || []) || []
    }
    const nonClosed = nonClosedInfiniteData?.pages.flatMap((p: any) => p?.data?.items || []) || []
    const closed = closedInfiniteData?.pages.flatMap((p: any) => p?.data?.items || []) || []
    return [...nonClosed, ...closed]
  }, [status, nonClosedInfiniteData, closedInfiniteData, filteredInfiniteData])

  const ticketsLoading = status ? filteredInitialLoading : nonClosedInitialLoading
  const hasMore = status ? (hasMoreFiltered ?? false) : (hasMoreClosed ?? false)
  const loadingMore = status ? loadingMoreFiltered : loadingMoreNonClosed || loadingMoreClosed
  const loadMore = useCallback(() => {
    if (status) {
      fetchMoreFiltered()
    } else {
      fetchMoreClosed()
    }
  }, [status, fetchMoreFiltered, fetchMoreClosed])
  const stats = statsData?.data
  const selectedTicket = ticketData?.data
  const messages: TicketMessage[] = messagesData?.data || []
  const sharedOrders = sharedOrdersData?.data || []
  const ticketAttachment = publicConfigData?.data?.ticket?.attachment
  const maxContentLength = publicConfigData?.data?.ticket?.max_content_length || 0
  const activeTicketFilters: string[] = []
  if (deferredSearch.trim()) {
    activeTicketFilters.push(`${t.common.search}: ${deferredSearch.trim()}`)
  }
  if (status) {
    activeTicketFilters.push(
      `${t.ticket.status}: ${
        t.ticket.ticketStatus[status as keyof typeof t.ticket.ticketStatus] || status
      }`
    )
  }
  if (assignedTo) {
    activeTicketFilters.push(
      `${t.ticket.assign}: ${
        assignedTo === 'me'
          ? t.ticket.assignMe
          : assignedTo === 'unassigned'
            ? t.ticket.assignUnassigned
            : assignedTo
      }`
    )
  }
  const adminTicketsPluginContext = {
    view: 'admin_tickets',
    filters: {
      search: deferredSearch || undefined,
      status: status || undefined,
      assigned_to: assignedTo || undefined,
    },
    selection: {
      selected_ticket_id: selectedTicketId || undefined,
      viewing_order_id: viewingOrderId || undefined,
    },
    pagination: {
      total_loaded: tickets.length,
      has_more: hasMore,
      loading_more: loadingMore,
    },
    summary: {
      active_filter_count: activeTicketFilters.length,
      selected_message_count: messages.length,
      selected_shared_order_count: sharedOrders.length,
      stats: stats || null,
    },
  }
  const adminTicketRowActionItems = tickets.map((ticket, index) => ({
    key: String(ticket.id),
    slot: 'admin.tickets.row_actions',
    path: '/admin/tickets',
    hostContext: {
      view: 'admin_tickets_row',
      ticket: buildAdminTicketSummary(ticket),
      row: {
        index: index + 1,
        selected: selectedTicketId === ticket.id,
      },
      filters: adminTicketsPluginContext.filters,
      summary: adminTicketsPluginContext.summary,
    },
  }))
  const adminTicketRowActionExtensions = usePluginExtensionBatch({
    scope: 'admin',
    path: '/admin/tickets',
    items: adminTicketRowActionItems,
    enabled: tickets.length > 0,
  })
  const adminTicketDetailPluginContext = selectedTicket
    ? {
        view: 'admin_ticket_detail',
        ticket: buildAdminTicketSummary(selectedTicket),
        composer: {
          draft_length: message.length,
          can_reply: selectedTicket.status !== 'closed',
          max_content_length: maxContentLength || undefined,
        },
        shared_orders: {
          count: sharedOrders.length,
          viewing_order_id: viewingOrderId || undefined,
        },
        messages: {
          count: messages.length,
        },
        state: {
          is_closed: selectedTicket.status === 'closed',
          messages_loading: messagesLoading,
          messages_load_failed: messagesLoadFailed,
          messages_empty: !messagesLoading && !messagesLoadFailed && messages.length === 0,
        },
      }
    : null
  const adminTicketMessageActionItems =
    selectedTicket && messages.length > 0
      ? messages.map((msg, index) => ({
          key: String(msg.id),
          slot: 'admin.ticket_detail.message_actions',
          path: '/admin/tickets',
          hostContext: {
            view: 'admin_ticket_detail_message',
            ticket: buildAdminTicketSummary(selectedTicket),
            message: buildAdminTicketMessageSummary(msg),
            row: {
              index: index + 1,
              is_admin_message: msg.sender_type === 'admin',
            },
            messages: {
              count: messages.length,
            },
            shared_orders: {
              count: sharedOrders.length,
            },
          },
        }))
      : []
  const adminTicketMessageActionExtensions = usePluginExtensionBatch({
    scope: 'admin',
    path: '/admin/tickets',
    items: adminTicketMessageActionItems,
    enabled: adminTicketMessageActionItems.length > 0,
  })
  // 当选中工单并获取消息后，刷新工单列表以更新未读状态
  useEffect(() => {
    if (selectedTicketId && messages.length > 0) {
      // 延迟刷新，确保后端已处理已读状态
      const timer = setTimeout(() => {
        queryClient.invalidateQueries({ queryKey: ['adminTickets'] })
        queryClient.invalidateQueries({ queryKey: ['ticketStats'] })
      }, 500)
      return () => clearTimeout(timer)
    }
  }, [selectedTicketId, messages.length, queryClient])

  // 滚动到底部
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages.length])

  // 工单列表无限滚动
  useEffect(() => {
    const sentinel = sentinelRef.current
    const container = ticketListRef.current
    if (!sentinel || !container) return

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting && hasMore && !loadingMore) {
          loadMore()
        }
      },
      { root: container, threshold: 0.1 }
    )

    observer.observe(sentinel)
    return () => observer.disconnect()
  }, [hasMore, loadingMore, loadMore])

  const handleSend = () => {
    if (!message.trim()) return
    if (maxContentLength > 0 && message.trim().length > maxContentLength) {
      toast.error(t.ticket.contentTooLong.replace('{max}', String(maxContentLength)))
      return
    }
    sendMessageMutation.mutate(message.trim())
  }

  const handleUploadFile = async (file: File): Promise<string> => {
    try {
      const res = await uploadAdminTicketFile(selectedTicketId!, file)
      return res.data?.url || res.data
    } catch (error) {
      toast.error(resolveTicketError(error, t.ticket.uploadFailed))
      throw error
    }
  }

  // 状态颜色映射（支持暗色模式）
  const statusColorMap: Record<string, string> = {
    open: 'bg-yellow-500/20 text-yellow-700 dark:text-yellow-400 border-yellow-500/30',
    processing: 'bg-blue-500/20 text-blue-700 dark:text-blue-400 border-blue-500/30',
    resolved: 'bg-green-500/20 text-green-700 dark:text-green-400 border-green-500/30',
    closed: 'bg-muted text-muted-foreground border-border',
  }

  // 优先级颜色映射（支持暗色模式）
  const priorityColorMap: Record<string, string> = {
    low: 'bg-muted text-muted-foreground border-border',
    normal: 'bg-blue-500/20 text-blue-700 dark:text-blue-400 border-blue-500/30',
    high: 'bg-orange-500/20 text-orange-700 dark:text-orange-400 border-orange-500/30',
    urgent: 'bg-red-500/20 text-red-700 dark:text-red-400 border-red-500/30',
  }

  const getStatusBadge = (status: string) => {
    const config = TICKET_STATUS_CONFIG[status as keyof typeof TICKET_STATUS_CONFIG]
    if (!config) return <Badge variant="secondary">{status}</Badge>
    const colorClass = statusColorMap[status] || ''
    const label =
      t.ticket.ticketStatus[status as keyof typeof t.ticket.ticketStatus] || config.label
    return <Badge className={colorClass}>{label}</Badge>
  }

  const getPriorityBadge = (priority: string) => {
    const config = TICKET_PRIORITY_CONFIG[priority as keyof typeof TICKET_PRIORITY_CONFIG]
    if (!config) return null
    const colorClass = priorityColorMap[priority] || ''
    const label =
      t.ticket.ticketPriority[priority as keyof typeof t.ticket.ticketPriority] || config.label
    return <Badge className={cn('text-xs', colorClass)}>{label}</Badge>
  }

  return (
    <div className="flex h-[calc(100vh-4rem)] flex-col">
      <PluginSlot
        slot="admin.tickets.top"
        context={adminTicketsPluginContext}
        className="px-4 pt-3"
      />
      {/* 顶部统计栏 - 更紧凑 */}
      <div className="flex flex-col gap-3 border-b px-4 py-2 md:flex-row md:items-center md:justify-between">
        <h1 className="text-xl font-bold">{t.ticket.ticketManagement}</h1>
        {stats && (
          <div className="flex flex-wrap gap-3 text-sm">
            <span>
              {t.ticket.total}: <strong>{stats.total}</strong>
            </span>
            <span className="text-yellow-600">
              {t.ticket.pending}: <strong>{stats.open}</strong>
            </span>
            <span className="text-blue-600">
              {t.ticket.processing}: <strong>{stats.processing}</strong>
            </span>
            <span className="text-red-600">
              {t.ticket.unread}: <strong>{stats.unread}</strong>
            </span>
          </div>
        )}
      </div>

      <div className="flex flex-1 overflow-hidden">
        {/* 左侧工单列表 - 缩窄 */}
        <div className="flex w-72 shrink-0 flex-col overflow-hidden border-r">
          {/* 筛选 */}
          <div className="space-y-2 border-b p-3">
            <div className="relative">
              <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                placeholder={t.ticket.searchPlaceholder}
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                className="pl-10"
              />
            </div>
            <div className="flex gap-2">
              <Select
                value={status || 'all'}
                onValueChange={(v) => setStatus(v === 'all' ? '' : v)}
              >
                <SelectTrigger className="flex-1">
                  <SelectValue placeholder={t.ticket.status} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">{t.ticket.allStatus}</SelectItem>
                  {Object.entries(TICKET_STATUS_CONFIG).map(([key, config]) => (
                    <SelectItem key={key} value={key}>
                      {t.ticket.ticketStatus[key as keyof typeof t.ticket.ticketStatus] ||
                        config.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Select
                value={assignedTo || 'all'}
                onValueChange={(v) => setAssignedTo(v === 'all' ? '' : v)}
              >
                <SelectTrigger className="flex-1">
                  <SelectValue placeholder={t.ticket.assign} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">{t.ticket.assignAll}</SelectItem>
                  <SelectItem value="me">{t.ticket.assignMe}</SelectItem>
                  <SelectItem value="unassigned">{t.ticket.assignUnassigned}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            {activeTicketFilters.length > 0 ? (
              <p className="text-xs text-muted-foreground">{activeTicketFilters.join(' · ')}</p>
            ) : null}
          </div>
          <PluginSlot
            slot="admin.tickets.before_list"
            context={adminTicketsPluginContext}
            className="px-3 pt-2"
          />

          {/* 工单列表 */}
          <div ref={ticketListRef} className="scrollbar-hide flex-1 overflow-y-auto">
            {ticketsLoading ? (
              <div className="py-8 text-center text-muted-foreground">{t.ticket.loading}</div>
            ) : tickets.length === 0 ? (
              <div className="py-8 text-center text-muted-foreground">{t.ticket.noTickets}</div>
            ) : (
              <>
                <div className="divide-y">
                  {tickets.map((ticket) => {
                    const rowExtensions = adminTicketRowActionExtensions[String(ticket.id)] || []
                    return (
                    <div
                      key={ticket.id}
                      className={cn(
                        'cursor-pointer p-3 transition-colors hover:bg-accent/50',
                        selectedTicketId === ticket.id && 'bg-accent'
                      )}
                      onClick={() => {
                        setSelectedTicketId(ticket.id)
                        queryClient.invalidateQueries({
                          queryKey: ['adminTicketMessages', ticket.id],
                        })
                      }}
                    >
                      <div className="min-w-0">
                        <div className="flex min-w-0 flex-wrap items-center gap-2">
                          <span className="min-w-0 flex-1 truncate text-sm font-medium">
                            {ticket.subject}
                          </span>
                          {getPriorityBadge(ticket.priority)}
                          {getStatusBadge(ticket.status)}
                          {ticket.unread_count_admin > 0 && (
                            <Badge variant="destructive" className="h-5 text-xs">
                              {ticket.unread_count_admin}
                            </Badge>
                          )}
                        </div>
                        <p className="mt-1 truncate text-xs text-muted-foreground">
                          {ticket.last_message_preview}
                        </p>
                        <div className="mt-1 flex items-center gap-2">
                          <span className="flex items-center gap-1 text-xs text-muted-foreground">
                            <User className="h-3 w-3" />
                            {ticket.user?.name || ticket.user?.email || 'Unknown'}
                          </span>
                          <span className="flex items-center gap-1 text-xs text-muted-foreground">
                            <Clock className="h-3 w-3" />
                            {ticket.last_message_at
                              ? formatDistanceToNow(new Date(ticket.last_message_at), {
                                  locale: locale === 'zh' ? zhCN : undefined,
                                })
                              : formatDistanceToNow(new Date(ticket.created_at), {
                                  locale: locale === 'zh' ? zhCN : undefined,
                                })}
                          </span>
                        </div>
                        {rowExtensions.length > 0 ? (
                          <div className="mt-2" onClick={(event) => event.stopPropagation()}>
                            <PluginExtensionList extensions={rowExtensions} display="inline" />
                          </div>
                        ) : null}
                      </div>
                    </div>
                  )})}
                </div>
                <div ref={sentinelRef} className="py-1 text-center">
                  {loadingMore && (
                    <span className="text-xs text-muted-foreground">{t.ticket.loading}</span>
                  )}
                </div>
              </>
            )}
          </div>
        </div>

        {/* 右侧聊天区域 */}
        <div className="flex min-w-0 flex-1 flex-col">
          {!selectedTicketId ? (
            <div className="flex flex-1 items-center justify-center px-6">
              <Card className="w-full max-w-xl border-dashed">
                <CardContent className="flex flex-col items-center justify-center px-6 py-12 text-center">
                  <div className="rounded-xl bg-muted p-3">
                    <Package className="h-6 w-6 text-muted-foreground" />
                  </div>
                  <div className="mt-4 text-base font-semibold">{t.ticket.selectTicket}</div>
                  <p className="mt-2 text-sm text-muted-foreground">{t.ticket.selectionHint}</p>
                </CardContent>
              </Card>
            </div>
          ) : (
            <>
              {/* 工单头部 - 更紧凑 */}
              {selectedTicket && (
                <div className="border-b px-3 py-2">
                  <PluginSlot
                    slot="admin.ticket_detail.top"
                    context={adminTicketDetailPluginContext || undefined}
                    className="mb-2"
                  />
                  <div className="flex items-center justify-between gap-2">
                    <div className="flex min-w-0 flex-1 items-center gap-2">
                      <h2 className="truncate font-semibold">{selectedTicket.subject}</h2>
                      {getPriorityBadge(selectedTicket.priority)}
                      {getStatusBadge(selectedTicket.status)}
                    </div>
                    <Select
                      value={selectedTicket.status}
                      onValueChange={(v) => updateTicketMutation.mutate({ status: v })}
                    >
                      <SelectTrigger className="h-8 w-28">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {Object.entries(TICKET_STATUS_CONFIG).map(([key, config]) => (
                          <SelectItem key={key} value={key}>
                            {t.ticket.ticketStatus[key as keyof typeof t.ticket.ticketStatus] ||
                              config.label}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <p className="mt-1 text-xs text-muted-foreground">
                    #{selectedTicket.ticket_no} |{' '}
                    {selectedTicket.user?.name || selectedTicket.user?.email} |{' '}
                    {format(new Date(selectedTicket.created_at), 'MM-dd HH:mm')}
                  </p>
                  <p className="mt-2 text-xs text-muted-foreground">
                    {[
                      selectedTicket.user?.name || selectedTicket.user?.email || t.ticket.user,
                      t.ticket.messageCount.replace('{count}', String(messages.length)),
                      t.ticket.sharedOrderCount.replace('{count}', String(sharedOrders.length)),
                      selectedTicket.unread_count_admin > 0
                        ? `${t.ticket.unread}: ${selectedTicket.unread_count_admin}`
                        : null,
                    ]
                      .filter(Boolean)
                      .join(' · ')}
                  </p>
                  {/* 分享的订单 */}
                  {sharedOrders.length > 0 && (
                    <div className="mt-1.5 flex flex-wrap items-center gap-1.5">
                      <span className="text-xs text-muted-foreground">{t.ticket.orders}:</span>
                      {sharedOrders.map((access: any) => (
                        <button
                          key={access.id}
                          onClick={() => setViewingOrderId(access.order?.id)}
                          className="inline-flex"
                        >
                          <Badge
                            variant="outline"
                            className="h-5 cursor-pointer text-xs transition-colors hover:bg-accent"
                          >
                            <Package className="mr-1 h-3 w-3" />
                            {access.order?.order_no}
                          </Badge>
                        </button>
                      ))}
                    </div>
                  )}
                </div>
              )}

              {/* 消息列表 */}
              <div className="scrollbar-hide flex-1 space-y-2 overflow-y-auto px-3 py-2">
                {messagesLoading ? (
                  <div className="py-8 text-center text-muted-foreground">
                    {t.ticket.loadingMessages}
                  </div>
                ) : messagesLoadFailed ? (
                  <div className="flex h-full min-h-[220px] flex-col items-center justify-center text-center">
                    <MessageSquare className="mb-4 h-10 w-10 text-muted-foreground" />
                    <p className="text-base font-medium text-foreground">
                      {t.ticket.messagesLoadFailed}
                    </p>
                    <p className="mt-2 max-w-sm text-sm text-muted-foreground">
                      {t.ticket.messagesLoadFailedDesc}
                    </p>
                    <Button className="mt-4" variant="outline" onClick={() => refetchMessages()}>
                      {t.common.refresh}
                    </Button>
                    <PluginSlot
                      slot="admin.ticket_detail.messages_load_failed"
                      context={{
                        ...(adminTicketDetailPluginContext || {}),
                        section: 'messages_state',
                      }}
                    />
                  </div>
                ) : messages.length === 0 ? (
                  <div className="flex h-full min-h-[220px] flex-col items-center justify-center text-center text-muted-foreground">
                    <MessageSquare className="mb-4 h-10 w-10" />
                    <p className="text-base font-medium text-foreground">{t.ticket.noMessagesYet}</p>
                    <p className="mt-2 max-w-sm text-sm text-muted-foreground">
                      {t.ticket.noMessagesYetDesc}
                    </p>
                    <PluginSlot
                      slot="admin.ticket_detail.empty"
                      context={{
                        ...(adminTicketDetailPluginContext || {}),
                        section: 'messages_state',
                      }}
                    />
                  </div>
                ) : (
                  <>
                    {messages.map((msg) => {
                      const orderId = resolveTicketMessageOrderID(msg)
                      const rowExtensions = adminTicketMessageActionExtensions[String(msg.id)] || []
                      return (
                        <div
                          key={msg.id}
                          className={cn(
                            'flex',
                            msg.sender_type === 'admin' ? 'justify-end' : 'justify-start'
                          )}
                        >
                          <div className="max-w-[80%]">
                            <div
                              className={cn(
                                'rounded-lg px-3 py-1.5',
                                msg.sender_type === 'admin'
                                  ? 'bg-primary text-primary-foreground'
                                  : 'bg-muted'
                              )}
                            >
                              <div className="mb-0.5 flex items-center gap-2">
                                <span className="text-xs font-medium">
                                  {msg.sender_type === 'admin'
                                    ? msg.sender_name || t.ticket.adminAgent
                                    : t.ticket.user}
                                </span>
                                <span className="text-xs opacity-70">
                                  {format(new Date(msg.created_at), 'MM-dd HH:mm', {
                                    locale: locale === 'zh' ? zhCN : undefined,
                                  })}
                                </span>
                                {/* 已读/未读状态 */}
                                {msg.sender_type === 'admin' && (
                                  <span className="text-xs opacity-70">
                                    {msg.is_read_by_user ? (
                                      <CheckCheck className="inline h-3 w-3" />
                                    ) : (
                                      <Check className="inline h-3 w-3" />
                                    )}
                                  </span>
                                )}
                              </div>
                              {msg.content_type === 'order' ? (
                                orderId ? (
                                  <button
                                    onClick={() => setViewingOrderId(orderId)}
                                    className="flex items-center gap-2 text-sm hover:underline"
                                  >
                                    <Package className="h-4 w-4" />
                                    <span>{msg.content}</span>
                                  </button>
                                ) : (
                                  <div className="flex items-center gap-2 text-sm">
                                    <Package className="h-4 w-4" />
                                    <span>{msg.content}</span>
                                  </div>
                                )
                              ) : (
                                <MarkdownMessage
                                  content={msg.content}
                                  isOwnMessage={msg.sender_type === 'admin'}
                                />
                              )}
                            </div>
                            {rowExtensions.length > 0 ? (
                              <div className="mt-2 px-1">
                                <PluginExtensionList extensions={rowExtensions} display="inline" />
                              </div>
                            ) : null}
                          </div>
                        </div>
                      )
                    })}
                    <div ref={messagesEndRef} />
                  </>
                )}
              </div>

              {/* 输入框 */}
              {selectedTicket?.status !== 'closed' ? (
                <div className="shrink-0 border-t bg-background p-2 md:p-3">
                  <PluginSlot
                    slot="admin.ticket_detail.composer.top"
                    context={{
                      ...(adminTicketDetailPluginContext || {}),
                      section: 'composer',
                    }}
                  />
                  <MessageToolbar
                    value={message}
                    onChange={setMessage}
                    onSend={handleSend}
                    onUploadFile={handleUploadFile}
                    isSending={sendMessageMutation.isPending}
                    placeholder={t.ticket.adminMessagePlaceholder}
                    enableImage={ticketAttachment?.enable_image ?? true}
                    enableVoice={ticketAttachment?.enable_voice ?? true}
                    acceptImageTypes={ticketAttachment?.allowed_image_types}
                    maxLength={maxContentLength > 0 ? maxContentLength : undefined}
                    pluginSlotNamespace="admin.ticket_detail.composer"
                    pluginSlotContext={{
                      ...(adminTicketDetailPluginContext || {}),
                      section: 'composer',
                    }}
                    pluginSlotPath="/admin/tickets"
                    translations={{
                      messagePlaceholder: t.ticket.adminMessagePlaceholder,
                      uploadImage: t.ticket.uploadImage,
                      recordVoice: t.ticket.recordVoice,
                      recording: t.ticket.recording,
                      recordingTip: t.ticket.recordingTip,
                      voiceMessage: t.ticket.voiceMessage,
                      bold: t.ticket.bold,
                      italic: t.ticket.italic,
                      code: t.ticket.code,
                      list: t.ticket.list,
                      link: t.ticket.link,
                      preview: t.ticket.preview,
                      editMode: t.ticket.editMode,
                      send: t.ticket.send,
                      noPreviewContent: t.ticket.noPreviewContent,
                    }}
                  />
                </div>
              ) : (
                <div className="shrink-0 border-t p-2 text-center text-sm text-muted-foreground md:p-3">
                  <PluginSlot
                    slot="admin.ticket_detail.composer.top"
                    context={{
                      ...(adminTicketDetailPluginContext || {}),
                      section: 'composer',
                    }}
                  />
                  {t.ticket.ticketClosed}
                </div>
              )}
            </>
          )}
        </div>
      </div>
      <PluginSlot
        slot="admin.tickets.bottom"
        context={adminTicketsPluginContext}
        className="px-4 pb-3"
      />

      {/* 订单详情对话框 */}
      <Dialog open={!!viewingOrderId} onOpenChange={(open) => !open && setViewingOrderId(null)}>
        <DialogContent className="max-h-[85vh] max-w-2xl overflow-y-auto">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <Package className="h-5 w-5" />
              {t.ticket.orderDetail}
            </DialogTitle>
          </DialogHeader>

          {orderDetailLoading ? (
            <div className="py-8 text-center text-muted-foreground">{t.ticket.loading}</div>
          ) : sharedOrderDetailData?.data?.order ? (
            (() => {
              const order = sharedOrderDetailData.data.order
              return (
                <div className="space-y-4">
                  {/* 订单基本信息 */}
                  <Card>
                    <CardHeader className="py-3">
                      <CardTitle className="text-base">{t.ticket.orderInfo}</CardTitle>
                    </CardHeader>
                    <CardContent className="py-2">
                      <dl className="grid grid-cols-2 gap-2 text-sm">
                        <div>
                          <dt className="text-muted-foreground">{t.ticket.orderNo}</dt>
                          <dd className="font-mono font-medium">{order.order_no}</dd>
                        </div>
                        <div>
                          <dt className="text-muted-foreground">{t.ticket.orderStatus}</dt>
                          <dd>
                            <Badge variant="outline">{order.status}</Badge>
                          </dd>
                        </div>
                        <div>
                          <dt className="text-muted-foreground">{t.ticket.amount}</dt>
                          <dd className="font-semibold text-primary">
                            {formatCurrency(order.total_amount_minor ?? 0, order.currency)}
                          </dd>
                        </div>
                        <div>
                          <dt className="text-muted-foreground">{t.ticket.createdAt}</dt>
                          <dd>{formatDate(order.created_at)}</dd>
                        </div>
                        {order.external_user_name && (
                          <div>
                            <dt className="text-muted-foreground">{t.ticket.platformUser}</dt>
                            <dd>{order.external_user_name}</dd>
                          </div>
                        )}
                        {order.user_email && (
                          <div>
                            <dt className="text-muted-foreground">{t.ticket.email}</dt>
                            <dd>{order.user_email}</dd>
                          </div>
                        )}
                      </dl>
                    </CardContent>
                  </Card>

                  {/* 商品信息 */}
                  <Card>
                    <CardHeader className="py-3">
                      <CardTitle className="flex items-center gap-2 text-base">
                        <Package className="h-4 w-4" />
                        {t.ticket.productInfo}
                      </CardTitle>
                    </CardHeader>
                    <CardContent className="py-2">
                      <div className="space-y-3">
                        {order.items?.map((item: any, index: number) => (
                          <div key={index} className="flex gap-3">
                            {item.image_url ? (
                              <>
                                {/* Arbitrary remote product images may not be present in Next image allowlists. */}
                                {/* eslint-disable-next-line @next/next/no-img-element */}
                                <img
                                  src={item.image_url}
                                  alt={item.name}
                                  className="h-14 w-14 rounded bg-muted object-cover"
                                  loading="lazy"
                                  decoding="async"
                                  onError={(e) => {
                                    e.currentTarget.style.display = 'none'
                                    e.currentTarget.parentElement
                                      ?.querySelector('.img-fallback')
                                      ?.classList.remove('hidden')
                                  }}
                                />
                              </>
                            ) : null}
                            <div
                              className={`img-fallback flex h-14 w-14 items-center justify-center rounded bg-muted ${item.image_url ? 'hidden' : ''}`}
                            >
                              <Package className="h-6 w-6 text-muted-foreground" />
                            </div>
                            <div className="min-w-0 flex-1">
                              <p className="truncate text-sm font-medium">{item.name}</p>
                              <p className="text-xs text-muted-foreground">SKU: {item.sku}</p>
                              <p className="text-xs text-muted-foreground">x{item.quantity}</p>
                            </div>
                          </div>
                        ))}
                      </div>
                    </CardContent>
                  </Card>

                  {/* 收货信息 */}
                  {(order.receiver_name || order.receiver_phone || order.receiver_address) && (
                    <Card>
                      <CardHeader className="py-3">
                        <CardTitle className="flex items-center gap-2 text-base">
                          <MapPin className="h-4 w-4" />
                          {t.ticket.shippingInfo}
                        </CardTitle>
                      </CardHeader>
                      <CardContent className="py-2">
                        <dl className="space-y-2 text-sm">
                          {order.receiver_name && (
                            <div className="flex">
                              <dt className="w-20 shrink-0 text-muted-foreground">
                                {t.ticket.recipient}
                              </dt>
                              <dd>{order.receiver_name}</dd>
                            </div>
                          )}
                          {order.receiver_phone && (
                            <div className="flex">
                              <dt className="w-20 shrink-0 text-muted-foreground">
                                {t.ticket.phone}
                              </dt>
                              <dd>
                                {order.phone_code || ''}
                                {order.receiver_phone}
                              </dd>
                            </div>
                          )}
                          {order.receiver_email && (
                            <div className="flex">
                              <dt className="w-20 shrink-0 text-muted-foreground">
                                {t.ticket.email}
                              </dt>
                              <dd className="break-all">{order.receiver_email}</dd>
                            </div>
                          )}
                          {(order.receiver_province ||
                            order.receiver_city ||
                            order.receiver_address) && (
                            <div className="flex">
                              <dt className="w-20 shrink-0 text-muted-foreground">
                                {t.ticket.address}
                              </dt>
                              <dd className="break-all">
                                {order.receiver_province} {order.receiver_city}{' '}
                                {order.receiver_district} {order.receiver_address}
                                {order.receiver_postcode && ` (${order.receiver_postcode})`}
                              </dd>
                            </div>
                          )}
                        </dl>
                      </CardContent>
                    </Card>
                  )}

                  {/* 物流信息 */}
                  {order.tracking_no && (
                    <Card>
                      <CardHeader className="py-3">
                        <CardTitle className="flex items-center gap-2 text-base">
                          <Truck className="h-4 w-4" />
                          {t.ticket.logisticsInfo}
                        </CardTitle>
                      </CardHeader>
                      <CardContent className="py-2">
                        <dl className="space-y-2 text-sm">
                          <div className="flex">
                            <dt className="w-20 shrink-0 text-muted-foreground">
                              {t.ticket.trackingNo}
                            </dt>
                            <dd className="font-mono">{order.tracking_no}</dd>
                          </div>
                          {order.shipped_at && (
                            <div className="flex">
                              <dt className="w-20 shrink-0 text-muted-foreground">
                                {t.ticket.shippedAt}
                              </dt>
                              <dd>{formatDate(order.shipped_at)}</dd>
                            </div>
                          )}
                        </dl>
                      </CardContent>
                    </Card>
                  )}

                  {/* 备注 */}
                  {order.remark && (
                    <Card>
                      <CardHeader className="py-3">
                        <CardTitle className="text-base">{t.ticket.userRemark}</CardTitle>
                      </CardHeader>
                      <CardContent className="py-2">
                        <p className="whitespace-pre-wrap text-sm">{order.remark}</p>
                      </CardContent>
                    </Card>
                  )}
                </div>
              )
            })()
          ) : (
            <div className="py-8 text-center text-muted-foreground">
              {t.ticket.orderNotAccessible}
            </div>
          )}
        </DialogContent>
      </Dialog>
    </div>
  )
}

export default function AdminTicketsPage() {
  return (
    <Suspense fallback={<div className="h-[calc(100vh-4rem)]" />}>
      <AdminTicketsPageContent />
    </Suspense>
  )
}
