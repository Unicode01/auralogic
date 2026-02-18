'use client'

import { createContext, useContext, useState, useEffect, useLayoutEffect, useCallback, ReactNode } from 'react'
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
    const [locale, setLocaleState] = useState<Locale>('en')
    const [mounted, setMounted] = useState(false)

    // paint 前从 window.__LOCALE__（head 脚本已同步设好）修正语言，零 I/O
    useLayoutEffect(() => {
        setMounted(true)
        const w = (window as any).__LOCALE__
        const correct: Locale = (w === 'zh' || w === 'en') ? w : (getStoredLocale() || getBrowserLocale())
        if (correct !== 'en') setLocaleState(correct)
    }, [])

    // API 同步（不阻塞渲染）
    useEffect(() => {
        const pendingLocale = getPendingLocale()
        const hasToken = !!getToken()
        if (hasToken && pendingLocale) {
            updateUserPreferences({ locale: pendingLocale })
                .then(() => clearPendingLocale())
                .catch(() => {})
        }
        const storedLocale = getStoredLocale()
        if (!storedLocale) {
            const browserLocale = getBrowserLocale()
            if (hasToken) {
                updateUserPreferences({ locale: browserLocale })
                    .then(() => clearPendingLocale())
                    .catch(() => setPendingLocale(browserLocale))
            } else {
                setPendingLocale(browserLocale)
            }
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
