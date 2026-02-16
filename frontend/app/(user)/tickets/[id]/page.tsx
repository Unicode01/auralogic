'use client'

import { useState, useEffect, useRef } from 'react'
import { useParams, useRouter } from 'next/navigation'
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
import { MarkdownMessage } from '@/components/ui/markdown-message'
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
import { ArrowLeft, Package, CheckCircle, XCircle, Share2, Check, CheckCheck, MoreVertical } from 'lucide-react'
import { useToast } from '@/hooks/use-toast'
import { TICKET_STATUS_CONFIG } from '@/lib/constants'
import Link from 'next/link'
import { format } from 'date-fns'
import { zhCN } from 'date-fns/locale'
import { cn } from '@/lib/utils'
import { useLocale } from '@/hooks/use-locale'
import { usePageTitle } from '@/hooks/use-page-title'
import { getTranslations } from '@/lib/i18n'

export default function TicketDetailPage() {
  const params = useParams()
  const router = useRouter()
  const ticketId = Number(params.id)
  const [message, setMessage] = useState('')
  const [openShare, setOpenShare] = useState(false)
  const [selectedOrder, setSelectedOrder] = useState<number | null>(null)
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const queryClient = useQueryClient()
  const toast = useToast()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.ticketDetail)

  const { data: ticketData, isLoading: ticketLoading } = useQuery({
    queryKey: ['ticket', ticketId],
    queryFn: () => getTicket(ticketId),
    enabled: !!ticketId,
  })

  const { data: messagesData, isLoading: messagesLoading } = useQuery({
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
      toast.error(error.message || t.ticket.sendFailed)
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
      toast.error(error.message || t.ticket.statusUpdateFailed)
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
      toast.error(error.message || t.ticket.shareFailed)
    },
  })

  const ticket = ticketData?.data
  const messages: TicketMessage[] = messagesData?.data || []
  const orders = ordersData?.data?.items || []
  const sharedOrders = sharedOrdersData?.data || []
  const sharedOrderIds = new Set(sharedOrders.map((s: any) => s.order_id))
  const ticketAttachment = publicConfigData?.data?.ticket?.attachment
  const maxContentLength = publicConfigData?.data?.ticket?.max_content_length || 0

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
    const res = await uploadTicketFile(ticketId, file)
    return res.data?.url || res.data
  }

  const getStatusBadge = (status: string) => {
    const config = TICKET_STATUS_CONFIG[status as keyof typeof TICKET_STATUS_CONFIG]
    if (!config) return <Badge variant="secondary">{status}</Badge>
    const label = t.ticket.ticketStatus[status as keyof typeof t.ticket.ticketStatus] || config.label
    return <Badge variant={config.color as any}>{label}</Badge>
  }

  if (ticketLoading) {
    return <div className="text-center py-12 text-muted-foreground">{t.ticket.loading}</div>
  }

  if (!ticket) {
    return <div className="text-center py-12 text-muted-foreground">{t.ticket.ticketNotFound}</div>
  }

  const isClosed = ticket.status === 'closed'

  return (
    <div className="h-[calc(100vh-6rem)] md:h-[calc(100vh-4rem)] flex flex-col">
      {/* 头部 - 更紧凑 */}
      <div className="flex items-center justify-between px-2 py-1.5 md:px-3 md:py-2 border-b shrink-0">
        <div className="flex items-center gap-2 min-w-0">
          <Button variant="ghost" size="icon" asChild className="shrink-0 h-8 w-8">
            <Link href="/tickets">
              <ArrowLeft className="h-4 w-4" />
            </Link>
          </Button>
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <h1 className="font-medium text-sm truncate">{ticket.subject}</h1>
              {getStatusBadge(ticket.status)}
            </div>
            <p className="text-xs text-muted-foreground">#{ticket.ticket_no}</p>
          </div>
        </div>

        {/* 操作菜单 */}
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" className="h-8 w-8">
              <MoreVertical className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <Dialog open={openShare} onOpenChange={setOpenShare}>
              <DialogTrigger asChild>
                <DropdownMenuItem onSelect={(e) => e.preventDefault()}>
                  <Share2 className="h-4 w-4 mr-2" />
                  {t.ticket.shareOrder}
                </DropdownMenuItem>
              </DialogTrigger>
            </Dialog>
            {!isClosed && ticket.status !== 'resolved' && (
              <DropdownMenuItem onClick={() => updateStatusMutation.mutate('resolved')}>
                <CheckCircle className="h-4 w-4 mr-2" />
                {t.ticket.markResolved}
              </DropdownMenuItem>
            )}
            {!isClosed && (
              <DropdownMenuItem onClick={() => updateStatusMutation.mutate('closed')}>
                <XCircle className="h-4 w-4 mr-2" />
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

              <p className="text-sm text-muted-foreground">
                {t.ticket.shareOrderTip}
              </p>

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
      <div className="flex-1 overflow-y-auto px-3 py-2 space-y-2 scrollbar-hide">
        {messagesLoading ? (
          <div className="text-center py-12 text-muted-foreground">{t.ticket.loadingMessages}</div>
        ) : (
          messages.map((msg) => (
            <div
              key={msg.id}
              className={cn(
                'flex',
                msg.sender_type === 'user' ? 'justify-end' : 'justify-start'
              )}
            >
              <div
                className={cn(
                  'max-w-[85%] rounded-lg px-3 py-1.5',
                  msg.sender_type === 'user'
                    ? 'bg-primary text-primary-foreground'
                    : 'bg-muted'
                )}
              >
                <div className="flex items-center gap-2 mb-0.5">
                  <span className="text-xs font-medium">
                    {msg.sender_type === 'user' ? t.ticket.me : msg.sender_name || t.ticket.agent}
                  </span>
                  <span className="text-xs opacity-70">
                    {format(new Date(msg.created_at), 'HH:mm', { locale: locale === 'zh' ? zhCN : undefined })}
                  </span>
                  {msg.sender_type === 'user' && (
                    <span className="text-xs opacity-70">
                      {msg.is_read_by_admin ? (
                        <CheckCheck className="h-3 w-3 inline" />
                      ) : (
                        <Check className="h-3 w-3 inline" />
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
            </div>
          ))
        )}
        <div ref={messagesEndRef} />
      </div>

      {/* 输入框 */}
      {!isClosed ? (
        <div className="px-2 py-1.5 md:px-3 md:py-2 border-t shrink-0">
          <MessageToolbar
            value={message}
            onChange={setMessage}
            onSend={handleSend}
            onUploadFile={handleUploadFile}
            isSending={sendMessageMutation.isPending}
            enableImage={ticketAttachment?.enable_image ?? true}
            enableVoice={ticketAttachment?.enable_voice ?? true}
            acceptImageTypes={ticketAttachment?.allowed_image_types}
            maxLength={maxContentLength > 0 ? maxContentLength : undefined}
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
        <div className="px-2 py-1.5 md:px-3 md:py-2 border-t text-center text-muted-foreground text-sm shrink-0">
          {t.ticket.ticketClosed}
        </div>
      )}
    </div>
  )
}
