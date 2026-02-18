import axios, { AxiosInstance } from 'axios'
import { getToken, clearToken } from './auth'

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'https://auralogic.un1c0de.com'

// 创建axios实例
const apiClient: AxiosInstance = axios.create({
  baseURL: API_BASE_URL,
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
})

// 请求拦截器
apiClient.interceptors.request.use(
  (config) => {
    const token = getToken()
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  (error) => {
    return Promise.reject(error)
  }
)

// 响应拦截器
apiClient.interceptors.response.use(
  (response) => {
    return response.data
  },
  (error) => {
    if (error.response?.status === 401) {
      // Token过期，清除token但不自动跳转
      // 跳转逻辑由各页面的布局组件控制
      clearToken()
    }

    const message = error.response?.data?.message || '请求失败'
    const apiError: any = new Error(message)
    apiError.code = error.response?.data?.code
    apiError.data = error.response?.data?.data
    return Promise.reject(apiError)
  }
)

// ==========================================
// 库存管理API
// ==========================================

export interface Inventory {
  id: number
  product_id: number
  sku: string
  attributes: Record<string, string>
  stock: number
  available_quantity: number
  sold_quantity: number
  reserved_quantity: number
  safety_stock: number
  alert_email?: string
  is_active: boolean
  notes?: string
  created_at: string
  updated_at: string
  product?: any
}

export interface CreateInventoryRequest {
  name: string  // 库存配置名称
  sku?: string  // SKU（可选）
  attributes?: Record<string, string>  // 属性组合
  stock: number
  available_quantity: number
  safety_stock: number
  alert_email?: string
  notes?: string
}

export interface UpdateInventoryRequest {
  stock: number
  available_quantity: number
  safety_stock: number
  is_active: boolean
  alert_email?: string
  notes?: string
}

export interface AdjustStockRequest {
  stock: number
  available_quantity: number
  reason: string
  notes?: string
}

// 获取库存列表
export async function getInventories(params?: {
  page?: number
  limit?: number
  is_active?: boolean
  low_stock?: boolean
}) {
  const query = new URLSearchParams()
  if (params?.page) query.append('page', params.page.toString())
  if (params?.limit) query.append('limit', params.limit.toString())
  if (params?.is_active !== undefined) query.append('is_active', params.is_active.toString())
  if (params?.low_stock) query.append('low_stock', 'true')

  return apiClient.get(`/api/admin/inventories?${query}`)
}

// 创建库存配置（独立创建）
export async function createInventory(data: CreateInventoryRequest) {
  return apiClient.post('/api/admin/inventories', data)
}

// 商品-库存绑定相关
export async function getProductBindings(productId: number) {
  return apiClient.get(`/api/admin/products/${productId}/inventory-bindings`)
}

export async function createProductBinding(productId: number, data: {
  inventory_id: number
  is_random: boolean
  priority: number
  notes?: string
}) {
  return apiClient.post(`/api/admin/products/${productId}/inventory-bindings`, data)
}

export async function batchCreateProductBindings(productId: number, bindings: Array<{
  inventory_id: number
  is_random: boolean
  priority: number
  notes?: string
}>) {
  return apiClient.post(`/api/admin/products/${productId}/inventory-bindings/batch`, {
    bindings
  })
}

export async function updateProductBinding(productId: number, bindingId: number, data: {
  is_random: boolean
  priority: number
  notes?: string
}) {
  return apiClient.put(`/api/admin/products/${productId}/inventory-bindings/${bindingId}`, data)
}

export async function deleteProductBinding(productId: number, bindingId: number) {
  return apiClient.delete(`/api/admin/products/${productId}/inventory-bindings/${bindingId}`)
}

// 批量删除商品的所有库存绑定
export async function deleteAllProductBindings(productId: number) {
  return apiClient.delete(`/api/admin/products/${productId}/inventory-bindings`)
}

// 替换商品的所有库存绑定（先删除所有，再批量创建）
export async function replaceProductBindings(productId: number, bindings: any[]) {
  return apiClient.put(`/api/admin/products/${productId}/inventory-bindings/replace`, {
    bindings
  })
}

export async function updateProductInventoryMode(productId: number, mode: 'fixed' | 'random') {
  return apiClient.put(`/api/admin/products/${productId}/inventory-mode`, {
    inventory_mode: mode
  })
}

// 获取库存详情
export async function getInventory(id: number) {
  return apiClient.get(`/api/admin/inventories/${id}`)
}

// 更新库存配置
export async function updateInventory(id: number, data: UpdateInventoryRequest) {
  return apiClient.put(`/api/admin/inventories/${id}`, data)
}

// 调整库存
export async function adjustStock(id: number, data: AdjustStockRequest) {
  return apiClient.post(`/api/admin/inventories/${id}/adjust`, data)
}

// 删除库存配置
export async function deleteInventory(id: number) {
  return apiClient.delete(`/api/admin/inventories/${id}`)
}

// 获取低库存列表
export async function getLowStockList() {
  return apiClient.get('/api/admin/inventories/low-stock')
}

// 获取库存日志
export async function getInventoryLogs(params?: {
  page?: number
  limit?: number
  inventory_id?: number
  type?: string
  order_no?: string
  start_date?: string
  end_date?: string
}) {
  const query = new URLSearchParams()
  if (params?.page) query.append('page', params.page.toString())
  if (params?.limit) query.append('limit', params.limit.toString())
  if (params?.inventory_id) query.append('inventory_id', params.inventory_id.toString())
  if (params?.type) query.append('type', params.type)
  if (params?.order_no) query.append('order_no', params.order_no)
  if (params?.start_date) query.append('start_date', params.start_date)
  if (params?.end_date) query.append('end_date', params.end_date)

  return apiClient.get(`/api/admin/logs/inventories?${query}`)
}

// ==========================================
// 表单API
// ==========================================

export interface ShippingFormData {
  form_token: string
  receiver_name: string
  phone_code?: string  // 手机区号
  receiver_phone: string
  receiver_email: string
  receiver_country: string  // 国家代码
  receiver_province?: string
  receiver_city?: string
  receiver_district?: string
  receiver_address: string
  receiver_postcode?: string
  privacy_protected?: boolean
  password?: string
  user_remark?: string  // 用户备注
}

export async function getFormInfo(formToken: string) {
  return apiClient.get(`/api/form/shipping?token=${formToken}`)
}

export async function submitShippingForm(data: ShippingFormData) {
  return apiClient.post('/api/form/shipping', data)
}

// 获取国家列表
export async function getCountries() {
  return apiClient.get('/api/form/countries')
}

// ==========================================
// 认证API
// ==========================================

export interface LoginData {
  email: string
  password: string
  captcha_token?: string
}

export interface RegisterData {
  email: string
  password: string
  name: string
  captcha_token?: string
}

export async function login(data: LoginData) {
  return apiClient.post('/api/user/auth/login', data)
}

export async function register(data: RegisterData) {
  return apiClient.post('/api/user/auth/register', data)
}

export async function verifyEmail(token: string) {
  return apiClient.get(`/api/user/auth/verify-email?token=${token}`)
}

export async function resendVerification(email: string) {
  return apiClient.post('/api/user/auth/resend-verification', { email })
}

export async function sendLoginCode(data: { email: string; captcha_token?: string }) {
  return apiClient.post('/api/user/auth/send-login-code', data)
}

export async function loginWithCode(data: { email: string; code: string }) {
  return apiClient.post('/api/user/auth/login-with-code', data)
}

export async function forgotPassword(data: { email: string; captcha_token?: string }) {
  return apiClient.post('/api/user/auth/forgot-password', data)
}

export async function resetPassword(data: { token: string; new_password: string }) {
  return apiClient.post('/api/user/auth/reset-password', data)
}

export async function logout() {
  return apiClient.post('/api/user/auth/logout')
}

export async function getCurrentUser() {
  return apiClient.get('/api/user/auth/me')
}

export async function changePassword(oldPassword: string, newPassword: string) {
  return apiClient.post('/api/user/auth/change-password', {
    old_password: oldPassword,
    new_password: newPassword,
  })
}

export async function updateUserPreferences(data: { locale?: string; country?: string }) {
  return apiClient.put('/api/user/auth/preferences', data)
}

export async function getCaptcha() {
  return apiClient.get('/api/user/auth/captcha')
}

// ==========================================
// 订单API
// ==========================================

export interface OrderQueryParams {
  page?: number
  limit?: number
  status?: string
  search?: string
  product_search?: string  // 新增：按商品SKU/名称搜索
  promo_code_id?: number
  promo_code?: string
  user_id?: number
  country?: string
  start_date?: string
  end_date?: string
}

export async function getOrders(params?: OrderQueryParams) {
  const query = new URLSearchParams()
  if (params?.page) query.append('page', params.page.toString())
  if (params?.limit) query.append('limit', params.limit.toString())
  if (params?.status) query.append('status', params.status)
  if (params?.search) query.append('search', params.search)

  return apiClient.get(`/api/user/orders?${query}`)
}

export async function getOrder(orderNo: string) {
  return apiClient.get(`/api/user/orders/${orderNo}`)
}

export async function createOrder(data: { items: any[], promo_code?: string }) {
  return apiClient.post('/api/user/orders', data)
}

export async function getOrRefreshFormToken(orderNo: string) {
  return apiClient.get(`/api/user/orders/${orderNo}/form-token`)
}

// Get virtual products for an order
export async function getOrderVirtualProducts(orderNo: string) {
  return apiClient.get(`/api/user/orders/${orderNo}/virtual-products`)
}

// ==========================================
// 商品API
// ==========================================

export async function getProducts(params?: {
  page?: number
  limit?: number
  category?: string
  search?: string
}) {
  const query = new URLSearchParams()
  if (params?.page) query.append('page', params.page.toString())
  if (params?.limit) query.append('limit', params.limit.toString())
  if (params?.category) query.append('category', params.category)
  if (params?.search) query.append('search', params.search)

  return apiClient.get(`/api/user/products?${query}`)
}

export async function getProduct(id: number) {
  return apiClient.get(`/api/user/products/${id}`)
}

// 获取商品可用库存
export async function getProductAvailableStock(id: number, attributes?: Record<string, string>) {
  let url = `/api/user/products/${id}/available-stock`
  if (attributes && Object.keys(attributes).length > 0) {
    const encodedAttrs = encodeURIComponent(JSON.stringify(attributes))
    url += `?attributes=${encodedAttrs}`
  }
  return apiClient.get(url)
}

export async function getFeaturedProducts(limit?: number) {
  const query = limit ? `?limit=${limit}` : ''
  return apiClient.get(`/api/user/products/featured${query}`)
}

export async function getCategories() {
  return apiClient.get('/api/user/products/categories')
}

export async function getProductCategories() {
  return apiClient.get('/api/user/products/categories')
}

// ==========================================
// 购物车API
// ==========================================

export interface CartItem {
  id: number
  product_id: number
  sku: string
  name: string
  price: number
  image_url: string
  product_type: string
  quantity: number
  attributes: Record<string, string>
  available_stock: number
  is_available: boolean
  product?: any
}

export interface CartResponse {
  items: CartItem[]
  total_price: number
  total_quantity: number
  item_count: number
}

// 获取购物车
export async function getCart() {
  return apiClient.get('/api/user/cart')
}

// 获取购物车商品数量
export async function getCartCount() {
  return apiClient.get('/api/user/cart/count')
}

// 添加商品到购物车
export async function addToCart(data: {
  product_id: number
  quantity: number
  attributes?: Record<string, string>
}) {
  return apiClient.post('/api/user/cart/items', data)
}

// 更新购物车商品数量
export async function updateCartItemQuantity(itemId: number, quantity: number) {
  return apiClient.put(`/api/user/cart/items/${itemId}`, { quantity })
}

// 从购物车移除商品
export async function removeFromCart(itemId: number) {
  return apiClient.delete(`/api/user/cart/items/${itemId}`)
}

// 清空购物车
export async function clearCart() {
  return apiClient.delete('/api/user/cart')
}

// ==========================================
// 管理员API
// ==========================================

// 管理员订单管理
export async function getAdminOrders(params?: OrderQueryParams) {
  const query = new URLSearchParams()
  if (params?.page) query.append('page', params.page.toString())
  if (params?.limit) query.append('limit', params.limit.toString())
  if (params?.status) query.append('status', params.status)
  if (params?.search) query.append('search', params.search)
  if (params?.product_search) query.append('product_search', params.product_search) // 新增
  if (params?.promo_code_id) query.append('promo_code_id', params.promo_code_id.toString())
  if (params?.promo_code) query.append('promo_code', params.promo_code)
  if (params?.user_id) query.append('user_id', params.user_id.toString())
  if (params?.country) query.append('country', params.country)
  if (params?.start_date) query.append('start_date', params.start_date)
  if (params?.end_date) query.append('end_date', params.end_date)

  return apiClient.get(`/api/admin/orders?${query}`)
}

export async function getAdminOrder(id: number) {
  return apiClient.get(`/api/admin/orders/${id}`)
}

// 获取有订单的国家列表
export async function getOrderCountries() {
  return apiClient.get('/api/admin/orders/countries')
}

// 管理员商品管理
export async function getAdminProducts(params?: {
  page?: number
  limit?: number
  category?: string
  status?: string
  search?: string
}) {
  const query = new URLSearchParams()
  if (params?.page) query.append('page', params.page.toString())
  if (params?.limit) query.append('limit', params.limit.toString())
  if (params?.category) query.append('category', params.category)
  if (params?.status) query.append('status', params.status)
  if (params?.search) query.append('search', params.search)

  return apiClient.get(`/api/admin/products?${query}`)
}

export async function getAdminProduct(id: number) {
  return apiClient.get(`/api/admin/products/${id}`)
}

export async function createProduct(data: any) {
  return apiClient.post('/api/admin/products', data)
}

export async function updateProduct(id: number, data: any) {
  return apiClient.put(`/api/admin/products/${id}`, data)
}

export async function deleteProduct(id: number) {
  return apiClient.delete(`/api/admin/products/${id}`)
}

export async function toggleProductFeatured(id: number) {
  return apiClient.post(`/api/admin/products/${id}/toggle-featured`)
}

export async function updateProductStatus(id: number, status: string) {
  return apiClient.put(`/api/admin/products/${id}/status`, { status })
}

export async function uploadImage(file: File) {
  const formData = new FormData()
  formData.append('file', file)
  return apiClient.post('/api/admin/upload/image', formData, {
    headers: {
      'Content-Type': 'multipart/form-data',
    },
  })
}

// 管理员订单详情
export async function getAdminOrderDetail(id: number) {
  return apiClient.get(`/api/admin/orders/${id}`)
}

// 管理员创建订单
export async function createAdminOrder(data: any) {
  return apiClient.post('/api/admin/orders', data)
}

export async function assignTracking(id: number, data: any) {
  return apiClient.post(`/api/admin/orders/${id}/assign-shipping`, data)
}

export async function adminCompleteOrder(id: number, remark?: string) {
  return apiClient.post(`/api/admin/orders/${id}/complete`, { remark })
}

export async function adminCancelOrder(id: number, reason?: string) {
  return apiClient.post(`/api/admin/orders/${id}/cancel`, { reason })
}

export async function adminDeleteOrder(id: number) {
  return apiClient.delete(`/api/admin/orders/${id}`)
}

export async function completeAllShippedOrders() {
  return apiClient.post('/api/admin/orders/batch/complete-shipped')
}

export async function batchUpdateOrders(orderIds: number[], action: string) {
  return apiClient.post('/api/admin/orders/batch/update', { order_ids: orderIds, action })
}

export async function updateOrderShippingInfo(id: number, data: any) {
  return apiClient.put(`/api/admin/orders/${id}/shipping-info`, data)
}

export async function requestOrderResubmit(id: number, reason: string) {
  return apiClient.post(`/api/admin/orders/${id}/request-resubmit`, { reason })
}

export async function adminMarkOrderAsPaid(id: number) {
  return apiClient.post(`/api/admin/orders/${id}/mark-paid`)
}

export async function adminDeliverVirtualStock(id: number) {
  return apiClient.post(`/api/admin/orders/${id}/deliver-virtual`)
}

export async function updateOrderPrice(id: number, totalAmount: number) {
  return apiClient.put(`/api/admin/orders/${id}/price`, { total_amount: totalAmount })
}

// 用户管理
export async function getUsers(params?: {
  page?: number
  limit?: number
  role?: string
  search?: string
}) {
  const query = new URLSearchParams()
  if (params?.page) query.append('page', params.page.toString())
  if (params?.limit) query.append('limit', params.limit.toString())
  if (params?.role) query.append('role', params.role)
  if (params?.search) query.append('search', params.search)

  return apiClient.get(`/api/admin/users?${query}`)
}

export async function getUserDetail(id: number) {
  return apiClient.get(`/api/admin/users/${id}`)
}

export async function createUser(data: any) {
  return apiClient.post('/api/admin/users', data)
}

export async function updateUser(id: number, data: any) {
  return apiClient.put(`/api/admin/users/${id}`, data)
}

export async function deleteUser(id: number) {
  return apiClient.delete(`/api/admin/users/${id}`)
}

// 管理员用户管理
export async function getAdmins(params?: {
  page?: number
  limit?: number
}) {
  const query = new URLSearchParams()
  if (params?.page) query.append('page', params.page.toString())
  if (params?.limit) query.append('limit', params.limit.toString())

  return apiClient.get(`/api/admin/admins?${query}`)
}

export async function createAdmin(data: any) {
  return apiClient.post('/api/admin/admins', data)
}

export async function updateAdmin(id: number, data: any) {
  return apiClient.put(`/api/admin/admins/${id}`, data)
}

export async function deleteAdmin(id: number) {
  return apiClient.delete(`/api/admin/admins/${id}`)
}

// API密钥管理
export async function getApiKeys() {
  return apiClient.get('/api/admin/api-keys')
}

export async function createApiKey(data: {
  key_name: string
  platform: string
  scopes: string[]
  rate_limit?: number
  expires_at?: string
}) {
  return apiClient.post('/api/admin/api-keys', data)
}

export async function deleteApiKey(id: number) {
  return apiClient.delete(`/api/admin/api-keys/${id}`)
}

// 系统日志
export async function getOperationLogs(params?: {
  page?: number
  limit?: number
  action?: string
  resource_type?: string
  user_id?: string
  start_date?: string
  end_date?: string
}) {
  const query = new URLSearchParams()
  if (params?.page) query.append('page', params.page.toString())
  if (params?.limit) query.append('limit', params.limit.toString())
  if (params?.action) query.append('action', params.action)
  if (params?.resource_type) query.append('resource_type', params.resource_type)
  if (params?.user_id) query.append('user_id', params.user_id)
  if (params?.start_date) query.append('start_date', params.start_date)
  if (params?.end_date) query.append('end_date', params.end_date)

  return apiClient.get(`/api/admin/logs/operations?${query}`)
}

export async function getEmailLogs(params?: {
  page?: number
  limit?: number
  status?: string
  event_type?: string
  to_email?: string
  start_date?: string
  end_date?: string
}) {
  const query = new URLSearchParams()
  if (params?.page) query.append('page', params.page.toString())
  if (params?.limit) query.append('limit', params.limit.toString())
  if (params?.status) query.append('status', params.status)
  if (params?.event_type) query.append('event_type', params.event_type)
  if (params?.to_email) query.append('to_email', params.to_email)
  if (params?.start_date) query.append('start_date', params.start_date)
  if (params?.end_date) query.append('end_date', params.end_date)

  return apiClient.get(`/api/admin/logs/emails?${query}`)
}

export async function getLogStatistics() {
  return apiClient.get('/api/admin/logs/statistics')
}

export async function retryFailedEmails(emailIds?: number[]) {
  if (emailIds && emailIds.length > 0) {
    return apiClient.post('/api/admin/logs/emails/retry', { email_ids: emailIds })
  }
  return apiClient.post('/api/admin/logs/emails/retry')
}

// 仪表盘
export async function getDashboardStatistics() {
  return apiClient.get('/api/admin/dashboard/statistics')
}

export async function getRecentActivities() {
  return apiClient.get('/api/admin/dashboard/activities')
}

// ==================== Analytics ====================

export async function getUserAnalytics() {
  return apiClient.get('/api/admin/analytics/users')
}

export async function getOrderAnalytics() {
  return apiClient.get('/api/admin/analytics/orders')
}

export async function getRevenueAnalytics() {
  return apiClient.get('/api/admin/analytics/revenue')
}

export async function getDeviceAnalytics() {
  return apiClient.get('/api/admin/analytics/devices')
}

// ==================== Virtual Product Stock ====================

// Import virtual product stock
export async function importVirtualStock(productId: number, data: {
  import_type: 'file' | 'text'
  file?: File
  content?: string
}) {
  const formData = new FormData()
  formData.append('import_type', data.import_type)

  if (data.import_type === 'file' && data.file) {
    formData.append('file', data.file)
  } else if (data.import_type === 'text' && data.content) {
    formData.append('content', data.content)
  }

  return apiClient.post(`/api/admin/virtual-products/${productId}/import`, formData, {
    headers: { 'Content-Type': 'multipart/form-data' }
  })
}

// Get virtual product stock list
export async function getVirtualStockList(productId: number, params?: {
  page?: number
  limit?: number
  status?: string
}) {
  const query = new URLSearchParams()
  if (params?.page) query.append('page', params.page.toString())
  if (params?.limit) query.append('limit', params.limit.toString())
  if (params?.status) query.append('status', params.status)

  return apiClient.get(`/api/admin/virtual-products/${productId}/stocks?${query}`)
}

// Get virtual product stock stats
export async function getVirtualStockStats(productId: number) {
  return apiClient.get(`/api/admin/virtual-products/${productId}/stats`)
}

// Delete virtual stock
export async function deleteVirtualStock(stockId: number) {
  return apiClient.delete(`/api/admin/virtual-products/stocks/${stockId}`)
}

// Delete stock batch
export async function deleteStockBatch(batchNo: string) {
  return apiClient.delete('/api/admin/virtual-products/batch', {
    data: { batch_no: batchNo }
  })
}

// ==================== Virtual Inventory (New API) ====================

// Virtual Inventory interface
export interface VirtualInventory {
  id: number
  name: string
  sku: string
  description: string
  is_active: boolean
  notes: string
  total: number
  available: number
  reserved: number
  sold: number
  created_at: string
}

// Get virtual inventories list
export async function getVirtualInventories(params?: {
  page?: number
  limit?: number
  search?: string
}) {
  const query = new URLSearchParams()
  if (params?.page) query.append('page', params.page.toString())
  if (params?.limit) query.append('limit', params.limit.toString())
  if (params?.search) query.append('search', params.search)

  return apiClient.get(`/api/admin/virtual-inventories?${query}`)
}

// Create virtual inventory
export async function createVirtualInventory(data: {
  name: string
  sku?: string
  description?: string
  is_active?: boolean
  notes?: string
}) {
  return apiClient.post('/api/admin/virtual-inventories', data)
}

// Get virtual inventory detail
export async function getVirtualInventory(id: number) {
  return apiClient.get(`/api/admin/virtual-inventories/${id}`)
}

// Update virtual inventory
export async function updateVirtualInventory(id: number, data: {
  name?: string
  sku?: string
  description?: string
  is_active?: boolean
  notes?: string
}) {
  return apiClient.put(`/api/admin/virtual-inventories/${id}`, data)
}

// Delete virtual inventory
export async function deleteVirtualInventory(id: number) {
  return apiClient.delete(`/api/admin/virtual-inventories/${id}`)
}

// Import stock to virtual inventory
export async function importVirtualInventoryStock(virtualInventoryId: number, data: {
  import_type: 'file' | 'text'
  file?: File
  content?: string
}) {
  const formData = new FormData()
  formData.append('import_type', data.import_type)

  if (data.import_type === 'file' && data.file) {
    formData.append('file', data.file)
  } else if (data.import_type === 'text' && data.content) {
    formData.append('content', data.content)
  }

  return apiClient.post(`/api/admin/virtual-inventories/${virtualInventoryId}/import`, formData, {
    headers: { 'Content-Type': 'multipart/form-data' }
  })
}

// Create stock manually in virtual inventory
export async function createVirtualInventoryStockManually(virtualInventoryId: number, data: {
  content: string
  remark?: string
}) {
  return apiClient.post(`/api/admin/virtual-inventories/${virtualInventoryId}/stocks`, data)
}

// Get virtual inventory stock list
export async function getVirtualInventoryStockList(virtualInventoryId: number, params?: {
  page?: number
  limit?: number
  status?: string
}) {
  const query = new URLSearchParams()
  if (params?.page) query.append('page', params.page.toString())
  if (params?.limit) query.append('limit', params.limit.toString())
  if (params?.status) query.append('status', params.status)

  return apiClient.get(`/api/admin/virtual-inventories/${virtualInventoryId}/stocks?${query}`)
}

// Alias for getVirtualInventoryStockList
export const getVirtualInventoryStocks = getVirtualInventoryStockList

// Get virtual inventory stock stats
export async function getVirtualInventoryStockStats(virtualInventoryId: number) {
  return apiClient.get(`/api/admin/virtual-inventories/${virtualInventoryId}/stats`)
}

// Delete virtual inventory stock item
export async function deleteVirtualInventoryStock(virtualInventoryId: number, stockId: number) {
  return apiClient.delete(`/api/admin/virtual-inventories/${virtualInventoryId}/stocks/${stockId}`)
}

// Reserve virtual inventory stock item (manual)
export async function reserveVirtualInventoryStock(virtualInventoryId: number, stockId: number, remark?: string) {
  return apiClient.post(`/api/admin/virtual-inventories/${virtualInventoryId}/stocks/${stockId}/reserve`, { remark })
}

// Release virtual inventory stock item (manual)
export async function releaseVirtualInventoryStock(virtualInventoryId: number, stockId: number) {
  return apiClient.post(`/api/admin/virtual-inventories/${virtualInventoryId}/stocks/${stockId}/release`)
}

// Get products bound to virtual inventory
export async function getVirtualInventoryProducts(virtualInventoryId: number) {
  return apiClient.get(`/api/admin/virtual-inventories/${virtualInventoryId}/products`)
}

// ==================== Product Virtual Inventory Bindings ====================

// Get product virtual inventory bindings
export async function getProductVirtualInventoryBindings(productId: number) {
  return apiClient.get(`/api/admin/products/${productId}/virtual-inventory-bindings`)
}

// Create product virtual inventory binding
export async function createProductVirtualInventoryBinding(productId: number, data: {
  virtual_inventory_id: number
  is_random?: boolean
  priority?: number
  notes?: string
}) {
  return apiClient.post(`/api/admin/products/${productId}/virtual-inventory-bindings`, data)
}

// Update product virtual inventory binding
export async function updateProductVirtualInventoryBinding(productId: number, bindingId: number, data: {
  is_random?: boolean
  priority?: number
  notes?: string
}) {
  return apiClient.put(`/api/admin/products/${productId}/virtual-inventory-bindings/${bindingId}`, data)
}

// Delete product virtual inventory binding
export async function deleteProductVirtualInventoryBinding(productId: number, bindingId: number) {
  return apiClient.delete(`/api/admin/products/${productId}/virtual-inventory-bindings/${bindingId}`)
}

// Save product virtual inventory variant bindings (batch save)
export async function saveProductVirtualVariantBindings(productId: number, bindings: Array<{
  attributes: Record<string, string>
  virtual_inventory_id: number | null
  is_random?: boolean
  priority?: number
}>) {
  return apiClient.put(`/api/admin/products/${productId}/virtual-inventory-bindings`, { bindings })
}

// 系统设置
export async function getSettings() {
  return apiClient.get('/api/admin/settings')
}

export async function updateSettings(data: any) {
  return apiClient.put('/api/admin/settings', data)
}

export async function testSMTP(data: any) {
  return apiClient.post('/api/admin/settings/smtp/test', data)
}

// 邮件模板管理
export async function getEmailTemplates() {
  return apiClient.get('/api/admin/settings/email-templates')
}

export async function getEmailTemplate(filename: string) {
  return apiClient.get(`/api/admin/settings/email-templates/${filename}`)
}

export async function updateEmailTemplate(filename: string, content: string) {
  return apiClient.put(`/api/admin/settings/email-templates/${filename}`, { content })
}

// 落地页管理
export async function getLandingPage() {
  return apiClient.get('/api/admin/settings/landing-page')
}

export async function updateLandingPage(htmlContent: string) {
  return apiClient.put('/api/admin/settings/landing-page', { html_content: htmlContent })
}

export async function resetLandingPage() {
  return apiClient.post('/api/admin/settings/landing-page/reset')
}

// 页面访问分析
export async function getPageViewAnalytics() {
  return apiClient.get('/api/admin/analytics/pageviews')
}

// 公开配置（无需登录）
export async function getPublicConfig() {
  return apiClient.get('/api/config/public')
}

// 获取页面注入脚本/样式（无需登录，通过path参数穿透CDN）
export async function getPageInject(path: string) {
  return apiClient.get(`/api/config/page-inject?path=${encodeURIComponent(path)}`)
}

// ==========================================
// 付款方式API
// ==========================================

export interface PaymentMethod {
  id: number
  name: string
  description: string
  type: 'builtin' | 'custom'
  enabled: boolean
  script?: string
  config?: string
  icon?: string
  sort_order: number
  poll_interval: number
  created_at: string
  updated_at: string
}

export interface PaymentCardResult {
  html: string
  title?: string
  description?: string
  data?: Record<string, any>
}

// 管理端API
export async function getPaymentMethods(enabledOnly?: boolean) {
  return apiClient.get('/api/admin/payment-methods', { params: { enabled_only: enabledOnly } })
}

export async function getPaymentMethod(id: number) {
  return apiClient.get(`/api/admin/payment-methods/${id}`)
}

export async function createPaymentMethod(data: Partial<PaymentMethod>) {
  return apiClient.post('/api/admin/payment-methods', data)
}

export async function updatePaymentMethod(id: number, data: Partial<PaymentMethod>) {
  return apiClient.put(`/api/admin/payment-methods/${id}`, data)
}

export async function deletePaymentMethod(id: number) {
  return apiClient.delete(`/api/admin/payment-methods/${id}`)
}

export async function togglePaymentMethodEnabled(id: number) {
  return apiClient.post(`/api/admin/payment-methods/${id}/toggle`)
}

export async function reorderPaymentMethods(ids: number[]) {
  return apiClient.post('/api/admin/payment-methods/reorder', { ids })
}

export async function testPaymentScript(script: string, config?: Record<string, any>) {
  return apiClient.post('/api/admin/payment-methods/test-script', { script, config })
}

export async function initBuiltinPaymentMethods() {
  return apiClient.post('/api/admin/payment-methods/init-builtin')
}

// 用户端API
export async function getUserPaymentMethods() {
  return apiClient.get('/api/user/payment-methods')
}

export async function getOrderPaymentInfo(orderNo: string) {
  return apiClient.get(`/api/user/orders/${orderNo}/payment-info`)
}

export async function getOrderPaymentCard(orderNo: string, paymentMethodId: number) {
  return apiClient.get(`/api/user/orders/${orderNo}/payment-card`, { params: { payment_method_id: paymentMethodId } })
}

export async function selectOrderPaymentMethod(orderNo: string, paymentMethodId: number) {
  return apiClient.post(`/api/user/orders/${orderNo}/select-payment`, { payment_method_id: paymentMethodId })
}

// ==========================================
// 工单/客服中心 API
// ==========================================

export interface Ticket {
  id: number
  ticket_no: string
  user_id: number
  subject: string
  content: string
  category?: string
  priority: string
  status: string
  assigned_to?: number
  last_message_at?: string
  last_message_preview?: string
  last_message_by?: string
  unread_count_user: number
  unread_count_admin: number
  created_at: string
  updated_at: string
  closed_at?: string
  user?: any
  assigned_user?: any
}

export interface TicketMessage {
  id: number
  ticket_id: number
  sender_type: string
  sender_id: number
  sender_name: string
  content: string
  content_type: string
  metadata?: any
  is_read_by_user: boolean
  is_read_by_admin: boolean
  created_at: string
}

// 用户端工单 API
export async function createTicket(data: {
  subject: string
  content: string
  category?: string
  priority?: string
  order_id?: number
}) {
  return apiClient.post('/api/user/tickets', data)
}

export async function getTickets(params?: {
  page?: number
  limit?: number
  status?: string
  search?: string
}) {
  const query = new URLSearchParams()
  if (params?.page) query.append('page', params.page.toString())
  if (params?.limit) query.append('limit', params.limit.toString())
  if (params?.status) query.append('status', params.status)
  if (params?.search) query.append('search', params.search)

  return apiClient.get(`/api/user/tickets?${query}`)
}

export async function getTicket(id: number) {
  return apiClient.get(`/api/user/tickets/${id}`)
}

export async function getTicketMessages(id: number) {
  return apiClient.get(`/api/user/tickets/${id}/messages`)
}

export async function sendTicketMessage(id: number, data: {
  content: string
  content_type?: string
}) {
  return apiClient.post(`/api/user/tickets/${id}/messages`, data)
}

export async function updateTicketStatus(id: number, status: string) {
  return apiClient.put(`/api/user/tickets/${id}/status`, { status })
}

export async function shareOrderToTicket(ticketId: number, data: {
  order_id: number
  can_edit?: boolean
  can_view_privacy?: boolean
}) {
  return apiClient.post(`/api/user/tickets/${ticketId}/share-order`, data)
}

export async function getTicketSharedOrders(ticketId: number) {
  return apiClient.get(`/api/user/tickets/${ticketId}/shared-orders`)
}

export async function revokeTicketOrderAccess(ticketId: number, orderId: number) {
  return apiClient.delete(`/api/user/tickets/${ticketId}/shared-orders/${orderId}`)
}

// 管理端工单 API
export async function getAdminTickets(params?: {
  page?: number
  limit?: number
  status?: string
  search?: string
  assigned_to?: string
}) {
  const query = new URLSearchParams()
  if (params?.page) query.append('page', params.page.toString())
  if (params?.limit) query.append('limit', params.limit.toString())
  if (params?.status) query.append('status', params.status)
  if (params?.search) query.append('search', params.search)
  if (params?.assigned_to) query.append('assigned_to', params.assigned_to)

  return apiClient.get(`/api/admin/tickets?${query}`)
}

export async function getAdminTicket(id: number) {
  return apiClient.get(`/api/admin/tickets/${id}`)
}

export async function getAdminTicketMessages(id: number) {
  return apiClient.get(`/api/admin/tickets/${id}/messages`)
}

export async function sendAdminTicketMessage(id: number, data: {
  content: string
  content_type?: string
}) {
  return apiClient.post(`/api/admin/tickets/${id}/messages`, data)
}

export async function updateAdminTicket(id: number, data: {
  status?: string
  priority?: string
  assigned_to?: number
}) {
  return apiClient.put(`/api/admin/tickets/${id}`, data)
}

export async function getAdminTicketSharedOrders(ticketId: number) {
  return apiClient.get(`/api/admin/tickets/${ticketId}/shared-orders`)
}

export async function getAdminTicketSharedOrder(ticketId: number, orderId: number) {
  return apiClient.get(`/api/admin/tickets/${ticketId}/shared-orders/${orderId}`)
}

export async function getTicketStats() {
  return apiClient.get('/api/admin/tickets/stats')
}

// 工单附件上传
export async function uploadTicketFile(ticketId: number, file: File) {
  const formData = new FormData()
  formData.append('file', file)
  return apiClient.post(`/api/user/tickets/${ticketId}/upload`, formData, {
    headers: {
      'Content-Type': 'multipart/form-data',
    },
  })
}

export async function uploadAdminTicketFile(ticketId: number, file: File) {
  const formData = new FormData()
  formData.append('file', file)
  return apiClient.post(`/api/admin/tickets/${ticketId}/upload`, formData, {
    headers: {
      'Content-Type': 'multipart/form-data',
    },
  })
}

// ==========================================
// 优惠码 API
// ==========================================

// 管理端 - 优惠码列表
export async function getAdminPromoCodes(params?: {
  page?: number
  limit?: number
  status?: string
  search?: string
}) {
  const query = new URLSearchParams()
  if (params?.page) query.append('page', params.page.toString())
  if (params?.limit) query.append('limit', params.limit.toString())
  if (params?.status) query.append('status', params.status)
  if (params?.search) query.append('search', params.search)

  return apiClient.get(`/api/admin/promo-codes?${query}`)
}

// 管理端 - 获取优惠码详情
export async function getAdminPromoCode(id: number) {
  return apiClient.get(`/api/admin/promo-codes/${id}`)
}

// 管理端 - 创建优惠码
export async function createPromoCode(data: {
  code: string
  name: string
  description?: string
  discount_type: 'percentage' | 'fixed'
  discount_value: number
  max_discount?: number
  min_order_amount?: number
  total_quantity?: number
  product_ids?: number[]
  status?: string
  expires_at?: string
}) {
  return apiClient.post('/api/admin/promo-codes', data)
}

// 管理端 - 更新优惠码
export async function updatePromoCode(id: number, data: {
  code?: string
  name?: string
  description?: string
  discount_type?: 'percentage' | 'fixed'
  discount_value?: number
  max_discount?: number
  min_order_amount?: number
  total_quantity?: number
  product_ids?: number[]
  status?: string
  expires_at?: string
}) {
  return apiClient.put(`/api/admin/promo-codes/${id}`, data)
}

// 管理端 - 删除优惠码
export async function deletePromoCode(id: number) {
  return apiClient.delete(`/api/admin/promo-codes/${id}`)
}

// 用户端 - 验证优惠码
export async function validatePromoCode(data: {
  code: string
  product_ids?: number[]
  amount?: number
}) {
  return apiClient.post('/api/user/promo-codes/validate', data)
}

// ==========================================
// 知识库 API
// ==========================================

export interface KnowledgeCategory {
  id: number
  parent_id?: number
  name: string
  sort_order: number
  children?: KnowledgeCategory[]
  article_count?: number
  total_article_count?: number
  created_at: string
  updated_at: string
}

export interface KnowledgeArticle {
  id: number
  category_id?: number
  category?: KnowledgeCategory
  title: string
  content: string
  sort_order: number
  created_at: string
  updated_at: string
}

// 用户端 - 知识库
export async function getKnowledgeCategoryTree() {
  return apiClient.get('/api/user/knowledge/categories')
}

export async function getKnowledgeArticles(params?: {
  page?: number
  limit?: number
  category_id?: string
  search?: string
}) {
  const query = new URLSearchParams()
  if (params?.page) query.append('page', params.page.toString())
  if (params?.limit) query.append('limit', params.limit.toString())
  if (params?.category_id) query.append('category_id', params.category_id)
  if (params?.search) query.append('search', params.search)
  return apiClient.get(`/api/user/knowledge/articles?${query}`)
}

export async function getKnowledgeArticle(id: number) {
  return apiClient.get(`/api/user/knowledge/articles/${id}`)
}

// 管理端 - 知识库
export async function getAdminKnowledgeCategories() {
  return apiClient.get('/api/admin/knowledge/categories')
}

export async function createKnowledgeCategory(data: { name: string; parent_id?: number; sort_order?: number }) {
  return apiClient.post('/api/admin/knowledge/categories', data)
}

export async function updateKnowledgeCategory(id: number, data: { name?: string; parent_id?: number; sort_order?: number }) {
  return apiClient.put(`/api/admin/knowledge/categories/${id}`, data)
}

export async function deleteKnowledgeCategory(id: number) {
  return apiClient.delete(`/api/admin/knowledge/categories/${id}`)
}

export async function getAdminKnowledgeArticles(params?: {
  page?: number
  limit?: number
  category_id?: string
  search?: string
}) {
  const query = new URLSearchParams()
  if (params?.page) query.append('page', params.page.toString())
  if (params?.limit) query.append('limit', params.limit.toString())
  if (params?.category_id) query.append('category_id', params.category_id)
  if (params?.search) query.append('search', params.search)
  return apiClient.get(`/api/admin/knowledge/articles?${query}`)
}

export async function getAdminKnowledgeArticle(id: number) {
  return apiClient.get(`/api/admin/knowledge/articles/${id}`)
}

export async function createKnowledgeArticle(data: { title: string; content: string; category_id?: number; sort_order?: number }) {
  return apiClient.post('/api/admin/knowledge/articles', data)
}

export async function updateKnowledgeArticle(id: number, data: { title?: string; content?: string; category_id?: number; sort_order?: number }) {
  return apiClient.put(`/api/admin/knowledge/articles/${id}`, data)
}

export async function deleteKnowledgeArticle(id: number) {
  return apiClient.delete(`/api/admin/knowledge/articles/${id}`)
}

// ==========================================
// 公告 API
// ==========================================

export interface Announcement {
  id: number
  title: string
  content: string
  is_mandatory: boolean
  require_full_read: boolean
  created_at: string
  updated_at: string
}

export interface AnnouncementWithRead extends Announcement {
  is_read: boolean
}

// 用户端 - 公告
export async function getAnnouncements(params?: { page?: number; limit?: number }) {
  const query = new URLSearchParams()
  if (params?.page) query.append('page', params.page.toString())
  if (params?.limit) query.append('limit', params.limit.toString())
  return apiClient.get(`/api/user/announcements?${query}`)
}

export async function getAnnouncement(id: number) {
  return apiClient.get(`/api/user/announcements/${id}`)
}

export async function getUnreadMandatoryAnnouncements() {
  return apiClient.get('/api/user/announcements/unread-mandatory')
}

export async function markAnnouncementAsRead(id: number) {
  return apiClient.post(`/api/user/announcements/${id}/read`)
}

// 管理端 - 公告
export async function getAdminAnnouncements(params?: { page?: number; limit?: number }) {
  const query = new URLSearchParams()
  if (params?.page) query.append('page', params.page.toString())
  if (params?.limit) query.append('limit', params.limit.toString())
  return apiClient.get(`/api/admin/announcements?${query}`)
}

export async function getAdminAnnouncement(id: number) {
  return apiClient.get(`/api/admin/announcements/${id}`)
}

export async function createAnnouncement(data: { title: string; content: string; is_mandatory?: boolean; require_full_read?: boolean }) {
  return apiClient.post('/api/admin/announcements', data)
}

export async function updateAnnouncement(id: number, data: { title?: string; content?: string; is_mandatory?: boolean; require_full_read?: boolean }) {
  return apiClient.put(`/api/admin/announcements/${id}`, data)
}

export async function deleteAnnouncement(id: number) {
  return apiClient.delete(`/api/admin/announcements/${id}`)
}
