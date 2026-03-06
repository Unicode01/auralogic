'use client'

import { useEffect, useState } from 'react'
import { useRouter, usePathname } from 'next/navigation'
import { useQuery } from '@tanstack/react-query'
import { useAuth } from '@/hooks/use-auth'
import { useIsMobile } from '@/hooks/use-mobile'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { getPublicConfig } from '@/lib/api'
import { UserSidebar } from '@/components/layout/user-sidebar'
import { MobileBottomNav } from '@/components/layout/mobile-bottom-nav'
import { CartProvider } from '@/contexts/cart-context'
import { AnnouncementPopup } from '@/components/announcement-popup'

export default function UserLayout({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isLoading } = useAuth()
  const { isMobile, mounted: mobileMounted } = useIsMobile()
  const { locale, mounted: localeMounted } = useLocale()
  const router = useRouter()
  const pathname = usePathname()
  const [mounted, setMounted] = useState(false)
  const { data: publicConfigData, isLoading: publicConfigLoading } = useQuery({
    queryKey: ['publicConfig'],
    queryFn: getPublicConfig,
    staleTime: 5 * 60 * 1000,
  })

  const isProductsRoute = pathname === '/products' || pathname.startsWith('/products/')
  const isCartRoute = pathname === '/cart'
  const isPreferencesRoute = pathname === '/profile/preferences'
  const allowGuestProductBrowse = publicConfigData?.data?.allow_guest_product_browse === true
  const allowGuestAccess =
    !isAuthenticated &&
    allowGuestProductBrowse &&
    (isProductsRoute || isCartRoute || isPreferencesRoute)

  useEffect(() => {
    setMounted(true)
  }, [])

  useEffect(() => {
    if (mounted && !isLoading && !publicConfigLoading && !isAuthenticated && !allowGuestAccess) {
      router.push('/login')
    }
  }, [mounted, isAuthenticated, isLoading, publicConfigLoading, allowGuestAccess, router])

  const loadingText = localeMounted ? getTranslations(locale).common.loading : '...'
  // Show loading before mount or while guest-access config is being fetched.
  if (!mounted || !mobileMounted || (!isLoading && !isAuthenticated && publicConfigLoading)) {
    return (
      <div className="min-h-screen p-4 md:p-8">
        <div className="flex items-center justify-center h-[50vh]">
          <div className="text-center">{loadingText}</div>
        </div>
      </div>
    )
  }

  // 鍔犺浇涓?
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
    if (allowGuestAccess) {
      const guestLayout = (
        <div className="flex h-screen sidebar-layout">
          {!isMobile && <UserSidebar guestMode />}
          <main className={`flex-1 overflow-y-auto p-4 md:p-8 ${isMobile ? 'pb-20' : ''}`}>
            {children}
          </main>
          {isMobile && <MobileBottomNav guestMode />}
        </div>
      )

      if (isCartRoute) {
        return (
          <CartProvider>
            {guestLayout}
          </CartProvider>
        )
      }

      return guestLayout
    }
    return null
  }

  return (
    <CartProvider>
      <div className="flex h-screen sidebar-layout">
        {/* 妗岄潰绔樉绀轰晶杈规爮 */}
        {!isMobile && <UserSidebar />}

        {/* 涓诲唴瀹瑰尯 - 绉诲姩绔簳閮ㄧ暀鍑哄鑸爮绌洪棿 */}
        <main className={`flex-1 overflow-y-auto p-4 md:p-8 ${isMobile ? 'pb-20' : ''}`}>
          {children}
        </main>

        {/* 绉诲姩绔樉绀哄簳閮ㄥ鑸?*/}
        {isMobile && <MobileBottomNav />}
      </div>
      <AnnouncementPopup />
    </CartProvider>
  )
}
