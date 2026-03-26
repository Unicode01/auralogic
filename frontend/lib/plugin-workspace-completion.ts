type PluginWorkspaceCompletionNode = {
  children: Record<string, PluginWorkspaceCompletionNode>
}

export type PluginWorkspaceCompletionKind = 'completed' | 'extended' | 'suggestions' | 'none'

export type PluginWorkspaceCompletionResult = {
  kind: PluginWorkspaceCompletionKind
  nextValue: string
  nextSelectionStart: number
  nextSelectionEnd: number
  token: string
  resolvedToken: string
  suggestions: string[]
}

const ROOT_COMPLETION_PATHS = [
  ':inspect',
  'help',
  'keys',
  'runtimeState',
  'commands',
  'permissions',
  'workspaceState',
  'inspect',
  'clearOutput',
  'URLSearchParams',
  'TextEncoder',
  'TextDecoder',
  'atob',
  'btoa',
  'structuredClone',
  'queueMicrotask',
  'setTimeout',
  'clearTimeout',
  'module',
  'exports',
  'Plugin',
  'sandbox',
  'console',
  'globalThis',
  'Worker',
  '$_',
  '$1',
  '$2',
  '$3',
  '$4',
  '$5',
] as const

const CONSOLE_MEMBERS = ['log', 'info', 'warn', 'error', 'debug'] as const
const WORKSPACE_MEMBERS = [
  'enabled',
  'commandName',
  'commandId',
  'write',
  'writeln',
  'info',
  'warn',
  'error',
  'clear',
  'tail',
  'snapshot',
  'read',
  'readLine',
] as const
const STORAGE_MEMBERS = ['get', 'set', 'delete', 'list', 'clear'] as const
const SECRET_MEMBERS = ['get', 'has', 'list'] as const
const WEBHOOK_MEMBERS = [
  'enabled',
  'key',
  'method',
  'path',
  'queryString',
  'contentType',
  'remoteAddr',
  'headers',
  'queryParams',
  'bodyText',
  'bodyBase64',
  'header',
  'query',
  'text',
  'json',
] as const
const HTTP_MEMBERS = [
  'enabled',
  'defaultTimeoutMs',
  'maxResponseBytes',
  'get',
  'post',
  'request',
] as const
const FS_MEMBERS = [
  'enabled',
  'root',
  'codeRoot',
  'dataRoot',
  'pluginID',
  'pluginName',
  'maxFiles',
  'maxTotalBytes',
  'maxReadBytes',
  'exists',
  'readText',
  'readBase64',
  'readJSON',
  'writeText',
  'writeJSON',
  'writeBase64',
  'delete',
  'mkdir',
  'list',
  'stat',
  'usage',
  'recalculateUsage',
] as const
const SANDBOX_MEMBERS = [
  'allowNetwork',
  'allowFileSystem',
  'currentAction',
  'declaredStorageAccessMode',
  'storageAccessMode',
  'allowHookExecute',
  'allowHookBlock',
  'allowPayloadPatch',
  'allowFrontendExtensions',
  'allowExecuteAPI',
  'requestedPermissions',
  'grantedPermissions',
  'executeActionStorage',
  'defaultTimeoutMs',
  'maxConcurrency',
  'maxMemoryMB',
  'fsMaxFiles',
  'fsMaxTotalBytes',
  'fsMaxReadBytes',
  'storageMaxKeys',
  'storageMaxTotalBytes',
  'storageMaxValueBytes',
] as const
const HOST_RESOURCES: Record<string, readonly string[]> = {
  order: ['get', 'list', 'assignTracking', 'requestResubmit', 'markPaid', 'updatePrice'],
  user: ['get', 'list'],
  product: ['get', 'list'],
  inventory: ['get', 'list'],
  inventoryBinding: ['get', 'list'],
  promo: ['get', 'list'],
  ticket: ['get', 'list', 'reply', 'update'],
  serial: ['get', 'list'],
  announcement: ['get', 'list'],
  knowledge: ['get', 'list', 'categories'],
  paymentMethod: ['get', 'list'],
  virtualInventory: ['get', 'list'],
  virtualInventoryBinding: ['get', 'list'],
} as const

