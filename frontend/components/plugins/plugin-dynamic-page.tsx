'use client'

import axios, { type AxiosInstance } from 'axios'
import Link from 'next/link'
import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { usePathname, useRouter, useSearchParams } from 'next/navigation'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import {
  apiClient,
  executePluginRouteAction,
  executePluginRouteActionStream,
  getPublicConfig,
  pluginRouteShouldStream,
  type PluginFrontendBootstrapRoute,
  type PluginFrontendRouteExecuteAPI,
  type PluginRouteStreamChunk,
} from '@/lib/api'
import {
  buildPluginFullPath as buildFullPath,
  buildPluginQueryString as buildQueryString,
  matchPluginRoute as matchRoute,
  normalizePluginStringMap as normalizeStringMap,
  readPluginSearchParams,
  stringifyPluginStringMap as stringifyStringMap,
} from '@/lib/plugin-frontend-routing'
import { resolvePluginOperationErrorMessage } from '@/lib/api-error'
import { useLocale } from '@/hooks/use-locale'
import { useAuth } from '@/hooks/use-auth'
import { usePermission } from '@/hooks/use-permission'
import { useVisibilityActivation } from '@/hooks/use-visibility-activation'
import { getTranslations } from '@/lib/i18n'
import { setAuthReturnState } from '@/lib/auth-return-state'
import { usePluginBootstrapQuery } from '@/lib/plugin-bootstrap-query'
import { preparePluginHtmlForRender } from '@/lib/plugin-html-sanitize'
import { normalizePluginLinkTarget, sanitizePluginLinkUrl } from '@/lib/plugin-link-sanitize'
import { prefetchPluginPageData, resolvePluginPagePrefetchTarget } from '@/lib/plugin-page-prefetch'
import { manifestString } from '@/lib/package-manifest-schema'
import {
  resolvePluginPlatformEnabled,
  resolvePluginGlobalSlotAnimationsEnabled,
  resolvePluginGlobalSlotLoadingEnabled,
  resolvePluginPageSlotAnimation,
  resolvePluginSlotAnimationDefault,
  resolvePluginSlotSkeletonVariant,
} from '@/lib/plugin-slot-behavior'
import { usePluginSlotExtensionsQuery } from '@/lib/plugin-slot-query'
import {
  PluginSlotContent,
  shouldShowPluginSlotRefreshing,
} from '@/components/plugins/plugin-slot-content'
import { PluginStructuredBlock } from '@/components/plugins/plugin-structured-block'
import { PluginDynamicPageSkeleton, PluginSlotSkeleton } from '@/components/plugins/plugin-loading'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'

type PluginDynamicPageProps = {
  area: 'admin' | 'user'
  slotAnimate?: boolean
}

type PluginPageBridgeExecuteInput = {
  action: string
  params?: Record<string, unknown>
  mode?: string
  onChunk?: (chunk: PluginRouteStreamChunk) => void | Promise<void>
}

type PluginPageBridge = {
  version: 1
  area: 'admin' | 'user'
  plugin_id: number
  plugin_name?: string
  path: string
  full_path: string
  query_params: Record<string, string>
  route_params: Record<string, string>
  html_mode: 'sanitize' | 'trusted'
  execute_api?: PluginFrontendRouteExecuteAPI
  axios: AxiosInstance
  axios_raw: typeof axios
  execute: (input: PluginPageBridgeExecuteInput) => Promise<any>
  should_stream: (action: string, preferredMode?: string) => boolean
}

declare global {
  interface Window {
    axios?: AxiosInstance
    pluginAxios?: AxiosInstance
    AuraLogicPluginPage?: PluginPageBridge
  }
}

type PluginPageBlock = {
  type?: string
  title?: unknown
  content?: unknown
  data?: Record<string, any>
}

type PluginActionFormFieldOption = {
  label?: string
  value?: string | number | boolean
  key?: string
}

type PluginActionFormConditionMatcher = {
  field?: string
  equals?: string | number | boolean
  in?: Array<string | number | boolean>
  not_equals?: string | number | boolean
  not_in?: Array<string | number | boolean>
  truthy?: boolean
  falsy?: boolean
}

type PluginActionFormField = {
  key?: string
  type?: string
  label?: unknown
  description?: unknown
  placeholder?: unknown
  rows?: number
  options?: PluginActionFormFieldOption[] | Array<string | number | boolean>
  required?: boolean
  visible_when?:
    | Record<string, unknown>
    | PluginActionFormConditionMatcher
    | PluginActionFormConditionMatcher[]
  required_when?:
    | Record<string, unknown>
    | PluginActionFormConditionMatcher
    | PluginActionFormConditionMatcher[]
}

type PluginActionFormExtraAction = {
  key?: string
  label?: unknown
  action?: string
  variant?: 'default' | 'outline' | 'secondary' | 'destructive'
  include_fields?: boolean
  required_fields?: string[]
  visible_when?:
    | Record<string, unknown>
    | PluginActionFormConditionMatcher
    | PluginActionFormConditionMatcher[]
}

type PluginActionFormPreset = {
  key?: string
  label?: unknown
  description?: unknown
  values?: Record<string, unknown>
}

type PluginActionFormActions = {
  load?: string
  save?: string
  reset?: string
  load_label?: unknown
  save_label?: unknown
  reset_label?: unknown
  load_required_fields?: unknown
  save_required_fields?: unknown
  reset_required_fields?: unknown
  load_visible_when?: unknown
  save_visible_when?: unknown
  reset_visible_when?: unknown
  collapse_extra?: boolean
  max_primary_actions?: unknown
  extra?: unknown[]
  buttons?: unknown[]
}

type PluginPageSchema = {
  title?: string
  description?: string
  host_header?: 'show' | 'hide'
  host_market_workspace?: boolean
  blocks?: PluginPageBlock[]
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

type PluginHTMLPresentation = {
  chrome: 'card' | 'bare'
  theme: 'default' | 'host'
}

type ParsedPluginActionFormPreset = {
  key: string
  label: string
  description: string
  values: Record<string, unknown>
}

type ParsedPluginActionFormAction = {
  key: string
  label: string
  action: string
  variant: 'default' | 'outline' | 'secondary' | 'destructive'
  includeFields: boolean
  requiredFields: string[]
  visibleWhen:
    | Record<string, unknown>
    | PluginActionFormConditionMatcher
    | PluginActionFormConditionMatcher[]
    | undefined
}

type RenderablePluginActionButton = {
  key: string
  label: string
  action: string
  variant: 'default' | 'outline' | 'secondary' | 'destructive'
  includeFields: boolean
  requiredFields: string[]
  order: number
  priority: number
}

type NormalizedPluginActionFormConditionMatcher = PluginActionFormConditionMatcher & {
  field: string
}

type PluginActionFormRecentEntry = {
  key: string
  label: string
  summary: string
  updated_at: string
  values: Record<string, string | boolean>
}

function extractRoutes(data: any): PluginFrontendBootstrapRoute[] {
  if (Array.isArray(data?.data?.routes)) {
    return data.data.routes as PluginFrontendBootstrapRoute[]
  }
  if (Array.isArray(data?.routes)) {
    return data.routes as PluginFrontendBootstrapRoute[]
  }
  return []
}

function extractPageSchema(
  route: PluginFrontendBootstrapRoute | null,
  locale?: string
): PluginPageSchema {
  if (!route || !route.page || typeof route.page !== 'object') {
    return {}
  }
  const page = route.page as Record<string, any>
  return {
    title: manifestString(page, 'title', locale) || undefined,
    description: manifestString(page, 'description', locale) || undefined,
    host_header:
      String(page.host_header || '')
        .trim()
        .toLowerCase() === 'hide'
        ? 'hide'
        : 'show',
    host_market_workspace:
      page.host_market_workspace === true ||
      String(page.host_market_workspace || '')
        .trim()
        .toLowerCase() === 'true',
    blocks: Array.isArray(page.blocks) ? page.blocks : undefined,
  }
}

function parseLinks(
  data: Record<string, any> | undefined,
  locale?: string
): Array<{ label: string; url: string; target?: string }> {
  if (!data || !Array.isArray(data.links)) {
    return []
  }
  const out: Array<{ label: string; url: string; target?: string }> = []
  data.links.forEach((item) => {
    if (!item || typeof item !== 'object') return
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

function normalizePermissionList(value: unknown): string[] {
  if (!Array.isArray(value)) {
    return []
  }
  const seen = new Set<string>()
  const out: string[] = []
  value.forEach((item) => {
    const normalized = String(item || '')
      .trim()
      .toLowerCase()
    if (!normalized || seen.has(normalized)) return
    seen.add(normalized)
    out.push(normalized)
  })
  return out
}

function boolOrDefault(value: unknown, fallback: boolean): boolean {
  if (typeof value === 'boolean') return value
  if (typeof value === 'string') {
    const normalized = value.trim().toLowerCase()
    if (['1', 'true', 'yes', 'on'].includes(normalized)) return true
    if (['0', 'false', 'no', 'off'].includes(normalized)) return false
  }
  if (typeof value === 'number') return value !== 0
  return fallback
}

function looksLikeExecutePayload(value: unknown): value is Record<string, any> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return false
  }
  const record = value as Record<string, any>
  return (
    Array.isArray(record.blocks) ||
    !!(
      record.view &&
      typeof record.view === 'object' &&
      Array.isArray((record.view as Record<string, any>).blocks)
    ) ||
    !!(record.values && typeof record.values === 'object' && !Array.isArray(record.values)) ||
    !!(record.form && typeof record.form === 'object' && !Array.isArray(record.form)) ||
    !!(record.fields && typeof record.fields === 'object' && !Array.isArray(record.fields)) ||
    !!(record.config && typeof record.config === 'object' && !Array.isArray(record.config)) ||
    !!(record.checker && typeof record.checker === 'object' && !Array.isArray(record.checker)) ||
    typeof record.source === 'string' ||
    typeof record.message === 'string' ||
    typeof record.notice === 'string'
  )
}

function parseExecutePayload(resp: any): Record<string, any> {
  if (!resp || typeof resp !== 'object') {
    return {}
  }

  const candidates: Record<string, any>[] = []
  if (resp.data && typeof resp.data === 'object' && !Array.isArray(resp.data)) {
    const outerData = resp.data as Record<string, any>
    if (outerData.data && typeof outerData.data === 'object' && !Array.isArray(outerData.data)) {
      candidates.push(outerData.data as Record<string, any>)
    }
    candidates.push(outerData)
  }
  if (looksLikeExecutePayload(resp)) {
    candidates.push(resp as Record<string, any>)
  }

  for (const candidate of candidates) {
    if (looksLikeExecutePayload(candidate)) {
      return candidate
    }
  }
  return {}
}

function normalizeExecuteParamValue(value: unknown): string {
  if (typeof value === 'string') return value
  if (typeof value === 'number' || typeof value === 'boolean') return String(value)
  if (value === null || value === undefined) return ''
  return prettyJSON(value)
}

function parseExecuteParamMap(value: unknown): Record<string, string> {
  if (typeof value !== 'string' || value.trim() === '') {
    return {}
  }
  try {
    const parsed = JSON.parse(value)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return {}
    }
    return Object.entries(parsed).reduce<Record<string, string>>((acc, [key, item]) => {
      const normalizedKey = String(key || '').trim()
      if (!normalizedKey) return acc
      acc[normalizedKey] = normalizeExecuteParamValue(item)
      return acc
    }, {})
  } catch {
    return {}
  }
}

function buildPluginTemplateValues(input: {
  area: 'admin' | 'user'
  pagePath: string
  pageFullPath: string
  pluginID: number
  pluginName?: string
  queryParams?: Record<string, string>
  routeParams?: Record<string, string>
  executeAPI?: PluginFrontendRouteExecuteAPI
}): Record<string, string> {
  const executeAPI = input.executeAPI || {}
  const queryParams = normalizeStringMap(input.queryParams)
  const routeParams = normalizeStringMap(input.routeParams)
  const values: Record<string, string> = {
    'plugin.id': String(input.pluginID || 0),
    'plugin.name': String(input.pluginName || '').trim(),
    'plugin.area': input.area,
    'plugin.path': input.pagePath,
    'plugin.full_path': input.pageFullPath,
    'plugin.query_string': buildQueryString(queryParams),
    'plugin.query_params_json': stringifyStringMap(queryParams),
    'plugin.route_params_json': stringifyStringMap(routeParams),
    'plugin.execute_api_url': String(executeAPI.url || '').trim(),
    'plugin.execute_api_method': String(executeAPI.method || 'POST')
      .trim()
      .toUpperCase(),
    'plugin.execute_api_scope': String(executeAPI.scope || '').trim(),
    'plugin.execute_api_requires_auth': String(!!executeAPI.requires_auth),
    'plugin.execute_stream_url': String(executeAPI.stream_url || '').trim(),
    'plugin.execute_stream_format': String(executeAPI.stream_format || '').trim(),
    'plugin.execute_stream_actions': prettyJSON(executeAPI.stream_actions || []),
    'plugin.execute_api_json': prettyJSON(executeAPI),
  }
  Object.entries(queryParams).forEach(([key, value]) => {
    values[`plugin.query.${key}`] = value
  })
  Object.entries(routeParams).forEach(([key, value]) => {
    values[`plugin.route.${key}`] = value
  })
  return values
}

function normalizePluginBridgeParams(value?: Record<string, unknown>): Record<string, string> {
  const output: Record<string, string> = {}
  if (!value || typeof value !== 'object') {
    return output
  }
  Object.entries(value).forEach(([key, item]) => {
    const normalizedKey = String(key || '').trim()
    if (!normalizedKey) {
      return
    }
    output[normalizedKey] = item === undefined || item === null ? '' : String(item)
  })
  return output
}

function interpolatePluginTemplate(template: string, values: Record<string, string>): string {
  if (!template) return ''
  return template.replace(/\{\{\s*([a-zA-Z0-9._-]+)\s*\}\}/g, (_matched, key: string) => {
    const normalizedKey = String(key || '').trim()
    return Object.prototype.hasOwnProperty.call(values, normalizedKey) ? values[normalizedKey] : ''
  })
}

function setBridgeElementValue(element: Element, value: string) {
  if (element instanceof HTMLInputElement || element instanceof HTMLTextAreaElement) {
    element.value = value
    return
  }
  element.textContent = value
}

function findBridgeTargets(root: HTMLElement, attributeName: string, target: string): Element[] {
  return Array.from(root.querySelectorAll(`[${attributeName}]`)).filter((element) => {
    const attrValue = (element.getAttribute(attributeName) || '').trim()
    if (!target) {
      return attrValue === '' || attrValue === 'true'
    }
    return attrValue === '' || attrValue === 'true' || attrValue === target
  })
}

function writeBridgeValue(root: HTMLElement, attributeName: string, target: string, value: string) {
  findBridgeTargets(root, attributeName, target).forEach((element) => {
    setBridgeElementValue(element, value)
  })
}

function clearBridgeFeedback(root: HTMLElement, target: string) {
  writeBridgeValue(root, 'data-plugin-exec-error', target, '')
  writeBridgeValue(root, 'data-plugin-exec-result', target, '')
}

function writeAxiosBridgeValue(
  root: HTMLElement,
  attributeName: string,
  target: string,
  value: string
) {
  writeBridgeValue(root, attributeName, target, value)
}

function clearAxiosBridgeFeedback(root: HTMLElement, target: string) {
  writeAxiosBridgeValue(root, 'data-plugin-axios-error', target, '')
  writeAxiosBridgeValue(root, 'data-plugin-axios-result', target, '')
}

function parseAxiosBridgeBody(value: string): unknown {
  const normalized = String(value || '').trim()
  if (!normalized) {
    return undefined
  }
  try {
    return JSON.parse(normalized)
  } catch {
    return normalized
  }
}

function findPluginBridgeTrigger(eventTarget: EventTarget | null): HTMLElement | null {
  if (eventTarget instanceof HTMLElement) {
    const direct =
      eventTarget.closest('[data-plugin-exec-action]') ||
      eventTarget.closest('[data-plugin-axios-url]')
    if (direct instanceof HTMLElement) {
      return direct
    }
  }

  const path =
    typeof Event !== 'undefined' &&
    eventTarget &&
    typeof (eventTarget as EventTarget & { composedPath?: () => EventTarget[] }).composedPath ===
      'function'
      ? (eventTarget as EventTarget & { composedPath: () => EventTarget[] }).composedPath()
      : []

  for (const item of path) {
    if (!(item instanceof HTMLElement)) {
      continue
    }
    if (
      item.hasAttribute('data-plugin-exec-action') ||
      item.hasAttribute('data-plugin-axios-url')
    ) {
      return item
    }
  }
  return null
}

function findPluginAnchorTrigger(eventTarget: EventTarget | null): HTMLAnchorElement | null {
  if (eventTarget instanceof Element) {
    const direct = eventTarget.closest('a[href]')
    if (direct instanceof HTMLAnchorElement) {
      return direct
    }
  }

  const path =
    typeof Event !== 'undefined' &&
    eventTarget &&
    typeof (eventTarget as EventTarget & { composedPath?: () => EventTarget[] }).composedPath ===
      'function'
      ? (eventTarget as EventTarget & { composedPath: () => EventTarget[] }).composedPath()
      : []

  for (const item of path) {
    if (item instanceof HTMLAnchorElement && item.hasAttribute('href')) {
      return item
    }
  }

  return null
}

function shouldHandlePluginAnchorClick(event: Event): boolean {
  if (event.defaultPrevented) {
    return false
  }
  if (!(event instanceof MouseEvent)) {
    return true
  }
  if (event.button !== 0) {
    return false
  }
  if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) {
    return false
  }
  return true
}

