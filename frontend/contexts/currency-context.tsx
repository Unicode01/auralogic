'use client'

import { createContext, useContext, ReactNode } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getPublicConfig } from '@/lib/api'

interface CurrencyContextType {
  currency: string
  isLoading: boolean
}

const CurrencyContext = createContext<CurrencyContextType | undefined>(undefined)

// 货币符号映射
const currencySymbols: Record<string, string> = {
  CNY: '¥',
  USD: '$',
  EUR: '€',
  JPY: '¥',
  GBP: '£',
  KRW: '₩',
  HKD: 'HK$',
  TWD: 'NT$',
  SGD: 'S$',
  AUD: 'A$',
  CAD: 'C$',
}

export function CurrencyProvider({ children }: { children: ReactNode }) {
  // 复用 React Query 缓存，与 ThemeProvider 共享同一个 publicConfig 请求
  const { data: publicConfig, isLoading } = useQuery({
    queryKey: ['publicConfig'],
    queryFn: getPublicConfig,
    staleTime: 1000 * 60 * 5,
  })

  const currency = publicConfig?.data?.currency || 'CNY'

  return (
    <CurrencyContext.Provider value={{ currency, isLoading }}>
      {children}
    </CurrencyContext.Provider>
  )
}

export function useCurrency() {
  const context = useContext(CurrencyContext)
  if (context === undefined) {
    throw new Error('useCurrency must be used within a CurrencyProvider')
  }
  return context
}

// 格式化金额（带货币符号）
export function formatPrice(amount: number | undefined, currency: string = 'CNY') {
  if (amount === undefined || amount === null) return '-'
  const symbol = currencySymbols[currency] || currency + ' '
  return `${symbol}${amount.toFixed(2)}`
}

// 获取货币符号
export function getCurrencySymbol(currency: string = 'CNY') {
  return currencySymbols[currency] || currency + ' '
}
