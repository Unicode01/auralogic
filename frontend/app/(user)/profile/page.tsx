'use client'

import { Suspense, useMemo, useState } from 'react'
import { useSearchParams } from 'next/navigation'
import { useAuth } from '@/hooks/use-auth'
import { useQuery } from '@tanstack/react-query'
import { getPublicConfig } from '@/lib/api'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import {
  Mail,
  Shield,
  Calendar,
  Copy,
  Settings,
  Package,
  LogOut,
  ChevronRight,
  ShieldCheck,
  MessageSquare,
  BookOpen,
  Megaphone,
  Bell,
  type LucideIcon,
} from 'lucide-react'
import Link from 'next/link'
import { copyToClipboard, formatDate } from '@/lib/utils'
import { useLocale } from '@/hooks/use-locale'
import { usePageTitle } from '@/hooks/use-page-title'
import { getTranslations } from '@/lib/i18n'
import { useIsMobile } from '@/hooks/use-mobile'
import { usePluginBootstrapQuery } from '@/lib/plugin-bootstrap-query'
import { extractBootstrapMenus } from '@/lib/plugin-bootstrap-cache'
import { readPluginSearchParams } from '@/lib/plugin-frontend-routing'
import { resolvePluginMenuIcon } from '@/lib/plugin-menu-icons'
import { parseUserPluginMenuItems } from '@/lib/plugin-user-menu'
import { clearToken } from '@/lib/auth'
import { PluginPageLink } from '@/components/plugins/plugin-page-link'
import { useToast } from '@/hooks/use-toast'
import { useCurrency, formatPrice } from '@/contexts/currency-context'
import { PluginSlot } from '@/components/plugins/plugin-slot'
import { PluginSlotBatchBoundary } from '@/lib/plugin-slot-batch'

type ProfilePluginQuickAction = {
  id: string
  title: string
  href: string
  icon: LucideIcon
}

