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
