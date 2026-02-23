'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import { useAuth } from '@/hooks/use-auth'
import { useIsMobile } from '@/hooks/use-mobile'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { UserSidebar } from '@/components/layout/user-sidebar'
import { MobileBottomNav } from '@/components/layout/mobile-bottom-nav'
import { CartProvider } from '@/contexts/cart-context'
import { AnnouncementPopup } from '@/components/announcement-popup'

export default function UserLayout({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isLoading } = useAuth()
  const { isMobile, mounted: mobileMounted } = useIsMobile()
  const { locale, mounted: localeMounted } = useLocale()
  const router = useRouter()
  const [mounted, setMounted] = useState(false)

  useEffect(() => {
    setMounted(true)
  }, [])

  useEffect(() => {
    if (mounted && !isLoading && !isAuthenticated) {
      router.push('/login')
    }
  }, [mounted, isAuthenticated, isLoading, router])

  const loadingText = localeMounted ? getTranslations(locale).common.loading : '...'

  // 在挂载前，显示完整布局（避免 hydration 错误）
  if (!mounted || !mobileMounted) {
    return (
      <div className="flex h-screen sidebar-layout">
        <UserSidebar className="hidden md:flex" />
        <main className="flex-1 overflow-y-auto p-4 md:p-8 pb-20 md:pb-8">
          <div className="flex items-center justify-center h-full">
            <div className="text-center">{loadingText}</div>
          </div>
        </main>
        <div className="md:hidden">
          <MobileBottomNav />
        </div>
      </div>
    )
  }

  // 加载中
  if (isLoading) {
    return (
      <div className="flex h-screen sidebar-layout">
        {!isMobile && <UserSidebar />}
        <main className={`flex-1 overflow-y-auto p-4 md:p-8 ${isMobile ? 'pb-20' : ''}`}>
          <div className="flex items-center justify-center h-full">
            <div className="text-center">{loadingText}</div>
          </div>
        </main>
        {isMobile && <MobileBottomNav />}
      </div>
    )
  }

  if (!isAuthenticated) {
    return null
  }

  return (
    <CartProvider>
      <div className="flex h-screen sidebar-layout">
        {/* 桌面端显示侧边栏 */}
        {!isMobile && <UserSidebar />}

        {/* 主内容区 - 移动端底部留出导航栏空间 */}
        <main className={`flex-1 overflow-y-auto p-4 md:p-8 ${isMobile ? 'pb-20' : ''}`}>
          {children}
        </main>

        {/* 移动端显示底部导航 */}
        {isMobile && <MobileBottomNav />}
      </div>
      <AnnouncementPopup />
    </CartProvider>
  )
}
