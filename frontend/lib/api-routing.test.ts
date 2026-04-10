type MockAxiosClient = {
  get: jest.Mock
  post: jest.Mock
  put: jest.Mock
  delete: jest.Mock
  request: jest.Mock
  interceptors: {
    request: {
      use: jest.Mock
    }
    response: {
      use: jest.Mock
    }
  }
}

function createMockAxiosClient(): MockAxiosClient {
  return {
    get: jest.fn(),
    post: jest.fn(),
    put: jest.fn(),
    delete: jest.fn(),
    request: jest.fn(),
    interceptors: {
      request: {
        use: jest.fn(),
      },
      response: {
        use: jest.fn(),
      },
    },
  }
}

function createFetchJSONResponse(payload: unknown) {
  return {
    ok: true,
    status: 200,
    statusText: 'OK',
    headers: {
      get(name: string) {
        return name.toLowerCase() === 'content-type' ? 'application/json' : null
      },
    },
    json: jest.fn(async () => payload),
    text: jest.fn(async () => JSON.stringify(payload)),
  }
}

function loadAPIModule() {
  const proxyClient = createMockAxiosClient()
  const publicClient = createMockAxiosClient()
  const axiosCreateMock = jest
    .fn()
    .mockImplementationOnce(() => proxyClient)
    .mockImplementationOnce(() => publicClient)
  const clearTokenMock = jest.fn()
  const getTokenMock = jest.fn(() => 'browser-token')

  jest.resetModules()
  jest.doMock('axios', () => ({
    __esModule: true,
    default: {
      create: axiosCreateMock,
    },
    create: axiosCreateMock,
  }))
  jest.doMock('./auth', () => ({
    clearToken: clearTokenMock,
    getToken: getTokenMock,
  }))
  jest.doMock('./api-base-url', () => ({
    getClientAPIProxyBaseURL: () => '/api/_backend',
    getConfiguredPublicAPIBaseURL: () => '',
    resolveClientAPIProxyURL: (path: string) => `/api/_backend${path}`,
    resolvePublicAPIURL: (path: string) => path,
  }))
  jest.doMock('./plugin-frontend-routing', () => ({
    stringifyPluginHostContext: (value?: Record<string, any>) =>
      value && Object.keys(value).length > 0 ? JSON.stringify(value) : '{}',
  }))

  let loadedModule: typeof import('./api')
  jest.isolateModules(() => {
    loadedModule = require('./api')
  })

  return {
    module: loadedModule!,
    proxyClient,
    publicClient,
    axiosCreateMock,
    clearTokenMock,
    getTokenMock,
  }
}

describe('api routing', () => {
  const originalFetch = global.fetch

  afterEach(() => {
    global.fetch = originalFetch
    jest.resetModules()
    jest.clearAllMocks()
  })

  test('routes public helpers through the direct public client and keeps protected calls proxied', async () => {
    const { module, proxyClient, publicClient } = loadAPIModule()

    await module.getCaptcha()
    await module.login({ email: 'demo@example.com', password: 'Secret123!' })
    await module.getPublicConfig()
    await module.getCountries()
    await module.getFeaturedProducts(6)
    await module.getPluginExtensions('/', 'default')
    await module.logout()

    expect(publicClient.get).toHaveBeenCalledWith('/api/user/auth/captcha')
    expect(publicClient.post).toHaveBeenCalledWith('/api/user/auth/login', {
      email: 'demo@example.com',
      password: 'Secret123!',
    })
    expect(publicClient.get).toHaveBeenCalledWith('/api/config/public')
    expect(publicClient.get).toHaveBeenCalledWith('/api/form/countries')
    expect(publicClient.get).toHaveBeenCalledWith('/api/user/products/featured?limit=6')
    expect(publicClient.get).toHaveBeenCalledWith('/api/config/plugin-extensions?path=%2F&slot=default', {
      headers: undefined,
      signal: undefined,
    })

    expect(proxyClient.post).toHaveBeenCalledWith('/api/user/auth/logout')
    expect(proxyClient.get).not.toHaveBeenCalledWith('/api/user/auth/captcha')
    expect(proxyClient.post).not.toHaveBeenCalledWith('/api/user/auth/login', expect.anything())
  })

  test('routes public plugin execute actions directly and protected ones through the proxy client', async () => {
    const { module, proxyClient, publicClient } = loadAPIModule()

    await module.executePluginRouteAction(
      {
        scope: 'public',
        requires_auth: false,
        url: '/api/config/plugins/demo/execute',
      },
      {
        action: 'open',
      }
    )

    await module.executePluginRouteAction(
      {
        scope: 'admin',
        requires_auth: true,
        url: '/api/admin/plugins/1/execute',
      },
      {
        action: 'refresh',
      }
    )

    expect(module.shouldUseDirectPublicPluginRouteAPI({ scope: 'public' })).toBe(true)
    expect(module.shouldUseDirectPublicPluginRouteAPI({ scope: 'public', requires_auth: true })).toBe(
      false
    )
    expect(publicClient.request).toHaveBeenCalledWith({
      data: { action: 'open' },
      headers: undefined,
      method: 'post',
      url: '/api/config/plugins/demo/execute',
    })
    expect(proxyClient.request).toHaveBeenCalledWith({
      data: { action: 'refresh' },
      headers: undefined,
      method: 'post',
      url: '/api/admin/plugins/1/execute',
    })
  })

  test('routes plugin execute streams to direct /api only for public unauthenticated routes', async () => {
    const fetchMock = jest
      .fn()
      .mockResolvedValueOnce(createFetchJSONResponse({ ok: true }))
      .mockResolvedValueOnce(createFetchJSONResponse({ ok: true }))
    global.fetch = fetchMock as unknown as typeof fetch

    const { module } = loadAPIModule()

    await module.executePluginRouteActionStream(
      {
        scope: 'public',
        requires_auth: false,
        stream_url: '/api/config/plugins/demo/execute/stream',
      },
      {
        action: 'open',
      }
    )

    await module.executePluginRouteActionStream(
      {
        scope: 'admin',
        requires_auth: true,
        stream_url: '/api/admin/plugins/1/execute/stream',
      },
      {
        action: 'refresh',
      }
    )

    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      '/api/config/plugins/demo/execute/stream',
      expect.objectContaining({
        credentials: 'same-origin',
        method: 'POST',
      })
    )
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      '/api/_backend/api/admin/plugins/1/execute/stream',
      expect.objectContaining({
        credentials: 'same-origin',
        method: 'POST',
      })
    )

    const publicHeaders = fetchMock.mock.calls[0][1]?.headers as Headers
    expect(publicHeaders.get('Authorization')).toBe('Bearer browser-token')
  })
})
