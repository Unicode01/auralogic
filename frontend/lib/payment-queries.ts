import {
  getOrderPaymentInfo,
  getUserPaymentMethods,
  type PaymentCardResult,
  type PaymentMethod,
} from '@/lib/api'

type AnyRecord = Record<string, unknown>

export const adminPaymentMethodsQueryKey = ['adminPaymentMethods'] as const
export const userPaymentMethodsQueryKey = ['userPaymentMethods'] as const

function asRecord(value: unknown): AnyRecord | undefined {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return undefined
  }
  return value as AnyRecord
}

export function getOrderPaymentInfoQueryKey(orderNo: string) {
  return ['orderPaymentInfo', orderNo] as const
}

export function getOrderPaymentInfoQueryOptions(orderNo: string) {
  return {
    queryKey: getOrderPaymentInfoQueryKey(orderNo),
    queryFn: () => getOrderPaymentInfo(orderNo),
    refetchOnWindowFocus: false,
    staleTime: 1000 * 60 * 2,
  }
}

export function getUserPaymentMethodsQueryOptions() {
  return {
    queryKey: userPaymentMethodsQueryKey,
    queryFn: () => getUserPaymentMethods(),
    staleTime: 1000 * 60 * 5,
  }
}

export function mergeSelectedOrderPaymentInfo(
  currentPayload: unknown,
  selectedMethod: Pick<PaymentMethod, 'id' | 'name' | 'icon'>,
  paymentCard: PaymentCardResult
) {
  const root = asRecord(currentPayload) || {}
  const currentData = asRecord(root.data) || {}

  return {
    ...root,
    data: {
      ...currentData,
      selected: true,
      payment_method: {
        id: selectedMethod.id,
        name: selectedMethod.name,
        icon: selectedMethod.icon,
      },
      payment_card: paymentCard,
    },
  }
}
