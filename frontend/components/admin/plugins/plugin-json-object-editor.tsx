'use client'

import { useEffect, useMemo, useRef, useState } from 'react'

import { Plus, Trash2 } from 'lucide-react'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
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

type JsonObjectFieldType = 'string' | 'number' | 'boolean' | 'json' | 'null'

type JsonObjectEditorRow = {
  id: string
  key: string
  type: JsonObjectFieldType
  textValue: string
  booleanValue: boolean
}

type PluginJSONObjectEditorProps = {
  title: string
  description: string
  value: string
  onChange: (value: string) => void
  excludedKeys?: string[]
  emptyMessage?: string
  disabled?: boolean
  disabledReason?: string
  t: Translations
}

function isJSONObject(value: unknown): value is Record<string, unknown> {
  return !!value && typeof value === 'object' && !Array.isArray(value)
}

function inferRowType(value: unknown): JsonObjectFieldType {
  if (value === null) return 'null'
  if (typeof value === 'boolean') return 'boolean'
  if (typeof value === 'number') return 'number'
  if (typeof value === 'string') return 'string'
  return 'json'
}

function toRowTextValue(value: unknown, type: JsonObjectFieldType): string {
  switch (type) {
    case 'number':
      return typeof value === 'number' ? String(value) : ''
    case 'string':
      return typeof value === 'string' ? value : ''
    case 'json':
      return JSON.stringify(value ?? {}, null, 2)
    default:
      return ''
  }
}

function toRowBooleanValue(value: unknown): boolean {
  return typeof value === 'boolean' ? value : false
}

function makeDefaultTextValue(type: JsonObjectFieldType): string {
  switch (type) {
    case 'number':
      return '0'
    case 'json':
      return '{}'
    case 'string':
      return ''
    default:
      return ''
  }
}

function parseObjectText(value: string): Record<string, unknown> | null {
  const trimmed = value.trim()
  const source = trimmed || '{}'
  try {
    const parsed = JSON.parse(source)
    return isJSONObject(parsed) ? parsed : null
  } catch {
    return null
  }
}

function jsonObjectTypeLabel(type: JsonObjectFieldType, t: Translations): string {
  switch (type) {
    case 'number':
      return t.admin.pluginJsonObjectEditorTypeNumber
    case 'boolean':
      return t.admin.pluginJsonObjectEditorTypeBoolean
    case 'json':
      return t.admin.pluginJsonObjectEditorTypeJson
    case 'null':
      return t.admin.pluginJsonObjectEditorTypeNull
    case 'string':
    default:
      return t.admin.pluginJsonObjectEditorTypeString
  }
}

function buildObjectFromRows(
  rows: JsonObjectEditorRow[],
  t: Translations
): { normalized: string; formatted: string; error: string | null } {
  const nextObject: Record<string, unknown> = {}
  const seen = new Set<string>()

  for (const row of rows) {
    const key = row.key.trim()
    if (!key) continue
    if (seen.has(key)) {
      return {
        normalized: '',
        formatted: '',
        error: t.admin.pluginJsonObjectEditorDuplicateKey.replace('{key}', key),
      }
    }
    seen.add(key)

    switch (row.type) {
      case 'string':
        nextObject[key] = row.textValue
        break
      case 'number': {
        const numeric = Number(row.textValue.trim())
        if (!Number.isFinite(numeric)) {
          return {
            normalized: '',
            formatted: '',
            error: t.admin.pluginJsonObjectEditorInvalidNumber.replace('{key}', key),
          }
        }
        nextObject[key] = numeric
        break
      }
      case 'boolean':
        nextObject[key] = row.booleanValue
        break
      case 'null':
        nextObject[key] = null
        break
      case 'json':
        try {
          nextObject[key] = JSON.parse(row.textValue.trim() || '{}')
        } catch {
          return {
            normalized: '',
            formatted: '',
            error: t.admin.pluginJsonObjectEditorInvalidJsonValue.replace('{key}', key),
          }
        }
        break
    }
  }

  return {
    normalized: JSON.stringify(nextObject),
    formatted: JSON.stringify(nextObject, null, 2),
    error: null,
  }
}

