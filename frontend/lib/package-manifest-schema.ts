import type {
  PluginJSONSchema,
  PluginJSONSchemaField,
  PluginJSONSchemaFieldOption,
} from '@/components/admin/plugins/types'

function isManifestObject(value: unknown): value is Record<string, unknown> {
  return !!value && typeof value === 'object' && !Array.isArray(value)
}

function formatSchemaFieldLabel(key: string): string {
  const normalized = String(key || '').trim()
  if (!normalized) return ''
  return normalized
    .replace(/[_-]+/g, ' ')
    .replace(/([a-z0-9])([A-Z])/g, '$1 $2')
    .replace(/\s+/g, ' ')
    .trim()
    .replace(/\b\w/g, (char) => char.toUpperCase())
}

export function parseManifestObject(value: unknown): Record<string, unknown> | null {
  if (isManifestObject(value)) {
    return value
  }
  if (typeof value !== 'string') {
    return null
  }

  const trimmed = value.trim()
  const source = trimmed || '{}'
  try {
    const parsed = JSON.parse(source)
    return isManifestObject(parsed) ? (parsed as Record<string, unknown>) : null
  } catch {
    return null
  }
}

export function parseJSONObjectText(value: string): Record<string, unknown> | null {
  return parseManifestObject(value)
}

export function isJSONObjectText(value: string): boolean {
  return parseJSONObjectText(value) !== null
}

export function formatHumanReadableJSONText(value: string, fallback = '{}'): string {
  const trimmed = value.trim()
  if (!trimmed) return fallback
  const parsed = parseJSONObjectText(trimmed)
  if (!parsed) {
    return fallback
  }
  return JSON.stringify(parsed, null, 2)
}

export function tryFormatTextareaJSON(
  value: string,
  fallback: string,
  formatter: (value: string, fallback: string) => string
): string {
  const trimmed = value.trim()
  if (!trimmed) return fallback
  try {
    return formatter(trimmed, fallback)
  } catch {
    return value
  }
}

function normalizeManifestLocaleKey(value: string): string {
  return value.trim().toLowerCase().replace(/_/g, '-')
}

function buildManifestLocaleCandidates(locale?: string): string[] {
  const normalized = normalizeManifestLocaleKey(String(locale || ''))
  if (!normalized) return []

  const seen = new Set<string>()
  const out: string[] = []
  const parts = normalized.split('-').filter(Boolean)
  for (let index = parts.length; index >= 1; index -= 1) {
    const candidate = parts.slice(0, index).join('-')
    if (!candidate || seen.has(candidate)) continue
    seen.add(candidate)
    out.push(candidate)
  }
  return out
}

function manifestScalarString(value: unknown): string {
  if (typeof value === 'string') return value.trim()
  if (typeof value === 'number' || typeof value === 'boolean') return String(value)
  return ''
}

export function resolveManifestLocalizedString(value: unknown, locale?: string): string {
  const direct = manifestScalarString(value)
  if (direct) return direct
  if (!isManifestObject(value)) return ''

  const entries = Object.entries(value)
    .map(([key, item]) => [normalizeManifestLocaleKey(key), manifestScalarString(item)] as const)
    .filter((entry) => entry[1] !== '')

  if (entries.length === 0) return ''

  const byKey = new Map<string, string>()
  entries.forEach(([key, item]) => {
    if (!byKey.has(key)) {
      byKey.set(key, item)
    }
  })

  const prioritizedKeys = [
    ...buildManifestLocaleCandidates(locale),
    'default',
    'fallback',
    'value',
    'text',
    'title',
    'label',
    'en-us',
    'en',
    'zh-cn',
    'zh',
  ]
  for (const key of prioritizedKeys) {
    const matched = byKey.get(key)
    if (matched) {
      return matched
    }
  }

  return entries[0]?.[1] || ''
}

export function manifestString(
  manifest: unknown,
  key: string,
  locale?: string
): string {
  const source = parseManifestObject(manifest)
  if (!source) return ''
  const localizedValue = resolveManifestLocalizedString(source[`${key}_i18n`], locale)
  if (localizedValue) {
    return localizedValue
  }
  return resolveManifestLocalizedString(source[key], locale)
}

function parseManifestSchemaFieldType(value: unknown): PluginJSONSchemaField['type'] {
  const normalized = String(value || '')
    .trim()
    .toLowerCase()
  switch (normalized) {
    case 'secret':
    case 'textarea':
    case 'number':
    case 'boolean':
    case 'select':
    case 'json':
      return normalized
    default:
      return 'string'
  }
}

function parseManifestSchemaFieldOptions(
  value: unknown,
  locale?: string
): PluginJSONSchemaFieldOption[] {
  if (!Array.isArray(value)) return []
  const options: PluginJSONSchemaFieldOption[] = []
  value.forEach((item) => {
    if (item && typeof item === 'object' && !Array.isArray(item)) {
      const option = item as Record<string, unknown>
      const optionValue =
        option.value !== undefined
          ? option.value
          : typeof option.key === 'string'
            ? option.key
            : undefined
      if (optionValue === undefined) return
      options.push({
        label: manifestString(option, 'label', locale) || String(optionValue),
        value: optionValue,
        description: manifestString(option, 'description', locale) || undefined,
      })
      return
    }
    if (item === null || item === undefined) return
    options.push({
      label: String(item),
      value: item,
    })
  })
  return options
}

export function parseManifestObjectSchema(sourceValue: unknown, locale?: string): PluginJSONSchema | null {
  const source = parseManifestObject(sourceValue)
  if (!source) return null
  const fields = Array.isArray(source.fields) ? source.fields : []
  const seen = new Set<string>()
  const normalizedFields: PluginJSONSchemaField[] = []

  fields.forEach((item) => {
    if (!item || typeof item !== 'object' || Array.isArray(item)) return
    const raw = item as Record<string, unknown>
    const fieldKey = String(raw.key || '').trim()
    if (!fieldKey || seen.has(fieldKey)) return
    seen.add(fieldKey)
    normalizedFields.push({
      key: fieldKey,
      label: manifestString(raw, 'label', locale) || formatSchemaFieldLabel(fieldKey),
      description: manifestString(raw, 'description', locale) || undefined,
      type: parseManifestSchemaFieldType(raw.type),
      placeholder: manifestString(raw, 'placeholder', locale) || undefined,
      required: raw.required === true,
      defaultValue: raw.default,
      options: parseManifestSchemaFieldOptions(raw.options, locale),
    })
  })

  if (normalizedFields.length === 0) return null
  return {
    title: manifestString(source, 'title', locale) || undefined,
    description: manifestString(source, 'description', locale) || undefined,
    fields: normalizedFields,
  }
}

export function findMissingRequiredSchemaFields(
  schema: PluginJSONSchema | null,
  value: Record<string, unknown> | null
): PluginJSONSchemaField[] {
  if (!schema || schema.fields.length === 0) return []
  const source = value || {}
  return schema.fields.filter((field) => {
    if (!field.required) return false
    const currentValue = source[field.key]
    if (currentValue === undefined || currentValue === null) {
      return true
    }
    if (typeof currentValue === 'string') {
      return currentValue.trim() === ''
    }
    return false
  })
}
