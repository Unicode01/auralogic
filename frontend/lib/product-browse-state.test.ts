import {
  buildProductListReturnPath,
  clearProductBrowseState,
  parseFocusedProductIdQuery,
  productListFocusParamKey,
  readProductBrowseState,
  setProductBrowseState,
  stripProductListFocusFromPath,
} from '@/lib/product-browse-state'

describe('product-browse-state', () => {
  beforeEach(() => {
    sessionStorage.clear()
    jest.restoreAllMocks()
  })

  it('stores and reads normalized browse state', () => {
    setProductBrowseState({
      listPath: `/products?category=ebooks&page=2&${productListFocusParamKey}=99`,
      scrollTop: 320,
      focusedProductId: 42.8,
    })

    expect(readProductBrowseState()).toEqual({
      listPath: '/products?category=ebooks&page=2',
      scrollTop: 320,
      focusedProductId: 42,
    })
  })

  it('builds a clean return path without keeping the focused product query', () => {
    expect(buildProductListReturnPath('/products?search=cloud&page=3', 15)).toBe(
      '/products?search=cloud&page=3'
    )
  })

  it('strips focus params from list paths', () => {
    expect(
      stripProductListFocusFromPath(`/products?search=cloud&${productListFocusParamKey}=15`)
    ).toBe('/products?search=cloud')
  })

  it('expires stale state', () => {
    const now = 1_700_000_000_000
    jest.spyOn(Date, 'now').mockReturnValue(now)

    sessionStorage.setItem(
      'auralogic_product_browse_state_v1',
      JSON.stringify({
        listPath: '/products?page=3',
        scrollTop: 200,
        focusedProductId: 15,
        ts: now - 1000 * 60 * 31,
      })
    )

    expect(readProductBrowseState()).toBeNull()
    expect(sessionStorage.getItem('auralogic_product_browse_state_v1')).toBeNull()
  })

  it('parses focused product query safely', () => {
    expect(parseFocusedProductIdQuery('18')).toBe(18)
    expect(parseFocusedProductIdQuery('0')).toBeUndefined()
    expect(parseFocusedProductIdQuery('abc')).toBeUndefined()
  })

  afterEach(() => {
    clearProductBrowseState()
  })
})
