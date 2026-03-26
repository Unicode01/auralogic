'use client'

import {
  getAdminPluginExtensions,
  getAdminPluginExtensionsBatch,
  getPluginExtensions,
  getPluginExtensionsBatch,
  type PluginFrontendExtension,
  type PluginFrontendExtensionBatchRequestItem,
  type PluginFrontendExtensionBatchResponseItem,
} from '@/lib/api'
import {
  normalizePluginHostContext,
  normalizePluginPath,
  normalizePluginStringMap,
  stringifyPluginHostContext,
  stringifyPluginStringMap,
} from '@/lib/plugin-frontend-routing'

export type PluginSlotRequestScope = 'public' | 'admin'

type PluginSlotRequestInput = {
  path: string
  slot: string
  locale?: string
  queryParams?: Record<string, string>
  hostContext?: Record<string, any>
  signal?: AbortSignal
}

type PluginSlotRequestResult = {
  data: {
    path: string
    slot: string
    extensions: PluginFrontendExtension[]
  }
}

type NormalizedPluginSlotRequest = {
  key: string
  path: string
  slot: string
  locale: string
  queryParams?: Record<string, string>
  hostContext?: Record<string, any>
}

type PendingPluginSlotRequest = {
  request: NormalizedPluginSlotRequest
  signal?: AbortSignal
  aborted: boolean
  settled: boolean
  cleanupAbort?: () => void
  resolve: (value: PluginSlotRequestResult) => void
  reject: (reason?: unknown) => void
}

type PluginSlotRequestQueueState = {
  scheduled: boolean
  pending: PendingPluginSlotRequest[]
}

type PendingPluginSlotRequestGroup = {
  key: string
  request: NormalizedPluginSlotRequest
  requests: PendingPluginSlotRequest[]
}

type PendingPluginSlotRequestLocaleBatch = {
  locale: string
  groups: PendingPluginSlotRequestGroup[]
}

const pluginSlotRequestQueues: Record<PluginSlotRequestScope, PluginSlotRequestQueueState> = {
  public: {
    scheduled: false,
    pending: [],
  },
  admin: {
    scheduled: false,
    pending: [],
  },
}

function schedulePluginSlotQueueFlush(scope: PluginSlotRequestScope) {
  const queue = pluginSlotRequestQueues[scope]
  if (queue.scheduled) {
    return
  }
  queue.scheduled = true
  Promise.resolve().then(() => {
    void flushPluginSlotQueue(scope)
  })
}

function normalizePluginSlotRequest(
  scope: PluginSlotRequestScope,
  input: PluginSlotRequestInput
): NormalizedPluginSlotRequest {
  const normalizedPath = normalizePluginPath(
    String(input.path || '').trim() || (scope === 'admin' ? '/admin' : '/')
  )
  const normalizedSlot = String(input.slot || '').trim() || 'default'
  const normalizedQueryParams = normalizePluginStringMap(input.queryParams)
  const normalizedHostContext = normalizePluginHostContext(input.hostContext)

  return {
    key: JSON.stringify({
      path: normalizedPath,
      slot: normalizedSlot,
      locale: String(input.locale || '').trim().toLowerCase() || 'default',
      query_params: stringifyPluginStringMap(normalizedQueryParams),
      host_context: stringifyPluginHostContext(normalizedHostContext),
    }),
    path: normalizedPath,
    slot: normalizedSlot,
    locale: String(input.locale || '').trim().toLowerCase() || 'default',
    queryParams: Object.keys(normalizedQueryParams).length > 0 ? normalizedQueryParams : undefined,
    hostContext:
      Object.keys(normalizedHostContext).length > 0
        ? (normalizedHostContext as Record<string, any>)
        : undefined,
  }
}

