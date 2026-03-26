'use client'

import type { ReactNode } from 'react'

export function PluginGovernanceSummaryCard({
  label,
  value,
  badge,
}: {
  label: string
  value: ReactNode
  badge?: ReactNode
}) {
  return (
    <div className="rounded-md border border-input/60 bg-muted/10 p-3">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <p className="text-xs text-muted-foreground">{label}</p>
        {badge ? <div className="shrink-0">{badge}</div> : null}
      </div>
      <div className="mt-2 text-sm font-semibold">{value}</div>
    </div>
  )
}

export function PluginGovernanceNotice({
  children,
  tone = 'warning',
}: {
  children: ReactNode
  tone?: 'neutral' | 'warning' | 'danger'
}) {
  const toneClassName =
    tone === 'danger'
      ? 'border-destructive/30 bg-destructive/5 text-destructive dark:border-destructive/40 dark:bg-destructive/10'
      : tone === 'neutral'
        ? 'border-input/60 bg-muted/10 text-foreground'
        : 'border-amber-500/30 bg-amber-500/10 text-amber-700 dark:border-amber-500/40 dark:bg-amber-950/20 dark:text-amber-300'

  return (
    <div className={`rounded-md border p-3 text-sm ${toneClassName}`}>{children}</div>
  )
}
