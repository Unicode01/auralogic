'use client'
/* eslint-disable @next/next/no-img-element */

import { Package } from 'lucide-react'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePublicBranding } from '@/hooks/use-public-branding'

export function Footer() {
  const { locale } = useLocale()
  const t = getTranslations(locale)
  const { appName, logoUrl } = usePublicBranding()

  return (
    <footer className="border-t py-6 md:py-0">
      <div className="container flex flex-col items-center justify-between gap-4 md:h-16 md:flex-row">
        <div className="flex flex-col items-center gap-2 text-center md:flex-row md:text-left">
          <div className="flex items-center gap-2">
            {logoUrl ? (
              <img src={logoUrl} alt={appName} className="max-h-5 w-auto max-w-[112px] object-contain" />
            ) : (
              <Package className="h-4 w-4 text-muted-foreground" />
            )}
          </div>
          <p className="text-sm text-muted-foreground">
            © {new Date().getFullYear()} {appName}. All rights reserved.
          </p>
        </div>
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
