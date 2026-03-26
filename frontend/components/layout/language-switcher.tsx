'use client'

import { Languages } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'

export function LanguageSwitcher() {
  const { locale, setLocale, mounted } = useLocale()
  const t = getTranslations(locale)

  if (!mounted) {
    return null
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="outline"
          size="sm"
          className="w-full justify-start"
          aria-label={t.sidebar.language}
          title={t.sidebar.language}
        >
          <Languages className="mr-2 h-4 w-4" />
          {t.sidebar.language}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-[200px]">
        <DropdownMenuItem
          onClick={() => setLocale('zh')}
          className={locale === 'zh' ? 'bg-accent' : ''}
        >
          {t.language.zh}
        </DropdownMenuItem>
        <DropdownMenuItem
          onClick={() => setLocale('en')}
          className={locale === 'en' ? 'bg-accent' : ''}
        >
          {t.language.en}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
