'use client'

import { Suspense, useEffect, useMemo, useState } from 'react'
import { usePathname, useRouter } from 'next/navigation'
import { useAuth } from '@/hooks/use-auth'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePreventScrollLock } from '@/hooks/use-prevent-scroll-lock'
import { PluginSlot } from '@/components/plugins/plugin-slot'
import { Sidebar } from '@/components/layout/sidebar'
import { getToken } from '@/lib/auth'

function AdminSidebarFallback() {
  return <div className="w-64 shrink-0 border-r bg-card" />
}

function AdminLayoutFrame({
  children,
  slotContext,
}: {
  children: React.ReactNode
  slotContext?: Record<string, unknown>
}) {
  return (
    <div className="flex h-screen overflow-hidden sidebar-layout">
      <Suspense fallback={<AdminSidebarFallback />}>
        <Sidebar />
      </Suspense>
      <main className="flex-1 overflow-y-auto bg-background p-8">
        {slotContext ? (
          <Suspense fallback={null}>
            <PluginSlot slot="admin.layout.content.top" context={slotContext} />
          </Suspense>
        ) : null}
        {children}
        {slotContext ? (
          <Suspense fallback={null}>
            <PluginSlot slot="admin.layout.content.bottom" context={slotContext} />
          </Suspense>
        ) : null}
      </main>
    </div>
  )
}

export default function AdminLayout({ children }: { children: React.ReactNode }) {
  const { user, isLoading, isAuthenticated } = useAuth()
  const { locale, mounted: localeMounted } = useLocale()
  const router = useRouter()
  const pathname = usePathname()
  const [mounted, setMounted] = useState(false)
  const hasToken = getToken()

  // 防止 Radix UI Dialog/Select 锁定页面滚动
  usePreventScrollLock()

  useEffect(() => {
    setMounted(true)
  }, [])

  useEffect(() => {
    if (mounted && !isLoading) {
      if (!isAuthenticated || !user) {
        router.push('/login')
      } else if (user.role !== 'admin' && user.role !== 'super_admin') {
        router.push('/orders')
      }
    }
  }, [mounted, user, isLoading, isAuthenticated, router])

  const loadingText = localeMounted ? getTranslations(locale).common.loading : '...'
  const verifyingText = localeMounted ? getTranslations(locale).common.verifying : '...'
  const noAccessText = localeMounted ? getTranslations(locale).common.noAccess : '...'
  const adminLayoutPluginContext = useMemo(
    () => ({
      view: 'admin_layout_content',
      layout: {
        current_path: pathname || '/admin',
        locale,
      },
      auth: {
        is_loading: isLoading,
        is_authenticated: isAuthenticated,
        has_token: !!hasToken,
      },
      admin: user
        ? {
            user_id: user.id,
            role: user.role,
            is_super_admin: user.role === 'super_admin',
          }
        : null,
    }),
    [hasToken, isAuthenticated, isLoading, locale, pathname, user]
  )

  // 在挂载前，显示完整布局（避免 hydration 错误）
  if (!mounted) {
    return (
      <AdminLayoutFrame slotContext={adminLayoutPluginContext}>
          <div className="flex items-center justify-center h-full">
            <div className="text-center">{loadingText}</div>
          </div>
      </AdminLayoutFrame>
    )
  }

  // 加载中或检查权限中
  if (isLoading) {
    return (
      <AdminLayoutFrame slotContext={adminLayoutPluginContext}>
          <div className="flex items-center justify-center h-full">
            <div className="text-center">{loadingText}</div>
          </div>
      </AdminLayoutFrame>
    )
  }

  // 如果有 token 但用户数据还未加载，或者用户没有权限，显示加载（等待跳转）
  if (!user || !hasToken) {
    return (
      <AdminLayoutFrame slotContext={adminLayoutPluginContext}>
          <div className="flex items-center justify-center h-full">
            <div className="text-center">{verifyingText}</div>
          </div>
      </AdminLayoutFrame>
    )
  }

  // 检查是否为管理员
  if (user.role !== 'admin' && user.role !== 'super_admin') {
    return (
      <AdminLayoutFrame slotContext={adminLayoutPluginContext}>
          <div className="flex items-center justify-center h-full">
            <div className="text-center">{noAccessText}</div>
          </div>
      </AdminLayoutFrame>
    )
  }

  // 通过所有检查，显示内容
  return (
    <AdminLayoutFrame slotContext={adminLayoutPluginContext}>{children}</AdminLayoutFrame>
  )
}

