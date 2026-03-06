import { type ClassValue, clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'
import { format, parseISO } from 'date-fns'
import { zhCN } from 'date-fns/locale'

// 合并className
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

// 格式化日期
export function formatDate(date: string | Date, formatStr = 'yyyy-MM-dd HH:mm') {
  const dateObj = typeof date === 'string' ? parseISO(date) : date
  return format(dateObj, formatStr, { locale: zhCN })
}

// 复制到剪贴板
export async function copyToClipboard(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    return true
  } catch {
    return false
  }
}

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

// 格式化金额
export function formatCurrency(amount: number | undefined, currency: string = 'CNY') {
  if (amount === undefined || amount === null) return '-'
  const symbol = currencySymbols[currency] || currency + ' '
  return `${symbol}${(amount / 100).toFixed(2)}`
}

// Backward-compatible alias used by existing pages.
export function formatPrice(amount: number | undefined, currency: string = 'CNY') {
  return formatCurrency(amount, currency)
}

function pow10BigInt(scale: number): bigint {
  return BigInt(10) ** BigInt(scale)
}

// Parse major-unit decimal into integer minor units with half-up rounding.
// Returns null for invalid formats or values outside JS safe integer range.
export function parseMajorToMinor(amountMajor: number | string, scale: number = 2): number | null {
  if (!Number.isInteger(scale) || scale < 0 || scale > 9) {
    return null
  }

  const raw = typeof amountMajor === 'number'
    ? (Number.isFinite(amountMajor) ? amountMajor.toString() : '')
    : amountMajor.trim()

  const match = raw.match(/^([+-]?)(\d+)(?:\.(\d*))?$/)
  if (!match) return null

  const sign = match[1] === '-' ? -1n : 1n
  const intPart = match[2]
  const fracPartRaw = match[3] ?? ''

  const unit = pow10BigInt(scale)
  const padded = fracPartRaw.padEnd(scale + 1, '0')
  const keptFrac = padded.slice(0, scale)
  const roundingDigit = padded.charCodeAt(scale) - 48

  let minor = BigInt(intPart) * unit
  if (scale > 0 && keptFrac.length > 0) {
    minor += BigInt(keptFrac)
  }
  if (roundingDigit >= 5) {
    minor += 1n
  }

  minor *= sign

  const maxSafe = BigInt(Number.MAX_SAFE_INTEGER)
  const minSafe = BigInt(Number.MIN_SAFE_INTEGER)
  if (minor > maxSafe || minor < minSafe) {
    return null
  }

  return Number(minor)
}

export function majorToMinor(amountMajor: number | string): number {
  return parseMajorToMinor(amountMajor) ?? 0
}

export function minorToMajor(amountMinor: number): number {
  return amountMinor / 100
}

