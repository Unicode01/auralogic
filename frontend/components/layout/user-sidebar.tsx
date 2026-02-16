'use client'

import { useState, useEffect } from 'react'
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
  LogOut,
  Shield,
  ShieldCheck,
  MessageSquare,
  BookOpen,
  Megaphone
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useAuth } from '@/hooks/use-auth'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { getPublicConfig } from '@/lib/api'
import { LanguageSwitcher } from './language-switcher'

const getMenuItems = (t: any) => [
  {
    title: t.sidebar.productCenter,
    href: '/products',
    icon: ShoppingBag,
  },
  {
    title: t.sidebar.cart || '购物车',
    href: '/cart',
    icon: ShoppingCart,
  },
  {
    title: t.sidebar.myOrders,
    href: '/orders',
    icon: Package,
  },
  {
    title: t.sidebar.serialVerify,
    href: '/serial-verify',
    icon: ShieldCheck,
  },
  {
    title: t.sidebar.supportCenter || '客服中心',
    href: '/tickets',
    icon: MessageSquare,
  },
  {
    title: t.sidebar.knowledgeBase || '知识库',
    href: '/knowledge',
    icon: BookOpen,
  },
  {
    title: t.sidebar.announcements || '公告',
    href: '/announcements',
    icon: Megaphone,
  },
  {
    title: t.sidebar.profile,
    href: '/profile',
    icon: User,
  },
  {
    title: t.sidebar.accountSettings,
    href: '/profile/settings',
    icon: Settings,
  },
]

interface UserSidebarProps {
  className?: string
}

export function UserSidebar({ className }: UserSidebarProps) {
  const pathname = usePathname()
  const { user } = useAuth()
  const { locale, mounted } = useLocale()
  const t = getTranslations(locale)
  const allMenuItems = getMenuItems(t)

  const { data: publicConfigData } = useQuery({
    queryKey: ['publicConfig'],
    queryFn: getPublicConfig,
    staleTime: 5 * 60 * 1000,
  })

  const ticketEnabled = publicConfigData?.data?.ticket?.enabled ?? true
  const serialEnabled = publicConfigData?.data?.serial?.enabled ?? true

  // 根据配置过滤菜单项
  const menuItems = allMenuItems.filter((item) => {
    if (item.href === '/tickets' && !ticketEnabled) return false
    if (item.href === '/serial-verify' && !serialEnabled) return false
    return true
  })

  // 检查是否为管理员
  const isAdmin = user?.role === 'admin' || user?.role === 'super_admin'

  // 在客户端挂载前，使用默认中文避免 hydration 错误
  if (!mounted) {
    const defaultT = getTranslations('zh')
    const defaultMenuItems = getMenuItems(defaultT)

    return (
      <div className={cn('w-64 border-r bg-card flex-col hidden md:flex', className)}>
        <div className="p-6">
          <h2 className="text-lg font-bold">{defaultT.sidebar.userCenter}</h2>
          <p className="text-sm text-muted-foreground">{defaultT.sidebar.welcome}</p>
        </div>

        <nav className="space-y-1 px-3 flex-1 overflow-y-auto">
          {defaultMenuItems.map((item) => {
            const Icon = item.icon
            const isActive = pathname === item.href

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
          <Button
            variant="outline"
            className="w-full justify-start"
            size="sm"
            onClick={() => {
              if (typeof window !== 'undefined') {
                localStorage.removeItem('auth_token')
                window.location.href = '/login'
              }
            }}
          >
            <LogOut className="h-4 w-4 mr-2" />
            {defaultT.auth.logout}
          </Button>
        </div>
      </div>
    )
  }

  return (
    <div className={cn('w-64 border-r bg-card flex-col hidden md:flex', className)}>
      <div className="p-6">
        <h2 className="text-lg font-bold">{t.sidebar.userCenter}</h2>
        <p className="text-sm text-muted-foreground">{t.sidebar.welcome}</p>
      </div>

      <nav className="space-y-1 px-3 flex-1 overflow-y-auto">
        {menuItems.map((item) => {
          const Icon = item.icon
          // 精确匹配路由，避免 /profile/settings 同时激活 /profile
          const isActive = pathname === item.href

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
        {isAdmin && (
          <Button asChild variant="outline" className="w-full justify-start" size="sm">
            <Link href="/admin/dashboard">
              <Shield className="h-4 w-4 mr-2" />
              {t.sidebar.adminPanel}
            </Link>
          </Button>
        )}
        <LanguageSwitcher />
        <Button
          variant="outline"
          className="w-full justify-start"
          size="sm"
          onClick={() => {
            if (typeof window !== 'undefined') {
              localStorage.removeItem('auth_token')
              window.location.href = '/login'
            }
          }}
        >
          <LogOut className="h-4 w-4 mr-2" />
          {t.auth.logout}
        </Button>
      </div>
    </div>
  )
}

