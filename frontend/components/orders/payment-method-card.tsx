'use client'

import { useEffect, useRef, useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  getUserPaymentMethods,
  getOrderPaymentInfo,
  selectOrderPaymentMethod,
  PaymentCardResult,
} from '@/lib/api'
import { resolveApiErrorMessage } from '@/lib/api-error'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { SandboxedHtmlFrame } from '@/components/ui/sandboxed-html-frame'
import { resolvePaymentMethodIcon } from '@/lib/payment-method-icons'
import {
  CreditCard,
  Check,
  Loader2,
  ChevronDown,
  ChevronUp,
  AlertTriangle,
} from 'lucide-react'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import toast from 'react-hot-toast'
import { PluginSlot } from '@/components/plugins/plugin-slot'

const localizedBuiltinPaymentMethodDescriptions: Record<string, { zh: string; en: string }> = {
  'Pay with USDT via TRC20 network, supports auto-confirmation': {
    zh: '使用 USDT TRC20 网络转账，支持自动确认',
    en: 'Pay with USDT via TRC20 network, supports auto-confirmation',
  },
  'Pay with USDT via BEP20 (BSC) network, supports auto-confirmation': {
    zh: '使用 USDT BEP20（BSC）网络转账，支持自动确认',
    en: 'Pay with USDT via BEP20 (BSC) network, supports auto-confirmation',
  },
}

interface PaymentMethodCardProps {
  orderNo: string
  onPaymentSelected?: () => void
  pluginSlotNamespace?: string
  pluginSlotContext?: Record<string, any>
  pluginSlotPath?: string
}

