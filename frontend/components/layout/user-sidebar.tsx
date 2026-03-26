'use client'

import { Suspense, useEffect, useMemo, useRef, useState } from 'react'
import Link from 'next/link'
import { usePathname, useSearchParams } from 'next/navigation'
import { useQuery } from '@tanstack/react-query'
import { cn } from '@/lib/utils'
import {
  ShoppingBag,
  ShoppingCart,
  Package,
  User,
  Settings,
  Bell,
  LogOut,
  LogIn,
  UserPlus,
  Shield,
  ShieldCheck,
  MessageSquare,
  BookOpen,
  Megaphone,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useAuth } from '@/hooks/use-auth'
import { useAuthEntry } from '@/hooks/use-auth-entry'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { resolvePluginMenuIcon } from '@/lib/plugin-menu-icons'
import { usePluginBootstrapQuery } from '@/lib/plugin-bootstrap-query'
import {
  buildPluginBootstrapContextKey,
  isPluginMenuPathActive,
  readPluginSearchParams,
} from '@/lib/plugin-frontend-routing'
import { parseUserPluginMenuItems } from '@/lib/plugin-user-menu'
import {
  getPublicConfig,
  type PluginFrontendBootstrapMenuItem,
} from '@/lib/api'
import {
  extractBootstrapMenus,
  getCachedUserBootstrapMenusResult,
  setCachedUserBootstrapMenus,
} from '@/lib/plugin-bootstrap-cache'
import { PluginPageLink } from '@/components/plugins/plugin-page-link'
import { LanguageSwitcher } from './language-switcher'
import { clearToken } from '@/lib/auth'
import { PluginSlot } from '@/components/plugins/plugin-slot'
import { resolvePluginPlatformEnabled } from '@/lib/plugin-slot-behavior'

const getUserMenuItems = (t: any) => [
  { title: t.sidebar.productCenter, href: '/products', icon: ShoppingBag, matchDescendants: true },
  { title: t.sidebar.cart || 'Cart', href: '/cart', icon: ShoppingCart },
  { title: t.sidebar.myOrders, href: '/orders', icon: Package, matchDescendants: true },
  { title: t.sidebar.serialVerify, href: '/serial-verify', icon: ShieldCheck, matchDescendants: true },
  { title: t.sidebar.supportCenter || 'Support', href: '/tickets', icon: MessageSquare, matchDescendants: true },
  { title: t.sidebar.knowledgeBase || 'Knowledge', href: '/knowledge', icon: BookOpen, matchDescendants: true },
  { title: t.sidebar.announcements || 'Announcements', href: '/announcements', icon: Megaphone, matchDescendants: true },
  { title: t.sidebar.profile, href: '/profile', icon: User },
  { title: t.sidebar.accountSettings, href: '/profile/settings', icon: Settings },
]

const getGuestMenuItems = (t: any) => [
  { title: t.sidebar.productCenter, href: '/products', icon: ShoppingBag, matchDescendants: true },
  { title: t.sidebar.cart || 'Cart', href: '/cart', icon: ShoppingCart },
  { title: t.sidebar.serialVerify, href: '/serial-verify', icon: ShieldCheck, matchDescendants: true },
  { title: t.sidebar.preferences, href: '/profile/preferences', icon: Bell, matchDescendants: true },
]

interface UserSidebarProps {
  className?: string
  guestMode?: boolean
}

type UserSidebarMenuItem = {
  title: string
  href: string
  icon: any
  matchDescendants?: boolean
  pluginRuntime?: boolean
}

