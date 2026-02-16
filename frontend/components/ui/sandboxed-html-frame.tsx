'use client'

import { useMemo } from 'react'
import DOMPurify from 'dompurify'
import { cn } from '@/lib/utils'

type Props = {
  html: string
  className?: string
  title?: string
}

function sanitizeHtml(html: string): string {
  // Keep this tight: scripts and active content are stripped, even though iframe sandbox blocks JS.
  return DOMPurify.sanitize(html, {
    ALLOWED_TAGS: [
      'p', 'br', 'strong', 'em', 'u', 's', 'del', 'ins', 'mark', 'sub', 'sup',
      'h1', 'h2', 'h3', 'h4', 'h5', 'h6',
      'ul', 'ol', 'li', 'dl', 'dt', 'dd',
      'blockquote', 'pre', 'code',
      'a', 'img',
      'table', 'thead', 'tbody', 'tr', 'th', 'td',
      'div', 'span', 'hr',
    ],
    ALLOWED_ATTR: ['href', 'src', 'alt', 'title', 'class', 'target', 'rel'],
    FORBID_TAGS: ['script', 'iframe', 'object', 'embed', 'link', 'meta', 'base', 'form', 'input', 'style'],
    FORBID_ATTR: ['style', 'onerror', 'onload'],
  })
}

export function SandboxedHtmlFrame({ html, className, title }: Props) {
  const srcDoc = useMemo(() => {
    const body = sanitizeHtml(html || '')
    return `<!doctype html>
<html>
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src data: http: https:; style-src 'unsafe-inline'; font-src data:; connect-src http: https:; frame-ancestors 'none'; base-uri 'none';" />
    <style>
      :root { color-scheme: light dark; }
      body { margin: 0; padding: 12px; font: 14px/1.5 ui-sans-serif, system-ui, -apple-system, Segoe UI, Roboto, Arial; }
      img { max-width: 100%; height: auto; }
      table { border-collapse: collapse; width: 100%; }
      th, td { border: 1px solid rgba(0,0,0,.15); padding: 6px 8px; }
      a { color: inherit; }
      pre { overflow: auto; }
    </style>
  </head>
  <body>${body}</body>
</html>`
  }, [html])

  return (
    <iframe
      className={cn('w-full rounded-md border bg-background', className)}
      title={title || 'sandboxed-html'}
      // No scripts, no same-origin: prevents token/localStorage theft even if HTML is malicious.
      sandbox="allow-forms allow-popups"
      referrerPolicy="no-referrer"
      srcDoc={srcDoc}
    />
  )
}

