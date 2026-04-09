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
import { cn } from '@/lib/utils'

interface LanguageSwitcherProps {
  compact?: boolean
}

export function LanguageSwitcher({ compact = false }: LanguageSwitcherProps) {
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
          className={cn(
            'w-full',
            compact ? 'mx-auto h-9 w-9 justify-center rounded-lg px-0' : 'justify-start'
          )}
          aria-label={t.sidebar.language}
          title={t.sidebar.language}
        >
          <Languages className={cn(compact ? 'h-4 w-4' : 'h-4 w-4', !compact && 'mr-2')} />
          {compact ? <span className="sr-only">{t.sidebar.language}</span> : t.sidebar.language}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align={compact ? 'center' : 'start'}
        side={compact ? 'right' : 'bottom'}
        className={cn('w-[200px]', compact && 'ml-1')}
      >
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
