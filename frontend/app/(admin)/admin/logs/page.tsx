'use client'

import { useCallback, useMemo, useState } from 'react'
import Link from 'next/link'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  getOperationLogs,
  getEmailLogs,
  getSmsLogs,
  getLogStatistics,
  retryFailedEmails,
  getInventoryLogs,
} from '@/lib/api'
import { resolveClientAPIProxyURL } from '@/lib/api-base-url'
import { DataTable } from '@/components/admin/data-table'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useToast } from '@/hooks/use-toast'
import { Download, FileText, Mail, RefreshCw, Package, Smartphone } from 'lucide-react'
import { formatDate } from '@/lib/utils'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'
import { resolveApiErrorMessage } from '@/lib/api-error'
import { PluginSlot } from '@/components/plugins/plugin-slot'

const operationResourceOrder = [
  'auth',
  'order',
  'user',
  'admin',
  'api_key',
  'product',
  'inventory',
  'payment_method',
  'payment',
  'marketing_batch',
  'plugin',
  'system',
  'system_config',
] as const

const operationActionsByResource: Record<string, string[]> = {
  auth: ['login'],
  order: [
    'assign_tracking',
    'complete',
    'cancel',
    'refund',
    'mark_paid',
    'deliver_virtual_stock',
    'delete',
    'request_resubmit',
    'batch_complete_orders',
    'batch_cancel_orders',
    'batch_delete_orders',
    'create_draft',
    'create_draft_failed',
    'admin_create_order',
    'update_price',
    'form_submit',
    'form_submit_failed',
    'payment_method_selected',
  ],
  user: ['create', 'update', 'delete', 'register', 'verify_email', 'auto_create'],
  admin: ['create', 'update', 'delete', 'login'],
  api_key: ['create', 'update', 'delete'],
  product: ['create', 'update', 'delete'],
  inventory: ['create', 'update', 'delete', 'adjust_stock'],
  payment_method: [
    'create',
    'update',
    'delete',
    'import',
    'payment_method_market_preview',
    'payment_method_init',
    'payment_method_legacy_migration',
  ],
  payment: [
    'payment_method_selected',
    'payment_success',
    'payment_update_failed',
    'payment_polling_add',
    'payment_polling_timeout',
    'payment_polling_max_retries',
    'payment_polling_check_failed',
    'payment_polling_backfill',
    'check_auto_delivery_failed',
    'virtual_delivery_failed',
    'order_auto_cancelled',
  ],
  marketing_batch: ['queue_marketing'],
  plugin: [],
  system: [
    'maintenance',
    'order_cancel_service_start',
    'order_cancel_service_stop',
    'order_auto_cancel',
    'payment_polling_start',
    'payment_polling_stop',
    'payment_polling_backfill',
    'payment_polling_recover',
    'ticket_attachment_cleanup_start',
    'ticket_attachment_cleanup_stop',
    'ticket_attachment_cleanup',
    'ticket_auto_close_start',
    'ticket_auto_close_stop',
    'ticket_auto_close',
  ],
  system_config: ['update'],
}

function uniqueStrings(values: string[]): string[] {
  const seen = new Set<string>()
  const out: string[] = []
  values.forEach((value) => {
    const normalized = String(value || '').trim()
    if (!normalized || seen.has(normalized)) {
      return
    }
    seen.add(normalized)
    out.push(normalized)
  })
  return out
}

function humanizeLogToken(value: string): string {
  return String(value || '')
    .trim()
    .replace(/_/g, ' ')
    .replace(/\b\w/g, (char) => char.toUpperCase())
}

function getOperationDetailValue(details: unknown, key: string): string {
  if (!details || typeof details !== 'object' || Array.isArray(details)) {
    return ''
  }
  const value = (details as Record<string, unknown>)[key]
  if (value === undefined || value === null) {
    return ''
  }
  return String(value).trim()
}

