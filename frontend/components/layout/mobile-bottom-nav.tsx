'use client'

import Link from 'next/link'
import { usePathname } from 'next/navigation'
import { cn } from '@/lib/utils'
import { ShoppingBag, ShoppingCart, Package, User, LogIn, UserPlus } from 'lucide-react'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'

interface MobileBottomNavProps {
  guestMode?: boolean
}

export function MobileBottomNav({ guestMode = false }: MobileBottomNavProps) {
  const pathname = usePathname()
  const { locale, mounted } = useLocale()
  const t = getTranslations(locale)

  const navItems = guestMode
    ? [
      { title: t.sidebar.productCenter, href: '/products', icon: ShoppingBag },
      { title: t.sidebar.cart || 'Cart', href: '/cart', icon: ShoppingCart },
      { title: t.auth.login, href: '/login', icon: LogIn },
      { title: t.auth.register, href: '/register', icon: UserPlus },
    ]
    : [
      { title: t.sidebar.productCenter, href: '/products', icon: ShoppingBag },
      { title: t.sidebar.cart || 'Cart', href: '/cart', icon: ShoppingCart },
      { title: t.sidebar.myOrders, href: '/orders', icon: Package },
      { title: t.sidebar.profile, href: '/profile', icon: User },
    ]

  if (!mounted) return null

  return (
    <nav className="fixed bottom-0 left-0 right-0 z-50 bg-background border-t safe-area-bottom">
      <div className="flex items-center justify-around h-16">
        {navItems.map((item) => {
          const Icon = item.icon
          const isActive = pathname === item.href || pathname.startsWith(item.href + '/')

          return (
            <Link
              key={item.href}
              href={item.href}
              className={cn(
                'flex flex-col items-center justify-center flex-1 h-full gap-1 transition-colors',
                isActive ? 'text-primary' : 'text-muted-foreground hover:text-foreground'
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
