const DISALLOWED_PROTOCOL_PREFIXES = ['javascript:', 'data:', 'vbscript:', 'file:']
const ALLOWED_ABSOLUTE_PROTOCOLS = new Set(['http:', 'https:', 'mailto:', 'tel:'])

function containsControlChars(value: string): boolean {
  for (let idx = 0; idx < value.length; idx += 1) {
    const code = value.charCodeAt(idx)
    if ((code >= 0 && code <= 31) || code === 127) {
      return true
    }
  }
  return false
}

export function sanitizePluginLinkUrl(rawUrl: string): string | null {
  const trimmed = String(rawUrl || '').trim()
  if (!trimmed) {
    return null
  }
  if (containsControlChars(trimmed)) {
    return null
  }

  const lowered = trimmed.toLowerCase()
  if (DISALLOWED_PROTOCOL_PREFIXES.some((prefix) => lowered.startsWith(prefix))) {
    return null
  }

  if (trimmed.startsWith('//')) {
    return null
  }

  if (
    trimmed.startsWith('/') ||
    trimmed.startsWith('./') ||
    trimmed.startsWith('../') ||
    trimmed.startsWith('#') ||
    trimmed.startsWith('?')
  ) {
    return trimmed
  }

  try {
    const parsed = new URL(trimmed)
    if (!ALLOWED_ABSOLUTE_PROTOCOLS.has(parsed.protocol)) {
      return null
    }
    return parsed.toString()
  } catch {
    return null
  }
}

export function normalizePluginLinkTarget(rawTarget: string | undefined): string {
  const normalized = String(rawTarget || '').trim().toLowerCase()
  if (normalized === '_blank') {
    return '_blank'
  }
  if (normalized === '_parent') {
    return '_parent'
  }
  if (normalized === '_top') {
    return '_top'
  }
  return '_self'
}
