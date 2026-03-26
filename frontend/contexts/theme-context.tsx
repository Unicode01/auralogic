'use client'

import { createContext, useContext, useEffect, useState, useRef, useCallback, ReactNode } from 'react'
import { useQuery } from '@tanstack/react-query'
import { usePathname } from 'next/navigation'
import { getPublicConfig, getPageInject } from '@/lib/api'

export type Theme = 'light' | 'dark' | 'system'

interface ThemeContextType {
  theme: Theme
  setTheme: (theme: Theme) => void
  resolvedTheme: 'light' | 'dark'
}

const ThemeContext = createContext<ThemeContextType | undefined>(undefined)

const THEME_STORAGE_KEY = 'auralogic-theme'
const PAGE_INJECT_CACHE_KEY = 'auralogic-page-inject'
const PAGE_INJECT_TTL = 5 * 60 * 1000 // 5分钟

interface PageInjectCache {
  [path: string]: {
    css: string
    js: string
    rules?: PageInjectRule[]
    ts: number // 缓存时间戳
  }
}

interface PageInjectRule {
  name?: string
  pattern?: string
  match_type?: string
  css?: string
  js?: string
}

interface PageInjectPayload {
  css: string
  js: string
  rules: PageInjectRule[]
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<Theme>('system')
  const [resolvedTheme, setResolvedTheme] = useState<'light' | 'dark'>('light')
  const [mounted, setMounted] = useState(false)
  const pathname = usePathname()
  const pageInjectIdsRef = useRef<string[]>([])
  const lastInjectPathRef = useRef<string>('')

  // 获取系统默认主题配置
  const { data: publicConfig } = useQuery({
    queryKey: ['publicConfig'],
    queryFn: getPublicConfig,
    staleTime: 1000 * 60 * 5, // 5分钟
  })

  // 获取系统偏好
  const getSystemTheme = (): 'light' | 'dark' => {
    if (typeof window !== 'undefined') {
      return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
    }
    return 'light'
  }

  // 客户端挂载，不依赖 publicConfig
  useEffect(() => {
    setMounted(true)
    const savedTheme = localStorage.getItem(THEME_STORAGE_KEY) as Theme | null
    if (savedTheme) {
      setThemeState(savedTheme)
    }
  }, [])

  // publicConfig 加载完成后，如果用户没有手动设置过主题，应用服务端默认主题
  useEffect(() => {
    if (!publicConfig?.data) return
    // 缓存 app_name 供 usePageTitle 使用
    if (publicConfig.data.app_name) {
      localStorage.setItem('auralogic_app_name', publicConfig.data.app_name)
      ;(window as any).__APP_NAME__ = publicConfig.data.app_name
    }
    if (!publicConfig.data.default_theme) return
    const savedTheme = localStorage.getItem(THEME_STORAGE_KEY) as Theme | null
    if (!savedTheme) {
      setThemeState(publicConfig.data.default_theme)
    }
  }, [publicConfig])

  // 应用个性化配置（主色调、favicon）
  useEffect(() => {
    if (!mounted || !publicConfig?.data?.customization) return
    const customization = publicConfig.data.customization

    // 应用主色调
    const root = document.documentElement
    if (customization.primary_color) {
      root.style.setProperty('--primary', customization.primary_color)
      root.style.setProperty('--ring', customization.primary_color)
      localStorage.setItem('auralogic_primary_color', customization.primary_color)
    }

    // 更新favicon
    if (customization.favicon_url) {
      const faviconEl = document.querySelector('link[rel="icon"]') as HTMLLinkElement | null
      if (faviconEl) {
        faviconEl.href = customization.favicon_url
      }
    }
  }, [mounted, publicConfig])

  // 清除当前页面注入的元素
  const clearPageInject = useCallback(() => {
    pageInjectIdsRef.current.forEach(id => {
      document.getElementById(id)?.remove()
    })
    pageInjectIdsRef.current = []
  }, [])

  const normalizePageInjectPayload = useCallback((raw: any): PageInjectPayload => {
    const css = typeof raw?.css === 'string' ? raw.css : ''
    const js = typeof raw?.js === 'string' ? raw.js : ''
    const rules = Array.isArray(raw?.rules)
      ? raw.rules
        .filter((item: any) => item && typeof item === 'object')
        .map((item: any) => ({
          name: typeof item.name === 'string' ? item.name : undefined,
          pattern: typeof item.pattern === 'string' ? item.pattern : undefined,
          match_type: typeof item.match_type === 'string' ? item.match_type : undefined,
          css: typeof item.css === 'string' ? item.css : '',
          js: typeof item.js === 'string' ? item.js : '',
        }))
      : []
    return { css, js, rules }
  }, [])

