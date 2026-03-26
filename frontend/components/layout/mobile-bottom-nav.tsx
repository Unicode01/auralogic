'use client'

import { Suspense } from 'react'
import Link from 'next/link'
import { usePathname, useSearchParams } from 'next/navigation'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  Bell,
  BookOpen,
  LogIn,
  Megaphone,
  MessageSquare,
  MoreHorizontal,
  Package,
  Settings,
  Shield,
  ShieldCheck,
  ShoppingBag,
  ShoppingCart,
  User,
  UserPlus,
} from 'lucide-react'

import { useAuth } from '@/hooks/use-auth'
import { useAuthEntry } from '@/hooks/use-auth-entry'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { cn } from '@/lib/utils'
import {
  getCartCount,
  getPublicConfig,
  getUnreadMandatoryAnnouncements,
  type PluginFrontendBootstrapMenuItem,
} from '@/lib/api'
import { GUEST_CART_CHANGED_EVENT, getGuestCart } from '@/lib/guest-cart'
import { resolvePluginMenuIcon } from '@/lib/plugin-menu-icons'
import { usePluginBootstrapQuery } from '@/lib/plugin-bootstrap-query'
import {
  buildPluginBootstrapContextKey,
  isPluginMenuPathActive,
  readPluginSearchParams,
} from '@/lib/plugin-frontend-routing'
import {
  extractBootstrapMenus,
  getCachedUserBootstrapMenusResult,
  setCachedUserBootstrapMenus,
} from '@/lib/plugin-bootstrap-cache'
import { parseUserPluginMenuItems } from '@/lib/plugin-user-menu'
import { Badge } from '@/components/ui/badge'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { PluginPageLink } from '@/components/plugins/plugin-page-link'
import { PluginSlot } from '@/components/plugins/plugin-slot'
import { resolvePluginPlatformEnabled } from '@/lib/plugin-slot-behavior'

interface MobileBottomNavProps {
  guestMode?: boolean
}

type AuthEntryTarget = '/login' | '/register'

type NavItem = {
  title: string
  href?: string
  icon: any
  badgeCount?: number
  matchDescendants?: boolean
  pluginRuntime?: boolean
  authEntryTarget?: AuthEntryTarget
  section?: 'builtin' | 'plugin' | 'auth'
}

function formatBadgeCount(count: number): string {
  if (count > 99) return '99+'
  return String(count)
}

function NavCountBadge({ count }: { count?: number }) {
  if (!count || count <= 0) return null

  return (
    <Badge className="pointer-events-none absolute -right-2.5 -top-2 flex h-5 min-w-5 items-center justify-center rounded-full px-1 text-[10px] leading-none shadow-sm">
      {formatBadgeCount(count)}
    </Badge>
  )
}

function isNavItemActive(pathname: string, item: NavItem): boolean {
  if (!pathname || !item.href) return false
  if (item.matchDescendants || item.pluginRuntime) {
    return isPluginMenuPathActive(pathname, item.href)
  }
  return pathname === item.href
}

