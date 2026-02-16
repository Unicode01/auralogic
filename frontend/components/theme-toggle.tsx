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

export function ThemeToggle() {
  const { theme, setTheme, resolvedTheme } = useTheme()
  const { locale } = useLocale()

  const themes: { value: Theme; label: string; icon: React.ReactNode }[] = [
    {
      value: 'light',
      label: locale === 'zh' ? '浅色' : 'Light',
      icon: <Sun className="h-4 w-4" />,
    },
    {
      value: 'dark',
      label: locale === 'zh' ? '深色' : 'Dark',
      icon: <Moon className="h-4 w-4" />,
    },
    {
      value: 'system',
      label: locale === 'zh' ? '跟随系统' : 'System',
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
            {locale === 'zh' ? '切换主题' : 'Toggle theme'}
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
