'use client'

import Link from 'next/link'
import { Suspense, useEffect, useMemo, useState } from 'react'
import { usePathname, useSearchParams } from 'next/navigation'
import { useQuery } from '@tanstack/react-query'
import { LogIn, RefreshCw, ShieldAlert, ShoppingBag } from 'lucide-react'
import { useAuth } from '@/hooks/use-auth'
import { useAuthEntry } from '@/hooks/use-auth-entry'
import { useIsMobile } from '@/hooks/use-mobile'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { getPublicConfig, type PluginFrontendBootstrapRoute } from '@/lib/api'
import { matchPluginRoute, readPluginSearchParams } from '@/lib/plugin-frontend-routing'
import { usePluginBootstrapQuery } from '@/lib/plugin-bootstrap-query'
import { resolvePluginPlatformEnabled } from '@/lib/plugin-slot-behavior'
import { PluginSlot } from '@/components/plugins/plugin-slot'
import { UserSidebar } from '@/components/layout/user-sidebar'
import { MobileBottomNav } from '@/components/layout/mobile-bottom-nav'
import { CartProvider } from '@/contexts/cart-context'
import { AnnouncementPopup } from '@/components/announcement-popup'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { FullPageLoading } from '@/components/ui/page-loading'

function extractBootstrapRoutes(payload: any): PluginFrontendBootstrapRoute[] {
  if (Array.isArray(payload?.data?.routes)) {
    return payload.data.routes as PluginFrontendBootstrapRoute[]
  }
  if (Array.isArray(payload?.routes)) {
    return payload.routes as PluginFrontendBootstrapRoute[]
  }
  return []
}

function UserLayoutFallback() {
  return <FullPageLoading />
}

function UserLayoutFrame({
  children,
  guestMode = false,
  isMobile,
  slotContext,
}: {
  children: React.ReactNode
  guestMode?: boolean
  isMobile: boolean
  slotContext?: Record<string, unknown>
}) {
  return (
    <div className="sidebar-layout flex h-screen">
      {!isMobile && <UserSidebar guestMode={guestMode} />}
      <main className={`flex-1 overflow-y-auto p-4 md:p-8 ${isMobile ? 'pb-20' : ''}`}>
        {slotContext ? <PluginSlot slot="user.layout.content.top" context={slotContext} /> : null}
        {children}
        {slotContext ? (
          <PluginSlot slot="user.layout.content.bottom" context={slotContext} />
        ) : null}
      </main>
      {isMobile && <MobileBottomNav guestMode={guestMode} />}
    </div>
  )
}