function createCompletionNode(): PluginWorkspaceCompletionNode {
  return { children: {} }
}

function insertCompletionPath(
  root: PluginWorkspaceCompletionNode,
  path: string
): PluginWorkspaceCompletionNode {
  const normalized = String(path || '').trim()
  if (!normalized) {
    return root
  }
  if (!normalized.includes('.') || normalized.startsWith(':')) {
    if (!root.children[normalized]) {
      root.children[normalized] = createCompletionNode()
    }
    return root.children[normalized]
  }
  const segments = normalized.split('.').filter(Boolean)
  let current = root
  for (const segment of segments) {
    if (!current.children[segment]) {
      current.children[segment] = createCompletionNode()
    }
    current = current.children[segment]
  }
  return current
}

function buildCompletionTree(): PluginWorkspaceCompletionNode {
  const root = createCompletionNode()
  ROOT_COMPLETION_PATHS.forEach((path) => {
    insertCompletionPath(root, path)
  })

  CONSOLE_MEMBERS.forEach((member) => {
    insertCompletionPath(root, `console.${member}`)
  })
  WORKSPACE_MEMBERS.forEach((member) => {
    insertCompletionPath(root, `Plugin.workspace.${member}`)
  })
  STORAGE_MEMBERS.forEach((member) => {
    insertCompletionPath(root, `Plugin.storage.${member}`)
  })
  SECRET_MEMBERS.forEach((member) => {
    insertCompletionPath(root, `Plugin.secret.${member}`)
  })
  WEBHOOK_MEMBERS.forEach((member) => {
    insertCompletionPath(root, `Plugin.webhook.${member}`)
  })
  HTTP_MEMBERS.forEach((member) => {
    insertCompletionPath(root, `Plugin.http.${member}`)
  })
  FS_MEMBERS.forEach((member) => {
    insertCompletionPath(root, `Plugin.fs.${member}`)
  })
  SANDBOX_MEMBERS.forEach((member) => {
    insertCompletionPath(root, `sandbox.${member}`)
  })
  Object.keys(HOST_RESOURCES).forEach((resource) => {
    insertCompletionPath(root, `Plugin.host.${resource}`)
    insertCompletionPath(root, `Plugin.${resource}`)
    HOST_RESOURCES[resource].forEach((method) => {
      insertCompletionPath(root, `Plugin.host.${resource}.${method}`)
      insertCompletionPath(root, `Plugin.${resource}.${method}`)
    })
  })
  ;[
    'globalThis',
    'Object.keys',
    'Object.entries',
    'Object.values',
    'JSON.parse',
    'JSON.stringify',
    'Array.isArray',
    'Promise.resolve',
    'Promise.reject',
    'Math.max',
    'Math.min',
    'Date.now',
  ].forEach((path) => {
    insertCompletionPath(root, path)
  })

  return root
}

const PLUGIN_WORKSPACE_COMPLETION_TREE = buildCompletionTree()

function resolveCompletionNode(
  root: PluginWorkspaceCompletionNode,
  pathSegments: string[]
): PluginWorkspaceCompletionNode | null {
  let current: PluginWorkspaceCompletionNode | undefined = root
  for (const segment of pathSegments) {
    current = current?.children[segment]
    if (!current) {
      return null
    }
  }
  return current
}

function extractCompletionToken(
  value: string,
  cursor: number
): { token: string; startIndex: number } | null {
  const beforeCursor = value.slice(0, cursor)
  const match = beforeCursor.match(/([:$A-Za-z_][$\w.:]*)$/)
  if (!match) {
    return null
  }
  const token = String(match[1] || '')
  if (!token) {
    return null
  }
  return {
    token,
    startIndex: cursor - token.length,
  }
}

function commonPrefix(values: string[]): string {
  if (values.length === 0) {
    return ''
  }
  let prefix = values[0]
  for (let index = 1; index < values.length; index += 1) {
    while (prefix && !values[index].startsWith(prefix)) {
      prefix = prefix.slice(0, -1)
    }
    if (!prefix) {
      return ''
    }
  }
  return prefix
}