function resolvePluginAnchorNavigation(anchor: HTMLAnchorElement): {
  href: string
  pluginPrefetchHref?: string
  pluginPrefetchTarget?: ReturnType<typeof resolvePluginPagePrefetchTarget>
} | null {
  const rawHref = String(anchor.getAttribute('href') || '').trim()
  const href = sanitizePluginLinkUrl(rawHref)
  if (!href || href.startsWith('#') || typeof window === 'undefined') {
    return null
  }

  const target = normalizePluginLinkTarget(anchor.getAttribute('target') || anchor.target || '')
  if (target !== '_self' || anchor.hasAttribute('download')) {
    return null
  }

  try {
    const currentURL = new URL(window.location.href)
    const resolvedURL = new URL(href, currentURL)
    if (resolvedURL.origin !== currentURL.origin) {
      return null
    }

    if (
      resolvedURL.pathname === currentURL.pathname &&
      resolvedURL.search === currentURL.search &&
      resolvedURL.hash &&
      resolvedURL.hash !== currentURL.hash
    ) {
      return null
    }

    const pluginPrefetchTarget = resolvePluginPagePrefetchTarget(href, currentURL.toString())
    return {
      href: `${resolvedURL.pathname}${resolvedURL.search}${resolvedURL.hash}`,
      pluginPrefetchHref: pluginPrefetchTarget?.href,
      pluginPrefetchTarget,
    }
  } catch {
    return null
  }
}

function resolvePluginErrorText(
  input: unknown,
  t: ReturnType<typeof getTranslations>,
  fallbackText: string
): string {
  return resolvePluginOperationErrorMessage(input, t, fallbackText)
}

function isExecuteFailed(resp: any): boolean {
  if (!resp || typeof resp !== 'object') return false
  if (resp.success === false) return true
  if (resp.data && typeof resp.data === 'object' && resp.data.success === false) return true
  return false
}

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

function formatAxiosBridgeResponse(response: any): string {
  if (!response || typeof response !== 'object') {
    return prettyJSON(response)
  }
  const record = response as Record<string, any>
  return prettyJSON({
    status: record.status,
    statusText: record.statusText,
    headers: record.headers,
    data: record.data,
  })
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
  return `${text.slice(0, maxLength)}…`
}

function normalizeActionFormFieldType(
  value: unknown
): 'boolean' | 'textarea' | 'number' | 'select' | 'json' | 'string' {
  const normalized = String(value || '')
    .trim()
    .toLowerCase()
  switch (normalized) {
    case 'boolean':
    case 'bool':
      return 'boolean'
    case 'textarea':
      return 'textarea'
    case 'number':
      return 'number'
    case 'select':
      return 'select'
    case 'json':
      return 'json'
    default:
      return 'string'
  }
}

function normalizeActionFormFieldOptions(
  field: PluginActionFormField,
  locale?: string
): PluginActionFormFieldOption[] {
  if (!Array.isArray(field.options)) {
    return []
  }
  const out: PluginActionFormFieldOption[] = []
  field.options.forEach((item) => {
    if (item && typeof item === 'object' && !Array.isArray(item)) {
      const option = item as PluginActionFormFieldOption
      const value = option.value ?? option.key
      if (value === undefined || value === null) return
      out.push({
        label: manifestString(option as Record<string, unknown>, 'label', locale) || String(value),
        value,
      })
      return
    }
    out.push({
      label: String(item),
      value: item as string | number | boolean,
    })
  })
  return out
}

function normalizeActionFormComparableValue(value: unknown): string {
  if (typeof value === 'boolean') {
    return value ? 'true' : 'false'
  }
  if (typeof value === 'number') {
    return String(value)
  }
  return String(value ?? '').trim()
}

function normalizeActionFormConditionList(
  value: unknown
): NormalizedPluginActionFormConditionMatcher[] {
  if (!value) {
    return []
  }
  if (Array.isArray(value)) {
    const matchers = value
      .map((item) => {
        if (!item || typeof item !== 'object' || Array.isArray(item)) {
          return null
        }
        const candidate = item as PluginActionFormConditionMatcher
        const field = String(candidate.field || '').trim()
        if (!field) {
          return null
        }
        return {
          field,
          equals: candidate.equals,
          in: Array.isArray(candidate.in) ? candidate.in : undefined,
          not_equals: candidate.not_equals,
          not_in: Array.isArray(candidate.not_in) ? candidate.not_in : undefined,
          truthy: candidate.truthy === true ? true : undefined,
          falsy: candidate.falsy === true ? true : undefined,
        }
      })
      .filter(Boolean) as NormalizedPluginActionFormConditionMatcher[]
    return matchers
  }
  if (value && typeof value === 'object') {
    const candidate = value as Record<string, unknown>
    if (
      'field' in candidate ||
      'equals' in candidate ||
      'in' in candidate ||
      'not_equals' in candidate ||
      'not_in' in candidate ||
      'truthy' in candidate ||
      'falsy' in candidate
    ) {
      return normalizeActionFormConditionList([candidate])
    }
    const matchers = Object.entries(candidate)
      .map(([field, expected]) => {
        const normalizedField = String(field || '').trim()
        if (!normalizedField) {
          return null
        }
        if (Array.isArray(expected)) {
          return {
            field: normalizedField,
            in: expected.filter(
              (item): item is string | number | boolean =>
                typeof item === 'string' || typeof item === 'number' || typeof item === 'boolean'
            ),
          }
        }
        if (
          typeof expected === 'string' ||
          typeof expected === 'number' ||
          typeof expected === 'boolean'
        ) {
          return {
            field: normalizedField,
            equals: expected,
          }
        }
        return null
      })
      .filter(Boolean) as NormalizedPluginActionFormConditionMatcher[]
    return matchers
  }
  return []
}

function matchesActionFormConditions(
  conditions: unknown,
  values: Record<string, string | boolean>
): boolean {
  const matchers = normalizeActionFormConditionList(conditions)
  if (matchers.length === 0) {
    return true
  }
  return matchers.every((matcher) => {
    const fieldKey = String(matcher.field || '').trim()
    if (!fieldKey) {
      return true
    }
    const rawValue = values[fieldKey]
    const comparable = normalizeActionFormComparableValue(rawValue)
    if (matcher.truthy === true) {
      if (typeof rawValue === 'boolean') {
        if (!rawValue) return false
      } else if (!comparable) {
        return false
      }
    }
    if (matcher.falsy === true) {
      if (typeof rawValue === 'boolean') {
        if (rawValue) return false
      } else if (comparable) {
        return false
      }
    }
    if (matcher.equals !== undefined) {
      if (comparable !== normalizeActionFormComparableValue(matcher.equals)) {
        return false
      }
    }
    if (Array.isArray(matcher.in) && matcher.in.length > 0) {
      const allowed = matcher.in.map((item) => normalizeActionFormComparableValue(item))
      if (!allowed.includes(comparable)) {
        return false
      }
    }
    if (matcher.not_equals !== undefined) {
      if (comparable === normalizeActionFormComparableValue(matcher.not_equals)) {
        return false
      }
    }
    if (Array.isArray(matcher.not_in) && matcher.not_in.length > 0) {
      const denied = matcher.not_in.map((item) => normalizeActionFormComparableValue(item))
      if (denied.includes(comparable)) {
        return false
      }
    }
    return true
  })
}

function isActionFormFieldVisible(
  field: PluginActionFormField,
  values: Record<string, string | boolean>
): boolean {
  return matchesActionFormConditions(field.visible_when, values)
}

function isActionFormFieldRequired(
  field: PluginActionFormField,
  values: Record<string, string | boolean>
): boolean {
  if (field.required === true) {
    return true
  }
  if (!field.required_when) {
    return false
  }
  return matchesActionFormConditions(field.required_when, values)
}

function isActionFormFieldValuePresent(
  field: PluginActionFormField,
  value: string | boolean | undefined
): boolean {
  if (normalizeActionFormFieldType(field.type) === 'boolean') {
    return value === true
  }
  return String(value ?? '').trim() !== ''
}

function normalizeFieldValueForState(
  field: PluginActionFormField,
  value: unknown
): string | boolean {
  const fieldType = normalizeActionFormFieldType(field.type)
  if (fieldType === 'boolean') {
    return boolOrDefault(value, false)
  }
  if (fieldType === 'json') {
    return typeof value === 'string' ? value : prettyJSON(value)
  }
  if (typeof value === 'string') {
    return value
  }
  if (value === null || value === undefined) {
    return ''
  }
  return String(value)
}

function extractFieldValueMap(payload: Record<string, any>): Record<string, any> {
  const candidates = [payload.values, payload.form, payload.fields, payload.config, payload.checker]
  for (const candidate of candidates) {
    if (candidate && typeof candidate === 'object' && !Array.isArray(candidate)) {
      return candidate as Record<string, any>
    }
  }
  return {}
}

function extractActionResultBlocks(payload: Record<string, any>): PluginPageBlock[] {
  if (Array.isArray(payload.blocks)) {
    return payload.blocks as PluginPageBlock[]
  }
  if (payload.view && typeof payload.view === 'object' && Array.isArray(payload.view.blocks)) {
    return payload.view.blocks as PluginPageBlock[]
  }
  return []
}

function buildActionFormState(
  fields: PluginActionFormField[],
  source: Record<string, unknown>
): Record<string, string | boolean> {
  const next: Record<string, string | boolean> = {}
  fields.forEach((field) => {
    const key = String(field.key || '').trim()
    if (!key) return
    next[key] = normalizeFieldValueForState(field, source[key])
  })
  if (Object.prototype.hasOwnProperty.call(next, 'workflow_stage')) {
    const derived = deriveActionFormWorkflowStage(next)
    if (
      String(next.workflow_stage ?? '').trim() === '' ||
      (next.workflow_stage === 'source' && derived !== 'source')
    ) {
      next.workflow_stage = derived
    }
  }
  return next
}

function deriveActionFormWorkflowStage(values: Record<string, string | boolean>): string {
  const kind = String(values.kind ?? '')
    .trim()
    .toLowerCase()
  const taskID = String(values.task_id ?? '').trim()
  const name = String(values.name ?? '').trim()
  const version = String(values.version ?? '').trim()
  if (kind === 'plugin_package' && taskID) {
    return 'task'
  }
  if (name && version) {
    return 'release'
  }
  if (name) {
    return 'artifact'
  }
  return 'source'
}

function hasActionFormConditions(conditions: unknown): boolean {
  return normalizeActionFormConditionList(conditions).length > 0
}

