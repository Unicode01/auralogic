'use client'

import { useState, type ReactNode } from 'react'
import {
  ArrowUpRight,
  ChevronDown,
  ChevronRight,
  ExternalLink,
  Link2,
  Rows,
  TableProperties,
} from 'lucide-react'

import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { PluginPageLink } from '@/components/plugins/plugin-page-link'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { normalizePluginLinkTarget, sanitizePluginLinkUrl } from '@/lib/plugin-link-sanitize'
import { manifestString } from '@/lib/package-manifest-schema'

type PluginStructuredBlockData = Record<string, any>

type PluginStructuredBlockInput = {
  type?: string
  title?: unknown
  content?: unknown
  data?: PluginStructuredBlockData
}

type PluginLinkItem = {
  label: string
  url: string
  target?: string
}

type PluginTableColumn = {
  key?: string
  label?: string
}

type PluginStatsItem = {
  label?: string
  value?: unknown
  description?: string
}

type PluginKeyValueItem = {
  key?: string
  label?: string
  value?: unknown
  description?: string
}

type PluginJSONViewConfig = {
  value?: unknown
  summary?: string
  collapsible: boolean
  collapsed: boolean
  previewLines: number
  maxHeight: number
}

type StructuredTranslations = ReturnType<typeof getTranslations>

function prettyJSON(value: unknown): string {
  if (typeof value === 'string') {
    try {
      return JSON.stringify(JSON.parse(value), null, 2)
    } catch {
      return value
    }
  }
  try {
    return JSON.stringify(value ?? {}, null, 2)
  } catch {
    return String(value ?? '')
  }
}

function formatDisplayValue(value: unknown): string {
  if (value === null || value === undefined) return '-'
  if (typeof value === 'string') return value.trim() === '' ? '-' : value
  if (typeof value === 'number' || typeof value === 'boolean') return String(value)
  return prettyJSON(value)
}

function truncateDisplayValue(value: unknown, maxLength = 160): string {
  const text = formatDisplayValue(value)
  if (text.length <= maxLength) return text
  return `${text.slice(0, maxLength)}...`
}

function parseLinks(data: PluginStructuredBlockData | undefined, locale?: string): PluginLinkItem[] {
  if (!data || !Array.isArray(data.links)) {
    return []
  }
  const out: PluginLinkItem[] = []
  data.links.forEach((item) => {
    if (!item || typeof item !== 'object' || Array.isArray(item)) return
    const candidate = item as Record<string, any>
    if (typeof candidate.url !== 'string' || candidate.url.trim() === '') return
    const safeURL = sanitizePluginLinkUrl(candidate.url)
    if (!safeURL) return
    out.push({
      label: manifestString(candidate, 'label', locale) || safeURL,
      url: safeURL,
      target: normalizePluginLinkTarget(
        typeof candidate.target === 'string' ? candidate.target : undefined
      ),
    })
  })
  return out
}

function isInternalStructuredLink(link: PluginLinkItem): boolean {
  return link.url.startsWith('/plugin-pages/') || link.url.startsWith('/admin/plugin-pages/')
}

function parseStatsItems(data: PluginStructuredBlockData | undefined, locale?: string): PluginStatsItem[] {
  if (!data || !Array.isArray(data.items)) return []
  const items: Array<PluginStatsItem | null> = data.items.map((item) => {
    if (!item || typeof item !== 'object' || Array.isArray(item)) return null
    const candidate = item as PluginStatsItem
    return {
      label: manifestString(candidate as Record<string, unknown>, 'label', locale),
      value: candidate.value,
      description: manifestString(candidate as Record<string, unknown>, 'description', locale),
    }
  })
  return items.filter((item): item is PluginStatsItem => item !== null)
}

function parseKeyValueItems(data: PluginStructuredBlockData | undefined, locale?: string): PluginKeyValueItem[] {
  if (!data) return []
  if (Array.isArray(data.items)) {
    const out: PluginKeyValueItem[] = []
    data.items.forEach((item) => {
      if (!item || typeof item !== 'object' || Array.isArray(item)) return
      const candidate = item as PluginKeyValueItem
      out.push({
        key: candidate.key,
        label: manifestString(candidate as Record<string, unknown>, 'label', locale),
        value: candidate.value,
        description: manifestString(candidate as Record<string, unknown>, 'description', locale),
      })
    })
    return out
  }
  if (data.values && typeof data.values === 'object' && !Array.isArray(data.values)) {
    return Object.entries(data.values as Record<string, unknown>).map(([key, value]) => ({
      key,
      label: key,
      value,
    }))
  }
  return []
}

