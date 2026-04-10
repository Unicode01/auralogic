import type { Locale } from '@/hooks/use-locale'
import { zhTranslations } from './zh'

export type Translations = typeof zhTranslations

const translations: Partial<Record<Locale, Translations>> = {
    zh: zhTranslations,
}

export function getTranslations(locale: Locale): Translations {
    return translations[locale] || zhTranslations
}

export async function loadTranslations(locale: Locale): Promise<Translations> {
    const cached = translations[locale]
    if (cached) {
        return cached
    }

    let loaded: Translations
    switch (locale) {
        case 'en': {
            const mod = await import('./en')
            loaded = mod.enTranslations
            break
        }
        case 'zh':
        default:
            loaded = zhTranslations
            break
    }

    translations[locale] = loaded
    return loaded
}

export function hasLoadedTranslations(locale: Locale): boolean {
    return Boolean(translations[locale])
}

export { zhTranslations }

type BizErrorDictionary = Record<string, string>

const bizErrorDictionaryCache = new WeakMap<object, BizErrorDictionary>()

function isRecord(value: unknown): value is Record<string, unknown> {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}

function collectBizErrorDictionaries(
    value: unknown,
    output: BizErrorDictionary,
    seen: WeakSet<object>
): void {
    if (!isRecord(value) || seen.has(value)) {
        return
    }

    seen.add(value)

    const bizErrorValue = value.bizError
    if (isRecord(bizErrorValue)) {
        for (const [key, template] of Object.entries(bizErrorValue)) {
            if (typeof template === 'string' && template.trim()) {
                output[key] = template
            }
        }
    }

    for (const [key, child] of Object.entries(value)) {
        if (key === 'bizError') continue
        collectBizErrorDictionaries(child, output, seen)
    }
}

function getBizErrorDictionary(t: Translations): BizErrorDictionary {
    const cacheKey = t as object
    const cached = bizErrorDictionaryCache.get(cacheKey)
    if (cached) {
        return cached
    }

    const dictionary: BizErrorDictionary = {}
    collectBizErrorDictionaries(t, dictionary, new WeakSet<object>())
    bizErrorDictionaryCache.set(cacheKey, dictionary)
    return dictionary
}

/**
 * Translate a bizerr error_key with parameter interpolation.
 * Falls back to the raw message if no translation is found.
 */
export function translateBizError(
    t: Translations,
    errorKey: string,
    params?: Record<string, any>,
    fallbackMessage?: string
): string {
    const template = getBizErrorDictionary(t)[errorKey]
    if (!template) return fallbackMessage || errorKey

    const normalizedParams = normalizeTemplateParams(params)
    if (!normalizedParams) return template
    return template.replace(/\{(\w+)\}/g, (_, key) =>
        normalizedParams[key] !== undefined ? String(normalizedParams[key]) : `{${key}}`
    )
}

function normalizeTemplateParams(params?: Record<string, any>): Record<string, any> | undefined {
    if (!params || typeof params !== 'object') return undefined

    const normalized: Record<string, any> = { ...params }
    const resolveText = (...keys: string[]) => {
        for (const key of keys) {
            const value = normalized[key]
            if (value === undefined || value === null) continue
            const text = String(value).trim()
            if (text) return text
        }
        return ''
    }

    const cause = resolveText('cause', 'case', 'details', 'reason', 'error', 'message')
    if (cause) {
        if (normalized.cause === undefined) normalized.cause = cause
        if (normalized.case === undefined) normalized.case = cause
        if (normalized.details === undefined) normalized.details = cause
    }

    const reason = resolveText('reason', 'cause', 'case', 'details')
    if (reason && normalized.reason === undefined) {
        normalized.reason = reason
    }

    return normalized
}