function resolveLegacyMarketActionVisible(
  action: string,
  values: Record<string, string | boolean>
): boolean {
  const normalizedAction = String(action || '')
    .trim()
    .toLowerCase()
  if (!normalizedAction) {
    return false
  }
  if (!normalizedAction.startsWith('market.')) {
    return true
  }

  const stage =
    String(values.workflow_stage ?? '')
      .trim()
      .toLowerCase() || deriveActionFormWorkflowStage(values)
  const kind = String(values.kind ?? '')
    .trim()
    .toLowerCase()
  const hasName = String(values.name ?? '').trim() !== ''
  const hasVersion = String(values.version ?? '').trim() !== ''
  const hasTaskID = String(values.task_id ?? '').trim() !== ''

  switch (normalizedAction) {
    case 'market.console.load':
    case 'market.package.load':
    case 'market.template.load':
    case 'market.catalog.query':
      return stage === 'source' || stage === 'catalog'
    case 'market.package.reset':
    case 'market.template.reset':
      return [
        'catalog',
        'artifact',
        'release',
        'preview',
        'install',
        'task',
        'history',
        'rollback',
      ].includes(stage)
    case 'market.source.detail':
      return (stage === 'source' || stage === 'catalog') && !hasTaskID && !hasName
    case 'market.artifact.detail':
      return (stage === 'artifact' || stage === 'catalog') && !hasTaskID && hasName && !hasVersion
    case 'market.release.detail':
      return (
        ['release', 'artifact', 'history', 'rollback'].includes(stage) &&
        !hasTaskID &&
        hasName &&
        hasVersion
      )
    case 'market.release.preview':
      return ['artifact', 'release', 'history'].includes(stage) && !hasTaskID && hasName
    case 'market.install.execute':
      return ['preview', 'release'].includes(stage) && !hasTaskID && hasName
    case 'market.install.task.get':
    case 'market.install.task.list':
      return ['task', 'install'].includes(stage) && kind === 'plugin_package' && hasTaskID
    case 'market.install.history.list':
      return (
        ['artifact', 'release', 'preview', 'install', 'task', 'history', 'rollback'].includes(
          stage
        ) && hasName
      )
    case 'market.install.rollback':
      return ['history', 'task', 'rollback'].includes(stage) && hasName && hasVersion
    default:
      return true
  }
}

function normalizeActionFormPresets(
  value: unknown,
  locale?: string
): ParsedPluginActionFormPreset[] {
  if (!Array.isArray(value)) {
    return []
  }
  return value
    .map((item, index) => {
      if (!item || typeof item !== 'object' || Array.isArray(item)) {
        return null
      }
      const candidate = item as PluginActionFormPreset
      const label = manifestString(candidate as Record<string, unknown>, 'label', locale)
      const values =
        candidate.values && typeof candidate.values === 'object' && !Array.isArray(candidate.values)
          ? (candidate.values as Record<string, unknown>)
          : {}
      if (!label || Object.keys(values).length === 0) {
        return null
      }
      return {
        key: String(candidate.key || `preset-${index}`),
        label,
        description: manifestString(candidate as Record<string, unknown>, 'description', locale),
        values,
      }
    })
    .filter((item): item is ParsedPluginActionFormPreset => !!item)
}

function normalizeActionFormRecentLimit(value: unknown): number {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return Math.max(1, Math.min(12, Math.trunc(value)))
  }
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Number(value)
    if (Number.isFinite(parsed)) {
      return Math.max(1, Math.min(12, Math.trunc(parsed)))
    }
  }
  return 6
}

function normalizeActionFormPrimaryActionLimit(value: unknown): number {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return Math.max(1, Math.min(6, Math.trunc(value)))
  }
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Number(value)
    if (Number.isFinite(parsed)) {
      return Math.max(1, Math.min(6, Math.trunc(parsed)))
    }
  }
  return 3
}

function resolveActionButtonPriority(
  actionName: string,
  variant: 'default' | 'outline' | 'secondary' | 'destructive'
): number {
  const normalized = String(actionName || '')
    .trim()
    .toLowerCase()
  if (!normalized) {
    return 0
  }
  let score =
    variant === 'default' ? 60 : variant === 'secondary' ? 48 : variant === 'outline' ? 40 : 12

  if (
    normalized.includes('install.execute') ||
    normalized.includes('.install') ||
    normalized.includes('.import')
  ) {
    score += 72
  } else if (normalized.includes('release.preview') || normalized.includes('.preview')) {
    score += 52
  } else if (
    normalized.includes('catalog.query') ||
    normalized.includes('.query') ||
    normalized.includes('.search')
  ) {
    score += 46
  } else if (normalized.includes('.load')) {
    score += 34
  } else if (normalized.includes('task.get') || normalized.includes('task.inspect')) {
    score += 34
  } else if (normalized.includes('history')) {
    score += 20
  } else if (normalized.includes('detail') || normalized.endsWith('.get')) {
    score += 12
  }

  if (normalized.includes('.list')) {
    score += 8
  }
  if (normalized.includes('rollback')) {
    score -= 30
  }
  if (normalized.endsWith('.reset') || normalized.includes('.reset.')) {
    score -= 15
  }
  return score
}

function normalizeActionFormFieldKeys(value: unknown): string[] {
  if (!Array.isArray(value)) {
    return []
  }
  const seen = new Set<string>()
  const keys: string[] = []
  value.forEach((item) => {
    const normalized = String(item || '').trim()
    if (!normalized || seen.has(normalized)) {
      return
    }
    seen.add(normalized)
    keys.push(normalized)
  })
  return keys
}

function buildActionFormRecentStorageKey(input: {
  pluginID: number
  pageFullPath: string
  title: string
  recentKey: string
  actions: string[]
}): string {
  const pageKey = input.pageFullPath.trim() || '/'
  const actionKey = input.actions.filter(Boolean).join('|')
  const suffix = input.recentKey.trim() || `${input.title.trim()}|${actionKey}`
  return `plugin-action-form-recents:${input.pluginID}:${pageKey}:${suffix}`
}

function readActionFormRecentEntries(storageKey: string): PluginActionFormRecentEntry[] {
  if (typeof window === 'undefined' || !storageKey) {
    return []
  }
  try {
    const raw = window.localStorage.getItem(storageKey)
    if (!raw) {
      return []
    }
    const parsed = JSON.parse(raw)
    if (!Array.isArray(parsed)) {
      return []
    }
    return parsed
      .map((item) => {
        if (!item || typeof item !== 'object' || Array.isArray(item)) {
          return null
        }
        const candidate = item as PluginActionFormRecentEntry
        const values =
          candidate.values &&
          typeof candidate.values === 'object' &&
          !Array.isArray(candidate.values)
            ? Object.entries(candidate.values).reduce<Record<string, string | boolean>>(
                (acc, [key, value]) => {
                  if (typeof value === 'boolean') {
                    acc[key] = value
                    return acc
                  }
                  acc[key] = String(value ?? '')
                  return acc
                },
                {}
              )
            : {}
        const label = manifestString(candidate as Record<string, unknown>, 'label')
        if (!label || Object.keys(values).length === 0) {
          return null
        }
        return {
          key: String(candidate.key || label),
          label,
          summary: manifestString(candidate as Record<string, unknown>, 'summary'),
          updated_at: String(candidate.updated_at || '').trim(),
          values,
        }
      })
      .filter((item): item is PluginActionFormRecentEntry => !!item)
  } catch {
    return []
  }
}

function writeActionFormRecentEntries(storageKey: string, entries: PluginActionFormRecentEntry[]) {
  if (typeof window === 'undefined' || !storageKey) {
    return
  }
  try {
    if (entries.length === 0) {
      window.localStorage.removeItem(storageKey)
      return
    }
    window.localStorage.setItem(storageKey, JSON.stringify(entries))
  } catch {}
}

function buildActionFormSnapshot(
  fields: PluginActionFormField[],
  values: Record<string, string | boolean>
): Record<string, string | boolean> {
  const snapshot: Record<string, string | boolean> = {}
  fields.forEach((field) => {
    const key = String(field.key || '').trim()
    if (!key) return
    snapshot[key] = normalizeFieldValueForState(field, values[key])
  })
  return snapshot
}

function buildActionFormSnapshotSignature(
  fields: PluginActionFormField[],
  values: Record<string, string | boolean>
): string {
  return JSON.stringify(
    fields.map((field) => {
      const key = String(field.key || '').trim()
      return [key, key ? values[key] : '']
    })
  )
}

function buildActionFormSnapshotDisplay(input: {
  fields: PluginActionFormField[]
  values: Record<string, string | boolean>
  labelFields: string[]
  fallbackLabel: string
  locale?: string
}): { label: string; summary: string } {
  const fieldLabelMap = new Map<string, string>()
  input.fields.forEach((field) => {
    const key = String(field.key || '').trim()
    if (!key) return
    fieldLabelMap.set(
      key,
      manifestString(field as Record<string, unknown>, 'label', input.locale) || key
    )
  })
  const orderedKeys =
    input.labelFields.length > 0 ? input.labelFields : Array.from(fieldLabelMap.keys())
  const descriptorKeys = Array.from(new Set([...orderedKeys, ...Array.from(fieldLabelMap.keys())]))
  const descriptors = descriptorKeys
    .map((key) => {
      const value = input.values[key]
      if (typeof value === 'boolean') {
        if (!value) {
          return null
        }
        return {
          key,
          label: fieldLabelMap.get(key) || key,
          value: 'true',
        }
      }
      const text = String(value ?? '').trim()
      if (!text) {
        return null
      }
      return {
        key,
        label: fieldLabelMap.get(key) || key,
        value: truncateDisplayValue(text, 80),
      }
    })
    .filter((item): item is { key: string; label: string; value: string } => !!item)

  const preferredDescriptors = descriptors.filter((item) => input.labelFields.includes(item.key))
  const labelSource = (preferredDescriptors.length > 0 ? preferredDescriptors : descriptors).slice(
    0,
    3
  )
  const label = labelSource.map((item) => item.value).join(' / ') || input.fallbackLabel
  const summary = descriptors
    .slice(0, 3)
    .map((item) => `${item.label}: ${item.value}`)
    .join(' · ')
  return { label, summary }
}

