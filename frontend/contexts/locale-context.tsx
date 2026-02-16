'use client'

import { createContext, useContext, useState, useEffect, useCallback, ReactNode } from 'react'
import { updateUserPreferences } from '@/lib/api'
import { getToken } from '@/lib/auth'

export type Locale = 'zh' | 'en'

const LOCALE_STORAGE_KEY = 'auralogic_locale'
const LOCALE_PENDING_SYNC_KEY = 'auralogic_locale_pending_sync'

interface LocaleContextType {
    locale: Locale
    setLocale: (locale: Locale) => void
    toggleLocale: () => void
    mounted: boolean
}

const LocaleContext = createContext<LocaleContextType | undefined>(undefined)

/**
 * 获取浏览器默认语言
 */
function getBrowserLocale(): Locale {
    if (typeof window === 'undefined') return 'zh'

    const browserLang = navigator.language.toLowerCase()
    if (browserLang.startsWith('zh')) return 'zh'
    return 'en'
}

/**
 * 获取存储的语言设置
 */
function getStoredLocale(): Locale | null {
    if (typeof window === 'undefined') return null

    const stored = localStorage.getItem(LOCALE_STORAGE_KEY)
    if (stored === 'zh' || stored === 'en') return stored
    return null
}

/**
 * 保存语言设置
 */
function setStoredLocale(locale: Locale) {
    if (typeof window === 'undefined') return
    localStorage.setItem(LOCALE_STORAGE_KEY, locale)
}

function getPendingLocale(): Locale | null {
    if (typeof window === 'undefined') return null
    const v = localStorage.getItem(LOCALE_PENDING_SYNC_KEY)
    if (v === 'zh' || v === 'en') return v
    return null
}

function setPendingLocale(locale: Locale) {
    if (typeof window === 'undefined') return
    localStorage.setItem(LOCALE_PENDING_SYNC_KEY, locale)
}

function clearPendingLocale() {
    if (typeof window === 'undefined') return
    localStorage.removeItem(LOCALE_PENDING_SYNC_KEY)
}

export function LocaleProvider({ children }: { children: ReactNode }) {
    // 始终使用 'zh' 作为初始值以避免 hydration 错误
    const [locale, setLocaleState] = useState<Locale>('zh')
    const [mounted, setMounted] = useState(false)

    // 客户端挂载后从 localStorage 或浏览器语言同步状态
    useEffect(() => {
        setMounted(true)
        const storedLocale = getStoredLocale()
        const pendingLocale = getPendingLocale()
        const hasToken = !!getToken()

        // Retry a previously failed sync once we have a token.
        if (hasToken && pendingLocale) {
            updateUserPreferences({ locale: pendingLocale })
                .then(() => clearPendingLocale())
                .catch(() => {})
        }

        if (storedLocale) {
            setLocaleState(storedLocale)
            return
        }

        // First initialization: use browser locale, persist it, then sync preference.
        const browserLocale = getBrowserLocale()
        setLocaleState(browserLocale)
        setStoredLocale(browserLocale)
        if (hasToken) {
            updateUserPreferences({ locale: browserLocale })
                .then(() => clearPendingLocale())
                .catch(() => setPendingLocale(browserLocale))
        } else {
            setPendingLocale(browserLocale)
        }
    }, [])

    // 同步 html lang 属性
    useEffect(() => {
        document.documentElement.lang = locale === 'zh' ? 'zh-CN' : 'en'
    }, [locale])

    // 切换语言
    const setLocale = useCallback((newLocale: Locale) => {
        setLocaleState(newLocale)
        setStoredLocale(newLocale)
        // 如果已登录，同步到后端；否则记录为待同步（登录后自动同步）
        if (getToken()) {
            updateUserPreferences({ locale: newLocale })
                .then(() => clearPendingLocale())
                .catch(() => setPendingLocale(newLocale))
        } else {
            setPendingLocale(newLocale)
        }
    }, [])

    // 切换到另一种语言
    const toggleLocale = useCallback(() => {
        const newLocale = locale === 'zh' ? 'en' : 'zh'
        setLocale(newLocale)
    }, [locale, setLocale])

    return (
        <LocaleContext.Provider value={{ locale, setLocale, toggleLocale, mounted }}>
            {children}
        </LocaleContext.Provider>
    )
}

export function useLocale() {
    const context = useContext(LocaleContext)
    if (context === undefined) {
        throw new Error('useLocale must be used within a LocaleProvider')
    }
    return context
}
