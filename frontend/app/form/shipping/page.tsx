'use client'

import { Suspense, useEffect, useState } from 'react'
import { useSearchParams, useRouter } from 'next/navigation'
import { getFormInfo } from '@/lib/api'
import { ShippingForm } from '@/components/forms/shipping-form'
import { Button } from '@/components/ui/button'
import { Globe } from 'lucide-react'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'
import { getToken } from '@/lib/auth'

function ShippingFormContent() {
  const searchParams = useSearchParams()
  const router = useRouter()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.shippingForm)
  const token = searchParams.get('token')
  const [formData, setFormData] = useState<any>(null)
  const [error, setError] = useState('')
  const [isLoading, setIsLoading] = useState(true)

  // 初始化语言状态（只在首次渲染时执行）
  const [lang, setLang] = useState<string>(() => {
    // 服务端渲染时返回默认值
    if (typeof window === 'undefined') {
      return 'zh'
    }
    return 'zh' // 客户端也先用默认值，在 useEffect 中再更新
  })

  // 初始化语言：从 URL 读取或自动检测（只在组件挂载时执行一次）
  useEffect(() => {
    const urlLang = searchParams.get('lang')
    if (urlLang && (urlLang === 'zh' || urlLang === 'en')) {
      setLang(urlLang)
    } else if (typeof window !== 'undefined') {
      // 优先读取用户已保存的语言偏好，其次检测浏览器语言
      const storedLocale = localStorage.getItem('auralogic_locale')
      let detectedLang: string
      if (storedLocale === 'zh' || storedLocale === 'en') {
        detectedLang = storedLocale
      } else {
        const browserLang = navigator.language.toLowerCase()
        detectedLang = browserLang.startsWith('zh') ? 'zh' : 'en'
      }
      // 更新 URL 并刷新页面
      const url = new URL(window.location.href)
      url.searchParams.set('lang', detectedLang)
      window.location.href = url.toString()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []) // 只在挂载时执行一次，忽略依赖检查

  // 切换语言 - 使用页面刷新确保状态一致性
  const toggleLanguage = () => {
    const newLang = lang === 'zh' ? 'en' : 'zh'
    const url = new URL(window.location.href)
    url.searchParams.set('lang', newLang)
    // 使用 window.location.href 触发页面刷新
    window.location.href = url.toString()
  }

  useEffect(() => {
    if (!token) {
      setError(lang === 'zh' ? '缺少表单令牌' : 'Missing form token')
      setIsLoading(false)
      return
    }

    getFormInfo(token)
      .then((res) => {
        setFormData(res.data)
        setIsLoading(false)
      })
      .catch((err) => {
        setError(err.message || (lang === 'zh' ? '表单加载失败' : 'Failed to load form'))
        setIsLoading(false)
      })
  }, [token]) // 移除 lang 依赖，避免切换语言时重新加载数据

  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div>{lang === 'zh' ? '加载中...' : 'Loading...'}</div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="text-center">
          <p className="text-red-600">{error}</p>
        </div>
      </div>
    )
  }

  if (!formData) {
    return null
  }

  return (
    <div className="min-h-screen bg-muted/50 py-12 relative">
      {/* 悬浮的语言切换按钮 */}
      <Button
        variant="outline"
        size="sm"
        onClick={toggleLanguage}
        className="fixed top-4 right-4 z-50 gap-2 shadow-lg hover:shadow-xl transition-shadow bg-background"
      >
        <Globe className="h-4 w-4" />
        {lang === 'zh' ? 'English' : '中文'}
      </Button>

      <ShippingForm formToken={token!} orderInfo={formData} lang={lang} hidePassword={typeof window !== 'undefined' && !!getToken()} />
    </div>
  )
}

export default function ShippingFormPage() {
  return (
    <Suspense fallback={
      <div className="min-h-screen flex items-center justify-center">
        <div>加载中...</div>
      </div>
    }>
      <ShippingFormContent />
    </Suspense>
  )
}

