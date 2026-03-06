export interface GuestCartItem {
  product_id: number
  quantity: number
  attributes?: Record<string, string>
}

const GUEST_CART_KEY = 'auralogic_guest_cart'

function isBrowser(): boolean {
  return typeof window !== 'undefined'
}

function normalizeAttributes(attributes?: Record<string, string>): Record<string, string> | undefined {
  if (!attributes || Object.keys(attributes).length === 0) return undefined
  const sortedKeys = Object.keys(attributes).sort()
  const normalized: Record<string, string> = {}
  for (const key of sortedKeys) {
    normalized[key] = attributes[key]
  }
  return normalized
}

export function getGuestCartItemKey(item: GuestCartItem): string {
  const attrs = normalizeAttributes(item.attributes)
  return `${item.product_id}:${JSON.stringify(attrs || {})}`
}

export function getGuestCart(): GuestCartItem[] {
  if (!isBrowser()) return []
  try {
    const raw = localStorage.getItem(GUEST_CART_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw)
    if (!Array.isArray(parsed)) return []
    return parsed
      .filter((item) => item && Number.isFinite(item.product_id) && Number.isFinite(item.quantity))
      .map((item) => ({
        product_id: Number(item.product_id),
        quantity: Math.max(1, Number(item.quantity)),
        attributes: normalizeAttributes(item.attributes),
      }))
  } catch {
    return []
  }
}

export function setGuestCart(items: GuestCartItem[]): void {
  if (!isBrowser()) return
  if (!items.length) {
    localStorage.removeItem(GUEST_CART_KEY)
    return
  }
  localStorage.setItem(GUEST_CART_KEY, JSON.stringify(items))
}

export function clearGuestCart(): void {
  if (!isBrowser()) return
  localStorage.removeItem(GUEST_CART_KEY)
}

export function addToGuestCart(
  item: GuestCartItem,
  maxItemQuantity = 9999
): { mergedQuantity: number; itemCount: number } {
  const items = getGuestCart()
  const normalized: GuestCartItem = {
    product_id: item.product_id,
    quantity: Math.max(1, item.quantity),
    attributes: normalizeAttributes(item.attributes),
  }

  const key = getGuestCartItemKey(normalized)
  const index = items.findIndex((existing) => getGuestCartItemKey(existing) === key)

  if (index >= 0) {
    const merged = Math.min(maxItemQuantity, items[index].quantity + normalized.quantity)
    items[index] = { ...items[index], quantity: merged }
  } else {
    items.push({ ...normalized, quantity: Math.min(maxItemQuantity, normalized.quantity) })
  }

  setGuestCart(items)

  const finalIndex = items.findIndex((existing) => getGuestCartItemKey(existing) === key)
  const mergedQuantity = finalIndex >= 0 ? items[finalIndex].quantity : normalized.quantity

  return {
    mergedQuantity,
    itemCount: items.length,
  }
}

export function updateGuestCartItemQuantityByKey(
  key: string,
  quantity: number,
  maxItemQuantity = 9999
): { updated: boolean; itemCount: number } {
  const items = getGuestCart()
  const index = items.findIndex((existing) => getGuestCartItemKey(existing) === key)
  if (index < 0) {
    return { updated: false, itemCount: items.length }
  }

  const nextQuantity = Math.max(1, Math.min(maxItemQuantity, quantity))
  items[index] = {
    ...items[index],
    quantity: nextQuantity,
    attributes: normalizeAttributes(items[index].attributes),
  }
  setGuestCart(items)
  return { updated: true, itemCount: items.length }
}

export function removeGuestCartItemByKey(key: string): { removed: boolean; itemCount: number } {
  const items = getGuestCart()
  const next = items.filter((existing) => getGuestCartItemKey(existing) !== key)
  if (next.length === items.length) {
    return { removed: false, itemCount: items.length }
  }
  setGuestCart(next)
  return { removed: true, itemCount: next.length }
}
