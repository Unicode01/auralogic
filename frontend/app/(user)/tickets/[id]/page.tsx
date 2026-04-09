'use client'

import { useState, useEffect, useRef } from 'react'
import { useParams } from 'next/navigation'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  getTicket,
  getTicketMessages,
  sendTicketMessage,
  updateTicketStatus,
  shareOrderToTicket,
  getTicketSharedOrders,
  getOrders,
  uploadTicketFile,
  getPublicConfig,
  TicketMessage,
} from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import { MarkdownMessage } from '@/components/ui/markdown-message'
import { Skeleton } from '@/components/ui/page-loading'
import { MessageToolbar } from '@/components/ticket/message-toolbar'
import {
  Dialog,
  DialogContent,
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
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  ArrowLeft,
  Package,
  CheckCircle,
  XCircle,
  Share2,
  Check,
  CheckCheck,
  MoreVertical,
  MessageSquare,
} from 'lucide-react'
import { useToast } from '@/hooks/use-toast'
import { TICKET_STATUS_CONFIG } from '@/lib/constants'
import Link from 'next/link'
import { format } from 'date-fns'
import { zhCN } from 'date-fns/locale'
import { cn } from '@/lib/utils'
import { useLocale } from '@/hooks/use-locale'
import { useIsMobile } from '@/hooks/use-mobile'
import { usePageTitle } from '@/hooks/use-page-title'
import { getTranslations } from '@/lib/i18n'
import { buildListReturnPath, readListBrowseState } from '@/lib/list-browse-state'
import { resolveApiErrorMessage } from '@/lib/api-error'
import { PluginExtensionList } from '@/components/plugins/plugin-extension-list'
import { PluginSlot } from '@/components/plugins/plugin-slot'
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

