'use client'

import { useState, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { getTickets, createTicket, getOrders, getPublicConfig, Ticket } from '@/lib/api'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Badge } from '@/components/ui/badge'
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
import { Plus, MessageSquare, Clock, Package, ChevronLeft, ChevronRight, Search, XCircle } from 'lucide-react'
import { useToast } from '@/hooks/use-toast'
import { TICKET_STATUS_CONFIG, TICKET_PRIORITY_CONFIG } from '@/lib/constants'
import Link from 'next/link'
import { formatDistanceToNow } from 'date-fns'
import { zhCN } from 'date-fns/locale'
import { useLocale } from '@/hooks/use-locale'
import { usePageTitle } from '@/hooks/use-page-title'
import { getTranslations } from '@/lib/i18n'

export default function TicketsPage() {
  const [openCreate, setOpenCreate] = useState(false)
  const [status, setStatus] = useState('')
  const [searchText, setSearchText] = useState('')
  const [debouncedSearch, setDebouncedSearch] = useState('')
  const [page, setPage] = useState(1)
  const [selectedOrderId, setSelectedOrderId] = useState<number | null>(null)
  const queryClient = useQueryClient()
  const toast = useToast()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.tickets)
  const limit = 10

  const { data: publicConfigData, isLoading: configLoading } = useQuery({
    queryKey: ['publicConfig'],
    queryFn: getPublicConfig,
    staleTime: 5 * 60 * 1000,
  })
  const ticketEnabled = publicConfigData?.data?.ticket?.enabled ?? true
  const maxContentLength = publicConfigData?.data?.ticket?.max_content_length || 0

  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedSearch(searchText)
      setPage(1)
    }, 300)
    return () => clearTimeout(timer)
  }, [searchText])

  const { data, isLoading } = useQuery({
    queryKey: ['userTickets', status, debouncedSearch, page],
    queryFn: () => getTickets({
      status: status || undefined,
      search: debouncedSearch || undefined,
      page,
      limit
    }),
  })

  const { data: ordersData } = useQuery({
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
    },
    onError: (error: any) => {
      toast.error(error.message || t.ticket.createFailed)
    },
  })

  const tickets: Ticket[] = data?.data?.items || []
  const total = data?.data?.total || 0
  const totalPages = Math.ceil(total / limit)
  const orders = ordersData?.data?.items || []

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

  const getStatusBadge = (status: string) => {
    const config = TICKET_STATUS_CONFIG[status as keyof typeof TICKET_STATUS_CONFIG]
    if (!config) return <Badge variant="secondary">{status}</Badge>
    const label = t.ticket.ticketStatus[status as keyof typeof t.ticket.ticketStatus] || config.label
    return <Badge variant={config.color as any}>{label}</Badge>
  }

  const handleStatusChange = (newStatus: string) => {
    setStatus(newStatus === 'all' ? '' : newStatus)
    setPage(1)
  }

  if (!configLoading && !ticketEnabled) {
    return (
      <div className="flex flex-col items-center justify-center py-20 text-center">
        <XCircle className="h-12 w-12 text-muted-foreground mb-4" />
        <h2 className="text-lg font-medium mb-2">{t.ticket.disabledTitle}</h2>
        <p className="text-sm text-muted-foreground">{t.ticket.disabledDesc}</p>
      </div>
    )
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl md:text-2xl font-bold">{t.ticket.supportCenter}</h1>
          <p className="text-muted-foreground text-sm">{t.ticket.supportCenterDesc}</p>
        </div>
        <Dialog open={openCreate} onOpenChange={(open) => {
          setOpenCreate(open)
          if (!open) setSelectedOrderId(null)
        }}>
          <DialogTrigger asChild>
            <Button size="sm">
              <Plus className="h-4 w-4" />
            </Button>
          </DialogTrigger>
          <DialogContent className="max-w-lg max-h-[90vh] overflow-y-auto">
            <DialogHeader>
              <DialogTitle>{t.ticket.createTicket}</DialogTitle>
            </DialogHeader>
            <form onSubmit={handleSubmit} className="space-y-4">
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
                        <SelectItem key={key} value={key}>{t.ticket.ticketPriority[key as keyof typeof t.ticket.ticketPriority] || config.label}</SelectItem>
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
                  required
                  maxLength={maxContentLength > 0 ? maxContentLength : undefined}
                />
              </div>

              <div>
                <label className="text-sm font-medium flex items-center gap-2">
                  <Package className="h-4 w-4" />
                  {t.ticket.relatedOrder}
                </label>
                <Select
                  value={selectedOrderId?.toString() || 'none'}
                  onValueChange={(v) => setSelectedOrderId(v === 'none' ? null : Number(v))}
                >
                  <SelectTrigger className="mt-1.5">
                    <SelectValue placeholder={t.ticket.selectOrder} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="none">{t.ticket.noRelatedOrder}</SelectItem>
                    {orders.map((order: any) => (
                      <SelectItem key={order.id} value={order.id.toString()}>
                        {order.order_no} - {order.product?.name || t.ticket.items}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <p className="text-xs text-muted-foreground mt-1">
                  {t.ticket.relatedOrderTip}
                </p>
              </div>

              <div className="flex gap-2">
                <Button type="submit" disabled={createMutation.isPending} className="flex-1">
                  {createMutation.isPending ? t.ticket.submitting : t.ticket.submitTicket}
                </Button>
                <Button type="button" variant="outline" onClick={() => setOpenCreate(false)}>
                  {t.common.cancel}
                </Button>
              </div>
            </form>
          </DialogContent>
        </Dialog>
      </div>

      <div className="flex gap-2">
        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder={t.ticket.searchPlaceholder}
            value={searchText}
            onChange={(e) => setSearchText(e.target.value)}
            className="pl-9"
          />
        </div>
        <Select value={status || 'all'} onValueChange={handleStatusChange}>
          <SelectTrigger className="w-28">
            <SelectValue placeholder={t.ticket.status} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">{t.ticket.allStatus}</SelectItem>
            {Object.entries(TICKET_STATUS_CONFIG).map(([key, config]) => (
              <SelectItem key={key} value={key}>{t.ticket.ticketStatus[key as keyof typeof t.ticket.ticketStatus] || config.label}</SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {isLoading ? (
        <div className="text-center py-12 text-muted-foreground">{t.ticket.loading}</div>
      ) : tickets.length === 0 ? (
        <Card>
          <CardContent className="text-center py-12">
            <MessageSquare className="h-12 w-12 mx-auto text-muted-foreground mb-4" />
            <p className="text-muted-foreground">{t.ticket.noTickets}</p>
            <Button className="mt-4" onClick={() => setOpenCreate(true)}>
              {t.ticket.createFirst}
            </Button>
          </CardContent>
        </Card>
      ) : (
        <>
          <div className="space-y-4">
            {tickets.map((ticket) => (
              <Link key={ticket.id} href={`/tickets/${ticket.id}`} className="block">
                <Card className="hover:bg-accent/50 transition-colors cursor-pointer border">
                  <CardContent className="p-4">
                    <div className="flex items-center justify-between gap-2">
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2">
                          <h3 className="font-medium text-sm truncate flex-1">{ticket.subject}</h3>
                          {getStatusBadge(ticket.status)}
                        </div>
                        <p className="text-xs text-muted-foreground mt-1 truncate">
                          {ticket.last_message_preview || ticket.content}
                        </p>
                        <div className="flex items-center gap-3 mt-1.5 text-xs text-muted-foreground">
                          <span className="flex items-center gap-1">
                            <Clock className="h-3 w-3" />
                            {ticket.last_message_at
                              ? formatDistanceToNow(new Date(ticket.last_message_at), { addSuffix: true, locale: locale === 'zh' ? zhCN : undefined })
                              : formatDistanceToNow(new Date(ticket.created_at), { addSuffix: true, locale: locale === 'zh' ? zhCN : undefined })}
                          </span>
                          <span className="truncate">#{ticket.ticket_no}</span>
                        </div>
                      </div>
                      {ticket.unread_count_user > 0 && (
                        <Badge variant="destructive" className="text-xs shrink-0">
                          {ticket.unread_count_user}
                        </Badge>
                      )}
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
