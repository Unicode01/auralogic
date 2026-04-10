import axios, { AxiosInstance } from 'axios'
import { getToken, clearToken } from './auth'
import {
  getClientAPIProxyBaseURL,
  getConfiguredPublicAPIBaseURL,
  resolveClientAPIProxyURL,
  resolvePublicAPIURL,
} from './api-base-url'
import { stringifyPluginHostContext } from './plugin-frontend-routing'

const API_BASE_URL =
  typeof window === 'undefined' ? getConfiguredPublicAPIBaseURL() : getClientAPIProxyBaseURL()
const APP_LOCALE_STORAGE_KEY = 'auralogic_locale'
const APP_LOCALE_HEADER = 'X-AuraLogic-Locale'

type AnyRecord = Record<string, any>

function normalizeAppLocale(value: unknown): string | undefined {
  const normalized = String(value || '')
    .trim()
    .toLowerCase()
  if (normalized === 'zh' || normalized === 'en') {
    return normalized
  }
  if (normalized.startsWith('zh')) {
    return 'zh'
  }
  if (normalized.startsWith('en')) {
    return 'en'
  }
  return undefined
}

function resolveClientLocaleHeaderValue(): string | undefined {
  if (typeof window === 'undefined') {
    return undefined
  }
  try {
    const stored = normalizeAppLocale(window.localStorage?.getItem(APP_LOCALE_STORAGE_KEY))
    if (stored) {
      return stored
    }
  } catch {
    // ignore storage access failures
  }
  return normalizeAppLocale((window as any).__LOCALE__)
}

function buildLocaleHeaders(locale?: string): Record<string, string> | undefined {
  const resolved = normalizeAppLocale(locale) || resolveClientLocaleHeaderValue()
  if (!resolved) {
    return undefined
  }
  return {
    [APP_LOCALE_HEADER]: resolved,
  }
}

export type ApiErrorInfo = {
  message: string
  code?: number
  status?: number
  errorKey?: string
  errorParams?: Record<string, any>
  data?: any
}

function asRecord(value: unknown): AnyRecord | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return null
  }
  return value as AnyRecord
}

function asString(value: unknown): string {
  if (typeof value !== 'string') return ''
  return value.trim()
}

function asNumber(value: unknown): number | undefined {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string') {
    const parsed = Number(value)
    if (Number.isFinite(parsed)) return parsed
  }
  return undefined
}

function asParams(value: unknown): Record<string, any> | undefined {
  const obj = asRecord(value)
  if (!obj) return undefined
  return obj
}

function readErrorCause(params: Record<string, any> | undefined): string {
  if (!params) return ''
  return (
    asString(params.cause) ||
    asString(params.details) ||
    asString(params.reason) ||
    asString(params.case)
  )
}

function parseApiErrorPayload(payload: unknown): ApiErrorInfo {
  const root = asRecord(payload)
  const rootData = asRecord(root?.data)
  const nestedData = asRecord(rootData?.data)
  const errorParams =
    asParams(root?.error_params) ||
    asParams(root?.params) ||
    asParams(rootData?.params) ||
    asParams(rootData?.error_params) ||
    asParams(nestedData?.params) ||
    asParams(nestedData?.error_params)

  const message =
    asString(root?.message) ||
    asString(root?.error) ||
    asString(rootData?.message) ||
    asString(rootData?.error) ||
    asString(nestedData?.message) ||
    asString(nestedData?.error) ||
    asString(root?.cause) ||
    asString(root?.details) ||
    asString(rootData?.cause) ||
    asString(rootData?.details) ||
    asString(nestedData?.cause) ||
    asString(nestedData?.details) ||
    readErrorCause(errorParams) ||
    'Request failed'

  const errorKey =
    asString(root?.error_key) ||
    asString(rootData?.error_key) ||
    asString(nestedData?.error_key) ||
    undefined

  return {
    message,
    code: asNumber(root?.code),
    status: asNumber(root?.status),
    errorKey,
    errorParams,
    data: root?.data ?? root,
  }
}

export function extractApiErrorInfo(error: unknown): ApiErrorInfo {
  const errObj = asRecord(error)
  const directData = asRecord(errObj?.data)
  const nestedData = asRecord(directData?.data)
  const errorParams =
    asParams(errObj?.errorParams) ||
    asParams(errObj?.error_params) ||
    asParams(errObj?.params) ||
    asParams(directData?.params) ||
    asParams(directData?.error_params) ||
    asParams(nestedData?.params) ||
    asParams(nestedData?.error_params)

  const message =
    asString(errObj?.message) ||
    asString(errObj?.error) ||
    asString(directData?.message) ||
    asString(directData?.error) ||
    asString(nestedData?.message) ||
    asString(nestedData?.error) ||
    asString(errObj?.cause) ||
    asString(errObj?.details) ||
    asString(directData?.cause) ||
    asString(directData?.details) ||
    asString(nestedData?.cause) ||
    asString(nestedData?.details) ||
    readErrorCause(errorParams) ||
    'Request failed'

  const errorKey =
    asString(errObj?.errorKey) ||
    asString(errObj?.error_key) ||
    asString(directData?.error_key) ||
    asString(nestedData?.error_key) ||
    undefined

  const code = asNumber(errObj?.code) ?? asNumber(directData?.code)
  const status =
    asNumber(errObj?.status) ?? asNumber(directData?.status) ?? asNumber(nestedData?.status)

  return {
    message,
    code,
    status,
    errorKey,
    errorParams,
    data: errObj?.data ?? errObj,
  }
}

