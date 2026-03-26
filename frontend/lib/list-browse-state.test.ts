import {
  buildListReturnPath,
  clearListBrowseState,
  getListFocusParamKey,
  parseFocusedListItemQuery,
  readListBrowseState,
  setListBrowseState,
  stripListFocusFromPath,
} from '@/lib/list-browse-state'

describe('list-browse-state', () => {
  beforeEach(() => {
    sessionStorage.clear()
    jest.restoreAllMocks()
  })

  it('stores and reads normalized order browse state', () => {
    setListBrowseState('orders', {
      listPath: `/orders?page=2&status=pending&${getListFocusParamKey('orders')}=ORD-9`,
      scrollTop: 180,
      focusedItemKey: ' ORD-42 ',
    })

    expect(readListBrowseState('orders')).toEqual({
      listPath: '/orders?page=2&status=pending',
      scrollTop: 180,
      focusedItemKey: 'ORD-42',
    })
  })

  it('stores and reads normalized ticket browse state', () => {
    setListBrowseState('tickets', {
      listPath: `/tickets?status=open&${getListFocusParamKey('tickets')}=17`,
      scrollTop: 64,
      focusedItemKey: '17',
    })

    expect(readListBrowseState('tickets')).toEqual({
      listPath: '/tickets?status=open',
      scrollTop: 64,
      focusedItemKey: '17',
    })
  })

  it('builds clean return paths without keeping focused item query', () => {
    expect(buildListReturnPath('orders', '/orders?search=abc&page=3', 'ORD-77')).toBe(
      '/orders?search=abc&page=3'
    )
    expect(buildListReturnPath('tickets', '/tickets?page=2', '12')).toBe(
      '/tickets?page=2'
    )
  })

  it('strips focus query params from list paths', () => {
    expect(
      stripListFocusFromPath('orders', `/orders?page=2&${getListFocusParamKey('orders')}=ORD-77`)
    ).toBe('/orders?page=2')
    expect(
      stripListFocusFromPath('tickets', `/tickets?status=open&${getListFocusParamKey('tickets')}=12`)
    ).toBe('/tickets?status=open')
  })

  it('expires stale state', () => {
    const now = 1_700_000_000_000
    jest.spyOn(Date, 'now').mockReturnValue(now)

    sessionStorage.setItem(
      'auralogic_list_browse_state_v1:orders',
      JSON.stringify({
        listPath: '/orders?page=3',
        scrollTop: 200,
        focusedItemKey: 'ORD-18',
        ts: now - 1000 * 60 * 31,
      })
    )

    expect(readListBrowseState('orders')).toBeNull()
    expect(sessionStorage.getItem('auralogic_list_browse_state_v1:orders')).toBeNull()
  })

  it('parses focused list item query safely', () => {
    expect(parseFocusedListItemQuery(' ORD-18 ')).toBe('ORD-18')
    expect(parseFocusedListItemQuery('')).toBeUndefined()
    expect(parseFocusedListItemQuery(undefined)).toBeUndefined()
  })

  afterEach(() => {
    clearListBrowseState('orders')
    clearListBrowseState('tickets')
  })
})
