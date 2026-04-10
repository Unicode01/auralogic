'use client'

import { Suspense, useCallback, useEffect, useState } from 'react'
import { useSearchParams } from 'next/navigation'
import { getFormInfo } from '@/lib/api'
import { ShippingForm } from '@/components/forms/shipping-form'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Globe } from 'lucide-react'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { PageLoading } from '@/components/ui/page-loading'
import { usePageTitle } from '@/hooks/use-page-title'
import { PluginSlot } from '@/components/plugins/plugin-slot'
import { isAuthenticated } from '@/lib/auth'

function ShippingFormContent() {
  const searchParams = useSearchParams()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.shippingForm)
  const token = searchParams.get('token')
  const [formData, setFormData] = useState<any>(null)
  const [errorType, setErrorType] = useState<'missingToken' | 'loadFailed' | null>(null)
  const [errorDetail, setErrorDetail] = useState('')
  const [isLoading, setIsLoading] = useState(true)

  // 初始化语言状态（只在首次渲染时执行）
  const [lang, setLang] = useState<string>(() => {
    // 服务端渲染时返回默认值
    if (typeof window === 'undefined') {
      return 'zh'
    }
    return 'zh' // 客户端也先用默认值，在 useEffect 中再更新
  })
  const activeLocale = lang === 'en' ? 'en' : 'zh'
  const activeT = getTranslations(activeLocale)

  // 初始化语言：从 URL 读取或自动检测（只在组件挂载时执行一次）
  useEffect(() => {
    const urlLang = searchParams.get('lang')
    let nextLang: 'zh' | 'en'
    if (urlLang && (urlLang === 'zh' || urlLang === 'en')) {
      nextLang = urlLang
    } else if (typeof window !== 'undefined') {
      // 优先读取用户已保存的语言偏好，其次检测浏览器语言
      const storedLocale = localStorage.getItem('auralogic_locale')
      if (storedLocale === 'zh' || storedLocale === 'en') {
        nextLang = storedLocale
      } else {
        const browserLang = navigator.language.toLowerCase()
        nextLang = browserLang.startsWith('zh') ? 'zh' : 'en'
      }
    } else {
      nextLang = 'zh'
    }

    setLang(nextLang)

    if (typeof window !== 'undefined') {
      const url = new URL(window.location.href)
      if (url.searchParams.get('lang') !== nextLang) {
        url.searchParams.set('lang', nextLang)
        window.history.replaceState({}, '', url.toString())
      }
    }
  }, [searchParams])

  const toggleLanguage = () => {
    const newLang = lang === 'zh' ? 'en' : 'zh'
    setLang(newLang)
    const url = new URL(window.location.href)
    url.searchParams.set('lang', newLang)
    window.history.replaceState({}, '', url.toString())
  }

  const loadFormInfo = useCallback(() => {
    if (!token) {
      setFormData(null)
      setErrorType('missingToken')
      setErrorDetail('')
      setIsLoading(false)
      return
    }

    setIsLoading(true)
    setErrorType(null)
    setErrorDetail('')

    getFormInfo(token)
      .then((res) => {
        setFormData(res.data)
        setIsLoading(false)
      })
      .catch((err) => {
        setFormData(null)
        setErrorType('loadFailed')
        setErrorDetail(err?.message || '')
        setIsLoading(false)
      })
  }, [token])

  useEffect(() => {
    loadFormInfo()
  }, [loadFormInfo])
  const hasAuthToken = typeof window !== 'undefined' && isAuthenticated()
  const publicShippingFormPluginContext = {
    view: 'public_shipping_form',
    locale: activeLocale,
    shipping_form: {
      token_present: Boolean(token),
      is_loading: isLoading,
      error_type: errorType || undefined,
      has_form_data: Boolean(formData),
      hide_password: hasAuthToken,
    },
    order: formData
      ? {
          order_no: formData.orderNo || formData.order_no || undefined,
          item_count: Array.isArray(formData.items) ? formData.items.length : 0,
        }
      : undefined,
    state: {
      loading: isLoading,
      load_failed: Boolean(errorType),
      missing_token: errorType === 'missingToken',
      ready: !isLoading && !errorType && Boolean(formData),
    },
  }

  if (isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center px-4">
        <div className="w-full max-w-md space-y-4">
          <PluginSlot slot="public.shipping_form.top" context={publicShippingFormPluginContext} />
          <div className="flex justify-center">
            <PageLoading text={activeT.common.loading} />
          </div>
          <PluginSlot
            slot="public.shipping_form.bottom"
            context={publicShippingFormPluginContext}
          />
        </div>
      </div>
    )
  }

  const errorMessage =
    errorType === 'missingToken'
      ? activeT.shippingPublic.missingFormToken
      : errorDetail || activeT.shippingPublic.formLoadFailed

  if (errorType) {
    return (
      <div className="flex min-h-screen items-center justify-center px-4">
        <div className="w-full max-w-md space-y-4">
          <PluginSlot slot="public.shipping_form.top" context={publicShippingFormPluginContext} />
          <Card>
            <CardHeader>
              <CardTitle>{activeT.common.error}</CardTitle>
              <CardDescription>{activeT.shippingPublic.formLoadFailed}</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <p className="rounded-lg border border-destructive/20 bg-destructive/5 px-3 py-2 text-sm text-destructive">
                {errorMessage}
              </p>
              {token && (
                <Button variant="outline" className="w-full" onClick={loadFormInfo}>
                  {activeT.common.refresh}
                </Button>
              )}
            </CardContent>
          </Card>
          <PluginSlot
            slot="public.shipping_form.load_failed"
            context={{ ...publicShippingFormPluginContext, section: 'form_state' }}
          />
          <PluginSlot
            slot="public.shipping_form.bottom"
            context={publicShippingFormPluginContext}
          />
        </div>
      </div>
    )
  }

  if (!formData) {
    return null
  }

  return (
    <div className="relative min-h-screen bg-muted/50 py-12">
      <div className="mx-auto flex max-w-5xl flex-col gap-6 px-4">
        <PluginSlot slot="public.shipping_form.top" context={publicShippingFormPluginContext} />
      </div>

      {/* 悬浮的语言切换按钮 */}
      <Button
        variant="outline"
        size="sm"
        onClick={toggleLanguage}
        className="fixed right-4 top-4 z-50 gap-2 bg-background shadow-lg transition-shadow hover:shadow-xl"
      >
        <Globe className="h-4 w-4" />
        {lang === 'zh' ? activeT.language.en : activeT.language.zh}
      </Button>

      <ShippingForm
        formToken={token!}
        orderInfo={formData}
        lang={lang}
        hidePassword={hasAuthToken}
        pluginSlotNamespace="public.shipping_form"
        pluginSlotContext={publicShippingFormPluginContext}
      />

      <div className="mx-auto flex max-w-5xl flex-col gap-6 px-4">
        <PluginSlot
          slot="public.shipping_form.bottom"
          context={publicShippingFormPluginContext}
        />
      </div>
    </div>
  )
}

export default function ShippingFormPage() {
  return (
    <Suspense
      fallback={
        <div className="flex min-h-screen items-center justify-center">
          <PageLoading />
        </div>
      }
    >
      <ShippingFormContent />
    </Suspense>
  )
}
