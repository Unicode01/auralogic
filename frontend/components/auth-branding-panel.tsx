'use client'

import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getPublicConfig } from '@/lib/api'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'

const CACHE_KEY = 'auth_branding_cache'

// Read cache from window (set by head script) — fastest possible access
let _initialCache: any = null
if (typeof window !== 'undefined') {
  _initialCache = (window as any).__AUTH_BRAND__ || null
  if (!_initialCache) {
    try {
      const raw = localStorage.getItem(CACHE_KEY)
      if (raw) _initialCache = JSON.parse(raw)
    } catch {}
  }
}

const LoadingSkeleton = (
  <div className="hidden lg:flex lg:w-1/2 relative overflow-hidden bg-primary">
    <div className="relative z-10 flex flex-col justify-between w-full p-12 animate-pulse">
      <div className="h-8 w-32 rounded bg-white/10" />
      <div className="space-y-4">
        <div className="h-10 w-64 rounded bg-white/10" />
        <div className="h-10 w-48 rounded bg-white/10" />
        <div className="h-5 w-72 rounded bg-white/10" />
      </div>
      <div className="h-4 w-36 rounded bg-white/10" />
    </div>
  </div>
)

export function AuthBrandingPanel() {
  const { locale } = useLocale()
  const t = getTranslations(locale)
  const cacheRef = useRef<any>(_initialCache)
  const [mounted, setMounted] = useState(false)

  // Module-level cache already loaded into ref — just flip the boolean before paint
  useLayoutEffect(() => { setMounted(true) }, [])

  const { data: publicConfig } = useQuery({
    queryKey: ['publicConfig'],
    queryFn: getPublicConfig,
  })

  // Update cache when network data arrives
  useEffect(() => {
    if (!publicConfig?.data) return
    const d = publicConfig.data
    const val = {
      app_name: d.app_name,
      primary_color: d.customization?.primary_color,
      auth_branding: d.customization?.auth_branding,
    }
    cacheRef.current = val
    localStorage.setItem(CACHE_KEY, JSON.stringify(val))
  }, [publicConfig])

  // SSR or first visit (no cache): gray loading skeleton
  if (!mounted) return LoadingSkeleton

  const net = publicConfig?.data
  const cached = cacheRef.current
  const appName = net?.app_name || cached?.app_name || 'AuraLogic'
  const primaryColor = net?.customization?.primary_color || cached?.primary_color
  const branding = net?.customization?.auth_branding || cached?.auth_branding
  const bgStyle = primaryColor ? { backgroundColor: `hsl(${primaryColor})` } : undefined

  // No cache + no network yet: keep showing skeleton
  if (!net && !cached) return LoadingSkeleton

  // Custom HTML mode
  if (branding?.mode === 'custom' && branding.custom_html) {
    return (
      <div
        className="hidden lg:flex lg:w-1/2 relative overflow-hidden bg-primary"
        style={bgStyle}
        dangerouslySetInnerHTML={{ __html: branding.custom_html }}
      />
    )
  }

  // Default mode
  const title = locale === 'zh'
    ? (branding?.title || '现代化电商\n管理平台')
    : (branding?.title_en || 'Modern\nE-commerce\nPlatform')
  const subtitle = locale === 'zh'
    ? (branding?.subtitle || t.home.subtitle)
    : (branding?.subtitle_en || t.home.subtitle)

  return (
    <div className="hidden lg:flex lg:w-1/2 relative overflow-hidden bg-primary" style={bgStyle}>
      <div className="absolute inset-0 opacity-10">
        <div className="absolute top-0 left-0 w-full h-full"
          style={{
            backgroundImage: `radial-gradient(circle at 25% 25%, rgba(255,255,255,0.2) 0%, transparent 50%),
                             radial-gradient(circle at 75% 75%, rgba(255,255,255,0.15) 0%, transparent 50%)`,
          }}
        />
        <div className="absolute -top-24 -left-24 w-96 h-96 rounded-full border border-white/20" />
        <div className="absolute -bottom-32 -right-32 w-[500px] h-[500px] rounded-full border border-white/10" />
        <div className="absolute top-1/2 left-1/4 w-64 h-64 rounded-full border border-white/10" />
      </div>
      <div className="relative z-10 flex flex-col justify-between w-full p-12">
        <div>
          <h1 className="text-3xl font-bold text-primary-foreground tracking-tight">
            {appName}
          </h1>
        </div>
        <div className="space-y-6">
          <h2 className="text-4xl font-bold text-primary-foreground leading-tight">
            {title}
          </h2>
          <p className="text-primary-foreground/70 text-lg max-w-md leading-relaxed">
            {subtitle}
          </p>
        </div>
        <p className="text-primary-foreground/50 text-sm">
          &copy; {new Date().getFullYear()} {appName}
        </p>
      </div>
    </div>
  )
}
