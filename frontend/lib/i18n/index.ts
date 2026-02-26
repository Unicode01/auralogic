import { Locale } from '@/hooks/use-locale'
import { zhTranslations } from './zh'
import { enTranslations } from './en'

export type Translations = typeof zhTranslations

const translations: Record<Locale, Translations> = {
    zh: zhTranslations,
    en: enTranslations,
}

export function getTranslations(locale: Locale): Translations {
    return translations[locale]
}

export { zhTranslations, enTranslations }

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
    const template = (t.cart.bizError as Record<string, string>)?.[errorKey]
    if (!template) return fallbackMessage || errorKey

    if (!params) return template
    return template.replace(/\{(\w+)\}/g, (_, key) =>
        params[key] !== undefined ? String(params[key]) : `{${key}}`
    )
}
