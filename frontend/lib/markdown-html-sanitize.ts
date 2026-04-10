import DOMPurify from 'dompurify'

const MARKDOWN_HTML_ALLOWED_TAGS = [
  'p', 'br', 'strong', 'em', 'u', 's', 'del', 'ins', 'mark', 'sub', 'sup',
  'h1', 'h2', 'h3', 'h4', 'h5', 'h6',
  'ul', 'ol', 'li', 'dl', 'dt', 'dd',
  'blockquote', 'pre', 'code',
  'a', 'img',
  'table', 'thead', 'tbody', 'tr', 'th', 'td',
  'div', 'span', 'hr',
] as const

const MARKDOWN_HTML_ALLOWED_ATTR = [
  'href', 'src', 'alt', 'title', 'class', 'id', 'target', 'rel', 'style',
] as const

const MARKDOWN_HTML_ALLOWED_URI_REGEXP =
  /^(?:(?:(?:f|ht)tps?|mailto|tel|callto|sms|cid|xmpp):|[^a-z]|[a-z+.\-]+(?:[^a-z+.\-:]|$))/i

const SCRIPT_STYLE_IFRAME_PATTERN =
  /<(script|style|iframe)\b[^<]*(?:(?!<\/\1>)<[^<]*)*<\/\1>/gi
const HTML_TAG_PATTERN = /<\/?[a-zA-Z][^>]*>/g

function normalizeMarkdownContent(content: string): string {
  return typeof content === 'string' ? content : ''
}

export function stripMarkdownHtmlForStaticRender(content: string): string {
  const source = normalizeMarkdownContent(content)
  if (!source.trim()) {
    return ''
  }

  return source
    .replace(SCRIPT_STYLE_IFRAME_PATTERN, '')
    .replace(HTML_TAG_PATTERN, '')
}

export function sanitizeMarkdownHtml(content: string): string {
  const source = normalizeMarkdownContent(content)
  if (!source.trim()) {
    return ''
  }

  return DOMPurify.sanitize(source, {
    ALLOWED_TAGS: [...MARKDOWN_HTML_ALLOWED_TAGS],
    ALLOWED_ATTR: [...MARKDOWN_HTML_ALLOWED_ATTR],
    ALLOWED_URI_REGEXP: MARKDOWN_HTML_ALLOWED_URI_REGEXP,
  })
}

export function stripMarkdownHtmlToText(content: string): string {
  const source = normalizeMarkdownContent(content)
  if (!source.trim()) {
    return ''
  }

  return DOMPurify.sanitize(source, {
    ALLOWED_TAGS: [],
    KEEP_CONTENT: true,
  })
}

export function prepareMarkdownContentForRender(
  content: string,
  options?: {
    allowHtml?: boolean
    hydrated?: boolean
  }
): string {
  const source = normalizeMarkdownContent(content)
  if (!source.trim()) {
    return ''
  }

  if (!options?.allowHtml) {
    return options?.hydrated ? stripMarkdownHtmlToText(source) : stripMarkdownHtmlForStaticRender(source)
  }

  if (!options.hydrated) {
    return stripMarkdownHtmlForStaticRender(source)
  }

  return sanitizeMarkdownHtml(source)
}
