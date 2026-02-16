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

// 格式化相对时间
export function formatRelativeTime(date: string | Date) {
  const dateObj = typeof date === 'string' ? parseISO(date) : date
  const now = new Date()
  const diff = now.getTime() - dateObj.getTime()

  const minutes = Math.floor(diff / 60000)
  const hours = Math.floor(diff / 3600000)
  const days = Math.floor(diff / 86400000)

  if (minutes < 1) return '刚刚'
  if (minutes < 60) return `${minutes}分钟前`
  if (hours < 24) return `${hours}小时前`
  if (days < 7) return `${days}天前`
  return formatDate(dateObj, 'yyyy-MM-dd')
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

// 下载文件
export function downloadFile(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  link.click()
  URL.revokeObjectURL(url)
}

// 手机号打码
export function maskPhone(phone: string) {
  if (!phone || phone.length < 7) return phone
  return phone.replace(/(\d{3})\d{4}(\d{4})/, '$1****$2')
}

// 邮箱打码
export function maskEmail(email: string) {
  if (!email) return email
  const [local, domain] = email.split('@')
  if (!domain) return email
  return `***@${domain}`
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
  return `${symbol}${amount.toFixed(2)}`
}

