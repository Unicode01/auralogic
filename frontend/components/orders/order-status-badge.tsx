'use client'

import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import type { OrderStatus } from '@/types/order'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'

interface OrderStatusBadgeProps {
  status: OrderStatus
  className?: string
}

export function OrderStatusBadge({ status, className }: OrderStatusBadgeProps) {
  const { locale } = useLocale()
  const t = getTranslations(locale)

  const statusConfig = {
    pending_payment: { labelKey: 'pending_payment' as const, variant: 'destructive' as const },
    draft: { labelKey: 'draft' as const, variant: 'secondary' as const },
    pending: { labelKey: 'pending' as const, variant: 'default' as const },
    need_resubmit: { labelKey: 'need_resubmit' as const, variant: 'destructive' as const },
    shipped: { labelKey: 'shipped' as const, variant: 'default' as const },
    completed: { labelKey: 'completed' as const, variant: 'outline' as const },
    cancelled: { labelKey: 'cancelled' as const, variant: 'secondary' as const },
    refunded: { labelKey: 'refunded' as const, variant: 'destructive' as const },
  }

  const config = statusConfig[status] || statusConfig.draft
  const label = t.order.status[config.labelKey] || status

  return (
    <Badge variant={config.variant} className={cn(className)}>
      {label}
    </Badge>
  )
}