function buildUserTicketMessageSummary(msg: TicketMessage) {
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

export default function TicketDetailPage() {
  const params = useParams()
  const ticketId = Number(params.id)
  const [message, setMessage] = useState('')
  const [openShare, setOpenShare] = useState(false)
  const [selectedOrder, setSelectedOrder] = useState<number | null>(null)
  const [ticketListBackHref, setTicketListBackHref] = useState('/tickets')
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const queryClient = useQueryClient()
  const toast = useToast()
  const { locale } = useLocale()
  const { isMobile, mounted } = useIsMobile()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.ticketDetail)
  const isCompactLayout = mounted ? isMobile : false

  const {
    data: ticketData,
    isLoading: ticketLoading,
    isError: ticketLoadFailed,
    refetch: refetchTicket,
  } = useQuery({
    queryKey: ['ticket', ticketId],
    queryFn: () => getTicket(ticketId),
    enabled: !!ticketId,
  })

  const {
    data: messagesData,
    isLoading: messagesLoading,
    isError: messagesLoadFailed,
    refetch: refetchMessages,
  } = useQuery({
    queryKey: ['ticketMessages', ticketId],
    queryFn: () => getTicketMessages(ticketId),
    enabled: !!ticketId,
    refetchInterval: 5000, // 5秒轮询
  })

  const { data: ordersData } = useQuery({
    queryKey: ['userOrders'],
    queryFn: () => getOrders({ limit: 50 }),
  })

  const { data: sharedOrdersData } = useQuery({
    queryKey: ['ticketSharedOrders', ticketId],
    queryFn: () => getTicketSharedOrders(ticketId),
    enabled: !!ticketId,
  })

  const { data: publicConfigData } = useQuery({
    queryKey: ['publicConfig'],
    queryFn: getPublicConfig,
    staleTime: 5 * 60 * 1000,
  })

  const sendMessageMutation = useMutation({
    mutationFn: (content: string) => sendTicketMessage(ticketId, { content }),
    onSuccess: () => {
      setMessage('')
      queryClient.invalidateQueries({ queryKey: ['ticketMessages', ticketId] })
      queryClient.invalidateQueries({ queryKey: ['ticket', ticketId] })
    },
    onError: (error: any) => {
      toast.error(resolveApiErrorMessage(error, t, t.ticket.sendFailed))
    },
  })

  const updateStatusMutation = useMutation({
    mutationFn: (status: string) => updateTicketStatus(ticketId, status),
    onSuccess: () => {
      toast.success(t.ticket.statusUpdateSuccess)
      queryClient.invalidateQueries({ queryKey: ['ticket', ticketId] })
      queryClient.invalidateQueries({ queryKey: ['userTickets'] })
    },
    onError: (error: any) => {
      toast.error(resolveApiErrorMessage(error, t, t.ticket.statusUpdateFailed))
    },
  })

  const shareOrderMutation = useMutation({
    mutationFn: () =>
      shareOrderToTicket(ticketId, {
        order_id: selectedOrder!,
      }),
    onSuccess: () => {
      toast.success(t.ticket.shareSuccess)
      setOpenShare(false)
      setSelectedOrder(null)
      queryClient.invalidateQueries({ queryKey: ['ticketMessages', ticketId] })
      queryClient.invalidateQueries({ queryKey: ['ticketSharedOrders', ticketId] })
    },
    onError: (error: any) => {
      toast.error(resolveApiErrorMessage(error, t, t.ticket.shareFailed))
    },
  })

  const ticket = ticketData?.data
  const messages: TicketMessage[] = messagesData?.data || []
  const orders = ordersData?.data?.items || []
  const sharedOrders = sharedOrdersData?.data || []
  const sharedOrderIds = new Set(sharedOrders.map((s: any) => s.order_id))
  const ticketAttachment = publicConfigData?.data?.ticket?.attachment
  const maxContentLength = publicConfigData?.data?.ticket?.max_content_length || 0
  const ticketDetailPath = ticketId ? `/tickets/${ticketId}` : '/tickets'
  const isClosed = ticket?.status === 'closed'
  const userTicketDetailPluginContext = {
    view: 'user_ticket_detail',
    ticket: ticket
      ? {
          id: ticket.id,
          subject: ticket.subject,
          status: ticket.status,
          created_at: ticket.created_at,
        }
      : {
          id: ticketId || undefined,
        },
    composer: {
      draft_length: message.length,
      can_reply: !isClosed,
      max_content_length: maxContentLength || undefined,
    },
    summary: {
      message_count: messages.length,
      shared_order_count: sharedOrders.length,
      is_closed: isClosed,
      max_content_length: maxContentLength || undefined,
      attachment_enabled: Boolean(ticketAttachment),
    },
    state: {
      ticket_load_failed: ticketLoadFailed && !ticket,
      ticket_not_found: !ticketLoadFailed && !ticketLoading && !ticket,
      messages_loading: messagesLoading,
      messages_load_failed: messagesLoadFailed,
      messages_empty: !messagesLoading && !messagesLoadFailed && messages.length === 0,
    },
  }
  const userTicketMessageActionItems = ticket
    ? messages.map((msg, index) => ({
        key: String(msg.id),
        slot: 'user.ticket_detail.message_actions',
        path: ticketDetailPath,
        hostContext: {
          view: 'user_ticket_detail_message',
          ticket: userTicketDetailPluginContext.ticket,
          summary: userTicketDetailPluginContext.summary,
          message: buildUserTicketMessageSummary(msg),
          row: {
            index: index + 1,
            is_user_message: msg.sender_type === 'user',
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
  const userTicketMessageActionExtensions = usePluginExtensionBatch({
    scope: 'public',
    path: ticketDetailPath,
    items: userTicketMessageActionItems,
    enabled: userTicketMessageActionItems.length > 0,
  })

  useEffect(() => {
    const browseState = readListBrowseState('tickets')
    setTicketListBackHref(buildListReturnPath('tickets', browseState?.listPath, String(ticketId)))
  }, [ticketId])

  // 当获取消息后，刷新工单列表以更新未读状态
  useEffect(() => {
    if (ticketId && messages.length > 0) {
      const timer = setTimeout(() => {
        queryClient.invalidateQueries({ queryKey: ['userTickets'] })
      }, 500)
      return () => clearTimeout(timer)
    }
  }, [ticketId, messages.length, queryClient])

  // 滚动到底部
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages.length])

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
      const res = await uploadTicketFile(ticketId, file)
      return res.data?.url || res.data
    } catch (error) {
      toast.error(resolveApiErrorMessage(error, t, t.ticket.uploadFailed))
      throw error
    }
  }

  const getStatusBadge = (status: string) => {
    const config = TICKET_STATUS_CONFIG[status as keyof typeof TICKET_STATUS_CONFIG]
    if (!config) return <Badge variant="secondary">{status}</Badge>
    const label =
      t.ticket.ticketStatus[status as keyof typeof t.ticket.ticketStatus] || config.label
    return <Badge variant={config.color as any}>{label}</Badge>
  }

  if (ticketLoading) {
    return (
      <div
        className={cn(
          'flex flex-col space-y-2',
          isCompactLayout ? 'h-[calc(100vh-6rem)]' : 'h-[calc(100vh-4rem)]'
        )}
      >
        <div
          className={cn(
            'flex shrink-0 items-center justify-between border-b',
            isCompactLayout ? 'px-2 py-1.5' : 'px-3 py-2'
          )}
        >
          <div className="flex min-w-0 items-center gap-2">
            <Skeleton className="h-8 w-8 rounded-md" />
            <div className="space-y-1">
              <Skeleton className="h-4 w-40" />
              <Skeleton className="h-3 w-20" />
            </div>
          </div>
          <Skeleton className="h-8 w-8 rounded-md" />
        </div>
        <div
          className={cn(
            'grid shrink-0 gap-2 border-b bg-muted/15 px-3 py-2',
            !isCompactLayout && 'md:grid-cols-4'
          )}
        >
          {[...Array(4)].map((_, index) => (
            <div key={index} className="rounded-lg border bg-background px-3 py-2 text-sm">
              <Skeleton className="h-3 w-20" />
              <Skeleton className="mt-2 h-4 w-16" />
            </div>
          ))}
        </div>
        <div className="flex-1 space-y-3 overflow-hidden px-3 py-2">
          {[...Array(4)].map((_, index) => (
            <div
              key={index}
              className={cn('flex', index % 2 === 0 ? 'justify-start' : 'justify-end')}
            >
              <div className="max-w-[85%] space-y-2 rounded-lg border bg-muted/30 px-3 py-2">
                <Skeleton className="h-3 w-24" />
                <Skeleton className="h-4 w-56 max-w-[70vw]" />
                <Skeleton className="h-4 w-40 max-w-[50vw]" />
              </div>
            </div>
          ))}
        </div>
        <div
          className={cn(
            'shrink-0 border-t',
            isCompactLayout ? 'px-2 py-1.5' : 'px-3 py-2'
          )}
        >
          <Skeleton className="h-24 w-full rounded-lg" />
        </div>
      </div>
    )
  }

  if (ticketLoadFailed && !ticket) {
    return (
      <Card className="border-dashed bg-muted/15">
        <CardContent className="flex flex-col items-center justify-center py-16 text-center">
          <MessageSquare className="mb-4 h-10 w-10 text-muted-foreground" />
          <p className="text-base font-medium">{t.ticket.ticketDetailLoadFailed}</p>
          <p className="mt-2 max-w-md text-sm text-muted-foreground">
            {t.ticket.ticketDetailLoadFailedDesc}
          </p>
          <div className="mt-4 flex flex-wrap justify-center gap-2">
            <Button variant="outline" onClick={() => refetchTicket()}>
              {t.common.refresh}
            </Button>
            <Button variant="ghost" asChild>
              <Link href={ticketListBackHref}>{t.common.back}</Link>
            </Button>
          </div>
          <PluginSlot
            slot="user.ticket_detail.load_failed"
            context={{ ...userTicketDetailPluginContext, section: 'ticket_state' }}
          />
        </CardContent>
      </Card>
    )
  }

  if (!ticket) {
    return (
      <Card>
        <CardContent className="flex flex-col items-center justify-center py-16 text-center">
          <Package className="mb-4 h-10 w-10 text-muted-foreground" />
          <p className="text-base font-medium">{t.ticket.ticketNotFound}</p>
          <p className="mt-2 text-sm text-muted-foreground">{t.ticket.ticketNotFoundDesc}</p>
          <Button variant="outline" asChild className="mt-4">
            <Link href={ticketListBackHref}>
              <ArrowLeft className="mr-2 h-4 w-4" />
              {t.common.back}
            </Link>
          </Button>
          <PluginSlot
            slot="user.ticket_detail.not_found"
            context={{ ...userTicketDetailPluginContext, section: 'ticket_state' }}
          />
        </CardContent>
      </Card>
    )
  }

  return (
    <div
      className={cn(
        'flex flex-col',
        isCompactLayout ? 'h-[calc(100vh-6rem)]' : 'h-[calc(100vh-4rem)]'
      )}
    >
      <PluginSlot slot="user.ticket_detail.top" context={userTicketDetailPluginContext} />
      {/* 头部 - 更紧凑 */}
      <div
        className={cn(
          'flex shrink-0 items-center justify-between border-b',
          isCompactLayout ? 'px-2 py-1.5' : 'px-3 py-2'
        )}
      >
        <div className="flex min-w-0 items-center gap-2">
          <Button variant="outline" size="icon" asChild className="h-8 w-8 shrink-0">
            <Link href={ticketListBackHref}>
              <ArrowLeft className="h-4 w-4" />
              <span className="sr-only">{t.ticket.supportCenter}</span>
            </Link>
          </Button>
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <h1 className="truncate text-sm font-medium">{ticket.subject}</h1>
              {getStatusBadge(ticket.status)}
            </div>
          </div>
        </div>

        {/* 操作菜单 */}
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              className="h-8 w-8"
              aria-label={t.common.more}
              title={t.common.more}
            >
              <MoreVertical className="h-4 w-4" />
              <span className="sr-only">{t.common.more}</span>
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <Dialog open={openShare} onOpenChange={setOpenShare}>
              <DialogTrigger asChild>
                <DropdownMenuItem onSelect={(e) => e.preventDefault()}>
                  <Share2 className="mr-2 h-4 w-4" />
                  {t.ticket.shareOrder}
                </DropdownMenuItem>
              </DialogTrigger>
            </Dialog>
            {!isClosed && ticket.status !== 'resolved' && (
              <DropdownMenuItem onClick={() => updateStatusMutation.mutate('resolved')}>
                <CheckCircle className="mr-2 h-4 w-4" />
                {t.ticket.markResolved}
              </DropdownMenuItem>
            )}
            {!isClosed && (
              <DropdownMenuItem onClick={() => updateStatusMutation.mutate('closed')}>
                <XCircle className="mr-2 h-4 w-4" />
                {t.ticket.closeTicket}
              </DropdownMenuItem>
            )}
            {isClosed && (
              <DropdownMenuItem onClick={() => updateStatusMutation.mutate('open')}>
                {t.ticket.reopen}
              </DropdownMenuItem>
            )}
          </DropdownMenuContent>
        </DropdownMenu>

        {/* 分享订单对话框 */}
        <Dialog open={openShare} onOpenChange={setOpenShare}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{t.ticket.shareOrderToAgent}</DialogTitle>
            </DialogHeader>
            <div className="space-y-4">
              <div>
                <label className="text-sm font-medium">{t.ticket.selectOrderToShare}</label>
                <Select
                  value={selectedOrder?.toString() || ''}
                  onValueChange={(v) => setSelectedOrder(Number(v))}
                >
                  <SelectTrigger className="mt-1.5">
                    <SelectValue placeholder={t.ticket.selectOrderPlaceholder} />
                  </SelectTrigger>
                  <SelectContent>
                    {orders.map((order: any) => (
                      <SelectItem
                        key={order.id}
                        value={order.id.toString()}
                        disabled={sharedOrderIds.has(order.id)}
                      >
                        {order.order_no} {sharedOrderIds.has(order.id) && t.ticket.alreadyShared}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <p className="text-sm text-muted-foreground">{t.ticket.shareOrderTip}</p>

              <div className="flex gap-2">
                <Button
                  onClick={() => shareOrderMutation.mutate()}
                  disabled={!selectedOrder || shareOrderMutation.isPending}
                  className="flex-1"
                >
                  {shareOrderMutation.isPending ? t.ticket.sharing : t.ticket.confirmShare}
                </Button>
                <Button variant="outline" onClick={() => setOpenShare(false)}>
                  {t.common.cancel}
                </Button>
              </div>
            </div>
          </DialogContent>
        </Dialog>
      </div>

      {/* 消息列表 */}
      <div className="flex shrink-0 flex-wrap items-center gap-x-4 gap-y-1 border-b bg-muted/15 px-3 py-2 text-xs text-muted-foreground">
        <span>
          {t.ticket.createdAt}:{' '}
          <span className="text-foreground">
            {format(new Date(ticket.created_at), 'yyyy-MM-dd HH:mm', {
              locale: locale === 'zh' ? zhCN : undefined,
            })}
          </span>
        </span>
        <span>
          {t.ticket.messageCountLabel}: <span className="text-foreground">{messages.length}</span>
        </span>
        {sharedOrders.length > 0 ? (
          <span>
            {t.ticket.sharedOrderCountLabel}:{' '}
            <span className="text-foreground">{sharedOrders.length}</span>
          </span>
        ) : null}
      </div>
      <PluginSlot
        slot="user.ticket_detail.meta.after"
        context={{ ...userTicketDetailPluginContext, section: 'meta' }}
      />

      <div className="scrollbar-hide flex-1 space-y-2 overflow-y-auto px-3 py-2">
        {messagesLoading ? (
          <div className="space-y-3 py-2">
            {[...Array(3)].map((_, index) => (
              <div
                key={index}
                className={cn('flex', index % 2 === 0 ? 'justify-start' : 'justify-end')}
              >
                <div className="max-w-[85%] space-y-2 rounded-lg border bg-muted/30 px-3 py-2">
                  <Skeleton className="h-3 w-24" />
                  <Skeleton className="h-4 w-56 max-w-[70vw]" />
                  <Skeleton className="h-4 w-40 max-w-[50vw]" />
                </div>
              </div>
            ))}
          </div>
        ) : messagesLoadFailed ? (
          <div className="flex h-full min-h-[220px] flex-col items-center justify-center text-center">
            <MessageSquare className="mb-4 h-10 w-10 text-muted-foreground" />
            <p className="text-base font-medium text-foreground">{t.ticket.messagesLoadFailed}</p>
            <p className="mt-2 max-w-sm text-sm text-muted-foreground">
              {t.ticket.messagesLoadFailedDesc}
            </p>
            <Button className="mt-4" variant="outline" onClick={() => refetchMessages()}>
              {t.common.refresh}
            </Button>
            <PluginSlot
              slot="user.ticket_detail.messages_load_failed"
              context={{ ...userTicketDetailPluginContext, section: 'messages_state' }}
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
              slot="user.ticket_detail.empty"
              context={{ ...userTicketDetailPluginContext, section: 'messages_state' }}
            />
          </div>
        ) : (
          messages.map((msg) => {
            const rowExtensions = userTicketMessageActionExtensions[String(msg.id)] || []

            return (
              <div
                key={msg.id}
                className={cn('flex', msg.sender_type === 'user' ? 'justify-end' : 'justify-start')}
              >
                <div
                  className={cn(
                    'flex max-w-[85%] flex-col gap-2',
                    msg.sender_type === 'user' ? 'items-end' : 'items-start'
                  )}
                >
                  <div
                    className={cn(
                      'rounded-lg px-3 py-1.5',
                      msg.sender_type === 'user'
                        ? 'bg-primary text-primary-foreground'
                        : 'bg-muted'
                    )}
                  >
                    <div className="mb-0.5 flex items-center gap-2">
                      <span className="text-xs font-medium">
                        {msg.sender_type === 'user' ? t.ticket.me : msg.sender_name || t.ticket.agent}
                      </span>
                      <span className="text-xs opacity-70">
                        {format(new Date(msg.created_at), 'HH:mm', {
                          locale: locale === 'zh' ? zhCN : undefined,
                        })}
                      </span>
                      {msg.sender_type === 'user' && (
                        <span className="text-xs opacity-70">
                          {msg.is_read_by_admin ? (
                            <CheckCheck className="inline h-3 w-3" />
                          ) : (
                            <Check className="inline h-3 w-3" />
                          )}
                        </span>
                      )}
                    </div>
                    {msg.content_type === 'order' ? (
                      <div className="flex items-center gap-2 text-sm">
                        <Package className="h-4 w-4" />
                        <span>{msg.content}</span>
                      </div>
                    ) : (
                      <MarkdownMessage
                        content={msg.content}
                        isOwnMessage={msg.sender_type === 'user'}
                      />
                    )}
                  </div>
                  {rowExtensions.length > 0 ? (
                    <div className="px-1">
                      <PluginExtensionList extensions={rowExtensions} display="inline" />
                    </div>
                  ) : null}
                </div>
              </div>
            )
          })
        )}
        <div ref={messagesEndRef} />
      </div>

      {/* 输入框 */}
      {!isClosed ? (
        <div
          className={cn(
            'shrink-0 border-t',
            isCompactLayout ? 'px-2 py-1.5' : 'px-3 py-2'
          )}
        >
          <PluginSlot
            slot="user.ticket_detail.composer.top"
            context={{ ...userTicketDetailPluginContext, section: 'composer' }}
          />
          <MessageToolbar
            value={message}
            onChange={setMessage}
            onSend={handleSend}
            onUploadFile={handleUploadFile}
            compactLayout={isCompactLayout}
            isSending={sendMessageMutation.isPending}
            enableImage={ticketAttachment?.enable_image ?? true}
            enableVoice={ticketAttachment?.enable_voice ?? true}
            acceptImageTypes={ticketAttachment?.allowed_image_types}
            maxLength={maxContentLength > 0 ? maxContentLength : undefined}
            pluginSlotNamespace="user.ticket_detail.composer"
            pluginSlotContext={{ ...userTicketDetailPluginContext, section: 'composer' }}
            pluginSlotPath={ticketDetailPath}
            translations={{
              messagePlaceholder: t.ticket.messagePlaceholder,
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
        <div
          className={cn(
            'shrink-0 border-t text-center text-sm text-muted-foreground',
            isCompactLayout ? 'px-2 py-1.5' : 'px-3 py-2'
          )}
        >
          <p className="font-medium">{t.ticket.ticketClosed}</p>
          <p className="mt-1 text-xs text-muted-foreground">{t.ticket.ticketClosedHint}</p>
          <PluginSlot
            slot="user.ticket_detail.composer.top"
            context={{ ...userTicketDetailPluginContext, section: 'composer' }}
          />
        </div>
      )}
    </div>
  )
}