function parseStatsItems(
  data: Record<string, any> | undefined,
  locale?: string
): PluginStatsItem[] {
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

function parseKeyValueItems(
  data: Record<string, any> | undefined,
  locale?: string
): PluginKeyValueItem[] {
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

function parseBadgeItems(data: Record<string, any> | undefined, locale?: string): string[] {
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

function executeTrustedPluginInlineScripts(root: HTMLElement) {
  const scripts = Array.from(root.querySelectorAll('script')).filter(
    (script) => !(script as HTMLScriptElement).hasAttribute('data-plugin-script-executed')
  )
  scripts.forEach((script) => {
    const current = script as HTMLScriptElement
    const source = String(current.getAttribute('src') || '').trim()
    const replacement = document.createElement('script')
    Array.from(current.attributes).forEach((attribute) => {
      replacement.setAttribute(attribute.name, attribute.value)
    })
    replacement.setAttribute('data-plugin-script-executed', 'true')
    if (!replacement.getAttribute('type')) {
      replacement.setAttribute('type', 'text/javascript')
    }
    if (source) {
      replacement.src = source
    }
    replacement.text = current.textContent || ''
    const parent = current.parentNode
    if (!parent) {
      return
    }
    parent.insertBefore(replacement, current.nextSibling)
    parent.removeChild(current)
  })

  const marketTrustedWindow = window as typeof window & {
    __AuraLogicMarketTrustedInitAll?: () => void
  }
  if (typeof marketTrustedWindow.__AuraLogicMarketTrustedInitAll === 'function') {
    marketTrustedWindow.__AuraLogicMarketTrustedInitAll()
  }
}

function cleanupTrustedPluginDetachedLayers() {
  if (typeof document === 'undefined') {
    return
  }
  document
    .querySelectorAll<HTMLElement>('[data-market-row-menu][data-market-owner]')
    .forEach((node) => {
      node.remove()
    })
}

function ensureTrustedPluginMarkup(root: HTMLElement, html: string) {
  if (!root || typeof window === 'undefined') {
    return
  }

  const marketRoots = Array.from(root.querySelectorAll<HTMLElement>('[data-market-root]'))
  const pendingScripts = Array.from(root.querySelectorAll<HTMLScriptElement>('script')).filter(
    (script) => !script.hasAttribute('data-plugin-script-executed')
  )
  const marketTrustedWindow = window as typeof window & {
    __AuraLogicMarketTrustedInitRoot?: (root: HTMLElement) => void
  }

  if (pendingScripts.length > 0) {
    if (root.getAttribute('data-plugin-trusted-bootstrap') === 'running') {
      return
    }
    root.setAttribute('data-plugin-trusted-bootstrap', 'running')
    executeTrustedPluginInlineScripts(root)
    window.requestAnimationFrame(() => {
      root.removeAttribute('data-plugin-trusted-bootstrap')
    })
    return
  }

  if (
    marketRoots.length > 0 &&
    typeof marketTrustedWindow.__AuraLogicMarketTrustedInitRoot === 'function'
  ) {
    marketRoots.forEach((marketRoot) => {
      const reboundRoot = marketRoot as HTMLElement & {
        __AuraLogicMarketTrustedBound?: boolean
      }
      const isBound = reboundRoot.__AuraLogicMarketTrustedBound === true
      const isReady = reboundRoot.getAttribute('data-market-ready') === 'true'
      if (isBound && isReady) {
        return
      }
      reboundRoot.__AuraLogicMarketTrustedBound = false
      reboundRoot.removeAttribute('data-market-ready')
      marketTrustedWindow.__AuraLogicMarketTrustedInitRoot?.(reboundRoot)
    })
    return
  }

  const markup = typeof html === 'string' ? html : ''
  if (!markup.trim()) {
    return
  }
  root.innerHTML = markup
  executeTrustedPluginInlineScripts(root)
}

function parseTableConfig(data: Record<string, any> | undefined): {
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
          label:
            typeof column.label === 'string' && column.label.trim() !== '' ? column.label : key,
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
  const emptyText =
    typeof data?.empty_text === 'string' && data.empty_text.trim() !== '' ? data.empty_text : ''
  return { columns, rows, emptyText }
}

type PluginActionFormBlockProps = {
  block: PluginPageBlock
  area: 'admin' | 'user'
  pluginID: number
  pagePath: string
  pageFullPath: string
  queryParams: Record<string, string>
  routeParams: Record<string, string>
  executeAPI?: PluginFrontendRouteExecuteAPI
  t: ReturnType<typeof getTranslations>
}

function PluginActionFormBlock({
  block,
  area,
  pluginID,
  pagePath,
  pageFullPath,
  queryParams,
  routeParams,
  executeAPI,
  t,
}: PluginActionFormBlockProps) {
  const { locale } = useLocale()
  const data = useMemo(
    () => (block.data && typeof block.data === 'object' ? block.data : {}),
    [block]
  )
  const fields = useMemo(
    () => (Array.isArray(data.fields) ? (data.fields as PluginActionFormField[]) : []),
    [data]
  )
  const actions = useMemo(
    () =>
      data.actions && typeof data.actions === 'object'
        ? (data.actions as PluginActionFormActions)
        : {},
    [data]
  )
  const presets = useMemo(
    () => normalizeActionFormPresets(data.presets, locale),
    [data.presets, locale]
  )
  const rememberRecent = boolOrDefault(data.remember_recent, false)
  const recentKey = typeof data.recent_key === 'string' ? data.recent_key.trim() : ''
  const recentTitle = manifestString(data, 'recent_title', locale) || t.admin.pluginRecentContexts
  const recentLimit = useMemo(
    () => normalizeActionFormRecentLimit(data.recent_limit),
    [data.recent_limit]
  )
  const recentLabelFields = useMemo(
    () => normalizeActionFormFieldKeys(data.recent_label_fields),
    [data.recent_label_fields]
  )
  const loadAction = typeof actions.load === 'string' ? actions.load.trim() : ''
  const saveAction = typeof actions.save === 'string' ? actions.save.trim() : ''
  const resetAction = typeof actions.reset === 'string' ? actions.reset.trim() : ''
  const loadLabel =
    manifestString(actions as Record<string, unknown>, 'load_label', locale) || t.common.refresh
  const saveLabel =
    manifestString(actions as Record<string, unknown>, 'save_label', locale) || t.common.save
  const resetLabel =
    manifestString(actions as Record<string, unknown>, 'reset_label', locale) || t.common.reset
  const title = manifestString(block as Record<string, unknown>, 'title', locale)
  const initialRaw = useMemo(
    () =>
      data.initial && typeof data.initial === 'object'
        ? (data.initial as Record<string, unknown>)
        : {},
    [data]
  )
  const extraActions = useMemo(() => {
    const raw = Array.isArray(actions.extra)
      ? actions.extra
      : Array.isArray(actions.buttons)
        ? actions.buttons
        : []
    return raw
      .map((item, index) => {
        if (!item || typeof item !== 'object' || Array.isArray(item)) return null
        const candidate = item as PluginActionFormExtraAction
        const action = String(candidate.action || '').trim()
        if (!action) return null
        return {
          key: String(candidate.key || `extra-${index}`),
          label: manifestString(candidate as Record<string, unknown>, 'label', locale) || action,
          action,
          variant: candidate.variant || 'outline',
          includeFields: candidate.include_fields !== false,
          requiredFields: normalizeActionFormFieldKeys(candidate.required_fields),
          visibleWhen: candidate.visible_when,
        }
      })
      .filter((item): item is ParsedPluginActionFormAction => !!item)
  }, [actions.buttons, actions.extra, locale])
  const loadRequiredFields = useMemo(
    () => normalizeActionFormFieldKeys(actions.load_required_fields),
    [actions.load_required_fields]
  )
  const saveRequiredFields = useMemo(
    () => normalizeActionFormFieldKeys(actions.save_required_fields),
    [actions.save_required_fields]
  )
  const resetRequiredFields = useMemo(
    () => normalizeActionFormFieldKeys(actions.reset_required_fields),
    [actions.reset_required_fields]
  )
  const autoload = data.autoload !== false && !!loadAction
  const autoloadIncludeFields = data.autoload_include_fields !== false

  const [values, setValues] = useState<Record<string, string | boolean>>({})
  const [source, setSource] = useState('')
  const [errorText, setErrorText] = useState('')
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [noticeText, setNoticeText] = useState('')
  const [resultBlocks, setResultBlocks] = useState<PluginPageBlock[]>([])
  const [lastActionLabel, setLastActionLabel] = useState('')
  const [lastActionStatus, setLastActionStatus] = useState<
    'idle' | 'running' | 'success' | 'error'
  >('idle')
  const [recentEntries, setRecentEntries] = useState<PluginActionFormRecentEntry[]>([])
  const [showOverflowActions, setShowOverflowActions] = useState(false)
  const resultRef = useRef<HTMLDivElement | null>(null)
  const shouldScrollToResultRef = useRef(false)
  const enableLegacyMarketVisibilityFallback = useMemo(() => {
    if (pagePath !== '/admin/plugin-pages/market') {
      return false
    }
    const declaredActions = [
      loadAction,
      saveAction,
      resetAction,
      ...extraActions.map((action) => action.action),
    ]
    return declaredActions.some((action) =>
      String(action || '')
        .trim()
        .toLowerCase()
        .startsWith('market.')
    )
  }, [extraActions, loadAction, pagePath, resetAction, saveAction])
  const isActionVisible = useCallback(
    (actionName: string, conditions: unknown): boolean => {
      if (!actionName) {
        return false
      }
      if (!matchesActionFormConditions(conditions, values)) {
        return false
      }
      if (!enableLegacyMarketVisibilityFallback || hasActionFormConditions(conditions)) {
        return true
      }
      return resolveLegacyMarketActionVisible(actionName, values)
    },
    [enableLegacyMarketVisibilityFallback, values]
  )
  const loadVisible = useMemo(
    () => isActionVisible(loadAction, actions.load_visible_when),
    [actions.load_visible_when, isActionVisible, loadAction]
  )
  const saveVisible = useMemo(
    () => isActionVisible(saveAction, actions.save_visible_when),
    [actions.save_visible_when, isActionVisible, saveAction]
  )
  const resetVisible = useMemo(
    () => isActionVisible(resetAction, actions.reset_visible_when),
    [actions.reset_visible_when, isActionVisible, resetAction]
  )

  const actionLabelMap = useMemo(() => {
    const next = new Map<string, string>()
    if (loadAction) next.set(loadAction, loadLabel)
    if (saveAction) next.set(saveAction, saveLabel)
    if (resetAction) next.set(resetAction, resetLabel)
    extraActions.forEach((action) => next.set(action.action, action.label))
    return next
  }, [extraActions, loadAction, loadLabel, resetAction, resetLabel, saveAction, saveLabel])
  const visibleExtraActions = useMemo(
    () => extraActions.filter((action) => isActionVisible(action.action, action.visibleWhen)),
    [extraActions, isActionVisible]
  )
  const availableActionCount = useMemo(
    () =>
      [
        loadVisible ? loadAction : '',
        saveVisible ? saveAction : '',
        resetVisible ? resetAction : '',
        ...visibleExtraActions.map((action) => action.action),
      ].filter((action) => String(action || '').trim() !== '').length,
    [
      loadAction,
      loadVisible,
      resetAction,
      resetVisible,
      saveAction,
      saveVisible,
      visibleExtraActions,
    ]
  )
  const compactActionLayout = useMemo(() => {
    if (typeof actions.collapse_extra === 'boolean') {
      return actions.collapse_extra
    }
    const layoutHint = String((data as Record<string, unknown>).action_layout || '')
      .trim()
      .toLowerCase()
    if (layoutHint === 'compact' || layoutHint === 'focus') {
      return true
    }
    if (layoutHint === 'expanded' || layoutHint === 'full') {
      return false
    }
    return enableLegacyMarketVisibilityFallback
  }, [actions.collapse_extra, data, enableLegacyMarketVisibilityFallback])
  const primaryActionLimit = useMemo(
    () =>
      normalizeActionFormPrimaryActionLimit(
        actions.max_primary_actions ?? (data as Record<string, unknown>).max_primary_actions
      ),
    [actions.max_primary_actions, data]
  )
  const visibleActionButtons = useMemo<RenderablePluginActionButton[]>(() => {
    const items: RenderablePluginActionButton[] = []
    let order = 0
    if (loadAction && loadVisible) {
      items.push({
        key: 'load',
        label: loadLabel,
        action: loadAction,
        variant: 'outline',
        includeFields: true,
        requiredFields: loadRequiredFields,
        order,
        priority: resolveActionButtonPriority(loadAction, 'outline'),
      })
      order += 1
    }
    if (resetAction && resetVisible) {
      items.push({
        key: 'reset',
        label: resetLabel,
        action: resetAction,
        variant: 'outline',
        includeFields: true,
        requiredFields: resetRequiredFields,
        order,
        priority: resolveActionButtonPriority(resetAction, 'outline'),
      })
      order += 1
    }
    if (saveAction && saveVisible) {
      items.push({
        key: 'save',
        label: saveLabel,
        action: saveAction,
        variant: 'default',
        includeFields: true,
        requiredFields: saveRequiredFields,
        order,
        priority: resolveActionButtonPriority(saveAction, 'default'),
      })
      order += 1
    }
    visibleExtraActions.forEach((action) => {
      items.push({
        key: action.key,
        label: action.label,
        action: action.action,
        variant: action.variant,
        includeFields: action.includeFields,
        requiredFields: action.requiredFields,
        order,
        priority: resolveActionButtonPriority(action.action, action.variant),
      })
      order += 1
    })
    if (!compactActionLayout) {
      return items.sort((left, right) => left.order - right.order)
    }
    return items.sort((left, right) => {
      if (left.priority !== right.priority) {
        return right.priority - left.priority
      }
      return left.order - right.order
    })
  }, [
    compactActionLayout,
    loadAction,
    loadLabel,
    loadRequiredFields,
    loadVisible,
    resetAction,
    resetLabel,
    resetRequiredFields,
    resetVisible,
    saveAction,
    saveLabel,
    saveRequiredFields,
    saveVisible,
    visibleExtraActions,
  ])
  const primaryActionButtons = useMemo(
    () =>
      compactActionLayout
        ? visibleActionButtons.slice(0, primaryActionLimit)
        : visibleActionButtons,
    [compactActionLayout, primaryActionLimit, visibleActionButtons]
  )
  const overflowActionButtons = useMemo(
    () => (compactActionLayout ? visibleActionButtons.slice(primaryActionLimit) : []),
    [compactActionLayout, primaryActionLimit, visibleActionButtons]
  )
  const trustedFallbackFlag =
    boolOrDefault(data.fallback_when_untrusted, false) ||
    boolOrDefault(data.fallback_when_no_trusted, false) ||
    boolOrDefault(data.hide_after_trusted_boot, false)

  const resolveActionDisplayLabel = (action: string): string =>
    actionLabelMap.get(action) || action || t.common.refresh

  const setFieldValue = (key: string, value: string | boolean) => {
    setValues((prev) => {
      const next = { ...prev, [key]: value }
      if (
        Object.prototype.hasOwnProperty.call(next, 'workflow_stage') &&
        key !== 'workflow_stage' &&
        ['kind', 'name', 'version', 'task_id'].includes(key)
      ) {
        next.workflow_stage = deriveActionFormWorkflowStage(next)
      }
      return next
    })
    setFieldErrors((prev) => {
      if (!(key in prev)) {
        return prev
      }
      const next = { ...prev }
      delete next[key]
      return next
    })
  }

  const recentStorageKey = useMemo(
    () =>
      rememberRecent
        ? buildActionFormRecentStorageKey({
            pluginID,
            pageFullPath,
            title,
            recentKey,
            actions: [
              loadAction,
              saveAction,
              resetAction,
              ...extraActions.map((item) => item.action),
            ],
          })
        : '',
    [
      extraActions,
      loadAction,
      pageFullPath,
      pluginID,
      recentKey,
      rememberRecent,
      resetAction,
      saveAction,
      title,
    ]
  )

  const applyFormSnapshot = (snapshot: Record<string, unknown>) => {
    setErrorText('')
    setFieldErrors({})
    setNoticeText('')
    setResultBlocks([])
    setSource('')
    setLastActionLabel('')
    setLastActionStatus('idle')
    setValues(buildActionFormState(fields, { ...initialRaw, ...queryParams, ...snapshot }))
  }

  const persistRecentSnapshot = (
    snapshot: Record<string, string | boolean>,
    fallbackLabel: string
  ) => {
    if (!rememberRecent || !recentStorageKey || fields.length === 0) {
      return
    }
    const display = buildActionFormSnapshotDisplay({
      fields,
      values: snapshot,
      labelFields: recentLabelFields,
      fallbackLabel: fallbackLabel || t.admin.pluginRecentContextFallback,
      locale,
    })
    const entry: PluginActionFormRecentEntry = {
      key: buildActionFormSnapshotSignature(fields, snapshot),
      label: display.label || t.admin.pluginRecentContextFallback,
      summary: display.summary,
      updated_at: new Date().toISOString(),
      values: snapshot,
    }
    setRecentEntries((prev) => {
      const next = [entry, ...prev.filter((item) => item.key !== entry.key)].slice(0, recentLimit)
      writeActionFormRecentEntries(recentStorageKey, next)
      return next
    })
  }

  const clearRecentSnapshots = () => {
    if (!recentStorageKey) {
      return
    }
    writeActionFormRecentEntries(recentStorageKey, [])
    setRecentEntries([])
  }

  const visibleFields = useMemo(
    () => fields.filter((field) => isActionFormFieldVisible(field, values)),
    [fields, values]
  )

  const dispatchAction = (
    input: { action: string; includeFields: boolean; requiredFields: string[] },
    trigger: 'manual' | 'autoload' = 'manual'
  ) => {
    shouldScrollToResultRef.current = trigger === 'manual'
    setLastActionLabel(resolveActionDisplayLabel(input.action))
    if (input.includeFields) {
      const requiredKeys = new Set<string>(input.requiredFields)
      visibleFields.forEach((field) => {
        const key = String(field.key || '').trim()
        if (!key) return
        if (isActionFormFieldRequired(field, values)) {
          requiredKeys.add(key)
        }
      })
      if (requiredKeys.size > 0) {
        const nextErrors: Record<string, string> = {}
        visibleFields.forEach((field) => {
          const key = String(field.key || '').trim()
          if (!key || !requiredKeys.has(key)) {
            return
          }
          if (!isActionFormFieldValuePresent(field, values[key])) {
            nextErrors[key] = t.admin.pluginActionFieldRequired
          }
        })
        if (Object.keys(nextErrors).length > 0) {
          setFieldErrors(nextErrors)
          setLastActionStatus('error')
          setNoticeText('')
          setResultBlocks([])
          setErrorText(t.admin.pluginActionValidationFailed)
          return
        }
      }
    }
    setFieldErrors({})
    setLastActionStatus('running')
    runAction({
      ...input,
      trigger,
      paramsSnapshot: input.includeFields ? buildActionFormSnapshot(fields, values) : {},
    })
  }

  useEffect(() => {
    setValues(buildActionFormState(fields, { ...initialRaw, ...queryParams }))
  }, [fields, initialRaw, queryParams])

  useEffect(() => {
    setFieldErrors((prev) => {
      const visibleKeys = new Set(
        visibleFields.map((field) => String(field.key || '').trim()).filter((key) => key !== '')
      )
      const next = Object.entries(prev).reduce<Record<string, string>>((acc, [key, value]) => {
        if (visibleKeys.has(key)) {
          acc[key] = value
        }
        return acc
      }, {})
      if (Object.keys(next).length === Object.keys(prev).length) {
        return prev
      }
      return next
    })
  }, [visibleFields])

  useEffect(() => {
    if (!recentStorageKey) {
      setRecentEntries([])
      return
    }
    setRecentEntries(readActionFormRecentEntries(recentStorageKey))
  }, [recentStorageKey])

  const applyPayloadToStateFromPayload = (payload: Record<string, any>) => {
    const nextValues = extractFieldValueMap(payload)
    setSource(typeof payload.source === 'string' ? payload.source : '')
    setFieldErrors({})
    setNoticeText(
      manifestString(payload, 'message', locale) || manifestString(payload, 'notice', locale)
    )
    setResultBlocks(extractActionResultBlocks(payload))

    setValues((prev) => {
      const next = { ...prev }
      fields.forEach((field) => {
        const key = String(field.key || '').trim()
        if (!key || !(key in nextValues)) return
        next[key] = normalizeFieldValueForState(field, nextValues[key])
      })
      return next
    })
    return true
  }

  const applyPayloadToState = (resp: any) => {
    const payload = parseExecutePayload(resp)
    if (!looksLikeExecutePayload(payload)) {
      return false
    }
    return applyPayloadToStateFromPayload(payload)
  }

  const applyStreamChunkToState = (chunk: PluginRouteStreamChunk) => {
    const payload = chunk?.data
    if (!payload || typeof payload !== 'object' || Array.isArray(payload)) {
      return
    }
    if (looksLikeExecutePayload(payload)) {
      applyPayloadToStateFromPayload(payload)
      return
    }
    const payloadRecord = payload as Record<string, any>

    const statusText = typeof payloadRecord.status === 'string' ? payloadRecord.status.trim() : ''
    const progressValue = payloadRecord.progress
    if (statusText) {
      if (typeof progressValue === 'number' && Number.isFinite(progressValue)) {
        setNoticeText(`${statusText}: ${progressValue}%`)
        return
      }
      setNoticeText(statusText)
    }
  }

  const buildParams = (): Record<string, string> => {
    const params: Record<string, string> = {}
    visibleFields.forEach((field) => {
      const key = String(field.key || '').trim()
      if (!key) return
      const fieldType = normalizeActionFormFieldType(field.type)
      const value = values[key]
      params[key] = fieldType === 'boolean' ? String(!!value) : String(value ?? '')
    })
    return params
  }

  const actionMutation = useMutation({
    mutationFn: ({
      action,
      includeFields,
    }: {
      action: string
      includeFields: boolean
      requiredFields?: string[]
      trigger?: 'manual' | 'autoload'
      paramsSnapshot?: Record<string, string | boolean>
    }) => {
      const request = {
        action,
        params: includeFields ? buildParams() : {},
        path: pagePath,
        query_params: queryParams,
        route_params: routeParams,
      }
      if (pluginRouteShouldStream(executeAPI || {}, action)) {
        return executePluginRouteActionStream(executeAPI || {}, request, {
          locale,
          onChunk: (chunk) => {
            applyStreamChunkToState(chunk)
          },
        })
      }
      return executePluginRouteAction(executeAPI || {}, request, { locale })
    },
    onMutate: (variables) => {
      setErrorText('')
      setFieldErrors({})
      setNoticeText('')
      setResultBlocks([])
      setSource('')
      setLastActionLabel(resolveActionDisplayLabel(variables.action))
      setLastActionStatus('running')
    },
    onSuccess: (resp, variables) => {
      if (isExecuteFailed(resp)) {
        applyPayloadToState(resp)
        const fallback =
          variables.action === loadAction
            ? t.admin.pluginBizErrorActionLoad
            : variables.action === saveAction
              ? t.admin.pluginBizErrorActionSave
              : variables.action === resetAction
                ? t.admin.pluginBizErrorActionReset
                : t.admin.operationFailed
        setLastActionStatus('error')
        setErrorText(resolvePluginErrorText(resp, t, fallback))
        return
      }
      setErrorText('')
      setFieldErrors({})
      applyPayloadToState(resp)
      setLastActionStatus('success')
      if (variables.trigger === 'manual' && variables.includeFields) {
        persistRecentSnapshot(
          variables.paramsSnapshot || buildActionFormSnapshot(fields, values),
          resolveActionDisplayLabel(variables.action)
        )
      }
    },
    onError: (error: any, variables) => {
      const fallback =
        variables.action === loadAction
          ? t.admin.pluginBizErrorActionLoad
          : variables.action === saveAction
            ? t.admin.pluginBizErrorActionSave
            : variables.action === resetAction
              ? t.admin.pluginBizErrorActionReset
              : t.admin.operationFailed
      setLastActionStatus('error')
      setErrorText(resolvePluginErrorText(error, t, fallback))
    },
  })
  const runAction = actionMutation.mutate
  const pending = actionMutation.isPending

  useEffect(() => {
    if (!showOverflowActions) {
      return
    }
    if (overflowActionButtons.length === 0) {
      setShowOverflowActions(false)
    }
  }, [overflowActionButtons.length, showOverflowActions])

  useEffect(() => {
    if (pending || !shouldScrollToResultRef.current) {
      return
    }
    shouldScrollToResultRef.current = false
    if (typeof window === 'undefined') {
      return
    }
    window.requestAnimationFrame(() => {
      resultRef.current?.scrollIntoView({
        behavior: 'smooth',
        block: 'start',
      })
    })
  }, [pending, errorText, noticeText, resultBlocks.length])

  useEffect(() => {
    if (pluginID <= 0 || !executeAPI || !autoload) return
    shouldScrollToResultRef.current = false
    setLastActionLabel(actionLabelMap.get(loadAction) || loadAction || t.common.refresh)
    setLastActionStatus('running')
    runAction({
      action: loadAction,
      includeFields: autoloadIncludeFields,
      trigger: 'autoload',
      paramsSnapshot: {},
    })
  }, [
    actionLabelMap,
    autoload,
    autoloadIncludeFields,
    executeAPI,
    loadAction,
    pageFullPath,
    pluginID,
    runAction,
    t.common.refresh,
  ])

  if (pluginID <= 0) {
    return (
      <Card>
        <CardContent className="p-4 text-sm text-muted-foreground">
          {t.admin.pluginActionMissingPluginId}
        </CardContent>
      </Card>
    )
  }
  if (!executeAPI?.url) {
    return (
      <Card>
        <CardContent className="p-4 text-sm text-muted-foreground">
          {t.admin.pluginActionExecuteUnavailable}
        </CardContent>
      </Card>
    )
  }
  const hasResultState =
    !!lastActionLabel || pending || !!errorText || !!noticeText || resultBlocks.length > 0
  const resultToneClassName =
    lastActionStatus === 'error'
      ? 'border-destructive/40 bg-destructive/5'
      : lastActionStatus === 'success'
        ? 'border-emerald-500/30 bg-emerald-500/5 dark:border-emerald-500/40 dark:bg-emerald-950/20'
        : lastActionStatus === 'running'
          ? 'border-amber-500/30 bg-amber-500/5 dark:border-amber-500/40 dark:bg-amber-950/20'
          : 'border-input/50 bg-muted/10'

  return (
    <Card data-plugin-action-form-fallback={trustedFallbackFlag ? 'true' : undefined}>
      <CardHeader className="pb-2">
        <div className="space-y-1">
          {title && <CardTitle className="text-base">{title}</CardTitle>}
          {source ? (
            <CardDescription>{`${t.admin.pluginSourceLabel}: ${source}`}</CardDescription>
          ) : null}
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        {presets.length > 0 ? (
          <div className="space-y-2 rounded-lg border border-input/60 bg-muted/5 p-3">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <p className="text-sm font-medium">{t.admin.pluginQuickPresets}</p>
              <Badge variant="outline">{presets.length}</Badge>
            </div>
            <div className="grid gap-2 md:grid-cols-2 xl:grid-cols-3">
              {presets.map((preset) => (
                <button
                  key={preset.key}
                  type="button"
                  className="rounded-lg border border-input/60 bg-muted/20 p-3 text-left transition hover:border-primary/40 hover:bg-muted/35 disabled:cursor-not-allowed disabled:opacity-60"
                  onClick={() => applyFormSnapshot(preset.values)}
                  disabled={pending}
                >
                  <div className="space-y-1">
                    <p className="text-sm font-medium">{preset.label}</p>
                    {preset.description ? (
                      <p className="text-xs text-muted-foreground">{preset.description}</p>
                    ) : null}
                  </div>
                </button>
              ))}
            </div>
          </div>
        ) : null}
        {rememberRecent ? (
          <div className="space-y-2 rounded-lg border border-input/60 bg-muted/5 p-3">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <p className="text-sm font-medium">{recentTitle}</p>
              <div className="flex flex-wrap items-center gap-2">
                <Badge variant={recentEntries.length > 0 ? 'secondary' : 'outline'}>
                  {t.admin.pluginActionRecentCount.replace('{count}', String(recentEntries.length))}
                </Badge>
                {recentEntries.length > 0 ? (
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={clearRecentSnapshots}
                    disabled={pending}
                  >
                    {t.admin.pluginRecentContextsClear}
                  </Button>
                ) : null}
              </div>
            </div>
            {recentEntries.length === 0 ? (
              <div className="rounded-lg border border-dashed border-input/60 bg-muted/10 p-3 text-xs text-muted-foreground">
                {t.admin.pluginRecentContextsEmpty}
              </div>
            ) : (
              <div className="grid gap-2 md:grid-cols-2 xl:grid-cols-3">
                {recentEntries.map((entry) => {
                  const updatedAtText = entry.updated_at
                    ? new Date(entry.updated_at).toLocaleString()
                    : ''
                  return (
                    <button
                      key={entry.key}
                      type="button"
                      className="rounded-lg border border-input/60 bg-background p-3 text-left transition hover:border-primary/40 hover:bg-muted/20 disabled:cursor-not-allowed disabled:opacity-60"
                      onClick={() => applyFormSnapshot(entry.values)}
                      disabled={pending}
                    >
                      <div className="space-y-1">
                        <div className="flex flex-wrap items-start justify-between gap-2">
                          <p className="text-sm font-medium">{entry.label}</p>
                          <Badge variant="outline">{t.admin.pluginRecentContextsApply}</Badge>
                        </div>
                        {entry.summary ? (
                          <p className="text-xs text-muted-foreground">{entry.summary}</p>
                        ) : null}
                        {updatedAtText ? (
                          <p className="text-[11px] text-muted-foreground">{updatedAtText}</p>
                        ) : null}
                      </div>
                    </button>
                  )
                })}
              </div>
            )}
          </div>
        ) : null}
        <div className="space-y-3 rounded-lg border border-input/60 bg-muted/5 p-3">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <p className="text-sm font-medium">{t.admin.pluginActionFields}</p>
            <Badge variant="outline">
              {t.admin.pluginActionVisibleFieldCount.replace(
                '{count}',
                String(visibleFields.length)
              )}
            </Badge>
          </div>
          {visibleFields.length === 0 ? (
            <div className="rounded-lg border border-dashed border-input/60 bg-background/80 p-3 text-sm text-muted-foreground">
              {t.common.noData}
            </div>
          ) : (
            <div className="grid gap-3 md:grid-cols-2">
              {visibleFields.map((field, idx) => {
                const key = String(field.key || '').trim()
                if (!key) return null
                const fieldType = normalizeActionFormFieldType(field.type)
                const label =
                  manifestString(field as Record<string, unknown>, 'label', locale) || key
                const description = manifestString(
                  field as Record<string, unknown>,
                  'description',
                  locale
                )
                const placeholder = manifestString(
                  field as Record<string, unknown>,
                  'placeholder',
                  locale
                )
                const currentValue = values[key]
                const options = normalizeActionFormFieldOptions(field, locale)
                const required = isActionFormFieldRequired(field, values)
                const fieldError = fieldErrors[key]
                const fieldContainerClassName =
                  fieldType === 'textarea' || fieldType === 'json' ? 'md:col-span-2' : ''

                return (
                  <div
                    key={`${key}-${idx}`}
                    className={`${fieldContainerClassName} space-y-2 rounded-lg border p-3 ${
                      fieldError
                        ? 'border-destructive/50 bg-destructive/5'
                        : 'border-input/50 bg-background/90'
                    }`.trim()}
                  >
                    <div className="space-y-1">
                      <div className="flex flex-wrap items-center gap-2">
                        <p className="text-sm font-medium">{label}</p>
                        {required ? (
                          <Badge variant="outline">{t.admin.pluginActionRequiredBadge}</Badge>
                        ) : null}
                        <Badge variant="secondary" className="font-mono text-[11px]">
                          {key}
                        </Badge>
                      </div>
                      {description ? (
                        <p className="text-xs text-muted-foreground">{description}</p>
                      ) : null}
                    </div>
                    {fieldType === 'boolean' ? (
                      <div className="flex items-center justify-between gap-3 rounded-md border border-input/50 bg-muted/20 px-3 py-2">
                        <p className="text-xs text-muted-foreground">
                          {!!currentValue
                            ? t.admin.pluginToggleEnabled
                            : t.admin.pluginToggleDisabled}
                        </p>
                        <Switch
                          checked={!!currentValue}
                          disabled={pending}
                          onCheckedChange={(checked) => setFieldValue(key, checked)}
                        />
                      </div>
                    ) : fieldType === 'textarea' || fieldType === 'json' ? (
                      <Textarea
                        value={typeof currentValue === 'string' ? currentValue : ''}
                        placeholder={placeholder}
                        disabled={pending}
                        rows={
                          typeof field.rows === 'number' && field.rows > 0
                            ? field.rows
                            : fieldType === 'json'
                              ? 8
                              : 4
                        }
                        className={`${fieldType === 'json' ? 'font-mono text-xs' : ''} ${
                          fieldError
                            ? 'border-destructive/60 focus-visible:ring-destructive/30'
                            : ''
                        }`.trim()}
                        onChange={(event) => setFieldValue(key, event.target.value)}
                      />
                    ) : fieldType === 'select' ? (
                      <Select
                        value={
                          typeof currentValue === 'string'
                            ? currentValue
                            : String(currentValue ?? '')
                        }
                        onValueChange={(value) => setFieldValue(key, value)}
                        disabled={pending}
                      >
                        <SelectTrigger
                          className={
                            fieldError ? 'border-destructive/60 focus:ring-destructive/30' : ''
                          }
                        >
                          <SelectValue placeholder={placeholder || label} />
                        </SelectTrigger>
                        <SelectContent>
                          {options.map((option) => {
                            const optionValue = String(option.value ?? '')
                            return (
                              <SelectItem key={`${key}-${optionValue}`} value={optionValue}>
                                {option.label || optionValue}
                              </SelectItem>
                            )
                          })}
                        </SelectContent>
                      </Select>
                    ) : (
                      <Input
                        type={fieldType === 'number' ? 'number' : 'text'}
                        value={
                          typeof currentValue === 'string'
                            ? currentValue
                            : String(currentValue ?? '')
                        }
                        placeholder={placeholder}
                        disabled={pending}
                        className={
                          fieldError
                            ? 'border-destructive/60 focus-visible:ring-destructive/30'
                            : ''
                        }
                        onChange={(event) => setFieldValue(key, event.target.value)}
                      />
                    )}
                    {fieldError ? <p className="text-xs text-destructive">{fieldError}</p> : null}
                  </div>
                )
              })}
            </div>
          )}
        </div>
        <div className="space-y-3 rounded-lg border border-input/60 bg-muted/5 p-3">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <p className="text-sm font-medium">{t.admin.actions}</p>
            <div className="flex flex-wrap items-center gap-2">
              {executeAPI?.stream_url ? (
                <Badge variant="outline">{t.admin.pluginDynamicHostStreaming}</Badge>
              ) : null}
              <Badge variant={pending ? 'secondary' : 'outline'}>
                {t.admin.pluginActionAvailableCount.replace(
                  '{count}',
                  String(availableActionCount)
                )}
              </Badge>
            </div>
          </div>
          <div className="space-y-2">
            <div className="flex flex-wrap gap-2">
              {primaryActionButtons.map((action) => (
                <Button
                  key={action.key}
                  size="sm"
                  variant={action.variant}
                  onClick={() =>
                    dispatchAction({
                      action: action.action,
                      includeFields: action.includeFields,
                      requiredFields: action.requiredFields,
                    })
                  }
                  disabled={pending}
                >
                  {action.label}
                </Button>
              ))}
              {overflowActionButtons.length > 0 ? (
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => setShowOverflowActions((current) => !current)}
                  disabled={pending}
                >
                  {showOverflowActions
                    ? `${t.common.collapse} ${t.admin.pluginMoreActions}`
                    : `${t.admin.pluginMoreActions} (${overflowActionButtons.length})`}
                </Button>
              ) : null}
            </div>
            {showOverflowActions && overflowActionButtons.length > 0 ? (
              <div className="flex flex-wrap gap-2 rounded-md border border-input/60 bg-background/90 p-2">
                {overflowActionButtons.map((action) => (
                  <Button
                    key={`overflow-${action.key}`}
                    size="sm"
                    variant={action.variant}
                    onClick={() =>
                      dispatchAction({
                        action: action.action,
                        includeFields: action.includeFields,
                        requiredFields: action.requiredFields,
                      })
                    }
                    disabled={pending}
                  >
                    {action.label}
                  </Button>
                ))}
              </div>
            ) : null}
          </div>
        </div>
        {hasResultState ? (
          <div ref={resultRef} className={`space-y-3 rounded-lg border p-3 ${resultToneClassName}`}>
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div className="space-y-1">
                <p className="text-sm font-medium">{t.admin.pluginLatestResult}</p>
                {lastActionLabel ? (
                  <p className="text-xs text-muted-foreground">
                    {lastActionLabel}
                    {source ? ` / ${source}` : ''}
                  </p>
                ) : null}
              </div>
              <div className="flex flex-wrap items-center gap-2">
                <Badge
                  variant={
                    lastActionStatus === 'error'
                      ? 'destructive'
                      : lastActionStatus === 'success'
                        ? 'secondary'
                        : 'outline'
                  }
                >
                  {lastActionStatus === 'running'
                    ? t.admin.pluginActionStatusRunning
                    : lastActionStatus === 'success'
                      ? t.admin.pluginActionStatusSuccess
                      : lastActionStatus === 'error'
                        ? t.admin.pluginActionStatusError
                        : t.admin.pluginActionStatusIdle}
                </Badge>
                {lastActionStatus === 'success' && !pending ? (
                  <Badge variant="secondary">{t.admin.pluginActionResultUpdated}</Badge>
                ) : null}
                {resultBlocks.length > 0 ? (
                  <Badge variant="outline">
                    {t.admin.pluginResultBlockCount.replace('{count}', String(resultBlocks.length))}
                  </Badge>
                ) : null}
              </div>
            </div>
            {errorText ? (
              <div className="whitespace-pre-line break-words rounded border border-destructive/40 bg-destructive/10 p-2 text-xs text-destructive">
                {errorText}
              </div>
            ) : null}
            {noticeText ? (
              <div className="whitespace-pre-line break-words rounded border border-input/60 bg-muted/30 p-2 text-xs text-muted-foreground">
                {noticeText}
              </div>
            ) : null}
            {resultBlocks.map((nestedBlock, nestedIndex) =>
              renderBlock(nestedBlock, nestedIndex, {
                area,
                pluginID,
                pagePath,
                pageFullPath,
                queryParams,
                routeParams,
                htmlMode: 'sanitize',
                executeAPI,
                t,
              })
            )}
          </div>
        ) : null}
      </CardContent>
    </Card>
  )
}

type PluginHTMLBlockProps = {
  block: PluginPageBlock
  area: 'admin' | 'user'
  pluginID: number
  pluginName?: string
  pagePath: string
  pageFullPath: string
  queryParams: Record<string, string>
  routeParams: Record<string, string>
  htmlMode: 'sanitize' | 'trusted'
  executeAPI?: PluginFrontendRouteExecuteAPI
  t: ReturnType<typeof getTranslations>
}

function PluginHTMLBlock({
  block,
  area,
  pluginID,
  pluginName,
  pagePath,
  pageFullPath,
  queryParams,
  routeParams,
  htmlMode,
  executeAPI,
  t,
}: PluginHTMLBlockProps) {
  const { locale } = useLocale()
  const router = useRouter()
  const queryClient = useQueryClient()
  const containerRef = useRef<HTMLDivElement | null>(null)
  const title = manifestString(block as Record<string, unknown>, 'title', locale)
  const content = manifestString(block as Record<string, unknown>, 'content', locale)
  const data = useMemo(
    () => (block.data && typeof block.data === 'object' ? block.data : {}),
    [block]
  )
  const htmlPresentation = useMemo(() => normalizePluginHTMLPresentation(data), [data])
  const templateValues = useMemo(
    () =>
      buildPluginTemplateValues({
        area,
        pagePath,
        pageFullPath,
        pluginID,
        pluginName,
        queryParams,
        routeParams,
        executeAPI,
      }),
    [area, pagePath, pageFullPath, pluginID, pluginName, queryParams, routeParams, executeAPI]
  )
  const renderedHtml = useMemo(() => {
    return preparePluginHtmlForRender(interpolatePluginTemplate(content, templateValues), {
      trusted: htmlMode === 'trusted',
    })
  }, [content, htmlMode, templateValues])

  useEffect(() => {
    const root = containerRef.current
    if (!root) return
    const prefetchedPluginPageHrefs = new Set<string>()

    const prefetchAnchor = (anchor: HTMLAnchorElement) => {
      const navigation = resolvePluginAnchorNavigation(anchor)
      if (!navigation?.pluginPrefetchTarget || !navigation.pluginPrefetchHref) {
        return
      }
      if (prefetchedPluginPageHrefs.has(navigation.pluginPrefetchHref)) {
        return
      }
      prefetchedPluginPageHrefs.add(navigation.pluginPrefetchHref)
      router.prefetch(navigation.pluginPrefetchHref)
      void prefetchPluginPageData(queryClient, navigation.pluginPrefetchTarget).catch(() => {
        prefetchedPluginPageHrefs.delete(navigation.pluginPrefetchHref as string)
      })
    }

    const runAxiosBridgeRequest = async (input: {
      url: string
      method?: string
      target: string
      body?: string
    }) => {
      const target = String(input.target || '').trim()
      clearAxiosBridgeFeedback(root, target)

      if (htmlMode !== 'trusted') {
        writeAxiosBridgeValue(
          root,
          'data-plugin-axios-status',
          target,
          t.admin.pluginActionStatusError
        )
        writeAxiosBridgeValue(
          root,
          'data-plugin-axios-error',
          target,
          'Trusted HTML page bridge is unavailable.'
        )
        return
      }

      const url = String(input.url || '').trim()
      if (!url) {
        writeAxiosBridgeValue(
          root,
          'data-plugin-axios-status',
          target,
          t.admin.pluginActionStatusError
        )
        writeAxiosBridgeValue(
          root,
          'data-plugin-axios-error',
          target,
          'Axios bridge URL is required.'
        )
        return
      }

      const method = String(input.method || 'get')
        .trim()
        .toLowerCase()
      const client = typeof window !== 'undefined' && window.axios ? window.axios : apiClient

      writeAxiosBridgeValue(
        root,
        'data-plugin-axios-status',
        target,
        t.admin.pluginActionStatusRunning
      )
      try {
        const response = await client.request({
          url,
          method: method as any,
          data: parseAxiosBridgeBody(String(input.body || '')),
        })
        writeAxiosBridgeValue(
          root,
          'data-plugin-axios-status',
          target,
          t.admin.pluginActionStatusSuccess
        )
        writeAxiosBridgeValue(
          root,
          'data-plugin-axios-result',
          target,
          formatAxiosBridgeResponse(response)
        )
      } catch (error) {
        writeAxiosBridgeValue(
          root,
          'data-plugin-axios-status',
          target,
          t.admin.pluginActionStatusError
        )
        writeAxiosBridgeValue(
          root,
          'data-plugin-axios-error',
          target,
          resolvePluginErrorText(error, t, t.admin.pluginBizErrorDefault)
        )
      }
    }

    const runExecute = async (input: {
      action: string
      params: Record<string, string>
      target: string
      mode?: string
    }) => {
      const action = String(input.action || '').trim()
      const target = String(input.target || '').trim()
      const preferredMode = String(input.mode || '')
        .trim()
        .toLowerCase()
      clearBridgeFeedback(root, target)

      if (!action) {
        writeBridgeValue(root, 'data-plugin-exec-status', target, t.admin.pluginActionStatusError)
        writeBridgeValue(root, 'data-plugin-exec-error', target, t.admin.pluginExecMissingAction)
        return
      }
      if (!executeAPI?.url) {
        writeBridgeValue(root, 'data-plugin-exec-status', target, t.admin.pluginActionStatusError)
        writeBridgeValue(
          root,
          'data-plugin-exec-error',
          target,
          t.admin.pluginActionExecuteUnavailable
        )
        return
      }

      writeBridgeValue(root, 'data-plugin-exec-status', target, t.admin.pluginActionStatusRunning)
      try {
        const request = {
          action,
          params: input.params,
          path: pagePath,
          query_params: queryParams,
          route_params: routeParams,
        }
        const resp = pluginRouteShouldStream(executeAPI, action, preferredMode)
          ? await executePluginRouteActionStream(executeAPI, request, {
              onChunk: (chunk) => {
                if (chunk?.type === 'task' && chunk.task_id) {
                  writeBridgeValue(
                    root,
                    'data-plugin-exec-status',
                    target,
                    `Task ${chunk.task_id} started`
                  )
                  return
                }
                const payload = chunk?.data
                if (payload && typeof payload === 'object' && !Array.isArray(payload)) {
                  writeBridgeValue(root, 'data-plugin-exec-result', target, prettyJSON(payload))
                  const payloadRecord = payload as Record<string, any>
                  const statusText =
                    typeof payloadRecord.status === 'string' ? payloadRecord.status.trim() : ''
                  const progressValue = payloadRecord.progress
                  if (statusText) {
                    if (typeof progressValue === 'number' && Number.isFinite(progressValue)) {
                      writeBridgeValue(
                        root,
                        'data-plugin-exec-status',
                        target,
                        `${statusText}: ${progressValue}%`
                      )
                    } else {
                      writeBridgeValue(root, 'data-plugin-exec-status', target, statusText)
                    }
                  }
                }
              },
            })
          : await executePluginRouteAction(executeAPI, request)
        const payload = parseExecutePayload(resp)
        const displayPayload = looksLikeExecutePayload(payload)
          ? payload
          : resp?.data && typeof resp.data === 'object' && !Array.isArray(resp.data)
            ? (resp.data as Record<string, any>)
            : payload
        if (isExecuteFailed(resp)) {
          writeBridgeValue(root, 'data-plugin-exec-status', target, t.admin.pluginActionStatusError)
          writeBridgeValue(
            root,
            'data-plugin-exec-error',
            target,
            resolvePluginErrorText(resp, t, t.admin.pluginBizErrorDefault)
          )
          if (displayPayload && Object.keys(displayPayload).length > 0) {
            writeBridgeValue(root, 'data-plugin-exec-result', target, prettyJSON(displayPayload))
          }
          return
        }
        writeBridgeValue(root, 'data-plugin-exec-status', target, t.admin.pluginActionStatusSuccess)
        if (displayPayload && Object.keys(displayPayload).length > 0) {
          writeBridgeValue(root, 'data-plugin-exec-result', target, prettyJSON(displayPayload))
        }
      } catch (error) {
        writeBridgeValue(root, 'data-plugin-exec-status', target, t.admin.pluginActionStatusError)
        writeBridgeValue(
          root,
          'data-plugin-exec-error',
          target,
          resolvePluginErrorText(error, t, t.admin.pluginBizErrorDefault)
        )
      }
    }

    const handleSubmit = (event: Event) => {
      const eventTarget = event.target
      const form =
        eventTarget instanceof HTMLFormElement
          ? eventTarget
          : eventTarget instanceof Element
            ? eventTarget.closest('form[data-plugin-exec-form]')
            : null
      if (!(form instanceof HTMLFormElement) || !root.contains(form)) {
        return
      }

      event.preventDefault()
      const submitter =
        (event as SubmitEvent & { submitter?: HTMLElement | null }).submitter || null
      const target = String(
        submitter?.dataset.pluginExecTarget || form.dataset.pluginExecTarget || ''
      ).trim()
      const mode = String(
        submitter?.dataset.pluginExecMode || form.dataset.pluginExecMode || ''
      ).trim()
      const params: Record<string, string> = {
        ...parseExecuteParamMap(form.dataset.pluginExecParams),
        ...parseExecuteParamMap(submitter?.dataset.pluginExecParams),
      }
      let action = String(
        submitter?.dataset.pluginExecAction || form.dataset.pluginExecAction || ''
      ).trim()

      const formData = new FormData(form)
      formData.forEach((value, rawKey) => {
        const key = String(rawKey || '').trim()
        if (!key) return
        const normalizedValue = value instanceof File ? value.name : String(value)
        if (key === 'action') {
          if (!action && normalizedValue.trim()) {
            action = normalizedValue.trim()
          }
          return
        }
        if (key === 'path') {
          return
        }
        params[key] = normalizedValue
      })

      void runExecute({ action, params, target, mode })
    }

    const handleClick = (event: Event) => {
      const eventTarget = event.target
      const trigger = findPluginBridgeTrigger(eventTarget)
      if (trigger instanceof HTMLElement && root.contains(trigger)) {
        const parentForm = trigger.closest('form[data-plugin-exec-form]')
        const triggerType =
          trigger instanceof HTMLButtonElement
            ? String(trigger.getAttribute('type') || 'submit')
                .trim()
                .toLowerCase()
            : ''
        if (parentForm && triggerType !== 'button') {
          return
        }

        const axiosURL = String(trigger.dataset.pluginAxiosUrl || '').trim()
        if (axiosURL) {
          event.preventDefault()
          const target = String(trigger.dataset.pluginAxiosTarget || '').trim()
          const method = String(trigger.dataset.pluginAxiosMethod || '').trim()
          const body = String(trigger.dataset.pluginAxiosBody || '')
          void runAxiosBridgeRequest({ url: axiosURL, method, target, body })
          return
        }

        event.preventDefault()
        const action = String(trigger.dataset.pluginExecAction || '').trim()
        const target = String(trigger.dataset.pluginExecTarget || '').trim()
        const mode = String(trigger.dataset.pluginExecMode || '').trim()
        const params = parseExecuteParamMap(trigger.dataset.pluginExecParams)
        void runExecute({ action, params, target, mode })
        return
      }

      const anchor = findPluginAnchorTrigger(eventTarget)
      if (!(anchor instanceof HTMLAnchorElement) || !root.contains(anchor)) {
        return
      }

      const navigation = resolvePluginAnchorNavigation(anchor)
      if (!navigation || !shouldHandlePluginAnchorClick(event)) {
        return
      }

      event.preventDefault()
      prefetchAnchor(anchor)
      router.push(navigation.href)
    }

    const handleMouseOver = (event: Event) => {
      const anchor = findPluginAnchorTrigger(event.target)
      if (!(anchor instanceof HTMLAnchorElement) || !root.contains(anchor)) {
        return
      }
      prefetchAnchor(anchor)
    }

    const handleFocusIn = (event: Event) => {
      const anchor = findPluginAnchorTrigger(event.target)
      if (!(anchor instanceof HTMLAnchorElement) || !root.contains(anchor)) {
        return
      }
      prefetchAnchor(anchor)
    }

    const handleTouchStart = (event: Event) => {
      const anchor = findPluginAnchorTrigger(event.target)
      if (!(anchor instanceof HTMLAnchorElement) || !root.contains(anchor)) {
        return
      }
      prefetchAnchor(anchor)
    }

    root.addEventListener('submit', handleSubmit)
    root.addEventListener('click', handleClick)
    root.addEventListener('mouseover', handleMouseOver)
    root.addEventListener('focusin', handleFocusIn)
    root.addEventListener('touchstart', handleTouchStart, { passive: true })
    return () => {
      root.removeEventListener('submit', handleSubmit)
      root.removeEventListener('click', handleClick)
      root.removeEventListener('mouseover', handleMouseOver)
      root.removeEventListener('focusin', handleFocusIn)
      root.removeEventListener('touchstart', handleTouchStart)
    }
  }, [
    executeAPI,
    htmlMode,
    pagePath,
    queryClient,
    queryParams,
    routeParams,
    renderedHtml,
    router,
    t,
  ])

  useEffect(() => {
    const root = containerRef.current
    if (!root || htmlMode !== 'trusted') {
      return
    }
    ensureTrustedPluginMarkup(root, renderedHtml)

    let rafId = 0
    let timeoutId: number | null = null
    const scheduleTrustedEnsure = (delay = 0) => {
      if (rafId) {
        window.cancelAnimationFrame(rafId)
      }
      if (timeoutId) {
        window.clearTimeout(timeoutId)
        timeoutId = null
      }
      const run = () => {
        rafId = window.requestAnimationFrame(() => {
          const liveRoot = containerRef.current
          if (!liveRoot || htmlMode !== 'trusted') {
            return
          }
          ensureTrustedPluginMarkup(liveRoot, renderedHtml)
        })
      }
      if (delay > 0) {
        timeoutId = window.setTimeout(run, delay)
        return
      }
      run()
    }

    const observer = new MutationObserver(() => {
      scheduleTrustedEnsure()
    })
    observer.observe(root, {
      childList: true,
      subtree: true,
    })

    const handleTrustedResume = () => {
      scheduleTrustedEnsure()
      scheduleTrustedEnsure(40)
      scheduleTrustedEnsure(120)
    }

    const handleVisibilityChange = () => {
      if (document.visibilityState === 'visible') {
        handleTrustedResume()
      }
    }

    window.addEventListener('pageshow', handleTrustedResume)
    window.addEventListener('popstate', handleTrustedResume)
    document.addEventListener('visibilitychange', handleVisibilityChange)

    return () => {
      observer.disconnect()
      if (rafId) {
        window.cancelAnimationFrame(rafId)
      }
      if (timeoutId) {
        window.clearTimeout(timeoutId)
      }
      window.removeEventListener('pageshow', handleTrustedResume)
      window.removeEventListener('popstate', handleTrustedResume)
      document.removeEventListener('visibilitychange', handleVisibilityChange)
      cleanupTrustedPluginDetachedLayers()
    }
  }, [htmlMode, renderedHtml])

  if (!renderedHtml) return null

  const contentNode = (
    <div
      ref={containerRef}
      className={htmlPresentation.theme === 'host' ? 'plugin-html-host' : undefined}
      dangerouslySetInnerHTML={{ __html: renderedHtml }}
    />
  )

  if (htmlPresentation.chrome === 'bare') {
    return (
      <div className="space-y-2">
        {title ? (
          <p className="px-1 text-xs font-medium uppercase tracking-[0.08em] text-muted-foreground">
            {title}
          </p>
        ) : null}
        {contentNode}
      </div>
    )
  }

  return (
    <Card>
      {title ? (
        <CardHeader className="pb-2">
          <CardTitle className="text-base">{title}</CardTitle>
        </CardHeader>
      ) : null}
      <CardContent className="p-4">{contentNode}</CardContent>
    </Card>
  )
}

function renderBlock(
  block: PluginPageBlock,
  index: number,
  context: {
    area: 'admin' | 'user'
    pluginID: number
    pluginName?: string
    pagePath: string
    pageFullPath: string
    queryParams: Record<string, string>
    routeParams: Record<string, string>
    htmlMode: 'sanitize' | 'trusted'
    executeAPI?: PluginFrontendRouteExecuteAPI
    t: ReturnType<typeof getTranslations>
  }
) {
  const type = (block.type || 'text').trim().toLowerCase()
  const key = `${type}-${index}`
  const blockData =
    block.data && typeof block.data === 'object' && !Array.isArray(block.data)
      ? (block.data as Record<string, unknown>)
      : {}
  const fallbackWhenUntrusted =
    boolOrDefault(blockData.fallback_when_untrusted, false) ||
    boolOrDefault(blockData.fallback_when_no_trusted, false)
  const hideAfterTrustedBoot = boolOrDefault(blockData.hide_after_trusted_boot, false)

  if (type === 'action_form') {
    if (context.htmlMode === 'trusted' && fallbackWhenUntrusted && !hideAfterTrustedBoot) {
      return null
    }
    return (
      <PluginActionFormBlock
        key={key}
        block={block}
        area={context.area}
        pluginID={context.pluginID}
        pagePath={context.pagePath}
        pageFullPath={context.pageFullPath}
        queryParams={context.queryParams}
        routeParams={context.routeParams}
        executeAPI={context.executeAPI}
        t={context.t}
      />
    )
  }

  if (type === 'html') {
    if (context.htmlMode === 'trusted' && fallbackWhenUntrusted) {
      return null
    }
    const trustedOnly = boolOrDefault(blockData.trusted_only, false)
    if (trustedOnly && context.htmlMode !== 'trusted') {
      return null
    }
    return (
      <PluginHTMLBlock
        key={key}
        block={block}
        area={context.area}
        pluginID={context.pluginID}
        pluginName={context.pluginName}
        pagePath={context.pagePath}
        pageFullPath={context.pageFullPath}
        queryParams={context.queryParams}
        routeParams={context.routeParams}
        htmlMode={context.htmlMode}
        executeAPI={context.executeAPI}
        t={context.t}
      />
    )
  }

  return <PluginStructuredBlock key={key} block={block} />
}

export function PluginDynamicPage({ area, slotAnimate }: PluginDynamicPageProps) {
  const router = useRouter()
  const pathname = usePathname() || '/'
  const searchParams = useSearchParams()
  const { data: publicConfig, isFetched: publicConfigLoaded } = useQuery({
    queryKey: ['publicConfig'],
    queryFn: getPublicConfig,
    staleTime: 5 * 60 * 1000,
  })
  const queryParams = useMemo(() => readPluginSearchParams(searchParams), [searchParams])
  const { locale } = useLocale()
  const t = getTranslations(locale)
  const { isAuthenticated, isLoading: authLoading } = useAuth()
  const { hasPermission, isSuperAdmin } = usePermission()
  const pageFullPath = useMemo(() => buildFullPath(pathname, queryParams), [pathname, queryParams])
  const pluginScope = area === 'admin' ? 'admin' : 'public'
  const topSlotName = `${area}.plugin_page.top`
  const bottomSlotName = `${area}.plugin_page.bottom`
  const pluginPlatformEnabled = useMemo(
    () => resolvePluginPlatformEnabled(publicConfig?.data, true),
    [publicConfig]
  )

  const bootstrapQuery = usePluginBootstrapQuery({
    scope: pluginScope,
    path: pathname,
    queryParams,
    enabled: publicConfigLoaded && pluginPlatformEnabled,
  })
  const { targetRef: bottomSlotVisibilityRef, activated: bottomSlotActivated } =
    useVisibilityActivation({
      enabled: true,
      rootMargin: '480px 0px',
    })
  const topSlotQuery = usePluginSlotExtensionsQuery({
    scope: pluginScope,
    path: pathname,
    slot: topSlotName,
    queryParams,
    enabled: publicConfigLoaded && pluginPlatformEnabled,
  })
  const bottomSlotQuery = usePluginSlotExtensionsQuery({
    scope: pluginScope,
    path: pathname,
    slot: bottomSlotName,
    queryParams,
    enabled: publicConfigLoaded && pluginPlatformEnabled && bottomSlotActivated,
  })

  const routes = useMemo(() => extractRoutes(bootstrapQuery.data), [bootstrapQuery.data])
  const matchedRoute = useMemo(() => {
    for (const route of routes) {
      if (typeof route.path !== 'string' || route.path.trim() === '') continue
      const match = matchRoute(route.path, pathname)
      if (match.matched) {
        return {
          ...route,
          route_params:
            Object.keys(normalizeStringMap(route.route_params)).length > 0
              ? normalizeStringMap(route.route_params)
              : match.routeParams,
        }
      }
    }
    return null
  }, [routes, pathname])
  const routeParams = useMemo(() => normalizeStringMap(matchedRoute?.route_params), [matchedRoute])
  const pageSchema = useMemo(() => extractPageSchema(matchedRoute, locale), [locale, matchedRoute])
  const pageConfig = useMemo(
    () =>
      matchedRoute?.page &&
      typeof matchedRoute.page === 'object' &&
      !Array.isArray(matchedRoute.page)
        ? (matchedRoute.page as Record<string, any>)
        : undefined,
    [matchedRoute]
  )
  const globalSlotAnimate = useMemo(
    () => resolvePluginGlobalSlotAnimationsEnabled(publicConfig?.data, true),
    [publicConfig]
  )
  const globalSlotLoadingEnabled = useMemo(
    () => resolvePluginGlobalSlotLoadingEnabled(publicConfig?.data, false),
    [publicConfig]
  )
  const slotAnimationFallback = useMemo(
    () => resolvePluginSlotAnimationDefault(slotAnimate, globalSlotAnimate),
    [globalSlotAnimate, slotAnimate]
  )
  const topSlotAnimate = useMemo(
    () => resolvePluginPageSlotAnimation(pageConfig, 'top', slotAnimationFallback),
    [pageConfig, slotAnimationFallback]
  )
  const bottomSlotAnimate = useMemo(
    () => resolvePluginPageSlotAnimation(pageConfig, 'bottom', slotAnimationFallback),
    [pageConfig, slotAnimationFallback]
  )
  const routeAllowed = useMemo(() => {
    if (!matchedRoute) return false
    const requiredPermissions = normalizePermissionList(matchedRoute.required_permissions)
    if (area === 'admin') {
      if (matchedRoute.super_admin_only && !isSuperAdmin()) return false
      if (
        requiredPermissions.length > 0 &&
        !requiredPermissions.every((permission) => hasPermission(permission))
      ) {
        return false
      }
      return true
    }
    if (!isAuthenticated && !matchedRoute.guest_visible) return false
    return true
  }, [area, hasPermission, isAuthenticated, isSuperAdmin, matchedRoute])
  const pageBlocks = Array.isArray(pageSchema.blocks) ? pageSchema.blocks : []
  const pageTitle =
    pageSchema.title ||
    manifestString((matchedRoute || {}) as Record<string, unknown>, 'title', locale) ||
    pathname
  const pageDescription = (pageSchema.description || '').trim()
  const pageHostHeader = pageSchema.host_header === 'hide' ? 'hide' : 'show'
  const pageHTMLMode: 'sanitize' | 'trusted' =
    String(matchedRoute?.html_mode || '')
      .trim()
      .toLowerCase() === 'trusted'
      ? 'trusted'
      : 'sanitize'
  const executeAPI = matchedRoute?.execute_api
  const bootstrapErrorText = bootstrapQuery.isError
    ? resolvePluginOperationErrorMessage(bootstrapQuery.error, t, t.admin.pluginBizErrorDefault)
    : ''
  const topSlotRefreshing = shouldShowPluginSlotRefreshing(
    topSlotQuery.hasData,
    topSlotQuery.isFetching,
    topSlotQuery.extensions
  )
  const bottomSlotRefreshing = shouldShowPluginSlotRefreshing(
    bottomSlotQuery.hasData,
    bottomSlotQuery.isFetching,
    bottomSlotQuery.extensions
  )
  const pluginID = Number(matchedRoute?.plugin_id || 0)
  const pluginName = String(matchedRoute?.plugin_name || '').trim()
  const pluginIdentityText = pluginName || (pluginID > 0 ? `#${pluginID}` : t.common.noData)
  const isLegacyAdminMarketPage = area === 'admin' && pathname === '/admin/plugin-pages/market'
  const isAdminMarketPage =
    area === 'admin' && (pageSchema.host_market_workspace === true || isLegacyAdminMarketPage)

  useEffect(() => {
    if (typeof window === 'undefined') {
      return
    }

    const previousAxios = window.axios
    const previousPluginAxios = window.pluginAxios
    const previousBridge = window.AuraLogicPluginPage

    const bridge: PluginPageBridge = {
      version: 1,
      area,
      plugin_id: pluginID,
      plugin_name: pluginName || undefined,
      path: pathname,
      full_path: pageFullPath,
      query_params: { ...queryParams },
      route_params: { ...routeParams },
      html_mode: pageHTMLMode,
      execute_api: executeAPI,
      axios: apiClient,
      axios_raw: axios,
      should_stream: (action: string, preferredMode?: string) =>
        pluginRouteShouldStream(executeAPI || {}, action, preferredMode),
      execute: async (input: PluginPageBridgeExecuteInput) => {
        const action = String(input?.action || '').trim()
        if (!action) {
          throw new Error('Plugin execute action is required')
        }
        if (!executeAPI?.url) {
          throw new Error('Plugin execute API is unavailable for the current page route')
        }

        const request = {
          action,
          params: normalizePluginBridgeParams(input?.params),
          path: pathname,
          query_params: queryParams,
          route_params: routeParams,
        }

        if (pluginRouteShouldStream(executeAPI, action, input?.mode)) {
          return executePluginRouteActionStream(executeAPI, request, {
            locale,
            onChunk: input?.onChunk,
          })
        }

        return executePluginRouteAction(executeAPI, request, { locale })
      },
    }

    window.axios = apiClient
    window.pluginAxios = apiClient
    window.AuraLogicPluginPage = bridge

    return () => {
      if (window.AuraLogicPluginPage === bridge) {
        window.AuraLogicPluginPage = previousBridge
      }
      if (window.pluginAxios === apiClient) {
        window.pluginAxios = previousPluginAxios
      }
      if (window.axios === apiClient) {
        window.axios = previousAxios
      }
    }
  }, [
    area,
    executeAPI,
    locale,
    pageFullPath,
    pageHTMLMode,
    pathname,
    pluginID,
    pluginName,
    queryParams,
    routeParams,
  ])

  const marketKind = String(queryParams.kind || '')
    .trim()
    .toLowerCase()
  const marketSourceID = String(queryParams.source_id || '').trim()
  const marketName = String(queryParams.name || '').trim()
  const marketVersion = String(queryParams.version || '').trim()
  const marketTarget = (() => {
    if (marketKind === 'email_template') {
      return String(queryParams.email_key || '').trim()
    }
    if (marketKind === 'landing_page_template') {
      return String(queryParams.landing_slug || '').trim()
    }
    if (marketKind === 'invoice_template') {
      return String(queryParams.target_key || 'invoice').trim()
    }
    if (marketKind === 'auth_branding_template') {
      return String(queryParams.target_key || 'auth_branding').trim()
    }
    if (marketKind === 'page_rule_pack') {
      return String(queryParams.target_key || 'page_rules').trim()
    }
    return ''
  })()
  const marketKindLabel = (() => {
    switch (marketKind) {
      case 'plugin_package':
        return t.pageTitle.adminPlugins
      case 'payment_package':
        return t.pageTitle.adminPaymentMethods
      case 'email_template':
        return t.admin.emailTemplateEditor
      case 'landing_page_template':
        return t.admin.landingPage
      case 'invoice_template':
        return t.admin.invoiceTitle
      case 'auth_branding_template':
        return t.admin.authBranding
      case 'page_rule_pack':
        return t.admin.pageRules
      default:
        return marketKind || t.common.noData
    }
  })()
  let adminTargetHref = '/admin/plugins'
  let adminTargetName = t.pageTitle.adminPlugins
  if (isAdminMarketPage) {
    switch (marketKind) {
      case 'payment_package':
        adminTargetHref = '/admin/payment-methods'
        adminTargetName = t.pageTitle.adminPaymentMethods
        break
      case 'email_template':
      case 'landing_page_template':
      case 'invoice_template':
      case 'auth_branding_template':
      case 'page_rule_pack':
        adminTargetHref = '/admin/settings'
        adminTargetName = t.pageTitle.adminSettings
        break
      default:
        adminTargetHref = '/admin/plugins'
        adminTargetName = t.pageTitle.adminPlugins
        break
    }
  }
  const adminTargetLabel = isAdminMarketPage
    ? t.admin.pluginDynamicHostBackToTarget.replace('{target}', adminTargetName)
    : t.admin.pluginObservabilityBackToPlugins
  const handleLoginToContinue = () => {
    setAuthReturnState({ redirectPath: pageFullPath })
    router.push('/login')
  }
  const renderHostHeaderCard = (options: {
    title: string
    description?: string
    content?: ReactNode
    contentTone?: 'default' | 'danger' | 'warning'
    showAreaTargetLink?: boolean
    actions?: ReactNode
  }) => {
    const adminTrailSegments =
      area === 'admin'
        ? [
            t.pageTitle.adminPlugins,
            isAdminMarketPage ? t.admin.pluginMarket : pluginIdentityText,
            options.title,
          ]
        : []
    const contentToneClassName =
      options.contentTone === 'danger'
        ? 'border-destructive/30 bg-destructive/5 text-destructive'
        : options.contentTone === 'warning'
          ? 'border-amber-500/30 bg-amber-500/10 text-amber-700 dark:border-amber-500/40 dark:bg-amber-950/20 dark:text-amber-300'
          : 'border-input/60 bg-muted/10 text-foreground'

    return (
      <Card>
        <CardHeader className="space-y-4">
          {area === 'admin' ? (
            <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
              {adminTrailSegments.map((segment, index) => (
                <div key={`${segment}-${index}`} className="flex items-center gap-2">
                  {index > 0 ? <span className="text-muted-foreground/60">/</span> : null}
                  <Badge
                    variant={index === adminTrailSegments.length - 1 ? 'secondary' : 'outline'}
                  >
                    {segment}
                  </Badge>
                </div>
              ))}
            </div>
          ) : null}
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div className="min-w-0 flex-1 space-y-1">
              <div className="flex flex-wrap items-center gap-2">
                <CardTitle className="break-words">{options.title}</CardTitle>
                {area === 'admin' ? (
                  <>
                    <Badge variant="outline">{pluginIdentityText}</Badge>
                    {isAdminMarketPage ? (
                      <Badge variant="active">{t.admin.pluginMarket}</Badge>
                    ) : null}
                    {bootstrapQuery.isFetching ? (
                      <Badge variant="secondary">{t.common.processing}</Badge>
                    ) : null}
                  </>
                ) : null}
              </div>
              {options.description ? (
                <CardDescription>{options.description}</CardDescription>
              ) : null}
            </div>
            <div className="flex flex-wrap gap-2">
              {options.actions ? (
                options.actions
              ) : area === 'admin' ? (
                <>
                  <Button asChild>
                    <Link href={adminTargetHref}>{adminTargetLabel}</Link>
                  </Button>
                  <Button variant="outline" asChild>
                    <Link href="/admin/plugins/observability">{t.admin.pluginObservability}</Link>
                  </Button>
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => void bootstrapQuery.refetch()}
                  >
                    {t.common.refresh}
                  </Button>
                  <Button type="button" variant="ghost" onClick={() => router.back()}>
                    {t.common.back}
                  </Button>
                </>
              ) : options.showAreaTargetLink ? (
                <>
                  <Button asChild>
                    <Link href="/products">{t.pageTitle.products}</Link>
                  </Button>
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => void bootstrapQuery.refetch()}
                  >
                    {t.common.refresh}
                  </Button>
                  <Button type="button" variant="ghost" onClick={() => router.back()}>
                    {t.common.back}
                  </Button>
                </>
              ) : (
                <>
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => void bootstrapQuery.refetch()}
                  >
                    {t.common.refresh}
                  </Button>
                  <Button type="button" variant="ghost" onClick={() => router.back()}>
                    {t.common.back}
                  </Button>
                </>
              )}
            </div>
          </div>
        </CardHeader>
        {options.content ? (
          <CardContent>
            <div
              className={`whitespace-pre-line break-words rounded-md border p-4 text-sm ${contentToneClassName}`}
            >
              {options.content}
            </div>
          </CardContent>
        ) : null}
      </Card>
    )
  }
  const renderPageHeader = (options: {
    title: string
    description?: string
    actions?: ReactNode
  }) => (
    <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
      <div className="min-w-0 flex-1 space-y-1">
        <h1 className="break-words text-2xl font-semibold tracking-tight">{options.title}</h1>
        {options.description ? (
          <p className="text-sm text-muted-foreground">{options.description}</p>
        ) : null}
      </div>
      {options.actions ? <div className="flex flex-wrap gap-2">{options.actions}</div> : null}
    </div>
  )
  const pageContainerClassName = isAdminMarketPage
    ? 'mx-auto w-full max-w-6xl space-y-4'
    : 'mx-auto max-w-5xl space-y-4'

  if (
    !publicConfigLoaded ||
    bootstrapQuery.isLoading ||
    (area === 'user' && authLoading && !isAuthenticated)
  ) {
    return <PluginDynamicPageSkeleton />
  }

  if (!pluginPlatformEnabled) {
    return (
      <div className="mx-auto max-w-4xl space-y-4">
        {renderHostHeaderCard({
          title: area === 'admin' ? t.common.noData : t.admin.pluginDynamicRouteUnavailableTitle,
          description: pathname,
          content:
            area === 'admin'
              ? t.admin.pluginDynamicRouteMissingAdminHint
              : t.admin.pluginDynamicRouteMissingUserHint,
          contentTone: 'warning',
          showAreaTargetLink: area !== 'admin',
        })}
      </div>
    )
  }

  if (bootstrapQuery.isError) {
    return (
      <div className="mx-auto max-w-4xl space-y-4">
        {renderHostHeaderCard({
          title: t.common.error,
          description: pathname,
          content:
            area === 'admin' ? (
              bootstrapErrorText
            ) : (
              <div className="space-y-2 whitespace-normal">
                <p className="font-medium">{t.admin.pluginDynamicUserErrorHint}</p>
                <p className="text-xs opacity-80">{bootstrapErrorText || t.common.error}</p>
              </div>
            ),
          contentTone: 'danger',
          showAreaTargetLink: area !== 'admin',
        })}
      </div>
    )
  }

  if (!matchedRoute) {
    return (
      <div className="mx-auto max-w-4xl space-y-4">
        {renderHostHeaderCard({
          title:
            area === 'admin'
              ? isAdminMarketPage
                ? t.admin.pluginMarket
                : t.common.noData
              : t.admin.pluginDynamicRouteUnavailableTitle,
          description: pathname,
          content:
            area === 'admin'
              ? isAdminMarketPage
                ? t.admin.pluginMarketUnavailableHint
                : t.admin.pluginDynamicRouteMissingAdminHint
              : t.admin.pluginDynamicRouteMissingUserHint,
          contentTone: 'warning',
          showAreaTargetLink: area !== 'admin',
        })}
      </div>
    )
  }

  if (!routeAllowed) {
    return (
      <div className="mx-auto max-w-4xl space-y-4">
        {renderHostHeaderCard({
          title: area === 'admin' ? t.common.noAccess : t.auth.loginRequiredTitle,
          description: pathname,
          content: area === 'admin' ? t.common.noAccess : t.auth.loginRequiredDesc,
          contentTone: 'warning',
          showAreaTargetLink: area !== 'admin',
          actions:
            area === 'admin' ? undefined : (
              <>
                <Button type="button" onClick={handleLoginToContinue}>
                  {t.auth.loginToContinue}
                </Button>
                <Button variant="outline" asChild>
                  <Link href="/products">{t.pageTitle.products}</Link>
                </Button>
                <Button type="button" variant="ghost" onClick={() => router.back()}>
                  {t.common.back}
                </Button>
              </>
            ),
        })}
      </div>
    )
  }

  return (
    <div className={pageContainerClassName} data-plugin-page-root="">
      {topSlotQuery.extensions.length > 0 ? (
        <PluginSlotContent
          extensions={topSlotQuery.extensions}
          animate={topSlotAnimate}
          refreshing={topSlotRefreshing}
        />
      ) : globalSlotLoadingEnabled && !topSlotQuery.hasData && topSlotQuery.isLoading ? (
        <PluginSlotSkeleton variant={resolvePluginSlotSkeletonVariant(topSlotName)} />
      ) : null}
      {pageHostHeader === 'show'
        ? renderPageHeader({
            title: pageTitle,
            description: pageDescription || undefined,
            actions:
              area === 'admin' ? (
                <>
                  <Button asChild>
                    <Link href={adminTargetHref}>{adminTargetLabel}</Link>
                  </Button>
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => void bootstrapQuery.refetch()}
                  >
                    {t.common.refresh}
                  </Button>
                </>
              ) : (
                <>
                  <Button asChild>
                    <Link href="/products">{t.pageTitle.products}</Link>
                  </Button>
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => void bootstrapQuery.refetch()}
                  >
                    {t.common.refresh}
                  </Button>
                </>
              ),
          })
        : null}
      {pageBlocks.map((block, index) =>
        renderBlock(block, index, {
          area,
          pluginID,
          pluginName: matchedRoute?.plugin_name,
          pagePath: pathname,
          pageFullPath,
          queryParams,
          routeParams,
          htmlMode: pageHTMLMode,
          executeAPI,
          t,
        })
      )}
      <div ref={bottomSlotVisibilityRef}>
        {bottomSlotQuery.extensions.length > 0 ? (
          <PluginSlotContent
            extensions={bottomSlotQuery.extensions}
            animate={bottomSlotAnimate}
            refreshing={bottomSlotRefreshing}
          />
        ) : bottomSlotActivated &&
          globalSlotLoadingEnabled &&
          !bottomSlotQuery.hasData &&
          (bottomSlotQuery.isLoading || bottomSlotQuery.isFetching) ? (
          <PluginSlotSkeleton variant={resolvePluginSlotSkeletonVariant(bottomSlotName)} />
        ) : null}
      </div>
    </div>
  )
}
