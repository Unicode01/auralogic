'use client'

import { useEffect, useLayoutEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getPublicConfig } from '@/lib/api'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { PluginSlot } from '@/components/plugins/plugin-slot'
import { sanitizeAuthBrandingHtml } from '@/lib/auth-branding-html'

const CACHE_KEY = 'auth_branding_cache'

type AuthBrandingCache = {
  app_name?: string
  primary_color?: string
  auth_branding?: {
    mode?: string
    title?: string
    title_en?: string
    subtitle?: string
    subtitle_en?: string
    custom_html?: string
  } | null
}

function readAuthBrandingCache(): AuthBrandingCache | null {
  if (typeof window === 'undefined') {
    return null
  }

  const preload = (window as Window & { __AUTH_BRAND__?: AuthBrandingCache | null }).__AUTH_BRAND__
  if (preload && typeof preload === 'object') {
    return preload
  }

  try {
    const raw = window.localStorage.getItem(CACHE_KEY)
    if (!raw) {
      return null
    }
    const parsed = JSON.parse(raw)
    if (parsed && typeof parsed === 'object') {
      return parsed as AuthBrandingCache
    }
  } catch {}

  return null
}

export function AuthBrandingPanel() {
  const { locale } = useLocale()
  const t = getTranslations(locale)
  const [cachedBranding, setCachedBranding] = useState<AuthBrandingCache | null>(null)

  useLayoutEffect(() => {
    const initialCache = readAuthBrandingCache()
    if (initialCache) {
      setCachedBranding(initialCache)
    }
  }, [])

  const { data: publicConfig } = useQuery({
    queryKey: ['publicConfig'],
    queryFn: getPublicConfig,
  })

  // Update cache when network data arrives
  useEffect(() => {
    if (!publicConfig?.data) return
    const d = publicConfig.data
    const val: AuthBrandingCache = {
      app_name: d.app_name,
      primary_color: d.customization?.primary_color,
      auth_branding: d.customization?.auth_branding,
    }
    setCachedBranding(val)
    if (typeof window !== 'undefined') {
      ;(window as Window & { __AUTH_BRAND__?: AuthBrandingCache | null }).__AUTH_BRAND__ = val
      window.localStorage.setItem(CACHE_KEY, JSON.stringify(val))
    }
  }, [publicConfig])

  const net = publicConfig?.data
  const cached = cachedBranding
  const appName = net?.app_name || cached?.app_name || 'AuraLogic'
  const primaryColor = net?.customization?.primary_color || cached?.primary_color
  const branding = net?.customization?.auth_branding || cached?.auth_branding
  const bgStyle = primaryColor ? { backgroundColor: `hsl(${primaryColor})` } : undefined
  const safeCustomHtml = useMemo(() => {
    if (branding?.mode !== 'custom' || !branding.custom_html) {
      return ''
    }
    return sanitizeAuthBrandingHtml(branding.custom_html)
  }, [branding?.custom_html, branding?.mode])
  const authBrandingPluginContext = useMemo(
    () => ({
      view: 'auth_branding_panel',
      locale,
      branding: {
        mode: branding?.mode || 'default',
        app_name: appName,
        primary_color: primaryColor || null,
        has_custom_html: Boolean(branding?.custom_html),
      },
    }),
    [appName, branding?.custom_html, branding?.mode, locale, primaryColor]
  )

  // Custom HTML mode
  if (branding?.mode === 'custom' && safeCustomHtml) {
    return (
      <div
        className="hidden lg:flex lg:w-1/2 relative overflow-hidden bg-primary"
        style={bgStyle}
        dangerouslySetInnerHTML={{ __html: safeCustomHtml }}
      />
    )
  }

  // Default mode
  const title = (locale === 'zh' ? branding?.title : branding?.title_en) || t.home.defaultBrandingTitle
  const subtitle = (locale === 'zh' ? branding?.subtitle : branding?.subtitle_en) || t.home.subtitle

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
        <div className="space-y-6">
          <PluginSlot slot="auth.layout.branding.top" context={authBrandingPluginContext} />
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
        <div className="space-y-4">
          <PluginSlot slot="auth.layout.branding.bottom" context={authBrandingPluginContext} />
          <p className="text-primary-foreground/50 text-sm">
            &copy; {new Date().getFullYear()} {appName}
          </p>
        </div>
      </div>
    </div>
  )
}
