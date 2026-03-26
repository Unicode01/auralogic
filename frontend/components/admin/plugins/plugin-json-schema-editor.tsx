'use client'

import { useEffect, useMemo, useState } from 'react'

import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import type { Translations } from '@/lib/i18n'

import { PluginGovernanceNotice } from './plugin-governance-summary'
import type { PluginJSONSchema, PluginJSONSchemaField } from './types'

type PluginJSONSchemaEditorProps = {
  title: string
  description: string
  schema: PluginJSONSchema
  value: string
  onChange: (value: string) => void
  disabled?: boolean
  disabledReason?: string
  t: Translations
}

function parseObjectText(value: string): Record<string, unknown> {
  const trimmed = value.trim()
  if (!trimmed) return {}
  try {
    const parsed = JSON.parse(trimmed)
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      return parsed as Record<string, unknown>
    }
  } catch {
    return {}
  }
  return {}
}

function encodeSelectValue(value: unknown): string {
  return JSON.stringify(value)
}

function decodeSelectValue(value: string): unknown {
  return JSON.parse(value)
}

function formatDraftValue(field: PluginJSONSchemaField, value: unknown): string {
  const source = value === undefined ? field.defaultValue : value
  switch (field.type) {
    case 'number':
      return typeof source === 'number' ? String(source) : source === undefined ? '' : String(source)
    case 'json':
      return source === undefined ? '' : JSON.stringify(source, null, 2)
    case 'select':
      return source === undefined ? '' : encodeSelectValue(source)
    default:
      return source === undefined ? '' : String(source)
  }
}

function formatDisplayLabel(field: PluginJSONSchemaField): string {
  return field.required ? `${field.label} *` : field.label
}

function hasOwnObjectKey(object: Record<string, unknown>, key: string): boolean {
  return Object.prototype.hasOwnProperty.call(object, key)
}

function schemaFieldTypeLabel(field: PluginJSONSchemaField, t: Translations): string {
  switch (field.type) {
    case 'number':
      return t.admin.pluginJsonObjectEditorTypeNumber
    case 'boolean':
      return t.admin.pluginJsonObjectEditorTypeBoolean
    case 'json':
      return t.admin.pluginJsonObjectEditorTypeJson
    case 'string':
      return t.admin.pluginJsonObjectEditorTypeString
    default:
      return field.type
  }
}

