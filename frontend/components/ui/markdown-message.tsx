'use client'
/* eslint-disable @next/next/no-img-element */

import { memo, useEffect, useMemo, useState } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import rehypeRaw from 'rehype-raw'
import { prepareMarkdownContentForRender } from '@/lib/markdown-html-sanitize'
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
  const [hydrated, setHydrated] = useState(false)

  useEffect(() => {
    setHydrated(true)
  }, [])

  const sanitizedContent = useMemo(
    () => prepareMarkdownContentForRender(content, { allowHtml, hydrated }),
    [content, allowHtml, hydrated]
  )

  const enableRawHtml = allowHtml && hydrated

  return (
    <div
      className={cn(
        'max-w-none break-words',
        // 基础文本样式（紧凑模式，适用于聊天消息等）
        'text-sm [&_*]:text-inherit',
        '[&_p]:m-0 [&_p:not(:last-child)]:mb-2',
        '[&_ul]:my-1 [&_ol]:my-1 [&_ul]:pl-4 [&_ol]:pl-4',
        '[&_ul]:list-disc [&_ol]:list-decimal',
        '[&_pre]:my-2 [&_pre]:p-2 [&_pre]:rounded [&_pre]:bg-black/20',
        '[&_code]:px-1 [&_code]:py-0.5 [&_code]:rounded [&_code]:bg-black/20 [&_code]:text-xs',
        '[&_pre_code]:p-0 [&_pre_code]:bg-transparent',
        '[&_blockquote]:border-l-2 [&_blockquote]:pl-2 [&_blockquote]:my-2 [&_blockquote]:opacity-80',
        '[&_a]:underline [&_a]:underline-offset-2',
        // 标题层级区分（即使在紧凑模式也保持区分度）
        '[&_h1]:text-xl [&_h1]:font-bold [&_h1]:mt-3 [&_h1]:mb-2',
        '[&_h2]:text-lg [&_h2]:font-bold [&_h2]:mt-2.5 [&_h2]:mb-1.5',
        '[&_h3]:text-base [&_h3]:font-semibold [&_h3]:mt-2 [&_h3]:mb-1',
        '[&_h4]:text-sm [&_h4]:font-semibold [&_h4]:mt-1.5 [&_h4]:mb-1',
        // 表格基础样式
        '[&_table]:w-full [&_table]:border-collapse [&_table]:my-2 [&_table]:text-xs',
        '[&_th]:border [&_th]:border-border [&_th]:px-2 [&_th]:py-1 [&_th]:bg-muted/50 [&_th]:font-semibold [&_th]:text-left',
        '[&_td]:border [&_td]:border-border [&_td]:px-2 [&_td]:py-1',
        '[&_hr]:my-3 [&_hr]:border-border',
        '[&_img]:max-w-full [&_img]:rounded',
        className
      )}
    >
      <ReactMarkdown
        remarkPlugins={remarkPluginsConfig}
        rehypePlugins={enableRawHtml ? rehypePluginsConfig : undefined}
        components={markdownComponents}
      >
        {sanitizedContent}
      </ReactMarkdown>
    </div>
  )
})
