'use client'

import Link from 'next/link'
import { usePathname } from 'next/navigation'
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
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { getPublicConfig } from '@/lib/api'
import { LanguageSwitcher } from './language-switcher'
import { clearToken } from '@/lib/auth'

const getUserMenuItems = (t: any) => [
  { title: t.sidebar.productCenter, href: '/products', icon: ShoppingBag },
  { title: t.sidebar.cart || 'Cart', href: '/cart', icon: ShoppingCart },
  { title: t.sidebar.myOrders, href: '/orders', icon: Package },
  { title: t.sidebar.serialVerify, href: '/serial-verify', icon: ShieldCheck },
  { title: t.sidebar.supportCenter || 'Support', href: '/tickets', icon: MessageSquare },
  { title: t.sidebar.knowledgeBase || 'Knowledge', href: '/knowledge', icon: BookOpen },
  { title: t.sidebar.announcements || 'Announcements', href: '/announcements', icon: Megaphone },
  { title: t.sidebar.profile, href: '/profile', icon: User },
  { title: t.sidebar.accountSettings, href: '/profile/settings', icon: Settings },
]

const getGuestMenuItems = (t: any) => [
  { title: t.sidebar.productCenter, href: '/products', icon: ShoppingBag },
  { title: t.sidebar.cart || 'Cart', href: '/cart', icon: ShoppingCart },
  { title: t.sidebar.serialVerify, href: '/serial-verify', icon: ShieldCheck },
  { title: t.sidebar.preferences, href: '/profile/preferences', icon: Bell },
]

interface UserSidebarProps {
  className?: string
  guestMode?: boolean
}

export function UserSidebar({ className, guestMode = false }: UserSidebarProps) {
  const pathname = usePathname()
  const { user } = useAuth()
  const { locale, mounted } = useLocale()
  const t = getTranslations(locale)

  const { data: publicConfigData } = useQuery({
    queryKey: ['publicConfig'],
    queryFn: getPublicConfig,
    staleTime: 5 * 60 * 1000,
  })

  const ticketEnabled = publicConfigData?.data?.ticket?.enabled ?? true
  const serialEnabled = publicConfigData?.data?.serial?.enabled ?? true
  const filterMenuItems = (items: Array<{ title: string; href: string; icon: any }>) =>
    items.filter((item) => {
      if (item.href === '/serial-verify' && !serialEnabled) return false
      if (!guestMode && item.href === '/tickets' && !ticketEnabled) return false
      return true
    })

  const baseMenuItems = guestMode ? getGuestMenuItems(t) : getUserMenuItems(t)
  const menuItems = filterMenuItems(baseMenuItems)
  const isAdmin = user?.role === 'admin' || user?.role === 'super_admin'

  if (!mounted) {
    const defaultT = getTranslations('zh')
    const defaultMenuItems = filterMenuItems(
      guestMode ? getGuestMenuItems(defaultT) : getUserMenuItems(defaultT)
    )

    return (
      <div className={cn('w-64 border-r bg-card flex-col hidden md:flex', className)}>
        <div className="p-6">
          <h2 className="text-lg font-bold">AuraLogic</h2>
          <p className="text-sm text-muted-foreground">
            {guestMode ? `${defaultT.auth.login} / ${defaultT.auth.register}` : defaultT.sidebar.welcome}
          </p>
        </div>

        <nav className="space-y-1 px-3 flex-1 overflow-y-auto">
          {defaultMenuItems.map((item) => {
            const Icon = item.icon
            const isActive = guestMode
              ? pathname === item.href || pathname.startsWith(item.href + '/')
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

        <div className="p-3 border-t space-y-2">
          <LanguageSwitcher />
          {guestMode ? (
            <>
              <Button asChild variant="outline" className="w-full justify-start" size="sm">
                <Link href="/login">
                  <LogIn className="h-4 w-4 mr-2" />
                  {defaultT.auth.login}
                </Link>
              </Button>
              <Button asChild variant="outline" className="w-full justify-start" size="sm">
                <Link href="/register">
                  <UserPlus className="h-4 w-4 mr-2" />
                  {defaultT.auth.register}
                </Link>
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
              <LogOut className="h-4 w-4 mr-2" />
              {defaultT.auth.logout}
            </Button>
          )}
        </div>
      </div>
    )
  }

  return (
    <div className={cn('w-64 border-r bg-card flex-col hidden md:flex', className)}>
      <div className="p-6">
        <h2 className="text-lg font-bold">AuraLogic</h2>
        <p className="text-sm text-muted-foreground">
          {guestMode ? `${t.auth.login} / ${t.auth.register}` : t.sidebar.welcome}
        </p>
      </div>

      <nav className="space-y-1 px-3 flex-1 overflow-y-auto">
        {menuItems.map((item) => {
          const Icon = item.icon
          const isActive = guestMode
            ? pathname === item.href || pathname.startsWith(item.href + '/')
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

      <div className="p-3 border-t space-y-2">
        {!guestMode && isAdmin && (
          <Button asChild variant="outline" className="w-full justify-start" size="sm">
            <Link href="/admin/dashboard">
              <Shield className="h-4 w-4 mr-2" />
              {t.sidebar.adminPanel}
            </Link>
          </Button>
        )}
        <LanguageSwitcher />
        {guestMode ? (
          <>
            <Button asChild variant="outline" className="w-full justify-start" size="sm">
              <Link href="/login">
                <LogIn className="h-4 w-4 mr-2" />
                {t.auth.login}
              </Link>
            </Button>
            <Button asChild variant="outline" className="w-full justify-start" size="sm">
              <Link href="/register">
                <UserPlus className="h-4 w-4 mr-2" />
                {t.auth.register}
              </Link>
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
            <LogOut className="h-4 w-4 mr-2" />
            {t.auth.logout}
          </Button>
        )}
      </div>
    </div>
  )
}