export default function LogsPage() {
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.adminLogs)

  const [activeTab, setActiveTab] = useState('operations')
  const [operationPage, setOperationPage] = useState(1)
  const [emailPage, setEmailPage] = useState(1)
  const [inventoryPage, setInventoryPage] = useState(1)
  const [operationFilters, setOperationFilters] = useState({
    action: '',
    resource_type: '',
    resource_id: '',
    order_no: '',
    user_id: '',
    start_date: '',
    end_date: '',
  })
  const [emailFilters, setEmailFilters] = useState({
    status: '',
    event_type: '',
    to_email: '',
    start_date: '',
    end_date: '',
  })
  const [smsPage, setSmsPage] = useState(1)
  const [smsFilters, setSmsFilters] = useState({
    status: '',
    event_type: '',
    phone: '',
    start_date: '',
    end_date: '',
  })
  const [inventoryFilters, setInventoryFilters] = useState({
    source: '',
    type: '',
    inventory_id: '',
    order_no: '',
    start_date: '',
    end_date: '',
  })
  const [selectedEmails, setSelectedEmails] = useState<number[]>([])

  const queryClient = useQueryClient()
  const toast = useToast()
  const resolveLogError = (error: unknown, fallback: string) =>
    resolveApiErrorMessage(error, t, fallback)
  const readFetchErrorMessage = useCallback(
    async (response: Response, fallback: string) => {
      try {
        const contentType = response.headers.get('content-type') || ''
        if (contentType.includes('application/json')) {
          const payload = await response.json()
          return resolveApiErrorMessage(payload, t, fallback)
        }

        const text = (await response.text()).trim()
        if (text) {
          try {
            return resolveApiErrorMessage(JSON.parse(text), t, fallback)
          } catch {
            return text
          }
        }
      } catch {
        // ignore parse errors and fall back to the provided message
      }
      return fallback
    },
    [t]
  )

  // 操作日志查询
  const { data: operationLogs, isLoading: operationLoading } = useQuery({
    queryKey: ['operationLogs', operationPage, operationFilters],
    queryFn: () => getOperationLogs({ ...operationFilters, page: operationPage, limit: 20 }),
  })

  // 邮件日志查询
  const { data: emailLogs, isLoading: emailLoading } = useQuery({
    queryKey: ['emailLogs', emailPage, emailFilters],
    queryFn: () => getEmailLogs({ ...emailFilters, page: emailPage, limit: 20 }),
  })

  // 短信日志查询
  const { data: smsLogs, isLoading: smsLoading } = useQuery({
    queryKey: ['smsLogs', smsPage, smsFilters],
    queryFn: () => getSmsLogs({ ...smsFilters, page: smsPage, limit: 20 }),
  })

  // 统计信息查询
  const { data: statistics } = useQuery({
    queryKey: ['logStatistics'],
    queryFn: getLogStatistics,
  })

  // 库存日志查询
  const { data: inventoryLogs, isLoading: inventoryLoading } = useQuery({
    queryKey: ['inventoryLogs', inventoryPage, inventoryFilters],
    queryFn: () =>
      getInventoryLogs({
        ...inventoryFilters,
        inventory_id: inventoryFilters.inventory_id
          ? parseInt(inventoryFilters.inventory_id) || undefined
          : undefined,
        page: inventoryPage,
        limit: 20,
      }),
  })
  const activeLogTabLabel =
    activeTab === 'operations'
      ? t.admin.operationLogs
      : activeTab === 'emails'
        ? t.admin.emailLogs
        : activeTab === 'inventories'
          ? t.admin.inventoryLogs
          : activeTab === 'sms'
            ? t.admin.smsLogs
            : t.admin.systemLogs
  const handleExport = useCallback(() => {
    let path = '/api/admin/logs/operations/export'
    let fileName = `operation_logs_${new Date().toISOString().slice(0, 10)}.xlsx`
    const params = new URLSearchParams()

    switch (activeTab) {
      case 'emails':
        path = '/api/admin/logs/emails/export'
        fileName = `email_logs_${new Date().toISOString().slice(0, 10)}.xlsx`
        if (emailFilters.status) params.append('status', emailFilters.status)
        if (emailFilters.event_type) params.append('event_type', emailFilters.event_type)
        if (emailFilters.to_email) params.append('to_email', emailFilters.to_email)
        if (emailFilters.start_date) params.append('start_date', emailFilters.start_date)
        if (emailFilters.end_date) params.append('end_date', emailFilters.end_date)
        break
      case 'inventories':
        path = '/api/admin/logs/inventories/export'
        fileName = `inventory_logs_${new Date().toISOString().slice(0, 10)}.xlsx`
        if (inventoryFilters.source) params.append('source', inventoryFilters.source)
        if (inventoryFilters.type) params.append('type', inventoryFilters.type)
        if (inventoryFilters.inventory_id)
          params.append('inventory_id', inventoryFilters.inventory_id)
        if (inventoryFilters.order_no) params.append('order_no', inventoryFilters.order_no)
        if (inventoryFilters.start_date) params.append('start_date', inventoryFilters.start_date)
        if (inventoryFilters.end_date) params.append('end_date', inventoryFilters.end_date)
        break
      case 'sms':
        path = '/api/admin/logs/sms/export'
        fileName = `sms_logs_${new Date().toISOString().slice(0, 10)}.xlsx`
        if (smsFilters.status) params.append('status', smsFilters.status)
        if (smsFilters.event_type) params.append('event_type', smsFilters.event_type)
        if (smsFilters.phone) params.append('phone', smsFilters.phone)
        if (smsFilters.start_date) params.append('start_date', smsFilters.start_date)
        if (smsFilters.end_date) params.append('end_date', smsFilters.end_date)
        break
      case 'operations':
      default:
        if (operationFilters.action) params.append('action', operationFilters.action)
        if (operationFilters.resource_type)
          params.append('resource_type', operationFilters.resource_type)
        if (operationFilters.resource_id) params.append('resource_id', operationFilters.resource_id)
        if (operationFilters.order_no) params.append('order_no', operationFilters.order_no)
        if (operationFilters.user_id) params.append('user_id', operationFilters.user_id)
        if (operationFilters.start_date) params.append('start_date', operationFilters.start_date)
        if (operationFilters.end_date) params.append('end_date', operationFilters.end_date)
        break
    }

    const url = resolveClientAPIProxyURL(`${path}${params.toString() ? `?${params}` : ''}`)
    fetch(url)
      .then(async (res) => {
        if (!res.ok) {
          throw new Error(await readFetchErrorMessage(res, t.admin.exportFailed))
        }
        return res.blob()
      })
      .then((blob) => {
        const blobUrl = window.URL.createObjectURL(blob)
        const a = document.createElement('a')
        a.href = blobUrl
        a.download = fileName
        document.body.appendChild(a)
        a.click()
        document.body.removeChild(a)
        window.URL.revokeObjectURL(blobUrl)
        toast.success(t.admin.logsExportSuccess)
      })
      .catch((err) => {
        toast.error(`${t.admin.exportFailed}: ${err.message}`)
      })
  }, [
    activeTab,
    emailFilters,
    inventoryFilters,
    operationFilters,
    readFetchErrorMessage,
    smsFilters,
    t.admin.exportFailed,
    t.admin.logsExportSuccess,
    toast,
  ])
  const activeLogFilters = (
    activeTab === 'operations'
      ? [
          operationFilters.action,
          operationFilters.resource_type,
          operationFilters.resource_id,
          operationFilters.order_no,
          operationFilters.user_id,
          operationFilters.start_date,
          operationFilters.end_date,
        ]
      : activeTab === 'emails'
        ? [
            emailFilters.status,
            emailFilters.event_type,
            emailFilters.to_email,
            emailFilters.start_date,
            emailFilters.end_date,
          ]
        : activeTab === 'inventories'
          ? [
              inventoryFilters.source,
              inventoryFilters.type,
              inventoryFilters.inventory_id,
              inventoryFilters.order_no,
              inventoryFilters.start_date,
              inventoryFilters.end_date,
            ]
          : [
              smsFilters.status,
              smsFilters.event_type,
              smsFilters.phone,
              smsFilters.start_date,
              smsFilters.end_date,
            ]
  ).filter(Boolean)
  const resolveOperationResourceLabel = useCallback(
    (resourceType: string) => {
      switch (resourceType) {
        case 'auth':
          return t.admin.resourceAuth
        case 'order':
          return t.admin.resourceOrder
        case 'user':
          return t.admin.resourceUser
        case 'admin':
          return t.admin.resourceAdmin
        case 'api_key':
          return t.admin.resourceApiKey
        case 'product':
          return t.admin.logResourceProduct
        case 'inventory':
          return t.admin.logResourceInventory
        case 'payment_method':
          return t.admin.logResourcePaymentMethod
        case 'payment':
          return t.admin.logResourcePayment
        case 'marketing_batch':
          return t.admin.logResourceMarketingBatch
        case 'plugin':
          return t.admin.logResourcePlugin
        case 'system':
          return t.admin.system
        case 'system_config':
          return t.admin.logResourceSystemConfig
        default:
          return locale === 'zh' ? String(resourceType || '') : humanizeLogToken(resourceType)
      }
    },
    [locale, t.admin]
  )
  const resolveOperationActionLabel = useCallback(
    (action: string) => {
      switch (action) {
        case 'create':
          return t.admin.actionCreate
        case 'update':
          return t.admin.actionUpdate
        case 'delete':
          return t.admin.actionDelete
        case 'login':
          return t.admin.actionLogin
        case 'register':
          return t.admin.logActionRegister
        case 'verify_email':
          return t.admin.logActionVerifyEmail
        case 'assign_tracking':
          return t.admin.logActionAssignTracking
        case 'complete':
          return t.admin.logActionComplete
        case 'cancel':
          return t.admin.logActionCancel
        case 'refund':
          return t.admin.logActionRefund
        case 'mark_paid':
          return t.admin.logActionMarkPaid
        case 'deliver_virtual_stock':
          return t.admin.logActionDeliverVirtualStock
        case 'request_resubmit':
          return t.admin.logActionRequestResubmit
        case 'batch_complete_orders':
          return t.admin.logActionBatchCompleteOrders
        case 'batch_cancel_orders':
          return t.admin.logActionBatchCancelOrders
        case 'batch_delete_orders':
          return t.admin.logActionBatchDeleteOrders
        case 'create_draft':
          return t.admin.logActionCreateDraft
        case 'create_draft_failed':
          return t.admin.logActionCreateDraftFailed
        case 'admin_create_order':
          return t.admin.logActionAdminCreateOrder
        case 'update_price':
          return t.admin.logActionUpdatePrice
        case 'form_submit':
          return t.admin.logActionFormSubmit
        case 'form_submit_failed':
          return t.admin.logActionFormSubmitFailed
        case 'payment_method_selected':
          return t.admin.logActionPaymentMethodSelected
        case 'adjust_stock':
          return t.admin.logActionAdjustStock
        case 'import':
          return t.admin.logActionImport
        case 'payment_method_market_preview':
          return t.admin.logActionPaymentMethodMarketPreview
        case 'payment_method_init':
          return t.admin.logActionPaymentMethodInit
        case 'payment_method_legacy_migration':
          return t.admin.logActionPaymentMethodLegacyMigration
        case 'payment_success':
          return t.admin.logActionPaymentSuccess
        case 'payment_update_failed':
          return t.admin.logActionPaymentUpdateFailed
        case 'payment_polling_add':
          return t.admin.logActionPaymentPollingAdd
        case 'payment_polling_timeout':
          return t.admin.logActionPaymentPollingTimeout
        case 'payment_polling_max_retries':
          return t.admin.logActionPaymentPollingMaxRetries
        case 'payment_polling_check_failed':
          return t.admin.logActionPaymentPollingCheckFailed
        case 'payment_polling_backfill':
          return t.admin.logActionPaymentPollingBackfill
        case 'check_auto_delivery_failed':
          return t.admin.logActionCheckAutoDeliveryFailed
        case 'virtual_delivery_failed':
          return t.admin.logActionVirtualDeliveryFailed
        case 'order_auto_cancelled':
          return t.admin.logActionOrderAutoCancelled
        case 'queue_marketing':
          return t.admin.logActionQueueMarketing
        case 'maintenance':
          return t.admin.logActionMaintenance
        case 'order_cancel_service_start':
          return t.admin.logActionOrderCancelServiceStart
        case 'order_cancel_service_stop':
          return t.admin.logActionOrderCancelServiceStop
        case 'order_auto_cancel':
          return t.admin.logActionOrderAutoCancel
        case 'payment_polling_start':
          return t.admin.logActionPaymentPollingStart
        case 'payment_polling_stop':
          return t.admin.logActionPaymentPollingStop
        case 'payment_polling_recover':
          return t.admin.logActionPaymentPollingRecover
        case 'ticket_attachment_cleanup_start':
          return t.admin.logActionTicketAttachmentCleanupStart
        case 'ticket_attachment_cleanup_stop':
          return t.admin.logActionTicketAttachmentCleanupStop
        case 'ticket_attachment_cleanup':
          return t.admin.logActionTicketAttachmentCleanup
        case 'ticket_auto_close_start':
          return t.admin.logActionTicketAutoCloseStart
        case 'ticket_auto_close_stop':
          return t.admin.logActionTicketAutoCloseStop
        case 'ticket_auto_close':
          return t.admin.logActionTicketAutoClose
        default:
          return locale === 'zh' ? String(action || '') : humanizeLogToken(action)
      }
    },
    [locale, t.admin]
  )
  const operationResourceOptions = useMemo(() => {
    const observedResourceTypes = uniqueStrings(
      (operationLogs?.data?.items || []).map((item: any) => item?.resource_type)
    )
    const orderedValues = uniqueStrings([...operationResourceOrder, ...observedResourceTypes])
    return orderedValues.map((value) => ({
      value,
      label: resolveOperationResourceLabel(value),
    }))
  }, [operationLogs?.data?.items, resolveOperationResourceLabel])
  const operationActionOptions = useMemo(() => {
    const selectedResourceType = operationFilters.resource_type
    const defaultActions = selectedResourceType
      ? operationActionsByResource[selectedResourceType] || []
      : Object.values(operationActionsByResource).flat()
    const observedActions = uniqueStrings(
      (operationLogs?.data?.items || []).map((item: any) => item?.action)
    )
    const orderedValues = uniqueStrings([...defaultActions, ...observedActions])
    return orderedValues.map((value) => ({
      value,
      label: resolveOperationActionLabel(value),
    }))
  }, [operationFilters.resource_type, operationLogs?.data?.items, resolveOperationActionLabel])
  const adminLogsPluginContext = {
    view: 'admin_logs',
    active_tab: activeTab,
    filters: {
      operations: operationFilters,
      emails: emailFilters,
      inventories: inventoryFilters,
      sms: smsFilters,
    },
    pagination: {
      operations: {
        page: operationPage,
        total_pages: operationLogs?.data?.pagination?.total_pages,
        total: operationLogs?.data?.pagination?.total,
        limit: operationLogs?.data?.pagination?.limit,
      },
      emails: {
        page: emailPage,
        total_pages: emailLogs?.data?.pagination?.total_pages,
        total: emailLogs?.data?.pagination?.total,
        limit: emailLogs?.data?.pagination?.limit,
      },
      inventories: {
        page: inventoryPage,
        total_pages: inventoryLogs?.data?.pagination?.total_pages,
        total: inventoryLogs?.data?.pagination?.total,
        limit: inventoryLogs?.data?.pagination?.limit,
      },
      sms: {
        page: smsPage,
        total_pages: smsLogs?.data?.pagination?.total_pages,
        total: smsLogs?.data?.pagination?.total,
        limit: smsLogs?.data?.pagination?.limit,
      },
    },
    selection: {
      selected_email_count: selectedEmails.length,
      selected_email_ids: selectedEmails.slice(0, 20),
    },
    summary: {
      active_filter_count: activeLogFilters.length,
      active_tab_label: activeLogTabLabel,
    },
  }
  // 重试邮件
  const retryMutation = useMutation<any, Error, number[]>({
    mutationFn: async (emailIds: number[]) => {
      return await retryFailedEmails(emailIds)
    },
    onSuccess: () => {
      toast.success(t.admin.emailRetryQueued)
      queryClient.invalidateQueries({ queryKey: ['emailLogs'] })
      setSelectedEmails([])
    },
    onError: (error: unknown) => {
      toast.error(resolveLogError(error, t.admin.retryError))
    },
  })

  // 操作日志列定义
  const operationColumns = [
    {
      header: 'ID',
      accessorKey: 'id',
      cell: ({ row }: { row: { original: any } }) => (
        <span className="text-xs text-muted-foreground">#{row.original.id}</span>
      ),
    },
    {
      header: t.admin.actions,
      accessorKey: 'action',
      cell: ({ row }: { row: { original: any } }) => (
        <Badge variant="outline">{resolveOperationActionLabel(row.original.action)}</Badge>
      ),
    },
    {
      header: t.admin.resourceType,
      accessorKey: 'resource_type',
      cell: ({ row }: { row: { original: any } }) =>
        row.original.resource_type ? (
          <span className="text-sm">
            {resolveOperationResourceLabel(row.original.resource_type)}
          </span>
        ) : (
          <span className="text-muted-foreground">-</span>
        ),
    },
    {
      header: t.admin.resourceIdLabel,
      accessorKey: 'resource_id',
      cell: ({ row }: { row: { original: any } }) => {
        const resourceId = row.original.resource_id
        const isOrderResource = row.original.resource_type === 'order'
        return resourceId ? (
          isOrderResource ? (
            <Link
              href={`/admin/orders/${resourceId}`}
              className="text-sm font-medium text-primary hover:underline"
            >
              #{resourceId}
            </Link>
          ) : (
            <span className="text-sm">#{resourceId}</span>
          )
        ) : (
          <span className="text-muted-foreground">-</span>
        )
      },
    },
    {
      header: t.admin.orderNoLabel,
      cell: ({ row }: { row: { original: any } }) => {
        const orderNo =
          getOperationDetailValue(row.original.details, 'order_no') ||
          (operationFilters.order_no && row.original.resource_type === 'order'
            ? operationFilters.order_no
            : '')
        return orderNo ? (
          <Link
            href={`/admin/orders?search=${encodeURIComponent(orderNo)}`}
            className="font-mono text-sm text-primary hover:underline"
          >
            {orderNo}
          </Link>
        ) : (
          <span className="text-muted-foreground">-</span>
        )
      },
    },
    {
      header: t.admin.operatorUser,
      cell: ({ row }: { row: { original: any } }) =>
        row.original.operator_name ? (
          <div className="flex flex-col">
            <span className="text-sm font-medium">{row.original.operator_name}</span>
            <span className="text-xs text-muted-foreground">{t.admin.apiPlatform}</span>
          </div>
        ) : row.original.user ? (
          <div className="flex flex-col">
            <span className="text-sm font-medium">
              {row.original.user.name || row.original.user.email}
            </span>
            <span className="text-xs text-muted-foreground">{row.original.user.role}</span>
          </div>
        ) : (
          <span className="text-muted-foreground">{t.admin.system}</span>
        ),
    },
    {
      header: t.admin.ipAddress,
      accessorKey: 'ip_address',
      cell: ({ row }: { row: { original: any } }) =>
        row.original.ip_address ? (
          <code className="rounded bg-muted px-2 py-1 text-xs">{row.original.ip_address}</code>
        ) : (
          <span className="text-muted-foreground">-</span>
        ),
    },
    {
      header: t.admin.time,
      cell: ({ row }: { row: { original: any } }) =>
        row.original.created_at ? formatDate(row.original.created_at) : '-',
    },
  ]

  // 邮件日志列定义
  const emailColumns = [
    {
      header: 'ID',
      accessorKey: 'id',
      cell: ({ row }: { row: { original: any } }) => (
        <span className="text-xs text-muted-foreground">#{row.original.id}</span>
      ),
    },
    {
      header: t.admin.recipient,
      accessorKey: 'to_email',
      cell: ({ row }: { row: { original: any } }) => (
        <span className="text-sm">{row.original.to_email}</span>
      ),
    },
    {
      header: t.admin.subject,
      accessorKey: 'subject',
      cell: ({ row }: { row: { original: any } }) => (
        <span className="block max-w-xs truncate text-sm">{row.original.subject}</span>
      ),
    },
    {
      header: t.admin.eventType,
      accessorKey: 'event_type',
      cell: ({ row }: { row: { original: any } }) =>
        row.original.event_type ? (
          <Badge variant="secondary">{row.original.event_type}</Badge>
        ) : (
          <span className="text-muted-foreground">-</span>
        ),
    },
    {
      header: t.admin.status,
      accessorKey: 'status',
      cell: ({ row }: { row: { original: any } }) => {
        const status = row.original.status
        const variants: Record<string, 'default' | 'secondary' | 'destructive' | 'outline'> = {
          sent: 'default',
          pending: 'secondary',
          failed: 'destructive',
          expired: 'outline',
        }
        return <Badge variant={variants[status] || 'secondary'}>{status}</Badge>
      },
    },
    {
      header: t.admin.retryCount,
      accessorKey: 'retry_count',
      cell: ({ row }: { row: { original: any } }) => (
        <span className="text-sm">{row.original.retry_count || 0}</span>
      ),
    },
    {
      header: t.admin.createdAt,
      cell: ({ row }: { row: { original: any } }) =>
        row.original.created_at ? formatDate(row.original.created_at) : '-',
    },
    {
      header: t.admin.sentTime,
      cell: ({ row }: { row: { original: any } }) =>
        row.original.sent_at ? (
          formatDate(row.original.sent_at)
        ) : (
          <span className="text-muted-foreground">-</span>
        ),
    },
  ]

  return (
    <div className="space-y-6">
      <PluginSlot slot="admin.logs.top" context={adminLogsPluginContext} />
      <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
        <div>
          <h1 className="text-3xl font-bold">{t.admin.systemLogs}</h1>
          <p className="mt-1 text-sm text-muted-foreground">{activeLogTabLabel}</p>
          <p className="text-xs text-muted-foreground">
            {activeLogFilters.length > 0
              ? t.admin.logsFilterSummary.replace('{count}', String(activeLogFilters.length))
              : t.admin.logsFilterHint}
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={handleExport}>
            <Download className="mr-2 h-4 w-4" />
            {t.admin.exportCurrentLogs}
          </Button>
        </div>
      </div>

      {/* 统计卡片 */}
      {statistics?.data && (
        <div className="grid gap-4 md:grid-cols-3 lg:grid-cols-6">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">{t.admin.todayOperations}</CardTitle>
              <FileText className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">
                {statistics.data.operation_log_count?.today ?? 0}
              </div>
              <p className="text-xs text-muted-foreground">
                {t.admin.thisWeek}: {statistics.data.operation_log_count?.week ?? 0}
              </p>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">{t.admin.todayEmails}</CardTitle>
              <Mail className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">
                {statistics.data.email_log_count?.today ?? 0}
              </div>
              <p className="text-xs text-muted-foreground">
                {t.admin.thisWeek}: {statistics.data.email_log_count?.week ?? 0}
              </p>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">{t.admin.pendingEmails}</CardTitle>
              <Mail className="h-4 w-4 text-yellow-500" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold text-yellow-600">
                {statistics.data.email_log_count?.pending ?? 0}
              </div>
              <p className="text-xs text-muted-foreground">{t.admin.pendingQueue}</p>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">{t.admin.failedEmails}</CardTitle>
              <Mail className="h-4 w-4 text-red-500" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold text-red-600">
                {statistics.data.email_log_count?.failed ?? 0}
              </div>
              <p className="text-xs text-muted-foreground">{t.admin.needRetry}</p>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">{t.admin.todaySms}</CardTitle>
              <Smartphone className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{statistics.data.sms_log_count?.today || 0}</div>
              <p className="text-xs text-muted-foreground">
                {t.admin.thisWeek}: {statistics.data.sms_log_count?.week || 0}
              </p>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">{t.admin.failedSms}</CardTitle>
              <Smartphone className="h-4 w-4 text-red-500" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold text-red-600">
                {statistics.data.sms_log_count?.failed || 0}
              </div>
              <p className="text-xs text-muted-foreground">{t.admin.needRetry}</p>
            </CardContent>
          </Card>
        </div>
      )}

      {/* 日志标签页 */}
      <Tabs value={activeTab} onValueChange={setActiveTab}>
        <TabsList>
          <TabsTrigger value="operations">
            <FileText className="mr-2 h-4 w-4" />
            {t.admin.operationLogs}
          </TabsTrigger>
          <TabsTrigger value="emails">
            <Mail className="mr-2 h-4 w-4" />
            {t.admin.emailLogs}
          </TabsTrigger>
          <TabsTrigger value="inventories">
            <Package className="mr-2 h-4 w-4" />
            {t.admin.inventoryLogs}
          </TabsTrigger>
          <TabsTrigger value="sms">
            <Smartphone className="mr-2 h-4 w-4" />
            {t.admin.smsLogs}
          </TabsTrigger>
        </TabsList>

        <TabsContent value="operations" className="space-y-4">
          {/* 筛选器 */}
          <Card>
            <CardHeader>
              <CardTitle className="text-base">{t.admin.filterConditions}</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
                <div>
                  <Label htmlFor="resource_type">{t.admin.resourceType}</Label>
                  <Select
                    value={operationFilters.resource_type || 'all'}
                    onValueChange={(value) => {
                      setOperationFilters({
                        ...operationFilters,
                        resource_type: value === 'all' ? '' : value,
                        action: '',
                      })
                      setOperationPage(1)
                    }}
                  >
                    <SelectTrigger id="resource_type">
                      <SelectValue placeholder={t.admin.all} />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="all">{t.admin.all}</SelectItem>
                      {operationResourceOptions.map((option) => (
                        <SelectItem key={option.value} value={option.value}>
                          {option.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div>
                  <Label htmlFor="action">{t.admin.operationType}</Label>
                  <Select
                    value={operationFilters.action || 'all'}
                    onValueChange={(value) => {
                      setOperationFilters({
                        ...operationFilters,
                        action: value === 'all' ? '' : value,
                      })
                      setOperationPage(1)
                    }}
                  >
                    <SelectTrigger id="action">
                      <SelectValue placeholder={t.admin.all} />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="all">{t.admin.all}</SelectItem>
                      {operationActionOptions.map((option) => (
                        <SelectItem key={option.value} value={option.value}>
                          {option.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div>
                  <Label htmlFor="resource_id">{t.admin.resourceIdLabel}</Label>
                  <Input
                    id="resource_id"
                    type="number"
                    min="1"
                    placeholder={t.admin.resourceIdLabel}
                    value={operationFilters.resource_id}
                    onChange={(e) => {
                      setOperationFilters({ ...operationFilters, resource_id: e.target.value })
                      setOperationPage(1)
                    }}
                  />
                </div>
                <div>
                  <Label htmlFor="operation_order_no">{t.admin.orderNoLabel}</Label>
                  <Input
                    id="operation_order_no"
                    placeholder={t.admin.orderNoLabel}
                    value={operationFilters.order_no}
                    onChange={(e) => {
                      setOperationFilters({ ...operationFilters, order_no: e.target.value })
                      setOperationPage(1)
                    }}
                  />
                </div>
                <div>
                  <Label htmlFor="operation_user_id">{t.admin.userId}</Label>
                  <Input
                    id="operation_user_id"
                    type="number"
                    min="1"
                    placeholder={t.admin.userId}
                    value={operationFilters.user_id}
                    onChange={(e) => {
                      setOperationFilters({ ...operationFilters, user_id: e.target.value })
                      setOperationPage(1)
                    }}
                  />
                </div>
                <div>
                  <Label htmlFor="start_date">{t.admin.startDate}</Label>
                  <Input
                    id="start_date"
                    type="date"
                    value={operationFilters.start_date}
                    onChange={(e) => {
                      setOperationFilters({ ...operationFilters, start_date: e.target.value })
                      setOperationPage(1)
                    }}
                  />
                </div>
                <div>
                  <Label htmlFor="end_date">{t.admin.endDate}</Label>
                  <Input
                    id="end_date"
                    type="date"
                    value={operationFilters.end_date}
                    onChange={(e) => {
                      setOperationFilters({ ...operationFilters, end_date: e.target.value })
                      setOperationPage(1)
                    }}
                  />
                </div>
                <div className="flex items-end">
                  <Button
                    variant="outline"
                    onClick={() => {
                      setOperationFilters({
                        action: '',
                        resource_type: '',
                        resource_id: '',
                        order_no: '',
                        user_id: '',
                        start_date: '',
                        end_date: '',
                      })
                      setOperationPage(1)
                    }}
                  >
                    {t.admin.reset}
                  </Button>
                </div>
              </div>
            </CardContent>
          </Card>

          <DataTable
            columns={operationColumns}
            data={operationLogs?.data?.items || []}
            isLoading={operationLoading}
            pagination={{
              page: operationPage,
              total_pages: operationLogs?.data?.pagination?.total_pages || 1,
              onPageChange: setOperationPage,
            }}
          />
        </TabsContent>

        <TabsContent value="emails" className="space-y-4">
          {/* 筛选器 */}
          <Card>
            <CardHeader>
              <CardTitle className="text-base">{t.admin.filterConditions}</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="grid gap-4 md:grid-cols-5">
                <div>
                  <Label htmlFor="status">{t.admin.status}</Label>
                  <Select
                    value={emailFilters.status || 'all'}
                    onValueChange={(value) =>
                      setEmailFilters({ ...emailFilters, status: value === 'all' ? '' : value })
                    }
                  >
                    <SelectTrigger id="status">
                      <SelectValue placeholder={t.admin.all} />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="all">{t.admin.all}</SelectItem>
                      <SelectItem value="pending">{t.admin.pending}</SelectItem>
                      <SelectItem value="sent">{t.admin.sent}</SelectItem>
                      <SelectItem value="failed">{t.admin.failed}</SelectItem>
                      <SelectItem value="expired">{t.admin.expired}</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div>
                  <Label htmlFor="to_email">{t.admin.recipient}</Label>
                  <Input
                    id="to_email"
                    type="email"
                    placeholder={t.admin.searchEmail}
                    value={emailFilters.to_email}
                    onChange={(e) => setEmailFilters({ ...emailFilters, to_email: e.target.value })}
                  />
                </div>
                <div>
                  <Label htmlFor="email_start_date">{t.admin.startDate}</Label>
                  <Input
                    id="email_start_date"
                    type="date"
                    value={emailFilters.start_date}
                    onChange={(e) =>
                      setEmailFilters({ ...emailFilters, start_date: e.target.value })
                    }
                  />
                </div>
                <div>
                  <Label htmlFor="email_end_date">{t.admin.endDate}</Label>
                  <Input
                    id="email_end_date"
                    type="date"
                    value={emailFilters.end_date}
                    onChange={(e) => setEmailFilters({ ...emailFilters, end_date: e.target.value })}
                  />
                </div>
                <div className="flex items-end gap-2">
                  <Button
                    variant="outline"
                    onClick={() => {
                      setEmailFilters({
                        status: '',
                        event_type: '',
                        to_email: '',
                        start_date: '',
                        end_date: '',
                      })
                      setEmailPage(1)
                    }}
                  >
                    {t.admin.reset}
                  </Button>
                  {selectedEmails.length > 0 && (
                    <Button
                      onClick={() => retryMutation.mutate(selectedEmails)}
                      disabled={retryMutation.isPending}
                    >
                      <RefreshCw className="mr-2 h-4 w-4" />
                      {t.admin.retryFailed}
                    </Button>
                  )}
                </div>
              </div>
            </CardContent>
          </Card>

          <DataTable
            columns={emailColumns}
            data={emailLogs?.data?.items || []}
            isLoading={emailLoading}
            pagination={{
              page: emailPage,
              total_pages: emailLogs?.data?.pagination?.total_pages || 1,
              onPageChange: setEmailPage,
            }}
          />
        </TabsContent>

        <TabsContent value="inventories" className="space-y-4">
          {/* 筛选器 */}
          <Card>
            <CardHeader>
              <CardTitle className="text-base">{t.admin.filterConditions}</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="grid gap-4 md:grid-cols-6">
                <div>
                  <Label htmlFor="inv_source">{t.admin.inventorySource}</Label>
                  <Select
                    value={inventoryFilters.source || 'all'}
                    onValueChange={(value) =>
                      setInventoryFilters({
                        ...inventoryFilters,
                        source: value === 'all' ? '' : value,
                      })
                    }
                  >
                    <SelectTrigger id="inv_source">
                      <SelectValue placeholder={t.admin.all} />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="all">{t.admin.all}</SelectItem>
                      <SelectItem value="physical">{t.admin.physicalInventory}</SelectItem>
                      <SelectItem value="virtual">{t.admin.virtualInventory}</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div>
                  <Label htmlFor="inv_type">{t.admin.operationType}</Label>
                  <Select
                    value={inventoryFilters.type || 'all'}
                    onValueChange={(value) =>
                      setInventoryFilters({
                        ...inventoryFilters,
                        type: value === 'all' ? '' : value,
                      })
                    }
                  >
                    <SelectTrigger id="inv_type">
                      <SelectValue placeholder={t.admin.all} />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="all">{t.admin.all}</SelectItem>
                      <SelectItem value="in">{t.admin.stockIn}</SelectItem>
                      <SelectItem value="out">{t.admin.stockOut}</SelectItem>
                      <SelectItem value="reserve">{t.admin.reserve}</SelectItem>
                      <SelectItem value="release">{t.admin.release}</SelectItem>
                      <SelectItem value="adjust">{t.admin.adjust}</SelectItem>
                      <SelectItem value="import">{t.admin.stockImport}</SelectItem>
                      <SelectItem value="deliver">{t.admin.stockDeliver}</SelectItem>
                      <SelectItem value="delete">{t.admin.stockDelete}</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div>
                  <Label htmlFor="inv_id">{t.admin.inventoryIdLabel}</Label>
                  <Input
                    id="inv_id"
                    type="number"
                    placeholder={t.admin.inventoryIdLabel}
                    value={inventoryFilters.inventory_id}
                    onChange={(e) =>
                      setInventoryFilters({ ...inventoryFilters, inventory_id: e.target.value })
                    }
                  />
                </div>
                <div>
                  <Label htmlFor="inv_order">{t.admin.orderNoLabel}</Label>
                  <Input
                    id="inv_order"
                    placeholder={t.admin.orderNoLabel}
                    value={inventoryFilters.order_no}
                    onChange={(e) =>
                      setInventoryFilters({ ...inventoryFilters, order_no: e.target.value })
                    }
                  />
                </div>
                <div>
                  <Label htmlFor="inv_start">{t.admin.startDate}</Label>
                  <Input
                    id="inv_start"
                    type="date"
                    value={inventoryFilters.start_date}
                    onChange={(e) =>
                      setInventoryFilters({ ...inventoryFilters, start_date: e.target.value })
                    }
                  />
                </div>
                <div className="flex items-end">
                  <Button
                    variant="outline"
                    onClick={() => {
                      setInventoryFilters({
                        source: '',
                        type: '',
                        inventory_id: '',
                        order_no: '',
                        start_date: '',
                        end_date: '',
                      })
                      setInventoryPage(1)
                    }}
                  >
                    {t.admin.reset}
                  </Button>
                </div>
              </div>
            </CardContent>
          </Card>

          <DataTable
            columns={[
              {
                header: 'ID',
                accessorKey: 'id',
                cell: ({ row }: any) => (
                  <span className="text-xs text-muted-foreground">#{row.original.id}</span>
                ),
              },
              {
                header: t.admin.inventoryIdLabel,
                accessorKey: 'inventory_id',
                cell: ({ row }: any) => (
                  <Badge variant="outline">{row.original.inventory_id}</Badge>
                ),
              },
              {
                header: t.admin.type,
                accessorKey: 'type',
                cell: ({ row }: any) => {
                  const typeMap: Record<
                    string,
                    { label: string; color: 'default' | 'secondary' | 'destructive' }
                  > = {
                    in: { label: t.admin.stockIn, color: 'default' },
                    out: { label: t.admin.stockOut, color: 'destructive' },
                    reserve: { label: t.admin.reserve, color: 'secondary' },
                    release: { label: t.admin.release, color: 'secondary' },
                    adjust: { label: t.admin.adjust, color: 'default' },
                    import: { label: t.admin.stockImport, color: 'default' },
                    deliver: { label: t.admin.stockDeliver, color: 'default' },
                    delete: { label: t.admin.stockDelete, color: 'destructive' },
                  }
                  const config = typeMap[row.original.type] || {
                    label: row.original.type,
                    color: 'secondary',
                  }
                  return <Badge variant={config.color}>{config.label}</Badge>
                },
              },
              {
                header: t.admin.quantity,
                accessorKey: 'quantity',
                cell: ({ row }: any) => (
                  <span className={row.original.quantity > 0 ? 'text-green-600' : 'text-red-600'}>
                    {row.original.quantity > 0 ? '+' : ''}
                    {row.original.quantity}
                  </span>
                ),
              },
              {
                header: t.admin.beforeChange,
                accessorKey: 'before_stock',
                cell: ({ row }: any) =>
                  row.original.source === 'virtual' ? (
                    <span className="text-muted-foreground">-</span>
                  ) : (
                    row.original.before_stock
                  ),
              },
              {
                header: t.admin.afterChange,
                accessorKey: 'after_stock',
                cell: ({ row }: any) =>
                  row.original.source === 'virtual' ? (
                    <span className="text-muted-foreground">-</span>
                  ) : (
                    row.original.after_stock
                  ),
              },
              {
                header: t.admin.batchNo,
                accessorKey: 'batch_no',
                cell: ({ row }: any) =>
                  row.original.batch_no ? (
                    <code className="rounded bg-muted px-2 py-1 text-xs">
                      {row.original.batch_no}
                    </code>
                  ) : (
                    <span className="text-muted-foreground">-</span>
                  ),
              },
              {
                header: t.admin.orderNoLabel,
                accessorKey: 'order_no',
                cell: ({ row }: any) =>
                  row.original.order_no || <span className="text-muted-foreground">-</span>,
              },
              {
                header: t.admin.operator,
                accessorKey: 'operator',
              },
              {
                header: t.admin.reason,
                accessorKey: 'reason',
                cell: ({ row }: any) => (
                  <span className="block max-w-xs truncate text-sm">{row.original.reason}</span>
                ),
              },
              {
                header: t.admin.time,
                cell: ({ row }: any) =>
                  row.original.created_at ? formatDate(row.original.created_at) : '-',
              },
            ]}
            data={inventoryLogs?.data?.items || []}
            isLoading={inventoryLoading}
            pagination={{
              page: inventoryPage,
              total_pages: inventoryLogs?.data?.pagination?.total_pages || 1,
              onPageChange: setInventoryPage,
            }}
          />
        </TabsContent>

        <TabsContent value="sms" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">{t.admin.filterConditions}</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="grid gap-4 md:grid-cols-5">
                <div>
                  <Label htmlFor="sms_status">{t.admin.status}</Label>
                  <Select
                    value={smsFilters.status || 'all'}
                    onValueChange={(value) =>
                      setSmsFilters({ ...smsFilters, status: value === 'all' ? '' : value })
                    }
                  >
                    <SelectTrigger id="sms_status">
                      <SelectValue placeholder={t.admin.all} />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="all">{t.admin.all}</SelectItem>
                      <SelectItem value="pending">{t.admin.pending}</SelectItem>
                      <SelectItem value="sent">{t.admin.sent}</SelectItem>
                      <SelectItem value="failed">{t.admin.failed}</SelectItem>
                      <SelectItem value="expired">{t.admin.expired}</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div>
                  <Label htmlFor="sms_phone">{t.admin.smsPhone}</Label>
                  <Input
                    id="sms_phone"
                    placeholder={t.admin.smsPhone}
                    value={smsFilters.phone}
                    onChange={(e) => setSmsFilters({ ...smsFilters, phone: e.target.value })}
                  />
                </div>
                <div>
                  <Label htmlFor="sms_start_date">{t.admin.startDate}</Label>
                  <Input
                    id="sms_start_date"
                    type="date"
                    value={smsFilters.start_date}
                    onChange={(e) => setSmsFilters({ ...smsFilters, start_date: e.target.value })}
                  />
                </div>
                <div>
                  <Label htmlFor="sms_end_date">{t.admin.endDate}</Label>
                  <Input
                    id="sms_end_date"
                    type="date"
                    value={smsFilters.end_date}
                    onChange={(e) => setSmsFilters({ ...smsFilters, end_date: e.target.value })}
                  />
                </div>
                <div className="flex items-end">
                  <Button
                    variant="outline"
                    onClick={() => {
                      setSmsFilters({
                        status: '',
                        event_type: '',
                        phone: '',
                        start_date: '',
                        end_date: '',
                      })
                      setSmsPage(1)
                    }}
                  >
                    {t.admin.reset}
                  </Button>
                </div>
              </div>
            </CardContent>
          </Card>

          <DataTable
            columns={[
              {
                header: 'ID',
                accessorKey: 'id',
                cell: ({ row }: any) => (
                  <span className="text-xs text-muted-foreground">#{row.original.id}</span>
                ),
              },
              {
                header: t.admin.smsPhone,
                accessorKey: 'phone',
                cell: ({ row }: any) => <span className="text-sm">{row.original.phone}</span>,
              },
              {
                header: t.admin.smsContent,
                accessorKey: 'content',
                cell: ({ row }: any) => (
                  <span className="block max-w-xs truncate text-sm">{row.original.content}</span>
                ),
              },
              {
                header: t.admin.smsEventType,
                accessorKey: 'event_type',
                cell: ({ row }: any) =>
                  row.original.event_type ? (
                    <Badge variant="secondary">{row.original.event_type}</Badge>
                  ) : (
                    <span className="text-muted-foreground">-</span>
                  ),
              },
              {
                header: t.admin.smsLogProvider,
                accessorKey: 'provider',
                cell: ({ row }: any) => <Badge variant="outline">{row.original.provider}</Badge>,
              },
              {
                header: t.admin.status,
                accessorKey: 'status',
                cell: ({ row }: any) => {
                  const status = row.original.status
                  const variants: Record<
                    string,
                    'default' | 'secondary' | 'destructive' | 'outline'
                  > = {
                    sent: 'default',
                    pending: 'secondary',
                    failed: 'destructive',
                    expired: 'outline',
                  }
                  return <Badge variant={variants[status] || 'secondary'}>{status}</Badge>
                },
              },
              {
                header: t.admin.time,
                cell: ({ row }: any) =>
                  row.original.created_at ? formatDate(row.original.created_at) : '-',
              },
            ]}
            data={smsLogs?.data?.items || []}
            isLoading={smsLoading}
            pagination={{
              page: smsPage,
              total_pages: smsLogs?.data?.pagination?.total_pages || 1,
              onPageChange: setSmsPage,
            }}
          />
        </TabsContent>
      </Tabs>
    </div>
  )
}
