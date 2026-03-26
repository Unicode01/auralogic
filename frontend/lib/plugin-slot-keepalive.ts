import { normalizePluginPath } from '@/lib/plugin-frontend-routing'
import { type PluginSlotRequestScope } from '@/lib/plugin-slot-request'

const PLUGIN_SLOT_KEEPALIVE_TTL_MS = 20 * 1000

type PluginSlotKeepaliveValue = {
  expiresAt: number
  value: {
    data: {
      path: string
      slot: string
      extensions: any[]
    }
  }
}

const pluginSlotKeepaliveStore = new Map<string, PluginSlotKeepaliveValue>()

export function buildPluginSlotKeepaliveKey(
  scope: PluginSlotRequestScope,
  path: string,
  slot: string,
  locale?: string
): string {
  return JSON.stringify({
    scope,
    path: normalizePluginPath(path || (scope === 'admin' ? '/admin' : '/')),
    slot: String(slot || '').trim() || 'default',
    locale: String(locale || '').trim().toLowerCase() || 'default',
  })
}

function pruneExpiredPluginSlotKeepalive() {
  const now = Date.now()
  for (const [key, entry] of pluginSlotKeepaliveStore.entries()) {
    if (entry.expiresAt <= now) {
      pluginSlotKeepaliveStore.delete(key)
    }
  }
}

export function readPluginSlotKeepalive<T = any>(key: string): T | undefined {
  pruneExpiredPluginSlotKeepalive()
  const entry = pluginSlotKeepaliveStore.get(key)
  if (!entry) {
    return undefined
  }
  if (entry.expiresAt <= Date.now()) {
    pluginSlotKeepaliveStore.delete(key)
    return undefined
  }
  return entry.value as T
}

export function writePluginSlotKeepalive<T = any>(
  key: string,
  value: T,
  ttlMs = PLUGIN_SLOT_KEEPALIVE_TTL_MS
) {
  pluginSlotKeepaliveStore.set(key, {
    expiresAt: Date.now() + Math.max(0, ttlMs),
    value: value as PluginSlotKeepaliveValue['value'],
  })
}

export function resetPluginSlotKeepaliveForTest() {
  pluginSlotKeepaliveStore.clear()
}
