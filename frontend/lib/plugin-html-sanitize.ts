import DOMPurify from 'dompurify'

const PLUGIN_HTML_FORBID_TAGS = [
  'script',
  'iframe',
  'object',
  'embed',
  'link',
  'meta',
  'base',
]

const PLUGIN_HTML_FORBID_ATTR = [
  /^on/i, // block all inline event handlers
  'srcdoc',
]

const PLUGIN_HTML_IMG_TAG_PATTERN = /<img\b[^>]*?>/gi

function readPluginHTMLAttribute(tag: string, attributeName: string): string {
  const pattern = new RegExp(
    `${attributeName}\\s*=\\s*(?:"([^"]*)"|'([^']*)'|([^\\s"'=<>` + '`' + `]+))`,
    'i'
  )
  const match = tag.match(pattern)
  if (!match) {
    return ''
  }
  return String(match[1] ?? match[2] ?? match[3] ?? '').trim()
}

function parsePluginPositiveNumber(value: string): number | null {
  if (!value) {
    return null
  }
  const parsed = Number.parseFloat(value)
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return null
  }
  return parsed
}

function normalizePluginStyleDeclaration(styleValue: string): string {
  return styleValue
    .split(';')
    .map((item) => item.trim())
    .filter(Boolean)
    .join('; ')
}

function hasPluginStyleProperty(styleValue: string, propertyName: string): boolean {
  const normalizedProperty = String(propertyName || '')
    .trim()
    .toLowerCase()
  if (!normalizedProperty) {
    return false
  }
  return normalizePluginStyleDeclaration(styleValue)
    .split(';')
    .some((item) => {
      const separatorIndex = item.indexOf(':')
      if (separatorIndex <= 0) {
        return false
      }
      return item.slice(0, separatorIndex).trim().toLowerCase() === normalizedProperty
    })
}

function appendPluginStyleDeclaration(styleValue: string, declarations: string[]): string {
  const existing = normalizePluginStyleDeclaration(styleValue)
  const next = declarations
    .map((item) => item.trim())
    .filter(Boolean)
    .filter((item) => {
      const separatorIndex = item.indexOf(':')
      if (separatorIndex <= 0) {
        return false
      }
      return !hasPluginStyleProperty(existing, item.slice(0, separatorIndex))
    })

  if (next.length === 0) {
    return existing
  }

  return existing ? `${existing}; ${next.join('; ')}` : next.join('; ')
}

function injectPluginHTMLAttribute(tag: string, attributeName: string, attributeValue: string): string {
  const suffixMatch = tag.match(/\s*\/?>$/)
  if (!suffixMatch) {
    return tag
  }
  const suffix = suffixMatch[0]
  return `${tag.slice(0, -suffix.length)} ${attributeName}="${attributeValue}"${suffix}`
}

function upsertPluginHTMLStyleAttribute(tag: string, styleValue: string): string {
  if (!styleValue.trim()) {
    return tag
  }
  if (/style\s*=/i.test(tag)) {
    return tag.replace(
      /style\s*=\s*(?:"[^"]*"|'[^']*'|[^\s"'=<>`]+)/i,
      `style="${styleValue}"`
    )
  }
  return injectPluginHTMLAttribute(tag, 'style', styleValue)
}

function enhancePluginHTMLImageTag(tag: string): string {
  let nextTag = tag
  const loading = readPluginHTMLAttribute(nextTag, 'loading')
  if (!loading) {
    nextTag = injectPluginHTMLAttribute(nextTag, 'loading', 'lazy')
  }

  const decoding = readPluginHTMLAttribute(nextTag, 'decoding')
  if (!decoding) {
    nextTag = injectPluginHTMLAttribute(nextTag, 'decoding', 'async')
  }

  const width = parsePluginPositiveNumber(readPluginHTMLAttribute(nextTag, 'width'))
  const height = parsePluginPositiveNumber(readPluginHTMLAttribute(nextTag, 'height'))
  const currentStyle = readPluginHTMLAttribute(nextTag, 'style')
  const nextStyle = appendPluginStyleDeclaration(currentStyle, [
    'max-width: 100%',
    'height: auto',
    width && height ? `aspect-ratio: ${width} / ${height}` : '',
  ])

  return upsertPluginHTMLStyleAttribute(nextTag, nextStyle)
}

function enhancePluginHTMLImages(html: string): string {
  if (!html.trim()) {
    return ''
  }
  return html.replace(PLUGIN_HTML_IMG_TAG_PATTERN, (tag) => enhancePluginHTMLImageTag(tag))
}

export function sanitizePluginHtml(html: string): string {
  const source = typeof html === 'string' ? html : ''
  if (!source.trim()) {
    return ''
  }

  return DOMPurify.sanitize(source, {
    FORBID_TAGS: PLUGIN_HTML_FORBID_TAGS,
    FORBID_ATTR: PLUGIN_HTML_FORBID_ATTR as unknown as string[],
    FORCE_BODY: true,
  })
}

export function preparePluginHtmlForRender(
  html: string,
  options?: {
    trusted?: boolean
  }
): string {
  const source = typeof html === 'string' ? html : ''
  if (!source.trim()) {
    return ''
  }

  const normalized = options?.trusted ? source : sanitizePluginHtml(source)
  if (!normalized.trim()) {
    return ''
  }

  return enhancePluginHTMLImages(normalized)
}
