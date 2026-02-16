'use client'

import { useEffect, useState } from 'react'
import Link from 'next/link'
import { useAuth } from '@/hooks/use-auth'
import { Button } from '@/components/ui/button'
import { UserNav } from './user-nav'
import { Package } from 'lucide-react'

export function Header() {
  const { user, isAuthenticated, isLoading } = useAuth()
  const [mounted, setMounted] = useState(false)

  useEffect(() => {
    setMounted(true)
  }, [])

  return (
    <header className="sticky top-0 z-50 border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
      <div className="container flex h-16 items-center justify-between">
        {/* Logo */}
        <Link href="/" className="flex items-center gap-2 font-bold text-xl">
          <Package className="h-6 w-6" />
          <span className="hidden sm:inline">AuraLogic</span>
        </Link>

        {/* 用户菜单 */}
        <div>
          {!mounted || isLoading ? (
            <div className="h-10 w-20 animate-pulse bg-muted rounded" />
          ) : isAuthenticated ? (
            <UserNav user={user} />
          ) : (
            <Button asChild>
              <Link href="/login">登录</Link>
            </Button>
          )}
        </div>
      </div>
    </header>
  )
}

