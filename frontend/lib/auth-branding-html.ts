import DOMPurify from 'dompurify'

export function sanitizeAuthBrandingHtml(html: string): string {
  if (typeof window === 'undefined') {
    return ''
  }

  const source = typeof html === 'string' ? html : ''
  if (!source.trim()) {
    return ''
  }

  return DOMPurify.sanitize(source, {
    FORBID_TAGS: ['script', 'iframe', 'object', 'embed'],
    FORBID_ATTR: ['onerror', 'onload', 'onclick', 'onmouseover'],
    FORCE_BODY: true,
  })
}
