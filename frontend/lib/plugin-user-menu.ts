import type { PluginFrontendBootstrapMenuItem } from '@/lib/api'
import { manifestString } from '@/lib/package-manifest-schema'

export type UserPluginMenuItem = {
  id: string
  title: string
  href: string
  iconName: string
  guestVisible: boolean
  priority: number
}

function normalizeUserPluginMenuPath(path: string): string {
  const trimmed = (path || '').trim()
  if (!trimmed) return ''
  const normalized = trimmed.startsWith('/') ? trimmed : `/${trimmed}`
  return normalized.replace(/\/+$/, '') || '/'
}

export function parseUserPluginMenuItems(
  source: PluginFrontendBootstrapMenuItem[],
  locale?: string
): UserPluginMenuItem[] {
  const out: UserPluginMenuItem[] = []
  const seen = new Set<string>()

  source.forEach((item, index) => {
    if (!item || typeof item !== 'object') return
    const href = normalizeUserPluginMenuPath(String(item.path || ''))
    if (!href || !href.startsWith('/plugin-pages/')) return
    const title = manifestString(item as Record<string, unknown>, 'title', locale)
    if (!title) return
    const id = String(item.id || '').trim() || `runtime-user-menu-${index}`
    if (seen.has(id) || seen.has(href)) return
    seen.add(id)
    seen.add(href)
    out.push({
      id,
      title,
      href,
      iconName: String(item.icon || '').trim(),
      guestVisible: !!item.guest_visible,
      priority: Number.isFinite(item.priority) ? Number(item.priority) : 0,
    })
  })

  out.sort((a, b) => {
    if (a.priority === b.priority) return a.href.localeCompare(b.href)
    return a.priority - b.priority
  })

  return out
}
