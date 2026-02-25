'use client'

import { useQuery } from '@tanstack/react-query'
import { getUserAnalytics, getOrderAnalytics, getRevenueAnalytics, getDeviceAnalytics, getSettings } from '@/lib/api'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Users, ShoppingCart, DollarSign, TrendingUp, TrendingDown, BarChart3, Smartphone, Monitor, AlertTriangle } from 'lucide-react'
import { formatCurrency } from '@/lib/utils'
import { useRouter } from 'next/navigation'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'
import { usePermission } from '@/hooks/use-permission'
import { useTheme } from '@/contexts/theme-context'
import {
  BarChart, Bar, LineChart, Line, PieChart, Pie, Cell,
  XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Legend,
} from 'recharts'

const COLORS = ['#3b82f6', '#10b981', '#f59e0b', '#ef4444', '#8b5cf6', '#ec4899', '#06b6d4', '#84cc16', '#f97316', '#6366f1']
const DEVICE_COLORS = ['#3b82f6', '#10b981', '#f59e0b']
const OS_COLORS = ['#3b82f6', '#10b981', '#f59e0b', '#8b5cf6', '#ef4444', '#6b7280']

function useChartTheme() {
  const { resolvedTheme } = useTheme()
  const isDark = resolvedTheme === 'dark'
  return {
    tickColor: isDark ? '#a1a1aa' : '#71717a',
    gridColor: isDark ? '#27272a' : '#e4e4e7',
    textColor: isDark ? '#e4e4e7' : '#18181b',
    tooltipBg: isDark ? '#1c1c22' : '#ffffff',
    tooltipBorder: isDark ? '#3f3f46' : '#e4e4e7',
    legendColor: isDark ? '#a1a1aa' : '#71717a',
    pieLabelColor: isDark ? '#d4d4d8' : '#3f3f46',
    mutedFill: isDark ? '#52525b' : '#d4d4d8',
    cursorFill: isDark ? 'rgba(255,255,255,0.06)' : 'rgba(0,0,0,0.06)',
  }
}

