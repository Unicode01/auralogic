import { QueryClient } from '@tanstack/react-query'

import { primePluginSlotBatchResponseInQueryCache } from '@/lib/plugin-extension-batch'
import { buildPluginSlotQueryKey } from '@/lib/plugin-slot-query'

describe('plugin-extension-batch cache priming', () => {
  it('writes batch responses into per-slot query cache entries', () => {
    const queryClient = new QueryClient()

    primePluginSlotBatchResponseInQueryCache(queryClient, {
      scope: 'admin',
      defaultPath: '/admin/orders',
      locale: 'en',
      items: [
        {
          key: 'admin.orders.top',
          slot: 'admin.orders.top',
          path: '/admin/orders',
          queryParams: {
            status: 'pending',
          },
          hostContext: {
            section: 'toolbar',
          },
        },
      ],
      responseItems: [
        {
          key: 'admin.orders.top',
          slot: 'admin.orders.top',
          path: '/admin/orders',
          extensions: [
            {
              id: 'ext:orders-top',
              type: 'text',
              content: 'Orders top',
            },
          ],
        },
      ],
    })

    expect(
      queryClient.getQueryData(
        buildPluginSlotQueryKey({
          scope: 'admin',
          path: '/admin/orders',
          slot: 'admin.orders.top',
          locale: 'en',
          queryParams: {
            status: 'pending',
          },
          hostContext: {
            section: 'toolbar',
          },
        })
      )
    ).toEqual({
      data: {
        path: '/admin/orders',
        slot: 'admin.orders.top',
        extensions: [
          {
            id: 'ext:orders-top',
            type: 'text',
            content: 'Orders top',
          },
        ],
      },
    })
  })

  it('falls back to the requested path and empty extensions when a response item is missing', () => {
    const queryClient = new QueryClient()

    primePluginSlotBatchResponseInQueryCache(queryClient, {
      scope: 'public',
      defaultPath: '/tickets',
      locale: 'zh',
      items: [
        {
          key: 'user.tickets.empty',
          slot: 'user.tickets.empty',
          queryParams: {
            page: '2',
          },
          hostContext: {
            section: 'list_state',
          },
        },
      ],
      responseItems: [],
    })

    expect(
      queryClient.getQueryData(
        buildPluginSlotQueryKey({
          scope: 'public',
          path: '/tickets',
          slot: 'user.tickets.empty',
          locale: 'zh',
          queryParams: {
            page: '2',
          },
          hostContext: {
            section: 'list_state',
          },
        })
      )
    ).toEqual({
      data: {
        path: '/tickets',
        slot: 'user.tickets.empty',
        extensions: [],
      },
    })
  })
})
