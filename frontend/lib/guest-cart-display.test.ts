import {
  buildGuestCartFallbackItem,
  clearCachedGuestCartDisplayItems,
  getGuestCartDisplayAttributesKey,
  getRestorableGuestCartDisplayItems,
  hydrateGuestCartDisplayItems,
  setCachedGuestCartDisplayItems,
} from '@/lib/guest-cart-display'

describe('guest-cart-display', () => {
  beforeEach(() => {
    localStorage.clear()
    jest.restoreAllMocks()
  })

  it('restores cached display items with current cart quantity and order', () => {
    setCachedGuestCartDisplayItems([
      {
        id: 101,
        guest_key: '1:{"color":"blue"}',
        product_id: 1,
        sku: 'SKU-1',
        name: 'Cached Product',
        price_minor: 1999,
        image_url: 'https://cdn.example.com/a.png',
        product_type: 'physical',
        quantity: 1,
        attributes: { color: 'blue' },
        available_stock: 8,
        is_available: true,
      },
    ])

    const restored = getRestorableGuestCartDisplayItems([
      {
        product_id: 1,
        quantity: 3,
        attributes: { color: 'blue' },
      },
      {
        product_id: 2,
        quantity: 1,
        attributes: { size: 'M' },
      },
    ])

    expect(restored).toHaveLength(2)
    expect(restored[0]).toMatchObject({
      product_id: 1,
      quantity: 3,
      name: 'Cached Product',
      sku: 'SKU-1',
      is_available: true,
    })
    expect(restored[1]).toMatchObject({
      product_id: 2,
      name: '#2',
      sku: 'guest-2',
      is_available: false,
    })
  })

  it('clears expired cache entries before restore', () => {
    const now = 1_700_000_000_000
    jest.spyOn(Date, 'now').mockReturnValue(now)

    localStorage.setItem(
      'auralogic_guest_cart_display_v1',
      JSON.stringify({
        ts: now - 1000 * 60 * 6,
        items: [
          {
            id: 101,
            guest_key: '1:{}',
            product_id: 1,
            sku: 'SKU-1',
            name: 'Expired Product',
            price_minor: 1999,
            image_url: '',
            product_type: 'physical',
            quantity: 1,
            attributes: {},
            available_stock: 8,
            is_available: true,
          },
        ],
      })
    )

    expect(
      getRestorableGuestCartDisplayItems([
        {
          product_id: 1,
          quantity: 1,
        },
      ])
    ).toEqual([])
    expect(localStorage.getItem('auralogic_guest_cart_display_v1')).toBeNull()
  })

  it('hydrates items with deduped product requests and per-attribute stock', async () => {
    const fetchProduct = jest.fn(async (productId: number) => ({
      sku: `SKU-${productId}`,
      name: `Product ${productId}`,
      price_minor: 2500,
      stock: 9,
      status: 'active',
      product_type: 'virtual',
      images: [{ url: `https://cdn.example.com/${productId}.png`, is_primary: true }],
    }))
    const fetchStock = jest.fn(async (_productId: number, attributes?: Record<string, string>) => ({
      is_unlimited: false,
      available_stock: attributes?.edition === 'deluxe' ? 2 : 5,
    }))

    const result = await hydrateGuestCartDisplayItems({
      localItems: [
        {
          product_id: 7,
          quantity: 1,
          attributes: { edition: 'standard' },
        },
        {
          product_id: 7,
          quantity: 2,
          attributes: { edition: 'deluxe' },
        },
      ],
      maxItemQuantity: 99,
      fetchProduct,
      fetchStock,
    })

    expect(fetchProduct).toHaveBeenCalledTimes(1)
    expect(fetchStock).toHaveBeenCalledTimes(2)
    expect(result.hasFailures).toBe(false)
    expect(result.items[0]).toMatchObject({
      product_id: 7,
      name: 'Product 7',
      available_stock: 5,
      is_available: true,
      image_url: 'https://cdn.example.com/7.png',
    })
    expect(result.items[1]).toMatchObject({
      product_id: 7,
      quantity: 2,
      available_stock: 2,
      is_available: true,
    })
  })

  it('marks refresh failures and keeps fallback items when remote data fails', async () => {
    const fallback = buildGuestCartFallbackItem({
      product_id: 3,
      quantity: 1,
      attributes: { license: 'basic' },
    })

    const result = await hydrateGuestCartDisplayItems({
      localItems: [
        {
          product_id: 3,
          quantity: 1,
          attributes: { license: 'basic' },
        },
      ],
      maxItemQuantity: 99,
      fetchProduct: async () => {
        throw new Error('network-error')
      },
      fetchStock: async () => {
        throw new Error('network-error')
      },
    })

    expect(result.hasFailures).toBe(true)
    expect(result.items[0]).toMatchObject(fallback)
  })

  it('builds stable attribute cache keys regardless of property order', () => {
    expect(
      getGuestCartDisplayAttributesKey({
        b: '2',
        a: '1',
      })
    ).toBe(
      getGuestCartDisplayAttributesKey({
        a: '1',
        b: '2',
      })
    )
  })

  afterEach(() => {
    clearCachedGuestCartDisplayItems()
  })
})
