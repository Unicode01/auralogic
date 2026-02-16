'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import { useAuth } from '@/hooks/use-auth'
import { useLocale } from '@/hooks/use-locale'
import { usePreventScrollLock } from '@/hooks/use-prevent-scroll-lock'
import { Sidebar } from '@/components/layout/sidebar'
import { getToken } from '@/lib/auth'

export default function AdminLayout({ children }: { children: React.ReactNode }) {
  const { user, isLoading, isAuthenticated } = useAuth()
  const { locale, mounted: localeMounted } = useLocale()
  const router = useRouter()
  const [mounted, setMounted] = useState(false)

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

  const loadingText = localeMounted ? (locale === 'zh' ? '加载中...' : 'Loading...') : '...'
  const verifyingText = localeMounted ? (locale === 'zh' ? '验证权限...' : 'Verifying...') : '...'
  const noAccessText = localeMounted ? (locale === 'zh' ? '无权限访问...' : 'No access...') : '...'

  // 在挂载前，显示完整布局（避免 hydration 错误）
  if (!mounted) {
    return (
      <div className="flex h-screen overflow-hidden sidebar-layout">
        <Sidebar />
        <main className="flex-1 overflow-y-auto p-8 bg-background">
          <div className="flex items-center justify-center h-full">
            <div className="text-center">{loadingText}</div>
          </div>
        </main>
      </div>
    )
  }

  // 加载中或检查权限中
  if (isLoading) {
    return (
      <div className="flex h-screen overflow-hidden sidebar-layout">
        <Sidebar />
        <main className="flex-1 overflow-y-auto p-8 bg-background">
          <div className="flex items-center justify-center h-full">
            <div className="text-center">{loadingText}</div>
          </div>
        </main>
      </div>
    )
  }

  // 如果有 token 但用户数据还未加载，或者用户没有权限，显示加载（等待跳转）
  const hasToken = getToken()
  if (!user || !hasToken) {
    return (
      <div className="flex h-screen overflow-hidden sidebar-layout">
        <Sidebar />
        <main className="flex-1 overflow-y-auto p-8 bg-background">
          <div className="flex items-center justify-center h-full">
            <div className="text-center">{verifyingText}</div>
          </div>
        </main>
      </div>
    )
  }

  // 检查是否为管理员
  if (user.role !== 'admin' && user.role !== 'super_admin') {
    return (
      <div className="flex h-screen overflow-hidden sidebar-layout">
        <Sidebar />
        <main className="flex-1 overflow-y-auto p-8 bg-background">
          <div className="flex items-center justify-center h-full">
            <div className="text-center">{noAccessText}</div>
          </div>
        </main>
      </div>
    )
  }

  // 通过所有检查，显示内容
  return (
    <div className="flex h-screen overflow-hidden sidebar-layout">
      <Sidebar />
      <main className="flex-1 overflow-y-auto p-8 bg-background">{children}</main>
    </div>
  )
}

