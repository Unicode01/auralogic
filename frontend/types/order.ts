import type { ProductType } from './product'

export interface OrderItem {
  sku: string
  name: string
  quantity: number
  imageUrl?: string
  image_url?: string
  attributes?: Record<string, any>
  product_type?: ProductType
  productType?: ProductType
}

export interface Order {
  id: number
  orderNo: string
  order_no?: string
  userId?: number
  user_id?: number
  externalUserId?: string
  external_user_id?: string
  externalUserName?: string
  external_user_name?: string
  platform?: string
  status: OrderStatus
  items: OrderItem[]
  totalAmount?: number
  total_amount?: number
  currency?: string
  receiverName?: string
  receiver_name?: string
  receiverPhone?: string
  receiver_phone?: string
  receiverEmail?: string
  receiver_email?: string
  receiverProvince?: string
  receiver_province?: string
  receiverCity?: string
  receiver_city?: string
  receiverDistrict?: string
  receiver_district?: string
  receiverAddress?: string
  receiver_address?: string
  receiverPostcode?: string
  receiver_postcode?: string
  privacyProtected: boolean
  privacy_protected?: boolean
  trackingNo?: string
  tracking_no?: string
  shippedAt?: string
  shipped_at?: string
  formToken?: string
  form_token?: string
  formSubmittedAt?: string
  form_submitted_at?: string
  formExpiresAt?: string
  form_expires_at?: string
  userEmail?: string
  user_email?: string
  remark?: string
  adminRemark?: string
  admin_remark?: string
  sharedToSupport?: boolean
  shared_to_support?: boolean
  createdAt: string
  created_at?: string
  updatedAt: string
  updated_at?: string
}

export type OrderStatus =
  | 'pending_payment'
  | 'draft'
  | 'pending'
  | 'need_resubmit'
  | 'shipped'
  | 'completed'
  | 'cancelled'
  | 'refunded'

export interface OrderListResponse {
  items: Order[]
  pagination: {
    page: number
    limit: number
    total: number
    total_pages: number
    has_next?: boolean
    has_prev?: boolean
  }
}

export interface OrderQueryParams {
  page?: number
  limit?: number
  status?: string
  search?: string
}

