'use client'

import { Suspense, useState, useEffect, useMemo, useRef } from 'react'
import { useQuery } from '@tanstack/react-query'
import Link from 'next/link'
import { usePathname, useSearchParams } from 'next/navigation'
import { cn } from '@/lib/utils'
import {
  LayoutDashboard,
  Package,
  Users,
  Settings,
  Key,
  ArrowLeft,
  FileText,
  ShoppingBag,
  Warehouse,
  ShieldCheck,
  CreditCard,
  MessageSquare,
  BarChart3,
  Tag,
  BookOpen,
  Megaphone,
  Send,
  Puzzle,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { usePermission } from '@/hooks/use-permission'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { PluginPageLink } from '@/components/plugins/plugin-page-link'
import { resolvePluginMenuIcon } from '@/lib/plugin-menu-icons'
import { usePluginBootstrapQuery } from '@/lib/plugin-bootstrap-query'
import {
  buildPluginBootstrapContextKey,
  readPluginSearchParams,
} from '@/lib/plugin-frontend-routing'
import { LanguageSwitcher } from '@/components/layout/language-switcher'
import { PluginSlot } from '@/components/plugins/plugin-slot'
import { getPublicConfig, type PluginFrontendBootstrapMenuItem } from '@/lib/api'
import { manifestString } from '@/lib/package-manifest-schema'
import {
  extractBootstrapMenus,
  getCachedAdminBootstrapMenusResult,
  setCachedAdminBootstrapMenus,
} from '@/lib/plugin-bootstrap-cache'
import { resolvePluginPlatformEnabled } from '@/lib/plugin-slot-behavior'

const menuItems = [
  {
    titleKey: 'dashboard' as const,
    href: '/admin/dashboard',
    icon: LayoutDashboard,
    permission: undefined,
    superAdminOnly: true,
  },
  {
    titleKey: 'analytics' as const,
    href: '/admin/analytics',
    icon: BarChart3,
    permission: undefined,
    superAdminOnly: true,
  },
  {
    titleKey: 'productManagement' as const,
    href: '/admin/products',
    icon: ShoppingBag,
    permission: 'product.view',
  },
  {
    titleKey: 'inventoryManagement' as const,
    href: '/admin/inventories',
    icon: Warehouse,
    permission: 'product.view',
  },
  {
    titleKey: 'promoCodeManagement' as const,
    href: '/admin/promo-codes',
    icon: Tag,
    permission: 'product.view',
  },
  {
    titleKey: 'orderManagement' as const,
    href: '/admin/orders',
    icon: Package,
    permission: 'order.view',
  },
  {
    titleKey: 'serialManagement' as const,
    href: '/admin/serials',
    icon: ShieldCheck,
    permission: 'serial.view',
  },
  {
    titleKey: 'userManagement' as const,
    href: '/admin/users',
    icon: Users,
    permission: 'user.view',
  },
  {
    titleKey: 'ticketManagement' as const,
    href: '/admin/tickets',
    icon: MessageSquare,
    permission: 'ticket.view',
  },
  {
    titleKey: 'knowledgeManagement' as const,
    href: '/admin/knowledge',
    icon: BookOpen,
    permission: 'knowledge.view',
  },
  {
    titleKey: 'announcementManagement' as const,
    href: '/admin/announcements',
    icon: Megaphone,
    permission: 'announcement.view',
  },
  {
    titleKey: 'marketingManagement' as const,
    href: '/admin/marketing',
    icon: Send,
    permission: 'marketing.view',
  },
  {
    titleKey: 'pluginManagement' as const,
    href: '/admin/plugins',
    icon: Puzzle,
    permission: 'system.config',
    superAdminOnly: true,
    pluginPlatformOnly: true,
  },
  {
    titleKey: 'paymentMethods' as const,
    href: '/admin/payment-methods',
    icon: CreditCard,
    permission: 'system.config',
  },
  {
    titleKey: 'apiKeys' as const,
    href: '/admin/api-keys',
    icon: Key,
    permission: 'api.manage',
  },
  {
    titleKey: 'systemLogs' as const,
    href: '/admin/logs',
    icon: FileText,
    permission: 'system.logs',
  },
  {
    titleKey: 'systemSettings' as const,
    href: '/admin/settings',
    icon: Settings,
    permission: 'system.config',
  },
]

type AdminRuntimeMenuItem = {
  id: string
  title: string
  href: string
  iconName: string
  priority: number
  requiredPermissions: string[]
  superAdminOnly: boolean
}

function normalizeRuntimeMenuPath(path: string): string {
  const trimmed = (path || '').trim()
  if (!trimmed) return ''
  const normalized = trimmed.startsWith('/') ? trimmed : `/${trimmed}`
  return normalized.replace(/\/+$/, '') || '/'
}

function parseRuntimeMenuItems(
  source: PluginFrontendBootstrapMenuItem[],
  locale?: string
): AdminRuntimeMenuItem[] {
  const out: AdminRuntimeMenuItem[] = []
  const seen = new Set<string>()

  source.forEach((item, index) => {
    if (!item || typeof item !== 'object') return
    const href = normalizeRuntimeMenuPath(String(item.path || ''))
    if (!href || !href.startsWith('/admin/plugin-pages/')) return
    const title = manifestString(item as Record<string, unknown>, 'title', locale)
    if (!title) return
    const id = String(item.id || '').trim() || `runtime-menu-${index}`
    if (seen.has(id)) return
    seen.add(id)
    const permissions = Array.isArray(item.required_permissions)
      ? item.required_permissions
          .map((permission) => String(permission || '').trim())
          .filter(Boolean)
      : []
    out.push({
      id,
      title,
      href,
      iconName: String(item.icon || '').trim(),
      priority: Number.isFinite(item.priority) ? Number(item.priority) : 0,
      requiredPermissions: permissions,
      superAdminOnly: !!item.super_admin_only,
    })
  })

  out.sort((a, b) => {
    if (a.priority === b.priority) return a.href.localeCompare(b.href)
    return a.priority - b.priority
  })
  return out
}

export function Sidebar() {
  const pathname = usePathname()
  const searchParams = useSearchParams()
  const { hasPermission, isSuperAdmin, user } = usePermission()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  const [mounted, setMounted] = useState(false)
  const [displayedBootstrapMenus, setDisplayedBootstrapMenus] = useState<
    PluginFrontendBootstrapMenuItem[]
  >([])
  const queryParams = useMemo(() => readPluginSearchParams(searchParams), [searchParams])
  const adminBootstrapScopeKey = user?.id
    ? `admin-user:${user.id}`
    : `admin-role:${user?.role || 'guest'}`
  const previousBootstrapScopeKeyRef = useRef(adminBootstrapScopeKey)
  const bootstrapContextKey = useMemo(
    () => buildPluginBootstrapContextKey(pathname || '/admin', queryParams),
    [pathname, queryParams]
  )
  const localizedBootstrapContextKey = useMemo(
    () => `${bootstrapContextKey}|locale=${locale}`,
    [bootstrapContextKey, locale]
  )

  useEffect(() => {
    setMounted(true)
  }, [])

  useEffect(() => {
    if (!mounted) return
    const cachedBootstrapResult = getCachedAdminBootstrapMenusResult(
      adminBootstrapScopeKey,
      localizedBootstrapContextKey
    )
    const scopeChanged = previousBootstrapScopeKeyRef.current !== adminBootstrapScopeKey
    previousBootstrapScopeKeyRef.current = adminBootstrapScopeKey

    if (cachedBootstrapResult.found) {
      setDisplayedBootstrapMenus(cachedBootstrapResult.menus)
      return
    }

    if (scopeChanged) {
      setDisplayedBootstrapMenus([])
    }
  }, [adminBootstrapScopeKey, localizedBootstrapContextKey, mounted])
  const { data: publicConfigData } = useQuery({
    queryKey: ['publicConfig'],
    queryFn: getPublicConfig,
    staleTime: 5 * 60 * 1000,
  })
  const pluginPlatformEnabled = resolvePluginPlatformEnabled(publicConfigData?.data, true)

  const pluginBootstrapQuery = usePluginBootstrapQuery({
    scope: 'admin',
    path: pathname || '/admin',
    queryParams,
    enabled: mounted && !!user,
  })

  const networkBootstrapMenus = useMemo(
    () => extractBootstrapMenus(pluginBootstrapQuery.data),
    [pluginBootstrapQuery.data]
  )

  useEffect(() => {
    if (!mounted) return
    if (pluginBootstrapQuery.data === undefined) return
    setDisplayedBootstrapMenus(networkBootstrapMenus)
    setCachedAdminBootstrapMenus(
      networkBootstrapMenus,
      adminBootstrapScopeKey,
      localizedBootstrapContextKey
    )
  }, [
    adminBootstrapScopeKey,
    localizedBootstrapContextKey,
    mounted,
    networkBootstrapMenus,
    pluginBootstrapQuery.data,
  ])

  const runtimeMenuItems = mounted && pluginPlatformEnabled
    ? parseRuntimeMenuItems(displayedBootstrapMenus, locale).filter((item) => {
        if (item.superAdminOnly && !isSuperAdmin()) return false
        if (
          item.requiredPermissions.length > 0 &&
          !item.requiredPermissions.every((permission) => hasPermission(permission))
        ) {
          return false
        }
        return true
      })
    : []

  // 过滤出用户有权限访问的菜单项
  const visibleMenuItems = mounted
    ? menuItems.filter((item) => {
        if (item.pluginPlatformOnly && !pluginPlatformEnabled) return false
        if (item.superAdminOnly && !isSuperAdmin()) return false
        if (!item.permission) return true
        return hasPermission(item.permission)
      })
    : menuItems.filter((item) => !item.permission && !item.superAdminOnly)
  const adminSidebarPluginContext = {
    view: 'admin_sidebar',
    layout: {
      current_path: pathname || '/admin',
      locale,
    },
    admin: {
      user_id: user?.id,
      role: user?.role,
      is_super_admin: isSuperAdmin(),
    },
    summary: {
      visible_builtin_count: visibleMenuItems.length,
      runtime_item_count: runtimeMenuItems.length,
      total_item_count: visibleMenuItems.length + runtimeMenuItems.length,
    },
  }

  return (
    <div className="flex w-64 flex-col border-r bg-card">
      <div className="p-6">
        <h2 className="text-lg font-bold">{t.admin.adminPanel}</h2>
        <Suspense fallback={null}>
          <PluginSlot slot="admin.layout.sidebar.top" context={adminSidebarPluginContext} />
        </Suspense>
      </div>

      <nav className="flex-1 space-y-1 overflow-y-auto px-3">
        {visibleMenuItems.map((item) => {
          const Icon = item.icon
          const isActive = pathname === item.href || pathname.startsWith(item.href + '/')

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
              {t.admin[item.titleKey]}
            </Link>
          )
        })}
        {runtimeMenuItems.map((item) => {
          const Icon = resolvePluginMenuIcon(item.iconName)
          const isActive = pathname === item.href || pathname.startsWith(item.href + '/')
          return (
            <PluginPageLink
              key={item.id}
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
            </PluginPageLink>
          )
        })}
      </nav>

      <div className="space-y-2 border-t p-3">
        <Suspense fallback={null}>
          <PluginSlot slot="admin.layout.sidebar.bottom" context={adminSidebarPluginContext} />
        </Suspense>
        <LanguageSwitcher />
        <Button asChild variant="outline" className="w-full justify-start" size="sm">
          <Link href="/orders">
            <ArrowLeft className="mr-2 h-4 w-4" />
            {t.admin.backToUser}
          </Link>
        </Button>
      </div>
    </div>
  )
}
