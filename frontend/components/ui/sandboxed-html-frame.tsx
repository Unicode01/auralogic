'use client'

import { useMemo } from 'react'
import DOMPurify from 'dompurify'
import { cn } from '@/lib/utils'

type Props = {
  html: string
  className?: string
  title?: string
  locale?: string
}

const i18nCSS = `
<style>
[data-locale="zh"] .lang-en, [data-locale="zh"] .lang-en-block { display: none !important; }
[data-locale="en"] .lang-zh, [data-locale="en"] .lang-zh-block { display: none !important; }
.lang-zh, .lang-en { display: inline; }
.lang-zh-block, .lang-en-block { display: block; }
</style>`

function sanitizeHtml(html: string): string {
  return DOMPurify.sanitize(html, {
    ALLOWED_TAGS: [
      'p', 'br', 'strong', 'em', 'u', 's', 'del', 'ins', 'mark', 'sub', 'sup',
      'h1', 'h2', 'h3', 'h4', 'h5', 'h6',
      'ul', 'ol', 'li', 'dl', 'dt', 'dd',
      'blockquote', 'pre', 'code',
      'a', 'img', 'button', 'svg', 'path',
      'table', 'thead', 'tbody', 'tr', 'th', 'td',
      'div', 'span', 'hr', 'style',
    ],
    ALLOWED_ATTR: [
      'href', 'src', 'alt', 'title', 'class', 'target', 'rel',
      'style', 'id', 'type', 'onclick',
      'viewBox', 'fill', 'stroke', 'stroke-width', 'stroke-linecap',
      'stroke-linejoin', 'd',
    ],
    FORBID_TAGS: ['script', 'iframe', 'object', 'embed', 'link', 'meta', 'base', 'input'],
    FORBID_ATTR: ['onerror', 'onload'],
    FORCE_BODY: true,
  })
}

export function SandboxedHtmlFrame({ html, className, locale }: Props) {
  const sanitized = useMemo(() => i18nCSS + sanitizeHtml(html || ''), [html])

  return (
    <div
      data-locale={locale || 'zh'}
      className={cn('w-full rounded-md border bg-background p-3 overflow-auto text-sm', className)}
      dangerouslySetInnerHTML={{ __html: sanitized }}
    />
  )
}
