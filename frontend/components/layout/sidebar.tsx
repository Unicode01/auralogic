'use client'

import { useState, useEffect } from 'react'
import Link from 'next/link'
import { usePathname } from 'next/navigation'
import { cn } from '@/lib/utils'
import { LayoutDashboard, Package, Users, Settings, Key, ArrowLeft, FileText, ShoppingBag, Warehouse, ShieldCheck, CreditCard, MessageSquare, BarChart3, Tag, BookOpen, Megaphone } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { usePermission } from '@/hooks/use-permission'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { LanguageSwitcher } from '@/components/layout/language-switcher'

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

export function Sidebar() {
  const pathname = usePathname()
  const { hasPermission, isSuperAdmin, user } = usePermission()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  const [mounted, setMounted] = useState(false)

  useEffect(() => {
    setMounted(true)
  }, [])

  // 过滤出用户有权限访问的菜单项
  const visibleMenuItems = mounted ? menuItems.filter((item) => {
    if (item.superAdminOnly && !isSuperAdmin()) return false
    if (!item.permission) return true
    return hasPermission(item.permission)
  }) : menuItems.filter((item) => !item.permission && !item.superAdminOnly)

  return (
    <div className="w-64 border-r bg-card flex flex-col">
      <div className="p-6">
        <h2 className="text-lg font-bold">{t.admin.adminPanel}</h2>
      </div>

      <nav className="space-y-1 px-3 flex-1">
        {visibleMenuItems.map((item) => {
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
              {t.admin[item.titleKey]}
            </Link>
          )
        })}
      </nav>

      <div className="p-3 border-t space-y-2">
        <LanguageSwitcher />
        <Button asChild variant="outline" className="w-full justify-start" size="sm">
          <Link href="/orders">
            <ArrowLeft className="h-4 w-4 mr-2" />
            {t.admin.backToUser}
          </Link>
        </Button>
      </div>
    </div>
  )
}