// 创建axios实例
export const apiClient: AxiosInstance = axios.create({
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
    const locale = resolveClientLocaleHeaderValue()
    if (locale) {
      config.headers[APP_LOCALE_HEADER] = locale
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

    const parsed = parseApiErrorPayload(error.response?.data)
    const fallback = asString(error?.message) || parsed.message || 'Request failed'
    const message = parsed.message || fallback
    const apiError: any = new Error(message)
    apiError.code = parsed.code
    apiError.data = parsed.data
    apiError.status = parsed.status ?? asNumber(error.response?.status)
    apiError.errorKey = parsed.errorKey
    apiError.error_key = parsed.errorKey
    apiError.errorParams = parsed.errorParams
    apiError.error_params = parsed.errorParams
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
  name: string // 库存配置名称
  sku?: string // SKU（可选）
  attributes?: Record<string, string> // 属性组合
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

export async function createProductBinding(
  productId: number,
  data: {
    inventory_id: number
    is_random: boolean
    priority: number
    notes?: string
  }
) {
  return apiClient.post(`/api/admin/products/${productId}/inventory-bindings`, data)
}

export async function batchCreateProductBindings(
  productId: number,
  bindings: Array<{
    inventory_id: number
    is_random: boolean
    priority: number
    notes?: string
  }>
) {
  return apiClient.post(`/api/admin/products/${productId}/inventory-bindings/batch`, {
    bindings,
  })
}

export async function updateProductBinding(
  productId: number,
  bindingId: number,
  data: {
    is_random: boolean
    priority: number
    notes?: string
  }
) {
  return apiClient.put(`/api/admin/products/${productId}/inventory-bindings/${bindingId}`, data)
}

export async function deleteProductBinding(productId: number, bindingId: number) {
  return apiClient.delete(`/api/admin/products/${productId}/inventory-bindings/${bindingId}`)
}

// 替换商品的所有库存绑定（先删除所有，再批量创建）
export async function replaceProductBindings(productId: number, bindings: any[]) {
  return apiClient.put(`/api/admin/products/${productId}/inventory-bindings/replace`, {
    bindings,
  })
}

export async function updateProductInventoryMode(productId: number, mode: 'fixed' | 'random') {
  return apiClient.put(`/api/admin/products/${productId}/inventory-mode`, {
    inventory_mode: mode,
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
  source?: string
  inventory_id?: number
  type?: string
  order_no?: string
  start_date?: string
  end_date?: string
}) {
  const query = new URLSearchParams()
  if (params?.page) query.append('page', params.page.toString())
  if (params?.limit) query.append('limit', params.limit.toString())
  if (params?.source) query.append('source', params.source)
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
  phone_code?: string // 手机区号
  receiver_phone: string
  receiver_email: string
  receiver_country: string // 国家代码
  receiver_province?: string
  receiver_city?: string
  receiver_district?: string
  receiver_address: string
  receiver_postcode?: string
  privacy_protected?: boolean
  password?: string
  user_remark?: string // 用户备注
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

export async function sendPhoneCode(data: {
  phone: string
  phone_code?: string
  captcha_token?: string
}) {
  return apiClient.post('/api/user/auth/send-phone-code', data)
}

export async function loginWithPhoneCode(data: {
  phone: string
  phone_code?: string
  code: string
}) {
  return apiClient.post('/api/user/auth/login-with-phone-code', data)
}

export async function sendPhoneRegisterCode(data: {
  phone: string
  phone_code?: string
  captcha_token?: string
}) {
  return apiClient.post('/api/user/auth/send-phone-register-code', data)
}

export async function phoneRegister(data: {
  phone: string
  phone_code?: string
  name: string
  password: string
  code: string
  captcha_token?: string
}) {
  return apiClient.post('/api/user/auth/phone-register', data)
}

export async function phoneForgotPassword(data: {
  phone: string
  phone_code?: string
  captcha_token?: string
}) {
  return apiClient.post('/api/user/auth/phone-forgot-password', data)
}

export async function phoneResetPassword(data: {
  phone: string
  phone_code?: string
  code: string
  new_password: string
}) {
  return apiClient.post('/api/user/auth/phone-reset-password', data)
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

export async function updateUserPreferences(data: {
  locale?: string
  country?: string
  email_notify_order?: boolean
  email_notify_ticket?: boolean
  email_notify_marketing?: boolean
  sms_notify_marketing?: boolean
}) {
  return apiClient.put('/api/user/auth/preferences', data)
}

export async function sendBindEmailCode(email: string, captcha_token?: string) {
  return apiClient.post('/api/user/auth/send-bind-email-code', { email, captcha_token })
}

export async function bindEmail(email: string, code: string) {
  return apiClient.post('/api/user/auth/bind-email', { email, code })
}

export async function sendBindPhoneCode(
  phone: string,
  phone_code?: string,
  captcha_token?: string
) {
  return apiClient.post('/api/user/auth/send-bind-phone-code', { phone, phone_code, captcha_token })
}

export async function bindPhone(phone: string, code: string) {
  return apiClient.post('/api/user/auth/bind-phone', { phone, code })
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
  product_search?: string // 新增：按商品SKU/名称搜索
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

export async function createOrder(data: { items: any[]; promo_code?: string }) {
  return apiClient.post('/api/user/orders', data)
}

export async function getOrRefreshFormToken(orderNo: string) {
  return apiClient.get(`/api/user/orders/${orderNo}/form-token`)
}

// Get virtual products for an order
export async function getOrderVirtualProducts(orderNo: string) {
  return apiClient.get(`/api/user/orders/${orderNo}/virtual-products`)
}

export async function getInvoiceToken(orderNo: string) {
  return apiClient.get(`/api/user/orders/${orderNo}/invoice-token`)
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
  // Minor units (e.g. cents)
  price_minor: number
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
  // Minor units (e.g. cents)
  total_price_minor: number
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

export async function adminRefundOrder(id: number, reason?: string) {
  return apiClient.post(`/api/admin/orders/${id}/refund`, { reason })
}

export async function adminConfirmRefund(
  id: number,
  data?: { transaction_id?: string; remark?: string }
) {
  return apiClient.post(`/api/admin/orders/${id}/confirm-refund`, data || {})
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

export async function adminDeliverVirtualStock(id: number, data?: { mark_only_shipped?: boolean }) {
  return apiClient.post(`/api/admin/orders/${id}/deliver-virtual`, data || {})
}

export async function updateOrderPrice(id: number, totalAmountMinor: number) {
  return apiClient.put(`/api/admin/orders/${id}/price`, { total_amount_minor: totalAmountMinor })
}

// 用户管理
export async function getUsers(params?: {
  page?: number
  limit?: number
  role?: string
  search?: string
  is_active?: boolean
  email_verified?: boolean
  email_notify_marketing?: boolean
  sms_notify_marketing?: boolean
  has_phone?: boolean
  locale?: string
  country?: string
}) {
  const query = new URLSearchParams()
  if (params?.page) query.append('page', params.page.toString())
  if (params?.limit) query.append('limit', params.limit.toString())
  if (params?.role) query.append('role', params.role)
  if (params?.search) query.append('search', params.search)
  if (params?.is_active !== undefined) query.append('is_active', String(params.is_active))
  if (params?.email_verified !== undefined)
    query.append('email_verified', String(params.email_verified))
  if (params?.email_notify_marketing !== undefined)
    query.append('email_notify_marketing', String(params.email_notify_marketing))
  if (params?.sms_notify_marketing !== undefined)
    query.append('sms_notify_marketing', String(params.sms_notify_marketing))
  if (params?.has_phone !== undefined) query.append('has_phone', String(params.has_phone))
  if (params?.locale) query.append('locale', params.locale)
  if (params?.country) query.append('country', params.country)

  return apiClient.get(`/api/admin/users?${query}`)
}

export async function getUserCountries(params?: { role?: string }) {
  const query = new URLSearchParams()
  if (params?.role) query.append('role', params.role)
  return apiClient.get(`/api/admin/users/countries?${query}`)
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
export async function getAdmins(params?: { page?: number; limit?: number }) {
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

// 插件管理
export interface AdminPluginEffectiveCapabilityPolicy {
  hooks: string[]
  disabled_hooks: string[]
  requested_permissions: string[]
  granted_permissions: string[]
  frontend_min_scope: 'guest' | 'authenticated' | 'super_admin' | string
  frontend_required_permissions: string[]
  frontend_allowed_areas: string[]
  allowed_frontend_slots: string[]
  allow_hook_execute: boolean
  allow_block: boolean
  allow_payload_patch: boolean
  allow_frontend_extensions: boolean
  allow_execute_api: boolean
  allow_network: boolean
  allow_file_system: boolean
  valid: boolean
}

export interface AdminPluginWorkspaceCommand {
  name: string
  title?: string
  description?: string
  entry?: string
  interactive: boolean
  builtin?: boolean
  permissions?: string[]
  missing_permissions?: string[]
  granted: boolean
}

export interface AdminPlugin {
  id: number
  name: string
  display_name?: string
  description?: string
  type: string
  runtime?: string
  address: string
  version?: string
  config?: string
  runtime_params?: string
  capabilities?: string
  manifest?: string
  package_path?: string
  package_path_display?: string
  package_checksum?: string
  address_display?: string
  runtime_spec_hash?: string
  desired_generation?: number
  applied_generation?: number
  enabled: boolean
  status?: string
  lifecycle_status?: string
  last_error?: string
  last_healthy?: string
  installed_at?: string
  started_at?: string
  stopped_at?: string
  retired_at?: string
  fail_count?: number
  created_at?: string
  updated_at?: string
  effective_capability_policy?: AdminPluginEffectiveCapabilityPolicy
  workspace_commands?: AdminPluginWorkspaceCommand[]
  latest_deployment?: AdminPluginDeployment
}

export interface AdminPluginSecretMeta {
  key: string
  configured: boolean
  updated_at?: string
}

export interface AdminPluginExecution {
  id: number
  plugin_id: number
  user_id?: number
  order_id?: number
  action: string
  params?: string
  success: boolean
  result?: string
  error?: string
  duration?: number
  created_at?: string
}

export interface AdminPluginVersion {
  id: number
  plugin_id: number
  version: string
  package_name?: string
  package_path?: string
  package_path_display?: string
  package_checksum?: string
  manifest?: string
  type?: string
  runtime?: string
  address?: string
  config_snapshot?: string
  runtime_params?: string
  capabilities_snapshot?: string
  changelog?: string
  lifecycle_status?: string
  is_active: boolean
  uploaded_by?: number
  activated_at?: string
  created_at?: string
  updated_at?: string
}

export interface AdminPluginDeployment {
  id: number
  plugin_id: number
  operation: string
  trigger?: string
  status: string
  target_version_id?: number
  requested_generation?: number
  applied_generation?: number
  runtime_spec_hash?: string
  auto_start?: boolean
  requested_by?: number
  detail?: string
  error?: string
  started_at?: string
  finished_at?: string
  created_at?: string
  updated_at?: string
}

export interface PluginPermissionRequest {
  key: string
  required: boolean
  reason?: string
  title?: string
  description?: string
  default_granted?: boolean
}

export interface AdminPluginMarketSource {
  source_id: string
  name?: string
  base_url: string
  public_key?: string
  default_channel?: string
  allowed_kinds?: string[]
  enabled?: boolean
}

export interface AdminPluginMarketPreviewRequest {
  source: AdminPluginMarketSource
  kind: string
  name: string
  version: string
}

export interface AdminPluginMarketInstallRequest extends AdminPluginMarketPreviewRequest {
  granted_permissions?: string[]
  activate?: boolean
  auto_start?: boolean
  note?: string
}

export interface AdminPaymentMethodMarketPreviewRequest extends AdminPluginMarketPreviewRequest {
  payment_method_id?: number
}

export interface AdminPaymentMethodMarketImportRequest extends AdminPluginMarketPreviewRequest {
  payment_method_id?: number
  payment_name?: string
  payment_description?: string
  icon?: string
  entry?: string
  config?: string
  poll_interval?: number
}

export type PluginHookGroupKey =
  | 'frontend'
  | 'auth'
  | 'order'
  | 'payment'
  | 'ticket'
  | 'product_inventory'
  | 'promo'
  | 'other'

export interface AdminPluginHookCatalogGroup {
  key: PluginHookGroupKey | string
  hooks: string[]
}

export interface AdminPluginHookCatalog {
  groups: AdminPluginHookCatalogGroup[]
  hooks: string[]
}

export interface PluginObservabilityExecutionCounters {
  total?: number
  success?: number
  failed?: number
  error_rate?: number
  timeout?: number
  timeout_rate?: number
  avg_duration_ms?: number
  max_duration_ms?: number
}

export interface PluginObservabilityPluginExecution extends PluginObservabilityExecutionCounters {
  plugin_id?: number
  plugin_name?: string
  runtime?: string
}

export interface PluginObservabilityExecutionSnapshot {
  overall?: PluginObservabilityExecutionCounters
  by_runtime?: Record<string, PluginObservabilityExecutionCounters>
  by_action?: Record<string, PluginObservabilityExecutionCounters>
  by_plugin?: PluginObservabilityPluginExecution[]
}

export interface PluginObservabilityHookLimiterSnapshot {
  total_hits?: number
  by_hook?: Record<string, number>
}

export interface PluginObservabilityPublicEndpointSnapshot {
  requests?: number
  rate_limited?: number
  rate_limit_hit_rate?: number
  cache_hits?: number
  cache_misses?: number
  cache_hit_rate?: number
}

export interface PluginObservabilityFrontendResolverSnapshot {
  cache_hits?: number
  cache_misses?: number
  cache_hit_rate?: number
  singleflight_waits?: number
  catalog_hits?: number
  db_fallbacks?: number
}

export interface PluginObservabilityFrontendSnapshot {
  slot_requests?: number
  batch_requests?: number
  bootstrap_requests?: number
  batch_items?: number
  batch_unique_items?: number
  batch_deduped_items?: number
  html_mode?: PluginObservabilityFrontendResolverSnapshot
  execute_api?: PluginObservabilityFrontendResolverSnapshot
  prepared_hook?: PluginObservabilityFrontendResolverSnapshot
}

export interface PluginObservabilityExecutionWindowBucket {
  hour_start?: string
  total_executions?: number
  failed_executions?: number
}

export interface PluginObservabilityExecutionFailureGroup {
  plugin_id?: number
  plugin_name?: string
  action?: string
  hook?: string
  failure_count?: number
  last_failure_at?: string
}

export interface PluginObservabilityExecutionFailureSample {
  id?: number
  plugin_id?: number
  plugin_name?: string
  action?: string
  hook?: string
  error?: string
  duration?: number
  created_at?: string
}

export interface PluginObservabilityExecutionHookGroup {
  plugin_id?: number
  plugin_name?: string
  hook?: string
  failure_count?: number
  last_failure_at?: string
  last_error?: string
}

export interface PluginObservabilityExecutionErrorSignature {
  plugin_id?: number
  plugin_name?: string
  signature?: string
  failure_count?: number
  last_failure_at?: string
  sample_error?: string
}

export interface PluginObservabilityExecutionWindow {
  window_hours?: number
  total_executions?: number
  failed_executions?: number
  hook_failed_executions?: number
  action_failed_executions?: number
  last_failure_at?: string
  last_success_at?: string
  by_hour?: PluginObservabilityExecutionWindowBucket[]
  failure_groups?: PluginObservabilityExecutionFailureGroup[]
  hook_groups?: PluginObservabilityExecutionHookGroup[]
  error_signatures?: PluginObservabilityExecutionErrorSignature[]
  recent_failures?: PluginObservabilityExecutionFailureSample[]
}

export interface PluginObservabilityBreakerPlugin {
  plugin_id?: number
  plugin_name?: string
  runtime?: string
  enabled?: boolean
  lifecycle_status?: string
  health_status?: string
  breaker_state?: string
  failure_count?: number
  failure_threshold?: number
  cooldown_active?: boolean
  cooldown_until?: string
  cooldown_reason?: string
  probe_in_flight?: boolean
  probe_started_at?: string
  window_total_executions?: number
  window_failed_executions?: number
}

export interface PluginObservabilityBreakerOverview {
  window_hours?: number
  total_plugins?: number
  enabled_plugins?: number
  open_count?: number
  half_open_count?: number
  closed_count?: number
  cooldown_active_count?: number
  probe_in_flight_count?: number
  rows?: PluginObservabilityBreakerPlugin[]
}

export interface PluginObservabilitySnapshot {
  generated_at?: string
  execution?: PluginObservabilityExecutionSnapshot
  hook_limiter?: PluginObservabilityHookLimiterSnapshot
  public_access?: Record<string, PluginObservabilityPublicEndpointSnapshot>
  frontend?: PluginObservabilityFrontendSnapshot
  breaker_overview?: PluginObservabilityBreakerOverview
  execution_window?: PluginObservabilityExecutionWindow
}

export interface AdminPluginRuntimeInspection {
  configured_runtime?: string
  resolved_runtime?: string
  valid?: boolean
  enabled?: boolean
  lifecycle_status?: string
  health_status?: string
  connection_state?: string
  address_present?: boolean
  package_path_present?: boolean
  ready?: boolean
  breaker_state?: string
  failure_count?: number
  failure_threshold?: number
  cooldown_active?: boolean
  cooldown_until?: string
  cooldown_reason?: string
  probe_in_flight?: boolean
  probe_started_at?: string
  active_generation?: number
  active_in_flight?: number
  draining_slot_count?: number
  draining_in_flight?: number
  draining_generations?: number[]
  last_error?: string
}

export interface AdminPluginProtocolCompatibilityInspection {
  manifest_present?: boolean
  runtime?: string
  host_manifest_version?: string
  manifest_version?: string
  host_protocol_version?: string
  protocol_version?: string
  min_host_protocol_version?: string
  max_host_protocol_version?: string
  compatible?: boolean
  legacy_defaults_applied?: boolean
  reason_code?: string
  reason?: string
}

export interface AdminPluginRegistrationInspection {
  state?: 'success' | 'error' | 'never_attempted' | 'unavailable' | string
  trigger?: string
  runtime?: string
  attempted_at?: string
  completed_at?: string
  duration_ms?: number
  detail?: string
}

export interface AdminPluginHookParticipationDiagnosis {
  hook?: string
  area?: string
  path?: string
  slot?: string
  participates?: boolean
  supports_hook?: boolean
  access_allowed?: boolean
  supports_frontend_area?: boolean
  supports_frontend_slot?: boolean
  allow_block?: boolean
  allow_payload_patch?: boolean
  allow_frontend_extensions?: boolean
  valid_capability_policy?: boolean
  reason_code?: string
  reason?: string
}

export interface AdminPluginDiagnosticCheck {
  key: string
  state: string
  summary: string
  detail?: string
}

export interface AdminPluginDiagnosticIssue {
  code: string
  severity: 'error' | 'warn' | 'info' | string
  summary: string
  detail?: string
  hint?: string
}

export interface AdminPluginFrontendRouteScopeDiagnostic {
  scope: 'guest' | 'authenticated' | 'super_admin' | string
  eligible: boolean
  frontend_visible: boolean
  reason_code?: string
  reason?: string
  diagnostic: AdminPluginHookParticipationDiagnosis
}

export interface AdminPluginFrontendRouteDiagnostic {
  area: 'user' | 'admin' | string
  path: string
  execute_api_available: boolean
  scope_checks: AdminPluginFrontendRouteScopeDiagnostic[]
}

export interface AdminPluginPublicCacheEntryDiagnostic {
  key: string
  created_at?: string
  expires_at?: string
  area?: string
  path?: string
  slot?: string
  extension_count?: number
  menu_count?: number
  route_count?: number
}

export interface AdminPluginPublicCacheBucketDiagnostic {
  total_entries: number
  matching_entries: number
  entries: AdminPluginPublicCacheEntryDiagnostic[]
}

export interface AdminPluginPublicCacheDiagnostics {
  ttl_seconds: number
  max_entries: number
  extensions: AdminPluginPublicCacheBucketDiagnostic
  bootstrap: AdminPluginPublicCacheBucketDiagnostic
}

export interface AdminPluginExecutionTaskSnapshot {
  id: string
  plugin_id: number
  plugin_name?: string
  runtime?: string
  action?: string
  hook?: string
  stream: boolean
  status: 'running' | 'completed' | 'failed' | 'canceled' | 'timed_out' | string
  cancelable: boolean
  started_at: string
  updated_at: string
  completed_at?: string
  duration_ms?: number
  chunk_count?: number
  user_id?: number
  order_id?: number
  session_id?: string
  request_path?: string
  plugin_page_path?: string
  error?: string
  metadata?: Record<string, string>
}

export interface AdminPluginExecutionTaskOverview {
  active_count: number
  recent_count: number
  active: AdminPluginExecutionTaskSnapshot[]
  recent: AdminPluginExecutionTaskSnapshot[]
}

export interface AdminPluginExecutionTaskListResponse {
  plugin_id: number
  status: string
  limit: number
  tasks: AdminPluginExecutionTaskSnapshot[]
  active_count: number
  recent_count: number
  active_tasks: AdminPluginExecutionTaskSnapshot[]
  recent_tasks: AdminPluginExecutionTaskSnapshot[]
}

export interface AdminPluginWorkspaceEntry {
  seq: number
  timestamp?: string
  channel?: string
  level?: string
  message?: string
  source?: string
  action?: string
  hook?: string
  task_id?: string
  metadata?: Record<string, string>
}

export interface AdminPluginWorkspaceControlEvent {
  seq: number
  timestamp?: string
  type?: string
  admin_id?: number
  owner_admin_id?: number
  previous_owner_id?: number
  signal?: string
  result?: string
  message?: string
}

export interface AdminPluginWorkspaceSnapshot {
  owner_admin_id?: number
  viewer_count?: number
  control_granted?: boolean
  status?: string
  active_task_id?: string
  active_command?: string
  prompt?: string
  completion_reason?: string
  last_error?: string
  buffer_capacity: number
  entry_count: number
  last_seq: number
  updated_at?: string
  has_more?: boolean
  recent_control_events?: AdminPluginWorkspaceControlEvent[]
  entries: AdminPluginWorkspaceEntry[]
}

export interface AdminPluginWorkspaceRuntimeState {
  available: boolean
  exists: boolean
  instance_id?: string
  script_path?: string
  loaded?: boolean
  busy?: boolean
  current_action?: string
  last_action?: string
  created_at?: string
  last_used_at?: string
  boot_count?: number
  total_requests?: number
  execute_count?: number
  eval_count?: number
  inspect_count?: number
  last_error?: string
  completion_paths?: string[]
}

export interface AdminPluginWorkspaceResponse {
  workspace?: AdminPluginWorkspaceSnapshot
}

export interface AdminPluginWorkspaceStreamEvent {
  type?: 'snapshot' | 'delta' | 'keepalive' | string
  workspace?: AdminPluginWorkspaceSnapshot
  entries?: AdminPluginWorkspaceEntry[]
  cleared?: boolean
  last_seq?: number
  entry_count?: number
  updated_at?: string
}

export interface AdminPluginWorkspaceWebSocketClientFrame {
  type: 'terminal_line' | 'runtime_eval' | 'runtime_inspect' | 'input' | 'signal' | string
  request_id?: string
  task_id?: string
  input?: string
  line?: string
  depth?: number
  signal?: string
}

export interface AdminPluginWorkspaceWebSocketAck {
  request_id?: string
  action?: 'terminal_line' | 'runtime_eval' | 'runtime_inspect' | 'signal' | string
  success: boolean
  error?: string
  runtime_state?: AdminPluginWorkspaceRuntimeState
  workspace?: AdminPluginWorkspaceSnapshot
}

export interface AdminPluginWorkspaceWebSocketServerFrame {
  type?:
    | 'workspace_snapshot'
    | 'workspace_delta'
    | 'workspace_request_ack'
    | 'workspace_error'
    | string
  event?: AdminPluginWorkspaceStreamEvent
  ack?: AdminPluginWorkspaceWebSocketAck
  message?: string
}

export interface AdminPluginExecutionFailureSample {
  id: number
  action: string
  hook?: string
  error?: string
  duration: number
  created_at?: string
}

export interface AdminPluginExecutionFailureGroup {
  action: string
  hook?: string
  failure_count: number
  last_failure_at?: string
}

export interface AdminPluginExecutionObservability {
  window_hours: number
  total_executions: number
  failed_executions: number
  hook_failed_executions: number
  action_failed_executions: number
  last_failure_at?: string
  last_success_at?: string
  failure_groups: AdminPluginExecutionFailureGroup[]
  recent_failures: AdminPluginExecutionFailureSample[]
}

export interface AdminPluginStorageActionProfileDiagnostic {
  action: string
  mode: 'unknown' | 'none' | 'read' | 'write' | string
}

export interface AdminPluginStorageObservationDiagnostic {
  source: string
  task_id?: string
  action?: string
  hook?: string
  status?: string
  stream: boolean
  declared_access_mode?: 'unknown' | 'none' | 'read' | 'write' | string
  observed_access_mode?: 'unknown' | 'none' | 'read' | 'write' | string
  updated_at?: string
  completed_at?: string
}

export interface AdminPluginStorageDiagnostics {
  profile_count: number
  declared_profiles: AdminPluginStorageActionProfileDiagnostic[]
  last_observed?: AdminPluginStorageObservationDiagnostic
  has_observed_access: boolean
}

export interface AdminPluginDiagnostics {
  plugin: AdminPlugin
  runtime: AdminPluginRuntimeInspection
  compatibility: AdminPluginProtocolCompatibilityInspection
  registration: AdminPluginRegistrationInspection
  recent_deployments: AdminPluginDeployment[]
  execution_tasks: AdminPluginExecutionTaskOverview
  execution_observability: AdminPluginExecutionObservability
  storage_diagnostics: AdminPluginStorageDiagnostics
  public_cache: AdminPluginPublicCacheDiagnostics
  missing_permissions: string[]
  checks: AdminPluginDiagnosticCheck[]
  frontend_routes: AdminPluginFrontendRouteDiagnostic[]
  requested_hook?: AdminPluginHookParticipationDiagnosis
  issues: AdminPluginDiagnosticIssue[]
}

export async function getAdminPlugins() {
  return apiClient.get('/api/admin/plugins')
}

export async function getAdminPluginHookCatalog() {
  return apiClient.get('/api/admin/plugins/hook-catalog')
}

export async function getAdminPlugin(id: number) {
  return apiClient.get(`/api/admin/plugins/${id}`)
}

export async function getAdminPluginDiagnostics(id: number) {
  return apiClient.get(`/api/admin/plugins/${id}/diagnostics`)
}

export async function getAdminPluginWorkspace(id: number, params?: { limit?: number }) {
  const query = new URLSearchParams()
  if (params?.limit) query.append('limit', params.limit.toString())
  query.append('_ts', String(Date.now()))
  const suffix = query.toString()
  return apiClient.get(`/api/admin/plugins/${id}/workspace${suffix ? `?${suffix}` : ''}`)
}

export async function getAdminPluginWorkspaceRuntimeState(id: number) {
  return apiClient.get(`/api/admin/plugins/${id}/workspace/runtime?_ts=${Date.now()}`)
}

export async function clearAdminPluginWorkspace(id: number) {
  return apiClient.post(`/api/admin/plugins/${id}/workspace/clear`)
}

export async function resetAdminPluginWorkspace(id: number) {
  return apiClient.post(`/api/admin/plugins/${id}/workspace/reset`)
}

export async function resetAdminPluginWorkspaceRuntime(id: number) {
  return apiClient.post(`/api/admin/plugins/${id}/workspace/runtime/reset`)
}

export async function claimAdminPluginWorkspaceControl(id: number) {
  return apiClient.post(`/api/admin/plugins/${id}/workspace/control/claim`)
}

export async function submitAdminPluginWorkspaceInput(
  id: number,
  data: {
    task_id?: string
    input: string
  }
) {
  return apiClient.post(`/api/admin/plugins/${id}/workspace/input`, data)
}

export async function enterAdminPluginWorkspaceTerminalLine(
  id: number,
  data: {
    line?: string
    context?: {
      user_id?: number
      order_id?: number
      session_id?: string
      metadata?: Record<string, string>
    }
  }
) {
  return apiClient.post(`/api/admin/plugins/${id}/workspace/terminal`, data)
}

const pluginWorkspaceRuntimeRequestTimeoutMs = 180000

export async function evaluateAdminPluginWorkspaceRuntime(
  id: number,
  data: {
    line?: string
    task_id?: string
    silent?: boolean
    context?: {
      user_id?: number
      order_id?: number
      session_id?: string
      metadata?: Record<string, string>
    }
  }
) {
  return apiClient.post(`/api/admin/plugins/${id}/workspace/runtime/eval`, data, {
    timeout: pluginWorkspaceRuntimeRequestTimeoutMs,
  })
}

export async function inspectAdminPluginWorkspaceRuntime(
  id: number,
  data: {
    line?: string
    depth?: number
    task_id?: string
    silent?: boolean
    context?: {
      user_id?: number
      order_id?: number
      session_id?: string
      metadata?: Record<string, string>
    }
  }
) {
  return apiClient.post(`/api/admin/plugins/${id}/workspace/runtime/inspect`, data, {
    timeout: pluginWorkspaceRuntimeRequestTimeoutMs,
  })
}

export async function signalAdminPluginWorkspace(
  id: number,
  data: {
    task_id?: string
    signal?: 'interrupt' | 'terminate' | string
  }
) {
  return apiClient.post(`/api/admin/plugins/${id}/workspace/signal`, data)
}

export async function streamAdminPluginWorkspace(
  id: number,
  options?: {
    limit?: number
    signal?: AbortSignal
    locale?: string
    onEvent?: (event: AdminPluginWorkspaceStreamEvent) => void | Promise<void>
  }
) {
  const query = new URLSearchParams()
  if (options?.limit) query.append('limit', String(options.limit))
  const headers = new Headers({
    Accept: 'application/x-ndjson',
  })
  const token = getToken()
  if (token) {
    headers.set('Authorization', `Bearer ${token}`)
  }
  const locale = normalizeAppLocale(options?.locale) || resolveClientLocaleHeaderValue()
  if (locale) {
    headers.set(APP_LOCALE_HEADER, locale)
  }

  const response = await fetch(
    resolveFetchAPIURL(
      `/api/admin/plugins/${id}/workspace/stream${query.toString() ? `?${query}` : ''}`
    ),
    {
      method: 'GET',
      headers,
      signal: options?.signal,
      credentials: 'same-origin',
    }
  )

  const contentType = response.headers.get('content-type') || ''
  if (contentType.includes('application/json')) {
    const payload = await response.json().catch(() => ({}))
    if (!response.ok) {
      throw createAPIErrorFromPayload(payload, response.statusText || 'Request failed')
    }
    return payload
  }
  if (!response.ok) {
    const payload = await response.text().catch(() => '')
    throw createAPIErrorFromPayload(
      { message: payload || response.statusText },
      response.statusText || 'Request failed'
    )
  }
  if (!response.body) {
    throw new Error('Workspace stream response body is unavailable')
  }

  const decoder = new TextDecoder()
  const reader = response.body.getReader()
  let buffer = ''

  const parseLine = async (line: string) => {
    const trimmed = line.trim()
    if (!trimmed) return
    let parsed: PluginRouteStreamChunk
    try {
      parsed = JSON.parse(trimmed) as PluginRouteStreamChunk
    } catch {
      throw new Error('Invalid workspace stream response chunk')
    }
    const root = asRecord(parsed?.data)
    const event = asRecord(root?.event) as AdminPluginWorkspaceStreamEvent | null
    if (event && options?.onEvent) {
      await options.onEvent(event)
    }
  }

  for (;;) {
    const { value, done } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })
    const lines = buffer.split(/\r?\n/)
    buffer = lines.pop() || ''
    for (const line of lines) {
      await parseLine(line)
    }
  }
  buffer += decoder.decode()
  if (buffer.trim() !== '') {
    await parseLine(buffer)
  }
}

export function resolveAdminPluginWorkspaceWebSocketURL(
  id: number,
  options?: {
    limit?: number
    locale?: string
  }
) {
  const query = new URLSearchParams()
  if (options?.limit) query.append('limit', String(options.limit))
  const locale = normalizeAppLocale(options?.locale) || resolveClientLocaleHeaderValue()
  if (locale) {
    query.append('locale', locale)
  }

  const suffix = query.toString()
  const absoluteURL = resolvePublicAPIURL(
    `/api/admin/plugins/${id}/workspace/ws${suffix ? `?${suffix}` : ''}`
  )
  const base =
    typeof window !== 'undefined' && window.location?.origin
      ? window.location.origin
      : 'http://localhost'
  const url = new URL(absoluteURL, base)
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
  return url.toString()
}

export function resolveAdminPluginWorkspaceWebSocketProtocols(): string[] {
  const protocols = ['auralogic.workspace.v1']
  const token = getToken()
  if (token) {
    protocols.push(`auralogic.auth.bearer.${token}`)
  }
  return protocols
}

export async function createAdminPlugin(data: Partial<AdminPlugin>) {
  return apiClient.post('/api/admin/plugins', data)
}

export async function updateAdminPlugin(id: number, data: Partial<AdminPlugin>) {
  return apiClient.put(`/api/admin/plugins/${id}`, data)
}

export async function getAdminPluginSecrets(id: number) {
  return apiClient.get(`/api/admin/plugins/${id}/secrets`)
}

export async function updateAdminPluginSecrets(
  id: number,
  data: { upserts?: Record<string, string>; delete_keys?: string[] }
) {
  return apiClient.put(`/api/admin/plugins/${id}/secrets`, data)
}

export async function deleteAdminPlugin(id: number) {
  return apiClient.delete(`/api/admin/plugins/${id}`)
}

export async function testAdminPlugin(id: number) {
  return apiClient.post(`/api/admin/plugins/${id}/test`)
}

export interface PluginRouteExecuteRequest {
  action: string
  params?: Record<string, string>
  path?: string
  query_params?: Record<string, string>
  route_params?: Record<string, string>
}

export interface PluginFrontendRouteExecuteAPI {
  url?: string
  method?: string
  scope?: 'public' | 'admin' | string
  requires_auth?: boolean
  path_param?: string
  action_param?: string
  params_format?: string
  allowed_actions?: string[]
  stream_url?: string
  stream_format?: string
  stream_actions?: string[]
}

export interface PluginRouteStreamChunk {
  type?: 'chunk' | 'error' | 'task' | string
  index?: number
  task_id?: string
  task_status_url?: string
  task_cancel_url?: string
  success?: boolean
  data?: Record<string, any>
  error?: string
  metadata?: Record<string, string>
  is_final?: boolean
}

function normalizePluginExecuteActionList(value: unknown): string[] {
  if (!Array.isArray(value)) {
    return []
  }
  return value
    .map((item) =>
      String(item || '')
        .trim()
        .toLowerCase()
    )
    .filter((item, index, source) => !!item && source.indexOf(item) === index)
}

function createAPIErrorFromPayload(payload: unknown, fallbackMessage: string): any {
  const parsed = parseApiErrorPayload(payload)
  const message = parsed.message || fallbackMessage || 'Request failed'
  const apiError: any = new Error(message)
  apiError.code = parsed.code
  apiError.data = parsed.data
  apiError.status = parsed.status
  apiError.errorKey = parsed.errorKey
  apiError.error_key = parsed.errorKey
  apiError.errorParams = parsed.errorParams
  apiError.error_params = parsed.errorParams
  return apiError
}

function resolveFetchAPIURL(url: string): string {
  return resolveClientAPIProxyURL(url)
}

export function pluginRouteShouldStream(
  executeAPI: PluginFrontendRouteExecuteAPI,
  action: string,
  preferredMode?: string
): boolean {
  const normalizedAction = String(action || '')
    .trim()
    .toLowerCase()
  if (!normalizedAction) {
    return false
  }
  const streamURL = typeof executeAPI?.stream_url === 'string' ? executeAPI.stream_url.trim() : ''
  if (!streamURL) {
    return false
  }
  const streamActions = normalizePluginExecuteActionList(executeAPI?.stream_actions)
  const explicitMode = String(preferredMode || '')
    .trim()
    .toLowerCase()
  if (explicitMode === 'stream') {
    return streamActions.length === 0 || streamActions.includes(normalizedAction)
  }
  return streamActions.includes(normalizedAction)
}

export async function executePluginRouteAction(
  executeAPI: PluginFrontendRouteExecuteAPI,
  data: PluginRouteExecuteRequest,
  options?: {
    locale?: string
  }
) {
  const url = typeof executeAPI?.url === 'string' ? executeAPI.url.trim() : ''
  if (!url) {
    throw new Error('Plugin execute API URL is missing')
  }
  const method =
    typeof executeAPI?.method === 'string' && executeAPI.method.trim() !== ''
      ? executeAPI.method.trim().toLowerCase()
      : 'post'
  const action = String(data?.action || '').trim()
  const allowedActions = normalizePluginExecuteActionList(executeAPI?.allowed_actions)
  if (allowedActions.length > 0 && !allowedActions.includes(action.toLowerCase())) {
    throw new Error('Plugin execute action is not declared for this page route')
  }
  return apiClient.request({
    url,
    method: method as any,
    data,
    headers: buildLocaleHeaders(options?.locale),
  })
}

export async function executePluginRouteActionStream(
  executeAPI: PluginFrontendRouteExecuteAPI,
  data: PluginRouteExecuteRequest,
  options?: {
    signal?: AbortSignal
    onChunk?: (chunk: PluginRouteStreamChunk) => void | Promise<void>
    locale?: string
  }
) {
  const url = typeof executeAPI?.stream_url === 'string' ? executeAPI.stream_url.trim() : ''
  if (!url) {
    throw new Error('Plugin execute stream API URL is missing')
  }
  const action = String(data?.action || '').trim()
  const allowedActions = normalizePluginExecuteActionList(executeAPI?.allowed_actions)
  if (allowedActions.length > 0 && !allowedActions.includes(action.toLowerCase())) {
    throw new Error('Plugin execute action is not declared for this page route')
  }
  const streamActions = normalizePluginExecuteActionList(executeAPI?.stream_actions)
  if (streamActions.length > 0 && !streamActions.includes(action.toLowerCase())) {
    throw new Error('Plugin stream action is not declared for this page route')
  }

  const headers = new Headers({
    Accept:
      typeof executeAPI?.stream_format === 'string' && executeAPI.stream_format.trim() !== ''
        ? `application/x-${executeAPI.stream_format.trim().toLowerCase()}`
        : 'application/x-ndjson',
    'Content-Type': 'application/json',
  })
  const token = getToken()
  if (token) {
    headers.set('Authorization', `Bearer ${token}`)
  }
  const locale = resolveClientLocaleHeaderValue()
  const requestedLocale = normalizeAppLocale(options?.locale) || locale
  if (requestedLocale) {
    headers.set(APP_LOCALE_HEADER, requestedLocale)
  }

  const response = await fetch(resolveFetchAPIURL(url), {
    method: 'POST',
    headers,
    body: JSON.stringify(data || {}),
    signal: options?.signal,
    credentials: 'same-origin',
  })

  const contentType = response.headers.get('content-type') || ''
  if (contentType.includes('application/json')) {
    const payload = await response.json().catch(() => ({}))
    if (!response.ok) {
      throw createAPIErrorFromPayload(payload, response.statusText || 'Request failed')
    }
    return payload
  }
  if (!response.ok) {
    const payload = await response.text().catch(() => '')
    throw createAPIErrorFromPayload(
      { message: payload || response.statusText },
      response.statusText || 'Request failed'
    )
  }
  if (!response.body) {
    throw new Error('Streaming response body is unavailable')
  }

  const decoder = new TextDecoder()
  const reader = response.body.getReader()
  const chunks: PluginRouteStreamChunk[] = []
  let buffer = ''

  const emitChunk = async (chunk: PluginRouteStreamChunk) => {
    chunks.push(chunk)
    if (options?.onChunk) {
      await options.onChunk(chunk)
    }
  }

  const parseLine = async (line: string) => {
    const trimmed = line.trim()
    if (!trimmed) {
      return
    }
    let parsed: PluginRouteStreamChunk
    try {
      parsed = JSON.parse(trimmed) as PluginRouteStreamChunk
    } catch {
      throw new Error('Invalid plugin stream response chunk')
    }
    await emitChunk(parsed)
  }

  for (;;) {
    const { value, done } = await reader.read()
    if (done) {
      break
    }
    buffer += decoder.decode(value, { stream: true })
    const lines = buffer.split(/\r?\n/)
    buffer = lines.pop() || ''
    for (const line of lines) {
      await parseLine(line)
    }
  }
  buffer += decoder.decode()
  if (buffer.trim() !== '') {
    await parseLine(buffer)
  }

  const finalChunk =
    [...chunks].reverse().find((chunk) => chunk?.is_final) || chunks[chunks.length - 1]
  if (!finalChunk) {
    throw new Error('Plugin stream returned no chunks')
  }

  return {
    success: finalChunk.success !== false,
    task_id: finalChunk.task_id,
    task_status_url: finalChunk.task_status_url,
    task_cancel_url: finalChunk.task_cancel_url,
    data: finalChunk.data,
    metadata: finalChunk.metadata,
    error: finalChunk.error,
  }
}

export async function getAdminPluginExecutions(id: number) {
  return apiClient.get(`/api/admin/plugins/${id}/executions`)
}

export async function getAdminPluginExecutionTasks(
  id: number,
  params?: {
    status?: string
    limit?: number
  }
) {
  return apiClient.get(`/api/admin/plugins/${id}/tasks`, {
    params,
  })
}

export async function getAdminPluginExecutionTask(id: number, taskId: string) {
  return apiClient.get(`/api/admin/plugins/${id}/tasks/${encodeURIComponent(taskId)}`)
}

export async function cancelAdminPluginExecutionTask(id: number, taskId: string) {
  return apiClient.post(`/api/admin/plugins/${id}/tasks/${encodeURIComponent(taskId)}/cancel`)
}

export async function uploadAdminPluginPackage(data: FormData) {
  return apiClient.post('/api/admin/plugins/upload', data, {
    headers: {
      'Content-Type': 'multipart/form-data',
    },
  })
}

export async function previewAdminPluginPackage(data: FormData) {
  return apiClient.post('/api/admin/plugins/upload/preview', data, {
    headers: {
      'Content-Type': 'multipart/form-data',
    },
  })
}

export async function previewAdminPluginMarketInstall(data: AdminPluginMarketPreviewRequest) {
  return apiClient.post('/api/admin/plugins/market/preview', data)
}

export async function installAdminPluginFromMarket(data: AdminPluginMarketInstallRequest) {
  return apiClient.post('/api/admin/plugins/market/install', data)
}

export async function pluginLifecycleAction(
  id: number,
  data: {
    action: string
    version_id?: number
    auto_start?: boolean
  }
) {
  return apiClient.post(`/api/admin/plugins/${id}/lifecycle`, data)
}

export async function getAdminPluginVersions(id: number) {
  return apiClient.get(`/api/admin/plugins/${id}/versions`)
}

export async function deleteAdminPluginVersion(id: number, versionId: number) {
  return apiClient.delete(`/api/admin/plugins/${id}/versions/${versionId}`)
}

export async function activateAdminPluginVersion(
  id: number,
  versionId: number,
  data?: {
    auto_start?: boolean
  }
) {
  return apiClient.post(`/api/admin/plugins/${id}/versions/${versionId}/activate`, data || {})
}

export async function getAdminPluginObservability(params?: { plugin_id?: number; hours?: number }) {
  const query = new URLSearchParams()
  if (params?.plugin_id) query.append('plugin_id', String(params.plugin_id))
  if (params?.hours) query.append('hours', String(params.hours))
  const suffix = query.toString()
  return apiClient.get(`/api/admin/plugins/observability${suffix ? `?${suffix}` : ''}`)
}

// 系统日志
export async function getOperationLogs(params?: {
  page?: number
  limit?: number
  action?: string
  resource_type?: string
  resource_id?: string
  order_no?: string
  user_id?: string
  start_date?: string
  end_date?: string
}) {
  const query = new URLSearchParams()
  if (params?.page) query.append('page', params.page.toString())
  if (params?.limit) query.append('limit', params.limit.toString())
  if (params?.action) query.append('action', params.action)
  if (params?.resource_type) query.append('resource_type', params.resource_type)
  if (params?.resource_id) query.append('resource_id', params.resource_id)
  if (params?.order_no) query.append('order_no', params.order_no)
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

export async function getSmsLogs(params?: {
  page?: number
  limit?: number
  status?: string
  event_type?: string
  phone?: string
  start_date?: string
  end_date?: string
}) {
  const query = new URLSearchParams()
  if (params?.page) query.append('page', params.page.toString())
  if (params?.limit) query.append('limit', params.limit.toString())
  if (params?.status) query.append('status', params.status)
  if (params?.event_type) query.append('event_type', params.event_type)
  if (params?.phone) query.append('phone', params.phone)
  if (params?.start_date) query.append('start_date', params.start_date)
  if (params?.end_date) query.append('end_date', params.end_date)

  return apiClient.get(`/api/admin/logs/sms?${query}`)
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
export async function importVirtualStock(
  productId: number,
  data: {
    import_type: 'file' | 'text'
    file?: File
    content?: string
  }
) {
  const formData = new FormData()
  formData.append('import_type', data.import_type)

  if (data.import_type === 'file' && data.file) {
    formData.append('file', data.file)
  } else if (data.import_type === 'text' && data.content) {
    formData.append('content', data.content)
  }

  return apiClient.post(`/api/admin/virtual-products/${productId}/import`, formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
}

// Get virtual product stock list
export async function getVirtualStockList(
  productId: number,
  params?: {
    page?: number
    limit?: number
    status?: string
  }
) {
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
    data: { batch_no: batchNo },
  })
}

// ==================== Virtual Inventory (New API) ====================

// Virtual Inventory interface
export interface VirtualInventory {
  id: number
  name: string
  sku: string
  type: 'static' | 'script'
  script: string
  script_config: string
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
  type?: string
  script?: string
  script_config?: string
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
export async function updateVirtualInventory(
  id: number,
  data: {
    name?: string
    sku?: string
    type?: string
    script?: string
    script_config?: string
    description?: string
    is_active?: boolean
    notes?: string
  }
) {
  return apiClient.put(`/api/admin/virtual-inventories/${id}`, data)
}

// Delete virtual inventory
export async function deleteVirtualInventory(id: number) {
  return apiClient.delete(`/api/admin/virtual-inventories/${id}`)
}

// Import stock to virtual inventory
export async function importVirtualInventoryStock(
  virtualInventoryId: number,
  data: {
    import_type: 'file' | 'text'
    file?: File
    content?: string
  }
) {
  const formData = new FormData()
  formData.append('import_type', data.import_type)

  if (data.import_type === 'file' && data.file) {
    formData.append('file', data.file)
  } else if (data.import_type === 'text' && data.content) {
    formData.append('content', data.content)
  }

  return apiClient.post(`/api/admin/virtual-inventories/${virtualInventoryId}/import`, formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
}

// Create stock manually in virtual inventory
export async function createVirtualInventoryStockManually(
  virtualInventoryId: number,
  data: {
    content: string
    remark?: string
  }
) {
  return apiClient.post(`/api/admin/virtual-inventories/${virtualInventoryId}/stocks`, data)
}

// Get virtual inventory stock list
export async function getVirtualInventoryStockList(
  virtualInventoryId: number,
  params?: {
    page?: number
    limit?: number
    status?: string
  }
) {
  const query = new URLSearchParams()
  if (params?.page) query.append('page', params.page.toString())
  if (params?.limit) query.append('limit', params.limit.toString())
  if (params?.status) query.append('status', params.status)

  return apiClient.get(`/api/admin/virtual-inventories/${virtualInventoryId}/stocks?${query}`)
}

// Alias for getVirtualInventoryStockList
export const getVirtualInventoryStocks = getVirtualInventoryStockList

// Delete virtual inventory stock item
export async function deleteVirtualInventoryStock(virtualInventoryId: number, stockId: number) {
  return apiClient.delete(`/api/admin/virtual-inventories/${virtualInventoryId}/stocks/${stockId}`)
}

// Reserve virtual inventory stock item (manual)
export async function reserveVirtualInventoryStock(
  virtualInventoryId: number,
  stockId: number,
  remark?: string
) {
  return apiClient.post(
    `/api/admin/virtual-inventories/${virtualInventoryId}/stocks/${stockId}/reserve`,
    { remark }
  )
}

// Release virtual inventory stock item (manual)
export async function releaseVirtualInventoryStock(virtualInventoryId: number, stockId: number) {
  return apiClient.post(
    `/api/admin/virtual-inventories/${virtualInventoryId}/stocks/${stockId}/release`
  )
}

// Test delivery script
export async function testDeliveryScript(
  script: string,
  config?: Record<string, any>,
  quantity?: number
) {
  return apiClient.post('/api/admin/virtual-inventories/test-script', { script, config, quantity })
}

// ==================== Product Virtual Inventory Bindings ====================

// Get product virtual inventory bindings
export async function getProductVirtualInventoryBindings(productId: number) {
  return apiClient.get(`/api/admin/products/${productId}/virtual-inventory-bindings`)
}

// Create product virtual inventory binding
export async function createProductVirtualInventoryBinding(
  productId: number,
  data: {
    virtual_inventory_id: number
    is_random?: boolean
    priority?: number
    notes?: string
  }
) {
  return apiClient.post(`/api/admin/products/${productId}/virtual-inventory-bindings`, data)
}

// Delete product virtual inventory binding
export async function deleteProductVirtualInventoryBinding(productId: number, bindingId: number) {
  return apiClient.delete(
    `/api/admin/products/${productId}/virtual-inventory-bindings/${bindingId}`
  )
}

// Save product virtual inventory variant bindings (batch save)
export async function saveProductVirtualVariantBindings(
  productId: number,
  bindings: Array<{
    attributes: Record<string, string>
    virtual_inventory_id: number | null
    is_random?: boolean
    priority?: number
  }>
) {
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

export async function testSMS(data: { phone: string }) {
  return apiClient.post('/api/admin/settings/sms/test', data)
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

export async function importAdminTemplatePackage(
  file: File,
  expectedKind?: string,
  targetKey?: string
) {
  const formData = new FormData()
  formData.append('file', file)
  if (expectedKind) {
    formData.append('expected_kind', expectedKind)
  }
  if (targetKey) {
    formData.append('target_key', targetKey)
  }
  return apiClient.post('/api/admin/settings/template-packages/import', formData, {
    headers: {
      'Content-Type': 'multipart/form-data',
    },
  })
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

// 公开配置（无需登录）
export async function getPublicConfig() {
  return apiClient.get('/api/config/public')
}

// 获取页面注入脚本/样式（无需登录，通过path参数穿透CDN）
export async function getPageInject(path: string) {
  return apiClient.get(`/api/config/page-inject?path=${encodeURIComponent(path)}`)
}

export interface PluginFrontendExtension {
  id?: string
  slot?: string
  type?: string
  title?: LocalizedTextValue
  content?: LocalizedTextValue
  link?: string
  priority?: number
  data?: Record<string, any>
  metadata?: Record<string, string>
  plugin_id?: number
  plugin_name?: string
}

export interface PluginFrontendExtensionBatchRequestItem {
  key?: string
  slot: string
  path?: string
  query_params?: Record<string, string>
  host_context?: Record<string, any>
}

export interface PluginFrontendExtensionBatchResponseItem {
  key?: string
  slot?: string
  path?: string
  extensions?: PluginFrontendExtension[]
}

export interface PluginFrontendBootstrapMenuItem {
  id?: string
  area?: 'user' | 'admin' | string
  title?: LocalizedTextValue
  path?: string
  icon?: string
  priority?: number
  required_permissions?: string[]
  super_admin_only?: boolean
  guest_visible?: boolean
  mobile_visible?: boolean
  plugin_id?: number
  plugin_name?: string
}

export interface PluginFrontendBootstrapRoute {
  id?: string
  area?: 'user' | 'admin' | string
  title?: LocalizedTextValue
  path?: string
  route_params?: Record<string, string>
  priority?: number
  required_permissions?: string[]
  super_admin_only?: boolean
  guest_visible?: boolean
  html_mode?: 'sanitize' | 'trusted' | string
  page?: Record<string, any>
  execute_api?: PluginFrontendRouteExecuteAPI
  plugin_id?: number
  plugin_name?: string
}

function appendPluginQueryParams(query: URLSearchParams, queryParams?: Record<string, string>) {
  if (!queryParams || Object.keys(queryParams).length === 0) {
    return
  }
  query.append('query_params', JSON.stringify(queryParams))
}

function appendPluginHostContext(query: URLSearchParams, hostContext?: Record<string, any>) {
  const serialized = stringifyPluginHostContext(hostContext)
  if (serialized === '{}') {
    return
  }
  query.append('host_context', serialized)
}

// 获取指定页面插槽的插件扩展（无需登录）
export async function getPluginExtensions(
  path: string,
  slot: string,
  queryParams?: Record<string, string>,
  hostContext?: Record<string, any>,
  signal?: AbortSignal,
  locale?: string
) {
  const query = new URLSearchParams()
  query.append('path', path || '/')
  query.append('slot', slot || 'default')
  appendPluginQueryParams(query, queryParams)
  appendPluginHostContext(query, hostContext)
  return apiClient.get(`/api/config/plugin-extensions?${query.toString()}`, {
    signal,
    headers: buildLocaleHeaders(locale),
  })
}

// 获取管理端页面插槽的插件扩展（需管理员登录）
export async function getAdminPluginExtensions(
  path: string,
  slot: string,
  queryParams?: Record<string, string>,
  hostContext?: Record<string, any>,
  signal?: AbortSignal,
  locale?: string
) {
  const query = new URLSearchParams()
  query.append('path', path || '/admin')
  query.append('slot', slot || 'default')
  appendPluginQueryParams(query, queryParams)
  appendPluginHostContext(query, hostContext)
  return apiClient.get(`/api/admin/plugin-extensions?${query.toString()}`, {
    signal,
    headers: buildLocaleHeaders(locale),
  })
}

export async function getPluginExtensionsBatch(
  path: string,
  items: PluginFrontendExtensionBatchRequestItem[],
  signal?: AbortSignal,
  locale?: string
) {
  return apiClient.post(
    '/api/config/plugin-extensions/batch',
    {
      path: path || '/',
      items,
    },
    {
      signal,
      headers: buildLocaleHeaders(locale),
    }
  )
}

export async function getAdminPluginExtensionsBatch(
  path: string,
  items: PluginFrontendExtensionBatchRequestItem[],
  signal?: AbortSignal,
  locale?: string
) {
  return apiClient.post(
    '/api/admin/plugin-extensions/batch',
    {
      path: path || '/admin',
      items,
    },
    {
      signal,
      headers: buildLocaleHeaders(locale),
    }
  )
}

// 获取用户端前端插件 bootstrap（无需登录）
export async function getPluginFrontendBootstrap(
  path: string,
  queryParams?: Record<string, string>,
  signal?: AbortSignal,
  locale?: string
) {
  const query = new URLSearchParams()
  query.append('path', path || '/')
  appendPluginQueryParams(query, queryParams)
  return apiClient.get(`/api/config/plugin-bootstrap?${query.toString()}`, {
    signal,
    headers: buildLocaleHeaders(locale),
  })
}

// 获取管理端前端插件 bootstrap（需管理员登录）
export async function getAdminPluginFrontendBootstrap(
  path: string,
  queryParams?: Record<string, string>,
  signal?: AbortSignal,
  locale?: string
) {
  const query = new URLSearchParams()
  query.append('path', path || '/admin')
  appendPluginQueryParams(query, queryParams)
  return apiClient.get(`/api/admin/plugin-bootstrap?${query.toString()}`, {
    signal,
    headers: buildLocaleHeaders(locale),
  })
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
  version?: string
  package_name?: string
  package_entry?: string
  package_checksum?: string
  manifest?: string
  sort_order: number
  poll_interval: number
  created_at: string
  updated_at: string
}

export type LocalizedTextValue = string | Record<string, unknown>

export interface PaymentMethodPackageWebhookManifest {
  key: string
  description?: LocalizedTextValue
  action?: string
  method?: string
  auth_mode?: string
  secret_key?: string
  header?: string
  query_param?: string
  signature_header?: string
}

export interface PaymentMethodPackageManifest {
  name?: string
  display_name?: LocalizedTextValue
  description?: LocalizedTextValue
  icon?: string
  address?: string
  entry?: string
  version?: string
  poll_interval?: number
  config?: Record<string, any>
  config_schema?: Record<string, any>
  webhooks?: PaymentMethodPackageWebhookManifest[]
}

export interface PaymentMethodPackageResolvedPreview {
  name: string
  description: string
  version: string
  entry: string
  icon: string
  config: string
  poll_interval: number
  script_bytes: number
  checksum: string
}

export interface PaymentMethodPackagePreview {
  manifest?: PaymentMethodPackageManifest
  resolved: PaymentMethodPackageResolvedPreview
}

export interface PaymentMethodMarketPreview extends PaymentMethodPackagePreview {
  source: AdminPluginMarketSource
  coordinates: {
    source_id: string
    kind: string
    name: string
    version: string
  }
  release?: Record<string, any>
  compatibility?: Record<string, any>
  target_state?: Record<string, any>
  warnings?: string[]
}

export interface PaymentMethodMarketCatalogItem {
  kind?: string
  name: string
  title?: LocalizedTextValue
  description?: LocalizedTextValue
  summary?: LocalizedTextValue
  latest_version?: string
  channels?: string[]
}

export interface PaymentMethodMarketArtifactVersion {
  version: string
  channel?: string
  published_at?: string
}

export interface PaymentMethodMarketArtifact {
  source: AdminPluginMarketSource
  kind?: string
  name: string
  title?: LocalizedTextValue
  description?: LocalizedTextValue
  summary?: LocalizedTextValue
  latest_version?: string
  channels?: string[]
  versions?: PaymentMethodMarketArtifactVersion[]
  governance?: Record<string, any>
}

export interface PaymentCardResult {
  html: string
  title?: LocalizedTextValue
  description?: LocalizedTextValue
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

export async function previewPaymentMethodPackage(formData: FormData) {
  return apiClient.post('/api/admin/payment-methods/preview-package', formData, {
    headers: {
      'Content-Type': 'multipart/form-data',
    },
  })
}

export async function previewPaymentMethodMarketPackage(
  data: AdminPaymentMethodMarketPreviewRequest
) {
  return apiClient.post('/api/admin/payment-methods/market/preview', data)
}

export async function getPaymentMethodMarketSources() {
  return apiClient.get('/api/admin/payment-methods/market/sources')
}

export async function getPaymentMethodMarketCatalog(params?: {
  source_id?: string
  channel?: string
  q?: string
  limit?: number
  offset?: number
}) {
  return apiClient.get('/api/admin/payment-methods/market/catalog', { params })
}

export async function getPaymentMethodMarketArtifact(
  name: string,
  params?: {
    source_id?: string
  }
) {
  return apiClient.get(`/api/admin/payment-methods/market/artifacts/${encodeURIComponent(name)}`, {
    params,
  })
}

export async function uploadPaymentMethodPackage(formData: FormData) {
  return apiClient.post('/api/admin/payment-methods/upload-package', formData, {
    headers: {
      'Content-Type': 'multipart/form-data',
    },
  })
}

export async function importPaymentMethodPackageFromMarket(
  data: AdminPaymentMethodMarketImportRequest
) {
  return apiClient.post('/api/admin/payment-methods/market/import', data)
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

export async function selectOrderPaymentMethod(orderNo: string, paymentMethodId: number) {
  return apiClient.post(`/api/user/orders/${orderNo}/select-payment`, {
    payment_method_id: paymentMethodId,
  })
}

// ==========================================
// 市场平台 API
// ==========================================

export interface MarketPagination {
  page: number
  limit: number
  total: number
  total_pages: number
  has_next: boolean
  has_prev: boolean
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

export async function sendTicketMessage(
  id: number,
  data: {
    content: string
    content_type?: string
  }
) {
  return apiClient.post(`/api/user/tickets/${id}/messages`, data)
}

export async function updateTicketStatus(id: number, status: string) {
  return apiClient.put(`/api/user/tickets/${id}/status`, { status })
}

export async function shareOrderToTicket(
  ticketId: number,
  data: {
    order_id: number
    can_edit?: boolean
    can_view_privacy?: boolean
  }
) {
  return apiClient.post(`/api/user/tickets/${ticketId}/share-order`, data)
}

export async function getTicketSharedOrders(ticketId: number) {
  return apiClient.get(`/api/user/tickets/${ticketId}/shared-orders`)
}

// 管理端工单 API
export async function getAdminTickets(params?: {
  page?: number
  limit?: number
  status?: string
  exclude_status?: string
  search?: string
  assigned_to?: string
}) {
  const query = new URLSearchParams()
  if (params?.page) query.append('page', params.page.toString())
  if (params?.limit) query.append('limit', params.limit.toString())
  if (params?.status) query.append('status', params.status)
  if (params?.exclude_status) query.append('exclude_status', params.exclude_status)
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

export async function sendAdminTicketMessage(
  id: number,
  data: {
    content: string
    content_type?: string
  }
) {
  return apiClient.post(`/api/admin/tickets/${id}/messages`, data)
}

export async function updateAdminTicket(
  id: number,
  data: {
    status?: string
    priority?: string
    assigned_to?: number
  }
) {
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
  // fixed: minor units, percentage: basis points (10000 = 100%)
  discount_value_minor: number
  // minor units
  max_discount_minor?: number
  // minor units
  min_order_amount_minor?: number
  total_quantity?: number
  product_ids?: number[]
  status?: string
  expires_at?: string
}) {
  return apiClient.post('/api/admin/promo-codes', data)
}

// 管理端 - 更新优惠码
export async function updatePromoCode(
  id: number,
  data: {
    code?: string
    name?: string
    description?: string
    discount_type?: 'percentage' | 'fixed'
    // fixed: minor units, percentage: basis points (10000 = 100%)
    discount_value_minor?: number
    // minor units
    max_discount_minor?: number
    // minor units
    min_order_amount_minor?: number
    total_quantity?: number
    product_ids?: number[]
    status?: string
    expires_at?: string
  }
) {
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
  // minor units
  amount_minor?: number
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

export async function createKnowledgeCategory(data: {
  name: string
  parent_id?: number
  sort_order?: number
}) {
  return apiClient.post('/api/admin/knowledge/categories', data)
}

export async function updateKnowledgeCategory(
  id: number,
  data: { name?: string; parent_id?: number; sort_order?: number }
) {
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

export async function createKnowledgeArticle(data: {
  title: string
  content: string
  category_id?: number
  sort_order?: number
}) {
  return apiClient.post('/api/admin/knowledge/articles', data)
}

export async function updateKnowledgeArticle(
  id: number,
  data: { title?: string; content?: string; category_id?: number; sort_order?: number }
) {
  return apiClient.put(`/api/admin/knowledge/articles/${id}`, data)
}

export async function deleteKnowledgeArticle(id: number) {
  return apiClient.delete(`/api/admin/knowledge/articles/${id}`)
}

// ==========================================
// Marketing API
// ==========================================

export type MarketingAudienceMode = 'all' | 'selected' | 'rules'
export type MarketingAudienceCombinator = 'and' | 'or'
export type MarketingAudienceOperator =
  | 'eq'
  | 'neq'
  | 'contains'
  | 'not_contains'
  | 'in'
  | 'not_in'
  | 'gte'
  | 'lte'
  | 'is_empty'
  | 'is_not_empty'
export type MarketingAudienceField =
  | 'id'
  | 'email'
  | 'name'
  | 'phone'
  | 'is_active'
  | 'email_verified'
  | 'email_notify_marketing'
  | 'sms_notify_marketing'
  | 'locale'
  | 'country'
  | 'total_order_count'
  | 'total_spent_minor'
  | 'last_login_at'
  | 'created_at'

export interface MarketingAudienceGroup {
  type: 'group'
  combinator: MarketingAudienceCombinator
  rules: MarketingAudienceNode[]
}

export interface MarketingAudienceCondition {
  type: 'condition'
  field: MarketingAudienceField
  operator: MarketingAudienceOperator
  value?: string | number | boolean | string[]
}

export type MarketingAudienceNode = MarketingAudienceGroup | MarketingAudienceCondition

export interface MarketingAudiencePreviewUser {
  id: number
  name?: string
  email?: string
  phone?: string
  is_active?: boolean
  email_verified?: boolean
  locale?: string
  country?: string
  email_notify_marketing?: boolean
  sms_notify_marketing?: boolean
  total_order_count?: number
  total_spent_minor?: number
  last_login_at?: string
  created_at?: string
}

export interface MarketingAudiencePreviewSummary {
  mode: MarketingAudienceMode
  matched_users: number
  emailable_users: number
  sms_reachable_users: number
  sample_users: MarketingAudiencePreviewUser[]
}

export interface SendAdminMarketingData {
  title: string
  content: string
  send_email: boolean
  send_sms: boolean
  target_all: boolean
  audience_mode?: MarketingAudienceMode
  audience_query?: MarketingAudienceNode
  user_ids?: number[]
}

export interface PreviewAdminMarketingData {
  title: string
  content: string
  audience_mode?: MarketingAudienceMode
  audience_query?: MarketingAudienceNode
  user_id?: number
  user_ids?: number[]
  sample_limit?: number
}

export interface PreviewAdminMarketingResult {
  title: string
  email_subject: string
  content_html: string
  email_html: string
  sms_text: string
  resolved_variables?: Record<string, string>
  supported_placeholders?: string[]
  supported_template_variables?: string[]
  audience?: MarketingAudiencePreviewSummary
}

export interface SendAdminMarketingResult {
  id?: number
  batch_id?: number
  batch_no?: string
  operator_id?: number
  operator_name?: string
  created_at?: string
  started_at?: string
  completed_at?: string
  status?: 'queued' | 'running' | 'completed' | 'failed'
  total_tasks?: number
  processed_tasks?: number
  failed_reason?: string
  target_all: boolean
  audience_mode?: MarketingAudienceMode
  audience_query?: MarketingAudienceNode
  requested_user_count: number
  targeted_users: number
  send_email: boolean
  send_sms: boolean
  email_sent: number
  email_failed: number
  email_skipped: number
  sms_sent: number
  sms_failed: number
  sms_skipped: number
}

export interface MarketingBatchItem {
  id: number
  batch_no: string
  title: string
  status: 'queued' | 'running' | 'completed' | 'failed'
  total_tasks: number
  processed_tasks: number
  send_email: boolean
  send_sms: boolean
  target_all: boolean
  audience_mode?: MarketingAudienceMode
  audience_query?: MarketingAudienceNode
  requested_user_count: number
  targeted_users: number
  email_sent: number
  email_failed: number
  email_skipped: number
  sms_sent: number
  sms_failed: number
  sms_skipped: number
  operator_id?: number
  operator_name?: string
  started_at?: string
  completed_at?: string
  failed_reason?: string
  created_at: string
  updated_at: string
}

export interface MarketingBatchTaskItem {
  id: number
  batch_id: number
  user_id: number
  channel: 'email' | 'sms'
  status: 'pending' | 'queued' | 'sent' | 'failed' | 'skipped'
  error_message?: string
  processed_at?: string
  created_at: string
  user?: {
    id: number
    name?: string
    email?: string
    phone?: string
  }
}

export async function getMarketingUsers(params?: {
  page?: number
  limit?: number
  search?: string
  is_active?: boolean
  email_verified?: boolean
  email_notify_marketing?: boolean
  sms_notify_marketing?: boolean
  has_phone?: boolean
  locale?: string
  country?: string
}) {
  const query = new URLSearchParams()
  if (params?.page) query.append('page', params.page.toString())
  if (params?.limit) query.append('limit', params.limit.toString())
  if (params?.search) query.append('search', params.search)
  if (params?.is_active !== undefined) query.append('is_active', String(params.is_active))
  if (params?.email_verified !== undefined)
    query.append('email_verified', String(params.email_verified))
  if (params?.email_notify_marketing !== undefined)
    query.append('email_notify_marketing', String(params.email_notify_marketing))
  if (params?.sms_notify_marketing !== undefined)
    query.append('sms_notify_marketing', String(params.sms_notify_marketing))
  if (params?.has_phone !== undefined) query.append('has_phone', String(params.has_phone))
  if (params?.locale) query.append('locale', params.locale)
  if (params?.country) query.append('country', params.country)
  return apiClient.get(`/api/admin/marketing/users?${query}`)
}

export async function getMarketingUserCountries() {
  return apiClient.get('/api/admin/marketing/countries')
}

export async function getMarketingBatches(params?: {
  page?: number
  limit?: number
  batch_no?: string
  operator?: string
  status?: string
}) {
  const query = new URLSearchParams()
  if (params?.page) query.append('page', params.page.toString())
  if (params?.limit) query.append('limit', params.limit.toString())
  if (params?.batch_no) query.append('batch_no', params.batch_no)
  if (params?.operator) query.append('operator', params.operator)
  if (params?.status) query.append('status', params.status)
  return apiClient.get(`/api/admin/marketing/batches?${query}`)
}

export async function getMarketingBatch(id: number) {
  return apiClient.get(`/api/admin/marketing/batches/${id}`)
}

export async function getMarketingBatchTasks(
  id: number,
  params?: { page?: number; limit?: number; status?: string; channel?: string; search?: string }
) {
  const query = new URLSearchParams()
  if (params?.page) query.append('page', params.page.toString())
  if (params?.limit) query.append('limit', params.limit.toString())
  if (params?.status) query.append('status', params.status)
  if (params?.channel) query.append('channel', params.channel)
  if (params?.search) query.append('search', params.search)
  return apiClient.get(`/api/admin/marketing/batches/${id}/tasks?${query}`)
}

export async function previewAdminMarketing(data: PreviewAdminMarketingData) {
  return apiClient.post('/api/admin/marketing/preview', data)
}

export async function sendAdminMarketing(data: SendAdminMarketingData) {
  return apiClient.post('/api/admin/marketing/send', data)
}

// ==========================================
// 公告 API
// ==========================================

export interface Announcement {
  id: number
  title: string
  content: string
  category?: 'general' | 'marketing'
  send_email?: boolean
  send_sms?: boolean
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

export async function createAnnouncement(data: {
  title: string
  content: string
  category?: 'general' | 'marketing'
  send_email?: boolean
  send_sms?: boolean
  is_mandatory?: boolean
  require_full_read?: boolean
}) {
  return apiClient.post('/api/admin/announcements', data)
}

export async function updateAnnouncement(
  id: number,
  data: {
    title?: string
    content?: string
    category?: 'general' | 'marketing'
    send_email?: boolean
    send_sms?: boolean
    is_mandatory?: boolean
    require_full_read?: boolean
  }
) {
  return apiClient.put(`/api/admin/announcements/${id}`, data)
}

export async function deleteAnnouncement(id: number) {
  return apiClient.delete(`/api/admin/announcements/${id}`)
}
