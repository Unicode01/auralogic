import { type CartItem } from '@/lib/api'
import { type GuestCartItem, getGuestCartItemKey } from '@/lib/guest-cart'

const GUEST_CART_DISPLAY_CACHE_KEY = 'auralogic_guest_cart_display_v1'
const GUEST_CART_DISPLAY_CACHE_TTL_MS = 1000 * 60 * 5
const DEFAULT_HYDRATE_CONCURRENCY = 4

type GuestCartProductData = {
  sku?: string
  name?: string
  price_minor?: number
  images?: Array<{ url?: string; is_primary?: boolean; isPrimary?: boolean }>
  stock?: number
  status?: string
  product_type?: string
  productType?: string
  max_purchase_limit?: number
  maxPurchaseLimit?: number
}

type GuestCartStockData = {
  is_unlimited?: boolean
  available_stock?: number
}

type GuestCartDisplayCachePayload = {
  ts: number
  items: Array<Omit<GuestCartDisplayItem, 'product'>>
}

export type GuestCartDisplayItem = CartItem & {
  guest_key: string
}

export interface HydrateGuestCartDisplayItemsOptions {
  localItems: GuestCartItem[]
  maxItemQuantity: number
  fetchProduct: (productId: number) => Promise<GuestCartProductData | null | undefined>
  fetchStock: (
    productId: number,
    attributes?: Record<string, string>
  ) => Promise<GuestCartStockData | null | undefined>
  concurrency?: number
}

export interface HydrateGuestCartDisplayItemsResult {
  items: GuestCartDisplayItem[]
  hasFailures: boolean
}

function isBrowser(): boolean {
  return typeof window !== 'undefined'
}

function normalizeAttributes(attributes?: Record<string, string>): Record<string, string> {
  if (!attributes || typeof attributes !== 'object') return {}

  const normalized: Record<string, string> = {}
  for (const key of Object.keys(attributes).sort()) {
    const value = attributes[key]
    if (value === undefined || value === null) continue
    normalized[key] = String(value)
  }
  return normalized
}

function getPositiveQuantity(quantity: number): number {
  return Math.max(1, Number(quantity) || 1)
}

function getSnapshotItemMap(): Map<string, Omit<GuestCartDisplayItem, 'product'>> {
  if (!isBrowser()) return new Map()

  try {
    const raw = localStorage.getItem(GUEST_CART_DISPLAY_CACHE_KEY)
    if (!raw) return new Map()

    const parsed = JSON.parse(raw) as GuestCartDisplayCachePayload | null
    if (!parsed || !Number.isFinite(parsed.ts) || !Array.isArray(parsed.items)) {
      clearCachedGuestCartDisplayItems()
      return new Map()
    }

    if (Date.now() - Number(parsed.ts) > GUEST_CART_DISPLAY_CACHE_TTL_MS) {
      clearCachedGuestCartDisplayItems()
      return new Map()
    }

    const map = new Map<string, Omit<GuestCartDisplayItem, 'product'>>()
    for (const item of parsed.items) {
      if (!item || typeof item.guest_key !== 'string' || !Number.isFinite(item.product_id)) continue
      map.set(item.guest_key, item)
    }
    return map
  } catch {
    clearCachedGuestCartDisplayItems()
    return new Map()
  }
}

async function mapWithConcurrency<T, TResult>(
  items: T[],
  concurrency: number,
  mapper: (item: T, index: number) => Promise<TResult>
): Promise<TResult[]> {
  if (!items.length) return []

  const workerCount = Math.max(1, Math.min(concurrency, items.length))
  const results = new Array<TResult>(items.length)
  let cursor = 0

  const worker = async () => {
    while (true) {
      const currentIndex = cursor
      cursor += 1
      if (currentIndex >= items.length) return
      results[currentIndex] = await mapper(items[currentIndex], currentIndex)
    }
  }

  await Promise.all(Array.from({ length: workerCount }, () => worker()))
  return results
}

function resolvePrimaryImage(product?: GuestCartProductData | null): string {
  if (!product?.images?.length) return ''
  return (
    product.images.find((image) => image?.is_primary || image?.isPrimary)?.url
    || product.images[0]?.url
    || ''
  )
}

export function getGuestCartDisplayAttributesKey(attributes?: Record<string, string>): string {
  return JSON.stringify(normalizeAttributes(attributes))
}

export function getGuestDisplayItemId(key: string): number {
  let hash = 0
  for (let i = 0; i < key.length; i++) {
    hash = ((hash << 5) - hash + key.charCodeAt(i)) | 0
  }
  const normalized = Math.abs(hash)
  return normalized === 0 ? 1 : normalized
}

