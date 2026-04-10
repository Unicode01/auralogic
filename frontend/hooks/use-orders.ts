'use client'

import { useQuery } from '@tanstack/react-query'
import { getOrders } from '@/lib/api'
import type { OrderQueryParams } from '@/lib/api'
import { getOrderListQueryOptions } from '@/lib/order-list-queries'
import { getOrderDetailQueryOptions } from '@/lib/order-detail-queries'

export function useOrders(params: OrderQueryParams = {}) {
  const queryOptions = getOrderListQueryOptions(params)
  return useQuery({
    ...queryOptions,
  })
}

export function useOrderDetail(orderNo: string, options?: { refetchInterval?: number | false }) {
  const queryOptions = getOrderDetailQueryOptions(orderNo)
  return useQuery({
    ...queryOptions,
    enabled: !!orderNo,
    staleTime: 0, // 数据立即过期，确保每次都获取最新数据
    refetchOnMount: true, // 组件挂载时重新获取
    refetchInterval: options?.refetchInterval,
  })
}
