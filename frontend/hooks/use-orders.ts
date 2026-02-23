'use client'

import { useQuery } from '@tanstack/react-query'
import { getOrders, getOrder } from '@/lib/api'
import type { OrderQueryParams } from '@/lib/api'

export function useOrders(params: OrderQueryParams = {}) {
  return useQuery({
    queryKey: ['orders', params],
    queryFn: () => getOrders(params),
  })
}

export function useOrderDetail(orderNo: string, options?: { refetchInterval?: number | false }) {
  return useQuery({
    queryKey: ['order', orderNo],
    queryFn: () => getOrder(orderNo),
    enabled: !!orderNo,
    staleTime: 0, // 数据立即过期，确保每次都获取最新数据
    refetchOnMount: true, // 组件挂载时重新获取
    refetchInterval: options?.refetchInterval,
  })
}

