import type { LucideIcon } from 'lucide-react'
import {
  Bitcoin,
  Building2,
  Code,
  Coins,
  CreditCard,
  MessageCircle,
  Wallet,
} from 'lucide-react'

export const paymentMethodIconMap = {
  CreditCard,
  Building2,
  Wallet,
  MessageCircle,
  Bitcoin,
  Code,
  Coins,
} satisfies Record<string, LucideIcon>

export type PaymentMethodIconName = keyof typeof paymentMethodIconMap

export const paymentMethodIconNames = Object.keys(paymentMethodIconMap) as PaymentMethodIconName[]

export function resolvePaymentMethodIcon(
  iconName?: string,
  fallback: LucideIcon = CreditCard
): LucideIcon {
  if (!iconName) {
    return fallback
  }
  return paymentMethodIconMap[iconName as PaymentMethodIconName] || fallback
}
