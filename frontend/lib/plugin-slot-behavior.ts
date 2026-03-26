export type PluginSlotSkeletonVariant = 'panel' | 'inline' | 'list'
export type PluginPageSlotPosition = 'top' | 'bottom'

export function coercePluginSlotAnimation(value: unknown, fallback: boolean): boolean {
  if (typeof value === 'boolean') {
    return value
  }

  if (typeof value === 'string') {
    const normalized = value.trim().toLowerCase()
    if (['1', 'true', 'yes', 'on'].includes(normalized)) {
      return true
    }
    if (['0', 'false', 'no', 'off'].includes(normalized)) {
      return false
    }
  }

  if (typeof value === 'number') {
    return value !== 0
  }

  return fallback
}

export function resolvePluginGlobalSlotAnimationsEnabled(
  publicConfig: Record<string, any> | undefined | null,
  fallback = true
): boolean {
  const plugin =
    publicConfig?.plugin &&
    typeof publicConfig.plugin === 'object' &&
    !Array.isArray(publicConfig.plugin)
      ? (publicConfig.plugin as Record<string, any>)
      : undefined
  const frontend =
    plugin?.frontend && typeof plugin.frontend === 'object' && !Array.isArray(plugin.frontend)
      ? (plugin.frontend as Record<string, any>)
      : undefined

  if (frontend && 'slot_animations_enabled' in frontend) {
    return coercePluginSlotAnimation(frontend.slot_animations_enabled, fallback)
  }

  return fallback
}

export function resolvePluginPlatformEnabled(
  publicConfig: Record<string, any> | undefined | null,
  fallback = true
): boolean {
  const plugin =
    publicConfig?.plugin &&
    typeof publicConfig.plugin === 'object' &&
    !Array.isArray(publicConfig.plugin)
      ? (publicConfig.plugin as Record<string, any>)
      : undefined
  if (plugin && 'enabled' in plugin) {
    return coercePluginSlotAnimation(plugin.enabled, fallback)
  }

  return fallback
}

export function resolvePluginGlobalSlotLoadingEnabled(
  publicConfig: Record<string, any> | undefined | null,
  fallback = true
): boolean {
  return resolvePluginGlobalSlotAnimationsEnabled(publicConfig, fallback)
}

export function resolvePluginSlotAnimationDefault(
  localAnimate: boolean | undefined,
  globalAnimate = true
): boolean {
  return typeof localAnimate === 'boolean' ? localAnimate : globalAnimate
}

export function resolvePluginPageSlotAnimation(
  page: Record<string, any> | undefined | null,
  position: PluginPageSlotPosition,
  fallback = true
): boolean {
  if (!page || typeof page !== 'object') {
    return fallback
  }

  const slots =
    page.slots && typeof page.slots === 'object' && !Array.isArray(page.slots)
      ? (page.slots as Record<string, any>)
      : undefined
  const scoped =
    slots?.[position] && typeof slots[position] === 'object' && !Array.isArray(slots[position])
      ? (slots[position] as Record<string, any>)
      : undefined

  if (scoped && 'animate' in scoped) {
    return coercePluginSlotAnimation(scoped.animate, fallback)
  }

  const positionKey = position === 'top' ? 'top_slot_animate' : 'bottom_slot_animate'
  if (positionKey in page) {
    return coercePluginSlotAnimation(page[positionKey], fallback)
  }

  if (slots && 'animate' in slots) {
    return coercePluginSlotAnimation(slots.animate, fallback)
  }

  if ('slot_animate' in page) {
    return coercePluginSlotAnimation(page.slot_animate, fallback)
  }

  return fallback
}

export function shouldAutoDeferPluginSlot(
  slot: string,
  display: 'stack' | 'inline' = 'stack'
): boolean {
  if (display !== 'stack') {
    return false
  }

  const normalizedSlot = String(slot || '')
    .trim()
    .toLowerCase()

  if (!normalizedSlot) {
    return false
  }

  return (
    normalizedSlot.endsWith('.bottom') ||
    normalizedSlot.endsWith('.footer') ||
    normalizedSlot.endsWith('.after_list') ||
    normalizedSlot.endsWith('.after_table') ||
    normalizedSlot.endsWith('.after_content') ||
    normalizedSlot.endsWith('.before_checkout')
  )
}

export function resolvePluginSlotSkeletonVariant(
  slot: string,
  display: 'stack' | 'inline' = 'stack'
): PluginSlotSkeletonVariant {
  if (display === 'inline') {
    return 'inline'
  }

  const normalizedSlot = String(slot || '')
    .trim()
    .toLowerCase()

  if (
    normalizedSlot.endsWith('.before_list') ||
    normalizedSlot.endsWith('.after_list') ||
    normalizedSlot.endsWith('.before_table') ||
    normalizedSlot.endsWith('.after_table') ||
    normalizedSlot.endsWith('.before_content') ||
    normalizedSlot.endsWith('.after_content')
  ) {
    return 'list'
  }

  return 'panel'
}
