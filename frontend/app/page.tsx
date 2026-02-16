'use client'

import { useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { Loader2 } from 'lucide-react'
import { getToken } from '@/lib/auth'

export default function HomePage() {
  const router = useRouter()

  useEffect(() => {
    // 检查登录状态
    const token = getToken()
    
    if (token) {
      // 已登录，重定向到商品页
      router.replace('/products')
    } else {
      // 未登录，重定向到登录页
      router.replace('/login')
    }
  }, [router])

  // 显示加载状态
  return (
    <div className="min-h-screen flex items-center justify-center">
      <div className="text-center">
        <Loader2 className="h-8 w-8 animate-spin mx-auto mb-4 text-primary" />
        <p className="text-muted-foreground">Loading...</p>
      </div>
    </div>
  )
}

