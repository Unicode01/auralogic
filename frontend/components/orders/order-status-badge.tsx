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
    pending_payment: { label: '待付款', labelKey: 'pending_payment' as const, variant: 'destructive' as const },
    draft: { label: '草稿', labelKey: 'draft' as const, variant: 'secondary' as const },
    pending: { label: '待发货', labelKey: 'pending' as const, variant: 'default' as const },
    need_resubmit: { label: '需要重填', labelKey: 'need_resubmit' as const, variant: 'destructive' as const },
    shipped: { label: '已发货', labelKey: 'shipped' as const, variant: 'default' as const },
    completed: { label: '已完成', labelKey: 'completed' as const, variant: 'outline' as const },
    cancelled: { label: '已取消', labelKey: 'cancelled' as const, variant: 'secondary' as const },
    refunded: { label: '已退款', labelKey: 'refunded' as const, variant: 'destructive' as const },
  }

  const config = statusConfig[status] || statusConfig.draft
  const label = t.order.status[config.labelKey] || config.label

  return (
    <Badge variant={config.variant} className={cn(className)}>
      {label}
    </Badge>
  )
}

