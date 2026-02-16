'use client'

import { use } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getUserDetail } from '@/lib/api'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ArrowLeft, User, Mail, Shield, Calendar } from 'lucide-react'
import Link from 'next/link'
import { formatDate } from '@/lib/utils'
import { getRoleColor } from '@/lib/role-utils'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'

export default function UserDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params)
  const userId = parseInt(id)
  const { locale } = useLocale()
  const t = getTranslations(locale)
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

  if (isLoading) {
    return <div className="text-center py-12">{t.common.loading}</div>
  }

  if (!data?.data) {
    return <div className="text-center py-12">{t.admin.noData}</div>
  }

  const user = data.data

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Button asChild variant="outline" size="sm">
          <Link href="/admin/users">
            <ArrowLeft className="mr-1.5 h-4 w-4" />
            <span className="hidden md:inline">{t.admin.backToList}</span>
          </Link>
        </Button>
        <h1 className="text-lg md:text-xl font-bold">{t.admin.userDetail}</h1>
      </div>

      <div className="grid gap-6 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>{t.admin.basicInfo}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex items-start gap-3">
              <User className="h-5 w-5 text-muted-foreground mt-0.5" />
              <div className="flex-1">
                <dt className="text-sm text-muted-foreground">{t.admin.userId}</dt>
                <dd className="font-medium">{user.id}</dd>
              </div>
            </div>

            <div className="flex items-start gap-3">
              <Mail className="h-5 w-5 text-muted-foreground mt-0.5" />
              <div className="flex-1">
                <dt className="text-sm text-muted-foreground">{t.admin.email}</dt>
                <dd className="font-medium">{user.email}</dd>
              </div>
            </div>

            <div className="flex items-start gap-3">
              <User className="h-5 w-5 text-muted-foreground mt-0.5" />
              <div className="flex-1">
                <dt className="text-sm text-muted-foreground">{t.admin.name}</dt>
                <dd className="font-medium">{user.name || t.admin.notSet}</dd>
              </div>
            </div>

            <div className="flex items-start gap-3">
              <Shield className="h-5 w-5 text-muted-foreground mt-0.5" />
              <div className="flex-1">
                <dt className="text-sm text-muted-foreground">{t.admin.role}</dt>
                <dd>
                  <Badge variant={getRoleColor(user.role)}>
                    {roleLabels[user.role] || user.role}
                  </Badge>
                </dd>
              </div>
            </div>

            <div className="flex items-start gap-3">
              <Calendar className="h-5 w-5 text-muted-foreground mt-0.5" />
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
            <div className="flex justify-between items-center">
              <span className="text-sm">{t.admin.accountStatus}</span>
              <Badge variant={user.isActive || user.is_active ? 'default' : 'secondary'}>
                {user.isActive || user.is_active ? t.admin.active : t.admin.inactive}
              </Badge>
            </div>
            <div className="flex justify-between items-center">
              <span className="text-sm">UUID</span>
              <code className="text-xs bg-gray-100 px-2 py-1 rounded">{user.uuid}</code>
            </div>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{t.admin.orderHistory}</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-muted-foreground text-center py-8">{t.admin.orderHistoryDeveloping}</p>
        </CardContent>
      </Card>
    </div>
  )
}
