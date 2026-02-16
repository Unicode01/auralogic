'use client'

import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { useState } from 'react'
import { LocaleProvider } from '@/contexts/locale-context'
import { CurrencyProvider } from '@/contexts/currency-context'
import { ThemeProvider } from '@/contexts/theme-context'

export function Providers({ children }: { children: React.ReactNode }) {
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: {
            staleTime: 60 * 1000,
            refetchOnWindowFocus: false,
          },
        },
      })
  )

  return (
    <QueryClientProvider client={queryClient}>
      <ThemeProvider>
        <LocaleProvider>
          <CurrencyProvider>
            {children}
          </CurrencyProvider>
        </LocaleProvider>
      </ThemeProvider>
    </QueryClientProvider>
  )
}