export function PluginJSONSchemaEditor({
  title,
  description,
  schema,
  value,
  onChange,
  disabled = false,
  disabledReason,
  t,
}: PluginJSONSchemaEditorProps) {
  const currentObject = useMemo(() => parseObjectText(value), [value])
  const [drafts, setDrafts] = useState<Record<string, string>>({})
  const [errors, setErrors] = useState<Record<string, string>>({})
  const configuredFieldCount = schema.fields.filter((field) =>
    hasOwnObjectKey(currentObject, field.key)
  ).length
  const defaultedFieldCount = schema.fields.filter(
    (field) => !hasOwnObjectKey(currentObject, field.key) && field.defaultValue !== undefined
  ).length
  const requiredFieldCount = schema.fields.filter((field) => field.required).length
  const errorCount = Object.keys(errors).length

  useEffect(() => {
    const nextDrafts: Record<string, string> = {}
    schema.fields.forEach((field) => {
      nextDrafts[field.key] = formatDraftValue(field, currentObject[field.key])
    })
    setDrafts(nextDrafts)
    setErrors({})
  }, [currentObject, schema.fields])

  const commitFieldValue = (field: PluginJSONSchemaField, nextValue: unknown) => {
    const nextObject: Record<string, unknown> = { ...currentObject }
    if (nextValue === undefined) {
      delete nextObject[field.key]
    } else {
      nextObject[field.key] = nextValue
    }
    onChange(JSON.stringify(nextObject, null, 2))
  }

  const setFieldError = (fieldKey: string, message?: string) => {
    setErrors((current) => {
      const next = { ...current }
      if (message) next[fieldKey] = message
      else delete next[fieldKey]
      return next
    })
  }

  const renderField = (field: PluginJSONSchemaField) => {
    const fieldKey = field.key
    const draftValue = drafts[fieldKey] ?? formatDraftValue(field, currentObject[fieldKey])

    switch (field.type) {
      case 'textarea':
        return (
          <Textarea
            value={draftValue}
            disabled={disabled}
            rows={3}
            placeholder={field.placeholder}
            onChange={(e) => {
              const nextValue = e.target.value
              setDrafts((current) => ({ ...current, [fieldKey]: nextValue }))
              setFieldError(fieldKey)
              commitFieldValue(field, nextValue)
            }}
          />
        )
      case 'number':
        return (
          <Input
            value={draftValue}
            disabled={disabled}
            inputMode="decimal"
            placeholder={field.placeholder || t.admin.pluginSchemaNumberPlaceholder}
            onChange={(e) => {
              const nextValue = e.target.value
              setDrafts((current) => ({ ...current, [fieldKey]: nextValue }))
              if (!nextValue.trim()) {
                setFieldError(fieldKey)
                commitFieldValue(field, undefined)
                return
              }
              const numeric = Number(nextValue)
              if (!Number.isFinite(numeric)) {
                setFieldError(fieldKey, t.admin.pluginSchemaInvalidNumber.replace('{key}', field.label))
                return
              }
              setFieldError(fieldKey)
              commitFieldValue(field, numeric)
            }}
          />
        )
      case 'boolean':
        return (
          <div className="flex h-10 items-center justify-between rounded-md border border-input px-3">
            <span className="text-sm text-muted-foreground">
              {currentObject[fieldKey] === true ||
              (currentObject[fieldKey] === undefined && field.defaultValue === true)
                ? t.admin.pluginJsonObjectEditorBooleanTrue
                : t.admin.pluginJsonObjectEditorBooleanFalse}
            </span>
            <Switch
              checked={
                typeof currentObject[fieldKey] === 'boolean'
                  ? Boolean(currentObject[fieldKey])
                  : field.defaultValue === true
              }
              disabled={disabled}
              onCheckedChange={(checked) => {
                setFieldError(fieldKey)
                commitFieldValue(field, checked)
              }}
            />
          </div>
        )
      case 'select':
        return (
          <Select
            value={draftValue}
            disabled={disabled}
            onValueChange={(nextValue) => {
              setDrafts((current) => ({ ...current, [fieldKey]: nextValue }))
              setFieldError(fieldKey)
              commitFieldValue(field, decodeSelectValue(nextValue))
            }}
          >
            <SelectTrigger>
              <SelectValue placeholder={field.placeholder || t.admin.pluginSchemaSelectPlaceholder} />
            </SelectTrigger>
            <SelectContent>
              {field.options?.map((option) => (
                <SelectItem key={`${fieldKey}-${encodeSelectValue(option.value)}`} value={encodeSelectValue(option.value)}>
                  {option.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )
      case 'json':
        return (
          <Textarea
            value={draftValue}
            disabled={disabled}
            rows={4}
            className="font-mono text-xs"
            placeholder={field.placeholder || t.admin.pluginJsonObjectEditorJsonPlaceholder}
            onChange={(e) => {
              const nextValue = e.target.value
              setDrafts((current) => ({ ...current, [fieldKey]: nextValue }))
              if (!nextValue.trim()) {
                setFieldError(fieldKey)
                commitFieldValue(field, undefined)
                return
              }
              try {
                const parsed = JSON.parse(nextValue)
                setFieldError(fieldKey)
                commitFieldValue(field, parsed)
              } catch {
                setFieldError(fieldKey, t.admin.pluginSchemaInvalidJson.replace('{key}', field.label))
              }
            }}
          />
        )
      default:
        return (
          <Input
            value={draftValue}
            disabled={disabled}
            placeholder={field.placeholder}
            onChange={(e) => {
              const nextValue = e.target.value
              setDrafts((current) => ({ ...current, [fieldKey]: nextValue }))
              setFieldError(fieldKey)
              commitFieldValue(field, nextValue)
            }}
          />
        )
    }
  }

  return (
    <div className="space-y-3 rounded-md border border-input p-3">
      <div className="space-y-1">
        <p className="text-sm font-medium">{title}</p>
        <p className="text-xs text-muted-foreground">{description}</p>
      </div>

      {disabledReason ? <PluginGovernanceNotice tone="danger">{disabledReason}</PluginGovernanceNotice> : null}
      {errorCount > 0 ? (
        <PluginGovernanceNotice tone="danger">
          {`${t.common.warning}: ${errorCount}`}
        </PluginGovernanceNotice>
      ) : null}

      <div className="grid gap-3">
        {schema.fields.length === 0 ? (
          <PluginGovernanceNotice tone="neutral">{t.common.noData}</PluginGovernanceNotice>
        ) : (
          schema.fields.map((field) => {
            const hasExplicitValue = hasOwnObjectKey(currentObject, field.key)
            const usingDefault = !hasExplicitValue && field.defaultValue !== undefined
            const hasError = !!errors[field.key]
            return (
              <div
                key={field.key}
                className={`space-y-2 rounded-md border p-3 ${
                  hasError
                    ? 'border-destructive/30 bg-destructive/5'
                    : hasExplicitValue
                      ? 'border-primary/30 bg-primary/5'
                      : 'border-input/60'
                }`}
              >
                <div className="space-y-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <Label>{formatDisplayLabel(field)}</Label>
                    <Badge variant="outline">{schemaFieldTypeLabel(field, t)}</Badge>
                    <Badge
                      variant={
                        hasError
                          ? 'destructive'
                          : hasExplicitValue
                            ? 'active'
                            : usingDefault
                              ? 'outline'
                              : 'outline'
                      }
                    >
                      {hasError
                        ? t.common.warning
                        : hasExplicitValue
                          ? t.admin.pluginSecretsConfigured
                          : usingDefault
                            ? t.common.detail
                            : t.admin.pluginSecretsNotConfigured}
                    </Badge>
                  </div>
                  <p className="font-mono text-[11px] text-muted-foreground">{field.key}</p>
                  {field.description ? (
                    <p className="text-xs text-muted-foreground">{field.description}</p>
                  ) : null}
                </div>
                {renderField(field)}
                {errors[field.key] ? (
                  <p className="text-xs text-destructive">{errors[field.key]}</p>
                ) : null}
              </div>
            )
          })
        )}
      </div>

      <p className="text-xs text-muted-foreground">{t.admin.pluginSchemaEditorHint}</p>
    </div>
  )
}
