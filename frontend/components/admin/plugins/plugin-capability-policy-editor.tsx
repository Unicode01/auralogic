'use client'

import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import { Switch } from '@/components/ui/switch'
import type { Translations } from '@/lib/i18n'

import { PluginGovernanceNotice } from './plugin-governance-summary'
import type { PluginCapabilityPolicyState } from './types'

export type PluginCapabilityPermissionOption = {
  value: string
  title: string
  description: string
}

type PluginCapabilityPolicyEditorProps = {
  capabilityPolicyState: PluginCapabilityPolicyState
  onCapabilityPolicyChange: (state: PluginCapabilityPolicyState) => void
  permissionOptions: PluginCapabilityPermissionOption[]
  showPermissionEditor?: boolean
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

export function PluginCapabilityPolicyEditor({
  capabilityPolicyState,
  onCapabilityPolicyChange,
  permissionOptions,
  showPermissionEditor = true,
  disabled = false,
  disabledReason,
  t,
}: PluginCapabilityPolicyEditorProps) {
  const capabilityFlags: Array<{
    key:
      | 'allowBlock'
      | 'allowPayloadPatch'
      | 'allowExecuteApi'
      | 'allowNetwork'
      | 'allowFileSystem'
      | 'trustedHtmlMode'
    title: string
    description: string
    elevated?: boolean
  }> = [
    {
      key: 'allowBlock',
      title: t.admin.pluginCapabilityAllowBlock,
      description: t.admin.pluginCapabilityAllowBlockDesc,
    },
    {
      key: 'allowPayloadPatch',
      title: t.admin.pluginCapabilityAllowPayloadPatch,
      description: t.admin.pluginCapabilityAllowPayloadPatchDesc,
    },
    {
      key: 'allowExecuteApi',
      title: t.admin.pluginCapabilityAllowExecuteApi,
      description: t.admin.pluginCapabilityAllowExecuteApiDesc,
      elevated: true,
    },
    {
      key: 'allowNetwork',
      title: t.admin.pluginCapabilityAllowNetwork,
      description: t.admin.pluginCapabilityAllowNetworkDesc,
      elevated: true,
    },
    {
      key: 'allowFileSystem',
      title: t.admin.pluginCapabilityAllowFileSystem,
      description: t.admin.pluginCapabilityAllowFileSystemDesc,
      elevated: true,
    },
    {
      key: 'trustedHtmlMode',
      title: t.admin.pluginCapabilityTrustedHtmlMode,
      description: t.admin.pluginCapabilityTrustedHtmlModeDesc,
      elevated: true,
    },
  ]
  const enabledFlagCount = capabilityFlags.filter((item) => capabilityPolicyState[item.key]).length
  const elevatedFlagCount = capabilityFlags.filter(
    (item) => item.elevated && capabilityPolicyState[item.key]
  ).length
  const requestedCount = capabilityPolicyState.requestedPermissions.length
  const grantedCount = capabilityPolicyState.grantedPermissions.length
  const pendingPermissions = capabilityPolicyState.requestedPermissions.filter(
    (permission) => !capabilityPolicyState.grantedPermissions.includes(permission)
  )
  const pendingGrantCount = pendingPermissions.length
  const selectedCountLabel = (count: number) =>
    t.admin.selectedCount.replace('{count}', String(count))

  const updateRequestedPermissions = (permissions: string[]) => {
    const normalizedRequested = normalizeStringList(permissions)
    onCapabilityPolicyChange({
      ...capabilityPolicyState,
      requestedPermissions: normalizedRequested,
      grantedPermissions: capabilityPolicyState.grantedPermissions.filter((permission) =>
        normalizedRequested.includes(permission)
      ),
    })
  }

  const updateGrantedPermissions = (permissions: string[]) => {
    const normalizedGranted = normalizeStringList(permissions)
    onCapabilityPolicyChange({
      ...capabilityPolicyState,
      requestedPermissions: normalizeStringList([
        ...capabilityPolicyState.requestedPermissions,
        ...normalizedGranted,
      ]),
      grantedPermissions: normalizedGranted,
    })
  }

  return (
    <div className="space-y-3 rounded-md border border-input p-3">
      <div className="space-y-1">
        <p className="text-sm font-medium">{t.admin.pluginCapabilityPolicy}</p>
        <p className="text-xs text-muted-foreground">{t.admin.pluginCapabilityPolicyDesc}</p>
        <p className="text-xs text-muted-foreground">
          {[
            `${t.admin.pluginCapabilityFlags}: ${enabledFlagCount}`,
            `${t.admin.pluginCapabilityRequestedPermissions}: ${requestedCount}`,
            `${t.admin.pluginCapabilityGrantedPermissions}: ${grantedCount}`,
            `${t.admin.pluginCapabilityTrustedHtmlMode}: ${
              capabilityPolicyState.trustedHtmlMode ? t.common.yes : t.common.no
            }`,
            elevatedFlagCount > 0 ? `${t.common.warning}: ${elevatedFlagCount}` : null,
          ]
            .filter(Boolean)
            .join(' · ')}
        </p>
      </div>

      {disabledReason ? <PluginGovernanceNotice tone="danger">{disabledReason}</PluginGovernanceNotice> : null}

      <div className="space-y-3 rounded-md border border-input/60 p-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <p className="text-sm font-medium">{t.admin.pluginCapabilityFlags}</p>
          <p className="text-xs text-muted-foreground">
            {selectedCountLabel(enabledFlagCount)}
            {elevatedFlagCount > 0 ? ` · ${t.common.warning}: ${elevatedFlagCount}` : ''}
          </p>
        </div>

        {capabilityFlags.map((item) => (
          <div
            key={item.key}
            className={`flex items-center justify-between gap-3 rounded border p-3 ${
              capabilityPolicyState[item.key]
                ? 'border-primary/30 bg-primary/5'
                : 'border-input/50 bg-background'
            }`}
          >
            <div className="space-y-1">
              <div className="flex flex-wrap items-center gap-2">
                <p className="text-sm font-medium">{item.title}</p>
                {item.elevated ? <Badge variant="secondary">{t.common.warning}</Badge> : null}
              </div>
              <p className="text-xs text-muted-foreground">{item.description}</p>
            </div>
            <Switch
              checked={capabilityPolicyState[item.key]}
              disabled={disabled}
              onCheckedChange={(checked) =>
                onCapabilityPolicyChange({
                  ...capabilityPolicyState,
                  [item.key]: checked,
                })
              }
            />
          </div>
        ))}
        <p className="text-xs text-muted-foreground">{t.admin.pluginCapabilityTrustedHtmlModeHint}</p>
      </div>

      {showPermissionEditor ? (
        <div className="space-y-3 rounded-md border border-input/60 p-3">
        <div className="space-y-1">
          <div className="flex flex-wrap items-center gap-2">
            <p className="text-sm font-medium">{t.admin.pluginCapabilityPermissions}</p>
            <span className="text-xs text-muted-foreground">
              {requestedCount}/{grantedCount}
            </span>
          </div>
          <p className="text-xs text-muted-foreground">
            {t.admin.pluginCapabilityPermissionsDesc}
            </p>
          </div>

          <div className="space-y-2">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <p className="text-sm font-medium">{t.admin.pluginCapabilityRequestedPermissions}</p>
              <span className="text-xs text-muted-foreground">
                {selectedCountLabel(requestedCount)}
              </span>
            </div>
            {permissionOptions.length === 0 ? (
              <PluginGovernanceNotice tone="neutral">{t.common.noData}</PluginGovernanceNotice>
            ) : (
              <div className="grid gap-2 md:grid-cols-2">
                {permissionOptions.map((permission) => {
                  const checked = capabilityPolicyState.requestedPermissions.includes(permission.value)
                  return (
                    <label
                      key={`requested-${permission.value}`}
                      className={`space-y-1 rounded border px-3 py-2 ${
                        checked ? 'border-primary/30 bg-primary/5' : 'border-input/50'
                      }`}
                    >
                      <span className="flex items-center gap-2 text-sm">
                        <Checkbox
                          checked={checked}
                          disabled={disabled}
                          onCheckedChange={(value) => {
                            if (value === true) {
                              updateRequestedPermissions([
                                ...capabilityPolicyState.requestedPermissions,
                                permission.value,
                              ])
                              return
                            }
                            updateRequestedPermissions(
                              capabilityPolicyState.requestedPermissions.filter(
                                (item) => item !== permission.value
                              )
                            )
                          }}
                        />
                        <span>{permission.title}</span>
                      </span>
                      <p className="font-mono text-[11px] text-muted-foreground">{permission.value}</p>
                      <p className="text-xs text-muted-foreground">{permission.description}</p>
                    </label>
                  )
                })}
              </div>
            )}
          </div>

          <div className="space-y-2">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <p className="text-sm font-medium">{t.admin.pluginCapabilityGrantedPermissions}</p>
              <p className="text-xs text-muted-foreground">
                {selectedCountLabel(grantedCount)}
                {pendingGrantCount > 0 ? ` · ${t.common.warning}: ${pendingGrantCount}` : ''}
              </p>
            </div>
            {permissionOptions.length === 0 ? (
              <PluginGovernanceNotice tone="neutral">{t.common.noData}</PluginGovernanceNotice>
            ) : (
              <div className="grid gap-2 md:grid-cols-2">
                {permissionOptions.map((permission) => {
                  const requested = capabilityPolicyState.requestedPermissions.includes(permission.value)
                  const checked = capabilityPolicyState.grantedPermissions.includes(permission.value)
                  return (
                    <label
                      key={`granted-${permission.value}`}
                      className={`space-y-1 rounded border px-3 py-2 ${
                        checked ? 'border-primary/30 bg-primary/5' : 'border-input/50'
                      }`}
                    >
                      <span className="flex items-center gap-2 text-sm">
                        <Checkbox
                          checked={checked}
                          disabled={disabled}
                          onCheckedChange={(value) => {
                            if (value === true) {
                              updateGrantedPermissions([
                                ...capabilityPolicyState.grantedPermissions,
                                permission.value,
                              ])
                              return
                            }
                            updateGrantedPermissions(
                              capabilityPolicyState.grantedPermissions.filter(
                                (item) => item !== permission.value
                              )
                            )
                          }}
                        />
                        <span>{permission.title}</span>
                      </span>
                      <p className="font-mono text-[11px] text-muted-foreground">
                        {permission.value}
                      </p>
                      <p className="text-xs text-muted-foreground">{permission.description}</p>
                      {!requested ? (
                        <p className="text-xs text-amber-600">
                          {t.admin.pluginCapabilityGrantAutoRequestHint}
                        </p>
                      ) : null}
                    </label>
                  )
                })}
              </div>
            )}
          </div>
        </div>
      ) : null}

      {pendingGrantCount > 0 ? (
        <PluginGovernanceNotice>
          {t.admin.pluginPermissionMissingRequired.replace('{list}', pendingPermissions.join(', '))}
        </PluginGovernanceNotice>
      ) : null}
    </div>
  )
}