function parseBadgeItems(data: PluginStructuredBlockData | undefined, locale?: string): string[] {
  if (!data || !Array.isArray(data.items)) return []
  const seen = new Set<string>()
  const out: string[] = []
  data.items.forEach((item) => {
    const text = manifestString({ value: item }, 'value', locale)
    if (!text || seen.has(text)) return
    seen.add(text)
    out.push(text)
  })
  return out
}

function parseTableConfig(data: PluginStructuredBlockData | undefined, locale?: string): {
  columns: PluginTableColumn[]
  rows: Record<string, unknown>[]
  emptyText: string
} {
  const rows = Array.isArray(data?.rows)
    ? data.rows.filter(
        (item): item is Record<string, unknown> =>
          !!item && typeof item === 'object' && !Array.isArray(item)
      )
    : []
  const explicitColumns: PluginTableColumn[] = []
  if (Array.isArray(data?.columns)) {
    data.columns.forEach((item) => {
      if (typeof item === 'string') {
        explicitColumns.push({ key: item, label: item })
        return
      }
      if (item && typeof item === 'object' && !Array.isArray(item)) {
        const column = item as PluginTableColumn
        const key = String(column.key || '').trim()
        if (!key) return
        explicitColumns.push({
          key,
          label: manifestString(column as Record<string, unknown>, 'label', locale) || key,
        })
      }
    })
  }
  const columns =
    explicitColumns.length > 0
      ? explicitColumns
      : rows.length > 0
        ? Array.from(
            rows.reduce((keys, row) => {
              Object.keys(row).forEach((key) => key && keys.add(key))
              return keys
            }, new Set<string>())
          )
            .sort()
            .map((key) => ({ key, label: key }))
        : []
  const emptyText = manifestString(data, 'empty_text', locale)
  return { columns, rows, emptyText }
}

function parseJSONViewConfig(
  data: PluginStructuredBlockData | undefined,
  fallbackContent: string,
  locale?: string
): PluginJSONViewConfig {
  const previewLinesCandidate = Number(data?.preview_lines)
  const maxHeightCandidate = Number(data?.max_height)
  return {
    value: Object.prototype.hasOwnProperty.call(data || {}, 'value') ? data?.value : fallbackContent,
    summary: manifestString(data, 'summary', locale),
    collapsible: data?.collapsible === true || data?.collapsed === true,
    collapsed: data?.collapsed === true,
    previewLines:
      Number.isFinite(previewLinesCandidate) && previewLinesCandidate > 0
        ? Math.floor(previewLinesCandidate)
        : 8,
    maxHeight:
      Number.isFinite(maxHeightCandidate) && maxHeightCandidate > 0
        ? Math.floor(maxHeightCandidate)
        : 420,
  }
}

function looksLikeJSONValue(value: unknown): boolean {
  if (value && typeof value === 'object') {
    return true
  }
  if (typeof value !== 'string') {
    return false
  }
  const trimmed = value.trim()
  if (!trimmed) return false
  return (
    (trimmed.startsWith('{') && trimmed.endsWith('}')) ||
    (trimmed.startsWith('[') && trimmed.endsWith(']'))
  )
}

function resolveStructuredLink(value: unknown): PluginLinkItem | null {
  if (typeof value !== 'string' || value.trim() === '') {
    return null
  }
  const safeURL = sanitizePluginLinkUrl(value)
  if (!safeURL) return null
  return {
    label: safeURL,
    url: safeURL,
    target: normalizePluginLinkTarget(undefined),
  }
}

function normalizeStructuredFieldKey(value: unknown): string {
  return String(value || '')
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '_')
    .replace(/^_+|_+$/g, '')
}

