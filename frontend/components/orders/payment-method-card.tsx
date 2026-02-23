'use client'

import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { getUserPaymentMethods, getOrderPaymentInfo, selectOrderPaymentMethod, PaymentCardResult } from '@/lib/api'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { SandboxedHtmlFrame } from '@/components/ui/sandboxed-html-frame'
import {
  CreditCard,
  Building2,
  Wallet,
  MessageCircle,
  Bitcoin,
  Code,
  Check,
  Loader2,
  ChevronDown,
  ChevronUp,
  Coins,
} from 'lucide-react'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import toast from 'react-hot-toast'

const iconMap: Record<string, any> = {
  CreditCard,
  Building2,
  Wallet,
  MessageCircle,
  Bitcoin,
  Code,
  Coins,
}

interface PaymentMethodCardProps {
  orderNo: string
  onPaymentSelected?: () => void
}

export function PaymentMethodCard({ orderNo, onPaymentSelected }: PaymentMethodCardProps) {
  const { locale } = useLocale()
  const t = getTranslations(locale)
  const queryClient = useQueryClient()
  const [expanded, setExpanded] = useState(true)
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const [isChanging, setIsChanging] = useState(false)

  // 获取订单付款信息
  const { data: paymentInfo, isLoading, refetch } = useQuery({
    queryKey: ['orderPaymentInfo', orderNo],
    queryFn: () => getOrderPaymentInfo(orderNo),
    refetchOnWindowFocus: false,
    staleTime: 1000 * 60 * 2,
  })

  // 获取可用付款方式列表（用于更换时）
  const { data: methodsData } = useQuery({
    queryKey: ['paymentMethods'],
    queryFn: () => getUserPaymentMethods(),
    enabled: isChanging,
    staleTime: 1000 * 60 * 5,
  })

  // 选择付款方式
  const selectMutation = useMutation({
    mutationFn: (paymentMethodId: number) => selectOrderPaymentMethod(orderNo, paymentMethodId),
    onSuccess: (response) => {
      setIsChanging(false)
      // 直接用 select-payment 返回的数据更新缓存，避免再次调用 payment-info 触发重复的 JSVM 执行
      const selectedMethod = availableMethods.find((m: any) => m.id === selectedId)
      if (selectedMethod && response?.data) {
        queryClient.setQueryData(['orderPaymentInfo', orderNo], {
          data: {
            selected: true,
            payment_method: { id: selectedMethod.id, name: selectedMethod.name, icon: selectedMethod.icon },
            payment_card: response.data,
          },
        })
      }
      toast.success(t.order.paymentMethodSelected)
      onPaymentSelected?.()
    },
    onError: (error: Error) => {
      toast.error(error.message)
    },
  })

  const getIcon = (iconName: string) => {
    const Icon = iconMap[iconName] || CreditCard
    return <Icon className="h-5 w-5" />
  }

  if (isLoading) {
    return (
      <Card>
        <CardContent className="py-8 flex items-center justify-center">
          <Loader2 className="h-6 w-6 animate-spin" />
        </CardContent>
      </Card>
    )
  }

  const info = paymentInfo?.data
  const isSelected = info?.selected && !isChanging
  const availableMethods = isChanging
    ? (methodsData?.data?.items || [])
    : (info?.available_methods || [])
  const paymentCard = info?.payment_card as PaymentCardResult | undefined
  const currentMethod = info?.payment_method

  return (
    <Card>
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <CreditCard className="h-5 w-5" />
            <CardTitle className="text-lg">
              {t.order.paymentMethodTitle}
            </CardTitle>
          </div>
          <Button variant="ghost" size="sm" onClick={() => setExpanded(!expanded)}>
            {expanded ? <ChevronUp className="h-4 w-4" /> : <ChevronDown className="h-4 w-4" />}
          </Button>
        </div>
        {!isSelected && (
          <CardDescription>
            {t.order.selectPaymentMethodHint}
          </CardDescription>
        )}
      </CardHeader>

      {expanded && (
        <CardContent className="space-y-4">
          {isSelected && currentMethod ? (
            // 已选择付款方式 - 显示付款信息
            <div className="space-y-4">
              <div className="flex items-center gap-2 p-3 bg-muted rounded-lg">
                {getIcon(currentMethod.icon)}
                <span className="font-medium">{currentMethod.name}</span>
                <Badge variant="secondary" className="ml-auto">
                  <Check className="h-3 w-3 mr-1" />
                  {t.order.paymentMethodSelected}
                </Badge>
              </div>

              {paymentCard?.html && (
                <SandboxedHtmlFrame
                  html={paymentCard.html}
                  title={t.order.paymentInfoTitle}
                  className="payment-card-content"
                  locale={locale}
                />
              )}

              <Button
                variant="outline"
                className="w-full"
                onClick={() => {
                  setIsChanging(true)
                  setSelectedId(null)
                }}
              >
                {t.order.changePaymentMethod}
              </Button>
            </div>
          ) : (
            // 未选择 - 显示可选列表
            <div className="space-y-2">
              {availableMethods.length === 0 ? (
                <p className="text-sm text-muted-foreground text-center py-4">
                  {t.order.noPaymentMethods}
                </p>
              ) : (
                availableMethods.map((method: any) => (
                  <div
                    key={method.id}
                    className={`flex items-center gap-3 p-3 border rounded-lg cursor-pointer transition-colors hover:bg-muted ${
                      selectedId === method.id ? 'border-primary bg-primary/5' : ''
                    }`}
                    onClick={() => setSelectedId(method.id)}
                  >
                    <div className="p-2 rounded-lg bg-muted">
                      {getIcon(method.icon)}
                    </div>
                    <div className="flex-1 min-w-0">
                      <div className="font-medium">{method.name}</div>
                      <div className="text-sm text-muted-foreground truncate">
                        {method.description}
                      </div>
                    </div>
                    {selectedId === method.id && (
                      <Check className="h-5 w-5 text-primary" />
                    )}
                  </div>
                ))
              )}

              {selectedId && (
                <Button
                  className="w-full mt-4"
                  onClick={() => selectMutation.mutate(selectedId)}
                  disabled={selectMutation.isPending}
                >
                  {selectMutation.isPending ? (
                    <>
                      <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                      {t.common.processing}
                    </>
                  ) : (
                    t.order.confirmSelection
                  )}
                </Button>
              )}
            </div>
          )}
        </CardContent>
      )}
    </Card>
  )
}
