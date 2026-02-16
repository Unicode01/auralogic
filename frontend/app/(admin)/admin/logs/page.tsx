'use client'

import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { getOperationLogs, getEmailLogs, getLogStatistics, retryFailedEmails, getInventoryLogs } from '@/lib/api'
import type { } from '@/lib/api'
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
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useToast } from '@/hooks/use-toast'
import { FileText, Mail, BarChart3, RefreshCw, Search, Package } from 'lucide-react'
import { formatDate } from '@/lib/utils'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'

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
  const [inventoryFilters, setInventoryFilters] = useState({
    type: '',
    inventory_id: '',
    order_no: '',
    start_date: '',
    end_date: '',
  })
  const [selectedEmails, setSelectedEmails] = useState<number[]>([])

  const queryClient = useQueryClient()
  const toast = useToast()

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

  // 统计信息查询
  const { data: statistics } = useQuery({
    queryKey: ['logStatistics'],
    queryFn: getLogStatistics,
  })

  // 库存日志查询
  const { data: inventoryLogs, isLoading: inventoryLoading } = useQuery({
    queryKey: ['inventoryLogs', inventoryPage, inventoryFilters],
    queryFn: () => getInventoryLogs({
      ...inventoryFilters,
      inventory_id: inventoryFilters.inventory_id ? parseInt(inventoryFilters.inventory_id) : undefined,
      page: inventoryPage,
      limit: 20
    }),
  })

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
    onError: (error: any) => {
      toast.error(error.message || t.admin.retryError)
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
        <Badge variant="outline">{row.original.action}</Badge>
      ),
    },
    {
      header: t.admin.resourceType,
      accessorKey: 'resource_type',
      cell: ({ row }: { row: { original: any } }) =>
        row.original.resource_type ? (
          <span className="text-sm">{row.original.resource_type}</span>
        ) : (
          <span className="text-muted-foreground">-</span>
        ),
    },
    {
      header: 'Resource ID',
      accessorKey: 'resource_id',
      cell: ({ row }: { row: { original: any } }) =>
        row.original.resource_id ? (
          <span className="text-sm">#{row.original.resource_id}</span>
        ) : (
          <span className="text-muted-foreground">-</span>
        ),
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
            <span className="text-sm font-medium">{row.original.user.name || row.original.user.email}</span>
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
          <code className="text-xs bg-muted px-2 py-1 rounded">{row.original.ip_address}</code>
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
        <span className="text-xs text-gray-500">#{row.original.id}</span>
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
        <span className="text-sm truncate max-w-xs block">{row.original.subject}</span>
      ),
    },
    {
      header: t.admin.eventType,
      accessorKey: 'event_type',
      cell: ({ row }: { row: { original: any } }) =>
        row.original.event_type ? (
          <Badge variant="secondary">{row.original.event_type}</Badge>
        ) : (
          <span className="text-gray-400">-</span>
        ),
    },
    {
      header: t.admin.status,
      accessorKey: 'status',
      cell: ({ row }: { row: { original: any } }) => {
        const status = row.original.status
        const variants: Record<string, 'default' | 'secondary' | 'destructive'> = {
          sent: 'default',
          pending: 'secondary',
          failed: 'destructive',
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
        row.original.sent_at ? formatDate(row.original.sent_at) : <span className="text-gray-400">-</span>,
    },
  ]

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-bold">{t.admin.systemLogs}</h1>
      </div>

      {/* 统计卡片 */}
      {statistics?.data && (
        <div className="grid gap-4 md:grid-cols-4">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">{t.admin.todayOperations}</CardTitle>
              <FileText className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{statistics.data.operation_log_count.today}</div>
              <p className="text-xs text-muted-foreground">
                {t.admin.thisWeek}: {statistics.data.operation_log_count.week}
              </p>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">{t.admin.todayEmails}</CardTitle>
              <Mail className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{statistics.data.email_log_count.today}</div>
              <p className="text-xs text-muted-foreground">
                {t.admin.thisWeek}: {statistics.data.email_log_count.week}
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
                {statistics.data.email_log_count.pending}
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
                {statistics.data.email_log_count.failed}
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
        </TabsList>

        <TabsContent value="operations" className="space-y-4">
          {/* 筛选器 */}
          <Card>
            <CardHeader>
              <CardTitle className="text-base">{t.admin.filterConditions}</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="grid gap-4 md:grid-cols-5">
                <div>
                  <Label htmlFor="action">{t.admin.operationType}</Label>
                  <Select
                    value={operationFilters.action || 'all'}
                    onValueChange={(value) =>
                      setOperationFilters({ ...operationFilters, action: value === 'all' ? '' : value })
                    }
                  >
                    <SelectTrigger id="action">
                      <SelectValue placeholder={t.admin.all} />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="all">{t.admin.all}</SelectItem>
                      <SelectItem value="create">{t.admin.actionCreate}</SelectItem>
                      <SelectItem value="update">{t.admin.actionUpdate}</SelectItem>
                      <SelectItem value="delete">{t.admin.actionDelete}</SelectItem>
                      <SelectItem value="login">{t.admin.actionLogin}</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div>
                  <Label htmlFor="resource_type">{t.admin.resourceType}</Label>
                  <Select
                    value={operationFilters.resource_type || 'all'}
                    onValueChange={(value) =>
                      setOperationFilters({ ...operationFilters, resource_type: value === 'all' ? '' : value })
                    }
                  >
                    <SelectTrigger id="resource_type">
                      <SelectValue placeholder={t.admin.all} />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="all">{t.admin.all}</SelectItem>
                      <SelectItem value="order">{t.admin.order}</SelectItem>
                      <SelectItem value="user">{t.admin.user}</SelectItem>
                      <SelectItem value="admin">{t.admin.admin}</SelectItem>
                      <SelectItem value="api_key">{t.admin.resourceApiKey}</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div>
                  <Label htmlFor="start_date">{t.admin.startDate}</Label>
                  <Input
                    id="start_date"
                    type="date"
                    value={operationFilters.start_date}
                    onChange={(e) =>
                      setOperationFilters({ ...operationFilters, start_date: e.target.value })
                    }
                  />
                </div>
                <div>
                  <Label htmlFor="end_date">{t.admin.endDate}</Label>
                  <Input
                    id="end_date"
                    type="date"
                    value={operationFilters.end_date}
                    onChange={(e) =>
                      setOperationFilters({ ...operationFilters, end_date: e.target.value })
                    }
                  />
                </div>
                <div className="flex items-end">
                  <Button
                    variant="outline"
                    onClick={() => {
                      setOperationFilters({
                        action: '',
                        resource_type: '',
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
                    onChange={(e) =>
                      setEmailFilters({ ...emailFilters, to_email: e.target.value })
                    }
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
                    onChange={(e) =>
                      setEmailFilters({ ...emailFilters, end_date: e.target.value })
                    }
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
              <div className="grid gap-4 md:grid-cols-5">
                <div>
                  <Label htmlFor="inv_type">{t.admin.operationType}</Label>
                  <Select
                    value={inventoryFilters.type || 'all'}
                    onValueChange={(value) =>
                      setInventoryFilters({ ...inventoryFilters, type: value === 'all' ? '' : value })
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
                cell: ({ row }: any) => <span className="text-xs text-gray-500">#{row.original.id}</span>,
              },
              {
                header: t.admin.inventoryIdLabel,
                accessorKey: 'inventory_id',
                cell: ({ row }: any) => <Badge variant="outline">{row.original.inventory_id}</Badge>,
              },
              {
                header: t.admin.type,
                accessorKey: 'type',
                cell: ({ row }: any) => {
                  const typeMap: Record<string, { label: string; color: 'default' | 'secondary' | 'destructive' }> = {
                    in: { label: t.admin.stockIn, color: 'default' },
                    out: { label: t.admin.stockOut, color: 'destructive' },
                    reserve: { label: t.admin.reserve, color: 'secondary' },
                    release: { label: t.admin.release, color: 'secondary' },
                    adjust: { label: t.admin.adjust, color: 'default' },
                  }
                  const config = typeMap[row.original.type] || { label: row.original.type, color: 'secondary' }
                  return <Badge variant={config.color}>{config.label}</Badge>
                },
              },
              {
                header: t.admin.quantity,
                accessorKey: 'quantity',
                cell: ({ row }: any) => (
                  <span className={row.original.quantity > 0 ? 'text-green-600' : 'text-red-600'}>
                    {row.original.quantity > 0 ? '+' : ''}{row.original.quantity}
                  </span>
                ),
              },
              {
                header: t.admin.beforeChange,
                accessorKey: 'before_stock',
              },
              {
                header: t.admin.afterChange,
                accessorKey: 'after_stock',
              },
              {
                header: t.admin.orderNoLabel,
                accessorKey: 'order_no',
                cell: ({ row }: any) => row.original.order_no || <span className="text-gray-400">-</span>,
              },
              {
                header: t.admin.operator,
                accessorKey: 'operator',
              },
              {
                header: t.admin.reason,
                accessorKey: 'reason',
                cell: ({ row }: any) => (
                  <span className="text-sm max-w-xs truncate block">{row.original.reason}</span>
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
      </Tabs>
    </div>
  )
}
