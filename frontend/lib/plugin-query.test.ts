import { buildPluginBootstrapQueryKey } from '@/lib/plugin-bootstrap-query'
import {
  buildPluginSlotQueryKey,
  resolvePluginSlotQueryScope,
} from '@/lib/plugin-slot-query'

describe('plugin query helpers', () => {
  it('builds stable bootstrap query keys regardless of query param order', () => {
    expect(
      buildPluginBootstrapQueryKey('public', '/plugin-pages/logistics/orders/ORD-1001', {
        tab: 'timeline',
        order_id: '123',
      })
    ).toEqual(
      buildPluginBootstrapQueryKey('public', '/plugin-pages/logistics/orders/ORD-1001', {
        order_id: '123',
        tab: 'timeline',
      })
    )
  })

  it('builds stable slot query keys regardless of map ordering', () => {
    expect(
      buildPluginSlotQueryKey({
        path: '/admin/plugin-pages/logistics/orders/ORD-1001/',
        slot: 'admin.plugin_page.top',
        queryParams: {
          tab: 'timeline',
          order_id: '123',
        },
        hostContext: {
          selection: {
            current_page_ids: [11, 12],
            selected_count: 2,
          },
          view: 'admin_orders',
        },
      })
    ).toEqual(
      buildPluginSlotQueryKey({
        path: '/admin/plugin-pages/logistics/orders/ORD-1001',
        slot: 'admin.plugin_page.top',
        queryParams: {
          order_id: '123',
          tab: 'timeline',
        },
        hostContext: {
          view: 'admin_orders',
          selection: {
            selected_count: 2,
            current_page_ids: [11, 12],
          },
        },
      })
    )
  })

  it('infers slot query scope from explicit scope, path, and slot prefix', () => {
    expect(resolvePluginSlotQueryScope('/orders', 'user.orders.top')).toBe('public')
    expect(resolvePluginSlotQueryScope('/admin/orders', 'user.orders.top')).toBe('admin')
    expect(resolvePluginSlotQueryScope('/orders', 'admin.orders.top')).toBe('admin')
    expect(resolvePluginSlotQueryScope('/orders', 'user.orders.top', 'admin')).toBe('admin')
  })
})