export function buildGuestCartFallbackItem(localItem: GuestCartItem): GuestCartDisplayItem {
  const guestKey = getGuestCartItemKey(localItem)
  return {
    id: getGuestDisplayItemId(guestKey),
    guest_key: guestKey,
    product_id: localItem.product_id,
    sku: `guest-${localItem.product_id}`,
    name: `#${localItem.product_id}`,
    price_minor: 0,
    image_url: '',
    product_type: 'physical',
    quantity: getPositiveQuantity(localItem.quantity),
    attributes: normalizeAttributes(localItem.attributes),
    available_stock: 0,
    is_available: false,
  }
}

export function getRestorableGuestCartDisplayItems(
  localItems: GuestCartItem[]
): GuestCartDisplayItem[] {
  if (!localItems.length) return []

  const snapshotItems = getSnapshotItemMap()
  if (snapshotItems.size === 0) return []

  return localItems.map((localItem) => {
    const fallback = buildGuestCartFallbackItem(localItem)
    const snapshotItem = snapshotItems.get(fallback.guest_key)
    if (!snapshotItem) return fallback

    return {
      ...fallback,
      ...snapshotItem,
      id: fallback.id,
      guest_key: fallback.guest_key,
      product_id: fallback.product_id,
      quantity: fallback.quantity,
      attributes: fallback.attributes,
    }
  })
}

export function setCachedGuestCartDisplayItems(items: GuestCartDisplayItem[]): void {
  if (!isBrowser()) return

  if (!items.length) {
    clearCachedGuestCartDisplayItems()
    return
  }

  const snapshotItems = items.map(({ product, ...item }) => item)
  const payload: GuestCartDisplayCachePayload = {
    ts: Date.now(),
    items: snapshotItems,
  }
  localStorage.setItem(GUEST_CART_DISPLAY_CACHE_KEY, JSON.stringify(payload))
}

export function clearCachedGuestCartDisplayItems(): void {
  if (!isBrowser()) return
  localStorage.removeItem(GUEST_CART_DISPLAY_CACHE_KEY)
}

export async function hydrateGuestCartDisplayItems(
  options: HydrateGuestCartDisplayItemsOptions
): Promise<HydrateGuestCartDisplayItemsResult> {
  const {
    localItems,
    maxItemQuantity,
    fetchProduct,
    fetchStock,
    concurrency = DEFAULT_HYDRATE_CONCURRENCY,
  } = options

  const productPromiseCache = new Map<number, Promise<GuestCartProductData | null | undefined>>()
  const stockPromiseCache = new Map<string, Promise<GuestCartStockData | null | undefined>>()
  let hasFailures = false

  const items = await mapWithConcurrency(localItems, concurrency, async (localItem) => {
    const fallback = buildGuestCartFallbackItem(localItem)
    const productId = localItem.product_id
    const attributes = normalizeAttributes(localItem.attributes)
    const stockCacheKey = `${productId}:${getGuestCartDisplayAttributesKey(attributes)}`

    const productPromise = productPromiseCache.get(productId)
      || fetchProduct(productId)
    if (!productPromiseCache.has(productId)) {
      productPromiseCache.set(productId, productPromise)
    }

    const stockPromise = stockPromiseCache.get(stockCacheKey)
      || fetchStock(productId, attributes)
    if (!stockPromiseCache.has(stockCacheKey)) {
      stockPromiseCache.set(stockCacheKey, stockPromise)
    }

    const [productResult, stockResult] = await Promise.allSettled([productPromise, stockPromise])
    if (productResult.status === 'rejected' || stockResult.status === 'rejected') {
      hasFailures = true
    }

    const product = productResult.status === 'fulfilled' ? productResult.value : null
    const stockData = stockResult.status === 'fulfilled' ? stockResult.value : null
    const primaryImage = resolvePrimaryImage(product)
    const isUnlimitedStock = stockData?.is_unlimited === true
    const availableStock = isUnlimitedStock
      ? maxItemQuantity
      : Math.max(0, Number(stockData?.available_stock ?? product?.stock ?? 0))
    const isAvailable = Boolean(product)
      && (isUnlimitedStock || availableStock > 0)
      && product?.status !== 'inactive'
      && product?.status !== 'draft'

    return {
      ...fallback,
      sku: product?.sku || fallback.sku,
      name: product?.name || fallback.name,
      price_minor: Number(product?.price_minor ?? fallback.price_minor),
      image_url: primaryImage || fallback.image_url,
      product_type: product?.product_type || product?.productType || fallback.product_type,
      available_stock: availableStock,
      is_available: isAvailable,
      ...(product ? { product } : {}),
    }
  })

  return {
    items,
    hasFailures,
  }
}
