'use client'

import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'

export function Footer() {
  const { locale } = useLocale()
  const t = getTranslations(locale)

  return (
    <footer className="border-t py-6 md:py-0">
      <div className="container flex flex-col items-center justify-between gap-4 md:h-16 md:flex-row">
        <p className="text-center text-sm text-muted-foreground md:text-left">
          © {new Date().getFullYear()} AuraLogic. All rights reserved.
        </p>
        <div className="flex items-center gap-4 text-sm text-muted-foreground">
          <a href="#" className="hover:text-primary">
            {t.footer?.termsOfService || 'Terms of Service'}
          </a>
          <a href="#" className="hover:text-primary">
            {t.footer?.privacyPolicy || 'Privacy Policy'}
          </a>
          <a href="#" className="hover:text-primary">
            {t.footer?.contactUs || 'Contact Us'}
          </a>
        </div>
      </div>
    </footer>
  )
}
