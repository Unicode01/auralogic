'use client'

import { memo, useMemo } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import rehypeRaw from 'rehype-raw'
import DOMPurify from 'dompurify'
import { cn } from '@/lib/utils'

interface MarkdownMessageProps {
  content: string
  className?: string
  isOwnMessage?: boolean
  allowHtml?: boolean
}

// 安全的URL协议白名单
const ALLOWED_PROTOCOLS = ['http:', 'https:', 'mailto:']

// 验证URL是否安全
function isSafeUrl(url: string): boolean {
  try {
    const parsed = new URL(url, 'https://example.com')
    return ALLOWED_PROTOCOLS.includes(parsed.protocol)
  } catch {
    // 相对URL是安全的
    return !url.startsWith('javascript:') && !url.startsWith('data:')
  }
}

// 提取为模块级常量，避免每次渲染创建新引用
const remarkPluginsConfig = [remarkGfm]
const rehypePluginsConfig = [rehypeRaw]

const markdownComponents = {
  a: ({ node, href, ...props }: any) => {
    const safeHref = href && isSafeUrl(href) ? href : '#'
    return (
      <a
        {...props}
        href={safeHref}
        target="_blank"
        rel="noopener noreferrer"
        onClick={(e: React.MouseEvent) => {
          if (safeHref === '#') {
            e.preventDefault()
          }
        }}
      />
    )
  },
  script: () => null,
  iframe: () => null,
  img: ({ node, src, alt, ...props }: any) => {
    const safeSrc = src && isSafeUrl(src) ? src : ''
    if (!safeSrc) return null
    return <img {...props} src={safeSrc} alt={alt || ''} loading="lazy" />
  },
}

export const MarkdownMessage = memo(function MarkdownMessage({ content, className, isOwnMessage, allowHtml = false }: MarkdownMessageProps) {
  // 对输入内容进行基础清理
  const sanitizedContent = useMemo(() => {
    if (typeof window === 'undefined') {
      // SSR: 仅做基础清理
      return content
        .replace(/<script\b[^<]*(?:(?!<\/script>)<[^<]*)*<\/script>/gi, '')
        .replace(/javascript:/gi, '')
        .replace(/on\w+=/gi, '')
    }
    // 客户端: 使用DOMPurify清理可能的HTML注入
    if (allowHtml) {
      // 允许HTML标签，但过滤危险内容
      return DOMPurify.sanitize(content, {
        ALLOWED_TAGS: [
          'p', 'br', 'strong', 'em', 'u', 's', 'del', 'ins', 'mark', 'sub', 'sup',
          'h1', 'h2', 'h3', 'h4', 'h5', 'h6',
          'ul', 'ol', 'li', 'dl', 'dt', 'dd',
          'blockquote', 'pre', 'code',
          'a', 'img',
          'table', 'thead', 'tbody', 'tr', 'th', 'td',
          'div', 'span', 'hr',
        ],
        ALLOWED_ATTR: ['href', 'src', 'alt', 'title', 'class', 'id', 'target', 'rel', 'style'],
        ALLOWED_URI_REGEXP: /^(?:(?:(?:f|ht)tps?|mailto|tel|callto|sms|cid|xmpp):|[^a-z]|[a-z+.\-]+(?:[^a-z+.\-:]|$))/i,
      })
    } else {
      // 不允许HTML标签，只保留文本
      return DOMPurify.sanitize(content, {
        ALLOWED_TAGS: [],
        KEEP_CONTENT: true,
      })
    }
  }, [content, allowHtml])

  return (
    <div
      className={cn(
        'text-sm prose prose-sm max-w-none break-words',
        // 继承父元素的文字颜色，而不是使用 prose-invert
        '[&_*]:text-inherit',
        '[&_p]:m-0 [&_p:not(:last-child)]:mb-2',
        '[&_ul]:my-1 [&_ol]:my-1 [&_ul]:pl-4 [&_ol]:pl-4',
        '[&_pre]:my-2 [&_pre]:p-2 [&_pre]:rounded [&_pre]:bg-black/20',
        '[&_code]:px-1 [&_code]:py-0.5 [&_code]:rounded [&_code]:bg-black/20 [&_code]:text-xs',
        '[&_blockquote]:border-l-2 [&_blockquote]:pl-2 [&_blockquote]:my-2 [&_blockquote]:opacity-80',
        '[&_a]:underline [&_a]:underline-offset-2',
        className
      )}
    >
      <ReactMarkdown
        remarkPlugins={remarkPluginsConfig}
        rehypePlugins={allowHtml ? rehypePluginsConfig : undefined}
        components={markdownComponents}
      >
        {sanitizedContent}
      </ReactMarkdown>
    </div>
  )
})
