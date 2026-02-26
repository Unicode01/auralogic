'use client'

import { useQuery } from '@tanstack/react-query'
import { getDashboardStatistics, getRecentActivities } from '@/lib/api'
import { StatsCard } from '@/components/admin/stats-card'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { OrderStatusBadge } from '@/components/orders/order-status-badge'
import { Package, Users, Truck, CheckCircle, TrendingUp, TrendingDown, UserCog, Key, Activity, DollarSign } from 'lucide-react'
import { formatDate, formatCurrency } from '@/lib/utils'
import Link from 'next/link'
import { useRouter } from 'next/navigation'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'
import { usePermission } from '@/hooks/use-permission'

export default function AdminDashboardPage() {
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.adminDashboard)
  const router = useRouter()
  const { isSuperAdmin } = usePermission()
  const isSuper = isSuperAdmin()

  const { data: stats, isLoading } = useQuery({
    queryKey: ['dashboardStats'],
    queryFn: getDashboardStatistics,
    enabled: isSuper,
  })

  const { data: activities } = useQuery({
    queryKey: ['recentActivities'],
    queryFn: getRecentActivities,
    enabled: isSuper,
  })

  if (!isSuper) {
    router.push('/admin/orders')
    return null
  }

  const statsData = stats?.data

  const actionLabels: Record<string, string> = {
    create: t.admin.actionCreate,
    update: t.admin.actionUpdate,
    delete: t.admin.actionDelete,
    login: t.admin.actionLogin,
  }

  const resourceLabels: Record<string, string> = {
    user: t.admin.resourceUser,
    admin: t.admin.resourceAdmin,
    api_key: t.admin.resourceApiKey,
    order: t.admin.resourceOrder,
    auth: t.admin.resourceAuth,
  }

  const statusLabels: Record<string, string> = {
    pending_payment: t.order.status.pending_payment,
    draft: t.order.status.draft,
    pending: t.order.status.pending,
    need_resubmit: t.order.status.need_resubmit,
    shipped: t.order.status.shipped,
    completed: t.order.status.completed,
    cancelled: t.order.status.cancelled,
    refunded: t.order.status.refunded,
  }

  return (
    <div className="space-y-8">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <h1 className="text-3xl font-bold">{t.admin.dashboard}</h1>
          <div className="flex items-center gap-1.5">
            <Badge variant="outline" className="text-xs font-mono">
              {t.admin.frontendVersion} {process.env.NEXT_PUBLIC_GIT_COMMIT || '-'}
            </Badge>
            {statsData?.git_commit && (
              <Badge variant="outline" className="text-xs font-mono">
                {t.admin.backendVersion} {statsData.git_commit}
              </Badge>
            )}
          </div>
        </div>
        <div className="text-sm text-muted-foreground">
          {t.admin.lastUpdated}{new Date().toLocaleString(locale === 'zh' ? 'zh-CN' : 'en-US')}
        </div>
      </div>

      {/* Stats cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <StatsCard
          title={t.admin.totalOrders}
          value={isLoading ? '-' : statsData?.orders?.total?.toString() || '0'}
          description={t.admin.monthlyNew.replace('{count}', String(statsData?.orders?.this_month || 0))}
          icon={Package}
          trend={
            statsData?.orders?.monthly_growth
              ? {
                  value: Math.abs(statsData.orders.monthly_growth),
                  isPositive: statsData.orders.monthly_growth >= 0,
                }
              : undefined
          }
        />
        <StatsCard
          title={t.admin.pendingShipment}
          value={isLoading ? '-' : statsData?.orders?.pending?.toString() || '0'}
          description={t.admin.needProcess}
          icon={Truck}
        />
        <StatsCard
          title={t.admin.completed}
          value={isLoading ? '-' : statsData?.orders?.completed?.toString() || '0'}
          description={`${t.admin.thisMonth} ${statsData?.orders?.this_month || 0}`}
          icon={CheckCircle}
        />
        <StatsCard
          title={t.admin.totalUsers}
          value={isLoading ? '-' : statsData?.users?.total?.toString() || '0'}
          description={t.admin.activeUsers.replace('{count}', String(statsData?.users?.active || 0))}
          icon={Users}
          trend={
            statsData?.users?.monthly_growth
              ? {
                  value: Math.abs(statsData.users.monthly_growth),
                  isPositive: statsData.users.monthly_growth >= 0,
                }
              : undefined
          }
        />
      </div>

      {/* Secondary stats cards */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <Card className="md:col-span-1 bg-gradient-to-br from-green-50 to-emerald-50 dark:from-green-950 dark:to-emerald-950 border-green-200 dark:border-green-800">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium text-green-800 dark:text-green-200">{t.admin.monthlySales}</CardTitle>
            <DollarSign className="h-4 w-4 text-green-600 dark:text-green-400" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-green-700 dark:text-green-300">
              {formatCurrency(statsData?.sales?.this_month || 0, statsData?.sales?.currency || 'CNY')}
            </div>
            <div className="flex items-center gap-2 mt-1">
              {statsData?.sales?.monthly_growth !== undefined && statsData.sales.monthly_growth !== 0 && (
                <span className={`text-xs flex items-center ${statsData.sales.monthly_growth >= 0 ? 'text-green-600' : 'text-red-600'}`}>
                  {statsData.sales.monthly_growth >= 0 ? <TrendingUp className="h-3 w-3 mr-0.5" /> : <TrendingDown className="h-3 w-3 mr-0.5" />}
                  {Math.abs(statsData.sales.monthly_growth).toFixed(1)}%
                </span>
              )}
              <p className="text-xs text-muted-foreground">
                {t.admin.today} {formatCurrency(statsData?.sales?.today || 0, statsData?.sales?.currency || 'CNY')}
              </p>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">{t.admin.administrators}</CardTitle>
            <UserCog className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{statsData?.admins?.total || 0}</div>
            <p className="text-xs text-muted-foreground">
              {t.admin.superAdmin} {statsData?.admins?.super_admins || 0} · {t.admin.normalAdmin} {statsData?.admins?.admins || 0}
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">{t.admin.apiKeysCount}</CardTitle>
            <Key className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{statsData?.api_keys?.total || 0}</div>
            <p className="text-xs text-muted-foreground">
              {t.admin.activeCount.replace('{count}', String(statsData?.api_keys?.active || 0))}
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">{t.admin.todayData}</CardTitle>
            <Activity className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{statsData?.orders?.today || 0}</div>
            <p className="text-xs text-muted-foreground">
              {t.admin.todayOrders} · {t.admin.newUsers} {statsData?.users?.today || 0}
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Order status distribution and recent orders */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        {/* Order status distribution */}
        <Card>
          <CardHeader>
            <CardTitle>{t.admin.orderStatusDist}</CardTitle>
            <CardDescription>{t.admin.orderStatusOverview}</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-3">
              {statsData?.order_status_distribution?.map((item: any) => {
                const statusColors: Record<string, string> = {
                  pending_payment: 'bg-orange-500',
                  draft: 'bg-slate-500',
                  pending: 'bg-blue-500',
                  need_resubmit: 'bg-red-500',
                  shipped: 'bg-purple-500',
                  completed: 'bg-green-500',
                  cancelled: 'bg-gray-500',
                  refunded: 'bg-red-400',
                }
                const total = statsData?.orders?.total || 1
                const percentage = ((item.count / total) * 100).toFixed(1)

                return (
                  <div key={item.status} className="flex items-center">
                    <div className="flex-1">
                      <div className="flex items-center justify-between mb-1">
                        <span className="text-sm font-medium">
                          {statusLabels[item.status] || item.status}
                        </span>
                        <span className="text-sm text-muted-foreground">
                          {item.count} ({percentage}%)
                        </span>
                      </div>
                      <div className="h-2 bg-secondary rounded-full overflow-hidden">
                        <div
                          className={`h-full ${statusColors[item.status] || 'bg-gray-500'}`}
                          style={{ width: `${percentage}%` }}
                        />
                      </div>
                    </div>
                  </div>
                )
              })}
              {!statsData?.order_status_distribution?.length && (
                <div className="text-center text-sm text-muted-foreground py-8">
                  {t.admin.noOrderData}
                </div>
              )}
            </div>
          </CardContent>
        </Card>

        {/* Recent orders */}
        <Card>
          <CardHeader>
            <CardTitle>{t.admin.recentOrders}</CardTitle>
            <CardDescription>{t.admin.latestOrders}</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-3">
              {statsData?.recent_orders?.slice(0, 10).map((order: any) => (
                <div
                  key={order.id}
                  className="flex items-center justify-between p-3 rounded-lg border hover:bg-accent transition-colors"
                >
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="font-medium text-sm truncate">
                        {order.order_no}
                      </span>
                      <OrderStatusBadge status={order.status} />
                    </div>
                    <div className="text-xs text-muted-foreground mt-1">
                      {order.receiver_name || order.receiver_email}
                    </div>
                  </div>
                  <div className="text-right ml-4">
                    <div className="text-sm font-medium">
                      {formatCurrency(order.total_amount || 0, order.currency || statsData?.sales?.currency || 'CNY')}
                    </div>
                    <div className="text-xs text-muted-foreground">
                      {order.created_at ? formatDate(order.created_at) : '-'}
                    </div>
                  </div>
                </div>
              ))}
              {!statsData?.recent_orders?.length && (
                <div className="text-center text-sm text-muted-foreground py-8">
                  {t.admin.noOrders}
                </div>
              )}
            </div>
            <div className="mt-4">
              <Link
                href="/admin/orders"
                className="text-sm text-primary hover:underline block text-center"
              >
                {t.admin.viewAllOrders}
              </Link>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Recent activity */}
      <Card>
        <CardHeader>
          <CardTitle>{t.admin.recentActivity}</CardTitle>
          <CardDescription>{t.admin.systemOperationLog}</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="space-y-3">
            {activities?.data?.slice(0, 10).map((activity: any) => {
              return (
                <div
                  key={activity.id}
                  className="flex items-start gap-3 p-3 rounded-lg border"
                >
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <Badge variant="secondary" className="text-xs">
                        {actionLabels[activity.action] || activity.action}
                      </Badge>
                      <span className="text-sm">
                        {resourceLabels[activity.resource_type] || activity.resource_type}
                      </span>
                      {activity.resource_id && (
                        <span className="text-xs text-muted-foreground">
                          #{activity.resource_id}
                        </span>
                      )}
                    </div>
                    <div className="flex items-center gap-2 mt-1 text-xs text-muted-foreground">
                      <span>
                        {activity.operator_name || activity.user?.name || activity.user?.email || t.admin.system}
                      </span>
                      <span>·</span>
                      <span>{activity.ip_address}</span>
                      <span>·</span>
                      <span>{activity.created_at ? formatDate(activity.created_at) : '-'}</span>
                    </div>
                  </div>
                </div>
              )
            })}
            {!activities?.data?.length && (
              <div className="text-center text-sm text-muted-foreground py-8">
                {t.admin.noActivityLog}
              </div>
            )}
          </div>
          <div className="mt-4">
            <Link
              href="/admin/logs"
              className="text-sm text-primary hover:underline block text-center"
            >
              {t.admin.viewAllLogs}
            </Link>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