export function PaymentMethodCard({
  orderNo,
  onPaymentSelected,
  pluginSlotNamespace,
  pluginSlotContext,
  pluginSlotPath,
}: PaymentMethodCardProps) {
  const { locale } = useLocale()
  const t = getTranslations(locale)
  const queryClient = useQueryClient()
  const [expanded, setExpanded] = useState(true)
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const [isChanging, setIsChanging] = useState(false)
  const [paymentInfoErrorMessage, setPaymentInfoErrorMessage] = useState('')

  // 获取订单付款信息
  const {
    data: paymentInfo,
    isLoading,
    isFetching,
    isError,
    error: paymentInfoError,
    refetch,
  } = useQuery({
    queryKey: ['orderPaymentInfo', orderNo],
    queryFn: () => getOrderPaymentInfo(orderNo),
    refetchOnWindowFocus: false,
    staleTime: 1000 * 60 * 2,
  })
  const paymentInfoErrorRef = useRef('')

  useEffect(() => {
    if (!paymentInfoError) {
      paymentInfoErrorRef.current = ''
      setPaymentInfoErrorMessage('')
      return
    }
    const error = paymentInfoError as any
    const signature = `${error?.code ?? 'unknown'}:${error?.errorKey ?? error?.data?.error_key ?? ''}:${error?.message ?? ''}`
    if (paymentInfoErrorRef.current === signature) {
      return
    }
    paymentInfoErrorRef.current = signature

    const message = resolveApiErrorMessage(error, t, t.order.operationFailed)
    setPaymentInfoErrorMessage(message)
    toast.error(message)
  }, [paymentInfoError, t])

  // 获取可用付款方式列表（用于更换时）
  const { data: methodsData, isLoading: methodsLoading } = useQuery({
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
      setPaymentInfoErrorMessage('')
      // 直接用 select-payment 返回的数据更新缓存，避免再次调用 payment-info 触发重复的 JSVM 执行
      const selectedMethod = availableMethods.find((m: any) => m.id === selectedId)
      if (selectedMethod && response?.data) {
        queryClient.setQueryData(['orderPaymentInfo', orderNo], {
          data: {
            selected: true,
            payment_method: {
              id: selectedMethod.id,
              name: selectedMethod.name,
              icon: selectedMethod.icon,
            },
            payment_card: response.data,
          },
        })
      }
      toast.success(t.order.paymentMethodSelected)
      onPaymentSelected?.()
    },
    onError: (error: any) => {
      toast.error(resolveApiErrorMessage(error, t, t.order.operationFailed))
    },
  })

  const getIcon = (iconName: string) => {
    const Icon = resolvePaymentMethodIcon(iconName)
    return <Icon className="h-5 w-5" />
  }

  const getMethodDescription = (method: any) => {
    const rawDescription = String(method?.description || '').trim()
    if (!rawDescription) {
      return t.order.paymentMethodNoDescription
    }
    const localized = localizedBuiltinPaymentMethodDescriptions[rawDescription]
    if (!localized) {
      return rawDescription
    }
    return locale.startsWith('zh') ? localized.zh : localized.en
  }

  const info = paymentInfo?.data
  const isSelected = info?.selected && !isChanging
  const availableMethods = isChanging
    ? methodsData?.data?.items || []
    : info?.available_methods || []
  const paymentCard = info?.payment_card as PaymentCardResult | undefined
  const currentMethod = info?.payment_method
  const paymentMethodPluginContext = {
    ...(pluginSlotContext || {}),
    payment_panel: {
      order_no: orderNo,
      expanded,
      is_changing: isChanging,
      selected_method_id: selectedId || undefined,
      current_method: currentMethod
        ? {
            id: currentMethod.id,
            name: currentMethod.name,
            icon: currentMethod.icon,
          }
        : undefined,
    },
    summary: {
      available_method_count: availableMethods.length,
      has_payment_card_html: Boolean(paymentCard?.html),
      payment_info_error_message: paymentInfoErrorMessage || undefined,
    },
    state: {
      expanded,
      info_loading: isLoading,
      info_load_failed: isError,
      method_selected: Boolean(isSelected && currentMethod),
      methods_loading: methodsLoading,
      methods_empty: !methodsLoading && availableMethods.length === 0,
      changing: isChanging,
      selection_pending: selectMutation.isPending,
    },
  }

  return (
    <Card>
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <CreditCard className="h-5 w-5" />
            <CardTitle className="text-lg">{t.order.paymentMethodTitle}</CardTitle>
          </div>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setExpanded(!expanded)}
            aria-label={`${expanded ? t.common.collapse : t.common.expand} ${t.order.paymentMethodTitle}`}
            title={`${expanded ? t.common.collapse : t.common.expand} ${t.order.paymentMethodTitle}`}
          >
            {expanded ? <ChevronUp className="h-4 w-4" /> : <ChevronDown className="h-4 w-4" />}
          </Button>
        </div>
      </CardHeader>

      {expanded && (
        <CardContent className="space-y-4">
          {pluginSlotNamespace ? (
            <PluginSlot
              slot={`${pluginSlotNamespace}.top`}
              path={pluginSlotPath}
              context={{ ...paymentMethodPluginContext, section: 'payment' }}
            />
          ) : null}
          {isLoading ? (
            <div className="flex flex-col items-center justify-center gap-3 py-8 text-center">
              <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
              <div className="space-y-1">
                <p className="font-medium">{t.order.paymentMethodTitle}</p>
                <p className="text-sm text-muted-foreground">{t.order.paymentMethodLoadingDesc}</p>
              </div>
            </div>
          ) : isError ? (
            <div className="space-y-3">
              <Alert variant="destructive">
                <AlertTriangle className="h-4 w-4" />
                <AlertTitle>{t.order.paymentMethodLoadFailed}</AlertTitle>
                <AlertDescription>{t.order.paymentMethodLoadFailedDesc}</AlertDescription>
              </Alert>
              <div className="rounded-md border border-destructive/20 bg-destructive/5 p-3 text-sm text-muted-foreground">
                {paymentInfoErrorMessage || t.order.operationFailed}
              </div>
              <Button variant="outline" size="sm" onClick={() => refetch()}>
                {t.order.retryLoadPaymentInfo}
              </Button>
              {pluginSlotNamespace ? (
                <PluginSlot
                  slot={`${pluginSlotNamespace}.load_failed`}
                  path={pluginSlotPath}
                  context={{ ...paymentMethodPluginContext, section: 'payment_state' }}
                />
              ) : null}
            </div>
          ) : isSelected && currentMethod ? (
            // 已选择付款方式 - 显示付款信息
            <div className="space-y-4">
              <div className="flex items-center gap-2 rounded-lg bg-muted p-3">
                {getIcon(currentMethod.icon)}
                <span className="font-medium">{currentMethod.name}</span>
                <Badge variant="secondary" className="ml-auto">
                  <Check className="mr-1 h-3 w-3" />
                  {t.order.paymentMethodSelected}
                </Badge>
              </div>

              {paymentCard?.html ? (
                <SandboxedHtmlFrame
                  html={paymentCard.html}
                  title={t.order.paymentInfoTitle}
                  className="payment-card-content"
                  locale={locale}
                />
              ) : (
                <Alert>
                  <AlertTitle>{t.order.paymentInfoTitle}</AlertTitle>
                  <AlertDescription>{t.order.paymentCardPendingHint}</AlertDescription>
                </Alert>
              )}

              <div className="flex flex-wrap gap-2">
                <Button
                  variant="outline"
                  className="min-w-[180px] flex-1"
                  onClick={() => {
                    setIsChanging(true)
                    setSelectedId(null)
                  }}
                >
                  {t.order.changePaymentMethod}
                </Button>
                <Button
                  variant="outline"
                  className="min-w-[140px] flex-1"
                  onClick={() => refetch()}
                  disabled={isFetching}
                >
                  {t.common.refresh}
                </Button>
              </div>
              {pluginSlotNamespace ? (
                <PluginSlot
                  slot={`${pluginSlotNamespace}.selected.after`}
                  path={pluginSlotPath}
                  context={{ ...paymentMethodPluginContext, section: 'selected' }}
                />
              ) : null}
            </div>
          ) : (
            // 未选择 - 显示可选列表
            <div className="space-y-3">
              {methodsLoading ? (
                <div className="flex flex-col items-center justify-center gap-3 py-8 text-center">
                  <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
                  <p className="text-sm text-muted-foreground">
                    {t.order.paymentMethodListLoading}
                  </p>
                </div>
              ) : availableMethods.length === 0 ? (
                <div className="space-y-3">
                  <Alert>
                    <AlertTitle>{t.order.noPaymentMethods}</AlertTitle>
                    <AlertDescription>{t.order.noPaymentMethodsHint}</AlertDescription>
                  </Alert>
                  {isChanging && currentMethod ? (
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => {
                        setIsChanging(false)
                        setSelectedId(null)
                      }}
                    >
                      {t.common.cancel}
                    </Button>
                  ) : null}
                </div>
              ) : (
                <>
                  <div className="rounded-lg border border-input/60 bg-muted/10 px-3 py-2 text-sm text-muted-foreground">
                    {t.order.selectPaymentMethodHint}
                  </div>
                  <div className="grid gap-3">
                    {availableMethods.map((method: any) => {
                      const methodDescription = getMethodDescription(method)

                      return (
                        <button
                          key={method.id}
                          type="button"
                          className={`flex w-full cursor-pointer items-center justify-start gap-3 rounded-xl border px-4 py-3 text-left transition-all hover:border-primary/35 hover:bg-muted/70 ${
                            selectedId === method.id
                              ? 'border-primary/60 bg-primary/10 shadow-sm ring-1 ring-primary/20'
                              : 'bg-background'
                          }`}
                          onClick={() => setSelectedId(method.id)}
                          aria-pressed={selectedId === method.id}
                          title={`${method.name} - ${methodDescription}`}
                        >
                          <div
                            className={`rounded-lg p-2 ${
                              selectedId === method.id ? 'bg-primary/15 text-primary' : 'bg-muted'
                            }`}
                          >
                            {getIcon(method.icon)}
                          </div>
                          <div className="min-w-0 flex-1">
                            <div className="font-medium">{method.name}</div>
                            <div className="mt-1 line-clamp-2 break-words text-sm leading-5 text-muted-foreground">
                              {methodDescription}
                            </div>
                          </div>
                          {selectedId === method.id ? (
                            <Badge className="shrink-0 gap-1">
                              <Check className="h-3.5 w-3.5" />
                              {t.order.paymentMethodSelected}
                            </Badge>
                          ) : null}
                        </button>
                      )
                    })}
                  </div>
                  {pluginSlotNamespace ? (
                    <PluginSlot
                      slot={`${pluginSlotNamespace}.methods.after`}
                      path={pluginSlotPath}
                      context={{ ...paymentMethodPluginContext, section: 'methods' }}
                    />
                  ) : null}

                  <div className="flex flex-wrap gap-2">
                    <Button
                      className="min-w-[180px] flex-1"
                      onClick={() => selectedId && selectMutation.mutate(selectedId)}
                      disabled={selectMutation.isPending || !selectedId}
                    >
                      {selectMutation.isPending ? (
                        <>
                          <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                          {t.common.processing}
                        </>
                      ) : (
                        t.order.confirmSelection
                      )}
                    </Button>
                    {isChanging && currentMethod ? (
                      <Button
                        variant="outline"
                        className="min-w-[140px] flex-1"
                        onClick={() => {
                          setIsChanging(false)
                          setSelectedId(null)
                        }}
                      >
                        {t.common.cancel}
                      </Button>
                    ) : null}
                  </div>
                </>
              )}
            </div>
          )}
          {pluginSlotNamespace ? (
            <PluginSlot
              slot={`${pluginSlotNamespace}.bottom`}
              path={pluginSlotPath}
              context={{ ...paymentMethodPluginContext, section: 'payment' }}
            />
          ) : null}
        </CardContent>
      )}
    </Card>
  )
}