function groupPendingPluginSlotRequests(
  items: PendingPluginSlotRequest[]
): PendingPluginSlotRequestGroup[] {
  const groups = new Map<string, PendingPluginSlotRequestGroup>()
  for (const item of items) {
    const existing = groups.get(item.request.key)
    if (existing) {
      existing.requests.push(item)
      continue
    }
    groups.set(item.request.key, {
      key: item.request.key,
      request: item.request,
      requests: [item],
    })
  }
  return Array.from(groups.values())
}

function buildBatchRequestItems(
  groups: PendingPluginSlotRequestGroup[]
): PluginFrontendExtensionBatchRequestItem[] {
  return groups.map((group) => ({
    key: group.key,
    slot: group.request.slot,
    path: group.request.path,
    query_params: group.request.queryParams,
    host_context: group.request.hostContext,
  }))
}

function buildPluginSlotRequestResult(
  request: NormalizedPluginSlotRequest,
  item?: PluginFrontendExtensionBatchResponseItem
): PluginSlotRequestResult {
  return {
    data: {
      path: String(item?.path || request.path || '/'),
      slot: String(item?.slot || request.slot || 'default'),
      extensions: Array.isArray(item?.extensions) ? item.extensions : [],
    },
  }
}

function partitionPendingPluginSlotRequestGroupsByLocale(
  groups: PendingPluginSlotRequestGroup[]
): PendingPluginSlotRequestLocaleBatch[] {
  const batches = new Map<string, PendingPluginSlotRequestLocaleBatch>()
  for (const group of groups) {
    const locale = String(group.request.locale || '').trim().toLowerCase() || 'default'
    const existing = batches.get(locale)
    if (existing) {
      existing.groups.push(group)
      continue
    }
    batches.set(locale, {
      locale,
      groups: [group],
    })
  }
  return Array.from(batches.values())
}

async function fetchSinglePluginSlotRequest(
  scope: PluginSlotRequestScope,
  request: NormalizedPluginSlotRequest,
  signal?: AbortSignal
) {
  return scope === 'admin'
    ? getAdminPluginExtensions(
        request.path,
        request.slot,
        request.queryParams,
        request.hostContext,
        signal,
        request.locale
      )
    : getPluginExtensions(
        request.path,
        request.slot,
        request.queryParams,
        request.hostContext,
        signal,
        request.locale
      )
}

function buildPluginSlotAbortError(): Error {
  if (typeof DOMException !== 'undefined') {
    return new DOMException('Aborted', 'AbortError')
  }
  const error = new Error('Aborted')
  error.name = 'AbortError'
  return error
}

function settlePendingPluginSlotRequest(
  item: PendingPluginSlotRequest,
  settle: () => void
) {
  if (item.settled) {
    return
  }
  item.settled = true
  if (item.cleanupAbort) {
    item.cleanupAbort()
    item.cleanupAbort = undefined
  }
  settle()
}

async function resolvePluginSlotRequestGroupWithSingleFetch(
  scope: PluginSlotRequestScope,
  group: PendingPluginSlotRequestGroup
) {
  const activeRequests = group.requests.filter((item) => !item.aborted && !item.settled)
  if (activeRequests.length === 0) {
    return
  }

  try {
    const response = await fetchSinglePluginSlotRequest(
      scope,
      group.request,
      activeRequests.length === 1 ? activeRequests[0].signal : undefined
    )
    for (const item of activeRequests) {
      settlePendingPluginSlotRequest(item, () => {
        item.resolve(response as PluginSlotRequestResult)
      })
    }
  } catch (error) {
    for (const item of activeRequests) {
      settlePendingPluginSlotRequest(item, () => {
        item.reject(error)
      })
    }
  }
}

