'use client'

import { type ReactNode } from 'react'

import { useLocale } from '@/hooks/use-locale'
import { PluginPageLink } from '@/components/plugins/plugin-page-link'
import { PluginStructuredBlock } from '@/components/plugins/plugin-structured-block'
import { Button, type ButtonProps } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { type PluginFrontendExtension } from '@/lib/api'
import { preparePluginHtmlForRender } from '@/lib/plugin-html-sanitize'
import { normalizePluginLinkTarget, sanitizePluginLinkUrl } from '@/lib/plugin-link-sanitize'
import { resolvePluginMenuIcon } from '@/lib/plugin-menu-icons'
import { manifestString } from '@/lib/package-manifest-schema'
import { cn } from '@/lib/utils'

type PluginHTMLPresentation = {
  chrome: 'card' | 'bare'
  theme: 'default' | 'host'
}

type RenderedPluginExtension = {
  inline: boolean
  node: ReactNode
}

type PluginButtonPresentation = {
  href: string
  label: string
  target: string
  external: boolean
  icon?: string
  variant?: ButtonProps['variant']
  size?: ButtonProps['size']
}

type PluginExtensionListProps = {
  extensions: PluginFrontendExtension[]
  className?: string
  display?: 'stack' | 'inline'
}

const pluginButtonTypes = new Set(['action_button', 'toolbar_button', 'button'])

function normalizePluginHTMLPresentation(
  data: Record<string, any> | undefined
): PluginHTMLPresentation {
  const chromeRaw = String(data?.chrome || data?.container || '')
    .trim()
    .toLowerCase()
  const themeRaw = String(data?.theme || data?.presentation || '')
    .trim()
    .toLowerCase()

  return {
    chrome: chromeRaw === 'bare' || chromeRaw === 'plain' || chromeRaw === 'none' ? 'bare' : 'card',
    theme: themeRaw === 'host' ? 'host' : 'default',
  }
}

function isPluginButtonType(type: string): boolean {
  return pluginButtonTypes.has(type.trim().toLowerCase())
}

function normalizePluginButtonVariant(value: unknown): ButtonProps['variant'] {
  const normalized = String(value || '')
    .trim()
    .toLowerCase()
  if (
    normalized === 'default' ||
    normalized === 'active' ||
    normalized === 'destructive' ||
    normalized === 'outline' ||
    normalized === 'secondary' ||
    normalized === 'ghost' ||
    normalized === 'link'
  ) {
    return normalized
  }
  return 'outline'
}

function normalizePluginButtonSize(value: unknown): ButtonProps['size'] {
  const normalized = String(value || '')
    .trim()
    .toLowerCase()
  if (normalized === 'sm' || normalized === 'lg' || normalized === 'icon') {
    return normalized
  }
  return 'sm'
}

function resolvePluginButtonPresentation(
  extension: PluginFrontendExtension,
  locale?: string
): PluginButtonPresentation | null {
  const data = extension.data && typeof extension.data === 'object' ? extension.data : undefined
  const rawHref =
    extension.link ||
    String(data?.href || '').trim() ||
    String(data?.url || '').trim() ||
    String(data?.path || '').trim()
  const href = sanitizePluginLinkUrl(rawHref)
  if (!href) {
    return null
  }
  const label =
    manifestString(data, 'label', locale) ||
    manifestString(extension as Record<string, unknown>, 'title', locale) ||
    manifestString(extension as Record<string, unknown>, 'content', locale) ||
    href
  const target = normalizePluginLinkTarget(String(data?.target || extension.metadata?.target || ''))
  const external =
    target !== '_self' || /^(https?:|mailto:|tel:)/i.test(href) || data?.external === true

  return {
    href,
    label,
    target,
    external,
    icon: String(data?.icon || '').trim() || undefined,
    variant: normalizePluginButtonVariant(data?.variant),
    size: normalizePluginButtonSize(data?.size),
  }
}

