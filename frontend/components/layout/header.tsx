'use client'
/* eslint-disable @next/next/no-img-element */

import { useEffect, useState } from 'react'
import Link from 'next/link'
import { useAuth } from '@/hooks/use-auth'
import { useAuthEntry } from '@/hooks/use-auth-entry'
import { Button } from '@/components/ui/button'
import { UserNav } from './user-nav'
import { Package } from 'lucide-react'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePublicBranding } from '@/hooks/use-public-branding'

export function Header() {
  const { user, isAuthenticated, isLoading } = useAuth()
  const { goToAuth } = useAuthEntry()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  const { appName, logoUrl } = usePublicBranding()
  const [mounted, setMounted] = useState(false)

  useEffect(() => {
    setMounted(true)
  }, [])

  return (
    <header className="sticky top-0 z-50 border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
      <div className="container flex h-16 items-center justify-between">
        {/* Logo */}
        <Link href="/" className="flex items-center gap-2 font-bold text-xl">
          {logoUrl ? (
            <img src={logoUrl} alt={appName} className="max-h-8 w-auto max-w-[160px] object-contain" />
          ) : (
            <>
              <Package className="h-6 w-6" />
              <span className="hidden sm:inline">{appName}</span>
            </>
          )}
        </Link>

        {/* 用户菜单 */}
        <div>
          {!mounted || isLoading ? (
            <div className="h-10 w-20 rounded bg-muted animate-pulse sm:w-40" />
          ) : isAuthenticated ? (
            <UserNav user={user} />
          ) : (
            <div className="flex items-center gap-2">
              <Button
                variant="ghost"
                className="hidden sm:inline-flex"
                onClick={() => goToAuth('/register')}
              >
                {t.auth.register}
              </Button>
              <Button onClick={() => goToAuth('/login')}>
                {t.auth.login}
              </Button>
            </div>
          )}
        </div>
      </div>
    </header>
  )
}

