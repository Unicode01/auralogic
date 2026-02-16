'use client'

import Link from 'next/link'
import { usePathname } from 'next/navigation'
import { cn } from '@/lib/utils'
import { ShoppingBag, ShoppingCart, Package, User } from 'lucide-react'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'

export function MobileBottomNav() {
  const pathname = usePathname()
  const { locale, mounted } = useLocale()
  const t = getTranslations(locale)

  const navItems = [
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
      title: t.sidebar.profile,
      href: '/profile',
      icon: User,
    },
  ]

  // 避免 hydration 错误
  if (!mounted) {
    const defaultT = getTranslations('zh')
    const defaultItems = [
      { title: defaultT.sidebar.productCenter, href: '/products', icon: ShoppingBag },
      { title: defaultT.sidebar.cart || '购物车', href: '/cart', icon: ShoppingCart },
      { title: defaultT.sidebar.myOrders, href: '/orders', icon: Package },
      { title: defaultT.sidebar.profile, href: '/profile', icon: User },
    ]

    return (
      <nav className="fixed bottom-0 left-0 right-0 z-50 bg-background border-t safe-area-bottom">
        <div className="flex items-center justify-around h-16">
          {defaultItems.map((item) => {
            const Icon = item.icon
            const isActive = pathname === item.href || pathname.startsWith(item.href + '/')

            return (
              <Link
                key={item.href}
                href={item.href}
                className={cn(
                  'flex flex-col items-center justify-center flex-1 h-full gap-1 transition-colors',
                  isActive
                    ? 'text-primary'
                    : 'text-muted-foreground'
                )}
              >
                <Icon className={cn('h-5 w-5', isActive && 'text-primary')} />
                <span className="text-xs font-medium">{item.title}</span>
              </Link>
            )
          })}
        </div>
      </nav>
    )
  }

  return (
    <nav className="fixed bottom-0 left-0 right-0 z-50 bg-background border-t safe-area-bottom">
      <div className="flex items-center justify-around h-16">
        {navItems.map((item) => {
          const Icon = item.icon
          // 检查是否激活 - 精确匹配或子路径匹配
          const isActive = pathname === item.href || pathname.startsWith(item.href + '/')

          return (
            <Link
              key={item.href}
              href={item.href}
              className={cn(
                'flex flex-col items-center justify-center flex-1 h-full gap-1 transition-colors',
                isActive
                  ? 'text-primary'
                  : 'text-muted-foreground hover:text-foreground'
              )}
            >
              <Icon className={cn('h-5 w-5', isActive && 'text-primary')} />
              <span className="text-xs font-medium">{item.title}</span>
            </Link>
          )
        })}
      </div>
    </nav>
  )
}