function ProfilePageContent() {
  const { user } = useAuth()
  const searchParams = useSearchParams()
  const { locale } = useLocale()
  const { isMobile } = useIsMobile()
  const { currency } = useCurrency()
  const t = getTranslations(locale)
  const toast = useToast()
  const [logoutOpen, setLogoutOpen] = useState(false)
  usePageTitle(t.pageTitle.profile)
  const queryParams = useMemo(() => readPluginSearchParams(searchParams), [searchParams])

  const { data: publicConfigData } = useQuery({
    queryKey: ['publicConfig'],
    queryFn: getPublicConfig,
    staleTime: 5 * 60 * 1000,
  })
  const pluginBootstrapQuery = usePluginBootstrapQuery({
    scope: 'public',
    path: '/profile',
    queryParams,
  })
  const ticketEnabled = publicConfigData?.data?.ticket?.enabled ?? true
  const hasAdminAccess = user?.role === 'admin' || user?.role === 'super_admin'
  const pluginQuickActions = useMemo<ProfilePluginQuickAction[]>(
    () =>
      parseUserPluginMenuItems(extractBootstrapMenus(pluginBootstrapQuery.data), locale).map(
        (item) => ({
          id: item.id,
          title: item.title,
          href: item.href,
          icon: resolvePluginMenuIcon(item.iconName),
        })
      ),
    [locale, pluginBootstrapQuery.data]
  )

  const roleLabels: Record<string, string> = {
    user: t.profile.roleUser,
    admin: t.profile.roleAdmin,
    super_admin: t.profile.roleSuperAdmin,
  }
  const accountId = user?.id ?? null
  const registeredAt = user?.createdAt || user?.created_at
  const roleLabel = roleLabels[user?.role || ''] || user?.role || t.profile.unknown
  const userProfilePluginContext = useMemo(
    () => ({
      view: 'user_profile',
      user: {
        id: user?.id,
        role: user?.role,
        email: user?.email || undefined,
        name: user?.name || undefined,
        total_order_count: user?.total_order_count ?? 0,
        total_spent_minor: user?.total_spent_minor ?? 0,
        email_verified: user?.email_verified,
      },
      summary: {
        has_admin_access: hasAdminAccess,
        quick_action_count: pluginQuickActions.length,
        ticket_enabled: ticketEnabled,
        is_mobile: isMobile,
      },
      state: {
        has_email: Boolean(user?.email),
        has_registered_at: Boolean(registeredAt),
        has_total_orders: (user?.total_order_count ?? 0) > 0,
        has_total_spent: (user?.total_spent_minor ?? 0) > 0,
        has_plugin_quick_actions: pluginQuickActions.length > 0,
        can_open_tickets: ticketEnabled,
        has_admin_access: hasAdminAccess,
        email_verified: Boolean(user?.email_verified),
      },
    }),
    [hasAdminAccess, isMobile, pluginQuickActions.length, registeredAt, ticketEnabled, user]
  )
  const profileBatchItems = useMemo(
    () => [
      {
        slot: 'user.profile.top',
        hostContext: userProfilePluginContext,
      },
      {
        slot: 'user.profile.header.after',
        hostContext: { ...userProfilePluginContext, section: 'header' },
      },
      {
        slot: 'user.profile.identity.after',
        hostContext: { ...userProfilePluginContext, section: 'identity' },
      },
      {
        slot: 'user.profile.stats.after',
        hostContext: { ...userProfilePluginContext, section: 'stats' },
      },
      {
        slot: 'user.profile.overview.after',
        hostContext: { ...userProfilePluginContext, section: 'overview' },
      },
      {
        slot: 'user.profile.quick_actions.before',
        hostContext: { ...userProfilePluginContext, section: 'quick_actions' },
      },
      {
        slot: 'user.profile.quick_actions.after',
        hostContext: { ...userProfilePluginContext, section: 'quick_actions' },
      },
      {
        slot: 'user.profile.logout.before',
        hostContext: { ...userProfilePluginContext, section: 'logout' },
      },
      {
        slot: 'user.profile.logout.dialog.before',
        hostContext: { ...userProfilePluginContext, section: 'logout_dialog' },
      },
    ],
    [userProfilePluginContext]
  )

  const handleCopy = async (value?: string | number | null) => {
    if (!value) return
    const copied = await copyToClipboard(String(value))
    if (copied) {
      toast.success(t.common.copiedToClipboard)
    } else {
      toast.error(t.common.failed)
    }
  }

  const handleLogout = () => {
    if (typeof window !== 'undefined') {
      clearToken()
      window.location.href = '/login'
    }
  }

  return (
    <PluginSlotBatchBoundary scope="public" path="/profile" items={profileBatchItems}>
      <div className="space-y-6">
        <PluginSlot slot="user.profile.top" context={userProfilePluginContext} />
        <div className="flex items-center justify-between">
          <h1 className="text-2xl font-bold md:text-3xl">{t.profile.profileCenter}</h1>
          <Button
            asChild
            variant="outline"
            size={isMobile ? 'icon' : 'default'}
            aria-label={t.common.edit}
            title={t.common.edit}
          >
            <Link href="/profile/settings">
              <Settings className="h-4 w-4 md:mr-2" />
              <span className="sr-only md:not-sr-only">{t.common.edit}</span>
            </Link>
          </Button>
        </div>
        <PluginSlot
          slot="user.profile.header.after"
          context={{ ...userProfilePluginContext, section: 'header' }}
        />

        <Card className="overflow-hidden">
          <CardContent className="p-0">
            <div className="border-b bg-gradient-to-r from-muted/60 via-background to-background p-6">
              <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
                <div className="space-y-3">
                  <div className="flex flex-wrap items-center gap-2">
                    <div className="text-2xl font-semibold">{user?.name || t.profile.notSet}</div>
                    <span className="rounded-full border bg-background/80 px-2.5 py-1 text-xs text-muted-foreground">
                      {roleLabel}
                    </span>
                  </div>
                  <div className="flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
                    <span className="flex min-w-0 items-center gap-2 break-all">
                      <Mail className="h-4 w-4 shrink-0" />
                      <span className="break-all">{user?.email || t.profile.notSet}</span>
                    </span>
                    {user?.email ? (
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon"
                        className="h-8 w-8"
                        onClick={() => void handleCopy(user.email)}
                        aria-label={`${t.common.copy} ${t.profile.email}`}
                        title={`${t.common.copy} ${t.profile.email}`}
                      >
                        <Copy className="h-4 w-4" />
                        <span className="sr-only">
                          {t.common.copy} {t.profile.email}
                        </span>
                      </Button>
                    ) : null}
                  </div>
                  <PluginSlot
                    slot="user.profile.identity.after"
                    context={{ ...userProfilePluginContext, section: 'identity' }}
                  />
                </div>
                <div className="flex flex-wrap gap-2">
                  <Button asChild variant="outline" size="sm">
                    <Link href="/profile/preferences">
                      <Bell className="mr-2 h-4 w-4" />
                      {t.sidebar.preferences}
                    </Link>
                  </Button>
                  <Button asChild size="sm">
                    <Link href="/profile/settings">
                      <Settings className="mr-2 h-4 w-4" />
                      {t.profile.accountSettings}
                    </Link>
                  </Button>
                </div>
              </div>
            </div>
            <div className="grid gap-3 p-6 md:grid-cols-2 xl:grid-cols-4">
              <div className="rounded-xl border bg-background p-4">
                <div className="text-xs text-muted-foreground">{t.profile.accountId}</div>
                <div className="mt-1 flex items-center gap-2">
                  <span className="text-lg font-semibold">{accountId ?? '-'}</span>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className="h-8 w-8"
                    onClick={() => void handleCopy(accountId)}
                    aria-label={`${t.common.copy} ${t.profile.accountId}`}
                    title={`${t.common.copy} ${t.profile.accountId}`}
                    disabled={accountId === null}
                  >
                    <Copy className="h-4 w-4" />
                    <span className="sr-only">
                      {t.common.copy} {t.profile.accountId}
                    </span>
                  </Button>
                </div>
              </div>
              <div className="rounded-xl border bg-background p-4">
                <div className="text-xs text-muted-foreground">{t.profile.registerTime}</div>
                <div className="mt-1 flex items-center gap-2 text-lg font-semibold">
                  <Calendar className="h-4 w-4 text-muted-foreground" />
                  <span>{registeredAt ? formatDate(registeredAt) : t.profile.unknown}</span>
                </div>
              </div>
              <div className="rounded-xl border bg-background p-4">
                <div className="text-xs text-muted-foreground">{t.profile.totalOrders}</div>
                <div className="mt-1 text-lg font-semibold">{user?.total_order_count ?? 0}</div>
              </div>
              <div className="rounded-xl border bg-background p-4">
                <div className="text-xs text-muted-foreground">{t.profile.totalSpent}</div>
                <div className="mt-1 text-lg font-semibold">
                  {formatPrice(user?.total_spent_minor ?? 0, currency)}
                </div>
              </div>
            </div>
            <PluginSlot
              slot="user.profile.stats.after"
              context={{ ...userProfilePluginContext, section: 'stats' }}
            />
          </CardContent>
        </Card>
        <PluginSlot
          slot="user.profile.overview.after"
          context={{ ...userProfilePluginContext, section: 'overview' }}
        />

        <PluginSlot
          slot="user.profile.quick_actions.before"
          context={{ ...userProfilePluginContext, section: 'quick_actions' }}
        />
        <Card>
          <CardHeader>
            <CardTitle>{t.profile.quickActions}</CardTitle>
          </CardHeader>
          <CardContent className="p-0">
            <div className="divide-y">
              <Link
                href="/orders"
                className="flex items-center justify-between p-4 transition-colors hover:bg-accent"
              >
                <div className="flex items-center gap-3">
                  <Package className="h-5 w-5 text-muted-foreground" />
                  <span>{t.sidebar.myOrders}</span>
                </div>
                <ChevronRight className="h-5 w-5 text-muted-foreground" />
              </Link>

              <Link
                href="/serial-verify"
                className="flex items-center justify-between p-4 transition-colors hover:bg-accent"
              >
                <div className="flex items-center gap-3">
                  <ShieldCheck className="h-5 w-5 text-muted-foreground" />
                  <span>{t.sidebar.serialVerify}</span>
                </div>
                <ChevronRight className="h-5 w-5 text-muted-foreground" />
              </Link>

              {ticketEnabled && (
                <Link
                  href="/tickets"
                  className="flex items-center justify-between p-4 transition-colors hover:bg-accent"
                >
                  <div className="flex items-center gap-3">
                    <MessageSquare className="h-5 w-5 text-muted-foreground" />
                    <span>{t.sidebar.supportCenter}</span>
                  </div>
                  <ChevronRight className="h-5 w-5 text-muted-foreground" />
                </Link>
              )}

              <Link
                href="/knowledge"
                className="flex items-center justify-between p-4 transition-colors hover:bg-accent"
              >
                <div className="flex items-center gap-3">
                  <BookOpen className="h-5 w-5 text-muted-foreground" />
                  <span>{t.sidebar.knowledgeBase}</span>
                </div>
                <ChevronRight className="h-5 w-5 text-muted-foreground" />
              </Link>

              <Link
                href="/announcements"
                className="flex items-center justify-between p-4 transition-colors hover:bg-accent"
              >
                <div className="flex items-center gap-3">
                  <Megaphone className="h-5 w-5 text-muted-foreground" />
                  <span>{t.sidebar.announcements}</span>
                </div>
                <ChevronRight className="h-5 w-5 text-muted-foreground" />
              </Link>

              <Link
                href="/profile/settings"
                className="flex items-center justify-between p-4 transition-colors hover:bg-accent"
              >
                <div className="flex items-center gap-3">
                  <Settings className="h-5 w-5 text-muted-foreground" />
                  <span>{t.profile.accountSettings}</span>
                </div>
                <ChevronRight className="h-5 w-5 text-muted-foreground" />
              </Link>

              <Link
                href="/profile/preferences"
                className="flex items-center justify-between p-4 transition-colors hover:bg-accent"
              >
                <div className="flex items-center gap-3">
                  <Bell className="h-5 w-5 text-muted-foreground" />
                  <span>{t.sidebar.preferences}</span>
                </div>
                <ChevronRight className="h-5 w-5 text-muted-foreground" />
              </Link>

              {pluginQuickActions.map((item) => {
                const Icon = item.icon

                return (
                  <PluginPageLink
                    key={item.id}
                    href={item.href}
                    className="flex items-center justify-between p-4 transition-colors hover:bg-accent"
                  >
                    <div className="flex items-center gap-3">
                      <Icon className="h-5 w-5 text-muted-foreground" />
                      <span>{item.title}</span>
                    </div>
                    <ChevronRight className="h-5 w-5 text-muted-foreground" />
                  </PluginPageLink>
                )
              })}

              {(user?.role === 'admin' || user?.role === 'super_admin') && (
                <Link
                  href="/admin/dashboard"
                  className="flex items-center justify-between p-4 transition-colors hover:bg-accent"
                >
                  <div className="flex items-center gap-3">
                    <Shield className="h-5 w-5 text-muted-foreground" />
                    <span>{t.sidebar.adminPanel}</span>
                  </div>
                  <ChevronRight className="h-5 w-5 text-muted-foreground" />
                </Link>
              )}
            </div>
          </CardContent>
        </Card>
        <PluginSlot
          slot="user.profile.quick_actions.after"
          context={{ ...userProfilePluginContext, section: 'quick_actions' }}
        />

        <PluginSlot
          slot="user.profile.logout.before"
          context={{ ...userProfilePluginContext, section: 'logout' }}
        />
        <Card className="border-destructive/20">
          <CardContent className="p-0">
            <button
              onClick={() => setLogoutOpen(true)}
              className="flex w-full items-center justify-between p-4 text-left text-destructive transition-colors hover:bg-destructive/5"
            >
              <div className="flex items-center gap-3">
                <LogOut className="h-5 w-5" />
                <span>{t.auth.logout}</span>
              </div>
            </button>
          </CardContent>
        </Card>

        <AlertDialog open={logoutOpen} onOpenChange={setLogoutOpen}>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>{t.profile.logoutConfirmTitle}</AlertDialogTitle>
              <AlertDialogDescription>{t.profile.logoutConfirmDesc}</AlertDialogDescription>
            </AlertDialogHeader>
            <PluginSlot
              slot="user.profile.logout.dialog.before"
              context={{ ...userProfilePluginContext, section: 'logout_dialog' }}
            />
            <AlertDialogFooter>
              <AlertDialogCancel>{t.common.cancel}</AlertDialogCancel>
              <AlertDialogAction onClick={handleLogout}>{t.auth.logout}</AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
        <PluginSlot slot="user.profile.bottom" context={userProfilePluginContext} />
      </div>
    </PluginSlotBatchBoundary>
  )
}

export default function ProfilePage() {
  return (
    <Suspense fallback={<div className="min-h-[40vh]" />}>
      <ProfilePageContent />
    </Suspense>
  )
}
