'use client'

import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import type { AdminPluginSecretMeta } from '@/lib/api'
import type { Translations } from '@/lib/i18n'

import { PluginGovernanceNotice } from './plugin-governance-summary'
import type { PluginJSONSchema } from './types'

type PluginSecretSchemaEditorProps = {
  title: string
  description: string
  schema: PluginJSONSchema
  secretMeta: Record<string, AdminPluginSecretMeta | undefined>
  drafts: Record<string, string>
  deleteKeys: string[]
  onDraftChange: (key: string, value: string) => void
  onDeleteToggle: (key: string, checked: boolean) => void
  t: Translations
}

export function PluginSecretSchemaEditor({
  title,
  description,
  schema,
  secretMeta,
  drafts,
  deleteKeys,
  onDraftChange,
  onDeleteToggle,
  t,
}: PluginSecretSchemaEditorProps) {
  const deleteKeySet = new Set(deleteKeys)
  const configuredCount = schema.fields.filter(
    (field) => secretMeta[field.key]?.configured === true
  ).length
  const pendingDeleteCount = schema.fields.filter((field) => deleteKeySet.has(field.key)).length
  const pendingUpdateCount = schema.fields.filter((field) => {
    if (deleteKeySet.has(field.key)) return false
    return (drafts[field.key] || '').trim() !== ''
  }).length
  const selectedCountLabel = (count: number) =>
    t.admin.selectedCount.replace('{count}', String(count))

  return (
    <div className="space-y-3 rounded-md border border-input p-3">
      <div className="space-y-1">
        <p className="text-sm font-medium">{title}</p>
        <p className="text-xs text-muted-foreground">{description}</p>
      </div>

      <div className="flex flex-wrap gap-2">
        <Badge variant="outline">
          {t.common.all}: {schema.fields.length}
        </Badge>
        <Badge variant={configuredCount > 0 ? 'active' : 'outline'}>
          {t.admin.pluginSecretsConfigured}: {configuredCount}
        </Badge>
        <Badge variant={pendingUpdateCount > 0 ? 'secondary' : 'outline'}>
          {t.admin.pluginSecretsPendingUpdate}: {pendingUpdateCount}
        </Badge>
        <Badge variant={pendingDeleteCount > 0 ? 'destructive' : 'outline'}>
          {t.admin.pluginSecretsPendingClear}: {pendingDeleteCount}
        </Badge>
      </div>

      {pendingDeleteCount > 0 ? (
        <PluginGovernanceNotice>
          {`${t.admin.pluginSecretsPendingClear}: ${selectedCountLabel(pendingDeleteCount)}`}
        </PluginGovernanceNotice>
      ) : null}

      <div className="grid gap-3">
        {schema.fields.length === 0 ? (
          <PluginGovernanceNotice tone="neutral">{t.common.noData}</PluginGovernanceNotice>
        ) : (
          schema.fields.map((field) => {
            const fieldKey = field.key
            const configured = secretMeta[fieldKey]?.configured === true
            const pendingDelete = deleteKeySet.has(fieldKey)
            const pendingUpdate = !pendingDelete && (drafts[fieldKey] || '').trim() !== ''
            const statusLabel = pendingDelete
              ? t.admin.pluginSecretsPendingClear
              : pendingUpdate
                ? t.admin.pluginSecretsPendingUpdate
                : configured
                  ? t.admin.pluginSecretsConfigured
                  : t.admin.pluginSecretsNotConfigured
            const isTextarea = field.type === 'textarea'
            const inputPlaceholder = field.placeholder || t.admin.pluginSecretsKeepExistingHint

            return (
              <div
                key={fieldKey}
                className={`space-y-3 rounded-md border p-3 ${
                  pendingDelete
                    ? 'border-destructive/30 bg-destructive/5'
                    : pendingUpdate
                      ? 'border-primary/30 bg-primary/5'
                      : 'border-input/60'
                }`}
              >
                <div className="space-y-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <Label>{field.required ? `${field.label} *` : field.label}</Label>
                    <Badge
                      variant={pendingDelete ? 'destructive' : pendingUpdate ? 'secondary' : configured ? 'active' : 'outline'}
                    >
                      {statusLabel}
                    </Badge>
                  </div>
                  <p className="font-mono text-[11px] text-muted-foreground">{fieldKey}</p>
                  {field.description ? (
                    <p className="text-xs text-muted-foreground">{field.description}</p>
                  ) : null}
                </div>

                {isTextarea ? (
                  <Textarea
                    value={drafts[fieldKey] || ''}
                    rows={3}
                    placeholder={inputPlaceholder}
                    disabled={pendingDelete}
                    onChange={(e) => onDraftChange(fieldKey, e.target.value)}
                  />
                ) : (
                  <Input
                    type="password"
                    value={drafts[fieldKey] || ''}
                    placeholder={inputPlaceholder}
                    disabled={pendingDelete}
                    onChange={(e) => onDraftChange(fieldKey, e.target.value)}
                  />
                )}

                <label className="flex items-center gap-2 text-xs text-muted-foreground">
                  <Checkbox
                    checked={pendingDelete}
                    onCheckedChange={(checked) => onDeleteToggle(fieldKey, checked === true)}
                  />
                  {t.admin.pluginSecretsClearExisting}
                </label>
              </div>
            )
          })
        )}
      </div>

      <p className="text-xs text-muted-foreground">{t.admin.pluginSecretsEditorHint}</p>
    </div>
  )
}
