'use client'

import { useEffect, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getPublicConfig } from '@/lib/api'

type BrandingCache = {
  app_name?: string
  logo_url?: string
}

function readBrandingCache(): BrandingCache | null {
  if (typeof window === 'undefined') {
    return null
  }

  try {
    const appName = window.localStorage.getItem('auralogic_app_name') || undefined
    const raw = window.localStorage.getItem('auth_branding_cache')
    let logoUrl: string | undefined
    let cachedAppName = appName

    if (raw) {
      const parsed = JSON.parse(raw)
      if (parsed && typeof parsed === 'object') {
        if (typeof parsed.logo_url === 'string' && parsed.logo_url.trim()) {
          logoUrl = parsed.logo_url
        }
        if (!cachedAppName && typeof parsed.app_name === 'string' && parsed.app_name.trim()) {
          cachedAppName = parsed.app_name
        }
      }
    }

    if (!cachedAppName && !logoUrl) {
      return null
    }

    return {
      app_name: cachedAppName,
      logo_url: logoUrl,
    }
  } catch {
    return null
  }
}

export function usePublicBranding() {
  const [cached, setCached] = useState<BrandingCache | null>(null)

  useEffect(() => {
    setCached(readBrandingCache())
  }, [])

  const { data } = useQuery({
    queryKey: ['publicConfig'],
    queryFn: getPublicConfig,
    staleTime: 5 * 60 * 1000,
  })

  return {
    appName: data?.data?.app_name || cached?.app_name || 'AuraLogic',
    logoUrl: data?.data?.customization?.logo_url || cached?.logo_url || '',
  }
}