function normalizeCompletionPaths(paths?: string[]): string[] {
  if (!Array.isArray(paths) || paths.length === 0) {
    return []
  }
  const seen = new Set<string>()
  const out: string[] = []
  paths.forEach((item) => {
    const normalized = String(item || '').trim()
    if (!normalized || seen.has(normalized)) {
      return
    }
    seen.add(normalized)
    out.push(normalized)
  })
  out.sort((left, right) => left.localeCompare(right))
  return out
}

function buildMergedCompletionTree(dynamicPaths?: string[]): PluginWorkspaceCompletionNode {
  const normalizedDynamicPaths = normalizeCompletionPaths(dynamicPaths)
  if (normalizedDynamicPaths.length === 0) {
    return PLUGIN_WORKSPACE_COMPLETION_TREE
  }
  const root = buildCompletionTree()
  normalizedDynamicPaths.forEach((path) => {
    insertCompletionPath(root, path)
  })
  return root
}

function lookupCompletionSuggestions(
  root: PluginWorkspaceCompletionNode,
  token: string
): string[] {
  const normalized = String(token || '')
  if (!normalized) {
    return []
  }
  if (normalized.startsWith(':')) {
    return Object.keys(root.children)
      .filter((candidate) => candidate.startsWith(normalized))
      .sort((left, right) => left.localeCompare(right))
  }

  const endsWithDot = normalized.endsWith('.')
  const segments = normalized.split('.')
  const baseSegments = endsWithDot
    ? segments.filter(Boolean)
    : segments.slice(0, -1).filter(Boolean)
  const lastSegment = endsWithDot ? '' : String(segments[segments.length - 1] || '')
  const parentNode = resolveCompletionNode(root, baseSegments)
  if (!parentNode) {
    return []
  }
  return Object.keys(parentNode.children)
    .filter((candidate) => candidate.startsWith(lastSegment))
    .sort((left, right) => left.localeCompare(right))
    .map((candidate) =>
      baseSegments.length > 0 ? `${baseSegments.join('.')}.${candidate}` : candidate
    )
}

export function resolvePluginWorkspaceCompletion(payload: {
  value: string
  selectionStart: number
  selectionEnd?: number
  dynamicPaths?: string[]
}): PluginWorkspaceCompletionResult {
  const value = String(payload.value || '')
  const selectionStart = Math.max(
    0,
    Math.min(value.length, Math.trunc(payload.selectionStart || 0))
  )
  const selectionEnd = Math.max(
    selectionStart,
    Math.min(value.length, Math.trunc(payload.selectionEnd ?? selectionStart))
  )
  const completionTree = buildMergedCompletionTree(payload.dynamicPaths)
  const extracted = extractCompletionToken(value, selectionStart)
  if (!extracted) {
    return {
      kind: 'none',
      nextValue: value,
      nextSelectionStart: selectionStart,
      nextSelectionEnd: selectionEnd,
      token: '',
      resolvedToken: '',
      suggestions: [],
    }
  }

  const suggestions = lookupCompletionSuggestions(completionTree, extracted.token)
  if (suggestions.length === 0) {
    return {
      kind: 'none',
      nextValue: value,
      nextSelectionStart: selectionStart,
      nextSelectionEnd: selectionEnd,
      token: extracted.token,
      resolvedToken: extracted.token,
      suggestions: [],
    }
  }

  const applyToken = (nextToken: string, kind: PluginWorkspaceCompletionKind) => ({
    kind,
    nextValue: `${value.slice(0, extracted.startIndex)}${nextToken}${value.slice(selectionEnd)}`,
    nextSelectionStart: extracted.startIndex + nextToken.length,
    nextSelectionEnd: extracted.startIndex + nextToken.length,
    token: extracted.token,
    resolvedToken: nextToken,
    suggestions,
  })

  if (suggestions.length === 1) {
    return applyToken(suggestions[0], 'completed')
  }

  const sharedPrefix = commonPrefix(suggestions)
  if (sharedPrefix.length > extracted.token.length) {
    return applyToken(sharedPrefix, 'extended')
  }

  return {
    kind: 'suggestions',
    nextValue: value,
    nextSelectionStart: selectionStart,
    nextSelectionEnd: selectionEnd,
    token: extracted.token,
    resolvedToken: extracted.token,
    suggestions,
  }
}