export function PluginJSONObjectEditor({
  title,
  description,
  value,
  onChange,
  excludedKeys = [],
  emptyMessage,
  disabled = false,
  disabledReason,
  t,
}: PluginJSONObjectEditorProps) {
  const excludedKeySet = useMemo(
    () => new Set(excludedKeys.map((item) => item.trim()).filter(Boolean)),
    [excludedKeys]
  )
  const rowSeedRef = useRef(0)
  const lastCommittedRef = useRef<string>('')
  const [rows, setRows] = useState<JsonObjectEditorRow[]>([])
  const [editorError, setEditorError] = useState<string | null>(null)
  const uniqueTypeCount = new Set(rows.map((row) => row.type)).size
  const blankKeyCount = rows.filter((row) => !row.key.trim()).length
  const preservedFieldCount = excludedKeySet.size

  const nextRowId = () => {
    rowSeedRef.current += 1
    return `json-row-${rowSeedRef.current}`
  }

  useEffect(() => {
    const parsed = parseObjectText(value)
    if (!parsed) return
    const visibleObject = Object.fromEntries(
      Object.entries(parsed).filter(([key]) => !excludedKeySet.has(key))
    )
    const normalized = JSON.stringify(visibleObject)
    if (normalized === lastCommittedRef.current) return
    setRows(
      Object.entries(visibleObject).map(([key, fieldValue]) => {
        const type = inferRowType(fieldValue)
        return {
          id: nextRowId(),
          key,
          type,
          textValue: toRowTextValue(fieldValue, type),
          booleanValue: toRowBooleanValue(fieldValue),
        }
      })
    )
    setEditorError(null)
    lastCommittedRef.current = normalized
  }, [excludedKeySet, value])

  const syncRows = (nextRows: JsonObjectEditorRow[]) => {
    const built = buildObjectFromRows(nextRows, t)
    if (built.error) {
      setEditorError(built.error)
      return
    }
    const baseObject = parseObjectText(value) || {}
    const preservedEntries = Object.entries(baseObject).filter(([key]) => excludedKeySet.has(key))
    const nextObject = {
      ...Object.fromEntries(preservedEntries),
      ...(parseObjectText(built.normalized) || {}),
    }
    const formatted = JSON.stringify(nextObject, null, 2)
    setEditorError(null)
    lastCommittedRef.current = JSON.stringify(Object.fromEntries(
      Object.entries(nextObject).filter(([key]) => !excludedKeySet.has(key))
    ))
    onChange(formatted)
  }

  const updateRows = (
    updater: JsonObjectEditorRow[] | ((current: JsonObjectEditorRow[]) => JsonObjectEditorRow[]),
    options?: { sync?: boolean }
  ) => {
    const nextRows = typeof updater === 'function' ? updater(rows) : updater
    setRows(nextRows)
    if (options?.sync !== false) {
      syncRows(nextRows)
    }
  }

  const addRow = () => {
    setRows((current) => [
      ...current,
      {
        id: nextRowId(),
        key: '',
        type: 'string',
        textValue: '',
        booleanValue: false,
      },
    ])
    setEditorError(null)
  }

  const renderValueInput = (row: JsonObjectEditorRow) => {
    switch (row.type) {
      case 'boolean':
        return (
          <div className="flex h-10 items-center justify-between rounded-md border border-input px-3">
            <span className="text-sm text-muted-foreground">
              {row.booleanValue
                ? t.admin.pluginJsonObjectEditorBooleanTrue
                : t.admin.pluginJsonObjectEditorBooleanFalse}
            </span>
            <Switch
              checked={row.booleanValue}
              disabled={disabled}
              onCheckedChange={(checked) =>
                updateRows((current) =>
                  current.map((item) =>
                    item.id === row.id ? { ...item, booleanValue: checked } : item
                  )
                )
              }
            />
          </div>
        )
      case 'null':
        return (
          <div className="flex h-10 items-center rounded-md border border-dashed border-input px-3 text-sm text-muted-foreground">
            {t.admin.pluginJsonObjectEditorNullHint}
          </div>
        )
      case 'json':
        return (
          <Textarea
            value={row.textValue}
            onChange={(e) =>
              updateRows((current) =>
                current.map((item) =>
                  item.id === row.id ? { ...item, textValue: e.target.value } : item
                )
              )
            }
            disabled={disabled}
            className="font-mono text-xs"
            rows={4}
            placeholder={t.admin.pluginJsonObjectEditorJsonPlaceholder}
          />
        )
      default:
        return (
          <Input
            value={row.textValue}
            onChange={(e) =>
              updateRows((current) =>
                current.map((item) =>
                  item.id === row.id ? { ...item, textValue: e.target.value } : item
                )
              )
            }
            disabled={disabled}
            className={row.type === 'number' ? 'font-mono text-xs' : undefined}
            placeholder={
              row.type === 'number'
                ? t.admin.pluginJsonObjectEditorNumberPlaceholder
                : t.admin.pluginJsonObjectEditorStringPlaceholder
            }
          />
        )
    }
  }

  return (
    <div className="space-y-3 rounded-md border border-input p-3">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="space-y-1">
          <p className="text-sm font-medium">{title}</p>
          <p className="text-xs text-muted-foreground">{description}</p>
        </div>
        <Button type="button" variant="outline" size="sm" onClick={addRow} disabled={disabled}>
          <Plus className="mr-2 h-4 w-4" />
          {t.admin.pluginJsonObjectEditorAddField}
        </Button>
      </div>

      {editorError ? <PluginGovernanceNotice tone="danger">{editorError}</PluginGovernanceNotice> : null}
      {disabledReason ? <PluginGovernanceNotice tone="danger">{disabledReason}</PluginGovernanceNotice> : null}

      {rows.length === 0 ? (
        <div className="rounded-md border border-dashed border-input px-3 py-4 text-sm text-muted-foreground">
          {emptyMessage || t.admin.pluginJsonObjectEditorEmpty}
        </div>
      ) : (
        <div className="space-y-3">
          {rows.map((row) => (
            <div
              key={row.id}
              className={`rounded-md border p-3 ${
                row.key.trim()
                  ? 'border-input/60 bg-background'
                  : 'border-dashed border-input/70 bg-muted/10'
              }`}
            >
              <div className="mb-3 flex flex-wrap items-start justify-between gap-2">
                <div className="min-w-0 space-y-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <p className="text-sm font-medium">
                      {row.key.trim() || t.admin.pluginJsonObjectEditorFieldKey}
                    </p>
                    <Badge variant="outline">{jsonObjectTypeLabel(row.type, t)}</Badge>
                    {!row.key.trim() ? <Badge variant="secondary">{t.common.warning}</Badge> : null}
                  </div>
                  <p className="text-xs text-muted-foreground">
                    {row.key.trim()
                      ? row.key.trim()
                      : t.admin.pluginJsonObjectEditorKeyPlaceholder}
                  </p>
                </div>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  onClick={() => updateRows((current) => current.filter((item) => item.id !== row.id))}
                  disabled={disabled}
                  aria-label={t.common.delete}
                >
                  <Trash2 className="h-4 w-4" />
                </Button>
              </div>

              <div className="grid gap-3 md:grid-cols-[minmax(0,1fr)_160px_minmax(0,1fr)] md:items-start">
                <div className="space-y-2">
                  <Label>{t.admin.pluginJsonObjectEditorFieldKey}</Label>
                  <Input
                    value={row.key}
                    onChange={(e) =>
                      updateRows(
                        (current) =>
                          current.map((item) =>
                            item.id === row.id ? { ...item, key: e.target.value } : item
                          ),
                        { sync: true }
                      )
                    }
                    disabled={disabled}
                    placeholder={t.admin.pluginJsonObjectEditorKeyPlaceholder}
                  />
                </div>

                <div className="space-y-2">
                  <Label>{t.admin.pluginJsonObjectEditorFieldType}</Label>
                  <Select
                    value={row.type}
                    onValueChange={(nextType) =>
                      updateRows((current) =>
                        current.map((item) => {
                          if (item.id !== row.id) return item
                          const fieldType = nextType as JsonObjectFieldType
                          return {
                            ...item,
                            type: fieldType,
                            textValue: makeDefaultTextValue(fieldType),
                            booleanValue: false,
                          }
                        })
                      )
                    }
                    disabled={disabled}
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="string">{t.admin.pluginJsonObjectEditorTypeString}</SelectItem>
                      <SelectItem value="number">{t.admin.pluginJsonObjectEditorTypeNumber}</SelectItem>
                      <SelectItem value="boolean">
                        {t.admin.pluginJsonObjectEditorTypeBoolean}
                      </SelectItem>
                      <SelectItem value="json">{t.admin.pluginJsonObjectEditorTypeJson}</SelectItem>
                      <SelectItem value="null">{t.admin.pluginJsonObjectEditorTypeNull}</SelectItem>
                    </SelectContent>
                  </Select>
                </div>

                <div className="space-y-2">
                  <Label>{t.admin.pluginJsonObjectEditorFieldValue}</Label>
                  {renderValueInput(row)}
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      <p className="text-xs text-muted-foreground">{t.admin.pluginJsonObjectEditorHint}</p>
    </div>
  )
}