export default function AnalyticsPage() {
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.adminAnalytics)
  const router = useRouter()
  const { isSuperAdmin } = usePermission()
  const isSuper = isSuperAdmin()
  const chart = useChartTheme()

  const { data: settingsRes } = useQuery({
    queryKey: ['settings'],
    queryFn: getSettings,
    enabled: isSuper,
  })
  const analyticsEnabled = settingsRes?.data?.analytics?.enabled ?? false

  const { data: userData } = useQuery({
    queryKey: ['analyticsUsers'],
    queryFn: getUserAnalytics,
    enabled: isSuper && analyticsEnabled,
  })

  const { data: orderData } = useQuery({
    queryKey: ['analyticsOrders'],
    queryFn: getOrderAnalytics,
    enabled: isSuper && analyticsEnabled,
  })

  const { data: revenueData } = useQuery({
    queryKey: ['analyticsRevenue'],
    queryFn: getRevenueAnalytics,
    enabled: isSuper && analyticsEnabled,
  })

  const { data: deviceData } = useQuery({
    queryKey: ['analyticsDevices'],
    queryFn: getDeviceAnalytics,
    enabled: isSuper && analyticsEnabled,
  })

  if (!isSuper) {
    router.push('/admin/orders')
    return null
  }

  if (!analyticsEnabled) {
    return (
      <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold">{t.admin.analyticsTitle}</h1>
          <p className="text-muted-foreground mt-1">{t.admin.analyticsDesc}</p>
        </div>
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-16 text-center">
            <AlertTriangle className="h-12 w-12 text-muted-foreground mb-4" />
            <h2 className="text-xl font-semibold mb-2">{t.admin.analyticsDisabledTitle}</h2>
            <p className="text-muted-foreground max-w-md">{t.admin.analyticsDisabledDesc}</p>
          </CardContent>
        </Card>
      </div>
    )
  }

  const users = userData?.data
  const orders = orderData?.data
  const rev = revenueData?.data
  const devices = deviceData?.data

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

  const tooltipStyle = {
    contentStyle: { backgroundColor: chart.tooltipBg, border: `1px solid ${chart.tooltipBorder}`, borderRadius: '8px' },
    labelStyle: { color: chart.textColor },
    itemStyle: { color: chart.textColor },
    cursor: { fill: chart.cursorFill },
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">{t.admin.analyticsTitle}</h1>
        <p className="text-muted-foreground mt-1">{t.admin.analyticsDesc}</p>
      </div>

      <Tabs defaultValue="users">
        <TabsList>
          <TabsTrigger value="users" className="gap-1.5">
            <Users className="h-4 w-4" />
            {t.admin.userAnalytics}
          </TabsTrigger>
          <TabsTrigger value="orders" className="gap-1.5">
            <ShoppingCart className="h-4 w-4" />
            {t.admin.orderAnalytics}
          </TabsTrigger>
          <TabsTrigger value="revenue" className="gap-1.5">
            <DollarSign className="h-4 w-4" />
            {t.admin.revenueAnalytics}
          </TabsTrigger>
        </TabsList>

        {/* Users Tab */}
        <TabsContent value="users" className="space-y-6">
          {/* Overview cards */}
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
            <StatCard
              title={t.admin.totalUsersCount}
              value={users?.overview?.total || 0}
              icon={<Users className="h-4 w-4" />}
            />
            <StatCard
              title={t.admin.activeUsersCount}
              value={users?.overview?.active || 0}
              icon={<Users className="h-4 w-4" />}
              className="text-green-600 dark:text-green-400"
            />
            <StatCard
              title={t.admin.thisMonthNewUsers}
              value={users?.overview?.this_month || 0}
              growth={users?.overview?.monthly_growth}
              icon={<TrendingUp className="h-4 w-4" />}
            />
            <StatCard
              title={t.admin.lastMonthNewUsers}
              value={users?.overview?.last_month || 0}
              icon={<BarChart3 className="h-4 w-4" />}
            />
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
            {/* Registration Trend */}
            <Card>
              <CardHeader>
                <CardTitle>{t.admin.registrationTrend}</CardTitle>
                <CardDescription>{t.admin.last30Days}</CardDescription>
              </CardHeader>
              <CardContent>
                {users?.registration_trend?.length ? (
                  <ResponsiveContainer width="100%" height={300}>
                    <BarChart data={users.registration_trend}>
                      <CartesianGrid strokeDasharray="3 3" stroke={chart.gridColor} />
                      <XAxis dataKey="date" tick={{ fontSize: 12, fill: chart.tickColor }} stroke={chart.gridColor} />
                      <YAxis tick={{ fontSize: 12, fill: chart.tickColor }} stroke={chart.gridColor} />
                      <Tooltip {...tooltipStyle} />
                      <Bar dataKey="count" fill="#3b82f6" radius={[4, 4, 0, 0]} name={t.admin.count} />
                    </BarChart>
                  </ResponsiveContainer>
                ) : (
                  <EmptyState text={t.admin.noAnalyticsData} />
                )}
              </CardContent>
            </Card>

            {/* Order Engagement Pie */}
            <Card>
              <CardHeader>
                <CardTitle>{t.admin.orderEngagement}</CardTitle>
              </CardHeader>
              <CardContent>
                {users?.overview?.total ? (
                  <ResponsiveContainer width="100%" height={300}>
                    <PieChart>
                      <Pie
                        data={[
                          { name: t.admin.withOrders, value: users.order_engagement?.with_orders || 0 },
                          { name: t.admin.withoutOrders, value: users.order_engagement?.without_orders || 0 },
                        ]}
                        cx="50%"
                        cy="50%"
                        innerRadius={60}
                        outerRadius={100}
                        paddingAngle={5}
                        dataKey="value"
                        label={({ name, percent }: any) => `${name} ${((percent || 0) * 100).toFixed(0)}%`}
                        labelLine={{ stroke: chart.tickColor }}
                      >
                        <Cell fill="#3b82f6" />
                        <Cell fill={chart.mutedFill} />
                      </Pie>
                      <Tooltip {...tooltipStyle} />
                      <Legend wrapperStyle={{ color: chart.legendColor }} />
                    </PieChart>
                  </ResponsiveContainer>
                ) : (
                  <EmptyState text={t.admin.noAnalyticsData} />
                )}
              </CardContent>
            </Card>

            {/* Country Distribution */}
            <Card>
              <CardHeader>
                <CardTitle>{t.admin.countryDistribution}</CardTitle>
              </CardHeader>
              <CardContent>
                {users?.country_distribution?.length ? (
                  <ResponsiveContainer width="100%" height={300}>
                    <BarChart data={users.country_distribution.slice(0, 10)} layout="vertical">
                      <CartesianGrid strokeDasharray="3 3" stroke={chart.gridColor} />
                      <XAxis type="number" tick={{ fontSize: 12, fill: chart.tickColor }} stroke={chart.gridColor} />
                      <YAxis dataKey="country" type="category" width={80} tick={{ fontSize: 12, fill: chart.tickColor }} stroke={chart.gridColor} />
                      <Tooltip {...tooltipStyle} />
                      <Bar dataKey="count" fill="#10b981" radius={[0, 4, 4, 0]} name={t.admin.count} />
                    </BarChart>
                  </ResponsiveContainer>
                ) : (
                  <EmptyState text={t.admin.noAnalyticsData} />
                )}
              </CardContent>
            </Card>

            {/* Locale Distribution */}
            <Card>
              <CardHeader>
                <CardTitle>{t.admin.localeDistribution}</CardTitle>
              </CardHeader>
              <CardContent>
                {users?.locale_distribution?.length ? (
                  <ResponsiveContainer width="100%" height={300}>
                    <PieChart>
                      <Pie
                        data={users.locale_distribution}
                        cx="50%"
                        cy="50%"
                        outerRadius={100}
                        dataKey="count"
                        nameKey="locale"
                        label={({ locale, percent }: any) => `${locale || t.admin.unknown} ${((percent || 0) * 100).toFixed(0)}%`}
                        labelLine={{ stroke: chart.tickColor }}
                      >
                        {users.locale_distribution.map((_: any, i: number) => (
                          <Cell key={i} fill={COLORS[i % COLORS.length]} />
                        ))}
                      </Pie>
                      <Tooltip {...tooltipStyle} />
                      <Legend wrapperStyle={{ color: chart.legendColor }} />
                    </PieChart>
                  </ResponsiveContainer>
                ) : (
                  <EmptyState text={t.admin.noAnalyticsData} />
                )}
              </CardContent>
            </Card>
          </div>

          {/* Device Analytics */}
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
            {/* Device Type Distribution */}
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Monitor className="h-4 w-4" />
                  {t.admin.deviceDistribution}
                </CardTitle>
              </CardHeader>
              <CardContent>
                {devices?.device_distribution?.length ? (
                  <ResponsiveContainer width="100%" height={300}>
                    <PieChart>
                      <Pie
                        data={devices.device_distribution}
                        cx="50%"
                        cy="50%"
                        innerRadius={60}
                        outerRadius={100}
                        paddingAngle={5}
                        dataKey="count"
                        nameKey="name"
                        label={({ name, percent }: any) => `${name} ${((percent || 0) * 100).toFixed(0)}%`}
                        labelLine={{ stroke: chart.tickColor }}
                      >
                        {devices.device_distribution.map((_: any, i: number) => (
                          <Cell key={i} fill={DEVICE_COLORS[i % DEVICE_COLORS.length]} />
                        ))}
                      </Pie>
                      <Tooltip {...tooltipStyle} />
                      <Legend wrapperStyle={{ color: chart.legendColor }} />
                    </PieChart>
                  </ResponsiveContainer>
                ) : (
                  <EmptyState text={t.admin.noAnalyticsData} />
                )}
              </CardContent>
            </Card>

            {/* OS Distribution */}
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Smartphone className="h-4 w-4" />
                  {t.admin.osDistribution}
                </CardTitle>
              </CardHeader>
              <CardContent>
                {devices?.os_distribution?.length ? (
                  <ResponsiveContainer width="100%" height={300}>
                    <BarChart data={devices.os_distribution}>
                      <CartesianGrid strokeDasharray="3 3" stroke={chart.gridColor} />
                      <XAxis dataKey="name" tick={{ fontSize: 12, fill: chart.tickColor }} stroke={chart.gridColor} />
                      <YAxis tick={{ fontSize: 12, fill: chart.tickColor }} stroke={chart.gridColor} />
                      <Tooltip {...tooltipStyle} />
                      <Bar dataKey="count" radius={[4, 4, 0, 0]} name={t.admin.count}>
                        {devices.os_distribution.map((_: any, i: number) => (
                          <Cell key={i} fill={OS_COLORS[i % OS_COLORS.length]} />
                        ))}
                      </Bar>
                    </BarChart>
                  </ResponsiveContainer>
                ) : (
                  <EmptyState text={t.admin.noAnalyticsData} />
                )}
              </CardContent>
            </Card>
          </div>

          {/* Top Users Table */}
          <Card>
            <CardHeader>
              <CardTitle>{t.admin.topUsersByOrders}</CardTitle>
            </CardHeader>
            <CardContent>
              {users?.top_users?.length ? (
                <div className="overflow-x-auto">
                  <table className="w-full text-sm">
                    <thead>
                      <tr className="border-b">
                        <th className="text-left py-2 px-3 font-medium">#</th>
                        <th className="text-left py-2 px-3 font-medium">{t.auth.name}</th>
                        <th className="text-left py-2 px-3 font-medium">{t.auth.email}</th>
                        <th className="text-right py-2 px-3 font-medium">{t.admin.orderCount}</th>
                        <th className="text-right py-2 px-3 font-medium">{t.admin.totalSpent}</th>
                      </tr>
                    </thead>
                    <tbody>
                      {users.top_users.map((user: any, i: number) => (
                        <tr key={user.id} className="border-b last:border-0 hover:bg-accent/50">
                          <td className="py-2 px-3 text-muted-foreground">{i + 1}</td>
                          <td className="py-2 px-3 font-medium">{user.name || '-'}</td>
                          <td className="py-2 px-3 text-muted-foreground">{user.email}</td>
                          <td className="py-2 px-3 text-right">{user.order_count}</td>
                          <td className="py-2 px-3 text-right font-medium">
                            {formatCurrency(user.total_spent, rev?.overview?.currency || 'CNY')}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              ) : (
                <EmptyState text={t.admin.noAnalyticsData} />
              )}
            </CardContent>
          </Card>
        </TabsContent>

        {/* Orders Tab */}
        <TabsContent value="orders" className="space-y-6">
          {/* Overview cards */}
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
            <StatCard
              title={t.admin.totalOrdersCount}
              value={orders?.overview?.total || 0}
              icon={<ShoppingCart className="h-4 w-4" />}
            />
            <StatCard
              title={t.admin.thisMonthNewUsers}
              value={orders?.overview?.this_month || 0}
              growth={orders?.overview?.monthly_growth}
              icon={<TrendingUp className="h-4 w-4" />}
            />
            <StatCard
              title={t.admin.lastMonthNewUsers}
              value={orders?.overview?.last_month || 0}
              icon={<BarChart3 className="h-4 w-4" />}
            />
            <StatCard
              title={t.admin.avgOrderValue}
              value={formatCurrency(orders?.overview?.avg_order_value || 0, orders?.overview?.currency || 'CNY')}
              icon={<DollarSign className="h-4 w-4" />}
            />
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
            {/* Order Trend */}
            <Card>
              <CardHeader>
                <CardTitle>{t.admin.orderTrend}</CardTitle>
                <CardDescription>{t.admin.last30Days}</CardDescription>
              </CardHeader>
              <CardContent>
                {orders?.order_trend?.length ? (
                  <ResponsiveContainer width="100%" height={300}>
                    <LineChart data={orders.order_trend}>
                      <CartesianGrid strokeDasharray="3 3" stroke={chart.gridColor} />
                      <XAxis dataKey="date" tick={{ fontSize: 12, fill: chart.tickColor }} stroke={chart.gridColor} />
                      <YAxis tick={{ fontSize: 12, fill: chart.tickColor }} stroke={chart.gridColor} />
                      <Tooltip {...tooltipStyle} />
                      <Line type="monotone" dataKey="count" stroke="#3b82f6" strokeWidth={2} dot={false} name={t.admin.count} />
                    </LineChart>
                  </ResponsiveContainer>
                ) : (
                  <EmptyState text={t.admin.noAnalyticsData} />
                )}
              </CardContent>
            </Card>

            {/* Status Distribution */}
            <Card>
              <CardHeader>
                <CardTitle>{t.admin.statusDistribution}</CardTitle>
              </CardHeader>
              <CardContent>
                {orders?.status_distribution?.length ? (
                  <ResponsiveContainer width="100%" height={300}>
                    <PieChart>
                      <Pie
                        data={orders.status_distribution.map((item: any) => ({
                          ...item,
                          label: statusLabels[item.status] || item.status,
                        }))}
                        cx="50%"
                        cy="50%"
                        outerRadius={100}
                        dataKey="count"
                        nameKey="label"
                        label={({ label, percent }: any) => `${label} ${((percent || 0) * 100).toFixed(0)}%`}
                        labelLine={{ stroke: chart.tickColor }}
                      >
                        {orders.status_distribution.map((_: any, i: number) => (
                          <Cell key={i} fill={COLORS[i % COLORS.length]} />
                        ))}
                      </Pie>
                      <Tooltip {...tooltipStyle} />
                      <Legend wrapperStyle={{ color: chart.legendColor }} />
                    </PieChart>
                  </ResponsiveContainer>
                ) : (
                  <EmptyState text={t.admin.noAnalyticsData} />
                )}
              </CardContent>
            </Card>

            {/* Source Distribution */}
            <Card>
              <CardHeader>
                <CardTitle>{t.admin.sourceDistribution}</CardTitle>
              </CardHeader>
              <CardContent>
                {orders?.source_distribution?.length ? (
                  <ResponsiveContainer width="100%" height={300}>
                    <BarChart data={orders.source_distribution}>
                      <CartesianGrid strokeDasharray="3 3" stroke={chart.gridColor} />
                      <XAxis dataKey="source" tick={{ fontSize: 12, fill: chart.tickColor }} stroke={chart.gridColor} />
                      <YAxis tick={{ fontSize: 12, fill: chart.tickColor }} stroke={chart.gridColor} />
                      <Tooltip {...tooltipStyle} />
                      <Bar dataKey="count" fill="#f59e0b" radius={[4, 4, 0, 0]} name={t.admin.count} />
                    </BarChart>
                  </ResponsiveContainer>
                ) : (
                  <EmptyState text={t.admin.noAnalyticsData} />
                )}
              </CardContent>
            </Card>

            {/* Country Distribution */}
            <Card>
              <CardHeader>
                <CardTitle>{t.admin.orderCountryDistribution}</CardTitle>
              </CardHeader>
              <CardContent>
                {orders?.country_distribution?.length ? (
                  <ResponsiveContainer width="100%" height={300}>
                    <BarChart data={orders.country_distribution.slice(0, 10)} layout="vertical">
                      <CartesianGrid strokeDasharray="3 3" stroke={chart.gridColor} />
                      <XAxis type="number" tick={{ fontSize: 12, fill: chart.tickColor }} stroke={chart.gridColor} />
                      <YAxis dataKey="country" type="category" width={80} tick={{ fontSize: 12, fill: chart.tickColor }} stroke={chart.gridColor} />
                      <Tooltip {...tooltipStyle} />
                      <Bar dataKey="count" fill="#8b5cf6" radius={[0, 4, 4, 0]} name={t.admin.count} />
                    </BarChart>
                  </ResponsiveContainer>
                ) : (
                  <EmptyState text={t.admin.noAnalyticsData} />
                )}
              </CardContent>
            </Card>

            {/* Amount Distribution */}
            <Card>
              <CardHeader>
                <CardTitle>{t.admin.amountDistribution}</CardTitle>
              </CardHeader>
              <CardContent>
                {orders?.amount_distribution?.length ? (
                  <ResponsiveContainer width="100%" height={300}>
                    <BarChart data={orders.amount_distribution}>
                      <CartesianGrid strokeDasharray="3 3" stroke={chart.gridColor} />
                      <XAxis dataKey="range" tick={{ fontSize: 12, fill: chart.tickColor }} stroke={chart.gridColor} />
                      <YAxis tick={{ fontSize: 12, fill: chart.tickColor }} stroke={chart.gridColor} />
                      <Tooltip {...tooltipStyle} />
                      <Bar dataKey="count" fill="#ec4899" radius={[4, 4, 0, 0]} name={t.admin.count} />
                    </BarChart>
                  </ResponsiveContainer>
                ) : (
                  <EmptyState text={t.admin.noAnalyticsData} />
                )}
              </CardContent>
            </Card>

            {/* Top Products */}
            <Card>
              <CardHeader>
                <CardTitle>{t.admin.topProducts}</CardTitle>
              </CardHeader>
              <CardContent>
                {orders?.top_products?.length ? (
                  <ResponsiveContainer width="100%" height={300}>
                    <BarChart data={orders.top_products} layout="vertical">
                      <CartesianGrid strokeDasharray="3 3" stroke={chart.gridColor} />
                      <XAxis type="number" tick={{ fontSize: 12, fill: chart.tickColor }} stroke={chart.gridColor} />
                      <YAxis dataKey="name" type="category" width={120} tick={{ fontSize: 11, fill: chart.tickColor }} stroke={chart.gridColor} />
                      <Tooltip {...tooltipStyle} />
                      <Bar dataKey="sale_count" fill="#06b6d4" radius={[0, 4, 4, 0]} name={t.admin.salesCount} />
                    </BarChart>
                  </ResponsiveContainer>
                ) : (
                  <EmptyState text={t.admin.noAnalyticsData} />
                )}
              </CardContent>
            </Card>
          </div>
        </TabsContent>

        {/* Revenue Tab */}
        <TabsContent value="revenue" className="space-y-6">
          {/* Overview cards */}
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
            <StatCard
              title={t.admin.totalRevenue}
              value={formatCurrency(rev?.overview?.total_revenue || 0, rev?.overview?.currency || 'CNY')}
              icon={<DollarSign className="h-4 w-4" />}
            />
            <StatCard
              title={t.admin.thisMonthRevenue}
              value={formatCurrency(rev?.overview?.this_month || 0, rev?.overview?.currency || 'CNY')}
              growth={rev?.overview?.monthly_growth}
              icon={<TrendingUp className="h-4 w-4" />}
            />
            <StatCard
              title={t.admin.todayRevenue}
              value={formatCurrency(rev?.overview?.today_revenue || 0, rev?.overview?.currency || 'CNY')}
              icon={<DollarSign className="h-4 w-4" />}
            />
            <StatCard
              title={t.admin.avgOrderValue}
              value={formatCurrency(rev?.overview?.avg_order_value || 0, rev?.overview?.currency || 'CNY')}
              icon={<BarChart3 className="h-4 w-4" />}
            />
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
            {/* Daily Revenue Trend */}
            <Card className="lg:col-span-2">
              <CardHeader>
                <CardTitle>{t.admin.dailyRevenueTrend}</CardTitle>
                <CardDescription>{t.admin.last30Days}</CardDescription>
              </CardHeader>
              <CardContent>
                {rev?.daily_trend?.length ? (
                  <ResponsiveContainer width="100%" height={350}>
                    <LineChart data={rev.daily_trend}>
                      <CartesianGrid strokeDasharray="3 3" stroke={chart.gridColor} />
                      <XAxis dataKey="date" tick={{ fontSize: 12, fill: chart.tickColor }} stroke={chart.gridColor} />
                      <YAxis tick={{ fontSize: 12, fill: chart.tickColor }} stroke={chart.gridColor} />
                      <Tooltip
                        {...tooltipStyle}
                        formatter={(value: any) => [formatCurrency(value || 0, rev.overview.currency), t.admin.revenue]}
                      />
                      <Line type="monotone" dataKey="revenue" stroke="#10b981" strokeWidth={2} dot={false} name={t.admin.revenue} />
                    </LineChart>
                  </ResponsiveContainer>
                ) : (
                  <EmptyState text={t.admin.noAnalyticsData} />
                )}
              </CardContent>
            </Card>

            {/* Monthly Revenue Trend */}
            <Card className="lg:col-span-2">
              <CardHeader>
                <CardTitle>{t.admin.monthlyRevenueTrend}</CardTitle>
              </CardHeader>
              <CardContent>
                {rev?.monthly_trend?.length ? (
                  <ResponsiveContainer width="100%" height={350}>
                    <BarChart data={rev.monthly_trend}>
                      <CartesianGrid strokeDasharray="3 3" stroke={chart.gridColor} />
                      <XAxis dataKey="month" tick={{ fontSize: 12, fill: chart.tickColor }} stroke={chart.gridColor} />
                      <YAxis tick={{ fontSize: 12, fill: chart.tickColor }} stroke={chart.gridColor} />
                      <Tooltip
                        {...tooltipStyle}
                        formatter={(value: any) => [formatCurrency(value || 0, rev.overview.currency), t.admin.revenue]}
                      />
                      <Bar dataKey="revenue" fill="#3b82f6" radius={[4, 4, 0, 0]} name={t.admin.revenue} />
                    </BarChart>
                  </ResponsiveContainer>
                ) : (
                  <EmptyState text={t.admin.noAnalyticsData} />
                )}
              </CardContent>
            </Card>

            {/* Revenue by Source */}
            <Card>
              <CardHeader>
                <CardTitle>{t.admin.revenueBySource}</CardTitle>
              </CardHeader>
              <CardContent>
                {rev?.revenue_by_source?.length ? (
                  <ResponsiveContainer width="100%" height={300}>
                    <PieChart>
                      <Pie
                        data={rev.revenue_by_source}
                        cx="50%"
                        cy="50%"
                        outerRadius={100}
                        dataKey="revenue"
                        nameKey="source"
                        label={({ source, percent }: any) => `${source} ${((percent || 0) * 100).toFixed(0)}%`}
                        labelLine={{ stroke: chart.tickColor }}
                      >
                        {rev.revenue_by_source.map((_: any, i: number) => (
                          <Cell key={i} fill={COLORS[i % COLORS.length]} />
                        ))}
                      </Pie>
                      <Tooltip
                        {...tooltipStyle}
                        formatter={(value: any) => [formatCurrency(value || 0, rev.overview.currency), t.admin.revenue]}
                      />
                      <Legend wrapperStyle={{ color: chart.legendColor }} />
                    </PieChart>
                  </ResponsiveContainer>
                ) : (
                  <EmptyState text={t.admin.noAnalyticsData} />
                )}
              </CardContent>
            </Card>

            {/* Revenue by Country */}
            <Card>
              <CardHeader>
                <CardTitle>{t.admin.revenueByCountry}</CardTitle>
              </CardHeader>
              <CardContent>
                {rev?.revenue_by_country?.length ? (
                  <ResponsiveContainer width="100%" height={300}>
                    <BarChart data={rev.revenue_by_country.slice(0, 10)} layout="vertical">
                      <CartesianGrid strokeDasharray="3 3" stroke={chart.gridColor} />
                      <XAxis type="number" tick={{ fontSize: 12, fill: chart.tickColor }} stroke={chart.gridColor} />
                      <YAxis dataKey="country" type="category" width={80} tick={{ fontSize: 12, fill: chart.tickColor }} stroke={chart.gridColor} />
                      <Tooltip
                        {...tooltipStyle}
                        formatter={(value: any) => [formatCurrency(value || 0, rev.overview.currency), t.admin.revenue]}
                      />
                      <Bar dataKey="revenue" fill="#f59e0b" radius={[0, 4, 4, 0]} name={t.admin.revenue} />
                    </BarChart>
                  </ResponsiveContainer>
                ) : (
                  <EmptyState text={t.admin.noAnalyticsData} />
                )}
              </CardContent>
            </Card>
          </div>
        </TabsContent>
      </Tabs>
    </div>
  )
}

function StatCard({
  title,
  value,
  icon,
  growth,
  className,
}: {
  title: string
  value: string | number
  icon: React.ReactNode
  growth?: number
  className?: string
}) {
  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">{title}</CardTitle>
        <span className="text-muted-foreground">{icon}</span>
      </CardHeader>
      <CardContent>
        <div className={`text-2xl font-bold ${className || ''}`}>{value}</div>
        {growth !== undefined && growth !== 0 && (
          <div className={`flex items-center gap-1 text-xs mt-1 ${growth >= 0 ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'}`}>
            {growth >= 0 ? <TrendingUp className="h-3 w-3" /> : <TrendingDown className="h-3 w-3" />}
            {Math.abs(growth).toFixed(1)}%
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function EmptyState({ text }: { text: string }) {
  return (
    <div className="flex items-center justify-center h-[200px] text-sm text-muted-foreground">
      {text}
    </div>
  )
}
