'use client'

import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Switch } from '@/components/ui/switch'
import type { AdminPluginHookCatalogGroup } from '@/lib/api'
import type { Translations } from '@/lib/i18n'

import { PluginGovernanceNotice } from './plugin-governance-summary'
import type { PluginHookAccessState } from './types'

type PluginHookAccessEditorProps = {
  hookCatalog: AdminPluginHookCatalogGroup[]
  hookAccessState: PluginHookAccessState
  onHookAccessChange: (state: PluginHookAccessState) => void
  resolveHookGroupLabel: (groupKey: string) => string
  disabled?: boolean
  disabledReason?: string
  t: Translations
}

function normalizeStringList(values: string[]): string[] {
  const seen = new Set<string>()
  const out: string[] = []
  values.forEach((item) => {
    const normalized = String(item || '')
      .trim()
      .toLowerCase()
    if (!normalized || seen.has(normalized)) return
    seen.add(normalized)
    out.push(normalized)
  })
  return out
}

export function PluginHookAccessEditor({
  hookCatalog,
  hookAccessState,
  onHookAccessChange,
  resolveHookGroupLabel,
  disabled = false,
  disabledReason,
  t,
}: PluginHookAccessEditorProps) {
  const totalHookCount = hookCatalog.reduce((sum, group) => sum + group.hooks.length, 0)
  const targetHooks = hookAccessState.allowAllHooks
    ? hookAccessState.disabledHooks
    : hookAccessState.selectedHooks
  const sectionTitle = hookAccessState.allowAllHooks
    ? t.admin.pluginHookAccessDisabledList
    : t.admin.pluginHookAccessAllowedList

  const updateTargetHooks = (nextHooks: string[]) => {
    const normalized = normalizeStringList(nextHooks)
    if (hookAccessState.allowAllHooks) {
      onHookAccessChange({
        ...hookAccessState,
        disabledHooks: normalized,
      })
      return
    }
    onHookAccessChange({
      ...hookAccessState,
      selectedHooks: normalized,
    })
  }

  const toggleHook = (hook: string, checked: boolean) => {
    if (checked) {
      updateTargetHooks([...targetHooks, hook])
      return
    }
    updateTargetHooks(targetHooks.filter((item) => item !== hook))
  }

  const applyGroupSelection = (groupHooks: string[], checked: boolean) => {
    if (checked) {
      updateTargetHooks([...targetHooks, ...groupHooks])
      return
    }
    const groupSet = new Set(normalizeStringList(groupHooks))
    updateTargetHooks(targetHooks.filter((item) => !groupSet.has(item)))
  }

  return (
    <div className="space-y-3 rounded-md border border-input p-3">
      <div className="space-y-1">
        <p className="text-sm font-medium">{t.admin.pluginHookAccess}</p>
        <p className="text-xs text-muted-foreground">{t.admin.pluginHookAccessDesc}</p>
        <p className="text-xs text-muted-foreground">
          {[
            `${t.admin.pluginHookAccessAllowAll}: ${
              hookAccessState.allowAllHooks ? t.common.yes : t.common.no
            }`,
            `${sectionTitle}: ${targetHooks.length}/${totalHookCount}`,
            `${t.common.all}: ${totalHookCount}`,
            `${t.common.detail}: ${
              hookAccessState.allowAllHooks
                ? `${Math.max(totalHookCount - targetHooks.length, 0)}/${totalHookCount}`
                : `${targetHooks.length}/${totalHookCount}`
            }`,
          ].join(' · ')}
        </p>
      </div>

      {disabledReason ? <PluginGovernanceNotice tone="danger">{disabledReason}</PluginGovernanceNotice> : null}

      <div className="flex items-center justify-between gap-3 rounded-md border border-input/60 p-3">
        <div className="space-y-1">
          <p className="text-sm font-medium">{t.admin.pluginHookAccessAllowAll}</p>
          <p className="text-xs text-muted-foreground">{t.admin.pluginHookAccessAllowAllDesc}</p>
        </div>
        <Switch
          checked={hookAccessState.allowAllHooks}
          disabled={disabled}
          onCheckedChange={(checked) => {
            onHookAccessChange({
              ...hookAccessState,
              allowAllHooks: checked,
            })
          }}
        />
      </div>

      <div className="space-y-3 rounded-md border border-input/60 p-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="space-y-1">
            <p className="text-sm font-medium">{sectionTitle}</p>
            <p className="text-xs text-muted-foreground">{t.admin.pluginHookAccessDesc}</p>
          </div>
          <span className="text-xs text-muted-foreground">
            {hookAccessState.allowAllHooks
              ? `${targetHooks.length}/${totalHookCount}`
              : `${targetHooks.length}/${totalHookCount}`}
          </span>
        </div>

        {hookCatalog.length === 0 ? (
          <PluginGovernanceNotice tone="neutral">{t.common.noData}</PluginGovernanceNotice>
        ) : (
          <div className="space-y-3">
            {hookCatalog.map((group) => {
              const selectedInGroup = group.hooks.filter((hook) => targetHooks.includes(hook)).length
              return (
                <div key={group.key} className="space-y-3 rounded-md border border-input/60 p-3">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <div className="flex flex-wrap items-center gap-2">
                      <p className="text-sm font-medium">{resolveHookGroupLabel(group.key)}</p>
                      <span className="text-xs text-muted-foreground">
                        {selectedInGroup}/{group.hooks.length}
                      </span>
                    </div>
                    <div className="flex flex-wrap gap-2">
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        disabled={disabled || group.hooks.length === 0}
                        onClick={() => applyGroupSelection(group.hooks, true)}
                      >
                        {t.admin.pluginHookAccessSelectAll}
                      </Button>
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        disabled={disabled || group.hooks.length === 0}
                        onClick={() => applyGroupSelection(group.hooks, false)}
                      >
                        {t.admin.pluginHookAccessClear}
                      </Button>
                    </div>
                  </div>

                  <div className="grid gap-2 md:grid-cols-2">
                    {group.hooks.map((hook) => {
                      const checked = targetHooks.includes(hook)
                      return (
                        <label
                          key={hook}
                          className={`flex items-center gap-2 rounded border px-3 py-2 text-xs ${
                            checked ? 'border-primary/30 bg-primary/5' : 'border-input/50'
                          }`}
                        >
                          <Checkbox
                            checked={checked}
                            disabled={disabled}
                            onCheckedChange={(value) => toggleHook(hook, value === true)}
                          />
                          <span className="font-mono">{hook}</span>
                        </label>
                      )
                    })}
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </div>
    </div>
  )
}
