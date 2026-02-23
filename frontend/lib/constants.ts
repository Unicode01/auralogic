// 订单状态配置
export const ORDER_STATUS_CONFIG = {
  draft: { label: '草稿', color: 'gray' },
  pending: { label: '待发货', color: 'blue' },
  need_resubmit: { label: '需要重填', color: 'red' },
  shipped: { label: '已发货', color: 'green' },
  completed: { label: '已完成', color: 'purple' },
  cancelled: { label: '已取消', color: 'gray' },
  refunded: { label: '已退款', color: 'red' },
} as const

// 权限列表 - 与后端保持一致
export const PERMISSIONS = [
  // 订单权限
  { value: 'order.view', labelKey: 'permOrderView' as const, category: 'order' },
  { value: 'order.view_privacy', labelKey: 'permOrderViewPrivacy' as const, category: 'order' },
  { value: 'order.edit', labelKey: 'permOrderEdit' as const, category: 'order' },
  { value: 'order.delete', labelKey: 'permOrderDelete' as const, category: 'order' },
  { value: 'order.status_update', labelKey: 'permOrderStatusUpdate' as const, category: 'order' },
  { value: 'order.refund', labelKey: 'permOrderRefund' as const, category: 'order' },
  { value: 'order.assign_tracking', labelKey: 'permOrderAssignTracking' as const, category: 'order' },
  { value: 'order.request_resubmit', labelKey: 'permOrderRequestResubmit' as const, category: 'order' },

  // 商品权限
  { value: 'product.view', labelKey: 'permProductView' as const, category: 'product' },
  { value: 'product.edit', labelKey: 'permProductEdit' as const, category: 'product' },
  { value: 'product.delete', labelKey: 'permProductDelete' as const, category: 'product' },

  // 用户权限
  { value: 'user.view', labelKey: 'permUserView' as const, category: 'user' },
  { value: 'user.edit', labelKey: 'permUserEdit' as const, category: 'user' },
  { value: 'user.permission', labelKey: 'permUserPermission' as const, category: 'user' },

  // 工单权限
  { value: 'ticket.view', labelKey: 'permTicketView' as const, category: 'ticket' },
  { value: 'ticket.reply', labelKey: 'permTicketReply' as const, category: 'ticket' },
  { value: 'ticket.status_update', labelKey: 'permTicketStatusUpdate' as const, category: 'ticket' },

  // 序列号权限
  { value: 'serial.view', labelKey: 'permSerialView' as const, category: 'serial' },
  { value: 'serial.manage', labelKey: 'permSerialManage' as const, category: 'serial' },

  // 知识库权限
  { value: 'knowledge.view', labelKey: 'permKnowledgeView' as const, category: 'knowledge' },
  { value: 'knowledge.edit', labelKey: 'permKnowledgeEdit' as const, category: 'knowledge' },

  // 公告权限
  { value: 'announcement.view', labelKey: 'permAnnouncementView' as const, category: 'announcement' },
  { value: 'announcement.edit', labelKey: 'permAnnouncementEdit' as const, category: 'announcement' },

  // 管理员权限
  { value: 'admin.create', labelKey: 'permAdminCreate' as const, category: 'admin' },
  { value: 'admin.edit', labelKey: 'permAdminEdit' as const, category: 'admin' },
  { value: 'admin.delete', labelKey: 'permAdminDelete' as const, category: 'admin' },
  { value: 'admin.permission', labelKey: 'permAdminPermission' as const, category: 'admin' },

  // 系统权限
  { value: 'system.config', labelKey: 'permSystemConfig' as const, category: 'system' },
  { value: 'system.logs', labelKey: 'permSystemLogs' as const, category: 'system' },
  { value: 'api.manage', labelKey: 'permApiManage' as const, category: 'system' },
]

// 权限分类键名
export const PERMISSION_CATEGORIES = ['order', 'product', 'serial', 'user', 'ticket', 'knowledge', 'announcement', 'admin', 'system'] as const

// 分类键名到翻译键的映射
export const CATEGORY_LABEL_KEYS: Record<string, string> = {
  order: 'permCategoryOrder',
  product: 'permCategoryProduct',
  serial: 'permCategorySerial',
  user: 'permCategoryUser',
  ticket: 'permCategoryTicket',
  knowledge: 'permCategoryKnowledge',
  announcement: 'permCategoryAnnouncement',
  admin: 'permCategoryAdmin',
  system: 'permCategorySystem',
}

// 按分类组织的权限（使用分类键名）
export const PERMISSIONS_BY_CATEGORY: Record<string, typeof PERMISSIONS> = {
  order: PERMISSIONS.filter(p => p.category === 'order'),
  product: PERMISSIONS.filter(p => p.category === 'product'),
  serial: PERMISSIONS.filter(p => p.category === 'serial'),
  user: PERMISSIONS.filter(p => p.category === 'user'),
  ticket: PERMISSIONS.filter(p => p.category === 'ticket'),
  knowledge: PERMISSIONS.filter(p => p.category === 'knowledge'),
  announcement: PERMISSIONS.filter(p => p.category === 'announcement'),
  admin: PERMISSIONS.filter(p => p.category === 'admin'),
  system: PERMISSIONS.filter(p => p.category === 'system'),
}

// 工单状态配置
export const TICKET_STATUS_CONFIG = {
  open: { label: '待处理', color: 'yellow' },
  processing: { label: '处理中', color: 'blue' },
  resolved: { label: '已解决', color: 'green' },
  closed: { label: '已关闭', color: 'gray' },
} as const

// 工单优先级配置
export const TICKET_PRIORITY_CONFIG = {
  low: { label: '低', color: 'gray' },
  normal: { label: '普通', color: 'blue' },
  high: { label: '高', color: 'orange' },
  urgent: { label: '紧急', color: 'red' },
} as const

// 分页默认值
export const DEFAULT_PAGE_SIZE = 20
export const MAX_PAGE_SIZE = 100

