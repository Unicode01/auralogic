'use client'

import { Moon, Sun, Monitor } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { useTheme, Theme } from '@/contexts/theme-context'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'

export function ThemeToggle() {
  const { theme, setTheme, resolvedTheme } = useTheme()
  const { locale } = useLocale()
  const t = getTranslations(locale)

  const themes: { value: Theme; label: string; icon: React.ReactNode }[] = [
    {
      value: 'light',
      label: t.theme.light,
      icon: <Sun className="h-4 w-4" />,
    },
    {
      value: 'dark',
      label: t.theme.dark,
      icon: <Moon className="h-4 w-4" />,
    },
    {
      value: 'system',
      label: t.theme.system,
      icon: <Monitor className="h-4 w-4" />,
    },
  ]

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" className="h-9 w-9">
          {resolvedTheme === 'dark' ? (
            <Moon className="h-4 w-4" />
          ) : (
            <Sun className="h-4 w-4" />
          )}
          <span className="sr-only">
            {t.theme.toggleTheme}
          </span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        {themes.map((t) => (
          <DropdownMenuItem
            key={t.value}
            onClick={() => setTheme(t.value)}
            className={theme === t.value ? 'bg-accent' : ''}
          >
            {t.icon}
            <span className="ml-2">{t.label}</span>
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