export function MobileBottomNav({ guestMode = false }: MobileBottomNavProps) {
  const pathname = usePathname()
  const searchParams = useSearchParams()
  const { user, isAuthenticated } = useAuth()
  const { goToAuth } = useAuthEntry()
  const { locale, mounted } = useLocale()
  const t = getTranslations(locale)
  const [guestCartCount, setGuestCartCount] = useState(0)
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

  const {
    data: publicConfigData,
    isError: publicConfigLoadFailed,
  } = useQuery({
    queryKey: ['publicConfig'],
    queryFn: getPublicConfig,
    staleTime: 5 * 60 * 1000,
  })

  const { data: cartCountData } = useQuery({
    queryKey: ['cartCount'],
    queryFn: getCartCount,
    enabled: mounted && !guestMode && isAuthenticated,
    staleTime: 60 * 1000,
  })

  const { data: unreadAnnouncementsData } = useQuery({
    queryKey: ['unreadMandatoryAnnouncements'],
    queryFn: getUnreadMandatoryAnnouncements,
    enabled: mounted && !guestMode && isAuthenticated,
    staleTime: 60 * 1000,
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

  useEffect(() => {
    if (!mounted || !guestMode) return

    const updateGuestCartCount = () => {
      setGuestCartCount(getGuestCart().length)
    }

    const handleStorage = () => updateGuestCartCount()
    const handleGuestCartChanged = () => updateGuestCartCount()

    updateGuestCartCount()
    window.addEventListener('storage', handleStorage)
    window.addEventListener(GUEST_CART_CHANGED_EVENT, handleGuestCartChanged as EventListener)

    return () => {
      window.removeEventListener('storage', handleStorage)
      window.removeEventListener(GUEST_CART_CHANGED_EVENT, handleGuestCartChanged as EventListener)
    }
  }, [guestMode, mounted])

  const ticketEnabled = publicConfigData?.data?.ticket?.enabled ?? true
  const serialEnabled = publicConfigData?.data?.serial?.enabled ?? true
  const pluginPlatformEnabled = resolvePluginPlatformEnabled(publicConfigData?.data, true)
  const isAdmin = user?.role === 'admin' || user?.role === 'super_admin'
  const cartCountPayload = cartCountData as any
  const unreadAnnouncementsPayload = unreadAnnouncementsData as any
  const authCartCount = cartCountPayload?.data?.item_count ?? cartCountPayload?.item_count ?? 0
  const announcementCount = Array.isArray(unreadAnnouncementsPayload?.data)
    ? unreadAnnouncementsPayload.data.length
    : Array.isArray(unreadAnnouncementsPayload)
      ? unreadAnnouncementsPayload.length
      : 0
  const cartCount = guestMode ? guestCartCount : authCartCount
  const runtimeMenuItems = useMemo<NavItem[]>(
    () =>
      (pluginPlatformEnabled ? parseUserPluginMenuItems(displayedBootstrapMenus, locale) : [])
        .filter((item) => !guestMode || item.guestVisible)
        .map((item) => ({
          title: item.title,
          href: item.href,
          icon: resolvePluginMenuIcon(item.iconName),
          matchDescendants: true,
          pluginRuntime: true,
          section: 'plugin' as const,
        })),
    [displayedBootstrapMenus, guestMode, locale, pluginPlatformEnabled]
  )

  const primaryItems = useMemo<NavItem[]>(
    () =>
      guestMode
        ? [
            {
              title: t.sidebar.productCenter,
              href: '/products',
              icon: ShoppingBag,
              matchDescendants: true,
            },
            {
              title: t.sidebar.cart || 'Cart',
              href: '/cart',
              icon: ShoppingCart,
              badgeCount: cartCount,
            },
          ]
        : [
            {
              title: t.sidebar.productCenter,
              href: '/products',
              icon: ShoppingBag,
              matchDescendants: true,
            },
            {
              title: t.sidebar.cart || 'Cart',
              href: '/cart',
              icon: ShoppingCart,
              badgeCount: cartCount,
            },
            {
              title: t.sidebar.myOrders,
              href: '/orders',
              icon: Package,
              matchDescendants: true,
            },
          ],
    [cartCount, guestMode, t.sidebar.cart, t.sidebar.myOrders, t.sidebar.productCenter]
  )

  const moreItems = useMemo<NavItem[]>(() => {
    if (guestMode) {
      const guestItems: NavItem[] = []

      if (serialEnabled) {
        guestItems.push({
          title: t.sidebar.serialVerify,
          href: '/serial-verify',
          icon: ShieldCheck,
          matchDescendants: true,
          section: 'builtin',
        })
      }

      guestItems.push({
        title: t.sidebar.preferences,
        href: '/profile/preferences',
        icon: Bell,
        matchDescendants: true,
        section: 'builtin',
      })

      return [
        ...guestItems,
        ...runtimeMenuItems,
        {
          title: t.auth.login,
          icon: LogIn,
          authEntryTarget: '/login',
          section: 'auth',
        },
        {
          title: t.auth.register,
          icon: UserPlus,
          authEntryTarget: '/register',
          section: 'auth',
        },
      ]
    }

    const userItems: NavItem[] = []

    if (serialEnabled) {
      userItems.push({
        title: t.sidebar.serialVerify,
        href: '/serial-verify',
        icon: ShieldCheck,
        matchDescendants: true,
        section: 'builtin',
      })
    }

    if (ticketEnabled) {
      userItems.push({
        title: t.sidebar.supportCenter || 'Support',
        href: '/tickets',
        icon: MessageSquare,
        matchDescendants: true,
        section: 'builtin',
      })
    }

    userItems.push(
      {
        title: t.sidebar.knowledgeBase || 'Knowledge',
        href: '/knowledge',
        icon: BookOpen,
        matchDescendants: true,
        section: 'builtin',
      },
      {
        title: t.sidebar.announcements || 'Announcements',
        href: '/announcements',
        icon: Megaphone,
        badgeCount: announcementCount,
        matchDescendants: true,
        section: 'builtin',
      },
      {
        title: t.sidebar.profile,
        href: '/profile',
        icon: User,
        section: 'builtin',
      },
      {
        title: t.sidebar.preferences,
        href: '/profile/preferences',
        icon: Bell,
        matchDescendants: true,
        section: 'builtin',
      },
      {
        title: t.sidebar.accountSettings,
        href: '/profile/settings',
        icon: Settings,
        matchDescendants: true,
        section: 'builtin',
      }
    )

    if (isAdmin) {
      userItems.push({
        title: t.sidebar.adminPanel,
        href: '/admin/dashboard',
        icon: Shield,
        matchDescendants: true,
        section: 'builtin',
      })
    }

    return [...userItems, ...runtimeMenuItems]
  }, [
    announcementCount,
    guestMode,
    isAdmin,
    runtimeMenuItems,
    serialEnabled,
    t.auth.login,
    t.auth.register,
    t.sidebar.accountSettings,
    t.sidebar.adminPanel,
    t.sidebar.announcements,
    t.sidebar.knowledgeBase,
    t.sidebar.preferences,
    t.sidebar.profile,
    t.sidebar.serialVerify,
    t.sidebar.supportCenter,
    ticketEnabled,
  ])

  const moreActive = moreItems.some((item) => isNavItemActive(pathname || '', item))
  const mobileNavPluginContext = useMemo(
    () => ({
      view: 'user_mobile_bottom_nav',
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
      menu: {
        primary_items: primaryItems.map((item) => ({
          title: item.title,
          href: item.href || null,
          badge_count: item.badgeCount ?? 0,
          plugin_runtime: !!item.pluginRuntime,
        })),
        more_items: moreItems.map((item) => ({
          title: item.title,
          href: item.href || null,
          badge_count: item.badgeCount ?? 0,
          plugin_runtime: !!item.pluginRuntime,
          auth_entry_target: item.authEntryTarget || null,
          section: item.section || 'builtin',
        })),
      },
      summary: {
        primary_item_count: primaryItems.length,
        more_item_count: moreItems.length,
        runtime_item_count: runtimeMenuItems.length,
        cart_badge_count: cartCount,
        announcement_badge_count: announcementCount,
      },
      state: {
        guest_mode: guestMode,
        has_runtime_items: runtimeMenuItems.length > 0,
        has_admin_entry: !guestMode && isAdmin,
        has_cart_badge: cartCount > 0,
        has_announcement_badge: announcementCount > 0,
        bootstrap_loading: pluginBootstrapQuery.isLoading,
        bootstrap_error: pluginBootstrapQuery.isError,
        public_config_load_failed: publicConfigLoadFailed,
      },
    }),
    [
      announcementCount,
      cartCount,
      guestMode,
      isAdmin,
      locale,
      moreItems,
      pathname,
      pluginBootstrapQuery.isError,
      pluginBootstrapQuery.isLoading,
      primaryItems,
      publicConfigLoadFailed,
      runtimeMenuItems.length,
      user?.id,
      user?.role,
    ]
  )

  if (!mounted) return null

  return (
    <nav className="safe-area-bottom fixed bottom-0 left-0 right-0 z-50 border-t bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/85">
      <Suspense fallback={null}>
        <PluginSlot slot="user.layout.mobile_nav.top" context={mobileNavPluginContext} />
      </Suspense>
      <div className={cn('grid h-16 items-stretch', guestMode ? 'grid-cols-3' : 'grid-cols-4')}>
        {primaryItems.map((item) => {
          const Icon = item.icon
          const isActive = isNavItemActive(pathname || '', item)

          return (
            <Link
              key={item.href}
              href={item.href || '/'}
              className={cn(
                'flex min-w-0 flex-col items-center justify-center gap-1 px-2 transition-colors',
                isActive ? 'text-primary' : 'text-muted-foreground hover:text-foreground'
              )}
            >
              <span className="relative">
                <Icon className={cn('h-5 w-5', isActive && 'text-primary')} />
                <NavCountBadge count={item.badgeCount} />
              </span>
              <span className="max-w-full truncate text-[11px] font-medium">{item.title}</span>
            </Link>
          )
        })}
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <button
              type="button"
              className={cn(
                'flex min-w-0 flex-col items-center justify-center gap-1 px-2 transition-colors',
                moreActive ? 'text-primary' : 'text-muted-foreground hover:text-foreground'
              )}
              aria-label={t.common.more}
              title={t.common.more}
            >
              <span className="relative">
                <MoreHorizontal className={cn('h-5 w-5', moreActive && 'text-primary')} />
                <NavCountBadge count={guestMode ? 0 : announcementCount} />
              </span>
              <span className="max-w-full truncate text-[11px] font-medium">{t.common.more}</span>
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent
            align="end"
            side="top"
            className="mb-2 max-h-[min(70vh,28rem)] w-60 max-w-[calc(100vw-1rem)] overflow-y-auto"
          >
            <Suspense fallback={null}>
              <PluginSlot
                slot="user.layout.mobile_nav.more.before"
                context={mobileNavPluginContext}
              />
            </Suspense>
            {moreItems.map((item, index) => {
              const Icon = item.icon
              const isActive = isNavItemActive(pathname || '', item)
              const previousSection = index > 0 ? moreItems[index - 1].section : item.section
              const showSeparator = previousSection !== item.section

              return (
                <div key={item.href || item.authEntryTarget || item.title}>
                  {showSeparator ? <DropdownMenuSeparator /> : null}
                  {item.authEntryTarget ? (
                    <DropdownMenuItem onSelect={() => goToAuth(item.authEntryTarget!)}>
                      <Icon className="mr-2 h-4 w-4" />
                      <span className="min-w-0 flex-1 truncate">{item.title}</span>
                    </DropdownMenuItem>
                  ) : item.pluginRuntime ? (
                    <DropdownMenuItem asChild>
                      <PluginPageLink
                        href={item.href || '/'}
                        className={cn(
                          'flex w-full items-center gap-2',
                          isActive && 'font-semibold text-foreground'
                        )}
                      >
                        <Icon className="h-4 w-4" />
                        <span className="min-w-0 flex-1 truncate">{item.title}</span>
                        {item.badgeCount && item.badgeCount > 0 ? (
                          <Badge variant="secondary" className="rounded-full px-1.5 text-[10px]">
                            {formatBadgeCount(item.badgeCount)}
                          </Badge>
                        ) : null}
                      </PluginPageLink>
                    </DropdownMenuItem>
                  ) : (
                    <DropdownMenuItem asChild>
                      <Link
                        href={item.href || '/'}
                        className={cn(
                          'flex w-full items-center gap-2',
                          isActive && 'font-semibold text-foreground'
                        )}
                      >
                        <Icon className="h-4 w-4" />
                        <span className="min-w-0 flex-1 truncate">{item.title}</span>
                        {item.badgeCount && item.badgeCount > 0 ? (
                          <Badge variant="secondary" className="rounded-full px-1.5 text-[10px]">
                            {formatBadgeCount(item.badgeCount)}
                          </Badge>
                        ) : null}
                      </Link>
                    </DropdownMenuItem>
                  )}
                </div>
              )
            })}
            <Suspense fallback={null}>
              <PluginSlot
                slot="user.layout.mobile_nav.more.after"
                context={mobileNavPluginContext}
              />
            </Suspense>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
      <Suspense fallback={null}>
        <PluginSlot slot="user.layout.mobile_nav.bottom" context={mobileNavPluginContext} />
      </Suspense>
    </nav>
  )
}