  // 注入CSS/JS到页面
  const applyPageInject = useCallback((payload: PageInjectPayload) => {
    clearPageInject()

    const { css, js, rules } = payload
    const hasRuleBlocks = Array.isArray(rules) && rules.length > 0

    if (hasRuleBlocks) {
      rules.forEach((rule, index) => {
        if (rule.css) {
          const styleEl = document.createElement('style')
          styleEl.id = `auralogic-page-inject-css-${index}`
          styleEl.textContent = rule.css
          document.head.appendChild(styleEl)
          pageInjectIdsRef.current.push(styleEl.id)
        }
        if (rule.js) {
          const scriptEl = document.createElement('script')
          scriptEl.id = `auralogic-page-inject-js-${index}`
          scriptEl.textContent = rule.js
          document.body.appendChild(scriptEl)
          pageInjectIdsRef.current.push(scriptEl.id)
        }
      })
      return
    }

    if (css) {
      const styleEl = document.createElement('style')
      styleEl.id = 'auralogic-page-inject-css'
      styleEl.textContent = css
      document.head.appendChild(styleEl)
      pageInjectIdsRef.current.push(styleEl.id)
    }

    if (js) {
      const scriptEl = document.createElement('script')
      scriptEl.id = 'auralogic-page-inject-js'
      scriptEl.textContent = js
      document.body.appendChild(scriptEl)
      pageInjectIdsRef.current.push(scriptEl.id)
    }
  }, [clearPageInject])

  // 页面切换时获取并应用定向注入
  useEffect(() => {
    if (!mounted) return
    // 避免同一路径重复执行
    if (lastInjectPathRef.current === pathname) return
    lastInjectPathRef.current = pathname

    // 读取 localStorage 缓存
    let cache: PageInjectCache = {}
    try {
      const raw = localStorage.getItem(PAGE_INJECT_CACHE_KEY)
      if (raw) cache = JSON.parse(raw)
    } catch {
      // 忽略解析错误
    }

    const cached = cache[pathname]
    const now = Date.now()

    if (cached && (now - cached.ts) < PAGE_INJECT_TTL) {
      // 缓存有效，直接应用
      const cachedPayload = normalizePageInjectPayload(cached)
      applyPageInject(cachedPayload)
      return
    }

    // 缓存不存在或已过期，从API获取
    clearPageInject()
    getPageInject(pathname).then((res: any) => {
      const payload = normalizePageInjectPayload(res?.data)

      // 写入缓存
      cache[pathname] = { ...payload, ts: Date.now() }
      // 清理过期缓存条目
      for (const key in cache) {
        if (Date.now() - cache[key].ts > PAGE_INJECT_TTL) {
          delete cache[key]
        }
      }
      try {
        localStorage.setItem(PAGE_INJECT_CACHE_KEY, JSON.stringify(cache))
      } catch {
        // localStorage 满了就忽略
      }

      // 确保还在同一页面才应用
      if (lastInjectPathRef.current === pathname) {
        applyPageInject(payload)
      }
    }).catch(() => {
      // 请求失败，静默处理
    })

    return () => {
      clearPageInject()
    }
  }, [mounted, pathname, applyPageInject, clearPageInject, normalizePageInjectPayload])

  // 应用主题
  useEffect(() => {
    if (!mounted) return

    const root = document.documentElement
    let resolved: 'light' | 'dark'

    if (theme === 'system') {
      resolved = getSystemTheme()
    } else {
      resolved = theme
    }

    setResolvedTheme(resolved)

    // 应用主题类
    root.classList.remove('light', 'dark')
    root.classList.add(resolved)
  }, [theme, mounted])

  // 监听系统主题变化
  useEffect(() => {
    if (!mounted) return

    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')

    const handleChange = () => {
      if (theme === 'system') {
        const resolved = getSystemTheme()
        setResolvedTheme(resolved)
        document.documentElement.classList.remove('light', 'dark')
        document.documentElement.classList.add(resolved)
      }
    }

    mediaQuery.addEventListener('change', handleChange)
    return () => mediaQuery.removeEventListener('change', handleChange)
  }, [theme, mounted])

  const setTheme = (newTheme: Theme) => {
    setThemeState(newTheme)
    localStorage.setItem(THEME_STORAGE_KEY, newTheme)
  }

  // 始终提供 context，避免子组件在 mounted 前访问 useTheme() 时抛出异常
  return (
    <ThemeContext.Provider value={{ theme, setTheme, resolvedTheme }}>
      {children}
    </ThemeContext.Provider>
  )
}

export function useTheme() {
  const context = useContext(ThemeContext)
  if (context === undefined) {
    throw new Error('useTheme must be used within a ThemeProvider')
  }
  return context
}