function renderPluginButtonExtension(
  extension: PluginFrontendExtension,
  key: string,
  locale?: string
): RenderedPluginExtension | null {
  const presentation = resolvePluginButtonPresentation(extension, locale)
  if (!presentation) {
    return null
  }

  const Icon = resolvePluginMenuIcon(presentation.icon)
  const content = (
    <>
      {presentation.icon ? <Icon className="h-4 w-4" /> : null}
      <span>{presentation.label}</span>
    </>
  )

  if (presentation.external) {
    return {
      inline: true,
      node: (
        <Button key={key} asChild size={presentation.size} variant={presentation.variant} className="gap-2">
          <a
            href={presentation.href}
            target={presentation.target}
            rel={presentation.target === '_blank' ? 'noreferrer noopener' : undefined}
          >
            {content}
          </a>
        </Button>
      ),
    }
  }

  return {
    inline: true,
    node: (
      <Button key={key} asChild size={presentation.size} variant={presentation.variant} className="gap-2">
        <PluginPageLink href={presentation.href} target={presentation.target}>
          {content}
        </PluginPageLink>
      </Button>
    ),
  }
}

function renderExtension(
  extension: PluginFrontendExtension,
  index: number,
  locale?: string
): RenderedPluginExtension | null {
  const key =
    extension.id || `${extension.plugin_id || 'plugin'}-${extension.type || 'block'}-${index}`
  const type = extension.type || 'text'
  const normalizedType = type.trim().toLowerCase()
  const title = manifestString(extension as Record<string, unknown>, 'title', locale)
  const content = manifestString(extension as Record<string, unknown>, 'content', locale)

  if (isPluginButtonType(normalizedType)) {
    return renderPluginButtonExtension(extension, key, locale)
  }

  if (normalizedType === 'html') {
    if (!content) {
      return null
    }
    const htmlMode = String(extension.metadata?.html_mode || '')
      .trim()
      .toLowerCase()
    const renderedHtml = preparePluginHtmlForRender(content, {
      trusted: htmlMode === 'trusted',
    })
    const htmlPresentation = normalizePluginHTMLPresentation(
      extension.data && typeof extension.data === 'object' ? extension.data : undefined
    )
    const hostClassName = htmlPresentation.theme === 'host' ? 'plugin-html-host' : undefined
    if (!renderedHtml) {
      return null
    }
    if (htmlPresentation.chrome === 'bare') {
      return {
        inline: false,
        node: (
          <div key={key} className="space-y-2">
            {title ? (
              <p className="px-1 text-xs font-medium uppercase tracking-[0.08em] text-muted-foreground">
                {title}
              </p>
            ) : null}
            <div className={hostClassName} dangerouslySetInnerHTML={{ __html: renderedHtml }} />
          </div>
        ),
      }
    }
    return {
      inline: false,
      node: (
        <Card key={key}>
          <CardContent className="p-4">
            <div className={hostClassName} dangerouslySetInnerHTML={{ __html: renderedHtml }} />
          </CardContent>
        </Card>
      ),
    }
  }

  return {
    inline: false,
    node: (
      <PluginStructuredBlock
        key={key}
        block={{
          type,
          title,
          content,
          data: extension.data,
        }}
      />
    ),
  }
}

export function PluginExtensionList({
  extensions,
  className,
  display = 'stack',
}: PluginExtensionListProps) {
  const { locale } = useLocale()

  if (!Array.isArray(extensions) || extensions.length === 0) {
    return null
  }

  const renderedExtensions = extensions
    .map((extension, index) => renderExtension(extension, index, locale))
    .filter((item): item is RenderedPluginExtension => item !== null)

  if (renderedExtensions.length === 0) {
    return null
  }

  if (display === 'inline') {
    const inlineItems = renderedExtensions.filter((item) => item.inline)
    const stackedItems = renderedExtensions.filter((item) => !item.inline)
    return (
      <div className={cn(stackedItems.length > 0 ? 'space-y-3' : undefined, className)}>
        {inlineItems.length > 0 ? (
          <div className="flex flex-wrap items-center gap-2">
            {inlineItems.map((item) => item.node)}
          </div>
        ) : null}
        {stackedItems.map((item) => item.node)}
      </div>
    )
  }

  return <div className={cn('space-y-3', className)}>{renderedExtensions.map((item) => item.node)}</div>
}
