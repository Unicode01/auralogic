export interface ProductImage {
  url: string
  alt?: string
  is_primary: boolean
  isPrimary?: boolean
}

export interface ProductAttribute {
  name: string
  values: string[]
}

export type ProductStatus = 'draft' | 'active' | 'inactive' | 'out_of_stock'

export type ProductType = 'physical' | 'virtual'

export interface Product {
  id: number
  sku: string
  name: string
  description?: string
  shortDescription?: string
  short_description?: string
  category?: string
  tags?: string[]
  price: number
  originalPrice?: number
  original_price?: number
  stock: number
  images?: ProductImage[]
  attributes?: ProductAttribute[]
  status: ProductStatus
  product_type?: ProductType
  productType?: ProductType
  sortOrder?: number
  sort_order?: number
  isFeatured?: boolean
  is_featured?: boolean
  isRecommended?: boolean
  is_recommended?: boolean
  auto_delivery?: boolean
  autoDelivery?: boolean
  viewCount?: number
  view_count?: number
  saleCount?: number
  sale_count?: number
  remark?: string
  createdAt: string
  created_at?: string
  updatedAt: string
  updated_at?: string
}

export interface ProductListResponse {
  items: Product[]
  pagination: {
    page: number
    limit: number
    total: number
    total_pages: number
    has_next?: boolean
    has_prev?: boolean
  }
}

export interface ProductQueryParams {
  page?: number
  limit?: number
  status?: string
  category?: string
  search?: string
  is_featured?: boolean
}

export interface CreateProductRequest {
  sku: string
  name: string
  description?: string
  short_description?: string
  category?: string
  tags?: string[]
  price: number
  original_price?: number
  stock: number
  images?: ProductImage[]
  attributes?: ProductAttribute[]
  status?: ProductStatus
  sort_order?: number
  is_featured?: boolean
  is_recommended?: boolean
  remark?: string
}

export interface UpdateProductRequest extends Partial<CreateProductRequest> { }

// Virtual Product Stock Types
export type VirtualStockStatus = 'available' | 'sold' | 'reserved' | 'invalid'

export interface VirtualProductStock {
  id: number
  virtual_inventory_id: number
  content: string
  remark?: string
  status: VirtualStockStatus
  order_id?: number
  order_no?: string
  delivered_at?: string
  delivered_by?: number
  batch_no?: string
  imported_by?: string
  created_at: string
  updated_at: string
}

export interface VirtualStockStats {
  total: number
  available: number
  reserved: number
  sold: number
}

