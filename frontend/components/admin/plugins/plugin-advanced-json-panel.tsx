'use client'

import { Badge } from '@/components/ui/badge'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import type { Translations } from '@/lib/i18n'

import { PluginGovernanceNotice } from './plugin-governance-summary'

type PluginAdvancedJsonPanelProps = {
  title: string
  fieldLabel: string
  description: string
  value: string
  onChange: (value: string) => void
  onBlur?: () => void
  rows?: number
  invalid?: boolean
  invalidMessage?: string
  hints?: string[]
  t: Translations
}

export function PluginAdvancedJsonPanel({
  title,
  fieldLabel,
  description,
  value,
  onChange,
  onBlur,
  rows = 8,
  invalid = false,
  invalidMessage,
  hints = [],
  t,
}: PluginAdvancedJsonPanelProps) {
  const trimmedValue = value.trim()

  return (
    <details
      className={`overflow-hidden rounded-md border ${
        invalid ? 'border-destructive/30 bg-destructive/5' : 'border-input bg-muted/10'
      }`}
      open={invalid}
    >
      <summary className="cursor-pointer list-none px-3 py-3 [&::-webkit-details-marker]:hidden">
        <div className="flex flex-wrap items-start justify-between gap-2">
          <div className="min-w-0 space-y-1">
            <div className="flex flex-wrap items-center gap-2">
              <p className="text-sm font-medium">{title}</p>
              <Badge variant={invalid ? 'destructive' : trimmedValue ? 'active' : 'outline'}>
                {invalid ? t.common.warning : trimmedValue ? t.common.success : t.common.noData}
              </Badge>
            </div>
            <p className="text-xs text-muted-foreground">{description}</p>
          </div>
          <Badge variant="outline">{fieldLabel}</Badge>
        </div>
      </summary>
      <div className="space-y-3 border-t border-input/60 bg-background px-3 py-3">
        <div className="space-y-1">
          <Label>{fieldLabel}</Label>
          {hints.map((hint) => (
            <p key={hint} className="text-xs text-muted-foreground">
              {hint}
            </p>
          ))}
        </div>
        {invalid && invalidMessage ? (
          <PluginGovernanceNotice tone="danger">{invalidMessage}</PluginGovernanceNotice>
        ) : null}
        <Textarea
          value={value}
          onChange={(event) => onChange(event.target.value)}
          onBlur={onBlur}
          className="font-mono text-xs"
          rows={rows}
        />
      </div>
    </details>
  )
}