function humanizeStructuredLabel(value: unknown): string {
  const raw = String(value || '').trim()
  if (!raw) {
    return '-'
  }
  if (!/[._/:-]|[a-z0-9][A-Z]|[A-Z]{2,}[a-z]/.test(raw)) {
    return raw
  }

  const normalized = raw
    .replace(/([A-Z]+)([A-Z][a-z])/g, '$1 $2')
    .replace(/([a-z0-9])([A-Z])/g, '$1 $2')
    .replace(/[._/:-]+/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()

  return normalized
    .split(' ')
    .map((part) => {
      if (!part) return part
      const lower = part.toLowerCase()
      if (['id', 'ip', 'ui', 'db', 'fs', 'js', 'api', 'http', 'https', 'url'].includes(lower)) {
        return lower.toUpperCase()
      }
      if (/^[A-Z0-9]{2,}$/.test(part)) {
        return part
      }
      return `${part.charAt(0).toUpperCase()}${part.slice(1)}`
    })
    .join(' ')
}

function normalizeStructuredLabelIdentity(value: unknown): string {
  return normalizeStructuredFieldKey(humanizeStructuredLabel(value))
}

function shouldShowStructuredKeyBadge(key?: string, label?: string): boolean {
  const normalizedKey = normalizeStructuredLabelIdentity(key)
  const normalizedLabel = normalizeStructuredLabelIdentity(label || key)
  return !!normalizedKey && !!normalizedLabel && normalizedKey !== normalizedLabel
}

function isStructuredBooleanField(fieldKey: string): boolean {
  return (
    fieldKey.startsWith('is_') ||
    fieldKey.startsWith('has_') ||
    fieldKey.startsWith('can_') ||
    fieldKey.includes('enabled') ||
    fieldKey.includes('active') ||
    fieldKey.includes('installed') ||
    fieldKey.includes('activate') ||
    fieldKey.includes('auto_start') ||
    fieldKey.includes('exists') ||
    fieldKey.includes('available')
  )
}

function isStructuredStatusField(fieldKey: string): boolean {
  return (
    fieldKey === 'status' ||
    fieldKey.endsWith('_status') ||
    fieldKey === 'state' ||
    fieldKey.endsWith('_state') ||
    fieldKey === 'health' ||
    fieldKey.endsWith('_health') ||
    fieldKey === 'compatibility' ||
    fieldKey === 'result'
  )
}

function isStructuredPhaseField(fieldKey: string): boolean {
  return fieldKey === 'phase' || fieldKey.endsWith('_phase')
}

function isStructuredKindField(fieldKey: string): boolean {
  return fieldKey === 'kind' || fieldKey.endsWith('_kind')
}

function isStructuredChannelField(fieldKey: string): boolean {
  return fieldKey === 'channel' || fieldKey.endsWith('_channel')
}

function isStructuredProgressField(fieldKey: string): boolean {
  return fieldKey === 'progress' || fieldKey.endsWith('_progress')
}

function isStructuredDateField(fieldKey: string): boolean {
  return (
    fieldKey.endsWith('_at') ||
    fieldKey.includes('time') ||
    fieldKey.includes('date') ||
    fieldKey === 'published'
  )
}

function isStructuredMonospaceField(fieldKey: string): boolean {
  return (
    fieldKey === 'version' ||
    fieldKey.endsWith('_version') ||
    fieldKey.endsWith('_id') ||
    fieldKey.endsWith('_key') ||
    fieldKey.endsWith('_slug') ||
    fieldKey.endsWith('_url') ||
    fieldKey.endsWith('_path') ||
    fieldKey.includes('digest') ||
    fieldKey.includes('checksum') ||
    fieldKey.includes('target') ||
    fieldKey === 'coordinates' ||
    isStructuredDateField(fieldKey)
  )
}

function looksLikeStructuredDateTimeValue(value: unknown): boolean {
  if (typeof value !== 'string') {
    return false
  }
  const trimmed = value.trim()
  return /^\d{4}-\d{2}-\d{2}(?:[Tt ][0-9]{2}:[0-9]{2}(?::[0-9]{2})?(?:\.\d+)?(?:Z|[+-][0-9]{2}:[0-9]{2})?)?$/.test(
    trimmed
  )
}

function looksLikeStructuredVersionValue(value: unknown): boolean {
  if (typeof value !== 'string') {
    return false
  }
  const trimmed = value.trim()
  return /^v?\d+\.\d+\.\d+(?:[-+._][a-z0-9]+)*$/i.test(trimmed)
}

function looksLikeStructuredIdentifier(value: unknown): boolean {
  if (typeof value !== 'string') {
    return false
  }
  const trimmed = value.trim()
  if (trimmed.length < 8) {
    return false
  }
  if (looksLikeStructuredDateTimeValue(trimmed) || looksLikeStructuredVersionValue(trimmed)) {
    return false
  }
  return /^[a-z0-9][a-z0-9._:/@-]+$/i.test(trimmed)
}

function resolveStructuredDisplayText(
  fieldKey: string,
  value: unknown,
  t?: StructuredTranslations
): string {
  if (typeof value === 'boolean') {
    return value ? 'true' : 'false'
  }
  const formatted = formatDisplayValue(value)
  if (
    isStructuredProgressField(fieldKey) &&
    typeof formatted === 'string' &&
    /^\d+(?:\.\d+)?$/.test(formatted.trim())
  ) {
    return `${formatted.trim()}%`
  }
  return formatted
}

function resolveStructuredBadgeMeta(
  fieldKey: string,
  value: unknown,
  t?: StructuredTranslations
):
  | {
      label: string
      variant: 'default' | 'secondary' | 'destructive' | 'outline' | 'active'
      mono?: boolean
    }
  | null {
  const label = resolveStructuredDisplayText(fieldKey, value, t)
  const normalized = label.trim().toLowerCase()
  if (!normalized || normalized === '-' || normalized.length > 40 || normalized.includes('\n')) {
    return null
  }

  if (typeof value === 'boolean') {
    return { label, variant: value ? 'default' : 'secondary' }
  }

  if (isStructuredBooleanField(fieldKey)) {
    if (['true', 'yes', 'on', '1'].includes(normalized)) {
      return { label, variant: 'default' }
    }
    if (['false', 'no', 'off', '0'].includes(normalized)) {
      return { label, variant: 'secondary' }
    }
  }

  if (isStructuredProgressField(fieldKey) && /^\d+(?:\.\d+)?%$/.test(normalized)) {
    return { label, variant: 'outline', mono: true }
  }

  if (isStructuredKindField(fieldKey)) {
    return { label, variant: 'secondary', mono: true }
  }

  if (isStructuredChannelField(fieldKey)) {
    return { label, variant: 'outline', mono: true }
  }

  if (isStructuredPhaseField(fieldKey)) {
    return { label, variant: 'outline', mono: true }
  }

  if (isStructuredStatusField(fieldKey)) {
    if (
      [
        'ok',
        'compatible',
        'success',
        'succeeded',
        'completed',
        'healthy',
        'active',
        'activated',
        'imported',
        'installed',
        'ready',
        'rolled_back',
        'already_active',
      ].includes(normalized)
    ) {
      return { label, variant: 'default', mono: true }
    }
    if (
      [
        'running',
        'pending',
        'processing',
        'installing',
        'queued',
        'in_progress',
        'loading',
      ].includes(normalized)
    ) {
      return { label, variant: 'active', mono: true }
    }
    if (
      [
        'warning',
        'incompatible',
        'rejected',
        'failed',
        'error',
        'activate_failed',
        'rollback_failed',
        'unhealthy',
      ].includes(normalized)
    ) {
      return { label, variant: 'destructive', mono: true }
    }
    if (['unknown', 'idle', 'paused', 'draft'].includes(normalized)) {
      return { label, variant: 'secondary', mono: true }
    }
    return { label, variant: 'outline', mono: true }
  }

  return null
}

function resolveStructuredValueTextClass(fieldKey: string, value: unknown, compact: boolean): string {
  if (
    isStructuredMonospaceField(fieldKey) ||
    looksLikeStructuredDateTimeValue(value) ||
    looksLikeStructuredVersionValue(value) ||
    looksLikeStructuredIdentifier(value)
  ) {
    return compact
      ? 'break-all font-mono text-xs text-muted-foreground'
      : 'break-all font-mono text-sm text-muted-foreground'
  }
  return compact
    ? 'break-words text-xs text-muted-foreground'
    : 'break-words text-sm text-muted-foreground'
}

function isStructuredWideValue(value: unknown): boolean {
  const formatted = formatDisplayValue(value)
  return looksLikeJSONValue(value) || formatted.includes('\n') || formatted.length > 140
}

function isStructuredCompactColumn(fieldKey: string): boolean {
  return (
    isStructuredStatusField(fieldKey) ||
    isStructuredPhaseField(fieldKey) ||
    isStructuredKindField(fieldKey) ||
    isStructuredChannelField(fieldKey) ||
    isStructuredProgressField(fieldKey) ||
    isStructuredDateField(fieldKey) ||
    fieldKey === 'version' ||
    fieldKey.endsWith('_version') ||
    fieldKey.endsWith('_id') ||
    fieldKey.endsWith('_key')
  )
}

function renderHeader(title: string, content: string, meta?: ReactNode) {
  if (!title && !content && !meta) return null
  return (
    <CardHeader className="pb-2">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0 flex-1 space-y-1">
          {title ? <CardTitle className="text-base break-words">{title}</CardTitle> : null}
          {content ? <CardDescription>{content}</CardDescription> : null}
        </div>
        {meta ? <div className="flex shrink-0 flex-wrap gap-2">{meta}</div> : null}
      </div>
    </CardHeader>
  )
}

function PluginStructuredValue({
  value,
  compact = false,
  fieldKey,
}: {
  value: unknown
  compact?: boolean
  fieldKey?: string
}) {
  const { locale } = useLocale()
  const t = getTranslations(locale)
  const normalizedFieldKey = normalizeStructuredFieldKey(fieldKey)
  const link = resolveStructuredLink(value)
  const formatted = resolveStructuredDisplayText(normalizedFieldKey, value, t)
  const multiline = formatted.includes('\n')
  const jsonLike = looksLikeJSONValue(value)
  const badgeMeta = resolveStructuredBadgeMeta(normalizedFieldKey, value, t)

  if (link) {
    const internal = isInternalStructuredLink(link)
    const label = truncateDisplayValue(formatted, compact ? 88 : 140)
    const className =
      'inline-flex max-w-full items-center gap-1 break-all text-primary hover:underline'
    return internal ? (
      <PluginPageLink
        href={link.url}
        target={link.target || '_self'}
        className={`${className} ${compact ? 'text-xs' : 'text-sm'}`}
        title={formatted}
      >
        <span className={compact ? 'font-mono' : undefined}>{label}</span>
        <ArrowUpRight className="h-3.5 w-3.5 shrink-0" />
      </PluginPageLink>
    ) : (
      <a
        href={link.url}
        target={link.target || '_self'}
        rel={link.target === '_blank' ? 'noopener noreferrer' : undefined}
        className={`${className} ${compact ? 'text-xs' : 'text-sm'}`}
        title={formatted}
      >
        <span className={compact ? 'font-mono' : undefined}>{label}</span>
        <ExternalLink className="h-3.5 w-3.5 shrink-0" />
      </a>
    )
  }

  if (badgeMeta) {
    return (
      <Badge
        variant={badgeMeta.variant}
        className={`${badgeMeta.mono ? 'font-mono' : ''} max-w-full break-all whitespace-normal ${compact ? 'text-[11px]' : 'text-xs'}`.trim()}
      >
        {badgeMeta.label}
      </Badge>
    )
  }

  if (!compact && (jsonLike || multiline || formatted.length > 180)) {
    return (
      <pre className="max-h-56 overflow-auto rounded-md border border-input/50 bg-muted/30 p-3 text-xs leading-5 text-muted-foreground">
        {formatted}
      </pre>
    )
  }

  if (compact && (jsonLike || multiline || formatted.length > 120)) {
    return (
      <span
        className="block break-words font-mono text-xs text-muted-foreground"
        title={formatted}
      >
        {truncateDisplayValue(formatted, 180)}
      </span>
    )
  }

  return (
    <span className={resolveStructuredValueTextClass(normalizedFieldKey, value, compact)} title={formatted}>
      {formatted}
    </span>
  )
}

function PluginStructuredJSONBlock({
  title,
  content,
  data,
  t,
}: {
  title: string
  content: string
  data: PluginStructuredBlockData
  t: ReturnType<typeof getTranslations>
}) {
  const { locale } = useLocale()
  const config = parseJSONViewConfig(data, content, locale)
  const rendered = prettyJSON(config.value)
  const [expanded, setExpanded] = useState(!(config.collapsible && config.collapsed))

  if (!rendered.trim()) return null

  const lines = rendered.split('\n')
  const preview = lines.slice(0, config.previewLines).join('\n')
  const hasPreviewOverflow = lines.length > config.previewLines

  return (
    <Card>
      <CardHeader className="pb-2">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="min-w-0 flex-1 space-y-1">
            {title ? <CardTitle className="text-base break-words">{title}</CardTitle> : null}
            {content ? <CardDescription>{content}</CardDescription> : null}
            {config.summary ? <CardDescription className="text-xs">{config.summary}</CardDescription> : null}
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant="outline" className="gap-1 text-[11px]">
              {t.admin.pluginStructuredJsonLineCount.replace('{count}', String(lines.length))}
            </Badge>
            <Badge variant="outline" className="gap-1 text-[11px]">
              {t.admin.pluginStructuredJsonCharCount.replace('{count}', String(rendered.length))}
            </Badge>
            {config.collapsible ? (
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => setExpanded((current) => !current)}
                aria-expanded={expanded}
                className="h-7 gap-1 px-2 text-xs"
              >
                {expanded ? (
                  <ChevronDown className="h-3.5 w-3.5" />
                ) : (
                  <ChevronRight className="h-3.5 w-3.5" />
                )}
                {expanded ? t.common.collapse : t.common.expand}
              </Button>
            ) : null}
          </div>
        </div>
      </CardHeader>
      <CardContent className="space-y-3">
        {!expanded && config.collapsible ? (
          <div className="rounded border border-input/50 bg-muted/20 p-3">
            <pre className="overflow-x-auto text-xs leading-5 text-muted-foreground">
              {preview}
              {hasPreviewOverflow ? '\n...' : ''}
            </pre>
          </div>
        ) : null}
        {expanded ? (
          <pre
            className="overflow-auto rounded border border-input/50 bg-muted/30 p-3 text-xs leading-5"
            style={{ maxHeight: `${config.maxHeight}px` }}
          >
            {rendered}
          </pre>
        ) : null}
      </CardContent>
    </Card>
  )
}

