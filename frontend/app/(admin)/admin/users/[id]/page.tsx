'use client'

import { use } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getPublicConfig, getUserDetail } from '@/lib/api'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { ArrowLeft, Calendar, Copy, Mail, Shield, User } from 'lucide-react'
import Link from 'next/link'
import { formatDate, formatPrice } from '@/lib/utils'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'
import { useToast } from '@/hooks/use-toast'
import { PluginSlot } from '@/components/plugins/plugin-slot'

export default function UserDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params)
  const userId = parseInt(id)
  const { locale } = useLocale()
  const t = getTranslations(locale)
  const toast = useToast()
  usePageTitle(t.pageTitle.adminUserDetail)

  const roleLabels: Record<string, string> = {
    user: t.admin.normalUser,
    admin: t.admin.admin,
    super_admin: t.admin.superAdminRole,
  }

  const { data, isLoading } = useQuery({
    queryKey: ['userDetail', userId],
    queryFn: () => getUserDetail(userId),
    enabled: !!userId,
  })
  const { data: publicConfigData } = useQuery({
    queryKey: ['publicConfig'],
    queryFn: getPublicConfig,
    staleTime: 5 * 60 * 1000,
  })

  if (isLoading) {
    return <div className="py-12 text-center">{t.common.loading}</div>
  }

  if (!data?.data) {
    return <div className="py-12 text-center">{t.admin.noData}</div>
  }

  const user = data.data
  const currency = publicConfigData?.data?.currency || 'CNY'
  const adminUserDetailPluginContext = {
    view: 'admin_user_detail',
    user: {
      id: user.id,
      email: user.email || undefined,
      phone: user.phone || undefined,
      role: user.role,
      uuid: user.uuid || undefined,
      is_active: Boolean(user.isActive || user.is_active),
      email_verified: Boolean(user.email_verified),
      total_order_count: Number(user.total_order_count || 0),
      total_spent_minor: Number(user.total_spent_minor || 0),
      country: user.country || undefined,
      locale: user.locale || undefined,
    },
    summary: {
      has_phone: Boolean(user.phone),
      has_email: Boolean(user.email),
    },
  }
  const copyToClipboard = async (value?: string | number | null) => {
    if (
      value === undefined ||
      value === null ||
      value === '' ||
      typeof navigator === 'undefined' ||
      !navigator.clipboard
    ) {
      return
    }
    await navigator.clipboard.writeText(String(value))
    toast.success(t.common.copiedToClipboard)
  }

  return (
    <div className="space-y-6">
      <PluginSlot slot="admin.user_detail.top" context={adminUserDetailPluginContext} />
      <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
        <div className="flex items-center gap-4">
          <Button asChild variant="outline" size="sm">
            <Link href="/admin/users">
              <ArrowLeft className="h-4 w-4 md:mr-1.5" />
              <span className="hidden md:inline">{t.admin.backToList}</span>
            </Link>
          </Button>
          <div>
            <h1 className="text-lg font-bold md:text-xl">{t.admin.userDetail}</h1>
            <p className="mt-1 flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
              <span>#{user.id}</span>
              {user.email ? <span className="break-all">{user.email}</span> : null}
              {user.phone ? <span>{user.phone}</span> : null}
              <span>{roleLabels[user.role] || user.role}</span>
              <span>{user.isActive || user.is_active ? t.admin.active : t.admin.inactive}</span>
              <span>{user.email_verified ? t.admin.verified : t.admin.unverified}</span>
            </p>
          </div>
        </div>
      </div>

      <div className="grid gap-6 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>{t.admin.basicInfo}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex items-start gap-3">
              <User className="mt-0.5 h-5 w-5 text-muted-foreground" />
              <div className="flex-1">
                <dt className="text-sm text-muted-foreground">{t.admin.userId}</dt>
                <dd className="flex items-center gap-2 font-medium">
                  <span>{user.id}</span>
                  <Button
                    type="button"
                    size="icon"
                    variant="ghost"
                    className="h-7 w-7"
                    onClick={() => void copyToClipboard(user.id)}
                  >
                    <Copy className="h-3.5 w-3.5" />
                  </Button>
                </dd>
              </div>
            </div>

            <div className="flex items-start gap-3">
              <Mail className="mt-0.5 h-5 w-5 text-muted-foreground" />
              <div className="flex-1">
                <dt className="text-sm text-muted-foreground">{t.admin.email}</dt>
                <dd className="flex items-center gap-2 font-medium">
                  <span className="break-all">{user.email || '-'}</span>
                  {user.email ? (
                    <Button
                      type="button"
                      size="icon"
                      variant="ghost"
                      className="h-7 w-7 shrink-0"
                      onClick={() => void copyToClipboard(user.email)}
                    >
                      <Copy className="h-3.5 w-3.5" />
                    </Button>
                  ) : null}
                </dd>
              </div>
            </div>

            <div className="flex items-start gap-3">
              <User className="mt-0.5 h-5 w-5 text-muted-foreground" />
              <div className="flex-1">
                <dt className="text-sm text-muted-foreground">{t.admin.name}</dt>
                <dd className="font-medium">{user.name || t.admin.notSet}</dd>
              </div>
            </div>

            {user.phone ? (
              <div className="flex items-start gap-3">
                <User className="mt-0.5 h-5 w-5 text-muted-foreground" />
                <div className="flex-1">
                  <dt className="text-sm text-muted-foreground">{t.ticket.phone}</dt>
                  <dd className="flex items-center gap-2 font-medium">
                    <span>{user.phone}</span>
                    <Button
                      type="button"
                      size="icon"
                      variant="ghost"
                      className="h-7 w-7"
                      onClick={() => void copyToClipboard(user.phone)}
                    >
                      <Copy className="h-3.5 w-3.5" />
                    </Button>
                  </dd>
                </div>
              </div>
            ) : null}

            <div className="flex items-start gap-3">
              <Shield className="mt-0.5 h-5 w-5 text-muted-foreground" />
              <div className="flex-1">
                <dt className="text-sm text-muted-foreground">{t.admin.role}</dt>
                <dd className="font-medium">{roleLabels[user.role] || user.role}</dd>
              </div>
            </div>

            <div className="flex items-start gap-3">
              <Calendar className="mt-0.5 h-5 w-5 text-muted-foreground" />
              <div className="flex-1">
                <dt className="text-sm text-muted-foreground">{t.admin.registrationTime}</dt>
                <dd className="font-medium">
                  {user.createdAt || user.created_at
                    ? formatDate(user.createdAt || user.created_at)
                    : '-'}
                </dd>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t.admin.accountStatus}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex items-center justify-between">
              <span className="text-sm">{t.admin.accountStatus}</span>
              <span className="text-sm text-muted-foreground">
                {user.isActive || user.is_active ? t.admin.active : t.admin.inactive}
              </span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-sm">UUID</span>
              <div className="flex items-center gap-2">
                <code className="rounded bg-gray-100 px-2 py-1 text-xs dark:bg-muted">
                  {user.uuid}
                </code>
                <Button
                  type="button"
                  size="icon"
                  variant="ghost"
                  className="h-7 w-7"
                  onClick={() => void copyToClipboard(user.uuid)}
                >
                  <Copy className="h-3.5 w-3.5" />
                </Button>
              </div>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-sm">{t.admin.totalSpent}</span>
              <span className="font-medium">
                {formatPrice(Number(user.total_spent_minor || 0), currency)}
              </span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-sm">{t.admin.orderCount}</span>
              <span className="font-medium">{Number(user.total_order_count || 0)}</span>
            </div>
            <div className="flex items-center justify-between gap-4">
              <span className="text-sm">{t.admin.locale}</span>
              <span className="font-medium">{user.locale || t.admin.notSet}</span>
            </div>
            <div className="flex items-center justify-between gap-4">
              <span className="text-sm">{t.admin.country}</span>
              <span className="font-medium">{user.country || t.admin.notSet}</span>
            </div>
            <div className="flex items-center justify-between gap-4">
              <span className="text-sm">{t.admin.lastLoginAt}</span>
              <span className="font-medium">
                {user.last_login_at ? formatDate(user.last_login_at) : t.admin.notSet}
              </span>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