async function resolvePluginSlotRequestGroupsWithBatch(
  scope: PluginSlotRequestScope,
  groups: PendingPluginSlotRequestGroup[]
) {
  if (groups.length === 0) {
    return
  }
  if (groups.length === 1) {
    await resolvePluginSlotRequestGroupWithSingleFetch(scope, groups[0])
    return
  }
  try {
    const defaultPath = scope === 'admin' ? '/admin' : '/'
    const locale = groups[0]?.request.locale
    const batchResponse =
      scope === 'admin'
        ? await getAdminPluginExtensionsBatch(
            defaultPath,
            buildBatchRequestItems(groups),
            undefined,
            locale
          )
        : await getPluginExtensionsBatch(
            defaultPath,
            buildBatchRequestItems(groups),
            undefined,
            locale
          )
    const responseItems = Array.isArray(batchResponse?.data?.items)
      ? (batchResponse.data.items as PluginFrontendExtensionBatchResponseItem[])
      : []
    const responseByKey = new Map<string, PluginFrontendExtensionBatchResponseItem>()
    for (const item of responseItems) {
      const key = String(item?.key || '').trim()
      if (!key) {
        continue
      }
      responseByKey.set(key, item)
    }

    const fallbackGroups: PendingPluginSlotRequestGroup[] = []
    for (const group of groups) {
      const matched = responseByKey.get(group.key)
      if (!matched) {
        fallbackGroups.push(group)
        continue
      }
      const result = buildPluginSlotRequestResult(group.request, matched)
      const activeRequests = group.requests.filter((item) => !item.aborted && !item.settled)
      for (const item of activeRequests) {
        settlePendingPluginSlotRequest(item, () => {
          item.resolve(result)
        })
      }
    }

    if (fallbackGroups.length > 0) {
      await Promise.all(
        fallbackGroups.map((group) => resolvePluginSlotRequestGroupWithSingleFetch(scope, group))
      )
    }
  } catch (_error) {
    await Promise.all(groups.map((group) => resolvePluginSlotRequestGroupWithSingleFetch(scope, group)))
  }
}

async function flushPluginSlotQueue(scope: PluginSlotRequestScope) {
  const queue = pluginSlotRequestQueues[scope]
  queue.scheduled = false
  const pending = queue.pending.splice(0, queue.pending.length)
  const activePending = pending.filter((item) => !item.aborted && !item.settled)
  if (activePending.length === 0) {
    return
  }

  const groups = groupPendingPluginSlotRequests(activePending)
  const localeBatches = partitionPendingPluginSlotRequestGroupsByLocale(groups)
  await Promise.all(
    localeBatches.map((batch) => resolvePluginSlotRequestGroupsWithBatch(scope, batch.groups))
  )
}

export function fetchPluginSlotExtensions(
  scope: PluginSlotRequestScope,
  input: PluginSlotRequestInput
): Promise<PluginSlotRequestResult> {
  const request = normalizePluginSlotRequest(scope, input)
  if (input.signal?.aborted) {
    return Promise.reject(buildPluginSlotAbortError())
  }

  return new Promise<PluginSlotRequestResult>((resolve, reject) => {
    const item: PendingPluginSlotRequest = {
      request,
      signal: input.signal,
      aborted: false,
      settled: false,
      resolve,
      reject,
    }
    if (input.signal) {
      const onAbort = () => {
        item.aborted = true
        const queue = pluginSlotRequestQueues[scope]
        const index = queue.pending.indexOf(item)
        if (index >= 0) {
          queue.pending.splice(index, 1)
        }
        settlePendingPluginSlotRequest(item, () => {
          reject(buildPluginSlotAbortError())
        })
      }
      input.signal.addEventListener('abort', onAbort, { once: true })
      item.cleanupAbort = () => {
        input.signal?.removeEventListener('abort', onAbort)
      }
    }
    pluginSlotRequestQueues[scope].pending.push(item)
    schedulePluginSlotQueueFlush(scope)
  })
}

export function resetPluginSlotRequestQueueForTest() {
  for (const scope of Object.keys(pluginSlotRequestQueues) as PluginSlotRequestScope[]) {
    pluginSlotRequestQueues[scope].scheduled = false
    pluginSlotRequestQueues[scope].pending.splice(0, pluginSlotRequestQueues[scope].pending.length)
  }
}