export function UserSidebar({ className, guestMode = false }: UserSidebarProps) {
  const pathname = usePathname()
  const searchParams = useSearchParams()
  const { user } = useAuth()
  const { goToAuth } = useAuthEntry()
  const { locale, mounted } = useLocale()
  const t = getTranslations(locale)
  const [displayedBootstrapMenus, setDisplayedBootstrapMenus] = useState<
    PluginFrontendBootstrapMenuItem[]
  >([])
  const queryParams = useMemo(() => readPluginSearchParams(searchParams), [searchParams])
  const userBootstrapScopeKey = guestMode
    ? 'guest'
    : user?.id
      ? `user:${user.id}`
      : 'user-anonymous'
  const previousBootstrapScopeKeyRef = useRef(userBootstrapScopeKey)
  const bootstrapContextKey = useMemo(
    () => buildPluginBootstrapContextKey(pathname || '/', queryParams),
    [pathname, queryParams]
  )
  const localizedBootstrapContextKey = useMemo(
    () => `${bootstrapContextKey}|locale=${locale}`,
    [bootstrapContextKey, locale]
  )

  useEffect(() => {
    if (!mounted) return
    const cachedBootstrapResult = getCachedUserBootstrapMenusResult(
      userBootstrapScopeKey,
      localizedBootstrapContextKey
    )
    const scopeChanged = previousBootstrapScopeKeyRef.current !== userBootstrapScopeKey
    previousBootstrapScopeKeyRef.current = userBootstrapScopeKey

    if (cachedBootstrapResult.found) {
      setDisplayedBootstrapMenus(cachedBootstrapResult.menus)
      return
    }

    if (scopeChanged) {
      setDisplayedBootstrapMenus([])
    }
  }, [localizedBootstrapContextKey, mounted, userBootstrapScopeKey])

  const {
    data: publicConfigData,
    isError: publicConfigLoadFailed,
  } = useQuery({
    queryKey: ['publicConfig'],
    queryFn: getPublicConfig,
    staleTime: 5 * 60 * 1000,
  })
  const pluginBootstrapQuery = usePluginBootstrapQuery({
    scope: 'public',
    path: pathname || '/',
    queryParams,
  })

  const networkBootstrapMenus = useMemo(
    () => extractBootstrapMenus(pluginBootstrapQuery.data),
    [pluginBootstrapQuery.data]
  )

  useEffect(() => {
    if (!mounted) return
    if (pluginBootstrapQuery.data === undefined) return
    setDisplayedBootstrapMenus(networkBootstrapMenus)
    setCachedUserBootstrapMenus(
      networkBootstrapMenus,
      userBootstrapScopeKey,
      localizedBootstrapContextKey
    )
  }, [
    localizedBootstrapContextKey,
    mounted,
    networkBootstrapMenus,
    pluginBootstrapQuery.data,
    userBootstrapScopeKey,
  ])

  const ticketEnabled = publicConfigData?.data?.ticket?.enabled ?? true
  const serialEnabled = publicConfigData?.data?.serial?.enabled ?? true
  const pluginPlatformEnabled = resolvePluginPlatformEnabled(publicConfigData?.data, true)
  const baseMenuItems = useMemo(
    () =>
      (guestMode ? getGuestMenuItems(t) : getUserMenuItems(t)).map((item) => ({
        ...item,
        matchDescendants: item.matchDescendants ?? guestMode,
        pluginRuntime: false,
      })),
    [guestMode, t]
  )
  const runtimeMenuItems = useMemo(
    () =>
      parseUserPluginMenuItems(displayedBootstrapMenus, locale)
        .filter((item) => !guestMode || item.guestVisible)
        .map((item) => ({
          title: item.title,
          href: item.href,
          icon: resolvePluginMenuIcon(item.iconName),
          matchDescendants: true,
          pluginRuntime: true,
        })),
    [displayedBootstrapMenus, guestMode, locale]
  )
  const visibleBaseMenuItems = useMemo(
    () =>
      baseMenuItems.filter((item) => {
        if (item.href === '/serial-verify' && !serialEnabled) return false
        if (!guestMode && item.href === '/tickets' && !ticketEnabled) return false
        return true
      }),
    [baseMenuItems, guestMode, serialEnabled, ticketEnabled]
  )
  const visibleRuntimeMenuItems = useMemo(
    () =>
      (pluginPlatformEnabled ? runtimeMenuItems : []).filter((item) => {
        if (item.href === '/serial-verify' && !serialEnabled) return false
        if (!guestMode && item.href === '/tickets' && !ticketEnabled) return false
        return true
      }),
    [guestMode, pluginPlatformEnabled, runtimeMenuItems, serialEnabled, ticketEnabled]
  )
  const menuItems = useMemo(
    () => [...visibleBaseMenuItems, ...visibleRuntimeMenuItems],
    [visibleBaseMenuItems, visibleRuntimeMenuItems]
  )
  const isAdmin = user?.role === 'admin' || user?.role === 'super_admin'
  const sidebarMenuContext = useMemo(
    () => ({
      builtin_items: visibleBaseMenuItems.map((item) => ({
        title: item.title,
        href: item.href,
        plugin_runtime: false,
      })),
      runtime_items: visibleRuntimeMenuItems.map((item) => ({
        title: item.title,
        href: item.href,
        plugin_runtime: true,
      })),
      all_items: menuItems.map((item) => ({
        title: item.title,
        href: item.href,
        plugin_runtime: !!item.pluginRuntime,
      })),
    }),
    [menuItems, visibleBaseMenuItems, visibleRuntimeMenuItems]
  )
  const authActionsContext = useMemo(
    () =>
      guestMode
        ? {
            guest_actions: [
              { key: 'login', href: '/login', action: 'login' },
              { key: 'register', href: '/register', action: 'register' },
            ],
            authenticated_actions: [],
          }
        : {
            guest_actions: [],
            authenticated_actions: [
              ...(isAdmin
                ? [{ key: 'admin_panel', href: '/admin/dashboard', action: 'navigate_admin' }]
                : []),
              { key: 'logout', href: '/login', action: 'logout' },
            ],
          },
    [guestMode, isAdmin]
  )
  const userSidebarPluginContext = useMemo(
    () => ({
      view: 'user_sidebar',
      layout: {
        current_path: pathname || '/',
        locale,
        guest_mode: guestMode,
      },
      user: guestMode
        ? null
        : {
            id: user?.id,
            role: user?.role,
            is_admin: isAdmin,
          },
      menu: sidebarMenuContext,
      auth_actions: authActionsContext,
      summary: {
        visible_builtin_count: visibleBaseMenuItems.length,
        visible_runtime_count: visibleRuntimeMenuItems.length,
        visible_item_count: menuItems.length,
        serial_enabled: serialEnabled,
        ticket_enabled: ticketEnabled,
      },
      state: {
        guest_mode: guestMode,
        has_runtime_items: visibleRuntimeMenuItems.length > 0,
        has_admin_entry: !guestMode && isAdmin,
        bootstrap_loading: pluginBootstrapQuery.isLoading,
        bootstrap_error: pluginBootstrapQuery.isError,
        public_config_load_failed: publicConfigLoadFailed,
      },
    }),
    [
      authActionsContext,
      guestMode,
      isAdmin,
      locale,
      menuItems.length,
      pathname,
      pluginBootstrapQuery.isError,
      pluginBootstrapQuery.isLoading,
      publicConfigLoadFailed,
      serialEnabled,
      sidebarMenuContext,
      ticketEnabled,
      user?.id,
      user?.role,
      visibleBaseMenuItems.length,
      visibleRuntimeMenuItems.length,
    ]
  )

  if (!mounted) {
    const defaultT = getTranslations('zh')
    const defaultMenuItems = (guestMode ? getGuestMenuItems(defaultT) : getUserMenuItems(defaultT))
      .map((item) => ({
        ...item,
        matchDescendants: item.matchDescendants ?? guestMode,
        pluginRuntime: false,
      }))
      .filter((item) => {
        if (item.href === '/serial-verify' && !serialEnabled) return false
        if (!guestMode && item.href === '/tickets' && !ticketEnabled) return false
        return true
      })

    return (
      <div className={cn('hidden w-64 flex-col border-r bg-card md:flex', className)}>
        <div className="p-6">
          <h2 className="text-lg font-bold">AuraLogic</h2>
          <p className="text-sm text-muted-foreground">
            {guestMode
              ? `${defaultT.auth.login} / ${defaultT.auth.register}`
              : defaultT.sidebar.welcome}
          </p>
        </div>

        <nav className="flex-1 space-y-1 overflow-y-auto px-3">
          {defaultMenuItems.map((item) => {
            const Icon = item.icon
            const isActive = item.matchDescendants
              ? isPluginMenuPathActive(pathname || '/', item.href)
              : pathname === item.href

            return (
              <Link
                key={item.href}
                href={item.href}
                className={cn(
                  'flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-all',
                  isActive
                    ? 'bg-primary text-primary-foreground shadow-sm'
                    : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground'
                )}
              >
                <Icon className="h-4 w-4" />
                {item.title}
              </Link>
            )
          })}
        </nav>

        <div className="space-y-2 border-t p-3">
          <LanguageSwitcher />
          {guestMode ? (
            <>
              <Button
                variant="outline"
                className="w-full justify-start"
                size="sm"
                onClick={() => goToAuth('/login')}
              >
                <LogIn className="mr-2 h-4 w-4" />
                {defaultT.auth.login}
              </Button>
              <Button
                variant="outline"
                className="w-full justify-start"
                size="sm"
                onClick={() => goToAuth('/register')}
              >
                <UserPlus className="mr-2 h-4 w-4" />
                {defaultT.auth.register}
              </Button>
            </>
          ) : (
            <Button
              variant="outline"
              className="w-full justify-start"
              size="sm"
              onClick={() => {
                if (typeof window !== 'undefined') {
                  clearToken()
                  window.location.href = '/login'
                }
              }}
            >
              <LogOut className="mr-2 h-4 w-4" />
              {defaultT.auth.logout}
            </Button>
          )}
        </div>
      </div>
    )
  }

  return (
    <div className={cn('hidden w-64 flex-col border-r bg-card md:flex', className)}>
      <div className="p-6">
        <h2 className="text-lg font-bold">AuraLogic</h2>
        <p className="text-sm text-muted-foreground">
          {guestMode ? `${t.auth.login} / ${t.auth.register}` : t.sidebar.welcome}
        </p>
        <Suspense fallback={null}>
          <PluginSlot slot="user.layout.sidebar.top" context={userSidebarPluginContext} />
        </Suspense>
      </div>

      <nav className="flex-1 space-y-1 overflow-y-auto px-3">
        {visibleBaseMenuItems.map((item) => {
          const Icon = item.icon
          const isActive = item.matchDescendants
            ? isPluginMenuPathActive(pathname || '/', item.href)
            : pathname === item.href

          const linkClassName = cn(
            'flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-all',
            isActive
              ? 'bg-primary text-primary-foreground shadow-sm'
              : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground'
          )

          return item.pluginRuntime ? (
            <PluginPageLink key={item.href} href={item.href} className={linkClassName}>
              <Icon className="h-4 w-4" />
              {item.title}
            </PluginPageLink>
          ) : (
            <Link key={item.href} href={item.href} className={linkClassName}>
              <Icon className="h-4 w-4" />
              {item.title}
            </Link>
          )
        })}
        <Suspense fallback={null}>
          <PluginSlot slot="user.layout.sidebar.menu.after" context={userSidebarPluginContext} />
        </Suspense>
        {visibleRuntimeMenuItems.map((item) => {
          const Icon = item.icon
          const isActive = item.matchDescendants
            ? isPluginMenuPathActive(pathname || '/', item.href)
            : pathname === item.href

          const linkClassName = cn(
            'flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-all',
            isActive
              ? 'bg-primary text-primary-foreground shadow-sm'
              : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground'
          )

          return (
            <PluginPageLink key={item.href} href={item.href} className={linkClassName}>
              <Icon className="h-4 w-4" />
              {item.title}
            </PluginPageLink>
          )
        })}
        <Suspense fallback={null}>
          <PluginSlot
            slot="user.layout.sidebar.runtime_menu.after"
            context={userSidebarPluginContext}
          />
        </Suspense>
      </nav>

      <div className="space-y-2 border-t p-3">
        <Suspense fallback={null}>
          <PluginSlot slot="user.layout.sidebar.bottom" context={userSidebarPluginContext} />
        </Suspense>
        {guestMode ? (
          <Suspense fallback={null}>
            <PluginSlot
              slot="user.layout.sidebar.guest_actions.before"
              context={userSidebarPluginContext}
            />
          </Suspense>
        ) : (
          <Suspense fallback={null}>
            <PluginSlot
              slot="user.layout.sidebar.authed_actions.before"
              context={userSidebarPluginContext}
            />
          </Suspense>
        )}
        {!guestMode && isAdmin && (
          <Button asChild variant="outline" className="w-full justify-start" size="sm">
            <Link href="/admin/dashboard">
              <Shield className="mr-2 h-4 w-4" />
              {t.sidebar.adminPanel}
            </Link>
          </Button>
        )}
        <LanguageSwitcher />
        {guestMode ? (
          <>
            <Button
              variant="outline"
              className="w-full justify-start"
              size="sm"
              onClick={() => goToAuth('/login')}
            >
              <LogIn className="mr-2 h-4 w-4" />
              {t.auth.login}
            </Button>
            <Button
              variant="outline"
              className="w-full justify-start"
              size="sm"
              onClick={() => goToAuth('/register')}
            >
              <UserPlus className="mr-2 h-4 w-4" />
              {t.auth.register}
            </Button>
          </>
        ) : (
          <Button
            variant="outline"
            className="w-full justify-start"
            size="sm"
            onClick={() => {
              if (typeof window !== 'undefined') {
                clearToken()
                window.location.href = '/login'
              }
            }}
          >
            <LogOut className="mr-2 h-4 w-4" />
            {t.auth.logout}
          </Button>
        )}
      </div>
    </div>
  )
}
