import type { AdminPlugin } from '@/lib/api'

export type PluginManifestPagePaths = {
  adminPath: string
  userPath: string
}

export type PluginUploadNameConflict = {
  pluginId: number
  pluginName: string
  pluginDisplayName: string
}

export type PluginUploadPageConflict = {
  area: 'admin' | 'user'
  path: string
  pluginId: number
  pluginName: string
  pluginDisplayName: string
}

export type PluginUploadConflictSummary = {
  manifestPagePaths: PluginManifestPagePaths
  nameConflict: PluginUploadNameConflict | null
  pageConflicts: PluginUploadPageConflict[]
  hasConflict: boolean
}

function asObject(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return null
  }
  return value as Record<string, unknown>
}

function parseManifestObject(value: unknown): Record<string, unknown> | null {
  if (typeof value === 'string') {
    const trimmed = value.trim()
    if (!trimmed) return null
    try {
      const parsed = JSON.parse(trimmed)
      return asObject(parsed)
    } catch {
      return null
    }
  }
  return asObject(value)
}

function normalizePluginName(value: unknown): string {
  return String(value || '')
    .trim()
    .toLowerCase()
}

function normalizePluginPagePath(value: unknown, area: 'admin' | 'user'): string {
  const trimmed = String(value || '').trim()
  if (!trimmed) return ''
  const normalized = trimmed.startsWith('/') ? trimmed : `/${trimmed}`
  const clean = normalized.replace(/\/+$/, '') || '/'
  const prefix = area === 'admin' ? '/admin/plugin-pages/' : '/plugin-pages/'
  if (!clean.startsWith(prefix)) {
    return ''
  }
  return clean
}

export function resolvePluginManifestPagePaths(manifestValue: unknown): PluginManifestPagePaths {
  const manifest = parseManifestObject(manifestValue)
  if (!manifest) {
    return { adminPath: '', userPath: '' }
  }

  const frontend = asObject(manifest.frontend)
  const adminPage = asObject(frontend?.admin_page ?? manifest.admin_page)
  const userPage = asObject(frontend?.user_page ?? manifest.user_page)

  return {
    adminPath: normalizePluginPagePath(
      adminPage?.path ?? frontend?.admin_page_path ?? manifest.admin_page_path,
      'admin'
    ),
    userPath: normalizePluginPagePath(
      userPage?.path ?? frontend?.user_page_path ?? manifest.user_page_path,
      'user'
    ),
  }
}

function normalizePluginId(value: unknown): number {
  const parsed = Number(value)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : 0
}

export function resolvePluginUploadConflictSummary(input: {
  manifest: unknown
  plugins: AdminPlugin[]
  targetPluginID?: unknown
  pluginName?: unknown
}): PluginUploadConflictSummary {
  const manifestPagePaths = resolvePluginManifestPagePaths(input.manifest)
  const targetPluginID = normalizePluginId(input.targetPluginID)
  const normalizedPluginName = normalizePluginName(input.pluginName)

  let nameConflict: PluginUploadNameConflict | null = null
  if (!targetPluginID && normalizedPluginName) {
    const matchedPlugin = input.plugins.find(
      (plugin) => normalizePluginName(plugin.name) === normalizedPluginName
    )
    if (matchedPlugin) {
      nameConflict = {
        pluginId: matchedPlugin.id,
        pluginName: matchedPlugin.name || '',
        pluginDisplayName: matchedPlugin.display_name || matchedPlugin.name || '',
      }
    }
  }

  const effectiveTargetPluginID = targetPluginID || nameConflict?.pluginId || 0
  const pageConflicts: PluginUploadPageConflict[] = []

  input.plugins.forEach((plugin) => {
    if (!plugin || plugin.id === effectiveTargetPluginID) {
      return
    }
    const existingPagePaths = resolvePluginManifestPagePaths(plugin.manifest)
    if (manifestPagePaths.adminPath && manifestPagePaths.adminPath === existingPagePaths.adminPath) {
      pageConflicts.push({
        area: 'admin',
        path: manifestPagePaths.adminPath,
        pluginId: plugin.id,
        pluginName: plugin.name || '',
        pluginDisplayName: plugin.display_name || plugin.name || '',
      })
    }
    if (manifestPagePaths.userPath && manifestPagePaths.userPath === existingPagePaths.userPath) {
      pageConflicts.push({
        area: 'user',
        path: manifestPagePaths.userPath,
        pluginId: plugin.id,
        pluginName: plugin.name || '',
        pluginDisplayName: plugin.display_name || plugin.name || '',
      })
    }
  })

  return {
    manifestPagePaths,
    nameConflict,
    pageConflicts,
    hasConflict: !!nameConflict || pageConflicts.length > 0,
  }
}
