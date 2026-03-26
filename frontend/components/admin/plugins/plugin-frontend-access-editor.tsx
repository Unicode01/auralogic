'use client'

import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import type { Translations } from '@/lib/i18n'

import { PluginGovernanceNotice } from './plugin-governance-summary'
import type { PluginFrontendAccessState } from './types'

export type PluginFrontendSlotCatalogGroup = {
  key: string
  slots: string[]
}

export type PluginFrontendPermissionOption = {
  value: string
  label: string
}

export type PluginFrontendPermissionCatalogGroup = {
  key: string
  label: string
  permissions: PluginFrontendPermissionOption[]
}

type PluginFrontendAccessEditorProps = {
  slotCatalog: PluginFrontendSlotCatalogGroup[]
  permissionCatalog: PluginFrontendPermissionCatalogGroup[]
  frontendAccessState: PluginFrontendAccessState
  onFrontendAccessChange: (state: PluginFrontendAccessState) => void
  resolveSlotGroupLabel: (groupKey: string) => string
  disabled?: boolean
  disabledReason?: string
  validationMessage?: string | null
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

function frontendScopeLabel(value: PluginFrontendAccessState['frontendMinScope'], t: Translations) {
  switch (value) {
    case 'authenticated':
      return t.admin.pluginFrontendMinScopeAuthenticated
    case 'super_admin':
      return t.admin.pluginFrontendMinScopeSuperAdmin
    case 'guest':
    default:
      return t.admin.pluginFrontendMinScopeGuest
  }
}

export function PluginFrontendAccessEditor({
  slotCatalog,
  permissionCatalog,
  frontendAccessState,
  onFrontendAccessChange,
  resolveSlotGroupLabel,
  disabled = false,
  disabledReason,
  validationMessage,
  t,
}: PluginFrontendAccessEditorProps) {
  const availableAreas = [
    { key: 'user', label: t.admin.pluginFrontendAreaUser },
    { key: 'admin', label: t.admin.pluginFrontendAreaAdmin },
  ]
  const totalSlotCount = slotCatalog.reduce((sum, group) => sum + group.slots.length, 0)
  const totalPermissionCount = permissionCatalog.reduce((sum, group) => sum + group.permissions.length, 0)
  const selectedCountLabel = (count: number) =>
    t.admin.selectedCount.replace('{count}', String(count))

  const updateAreas = (areas: string[]) => {
    onFrontendAccessChange({
      ...frontendAccessState,
      selectedFrontendAreas: normalizeStringList(areas),
    })
  }

  const updateSlots = (slots: string[]) => {
    onFrontendAccessChange({
      ...frontendAccessState,
      selectedFrontendSlots: normalizeStringList(slots),
    })
  }

  const updatePermissions = (permissions: string[]) => {
    onFrontendAccessChange({
      ...frontendAccessState,
      frontendRequiredPermissions: normalizeStringList(permissions),
    })
  }

  return (
    <div className="space-y-3 rounded-md border border-input p-3">
      <div className="space-y-1">
        <p className="text-sm font-medium">{t.admin.pluginFrontendAccess}</p>
        <p className="text-xs text-muted-foreground">{t.admin.pluginFrontendAccessDesc}</p>
        <p className="text-xs text-muted-foreground">
          {[
            `${t.admin.pluginFrontendExtensionsEnabled}: ${
              frontendAccessState.allowFrontendExtensions ? t.common.yes : t.common.no
            }`,
            `${t.admin.pluginFrontendMinScope}: ${frontendScopeLabel(
              frontendAccessState.frontendMinScope,
              t
            )}`,
            `${t.admin.pluginFrontendAreas}: ${
              frontendAccessState.allowAllFrontendAreas
                ? t.common.all
                : selectedCountLabel(frontendAccessState.selectedFrontendAreas.length)
            }`,
            `${t.admin.pluginFrontendSlots}: ${
              frontendAccessState.allowAllFrontendSlots
                ? t.common.all
                : `${frontendAccessState.selectedFrontendSlots.length}/${totalSlotCount}`
            }`,
            `${t.admin.pluginFrontendRequiredPermissions}: ${
              frontendAccessState.frontendRequiredPermissions.length
            }/${totalPermissionCount}`,
          ].join(' · ')}
        </p>
      </div>

      {disabledReason ? <PluginGovernanceNotice tone="danger">{disabledReason}</PluginGovernanceNotice> : null}
      {validationMessage ? (
        <PluginGovernanceNotice tone="danger">{validationMessage}</PluginGovernanceNotice>
      ) : null}

      <div className="flex items-center justify-between gap-3 rounded-md border border-input/60 p-3">
        <div className="space-y-1">
          <p className="text-sm font-medium">{t.admin.pluginFrontendExtensionsEnabled}</p>
          <p className="text-xs text-muted-foreground">
            {t.admin.pluginFrontendExtensionsEnabledDesc}
          </p>
        </div>
        <Switch
          checked={frontendAccessState.allowFrontendExtensions}
          disabled={disabled}
          onCheckedChange={(checked) => {
            onFrontendAccessChange({
              ...frontendAccessState,
              allowFrontendExtensions: checked,
            })
          }}
        />
      </div>

      <div className="space-y-2 rounded-md border border-input/60 p-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="space-y-1">
            <p className="text-sm font-medium">{t.admin.pluginFrontendMinScope}</p>
            <p className="text-xs text-muted-foreground">{t.admin.pluginFrontendMinScopeDesc}</p>
          </div>
          <span className="text-xs text-muted-foreground">
            {frontendScopeLabel(frontendAccessState.frontendMinScope, t)}
          </span>
        </div>
        <Select
          value={frontendAccessState.frontendMinScope}
          onValueChange={(value: 'guest' | 'authenticated' | 'super_admin') => {
            onFrontendAccessChange({
              ...frontendAccessState,
              frontendMinScope: value,
            })
          }}
          disabled={disabled}
        >
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="guest">{t.admin.pluginFrontendMinScopeGuest}</SelectItem>
            <SelectItem value="authenticated">
              {t.admin.pluginFrontendMinScopeAuthenticated}
            </SelectItem>
            <SelectItem value="super_admin">{t.admin.pluginFrontendMinScopeSuperAdmin}</SelectItem>
          </SelectContent>
        </Select>
      </div>

      <div className="space-y-3 rounded-md border border-input/60 p-3">
        <div className="flex items-center justify-between gap-3">
          <div className="space-y-1">
            <p className="text-sm font-medium">{t.admin.pluginFrontendAreas}</p>
            <p className="text-xs text-muted-foreground">{t.admin.pluginFrontendAreasDesc}</p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-xs text-muted-foreground">
              {frontendAccessState.allowAllFrontendAreas
                ? t.admin.pluginFrontendAreasAll
                : selectedCountLabel(frontendAccessState.selectedFrontendAreas.length)}
            </span>
            <div className="flex items-center gap-2">
              <span className="text-xs text-muted-foreground">
                {t.admin.pluginFrontendAreasAll}
              </span>
              <Switch
                checked={frontendAccessState.allowAllFrontendAreas}
                disabled={disabled}
                onCheckedChange={(checked) => {
                  onFrontendAccessChange({
                    ...frontendAccessState,
                    allowAllFrontendAreas: checked,
                  })
                }}
              />
            </div>
          </div>
        </div>

        {!frontendAccessState.allowAllFrontendAreas ? (
          <div className="grid gap-2 md:grid-cols-2">
            {availableAreas.map((item) => {
              const checked = frontendAccessState.selectedFrontendAreas.includes(item.key)
              return (
                <label
                  key={item.key}
                  className={`flex items-center gap-2 rounded border px-3 py-2 text-sm ${
                    checked ? 'border-primary/30 bg-primary/5' : 'border-input/50'
                  }`}
                >
                  <Checkbox
                    checked={checked}
                    disabled={disabled}
                    onCheckedChange={(value) => {
                      if (value === true) {
                        updateAreas([...frontendAccessState.selectedFrontendAreas, item.key])
                        return
                      }
                      updateAreas(
                        frontendAccessState.selectedFrontendAreas.filter((area) => area !== item.key)
                      )
                    }}
                  />
                  <span>{item.label}</span>
                </label>
              )
            })}
          </div>
        ) : null}
      </div>

      <div className="space-y-3 rounded-md border border-input/60 p-3">
        <div className="flex items-center justify-between gap-3">
          <div className="space-y-1">
            <p className="text-sm font-medium">{t.admin.pluginFrontendSlots}</p>
            <p className="text-xs text-muted-foreground">{t.admin.pluginFrontendSlotsDesc}</p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-xs text-muted-foreground">
              {frontendAccessState.allowAllFrontendSlots
                ? t.admin.pluginFrontendSlotsAll
                : `${frontendAccessState.selectedFrontendSlots.length}/${totalSlotCount}`}
            </span>
            <div className="flex items-center gap-2">
              <span className="text-xs text-muted-foreground">{t.admin.pluginFrontendSlotsAll}</span>
              <Switch
                checked={frontendAccessState.allowAllFrontendSlots}
                disabled={disabled}
                onCheckedChange={(checked) => {
                  onFrontendAccessChange({
                    ...frontendAccessState,
                    allowAllFrontendSlots: checked,
                  })
                }}
              />
            </div>
          </div>
        </div>

        {!frontendAccessState.allowAllFrontendSlots ? (
          slotCatalog.length === 0 ? (
            <PluginGovernanceNotice tone="neutral">{t.common.noData}</PluginGovernanceNotice>
          ) : (
            <div className="space-y-3">
              {slotCatalog.map((group) => {
                const selectedInGroup = group.slots.filter((slot) =>
                  frontendAccessState.selectedFrontendSlots.includes(slot)
                ).length
                return (
                  <div key={group.key} className="space-y-3 rounded-md border border-input/50 p-3">
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <div className="flex flex-wrap items-center gap-2">
                        <p className="text-sm font-medium">{resolveSlotGroupLabel(group.key)}</p>
                        <span className="text-xs text-muted-foreground">
                          {selectedInGroup}/{group.slots.length}
                        </span>
                      </div>
                      <div className="flex flex-wrap gap-2">
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          disabled={disabled || group.slots.length === 0}
                          onClick={() =>
                            updateSlots([
                              ...frontendAccessState.selectedFrontendSlots,
                              ...group.slots,
                            ])
                          }
                        >
                          {t.admin.pluginFrontendSlotsSelectAll}
                        </Button>
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          disabled={disabled || group.slots.length === 0}
                          onClick={() => {
                            const groupSet = new Set(normalizeStringList(group.slots))
                            updateSlots(
                              frontendAccessState.selectedFrontendSlots.filter(
                                (slot) => !groupSet.has(slot)
                              )
                            )
                          }}
                        >
                          {t.admin.pluginFrontendSlotsClear}
                        </Button>
                      </div>
                    </div>

                    <div className="grid gap-2 md:grid-cols-2">
                      {group.slots.map((slot) => {
                        const checked = frontendAccessState.selectedFrontendSlots.includes(slot)
                        return (
                          <label
                            key={slot}
                            className={`flex items-center gap-2 rounded border px-3 py-2 text-xs ${
                              checked ? 'border-primary/30 bg-primary/5' : 'border-input/50'
                            }`}
                          >
                            <Checkbox
                              checked={checked}
                              disabled={disabled}
                              onCheckedChange={(value) => {
                                if (value === true) {
                                  updateSlots([...frontendAccessState.selectedFrontendSlots, slot])
                                  return
                                }
                                updateSlots(
                                  frontendAccessState.selectedFrontendSlots.filter(
                                    (item) => item !== slot
                                  )
                                )
                              }}
                            />
                            <span className="font-mono">{slot}</span>
                          </label>
                        )
                      })}
                    </div>
                  </div>
                )
              })}
            </div>
          )
        ) : null}
      </div>

      <div className="space-y-3 rounded-md border border-input/60 p-3">
        <div className="space-y-1">
          <div className="flex flex-wrap items-center gap-2">
            <p className="text-sm font-medium">{t.admin.pluginFrontendRequiredPermissions}</p>
            <span className="text-xs text-muted-foreground">
              {frontendAccessState.frontendRequiredPermissions.length}
            </span>
          </div>
          <p className="text-xs text-muted-foreground">
            {t.admin.pluginFrontendRequiredPermissionsDesc}
          </p>
        </div>

        {permissionCatalog.length === 0 ? (
          <PluginGovernanceNotice tone="neutral">{t.common.noData}</PluginGovernanceNotice>
        ) : (
          <div className="space-y-3">
            {permissionCatalog.map((group) => {
              const selectedInGroup = group.permissions.filter((permission) =>
                frontendAccessState.frontendRequiredPermissions.includes(permission.value)
              ).length
              return (
                <div key={group.key} className="space-y-3 rounded-md border border-input/50 p-3">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <div className="flex flex-wrap items-center gap-2">
                      <p className="text-sm font-medium">{group.label}</p>
                      <span className="text-xs text-muted-foreground">
                        {selectedInGroup}/{group.permissions.length}
                      </span>
                    </div>
                    <div className="flex flex-wrap gap-2">
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        disabled={disabled || group.permissions.length === 0}
                        onClick={() =>
                          updatePermissions([
                            ...frontendAccessState.frontendRequiredPermissions,
                            ...group.permissions.map((item) => item.value),
                          ])
                        }
                      >
                        {t.admin.pluginFrontendPermissionsSelectAll}
                      </Button>
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        disabled={disabled || group.permissions.length === 0}
                        onClick={() => {
                          const groupSet = new Set(
                            normalizeStringList(group.permissions.map((item) => item.value))
                          )
                          updatePermissions(
                            frontendAccessState.frontendRequiredPermissions.filter(
                              (permission) => !groupSet.has(permission)
                            )
                          )
                        }}
                      >
                        {t.admin.pluginFrontendPermissionsClear}
                      </Button>
                    </div>
                  </div>

                  <div className="grid gap-2 md:grid-cols-2">
                    {group.permissions.map((permission) => {
                      const checked = frontendAccessState.frontendRequiredPermissions.includes(
                        permission.value
                      )
                      return (
                        <label
                          key={permission.value}
                          className={`flex items-center gap-2 rounded border px-3 py-2 text-xs ${
                            checked ? 'border-primary/30 bg-primary/5' : 'border-input/50'
                          }`}
                        >
                          <Checkbox
                            checked={checked}
                            disabled={disabled}
                            onCheckedChange={(value) => {
                              if (value === true) {
                                updatePermissions([
                                  ...frontendAccessState.frontendRequiredPermissions,
                                  permission.value,
                                ])
                                return
                              }
                              updatePermissions(
                                frontendAccessState.frontendRequiredPermissions.filter(
                                  (item) => item !== permission.value
                                )
                              )
                            }}
                          />
                          <span>{permission.label}</span>
                          <span className="font-mono text-muted-foreground">
                            {permission.value}
                          </span>
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