function UserLayoutContent({ children }: { children: React.ReactNode }) {
  const { user, isAuthenticated, isLoading } = useAuth()
  const { isMobile, mounted: mobileMounted } = useIsMobile()
  const { locale, mounted: localeMounted } = useLocale()
  const { goToAuth } = useAuthEntry()
  const pathname = usePathname()
  const searchParams = useSearchParams()
  const [mounted, setMounted] = useState(false)
  const queryParams = useMemo(() => readPluginSearchParams(searchParams), [searchParams])
  const {
    data: publicConfigData,
    isLoading: publicConfigLoading,
    isError: publicConfigLoadFailed,
    refetch: refetchPublicConfig,
  } = useQuery({
    queryKey: ['publicConfig'],
    queryFn: getPublicConfig,
    staleTime: 5 * 60 * 1000,
  })

  const isProductsRoute = pathname === '/products' || pathname.startsWith('/products/')
  const isCartRoute = pathname === '/cart'
  const isPreferencesRoute = pathname === '/profile/preferences'
  const isPluginPageRoute = pathname === '/plugin-pages' || pathname.startsWith('/plugin-pages/')
  const allowGuestProductBrowse = publicConfigData?.data?.allow_guest_product_browse === true
  const pluginPlatformEnabled = useMemo(
    () => resolvePluginPlatformEnabled(publicConfigData?.data, true),
    [publicConfigData]
  )
  const pluginBootstrapQuery = usePluginBootstrapQuery({
    scope: 'public',
    path: pathname || '/',
    queryParams,
    enabled: !isAuthenticated && isPluginPageRoute && pluginPlatformEnabled,
  })
  const guestBootstrapLoading =
    !isLoading && !isAuthenticated && isPluginPageRoute && pluginBootstrapQuery.isLoading
  const guestAccessCheckFailed =
    !isLoading &&
    !isAuthenticated &&
    ((publicConfigLoadFailed && (isProductsRoute || isCartRoute || isPreferencesRoute)) ||
      (isPluginPageRoute && pluginBootstrapQuery.isError))
  const allowGuestPluginPage = useMemo(() => {
    if (!isPluginPageRoute || !pluginPlatformEnabled) {
      return false
    }
    const routes = extractBootstrapRoutes(pluginBootstrapQuery.data)
    for (const route of routes) {
      const routePath = typeof route.path === 'string' ? route.path : ''
      if (!routePath) continue
      if (!matchPluginRoute(routePath, pathname || '/').matched) continue
      return !!route.guest_visible
    }
    return false
  }, [isPluginPageRoute, pathname, pluginBootstrapQuery.data, pluginPlatformEnabled])
  const guestPluginPageDisabled = !isAuthenticated && isPluginPageRoute && !pluginPlatformEnabled
  const allowGuestAccess =
    !isAuthenticated &&
    ((allowGuestProductBrowse && (isProductsRoute || isCartRoute || isPreferencesRoute)) ||
      allowGuestPluginPage ||
      guestPluginPageDisabled)
  const isAdmin = user?.role === 'admin' || user?.role === 'super_admin'
  const loginRequired =
    !isLoading && !isAuthenticated && !allowGuestAccess && !guestAccessCheckFailed
  const authenticatedLayoutPluginContext = useMemo(
    () => ({
      view: 'user_layout_content',
      layout: {
        current_path: pathname || '/',
        locale,
        guest_mode: false,
        is_mobile: isMobile,
        query_params: queryParams,
      },
      auth: {
        is_loading: isLoading,
        is_authenticated: isAuthenticated,
        guest_access_allowed: allowGuestAccess,
      },
      state: {
        auth_loading: isLoading,
        guest_mode: false,
        guest_access_allowed: allowGuestAccess,
        guest_access_check_failed: guestAccessCheckFailed,
        login_required: loginRequired,
        public_config_load_failed: publicConfigLoadFailed,
        plugin_platform_enabled: pluginPlatformEnabled,
        plugin_bootstrap_loading: guestBootstrapLoading,
        plugin_bootstrap_load_failed: isPluginPageRoute && pluginBootstrapQuery.isError,
      },
      route: {
        is_plugin_page: isPluginPageRoute,
        is_products_route: isProductsRoute,
        is_cart_route: isCartRoute,
        is_preferences_route: isPreferencesRoute,
      },
      user: user
        ? {
            id: user.id,
            role: user.role,
            is_admin: isAdmin,
          }
        : null,
    }),
    [
      allowGuestAccess,
      isAdmin,
      isAuthenticated,
      isCartRoute,
      isLoading,
      isMobile,
      isPluginPageRoute,
      guestAccessCheckFailed,
      guestBootstrapLoading,
      isPreferencesRoute,
      isProductsRoute,
      loginRequired,
      locale,
      pathname,
      pluginBootstrapQuery.isError,
      pluginPlatformEnabled,
      publicConfigLoadFailed,
      queryParams,
      user,
    ]
  )
  const guestLayoutPluginContext = useMemo(
    () => ({
      ...authenticatedLayoutPluginContext,
      layout: {
        ...authenticatedLayoutPluginContext.layout,
        guest_mode: true,
      },
      state: {
        ...authenticatedLayoutPluginContext.state,
        guest_mode: true,
      },
      user: null,
    }),
    [authenticatedLayoutPluginContext]
  )

  useEffect(() => {
    setMounted(true)
  }, [])

  const t = getTranslations(localeMounted ? locale : 'zh')
  const loadingText = t.common.loading

  if (
    !mounted ||
    !mobileMounted ||
    (!isLoading && !isAuthenticated && (publicConfigLoading || guestBootstrapLoading))
  ) {
    return <FullPageLoading text={loadingText} />
  }

  if (isLoading) {
    return (
      <UserLayoutFrame isMobile={isMobile}>
        <FullPageLoading text={loadingText} />
      </UserLayoutFrame>
    )
  }

  if (!isAuthenticated) {
    if (guestAccessCheckFailed) {
      const handleRetryGuestAccess = () => {
        void refetchPublicConfig()
        if (isPluginPageRoute) {
          void pluginBootstrapQuery.refetch()
        }
      }

      return (
        <UserLayoutFrame guestMode isMobile={isMobile} slotContext={guestLayoutPluginContext}>
          <div className="mx-auto flex min-h-[calc(100vh-10rem)] max-w-2xl items-center justify-center">
            <Card className="w-full max-w-xl border-dashed bg-muted/15">
              <CardHeader className="space-y-4 text-center">
                <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-muted">
                  <ShieldAlert className="h-6 w-6 text-muted-foreground" />
                </div>
                <CardTitle className="text-xl md:text-2xl">
                  {t.auth.guestAccessCheckFailedTitle}
                </CardTitle>
                <CardDescription className="text-sm leading-6 md:text-base">
                  {t.auth.guestAccessCheckFailedDesc}
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <PluginSlot
                  slot="user.layout.guest_access_check_failed"
                  context={{ ...guestLayoutPluginContext, section: 'access_state' }}
                />
                <div className="flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:justify-center">
                  <Button className="sm:min-w-40" onClick={handleRetryGuestAccess}>
                    <RefreshCw className="mr-2 h-4 w-4" />
                    {t.common.refresh}
                  </Button>
                  <Button
                    variant="outline"
                    className="sm:min-w-40"
                    onClick={() => goToAuth('/login')}
                  >
                    <LogIn className="mr-2 h-4 w-4" />
                    {t.auth.loginToContinue}
                  </Button>
                  {pathname !== '/products' && (
                    <Button variant="ghost" className="sm:min-w-40" asChild>
                      <Link href="/products">
                        <ShoppingBag className="mr-2 h-4 w-4" />
                        {t.product.backToProductList}
                      </Link>
                    </Button>
                  )}
                </div>
              </CardContent>
            </Card>
          </div>
        </UserLayoutFrame>
      )
    }

    if (allowGuestAccess) {
      const guestLayout = (
        <UserLayoutFrame guestMode isMobile={isMobile} slotContext={guestLayoutPluginContext}>
          {children}
        </UserLayoutFrame>
      )

      if (isCartRoute) {
        return <CartProvider>{guestLayout}</CartProvider>
      }

      return guestLayout
    }

    return (
      <UserLayoutFrame guestMode isMobile={isMobile} slotContext={guestLayoutPluginContext}>
        <div className="mx-auto flex min-h-[calc(100vh-10rem)] max-w-2xl items-center justify-center">
          <Card className="w-full max-w-xl">
            <CardHeader className="space-y-4 text-center">
              <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-muted">
                <ShieldAlert className="h-6 w-6 text-muted-foreground" />
              </div>
              <CardTitle className="text-xl md:text-2xl">{t.auth.loginRequiredTitle}</CardTitle>
              <CardDescription className="text-sm leading-6 md:text-base">
                {t.auth.loginRequiredDesc}
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <PluginSlot
                slot="user.layout.login_required"
                context={{ ...guestLayoutPluginContext, section: 'access_state' }}
              />
              <div className="flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:justify-center">
                <Button className="sm:min-w-40" onClick={() => goToAuth('/login')}>
                  <LogIn className="mr-2 h-4 w-4" />
                  {t.auth.loginToContinue}
                </Button>
                <Button variant="ghost" className="sm:min-w-40" asChild>
                  <Link href="/products">
                    <ShoppingBag className="mr-2 h-4 w-4" />
                    {t.product.backToProductList}
                  </Link>
                </Button>
              </div>
            </CardContent>
          </Card>
        </div>
      </UserLayoutFrame>
    )
  }

  return (
    <CartProvider>
      <UserLayoutFrame isMobile={isMobile} slotContext={authenticatedLayoutPluginContext}>
        {children}
      </UserLayoutFrame>
      <AnnouncementPopup />
    </CartProvider>
  )
}

export default function UserLayout({ children }: { children: React.ReactNode }) {
  return (
    <Suspense fallback={<UserLayoutFallback />}>
      <UserLayoutContent>{children}</UserLayoutContent>
    </Suspense>
  )
}
