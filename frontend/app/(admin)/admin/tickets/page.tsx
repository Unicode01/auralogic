'use client'

import { useState, useEffect, useRef } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
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
import { Search, Package, User, Clock, Check, CheckCheck, MapPin, Truck } from 'lucide-react'
import { useToast } from '@/hooks/use-toast'
import { TICKET_STATUS_CONFIG, TICKET_PRIORITY_CONFIG } from '@/lib/constants'
import { format, formatDistanceToNow } from 'date-fns'
import { zhCN } from 'date-fns/locale'
import { cn } from '@/lib/utils'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import { formatCurrency, formatDate } from '@/lib/utils'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'

export default function AdminTicketsPage() {
  const [selectedTicketId, setSelectedTicketId] = useState<number | null>(null)
  const [status, setStatus] = useState('')
  const [search, setSearch] = useState('')
  const [assignedTo, setAssignedTo] = useState('')
  const [message, setMessage] = useState('')
  const [viewingOrderId, setViewingOrderId] = useState<number | null>(null)
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const queryClient = useQueryClient()
  const toast = useToast()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.adminTickets)

  // 获取工单列表
  const { data: ticketsData, isLoading: ticketsLoading } = useQuery({
    queryKey: ['adminTickets', status, search, assignedTo],
    queryFn: () => getAdminTickets({ status: status || undefined, search: search || undefined, assigned_to: assignedTo || undefined }),
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
  const { data: messagesData, isLoading: messagesLoading } = useQuery({
    queryKey: ['adminTicketMessages', selectedTicketId],
    queryFn: () => getAdminTicketMessages(selectedTicketId!),
    enabled: !!selectedTicketId,
    refetchInterval: 2000, // 2秒轮询
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
    onError: (error: any) => {
      toast.error(error.message || t.ticket.sendFailed)
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
    onError: (error: any) => {
      toast.error(error.message || t.ticket.updateFailed)
    },
  })

  const tickets: Ticket[] = ticketsData?.data?.items || []
  const stats = statsData?.data
  const selectedTicket = ticketData?.data
  const messages: TicketMessage[] = messagesData?.data || []
  const sharedOrders = sharedOrdersData?.data || []
  const ticketAttachment = publicConfigData?.data?.ticket?.attachment
  const maxContentLength = publicConfigData?.data?.ticket?.max_content_length || 0

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

  const handleSend = () => {
    if (!message.trim()) return
    if (maxContentLength > 0 && message.trim().length > maxContentLength) {
      toast.error(t.ticket.contentTooLong.replace('{max}', String(maxContentLength)))
      return
    }
    sendMessageMutation.mutate(message.trim())
  }

  const handleUploadFile = async (file: File): Promise<string> => {
    const res = await uploadAdminTicketFile(selectedTicketId!, file)
    return res.data?.url || res.data
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
    const label = t.ticket.ticketStatus[status as keyof typeof t.ticket.ticketStatus] || config.label
    return <Badge className={colorClass}>{label}</Badge>
  }

  const getPriorityBadge = (priority: string) => {
    const config = TICKET_PRIORITY_CONFIG[priority as keyof typeof TICKET_PRIORITY_CONFIG]
    if (!config) return null
    const colorClass = priorityColorMap[priority] || ''
    const label = t.ticket.ticketPriority[priority as keyof typeof t.ticket.ticketPriority] || config.label
    return <Badge className={cn("text-xs", colorClass)}>{label}</Badge>
  }

  // 从消息 metadata 获取订单ID
  const getOrderIdFromMessage = (msg: TicketMessage): number | null => {
    if (msg.content_type !== 'order' || !msg.metadata) return null
    try {
      const meta = typeof msg.metadata === 'string' ? JSON.parse(msg.metadata) : msg.metadata
      return meta.order_id || null
    } catch {
      return null
    }
  }

  return (
    <div className="h-[calc(100vh-4rem)] flex flex-col">
      {/* 顶部统计栏 - 更紧凑 */}
      <div className="px-4 py-2 border-b flex items-center justify-between">
        <h1 className="text-xl font-bold">{t.ticket.ticketManagement}</h1>
        {stats && (
          <div className="flex gap-3 text-sm">
            <span>{t.ticket.total}: <strong>{stats.total}</strong></span>
            <span className="text-yellow-600">{t.ticket.pending}: <strong>{stats.open}</strong></span>
            <span className="text-blue-600">{t.ticket.processing}: <strong>{stats.processing}</strong></span>
            <span className="text-red-600">{t.ticket.unread}: <strong>{stats.unread}</strong></span>
          </div>
        )}
      </div>

      <div className="flex-1 flex overflow-hidden">
        {/* 左侧工单列表 - 缩窄 */}
        <div className="w-72 shrink-0 border-r flex flex-col overflow-hidden">
          {/* 筛选 */}
          <div className="p-3 border-b space-y-2">
            <div className="relative">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
              <Input
                placeholder={t.ticket.searchPlaceholder}
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                className="pl-10"
              />
            </div>
            <div className="flex gap-2">
              <Select value={status || 'all'} onValueChange={(v) => setStatus(v === 'all' ? '' : v)}>
                <SelectTrigger className="flex-1">
                  <SelectValue placeholder={t.ticket.status} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">{t.ticket.allStatus}</SelectItem>
                  {Object.entries(TICKET_STATUS_CONFIG).map(([key, config]) => (
                    <SelectItem key={key} value={key}>{t.ticket.ticketStatus[key as keyof typeof t.ticket.ticketStatus] || config.label}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Select value={assignedTo || 'all'} onValueChange={(v) => setAssignedTo(v === 'all' ? '' : v)}>
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
          </div>

          {/* 工单列表 */}
          <div className="flex-1 overflow-y-auto scrollbar-hide">
            {ticketsLoading ? (
              <div className="text-center py-8 text-muted-foreground">{t.ticket.loading}</div>
            ) : tickets.length === 0 ? (
              <div className="text-center py-8 text-muted-foreground">{t.ticket.noTickets}</div>
            ) : (
              <div className="divide-y">
                {tickets.map((ticket) => (
                  <div
                    key={ticket.id}
                    className={cn(
                      'p-3 cursor-pointer hover:bg-accent/50 transition-colors',
                      selectedTicketId === ticket.id && 'bg-accent'
                    )}
                    onClick={() => {
                      setSelectedTicketId(ticket.id)
                      queryClient.invalidateQueries({ queryKey: ['adminTicketMessages', ticket.id] })
                    }}
                  >
                    <div className="flex items-start justify-between gap-2">
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2 min-w-0">
                          <span className="font-medium text-sm truncate">{ticket.subject}</span>
                          {ticket.unread_count_admin > 0 && (
                            <Badge variant="destructive" className="text-xs h-5">
                              {ticket.unread_count_admin}
                            </Badge>
                          )}
                        </div>
                        <p className="text-xs text-muted-foreground mt-1 truncate">
                          {ticket.last_message_preview}
                        </p>
                        <div className="flex items-center gap-2 mt-1">
                          <span className="text-xs text-muted-foreground flex items-center gap-1">
                            <User className="h-3 w-3" />
                            {ticket.user?.name || ticket.user?.email || 'Unknown'}
                          </span>
                          <span className="text-xs text-muted-foreground flex items-center gap-1">
                            <Clock className="h-3 w-3" />
                            {ticket.last_message_at
                              ? formatDistanceToNow(new Date(ticket.last_message_at), { locale: locale === 'zh' ? zhCN : undefined })
                              : formatDistanceToNow(new Date(ticket.created_at), { locale: locale === 'zh' ? zhCN : undefined })}
                          </span>
                        </div>
                      </div>
                      <div className="ml-2 shrink-0">
                        {getStatusBadge(ticket.status)}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>

        {/* 右侧聊天区域 */}
        <div className="flex-1 min-w-0 flex flex-col">
          {!selectedTicketId ? (
            <div className="flex-1 flex items-center justify-center text-muted-foreground">
              {t.ticket.selectTicket}
            </div>
          ) : (
            <>
              {/* 工单头部 - 更紧凑 */}
              {selectedTicket && (
                <div className="px-3 py-2 border-b">
                  <div className="flex items-center justify-between gap-2">
                    <div className="flex items-center gap-2 min-w-0 flex-1">
                      <h2 className="font-semibold truncate">{selectedTicket.subject}</h2>
                      {getStatusBadge(selectedTicket.status)}
                      {getPriorityBadge(selectedTicket.priority)}
                    </div>
                    <Select
                      value={selectedTicket.status}
                      onValueChange={(v) => updateTicketMutation.mutate({ status: v })}
                    >
                      <SelectTrigger className="w-28 h-8">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {Object.entries(TICKET_STATUS_CONFIG).map(([key, config]) => (
                          <SelectItem key={key} value={key}>{t.ticket.ticketStatus[key as keyof typeof t.ticket.ticketStatus] || config.label}</SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <p className="text-xs text-muted-foreground mt-1">
                    #{selectedTicket.ticket_no} | {selectedTicket.user?.name || selectedTicket.user?.email} | {format(new Date(selectedTicket.created_at), 'MM-dd HH:mm')}
                  </p>
                  {/* 分享的订单 */}
                  {sharedOrders.length > 0 && (
                    <div className="mt-1.5 flex items-center gap-1.5 flex-wrap">
                      <span className="text-xs text-muted-foreground">{t.ticket.orders}:</span>
                      {sharedOrders.map((access: any) => (
                        <button
                          key={access.id}
                          onClick={() => setViewingOrderId(access.order?.id)}
                          className="inline-flex"
                        >
                          <Badge
                            variant="outline"
                            className="text-xs cursor-pointer hover:bg-accent transition-colors h-5"
                          >
                            <Package className="h-3 w-3 mr-1" />
                            {access.order?.order_no}
                          </Badge>
                        </button>
                      ))}
                    </div>
                  )}
                </div>
              )}

              {/* 消息列表 */}
              <div className="flex-1 overflow-y-auto px-3 py-2 space-y-2 scrollbar-hide">
                {messagesLoading ? (
                  <div className="text-center py-8 text-muted-foreground">{t.ticket.loadingMessages}</div>
                ) : (
                  <>
                    {messages.map((msg) => {
                      const orderId = getOrderIdFromMessage(msg)
                      return (
                        <div
                          key={msg.id}
                          className={cn(
                            'flex',
                            msg.sender_type === 'admin' ? 'justify-end' : 'justify-start'
                          )}
                        >
                          <div
                            className={cn(
                              'max-w-[80%] rounded-lg px-3 py-1.5',
                              msg.sender_type === 'admin'
                                ? 'bg-primary text-primary-foreground'
                                : 'bg-muted'
                            )}
                          >
                            <div className="flex items-center gap-2 mb-0.5">
                              <span className="text-xs font-medium">
                                {msg.sender_type === 'admin' ? msg.sender_name || t.ticket.adminAgent : t.ticket.user}
                              </span>
                              <span className="text-xs opacity-70">
                                {format(new Date(msg.created_at), 'MM-dd HH:mm', { locale: locale === 'zh' ? zhCN : undefined })}
                              </span>
                              {/* 已读/未读状态 */}
                              {msg.sender_type === 'admin' && (
                                <span className="text-xs opacity-70">
                                  {msg.is_read_by_user ? (
                                    <CheckCheck className="h-3 w-3 inline" />
                                  ) : (
                                    <Check className="h-3 w-3 inline" />
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
                        </div>
                      )
                    })}
                    <div ref={messagesEndRef} />
                  </>
                )}
              </div>

              {/* 输入框 */}
              {selectedTicket?.status !== 'closed' ? (
                <div className="p-2 md:p-3 border-t bg-background shrink-0">
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
                <div className="p-2 md:p-3 border-t text-center text-muted-foreground text-sm shrink-0">
                  {t.ticket.ticketClosed}
                </div>
              )}
            </>
          )}
        </div>
      </div>

      {/* 订单详情对话框 */}
      <Dialog open={!!viewingOrderId} onOpenChange={(open) => !open && setViewingOrderId(null)}>
        <DialogContent className="max-w-2xl max-h-[85vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <Package className="h-5 w-5" />
              {t.ticket.orderDetail}
            </DialogTitle>
          </DialogHeader>

          {orderDetailLoading ? (
            <div className="text-center py-8 text-muted-foreground">{t.ticket.loading}</div>
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
                            {formatCurrency(order.total_amount, order.currency)}
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
                      <CardTitle className="text-base flex items-center gap-2">
                        <Package className="h-4 w-4" />
                        {t.ticket.productInfo}
                      </CardTitle>
                    </CardHeader>
                    <CardContent className="py-2">
                      <div className="space-y-3">
                        {order.items?.map((item: any, index: number) => (
                          <div key={index} className="flex gap-3">
                            {item.image_url ? (
                              <img
                                src={item.image_url}
                                alt={item.name}
                                className="w-14 h-14 rounded object-cover bg-muted"
                                onError={(e) => {
                                  e.currentTarget.style.display = 'none'
                                  e.currentTarget.parentElement?.querySelector('.img-fallback')?.classList.remove('hidden')
                                }}
                              />
                            ) : null}
                            <div className={`img-fallback w-14 h-14 rounded bg-muted flex items-center justify-center ${item.image_url ? 'hidden' : ''}`}>
                              <Package className="h-6 w-6 text-muted-foreground" />
                            </div>
                            <div className="flex-1 min-w-0">
                              <p className="font-medium text-sm truncate">{item.name}</p>
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
                        <CardTitle className="text-base flex items-center gap-2">
                          <MapPin className="h-4 w-4" />
                          {t.ticket.shippingInfo}
                        </CardTitle>
                      </CardHeader>
                      <CardContent className="py-2">
                        <dl className="space-y-2 text-sm">
                          {order.receiver_name && (
                            <div className="flex">
                              <dt className="w-20 text-muted-foreground shrink-0">{t.ticket.recipient}</dt>
                              <dd>{order.receiver_name}</dd>
                            </div>
                          )}
                          {order.receiver_phone && (
                            <div className="flex">
                              <dt className="w-20 text-muted-foreground shrink-0">{t.ticket.phone}</dt>
                              <dd>{order.phone_code || ''}{order.receiver_phone}</dd>
                            </div>
                          )}
                          {order.receiver_email && (
                            <div className="flex">
                              <dt className="w-20 text-muted-foreground shrink-0">{t.ticket.email}</dt>
                              <dd className="break-all">{order.receiver_email}</dd>
                            </div>
                          )}
                          {(order.receiver_province || order.receiver_city || order.receiver_address) && (
                            <div className="flex">
                              <dt className="w-20 text-muted-foreground shrink-0">{t.ticket.address}</dt>
                              <dd className="break-all">
                                {order.receiver_province} {order.receiver_city} {order.receiver_district} {order.receiver_address}
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
                        <CardTitle className="text-base flex items-center gap-2">
                          <Truck className="h-4 w-4" />
                          {t.ticket.logisticsInfo}
                        </CardTitle>
                      </CardHeader>
                      <CardContent className="py-2">
                        <dl className="space-y-2 text-sm">
                          <div className="flex">
                            <dt className="w-20 text-muted-foreground shrink-0">{t.ticket.trackingNo}</dt>
                            <dd className="font-mono">{order.tracking_no}</dd>
                          </div>
                          {order.shipped_at && (
                            <div className="flex">
                              <dt className="w-20 text-muted-foreground shrink-0">{t.ticket.shippedAt}</dt>
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
                        <p className="text-sm whitespace-pre-wrap">{order.remark}</p>
                      </CardContent>
                    </Card>
                  )}
                </div>
              )
            })()
          ) : (
            <div className="text-center py-8 text-muted-foreground">{t.ticket.orderNotAccessible}</div>
          )}
        </DialogContent>
      </Dialog>
    </div>
  )
}