export function PluginStructuredBlock({ block }: { block: PluginStructuredBlockInput }) {
  const { locale } = useLocale()
  const t = getTranslations(locale)
  const type = String(block.type || 'text').trim().toLowerCase()
  const title = manifestString(block as Record<string, unknown>, 'title', locale)
  const content = manifestString(block as Record<string, unknown>, 'content', locale)
  const data = block.data && typeof block.data === 'object' ? block.data : {}

  if (type === 'link_list') {
    const links = parseLinks(data, locale)
    if (links.length === 0) return null
    return (
      <Card>
        {renderHeader(
          title,
          content,
          <Badge variant="outline" className="gap-1 text-[11px]">
            <Link2 className="h-3 w-3" />
            {links.length}
          </Badge>
        )}
        <CardContent className="grid gap-3 md:grid-cols-[repeat(2,minmax(0,1fr))] xl:grid-cols-[repeat(3,minmax(0,1fr))]">
          {links.map((link, index) => {
            const linkCard = (
              <div className="min-w-0 rounded-md border border-input/60 bg-muted/10 p-3 transition-colors hover:border-primary/40 hover:bg-muted/20">
                <div className="flex items-start justify-between gap-3">
                  <p className="min-w-0 flex-1 break-words text-sm font-medium">{link.label}</p>
                  <Badge variant="outline" className="shrink-0">
                    {isInternalStructuredLink(link) ? (
                      <Link2 className="h-3 w-3" />
                    ) : (
                      <ExternalLink className="h-3 w-3" />
                    )}
                  </Badge>
                </div>
                <p className="mt-2 break-all font-mono text-xs text-muted-foreground">{link.url}</p>
              </div>
            )

            return isInternalStructuredLink(link) ? (
              <PluginPageLink
                key={`${link.url}-${index}`}
                href={link.url}
                target={link.target || '_self'}
                className="block"
              >
                {linkCard}
              </PluginPageLink>
            ) : (
              <a
                key={`${link.url}-${index}`}
                href={link.url}
                target={link.target || '_self'}
                rel={link.target === '_blank' ? 'noopener noreferrer' : undefined}
                className="block"
              >
                {linkCard}
              </a>
            )
          })}
        </CardContent>
      </Card>
    )
  }

  if (type === 'stats_grid') {
    const items = parseStatsItems(data, locale)
    if (items.length === 0) return null
    return (
      <Card>
        {renderHeader(
          title,
          content,
          <Badge variant="outline" className="gap-1 text-[11px]">
            <Rows className="h-3 w-3" />
            {items.length}
          </Badge>
        )}
        <CardContent className="grid gap-3 md:grid-cols-[repeat(2,minmax(0,1fr))] xl:grid-cols-[repeat(4,minmax(0,1fr))]">
          {items.map((item, index) => {
            const statText = formatDisplayValue(item.value)
            const compactValue = statText.includes('\n') || statText.length > 40
            return (
              <div
                key={`${item.label || 'stat'}-${index}`}
                className="min-w-0 rounded-md border border-input/60 bg-muted/10 p-3"
              >
                <p className="text-xs text-muted-foreground">{item.label || '-'}</p>
                <p
                  className={`mt-1 break-all font-semibold ${
                    compactValue ? 'font-mono text-sm' : 'text-2xl'
                  }`}
                  title={compactValue ? statText : undefined}
                >
                  {compactValue ? truncateDisplayValue(statText, 120) : statText}
                </p>
                {item.description ? (
                  <p className="mt-2 text-xs text-muted-foreground">{item.description}</p>
                ) : null}
              </div>
            )
          })}
        </CardContent>
      </Card>
    )
  }

  if (type === 'key_value') {
    const items = parseKeyValueItems(data, locale)
    if (items.length === 0) return null
    return (
      <Card>
        {renderHeader(
          title,
          content,
          <Badge variant="outline" className="gap-1 text-[11px]">
            <Rows className="h-3 w-3" />
            {items.length}
          </Badge>
        )}
        <CardContent className="grid items-start gap-3 xl:grid-cols-[repeat(3,minmax(0,1fr))]">
          {items.map((item, index) => {
            const normalizedFieldKey = normalizeStructuredFieldKey(item.key || item.label)
            const wideValue = isStructuredWideValue(item.value)
            const displayLabel = humanizeStructuredLabel(item.label || item.key || '-')
            const showKeyBadge = shouldShowStructuredKeyBadge(item.key, item.label)
            return (
              <div
                key={`${item.key || item.label || 'kv'}-${index}`}
                className={`min-w-0 overflow-hidden rounded-md border border-input/60 bg-background p-3 ${
                  wideValue ? 'xl:col-span-2' : ''
                }`.trim()}
              >
                <div className="flex min-w-0 flex-wrap items-start justify-between gap-2">
                  <p className="min-w-0 flex-1 break-words text-sm font-medium">{displayLabel}</p>
                  {item.key && showKeyBadge ? (
                    <Badge
                      variant="outline"
                      className="max-w-full break-all whitespace-normal font-mono text-[11px]"
                    >
                      {item.key}
                    </Badge>
                  ) : null}
                </div>
                <div className="mt-3">
                  <PluginStructuredValue value={item.value} fieldKey={normalizedFieldKey} />
                </div>
                {item.description ? (
                  <p className="mt-2 text-xs text-muted-foreground">{item.description}</p>
                ) : null}
              </div>
            )
          })}
        </CardContent>
      </Card>
    )
  }

  if (type === 'badge_list') {
    const badges = parseBadgeItems(data, locale)
    if (badges.length === 0) return null
    return (
      <Card>
        {renderHeader(
          title,
          content,
          <Badge variant="outline" className="gap-1 text-[11px]">
            <Rows className="h-3 w-3" />
            {badges.length}
          </Badge>
        )}
        <CardContent className="flex flex-wrap gap-2">
          {badges.map((badge, index) => (
            <Badge key={`${badge}-${index}`} variant="secondary">
              {badge}
            </Badge>
          ))}
        </CardContent>
      </Card>
    )
  }

  if (type === 'json_view') {
    return <PluginStructuredJSONBlock title={title} content={content} data={data} t={t} />
  }

  if (type === 'table') {
    const table = parseTableConfig(data, locale)
    return (
      <Card>
        {renderHeader(
          title,
          content,
          <>
            <Badge variant="outline" className="gap-1 text-[11px]">
              <Rows className="h-3 w-3" />
              {table.rows.length}
            </Badge>
            <Badge variant="outline" className="gap-1 text-[11px]">
              <TableProperties className="h-3 w-3" />
              {table.columns.length}
            </Badge>
          </>
        )}
        <CardContent>
          {table.columns.length === 0 || table.rows.length === 0 ? (
            <p className="text-sm text-muted-foreground">{table.emptyText || t.common.noData}</p>
          ) : (
            <div className="max-h-[520px] overflow-auto rounded-md border border-input/50">
              <table className="min-w-full text-sm">
                <thead className="sticky top-0 z-10 bg-muted/95 text-left backdrop-blur">
                  <tr>
                    <th className="sticky left-0 z-20 w-[1%] border-b border-r border-input/40 bg-muted/95 px-3 py-2 text-xs font-medium text-muted-foreground">
                      #
                    </th>
                    {table.columns.map((column) => (
                      <th
                        key={column.key || column.label}
                        className={`border-b border-input/40 px-3 py-2 font-medium ${
                          isStructuredCompactColumn(
                            normalizeStructuredFieldKey(column.key || column.label)
                          )
                            ? 'whitespace-nowrap'
                            : 'min-w-[12rem]'
                        }`}
                      >
                        {column.label || column.key}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {table.rows.map((row, rowIndex) => (
                    <tr
                      key={rowIndex}
                      className="border-t border-input/30 align-top transition-colors hover:bg-muted/20 odd:bg-background even:bg-muted/10"
                    >
                      <td
                        className={`sticky left-0 z-10 border-r border-input/30 px-3 py-2 text-xs text-muted-foreground ${
                          rowIndex % 2 === 0 ? 'bg-background' : 'bg-muted/10'
                        }`}
                      >
                        {rowIndex + 1}
                      </td>
                      {table.columns.map((column) => (
                        <td
                          key={`${rowIndex}-${column.key}`}
                          className={`px-3 py-2 ${
                            isStructuredCompactColumn(
                              normalizeStructuredFieldKey(column.key || column.label)
                            )
                              ? 'whitespace-nowrap'
                              : ''
                          }`}
                        >
                          <PluginStructuredValue
                            compact
                            value={column.key ? row[column.key] : ''}
                            fieldKey={column.key || column.label}
                          />
                        </td>
                      ))}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>
    )
  }

  if (type === 'alert') {
    const variant = String(data.variant || '').trim().toLowerCase()
    const toneClass =
      variant === 'error'
        ? 'border-destructive/40 bg-destructive/10 text-destructive'
      : variant === 'warning'
          ? 'border-amber-500/40 bg-amber-500/10 text-amber-700 dark:border-amber-500/50 dark:bg-amber-950/20 dark:text-amber-300'
        : variant === 'info' || variant === 'notice'
            ? 'border-sky-500/40 bg-sky-500/10 text-sky-700 dark:text-sky-300'
          : variant === 'success'
            ? 'border-emerald-500/40 bg-emerald-500/10 text-emerald-700 dark:border-emerald-500/50 dark:bg-emerald-950/20 dark:text-emerald-300'
            : 'border-input/60 bg-muted/30 text-foreground'
    const text = content || manifestString(data, 'message', locale)
    if (!title && !text) return null
    return (
      <Card>
        <CardContent className={`p-4 ${toneClass}`}>
          {title ? <p className="text-sm font-medium">{title}</p> : null}
          {text ? (
            <p className={title ? 'mt-2 text-sm whitespace-pre-wrap' : 'text-sm whitespace-pre-wrap'}>
              {text}
            </p>
          ) : null}
        </CardContent>
      </Card>
    )
  }

  if (!title && !content) return null
  return (
    <Card>
      {title ? (
        <CardHeader className="pb-2">
          <CardTitle className="text-base">{title}</CardTitle>
        </CardHeader>
      ) : null}
      {content ? (
        <CardContent>
          <PluginStructuredValue value={content} />
        </CardContent>
      ) : null}
    </Card>
  )
}
